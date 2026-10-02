package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIValidation(t *testing.T) {
	for _, args := range [][]string{
		{"-max-parallel=0"}, {"-max-parallel=-1"}, {"-max-parallel=invalid"},
		{"-output-dir="}, {"-unknown"}, {"unexpected"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			var stderr bytes.Buffer
			if err := run(context.Background(), args, &stderr); err == nil {
				t.Fatal("expected invalid arguments to fail")
			}
		})
	}
}

func TestCLIHelp(t *testing.T) {
	var stderr bytes.Buffer
	if err := run(context.Background(), []string{"-help"}, &stderr); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"-max-parallel", "default 10", "-output-dir", "publish"} {
		if !strings.Contains(stderr.String(), text) {
			t.Errorf("help missing %q: %s", text, stderr.String())
		}
	}
}

type testTransport func(*http.Request) (*http.Response, error)

func (f testTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestCLIWritesToOutputDirectory(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	http.DefaultTransport = testTransport(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "en.greatfire.org" || r.URL.Path != "/api/tags" {
			t.Errorf("unexpected API URL: %s", r.URL)
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{"total":4,"items":[
					{"registrable":"EXAMPLE.COM"}, {"registrable":"www.example.com"},
					{"registrable":"192.0.2.0"}, {"registrable":"192.0.2.1"}
				]}`)),
			Header: make(http.Header),
		}, nil
	})
	dir := filepath.Join(t.TempDir(), "custom")
	var stderr bytes.Buffer
	if err := run(context.Background(), []string{"-max-parallel=2", "-output-dir", dir}, &stderr); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		"domains.txt":              "example.com\n",
		"deduplicated-domains.txt": "www.example.com\n",
		"ip.txt":                   "192.0.2.0/31\n",
	} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil || string(data) != want {
			t.Fatalf("%s: got %q, err=%v; want %q", name, data, err, want)
		}
	}

	http.DefaultTransport = testTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusForbidden,
			Status:     "403 Forbidden",
			Body:       http.NoBody,
			Header:     make(http.Header),
		}, nil
	})
	if err := run(context.Background(), []string{"-output-dir", dir}, &stderr); err == nil {
		t.Fatal("expected failed crawl to return an error")
	}
	data, err := os.ReadFile(filepath.Join(dir, "domains.txt"))
	if err != nil || string(data) != "example.com\n" {
		t.Fatal("failed crawl replaced existing output")
	}
}

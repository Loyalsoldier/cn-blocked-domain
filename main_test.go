package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Loyalsoldier/cn-blocked-domain/internal/greatfire"
)

func TestRun(t *testing.T) {
	entries := []string{
		"WWW.Google.COM", "google.com", "google.com", "bu\u0308cher.de", "İ.com",
		"1.1.1.0", "1.1.1.1", "::ffff:1.1.1.2", "1.1.1.3", "1.1.1.5",
		"2001:db8::", "2001:db8::1", "not_a_domain", "not_a_domain", "bad/domain",
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := greatfire.Page{Total: len(entries)}
		for _, entry := range entries {
			p.Items = append(p.Items, greatfire.Item{Registrable: entry})
		}
		json.NewEncoder(w).Encode(p)
	}))
	defer srv.Close()
	client := greatfire.NewClient(2)
	client.BaseURL = srv.URL
	outDir := filepath.Join(t.TempDir(), "publish")
	if err := run(context.Background(), client, outDir); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]string{
		domainsFile:      "google.com\nxn--bcher-kva.de\nxn--i-9bb.com\n",
		deduplicatedFile: "www.google.com\n",
		ipFile:           "1.1.1.0/30\n1.1.1.5/32\n2001:db8::/127\n",
		invalidFile:      "bad/domain\nnot_a_domain\n",
	} {
		got, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("%s = %q; want %q", name, got, want)
		}
	}
}

func TestRunFetchErrorPreservesFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("offset") == "0" {
			json.NewEncoder(w).Encode(greatfire.Page{
				Total: greatfire.PageSize + 1,
				Items: make([]greatfire.Item, greatfire.PageSize),
			})
			return
		}
		fmt.Fprint(w, `{"error":"temporarily unavailable"}`)
	}))
	defer srv.Close()
	client := greatfire.NewClient(2)
	client.BaseURL = srv.URL
	client.Retries = 0
	outDir := t.TempDir()
	names := []string{domainsFile, deduplicatedFile, ipFile, invalidFile}
	const contents = "previous output\n"
	for _, name := range names {
		if err := os.WriteFile(filepath.Join(outDir, name), []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := run(context.Background(), client, outDir); err == nil {
		t.Fatal("expected fetch error")
	}
	for _, name := range names {
		got, err := os.ReadFile(filepath.Join(outDir, name))
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != contents {
			t.Errorf("%s was overwritten: %q", name, got)
		}
	}
}

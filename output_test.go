package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteLists(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "output")
	result := aggregate([]string{"www.example.com", "example.com", "192.0.2.0", "192.0.2.1"})
	if err := writeLists(dir, result); err != nil {
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
	if err := writeLists(dir, lists{}); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 3 {
		t.Fatalf("unexpected output directory contents: %v, err=%v", entries, err)
	}
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || info.Size() != 0 {
			t.Fatalf("%s was not replaced with an empty file", entry.Name())
		}
	}
}

func TestWriteListsErrors(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, []byte("existing"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeLists(file, lists{}); err == nil {
		t.Fatal("expected error for non-directory output path")
	}
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "domains.txt"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := writeLists(dir, lists{domains: []string{"example.com"}}); err == nil {
		t.Fatal("expected error when target is a directory")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary files not cleaned up: %v, err=%v", entries, err)
	}
}

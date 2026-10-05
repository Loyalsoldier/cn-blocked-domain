package domain

import (
	"slices"
	"testing"
)

func TestNormalize(t *testing.T) {
	cases := map[string]string{
		"WWW.Google.COM": "www.google.com", " example.org. ": "example.org", "com": "com", "a-b.io": "a-b.io",
		"": "", "-a.com": "", "a..com": "", "1.2.3.4": "", "exa mple.com": "", "例子.com": "xn--fsqu00a.com", "Bücher.DE": "xn--bcher-kva.de", "a_b.com": "",
		"İ.com": "xn--i-9bb.com", "bu\u0308cher.de": "xn--bcher-kva.de", "例子。COM。": "xn--fsqu00a.com",
		"💩.la": "xn--ls8h.la", "xn--ls8h.la": "xn--ls8h.la", "a\u200db.com": "", "xn--.com": "",
	}
	for in, want := range cases {
		got, ok := Normalize(in)
		if ok != (want != "") || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
}

func TestDeduplicate(t *testing.T) {
	kept, removed := Deduplicate([]string{"www.google.com", "google.com", "google.com", "mail.google.com", "a.b.org", "example.net", "x.example.net", "foo.com", "com"})
	if want := []string{"a.b.org", "com", "example.net"}; !slices.Equal(kept, want) {
		t.Errorf("kept = %v, want %v", kept, want)
	}
	if want := []string{"google.com", "mail.google.com", "www.google.com", "foo.com", "x.example.net"}; !slices.Equal(removed, want) {
		t.Errorf("removed = %v, want %v", removed, want)
	}
}

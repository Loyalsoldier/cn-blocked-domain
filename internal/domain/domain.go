// Package domain validates, normalizes and deduplicates domain names.
package domain

import (
	"slices"
	"sort"
	"strings"

	"golang.org/x/net/idna"
)

// Normalize trims s, converts internationalized names to lower-cased punycode
// (e.g. "例子.com" to "xn--fsqu00a.com"), and reports whether it is a valid domain name.
func Normalize(s string) (string, bool) {
	d := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(s)), ".")
	d, err := idna.Registration.ToASCII(d)
	if err != nil {
		return "", false
	}
	if d == "" || len(d) > 253 {
		return "", false
	}
	labels := strings.Split(d, ".")
	for _, l := range labels {
		if !validLabel(l) {
			return "", false
		}
	}
	// A top-level domain must not be all-numeric (rules out IP-like strings).
	if isNumeric(labels[len(labels)-1]) {
		return "", false
	}
	return d, true
}

func validLabel(l string) bool {
	if len(l) == 0 || len(l) > 63 || l[0] == '-' || l[len(l)-1] == '-' {
		return false
	}
	for i := 0; i < len(l); i++ {
		c := l[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-') {
			return false
		}
	}
	return true
}

func isNumeric(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// Deduplicate removes duplicates and every domain whose parent domain is also
// present (e.g. "www.google.com" when "google.com" or "com" exists).
// Kept domains are sorted alphabetically; removed domains are sorted by their
// reversed string so that subdomains of the same parent are grouped together.
func Deduplicate(domains []string) (kept, removed []string) {
	set := make(map[string]struct{}, len(domains))
	for _, d := range domains {
		set[d] = struct{}{}
	}
	for d := range set {
		if hasParent(d, set) {
			removed = append(removed, d)
		} else {
			kept = append(kept, d)
		}
	}
	sort.Strings(kept)
	slices.SortFunc(removed, func(a, b string) int {
		return strings.Compare(reverse(a), reverse(b))
	})
	return kept, removed
}

func hasParent(d string, set map[string]struct{}) bool {
	for i := strings.IndexByte(d, '.'); i >= 0; i = strings.IndexByte(d, '.') {
		d = d[i+1:]
		if _, ok := set[d]; ok {
			return true
		}
	}
	return false
}

func reverse(s string) string {
	b := []byte(s)
	slices.Reverse(b)
	return string(b)
}

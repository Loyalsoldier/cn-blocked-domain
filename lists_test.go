package main

import (
	"net/netip"
	"reflect"
	"strings"
	"testing"
)

func TestValidDomain(t *testing.T) {
	for _, domain := range []string{
		"com", "example.com", "www.example.com", "xn--bcher-kva.de", "a-b.example",
		"123.example", strings.Repeat("a", 63) + ".com",
		strings.Repeat(strings.Repeat("a", 63)+".", 3) + strings.Repeat("a", 61),
	} {
		if !validDomain(domain) {
			t.Errorf("rejected valid domain %q", domain)
		}
	}
	for _, domain := range []string{
		"", ".", "EXAMPLE.COM", ".example.com", "example.com.", "a..com",
		"-a.com", "a-.com", "_srv.example.com", "*.example.com",
		"https://example.com", "example.com:443", "user@example.com",
		"example.com/path", "a b.com", "bücher.de", "999.1.2.3", "1.2.3",
		"123", "example.123", "a\x00.com", "127.01.0.1",
		strings.Repeat("a", 64) + ".com",
		strings.Repeat(strings.Repeat("a", 63)+".", 3) + strings.Repeat("a", 62),
	} {
		if validDomain(domain) {
			t.Errorf("accepted invalid domain %q", domain)
		}
	}
}

func TestAggregateDomains(t *testing.T) {
	result := aggregate([]string{
		"www.Google.com", "GOOGLE.COM", "google.com", "COM.",
		"www.example.org", " Example.ORG. ", "notexample.org",
		"badexample.org", "www.badexample.org", "a.b.www.example.org",
		"XN--BCHER-KVA.DE", "999.1.2.3", "1.2.3", "", "*.example.org", "https://example.org",
	})
	if want := []string{"badexample.org", "com", "example.org", "notexample.org", "xn--bcher-kva.de"}; !reflect.DeepEqual(result.domains, want) {
		t.Fatalf("domains=%v, want %v", result.domains, want)
	}
	if want := []string{"a.b.www.example.org", "google.com", "www.badexample.org", "www.example.org", "www.google.com"}; !reflect.DeepEqual(result.removed, want) {
		t.Fatalf("removed=%v, want %v", result.removed, want)
	}
	if result.invalid != 5 {
		t.Errorf("invalid=%d, want 5", result.invalid)
	}
}

func TestAggregateIPs(t *testing.T) {
	tests := []struct {
		name   string
		values []string
		want   []string
	}{
		{"single addresses", []string{"192.0.2.1", "2001:db8::1"}, []string{"192.0.2.1/32", "2001:db8::1/128"}},
		{"siblings", []string{"192.0.2.3", "192.0.2.1", "192.0.2.0", "192.0.2.2"}, []string{"192.0.2.0/30"}},
		{"gaps", []string{"192.0.2.0", "192.0.2.2", "192.0.2.3"}, []string{"192.0.2.0/32", "192.0.2.2/31"}},
		{"unaligned", []string{"192.0.2.1", "192.0.2.2"}, []string{"192.0.2.1/32", "192.0.2.2/32"}},
		{"mapped duplicate", []string{"::ffff:192.0.2.1", "192.0.2.1"}, []string{"192.0.2.1/32"}},
		{"IPv6 siblings", []string{"2001:DB8::1", "2001:db8::", "2001:db8::2", "2001:db8::3"}, []string{"2001:db8::/126"}},
		{"address limits", []string{"0.0.0.0", "::", "255.255.255.254", "255.255.255.255", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff"}, []string{"0.0.0.0/32", "255.255.255.254/31", "::/128", "ffff:ffff:ffff:ffff:ffff:ffff:ffff:ffff/128"}},
		{"invalid", []string{"256.1.1.1", "01.2.3.4", "[::1]", "fe80::1%eth0", "2001:db8::g", "192.0.2.1/24"}, []string{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := aggregate(test.values)
			if !reflect.DeepEqual(result.prefixes, test.want) {
				t.Fatalf("prefixes=%v, want %v", result.prefixes, test.want)
			}
			if len(result.domains) != 0 {
				t.Fatalf("IP input classified as domains: %v", result.domains)
			}
		})
	}
}

func TestCollapseAddressesExactCoverage(t *testing.T) {
	// Exhaust every subset of eight consecutive addresses, including holes and
	// unaligned ranges, for both IP families.
	for _, base := range []string{"192.0.2.0", "2001:db8::"} {
		for mask := 0; mask < 256; mask++ {
			addresses := make(map[netip.Addr]struct{})
			address := netip.MustParseAddr(base)
			for bit := 0; bit < 8; bit++ {
				if mask&(1<<bit) != 0 {
					addresses[address] = struct{}{}
				}
				address = address.Next()
			}
			covered := make(map[netip.Addr]struct{})
			var prefixes []netip.Prefix
			for _, value := range collapseAddresses(addresses) {
				prefix := netip.MustParsePrefix(value)
				prefixes = append(prefixes, prefix)
				for ip := prefix.Addr(); prefix.Contains(ip); ip = ip.Next() {
					if _, exists := addresses[ip]; !exists {
						t.Fatalf("mask %d: extra address %s in %s", mask, ip, prefix)
					}
					if _, exists := covered[ip]; exists {
						t.Fatalf("mask %d: overlapping prefix %s", mask, prefix)
					}
					covered[ip] = struct{}{}
				}
			}
			if !reflect.DeepEqual(covered, addresses) {
				t.Fatalf("mask %d: incorrect coverage", mask)
			}
			for i, left := range prefixes {
				for _, right := range prefixes[i+1:] {
					if left.Bits() == right.Bits() && netip.PrefixFrom(left.Addr(), left.Bits()-1).Masked().Contains(right.Addr()) {
						t.Fatalf("unmerged siblings: %s and %s", left, right)
					}
				}
			}
		}
	}
}

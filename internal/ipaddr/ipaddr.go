// Package ipaddr parses IP addresses and aggregates them into minimal CIDR lists.
package ipaddr

import (
	"net/netip"
	"slices"
	"strings"
)

// Parse parses an IP address or CIDR string into a canonical (masked) prefix.
// IPv4-mapped IPv6 addresses are converted to IPv4.
func Parse(s string) (netip.Prefix, bool) {
	s = strings.TrimSpace(s)
	if p, err := netip.ParsePrefix(s); err == nil {
		if p.Addr().Is4In6() && p.Bits() >= 96 {
			p = netip.PrefixFrom(p.Addr().Unmap(), p.Bits()-96)
		}
		return p.Masked(), true
	}
	a, err := netip.ParseAddr(s)
	if err != nil || a.Zone() != "" {
		return netip.Prefix{}, false
	}
	a = a.Unmap()
	return netip.PrefixFrom(a, a.BitLen()), true
}

// Aggregate returns the smallest sorted list of prefixes covering exactly the
// same addresses as the input: duplicates and covered prefixes are dropped and
// adjacent sibling prefixes are merged into their parent.
func Aggregate(prefixes []netip.Prefix) []netip.Prefix {
	// Stack-based merge over prefixes sorted by address, then by prefix length.
	sorted := append([]netip.Prefix(nil), prefixes...)
	sortPrefixes(sorted)

	var out []netip.Prefix
	for _, p := range sorted {
		if n := len(out); n > 0 && out[n-1].Overlaps(p) {
			// Sorted order guarantees the previous prefix contains p.
			continue
		}
		out = append(out, p)
		// Merge siblings as long as the top two form a complete parent.
		for len(out) >= 2 {
			a, b := out[len(out)-2], out[len(out)-1]
			parent, ok := mergeSiblings(a, b)
			if !ok {
				break
			}
			out = append(out[:len(out)-2], parent)
		}
	}
	return out
}

func mergeSiblings(a, b netip.Prefix) (netip.Prefix, bool) {
	if a.Bits() != b.Bits() || a.Bits() == 0 || a.Addr().Is4() != b.Addr().Is4() {
		return netip.Prefix{}, false
	}
	pa, _ := a.Addr().Prefix(a.Bits() - 1)
	pb, _ := b.Addr().Prefix(b.Bits() - 1)
	if pa != pb {
		return netip.Prefix{}, false
	}
	return pa, true
}

func sortPrefixes(ps []netip.Prefix) {
	slices.SortFunc(ps, func(x, y netip.Prefix) int {
		if c := x.Addr().Compare(y.Addr()); c != 0 {
			return c
		}
		return x.Bits() - y.Bits()
	})
}

package main

import (
	"net/netip"
	"sort"
	"strings"
)

type lists struct {
	domains  []string
	removed  []string
	prefixes []string
	invalid  int
}

func validDomain(domain string) bool {
	if domain == "" || len(domain) > 253 {
		return false
	}
	labels := strings.Split(domain, ".")
	for _, label := range labels {
		if label == "" || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, ch := range label {
			if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
				return false
			}
		}
	}
	// A numeric final label is not a hostname; reject malformed IPv4 addresses
	// rather than allowing them to fall through as domains.
	for _, ch := range labels[len(labels)-1] {
		if ch < '0' || ch > '9' {
			return true
		}
	}
	return false
}

func aggregate(values []string) lists {
	domains := make(map[string]struct{})
	addresses := make(map[netip.Addr]struct{})
	var result lists
	for _, value := range values {
		value = strings.TrimSpace(value)
		if address, err := netip.ParseAddr(value); err == nil && address.Zone() == "" {
			addresses[address.Unmap()] = struct{}{}
			continue
		}
		domain := strings.TrimSuffix(strings.ToLower(value), ".")
		if !validDomain(domain) {
			result.invalid++
			continue
		}
		domains[domain] = struct{}{}
	}
	for domain := range domains {
		redundant := false
		for parent := domain; strings.Contains(parent, "."); {
			parent = parent[strings.IndexByte(parent, '.')+1:]
			if _, exists := domains[parent]; exists {
				redundant = true
				break
			}
		}
		if redundant {
			result.removed = append(result.removed, domain)
		} else {
			result.domains = append(result.domains, domain)
		}
	}
	sort.Strings(result.domains)
	sort.Strings(result.removed)
	result.prefixes = collapseAddresses(addresses)
	return result
}

func collapseAddresses(addresses map[netip.Addr]struct{}) []string {
	sorted := make([]netip.Addr, 0, len(addresses))
	for address := range addresses {
		sorted = append(sorted, address)
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Less(sorted[j]) })

	// Merge only equal-size sibling networks. Every emitted address was present
	// in the input; gaps must never be filled by a larger covering network.
	stack := make([]netip.Prefix, 0, len(sorted))
	for _, address := range sorted {
		stack = append(stack, netip.PrefixFrom(address, address.BitLen()))
		for len(stack) >= 2 {
			left, right := stack[len(stack)-2], stack[len(stack)-1]
			if left.Bits() == 0 || left.Bits() != right.Bits() || left.Addr().BitLen() != right.Addr().BitLen() {
				break
			}
			parent := netip.PrefixFrom(left.Addr(), left.Bits()-1).Masked()
			if !parent.Contains(right.Addr()) {
				break
			}
			stack = append(stack[:len(stack)-2], parent)
		}
	}
	result := make([]string, 0, len(stack))
	for _, prefix := range stack {
		result = append(result, prefix.String())
	}
	return result
}

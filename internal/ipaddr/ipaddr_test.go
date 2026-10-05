package ipaddr

import (
	"net/netip"
	"testing"
)

func TestParse(t *testing.T) {
	cases := map[string]string{
		"1.2.3.4": "1.2.3.4/32", "10.0.0.5/8": "10.0.0.0/8", "::ffff:1.2.3.4": "1.2.3.4/32", "2001:db8::1": "2001:db8::1/128",
		"google.com": "", "1.2.3": "", "fe80::1%eth0": "", "": "",
	}
	for in, want := range cases {
		p, ok := Parse(in)
		if ok != (want != "") || (ok && p.String() != want) {
			t.Errorf("Parse(%q) = %v, %v; want %q", in, p, ok, want)
		}
	}
}

func TestAggregate(t *testing.T) {
	var in []netip.Prefix
	for _, s := range []string{"1.1.1.3", "1.1.1.0", "1.1.1.1", "1.1.1.2", "1.1.1.2", "8.8.8.8", "10.0.0.0/8", "10.1.2.3", "2001:db8::", "2001:db8::1", "1.1.1.5"} {
		p, _ := Parse(s)
		in = append(in, p)
	}
	got := Aggregate(in)
	want := []string{"1.1.1.0/30", "1.1.1.5/32", "8.8.8.8/32", "10.0.0.0/8", "2001:db8::/127"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i].String() != want[i] {
			t.Errorf("got %v, want %v", got, want)
		}
	}
}

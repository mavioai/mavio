package server

import (
	"net/netip"
	"slices"
	"testing"
)

func TestConnections(t *testing.T) {
	id := "abcdefghijklmnopqrstuvwxyz234567"
	addrs := []netip.Addr{netip.MustParseAddr("192.168.1.10"), netip.MustParseAddr("2001:db8::1")}
	tests := []struct {
		name      string
		certified bool
		https     int
		base      string
		want      []string
	}{
		{name: "without a certificate", https: 8920, want: []string{"http://192.168.1.10:8686", "http://[2001:db8::1]:8686"}},
		{name: "with a certificate", certified: true, https: 8920, base: "/mavio", want: []string{
			"https://192-168-1-10." + id + ".mavio.site:8920/mavio",
			"https://2001-db8--1." + id + ".mavio.site:8920/mavio",
			"http://192.168.1.10:8686/mavio",
			"http://[2001:db8::1]:8686/mavio",
		}},
		{name: "without HTTPS", certified: true, want: []string{"http://192.168.1.10:8686", "http://[2001:db8::1]:8686"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := connections(id, addrs, 8686, tc.https, tc.certified, tc.base); !slices.Equal(got, tc.want) {
				t.Errorf("connections = %v, want = %v", got, tc.want)
			}
		})
	}
}

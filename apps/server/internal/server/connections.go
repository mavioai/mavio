package server

import (
	"fmt"
	"net"
	"net/netip"
	"slices"
	"strconv"

	"github.com/mavioai/mavio/apps/server/internal/certs"
)

// connections returns the URLs clients reach the server at, best first
// (docs/mavio-dev.md §4.3): the names of its addresses under mavio.site
// over HTTPS once it has its certificate, then its addresses over HTTP,
// each with the base URL.
func connections(id string, addrs []netip.Addr, httpPort, httpsPort int, certified bool, base string) []string {
	var out []string
	if certified && httpsPort != 0 {
		for _, a := range addrs {
			out = append(out, fmt.Sprintf("https://%s:%d%s", certs.Host(id, a), httpsPort, base))
		}
	}
	if httpPort != 0 {
		for _, a := range addrs {
			out = append(out, "http://"+net.JoinHostPort(a.String(), strconv.Itoa(httpPort))+base)
		}
	}
	return out
}

// localAddresses returns the addresses of the host's interfaces that
// other devices reach: IPv4 addresses first, then global and unique local
// IPv6 addresses, without loopback and link-local ones.
func localAddresses() []netip.Addr {
	ifaces, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var out []netip.Addr
	for _, ia := range ifaces {
		prefix, err := netip.ParsePrefix(ia.String())
		if err != nil {
			continue
		}
		a := prefix.Addr().Unmap()
		if a.IsLoopback() || a.IsLinkLocalUnicast() || a.IsMulticast() || a.IsUnspecified() {
			continue
		}
		out = append(out, a)
	}
	slices.SortStableFunc(out, func(a, b netip.Addr) int {
		switch {
		case a.Is4() && !b.Is4():
			return -1
		case !a.Is4() && b.Is4():
			return 1
		}
		return 0
	})
	return slices.Compact(out)
}

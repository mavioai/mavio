package server

import (
	"net"
	"testing"
)

func TestGatewayAddress(t *testing.T) {
	for _, tc := range []struct {
		addr net.Addr
		want string
		port int
	}{
		{&net.TCPAddr{IP: net.IPv6unspecified, Port: 8686}, "127.0.0.1:8686", 8686},
		{&net.TCPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 8688}, "127.0.0.1:8688", 8688},
		{&net.TCPAddr{IP: net.IPv4(192, 168, 1, 5), Port: 8686}, "192.168.1.5:8686", 8686},
		{&net.UnixAddr{Name: "/run/mavio.sock", Net: "unix"}, "", 0},
	} {
		if got, port := gatewayAddress(tc.addr); got != tc.want || port != tc.port {
			t.Errorf("gatewayAddress(%v) got = %q, %d, want = %q, %d", tc.addr, got, port, tc.want, tc.port)
		}
	}
}

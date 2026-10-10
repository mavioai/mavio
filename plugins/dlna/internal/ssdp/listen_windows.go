package ssdp

import (
	"context"
	"net"
	"strconv"
	"syscall"
)

// listen opens a UDP socket on a port that other programs on this host,
// other UPnP stacks among them, may listen on as well.
func listen(ctx context.Context, port int) (net.PacketConn, error) {
	lc := net.ListenConfig{Control: func(_, _ string, c syscall.RawConn) error {
		var serr error
		err := c.Control(func(fd uintptr) {
			serr = syscall.SetsockoptInt(syscall.Handle(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
		})
		if err != nil {
			return err
		}
		return serr
	}}
	return lc.ListenPacket(ctx, "udp4", "0.0.0.0:"+strconv.Itoa(port))
}

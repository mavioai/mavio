//go:build !windows

package containers

import (
	"context"
	"errors"
	"net"
)

// dialPipe fails: named pipes exist only on Windows.
func dialPipe(context.Context, string) (net.Conn, error) {
	return nil, errors.ErrUnsupported
}

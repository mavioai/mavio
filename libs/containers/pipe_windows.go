package containers

import (
	"context"
	"errors"
	"net"
	"os"
	"strings"
	"syscall"
	"time"
)

// errPipeBusy is CreateFile's error when every instance of a pipe is busy.
const errPipeBusy = syscall.Errno(231)

// fileFlagOverlapped opens a handle for asynchronous I/O, which os.NewFile
// then runs through the I/O completion port, so that a pending read does
// not block writes as it would on a synchronous handle.
const fileFlagOverlapped = 0x40000000

// dialPipe opens the named pipe at the slashed path, waiting while every
// instance of it is busy.
func dialPipe(ctx context.Context, address string) (net.Conn, error) {
	name, err := syscall.UTF16PtrFromString(strings.ReplaceAll(address, "/", `\`))
	if err != nil {
		return nil, err
	}
	for {
		h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil,
			syscall.OPEN_EXISTING, fileFlagOverlapped, 0)
		if err == nil {
			return &pipeConn{File: os.NewFile(uintptr(h), address), address: address}, nil
		}
		if !errors.Is(err, errPipeBusy) {
			return nil, &os.PathError{Op: "open", Path: address, Err: err}
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(10 * time.Millisecond):
		}
	}
}

// pipeConn is a named pipe as a net.Conn.
type pipeConn struct {
	*os.File
	address string
}

// pipeAddr is a named pipe's address.
type pipeAddr string

func (a pipeAddr) Network() string { return NamedPipe }
func (a pipeAddr) String() string  { return string(a) }

func (c *pipeConn) LocalAddr() net.Addr  { return pipeAddr(c.address) }
func (c *pipeConn) RemoteAddr() net.Addr { return pipeAddr(c.address) }

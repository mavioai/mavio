package capi

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"unsafe"

	"google.golang.org/protobuf/proto"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
)

// Library is the core's shared library, loaded once for the process: its
// callback, which the C ABI calls on the core's threads, is never freed.
type Library struct {
	open  func(callback, context uintptr) uintptr
	call  func(client uintptr, call uint64, request *byte, length uintptr)
	close func(client uintptr)

	callback uintptr
	mu       sync.Mutex
	clients  map[uintptr]*Client
	next     uintptr
}

// Load loads the core's shared library at path.
func Load(path string) (*Library, error) {
	handle, err := openLibrary(path)
	if err != nil {
		return nil, fmt.Errorf("load the core %s: %w", path, err)
	}
	lib := &Library{clients: map[uintptr]*Client{}}
	register(&lib.open, handle, "mavio_open")
	register(&lib.call, handle, "mavio_call")
	register(&lib.close, handle, "mavio_close")
	lib.callback = newCallback(lib.receive)
	return lib, nil
}

// receive is the callback: a Message for the client of context, in a
// buffer valid only during the call.
func (l *Library) receive(context uintptr, message *byte, length uintptr) uintptr {
	l.mu.Lock()
	c := l.clients[context]
	l.mu.Unlock()
	if c == nil {
		return 0
	}
	m := &capiv1.Message{}
	if err := proto.Unmarshal(unsafe.Slice(message, length), m); err != nil {
		// The core writes every message; nothing else reaches here.
		panic(fmt.Sprintf("capi: the core sent an invalid message: %v", err))
	}
	c.receive(m)
	return 0
}

// Client is a client of the core, as the C ABI opens it.
type Client struct {
	lib     *Library
	handle  uintptr
	context uintptr
	events  func(*capiv1.Event)

	calls   atomic.Uint64
	mu      sync.Mutex
	pending map[uint64]chan *capiv1.Message
	closed  bool
}

// Open opens a client of the core with open's store, device and tokens;
// events receives the servers' events, on the core's threads, and must
// return promptly.
func (l *Library) Open(ctx context.Context, open *capiv1.Open, events func(*capiv1.Event)) (*Client, error) {
	l.mu.Lock()
	l.next++
	c := &Client{lib: l, context: l.next, events: events, pending: map[uint64]chan *capiv1.Message{}}
	l.clients[c.context] = c
	l.mu.Unlock()
	c.handle = l.open(l.callback, c.context)
	if _, err := c.Call(ctx, capiv1.Request_builder{Open: open}.Build()); err != nil {
		c.Close()
		return nil, err
	}
	return c, nil
}

func (c *Client) receive(m *capiv1.Message) {
	if m.GetCall() == 0 {
		if e := m.GetEvent(); e != nil && c.events != nil {
			c.events(e)
		}
		return
	}
	c.mu.Lock()
	ch := c.pending[m.GetCall()]
	delete(c.pending, m.GetCall())
	c.mu.Unlock()
	if ch != nil {
		ch <- m
	}
}

// ErrClosed is returned by calls of a closed client.
var ErrClosed = errors.New("capi: the client is closed")

// Error is a call the core answered with an error.
type Error struct {
	Code   capiv1.Error_Code
	Reason string
}

func (e *Error) Error() string {
	return fmt.Sprintf("core: %v: %s", e.Code, e.Reason)
}

// Call sends a request to the core and returns its answer; an error answer
// is returned as an *Error. A call whose ctx ends is abandoned, and its
// answer dropped.
func (c *Client) Call(ctx context.Context, request *capiv1.Request) (*capiv1.Message, error) {
	b, err := proto.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("capi: marshal the request: %w", err)
	}
	call := c.calls.Add(1)
	ch := make(chan *capiv1.Message, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	c.pending[call] = ch
	var p *byte
	if len(b) > 0 {
		p = &b[0]
	}
	// The core copies the request before it returns.
	c.lib.call(c.handle, call, p, uintptr(len(b)))
	runtime.KeepAlive(b)
	c.mu.Unlock()
	select {
	case m := <-ch:
		if e := m.GetError(); e != nil {
			return nil, &Error{Code: e.GetCode(), Reason: e.GetReason()}
		}
		return m, nil
	case <-ctx.Done():
		c.mu.Lock()
		delete(c.pending, call)
		c.mu.Unlock()
		return nil, ctx.Err()
	}
}

// Close stops the client's event streams and frees it; answers still due
// are dropped.
func (c *Client) Close() {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return
	}
	c.closed = true
	c.lib.close(c.handle)
	c.mu.Unlock()
	c.lib.mu.Lock()
	delete(c.lib.clients, c.context)
	c.lib.mu.Unlock()
}

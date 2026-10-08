//go:build wasip1

// Package wasm is the guest entry point for WebAssembly plugins. Importing it
// exports the module ABI and routes outbound HTTP through the host. Build
// plugins as reactors:
//
//	GOOS=wasip1 GOARCH=wasm go build -buildmode=c-shared -o plugin.wasm .
package wasm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"unsafe"

	"github.com/mavioai/mavio/libs/plugin/guest"
	"github.com/mavioai/mavio/libs/plugin/internal/abi"
)

func init() {
	guest.SetHTTPClient(&http.Client{Transport: fetchTransport{}})
}

// pinned keeps buffers shared with the host alive until freed.
var (
	pinMu  sync.Mutex
	pinned = map[uint32][]byte{}
)

func pin(b []byte) uint32 {
	if len(b) == 0 {
		b = make([]byte, 1)
	}
	p := uint32(uintptr(unsafe.Pointer(unsafe.SliceData(b))))
	pinMu.Lock()
	pinned[p] = b
	pinMu.Unlock()
	return p
}

func pinnedBytes(p, n uint32) []byte {
	pinMu.Lock()
	defer pinMu.Unlock()
	return pinned[p][:n]
}

//go:wasmexport mavio_alloc
func alloc(n uint32) uint32 { return pin(make([]byte, n)) }

//go:wasmexport mavio_free
func free(p uint32) {
	pinMu.Lock()
	delete(pinned, p)
	pinMu.Unlock()
}

// call serves one encoded request and returns the encoded response as
// (pointer << 32 | length). The host frees the response buffer.
//
//go:wasmexport mavio_call
func call(p, n uint32) uint64 {
	out := abi.EncodeResponse(serve(pinnedBytes(p, n)))
	return uint64(pin(out))<<32 | uint64(len(out))
}

func serve(in []byte) abi.Response {
	req, err := abi.DecodeRequest(in)
	if err != nil {
		return abi.Response{Status: http.StatusBadRequest, Body: []byte(err.Error())}
	}
	hreq, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://plugin"+req.Path, bytes.NewReader(req.Body))
	if err != nil {
		return abi.Response{Status: http.StatusBadRequest, Body: []byte(err.Error())}
	}
	hreq.Header = req.Header
	w := &recorder{header: http.Header{}, status: http.StatusOK}
	guest.Handler().ServeHTTP(w, hreq)
	return abi.Response{Status: w.status, Header: w.header, Body: w.body.Bytes()}
}

// recorder is a minimal http.ResponseWriter.
type recorder struct {
	header http.Header
	status int
	body   bytes.Buffer
	wrote  bool
}

func (r *recorder) Header() http.Header { return r.header }

func (r *recorder) WriteHeader(status int) {
	if !r.wrote {
		r.status, r.wrote = status, true
	}
}

func (r *recorder) Write(b []byte) (int, error) {
	r.WriteHeader(http.StatusOK)
	return r.body.Write(b)
}

func (r *recorder) Flush() {}

//go:wasmimport mavio http_fetch
func hostFetch(p, n uint32) uint64

//go:wasmimport mavio http_read
func hostRead(handle, p uint32)

// fetchTransport performs requests through the host's http_fetch, which
// enforces the manifest's host allowlist.
type fetchTransport struct{}

func (fetchTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body []byte
	if req.Body != nil {
		b, err := io.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			return nil, err
		}
		body = b
	}
	in, err := json.Marshal(abi.FetchRequest{Method: req.Method, URL: req.URL.String(), Header: req.Header, Body: body})
	if err != nil {
		return nil, err
	}
	p := pin(in)
	defer free(p)
	res := hostFetch(p, uint32(len(in)))
	handle, n := uint32(res>>32), uint32(res)

	buf := make([]byte, max(n, 1))
	bp := pin(buf)
	defer free(bp)
	hostRead(handle, bp)

	var out abi.FetchResponse
	if err := json.Unmarshal(buf[:n], &out); err != nil {
		return nil, fmt.Errorf("http_fetch: %w", err)
	}
	if out.Error != "" {
		return nil, fmt.Errorf("http_fetch: %s", out.Error)
	}
	return &http.Response{
		Status:        fmt.Sprintf("%d %s", out.Status, http.StatusText(out.Status)),
		StatusCode:    out.Status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        out.Header,
		Body:          io.NopCloser(bytes.NewReader(out.Body)),
		ContentLength: int64(len(out.Body)),
		Request:       req,
	}, nil
}

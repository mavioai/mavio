package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/client-rust/capi"
	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
)

// coreTransport sends the unary calls of libs/proto's Connect clients
// through the core, which calls the server as the session signed in to
// it: the screens the core does not model, such as a person or the
// settings, use the server's API as the web UI does. A client's base URL
// names the server, as serverURL makes it.
type coreTransport struct {
	core  Core
	bases *bases
}

// bases holds the base URL the core last reached each server at, which
// the paths of the server's responses are relative to.
type bases struct {
	mu   sync.Mutex
	urls map[string]string
}

func (b *bases) set(id, url string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.urls == nil {
		b.urls = map[string]string{}
	}
	b.urls[id] = url
}

// url resolves a path of a response of the server with id, such as an
// image's signed URL, empty for an empty path.
func (b *bases) url(id, p string) string {
	if p == "" || strings.Contains(p, "://") {
		return p
	}
	b.mu.Lock()
	base := b.urls[id]
	b.mu.Unlock()
	return strings.TrimSuffix(base, "/") + "/" + strings.TrimPrefix(p, "/")
}

// serverURL is the base URL of the Connect clients of the server with id.
func serverURL(id string) string { return "http://" + id + ".server" }

// connectHTTP returns the HTTP client of the Connect clients, through the
// core.
func (a *App) connectHTTP() connect.HTTPClient {
	return &http.Client{Transport: coreTransport{a.opts.Core, &a.bases}}
}

// serverURLOf resolves a path of a response of a server, such as an
// image's signed URL.
func (a *App) serverURLOf(server, path string) string { return a.bases.url(server, path) }

func (t coreTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	id, ok := strings.CutSuffix(r.URL.Hostname(), ".server")
	if !ok {
		return nil, fmt.Errorf("core transport: %s names no server", r.URL)
	}
	if r.Method != http.MethodPost {
		return nil, fmt.Errorf("core transport: %s %s: only unary POST calls go through the core", r.Method, r.URL.Path)
	}
	var body []byte
	if r.Body != nil {
		var err error
		body, err = io.ReadAll(r.Body)
		r.Body.Close()
		if err != nil {
			return nil, err
		}
	}
	m, err := t.core.Call(r.Context(), capiv1.Request_builder{CallServer: capiv1.CallServer_builder{
		ServerId: id, Procedure: r.URL.Path, Request: body,
	}.Build()}.Build())
	if err != nil {
		var e *capi.Error
		if !errors.As(err, &e) {
			return nil, err
		}
		return connectError(r, e), nil
	}
	res := m.GetResponse()
	t.bases.set(id, res.GetBaseUrl())
	return response(r, http.StatusOK, "application/proto", res.GetBody()), nil
}

// connectError answers as the server would have failed: with Connect's
// JSON error of the core's code.
func connectError(r *http.Request, e *capi.Error) *http.Response {
	code, status := connect.CodeUnknown, http.StatusInternalServerError
	switch e.Code {
	case capiv1.Error_CODE_UNREACHABLE:
		code, status = connect.CodeUnavailable, http.StatusServiceUnavailable
	case capiv1.Error_CODE_UNAUTHENTICATED:
		code, status = connect.CodeUnauthenticated, http.StatusUnauthorized
	case capiv1.Error_CODE_PERMISSION_DENIED:
		code, status = connect.CodePermissionDenied, http.StatusForbidden
	case capiv1.Error_CODE_NOT_FOUND:
		code, status = connect.CodeNotFound, http.StatusNotFound
	case capiv1.Error_CODE_INVALID:
		code, status = connect.CodeInvalidArgument, http.StatusBadRequest
	case capiv1.Error_CODE_FAILED_PRECONDITION:
		code, status = connect.CodeFailedPrecondition, http.StatusBadRequest
	}
	b, _ := json.Marshal(map[string]string{"code": code.String(), "message": e.Reason})
	return response(r, status, "application/json", b)
}

func response(r *http.Request, status int, contentType string, body []byte) *http.Response {
	return &http.Response{
		Status:        http.StatusText(status),
		StatusCode:    status,
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		Header:        http.Header{"Content-Type": {contentType}},
		Body:          io.NopCloser(bytes.NewReader(body)),
		ContentLength: int64(len(body)),
		Request:       r,
	}
}

// coreError returns the core's error of err, mapping back what a Connect
// client of coreTransport failed with; nil for other errors.
func coreError(err error) *capi.Error {
	var e *capi.Error
	if errors.As(err, &e) {
		return e
	}
	var ce *connect.Error
	if !errors.As(err, &ce) {
		return nil
	}
	code := capiv1.Error_CODE_SERVER
	switch ce.Code() {
	case connect.CodeUnavailable:
		code = capiv1.Error_CODE_UNREACHABLE
	case connect.CodeUnauthenticated:
		code = capiv1.Error_CODE_UNAUTHENTICATED
	case connect.CodePermissionDenied:
		code = capiv1.Error_CODE_PERMISSION_DENIED
	case connect.CodeNotFound:
		code = capiv1.Error_CODE_NOT_FOUND
	case connect.CodeInvalidArgument:
		code = capiv1.Error_CODE_INVALID
	case connect.CodeFailedPrecondition:
		code = capiv1.Error_CODE_FAILED_PRECONDITION
	}
	return &capi.Error{Code: code, Reason: ce.Message()}
}

package rclone

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ErrChanged reports a file that changed on its remote while it was read.
var ErrChanged = errors.New("file changed on the remote")

// version is the version of an object that its reads ask for: the ETag
// of the first response --rc-serve gave.
type version struct {
	mu   sync.Mutex
	etag string
}

// ask makes req ask for the pinned version, if any.
func (v *version) ask(req *http.Request) {
	v.mu.Lock()
	etag := v.etag
	v.mu.Unlock()
	if etag != "" {
		req.Header.Set("If-Match", etag)
	}
}

// check pins the version of a successful response, and fails when the
// response's object has another size than size, the size Stat gave; a
// negative size checks none.
func (v *version) check(resp *http.Response, size int64) error {
	if total, ok := totalSize(resp); ok && size >= 0 && total != size {
		return ErrChanged
	}
	if etag := resp.Header.Get("ETag"); etag != "" {
		v.mu.Lock()
		if v.etag == "" {
			v.etag = etag
		}
		v.mu.Unlock()
	}
	return nil
}

// totalSize returns the size of the object a response serves.
func totalSize(resp *http.Response) (int64, bool) {
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		_, total, ok := strings.Cut(cr, "/")
		if !ok || total == "*" {
			return 0, false
		}
		n, err := strconv.ParseInt(total, 10, 64)
		return n, err == nil
	}
	if resp.StatusCode == http.StatusOK && resp.ContentLength >= 0 {
		return resp.ContentLength, true
	}
	return 0, false
}

// serveError is the error of a failed --rc-serve response, which keeps
// how long the rcd asks to wait.
func serveError(resp *http.Response) *Error {
	e := &Error{Method: "serve", Status: resp.StatusCode, Message: resp.Status}
	if s, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && s > 0 {
		e.RetryAfter = time.Duration(s) * time.Second
	}
	return e
}

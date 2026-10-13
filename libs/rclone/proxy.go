package rclone

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Proxy serves the objects of an rcd's remotes to ffmpeg and ffprobe on a
// loopback address. Each URL carries a random token naming one object, so
// the rcd's credentials never appear in a command line, and the proxy adds
// them to the requests it forwards, ranges included.
type Proxy struct {
	c   *Client
	log *slog.Logger
	// ttl is how long a token lives after its last use.
	ttl time.Duration
	now func() time.Time

	base string
	srv  *http.Server

	mu     sync.Mutex
	tokens map[string]*target
	swept  time.Time
}

// target is the object a token names.
type target struct {
	m    Media
	used time.Time
	// ver is the version of the object the token reads from the rcd.
	ver version
}

// proxyTTL is how long a token lives after its last use: a paused
// playback picks up again within it.
const proxyTTL = 12 * time.Hour

// NewProxy returns a proxy for the rcd c calls; Listen starts it.
func NewProxy(c *Client, log *slog.Logger) *Proxy {
	if log == nil {
		log = slog.Default()
	}
	return &Proxy{c: c, log: log, ttl: proxyTTL, now: time.Now, tokens: map[string]*target{}}
}

// Listen serves the proxy on a loopback port until Close.
func (p *Proxy) Listen() error {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	p.base = "http://" + l.Addr().String()
	p.srv = &http.Server{Handler: p, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := p.srv.Serve(l); err != nil && !errors.Is(err, http.ErrServerClosed) {
			p.log.Error("rclone media proxy stopped", "err", err)
		}
	}()
	return nil
}

// Close stops the proxy.
func (p *Proxy) Close() error {
	if p.srv == nil {
		return nil
	}
	return p.srv.Close()
}

// URL returns a URL of the object m. A probe of an object read through
// the block cache has its head and tail fetched at once.
func (p *Proxy) URL(_ context.Context, m Media) (string, error) {
	if p.base == "" {
		return "", errors.New("rclone media proxy is not listening")
	}
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	token := hex.EncodeToString(b)
	now := p.now()
	p.mu.Lock()
	if now.Sub(p.swept) > time.Hour {
		for k, t := range p.tokens {
			if now.Sub(t.used) > p.ttl {
				delete(p.tokens, k)
			}
		}
		p.swept = now
	}
	p.tokens[token] = &target{m: m, used: now}
	p.mu.Unlock()
	if m.blocks != nil && m.Intent.Kind == core.ReadProbe {
		if it, err := m.blocks.open(m.key, p.c.objectURL(m.FsName, m.Remote), m.size, m.seq); err == nil {
			it.prefetch(nil, 0, probeHead)
			it.prefetch(nil, m.size-probeTail, m.size)
			it.release()
		}
	}
	// The name ends the URL, so that ffmpeg sees the file's extension.
	return p.base + "/" + token + "/" + url.PathEscape(path.Base(m.Remote)), nil
}

// ServeHTTP implements http.Handler.
func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	token, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/"), "/")
	now := p.now()
	p.mu.Lock()
	t, ok := p.tokens[token]
	if ok && now.Sub(t.used) > p.ttl {
		delete(p.tokens, token)
		ok = false
	}
	var m Media
	if ok {
		t.used = now
		m = t.m
	}
	p.mu.Unlock()
	if !ok {
		http.NotFound(w, r)
		return
	}
	fsName, remote := m.FsName, m.Remote
	if m.blocks != nil && p.serveCached(w, r, m) {
		return
	}
	req, err := p.c.request(r.Context(), r.Method, p.c.objectURL(fsName, remote), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, h := range []string{"Range", "If-Range", "If-Modified-Since", "If-None-Match"} {
		if v := r.Header.Get(h); v != "" {
			req.Header.Set(h, v)
		}
	}
	// Every request of a token reads the version its first one did
	t.ver.ask(req)
	resp, err := p.c.http.Do(req)
	if err != nil {
		if r.Context().Err() == nil {
			p.log.WarnContext(r.Context(), "rclone media proxy request failed", "fs", fsName, "remote", remote, "err", err)
		}
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusPartialContent {
		_ = t.ver.check(resp, -1)
	}
	for _, h := range []string{"Content-Type", "Content-Length", "Content-Range", "Accept-Ranges", "Last-Modified", "ETag"} {
		if v := resp.Header.Get(h); v != "" {
			w.Header().Set(h, v)
		}
	}
	w.WriteHeader(resp.StatusCode)
	if r.Method == http.MethodHead {
		return
	}
	if _, err := io.Copy(w, resp.Body); err != nil && r.Context().Err() == nil && !errors.Is(err, context.Canceled) {
		p.log.DebugContext(r.Context(), "rclone media proxy copy ended", "remote", remote, "err", err)
	}
}

// serveCached serves m from the block cache, reading ahead as its intent
// asks; false when its item does not open, for the rcd to serve it.
func (p *Proxy) serveCached(w http.ResponseWriter, r *http.Request, m Media) bool {
	it, err := m.blocks.open(m.key, p.c.objectURL(m.FsName, m.Remote), m.size, m.seq)
	if err != nil {
		p.log.WarnContext(r.Context(), "read past the block cache", "fs", m.FsName, "remote", m.Remote, "err", err)
		return false
	}
	defer it.release()
	ctype := mime.TypeByExtension(path.Ext(m.Remote))
	if ctype == "" {
		ctype = "application/octet-stream"
	}
	// ServeContent sniffs no type, which would read the head.
	w.Header().Set("Content-Type", ctype)
	cur := it.attach(0)
	defer it.detach(cur)
	rd := itemReader{ctx: r.Context(), it: it, cur: cur, ahead: aheadOf(m.Intent)}
	http.ServeContent(w, r, "", time.Time{}, io.NewSectionReader(rd, 0, m.size))
	return true
}

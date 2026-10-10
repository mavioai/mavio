package upnp

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Publisher keeps the GENA subscriptions to a device's services and sends
// them events.
type Publisher struct {
	// Client sends the events.
	Client *http.Client
	// MaxTimeout caps the subscriptions' durations; 0 means 30 minutes.
	MaxTimeout time.Duration
	Logger     *slog.Logger

	mu   sync.Mutex
	subs map[string]*subscription
	now  func() time.Time
}

// initialDelay is how long after a subscription its initial event goes.
const initialDelay = 100 * time.Millisecond

type subscription struct {
	service   string
	callbacks []string
	expires   time.Time
	// mu orders the subscription's events, numbered by seq.
	mu  sync.Mutex
	seq uint32
}

func (p *Publisher) maxTimeout() time.Duration {
	if p.MaxTimeout <= 0 {
		return 30 * time.Minute
	}
	return p.MaxTimeout
}

func (p *Publisher) clock() time.Time {
	if p.now != nil {
		return p.now()
	}
	return time.Now()
}

func (p *Publisher) logger() *slog.Logger {
	if p.Logger == nil {
		return slog.New(slog.DiscardHandler)
	}
	return p.Logger
}

// Handler serves SUBSCRIBE and UNSUBSCRIBE for a service; state returns
// the service's evented variables, sent to each new subscriber at once.
// Callbacks must point at the subscriber's own address.
func (p *Publisher) Handler(service string, state func() []Arg) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case "SUBSCRIBE":
			p.subscribe(w, r, service, state)
		case "UNSUBSCRIBE":
			p.unsubscribe(w, r)
		default:
			w.Header().Set("Allow", "SUBSCRIBE, UNSUBSCRIBE")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	}
}

func (p *Publisher) subscribe(w http.ResponseWriter, r *http.Request, service string, state func() []Arg) {
	sid, callback, nt := r.Header.Get("Sid"), r.Header.Get("Callback"), r.Header.Get("Nt")
	timeout := p.parseTimeout(r.Header.Get("Timeout"))
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expire()
	if sid != "" {
		// A renewal names the subscription and nothing else.
		if callback != "" || nt != "" {
			http.Error(w, "renewal with CALLBACK or NT", http.StatusBadRequest)
			return
		}
		s, ok := p.subs[sid]
		if !ok || s.service != service {
			http.Error(w, "no such subscription", http.StatusPreconditionFailed)
			return
		}
		s.expires = p.clock().Add(timeout)
		writeSubscribed(w, sid, timeout)
		return
	}
	if nt != "upnp:event" {
		http.Error(w, "NT must be upnp:event", http.StatusPreconditionFailed)
		return
	}
	callbacks := parseCallbacks(callback, requester(r))
	if len(callbacks) == 0 {
		http.Error(w, "no CALLBACK at the subscriber's address", http.StatusPreconditionFailed)
		return
	}
	if p.subs == nil {
		p.subs = map[string]*subscription{}
	}
	sid = "uuid:" + rand.Text()
	s := &subscription{service: service, callbacks: callbacks, expires: p.clock().Add(timeout)}
	p.subs[sid] = s
	writeSubscribed(w, sid, timeout)
	// The initial event follows the response, which subscribers wait for
	// to learn the SID.
	vars := state()
	ctx := context.WithoutCancel(r.Context())
	time.AfterFunc(initialDelay, func() { p.send(ctx, sid, s, vars) })
}

func (p *Publisher) unsubscribe(w http.ResponseWriter, r *http.Request) {
	sid := r.Header.Get("Sid")
	if r.Header.Get("Callback") != "" || r.Header.Get("Nt") != "" {
		http.Error(w, "UNSUBSCRIBE with CALLBACK or NT", http.StatusBadRequest)
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.subs[sid]; !ok {
		http.Error(w, "no such subscription", http.StatusPreconditionFailed)
		return
	}
	delete(p.subs, sid)
}

func writeSubscribed(w http.ResponseWriter, sid string, timeout time.Duration) {
	w.Header().Set("Sid", sid)
	w.Header().Set("Timeout", "Second-"+strconv.Itoa(int(timeout/time.Second)))
	w.Header().Set("Server", "")
	w.Header().Set("Content-Length", "0")
	w.WriteHeader(http.StatusOK)
}

// parseTimeout reads a TIMEOUT header, "Second-N" or "Second-infinite",
// capped by MaxTimeout.
func (p *Publisher) parseTimeout(h string) time.Duration {
	n, err := strconv.Atoi(strings.TrimPrefix(strings.TrimSpace(h), "Second-"))
	if err != nil || n <= 0 || time.Duration(n)*time.Second > p.maxTimeout() {
		return p.maxTimeout()
	}
	return time.Duration(n) * time.Second
}

// requester returns the address a request came from: the client the
// server's proxy names first in X-Forwarded-For, else the peer.
func requester(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		first, _, _ := strings.Cut(f, ",")
		return strings.TrimSpace(first)
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// parseCallbacks reads a CALLBACK header, "<url><url>…", keeping the HTTP
// URLs at the subscriber's address, so that events go nowhere else.
func parseCallbacks(h, from string) []string {
	var out []string
	for part := range strings.SplitSeq(h, "<") {
		raw, _, ok := strings.Cut(part, ">")
		if !ok {
			continue
		}
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil || u.Scheme != "http" {
			continue
		}
		ip, want := net.ParseIP(u.Hostname()), net.ParseIP(from)
		if ip == nil || want == nil || !ip.Equal(want) {
			continue
		}
		out = append(out, u.String())
	}
	return out
}

// expire drops the subscriptions past their timeout; p.mu is held.
func (p *Publisher) expire() {
	now := p.clock()
	for sid, s := range p.subs {
		if now.After(s.expires) {
			delete(p.subs, sid)
		}
	}
}

// Notify sends evented variables of a service to its subscribers.
func (p *Publisher) Notify(ctx context.Context, service string, vars []Arg) {
	p.mu.Lock()
	p.expire()
	targets := map[string]*subscription{}
	for sid, s := range p.subs {
		if s.service == service {
			targets[sid] = s
		}
	}
	p.mu.Unlock()
	var wg sync.WaitGroup
	for sid, s := range targets {
		wg.Go(func() { p.send(ctx, sid, s, vars) })
	}
	wg.Wait()
}

// Subscribers counts a service's subscriptions.
func (p *Publisher) Subscribers(service string) int {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.expire()
	n := 0
	for _, s := range p.subs {
		if s.service == service {
			n++
		}
	}
	return n
}

// send delivers an event to the first callback that takes it.
func (p *Publisher) send(ctx context.Context, sid string, s *subscription, vars []Arg) {
	s.mu.Lock()
	defer s.mu.Unlock()
	seq := s.seq
	// SEQ wraps to 1: 0 is the initial event's.
	if s.seq++; s.seq == 0 {
		s.seq = 1
	}
	body := PropertySet(vars)
	for _, cb := range s.callbacks {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		err := p.post(ctx, cb, sid, seq, body)
		cancel()
		if err == nil {
			return
		}
		p.logger().DebugContext(ctx, "event delivery failed", "callback", cb, "err", err)
	}
}

func (p *Publisher) post(ctx context.Context, callback, sid string, seq uint32, body []byte) error {
	req, err := http.NewRequestWithContext(ctx, "NOTIFY", callback, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("Nt", "upnp:event")
	req.Header.Set("Nts", "upnp:propchange")
	req.Header.Set("Sid", sid)
	req.Header.Set("Seq", strconv.FormatUint(uint64(seq), 10))
	c := p.Client
	if c == nil {
		c = http.DefaultClient
	}
	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("NOTIFY: %s", resp.Status)
	}
	return nil
}

// PropertySet encodes evented variables as a GENA property set.
func PropertySet(vars []Arg) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n" + `<e:propertyset xmlns:e="urn:schemas-upnp-org:event-1-0">`)
	for _, v := range vars {
		fmt.Fprintf(&b, "<e:property><%s>%s</%s></e:property>", v.Name, escape(v.Value), v.Name)
	}
	b.WriteString(`</e:propertyset>`)
	return b.Bytes()
}

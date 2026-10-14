package relay

import (
	"bufio"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// The protocol between servers and the relay (docs/mavio-dev.md §5).
const (
	// ConnectPath is where a server opens its control connection, and
	// AcceptPath where it opens a connection back for a client.
	ConnectPath = "/servers/connect"
	AcceptPath  = "/servers/accept"
	// Protocol is the Upgrade token of both.
	Protocol = "mavio-relay"
	// IdentityHeader carries the server's Ed25519 public key in base64url,
	// whose hash is its ID, and ConnectionHeader the connection's ID in
	// hex on AcceptPath.
	IdentityHeader   = "Mavio-Identity-Key"
	ConnectionHeader = "Mavio-Relay-Connection"
	// FrameSize is the size of a frame on the control connection: a
	// connection's ID, or zeros as a heartbeat.
	FrameSize = 16
)

// Defaults of the Relay.
const (
	// HeartbeatInterval is how often the relay writes a heartbeat on
	// control connections.
	HeartbeatInterval = 30 * time.Second
	// AcceptTimeout bounds reading a client's ClientHello and the server
	// opening its connection back.
	AcceptTimeout = 10 * time.Second
)

// Relay serves one region.
type Relay struct {
	// Region is the relay's region, Zone the domain of servers' names,
	// such as mavio.site.
	Region, Zone string
	// URL is the base URL servers reach the relay at, the audience of
	// their tokens.
	URL string
	// Secret is the ticket secret shared with mavio.dev.
	Secret []byte
	Logger *slog.Logger
	// Now returns the time; nil uses time.Now.
	Now func() time.Time
	// HeartbeatInterval and AcceptTimeout override the defaults when not
	// zero.
	HeartbeatInterval, AcceptTimeout time.Duration

	mu       sync.Mutex
	sessions map[string]*session // by server ID
	pending  map[string]pending  // by connection ID
}

// pending is a client's connection waiting for its server to connect
// back.
type pending struct {
	server string
	back   chan net.Conn
}

// session is a server's control connection.
type session struct {
	id   string
	mu   sync.Mutex // serializes frames
	conn net.Conn
}

func (s *session) send(frame []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_ = s.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err := s.conn.Write(frame)
	return err
}

// ServeClients splices the clients' connections accepted on ln until it
// fails.
func (r *Relay) ServeClients(ctx context.Context, ln net.Listener) error {
	r.init()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go r.client(ctx, conn)
	}
}

// client splices one client's connection to its server.
func (r *Relay) client(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	timeout := or(r.AcceptTimeout, AcceptTimeout)
	_ = conn.SetDeadline(time.Now().Add(timeout))
	name, hello, err := clientHello(conn)
	if err != nil {
		r.logger().DebugContext(ctx, "no ClientHello", "remote", conn.RemoteAddr(), "err", err)
		return
	}
	region, ticket, id, ok := ParseName(name, r.Zone)
	if !ok || region != r.Region || !CheckTicket(r.Secret, ticket, id, region, r.now()) {
		r.logger().DebugContext(ctx, "refused a client", "name", name)
		return
	}
	r.mu.Lock()
	s := r.sessions[id]
	r.mu.Unlock()
	if s == nil {
		r.logger().DebugContext(ctx, "server not connected", "server", id)
		return
	}
	cid := make([]byte, FrameSize)
	_, _ = rand.Read(cid)
	key := hex.EncodeToString(cid)
	back := make(chan net.Conn, 1)
	r.mu.Lock()
	r.pending[key] = pending{server: id, back: back}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.pending, key)
		r.mu.Unlock()
	}()
	if err := s.send(cid); err != nil {
		_ = s.conn.Close()
		return
	}
	var server net.Conn
	select {
	case server = <-back:
	case <-time.After(timeout):
		r.logger().DebugContext(ctx, "the server did not connect back", "server", id)
		return
	case <-ctx.Done():
		return
	}
	defer server.Close()
	_ = conn.SetDeadline(time.Time{})
	if _, err := server.Write(hello); err != nil {
		return
	}
	splice(conn, server)
}

// Handler serves servers' control connections and connections back.
func (r *Relay) Handler() http.Handler {
	r.init()
	mux := http.NewServeMux()
	mux.HandleFunc("GET "+ConnectPath, r.connect)
	mux.HandleFunc("GET "+AcceptPath, r.accept)
	return mux
}

func (r *Relay) connect(w http.ResponseWriter, req *http.Request) {
	id, err := r.authenticate(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	conn, err := upgrade(w, req)
	if err != nil {
		return
	}
	s := &session{id: id, conn: conn}
	r.mu.Lock()
	if old := r.sessions[id]; old != nil {
		_ = old.conn.Close()
	}
	r.sessions[id] = s
	r.mu.Unlock()
	r.logger().InfoContext(req.Context(), "server connected", "server", id)
	defer func() {
		r.mu.Lock()
		if r.sessions[id] == s {
			delete(r.sessions, id)
		}
		r.mu.Unlock()
		_ = conn.Close()
	}()
	// The server sends nothing; reading notices when it goes away.
	closed := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, conn)
		close(closed)
	}()
	ticker := time.NewTicker(or(r.HeartbeatInterval, HeartbeatInterval))
	defer ticker.Stop()
	for {
		select {
		case <-closed:
			return
		case <-ticker.C:
			if err := s.send(make([]byte, FrameSize)); err != nil {
				return
			}
		}
	}
}

func (r *Relay) accept(w http.ResponseWriter, req *http.Request) {
	id, err := r.authenticate(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	key := strings.ToLower(req.Header.Get(ConnectionHeader))
	r.mu.Lock()
	p, ok := r.pending[key]
	r.mu.Unlock()
	// A connection is accepted only by the server it was announced to.
	if !ok || p.server != id {
		http.Error(w, "no such connection", http.StatusNotFound)
		return
	}
	conn, err := upgrade(w, req)
	if err != nil {
		return
	}
	select {
	case p.back <- conn:
	default:
		_ = conn.Close()
	}
}

// upgrade switches a request's connection to the relay's protocol.
func upgrade(w http.ResponseWriter, req *http.Request) (net.Conn, error) {
	if !strings.EqualFold(req.Header.Get("Upgrade"), Protocol) {
		http.Error(w, "expected an upgrade to "+Protocol, http.StatusUpgradeRequired)
		return nil, errors.New("not an upgrade")
	}
	conn, rw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		http.Error(w, "cannot upgrade", http.StatusInternalServerError)
		return nil, err
	}
	_ = conn.SetDeadline(time.Time{})
	if _, err := rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: " + Protocol + "\r\n\r\n"); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if err := rw.Flush(); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return &bufferedConn{Conn: conn, r: rw.Reader}, nil
}

// bufferedConn reads what the HTTP server buffered first.
type bufferedConn struct {
	net.Conn
	r *bufio.Reader
}

func (c *bufferedConn) Read(p []byte) (int, error) { return c.r.Read(p) }

// CloseWrite half-closes the underlying connection.
func (c *bufferedConn) CloseWrite() error {
	if cw, ok := c.Conn.(interface{ CloseWrite() error }); ok {
		return cw.CloseWrite()
	}
	return c.Close()
}

// authenticate checks the server's token, an EdDSA JWT signed with the
// identity key the request carries, for the relay's URL; it returns the
// server's ID.
func (r *Relay) authenticate(req *http.Request) (string, error) {
	pub, err := base64.RawURLEncoding.DecodeString(req.Header.Get(IdentityHeader))
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return "", errors.New("missing identity key")
	}
	id := ServerID(pub)
	token, ok := strings.CutPrefix(req.Header.Get("Authorization"), "Bearer ")
	if !ok {
		return "", errors.New("missing token")
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("malformed token")
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(pub, []byte(parts[0]+"."+parts[1]), sig) {
		return "", errors.New("bad signature")
	}
	var header struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
	}
	var claims struct {
		Iss string `json:"iss"`
		Sub string `json:"sub"`
		Aud string `json:"aud"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
	}
	if decodeJSON(parts[0], &header) != nil || decodeJSON(parts[1], &claims) != nil {
		return "", errors.New("malformed token")
	}
	now := r.now().Unix()
	switch {
	case header.Alg != "EdDSA" || header.Kid != id || claims.Iss != id || claims.Sub != id:
		return "", errors.New("token of another server")
	case claims.Aud != r.URL:
		return "", fmt.Errorf("token for %q, not this relay", claims.Aud)
	case claims.Exp < now || claims.Exp-claims.Iat > 60 || claims.Iat > now+60:
		return "", errors.New("expired token")
	}
	return id, nil
}

// ServerID is the ID of the server with an identity public key: the
// first 20 bytes of its SHA-256 in lowercase base32.
func ServerID(pub []byte) string {
	sum := sha256.Sum256(pub)
	return encoding.EncodeToString(sum[:20])
}

func decodeJSON(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

var errHello = errors.New("read the ClientHello")

// clientHello reads a connection's TLS ClientHello without answering it,
// returning the server name and the bytes read.
func clientHello(conn net.Conn) (string, []byte, error) {
	var buf bytes.Buffer
	var name string
	err := tls.Server(readOnly{Conn: conn, r: io.TeeReader(conn, &buf)}, &tls.Config{
		GetConfigForClient: func(h *tls.ClientHelloInfo) (*tls.Config, error) {
			name = h.ServerName
			return nil, errHello
		},
	}).Handshake()
	if name == "" {
		if err == nil || errors.Is(err, errHello) {
			err = errors.New("no server name")
		}
		return "", nil, err
	}
	return name, buf.Bytes(), nil
}

// readOnly reads through r and writes nothing, so that the handshake's
// alert never reaches the client.
type readOnly struct {
	net.Conn
	r io.Reader
}

func (c readOnly) Read(p []byte) (int, error)  { return c.r.Read(p) }
func (c readOnly) Write(p []byte) (int, error) { return len(p), nil }

// splice copies between two connections until both directions end.
func splice(a, b net.Conn) {
	var wg sync.WaitGroup
	cp := func(dst, src net.Conn) {
		defer wg.Done()
		_, _ = io.Copy(dst, src)
		if cw, ok := dst.(interface{ CloseWrite() error }); ok {
			_ = cw.CloseWrite()
		} else {
			_ = dst.Close()
		}
	}
	wg.Add(2)
	go cp(a, b)
	go cp(b, a)
	wg.Wait()
}

func (r *Relay) init() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.sessions == nil {
		r.sessions, r.pending = map[string]*session{}, map[string]pending{}
	}
}

// New returns a relay of a region.
func New(region, zone, url string, secret []byte, logger *slog.Logger) *Relay {
	r := &Relay{Region: region, Zone: zone, URL: url, Secret: secret, Logger: logger}
	r.init()
	return r
}

func (r *Relay) logger() *slog.Logger {
	if r.Logger == nil {
		return slog.Default()
	}
	return r.Logger
}

func (r *Relay) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func or(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

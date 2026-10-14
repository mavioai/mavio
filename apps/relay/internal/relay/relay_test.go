package relay

import (
	"bufio"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const relayURL = "https://eu.relay.test"

// fixture is a relay with its client listener and servers' endpoint.
type fixture struct {
	relay   *Relay
	clients string // address
	servers *httptest.Server
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	r := New("eu", "mavio.site", relayURL, vectorSecret, slog.New(slog.DiscardHandler))
	r.AcceptTimeout = 2 * time.Second
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	go func() { _ = r.ServeClients(ctx, ln) }()
	srv := httptest.NewServer(r.Handler())
	t.Cleanup(func() {
		cancel()
		_ = ln.Close()
		srv.Close()
	})
	return &fixture{relay: r, clients: ln.Addr().String(), servers: srv}
}

// server is a fake Mavio server behind the relay.
type server struct {
	key ed25519.PrivateKey
	id  string
}

func newServer() *server {
	pub, key, _ := ed25519.GenerateKey(rand.Reader)
	return &server{key: key, id: ServerID(pub)}
}

// token is the server's JWT for an audience, as cloud.Token makes it.
func (s *server) token(aud string, now time.Time) string {
	enc := base64.RawURLEncoding
	header, _ := json.Marshal(map[string]string{"alg": "EdDSA", "typ": "JWT", "kid": s.id})
	claims, _ := json.Marshal(map[string]any{
		"iss": s.id, "sub": s.id, "aud": aud, "iat": now.Unix(), "exp": now.Add(time.Minute).Unix(), "jti": rand.Text(),
	})
	signed := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	return signed + "." + enc.EncodeToString(ed25519.Sign(s.key, []byte(signed)))
}

// open upgrades a connection to the relay's servers' endpoint.
func (s *server) open(t *testing.T, f *fixture, path, aud, connection string) (net.Conn, *bufio.Reader, int) {
	t.Helper()
	conn, err := net.Dial("tcp", f.servers.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest(http.MethodGet, "http://relay"+path, nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", Protocol)
	req.Header.Set("Authorization", "Bearer "+s.token(aud, time.Now()))
	req.Header.Set(IdentityHeader, base64.RawURLEncoding.EncodeToString(s.key.Public().(ed25519.PublicKey)))
	if connection != "" {
		req.Header.Set(ConnectionHeader, connection)
	}
	if err := req.Write(conn); err != nil {
		t.Fatal(err)
	}
	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		_ = conn.Close()
	}
	return conn, br, resp.StatusCode
}

// serve answers the connections the relay announces with TLS and echoes
// what clients send, until the control connection ends.
func (s *server) serve(t *testing.T, f *fixture, ctrl io.Reader, cert tls.Certificate) {
	frame := make([]byte, FrameSize)
	for {
		if _, err := io.ReadFull(ctrl, frame); err != nil {
			return
		}
		if allZero(frame) {
			continue
		}
		conn, br, code := s.open(t, f, AcceptPath, relayURL, hex.EncodeToString(frame))
		if code != http.StatusSwitchingProtocols {
			t.Errorf("accept = %d, want = 101", code)
			return
		}
		go func() {
			defer conn.Close()
			tc := tls.Server(&readerConn{Conn: conn, r: br}, &tls.Config{Certificates: []tls.Certificate{cert}})
			_, _ = io.Copy(tc, tc)
		}()
	}
}

type readerConn struct {
	net.Conn
	r io.Reader
}

func (c *readerConn) Read(p []byte) (int, error) { return c.r.Read(p) }

func allZero(b []byte) bool {
	for _, c := range b {
		if c != 0 {
			return false
		}
	}
	return true
}

// wildcard returns a self-signed certificate of *.<id>.mavio.site.
func wildcard(t *testing.T, id string) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "*." + id + ".mavio.site"},
		DNSNames:     []string{"*." + id + ".mavio.site"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, _ := x509.ParseCertificate(der)
	pool := x509.NewCertPool()
	pool.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, pool
}

// connected connects a server and waits for the relay to know it.
func connected(t *testing.T, f *fixture, s *server) (net.Conn, *bufio.Reader) {
	t.Helper()
	conn, br, code := s.open(t, f, ConnectPath, relayURL, "")
	if code != http.StatusSwitchingProtocols {
		t.Fatalf("connect = %d, want = 101", code)
	}
	for range 100 {
		f.relay.mu.Lock()
		ok := f.relay.sessions[s.id] != nil
		f.relay.mu.Unlock()
		if ok {
			return conn, br
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("the relay did not register the server")
	return nil, nil
}

func name(s *server, ticket string) string {
	return "relay-eu-" + ticket + "." + s.id + ".mavio.site"
}

// dial connects a client to a name through the relay.
func dial(f *fixture, name string, roots *x509.CertPool) (*tls.Conn, error) {
	d := &net.Dialer{Timeout: 5 * time.Second}
	return tls.DialWithDialer(d, "tcp", f.clients, &tls.Config{ServerName: name, RootCAs: roots})
}

func TestRelay(t *testing.T) {
	f := newFixture(t)
	s := newServer()
	cert, roots := wildcard(t, s.id)
	ctrl, br := connected(t, f, s)
	defer ctrl.Close()
	go s.serve(t, f, br, cert)

	ticket := Ticket(vectorSecret, s.id, "eu", time.Now().Add(5*time.Minute))
	for i := range 2 {
		c, err := dial(f, name(s, ticket), roots)
		if err != nil {
			t.Fatalf("dial %d: %v", i, err)
		}
		msg := strings.Repeat("hello ", 1000)
		go func() { _, _ = io.WriteString(c, msg) }()
		got := make([]byte, len(msg))
		if _, err := io.ReadFull(c, got); err != nil || string(got) != msg {
			t.Errorf("echo %d = %d bytes, %v, want = %d bytes", i, len(got), err, len(msg))
		}
		_ = c.Close()
	}
}

func TestRelayRefuses(t *testing.T) {
	f := newFixture(t)
	s := newServer()
	cert, roots := wildcard(t, s.id)
	ctrl, br := connected(t, f, s)
	defer ctrl.Close()
	go s.serve(t, f, br, cert)
	absent := newServer()
	_, absentRoots := wildcard(t, absent.id)

	valid := time.Now().Add(5 * time.Minute)
	tests := []struct {
		name  string
		host  string
		roots *x509.CertPool
	}{
		{"expired", name(s, Ticket(vectorSecret, s.id, "eu", time.Now().Add(-time.Minute))), roots},
		{"other secret", name(s, Ticket([]byte("another secret, another secret!!"), s.id, "eu", valid)), roots},
		{"other server's ticket", name(s, Ticket(vectorSecret, absent.id, "eu", valid)), roots},
		{"other region", "relay-us-" + Ticket(vectorSecret, s.id, "us", valid) + "." + s.id + ".mavio.site", roots},
		{"not connected", name(absent, Ticket(vectorSecret, absent.id, "eu", valid)), absentRoots},
		{"not a relay name", "192-0-2-1." + s.id + ".mavio.site", roots},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if c, err := dial(f, tt.host, tt.roots); err == nil {
				_ = c.Close()
				t.Error("dial succeeded, want an error")
			}
		})
	}
}

func TestAuthenticate(t *testing.T) {
	f := newFixture(t)
	s := newServer()
	if _, _, code := s.open(t, f, ConnectPath, "https://us.relay.test", ""); code != http.StatusUnauthorized {
		t.Errorf("connect for another relay = %d, want = 401", code)
	}
	// A token naming another server than the key presented.
	other := newServer()
	other.id = s.id
	if _, _, code := other.open(t, f, ConnectPath, relayURL, ""); code != http.StatusUnauthorized {
		t.Errorf("connect as another server = %d, want = 401", code)
	}
	ctrl, _ := connected(t, f, s)
	defer ctrl.Close()

	// A connection announced to one server cannot be taken by another.
	f.relay.mu.Lock()
	f.relay.pending["00"] = pending{server: s.id, back: make(chan net.Conn, 1)}
	f.relay.mu.Unlock()
	thief := newServer()
	if _, _, code := thief.open(t, f, AcceptPath, relayURL, "00"); code != http.StatusNotFound {
		t.Errorf("accept by another server = %d, want = 404", code)
	}
}

func TestReplace(t *testing.T) {
	f := newFixture(t)
	s := newServer()
	first, br := connected(t, f, s)
	defer first.Close()
	f.relay.mu.Lock()
	old := f.relay.sessions[s.id]
	f.relay.mu.Unlock()
	second, _, code := s.open(t, f, ConnectPath, relayURL, "")
	if code != http.StatusSwitchingProtocols {
		t.Fatalf("connect = %d, want = 101", code)
	}
	defer second.Close()
	// The first control connection is closed by the relay.
	_ = first.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := br.ReadByte(); err == nil {
		t.Error("the replaced control connection is still open")
	}
	f.relay.mu.Lock()
	cur := f.relay.sessions[s.id]
	f.relay.mu.Unlock()
	if cur == old || cur == nil {
		t.Error("the session was not replaced")
	}
}

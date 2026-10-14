package relay_test

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/cloud/cloudtest"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	"github.com/mavioai/mavio/apps/server/internal/relay"
	cloudv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/cloud/v1"
)

// fakeRelay speaks the relay's side of the protocol: it hands control
// connections to the test and answers connections back with a greeting.
type fakeRelay struct {
	*httptest.Server
	key      *identity.Key
	controls chan net.Conn
	accepted chan string
}

func newFakeRelay(t *testing.T, key *identity.Key) *fakeRelay {
	f := &fakeRelay{key: key, controls: make(chan net.Conn, 4), accepted: make(chan string, 4)}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /servers/connect", func(w http.ResponseWriter, r *http.Request) {
		if conn := f.upgrade(t, w, r); conn != nil {
			f.controls <- conn
		}
	})
	mux.HandleFunc("GET /servers/accept", func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("Mavio-Relay-Connection")
		conn := f.upgrade(t, w, r)
		if conn == nil {
			return
		}
		f.accepted <- id
		// The client's ClientHello would come first.
		_, _ = io.WriteString(conn, "client hello")
		_, _ = io.Copy(conn, conn)
		_ = conn.Close()
	})
	f.Server = httptest.NewServer(mux)
	t.Cleanup(f.Close)
	return f
}

// upgrade checks a request as the relay does and switches protocols.
func (f *fakeRelay) upgrade(t *testing.T, w http.ResponseWriter, r *http.Request) net.Conn {
	pub, _ := base64.RawURLEncoding.DecodeString(r.Header.Get("Mavio-Identity-Key"))
	token, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	parts := strings.Split(token, ".")
	var claims cloud.Claims
	if len(parts) == 3 {
		b, _ := base64.RawURLEncoding.DecodeString(parts[1])
		_ = json.Unmarshal(b, &claims)
	}
	if r.Header.Get("Upgrade") != "mavio-relay" || identity.IDOf(pub) != f.key.ID() || claims.Audience != f.URL || claims.Subject != f.key.ID() {
		t.Errorf("relay request: upgrade %q, key of %q, claims %+v", r.Header.Get("Upgrade"), identity.IDOf(pub), claims)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return nil
	}
	conn, rw, err := http.NewResponseController(w).Hijack()
	if err != nil {
		t.Error(err)
		return nil
	}
	_, _ = rw.WriteString("HTTP/1.1 101 Switching Protocols\r\nConnection: Upgrade\r\nUpgrade: mavio-relay\r\n\r\n")
	_ = rw.Flush()
	return conn
}

func TestClient(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	dev := cloudtest.New(t)
	key := identity.Generate()
	mavioDev := cloud.NewClient(dev.URL, key, nil)
	dev.AddClaimCode("code", cloudv1.Account_builder{Id: new("acct-1"), UserName: new("alice")}.Build())
	if _, err := mavioDev.ClaimServer(ctx, cloudv1.ClaimServerRequest_builder{ClaimCode: new("code"), PublicKey: key.Public()}.Build()); err != nil {
		t.Fatal(err)
	}
	fake := newFakeRelay(t, key)
	// The other relay is unreachable, so the reachable one is nearest.
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	dev.SetRelays(map[string]string{"eu": fake.URL, "us": closed.URL})

	var allowed atomic.Bool
	c := relay.NewClient(key, mavioDev, func(context.Context) (bool, error) { return allowed.Load(), nil }, nil)
	c.Logger = slog.New(slog.DiscardHandler)
	ln := c.Listener()
	defer ln.Close()
	go c.Run(ctx)

	// Not allowed: no connection.
	select {
	case <-fake.controls:
		t.Fatal("connected while not allowed")
	case <-time.After(100 * time.Millisecond):
	}
	allowed.Store(true)
	c.Wake()
	var ctrl net.Conn
	select {
	case ctrl = <-fake.controls:
	case <-time.After(10 * time.Second):
		t.Fatal("no control connection")
	}
	waitFor(t, func() bool { return dev.RelayRegion(key.ID()) == "eu" }, "the region recorded")

	// A heartbeat, then a client.
	_, _ = ctrl.Write(make([]byte, 16))
	id := []byte("0123456789abcdef")
	_, _ = ctrl.Write(id)
	conn, err := ln.Accept()
	if err != nil {
		t.Fatal(err)
	}
	if got := <-fake.accepted; got != "30313233343536373839616263646566" {
		t.Errorf("accepted connection = %q, want = the announced ID in hex", got)
	}
	br := bufio.NewReader(conn)
	got := make([]byte, len("client hello"))
	if _, err := io.ReadFull(br, got); err != nil || string(got) != "client hello" {
		t.Errorf("relayed = %q, %v, want = client hello", got, err)
	}
	_, _ = io.WriteString(conn, "echo")
	got = make([]byte, 4)
	if _, err := io.ReadFull(br, got); err != nil || string(got) != "echo" {
		t.Errorf("echo = %q, %v, want = echo", got, err)
	}
	_ = conn.Close()

	// No longer allowed: the connection closes and the region is cleared.
	allowed.Store(false)
	c.Wake()
	_ = ctrl.SetReadDeadline(time.Now().Add(10 * time.Second))
	if _, err := ctrl.Read(make([]byte, 1)); err == nil {
		t.Error("the control connection stayed open")
	}
	waitFor(t, func() bool { return dev.RelayRegion(key.ID()) == "" }, "the region cleared")
}

func waitFor(t *testing.T, cond func() bool, what string) {
	t.Helper()
	for range 500 {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

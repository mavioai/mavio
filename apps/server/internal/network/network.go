// Package network serves the server the ways its network settings ask: under
// a base URL behind a reverse proxy, over HTTPS with a configured
// certificate, and discoverable on the local network. Each follows the
// settings as they change, without a restart.
package network

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Prefix serves a handler at the root and under a base URL.
type Prefix struct {
	h    http.Handler
	base atomic.Pointer[string]
}

// NewPrefix returns h served at the root only, until a base URL is set.
func NewPrefix(h http.Handler) *Prefix {
	p := &Prefix{h: h}
	p.Set("")
	return p
}

// Set changes the base URL, such as "/mavio"; empty serves at the root
// only.
func (p *Prefix) Set(base string) { p.base.Store(&base) }

// Base returns the base URL.
func (p *Prefix) Base() string { return *p.base.Load() }

func (p *Prefix) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	base := p.Base()
	if base != "" {
		switch {
		case r.URL.Path == base:
			http.Redirect(w, r, base+"/", http.StatusMovedPermanently)
			return
		case strings.HasPrefix(r.URL.Path, base+"/"):
			r2 := r.Clone(r.Context())
			r2.URL.Path = strings.TrimPrefix(r.URL.Path, base)
			r2.URL.RawPath = ""
			r = r2
		}
	}
	p.h.ServeHTTP(w, r)
}

// HTTPS runs an HTTPS server on the port and with the certificate the
// settings name or, without one, the managed certificate.
type HTTPS struct {
	// Handler serves the requests.
	Handler http.Handler
	// Host is the host listened on; empty means all interfaces.
	Host string
	// Managed, when set, is the certificate served when the settings name
	// none, as soon as it is ready: the server's mavio.site certificate.
	Managed Managed
	Logger  *slog.Logger

	mu      sync.Mutex
	current core.NetworkSettings
	managed bool
	srv     *http.Server
	addr    string
}

// Managed is a certificate kept up to date by something else.
type Managed interface {
	Certificate(*tls.ClientHelloInfo) (*tls.Certificate, error)
	Ready() bool
}

// Apply serves HTTPS as the settings ask: starts, restarts or stops it.
// An unreadable certificate or a busy port reject the settings. Without
// a certificate in the settings, it serves the managed one once ready;
// apply the settings again when it becomes ready.
func (h *HTTPS) Apply(ctx context.Context, n core.NetworkSettings) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	managed := n.CertificatePath == "" && h.Managed != nil && h.Managed.Ready()
	serve := n.HTTPSPort != 0 && (n.CertificatePath != "" || managed)
	same := n.HTTPSPort == h.current.HTTPSPort && n.CertificatePath == h.current.CertificatePath && n.KeyPath == h.current.KeyPath && managed == h.managed
	if same && (h.srv != nil) == serve {
		return nil
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS12}
	switch {
	case !serve:
		h.current, h.managed = n, managed
		return h.stop(ctx)
	case managed:
		cfg.GetCertificate = h.Managed.Certificate
	default:
		cert, err := tls.LoadX509KeyPair(n.CertificatePath, n.KeyPath)
		if err != nil {
			return fmt.Errorf("%w: HTTPS certificate: %w", core.ErrInvalid, err)
		}
		cfg.Certificates = []tls.Certificate{cert}
	}
	// The old server frees the port first when it is the same one.
	if err := h.stop(ctx); err != nil {
		return err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", net.JoinHostPort(h.Host, strconv.Itoa(n.HTTPSPort)))
	if err != nil {
		return fmt.Errorf("%w: HTTPS port: %w", core.ErrInvalid, err)
	}
	srv := &http.Server{
		Handler:           h.Handler,
		TLSConfig:         cfg,
		ReadHeaderTimeout: 10 * time.Second,
	}
	go func() {
		if err := srv.ServeTLS(ln, "", ""); !errors.Is(err, http.ErrServerClosed) && h.Logger != nil {
			h.Logger.Error("HTTPS server stopped", "err", err)
		}
	}()
	h.srv, h.addr, h.current, h.managed = srv, ln.Addr().String(), n, managed
	if h.Logger != nil {
		h.Logger.InfoContext(ctx, "serving HTTPS", "addr", h.addr)
	}
	return nil
}

// Port returns the port HTTPS is served on, zero when it is not.
func (h *HTTPS) Port() int {
	_, port, err := net.SplitHostPort(h.Addr())
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(port)
	return n
}

// Addr returns the address HTTPS is served on, empty when it is not.
func (h *HTTPS) Addr() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.addr
}

func (h *HTTPS) stop(ctx context.Context) error {
	if h.srv == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	err := h.srv.Shutdown(ctx)
	h.srv, h.addr = nil, ""
	if errors.Is(err, context.DeadlineExceeded) {
		err = nil // long requests are cut off
	}
	return err
}

// Close stops serving HTTPS.
func (h *HTTPS) Close(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stop(ctx)
}

// DiscoveryRequest is what clients broadcast to find servers.
const DiscoveryRequest = "who is MavioServer?"

// DiscoveryReply is what a server answers.
type DiscoveryReply struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Address string `json:"address"`
	Version string `json:"version"`
}

// Discovery answers discovery requests on a UDP address while enabled.
type Discovery struct {
	// Addr is the UDP address listened on, such as ":7359".
	Addr string
	// Reply describes the server; address is filled in with the
	// interface the request came in on.
	Reply func() DiscoveryReply
	// Port is the HTTP port addresses name.
	Port   int
	Logger *slog.Logger

	mu   sync.Mutex
	conn net.PacketConn
}

// Apply starts or stops answering as the settings ask.
func (d *Discovery) Apply(ctx context.Context, n core.NetworkSettings) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	switch {
	case !n.LocalDiscovery || d.Addr == "":
		return d.stop()
	case d.conn != nil:
		return nil
	}
	var lc net.ListenConfig
	conn, err := lc.ListenPacket(ctx, "udp", d.Addr)
	if err != nil {
		// Another server on the machine may answer already.
		if d.Logger != nil {
			d.Logger.WarnContext(ctx, "local discovery unavailable", "addr", d.Addr, "err", err)
		}
		return nil
	}
	d.conn = conn
	go d.serve(conn)
	return nil
}

// LocalAddr returns the address answered on, nil when not answering.
func (d *Discovery) LocalAddr() net.Addr {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.conn == nil {
		return nil
	}
	return d.conn.LocalAddr()
}

func (d *Discovery) serve(conn net.PacketConn) {
	buf := make([]byte, 512)
	for {
		n, from, err := conn.ReadFrom(buf)
		if err != nil {
			return
		}
		if strings.TrimSpace(string(buf[:n])) != DiscoveryRequest {
			continue
		}
		reply := d.Reply()
		reply.Address = "http://" + net.JoinHostPort(localIP(from).String(), strconv.Itoa(d.Port))
		data, _ := json.Marshal(reply)
		_, _ = conn.WriteTo(data, from)
	}
}

// localIP returns the address of the interface that reaches to.
func localIP(to net.Addr) net.IP {
	c, err := net.Dial("udp", to.String())
	if err != nil {
		return net.IPv4(127, 0, 0, 1)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP
}

func (d *Discovery) stop() error {
	if d.conn == nil {
		return nil
	}
	err := d.conn.Close()
	d.conn = nil
	return err
}

// Close stops answering.
func (d *Discovery) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stop()
}

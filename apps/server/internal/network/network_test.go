package network

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func echoPath() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, r.URL.Path) })
}

func TestPrefix(t *testing.T) {
	p := NewPrefix(echoPath())
	srv := httptest.NewServer(p)
	t.Cleanup(srv.Close)
	get := func(path string) (string, int) {
		t.Helper()
		c := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
		resp, err := c.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return string(body), resp.StatusCode
	}
	if got, _ := get("/mavio/x"); got != "/mavio/x" {
		t.Errorf("without a base URL: %q", got)
	}
	p.Set("/mavio")
	for path, want := range map[string]string{"/mavio/x": "/x", "/x": "/x", "/maviox": "/maviox"} {
		if got, _ := get(path); got != want {
			t.Errorf("GET %s = %q, want = %q", path, got, want)
		}
	}
	if _, code := get("/mavio"); code != http.StatusMovedPermanently {
		t.Errorf("GET /mavio = %d, want a redirect", code)
	}
}

// certificate writes a self-signed certificate for 127.0.0.1.
func certificate(t *testing.T) (string, string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "test"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IPAddresses: []net.IP{net.IPv4(127, 0, 0, 1)},
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	kder, _ := x509.MarshalECPrivateKey(key)
	dir := t.TempDir()
	cert, kf := filepath.Join(dir, "cert.pem"), filepath.Join(dir, "key.pem")
	_ = os.WriteFile(cert, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600)
	_ = os.WriteFile(kf, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kder}), 0o600)
	return cert, kf
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestHTTPS(t *testing.T) {
	ctx := t.Context()
	h := &HTTPS{Handler: echoPath(), Host: "127.0.0.1"}
	t.Cleanup(func() { _ = h.Close(ctx) })
	cert, key := certificate(t)
	client := &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}} //nolint:gosec // self-signed test certificate
	port := freePort(t)
	if err := h.Apply(ctx, core.NetworkSettings{HTTPSPort: port, CertificatePath: cert, KeyPath: key}); err != nil {
		t.Fatal(err)
	}
	resp, err := client.Get("https://" + h.Addr() + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if string(body) != "/hello" || resp.TLS == nil {
		t.Errorf("HTTPS response = %q", body)
	}
	if err := h.Apply(ctx, core.NetworkSettings{HTTPSPort: port, CertificatePath: cert, KeyPath: "/missing"}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("missing key: %v, want ErrInvalid", err)
	}
	if err := h.Apply(ctx, core.NetworkSettings{}); err != nil || h.Addr() != "" {
		t.Errorf("stopping HTTPS: %v, addr %q", err, h.Addr())
	}
}

func TestDiscovery(t *testing.T) {
	ctx := t.Context()
	d := &Discovery{Addr: "127.0.0.1:0", Port: 8686, Reply: func() DiscoveryReply { return DiscoveryReply{Name: "Home", Version: "v1"} }}
	t.Cleanup(func() { _ = d.Close() })
	if err := d.Apply(ctx, core.NetworkSettings{LocalDiscovery: true}); err != nil {
		t.Fatal(err)
	}
	conn, err := net.Dial("udp", d.LocalAddr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := conn.Write([]byte(DiscoveryRequest)); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 512)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	var reply DiscoveryReply
	if err := json.Unmarshal(buf[:n], &reply); err != nil || reply.Name != "Home" || reply.Address != "http://127.0.0.1:8686" {
		t.Errorf("reply = %+v, %v", reply, err)
	}
	if err := d.Apply(ctx, core.NetworkSettings{}); err != nil || d.LocalAddr() != nil {
		t.Errorf("disabling: %v", err)
	}
}

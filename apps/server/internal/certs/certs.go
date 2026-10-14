package certs

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	mrand "math/rand/v2"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"golang.org/x/crypto/acme"
)

// The certificate authorities a server turns to, in order.
const (
	LetsEncrypt         = "https://acme-v02.api.letsencrypt.org/directory"
	GoogleTrustServices = "https://dv.acme-v02.api.pki.goog/directory"
)

// Zone is the domain servers' names are under.
const Zone = "mavio.site"

// Name returns the certificate name of the server whose ID is id.
func Name(id string) string { return "*." + id + "." + Zone }

// Defaults of the Manager.
const (
	// CheckInterval is how often the manager asks ACME Renewal Information
	// whether to renew early.
	CheckInterval = 6 * time.Hour
	// PropagationDelay is how long the manager waits for a challenge's TXT
	// records to reach the name servers before the CA checks them.
	PropagationDelay = 10 * time.Second
)

// Challenges publishes the TXT records of DNS-01 challenges and gives
// external account bindings; mavio.dev does both.
type Challenges interface {
	SetAcmeChallenge(ctx context.Context, values []string) error
	ClearAcmeChallenge(ctx context.Context) error
	// AcmeBinding returns an external account binding for registering
	// at a CA that requires one.
	AcmeBinding(ctx context.Context) (*acme.ExternalAccountBinding, error)
}

// Manager obtains, renews and serves the certificate of one name.
type Manager struct {
	// Dir holds the certificate, its key, and an ACME account key per CA.
	Dir string
	// Name is the certificate's name, Name(id).
	Name string
	// CAs are the ACME directory URLs tried in order.
	CAs        []string
	Challenges Challenges
	// Active reports whether the server should hold a certificate: in
	// mavio.dev mode, once claimed.
	Active func(ctx context.Context) (bool, error)
	// Changed, when set, is called whenever the certificate changes.
	Changed func()
	// HTTPClient reaches the CAs; nil uses http.DefaultClient.
	HTTPClient *http.Client
	Logger     *slog.Logger
	// CheckInterval, PropagationDelay, MinRetry and MaxRetry override
	// the defaults when not zero; failures are retried after MinRetry
	// (ten minutes), doubling to MaxRetry (six hours).
	CheckInterval, PropagationDelay, MinRetry, MaxRetry time.Duration
	// Now returns the time; nil uses time.Now.
	Now func() time.Time

	cert atomic.Pointer[tls.Certificate]
	wake chan struct{}
}

// The files in Dir.
const (
	certFile = "certificate.pem"
	keyFile  = "certificate.key"
	metaFile = "certificate.json"
)

// meta records where the certificate came from, for ACME Renewal
// Information.
type meta struct {
	CA string `json:"ca"`
}

// NewManager returns a manager of name's certificate in dir.
func NewManager(dir, name string, challenges Challenges, active func(context.Context) (bool, error), logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		Dir: dir, Name: name, CAs: []string{LetsEncrypt, GoogleTrustServices},
		Challenges: challenges, Active: active, Logger: logger, wake: make(chan struct{}, 1),
	}
}

// Certificate serves the certificate, as tls.Config.GetCertificate.
func (m *Manager) Certificate(*tls.ClientHelloInfo) (*tls.Certificate, error) {
	if c := m.cert.Load(); c != nil {
		return c, nil
	}
	return nil, errors.New("no certificate yet")
}

// Ready reports whether the manager has a certificate to serve.
func (m *Manager) Ready() bool { return m.cert.Load() != nil }

// Wake has the manager check again whether it is active, as after the
// server is claimed.
func (m *Manager) Wake() {
	select {
	case m.wake <- struct{}{}:
	default:
	}
}

// Run loads the stored certificate and keeps it current until ctx ends.
func (m *Manager) Run(ctx context.Context) {
	if err := m.load(); err != nil {
		m.Logger.WarnContext(ctx, "stored certificate unusable", "err", err)
	}
	minRetry, maxRetry := or(m.MinRetry, 10*time.Minute), or(m.MaxRetry, 6*time.Hour)
	retry := minRetry
	for ctx.Err() == nil {
		var wait time.Duration
		active, err := m.Active(ctx)
		switch {
		case err != nil:
			m.Logger.ErrorContext(ctx, "certificate: read the account settings", "err", err)
			wait = minRetry
		case !active:
			select {
			case <-ctx.Done():
			case <-m.wake:
			}
			continue
		default:
			due := m.renewAt(ctx)
			if !m.now().Before(due) {
				if err := m.obtain(ctx); err != nil {
					if ctx.Err() != nil {
						return
					}
					m.Logger.ErrorContext(ctx, "obtain the server's certificate", "name", m.Name, "err", err)
					wait = retry/2 + mrand.N(retry/2+1)
					retry = min(2*retry, maxRetry)
					break
				}
				retry = minRetry
				continue
			}
			wait = min(due.Sub(m.now()), or(m.CheckInterval, CheckInterval))
		}
		select {
		case <-ctx.Done():
		case <-m.wake:
		case <-time.After(wait):
		}
	}
}

// renewAt returns when the certificate should be renewed: at two thirds
// of its lifetime, or earlier within the window its CA's renewal
// information suggests; now when there is none.
func (m *Manager) renewAt(ctx context.Context) time.Time {
	c := m.cert.Load()
	if c == nil || c.Leaf == nil {
		return m.now()
	}
	leaf := c.Leaf
	at := leaf.NotBefore.Add(leaf.NotAfter.Sub(leaf.NotBefore) * 2 / 3)
	md, err := m.readMeta()
	if err != nil {
		return at
	}
	start, end, err := renewalWindow(ctx, m.client(md.CA, nil), md.CA, leaf)
	if err != nil {
		m.Logger.DebugContext(ctx, "no renewal information", "err", err)
		return at
	}
	if suggested := start.Add(mrand.N(end.Sub(start) + 1)); suggested.Before(at) {
		return suggested
	}
	return at
}

// obtain gets a new certificate from the first CA that issues one.
func (m *Manager) obtain(ctx context.Context) error {
	var errs []error
	for _, ca := range m.CAs {
		// The certificate is stored once the challenge is cleared.
		chain, key, err := m.obtainFrom(ctx, ca)
		if err == nil {
			m.Logger.InfoContext(ctx, "obtained the server's certificate", "name", m.Name, "ca", ca)
			return m.store(chain, key, ca)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		m.Logger.WarnContext(ctx, "certificate authority failed", "ca", ca, "err", err)
		errs = append(errs, fmt.Errorf("%s: %w", ca, err))
	}
	return errors.Join(errs...)
}

// obtainFrom gets a certificate chain and its key from a CA.
func (m *Manager) obtainFrom(ctx context.Context, ca string) (chain [][]byte, certKey *ecdsa.PrivateKey, err error) {
	key, err := m.accountKey(ca)
	if err != nil {
		return nil, nil, err
	}
	client := m.client(ca, key)
	if err := m.register(ctx, client); err != nil {
		return nil, nil, err
	}
	order, err := client.AuthorizeOrder(ctx, acme.DomainIDs(m.Name))
	if err != nil {
		return nil, nil, fmt.Errorf("order: %w", err)
	}
	var values []string
	var pending []*acme.Challenge
	var authzs []string
	for _, u := range order.AuthzURLs {
		z, err := client.GetAuthorization(ctx, u)
		if err != nil {
			return nil, nil, fmt.Errorf("authorization: %w", err)
		}
		if z.Status == acme.StatusValid {
			continue
		}
		var chal *acme.Challenge
		for _, c := range z.Challenges {
			if c.Type == "dns-01" {
				chal = c
			}
		}
		if chal == nil {
			return nil, nil, fmt.Errorf("%s offers no dns-01 challenge", z.Identifier.Value)
		}
		value, err := client.DNS01ChallengeRecord(chal.Token)
		if err != nil {
			return nil, nil, err
		}
		values, pending, authzs = append(values, value), append(pending, chal), append(authzs, z.URI)
	}
	if len(pending) > 0 {
		if err := m.Challenges.SetAcmeChallenge(ctx, values); err != nil {
			return nil, nil, fmt.Errorf("publish the challenge: %w", err)
		}
		defer func() {
			clearCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
			defer cancel()
			if err := m.Challenges.ClearAcmeChallenge(clearCtx); err != nil {
				m.Logger.WarnContext(ctx, "clear the challenge", "err", err)
			}
		}()
		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()
		case <-time.After(or(m.PropagationDelay, PropagationDelay)):
		}
		for i, chal := range pending {
			if _, err := client.Accept(ctx, chal); err != nil {
				return nil, nil, fmt.Errorf("accept the challenge: %w", err)
			}
			if _, err := client.WaitAuthorization(ctx, authzs[i]); err != nil {
				return nil, nil, fmt.Errorf("challenge: %w", err)
			}
		}
	}
	if _, err := client.WaitOrder(ctx, order.URI); err != nil {
		return nil, nil, fmt.Errorf("order: %w", err)
	}
	certKey, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, err
	}
	csr, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{DNSNames: []string{m.Name}}, certKey)
	if err != nil {
		return nil, nil, err
	}
	if chain, _, err = client.CreateOrderCert(ctx, order.FinalizeURL, csr, true); err != nil {
		return nil, nil, fmt.Errorf("finalize: %w", err)
	}
	return chain, certKey, nil
}

// register registers the account key at the CA unless it already is,
// with an external account binding when the CA requires one.
func (m *Manager) register(ctx context.Context, client *acme.Client) error {
	_, err := client.GetReg(ctx, "")
	if err == nil {
		return nil
	}
	if !errors.Is(err, acme.ErrNoAccount) {
		return fmt.Errorf("account: %w", err)
	}
	dir, err := client.Discover(ctx)
	if err != nil {
		return fmt.Errorf("directory: %w", err)
	}
	acct := &acme.Account{}
	if dir.ExternalAccountRequired {
		if acct.ExternalAccountBinding, err = m.Challenges.AcmeBinding(ctx); err != nil {
			return fmt.Errorf("external account binding: %w", err)
		}
	}
	if _, err := client.Register(ctx, acct, acme.AcceptTOS); err != nil && !errors.Is(err, acme.ErrAccountAlreadyExists) {
		return fmt.Errorf("register: %w", err)
	}
	return nil
}

func (m *Manager) client(ca string, key crypto.Signer) *acme.Client {
	return &acme.Client{Key: key, DirectoryURL: ca, HTTPClient: m.HTTPClient, UserAgent: "mavio"}
}

// accountKey returns the ACME account key of a CA, created when missing.
func (m *Manager) accountKey(ca string) (crypto.Signer, error) {
	sum := sha256.Sum256([]byte(ca))
	file := filepath.Join(m.Dir, "acme-"+hex.EncodeToString(sum[:8])+".key")
	data, err := os.ReadFile(file)
	if err == nil {
		return parseKey(data)
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("read the ACME account key: %w", err)
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	pemKey, err := encodeKey(key)
	if err != nil {
		return nil, err
	}
	if err := m.write(file, pemKey); err != nil {
		return nil, err
	}
	return key, nil
}

// store keeps a new certificate and starts serving it.
func (m *Manager) store(chain [][]byte, key *ecdsa.PrivateKey, ca string) error {
	var certPEM []byte
	for _, der := range chain {
		certPEM = append(certPEM, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})...)
	}
	keyPEM, err := encodeKey(key)
	if err != nil {
		return err
	}
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		return fmt.Errorf("issued certificate: %w", err)
	}
	md, _ := json.Marshal(meta{CA: ca})
	for _, f := range []struct {
		name string
		data []byte
	}{{keyFile, keyPEM}, {certFile, certPEM}, {metaFile, md}} {
		if err := m.write(filepath.Join(m.Dir, f.name), f.data); err != nil {
			return err
		}
	}
	m.set(&cert)
	return nil
}

// load serves the stored certificate, if any.
func (m *Manager) load() error {
	cert, err := tls.LoadX509KeyPair(filepath.Join(m.Dir, certFile), filepath.Join(m.Dir, keyFile))
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := cert.Leaf.VerifyHostname("x" + m.Name[1:]); err != nil {
		return fmt.Errorf("stored certificate is for another name: %w", err)
	}
	m.set(&cert)
	return nil
}

func (m *Manager) set(cert *tls.Certificate) {
	m.cert.Store(cert)
	if m.Changed != nil {
		m.Changed()
	}
}

func (m *Manager) readMeta() (meta, error) {
	var md meta
	data, err := os.ReadFile(filepath.Join(m.Dir, metaFile))
	if err != nil {
		return md, err
	}
	return md, json.Unmarshal(data, &md)
}

// write replaces a file, readable by the owner only, atomically.
func (m *Manager) write(file string, data []byte) error {
	if err := os.MkdirAll(m.Dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", m.Dir, err)
	}
	tmp, err := os.CreateTemp(m.Dir, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), file); err != nil {
		return fmt.Errorf("write %s: %w", file, err)
	}
	return nil
}

func (m *Manager) now() time.Time {
	if m.Now != nil {
		return m.Now()
	}
	return time.Now()
}

func encodeKey(key *ecdsa.PrivateKey) ([]byte, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	return pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}), nil
}

func parseKey(data []byte) (crypto.Signer, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, errors.New("ACME account key: not PEM")
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("ACME account key: %w", err)
	}
	signer, ok := key.(crypto.Signer)
	if !ok {
		return nil, errors.New("ACME account key: not a signing key")
	}
	return signer, nil
}

// renewalWindow asks a CA's ACME Renewal Information (RFC 9773) when to
// renew a certificate it issued.
func renewalWindow(ctx context.Context, client *acme.Client, ca string, leaf *x509.Certificate) (start, end time.Time, err error) {
	hc := client.HTTPClient
	if hc == nil {
		hc = http.DefaultClient
	}
	var dir struct {
		RenewalInfo string `json:"renewalInfo"`
	}
	if err := getJSON(ctx, hc, ca, &dir); err != nil {
		return start, end, err
	}
	if dir.RenewalInfo == "" {
		return start, end, errors.New("the CA offers no renewal information")
	}
	var info struct {
		SuggestedWindow struct {
			Start time.Time `json:"start"`
			End   time.Time `json:"end"`
		} `json:"suggestedWindow"`
	}
	if err := getJSON(ctx, hc, dir.RenewalInfo+"/"+certID(leaf), &info); err != nil {
		return start, end, err
	}
	w := info.SuggestedWindow
	if w.Start.IsZero() || w.End.Before(w.Start) {
		return start, end, errors.New("malformed renewal window")
	}
	return w.Start, w.End, nil
}

// certID is a certificate's ARI identifier: its authority key identifier
// and the DER encoding of its serial number, in base64url.
func certID(leaf *x509.Certificate) string {
	serial := leaf.SerialNumber.Bytes()
	if len(serial) == 0 || serial[0]&0x80 != 0 {
		serial = append([]byte{0}, serial...)
	}
	enc := base64.RawURLEncoding
	return enc.EncodeToString(leaf.AuthorityKeyId) + "." + enc.EncodeToString(serial)
}

func getJSON(ctx context.Context, hc *http.Client, url string, v any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", url, resp.Status)
	}
	return json.NewDecoder(resp.Body).Decode(v)
}

func or(d, def time.Duration) time.Duration {
	if d > 0 {
		return d
	}
	return def
}

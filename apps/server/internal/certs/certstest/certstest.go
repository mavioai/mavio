// Package certstest is an ACME certificate authority for tests: the parts
// of RFC 8555 the server uses, DNS-01 challenges checked against TXT
// records the test publishes, external account bindings and ACME Renewal
// Information. It trusts every JWS without checking its signature.
package certstest

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/acme"
)

// CA is an ACME server.
type CA struct {
	*httptest.Server
	// TXT returns the TXT values of a challenge's name, such as
	// _acme-challenge.<ID>.mavio.site.
	TXT func(name string) []string
	// BindingKeyID, when set, is the key ID of the external account
	// binding new accounts need.
	BindingKeyID string
	// Lifetime is how long certificates are valid; zero is 90 days.
	Lifetime time.Duration
	// Window, when set, is the renewal window ARI suggests.
	Window func() (start, end time.Time)

	key  *ecdsa.PrivateKey
	root *x509.Certificate

	mu       sync.Mutex
	accounts map[string]string // thumbprint by account URL
	orders   map[string]*order
	authzs   map[string]*authz
	certs    map[string][]byte
	n        int
	// Issued counts the certificates issued.
	Issued int
}

type order struct {
	Status         string         `json:"status"`
	Identifiers    []acme.AuthzID `json:"identifiers"`
	Authorizations []string       `json:"authorizations"`
	Finalize       string         `json:"finalize"`
	Certificate    string         `json:"certificate,omitempty"`
}

type authz struct {
	Identifier acme.AuthzID `json:"identifier"`
	Status     string       `json:"status"`
	Wildcard   bool         `json:"wildcard"`
	Challenges []challenge  `json:"challenges"`
	account    string
	order      *order
}

type challenge struct {
	Type   string `json:"type"`
	URL    string `json:"url"`
	Token  string `json:"token"`
	Status string `json:"status"`
}

// New starts a CA; the test closes it.
func New() *CA {
	key, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "certstest root"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(10 * 365 * 24 * time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, _ := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	root, _ := x509.ParseCertificate(der)
	ca := &CA{
		key: key, root: root,
		accounts: map[string]string{}, orders: map[string]*order{}, authzs: map[string]*authz{}, certs: map[string][]byte{},
	}
	ca.Server = httptest.NewServer(http.HandlerFunc(ca.serve))
	return ca
}

// Directory is the CA's directory URL.
func (ca *CA) Directory() string { return ca.URL + "/directory" }

// Roots returns a pool holding the CA's root.
func (ca *CA) Roots() *x509.CertPool {
	p := x509.NewCertPool()
	p.AddCert(ca.root)
	return p
}

type jws struct {
	Protected string `json:"protected"`
	Payload   string `json:"payload"`
}

type protected struct {
	JWK json.RawMessage `json:"jwk"`
	KID string          `json:"kid"`
}

func (ca *CA) serve(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Replay-Nonce", base64.RawURLEncoding.EncodeToString([]byte(time.Now().String())))
	w.Header().Set("Cache-Control", "no-store")
	switch {
	case r.URL.Path == "/directory":
		writeJSON(w, http.StatusOK, map[string]any{
			"newNonce": ca.URL + "/nonce", "newAccount": ca.URL + "/account", "newOrder": ca.URL + "/order",
			"revokeCert": ca.URL + "/revoke", "keyChange": ca.URL + "/key-change", "renewalInfo": ca.URL + "/renewal-info",
			"meta": map[string]any{"termsOfService": ca.URL + "/terms", "externalAccountRequired": ca.BindingKeyID != ""},
		})
		return
	case r.URL.Path == "/nonce":
		w.WriteHeader(http.StatusOK)
		return
	case strings.HasPrefix(r.URL.Path, "/renewal-info/"):
		if ca.Window == nil {
			http.NotFound(w, r)
			return
		}
		start, end := ca.Window()
		writeJSON(w, http.StatusOK, map[string]any{"suggestedWindow": map[string]time.Time{"start": start, "end": end}})
		return
	}
	var body jws
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		problem(w, "malformed", err.Error())
		return
	}
	var prot protected
	if err := decode(body.Protected, &prot); err != nil {
		problem(w, "malformed", err.Error())
		return
	}
	payload, _ := base64.RawURLEncoding.DecodeString(body.Payload)

	ca.mu.Lock()
	defer ca.mu.Unlock()
	if r.URL.Path == "/account" {
		ca.account(w, prot, payload)
		return
	}
	account := prot.KID
	if _, ok := ca.accounts[account]; !ok {
		problem(w, "accountDoesNotExist", "unknown account")
		return
	}
	switch path := r.URL.Path; {
	case path == "/order":
		ca.newOrder(w, account, payload)
	case strings.HasPrefix(path, "/order/"):
		o, ok := ca.orders[path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		ca.refresh(o)
		writeJSON(w, http.StatusOK, o)
	case strings.HasPrefix(path, "/finalize/"):
		ca.finalize(w, strings.Replace(path, "/finalize/", "/order/", 1), payload)
	case strings.HasPrefix(path, "/authz/"):
		z, ok := ca.authzs[path]
		if !ok || z.account != account {
			http.NotFound(w, r)
			return
		}
		writeJSON(w, http.StatusOK, z)
	case strings.HasPrefix(path, "/chal/"):
		ca.accept(w, strings.Replace(path, "/chal/", "/authz/", 1))
	case strings.HasPrefix(path, "/cert/"):
		w.Header().Set("Content-Type", "application/pem-certificate-chain")
		_, _ = w.Write(ca.certs[path])
	default:
		http.NotFound(w, r)
	}
}

func (ca *CA) account(w http.ResponseWriter, prot protected, payload []byte) {
	var req struct {
		OnlyReturnExisting     bool `json:"onlyReturnExisting"`
		ExternalAccountBinding *jws `json:"externalAccountBinding"`
	}
	_ = json.Unmarshal(payload, &req)
	thumb := thumbprint(prot.JWK)
	url := ca.URL + "/acct/" + thumb[:16]
	w.Header().Set("Location", url)
	if _, ok := ca.accounts[url]; ok {
		writeJSON(w, http.StatusOK, map[string]string{"status": "valid"})
		return
	}
	if req.OnlyReturnExisting {
		problem(w, "accountDoesNotExist", "no such account")
		return
	}
	if ca.BindingKeyID != "" {
		var eab protected
		if req.ExternalAccountBinding == nil || decode(req.ExternalAccountBinding.Protected, &eab) != nil || eab.KID != ca.BindingKeyID {
			problem(w, "externalAccountRequired", "an external account binding is required")
			return
		}
	}
	ca.accounts[url] = thumb
	writeJSON(w, http.StatusCreated, map[string]string{"status": "valid"})
}

func (ca *CA) newOrder(w http.ResponseWriter, account string, payload []byte) {
	var req struct {
		Identifiers []acme.AuthzID `json:"identifiers"`
	}
	_ = json.Unmarshal(payload, &req)
	ca.n++
	o := &order{Status: "pending", Identifiers: req.Identifiers, Finalize: fmt.Sprintf("%s/finalize/%d", ca.URL, ca.n)}
	for i, id := range req.Identifiers {
		name, wildcard := strings.CutPrefix(id.Value, "*.")
		path := fmt.Sprintf("/authz/%d-%d", ca.n, i)
		z := &authz{
			Identifier: acme.AuthzID{Type: "dns", Value: name}, Status: "pending", Wildcard: wildcard, account: account, order: o,
			Challenges: []challenge{{Type: "dns-01", URL: ca.URL + strings.Replace(path, "/authz/", "/chal/", 1), Token: rand.Text(), Status: "pending"}},
		}
		ca.authzs[path] = z
		o.Authorizations = append(o.Authorizations, ca.URL+path)
	}
	ca.orders[fmt.Sprintf("/order/%d", ca.n)] = o
	w.Header().Set("Location", fmt.Sprintf("%s/order/%d", ca.URL, ca.n))
	writeJSON(w, http.StatusCreated, o)
}

// accept checks a DNS-01 challenge against the TXT records at once.
func (ca *CA) accept(w http.ResponseWriter, path string) {
	z, ok := ca.authzs[path]
	if !ok {
		problem(w, "malformed", "no such challenge")
		return
	}
	c := &z.Challenges[0]
	sum := sha256.Sum256([]byte(c.Token + "." + ca.accounts[z.account]))
	want := base64.RawURLEncoding.EncodeToString(sum[:])
	c.Status, z.Status = "invalid", "invalid"
	for _, v := range ca.TXT("_acme-challenge." + z.Identifier.Value) {
		if v == want {
			c.Status, z.Status = "valid", "valid"
		}
	}
	ca.refresh(z.order)
	writeJSON(w, http.StatusOK, c)
}

// refresh makes an order ready once its authorizations are valid.
func (ca *CA) refresh(o *order) {
	if o.Status != "pending" {
		return
	}
	for _, u := range o.Authorizations {
		switch ca.authzs[strings.TrimPrefix(u, ca.URL)].Status {
		case "invalid":
			o.Status = "invalid"
			return
		case "pending":
			return
		}
	}
	o.Status = "ready"
}

func (ca *CA) finalize(w http.ResponseWriter, path string, payload []byte) {
	o, ok := ca.orders[path]
	if !ok || o.Status != "ready" {
		problem(w, "orderNotReady", "the order is not ready")
		return
	}
	var req struct {
		CSR string `json:"csr"`
	}
	_ = json.Unmarshal(payload, &req)
	der, _ := base64.RawURLEncoding.DecodeString(req.CSR)
	csr, err := x509.ParseCertificateRequest(der)
	if err != nil {
		problem(w, "badCSR", err.Error())
		return
	}
	lifetime := ca.Lifetime
	if lifetime == 0 {
		lifetime = 90 * 24 * time.Hour
	}
	ca.n++
	leaf := &x509.Certificate{
		SerialNumber: big.NewInt(int64(1000 + ca.n)), DNSNames: csr.DNSNames,
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(lifetime),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, leaf, ca.root, csr.PublicKey, crypto.Signer(ca.key))
	if err != nil {
		problem(w, "serverInternal", err.Error())
		return
	}
	certPath := fmt.Sprintf("/cert/%d", ca.n)
	ca.certs[certPath] = append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER}),
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: ca.root.Raw})...)
	ca.Issued++
	o.Status, o.Certificate = "valid", ca.URL+certPath
	w.Header().Set("Location", ca.URL+path)
	writeJSON(w, http.StatusOK, o)
}

// Accounts returns how many accounts are registered.
func (ca *CA) Accounts() int {
	ca.mu.Lock()
	defer ca.mu.Unlock()
	return len(ca.accounts)
}

// thumbprint is a JWK's RFC 7638 thumbprint.
func thumbprint(jwk json.RawMessage) string {
	var k map[string]string
	_ = json.Unmarshal(jwk, &k)
	var canonical string
	switch k["kty"] {
	case "EC":
		canonical = fmt.Sprintf(`{"crv":%q,"kty":"EC","x":%q,"y":%q}`, k["crv"], k["x"], k["y"])
	case "RSA":
		canonical = fmt.Sprintf(`{"e":%q,"kty":"RSA","n":%q}`, k["e"], k["n"])
	}
	sum := sha256.Sum256([]byte(canonical))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func decode(s string, v any) error {
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func problem(w http.ResponseWriter, typ, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"type": "urn:ietf:params:acme:error:" + typ, "detail": detail})
}

// Challenges is a mavio.dev for tests: it keeps the TXT values the server
// publishes, as the CA's TXT reads them for the name of id, and gives a
// binding with keyID.
type Challenges struct {
	ID, KeyID string

	mu     sync.Mutex
	values []string
	// Set and Cleared count the calls.
	Set, Cleared, Bindings int
}

func (c *Challenges) SetAcmeChallenge(_ context.Context, values []string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values, c.Set = values, c.Set+1
	return nil
}

func (c *Challenges) ClearAcmeChallenge(context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values, c.Cleared = nil, c.Cleared+1
	return nil
}

func (c *Challenges) AcmeBinding(context.Context) (*acme.ExternalAccountBinding, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.Bindings++
	return &acme.ExternalAccountBinding{KID: c.KeyID, Key: []byte("binding mac key")}, nil
}

// TXT gives the published values at the challenge name of c's ID.
func (c *Challenges) TXT(name string) []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if name != "_acme-challenge."+c.ID+".mavio.site" {
		return nil
	}
	return c.values
}

package cloud

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Sign-in assertions (docs/accounts.md §5.5).
const (
	// AssertionLifetime is the longest an assertion is valid.
	AssertionLifetime = 2 * time.Minute
	// clockSkew is how far the clocks of mavio.dev and the server may
	// differ.
	clockSkew = 30 * time.Second
	// assertionContext prefixes the assertion in the device's signature.
	assertionContext = "mavio-sign-in-assertion-v1"
)

// ErrInvalidAssertion is returned for an assertion the server does not
// accept.
var ErrInvalidAssertion = errors.New("invalid sign-in assertion")

// AssertionClaims are the claims of a sign-in assertion: mavio.dev, the
// server, the account, the device's public key and a nonce.
type AssertionClaims struct {
	Issuer    string `json:"iss"`
	Audience  string `json:"aud"`
	Subject   string `json:"sub"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
	// DeviceKey is the device's Ed25519 public key, base64url-encoded
	// without padding.
	DeviceKey string `json:"device_key"`
	Nonce     string `json:"nonce"`
}

// Assertion is a verified sign-in assertion.
type Assertion struct {
	AccountID string
	DeviceKey ed25519.PublicKey
}

// AssertionMessage returns what a device signs to present an assertion.
func AssertionMessage(assertion string) []byte {
	return []byte(assertionContext + "\x00" + assertion)
}

// Verifier checks sign-in assertions for one server and accepts each
// once.
type Verifier struct {
	Keys *KeySet
	// Issuer is mavio.dev's API base URL, and Audience the server's ID.
	Issuer, Audience string
	now              func() time.Time

	mu   sync.Mutex
	seen map[string]time.Time
}

// NewVerifier returns a verifier of the assertions mavio.dev at base
// issues for the server with an ID.
func NewVerifier(base, serverID string, keys *KeySet) *Verifier {
	return &Verifier{Keys: keys, Issuer: strings.TrimSuffix(base, "/"), Audience: serverID, now: time.Now}
}

// Verify checks an assertion and the device's signature of it. Assertions
// that do not verify return ErrInvalidAssertion; failing to fetch
// mavio.dev's keys returns another error.
func (v *Verifier) Verify(ctx context.Context, token string, signature []byte) (Assertion, error) {
	invalid := func(why string) (Assertion, error) {
		return Assertion{}, fmt.Errorf("%w: %s", ErrInvalidAssertion, why)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return invalid("not a JWT")
	}
	var header Header
	var claims AssertionClaims
	for i, out := range []any{&header, &claims} {
		raw, err := base64.RawURLEncoding.DecodeString(parts[i])
		if err != nil || json.Unmarshal(raw, out) != nil {
			return invalid("malformed")
		}
	}
	if header.Algorithm != "EdDSA" {
		return invalid("not signed with EdDSA")
	}
	key, err := v.Keys.Key(ctx, header.KeyID)
	if errors.Is(err, ErrUnknownKey) {
		return invalid(err.Error())
	} else if err != nil {
		return Assertion{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !ed25519.Verify(key, []byte(parts[0]+"."+parts[1]), sig) {
		return invalid("bad signature")
	}
	now := v.now()
	device, err := base64.RawURLEncoding.DecodeString(claims.DeviceKey)
	switch {
	case claims.Issuer != v.Issuer:
		return invalid("issued by " + claims.Issuer)
	case claims.Audience != v.Audience:
		return invalid("for another server")
	case claims.Subject == "" || claims.Nonce == "":
		return invalid("no account or nonce")
	case now.Unix() > claims.ExpiresAt+int64(clockSkew/time.Second):
		return invalid("expired")
	case claims.ExpiresAt-claims.IssuedAt > int64(AssertionLifetime/time.Second) || claims.IssuedAt > now.Add(clockSkew).Unix():
		return invalid("valid for too long")
	case err != nil || len(device) != ed25519.PublicKeySize:
		return invalid("no device key")
	case !ed25519.Verify(device, AssertionMessage(token), signature):
		return invalid("not signed by the device")
	}
	if !v.accept(claims.Nonce, time.Unix(claims.ExpiresAt, 0).Add(clockSkew), now) {
		return invalid("already used")
	}
	return Assertion{AccountID: claims.Subject, DeviceKey: device}, nil
}

// accept records a nonce until it expires, reporting whether it is new.
func (v *Verifier) accept(nonce string, expires, now time.Time) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	for n, at := range v.seen {
		if now.After(at) {
			delete(v.seen, n)
		}
	}
	if _, ok := v.seen[nonce]; ok {
		return false
	}
	if v.seen == nil {
		v.seen = map[string]time.Time{}
	}
	v.seen[nonce] = expires
	return true
}

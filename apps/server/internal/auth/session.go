package auth

import (
	"bytes"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Session token lifetimes.
const (
	// AccessTokenTTL is how long an access token is accepted.
	AccessTokenTTL = time.Hour
	// NonceTTL is how long a refresh nonce waits for its signature.
	NonceTTL = 2 * time.Minute
)

// RefreshContext prefixes what a device signs to refresh its session:
// the context, the session's 16-byte ID and the nonce. It keeps the
// signature from serving any other purpose.
const RefreshContext = "mavio-session-refresh-v1\x00"

// TokenKeyPurpose derives the key of access tokens from the server's
// identity key.
const TokenKeyPurpose = "mavio access tokens v1"

// Errors of access tokens.
var (
	// ErrTokenExpired is an access token past its time; the device
	// refreshes its session.
	ErrTokenExpired = errors.New("access token expired")
	// errNotAccessToken is a token of another kind, such as an API key's.
	errNotAccessToken = errors.New("not an access token")
	errBadToken       = errors.New("invalid access token")
)

const (
	accessTokenVersion = 1
	// version, session ID, expiry in Unix seconds, HMAC-SHA256.
	accessTokenLen = 1 + 16 + 8 + sha256.Size
)

// AccessTokens issues and checks the short-lived access tokens of
// sessions: a session's ID and expiry, authenticated with HMAC-SHA256
// under a key only the server holds. Nothing is stored for them, so
// every device of a session, or every tab of a browser, refreshes on its
// own; ending a session ends its tokens, which name it.
type AccessTokens struct {
	key []byte
	now func() time.Time
}

// NewAccessTokens returns tokens authenticated with key.
func NewAccessTokens(key []byte) *AccessTokens {
	return &AccessTokens{key: bytes.Clone(key), now: time.Now}
}

// Issue returns a new access token of a session and when it expires.
func (t *AccessTokens) Issue(session core.ID) (string, time.Time) {
	expires := t.now().Add(AccessTokenTTL).Truncate(time.Second)
	b := make([]byte, 0, accessTokenLen)
	b = append(b, accessTokenVersion)
	b = append(b, session[:]...)
	b = binary.BigEndian.AppendUint64(b, uint64(expires.Unix()))
	b = append(b, t.mac(b)...)
	return base64.RawURLEncoding.EncodeToString(b), expires
}

// Verify returns the session of an access token: errNotAccessToken for a
// token of another kind, ErrTokenExpired for one past its time.
func (t *AccessTokens) Verify(token string) (core.ID, error) {
	b, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil || len(b) != accessTokenLen || b[0] != accessTokenVersion {
		return core.ID{}, errNotAccessToken
	}
	body, sum := b[:accessTokenLen-sha256.Size], b[accessTokenLen-sha256.Size:]
	if !hmac.Equal(sum, t.mac(body)) {
		return core.ID{}, errBadToken
	}
	var id core.ID
	copy(id[:], body[1:17])
	if expires := time.Unix(int64(binary.BigEndian.Uint64(body[17:])), 0); !t.now().Before(expires) {
		return core.ID{}, ErrTokenExpired
	}
	return id, nil
}

func (t *AccessTokens) mac(body []byte) []byte {
	h := hmac.New(sha256.New, t.key)
	h.Write(body)
	return h.Sum(nil)
}

// Nonces keeps each session's pending refresh nonce in memory: one per
// session, used once, for NonceTTL.
type Nonces struct {
	mu      sync.Mutex
	pending map[core.ID]nonce
	now     func() time.Time
}

type nonce struct {
	value   []byte
	expires time.Time
}

// NewNonces returns no pending nonces.
func NewNonces() *Nonces {
	return &Nonces{pending: map[core.ID]nonce{}, now: time.Now}
}

// Start returns a new nonce for a session, replacing its pending one, and
// when it expires.
func (n *Nonces) Start(session core.ID) ([]byte, time.Time) {
	v := make([]byte, 32)
	_, _ = rand.Read(v) // never fails
	now := n.now()
	n.mu.Lock()
	defer n.mu.Unlock()
	for id, p := range n.pending {
		if !now.Before(p.expires) {
			delete(n.pending, id)
		}
	}
	expires := now.Add(NonceTTL)
	n.pending[session] = nonce{value: v, expires: expires}
	return bytes.Clone(v), expires
}

// Redeem reports whether value is the session's pending nonce, which
// then stops being pending, whatever the answer: a guess costs the
// device a new nonce.
func (n *Nonces) Redeem(session core.ID, value []byte) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	p, ok := n.pending[session]
	delete(n.pending, session)
	return ok && n.now().Before(p.expires) && hmac.Equal(p.value, value)
}

// RefreshMessage is what a device signs with its key to refresh a
// session with a nonce.
func RefreshMessage(session core.ID, nonce []byte) []byte {
	m := make([]byte, 0, len(RefreshContext)+16+len(nonce))
	m = append(m, RefreshContext...)
	m = append(m, session[:]...)
	return append(m, nonce...)
}

// VerifyRefresh reports whether signature is the device key's signature
// of the session's refresh with nonce.
func VerifyRefresh(deviceKey []byte, session core.ID, nonce, signature []byte) bool {
	return len(deviceKey) == ed25519.PublicKeySize && ed25519.Verify(deviceKey, RefreshMessage(session, nonce), signature)
}

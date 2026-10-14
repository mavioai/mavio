package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Quick Connect lifetimes, as in Jellyfin.
const (
	// QuickConnectTimeout is how long a request waits to be authorized.
	QuickConnectTimeout = 10 * time.Minute
	// QuickConnectClaimTime is how long an authorized request waits for
	// its device to sign in.
	QuickConnectClaimTime = time.Minute
)

// Device describes a client installation signing in.
type Device struct {
	ID, Name, Client, ClientVersion string
	// Key is the device's Ed25519 public key, which its session is bound
	// to.
	Key []byte
}

// QuickConnect keeps the pending Quick Connect requests in memory: a new
// device starts one and shows its code, a signed-in user authorizes the
// code, and the device claims its session with the request's secret.
type QuickConnect struct {
	mu       sync.Mutex
	byCode   map[string]*QuickConnectRequest
	bySecret map[string]*QuickConnectRequest
}

// QuickConnectRequest is a device waiting to be signed in.
type QuickConnectRequest struct {
	Secret, Code string
	Device       Device
	Expires      time.Time
	// Authorized is set once a user signed the device in; Session is its
	// session, claimed once.
	Authorized bool
	Session    core.AuthSession
}

// NewQuickConnect returns an empty set of requests.
func NewQuickConnect() *QuickConnect {
	return &QuickConnect{byCode: map[string]*QuickConnectRequest{}, bySecret: map[string]*QuickConnectRequest{}}
}

// expire drops the requests past their time; the caller holds mu.
func (q *QuickConnect) expire(now time.Time) {
	for code, r := range q.byCode {
		if !now.Before(r.Expires) {
			delete(q.byCode, code)
			delete(q.bySecret, r.Secret)
		}
	}
}

// Start registers a device waiting to be signed in, with a random secret
// and a six-digit code unique among the pending requests.
func (q *QuickConnect) Start(d Device) QuickConnectRequest {
	secret := make([]byte, 32)
	_, _ = rand.Read(secret)
	now := time.Now()
	q.mu.Lock()
	defer q.mu.Unlock()
	q.expire(now)
	var code string
	for code == "" || q.byCode[code] != nil {
		n, _ := rand.Int(rand.Reader, big.NewInt(900_000))
		code = fmt.Sprintf("%06d", 100_000+n.Int64())
	}
	r := &QuickConnectRequest{Secret: hex.EncodeToString(secret), Code: code, Device: d, Expires: now.Add(QuickConnectTimeout)}
	q.byCode[code], q.bySecret[r.Secret] = r, r
	return *r
}

// State returns a pending request by its secret.
func (q *QuickConnect) State(secret string) (QuickConnectRequest, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.expire(time.Now())
	r := q.bySecret[secret]
	if r == nil {
		return QuickConnectRequest{}, fmt.Errorf("quick connect request: %w", core.ErrNotFound)
	}
	return *r, nil
}

// Authorize signs the device waiting with code in through signIn, which
// creates its session. The request then waits a minute for its device to
// claim the session. An authorized code is ErrConflict.
func (q *QuickConnect) Authorize(code string, signIn func(Device) (core.AuthSession, error)) (Device, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.expire(time.Now())
	r := q.byCode[code]
	switch {
	case r == nil:
		return Device{}, fmt.Errorf("quick connect code: %w", core.ErrNotFound)
	case r.Authorized:
		return Device{}, fmt.Errorf("quick connect code already authorized: %w", core.ErrConflict)
	}
	sess, err := signIn(r.Device)
	if err != nil {
		return Device{}, err
	}
	r.Authorized, r.Session = true, sess
	r.Expires = time.Now().Add(QuickConnectClaimTime)
	return r.Device, nil
}

// Claim returns the session of an authorized request, once.
func (q *QuickConnect) Claim(secret string) (QuickConnectRequest, error) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.expire(time.Now())
	r := q.bySecret[secret]
	if r == nil || !r.Authorized {
		return QuickConnectRequest{}, fmt.Errorf("authorized quick connect request: %w", core.ErrNotFound)
	}
	delete(q.byCode, r.Code)
	delete(q.bySecret, secret)
	return *r, nil
}

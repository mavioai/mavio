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
}

// QuickConnect keeps the pending Quick Connect requests in memory: a new
// device starts one and shows its code, a signed-in user authorizes the
// code, and the device claims its access token with the request's secret.
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
	// Authorized is set once a user signed the device in; Token is its
	// access token, claimed once.
	Authorized bool
	Token      string
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
// creates its session and returns its access token. The request then
// waits a minute for its device to claim the token. An authorized code is
// ErrConflict.
func (q *QuickConnect) Authorize(code string, signIn func(Device) (core.AuthSession, string, error)) (Device, error) {
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
	sess, token, err := signIn(r.Device)
	if err != nil {
		return Device{}, err
	}
	r.Authorized, r.Token, r.Session = true, token, sess
	r.Expires = time.Now().Add(QuickConnectClaimTime)
	return r.Device, nil
}

// Claim returns the access token of an authorized request, once.
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

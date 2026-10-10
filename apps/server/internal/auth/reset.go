package auth

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Password reset limits.
const (
	// ResetTimeout is how long a PIN resets its user's password.
	ResetTimeout = 30 * time.Minute
	// ResetAttempts is how many wrong PINs a reset takes before it ends.
	ResetAttempts = 5
)

// PasswordResets keeps the pending password resets in memory: the hash of
// each user's latest PIN until it is used, expires or is guessed wrong
// too often.
type PasswordResets struct {
	mu      sync.Mutex
	pending map[core.ID]*passwordReset
	now     func() time.Time
}

type passwordReset struct {
	hash     string
	expires  time.Time
	attempts int
}

// NewPasswordResets returns an empty set of resets.
func NewPasswordResets() *PasswordResets {
	return &PasswordResets{pending: map[core.ID]*passwordReset{}, now: time.Now}
}

// expire drops the resets past their time; the caller holds mu.
func (r *PasswordResets) expire(now time.Time) {
	for id, p := range r.pending {
		if !now.Before(p.expires) {
			delete(r.pending, id)
		}
	}
}

// Start begins a reset of a user's password, replacing any pending one,
// and returns its eight-digit PIN and when it expires.
func (r *PasswordResets) Start(userID core.ID) (pin string, expires time.Time) {
	n, _ := rand.Int(rand.Reader, big.NewInt(100_000_000))
	pin = fmt.Sprintf("%08d", n.Int64())
	hash := HashPassword(pin)
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()
	r.expire(now)
	expires = now.Add(ResetTimeout)
	r.pending[userID] = &passwordReset{hash: hash, expires: expires}
	return pin, expires
}

// Cancel ends a user's pending reset.
func (r *PasswordResets) Cancel(userID core.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.pending, userID)
}

// Redeem reports whether pin is the PIN of a user's pending reset, which
// it then ends; a wrong PIN counts against the reset.
func (r *PasswordResets) Redeem(userID core.ID, pin string) bool {
	r.mu.Lock()
	r.expire(r.now())
	p := r.pending[userID]
	r.mu.Unlock()
	if p == nil {
		SpendPasswordCheck(pin)
		return false
	}
	ok, _, _ := VerifyPassword(p.hash, pin)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.pending[userID] != p {
		// Redeemed, replaced or ended meanwhile.
		return false
	}
	if ok {
		delete(r.pending, userID)
		return true
	}
	if p.attempts++; p.attempts >= ResetAttempts {
		delete(r.pending, userID)
	}
	return false
}

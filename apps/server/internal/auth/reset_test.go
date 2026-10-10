package auth

import (
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestPasswordResets(t *testing.T) {
	now := time.Now()
	r := NewPasswordResets()
	r.now = func() time.Time { return now }
	ann, bob := core.NewID(), core.NewID()

	pin, expires := r.Start(ann)
	if len(pin) != 8 || !expires.Equal(now.Add(ResetTimeout)) {
		t.Errorf("Start() = %q, %v", pin, expires)
	}
	if r.Redeem(bob, pin) {
		t.Error("another user redeemed the PIN")
	}
	if !r.Redeem(ann, pin) {
		t.Error("the PIN did not redeem")
	}
	if r.Redeem(ann, pin) {
		t.Error("the PIN redeemed twice")
	}

	// A new PIN replaces the old one.
	old, _ := r.Start(ann)
	pin, _ = r.Start(ann)
	if old != pin && r.Redeem(ann, old) {
		t.Error("a replaced PIN redeemed")
	}

	// Wrong PINs end the reset.
	for range ResetAttempts {
		r.Redeem(ann, "wrong")
	}
	if r.Redeem(ann, pin) {
		t.Error("the PIN redeemed after too many wrong ones")
	}

	// PINs expire.
	pin, _ = r.Start(ann)
	now = now.Add(ResetTimeout)
	if r.Redeem(ann, pin) {
		t.Error("an expired PIN redeemed")
	}
}

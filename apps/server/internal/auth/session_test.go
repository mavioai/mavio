package auth

import (
	"crypto/ed25519"
	"errors"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestAccessTokens(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	tokens := NewAccessTokens([]byte("key"))
	tokens.now = func() time.Time { return now }
	session := core.NewID()

	token, expires := tokens.Issue(session)
	if want := now.Add(AccessTokenTTL); !expires.Equal(want) {
		t.Errorf("expires = %v, want = %v", expires, want)
	}
	if got, err := tokens.Verify(token); err != nil || got != session {
		t.Errorf("Verify = %v, %v, want = %v", got, err, session)
	}

	other := NewAccessTokens([]byte("other key"))
	other.now = tokens.now
	if _, err := other.Verify(token); !errors.Is(err, errBadToken) {
		t.Errorf("Verify with another key = %v, want errBadToken", err)
	}
	b := []byte(token)
	b[10] ^= 1
	if _, err := tokens.Verify(string(b)); err == nil {
		t.Error("Verify(altered token) = nil, want an error")
	}
	apiKey, _ := NewToken()
	if _, err := tokens.Verify(apiKey); !errors.Is(err, errNotAccessToken) {
		t.Errorf("Verify(API key) = %v, want errNotAccessToken", err)
	}

	now = now.Add(AccessTokenTTL)
	if _, err := tokens.Verify(token); !errors.Is(err, ErrTokenExpired) {
		t.Errorf("Verify after an hour = %v, want ErrTokenExpired", err)
	}
}

func TestNonces(t *testing.T) {
	now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.UTC)
	n := NewNonces()
	n.now = func() time.Time { return now }
	a, b := core.NewID(), core.NewID()

	va, _ := n.Start(a)
	vb, _ := n.Start(b)
	if n.Redeem(a, vb) {
		t.Error("Redeem with another session's nonce = true")
	}
	if n.Redeem(a, va) {
		t.Error("Redeem after a wrong guess = true; want the nonce gone")
	}
	if !n.Redeem(b, vb) {
		t.Error("Redeem = false, want true")
	}
	if n.Redeem(b, vb) {
		t.Error("Redeem replayed = true, want false")
	}

	vc, _ := n.Start(a)
	now = now.Add(NonceTTL)
	if n.Redeem(a, vc) {
		t.Error("Redeem of an expired nonce = true")
	}
}

func TestVerifyRefresh(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(nil)
	session, nonce := core.NewID(), []byte("nonce")
	sig := ed25519.Sign(private, RefreshMessage(session, nonce))
	tests := []struct {
		name    string
		key     []byte
		session core.ID
		nonce   []byte
		want    bool
	}{
		{"signed", public, session, nonce, true},
		{"other session", public, core.NewID(), nonce, false},
		{"other nonce", public, session, []byte("other"), false},
		{"short key", public[:8], session, nonce, false},
	}
	for _, tt := range tests {
		if got := VerifyRefresh(tt.key, tt.session, tt.nonce, sig); got != tt.want {
			t.Errorf("%s: VerifyRefresh = %v, want = %v", tt.name, got, tt.want)
		}
	}
}

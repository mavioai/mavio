package auth

import (
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestTOTPCodes(t *testing.T) {
	// RFC 6238 Appendix B, SHA-1, the last six of its eight digits.
	secret := []byte("12345678901234567890")
	tests := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
	}
	for _, tt := range tests {
		if got := totpCode(secret, tt.unix/30); got != tt.want {
			t.Errorf("code at %d: got = %s, want = %s", tt.unix, got, tt.want)
		}
	}
}

func TestTOTP(t *testing.T) {
	now := time.Date(2026, 10, 1, 12, 0, 10, 0, time.UTC)
	totp := NewTOTP(make([]byte, 32))
	totp.now = func() time.Time { return now }
	alice, bob := core.NewID(), core.NewID()
	codeAt := func(secret string, at time.Time) string {
		raw, err := base32Secret(secret)
		if err != nil {
			t.Fatal(err)
		}
		return totpCode(raw, at.Unix()/30)
	}

	secret := totp.Enroll(alice)
	if len(secret) != 32 || strings.ContainsAny(secret, "=") {
		t.Fatalf("secret = %q", secret)
	}
	if _, _, ok := totp.Confirm(alice, "000000"); ok && codeAt(secret, now) != "000000" {
		t.Error("Confirm accepted a wrong code")
	}
	if _, _, ok := totp.Confirm(bob, codeAt(secret, now)); ok {
		t.Error("Confirm accepted another user's enrollment")
	}
	sealed, used, ok := totp.Confirm(alice, codeAt(secret, now))
	if !ok || !used.Equal(now.Truncate(TOTPPeriod)) {
		t.Fatalf("Confirm = %q, %v, %v", sealed, used, ok)
	}
	if _, _, ok := totp.Confirm(alice, codeAt(secret, now)); ok {
		t.Error("Confirm accepted a code after the enrollment ended")
	}

	tests := []struct {
		name     string
		user     core.ID
		at       time.Time
		lastUsed *time.Time
		ok       bool
	}{
		{"current period", alice, now, nil, true},
		{"previous period", alice, now.Add(-TOTPPeriod), nil, true},
		{"next period", alice, now.Add(TOTPPeriod), nil, true},
		{"two periods ago", alice, now.Add(-2 * TOTPPeriod), nil, false},
		{"used period", alice, now, &used, false},
		{"after the used period", alice, now.Add(TOTPPeriod), &used, true},
		{"another user", bob, now, nil, false},
	}
	for _, tt := range tests {
		_, ok, err := totp.Verify(tt.user, sealed, codeAt(secret, tt.at), tt.lastUsed)
		if tt.user != alice {
			if err == nil {
				t.Errorf("%s: secret opened for another user", tt.name)
			}
			continue
		}
		if err != nil || ok != tt.ok {
			t.Errorf("%s: Verify = %v, %v, want = %v", tt.name, ok, err, tt.ok)
		}
	}

	// Enrollments expire.
	secret = totp.Enroll(alice)
	now = now.Add(TOTPEnrollmentTTL)
	if _, _, ok := totp.Confirm(alice, codeAt(secret, now)); ok {
		t.Error("Confirm accepted an expired enrollment")
	}
}

func TestTOTPURI(t *testing.T) {
	got := TOTPURI("Mavio", "alice", "ABC")
	want := "otpauth://totp/Mavio:alice?algorithm=SHA1&digits=6&issuer=Mavio&period=30&secret=ABC"
	if got != want {
		t.Errorf("got = %s, want = %s", got, want)
	}
}

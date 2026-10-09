package auth

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

func TestPassword(t *testing.T) {
	hash := HashPassword("correct horse")
	if !strings.HasPrefix(hash, "$argon2id$v=19$m=19456,t=2,p=1$") {
		t.Errorf("HashPassword = %q", hash)
	}
	if other := HashPassword("correct horse"); other == hash {
		t.Error("two hashes of one password are equal; want different salts")
	}

	// A hash with older parameters still verifies and asks to be replaced.
	salt := []byte("0123456789abcdef")
	old := fmt.Sprintf("$argon2id$v=19$m=8192,t=1,p=2$%s$%s", base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(argon2.IDKey([]byte("correct horse"), salt, 1, 8192, 2, 32)))

	tests := []struct {
		name, hash, password string
		ok, rehash           bool
	}{
		{"match", hash, "correct horse", true, false},
		{"mismatch", hash, "battery staple", false, false},
		{"empty password", hash, "", false, false},
		{"old parameters", old, "correct horse", true, true},
		{"old parameters mismatch", old, "battery", false, true},
	}
	for _, tt := range tests {
		ok, rehash, err := VerifyPassword(tt.hash, tt.password)
		if err != nil || ok != tt.ok || rehash != tt.rehash {
			t.Errorf("%s: VerifyPassword = %v, %v, %v; want %v, %v, nil", tt.name, ok, rehash, err, tt.ok, tt.rehash)
		}
	}
}

func TestVerifyPasswordMalformed(t *testing.T) {
	for _, hash := range []string{
		"",
		"plain",
		"$argon2i$v=19$m=19456,t=2,p=1$c2FsdHNhbHQ$aGFzaA",
		"$argon2id$v=16$m=19456,t=2,p=1$c2FsdHNhbHQ$aGFzaA",
		"$argon2id$v=19$m=0,t=2,p=1$c2FsdHNhbHQ$aGFzaA",
		"$argon2id$v=19$m=19456,t=2$c2FsdHNhbHQ$aGFzaA",
		"$argon2id$v=19$m=19456,t=2,p=1$!!$aGFzaA",
		"$argon2id$v=19$m=19456,t=2,p=1$c2FsdHNhbHQ$",
	} {
		if ok, _, err := VerifyPassword(hash, "x"); ok || err == nil {
			t.Errorf("VerifyPassword(%q) = %v, %v; want false, error", hash, ok, err)
		}
	}
}

func TestNewToken(t *testing.T) {
	a, hashA := NewToken()
	b, _ := NewToken()
	if len(a) != 43 || a == b {
		t.Errorf("tokens %q, %q; want two different 43-character tokens", a, b)
	}
	if got := HashToken(a); string(got) != string(hashA) || len(got) != 32 {
		t.Errorf("HashToken = %x, want %x", got, hashA)
	}
}

func TestBearer(t *testing.T) {
	tests := []struct {
		header, want string
		ok           bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{"Bearer  abc ", "abc", true},
		{"Bearer ", "", false},
		{"Basic abc", "", false},
		{"abc", "", false},
		{"", "", false},
	}
	for _, tt := range tests {
		h := make(map[string][]string)
		if tt.header != "" {
			h["Authorization"] = []string{tt.header}
		}
		got, ok := bearer(h)
		if got != tt.want || ok != tt.ok {
			t.Errorf("bearer(%q) = %q, %v; want %q, %v", tt.header, got, ok, tt.want, tt.ok)
		}
	}
}

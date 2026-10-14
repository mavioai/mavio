package auth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// SecretKeyPurpose derives the key that seals TOTP secrets from the
// server's identity key.
const SecretKeyPurpose = "mavio credential secrets v1"

// TOTP parameters: RFC 6238's defaults, which authenticator apps assume.
const (
	// TOTPPeriod is how long a code is valid; the codes of the periods
	// before and after are accepted too, for clocks that drift.
	TOTPPeriod = 30 * time.Second
	// TOTPEnrollmentTTL is how long an enrollment waits for its first code.
	TOTPEnrollmentTTL = 10 * time.Minute

	totpDigits    = 6
	totpSecretLen = 20
)

var errBadSecret = errors.New("unreadable TOTP secret")

// TOTP checks time-based one-time passwords (RFC 6238: HMAC-SHA1, six
// digits, 30-second periods), the second factor of a password. Their
// secrets are stored sealed with AES-GCM under a key derived from the
// server's identity key, bound to their user, so that a copy of the
// database cannot make codes. A code is used once: a period no later than
// the last one used is refused.
type TOTP struct {
	aead cipher.AEAD
	now  func() time.Time

	mu      sync.Mutex
	pending map[core.ID]enrollment
}

type enrollment struct {
	secret  []byte
	expires time.Time
}

// NewTOTP returns TOTP sealing secrets with a 32-byte key.
func NewTOTP(key []byte) *TOTP {
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err) // only for key lengths AES does not take
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return &TOTP{aead: aead, now: time.Now, pending: map[core.ID]enrollment{}}
}

// Enroll starts setting up TOTP for user, replacing an enrollment it
// started before, and returns the new secret in base32, as authenticator
// apps take it.
func (t *TOTP) Enroll(user core.ID) string {
	secret := make([]byte, totpSecretLen)
	_, _ = rand.Read(secret)
	now := t.now()
	t.mu.Lock()
	defer t.mu.Unlock()
	for id, e := range t.pending {
		if !now.Before(e.expires) {
			delete(t.pending, id)
		}
	}
	t.pending[user] = enrollment{secret: secret, expires: now.Add(TOTPEnrollmentTTL)}
	return encodeTOTPSecret(secret)
}

// Confirm ends user's enrollment with a code of its secret, returning the
// sealed secret to store and the period the code used; false when the
// code is wrong or no enrollment is pending.
func (t *TOTP) Confirm(user core.ID, code string) (string, time.Time, bool) {
	t.mu.Lock()
	e, ok := t.pending[user]
	t.mu.Unlock()
	now := t.now()
	if !ok || !now.Before(e.expires) {
		return "", time.Time{}, false
	}
	used, ok := matchTOTP(e.secret, code, now, nil)
	if !ok {
		return "", time.Time{}, false
	}
	t.mu.Lock()
	delete(t.pending, user)
	t.mu.Unlock()
	return t.seal(user, e.secret), used, true
}

// Verify checks a code of user's sealed secret, refusing the periods up
// to lastUsed, and returns the period it used.
func (t *TOTP) Verify(user core.ID, sealed, code string, lastUsed *time.Time) (time.Time, bool, error) {
	secret, err := t.open(user, sealed)
	if err != nil {
		return time.Time{}, false, err
	}
	used, ok := matchTOTP(secret, code, t.now(), lastUsed)
	return used, ok, nil
}

// TOTPURI returns the otpauth URI of a secret, which authenticator apps
// read from a QR code.
func TOTPURI(issuer, account, secret string) string {
	u := url.URL{Scheme: "otpauth", Host: "totp", Path: "/" + issuer + ":" + account}
	u.RawQuery = url.Values{
		"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"},
		"digits": {fmt.Sprint(totpDigits)}, "period": {fmt.Sprint(int(TOTPPeriod / time.Second))},
	}.Encode()
	return u.String()
}

// matchTOTP finds the period around now, after lastUsed, whose code is
// code.
func matchTOTP(secret []byte, code string, now time.Time, lastUsed *time.Time) (time.Time, bool) {
	step := now.Unix() / int64(TOTPPeriod/time.Second)
	for _, s := range []int64{step, step - 1, step + 1} {
		at := time.Unix(s*int64(TOTPPeriod/time.Second), 0)
		if lastUsed != nil && !at.After(*lastUsed) {
			continue
		}
		if hmac.Equal([]byte(totpCode(secret, s)), []byte(code)) {
			return at, true
		}
	}
	return time.Time{}, false
}

// totpCode is the code of a period (RFC 4226 §5.3).
func totpCode(secret []byte, step int64) string {
	h := hmac.New(sha1.New, secret)
	_ = binary.Write(h, binary.BigEndian, step)
	sum := h.Sum(nil)
	offset := sum[len(sum)-1] & 0x0f
	n := binary.BigEndian.Uint32(sum[offset:]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1_000_000)
}

func encodeTOTPSecret(secret []byte) string {
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret)
}

func (t *TOTP) seal(user core.ID, secret []byte) string {
	nonce := make([]byte, t.aead.NonceSize())
	_, _ = rand.Read(nonce)
	return base64.RawStdEncoding.EncodeToString(t.aead.Seal(nonce, nonce, secret, user[:]))
}

func (t *TOTP) open(user core.ID, sealed string) ([]byte, error) {
	b, err := base64.RawStdEncoding.DecodeString(sealed)
	if err != nil || len(b) < t.aead.NonceSize() {
		return nil, errBadSecret
	}
	secret, err := t.aead.Open(nil, b[:t.aead.NonceSize()], b[t.aead.NonceSize():], user[:])
	if err != nil {
		return nil, errBadSecret
	}
	return secret, nil
}

func base32Secret(s string) ([]byte, error) {
	return base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(s)
}

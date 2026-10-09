package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/argon2"
)

// argon2id parameters for new hashes: OWASP's recommended minimum, cheap
// enough for small home servers.
const (
	argonMemory  = 19 * 1024 // KiB
	argonTime    = 2
	argonThreads = 1
	argonKeyLen  = 32
	argonSaltLen = 16
)

var errMalformedHash = errors.New("malformed argon2id hash")

// HashPassword hashes password with argon2id into a PHC string.
func HashPassword(password string) string {
	salt := make([]byte, argonSaltLen)
	_, _ = rand.Read(salt) // never fails
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s", argon2.Version, argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt), base64.RawStdEncoding.EncodeToString(key))
}

// VerifyPassword reports whether password matches the PHC string hash, and
// whether hash uses other parameters than HashPassword and should be
// replaced.
func VerifyPassword(hash, password string) (ok, rehash bool, err error) {
	var p struct {
		version, memory, time int
		threads               uint8
	}
	parts := strings.Split(hash, "$")
	if len(parts) != 6 || parts[0] != "" || parts[1] != "argon2id" {
		return false, false, errMalformedHash
	}
	if _, err := fmt.Sscanf(parts[2], "v=%d", &p.version); err != nil || p.version != argon2.Version {
		return false, false, errMalformedHash
	}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.memory, &p.time, &p.threads); err != nil ||
		p.memory <= 0 || p.time <= 0 || p.threads == 0 {
		return false, false, errMalformedHash
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, false, errMalformedHash
	}
	want, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false, false, errMalformedHash
	}
	got := argon2.IDKey([]byte(password), salt, uint32(p.time), uint32(p.memory), p.threads, uint32(len(want)))
	ok = subtle.ConstantTimeCompare(got, want) == 1
	rehash = p.memory != argonMemory || p.time != argonTime || p.threads != argonThreads ||
		len(want) != argonKeyLen || len(salt) != argonSaltLen
	return ok, rehash, nil
}

// dummyHash is checked against when signing in with an unknown name, so
// that it costs as much as a wrong password.
var dummyHash = sync.OnceValue(func() string { return HashPassword("") })

// SpendPasswordCheck costs as much as VerifyPassword with a current hash.
func SpendPasswordCheck(password string) {
	_, _, _ = VerifyPassword(dummyHash(), password)
}

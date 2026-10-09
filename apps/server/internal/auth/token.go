package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
)

// NewToken returns a new access token and the hash to store for it.
func NewToken() (token string, hash []byte) {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never fails
	token = base64.RawURLEncoding.EncodeToString(b)
	return token, HashToken(token)
}

// HashToken returns the SHA-256 hash stored for token.
func HashToken(token string) []byte {
	h := sha256.Sum256([]byte(token))
	return h[:]
}

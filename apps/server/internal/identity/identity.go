package identity

import (
	"crypto/ed25519"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Key is the server's identity key pair.
type Key struct {
	private ed25519.PrivateKey
}

// Generate returns a new key, kept nowhere.
func Generate() *Key {
	_, private, _ := ed25519.GenerateKey(rand.Reader) // never fails
	return &Key{private: private}
}

// Load reads the key in file, a PEM-encoded PKCS #8 Ed25519 private key,
// creating it, readable by the owner only, when it does not exist.
func Load(file string) (*Key, error) {
	data, err := os.ReadFile(file)
	if errors.Is(err, fs.ErrNotExist) {
		return create(file)
	}
	if err != nil {
		return nil, fmt.Errorf("read identity key: %w", err)
	}
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "PRIVATE KEY" {
		return nil, fmt.Errorf("identity key %s: not a PEM private key", file)
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("identity key %s: %w", file, err)
	}
	private, ok := parsed.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("identity key %s: not an Ed25519 key", file)
	}
	return &Key{private: private}, nil
}

func create(file string) (*Key, error) {
	k := Generate()
	der, err := x509.MarshalPKCS8PrivateKey(k.private)
	if err != nil {
		return nil, fmt.Errorf("encode identity key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(file), 0o750); err != nil {
		return nil, fmt.Errorf("create identity key: %w", err)
	}
	// O_EXCL keeps a key another process created meanwhile.
	f, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return Load(file)
	}
	if err != nil {
		return nil, fmt.Errorf("create identity key: %w", err)
	}
	err = pem.Encode(f, &pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(file)
		return nil, fmt.Errorf("write identity key: %w", err)
	}
	return k, nil
}

// Public returns the public key.
func (k *Key) Public() ed25519.PublicKey {
	return k.private.Public().(ed25519.PublicKey)
}

// Sign signs message.
func (k *Key) Sign(message []byte) []byte {
	return ed25519.Sign(k.private, message)
}

// Derive returns a 32-byte secret for one purpose, derived from the
// private key with HKDF-SHA256, so that one key serves several uses
// without their secrets revealing one another.
func (k *Key) Derive(purpose string) []byte {
	out, err := hkdf.Key(sha256.New, k.private.Seed(), nil, purpose, 32)
	if err != nil {
		panic(err) // only for lengths HKDF cannot produce
	}
	return out
}

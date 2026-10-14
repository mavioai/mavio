package cloud

import (
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// JWKSPath is the path of mavio.dev's key set under the API's base URL.
const JWKSPath = "/auth/jwks"

// Key set caching (docs/accounts.md §10).
const (
	// keysLifetime is how long keys are kept when the response does not
	// say.
	keysLifetime = time.Hour
	// refetchInterval is how often an unknown key ID refetches the keys.
	refetchInterval = time.Minute
)

// ErrUnknownKey is returned for a key ID mavio.dev does not publish.
var ErrUnknownKey = errors.New("unknown key")

// KeySet caches mavio.dev's Ed25519 signing keys, refetching them when
// they expire and, at most once a minute, for a key ID it does not know.
type KeySet struct {
	url string
	hc  *http.Client
	now func() time.Time

	mu      sync.Mutex
	keys    map[string]ed25519.PublicKey
	expires time.Time
	fetched time.Time
}

// NewKeySet returns the key set at base, the API's base URL; a nil hc
// uses a client with a 30-second timeout.
func NewKeySet(base string, hc *http.Client) *KeySet {
	if hc == nil {
		hc = &http.Client{Timeout: 30 * time.Second}
	}
	return &KeySet{url: strings.TrimSuffix(base, "/") + JWKSPath, hc: hc, now: time.Now}
}

// Key returns the key with an ID.
func (k *KeySet) Key(ctx context.Context, id string) (ed25519.PublicKey, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	now := k.now()
	fresh := now.Before(k.expires)
	if key, ok := k.keys[id]; ok && fresh {
		return key, nil
	}
	if fresh && now.Sub(k.fetched) < refetchInterval {
		return nil, fmt.Errorf("%w %q", ErrUnknownKey, id)
	}
	if err := k.fetch(ctx, now); err != nil {
		return nil, err
	}
	if key, ok := k.keys[id]; ok {
		return key, nil
	}
	return nil, fmt.Errorf("%w %q", ErrUnknownKey, id)
}

func (k *KeySet) fetch(ctx context.Context, now time.Time) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, k.url, nil)
	if err != nil {
		return fmt.Errorf("fetch mavio.dev's keys: %w", err)
	}
	resp, err := k.hc.Do(req)
	if err != nil {
		return fmt.Errorf("fetch mavio.dev's keys: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch mavio.dev's keys: %s", resp.Status)
	}
	var set struct {
		Keys []struct {
			KeyType string `json:"kty"`
			Curve   string `json:"crv"`
			X       string `json:"x"`
			ID      string `json:"kid"`
		} `json:"keys"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&set); err != nil {
		return fmt.Errorf("decode mavio.dev's keys: %w", err)
	}
	keys := map[string]ed25519.PublicKey{}
	for _, key := range set.Keys {
		if key.KeyType != "OKP" || key.Curve != "Ed25519" {
			continue
		}
		x, err := base64.RawURLEncoding.DecodeString(key.X)
		if err != nil || len(x) != ed25519.PublicKeySize {
			continue
		}
		keys[key.ID] = ed25519.PublicKey(x)
	}
	k.keys, k.fetched, k.expires = keys, now, now.Add(maxAge(resp.Header.Get("Cache-Control")))
	return nil
}

// maxAge returns the max-age of a Cache-Control header, keysLifetime
// without one.
func maxAge(header string) time.Duration {
	for d := range strings.SplitSeq(header, ",") {
		if v, ok := strings.CutPrefix(strings.TrimSpace(d), "max-age="); ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				return time.Duration(n) * time.Second
			}
		}
	}
	return keysLifetime
}

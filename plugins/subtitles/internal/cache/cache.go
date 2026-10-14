// Package cache keeps what providers look up on their sites, such as the
// IDs of titles and search results, for a while and across restarts in
// the provider's data folder, as Bazarr's region cache on its file
// backend.
package cache

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
	"github.com/mavioai/mavio/plugins/subtitles/internal/sitekit"
)

// fileName is the name of a region's file in the data folder.
const fileName = "cache"

// Region is a provider's cache of values by key, each until it expires.
// A Region without a data folder keeps its values in memory only.
type Region struct {
	env provider.Env

	mu      sync.Mutex
	loaded  bool
	entries map[string]entry
}

// entry is a cached value, as JSON.
type entry struct {
	Value   json.RawMessage `json:"value"`
	Expires time.Time       `json:"expires"`
}

// New returns the region of a provider's environment.
func New(env provider.Env) *Region {
	return &Region{env: env, entries: map[string]entry{}}
}

// load reads the region's file once, leaving expired values out; a
// file that cannot be read is logged and starts the region empty.
func (r *Region) load(ctx context.Context, now time.Time) {
	if r.loaded {
		return
	}
	r.loaded = true
	var entries map[string]entry
	ok, err := sitekit.Load(r.env.DataDir, fileName, &entries)
	if err != nil {
		r.env.Log.WarnContext(ctx, "the provider's cache could not be read", "error", err)
		return
	}
	if !ok {
		return
	}
	for k, e := range entries {
		if now.Before(e.Expires) {
			r.entries[k] = e
		}
	}
}

// Get returns the value cached under key; false when there is none or
// it expired.
func Get[T any](ctx context.Context, r *Region, key string) (T, bool) {
	var v T
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.env.Now()
	r.load(ctx, now)
	e, ok := r.entries[key]
	if !ok || !now.Before(e.Expires) {
		return v, false
	}
	if err := json.Unmarshal(e.Value, &v); err != nil {
		return v, false
	}
	return v, true
}

// Set caches a value under key for ttl, and saves the region with its
// expired values left out; failing to save is logged, the value staying
// cached in memory.
func Set[T any](ctx context.Context, r *Region, key string, v T, ttl time.Duration) {
	data, err := json.Marshal(v)
	if err != nil {
		r.env.Log.WarnContext(ctx, "a value could not be cached", "key", key, "error", err)
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	now := r.env.Now()
	r.load(ctx, now)
	for k, e := range r.entries {
		if !now.Before(e.Expires) {
			delete(r.entries, k)
		}
	}
	r.entries[key] = entry{Value: data, Expires: now.Add(ttl)}
	if err := sitekit.Save(r.env.DataDir, fileName, r.entries); err != nil {
		r.env.Log.WarnContext(ctx, "the provider's cache could not be saved", "error", err)
	}
}

// Delete forgets the value cached under key.
func Delete(ctx context.Context, r *Region, key string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.load(ctx, r.env.Now())
	if _, ok := r.entries[key]; !ok {
		return
	}
	delete(r.entries, key)
	if err := sitekit.Save(r.env.DataDir, fileName, r.entries); err != nil {
		r.env.Log.WarnContext(ctx, "the provider's cache could not be saved", "error", err)
	}
}

// Cached returns the value cached under key, or computes and caches it
// for ttl, as Bazarr's cache_on_arguments; failures are not cached.
func Cached[T any](ctx context.Context, r *Region, key string, ttl time.Duration, f func() (T, error)) (T, error) {
	if v, ok := Get[T](ctx, r, key); ok {
		return v, nil
	}
	v, err := f()
	if err != nil {
		return v, err
	}
	Set(ctx, r, key, v, ttl)
	return v, nil
}

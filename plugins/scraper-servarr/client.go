package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/plugin/guest"
)

// errNotFound is a service's answer for unknown IDs.
var errNotFound = errors.New("not found")

// client calls one of Servarr's services.
type client struct {
	baseURL   string
	userAgent func() string
}

// get GETs path with query and decodes the JSON answer into out.
func (c *client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

// post POSTs in as JSON and decodes the JSON answer into out.
func (c *client) post(ctx context.Context, path string, in, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, in, out)
}

func (c *client) do(ctx context.Context, method, path string, query url.Values, in, out any) error {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	var body io.Reader
	if in != nil {
		data, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent())
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := guest.HTTPClient().Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", u, err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return fmt.Errorf("%s: %w", u, errNotFound)
	case resp.StatusCode != http.StatusOK:
		msg, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: %s: %s", u, resp.Status, strings.TrimSpace(string(msg)))
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("%s: %w", u, err)
	}
	return nil
}

// number is a JSON number that SkyHook may write as a string.
type number float64

func (n *number) UnmarshalJSON(data []byte) error {
	s := strings.Trim(string(data), `"`)
	if s == "" || s == "null" {
		*n = 0
		return nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return fmt.Errorf("number %s: %w", data, err)
	}
	*n = number(f)
	return nil
}

// cache keeps a few values for a while.
type cache[K comparable, V any] struct {
	mu      sync.Mutex
	entries map[K]cached[V]
}

type cached[V any] struct {
	value   V
	expires time.Time
}

// cacheSize and cacheTTL bound the cache.
const (
	cacheSize = 16
	cacheTTL  = 10 * time.Minute
)

func (c *cache[K, V]) get(k K, now time.Time) (V, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[k]
	if !ok || now.After(e.expires) {
		var zero V
		return zero, false
	}
	return e.value, true
}

func (c *cache[K, V]) put(k K, v V, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil {
		c.entries = map[K]cached[V]{}
	}
	if len(c.entries) >= cacheSize {
		for k, e := range c.entries {
			if now.After(e.expires) || len(c.entries) >= cacheSize {
				delete(c.entries, k)
			}
		}
	}
	c.entries[k] = cached[V]{value: v, expires: now.Add(cacheTTL)}
}

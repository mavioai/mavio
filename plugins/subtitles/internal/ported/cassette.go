package ported

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"go.yaml.in/yaml/v3"
)

// cassette is a vcrpy recording of a test's requests and responses.
type cassette struct {
	Interactions []struct {
		Request struct {
			Method string `yaml:"method"`
			URI    string `yaml:"uri"`
		} `yaml:"request"`
		Response struct {
			Status struct {
				Code    int    `yaml:"code"`
				Message string `yaml:"message"`
			} `yaml:"status"`
			Headers map[string][]string `yaml:"headers"`
			Body    struct {
				String string `yaml:"string"`
			} `yaml:"body"`
		} `yaml:"response"`
	} `yaml:"interactions"`
}

// replay answers requests from a cassette, as vcrpy's default matching:
// by method and URL, each recorded answer once, in order.
type replay struct {
	t    *testing.T
	name string

	mu   sync.Mutex
	c    cassette
	used []bool
}

// Cassette returns a transport replaying a cassette of
// testdata/responses/cassettes, such as "test_napiprojekt/test_list_subtitles_movie.yaml".
// A request it has no answer for fails the test.
func Cassette(t *testing.T, name string) http.RoundTripper {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(Dir(), "responses", "cassettes", name))
	if err != nil {
		t.Fatal(err)
	}
	r := &replay{t: t, name: name}
	if err := yaml.Unmarshal(data, &r.c); err != nil {
		t.Fatalf("cassette %s: %v", name, err)
	}
	r.used = make([]bool, len(r.c.Interactions))
	return r
}

func (r *replay) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, in := range r.c.Interactions {
		if r.used[i] || !strings.EqualFold(in.Request.Method, req.Method) || !sameURL(in.Request.URI, req.URL) {
			continue
		}
		r.used[i] = true
		body := []byte(in.Response.Body.String)
		h := http.Header{}
		for k, v := range in.Response.Headers {
			h[http.CanonicalHeaderKey(k)] = v
		}
		if strings.EqualFold(h.Get("Content-Encoding"), "gzip") {
			zr, err := gzip.NewReader(bytes.NewReader(body))
			if err != nil {
				return nil, fmt.Errorf("cassette %s: %w", r.name, err)
			}
			if body, err = io.ReadAll(zr); err != nil {
				return nil, fmt.Errorf("cassette %s: %w", r.name, err)
			}
			h.Del("Content-Encoding")
			h.Del("Content-Length")
		}
		return &http.Response{
			StatusCode: in.Response.Status.Code, Status: fmt.Sprintf("%d %s", in.Response.Status.Code, in.Response.Status.Message),
			Header: h, Body: io.NopCloser(bytes.NewReader(body)), ContentLength: int64(len(body)), Request: req,
			Proto: "HTTP/1.1", ProtoMajor: 1, ProtoMinor: 1,
		}, nil
	}
	r.t.Errorf("cassette %s: no answer for %s %s", r.name, req.Method, req.URL)
	return nil, fmt.Errorf("cassette %s: no answer for %s %s", r.name, req.Method, req.URL)
}

// sameURL compares URLs as vcrpy: scheme, host, port, path and query,
// the query's parameters in any order.
func sameURL(recorded string, u *url.URL) bool {
	ru, err := url.Parse(recorded)
	if err != nil {
		return false
	}
	return ru.Scheme == u.Scheme && ru.Host == u.Host && ru.Path == u.Path && ru.Query().Encode() == u.Query().Encode()
}

package containers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/mavioai/mavio/libs/core"
)

// API versions.
const (
	// MinAPIVersion is the oldest Engine API an engine must offer:
	// Docker 20.10 and Podman 3.
	MinAPIVersion = "1.41"
	// maxAPIVersion is the newest Engine API requests are made in; an
	// engine offering a newer one answers in this one.
	maxAPIVersion = "1.47"
)

// ErrAPIVersion reports an engine older than MinAPIVersion.
var ErrAPIVersion = errors.New("the container engine's API is older than Mavio requires")

// Client calls a container engine's API.
type Client struct {
	ep   Endpoint
	http *http.Client

	mu  sync.Mutex
	api string // the negotiated API version, empty until negotiated
}

// NewClient returns a client of the engine at ep. The API version is
// negotiated with the engine on the first request that needs it.
func NewClient(ep Endpoint) *Client {
	return &Client{ep: ep, http: &http.Client{Transport: &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return ep.dial(ctx)
		},
		MaxIdleConnsPerHost: 4,
	}}}
}

// Endpoint returns the engine's endpoint.
func (c *Client) Endpoint() Endpoint {
	return c.ep
}

// APIVersion returns the API version requests are made in, empty until
// negotiated.
func (c *Client) APIVersion() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.api
}

// Close closes the client's idle connections.
func (c *Client) Close() {
	c.http.CloseIdleConnections()
}

// Error is an error the engine returned.
type Error struct {
	Method, Path string
	Status       int
	Message      string
}

func (e *Error) Error() string {
	return fmt.Sprintf("container engine: %s %s: %d %s", e.Method, e.Path, e.Status, e.Message)
}

// Unwrap returns core.ErrNotFound for 404, core.ErrConflict for 409 and
// core.ErrInvalid for 400.
func (e *Error) Unwrap() error {
	switch e.Status {
	case http.StatusNotFound:
		return core.ErrNotFound
	case http.StatusConflict:
		return core.ErrConflict
	case http.StatusBadRequest:
		return core.ErrInvalid
	}
	return nil
}

// Version is what an engine tells of itself.
type Version struct {
	// Engine is "docker" or "podman".
	Engine string
	// Version is the engine's release, APIVersion the newest Engine API
	// it offers and MinAPIVersion the oldest.
	Version, APIVersion, MinAPIVersion string
	OS, Arch                           string
}

// Ping checks that the engine answers.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.send(ctx, http.MethodGet, "/_ping", nil, nil)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
}

// Version asks the engine what it is.
func (c *Client) Version(ctx context.Context) (Version, error) {
	var v struct {
		Version       string
		APIVersion    string `json:"ApiVersion"`
		MinAPIVersion string `json:"MinAPIVersion"`
		Os, Arch      string
		Components    []struct{ Name string }
	}
	resp, err := c.send(ctx, http.MethodGet, "/version", nil, nil)
	if err != nil {
		return Version{}, err
	}
	if err := decode(resp, &v); err != nil {
		return Version{}, err
	}
	engine := "docker"
	for _, comp := range v.Components {
		if strings.HasPrefix(comp.Name, "Podman") {
			engine = "podman"
		}
	}
	return Version{Engine: engine, Version: v.Version, APIVersion: v.APIVersion, MinAPIVersion: v.MinAPIVersion, OS: v.Os, Arch: v.Arch}, nil
}

// Negotiate settles the API version requests are made in: the engine's
// newest, at most the newest Mavio speaks, and at least the engine's
// oldest. An engine older than MinAPIVersion is ErrAPIVersion.
func (c *Client) Negotiate(ctx context.Context) (Version, error) {
	v, err := c.Version(ctx)
	if err != nil {
		return Version{}, err
	}
	api, err := negotiate(v)
	if err != nil {
		return v, err
	}
	c.mu.Lock()
	c.api = api
	c.mu.Unlock()
	return v, nil
}

// negotiate picks the API version requests to an engine are made in.
func negotiate(v Version) (string, error) {
	if v.APIVersion == "" || compareVersions(v.APIVersion, MinAPIVersion) < 0 {
		return "", fmt.Errorf("%w: it offers API %q, Mavio requires %s", ErrAPIVersion, v.APIVersion, MinAPIVersion)
	}
	api := v.APIVersion
	if compareVersions(api, maxAPIVersion) > 0 {
		api = maxAPIVersion
	}
	if v.MinAPIVersion != "" && compareVersions(api, v.MinAPIVersion) < 0 {
		api = v.MinAPIVersion
	}
	return api, nil
}

// compareVersions compares API versions such as "1.41" by their numbers.
func compareVersions(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := range max(len(as), len(bs)) {
		var x, y int
		if i < len(as) {
			x, _ = strconv.Atoi(as[i])
		}
		if i < len(bs) {
			y, _ = strconv.Atoi(bs[i])
		}
		if x != y {
			return x - y
		}
	}
	return 0
}

// apiVersion returns the negotiated API version, negotiating it first.
func (c *Client) apiVersion(ctx context.Context) (string, error) {
	c.mu.Lock()
	api := c.api
	c.mu.Unlock()
	if api != "" {
		return api, nil
	}
	if _, err := c.Negotiate(ctx); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.api, nil
}

// do sends a request of the negotiated API version: body, when not nil,
// as JSON. A response other than 2xx or 304 is an *Error.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	api, err := c.apiVersion(ctx)
	if err != nil {
		return nil, err
	}
	return c.send(ctx, method, "/v"+api+path, query, body)
}

// call sends a request as do does and decodes the response's JSON into
// out, when not nil.
func (c *Client) call(ctx context.Context, method, path string, query url.Values, body, out any) error {
	resp, err := c.do(ctx, method, path, query, body)
	if err != nil {
		return err
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		return resp.Body.Close()
	}
	return decode(resp, out)
}

// send sends a request of the path as given.
func (c *Client) send(ctx context.Context, method, path string, query url.Values, body any) (*http.Response, error) {
	var r io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("container engine: %s %s: %w", method, path, err)
		}
		r = bytes.NewReader(data)
	}
	u := url.URL{Scheme: "http", Host: "engine", Path: path, RawQuery: query.Encode()}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), r)
	if err != nil {
		return nil, fmt.Errorf("container engine: %s %s: %w", method, path, err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("container engine %s: %w", c.ep, err)
	}
	if resp.StatusCode/100 == 2 || resp.StatusCode == http.StatusNotModified {
		return resp, nil
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	var msg struct{ Message string }
	if json.Unmarshal(data, &msg) != nil || msg.Message == "" {
		msg.Message = strings.TrimSpace(string(data))
	}
	return nil, &Error{Method: method, Path: path, Status: resp.StatusCode, Message: msg.Message}
}

// decode decodes a response's JSON into out and closes its body.
func decode(resp *http.Response, out any) error {
	defer func() { _ = resp.Body.Close() }()
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("container engine: %s %s: %w", resp.Request.Method, resp.Request.URL.Path, err)
	}
	return nil
}

// labelFilters returns the filters query parameter selecting what carries
// every label, or nil when there are none.
func labelFilters(labels map[string]string) url.Values {
	if len(labels) == 0 {
		return nil
	}
	set := make(map[string]bool, len(labels))
	for k, v := range labels {
		set[k+"="+v] = true
	}
	data, _ := json.Marshal(map[string]map[string]bool{"label": set})
	return url.Values{"filters": {string(data)}}
}

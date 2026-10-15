package rclone

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync/atomic"
	"time"
)

// Version is the release of mavioai/rclone this Mavio is built for, the one
// mise.toml and the container image pin. Mavio relies on the fork's changes
// to the rcd, so an rcd of another release, upstream rclone's above all,
// is refused.
const Version = "v1.75.2-mavio"

// ErrVersion reports an rcd of another rclone release than the one
// required.
var ErrVersion = errors.New("rclone is not the release Mavio requires")

// Endpoint is where an rcd listens and the credentials it takes.
type Endpoint struct {
	// URL is the rcd's address: http(s)://host:port, or unix:///path for
	// a Unix socket.
	URL string
	// User and Password are the rcd's --rc-user and --rc-pass; empty for
	// an rcd without authentication.
	User, Password string
}

// Client calls an rcd's rc API.
type Client struct {
	ep   Endpoint
	base *url.URL
	http *http.Client
	// poll is the period of job status checks.
	poll time.Duration
	// require is the rclone release the rcd must run, empty for any;
	// verified is set once the rcd was seen running it.
	require  string
	verified atomic.Bool
}

// Require makes every request first check, until it once succeeds, that
// the rcd runs the release version, failing with ErrVersion otherwise.
// It is called before the client is used.
func (c *Client) Require(version string) {
	c.require = version
}

// CheckVersion checks that the rcd runs the release Require set.
func (c *Client) CheckVersion(ctx context.Context) error {
	if c.require == "" || c.verified.Load() {
		return nil
	}
	v, err := c.Version(ctx)
	if err != nil {
		return err
	}
	if v != c.require {
		return fmt.Errorf("%w: the rcd runs %s, Mavio requires %s of github.com/mavioai/rclone", ErrVersion, v, c.require)
	}
	c.verified.Store(true)
	return nil
}

// maxIdlePerHost is how many idle connections to the rcd are kept: every
// read of a media range is a request of its own, and with Go's default of
// two, concurrent reads would open a new connection for most of them.
const maxIdlePerHost = 16

// NewClient returns a client of the rcd at ep; transport carries requests
// to TCP addresses, a copy of http.DefaultTransport keeping more idle
// connections when nil.
func NewClient(ep Endpoint, transport http.RoundTripper) (*Client, error) {
	u, err := url.Parse(ep.URL)
	if err != nil {
		return nil, fmt.Errorf("rclone address: %w", err)
	}
	c := &Client{ep: ep, poll: 500 * time.Millisecond}
	switch u.Scheme {
	case "unix":
		socket := u.Path
		c.base = &url.URL{Scheme: "http", Host: "rclone"}
		c.http = &http.Client{Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", socket)
			},
			MaxIdleConnsPerHost: maxIdlePerHost,
		}}
	case "http", "https":
		u.Path = strings.TrimSuffix(u.Path, "/")
		c.base = u
		if transport == nil {
			t := http.DefaultTransport.(*http.Transport).Clone()
			t.MaxIdleConnsPerHost = maxIdlePerHost
			transport = t
		}
		c.http = &http.Client{Transport: transport}
	default:
		return nil, fmt.Errorf("rclone address %q: want http, https or unix", ep.URL)
	}
	return c, nil
}

// Error is an error the rc API returned.
type Error struct {
	Method  string
	Status  int
	Message string
	// RetryAfter is how long the rcd asks to wait before trying again,
	// for statuses 429 and 503.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	return fmt.Sprintf("rclone %s: %s (status %d)", e.Method, e.Message, e.Status)
}

// Unwrap gives fs.ErrNotExist for objects and folders not found, and
// ErrChanged for objects whose version changed.
func (e *Error) Unwrap() error {
	switch e.Status {
	case http.StatusNotFound:
		return fs.ErrNotExist
	case http.StatusPreconditionFailed:
		return ErrChanged
	}
	return nil
}

// Call calls an rc method with the JSON of in, decoding its answer into
// out unless out is nil.
func (c *Client) Call(ctx context.Context, method string, in, out any) error {
	if err := c.CheckVersion(ctx); err != nil {
		return err
	}
	return c.call(ctx, method, in, out)
}

// call calls an rc method whatever release the rcd runs.
func (c *Client) call(ctx context.Context, method string, in, out any) error {
	body, err := json.Marshal(in)
	if err != nil {
		return err
	}
	req, err := c.newRequest(ctx, http.MethodPost, c.methodURL(method, nil), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	return c.do(req, method, out)
}

func (c *Client) methodURL(method string, query url.Values) string {
	u := *c.base
	u.Path += "/" + method
	u.RawQuery = query.Encode()
	return u.String()
}

// request makes a request of the rcd, once it runs the release required.
func (c *Client) request(ctx context.Context, method, u string, body io.Reader) (*http.Request, error) {
	if err := c.CheckVersion(ctx); err != nil {
		return nil, err
	}
	return c.newRequest(ctx, method, u, body)
}

func (c *Client) newRequest(ctx context.Context, method, u string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if c.ep.User != "" || c.ep.Password != "" {
		req.SetBasicAuth(c.ep.User, c.ep.Password)
	}
	return req, nil
}

func (c *Client) do(req *http.Request, method string, out any) error {
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("rclone %s: %w", method, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return fmt.Errorf("rclone %s: %w", method, err)
	}
	if resp.StatusCode != http.StatusOK {
		var e struct {
			Error  string `json:"error"`
			Status int    `json:"status"`
		}
		if json.Unmarshal(data, &e) != nil || e.Error == "" {
			e.Error = strings.TrimSpace(string(data))
		}
		if e.Status == 0 {
			e.Status = resp.StatusCode
		}
		return &Error{Method: method, Status: e.Status, Message: e.Error}
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return fmt.Errorf("rclone %s: %w", method, err)
	}
	return nil
}

// Noop checks that the rcd answers, with the credentials when it asks for
// them.
func (c *Client) Noop(ctx context.Context) error {
	method := "rc/noop"
	if c.ep.User != "" {
		method = "rc/noopauth"
	}
	return c.call(ctx, method, struct{}{}, nil)
}

// Version returns the rcd's rclone version.
func (c *Client) Version(ctx context.Context) (string, error) {
	var out struct {
		Version string `json:"version"`
	}
	err := c.call(ctx, "core/version", struct{}{}, &out)
	return out.Version, err
}

// JobStatus is the state of an rc job, as job/status gives it.
type JobStatus struct {
	ID        int64   `json:"id"`
	ExecuteID string  `json:"executeId"`
	Finished  bool    `json:"finished"`
	Success   bool    `json:"success"`
	Error     string  `json:"error"`
	Duration  float64 `json:"duration"`
}

// Start starts an rc method as a job and returns its ID and the rcd's
// execution ID, which tells the rcd's runs apart: job IDs start again when
// it restarts.
func (c *Client) Start(ctx context.Context, method string, in map[string]any) (JobStatus, error) {
	params := map[string]any{"_async": true}
	for k, v := range in {
		params[k] = v
	}
	var out struct {
		JobID     int64  `json:"jobid"`
		ExecuteID string `json:"executeId"`
	}
	if err := c.Call(ctx, method, params, &out); err != nil {
		return JobStatus{}, err
	}
	return JobStatus{ID: out.JobID, ExecuteID: out.ExecuteID}, nil
}

// Job reads the state of a job.
func (c *Client) Job(ctx context.Context, id int64) (JobStatus, error) {
	var st JobStatus
	err := c.Call(ctx, "job/status", map[string]any{"jobid": id}, &st)
	return st, err
}

// StopJob stops a job.
func (c *Client) StopJob(ctx context.Context, id int64) error {
	return c.Call(ctx, "job/stop", map[string]any{"jobid": id}, nil)
}

// ErrJobLost is returned for a job of an rcd run that ended: the rcd
// restarted and forgot it.
var ErrJobLost = errors.New("rclone job lost: the rcd restarted")

// Wait waits for a job of the rcd run executeID to finish, stopping it when
// ctx ends.
func (c *Client) Wait(ctx context.Context, job JobStatus) error {
	t := time.NewTicker(c.poll)
	defer t.Stop()
	for {
		st, err := c.Job(ctx, job.ID)
		switch {
		case ctx.Err() != nil:
			_ = c.StopJob(context.WithoutCancel(ctx), job.ID)
			return ctx.Err()
		case jobNotFound(err) || (err == nil && job.ExecuteID != "" && st.ExecuteID != job.ExecuteID):
			return ErrJobLost
		case err != nil:
			return err
		case st.Finished && st.Success:
			return nil
		case st.Finished:
			return &Error{Method: "job", Status: http.StatusInternalServerError, Message: st.Error}
		}
		select {
		case <-ctx.Done():
		case <-t.C:
		}
	}
}

// jobNotFound reports whether err says the rcd knows no such job.
func jobNotFound(err error) bool {
	var e *Error
	return errors.As(err, &e) && strings.Contains(e.Message, "job not found")
}

// Run starts a method as a job and waits for it.
func (c *Client) Run(ctx context.Context, method string, in map[string]any) error {
	job, err := c.Start(ctx, method, in)
	if err != nil {
		return err
	}
	return c.Wait(ctx, job)
}

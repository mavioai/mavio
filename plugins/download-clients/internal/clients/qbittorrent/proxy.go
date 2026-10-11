package qbittorrent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// torrent is a torrent of the list of torrents.
type torrent struct {
	Hash     string  `json:"hash"`
	Name     string  `json:"name"`
	Size     int64   `json:"size"`
	Progress float64 `json:"progress"`
	// ETA is in seconds; qBittorrent may send values beyond 64 bits.
	ETA         json.Number `json:"eta"`
	State       string      `json:"state"`
	Label       string      `json:"label"`
	Category    string      `json:"category"`
	SavePath    string      `json:"save_path"`
	ContentPath string      `json:"content_path"`
	Ratio       float64     `json:"ratio"`
	// The limits are -2 for the global one and -1 for none; the times are
	// in minutes.
	RatioLimit               float64 `json:"ratio_limit"`
	SeedingTimeLimit         int64   `json:"seeding_time_limit"`
	InactiveSeedingTimeLimit int64   `json:"inactive_seeding_time_limit"`
	// SeedingTime is in seconds; older versions leave it out of the list.
	SeedingTime *int64 `json:"seeding_time"`
	// LastActivity is when a piece was last sent or received, in Unix
	// seconds.
	LastActivity int64 `json:"last_activity"`
}

// UnmarshalJSON reads a torrent, its limits the global ones when absent.
func (t *torrent) UnmarshalJSON(b []byte) error {
	type plain torrent
	p := plain{RatioLimit: -2, SeedingTimeLimit: -2, InactiveSeedingTimeLimit: -2}
	if err := json.Unmarshal(b, &p); err != nil {
		return err
	}
	*t = torrent(p)
	return nil
}

// The actions qBittorrent takes on torrents reaching their share limits
// that remove them.
const (
	ratioActionRemove      = 1
	ratioActionDeleteFiles = 3
)

// preferences are qBittorrent's preferences.
type preferences struct {
	SavePath                      string  `json:"save_path"`
	MaxRatioEnabled               bool    `json:"max_ratio_enabled"`
	MaxRatio                      float64 `json:"max_ratio"`
	MaxSeedingTimeEnabled         bool    `json:"max_seeding_time_enabled"`
	MaxSeedingTime                int64   `json:"max_seeding_time"`
	MaxInactiveSeedingTimeEnabled bool    `json:"max_inactive_seeding_time_enabled"`
	MaxInactiveSeedingTime        int64   `json:"max_inactive_seeding_time"`
	MaxRatioAction                int     `json:"max_ratio_act"`
	QueueingEnabled               bool    `json:"queueing_enabled"`
	DHT                           bool    `json:"dht"`
}

// properties are a torrent's general properties.
type properties struct {
	SavePath    string `json:"save_path"`
	SeedingTime int64  `json:"seeding_time"`
}

// file is a file of a torrent.
type file struct {
	Name string `json:"name"`
}

// category is a category and where its torrents go.
type category struct {
	Name     string `json:"name"`
	SavePath string `json:"savePath"`
}

// api is a version of qBittorrent's Web API, as Radarr's
// IQBittorrentProxy.
type api interface {
	// supported reports whether qBittorrent speaks the version.
	supported(ctx context.Context) (bool, error)
	apiVersion(ctx context.Context) ([]int, error)
	config(ctx context.Context) (*preferences, error)
	// torrents lists the torrents of a category; all for "".
	torrents(ctx context.Context, category string) ([]torrent, error)
	loaded(ctx context.Context, hash string) bool
	properties(ctx context.Context, hash string) (*properties, error)
	files(ctx context.Context, hash string) ([]file, error)
	addURL(ctx context.Context, link, category string, r *client.Release) error
	addFile(ctx context.Context, name string, data []byte, category string, r *client.Release) error
	remove(ctx context.Context, hash string, deleteData bool) error
	setCategory(ctx context.Context, hash, category string) error
	addCategory(ctx context.Context, category string) error
	categories(ctx context.Context) (map[string]category, error)
	// setShareLimits sets the seeding limits of a release.
	setShareLimits(ctx context.Context, hash string, r *client.Release) error
	moveToTop(ctx context.Context, hash string) error
	setForceStart(ctx context.Context, hash string, enabled bool) error
}

// request is a request to the Web API: a POST when it has a form or file.
type request struct {
	path  string
	query url.Values
	form  url.Values
	// file is uploaded as the multipart field "torrents".
	fileName string
	file     []byte
}

// conn sends requests to qBittorrent, signing in with a cookie kept until
// qBittorrent refuses it, as Radarr's proxies' authCookieCache.
type conn struct {
	http     *http.Client
	settings *settings
	// v2 is set for the Web API v2, which also takes API keys.
	v2 bool

	mu       sync.Mutex
	loggedIn bool
	cookies  []*http.Cookie
}

func blank(s string) bool { return strings.TrimSpace(s) == "" }

func (c *conn) apiKey() bool { return c.v2 && !blank(c.settings.APIKey) }

// send sends a request with cookies.
func (c *conn) send(ctx context.Context, r request, cookies []*http.Cookie) (*client.Response, error) {
	u := c.settings.BaseURL() + r.path
	if len(r.query) > 0 {
		u += "?" + r.query.Encode()
	}
	method, contentType := http.MethodGet, ""
	var body io.Reader
	switch {
	case r.file != nil:
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		for k, vs := range r.form {
			for _, v := range vs {
				if err := w.WriteField(k, v); err != nil {
					return nil, err
				}
			}
		}
		part, err := w.CreateFormFile("torrents", r.fileName)
		if err != nil {
			return nil, err
		}
		if _, err := part.Write(r.file); err != nil {
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		method, contentType, body = http.MethodPost, w.FormDataContentType(), &buf
	case r.form != nil:
		method, contentType, body = http.MethodPost, "application/x-www-form-urlencoded", strings.NewReader(r.form.Encode())
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	s := c.settings
	switch {
	case c.apiKey():
		req.Header.Set("Authorization", "Bearer "+s.APIKey)
	case !blank(s.Username) || !blank(s.Password):
		req.SetBasicAuth(s.Username, s.Password)
	}
	for _, ck := range cookies {
		req.AddCookie(&http.Cookie{Name: ck.Name, Value: ck.Value})
	}
	return client.Send(c.http, req)
}

// errConnect wraps a failure to talk to qBittorrent.
func errConnect(err error) error {
	return fmt.Errorf("failed to connect to qBittorrent, check your settings: %w", err)
}

// process sends a request signed in, signing in again when qBittorrent
// answers 403, and returns the answer's body.
func (c *conn) process(ctx context.Context, r request) ([]byte, error) {
	if c.apiKey() {
		resp, err := c.send(ctx, r, nil)
		if err != nil {
			return nil, errConnect(err)
		}
		if err := resp.Err(); err != nil {
			var auth *client.AuthError
			if errors.As(err, &auth) {
				return nil, &client.AuthError{Err: fmt.Errorf("failed to authenticate with qBittorrent: %w", auth.Err)}
			}
			return nil, errConnect(err)
		}
		return resp.Body, nil
	}
	cookies, err := c.authenticate(ctx, false)
	if err != nil {
		return nil, err
	}
	resp, err := c.send(ctx, r, cookies)
	if err != nil {
		return nil, errConnect(err)
	}
	if resp.StatusCode == http.StatusForbidden {
		if cookies, err = c.authenticate(ctx, true); err != nil {
			return nil, err
		}
		if resp, err = c.send(ctx, r, cookies); err != nil {
			return nil, errConnect(err)
		}
	}
	if err := resp.Err(); err != nil {
		return nil, errConnect(err)
	}
	return resp.Body, nil
}

// processJSON sends a request and decodes its answer into out.
func (c *conn) processJSON(ctx context.Context, r request, out any) error {
	body, err := c.process(ctx, r)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("qBittorrent: %s: %w", r.path, err)
	}
	return nil
}

// authenticate signs in, unless signed in already and renew is not set,
// and returns the session's cookies. Without a username and password it
// does not sign in.
func (c *conn) authenticate(ctx context.Context, renew bool) ([]*http.Cookie, error) {
	s := c.settings
	if blank(s.Username) || blank(s.Password) {
		if renew && c.v2 {
			return nil, &client.AuthError{Err: errors.New("failed to authenticate with qBittorrent")}
		}
		return nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loggedIn && !renew {
		return c.cookies, nil
	}
	c.loggedIn, c.cookies = false, nil
	path := "/login"
	if c.v2 {
		path = "/api/v2/auth/login"
	}
	resp, err := c.send(ctx, request{path: path, form: url.Values{"username": {s.Username}, "password": {s.Password}}}, nil)
	if err != nil {
		return nil, err
	}
	if err := resp.Err(); err != nil {
		var auth *client.AuthError
		if errors.As(err, &auth) {
			return nil, &client.AuthError{Err: fmt.Errorf("failed to authenticate with qBittorrent: %w", auth.Err)}
		}
		return nil, errConnect(err)
	}
	// qBittorrent answers "Fails." to a wrong password; v2 may answer
	// nothing on success.
	if content := string(resp.Body); content != "Ok." && (!c.v2 || !blank(content)) {
		return nil, &client.AuthError{Err: errors.New("failed to authenticate with qBittorrent")}
	}
	c.loggedIn, c.cookies = true, (&http.Response{Header: resp.Header}).Cookies()
	return c.cookies, nil
}

// probe asks without signing in for a resource only a version has: 404
// is another version, 403 this one asking to sign in.
func (c *conn) probe(ctx context.Context, path string) (bool, error) {
	resp, err := c.send(ctx, request{path: path}, nil)
	if err != nil {
		return false, errConnect(err)
	}
	switch resp.StatusCode {
	case http.StatusNotFound:
		return false, nil
	case http.StatusForbidden:
		return true, nil
	}
	if err := resp.Err(); err != nil {
		return false, errConnect(err)
	}
	return true, nil
}

// parseVersion reads a version as .NET's Version.Parse: two to four
// numbers.
func parseVersion(s string) ([]int, error) {
	parts := strings.Split(strings.TrimSpace(s), ".")
	if len(parts) < 2 || len(parts) > 4 {
		return nil, fmt.Errorf("unable to parse the qBittorrent API version %q", s)
	}
	out := make([]int, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil || n < 0 {
			return nil, fmt.Errorf("unable to parse the qBittorrent API version %q", s)
		}
		out[i] = n
	}
	return out, nil
}

// formatVersion writes a version back.
func formatVersion(v []int) string {
	s := make([]string, len(v))
	for i, n := range v {
		s[i] = strconv.Itoa(n)
	}
	return strings.Join(s, ".")
}

// failed checks the answer of adding a torrent: "Fails." is a failure,
// while older versions answer nothing on success.
func failed(body []byte, what string) error {
	if string(body) == "Fails." {
		return fmt.Errorf("download client failed to add torrent%s", what)
	}
	return nil
}

// isStatus reports whether err is qBittorrent answering a status.
func isStatus(err error, code int) bool {
	var h *client.HTTPError
	return errors.As(err, &h) && h.StatusCode == code
}

// v2 is the Web API v2 of qBittorrent 4.1 and later, as Radarr's
// QBittorrentProxyV2.
type v2 struct{ *conn }

func (p v2) supported(ctx context.Context) (bool, error) {
	return p.probe(ctx, "/api/v2/app/webapiVersion")
}

func (p v2) apiVersion(ctx context.Context) ([]int, error) {
	body, err := p.process(ctx, request{path: "/api/v2/app/webapiVersion"})
	if err != nil {
		return nil, err
	}
	return parseVersion(string(body))
}

func (p v2) config(ctx context.Context) (*preferences, error) {
	c := preferences{QueueingEnabled: true}
	return &c, p.processJSON(ctx, request{path: "/api/v2/app/preferences"}, &c)
}

func (p v2) torrents(ctx context.Context, category string) ([]torrent, error) {
	r := request{path: "/api/v2/torrents/info"}
	if !blank(category) {
		r.query = url.Values{"category": {category}}
	}
	var out []torrent
	return out, p.processJSON(ctx, r, &out)
}

func (p v2) loaded(ctx context.Context, hash string) bool {
	_, err := p.process(ctx, request{path: "/api/v2/torrents/properties", query: url.Values{"hash": {hash}}})
	return err == nil
}

func (p v2) properties(ctx context.Context, hash string) (*properties, error) {
	var out properties
	return &out, p.processJSON(ctx, request{path: "/api/v2/torrents/properties", query: url.Values{"hash": {hash}}}, &out)
}

func (p v2) files(ctx context.Context, hash string) ([]file, error) {
	var out []file
	return out, p.processJSON(ctx, request{path: "/api/v2/torrents/files", query: url.Values{"hash": {hash}}}, &out)
}

// addForm is the form of adding a torrent: its category, initial state,
// download order, content layout and seeding limits.
func (p v2) addForm(ctx context.Context, form url.Values, category string, r *client.Release) error {
	s := p.settings
	if !blank(category) {
		form.Set("category", category)
	}
	if s.InitialState == stateStart || s.InitialState == stateStop {
		v, err := p.apiVersion(ctx)
		if err != nil {
			return err
		}
		name := "paused"
		if atLeast(v, 2, 11, 0) {
			name = "stopped"
		}
		form.Set(name, strconv.FormatBool(s.InitialState == stateStop))
	}
	if s.SequentialOrder {
		form.Set("sequentialDownload", "true")
	}
	if s.FirstAndLast {
		form.Set("firstLastPiecePrio", "true")
	}
	switch s.ContentLayout {
	case layoutOriginal:
		form.Set("contentLayout", "Original")
	case layoutSubfolder:
		form.Set("contentLayout", "Subfolder")
	}
	if r != nil {
		shareLimits(form, r, false)
	}
	return nil
}

// shareLimits adds a release's seeding limits to a form; -2 for the
// global one, left out unless always is set.
func shareLimits(form url.Values, r *client.Release, always bool) {
	ratio, minutes := "-2", "-2"
	if r.SeedRatio != nil {
		ratio = strconv.FormatFloat(*r.SeedRatio, 'f', -1, 64)
	}
	if r.SeedTime != nil {
		minutes = strconv.FormatInt(int64(r.SeedTime.Minutes()), 10)
	}
	if ratio != "-2" || always {
		form.Set("ratioLimit", ratio)
	}
	if minutes != "-2" || always {
		form.Set("seedingTimeLimit", minutes)
	}
}

func (p v2) addURL(ctx context.Context, link, category string, r *client.Release) error {
	form := url.Values{"urls": {link}}
	if err := p.addForm(ctx, form, category, r); err != nil {
		return err
	}
	body, err := p.process(ctx, request{path: "/api/v2/torrents/add", form: form})
	if err != nil {
		return err
	}
	return failed(body, " by url")
}

func (p v2) addFile(ctx context.Context, name string, data []byte, category string, r *client.Release) error {
	form := url.Values{}
	if err := p.addForm(ctx, form, category, r); err != nil {
		return err
	}
	body, err := p.process(ctx, request{path: "/api/v2/torrents/add", form: form, fileName: name, file: data})
	if err != nil {
		return err
	}
	return failed(body, "")
}

func (p v2) remove(ctx context.Context, hash string, deleteData bool) error {
	form := url.Values{"hashes": {hash}}
	if deleteData {
		form.Set("deleteFiles", "true")
	}
	_, err := p.process(ctx, request{path: "/api/v2/torrents/delete", form: form})
	return err
}

func (p v2) setCategory(ctx context.Context, hash, category string) error {
	_, err := p.process(ctx, request{path: "/api/v2/torrents/setCategory", form: url.Values{"hashes": {hash}, "category": {category}}})
	return err
}

func (p v2) addCategory(ctx context.Context, category string) error {
	_, err := p.process(ctx, request{path: "/api/v2/torrents/createCategory", form: url.Values{"category": {category}}})
	return err
}

func (p v2) categories(ctx context.Context) (map[string]category, error) {
	var out map[string]category
	return out, p.processJSON(ctx, request{path: "/api/v2/torrents/categories"}, &out)
}

func (p v2) setShareLimits(ctx context.Context, hash string, r *client.Release) error {
	form := url.Values{"hashes": {hash}}
	shareLimits(form, r, true)
	_, err := p.process(ctx, request{path: "/api/v2/torrents/setShareLimits", form: form})
	if isStatus(err, http.StatusNotFound) {
		// setShareLimits came with API 2.0.1.
		return nil
	}
	return err
}

func (p v2) moveToTop(ctx context.Context, hash string) error {
	_, err := p.process(ctx, request{path: "/api/v2/torrents/topPrio", form: url.Values{"hashes": {hash}}})
	if isStatus(err, http.StatusConflict) {
		// qBittorrent rejects priorities with 409 when queueing is off.
		return nil
	}
	return err
}

func (p v2) setForceStart(ctx context.Context, hash string, enabled bool) error {
	_, err := p.process(ctx, request{path: "/api/v2/torrents/setForceStart", form: url.Values{"hashes": {hash}, "value": {strconv.FormatBool(enabled)}}})
	return err
}

// v1 is the Web API of qBittorrent before 4.1, as Radarr's
// QBittorrentProxyV1.
type v1 struct{ *conn }

func (p v1) supported(ctx context.Context) (bool, error) {
	return p.probe(ctx, "/version/api")
}

func (p v1) apiVersion(ctx context.Context) ([]int, error) {
	body, err := p.process(ctx, request{path: "/version/api"})
	if err != nil {
		return nil, err
	}
	return parseVersion("1." + strings.TrimSpace(string(body)))
}

func (p v1) config(ctx context.Context) (*preferences, error) {
	c := preferences{QueueingEnabled: true}
	return &c, p.processJSON(ctx, request{path: "/query/preferences"}, &c)
}

func (p v1) torrents(ctx context.Context, category string) ([]torrent, error) {
	r := request{path: "/query/torrents"}
	if !blank(category) {
		r.query = url.Values{"label": {category}, "category": {category}}
	}
	var out []torrent
	return out, p.processJSON(ctx, r, &out)
}

func (p v1) loaded(ctx context.Context, hash string) bool {
	_, err := p.process(ctx, request{path: "/query/propertiesGeneral/" + url.PathEscape(hash)})
	return err == nil
}

func (p v1) properties(ctx context.Context, hash string) (*properties, error) {
	var out properties
	return &out, p.processJSON(ctx, request{path: "/query/propertiesGeneral/" + url.PathEscape(hash)}, &out)
}

func (p v1) files(ctx context.Context, hash string) ([]file, error) {
	var out []file
	return out, p.processJSON(ctx, request{path: "/query/propertiesFiles/" + url.PathEscape(hash)}, &out)
}

// addForm is the form of adding a torrent: its category and whether it
// starts paused; v1 has no seeding limits.
func (p v1) addForm(form url.Values, category string) {
	if !blank(category) {
		form.Set("category", category)
	}
	switch p.settings.InitialState {
	case stateStart:
		form.Set("paused", "false")
	case stateStop:
		form.Set("paused", "true")
	}
}

func (p v1) addURL(ctx context.Context, link, category string, _ *client.Release) error {
	form := url.Values{"urls": {link}}
	p.addForm(form, category)
	body, err := p.process(ctx, request{path: "/command/download", form: form})
	if err != nil {
		return err
	}
	return failed(body, " by url")
}

func (p v1) addFile(ctx context.Context, name string, data []byte, category string, _ *client.Release) error {
	form := url.Values{}
	p.addForm(form, category)
	body, err := p.process(ctx, request{path: "/command/upload", form: form, fileName: name, file: data})
	if err != nil {
		return err
	}
	return failed(body, "")
}

func (p v1) remove(ctx context.Context, hash string, deleteData bool) error {
	path := "/command/delete"
	if deleteData {
		path = "/command/deletePerm"
	}
	_, err := p.process(ctx, request{path: path, form: url.Values{"hashes": {hash}}})
	return err
}

// setCategory sets a torrent's category, or its label before
// qBittorrent 3.3.5.
func (p v1) setCategory(ctx context.Context, hash, category string) error {
	_, err := p.process(ctx, request{path: "/command/setCategory", form: url.Values{"hashes": {hash}, "category": {category}}})
	if isStatus(err, http.StatusNotFound) {
		_, err = p.process(ctx, request{path: "/command/setLabel", form: url.Values{"hashes": {hash}, "label": {category}}})
	}
	return err
}

func (p v1) addCategory(ctx context.Context, category string) error {
	_, err := p.process(ctx, request{path: "/command/addCategory", form: url.Values{"category": {category}}})
	return err
}

func (p v1) categories(context.Context) (map[string]category, error) {
	return nil, fmt.Errorf("qBittorrent api v1 does not support getting all torrent categories: %w", client.ErrNotSupported)
}

func (p v1) setShareLimits(context.Context, string, *client.Release) error { return nil }

func (p v1) moveToTop(ctx context.Context, hash string) error {
	_, err := p.process(ctx, request{path: "/command/topPrio", form: url.Values{"hashes": {hash}}})
	if isStatus(err, http.StatusForbidden) {
		// qBittorrent rejects priorities with 403 when queueing is off.
		return nil
	}
	return err
}

func (p v1) setForceStart(ctx context.Context, hash string, enabled bool) error {
	_, err := p.process(ctx, request{path: "/command/setForceStart", form: url.Values{"hashes": {hash}, "value": {strconv.FormatBool(enabled)}}})
	return err
}

// selector picks the Web API qBittorrent speaks, kept 10 minutes, as
// Radarr's QBittorrentProxySelector.
type selector struct {
	apis []api
	now  func() time.Time

	mu       sync.Mutex
	api      api
	version  []int
	selected time.Time
}

// get returns the API and its version, asking again when force is set.
func (s *selector) get(ctx context.Context, force bool) (api, []int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if !force && s.api != nil && now.Sub(s.selected) < 10*time.Minute {
		return s.api, s.version, nil
	}
	s.api, s.version = nil, nil
	for _, a := range s.apis {
		ok, err := a.supported(ctx)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			continue
		}
		v, err := a.apiVersion(ctx)
		if err != nil {
			return nil, nil, err
		}
		s.api, s.version, s.selected = a, v, now
		return a, v, nil
	}
	return nil, nil, errors.New("unable to determine qBittorrent API version")
}

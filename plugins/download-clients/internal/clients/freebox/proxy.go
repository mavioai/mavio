package freebox

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // The Freebox API signs challenges with HMAC-SHA1.
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"sync"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// The descriptions of the API's error codes, as Radarr's FreeboxResponse.
var errorDescriptions = map[string]string{
	// Common errors
	"invalid_request":     "Your request is invalid.",
	"invalid_api_version": "Invalid API base url or unknown API version.",
	"internal_error":      "Internal error.",
	// Login API errors
	"auth_required":           "Invalid session token, or no session token sent.",
	"invalid_token":           "The app token you are trying to use is invalid or has been revoked.",
	"pending_token":           "The app token you are trying to use has not been validated by user yet.",
	"insufficient_rights":     "Your app permissions does not allow accessing this API.",
	"denied_from_external_ip": "You are trying to get an app_token from a remote IP.",
	"ratelimited":             "Too many auth error have been made from your IP.",
	"new_apps_denied":         "New application token request has been disabled.",
	"apps_denied":             "API access from apps has been disabled.",
	// Download API errors
	"task_not_found":       "No task was found with the given id.",
	"invalid_operation":    "Attempt to perform an invalid operation.",
	"invalid_file":         "Error with the download file (invalid format ?).",
	"invalid_url":          "URL is invalid.",
	"not_implemented":      "Method not implemented.",
	"out_of_memory":        "No more memory available to perform the requested action.",
	"invalid_task_type":    "The task type is invalid.",
	"hibernating":          "The downloader is hibernating.",
	"need_bt_stopped_done": "This action is only valid for Bittorrent task in stopped or done state.",
	"bt_tracker_not_found": "Attempt to access an invalid tracker object.",
	"too_many_tasks":       "Too many tasks.",
	"invalid_address":      "Invalid peer address.",
	"port_conflict":        "Port conflict when setting config.",
	"invalid_priority":     "Invalid priority.",
	"ctx_file_error":       "Failed to initialize task context file (need to check disk).",
	"exists":               "Same task already exists.",
	"port_outside_range":   "Incoming port is not available for this customer.",
}

func errorDescription(code string) string {
	if d, ok := errorDescriptions[code]; ok {
		return d
	}
	return code + " - Unknown error"
}

// response is the envelope of the API's answers.
type response struct {
	Success   bool            `json:"success"`
	Message   string          `json:"msg"`
	ErrorCode string          `json:"error_code"`
	Result    json.RawMessage `json:"result"`
}

// login is the result of the login API.
type login struct {
	LoggedIn     bool   `json:"logged_in"`
	Challenge    string `json:"challenge"`
	SessionToken string `json:"session_token"`
}

// taskID is a task's ID, which the API sends as a number.
type taskID string

// UnmarshalJSON implements json.Unmarshaler, taking strings and numbers.
func (id *taskID) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		*id = taskID(s)
		return nil
	}
	var n json.Number
	if err := json.Unmarshal(data, &n); err != nil {
		return fmt.Errorf("task id: %w", err)
	}
	*id = taskID(n.String())
	return nil
}

// task is a download task, as Radarr's FreeboxDownloadTask.
type task struct {
	ID          taskID  `json:"id"`
	Name        string  `json:"name"`
	DownloadDir string  `json:"download_dir"`
	InfoHash    string  `json:"info_hash"`
	Status      string  `json:"status"`
	ETA         int64   `json:"eta"`
	Error       string  `json:"error"`
	Type        string  `json:"type"`
	StopRatio   float64 `json:"stop_ratio"`
	Size        int64   `json:"size"`
	ReceivedPct int64   `json:"rx_pct"`
}

// The descriptions of the tasks' errors, as Radarr's FreeboxDownloadTask.
var taskErrors = map[string]string{
	"internal":                    "Internal error.",
	"disk_full":                   "The disk is full.",
	"unknown":                     "Unknown error.",
	"parse_error":                 "Parse error.",
	"unknown_host":                "Unknown host.",
	"timeout":                     "Timeout.",
	"bad_authentication":          "Invalid credentials.",
	"connection_refused":          "Remote host refused connection.",
	"bt_tracker_error":            "Unable to announce on tracker.",
	"bt_missing_files":            "Missing torrent files.",
	"bt_file_error":               "Error accessing torrent files.",
	"missing_ctx_file":            "Error accessing task context file.",
	"nzb_no_group":                "Cannot find the requested group on server.",
	"nzb_not_found":               "Article not found on the server.",
	"nzb_invalid_crc":             "Invalid article CRC.",
	"nzb_invalid_size":            "Invalid article size.",
	"nzb_invalid_filename":        "Invalid filename.",
	"nzb_open_failed":             "Error opening.",
	"nzb_write_failed":            "Error writing.",
	"nzb_missing_size":            "Missing article size.",
	"nzb_decode_error":            "Article decoding error.",
	"nzb_missing_segments":        "Missing article segments.",
	"nzb_error":                   "Other nzb error.",
	"nzb_authentication_required": "Nzb server need authentication.",
}

func (t *task) errorDescription() string {
	if d, ok := taskErrors[t.Error]; ok {
		return d
	}
	return t.Error + " - Unknown error"
}

// downloadDir returns the task's folder, which the API encodes in base64.
func (t *task) downloadDir() string { return decodeBase64(t.DownloadDir) }

func decodeBase64(s string) string {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return s
	}
	return string(b)
}

// errNotLoggedIn is the API telling the session is not logged in.
var errNotLoggedIn = errors.New("not logged in")

// proxy calls the Freebox API, as Radarr's FreeboxDownloadProxy: with a
// session token opened from the app token, kept until the API refuses it.
type proxy struct {
	http *http.Client
	s    *settings
	log  *slog.Logger

	mu    sync.Mutex
	token string
}

// request is a call of the API.
type request struct {
	method, resource string
	// body is sent with contentType; JSON by default.
	body        []byte
	contentType string
}

// sessionToken returns the session token, opening a session when there
// is none.
func (p *proxy) sessionToken(ctx context.Context) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" {
		return p.token, nil
	}
	p.log.DebugContext(ctx, "opening a Freebox API session", "app_id", p.s.AppID)
	var challenge login
	if err := p.send(ctx, request{method: http.MethodGet, resource: "/login"}, "", &challenge); err != nil {
		return "", err
	}
	mac := hmac.New(sha1.New, []byte(p.s.AppToken))
	mac.Write([]byte(challenge.Challenge))
	body, err := json.Marshal(map[string]string{"app_id": p.s.AppID, "password": hex.EncodeToString(mac.Sum(nil))})
	if err != nil {
		return "", err
	}
	var session login
	if err := p.send(ctx, request{method: http.MethodPost, resource: "/login/session", body: body}, "", &session); err != nil {
		return "", err
	}
	p.token = session.SessionToken
	return p.token, nil
}

func (p *proxy) clearToken(token string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token == token {
		p.token = ""
	}
}

// do calls the API with the session and decodes the result into out, when
// not nil. A session the API no longer takes is opened again once.
func (p *proxy) do(ctx context.Context, r request, out any) error {
	for retry := true; ; retry = false {
		token, err := p.sessionToken(ctx)
		if err != nil {
			return err
		}
		err = p.send(ctx, r, token, out)
		var auth *client.AuthError
		if !errors.As(err, &auth) || !retry {
			return err
		}
		p.log.DebugContext(ctx, "the Freebox API refused the session, opening another", "err", err)
	}
}

// send calls the API, as Radarr's ProcessRequest.
func (p *proxy) send(ctx context.Context, r request, token string, out any) error {
	var body io.Reader
	if r.body != nil {
		body = bytes.NewReader(r.body)
	}
	base := client.Server{Host: p.s.Host, Port: p.s.Port, UseSSL: p.s.UseSSL, URLBase: p.s.APIURL}.BaseURL()
	req, err := http.NewRequestWithContext(ctx, r.method, base+r.resource, body)
	if err != nil {
		return err
	}
	contentType := r.contentType
	if contentType == "" {
		contentType = "application/json"
	}
	req.Header.Set("Content-Type", contentType)
	if token != "" {
		req.Header.Set("X-Fbx-App-Auth", token)
	}
	resp, err := client.Send(p.http, req)
	var unavailable *client.UnavailableError
	if errors.As(err, &unavailable) {
		return &client.UnavailableError{Err: fmt.Errorf("unable to reach the Freebox API; verify the host, port and SSL settings (%w)", unavailable.Err)}
	} else if err != nil {
		return err
	}
	var answer response
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		if token != "" {
			p.clearToken(token)
		}
		_ = json.Unmarshal(resp.Body, &answer)
		msg := "authentication to the Freebox API failed. Reason: " + errorDescription(answer.ErrorCode)
		p.log.ErrorContext(ctx, msg)
		return &client.AuthError{Err: errors.New(msg)}
	case http.StatusNotFound:
		return errors.New("unable to reach the Freebox API; verify the API URL setting for its base URL and version")
	case http.StatusOK:
		if err := json.Unmarshal(resp.Body, &answer); err != nil {
			return fmt.Errorf("decode the Freebox API's answer: %w", err)
		}
		if !answer.Success {
			msg := "the Freebox API returned an error: " + errorDescription(answer.ErrorCode)
			p.log.ErrorContext(ctx, msg)
			return errors.New(msg)
		}
		if out != nil && len(answer.Result) > 0 && string(answer.Result) != "null" {
			if err := json.Unmarshal(answer.Result, out); err != nil {
				return fmt.Errorf("decode the Freebox API's answer: %w", err)
			}
		}
		return nil
	}
	return fmt.Errorf("unable to connect to the Freebox, please check your settings: %w", &client.HTTPError{StatusCode: resp.StatusCode})
}

// authenticate checks that the session is logged in.
func (p *proxy) authenticate(ctx context.Context) error {
	var l login
	if err := p.do(ctx, request{method: http.MethodGet, resource: "/login"}, &l); err != nil {
		return err
	}
	if !l.LoggedIn {
		return &client.AuthError{Err: errNotLoggedIn}
	}
	return nil
}

// addURL adds a task from a link to a folder, empty for the default.
func (p *proxy) addURL(ctx context.Context, link, dir string, opts addOptions) (string, error) {
	form := url.Values{"download_url": {urlPathEncode(link)}}
	if dir != "" {
		form.Set("download_dir", base64.StdEncoding.EncodeToString([]byte(dir)))
	}
	return p.add(ctx, request{
		method: http.MethodPost, resource: "/downloads/add",
		body: []byte(form.Encode()), contentType: "application/x-www-form-urlencoded",
	}, opts)
}

// addFile adds a task from a torrent file to a folder, empty for the
// default.
func (p *proxy) addFile(ctx context.Context, name string, data []byte, dir string, opts addOptions) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="download_file"; filename=%q`, name))
	h.Set("Content-Type", "multipart/form-data")
	part, err := w.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if dir != "" {
		if err := w.WriteField("download_dir", base64.StdEncoding.EncodeToString([]byte(dir))); err != nil {
			return "", err
		}
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return p.add(ctx, request{
		method: http.MethodPost, resource: "/downloads/add", body: body.Bytes(), contentType: w.FormDataContentType(),
	}, opts)
}

// addOptions are the settings of a task set once added.
type addOptions struct {
	paused, first bool
	// seedRatio is the ratio times 100; nil for the Freebox's own.
	seedRatio *float64
}

func (p *proxy) add(ctx context.Context, r request, opts addOptions) (string, error) {
	var t task
	if err := p.do(ctx, r, &t); err != nil {
		return "", err
	}
	body := map[string]any{}
	if opts.paused {
		body["status"] = "stopped"
	}
	if opts.first {
		body["queue_pos"] = "1"
	}
	if opts.seedRatio != nil {
		// 0 is unlimited seeding.
		body["stop_ratio"] = *opts.seedRatio
	}
	if len(body) > 0 {
		data, err := json.Marshal(body)
		if err != nil {
			return "", err
		}
		if err := p.do(ctx, request{method: http.MethodPut, resource: "/downloads/" + string(t.ID), body: data}, nil); err != nil {
			return "", err
		}
	}
	return string(t.ID), nil
}

// remove deletes a task, with its files when erase is set.
func (p *proxy) remove(ctx context.Context, id string, erase bool) error {
	resource := "/downloads/" + id
	if erase {
		resource += "/erase"
	}
	return p.do(ctx, request{method: http.MethodDelete, resource: resource}, nil)
}

// defaultDir returns the downloader's default folder.
func (p *proxy) defaultDir(ctx context.Context) (string, error) {
	var config struct {
		DownloadDir string `json:"download_dir"`
	}
	if err := p.do(ctx, request{method: http.MethodGet, resource: "/downloads/config/"}, &config); err != nil {
		return "", err
	}
	return decodeBase64(config.DownloadDir), nil
}

// tasks lists the tasks.
func (p *proxy) tasks(ctx context.Context) ([]task, error) {
	var tasks []task
	if err := p.do(ctx, request{method: http.MethodGet, resource: "/downloads/"}, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

// urlPathEncode encodes a URL as .NET's HttpUtility.UrlPathEncode: spaces,
// control and non-ASCII characters before its query.
func urlPathEncode(u string) string {
	p, query, hasQuery := strings.Cut(u, "?")
	var b strings.Builder
	for i := range len(p) {
		if c := p[i]; c <= ' ' || c >= 0x7f {
			fmt.Fprintf(&b, "%%%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	if hasQuery {
		b.WriteString("?" + query)
	}
	return b.String()
}

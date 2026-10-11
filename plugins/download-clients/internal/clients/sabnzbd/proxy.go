package sabnzbd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// apiError is an error SABnzbd answers with.
type apiError struct{ Message string }

func (e *apiError) Error() string {
	return "error response received from SABnzbd: " + e.Message
}

// proxy calls SABnzbd's API as Radarr's SabnzbdProxy.
type proxy struct {
	http     *http.Client
	settings *settings
}

// baseURL returns SABnzbd's address with a path under it.
func (p *proxy) baseURL(rel string) string {
	return p.settings.BaseURL() + "/" + strings.TrimLeft(rel, "/")
}

// requestURL returns the URL of a mode with its parameters, then the
// credentials and the output format, as Radarr's BuildRequest.
func (p *proxy) requestURL(mode string, params url.Values) string {
	q := url.Values{"mode": {mode}}
	for k, v := range params {
		q[k] = v
	}
	if strings.TrimSpace(p.settings.APIKey) != "" {
		q.Set("apikey", p.settings.APIKey)
	} else {
		q.Set("ma_username", p.settings.Username)
		q.Set("ma_password", p.settings.Password)
	}
	q.Set("output", "json")
	return p.baseURL("api") + "?" + q.Encode()
}

// call calls a mode and returns the answer, failing on HTTP errors and on
// the errors SABnzbd reports.
func (p *proxy) call(ctx context.Context, mode string, params url.Values, body io.Reader, contentType string) ([]byte, error) {
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, p.requestURL(mode, params), body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := client.Send(p.http, req)
	if err != nil {
		return nil, err
	}
	if err := resp.Err(); err != nil {
		return nil, fmt.Errorf("unable to connect to SABnzbd: %w", err)
	}
	if err := checkForError(resp.Body); err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// checkForError returns the error of an answer, as Radarr's CheckForError:
// a JSON status of false, or plain text starting with "error". Wrong or
// missing API keys are AuthErrors.
func checkForError(body []byte) error {
	var r struct {
		Status json.RawMessage `json:"status"`
		Error  flexString      `json:"error"`
	}
	var msg string
	if err := json.Unmarshal(body, &r); err == nil {
		if !strings.EqualFold(strings.TrimSpace(unquote(r.Status)), "false") {
			return nil
		}
		msg = string(r.Error)
	} else {
		content := string(body)
		if !strings.HasPrefix(strings.ToLower(content), "error") {
			return nil
		}
		msg = strings.TrimSpace(strings.ReplaceAll(content, "error: ", ""))
	}
	err := &apiError{Message: msg}
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "api key incorrect") || strings.Contains(lower, "api key required") ||
		strings.Contains(lower, "authentication") {
		return &client.AuthError{Err: err}
	}
	return err
}

// get calls a mode and decodes the answer into out, or the member key of
// it when key is set.
func (p *proxy) get(ctx context.Context, mode string, params url.Values, key string, out any) error {
	body, err := p.call(ctx, mode, params, nil, "")
	if err != nil {
		return err
	}
	if key != "" {
		var m map[string]json.RawMessage
		if err := json.Unmarshal(body, &m); err != nil {
			return fmt.Errorf("sabnzbd: %s: %w", mode, err)
		}
		body = m[key]
		if body == nil {
			return fmt.Errorf("sabnzbd: %s: no %s in the answer", mode, key)
		}
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("sabnzbd: %s: %w", mode, err)
	}
	return nil
}

// addFile uploads an NZB and returns the IDs SABnzbd gives it; none when
// the answer has none.
func (p *proxy) addFile(ctx context.Context, data []byte, fileName, category string, prio int) ([]string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="name"; filename=%q`, fileName))
	h.Set("Content-Type", "application/x-nzb")
	part, err := w.CreatePart(h)
	if err != nil {
		return nil, err
	}
	if _, err := part.Write(data); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	params := url.Values{"cat": {category}, "priority": {strconv.Itoa(prio)}}
	body, err := p.call(ctx, "addfile", params, &buf, w.FormDataContentType())
	if err != nil {
		return nil, err
	}
	var r struct {
		IDs []string `json:"nzo_ids"`
	}
	_ = json.Unmarshal(body, &r)
	return r.IDs, nil
}

// remove deletes a download from the queue or the history; from the
// history it is archived unless permanently is set.
func (p *proxy) remove(ctx context.Context, mode, id string, deleteData, permanently bool) error {
	params := url.Values{"name": {"delete"}, "del_files": {boolParam(deleteData)}, "value": {id}}
	if mode == "history" {
		params.Set("archive", boolParam(!permanently))
	}
	_, err := p.call(ctx, mode, params, nil, "")
	return err
}

func boolParam(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

// version returns SABnzbd's version; empty when it tells none.
func (p *proxy) version(ctx context.Context) (string, error) {
	body, err := p.call(ctx, "version", nil, nil, "")
	if err != nil {
		return "", err
	}
	var r struct {
		Version flexString `json:"version"`
	}
	_ = json.Unmarshal(body, &r)
	return string(r.Version), nil
}

func (p *proxy) config(ctx context.Context) (*config, error) {
	var c config
	if err := p.get(ctx, "get_config", nil, "config", &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// completeDir returns the resolved complete folder of SABnzbd 2.0 and
// later.
func (p *proxy) completeDir(ctx context.Context) (string, error) {
	var s struct {
		CompleteDir string `json:"completedir"`
	}
	if err := p.get(ctx, "fullstatus", url.Values{"skip_dashboard": {"1"}}, "status", &s); err != nil {
		return "", err
	}
	return s.CompleteDir, nil
}

// listParams are the parameters of the queue and the history.
func listParams(start, limit int, category string) url.Values {
	v := url.Values{"start": {strconv.Itoa(start)}, "limit": {strconv.Itoa(limit)}}
	if strings.TrimSpace(category) != "" {
		v.Set("category", category)
	}
	return v
}

func (p *proxy) queue(ctx context.Context, start, limit int, category string) (*queue, error) {
	var q queue
	if err := p.get(ctx, "queue", listParams(start, limit, category), "queue", &q); err != nil {
		return nil, err
	}
	return &q, nil
}

func (p *proxy) history(ctx context.Context, start, limit int, category string) (*history, error) {
	var h history
	if err := p.get(ctx, "history", listParams(start, limit, category), "history", &h); err != nil {
		return nil, err
	}
	return &h, nil
}

// isAPIKeyError reports whether err is SABnzbd refusing an API key, and
// which way.
func isAPIKeyError(err error) (string, bool) {
	var e *apiError
	if !errors.As(err, &e) {
		return "", false
	}
	lower := strings.ToLower(e.Message)
	switch {
	case strings.Contains(lower, "api key incorrect"):
		return "API Key Incorrect", true
	case strings.Contains(lower, "api key required"):
		return "API Key Required", true
	}
	return "", false
}

package nzbvortex

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// The states of a download.
const (
	stateWaiting                = 0
	stateDownloading            = 1
	statePasswordRequest        = 5
	stateDone                   = 20
	stateUncompressFailed       = 21
	stateCheckFailedDataCorrupt = 22
	stateBadlyEncoded           = 24
)

// queueItem is a download of the queue.
type queueItem struct {
	ID                int    `json:"id"`
	UITitle           string `json:"uiTitle"`
	DestinationPath   string `json:"destinationPath"`
	IsPaused          bool   `json:"isPaused"`
	State             int    `json:"state"`
	DownloadedSize    int64  `json:"downloadedSize"`
	TotalDownloadSize int64  `json:"totalDownloadSize"`
	AddUUID           string `json:"addUUID"`
	GroupName         string `json:"groupName"`
}

// file is a file of a download.
type file struct {
	ID       int    `json:"id"`
	FileName string `json:"fileName"`
}

// result is the result every response has.
type result struct {
	Result string `json:"result"`
}

// notLoggedIn reports whether the response tells the session expired, as
// Radarr's NzbVortexResultTypeConverter reads "notLoggedIn".
func (r *result) notLoggedIn() bool {
	return strings.EqualFold(strings.ReplaceAll(r.Result, "_", ""), "notloggedin")
}

func (r *result) base() *result { return r }

// response is any response.
type response interface{ base() *result }

// errAuth is NZBVortex refusing the API key.
var errAuth = &client.AuthError{Err: errors.New("authentication failed, check your API key")}

// proxy calls NZBVortex's API as Radarr's NzbVortexProxy.
type proxy struct {
	http     *http.Client
	settings *settings

	mu        sync.Mutex
	sessionID string
}

// url returns the address of an API resource with a query.
func (p *proxy) url(resource string, query url.Values) string {
	u := p.settings.server().BaseURL() + "/api/" + resource
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	return u
}

// get sends a GET request and decodes the answer into out.
func (p *proxy) get(ctx context.Context, resource string, query url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url(resource, query), nil)
	if err != nil {
		return err
	}
	return p.send(req, out)
}

func (p *proxy) send(req *http.Request, out any) error {
	req.Header.Set("Accept", "application/json")
	resp, err := client.Send(p.http, req)
	if err != nil {
		return err
	}
	if err := resp.Err(); err != nil {
		return fmt.Errorf("unable to connect to NZBVortex, please check your settings: %w", err)
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("NZBVortex response could not be processed: %w", err)
	}
	return nil
}

// session returns the session ID, logging in when there is none or renew
// is set.
func (p *proxy) session(ctx context.Context, renew bool) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sessionID != "" && !renew {
		return p.sessionID, nil
	}
	p.sessionID = ""
	var n struct {
		AuthNonce string `json:"authNonce"`
	}
	if err := p.get(ctx, "auth/nonce", nil, &n); err != nil {
		return "", err
	}
	cnonce := newUUID()
	sum := sha256.Sum256([]byte(n.AuthNonce + ":" + cnonce + ":" + p.settings.APIKey))
	var login struct {
		LoginResult string `json:"loginResult"`
		SessionID   string `json:"sessionID"`
	}
	query := url.Values{"nonce": {n.AuthNonce}, "cnonce": {cnonce}, "hash": {base64.StdEncoding.EncodeToString(sum[:])}}
	if err := p.get(ctx, "auth/login", query, &login); err != nil {
		return "", err
	}
	if strings.EqualFold(login.LoginResult, "failed") {
		return "", errAuth
	}
	p.sessionID = login.SessionID
	return p.sessionID, nil
}

// newUUID returns a random UUID, as Radarr's Guid.NewGuid.
func newUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6], b[8] = b[6]&0x0f|0x40, b[8]&0x3f|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// call sends an authenticated request, built by build with the session
// ID, and logs in again once when the session expired.
func (p *proxy) call(ctx context.Context, build func(sessionID string) (*http.Request, error), out response) error {
	send := func(renew bool) error {
		id, err := p.session(ctx, renew)
		if err != nil {
			return err
		}
		req, err := build(id)
		if err != nil {
			return err
		}
		return p.send(req, out)
	}
	if err := send(false); err != nil {
		return err
	}
	if !out.base().notLoggedIn() {
		return nil
	}
	if err := send(true); err != nil {
		return err
	}
	if out.base().notLoggedIn() {
		return errors.New("unable to remain authenticated to NZBVortex")
	}
	return nil
}

// getAuth sends an authenticated GET request.
func (p *proxy) getAuth(ctx context.Context, resource string, query url.Values, out response) error {
	return p.call(ctx, func(id string) (*http.Request, error) {
		q := url.Values{}
		for k, v := range query {
			q[k] = v
		}
		q.Set("sessionid", id)
		return http.NewRequestWithContext(ctx, http.MethodGet, p.url(resource, q), nil)
	}, out)
}

// downloadNzb uploads an NZB and returns its add UUID.
func (p *proxy) downloadNzb(ctx context.Context, data []byte, fileName string, priority int, group string) (string, error) {
	var out struct {
		result
		ID string `json:"add_uuid"`
	}
	err := p.call(ctx, func(id string) (*http.Request, error) {
		q := url.Values{"priority": {strconv.Itoa(priority)}, "sessionid": {id}}
		if strings.TrimSpace(group) != "" {
			q.Set("groupname", group)
		}
		var body bytes.Buffer
		w := multipart.NewWriter(&body)
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
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url("nzb/add", q), &body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", w.FormDataContentType())
		return req, nil
	}, &out)
	return out.ID, err
}

func (p *proxy) remove(ctx context.Context, id int, deleteData bool) error {
	action := "cancel"
	if deleteData {
		action = "cancelDelete"
	}
	var out result
	return p.getAuth(ctx, fmt.Sprintf("nzb/%d/%s", id, action), nil, &out)
}

func (p *proxy) version(ctx context.Context) (string, error) {
	var out struct {
		result
		Version string `json:"version"`
	}
	err := p.get(ctx, "app/appversion", nil, &out)
	return out.Version, err
}

func (p *proxy) apiVersion(ctx context.Context) (string, error) {
	var out struct {
		result
		APILevel string `json:"apilevel"`
	}
	err := p.get(ctx, "app/apilevel", nil, &out)
	return out.APILevel, err
}

func (p *proxy) groups(ctx context.Context) ([]string, error) {
	var out struct {
		result
		Groups []struct {
			GroupName string `json:"groupname"`
		} `json:"groups"`
	}
	if err := p.getAuth(ctx, "group", nil, &out); err != nil {
		return nil, err
	}
	names := make([]string, len(out.Groups))
	for i, g := range out.Groups {
		names[i] = g.GroupName
	}
	return names, nil
}

// queue lists the downloads of a group, or of all when group is empty,
// with at most doneLimit of those done.
func (p *proxy) queue(ctx context.Context, doneLimit int, group string) ([]queueItem, error) {
	q := url.Values{"limitDone": {strconv.Itoa(doneLimit)}}
	if strings.TrimSpace(group) != "" {
		q.Set("groupName", group)
	}
	var out struct {
		result
		Items []queueItem `json:"nzbs"`
	}
	if err := p.getAuth(ctx, "nzb", q, &out); err != nil {
		return nil, err
	}
	return out.Items, nil
}

func (p *proxy) files(ctx context.Context, id int) ([]file, error) {
	var out struct {
		result
		Files []file `json:"files"`
	}
	if err := p.getAuth(ctx, fmt.Sprintf("file/%d", id), nil, &out); err != nil {
		return nil, err
	}
	return out.Files, nil
}

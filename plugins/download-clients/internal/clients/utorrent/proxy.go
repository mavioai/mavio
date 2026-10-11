package utorrent

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// The torrent status flags.
const (
	statusStarted  = 1
	statusChecking = 2
	statusChecked  = 8
	statusError    = 16
	statusPaused   = 32
	statusQueued   = 64
	statusLoaded   = 128
)

// torrent is a torrent of the list, which uTorrent sends as an array.
type torrent struct {
	Hash      string
	Status    int
	Name      string
	Size      int64
	Progress  float64
	Ratio     float64
	ETA       int64
	Label     string
	Remaining int64
	// RootDownloadPath is empty for builds before 25406.
	RootDownloadPath string
}

// The positions of the fields in a torrent's array.
const (
	fieldHash             = 0
	fieldStatus           = 1
	fieldName             = 2
	fieldSize             = 3
	fieldProgress         = 4
	fieldRatio            = 7
	fieldETA              = 10
	fieldLabel            = 11
	fieldRemaining        = 18
	fieldRootDownloadPath = 26
)

// UnmarshalJSON reads a torrent's array, as Radarr's
// UTorrentTorrentJsonConverter.
func (t *torrent) UnmarshalJSON(data []byte) error {
	var a []json.RawMessage
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}
	if len(a) <= fieldRemaining {
		return fmt.Errorf("torrent has %d fields, want at least %d", len(a), fieldRemaining+1)
	}
	var progress, ratio int64
	for _, f := range []struct {
		i   int
		dst any
	}{
		{fieldHash, &t.Hash},
		{fieldStatus, &t.Status},
		{fieldName, &t.Name},
		{fieldSize, &t.Size},
		{fieldProgress, &progress},
		{fieldRatio, &ratio},
		{fieldETA, &t.ETA},
		{fieldLabel, &t.Label},
		{fieldRemaining, &t.Remaining},
	} {
		if err := json.Unmarshal(a[f.i], f.dst); err != nil {
			return fmt.Errorf("torrent field %d: %w", f.i, err)
		}
	}
	t.Progress, t.Ratio = float64(progress)/1000, float64(ratio)/1000
	// Builds before 25406 do not send the rest.
	if len(a) > fieldRootDownloadPath {
		if err := json.Unmarshal(a[fieldRootDownloadPath], &t.RootDownloadPath); err != nil {
			return fmt.Errorf("torrent field %d: %w", fieldRootDownloadPath, err)
		}
	}
	return nil
}

// response is the answer of the API, as Radarr's UTorrentResponse.
type response struct {
	Build    int       `json:"build"`
	Torrents []torrent `json:"torrents"`
	// Changed, Removed and CacheID are the incremental list.
	Changed  []torrent           `json:"torrentp"`
	Removed  []string            `json:"torrentm"`
	CacheID  string              `json:"torrentc"`
	Settings [][]json.RawMessage `json:"settings"`
}

// param is a query parameter; uTorrent pairs repeated parameters by their
// order, so they are kept in it.
type param struct{ key, value string }

// proxy calls uTorrent's Web UI API as Radarr's UTorrentProxy.
type proxy struct {
	http     *http.Client
	settings *settings

	mu      sync.Mutex
	token   string
	cookies []*http.Cookie
}

func (p *proxy) url() string { return p.settings.BaseURL() + "/gui/" }

func (p *proxy) request(ctx context.Context, method, u string, body []byte, contentType string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, u, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Cache-Control", "no-cache")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.SetBasicAuth(p.settings.Username, p.settings.Password)
	return req, nil
}

// authenticate gets a token and the GUID cookie from token.html, as
// Radarr's AuthenticateClient: those kept unless renew is set.
func (p *proxy) authenticate(ctx context.Context, renew bool) (string, []*http.Cookie, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.token != "" && !renew {
		return p.token, p.cookies, nil
	}
	p.token, p.cookies = "", nil
	req, err := p.request(ctx, http.MethodGet, p.url()+"token.html", nil, "")
	if err != nil {
		return "", nil, err
	}
	resp, err := client.Send(p.http, req)
	if err != nil {
		return "", nil, err
	}
	switch {
	case resp.StatusCode == http.StatusUnauthorized:
		return "", nil, &client.AuthError{Err: errors.New("failed to authenticate with uTorrent")}
	case resp.Err() != nil:
		return "", nil, fmt.Errorf("unable to connect to uTorrent, please check your settings: %w", resp.Err())
	}
	token, err := parseToken(resp.Body)
	if err != nil {
		return "", nil, fmt.Errorf("unable to connect to uTorrent, please check your settings: %w", err)
	}
	p.token = token
	p.cookies = (&http.Response{Header: resp.Header}).Cookies()
	return p.token, p.cookies, nil
}

// parseToken reads the token of token.html, the text of the first
// element in its root, such as <html><div id='token'>…</div></html>.
func parseToken(body []byte) (string, error) {
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = false
	depth := 0
	var text strings.Builder
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read token: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
			if depth == 1 {
				return strings.TrimSpace(text.String()), nil
			}
		case xml.CharData:
			if depth >= 2 {
				text.Write(tok)
			}
		}
	}
	return "", errors.New("no token in token.html")
}

// call calls the API with params after the token, sending body when not
// nil, and decodes the answer.
func (p *proxy) call(ctx context.Context, params []param, body []byte, contentType string) (*response, error) {
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	send := func(renew bool) (*client.Response, error) {
		token, cookies, err := p.authenticate(ctx, renew)
		if err != nil {
			return nil, err
		}
		q := "token=" + url.QueryEscape(token)
		for _, pr := range params {
			q += "&" + url.QueryEscape(pr.key) + "=" + url.QueryEscape(pr.value)
		}
		req, err := p.request(ctx, method, p.url()+"?"+q, body, contentType)
		if err != nil {
			return nil, err
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		return client.Send(p.http, req)
	}
	resp, err := send(false)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized:
		// The token or the session expired: authenticate again.
		if resp, err = send(true); err != nil {
			return nil, err
		}
		if err := resp.Err(); err != nil {
			return nil, err
		}
	case resp.Err() != nil:
		return nil, fmt.Errorf("unable to connect to uTorrent, please check your settings: %w", resp.Err())
	}
	var r response
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return &r, nil
	}
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		return nil, fmt.Errorf("utorrent: unexpected response: %w", err)
	}
	return &r, nil
}

func (p *proxy) action(ctx context.Context, params ...param) error {
	_, err := p.call(ctx, params, nil, "")
	return err
}

// version returns uTorrent's build.
func (p *proxy) version(ctx context.Context) (int, error) {
	r, err := p.call(ctx, []param{{"action", "getsettings"}}, nil, "")
	if err != nil {
		return 0, err
	}
	return r.Build, nil
}

// config returns uTorrent's settings by name, their values as text.
func (p *proxy) config(ctx context.Context) (map[string]string, error) {
	r, err := p.call(ctx, []param{{"action", "getsettings"}}, nil, "")
	if err != nil {
		return nil, err
	}
	config := map[string]string{}
	for _, s := range r.Settings {
		if len(s) < 3 {
			continue
		}
		var name string
		if err := json.Unmarshal(s[0], &name); err != nil {
			continue
		}
		var value string
		if err := json.Unmarshal(s[2], &value); err != nil {
			value = strings.Trim(string(s[2]), `"`)
		}
		config[name] = value
	}
	return config, nil
}

// torrents lists the torrents, those changed since cacheID when it is
// not empty.
func (p *proxy) torrents(ctx context.Context, cacheID string) (*response, error) {
	params := []param{{"list", "1"}}
	if strings.TrimSpace(cacheID) != "" {
		params = append(params, param{"cid", cacheID})
	}
	return p.call(ctx, params, nil, "")
}

func (p *proxy) addURL(ctx context.Context, link string) error {
	return p.action(ctx, param{"action", "add-url"}, param{"s", link})
}

func (p *proxy) addFile(ctx context.Context, fileName string, data []byte) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, err := w.CreateFormFile("torrent_file", fileName)
	if err != nil {
		return err
	}
	if _, err := part.Write(data); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	_, err = p.call(ctx, []param{{"action", "add-file"}, {"path", ""}}, buf.Bytes(), w.FormDataContentType())
	return err
}

// setSeeding overrides a torrent's seed limits with the release's: the
// ratio in thousandths, the time in seconds.
func (p *proxy) setSeeding(ctx context.Context, hash string, r *client.Release) error {
	if r.SeedRatio == nil && r.SeedTime == nil {
		return nil
	}
	params := []param{{"action", "setprops"}, {"hash", hash}, {"s", "seed_override"}, {"v", "1"}}
	if r.SeedRatio != nil {
		params = append(params, param{"s", "seed_ratio"}, param{"v", strconv.FormatInt(int64(math.RoundToEven(*r.SeedRatio*1000)), 10)})
	}
	if r.SeedTime != nil {
		params = append(params, param{"s", "seed_time"}, param{"v", strconv.FormatInt(int64(math.RoundToEven(r.SeedTime.Seconds())), 10)})
	}
	return p.action(ctx, params...)
}

func (p *proxy) remove(ctx context.Context, hash string, deleteData bool) error {
	action := "remove"
	if deleteData {
		action = "removedata"
	}
	return p.action(ctx, param{"action", action}, param{"hash", hash})
}

func (p *proxy) setLabel(ctx context.Context, hash, label string) error {
	return p.action(ctx, param{"action", "setprops"}, param{"hash", hash}, param{"s", "label"}, param{"v", label})
}

// removeLabel takes a label off a torrent, which uTorrent needs told
// explicitly.
func (p *proxy) removeLabel(ctx context.Context, hash, label string) error {
	return p.action(ctx, param{"action", "setprops"}, param{"hash", hash},
		param{"s", "label"}, param{"v", label}, param{"s", "label"}, param{"v", ""})
}

func (p *proxy) moveToTop(ctx context.Context, hash string) error {
	return p.action(ctx, param{"action", "queuetop"}, param{"hash", hash})
}

func (p *proxy) setState(ctx context.Context, hash string, state int) error {
	return p.action(ctx, param{"action", stateActions[state]}, param{"hash", hash})
}

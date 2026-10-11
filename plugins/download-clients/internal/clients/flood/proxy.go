package flood

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// torrent is a torrent of Flood's list.
type torrent struct {
	BytesDone int64    `json:"bytesDone"`
	Directory string   `json:"directory"`
	ETA       int64    `json:"eta"`
	Message   string   `json:"message"`
	Name      string   `json:"name"`
	Ratio     float64  `json:"ratio"`
	SizeBytes int64    `json:"sizeBytes"`
	Status    []string `json:"status"`
	Tags      []string `json:"tags"`
	// DateFinished is in Unix seconds, from Flood 4.5.
	DateFinished *int64 `json:"dateFinished"`
}

// proxy calls Flood's API as Radarr's FloodProxy.
type proxy struct {
	http     *http.Client
	settings *settings

	mu      sync.Mutex
	cookies []*http.Cookie
}

func (p *proxy) url(path string) string { return p.settings.BaseURL() + "/api" + path }

func (p *proxy) request(ctx context.Context, method, path string, body any) (*http.Request, error) {
	var data []byte
	if body != nil {
		var err error
		if data, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, p.url(path), bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

var errAuth = errors.New("failed to authenticate with Flood")

// check returns the error of a failed answer, as Radarr's HandleRequest.
func check(resp *client.Response) error {
	switch err := resp.Err(); {
	case err == nil:
		return nil
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return &client.AuthError{Err: errAuth}
	default:
		return fmt.Errorf("unable to connect to Flood, please check your settings: %w", err)
	}
}

// authenticate logs in for the session cookie, as Radarr's
// AuthAuthenticate: the one kept unless renew is set. It reports whether
// the cookie was kept.
func (p *proxy) authenticate(ctx context.Context, renew bool) ([]*http.Cookie, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cookies != nil && !renew {
		return p.cookies, true, nil
	}
	p.cookies = nil
	req, err := p.request(ctx, http.MethodPost, "/auth/authenticate",
		map[string]string{"username": p.settings.Username, "password": p.settings.Password})
	if err != nil {
		return nil, false, err
	}
	resp, err := client.Send(p.http, req)
	if err != nil {
		return nil, false, err
	}
	if err := check(resp); err != nil {
		return nil, false, err
	}
	// Not nil, so a login without cookies is kept too.
	p.cookies = append([]*http.Cookie{}, (&http.Response{Header: resp.Header}).Cookies()...)
	return p.cookies, false, nil
}

func (p *proxy) forget() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cookies = nil
}

// call calls the API and decodes its answer into out, when not nil. A
// kept session cookie refused is renewed once.
func (p *proxy) call(ctx context.Context, method, path string, body, out any) error {
	send := func(renew bool) (*client.Response, bool, error) {
		cookies, kept, err := p.authenticate(ctx, renew)
		if err != nil {
			return nil, false, err
		}
		req, err := p.request(ctx, method, path, body)
		if err != nil {
			return nil, false, err
		}
		for _, c := range cookies {
			req.AddCookie(c)
		}
		resp, err := client.Send(p.http, req)
		return resp, kept, err
	}
	resp, kept, err := send(false)
	if err != nil {
		return err
	}
	if (resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden) && kept {
		if resp, _, err = send(true); err != nil {
			return err
		}
	}
	if err := check(resp); err != nil {
		var auth *client.AuthError
		if errors.As(err, &auth) {
			p.forget()
		}
		return err
	}
	if out != nil {
		if err := json.Unmarshal(resp.Body, out); err != nil {
			return fmt.Errorf("flood: %s %s: %w", method, path, err)
		}
	}
	return nil
}

func (p *proxy) verify(ctx context.Context) error {
	return p.call(ctx, http.MethodGet, "/auth/verify", nil, nil)
}

// add adds torrents by files or URLs, as key says.
func (p *proxy) add(ctx context.Context, path, key, value string, tags []string, destination string) error {
	body := map[string]any{key: []string{value}, "tags": tags}
	if destination != "" {
		body["destination"] = destination
	}
	if p.settings.StartOnAdd {
		body["start"] = true
	}
	return p.call(ctx, http.MethodPost, path, body, nil)
}

func (p *proxy) addFile(ctx context.Context, file string, tags []string, destination string) error {
	return p.add(ctx, "/torrents/add-files", "files", file, tags, destination)
}

func (p *proxy) addURL(ctx context.Context, link string, tags []string, destination string) error {
	return p.add(ctx, "/torrents/add-urls", "urls", link, tags, destination)
}

func (p *proxy) remove(ctx context.Context, hash string, deleteData bool) error {
	return p.call(ctx, http.MethodPost, "/torrents/delete", map[string]any{"hashes": []string{hash}, "deleteData": deleteData}, nil)
}

func (p *proxy) torrents(ctx context.Context) (map[string]torrent, error) {
	var out struct {
		Torrents map[string]torrent `json:"torrents"`
	}
	if err := p.call(ctx, http.MethodGet, "/torrents", nil, &out); err != nil {
		return nil, err
	}
	return out.Torrents, nil
}

// contentPaths returns the paths of a torrent's files.
func (p *proxy) contentPaths(ctx context.Context, hash string) ([]string, error) {
	var out []struct {
		Path string `json:"path"`
	}
	if err := p.call(ctx, http.MethodGet, "/torrents/"+url.PathEscape(hash)+"/contents", nil, &out); err != nil {
		return nil, err
	}
	paths := make([]string, len(out))
	for i, c := range out {
		paths[i] = c.Path
	}
	return paths, nil
}

func (p *proxy) setTags(ctx context.Context, hash string, tags []string) error {
	return p.call(ctx, http.MethodPatch, "/torrents/tags", map[string]any{"hashes": []string{hash}, "tags": tags}, nil)
}

// defaultDirectory returns the folder Flood downloads to by default.
func (p *proxy) defaultDirectory(ctx context.Context) (string, error) {
	var out struct {
		DirectoryDefault string `json:"directoryDefault"`
	}
	if err := p.call(ctx, http.MethodGet, "/client/settings", nil, &out); err != nil {
		return "", err
	}
	return out.DirectoryDefault, nil
}

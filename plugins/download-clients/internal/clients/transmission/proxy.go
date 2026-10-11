package transmission

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// torrent is a torrent of torrent-get.
type torrent struct {
	ID                int      `json:"id"`
	HashString        string   `json:"hashString"`
	Name              string   `json:"name"`
	DownloadDir       string   `json:"downloadDir"`
	TotalSize         int64    `json:"totalSize"`
	LeftUntilDone     int64    `json:"leftUntilDone"`
	IsFinished        bool     `json:"isFinished"`
	Labels            []string `json:"labels"`
	ETA               int64    `json:"eta"`
	Status            int      `json:"status"`
	SecondsSeeding    int64    `json:"secondsSeeding"`
	ErrorString       string   `json:"errorString"`
	DownloadedEver    int64    `json:"downloadedEver"`
	UploadedEver      int64    `json:"uploadedEver"`
	SeedRatioLimit    float64  `json:"seedRatioLimit"`
	SeedRatioMode     int      `json:"seedRatioMode"`
	SeedIdleLimit     int64    `json:"seedIdleLimit"`
	SeedIdleMode      int      `json:"seedIdleMode"`
	TransmissionFiles *int     `json:"file-count"`
	VuzeFiles         *int     `json:"fileCount"`
}

// fileCount is the number of files, which Transmission and Vuze name
// differently.
func (t *torrent) fileCount() int {
	switch {
	case t.TransmissionFiles != nil:
		return *t.TransmissionFiles
	case t.VuzeFiles != nil:
		return *t.VuzeFiles
	}
	return 0
}

// The torrent statuses.
const (
	statusStopped = iota
	statusCheckWait
	statusCheck
	statusQueued
	statusDownloading
	statusSeedingWait
	statusSeeding
)

// sessionConfig is the session of session-get.
type sessionConfig struct {
	RPCVersion              json.Number `json:"rpc-version"`
	Version                 string      `json:"version"`
	DownloadDir             string      `json:"download-dir"`
	SeedRatioLimit          float64     `json:"seedRatioLimit"`
	SeedRatioLimited        bool        `json:"seedRatioLimited"`
	IdleSeedingLimit        int64       `json:"idle-seeding-limit"`
	IdleSeedingLimitEnabled bool        `json:"idle-seeding-limit-enabled"`
}

// torrentFields are the fields torrent-get asks for.
var torrentFields = []string{
	"id", "hashString", "name", "downloadDir", "totalSize", "leftUntilDone", "isFinished", "eta", "status",
	"secondsDownloading", "secondsSeeding", "errorString", "uploadedEver", "downloadedEver", "seedRatioLimit",
	"seedRatioMode", "seedIdleLimit", "seedIdleMode", "fileCount", "file-count", "labels",
}

// proxy calls Transmission's RPC as Radarr's TransmissionProxy.
type proxy struct {
	http     *http.Client
	settings *settings

	mu        sync.Mutex
	sessionID string
}

func (p *proxy) url() string { return p.settings.BaseURL() + "/rpc" }

func (p *proxy) request(ctx context.Context, body []byte) (*http.Request, error) {
	method := http.MethodGet
	if body != nil {
		method = http.MethodPost
	}
	req, err := http.NewRequestWithContext(ctx, method, p.url(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.SetBasicAuth(p.settings.Username, p.settings.Password)
	return req, nil
}

// authenticate gets a session ID, as Radarr's AuthenticateClient: one
// kept unless renew is set.
func (p *proxy) authenticate(ctx context.Context, renew bool) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.sessionID != "" && !renew {
		return p.sessionID, nil
	}
	p.sessionID = ""
	req, err := p.request(ctx, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Send(p.http, req)
	if err != nil {
		return "", err
	}
	switch resp.StatusCode {
	case http.StatusMovedPermanently:
		return "", fmt.Errorf("remote site redirected to %s", resp.Header.Get("Location"))
	case http.StatusForbidden:
		return "", errors.New("failed to authenticate with Transmission; it may be necessary to add Mavio's IP address to the RPC whitelist")
	case http.StatusConflict:
		id := resp.Header.Get("X-Transmission-Session-Id")
		if id == "" {
			return "", errors.New("remote host did not return a session ID")
		}
		p.sessionID = id
		return id, nil
	}
	return "", &client.AuthError{Err: errors.New("failed to authenticate with Transmission")}
}

// call calls an RPC method and decodes its arguments into out, when not
// nil.
func (p *proxy) call(ctx context.Context, method string, args any, out any) error {
	data := map[string]any{"method": method}
	if args != nil {
		data["arguments"] = args
	}
	body, err := json.Marshal(data)
	if err != nil {
		return err
	}
	send := func(renew bool) (*client.Response, error) {
		id, err := p.authenticate(ctx, renew)
		if err != nil {
			return nil, err
		}
		req, err := p.request(ctx, body)
		if err != nil {
			return nil, err
		}
		req.Header.Set("X-Transmission-Session-Id", id)
		return client.Send(p.http, req)
	}
	resp, err := send(false)
	if err != nil {
		return err
	}
	switch resp.StatusCode {
	case http.StatusConflict:
		if resp, err = send(true); err != nil {
			return err
		}
	case http.StatusUnauthorized:
		return &client.AuthError{Err: errors.New("user authentication failed")}
	}
	var r struct {
		Result    string          `json:"result"`
		Arguments json.RawMessage `json:"arguments"`
	}
	if err := json.Unmarshal(resp.Body, &r); err != nil || r.Result == "" {
		if err := resp.Err(); err != nil {
			return fmt.Errorf("unable to connect to Transmission: %w", err)
		}
		return errors.New("transmission: unexpected response")
	}
	if r.Result != "success" {
		return fmt.Errorf("transmission: %s", r.Result)
	}
	if out != nil && len(r.Arguments) > 0 {
		if err := json.Unmarshal(r.Arguments, out); err != nil {
			return fmt.Errorf("transmission: %s: %w", method, err)
		}
	}
	return nil
}

func (p *proxy) torrents(ctx context.Context, hashes []string) ([]torrent, error) {
	args := map[string]any{"fields": torrentFields}
	if hashes != nil {
		args["ids"] = hashes
	}
	var out struct {
		Torrents []torrent `json:"torrents"`
	}
	if err := p.call(ctx, "torrent-get", args, &out); err != nil {
		return nil, err
	}
	return out.Torrents, nil
}

func (p *proxy) config(ctx context.Context) (*sessionConfig, error) {
	var c sessionConfig
	if err := p.call(ctx, "session-get", nil, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// add adds a torrent from a link (filename) or data (metainfo).
func (p *proxy) add(ctx context.Context, key, value, dir, label string) error {
	args := map[string]any{key: value, "paused": p.settings.AddPaused}
	if dir != "" {
		args["download-dir"] = dir
	}
	if label != "" {
		args["labels"] = []string{label}
	}
	return p.call(ctx, "torrent-add", args, nil)
}

func (p *proxy) addURL(ctx context.Context, link, dir, label string) error {
	return p.add(ctx, "filename", link, dir, label)
}

func (p *proxy) addData(ctx context.Context, data []byte, dir, label string) error {
	return p.add(ctx, "metainfo", base64.StdEncoding.EncodeToString(data), dir, label)
}

// setSeeding sets a torrent's seed limits; Transmission has no seed time
// limit, so the idle limit stands in for it.
func (p *proxy) setSeeding(ctx context.Context, hash string, r *client.Release) error {
	args := map[string]any{"ids": []string{hash}}
	if r.SeedRatio != nil {
		args["seedRatioLimit"] = *r.SeedRatio
		args["seedRatioMode"] = 1
	}
	if r.SeedTime != nil {
		args["seedIdleLimit"] = int(r.SeedTime.Minutes())
		args["seedIdleMode"] = 1
	}
	if len(args) == 1 {
		return nil
	}
	return p.call(ctx, "torrent-set", args, nil)
}

func (p *proxy) remove(ctx context.Context, hash string, deleteData bool) error {
	return p.call(ctx, "torrent-remove", map[string]any{"ids": []string{hash}, "delete-local-data": deleteData}, nil)
}

func (p *proxy) moveToTop(ctx context.Context, hash string) error {
	return p.call(ctx, "queue-move-top", map[string]any{"ids": []string{hash}}, nil)
}

func (p *proxy) setLabels(ctx context.Context, hash string, labels []string) error {
	return p.call(ctx, "torrent-set", map[string]any{"ids": []string{hash}, "labels": labels}, nil)
}

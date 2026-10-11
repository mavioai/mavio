package deluge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
	"github.com/mavioai/mavio/plugins/download-clients/internal/jsonrpc"
)

// torrent is a torrent of web.update_ui.
type torrent struct {
	Hash          string  `json:"hash"`
	Name          string  `json:"name"`
	State         string  `json:"state"`
	Progress      float64 `json:"progress"`
	ETA           float64 `json:"eta"`
	Message       string  `json:"message"`
	IsFinished    bool    `json:"is_finished"`
	SavePath      string  `json:"save_path"`
	TotalSize     int64   `json:"total_size"`
	TotalDone     int64   `json:"total_done"`
	Ratio         float64 `json:"ratio"`
	IsAutoManaged bool    `json:"is_auto_managed"`
	StopAtRatio   bool    `json:"stop_at_ratio"`
	RemoveAtRatio bool    `json:"remove_at_ratio"`
	StopRatio     float64 `json:"stop_ratio"`
}

// The torrent states.
const (
	statePaused      = "Paused"
	stateQueued      = "Queued"
	stateDownloading = "Downloading"
	stateSeeding     = "Seeding"
	stateChecking    = "Checking"
	stateError       = "Error"
)

// labelOptions are the options of a label of label.get_options.
type labelOptions struct {
	ApplyMoveCompleted bool   `json:"apply_move_completed"`
	MoveCompleted      bool   `json:"move_completed"`
	MoveCompletedPath  string `json:"move_completed_path"`
}

// torrentFields are the fields web.update_ui asks for.
var torrentFields = []string{
	"hash", "name", "state", "progress", "eta", "message", "is_finished", "save_path", "total_size", "total_done",
	"time_added", "active_time", "ratio", "is_auto_managed", "stop_at_ratio", "remove_at_ratio", "stop_ratio",
}

// requestTimeout bounds each request, as Radarr's.
const requestTimeout = 15 * time.Second

// rpcError is the error of a JSON-RPC answer, as Radarr's DelugeError and
// DelugeException.
type rpcError struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

func (e *rpcError) Error() string { return fmt.Sprintf("%s (code %d)", e.Message, e.Code) }

// rpcResponse is a JSON-RPC answer.
type rpcResponse struct {
	Result json.RawMessage `json:"result"`
	Error  *rpcError       `json:"error"`
}

// proxy calls Deluge's web UI as Radarr's DelugeProxy; the web UI's
// session cookies are kept, as Radarr's auth cookie cache keeps them.
type proxy struct {
	http     *http.Client
	url      string
	password string
	log      *slog.Logger

	ids     jsonrpc.IDs
	mu      sync.Mutex
	cookies []*http.Cookie
}

// execute sends a JSON-RPC request; a timeout answers as a broken daemon
// connection would, error code 2, as Radarr's ExecuteRequest.
func (p *proxy) execute(ctx context.Context, cookies []*http.Cookie, method string, params ...any) (*rpcResponse, *client.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	req, err := jsonrpc.NewRequest(ctx, p.url, p.ids.Next(), method, params...)
	if err != nil {
		return nil, nil, err
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := client.Send(p.http, req)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to connect to Deluge, please check your settings: %w", err)
	}
	if resp.StatusCode == http.StatusRequestTimeout {
		p.log.DebugContext(ctx, "Deluge timeout during request, daemon connection may have been broken; attempting to reconnect")
		return &rpcResponse{Error: &rpcError{Code: 2}}, resp, nil
	}
	if resp.StatusCode >= 400 {
		return nil, nil, fmt.Errorf("unable to connect to Deluge, please check your settings: %w", resp.Err())
	}
	raw, err := jsonrpc.Decode(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("deluge: %s: %w", method, err)
	}
	r := &rpcResponse{Result: raw.Result}
	if raw.Failed() {
		r.Error = &rpcError{}
		if err := json.Unmarshal(raw.Error, r.Error); err != nil {
			return nil, nil, fmt.Errorf("deluge: %s: unexpected error: %w", method, err)
		}
	}
	return r, resp, nil
}

// verified returns the error of an answer, if any.
func verified(r *rpcResponse, _ *client.Response, err error) (*rpcResponse, error) {
	if err != nil {
		return nil, err
	}
	if r.Error != nil {
		return nil, r.Error
	}
	return r, nil
}

// authenticate logs in and connects the web UI to the daemon, as Radarr's
// AuthenticateClient: the cookies kept are returned unless renew is set.
func (p *proxy) authenticate(ctx context.Context, renew bool) ([]*http.Cookie, error) {
	p.mu.Lock()
	cookies := p.cookies
	p.mu.Unlock()
	if cookies != nil && !renew {
		return cookies, nil
	}
	r, resp, err := p.execute(ctx, nil, "auth.login", p.password)
	if err != nil {
		return nil, err
	}
	var ok bool
	if r.Error != nil || json.Unmarshal(r.Result, &ok) != nil || !ok {
		p.log.DebugContext(ctx, "Deluge authentication failed")
		return nil, &client.AuthError{Err: errors.New("failed to authenticate with Deluge")}
	}
	p.log.DebugContext(ctx, "Deluge authentication succeeded")
	cookies = (&http.Response{Header: resp.Header}).Cookies()
	if cookies == nil {
		cookies = []*http.Cookie{}
	}
	if err := p.connectDaemon(ctx, cookies); err != nil {
		return nil, err
	}
	p.mu.Lock()
	p.cookies = cookies
	p.mu.Unlock()
	return cookies, nil
}

// connectDaemon connects the web UI to the daemon unless it is: to the
// host on 127.0.0.1, as Radarr does, else to the first host.
func (p *proxy) connectDaemon(ctx context.Context, cookies []*http.Cookie) error {
	r, err := verified(p.execute(ctx, cookies, "web.connected"))
	if err != nil {
		return err
	}
	var connected bool
	if json.Unmarshal(r.Result, &connected) == nil && connected {
		return nil
	}
	if r, err = verified(p.execute(ctx, cookies, "web.get_hosts")); err != nil {
		return err
	}
	var hosts [][]any
	_ = json.Unmarshal(r.Result, &hosts)
	hosts = slices.DeleteFunc(hosts, func(h []any) bool { return len(h) < 2 })
	if len(hosts) == 0 {
		return errors.New("failed to connect to Deluge daemon")
	}
	host := hosts[0]
	if i := slices.IndexFunc(hosts, func(h []any) bool { return h[1] == "127.0.0.1" }); i >= 0 {
		host = hosts[i]
	}
	_, err = verified(p.execute(ctx, cookies, "web.connect", host[0]))
	return err
}

// call calls a method and decodes its result into out, when not nil, as
// Radarr's ProcessRequest: errors 1 and 2, of an expired session or a
// lost daemon, log in again once.
func (p *proxy) call(ctx context.Context, out any, method string, params ...any) error {
	cookies, err := p.authenticate(ctx, false)
	if err != nil {
		return err
	}
	r, _, err := p.execute(ctx, cookies, method, params...)
	if err != nil {
		return err
	}
	if r.Error != nil {
		if r.Error.Code != 1 && r.Error.Code != 2 {
			return r.Error
		}
		if cookies, err = p.authenticate(ctx, true); err != nil {
			return err
		}
		if r, _, err = p.execute(ctx, cookies, method, params...); err != nil {
			return err
		}
		if r.Error != nil {
			return &client.AuthError{Err: errors.New(r.Error.Message)}
		}
	}
	if out != nil && len(r.Result) > 0 && string(r.Result) != "null" {
		if err := json.Unmarshal(r.Result, out); err != nil {
			return fmt.Errorf("deluge: %s: %w", method, err)
		}
	}
	return nil
}

func (p *proxy) methods(ctx context.Context) ([]string, error) {
	var out []string
	err := p.call(ctx, &out, "system.listMethods")
	return out, err
}

func (p *proxy) version(ctx context.Context) (string, error) {
	methods, err := p.methods(ctx)
	if err != nil {
		return "", err
	}
	method := "daemon.info"
	if slices.Contains(methods, "daemon.get_version") {
		method = "daemon.get_version"
	}
	var v string
	err = p.call(ctx, &v, method)
	return v, err
}

func (p *proxy) config(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := p.call(ctx, &out, "core.get_config")
	return out, err
}

// torrents lists the torrents with a label, or all of them for "", in
// the order Deluge lists them.
func (p *proxy) torrents(ctx context.Context, label string) ([]torrent, error) {
	filter := map[string]any{}
	if label != "" {
		filter["label"] = label
	}
	var out struct {
		Torrents json.RawMessage `json:"torrents"`
	}
	if err := p.call(ctx, &out, "web.update_ui", torrentFields, filter); err != nil {
		return nil, err
	}
	return decodeTorrents(out.Torrents)
}

// decodeTorrents reads the torrents of web.update_ui, an object by hash,
// keeping their order.
func decodeTorrents(raw json.RawMessage) ([]torrent, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errors.New("deluge: web.update_ui: torrents are not an object")
	}
	var out []torrent
	for dec.More() {
		if _, err := dec.Token(); err != nil {
			return nil, fmt.Errorf("deluge: web.update_ui: %w", err)
		}
		var t torrent
		if err := dec.Decode(&t); err != nil {
			return nil, fmt.Errorf("deluge: web.update_ui: %w", err)
		}
		out = append(out, t)
	}
	return out, nil
}

// addOptions are the options torrents are added with.
func addOptions(addPaused bool, dir, completedDir string) map[string]any {
	o := map[string]any{"add_paused": addPaused, "remove_at_ratio": false}
	if dir != "" {
		o["download_location"] = dir
	}
	if completedDir != "" {
		o["move_completed_path"] = completedDir
		o["move_completed"] = true
	}
	return o
}

func (p *proxy) addMagnet(ctx context.Context, link string, options map[string]any) (string, error) {
	var hash string
	err := p.call(ctx, &hash, "core.add_torrent_magnet", link, options)
	return hash, err
}

func (p *proxy) addFile(ctx context.Context, name string, data []byte, options map[string]any) (string, error) {
	var hash string
	err := p.call(ctx, &hash, "core.add_torrent_file", name, base64.StdEncoding.EncodeToString(data), options)
	return hash, err
}

func (p *proxy) remove(ctx context.Context, hash string, deleteData bool) error {
	return p.call(ctx, nil, "core.remove_torrent", hash, deleteData)
}

func (p *proxy) queueTop(ctx context.Context, hash string) error {
	return p.call(ctx, nil, "core.queue_top", []string{hash})
}

func (p *proxy) labels(ctx context.Context) ([]string, error) {
	var out []string
	err := p.call(ctx, &out, "label.get_labels")
	return out, err
}

func (p *proxy) labelOptions(ctx context.Context, label string) (*labelOptions, error) {
	var out labelOptions
	err := p.call(ctx, &out, "label.get_options", label)
	return &out, err
}

func (p *proxy) setTorrentOptions(ctx context.Context, hash string, options map[string]any) error {
	return p.call(ctx, nil, "core.set_torrent_options", []string{hash}, options)
}

// setSeeding sets a torrent's seed ratio; Deluge's seed time is not set,
// as Radarr does not.
func (p *proxy) setSeeding(ctx context.Context, hash string, r *client.Release) error {
	if r.SeedRatio == nil {
		return nil
	}
	return p.setTorrentOptions(ctx, hash, map[string]any{"stop_ratio": *r.SeedRatio, "stop_at_ratio": 1})
}

func (p *proxy) addLabel(ctx context.Context, label string) error {
	return p.call(ctx, nil, "label.add", label)
}

func (p *proxy) setLabel(ctx context.Context, hash, label string) error {
	return p.call(ctx, nil, "label.set_torrent", hash, label)
}

// reconnect disconnects the web UI from the daemon and connects it again.
func (p *proxy) reconnect(ctx context.Context) error {
	if err := p.call(ctx, nil, "web.disconnect"); err != nil {
		return err
	}
	cookies, err := p.authenticate(ctx, false)
	if err != nil {
		return err
	}
	return p.connectDaemon(ctx, cookies)
}

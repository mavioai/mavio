package hadouken

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
	"github.com/mavioai/mavio/plugins/download-clients/internal/jsonrpc"
)

// state is a torrent's state, as Radarr's HadoukenTorrentState.
type state int

// The states Radarr tells apart.
const (
	stateUnknown state = iota
	stateQueuedForChecking
	stateCheckingFiles
	stateDownloading state = 4
	statePaused      state = 9
)

// parseState reads the state bits of webui.list.
func parseState(bits int64) state {
	switch {
	case bits&1 == 1:
		return stateDownloading
	case bits&2 == 2:
		return stateCheckingFiles
	case bits&32 == 32:
		return statePaused
	case bits&64 == 64:
		return stateQueuedForChecking
	}
	return stateUnknown
}

// torrent is a torrent of webui.list.
type torrent struct {
	InfoHash        string
	State           state
	Name            string
	TotalSize       int64
	Progress        float64
	DownloadedBytes int64
	UploadedBytes   int64
	DownloadRate    int64
	Label           string
	Error           string
	SavePath        string
	IsFinished      bool
}

// proxy calls Hadouken's JSON-RPC API as Radarr's HadoukenProxy.
type proxy struct {
	http     *http.Client
	url      string
	username string
	password string
	log      *slog.Logger
	ids      jsonrpc.IDs
}

// call calls a method and decodes its result into out, when not nil.
func (p *proxy) call(ctx context.Context, out any, method string, params ...any) error {
	req, err := jsonrpc.NewRequest(ctx, p.url, p.ids.Next(), method, params...)
	if err != nil {
		return err
	}
	req.SetBasicAuth(p.username, p.password)
	resp, err := client.Send(p.http, req)
	if err != nil {
		return fmt.Errorf("unable to connect to Hadouken, please check your settings: %w", err)
	}
	if err := resp.Err(); err != nil {
		return fmt.Errorf("unable to connect to Hadouken, please check your settings: %w", err)
	}
	r, err := jsonrpc.Decode(resp.Body)
	if err != nil {
		return fmt.Errorf("hadouken: %s: %w", method, err)
	}
	if r.Failed() {
		return fmt.Errorf("error response received from Hadouken: %s", r.Error)
	}
	if out != nil && r.HasResult() {
		if err := json.Unmarshal(r.Result, out); err != nil {
			return fmt.Errorf("hadouken: %s: %w", method, err)
		}
	}
	return nil
}

// systemInfo is core.getSystemInfo's.
type systemInfo struct {
	Versions map[string]string `json:"versions"`
}

func (p *proxy) systemInfo(ctx context.Context) (*systemInfo, error) {
	var out systemInfo
	err := p.call(ctx, &out, "core.getSystemInfo")
	return &out, err
}

// torrents lists the torrents; those that do not read are logged and left
// out, as Radarr's MapTorrent.
func (p *proxy) torrents(ctx context.Context) ([]torrent, error) {
	var out struct {
		Torrents [][]json.RawMessage `json:"torrents"`
	}
	if err := p.call(ctx, &out, "webui.list"); err != nil {
		return nil, err
	}
	torrents := make([]torrent, 0, len(out.Torrents))
	for _, raw := range out.Torrents {
		t, err := mapTorrent(raw)
		if err != nil {
			p.log.ErrorContext(ctx, "failed to map Hadouken torrent data", "err", err)
			continue
		}
		t.IsFinished = t.Progress >= 1000
		torrents = append(torrents, t)
	}
	return torrents, nil
}

// mapTorrent reads a torrent's fields from webui.list's array, converting
// them as .NET's Convert does.
func mapTorrent(raw []json.RawMessage) (torrent, error) {
	if len(raw) <= 26 {
		return torrent{}, fmt.Errorf("torrent has %d fields, want 27", len(raw))
	}
	var t torrent
	var bits int64
	var errs []error
	str := func(i int) string {
		s, err := toString(raw[i])
		errs = append(errs, err)
		return s
	}
	num := func(i int) float64 {
		f, err := toFloat(raw[i])
		errs = append(errs, err)
		return f
	}
	t.InfoHash = str(0)
	bits = int64(num(1))
	t.State = parseState(bits)
	t.Name = str(2)
	t.TotalSize = int64(num(3))
	t.Progress = num(4)
	t.DownloadedBytes = int64(num(5))
	t.UploadedBytes = int64(num(6))
	t.DownloadRate = int64(num(9))
	t.Label = str(11)
	t.Error = str(21)
	t.SavePath = str(26)
	return t, errors.Join(errs...)
}

// toString converts a value to a string; null is empty.
func toString(raw json.RawMessage) (string, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return "", err
	}
	switch v := v.(type) {
	case nil:
		return "", nil
	case string:
		return v, nil
	case bool:
		return strconv.FormatBool(v), nil
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64), nil
	}
	return "", fmt.Errorf("cannot convert %s to a string", raw)
}

// toFloat converts a number or numeric string; null is zero.
func toFloat(raw json.RawMessage) (float64, error) {
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, err
	}
	switch v := v.(type) {
	case nil:
		return 0, nil
	case float64:
		return v, nil
	case bool:
		if v {
			return 1, nil
		}
		return 0, nil
	case string:
		return strconv.ParseFloat(v, 64)
	}
	return 0, fmt.Errorf("cannot convert %s to a number", raw)
}

func (p *proxy) config(ctx context.Context) (map[string]any, error) {
	var out map[string]any
	err := p.call(ctx, &out, "webui.getSettings")
	return out, err
}

// addOptions are the options a torrent is added with: its label, null
// for none.
func addOptions(label string) map[string]any {
	if label == "" {
		return map[string]any{"label": nil}
	}
	return map[string]any{"label": label}
}

func (p *proxy) addFile(ctx context.Context, data []byte, label string) (string, error) {
	var hash string
	err := p.call(ctx, &hash, "webui.addTorrent", "file", base64.StdEncoding.EncodeToString(data), addOptions(label))
	return hash, err
}

func (p *proxy) addURL(ctx context.Context, link, label string) error {
	return p.call(ctx, nil, "webui.addTorrent", "url", link, addOptions(label))
}

// remove removes a torrent, with its data when deleteData is set.
func (p *proxy) remove(ctx context.Context, id string, deleteData bool) error {
	action := "remove"
	if deleteData {
		action = "removedata"
	}
	return p.call(ctx, nil, "webui.perform", action, []string{id})
}

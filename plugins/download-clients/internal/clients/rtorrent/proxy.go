package rtorrent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// torrent is a torrent of d.multicall2, as Radarr's RTorrentTorrent.
type torrent struct {
	Name          string
	Hash          string
	Path          string
	Category      string
	TotalSize     int64
	RemainingSize int64
	DownRate      int64
	// Ratio is in thousandths.
	Ratio        int64
	IsOpen       bool
	IsActive     bool
	IsFinished   bool
	FinishedTime int64
}

// multicallFields are what d.multicall2 asks of each torrent, in the
// order of torrent.
var multicallFields = []string{
	"d.name=", "d.hash=", "d.base_path=", "d.custom1=", "d.size_bytes=", "d.left_bytes=", "d.down.rate=",
	"d.ratio=", "d.is_open=", "d.is_active=", "d.complete=", "d.timestamp.finished=",
}

// The priorities of rTorrent.
const (
	priorityDoNotDownload = 0
	priorityLow           = 1
	priorityNormal        = 2
	priorityHigh          = 3
)

// proxy calls rTorrent's XML-RPC as Radarr's RTorrentProxy.
type proxy struct {
	http     *http.Client
	settings *settings
}

// call calls a method and returns its result.
func (p *proxy) call(ctx context.Context, method string, params ...any) (*value, error) {
	body, err := encodeCall(method, params...)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.settings.BaseURL(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "text/xml")
	if strings.TrimSpace(p.settings.Username) != "" {
		req.SetBasicAuth(p.settings.Username, p.settings.Password)
	}
	resp, err := client.Send(p.http, req)
	if err != nil {
		return nil, err
	}
	if err := resp.Err(); err != nil {
		return nil, err
	}
	return decodeResponse(resp.Body)
}

func (p *proxy) version(ctx context.Context) (string, error) {
	v, err := p.call(ctx, "system.client_version")
	if err != nil {
		return "", err
	}
	if s := v.str(); s != "" {
		return s, nil
	}
	return "0.0.0", nil
}

func (p *proxy) torrents(ctx context.Context) ([]torrent, error) {
	params := []any{"", ""}
	for _, f := range multicallFields {
		params = append(params, f)
	}
	v, err := p.call(ctx, "d.multicall2", params...)
	if err != nil {
		return nil, err
	}
	var out []torrent
	for _, row := range v.values() {
		d := row.values()
		if len(d) < len(multicallFields) {
			return nil, fmt.Errorf("rTorrent returned %d fields of a torrent, want %d", len(d), len(multicallFields))
		}
		category := d[3].str()
		if c, err := url.QueryUnescape(category); err == nil {
			category = c
		}
		out = append(out, torrent{
			Name: d[0].str(), Hash: d[1].str(), Path: d[2].str(), Category: category,
			TotalSize: d[4].int(), RemainingSize: d[5].int(), DownRate: d[6].int(), Ratio: d[7].int(),
			IsOpen: d[8].int() != 0, IsActive: d[9].int() != 0, IsFinished: d[10].int() != 0, FinishedTime: d[11].int(),
		})
	}
	return out, nil
}

// commands are the commands run on a torrent as it is loaded.
func commands(label string, priority int, dir string) []any {
	var out []any
	if strings.TrimSpace(label) != "" {
		out = append(out, "d.custom1.set="+label)
	}
	if priority != priorityNormal {
		out = append(out, "d.priority.set="+strconv.Itoa(priority))
	}
	if strings.TrimSpace(dir) != "" {
		out = append(out, "d.directory.set="+dir)
	}
	return out
}

// load loads a torrent with the method stopped when AddStopped is set,
// else with the method start.
func (p *proxy) load(ctx context.Context, stopped, start string, source any, name, label string, priority int, dir string) error {
	method := start
	if p.settings.AddStopped {
		method = stopped
	}
	params := append([]any{"", source}, commands(label, priority, dir)...)
	v, err := p.call(ctx, method, params...)
	if err != nil {
		return err
	}
	if v.int() != 0 {
		return fmt.Errorf("could not add torrent: %s", name)
	}
	return nil
}

func (p *proxy) addURL(ctx context.Context, link, label string, priority int, dir string) error {
	return p.load(ctx, "load.normal", "load.start", link, link, label, priority, dir)
}

func (p *proxy) addFile(ctx context.Context, name string, data []byte, label string, priority int, dir string) error {
	return p.load(ctx, "load.raw", "load.raw_start", data, name, label, priority, dir)
}

func (p *proxy) remove(ctx context.Context, hash string) error {
	v, err := p.call(ctx, "d.erase", hash)
	if err != nil {
		return err
	}
	if v.int() != 0 {
		return fmt.Errorf("could not remove torrent: %s", hash)
	}
	return nil
}

func (p *proxy) setLabel(ctx context.Context, hash, label string) error {
	v, err := p.call(ctx, "d.custom1.set", hash, label)
	if err != nil {
		return err
	}
	if v.str() != label {
		return fmt.Errorf("could not set label to %s for torrent: %s", label, hash)
	}
	return nil
}

func (p *proxy) pushUniqueView(ctx context.Context, hash, view string) error {
	v, err := p.call(ctx, "d.views.push_back_unique", hash, view)
	if err != nil {
		return err
	}
	if v.int() != 0 {
		return fmt.Errorf("could not push unique view %s for torrent: %s", view, hash)
	}
	return nil
}

// hasTorrent reports whether rTorrent has a torrent, not just the
// metadata of a magnet link; cancellation is returned, other failures
// are a no.
func (p *proxy) hasTorrent(ctx context.Context, hash string) (bool, error) {
	v, err := p.call(ctx, "d.name", hash)
	if err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil && errors.Is(err, ctxErr) {
			return false, err
		}
		return false, nil
	}
	name := v.str()
	return strings.TrimSpace(name) != "" && name != hash+".meta", nil
}

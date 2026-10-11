package rqbit

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// state is a torrent's state, which rqbit names; numbers are read too,
// and unknown names are stateUnknown.
type state int

const stateUnknown state = -1

// The states of rqbit's TorrentState.
const (
	stateInitializing state = iota
	statePaused
	stateLive
	stateError
	stateInvalid
)

var stateNames = map[string]state{
	"initializing": stateInitializing, "paused": statePaused, "live": stateLive, "error": stateError, "invalid": stateInvalid,
}

func (s *state) UnmarshalJSON(b []byte) error {
	var name string
	if err := json.Unmarshal(b, &name); err != nil {
		var n int
		if err := json.Unmarshal(b, &n); err != nil {
			return fmt.Errorf("rqbit: torrent state %s", b)
		}
		*s = state(n)
		return nil
	}
	v, ok := stateNames[strings.ToLower(name)]
	if !ok {
		v = stateUnknown
	}
	*s = v
	return nil
}

// torrentWithStats is a torrent of /torrents?with_stats=true.
type torrentWithStats struct {
	ID           int64  `json:"id"`
	InfoHash     string `json:"info_hash"`
	Name         string `json:"name"`
	OutputFolder string `json:"output_folder"`
	Stats        *struct {
		State         state  `json:"state"`
		Error         string `json:"error"`
		ProgressBytes int64  `json:"progress_bytes"`
		UploadedBytes int64  `json:"uploaded_bytes"`
		TotalBytes    int64  `json:"total_bytes"`
		Finished      bool   `json:"finished"`
		Live          *struct {
			DownloadSpeed *struct {
				Mbps float64 `json:"mbps"`
			} `json:"download_speed"`
		} `json:"live"`
	} `json:"stats"`
}

// torrent is a torrent as Radarr's RQBitTorrent.
type torrent struct {
	Name          string
	Hash          string
	TotalSize     int64
	RemainingSize int64
	Ratio         float64
	// DownRate is in bytes a second.
	DownRate   int64
	IsFinished bool
	IsActive   bool
	HasError   bool
	Path       string
	Message    string
}

// proxy calls rqbit's HTTP API as Radarr's RQbitProxy.
type proxy struct {
	http     *http.Client
	settings *settings
}

// do sends a request to a resource and decodes the answer into out,
// when not nil.
func (p *proxy) do(ctx context.Context, method, resource, contentType string, body []byte, out any) error {
	var r io.Reader
	if body != nil {
		r = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, p.settings.BaseURL()+resource, r)
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Send(p.http, req)
	if err != nil {
		return err
	}
	if err := resp.Err(); err != nil {
		return err
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(resp.Body, out); err != nil {
		return fmt.Errorf("rqbit: %s %s: %w", method, resource, err)
	}
	return nil
}

func (p *proxy) version(ctx context.Context) (string, error) {
	var root struct {
		Version string `json:"version"`
	}
	if err := p.do(ctx, http.MethodGet, "/", "", nil, &root); err != nil {
		return "", err
	}
	return root.Version, nil
}

func (p *proxy) torrents(ctx context.Context) ([]torrent, error) {
	var list struct {
		Torrents []torrentWithStats `json:"torrents"`
	}
	if err := p.do(ctx, http.MethodGet, "/torrents?with_stats=true", "", nil, &list); err != nil {
		return nil, err
	}
	var out []torrent
	for _, t := range list.Torrents {
		s := t.Stats
		if s == nil {
			continue
		}
		tor := torrent{
			Name: t.Name, Hash: t.InfoHash, TotalSize: s.TotalBytes, Path: t.OutputFolder,
			RemainingSize: s.TotalBytes - s.ProgressBytes, IsFinished: s.Finished, Message: s.Error,
			IsActive: s.State == stateLive || s.State == stateInitializing,
			HasError: s.State == stateError || s.State == stateInvalid,
		}
		if !strings.HasSuffix(t.OutputFolder, t.Name) {
			tor.Path = client.JoinRemote(t.OutputFolder, t.Name)
		}
		if s.Live != nil && s.Live.DownloadSpeed != nil {
			// MiB a second.
			tor.DownRate = int64(s.Live.DownloadSpeed.Mbps * 1048576)
		}
		if s.ProgressBytes > 0 {
			tor.Ratio = float64(s.UploadedBytes) / float64(s.ProgressBytes)
		}
		out = append(out, tor)
	}
	return out, nil
}

// add adds a torrent from a link or a torrent file and returns its info
// hash.
func (p *proxy) add(ctx context.Context, contentType string, body []byte) (string, error) {
	var r struct {
		Details *struct {
			InfoHash string `json:"info_hash"`
		} `json:"details"`
	}
	if err := p.do(ctx, http.MethodPost, "/torrents?overwrite=true", contentType, body, &r); err != nil {
		return "", err
	}
	if r.Details == nil {
		return "", nil
	}
	return r.Details.InfoHash, nil
}

// remove removes a torrent, deleting its files or forgetting it.
func (p *proxy) remove(ctx context.Context, hash string, deleteData bool) error {
	action := "/forget"
	if deleteData {
		action = "/delete"
	}
	return p.do(ctx, http.MethodPost, "/torrents/"+url.PathEscape(hash)+action, "", nil, nil)
}

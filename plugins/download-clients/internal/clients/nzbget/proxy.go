package nzbget

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

// parameter is a post-processing parameter of a download.
type parameter struct {
	Name  string          `json:"Name"`
	Value json.RawMessage `json:"Value"`
}

// value returns the parameter's value as text.
func (p parameter) value() string {
	var s string
	if err := json.Unmarshal(p.Value, &s); err == nil {
		return s
	}
	return strings.TrimSpace(string(p.Value))
}

// droneID returns the value of the "drone" parameter, which Radarr and
// Sonarr identify their downloads by.
func droneID(params []parameter) (string, bool) {
	for _, p := range params {
		if p.Name == "drone" {
			return p.value(), true
		}
	}
	return "", false
}

// queueItem is a download of listgroups.
type queueItem struct {
	NzbID           int         `json:"NZBID"`
	FirstID         int         `json:"FirstID"`
	LastID          int         `json:"LastID"`
	NzbName         string      `json:"NZBName"`
	Category        string      `json:"Category"`
	FileSizeLo      uint32      `json:"FileSizeLo"`
	FileSizeHi      uint32      `json:"FileSizeHi"`
	RemainingSizeLo uint32      `json:"RemainingSizeLo"`
	RemainingSizeHi uint32      `json:"RemainingSizeHi"`
	PausedSizeLo    uint32      `json:"PausedSizeLo"`
	PausedSizeHi    uint32      `json:"PausedSizeHi"`
	ActiveDownloads int         `json:"ActiveDownloads"`
	Parameters      []parameter `json:"Parameters"`
}

// historyItem is a download of history.
type historyItem struct {
	ID           int         `json:"ID"`
	Name         string      `json:"Name"`
	Category     string      `json:"Category"`
	FileSizeLo   uint32      `json:"FileSizeLo"`
	FileSizeHi   uint32      `json:"FileSizeHi"`
	ParStatus    string      `json:"ParStatus"`
	UnpackStatus string      `json:"UnpackStatus"`
	MoveStatus   string      `json:"MoveStatus"`
	ScriptStatus string      `json:"ScriptStatus"`
	DeleteStatus string      `json:"DeleteStatus"`
	MarkStatus   string      `json:"MarkStatus"`
	DestDir      string      `json:"DestDir"`
	FinalDir     string      `json:"FinalDir"`
	Parameters   []parameter `json:"Parameters"`
}

// globalStatus is the server's state of status.
type globalStatus struct {
	DownloadRate   int64 `json:"DownloadRate"`
	DownloadPaused bool  `json:"DownloadPaused"`
}

// makeInt64 joins the two 32-bit halves NZBGet sends sizes in, as
// JavaScript has no 64-bit integers.
func makeInt64(hi, lo uint32) int64 { return int64(hi)<<32 | int64(lo) }

// versionTTL is how long the version NZBGet reports is kept.
const versionTTL = 24 * time.Hour

// proxy calls NZBGet's JSON-RPC as Radarr's NzbgetProxy.
type proxy struct {
	http     *http.Client
	settings *settings
	now      func() time.Time
	log      *slog.Logger
	ids      atomic.Int64

	mu      sync.Mutex
	version string
	checked time.Time
}

func (p *proxy) url() string { return p.settings.BaseURL() + "/jsonrpc" }

// call calls a method with params and decodes its result into out, when
// not nil.
func (p *proxy) call(ctx context.Context, out any, method string, params ...any) error {
	if params == nil {
		params = []any{}
	}
	body, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "method": method, "params": params, "id": p.ids.Add(1)})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.url(), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json-rpc, application/json")
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(p.settings.Username, p.settings.Password)
	resp, err := client.Send(p.http, req)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return &client.AuthError{Err: errors.New("authentication failed for NZBGet, please check your settings")}
	}
	if err := resp.Err(); err != nil {
		return fmt.Errorf("unable to connect to NZBGet: %w", err)
	}
	var r struct {
		Result json.RawMessage `json:"result"`
		Error  json.RawMessage `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &r); err != nil {
		return fmt.Errorf("NZBGet: %s: %w", method, err)
	}
	if len(r.Error) > 0 && string(r.Error) != "null" {
		return fmt.Errorf("error response received from NZBGet: %s", rpcError(r.Error))
	}
	if out != nil {
		if err := json.Unmarshal(r.Result, out); err != nil {
			return fmt.Errorf("NZBGet: %s: %w", method, err)
		}
	}
	return nil
}

// rpcError describes an error of a response as Radarr's ErrorModel.
func rpcError(raw json.RawMessage) string {
	var e struct {
		Name    string `json:"name"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(raw, &e); err != nil || e.Name == "" && e.Message == "" {
		return string(raw)
	}
	return fmt.Sprintf("Name: %s, Code: %d, Message: %s", e.Name, e.Code, e.Message)
}

// getVersion asks NZBGet's version and keeps it.
func (p *proxy) getVersion(ctx context.Context) (string, error) {
	var v string
	if err := p.call(ctx, &v, "version"); err != nil {
		return "", err
	}
	p.mu.Lock()
	p.version, p.checked = v, p.now()
	p.mu.Unlock()
	return v, nil
}

// hasVersion reports whether NZBGet's major version is at least
// minimum, by the version kept for a day.
func (p *proxy) hasVersion(ctx context.Context, minimum int) (bool, error) {
	p.mu.Lock()
	v := p.version
	if v != "" && p.now().Sub(p.checked) >= versionTTL {
		v = ""
	}
	p.mu.Unlock()
	if v == "" {
		var err error
		if v, err = p.getVersion(ctx); err != nil {
			return false, err
		}
	}
	major, err := strconv.Atoi(strings.FieldsFunc(v, func(r rune) bool { return r == '.' || r == '-' })[0])
	if err != nil {
		return false, fmt.Errorf("unable to parse NZBGet version %q", v)
	}
	return major >= minimum, nil
}

// newDroneID returns an ID such as Radarr's GUID without dashes.
func newDroneID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// downloadNzb appends an NZB and returns its drone ID; empty when NZBGet
// rejects it.
func (p *proxy) downloadNzb(ctx context.Context, data []byte, fileName, category string, priority int, paused bool) (string, error) {
	v16, err := p.hasVersion(ctx, 16)
	if err != nil {
		return "", err
	}
	if v16 {
		id := newDroneID()
		var n int
		if err := p.call(ctx, &n, "append", fileName, data, category, priority, false, paused, "", 0, "all",
			[]string{"drone", id}); err != nil {
			return "", err
		}
		if n <= 0 {
			return "", nil
		}
		return id, nil
	}
	v13, err := p.hasVersion(ctx, 13)
	if err != nil {
		return "", err
	}
	var nzbID int
	if v13 {
		var n int
		if err := p.call(ctx, &n, "append", fileName, data, category, priority, false, false, "", 0, "all"); err != nil {
			return "", err
		}
		if n <= 0 {
			return "", nil
		}
		nzbID = n
	} else {
		var ok bool
		if err := p.call(ctx, &ok, "append", fileName, category, priority, false, data); err != nil {
			return "", err
		}
		if !ok {
			return "", nil
		}
	}
	queue, err := p.queue(ctx)
	if err != nil {
		return "", err
	}
	// NZBGet 13 returns the download's ID; 12 is found by its name, the
	// file name without ".nzb", and edited by its last file's ID.
	editID := -1
	name := fileName[:max(0, len(fileName)-4)]
	for _, q := range queue {
		if v13 && q.NzbID == nzbID {
			editID = q.NzbID
			break
		}
		if !v13 && q.NzbName == name {
			editID = q.LastID
			break
		}
	}
	if editID < 0 {
		return "", nil
	}
	id := newDroneID()
	ok, err := p.editQueue(ctx, "GroupSetParameter", 0, "drone="+id, editID)
	if err != nil {
		return "", err
	}
	if ok {
		p.log.DebugContext(ctx, "NZBGet download drone parameter set", "drone", id)
	}
	return id, nil
}

func (p *proxy) globalStatus(ctx context.Context) (*globalStatus, error) {
	var s globalStatus
	if err := p.call(ctx, &s, "status"); err != nil {
		return nil, err
	}
	return &s, nil
}

func (p *proxy) queue(ctx context.Context) ([]queueItem, error) {
	var q []queueItem
	if err := p.call(ctx, &q, "listgroups"); err != nil {
		return nil, err
	}
	return q, nil
}

func (p *proxy) history(ctx context.Context) ([]historyItem, error) {
	var h []historyItem
	if err := p.call(ctx, &h, "history"); err != nil {
		return nil, err
	}
	return h, nil
}

// config returns NZBGet's configuration by option name.
func (p *proxy) config(ctx context.Context) (map[string]string, error) {
	var items []struct {
		Name  string `json:"Name"`
		Value string `json:"Value"`
	}
	if err := p.call(ctx, &items, "config"); err != nil {
		return nil, err
	}
	m := make(map[string]string, len(items))
	for _, it := range items {
		m[it.Name] = it.Value
	}
	return m, nil
}

func (p *proxy) editQueue(ctx context.Context, command string, offset int, text string, id int) (bool, error) {
	var ok bool
	err := p.call(ctx, &ok, "editqueue", command, offset, text, id)
	return ok, err
}

// remove deletes a download from the queue or the history: by NZBGet's
// ID for downloads not added by Mavio, else by drone ID.
func (p *proxy) remove(ctx context.Context, id string) error {
	queue, err := p.queue(ctx)
	if err != nil {
		return err
	}
	history, err := p.history(ctx)
	if err != nil {
		return err
	}
	n, err := strconv.Atoi(id)
	byNzbID := err == nil && len(id) < 10
	matches := func(nzbID int, params []parameter) bool {
		if byNzbID {
			return nzbID == n
		}
		d, ok := droneID(params)
		return ok && d == id
	}
	for _, q := range queue {
		if matches(q.NzbID, q.Parameters) {
			return p.removeQueued(ctx, &q)
		}
	}
	for _, h := range history {
		if matches(h.ID, h.Parameters) {
			return p.removeHistory(ctx, &h)
		}
	}
	p.log.WarnContext(ctx, "unable to remove item from NZBGet, unknown ID", "id", id)
	return nil
}

func (p *proxy) removeQueued(ctx context.Context, q *queueItem) error {
	ok, err := p.editQueue(ctx, "GroupFinalDelete", 0, "", q.NzbID)
	if err != nil {
		return err
	}
	if !ok {
		p.log.WarnContext(ctx, "failed to remove item from the NZBGet queue", "name", q.NzbName, "id", q.NzbID)
	}
	return nil
}

func (p *proxy) removeHistory(ctx context.Context, h *historyItem) error {
	ok, err := p.editQueue(ctx, "HistoryDelete", 0, "", h.ID)
	if err != nil {
		return err
	}
	if !ok {
		p.log.WarnContext(ctx, "failed to remove item from the NZBGet history", "name", h.Name, "id", h.ID)
	}
	return nil
}

package hadouken

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const title = "Droned.1998.1080p.WEB-DL-DRONE"

// fake is Hadouken's JSON-RPC API.
type fake struct {
	t        *testing.T
	mu       sync.Mutex
	torrents [][]any
	settings map[string]any
	version  string
	addHash  string
	calls    []call
}

type call struct {
	Method string
	Params []any
}

func newFake(t *testing.T) *fake {
	return &fake{t: t, version: "5.1.0.0", addHash: "cbc2f069fe8bb2f544eae707d75bcd3de9dcf951", settings: map[string]any{}}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/api" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if user, pass, _ := r.BasicAuth(); user != "admin" || pass != "pass" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var req struct {
		JSONRPC string `json:"jsonrpc"`
		Method  string `json:"method"`
		Params  []any  `json:"params"`
		ID      any    `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.JSONRPC != "2.0" {
		f.t.Errorf("request = %+v, %v, want JSON-RPC 2.0", req, err)
	}
	f.calls = append(f.calls, call{req.Method, req.Params})
	var result any = true
	switch req.Method {
	case "core.getSystemInfo":
		result = map[string]any{"commitish": "x", "branch": "master", "versions": map[string]string{"hadouken": f.version}}
	case "webui.list":
		result = map[string]any{"torrents": f.torrents}
	case "webui.getSettings":
		result = f.settings
	case "webui.addTorrent":
		result = f.addHash
	case "fail":
		_ = json.NewEncoder(w).Encode(map[string]any{"id": req.ID, "error": map[string]any{"code": -32601, "message": "Method not found"}})
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"id": req.ID, "result": result})
}

func (f *fake) methodCalls(method string) []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// newClient returns a client talking to f, with settings over the
// fixture's: Radarr's and Sonarr's default categories.
func newClient(t *testing.T, f *fake, settings map[string]any) client.Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{
		"host": host, "port": p, "username": "admin", "password": "pass",
		"movie_category": "radarr", "tv_category": "sonarr-tv",
	}
	for k, v := range settings {
		s[k] = v
	}
	raw, _ := json.Marshal(s)
	c, err := Definitions()[0].New(raw, client.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c
}

// hTorrent is a torrent as webui.list sends it.
type hTorrent struct {
	hash, name, label, err, savePath string
	state                            int
	size, progress, downloaded, rate int64
}

func (h hTorrent) wire() []any {
	a := make([]any, 27)
	a[0], a[1], a[2], a[3], a[4], a[5], a[6] = h.hash, h.state, h.name, h.size, h.progress, h.downloaded, 0
	a[9], a[11], a[21], a[26] = h.rate, h.label, h.err, h.savePath
	return a
}

// The state bits.
const (
	bitsDownloading = 1
	bitsChecking    = 2
	bitsPaused      = 32
	bitsQueued      = 64
)

// The torrents of Radarr's HadoukenFixture; finished torrents have a
// progress of 1000, as Hadouken reports them.
func queued() hTorrent {
	return hTorrent{hash: "HASH", state: bitsQueued, name: title, size: 1000, savePath: "somepath", label: "radarr"}
}

func downloading() hTorrent {
	return hTorrent{hash: "HASH", state: bitsDownloading, name: title, size: 1000, downloaded: 100, progress: 100, savePath: "somepath", label: "radarr"}
}

func failed() hTorrent {
	t := downloading()
	t.err = "some error"
	return t
}

func completed() hTorrent {
	return hTorrent{hash: "HASH", state: bitsPaused, name: title, size: 1000, downloaded: 1000, progress: 1000, savePath: "somepath", label: "radarr"}
}

func items(t *testing.T, c client.Client) []client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), client.Options{RemoveCompletedDownloads: true})
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	return items
}

// TestRequiredProperties ports the *_item_should_have_required_properties
// cases.
func TestRequiredProperties(t *testing.T) {
	tests := []struct {
		name    string
		torrent hTorrent
		want    client.Status
	}{
		{"queued", queued(), client.Queued},
		{"downloading", downloading(), client.Downloading},
		{"failed", failed(), client.Warning},
		{"completed", completed(), client.Completed},
		{"paused", hTorrent{hash: "HASH", state: bitsPaused, name: title, size: 1000, label: "radarr"}, client.Paused},
		{"checking", hTorrent{hash: "HASH", state: bitsChecking, name: title, size: 1000, progress: 1000, label: "radarr"}, client.Downloading},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.torrents = [][]any{tt.torrent.wire()}
			got := items(t, newClient(t, f, nil))
			if len(got) != 1 {
				t.Fatalf("Items() = %d items, want 1", len(got))
			}
			item := got[0]
			if item.Status != tt.want {
				t.Errorf("Status = %v, want = %v", item.Status, tt.want)
			}
			if item.ID != "HASH" || item.Title != title || item.TotalSize != 1000 || item.Kind != client.Movie || item.Category != "radarr" {
				t.Errorf("item = %+v, want the torrent's hash, name, size and the movies' label", item)
			}
			if tt.torrent.savePath != "" && item.OutputPath != "somepath/"+title {
				t.Errorf("OutputPath = %q, want = %q", item.OutputPath, "somepath/"+title)
			}
			switch tt.want {
			case client.Completed:
				if item.RemainingSize != 0 || item.RemainingTime == nil || *item.RemainingTime != 0 {
					t.Errorf("RemainingSize, RemainingTime = %d, %v, want = 0, 0", item.RemainingSize, item.RemainingTime)
				}
				if !item.CanBeRemoved || !item.CanMoveFiles {
					t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = true, true", item.CanBeRemoved, item.CanMoveFiles)
				}
			case client.Warning:
				if item.Message != "some error" {
					t.Errorf("Message = %q, want = %q", item.Message, "some error")
				}
			default:
				if item.RemainingSize == 0 || item.CanBeRemoved {
					t.Errorf("RemainingSize, CanBeRemoved = %d, %v, want non-zero, false", item.RemainingSize, item.CanBeRemoved)
				}
			}
		})
	}
}

func TestETAAndRatio(t *testing.T) {
	f := newFake(t)
	tor := downloading()
	tor.rate = 10
	w := tor.wire()
	w[6] = 50
	f.torrents = [][]any{w}
	item := items(t, newClient(t, f, nil))[0]
	if item.RemainingTime == nil || *item.RemainingTime != 100*time.Second {
		t.Errorf("RemainingTime = %v, want = 100s", item.RemainingTime)
	}
	if item.SeedRatio == nil || *item.SeedRatio != 0.5 {
		t.Errorf("SeedRatio = %v, want = 0.5", item.SeedRatio)
	}
}

// TestDownloadIDUppercase ports GetItems_should_return_torrents_with_DownloadId_uppercase.
func TestDownloadIDUppercase(t *testing.T) {
	f := newFake(t)
	tor := completed()
	tor.hash = "hash"
	f.torrents = [][]any{tor.wire()}
	if got := items(t, newClient(t, f, nil))[0].ID; got != "HASH" {
		t.Errorf("ID = %q, want = %q", got, "HASH")
	}
}

// TestCategories ports GetItems_should_ignore_torrents_with_a_different_category,
// for either kind.
func TestCategories(t *testing.T) {
	tor := func(hash, label string) []any {
		h := completed()
		h.hash, h.label = hash, label
		return h.wire()
	}
	tests := []struct {
		name     string
		settings map[string]any
		want     map[string]client.Kind
	}{
		{"fixture", nil, map[string]client.Kind{"A": client.Movie, "B": client.Series}},
		{"no tv category", map[string]any{"tv_category": ""}, map[string]client.Kind{"A": 0, "B": client.Series, "C": client.Series, "D": client.Series}},
		{"no categories", map[string]any{"movie_category": "", "tv_category": ""}, map[string]client.Kind{"A": 0, "B": 0, "C": 0, "D": 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.torrents = [][]any{tor("A", "radarr"), tor("B", "sonarr-tv"), tor("C", "radarr-other"), tor("D", "")}
			got := map[string]client.Kind{}
			for _, item := range items(t, newClient(t, f, tt.settings)) {
				got[item.ID] = item.Kind
			}
			if len(got) != len(tt.want) {
				t.Fatalf("Items() kinds = %v, want = %v", got, tt.want)
			}
			for id, k := range tt.want {
				if g, ok := got[id]; !ok || g != k {
					t.Errorf("Items() kinds = %v, want = %v", got, tt.want)
				}
			}
		})
	}
}

func TestBadTorrentSkipped(t *testing.T) {
	f := newFake(t)
	bad := queued().wire()
	bad[3] = map[string]any{}
	f.torrents = [][]any{bad, {"short"}, downloading().wire()}
	got := items(t, newClient(t, f, nil))
	if len(got) != 1 || got[0].Status != client.Downloading {
		t.Errorf("Items() = %+v, want the downloading torrent alone", got)
	}
}

func TestDownload(t *testing.T) {
	data := []byte("d4:infod6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
	tests := []struct {
		name    string
		release client.Release
		addHash string
		want    string
		params  string
	}{
		// Download_should_return_unique_id
		{
			"unique id",
			client.Release{Kind: client.Movie, Data: data},
			"cbc2f069fe8bb2f544eae707d75bcd3de9dcf951",
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951",
			`["file","` + base64.StdEncoding.EncodeToString(data) + `",{"label":"radarr"}]`,
		},
		// Download_from_torrent_file_should_return_hash_uppercase
		{
			"torrent file uppercase",
			client.Release{Kind: client.Series, Data: data},
			"hash", "HASH",
			`["file","` + base64.StdEncoding.EncodeToString(data) + `",{"label":"sonarr-tv"}]`,
		},
		// Download_from_magnet_link_should_return_hash_uppercase
		{
			"magnet uppercase",
			client.Release{Kind: client.Movie, MagnetURL: "magnet:?xt=urn:btih:a45129e59d8750f9da982f53552b1e4f0457ee9f"},
			"", "A45129E59D8750F9DA982F53552B1E4F0457EE9F",
			`["url","magnet:?xt=urn:btih:a45129e59d8750f9da982f53552b1e4f0457ee9f",{"label":"radarr"}]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.addHash = tt.addHash
			r := tt.release
			r.Title = title
			id, err := newClient(t, f, nil).Add(context.Background(), &r)
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if id != tt.want || strings.ToUpper(id) != id {
				t.Errorf("Add() = %q, want = %q", id, tt.want)
			}
			calls := f.methodCalls("webui.addTorrent")
			if len(calls) != 1 {
				t.Fatalf("webui.addTorrent calls = %d, want 1", len(calls))
			}
			if got, _ := json.Marshal(calls[0].Params); string(got) != tt.params {
				t.Errorf("params = %s, want = %s", got, tt.params)
			}
		})
	}
}

// TestInfo ports should_return_status_with_outputdirs.
func TestInfo(t *testing.T) {
	f := newFake(t)
	f.settings["bittorrent.defaultSavePath"] = "C:/Downloads/Downloading/deluge"
	c := newClient(t, f, map[string]any{"host": "127.0.0.1"})
	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatalf("Info() error = %v", err)
	}
	if !info.IsLocalhost || !slices.Equal(info.OutputRootFolders, []string{"C:/Downloads/Downloading/deluge"}) {
		t.Errorf("Info() = %+v, want localhost with C:/Downloads/Downloading/deluge", info)
	}
}

// TestTest ports Test_should_return_validation_failure_for_old_hadouken.
func TestTest(t *testing.T) {
	tests := []struct {
		version string
		ok      bool
	}{
		{"5.0.0.0", false},
		{"5.1", true},
		{"5.1.0.0", true},
		{"6.0", true},
		{"bogus", false},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.version = tt.version
		if err := newClient(t, f, nil).Test(context.Background()); (err == nil) != tt.ok {
			t.Errorf("version %q: Test() error = %v, want ok = %v", tt.version, err, tt.ok)
		}
	}
	var auth *client.AuthError
	if err := newClient(t, newFake(t), map[string]any{"password": "wrong"}).Test(context.Background()); !errors.As(err, &auth) {
		t.Errorf("Test() with a wrong password error = %v, want an AuthError", err)
	}
}

func TestRemove(t *testing.T) {
	f := newFake(t)
	c := newClient(t, f, nil)
	for _, del := range []bool{false, true} {
		if err := c.Remove(context.Background(), client.Item{ID: "HASH"}, del); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
	}
	var got []string
	for _, c := range f.methodCalls("webui.perform") {
		p, _ := json.Marshal(c.Params)
		got = append(got, string(p))
	}
	if want := []string{`["remove",["HASH"]]`, `["removedata",["HASH"]]`}; !slices.Equal(got, want) {
		t.Errorf("webui.perform params = %v, want = %v", got, want)
	}
}

func TestRPCError(t *testing.T) {
	f := newFake(t)
	h := newClient(t, f, nil).(*hadouken)
	if err := h.proxy.call(context.Background(), nil, "fail"); err == nil || !strings.Contains(err.Error(), "Method not found") {
		t.Errorf("call() error = %v, want Hadouken's error", err)
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"credentials", `{"username":"u","password":"p"}`, true},
		{"no username", `{"password":"p"}`, false},
		{"no password", `{"username":"u"}`, false},
		{"url base", `{"username":"u","password":"p","url_base":"/hadouken"}`, true},
		{"url as url base", `{"username":"u","password":"p","url_base":"http://host"}`, false},
		{"unknown", `{"username":"u","password":"p","bogus":1}`, false},
	}
	for _, tt := range tests {
		_, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
		if (err == nil) != tt.ok {
			t.Errorf("%s: New() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

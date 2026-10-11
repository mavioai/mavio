package rqbit

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const title = "Droned.1998.1080p.WEB-DL-DRONE"

// fake is an rqbit HTTP API.
type fake struct {
	t        *testing.T
	mu       sync.Mutex
	torrents []map[string]any
	version  string
	// addHash is the info hash adding a torrent answers.
	addHash string
	posts   []post
}

type post struct {
	Path, ContentType string
	Body              string
}

func newFake(t *testing.T) *fake { return &fake{t: t, version: "8.0.0"} }

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	if r.Method == http.MethodPost {
		f.posts = append(f.posts, post{Path: r.URL.RequestURI(), ContentType: r.Header.Get("Content-Type"), Body: string(body)})
	}
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/":
		_ = json.NewEncoder(w).Encode(map[string]any{"apis": map[string]string{}, "server": "rqbit", "version": f.version})
	case r.Method == http.MethodGet && r.URL.Path == "/torrents" && r.URL.Query().Get("with_stats") == "true":
		_ = json.NewEncoder(w).Encode(map[string]any{"torrents": f.torrents})
	case r.Method == http.MethodPost && r.URL.Path == "/torrents" && r.URL.Query().Get("overwrite") == "true":
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": 1, "details": map[string]any{"info_hash": f.addHash, "name": title, "files": []any{}},
			"output_folder": "/downloads", "seen_peers": []string{},
		})
	case r.Method == http.MethodPost && strings.HasPrefix(r.URL.Path, "/torrents/"):
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func newClient(t *testing.T, f *fake, settings map[string]any) *rqbit {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{"host": host, "port": p}
	for k, v := range settings {
		s[k] = v
	}
	raw, _ := json.Marshal(s)
	c, err := Definitions()[0].New(raw, client.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c.(*rqbit)
}

// rqTorrent is a torrent of the list with stats; remaining is the bytes
// left of 1000.
func rqTorrent(hash, name, folder, st string, remaining int64, finished bool) map[string]any {
	return map[string]any{
		"id": 1, "info_hash": hash, "name": name, "output_folder": folder,
		"stats": map[string]any{
			"state": st, "file_progress": []int64{1000 - remaining}, "error": nil,
			"progress_bytes": 1000 - remaining, "uploaded_bytes": 0, "total_bytes": 1000, "finished": finished,
			"live": nil,
		},
	}
}

// The torrents of Radarr's RQBitFixture.
func queued() map[string]any { return rqTorrent("hash", title, "/somepath/", "paused", 1000, false) }

func downloading() map[string]any { return rqTorrent("hash", title, "/somepath/", "live", 100, false) }

func failed() map[string]any { return rqTorrent("hash", title, "/somepath/", "paused", 1000, false) }

func completed() map[string]any { return rqTorrent("hash", title, "/somepath/", "paused", 0, true) }

func items(t *testing.T, c client.Client, opts client.Options) []client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), opts)
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	return items
}

func TestItemStatus(t *testing.T) {
	tests := []struct {
		name    string
		torrent map[string]any
		want    client.Status
	}{
		// queued_item_should_have_required_properties
		{"queued", queued(), client.Paused},
		// downloading_item_should_have_required_properties
		{"downloading", downloading(), client.Downloading},
		// failed_item_should_have_required_properties
		{"failed", failed(), client.Paused},
		// completed_download_should_have_required_properties
		{"completed", completed(), client.Completed},
		{"initializing", rqTorrent("hash", title, "/somepath/", "initializing", 1000, false), client.Downloading},
		{"error", rqTorrent("hash", title, "/somepath/", "error", 1000, false), client.Warning},
		{"numeric live", rqTorrent("hash", title, "/somepath/", "", 100, false), client.Downloading},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			if tt.name == "numeric live" {
				tt.torrent["stats"].(map[string]any)["state"] = 2
			}
			f.torrents = []map[string]any{tt.torrent}
			got := items(t, newClient(t, f, nil), client.Options{RemoveCompletedDownloads: true})
			if len(got) != 1 {
				t.Fatalf("Items() = %d items, want 1", len(got))
			}
			item := got[0]
			if item.Status != tt.want {
				t.Errorf("Status = %v, want = %v", item.Status, tt.want)
			}
			if item.ID != "hash" || item.Title != title || item.OutputPath != "/somepath/"+title {
				t.Errorf("item = %+v, want the torrent's hash, name and path", item)
			}
			if (item.RemainingSize == 0) != (tt.want == client.Completed) {
				t.Errorf("RemainingSize = %d, want zero only when completed", item.RemainingSize)
			}
			if tt.want == client.Completed && (item.RemainingTime == nil || *item.RemainingTime != 0) {
				t.Errorf("RemainingTime = %v, want = 0", item.RemainingTime)
			}
		})
	}
}

func TestItemValues(t *testing.T) {
	f := newFake(t)
	tor := rqTorrent("abc", title, "/downloads/"+title, "live", 600, false)
	stats := tor["stats"].(map[string]any)
	stats["uploaded_bytes"], stats["error"] = 200, "tracker down"
	stats["live"] = map[string]any{"download_speed": map[string]any{"mbps": 100.0 / 1048576, "human_readable": "100 B/s"}}
	f.torrents = []map[string]any{tor}
	got := items(t, newClient(t, f, nil), client.Options{})
	if len(got) != 1 {
		t.Fatalf("Items() = %d items, want 1", len(got))
	}
	item := got[0]
	if item.OutputPath != "/downloads/"+title {
		t.Errorf("OutputPath = %q, want = %q", item.OutputPath, "/downloads/"+title)
	}
	if item.RemainingTime == nil || *item.RemainingTime != 6*time.Second {
		t.Errorf("RemainingTime = %v, want = 6s", item.RemainingTime)
	}
	if item.SeedRatio == nil || *item.SeedRatio != 0.5 || item.Message != "tracker down" || item.Kind != 0 {
		t.Errorf("SeedRatio, Message, Kind = %v, %q, %v, want = 0.5, %q, 0", item.SeedRatio, item.Message, item.Kind, "tracker down")
	}
}

func TestItemsIgnorePaths(t *testing.T) {
	tests := []struct {
		name, bad string
	}{
		// GetItems_should_ignore_torrents_with_empty_path
		{"empty path", ""},
		// GetItems_should_ignore_torrents_with_relative_path
		{"relative path", "./relative/"},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.torrents = []map[string]any{
			rqTorrent("Hash1", "", tt.bad, "live", 100, false),
			rqTorrent("Hash2", "Test2", "/valid/path/", "live", 100, false),
		}
		got := items(t, newClient(t, f, nil), client.Options{})
		if len(got) != 1 || got[0].Title != "Test2" {
			t.Errorf("%s: Items() = %+v, want only Test2", tt.name, got)
		}
	}
}

// torrentFile is a minimal torrent file.
func torrentFile() []byte {
	return []byte("d4:infod6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
}

func TestAdd(t *testing.T) {
	const magnet = "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp"
	const hash = "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951"
	tests := []struct {
		name    string
		release client.Release
		addHash string
		body    string
		want    string
	}{
		// Download_should_return_unique_id: the ID in the case rqbit
		// gives it.
		{"torrent file", client.Release{Data: torrentFile()}, strings.ToLower(hash), string(torrentFile()), strings.ToLower(hash)},
		// Download_should_get_hash_from_magnet_url
		{"magnet", client.Release{MagnetURL: magnet}, strings.ToLower(hash), magnet, strings.ToLower(hash)},
		{"no details", client.Release{MagnetURL: magnet}, "", magnet, hash},
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
			if id != tt.want {
				t.Errorf("Add() = %q, want = %q", id, tt.want)
			}
			if len(f.posts) != 1 || f.posts[0].Path != "/torrents?overwrite=true" || f.posts[0].Body != tt.body {
				t.Errorf("posts = %+v, want the release posted to /torrents?overwrite=true", f.posts)
			}
		})
	}
}

func TestSeedLimits(t *testing.T) {
	data := torrentFile()
	hash, _ := client.InfoHash(data)
	tests := []struct {
		name      string
		ratio     *float64
		seedTime  *time.Duration
		uploaded  int
		opts      client.Options
		removable bool
	}{
		{"ratio reached", ptr(1.0), nil, 1000, client.Options{RemoveCompletedDownloads: true}, true},
		{"ratio not reached", ptr(2.0), nil, 1000, client.Options{RemoveCompletedDownloads: true}, false},
		{"seed time only", nil, ptr(time.Hour), 1000, client.Options{RemoveCompletedDownloads: true}, false},
		{"not removing completed", ptr(1.0), nil, 1000, client.Options{}, false},
		{"no seed configuration", nil, nil, 1000, client.Options{RemoveCompletedDownloads: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.addHash = hash
			tor := rqTorrent(strings.ToLower(hash), title, "/d/", "paused", 0, true)
			tor["stats"].(map[string]any)["uploaded_bytes"] = tt.uploaded
			f.torrents = []map[string]any{tor}
			c := newClient(t, f, nil)
			r := client.Release{Title: title, Data: data, SeedRatio: tt.ratio, SeedTime: tt.seedTime}
			if _, err := c.Add(context.Background(), &r); err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			got := items(t, c, tt.opts)
			if len(got) != 1 {
				t.Fatalf("Items() = %d items, want 1", len(got))
			}
			if got[0].CanBeRemoved != tt.removable || got[0].CanMoveFiles != tt.removable {
				t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = %v", got[0].CanBeRemoved, got[0].CanMoveFiles, tt.removable)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestRemove(t *testing.T) {
	for _, deleteData := range []bool{false, true} {
		f := newFake(t)
		if err := newClient(t, f, nil).Remove(context.Background(), client.Item{ID: "ABC"}, deleteData); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		want := "/torrents/abc/forget"
		if deleteData {
			want = "/torrents/abc/delete"
		}
		if len(f.posts) != 1 || f.posts[0].Path != want {
			t.Errorf("posts = %+v, want = %s", f.posts, want)
		}
	}
}

func TestInfo(t *testing.T) {
	// should_return_status_with_outputdirs
	info, err := newClient(t, newFake(t), map[string]any{"host": "127.0.0.1"}).Info(context.Background())
	if err != nil || !info.IsLocalhost {
		t.Errorf("Info() = %+v, %v, want localhost", info, err)
	}
}

func TestTest(t *testing.T) {
	tests := []struct {
		version string
		ok      bool
	}{
		{"8.0.0", true},
		{"v8.1.0-beta", true},
		{"7.9.9", false},
		{"", false},
		{"eight", false},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.version = tt.version
		if err := newClient(t, f, nil).Test(context.Background()); (err == nil) != tt.ok {
			t.Errorf("%q: Test() error = %v, want ok = %v", tt.version, err, tt.ok)
		}
	}
	f := newFake(t)
	if err := newClient(t, f, map[string]any{"url_base": "/missing"}).Test(context.Background()); err == nil {
		t.Errorf("Test() at a wrong path error = nil, want an error")
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"defaults", `{}`, true},
		{"unknown", `{"bogus":1}`, false},
		{"no host", `{"host":""}`, false},
		{"url as base", `{"url_base":"http://rqbit"}`, false},
	}
	for _, tt := range tests {
		c, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
		if (err == nil) != tt.ok {
			t.Errorf("%s: New() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
		if tt.name == "defaults" {
			if got := c.(*rqbit).settings.BaseURL(); got != "http://localhost:3030" {
				t.Errorf("BaseURL() = %q, want = %q", got, "http://localhost:3030")
			}
		}
	}
}

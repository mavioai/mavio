package utorrent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const title = "Droned.S01E01.Pilot.1080p.WEB-DL-DRONE"

// tor is a torrent of the fake.
type tor struct {
	hash, name, label, root string
	status                  int
	size, remaining         int64
	progress                float64
	eta                     int64
}

// array is the torrent as uTorrent lists it.
func (t tor) array() []any {
	a := make([]any, 29)
	a[0], a[1], a[2], a[3] = t.hash, t.status, t.name, t.size
	a[4], a[5], a[6], a[7] = int(t.progress*1000), 0, 0, 0
	for i := 8; i <= 17; i++ {
		a[i] = 0
	}
	a[10], a[11] = t.eta, t.label
	a[18], a[19] = t.remaining, "http://test.mavio/not-a-real-torrent.torrent"
	a[20], a[21], a[22], a[23], a[24], a[25] = "", "", "", 0, 0, ""
	a[26], a[27], a[28] = t.root, "", ""
	return a
}

// diff is an incremental list the fake answers for a cache ID.
type diff struct {
	changed []tor
	removed []string
	cacheID string
}

// fake is uTorrent's Web UI API.
type fake struct {
	t        *testing.T
	mu       sync.Mutex
	torrents []tor
	cacheID  string
	diffs    map[string]diff
	build    int
	config   [][]any
	calls    []url.Values
	lists    []string
	files    []string
	// expireToken makes the next API request fail with 400 once.
	expireToken bool
	tokens      int
}

func newFake(t *testing.T) *fake {
	return &fake{t: t, build: 30000, diffs: map[string]diff{}}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if user, pass, _ := r.BasicAuth(); user != "admin" || pass != "pass" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	switch r.URL.Path {
	case "/gui/token.html":
		f.tokens++
		http.SetCookie(w, &http.Cookie{Name: "GUID", Value: "guid"})
		_, _ = io.WriteString(w, "<html><div id='token' style='display:none;'>tok</div></html>")
		return
	case "/gui/":
	default:
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	if c, err := r.Cookie("GUID"); err != nil || c.Value != "guid" || q.Get("token") != "tok" || f.expireToken {
		f.expireToken = false
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(r.URL.RawQuery, "token=") {
		f.t.Errorf("query = %s, want the token first", r.URL.RawQuery)
	}
	var resp map[string]any
	switch {
	case q.Get("list") == "1":
		f.lists = append(f.lists, q.Get("cid"))
		if d, ok := f.diffs[q.Get("cid")]; ok && q.Get("cid") != "" {
			var changed [][]any
			for _, t := range d.changed {
				changed = append(changed, t.array())
			}
			resp = map[string]any{"build": f.build, "torrentp": changed, "torrentm": d.removed, "torrentc": d.cacheID}
			break
		}
		torrents := [][]any{}
		for _, t := range f.torrents {
			torrents = append(torrents, t.array())
		}
		resp = map[string]any{"build": f.build, "torrents": torrents, "label": [][]any{}, "torrentc": f.cacheID}
	case q.Get("action") == "getsettings":
		resp = map[string]any{"build": f.build, "settings": f.config}
	default:
		if q.Get("action") == "add-file" {
			file, h, err := r.FormFile("torrent_file")
			if err != nil {
				f.t.Errorf("add-file: %v", err)
				break
			}
			data, _ := io.ReadAll(file)
			f.files = append(f.files, h.Filename+":"+string(data))
		}
		q.Del("token")
		f.calls = append(f.calls, q)
		resp = map[string]any{"build": f.build}
	}
	_ = json.NewEncoder(w).Encode(resp)
}

// actions returns the actions called, with their parameters in order.
func (f *fake) actions() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, c := range f.calls {
		s := c.Get("action")
		if s == "setprops" {
			for i := range c["s"] {
				s += " " + c["s"][i] + "=" + c["v"][i]
			}
		}
		out = append(out, s)
	}
	return out
}

func newClient(t *testing.T, f *fake, settings map[string]any) client.Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{
		"host": host, "port": p, "username": "admin", "password": "pass", "movie_category": "movie", "tv_category": "tv",
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

// The torrents of Radarr's and Sonarr's UTorrentFixture.
func queued() tor {
	return tor{hash: "HASH", status: statusQueued | statusLoaded, name: title, size: 1000, remaining: 1000, label: "movie", root: "somepath"}
}

func downloading() tor {
	return tor{hash: "HASH", status: statusStarted | statusLoaded, name: title, size: 1000, remaining: 100, progress: 0.9, label: "movie", root: "somepath"}
}

func failed() tor {
	return tor{hash: "HASH", status: statusError, name: title, size: 1000, remaining: 100, progress: 0.9, label: "movie", root: "somepath"}
}

func completed() tor {
	return tor{hash: "HASH", status: statusChecked | statusLoaded, name: title, size: 1000, progress: 1, label: "movie", root: "somepath"}
}

func items(t *testing.T, c client.Client) []client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), client.Options{RemoveCompletedDownloads: true})
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	return items
}

func single(t *testing.T, c client.Client) client.Item {
	t.Helper()
	got := items(t, c)
	if len(got) != 1 {
		t.Fatalf("Items() = %d items, want = 1", len(got))
	}
	return got[0]
}

func TestItemStatus(t *testing.T) {
	tests := []struct {
		name      string
		torrent   tor
		status    int
		want      client.Status
		removable bool
	}{
		// *_item_should_have_required_properties
		{"queued", queued(), statusQueued | statusLoaded, client.Queued, false},
		{"downloading", downloading(), statusStarted | statusLoaded, client.Downloading, false},
		{"failed", failed(), statusError, client.Warning, true},
		{"completed", completed(), statusChecked | statusLoaded, client.Completed, true},
		// GetItems_should_return_queued_item_as_downloadItemStatus
		{"queued loaded", queued(), statusLoaded, client.Queued, true},
		{"queued checking", queued(), statusLoaded | statusChecking, client.Queued, true},
		{"queued queued", queued(), statusLoaded | statusQueued, client.Queued, false},
		{"queued started", queued(), statusLoaded | statusStarted, client.Downloading, false},
		{"queued queued started", queued(), statusLoaded | statusQueued | statusStarted, client.Downloading, false},
		// GetItems_should_return_downloading_item_as_downloadItemStatus
		{"downloading checking", downloading(), statusLoaded | statusChecking, client.Queued, true},
		{"downloading checked queued", downloading(), statusLoaded | statusChecked | statusQueued, client.Queued, false},
		{"downloading started", downloading(), statusLoaded | statusStarted, client.Downloading, false},
		{"downloading queued started", downloading(), statusLoaded | statusQueued | statusStarted, client.Downloading, false},
		// GetItems_should_return_completed_item_as_downloadItemStatus
		{"completed checking", completed(), statusLoaded | statusChecking, client.Queued, true},
		{"completed checked", completed(), statusLoaded | statusChecked, client.Completed, true},
		{"completed checked queued", completed(), statusLoaded | statusChecked | statusQueued, client.Completed, false},
		{"completed checked started", completed(), statusLoaded | statusChecked | statusStarted, client.Completed, false},
		{"completed checked queued paused", completed(), statusLoaded | statusChecked | statusQueued | statusPaused, client.Completed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			tt.torrent.status = tt.status
			f.torrents = []tor{tt.torrent}
			item := single(t, newClient(t, f, nil))
			if item.Status != tt.want {
				t.Errorf("Status = %v, want = %v", item.Status, tt.want)
			}
			if item.CanBeRemoved != tt.removable || item.CanMoveFiles != tt.removable {
				t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = %v", item.CanBeRemoved, item.CanMoveFiles, tt.removable)
			}
			if item.ID != "HASH" || item.Title != title || item.Category != "movie" || item.Kind != client.Movie {
				t.Errorf("item = %+v, want the torrent's hash, name and label", item)
			}
			if item.RemainingSize != tt.torrent.remaining {
				t.Errorf("RemainingSize = %d, want = %d", item.RemainingSize, tt.torrent.remaining)
			}
			if item.RemainingTime == nil || *item.RemainingTime != 0 {
				t.Errorf("RemainingTime = %v, want = 0", item.RemainingTime)
			}
			if tt.want == client.Warning && item.Message == "" {
				t.Errorf("Message is empty, want the error")
			}
		})
	}
}

func TestItemsFilter(t *testing.T) {
	tests := []struct {
		name     string
		label    string
		settings map[string]any
		want     int
		kind     client.Kind
	}{
		// GetItems_should_ignore_downloads_from_other_categories
		{"other category", "myowncat", nil, 0, 0},
		{"movie", "movie", nil, 1, client.Movie},
		{"series", "tv", nil, 1, client.Series},
		{"one category", "both", map[string]any{"movie_category": "both", "tv_category": "both"}, 1, 0},
	}
	for _, tt := range tests {
		f := newFake(t)
		tr := completed()
		tr.label = tt.label
		f.torrents = []tor{tr}
		got := items(t, newClient(t, f, tt.settings))
		if len(got) != tt.want {
			t.Fatalf("%s: Items() = %d items, want = %d", tt.name, len(got), tt.want)
		}
		if tt.want > 0 && got[0].Kind != tt.kind {
			t.Errorf("%s: Kind = %v, want = %v", tt.name, got[0].Kind, tt.kind)
		}
	}
}

func TestOutputPath(t *testing.T) {
	tests := []struct{ root, want string }{
		// should_combine_drive_letter
		{"D:", `D:\` + title},
		{`C:\Downloads\` + title, `C:\Downloads\` + title},
		{`C:\Downloads\`, `C:\Downloads\` + title},
		{"/downloads", "/downloads/" + title},
		{"/downloads/" + title, "/downloads/" + title},
	}
	for _, tt := range tests {
		f := newFake(t)
		tr := completed()
		tr.root = tt.root
		f.torrents = []tor{tr}
		if got := single(t, newClient(t, f, nil)).OutputPath; got != tt.want {
			t.Errorf("root %q: OutputPath = %q, want = %q", tt.root, got, tt.want)
		}
	}
}

// GetItems_should_query_with_cache_id_if_available
func TestCacheID(t *testing.T) {
	f := newFake(t)
	d := downloading()
	d.status = statusStarted
	f.torrents, f.cacheID = []tor{d}, "abc"
	c := newClient(t, f, nil)
	single(t, c)

	f.torrents, f.cacheID = []tor{d, queued()}, "abc2"
	f.diffs["abc"] = diff{changed: []tor{queued()}, cacheID: "abc2"}
	f.diffs["abc2"] = diff{cacheID: "abc2"}
	if got := single(t, c); got.Status != client.Queued {
		t.Errorf("Status = %v, want the changed torrent's %v", got.Status, client.Queued)
	}
	if got := single(t, c); got.Status != client.Queued {
		t.Errorf("Status = %v, want the cached torrent's %v", got.Status, client.Queued)
	}
	if want := []string{"", "abc", "abc2"}; strings.Join(f.lists, ",") != strings.Join(want, ",") {
		t.Errorf("cache IDs = %q, want = %q", f.lists, want)
	}

	f.diffs["abc2"] = diff{removed: []string{"HASH"}, cacheID: "abc3"}
	if got := items(t, c); len(got) != 0 {
		t.Errorf("Items() = %d items, want the removed torrent gone", len(got))
	}
}

// torrentFile is a minimal torrent file.
func torrentFile() []byte {
	return []byte("d4:infod6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
}

func ptr[T any](v T) *T { return &v }

func TestAdd(t *testing.T) {
	data := torrentFile()
	hash, err := client.InfoHash(data)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name     string
		settings map[string]any
		release  client.Release
		wantID   string
		want     []string
	}{
		// Download_should_return_unique_id
		{
			"torrent file", nil,
			client.Release{Kind: client.Movie, Data: data},
			hash,
			[]string{"add-file", "setprops label=movie", "start"},
		},
		{
			"series", nil,
			client.Release{Kind: client.Series, Data: data},
			hash,
			[]string{"add-file", "setprops label=tv", "start"},
		},
		// Download_should_get_hash_from_magnet_url
		{
			"magnet", nil,
			client.Release{Kind: client.Movie, MagnetURL: "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp"},
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951",
			[]string{"add-url", "setprops label=movie", "start"},
		},
		{
			"seeding, first and paused",
			map[string]any{"older_tv_priority": priorityFirst, "initial_state": statePause},
			client.Release{Kind: client.Series, Data: data, SeedRatio: ptr(1.5), SeedTime: ptr(90 * time.Minute)},
			hash,
			[]string{"add-file", "setprops seed_override=1 seed_ratio=1500 seed_time=5400", "setprops label=tv", "queuetop", "pause"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			r := tt.release
			r.Title = title
			id, err := newClient(t, f, tt.settings).Add(context.Background(), &r)
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if id != tt.wantID {
				t.Errorf("Add() = %q, want = %q", id, tt.wantID)
			}
			if got := f.actions(); strings.Join(got, "; ") != strings.Join(tt.want, "; ") {
				t.Errorf("actions = %q, want = %q", got, tt.want)
			}
			for _, c := range f.calls {
				if c.Get("action") != "add-file" && c.Get("action") != "add-url" && c.Get("hash") != tt.wantID {
					t.Errorf("%s hash = %q, want = %q", c.Get("action"), c.Get("hash"), tt.wantID)
				}
			}
			if r.Data != nil && (len(f.files) != 1 || f.files[0] != title+".torrent:"+string(data)) {
				t.Errorf("files = %q, want the torrent file", f.files)
			}
			if r.MagnetURL != "" && f.calls[0].Get("s") != r.MagnetURL {
				t.Errorf("add-url s = %q, want = %q", f.calls[0].Get("s"), r.MagnetURL)
			}
		})
	}
}

// should_return_status_with_outputdirs
func TestInfo(t *testing.T) {
	tests := []struct {
		name   string
		config [][]any
		want   []string
	}{
		{
			"completed with label",
			[][]any{
				{"dir_active_download_flag", 1, "true", map[string]any{}},
				{"dir_active_download", 2, `C:\Downloads\Downloading\utorrent`},
				{"dir_completed_download", 2, `C:\Downloads\Finished\utorrent`},
				{"dir_completed_download_flag", 1, "true"},
				{"dir_add_label", 1, "true"},
			},
			[]string{`C:\Downloads\Finished\utorrent\movie`, `C:\Downloads\Finished\utorrent\tv`},
		},
		{
			"active",
			[][]any{
				{"dir_active_download_flag", 1, "true"},
				{"dir_active_download", 2, "/downloads/active"},
				{"dir_add_label", 1, "true"},
				{"max_active_downloads", 0, 3},
			},
			[]string{"/downloads/active"},
		},
		{"none", nil, nil},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.config = tt.config
		info, err := newClient(t, f, map[string]any{"host": "127.0.0.1"}).Info(context.Background())
		if err != nil {
			t.Fatalf("%s: Info() error = %v", tt.name, err)
		}
		if !info.IsLocalhost || strings.Join(info.OutputRootFolders, "|") != strings.Join(tt.want, "|") {
			t.Errorf("%s: Info() = %+v, want localhost with %q", tt.name, info, tt.want)
		}
	}
}

func TestTest(t *testing.T) {
	tests := []struct {
		build int
		pass  string
		ok    bool
	}{
		{25406, "pass", true},
		{25405, "pass", false},
		{30000, "wrong", false},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.build = tt.build
		err := newClient(t, f, map[string]any{"password": tt.pass}).Test(context.Background())
		if (err == nil) != tt.ok {
			t.Errorf("build %d, password %q: Test() error = %v, want ok = %v", tt.build, tt.pass, err, tt.ok)
		}
	}
}

func TestAuth(t *testing.T) {
	f := newFake(t)
	c := newClient(t, f, nil)
	items(t, c)
	items(t, c)
	if f.tokens != 1 {
		t.Errorf("tokens = %d, want = 1", f.tokens)
	}
	f.expireToken = true
	items(t, c)
	if f.tokens != 2 {
		t.Errorf("tokens after the token expired = %d, want = 2", f.tokens)
	}
	var auth *client.AuthError
	if _, err := newClient(t, f, map[string]any{"password": "wrong"}).Items(context.Background(), client.Options{}); !errors.As(err, &auth) {
		t.Errorf("Items() with a wrong password error = %v, want an AuthError", err)
	}
}

func TestRemoveAndMarkImported(t *testing.T) {
	f := newFake(t)
	f.torrents = []tor{completed()}
	c := newClient(t, f, map[string]any{"movie_imported_category": "movie-imported"})
	item := single(t, c)
	if !item.HasPostImportCategory {
		t.Errorf("HasPostImportCategory = false, want true")
	}
	if err := c.(client.Marker).MarkImported(context.Background(), item); err != nil {
		t.Fatalf("MarkImported() error = %v", err)
	}
	if err := c.Remove(context.Background(), item, true); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if err := c.Remove(context.Background(), item, false); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	want := []string{"setprops label=movie-imported", "setprops label=movie label=", "removedata", "remove"}
	if got := f.actions(); strings.Join(got, "; ") != strings.Join(want, "; ") {
		t.Errorf("actions = %q, want = %q", got, want)
	}
	if f.calls[0].Get("hash") != "hash" || f.calls[2].Get("hash") != "HASH" {
		t.Errorf("hashes = %q, %q, want the lower-case hash to label and the ID to remove", f.calls[0].Get("hash"), f.calls[2].Get("hash"))
	}

	f.calls = nil
	c = newClient(t, f, nil)
	if err := c.(client.Marker).MarkImported(context.Background(), item); err != nil || len(f.calls) != 0 {
		t.Errorf("MarkImported() without a post-import category = %v, %d calls, want nothing done", err, len(f.calls))
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"defaults", `{}`, true},
		{"no movie category", `{"movie_category":""}`, false},
		{"no series category", `{"tv_category":" "}`, false},
		{"bad state", `{"initial_state":4}`, false},
		{"unknown", `{"bogus":1}`, false},
		{"no host", `{"host":""}`, false},
	}
	for _, tt := range tests {
		_, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
		if (err == nil) != tt.ok {
			t.Errorf("%s: New() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

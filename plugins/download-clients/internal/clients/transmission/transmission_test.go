package transmission

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const title = "Droned.S01E01.Pilot.1080p.WEB-DL-DRONE"

// fake is a Transmission RPC server.
type fake struct {
	t        *testing.T
	mu       sync.Mutex
	torrents []map[string]any
	config   map[string]any
	calls    []call
	// rejectSession makes requests with a session ID fail with 409 once.
	rejectSession bool
}

type call struct {
	Method    string
	Arguments map[string]any
}

func newFake(t *testing.T) *fake {
	return &fake{t: t, config: map[string]any{
		"download-dir": "C:/Downloads/Finished/transmission", "incomplete-dir": nil, "incomplete-dir-enabled": false,
		"version": "4.0.6", "rpc-version": 17,
	}}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/transmission/rpc" {
		http.NotFound(w, r)
		return
	}
	if user, pass, _ := r.BasicAuth(); user != "admin" || pass != "pass" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Header.Get("X-Transmission-Session-Id") != "session" || f.rejectSession {
		f.rejectSession = false
		w.Header().Set("X-Transmission-Session-Id", "session")
		w.WriteHeader(http.StatusConflict)
		return
	}
	var c call
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &c); err != nil {
		f.t.Errorf("request body: %v", err)
	}
	f.calls = append(f.calls, c)
	var args any
	switch c.Method {
	case "torrent-get":
		args = map[string]any{"torrents": f.torrents}
	case "session-get":
		args = f.config
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"result": "success", "arguments": args})
}

func (f *fake) methods() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if c.Method != "torrent-get" && c.Method != "session-get" {
			out = append(out, c)
		}
	}
	return out
}

// newClient returns a client of a definition talking to f, with settings
// over the fixture's.
func newClient(t *testing.T, def string, f *fake, settings map[string]any) client.Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{
		"host": host, "port": p, "username": "admin", "password": "pass", "movie_category": "", "tv_category": "",
	}
	for k, v := range settings {
		s[k] = v
	}
	raw, _ := json.Marshal(s)
	for _, d := range Definitions() {
		if d.ID == def {
			c, err := d.New(raw, client.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}
			return c
		}
	}
	t.Fatalf("no definition %q", def)
	return nil
}

// The torrents of Radarr's TransmissionFixtureBase.
func queued() map[string]any {
	return map[string]any{
		"hashString": "HASH", "isFinished": false, "status": statusQueued, "name": title,
		"totalSize": 1000, "leftUntilDone": 1000, "downloadDir": "somepath", "eta": -1,
	}
}

func downloading() map[string]any {
	return map[string]any{
		"hashString": "HASH", "isFinished": false, "status": statusDownloading, "name": title,
		"totalSize": 1000, "leftUntilDone": 100, "downloadDir": "somepath", "eta": -1,
	}
}

func failed() map[string]any {
	return map[string]any{
		"hashString": "HASH", "isFinished": false, "status": statusStopped, "name": title,
		"totalSize": 1000, "leftUntilDone": 100, "errorString": "Error", "downloadDir": "somepath", "eta": -1,
	}
}

func magnet() map[string]any {
	return map[string]any{
		"hashString": "HASH", "isFinished": false, "status": statusDownloading, "name": title,
		"totalSize": 0, "leftUntilDone": 100, "downloadDir": "somepath", "eta": -1,
	}
}

// completed is Radarr's PrepareClientToReturnCompletedItem; zero limits
// are unset.
func completed(stopped bool, ratio float64, seedingMinutes int, ratioLimit float64, idleLimit int) map[string]any {
	m := map[string]any{
		"hashString": "HASH", "isFinished": true, "status": statusSeeding, "name": title,
		"totalSize": 1000, "leftUntilDone": 0, "downloadDir": "somepath", "downloadedEver": 1000,
		"uploadedEver": int(1000 * ratio), "secondsSeeding": seedingMinutes * 60, "eta": -1,
	}
	if stopped {
		m["status"] = statusStopped
	}
	if ratioLimit > 0 {
		m["seedRatioMode"], m["seedRatioLimit"] = 1, ratioLimit
	}
	if idleLimit > 0 {
		m["seedIdleMode"], m["seedIdleLimit"] = 1, idleLimit
	}
	return m
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
		t.Fatalf("Items() = %d items, want 1", len(got))
	}
	return got[0]
}

func TestItemStatus(t *testing.T) {
	tests := []struct {
		name    string
		torrent map[string]any
		status  int
		want    client.Status
	}{
		{"queued", queued(), statusQueued, client.Queued},
		{"downloading", downloading(), statusDownloading, client.Downloading},
		{"failed", failed(), statusStopped, client.Warning},
		{"completed", completed(false, 0.9, 60, 0, 0), statusSeeding, client.Completed},
		{"magnet", magnet(), statusDownloading, client.Queued},
		// GetItems_should_return_queued_item_as_downloadItemStatus
		{"queued stopped", queued(), statusStopped, client.Downloading},
		{"queued check wait", queued(), statusCheckWait, client.Downloading},
		{"queued check", queued(), statusCheck, client.Downloading},
		{"queued queued", queued(), statusQueued, client.Queued},
		{"queued downloading", queued(), statusDownloading, client.Downloading},
		{"queued seeding wait", queued(), statusSeedingWait, client.Downloading},
		{"queued seeding", queued(), statusSeeding, client.Downloading},
		// GetItems_should_return_downloading_item_as_downloadItemStatus
		{"downloading queued", downloading(), statusQueued, client.Queued},
		{"downloading downloading", downloading(), statusDownloading, client.Downloading},
		{"downloading seeding", downloading(), statusSeeding, client.Downloading},
		// GetItems_should_return_completed_item_as_downloadItemStatus
		{"completed stopped", completed(false, 0.9, 60, 0, 0), statusStopped, client.Completed},
		{"completed check wait", completed(false, 0.9, 60, 0, 0), statusCheckWait, client.Downloading},
		{"completed check", completed(false, 0.9, 60, 0, 0), statusCheck, client.Downloading},
		{"completed queued", completed(false, 0.9, 60, 0, 0), statusQueued, client.Completed},
		{"completed seeding wait", completed(false, 0.9, 60, 0, 0), statusSeedingWait, client.Completed},
		{"completed seeding", completed(false, 0.9, 60, 0, 0), statusSeeding, client.Completed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			tt.torrent["status"] = tt.status
			f.torrents = []map[string]any{tt.torrent}
			item := single(t, newClient(t, "transmission", f, nil))
			if item.Status != tt.want {
				t.Errorf("Status = %v, want = %v", item.Status, tt.want)
			}
			if item.ID != "HASH" || item.Title != title || item.TotalSize != int64(tt.torrent["totalSize"].(int)) {
				t.Errorf("item = %+v, want the torrent's hash, name and size", item)
			}
			if tt.want == client.Warning && item.Message == "" {
				t.Errorf("Message is empty, want the torrent's error")
			}
			if tt.name == "completed" && (item.CanBeRemoved || item.CanMoveFiles) {
				t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want false, false", item.CanBeRemoved, item.CanMoveFiles)
			}
		})
	}
}

func TestSeedLimits(t *testing.T) {
	tests := []struct {
		name               string
		ratioLimit         *float64
		idleLimit          *int
		torrent            map[string]any
		removable, movable bool
	}{
		{"max ratio reached and not stopped", ptr(1.0), nil, completed(false, 1.0, 60, 0, 0), false, false},
		{"max ratio not set", nil, nil, completed(true, 1.0, 60, 0, 0), false, false},
		{"max ratio reached and paused", ptr(1.0), nil, completed(true, 1.0, 60, 0, 0), true, true},
		{"overridden max ratio reached and paused", ptr(2.0), nil, completed(true, 1.0, 60, 0.8, 0), true, true},
		{"overridden max ratio not reached and paused", ptr(0.2), nil, completed(true, 0.5, 60, 0.8, 0), false, false},
		{"max idle time reached and not paused", nil, ptr(20), completed(false, 2.0, 30, 0, 0), false, false},
		{"max idle time reached and paused", nil, ptr(20), completed(true, 2.0, 20, 0, 0), true, true},
		{"overridden max idle time reached and paused", nil, ptr(40), completed(true, 2.0, 20, 0, 10), true, true},
		{"overridden max idle time reached and not paused", nil, ptr(40), completed(false, 2.0, 20, 0, 10), true, false},
		{"overridden max idle time not reached and paused", nil, ptr(20), completed(true, 2.0, 30, 0, 40), false, false},
		{"max idle time reached but ratio not and not paused", ptr(2.0), ptr(20), completed(false, 1.0, 30, 0, 0), false, false},
		{"max idle time configured and paused", ptr(2.0), ptr(20), completed(true, 1.0, 30, 0, 0), true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.config["seedRatioLimited"] = tt.ratioLimit != nil
			if tt.ratioLimit != nil {
				f.config["seedRatioLimit"] = *tt.ratioLimit
			}
			f.config["idle-seeding-limit-enabled"] = tt.idleLimit != nil
			if tt.idleLimit != nil {
				f.config["idle-seeding-limit"] = *tt.idleLimit
			}
			f.torrents = []map[string]any{tt.torrent}
			item := single(t, newClient(t, "transmission", f, nil))
			if item.CanBeRemoved != tt.removable || item.CanMoveFiles != tt.movable {
				t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = %v, %v", item.CanBeRemoved, item.CanMoveFiles, tt.removable, tt.movable)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestETA(t *testing.T) {
	tests := []struct {
		eta  int64
		want *time.Duration
	}{
		{-1, nil}, // infinite or unknown
		{-2, nil}, // magnet downloading
		{2147483648, ptr(2147483648 * time.Second)},
		{2147483648000, ptr(2147483648000 * time.Millisecond)},
	}
	for _, tt := range tests {
		f := newFake(t)
		tor := downloading()
		tor["eta"] = tt.eta
		f.torrents = []map[string]any{tor}
		item := single(t, newClient(t, "transmission", f, nil))
		switch {
		case tt.want == nil && item.RemainingTime != nil:
			t.Errorf("eta %d: RemainingTime = %v, want = nil", tt.eta, *item.RemainingTime)
		case tt.want != nil && (item.RemainingTime == nil || *item.RemainingTime != *tt.want):
			t.Errorf("eta %d: RemainingTime = %v, want = %v", tt.eta, item.RemainingTime, *tt.want)
		}
	}
}

func TestItemsFilter(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		dir      string
		labels   []string
		version  string
		want     int
		kind     client.Kind
	}{
		// should_exclude_items_not_in_category
		{"category folder", map[string]any{"movie_category": "radarr", "tv_category": "tv-sonarr"}, "C:/Downloads/Finished/transmission/radarr", nil, "", 1, client.Movie},
		{"series category folder", map[string]any{"movie_category": "radarr", "tv_category": "sonarr"}, "C:/Downloads/Finished/transmission/sonarr", nil, "", 1, client.Series},
		// should_exclude_items_not_in_TvDirectory
		{"directory", map[string]any{"movie_directory": "C:/Downloads/Finished/radarr", "tv_directory": "C:/Downloads/Finished/sonarr"}, "C:/Downloads/Finished/radarr/subdir", nil, "", 1, client.Movie},
		{"labels", map[string]any{"movie_category": "radarr", "tv_category": "tv-sonarr"}, "somepath", []string{"TV-Sonarr"}, "", 1, client.Series},
		{"labels before 4.0", map[string]any{"movie_category": "radarr", "tv_category": "tv-sonarr"}, "somepath", []string{"tv-sonarr"}, "3.00", 0, 0},
		{"no categories", nil, "somepath", nil, "", 2, 0},
		// A kind without a category or directory takes every other torrent,
		// as Radarr or Sonarr would by itself.
		{"one category", map[string]any{"movie_category": "radarr"}, "C:/Downloads/radarr", nil, "", 2, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			if tt.version != "" {
				f.config["version"] = tt.version
			}
			tor := downloading()
			tor["downloadDir"] = tt.dir
			tor["labels"] = tt.labels
			f.torrents = []map[string]any{tor, queued()}
			got := items(t, newClient(t, "transmission", f, tt.settings))
			if len(got) != tt.want {
				t.Fatalf("Items() = %d items, want = %d", len(got), tt.want)
			}
			if tt.want > 0 && (got[0].Status != client.Downloading || got[0].Kind != tt.kind) {
				t.Errorf("Items()[0] status, kind = %v, %v, want = %v, %v", got[0].Status, got[0].Kind, client.Downloading, tt.kind)
			}
		})
	}
}

func TestOutputPath(t *testing.T) {
	tests := []struct {
		def, dir, name string
		files          int
		want           string
	}{
		{"transmission", "C:/Downloads/Finished/transmission", title, 1, "C:/Downloads/Finished/transmission/" + title},
		{"transmission", "/downloads", "Movie: Part 2", 1, "/downloads/Movie_ Part 2"},
		// Vuze: a multi-file torrent's folder is its download dir.
		{"vuze", "/downloads/" + title, title, 2, "/downloads/" + title},
		{"vuze", "/downloads/job", title, 2, "/downloads/job"},
		{"vuze", "/downloads", title, 1, "/downloads/" + title},
	}
	for _, tt := range tests {
		f := newFake(t)
		tor := downloading()
		tor["downloadDir"], tor["name"] = tt.dir, tt.name
		tor["file-count"] = tt.files
		f.torrents = []map[string]any{tor}
		if got := single(t, newClient(t, tt.def, f, nil)).OutputPath; got != tt.want {
			t.Errorf("%s %s: OutputPath = %q, want = %q", tt.def, tt.dir, got, tt.want)
		}
	}
}

// torrentFile is a minimal torrent file.
func torrentFile() []byte {
	return []byte("d4:infod6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
}

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
		config   string
		wantDir  any
		wantID   string
		wantRest []string
	}{
		{
			"movie directory",
			map[string]any{"movie_directory": "C:/Downloads/Finished/radarr"},
			client.Release{Kind: client.Movie, Data: data},
			"", "C:/Downloads/Finished/radarr", hash, nil,
		},
		{
			"series directory",
			map[string]any{"tv_directory": "C:/Downloads/Finished/sonarr"},
			client.Release{Kind: client.Series, Data: data},
			"", "C:/Downloads/Finished/sonarr", hash, nil,
		},
		{
			"category",
			map[string]any{"movie_category": "radarr"},
			client.Release{Kind: client.Movie, Data: data},
			"", "C:/Downloads/Finished/transmission/radarr", hash, nil,
		},
		{
			"category without double slashes",
			map[string]any{"tv_category": "sonarr"},
			client.Release{Kind: client.Series, Data: data},
			"C:/Downloads/Finished/transmission/", "C:/Downloads/Finished/transmission/sonarr", hash, nil,
		},
		{"default", nil, client.Release{Kind: client.Movie, Data: data}, "", nil, hash, nil},
		{
			"magnet", nil,
			client.Release{Kind: client.Movie, MagnetURL: "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp"},
			"", nil, "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951", nil,
		},
		{
			"seeding and first",
			map[string]any{"recent_movie_priority": priorityFirst},
			client.Release{Kind: client.Movie, Recent: true, Data: data, SeedRatio: ptr(1.5), SeedTime: ptr(90 * time.Minute)},
			"", nil, hash,
			[]string{"torrent-set", "queue-move-top"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			if tt.config != "" {
				f.config["download-dir"] = tt.config
			}
			r := tt.release
			r.Title = title
			id, err := newClient(t, "transmission", f, tt.settings).Add(context.Background(), &r)
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if id != tt.wantID {
				t.Errorf("Add() = %q, want = %q", id, tt.wantID)
			}
			calls := f.methods()
			if len(calls) != 1+len(tt.wantRest) || calls[0].Method != "torrent-add" {
				t.Fatalf("calls = %+v, want torrent-add then %v", calls, tt.wantRest)
			}
			if got := calls[0].Arguments["download-dir"]; got != tt.wantDir {
				t.Errorf("download-dir = %v, want = %v", got, tt.wantDir)
			}
			if r.Data != nil && calls[0].Arguments["metainfo"] != base64.StdEncoding.EncodeToString(data) {
				t.Errorf("metainfo = %v, want the torrent file", calls[0].Arguments["metainfo"])
			}
			if r.MagnetURL != "" && calls[0].Arguments["filename"] != r.MagnetURL {
				t.Errorf("filename = %v, want = %v", calls[0].Arguments["filename"], r.MagnetURL)
			}
			for i, m := range tt.wantRest {
				if calls[i+1].Method != m {
					t.Errorf("calls[%d] = %s, want = %s", i+1, calls[i+1].Method, m)
				}
			}
			if len(tt.wantRest) > 0 {
				args := calls[1].Arguments
				if args["seedRatioLimit"] != 1.5 || args["seedIdleLimit"] != 90.0 || args["seedRatioMode"] != 1.0 {
					t.Errorf("torrent-set = %v, want the ratio 1.5 and 90 idle minutes", args)
				}
			}
		})
	}
}

func TestInfo(t *testing.T) {
	f := newFake(t)
	c := newClient(t, "transmission", f, map[string]any{"host": "127.0.0.1", "tv_category": "sonarr"})
	info, err := c.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"C:/Downloads/Finished/transmission", "C:/Downloads/Finished/transmission/sonarr"}
	if !info.IsLocalhost || len(info.OutputRootFolders) != 2 || info.OutputRootFolders[0] != want[0] || info.OutputRootFolders[1] != want[1] {
		t.Errorf("Info() = %+v, want localhost with %v", info, want)
	}
}

func TestTestVersion(t *testing.T) {
	tests := []struct {
		def, version string
		rpc          int
		ok           bool
	}{
		{"transmission", "2.84 ()", 0, true},
		{"transmission", "2.84+ ()", 0, true},
		{"transmission", "2.84 (other info)", 0, true},
		{"transmission", "2.84 (2.84)", 0, true},
		{"transmission", "2.30", 0, false},
		{"vuze", "", 14, true},
		{"vuze", "", 13, false},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.config["version"], f.config["rpc-version"] = tt.version, tt.rpc
		err := newClient(t, tt.def, f, nil).Test(context.Background())
		if (err == nil) != tt.ok {
			t.Errorf("%s %q %d: Test() error = %v, want ok = %v", tt.def, tt.version, tt.rpc, err, tt.ok)
		}
	}
}

func TestSessionAndAuth(t *testing.T) {
	f := newFake(t)
	c := newClient(t, "transmission", f, nil)
	if _, err := c.Items(context.Background(), client.Options{}); err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	f.rejectSession = true
	if _, err := c.Items(context.Background(), client.Options{}); err != nil {
		t.Fatalf("Items() after the session expired error = %v", err)
	}
	bad := newClient(t, "transmission", f, map[string]any{"password": "wrong"})
	var auth *client.AuthError
	if _, err := bad.Items(context.Background(), client.Options{}); !errors.As(err, &auth) {
		t.Errorf("Items() with a wrong password error = %v, want an AuthError", err)
	}
}

func TestRemoveAndMarkImported(t *testing.T) {
	f := newFake(t)
	tor := completed(true, 1, 60, 0, 0)
	tor["labels"] = []string{"radarr", "keep"}
	f.torrents = []map[string]any{tor}
	c := newClient(t, "transmission", f, map[string]any{"movie_category": "radarr", "movie_imported_category": "radarr-imported"})
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
	calls := f.methods()
	if len(calls) != 2 {
		t.Fatalf("calls = %+v, want torrent-set and torrent-remove", calls)
	}
	if got, _ := json.Marshal(calls[0].Arguments["labels"]); string(got) != `["keep","radarr-imported"]` {
		t.Errorf("labels = %s, want = [keep radarr-imported]", got)
	}
	if got, _ := json.Marshal(calls[1].Arguments); string(got) != `{"delete-local-data":true,"ids":["hash"]}` {
		t.Errorf("torrent-remove = %s, want the lower-case hash with its data", got)
	}

	f.config["version"] = "3.00"
	c = newClient(t, "transmission", f, map[string]any{"movie_category": "radarr", "movie_imported_category": "radarr-imported"})
	if err := c.(client.Marker).MarkImported(context.Background(), item); !errors.Is(err, client.ErrNotSupported) {
		t.Errorf("MarkImported() before 4.0 error = %v, want ErrNotSupported", err)
	}
	if _, ok := newClient(t, "vuze", f, nil).(client.Marker); ok {
		t.Errorf("Vuze is a Marker, want not")
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"defaults", `{}`, true},
		{"bad category", `{"movie_category":"radarr_1"}`, false},
		{"category and directory", `{"tv_category":"tv","tv_directory":"/tv"}`, false},
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

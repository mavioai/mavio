package rtorrent

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const title = "Droned.1998.1080p.WEB-DL-DRONE"

// fake is an rTorrent XML-RPC server.
type fake struct {
	t        *testing.T
	mu       sync.Mutex
	torrents []torrent
	version  string
	calls    []call
	// names are what d.name returns by hash; the torrent's name otherwise.
	names map[string]string
	// fault makes every call fail with an XML-RPC fault.
	fault bool
	// labelResult is what d.custom1.set returns; the label when empty.
	labelResult string
}

type call struct {
	Method string
	Params []string
}

func newFake(t *testing.T) *fake {
	return &fake{t: t, version: "0.9.8", names: map[string]string{}}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/RPC2" {
		http.NotFound(w, r)
		return
	}
	if user, pass, ok := r.BasicAuth(); ok && (user != "admin" || pass != "pass") {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if ct := r.Header.Get("Content-Type"); ct != "text/xml" {
		f.t.Errorf("Content-Type = %q, want = text/xml", ct)
	}
	body, _ := io.ReadAll(r.Body)
	var mc struct {
		Method string  `xml:"methodName"`
		Params []value `xml:"params>param>value"`
	}
	if err := xml.Unmarshal(body, &mc); err != nil {
		f.t.Errorf("request body: %v", err)
	}
	c := call{Method: mc.Method}
	for i := range mc.Params {
		c.Params = append(c.Params, mc.Params[i].str())
	}
	f.calls = append(f.calls, c)
	if f.fault {
		_, _ = io.WriteString(w, `<?xml version="1.0"?><methodResponse><fault><value><struct>`+
			`<member><name>faultCode</name><value><i4>-506</i4></value></member>`+
			`<member><name>faultString</name><value><string>Method 'x' not defined</string></value></member>`+
			`</struct></value></fault></methodResponse>`)
		return
	}
	var v string
	switch c.Method {
	case "system.client_version":
		v = str(f.version)
	case "d.multicall2":
		var rows strings.Builder
		for _, t := range f.torrents {
			fmt.Fprintf(&rows, "<value><array><data>%s%s%s%s%s%s%s%s%s%s%s%s</data></array></value>",
				str(t.Name), str(t.Hash), str(t.Path), str(url.QueryEscape(t.Category)), i8(t.TotalSize),
				i8(t.RemainingSize), i8(t.DownRate), i8(t.Ratio), i8(b2i(t.IsOpen)), i8(b2i(t.IsActive)),
				i8(b2i(t.IsFinished)), i8(t.FinishedTime))
		}
		v = "<value><array><data>" + rows.String() + "</data></array></value>"
	case "d.name":
		name, ok := f.names[c.Params[0]]
		if !ok {
			for _, t := range f.torrents {
				if strings.EqualFold(t.Hash, c.Params[0]) {
					name = t.Name
				}
			}
		}
		v = str(name)
	case "d.custom1.set":
		v = str(c.Params[1])
		if f.labelResult != "" {
			v = str(f.labelResult)
		}
	default:
		v = "<value><i8>0</i8></value>"
	}
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><methodResponse><params><param>`+v+`</param></params></methodResponse>`)
}

func str(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return "<value><string>" + b.String() + "</string></value>"
}

func i8(n int64) string { return "<value><i8>" + strconv.FormatInt(n, 10) + "</i8></value>" }

func b2i(b bool) int64 {
	if b {
		return 1
	}
	return 0
}

func (f *fake) methods() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		switch c.Method {
		case "d.multicall2", "d.name", "system.client_version":
		default:
			out = append(out, c)
		}
	}
	return out
}

// newClient returns rTorrent talking to f, with settings over the
// fixture's, which has no categories.
func newClient(t *testing.T, f *fake, settings map[string]any) *rtorrent {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{"host": host, "port": p, "movie_category": "", "tv_category": ""}
	for k, v := range settings {
		s[k] = v
	}
	raw, _ := json.Marshal(s)
	c, err := Definitions()[0].New(raw, client.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	r := c.(*rtorrent)
	r.retryDelay = time.Millisecond
	return r
}

// The torrents of Radarr's RTorrentFixture.
func downloading() torrent {
	return torrent{
		Hash: "HASH", IsFinished: false, IsOpen: true, IsActive: true, Name: title,
		TotalSize: 1000, RemainingSize: 500, Path: "somepath",
	}
}

func completed() torrent {
	return torrent{Hash: "HASH", IsFinished: true, Name: title, TotalSize: 1000, RemainingSize: 0, Path: "somepath"}
}

func items(t *testing.T, c client.Client, opts client.Options) []client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), opts)
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	return items
}

func single(t *testing.T, c client.Client) client.Item {
	t.Helper()
	got := items(t, c, client.Options{RemoveCompletedDownloads: true})
	if len(got) != 1 {
		t.Fatalf("Items() = %d items, want 1", len(got))
	}
	return got[0]
}

func TestItemStatus(t *testing.T) {
	tests := []struct {
		name    string
		torrent torrent
		want    client.Status
	}{
		// downloading_item_should_have_required_properties
		{"downloading", downloading(), client.Downloading},
		// completed_download_should_have_required_properties
		{"completed", completed(), client.Completed},
		{"paused", torrent{Hash: "HASH", Name: title, TotalSize: 1000, RemainingSize: 500, Path: "/d"}, client.Paused},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.torrents = []torrent{tt.torrent}
			item := single(t, newClient(t, f, nil))
			if item.Status != tt.want {
				t.Errorf("Status = %v, want = %v", item.Status, tt.want)
			}
			if item.ID != "HASH" || item.Title != title || item.TotalSize != 1000 || item.OutputPath != tt.torrent.Path {
				t.Errorf("item = %+v, want the torrent's hash, name, size and path", item)
			}
			if (item.RemainingSize == 0) != (tt.want == client.Completed) {
				t.Errorf("RemainingSize = %d, want zero only when completed", item.RemainingSize)
			}
			if item.RemainingTime == nil || *item.RemainingTime != 0 {
				t.Errorf("RemainingTime = %v, want = 0", item.RemainingTime)
			}
			if item.CanBeRemoved || item.CanMoveFiles {
				t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want false without a seed configuration", item.CanBeRemoved, item.CanMoveFiles)
			}
		})
	}
}

func TestItemValues(t *testing.T) {
	f := newFake(t)
	tor := downloading()
	tor.DownRate, tor.Ratio, tor.Category = 100, 1500, "movies mavio"
	f.torrents = []torrent{tor}
	item := single(t, newClient(t, f, map[string]any{"movie_category": "movies mavio", "tv_category": "tv"}))
	if item.RemainingTime == nil || *item.RemainingTime != 5*time.Second {
		t.Errorf("RemainingTime = %v, want = 5s", item.RemainingTime)
	}
	if item.SeedRatio == nil || *item.SeedRatio != 1.5 {
		t.Errorf("SeedRatio = %v, want = 1.5", item.SeedRatio)
	}
	if item.Category != "movies mavio" || item.Kind != client.Movie {
		t.Errorf("Category, Kind = %q, %v, want = %q, %v", item.Category, item.Kind, "movies mavio", client.Movie)
	}
}

func TestItemsFilter(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		category string
		path     string
		want     int
		kind     client.Kind
	}{
		{"movie category", map[string]any{"movie_category": "radarr", "tv_category": "tv-sonarr"}, "radarr", "/d", 1, client.Movie},
		{"series category", map[string]any{"movie_category": "radarr", "tv_category": "tv-sonarr"}, "tv-sonarr", "/d", 1, client.Series},
		{"other category", map[string]any{"movie_category": "radarr", "tv_category": "tv-sonarr"}, "other", "/d", 0, 0},
		{"same categories", map[string]any{"movie_category": "media", "tv_category": "media"}, "media", "/d", 1, 0},
		{"no categories", nil, "other", "/d", 1, 0},
		// A kind without a category takes every torrent, as Radarr or
		// Sonarr would by itself.
		{"one category", map[string]any{"movie_category": "radarr"}, "other", "/d", 1, client.Series},
		{"empty path", nil, "", "", 0, 0},
		{"relative path", nil, "", "./relative/path", 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			tor := downloading()
			tor.Category, tor.Path = tt.category, tt.path
			f.torrents = []torrent{tor}
			got := items(t, newClient(t, f, tt.settings), client.Options{})
			if len(got) != tt.want {
				t.Fatalf("Items() = %d items, want = %d", len(got), tt.want)
			}
			if tt.want > 0 && got[0].Kind != tt.kind {
				t.Errorf("Kind = %v, want = %v", got[0].Kind, tt.kind)
			}
		})
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
	const magnet = "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp"
	const magnetHash = "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951"
	tests := []struct {
		name     string
		settings map[string]any
		release  client.Release
		method   string
		wantID   string
		wantArgs []string
	}{
		// Download_should_return_unique_id
		{"torrent file", nil, client.Release{Kind: client.Movie, Data: data}, "load.raw_start", hash, nil},
		{
			"magnet", nil,
			client.Release{Kind: client.Movie, MagnetURL: magnet},
			"load.start", magnetHash, nil,
		},
		{
			"stopped with label, priority and directory",
			map[string]any{"add_stopped": true, "tv_category": "tv-mavio", "tv_directory": "/tv", "recent_tv_priority": priorityHigh},
			client.Release{Kind: client.Series, Recent: true, Data: data},
			"load.raw", hash,
			[]string{"d.custom1.set=tv-mavio", "d.priority.set=3", "d.directory.set=/tv"},
		},
		{
			"magnet stopped, older movie priority",
			map[string]any{"add_stopped": true, "movie_category": "movies", "older_movie_priority": priorityLow},
			client.Release{Kind: client.Movie, MagnetURL: magnet},
			"load.normal", magnetHash,
			[]string{"d.custom1.set=movies", "d.priority.set=1"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.names[hash], f.names[magnetHash] = title, title
			r := tt.release
			r.Title = title
			id, err := newClient(t, f, tt.settings).Add(context.Background(), &r)
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if id != tt.wantID {
				t.Errorf("Add() = %q, want = %q", id, tt.wantID)
			}
			calls := f.methods()
			if len(calls) != 1 || calls[0].Method != tt.method {
				t.Fatalf("calls = %+v, want %s", calls, tt.method)
			}
			source := r.MagnetURL
			if r.Data != nil {
				source = base64.StdEncoding.EncodeToString(data)
			}
			want := append([]string{"", source}, tt.wantArgs...)
			if !slices.Equal(calls[0].Params, want) {
				t.Errorf("params = %q, want = %q", calls[0].Params, want)
			}
		})
	}
}

func TestAddWaitsForTorrent(t *testing.T) {
	data := torrentFile()
	const magnet = "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp"
	const magnetHash = "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951"

	// A torrent file rTorrent does not add fails.
	f := newFake(t)
	r := client.Release{Title: title, Data: data}
	if _, err := newClient(t, f, nil).Add(context.Background(), &r); err == nil {
		t.Errorf("Add() of a torrent never added error = nil, want an error")
	}
	if n := countCalls(f, "d.name"); n != 10 {
		t.Errorf("d.name calls = %d, want = 10", n)
	}

	// A magnet still resolving is kept track of.
	f = newFake(t)
	f.names[magnetHash] = magnetHash + ".meta"
	r = client.Release{Title: title, MagnetURL: magnet}
	if id, err := newClient(t, f, nil).Add(context.Background(), &r); err != nil || id != magnetHash {
		t.Errorf("Add() of an unresolved magnet = %q, %v, want = %q, nil", id, err, magnetHash)
	}
}

func countCalls(f *fake, method string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		if c.Method == method {
			n++
		}
	}
	return n
}

func TestSeedLimits(t *testing.T) {
	data := torrentFile()
	hash, _ := client.InfoHash(data)
	now := time.Now()
	tests := []struct {
		name      string
		ratio     *float64
		seedTime  *time.Duration
		torrent   torrent
		opts      client.Options
		removable bool
	}{
		{"ratio reached", ptr(1.0), nil, finished(hash, 1000, now), client.Options{RemoveCompletedDownloads: true}, true},
		{"ratio not reached", ptr(2.0), nil, finished(hash, 1000, now), client.Options{RemoveCompletedDownloads: true}, false},
		{"seed time reached", nil, ptr(time.Hour), finished(hash, 0, now.Add(-2*time.Hour)), client.Options{RemoveCompletedDownloads: true}, true},
		{"seed time not reached", nil, ptr(time.Hour), finished(hash, 0, now.Add(-time.Minute)), client.Options{RemoveCompletedDownloads: true}, false},
		{"not removing completed", ptr(1.0), nil, finished(hash, 1000, now), client.Options{}, false},
		{"no seed configuration", nil, nil, finished(hash, 1000, now), client.Options{RemoveCompletedDownloads: true}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.torrents = []torrent{tt.torrent}
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

func finished(hash string, ratio int64, at time.Time) torrent {
	t := completed()
	t.Hash, t.Ratio, t.FinishedTime = hash, ratio, at.Unix()
	return t
}

func ptr[T any](v T) *T { return &v }

func TestRemoveAndMarkImported(t *testing.T) {
	f := newFake(t)
	f.torrents = []torrent{completed()}
	c := newClient(t, f, map[string]any{"movie_imported_category": "radarr-imported"})
	item := single(t, c)
	if !item.HasPostImportCategory {
		t.Errorf("HasPostImportCategory = false, want true")
	}
	if err := c.MarkImported(context.Background(), item); err != nil {
		t.Fatalf("MarkImported() error = %v", err)
	}
	if err := c.Remove(context.Background(), item, false); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	want := []call{
		{"d.custom1.set", []string{"hash", "radarr-imported"}},
		{"d.views.push_back_unique", []string{"hash", importedView}},
		{"d.erase", []string{"HASH"}},
	}
	got := f.methods()
	if len(got) != len(want) {
		t.Fatalf("calls = %+v, want = %+v", got, want)
	}
	for i := range want {
		if got[i].Method != want[i].Method || !slices.Equal(got[i].Params, want[i].Params) {
			t.Errorf("calls[%d] = %+v, want = %+v", i, got[i], want[i])
		}
	}

	// A label rTorrent does not take is only logged; so are failing views.
	f = newFake(t)
	f.labelResult = "other"
	c = newClient(t, f, map[string]any{"movie_imported_category": "radarr-imported"})
	if err := c.MarkImported(context.Background(), item); err != nil {
		t.Errorf("MarkImported() with the label refused error = %v, want nil", err)
	}
	f.fault = true
	if err := c.MarkImported(context.Background(), item); err != nil {
		t.Errorf("MarkImported() with faults error = %v, want nil", err)
	}
	// Without a post-import category, only the view is set.
	f = newFake(t)
	c = newClient(t, f, nil)
	if err := c.MarkImported(context.Background(), item); err != nil {
		t.Fatal(err)
	}
	if got := f.methods(); len(got) != 1 || got[0].Method != "d.views.push_back_unique" {
		t.Errorf("calls = %+v, want d.views.push_back_unique", got)
	}
}

func TestTest(t *testing.T) {
	tests := []struct {
		name    string
		version string
		fault   bool
		ok      bool
	}{
		{"supported", "0.9.8", false, true},
		{"0.9.0", "0.9.0", false, true},
		{"old", "0.8.9", false, false},
		{"fault", "0.9.8", true, false},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.version, f.fault = tt.version, tt.fault
		err := newClient(t, f, nil).Test(context.Background())
		if (err == nil) != tt.ok {
			t.Errorf("%s: Test() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

func TestDirectoryValidator(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		name     string
		settings map[string]any
		ok       bool
	}{
		{"existing", map[string]any{"host": "localhost", "movie_directory": dir}, true},
		{"missing", map[string]any{"host": "localhost", "tv_directory": dir + "/missing"}, false},
		{"relative", map[string]any{"host": "localhost", "movie_directory": "relative/dir"}, false},
		{"remote", map[string]any{"host": "seedbox", "movie_directory": dir + "/missing"}, true},
	}
	for _, tt := range tests {
		f := newFake(t)
		c := newClient(t, f, nil)
		raw, _ := json.Marshal(tt.settings)
		if err := client.Decode(raw, &c.settings); err != nil {
			t.Fatal(err)
		}
		if err := c.testDirectories(); (err == nil) != tt.ok {
			t.Errorf("%s: testDirectories() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

func TestFaultAndAuth(t *testing.T) {
	f := newFake(t)
	f.fault = true
	_, err := newClient(t, f, nil).Items(context.Background(), client.Options{})
	var fe *faultError
	if !errors.As(err, &fe) || fe.Code != -506 || fe.Message != "Method 'x' not defined" {
		t.Errorf("Items() error = %v, want the fault -506", err)
	}
	f = newFake(t)
	var auth *client.AuthError
	if _, err := newClient(t, f, map[string]any{"username": "admin", "password": "wrong"}).Items(context.Background(), client.Options{}); !errors.As(err, &auth) {
		t.Errorf("Items() with a wrong password error = %v, want an AuthError", err)
	}
	if _, err := newClient(t, f, map[string]any{"username": "admin", "password": "pass"}).Items(context.Background(), client.Options{}); err != nil {
		t.Errorf("Items() with the password error = %v", err)
	}
}

func TestInfo(t *testing.T) {
	info, err := newClient(t, newFake(t), map[string]any{"host": "127.0.0.1"}).Info(context.Background())
	if err != nil || !info.IsLocalhost {
		t.Errorf("Info() = %+v, %v, want localhost", info, err)
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
		{"bad port", `{"port":70000}`, false},
	}
	for _, tt := range tests {
		c, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
		if (err == nil) != tt.ok {
			t.Errorf("%s: New() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
		if tt.name == "defaults" {
			s := c.(*rtorrent).settings
			if s.BaseURL() != "http://localhost:8080/RPC2" || s.MovieCategory != client.DefaultMovieCategory || s.RecentTVPriority != priorityNormal {
				t.Errorf("defaults = %+v, want Radarr's", s)
			}
		}
	}
}

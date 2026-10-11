package flood

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
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const title = "Droned.S01E01.Pilot.1080p.WEB-DL-DRONE"

// fake is Flood's API.
type fake struct {
	t        *testing.T
	mu       sync.Mutex
	torrents map[string]map[string]any
	contents map[string][]string
	calls    []call
	logins   int
	// expireSession makes the next request with the cookie fail once.
	expireSession bool
}

type call struct {
	Method, Path string
	Body         map[string]any
}

func newFake(t *testing.T) *fake {
	return &fake{t: t, torrents: map[string]map[string]any{}, contents: map[string][]string{}}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body map[string]any
	if data, _ := io.ReadAll(r.Body); len(data) > 0 {
		if err := json.Unmarshal(data, &body); err != nil {
			f.t.Errorf("%s %s body: %v", r.Method, r.URL.Path, err)
		}
	}
	path := strings.TrimPrefix(r.URL.Path, "/api")
	if path == "/auth/authenticate" {
		if body["username"] != "admin" || body["password"] != "pass" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		f.logins++
		http.SetCookie(w, &http.Cookie{Name: "jwt", Value: "session"})
		_, _ = io.WriteString(w, `{"success":true}`)
		return
	}
	if c, err := r.Cookie("jwt"); err != nil || c.Value != "session" || f.expireSession {
		f.expireSession = false
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if r.Method != http.MethodGet {
		f.calls = append(f.calls, call{r.Method, path, body})
	}
	var out any = map[string]any{}
	switch {
	case r.Method == http.MethodGet && path == "/torrents":
		out = map[string]any{"id": 1, "torrents": f.torrents}
	case r.Method == http.MethodGet && path == "/client/settings":
		out = map[string]any{"directoryDefault": "/downloads/flood"}
	case r.Method == http.MethodGet && strings.HasSuffix(path, "/contents"):
		var c []map[string]any
		for _, p := range f.contents[strings.TrimSuffix(strings.TrimPrefix(path, "/torrents/"), "/contents")] {
			c = append(c, map[string]any{"path": p})
		}
		out = c
	}
	_ = json.NewEncoder(w).Encode(out)
}

func newClient(t *testing.T, f *fake, settings map[string]any, now func() time.Time) client.Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{"host": host, "port": p, "username": "admin", "password": "pass"}
	for k, v := range settings {
		s[k] = v
	}
	raw, _ := json.Marshal(s)
	if now == nil {
		now = time.Now
	}
	c, err := Definitions()[0].New(raw, client.Env{HTTP: srv.Client(), Now: now, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c
}

func torrentOf(status string, tags ...string) map[string]any {
	return map[string]any{
		"bytesDone": 400, "directory": "/downloads/flood", "eta": 60, "message": "", "name": title,
		"ratio": 0.5, "sizeBytes": 1000, "status": []string{status}, "tags": tags,
	}
}

func items(t *testing.T, c client.Client) []client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), client.Options{RemoveCompletedDownloads: true})
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	return items
}

func TestItems(t *testing.T) {
	tests := []struct {
		name   string
		status []string
		want   client.Status
	}{
		{"seeding", []string{"seeding", "complete"}, client.Completed},
		{"complete", []string{"complete", "stopped"}, client.Completed},
		{"stopped", []string{"stopped", "inactive"}, client.Paused},
		{"error", []string{"error"}, client.Warning},
		{"downloading", []string{"downloading", "active"}, client.Downloading},
		{"checking", []string{"checking"}, client.Queued},
	}
	for _, tt := range tests {
		f := newFake(t)
		tor := torrentOf("")
		tor["status"] = tt.status
		tor["tags"] = []string{client.DefaultMovieCategory, "other"}
		f.torrents["ABC"] = tor
		got := items(t, newClient(t, f, nil, nil))
		if len(got) != 1 {
			t.Fatalf("%s: Items() = %d items, want = 1", tt.name, len(got))
		}
		item := got[0]
		if item.Status != tt.want {
			t.Errorf("%s: Status = %v, want = %v", tt.name, item.Status, tt.want)
		}
		if item.ID != "ABC" || item.Title != title || item.Category != client.DefaultMovieCategory || item.Kind != client.Movie ||
			item.TotalSize != 1000 || item.RemainingSize != 600 || item.OutputPath != "/downloads/flood" ||
			item.RemainingTime == nil || *item.RemainingTime != time.Minute || item.SeedRatio == nil || *item.SeedRatio != 0.5 {
			t.Errorf("%s: item = %+v, want the torrent's", tt.name, item)
		}
		if item.CanBeRemoved || item.CanMoveFiles {
			t.Errorf("%s: CanBeRemoved, CanMoveFiles = true, want false without seed criteria", tt.name)
		}
	}
}

func TestItemsFilter(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		tags     []string
		want     bool
		kind     client.Kind
	}{
		{"movie", nil, []string{client.DefaultMovieCategory}, true, client.Movie},
		{"series", nil, []string{"x", client.DefaultTVCategory}, true, client.Series},
		{"other", nil, []string{"other"}, false, 0},
		{"not all tags", map[string]any{"movie_tags": []string{"a", "b"}}, []string{"a"}, false, 0},
		{"all tags", map[string]any{"movie_tags": []string{"a", "b"}}, []string{"b", "a"}, true, client.Movie},
		{"same tags", map[string]any{"movie_tags": []string{"a"}, "tv_tags": []string{"a"}}, []string{"a"}, true, 0},
		// A kind without tags takes every download, as Radarr would.
		{"no movie tags", map[string]any{"movie_tags": []string{}}, []string{"other"}, true, client.Movie},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.torrents["ABC"] = torrentOf("downloading", tt.tags...)
		got := items(t, newClient(t, f, tt.settings, nil))
		if (len(got) == 1) != tt.want {
			t.Fatalf("%s: Items() = %d items, want = %v", tt.name, len(got), tt.want)
		}
		if tt.want && got[0].Kind != tt.kind {
			t.Errorf("%s: Kind = %v, want = %v", tt.name, got[0].Kind, tt.kind)
		}
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
	const magnet = "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp"
	tests := []struct {
		name     string
		settings map[string]any
		release  client.Release
		wantID   string
		want     string
	}{
		{
			"torrent file", nil,
			client.Release{Kind: client.Movie, Data: data},
			hash,
			`/torrents/add-files {"files":["` + base64.StdEncoding.EncodeToString(data) + `"],"start":true,"tags":["movies-mavio"]}`,
		},
		{
			"magnet",
			map[string]any{"start_on_add": false, "tv_directory": "/tv", "tv_tags": []string{"tv", "", "tv", "x"}},
			client.Release{Kind: client.Series, MagnetURL: magnet},
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951",
			`/torrents/add-urls {"destination":"/tv","tags":["tv","x"],"urls":["` + magnet + `"]}`,
		},
		{
			// Radarr's HandleTags: the additional tags of the movie and
			// release, without blanks and duplicates.
			"movie additional tags",
			map[string]any{"additional_tags": []string{"collection", "title_slug", "quality", "languages", "release_group", "year", "indexer", "studio"}},
			client.Release{Kind: client.Movie, MagnetURL: magnet, Details: client.Details{
				Quality: "Bluray-2160p", Languages: []string{"English", "French"}, ReleaseGroup: "DRONE",
				Indexer: "Torznab", Year: 2020, Collection: "Big Buck Bunny Series", Studio: "movies-mavio",
			}},
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951",
			`/torrents/add-urls {"start":true,"tags":["movies-mavio","Big Buck Bunny Series","Bluray-2160p","English","French","DRONE","2020","Torznab"],"urls":["` + magnet + `"]}`,
		},
		{
			// Sonarr's HandleTags: the title slug and network of the series.
			"series additional tags",
			map[string]any{"additional_tags": []string{"title_slug", "studio", "year"}},
			client.Release{Kind: client.Series, MagnetURL: magnet, Details: client.Details{TitleSlug: "the-series", Studio: "C-SPAN"}},
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951",
			`/torrents/add-urls {"start":true,"tags":["tv-mavio","the-series","C-SPAN"],"urls":["` + magnet + `"]}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			r := tt.release
			r.Title = title
			id, err := newClient(t, f, tt.settings, nil).Add(context.Background(), &r)
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if id != tt.wantID {
				t.Errorf("Add() = %q, want = %q", id, tt.wantID)
			}
			if len(f.calls) != 1 {
				t.Fatalf("calls = %+v, want one", f.calls)
			}
			var body strings.Builder
			enc := json.NewEncoder(&body)
			enc.SetEscapeHTML(false)
			_ = enc.Encode(f.calls[0].Body)
			if got := f.calls[0].Path + " " + strings.TrimSpace(body.String()); got != tt.want {
				t.Errorf("call = %s, want = %s", got, tt.want)
			}
		})
	}
}

func TestSeedLimits(t *testing.T) {
	now := time.Unix(1_700_000_000, 0)
	tests := []struct {
		name      string
		ratio     *float64
		seedTime  *time.Duration
		status    string
		torRatio  float64
		finished  *int64
		removable bool
	}{
		{"ratio reached", ptr(1.0), nil, "seeding", 1.0, nil, true},
		{"ratio not reached", ptr(2.0), nil, "seeding", 1.0, nil, false},
		{"ratio reached but downloading", ptr(1.0), nil, "downloading", 1.0, nil, false},
		{"seed time reached", ptr(2.0), ptr(time.Hour), "complete", 1.0, ptr(now.Add(-2 * time.Hour).Unix()), true},
		{"seed time not reached", nil, ptr(time.Hour), "complete", 1.0, ptr(now.Add(-30 * time.Minute).Unix()), false},
		{"seed time without finish date", nil, ptr(time.Hour), "complete", 1.0, nil, false},
		{"no seed criteria", nil, nil, "seeding", 5.0, nil, false},
	}
	for _, tt := range tests {
		f := newFake(t)
		c := newClient(t, f, nil, func() time.Time { return now })
		r := client.Release{Title: title, Kind: client.Movie, Data: torrentFile(), SeedRatio: tt.ratio, SeedTime: tt.seedTime}
		id, err := c.Add(context.Background(), &r)
		if err != nil {
			t.Fatalf("%s: Add() error = %v", tt.name, err)
		}
		tor := torrentOf(tt.status, client.DefaultMovieCategory)
		tor["ratio"] = tt.torRatio
		if tt.finished != nil {
			tor["dateFinished"] = *tt.finished
		}
		f.torrents[id] = tor
		got := items(t, c)
		if len(got) != 1 || got[0].CanBeRemoved != tt.removable || got[0].CanMoveFiles != tt.removable {
			t.Errorf("%s: Items() = %+v, want CanBeRemoved = CanMoveFiles = %v", tt.name, got, tt.removable)
		}
	}
}

func TestImportItem(t *testing.T) {
	tests := []struct {
		name     string
		contents []string
		want     string
		fails    bool
	}{
		{"single file", []string{title + ".mkv"}, "/downloads/flood/" + title + ".mkv", false},
		{"single file in a folder", []string{title + "/" + title + ".mkv"}, "/downloads/flood/" + title + "/" + title + ".mkv", false},
		{"shared folder", []string{title + "/a.mkv", title + `\sub\b.srt`}, "/downloads/flood/" + title, false},
		{"no shared folder", []string{"a/a.mkv", "b/b.mkv"}, "/downloads/flood", false},
		{"no contents", nil, "", true},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.torrents["ABC"] = torrentOf("seeding", client.DefaultMovieCategory)
		f.contents["ABC"] = tt.contents
		c := newClient(t, f, nil, nil)
		item := items(t, c)[0]
		got, err := c.(client.Importer).ImportItem(context.Background(), item, client.Options{})
		if tt.fails {
			var unavailable *client.UnavailableError
			if !errors.As(err, &unavailable) {
				t.Errorf("%s: ImportItem() error = %v, want an UnavailableError", tt.name, err)
			}
			continue
		}
		if err != nil {
			t.Fatalf("%s: ImportItem() error = %v", tt.name, err)
		}
		if got.OutputPath != tt.want || got.ID != item.ID {
			t.Errorf("%s: OutputPath = %q, want = %q", tt.name, got.OutputPath, tt.want)
		}
	}
}

func TestMarkImportedAndRemove(t *testing.T) {
	f := newFake(t)
	f.torrents["ABC"] = torrentOf("seeding", "tv-mavio", "keep")
	c := newClient(t, f, map[string]any{"tv_post_import_tags": []string{"imported", "keep"}}, nil)
	item := items(t, c)[0]
	if item.Kind != client.Series {
		t.Fatalf("Kind = %v, want = %v", item.Kind, client.Series)
	}
	if err := c.(client.Marker).MarkImported(context.Background(), item); err != nil {
		t.Fatalf("MarkImported() error = %v", err)
	}
	if err := c.Remove(context.Background(), item, true); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	var got []string
	for _, c := range f.calls {
		body, _ := json.Marshal(c.Body)
		got = append(got, c.Method+" "+c.Path+" "+string(body))
	}
	want := []string{
		`PATCH /torrents/tags {"hashes":["ABC"],"tags":["tv-mavio","keep","imported"]}`,
		`POST /torrents/delete {"deleteData":true,"hashes":["ABC"]}`,
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls = %q, want = %q", got, want)
	}

	f.calls = nil
	if err := newClient(t, f, nil, nil).(client.Marker).MarkImported(context.Background(), item); err != nil || len(f.calls) != 0 {
		t.Errorf("MarkImported() without post-import tags = %v, %d calls, want nothing done", err, len(f.calls))
	}
}

func TestInfo(t *testing.T) {
	f := newFake(t)
	info, err := newClient(t, f, map[string]any{"host": "localhost", "movie_directory": "/movies"}, nil).Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/movies", "/downloads/flood"}
	if !info.IsLocalhost || strings.Join(info.OutputRootFolders, "|") != strings.Join(want, "|") {
		t.Errorf("Info() = %+v, want localhost with %q", info, want)
	}
}

func TestAuth(t *testing.T) {
	f := newFake(t)
	c := newClient(t, f, nil, nil)
	if err := c.Test(context.Background()); err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	items(t, c)
	if f.logins != 1 {
		t.Errorf("logins = %d, want = 1", f.logins)
	}
	f.expireSession = true
	items(t, c)
	if f.logins != 2 {
		t.Errorf("logins after the session expired = %d, want = 2", f.logins)
	}
	var auth *client.AuthError
	if err := newClient(t, f, map[string]any{"password": "wrong"}, nil).Test(context.Background()); !errors.As(err, &auth) {
		t.Errorf("Test() with a wrong password error = %v, want an AuthError", err)
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"defaults", `{}`, true},
		{"tags", `{"movie_tags":["a","b"],"tv_post_import_tags":["done"]}`, true},
		{"additional tags", `{"additional_tags":["quality","studio"]}`, true},
		// Unexpected additional tag ID
		{"unexpected additional tag", `{"additional_tags":["network"]}`, false},
		{"unknown", `{"bogus":1}`, false},
		{"no host", `{"host":""}`, false},
		{"bad port", `{"port":0}`, false},
	}
	for _, tt := range tests {
		_, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
		if (err == nil) != tt.ok {
			t.Errorf("%s: New() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

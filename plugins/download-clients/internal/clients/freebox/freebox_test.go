package freebox

import (
	"context"
	"crypto/hmac"
	"crypto/sha1" //nolint:gosec // The Freebox API signs challenges with HMAC-SHA1.
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
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

const (
	magnetURL          = "magnet:?xt=urn:btih:5dee65101db281ac9c46344cd6b175cdcad53426&dn=download"
	movieTitle         = "Droned.1998.1080p.WEB-DL-DRONE"
	episodeTitle       = "Droned.S01E01.Pilot.1080p.WEB-DL-DRONE"
	defaultDestination = "/some/path"
	category           = "somecat"
	destinationDir     = "/path/to/media"
	appID              = "someid"
	appToken           = "S0mEv3RY1oN9T0k3n"
)

func encode(s string) string { return base64.StdEncoding.EncodeToString([]byte(s)) }

// added is a task added to the fake.
type added struct {
	URL, Dir, FileName, FileType string
	Data                         []byte
	// Settings are the settings PUT afterwards; nil without.
	Settings map[string]any
}

// fake is the Freebox OS API.
type fake struct {
	t  *testing.T
	mu sync.Mutex

	challenge string
	token     string
	sessions  int
	// loggedIn is what GET /login tells an authenticated app.
	loggedIn   bool
	defaultDir string
	tasks      []map[string]any
	// status answers every authenticated call with a status and error
	// code when set.
	status    int
	errorCode string

	added   []added
	removed []string
}

func newFake(t *testing.T) *fake {
	return &fake{t: t, challenge: "challenge1", loggedIn: true, defaultDir: defaultDestination}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	reply := func(result any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "result": result})
	}
	fail := func(status int, code string) {
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error_code": code, "msg": code})
	}
	resource, ok := strings.CutPrefix(r.URL.Path, "/api/v1")
	if !ok {
		http.NotFound(w, r)
		return
	}
	auth := r.Header.Get("X-Fbx-App-Auth")
	switch {
	case r.Method == http.MethodGet && resource == "/login" && auth == "":
		reply(map[string]any{"logged_in": false, "challenge": f.challenge})
		return
	case r.Method == http.MethodPost && resource == "/login/session":
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		mac := hmac.New(sha1.New, []byte(appToken))
		mac.Write([]byte(f.challenge))
		if body["app_id"] != appID || body["password"] != hex.EncodeToString(mac.Sum(nil)) {
			fail(http.StatusForbidden, "invalid_token")
			return
		}
		f.sessions++
		f.token = "token" + strconv.Itoa(f.sessions)
		reply(map[string]any{"session_token": f.token, "challenge": f.challenge})
		return
	}
	if auth != f.token || f.token == "" {
		fail(http.StatusForbidden, "auth_required")
		return
	}
	if f.status != 0 {
		fail(f.status, f.errorCode)
		return
	}
	switch {
	case r.Method == http.MethodGet && resource == "/login":
		reply(map[string]any{"logged_in": f.loggedIn})
	case r.Method == http.MethodGet && resource == "/downloads/config/":
		reply(map[string]any{"download_dir": encode(f.defaultDir)})
	case r.Method == http.MethodGet && resource == "/downloads/":
		reply(f.tasks)
	case r.Method == http.MethodPost && resource == "/downloads/add":
		var a added
		if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/") {
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				f.t.Errorf("multipart form: %v", err)
			}
			if fh := r.MultipartForm.File["download_file"]; len(fh) == 1 {
				file, _ := fh[0].Open()
				a.Data, _ = io.ReadAll(file)
				a.FileName, a.FileType = fh[0].Filename, fh[0].Header.Get("Content-Type")
			}
		} else {
			_ = r.ParseForm()
		}
		a.URL = r.Form.Get("download_url")
		if d := r.Form.Get("download_dir"); d != "" {
			dir, _ := base64.StdEncoding.DecodeString(d)
			a.Dir = string(dir)
		}
		f.added = append(f.added, a)
		reply(map[string]any{"id": 41 + len(f.added)})
	case r.Method == http.MethodPut && strings.HasPrefix(resource, "/downloads/"):
		if id := strings.TrimPrefix(resource, "/downloads/"); id != strconv.Itoa(41+len(f.added)) {
			f.t.Errorf("PUT of task %s, want the last added", id)
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.added[len(f.added)-1].Settings = body
		reply(nil)
	case r.Method == http.MethodDelete && strings.HasPrefix(resource, "/downloads/"):
		f.removed = append(f.removed, strings.TrimPrefix(resource, "/downloads/"))
		reply(nil)
	default:
		http.NotFound(w, r)
	}
}

// fixtureTask is the task of Radarr's fixture with a status.
func fixtureTask(status string) map[string]any {
	return map[string]any{
		"id": "id0", "name": "name", "download_dir": "L3NvbWUvcGF0aA==", "info_hash": "HASH", "queue_pos": 1,
		"status": status, "eta": 0, "error": "none", "type": "bt", "io_priority": "normal", "stop_ratio": 150,
		"piece_length": 125, "created_ts": 1665261599, "size": 1000, "rx_pct": 0, "rx_bytes": 0, "rx_rate": 0,
		"tx_pct": 0, "tx_bytes": 0, "tx_rate": 0,
	}
}

func (f *fake) setTasks(tasks ...map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tasks = tasks
}

// newClient returns a client talking to f, with settings over the
// fixture's.
func newClient(t *testing.T, f *fake, settings map[string]any) client.Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{"host": host, "port": p, "use_ssl": false, "api_url": "/api/v1/", "app_id": appID, "app_token": appToken}
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

// add sends a release of a kind with the fixture's magnet link.
func add(t *testing.T, c client.Client, r *client.Release) {
	t.Helper()
	if r.MagnetURL == "" && r.Data == nil {
		r.MagnetURL = magnetURL
	}
	id, err := c.Add(context.Background(), r)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}
	if id != "42" {
		t.Errorf("Add() = %q, want = 42", id)
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

// releases are the fixtures' releases: Radarr's movie and Sonarr's
// episode.
var releases = []struct {
	kind  client.Kind
	title string
	// age is the days of the older release in the priority cases.
	age int
}{{client.Movie, movieTitle, 22}, {client.Series, episodeTitle, 15}}

func TestAddDirectory(t *testing.T) {
	for _, rel := range releases {
		dirKey, categoryKey := "movie_directory", "movie_category"
		if rel.kind == client.Series {
			dirKey, categoryKey = "tv_directory", "tv_category"
		}
		tests := []struct {
			name     string
			settings map[string]any
			want     string
		}{
			// Download_with_DestinationDirectory_should_force_directory
			{"destination", map[string]any{dirKey: destinationDir}, destinationDir + "/" + rel.title},
			// Download_with_Category_should_force_directory
			{"category", map[string]any{categoryKey: category}, defaultDestination + "/" + category + "/" + rel.title},
			// Download_without_DestinationDirectory_and_Category_should_use_default
			{"default", nil, defaultDestination + "/" + rel.title},
		}
		for _, tt := range tests {
			f := newFake(t)
			add(t, newClient(t, f, tt.settings), &client.Release{Title: rel.title, Kind: rel.kind})
			if len(f.added) != 1 || f.added[0].Dir != tt.want || f.added[0].URL != magnetURL {
				t.Errorf("%v %s: added = %+v, want the magnet link in %q", rel.kind, tt.name, f.added, tt.want)
			}
		}
	}
}

func TestAddPaused(t *testing.T) {
	// Download_should_pause_torrent_as_expected
	for _, paused := range []bool{false, true} {
		f := newFake(t)
		add(t, newClient(t, f, map[string]any{"add_paused": paused}), &client.Release{Title: movieTitle, Kind: client.Movie})
		got := f.added[0].Settings["status"] == "stopped"
		if got != paused {
			t.Errorf("add_paused %v: settings = %v, want paused = %v", paused, f.added[0].Settings, paused)
		}
	}
}

func TestAddFirst(t *testing.T) {
	// Download_should_queue_torrent_first_as_expected
	tests := []struct {
		recent        bool
		older, latest int
		want          bool
	}{
		{true, priorityFirst, priorityFirst, true},
		{true, priorityLast, priorityFirst, true},
		{true, priorityFirst, priorityLast, false},
		{true, priorityLast, priorityLast, false},
		{false, priorityFirst, priorityFirst, true},
		{false, priorityLast, priorityFirst, false},
		{false, priorityFirst, priorityLast, true},
		{false, priorityLast, priorityLast, false},
	}
	for _, rel := range releases {
		prefix := "movie"
		if rel.kind == client.Series {
			prefix = "tv"
		}
		for _, tt := range tests {
			f := newFake(t)
			c := newClient(t, f, map[string]any{"older_" + prefix + "_priority": tt.older, "recent_" + prefix + "_priority": tt.latest})
			add(t, c, &client.Release{Title: rel.title, Kind: rel.kind, Recent: tt.recent})
			if got := f.added[0].Settings["queue_pos"] == "1"; got != tt.want {
				t.Errorf("%v recent %v older %d recent %d: settings = %v, want first = %v",
					rel.kind, tt.recent, tt.older, tt.latest, f.added[0].Settings, tt.want)
			}
		}
	}
}

func TestAddSeedRatio(t *testing.T) {
	// Download_should_define_seed_ratio_as_expected
	for _, tt := range []struct{ ratio, want float64 }{{0, 0}, {1.5, 150}} {
		f := newFake(t)
		add(t, newClient(t, f, nil), &client.Release{Title: movieTitle, Kind: client.Movie, SeedRatio: &tt.ratio})
		if got, ok := f.added[0].Settings["stop_ratio"].(float64); !ok || got != tt.want {
			t.Errorf("ratio %v: settings = %v, want stop_ratio = %v", tt.ratio, f.added[0].Settings, tt.want)
		}
	}
	// Without a ratio nor anything else, nothing is set.
	f := newFake(t)
	add(t, newClient(t, f, nil), &client.Release{Title: movieTitle, Kind: client.Movie})
	if f.added[0].Settings != nil {
		t.Errorf("settings = %v, want none", f.added[0].Settings)
	}
}

func TestAddTorrentFile(t *testing.T) {
	data := []byte("d4:infod6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
	f := newFake(t)
	add(t, newClient(t, f, nil), &client.Release{Title: movieTitle, Kind: client.Movie, Data: data})
	got := f.added[0]
	if got.FileName != movieTitle+".torrent" || string(got.Data) != string(data) || got.FileType != "multipart/form-data" ||
		got.Dir != defaultDestination+"/"+movieTitle {
		t.Errorf("added = %+v, want the torrent file in %s", got, defaultDestination+"/"+movieTitle)
	}
}

func TestItemsFilter(t *testing.T) {
	f := newFake(t)
	// GetItems_should_return_empty_list_if_no_tasks_available
	if got := items(t, newClient(t, f, nil)); len(got) != 0 {
		t.Errorf("no tasks: Items() = %+v, want none", got)
	}
	// GetItems_should_return_ignore_tasks_of_unknown_type
	unknown := fixtureTask("done")
	unknown["type"] = "toto"
	f.setTasks(unknown)
	if got := items(t, newClient(t, f, nil)); len(got) != 0 {
		t.Errorf("unknown type: Items() = %+v, want none", got)
	}
	f.setTasks(fixtureTask("done"))
	// GetItems_when_destinationdirectory_is_set_should_ignore_downloads_in_wrong_folder
	wrong := "/some/path/that/will/not/match"
	if got := items(t, newClient(t, f, map[string]any{"movie_directory": wrong, "tv_directory": wrong})); len(got) != 0 {
		t.Errorf("wrong directory: Items() = %+v, want none", got)
	}
	// GetItems_when_category_is_set_should_ignore_downloads_in_wrong_folder
	if got := items(t, newClient(t, f, map[string]any{"movie_category": "somecategory", "tv_category": "somecategory"})); len(got) != 0 {
		t.Errorf("wrong category: Items() = %+v, want none", got)
	}
}

func TestItemsKinds(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		dir      string
		want     int
		kind     client.Kind
		category string
	}{
		{"movie category", map[string]any{"movie_category": "radarr", "tv_category": "sonarr"}, "/some/path/radarr", 1, client.Movie, "radarr"},
		{"series category", map[string]any{"movie_category": "radarr", "tv_category": "sonarr"}, "/some/path/sonarr/x", 1, client.Series, "sonarr"},
		{"neither category", map[string]any{"movie_category": "radarr", "tv_category": "sonarr"}, "/some/path/other", 0, 0, ""},
		{"series directory", map[string]any{"movie_directory": "/media/movies", "tv_directory": "/media/tv"}, "/media/tv/x", 1, client.Series, ""},
		{"no filters", nil, "/some/path", 1, 0, ""},
		// A kind without a category or directory takes every other task.
		{"one category", map[string]any{"tv_category": "sonarr"}, "/some/path", 1, client.Movie, ""},
		{"same category", map[string]any{"movie_category": "media", "tv_category": "media"}, "/some/path/media", 1, 0, "media"},
	}
	for _, tt := range tests {
		f := newFake(t)
		task := fixtureTask("downloading")
		task["download_dir"] = encode(tt.dir)
		f.setTasks(task)
		got := items(t, newClient(t, f, tt.settings))
		if len(got) != tt.want {
			t.Errorf("%s: Items() = %d items, want = %d", tt.name, len(got), tt.want)
			continue
		}
		if tt.want > 0 && (got[0].Kind != tt.kind || got[0].Category != tt.category) {
			t.Errorf("%s: kind, category = %v, %q, want = %v, %q", tt.name, got[0].Kind, got[0].Category, tt.kind, tt.category)
		}
	}
}

func TestCanBeRemoved(t *testing.T) {
	// GetItems_should_return_canBeMoved_and_canBeDeleted_as_expected
	tests := []struct {
		status             string
		movable, removable bool
	}{
		{"downloading", false, false},
		{"done", true, true},
		{"seeding", false, false},
		{"stopped", false, false},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.setTasks(fixtureTask(tt.status))
		got := items(t, newClient(t, f, nil))
		if len(got) != 1 || got[0].CanMoveFiles != tt.movable || got[0].CanBeRemoved != tt.removable {
			t.Errorf("%s: Items() = %+v, want CanMoveFiles, CanBeRemoved = %v, %v", tt.status, got, tt.movable, tt.removable)
		}
	}
	// Only when completed downloads are to be removed.
	f := newFake(t)
	f.setTasks(fixtureTask("done"))
	got, err := newClient(t, f, nil).Items(context.Background(), client.Options{})
	if err != nil || len(got) != 1 || got[0].CanBeRemoved || got[0].CanMoveFiles {
		t.Errorf("Items() without removing = %+v, %v, want not removable", got, err)
	}
}

func TestItemStatus(t *testing.T) {
	// GetItems_should_return_item_as_downloadItemStatus
	tests := []struct {
		status string
		want   client.Status
	}{
		{"stopped", client.Paused},
		{"stopping", client.Paused},
		{"queued", client.Queued},
		{"starting", client.Downloading},
		{"downloading", client.Downloading},
		{"retry", client.Downloading},
		{"checking", client.Downloading},
		{"error", client.Warning},
		{"seeding", client.Completed},
		{"done", client.Completed},
		{"unknown", client.Downloading},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.setTasks(fixtureTask(tt.status))
		got := items(t, newClient(t, f, nil))
		if len(got) != 1 || got[0].Status != tt.want {
			t.Errorf("%s: Items() = %+v, want Status = %v", tt.status, got, tt.want)
		}
	}
}

func TestItem(t *testing.T) {
	f := newFake(t)
	task := fixtureTask("downloading")
	task["id"], task["rx_pct"], task["eta"] = 7, 2500, 30
	f.setTasks(task)
	got := items(t, newClient(t, f, nil))
	want := client.Item{ID: "7", Title: "name", TotalSize: 1000, RemainingSize: 750, OutputPath: defaultDestination, Status: client.Downloading}
	if len(got) != 1 {
		t.Fatalf("Items() = %+v, want one", got)
	}
	it := got[0]
	if it.ID != want.ID || it.Title != want.Title || it.TotalSize != want.TotalSize || it.RemainingSize != want.RemainingSize ||
		it.OutputPath != want.OutputPath || it.Status != want.Status {
		t.Errorf("Items() = %+v, want = %+v", it, want)
	}
	if it.RemainingTime == nil || *it.RemainingTime != 30*time.Second {
		t.Errorf("RemainingTime = %v, want = 30s", it.RemainingTime)
	}
	if it.SeedRatio == nil || *it.SeedRatio != 1.5 {
		t.Errorf("SeedRatio = %v, want = 1.5", it.SeedRatio)
	}
	if it.Message != "" {
		t.Errorf("Message = %q, want none", it.Message)
	}
}

func TestDecodedDirectory(t *testing.T) {
	// GetItems_should_return_decoded_destination_directory
	f := newFake(t)
	task := fixtureTask("done")
	task["download_dir"] = "L3RoYXQvdGhlL3BhdGg="
	f.setTasks(task)
	got := items(t, newClient(t, f, nil))
	if len(got) != 1 || got[0].OutputPath != "/that/the/path" {
		t.Errorf("Items() = %+v, want OutputPath = /that/the/path", got)
	}
}

func TestErrorMessage(t *testing.T) {
	// GetItems_should_return_message_if_tasks_in_error
	f := newFake(t)
	task := fixtureTask("error")
	task["error"] = "internal"
	f.setTasks(task)
	got := items(t, newClient(t, f, nil))
	if len(got) != 1 || got[0].Message != "Internal error." || got[0].Status != client.Warning {
		t.Errorf("Items() = %+v, want a warning with Internal error.", got)
	}
}

func TestInfo(t *testing.T) {
	tests := []struct {
		settings map[string]any
		want     []string
	}{
		{nil, []string{defaultDestination}},
		{map[string]any{"tv_category": "tv"}, []string{defaultDestination, defaultDestination + "/tv"}},
		{map[string]any{"movie_directory": "/media/movies/", "tv_directory": "/media/movies"}, []string{"/media/movies"}},
	}
	for _, tt := range tests {
		info, err := newClient(t, newFake(t), tt.settings).Info(context.Background())
		if err != nil || !slices.Equal(info.OutputRootFolders, tt.want) || !info.IsLocalhost {
			t.Errorf("%v: Info() = %+v, %v, want OutputRootFolders = %v on localhost", tt.settings, info, err, tt.want)
		}
	}
}

func TestRemove(t *testing.T) {
	f := newFake(t)
	c := newClient(t, f, nil)
	if err := c.Remove(context.Background(), client.Item{ID: "id0"}, false); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if err := c.Remove(context.Background(), client.Item{ID: "id1"}, true); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if want := []string{"id0", "id1/erase"}; !slices.Equal(f.removed, want) {
		t.Errorf("removed = %v, want = %v", f.removed, want)
	}
}

func TestSession(t *testing.T) {
	f := newFake(t)
	c := newClient(t, f, nil)
	items(t, c)
	items(t, c)
	if f.sessions != 1 {
		t.Errorf("sessions = %d, want = 1", f.sessions)
	}
	// An expired session is opened again.
	f.mu.Lock()
	f.token, f.challenge = "expired", "challenge2"
	f.mu.Unlock()
	items(t, c)
	if f.sessions != 2 {
		t.Errorf("sessions = %d, want = 2", f.sessions)
	}
	// A wrong app token is an AuthError.
	var auth *client.AuthError
	_, err := newClient(t, f, map[string]any{"app_token": "wrong"}).Items(context.Background(), client.Options{})
	if !errors.As(err, &auth) || !strings.Contains(err.Error(), "The app token you are trying to use is invalid or has been revoked.") {
		t.Errorf("Items() with a wrong app token error = %v, want an AuthError", err)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		code      string
		settings  map[string]any
		want      string
		auth      bool
		available bool
	}{
		{"api error", http.StatusOK, "hibernating", nil, "the Freebox API returned an error: The downloader is hibernating.", false, true},
		{"unknown api error", http.StatusOK, "bogus", nil, "bogus - Unknown error", false, true},
		{"forbidden", http.StatusForbidden, "insufficient_rights", nil, "Your app permissions does not allow accessing this API.", true, true},
		{"server error", http.StatusInternalServerError, "", nil, "unable to connect to the Freebox", false, true},
		{"api url", 0, "", map[string]any{"api_url": "/api/v9/"}, "verify the API URL", false, true},
	}
	for _, tt := range tests {
		f := newFake(t)
		c := newClient(t, f, tt.settings)
		f.status, f.errorCode = tt.status, tt.code
		_, err := c.Items(context.Background(), client.Options{})
		var auth *client.AuthError
		if err == nil || !strings.Contains(err.Error(), tt.want) || errors.As(err, &auth) != tt.auth {
			t.Errorf("%s: Items() error = %v, want %q (auth %v)", tt.name, err, tt.want, tt.auth)
		}
	}
	// An unreachable Freebox.
	srv := httptest.NewServer(http.NotFoundHandler())
	addr := srv.Listener.Addr().String()
	srv.Close()
	host, port, _ := net.SplitHostPort(addr)
	p, _ := strconv.Atoi(port)
	raw, _ := json.Marshal(map[string]any{"host": host, "port": p, "use_ssl": false, "app_id": appID, "app_token": appToken})
	c, err := Definitions()[0].New(raw, client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	var unavailable *client.UnavailableError
	if err := c.Test(context.Background()); !errors.As(err, &unavailable) || !strings.Contains(err.Error(), "verify the host, port and SSL settings") {
		t.Errorf("Test() of an unreachable Freebox error = %v, want an UnavailableError", err)
	}
}

func TestTest(t *testing.T) {
	f := newFake(t)
	if err := newClient(t, f, nil).Test(context.Background()); err != nil {
		t.Errorf("Test() error = %v, want nil", err)
	}
	f.loggedIn = false
	var auth *client.AuthError
	if err := newClient(t, f, nil).Test(context.Background()); !errors.As(err, &auth) {
		t.Errorf("Test() not logged in error = %v, want an AuthError", err)
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"app", `{"app_id":"a","app_token":"t"}`, true},
		{"no app id", `{"app_token":"t"}`, false},
		{"no app token", `{"app_id":"a"}`, false},
		{"no api url", `{"app_id":"a","app_token":"t","api_url":""}`, false},
		{"absolute api url", `{"app_id":"a","app_token":"t","api_url":"http://box/api/v1/"}`, false},
		{"bad category", `{"app_id":"a","app_token":"t","tv_category":"tv_1"}`, false},
		{"relative directory", `{"app_id":"a","app_token":"t","tv_directory":"media/tv"}`, false},
		{"category and directory", `{"app_id":"a","app_token":"t","tv_category":"tv","tv_directory":"/media/tv"}`, false},
		{"category of one, directory of the other", `{"app_id":"a","app_token":"t","tv_category":"tv","movie_directory":"/media/movies"}`, true},
		{"no host", `{"app_id":"a","app_token":"t","host":""}`, false},
		{"unknown", `{"app_id":"a","app_token":"t","bogus":1}`, false},
	}
	for _, tt := range tests {
		_, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
		if (err == nil) != tt.ok {
			t.Errorf("%s: New() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

func TestURLPathEncode(t *testing.T) {
	tests := []struct{ in, want string }{
		{magnetURL, magnetURL},
		{"http://host/a b/é?q=a b", "http://host/a%20b/%C3%A9?q=a b"},
	}
	for _, tt := range tests {
		if got := urlPathEncode(tt.in); got != tt.want {
			t.Errorf("urlPathEncode(%q) = %q, want = %q", tt.in, got, tt.want)
		}
	}
}

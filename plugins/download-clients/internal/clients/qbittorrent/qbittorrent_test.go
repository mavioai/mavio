package qbittorrent

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
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const title = "Droned.1998.1080p.WEB-DL-DRONE"

// fake is qBittorrent's Web API, v2 or v1.
type fake struct {
	t  *testing.T
	mu sync.Mutex
	// api is 1 or 2; version is what webapiVersion, or for v1 version/api,
	// answers.
	api     int
	version string
	apiKey  string

	torrents   []map[string]any
	prefs      map[string]any
	categories map[string]map[string]any
	// files and seeding are the files and seeding times by lower-case hash.
	files   map[string][]map[string]any
	seeding map[string]int
	// status answers the paths with a status.
	status map[string]int
	// noCreate makes creating categories do nothing.
	noCreate bool
	// expire rejects the session once.
	expire bool

	calls     []call
	logins    int
	probes    int
	propCalls int
}

type call struct {
	Path     string
	Query    url.Values
	Form     url.Values
	FileName string
	File     []byte
}

func newFake(t *testing.T, api int) *fake {
	f := &fake{
		t: t, api: api, version: "2.5.1",
		prefs:      map[string]any{"dht": true},
		categories: map[string]map[string]any{},
		files:      map[string][]map[string]any{},
		seeding:    map[string]int{},
		status:     map[string]int{},
	}
	if api == 1 {
		f.version = "18"
	}
	return f
}

func (f *fake) write(w http.ResponseWriter, v any) {
	if s, ok := v.(string); ok {
		_, _ = io.WriteString(w, s)
		return
	}
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fake) torrent(hash string) map[string]any {
	for _, t := range f.torrents {
		if strings.EqualFold(t["hash"].(string), hash) {
			return t
		}
	}
	return nil
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := r.URL.Path
	if f.api != 1 && f.api != 2 || strings.HasPrefix(p, "/api/v2/") != (f.api == 2) {
		http.NotFound(w, r)
		return
	}
	if err := r.ParseMultipartForm(1 << 20); err != nil && !errors.Is(err, http.ErrNotMultipart) {
		f.t.Errorf("%s: form: %v", p, err)
	}
	if p == "/api/v2/auth/login" || p == "/login" {
		f.logins++
		if r.PostForm.Get("username") == "admin" && r.PostForm.Get("password") == "pass" {
			http.SetCookie(w, &http.Cookie{Name: "SID", Value: "session"})
			f.write(w, "Ok.")
		} else {
			f.write(w, "Fails.")
		}
		return
	}
	if p == "/version/api" {
		if _, err := r.Cookie("SID"); err != nil {
			f.probes++
		}
		f.write(w, f.version)
		return
	}
	authed := false
	if f.apiKey != "" {
		authed = r.Header.Get("Authorization") == "Bearer "+f.apiKey
	} else if ck, err := r.Cookie("SID"); err == nil && ck.Value == "session" {
		authed = !f.expire
		f.expire = false
	}
	if !authed {
		if p == "/api/v2/app/webapiVersion" {
			f.probes++
		}
		w.WriteHeader(http.StatusForbidden)
		return
	}
	c := call{Path: p, Query: r.URL.Query(), Form: r.PostForm}
	if r.MultipartForm != nil {
		for _, fh := range r.MultipartForm.File["torrents"] {
			c.FileName = fh.Filename
			file, _ := fh.Open()
			c.File, _ = io.ReadAll(file)
			_ = file.Close()
		}
	}
	f.calls = append(f.calls, c)
	if code := f.status[p]; code != 0 {
		w.WriteHeader(code)
		return
	}
	hash := r.URL.Query().Get("hash")
	if i := strings.LastIndex(p, "/"); strings.HasPrefix(p, "/query/properties") {
		hash = p[i+1:]
		p = p[:i]
	}
	switch p {
	case "/api/v2/app/webapiVersion":
		f.write(w, f.version)
	case "/api/v2/app/preferences", "/query/preferences":
		f.write(w, f.prefs)
	case "/api/v2/torrents/info", "/query/torrents":
		category := r.URL.Query().Get("category")
		out := []map[string]any{}
		for _, t := range f.torrents {
			if category == "" || t["category"] == category || t["label"] == category {
				out = append(out, t)
			}
		}
		f.write(w, out)
	case "/api/v2/torrents/properties", "/query/propertiesGeneral":
		f.propCalls++
		t := f.torrent(hash)
		if t == nil {
			http.NotFound(w, r)
			return
		}
		f.write(w, map[string]any{"save_path": t["save_path"], "seeding_time": f.seeding[strings.ToLower(hash)]})
	case "/api/v2/torrents/files", "/query/propertiesFiles":
		files, ok := f.files[strings.ToLower(hash)]
		if t := f.torrent(hash); !ok && t != nil {
			files = []map[string]any{{"name": t["name"]}}
		}
		f.write(w, files)
	case "/api/v2/torrents/add", "/command/download", "/command/upload":
		var h string
		if c.File != nil {
			h, _ = client.InfoHash(c.File)
		} else {
			h, _ = client.MagnetHash(r.PostForm.Get("urls"))
		}
		f.torrents = append(f.torrents, map[string]any{
			"hash": strings.ToLower(h), "name": title, "size": 1000, "progress": 1.0, "eta": 8640000,
			"state": "queuedUP", "category": r.PostForm.Get("category"), "save_path": "",
		})
	case "/api/v2/torrents/categories":
		f.write(w, f.categories)
	case "/api/v2/torrents/createCategory", "/command/addCategory":
		if !f.noCreate {
			name := r.PostForm.Get("category")
			f.categories[name] = map[string]any{"name": name, "savePath": ""}
		}
	}
}

// posts are the paths of the calls that change something.
func (f *fake) posts() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if len(c.Form) > 0 || c.File != nil {
			out = append(out, c)
		}
	}
	return out
}

// all are the calls signed in.
func (f *fake) all() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}

// stats are the counts of logins, probes and properties fetched.
func (f *fake) stats() (logins, probes, propCalls int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.logins, f.probes, f.propCalls
}

func (f *fake) paths() []string {
	var out []string
	for _, c := range f.posts() {
		out = append(out, c.Path)
	}
	return out
}

// newClient returns a client talking to f, with settings over the
// fixture's: Radarr's movie category and Sonarr's series one.
func newClient(t *testing.T, f *fake, settings map[string]any) *qbittorrent {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{
		"host": host, "port": p, "username": "admin", "password": "pass",
		"movie_category": "movies-radarr", "tv_category": "tv",
	}
	for k, v := range settings {
		s[k] = v
	}
	raw, _ := json.Marshal(s)
	c, err := Definitions()[0].New(raw, client.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c.(*qbittorrent)
}

// kinds are the categories of Radarr's and Sonarr's fixtures.
var kinds = []struct {
	kind     client.Kind
	category string
}{{client.Movie, "movies-radarr"}, {client.Series, "tv"}}

// each runs a test against both APIs and the categories of both kinds.
func each(t *testing.T, test func(t *testing.T, f *fake, category string, kind client.Kind)) {
	for _, api := range []int{2, 1} {
		for _, k := range kinds {
			t.Run("v"+strconv.Itoa(api)+"/"+k.category, func(t *testing.T) { test(t, newFake(t, api), k.category, k.kind) })
		}
	}
}

// newTorrent is the torrent of Radarr's fixture.
func newTorrent(category, state string, progress float64, eta int) map[string]any {
	return map[string]any{
		"hash": "HASH", "name": title, "size": 1000, "progress": progress, "eta": eta, "state": state,
		"label": "", "category": category, "save_path": "",
	}
}

func single(t *testing.T, c client.Client) client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), client.Options{RemoveCompletedDownloads: true})
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("Items() = %d items, want = 1", len(items))
	}
	return items[0]
}

func TestItemStatus(t *testing.T) {
	tests := []struct {
		name     string
		state    string
		progress float64
		eta      int
		want     client.Status
		// wantTime is the remaining time; -1 for none.
		wantTime time.Duration
	}{
		// error_item_should_have_required_properties
		{"error", "error", 0.7, 8640000, client.Warning, -1},
		// paused_item_should_have_required_properties
		{"pausedDL", "pausedDL", 0.7, 8640000, client.Paused, -1},
		{"stoppedDL", "stoppedDL", 0.7, 8640000, client.Paused, -1},
		// completed_item_should_have_required_properties
		{"pausedUP", "pausedUP", 1, 8640000, client.Completed, 0},
		{"stoppedUP", "stoppedUP", 1, 8640000, client.Completed, 0},
		{"queuedUP", "queuedUP", 1, 8640000, client.Completed, 0},
		{"uploading", "uploading", 1, 8640000, client.Completed, 0},
		{"stalledUP", "stalledUP", 1, 8640000, client.Completed, 0},
		{"forcedUP", "forcedUP", 1, 8640000, client.Completed, 0},
		// queued_item_should_have_required_properties
		{"queuedDL", "queuedDL", 0.7, 8640000, client.Queued, -1},
		{"checkingDL", "checkingDL", 0.7, 8640000, client.Queued, -1},
		{"checkingUP", "checkingUP", 0.7, 8640000, client.Queued, -1},
		{"metaDL", "metaDL", 0.7, 8640000, client.Queued, -1},
		{"checkingResumeData", "checkingResumeData", 0.7, 8640000, client.Queued, -1},
		// downloading_item_should_have_required_properties
		{"downloading", "downloading", 0.7, 60, client.Downloading, time.Minute},
		// stalledDL_item_should_have_required_properties
		{"stalledDL", "stalledDL", 0.7, 8640000, client.Warning, -1},
		// missingFiles_item_should_have_required_properties
		{"missingFiles", "missingFiles", 0.7, 8640000, client.Warning, -1},
		{"unknown", "somethingNew", 0.7, 60, client.Downloading, time.Minute},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			each(t, func(t *testing.T, f *fake, category string, kind client.Kind) {
				f.torrents = []map[string]any{newTorrent(category, tt.state, tt.progress, tt.eta)}
				item := single(t, newClient(t, f, nil))
				if item.Status != tt.want {
					t.Errorf("Status = %v, want = %v", item.Status, tt.want)
				}
				if item.ID != "HASH" || item.Title != title || item.Kind != kind || item.Category != category {
					t.Errorf("item = %+v, want the torrent's hash and name, kind %v", item, kind)
				}
				switch {
				case tt.wantTime < 0 && item.RemainingTime != nil:
					t.Errorf("RemainingTime = %v, want = nil", *item.RemainingTime)
				case tt.wantTime >= 0 && (item.RemainingTime == nil || *item.RemainingTime != tt.wantTime):
					t.Errorf("RemainingTime = %v, want = %v", item.RemainingTime, tt.wantTime)
				}
				wantRemaining := int64(300)
				if tt.progress == 1 {
					wantRemaining = 0
				}
				if item.RemainingSize != wantRemaining {
					t.Errorf("RemainingSize = %d, want = %d", item.RemainingSize, wantRemaining)
				}
				if (tt.want == client.Warning) != (item.Message != "") && tt.state != "metaDL" && tt.state != "somethingNew" {
					t.Errorf("Message = %q, want one for warnings alone", item.Message)
				}
			})
		})
	}
}

func TestMetadataWithoutDHT(t *testing.T) {
	f := newFake(t, 2)
	f.prefs["dht"] = false
	f.torrents = []map[string]any{newTorrent("movies-radarr", "metaDL", 0, 8640000)}
	if item := single(t, newClient(t, f, nil)); item.Status != client.Warning || item.Message == "" {
		t.Errorf("Status, Message = %v, %q, want = Warning with the reason", item.Status, item.Message)
	}
}

func TestImportItem(t *testing.T) {
	tests := []struct {
		name     string
		savePath string
		files    []string
		want     string
	}{
		// single_file_torrent_outputpath_should_have_sanitised_name
		{"single file", `C:\Torrents`, []string{"Droned.S01E01.Tests.1080p.WEB-DL-DRONE.mkv"}, `C:\Torrents\Droned.S01E01.Tests.1080p.WEB-DL-DRONE.mkv`},
		{"single file unix", "/torrents", []string{"Droned.S01E01.Tests.1080p.WEB-DL-DRONE.mkv"}, "/torrents/Droned.S01E01.Tests.1080p.WEB-DL-DRONE.mkv"},
		// single_file_torrent_with_folder_should_only_have_first_subfolder
		{"folder", `C:\Torrents`, []string{"Folder/Droned.S01E01.Tests.1080p.WEB-DL-DRONE.mkv"}, `C:\Torrents\Folder`},
		// multi_file_torrent_outputpath_should_have_sanitised_name
		{"multi file", "/torrents", []string{"Droned.S01.12/E01.mkv", "Droned.S01.12/E02.mkv"}, "/torrents/Droned.S01.12"},
		{"multi file backslashes", `C:\Torrents`, []string{`Droned.S01.12\E01.mkv`, `Droned.S01.12\E02.mkv`}, `C:\Torrents\Droned.S01.12`},
		{"no files", "/torrents", []string{}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			each(t, func(t *testing.T, f *fake, category string, _ client.Kind) {
				tor := newTorrent(category, "stalledDL", 0.7, 8640000)
				tor["name"], tor["save_path"] = `Droned.S01.\1/2`, tt.savePath
				f.torrents = []map[string]any{tor}
				files := []map[string]any{}
				for _, n := range tt.files {
					files = append(files, map[string]any{"name": n})
				}
				f.files["hash"] = files
				item, err := newClient(t, f, nil).ImportItem(context.Background(), client.Item{ID: "HASH"}, client.Options{})
				if err != nil {
					t.Fatalf("ImportItem() error = %v", err)
				}
				if item.OutputPath != tt.want {
					t.Errorf("OutputPath = %q, want = %q", item.OutputPath, tt.want)
				}
			})
		})
	}
}

// api_261_should_use_content_path
func TestContentPath(t *testing.T) {
	for _, state := range []string{"pausedUP", "stoppedUP"} {
		for _, version := range []string{"2.6.1", "2.6.0"} {
			f := newFake(t, 2)
			f.version = version
			tor := newTorrent("movies-radarr", state, 0.7, 8640000)
			tor["save_path"], tor["content_path"] = `C:\Torrents`, `C:\Torrents\Droned.S01.12`
			f.torrents = []map[string]any{tor}
			c := newClient(t, f, nil)
			item := single(t, c)
			want := `C:\Torrents\Droned.S01.12`
			if version == "2.6.0" {
				want = ""
			}
			if item.OutputPath != want {
				t.Errorf("%s %s: OutputPath = %q, want = %q", state, version, item.OutputPath, want)
			}
			if version == "2.6.0" {
				continue
			}
			imported, err := c.ImportItem(context.Background(), item, client.Options{})
			if err != nil || imported.OutputPath != want {
				t.Errorf("%s: ImportItem() = %q, %v, want = %q", state, imported.OutputPath, err, want)
			}
			if slices.ContainsFunc(f.all(), func(c call) bool { return strings.Contains(c.Path, "files") }) {
				t.Errorf("ImportItem() fetched the files, want the content path kept")
			}
			// The content path being the save path cannot be imported.
			tor["content_path"] = `C:\Torrents\`
			if item := single(t, c); item.Status != client.Warning || item.OutputPath != "" {
				t.Errorf("%s: status, OutputPath = %v, %q, want = Warning, empty", state, item.Status, item.OutputPath)
			}
		}
	}
}

// torrentFile is a minimal torrent file.
func torrentFile() []byte {
	return []byte("d4:infod6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
}

const magnetHash = "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951"

func add(t *testing.T, c client.Client, r client.Release) (string, error) {
	t.Helper()
	r.Title = title
	return c.Add(context.Background(), &r)
}

func TestAdd(t *testing.T) {
	data := torrentFile()
	hash, err := client.InfoHash(data)
	if err != nil {
		t.Fatal(err)
	}
	// Download_should_return_unique_id
	each(t, func(t *testing.T, f *fake, category string, kind client.Kind) {
		id, err := add(t, newClient(t, f, nil), client.Release{Kind: kind, Data: data, FileName: "a.torrent"})
		if err != nil || id != hash {
			t.Fatalf("Add() = %q, %v, want = %q", id, err, hash)
		}
		posts := f.posts()
		want := map[int]string{1: "/command/upload", 2: "/api/v2/torrents/add"}[f.api]
		if len(posts) != 1 || posts[0].Path != want || string(posts[0].File) != string(data) || posts[0].FileName != "a.torrent" {
			t.Fatalf("calls = %+v, want an upload of the torrent file to %s", posts, want)
		}
		if got := posts[0].Form.Get("category"); got != category {
			t.Errorf("category = %q, want = %q", got, category)
		}
	})
	// Download_should_get_hash_from_magnet_url
	each(t, func(t *testing.T, f *fake, _ string, kind client.Kind) {
		link := "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp"
		id, err := add(t, newClient(t, f, nil), client.Release{Kind: kind, MagnetURL: link})
		if err != nil || id != magnetHash {
			t.Fatalf("Add() = %q, %v, want = %q", id, err, magnetHash)
		}
		if posts := f.posts(); len(posts) != 1 || posts[0].Form.Get("urls") != link {
			t.Errorf("calls = %+v, want the magnet link added", posts)
		}
	})
}

func TestAddMagnetWithoutDHT(t *testing.T) {
	tests := []struct {
		name, link string
		ok         bool
	}{
		// Download_should_refuse_magnet_if_no_trackers_provided_and_dht_is_disabled
		{"no trackers", "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR", false},
		// Download_should_accept_magnet_if_trackers_provided_and_dht_is_disabled
		{"trackers", "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp://abc", true},
	}
	for _, tt := range tests {
		for _, api := range []int{2, 1} {
			f := newFake(t, api)
			f.prefs["dht"] = false
			_, err := add(t, newClient(t, f, nil), client.Release{Kind: client.Movie, MagnetURL: tt.link})
			if tt.ok && err != nil || !tt.ok && !errors.Is(err, client.ErrMagnetNotSupported) {
				t.Errorf("%s v%d: Add() error = %v, want ok = %v or ErrMagnetNotSupported", tt.name, api, err, tt.ok)
			}
			if n := len(f.posts()); tt.ok && n != 1 || !tt.ok && n != 0 {
				t.Errorf("%s v%d: %d adds, want = %v", tt.name, api, n, tt.ok)
			}
		}
	}
}

func TestAddTopPriority(t *testing.T) {
	data := torrentFile()
	first := map[string]any{
		"recent_movie_priority": priorityFirst, "older_movie_priority": priorityFirst,
		"recent_tv_priority": priorityFirst, "older_tv_priority": priorityFirst,
	}
	// Download_should_set_top_priority
	each(t, func(t *testing.T, f *fake, _ string, kind client.Kind) {
		if _, err := add(t, newClient(t, f, first), client.Release{Kind: kind, Data: data}); err != nil {
			t.Fatalf("Add() error = %v", err)
		}
		want := map[int]string{1: "/command/topPrio", 2: "/api/v2/torrents/topPrio"}[f.api]
		if got := f.paths(); len(got) != 2 || got[1] != want {
			t.Errorf("calls = %v, want the add and %s", got, want)
		}
	})
	// Download_should_not_fail_if_top_priority_not_available
	for _, api := range []int{2, 1} {
		for _, code := range []int{http.StatusConflict, http.StatusForbidden, http.StatusInternalServerError} {
			f := newFake(t, api)
			f.status["/api/v2/torrents/topPrio"], f.status["/command/topPrio"] = code, code
			id, err := add(t, newClient(t, f, first), client.Release{Kind: client.Movie, Recent: true, Data: data})
			if err != nil || id == "" {
				t.Errorf("v%d %d: Add() = %q, %v, want the hash", api, code, id, err)
			}
		}
	}
	// Only the kind's priority counts.
	f := newFake(t, 2)
	if _, err := add(t, newClient(t, f, map[string]any{"recent_tv_priority": priorityFirst}), client.Release{Kind: client.Movie, Recent: true, Data: data}); err != nil {
		t.Fatal(err)
	}
	if got := f.paths(); len(got) != 1 {
		t.Errorf("calls = %v, want the add alone for a movie", got)
	}
}

func TestAddForm(t *testing.T) {
	data := torrentFile()
	ratio, seedTime := 1.5, 90*time.Minute
	tests := []struct {
		name     string
		api      int
		version  string
		settings map[string]any
		seed     bool
		want     map[string]string
		wantRest []string
	}{
		{"started", 2, "2.11.0", nil, false, map[string]string{"stopped": "false"}, nil},
		{"stopped", 2, "2.11.0", map[string]any{"initial_state": stateStop}, false, map[string]string{"stopped": "true"}, nil},
		{"paused", 2, "2.10.4", map[string]any{"initial_state": stateStop}, false, map[string]string{"paused": "true"}, nil},
		{
			"force started", 2, "2.11.0",
			map[string]any{"initial_state": stateForceStart},
			false,
			map[string]string{"stopped": "", "paused": ""},
			[]string{"/api/v2/torrents/setForceStart"},
		},
		{
			"order and layout", 2, "2.11.0",
			map[string]any{"sequential_order": true, "first_and_last": true, "content_layout": layoutSubfolder},
			false,
			map[string]string{"sequentialDownload": "true", "firstLastPiecePrio": "true", "contentLayout": "Subfolder"},
			nil,
		},
		{"original layout", 2, "2.11.0", map[string]any{"content_layout": layoutOriginal}, false, map[string]string{"contentLayout": "Original"}, nil},
		{"seed limits on add", 2, "2.8.1", nil, true, map[string]string{"ratioLimit": "1.5", "seedingTimeLimit": "90"}, nil},
		{
			"seed limits after add", 2, "2.8.0", nil, true,
			map[string]string{"ratioLimit": "", "seedingTimeLimit": ""},
			[]string{"/api/v2/torrents/setShareLimits"},
		},
		{"v1 started", 1, "18", nil, true, map[string]string{"paused": "false", "ratioLimit": ""}, nil},
		{"v1 stopped", 1, "18", map[string]any{"initial_state": stateStop, "sequential_order": true}, false, map[string]string{"paused": "true", "sequentialDownload": ""}, nil},
		{"v1 force started", 1, "18", map[string]any{"initial_state": stateForceStart}, false, map[string]string{"paused": ""}, []string{"/command/setForceStart"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t, tt.api)
			f.version = tt.version
			r := client.Release{Kind: client.Series, Data: data}
			if tt.seed {
				r.SeedRatio, r.SeedTime = &ratio, &seedTime
			}
			if _, err := add(t, newClient(t, f, tt.settings), r); err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			posts := f.posts()
			if len(posts) != 1+len(tt.wantRest) {
				t.Fatalf("calls = %v, want the add then %v", f.paths(), tt.wantRest)
			}
			for k, v := range tt.want {
				if got := posts[0].Form.Get(k); got != v {
					t.Errorf("%s = %q, want = %q", k, got, v)
				}
			}
			for i, p := range tt.wantRest {
				if posts[i+1].Path != p || posts[i+1].Form.Get("hashes") != strings.ToLower(posts[i+1].Form.Get("hashes")) {
					t.Errorf("calls[%d] = %+v, want %s with the lower-case hash", i+1, posts[i+1], p)
				}
			}
			if slices.Contains(tt.wantRest, "/api/v2/torrents/setShareLimits") {
				if got := posts[1].Form; got.Get("ratioLimit") != "1.5" || got.Get("seedingTimeLimit") != "90" {
					t.Errorf("setShareLimits = %v, want the ratio 1.5 and 90 minutes", got)
				}
			}
			if slices.ContainsFunc(tt.wantRest, func(p string) bool { return strings.HasSuffix(p, "setForceStart") }) {
				if got := posts[1].Form.Get("value"); got != "true" {
					t.Errorf("setForceStart value = %q, want = true", got)
				}
			}
		})
	}
	// Share limits left to the global ones are -2 when set after adding.
	f := newFake(t, 2)
	f.version = "2.0.1"
	if _, err := add(t, newClient(t, f, nil), client.Release{Kind: client.Movie, Data: data, SeedRatio: &ratio}); err != nil {
		t.Fatal(err)
	}
	if posts := f.posts(); len(posts) != 2 || posts[1].Form.Get("seedingTimeLimit") != "-2" {
		t.Errorf("calls = %+v, want setShareLimits with the global seeding time", posts)
	}
}

func TestInfo(t *testing.T) {
	// should_return_status_with_outputdirs
	for _, api := range []int{2, 1} {
		f := newFake(t, api)
		f.prefs["save_path"] = `C:\Downloads\Finished\QBittorrent`
		info, err := newClient(t, f, map[string]any{"host": "127.0.0.1"}).Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsLocalhost || !slices.Equal(info.OutputRootFolders, []string{`C:\Downloads\Finished\QBittorrent`}) {
			t.Errorf("v%d: Info() = %+v, want localhost with the save path", api, info)
		}
	}
	// should_correct_category_output_path
	for _, k := range kinds {
		f := newFake(t, 2)
		f.version = "2.0"
		f.prefs["save_path"] = `C:\Downloads\Finished\QBittorrent`
		f.categories[k.category] = map[string]any{"name": k.category, "savePath": "//server/store/downloads"}
		info, err := newClient(t, f, map[string]any{"host": "127.0.0.1"}).Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		want := []string{`\\server\store\downloads`, `C:\Downloads\Finished\QBittorrent`}
		if k.kind == client.Series {
			want[0], want[1] = want[1], want[0]
		}
		if !slices.Equal(info.OutputRootFolders, want) {
			t.Errorf("%s: OutputRootFolders = %v, want = %v", k.category, info.OutputRootFolders, want)
		}
	}
	// A relative category folder is in the save path.
	f := newFake(t, 2)
	f.prefs["save_path"] = "/downloads"
	f.prefs["max_ratio_enabled"], f.prefs["max_ratio_act"] = true, ratioActionRemove
	f.categories["tv"] = map[string]any{"name": "tv", "savePath": "tv-folder"}
	info, err := newClient(t, f, nil).Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(info.OutputRootFolders, []string{"/downloads", "/downloads/tv-folder"}) || !info.RemovesCompletedDownloads {
		t.Errorf("Info() = %+v, want the save path and the category's, removing completed downloads", info)
	}
}

// globalSeedLimits are Radarr's GivenGlobalSeedLimits; negative is off.
func globalSeedLimits(f *fake, maxRatio float64, maxSeedingTime, maxInactiveSeedingTime int) {
	f.prefs["max_ratio_act"] = 0
	f.prefs["max_ratio"], f.prefs["max_ratio_enabled"] = maxRatio, maxRatio >= 0
	f.prefs["max_seeding_time"], f.prefs["max_seeding_time_enabled"] = maxSeedingTime, maxSeedingTime >= 0
	f.prefs["max_inactive_seeding_time"], f.prefs["max_inactive_seeding_time_enabled"] = maxInactiveSeedingTime, maxInactiveSeedingTime >= 0
}

// completed is Radarr's GivenCompletedTorrent.
type completed struct {
	state                                       string
	ratio, ratioLimit                           float64
	seedingTime, seedingTimeLimit, inactiveTime int
	// inactive is how long ago the torrent was last active.
	inactive time.Duration
}

func (c completed) torrent(f *fake, category string) {
	tor := newTorrent(category, c.state, 1, 8640000)
	tor["ratio"], tor["ratio_limit"] = c.ratio, c.ratioLimit
	tor["seeding_time_limit"], tor["inactive_seeding_time_limit"] = c.seedingTimeLimit, c.inactiveTime
	tor["last_activity"] = time.Now().Add(-c.inactive).Unix()
	f.torrents = []map[string]any{tor}
	f.seeding["hash"] = c.seedingTime * 60
}

func done(state string, ratio float64) completed {
	return completed{state: state, ratio: ratio, ratioLimit: -2, seedingTime: 1, seedingTimeLimit: -2, inactiveTime: -2}
}

func TestSeedLimits(t *testing.T) {
	with := func(c completed, set func(*completed)) completed { set(&c); return c }
	type limits struct {
		ratio             float64
		seeding, inactive int
	}
	tests := []struct {
		name      string
		global    limits
		torrent   completed
		removable bool
	}{
		{"max ratio not reached", limits{1, -1, -1}, done("uploading", 0.5), false},
		{"max ratio reached and not paused", limits{1, -1, -1}, done("uploading", 1), false},
		{"max seeding time reached and not paused", limits{-1, 20, -1}, with(done("uploading", 2), func(c *completed) { c.seedingTime = 30 }), false},
		{"max inactive seeding time reached and not paused", limits{-1, -1, 20}, with(done("uploading", 2), func(c *completed) { c.inactive = 25 * time.Minute }), false},
	}
	for _, state := range []string{"pausedUP", "stoppedUP"} {
		tests = append(tests, []struct {
			name      string
			global    limits
			torrent   completed
			removable bool
		}{
			{"max ratio is not set", limits{-1, -1, -1}, done(state, 1), false},
			{"max ratio reached and paused", limits{1, -1, -1}, done(state, 1), true},
			{"max ratio reached after rounding and paused", limits{1, -1, -1}, done(state, 1.1006066990976857), true},
			{"just under max ratio reached after rounding and paused", limits{1, -1, -1}, done(state, 0.9999), true},
			{"overridden max ratio reached and paused", limits{2, -1, -1}, with(done(state, 1), func(c *completed) { c.ratioLimit = 0.8 }), true},
			{"overridden max ratio reached after rounding and paused", limits{2, -1, -1}, with(done(state, 1.1006066990976857), func(c *completed) { c.ratioLimit = 1.1 }), true},
			{"just under overridden max ratio reached after rounding and paused", limits{2, -1, -1}, with(done(state, 0.9999), func(c *completed) { c.ratioLimit = 1 }), true},
			{"overridden max ratio not reached and paused", limits{0.2, -1, -1}, with(done(state, 0.5), func(c *completed) { c.ratioLimit = 0.8 }), false},
			{"max seeding time reached and paused", limits{-1, 20, -1}, with(done(state, 2), func(c *completed) { c.seedingTime = 20 }), true},
			{"overridden max seeding time reached and paused", limits{-1, 40, -1}, with(done(state, 2), func(c *completed) { c.seedingTime, c.seedingTimeLimit = 20, 10 }), true},
			{"overridden max seeding time not reached and paused", limits{-1, 20, -1}, with(done(state, 2), func(c *completed) { c.seedingTime, c.seedingTimeLimit = 30, 40 }), false},
			{"max inactive seeding time reached and paused", limits{-1, -1, 20}, with(done(state, 2), func(c *completed) { c.inactive = 25 * time.Minute }), true},
			{
				"overridden max inactive seeding time reached and paused",
				limits{-1, -1, 40},
				with(done(state, 2), func(c *completed) { c.seedingTime, c.inactiveTime, c.inactive = 20, 10, 15*time.Minute }), true,
			},
			{
				"overridden max inactive seeding time not reached and paused",
				limits{-1, -1, 20},
				with(done(state, 2), func(c *completed) { c.seedingTime, c.inactiveTime, c.inactive = 30, 40, 30*time.Minute }), false,
			},
			{"max seeding time reached but ratio not and paused", limits{2, 20, -1}, with(done(state, 1), func(c *completed) { c.seedingTime = 30 }), true},
			{"max inactive seeding time reached but ratio not and paused", limits{2, -1, 20}, with(done(state, 1), func(c *completed) { c.inactive = 25 * time.Minute }), true},
		}...)
	}
	for _, tt := range tests {
		t.Run(tt.torrent.state+" "+tt.name, func(t *testing.T) {
			each(t, func(t *testing.T, f *fake, category string, _ client.Kind) {
				globalSeedLimits(f, tt.global.ratio, tt.global.seeding, tt.global.inactive)
				tt.torrent.torrent(f, category)
				item := single(t, newClient(t, f, nil))
				if item.CanBeRemoved != tt.removable || item.CanMoveFiles != tt.removable {
					t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = %v", item.CanBeRemoved, item.CanMoveFiles, tt.removable)
				}
			})
		})
	}
	// Without removing completed downloads nothing is removable.
	f := newFake(t, 2)
	globalSeedLimits(f, 1, -1, -1)
	done("pausedUP", 1).torrent(f, "tv")
	items, err := newClient(t, f, nil).Items(context.Background(), client.Options{})
	if err != nil || len(items) != 1 || items[0].CanBeRemoved {
		t.Errorf("Items() = %+v, %v, want one not removable", items, err)
	}
}

// should_not_fetch_details_twice
func TestSeedingTimeFetchedOnce(t *testing.T) {
	for _, state := range []string{"pausedUP", "stoppedUP"} {
		f := newFake(t, 2)
		globalSeedLimits(f, -1, 30, -1)
		c := done(state, 2)
		c.seedingTime = 20
		c.torrent(f, "movies-radarr")
		q := newClient(t, f, nil)
		now := time.Now()
		q.env.Now = func() time.Time { return now }
		if item := single(t, q); item.CanBeRemoved || item.CanMoveFiles {
			t.Errorf("%s: CanBeRemoved, CanMoveFiles = %v, %v, want = false", state, item.CanBeRemoved, item.CanMoveFiles)
		}
		single(t, q)
		if props(f) != 1 {
			t.Errorf("%s: properties fetched %d times, want = 1", state, props(f))
		}
		// Once the limit may have been reached, it is fetched again.
		now = now.Add(6 * time.Minute)
		f.seeding["hash"] = 30 * 60
		if item := single(t, q); !item.CanBeRemoved || props(f) != 2 {
			t.Errorf("%s: CanBeRemoved = %v after %d fetches, want = true after 2", state, item.CanBeRemoved, props(f))
		}
		// The list's seeding time is used where qBittorrent gives it.
		f.torrents[0]["seeding_time"] = 1800
		single(t, q)
		if props(f) != 2 {
			t.Errorf("%s: properties fetched %d times, want = 2", state, props(f))
		}
	}
}

func TestItemCategory(t *testing.T) {
	for _, state := range []string{"pausedUP", "stoppedUP"} {
		for _, field := range []string{"category", "label"} {
			// should_get_category_from_the_category_if_set,
			// should_get_category_from_the_label_if_the_category_is_not_available
			each(t, func(t *testing.T, f *fake, category string, kind client.Kind) {
				globalSeedLimits(f, 1, -1, -1)
				tor := newTorrent("", state, 1, 8640000)
				tor[field], tor["ratio"] = category, 1.0
				f.torrents = []map[string]any{tor}
				if item := single(t, newClient(t, f, nil)); item.Category != category || item.Kind != kind {
					t.Errorf("%s %s: Category, Kind = %q, %v, want = %q, %v", state, field, item.Category, item.Kind, category, kind)
				}
			})
		}
	}
}

func TestItemsKinds(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     map[string]client.Kind
		queries  []string
	}{
		{
			"both", nil,
			map[string]client.Kind{"m": client.Movie, "t": client.Series},
			[]string{"movies-radarr", "tv"},
		},
		{
			"one category",
			map[string]any{"movie_category": "same", "tv_category": "same"},
			map[string]client.Kind{"s": 0},
			[]string{"same"},
		},
		{
			"no series category",
			map[string]any{"tv_category": ""},
			map[string]client.Kind{"m": 0, "t": client.Series, "s": client.Series, "o": client.Series},
			[]string{""},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t, 2)
			for h, c := range map[string]string{"m": "movies-radarr", "t": "tv", "s": "same", "o": ""} {
				tor := newTorrent(c, "downloading", 0.5, 60)
				tor["hash"] = h
				f.torrents = append(f.torrents, tor)
			}
			items, err := newClient(t, f, tt.settings).Items(context.Background(), client.Options{})
			if err != nil {
				t.Fatal(err)
			}
			got := map[string]client.Kind{}
			for _, it := range items {
				got[strings.ToLower(it.ID)] = it.Kind
			}
			if len(got) != len(tt.want) {
				t.Errorf("Items() = %v, want = %v", got, tt.want)
			}
			for h, k := range tt.want {
				if g, ok := got[h]; !ok || g != k {
					t.Errorf("Items()[%s] = %v, %v, want = %v", h, g, ok, k)
				}
			}
			var queries []string
			for _, c := range f.all() {
				if c.Path == "/api/v2/torrents/info" {
					queries = append(queries, c.Query.Get("category"))
				}
			}
			if !slices.Equal(queries, tt.queries) {
				t.Errorf("category queries = %q, want = %q", queries, tt.queries)
			}
		})
	}
}

// should_handle_eta_biginteger
func TestETA(t *testing.T) {
	var tor torrent
	if err := json.Unmarshal([]byte(`{ "eta": 18446744073709335000 }`), &tor); err != nil {
		t.Fatal(err)
	}
	if tor.ETA.String() != "18446744073709335000" || remainingTime(tor.ETA) != nil {
		t.Errorf("ETA = %s, remaining %v, want = 18446744073709335000, nil", tor.ETA, remainingTime(tor.ETA))
	}
	if tor.RatioLimit != -2 || tor.SeedingTimeLimit != -2 || tor.InactiveSeedingTimeLimit != -2 {
		t.Errorf("limits = %v, %v, %v, want = -2", tor.RatioLimit, tor.SeedingTimeLimit, tor.InactiveSeedingTimeLimit)
	}
	for eta, want := range map[string]*time.Duration{"-1": nil, "8640000": nil, "31536001": nil, "60": ptr(time.Minute), "31536000": ptr(365 * 24 * time.Hour)} {
		got := remainingTime(json.Number(eta))
		if (got == nil) != (want == nil) || got != nil && *got != *want {
			t.Errorf("remainingTime(%s) = %v, want = %v", eta, got, want)
		}
	}
}

func ptr[T any](v T) *T { return &v }

// Test_should_force_api_version_check
func TestTestForcesVersionCheck(t *testing.T) {
	f := newFake(t, 1)
	f.version = "0"
	c := newClient(t, f, nil)
	if _, err := c.Items(context.Background(), client.Options{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Items(context.Background(), client.Options{}); err != nil {
		t.Fatal(err)
	}
	if probes(f) != 1 {
		t.Fatalf("probes = %d, want = 1 with the API kept", probes(f))
	}
	if err := c.Test(context.Background()); err == nil || !strings.Contains(err.Error(), "3.2.4") {
		t.Errorf("Test() error = %v, want the version too old", err)
	}
	if probes(f) != 2 {
		t.Errorf("probes = %d, want = 2", probes(f))
	}
}

func TestTest(t *testing.T) {
	tests := []struct {
		name     string
		api      int
		version  string
		settings map[string]any
		setup    func(f *fake)
		// want is in the error; empty for none.
		want    string
		created []string
	}{
		{"ok", 2, "2.0", nil, nil, "", []string{"movies-radarr", "tv"}},
		{
			"imported categories", 2, "2.0",
			map[string]any{"movie_imported_category": "done", "tv_imported_category": "done"},
			func(f *fake) { f.categories["tv"] = map[string]any{"name": "tv"} }, "",
			[]string{"movies-radarr", "done"},
		},
		// A warning, as Radarr's IsWarning failure.
		{"no categories", 2, "2.0", map[string]any{"movie_category": "", "tv_category": ""}, nil, "a category is recommended", nil},
		{"category not created", 2, "2.0", nil, func(f *fake) { f.noCreate = true }, "configuration of category", []string{"movies-radarr"}},
		{"v1 creates on set", 1, "18", nil, nil, "", nil},
		{"v1 too old", 1, "4", nil, nil, "3.2.4", nil},
		{"v1 without categories", 1, "5", nil, nil, "categories are not supported", nil},
		{"v1 without categories, none set", 1, "5", map[string]any{"movie_category": "", "tv_category": ""}, nil, "", nil},
		{
			"removes at ratio", 2, "2.0", nil,
			func(f *fake) { f.prefs["max_ratio_enabled"], f.prefs["max_ratio_act"] = true, ratioActionDeleteFiles }, "Share Ratio Limit", nil,
		},
		{
			"removes at seeding time", 2, "2.0", nil,
			func(f *fake) {
				f.prefs["max_seeding_time_enabled"], f.prefs["max_seeding_time"], f.prefs["max_ratio_act"] = true, 60, ratioActionRemove
			}, "Share Ratio Limit", nil,
		},
		{
			"pauses at ratio", 2, "2.0", nil,
			func(f *fake) { f.prefs["max_ratio_enabled"], f.prefs["max_ratio_act"] = true, 0 }, "",
			[]string{"movies-radarr", "tv"},
		},
		{
			"queueing off", 2, "2.0",
			map[string]any{"older_tv_priority": priorityFirst},
			func(f *fake) { f.prefs["queueing_enabled"] = false }, "queueing not enabled",
			[]string{"movies-radarr", "tv"},
		},
		{"queueing off, last", 2, "2.0", nil, func(f *fake) { f.prefs["queueing_enabled"] = false }, "", []string{"movies-radarr", "tv"}},
		{"wrong password", 2, "2.0", map[string]any{"password": "wrong"}, nil, "authentication failure", nil},
		{"v1 wrong password", 1, "18", map[string]any{"password": "wrong"}, nil, "authentication failure", nil},
		{"torrents fail", 2, "2.0", nil, func(f *fake) { f.status["/api/v2/torrents/info"] = 500 }, "failed to get the list of torrents", []string{"movies-radarr", "tv"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t, tt.api)
			f.version = tt.version
			if tt.setup != nil {
				tt.setup(f)
			}
			err := newClient(t, f, tt.settings).Test(context.Background())
			switch {
			case tt.want == "" && err != nil:
				t.Fatalf("Test() error = %v, want = nil", err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Fatalf("Test() error = %v, want = %q", err, tt.want)
			}
			var warning *client.TestWarning
			if errors.As(err, &warning) != (tt.name == "no categories") {
				t.Errorf("Test() error = %v, want a warning only without categories", err)
			}
			if strings.Contains(tt.name, "password") {
				var auth *client.AuthError
				if !errors.As(err, &auth) {
					t.Errorf("Test() error = %v, want an AuthError", err)
				}
			}
			var created []string
			for _, c := range f.posts() {
				if strings.HasSuffix(c.Path, "Category") {
					created = append(created, c.Form.Get("category"))
				}
			}
			if !slices.Equal(created, tt.created) {
				t.Errorf("created = %q, want = %q", created, tt.created)
			}
		})
	}
}

func TestAuth(t *testing.T) {
	for _, api := range []int{2, 1} {
		f := newFake(t, api)
		c := newClient(t, f, nil)
		for range 2 {
			if _, err := c.Items(context.Background(), client.Options{}); err != nil {
				t.Fatalf("v%d: Items() error = %v", api, err)
			}
		}
		if logins(f) != 1 {
			t.Errorf("v%d: logins = %d, want = 1 with the cookie kept", api, logins(f))
		}
		f.expire = true
		if _, err := c.Items(context.Background(), client.Options{}); err != nil {
			t.Fatalf("v%d: Items() after the session expired error = %v", api, err)
		}
		if logins(f) != 2 {
			t.Errorf("v%d: logins = %d, want = 2 after the session expired", api, logins(f))
		}
		var auth *client.AuthError
		if _, err := newClient(t, f, map[string]any{"password": "wrong"}).Items(context.Background(), client.Options{}); !errors.As(err, &auth) {
			t.Errorf("v%d: Items() with a wrong password error = %v, want an AuthError", api, err)
		}
	}
	// API keys sign in to v2 without a session.
	f := newFake(t, 2)
	f.apiKey = "key"
	if _, err := newClient(t, f, map[string]any{"api_key": "key", "username": "", "password": ""}).Items(context.Background(), client.Options{}); err != nil {
		t.Fatalf("Items() with an API key error = %v", err)
	}
	if logins(f) != 0 {
		t.Errorf("logins = %d, want = 0 with an API key", logins(f))
	}
	var auth *client.AuthError
	_, err := newClient(t, f, map[string]any{"api_key": "bad", "username": "", "password": ""}).Items(context.Background(), client.Options{})
	if !errors.As(err, &auth) {
		t.Errorf("Items() with a wrong API key error = %v, want an AuthError", err)
	}
	// Neither API answering is no qBittorrent.
	f = newFake(t, 3)
	if _, err := newClient(t, f, nil).Items(context.Background(), client.Options{}); err == nil || !strings.Contains(err.Error(), "API version") {
		t.Errorf("Items() error = %v, want the API version undetermined", err)
	}
}

func TestRemoveAndMarkImported(t *testing.T) {
	tests := []struct {
		api        int
		deleteData bool
		remove     string
		marks      []string
		status     map[string]int
	}{
		{2, true, "/api/v2/torrents/delete", []string{"/api/v2/torrents/setCategory"}, nil},
		{2, false, "/api/v2/torrents/delete", []string{"/api/v2/torrents/setCategory"}, nil},
		{1, true, "/command/deletePerm", []string{"/command/setCategory"}, nil},
		{1, false, "/command/delete", []string{"/command/setCategory", "/command/setLabel"}, map[string]int{"/command/setCategory": 404}},
	}
	for _, tt := range tests {
		for _, k := range kinds {
			f := newFake(t, tt.api)
			f.status = tt.status
			c := newClient(t, f, map[string]any{"movie_imported_category": "movies-done", "tv_imported_category": "tv-done"})
			item := client.Item{ID: "ABCDEF", Title: title, Kind: k.kind}
			if err := c.MarkImported(context.Background(), item); err != nil {
				t.Fatalf("MarkImported() error = %v", err)
			}
			if err := c.Remove(context.Background(), item, tt.deleteData); err != nil {
				t.Fatalf("Remove() error = %v", err)
			}
			posts := f.posts()
			want := append(slices.Clone(tt.marks), tt.remove)
			if got := f.paths(); !slices.Equal(got, want) {
				t.Fatalf("v%d %s: calls = %v, want = %v", tt.api, k.category, got, want)
			}
			wantCategory := map[client.Kind]string{client.Movie: "movies-done", client.Series: "tv-done"}[k.kind]
			if got := posts[0].Form; got.Get("hashes") != "abcdef" || got.Get("category") != wantCategory {
				t.Errorf("v%d: setCategory = %v, want the lower-case hash in %s", tt.api, got, wantCategory)
			}
			last := posts[len(posts)-1].Form
			if last.Get("hashes") != "abcdef" || tt.api == 2 && (last.Get("deleteFiles") == "true") != tt.deleteData {
				t.Errorf("v%d: delete = %v, want the lower-case hash, deleting data = %v", tt.api, last, tt.deleteData)
			}
		}
	}
	// Without a post-import category, or the same, nothing moves; failing to
	// move is only logged.
	f := newFake(t, 2)
	for _, s := range []map[string]any{nil, {"movie_imported_category": "movies-radarr"}} {
		if err := newClient(t, f, s).MarkImported(context.Background(), client.Item{ID: "A", Kind: client.Movie}); err != nil {
			t.Fatal(err)
		}
	}
	f.status["/api/v2/torrents/setCategory"] = 409
	c := newClient(t, f, map[string]any{"movie_imported_category": "done"})
	if err := c.MarkImported(context.Background(), client.Item{ID: "A"}); err != nil {
		t.Errorf("MarkImported() error = %v, want it logged", err)
	}
	if got := f.paths(); len(got) != 1 {
		t.Errorf("calls = %v, want one setCategory", got)
	}
	f.torrents = []map[string]any{newTorrent("movies-radarr", "downloading", 0.5, 60)}
	if item := single(t, c); !item.HasPostImportCategory {
		t.Errorf("HasPostImportCategory = false, want = true")
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"defaults", `{}`, true},
		{"nested category", `{"movie_category":"movies/radarr"}`, true},
		{"leading slash", `{"movie_category":"/radarr"}`, false},
		{"trailing slash", `{"tv_imported_category":"tv/"}`, false},
		{"double slash", `{"tv_category":"tv//sonarr"}`, false},
		{"backslash", `{"movie_imported_category":"a\\b"}`, false},
		{"api key", `{"api_key":"key"}`, true},
		{"api key and username", `{"api_key":"key","username":"admin"}`, false},
		{"api key and password", `{"api_key":"key","password":"pass"}`, false},
		{"url base", `{"url_base":"/qbit"}`, true},
		{"url base with host", `{"url_base":"http://host"}`, false},
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

func logins(f *fake) int { n, _, _ := f.stats(); return n }
func probes(f *fake) int { _, n, _ := f.stats(); return n }
func props(f *fake) int  { _, _, n := f.stats(); return n }

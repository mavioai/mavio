package nzbvortex

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const (
	movieTitle  = "Droned.1998.1080p.WEB-DL-DRONE"
	seriesTitle = "Droned.S01E01.Pilot.1080p.WEB-DL-DRONE"
	apiKey      = "1234-ABCD"
)

// fake is an NZBVortex API.
type fake struct {
	t        *testing.T
	mu       sync.Mutex
	session  string
	logins   int
	apiLevel string
	groups   []string
	nzbs     []map[string]any
	files    []map[string]any
	// addUUID is what nzb/add returns.
	addUUID string
	// requests are the paths with queries of the requests made.
	requests []*http.Request
	added    []added
	// expire makes the next authenticated request find the session
	// expired.
	expire bool
}

type added struct {
	Query    map[string][]string
	FileName string
	Type     string
	Data     string
}

func newFake(t *testing.T) *fake {
	return &fake{t: t, apiLevel: "2.4", groups: []string{"movie", "tv"}, addUUID: "uuid"}
}

func (f *fake) handler() http.Handler {
	mux := http.NewServeMux()
	write := func(w http.ResponseWriter, v any) { _ = json.NewEncoder(w).Encode(v) }
	mux.HandleFunc("GET /api/auth/nonce", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"authNonce": "nonce"})
	})
	mux.HandleFunc("GET /api/auth/login", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		sum := sha256.Sum256([]byte(q.Get("nonce") + ":" + q.Get("cnonce") + ":" + apiKey))
		if q.Get("nonce") != "nonce" || q.Get("cnonce") == "" || q.Get("hash") != base64.StdEncoding.EncodeToString(sum[:]) {
			write(w, map[string]any{"result": "ok", "loginResult": "failed"})
			return
		}
		f.logins++
		f.session = "session" + strconv.Itoa(f.logins)
		write(w, map[string]any{"result": "ok", "loginResult": "successful", "sessionID": f.session})
	})
	mux.HandleFunc("GET /api/app/appversion", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"result": "ok", "version": "3.0.1"})
	})
	mux.HandleFunc("GET /api/app/apilevel", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"result": "ok", "apilevel": f.apiLevel})
	})
	auth := func(h func(w http.ResponseWriter, r *http.Request)) func(w http.ResponseWriter, r *http.Request) {
		return func(w http.ResponseWriter, r *http.Request) {
			if f.expire || r.URL.Query().Get("sessionid") != f.session {
				f.expire = false
				write(w, map[string]any{"result": "notLoggedIn"})
				return
			}
			h(w, r)
		}
	}
	mux.HandleFunc("GET /api/group", auth(func(w http.ResponseWriter, _ *http.Request) {
		var groups []map[string]any
		for _, g := range f.groups {
			groups = append(groups, map[string]any{"groupname": g})
		}
		write(w, map[string]any{"result": "ok", "groups": groups})
	}))
	mux.HandleFunc("GET /api/nzb", auth(func(w http.ResponseWriter, r *http.Request) {
		nzbs := []map[string]any{}
		for _, n := range f.nzbs {
			if g := r.URL.Query().Get("groupName"); g == "" || n["groupName"] == g {
				nzbs = append(nzbs, n)
			}
		}
		write(w, map[string]any{"result": "ok", "nzbs": nzbs})
	}))
	mux.HandleFunc("GET /api/nzb/{id}/{action}", auth(func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"result": "ok"})
	}))
	mux.HandleFunc("GET /api/file/{id}", auth(func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"result": "ok", "files": f.files})
	}))
	mux.HandleFunc("POST /api/nzb/add", auth(func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("name")
		if err != nil {
			f.t.Errorf("nzb/add form: %v", err)
			return
		}
		data, _ := io.ReadAll(file)
		f.added = append(f.added, added{
			Query: r.URL.Query(), FileName: header.Filename, Type: header.Header.Get("Content-Type"), Data: string(data),
		})
		write(w, map[string]any{"result": "ok", "add_uuid": f.addUUID})
	}))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests = append(f.requests, r)
		mux.ServeHTTP(w, r)
	})
}

// paths returns the paths of the requests made to authenticated
// resources.
func (f *fake) paths() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []string
	for _, r := range f.requests {
		if p := r.URL.Path; p != "/api/auth/nonce" && p != "/api/auth/login" {
			out = append(out, p)
		}
	}
	return out
}

// newClient returns an NZBVortex client talking to f over HTTPS, with
// settings over the fixture's.
func newClient(t *testing.T, f *fake, settings map[string]any) client.Client {
	t.Helper()
	srv := httptest.NewTLSServer(f.handler())
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{
		"host": host, "port": p, "api_key": apiKey, "movie_category": "movie", "tv_category": "tv",
		"recent_movie_priority": 1, "recent_tv_priority": 1,
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

// The downloads of Radarr's and Sonarr's NzbVortexFixture.
func queued(group, title string) map[string]any {
	return map[string]any{"id": 7, "downloadedSize": 1000, "totalDownloadSize": 10, "groupName": group, "uiTitle": title}
}

func failed(group, title string) map[string]any {
	return map[string]any{
		"downloadedSize": 1000, "totalDownloadSize": 1000, "groupName": group, "uiTitle": title,
		"destinationPath": "somedirectory", "state": stateUncompressFailed,
	}
}

func completed(group, title string) map[string]any {
	return map[string]any{
		"downloadedSize": 1000, "totalDownloadSize": 1000, "groupName": group, "uiTitle": title,
		"destinationPath": "/remote/mount/tv/" + title, "state": stateDone,
	}
}

var allOptions = client.Options{RemoveCompletedDownloads: true, RemoveFailedDownloads: true}

func items(t *testing.T, c client.Client) []client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), allOptions)
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	return items
}

// fixtures are Radarr's fixture, of movies, and Sonarr's, of series.
var fixtures = []struct {
	name, group, title string
	kind               client.Kind
}{
	{"radarr", "movie", movieTitle, client.Movie},
	{"sonarr", "tv", seriesTitle, client.Series},
}

func TestItems(t *testing.T) {
	tests := []struct {
		name string
		nzb  func(group, title string) map[string]any
		// change changes the fixture's download.
		change func(n map[string]any)
		want   client.Status
	}{
		// queued_item_should_have_required_properties
		{name: "queued", nzb: queued, want: client.Queued},
		// paused_item_should_have_required_properties
		{name: "paused", nzb: queued, change: func(n map[string]any) { n["isPaused"] = true }, want: client.Paused},
		// downloading_item_should_have_required_properties
		{name: "downloading", nzb: queued, change: func(n map[string]any) { n["state"] = stateDownloading }, want: client.Downloading},
		// completed_download_should_have_required_properties
		{name: "completed", nzb: completed, want: client.Completed},
		// failed_item_should_have_required_properties; should_report_UncompressFailed_as_failed
		{name: "uncompress failed", nzb: failed, want: client.Failed},
		// should_report_CheckFailedDataCorrupt_as_failed
		{name: "check failed", nzb: failed, change: func(n map[string]any) { n["state"] = stateCheckFailedDataCorrupt }, want: client.Failed},
		// should_report_BadlyEncoded_as_failed
		{name: "badly encoded", nzb: failed, change: func(n map[string]any) { n["state"] = stateBadlyEncoded }, want: client.Failed},
		{name: "password", nzb: queued, change: func(n map[string]any) { n["state"] = statePasswordRequest }, want: client.Downloading},
	}
	for _, fx := range fixtures {
		for _, tt := range tests {
			t.Run(fx.name+"/"+tt.name, func(t *testing.T) {
				f := newFake(t)
				n := tt.nzb(fx.group, fx.title)
				if tt.change != nil {
					tt.change(n)
				}
				f.nzbs = []map[string]any{n}
				got := items(t, newClient(t, f, nil))
				if len(got) != 1 {
					t.Fatalf("Items() = %d items, want 1", len(got))
				}
				item := got[0]
				if item.Status != tt.want {
					t.Errorf("Status = %v, want = %v", item.Status, tt.want)
				}
				if item.ID == "" || item.Title != fx.title || item.Kind != fx.kind || item.Category != fx.group {
					t.Errorf("ID, Title, Kind, Category = %q, %q, %v, %q, want = non-empty, %q, %v, %q",
						item.ID, item.Title, item.Kind, item.Category, fx.title, fx.kind, fx.group)
				}
				if !item.CanBeRemoved || !item.CanMoveFiles {
					t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = true, true", item.CanBeRemoved, item.CanMoveFiles)
				}
				if item.Encrypted != (tt.name == "password") {
					t.Errorf("Encrypted = %v, want = %v", item.Encrypted, tt.name == "password")
				}
				switch tt.want {
				case client.Queued, client.Paused, client.Downloading:
					if item.RemainingSize == 0 || item.RemainingTime != nil {
						t.Errorf("RemainingSize, RemainingTime = %d, %v, want = non-zero, nil", item.RemainingSize, item.RemainingTime)
					}
				case client.Completed:
					if item.RemainingSize != 0 || item.RemainingTime == nil || *item.RemainingTime != 0 {
						t.Errorf("RemainingSize, RemainingTime = %d, %v, want = 0, 0", item.RemainingSize, item.RemainingTime)
					}
				}
			})
		}
	}
}

// GetItems_should_return_no_items_when_queue_is_empty
func TestItemsEmpty(t *testing.T) {
	if got := items(t, newClient(t, newFake(t), nil)); len(got) != 0 {
		t.Errorf("Items() = %d items, want 0", len(got))
	}
}

func TestItemsGroups(t *testing.T) {
	f := newFake(t)
	f.nzbs = []map[string]any{
		completed("movie", movieTitle), completed("tv", seriesTitle),
		// GetItems_should_ignore_downloads_from_other_categories
		completed("mycat", "Other"),
	}
	f.nzbs[0]["addUUID"] = "uuid"
	tests := []struct {
		name     string
		settings map[string]any
		ids      []string
		kinds    []client.Kind
		// queries are the queue's requests' groups.
		queries []string
	}{
		{"both", nil, []string{"uuid", "0"}, []client.Kind{client.Movie, client.Series}, []string{"movie", "tv"}},
		{"same", map[string]any{"tv_category": "movie"}, []string{"uuid"}, []client.Kind{0}, []string{"movie"}},
		{"no group", map[string]any{"tv_category": ""}, []string{"uuid", "0", "0"}, []client.Kind{0, client.Series, client.Series}, []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f.requests = nil
			var ids []string
			var kinds []client.Kind
			for _, item := range items(t, newClient(t, f, tt.settings)) {
				ids, kinds = append(ids, item.ID), append(kinds, item.Kind)
			}
			if !slices.Equal(ids, tt.ids) || !slices.Equal(kinds, tt.kinds) {
				t.Errorf("IDs, Kinds = %v, %v, want = %v, %v", ids, kinds, tt.ids, tt.kinds)
			}
			var queries []string
			for _, r := range f.requests {
				if r.URL.Path == "/api/nzb" {
					queries = append(queries, r.URL.Query().Get("groupName"))
					if got := r.URL.Query().Get("limitDone"); got != "30" {
						t.Errorf("limitDone = %q, want = 30", got)
					}
				}
			}
			if !slices.Equal(queries, tt.queries) {
				t.Errorf("groups = %v, want = %v", queries, tt.queries)
			}
		})
	}
}

func TestOutputPath(t *testing.T) {
	tests := []struct {
		name  string
		state int
		dest  string
		files []string
		want  string
		// warning is set for a download whose files are not in a job
		// folder.
		warning bool
	}{
		// should_remap_storage_if_mounted reads the path as NZBVortex
		// reports it, as Mavio's server maps it.
		{"job folder", stateDone, "/remote/mount/tv/" + movieTitle, nil, "/remote/mount/tv/" + movieTitle, false},
		// should_get_files_if_completed_download_is_not_in_a_job_folder
		{"single file", stateDone, "/remote/mount/", []string{movieTitle + ".mkv"}, "/remote/mount/" + movieTitle + ".mkv", false},
		{"windows", stateDone, `O:\mymount`, []string{movieTitle + ".mkv"}, `O:\mymount\` + movieTitle + ".mkv", false},
		// should_be_warning_if_more_than_one_file_is_not_in_a_job_folder
		{"several files", stateDone, "/remote/mount/", []string{movieTitle + ".mkv", movieTitle + ".nfo"}, "/remote/mount/" + movieTitle + ".mkv", true},
		{"not done", stateDownloading, "/remote/mount/", []string{movieTitle + ".mkv"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			n := completed("movie", movieTitle)
			n["state"], n["destinationPath"] = tt.state, tt.dest
			f.nzbs = []map[string]any{n}
			for _, name := range tt.files {
				f.files = append(f.files, map[string]any{"fileName": name})
			}
			got := items(t, newClient(t, f, nil))[0]
			if got.OutputPath != tt.want {
				t.Errorf("OutputPath = %q, want = %q", got.OutputPath, tt.want)
			}
			if (got.Status == client.Warning) != tt.warning || tt.warning && got.Message == "" {
				t.Errorf("Status, Message = %v, %q, want warning = %v", got.Status, got.Message, tt.warning)
			}
		})
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name     string
		kind     client.Kind
		recent   bool
		settings map[string]any
		// query is the add's priority and group.
		priority, group string
	}{
		// Download_should_return_unique_id
		{"radarr", client.Movie, true, nil, "1", "movie"},
		{"sonarr", client.Series, false, map[string]any{"older_tv_priority": -1}, "-1", "tv"},
		{"no group", client.Series, false, map[string]any{"tv_category": ""}, "0", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			c := newClient(t, f, tt.settings)
			id, err := c.Add(context.Background(), &client.Release{Title: seriesTitle + ": x", Kind: tt.kind, Recent: tt.recent, Data: []byte("nzb")})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if id != "uuid" {
				t.Errorf("Add() = %q, want = uuid", id)
			}
			if len(f.added) != 1 {
				t.Fatalf("added = %d, want 1", len(f.added))
			}
			a := f.added[0]
			q := a.Query
			if a.FileName != seriesTitle+" - x.nzb" || a.Type != "application/x-nzb" || a.Data != "nzb" {
				t.Errorf("file = %q, %q, %q, want = %q, application/x-nzb, nzb", a.FileName, a.Type, a.Data, seriesTitle+" - x.nzb")
			}
			if got := q["priority"]; !slices.Equal(got, []string{tt.priority}) {
				t.Errorf("priority = %v, want = %v", got, tt.priority)
			}
			if got := q["groupname"]; tt.group == "" && got != nil || tt.group != "" && !slices.Equal(got, []string{tt.group}) {
				t.Errorf("groupname = %v, want = %q", got, tt.group)
			}
		})
	}
}

// Download_should_throw_if_failed
func TestAddFailed(t *testing.T) {
	f := newFake(t)
	f.addUUID = ""
	if _, err := newClient(t, f, nil).Add(context.Background(), &client.Release{Title: movieTitle, Kind: client.Movie, Data: []byte("nzb")}); err == nil {
		t.Errorf("Add() error = nil, want failed")
	}
}

func TestSession(t *testing.T) {
	f := newFake(t)
	c := newClient(t, f, nil)
	items(t, c)
	items(t, c)
	if f.logins != 1 {
		t.Errorf("logins = %d, want = 1", f.logins)
	}
	f.expire = true
	items(t, c)
	if f.logins != 2 {
		t.Errorf("logins = %d, want = 2", f.logins)
	}

	err := newClient(t, f, map[string]any{"api_key": "wrong"}).Test(context.Background())
	if _, ok := errors.AsType[*client.AuthError](err); !ok {
		t.Errorf("Test() error = %v, want an AuthError", err)
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name       string
		id         string
		deleteData bool
		want       string
	}{
		{"by ID", "12", false, "/api/nzb/12/cancel"},
		{"by ID with data", "12", true, "/api/nzb/12/cancelDelete"},
		{"by UUID", "uuid", true, "/api/nzb/7/cancelDelete"},
		{"unknown", "other", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			n := queued("movie", movieTitle)
			n["addUUID"] = "uuid"
			f.nzbs = []map[string]any{n}
			if err := newClient(t, f, nil).Remove(context.Background(), client.Item{ID: tt.id}, tt.deleteData); err != nil {
				t.Fatalf("Remove() error = %v", err)
			}
			var got string
			for _, p := range f.paths() {
				if p != "/api/nzb" {
					got = p
				}
			}
			if got != tt.want {
				t.Errorf("request = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestInfo(t *testing.T) {
	info, err := newClient(t, newFake(t), nil).Info(context.Background())
	if err != nil || !info.IsLocalhost || info.OutputRootFolders != nil {
		t.Errorf("Info() = %+v, %v, want localhost without folders", info, err)
	}
}

func TestTest(t *testing.T) {
	tests := []struct {
		name     string
		apiLevel string
		groups   []string
		settings map[string]any
		ok       bool
	}{
		// should_test_api_version
		{name: "api 1.0", apiLevel: "1.0"},
		{name: "api 2.2", apiLevel: "2.2"},
		{name: "api 2.3", apiLevel: "2.3", ok: true},
		{name: "api 2.4", apiLevel: "2.4", ok: true},
		{name: "api 3.0", apiLevel: "3.0", ok: true},
		{name: "api unparsable", apiLevel: "3"},
		{name: "group missing", apiLevel: "2.4", groups: []string{"movie"}},
		{name: "group empty", apiLevel: "2.4", groups: []string{"movie"}, settings: map[string]any{"tv_category": ""}, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.apiLevel = tt.apiLevel
			if tt.groups != nil {
				f.groups = tt.groups
			}
			err := newClient(t, f, tt.settings).Test(context.Background())
			if (err == nil) != tt.ok {
				t.Errorf("Test() error = %v, want ok = %v", err, tt.ok)
			}
		})
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"api key", `{"api_key": "key"}`, true},
		{"no api key", `{}`, false},
		{"no host", `{"api_key": "key", "host": ""}`, false},
		{"bad port", `{"api_key": "key", "port": 0}`, false},
		{"unknown", `{"api_key": "key", "use_ssl": true}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
			if (err == nil) != tt.ok {
				t.Errorf("New() error = %v, want ok = %v", err, tt.ok)
			}
		})
	}
}

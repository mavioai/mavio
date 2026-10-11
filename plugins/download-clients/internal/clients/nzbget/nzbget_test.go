package nzbget

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
	"os"
	"path/filepath"
	"regexp"
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
)

// fake is an NZBGet JSON-RPC server.
type fake struct {
	t       *testing.T
	mu      sync.Mutex
	version string
	status  map[string]any
	queue   []map[string]any
	history []map[string]any
	config  map[string]string
	// appendResult is what append returns; editResult what editqueue
	// does.
	appendResult any
	editResult   bool
	// rpcError makes every call answer with an error.
	rpcError bool
	calls    []rpcCall
}

type rpcCall struct {
	Method string `json:"method"`
	Params []any  `json:"params"`
}

func newFake(t *testing.T) *fake {
	return &fake{
		t: t, version: "14.0", status: map[string]any{"DownloadRate": 7000000},
		config: map[string]string{
			"Category1.Name": "movie", "Category1.DestDir": "/remote/mount/movie",
			"Category2.Name": "tv", "Category2.DestDir": "/remote/mount/tv",
		},
		appendResult: 1, editResult: true,
	}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/jsonrpc" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	if user, pass, _ := r.BasicAuth(); user != "admin" || pass != "pass" {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	var c rpcCall
	body, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(body, &c); err != nil {
		f.t.Errorf("request body: %v", err)
	}
	f.calls = append(f.calls, c)
	if f.rpcError {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version": "1.1", "result": nil,
			"error": map[string]any{"name": "JSONRPCError", "code": 3, "message": "Invalid parameter"},
		})
		return
	}
	var result any
	switch c.Method {
	case "version":
		result = f.version
	case "status":
		result = f.status
	case "listgroups":
		result = orEmpty(f.queue)
	case "history":
		result = orEmpty(f.history)
	case "config":
		var items []map[string]string
		for k, v := range f.config {
			items = append(items, map[string]string{"Name": k, "Value": v})
		}
		result = items
	case "append":
		result = f.appendResult
	case "editqueue":
		result = f.editResult
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"version": "1.1", "result": result})
}

func orEmpty(m []map[string]any) []map[string]any {
	if m == nil {
		return []map[string]any{}
	}
	return m
}

// method returns the calls of a method.
func (f *fake) method(name string) []rpcCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []rpcCall
	for _, c := range f.calls {
		if c.Method == name {
			out = append(out, c)
		}
	}
	return out
}

// newClient returns an NZBGet client talking to f, with settings over the
// fixture's.
func newClient(t *testing.T, f *fake, settings map[string]any) client.Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{
		"host": host, "port": p, "username": "admin", "password": "pass",
		"movie_category": "movie", "tv_category": "tv", "recent_movie_priority": 50, "recent_tv_priority": 50,
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

var drone = []map[string]any{{"Name": "drone", "Value": "id"}}

// The downloads of Radarr's and Sonarr's NzbgetFixture.
func queued(category, title string) map[string]any {
	return map[string]any{
		"FileSizeLo": 1000, "RemainingSizeLo": 10, "Category": category, "NZBName": title, "Parameters": drone,
	}
}

func failed(category, title string) map[string]any {
	return map[string]any{
		"FileSizeLo": 1000, "Category": category, "Name": title, "DestDir": "somedirectory", "Parameters": drone,
		"ParStatus": "Some Error", "UnpackStatus": "NONE", "MoveStatus": "NONE", "ScriptStatus": "NONE",
		"DeleteStatus": "NONE", "MarkStatus": "NONE",
	}
}

func completed(category, title string) map[string]any {
	return map[string]any{
		"FileSizeLo": 1000, "Category": category, "Name": title, "DestDir": "/remote/mount/tv/" + title,
		"Parameters": drone, "ParStatus": "SUCCESS", "UnpackStatus": "NONE", "MoveStatus": "SUCCESS",
		"ScriptStatus": "NONE", "DeleteStatus": "NONE", "MarkStatus": "NONE",
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

func single(t *testing.T, c client.Client) client.Item {
	t.Helper()
	got := items(t, c)
	if len(got) != 1 {
		t.Fatalf("Items() = %d items, want 1", len(got))
	}
	return got[0]
}

// fixtures are Radarr's fixture, of movies, and Sonarr's, of series.
var fixtures = []struct {
	name, category, title string
	kind                  client.Kind
}{
	{"radarr", "movie", movieTitle, client.Movie},
	{"sonarr", "tv", seriesTitle, client.Series},
}

func TestItems(t *testing.T) {
	tests := []struct {
		name string
		// queue and history are changed from the fixture's downloads.
		queue   func(q map[string]any)
		history func(category, title string) map[string]any
		change  func(h map[string]any)
		want    client.Status
		// none is set when no download is to be listed.
		none bool
	}{
		// queued_item_should_have_required_properties
		{name: "queued", queue: func(q map[string]any) { q["ActiveDownloads"] = 0 }, want: client.Queued},
		// paused_item_should_have_required_properties
		{name: "paused", queue: func(q map[string]any) { q["PausedSizeLo"] = q["RemainingSizeLo"] }, want: client.Paused},
		// downloading_item_should_have_required_properties
		{name: "downloading", queue: func(q map[string]any) { q["ActiveDownloads"] = 1 }, want: client.Downloading},
		// post_processing_item_should_have_required_properties
		{name: "post-processing", queue: func(q map[string]any) { q["ActiveDownloads"], q["RemainingSizeLo"] = 1, 0 }, want: client.Downloading},
		// completed_download_should_have_required_properties
		{name: "completed", history: completed, want: client.Completed},
		// failed_item_should_have_required_properties
		{name: "failed", history: failed, want: client.Failed},
		// should_report_deletestatus_health_as_failed
		{name: "delete health", history: completed, change: func(h map[string]any) { h["DeleteStatus"] = "HEALTH" }, want: client.Failed},
		// should_report_deletestatus_dupe_as_failed
		{name: "delete dupe", history: completed, change: func(h map[string]any) { h["DeleteStatus"] = "DUPE" }, want: client.Failed},
		// should_report_deletestatus_copy_as_failed
		{name: "delete copy", history: completed, change: func(h map[string]any) { h["DeleteStatus"] = "COPY" }, want: client.Failed},
		{name: "delete other", history: completed, change: func(h map[string]any) { h["DeleteStatus"] = "GOOD" }, want: client.Warning},
		// should_report_unpackstatus_freespace_as_warning
		{name: "unpack space", history: completed, change: func(h map[string]any) { h["UnpackStatus"] = "SPACE" }, want: client.Warning},
		{name: "unpack failure", history: completed, change: func(h map[string]any) { h["UnpackStatus"] = "FAILURE" }, want: client.Failed},
		// should_report_movestatus_failure_as_warning
		{name: "move failure", history: completed, change: func(h map[string]any) { h["MoveStatus"] = "FAILURE" }, want: client.Warning},
		// should_report_scriptstatus_failure_as_failed
		{name: "script failure", history: completed, change: func(h map[string]any) { h["ScriptStatus"] = "FAILURE" }, want: client.Failed},
		// should_report_deletestatus_manual_with_markstatus_bad_as_failed
		{name: "manual bad", history: completed, change: func(h map[string]any) { h["DeleteStatus"], h["MarkStatus"] = "MANUAL", "BAD" }, want: client.Failed},
		// should_ignore_deletestatus_manual_without_markstatus
		{name: "manual", history: completed, change: func(h map[string]any) { h["DeleteStatus"] = "MANUAL" }, none: true},
		// GetItems_should_ignore_downloads_from_other_categories
		{name: "other category", history: completed, change: func(h map[string]any) { h["Category"] = "mycat" }, none: true},
	}
	for _, fx := range fixtures {
		for _, tt := range tests {
			t.Run(fx.name+"/"+tt.name, func(t *testing.T) {
				f := newFake(t)
				if tt.queue != nil {
					q := queued(fx.category, fx.title)
					tt.queue(q)
					f.queue = []map[string]any{q}
				} else {
					h := tt.history(fx.category, fx.title)
					if tt.change != nil {
						tt.change(h)
					}
					f.history = []map[string]any{h}
				}
				got := items(t, newClient(t, f, nil))
				if tt.none {
					if len(got) != 0 {
						t.Fatalf("Items() = %d items, want 0", len(got))
					}
					return
				}
				if len(got) != 1 {
					t.Fatalf("Items() = %d items, want 1", len(got))
				}
				item := got[0]
				if item.Status != tt.want {
					t.Errorf("Status = %v, want = %v", item.Status, tt.want)
				}
				if item.ID != "id" || item.Title != fx.title || item.Kind != fx.kind || item.Category != fx.category {
					t.Errorf("ID, Title, Kind, Category = %q, %q, %v, %q, want = %q, %q, %v, %q",
						item.ID, item.Title, item.Kind, item.Category, "id", fx.title, fx.kind, fx.category)
				}
				if !item.CanBeRemoved || !item.CanMoveFiles {
					t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = true, true", item.CanBeRemoved, item.CanMoveFiles)
				}
				switch tt.want {
				case client.Queued, client.Paused:
					if item.RemainingSize == 0 {
						t.Errorf("RemainingSize = 0, want non-zero")
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

func TestItemsQueue(t *testing.T) {
	f := newFake(t)
	f.status = map[string]any{"DownloadRate": 100}
	first := queued("movie", movieTitle)
	first["ActiveDownloads"], first["RemainingSizeLo"], first["RemainingSizeHi"] = 1, 1000, 1
	second := queued("other", "Other")
	second["RemainingSizeLo"], second["PausedSizeLo"] = 2000, 1000
	third := queued("tv", seriesTitle)
	third["Parameters"], third["NZBID"] = []map[string]any{}, 7
	f.queue = []map[string]any{first, second, third}
	got := items(t, newClient(t, f, nil))
	if len(got) != 2 {
		t.Fatalf("Items() = %d items, want 2", len(got))
	}
	// The remaining time counts the downloads before, of any category.
	wantSize := int64(1)<<32 | 1000
	if got[0].RemainingSize != wantSize || got[0].TotalSize != 1000 || got[0].RemainingTime == nil ||
		*got[0].RemainingTime != time.Duration(wantSize/100)*time.Second {
		t.Errorf("RemainingSize, TotalSize, RemainingTime = %d, %d, %v, want = %d, 1000, %v",
			got[0].RemainingSize, got[0].TotalSize, got[0].RemainingTime, wantSize, time.Duration(wantSize/100)*time.Second)
	}
	if want := time.Duration((wantSize+1000+10)/100) * time.Second; got[1].RemainingTime == nil || *got[1].RemainingTime != want {
		t.Errorf("RemainingTime = %v, want = %v", got[1].RemainingTime, want)
	}
	if got[1].ID != "7" || got[1].Status != client.Queued {
		t.Errorf("ID, Status = %q, %v, want = %q, %v", got[1].ID, got[1].Status, "7", client.Queued)
	}

	f.status = map[string]any{"DownloadRate": 100, "DownloadPaused": true}
	for _, item := range items(t, newClient(t, f, nil)) {
		if item.Status != client.Paused || item.RemainingTime != nil {
			t.Errorf("Status, RemainingTime = %v, %v, want = %v, nil", item.Status, item.RemainingTime, client.Paused)
		}
	}
}

func TestItemsKinds(t *testing.T) {
	f := newFake(t)
	f.history = []map[string]any{completed("movie", movieTitle), completed("tv", seriesTitle), completed("", "None")}
	tests := []struct {
		name     string
		settings map[string]any
		want     []client.Kind
	}{
		{"both", nil, []client.Kind{client.Movie, client.Series}},
		{"same", map[string]any{"tv_category": "movie"}, []client.Kind{0}},
		{"empty", map[string]any{"movie_category": ""}, []client.Kind{client.Series, client.Movie}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []client.Kind
			for _, item := range items(t, newClient(t, f, tt.settings)) {
				got = append(got, item.Kind)
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("Kinds = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestItemsRemovable(t *testing.T) {
	f := newFake(t)
	f.history = []map[string]any{completed("movie", movieTitle), failed("movie", movieTitle)}
	c := newClient(t, f, nil)
	got, err := c.Items(context.Background(), client.Options{RemoveFailedDownloads: true})
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	if got[0].CanBeRemoved || !got[1].CanBeRemoved {
		t.Errorf("CanBeRemoved = %v, %v, want = false, true", got[0].CanBeRemoved, got[1].CanBeRemoved)
	}
}

func TestOutputPath(t *testing.T) {
	tests := []struct {
		name     string
		finalDir any
		want     string
	}{
		// should_use_dest_dir_if_final_dir_is_null; should_remap_storage_if_mounted
		// reads the path as NZBGet reports it, as Mavio's server maps it.
		{"final dir null", nil, "/remote/mount/tv/" + movieTitle},
		// should_use_dest_dir_if_final_dir_is_not_set
		{"final dir empty", "", "/remote/mount/tv/" + movieTitle},
		// should_use_final_dir_when_set_instead_of_dest_dir
		{"final dir", "/remote/mount/tv2/" + seriesTitle, "/remote/mount/tv2/" + seriesTitle},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			h := completed("movie", movieTitle)
			h["FinalDir"] = tt.finalDir
			f.history = []map[string]any{h}
			if got := single(t, newClient(t, f, nil)).OutputPath; got != tt.want {
				t.Errorf("OutputPath = %q, want = %q", got, tt.want)
			}
		})
	}
}

func TestItemsHistoryLimit(t *testing.T) {
	f := newFake(t)
	for range historyLimit + 5 {
		f.history = append(f.history, completed("movie", movieTitle))
	}
	if got := items(t, newClient(t, f, nil)); len(got) != historyLimit {
		t.Errorf("Items() = %d items, want %d", len(got), historyLimit)
	}
}

func TestAdd(t *testing.T) {
	nzb := []byte("<nzb></nzb>")
	tests := []struct {
		name    string
		kind    client.Kind
		recent  bool
		paused  bool
		version string
		// category and priority are those appended with.
		category string
		priority float64
	}{
		// Download_should_return_unique_id
		{"radarr", client.Movie, true, false, "16.0", "movie", 50},
		{"sonarr", client.Series, false, true, "21.1-r2", "tv", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.version = tt.version
			c := newClient(t, f, map[string]any{"add_paused": tt.paused})
			id, err := c.Add(context.Background(), &client.Release{Title: seriesTitle + ": x", Kind: tt.kind, Recent: tt.recent, Data: nzb})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) {
				t.Errorf("Add() = %q, want a 32-digit hex ID", id)
			}
			calls := f.method("append")
			if len(calls) != 1 {
				t.Fatalf("append calls = %d, want 1", len(calls))
			}
			want := []any{
				seriesTitle + " - x.nzb", base64.StdEncoding.EncodeToString(nzb), tt.category, tt.priority, false, tt.paused,
				"", float64(0), "all",
				[]any{"drone", id},
			}
			if got, _ := json.Marshal(calls[0].Params); string(got) != mustJSON(want) {
				t.Errorf("append params = %s, want = %s", got, mustJSON(want))
			}
		})
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// Download_should_throw_if_failed
func TestAddRejected(t *testing.T) {
	f := newFake(t)
	f.version = "16.0"
	f.appendResult = 0
	c := newClient(t, f, nil)
	if _, err := c.Add(context.Background(), &client.Release{Title: movieTitle, Kind: client.Movie, Data: []byte("nzb")}); err == nil {
		t.Errorf("Add() error = nil, want rejected")
	}
}

func TestAddLegacy(t *testing.T) {
	tests := []struct {
		name    string
		version string
		result  any
		queue   map[string]any
		// editID is the ID the drone parameter is set on.
		editID float64
	}{
		{"13", "13.0", 5, map[string]any{"NZBID": 5, "NZBName": movieTitle}, 5},
		{"12", "12.0", true, map[string]any{"NZBID": 5, "LastID": 9, "NZBName": movieTitle}, 9},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.version, f.appendResult, f.queue = tt.version, tt.result, []map[string]any{tt.queue}
			c := newClient(t, f, nil)
			id, err := c.Add(context.Background(), &client.Release{Title: movieTitle, Kind: client.Movie, Data: []byte("nzb")})
			if err != nil {
				t.Fatalf("Add() error = %v", err)
			}
			edits := f.method("editqueue")
			want := []any{"GroupSetParameter", float64(0), "drone=" + id, tt.editID}
			if len(edits) != 1 || !slices.Equal(edits[0].Params, want) {
				t.Errorf("editqueue calls = %v, want = %v", edits, want)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		queue   []map[string]any
		history []map[string]any
		// want is the editqueue command and ID; empty for none.
		command string
		editID  float64
	}{
		{"history by drone", "id", nil, []map[string]any{{"ID": 3, "Category": "movie", "Parameters": drone}}, "HistoryDelete", 3},
		{"queue by drone", "id", []map[string]any{{"NZBID": 4, "Parameters": drone}}, nil, "GroupFinalDelete", 4},
		{"queue by NZBGet ID", "4", []map[string]any{{"NZBID": 4}}, nil, "GroupFinalDelete", 4},
		{"history by NZBGet ID", "3", nil, []map[string]any{{"ID": 3}}, "HistoryDelete", 3},
		{"unknown", "_Droned.S01E01.Pilot.1080p.WEB-DL-DRONE_0", nil, []map[string]any{{"ID": 3, "Parameters": drone}}, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.queue, f.history = tt.queue, tt.history
			c := newClient(t, f, nil)
			if err := c.Remove(context.Background(), client.Item{ID: tt.id}, false); err != nil {
				t.Fatalf("Remove() error = %v", err)
			}
			edits := f.method("editqueue")
			if tt.command == "" {
				if len(edits) != 0 {
					t.Errorf("editqueue calls = %v, want none", edits)
				}
				return
			}
			want := []any{tt.command, float64(0), "", tt.editID}
			if len(edits) != 1 || !slices.Equal(edits[0].Params, want) {
				t.Errorf("editqueue calls = %v, want = %v", edits, want)
			}
		})
	}
}

// RemoveItem_should_delete_folder
func TestRemoveDeletesFolder(t *testing.T) {
	f := newFake(t)
	f.history = []map[string]any{completed("movie", movieTitle)}
	dir := filepath.Join(t.TempDir(), seriesTitle)
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	c := newClient(t, f, nil)
	if err := c.Remove(context.Background(), client.Item{ID: "_Droned.S01E01.Pilot.1080p.WEB-DL-DRONE_0", OutputPath: dir}, true); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat() error = %v, want not exist", err)
	}
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		config   map[string]string
		want     []string
	}{
		// should_return_status_with_outputdir; should_return_status_with_mounted_outputdir
		// reads the folder as NZBGet reports it, as Mavio's server maps it.
		{"radarr", map[string]any{"tv_category": ""}, nil, []string{"/remote/mount/movie"}},
		{"sonarr", map[string]any{"movie_category": ""}, nil, []string{"/remote/mount/tv"}},
		{"both", nil, nil, []string{"/remote/mount/movie", "/remote/mount/tv"}},
		{"same", map[string]any{"tv_category": "movie"}, nil, []string{"/remote/mount/movie"}},
		{"default dest dir", nil, map[string]string{
			"MainDir": "/data", "DestDir": "${MainDir}/dst", "Category1.Name": "movie", "Category2.Name": "tv",
			"Category2.DestDir": "/tv",
		}, []string{"/data/dst/movie", "/tv"}},
		{"no category dir", nil, map[string]string{
			"DestDir": "/dst", "AppendCategoryDir": "no", "Category1.Name": "movie",
		}, []string{"/dst"}},
		{"missing category", map[string]any{"movie_category": "x", "tv_category": "y"}, nil, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			if tt.config != nil {
				f.config = tt.config
			}
			info, err := newClient(t, f, tt.settings).Info(context.Background())
			if err != nil {
				t.Fatalf("Info() error = %v", err)
			}
			if !info.IsLocalhost || !slices.Equal(info.OutputRootFolders, tt.want) {
				t.Errorf("IsLocalhost, OutputRootFolders = %v, %v, want = true, %v", info.IsLocalhost, info.OutputRootFolders, tt.want)
			}
		})
	}
}

func TestTest(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		keep     *string
		settings map[string]any
		ok       bool
	}{
		// should_test_version
		{name: "version 11.0", version: "11.0"},
		{name: "version 12.0", version: "12.0", ok: true},
		{name: "version 11.0-b30ef0134", version: "11.0-b30ef0134"},
		{name: "version 13.0-b30ef0134", version: "13.0-b30ef0134", ok: true},
		{name: "version unparsable", version: "21"},
		// should_test_keephistory
		{name: "keephistory 0", version: "14.0", keep: new("0")},
		{name: "keephistory 1", version: "14.0", keep: new("1"), ok: true},
		{name: "keephistory  7", version: "14.0", keep: new(" 7")},
		{name: "keephistory 5000000", version: "14.0", keep: new("5000000")},
		{name: "category missing", version: "14.0", settings: map[string]any{"tv_category": "series"}},
		{name: "category empty", version: "14.0", settings: map[string]any{"tv_category": "", "movie_category": ""}, ok: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.version = tt.version
			if tt.keep != nil {
				f.config["KeepHistory"] = *tt.keep
			}
			err := newClient(t, f, tt.settings).Test(context.Background())
			if (err == nil) != tt.ok {
				t.Errorf("Test() error = %v, want ok = %v", err, tt.ok)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	f := newFake(t)
	err := newClient(t, f, map[string]any{"password": "wrong"}).Test(context.Background())
	if _, ok := errors.AsType[*client.AuthError](err); !ok {
		t.Errorf("Test() error = %v, want an AuthError", err)
	}
	f.rpcError = true
	_, err = newClient(t, f, nil).Items(context.Background(), allOptions)
	if err == nil || !regexp.MustCompile(`Name: JSONRPCError, Code: 3, Message: Invalid parameter`).MatchString(err.Error()) {
		t.Errorf("Items() error = %v, want the RPC error", err)
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"defaults", `{}`, true},
		{"no credentials", `{"username": "", "password": ""}`, true},
		{"username alone", `{"password": ""}`, false},
		{"password alone", `{"username": ""}`, false},
		{"no host", `{"host": ""}`, false},
		{"unknown", `{"category": "x"}`, false},
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

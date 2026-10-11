package sabnzbd

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const (
	movieTitle = "Droned.1998.1080p.WEB-DL-DRONE"
	tvTitle    = "Droned.S01E01.Pilot.1080p.WEB-DL-DRONE"
	id         = "sabnzbd_nzb12345"
)

// upload is an NZB the fake was sent.
type upload struct {
	fileName, category, priority, contentType string
	data                                      []byte
}

// fake is a SABnzbd API server.
type fake struct {
	t          *testing.T
	mu         sync.Mutex
	version    string
	config     map[string]any
	misc       map[string]any
	fullStatus map[string]any
	queue      map[string]any
	queueSlots []map[string]any
	history    []map[string]any
	// addAnswer is the answer to addfile; the default when empty.
	addAnswer string
	uploads   []upload
	deletes   []url.Values
}

// newFake returns the fake of Radarr's and Sonarr's SabnzbdFixture.
func newFake(t *testing.T) *fake {
	f := &fake{
		t: t, version: "1.2.3",
		misc:       map[string]any{"complete_dir": "/remote/mount"},
		fullStatus: map[string]any{"completedir": `Y:\sabnzbd\root\complete`},
		queue:      map[string]any{"my_home": `Y:\nzbget\root`, "paused": false},
	}
	f.config = map[string]any{
		"misc": f.misc,
		"categories": []map[string]any{
			{"name": "movie", "dir": "vv", "priority": 0, "pp": "", "script": "Default"},
			{"name": "tv", "dir": "vv", "priority": 0, "pp": "", "script": "Default"},
		},
	}
	return f
}

func (f *fake) write(w http.ResponseWriter, v any) {
	if err := json.NewEncoder(w).Encode(v); err != nil {
		f.t.Errorf("encode: %v", err)
	}
}

// filter returns the slots of a category, as SABnzbd filters them.
func filter(slots []map[string]any, key, category string) []map[string]any {
	out := []map[string]any{}
	for _, s := range slots {
		if category == "" || s[key] == category {
			out = append(out, s)
		}
	}
	return out
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/api" {
		http.NotFound(w, r)
		return
	}
	q := r.URL.Query()
	if q.Get("output") != "json" {
		f.t.Errorf("output = %q, want = json", q.Get("output"))
	}
	if key := q.Get("apikey"); key != "key" && (key != "" || q.Get("ma_username") != "admin" || q.Get("ma_password") != "pass") {
		if key == "" && q.Get("ma_username") == "" {
			_, _ = io.WriteString(w, "error: API Key Required\n")
			return
		}
		f.write(w, map[string]any{"status": false, "error": "API Key Incorrect"})
		return
	}
	switch q.Get("mode") {
	case "version":
		f.write(w, map[string]any{"version": f.version})
	case "get_config":
		f.write(w, map[string]any{"config": f.config})
	case "fullstatus":
		if q.Get("skip_dashboard") != "1" {
			f.t.Errorf("fullstatus skip_dashboard = %q, want = 1", q.Get("skip_dashboard"))
		}
		f.write(w, map[string]any{"status": f.fullStatus})
	case "queue", "history":
		if q.Get("name") == "delete" {
			f.deletes = append(f.deletes, q)
			f.write(w, map[string]any{"status": true})
			return
		}
		if q.Get("mode") == "queue" {
			v := map[string]any{"slots": filter(f.queueSlots, "cat", q.Get("category"))}
			for k, x := range f.queue {
				v[k] = x
			}
			f.write(w, map[string]any{"queue": v})
			return
		}
		if q.Get("limit") != strconv.Itoa(historyLimit) {
			f.t.Errorf("history limit = %q, want = %d", q.Get("limit"), historyLimit)
		}
		f.write(w, map[string]any{"history": map[string]any{"slots": filter(f.history, "category", q.Get("category"))}})
	case "addfile":
		f.addFile(w, r, q)
	default:
		_, _ = io.WriteString(w, "error: not implemented\n")
	}
}

func (f *fake) addFile(w http.ResponseWriter, r *http.Request, q url.Values) {
	_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if r.Method != http.MethodPost || err != nil {
		f.t.Errorf("addfile %s %q, want a multipart POST", r.Method, r.Header.Get("Content-Type"))
		return
	}
	part, err := multipart.NewReader(r.Body, params["boundary"]).NextPart()
	if err != nil {
		f.t.Errorf("addfile part: %v", err)
		return
	}
	data, _ := io.ReadAll(part)
	if part.FormName() != "name" {
		f.t.Errorf("addfile form name = %q, want = name", part.FormName())
	}
	f.uploads = append(f.uploads, upload{
		fileName: part.FileName(), category: q.Get("cat"), priority: q.Get("priority"),
		contentType: part.Header.Get("Content-Type"), data: data,
	})
	if f.addAnswer != "" {
		_, _ = io.WriteString(w, f.addAnswer)
		return
	}
	f.write(w, map[string]any{"status": true, "nzo_ids": []string{"sabznbd_nzo12345"}})
}

// newClient returns a client talking to f, with settings over the
// fixture's.
func newClient(t *testing.T, f *fake, settings map[string]any) client.Client {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{
		"host": host, "port": p, "api_key": "key", "username": "admin", "password": "pass",
		"movie_category": "movie", "tv_category": "tv",
		"recent_movie_priority": int(priorityHigh), "recent_tv_priority": int(priorityHigh),
	}
	for k, v := range settings {
		if v == nil {
			delete(s, k)
			continue
		}
		s[k] = v
	}
	raw, _ := json.Marshal(s)
	c, err := Definitions()[0].New(raw, client.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return c
}

// queued is the fixture's queued download of a kind.
func queued(kind client.Kind, status string) map[string]any {
	cat, title := "movie", movieTitle
	if kind == client.Series {
		cat, title = "tv", tvTitle
	}
	return map[string]any{
		"status": status, "mb": "1000", "mbleft": "10", "timeleft": "0:00:10", "cat": cat, "nzo_id": id,
		"filename": title, "priority": "Normal", "index": 0, "percentage": "99",
	}
}

// completed is the fixture's completed download of a kind.
func completed(kind client.Kind) map[string]any {
	cat, title := "movie", movieTitle
	if kind == client.Series {
		cat, title = "tv", tvTitle
	}
	return map[string]any{
		"status": "Completed", "bytes": 1000, "category": cat, "nzo_id": id, "name": title,
		"storage": "/remote/mount/vv/" + title, "fail_message": "", "download_time": 10,
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

func single(t *testing.T, c client.Client) client.Item {
	t.Helper()
	got := items(t, c)
	if len(got) != 1 {
		t.Fatalf("Items() = %d items, want 1", len(got))
	}
	return got[0]
}

func TestItemsEmpty(t *testing.T) {
	// GetItems_should_return_no_items_when_queue_is_empty
	if got := items(t, newClient(t, newFake(t), nil)); len(got) != 0 {
		t.Errorf("Items() = %+v, want none", got)
	}
}

func TestQueueItems(t *testing.T) {
	tests := []struct {
		status string
		want   client.Status
	}{
		// queued_item_should_have_required_properties
		{"Grabbing", client.Queued},
		{"Queued", client.Queued},
		{"Propagating", client.Queued},
		// paused_item_should_have_required_properties
		{"Paused", client.Paused},
		// downloading_item_should_have_required_properties
		{"Checking", client.Downloading},
		{"Downloading", client.Downloading},
		{"QuickCheck", client.Downloading},
		{"Verifying", client.Downloading},
		{"Repairing", client.Downloading},
		{"Fetching", client.Downloading},
		{"Extracting", client.Downloading},
		{"Moving", client.Downloading},
		{"Running", client.Downloading},
	}
	for _, kind := range client.Kinds {
		for _, tt := range tests {
			f := newFake(t)
			f.queueSlots = []map[string]any{queued(kind, tt.status)}
			item := single(t, newClient(t, f, nil))
			if item.Status != tt.want || item.Kind != kind {
				t.Errorf("%v %s: Status, Kind = %v, %v, want = %v, %v", kind, tt.status, item.Status, item.Kind, tt.want, kind)
			}
			if item.ID != id || item.Title == "" || item.RemainingSize != 10*1024*1024 || item.TotalSize != 1000*1024*1024 {
				t.Errorf("%v %s: item = %+v, want the slot's ID, title and sizes", kind, tt.status, item)
			}
			switch {
			case tt.want == client.Paused && item.RemainingTime != nil:
				t.Errorf("%s: RemainingTime = %v, want = nil", tt.status, *item.RemainingTime)
			case tt.want != client.Paused && (item.RemainingTime == nil || *item.RemainingTime != 10*time.Second):
				t.Errorf("%s: RemainingTime = %v, want = 10s", tt.status, item.RemainingTime)
			}
			if !item.CanBeRemoved || !item.CanMoveFiles {
				t.Errorf("%s: CanBeRemoved, CanMoveFiles = %v, %v, want = true, true", tt.status, item.CanBeRemoved, item.CanMoveFiles)
			}
		}
	}
}

func TestPausedQueue(t *testing.T) {
	tests := []struct {
		priority string
		want     client.Status
	}{
		{"Normal", client.Paused},
		{"Force", client.Downloading},
		{"2", client.Downloading},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.queue["paused"] = true
		slot := queued(client.Movie, "Downloading")
		slot["priority"] = tt.priority
		f.queueSlots = []map[string]any{slot}
		if got := single(t, newClient(t, f, nil)).Status; got != tt.want {
			t.Errorf("paused queue, priority %s: Status = %v, want = %v", tt.priority, got, tt.want)
		}
	}
}

func TestEncrypted(t *testing.T) {
	f := newFake(t)
	slot := queued(client.Movie, "Downloading")
	slot["filename"] = "ENCRYPTED / " + movieTitle
	f.queueSlots = []map[string]any{slot}
	item := single(t, newClient(t, f, nil))
	if !item.Encrypted || item.Title != " "+movieTitle {
		t.Errorf("Encrypted, Title = %v, %q, want = true, %q", item.Encrypted, item.Title, " "+movieTitle)
	}
}

func TestHistoryItems(t *testing.T) {
	tests := []struct {
		name, status, message string
		want                  client.Status
	}{
		// completed_download_should_have_required_properties
		{"completed", "Completed", "", client.Completed},
		// failed_item_should_have_required_properties
		{"failed", "Failed", "", client.Failed},
		{"failed with a message", "Failed", "Repair failed, not enough repair blocks", client.Failed},
		// should_report_diskspace_unpack_error_as_warning
		{"disk full", "Failed", "Unpacking failed, write error or disk is full?", client.Warning},
		{"verifying", "Verifying", "", client.Downloading},
		{"moving", "Moving", "", client.Downloading},
	}
	for _, kind := range client.Kinds {
		for _, tt := range tests {
			f := newFake(t)
			slot := completed(kind)
			slot["status"], slot["fail_message"] = tt.status, tt.message
			f.history = []map[string]any{slot}
			item := single(t, newClient(t, f, nil))
			if item.Status != tt.want || item.Kind != kind || item.Message != tt.message {
				t.Errorf("%v %s: Status, Kind, Message = %v, %v, %q, want = %v, %v, %q",
					kind, tt.name, item.Status, item.Kind, item.Message, tt.want, kind, tt.message)
			}
			if item.ID != id || item.RemainingSize != 0 || item.RemainingTime == nil || *item.RemainingTime != 0 || item.TotalSize != 1000 {
				t.Errorf("%v %s: item = %+v, want the slot's ID and size, nothing remaining", kind, tt.name, item)
			}
			if !item.CanBeRemoved || !item.CanMoveFiles {
				t.Errorf("%s: CanBeRemoved, CanMoveFiles = %v, %v, want = true, true", tt.name, item.CanBeRemoved, item.CanMoveFiles)
			}
		}
	}
}

func TestDeletedIgnored(t *testing.T) {
	// deleted_queue_item_should_be_ignored
	f := newFake(t)
	f.queueSlots = []map[string]any{queued(client.Movie, "Deleted")}
	if got := items(t, newClient(t, f, nil)); len(got) != 0 {
		t.Errorf("Items() with a deleted queue item = %+v, want none", got)
	}
	// deleted_history_item_should_be_ignored
	f = newFake(t)
	slot := completed(client.Series)
	slot["status"] = "Deleted"
	f.history = []map[string]any{slot}
	if got := items(t, newClient(t, f, nil)); len(got) != 0 {
		t.Errorf("Items() with a deleted history item = %+v, want none", got)
	}
}

func TestItemsCategories(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		category string
		want     int
		kind     client.Kind
	}{
		// GetItems_should_ignore_downloads_from_other_categories
		{"other category", nil, "myowncat", 0, 0},
		{"movie", nil, "movie", 1, client.Movie},
		{"series", nil, "tv", 1, client.Series},
		{"one category", map[string]any{"movie_category": "both", "tv_category": "both"}, "both", 1, 0},
		// SABnzbd's "*" is no category.
		{"no category", map[string]any{"movie_category": ""}, "*", 1, client.Movie},
		{"no category not taken", nil, "*", 0, 0},
		{"no category but other", map[string]any{"tv_category": ""}, "myowncat", 0, 0},
	}
	for _, tt := range tests {
		f := newFake(t)
		slot := completed(client.Movie)
		slot["category"] = tt.category
		f.history = []map[string]any{slot}
		got := items(t, newClient(t, f, tt.settings))
		if len(got) != tt.want {
			t.Fatalf("%s: Items() = %d items, want = %d", tt.name, len(got), tt.want)
		}
		if tt.want > 0 && got[0].Kind != tt.kind {
			t.Errorf("%s: Kind = %v, want = %v", tt.name, got[0].Kind, tt.kind)
		}
	}
}

func TestAdd(t *testing.T) {
	tests := []struct {
		name     string
		release  client.Release
		fileName string
		category string
		priority string
	}{
		// Download_should_use_clean_title
		{
			"clean title",
			client.Release{Title: "[ TOWN ]-[ http://www.town.ag ]-[ ANIME ]-[Usenet Provider >> http://www.ssl- <<] - [Commie] Aldnoah Zero 18 [234C8FC7]", Kind: client.Series},
			"[ TOWN ]-[ http-++www.town.ag ]-[ ANIME ]-[Usenet Provider  http-++www.ssl- ] - [Commie] Aldnoah Zero 18 [234C8FC7].nzb",
			"tv", "-100",
		},
		{"movie", client.Release{Title: movieTitle, Kind: client.Movie}, movieTitle + ".nzb", "movie", "-100"},
		{"file name kept", client.Release{Title: movieTitle, Kind: client.Movie, FileName: "x.nzb"}, "x.nzb", "movie", "-100"},
		// Download_should_use_sabRecentTvPriority_when_recentEpisode_is_true
		{"recent episode", client.Release{Title: tvTitle, Kind: client.Series, Recent: true}, tvTitle + ".nzb", "tv", "1"},
		{"recent movie", client.Release{Title: movieTitle, Kind: client.Movie, Recent: true}, movieTitle + ".nzb", "movie", "1"},
	}
	for _, tt := range tests {
		f := newFake(t)
		r := tt.release
		r.Data = []byte("<nzb/>")
		got, err := newClient(t, f, nil).Add(context.Background(), &r)
		// Download_should_return_unique_id
		if err != nil || got != "sabznbd_nzo12345" {
			t.Errorf("%s: Add() = %q, %v, want = sabznbd_nzo12345", tt.name, got, err)
		}
		if len(f.uploads) != 1 {
			t.Fatalf("%s: uploads = %d, want 1", tt.name, len(f.uploads))
		}
		u := f.uploads[0]
		if u.fileName != tt.fileName || u.category != tt.category || u.priority != tt.priority ||
			string(u.data) != "<nzb/>" || u.contentType != "application/x-nzb" {
			t.Errorf("%s: upload = %+v, want = %q in %s at %s", tt.name, u, tt.fileName, tt.category, tt.priority)
		}
	}
}

func TestAddRejected(t *testing.T) {
	for _, answer := range []string{`{"status":true,"nzo_ids":[]}`, "ok\n", `{"status":false,"error":"no file"}`} {
		f := newFake(t)
		f.addAnswer = answer
		r := client.Release{Title: movieTitle, Kind: client.Movie, Data: []byte("<nzb/>")}
		if got, err := newClient(t, f, nil).Add(context.Background(), &r); err == nil {
			t.Errorf("Add() answered %s = %q, want an error", answer, got)
		}
	}
}

func TestOutputPath(t *testing.T) {
	tests := []struct {
		name, title, storage, want string
	}{
		// should_return_path_to_jobfolder
		{"job folder", movieTitle, `C:\sorted\` + movieTitle + `\Droned.1998_1080p_WEB-DL-DRONE.mkv`, `C:\sorted\` + movieTitle},
		{"sub dir", movieTitle, `C:\sorted\` + movieTitle + `\SubDir\Droned.1998_1080p_WEB-DL-DRONE.mkv`, `C:\sorted\` + movieTitle},
		{"title with extension", movieTitle + ".mkv", `C:\sorted\` + movieTitle + `.mkv\SubDir\Droned.1998_1080p_WEB-DL-DRONE.mkv`, `C:\sorted\` + movieTitle + ".mkv"},
		{"sub sub dir", movieTitle + ".mkv", `C:\sorted\` + movieTitle + `.mkv\SubDir\SubDir\Droned.1998_1080p_WEB-DL-DRONE.mkv`, `C:\sorted\` + movieTitle + ".mkv"},
		{"unix job folder", tvTitle, "/sorted/" + tvTitle + "/SubDir/Droned.S01E01_Pilot_1080p_WEB-DL-DRONE.mkv", "/sorted/" + tvTitle},
		// The storage itself, unmapped: the server maps remote paths.
		{"storage", movieTitle, "/remote/mount/vv/" + movieTitle, "/remote/mount/vv/" + movieTitle},
		// should_not_blow_up_if_storage_is_drive_root
		{"drive root", movieTitle, `C:\`, `C:\`},
		{"unix root", movieTitle, "/", "/"},
		// should_not_blow_up_if_storage_doesnt_have_jobfolder
		{"no job folder", movieTitle, `C:\sorted\somewhere\asdfasdf\asdfasdf.mkv`, `C:\sorted\somewhere\asdfasdf\asdfasdf.mkv`},
		{"no storage", movieTitle, "", ""},
	}
	for _, tt := range tests {
		f := newFake(t)
		slot := completed(client.Movie)
		slot["name"], slot["storage"] = tt.title, tt.storage
		f.history = []map[string]any{slot}
		if got := single(t, newClient(t, f, nil)).OutputPath; got != tt.want {
			t.Errorf("%s: OutputPath = %q, want = %q", tt.name, got, tt.want)
		}
	}
}

func TestInfoOutputRootFolders(t *testing.T) {
	tests := []struct {
		version, root, completeDir, categoryDir, fullCompleteDir, want string
	}{
		// should_return_status_with_outputdir_for_version_lt_2
		{"1.2.1", `Y:\sabnzbd\root`, `completed\downloads`, "vv", "", `Y:\sabnzbd\root\completed\downloads\vv`},
		{"1.2.1", `Y:\sabnzbd\root`, "completed", "vv", "", `Y:\sabnzbd\root\completed\vv`},
		{"1.2.1", "/sabnzbd/root", "completed/downloads", "vv", "", "/sabnzbd/root/completed/downloads/vv"},
		{"1.2.1", "/sabnzbd/root", "completed", "vv", "", "/sabnzbd/root/completed/vv"},
		// should_return_status_with_outputdir_for_version_gte_2
		{"2.0.0beta1", "", `completed\downloads`, "vv", `Y:\sabnzbd\root\completed\downloads`, `Y:\sabnzbd\root\completed\downloads\vv`},
		{"2.0.0beta1", "", "completed", "vv", `Y:\sabnzbd\root\completed`, `Y:\sabnzbd\root\completed\vv`},
		{"2.0.0beta1", "", "completed/downloads", "vv", "/sabnzbd/root/completed/downloads", "/sabnzbd/root/completed/downloads/vv"},
		{"2.0.0beta1", "", "completed", "vv", "/sabnzbd/root/completed", "/sabnzbd/root/completed/vv"},
		// A rooted complete_dir is taken as is; job folders off (*) are
		// still the category's folder.
		{"4.3.0", "", "/downloads/complete", "vv*", "", "/downloads/complete/vv"},
		{"4.3.0", "", "/downloads/complete", "", "", "/downloads/complete"},
		{"4.3.0", "", "/downloads/complete", "/elsewhere", "", "/elsewhere"},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.version = tt.version
		f.queue["my_home"] = tt.root
		f.fullStatus["completedir"] = tt.fullCompleteDir
		f.misc["complete_dir"] = tt.completeDir
		f.config["categories"] = []map[string]any{{"name": "movie", "dir": tt.categoryDir}, {"name": "tv", "dir": tt.categoryDir}}
		info, err := newClient(t, f, nil).Info(context.Background())
		if err != nil {
			t.Fatalf("Info() error = %v", err)
		}
		if !info.IsLocalhost || len(info.OutputRootFolders) != 1 || info.OutputRootFolders[0] != tt.want {
			t.Errorf("%s %s+%s: Info() = %+v, want localhost with %q", tt.version, tt.root, tt.completeDir, info, tt.want)
		}
	}
}

func TestInfoCategories(t *testing.T) {
	f := newFake(t)
	f.config["categories"] = []map[string]any{{"name": "*", "dir": ""}, {"name": "tv", "dir": "series"}}
	f.misc["enable_movie_sorting"], f.misc["movie_categories"] = 1, []string{"tv"}
	info, err := newClient(t, f, nil).Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/remote/mount", "/remote/mount/series"}
	if len(info.OutputRootFolders) != 2 || info.OutputRootFolders[0] != want[0] || info.OutputRootFolders[1] != want[1] {
		t.Errorf("OutputRootFolders = %v, want = %v", info.OutputRootFolders, want)
	}
	if info.SortingMode != "Movie" {
		t.Errorf("SortingMode = %q, want = Movie", info.SortingMode)
	}
}

func TestSortingMode(t *testing.T) {
	tests := []struct {
		name string
		misc map[string]any
		want string
	}{
		{"none", nil, ""},
		{"tv", map[string]any{"enable_tv_sorting": true, "tv_categories": []string{"movie"}}, "TV"},
		{"tv all", map[string]any{"enable_tv_sorting": "1", "tv_categories": nil}, "TV"},
		{"tv other", map[string]any{"enable_tv_sorting": true, "tv_categories": []string{"other"}}, ""},
		{"movie", map[string]any{"enable_movie_sorting": 1, "movie_categories": []string{"movie"}}, "Movie"},
		{"date", map[string]any{"enable_date_sorting": 1, "date_categories": "movie"}, "Date"},
		// A null date_categories is a list of null, holding no category.
		{"date null", map[string]any{"enable_date_sorting": 1, "date_categories": nil}, ""},
	}
	for _, tt := range tests {
		f := newFake(t)
		for k, v := range tt.misc {
			f.misc[k] = v
		}
		info, err := newClient(t, f, map[string]any{"tv_category": "movie"}).Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.SortingMode != tt.want {
			t.Errorf("%s: SortingMode = %q, want = %q", tt.name, info.SortingMode, tt.want)
		}
	}
}

func TestRemovesCompletedDownloads(t *testing.T) {
	tests := []struct {
		retention any
		option    string
		number    any
		want      bool
	}{
		// should_set_history_removes_completed_downloads_false
		{"0", "", 0, false},
		{"15d", "", 0, false},
		{"", "", 0, false},
		{nil, "", 0, false},
		// should_set_history_removes_completed_downloads_true
		{"-1", "", 0, true},
		{"15", "", 0, true},
		{"3", "", 0, true},
		{"3d", "", 0, true},
		// should_set_history_removes_completed_downloads_false_for_separate_properties
		{nil, "all", 0, false},
		{nil, "days-archive", 15, false},
		{nil, "days-delete", "15", false},
		// should_set_history_removes_completed_downloads_true_for_separate_properties
		{nil, "number-archive", 10, true},
		{nil, "number-delete", 10, true},
		{nil, "number-archive", 0, true},
		{nil, "number-delete", 0, true},
		{nil, "days-archive", 3, true},
		{nil, "days-delete", 3, true},
		{nil, "all-archive", 0, true},
		{nil, "all-delete", 0, true},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.misc["history_retention"], f.misc["history_retention_option"], f.misc["history_retention_number"] = tt.retention, tt.option, tt.number
		info, err := newClient(t, f, nil).Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if info.RemovesCompletedDownloads != tt.want {
			t.Errorf("%v %q %v: RemovesCompletedDownloads = %v, want = %v", tt.retention, tt.option, tt.number, info.RemovesCompletedDownloads, tt.want)
		}
	}
}

func TestTestVersion(t *testing.T) {
	tests := []struct {
		version string
		ok      bool
	}{
		// should_test_version
		{"0.6.9", false},
		{"0.7.0", true},
		{"0.8.0", true},
		{"1.0.0", true},
		{"1.0.0RC1", true},
		{"1.1.x", true},
		// should_test_develop_version_successfully, with a warning logged
		{"develop", true},
		{"", false},
		{"nightly", false},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.version = tt.version
		err := newClient(t, f, nil).Test(context.Background())
		if (err == nil) != tt.ok {
			t.Errorf("%q: Test() error = %v, want ok = %v", tt.version, err, tt.ok)
		}
	}
}

func TestTestCategory(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		misc     map[string]any
		sorters  []map[string]any
		dir      string
		ok       bool
	}{
		// should_test_success_if_sorters_are_empty
		{"sorters empty", nil, map[string]any{"enable_tv_sorting": false, "tv_categories": nil}, []map[string]any{}, "vv", true},
		// should_test_failed_if_sorter_is_enabled_for_non_tv_category
		{
			"sorter of other category", nil,
			map[string]any{"enable_tv_sorting": false, "tv_categories": nil},
			[]map[string]any{{"is_active": true, "sort_cats": []string{"movie-custom"}}, {"is_active": 1, "sort_cats": []string{"tv-custom"}}},
			"vv", true,
		},
		// should_test_failed_if_sorter_is_enabled_for_tv_category
		{
			"sorter of movie category", nil,
			map[string]any{"enable_tv_sorting": false, "tv_categories": nil},
			[]map[string]any{{"is_active": true, "sort_cats": []string{"movie"}}},
			"vv", false,
		},
		{
			"sorter of tv category", nil,
			map[string]any{"enable_tv_sorting": false, "tv_categories": nil},
			[]map[string]any{{"is_active": 1, "sort_cats": []string{"tv"}}},
			"vv", false,
		},
		{"inactive sorter", nil, nil, []map[string]any{{"is_active": 0, "sort_cats": []string{"tv"}}}, "vv", true},
		// should_test_success_if_tv_sorting_disabled
		{"tv sorting disabled", nil, map[string]any{"enable_tv_sorting": false, "tv_categories": nil}, nil, "vv", true},
		// should_test_failed_if_tv_sorting_null
		{"tv sorting null", nil, map[string]any{"enable_tv_sorting": true, "tv_categories": nil}, nil, "vv", false},
		// should_test_failed_if_tv_sorting_empty
		{"tv sorting empty", nil, map[string]any{"enable_tv_sorting": true, "tv_categories": []string{}}, nil, "vv", false},
		// should_test_success_if_tv_sorting_contains_different_category
		{"tv sorting other", nil, map[string]any{"enable_tv_sorting": true, "tv_categories": []string{"tv-custom"}}, nil, "vv", true},
		// should_test_failed_if_tv_sorting_contains_category
		{"tv sorting movie", nil, map[string]any{"enable_tv_sorting": true, "tv_categories": []string{"movie"}}, nil, "vv", false},
		{"tv sorting tv", nil, map[string]any{"enable_tv_sorting": true, "tv_categories": []string{"tv"}}, nil, "vv", false},
		// should_test_failed_if_tv_sorting_default_category
		{
			"tv sorting default",
			map[string]any{"movie_category": "", "tv_category": ""},
			map[string]any{"enable_tv_sorting": true, "tv_categories": []string{"Default"}},
			nil, "vv", false,
		},
		{"movie sorting", nil, map[string]any{"enable_movie_sorting": true, "movie_categories": []string{"tv"}}, nil, "vv", false},
		{"date sorting", nil, map[string]any{"enable_date_sorting": true, "date_categories": "movie"}, nil, "vv", false},
		{"job folders off", nil, nil, nil, "vv*", false},
		{"missing category", map[string]any{"tv_category": "missing"}, nil, nil, "vv", false},
		{"check before download", nil, map[string]any{"pre_check": 1}, nil, "vv", false},
	}
	for _, tt := range tests {
		f := newFake(t)
		if tt.name == "check before download" {
			f.version = "1.0.3"
		}
		for k, v := range tt.misc {
			f.misc[k] = v
		}
		if tt.sorters != nil {
			f.config["sorters"] = tt.sorters
		}
		f.config["categories"] = []map[string]any{{"name": "movie", "dir": tt.dir}, {"name": "tv", "dir": tt.dir}}
		err := newClient(t, f, tt.settings).Test(context.Background())
		if (err == nil) != tt.ok {
			t.Errorf("%s: Test() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
	// Check before download is fine from 1.1.
	f := newFake(t)
	f.version, f.misc["pre_check"] = "1.1.0", true
	if err := newClient(t, f, nil).Test(context.Background()); err != nil {
		t.Errorf("pre_check on 1.1.0: Test() error = %v, want nil", err)
	}
}

func TestAuth(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     string
	}{
		{"wrong key", map[string]any{"api_key": "wrong"}, "API Key Incorrect"},
		{"no credentials", map[string]any{"api_key": "", "username": "", "password": ""}, ""},
		{"username and password", map[string]any{"api_key": ""}, "ok"},
		{"wrong password", map[string]any{"api_key": "", "password": "wrong"}, "API Key Incorrect"},
	}
	for _, tt := range tests {
		f := newFake(t)
		if tt.want == "" {
			raw, _ := json.Marshal(tt.settings)
			if _, err := Definitions()[0].New(raw, client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)}); err == nil {
				t.Errorf("%s: New() error = nil, want an error", tt.name)
			}
			continue
		}
		c := newClient(t, f, tt.settings)
		_, err := c.Items(context.Background(), client.Options{})
		var auth *client.AuthError
		switch {
		case tt.want == "ok" && err != nil:
			t.Errorf("%s: Items() error = %v, want nil", tt.name, err)
		case tt.want != "ok" && !errors.As(err, &auth):
			t.Errorf("%s: Items() error = %v, want an AuthError", tt.name, err)
		}
		// TestAuthentication, after the version, which needs no key.
		f.version = "4.3.0"
		err = c.Test(context.Background())
		if tt.want != "ok" && (!errors.As(err, &auth) || !strings.Contains(err.Error(), tt.want)) {
			t.Errorf("%s: Test() error = %v, want %q", tt.name, err, tt.want)
		}
	}
}

func TestPlainTextErrors(t *testing.T) {
	tests := []struct {
		body string
		ok   bool
	}{
		{"ok\n", true},
		{`{"status":true}`, true},
		{`{"version":"1.0.0"}`, true},
		{"error: API Key Required\n", false},
		{`{"status":false,"error":"API Key Incorrect"}`, false},
		{`{"status":"False","error":"oops"}`, false},
		{"Error: something\n", false},
	}
	for _, tt := range tests {
		err := checkForError([]byte(tt.body))
		if (err == nil) != tt.ok {
			t.Errorf("checkForError(%q) = %v, want ok = %v", tt.body, err, tt.ok)
		}
	}
	if err := checkForError([]byte("error: API Key Required\n")); err == nil || !strings.HasSuffix(err.Error(), ": API Key Required") {
		t.Errorf("checkForError() = %v, want the message without its prefix", err)
	}
}

func TestRemove(t *testing.T) {
	dir := t.TempDir()
	folder := filepath.Join(dir, "Series.Title.S01E01")
	file := filepath.Join(dir, "Series.Title.S01E01.mkv")
	missing := filepath.Join(dir, "missing.mkv")
	tests := []struct {
		name       string
		path       string
		isDir      bool
		status     client.Status
		deleteData bool
		inQueue    bool
		deleted    bool
		want       url.Values
	}{
		// should_remove_output_path_folder_when_deleting_a_completed_item_and_delete_data_is_true
		{"folder", folder, true, client.Completed, true, false, true, url.Values{"mode": {"history"}, "del_files": {"1"}, "archive": {"1"}}},
		// should_remove_output_path_file_when_deleting_a_completed_item_and_delete_data_is_true
		{"file", file, false, client.Completed, true, false, true, url.Values{"mode": {"history"}, "del_files": {"1"}, "archive": {"1"}}},
		// should_not_remove_output_path_file_when_deleting_a_completed_item_and_delete_data_is_true_if_it_does_not_exist
		{"missing", missing, false, client.Completed, true, false, false, url.Values{"mode": {"history"}, "del_files": {"1"}, "archive": {"1"}}},
		// should_not_remove_output_path_file_when_deleting_a_completed_item_and_delete_data_is_false
		{"keep data", file, false, client.Completed, false, false, false, url.Values{"mode": {"history"}, "del_files": {"0"}, "archive": {"1"}}},
		// Failed downloads leave the history for good.
		{"failed", file, false, client.Failed, true, false, false, url.Values{"mode": {"history"}, "del_files": {"1"}, "archive": {"0"}}},
		{"queued", file, false, client.Downloading, true, true, false, url.Values{"mode": {"queue"}, "del_files": {"1"}, "archive": nil}},
	}
	for _, tt := range tests {
		create := tt.path != missing
		if create && tt.isDir {
			if err := os.MkdirAll(filepath.Join(tt.path, "sub"), 0o755); err != nil {
				t.Fatal(err)
			}
		} else if create {
			if err := os.WriteFile(tt.path, []byte("x"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		f := newFake(t)
		slot := completed(client.Series)
		slot["storage"] = tt.path
		f.history = []map[string]any{slot}
		if tt.inQueue {
			f.queueSlots = []map[string]any{queued(client.Series, "Downloading")}
		}
		item := client.Item{ID: id, Title: tvTitle, Status: tt.status, OutputPath: tt.path, Kind: client.Series}
		if err := newClient(t, f, nil).Remove(context.Background(), item, tt.deleteData); err != nil {
			t.Fatalf("%s: Remove() error = %v", tt.name, err)
		}
		_, err := os.Stat(tt.path)
		if gone := errors.Is(err, os.ErrNotExist) && create; gone != tt.deleted {
			t.Errorf("%s: data deleted = %v, want = %v", tt.name, gone, tt.deleted)
		}
		if len(f.deletes) != 1 {
			t.Fatalf("%s: deletes = %v, want 1", tt.name, f.deletes)
		}
		d := f.deletes[0]
		if d.Get("value") != id || d.Get("mode") != tt.want.Get("mode") || d.Get("del_files") != tt.want.Get("del_files") ||
			d.Get("archive") != tt.want.Get("archive") {
			t.Errorf("%s: delete = %v, want = %v", tt.name, d, tt.want)
		}
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"api key", `{"api_key":"k"}`, true},
		{"username and password", `{"username":"u","password":"p"}`, true},
		{"nothing", `{}`, false},
		{"no password", `{"username":"u"}`, false},
		{"url base", `{"api_key":"k","url_base":"/sabnzbd"}`, true},
		{"url base with scheme", `{"api_key":"k","url_base":"http://sab"}`, false},
		{"unknown", `{"api_key":"k","bogus":1}`, false},
		{"no host", `{"api_key":"k","host":""}`, false},
	}
	for _, tt := range tests {
		_, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
		if (err == nil) != tt.ok {
			t.Errorf("%s: New() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

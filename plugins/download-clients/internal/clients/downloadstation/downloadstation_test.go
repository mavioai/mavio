package downloadstation

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"path"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const (
	magnetURL   = "magnet:?xt=urn:btih:5dee65101db281ac9c46344cd6b175cdcad53426&dn=download"
	movieTitle  = "Droned.1998.1080p.WEB-DL-DRONE"
	physical    = "/mnt/sdb1/mydata"
	tvDirectory = "video/Series"
	category    = "sonarr"
)

// hashedSerial is the hash of the fake's serial number.
var hashedSerial = hashSerial("SERIALNUMBER")

// fakeTask is a task of the fake Diskstation.
type fakeTask struct {
	ID, Title, Type string
	Size            int64
	Status          taskStatus
	Destination     string
	URI             string
	Transfer        map[string]string
	StatusExtra     map[string]string
}

// created is a task created on the fake.
type created struct {
	API      string
	Version  string
	Params   map[string]string
	SID      string
	FileName string
	Data     string
}

// fake is a Diskstation's DSM web API with Download Station and
// FileStation.
type fake struct {
	t  *testing.T
	mu sync.Mutex
	// v2 is set when the Diskstation has DownloadStation2.Task.
	v2               bool
	taskMin, taskMax int
	serial           string
	serialFails      bool
	defaultDest      string
	location         string
	// shares are the shared folders' physical paths, and dirs the other
	// folders.
	shares map[string]string
	dirs   map[string]bool
	tasks  []fakeTask
	// failTasks makes listing tasks fail with an API error.
	failTasks bool
	// expireSession makes the next authenticated call fail with code.
	expireSession int

	created                         []created
	removed                         []string
	logins, serialCalls, fileCalls  int
	infoQueries                     int
	locationCalls, defaultDestCalls int
}

func newFake(t *testing.T, v2 bool) *fake {
	return &fake{
		t: t, v2: v2, taskMin: 1, taskMax: 3, serial: "SERIALNUMBER", defaultDest: "somepath", location: "location",
		shares: map[string]string{"/shared": physical, "/somepath": physical, "/video": "/mnt/sdb1/video"},
		dirs:   map[string]bool{"/somepath/sonarr": true, "/video/Series": true, "/shared/folder": true},
	}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.Method == http.MethodPost {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			f.t.Errorf("multipart form: %v", err)
		}
	} else {
		_ = r.ParseForm()
	}
	api := r.Form.Get("api")
	reply := func(data any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
	}
	fail := func(code int) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": false, "error": map[string]any{"code": code}})
	}
	switch path.Base(r.URL.Path) {
	case "query.cgi":
		f.infoQueries++
		data := map[string]any{}
		for _, name := range strings.Split(r.Form.Get("query"), ",") {
			switch api := (map[string]any{"path": "entry.cgi", "minVersion": 1, "maxVersion": 2}); name {
			case string(synoAuth):
				data[name] = map[string]any{"path": "/auth.cgi", "minVersion": 1, "maxVersion": 7}
			case string(synoTask2):
				if f.v2 {
					data[name] = map[string]any{"path": "entry.cgi", "minVersion": f.taskMin, "maxVersion": f.taskMax}
				}
			case string(synoTask):
				data[name] = map[string]any{"path": "DownloadStation/task.cgi", "minVersion": f.taskMin, "maxVersion": f.taskMax}
			default:
				data[name] = api
			}
		}
		reply(data)
		return
	case "auth.cgi":
		if r.Form.Get("account") != "admin" || r.Form.Get("passwd") != "pass" {
			fail(400)
			return
		}
		f.logins++
		reply(map[string]any{"sid": "sid" + strconv.Itoa(f.logins)})
		return
	}
	if sid := r.Form.Get("_sid"); sid != "sid"+strconv.Itoa(f.logins) {
		fail(119)
		return
	}
	if f.expireSession != 0 {
		fail(f.expireSession)
		f.expireSession = 0
		return
	}
	method := r.Form.Get("method")
	switch {
	case api == string(synoDSMInfo):
		f.serialCalls++
		if f.serialFails {
			fail(100)
			return
		}
		reply(map[string]any{"serial": f.serial})
	case api == string(synoDSInfo):
		f.defaultDestCalls++
		reply(map[string]any{"default_destination": f.defaultDest})
	case api == string(synoLocation):
		f.locationCalls++
		reply(map[string]any{"default_destination": f.location})
	case api == string(synoFileList):
		f.fileCalls++
		var paths []string
		if err := json.Unmarshal([]byte(r.Form.Get("path")), &paths); err != nil || len(paths) != 1 {
			fail(400)
			return
		}
		p := paths[0]
		share := "/" + strings.Split(strings.TrimPrefix(p, "/"), "/")[0]
		real, ok := f.shares[share]
		if !ok {
			reply(map[string]any{"files": []any{map[string]any{"code": 408, "path": p}}})
			return
		}
		reply(map[string]any{"files": []any{map[string]any{
			"isdir": p == share || f.dirs[p], "path": p, "name": path.Base(p),
			"additional": map[string]any{"real_path": real + strings.TrimPrefix(p, share)},
		}}})
	case (api == string(synoTask) || api == string(synoTask2)) && method == "list":
		if f.failTasks {
			fail(100)
			return
		}
		f.list(w, api == string(synoTask2), r.Form.Get("additional"))
	case method == "create":
		c := created{API: api, Version: r.Form.Get("version"), Params: map[string]string{}, SID: r.URL.Query().Get("_sid")}
		for k, v := range r.Form {
			c.Params[k] = v[0]
		}
		uri := r.Form.Get("uri") + r.Form.Get("url")
		typ := "BT"
		if r.MultipartForm != nil {
			for _, files := range r.MultipartForm.File {
				c.FileName = files[0].Filename
				file, _ := files[0].Open()
				data, _ := io.ReadAll(file)
				c.Data = string(data)
				uri = c.FileName
				if strings.HasSuffix(uri, ".nzb") {
					typ = "NZB"
				} else {
					uri = strings.TrimSuffix(uri, ".torrent")
				}
			}
		}
		f.created = append(f.created, c)
		f.tasks = append(f.tasks, fakeTask{
			ID: "dbid_" + strconv.Itoa(100+len(f.created)), Title: "title", Type: typ, Size: 1000, Status: statusWaiting,
			Destination: "shared/folder", URI: uri, Transfer: map[string]string{"size_downloaded": "0", "speed_download": "0"},
		})
		reply(nil)
	case method == "delete":
		f.removed = append(f.removed, api+" "+r.Form.Get("version")+" "+r.Form.Get("id"))
		reply(nil)
	default:
		fail(103)
	}
}

// list answers a list of tasks: v1 with their status by name and both
// details and transfer, v2 with their status by number and one of them.
func (f *fake) list(w http.ResponseWriter, v2 bool, additional string) {
	var tasks []map[string]any
	for _, t := range f.tasks {
		m := map[string]any{"id": t.ID, "title": t.Title, "type": t.Type, "size": t.Size, "username": "admin"}
		if t.StatusExtra != nil {
			m["status_extra"] = t.StatusExtra
		}
		if v2 {
			m["status"] = int(t.Status)
		} else {
			m["status"] = t.Status.String()
		}
		add := map[string]any{}
		if strings.Contains(additional, "detail") {
			add["detail"] = map[string]any{"destination": t.Destination, "uri": t.URI, "priority": "auto"}
		}
		if strings.Contains(additional, "transfer") {
			transfer := map[string]any{}
			for k, v := range t.Transfer {
				// DSM sends numbers.
				if n, err := strconv.ParseInt(v, 10, 64); err == nil {
					transfer[k] = n
				} else {
					transfer[k] = v
				}
			}
			add["transfer"] = transfer
		}
		m["additional"] = add
		tasks = append(tasks, m)
	}
	data := map[string]any{"offset": 0, "total": len(tasks), "tasks": tasks}
	if v2 {
		data = map[string]any{"offset": 0, "total": len(tasks), "task": tasks}
	}
	_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
}

func (f *fake) setTasks(tasks ...fakeTask) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tasks = tasks
}

// newClient returns a client of a definition talking to f, with settings
// over the fixture's.
func newClient(t *testing.T, def string, f *fake, settings map[string]any) client.Client {
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

const (
	torrentDef = "torrent-download-station"
	usenetDef  = "usenet-download-station"
)

// variants are the definitions and task API versions each case runs
// with.
var variants = []struct {
	def string
	v2  bool
}{{torrentDef, false}, {torrentDef, true}, {usenetDef, false}, {usenetDef, true}}

func variantName(def string, v2 bool) string {
	if v2 {
		return def + " v2"
	}
	return def + " v1"
}

// The tasks of Radarr's fixtures.
func fixtureTask(def, id string, status taskStatus, downloaded, uploaded, speed string) fakeTask {
	typ, uri := "BT", magnetURL
	transfer := map[string]string{"size_downloaded": downloaded, "speed_download": speed}
	if def == usenetDef {
		typ, uri = "NZB", movieTitle+".nzb"
	} else {
		transfer["size_uploaded"] = uploaded
	}
	return fakeTask{
		ID: id, Title: "title", Type: typ, Size: 1000, Status: status, Destination: "shared/folder", URI: uri, Transfer: transfer,
	}
}

func queued(def string) fakeTask { return fixtureTask(def, "id1", statusWaiting, "0", "0", "0") }

func completed(def string) fakeTask {
	return fixtureTask(def, "id2", statusFinished, "1000", "100", "0")
}

func seeding(def string) fakeTask { return fixtureTask(def, "id2", statusSeeding, "1000", "100", "0") }

func downloading(def string) fakeTask {
	return fixtureTask(def, "id3", statusDownloading, "100", "10", "50")
}

func failed(def string) fakeTask { return fixtureTask(def, "id4", statusError, "10", "1", "0") }

func items(t *testing.T, c client.Client) []client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), client.Options{RemoveCompletedDownloads: true})
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	return items
}

func TestAddDirectory(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		kind     client.Kind
		want     string
	}{
		// Download_with_TvDirectory_should_force_directory
		{"movie directory", map[string]any{"movie_directory": tvDirectory}, client.Movie, tvDirectory},
		{"series directory", map[string]any{"tv_directory": tvDirectory}, client.Series, tvDirectory},
		{"directory with a leading slash trimmed", map[string]any{"tv_directory": tvDirectory + "/"}, client.Series, tvDirectory + "/"},
		// Download_with_category_should_force_directory
		{"movie category", map[string]any{"movie_category": category}, client.Movie, "somepath/" + category},
		{"series category", map[string]any{"tv_category": category, "movie_category": "radarr"}, client.Series, "somepath/" + category},
		// Download_without_TvDirectory_and_Category_should_use_default
		{"default", nil, client.Movie, "somepath"},
		{"other kind's category", map[string]any{"tv_category": category}, client.Movie, "somepath"},
	}
	for _, v := range variants {
		for _, tt := range tests {
			t.Run(variantName(v.def, v.v2)+" "+tt.name, func(t *testing.T) {
				f := newFake(t, v.v2)
				c := newClient(t, v.def, f, tt.settings)
				r := &client.Release{Title: movieTitle, Kind: tt.kind}
				if v.def == torrentDef {
					r.MagnetURL = magnetURL
				} else {
					r.Data = []byte("<nzb/>")
				}
				id, err := c.Add(context.Background(), r)
				if err != nil {
					t.Fatalf("Add() error = %v", err)
				}
				if want := hashedSerial + ":dbid_101"; id != want {
					t.Errorf("Add() = %q, want = %q", id, want)
				}
				if len(f.created) != 1 {
					t.Fatalf("created = %+v, want one task", f.created)
				}
				got := f.created[0]
				dest := got.Params["destination"]
				if v.v2 && v.def == usenetDef {
					dest = strings.Trim(dest, `"`)
				}
				if dest != tt.want {
					t.Errorf("destination = %q, want = %q", got.Params["destination"], tt.want)
				}
				switch {
				case v.def == usenetDef && (got.FileName != movieTitle+".nzb" || got.Data != "<nzb/>"):
					t.Errorf("upload = %q %q, want the NZB as %s.nzb", got.FileName, got.Data, movieTitle)
				case v.def == torrentDef && !v.v2 && (got.Params["uri"] != magnetURL || got.Version != "3"):
					t.Errorf("create = %+v, want the magnet link with version 3", got)
				case v.def == torrentDef && v.v2 && (got.Params["url"] != magnetURL || got.Params["type"] != "url" || got.Params["create_list"] != "false"):
					t.Errorf("create = %+v, want the magnet link as a url", got)
				case v.v2 && v.def == usenetDef && (got.SID == "" || got.Params["type"] != `"file"` || got.Params["file"] != `["fileData"]`):
					t.Errorf("create = %+v, want a file with the session in the query", got)
				}
			})
		}
	}
}

func TestAddTorrentFile(t *testing.T) {
	data := []byte("d4:infod6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
	for _, v2 := range []bool{false, true} {
		f := newFake(t, v2)
		c := newClient(t, torrentDef, f, nil)
		id, err := c.Add(context.Background(), &client.Release{Title: movieTitle, Kind: client.Movie, Data: data})
		if err != nil {
			t.Fatalf("v2 %v: Add() error = %v", v2, err)
		}
		if want := hashedSerial + ":dbid_101"; id != want {
			t.Errorf("v2 %v: Add() = %q, want = %q", v2, id, want)
		}
		if got := f.created[0]; got.FileName != movieTitle+".torrent" || got.Data != string(data) {
			t.Errorf("v2 %v: upload = %q, want the torrent file", v2, got.FileName)
		}
		if v2 && f.created[0].Params["destination"] != `"somepath"` {
			t.Errorf("v2: destination = %q, want = %q", f.created[0].Params["destination"], `"somepath"`)
		}
	}
}

// TestAddDefaultLocation checks that task API v2 falls back on Download
// Station 2's default location without a default destination, as Radarr
// does.
func TestAddDefaultLocation(t *testing.T) {
	for _, def := range []string{torrentDef, usenetDef} {
		f := newFake(t, true)
		f.defaultDest = ""
		r := &client.Release{Title: movieTitle, MagnetURL: magnetURL, Data: []byte("<nzb/>")}
		if def == torrentDef {
			r.Data = nil
		}
		if _, err := newClient(t, def, f, nil).Add(context.Background(), r); err != nil {
			t.Fatalf("%s: Add() error = %v", def, err)
		}
		if got := f.created[0].Params["destination"]; got != `"location"` || f.locationCalls != 1 {
			t.Errorf("%s: destination = %q, want = %q", def, got, `"location"`)
		}
	}
}

func TestAddFailures(t *testing.T) {
	for _, v := range variants {
		// Download_should_throw_and_not_add_task_if_cannot_get_serial_number
		f := newFake(t, v.v2)
		f.serialFails = true
		r := &client.Release{Title: movieTitle, MagnetURL: magnetURL, Data: []byte("<nzb/>")}
		if v.def == torrentDef {
			r.Data = nil
		}
		if _, err := newClient(t, v.def, f, nil).Add(context.Background(), r); err == nil {
			t.Errorf("%s: Add() error = nil, want one", variantName(v.def, v.v2))
		}
		if len(f.created) != 0 {
			t.Errorf("%s: created = %+v, want none", variantName(v.def, v.v2), f.created)
		}
	}
}

func TestItemsFilter(t *testing.T) {
	for _, v := range variants {
		name := variantName(v.def, v.v2)
		// GetItems_should_return_empty_list_if_no_tasks_available
		f := newFake(t, v.v2)
		if got := items(t, newClient(t, v.def, f, nil)); len(got) != 0 {
			t.Errorf("%s: no tasks: Items() = %+v, want none", name, got)
		}
		// GetItems_should_return_ignore_tasks_of_unknown_type
		other := completed(v.def)
		other.Type = "ipfs"
		f.setTasks(other)
		if got := items(t, newClient(t, v.def, f, nil)); len(got) != 0 {
			t.Errorf("%s: unknown type: Items() = %+v, want none", name, got)
		}
		// The other protocol's tasks are not this client's.
		otherDef := torrentDef
		if v.def == torrentDef {
			otherDef = usenetDef
		}
		f.setTasks(completed(otherDef))
		if got := items(t, newClient(t, v.def, f, nil)); len(got) != 0 {
			t.Errorf("%s: other protocol: Items() = %+v, want none", name, got)
		}
		// GetItems_should_ignore_downloads_in_wrong_folder; Radarr's
		// "/shared/folder/sub" does not pass validation.
		f.setTasks(completed(v.def))
		if got := items(t, newClient(t, v.def, f, map[string]any{"movie_directory": "shared/folder/sub", "tv_directory": "shared/folder/sub"})); len(got) != 0 {
			t.Errorf("%s: wrong folder: Items() = %+v, want none", name, got)
		}
		// Failing to list tasks lists none, as Radarr.
		f.failTasks = true
		if got := items(t, newClient(t, v.def, f, nil)); len(got) != 0 {
			t.Errorf("%s: failing list: Items() = %+v, want none", name, got)
		}
	}
}

func TestItemsKinds(t *testing.T) {
	tests := []struct {
		name        string
		settings    map[string]any
		destination string
		want        int
		kind        client.Kind
		category    string
	}{
		{"movie category", map[string]any{"movie_category": "radarr", "tv_category": "sonarr"}, "shared/radarr", 1, client.Movie, "radarr"},
		{"series category", map[string]any{"movie_category": "radarr", "tv_category": "sonarr"}, "shared/sonarr/x", 1, client.Series, "sonarr"},
		{"neither category", map[string]any{"movie_category": "radarr", "tv_category": "sonarr"}, "shared/other", 0, 0, ""},
		{"series directory", map[string]any{"movie_directory": "video/Movies", "tv_directory": tvDirectory}, "video/Series/sub", 1, client.Series, ""},
		{"no filters", nil, "shared/other", 1, 0, ""},
		// A kind without a category or directory takes every other task,
		// as Radarr or Sonarr would by itself.
		{"one category", map[string]any{"tv_category": "sonarr"}, "shared/other", 1, client.Movie, ""},
		{"same category", map[string]any{"movie_category": "media", "tv_category": "media"}, "shared/media", 1, 0, "media"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t, false)
			task := downloading(torrentDef)
			task.Destination = tt.destination
			f.setTasks(task)
			got := items(t, newClient(t, torrentDef, f, tt.settings))
			if len(got) != tt.want {
				t.Fatalf("Items() = %d items, want = %d", len(got), tt.want)
			}
			if tt.want > 0 && (got[0].Kind != tt.kind || got[0].Category != tt.category) {
				t.Errorf("kind, category = %v, %q, want = %v, %q", got[0].Kind, got[0].Category, tt.kind, tt.category)
			}
		})
	}
}

func TestItemsFailures(t *testing.T) {
	for _, v := range variants {
		name := variantName(v.def, v.v2)
		all := []fakeTask{queued(v.def), completed(v.def), failed(v.def), downloading(v.def), seeding(v.def)}
		// GetItems_should_throw_if_shared_folder_resolve_fails
		f := newFake(t, v.v2)
		delete(f.shares, "/shared")
		f.setTasks(all...)
		if _, err := newClient(t, v.def, f, nil).Items(context.Background(), client.Options{}); err == nil {
			t.Errorf("%s: Items() without the shared folder error = nil, want one", name)
		}
		// GetItems_should_throw_if_serial_number_unavailable
		f = newFake(t, v.v2)
		f.serialFails = true
		f.setTasks(all...)
		if _, err := newClient(t, v.def, f, nil).Items(context.Background(), client.Options{}); err == nil {
			t.Errorf("%s: Items() without the serial number error = nil, want one", name)
		}
	}
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     []string
	}{
		// GetStatus_should_map_outputpath_when_using_default
		{"default", nil, []string{physical}},
		// GetStatus_should_map_outputpath_when_using_destination
		{"destination", map[string]any{"movie_directory": tvDirectory, "tv_directory": tvDirectory}, []string{"/mnt/sdb1/video/Series"}},
		// GetStatus_should_map_outputpath_when_using_category
		{"category", map[string]any{"movie_category": category, "tv_category": category}, []string{physical + "/" + category}},
		{"both kinds", map[string]any{"tv_category": category}, []string{physical, physical + "/" + category}},
	}
	for _, v := range variants {
		for _, tt := range tests {
			f := newFake(t, v.v2)
			info, err := newClient(t, v.def, f, tt.settings).Info(context.Background())
			if err != nil {
				t.Fatalf("%s %s: Info() error = %v", variantName(v.def, v.v2), tt.name, err)
			}
			if !slices.Equal(info.OutputRootFolders, tt.want) {
				t.Errorf("%s %s: OutputRootFolders = %v, want = %v", variantName(v.def, v.v2), tt.name, info.OutputRootFolders, tt.want)
			}
		}
	}
	// IsLocalhost is from the settings.
	f := newFake(t, false)
	if info, err := newClient(t, torrentDef, f, nil).Info(context.Background()); err != nil || !info.IsLocalhost {
		t.Errorf("Info() = %+v, %v, want IsLocalhost", info, err)
	}
}

func TestOutputPath(t *testing.T) {
	file := func(id string, status taskStatus, title string) fakeTask {
		t := fixtureTask(torrentDef, id, status, "1000", "100", "0")
		t.Title = title
		return t
	}
	tests := []struct {
		name string
		task fakeTask
		want string
	}{
		// GetItems_should_set_outputPath_to_base_folder_when_single_file_non_finished_tasks
		{"single file seeding", file("id5", statusSeeding, "a.mkv"), physical + "/folder/a.mkv"},
		// GetItems_should_set_outputPath_to_torrent_folder_when_multiple_files_non_finished_tasks
		{"multiple files seeding", file("id6", statusSeeding, "title"), physical + "/folder/title"},
		// GetItems_should_set_outputPath_to_base_folder_when_single_file_finished_tasks
		{"single file finished", file("id6", statusFinished, "a.mkv"), physical + "/folder/a.mkv"},
		// GetItems_should_set_outputPath_to_torrent_folder_when_multiple_files_finished_tasks
		{"multiple files finished", file("id6", statusFinished, "title"), physical + "/folder/title"},
	}
	for _, v2 := range []bool{false, true} {
		for _, tt := range tests {
			f := newFake(t, v2)
			f.setTasks(tt.task)
			got := items(t, newClient(t, torrentDef, f, nil))
			if len(got) != 1 || got[0].OutputPath != tt.want {
				t.Errorf("v2 %v %s: Items() = %+v, want OutputPath = %q", v2, tt.name, got, tt.want)
			}
		}
	}
}

func TestOutputPathMapped(t *testing.T) {
	for _, v := range variants {
		name := variantName(v.def, v.v2)
		// GetItems_should_not_map_outputpath_for_queued_or_downloading_tasks
		f := newFake(t, v.v2)
		f.setTasks(queued(v.def), downloading(v.def))
		got := items(t, newClient(t, v.def, f, nil))
		if len(got) != 2 || got[0].OutputPath != "" || got[1].OutputPath != "" {
			t.Errorf("%s: queued and downloading: Items() = %+v, want 2 without OutputPath", name, got)
		}
		// GetItems_should_map_outputpath_for_completed_or_failed_tasks
		f.setTasks(completed(v.def), failed(v.def), seeding(v.def))
		got = items(t, newClient(t, v.def, f, nil))
		if len(got) != 3 {
			t.Fatalf("%s: Items() = %d items, want 3", name, len(got))
		}
		for _, it := range got {
			if it.OutputPath != physical+"/folder/title" {
				t.Errorf("%s: OutputPath = %q, want = %q", name, it.OutputPath, physical+"/folder/title")
			}
		}
	}
}

func TestCanBeRemoved(t *testing.T) {
	tests := []struct {
		status             taskStatus
		movable, removable bool
	}{
		{statusDownloading, false, false},
		{statusFinished, true, true},
		{statusSeeding, false, false},
		{statusWaiting, false, false},
	}
	for _, v2 := range []bool{false, true} {
		for _, tt := range tests {
			f := newFake(t, v2)
			task := queued(torrentDef)
			task.Status = tt.status
			f.setTasks(task)
			got := items(t, newClient(t, torrentDef, f, nil))
			if len(got) != 1 || got[0].CanMoveFiles != tt.movable || got[0].CanBeRemoved != tt.removable {
				t.Errorf("v2 %v %v: Items() = %+v, want CanMoveFiles, CanBeRemoved = %v, %v", v2, tt.status, got, tt.movable, tt.removable)
			}
		}
	}
	// Only when completed downloads are to be removed.
	f := newFake(t, false)
	f.setTasks(completed(torrentDef))
	got, err := newClient(t, torrentDef, f, nil).Items(context.Background(), client.Options{})
	if err != nil || len(got) != 1 || got[0].CanBeRemoved || got[0].CanMoveFiles {
		t.Errorf("Items() without removing = %+v, %v, want not removable", got, err)
	}
	// NZBs can always be removed, as Radarr's.
	f.setTasks(downloading(usenetDef))
	got, err = newClient(t, usenetDef, f, nil).Items(context.Background(), client.Options{})
	if err != nil || len(got) != 1 || !got[0].CanBeRemoved || !got[0].CanMoveFiles {
		t.Errorf("Items() of NZBs = %+v, %v, want removable", got, err)
	}
}

func TestItemStatus(t *testing.T) {
	tests := []struct {
		status taskStatus
		want   client.Status
	}{
		{statusDownloading, client.Downloading},
		{statusError, client.Failed},
		{statusExtracting, client.Downloading},
		{statusFinished, client.Completed},
		{statusFinishing, client.Downloading},
		{statusHashChecking, client.Downloading},
		{statusCaptchaNeeded, client.Downloading},
		{statusPaused, client.Paused},
		{statusSeeding, client.Completed},
		{statusFilehostingWaiting, client.Queued},
		{statusWaiting, client.Queued},
		{statusUnknown, client.Queued},
	}
	for _, v := range variants {
		for _, tt := range tests {
			f := newFake(t, v.v2)
			task := queued(v.def)
			task.Status = tt.status
			f.setTasks(task)
			got := items(t, newClient(t, v.def, f, nil))
			if len(got) != 1 || got[0].Status != tt.want {
				t.Errorf("%s %v: Items() = %+v, want Status = %v", variantName(v.def, v.v2), tt.status, got, tt.want)
				continue
			}
			if want := hashedSerial + ":id1"; got[0].ID != want || got[0].Title != "title" || got[0].TotalSize != 1000 || got[0].RemainingSize != 1000 {
				t.Errorf("%s %v: item = %+v, want the task's ID, title and sizes", variantName(v.def, v.v2), tt.status, got[0])
			}
		}
	}
	// A waiting task with nothing left is completed.
	f := newFake(t, false)
	task := completed(torrentDef)
	task.Status = statusWaiting
	f.setTasks(task)
	if got := items(t, newClient(t, torrentDef, f, nil)); got[0].Status != client.Completed {
		t.Errorf("waiting and downloaded: Status = %v, want = %v", got[0].Status, client.Completed)
	}
}

func TestMessage(t *testing.T) {
	f := newFake(t, false)
	extracting, errored := queued(torrentDef), failed(torrentDef)
	extracting.Status, extracting.StatusExtra = statusExtracting, map[string]string{"unzip_progress": "42"}
	errored.StatusExtra = map[string]string{"error_detail": "broken_link"}
	f.setTasks(extracting, errored)
	got := items(t, newClient(t, torrentDef, f, nil))
	if got[0].Message != "Extracting: 42%" || got[1].Message != "broken_link" {
		t.Errorf("messages = %q, %q, want = %q, %q", got[0].Message, got[1].Message, "Extracting: 42%", "broken_link")
	}
}

func TestRemainingTimeAndRatio(t *testing.T) {
	// Torrents: their own speed.
	f := newFake(t, false)
	f.setTasks(downloading(torrentDef), completed(torrentDef))
	got := items(t, newClient(t, torrentDef, f, nil))
	if got[0].RemainingTime == nil || *got[0].RemainingTime != 18*time.Second || got[1].RemainingTime != nil {
		t.Errorf("RemainingTime = %v, %v, want = 18s, nil", got[0].RemainingTime, got[1].RemainingTime)
	}
	if got[0].SeedRatio == nil || *got[0].SeedRatio != 0.1 {
		t.Errorf("SeedRatio = %v, want = 0.1", got[0].SeedRatio)
	}
	// NZBs: the speed of all, after those ahead; none while paused.
	paused := queued(usenetDef)
	paused.Status = statusPaused
	f.setTasks(downloading(usenetDef), queued(usenetDef), paused)
	got = items(t, newClient(t, usenetDef, f, nil))
	if got[0].RemainingTime == nil || *got[0].RemainingTime != 18*time.Second ||
		got[1].RemainingTime == nil || *got[1].RemainingTime != 38*time.Second || got[2].RemainingTime != nil {
		t.Errorf("RemainingTime = %v, %v, %v, want = 18s, 38s, nil", got[0].RemainingTime, got[1].RemainingTime, got[2].RemainingTime)
	}
	if got[0].SeedRatio != nil {
		t.Errorf("SeedRatio of an NZB = %v, want = nil", *got[0].SeedRatio)
	}
}

func TestRemove(t *testing.T) {
	for _, v2 := range []bool{false, true} {
		f := newFake(t, v2)
		f.setTasks(completed(torrentDef))
		c := newClient(t, torrentDef, f, nil)
		item := items(t, c)[0]
		if err := c.Remove(context.Background(), item, false); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		want := "SYNO.DownloadStation.Task 1 id2"
		if v2 {
			want = "SYNO.DownloadStation2.Task 2 id2"
		}
		if len(f.removed) != 1 || f.removed[0] != want {
			t.Errorf("removed = %v, want = [%s]", f.removed, want)
		}
		if err := c.Remove(context.Background(), client.Item{ID: "id2"}, false); err == nil {
			t.Errorf("Remove() of an ID without serial number error = nil, want one")
		}
	}
}

func TestStatusJSON(t *testing.T) {
	tests := []struct {
		value string
		want  taskStatus
	}{
		{"captcha_needed", statusCaptchaNeeded},
		{"filehosting_waiting", statusFilehostingWaiting},
		{"hash_checking", statusHashChecking},
		{"error", statusError},
		{"downloading", statusDownloading},
	}
	for _, tt := range tests {
		// should_parse_enum_correctly
		var got task
		if err := json.Unmarshal([]byte(`{"Status": "`+tt.value+`"}`), &got); err != nil || got.Status != tt.want {
			t.Errorf("unmarshal %q = %v, %v, want = %v", tt.value, got.Status, err, tt.want)
		}
		// should_serialize_enum_correctly
		data, err := json.Marshal(task{Status: tt.want})
		if err != nil || !strings.Contains(string(data), tt.value) {
			t.Errorf("marshal %v = %s, %v, want it to contain %q", tt.want, data, err, tt.value)
		}
	}
	// should_return_unknown_if_unknown_enum_value
	var got task
	if err := json.Unmarshal([]byte(`{"Status": "some_unknown_value"}`), &got); err != nil || got.Status != statusUnknown {
		t.Errorf("unmarshal some_unknown_value = %v, %v, want = %v", got.Status, err, statusUnknown)
	}
	// Task API v2 numbers statuses.
	if err := json.Unmarshal([]byte(`{"status": 8}`), &got); err != nil || got.Status != statusFilehostingWaiting {
		t.Errorf("unmarshal 8 = %v, %v, want = %v", got.Status, err, statusFilehostingWaiting)
	}
}

func TestSerialNumber(t *testing.T) {
	// should_return_hashedserialnumber: this hash should remain the same
	// for "serial".
	if got := hashSerial("serial"); got != "50DE66B735D30738618568294742FCF1DFA52A47" {
		t.Errorf("hashSerial(serial) = %q, want = 50DE66B735D30738618568294742FCF1DFA52A47", got)
	}
	f := newFake(t, false)
	f.serial = "serial"
	c := newClient(t, torrentDef, f, nil).(*torrentStation)
	// should_cache_serialnumber
	s1, err1 := c.serialNumber(context.Background())
	s2, err2 := c.serialNumber(context.Background())
	if err1 != nil || err2 != nil || s1 != "50DE66B735D30738618568294742FCF1DFA52A47" || s2 != s1 || f.serialCalls != 1 {
		t.Errorf("serialNumber() = %q, %q (%v, %v) with %d calls, want the hash with one call", s1, s2, err1, err2, f.serialCalls)
	}
	// should_throw_if_serial_number_unavailable
	f = newFake(t, false)
	f.serialFails = true
	c = newClient(t, torrentDef, f, nil).(*torrentStation)
	if _, err := c.serialNumber(context.Background()); err == nil {
		t.Errorf("serialNumber() error = nil, want one")
	}
}

func TestSharedFolderResolver(t *testing.T) {
	f := newFake(t, false)
	f.shares = map[string]string{"/myFolder": "/mnt/sda1/folder"}
	c := newClient(t, torrentDef, f, nil).(*torrentStation)
	ctx := context.Background()
	// should_throw_when_cannot_resolve_shared_folder
	if _, err := c.fullPath(ctx, "/unknownFolder", "SERIALNUMBER"); err == nil {
		t.Errorf("fullPath(/unknownFolder) error = nil, want one")
	}
	f.fileCalls = 0
	// should_return_valid_sharedfolder
	if got, err := c.fullPath(ctx, "/myFolder", "abc"); err != nil || got != "/mnt/sda1/folder" || f.fileCalls != 1 {
		t.Errorf("fullPath(/myFolder) = %q, %v with %d calls, want = /mnt/sda1/folder with one", got, err, f.fileCalls)
	}
	// should_cache_mapping
	if _, err := c.fullPath(ctx, "/myFolder", "abc"); err != nil || f.fileCalls != 1 {
		t.Errorf("fullPath(/myFolder) again: %v with %d calls, want one call", err, f.fileCalls)
	}
	// should_remap_subfolder
	if got, err := c.fullPath(ctx, "/myFolder/sub", "abc"); err != nil || got != "/mnt/sda1/folder/sub" {
		t.Errorf("fullPath(/myFolder/sub) = %q, %v, want = /mnt/sda1/folder/sub", got, err)
	}
}

func TestSession(t *testing.T) {
	f := newFake(t, true)
	f.setTasks(queued(torrentDef))
	c := newClient(t, torrentDef, f, nil)
	items(t, c)
	items(t, c)
	if f.logins != 1 {
		t.Errorf("logins = %d, want = 1", f.logins)
	}
	// A timed-out session lists no tasks, as Radarr, and is renewed by
	// the next call.
	f.expireSession = errCodeTimeout
	if got, err := c.Items(context.Background(), client.Options{}); err != nil || len(got) != 0 {
		t.Errorf("Items() with an expired session = %+v, %v, want none", got, err)
	}
	items(t, c)
	if f.logins != 2 {
		t.Errorf("logins = %d, want = 2", f.logins)
	}
	// A session without permission is an AuthError.
	f.expireSession = errCodePermission
	var auth *client.AuthError
	if err := c.Remove(context.Background(), client.Item{ID: "x:id1"}, false); !errors.As(err, &auth) {
		t.Errorf("Remove() without permission error = %v, want an AuthError", err)
	}
	// A wrong password is an AuthError.
	bad := newClient(t, torrentDef, f, map[string]any{"password": "wrong"})
	if _, err := bad.Items(context.Background(), client.Options{}); !errors.As(err, &auth) {
		t.Errorf("Items() with a wrong password error = %v, want an AuthError", err)
	}
}

func TestTest(t *testing.T) {
	tests := []struct {
		name     string
		setup    func(f *fake)
		settings map[string]any
		want     string
	}{
		{"ok", nil, nil, ""},
		{"ok with directories", nil, map[string]any{"movie_directory": tvDirectory, "tv_directory": "somepath/sonarr"}, ""},
		{"api version", func(f *fake) { f.taskMin = 3; f.taskMax = 3 }, nil, "Download Station API version not supported, should be at least 2. It supports from 3 to 3"},
		{"no default destination", func(f *fake) { f.defaultDest = "" }, nil, "no default destination"},
		{"shared folder missing", nil, map[string]any{"tv_directory": "nowhere/tv"}, `the Diskstation does not have a shared folder named "nowhere"`},
		{"folder missing", nil, map[string]any{"tv_category": "missing"}, `the folder "somepath/missing" must be created manually inside the shared folder "somepath"`},
		{"auth", nil, map[string]any{"password": "wrong"}, "authentication failed: verify the username and password"},
	}
	for _, v := range variants {
		for _, tt := range tests {
			f := newFake(t, v.v2)
			if tt.setup != nil {
				tt.setup(f)
			}
			err := newClient(t, v.def, f, tt.settings).Test(context.Background())
			switch {
			case tt.want == "" && err != nil:
				t.Errorf("%s %s: Test() error = %v, want nil", variantName(v.def, v.v2), tt.name, err)
			case tt.want != "" && (err == nil || !strings.Contains(err.Error(), tt.want)):
				t.Errorf("%s %s: Test() error = %v, want %q", variantName(v.def, v.v2), tt.name, err, tt.want)
			}
		}
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"defaults", `{}`, true},
		{"directory", `{"tv_directory":"video/Series"}`, true},
		{"directory with a leading slash", `{"tv_directory":"/video/Series"}`, false},
		{"bad category", `{"movie_category":"radarr_1"}`, false},
		{"category and directory", `{"tv_category":"tv","tv_directory":"video"}`, false},
		{"unknown", `{"bogus":1}`, false},
		{"no host", `{"host":""}`, false},
	}
	for _, tt := range tests {
		for _, d := range Definitions() {
			_, err := d.New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
			if (err == nil) != tt.ok {
				t.Errorf("%s %s: New() error = %v, want ok = %v", d.ID, tt.name, err, tt.ok)
			}
		}
	}
}

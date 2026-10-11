package deluge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
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

const title = "Droned.1998.1080p.WEB-DL-DRONE"

// fake is Deluge's web UI JSON-RPC.
type fake struct {
	t  *testing.T
	mu sync.Mutex
	// torrents are web.update_ui's; a "label" key gives the label.
	torrents  []map[string]any
	config    map[string]any
	methods   []string
	labels    []string
	labelOpts map[string]map[string]any
	// refuseLabels makes label.add do nothing.
	refuseLabels bool
	hosts        [][]any
	connected    bool
	connectedTo  any
	session      string
	logins       int
	addHash      string
	calls        []call
}

type call struct {
	Method string
	Params []any
}

func newFake(t *testing.T) *fake {
	return &fake{
		t: t, connected: true, addHash: "cbc2f069fe8bb2f544eae707d75bcd3de9dcf951",
		config: map[string]any{},
		methods: []string{
			"system.listMethods", "daemon.get_version", "core.get_config", "label.get_labels", "label.add",
			"label.set_torrent", "label.get_options",
		},
		labelOpts: map[string]map[string]any{},
		hosts:     [][]any{{"other", "10.0.0.2", 58846, "Online"}, {"local", "127.0.0.1", 58846, "Online"}},
	}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/json" || r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Method string `json:"method"`
		Params []any  `json:"params"`
		ID     any    `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		f.t.Errorf("request body: %v", err)
	}
	reply := func(result any, rpcErr any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"id": req.ID, "result": result, "error": rpcErr})
	}
	if req.Method == "auth.login" {
		f.logins++
		if req.Params[0] != "deluge" {
			reply(false, nil)
			return
		}
		f.session = "s" + strconv.Itoa(f.logins)
		http.SetCookie(w, &http.Cookie{Name: "_session_id", Value: f.session})
		reply(true, nil)
		return
	}
	if c, err := r.Cookie("_session_id"); err != nil || c.Value != f.session {
		reply(nil, map[string]any{"message": "Not authenticated", "code": 1})
		return
	}
	if !f.connected && !slices.Contains([]string{"web.connected", "web.get_hosts", "web.connect", "web.disconnect"}, req.Method) {
		reply(nil, map[string]any{"message": "Not connected", "code": 2})
		return
	}
	f.calls = append(f.calls, call{req.Method, req.Params})
	switch req.Method {
	case "web.connected":
		reply(f.connected, nil)
	case "web.get_hosts":
		reply(f.hosts, nil)
	case "web.connect":
		f.connected, f.connectedTo = true, req.Params[0]
		reply(nil, nil)
	case "web.disconnect":
		f.connected = false
		reply(true, nil)
	case "system.listMethods":
		reply(f.methods, nil)
	case "daemon.get_version":
		reply("2.1.1", nil)
	case "daemon.info":
		reply("1.3.15", nil)
	case "core.get_config":
		reply(f.config, nil)
	case "label.get_labels":
		reply(f.labels, nil)
	case "label.add":
		if !f.refuseLabels {
			f.labels = append(f.labels, req.Params[0].(string))
		}
		reply(nil, nil)
	case "label.get_options":
		reply(f.labelOpts[req.Params[0].(string)], nil)
	case "core.add_torrent_magnet", "core.add_torrent_file":
		reply(f.addHash, nil)
	case "web.update_ui":
		f.updateUI(w, req.ID, req.Params)
	default:
		reply(true, nil)
	}
}

// updateUI answers web.update_ui with the torrents of the label filter,
// by hash in order.
func (f *fake) updateUI(w http.ResponseWriter, id any, params []any) {
	label, _ := params[1].(map[string]any)["label"].(string)
	var b bytes.Buffer
	b.WriteString(`{"connected":true,"stats":{},"torrents":{`)
	n := 0
	for i, t := range f.torrents {
		if label != "" && t["label"] != label {
			continue
		}
		if n > 0 {
			b.WriteByte(',')
		}
		n++
		key, _ := t["hash"].(string)
		if key == "" {
			key = "nohash" + strconv.Itoa(i)
		}
		k, _ := json.Marshal(key)
		v, _ := json.Marshal(t)
		fmt.Fprintf(&b, "%s:%s", k, v)
	}
	b.WriteString("}}")
	idJSON, _ := json.Marshal(id)
	fmt.Fprintf(w, `{"id":%s,"result":%s,"error":null}`, idJSON, b.String())
}

// methodCalls returns the calls of a method, or of all for "".
func (f *fake) methodCalls(method string) []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		if method == "" || c.Method == method {
			out = append(out, c)
		}
	}
	return out
}

// newClient returns a client talking to f, with settings over the
// fixture's, which has no categories as Radarr's.
func newClient(t *testing.T, f *fake, settings map[string]any) client.Client {
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
	return c
}

// The torrents of Radarr's DelugeFixture.
func queued() map[string]any {
	return map[string]any{
		"hash": "HASH", "is_finished": false, "state": stateQueued, "name": title,
		"total_size": 1000, "total_done": 0, "progress": 0.0, "save_path": "somepath",
	}
}

func downloading() map[string]any {
	return map[string]any{
		"hash": "HASH", "is_finished": false, "state": stateDownloading, "name": title,
		"total_size": 1000, "total_done": 100, "progress": 10.0, "save_path": "somepath",
	}
}

func failed() map[string]any {
	return map[string]any{
		"hash": "HASH", "is_finished": false, "state": stateError, "name": title,
		"total_size": 1000, "total_done": 100, "progress": 10.0, "message": "Error", "save_path": "somepath",
	}
}

func completed() map[string]any {
	return map[string]any{
		"hash": "HASH", "is_finished": true, "state": statePaused, "name": title,
		"total_size": 1000, "total_done": 1000, "progress": 100.0, "save_path": "somepath",
		"is_auto_managed": true, "stop_at_ratio": true, "stop_ratio": 1.0, "ratio": 1.5,
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

// TestRequiredProperties ports the *_item_should_have_required_properties
// cases.
func TestRequiredProperties(t *testing.T) {
	tests := []struct {
		name    string
		torrent map[string]any
		want    client.Status
	}{
		{"queued", queued(), client.Queued},
		{"downloading", downloading(), client.Downloading},
		{"failed", failed(), client.Warning},
		{"completed", completed(), client.Completed},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.torrents = []map[string]any{tt.torrent}
			item := single(t, newClient(t, f, nil))
			if item.Status != tt.want {
				t.Errorf("Status = %v, want = %v", item.Status, tt.want)
			}
			if item.ID != "HASH" || item.Title != title || item.TotalSize != 1000 {
				t.Errorf("item = %+v, want the torrent's hash, name and size", item)
			}
			if got, want := item.OutputPath, "somepath/"+title; got != want {
				t.Errorf("OutputPath = %q, want = %q", got, want)
			}
			switch tt.want {
			case client.Completed:
				if item.RemainingSize != 0 || item.RemainingTime == nil || *item.RemainingTime != 0 {
					t.Errorf("RemainingSize, RemainingTime = %d, %v, want = 0, 0", item.RemainingSize, item.RemainingTime)
				}
				if !item.CanBeRemoved || !item.CanMoveFiles {
					t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = true, true", item.CanBeRemoved, item.CanMoveFiles)
				}
			case client.Warning:
				if item.Message == "" {
					t.Errorf("Message is empty, want Deluge's error")
				}
			default:
				if item.RemainingSize == 0 {
					t.Errorf("RemainingSize = 0, want non-zero")
				}
			}
		})
	}
}

func TestItemStatus(t *testing.T) {
	tests := []struct {
		name    string
		torrent map[string]any
		state   string
		want    client.Status
	}{
		// GetItems_should_return_queued_item_as_downloadItemStatus
		{"queued", queued(), statePaused, client.Paused},
		{"queued", queued(), stateChecking, client.Downloading},
		{"queued", queued(), stateQueued, client.Queued},
		{"queued", queued(), stateDownloading, client.Downloading},
		{"queued", queued(), stateSeeding, client.Downloading},
		// GetItems_should_return_downloading_item_as_downloadItemStatus
		{"downloading", downloading(), statePaused, client.Paused},
		{"downloading", downloading(), stateChecking, client.Downloading},
		{"downloading", downloading(), stateQueued, client.Queued},
		{"downloading", downloading(), stateDownloading, client.Downloading},
		{"downloading", downloading(), stateSeeding, client.Downloading},
		// GetItems_should_return_completed_item_as_downloadItemStatus
		{"completed", completed(), statePaused, client.Completed},
		{"completed", completed(), stateChecking, client.Downloading},
		{"completed", completed(), stateQueued, client.Completed},
		{"completed", completed(), stateSeeding, client.Completed},
	}
	for _, tt := range tests {
		t.Run(tt.name+" "+tt.state, func(t *testing.T) {
			f := newFake(t)
			tt.torrent["state"] = tt.state
			f.torrents = []map[string]any{tt.torrent}
			if got := single(t, newClient(t, f, nil)).Status; got != tt.want {
				t.Errorf("Status = %v, want = %v", got, tt.want)
			}
		})
	}
}

// TestShareRatio ports GetItems_should_check_share_ratio_for_moveFiles_and_remove.
func TestShareRatio(t *testing.T) {
	tests := []struct {
		ratio     float64
		removable bool
		opts      client.Options
	}{
		{0.5, false, client.Options{RemoveCompletedDownloads: true}},
		{1.01, true, client.Options{RemoveCompletedDownloads: true}},
		{1.01, false, client.Options{}},
	}
	for _, tt := range tests {
		f := newFake(t)
		tor := completed()
		tor["ratio"] = tt.ratio
		f.torrents = []map[string]any{tor}
		got, err := newClient(t, f, nil).Items(context.Background(), tt.opts)
		if err != nil || len(got) != 1 {
			t.Fatalf("Items() = %v, %v, want 1 item", got, err)
		}
		item := got[0]
		if item.Status != client.Completed || item.CanBeRemoved != tt.removable || item.CanMoveFiles != tt.removable {
			t.Errorf("ratio %v: Status, CanBeRemoved, CanMoveFiles = %v, %v, %v, want = %v, %v, %v",
				tt.ratio, item.Status, item.CanBeRemoved, item.CanMoveFiles, client.Completed, tt.removable, tt.removable)
		}
	}
}

// TestIgnoreWithoutHash ports GetItems_should_ignore_items_without_hash;
// the first time, the web UI reconnects to the daemon.
func TestIgnoreWithoutHash(t *testing.T) {
	f := newFake(t)
	tor := downloading()
	tor["hash"] = nil
	f.torrents = []map[string]any{tor, queued()}
	c := newClient(t, f, nil)
	for i := range 2 {
		got := items(t, c)
		if len(got) != 1 || got[0].Status != client.Queued {
			t.Fatalf("Items() = %+v, want the queued torrent alone", got)
		}
		if n := len(f.methodCalls("web.disconnect")); n != 1 {
			t.Errorf("after %d listings: web.disconnect calls = %d, want = 1", i+1, n)
		}
	}
}

func TestDownload(t *testing.T) {
	data := []byte("d4:infod6:lengthi1e4:name1:a12:piece lengthi16384e6:pieces20:aaaaaaaaaaaaaaaaaaaaee")
	tests := []struct {
		name     string
		settings map[string]any
		release  client.Release
		want     string
		// wantCalls are the calls after adding.
		wantCalls []string
		wantOpts  map[string]any
	}{
		// Download_should_return_unique_id
		{
			"torrent file", nil,
			client.Release{Kind: client.Movie, Data: data},
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951", nil,
			map[string]any{"add_paused": false, "remove_at_ratio": false},
		},
		// Download_should_get_hash_from_magnet_url
		{
			"magnet", nil,
			client.Release{Kind: client.Movie, MagnetURL: "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR&tr=udp"},
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951", nil, nil,
		},
		{
			"series label, seeding, first and directories",
			map[string]any{
				"movie_category": "radarr", "tv_category": "tv-sonarr", "recent_tv_priority": priorityFirst,
				"tv_directory": "/downloads/tv", "tv_completed_directory": "/done/tv", "add_paused": true,
			},
			client.Release{Kind: client.Series, Recent: true, Data: data, SeedRatio: ptr(2.5)},
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951",
			[]string{"core.set_torrent_options", "label.set_torrent", "core.queue_top"},
			map[string]any{
				"add_paused": true, "remove_at_ratio": false, "download_location": "/downloads/tv",
				"move_completed_path": "/done/tv", "move_completed": true,
			},
		},
		{
			"older movie stays last",
			map[string]any{"movie_category": "radarr", "recent_movie_priority": priorityFirst},
			client.Release{Kind: client.Movie, Data: data},
			"CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951",
			[]string{"label.set_torrent"},
			nil,
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
			if id != tt.want {
				t.Errorf("Add() = %q, want = %q", id, tt.want)
			}
			calls := f.methodCalls("")
			i := slices.IndexFunc(calls, func(c call) bool { return strings.HasPrefix(c.Method, "core.add_torrent_") })
			if i < 0 {
				t.Fatalf("calls = %+v, want a torrent added", calls)
			}
			add := calls[i]
			var rest []string
			for _, c := range calls[i+1:] {
				rest = append(rest, c.Method)
			}
			if !slices.Equal(rest, tt.wantCalls) {
				t.Errorf("calls after adding = %v, want = %v", rest, tt.wantCalls)
			}
			if r.MagnetURL != "" {
				if add.Method != "core.add_torrent_magnet" || add.Params[0] != r.MagnetURL {
					t.Errorf("add = %+v, want the magnet link", add)
				}
			} else if add.Method != "core.add_torrent_file" || add.Params[0] != title+".torrent" || add.Params[1] != base64.StdEncoding.EncodeToString(data) {
				t.Errorf("add = %s %v, want the torrent file", add.Method, add.Params[:1])
			}
			if tt.wantOpts != nil {
				got, _ := json.Marshal(add.Params[len(add.Params)-1])
				want, _ := json.Marshal(tt.wantOpts)
				if string(got) != string(want) {
					t.Errorf("options = %s, want = %s", got, want)
				}
			}
			for _, c := range calls[i+1:] {
				var want string
				switch c.Method {
				case "core.set_torrent_options":
					want = `[["cbc2f069fe8bb2f544eae707d75bcd3de9dcf951"],{"stop_at_ratio":1,"stop_ratio":2.5}]`
				case "label.set_torrent":
					want = `["cbc2f069fe8bb2f544eae707d75bcd3de9dcf951","` + tt.settings[map[client.Kind]string{client.Movie: "movie_category", client.Series: "tv_category"}[r.Kind]].(string) + `"]`
				case "core.queue_top":
					want = `[["cbc2f069fe8bb2f544eae707d75bcd3de9dcf951"]]`
				}
				if got, _ := json.Marshal(c.Params); string(got) != want {
					t.Errorf("%s params = %s, want = %s", c.Method, got, want)
				}
			}
		})
	}

	f := newFake(t)
	f.addHash = ""
	r := client.Release{Title: title, MagnetURL: "magnet:?xt=urn:btih:ZPBPA2P6ROZPKRHK44D5OW6NHXU5Z6KR"}
	if _, err := newClient(t, f, nil).Add(context.Background(), &r); err == nil {
		t.Errorf("Add() with no hash returned error = nil, want an error")
	}
}

func ptr[T any](v T) *T { return &v }

func TestItemsByLabel(t *testing.T) {
	tor := func(hash, label string) map[string]any {
		m := downloading()
		m["hash"], m["label"] = hash, label
		return m
	}
	torrents := []map[string]any{tor("A", "radarr"), tor("B", "tv-sonarr"), tor("C", "other"), tor("D", "")}
	tests := []struct {
		name     string
		settings map[string]any
		want     map[string]client.Kind
	}{
		{
			"both labels",
			map[string]any{"movie_category": "radarr", "tv_category": "tv-sonarr"},
			map[string]client.Kind{"A": client.Movie, "B": client.Series},
		},
		{
			"one label",
			map[string]any{"movie_category": "radarr"},
			map[string]client.Kind{"A": 0, "B": client.Series, "C": client.Series, "D": client.Series},
		},
		{
			"same label",
			map[string]any{"movie_category": "radarr", "tv_category": "radarr"},
			map[string]client.Kind{"A": 0},
		},
		{"no labels", nil, map[string]client.Kind{"A": 0, "B": 0, "C": 0, "D": 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.torrents = torrents
			got := map[string]client.Kind{}
			for _, item := range items(t, newClient(t, f, tt.settings)) {
				got[item.ID] = item.Kind
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("Items() kinds = %v, want = %v", got, tt.want)
			}
		})
	}
}

func TestInfo(t *testing.T) {
	tests := []struct {
		name      string
		settings  map[string]any
		config    map[string]any
		labelOpts map[string]map[string]any
		want      []string
	}{
		// should_return_status_with_outputdirs
		{
			"move completed", nil,
			map[string]any{"download_location": "C:/Downloads/Downloading/deluge", "move_completed_path": "C:/Downloads/Finished/deluge", "move_completed": true},
			nil,
			[]string{"C:/Downloads/Finished/deluge"},
		},
		{
			"download location", nil,
			map[string]any{"download_location": "C:/Downloads/Downloading/deluge", "move_completed_path": "C:/Downloads/Finished/deluge", "move_completed": false},
			nil,
			[]string{"C:/Downloads/Downloading/deluge"},
		},
		// should_return_status_with_outputdirs_for_directories_in_settings
		{
			"directories in settings",
			map[string]any{
				"movie_directory": "D:/Downloads/Downloading/deluge", "movie_completed_directory": "D:/Downloads/Finished/deluge",
				"tv_directory": "D:/Downloads/Downloading/deluge", "tv_completed_directory": "D:/Downloads/Finished/deluge",
			},
			nil, nil,
			[]string{"D:/Downloads/Finished/deluge"},
		},
		{
			"label move completed",
			map[string]any{"movie_category": "radarr", "tv_directory": "/tv"},
			map[string]any{"download_location": "/downloads"},
			map[string]map[string]any{"radarr": {"apply_move_completed": true, "move_completed": true, "move_completed_path": "/done/radarr"}},
			[]string{"/done/radarr", "/tv"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			f.config = tt.config
			if tt.labelOpts != nil {
				f.labelOpts = tt.labelOpts
			}
			s := map[string]any{"host": "127.0.0.1"}
			for k, v := range tt.settings {
				s[k] = v
			}
			srv := httptest.NewServer(f)
			defer srv.Close()
			_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
			s["port"], _ = strconv.Atoi(port)
			if _, ok := s["movie_category"]; !ok {
				s["movie_category"] = ""
			}
			s["tv_category"] = ""
			raw, _ := json.Marshal(s)
			c, err := Definitions()[0].New(raw, client.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)})
			if err != nil {
				t.Fatal(err)
			}
			info, err := c.Info(context.Background())
			if err != nil {
				t.Fatalf("Info() error = %v", err)
			}
			if !info.IsLocalhost || !slices.Equal(info.OutputRootFolders, tt.want) {
				t.Errorf("Info() = %+v, want localhost with %v", info, tt.want)
			}
		})
	}
}

func TestSessionAndDaemon(t *testing.T) {
	f := newFake(t)
	f.connected = false
	c := newClient(t, f, nil)
	if _, err := c.Items(context.Background(), client.Options{}); err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	if f.connectedTo != "local" {
		t.Errorf("connected to host %v, want = local, the one on 127.0.0.1", f.connectedTo)
	}
	// The session expires: the client logs in again.
	f.session = "expired"
	if _, err := c.Items(context.Background(), client.Options{}); err != nil {
		t.Fatalf("Items() after the session expired error = %v", err)
	}
	if f.logins != 2 {
		t.Errorf("logins = %d, want = 2", f.logins)
	}
	// The daemon is lost: the client connects again, to the first host
	// when none is on 127.0.0.1.
	f.connected, f.hosts = false, [][]any{{"remote", "10.0.0.2", 58846, "Online"}}
	if _, err := c.Items(context.Background(), client.Options{}); err != nil {
		t.Fatalf("Items() after the daemon was lost error = %v", err)
	}
	if f.connectedTo != "remote" {
		t.Errorf("connected to host %v, want = remote", f.connectedTo)
	}

	bad := newClient(t, f, map[string]any{"password": "wrong"})
	var auth *client.AuthError
	if _, err := bad.Items(context.Background(), client.Options{}); !errors.As(err, &auth) {
		t.Errorf("Items() with a wrong password error = %v, want an AuthError", err)
	}
	if err := bad.Test(context.Background()); !errors.As(err, &auth) {
		t.Errorf("Test() with a wrong password error = %v, want an AuthError", err)
	}
}

func TestTest(t *testing.T) {
	tests := []struct {
		name      string
		settings  map[string]any
		noPlugin  bool
		refuse    bool
		labels    []string
		ok        bool
		wantAdded []string
	}{
		{"no categories", nil, true, false, nil, true, nil},
		{"labels created", map[string]any{"movie_category": "radarr", "tv_imported_category": "tv-done"}, false, false, []string{"radarr"}, true, []string{"tv-done"}},
		{"label plugin inactive", map[string]any{"movie_category": "radarr"}, true, false, nil, false, nil},
		{"label refused", map[string]any{"tv_category": "tv-sonarr"}, false, true, nil, false, []string{"tv-sonarr"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			if tt.noPlugin {
				f.methods = []string{"system.listMethods", "daemon.info"}
			}
			f.refuseLabels, f.labels = tt.refuse, tt.labels
			err := newClient(t, f, tt.settings).Test(context.Background())
			if (err == nil) != tt.ok {
				t.Errorf("Test() error = %v, want ok = %v", err, tt.ok)
			}
			var added []string
			for _, c := range f.methodCalls("label.add") {
				added = append(added, c.Params[0].(string))
			}
			if !slices.Equal(added, tt.wantAdded) {
				t.Errorf("labels added = %v, want = %v", added, tt.wantAdded)
			}
		})
	}
}

func TestRemoveAndMarkImported(t *testing.T) {
	f := newFake(t)
	c := newClient(t, f, map[string]any{"tv_category": "tv-sonarr", "tv_imported_category": "tv-imported"})
	item := client.Item{ID: "HASH", Title: title, Kind: client.Series}
	if err := c.(client.Marker).MarkImported(context.Background(), item); err != nil {
		t.Fatalf("MarkImported() error = %v", err)
	}
	item.Kind = client.Movie
	if err := c.(client.Marker).MarkImported(context.Background(), item); err != nil {
		t.Fatalf("MarkImported() of a movie error = %v", err)
	}
	if err := c.Remove(context.Background(), item, true); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	var got []string
	for _, c := range f.methodCalls("") {
		if c.Method == "label.set_torrent" || c.Method == "core.remove_torrent" {
			p, _ := json.Marshal(c.Params)
			got = append(got, c.Method+string(p))
		}
	}
	want := []string{`label.set_torrent["hash","tv-imported"]`, `core.remove_torrent["hash",true]`}
	if !slices.Equal(got, want) {
		t.Errorf("calls = %v, want = %v", got, want)
	}
}

func TestSettings(t *testing.T) {
	tests := []struct {
		name     string
		settings string
		ok       bool
	}{
		{"defaults", `{}`, true},
		{"bad category", `{"movie_category":"Radarr"}`, false},
		{"bad imported category", `{"tv_imported_category":"tv_done"}`, false},
		{"unknown", `{"bogus":1}`, false},
		{"bad port", `{"port":0}`, false},
	}
	for _, tt := range tests {
		_, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
		if (err == nil) != tt.ok {
			t.Errorf("%s: New() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
	}
}

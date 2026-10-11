package aria2

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
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
)

const title = "Droned.1998.1080p.WEB-DL-DRONE"

// download is a download of the fake.
type download struct {
	GID, InfoHash, Name, Status, Error string
	Total, Completed, Uploaded, Speed  int64
	Files                              []string
}

// fake is an aria2 XML-RPC server.
type fake struct {
	t         *testing.T
	mu        sync.Mutex
	active    []download
	waiting   []download
	stopped   []download
	version   string
	dir       string
	calls     []call
	nextGID   int
	addedHash string
}

type call struct {
	Method string
	Params []value
}

func newFake(t *testing.T) *fake {
	return &fake{t: t, version: "1.37.0", dir: "/downloads"}
}

func (f *fake) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path != "/rpc" {
		http.NotFound(w, r)
		return
	}
	body, _ := io.ReadAll(r.Body)
	var mc struct {
		Method string  `xml:"methodName"`
		Params []value `xml:"params>param>value"`
	}
	if err := xml.Unmarshal(body, &mc); err != nil {
		f.t.Errorf("request body: %v", err)
	}
	if len(mc.Params) == 0 || mc.Params[0].str() != "token:secret" {
		writeResponse(w, `<fault><value><struct><member><name>faultCode</name><value><int>1</int></value></member>`+
			`<member><name>faultString</name><value><string>Unauthorized</string></value></member></struct></value></fault>`)
		return
	}
	f.calls = append(f.calls, call{Method: mc.Method, Params: mc.Params[1:]})
	var v string
	switch mc.Method {
	case "aria2.getVersion":
		v = structOf(map[string]string{"version": f.version})
	case "aria2.getGlobalOption":
		v = structOf(map[string]string{"dir": f.dir, "max-concurrent-downloads": "5"})
	case "aria2.tellActive":
		v = list(f.active)
	case "aria2.tellWaiting":
		v = list(f.waiting)
	case "aria2.tellStopped":
		v = list(f.stopped)
	case "aria2.addUri", "aria2.addTorrent":
		f.nextGID++
		gid := fmt.Sprintf("%016d", f.nextGID)
		f.active = append(f.active, download{GID: gid, InfoHash: f.addedHash, Status: "active"})
		v = str(gid)
	case "aria2.tellStatus":
		v = "<value><struct></struct></value>"
		for _, d := range slices.Concat(f.active, f.waiting, f.stopped) {
			if d.GID == mc.Params[1].str() {
				v = statusOf(d)
			}
		}
	case "aria2.forceRemove":
		v = str(mc.Params[1].str())
	case "aria2.removeDownloadResult":
		v = str("OK")
	default:
		f.t.Errorf("unexpected method %s", mc.Method)
	}
	writeResponse(w, "<params><param>"+v+"</param></params>")
}

func writeResponse(w io.Writer, inner string) {
	_, _ = io.WriteString(w, `<?xml version="1.0" encoding="UTF-8"?><methodResponse>`+inner+`</methodResponse>`)
}

func str(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return "<value><string>" + b.String() + "</string></value>"
}

func structOf(m map[string]string) string {
	var b strings.Builder
	b.WriteString("<value><struct>")
	for k, v := range m {
		fmt.Fprintf(&b, "<member><name>%s</name>%s</member>", k, str(v))
	}
	b.WriteString("</struct></value>")
	return b.String()
}

func statusOf(d download) string {
	var b strings.Builder
	member := func(name, v string) { fmt.Fprintf(&b, "<member><name>%s</name>%s</member>", name, v) }
	b.WriteString("<value><struct>")
	member("gid", str(d.GID))
	if d.InfoHash != "" {
		member("infoHash", str(d.InfoHash))
	}
	member("status", str(d.Status))
	member("totalLength", str(strconv.FormatInt(d.Total, 10)))
	member("completedLength", str(strconv.FormatInt(d.Completed, 10)))
	member("uploadLength", str(strconv.FormatInt(d.Uploaded, 10)))
	member("downloadSpeed", str(strconv.FormatInt(d.Speed, 10)))
	if d.Error != "" {
		member("errorMessage", str(d.Error))
	}
	var files strings.Builder
	for i, p := range d.Files {
		fmt.Fprintf(&files, "<value><struct><member><name>index</name>%s</member><member><name>path</name>%s</member></struct></value>", str(strconv.Itoa(i+1)), str(p))
	}
	member("files", "<value><array><data>"+files.String()+"</data></array></value>")
	if d.Name != "" {
		member("bittorrent", "<value><struct><member><name>announceList</name><value><array><data></data></array></value></member>"+
			"<member><name>info</name><value><struct><member><name>name</name>"+str(d.Name)+"</member></struct></value></member></struct></value>")
	}
	b.WriteString("</struct></value>")
	return b.String()
}

func list(ds []download) string {
	var b strings.Builder
	b.WriteString("<value><array><data>")
	for _, d := range ds {
		b.WriteString(statusOf(d))
	}
	b.WriteString("</data></array></value>")
	return b.String()
}

func (f *fake) methods() []call {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []call
	for _, c := range f.calls {
		switch c.Method {
		case "aria2.tellActive", "aria2.tellWaiting", "aria2.tellStopped", "aria2.tellStatus", "aria2.getGlobalOption", "aria2.getVersion":
		default:
			out = append(out, c)
		}
	}
	return out
}

func newClient(t *testing.T, f *fake, settings map[string]any) *aria2 {
	t.Helper()
	srv := httptest.NewServer(f)
	t.Cleanup(srv.Close)
	host, port, _ := net.SplitHostPort(srv.Listener.Addr().String())
	p, _ := strconv.Atoi(port)
	s := map[string]any{"host": host, "port": p, "secret_token": "secret"}
	for k, v := range settings {
		s[k] = v
	}
	raw, _ := json.Marshal(s)
	c, err := Definitions()[0].New(raw, client.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	a := c.(*aria2)
	a.retryDelay = time.Millisecond
	return a
}

func items(t *testing.T, c client.Client, opts client.Options) []client.Item {
	t.Helper()
	items, err := c.Items(context.Background(), opts)
	if err != nil {
		t.Fatalf("Items() error = %v", err)
	}
	return items
}

func base() download {
	return download{
		GID: "0000000000000001", InfoHash: "cbc2f069fe8bb2f544eae707d75bcd3de9dcf951", Name: title,
		Total: 1000, Completed: 400, Uploaded: 500, Files: []string{"/downloads/" + title + "/a.mkv"},
	}
}

func TestItemStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    string
		completed int64
		list      string
		want      client.Status
		removable bool
	}{
		{"downloading", "active", 400, "active", client.Downloading, false},
		{"active and done", "active", 1000, "active", client.Completed, false},
		{"waiting", "waiting", 0, "waiting", client.Queued, false},
		{"paused", "paused", 400, "waiting", client.Paused, false},
		{"error", "error", 400, "stopped", client.Failed, false},
		{"complete", "complete", 1000, "stopped", client.Completed, true},
		{"removed", "removed", 400, "stopped", client.Failed, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			d := base()
			d.Status, d.Completed = tt.status, tt.completed
			switch tt.list {
			case "active":
				f.active = []download{d}
			case "waiting":
				f.waiting = []download{d}
			default:
				f.stopped = []download{d}
			}
			got := items(t, newClient(t, f, nil), client.Options{RemoveCompletedDownloads: true})
			if len(got) != 1 {
				t.Fatalf("Items() = %d items, want 1", len(got))
			}
			item := got[0]
			if item.Status != tt.want {
				t.Errorf("Status = %v, want = %v", item.Status, tt.want)
			}
			if item.ID != "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951" || item.Title != title || item.TotalSize != 1000 ||
				item.RemainingSize != 1000-tt.completed || item.OutputPath != "/downloads/"+title+"/a.mkv" {
				t.Errorf("item = %+v, want the download's hash, name, sizes and file", item)
			}
			if item.SeedRatio == nil || *item.SeedRatio != 0.5 {
				t.Errorf("SeedRatio = %v, want = 0.5", item.SeedRatio)
			}
			if item.CanBeRemoved != tt.removable || item.CanMoveFiles {
				t.Errorf("CanBeRemoved, CanMoveFiles = %v, %v, want = %v, false", item.CanBeRemoved, item.CanMoveFiles, tt.removable)
			}
		})
	}
}

func TestItemValues(t *testing.T) {
	f := newFake(t)
	d := base()
	d.Status, d.Speed, d.Error = "active", 100, "slow"
	d.Files = []string{"/downloads/" + title + "/a.mkv", "/downloads/" + title + "/Subs/b.srt"}
	meta := base()
	meta.Status, meta.Files = "active", []string{"[METADATA]" + title}
	plain := base()
	plain.Status, plain.InfoHash = "active", ""
	f.active = []download{d, meta, plain}
	got := items(t, newClient(t, f, nil), client.Options{})
	if len(got) != 1 {
		t.Fatalf("Items() = %d items, want 1 without metadata and plain downloads", len(got))
	}
	item := got[0]
	if item.RemainingTime == nil || *item.RemainingTime != 6*time.Second {
		t.Errorf("RemainingTime = %v, want = 6s", item.RemainingTime)
	}
	if item.OutputPath != "/downloads/"+title || item.Message != "slow" {
		t.Errorf("OutputPath, Message = %q, %q, want = %q, %q", item.OutputPath, item.Message, "/downloads/"+title, "slow")
	}
	if item.CanBeRemoved {
		t.Errorf("CanBeRemoved = true without RemoveCompletedDownloads, want false")
	}
}

func TestItemsFilter(t *testing.T) {
	tests := []struct {
		name     string
		settings map[string]any
		want     int
		kind     client.Kind
	}{
		{"no directories", nil, 1, 0},
		{"movie directory", map[string]any{"movie_directory": "/downloads", "tv_directory": "/tv"}, 1, client.Movie},
		{"series directory", map[string]any{"movie_directory": "/movies", "tv_directory": "/downloads/"}, 1, client.Series},
		{"neither", map[string]any{"movie_directory": "/movies", "tv_directory": "/tv"}, 0, 0},
		{"one directory", map[string]any{"movie_directory": "/movies"}, 1, client.Series},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			d := base()
			d.Status = "active"
			f.active = []download{d}
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
		resolves bool
		method   string
		wantID   string
		wantDir  string
	}{
		{"torrent file", nil, client.Release{Kind: client.Movie, Data: data}, true, "aria2.addTorrent", hash, ""},
		{"magnet", nil, client.Release{Kind: client.Movie, MagnetURL: magnet}, true, "aria2.addUri", magnetHash, ""},
		{"magnet unresolved", nil, client.Release{Kind: client.Movie, MagnetURL: magnet}, false, "aria2.addUri", magnetHash, ""},
		{"torrent file unresolved", nil, client.Release{Kind: client.Movie, Data: data}, false, "aria2.addTorrent", hash, ""},
		{
			"series directory",
			map[string]any{"movie_directory": "/movies", "tv_directory": "/tv"},
			client.Release{Kind: client.Series, Data: data},
			true, "aria2.addTorrent", hash, "/tv",
		},
		{
			"movie directory",
			map[string]any{"movie_directory": "/movies", "tv_directory": "/tv"},
			client.Release{Kind: client.Movie, MagnetURL: magnet},
			true, "aria2.addUri", magnetHash, "/movies",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFake(t)
			if tt.resolves {
				f.addedHash = strings.ToLower(tt.wantID)
			}
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
			p := calls[0].Params
			var opts value
			switch tt.method {
			case "aria2.addUri":
				if len(p) != 2 || len(p[0].values()) != 1 || p[0].values()[0].str() != magnet {
					t.Fatalf("addUri params = %+v, want the magnet and options", p)
				}
				opts = p[1]
			default:
				if len(p) != 3 || strings.TrimSpace(*p[0].Base64) != base64.StdEncoding.EncodeToString(data) || p[1].Array == nil || len(p[1].values()) != 0 {
					t.Fatalf("addTorrent params = %+v, want the torrent, no URIs and options", p)
				}
				opts = p[2]
			}
			got := ""
			if m := opts.member("dir"); m != nil {
				got = m.str()
			}
			if opts.Struct == nil || got != tt.wantDir {
				t.Errorf("dir = %q, want = %q in a struct", got, tt.wantDir)
			}
			if n := countCalls(f, "aria2.tellStatus"); (n == 1) != tt.resolves || (!tt.resolves && n != 10) {
				t.Errorf("tellStatus calls = %d, want 1 when resolved and 10 otherwise", n)
			}
		})
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

func TestRemove(t *testing.T) {
	tests := []struct {
		status, list, method string
	}{
		{"active", "active", "aria2.forceRemove"},
		{"waiting", "waiting", "aria2.forceRemove"},
		{"paused", "waiting", "aria2.forceRemove"},
		{"complete", "stopped", "aria2.removeDownloadResult"},
		{"error", "stopped", "aria2.removeDownloadResult"},
		{"removed", "stopped", "aria2.removeDownloadResult"},
	}
	for _, tt := range tests {
		f := newFake(t)
		d := base()
		d.Status = tt.status
		switch tt.list {
		case "active":
			f.active = []download{d}
		case "waiting":
			f.waiting = []download{d}
		default:
			f.stopped = []download{d}
		}
		c := newClient(t, f, nil)
		item := client.Item{ID: "CBC2F069FE8BB2F544EAE707D75BCD3DE9DCF951", OutputPath: t.TempDir() + "/missing"}
		if err := c.Remove(context.Background(), item, true); err != nil {
			t.Fatalf("%s: Remove() error = %v", tt.status, err)
		}
		calls := f.methods()
		if len(calls) != 1 || calls[0].Method != tt.method || calls[0].Params[0].str() != d.GID {
			t.Errorf("%s: calls = %+v, want %s of %s", tt.status, calls, tt.method, d.GID)
		}
	}
	// A download not found is only logged.
	f := newFake(t)
	if err := newClient(t, f, nil).Remove(context.Background(), client.Item{ID: "OTHER"}, false); err != nil {
		t.Errorf("Remove() of an unknown download error = %v, want nil", err)
	}
	if calls := f.methods(); len(calls) != 0 {
		t.Errorf("calls = %+v, want none", calls)
	}
}

func TestInfo(t *testing.T) {
	tests := []struct {
		settings map[string]any
		want     []string
	}{
		{nil, []string{"/downloads"}},
		{map[string]any{"tv_directory": "/tv"}, []string{"/downloads", "/tv"}},
		{map[string]any{"movie_directory": "/movies", "tv_directory": "/tv"}, []string{"/movies", "/tv"}},
	}
	for _, tt := range tests {
		info, err := newClient(t, newFake(t), tt.settings).Info(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if !info.IsLocalhost || !slices.Equal(info.OutputRootFolders, tt.want) {
			t.Errorf("Info() = %+v, want localhost with %v", info, tt.want)
		}
	}
}

func TestTest(t *testing.T) {
	tests := []struct {
		name, version, token string
		ok                   bool
	}{
		{"supported", "1.37.0", "secret", true},
		{"minimum", "1.34.0", "secret", true},
		{"old", "1.33.1", "secret", false},
		{"unparsable", "dev", "secret", false},
		{"wrong token", "1.37.0", "wrong", false},
	}
	for _, tt := range tests {
		f := newFake(t)
		f.version = tt.version
		err := newClient(t, f, map[string]any{"secret_token": tt.token}).Test(context.Background())
		if (err == nil) != tt.ok {
			t.Errorf("%s: Test() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
		var fault *faultError
		if tt.token == "wrong" && (!errors.As(err, &fault) || fault.Code != 1 || fault.Message != "Unauthorized") {
			t.Errorf("%s: Test() error = %v, want the fault", tt.name, err)
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
		{"unknown", `{"bogus":1}`, false},
		{"no host", `{"host":""}`, false},
	}
	for _, tt := range tests {
		c, err := Definitions()[0].New([]byte(tt.settings), client.Env{HTTP: http.DefaultClient, Now: time.Now, Log: slog.New(slog.DiscardHandler)})
		if (err == nil) != tt.ok {
			t.Errorf("%s: New() error = %v, want ok = %v", tt.name, err, tt.ok)
		}
		if tt.name == "defaults" {
			s := c.(*aria2).settings
			if s.BaseURL() != "http://localhost:6800/rpc" || s.SecretToken != "MySecretToken" {
				t.Errorf("defaults = %+v, want Radarr's", s)
			}
		}
	}
}

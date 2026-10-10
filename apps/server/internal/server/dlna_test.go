package server_test

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// dlnaSource is the DLNA plugin's folder.
var dlnaSource = filepath.Join("..", "..", "..", "..", "plugins", "dlna")

// dlnaBuild builds the DLNA plugin once per test binary.
var dlnaBuild = sync.OnceValues(func() (string, error) {
	out := filepath.Join(smokeRoot, "dlna")
	build := exec.Command("go", "build", "-o", out, ".")
	build.Dir = dlnaSource
	if msg, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build the DLNA plugin: %w\n%s", err, msg)
	}
	return out, nil
})

// installDLNA installs the DLNA plugin into a new plugin folder.
func installDLNA(t *testing.T) string {
	t.Helper()
	bin, err := dlnaBuild()
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	dir := filepath.Join(root, "dlna")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := "plugin"
	if filepath.Separator == '\\' {
		exe = "plugin.exe"
	}
	copyFile(t, bin, filepath.Join(dir, exe))
	if err := os.Chmod(filepath.Join(dir, exe), 0o755); err != nil {
		t.Fatal(err)
	}
	copyFile(t, filepath.Join(dlnaSource, "manifest.json"), filepath.Join(dir, "manifest.json"))
	return root
}

// TestDLNA runs the assembled server with the DLNA plugin and real
// ffmpeg: a control point finds the media server, browses and searches a
// library and reads its media, ranges and all; a simulated renderer is
// found and driven through play, pause, seek, volume and stop, with
// progress reported.
func TestDLNA(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	ctx := t.Context()
	films := t.TempDir()
	// An MP4 the generic profile plays as it is and an MKV it takes as a
	// transcode.
	film := filepath.Join(films, "Film (2000)", "Film (2000).mp4")
	clip := filepath.Join(films, "Clip (2001)", "Clip (2001).mkv")
	for _, f := range []string{film, clip} {
		if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
			t.Fatal(err)
		}
		run(t, ffmpeg, "-v", "error", "-f", "lavfi", "-i", "testsrc2=size=160x90:rate=24", "-f", "lavfi", "-i", "sine=frequency=440",
			"-t", "6", "-c:v", "libx264", "-c:a", "aac", f)
	}
	settled(t, film, clip)

	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), FFmpeg: ffmpeg, FFprobe: ffprobe,
		TranscodeDir: t.TempDir(), CacheDir: t.TempDir(), MetadataDir: t.TempDir(), PluginDir: installDLNA(t), PluginDataDir: t.TempDir(),
	})
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"),
		Device: authv1.Device_builder{Id: new("e2e"), Name: new("Test"), Client: new("Test"), ClientVersion: new("0")}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	libraries := libraryv1connect.NewLibraryServiceClient(http.DefaultClient, srv.url, token)
	lib, err := libraries.CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: libraryv1.LibrarySpec_builder{
		Name: new("Films"), Kind: new(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES), Paths: []string{films},
	}.Build()}.Build())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	items := libraryv1connect.NewItemServiceClient(http.DefaultClient, srv.url, token)
	ids := map[string]string{}
	waitFor(t, "both films to be scanned with their media", func() bool {
		list, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MOVIE}}.Build())
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range list.GetItems() {
			resp, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: new(it.GetId())}.Build())
			if err == nil && len(resp.GetMediaSources()) > 0 && resp.GetMediaSources()[0].GetDuration() != nil {
				ids[it.GetName()] = it.GetId()
			}
		}
		return len(ids) == 2
	}, func() any { return ids })

	renderer := newFakeRenderer(t)
	ssdpPort := sparePort(t)
	lo := loopbackName()
	cfg := fmt.Sprintf(`{"user":"admin","server_name":"Mavio Test","ssdp_port":%d,"renderers":[%q]%s}`, ssdpPort, renderer.url+"/description.xml",
		map[bool]string{true: `,"interfaces":["` + lo + `"]`}[lo != ""])
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token)
	if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId: new("org.mavio.dlna"), ConfigJson: new(cfg),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}
	base := srv.url + "/plugins/org.mavio.dlna"
	var desc []byte
	waitFor(t, "the media server's description", func() bool {
		resp, err := http.Get(base + "/description.xml")
		if err != nil {
			return false
		}
		defer resp.Body.Close()
		desc, _ = io.ReadAll(resp.Body)
		return resp.StatusCode == http.StatusOK
	})
	for _, want := range []string{
		"<friendlyName>Mavio Test</friendlyName>", "urn:schemas-upnp-org:device:MediaServer:1",
		"<controlURL>" + base + "/control/cds</controlURL>", "<UDN>uuid:",
	} {
		if !bytes.Contains(desc, []byte(want)) {
			t.Errorf("description lacks %q:\n%s", want, desc)
		}
	}

	t.Run("discovery", func(t *testing.T) {
		if lo == "" {
			t.Skip("no loopback interface takes multicast")
		}
		// A unicast search reaches the media server's SSDP socket.
		conn, err := net.ListenPacket("udp4", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		search := "M-SEARCH * HTTP/1.1\r\nHOST: 239.255.255.250:1900\r\nMAN: \"ssdp:discover\"\r\nMX: 1\r\nST: urn:schemas-upnp-org:device:MediaServer:1\r\n\r\n"
		var answer string
		waitFor(t, "an answer to the search", func() bool {
			if _, err := conn.WriteTo([]byte(search), &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: ssdpPort}); err != nil {
				t.Fatal(err)
			}
			_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
			buf := make([]byte, 4096)
			n, _, err := conn.ReadFrom(buf)
			answer = string(buf[:n])
			return err == nil
		})
		for _, want := range []string{"HTTP/1.1 200 OK", "LOCATION: " + base + "/description.xml", "ST: urn:schemas-upnp-org:device:MediaServer:1"} {
			if !strings.Contains(answer, want) {
				t.Errorf("search answer lacks %q:\n%s", want, answer)
			}
		}
	})

	// The root lists the library; the library its films, by name.
	root := browse(t, base, "0", "BrowseDirectChildren")
	if len(root.Containers) != 1 || root.Containers[0].ID != "library:"+lib.GetLibrary().GetId() ||
		root.Containers[0].Title != "Films" || root.Containers[0].ChildCount != 2 {
		t.Fatalf("root = %+v", root)
	}
	library := browse(t, base, root.Containers[0].ID, "BrowseDirectChildren")
	if got := library.titles(); !slices.Equal(got, []string{"Clip", "Film"}) {
		t.Fatalf("library titles = %v, want = [Clip Film]", got)
	}
	filmRes, clipRes := library.Items[1].Res[0], library.Items[0].Res[0]
	if !strings.Contains(filmRes.Protocol, ":video/mp4:DLNA.ORG_OP=01;") {
		t.Errorf("film protocol info = %s, want MP4 seekable by bytes", filmRes.Protocol)
	}
	if !strings.Contains(clipRes.Protocol, ":video/mp2t:DLNA.ORG_OP=00;DLNA.ORG_CI=1;") {
		t.Errorf("clip protocol info = %s, want a transcode to MPEG-TS", clipRes.Protocol)
	}
	if meta := browse(t, base, ids["Film"], "BrowseMetadata"); len(meta.Items) != 1 || meta.Items[0].Title != "Film" {
		t.Errorf("film metadata = %+v", meta)
	}
	found := soap(t, base, "Search", "ContainerID", "0", "SearchCriteria", `upnp:class derivedfrom "object.item.videoItem" and dc:title contains "fil"`,
		"Filter", "*", "StartingIndex", "0", "RequestedCount", "0", "SortCriteria", "")
	if got := parseDIDL(t, found["Result"]).titles(); !slices.Equal(got, []string{"Film"}) {
		t.Errorf("search titles = %v, want = [Film]", got)
	}

	// The film is read in ranges, as it is.
	for _, r := range []string{"bytes=0-99", "bytes=100-199"} {
		resp := getRange(t, filmRes.URL, r)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusPartialContent || len(body) != 100 || resp.Header.Get("Content-Type") != "video/mp4" ||
			resp.Header.Get("transferMode.dlna.org") != "Streaming" || !strings.HasPrefix(resp.Header.Get("contentFeatures.dlna.org"), "DLNA.ORG_OP=01") {
			t.Errorf("GET %s %s = %d %v, %d bytes", filmRes.URL, r, resp.StatusCode, resp.Header, len(body))
		}
	}
	// The clip is transcoded to MPEG-TS.
	resp := getRange(t, clipRes.URL, "")
	packet := make([]byte, 188)
	_, err = io.ReadFull(resp.Body, packet)
	resp.Body.Close()
	if err != nil || resp.StatusCode != http.StatusOK || packet[0] != 0x47 || resp.Header.Get("Content-Type") != "video/mp2t" {
		t.Errorf("GET %s = %d %v, first byte %#x, %v", clipRes.URL, resp.StatusCode, resp.Header, packet[0], err)
	}

	// The renderer configured is listed as a device.
	sessions := sessionv1connect.NewSessionServiceClient(http.DefaultClient, srv.url, token)
	device := func() *sessionv1.Session {
		list, err := sessions.ListSessions(ctx, &sessionv1.ListSessionsRequest{})
		if err != nil {
			t.Fatalf("ListSessions: %v", err)
		}
		for _, s := range list.GetSessions() {
			if s.GetPluginId() == "org.mavio.dlna" {
				return s
			}
		}
		return nil
	}
	waitFor(t, "the renderer to be listed", func() bool { return device() != nil })
	if tv := device(); tv.GetDeviceId() != fakeUDN || tv.GetDeviceName() != "Test Renderer" {
		t.Fatalf("renderer session = %v", tv)
	}
	send := func(c *sessionv1.Command) {
		t.Helper()
		if _, err := sessions.SendCommand(ctx, sessionv1.SendCommandRequest_builder{SessionId: new(device().GetId()), Command: c}.Build()); err != nil {
			t.Fatalf("SendCommand(%v): %v", c, err)
		}
	}
	playing := func(what string, ok func(*sessionv1.NowPlaying) bool) {
		t.Helper()
		waitFor(t, what, func() bool { return ok(device().GetNowPlaying()) }, func() any { return device().GetNowPlaying() })
	}

	// Playing the film hands the renderer a URL it reads as it is, and
	// progress is reported.
	send(sessionv1.Command_builder{Play: sessionv1.Play_builder{ItemIds: []string{ids["Film"], ids["Clip"]}}.Build()}.Build())
	playing("the film to play", func(n *sessionv1.NowPlaying) bool {
		return n.GetItem().GetId() == ids["Film"] && n.GetPosition().AsDuration() > 0 && !n.GetPaused()
	})
	if got := renderer.fetched(); len(got) != 1 || got[0] != "206 video/mp4" {
		t.Errorf("renderer reads = %v, want = [206 video/mp4]", got)
	}
	send(sessionv1.Command_builder{PlayState: sessionv1.PlayState_builder{Command: sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_PAUSE.Enum()}.Build()}.Build())
	playing("the film to pause", func(n *sessionv1.NowPlaying) bool { return n.GetPaused() })
	send(sessionv1.Command_builder{Seek: sessionv1.Seek_builder{Position: durationpb.New(3 * time.Second)}.Build()}.Build())
	send(sessionv1.Command_builder{Volume: sessionv1.Volume_builder{Level: new(int32(30)), Muted: new(true)}.Build()}.Build())
	if got := renderer.calls(); !slices.Contains(got, "Seek 0:00:03") || !slices.Contains(got, "SetVolume 30") || !slices.Contains(got, "SetMute 1") {
		t.Errorf("renderer calls = %v, want Seek 0:00:03, SetVolume 30 and SetMute 1", got)
	}

	// The next item is the clip, transcoded; seeking it starts the
	// transcode again where it seeks to.
	send(sessionv1.Command_builder{PlayState: sessionv1.PlayState_builder{Command: sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_NEXT.Enum()}.Build()}.Build())
	playing("the clip to play", func(n *sessionv1.NowPlaying) bool { return n.GetItem().GetId() == ids["Clip"] })
	before := renderer.uri()
	send(sessionv1.Command_builder{Seek: sessionv1.Seek_builder{Position: durationpb.New(2 * time.Second)}.Build()}.Build())
	if after := renderer.uri(); after == before || !strings.Contains(after, "/play/") {
		t.Errorf("renderer URI after seeking the clip = %s, want a new playback (was %s)", after, before)
	}
	playing("the clip to play from the seek", func(n *sessionv1.NowPlaying) bool { return n.GetPosition().AsDuration() >= 2*time.Second })
	if got := renderer.fetched(); len(got) != 3 || got[1] != "200 video/mp2t" || got[2] != "200 video/mp2t" {
		t.Errorf("renderer reads = %v, want the clip twice as MPEG-TS", got)
	}

	// Stopping ends the playback.
	send(sessionv1.Command_builder{PlayState: sessionv1.PlayState_builder{Command: sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_STOP.Enum()}.Build()}.Build())
	playing("the playback to stop", func(n *sessionv1.NowPlaying) bool { return n == nil })
	if got := renderer.state(); got != "STOPPED" {
		t.Errorf("renderer state = %s, want = STOPPED", got)
	}
}

// loopbackName returns the name of a loopback interface that takes
// multicast, or "".
func loopbackName() string {
	all, _ := net.Interfaces()
	for _, ifi := range all {
		if ifi.Flags&(net.FlagLoopback|net.FlagUp|net.FlagMulticast) == net.FlagLoopback|net.FlagUp|net.FlagMulticast {
			return ifi.Name
		}
	}
	return ""
}

func sparePort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenPacket("udp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).Port
}

func getRange(t *testing.T, url, r string) *http.Response {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r != "" {
		req.Header.Set("Range", r)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	return resp
}

// soap invokes a ContentDirectory action and returns its out arguments.
func soap(t *testing.T, base, action string, args ...string) map[string]string {
	t.Helper()
	var b strings.Builder
	fmt.Fprintf(&b, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/" s:encodingStyle="http://schemas.xmlsoap.org/soap/encoding/">`+
		`<s:Body><u:%s xmlns:u="urn:schemas-upnp-org:service:ContentDirectory:1">`, action)
	for i := 0; i+1 < len(args); i += 2 {
		b.WriteString("<" + args[i] + ">")
		_ = xml.EscapeText(&b, []byte(args[i+1]))
		b.WriteString("</" + args[i] + ">")
	}
	fmt.Fprintf(&b, `</u:%s></s:Body></s:Envelope>`, action)
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, base+"/control/cds", strings.NewReader(b.String()))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", `text/xml; charset="utf-8"`)
	req.Header.Set("SOAPACTION", `"urn:schemas-upnp-org:service:ContentDirectory:1#`+action+`"`)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s: %v", action, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("%s = %d\n%s", action, resp.StatusCode, body)
	}
	return outArgs(t, body)
}

// outArgs reads the arguments of a SOAP response: the elements within the
// body's one element.
func outArgs(t *testing.T, body []byte) map[string]string {
	t.Helper()
	out := map[string]string{}
	d := xml.NewDecoder(bytes.NewReader(body))
	depth, name := 0, ""
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return out
		} else if err != nil {
			t.Fatalf("SOAP response: %v\n%s", err, body)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			if depth == 4 {
				name = tok.Name.Local
				out[name] = ""
			}
		case xml.EndElement:
			depth--
		case xml.CharData:
			if depth == 4 {
				out[name] += string(tok)
			}
		}
	}
}

type didlDoc struct {
	Containers []struct {
		ID         string `xml:"id,attr"`
		ChildCount int    `xml:"childCount,attr"`
		Title      string `xml:"title"`
	} `xml:"container"`
	Items []struct {
		ID    string `xml:"id,attr"`
		Title string `xml:"title"`
		Res   []struct {
			Protocol string `xml:"protocolInfo,attr"`
			URL      string `xml:",chardata"`
		} `xml:"res"`
	} `xml:"item"`
}

func (d didlDoc) titles() []string {
	var out []string
	for _, it := range d.Items {
		out = append(out, it.Title)
	}
	return out
}

func parseDIDL(t *testing.T, s string) didlDoc {
	t.Helper()
	var d didlDoc
	if err := xml.Unmarshal([]byte(s), &d); err != nil {
		t.Fatalf("DIDL-Lite: %v\n%s", err, s)
	}
	return d
}

func browse(t *testing.T, base, id, flag string) didlDoc {
	t.Helper()
	out := soap(t, base, "Browse", "ObjectID", id, "BrowseFlag", flag, "Filter", "*", "StartingIndex", "0", "RequestedCount", "0", "SortCriteria", "")
	return parseDIDL(t, out["Result"])
}

const fakeUDN = "uuid:7e57-7e57-7e57"

// fakeRenderer is a DLNA renderer: it keeps the transport's state, its
// position running while it plays, reads the start of what it is told to
// play and records the calls it gets.
type fakeRenderer struct {
	url string

	mu        sync.Mutex
	transport string
	current   string
	position  time.Duration
	since     time.Time
	log       []string
	reads     []string
}

var soapArg = regexp.MustCompile(`<(\w+)>([^<]*)</(\w+)>`)

func newFakeRenderer(t *testing.T) *fakeRenderer {
	f := &fakeRenderer{transport: "NO_MEDIA_PRESENT"}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /description.xml", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `<?xml version="1.0"?><root xmlns="urn:schemas-upnp-org:device-1-0"><specVersion><major>1</major><minor>0</minor></specVersion>`+
			`<device><deviceType>urn:schemas-upnp-org:device:MediaRenderer:1</deviceType><friendlyName>Test Renderer</friendlyName>`+
			`<manufacturer>Acme</manufacturer><modelName>Box</modelName><UDN>%s</UDN><serviceList>`+
			`<service><serviceType>urn:schemas-upnp-org:service:AVTransport:1</serviceType><serviceId>urn:upnp-org:serviceId:AVTransport</serviceId>`+
			`<SCPDURL>/avt.xml</SCPDURL><controlURL>/control/avt</controlURL><eventSubURL>/events/avt</eventSubURL></service>`+
			`<service><serviceType>urn:schemas-upnp-org:service:RenderingControl:1</serviceType><serviceId>urn:upnp-org:serviceId:RenderingControl</serviceId>`+
			`<SCPDURL>/rc.xml</SCPDURL><controlURL>/control/rc</controlURL><eventSubURL>/events/rc</eventSubURL></service>`+
			`</serviceList></device></root>`, fakeUDN)
	})
	mux.HandleFunc("POST /control/{service}", func(w http.ResponseWriter, r *http.Request) {
		_, action, _ := strings.Cut(strings.Trim(r.Header.Get("Soapaction"), `"`), "#")
		serviceType, _, _ := strings.Cut(strings.Trim(r.Header.Get("Soapaction"), `"`), "#")
		body, _ := io.ReadAll(r.Body)
		args := map[string]string{}
		for _, m := range soapArg.FindAllStringSubmatch(string(body), -1) {
			args[m[1]] = m[2]
		}
		out := f.act(action, args)
		var b strings.Builder
		for _, a := range out {
			fmt.Fprintf(&b, "<%s>%s</%s>", a[0], a[1], a[0])
		}
		w.Header().Set("Content-Type", `text/xml; charset="utf-8"`)
		fmt.Fprintf(w, `<?xml version="1.0"?><s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>`+
			`<u:%sResponse xmlns:u="%s">%s</u:%sResponse></s:Body></s:Envelope>`, action, serviceType, b.String(), action)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	f.url = srv.URL
	return f
}

// act carries out an action, returning its out arguments.
func (f *fakeRenderer) act(action string, args map[string]string) [][2]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch action {
	case "GetTransportInfo":
		return [][2]string{{"CurrentTransportState", f.transport}, {"CurrentTransportStatus", "OK"}, {"CurrentSpeed", "1"}}
	case "GetPositionInfo":
		s := int(f.now() / time.Second)
		return [][2]string{{"Track", "1"}, {"TrackURI", xmlEscape(f.current)}, {"RelTime", fmt.Sprintf("%d:%02d:%02d", s/3600, s/60%60, s%60)}}
	}
	f.log = append(f.log, strings.TrimSpace(action+" "+args["Target"]+args["DesiredVolume"]+args["DesiredMute"]))
	switch action {
	case "SetAVTransportURI":
		f.current, f.position, f.transport = unescapeXML(args["CurrentURI"]), 0, "STOPPED"
	case "Play":
		f.position, f.since, f.transport = f.now(), time.Now(), "PLAYING"
		go f.read(f.current)
	case "Pause":
		f.position, f.transport = f.now(), "PAUSED_PLAYBACK"
	case "Stop":
		f.position, f.transport = 0, "STOPPED"
	case "Seek":
		var h, m, s int
		_, _ = fmt.Sscanf(args["Target"], "%d:%d:%d", &h, &m, &s)
		f.position, f.since = time.Duration(h*3600+m*60+s)*time.Second, time.Now()
	}
	return nil
}

// now returns the position, running while playing.
func (f *fakeRenderer) now() time.Duration {
	if f.transport == "PLAYING" {
		return f.position + time.Since(f.since)
	}
	return f.position
}

// read reads the start of the media at uri, as renderers do once told to
// play, recording the status and type.
func (f *fakeRenderer) read(uri string) {
	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		return
	}
	req.Header.Set("Range", "bytes=0-1023")
	resp, err := http.DefaultClient.Do(req)
	got := "error"
	if err == nil {
		_, _ = io.CopyN(io.Discard, resp.Body, 1024)
		resp.Body.Close()
		got = fmt.Sprintf("%d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	f.mu.Lock()
	f.reads = append(f.reads, got)
	f.mu.Unlock()
}

// fetched returns what the reads of the media found, once each Play has
// read.
func (f *fakeRenderer) fetched() []string {
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		f.mu.Lock()
		plays := 0
		for _, c := range f.log {
			if c == "Play" {
				plays++
			}
		}
		done := len(f.reads) >= plays
		f.mu.Unlock()
		if done {
			break
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.reads)
}

func (f *fakeRenderer) calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.log)
}

func (f *fakeRenderer) uri() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current
}

func (f *fakeRenderer) state() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.transport
}

func xmlEscape(s string) string {
	var b strings.Builder
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func unescapeXML(s string) string {
	var out string
	if err := xml.Unmarshal([]byte("<x>"+s+"</x>"), &out); err != nil {
		return s
	}
	return out
}

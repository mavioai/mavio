package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"slices"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// hostAPI answers ListContainers with the containers.
func hostAPI(t *testing.T, containers ...*pluginv1.ContainerStatus) func() *http.Client {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mavio.plugin.v1.HostService/ListContainers" {
			http.NotFound(w, r)
			return
		}
		out, _ := proto.Marshal(pluginv1.ListContainersResponse_builder{Containers: containers}.Build())
		w.Header().Set("Content-Type", "application/proto")
		_, _ = w.Write(out)
	}))
	t.Cleanup(srv.Close)
	// The host client reaches guest.HostURL; send it to srv.
	c := srv.Client()
	c.Transport = rewrite{to: srv.URL, next: c.Transport}
	return func() *http.Client { return c }
}

type rewrite struct {
	to   string
	next http.RoundTripper
}

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	req.URL.Scheme, req.URL.Host = "http", strings.TrimPrefix(r.to, "http://")
	return r.next.RoundTrip(req)
}

func TestContainerClients(t *testing.T) {
	ready := func(name, address string, secrets map[string]string) *pluginv1.ContainerStatus {
		return pluginv1.ContainerStatus_builder{
			Name: proto.String(name), Mode: pluginv1.ContainerMode_CONTAINER_MODE_MANAGED.Enum(), Address: proto.String(address),
			State: pluginv1.ContainerState_CONTAINER_STATE_READY.Enum(), Secrets: secrets,
		}.Build()
	}
	host := hostAPI(t,
		ready("qbittorrent", "http://127.0.0.1:49001", map[string]string{"password": "pw"}),
		ready("sabnzbd", "http://mavio-download-clients-sabnzbd:8080", map[string]string{"api-key": "key"}),
		pluginv1.ContainerStatus_builder{Name: proto.String("transmission")}.Build(),
	)
	p, err := newPlugin(slog.New(slog.DiscardHandler), "test", t.TempDir(), http.DefaultClient, host)
	if err != nil {
		t.Fatal(err)
	}
	cfg := `{"container_clients":[{"name":"qbittorrent","priority":2,"tags":["anime"],"settings":{"movie_category":"films","host":"elsewhere","password":"guess","movie_directory":"/data"}},{"name":"transmission"},{"name":"sabnzbd"}]}`
	if _, err := p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: &cfg}.Build()); err != nil {
		t.Fatal(err)
	}
	resp, err := p.ListDownloadClients(t.Context(), &pluginv1.ListDownloadClientsRequest{})
	if err != nil {
		t.Fatal(err)
	}
	got := resp.GetClients()
	if len(got) != 2 {
		t.Fatalf("clients = %v, want = qBittorrent and SABnzbd, Transmission having no address", got)
	}
	if c := got[0]; c.GetId() != "container-qbittorrent" || c.GetName() != "qBittorrent (container)" || c.GetContainer() != "qbittorrent" ||
		c.GetHost() != "127.0.0.1" || c.GetPriority() != 2 || c.GetTags()[0] != "anime" || !c.GetEnabled() {
		t.Errorf("clients[0] = %v, want = qBittorrent's container at 127.0.0.1 with priority 2 and tag anime", c)
	}
	if c := got[1]; c.GetId() != "container-sabnzbd" || c.GetContainer() != "sabnzbd" || c.GetHost() != "mavio-download-clients-sabnzbd" {
		t.Errorf("clients[1] = %v, want = SABnzbd's container by name", c)
	}
	settings := func(id string) map[string]any {
		c, err := p.client(id)
		if err != nil {
			t.Fatal(err)
		}
		var s map[string]any
		_ = json.Unmarshal(c.cfg.Settings, &s)
		return s
	}
	if s := settings("container-qbittorrent"); s["port"] != float64(49001) || s["username"] != "admin" || s["password"] != "pw" ||
		s["host"] != "127.0.0.1" || s["movie_category"] != "films" || s["tv_category"] != "tv-mavio" || s["movie_directory"] != nil {
		t.Errorf("qBittorrent's settings = %v, want = 127.0.0.1:49001, admin and pw, the movie category given and the default series one, and no folder", s)
	}
	if s := settings("container-sabnzbd"); s["port"] != float64(8080) || s["api_key"] != "key" {
		t.Errorf("SABnzbd's settings = %v, want = port 8080 and the API key", s)
	}

	cfg = `{"container_clients":[{"name":"qbittorrent","enabled":false},{"name":"nzbget"}]}`
	if _, err := p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: &cfg}.Build()); err == nil {
		t.Error("Configure with an unknown container: got = nil error")
	}
	cfg = `{"container_clients":[{"name":"qbittorrent","enabled":false}]}`
	if _, err := p.Configure(t.Context(), pluginv1.ConfigureRequest_builder{ConfigJson: &cfg}.Build()); err != nil {
		t.Fatal(err)
	}
	if resp, _ := p.ListDownloadClients(t.Context(), &pluginv1.ListDownloadClientsRequest{}); len(resp.GetClients()) != 0 {
		t.Errorf("clients of a disabled container = %v, want = none", resp.GetClients())
	}
}

// TestContainerDefinitions checks that the clients of the containers are
// listed by name with the settings the containers do not fill in.
func TestContainerDefinitions(t *testing.T) {
	p, err := newPlugin(slog.New(slog.DiscardHandler), "test", t.TempDir(), http.DefaultClient, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := p.ListDownloadClientDefinitions(t.Context(), pluginv1.ListDownloadClientDefinitionsRequest_builder{Containers: proto.Bool(true)}.Build())
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, d := range resp.GetDefinitions() {
		names = append(names, d.GetId())
		var fields []string
		for _, f := range d.GetFields() {
			if containerFilled[f.GetName()] {
				t.Errorf("%s: field %s, which the container fills in", d.GetId(), f.GetName())
			}
			fields = append(fields, f.GetName())
		}
		if !slices.Contains(fields, "movie_category") {
			t.Errorf("%s: fields = %v, want = movie_category among them", d.GetId(), fields)
		}
	}
	if want := []string{"qbittorrent", "sabnzbd", "transmission"}; !slices.Equal(names, want) {
		t.Errorf("definitions = %v, want = %v", names, want)
	}
}

// TestManifestContainers checks that the manifest is valid and runs the
// setup scripts of containers/ as they are.
func TestManifestContainers(t *testing.T) {
	m, err := manifest.Load(".")
	if err != nil {
		t.Fatal(err)
	}
	for name := range containerClients {
		c, ok := manifest.Container(m, name)
		if !ok {
			t.Errorf("the manifest has no container %s", name)
			continue
		}
		script, err := os.ReadFile("containers/" + name + ".py")
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if cmd := c.GetCommand(); len(cmd) != 4 || cmd[3] != strings.ReplaceAll(string(script), "\r\n", "\n") {
			t.Errorf("container %s: command does not run containers/%s.py as it is; copy it into manifest.json", name, name)
		}
	}
}

package containers

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

func TestParseEndpoint(t *testing.T) {
	tests := []struct {
		in   string
		want Endpoint
		err  bool
	}{
		{in: "unix:///var/run/docker.sock", want: Endpoint{Unix, "/var/run/docker.sock"}},
		{in: "/run/user/1000/docker.sock", want: Endpoint{Unix, "/run/user/1000/docker.sock"}},
		{in: "unix://run/docker.sock", err: true},
		{in: "npipe:////./pipe/docker_engine", want: Endpoint{NamedPipe, "//./pipe/docker_engine"}},
		{in: `npipe://\\.\pipe\podman`, want: Endpoint{NamedPipe, "//./pipe/podman"}},
		{in: "npipe:////./pipe/", err: true},
		{in: "tcp://127.0.0.1:2375", want: Endpoint{TCP, "127.0.0.1:2375"}},
		{in: "tcp://localhost:2375", want: Endpoint{TCP, "localhost:2375"}},
		{in: "tcp://[::1]:2375", want: Endpoint{TCP, "[::1]:2375"}},
		{in: "tcp://192.168.1.2:2375", err: true},
		{in: "tcp://127.0.0.1", err: true},
		{in: "ssh://host", err: true},
		{in: "docker.sock", err: true},
	}
	for _, tt := range tests {
		got, err := ParseEndpoint(tt.in)
		if tt.err {
			if !errors.Is(err, core.ErrInvalid) {
				t.Errorf("ParseEndpoint(%q) = %v, %v, want core.ErrInvalid", tt.in, got, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("ParseEndpoint(%q) = %v, %v, want %v", tt.in, got, err, tt.want)
		}
	}
}

func TestCandidates(t *testing.T) {
	env := func(m map[string]string) func(string) string {
		return func(k string) string { return m[k] }
	}
	tests := []struct {
		name  string
		o     Options
		want  []string
		named bool
	}{
		{
			name: "linux",
			o:    Options{GOOS: "linux", Home: "/home/u", Getenv: env(map[string]string{"XDG_RUNTIME_DIR": "/run/user/1000"})},
			want: []string{"unix:///var/run/docker.sock", "unix:///run/user/1000/docker.sock", "unix:///home/u/.docker/run/docker.sock", "unix:///run/user/1000/podman/podman.sock", "unix:///run/podman/podman.sock"},
		},
		{
			name: "darwin",
			o:    Options{GOOS: "darwin", Home: "/Users/u", Getenv: env(nil)},
			want: []string{"unix:///var/run/docker.sock", "unix:///Users/u/.docker/run/docker.sock", "unix:///run/podman/podman.sock"},
		},
		{
			name: "windows",
			o:    Options{GOOS: "windows", Getenv: env(nil)},
			want: []string{"npipe:////./pipe/docker_engine"},
		},
		{
			name:  "DOCKER_HOST",
			o:     Options{GOOS: "linux", Getenv: env(map[string]string{"DOCKER_HOST": "unix:///srv/docker.sock"})},
			want:  []string{"unix:///srv/docker.sock"},
			named: true,
		},
		{
			name:  "flag over DOCKER_HOST",
			o:     Options{Endpoint: "tcp://127.0.0.1:2375", Getenv: env(map[string]string{"DOCKER_HOST": "unix:///srv/docker.sock"})},
			want:  []string{"tcp://127.0.0.1:2375"},
			named: true,
		},
	}
	for _, tt := range tests {
		eps, named, err := Candidates(tt.o)
		var got []string
		for _, ep := range eps {
			got = append(got, ep.String())
		}
		if err != nil || named != tt.named || !slices.Equal(got, tt.want) {
			t.Errorf("%s: Candidates = %q, %v, %v, want %q, %v", tt.name, got, named, err, tt.want, tt.named)
		}
	}
}

func TestNegotiate(t *testing.T) {
	tests := []struct {
		v    Version
		want string
		err  bool
	}{
		{v: Version{APIVersion: "1.41"}, want: "1.41"},
		{v: Version{APIVersion: "1.45", MinAPIVersion: "1.24"}, want: "1.45"},
		{v: Version{APIVersion: "1.52", MinAPIVersion: "1.44"}, want: "1.47"},
		{v: Version{APIVersion: "1.60", MinAPIVersion: "1.50"}, want: "1.50"},
		{v: Version{APIVersion: "1.40"}, err: true},
		{v: Version{}, err: true},
	}
	for _, tt := range tests {
		got, err := negotiate(tt.v)
		if tt.err {
			if !errors.Is(err, ErrAPIVersion) {
				t.Errorf("negotiate(%+v) = %q, %v, want ErrAPIVersion", tt.v, got, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("negotiate(%+v) = %q, %v, want %q", tt.v, got, err, tt.want)
		}
	}
}

func TestSplitReference(t *testing.T) {
	tests := []struct{ ref, repo, tag string }{
		{"onerahmet/openai-whisper-asr-webservice:latest@sha256:abc", "onerahmet/openai-whisper-asr-webservice", "sha256:abc"},
		{"busybox@sha256:abc", "busybox", "sha256:abc"},
		{"busybox:1.37", "busybox", "1.37"},
		{"busybox", "busybox", "latest"},
		{"registry.local:5000/team/app", "registry.local:5000/team/app", "latest"},
		{"registry.local:5000/team/app:v2", "registry.local:5000/team/app", "v2"},
	}
	for _, tt := range tests {
		repo, tag := SplitReference(tt.ref)
		if repo != tt.repo || tag != tt.tag {
			t.Errorf("SplitReference(%q) = %q, %q, want %q, %q", tt.ref, repo, tag, tt.repo, tt.tag)
		}
	}
}

// fakeEngine is an engine API on a Unix socket that answers what its
// routes say and records the requests it gets.
type fakeEngine struct {
	ep      Endpoint
	version string
	mu      sync.Mutex
	reqs    []string
	bodies  map[string]string
	routes  map[string]func(w http.ResponseWriter, r *http.Request)
}

// startFake starts a fake engine offering API version.
func startFake(t *testing.T, version string) *fakeEngine {
	t.Helper()
	// t.TempDir's paths are too long for a socket on macOS.
	dir, err := os.MkdirTemp("", "mc")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sock := filepath.Join(dir, "e.sock")
	l, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	f := &fakeEngine{ep: Endpoint{Unix, filepath.ToSlash(sock)}, version: version, bodies: map[string]string{}, routes: map[string]func(http.ResponseWriter, *http.Request){}}
	srv := httptest.NewUnstartedServer(http.HandlerFunc(f.serve))
	srv.Listener = l
	srv.Start()
	t.Cleanup(srv.Close)
	return f
}

func (f *fakeEngine) serve(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	key := r.Method + " " + r.URL.Path
	f.mu.Lock()
	f.reqs = append(f.reqs, key+"?"+r.URL.RawQuery)
	f.bodies[key] = string(body)
	h := f.routes[key]
	f.mu.Unlock()
	switch {
	case key == "GET /_ping":
		_, _ = io.WriteString(w, "OK")
	case key == "GET /version":
		_ = json.NewEncoder(w).Encode(map[string]any{"Version": "27.3.1", "ApiVersion": f.version, "MinAPIVersion": "1.24", "Os": "linux", "Arch": "arm64"})
	case h != nil:
		h(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		_, _ = fmt.Fprintf(w, `{"message":"no such route: %s"}`, key)
	}
}

func (f *fakeEngine) route(key string, h func(w http.ResponseWriter, r *http.Request)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.routes[key] = h
}

func (f *fakeEngine) body(key string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[key]
}

func (f *fakeEngine) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.reqs)
}

func jsonRoute(v any) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(v) }
}

func TestDetect(t *testing.T) {
	f := startFake(t, "1.45")
	f.route("GET /v1.45/info", jsonRoute(map[string]any{"SecurityOptions": []string{"name=seccomp,profile=builtin", "name=rootless"}}))
	f.route("GET /v1.45/containers/abc123/json", jsonRoute(map[string]any{"Id": "abc123full", "HostConfig": map[string]any{"NetworkMode": "bridge"}}))
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".dockerenv"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := Detect(t.Context(), Options{Endpoint: f.ep.String(), Root: root, Hostname: "abc123"})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Client.Close()
	if rt.Version.Engine != "docker" || rt.Version.Version != "27.3.1" || rt.APIVersion != "1.45" || !rt.Rootless || !rt.InContainer {
		t.Errorf("Detect = %+v, want docker 27.3.1, API 1.45, rootless, in a container", rt)
	}
	if rt.Self == nil || rt.Self.ID != "abc123full" || rt.OnHost() {
		t.Errorf("Detect: Self = %+v, OnHost = %v, want abc123full not on the host", rt.Self, rt.OnHost())
	}
}

func TestDetectGPU(t *testing.T) {
	tests := []struct {
		name string
		info map[string]any
		want GPU
	}{
		{"none", map[string]any{"Runtimes": map[string]any{"runc": map[string]any{"path": "runc"}}}, NoGPU},
		{"nvidia runtime", map[string]any{"Runtimes": map[string]any{"runc": map[string]any{}, "nvidia": map[string]any{"path": "nvidia-container-runtime"}}}, GPURuntime},
		{"cdi device", map[string]any{"DiscoveredDevices": []any{map[string]any{"Source": "cdi", "ID": "nvidia.com/gpu=all"}}}, GPUCDI},
		{"cdi device and runtime", map[string]any{
			"Runtimes":          map[string]any{"nvidia": map[string]any{"path": "nvidia-container-runtime"}},
			"DiscoveredDevices": []any{map[string]any{"Source": "cdi", "ID": "nvidia.com/gpu=0"}},
		}, GPUCDI},
		{"other cdi device", map[string]any{"DiscoveredDevices": []any{map[string]any{"Source": "cdi", "ID": "vendor.com/device=0"}}}, NoGPU},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := startFake(t, "1.47")
			f.route("GET /v1.47/info", jsonRoute(tt.info))
			rt, err := Detect(t.Context(), Options{Endpoint: f.ep.String(), Root: t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			defer rt.Client.Close()
			if rt.GPU != tt.want {
				t.Errorf("Detect: GPU = %v, want = %v", rt.GPU, tt.want)
			}
		})
	}
}

func TestDetectOnHost(t *testing.T) {
	f := startFake(t, "1.47")
	f.route("GET /v1.47/info", jsonRoute(map[string]any{}))
	rt, err := Detect(t.Context(), Options{Endpoint: f.ep.String(), Root: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Client.Close()
	if rt.InContainer || rt.Self != nil || !rt.OnHost() || rt.Rootless {
		t.Errorf("Detect = %+v, want on the host", rt)
	}
	for _, r := range f.requests() {
		if strings.Contains(r, "/containers/") {
			t.Errorf("Detect on the host requested %s", r)
		}
	}
}

func TestDetectUnavailable(t *testing.T) {
	old := startFake(t, "1.40")
	tests := []struct {
		name string
		o    Options
		also error
	}{
		{name: "old API", o: Options{Endpoint: old.ep.String()}, also: ErrAPIVersion},
		{name: "named socket missing", o: Options{Endpoint: "unix:///nonexistent/docker.sock"}},
		{name: "invalid endpoint", o: Options{Endpoint: "tcp://10.0.0.1:2375"}, also: core.ErrInvalid},
		{name: "nothing found", o: Options{GOOS: "linux", Home: "/nonexistent", Getenv: func(string) string { return "" }}},
	}
	for _, tt := range tests {
		if tt.name == "nothing found" {
			if _, err := os.Stat("/var/run/docker.sock"); err == nil {
				continue // this machine has an engine at the usual socket
			}
		}
		rt, err := Detect(t.Context(), tt.o)
		if !errors.Is(err, ErrUnavailable) || (tt.also != nil && !errors.Is(err, tt.also)) || rt.Client != nil {
			t.Errorf("%s: Detect = %+v, %v, want ErrUnavailable and %v", tt.name, rt, err, tt.also)
		}
	}
}

func TestErrors(t *testing.T) {
	f := startFake(t, "1.47")
	f.route("POST /v1.47/containers/create", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusConflict)
		_, _ = io.WriteString(w, `{"message":"Conflict. The container name \"/mavio-voice-whisper\" is already in use"}`)
	})
	c := NewClient(f.ep)
	defer c.Close()
	ctx := t.Context()
	_, err := c.InspectContainer(ctx, "missing")
	var e *Error
	if !errors.Is(err, core.ErrNotFound) || !errors.As(err, &e) || e.Status != 404 || !strings.Contains(e.Message, "no such route") {
		t.Errorf("InspectContainer = %v, want core.ErrNotFound with the engine's message", err)
	}
	if _, err := c.CreateContainer(ctx, ContainerSpec{Name: "mavio-voice-whisper", Image: "x"}); !errors.Is(err, core.ErrConflict) {
		t.Errorf("CreateContainer = %v, want core.ErrConflict", err)
	}
	if got := f.requests(); got[0] != "GET /version?" {
		t.Errorf("first request = %q, want the version negotiated first", got[0])
	}
}

func TestCreateContainer(t *testing.T) {
	f := startFake(t, "1.47")
	f.route("POST /v1.47/containers/create", jsonRoute(map[string]any{"Id": "c1"}))
	c := NewClient(f.ep)
	defer c.Close()
	labels := map[string]string{"dev.mavio.plugin": "voice"}
	id, err := c.CreateContainer(t.Context(), ContainerSpec{
		Name: "mavio-voice-whisper", Image: "busybox@sha256:abc", Cmd: []string{"serve"}, Env: []string{"A=1"},
		Labels: labels, Port: 9000, PublishLoopback: true, Network: "mavio", Aliases: []string{"whisper"},
		Volumes: map[string]string{"mavio-voice-whisper-cache": "/root/.cache"}, Memory: 1 << 30, CPUs: 1.5, GPU: GPURuntime,
		HostGateway: true,
	})
	if err != nil || id != "c1" {
		t.Fatalf("CreateContainer = %q, %v, want c1", id, err)
	}
	if got := f.requests()[1]; got != "POST /v1.47/containers/create?name=mavio-voice-whisper" {
		t.Errorf("request = %q", got)
	}
	var got, want any
	if err := json.Unmarshal([]byte(f.body("POST /v1.47/containers/create")), &got); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(`{
		"Image": "busybox@sha256:abc", "Cmd": ["serve"], "Env": ["A=1"], "Labels": {"dev.mavio.plugin": "voice"},
		"ExposedPorts": {"9000/tcp": {}},
		"HostConfig": {
			"PortBindings": {"9000/tcp": [{"HostIp": "127.0.0.1", "HostPort": ""}]},
			"Mounts": [{"Type": "volume", "Source": "mavio-voice-whisper-cache", "Target": "/root/.cache", "VolumeOptions": {"Labels": {"dev.mavio.plugin": "voice"}}}],
			"NetworkMode": "mavio", "Memory": 1073741824, "NanoCpus": 1500000000,
			"DeviceRequests": [{"Count": -1, "Capabilities": [["gpu"]]}],
			"ExtraHosts": ["host.docker.internal:host-gateway"],
			"RestartPolicy": {"Name": "no"}
		},
		"NetworkingConfig": {"EndpointsConfig": {"mavio": {"Aliases": ["whisper"]}}}
	}`), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("create body = %v, want %v", got, want)
	}
}

func TestCreateContainerBindsAndPeerPort(t *testing.T) {
	f := startFake(t, "1.47")
	f.route("POST /v1.47/containers/create", jsonRoute(map[string]any{"Id": "c1"}))
	c := NewClient(f.ep)
	defer c.Close()
	if _, err := c.CreateContainer(t.Context(), ContainerSpec{
		Name: "mavio-download-clients-qbittorrent", Image: "qbittorrent", Entrypoint: []string{"/bin/sh"}, Cmd: []string{"-c", "exec /init"},
		PeerPort: 6881, Binds: []Bind{{Source: "/srv/downloads", Target: "/downloads"}, {Volume: "mavio-data", Subpath: "downloads", Target: "/more"}},
	}); err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal([]byte(f.body("POST /v1.47/containers/create")), &got); err != nil {
		t.Fatal(err)
	}
	_ = json.Unmarshal([]byte(`{
		"Image": "qbittorrent", "Entrypoint": ["/bin/sh"], "Cmd": ["-c", "exec /init"],
		"ExposedPorts": {"6881/tcp": {}, "6881/udp": {}},
		"HostConfig": {
			"PortBindings": {"6881/tcp": [{"HostIp": "", "HostPort": "6881"}], "6881/udp": [{"HostIp": "", "HostPort": "6881"}]},
			"Mounts": [
				{"Type": "bind", "Source": "/srv/downloads", "Target": "/downloads"},
				{"Type": "volume", "Source": "mavio-data", "Target": "/more", "VolumeOptions": {"Subpath": "downloads"}}
			],
			"RestartPolicy": {"Name": "no"}
		}
	}`), &want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("create body = %v, want %v", got, want)
	}
}

func TestContainerLifecycle(t *testing.T) {
	f := startFake(t, "1.47")
	ok := func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }
	f.route("POST /v1.47/containers/c1/start", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotModified) })
	f.route("POST /v1.47/containers/c1/stop", ok)
	f.route("DELETE /v1.47/containers/c1", ok)
	f.route("GET /v1.47/containers/json", jsonRoute([]map[string]any{{"Id": "c1", "Names": []string{"/mavio-voice-whisper"}, "Labels": map[string]string{"dev.mavio.plugin": "voice"}, "State": "running"}}))
	f.route("GET /v1.47/containers/c1/json", jsonRoute(map[string]any{
		"Id": "c1", "Name": "/mavio-voice-whisper", "State": map[string]any{"Status": "running", "Running": true},
		"NetworkSettings": map[string]any{"Ports": map[string]any{"9000/tcp": []map[string]string{{"HostIp": "127.0.0.1", "HostPort": "49153"}}}},
	}))
	c := NewClient(f.ep)
	defer c.Close()
	ctx := t.Context()
	if err := c.StartContainer(ctx, "c1"); err != nil {
		t.Errorf("StartContainer of a running container = %v, want nil", err)
	}
	ctr, err := c.InspectContainer(ctx, "c1")
	if err != nil || !ctr.State.Running {
		t.Fatalf("InspectContainer = %+v, %v", ctr, err)
	}
	if p, ok := ctr.PublishedPort(9000); p != 49153 || !ok {
		t.Errorf("PublishedPort(9000) = %d, %v, want 49153", p, ok)
	}
	if _, ok := ctr.PublishedPort(8080); ok {
		t.Errorf("PublishedPort(8080) found a port, want none")
	}
	list, err := c.ListContainers(ctx, map[string]string{"dev.mavio.plugin": "voice"})
	if err != nil || len(list) != 1 || list[0].ID != "c1" {
		t.Errorf("ListContainers = %+v, %v", list, err)
	}
	if err := c.StopContainer(ctx, "c1", 10*time.Second); err != nil {
		t.Error(err)
	}
	if err := c.RemoveContainer(ctx, "c1"); err != nil {
		t.Error(err)
	}
	want := []string{
		"GET /version?",
		"POST /v1.47/containers/c1/start?",
		"GET /v1.47/containers/c1/json?",
		`GET /v1.47/containers/json?all=true&filters=%7B%22label%22%3A%7B%22dev.mavio.plugin%3Dvoice%22%3Atrue%7D%7D`,
		"POST /v1.47/containers/c1/stop?t=10",
		"DELETE /v1.47/containers/c1?force=true&v=true",
	}
	if got := f.requests(); !slices.Equal(got, want) {
		t.Errorf("requests = %q, want %q", got, want)
	}
}

func TestPullImage(t *testing.T) {
	f := startFake(t, "1.47")
	f.route("POST /v1.47/images/create", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("tag") == "bad" {
			_, _ = io.WriteString(w, `{"status":"Pulling from library/busybox"}`+"\n"+`{"error":"manifest unknown"}`+"\n")
			return
		}
		_, _ = io.WriteString(w, `{"status":"Pulling from library/busybox","id":"latest"}
{"status":"Downloading","progressDetail":{"current":512,"total":2048},"id":"a1"}
{"status":"Pull complete","progressDetail":{},"id":"a1"}
`)
	})
	c := NewClient(f.ep)
	defer c.Close()
	var steps []PullProgress
	if err := c.PullImage(t.Context(), "busybox:1.37@sha256:abc", func(p PullProgress) { steps = append(steps, p) }); err != nil {
		t.Fatal(err)
	}
	want := []PullProgress{
		{Layer: "latest", Status: "Pulling from library/busybox"},
		{Layer: "a1", Status: "Downloading", Current: 512, Total: 2048},
		{Layer: "a1", Status: "Pull complete"},
	}
	if !slices.Equal(steps, want) {
		t.Errorf("progress = %+v, want %+v", steps, want)
	}
	if got := f.requests()[1]; got != "POST /v1.47/images/create?fromImage=busybox&tag=sha256%3Aabc" {
		t.Errorf("request = %q", got)
	}
	if err := c.PullImage(t.Context(), "busybox:bad", nil); err == nil || !strings.Contains(err.Error(), "manifest unknown") {
		t.Errorf("PullImage of a missing tag = %v, want the engine's error", err)
	}
}

func TestNetworksAndVolumes(t *testing.T) {
	f := startFake(t, "1.47")
	f.route("POST /v1.47/networks/create", jsonRoute(map[string]any{"Id": "n1"}))
	f.route("POST /v1.47/networks/mavio/connect", func(w http.ResponseWriter, _ *http.Request) {})
	f.route("GET /v1.47/networks/mavio", jsonRoute(map[string]any{"Id": "n1", "Name": "mavio", "Containers": map[string]any{"c1": map[string]string{"Name": "mavio-voice-whisper"}}}))
	f.route("GET /v1.47/volumes", jsonRoute(map[string]any{"Volumes": []map[string]any{{"Name": "mavio-voice-whisper-cache"}}}))
	f.route("DELETE /v1.47/volumes/mavio-voice-whisper-cache", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	c := NewClient(f.ep)
	defer c.Close()
	ctx := t.Context()
	labels := map[string]string{"dev.mavio.server": "s1"}
	if id, err := c.CreateNetwork(ctx, "mavio", labels); err != nil || id != "n1" {
		t.Errorf("CreateNetwork = %q, %v", id, err)
	}
	if got, want := f.body("POST /v1.47/networks/create"), `{"Name":"mavio","Driver":"bridge","CheckDuplicate":true,"Labels":{"dev.mavio.server":"s1"}}`; got != want {
		t.Errorf("network body = %s, want %s", got, want)
	}
	if err := c.ConnectNetwork(ctx, "mavio", "self", []string{"mavio"}); err != nil {
		t.Error(err)
	}
	if got, want := f.body("POST /v1.47/networks/mavio/connect"), `{"Container":"self","EndpointConfig":{"Aliases":["mavio"]}}`; got != want {
		t.Errorf("connect body = %s, want %s", got, want)
	}
	if n, err := c.InspectNetwork(ctx, "mavio"); err != nil || n.Containers["c1"].Name != "mavio-voice-whisper" {
		t.Errorf("InspectNetwork = %+v, %v", n, err)
	}
	if vs, err := c.ListVolumes(ctx, labels); err != nil || len(vs) != 1 || vs[0].Name != "mavio-voice-whisper-cache" {
		t.Errorf("ListVolumes = %+v, %v", vs, err)
	}
	if err := c.RemoveVolume(ctx, "mavio-voice-whisper-cache"); err != nil {
		t.Error(err)
	}
}

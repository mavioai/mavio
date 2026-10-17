package dependencies_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/dependencies"
	"github.com/mavioai/mavio/libs/containers"
	"github.com/mavioai/mavio/libs/containers/containerstest"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/store"
)

const (
	pluginID   = "dev.mavio.voice"
	testDigest = "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
)

// fakePlugins holds one plugin and counts its reconfigurations.
type fakePlugins struct {
	manifest     *pluginv1.Manifest
	reconfigured atomic.Int32
}

func (f *fakePlugins) Manifest(id string) (*pluginv1.Manifest, bool) {
	return f.manifest, id == f.manifest.GetId()
}

func (f *fakePlugins) Reconfigure(context.Context, string) error {
	f.reconfigured.Add(1)
	return nil
}

// onHost detects an engine with a client that reaches nothing, for a
// server on the host.
func onHost(context.Context) (containers.Runtime, error) {
	return containers.Runtime{
		Client:  containers.NewClient(containers.Endpoint{Network: containers.Unix, Address: "/run/none.sock"}),
		Version: containers.Version{Engine: "docker", Version: "29.0.0"}, APIVersion: "1.47",
	}, nil
}

func newService(t *testing.T, mut func(c *dependencies.Config)) (*dependencies.Service, *fakePlugins, *time.Time) {
	t.Helper()
	db, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	plugs := &fakePlugins{manifest: pluginv1.Manifest_builder{Id: new(pluginID), Containers: []*pluginv1.ContainerDependency{
		pluginv1.ContainerDependency_builder{
			Name: new("whisper"), Image: new("whisper:1" + testDigest), Port: new(uint32(9000)), HealthPath: new("/health"),
			Env: map[string]string{"API_KEY": "${secret:api-key}"}, Gpu: pluginv1.GpuSupport_GPU_SUPPORT_OPTIONAL.Enum(),
			GpuImage: new("whisper:1-gpu" + testDigest), Volumes: map[string]string{"cache": "/root/.cache"},
		}.Build(),
		pluginv1.ContainerDependency_builder{Name: new("tts"), Image: new("tts:1" + testDigest), Port: new(uint32(5000))}.Build(),
	}}.Build()}
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	cfg := dependencies.Config{
		Store: db.PluginContainers(), Plugins: plugs, Server: "server-1", Now: func() time.Time { return now },
		Detect: func(context.Context) (containers.Runtime, error) {
			return containers.Runtime{}, fmt.Errorf("%w: none", containers.ErrUnavailable)
		},
	}
	if mut != nil {
		mut(&cfg)
	}
	s := dependencies.New(cfg)
	t.Cleanup(func() { _ = s.Close(context.Background()) })
	return s, plugs, &now
}

func TestListUnset(t *testing.T) {
	s, _, _ := newService(t, nil)
	deps, err := s.List(t.Context(), pluginID)
	if err != nil {
		t.Fatal(err)
	}
	if len(deps) != 2 || deps[0].Spec.GetName() != "whisper" || deps[1].Spec.GetName() != "tts" {
		t.Fatalf("List = %+v, want whisper and tts in manifest order", deps)
	}
	for _, d := range deps {
		if st := d.Status; st.GetMode() != pluginv1.ContainerMode_CONTAINER_MODE_UNSPECIFIED || st.GetState() != pluginv1.ContainerState_CONTAINER_STATE_UNAVAILABLE {
			t.Errorf("status of %s = %v, want unset and unavailable", d.Spec.GetName(), st)
		}
	}
	if _, err := s.List(t.Context(), "org.example.none"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("List of an unknown plugin = %v, want core.ErrNotFound", err)
	}
}

func TestSetExternal(t *testing.T) {
	healthy := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/health" || !healthy {
			http.Error(w, "down", http.StatusServiceUnavailable)
		}
	}))
	t.Cleanup(srv.Close)
	s, plugs, now := newService(t, nil)
	ctx := t.Context()

	d, err := s.Set(ctx, pluginID, "whisper", core.ContainerExternal, srv.URL+"/", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st := d.Status; st.GetAddress() != srv.URL || st.GetState() != pluginv1.ContainerState_CONTAINER_STATE_READY {
		t.Errorf("status = %v, want ready at %s", st, srv.URL)
	}
	secret := d.Kept.Secrets["api-key"]
	if len(secret) < 20 || len(d.Kept.Secrets) != 1 || !d.Kept.UseGPU {
		t.Errorf("kept = %+v, want an api-key secret and a GPU", d.Kept)
	}
	if plugs.reconfigured.Load() != 1 {
		t.Errorf("reconfigured %d times, want 1", plugs.reconfigured.Load())
	}
	host := strings.TrimPrefix(srv.URL, "http://")
	if !s.AllowsHost(ctx, pluginID, host) || s.AllowsHost(ctx, pluginID, "example.com") {
		t.Errorf("AllowsHost does not tell %s from example.com", host)
	}
	deps, err := s.List(ctx, pluginID)
	if err != nil {
		t.Fatal(err)
	}
	if c, g := deps[0].Commands, deps[0].GPUCommands; c == nil || g == nil || !strings.Contains(c.DockerRun, secret) || !strings.Contains(g.DockerRun, "--gpus all") {
		t.Errorf("commands = %+v and %+v, want both, with the secret", c, g)
	}

	// The same address keeps the secret and leaves the plugin alone.
	d, err = s.Set(ctx, pluginID, "whisper", core.ContainerExternal, srv.URL, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kept.Secrets["api-key"] != secret || d.Kept.UseGPU || plugs.reconfigured.Load() != 1 {
		t.Errorf("kept = %+v after %d reconfigurations, want the same secret, no GPU and 1", d.Kept, plugs.reconfigured.Load())
	}

	// Health is checked again once its result is old.
	healthy = false
	deps, _ = s.List(ctx, pluginID)
	if st := deps[0].Status.GetState(); st != pluginv1.ContainerState_CONTAINER_STATE_READY {
		t.Errorf("state before the result aged = %v, want the last, ready", st)
	}
	*now = now.Add(time.Minute)
	deps, _ = s.List(ctx, pluginID)
	if st := deps[0].Status; st.GetState() != pluginv1.ContainerState_CONTAINER_STATE_UNHEALTHY || st.GetError() == "" {
		t.Errorf("status after the result aged = %v, want unhealthy with an error", st)
	}

	// Without a health path, the port must accept connections.
	d, err = s.Set(ctx, pluginID, "tts", core.ContainerExternal, srv.URL, false, nil)
	if err != nil || d.Status.GetState() != pluginv1.ContainerState_CONTAINER_STATE_READY || d.Kept.Secrets != nil {
		t.Errorf("Set of tts = %+v, %v, want ready without secrets", d, err)
	}

	// Unsetting it changes how it is reached.
	d, err = s.Set(ctx, pluginID, "whisper", core.ContainerUnset, "", false, nil)
	if err != nil || d.Status.GetState() != pluginv1.ContainerState_CONTAINER_STATE_UNAVAILABLE || d.Kept.Secrets["api-key"] != secret {
		t.Errorf("Set to unset = %+v, %v, want unavailable with the secret kept", d, err)
	}
	if plugs.reconfigured.Load() != 3 {
		t.Errorf("reconfigured %d times, want 3", plugs.reconfigured.Load())
	}
}

func TestSetInvalid(t *testing.T) {
	s, _, _ := newService(t, nil)
	ctx := t.Context()
	tests := []struct {
		name, plugin, container string
		mode                    core.ContainerMode
		address                 string
		gpu                     bool
		want                    error
	}{
		{"unknown plugin", "org.example.none", "whisper", core.ContainerUnset, "", false, core.ErrNotFound},
		{"unknown container", pluginID, "none", core.ContainerUnset, "", false, core.ErrNotFound},
		{"external without address", pluginID, "whisper", core.ContainerExternal, "", false, core.ErrInvalid},
		{"unset with address", pluginID, "whisper", core.ContainerUnset, "http://a:1", false, core.ErrInvalid},
		{"GPU it cannot use", pluginID, "tts", core.ContainerUnset, "", true, core.ErrInvalid},
		{"managed without engine", pluginID, "whisper", core.ContainerManaged, "", false, dependencies.ErrNoEngine},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := s.Set(ctx, tt.plugin, tt.container, tt.mode, tt.address, tt.gpu, nil); !errors.Is(err, tt.want) {
				t.Errorf("Set = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRuntime(t *testing.T) {
	calls := 0
	s, _, _ := newService(t, func(c *dependencies.Config) {
		c.Detect = func(ctx context.Context) (containers.Runtime, error) {
			calls++
			return onHost(ctx)
		}
	})
	ctx := t.Context()
	for range 2 {
		if rt, err := s.Runtime(ctx, false); err != nil || rt.Version.Engine != "docker" {
			t.Fatalf("Runtime = %+v, %v, want docker", rt, err)
		}
	}
	if _, err := s.Runtime(ctx, true); err != nil || calls != 2 {
		t.Errorf("Runtime detected %d times, %v, want 2", calls, err)
	}
}

func TestManagedInContainer(t *testing.T) {
	for _, self := range []*containers.Container{nil, {ID: "self"}} {
		s, _, _ := newService(t, func(c *dependencies.Config) {
			c.Detect = func(ctx context.Context) (containers.Runtime, error) {
				rt, err := onHost(ctx)
				rt.InContainer, rt.Self = true, self
				return rt, err
			}
			c.Engine = func(containers.Runtime) containers.Engine { return containerstest.New() }
		})
		_, err := s.Set(t.Context(), pluginID, "whisper", core.ContainerManaged, "", false, nil)
		if self == nil && !errors.Is(err, dependencies.ErrNoEngine) {
			t.Errorf("Set of managed = %v, want ErrNoEngine without the server's container", err)
		}
		if self != nil && err != nil {
			t.Errorf("Set of managed = %v, want nil", err)
		}
	}
}

// eventually waits for cond to hold.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if cond() {
			return
		}
	}
	t.Fatalf("timed out waiting for %s", what)
}

func TestManaged(t *testing.T) {
	engine := containerstest.New()
	s, plugs, _ := newService(t, func(c *dependencies.Config) {
		c.Detect = onHost
		c.Engine = func(containers.Runtime) containers.Engine { return engine }
		c.Poll = 5 * time.Millisecond
		c.Probe = func(context.Context, string, string) error { return nil }
	})
	ctx := t.Context()
	state := func(name string) *pluginv1.ContainerStatus {
		deps, err := s.List(ctx, pluginID)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range deps {
			if d.Spec.GetName() == name {
				return d.Status
			}
		}
		t.Fatalf("no dependency %s", name)
		return nil
	}

	// Managed before the plugin runs, it waits for the plugin.
	d, err := s.Set(ctx, pluginID, "whisper", core.ContainerManaged, "", true, nil)
	if err != nil {
		t.Fatal(err)
	}
	if st := d.Status; st.GetMode() != pluginv1.ContainerMode_CONTAINER_MODE_MANAGED || st.GetState() != pluginv1.ContainerState_CONTAINER_STATE_UNAVAILABLE {
		t.Errorf("status before the plugin runs = %v, want managed and unavailable", st)
	}
	if got := engine.Took(); len(got) != 0 {
		t.Errorf("calls before the plugin runs = %q, want none", got)
	}
	secret := d.Kept.Secrets["api-key"]

	s.PluginStarted(ctx, plugs.manifest)
	eventually(t, "whisper ready", func() bool {
		return state("whisper").GetState() == pluginv1.ContainerState_CONTAINER_STATE_READY
	})
	if got := state("whisper").GetAddress(); got != "http://127.0.0.1:49001" {
		t.Errorf("address = %q, want the published port", got)
	}
	spec, running, _ := engine.Spec("mavio-voice-whisper")
	if !running || !slices.Equal(spec.Env, []string{"API_KEY=" + secret}) || !spec.GPU ||
		spec.Image != "whisper:1-gpu"+testDigest || spec.Labels[containers.LabelServer] != "server-1" {
		t.Errorf("container = %+v, running %v, want the GPU image with the secret", spec, running)
	}
	eventually(t, "the plugin reconfigured", func() bool { return plugs.reconfigured.Load() == 2 })

	// Leaving managed removes the container and keeps its volume.
	engine.Took()
	if _, err := s.Set(ctx, pluginID, "whisper", core.ContainerUnset, "", false, nil); err != nil {
		t.Fatal(err)
	}
	if got, want := engine.Took(), []string{"remove mavio-voice-whisper"}; !slices.Equal(got, want) {
		t.Errorf("calls of leaving managed = %q, want %q", got, want)
	}
	if !engine.HasVolume("mavio-voice-whisper-cache") {
		t.Error("leaving managed removed the volume")
	}

	// Managed while the plugin runs starts at once; stopping the plugin stops it.
	if _, err := s.Set(ctx, pluginID, "whisper", core.ContainerManaged, "", false, nil); err != nil {
		t.Fatal(err)
	}
	eventually(t, "whisper ready again", func() bool {
		return state("whisper").GetState() == pluginv1.ContainerState_CONTAINER_STATE_READY
	})
	if spec, _, _ := engine.Spec("mavio-voice-whisper"); spec.GPU || spec.Image != "whisper:1"+testDigest {
		t.Errorf("container = %+v, want the CPU image", spec)
	}
	s.PluginStopped(ctx, pluginID)
	if _, running, _ := engine.Spec("mavio-voice-whisper"); running {
		t.Error("the container runs after its plugin stopped")
	}
	if st := state("whisper"); st.GetState() != pluginv1.ContainerState_CONTAINER_STATE_UNAVAILABLE {
		t.Errorf("status after the plugin stopped = %v, want unavailable", st)
	}

	// Reconcile removes containers of dependencies not managed.
	s.PluginStarted(ctx, plugs.manifest)
	eventually(t, "whisper running", func() bool { _, running, _ := engine.Spec("mavio-voice-whisper"); return running })
	engine.AddImage("tts:1" + testDigest)
	if _, err := engine.CreateContainer(ctx, containers.ContainerSpec{Name: "mavio-voice-tts", Image: "tts:1" + testDigest, Labels: map[string]string{
		containers.LabelServer: "server-1", containers.LabelPlugin: pluginID, containers.LabelDependency: "tts",
	}}); err != nil {
		t.Fatal(err)
	}
	engine.Took()
	if err := s.Reconcile(ctx); err != nil {
		t.Fatal(err)
	}
	if got, want := engine.Took(), []string{"remove mavio-voice-tts"}; !slices.Equal(got, want) {
		t.Errorf("calls of Reconcile = %q, want %q", got, want)
	}

	// Uninstalling removes containers and volumes.
	s.PluginRemoved(ctx, pluginID)
	if _, _, ok := engine.Spec("mavio-voice-whisper"); ok || engine.HasVolume("mavio-voice-whisper-cache") {
		t.Error("the container or its volume is left after the plugin was removed")
	}
}

// queue is a job queue that hands the jobs enqueued to the test.
type queue struct {
	core.JobQueue
	jobs chan core.Job
}

func (q *queue) Enqueue(_ context.Context, j *core.Job) (bool, error) {
	q.jobs <- *j
	return true, nil
}

func TestPullJob(t *testing.T) {
	engine := containerstest.New()
	engine.OnPull(containers.PullProgress{Layer: "a", Status: "Downloading", Current: 1, Total: 4})
	q := &queue{jobs: make(chan core.Job, 4)}
	s, plugs, _ := newService(t, func(c *dependencies.Config) {
		c.Detect = onHost
		c.Engine = func(containers.Runtime) containers.Engine { return engine }
		c.Jobs = q
		c.Poll = 5 * time.Millisecond
		c.Probe = func(context.Context, string, string) error { return nil }
	})
	ctx := t.Context()
	if _, err := s.Set(ctx, pluginID, "whisper", core.ContainerManaged, "", false, nil); err != nil {
		t.Fatal(err)
	}
	s.PluginStarted(ctx, plugs.manifest)
	var job core.Job
	select {
	case job = <-q.jobs:
	case <-time.After(5 * time.Second):
		t.Fatal("no pull job was queued")
	}
	if job.Kind != dependencies.JobPull || !strings.Contains(string(job.Payload), "whisper:1") {
		t.Errorf("job = %s %s, want a pull of whisper:1", job.Kind, job.Payload)
	}
	deps, err := s.List(ctx, pluginID)
	if err != nil {
		t.Fatal(err)
	}
	if st := deps[0].Status.GetState(); st != pluginv1.ContainerState_CONTAINER_STATE_PULLING {
		t.Errorf("state while the job waits = %v, want pulling", st)
	}
	if _, err := s.RunPull(ctx, job); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.JobProgress(job.ID); ok {
		t.Error("JobProgress of a finished job is set")
	}
	eventually(t, "whisper ready", func() bool {
		deps, _ := s.List(ctx, pluginID)
		return deps[0].Status.GetState() == pluginv1.ContainerState_CONTAINER_STATE_READY
	})
}

func TestManagedProxy(t *testing.T) {
	engine := containerstest.New()
	const office = "01a14422-45e1-730c-a64f-a7e1a6b5e6b7"
	var systemSet atomic.Bool
	changed := make(chan string, 4)
	s, plugs, _ := newService(t, func(c *dependencies.Config) {
		c.Detect = onHost
		c.Engine = func(containers.Runtime) containers.Engine { return engine }
		c.Poll = 5 * time.Millisecond
		c.Probe = func(context.Context, string, string) error { return nil }
		c.ProxyEnv = func(_ context.Context, _ *pluginv1.Manifest, pc core.PluginContainer) func(string) map[string]string {
			if pc.Proxy == "" || pc.Proxy == core.SystemProxyID && !systemSet.Load() {
				return nil
			}
			return func(host string) map[string]string {
				return map[string]string{"HTTPS_PROXY": "http://" + host + ":8686"}
			}
		}
		c.CheckProxy = func(_ context.Context, id string) error {
			if id == "gone" {
				return core.ErrInvalid
			}
			return nil
		}
		c.ProxyChanged = func(container string) { changed <- container }
	})
	ctx := t.Context()
	proxied := func() bool {
		spec, _, _ := engine.Spec("mavio-voice-whisper")
		return slices.Contains(spec.Env, "HTTPS_PROXY=http://host.docker.internal:8686")
	}
	s.PluginStarted(ctx, plugs.manifest)

	// Through the system proxy, set to none, it reaches sites directly.
	d, err := s.Set(ctx, pluginID, "whisper", core.ContainerManaged, "", false, nil)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kept.Proxy != core.SystemProxyID || !strings.HasPrefix(d.Command, "docker run -d --name mavio-voice-whisper") {
		t.Errorf("new managed dependency got proxy %q, command %q; want the system proxy and its docker run", d.Kept.Proxy, d.Command)
	}
	eventually(t, "whisper running", func() bool { _, running, _ := engine.Spec("mavio-voice-whisper"); return running })
	if proxied() {
		t.Error("the container goes through the gateway with no system proxy set")
	}
	if got, ok := s.ProxyOf(ctx, "mavio-voice-whisper"); !ok || got != core.SystemProxyID {
		t.Errorf("ProxyOf got = %q, %v, want = system", got, ok)
	}

	// The system proxy set, the container is created again through it.
	systemSet.Store(true)
	s.ProxiesChanged(ctx)
	eventually(t, "whisper through the gateway", proxied)

	// Another proxy keeps the container, its connections ended.
	engine.Took()
	p := office
	if _, err := s.Set(ctx, pluginID, "whisper", core.ContainerManaged, "", false, &p); err != nil {
		t.Fatal(err)
	}
	if got := <-changed; got != "mavio-voice-whisper" {
		t.Errorf("ProxyChanged got = %q, want = mavio-voice-whisper", got)
	}
	if got := engine.Took(); slices.ContainsFunc(got, func(c string) bool { return strings.HasPrefix(c, "remove") }) {
		t.Errorf("calls after another proxy got = %q, want no container removed", got)
	}
	p = "gone"
	if _, err := s.Set(ctx, pluginID, "whisper", core.ContainerManaged, "", false, &p); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("Set of a missing proxy got = %v, want = ErrInvalid", err)
	}
	if got, _ := s.ProxyOf(ctx, "mavio-voice-whisper"); got != office {
		t.Errorf("ProxyOf got = %q, want = %s", got, office)
	}

	// None reaches sites directly again.
	p = ""
	if _, err := s.Set(ctx, pluginID, "whisper", core.ContainerManaged, "", false, &p); err != nil {
		t.Fatal(err)
	}
	eventually(t, "whisper direct", func() bool {
		_, running, _ := engine.Spec("mavio-voice-whisper")
		return running && !proxied()
	})
	if _, ok := s.ProxyOf(ctx, "mavio-voice-other"); ok {
		t.Error("ProxyOf of no dependency got = ok, want = false")
	}
}

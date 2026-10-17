package dependencies_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/dependencies"
	"github.com/mavioai/mavio/libs/containers"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/store"
)

const pluginID = "dev.mavio.voice"

// fakePlugins holds one plugin and counts its reconfigurations.
type fakePlugins struct {
	manifest     *pluginv1.Manifest
	reconfigured int
}

func (f *fakePlugins) Manifest(id string) (*pluginv1.Manifest, bool) {
	return f.manifest, id == f.manifest.GetId()
}

func (f *fakePlugins) Reconfigure(context.Context, string) error {
	f.reconfigured++
	return nil
}

func newService(t *testing.T, detect func(context.Context) (containers.Runtime, error)) (*dependencies.Service, *fakePlugins, *time.Time) {
	t.Helper()
	db, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	digest := "@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	plugs := &fakePlugins{manifest: pluginv1.Manifest_builder{Id: new(pluginID), Containers: []*pluginv1.ContainerDependency{
		pluginv1.ContainerDependency_builder{
			Name: new("whisper"), Image: new("whisper:1" + digest), Port: new(uint32(9000)), HealthPath: new("/health"),
			Env: map[string]string{"API_KEY": "${secret:api-key}"}, Gpu: pluginv1.GpuSupport_GPU_SUPPORT_OPTIONAL.Enum(),
		}.Build(),
		pluginv1.ContainerDependency_builder{Name: new("tts"), Image: new("tts:1" + digest), Port: new(uint32(5000))}.Build(),
	}}.Build()}
	now := time.Date(2026, 10, 17, 12, 0, 0, 0, time.UTC)
	if detect == nil {
		detect = func(context.Context) (containers.Runtime, error) {
			return containers.Runtime{}, fmt.Errorf("%w: none", containers.ErrUnavailable)
		}
	}
	s := dependencies.New(dependencies.Config{Store: db.PluginContainers(), Plugins: plugs, Detect: detect, Now: func() time.Time { return now }})
	t.Cleanup(s.Close)
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

	d, err := s.Set(ctx, pluginID, "whisper", core.ContainerExternal, srv.URL+"/", true)
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
	if plugs.reconfigured != 1 {
		t.Errorf("reconfigured %d times, want 1", plugs.reconfigured)
	}

	// The same address keeps the secret and leaves the plugin alone.
	d, err = s.Set(ctx, pluginID, "whisper", core.ContainerExternal, srv.URL, false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Kept.Secrets["api-key"] != secret || d.Kept.UseGPU || plugs.reconfigured != 1 {
		t.Errorf("kept = %+v after %d reconfigurations, want the same secret, no GPU and 1", d.Kept, plugs.reconfigured)
	}

	// Health is checked again once its result is old.
	healthy = false
	deps, _ := s.List(ctx, pluginID)
	if st := deps[0].Status.GetState(); st != pluginv1.ContainerState_CONTAINER_STATE_READY {
		t.Errorf("state before the result aged = %v, want the last, ready", st)
	}
	*now = now.Add(time.Minute)
	deps, _ = s.List(ctx, pluginID)
	if st := deps[0].Status; st.GetState() != pluginv1.ContainerState_CONTAINER_STATE_UNHEALTHY || st.GetError() == "" {
		t.Errorf("status after the result aged = %v, want unhealthy with an error", st)
	}

	// Without a health path, the port must accept connections.
	d, err = s.Set(ctx, pluginID, "tts", core.ContainerExternal, srv.URL, false)
	if err != nil || d.Status.GetState() != pluginv1.ContainerState_CONTAINER_STATE_READY || d.Kept.Secrets != nil {
		t.Errorf("Set of tts = %+v, %v, want ready without secrets", d, err)
	}

	// Unsetting it changes how it is reached.
	d, err = s.Set(ctx, pluginID, "whisper", core.ContainerUnset, "", false)
	if err != nil || d.Status.GetState() != pluginv1.ContainerState_CONTAINER_STATE_UNAVAILABLE || d.Kept.Secrets["api-key"] != secret {
		t.Errorf("Set to unset = %+v, %v, want unavailable with the secret kept", d, err)
	}
	if plugs.reconfigured != 3 {
		t.Errorf("reconfigured %d times, want 3", plugs.reconfigured)
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
			if _, err := s.Set(ctx, tt.plugin, tt.container, tt.mode, tt.address, tt.gpu); !errors.Is(err, tt.want) {
				t.Errorf("Set = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestRuntime(t *testing.T) {
	calls := 0
	s, _, _ := newService(t, func(context.Context) (containers.Runtime, error) {
		calls++
		return containers.Runtime{
			Client:  containers.NewClient(containers.Endpoint{Network: containers.Unix, Address: "/run/none.sock"}),
			Version: containers.Version{Engine: "docker", Version: "29.0.0"}, APIVersion: "1.47",
		}, nil
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
	if _, err := s.Set(ctx, pluginID, "whisper", core.ContainerManaged, "", false); !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("Set of managed = %v, want errors.ErrUnsupported until managed dependencies run", err)
	}
}

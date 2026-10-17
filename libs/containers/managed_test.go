package containers_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/libs/containers"
	"github.com/mavioai/mavio/libs/containers/containerstest"
	"github.com/mavioai/mavio/libs/core"
)

// health answers probes by address: unhealthy ones fail.
type health struct {
	mu        sync.Mutex
	unhealthy map[string]bool
	down      bool
}

func (h *health) probe(_ context.Context, address, path string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if path != "/health" {
		return fmt.Errorf("probe of %q, want /health", path)
	}
	if h.down || h.unhealthy[address] {
		return errors.New("503 Service containers.Unavailable")
	}
	return nil
}

func (h *health) set(down bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.down = down
}

// changes records the statuses reported.
type changes struct {
	mu   sync.Mutex
	seen []string
}

func (c *changes) record(plugin, name string, st containers.Status) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.seen = append(c.seen, fmt.Sprintf("%s/%s %s %s", plugin, name, st.State, st.Address))
}

func (c *changes) took() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := c.seen
	c.seen = nil
	return out
}

func whisper() containers.Dependency {
	return containers.Dependency{
		Plugin: "dev.mavio.voice", Name: "whisper", Image: "whisper:1@sha256:abc", Port: 9000, HealthPath: "/health",
		Env:     map[string]string{"B": "2", "A": "${not resolved here}", "SELF": "http://mavio-voice-whisper:9000"},
		Volumes: map[string]string{"cache": "/root/.cache"}, Memory: 1 << 30, CPUs: 2,
	}
}

func newTestSupervisor(e *containerstest.Engine, h *health, c *changes) *containers.Supervisor {
	return containers.NewSupervisor(containers.SupervisorOptions{
		Engine: e, Server: "server-1", OnChange: c.record, Probe: h.probe,
	})
}

func TestNames(t *testing.T) {
	n := containers.Names{}
	if got := n.Container("dev.mavio.voice", "whisper"); got != "mavio-voice-whisper" {
		t.Errorf("Container = %q, want mavio-voice-whisper", got)
	}
	if got := (containers.Names{Prefix: "mavio2"}).Volume("dev.mavio.voice", "whisper", "cache"); got != "mavio2-voice-whisper-cache" {
		t.Errorf("Volume = %q, want mavio2-voice-whisper-cache", got)
	}
	if got := n.Network(); got != "mavio" {
		t.Errorf("Network = %q, want mavio", got)
	}
	for prefix, ok := range map[string]bool{"": true, "mavio2": true, "my-mavio": true, "Mavio": false, "-m": false, "m-": false, "a--b": false, "a_b": false} {
		if err := (containers.Names{Prefix: prefix}).Validate(); (err == nil) != ok {
			t.Errorf("Validate of prefix %q = %v, want ok %v", prefix, err, ok)
		}
	}
	a, b := whisper(), whisper()
	b.Env["B"] = "3"
	if containers.Hash(a) == containers.Hash(b) || containers.Hash(a) != containers.Hash(whisper()) {
		t.Error("Hash does not tell dependencies apart")
	}
}

func TestSupervisor(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, h, c := containerstest.New(), &health{}, &changes{}
		h.set(true)
		s := newTestSupervisor(e, h, c)
		defer s.Close(context.Background())

		s.Start(whisper())
		synctest.Wait()
		if got, want := e.Took(), []string{
			"create network mavio", "pull whisper:1@sha256:abc", "create mavio-voice-whisper", "start mavio-voice-whisper",
		}; !slices.Equal(got, want) {
			t.Errorf("calls = %q, want %q", got, want)
		}
		spec, _, _ := e.Spec("mavio-voice-whisper")
		if spec.Labels[containers.LabelServer] != "server-1" || spec.Labels[containers.LabelPlugin] != "dev.mavio.voice" ||
			spec.Labels[containers.LabelDependency] != "whisper" || spec.Labels[containers.LabelSpec] != containers.Hash(whisper()) ||
			spec.VolumeLabels[containers.LabelSpec] != "" || spec.VolumeLabels[containers.LabelDependency] != "whisper" {
			t.Errorf("labels = %v, volume labels = %v", spec.Labels, spec.VolumeLabels)
		}
		if want := []string{"A=${not resolved here}", "B=2", "SELF=http://mavio-voice-whisper:9000"}; !slices.Equal(spec.Env, want) {
			t.Errorf("Env = %q, want %q", spec.Env, want)
		}
		if spec.Volumes["mavio-voice-whisper-cache"] != "/root/.cache" || !spec.PublishLoopback || spec.Network != "mavio" ||
			!slices.Equal(spec.Aliases, []string{"mavio-voice-whisper"}) || spec.Memory != 1<<30 || spec.CPUs != 2 {
			t.Errorf("spec = %+v", spec)
		}
		if st, _ := s.Status("dev.mavio.voice", "whisper"); st.State != containers.Starting || st.Address != "http://127.0.0.1:49001" {
			t.Errorf("Status while the health check fails = %+v, want starting at port 49001", st)
		}

		h.set(false)
		time.Sleep(time.Second)
		synctest.Wait()
		if got, want := c.took(), []string{
			"dev.mavio.voice/whisper pulling ", "dev.mavio.voice/whisper starting http://127.0.0.1:49001",
			"dev.mavio.voice/whisper ready http://127.0.0.1:49001",
		}; !slices.Equal(got, want) {
			t.Errorf("changes = %q, want %q", got, want)
		}

		// The same dependency changes nothing; another replaces the container.
		s.Start(whisper())
		synctest.Wait()
		if got := e.Took(); len(got) != 0 {
			t.Errorf("calls of Start with the same dependency = %q, want none", got)
		}
		changed := whisper()
		changed.Env["B"] = "3"
		s.Start(changed)
		synctest.Wait()
		if got, want := e.Took(), []string{"remove mavio-voice-whisper", "create mavio-voice-whisper", "start mavio-voice-whisper"}; !slices.Equal(got, want) {
			t.Errorf("calls of Start with another dependency = %q, want %q", got, want)
		}
		if st, _ := s.Status("dev.mavio.voice", "whisper"); st.State != containers.Ready || st.Address != "http://127.0.0.1:49002" {
			t.Errorf("Status after the replacement = %+v, want ready at port 49002", st)
		}
		c.took()

		// Three failed health checks in a row restart it.
		h.set(true)
		time.Sleep(90 * time.Second)
		synctest.Wait()
		if st, _ := s.Status("dev.mavio.voice", "whisper"); st.State != containers.Unhealthy || st.Err == nil {
			t.Errorf("Status after three failures = %+v, want unhealthy with the error", st)
		}
		if got, want := e.Took(), []string{"stop mavio-voice-whisper"}; !slices.Equal(got, want) {
			t.Errorf("calls after three failures = %q, want %q", got, want)
		}
		h.set(false)
		time.Sleep(time.Second)
		synctest.Wait()
		if got, want := e.Took(), []string{"start mavio-voice-whisper"}; !slices.Equal(got, want) {
			t.Errorf("calls after the backoff = %q, want %q", got, want)
		}
		if st, _ := s.Status("dev.mavio.voice", "whisper"); st.State != containers.Ready || st.Address != "http://127.0.0.1:49003" {
			t.Errorf("Status after the restart = %+v, want ready at its new port", st)
		}

		// A container exiting before it is ready is restarted with backoff.
		h.set(true)
		time.Sleep(90 * time.Second)
		synctest.Wait()
		time.Sleep(time.Second)
		synctest.Wait()
		e.Took()
		e.Exit("mavio-voice-whisper")
		time.Sleep(time.Second)
		synctest.Wait()
		if st, _ := s.Status("dev.mavio.voice", "whisper"); st.State != containers.Unhealthy || !strings.Contains(fmt.Sprint(st.Err), "exited") {
			t.Errorf("Status after the container exited = %+v, want unhealthy, exited", st)
		}

		// Stop stops the container and ends the run.
		if err := s.Stop(context.Background(), "dev.mavio.voice", ""); err != nil {
			t.Fatal(err)
		}
		if _, ok := s.Status("dev.mavio.voice", "whisper"); ok {
			t.Error("Status after Stop found the run")
		}
		e.Took()
		time.Sleep(time.Hour)
		synctest.Wait()
		if got := e.Took(); len(got) != 0 {
			t.Errorf("calls after Stop = %q, want none", got)
		}
	})
}

func TestSupervisorConflict(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, h, c := containerstest.New(), &health{}, &changes{}
		e.AddImage("whisper:1@sha256:abc")
		if _, err := e.CreateContainer(context.Background(), containers.ContainerSpec{
			Name: "mavio-voice-whisper", Image: "whisper:1@sha256:abc", Labels: map[string]string{containers.LabelServer: "server-2"},
		}); err != nil {
			t.Fatal(err)
		}
		s := newTestSupervisor(e, h, c)
		defer s.Close(context.Background())
		s.Start(whisper())
		synctest.Wait()
		if st, _ := s.Status("dev.mavio.voice", "whisper"); st.State != containers.Unavailable || !errors.Is(st.Err, core.ErrConflict) {
			t.Errorf("Status = %+v, want unavailable with core.ErrConflict", st)
		}
		if spec, _, _ := e.Spec("mavio-voice-whisper"); spec.Labels[containers.LabelServer] != "server-2" {
			t.Error("the other server's container was replaced")
		}
	})
}

func TestSupervisorRemoveAndReconcile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		e, h, c := containerstest.New(), &health{}, &changes{}
		s := newTestSupervisor(e, h, c)
		ctx := context.Background()
		tts := whisper()
		tts.Name, tts.Volumes = "tts", map[string]string{"models": "/models"}
		other := whisper()
		other.Plugin = "org.example.other"
		for _, d := range []containers.Dependency{whisper(), tts, other} {
			s.Start(d)
		}
		synctest.Wait()
		e.Took()

		// Remove of one dependency keeps its volumes unless asked.
		if err := s.Remove(ctx, "dev.mavio.voice", "tts", false); err != nil {
			t.Fatal(err)
		}
		if got, want := e.Took(), []string{"remove mavio-voice-tts"}; !slices.Equal(got, want) {
			t.Errorf("calls of Remove = %q, want %q", got, want)
		}
		if got := s.Running("dev.mavio.voice"); !slices.Equal(got, []string{"whisper"}) {
			t.Errorf("Running = %q, want whisper", got)
		}

		// Reconcile removes the containers keep rejects.
		err := s.Reconcile(ctx, func(plugin, name string) bool { return plugin == "dev.mavio.voice" })
		if err != nil {
			t.Fatal(err)
		}
		if got, want := e.Took(), []string{"remove mavio-other-whisper"}; !slices.Equal(got, want) {
			t.Errorf("calls of Reconcile = %q, want %q", got, want)
		}

		// Remove of a plugin's removes its containers and volumes.
		if err := s.Remove(ctx, "dev.mavio.voice", "", true); err != nil {
			t.Fatal(err)
		}
		if got, want := e.Took(), []string{
			"remove mavio-voice-whisper", "remove volume mavio-voice-tts-models", "remove volume mavio-voice-whisper-cache",
		}; !slices.Equal(got, want) {
			t.Errorf("calls of Remove of a plugin = %q, want %q", got, want)
		}
		if !e.HasVolume("mavio-other-whisper-cache") {
			t.Error("another plugin's volume was removed")
		}

		// Close stops what runs.
		s.Start(whisper())
		synctest.Wait()
		e.Took()
		if err := s.Close(ctx); err != nil {
			t.Fatal(err)
		}
		if got, want := e.Took(), []string{"stop mavio-voice-whisper"}; !slices.Equal(got, want) {
			t.Errorf("calls of Close = %q, want %q", got, want)
		}
		s.Start(whisper())
		synctest.Wait()
		if got := e.Took(); len(got) != 0 {
			t.Errorf("calls of Start after Close = %q, want none", got)
		}
	})
}

package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// TestEngine runs a container on the engine Detect finds, skipping when
// there is none. It pulls busybox from Docker Hub.
func TestEngine(t *testing.T) {
	ctx := t.Context()
	rt, err := Detect(ctx, Options{})
	if errors.Is(err, ErrUnavailable) {
		t.Skipf("no container engine: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	c := rt.Client
	defer c.Close()
	t.Logf("engine %s %s, API %s, rootless %v", rt.Version.Engine, rt.Version.Version, rt.APIVersion, rt.Rootless)

	const image = "busybox:1.37"
	if _, err := c.InspectImage(ctx, image); errors.Is(err, core.ErrNotFound) {
		if err := c.PullImage(ctx, image, nil); err != nil {
			t.Skipf("cannot pull %s: %v", image, err)
		}
	} else if err != nil {
		t.Fatal(err)
	}

	run := fmt.Sprintf("mavio-test-%d", time.Now().UnixNano())
	labels := map[string]string{"dev.mavio.test": run}
	cleanup := func() {
		ctx := context.WithoutCancel(ctx)
		cs, _ := c.ListContainers(ctx, labels)
		for _, ctr := range cs {
			_ = c.RemoveContainer(ctx, ctr.ID)
		}
		vs, _ := c.ListVolumes(ctx, labels)
		for _, v := range vs {
			_ = c.RemoveVolume(ctx, v.Name)
		}
		_ = c.RemoveNetwork(ctx, run)
	}
	t.Cleanup(cleanup)

	if _, err := c.CreateNetwork(ctx, run, labels); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateNetwork(ctx, run, labels); !errors.Is(err, core.ErrConflict) {
		t.Errorf("CreateNetwork of a taken name = %v, want core.ErrConflict", err)
	}
	id, err := c.CreateContainer(ctx, ContainerSpec{
		Name:   run,
		Image:  image,
		Cmd:    []string{"sh", "-c", "echo ready > /www/index.html && exec httpd -f -p 8080 -h /www"},
		Labels: labels, Port: 8080, PublishLoopback: true, Network: run, Aliases: []string{"web"},
		Volumes: map[string]string{run + "-www": "/www"}, Memory: 64 << 20, CPUs: 0.5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateContainer(ctx, ContainerSpec{Name: run, Image: image}); !errors.Is(err, core.ErrConflict) {
		t.Errorf("CreateContainer of a taken name = %v, want core.ErrConflict", err)
	}
	if err := c.StartContainer(ctx, id); err != nil {
		t.Fatal(err)
	}
	ctr, err := c.InspectContainer(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if !ctr.State.Running || ctr.Config.Labels["dev.mavio.test"] != run || ctr.NetworkSettings.Networks[run].NetworkID == "" {
		t.Errorf("InspectContainer = %+v, want it running with its label on its network", ctr)
	}
	port, ok := ctr.PublishedPort(8080)
	if !ok {
		t.Fatalf("port 8080 is not published: %+v", ctr.NetworkSettings.Ports)
	}
	var body string
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port))
		if err != nil {
			continue
		}
		data, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		body = string(data)
		break
	}
	if body != "ready\n" {
		t.Errorf("GET of the published port = %q, want ready", body)
	}
	if vs, err := c.ListVolumes(ctx, labels); err != nil || len(vs) != 1 || vs[0].Name != run+"-www" {
		t.Errorf("ListVolumes = %+v, %v, want the container's labeled volume", vs, err)
	}
	if err := c.StopContainer(ctx, id, time.Second); err != nil {
		t.Error(err)
	}
	if err := c.StopContainer(ctx, id, time.Second); err != nil {
		t.Errorf("StopContainer of a stopped container = %v, want nil", err)
	}
	if err := c.RemoveContainer(ctx, id); err != nil {
		t.Error(err)
	}
	if _, err := c.InspectContainer(ctx, id); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("InspectContainer after removal = %v, want core.ErrNotFound", err)
	}
}

// TestEngineSupervisor runs a dependency with a Supervisor on the engine
// Detect finds, skipping when there is none.
func TestEngineSupervisor(t *testing.T) {
	ctx := t.Context()
	rt, err := Detect(ctx, Options{})
	if errors.Is(err, ErrUnavailable) {
		t.Skipf("no container engine: %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	c := rt.Client
	defer c.Close()
	const image = "busybox:1.37"
	if _, err := c.InspectImage(ctx, image); errors.Is(err, core.ErrNotFound) {
		if err := c.PullImage(ctx, image, nil); err != nil {
			t.Skipf("cannot pull %s: %v", image, err)
		}
	}

	run := fmt.Sprintf("mavio-test-%d", time.Now().UnixNano())
	s := NewSupervisor(SupervisorOptions{Engine: c, Names: Names{Prefix: run}, Server: run, Poll: 100 * time.Millisecond})
	t.Cleanup(func() {
		ctx := context.WithoutCancel(ctx)
		_ = s.Remove(ctx, "dev.mavio.test", "", true)
		_ = c.RemoveNetwork(ctx, run)
	})
	s.Start(Dependency{
		Plugin: "dev.mavio.test", Name: "web", Image: image, Port: 8080, HealthPath: "/health",
		Cmd:     []string{"sh", "-c", "echo $GREETING > /www/health && exec httpd -f -p 8080 -h /www"},
		Env:     map[string]string{"GREETING": "ready"},
		Volumes: map[string]string{"www": "/www"}, Memory: 64 << 20,
	})
	var st Status
	for deadline := time.Now().Add(30 * time.Second); time.Now().Before(deadline); time.Sleep(100 * time.Millisecond) {
		if st, _ = s.Status("dev.mavio.test", "web"); st.State == Ready {
			break
		}
	}
	if st.State != Ready {
		t.Fatalf("Status = %+v, want ready", st)
	}
	resp, err := http.Get(st.Address + "/health")
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if string(body) != "ready\n" {
		t.Errorf("GET /health = %q, want ready", body)
	}
	if n, err := c.InspectNetwork(ctx, run); err != nil || n.Containers == nil {
		t.Errorf("InspectNetwork = %+v, %v, want the container on it", n, err)
	}

	if err := s.Remove(ctx, "dev.mavio.test", "", true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.InspectContainer(ctx, run+"-test-web"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("InspectContainer after Remove = %v, want core.ErrNotFound", err)
	}
	if vs, err := c.ListVolumes(ctx, map[string]string{LabelServer: run}); err != nil || len(vs) != 0 {
		t.Errorf("ListVolumes after Remove = %+v, %v, want none", vs, err)
	}
}

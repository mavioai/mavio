package containerstest

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/containers"
	"github.com/mavioai/mavio/libs/core"
)

var _ containers.Engine = (*Engine)(nil)

// Engine is a container engine in memory, for tests. Started containers
// publish their port at a new host port each time, from 49001.
type Engine struct {
	mu         sync.Mutex
	images     map[string]bool
	networks   map[string]map[string]struct{ Name string } // members by ID
	containers map[string]*container                       // by name
	volumes    map[string]map[string]string
	nextPort   int
	calls      []string
}

type container struct {
	id      string
	spec    containers.ContainerSpec
	running bool
	port    int
}

// New returns an engine with nothing.
func New() *Engine {
	return &Engine{
		images: map[string]bool{}, networks: map[string]map[string]struct{ Name string }{}, containers: map[string]*container{},
		volumes: map[string]map[string]string{}, nextPort: 49000,
	}
}

func (e *Engine) call(format string, args ...any) {
	e.calls = append(e.calls, fmt.Sprintf(format, args...))
}

// Took returns and clears the calls made, such as "create <name>".
func (e *Engine) Took() []string {
	e.mu.Lock()
	defer e.mu.Unlock()
	out := e.calls
	e.calls = nil
	return out
}

func (e *Engine) find(id string) *container {
	if c, ok := e.containers[id]; ok {
		return c
	}
	for _, c := range e.containers {
		if c.id == id {
			return c
		}
	}
	return nil
}

func (e *Engine) InspectImage(_ context.Context, ref string) (containers.Image, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if !e.images[ref] {
		return containers.Image{}, core.ErrNotFound
	}
	return containers.Image{ID: ref}, nil
}

func (e *Engine) PullImage(_ context.Context, ref string, _ func(containers.PullProgress)) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.call("pull %s", ref)
	e.images[ref] = true
	return nil
}

func (e *Engine) InspectNetwork(_ context.Context, id string) (containers.Network, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	members, ok := e.networks[id]
	if !ok {
		return containers.Network{}, core.ErrNotFound
	}
	return containers.Network{Name: id, Containers: maps.Clone(members)}, nil
}

func (e *Engine) ConnectNetwork(_ context.Context, network, container string, _ []string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	members, ok := e.networks[network]
	if !ok {
		return core.ErrNotFound
	}
	e.call("connect %s %s", network, container)
	members[container] = struct{ Name string }{container}
	return nil
}

func (e *Engine) CreateNetwork(_ context.Context, name string, _ map[string]string) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.call("create network %s", name)
	e.networks[name] = map[string]struct{ Name string }{}
	return name, nil
}

func (e *Engine) CreateContainer(_ context.Context, s containers.ContainerSpec) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.containers[s.Name]; ok {
		return "", core.ErrConflict
	}
	if !e.images[s.Image] {
		return "", core.ErrNotFound
	}
	e.call("create %s", s.Name)
	c := &container{id: fmt.Sprintf("id-%s-%d", s.Name, len(e.calls)), spec: s}
	e.containers[s.Name] = c
	for v := range s.Volumes {
		if _, ok := e.volumes[v]; !ok {
			e.volumes[v] = s.VolumeLabels
		}
	}
	return c.id, nil
}

func (e *Engine) StartContainer(_ context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.find(id)
	if c == nil {
		return core.ErrNotFound
	}
	if !c.running {
		e.call("start %s", c.spec.Name)
		c.running = true
		e.nextPort++
		c.port = e.nextPort
	}
	return nil
}

func (e *Engine) StopContainer(_ context.Context, id string, _ time.Duration) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.find(id)
	if c == nil {
		return core.ErrNotFound
	}
	if c.running {
		e.call("stop %s", c.spec.Name)
		c.running = false
	}
	return nil
}

// Exit stops a container as if its process ended.
func (e *Engine) Exit(name string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.containers[name].running = false
}

func (e *Engine) InspectContainer(_ context.Context, id string) (containers.Container, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.find(id)
	if c == nil {
		return containers.Container{}, core.ErrNotFound
	}
	var out containers.Container
	out.ID, out.Name = c.id, "/"+c.spec.Name
	out.Config.Labels = c.spec.Labels
	out.State.Running = c.running
	if c.running && c.spec.PublishLoopback {
		out.NetworkSettings.Ports = map[string][]containers.PortBinding{
			fmt.Sprintf("%d/tcp", c.spec.Port): {{HostIP: "127.0.0.1", HostPort: fmt.Sprint(c.port)}},
		}
	}
	return out, nil
}

func matches(have, want map[string]string) bool {
	for k, v := range want {
		if have[k] != v {
			return false
		}
	}
	return true
}

func (e *Engine) ListContainers(_ context.Context, labels map[string]string) ([]containers.ContainerSummary, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []containers.ContainerSummary
	for _, name := range slices.Sorted(maps.Keys(e.containers)) {
		c := e.containers[name]
		if matches(c.spec.Labels, labels) {
			out = append(out, containers.ContainerSummary{ID: c.id, Names: []string{"/" + name}, Labels: c.spec.Labels})
		}
	}
	return out, nil
}

func (e *Engine) RemoveContainer(_ context.Context, id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	c := e.find(id)
	if c == nil {
		return core.ErrNotFound
	}
	e.call("remove %s", c.spec.Name)
	delete(e.containers, c.spec.Name)
	return nil
}

func (e *Engine) ListVolumes(_ context.Context, labels map[string]string) ([]containers.Volume, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	var out []containers.Volume
	for _, name := range slices.Sorted(maps.Keys(e.volumes)) {
		if matches(e.volumes[name], labels) {
			out = append(out, containers.Volume{Name: name, Labels: e.volumes[name]})
		}
	}
	return out, nil
}

func (e *Engine) RemoveVolume(_ context.Context, name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if _, ok := e.volumes[name]; !ok {
		return core.ErrNotFound
	}
	e.call("remove volume %s", name)
	delete(e.volumes, name)
	return nil
}

// AddImage makes an image present.
func (e *Engine) AddImage(ref string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.images[ref] = true
}

// Spec returns what a container was created with, and whether it runs.
func (e *Engine) Spec(name string) (containers.ContainerSpec, bool, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	c, ok := e.containers[name]
	if !ok {
		return containers.ContainerSpec{}, false, false
	}
	return c.spec, c.running, true
}

// HasVolume reports whether a volume exists.
func (e *Engine) HasVolume(name string) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	_, ok := e.volumes[name]
	return ok
}

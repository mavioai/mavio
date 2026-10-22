package containers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// ContainerSpec is what a container is created with. It has no field for
// privileged mode, capabilities, devices or host namespaces, so no caller
// can create a container with them; it binds only the folders an
// administrator chose.
type ContainerSpec struct {
	// Name is the container's name.
	Name string
	// Image is the image's reference or ID.
	Image string
	// Entrypoint replaces the image's entrypoint when not empty, Cmd its
	// command when not nil; Env is KEY=value.
	Entrypoint []string
	Cmd        []string
	Env        []string
	// Labels label the container, and the volumes it creates unless
	// VolumeLabels is set.
	Labels       map[string]string
	VolumeLabels map[string]string
	// Port is the TCP port the service listens on in the container, 0
	// for none.
	Port int
	// PublishLoopback publishes Port on 127.0.0.1 at a port the engine
	// picks, which Container.PublishedPort tells.
	PublishLoopback bool
	// PeerPort is published at its own number on every interface of the
	// host, TCP and UDP; 0 for none.
	PeerPort int
	// Network is the network the container joins, with Aliases its names
	// there; empty for the engine's default.
	Network string
	Aliases []string
	// Volumes maps named volumes to the paths they are mounted at; the
	// engine creates volumes that do not exist yet, with Labels.
	Volumes map[string]string
	// Binds mount folders of the host, or subpaths of volumes.
	Binds []Bind
	// Memory is the memory limit in bytes and CPUs the CPU limit in
	// CPUs; 0 for none.
	Memory int64
	CPUs   float64
	// GPU is how the container is given every NVIDIA GPU; NoGPU for none.
	GPU GPU
	// HostGateway maps HostName to the host, as Docker's host-gateway.
	HostGateway bool
}

// Bind mounts a folder of the host, or a subpath of a volume, in a
// container.
type Bind struct {
	// Source is the host's folder; empty for a volume's.
	Source string
	// Volume is the volume's name and Subpath the folder in it, empty for
	// its root.
	Volume, Subpath string
	// Target is the path it is mounted at.
	Target string
}

// Mount is a container's mount as the engine tells it.
type Mount struct {
	// Type is bind, volume, tmpfs…; Name is a volume's name.
	Type, Name string
	// Source is where it is mounted from on the host, Destination where
	// in the container.
	Source, Destination string
}

// hostGateway maps HostName to the host.
const hostGateway = HostName + ":host-gateway"

// createBody is the body of a container creation.
type createBody struct {
	Image            string
	Entrypoint       []string            `json:",omitempty"`
	Cmd              []string            `json:",omitempty"`
	Env              []string            `json:",omitempty"`
	Labels           map[string]string   `json:",omitempty"`
	ExposedPorts     map[string]struct{} `json:",omitempty"`
	HostConfig       createHostConfig
	NetworkingConfig *networkingConfig `json:",omitempty"`
}

type createHostConfig struct {
	PortBindings   map[string][]PortBinding `json:",omitempty"`
	Mounts         []mount                  `json:",omitempty"`
	NetworkMode    string                   `json:",omitempty"`
	Memory         int64                    `json:",omitempty"`
	NanoCPUs       int64                    `json:"NanoCpus,omitempty"`
	DeviceRequests []deviceRequest          `json:",omitempty"`
	ExtraHosts     []string                 `json:",omitempty"`
	RestartPolicy  struct{ Name string }
}

type mount struct {
	Type, Source, Target string
	VolumeOptions        *volumeOptions `json:",omitempty"`
}

type volumeOptions struct {
	Labels  map[string]string `json:",omitempty"`
	Subpath string            `json:",omitempty"`
}

type deviceRequest struct {
	Driver       string     `json:",omitempty"`
	Count        int        `json:",omitempty"`
	DeviceIDs    []string   `json:",omitempty"`
	Capabilities [][]string `json:",omitempty"`
}

// GPU is how a container is given NVIDIA GPUs.
type GPU int

const (
	// NoGPU gives it none.
	NoGPU GPU = iota
	// GPURuntime asks the NVIDIA Container Toolkit's runtime for every
	// GPU, as docker run --gpus all does.
	GPURuntime
	// GPUCDI asks for the CDI device of every NVIDIA GPU, as docker run
	// --device nvidia.com/gpu=all does, on an engine with CDI enabled.
	GPUCDI
)

// cdiGPUs is the CDI device of every NVIDIA GPU.
const cdiGPUs = "nvidia.com/gpu=all"

// request is the engine's device request of the GPUs.
func (g GPU) request() []deviceRequest {
	switch g {
	case GPURuntime:
		return []deviceRequest{{Count: -1, Capabilities: [][]string{{"gpu"}}}}
	case GPUCDI:
		return []deviceRequest{{Driver: "cdi", DeviceIDs: []string{cdiGPUs}}}
	}
	return nil
}

// flags are docker run's flags of the GPUs.
func (g GPU) flags() []string {
	switch g {
	case GPURuntime:
		return []string{"--gpus", "all"}
	case GPUCDI:
		return []string{"--device", cdiGPUs}
	}
	return nil
}

// compose is a Compose file's device reservation of the GPUs.
func (g GPU) compose() string {
	switch g {
	case GPURuntime:
		return "            - driver: nvidia\n              count: all\n              capabilities: [gpu]\n"
	case GPUCDI:
		return "            - driver: cdi\n              device_ids: [" + strconv.Quote(cdiGPUs) + "]\n              capabilities: [gpu]\n"
	}
	return ""
}

type networkingConfig struct {
	EndpointsConfig map[string]endpointConfig
}

type endpointConfig struct {
	Aliases []string `json:",omitempty"`
}

// PortBinding is where a container's port is published on the host.
type PortBinding struct {
	HostIP   string `json:"HostIp"`
	HostPort string
}

// Container is what the engine tells of a container.
type Container struct {
	ID     string `json:"Id"`
	Name   string
	Image  string
	Config struct {
		Image  string
		Labels map[string]string
	}
	State struct {
		Status   string
		Running  bool
		ExitCode int
		Error    string
	}
	HostConfig struct {
		NetworkMode string
	}
	Mounts          []Mount
	NetworkSettings struct {
		Ports    map[string][]PortBinding
		Networks map[string]struct {
			NetworkID string
			IPAddress string
			Aliases   []string
		}
	}
}

// PublishedPort returns the host port the container's TCP port is
// published at on the loopback interface.
func (c Container) PublishedPort(port int) (int, bool) {
	for _, b := range c.NetworkSettings.Ports[strconv.Itoa(port)+"/tcp"] {
		if b.HostIP == "127.0.0.1" || b.HostIP == "" {
			if p, err := strconv.Atoi(b.HostPort); err == nil && p > 0 {
				return p, true
			}
		}
	}
	return 0, false
}

// ContainerSummary is a container as the engine lists it.
type ContainerSummary struct {
	ID     string `json:"Id"`
	Names  []string
	Image  string
	Labels map[string]string
	State  string
}

// CreateContainer creates a container and returns its ID; core.ErrConflict
// when its name is taken, core.ErrNotFound when its image is missing.
func (c *Client) CreateContainer(ctx context.Context, s ContainerSpec) (string, error) {
	body := createBody{Image: s.Image, Entrypoint: s.Entrypoint, Cmd: s.Cmd, Env: s.Env, Labels: s.Labels}
	body.HostConfig.RestartPolicy.Name = "no"
	expose := func(port string, b *PortBinding) {
		if body.ExposedPorts == nil {
			body.ExposedPorts = map[string]struct{}{}
		}
		body.ExposedPorts[port] = struct{}{}
		if b != nil {
			if body.HostConfig.PortBindings == nil {
				body.HostConfig.PortBindings = map[string][]PortBinding{}
			}
			body.HostConfig.PortBindings[port] = []PortBinding{*b}
		}
	}
	if s.Port > 0 {
		var b *PortBinding
		if s.PublishLoopback {
			b = &PortBinding{HostIP: "127.0.0.1"}
		}
		expose(strconv.Itoa(s.Port)+"/tcp", b)
	}
	if s.PeerPort > 0 {
		p := strconv.Itoa(s.PeerPort)
		expose(p+"/tcp", &PortBinding{HostPort: p})
		expose(p+"/udp", &PortBinding{HostPort: p})
	}
	volumeLabels := s.VolumeLabels
	if volumeLabels == nil {
		volumeLabels = s.Labels
	}
	for name, target := range s.Volumes {
		m := mount{Type: "volume", Source: name, Target: target}
		if len(volumeLabels) > 0 {
			m.VolumeOptions = &volumeOptions{Labels: volumeLabels}
		}
		body.HostConfig.Mounts = append(body.HostConfig.Mounts, m)
	}
	for _, b := range s.Binds {
		m := mount{Type: "bind", Source: b.Source, Target: b.Target}
		if b.Source == "" {
			m = mount{Type: "volume", Source: b.Volume, Target: b.Target}
			if b.Subpath != "" {
				m.VolumeOptions = &volumeOptions{Subpath: b.Subpath}
			}
		}
		body.HostConfig.Mounts = append(body.HostConfig.Mounts, m)
	}
	if s.Network != "" {
		body.HostConfig.NetworkMode = s.Network
		body.NetworkingConfig = &networkingConfig{EndpointsConfig: map[string]endpointConfig{s.Network: {Aliases: s.Aliases}}}
	}
	body.HostConfig.Memory = s.Memory
	body.HostConfig.NanoCPUs = int64(s.CPUs * 1e9)
	body.HostConfig.DeviceRequests = s.GPU.request()
	if s.HostGateway {
		body.HostConfig.ExtraHosts = []string{hostGateway}
	}
	var out struct {
		ID string `json:"Id"`
	}
	err := c.call(ctx, http.MethodPost, "/containers/create", url.Values{"name": {s.Name}}, body, &out)
	return out.ID, err
}

// StartContainer starts a container; starting a running one does nothing.
func (c *Client) StartContainer(ctx context.Context, id string) error {
	return c.call(ctx, http.MethodPost, "/containers/"+id+"/start", nil, nil, nil)
}

// StopContainer stops a container, killing it when it has not stopped
// after timeout; stopping a stopped one does nothing.
func (c *Client) StopContainer(ctx context.Context, id string, timeout time.Duration) error {
	q := url.Values{"t": {strconv.Itoa(int(timeout / time.Second))}}
	return c.call(ctx, http.MethodPost, "/containers/"+id+"/stop", q, nil, nil)
}

// InspectContainer returns a container by name or ID; core.ErrNotFound
// when there is none.
func (c *Client) InspectContainer(ctx context.Context, id string) (Container, error) {
	var ctr Container
	err := c.call(ctx, http.MethodGet, "/containers/"+id+"/json", nil, nil, &ctr)
	return ctr, err
}

// ListContainers lists every container, running or not, that carries all
// the labels.
func (c *Client) ListContainers(ctx context.Context, labels map[string]string) ([]ContainerSummary, error) {
	q := labelFilters(labels)
	if q == nil {
		q = url.Values{}
	}
	q.Set("all", "true")
	var out []ContainerSummary
	err := c.call(ctx, http.MethodGet, "/containers/json", q, nil, &out)
	return out, err
}

// RemoveContainer removes a container, stopping it first if it runs, and
// its anonymous volumes; named volumes stay.
func (c *Client) RemoveContainer(ctx context.Context, id string) error {
	return c.call(ctx, http.MethodDelete, "/containers/"+id, url.Values{"force": {"true"}, "v": {"true"}}, nil, nil)
}

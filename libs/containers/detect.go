package containers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// ErrUnavailable reports that no container engine could be reached.
var ErrUnavailable = errors.New("no container engine is available")

// probeTimeout bounds how long Detect waits for each endpoint.
const probeTimeout = 5 * time.Second

// Runtime is the container engine Detect found, and where the server runs.
type Runtime struct {
	// Client calls the engine; nil when none was found.
	Client *Client
	// Version is what the engine tells of itself; APIVersion is the
	// version requests are made in.
	Version    Version
	APIVersion string
	// Rootless reports an engine running without root.
	Rootless bool
	// GPU is how the engine gives containers NVIDIA GPUs: through a CDI
	// NVIDIA device, else the NVIDIA Container Toolkit's runtime; NoGPU
	// when it gives none.
	GPU GPU
	// InContainer reports that the server runs in a container.
	InContainer bool
	// Self is the server's own container, when it runs in one the
	// engine runs and could find by the server's host name.
	Self *Container
}

// OnHost reports whether the server reaches what it runs through ports
// published on the loopback interface: it runs on the host, or in a
// container on the host's network. A server in a container reaches them
// through a network its container joins instead.
func (r Runtime) OnHost() bool {
	return !r.InContainer || (r.Self != nil && r.Self.HostConfig.NetworkMode == "host")
}

// Detect finds the container engine the server can use: the endpoint
// named by Options.Endpoint or DOCKER_HOST, else the first of the usual
// endpoints that answers with Engine API MinAPIVersion or newer. It also
// tells whether the server runs in a container and, with an engine, which.
//
// With no engine, the Runtime still tells whether the server runs in a
// container, and the error wraps ErrUnavailable with each endpoint's
// failure; guessed endpoints with no socket are left out of it.
func Detect(ctx context.Context, o Options) (Runtime, error) {
	rt := Runtime{InContainer: inContainer(o.Root)}
	eps, named, err := Candidates(o)
	if err != nil {
		return rt, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	var errs []error
	for _, ep := range eps {
		c := NewClient(ep)
		v, info, err := probe(ctx, c)
		if err != nil {
			c.Close()
			if !named && errors.Is(err, fs.ErrNotExist) {
				continue
			}
			errs = append(errs, fmt.Errorf("%s: %w", ep, err))
			continue
		}
		rt.Client, rt.Version, rt.APIVersion = c, v, c.APIVersion()
		rt.Rootless = slices.Contains(info.SecurityOptions, "name=rootless")
		rt.GPU = info.gpu()
		if rt.InContainer {
			self, err := findSelf(ctx, c, o.Hostname)
			if err != nil {
				return rt, err
			}
			rt.Self = self
		}
		return rt, nil
	}
	if len(errs) == 0 {
		return rt, fmt.Errorf("%w: none of %v exists", ErrUnavailable, eps)
	}
	return rt, fmt.Errorf("%w: %w", ErrUnavailable, errors.Join(errs...))
}

// info is what Detect reads of an engine's /info.
type info struct {
	SecurityOptions []string
	// Runtimes are the OCI runtimes by name, "nvidia" among them once
	// the NVIDIA Container Toolkit configured the engine.
	Runtimes map[string]json.RawMessage
	// DiscoveredDevices are the CDI devices the engine found, Docker 28.2
	// and newer.
	DiscoveredDevices []struct {
		Source string
		ID     string
	}
}

// gpu tells how the engine gives containers NVIDIA GPUs: CDI where the
// engine found an NVIDIA device, which needs no runtime of its own, else
// the NVIDIA runtime.
func (i info) gpu() GPU {
	if slices.ContainsFunc(i.DiscoveredDevices, func(d struct{ Source, ID string }) bool {
		return d.Source == "cdi" && strings.HasPrefix(d.ID, "nvidia.com/gpu")
	}) {
		return GPUCDI
	}
	for name := range i.Runtimes {
		if strings.Contains(name, "nvidia") {
			return GPURuntime
		}
	}
	return NoGPU
}

// probe pings an engine, negotiates its API version and reads its info.
func probe(ctx context.Context, c *Client) (Version, info, error) {
	ctx, cancel := context.WithTimeout(ctx, probeTimeout)
	defer cancel()
	if err := c.Ping(ctx); err != nil {
		return Version{}, info{}, err
	}
	v, err := c.Negotiate(ctx)
	if err != nil {
		return v, info{}, err
	}
	var i info
	if err := c.call(ctx, "GET", "/info", nil, nil, &i); err != nil {
		return v, info{}, err
	}
	return v, i, nil
}

// inContainer reports whether the server runs in a container: Docker
// leaves /.dockerenv in its containers, Podman /run/.containerenv.
func inContainer(root string) bool {
	if root == "" {
		root = "/"
	}
	for _, name := range []string{".dockerenv", "run/.containerenv"} {
		if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(name))); err == nil {
			return true
		}
	}
	return false
}

// findSelf inspects the container named by the server's host name, which
// is its container's ID unless the container was given another; nil when
// the engine knows no such container.
func findSelf(ctx context.Context, c *Client, hostname string) (*Container, error) {
	if hostname == "" {
		var err error
		if hostname, err = os.Hostname(); err != nil {
			return nil, fmt.Errorf("host name: %w", err)
		}
	}
	self, err := c.InspectContainer(ctx, hostname)
	if errors.Is(err, core.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &self, nil
}

package containers

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// Networks of endpoints.
const (
	// Unix is a Unix domain socket, Address its path.
	Unix = "unix"
	// NamedPipe is a Windows named pipe, Address its path in slashes
	// (//./pipe/docker_engine).
	NamedPipe = "npipe"
	// TCP is a loopback TCP address, Address its host:port.
	TCP = "tcp"
)

// Endpoint is where a container engine's API listens.
type Endpoint struct {
	// Network is Unix, NamedPipe or TCP.
	Network string
	// Address is the socket's or pipe's path, or host:port.
	Address string
}

// String returns the endpoint as Docker writes it: unix:///var/run/docker.sock,
// npipe:////./pipe/docker_engine or tcp://127.0.0.1:2375.
func (e Endpoint) String() string {
	return e.Network + "://" + e.Address
}

// ParseEndpoint parses an endpoint written as Docker writes DOCKER_HOST:
// unix://, npipe:// or tcp:// followed by the address. An absolute path
// with no scheme is a Unix socket. A TCP endpoint must be on a loopback
// address, since the API carries no authentication without TLS, which is
// not supported; other endpoints are core.ErrInvalid.
func ParseEndpoint(s string) (Endpoint, error) {
	if strings.HasPrefix(s, "/") {
		return Endpoint{Network: Unix, Address: path.Clean(s)}, nil
	}
	network, address, ok := strings.Cut(s, "://")
	if !ok || address == "" {
		return Endpoint{}, fmt.Errorf("container endpoint %q: want unix://, npipe:// or tcp:// and an address: %w", s, core.ErrInvalid)
	}
	switch network {
	case Unix:
		// Windows has Unix sockets too, at paths such as C:/Users/….
		if !strings.HasPrefix(address, "/") && !filepath.IsAbs(filepath.FromSlash(address)) {
			return Endpoint{}, fmt.Errorf("container endpoint %q: want an absolute socket path: %w", s, core.ErrInvalid)
		}
		return Endpoint{Network: Unix, Address: path.Clean(address)}, nil
	case NamedPipe:
		address = strings.ReplaceAll(address, `\`, "/")
		if !strings.HasPrefix(address, "//./pipe/") || len(address) == len("//./pipe/") {
			return Endpoint{}, fmt.Errorf("container endpoint %q: want a pipe of //./pipe/: %w", s, core.ErrInvalid)
		}
		return Endpoint{Network: NamedPipe, Address: address}, nil
	case TCP:
		u, err := url.Parse("tcp://" + address)
		if err != nil || u.Port() == "" || (u.Path != "" && u.Path != "/") {
			return Endpoint{}, fmt.Errorf("container endpoint %q: want tcp://host:port: %w", s, core.ErrInvalid)
		}
		if !isLoopback(u.Hostname()) {
			return Endpoint{}, fmt.Errorf("container endpoint %q: TCP endpoints must be on a loopback address: %w", s, core.ErrInvalid)
		}
		return Endpoint{Network: TCP, Address: u.Host}, nil
	default:
		return Endpoint{}, fmt.Errorf("container endpoint %q: want unix://, npipe:// or tcp://: %w", s, core.ErrInvalid)
	}
}

// isLoopback reports whether host is localhost or a loopback IP address.
func isLoopback(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// dial connects to the endpoint.
func (e Endpoint) dial(ctx context.Context) (net.Conn, error) {
	switch e.Network {
	case Unix:
		var d net.Dialer
		return d.DialContext(ctx, "unix", e.Address)
	case NamedPipe:
		return dialPipe(ctx, e.Address)
	case TCP:
		var d net.Dialer
		return d.DialContext(ctx, "tcp", e.Address)
	default:
		return nil, fmt.Errorf("container endpoint %s: unknown network", e)
	}
}

// Options says where Detect looks for an engine. The zero value looks
// where an engine usually is.
type Options struct {
	// Endpoint is the endpoint an administrator named
	// (--container-endpoint), which is then the only one tried.
	Endpoint string
	// Getenv reads environment variables: DOCKER_HOST, XDG_RUNTIME_DIR.
	// os.Getenv when nil.
	Getenv func(string) string
	// Home is the user's home folder, os.UserHomeDir's when empty.
	Home string
	// GOOS is the operating system whose usual endpoints are tried,
	// runtime.GOOS when empty.
	GOOS string
	// Root is the folder /.dockerenv and /run/.containerenv are looked
	// for in, "/" when empty.
	Root string
	// Hostname is the server's host name, which in a container is its
	// container's ID; os.Hostname's when empty.
	Hostname string
}

func (o Options) getenv(key string) string {
	if o.Getenv == nil {
		return os.Getenv(key)
	}
	return o.Getenv(key)
}

// Candidates returns the endpoints Detect tries, in order, and whether
// they were named (by Options.Endpoint or DOCKER_HOST) rather than
// guessed: --container-endpoint, else DOCKER_HOST, else the usual
// endpoints of Docker, rootless Docker, Docker Desktop and Podman on the
// operating system.
func Candidates(o Options) (eps []Endpoint, named bool, err error) {
	for _, s := range []string{o.Endpoint, o.getenv("DOCKER_HOST")} {
		if s != "" {
			ep, err := ParseEndpoint(s)
			if err != nil {
				return nil, true, err
			}
			return []Endpoint{ep}, true, nil
		}
	}
	goos := o.GOOS
	if goos == "" {
		goos = runtime.GOOS
	}
	if goos == "windows" {
		return []Endpoint{{Network: NamedPipe, Address: "//./pipe/docker_engine"}}, false, nil
	}
	home := o.Home
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	xdg := o.getenv("XDG_RUNTIME_DIR")
	var paths []string
	paths = append(paths, "/var/run/docker.sock")
	if xdg != "" {
		paths = append(paths, path.Join(xdg, "docker.sock"))
	}
	if home != "" {
		paths = append(paths, path.Join(home, ".docker/run/docker.sock"))
	}
	if xdg != "" {
		paths = append(paths, path.Join(xdg, "podman/podman.sock"))
	}
	paths = append(paths, "/run/podman/podman.sock")
	for _, p := range paths {
		eps = append(eps, Endpoint{Network: Unix, Address: p})
	}
	return eps, false, nil
}

package core

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path/filepath"
	"time"
)

// PluginConfig is the configuration an administrator gave a plugin: a JSON
// document matching the configuration schema of the plugin's manifest. It
// is kept by plugin ID, so it survives upgrades and reinstalls.
type PluginConfig struct {
	PluginID  string
	JSON      string
	UpdatedAt time.Time
}

// Validate checks the configuration's invariants. Conformance to the
// plugin's schema is checked by whoever knows the manifest.
func (c *PluginConfig) Validate() error {
	switch {
	case c.PluginID == "":
		return fmt.Errorf("%w: plugin configuration requires a plugin ID", ErrInvalid)
	case !json.Valid([]byte(c.JSON)):
		return fmt.Errorf("%w: configuration of plugin %s is not JSON", ErrInvalid, c.PluginID)
	}
	return nil
}

// ContainerMode is how a plugin's container dependency is reached.
type ContainerMode string

const (
	// ContainerUnset is a dependency an administrator has not chosen for.
	ContainerUnset ContainerMode = "unset"
	// ContainerManaged is one the server runs on a container engine.
	ContainerManaged ContainerMode = "managed"
	// ContainerExternal is one running elsewhere, at an address an
	// administrator gave.
	ContainerExternal ContainerMode = "external"
)

// PluginContainer is what the server keeps of a container dependency a
// plugin's manifest declares, by plugin ID and name.
type PluginContainer struct {
	PluginID string
	Name     string
	Mode     ContainerMode
	// Address is the base URL of an external dependency.
	Address string
	// Secrets are the secrets generated for the dependency, by key.
	Secrets map[string]string
	// UseGPU is whether a managed dependency may use a GPU.
	UseGPU bool
	// Proxy is the ID of the proxy a managed dependency reaches sites
	// through: SystemProxyID for the system proxy, empty for none.
	Proxy string
	// Folders are the absolute paths of the server chosen for the
	// dependency's folders, by name.
	Folders map[string]string
	// PeerPort is the port a managed dependency publishes for peers; 0 for
	// none.
	PeerPort  int
	UpdatedAt time.Time
}

// Validate checks the dependency's invariants. Whether the manifest
// declares it is checked by whoever knows the manifest.
func (c *PluginContainer) Validate() error {
	switch {
	case c.PluginID == "":
		return fmt.Errorf("%w: plugin container requires a plugin ID", ErrInvalid)
	case c.Name == "":
		return fmt.Errorf("%w: container of plugin %s requires a name", ErrInvalid, c.PluginID)
	case !validProxyChoice(c.Proxy):
		return fmt.Errorf("%w: proxy %q of container %s of plugin %s", ErrInvalid, c.Proxy, c.Name, c.PluginID)
	case c.PeerPort < 0 || c.PeerPort > 65535:
		return fmt.Errorf("%w: peer port %d of container %s of plugin %s", ErrInvalid, c.PeerPort, c.Name, c.PluginID)
	}
	for name, p := range c.Folders {
		if !filepath.IsAbs(filepath.FromSlash(p)) {
			return fmt.Errorf("%w: folder %s of container %s of plugin %s is not an absolute path: %q", ErrInvalid, name, c.Name, c.PluginID, p)
		}
	}
	switch c.Mode {
	case ContainerUnset, ContainerManaged:
		if c.Address != "" {
			return fmt.Errorf("%w: %s container %s of plugin %s has an address", ErrInvalid, c.Mode, c.Name, c.PluginID)
		}
	case ContainerExternal:
		u, err := url.Parse(c.Address)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("%w: address %q of container %s of plugin %s is not an http or https URL", ErrInvalid, c.Address, c.Name, c.PluginID)
		}
	default:
		return fmt.Errorf("%w: container %s of plugin %s has unknown mode %q", ErrInvalid, c.Name, c.PluginID, c.Mode)
	}
	return nil
}

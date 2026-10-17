package server

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/apps/server/internal/proxies"
	"github.com/mavioai/mavio/libs/core"
)

// proxyUses returns what uses a proxy: what the server's own requests
// reach, the plugins by ID, and the managed containers.
func proxyUses(ctx context.Context, s core.ServerSettings, plugs *plugins.Manager, kept core.PluginContainerRepository, id string) ([]string, error) {
	var out []string
	for what, p := range s.Network.Proxies.All() {
		if *p == id {
			out = append(out, what)
		}
	}
	for plugin, p := range s.PluginProxies {
		if p == id {
			out = append(out, plugin)
		}
	}
	if plugs != nil {
		for _, p := range plugs.Plugins() {
			if len(p.Manifest.GetContainers()) == 0 {
				continue
			}
			list, err := kept.List(ctx, p.Manifest.GetId())
			if err != nil {
				return nil, err
			}
			for _, c := range list {
				if c.Mode == core.ContainerManaged && c.Proxy == id {
					out = append(out, "container "+c.Name+" of "+c.PluginID)
				}
			}
		}
	}
	slices.Sort(out)
	return out, nil
}

// checkProxy rejects a choice naming a proxy the server does not have.
func checkProxy(ctx context.Context, p *proxies.Service, what, id string) error {
	if id == "" || id == proxies.SystemID {
		return nil
	}
	if _, err := p.Resolve(ctx, id); err != nil {
		if errors.Is(err, core.ErrNotFound) {
			return fmt.Errorf("%w: the proxy of %s does not exist", core.ErrInvalid, what)
		}
		return err
	}
	return nil
}

// checkProxies rejects settings naming proxies the server does not have.
func checkProxies(ctx context.Context, p *proxies.Service, s core.ServerSettings) error {
	for what, id := range s.Network.Proxies.All() {
		if err := checkProxy(ctx, p, what, *id); err != nil {
			return err
		}
	}
	for plugin, id := range s.PluginProxies {
		if err := checkProxy(ctx, p, "plugin "+plugin, id); err != nil {
			return err
		}
	}
	return nil
}

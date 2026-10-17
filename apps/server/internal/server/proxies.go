package server

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/mavioai/mavio/apps/server/internal/proxies"
	"github.com/mavioai/mavio/libs/core"
)

// proxyUses returns what the settings use a proxy for: what the server's
// own requests reach, and the WASM plugins, by ID.
func proxyUses(s core.ServerSettings, id string) []string {
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
	slices.Sort(out)
	return out
}

// checkProxies rejects settings naming proxies the server does not have.
func checkProxies(ctx context.Context, p *proxies.Service, s core.ServerSettings) error {
	check := func(what, id string) error {
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
	for what, id := range s.Network.Proxies.All() {
		if err := check(what, *id); err != nil {
			return err
		}
	}
	for plugin, id := range s.PluginProxies {
		if err := check("plugin "+plugin, id); err != nil {
			return err
		}
	}
	return nil
}

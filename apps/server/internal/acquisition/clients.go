package acquisition

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// Client is a download client a download client provider plugin has
// configured.
type Client struct {
	PluginID string
	*pluginv1.DownloadClient
}

// Ref returns the client's name in history data, "<plugin>/<client>".
func (c Client) Ref() string { return c.PluginID + "/" + c.GetId() }

// ListClients lists the clients of every provider, by provider, then by
// priority and name, and the providers that failed to list theirs, by
// provider ID.
func (r *Runtime) ListClients(ctx context.Context) ([]Client, map[string]string) {
	providers := r.clients()
	lists := make([][]Client, len(providers))
	errs := make([]error, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Go(func() {
			resp, err := p.Client.ListDownloadClients(ctx, &pluginv1.ListDownloadClientsRequest{})
			if err != nil {
				errs[i] = err
				return
			}
			for _, c := range resp.GetClients() {
				lists[i] = append(lists[i], Client{PluginID: p.ID, DownloadClient: c})
			}
		})
	}
	wg.Wait()
	var out []Client
	failures := map[string]string{}
	for i, p := range providers {
		out = append(out, lists[i]...)
		if errs[i] != nil {
			failures[p.ID] = errs[i].Error()
		}
	}
	slices.SortStableFunc(out, func(a, b Client) int {
		return cmp.Or(cmp.Compare(a.PluginID, b.PluginID), cmp.Compare(a.GetPriority(), b.GetPriority()), cmp.Compare(a.GetName(), b.GetName()))
	})
	return out, failures
}

// clientAPI returns the provider of a plugin, or nil.
func (r *Runtime) clientAPI(pluginID string) pluginv1connect.DownloadClientProviderServiceClient {
	for _, p := range r.clients() {
		if p.ID == pluginID {
			return p.Client
		}
	}
	return nil
}

// indexerAPI returns the indexer provider of a plugin, or nil.
func (r *Runtime) indexerAPI(pluginID string) pluginv1connect.IndexerProviderServiceClient {
	for _, p := range r.indexers() {
		if p.ID == pluginID {
			return p.Client
		}
	}
	return nil
}

// chooseClient returns the client to send a release of a protocol to for
// an item of tags: the requested one, else, as Radarr's
// DownloadClientProvider.GetDownloadClient, among the enabled clients of
// the protocol, those sharing one of the tags, else those without tags;
// of those, the ones not left out after failing; then, of those of the
// first priority, the one after the client last used for the protocol,
// in turn. No client fails with errClientUnavailable.
func (r *Runtime) chooseClient(ctx context.Context, protocol core.Protocol, tags []string, pluginID, clientID string) (Client, error) {
	clients, failures := r.ListClients(ctx)
	if pluginID != "" || clientID != "" {
		for _, c := range clients {
			if c.PluginID == pluginID && c.GetId() == clientID {
				return c, nil
			}
		}
		if msg, ok := failures[pluginID]; ok {
			return Client{}, fmt.Errorf("%w: plugin %s: %s", errClientUnavailable, pluginID, msg)
		}
		return Client{}, fmt.Errorf("download client %s/%s: %w", pluginID, clientID, core.ErrNotFound)
	}
	var available []Client
	for _, c := range clients {
		if c.GetEnabled() && ProtocolFromPlugin(c.GetProtocol()) == protocol {
			available = append(available, c)
		}
	}
	if len(available) == 0 {
		return Client{}, fmt.Errorf("%w: no %s download client is available", errClientUnavailable, protocol)
	}
	tagged := slices.DeleteFunc(slices.Clone(available), func(c Client) bool {
		return !slices.ContainsFunc(c.GetTags(), func(t string) bool { return slices.Contains(tags, t) })
	})
	if len(tagged) == 0 {
		tagged = slices.DeleteFunc(available, func(c Client) bool { return len(c.GetTags()) > 0 })
	}
	if len(tagged) == 0 {
		return Client{}, fmt.Errorf("%w: no download client was found without tags or a matching tag", errClientUnavailable)
	}
	now := r.now()
	candidates := slices.DeleteFunc(tagged, func(c Client) bool {
		return c.HasDisabledUntil() && c.GetDisabledUntil().AsTime().After(now)
	})
	if len(candidates) == 0 {
		return Client{}, fmt.Errorf("%w: no %s download client is available", errClientUnavailable, protocol)
	}
	first := slices.MinFunc(candidates, func(a, b Client) int { return cmp.Compare(a.GetPriority(), b.GetPriority()) }).GetPriority()
	candidates = slices.DeleteFunc(candidates, func(c Client) bool { return c.GetPriority() != first })
	slices.SortFunc(candidates, func(a, b Client) int { return cmp.Compare(a.Ref(), b.Ref()) })

	r.mu.Lock()
	defer r.mu.Unlock()
	chosen := candidates[0]
	if i := slices.IndexFunc(candidates, func(c Client) bool { return c.Ref() > r.lastClients[protocol] }); i >= 0 {
		chosen = candidates[i]
	}
	if r.lastClients == nil {
		r.lastClients = map[core.Protocol]string{}
	}
	r.lastClients[protocol] = chosen.Ref()
	return chosen, nil
}

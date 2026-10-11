package acquisition

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
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

// clientChoice is what choosing the download client of a release takes.
type clientChoice struct {
	protocol core.Protocol
	// tags are the movie's or series'.
	tags []string
	// pluginID and clientID name the client a user asked for; empty for
	// one to choose.
	pluginID, clientID string
	// indexerClient is the download client the release's indexer sends
	// its releases to, as "<plugin>/<client>" or the client's ID alone;
	// empty for none. indexer is the indexer's name.
	indexerClient, indexer string
	// filterBlocked leaves out the clients left out after failing even
	// when they all are, for releases kept pending until a client is
	// back, as Radarr's DownloadService does for releases pending with
	// PendingReleaseReason.DownloadClientUnavailable.
	filterBlocked bool
}

// isClient reports whether a reference, "<plugin>/<client>" or a
// client's ID alone, names c.
func (c Client) isClient(ref string) bool {
	if plugin, id, ok := strings.Cut(ref, "/"); ok {
		return c.PluginID == plugin && c.GetId() == id
	}
	return c.GetId() == ref
}

// chooseClient returns the client to send a release to: the requested
// one, else, as Radarr's DownloadClientProvider.GetDownloadClient, among
// the enabled clients of the release's protocol, those sharing one of
// the tags, else those without tags; of those, the indexer's own client
// when it has one; else the ones not left out after failing, or all of
// them when they all are and the choice does not filter them; then, of
// those of the first priority, the one after the client last used for
// the protocol, in turn. No client fails with errClientUnavailable.
func (r *Runtime) chooseClient(ctx context.Context, ch clientChoice) (Client, error) {
	clients, failures := r.ListClients(ctx)
	if ch.pluginID != "" || ch.clientID != "" {
		for _, c := range clients {
			if c.PluginID == ch.pluginID && c.GetId() == ch.clientID {
				return c, nil
			}
		}
		if msg, ok := failures[ch.pluginID]; ok {
			return Client{}, fmt.Errorf("%w: plugin %s: %s", errClientUnavailable, ch.pluginID, msg)
		}
		return Client{}, fmt.Errorf("download client %s/%s: %w", ch.pluginID, ch.clientID, core.ErrNotFound)
	}
	protocol := ch.protocol
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
		return !slices.ContainsFunc(c.GetTags(), func(t string) bool { return slices.Contains(ch.tags, t) })
	})
	if len(tagged) == 0 {
		tagged = slices.DeleteFunc(available, func(c Client) bool { return len(c.GetTags()) > 0 })
	}
	if len(tagged) == 0 {
		return Client{}, fmt.Errorf("%w: no download client was found without tags or a matching tag", errClientUnavailable)
	}
	now := r.now()
	blocked := func(c Client) bool { return c.HasDisabledUntil() && c.GetDisabledUntil().AsTime().After(now) }
	if ch.indexerClient != "" {
		i := slices.IndexFunc(tagged, func(c Client) bool { return c.isClient(ch.indexerClient) })
		switch {
		case i < 0:
			return Client{}, fmt.Errorf("%w: the download client of indexer %s does not exist", errClientUnavailable, ch.indexer)
		case ch.filterBlocked && blocked(tagged[i]):
			return Client{}, fmt.Errorf("%w: the download client of indexer %s is not available due to recent failures", errClientUnavailable, ch.indexer)
		}
		return tagged[i], nil
	}
	candidates := slices.DeleteFunc(slices.Clone(tagged), blocked)
	if len(candidates) == 0 {
		if ch.filterBlocked {
			return Client{}, fmt.Errorf("%w: all %s download clients are not available", errClientUnavailable, protocol)
		}
		r.logger().DebugContext(ctx, "no download client is available that has not failed recently; trying one that has", "protocol", protocol)
		candidates = tagged
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

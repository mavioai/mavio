package releases

import (
	"cmp"
	"context"
	"slices"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/libs/release"
)

// Provider is a ready indexer provider plugin.
type Provider struct {
	ID     string
	Client pluginv1connect.IndexerProviderServiceClient
}

// Indexer is an indexer a provider has configured.
type Indexer struct {
	Provider string
	*pluginv1.Indexer
}

// Ref names an indexer of a provider.
type Ref struct{ Provider, Indexer string }

// Failure is an indexer, or with no indexer a provider, that failed.
type Failure struct {
	Provider, Indexer, Message string
	// DisabledUntil is set while the indexer is left out after failing.
	DisabledUntil time.Time
}

// Searcher searches the indexers of the indexer providers.
type Searcher struct {
	// Providers returns the ready indexer providers; nil returns none.
	Providers func() []Provider
	// Items finds the items searched for.
	Items core.ItemRepository
}

// ListIndexers lists the indexers of every provider, by provider, then by
// priority and name, and the providers that failed to list theirs.
func (s *Searcher) ListIndexers(ctx context.Context) ([]Indexer, []Failure) {
	var providers []Provider
	if s.Providers != nil {
		providers = s.Providers()
	}
	lists := make([][]Indexer, len(providers))
	failures := make([]*Failure, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Go(func() {
			resp, err := p.Client.ListIndexers(ctx, &pluginv1.ListIndexersRequest{})
			if err != nil {
				failures[i] = &Failure{Provider: p.ID, Message: err.Error()}
				return
			}
			for _, ix := range resp.GetIndexers() {
				lists[i] = append(lists[i], Indexer{Provider: p.ID, Indexer: ix})
			}
		})
	}
	wg.Wait()
	var out []Indexer
	var failed []Failure
	for i := range providers {
		out = append(out, lists[i]...)
		if failures[i] != nil {
			failed = append(failed, *failures[i])
		}
	}
	slices.SortStableFunc(out, func(a, b Indexer) int {
		return cmp.Or(cmp.Compare(a.Provider, b.Provider), cmp.Compare(a.GetPriority(), b.GetPriority()),
			cmp.Compare(a.GetName(), b.GetName()))
	})
	return out, failed
}

// Search is what to search for: an item of the library, else text, else
// the indexers' recent releases.
type Search struct {
	// ItemID is a movie, season or episode; zero for a text search.
	ItemID core.ID
	// Type, Query, Season and Episode make a text search; Season is set
	// only with a season.
	Type    pluginv1.IndexerSearchType
	Query   string
	Season  *int
	Episode string
	// Categories are Newznab categories; empty for those of the item or
	// type.
	Categories []int32
	// Indexers are those to search; empty for every enabled indexer.
	Indexers []Ref
	// Episodes, when set, searches for episodes of a series as the
	// acquisition of series asks, in place of ItemID.
	Episodes *EpisodeQuery
}

// Release is a release an indexer found, with its title parsed: Movie for
// movie releases, Episode for episode releases, neither when it cannot be
// parsed.
type Release struct {
	Provider    string
	IndexerName string
	Priority    int
	*pluginv1.Release
	Movie   *release.ParsedMovieInfo
	Episode *release.ParsedEpisodeInfo
}

// Result is what a search found.
type Result struct {
	// Releases are ordered by indexer priority, then newest first.
	Releases []Release
	// Failures are the indexers that failed, which Releases leave out.
	Failures []Failure
}

// Search searches the indexers. An unknown item fails with
// core.ErrNotFound; an item that cannot be searched for, with
// core.ErrInvalid.
func (s *Searcher) Search(ctx context.Context, q Search) (*Result, error) {
	c, err := s.criteria(ctx, q)
	if err != nil {
		return nil, err
	}
	indexers, failures := s.ListIndexers(ctx)
	clients := map[string]pluginv1connect.IndexerProviderServiceClient{}
	if s.Providers != nil {
		for _, p := range s.Providers() {
			clients[p.ID] = p.Client
		}
	}
	wanted := map[Ref]bool{}
	for _, r := range q.Indexers {
		wanted[r] = true
	}
	var targets []Indexer
	for _, ix := range indexers {
		ref := Ref{ix.Provider, ix.GetId()}
		if len(wanted) > 0 && !wanted[ref] || len(wanted) == 0 && !ix.GetEnabled() || clients[ix.Provider] == nil {
			continue
		}
		targets = append(targets, ix)
		delete(wanted, ref)
	}
	for ref := range wanted {
		failures = append(failures, Failure{Provider: ref.Provider, Indexer: ref.Indexer, Message: "unknown indexer"})
	}

	found := make([][]Release, len(targets))
	failed := make([][]Failure, len(targets))
	var wg sync.WaitGroup
	for i, ix := range targets {
		wg.Go(func() { found[i], failed[i] = search(ctx, clients[ix.Provider], ix, c) })
	}
	wg.Wait()
	res := &Result{Failures: failures}
	for i := range targets {
		res.Releases = append(res.Releases, found[i]...)
		res.Failures = append(res.Failures, failed[i]...)
	}
	slices.SortStableFunc(res.Releases, func(a, b Release) int {
		return cmp.Or(cmp.Compare(a.Priority, b.Priority),
			b.GetPublishDate().AsTime().Compare(a.GetPublishDate().AsTime()))
	})
	return res, nil
}

// search runs the tiers of requests the criteria make for an indexer,
// stopping at the first that finds releases, as Radarr's and Sonarr's
// FetchReleases do.
func search(ctx context.Context, client pluginv1connect.IndexerProviderServiceClient, ix Indexer, c *criteria) ([]Release, []Failure) {
	var failures []Failure
	for _, tier := range c.tiers(ix.GetCapabilities()) {
		seen := map[string]bool{}
		var out []Release
		for _, req := range tier {
			req.SetIndexerIds([]string{ix.GetId()})
			resp, err := client.SearchReleases(ctx, req)
			if err != nil {
				failures = append(failures, Failure{Provider: ix.Provider, Indexer: ix.GetId(), Message: err.Error()})
				return out, failures
			}
			for _, f := range resp.GetFailures() {
				failure := Failure{Provider: ix.Provider, Indexer: f.GetIndexerId(), Message: f.GetMessage()}
				if f.HasDisabledUntil() {
					failure.DisabledUntil = f.GetDisabledUntil().AsTime()
				}
				failures = append(failures, failure)
			}
			for _, r := range resp.GetReleases() {
				if seen[r.GetGuid()] {
					continue
				}
				seen[r.GetGuid()] = true
				if r.GetIndexerId() == "" {
					r.SetIndexerId(ix.GetId())
				}
				out = append(out, parse(Release{
					Provider: ix.Provider, IndexerName: ix.GetName(), Priority: int(ix.GetPriority()), Release: r,
				}, c.kind))
			}
			if len(failures) > 0 {
				return out, failures
			}
		}
		if len(out) > 0 {
			return out, failures
		}
	}
	return nil, failures
}

package requests

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// JobBlocklistedTags is the kind of the recurring job blocklisting the
// movies and series of the blocklisted tags, as Seerr's "Process
// Blocklisted Tags" job (server/job/blocklistedTagsProcessor.ts).
const JobBlocklistedTags = "requests.blocklisted_tags"

// BlocklistedTagsInterval is how often the blocklisted tags job runs:
// weekly, as Seerr's schedule of every seventh day.
const BlocklistedTagsInterval = 7 * 24 * time.Hour

// tagQueryDelay is the pause after each query of a keyword's titles,
// Seerr's TMDB_API_DELAY_MS.
const tagQueryDelay = 250 * time.Millisecond

// maxDiscoverPage is the last page DiscoverRequest takes, TMDB's.
const maxDiscoverPage = 500

// sleep waits d unless ctx ends first.
func (s *Service) sleep(ctx context.Context, d time.Duration) error {
	if s.Sleep != nil {
		return s.Sleep(ctx, d)
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// tagOrder is an order of a keyword's titles queried.
type tagOrder struct {
	sort      pluginv1.DiscoverSort
	ascending bool
}

// BlocklistTags blocklists the movies and series of the blocklisted tags
// of the request settings, as Seerr's BlocklistedTagProcessor: the titles
// blocklisted by tags are taken off the blocklist, then every keyword's
// titles are blocklisted with the tags they have, but for those
// blocklisted by hand, which are left untagged. A keyword's titles are
// read from its provider's popular catalog filtering by keyword, of
// movies and of series, a page of each of its orders, descending and
// ascending, after the other up to the settings' limit of pages; when the
// first page tells there are no more pages than queries, they are read
// in the catalog's own order instead, as Seerr's fixed sort mode. Each
// query is followed by a pause of 250ms; a query failing is logged and
// skipped. Keywords the provider does not know are skipped and dropped
// from the settings with DropBlocklistedTags, as Seerr cleans its
// settings of invalid keywords.
//
// Seerr reads the keywords' titles and changes the blocklist in one
// transaction; the titles are read first here, and the blocklist changed
// in one transaction after, so that the store is not held while the
// providers are queried.
func (s *Service) BlocklistTags(ctx context.Context) error {
	set := s.settings()
	limit := set.TagsLimit()
	type key struct {
		kind core.ItemKind
		id   int
	}
	found := map[key]*core.BlockedTitle{}
	var order []key
	add := func(kind core.ItemKind, t *pluginv1.DiscoveredTitle, tag string) {
		id, err := strconv.Atoi(t.GetExternalIds()[string(core.ProviderTMDB)])
		if err != nil || id <= 0 {
			return
		}
		k := key{kind, id}
		if b := found[k]; b != nil {
			if !slices.Contains(b.Tags, tag) {
				b.Tags = append(b.Tags, tag)
			}
			return
		}
		found[k] = &core.BlockedTitle{Kind: kind, TmdbID: id, Title: t.GetName(), Tags: []string{tag}, CreatedAt: s.now()}
		order = append(order, k)
	}
	var invalid []string
	for _, tag := range set.BlocklistedTags {
		plugin, keyword, _ := strings.Cut(tag, "/")
		i := slices.IndexFunc(s.discoveryProviders(), func(p DiscoveryProvider) bool { return p.ID == plugin })
		if i < 0 {
			s.logger().WarnContext(ctx, "skipping blocklisted tag of a discovery provider not loaded", "tag", tag)
			continue
		}
		p := s.discoveryProviders()[i]
		known, err := p.Client.ListFilterValues(ctx, pluginv1.ListFilterValuesRequest_builder{
			Filter: pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD.Enum(), Ids: []string{keyword},
		}.Build())
		switch {
		case ctx.Err() != nil:
			return ctx.Err()
		case err != nil:
			s.logger().WarnContext(ctx, "couldn't look blocklisted tag up", "tag", tag, "err", err)
			continue
		case !slices.ContainsFunc(known.GetValues(), func(v *pluginv1.FilterValue) bool { return v.GetId() == keyword }):
			s.logger().WarnContext(ctx, "skipping invalid keyword in blocklisted tags", "tag", tag)
			invalid = append(invalid, tag)
			continue
		}
		catalogs, err := p.Client.ListCatalogs(ctx, &pluginv1.ListCatalogsRequest{})
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger().WarnContext(ctx, "couldn't list catalogs for blocklisted tag", "tag", tag, "err", err)
			continue
		}
		for _, kind := range []core.ItemKind{core.KindMovie, core.KindSeries} {
			c := keywordCatalog(catalogs.GetCatalogs(), kind)
			if c == nil {
				s.logger().WarnContext(ctx, "no catalog filtering by keyword for blocklisted tag", "tag", tag, "kind", kind)
				continue
			}
			err := s.tagTitles(ctx, p, c, kind, keyword, limit, func(t *pluginv1.DiscoveredTitle) { add(kind, t, tag) })
			if err != nil {
				return err
			}
		}
	}
	err := s.Store.InTx(ctx, func(tx core.Store) error {
		removed, err := tx.BlockedTitles().DeleteTagged(ctx)
		if err != nil {
			return err
		}
		added := 0
		for _, k := range order {
			b := found[k]
			switch _, err := tx.BlockedTitles().Get(ctx, b.Kind, b.TmdbID); {
			case err == nil:
				// Blocklisted by hand: not marked with tags, as Seerr's.
				continue
			case !errors.Is(err, core.ErrNotFound):
				return err
			}
			if err := tx.BlockedTitles().Add(ctx, b); err != nil {
				return err
			}
			added++
		}
		s.logger().InfoContext(ctx, "blocklisted titles of blocklisted tags", "removed", removed, "added", added)
		return nil
	})
	if err != nil {
		return fmt.Errorf("blocklist titles of blocklisted tags: %w", err)
	}
	if len(invalid) > 0 && s.DropBlocklistedTags != nil {
		if err := s.DropBlocklistedTags(ctx, invalid); err != nil {
			return fmt.Errorf("drop invalid blocklisted tags: %w", err)
		}
		s.logger().InfoContext(ctx, "cleaned up invalid keywords from settings", "removed", invalid)
	}
	return nil
}

// keywordCatalog returns the popular catalog of a kind of titles filtering
// by keyword, Seerr's discover endpoints; nil for none.
func keywordCatalog(catalogs []*pluginv1.Catalog, kind core.ItemKind) *pluginv1.Catalog {
	mk := pluginv1.MediaKind_MEDIA_KIND_MOVIE
	if kind == core.KindSeries {
		mk = pluginv1.MediaKind_MEDIA_KIND_SERIES
	}
	for _, c := range catalogs {
		if c.GetKind() == pluginv1.CatalogKind_CATALOG_KIND_POPULAR && slices.Contains(c.GetMediaKinds(), mk) &&
			slices.Contains(c.GetFilters(), pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD) {
			return c
		}
	}
	return nil
}

// tagTitles reads the titles of a kind with a keyword from a catalog and
// hands them to add, as Seerr's processTagForType: the pages of every
// order in turn up to limit pages, or, once the first page tells there are
// no more pages than queries, every page in the catalog's own order.
func (s *Service) tagTitles(ctx context.Context, p DiscoveryProvider, c *pluginv1.Catalog, kind core.ItemKind, keyword string, limit int, add func(*pluginv1.DiscoveredTitle)) error {
	var orders []tagOrder
	for _, sort := range c.GetSorts() {
		orders = append(orders, tagOrder{sort, false}, tagOrder{sort, true})
	}
	if len(orders) == 0 {
		orders = []tagOrder{{}}
	}
	mk := pluginv1.MediaKind_MEDIA_KIND_MOVIE
	if kind == core.KindSeries {
		mk = pluginv1.MediaKind_MEDIA_KIND_SERIES
	}
	queryMax, fixed := limit*len(orders), false
	for query := 0; query < queryMax; query++ {
		page, o := query/len(orders)+1, orders[query%len(orders)]
		if fixed {
			page, o = query+1, tagOrder{}
		}
		if page > maxDiscoverPage {
			break
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		res, err := p.Client.Discover(ctx, pluginv1.DiscoverRequest_builder{
			Catalog: new(c.GetId()), MediaKind: new(mk), Keywords: []string{keyword},
			Sort: new(o.sort), Ascending: new(o.ascending), Page: new(int32(page)),
		}.Build())
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger().WarnContext(ctx, "error processing keyword in blocklisted tags", "plugin", p.ID, "keyword", keyword, "err", err)
			continue
		}
		for _, t := range res.GetTitles() {
			add(t)
		}
		if err := s.sleep(ctx, tagQueryDelay); err != nil {
			return err
		}
		if page == 1 && int(res.GetTotalPages()) <= queryMax {
			// Seerr restarts the count, so that sequential pages resume at
			// the second even when earlier queries failed.
			fixed, queryMax, query = true, int(res.GetTotalPages()), 0
		}
	}
	return nil
}

package requests

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// isBlocked reports whether a title is on the blocklist.
func (s *Service) isBlocked(ctx context.Context, kind core.ItemKind, tmdbID int) (bool, error) {
	_, err := s.Store.BlockedTitles().Get(ctx, kind, tmdbID)
	switch {
	case errors.Is(err, core.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

// Block puts a title on the blocklist, which keeps it from being
// requested, as Seerr's POST /blocklist and Blocklist.addToBlocklist
// (server/entity/Blocklist.ts): administrators only; a title on it
// already fails with core.ErrConflict. The title is named as the
// metadata providers name it, when they know it; its requests are kept.
func (s *Service) Block(ctx context.Context, caller core.User, kind core.ItemKind, tmdbID int) (core.BlockedTitle, error) {
	if !caller.Admin {
		return core.BlockedTitle{}, fmt.Errorf("%w: only administrators manage the blocklist", ErrPermission)
	}
	b := core.BlockedTitle{Kind: kind, TmdbID: tmdbID, UserID: caller.ID, CreatedAt: s.now()}
	if err := b.Validate(); err != nil {
		return core.BlockedTitle{}, err
	}
	switch t, err := s.Title(ctx, kind, tmdbID); {
	case err == nil:
		b.Title = t.Name
	case ctx.Err() != nil:
		return core.BlockedTitle{}, ctx.Err()
	default:
		s.logger().WarnContext(ctx, "couldn't name blocked title", "kind", kind, "tmdb", tmdbID, "err", err)
	}
	defer s.titles.lock(titleKey(kind, tmdbID))()
	if err := s.Store.BlockedTitles().Add(ctx, &b); err != nil {
		return core.BlockedTitle{}, err
	}
	return b, nil
}

// Unblock takes a title off the blocklist, as Seerr's DELETE
// /blocklist/:id: administrators only; a title not on it is left as it
// is.
func (s *Service) Unblock(ctx context.Context, caller core.User, kind core.ItemKind, tmdbID int) error {
	if !caller.Admin {
		return fmt.Errorf("%w: only administrators manage the blocklist", ErrPermission)
	}
	defer s.titles.lock(titleKey(kind, tmdbID))()
	if err := s.Store.BlockedTitles().Delete(ctx, kind, tmdbID); err != nil && !errors.Is(err, core.ErrNotFound) {
		return err
	}
	return nil
}

// BlockCollection blocklists the movies of a TMDB collection not on the
// blocklist yet, as Seerr's POST /blocklist/collection/:id: administrators
// only. The movies are those the collection catalog of the first
// discovery provider listing collections lists; it returns the titles
// blocklisted.
func (s *Service) BlockCollection(ctx context.Context, caller core.User, collectionID int) ([]core.BlockedTitle, error) {
	if !caller.Admin {
		return nil, fmt.Errorf("%w: only administrators manage the blocklist", ErrPermission)
	}
	parts, err := s.collection(ctx, collectionID)
	if err != nil {
		return nil, err
	}
	var out []core.BlockedTitle
	for _, p := range parts {
		b := core.BlockedTitle{Kind: core.KindMovie, TmdbID: p.id, Title: p.name, UserID: caller.ID, CreatedAt: s.now()}
		unlock := s.titles.lock(titleKey(b.Kind, b.TmdbID))
		err := s.Store.BlockedTitles().Add(ctx, &b)
		unlock()
		switch {
		case errors.Is(err, core.ErrConflict):
		case err != nil:
			return out, err
		default:
			out = append(out, b)
		}
	}
	return out, nil
}

// UnblockCollection takes the movies of a TMDB collection off the
// blocklist, as Seerr's DELETE /blocklist/collection/:id: administrators
// only.
func (s *Service) UnblockCollection(ctx context.Context, caller core.User, collectionID int) error {
	if !caller.Admin {
		return fmt.Errorf("%w: only administrators manage the blocklist", ErrPermission)
	}
	parts, err := s.collection(ctx, collectionID)
	if err != nil {
		return err
	}
	for _, p := range parts {
		unlock := s.titles.lock(titleKey(core.KindMovie, p.id))
		err := s.Store.BlockedTitles().Delete(ctx, core.KindMovie, p.id)
		unlock()
		if err != nil && !errors.Is(err, core.ErrNotFound) {
			return err
		}
	}
	return nil
}

// collectionPart is a movie of a collection.
type collectionPart struct {
	id   int
	name string
}

// collection returns the movies of a TMDB collection, once each, as the
// collection catalog of the first discovery provider listing collections
// lists them; ErrNotFound without one.
func (s *Service) collection(ctx context.Context, id int) ([]collectionPart, error) {
	if id <= 0 {
		return nil, fmt.Errorf("%w: collection ID %d", core.ErrInvalid, id)
	}
	for _, p := range s.discoveryProviders() {
		catalogs, err := p.Client.ListCatalogs(ctx, &pluginv1.ListCatalogsRequest{})
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.logger().WarnContext(ctx, "couldn't list catalogs", "plugin", p.ID, "err", err)
			continue
		}
		i := slices.IndexFunc(catalogs.GetCatalogs(), func(c *pluginv1.Catalog) bool {
			return c.GetKind() == pluginv1.CatalogKind_CATALOG_KIND_COLLECTION
		})
		if i < 0 {
			continue
		}
		res, err := p.Client.Discover(ctx, pluginv1.DiscoverRequest_builder{
			Catalog: new(catalogs.GetCatalogs()[i].GetId()), MediaKind: new(pluginv1.MediaKind_MEDIA_KIND_MOVIE),
			ExternalIds: map[string]string{string(core.ProviderTMDBCollection): strconv.Itoa(id)},
		}.Build())
		if err != nil {
			return nil, fmt.Errorf("list collection %d of %s: %w", id, p.ID, err)
		}
		var out []collectionPart
		for _, t := range res.GetTitles() {
			n, err := strconv.Atoi(t.GetExternalIds()[string(core.ProviderTMDB)])
			if err != nil || n <= 0 || t.GetMediaKind() == pluginv1.MediaKind_MEDIA_KIND_SERIES ||
				slices.ContainsFunc(out, func(c collectionPart) bool { return c.id == n }) {
				continue
			}
			out = append(out, collectionPart{id: n, name: t.GetName()})
		}
		return out, nil
	}
	return nil, fmt.Errorf("discovery provider listing collections: %w", core.ErrNotFound)
}

// ListBlocked lists the blocklist, newest first, as Seerr's GET
// /blocklist: administrators only.
func (s *Service) ListBlocked(ctx context.Context, caller core.User, q core.BlockedTitleQuery) (core.Page[core.BlockedTitle], error) {
	if !caller.Admin {
		return core.Page[core.BlockedTitle]{}, fmt.Errorf("%w: only administrators manage the blocklist", ErrPermission)
	}
	return s.Store.BlockedTitles().List(ctx, q)
}

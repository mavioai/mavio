package requests

import (
	"context"
	"errors"
	"fmt"

	"github.com/mavioai/mavio/libs/core"
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

// ListBlocked lists the blocklist, newest first, as Seerr's GET
// /blocklist: administrators only.
func (s *Service) ListBlocked(ctx context.Context, caller core.User, q core.BlockedTitleQuery) (core.Page[core.BlockedTitle], error) {
	if !caller.Admin {
		return core.Page[core.BlockedTitle]{}, fmt.Errorf("%w: only administrators manage the blocklist", ErrPermission)
	}
	return s.Store.BlockedTitles().List(ctx, q)
}

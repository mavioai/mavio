package requests

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Collection is a collection of movies as a discovery provider tells of
// it, as Seerr's collection page, with its parts marked as Discover's.
type Collection struct {
	Collection *pluginv1.Collection
	// Parts are its movies, without adult movies and those above the
	// user's rating limit.
	Parts []DiscoveredTitle
	// Hidden counts the movies left out.
	Hidden int
}

// Collection tells of a collection by its TMDB ID through the first
// discovery provider knowing collections, in a language (ISO 639-1), for
// a user. Collections no provider knows fail with core.ErrNotFound.
func (s *Service) Collection(ctx context.Context, caller core.User, tmdbID int, language string) (Collection, error) {
	req := pluginv1.GetCollectionRequest_builder{
		ExternalIds: map[string]string{string(core.ProviderTMDBCollection): strconv.Itoa(tmdbID)}, Language: &language,
	}.Build()
	for _, p := range s.discoveryProviders() {
		res, err := p.Client.GetCollection(ctx, req)
		switch {
		case connect.CodeOf(err) == connect.CodeUnimplemented:
			continue
		case err != nil:
			return Collection{}, fmt.Errorf("collection %s: %w", p.ID, err)
		case !res.GetFound():
			continue
		}
		c := res.GetCollection()
		page, err := s.page(ctx, &caller, p.ID, c.GetParts(), 1, 1, len(c.GetParts()), caller.EffectiveAccess().MaxParentalRating != nil)
		if err != nil {
			return Collection{}, err
		}
		return Collection{Collection: c, Parts: page.Titles, Hidden: page.Hidden}, nil
	}
	return Collection{}, fmt.Errorf("collection with TMDB ID %d: %w", tmdbID, core.ErrNotFound)
}

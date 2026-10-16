package requests

import (
	"context"
	"fmt"
	"strconv"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// RatingsProvider is a plugin rating titles, such as on Rotten Tomatoes
// and IMDb.
type RatingsProvider struct {
	ID     string
	Client pluginv1connect.RatingsProviderServiceClient
}

// Ratings tells a movie's or series' ratings by its TMDB ID for a user,
// as Seerr's ratings beside TMDB's score: those of every ratings
// provider, which find it by its English name, year and IMDb ID. Titles
// above the user's rating limit fail with ErrRating; a provider failing
// is left out.
func (s *Service) Ratings(ctx context.Context, caller core.User, kind core.ItemKind, tmdbID int) ([]*pluginv1.Rating, error) {
	t, err := s.titleIn(ctx, kind, tmdbID, "en")
	if err != nil {
		return nil, err
	}
	if !RatingAllowed(&caller, t.Rating, t.Country) {
		return nil, fmt.Errorf("%w: %s is rated %q", ErrRating, t.Name, t.Rating)
	}
	key := fmt.Sprintf("%s:%d", kind, tmdbID)
	if r, ok := s.ratings.get(key, s.now()); ok {
		return r, nil
	}
	mk := pluginv1.MediaKind_MEDIA_KIND_MOVIE
	if kind == core.KindSeries {
		mk = pluginv1.MediaKind_MEDIA_KIND_SERIES
	}
	ids := map[string]string{string(core.ProviderTMDB): strconv.Itoa(tmdbID)}
	if id := t.ExternalIDs[core.ProviderIMDb]; id != "" {
		ids[string(core.ProviderIMDb)] = id
	}
	req := pluginv1.GetRatingsRequest_builder{MediaKind: &mk, ExternalIds: ids, Name: &t.Name, Year: new(int32(t.Year))}.Build()
	var (
		out    []*pluginv1.Rating
		failed bool
	)
	var providers []RatingsProvider
	if s.RatingsProviders != nil {
		providers = s.RatingsProviders()
	}
	for _, p := range providers {
		res, err := p.Client.GetRatings(ctx, req)
		switch {
		case connect.CodeOf(err) == connect.CodeUnimplemented:
			continue
		case err != nil:
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.logger().WarnContext(ctx, "couldn't rate title", "provider", p.ID, "tmdb", tmdbID, "err", err)
			failed = true
			continue
		}
		out = append(out, res.GetRatings()...)
	}
	if !failed {
		s.ratings.put(key, out, s.now())
	}
	return out, nil
}

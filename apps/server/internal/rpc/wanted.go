package rpc

import (
	"context"
	"fmt"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
)

// WantedService implements mavio.acquisition.v1.WantedService.
type WantedService struct {
	rt *acquisition.Runtime
}

var _ acquisitionv1connect.WantedServiceHandler = (*WantedService)(nil)

// NewWantedService returns a WantedService acquiring movies with rt.
func NewWantedService(rt *acquisition.Runtime) *WantedService { return &WantedService{rt: rt} }

var minimumAvailabilities = map[acquisitionv1.MinimumAvailability]core.MovieAvailability{
	acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_UNSPECIFIED: core.AvailabilityReleased,
	acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_ANNOUNCED:   core.AvailabilityAnnounced,
	acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_IN_CINEMAS:  core.AvailabilityInCinemas,
	acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_RELEASED:    core.AvailabilityReleased,
}

func minimumAvailability(a acquisitionv1.MinimumAvailability) (core.MovieAvailability, error) {
	if v, ok := minimumAvailabilities[a]; ok {
		return v, nil
	}
	return "", invalid(fmt.Errorf("unknown minimum availability %v", a))
}

func minimumAvailabilityToProto(a core.MovieAvailability) acquisitionv1.MinimumAvailability {
	switch a {
	case core.AvailabilityAnnounced:
		return acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_ANNOUNCED
	case core.AvailabilityInCinemas:
		return acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_IN_CINEMAS
	}
	return acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_RELEASED
}

func wantedToProto(w core.Wanted, monitored bool) *acquisitionv1.Wanted {
	b := acquisitionv1.Wanted_builder{
		ItemId: new(w.ItemID.String()), Monitored: new(monitored), QualityProfileId: new(w.QualityProfileID.String()),
		MinimumAvailability: new(minimumAvailabilityToProto(w.MinimumAvailability)), Tags: w.Tags,
		AddTime: timestamp(w.AddedAt),
	}
	if !w.LastSearchedAt.IsZero() {
		b.LastSearchTime = timestamp(w.LastSearchedAt)
	}
	return b.Build()
}

func parseIDs(ctx context.Context, ids []string) ([]core.ID, error) {
	out := make([]core.ID, len(ids))
	for i, s := range ids {
		id, err := core.ParseID(s)
		if err != nil {
			return nil, connectError(ctx, err)
		}
		out[i] = id
	}
	return out, nil
}

// AddMovie adds a movie by its TMDB ID.
func (s *WantedService) AddMovie(ctx context.Context, req *acquisitionv1.AddMovieRequest) (*acquisitionv1.AddMovieResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	ids, err := parseIDs(ctx, []string{req.GetLibraryId(), req.GetQualityProfileId()})
	if err != nil {
		return nil, err
	}
	availability, err := minimumAvailability(req.GetMinimumAvailability())
	if err != nil {
		return nil, err
	}
	it, w, err := s.rt.AddMovie(ctx, acquisition.NewMovie{
		LibraryID: ids[0], TmdbID: int(req.GetTmdbId()), QualityProfileID: ids[1], MinimumAvailability: availability,
		Tags: req.GetTags(), Search: req.GetSearch(),
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.AddMovieResponse_builder{ItemId: new(it.ID.String()), Wanted: wantedToProto(w, it.Monitored)}.Build(), nil
}

// GetWanted returns how a movie is acquired.
func (s *WantedService) GetWanted(ctx context.Context, req *acquisitionv1.GetWantedRequest) (*acquisitionv1.GetWantedResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	id, err := core.ParseID(req.GetItemId())
	if err != nil {
		return nil, connectError(ctx, err)
	}
	w, err := s.rt.GetWanted(ctx, id)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.GetWantedResponse_builder{Wanted: wantedToProto(w.Wanted, w.Monitored)}.Build(), nil
}

// UpdateWanted changes how a movie is acquired.
func (s *WantedService) UpdateWanted(ctx context.Context, req *acquisitionv1.UpdateWantedRequest) (*acquisitionv1.UpdateWantedResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	pw := req.GetWanted()
	ids, err := parseIDs(ctx, []string{pw.GetItemId(), pw.GetQualityProfileId()})
	if err != nil {
		return nil, err
	}
	availability, err := minimumAvailability(pw.GetMinimumAvailability())
	if err != nil {
		return nil, err
	}
	w, err := s.rt.UpdateWanted(ctx, acquisition.WantedItem{
		Wanted:    core.Wanted{ItemID: ids[0], QualityProfileID: ids[1], MinimumAvailability: availability, Tags: pw.GetTags()},
		Monitored: pw.GetMonitored(),
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.UpdateWantedResponse_builder{Wanted: wantedToProto(w.Wanted, w.Monitored)}.Build(), nil
}

func wantedFilter(f acquisitionv1.WantedFilter) acquisition.WantedFilter {
	if f == acquisitionv1.WantedFilter_WANTED_FILTER_CUTOFF_UNMET {
		return acquisition.WantedCutoffUnmet
	}
	return acquisition.WantedMissing
}

// ListWanted lists the missing or cutoff unmet monitored movies.
func (s *WantedService) ListWanted(ctx context.Context, req *acquisitionv1.ListWantedRequest) (*acquisitionv1.ListWantedResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	movies, total, err := s.rt.ListWanted(ctx, wantedFilter(req.GetFilter()), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*acquisitionv1.WantedMovie, len(movies))
	for i, m := range movies {
		out[i] = acquisitionv1.WantedMovie_builder{
			ItemId: new(m.Item.ID.String()), Name: new(m.Item.Name), Year: new(int32(m.Item.ProductionYear)),
			Wanted: wantedToProto(m.Wanted, m.Item.Monitored), Quality: new(m.Quality), Available: new(m.Available),
		}.Build()
	}
	return acquisitionv1.ListWantedResponse_builder{Movies: out, Total: new(int32(total))}.Build(), nil
}

// SearchWanted queues searches for movies.
func (s *WantedService) SearchWanted(ctx context.Context, req *acquisitionv1.SearchWantedRequest) (*acquisitionv1.SearchWantedResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	ids, err := parseIDs(ctx, req.GetItemIds())
	if err != nil {
		return nil, err
	}
	n, err := s.rt.SearchWanted(ctx, ids, wantedFilter(req.GetFilter()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.SearchWantedResponse_builder{Queued: new(int32(n))}.Build(), nil
}

// SyncRSS queues an RSS sync.
func (s *WantedService) SyncRSS(ctx context.Context, _ *acquisitionv1.SyncRSSRequest) (*acquisitionv1.SyncRSSResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := s.rt.QueueRSS(ctx); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.SyncRSSResponse{}, nil
}

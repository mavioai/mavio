package rpc

import (
	"context"
	"fmt"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/libs/core"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
)

// WantedService implements mavio.acquisition.v1.WantedService.
type WantedService struct {
	rt *acquisition.Runtime
}

var _ acquisitionv1connect.WantedServiceHandler = (*WantedService)(nil)

// NewWantedService returns a WantedService acquiring movies and series
// with rt.
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

var newItemMonitors = map[acquisitionv1.NewItemMonitor]core.NewItemMonitor{
	acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_UNSPECIFIED: core.NewItemsAll,
	acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_ALL:         core.NewItemsAll,
	acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_NONE:        core.NewItemsNone,
}

func newItemMonitorToProto(m core.NewItemMonitor) acquisitionv1.NewItemMonitor {
	if m == core.NewItemsNone {
		return acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_NONE
	}
	return acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_ALL
}

var seriesMonitors = map[acquisitionv1.SeriesMonitor]acquisition.SeriesMonitor{
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_UNSPECIFIED:        acquisition.MonitorAll,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_ALL:                acquisition.MonitorAll,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_FUTURE:             acquisition.MonitorFuture,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_MISSING:            acquisition.MonitorMissing,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_EXISTING:           acquisition.MonitorExisting,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_FIRST_SEASON:       acquisition.MonitorFirstSeason,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_LAST_SEASON:        acquisition.MonitorLastSeason,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_PILOT:              acquisition.MonitorPilot,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_RECENT:             acquisition.MonitorRecent,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_MONITOR_SPECIALS:   acquisition.MonitorSpecials,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_UNMONITOR_SPECIALS: acquisition.UnmonitorSpecials,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_NONE:               acquisition.MonitorNone,
	acquisitionv1.SeriesMonitor_SERIES_MONITOR_SKIP:               acquisition.MonitorSkip,
}

func seriesMonitor(m acquisitionv1.SeriesMonitor) (acquisition.SeriesMonitor, error) {
	if v, ok := seriesMonitors[m]; ok {
		return v, nil
	}
	return "", invalid(fmt.Errorf("unknown series monitor %v", m))
}

func newItemMonitor(m acquisitionv1.NewItemMonitor) (core.NewItemMonitor, error) {
	if v, ok := newItemMonitors[m]; ok {
		return v, nil
	}
	return "", invalid(fmt.Errorf("unknown new item monitor %v", m))
}

func wantedToProto(w core.Wanted, monitored bool) *acquisitionv1.Wanted {
	b := acquisitionv1.Wanted_builder{
		ItemId: new(w.ItemID.String()), Monitored: new(monitored), QualityProfileId: new(w.QualityProfileID.String()),
		MinimumAvailability: new(minimumAvailabilityToProto(w.MinimumAvailability)), Tags: w.Tags,
		AddTime: timestamp(w.AddedAt), SeasonFolder: new(w.SeasonFolder),
	}
	if w.MonitorNewItems != "" {
		b.MonitorNewItems = new(newItemMonitorToProto(w.MonitorNewItems))
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

// AddSeries adds a series by its TVDB ID.
func (s *WantedService) AddSeries(ctx context.Context, req *acquisitionv1.AddSeriesRequest) (*acquisitionv1.AddSeriesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	ids, err := parseIDs(ctx, []string{req.GetLibraryId(), req.GetQualityProfileId()})
	if err != nil {
		return nil, err
	}
	monitor, err := seriesMonitor(req.GetMonitor())
	if err != nil {
		return nil, err
	}
	newItems, err := newItemMonitor(req.GetMonitorNewItems())
	if err != nil {
		return nil, err
	}
	n := acquisition.NewSeries{
		LibraryID: ids[0], TvdbID: int(req.GetTvdbId()), QualityProfileID: ids[1], SeasonFolder: req.GetSeasonFolder(),
		Monitor: monitor, MonitorNewItems: newItems, Tags: req.GetTags(), Search: req.GetSearch(),
	}
	if t := req.GetSeriesType(); t != libraryv1.SeriesType_SERIES_TYPE_UNSPECIFIED {
		st, ok := seriesTypesFromProto[t]
		if !ok {
			return nil, invalid(fmt.Errorf("unknown series type %v", t))
		}
		n.SeriesType = &st
	}
	it, w, err := s.rt.AddSeries(ctx, n)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return acquisitionv1.AddSeriesResponse_builder{ItemId: new(it.ID.String()), Wanted: wantedToProto(w, it.Monitored)}.Build(), nil
}

// SetMonitored monitors or unmonitors seasons and episodes, or applies a
// monitoring option to wanted series.
func (s *WantedService) SetMonitored(ctx context.Context, req *acquisitionv1.SetMonitoredRequest) (*acquisitionv1.SetMonitoredResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if req.HasItems() {
		items := req.GetItems()
		ids, err := parseIDs(ctx, items.GetItemIds())
		if err != nil {
			return nil, err
		}
		if err := s.rt.SetMonitored(ctx, ids, items.GetMonitored()); err != nil {
			return nil, connectError(ctx, err)
		}
		return &acquisitionv1.SetMonitoredResponse{}, nil
	}
	series := req.GetSeries()
	ids, err := parseIDs(ctx, series.GetItemIds())
	if err != nil {
		return nil, err
	}
	monitor, err := seriesMonitor(series.GetMonitor())
	if err != nil {
		return nil, err
	}
	var newItems core.NewItemMonitor
	if m := series.GetMonitorNewItems(); m != acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_UNSPECIFIED {
		if newItems, err = newItemMonitor(m); err != nil {
			return nil, err
		}
	}
	if err := s.rt.MonitorSeries(ctx, ids, monitor, newItems); err != nil {
		return nil, connectError(ctx, err)
	}
	return &acquisitionv1.SetMonitoredResponse{}, nil
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
	newItems, err := newItemMonitor(pw.GetMonitorNewItems())
	if err != nil {
		return nil, err
	}
	w, err := s.rt.UpdateWanted(ctx, acquisition.WantedItem{
		Wanted: core.Wanted{
			ItemID: ids[0], QualityProfileID: ids[1], MinimumAvailability: availability, Tags: pw.GetTags(),
			MonitorNewItems: newItems, SeasonFolder: pw.GetSeasonFolder(),
		},
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

// ListWanted lists the missing or cutoff unmet monitored movies or
// episodes.
func (s *WantedService) ListWanted(ctx context.Context, req *acquisitionv1.ListWantedRequest) (*acquisitionv1.ListWantedResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if req.GetKind() == acquisitionv1.WantedKind_WANTED_KIND_EPISODE {
		episodes, total, err := s.rt.ListWantedEpisodes(ctx, wantedFilter(req.GetFilter()), int(req.GetLimit()), int(req.GetOffset()))
		if err != nil {
			return nil, connectError(ctx, err)
		}
		out := make([]*acquisitionv1.WantedEpisode, len(episodes))
		for i, e := range episodes {
			b := acquisitionv1.WantedEpisode_builder{
				ItemId: new(e.ID.String()), SeriesId: new(e.Series.ID.String()), SeriesName: new(e.Series.Name),
				SeasonNumber: new(int32(e.Season)), EpisodeNumber: new(int32(e.Episode)), AbsoluteNumber: new(int32(e.Absolute)),
				Name: new(e.Name), Quality: new(e.Quality), Wanted: wantedToProto(e.Wanted, e.Series.Monitored),
			}
			if !e.AirTime.IsZero() {
				b.AirTime = timestamp(e.AirTime)
			}
			out[i] = b.Build()
		}
		return acquisitionv1.ListWantedResponse_builder{Episodes: out, Total: new(int32(total))}.Build(), nil
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

// SearchWanted queues searches for movies, series, seasons or episodes.
func (s *WantedService) SearchWanted(ctx context.Context, req *acquisitionv1.SearchWantedRequest) (*acquisitionv1.SearchWantedResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	ids, err := parseIDs(ctx, req.GetItemIds())
	if err != nil {
		return nil, err
	}
	kind := acquisition.WantedKindMovie
	if req.GetKind() == acquisitionv1.WantedKind_WANTED_KIND_EPISODE {
		kind = acquisition.WantedKindEpisode
	}
	n, err := s.rt.SearchWanted(ctx, kind, ids, wantedFilter(req.GetFilter()))
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

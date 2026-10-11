package requests

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/libs/core"
)

// errNoAcquisition is a server without an acquisition runtime.
var errNoAcquisition = errors.New("acquisition is not running")

// acquire makes an approved request's movie or seasons wanted, as Seerr's
// MediaRequestSubscriber sends approved requests to Radarr and Sonarr,
// and sets the request's item.
func (s *Service) acquire(ctx context.Context, r *core.Request) error {
	if s.Acquisition == nil {
		return errNoAcquisition
	}
	if r.Kind == core.KindMovie {
		return s.acquireMovie(ctx, r)
	}
	return s.acquireSeries(ctx, r)
}

// acquireMovie adds the movie to the requests' movie library, searching
// for it, unless a library has it: a movie with its file or wanted is
// left as it is, another one is wanted and searched for.
func (s *Service) acquireMovie(ctx context.Context, r *core.Request) error {
	set := s.settings()
	m, err := s.findMedia(ctx, core.KindMovie, r.TmdbID, 0)
	if err != nil {
		return err
	}
	status, itemID, err := s.movieStatus(ctx, m)
	if err != nil {
		return err
	}
	switch {
	case status == StatusAvailable || status == StatusWanted:
		r.ItemID = itemID
		if status == StatusWanted {
			_, err = s.Acquisition.QueueSearch(ctx, []core.ID{itemID}, false)
		}
		return err
	case set.MovieQualityProfileID.IsZero():
		return errors.New("no quality profile is set for movie requests")
	case !itemID.IsZero():
		it, err := s.Store.Items().Get(ctx, itemID)
		if err != nil {
			return err
		}
		if err := s.want(ctx, &it, core.Wanted{QualityProfileID: set.MovieQualityProfileID, MinimumAvailability: set.MinimumAvailability}); err != nil {
			return err
		}
		r.ItemID = it.ID
		_, err = s.Acquisition.QueueSearch(ctx, []core.ID{it.ID}, false)
		return err
	case set.MovieLibraryID.IsZero():
		return errors.New("no library is set for movie requests")
	}
	it, _, err := s.Acquisition.AddMovie(ctx, acquisition.NewMovie{
		LibraryID: set.MovieLibraryID, TmdbID: r.TmdbID, QualityProfileID: set.MovieQualityProfileID,
		MinimumAvailability: set.MinimumAvailability, Search: true,
	})
	if err != nil {
		return fmt.Errorf("add movie: %w", err)
	}
	r.ItemID = it.ID
	return nil
}

// want makes an item of a library wanted, monitored.
func (s *Service) want(ctx context.Context, it *core.Item, w core.Wanted) error {
	w.ItemID, w.AddedAt = it.ID, s.now()
	if w.MinimumAvailability == "" && it.Kind == core.KindMovie {
		w.MinimumAvailability = core.AvailabilityReleased
	}
	if err := s.Store.Wanted().Put(ctx, &w); err != nil {
		return err
	}
	if !it.Monitored {
		it.Monitored = true
		return s.Store.Items().Upsert(ctx, *it)
	}
	return nil
}

// acquireSeries adds the series to the requests' show library, unless a
// library has it, which is then wanted; the requested seasons are
// monitored and searched for, as Seerr asks Sonarr to.
func (s *Service) acquireSeries(ctx context.Context, r *core.Request) error {
	set := s.settings()
	anime := false
	switch t, err := s.Title(ctx, core.KindSeries, r.TmdbID); {
	case err == nil:
		anime = t.Anime()
		if r.TvdbID == 0 {
			r.TvdbID = t.TvdbID
		}
	case ctx.Err() != nil:
		return ctx.Err()
	case r.TvdbID == 0:
		return err
	default:
		s.logger().WarnContext(ctx, "couldn't look series up; taken as not anime", "tmdb", r.TmdbID, "err", err)
	}
	// Anime series go to the anime library and profile when set, as
	// Seerr's MediaRequestSubscriber picks activeAnimeDirectory and
	// activeAnimeProfileId, and are numbered as anime.
	libraryID, profileID := set.SeriesLibraryID, set.SeriesQualityProfileID
	var seriesType *core.SeriesType
	if anime {
		libraryID = cmp.Or(set.AnimeLibraryID, libraryID)
		profileID = cmp.Or(set.AnimeQualityProfileID, profileID)
		seriesType = new(core.SeriesAnime)
	}
	if r.TvdbID == 0 {
		return errors.New("no metadata provider knows the series' TVDB ID")
	}
	m, err := s.findMedia(ctx, core.KindSeries, r.TmdbID, r.TvdbID)
	if err != nil {
		return err
	}
	st, err := s.seriesState(ctx, m)
	if err != nil {
		return err
	}
	if profileID.IsZero() && !st.wanted {
		return errors.New("no quality profile is set for series requests")
	}
	var series core.Item
	switch {
	case !st.itemID.IsZero():
		if series, err = s.Store.Items().Get(ctx, st.itemID); err != nil {
			return err
		}
		if !st.wanted {
			w := core.Wanted{QualityProfileID: profileID, MonitorNewItems: set.MonitorNewItems, SeasonFolder: set.SeasonFolder}
			if err := s.want(ctx, &series, w); err != nil {
				return err
			}
		}
	case libraryID.IsZero():
		return errors.New("no library is set for series requests")
	default:
		series, _, err = s.Acquisition.AddSeries(ctx, acquisition.NewSeries{
			LibraryID: libraryID, TvdbID: r.TvdbID, QualityProfileID: profileID, SeriesType: seriesType,
			SeasonFolder: set.SeasonFolder, Monitor: acquisition.MonitorNone, MonitorNewItems: set.MonitorNewItems,
		})
		if err != nil {
			return fmt.Errorf("add series: %w", err)
		}
	}
	r.ItemID = series.ID
	numbers := r.SeasonNumbers()
	var seasons []core.ID
	var found []int
	for it, err := range s.Store.Items().Walk(ctx, core.ItemQuery{
		ParentID: series.ID, Kinds: []core.ItemKind{core.KindSeason}, IncludeVirtual: true, IncludeMissing: true,
	}) {
		if err != nil {
			return err
		}
		if it.IndexNumber != nil && slices.Contains(numbers, *it.IndexNumber) {
			seasons = append(seasons, it.ID)
			found = append(found, *it.IndexNumber)
		}
	}
	if len(seasons) == 0 {
		return fmt.Errorf("%s has none of seasons %v", series.Name, numbers)
	}
	if len(found) < len(numbers) {
		s.logger().WarnContext(ctx, "requested seasons not found", "series", series.Name, "requested", numbers, "found", found)
	}
	if err := s.Acquisition.SetMonitored(ctx, seasons, true); err != nil {
		return err
	}
	slices.Sort(found)
	for _, n := range found {
		if _, err := s.Acquisition.QueueEpisodeSearch(ctx, acquisition.EpisodeSearchPayload{
			Scope: acquisition.ScopeSeason, SeriesID: series.ID, SeasonNumber: n,
		}); err != nil {
			return err
		}
	}
	return nil
}

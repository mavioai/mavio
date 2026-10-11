package requests

import (
	"context"
	"errors"
	"strconv"

	"github.com/mavioai/mavio/libs/core"
)

// MediaStatus is what the server has of a title, as Seerr's media
// statuses.
type MediaStatus int

// The media statuses, from none to available.
const (
	// StatusNone is nothing.
	StatusNone MediaStatus = iota
	// StatusRequested is a request waiting for approval.
	StatusRequested
	// StatusWanted is monitored by acquisition, without files yet.
	StatusWanted
	// StatusPartial is a series some of whose wanted episodes have files.
	StatusPartial
	// StatusAvailable is a movie with a file, or a series whose wanted
	// aired episodes all have files.
	StatusAvailable
)

// hasFile reports whether an item has its file, as the acquisition
// runtime tells it.
func hasFile(it *core.Item) bool { return !it.Virtual && it.Path != "" && it.MissingSince == nil }

// media is what the libraries hold of a title: its movie or series items,
// in every library.
type media struct {
	items []core.Item
}

// findMedia returns the movies or series of the libraries with a TMDB ID,
// or with a TVDB ID for series.
func (s *Service) findMedia(ctx context.Context, kind core.ItemKind, tmdbID, tvdbID int) (media, error) {
	idx, err := s.index(ctx, kind)
	if err != nil {
		return media{}, err
	}
	return idx.find(tmdbID, tvdbID), nil
}

// mediaIndex holds the movies or series of the libraries by their IDs.
type mediaIndex struct {
	byTmdb, byTvdb map[int][]core.Item
}

func (x mediaIndex) find(tmdbID, tvdbID int) media {
	var m media
	add := func(items []core.Item) {
		for _, it := range items {
			if !containsID(m.items, it.ID) {
				m.items = append(m.items, it)
			}
		}
	}
	if tmdbID > 0 {
		add(x.byTmdb[tmdbID])
	}
	if tvdbID > 0 {
		add(x.byTvdb[tvdbID])
	}
	return m
}

func containsID(items []core.Item, id core.ID) bool {
	for i := range items {
		if items[i].ID == id {
			return true
		}
	}
	return false
}

// index reads the movies or series of every library.
func (s *Service) index(ctx context.Context, kind core.ItemKind) (mediaIndex, error) {
	x := mediaIndex{byTmdb: map[int][]core.Item{}, byTvdb: map[int][]core.Item{}}
	for it, err := range s.Store.Items().Walk(ctx, core.ItemQuery{Kinds: []core.ItemKind{kind}, IncludeVirtual: true, IncludeMissing: true}) {
		if err != nil {
			return mediaIndex{}, err
		}
		if n, _ := strconv.Atoi(it.ExternalIDs[core.ProviderTMDB]); n > 0 {
			x.byTmdb[n] = append(x.byTmdb[n], it)
		}
		if n, _ := strconv.Atoi(it.ExternalIDs[core.ProviderTVDB]); n > 0 {
			x.byTvdb[n] = append(x.byTvdb[n], it)
		}
	}
	return x, nil
}

// wanted reports whether acquisition monitors an item.
func (s *Service) wanted(ctx context.Context, it *core.Item) (bool, error) {
	if !it.Monitored {
		return false, nil
	}
	_, err := s.Store.Wanted().Get(ctx, it.ID)
	switch {
	case errors.Is(err, core.ErrNotFound):
		return false, nil
	case err != nil:
		return false, err
	}
	return true, nil
}

// movieStatus is the status of a movie's items: available when one has
// its file, wanted when acquisition monitors one.
func (s *Service) movieStatus(ctx context.Context, m media) (MediaStatus, core.ID, error) {
	status, id := StatusNone, core.ID{}
	for i := range m.items {
		it := &m.items[i]
		if hasFile(it) {
			return StatusAvailable, it.ID, nil
		}
		w, err := s.wanted(ctx, it)
		if err != nil {
			return StatusNone, core.ID{}, err
		}
		if w {
			status, id = StatusWanted, it.ID
		} else if id.IsZero() {
			id = it.ID
		}
	}
	return status, id, nil
}

// seasonState is what a series has of a season.
type seasonState struct {
	// aired and files count the season's aired episodes and those with
	// files; episodes all its episodes.
	aired, files, episodes int
	// wanted is a monitored season of a wanted series.
	wanted bool
}

// available reports whether a season's aired episodes all have files.
func (st seasonState) available() bool { return st.aired > 0 && st.files >= st.aired }

// taken reports whether a season cannot be requested, as Seerr's season
// statuses other than unknown: it is wanted or has files.
func (st seasonState) taken() bool { return st.wanted || st.files > 0 }

// seriesState is what the libraries have of a series.
type seriesState struct {
	itemID  core.ID
	wanted  bool
	seasons map[int]*seasonState
}

// seriesStates reads the seasons of a series' items, merged across
// libraries; the item is the wanted one, else the first.
func (s *Service) seriesState(ctx context.Context, m media) (seriesState, error) {
	st := seriesState{seasons: map[int]*seasonState{}}
	now := s.now()
	for i := range m.items {
		series := &m.items[i]
		wanted, err := s.wanted(ctx, series)
		if err != nil {
			return seriesState{}, err
		}
		if st.itemID.IsZero() || wanted && !st.wanted {
			st.itemID = series.ID
		}
		st.wanted = st.wanted || wanted
		season := func(n int) *seasonState {
			if st.seasons[n] == nil {
				st.seasons[n] = &seasonState{}
			}
			return st.seasons[n]
		}
		for it, err := range s.Store.Items().Walk(ctx, core.ItemQuery{
			ParentID: series.ID, Recursive: true, Kinds: []core.ItemKind{core.KindSeason, core.KindEpisode},
			IncludeVirtual: true, IncludeMissing: true,
		}) {
			if err != nil {
				return seriesState{}, err
			}
			switch {
			case it.Kind == core.KindSeason && it.IndexNumber != nil:
				if wanted && it.Monitored {
					season(*it.IndexNumber).wanted = true
				}
			case it.Kind == core.KindEpisode && it.ParentIndexNumber != nil:
				ss := season(*it.ParentIndexNumber)
				ss.episodes++
				file := hasFile(&it)
				if file || it.PremiereDate != nil && it.PremiereDate.Before(now) {
					ss.aired++
				}
				if file {
					ss.files++
				}
			}
		}
	}
	return st, nil
}

// status is a series' status: available when the aired episodes of its
// wanted seasons, or of all its seasons when none is wanted, all have
// files; partially available when some episodes have files; wanted when
// acquisition monitors it.
func (st seriesState) status() MediaStatus {
	files, complete, counted := 0, true, false
	for n, ss := range st.seasons {
		if n == 0 {
			continue
		}
		files += ss.files
		if st.wanted && !ss.wanted && ss.files == 0 {
			continue
		}
		if ss.aired > 0 {
			counted = true
			if !ss.available() {
				complete = false
			}
		}
	}
	switch {
	case files > 0 && counted && complete:
		return StatusAvailable
	case files > 0:
		return StatusPartial
	case st.wanted:
		return StatusWanted
	}
	return StatusNone
}

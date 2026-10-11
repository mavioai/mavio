package acquisition

import (
	"context"
	"errors"
	"fmt"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/organize"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// movie is a wanted movie with what decisions take of it.
type movie struct {
	item   core.Item
	wanted core.Wanted
	lib    core.Library
	target *acquire.Target
	acq    *acquire.Item
	// file is the path of the movie's file, "" without one.
	file string
}

// movie returns a wanted movie, read once per run. A movie that is not
// wanted fails with core.ErrNotFound.
func (rn *run) movie(ctx context.Context, itemID core.ID) (*movie, error) {
	if m, ok := rn.movies[itemID]; ok {
		return m, nil
	}
	st := rn.rt.Store
	w, err := st.Wanted().Get(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("wanted movie %s: %w", itemID, err)
	}
	it, err := st.Items().Get(ctx, itemID)
	if err != nil {
		return nil, fmt.Errorf("wanted movie %s: %w", itemID, err)
	}
	if it.Kind != core.KindMovie {
		return nil, fmt.Errorf("%w: item %s is not a movie", core.ErrInvalid, itemID)
	}
	lib, err := rn.library(ctx, it.LibraryID)
	if err != nil {
		return nil, err
	}
	p, err := rn.profile(ctx, w.QualityProfileID)
	if err != nil {
		return nil, err
	}
	m := &movie{item: it, wanted: w, lib: lib}
	m.target = &acquire.Target{
		ID: it.ID, Kind: release.Movie, Title: it.Name, Year: it.ProductionYear,
		OriginalLanguage: release.LanguageUnknown, Tags: w.Tags, Profile: p, Monitored: it.Monitored,
	}
	m.acq = &acquire.Item{ID: it.ID, Monitored: it.Monitored, Runtime: int(it.Runtime.Minutes()), Availability: availability(it, w)}
	if !it.Virtual && it.Path != "" {
		m.file = it.Path
		f, err := rn.currentFile(ctx, m)
		if err != nil {
			return nil, err
		}
		m.acq.File = f
	}
	m.target.Path = rn.movieFolder(m)
	rn.movies[itemID] = m
	return m, nil
}

// library returns a library, read once per run.
func (rn *run) library(ctx context.Context, id core.ID) (core.Library, error) {
	if lib, ok := rn.libs[id]; ok {
		return lib, nil
	}
	lib, err := rn.rt.Store.Libraries().Get(ctx, id)
	if err != nil {
		return core.Library{}, err
	}
	rn.libs[id] = lib
	return lib, nil
}

// profile returns a movie quality profile, read once per run. Without
// the movie's original language stored, a profile wanting the original
// language takes any.
func (rn *run) profile(ctx context.Context, id core.ID) (*acquire.QualityProfile, error) {
	if p, ok := rn.profiles[id]; ok {
		return p, nil
	}
	stored, err := rn.rt.Store.QualityProfiles().Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("quality profile %s: %w", id, err)
	}
	p, err := acquire.QualityProfileFromCore(stored)
	if err != nil {
		return nil, err
	}
	if p.Language == release.Original {
		p.Language = release.LanguageUnknown
	}
	rn.profiles[id] = &p
	return &p, nil
}

// availability returns when a movie counts as available. Only the
// premiere date is stored, as the date in cinemas.
func availability(it core.Item, w core.Wanted) *acquire.Availability {
	a := &acquire.Availability{Minimum: minimumStatus(w.MinimumAvailability)}
	if it.PremiereDate != nil {
		a.InCinemas = *it.PremiereDate
	}
	return a
}

func minimumStatus(a core.MovieAvailability) acquire.MovieStatus {
	switch a {
	case core.AvailabilityAnnounced:
		return acquire.StatusAnnounced
	case core.AvailabilityInCinemas:
		return acquire.StatusInCinemas
	}
	return acquire.StatusReleased
}

// currentFile returns the file a movie has: its quality and scene name
// those of the import that brought it, else parsed from its path.
func (rn *run) currentFile(ctx context.Context, m *movie) (*acquire.File, error) {
	st := rn.rt.Store
	f := &acquire.File{ID: m.item.ID, DateAdded: m.item.DateAdded}
	sources, err := st.MediaSources().ListForItem(ctx, m.item.ID)
	if err != nil {
		return nil, err
	}
	if len(sources) > 0 {
		f.ID, f.Size = sources[0].ID, sources[0].Size
	}
	title := strings.TrimSuffix(path.Base(m.file), path.Ext(m.file))
	parsed := release.ParseMoviePath(m.file)
	if parsed != nil {
		f.Quality, f.ReleaseGroup, f.Languages = acquire.ModelOf(parsed.Quality), parsed.ReleaseGroup, parsed.Languages
	} else {
		f.Quality = acquire.ModelOf(release.ParseQuality(path.Base(m.file), release.Movie))
	}
	imported, err := st.History().List(ctx, core.HistoryQuery{ItemIDs: []core.ID{m.item.ID}, Types: []core.HistoryEventType{core.HistoryImported}, Limit: 20})
	if err != nil {
		return nil, err
	}
	for _, h := range imported.Items {
		if h.Data[dataImportedPath] != m.file {
			continue
		}
		f.Quality, f.Languages, f.DateAdded = qualityModel(h.Quality), languagesOf(h.Languages), h.Date
		if h.SourceTitle != "" {
			title = h.SourceTitle
		}
		if g := h.Data[dataReleaseGroup]; g != "" {
			f.ReleaseGroup = g
		}
		break
	}
	f.Formats = rn.formats(title, f.Quality, f.Languages, f.Size, nil)
	return f, nil
}

// organizeMovie returns what names a movie's folder and files.
func organizeMovie(it core.Item) organize.Movie {
	m := organize.Movie{
		Title: it.Name, OriginalTitle: it.OriginalTitle, Year: it.ProductionYear, CollectionTitle: it.CollectionName,
		Certification: it.OfficialRating, ImdbID: it.ExternalIDs[core.ProviderIMDb],
	}
	m.TmdbID, _ = strconv.Atoi(it.ExternalIDs[core.ProviderTMDB])
	return m
}

// naming returns the movie naming settings.
func (rn *run) naming() organize.Naming {
	n := rn.settings.Naming
	return organize.Naming{
		RenameMovies: n.RenameMovies, ReplaceIllegalCharacters: n.ReplaceIllegalCharacters,
		ColonReplacement:  organize.ColonReplacement(n.ColonReplacement),
		MovieFolderFormat: n.MovieFolderFormat, StandardMovieFormat: n.StandardMovieFormat,
	}
}

// movieFolder returns the folder of a movie: that of its file, else one
// named by the naming settings in the library's first folder.
func (rn *run) movieFolder(m *movie) string {
	if m.file != "" {
		return path.Dir(m.file)
	}
	if len(m.lib.Paths) == 0 {
		return ""
	}
	return path.Join(filepath.ToSlash(filepath.Clean(m.lib.Paths[0])), organize.MovieFolder(rn.naming(), organizeMovie(m.item)))
}

// movieRefs returns every wanted movie, for matching releases.
func (rn *run) movieRefs(ctx context.Context) ([]acquire.MovieRef, error) {
	if rn.refsRead {
		return rn.refs, nil
	}
	wanted, err := rn.rt.Store.Wanted().List(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range wanted {
		it, err := rn.rt.Store.Items().Get(ctx, w.ItemID)
		if errors.Is(err, core.ErrNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		if it.Kind == core.KindMovie {
			rn.refs = append(rn.refs, movieRef(it))
		}
	}
	rn.refsRead = true
	return rn.refs, nil
}

func movieRef(it core.Item) acquire.MovieRef {
	ref := acquire.MovieRef{ID: it.ID, Title: it.Name, OriginalTitle: it.OriginalTitle, Year: it.ProductionYear, ImdbID: it.ExternalIDs[core.ProviderIMDb]}
	ref.TmdbID, _ = strconv.Atoi(it.ExternalIDs[core.ProviderTMDB])
	return ref
}

// findMovie returns the FindMovie of decisions: a pending release's own
// movie, else the movie the release matches among the wanted ones, as
// Radarr's ParsingService.Map.
func (rn *run) findMovie(b *batch) acquire.FindMovie {
	return func(ctx context.Context, parsed *release.ParsedMovieInfo, r *acquire.Release, s *acquire.Search) (*acquire.Target, *acquire.Item, error) {
		if id, ok := b.targets[r]; ok {
			return rn.targetOf(ctx, id)
		}
		refs, err := rn.movieRefs(ctx)
		if err != nil {
			return nil, nil, err
		}
		var searched *acquire.MovieRef
		if s != nil {
			m, err := rn.movie(ctx, s.TargetID)
			if err != nil {
				return nil, nil, err
			}
			ref := movieRef(m.item)
			searched = &ref
		}
		ref, err := acquire.MatchMovie(parsed, r.ImdbID, r.TmdbID, searched, refs)
		if err != nil || ref == nil {
			return nil, nil, err
		}
		return rn.targetOf(ctx, ref.ID)
	}
}

func (rn *run) targetOf(ctx context.Context, id core.ID) (*acquire.Target, *acquire.Item, error) {
	m, err := rn.movie(ctx, id)
	if errors.Is(err, core.ErrNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	return m.target, m.acq, nil
}

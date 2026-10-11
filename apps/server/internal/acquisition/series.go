package acquisition

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/organize"
	"github.com/mavioai/mavio/libs/acquire/series"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/release"
)

// show is a wanted series with what episode decisions take of it.
type show struct {
	item   core.Item
	wanted core.Wanted
	lib    core.Library
	series *series.Series
	// episodes are the series' episodes by season and number, with those
	// multi-episode files cover; byID indexes them.
	episodes []*series.Episode
	byID     map[core.ID]*series.Episode
	// items are the episode items by the IDs of the episodes they hold:
	// a multi-episode file's item holds several.
	items map[core.ID]core.Item
	// seasons are the season items by number.
	seasons map[int]core.Item
}

// coveredID returns the ID of an episode a multi-episode file holds
// beyond its first, which has no item of its own: derived from the
// holding item's ID and the episode's number, so that it stays the same.
func coveredID(holder core.ID, episode int) core.ID {
	sum := sha256.Sum256(append(holder[:], byte(episode>>24), byte(episode>>16), byte(episode>>8), byte(episode)))
	var id core.ID
	copy(id[:], sum[:16])
	id[6] = id[6]&0x0f | 0x80 // version 8
	id[8] = id[8]&0x3f | 0x80
	return id
}

// hasFile reports whether an episode item has its file.
func hasFile(it core.Item) bool { return !it.Virtual && it.Path != "" && it.MissingSince == nil }

// show returns a wanted series, read once per run. A series that is not
// wanted fails with core.ErrNotFound; an item that is not a series with
// core.ErrInvalid.
func (rn *run) show(ctx context.Context, seriesID core.ID) (*show, error) {
	if sh, ok := rn.shows[seriesID]; ok {
		return sh, nil
	}
	if rn.sceneMappings == nil {
		rn.sceneMappings = rn.rt.sceneMappings(ctx)
	}
	st := rn.rt.Store
	w, err := st.Wanted().Get(ctx, seriesID)
	if err != nil {
		return nil, fmt.Errorf("wanted series %s: %w", seriesID, err)
	}
	it, err := st.Items().Get(ctx, seriesID)
	if err != nil {
		return nil, fmt.Errorf("wanted series %s: %w", seriesID, err)
	}
	if it.Kind != core.KindSeries {
		return nil, fmt.Errorf("%w: item %s is not a series", core.ErrInvalid, seriesID)
	}
	lib, err := rn.library(ctx, it.LibraryID)
	if err != nil {
		return nil, err
	}
	p, err := rn.profile(ctx, w.QualityProfileID)
	if err != nil {
		return nil, err
	}
	sh := &show{item: it, wanted: w, lib: lib, byID: map[core.ID]*series.Episode{}, items: map[core.ID]core.Item{}, seasons: map[int]core.Item{}}
	s := &series.Series{
		Target: acquire.Target{
			ID: it.ID, Kind: release.Series, Title: it.Name, Year: it.ProductionYear,
			OriginalLanguage: release.LanguageUnknown, Tags: w.Tags, Profile: p, Monitored: it.Monitored,
		},
		Type: seriesType(it.SeriesType), Runtime: int(it.Runtime.Minutes()), ImdbID: it.ExternalIDs[core.ProviderIMDb],
	}
	s.TvdbID, _ = strconv.Atoi(it.ExternalIDs[core.ProviderTVDB])
	sh.series = s
	s.Path = rn.seriesFolder(sh)

	var tree []core.Item
	for child, err := range st.Items().Walk(ctx, core.ItemQuery{
		ParentID: seriesID, Recursive: true, Kinds: []core.ItemKind{core.KindSeason, core.KindEpisode},
		IncludeVirtual: true, IncludeMissing: true,
	}) {
		if err != nil {
			return nil, err
		}
		tree = append(tree, child)
	}
	numbers := rn.rt.sceneNumbers(ctx, s.TvdbID)
	s.UseSceneNumbering = len(numbers) > 0
	for _, c := range tree {
		switch {
		case c.Kind == core.KindSeason && c.IndexNumber != nil:
			if have, ok := sh.seasons[*c.IndexNumber]; !ok || (have.Virtual && !c.Virtual) {
				sh.seasons[*c.IndexNumber] = c
			}
			if !slices.Contains(s.Seasons, *c.IndexNumber) {
				s.Seasons = append(s.Seasons, *c.IndexNumber)
			}
			continue
		case c.Kind != core.KindEpisode || c.ParentIndexNumber == nil || c.IndexNumber == nil:
			continue
		}
		if !slices.Contains(s.Seasons, *c.ParentIndexNumber) {
			s.Seasons = append(s.Seasons, *c.ParentIndexNumber)
		}
		var file *acquire.File
		if hasFile(c) {
			if file, err = rn.episodeFile(ctx, c); err != nil {
				return nil, err
			}
		}
		last := *c.IndexNumber
		if c.IndexNumberEnd != nil && *c.IndexNumberEnd > last && hasFile(c) {
			last = *c.IndexNumberEnd
		}
		for n := *c.IndexNumber; n <= last; n++ {
			id := c.ID
			if n != *c.IndexNumber {
				id = coveredID(c.ID, n)
				if _, taken := sh.byID[id]; taken {
					continue
				}
			}
			ep := &series.Episode{
				Item:          acquire.Item{ID: id, Monitored: c.Monitored, Runtime: int(c.Runtime.Minutes()), File: file},
				SeasonNumber:  *c.ParentIndexNumber,
				EpisodeNumber: n,
			}
			if n == *c.IndexNumber {
				ep.Title = c.Name
				if c.AbsoluteNumber != nil {
					ep.AbsoluteEpisodeNumber = *c.AbsoluteNumber
				}
			} else if c.AbsoluteNumber != nil {
				ep.AbsoluteEpisodeNumber = *c.AbsoluteNumber + n - *c.IndexNumber
			}
			if c.PremiereDate != nil {
				ep.AirDateUTC = c.PremiereDate.UTC()
				ep.AirDate = c.PremiereDate.UTC().Format(time.DateOnly)
			}
			if sn, ok := numbers[[2]int{ep.SeasonNumber, ep.EpisodeNumber}]; ok {
				ep.SceneSeasonNumber, ep.SceneEpisodeNumber = new(sn.season), new(sn.episode)
				if sn.hasAbsolute {
					ep.SceneAbsoluteEpisodeNumber = new(sn.absolute)
				}
			}
			sh.episodes = append(sh.episodes, ep)
			sh.byID[id] = ep
			sh.items[id] = c
			if id != c.ID {
				rn.holders[id] = c.ID
			}
		}
	}
	slices.Sort(s.Seasons)
	slices.SortStableFunc(sh.episodes, func(a, b *series.Episode) int {
		return cmp.Or(cmp.Compare(a.SeasonNumber, b.SeasonNumber), cmp.Compare(a.EpisodeNumber, b.EpisodeNumber))
	})
	rn.shows[seriesID] = sh
	return sh, nil
}

// seriesType returns the decisions' type of a series type.
func seriesType(t core.SeriesType) series.Type {
	switch t {
	case core.SeriesDaily:
		return series.Daily
	case core.SeriesAnime:
		return series.Anime
	}
	return series.Standard
}

// episodeFile returns the file an episode item has: its quality and
// scene name those of the import that brought it, else parsed from its
// path. Its ID is the item's, shared by the episodes it holds.
func (rn *run) episodeFile(ctx context.Context, it core.Item) (*acquire.File, error) {
	st := rn.rt.Store
	f := &acquire.File{ID: it.ID, DateAdded: it.DateAdded}
	sources, err := st.MediaSources().ListForItem(ctx, it.ID)
	if err != nil {
		return nil, err
	}
	if len(sources) > 0 {
		f.Size = sources[0].Size
	}
	title := strings.TrimSuffix(path.Base(it.Path), path.Ext(it.Path))
	if parsed := release.ParseSeriesPath(it.Path); parsed != nil {
		f.Quality, f.ReleaseGroup, f.Languages = acquire.ModelOf(parsed.Quality), parsed.ReleaseGroup, parsed.Languages
	} else {
		f.Quality = acquire.ModelOf(release.ParseQuality(path.Base(it.Path), release.Series))
	}
	imported, err := st.History().List(ctx, core.HistoryQuery{ItemIDs: []core.ID{it.ID}, Types: []core.HistoryEventType{core.HistoryImported}, Limit: 20})
	if err != nil {
		return nil, err
	}
	for _, h := range imported.Items {
		if h.ItemID != it.ID || h.Data[dataImportedPath] != it.Path {
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
	f.Formats = rn.seriesFormats(title, f.Quality, f.Languages, f.Size, nil)
	return f, nil
}

// organizeSeries returns what names a series' folders and files.
func organizeSeries(sh *show) organize.Series {
	it := sh.item
	s := organize.Series{
		Title: it.Name, Year: it.ProductionYear, ImdbID: it.ExternalIDs[core.ProviderIMDb],
		Path: sh.series.Path, SeasonFolder: sh.wanted.SeasonFolder, Type: organize.SeriesStandard,
	}
	switch it.SeriesType {
	case core.SeriesDaily:
		s.Type = organize.SeriesDaily
	case core.SeriesAnime:
		s.Type = organize.SeriesAnime
	}
	s.TvdbID, _ = strconv.Atoi(it.ExternalIDs[core.ProviderTVDB])
	s.TmdbID, _ = strconv.Atoi(it.ExternalIDs[core.ProviderTMDB])
	s.TvMazeID, _ = strconv.Atoi(it.ExternalIDs[core.ProviderTVMaze])
	return s
}

// seriesNaming returns the series naming settings: Sonarr's defaults,
// renaming and replacing characters as the movie settings say.
func (rn *run) seriesNaming() organize.SeriesNaming {
	n := organize.DefaultSeriesNaming()
	s := rn.settings.Naming
	n.RenameEpisodes, n.ReplaceIllegalCharacters = s.RenameMovies, s.ReplaceIllegalCharacters
	if s.ColonReplacement != "" {
		n.ColonReplacement = organize.ColonReplacement(s.ColonReplacement)
	}
	return n
}

// seriesFolder returns the folder of a series: its own, else one named
// by the naming settings in the library's first folder.
func (rn *run) seriesFolder(sh *show) string {
	if !sh.item.Virtual && sh.item.Path != "" {
		return filepath.ToSlash(sh.item.Path)
	}
	if len(sh.lib.Paths) == 0 {
		return ""
	}
	s := organizeSeries(sh)
	s.Path = ""
	return path.Join(filepath.ToSlash(filepath.Clean(sh.lib.Paths[0])), organize.SeriesFolder(rn.seriesNaming(), s))
}

// shows returns every wanted series, for matching releases.
func (rn *run) allShows(ctx context.Context) ([]*show, error) {
	if rn.showsRead {
		return rn.showList, nil
	}
	wanted, err := rn.rt.Store.Wanted().List(ctx)
	if err != nil {
		return nil, err
	}
	for _, w := range wanted {
		sh, err := rn.show(ctx, w.ItemID)
		if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrInvalid) {
			continue
		}
		if err != nil {
			return nil, err
		}
		rn.showList = append(rn.showList, sh)
	}
	rn.showsRead = true
	return rn.showList, nil
}

// parsing returns what maps episode releases to the wanted series, as
// Sonarr's ParsingService over its SeriesService, EpisodeService and
// SceneMappingService.
func (rn *run) parsing(ctx context.Context) (*series.Parsing, error) {
	if rn.parser != nil {
		return rn.parser, nil
	}
	shows, err := rn.allShows(ctx)
	if err != nil {
		return nil, err
	}
	catalog := &series.Catalog{Load: func(ctx context.Context, id core.ID) ([]*series.Episode, error) {
		sh, err := rn.show(ctx, id)
		if err != nil {
			return nil, err
		}
		return sh.episodes, nil
	}}
	if rn.sceneMappings == nil {
		rn.sceneMappings = rn.rt.sceneMappings(ctx)
	}
	for _, sh := range shows {
		catalog.List = append(catalog.List, sh.series)
	}
	rn.parser = &series.Parsing{Series: catalog, Episodes: catalog, Scene: rn.sceneMappings, Logger: rn.rt.logger()}
	return rn.parser, nil
}

// findEpisodes returns the FindEpisodes of decisions: Sonarr's
// ParsingService.Map, with a pending release's own series.
func (rn *run) findEpisodes(ctx context.Context, b *batch) (series.FindEpisodes, series.ParseSpecial, error) {
	p, err := rn.parsing(ctx)
	if err != nil {
		return nil, nil, err
	}
	known := func(r *acquire.Release) *series.Series {
		id, ok := b.targets[r]
		if !ok {
			return nil
		}
		sh, err := rn.show(ctx, id)
		if err != nil {
			return nil
		}
		return sh.series
	}
	return p.FindEpisodes(known), p.ParseSpecial(), nil
}

// episodePorts are the episode decisions' view of the store and disks.
type episodePorts struct{ rn *run }

// EpisodesBySeason implements series.EpisodePorts.
func (p episodePorts) EpisodesBySeason(ctx context.Context, seriesID core.ID, season int) ([]*series.Episode, error) {
	sh, err := p.rn.show(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	var out []*series.Episode
	for _, ep := range sh.episodes {
		if ep.SeasonNumber == season {
			out = append(out, ep)
		}
	}
	return out, nil
}

// EpisodeIDsInFile implements series.EpisodePorts: a file's ID is that of
// the item holding it.
func (p episodePorts) EpisodeIDsInFile(_ context.Context, fileID core.ID) ([]core.ID, error) {
	for _, sh := range p.rn.shows {
		var ids []core.ID
		for _, ep := range sh.episodes {
			if ep.File != nil && ep.File.ID == fileID {
				ids = append(ids, ep.ID)
			}
		}
		if len(ids) > 0 {
			return ids, nil
		}
	}
	return []core.ID{fileID}, nil
}

// FileExists implements series.EpisodePorts.
func (p episodePorts) FileExists(ctx context.Context, s *series.Series, f *acquire.File) (bool, error) {
	it, err := p.rn.rt.Store.Items().Get(ctx, f.ID)
	if errors.Is(err, core.ErrNotFound) {
		return false, nil
	}
	if err != nil || it.Path == "" {
		return false, err
	}
	_, err = os.Stat(filepath.FromSlash(it.Path))
	return err == nil, nil
}

// Pending implements series.EpisodePorts.
func (p episodePorts) Pending(ctx context.Context, seriesID core.ID, episodeIDs []core.ID) ([]acquire.Release, error) {
	pending, err := p.rn.rt.pending(ctx, seriesID)
	if err != nil {
		return nil, err
	}
	var out []acquire.Release
	for _, pr := range pending {
		if slices.ContainsFunc(pr.ItemIDs, func(id core.ID) bool { return slices.Contains(episodeIDs, id) }) {
			out = append(out, *pr.release.decisionRelease())
		}
	}
	return out, nil
}

// holderOf returns the item holding an episode: a multi-episode file's
// for the episodes it covers beyond its first, else the episode's own.
func (rn *run) holderOf(id core.ID) core.ID {
	if h, ok := rn.holders[id]; ok {
		return h
	}
	return id
}

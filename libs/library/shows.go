package library

import (
	"path"
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/naming"
)

// resolveShows resolves a folder of a shows library: folders in the root
// or in plain folders are series, folders in a series are seasons, and
// video files are episodes.
func (r *Resolver) resolveShows(scope Scope, dir string, entries []Entry, l Lister) (Result, error) {
	files, dirs := split(entries)
	var res Result
	child := func(c Container, parent string, season *int) Scope {
		return Scope{Kind: scope.Kind, Root: scope.Root, Parent: c, ParentPath: parent, SeasonNumber: season}
	}
	switch {
	case dir == scope.Root:
		for _, d := range dirs {
			res.Subfolders = append(res.Subfolders, Subfolder{d.Path, child(InFolder, dir, nil)})
		}
		return res, nil

	case scope.Parent == InFolder:
		series := r.series(dir)
		extras, err := r.Extras(series, true, entries, l)
		if err != nil {
			return res, err
		}
		series.Extras = extras
		res.Item = &series
		res.Items = r.episodes(dir, files, nil, false)
		for _, d := range dirs {
			res.Subfolders = append(res.Subfolders, Subfolder{d.Path, child(InSeries, dir, nil)})
		}
		return res, nil

	case scope.Parent == InSeries:
		season, ok, err := r.season(dir, scope.ParentPath, l)
		if err != nil {
			return res, err
		}
		var number *int
		next := InSeries
		if ok {
			res.Item = &season
			number, next = season.Index, InSeason
		}
		res.Items = r.episodes(dir, files, number, ok)
		for _, d := range dirs {
			res.Subfolders = append(res.Subfolders, Subfolder{d.Path, child(next, dir, number)})
		}
		return res, nil
	}
	// Folders nested in a season keep its number.
	res.Items = r.episodes(dir, files, scope.SeasonNumber, true)
	for _, d := range dirs {
		res.Subfolders = append(res.Subfolders, Subfolder{d.Path, child(InSeason, scope.ParentPath, scope.SeasonNumber)})
	}
	return res, nil
}

// series ports SeriesResolver: the name and year come from the folder and
// provider IDs tagged in its name.
func (r *Resolver) series(dir string) Node {
	s := r.Parser.ResolveSeries(dir)
	n := Node{Kind: core.KindSeries, Path: dir, Name: s.Name, Year: s.Year}
	if n.Name == "" {
		n.Name = path.Base(dir)
	}
	setIDs(&n, path.Base(dir), seriesIDs)
	return n
}

// season ports SeasonResolver: a folder in a series is a season unless its
// name is an episode's, or it holds no video at all.
func (r *Resolver) season(dir, seriesDir string, l Lister) (Node, bool, error) {
	parsed := naming.ParseSeasonPath(dir, seriesDir, true, true)
	n := Node{Kind: core.KindSeason, Path: dir, Index: parsed.SeasonNumber}
	if parsed.SeasonNumber == nil || !parsed.IsSeasonFolder {
		ep := r.Parser.ParseEpisodePath(`\\test\`+path.Base(dir), true, naming.EpisodeOptions{})
		if ep.EpisodeNumber != nil && ep.SeasonNumber != nil {
			return Node{}, false, nil
		}
		if l != nil {
			has, err := r.hasVideo(dir, l)
			if err != nil || !has {
				return Node{}, false, err
			}
		}
	}
	switch {
	case n.Index != nil && *n.Index == 0:
		n.Name = "Specials"
	case n.Index != nil:
		n.Name = "Season " + strconv.Itoa(*n.Index)
	default:
		n.Name = path.Base(dir)
	}
	setIDs(&n, path.Base(dir), seasonIDs)
	return n, true, nil
}

// hasVideo reports whether a folder or one of its subfolders holds a video
// file.
func (r *Resolver) hasVideo(dir string, l Lister) (bool, error) {
	entries, err := l.List(dir)
	if err != nil {
		return false, err
	}
	for _, e := range entries {
		if !e.IsDir && r.Parser.IsVideoFile(e.Path) {
			return true, nil
		}
	}
	for _, e := range entries {
		if e.IsDir {
			if has, err := r.hasVideo(e.Path, l); err != nil || has {
				return has, err
			}
		}
	}
	return false, nil
}

// episodes ports EpisodeResolver for the video files of a folder: numbers
// come from the file names, provider IDs from the file name alone. An
// episode without a season number is in season, or in season 1 when it is
// not in a season folder.
func (r *Resolver) episodes(dir string, files []Entry, season *int, inSeason bool) []Node {
	nodes := r.videos(dir, files, core.KindEpisode, false, false)
	for i := range nodes {
		n := &nodes[i]
		ep := r.Parser.ParseEpisodePath(n.Path, false, naming.EpisodeOptions{})
		if ep.Success {
			n.ParentIndex, n.Index, n.IndexEnd = ep.SeasonNumber, ep.EpisodeNumber, ep.EndingEpisodeNumber
			if ep.IsByDate && ep.Year != nil && ep.Month != nil && ep.Day != nil {
				t := time.Date(*ep.Year, time.Month(*ep.Month), *ep.Day, 0, 0, 0, 0, time.UTC)
				n.Aired = &t
			}
		}
		switch {
		case n.ParentIndex != nil:
		case inSeason:
			n.ParentIndex = season
		case n.Index != nil || n.Aired != nil:
			one := 1
			n.ParentIndex = &one
		}
		setIDs(n, fileNameWithoutExt(n.Path), episodeIDs)
	}
	return nodes
}

func seasonOf(dir, seriesDir string) bool {
	return naming.ParseSeasonPath(dir, seriesDir, false, false).SeasonNumber != nil
}

// episodeOptionsNoExtended parses only what identifies an episode.
func episodeOptionsNoExtended() naming.EpisodeOptions {
	named, optimistic := true, false
	return naming.EpisodeOptions{Named: &named, Optimistic: &optimistic, SkipExtendedInfo: true}
}

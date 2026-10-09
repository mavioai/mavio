package library

import (
	"path"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

var (
	imageExtensions = []string{".jpeg", ".jpg", ".png", ".dng", ".webp", ".gif", ".bmp", ".ico", ".astc", ".ktx", ".pkm", ".wbmp", ".heic", ".heif", ".avif"}
	// artworkNames are local artwork of the items beside them, not photos.
	artworkNames = []string{"folder", "thumb", "landscape", "fanart", "backdrop", "poster", "cover", "logo", "default"}
)

// resolveHomeVideos resolves a folder of a home videos or photos library:
// folders are albums, videos and photos the items in them.
func (r *Resolver) resolveHomeVideos(scope Scope, dir string, entries []Entry) (Result, error) {
	files, dirs := split(entries)
	var res Result
	if dir != scope.Root {
		kind := core.KindFolder
		if scope.Kind == core.LibraryPhotos {
			kind = core.KindPhotoAlbum
		}
		res.Item = &Node{Kind: kind, Path: dir, Name: path.Base(dir)}
	}
	res.Items = r.videos(dir, files, core.KindVideo, false, true)
	res.Items = append(res.Items, r.photos(files)...)
	for _, d := range dirs {
		res.Subfolders = append(res.Subfolders, Subfolder{d.Path, Scope{Kind: scope.Kind, Root: scope.Root, ParentPath: dir}})
	}
	return res, nil
}

// photos ports PhotoResolver: image files that are neither artwork nor
// named after a video beside them.
func (r *Resolver) photos(files []Entry) []Node {
	var nodes []Node
	for _, f := range files {
		if !slices.Contains(imageExtensions, strings.ToLower(path.Ext(f.Path))) {
			continue
		}
		name := strings.ToLower(fileNameWithoutExt(f.Path))
		if slices.ContainsFunc(artworkNames, func(a string) bool { return strings.HasPrefix(name, a) }) {
			continue
		}
		owned := slices.ContainsFunc(files, func(v Entry) bool {
			return r.Parser.IsVideoFile(v.Path) && strings.HasPrefix(name, strings.ToLower(fileNameWithoutExt(v.Path)))
		})
		if !owned {
			nodes = append(nodes, Node{Kind: core.KindPhoto, Path: f.Path, Name: fileNameWithoutExt(f.Path)})
		}
	}
	return nodes
}

// resolveMixed resolves a folder of a mixed library: a folder with a
// tvshow.nfo, season folders or numbered episodes is a series; anything
// else follows the movie rules.
func (r *Resolver) resolveMixed(scope Scope, dir string, entries []Entry, l Lister) (Result, error) {
	if dir != scope.Root && scope.Parent == InFolder && r.isSeriesFolder(dir, entries) {
		shows := scope
		shows.Kind = core.LibraryShows
		return r.resolveShows(shows, dir, entries, l)
	}
	return r.resolveMovies(scope, dir, entries, l, core.KindMovie, true)
}

// isSeriesFolder ports SeriesResolver.IsSeriesFolder for folders outside
// shows libraries, which need a keyword in season folder names.
func (r *Resolver) isSeriesFolder(dir string, entries []Entry) bool {
	for _, e := range entries {
		if !e.IsDir && strings.EqualFold(e.Name(), "tvshow.nfo") {
			return true
		}
	}
	for _, e := range entries {
		if e.IsDir {
			if seasonOf(e.Path, dir) {
				return true
			}
			continue
		}
		if r.Parser.IsVideoFile(e.Path) {
			ep := r.Parser.ParseEpisodePath(e.Path, false, episodeOptionsNoExtended())
			if ep.EpisodeNumber != nil {
				return true
			}
		}
	}
	return false
}

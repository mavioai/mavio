package library

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/naming"
)

// Entry is a file or folder of a library, by slash-separated path.
type Entry struct {
	Path  string
	IsDir bool
}

// Name returns the last element of the path.
func (e Entry) Name() string { return path.Base(e.Path) }

// Lister lists the entries of a folder; resolving looks into extras folders
// and disc folders through it.
type Lister interface {
	List(dir string) ([]Entry, error)
}

// Container is what a folder resolved to, as seen by its subfolders.
type Container int

// Containers.
const (
	// InFolder is a plain folder, or the library root.
	InFolder Container = iota
	InSeries
	InSeason
	InMusicArtist
)

// Scope places a folder in its library.
type Scope struct {
	Kind core.LibraryKind
	// Root is the library folder the scanned folder belongs to.
	Root string
	// Parent is what the containing folder resolved to, and ParentPath
	// its path.
	Parent     Container
	ParentPath string
	// SeasonNumber is the number of the containing season, for episodes
	// whose names have none.
	SeasonNumber *int
}

// Node is an item found by resolving a folder, before it is stored.
type Node struct {
	Kind core.ItemKind
	// Path is the media file, or the folder for items made of a folder:
	// series, seasons, albums, artists and disc rips.
	Path string
	Name string
	Year *int
	// ParentIndex and Index are the season and episode numbers, or disc
	// and track numbers; IndexEnd ends a multi-episode file.
	ParentIndex, Index, IndexEnd *int
	// Aired is the air date of a daily show's episode.
	Aired       *time.Time
	ExternalIDs map[core.Provider]string
	// Parts are further files of a stacked video, after Path.
	Parts []string
	// Versions are alternate versions, by their first file.
	Versions []Version
	// Disc is "dvd" or "bluray" for disc folders.
	Disc     string
	Format3D core.Video3DFormat
	// Extra is set for extras, which belong to the item they are listed
	// under.
	Extra  core.ExtraKind
	Extras []Node
	// InMixedFolder is set for items sharing their folder with others.
	InMixedFolder bool
	// generatedNames are the names extra numbering may have given this
	// extra, see RenewExtraName.
	generatedNames []string
}

// Version is an alternate version of a video.
type Version struct {
	Path  string
	Parts []string
}

// Subfolder is a folder to resolve next.
type Subfolder struct {
	Path  string
	Scope Scope
}

// Result is what a folder resolved to.
type Result struct {
	// Item is set when the folder itself is an item: a movie in its own
	// folder, a disc rip, a series, a season, an album, an artist or an
	// audiobook.
	Item *Node
	// Items are made of the folder's files; they belong to Item, or to the
	// folder's container.
	Items []Node
	// Subfolders are resolved next.
	Subfolders []Subfolder
}

// Resolver maps folders to items following Jellyfin's naming rules.
type Resolver struct {
	Parser *naming.Parser
}

// NewResolver returns a resolver using the default naming rules.
func NewResolver() *Resolver { return &Resolver{Parser: naming.Default()} }

// Resolve resolves the folder dir with its entries.
func (r *Resolver) Resolve(scope Scope, dir string, entries []Entry, l Lister) (Result, error) {
	entries = r.visible(scope, dir, entries)
	switch scope.Kind {
	case core.LibraryMovies:
		return r.resolveMovies(scope, dir, entries, l, core.KindMovie, true)
	case core.LibraryMusicVideos:
		return r.resolveMovies(scope, dir, entries, l, core.KindMusicVideo, false)
	case core.LibraryHomeVideos, core.LibraryPhotos:
		return r.resolveHomeVideos(scope, dir, entries)
	case core.LibraryShows:
		return r.resolveShows(scope, dir, entries, l)
	case core.LibraryMusic:
		return r.resolveMusic(scope, dir, entries, l)
	case core.LibraryBooks:
		return r.resolveBooks(scope, dir, entries)
	case core.LibraryMixed:
		return r.resolveMixed(scope, dir, entries, l)
	}
	return Result{}, fmt.Errorf("%w: unknown library kind %q", core.ErrInvalid, scope.Kind)
}

// visible drops ignored entries: the built-in patterns, extras folders
// below the top level, which belong to the items beside them, and theme
// songs, which are extras.
func (r *Resolver) visible(scope Scope, dir string, entries []Entry) []Entry {
	top := dir == scope.Root
	return slices.DeleteFunc(slices.Clone(entries), func(e Entry) bool {
		return r.ignored(e, top)
	})
}

// ignored ports CoreResolutionIgnoreRule for entries of a folder; top says
// the folder is the library root.
func (r *Resolver) ignored(e Entry, top bool) bool {
	if IgnoredPath(e.Path) {
		return true
	}
	if e.IsDir {
		if top {
			return false
		}
		_, extras := r.extrasFolderKind(e.Name())
		return extras
	}
	return r.isThemeSong(e.Path)
}

// isThemeSong reports whether a file is a "theme" audio file.
func (r *Resolver) isThemeSong(p string) bool {
	name := path.Base(p)
	return strings.TrimSuffix(name, path.Ext(name)) == "theme" && r.Parser.IsAudioFile(p)
}

// extrasFolderKind returns the extra kind of an extras folder name.
func (r *Resolver) extrasFolderKind(name string) (core.ExtraKind, bool) {
	for _, rule := range r.Parser.Options().VideoExtraRules {
		if rule.Type == naming.ExtraDirectoryName && strings.EqualFold(rule.Token, name) {
			return rule.Kind, true
		}
	}
	return "", false
}

// sampleName matches files that are samples whatever their numbering.
var sampleName = regexp.MustCompile(`(?i)\bsample\b`)

func split(entries []Entry) (files, dirs []Entry) {
	for _, e := range entries {
		if e.IsDir {
			dirs = append(dirs, e)
		} else {
			files = append(files, e)
		}
	}
	return files, dirs
}

// setIDs sets the provider IDs tagged in name.
func setIDs(n *Node, name string, providers map[string]core.Provider) {
	for attr, provider := range providers {
		if id, err := AttributeValue(name, attr); err == nil && id != "" {
			if n.ExternalIDs == nil {
				n.ExternalIDs = map[core.Provider]string{}
			}
			n.ExternalIDs[provider] = id
		}
	}
}

var (
	seriesIDs = map[string]core.Provider{
		"imdbid": core.ProviderIMDb, "tvdbid": core.ProviderTVDB, "tvmazeid": core.ProviderTVMaze, "tmdbid": core.ProviderTMDB,
		"anidbid": core.ProviderAniDB, "anilistid": core.ProviderAniList, "anisearchid": core.ProviderAniSearch,
	}
	seasonIDs = map[string]core.Provider{
		"tvdbid": core.ProviderTVDB, "tvmazeid": core.ProviderTVMaze, "tmdbid": core.ProviderTMDB,
		"anidbid": core.ProviderAniDB, "anilistid": core.ProviderAniList, "anisearchid": core.ProviderAniSearch,
	}
	episodeIDs = map[string]core.Provider{
		"imdbid": core.ProviderIMDb, "tvdbid": core.ProviderTVDB, "tvmazeid": core.ProviderTVMaze, "tmdbid": core.ProviderTMDB,
	}
)

func fileNameWithoutExt(p string) string {
	name := path.Base(p)
	return strings.TrimSuffix(name, path.Ext(name))
}

func format3D(is3D bool, s string) core.Video3DFormat {
	if !is3D {
		return ""
	}
	switch strings.ToLower(s) {
	case "fsbs":
		return core.Video3DFullSideBySide
	case "ftab":
		return core.Video3DFullTopAndBottom
	case "hsbs", "sbs", "sbs3d":
		return core.Video3DHalfSideBySide
	case "htab", "tab":
		return core.Video3DHalfTopAndBottom
	case "mvc":
		return core.Video3DMVC
	}
	return ""
}

// disc reports whether a folder holds a disc rip: a DVD VIDEO_TS folder
// with VOB files or VIDEO_TS.IFO, or a Blu-ray BDMV folder.
func disc(entries []Entry, l Lister) (string, error) {
	for _, e := range entries {
		name := strings.ToLower(e.Name())
		switch {
		case e.IsDir && name == "bdmv":
			return "bluray", nil
		case !e.IsDir && name == "video_ts.ifo":
			return "dvd", nil
		case e.IsDir && name == "video_ts" && l != nil:
			sub, err := l.List(e.Path)
			if err != nil {
				return "", err
			}
			if slices.ContainsFunc(sub, func(f Entry) bool { return !f.IsDir && strings.EqualFold(path.Ext(f.Path), ".vob") }) {
				return "dvd", nil
			}
		}
	}
	return "", nil
}

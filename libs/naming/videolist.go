package naming

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// Video is a movie or episode made of one or more files (stacked parts),
// with its alternate versions.
type Video struct {
	Name              string
	Year              *int
	Files             []VideoFile
	AlternateVersions []Video
	Extra             core.ExtraKind
}

// VideoListOptions configures ResolveVideos.
type VideoListOptions struct {
	// NoMultiVersion keeps alternate versions as separate videos.
	NoMultiVersion bool
	// NoNameParsing keeps the names of stacked files as they are.
	NoNameParsing bool
	// LibraryRoot is the library folder; see ResolveVideo.
	LibraryRoot string
	// TVShows groups alternate versions by episode instead of by the
	// movie folder.
	TVShows bool
}

var (
	resolutionToken   = mustCompile(`[0-9]{2}[0-9]+[ip]`, true)
	multiVersionLabel = mustCompile(`^\[([^]]*)\]`, false)
)

// ResolveVideos groups the video files of a folder into videos: stacked
// parts become one video, alternate versions of a movie or episode are
// attached to the primary version, and extras are listed separately.
func (p *Parser) ResolveVideos(files []VideoFile, o VideoListOptions) []Video {
	// Extras could prevent stacks from being recognized.
	var nonExtras []FileEntry
	for _, f := range files {
		if f.Extra == "" {
			nonExtras = append(nonExtras, FileEntry{Path: f.Path, IsDir: f.IsDir})
		}
	}
	stacks := p.ResolveStacks(nonExtras)

	var standalone, remaining []VideoFile
	for _, f := range files {
		if slices.ContainsFunc(stacks, func(s FileStack) bool { return s.ContainsFile(f.Path, f.IsDir) }) {
			continue
		}
		if f.Extra == "" {
			standalone = append(standalone, f)
		} else {
			remaining = append(remaining, f)
		}
	}

	var list []*Video
	for _, s := range stacks {
		v := &Video{Name: s.Name}
		for _, path := range s.Files {
			if f, ok := p.ResolveVideo(path, s.IsDir, !o.NoNameParsing, o.LibraryRoot); ok {
				v.Files = append(v.Files, f)
			}
		}
		v.Year = v.Files[0].Year
		list = append(list, v)
	}
	for _, f := range standalone {
		list = append(list, &Video{Name: f.Name, Year: f.Year, Files: []VideoFile{f}})
	}

	if !o.NoMultiVersion {
		if o.TVShows {
			list = p.groupEpisodeVersions(list)
		} else {
			list = p.groupMovieVersions(list)
		}
	}

	out := make([]Video, 0, len(list)+len(remaining))
	for _, v := range list {
		out = append(out, *v)
	}
	for _, f := range remaining {
		out = append(out, Video{Name: f.Name, Year: f.Year, Files: []VideoFile{f}, Extra: f.Extra})
	}
	return out
}

// groupMovieVersions merges the videos of a movie folder into one when all
// are versions of the movie: named after the folder, with the same year.
func (p *Parser) groupMovieVersions(videos []*Video) []*Video {
	if len(videos) == 0 {
		return videos
	}
	folder := fileName(dirName(videos[0].Files[0].Path))
	if len([]rune(folder)) <= 1 || !sameYear(videos) {
		return videos
	}
	var primary *Video
	for _, v := range videos {
		if v.Extra != "" {
			continue
		}
		name := v.Files[0].FileNameWithoutExtension()
		if !p.isVersionOf(folder, name) {
			return videos
		}
		if folder == name {
			primary = v
		}
	}
	return []*Video{organizeVersions(videos, primary, folder)}
}

func sameYear(videos []*Video) bool {
	year := func(v *Video) int {
		if v.Year == nil {
			return -1
		}
		return *v.Year
	}
	for _, v := range videos[1:] {
		if year(v) != year(videos[0]) {
			return false
		}
	}
	return true
}

// isVersionOf reports whether file names a version of the movie folder:
// the folder name followed by nothing but release tags, a separator or a
// bracketed label.
func (p *Parser) isVersionOf(folder, file string) bool {
	if !hasPrefixFold(file, folder) {
		return false
	}
	rest := strings.TrimSpace(file[len(folder):])
	if cleaned, ok := p.CleanString(rest); ok {
		rest = strings.TrimSpace(cleaned)
	}
	return rest == "" || rest[0] == '-' || rest[0] == '_' || rest[0] == '.' || multiVersionLabel.isMatch(rest)
}

// groupEpisodeVersions merges files of the same episode into one video.
func (p *Parser) groupEpisodeVersions(videos []*Video) []*Video {
	if len(videos) < 2 {
		return videos
	}
	var result []*Video
	var keys []string
	groups := map[string][]*Video{}
	for _, v := range videos {
		key, ok := p.episodeVersionKey(v.Files[0].Path)
		if !ok {
			result = append(result, v)
			continue
		}
		key = strings.ToLower(key)
		if _, seen := groups[key]; !seen {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], v)
	}
	for _, key := range keys {
		group := groups[key]
		if len(group) == 1 {
			result = append(result, group[0])
			continue
		}
		result = append(result, organizeVersions(group, nil, ""))
	}
	return result
}

// episodeVersionKey identifies the episode of a path. Optimistic
// expressions are guesses and are not consulted: merging is destructive.
func (p *Parser) episodeVersionKey(path string) (string, bool) {
	r := p.ParseEpisodePath(path, false, EpisodeOptions{Optimistic: ptr(false), SkipExtendedInfo: true})
	switch {
	case !r.Success:
		return "", false
	case r.IsByDate:
		if r.Year == nil || r.Month == nil || r.Day == nil {
			return "", false
		}
		return fmt.Sprintf("D%d%02d%02d", *r.Year, *r.Month, *r.Day), true
	case r.SeasonNumber == nil || r.EpisodeNumber == nil:
		return "", false
	}
	return fmt.Sprintf("S%dE%d", *r.SeasonNumber, *r.EpisodeNumber), true
}

// organizeVersions picks the primary version and attaches the others,
// highest resolution first, then by name.
func organizeVersions(videos []*Video, primary *Video, name string) *Video {
	if len(videos) > 1 {
		type entry struct {
			name, resolution string
			hasResolution    bool
			v                *Video
		}
		var withRes, without []entry
		for _, v := range videos {
			e := entry{name: v.Files[0].FileNameWithoutExtension(), v: v}
			if m := resolutionToken.match(e.name); m != nil {
				e.resolution, _ = m.groupN(0)
				e.hasResolution = true
				withRes = append(withRes, e)
			} else {
				without = append(without, e)
			}
		}
		slices.SortStableFunc(withRes, func(a, b entry) int {
			if c := compareNumeric(b.resolution, a.resolution); c != 0 {
				return c
			}
			return compareNumeric(a.name, b.name)
		})
		slices.SortStableFunc(without, func(a, b entry) int { return compareNumeric(a.name, b.name) })
		videos = videos[:0:0]
		for _, e := range append(withRes, without...) {
			videos = append(videos, e.v)
		}
	}
	// A stacked version is preferred as the primary.
	if primary == nil {
		for _, v := range videos {
			if len(v.Files) > 1 {
				primary = v
				break
			}
		}
	}
	if primary == nil {
		primary = videos[0]
	}
	for _, v := range videos {
		if v != primary {
			primary.AlternateVersions = append(primary.AlternateVersions, *v)
		}
	}
	if name != "" {
		primary.Name = name
	}
	return primary
}

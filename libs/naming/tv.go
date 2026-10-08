package naming

import (
	"strings"
	"time"
	"unicode"

	"github.com/mavioai/mavio/libs/core"
)

// EpisodePath is what an episode path says about the episode.
type EpisodePath struct {
	Success    bool
	SeriesName string
	// SeasonNumber, EpisodeNumber and EndingEpisodeNumber are nil when
	// unknown; EndingEpisodeNumber is set for multi-episode files.
	SeasonNumber        *int
	EpisodeNumber       *int
	EndingEpisodeNumber *int
	// IsByDate is set for daily shows, whose air date is in Year, Month and
	// Day.
	IsByDate bool
	Year     *int
	Month    *int
	Day      *int
}

// EpisodeOptions restricts the episode expressions that are tried. A nil
// field does not restrict.
type EpisodeOptions struct {
	Named                   *bool
	Optimistic              *bool
	SupportsAbsoluteNumbers *bool
	// SkipExtendedInfo skips looking for the series name and the end of a
	// multi-episode range with further expressions.
	SkipExtendedInfo bool
}

// ParseEpisodePath parses an episode file or folder path with the episode
// expressions; the first matching expression wins.
func (p *Parser) ParseEpisodePath(path string, isDir bool, o EpisodeOptions) EpisodePath {
	// Some expressions require a file extension.
	if isDir {
		path += ".mp4"
	}
	var result EpisodePath
	found := false
	for _, e := range p.episodes {
		if (o.SupportsAbsoluteNumbers != nil && e.SupportsAbsoluteNumbers != *o.SupportsAbsoluteNumbers) ||
			(o.Named != nil && e.Named != *o.Named) ||
			(o.Optimistic != nil && e.Optimistic != *o.Optimistic) {
			continue
		}
		if r := parseEpisode(path, e); r.Success {
			result, found = r, true
			break
		}
	}
	if found && !o.SkipExtendedInfo {
		p.fillEpisodeInfo(path, &result)
		if result.SeriesName != "" {
			result.SeriesName = strings.TrimSpace(strings.Trim(strings.TrimSpace(result.SeriesName), "_.-"))
		}
	}
	return result
}

func parseEpisode(name string, e compiledEpisodeExpression) EpisodePath {
	var result EpisodePath
	// A hack for Windows Media Center names.
	if e.ByDate {
		name = strings.ReplaceAll(name, "_", "-")
	}
	m := e.re.match(name)
	if m == nil || m.groupCount() < 3 {
		return result
	}
	switch {
	case e.ByDate:
		whole, _ := m.groupN(0)
		if date, ok := parseDate(whole, e.DateFormats); ok {
			result.Year, result.Month, result.Day = ptr(date.Year()), ptr(int(date.Month())), ptr(date.Day())
		}
		// As in Jellyfin, a date expression succeeds even when the date
		// does not parse.
		result.Success = true
	case e.Named:
		if s, ok := m.group("seasonnumber"); ok {
			if n, ok := parseInt(s); ok {
				result.SeasonNumber = &n
			}
		}
		if s, ok := m.group("epnumber"); ok {
			if n, ok := parseInt(s); ok {
				result.EpisodeNumber = &n
			}
		}
		if s, ok := m.group("endingepnumber"); ok {
			// The end of a range must not be followed by more digits or by
			// "p" or "i" as in a resolution, so "series-s09e14-1080p.mkv"
			// is not episodes 14 to 108.
			runes := []rune(name)
			next := m.groupEnd("endingepnumber")
			if next >= len(runes) || !strings.ContainsRune("0123456789iIpP", runes[next]) {
				// A range cannot end before it starts; a lower number is
				// part of the title.
				if n, ok := parseInt(s); ok && result.EpisodeNumber != nil && n >= *result.EpisodeNumber {
					result.EndingEpisodeNumber = &n
				}
			}
		}
		result.SeriesName, _ = m.group("seriesname")
		result.Success = result.EpisodeNumber != nil
	default:
		if s, ok := m.groupN(1); ok {
			if n, ok := parseInt(s); ok {
				result.SeasonNumber = &n
			}
		}
		if s, ok := m.groupN(2); ok {
			if n, ok := parseInt(s); ok {
				result.EpisodeNumber = &n
			}
		}
		result.Success = result.EpisodeNumber != nil
	}
	// Seasons 200 to 1927 and above 2500 are errors, such as
	// "Series Special (1920x1080).mkv" read as season 1920 episode 1080.
	if s := result.SeasonNumber; s != nil && ((*s >= 200 && *s < 1928) || *s > 2500) {
		result.Success = false
	}
	result.IsByDate = e.ByDate
	return result
}

// fillEpisodeInfo looks for the series name and the end of a multi-episode
// range with the named multi-episode expressions, and the named episode
// expressions when the series name is missing.
func (p *Parser) fillEpisodeInfo(path string, info *EpisodePath) {
	var exprs []compiledEpisodeExpression
	if info.SeriesName == "" {
		for _, e := range p.episodes {
			if e.Named {
				exprs = append(exprs, e)
			}
		}
	}
	for _, e := range p.multipleEpisodes {
		if e.Named {
			exprs = append(exprs, e)
		}
	}
	for _, e := range exprs {
		r := parseEpisode(path, e)
		if !r.Success {
			continue
		}
		if info.SeriesName == "" {
			info.SeriesName = r.SeriesName
		}
		if info.EndingEpisodeNumber == nil && r.EndingEpisodeNumber != nil && info.EpisodeNumber != nil &&
			*r.EndingEpisodeNumber >= *info.EpisodeNumber {
			info.EndingEpisodeNumber = r.EndingEpisodeNumber
		}
		if info.SeriesName != "" && (info.EpisodeNumber == nil || info.EndingEpisodeNumber != nil) {
			break
		}
	}
}

// parseDate parses s in one of the .NET date formats, or in a few common
// layouts when there are none.
func parseDate(s string, formats []string) (time.Time, bool) {
	layouts := []string{time.DateTime, time.DateOnly, "2006/01/02", time.RFC3339}
	if len(formats) > 0 {
		layouts = layouts[:0]
		for _, f := range formats {
			layouts = append(layouts, strings.NewReplacer("yyyy", "2006", "MM", "01", "dd", "02").Replace(f))
		}
	}
	for _, l := range layouts {
		if t, err := time.Parse(l, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// Episode is a resolved episode file or folder.
type Episode struct {
	EpisodePath
	Path      string
	Container string
	Is3D      bool
	Format3D  string
	IsStub    bool
	StubType  string
}

// ResolveEpisode resolves an episode file or folder. It returns false for a
// file that is neither a video nor a stub, and for a video whose path does
// not parse.
func (p *Parser) ResolveEpisode(path string, isDir bool, o EpisodeOptions) (Episode, bool) {
	ep := Episode{Path: path}
	if !isDir {
		ext := extension(path)
		if !containsFold(p.opts.VideoFileExtensions, ext) {
			stubType, ok := p.ResolveStub(path)
			if !ok {
				return Episode{}, false
			}
			ep.IsStub, ep.StubType = true, stubType
		}
		ep.Container = strings.TrimLeft(ext, ".")
	}
	ep.Is3D, ep.Format3D = p.Parse3D(path)
	ep.EpisodePath = p.ParseEpisodePath(path, isDir, o)
	if !ep.Success && !ep.IsStub {
		return Episode{}, false
	}
	return ep, true
}

const seasonKeywords = `시즌|シーズン|сезон` +
	`|season|sæson|saison|staffel|series|stagione|säsong|seizoen|seasong` +
	`|sezon|sezona|sezóna|sezonul|série|séria|serie|seria|temporada|kausi`

var (
	seasonCleanName = mustCompile(`[ ._\-\[\]]`, false)
	seasonPre       = mustCompile(`^\s*((?<seasonnumber>(?>\d+))(?:st|nd|rd|th|\.)*(?!\s*[Ee]\d+))\s*(?:`+seasonKeywords+`)\s*(?<rightpart>.*)$`, true)
	seasonPost      = mustCompile(`^\s*(?:`+seasonKeywords+`)\s*(?<seasonnumber>\d+?)(?=\d{3,4}p|[^\d]|$)(?!\s*[Ee]\d)(?<rightpart>.*)$`, true)
	seasonPrefix    = mustCompile(`[sS](\d{1,4})(?!\d|[eE]\d)(?=\.|_|-|\[|\]|\s|$)`, false)
	seasonKeyword   = mustCompile(seasonKeywords, true)
)

// SeasonPath is what a season folder path says about the season.
type SeasonPath struct {
	Success      bool
	SeasonNumber *int
	// IsSeasonFolder is set when the folder holds a season rather than a
	// single episode.
	IsSeasonFolder bool
}

// ParseSeasonPath parses a season folder path. parentPath, if not empty,
// is the series folder, whose name is ignored in the season folder's name.
// supportSpecialAliases maps "Specials" and "Extras" to season 0;
// supportNumericSeasonFolders accepts bare numbers. Without both, as in
// mixed libraries, the name must contain a season keyword.
func ParseSeasonPath(path, parentPath string, supportSpecialAliases, supportNumericSeasonFolders bool) SeasonPath {
	parent := ""
	hasParent := parentPath != ""
	if hasParent {
		parent = fileName(strings.TrimRight(parentPath, `/\`))
	}
	n, isSeason := seasonNumberFromPath(path, parent, hasParent, supportSpecialAliases, supportNumericSeasonFolders)
	if n == nil {
		return SeasonPath{}
	}
	return SeasonPath{Success: true, SeasonNumber: n, IsSeasonFolder: isSeason}
}

func seasonNumberFromPath(path, parent string, hasParent, specialAliases, numericFolders bool) (*int, bool) {
	name := fileName(path)
	if m := seasonPrefix.match(name); m != nil {
		s, _ := m.groupN(1)
		if n, ok := parseInt(s); ok {
			return &n, true
		}
	}
	cleaned := seasonCleanName.replaceAll(name, "")
	if hasParent {
		cleaned = replaceFold(cleaned, seasonCleanName.replaceAll(parent, ""), "")
	}
	if specialAliases && (strings.EqualFold(cleaned, "specials") || strings.EqualFold(cleaned, "extras")) {
		return ptr(0), true
	}
	if numericFolders {
		if n, ok := parseInt(cleaned); ok {
			return &n, true
		}
	}
	mixed := !numericFolders && !specialAliases
	if m := seasonPre.match(cleaned); m != nil {
		if mixed && !seasonKeyword.isMatch(name) {
			return nil, false
		}
		return seasonMatch(m)
	}
	m := seasonPost.match(cleaned)
	if m == nil {
		return nil, false
	}
	if mixed && !seasonKeyword.isMatch(name) {
		return nil, false
	}
	return seasonMatch(m)
}

func seasonMatch(m *match) (*int, bool) {
	if s, ok := m.group("seasonnumber"); ok {
		if n, ok := parseInt(s); ok {
			return &n, true
		}
	}
	return nil, false
}

// ParseSeriesPath finds the series name in a path with the non-optimistic
// episode expressions.
func (p *Parser) ParseSeriesPath(path string) (string, bool) {
	for _, e := range p.episodes {
		// Optimistic expressions misread release folder names such as
		// "Silo.S03.1080p.WEB-DL…" ("264" as S02E64).
		if e.Optimistic {
			continue
		}
		if name, ok := parseSeries(path, e); ok {
			return strings.Trim(name, " _.-"), true
		}
	}
	return "", false
}

func parseSeries(name string, e compiledEpisodeExpression) (string, bool) {
	m := e.re.match(name)
	if m == nil || m.groupCount() < 3 || !e.Named {
		return "", false
	}
	season, hasSeason := m.group("seasonnumber")
	// Reject implausible seasons such as a 1280x720 resolution.
	if hasSeason {
		if n, ok := parseInt(season); ok && ((n >= 200 && n < 1928) || n > 2500) {
			return "", false
		}
	}
	series, _ := m.group("seriesname")
	return series, series != "" && season != ""
}

var (
	seriesNameSeparators = mustCompile(`(?<=[^\s\._]{2})[\._]+|[\._]+(?=[^\s\._]{2})`, false)
	seriesTitleWithYear  = mustCompile(`(?<title>.+?)\s*\((?<year>[0-9]{4})\)`, false)
)

// Series is a resolved series folder.
type Series struct {
	Path string
	Name string
	Year *int
}

// ResolveSeries derives the series name and year from its folder path.
func (p *Parser) ResolveSeries(path string) Series {
	name := fileName(path)
	// A title with a year handles numeric titles too.
	if name != "" {
		if m := seriesTitleWithYear.match(name); m != nil {
			title, _ := m.group("title")
			s := Series{Path: path, Name: strings.TrimSpace(title)}
			if y, _ := m.group("year"); y != "" {
				if n, ok := parseInt(y); ok {
					s.Year = &n
				}
			}
			return s
		}
	}
	if series, ok := p.ParseSeriesPath(path); ok && series != "" {
		name = series
	}
	if name != "" {
		name = strings.TrimSpace(seriesNameSeparators.replaceAll(name, " "))
	}
	return Series{Path: path, Name: name}
}

// ParseSeriesStatus parses an airing status from metadata, accepting the
// status names and the synonyms "Pilot", "Returning Series", "Returning",
// "Cancelled" and "Canceled".
func ParseSeriesStatus(s string) (core.SeriesStatus, bool) {
	t := strings.TrimFunc(s, unicode.IsSpace)
	for _, st := range []core.SeriesStatus{core.SeriesContinuing, core.SeriesEnded, core.SeriesUnreleased} {
		if strings.EqualFold(t, string(st)) {
			return st, true
		}
	}
	switch {
	case containsFold([]string{"Pilot", "Returning Series", "Returning"}, s):
		return core.SeriesContinuing, true
	case containsFold([]string{"Cancelled", "Canceled"}, s):
		return core.SeriesEnded, true
	}
	return "", false
}

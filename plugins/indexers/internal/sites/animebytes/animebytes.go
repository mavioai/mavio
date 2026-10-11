package animebytes

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var links = []string{"https://animebytes.tv/"}

// Definition is AnimeBytes.
var Definition = indexer.Definition{
	ID:          "animebytes",
	Name:        "AnimeBytes",
	Description: "AnimeBytes (AB) is the largest private torrent tracker that specialises in anime and anime-related content.",
	Language:    "en-US",
	Protocol:    indexer.ProtocolTorrent,
	Privacy:     indexer.Private,
	Links:       links,
	Fields: []indexer.Field{
		indexer.BaseURLField(links),
		{Name: "username", Label: "Username", Type: indexer.FieldText, HelpText: "Site Username"},
		{Name: "passkey", Label: "Passkey", Type: indexer.FieldText, Secret: true, HelpText: "Site Passkey"},
		{Name: "freeleech_only", Label: "Freeleech Only", Type: indexer.FieldCheckbox, HelpText: "Search freeleech torrents only"},
		{Name: "exclude_raw", Label: "Exclude RAW", Type: indexer.FieldCheckbox, HelpText: "Exclude RAW torrents from results"},
		{Name: "exclude_hentai", Label: "Exclude Hentai", Type: indexer.FieldCheckbox, HelpText: "Exclude Hentai torrents from results"},
		{Name: "search_by_year", Label: "Search By Year", Type: indexer.FieldCheckbox, HelpText: "Makes Prowlarr to search by year as a different argument in the request."},
		{
			Name: "enable_sonarr_compatibility", Label: "Enable Sonarr Compatibility", Type: indexer.FieldCheckbox, Default: true,
			HelpText: "Makes Prowlarr try to add Season information into Release names, without this Sonarr can't match any Seasons, but it has a lot of false positives as well",
		},
		{
			Name: "use_filename_for_single_episodes", Label: "Use Filenames for Single Episodes", Type: indexer.FieldCheckbox, Default: true,
			HelpText: "Add a release using the actual filename, this currently only works for single episode releases",
		},
		{
			Name: "add_japanese_title", Label: "Add Japanese title as a synonym", Type: indexer.FieldCheckbox, Default: true,
			HelpText: "Makes Prowlarr add Japanese titles as synonyms, i.e kanji/hiragana/katakana.",
		},
		{
			Name: "add_romaji_title", Label: "Add Romaji title as a synonym", Type: indexer.FieldCheckbox, Default: true,
			HelpText: `Makes Prowlarr add Romaji title as a synonym, i.e "Shingeki no Kyojin" with Attack on Titan`,
		},
		{
			Name: "add_alternative_title", Label: "Add alternative title as a synonym", Type: indexer.FieldCheckbox, Default: true,
			HelpText: `Makes Prowlarr add alternative title as a synonym, i.e "AoT" with Attack on Titan, but also "Attack on Titan Season 4" Instead of "Attack on Titan: The Final Season"`,
		},
	},
	New: newSite,
}

// Settings are AnimeBytes' settings. The booleans defaulting to true are
// pointers, true when unset.
type Settings struct {
	indexer.BaseSettings
	Username                     string `json:"username,omitempty"`
	Passkey                      string `json:"passkey,omitempty"`
	FreeleechOnly                bool   `json:"freeleech_only,omitempty"`
	ExcludeRaw                   bool   `json:"exclude_raw,omitempty"`
	ExcludeHentai                bool   `json:"exclude_hentai,omitempty"`
	SearchByYear                 bool   `json:"search_by_year,omitempty"`
	EnableSonarrCompatibility    *bool  `json:"enable_sonarr_compatibility,omitempty"`
	UseFilenameForSingleEpisodes *bool  `json:"use_filename_for_single_episodes,omitempty"`
	AddJapaneseTitle             *bool  `json:"add_japanese_title,omitempty"`
	AddRomajiTitle               *bool  `json:"add_romaji_title,omitempty"`
	AddAlternativeTitle          *bool  `json:"add_alternative_title,omitempty"`
}

// Validate checks that the user name is set and the passkey is 32, 48 or
// 56 characters long.
func (s Settings) Validate() error {
	var errs []error
	if strings.TrimSpace(s.Username) == "" {
		errs = append(errs, errors.New("username: must not be empty"))
	}
	switch {
	case strings.TrimSpace(s.Passkey) == "":
		errs = append(errs, errors.New("passkey: must not be empty"))
	case !slices.Contains([]int{32, 48, 56}, utf8.RuneCountInString(s.Passkey)):
		errs = append(errs, errors.New("passkey: Passkey length must be 32, 48 or 56"))
	}
	return errors.Join(errs...)
}

func orTrue(b *bool) bool { return b == nil || *b }

// Site is AnimeBytes.
type Site struct {
	settings Settings
	baseURL  string
	caps     *indexer.Capabilities
	// now is the clock; time.Now when nil.
	now func() time.Time
}

func newSite(data []byte) (indexer.Site, error) {
	var settings Settings
	if err := indexer.DecodeSettings(data, &settings); err != nil {
		return nil, err
	}
	if err := settings.Validate(); err != nil {
		return nil, err
	}
	return &Site{settings: settings, baseURL: settings.URL(links, nil), caps: capabilities()}, nil
}

func capabilities() *indexer.Capabilities {
	c := indexer.NewCapabilities()
	c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
	c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ}
	c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ, indexer.ParamArtist, indexer.ParamAlbum, indexer.ParamYear}
	c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
	c.Categories.Add("anime[tv_series]", indexer.TVAnime, "TV Series")
	c.Categories.Add("anime[tv_special]", indexer.TVAnime, "TV Special")
	c.Categories.Add("anime[ova]", indexer.TVAnime, "OVA")
	c.Categories.Add("anime[ona]", indexer.TVAnime, "ONA")
	c.Categories.Add("anime[dvd_special]", indexer.TVAnime, "DVD Special")
	c.Categories.Add("anime[bd_special]", indexer.TVAnime, "BD Special")
	c.Categories.Add("anime[movie]", indexer.Movies, "Movie")
	c.Categories.Add("audio", indexer.Audio, "Music")
	c.Categories.Add("gamec[game]", indexer.Console, "Game")
	c.Categories.Add("gamec[game]", indexer.PCGames, "Game")
	c.Categories.Add("gamec[visual_novel]", indexer.Console, "Game Visual Novel")
	c.Categories.Add("gamec[visual_novel]", indexer.PCGames, "Game Visual Novel")
	c.Categories.Add("printedtype[manga]", indexer.BooksComics, "Manga")
	c.Categories.Add("printedtype[oneshot]", indexer.BooksComics, "Oneshot")
	c.Categories.Add("printedtype[anthology]", indexer.BooksComics, "Anthology")
	c.Categories.Add("printedtype[manhwa]", indexer.BooksComics, "Manhwa")
	c.Categories.Add("printedtype[light_novel]", indexer.BooksComics, "Light Novel")
	c.Categories.Add("printedtype[artbook]", indexer.BooksComics, "Artbook")
	return c
}

// Info implements indexer.Site.
func (s *Site) Info() indexer.Info {
	return indexer.Info{
		Protocol:       indexer.ProtocolTorrent,
		Privacy:        indexer.Private,
		SupportsRSS:    true,
		SupportsSearch: true,
		RateLimit:      4 * time.Second,
	}
}

// PageCacheLife implements indexer.PageCacher: results are kept for three
// minutes, as Prowlarr's AnimeBytes.FetchPage keeps them.
func (s *Site) PageCacheLife() time.Duration { return 3 * time.Minute }

// Capabilities implements indexer.Site.
func (s *Site) Capabilities(context.Context, *indexer.Indexer) (*indexer.Capabilities, error) {
	return s.caps, nil
}

func mustCompile(expr string, opts regexp2.RegexOptions) *regexp2.Regexp {
	return regexp2.MustCompile(expr, opts)
}

var (
	yearInTerm      = mustCompile(`\b((?:19|20)\d{2})$`, regexp2.None)
	episodeX        = mustCompile(`\W(\dx)?\d?\d$`, regexp2.None)
	episodeSE       = mustCompile(`\W(S\d\d?E)?\d?\d$`, regexp2.None)
	trailingNumber  = mustCompile(`\W\d+$`, regexp2.None)
	theMovie        = mustCompile(`\bThe Movie$`, regexp2.IgnoreCase)
	simpleSeason    = mustCompile(`\bSeason (\d+)\b`, regexp2.None)
	episodeEdition  = mustCompile(`\bEpisode (\d+)\b`, regexp2.None)
	advancedSeason  = mustCompile(`\b(?:(?<season>\d+)(?:st|nd|rd|th) Season|Season (?<season>\d+))\b`, regexp2.IgnoreCase)
	seasonRoman     = mustCompile(`(I{2,})$`, regexp2.None)
	seasonNumber    = mustCompile(`\b(?<!(Part|No\.)[- ._])(?<!\d[/])(?<!\#)(?:S)?(?<season>[2-9])$`, regexp2.None)
	h265            = mustCompile(`\b(H\.?265)\b`, regexp2.IgnoreCase)
	h264            = mustCompile(`\b(H\.?264)\b`, regexp2.IgnoreCase)
	propertiesSplit = []string{" | ", " / "}
)

// replace replaces all matches of re in s.
func replace(re *regexp2.Regexp, s, repl string) string {
	out, err := re.Replace(s, repl, -1, -1)
	if err != nil {
		return s
	}
	return out
}

// find returns the match of re in s, nil for none.
func find(re *regexp2.Regexp, s string) *regexp2.Match {
	m, err := re.FindStringMatch(s)
	if err != nil {
		return nil
	}
	return m
}

// SearchRequests implements indexer.Site, as Prowlarr's
// AnimeBytesRequestGenerator does: music searches search music, all
// others anime.
func (s *Site) SearchRequests(_ context.Context, _ *indexer.Indexer, q *indexer.Query) (*indexer.Chain, error) {
	searchType := "anime"
	if q.Type == indexer.SearchMusic {
		searchType = "music"
	}
	term := strings.TrimSpace(q.SanitizedTerm())
	searchTerm := cleanSearchTerm(term)
	var p indexer.Params
	p.Set("username", s.settings.Username)
	p.Set("torrent_pass", s.settings.Passkey)
	p.Set("sort", "grouptime")
	p.Set("way", "desc")
	p.Set("type", searchType)
	p.Set("searchstr", searchTerm)
	if strings.TrimSpace(searchTerm) != "" {
		p.Set("limit", "50")
	} else {
		p.Set("limit", "20")
	}
	if s.settings.SearchByYear && searchType == "anime" {
		if m := find(yearInTerm, term); m != nil {
			if y, err := parse.Int(m.GroupByNumber(1).String()); err == nil && y > 0 {
				p.Set("year", strconv.FormatInt(y, 10))
			}
		}
	}
	if q.Type == indexer.SearchMusic {
		if strings.TrimSpace(q.Artist) != "" && q.Artist != "VA" {
			p.Set("artistnames", q.Artist)
		}
		if strings.TrimSpace(q.Album) != "" {
			p.Set("groupname", q.Album)
		}
		if q.Year > 0 {
			p.Set("year", strconv.Itoa(q.Year))
		}
	}
	cats := s.caps.Categories.MapToTracker(q.Categories, false)
	if len(cats) > 0 && q.Type == indexer.SearchTV && q.Season != nil && *q.Season > 0 {
		// Specials are not searched for in seasons other than the
		// specials.
		cats = slices.DeleteFunc(cats, func(c string) bool {
			return c == "anime[tv_special]" || c == "anime[ova]" || c == "anime[dvd_special]" || c == "anime[bd_special]"
		})
	}
	for _, c := range cats {
		p.Set(c, "1")
	}
	if s.settings.FreeleechOnly {
		p.Set("freeleech", "1")
	}
	if s.settings.ExcludeHentai && searchType == "anime" {
		p.Set("hentai", "0")
	}
	var c indexer.Chain
	r := indexer.NewRequest(strings.TrimRight(s.baseURL, "/")+"/scrape.php?"+p.Encode(), indexer.AcceptJSON)
	// Prowlarr's IndexerRequest of a URL follows redirects.
	r.FollowRedirects = true
	c.AddRequests(r)
	return &c, nil
}

// cleanSearchTerm strips episode numbers, which the site does not search
// by, and "The Movie".
func cleanSearchTerm(term string) string {
	term = replace(episodeX, term, "")
	term = replace(episodeSE, term, "")
	term = replace(trailingNumber, term, "")
	term = replace(theMovie, strings.TrimSpace(term), "")
	return strings.TrimSpace(term)
}

// CleanupReleases implements indexer.ReleaseCleaner: recent releases are
// those of the last day, or the first 20.
func (s *Site) CleanupReleases(releases []*indexer.Release, q *indexer.Query) []*indexer.Release {
	if !q.IsRSS() {
		return releases
	}
	now := time.Now
	if s.now != nil {
		now = s.now
	}
	since := now().UTC().AddDate(0, 0, -1)
	out := releases[:0]
	for i, r := range releases {
		if r.PublishDate.After(since) || i < 20 {
			out = append(out, r)
		}
	}
	return out
}

type response struct {
	Matches jsonx.Int    `json:"Matches"`
	Groups  []group      `json:"Groups"`
	Error   jsonx.String `json:"Error"`
}

type group struct {
	CategoryName jsonx.String      `json:"CategoryName"`
	FullName     jsonx.String      `json:"FullName"`
	GroupName    jsonx.String      `json:"GroupName"`
	SeriesName   jsonx.String      `json:"SeriesName"`
	Year         *jsonx.Int        `json:"Year"`
	Synonyms     map[string]string `json:"SynonymnsV2"`
	Description  jsonx.String      `json:"Description"`
	Torrents     []torrent         `json:"Torrents"`
}

type file struct {
	FileName jsonx.String `json:"filename"`
}

type torrent struct {
	ID          jsonx.Int64 `json:"ID"`
	EditionData *struct {
		EditionTitle jsonx.String `json:"EditionTitle"`
	} `json:"EditionData"`
	RawDownMultiplier jsonx.Float  `json:"RawDownMultiplier"`
	RawUpMultiplier   jsonx.Float  `json:"RawUpMultiplier"`
	Link              jsonx.String `json:"Link"`
	Property          jsonx.String `json:"Property"`
	Snatched          jsonx.Int    `json:"Snatched"`
	Seeders           jsonx.Int    `json:"Seeders"`
	Leechers          jsonx.Int    `json:"Leechers"`
	Size              jsonx.Int64  `json:"Size"`
	FileCount         jsonx.Int    `json:"FileCount"`
	Files             []file       `json:"FileList"`
	UploadTime        jsonx.String `json:"UploadTime"`
}

var (
	remuxResolutions     = []string{"1080i", "1080p", "2160p", "4K"}
	releaseGroupProps    = []string{"Softsubs", "Hardsubs", "RAW", "Translated"}
	excludedFileExts     = []string{".mka", ".mds", ".md5", ".nfo", ".sfv", ".ass", ".mks", ".srt", ".ssa", ".sup", ".jpeg", ".jpg", ".png", ".otf", ".ttf"}
	excludedPropertyPart = "Freeleech"
)

// ParseResponse implements indexer.Site, as Prowlarr's AnimeBytesParser
// does: a release for each title of a group's torrents, and one named by
// the file of single file torrents.
func (s *Site) ParseResponse(_ context.Context, _ *indexer.Indexer, r *indexer.Response) ([]*indexer.Release, error) {
	if r.StatusCode != http.StatusOK {
		return nil, indexer.Errorf("Unexpected response status %d code from indexer request", r.StatusCode)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return nil, indexer.Errorf("Unexpected response header %s from indexer request, expected %s", r.Header.Get("Content-Type"), indexer.AcceptJSON)
	}
	var resp response
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("animebytes: %w", err)
	}
	if strings.TrimSpace(string(resp.Error)) != "" {
		return nil, indexer.Errorf("Unexpected response from indexer request: %s", resp.Error)
	}
	if resp.Matches == 0 {
		return nil, nil
	}
	var releases []*indexer.Release
	for i := range resp.Groups {
		rels, err := s.groupReleases(&resp.Groups[i])
		if err != nil {
			return nil, err
		}
		releases = append(releases, rels...)
	}
	slices.SortStableFunc(releases, func(a, b *indexer.Release) int { return b.PublishDate.Compare(a.PublishDate) })
	return releases, nil
}

// synonyms returns a group's titles, unique ignoring case.
func (s *Site) synonyms(g *group) []string {
	main := html.UnescapeString(string(g.FullName))
	if strings.TrimSpace(string(g.SeriesName)) != "" {
		main = string(g.SeriesName)
	}
	titles := []string{main}
	add := func(t string) {
		if !slices.ContainsFunc(titles, func(v string) bool { return strings.EqualFold(v, t) }) {
			titles = append(titles, t)
		}
	}
	if len(g.Synonyms) > 0 {
		if v := g.Synonyms["Japanese"]; orTrue(s.settings.AddJapaneseTitle) && strings.TrimSpace(v) != "" {
			add(strings.TrimSpace(v))
		}
		if v := g.Synonyms["Romaji"]; orTrue(s.settings.AddRomajiTitle) && strings.TrimSpace(v) != "" {
			add(strings.TrimSpace(v))
		}
		if v := g.Synonyms["Alternative"]; orTrue(s.settings.AddAlternativeTitle) && strings.TrimSpace(v) != "" {
			for a := range strings.SplitSeq(v, ",") {
				if a = strings.TrimSpace(a); a != "" {
					add(a)
				}
			}
		}
	}
	return titles
}

func (s *Site) groupReleases(g *group) ([]*indexer.Release, error) {
	categoryName, groupName := string(g.CategoryName), string(g.GroupName)
	year := 0
	if g.Year != nil {
		year = int(*g.Year)
	}
	synonyms := s.synonyms(g)
	sonarr := orTrue(s.settings.EnableSonarrCompatibility)
	var categories []int
	var releases []*indexer.Release
	for _, t := range g.Torrents {
		if s.settings.FreeleechOnly && t.RawDownMultiplier != 0 {
			continue
		}
		published, err := time.ParseInLocation(time.DateTime, string(t.UploadTime), time.UTC)
		if err != nil {
			return nil, fmt.Errorf("animebytes: %w", err)
		}
		details := s.baseURL + "torrent/" + strconv.FormatInt(int64(t.ID), 10) + "/group"
		size := int64(t.Size)
		// 72 hours, and 5 more per GiB.
		minimumSeedTime := int64(259200 + int32(size/(1<<30)*18000))
		properties := torrentProperties(&t)
		bluRayDisk := slices.ContainsFunc(properties, func(p string) bool {
			return p == "RAW" || strings.HasPrefix(p, "M2TS") || strings.HasPrefix(p, "ISO")
		})
		if s.settings.ExcludeRaw && bluRayDisk {
			continue
		}
		remux := slices.ContainsFunc(t.Files, func(f file) bool {
			return strings.Contains(strings.ToLower(string(f.FileName)), "remux")
		})
		var props []string
		for _, p := range properties {
			if bluRayDisk {
				p = replace(h265, p, "HEVC")
				p = replace(h264, p, "AVC")
			}
			if remux && slices.ContainsFunc(remuxResolutions, func(r string) bool { return strings.EqualFold(r, p) }) {
				p += " Remux"
			}
			if !slices.Contains(props, p) {
				props = append(props, p)
			}
		}
		properties = props

		var season, episode int
		hasSeason := false
		releaseInfo := ""
		if sonarr && categoryName == "Anime" {
			releaseInfo = "S01"
		}
		if t.EditionData != nil && strings.TrimSpace(string(t.EditionData.EditionTitle)) != "" {
			releaseInfo = html.UnescapeString(string(t.EditionData.EditionTitle))
			if sonarr {
				if m := find(simpleSeason, releaseInfo); m != nil {
					season, hasSeason = coerceInt(m.GroupByNumber(1).String()), true
				}
			}
			if m := find(episodeEdition, releaseInfo); m != nil {
				episode = coerceInt(m.GroupByNumber(1).String())
			}
		}
		if sonarr && categoryName == "Anime" && !hasSeason {
			season, hasSeason = seasonFromTitles(synonyms)
		}
		if episode > 0 && !hasSeason {
			releaseInfo = fmt.Sprintf(" - %02d", episode)
		} else if sonarr && season > 0 {
			releaseInfo = fmt.Sprintf("S%02d", season)
			if episode > 0 {
				releaseInfo += fmt.Sprintf("E%02d - %02d", episode, episode)
			}
		}
		releaseInfo = strings.TrimSpace(releaseInfo)

		// Specials confuse matching releases with episodes.
		switch groupName {
		case "TV Special", "DVD Special", "BD Special":
			continue
		}
		if c := categoriesOf(categoryName, groupName, properties); c != nil {
			categories = c
		}

		releaseGroup := ""
		if i := lastIndexFunc(properties, func(p string) bool {
			return slices.ContainsFunc(releaseGroupProps, func(g string) bool { return hasPrefixFold(p, g) }) &&
				strings.Contains(p, "(") && strings.Contains(p, ")")
		}); i >= 0 {
			p := properties[i]
			start, end := strings.Index(p, "("), strings.Index(p, ")")
			if end <= start {
				return nil, fmt.Errorf("animebytes: release group of property %q", p)
			}
			releaseGroup = "[" + p[start+1:end] + "] "
		}
		var infoString strings.Builder
		for _, p := range properties {
			infoString.WriteString("[" + p + "]")
		}

		release := func(title, guid string) *indexer.Release {
			return &indexer.Release{
				MinimumRatio:         indexer.Ptr(1.0),
				MinimumSeedTime:      indexer.Ptr(minimumSeedTime),
				Title:                title,
				Year:                 year,
				InfoURL:              details,
				GUID:                 guid,
				DownloadURL:          string(t.Link),
				PublishDate:          published,
				Categories:           categories,
				Description:          string(g.Description),
				Size:                 indexer.Ptr(size),
				Seeders:              indexer.Ptr(int(t.Seeders)),
				Peers:                indexer.Ptr(int(t.Seeders + t.Leechers)),
				Grabs:                indexer.Ptr(int(t.Snatched)),
				Files:                indexer.Ptr(int(t.FileCount)),
				DownloadVolumeFactor: indexer.Ptr(float64(t.RawDownMultiplier)),
				UploadVolumeFactor:   indexer.Ptr(float64(t.RawUpMultiplier)),
			}
		}

		if orTrue(s.settings.UseFilenameForSingleEpisodes) {
			var files []string
			for _, f := range t.Files {
				files = append(files, string(f.FileName))
			}
			if len(files) > 1 {
				files = slices.DeleteFunc(files, func(f string) bool {
					return slices.ContainsFunc(excludedFileExts, func(e string) bool { return strings.EqualFold(e, extension(f)) })
				})
			}
			if len(files) == 1 {
				releases = append(releases, release(files[0], details+"?nh="+md5Hex(files[0])))
			}
		}

		yearText := ""
		if g.Year != nil {
			yearText = strconv.Itoa(year)
		}
		useYearInTitle := year > 0 && slices.ContainsFunc(t.Files, func(f file) bool {
			return strings.Contains(string(f.FileName), strconv.Itoa(year))
		})
		for _, title := range synonyms {
			var name string
			if groupName == "Movie" || groupName == "Live Action Movie" {
				name = fmt.Sprintf("%s%s %s %s", releaseGroup, title, yearText, infoString.String())
			} else {
				y := ""
				if useYearInTitle {
					y = " " + yearText
				}
				name = fmt.Sprintf("%s%s%s %s %s", releaseGroup, title, y, releaseInfo, infoString.String())
			}
			releases = append(releases, release(strings.TrimSpace(name), details+"?nh="+md5Hex(title)))
		}
	}
	return releases, nil
}

// torrentProperties returns a torrent's properties without freeleech,
// unique, with BR-DISK for M2TS ones.
func torrentProperties(t *torrent) []string {
	var out []string
	for _, p := range splitAny(html.UnescapeString(string(t.Property)), propertiesSplit) {
		p = strings.TrimSpace(p)
		if p == "" || strings.Contains(strings.ToLower(p), strings.ToLower(excludedPropertyPart)) || slices.Contains(out, p) {
			continue
		}
		out = append(out, p)
	}
	if slices.ContainsFunc(out, func(p string) bool { return strings.HasPrefix(p, "M2TS") }) && !slices.Contains(out, "BR-DISK") {
		out = append(out, "BR-DISK")
	}
	return out
}

// splitAny splits s at any of the separators, as .NET's String.Split
// does: left to right, the first separator matching at a position
// winning.
func splitAny(s string, seps []string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); {
		matched := false
		for _, sep := range seps {
			if strings.HasPrefix(s[i:], sep) {
				out = append(out, s[start:i])
				i += len(sep)
				start = i
				matched = true
				break
			}
		}
		if !matched {
			i++
		}
	}
	return append(out, s[start:])
}

// categoriesOf returns the categories of a torrent by its group's
// category and name and its properties; nil to keep the group's last.
func categoriesOf(categoryName, groupName string, properties []string) []int {
	var c []*indexer.Category
	switch groupName {
	case "TV Series", "OVA", "ONA":
		c = []*indexer.Category{indexer.TVAnime}
	case "Movie", "Live Action Movie":
		c = []*indexer.Category{indexer.Movies}
	}
	switch categoryName {
	case "Manga", "Oneshot", "Anthology", "Manhwa", "Manhua", "Light Novel", "Novel", "Artbook":
		c = []*indexer.Category{indexer.BooksComics}
	case "Game", "Visual Novel":
		has := func(p ...string) bool {
			return slices.ContainsFunc(p, func(v string) bool { return slices.Contains(properties, v) })
		}
		if has("PSP") {
			c = []*indexer.Category{indexer.Console, indexer.ConsolePSP}
		}
		if has("PS3") {
			c = []*indexer.Category{indexer.Console, indexer.ConsolePS3}
		}
		if has("PS Vita") {
			c = []*indexer.Category{indexer.Console, indexer.ConsolePSVita}
		}
		if has("3DS") {
			c = []*indexer.Category{indexer.Console, indexer.Console3DS}
		}
		if has("NDS") {
			c = []*indexer.Category{indexer.Console, indexer.ConsoleNDS}
		}
		if has("PSX", "PS2", "SNES", "NES", "GBA", "Switch", "N64") {
			c = []*indexer.Category{indexer.Console, indexer.ConsoleOther}
		}
		if has("PC") {
			c = []*indexer.Category{indexer.PCGames}
		}
	case "Single", "EP", "Album", "Compilation", "Soundtrack", "Remix CD", "PV", "Live Album", "Image CD", "Drama CD", "Vocal CD":
		switch {
		case slices.ContainsFunc(properties, func(p string) bool { return strings.Contains(p, "Lossless") }):
			c = []*indexer.Category{indexer.Audio, indexer.AudioLossless}
		case slices.ContainsFunc(properties, func(p string) bool { return strings.Contains(p, "MP3") }):
			c = []*indexer.Category{indexer.Audio, indexer.AudioMP3}
		default:
			c = []*indexer.Category{indexer.Audio, indexer.AudioOther}
		}
	}
	if c == nil {
		return nil
	}
	return indexer.IDs(c)
}

// seasonFromTitles returns the season the first title naming one names.
func seasonFromTitles(titles []string) (int, bool) {
	for _, t := range titles {
		if m := find(advancedSeason, t); m != nil {
			return coerceInt(m.GroupByName("season").String()), true
		}
		if m := find(seasonRoman, t); m != nil {
			return utf8.RuneCountInString(m.GroupByNumber(1).String()), true
		}
		if m := find(seasonNumber, t); m != nil {
			return coerceInt(m.GroupByName("season").String()), true
		}
	}
	return 0, false
}

func coerceInt(s string) int {
	n, _ := parse.Int(s)
	return int(n)
}

func lastIndexFunc(s []string, f func(string) bool) int {
	for i := len(s) - 1; i >= 0; i-- {
		if f(s[i]) {
			return i
		}
	}
	return -1
}

func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// extension returns a file name's extension as .NET's Path.GetExtension
// does: from its last dot, "" when the name ends with it.
func extension(name string) string {
	i := strings.LastIndexAny(name, `./`)
	if i < 0 || name[i] != '.' || i == len(name)-1 {
		return ""
	}
	return name[i:]
}

// md5Hex returns the upper case MD5 of text encoded as .NET's
// Encoding.ASCII encodes it: '?' for each UTF-16 unit beyond ASCII.
func md5Hex(s string) string {
	b := make([]byte, 0, len(s))
	for _, r := range s {
		switch {
		case r < utf8.RuneSelf:
			b = append(b, byte(r))
		case r > 0xFFFF:
			b = append(b, '?', '?')
		default:
			b = append(b, '?')
		}
	}
	sum := md5.Sum(b)
	return strings.ToUpper(hex.EncodeToString(sum[:]))
}

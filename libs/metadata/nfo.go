package metadata

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// NFOOptions configures ParseNFO.
type NFOOptions struct {
	// ReleaseDateFormat is the layout of release dates in .NET notation;
	// empty means "yyyy-MM-dd".
	ReleaseDateFormat string
	// FileExists reports whether a local image file exists; images it
	// rejects are dropped. Nil drops all local images.
	FileExists func(path string) bool
}

// YouTubeWatchURL is the URL trailers on YouTube are stored as.
const YouTubeWatchURL = "https://www.youtube.com/watch?v="

// ErrEmptyNFO is returned for an empty NFO file.
var ErrEmptyNFO = errors.New("empty NFO file")

// ParseNFO reads an NFO file of an item of the given kind. Movie, music
// video and series files may also be, or end with, plain provider URLs,
// from which IMDb, TMDB and TVDB IDs are taken. An episode file may
// describe several episodes of a multi-episode file.
func ParseNFO(data []byte, kind core.ItemKind, o NFOOptions) (*Result, error) {
	if !kind.Valid() {
		return nil, fmt.Errorf("%w: unknown item kind %q", core.ErrInvalid, kind)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, ErrEmptyNFO
	}
	p := &nfoParser{kind: kind, opts: o}
	if p.opts.ReleaseDateFormat == "" {
		p.opts.ReleaseDateFormat = "yyyy-MM-dd"
	}
	r := &Result{Item: core.Item{Kind: kind}}
	switch kind {
	case core.KindEpisode:
		return r, p.parseEpisodes(string(data), r)
	case core.KindMovie, core.KindVideo, core.KindMusicVideo, core.KindSeries:
		return r, p.parseWithURLs(string(data), r)
	}
	root, err := parseXML(data)
	if err != nil {
		return nil, err
	}
	p.apply(root, r)
	return r, nil
}

type nfoParser struct {
	kind core.ItemKind
	opts NFOOptions
}

func (p *nfoParser) apply(root *node, r *Result) {
	for _, n := range root.children {
		p.element(n, r)
	}
}

// parseWithURLs handles provider URLs after the closing tag, or a file of
// URLs only.
func (p *nfoParser) parseWithURLs(text string, r *Result) error {
	// The XML ends at the last closing tag; anything after it may be URLs.
	end := strings.LastIndex(text, "</")
	if end >= 0 {
		if i := strings.IndexByte(text[end:], '>'); i >= 0 {
			end += i
		} else {
			end = -1
		}
	}
	if end < 0 {
		p.providerLinks(text, &r.Item)
		return nil
	}
	p.providerLinks(text[end:], &r.Item)
	if end == 0 {
		return nil
	}
	root, err := parseXML([]byte(text[:end+1]))
	if err != nil {
		return nil // not XML after all; the links were all there is
	}
	p.apply(root, r)
	return nil
}

func (p *nfoParser) providerLinks(text string, it *core.Item) {
	if id, ok := FindIMDbID(text); ok {
		setID(it, core.ProviderIMDb, id)
	}
	switch p.kind {
	case core.KindMovie, core.KindVideo, core.KindMusicVideo:
		if id, ok := FindTMDBMovieID(text); ok {
			setID(it, core.ProviderTMDB, id)
		}
	case core.KindSeries:
		if id, ok := FindTMDBSeriesID(text); ok {
			setID(it, core.ProviderTMDB, id)
		}
		if id, ok := FindTVDBID(text); ok {
			setID(it, core.ProviderTVDB, id)
		}
	}
}

// parseEpisodes reads one <episodedetails> block per episode. With several
// blocks the item is the episode with the lowest number, and the others'
// names, original titles and overviews are appended, separated by " / ".
func (p *nfoParser) parseEpisodes(text string, r *Result) error {
	const end = "</episodedetails>"
	var blocks []string
	for {
		i := indexFold(text, end)
		if i < 0 {
			break
		}
		blocks = append(blocks, text[:i+len(end)])
		text = text[i+len(end):]
	}
	if len(blocks) == 0 {
		blocks = []string{text}
	}
	parse := func(block string, r *Result) bool {
		root, err := parseXML([]byte(block))
		if err != nil {
			return false
		}
		p.apply(root, r)
		return true
	}
	if len(blocks) == 1 {
		parse(blocks[0], r)
		return nil
	}
	type episode struct {
		block string
		r     *Result
	}
	var episodes []episode
	for _, b := range blocks {
		e := episode{b, &Result{Item: core.Item{Kind: p.kind}}}
		if parse(b, e.r) {
			episodes = append(episodes, e)
		}
	}
	if len(episodes) == 0 {
		return nil
	}
	number := func(e episode) int {
		if n := e.r.Item.IndexNumber; n != nil {
			return *n
		}
		return int(^uint(0) >> 1)
	}
	slices.SortStableFunc(episodes, func(a, b episode) int { return cmp.Compare(number(a), number(b)) })
	parse(episodes[0].block, r)
	it := &r.Item
	for _, e := range episodes[1:] {
		other := e.r.Item
		if other.Name != "" {
			it.Name += " / " + other.Name
		}
		if other.Overview != "" {
			it.Overview += " / " + other.Overview
		}
		if other.OriginalTitle != "" {
			it.OriginalTitle += " / " + other.OriginalTitle
		}
		if n := other.IndexNumber; n != nil {
			end := *n
			if it.IndexNumberEnd != nil {
				end = max(end, *it.IndexNumberEnd)
			}
			it.IndexNumberEnd = &end
		}
	}
	return nil
}

// element interprets one child of the root element.
func (p *nfoParser) element(n *node, r *Result) {
	if p.kindElement(n, r) {
		return
	}
	it := &r.Item
	switch n.name {
	case "dateadded":
		if t, ok := parseDateTime(n.value()); ok {
			it.DateAdded = t
		}
	case "originaltitle":
		it.OriginalTitle = n.value()
	case "name", "title", "localtitle":
		it.Name = n.value()
	case "sortname", "sorttitle":
		it.SortName = n.value()
	case "criticrating":
		if v, err := strconv.ParseFloat(n.value(), 64); err == nil {
			it.CriticRating = v
		}
	case "biography", "plot", "review":
		it.Overview = n.value()
	case "language":
		it.MetadataLanguage = n.value()
	case "countrycode":
		it.MetadataCountry = n.value()
	case "watched":
		if b, err := strconv.ParseBool(n.value()); err == nil {
			r.UserData.Played = &b
		}
	case "playcount":
		if v, ok := parseInt(n.value()); ok {
			r.UserData.PlayCount = &v
		}
	case "lastplayed":
		if t, ok := parseDateTime(n.value()); ok {
			r.UserData.LastPlayedAt = &t
		}
	case "lockedfields":
		for f := range strings.SplitSeq(n.value(), "|") {
			if field, ok := lockableField(f); ok && !slices.Contains(it.LockedFields, field) {
				it.LockedFields = append(it.LockedFields, field)
			}
		}
	case "lockdata":
		it.Locked = strings.EqualFold(n.value(), "true")
	case "tagline":
		it.Tagline = n.value()
	case "country":
		// Jellyfin keeps only the last element; every country is kept.
		for c := range strings.SplitSeq(n.value(), "/") {
			if c = strings.TrimSpace(c); c != "" && !slices.Contains(it.ProductionLocations, c) {
				it.ProductionLocations = append(it.ProductionLocations, c)
			}
		}
	case "mpaa":
		it.OfficialRating = n.value()
	case "customrating":
		it.CustomRating = n.value()
	case "runtime":
		minutes, _, _ := strings.Cut(n.value(), " ")
		if v, ok := parseInt(minutes); ok {
			it.Runtime = time.Duration(v) * time.Minute
		}
	case "aspectratio":
		if v := n.value(); v != "" {
			it.AspectRatio = v
		}
	case "studio":
		addValue(&it.Studios, n.value())
	case "director":
		for _, name := range stringArray(n.text.String()) {
			addPerson(&r.People, Person{Name: name, Kind: core.CreditDirector})
		}
	case "credits":
		for name := range strings.SplitSeq(n.text.String(), "/") {
			if name = strings.TrimSpace(name); name != "" {
				addPerson(&r.People, Person{Name: name, Kind: core.CreditWriter})
			}
		}
	case "writer":
		for _, name := range stringArray(n.text.String()) {
			addPerson(&r.People, Person{Name: name, Kind: core.CreditWriter})
		}
	case "actor":
		if person, ok := personFromNode(n); ok {
			addPerson(&r.People, person)
		}
	case "trailer":
		if url, ok := youTubeTrailer(n.value()); ok {
			it.RemoteTrailers = append(it.RemoteTrailers, url)
		}
	case "displayorder":
		if v := n.value(); v != "" && (p.kind == core.KindSeries || p.kind == core.KindCollection) {
			it.DisplayOrder = v
		}
	case "year":
		if v, ok := parseInt(n.value()); ok && v > 1850 {
			it.ProductionYear = v
		}
	case "rating", "communityrating":
		if v, ok := parseRating(n.value()); ok && (n.name == "rating" || v <= 10) {
			it.CommunityRating = v
		}
	case "ratings":
		ratings(n, it)
	case "aired", "formed", "premiered", "releasedate":
		if t, ok := parseExact(n.value(), p.opts.ReleaseDateFormat); ok {
			it.PremiereDate = &t
			if it.ProductionYear == 0 {
				it.ProductionYear = t.Year()
			}
		}
	case "enddate":
		if t, ok := parseExact(n.value(), p.opts.ReleaseDateFormat); ok {
			it.EndDate = &t
		}
	case "genre":
		for g := range strings.SplitSeq(n.text.String(), "/") {
			addValue(&it.Genres, strings.TrimSpace(g))
		}
	case "style", "tag":
		addValue(&it.Tags, n.value())
	case "fileinfo":
		p.fileInfo(n, r)
	case "uniqueid":
		if provider := n.attr("type"); provider != "" {
			setID(it, providerKey(provider), n.value())
		}
	case "thumb":
		p.thumb(n, n.attr("aspect"), "thumb", r)
	case "fanart":
		if t := n.child("thumb"); t != nil {
			p.thumb(t, t.attr("aspect"), "fanart", r)
		}
	default:
		if provider, ok := idElements[strings.ToLower(n.name)]; ok {
			setID(it, provider, n.value())
		}
	}
}

// kindElement handles the elements specific to the item kind and reports
// whether it did.
func (p *nfoParser) kindElement(n *node, r *Result) bool {
	it := &r.Item
	switch p.kind {
	case core.KindMovie, core.KindVideo, core.KindMusicVideo:
		switch n.name {
		case "id":
			p.idElement(n, it)
		case "set":
			if p.kind == core.KindMovie {
				setID(it, core.ProviderTMDBCollection, n.attr("tmdbcolid"))
				if name := n.child("name"); name != nil {
					it.CollectionName = name.text.String()
				} else if v := n.text.String(); strings.TrimSpace(v) != "" {
					it.CollectionName = v
				}
			}
		case "artist":
			if p.kind == core.KindMusicVideo {
				addValue(&it.Artists, n.value())
			}
		case "album":
			if v := n.value(); v != "" && p.kind == core.KindMusicVideo {
				it.Album = v
			}
		default:
			return false
		}
	case core.KindSeries:
		switch n.name {
		case "id":
			p.idElement(n, it)
		case "airs_dayofweek":
			it.AirDays = airDays(n.value())
		case "airs_time":
			it.AirTime = n.value()
		case "status":
			if s, ok := seriesStatus(n.value()); ok {
				it.SeriesStatus = s
			}
		case "namedseason":
		default:
			return false
		}
	case core.KindSeason:
		switch n.name {
		case "seasonnumber":
			if v, ok := parseInt(n.value()); ok {
				it.IndexNumber = &v
			}
		case "seasonname":
			it.Name = n.value()
		default:
			return false
		}
	case core.KindEpisode:
		number := func(dst **int) {
			if v, ok := parseInt(n.value()); ok {
				*dst = &v
			}
		}
		switch n.name {
		case "season":
			number(&it.ParentIndexNumber)
		case "episode":
			number(&it.IndexNumber)
		case "episodenumberend":
			number(&it.IndexNumberEnd)
		case "airsbefore_episode", "displayepisode":
			number(&it.AirsBeforeEpisodeNumber)
		case "airsafter_season", "displayafterseason":
			number(&it.AirsAfterSeasonNumber)
		case "airsbefore_season", "displayseason":
			number(&it.AirsBeforeSeasonNumber)
		case "showtitle":
			r.SeriesName = n.value()
		default:
			return false
		}
	default:
		return false
	}
	return true
}

// idElement reads <id>, which holds an IMDb ID or carries TMDB, TVDB and
// IMDb IDs as attributes.
func (p *nfoParser) idElement(n *node, it *core.Item) {
	setID(it, core.ProviderTMDB, n.attr("TMDB"))
	setID(it, core.ProviderTVDB, n.attr("TVDB"))
	imdb := n.attr("IMDB")
	if content := n.text.String(); imdb == "" && strings.HasPrefix(content, "tt") {
		imdb = content
	}
	setID(it, core.ProviderIMDb, imdb)
}

// idElements maps the elements holding a provider ID, such as <tmdbid>.
var idElements = map[string]core.Provider{
	"tmdbid":              core.ProviderTMDB,
	"imdbid":              core.ProviderIMDb,
	"imdb_id":             core.ProviderIMDb,
	"tvdbid":              core.ProviderTVDB,
	"tmdbcollectionid":    core.ProviderTMDBCollection,
	"collectionnumber":    core.ProviderTMDBCollection,
	"tmdbcolid":           core.ProviderTMDBCollection,
	"tmdbcol":             core.ProviderTMDBCollection,
	"musicbrainzartistid": core.ProviderMusicBrainzArtist,
}

// providerKey normalizes the provider of a <uniqueid type="…">.
func providerKey(name string) core.Provider {
	key := strings.ToLower(name)
	if p, ok := idElements[key]; ok {
		return p
	}
	if p, ok := idElements[key+"id"]; ok {
		return p
	}
	return core.Provider(key)
}

func setID(it *core.Item, provider core.Provider, id string) {
	if id = strings.TrimSpace(id); id == "" {
		return
	}
	if it.ExternalIDs == nil {
		it.ExternalIDs = map[core.Provider]string{}
	}
	it.ExternalIDs[provider] = id
}

// thumb reads an image. The aspect gives its kind; without one it is a
// backdrop inside <fanart> and a poster otherwise. Aspects with a dot
// ("set.poster") describe other items and are skipped, as are additional
// images of a kind already found.
func (p *nfoParser) thumb(n *node, aspect, parent string, r *Result) {
	if strings.TrimSpace(aspect) == "" {
		aspect = "poster"
		if parent == "fanart" {
			aspect = "fanart"
		}
	}
	location := n.text.String()
	if location == "" || strings.Contains(aspect, ".") {
		return
	}
	kind := imageKind(aspect)
	if isLocalPath(location) {
		if slices.ContainsFunc(r.LocalImages, func(i LocalImage) bool { return i.Kind == kind }) ||
			p.opts.FileExists == nil || !p.opts.FileExists(location) {
			return
		}
		r.LocalImages = append(r.LocalImages, LocalImage{Kind: kind, Path: location})
		return
	}
	if !strings.Contains(location, "://") ||
		slices.ContainsFunc(r.RemoteImages, func(i RemoteImage) bool { return i.Kind == kind }) {
		return
	}
	r.RemoteImages = append(r.RemoteImages, RemoteImage{Kind: kind, URL: location})
}

// isLocalPath reports whether s is an absolute file path, in Unix or
// Windows form.
func isLocalPath(s string) bool {
	return strings.HasPrefix(s, "/") || strings.HasPrefix(s, `\\`) ||
		(len(s) >= 3 && s[1] == ':' && (s[2] == '\\' || s[2] == '/'))
}

func imageKind(aspect string) core.ImageKind {
	switch aspect {
	case "banner":
		return core.ImageBanner
	case "clearlogo":
		return core.ImageLogo
	case "discart":
		return core.ImageDisc
	case "landscape":
		return core.ImageThumb
	case "clearart":
		return core.ImageArt
	case "fanart":
		return core.ImageBackdrop
	}
	return core.ImagePrimary
}

// fileInfo reads <fileinfo><streamdetails>.
func (p *nfoParser) fileInfo(n *node, r *Result) {
	details := n.child("streamdetails")
	if details == nil {
		return
	}
	for _, s := range details.children {
		switch s.name {
		case "video":
			if !p.kind.HasMedia() {
				continue
			}
			for _, v := range s.children {
				switch v.name {
				case "format3d":
					if f, ok := format3D(v.value()); ok {
						r.Item.Video3DFormat = f
					}
				case "aspect":
					r.Item.AspectRatio = v.value()
				case "width":
					r.Video.Width, _ = parseInt(v.value())
				case "height":
					r.Video.Height, _ = parseInt(v.value())
				case "durationinseconds":
					if secs, ok := parseInt(v.value()); ok {
						r.Item.Runtime = time.Duration(secs) * time.Second
					}
				}
			}
		case "subtitle":
			if s.child("language") != nil && p.kind.HasMedia() {
				r.Video.HasSubtitles = true
			}
		}
	}
}

func format3D(s string) (core.Video3DFormat, bool) {
	switch strings.ToUpper(s) {
	case "HSBS":
		return core.Video3DHalfSideBySide, true
	case "HTAB":
		return core.Video3DHalfTopAndBottom, true
	case "FTAB":
		return core.Video3DFullTopAndBottom, true
	case "FSBS":
		return core.Video3DFullSideBySide, true
	case "MVC":
		return core.Video3DMVC, true
	}
	return "", false
}

// ratings reads <ratings><rating name="…"><value>; Rotten Tomatoes
// critic scores are critic ratings, all others community ratings.
func ratings(n *node, it *core.Item) {
	for _, rating := range n.children {
		if rating.name != "rating" {
			continue
		}
		name := strings.ToLower(rating.attr("name"))
		for _, v := range rating.children {
			if v.name != "value" {
				continue
			}
			value, ok := parseRating(v.value())
			if !ok {
				continue
			}
			if strings.Contains(name, "tomato") && !strings.Contains(name, "audience") {
				if !strings.Contains(name, "avg") {
					it.CriticRating = value
				}
			} else {
				it.CommunityRating = value
			}
		}
	}
}

// parseRating parses a non-negative decimal with a dot or comma.
func parseRating(s string) (float64, bool) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", ".")
	if s == "" || s[0] == '-' || s[0] == '+' {
		return 0, false
	}
	v, err := strconv.ParseFloat(s, 64)
	return v, err == nil
}

// personFromNode reads an <actor> element; its type defaults to actor.
func personFromNode(n *node) (Person, bool) {
	p := Person{Kind: core.CreditActor}
	for _, c := range n.children {
		switch c.name {
		case "name", "Name":
			p.Name = c.value()
		case "role", "Role":
			p.Role = c.value()
		case "type", "Type":
			if kind, ok := creditKind(c.value()); ok {
				p.Kind = kind
			}
		case "order", "sortorder", "SortOrder":
			if v, ok := parseInt(c.value()); ok {
				p.Order = &v
			}
		case "thumb":
			p.ImageURL = c.value()
		}
	}
	return p, strings.TrimSpace(p.Name) != ""
}

// creditKind maps Jellyfin's person kinds; kinds Mavio does not
// distinguish are "other".
func creditKind(s string) (core.CreditKind, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "actor":
		return core.CreditActor, true
	case "gueststar":
		return core.CreditGuestStar, true
	case "director":
		return core.CreditDirector, true
	case "writer":
		return core.CreditWriter, true
	case "producer":
		return core.CreditProducer, true
	case "creator":
		return core.CreditCreator, true
	case "composer":
		return core.CreditComposer, true
	case "conductor":
		return core.CreditConductor, true
	case "lyricist":
		return core.CreditLyricist, true
	case "artist", "albumartist":
		return core.CreditArtist, true
	case "author":
		return core.CreditAuthor, true
	case "narrator":
		return core.CreditNarrator, true
	case "unknown", "arranger", "engineer", "mixer", "remixer", "illustrator", "penciller", "inker",
		"colorist", "letterer", "coverartist", "editor", "translator":
		return core.CreditOther, true
	}
	return "", false
}

// addPerson adds a credit unless it duplicates one by name, kind and role,
// as Jellyfin does: actor and guest star are the same credit, a missing
// role is filled in, and a role of "Director", "Writer", "Producer" or
// "Guest star" sets the kind.
func addPerson(people *[]Person, p Person) {
	p.Name = strings.TrimSpace(p.Name)
	switch strings.ToLower(p.Role) {
	case "gueststar":
		p.Kind = core.CreditGuestStar
	case "director":
		p.Kind = core.CreditDirector
	case "producer":
		p.Kind = core.CreditProducer
	case "writer":
		p.Kind = core.CreditWriter
	}
	isCast := func(k core.CreditKind) bool { return k == core.CreditActor || k == core.CreditGuestStar }
	same := func(e Person) bool {
		return strings.EqualFold(e.Name, p.Name) && (e.Kind == p.Kind || (isCast(e.Kind) && isCast(p.Kind)))
	}
	list := *people
	i := slices.IndexFunc(list, func(e Person) bool { return same(e) && strings.EqualFold(e.Role, p.Role) })
	if i < 0 {
		if p.Role == "" {
			i = slices.IndexFunc(list, same)
		} else if i = slices.IndexFunc(list, func(e Person) bool { return same(e) && e.Role == "" }); i >= 0 {
			list[i].Role = p.Role
		}
	}
	if i < 0 {
		*people = append(list, p)
		return
	}
	e := &list[i]
	if p.Kind == core.CreditGuestStar {
		e.Kind = core.CreditGuestStar
	}
	if p.Order != nil {
		e.Order = p.Order
	}
	if p.ImageURL != "" {
		e.ImageURL = p.ImageURL
	}
}

// stringArray splits a list of names on "|" or ";", or on commas when
// there are neither.
func stringArray(s string) []string {
	seps := ","
	if strings.ContainsAny(s, "|;") {
		seps = "|;"
	}
	var out []string
	for _, part := range strings.FieldsFunc(strings.Trim(strings.TrimSpace(s), seps), func(r rune) bool {
		return strings.ContainsRune(seps, r)
	}) {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// youTubeTrailer converts a Kodi YouTube plugin URL into a watch URL.
func youTubeTrailer(s string) (string, bool) {
	for _, prefix := range []string{
		"plugin://plugin.video.youtube/?action=play_video&videoid=", // deprecated
		"plugin://plugin.video.youtube/play/?video_id=",
	} {
		if len(s) > len(prefix) && strings.EqualFold(s[:len(prefix)], prefix) {
			return YouTubeWatchURL + s[len(prefix):], true
		}
	}
	return "", false
}

func addValue(list *[]string, v string) {
	if v = strings.TrimSpace(v); v != "" && !slices.Contains(*list, v) {
		*list = append(*list, v)
	}
}

func lockableField(s string) (core.MetadataField, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "productionlocations" {
		return core.FieldProductionLocations, true
	}
	if s == "officialrating" {
		return core.FieldOfficialRating, true
	}
	f := core.MetadataField(s)
	return f, f.Valid()
}

// airDays parses "Daily" or a day name.
func airDays(s string) []time.Weekday {
	if strings.EqualFold(s, "daily") {
		return []time.Weekday{time.Sunday, time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday, time.Saturday}
	}
	for d := time.Sunday; d <= time.Saturday; d++ {
		if strings.EqualFold(strings.TrimSpace(s), d.String()) {
			return []time.Weekday{d}
		}
	}
	return nil
}

// seriesStatus parses a status name or one of its synonyms.
func seriesStatus(s string) (core.SeriesStatus, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "continuing", "pilot", "returning series", "returning":
		return core.SeriesContinuing, true
	case "ended", "cancelled", "canceled":
		return core.SeriesEnded, true
	case "unreleased":
		return core.SeriesUnreleased, true
	}
	return "", false
}

// parseInt parses an integer like .NET's int.TryParse.
func parseInt(s string) (int, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	return int(n), err == nil
}

// dateTimeLayouts are the forms dates and times take in NFO files.
var dateTimeLayouts = []string{
	time.DateTime, "2006-01-02T15:04:05", time.RFC3339, "2006-01-02T15:04:05.999999999",
	time.DateOnly, "2006/01/02 15:04:05", "2006/01/02",
}

// parseDateTime parses a date and time, as UTC unless it has a zone.
func parseDateTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	for _, l := range dateTimeLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

// parseExact parses a date in a .NET layout such as "yyyy-MM-dd", as UTC.
func parseExact(s, format string) (time.Time, bool) {
	layout := strings.NewReplacer("yyyy", "2006", "MM", "01", "dd", "02", "HH", "15", "mm", "04", "ss", "05").Replace(format)
	t, err := time.Parse(layout, strings.TrimSpace(s))
	return t.UTC(), err == nil
}

// indexFold returns the index of the first case-insensitive occurrence of
// sub in s, or -1.
func indexFold(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if strings.EqualFold(s[i:i+len(sub)], sub) {
			return i
		}
	}
	return -1
}

// SeasonNameFromSeriesNFO returns the name a series NFO file gives a
// season in <namedseason number="…">.
func SeasonNameFromSeriesNFO(data []byte, season int) (string, bool) {
	end := strings.LastIndex(string(data), "</")
	if end >= 0 {
		if i := strings.IndexByte(string(data[end:]), '>'); i >= 0 {
			data = data[:end+i+1]
		}
	}
	root, err := parseXML(data)
	if err != nil {
		return "", false
	}
	for _, n := range root.children {
		if n.name != "namedseason" {
			continue
		}
		if number, ok := parseInt(n.attr("number")); ok && number == season {
			if name := strings.TrimSpace(n.text.String()); name != "" {
				return n.text.String(), true
			}
		}
	}
	return "", false
}

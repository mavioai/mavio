package main

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// External ID keys, as the host names providers.
const (
	keyTMDB           = "tmdb"
	keyTMDBCollection = "tmdb_collection"
	keyIMDb           = "imdb"
	keyTVDB           = "tvdb"
)

func (p *plugin) Search(ctx context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	s, err := p.session()
	if err != nil {
		return nil, err
	}
	l := req.GetLookup()
	name := cleanName(l.GetName())
	resp := &pluginv1.SearchResponse{}
	if name == "" {
		return resp, nil
	}
	lang := normalizeLanguage(l.GetLanguage(), l.GetCountry())
	year := int(l.GetYear())
	var results []*pluginv1.SearchResult
	switch l.GetKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MOVIE:
		movies, err := s.searchMovies(ctx, name, year, lang, s.adult)
		if err != nil {
			return nil, err
		}
		for _, m := range movies {
			results = append(results, searchResult(m.ID, m.Title, m.ReleaseDate, m.Overview, m.PosterPath, scoreCandidate(name, year, movieCandidate(m))))
		}
	case pluginv1.MediaKind_MEDIA_KIND_SERIES:
		series, err := s.searchSeries(ctx, name, year, lang, s.adult)
		if err != nil {
			return nil, err
		}
		for _, r := range series {
			results = append(results, searchResult(r.ID, r.Name, r.FirstAirDate, r.Overview, r.PosterPath, scoreCandidate(name, year, seriesCandidate(r))))
		}
	default:
		// Seasons and episodes are found through their series.
		return resp, nil
	}
	// Best match first; equal scores keep TMDB's order.
	slices.SortStableFunc(results, func(a, b *pluginv1.SearchResult) int { return cmp.Compare(b.GetScore(), a.GetScore()) })
	if limit := int(req.GetLimit()); limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	resp.SetResults(results)
	return resp, nil
}

// scoreCandidate is findBestMatch's score as relevance in [0, 1].
func scoreCandidate(name string, year int, c candidate) float64 {
	n := normalizeTitle(name)
	score := max(scoreTitle(n, c.title), scoreTitle(n, c.originalTitle)) + scoreYear(year, c.date)
	return float64(score) / (titleExactScore + yearExactScore)
}

func searchResult(id int, name, date, overview, poster string, score float64) *pluginv1.SearchResult {
	r := pluginv1.SearchResult_builder{
		Name:        proto.String(name),
		Overview:    proto.String(overview),
		ExternalIds: map[string]string{keyTMDB: strconv.Itoa(id)},
		Score:       proto.Float64(score),
	}.Build()
	if y, ok := yearOf(date); ok {
		r.SetYear(int32(y))
	}
	if poster != "" {
		r.SetImageUrl(imageURL(poster))
	}
	return r
}

func movieCandidate(m tmdbSearchMovie) candidate {
	return candidate{title: m.Title, originalTitle: m.OriginalTitle, date: m.ReleaseDate}
}

func seriesCandidate(s tmdbSearchSeries) candidate {
	return candidate{title: s.Name, originalTitle: s.OriginalName, date: s.FirstAirDate}
}

func (p *plugin) GetMetadata(ctx context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	s, err := p.session()
	if err != nil {
		return nil, err
	}
	l := req.GetLookup()
	var md *pluginv1.Metadata
	switch l.GetKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MOVIE:
		md, err = s.movieMetadata(ctx, l)
	case pluginv1.MediaKind_MEDIA_KIND_SERIES:
		md, err = s.seriesMetadata(ctx, l)
	case pluginv1.MediaKind_MEDIA_KIND_SEASON:
		md, err = s.seasonMetadata(ctx, l)
	case pluginv1.MediaKind_MEDIA_KIND_EPISODE:
		md, err = s.episodeMetadata(ctx, l)
	}
	if errors.Is(err, errNotFound) {
		md, err = nil, nil
	}
	if err != nil {
		return nil, err
	}
	resp := pluginv1.GetMetadataResponse_builder{Found: proto.Bool(md != nil)}.Build()
	if md != nil {
		resp.SetMetadata(md)
	}
	return resp, nil
}

// request holds a lookup's language preferences.
type request struct {
	language, country string
}

func requestOf(l *pluginv1.Lookup) request {
	return request{language: normalizeLanguage(l.GetLanguage(), l.GetCountry()), country: l.GetCountry()}
}

// ratingCountry is the country whose certification is used; US ratings
// are the fallback.
func (r request) ratingCountry() string {
	if r.country == "" {
		return "US"
	}
	return strings.ToUpper(r.country)
}

// movieID finds a movie's TMDB ID: the one known, the one TMDB has for its
// IMDb ID, or the best search result. It returns 0 when there is none.
func (s session) movieID(ctx context.Context, l *pluginv1.Lookup, r request) (int, error) {
	ids := l.GetExternalIds()
	if id, ok := parseTMDBID(ids[keyTMDB]); ok {
		return id, nil
	}
	if imdb := ids[keyIMDb]; imdb != "" {
		f, err := s.find(ctx, imdb, "imdb_id")
		if err != nil && !errors.Is(err, errNotFound) {
			return 0, err
		}
		if len(f.MovieResults) > 0 {
			return f.MovieResults[0].ID, nil
		}
	}
	name := cleanName(l.GetName())
	if name == "" {
		return 0, nil
	}
	movies, err := s.searchMovies(ctx, name, int(l.GetYear()), r.language, s.adult)
	if err != nil {
		return 0, err
	}
	cands := make([]candidate, len(movies))
	for i, m := range movies {
		cands[i] = movieCandidate(m)
	}
	if i := findBestMatch(cands, name, int(l.GetYear())); i >= 0 {
		return movies[i].ID, nil
	}
	return 0, nil
}

// seriesID finds a series' TMDB ID like movieID, also by its TVDB ID; name
// is empty for seasons and episodes, which only know their series' IDs.
func (s session) seriesID(ctx context.Context, ids map[string]string, name string, year int, r request) (int, error) {
	if id, ok := parseTMDBID(ids[keyTMDB]); ok {
		return id, nil
	}
	for _, src := range []struct{ key, source string }{{keyIMDb, "imdb_id"}, {keyTVDB, "tvdb_id"}} {
		id := ids[src.key]
		if id == "" {
			continue
		}
		f, err := s.find(ctx, id, src.source)
		if err != nil && !errors.Is(err, errNotFound) {
			return 0, err
		}
		if len(f.TVResults) > 0 {
			return f.TVResults[0].ID, nil
		}
	}
	name = cleanName(name)
	if name == "" {
		return 0, nil
	}
	series, err := s.searchSeries(ctx, name, year, r.language, s.adult)
	if err != nil {
		return 0, err
	}
	cands := make([]candidate, len(series))
	for i, r := range series {
		cands[i] = seriesCandidate(r)
	}
	if i := findBestMatch(cands, name, year); i >= 0 {
		return series[i].ID, nil
	}
	return 0, nil
}

func (s session) movieMetadata(ctx context.Context, l *pluginv1.Lookup) (*pluginv1.Metadata, error) {
	r := requestOf(l)
	id, err := s.movieID(ctx, l, r)
	if err != nil || id == 0 {
		return nil, err
	}
	m, err := s.movie(ctx, id, r.language, r.country)
	if err != nil {
		return nil, err
	}
	md := pluginv1.Metadata_builder{
		Name:            proto.String(m.Title),
		OriginalTitle:   proto.String(m.OriginalTitle),
		Overview:        proto.String(m.Overview),
		Tagline:         proto.String(m.Tagline),
		CommunityRating: proto.Float64(m.VoteAverage),
		Genres:          names(m.Genres),
		Tags:            names(m.Keywords.Keywords),
		Studios:         names(m.Companies),
		ExternalIds:     map[string]string{keyTMDB: strconv.Itoa(m.ID)},
	}.Build()
	setDate(md, m.ReleaseDate)
	setRuntime(md, m.Runtime)
	if m.IMDbID != "" {
		md.GetExternalIds()[keyIMDb] = m.IMDbID
	}
	if m.Collection != nil && m.Collection.ID > 0 {
		md.GetExternalIds()[keyTMDBCollection] = strconv.Itoa(m.Collection.ID)
	}
	ratings := map[string]string{}
	for _, rd := range m.ReleaseDates.Results {
		for _, d := range rd.Dates {
			if d.Certification != "" {
				ratings[strings.ToUpper(rd.Country)] = d.Certification
				break
			}
		}
	}
	setRating(md, r, ratings)
	people := castCredits(mapCast(m.Credits.Cast, s.cast), pluginv1.CreditKind_CREDIT_KIND_ACTOR)
	people = append(people, crewCredits(m.Credits.Crew)...)
	md.SetPeople(people)
	md.SetImages(images(r, []imageSet{
		{pluginv1.ImageKind_IMAGE_KIND_PRIMARY, m.Images.Posters},
		{pluginv1.ImageKind_IMAGE_KIND_BACKDROP, m.Images.Backdrops},
		{pluginv1.ImageKind_IMAGE_KIND_LOGO, m.Images.Logos},
	}))
	return md, nil
}

func (s session) seriesMetadata(ctx context.Context, l *pluginv1.Lookup) (*pluginv1.Metadata, error) {
	r := requestOf(l)
	id, err := s.seriesID(ctx, l.GetExternalIds(), l.GetName(), int(l.GetYear()), r)
	if err != nil || id == 0 {
		return nil, err
	}
	t, err := s.series(ctx, id, r.language, r.country)
	if err != nil {
		return nil, err
	}
	md := pluginv1.Metadata_builder{
		Name:            proto.String(t.Name),
		OriginalTitle:   proto.String(t.OriginalName),
		Overview:        proto.String(t.Overview),
		Tagline:         proto.String(t.Tagline),
		CommunityRating: proto.Float64(t.VoteAverage),
		Genres:          names(t.Genres),
		Tags:            names(t.Keywords.Results),
		Studios:         names(t.Networks),
		ExternalIds:     externalIDs(t.ID, t.ExternalIDs),
		SeriesStatus:    seriesStatus(t.Status).Enum(),
	}.Build()
	setDate(md, t.FirstAirDate)
	if md.GetSeriesStatus() == pluginv1.SeriesStatus_SERIES_STATUS_ENDED && t.LastAirDate != "" {
		md.SetEndDate(t.LastAirDate)
	}
	if len(t.EpisodeRunTime) > 0 {
		setRuntime(md, t.EpisodeRunTime[0])
	}
	ratings := map[string]string{}
	for _, cr := range t.ContentRatings.Results {
		if cr.Rating != "" {
			ratings[strings.ToUpper(cr.Country)] = cr.Rating
		}
	}
	setRating(md, r, ratings)
	people := castCredits(mapAggregateCast(t.Credits.Cast, s.cast), pluginv1.CreditKind_CREDIT_KIND_ACTOR)
	for _, c := range t.CreatedBy {
		if strings.TrimSpace(c.Name) != "" {
			people = append(people, person(strings.TrimSpace(c.Name), pluginv1.CreditKind_CREDIT_KIND_CREATOR, "", c.ID, c.ProfilePath))
		}
	}
	md.SetPeople(people)
	md.SetImages(images(r, []imageSet{
		{pluginv1.ImageKind_IMAGE_KIND_PRIMARY, t.Images.Posters},
		{pluginv1.ImageKind_IMAGE_KIND_BACKDROP, t.Images.Backdrops},
		{pluginv1.ImageKind_IMAGE_KIND_LOGO, t.Images.Logos},
	}))
	return md, nil
}

func (s session) seasonMetadata(ctx context.Context, l *pluginv1.Lookup) (*pluginv1.Metadata, error) {
	r := requestOf(l)
	if !l.HasSeasonNumber() {
		return nil, nil
	}
	seriesID, err := s.seriesID(ctx, l.GetSeriesExternalIds(), "", 0, r)
	if err != nil || seriesID == 0 {
		return nil, err
	}
	t, err := s.season(ctx, seriesID, int(l.GetSeasonNumber()), r.language, r.country)
	if err != nil {
		return nil, err
	}
	md := pluginv1.Metadata_builder{
		Name:        proto.String(t.Name),
		Overview:    proto.String(t.Overview),
		ExternalIds: externalIDs(t.ID, t.ExternalIDs),
	}.Build()
	setDate(md, t.AirDate)
	md.SetPeople(castCredits(mapCast(t.Credits.Cast, s.cast), pluginv1.CreditKind_CREDIT_KIND_ACTOR))
	md.SetImages(images(r, []imageSet{{pluginv1.ImageKind_IMAGE_KIND_PRIMARY, t.Images.Posters}}))
	return md, nil
}

func (s session) episodeMetadata(ctx context.Context, l *pluginv1.Lookup) (*pluginv1.Metadata, error) {
	r := requestOf(l)
	if !l.HasSeasonNumber() || !l.HasEpisodeNumber() {
		return nil, nil
	}
	seriesID, err := s.seriesID(ctx, l.GetSeriesExternalIds(), "", 0, r)
	if err != nil || seriesID == 0 {
		return nil, err
	}
	e, err := s.episode(ctx, seriesID, int(l.GetSeasonNumber()), int(l.GetEpisodeNumber()), r.language, r.country)
	if err != nil {
		return nil, err
	}
	md := pluginv1.Metadata_builder{
		Name:            proto.String(e.Name),
		Overview:        proto.String(e.Overview),
		CommunityRating: proto.Float64(e.VoteAverage),
		ExternalIds:     externalIDs(e.ID, e.ExternalIDs),
	}.Build()
	setDate(md, e.AirDate)
	setRuntime(md, e.Runtime)
	people := castCredits(mapCast(e.Credits.Cast, s.cast), pluginv1.CreditKind_CREDIT_KIND_ACTOR)
	people = append(people, castCredits(mapCast(e.Credits.GuestStars, s.cast), pluginv1.CreditKind_CREDIT_KIND_GUEST_STAR)...)
	people = append(people, crewCredits(e.Credits.Crew)...)
	md.SetPeople(people)
	md.SetImages(images(r, []imageSet{{pluginv1.ImageKind_IMAGE_KIND_PRIMARY, e.Images.Stills}}))
	return md, nil
}

func names(ns []tmdbName) []string {
	var out []string
	for _, n := range ns {
		if n := strings.TrimSpace(n.Name); n != "" {
			out = append(out, n)
		}
	}
	return out
}

func externalIDs(id int, ext tmdbExternalIDs) map[string]string {
	ids := map[string]string{keyTMDB: strconv.Itoa(id)}
	if ext.IMDbID != "" {
		ids[keyIMDb] = ext.IMDbID
	}
	if ext.TVDBID > 0 {
		ids[keyTVDB] = strconv.Itoa(ext.TVDBID)
	}
	return ids
}

// setDate sets the premiere date and production year from a TMDB date.
func setDate(md *pluginv1.Metadata, date string) {
	if y, ok := yearOf(date); ok {
		md.SetPremiereDate(date)
		md.SetProductionYear(int32(y))
	}
}

func setRuntime(md *pluginv1.Metadata, minutes int) {
	if minutes > 0 {
		md.SetRuntime(durationpb.New(time.Duration(minutes) * time.Minute))
	}
}

// setRating sets the certification for the requested country, or the US
// one when the country has none.
func setRating(md *pluginv1.Metadata, r request, ratings map[string]string) {
	for _, country := range []string{r.ratingCountry(), "US"} {
		if rating := ratings[country]; rating != "" {
			md.SetOfficialRating(parentalRating(country, rating))
			return
		}
	}
}

// seriesStatus maps TMDB's status: "Returning Series", "In Production",
// "Planned", "Pilot", "Ended" or "Canceled".
func seriesStatus(status string) pluginv1.SeriesStatus {
	switch strings.ToLower(status) {
	case "returning series", "in production":
		return pluginv1.SeriesStatus_SERIES_STATUS_CONTINUING
	case "ended", "canceled", "cancelled":
		return pluginv1.SeriesStatus_SERIES_STATUS_ENDED
	case "planned", "pilot":
		return pluginv1.SeriesStatus_SERIES_STATUS_UNRELEASED
	}
	return pluginv1.SeriesStatus_SERIES_STATUS_UNSPECIFIED
}

func person(name string, kind pluginv1.CreditKind, role string, id int, profile string) *pluginv1.PersonCredit {
	p := pluginv1.PersonCredit_builder{
		Name:        proto.String(name),
		Kind:        kind.Enum(),
		Role:        proto.String(role),
		ExternalIds: map[string]string{keyTMDB: strconv.Itoa(id)},
	}.Build()
	if profile != "" {
		p.SetImageUrl(imageURL(profile))
	}
	return p
}

func castCredits(cs []credit, kind pluginv1.CreditKind) []*pluginv1.PersonCredit {
	out := make([]*pluginv1.PersonCredit, 0, len(cs))
	for _, c := range cs {
		p := person(c.name, kind, c.role, c.id, c.profile)
		p.SetOrder(int32(c.order))
		out = append(out, p)
	}
	return out
}

// crewCredits credits directors, writers and producers once each per kind.
func crewCredits(crew []tmdbCrew) []*pluginv1.PersonCredit {
	kinds := map[string]pluginv1.CreditKind{
		"director": pluginv1.CreditKind_CREDIT_KIND_DIRECTOR,
		"writer":   pluginv1.CreditKind_CREDIT_KIND_WRITER,
		"producer": pluginv1.CreditKind_CREDIT_KIND_PRODUCER,
	}
	type key struct {
		name string
		kind pluginv1.CreditKind
	}
	seen := map[key]bool{}
	var out []*pluginv1.PersonCredit
	for _, c := range crew {
		kind, ok := kinds[crewKind(c.Department, c.Job)]
		name := strings.TrimSpace(c.Name)
		if !ok || name == "" || seen[key{name, kind}] {
			continue
		}
		seen[key{name, kind}] = true
		out = append(out, person(name, kind, strings.TrimSpace(c.Job), c.ID, c.ProfilePath))
	}
	return out
}

func langBase(lang string) string {
	base, _, _ := strings.Cut(lang, "-")
	return base
}

func imageURL(path string) string {
	return imageBaseURL + originalImageSize + path
}

type imageSet struct {
	kind   pluginv1.ImageKind
	images []tmdbImage
}

// images lists the images of each kind, those in the requested language
// first, then those without text (first for backdrops, which are best
// without), then English ones; better rated first within each group.
func images(r request, sets []imageSet) []*pluginv1.RemoteImage {
	var out []*pluginv1.RemoteImage
	base := langBase(r.language)
	for _, set := range sets {
		rank := func(lang string) int {
			switch {
			case lang == "" && set.kind == pluginv1.ImageKind_IMAGE_KIND_BACKDROP:
				return 0
			case r.language != "" && strings.EqualFold(lang, r.language):
				return 1
			case base != "" && strings.EqualFold(langBase(lang), base):
				return 2
			case lang == "":
				return 3
			case strings.EqualFold(langBase(lang), "en"):
				return 4
			}
			return 5
		}
		imgs := make([]*pluginv1.RemoteImage, 0, len(set.images))
		ranks := map[*pluginv1.RemoteImage]int{}
		for _, img := range set.images {
			if img.FilePath == "" {
				continue
			}
			lang := imageLanguage(img.Language, img.Region, r.language)
			ri := pluginv1.RemoteImage_builder{
				Kind:     set.kind.Enum(),
				Url:      proto.String(imageURL(img.FilePath)),
				Width:    proto.Int32(int32(img.Width)),
				Height:   proto.Int32(int32(img.Height)),
				Language: proto.String(lang),
				Score:    proto.Float64(img.VoteAverage / 10),
			}.Build()
			ranks[ri] = rank(lang)
			imgs = append(imgs, ri)
		}
		slices.SortStableFunc(imgs, func(a, b *pluginv1.RemoteImage) int {
			return cmp.Or(cmp.Compare(ranks[a], ranks[b]), cmp.Compare(b.GetScore(), a.GetScore()))
		})
		out = append(out, imgs...)
	}
	return out
}

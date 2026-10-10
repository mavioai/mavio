package main

import (
	"cmp"
	"context"
	"errors"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

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
	keyTVRage         = "tvrage"
	keyTVMaze         = "tvmaze"
)

// radarrMovie is Radarr's MovieResource.
type radarrMovie struct {
	TmdbID            int                    `json:"TmdbId"`
	ImdbID            string                 `json:"ImdbId"`
	Title             string                 `json:"Title"`
	OriginalTitle     string                 `json:"OriginalTitle"`
	OriginalLanguage  string                 `json:"OriginalLanguage"`
	Overview          string                 `json:"Overview"`
	Runtime           int                    `json:"Runtime"`
	Popularity        float64                `json:"Popularity"`
	Year              int                    `json:"Year"`
	Premier           *time.Time             `json:"Premier"`
	InCinema          *time.Time             `json:"InCinema"`
	PhysicalRelease   *time.Time             `json:"PhysicalRelease"`
	DigitalRelease    *time.Time             `json:"DigitalRelease"`
	Images            []radarrImage          `json:"Images"`
	Ratings           []radarrRating         `json:"Ratings"`
	MovieRatings      *radarrRatings         `json:"MovieRatings"`
	Genres            []string               `json:"Genres"`
	Keywords          []string               `json:"Keywords"`
	Studio            string                 `json:"Studio"`
	Certifications    []radarrCertificate    `json:"Certifications"`
	Collection        *radarrCollection      `json:"Collection"`
	Credits           radarrCredits          `json:"Credits"`
	AlternativeTitles []radarrAltTitle       `json:"AlternativeTitles"`
	Translations      []radarrTranslation    `json:"Translations"`
	Recommendations   []radarrRecommendation `json:"Recommendations"`
}

type radarrImage struct {
	CoverType string `json:"CoverType"`
	URL       string `json:"Url"`
}

type radarrRating struct {
	Count int     `json:"Count"`
	Value float64 `json:"Value"`
}

type radarrRatings struct {
	Tmdb, Imdb, Metacritic, RottenTomatoes, Trakt *radarrRating
}

type radarrCertificate struct {
	Country       string `json:"Country"`
	Certification string `json:"Certification"`
}

type radarrCollection struct {
	TmdbID int    `json:"TmdbId"`
	Name   string `json:"Name"`
}

type radarrCredits struct {
	Cast []radarrCredit `json:"Cast"`
	Crew []radarrCredit `json:"Crew"`
}

// radarrCredit is a CastResource or a CrewResource.
type radarrCredit struct {
	Name       string        `json:"Name"`
	Order      int           `json:"Order"`
	Character  string        `json:"Character"`
	Job        string        `json:"Job"`
	Department string        `json:"Department"`
	TmdbID     int           `json:"TmdbId"`
	Images     []radarrImage `json:"Images"`
}

type radarrAltTitle struct {
	Title string `json:"Title"`
}

type radarrTranslation struct {
	Title    string `json:"Title"`
	Overview string `json:"Overview"`
	// Language is a language tag such as "de-DE".
	Language string `json:"Language"`
}

type radarrRecommendation struct {
	TmdbID int `json:"TmdbId"`
}

func (p *plugin) movie(ctx context.Context, tmdbID int) (radarrMovie, error) {
	var m radarrMovie
	return m, p.radarr.get(ctx, "/movie/"+strconv.Itoa(tmdbID), nil, &m)
}

// movieByIMDb finds a movie by its IMDb ID, as GetMovieByImdbId does.
func (p *plugin) movieByIMDb(ctx context.Context, imdbID string) (radarrMovie, error) {
	imdbID = normalizeIMDb(imdbID)
	if imdbID == "" {
		return radarrMovie{}, errNotFound
	}
	var ms []radarrMovie
	if err := p.radarr.get(ctx, "/movie/imdb/"+imdbID, nil, &ms); err != nil {
		return radarrMovie{}, err
	}
	if len(ms) == 0 {
		return radarrMovie{}, errNotFound
	}
	return ms[0], nil
}

// moviesByID fetches movies by their TMDB IDs, as GetBulkMovieInfo does.
func (p *plugin) moviesByID(ctx context.Context, ids []int) ([]radarrMovie, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var ms []radarrMovie
	return ms, p.radarr.post(ctx, "/movie/bulk", ids, &ms)
}

// findMovie fetches the movie the external IDs name.
func (p *plugin) findMovie(ctx context.Context, ids map[string]string) (radarrMovie, error) {
	if id, err := strconv.Atoi(ids[keyTMDB]); err == nil && id > 0 {
		return p.movie(ctx, id)
	}
	if ids[keyIMDb] != "" {
		return p.movieByIMDb(ctx, ids[keyIMDb])
	}
	return radarrMovie{}, errNotFound
}

var imdbID = regexp.MustCompile(`^tt\d{7,}$`)

// normalizeIMDb is NormalizeImdbId: "tt" and at least seven digits, or
// empty.
func normalizeIMDb(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	if !strings.HasPrefix(id, "tt") {
		id = "tt" + id
	}
	if !imdbID.MatchString(id) {
		return ""
	}
	return id
}

// searchMovies finds movies by their IDs, else by title and year as
// SearchForNewMovie does.
func (p *plugin) searchMovies(ctx context.Context, l *pluginv1.Lookup) ([]radarrMovie, error) {
	switch m, err := p.findMovie(ctx, l.GetExternalIds()); {
	case err == nil:
		return []radarrMovie{m}, nil
	case !errors.Is(err, errNotFound):
		return nil, err
	}
	// The name is parsed already, as Radarr's parser result is, whose
	// periods become spaces.
	title := stripTrailingThe(strings.ReplaceAll(strings.ToLower(strings.TrimSpace(l.GetName())), ".", " "))
	term := strings.NewReplacer("_", "+", " ", "+", ".", "+").Replace(title)
	if term == "" {
		return nil, nil
	}
	q := url.Values{"q": {term}, "year": {""}}
	if l.GetYear() > 1800 {
		q.Set("year", strconv.Itoa(int(l.GetYear())))
	}
	var ms []radarrMovie
	return ms, p.radarr.get(ctx, "/search", q, &ms)
}

func stripTrailingThe(title string) string {
	if t, ok := strings.CutSuffix(title, ",the"); ok {
		return t
	}
	if t, ok := strings.CutSuffix(title, ", the"); ok {
		return t
	}
	return title
}

func (m radarrMovie) titles() []string {
	out := []string{m.Title, m.OriginalTitle}
	for _, a := range m.AlternativeTitles {
		out = append(out, a.Title)
	}
	return out
}

func (m radarrMovie) searchResult(name string, year int) *pluginv1.SearchResult {
	r := pluginv1.SearchResult_builder{
		Name:        proto.String(m.Title),
		Overview:    proto.String(m.Overview),
		ExternalIds: m.externalIDs(),
		Score:       proto.Float64(matchScore(name, year, m.titles(), m.Year)),
	}.Build()
	if m.Year > 0 {
		r.SetYear(int32(m.Year))
	}
	if u := imageOf(m.Images, "poster"); u != "" {
		r.SetImageUrl(u)
	}
	return r
}

func (m radarrMovie) externalIDs() map[string]string {
	ids := map[string]string{keyTMDB: strconv.Itoa(m.TmdbID)}
	if m.ImdbID != "" {
		ids[keyIMDb] = m.ImdbID
	}
	if m.Collection != nil && m.Collection.TmdbID > 0 {
		ids[keyTMDBCollection] = strconv.Itoa(m.Collection.TmdbID)
	}
	return ids
}

// premiere is the movie's release: in cinemas, else its premiere, else
// its digital or physical release.
func (m radarrMovie) premiere() *time.Time {
	for _, t := range []*time.Time{m.InCinema, m.Premier, m.DigitalRelease, m.PhysicalRelease} {
		if t != nil {
			return t
		}
	}
	return nil
}

// tmdbRating is TMDB's rating, as MapMovie takes it.
func (m radarrMovie) tmdbRating() *radarrRating {
	if m.MovieRatings != nil && m.MovieRatings.Tmdb != nil {
		return m.MovieRatings.Tmdb
	}
	if len(m.Ratings) > 0 {
		return &m.Ratings[0]
	}
	return nil
}

// metadata maps the movie for a lookup's language and country.
func (m radarrMovie) metadata(language, country string) *pluginv1.Metadata {
	title, overview := m.Title, m.Overview
	if t, ok := m.translation(language, country); ok {
		title, overview = cmp.Or(t.Title, title), cmp.Or(t.Overview, overview)
	}
	md := pluginv1.Metadata_builder{
		Name:          proto.String(title),
		OriginalTitle: proto.String(m.OriginalTitle),
		Overview:      proto.String(overview),
		Genres:        m.Genres,
		ExternalIds:   m.externalIDs(),
		People:        m.people(),
		Images:        mapImages(m.Images),
	}.Build()
	if m.Year > 0 {
		md.SetProductionYear(int32(m.Year))
	}
	if t := m.premiere(); t != nil {
		md.SetPremiereDate(t.Format(time.DateOnly))
	}
	if m.Runtime > 0 {
		md.SetRuntime(durationpb.New(time.Duration(m.Runtime) * time.Minute))
	}
	country = cmp.Or(strings.ToUpper(country), "US")
	if i := slices.IndexFunc(m.Certifications, func(c radarrCertificate) bool { return c.Country == country }); i >= 0 {
		md.SetOfficialRating(m.Certifications[i].Certification)
	}
	if r := m.tmdbRating(); r != nil && r.Value > 0 {
		md.SetCommunityRating(r.Value)
	}
	if m.MovieRatings != nil && m.MovieRatings.RottenTomatoes != nil {
		md.SetCriticRating(m.MovieRatings.RottenTomatoes.Value)
	}
	var tags []string
	for _, k := range m.Keywords {
		if k = strings.TrimSpace(k); k != "" {
			tags = append(tags, k)
		}
	}
	md.SetTags(tags)
	if m.Studio != "" {
		md.SetStudios([]string{m.Studio})
	}
	if m.Collection != nil && m.Collection.Name != "" {
		md.SetCollectionName(m.Collection.Name)
	}
	return md
}

// translation finds the movie's translation into language, preferring
// the country's; English is the movie's own.
func (m radarrMovie) translation(language, country string) (radarrTranslation, bool) {
	language = strings.ToLower(language)
	if language == "" || language == "en" {
		return radarrTranslation{}, false
	}
	var found *radarrTranslation
	for i, t := range m.Translations {
		lang, region, _ := strings.Cut(t.Language, "-")
		if !strings.EqualFold(lang, language) {
			continue
		}
		if strings.EqualFold(region, country) {
			return t, true
		}
		if found == nil {
			found = &m.Translations[i]
		}
	}
	if found == nil {
		return radarrTranslation{}, false
	}
	return *found, true
}

// people maps the cast and the directors, writers and producers of the
// crew.
func (m radarrMovie) people() []*pluginv1.PersonCredit {
	var out []*pluginv1.PersonCredit
	for _, c := range m.Credits.Cast {
		if strings.TrimSpace(c.Name) == "" {
			continue
		}
		out = append(out, c.credit(pluginv1.CreditKind_CREDIT_KIND_ACTOR, c.Character))
	}
	type key struct {
		id   int
		kind pluginv1.CreditKind
	}
	seen := map[key]bool{}
	for _, c := range m.Credits.Crew {
		kind := crewKind(c.Department, c.Job)
		if kind == pluginv1.CreditKind_CREDIT_KIND_UNSPECIFIED || strings.TrimSpace(c.Name) == "" || seen[key{c.TmdbID, kind}] {
			continue
		}
		seen[key{c.TmdbID, kind}] = true
		out = append(out, c.credit(kind, c.Job))
	}
	return out
}

func (c radarrCredit) credit(kind pluginv1.CreditKind, role string) *pluginv1.PersonCredit {
	pc := pluginv1.PersonCredit_builder{
		Name:  proto.String(strings.TrimSpace(c.Name)),
		Kind:  kind.Enum(),
		Role:  proto.String(role),
		Order: proto.Int32(int32(c.Order)),
	}.Build()
	if c.TmdbID > 0 {
		pc.SetExternalIds(map[string]string{keyTMDB: strconv.Itoa(c.TmdbID)})
	}
	if u := imageOf(c.Images, "headshot"); u != "" {
		pc.SetImageUrl(u)
	}
	return pc
}

// crewKind is the kind of credit Jellyfin keeps of a crew member, as
// scraper-tmdb tells it.
func crewKind(department, job string) pluginv1.CreditKind {
	switch {
	case strings.EqualFold(department, "directing") && strings.EqualFold(job, "director"):
		return pluginv1.CreditKind_CREDIT_KIND_DIRECTOR
	case strings.EqualFold(department, "production") && strings.EqualFold(job, "producer"):
		return pluginv1.CreditKind_CREDIT_KIND_PRODUCER
	case strings.EqualFold(department, "writing") && slices.ContainsFunc([]string{"writer", "screenplay", "novel"}, func(j string) bool {
		return strings.EqualFold(j, job)
	}):
		return pluginv1.CreditKind_CREDIT_KIND_WRITER
	}
	return pluginv1.CreditKind_CREDIT_KIND_UNSPECIFIED
}

// imageKinds maps Servarr's cover types to image kinds.
var imageKinds = map[string]pluginv1.ImageKind{
	"poster":    pluginv1.ImageKind_IMAGE_KIND_PRIMARY,
	"fanart":    pluginv1.ImageKind_IMAGE_KIND_BACKDROP,
	"banner":    pluginv1.ImageKind_IMAGE_KIND_BANNER,
	"clearlogo": pluginv1.ImageKind_IMAGE_KIND_LOGO,
}

func mapImages(images []radarrImage) []*pluginv1.RemoteImage {
	var out []*pluginv1.RemoteImage
	for _, im := range images {
		kind, ok := imageKinds[strings.ToLower(im.CoverType)]
		if !ok || im.URL == "" {
			continue
		}
		out = append(out, pluginv1.RemoteImage_builder{Kind: kind.Enum(), Url: proto.String(im.URL)}.Build())
	}
	return out
}

// imageOf returns the URL of the first image of a cover type.
func imageOf(images []radarrImage, coverType string) string {
	for _, im := range images {
		if strings.EqualFold(im.CoverType, coverType) {
			return im.URL
		}
	}
	return ""
}

// matchScore scores a candidate against the lookup's name and year in
// [0, 1]: half for a title equal to the name, ignoring case and
// punctuation, half for the year, a quarter for a year off by one.
func matchScore(name string, year int, titles []string, candidateYear int) float64 {
	score := 0.0
	n := normalizeTitle(name)
	if slices.ContainsFunc(titles, func(t string) bool { return t != "" && normalizeTitle(t) == n }) {
		score += 0.5
	}
	switch d := year - candidateYear; {
	case year == 0 || candidateYear == 0:
	case d == 0:
		score += 0.5
	case d == 1 || d == -1:
		score += 0.25
	}
	return score
}

// normalizeTitle keeps a title's letters and digits, lowercased.
func normalizeTitle(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

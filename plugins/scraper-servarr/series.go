package main

import (
	"context"
	"errors"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// skyHookLanguage is the language SkyHook is asked in, as Sonarr asks.
const skyHookLanguage = "en"

// skyShow is SkyHook's ShowResource.
type skyShow struct {
	TvdbID            int        `json:"tvdbId"`
	Title             string     `json:"title"`
	Overview          string     `json:"overview"`
	FirstAired        string     `json:"firstAired"`
	LastAired         string     `json:"lastAired"`
	TvRageID          int        `json:"tvRageId"`
	TvMazeID          int        `json:"tvMazeId"`
	TmdbID            int        `json:"tmdbId"`
	ImdbID            string     `json:"imdbId"`
	Status            string     `json:"status"`
	Runtime           int        `json:"runtime"`
	Network           string     `json:"network"`
	OriginalLanguage  string     `json:"originalLanguage"`
	Genres            []string   `json:"genres"`
	ContentRating     string     `json:"contentRating"`
	Rating            *skyRating `json:"rating"`
	AlternativeTitles []struct {
		Title string `json:"title"`
	} `json:"alternativeTitles"`
	Actors   []skyActor    `json:"actors"`
	Images   []radarrImage `json:"images"`
	Seasons  []skySeason   `json:"seasons"`
	Episodes []skyEpisode  `json:"episodes"`
}

type skyRating struct {
	Count int    `json:"count"`
	Value number `json:"value"`
}

type skyActor struct {
	Name      string `json:"name"`
	Character string `json:"character"`
	Image     string `json:"image"`
}

type skySeason struct {
	SeasonNumber int           `json:"seasonNumber"`
	Images       []radarrImage `json:"images"`
}

// skyEpisode is SkyHook's EpisodeResource, numbered in TheTVDB's aired
// order.
type skyEpisode struct {
	TvdbID                int        `json:"tvdbId"`
	SeasonNumber          int        `json:"seasonNumber"`
	EpisodeNumber         int        `json:"episodeNumber"`
	AbsoluteEpisodeNumber int        `json:"absoluteEpisodeNumber"`
	Title                 string     `json:"title"`
	AirDate               string     `json:"airDate"`
	Runtime               int        `json:"runtime"`
	Overview              string     `json:"overview"`
	Image                 string     `json:"image"`
	Rating                *skyRating `json:"rating"`
}

// show fetches a series by its TVDB ID, as GetSeriesInfo does.
func (p *plugin) show(ctx context.Context, tvdbID int) (skyShow, error) {
	if s, ok := p.shows.get(tvdbID, p.clock()); ok {
		return s, nil
	}
	var s skyShow
	if err := p.skyHook.get(ctx, "/shows/"+skyHookLanguage+"/"+strconv.Itoa(tvdbID), nil, &s); err != nil {
		return skyShow{}, err
	}
	p.shows.put(tvdbID, s, p.clock())
	return s, nil
}

// searchShows searches SkyHook with a term: a title, or "imdb:<ID>" or
// "tmdb:<ID>", as SearchForNewSeries does.
func (p *plugin) searchShows(ctx context.Context, term string) ([]skyShow, error) {
	term = strings.ToLower(strings.TrimSpace(term))
	if term == "" {
		return nil, nil
	}
	var shows []skyShow
	return shows, p.skyHook.get(ctx, "/search/"+skyHookLanguage+"/", url.Values{"term": {term}}, &shows)
}

// findShow fetches the series the external IDs name: by its TVDB ID, else
// the first series SkyHook finds by its IMDb or TMDB ID.
func (p *plugin) findShow(ctx context.Context, ids map[string]string) (skyShow, error) {
	if id, err := strconv.Atoi(ids[keyTVDB]); err == nil && id > 0 {
		return p.show(ctx, id)
	}
	var terms []string
	if id := normalizeIMDb(ids[keyIMDb]); id != "" {
		terms = append(terms, "imdb:"+id)
	}
	if id, err := strconv.Atoi(ids[keyTMDB]); err == nil && id > 0 {
		terms = append(terms, "tmdb:"+strconv.Itoa(id))
	}
	for _, term := range terms {
		shows, err := p.searchShows(ctx, term)
		if err != nil {
			return skyShow{}, err
		}
		if len(shows) > 0 {
			return p.show(ctx, shows[0].TvdbID)
		}
	}
	return skyShow{}, errNotFound
}

// searchSeries finds series by their IDs, else by title.
func (p *plugin) searchSeries(ctx context.Context, l *pluginv1.Lookup) ([]skyShow, error) {
	switch s, err := p.findShow(ctx, l.GetExternalIds()); {
	case err == nil:
		return []skyShow{s}, nil
	case !errors.Is(err, errNotFound):
		return nil, err
	}
	return p.searchShows(ctx, l.GetName())
}

func (s skyShow) year() int {
	if t, err := time.Parse(time.DateOnly, s.FirstAired); err == nil {
		return t.Year()
	}
	return 0
}

func (s skyShow) titles() []string {
	out := []string{s.Title}
	for _, a := range s.AlternativeTitles {
		out = append(out, a.Title)
	}
	return out
}

func (s skyShow) searchResult(name string, year int) *pluginv1.SearchResult {
	r := pluginv1.SearchResult_builder{
		Name:        proto.String(s.Title),
		Overview:    proto.String(s.Overview),
		ExternalIds: s.externalIDs(),
		Score:       proto.Float64(matchScore(name, year, s.titles(), s.year())),
	}.Build()
	if y := s.year(); y > 0 {
		r.SetYear(int32(y))
	}
	if u := imageOf(s.Images, "poster"); u != "" {
		r.SetImageUrl(u)
	}
	return r
}

func (s skyShow) externalIDs() map[string]string {
	ids := map[string]string{keyTVDB: strconv.Itoa(s.TvdbID)}
	if s.ImdbID != "" {
		ids[keyIMDb] = s.ImdbID
	}
	if s.TmdbID > 0 {
		ids[keyTMDB] = strconv.Itoa(s.TmdbID)
	}
	if s.TvRageID > 0 {
		ids[keyTVRage] = strconv.Itoa(s.TvRageID)
	}
	if s.TvMazeID > 0 {
		ids[keyTVMaze] = strconv.Itoa(s.TvMazeID)
	}
	return ids
}

// status maps the series' status as MapSeriesStatus does.
func (s skyShow) status() pluginv1.SeriesStatus {
	switch {
	case strings.EqualFold(s.Status, "ended"):
		return pluginv1.SeriesStatus_SERIES_STATUS_ENDED
	case strings.EqualFold(s.Status, "upcoming"):
		return pluginv1.SeriesStatus_SERIES_STATUS_UNRELEASED
	}
	return pluginv1.SeriesStatus_SERIES_STATUS_CONTINUING
}

func (s skyShow) metadata() *pluginv1.Metadata {
	md := pluginv1.Metadata_builder{
		Name:         proto.String(s.Title),
		Overview:     proto.String(s.Overview),
		Genres:       s.Genres,
		ExternalIds:  s.externalIDs(),
		SeriesStatus: s.status().Enum(),
		Images:       mapImages(s.Images),
	}.Build()
	if y := s.year(); y > 0 {
		md.SetProductionYear(int32(y))
		md.SetPremiereDate(s.FirstAired)
	}
	if s.status() == pluginv1.SeriesStatus_SERIES_STATUS_ENDED && s.LastAired != "" {
		md.SetEndDate(s.LastAired)
	}
	if s.Runtime > 0 {
		md.SetRuntime(durationpb.New(time.Duration(s.Runtime) * time.Minute))
	}
	if s.ContentRating != "" {
		md.SetOfficialRating(strings.ToUpper(s.ContentRating))
	}
	if s.Rating != nil && s.Rating.Value > 0 {
		md.SetCommunityRating(float64(s.Rating.Value))
	}
	if s.Network != "" {
		md.SetStudios([]string{s.Network})
	}
	var people []*pluginv1.PersonCredit
	for i, a := range s.Actors {
		if strings.TrimSpace(a.Name) == "" {
			continue
		}
		pc := pluginv1.PersonCredit_builder{
			Name: proto.String(strings.TrimSpace(a.Name)), Kind: pluginv1.CreditKind_CREDIT_KIND_ACTOR.Enum(),
			Role: proto.String(a.Character), Order: proto.Int32(int32(i)),
		}.Build()
		if a.Image != "" {
			pc.SetImageUrl(a.Image)
		}
		people = append(people, pc)
	}
	md.SetPeople(people)
	return md
}

// season returns the series' season, which is found when the series lists
// it or has episodes in it.
func (s skyShow) season(number int) (*pluginv1.Metadata, bool) {
	i := slices.IndexFunc(s.Seasons, func(se skySeason) bool { return se.SeasonNumber == number })
	first := s.firstAired(number)
	if i < 0 && first == "" {
		return nil, false
	}
	md := &pluginv1.Metadata{}
	if i >= 0 {
		md.SetImages(mapImages(s.Seasons[i].Images))
	}
	if first != "" {
		md.SetPremiereDate(first)
		if t, err := time.Parse(time.DateOnly, first); err == nil {
			md.SetProductionYear(int32(t.Year()))
		}
	}
	return md, true
}

// firstAired is the earliest air date of a season's episodes.
func (s skyShow) firstAired(season int) string {
	first := ""
	for _, e := range s.Episodes {
		if e.SeasonNumber == season && e.AirDate != "" && (first == "" || e.AirDate < first) {
			first = e.AirDate
		}
	}
	return first
}

// episode finds an episode by its TVDB ID, else by its numbers.
func (s skyShow) episode(tvdbID, season, number int) (skyEpisode, bool) {
	i := slices.IndexFunc(s.Episodes, func(e skyEpisode) bool {
		if tvdbID > 0 {
			return e.TvdbID == tvdbID
		}
		return e.SeasonNumber == season && e.EpisodeNumber == number
	})
	if i < 0 {
		return skyEpisode{}, false
	}
	return s.Episodes[i], true
}

func (e skyEpisode) metadata() *pluginv1.Metadata {
	md := pluginv1.Metadata_builder{
		Name:     proto.String(e.Title),
		Overview: proto.String(e.Overview),
	}.Build()
	if e.TvdbID > 0 {
		md.SetExternalIds(map[string]string{keyTVDB: strconv.Itoa(e.TvdbID)})
	}
	if t, err := time.Parse(time.DateOnly, e.AirDate); err == nil {
		md.SetPremiereDate(e.AirDate)
		md.SetProductionYear(int32(t.Year()))
	}
	if e.Runtime > 0 {
		md.SetRuntime(durationpb.New(time.Duration(e.Runtime) * time.Minute))
	}
	if e.Rating != nil && e.Rating.Value > 0 {
		md.SetCommunityRating(float64(e.Rating.Value))
	}
	if e.Image != "" {
		md.SetImages([]*pluginv1.RemoteImage{pluginv1.RemoteImage_builder{
			Kind: pluginv1.ImageKind_IMAGE_KIND_PRIMARY.Enum(), Url: proto.String(e.Image),
		}.Build()})
	}
	return md
}

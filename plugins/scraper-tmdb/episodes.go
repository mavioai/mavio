package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strconv"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// groupTypes are the types of TMDB's episode groups by display order.
var groupTypes = map[string]int{
	"originalAirDate": 1,
	"absolute":        2,
	"dvd":             3,
	"digital":         4,
	"storyArc":        5,
	"production":      6,
	"tv":              7,
}

// The subset of TMDB's responses that list episodes.
type (
	tmdbOutline struct {
		ID      int `json:"id"`
		Seasons []struct {
			ID           int    `json:"id"`
			Name         string `json:"name"`
			Overview     string `json:"overview"`
			AirDate      string `json:"air_date"`
			SeasonNumber int    `json:"season_number"`
			PosterPath   string `json:"poster_path"`
		} `json:"seasons"`
		EpisodeGroups struct {
			Results []struct {
				ID   string `json:"id"`
				Type int    `json:"type"`
			} `json:"results"`
		} `json:"episode_groups"`
	}
	tmdbListedEpisode struct {
		ID            int     `json:"id"`
		Name          string  `json:"name"`
		Overview      string  `json:"overview"`
		AirDate       string  `json:"air_date"`
		Runtime       int     `json:"runtime"`
		VoteAverage   float64 `json:"vote_average"`
		SeasonNumber  int     `json:"season_number"`
		EpisodeNumber int     `json:"episode_number"`
		StillPath     string  `json:"still_path"`
		// Order is the episode's position in its episode group, from 0.
		Order int `json:"order"`
	}
	tmdbSeasonEpisodes struct {
		Episodes []tmdbListedEpisode `json:"episodes"`
	}
	tmdbEpisodeGroup struct {
		Groups []tmdbGroupSeason `json:"groups"`
	}
	tmdbGroupSeason struct {
		Name string `json:"name"`
		// Order is the season's number.
		Order    int                 `json:"order"`
		Episodes []tmdbListedEpisode `json:"episodes"`
	}
)

func (c *client) outline(ctx context.Context, id int, language string) (tmdbOutline, error) {
	var o tmdbOutline
	q := url.Values{"append_to_response": {"episode_groups"}}
	setLanguage(q, language)
	return o, c.get(ctx, "/tv/"+strconv.Itoa(id), q, &o)
}

func (c *client) seasonEpisodes(ctx context.Context, seriesID, season int, language string) (tmdbSeasonEpisodes, error) {
	var s tmdbSeasonEpisodes
	q := url.Values{}
	setLanguage(q, language)
	return s, c.get(ctx, fmt.Sprintf("/tv/%d/season/%d", seriesID, season), q, &s)
}

func (c *client) episodeGroup(ctx context.Context, id, language string) (tmdbEpisodeGroup, error) {
	var g tmdbEpisodeGroup
	q := url.Values{}
	setLanguage(q, language)
	return g, c.get(ctx, "/tv/episode_group/"+url.PathEscape(id), q, &g)
}

// group returns the series' episode group of a display order, as Jellyfin
// picks it: the first of the order's type. It returns nil for the aired
// order and when the series has no such group.
func (s session) group(ctx context.Context, seriesID int, order, language string) (*tmdbEpisodeGroup, error) {
	if _, ok := groupTypes[order]; !ok {
		return nil, nil
	}
	o, err := s.outline(ctx, seriesID, language)
	if err != nil {
		return nil, err
	}
	return s.groupOf(ctx, o, order, language)
}

func (s session) groupOf(ctx context.Context, o tmdbOutline, order, language string) (*tmdbEpisodeGroup, error) {
	typ, ok := groupTypes[order]
	if !ok {
		return nil, nil
	}
	for _, g := range o.EpisodeGroups.Results {
		if g.Type == typ {
			eg, err := s.episodeGroup(ctx, g.ID, language)
			if err != nil {
				return nil, err
			}
			return &eg, nil
		}
	}
	return nil, nil
}

// airedNumbers maps a season and episode number of a display order to the
// aired ones, as Jellyfin does: the group's season with the number as its
// order, and its episode with the number less one as its order. Numbers
// the group lacks are taken as aired numbers.
func (g *tmdbEpisodeGroup) airedNumbers(season, episode int) (int, int) {
	if g == nil {
		return season, episode
	}
	for _, s := range g.Groups {
		if s.Order != season {
			continue
		}
		for _, e := range s.Episodes {
			if e.Order == episode-1 {
				return e.SeasonNumber, e.EpisodeNumber
			}
		}
	}
	return season, episode
}

func (p *plugin) ListEpisodes(ctx context.Context, req *pluginv1.ListEpisodesRequest) (*pluginv1.ListEpisodesResponse, error) {
	s, err := p.session()
	if err != nil {
		return nil, err
	}
	l := req.GetLookup()
	r := requestOf(l)
	resp := pluginv1.ListEpisodesResponse_builder{Found: proto.Bool(false)}.Build()
	id, err := s.seriesID(ctx, l.GetExternalIds(), l.GetName(), int(l.GetYear()), r)
	if err != nil || id == 0 {
		return resp, err
	}
	seasons, episodes, err := s.listEpisodes(ctx, id, l.GetDisplayOrder(), r)
	if errors.Is(err, errNotFound) {
		return resp, nil
	}
	if err != nil {
		return nil, err
	}
	resp.SetFound(true)
	resp.SetProvider(keyTMDB)
	resp.SetSeasons(seasons)
	resp.SetEpisodes(episodes)
	return resp, nil
}

// listEpisodes lists a series' seasons and episodes in a display order:
// those of its episode group, or those TMDB lists by season.
func (s session) listEpisodes(ctx context.Context, id int, order string, r request) ([]*pluginv1.ListedSeason, []*pluginv1.ListedEpisode, error) {
	o, err := s.outline(ctx, id, r.language)
	if err != nil {
		return nil, nil, err
	}
	g, err := s.groupOf(ctx, o, order, r.language)
	if err != nil {
		return nil, nil, err
	}
	if g != nil {
		seasons, episodes := groupEpisodes(*g, order == "absolute")
		return seasons, episodes, nil
	}
	var seasons []*pluginv1.ListedSeason
	var episodes []*pluginv1.ListedEpisode
	for _, ts := range o.Seasons {
		md := pluginv1.Metadata_builder{
			Name:        proto.String(ts.Name),
			Overview:    proto.String(ts.Overview),
			ExternalIds: map[string]string{keyTMDB: strconv.Itoa(ts.ID)},
		}.Build()
		setDate(md, ts.AirDate)
		if ts.PosterPath != "" {
			md.SetImages([]*pluginv1.RemoteImage{remoteImage(pluginv1.ImageKind_IMAGE_KIND_PRIMARY, ts.PosterPath)})
		}
		seasons = append(seasons, pluginv1.ListedSeason_builder{SeasonNumber: proto.Int32(int32(ts.SeasonNumber)), Metadata: md}.Build())
		se, err := s.seasonEpisodes(ctx, id, ts.SeasonNumber, r.language)
		if errors.Is(err, errNotFound) {
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		for _, e := range se.Episodes {
			episodes = append(episodes, listedEpisode(e, ts.SeasonNumber, e.EpisodeNumber, 0))
		}
	}
	return seasons, episodes, nil
}

// groupEpisodes lists the seasons and episodes of an episode group, with
// absolute numbers counted across its seasons but specials when absolute.
func groupEpisodes(g tmdbEpisodeGroup, absolute bool) ([]*pluginv1.ListedSeason, []*pluginv1.ListedEpisode) {
	groups := slices.Clone(g.Groups)
	slices.SortStableFunc(groups, func(a, b tmdbGroupSeason) int { return cmp.Compare(a.Order, b.Order) })
	var seasons []*pluginv1.ListedSeason
	var episodes []*pluginv1.ListedEpisode
	n := 0
	for _, s := range groups {
		seasons = append(seasons, pluginv1.ListedSeason_builder{
			SeasonNumber: proto.Int32(int32(s.Order)),
			Metadata:     pluginv1.Metadata_builder{Name: proto.String(s.Name)}.Build(),
		}.Build())
		eps := slices.Clone(s.Episodes)
		slices.SortStableFunc(eps, func(a, b tmdbListedEpisode) int { return cmp.Compare(a.Order, b.Order) })
		for _, e := range eps {
			abs := 0
			if absolute && s.Order > 0 {
				n++
				abs = n
			}
			episodes = append(episodes, listedEpisode(e, s.Order, e.Order+1, abs))
		}
	}
	return seasons, episodes
}

// listedEpisode lists an episode under its numbers in the display order;
// absolute is 0 when unknown.
func listedEpisode(e tmdbListedEpisode, season, episode, absolute int) *pluginv1.ListedEpisode {
	md := pluginv1.Metadata_builder{
		Name:        proto.String(e.Name),
		Overview:    proto.String(e.Overview),
		ExternalIds: map[string]string{keyTMDB: strconv.Itoa(e.ID)},
	}.Build()
	if e.VoteAverage > 0 {
		md.SetCommunityRating(e.VoteAverage)
	}
	setDate(md, e.AirDate)
	setRuntime(md, e.Runtime)
	if e.StillPath != "" {
		md.SetImages([]*pluginv1.RemoteImage{remoteImage(pluginv1.ImageKind_IMAGE_KIND_PRIMARY, e.StillPath)})
	}
	le := pluginv1.ListedEpisode_builder{
		SeasonNumber:  proto.Int32(int32(season)),
		EpisodeNumber: proto.Int32(int32(episode)),
		Metadata:      md,
	}.Build()
	if absolute > 0 {
		le.SetAbsoluteNumber(int32(absolute))
	}
	return le
}

func remoteImage(kind pluginv1.ImageKind, path string) *pluginv1.RemoteImage {
	return pluginv1.RemoteImage_builder{Kind: kind.Enum(), Url: proto.String(imageURL(path))}.Build()
}

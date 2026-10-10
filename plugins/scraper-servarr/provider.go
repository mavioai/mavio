package main

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"slices"
	"strconv"

	"google.golang.org/protobuf/proto"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func (p *plugin) Search(ctx context.Context, req *pluginv1.SearchRequest) (*pluginv1.SearchResponse, error) {
	l := req.GetLookup()
	name, year := l.GetName(), int(l.GetYear())
	var results []*pluginv1.SearchResult
	switch l.GetKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MOVIE:
		movies, err := p.searchMovies(ctx, l)
		if err != nil {
			return nil, err
		}
		for _, m := range movies {
			results = append(results, m.searchResult(name, year))
		}
	case pluginv1.MediaKind_MEDIA_KIND_SERIES:
		shows, err := p.searchSeries(ctx, l)
		if err != nil {
			return nil, err
		}
		for _, s := range shows {
			results = append(results, s.searchResult(name, year))
		}
	}
	// Best match first; equal scores keep the service's order.
	slices.SortStableFunc(results, func(a, b *pluginv1.SearchResult) int { return cmp.Compare(b.GetScore(), a.GetScore()) })
	if limit := int(req.GetLimit()); limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return pluginv1.SearchResponse_builder{Results: results}.Build(), nil
}

func (p *plugin) GetMetadata(ctx context.Context, req *pluginv1.GetMetadataRequest) (*pluginv1.GetMetadataResponse, error) {
	l := req.GetLookup()
	md, err := p.metadata(ctx, l)
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

func (p *plugin) metadata(ctx context.Context, l *pluginv1.Lookup) (*pluginv1.Metadata, error) {
	switch l.GetKind() {
	case pluginv1.MediaKind_MEDIA_KIND_MOVIE:
		m, err := p.findMovie(ctx, l.GetExternalIds())
		if errors.Is(err, errNotFound) {
			m, err = p.identifyMovie(ctx, l)
		}
		if err != nil {
			return nil, err
		}
		return m.metadata(l.GetLanguage(), l.GetCountry()), nil
	case pluginv1.MediaKind_MEDIA_KIND_SERIES:
		s, err := p.findShow(ctx, l.GetExternalIds())
		if errors.Is(err, errNotFound) {
			s, err = p.identifyShow(ctx, l)
		}
		if err != nil {
			return nil, err
		}
		return s.metadata(), nil
	case pluginv1.MediaKind_MEDIA_KIND_SEASON:
		s, err := p.findShow(ctx, l.GetSeriesExternalIds())
		if err != nil {
			return nil, err
		}
		md, ok := s.season(int(l.GetSeasonNumber()))
		if !ok {
			return nil, errNotFound
		}
		return md, nil
	case pluginv1.MediaKind_MEDIA_KIND_EPISODE:
		s, err := p.findShow(ctx, l.GetSeriesExternalIds())
		if err != nil {
			return nil, err
		}
		id, _ := strconv.Atoi(l.GetExternalIds()[keyTVDB])
		e, ok := s.episode(id, int(l.GetSeasonNumber()), int(l.GetEpisodeNumber()))
		if !ok {
			return nil, errNotFound
		}
		return e.metadata(), nil
	}
	return nil, errNotFound
}

// identifyMovie finds a movie without known IDs by its name and year: the
// search result whose title matches best, if any title matches.
func (p *plugin) identifyMovie(ctx context.Context, l *pluginv1.Lookup) (radarrMovie, error) {
	movies, err := p.searchMovies(ctx, l)
	if err != nil {
		return radarrMovie{}, err
	}
	i := bestMatch(l.GetName(), int(l.GetYear()), len(movies), func(i int) ([]string, int) { return movies[i].titles(), movies[i].Year })
	if i < 0 {
		return radarrMovie{}, errNotFound
	}
	return movies[i], nil
}

// identifyShow finds a series without known IDs like identifyMovie.
func (p *plugin) identifyShow(ctx context.Context, l *pluginv1.Lookup) (skyShow, error) {
	if l.GetName() == "" {
		return skyShow{}, errNotFound
	}
	shows, err := p.searchShows(ctx, l.GetName())
	if err != nil {
		return skyShow{}, err
	}
	i := bestMatch(l.GetName(), int(l.GetYear()), len(shows), func(i int) ([]string, int) { return shows[i].titles(), shows[i].year() })
	if i < 0 {
		return skyShow{}, errNotFound
	}
	// Search results lack the series' episodes.
	return p.show(ctx, shows[i].TvdbID)
}

// bestMatch returns the index of the one of n candidates, given by their
// titles and year, that matches name and year best, the first on ties;
// -1 when no title matches name.
func bestMatch(name string, year, n int, candidate func(i int) ([]string, int)) int {
	best, bestScore := -1, 0.0
	for i := range n {
		titles, y := candidate(i)
		if !slices.ContainsFunc(titles, func(t string) bool { return t != "" && normalizeTitle(t) == normalizeTitle(name) }) {
			continue
		}
		if s := matchScore(name, year, titles, y); best < 0 || s > bestScore {
			best, bestScore = i, s
		}
	}
	return best
}

// ListEpisodes lists a series' seasons and episodes as SkyHook numbers
// them, in TheTVDB's aired order, whatever the display order: releases
// and files are numbered that way.
func (p *plugin) ListEpisodes(ctx context.Context, req *pluginv1.ListEpisodesRequest) (*pluginv1.ListEpisodesResponse, error) {
	s, err := p.findShow(ctx, req.GetLookup().GetExternalIds())
	if errors.Is(err, errNotFound) {
		return pluginv1.ListEpisodesResponse_builder{Found: proto.Bool(false)}.Build(), nil
	}
	if err != nil {
		return nil, err
	}
	numbers := map[int]bool{}
	for _, se := range s.Seasons {
		numbers[se.SeasonNumber] = true
	}
	for _, e := range s.Episodes {
		numbers[e.SeasonNumber] = true
	}
	var seasons []*pluginv1.ListedSeason
	for _, n := range slices.Sorted(maps.Keys(numbers)) {
		md, _ := s.season(n)
		seasons = append(seasons, pluginv1.ListedSeason_builder{SeasonNumber: proto.Int32(int32(n)), Metadata: md}.Build())
	}
	episodes := make([]*pluginv1.ListedEpisode, 0, len(s.Episodes))
	for _, e := range s.Episodes {
		le := pluginv1.ListedEpisode_builder{
			SeasonNumber: proto.Int32(int32(e.SeasonNumber)), EpisodeNumber: proto.Int32(int32(e.EpisodeNumber)),
			Metadata: e.metadata(),
		}.Build()
		if e.AbsoluteEpisodeNumber > 0 {
			le.SetAbsoluteNumber(int32(e.AbsoluteEpisodeNumber))
		}
		episodes = append(episodes, le)
	}
	return pluginv1.ListEpisodesResponse_builder{
		Found: proto.Bool(true), Provider: proto.String(keyTVDB), Seasons: seasons, Episodes: episodes,
	}.Build(), nil
}

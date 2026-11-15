package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// TMDB searches no networks, so the discovery filter of networks lists
// those series are best known by, named as TMDB names them, and finds any
// other by its ID.

// knownNetworks are TMDB's IDs of the networks most series air on, with
// their names, which the query matches before TMDB names them.
var knownNetworks = []tmdbFilterValue{
	{ID: 213, Name: "Netflix"},
	{ID: 49, Name: "HBO"},
	{ID: 3186, Name: "HBO Max"},
	{ID: 1024, Name: "Prime Video"},
	{ID: 2739, Name: "Disney+"},
	{ID: 2552, Name: "Apple TV+"},
	{ID: 453, Name: "Hulu"},
	{ID: 4330, Name: "Paramount+"},
	{ID: 3353, Name: "Peacock"},
	{ID: 2, Name: "ABC"},
	{ID: 6, Name: "NBC"},
	{ID: 16, Name: "CBS"},
	{ID: 19, Name: "FOX"},
	{ID: 71, Name: "The CW"},
	{ID: 174, Name: "AMC"},
	{ID: 88, Name: "FX"},
	{ID: 67, Name: "Showtime"},
	{ID: 318, Name: "Starz"},
	{ID: 41, Name: "TNT"},
	{ID: 30, Name: "USA Network"},
	{ID: 77, Name: "Syfy"},
	{ID: 64, Name: "Discovery"},
	{ID: 43, Name: "National Geographic"},
	{ID: 54, Name: "Disney Channel"},
	{ID: 56, Name: "Cartoon Network"},
	{ID: 80, Name: "Adult Swim"},
	{ID: 13, Name: "Nickelodeon"},
	{ID: 47, Name: "Comedy Central"},
	{ID: 4, Name: "BBC One"},
	{ID: 332, Name: "BBC Two"},
	{ID: 9, Name: "ITV1"},
	{ID: 26, Name: "Channel 4"},
	{ID: 1, Name: "Fuji TV"},
}

// networkCache keeps the networks TMDB named, by ID: they change rarely,
// and the known ones are listed whenever the filter is.
type networkCache struct {
	mu   sync.Mutex
	byID map[int]tmdbFilterValue
}

func (c *networkCache) get(id int) (tmdbFilterValue, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.byID[id]
	return v, ok
}

func (c *networkCache) put(v tmdbFilterValue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.byID == nil {
		c.byID = map[int]tmdbFilterValue{}
	}
	c.byID[v.ID] = v
}

// networks lists the networks of the IDs given, or else the network whose
// ID the query is, or the known networks whose names contain the query,
// all of them without one, named and with their logos as TMDB has them.
func (s session) networks(ctx context.Context, query string, ids []string) ([]tmdbFilterValue, error) {
	if len(ids) > 0 {
		return s.networksByID(ctx, ids)
	}
	query = strings.TrimSpace(query)
	if id, ok := networkID(query); ok {
		return s.networksByID(ctx, []string{strconv.Itoa(id)})
	}
	var out []tmdbFilterValue
	q := strings.ToLower(query)
	for _, known := range knownNetworks {
		v, err := s.network(ctx, known.ID)
		switch {
		case errors.Is(err, errNotFound):
			// Kept as known, not to be asked again.
			v = known
			s.networkCache.put(v)
		case err != nil:
			return nil, err
		}
		if q == "" || strings.Contains(strings.ToLower(known.Name), q) || strings.Contains(strings.ToLower(v.Name), q) {
			out = append(out, v)
		}
	}
	return out, nil
}

// networkID reads a query naming a network by its TMDB ID, as "49" or
// "tmdb:49".
func networkID(query string) (int, bool) {
	id, err := strconv.Atoi(strings.TrimPrefix(strings.ToLower(query), "tmdb:"))
	return id, err == nil && id > 0
}

func (s session) networksByID(ctx context.Context, ids []string) ([]tmdbFilterValue, error) {
	var out []tmdbFilterValue
	for _, id := range ids {
		n, ok := parseTMDBID(id)
		if !ok {
			continue
		}
		switch v, err := s.network(ctx, n); {
		case errors.Is(err, errNotFound):
		case err != nil:
			return nil, err
		default:
			out = append(out, v)
		}
	}
	return out, nil
}

// network looks a network up by ID, once.
func (s session) network(ctx context.Context, id int) (tmdbFilterValue, error) {
	if v, ok := s.networkCache.get(id); ok {
		return v, nil
	}
	var v tmdbFilterValue
	if err := s.get(ctx, fmt.Sprintf("/network/%d", id), nil, &v); err != nil {
		return tmdbFilterValue{}, err
	}
	v.ID = id
	s.networkCache.put(v)
	return v, nil
}

// tmdbCertification is a content rating of TMDB's certification lists.
type tmdbCertification struct {
	Certification string `json:"certification"`
	Order         int    `json:"order"`
}

// certifications lists the content ratings of a country's rating system
// for movies ("movie") or series ("tv"), from the least restricted;
// nothing for a country TMDB has none of, or none.
func (s session) certifications(ctx context.Context, kind, country string) ([]tmdbCertification, error) {
	if country == "" {
		return nil, nil
	}
	var list struct {
		Certifications map[string][]tmdbCertification `json:"certifications"`
	}
	if err := s.get(ctx, "/certification/"+kind+"/list", nil, &list); err != nil {
		return nil, err
	}
	certs := list.Certifications[strings.ToUpper(country)]
	slices.SortStableFunc(certs, func(a, b tmdbCertification) int { return cmp.Compare(a.Order, b.Order) })
	return certs, nil
}

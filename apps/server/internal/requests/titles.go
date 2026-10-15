package requests

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
)

// Title is what the metadata providers tell of a movie or series by its
// TMDB ID.
type Title struct {
	Kind   core.ItemKind
	TmdbID int
	// TvdbID is a series' TVDB ID, 0 when unknown.
	TvdbID int
	Name   string
	Year   int
	// Rating is its content rating in Country, such as "PG-13"; empty
	// when unrated.
	Rating  string
	Country string

	// The details of a title's page, from the first provider giving
	// each.
	OriginalTitle   string
	Overview        string
	PremiereDate    *time.Time
	Genres          []string
	Tags            []string
	CommunityRating float64
	ExternalIDs     map[core.Provider]string
	PosterURL       string
	BackdropURL     string
}

// animeKeyword is the TMDB keyword of anime, which the TMDB provider
// gives as a tag: Seerr's ANIME_KEYWORD_ID (210024).
const animeKeyword = "anime"

// Anime reports whether a title is anime, as Seerr tells anime series by
// their TMDB keyword.
func (t *Title) Anime() bool {
	return slices.ContainsFunc(t.Tags, func(tag string) bool { return strings.EqualFold(tag, animeKeyword) })
}

// titleTTL is how long looked up titles are kept.
const titleTTL = 6 * time.Hour

// titleCacheSize bounds the titles kept; a full cache is emptied.
const titleCacheSize = 10000

// titleCache keeps looked up titles, by kind, TMDB ID and country.
type titleCache struct {
	mu      sync.Mutex
	entries map[string]cachedTitle
}

type cachedTitle struct {
	title Title
	at    time.Time
}

func (c *titleCache) get(key string, now time.Time) (Title, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[key]
	if !ok || now.Sub(e.at) > titleTTL {
		return Title{}, false
	}
	return e.title, true
}

func (c *titleCache) put(key string, t Title, now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.entries == nil || len(c.entries) >= titleCacheSize {
		c.entries = map[string]cachedTitle{}
	}
	c.entries[key] = cachedTitle{t, now}
}

// region is the country and language titles are looked up and rated in:
// the server's metadata settings, as Seerr's region and locale settings,
// else the US and English.
func (s *Service) region() (country, language string) {
	var m core.MetadataSettings
	if s.MetadataSettings != nil {
		m = s.MetadataSettings()
	}
	return cmp.Or(m.Country, "US"), cmp.Or(m.PreferredLanguage, "en")
}

// Title looks a movie or series up by its TMDB ID through the metadata
// providers, in the country requests of its kind go to: the first
// provider knowing it names it, the first giving a rating rates it.
// Unknown titles fail with core.ErrNotFound.
func (s *Service) Title(ctx context.Context, kind core.ItemKind, tmdbID int) (Title, error) {
	if kind != core.KindMovie && kind != core.KindSeries {
		return Title{}, fmt.Errorf("%w: request of kind %q", core.ErrInvalid, kind)
	}
	country, language := s.region()
	key := fmt.Sprintf("%s:%d:%s:%s", kind, tmdbID, country, language)
	if t, ok := s.cache.get(key, s.now()); ok {
		return t, nil
	}
	t := Title{Kind: kind, TmdbID: tmdbID, Country: country, ExternalIDs: map[core.Provider]string{}}
	found := false
	var providers []library.Provider
	if s.Metadata != nil {
		providers = s.Metadata()
	}
	for _, p := range providers {
		res, err := p.Metadata(ctx, library.Lookup{
			Kind: kind, ExternalIDs: map[core.Provider]string{core.ProviderTMDB: strconv.Itoa(tmdbID)},
			Language: language, Country: country,
		})
		if errors.Is(err, library.ErrProviderUnavailable) {
			continue
		}
		if err != nil {
			if ctx.Err() != nil {
				return Title{}, ctx.Err()
			}
			s.logger().WarnContext(ctx, "couldn't look title up", "provider", p.Name(), "tmdb", tmdbID, "err", err)
			continue
		}
		if res == nil {
			continue
		}
		found = true
		it := &res.Item
		if t.Name == "" {
			t.Name = it.Name
		}
		if t.Year == 0 {
			t.Year = yearOf(it)
		}
		if t.Rating == "" {
			t.Rating = it.OfficialRating
		}
		if t.TvdbID == 0 {
			t.TvdbID, _ = strconv.Atoi(it.ExternalIDs[core.ProviderTVDB])
		}
		t.OriginalTitle = cmp.Or(t.OriginalTitle, it.OriginalTitle)
		t.Overview = cmp.Or(t.Overview, it.Overview)
		if t.PremiereDate == nil {
			t.PremiereDate = it.PremiereDate
		}
		if len(t.Genres) == 0 {
			t.Genres = it.Genres
		}
		if len(t.Tags) == 0 {
			t.Tags = it.Tags
		}
		t.CommunityRating = cmp.Or(t.CommunityRating, it.CommunityRating)
		for p, id := range it.ExternalIDs {
			if _, ok := t.ExternalIDs[p]; !ok && id != "" {
				t.ExternalIDs[p] = id
			}
		}
		for _, img := range res.RemoteImages {
			switch {
			case img.Kind == core.ImagePrimary && t.PosterURL == "":
				t.PosterURL = img.URL
			case img.Kind == core.ImageBackdrop && t.BackdropURL == "":
				t.BackdropURL = img.URL
			}
		}
	}
	t.ExternalIDs[core.ProviderTMDB] = strconv.Itoa(tmdbID)
	if !found {
		return Title{}, fmt.Errorf("%w: no metadata provider knows the %s with TMDB ID %d", core.ErrNotFound, kind, tmdbID)
	}
	s.cache.put(key, t, s.now())
	return t, nil
}

func yearOf(it *core.Item) int {
	if it.ProductionYear > 0 {
		return it.ProductionYear
	}
	if it.PremiereDate != nil {
		return it.PremiereDate.Year()
	}
	return 0
}

// Seasons lists the numbers of a series' seasons that have episodes,
// through the episode listers, as Seerr counts the seasons of "all".
func (s *Service) Seasons(ctx context.Context, t Title) ([]int, error) {
	infos, err := s.seasonInfos(ctx, t)
	if err != nil {
		return nil, err
	}
	out := make([]int, len(infos))
	for i, si := range infos {
		out[i] = si.number
	}
	return out, nil
}

// seasonInfo is what the episode listers tell of a season.
type seasonInfo struct {
	number, episodes int
	// premiere is its first episode's air date, if known.
	premiere *time.Time
}

// seasonInfos lists a series' seasons that have episodes, by number,
// through the first episode lister knowing it.
func (s *Service) seasonInfos(ctx context.Context, t Title) ([]seasonInfo, error) {
	var listers []library.EpisodeLister
	if s.Episodes != nil {
		listers = s.Episodes()
	}
	ids := map[core.Provider]string{core.ProviderTMDB: strconv.Itoa(t.TmdbID)}
	if t.TvdbID > 0 {
		ids[core.ProviderTVDB] = strconv.Itoa(t.TvdbID)
	}
	_, language := s.region()
	for _, l := range listers {
		list, err := l.Episodes(ctx, library.Lookup{Kind: core.KindSeries, ExternalIDs: ids, Language: language, Country: t.Country})
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			s.logger().WarnContext(ctx, "couldn't list seasons", "lister", l.Name(), "tmdb", t.TmdbID, "err", err)
			continue
		}
		if list == nil {
			continue
		}
		byNumber := map[int]*seasonInfo{}
		for _, ep := range list.Episodes {
			n := ep.Item.ParentIndexNumber
			if n == nil {
				continue
			}
			si := byNumber[*n]
			if si == nil {
				si = &seasonInfo{number: *n}
				byNumber[*n] = si
			}
			si.episodes++
			if d := ep.Item.PremiereDate; d != nil && (si.premiere == nil || d.Before(*si.premiere)) {
				si.premiere = d
			}
		}
		out := make([]seasonInfo, 0, len(byNumber))
		for _, si := range byNumber {
			out = append(out, *si)
		}
		slices.SortFunc(out, func(a, b seasonInfo) int { return cmp.Compare(a.number, b.number) })
		return out, nil
	}
	return nil, fmt.Errorf("%w: no episode lister knows the series with TMDB ID %d", core.ErrNotFound, t.TmdbID)
}

// RatingAllowed reports whether a user's rating limit allows a content
// rating of a country, as the user's policy allows items: unrated
// content passes unless the policy blocks it.
func RatingAllowed(u *core.User, rating, country string) bool {
	access := u.EffectiveAccess()
	limit := access.MaxParentalRating
	if limit == nil {
		return true
	}
	score, ok := metadata.RatingScore(rating, country)
	if !ok {
		return !access.BlockUnrated
	}
	return score <= *limit
}

// Certifications returns the content ratings of a country a user's rating
// limit allows, for discovery providers filtering by certification; nil
// when the user has no limit.
func Certifications(u *core.User, country string) []string {
	limit := u.EffectiveAccess().MaxParentalRating
	if limit == nil {
		return nil
	}
	var out []string
	for _, r := range metadata.ParentalRatings(country) {
		if r.Score != nil && *r.Score <= *limit {
			out = append(out, r.Name)
		}
	}
	return out
}

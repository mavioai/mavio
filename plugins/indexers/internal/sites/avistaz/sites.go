package avistaz

import (
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
)

// Definitions are the AvistaZ sites.
var Definitions = []indexer.Definition{
	animeZDefinition(),
	definition(indexer.Definition{ID: "avistaz", Name: "AvistaZ", Description: "Aka AsiaTorrents"}, avistaZ),
	definition(indexer.Definition{
		ID: "cinemaz", Name: "CinemaZ",
		Description: "CinemaZ (EuTorrents) is a Private Torrent Tracker for FOREIGN NON-ASIAN MOVIES.",
	}, cinemaZ),
	definition(indexer.Definition{
		ID: "exoticaz", Name: "ExoticaZ",
		Description: "ExoticaZ (YourExotic) is a Private Torrent Tracker for 3X",
	}, exoticaZ),
	definition(indexer.Definition{
		ID: "privatehd", Name: "PrivateHD",
		Description: "PrivateHD (PHD) is a Private Torrent Tracker for HD MOVIES / TV and the sister-site of AvistaZ, CinemaZ, ExoticaZ, and AnimeZ",
	}, privateHD),
}

var (
	idTV    = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp, indexer.ParamIMDbID, indexer.ParamTVDBID, indexer.ParamGenre}
	idMovie = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID, indexer.ParamTMDBID, indexer.ParamGenre}
)

var avistaZ = site{
	links: []string{"https://avistaz.to/"},
	caps:  func() *indexer.Capabilities { return baseCapabilities(idTV, idMovie) },
	// AvistaZ has episodes without seasons, such as Running Man E323.
	episode: func(q *indexer.Query) string {
		if (q.Season == nil || *q.Season == 0) && strings.TrimSpace(q.Episode) != "" {
			return "E" + q.Episode
		}
		return q.EpisodeString()
	},
}

var cinemaZ = site{
	links: []string{"https://cinemaz.to/"},
	caps: func() *indexer.Capabilities {
		return baseCapabilities(
			[]indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp, indexer.ParamIMDbID, indexer.ParamGenre},
			[]indexer.Param{indexer.ParamQ, indexer.ParamIMDbID, indexer.ParamGenre})
	},
}

var privateHD = site{
	links: []string{"https://privatehd.to/"},
	caps:  func() *indexer.Capabilities { return baseCapabilities(idTV, idMovie) },
}

var exoticaZ = site{
	links: []string{"https://exoticaz.to/"},
	caps: func() *indexer.Capabilities {
		c := indexer.NewCapabilities()
		c.LimitsDefault, c.LimitsMax = pageSize, pageSize
		c.Categories.Add("1", indexer.XXXx264, "Video Clip")
		c.Categories.Add("2", indexer.XXXPack, "Video Pack")
		c.Categories.Add("3", indexer.XXXPack, "Siterip Pack")
		c.Categories.Add("4", indexer.XXXPack, "Pornstar Pack")
		c.Categories.Add("5", indexer.XXXDVD, "DVD")
		c.Categories.Add("6", indexer.XXXx264, "BluRay")
		c.Categories.Add("7", indexer.XXXImageSet, "Photo Pack")
		c.Categories.Add("8", indexer.XXXImageSet, "Books & Magazines")
		return c
	},
	// ExoticaZ's results name their categories by ID.
	categories: func(s *Site, r *result) ([]int, error) {
		var cats []int
		for _, id := range slices.Sorted(maps.Keys(r.Category)) {
			cats = append(cats, indexer.IDs(s.caps.Categories.MapTracker(id))...)
		}
		return cats, nil
	},
}

var animeZ = site{
	links: []string{"https://animez.to/"},
	caps: func() *indexer.Capabilities {
		c := indexer.NewCapabilities()
		c.LimitsDefault, c.LimitsMax = pageSize, pageSize
		c.Params[indexer.SearchTV] = []indexer.Param{indexer.ParamQ, indexer.ParamSeason, indexer.ParamEp}
		c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ}
		c.Params[indexer.SearchBook] = []indexer.Param{indexer.ParamQ}
		c.Categories.Add("TV", indexer.TVAnime, "Anime > TV")
		c.Categories.Add("TV_SHORT", indexer.TVAnime, "Anime > TV Short")
		c.Categories.Add("MOVIE", indexer.Movies, "Anime > Movie")
		c.Categories.Add("SPECIAL", indexer.TVAnime, "Anime > Special")
		c.Categories.Add("OVA", indexer.TVAnime, "Anime > OVA")
		c.Categories.Add("ONA", indexer.TVAnime, "Anime > ONA")
		c.Categories.Add("MUSIC", indexer.TVAnime, "Anime > Music")
		c.Categories.Add("MANGA", indexer.BooksComics, "Manga > Manga")
		c.Categories.Add("NOVEL", indexer.BooksForeign, "Manga > Novel")
		c.Categories.Add("ONE_SHOT", indexer.BooksForeign, "Manga > One-Shot")
		return c
	},
	// AnimeZ filters by formats instead of types and video qualities.
	params: func(s *Site, q *indexer.Query, _ string) *query {
		p := &query{}
		p.add("limit", limit(q))
		for _, c := range s.caps.Categories.MapToTracker(q.Categories, false) {
			p.add("format[]", c)
		}
		if q.Limit > 0 && q.Offset > 0 {
			p.add("page", strconv.Itoa(q.Offset/q.Limit+1))
		}
		if s.settings.FreeleechOnly {
			p.add("freeleech", "1")
		}
		return p
	},
	categories: func(s *Site, r *result) ([]int, error) {
		return indexer.IDs(s.caps.Categories.MapTracker(string(r.Format))), nil
	},
	title: func(r *result) string {
		if strings.TrimSpace(string(r.ReleaseTitle)) != "" {
			return string(r.ReleaseTitle)
		}
		return string(r.FileName)
	},
}

// animeZSite is AnimeZ, which downloads with the API's token.
type animeZSite struct{ *Site }

func animeZDefinition() indexer.Definition {
	d := definition(indexer.Definition{
		ID: "animez", Name: "AnimeZ",
		Description: "AnimeZ (ex-AnimeTorrents) is a Private Torrent Tracker for ANIME / MANGA",
	}, animeZ)
	newSite := d.New
	d.New = func(data []byte) (indexer.Site, error) {
		s, err := newSite(data)
		if err != nil {
			return nil, err
		}
		return animeZSite{s.(*Site)}, nil
	}
	return d
}

// DownloadRequest implements indexer.DownloadSite.
func (s animeZSite) DownloadRequest(_ context.Context, ix *indexer.Indexer, link string) (*indexer.Request, error) {
	r := indexer.NewRequest(link, indexer.AcceptJSON)
	r.Header.Set("Authorization", "Bearer "+s.getToken(ix))
	r.Cookies = map[string]string{}
	return r, nil
}

// ReloginOnUnauthorizedDownload implements indexer.DownloadRelogger.
func (animeZSite) ReloginOnUnauthorizedDownload() {}

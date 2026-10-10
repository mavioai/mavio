package gazelle

import (
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var secretCinema = site{
	links: []string{"https://secret-cinema.pw/"},
	info:  baseInfo,
	caps: func() *indexer.Capabilities {
		c := indexer.NewCapabilities()
		c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
		c.Params[indexer.SearchMusic] = []indexer.Param{indexer.ParamQ, indexer.ParamArtist, indexer.ParamAlbum, indexer.ParamLabel, indexer.ParamYear}
		c.Categories.Add("1", indexer.Movies, "Movies")
		c.Categories.Add("2", indexer.Audio, "Music")
		return c
	},
	parse: parseSecretCinema,
}

var (
	secretCinemaYear = regexp.MustCompile(`(\b|[-._ ])((?:19|20)\d{2})(\b|[-._ ])`)
	bdmv             = regexp.MustCompile(`(?i)\bBDMV\b`)
	sd               = regexp.MustCompile(`(?i)\bSD\b`)
	// secretCinemaZone is the site's time zone.
	secretCinemaZone = time.FixedZone("", 2*60*60)
)

// parseSecretCinema reads Secret Cinema's results, as Prowlarr's
// SecretCinemaParser does: movies are titled by their release names or
// their groups, music by artist and album; times are UTC+2.
func parseSecretCinema(s *Site, r *indexer.Response) ([]*indexer.Release, error) {
	var resp response[group]
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("secret cinema: %w", err)
	}
	if resp.Status != "success" || resp.Response == nil {
		return nil, nil
	}
	var releases []*indexer.Release
	for _, g := range resp.Response.Results {
		if g.Torrents == nil {
			date, err := parse.Int(string(g.GroupTime))
			if err != nil {
				return nil, fmt.Errorf("secret cinema: group time %q: %w", g.GroupTime, err)
			}
			freeleech := bool(g.IsFreeLeech || g.IsNeutralLeech || g.IsPersonalFreeLeech)
			rel, err := s.release(&g, &g.torrentStats, freeleech, html.UnescapeString(string(g.GroupName)), time.Unix(date, 0).UTC(), g.Category)
			if err != nil {
				return nil, err
			}
			rel.DownloadURL = secretCinemaDownloadURL(s, int(g.TorrentID))
			releases = append(releases, rel)
			continue
		}
		for _, t := range g.Torrents {
			// Directors are the artists of movies, and titles their
			// groups' names.
			artist := html.UnescapeString(string(g.Artist))
			title := html.UnescapeString(string(g.GroupName))
			t0 := t.Time.Time
			date := time.Date(t0.Year(), t0.Month(), t0.Day(), t0.Hour(), t0.Minute(), t0.Second(), t0.Nanosecond(), secretCinemaZone).UTC()
			freeleech := bool(t.IsFreeLeech || t.IsNeutralLeech || t.IsPersonalFreeLeech)
			rel, err := s.release(&g, &t.torrentStats, freeleech, title, date, t.Category)
			if err != nil {
				return nil, err
			}
			rel.DownloadURL = secretCinemaDownloadURL(s, int(t.TorrentID))
			rel.Scene = bool(t.Scene)
			if isMovie(rel.Categories) {
				if name := string(t.ReleaseName); strings.TrimSpace(name) != "" && secretCinemaYear.MatchString(name) {
					rel.Title = strings.TrimSpace(html.UnescapeString(name))
				} else {
					rel.Title = strings.TrimSpace(fmt.Sprintf("%s (%s) %s", title, g.GroupYear, t.Media))
					if rt := string(t.RemasterTitle); strings.TrimSpace(rt) != "" {
						rel.Title += " / " + strings.TrimSpace(html.UnescapeString(rt))
					}
					rel.Title = bdmv.ReplaceAllString(rel.Title, "COMPLETE BLURAY")
					rel.Title = sd.ReplaceAllString(rel.Title, "DVDRip")
				}
			} else {
				rel.Title = fmt.Sprintf("%s - %s (%s) [%s %s] [%s]", artist, title, g.GroupYear, t.Format, t.Encoding, t.Media)
				rel.Artist, rel.Album = artist, title
			}
			if t.HasCue {
				rel.Title += " [Cue]"
			}
			releases = append(releases, rel)
		}
	}
	SortByDate(releases)
	return releases, nil
}

func isMovie(categories []int) bool {
	return slices.ContainsFunc(categories, func(id int) bool {
		return id == indexer.Movies.ID || slices.ContainsFunc(indexer.Movies.Sub, func(c *indexer.Category) bool { return c.ID == id })
	})
}

// secretCinemaDownloadURL returns the link downloading a torrent; Secret
// Cinema takes useToken whenever tokens are used.
func secretCinemaDownloadURL(s *Site, id int) string {
	u := indexer.AddQueryParam(indexer.AddQueryParam(indexer.CombinePath(s.baseURL, "/torrents.php"), "action", "download"), "id", strconv.Itoa(id))
	if s.settings.UseFreeleechToken != TokenNever {
		u = indexer.AddQueryParam(u, "useToken", "1")
	}
	return u
}

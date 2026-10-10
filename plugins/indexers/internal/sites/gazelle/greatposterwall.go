package gazelle

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/indexer"
	"github.com/mavioai/mavio/plugins/indexers/internal/jsonx"
	"github.com/mavioai/mavio/plugins/indexers/internal/parse"
)

var greatPosterWall = site{
	links:  []string{"https://greatposterwall.com/"},
	cookie: true,
	info:   baseInfo,
	caps: func() *indexer.Capabilities {
		c := indexer.NewCapabilities()
		c.Params[indexer.SearchMovie] = []indexer.Param{indexer.ParamQ, indexer.ParamIMDbID}
		c.Categories.Add("1", indexer.Movies, "Movies 电影")
		return c
	},
	params: func(s *Site, p *indexer.Params, _ *indexer.Query) {
		if s.settings.FreeleechOnly {
			p.Set("freetorrent", "1")
		}
	},
	// Movies are searched by the term as given, or by IMDb ID.
	movieParams: func(s *Site, q *indexer.Query) *indexer.Params {
		p := s.BasicParams(q, q.Term)
		if q.IMDbID != "" {
			p.Set("searchstr", q.FullIMDbID())
		}
		return p
	},
	check: checkGreatPosterWall,
	parse: parseGreatPosterWall,
	// Recent releases are cut to a page.
	cleanup: func(releases []*indexer.Release, q *indexer.Query) []*indexer.Release {
		if q.IsRSS() && len(releases) > 50 {
			return releases[:50]
		}
		return releases
	},
}

// gpwGroup is a release group of GreatPosterWall.
type gpwGroup struct {
	GroupID  jsonx.Int    `json:"groupId"`
	Cover    jsonx.String `json:"cover"`
	IMDbID   jsonx.String `json:"imdbId"`
	Torrents []struct {
		TorrentID           jsonx.Int    `json:"torrentId"`
		Scene               jsonx.Bool   `json:"scene"`
		FileCount           jsonx.Int    `json:"fileCount"`
		Time                jsonx.Time   `json:"time"`
		Size                jsonx.Int64  `json:"size"`
		Snatches            jsonx.Int    `json:"snatches"`
		Seeders             jsonx.Int    `json:"seeders"`
		Leechers            jsonx.Int    `json:"leechers"`
		IsFreeleech         jsonx.Bool   `json:"isFreeleech"`
		IsNeutralLeech      jsonx.Bool   `json:"isNeutralLeech"`
		FreeType            jsonx.String `json:"freeType"`
		IsPersonalFreeleech jsonx.Bool   `json:"isPersonalFreeleech"`
		CanUseToken         jsonx.Bool   `json:"canUseToken"`
		Resolution          jsonx.String `json:"resolution"`
		FileName            jsonx.String `json:"fileName"`
	} `json:"torrents"`
}

// gpwZone is GreatPosterWall's time zone, China's.
var gpwZone = time.FixedZone("", 8*60*60)

// gpwFreeTypes are the download and upload factors of GreatPosterWall's
// free types.
var gpwFreeTypes = map[string][2]float64{
	"11": {0.75, 1},
	"12": {0.5, 1},
	"13": {0.25, 1},
	"1":  {0, 1},
	"2":  {0, 0},
}

// parseGreatPosterWall reads GreatPosterWall's results, as Prowlarr's
// GreatPosterWallParser does: titled by file name, categorized by
// resolution, with times in China's zone.
func parseGreatPosterWall(s *Site, r *indexer.Response) ([]*indexer.Release, error) {
	var resp response[gpwGroup]
	if err := json.Unmarshal([]byte(r.Content()), &resp); err != nil {
		return nil, fmt.Errorf("greatposterwall: %w", err)
	}
	if resp.Status != "success" || resp.Response == nil {
		return nil, nil
	}
	var releases []*indexer.Release
	for _, g := range resp.Response.Results {
		for _, t := range g.Torrents {
			freeleech := bool(t.IsFreeleech || t.IsNeutralLeech || t.IsPersonalFreeleech)
			if s.settings.UseFreeleechToken == TokenRequired && !bool(t.CanUseToken) && !freeleech {
				continue
			}
			info := s.InfoURL(strconv.Itoa(int(g.GroupID)), int(t.TorrentID))
			t0 := t.Time.Time
			rel := &indexer.Release{
				GUID:                 info,
				InfoURL:              info,
				DownloadURL:          s.DownloadURL(int(t.TorrentID), bool(t.CanUseToken) && !freeleech),
				Title:                strings.TrimSpace(html.UnescapeString(string(t.FileName))),
				Protocol:             indexer.ProtocolTorrent,
				PosterURL:            s.posterURL(string(g.Cover)),
				PublishDate:          time.Date(t0.Year(), t0.Month(), t0.Day(), t0.Hour(), t0.Minute(), t0.Second(), t0.Nanosecond(), gpwZone).UTC(),
				Categories:           []int{indexer.Movies.ID, gpwResolutionCategory(string(t.Resolution))},
				Size:                 indexer.Ptr(int64(t.Size)),
				Seeders:              indexer.Ptr(int(t.Seeders)),
				Peers:                indexer.Ptr(int(t.Seeders + t.Leechers)),
				Grabs:                indexer.Ptr(int(t.Snatches)),
				Files:                indexer.Ptr(int(t.FileCount)),
				Scene:                bool(t.Scene),
				DownloadVolumeFactor: indexer.Ptr(volumeFactor(freeleech)),
				UploadVolumeFactor:   indexer.Ptr(volumeFactor(bool(t.IsNeutralLeech))),
				MinimumRatio:         indexer.Ptr(1.0),
				MinimumSeedTime:      indexer.Ptr(int64(172800)),
			}
			if id, ok := parse.IMDbID(string(g.IMDbID)); ok {
				rel.IMDbID = id
			}
			if f, ok := gpwFreeTypes[string(t.FreeType)]; ok {
				rel.DownloadVolumeFactor = indexer.Ptr(f[0])
				if f[1] == 0 {
					rel.UploadVolumeFactor = indexer.Ptr(0.0)
				}
			}
			releases = append(releases, rel)
		}
	}
	SortByDate(releases)
	return releases, nil
}

func gpwResolutionCategory(resolution string) int {
	switch resolution {
	case "1080p", "1080i", "720p":
		return indexer.MoviesHD.ID
	case "2160p":
		return indexer.MoviesUHD.ID
	}
	return indexer.MoviesSD.ID
}

// checkGreatPosterWall fails responses as Prowlarr's
// GreatPosterWallParser does: the indexer's cookies are only forgotten on
// redirects to the login page.
func checkGreatPosterWall(_ *Site, ix *indexer.Indexer, r *indexer.Response) error {
	if r.StatusCode != http.StatusOK {
		if r.IsRedirect() {
			to := r.RedirectURL()
			if strings.Contains(strings.ToLower(to), "/login.php") {
				ix.SetCookies(nil, time.Time{})
				return indexer.Errorf("We are being redirected to the login page. Most likely your session expired or was killed. Recheck your cookie or credentials and try testing the indexer.")
			}
			return indexer.Errorf("Redirected to %s from indexer request", to)
		}
		return statusError(r)
	}
	if !r.HasContentType(indexer.AcceptJSON) {
		return contentTypeError(r)
	}
	return nil
}

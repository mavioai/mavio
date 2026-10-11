package acquisition

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/acquire/organize"
	"github.com/mavioai/mavio/libs/acquire/series"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/release"
)

func TestAggregateEpisodeQuality(t *testing.T) {
	video := func(w, h int) *library.ProbeResult {
		return &library.ProbeResult{Source: core.MediaSource{Streams: []core.MediaStream{{Kind: core.StreamAudio}, {Kind: core.StreamVideo, Width: w, Height: h}}}}
	}
	parse := release.ParseSeriesTitle
	for _, tt := range []struct {
		name                   string
		file, folder, download string
		probe                  *library.ProbeResult
		want                   release.Quality
	}{
		{name: "file name", file: "Show.S01E01.720p.HDTV.x264-GRP", download: "Show.S01.1080p.WEB-DL-GRP", want: release.HDTV720p},
		{name: "resolution from the video", file: "Show.S01E01.HDTV.x264-GRP", probe: video(1280, 720), want: release.HDTV720p},
		{name: "video over the name", file: "Show.S01E01.720p.WEB-DL-GRP", probe: video(1920, 1080), want: release.WEBDL1080p},
		{name: "source from the folder", file: "Show.S01E01.1080p", folder: "Show.S01E01.1080p.WEB-DL-GRP", want: release.WEBDL1080p},
		{name: "grabbed release only", download: "Show.S01E01.720p.WEB-DL-GRP", want: release.WEBDL720p},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var file, folder, download *release.ParsedEpisodeInfo
			if tt.file != "" {
				file = parse(tt.file)
			}
			if tt.folder != "" {
				folder = parse(tt.folder)
			}
			if tt.download != "" {
				download = parse(tt.download)
			}
			if got := aggregateQuality(file, folder, download, tt.probe, release.ParseQuality(tt.download, release.Series)); got.Quality != tt.want {
				t.Errorf("aggregateQuality() = %v, want = %v", got.Quality, tt.want)
			}
		})
	}
}

// Ports Sonarr's EpisodeTitleSpecificationFixture.
func TestMissingEpisodeTitle(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	naming := organize.DefaultSeriesNaming()
	naming.RenameEpisodes = true
	standard := organize.Series{Title: "30 Rock", Type: organize.SeriesStandard}
	episode := func(change func(*series.Episode)) []*series.Episode {
		ep := &series.Episode{SeasonNumber: 1, EpisodeNumber: 1, Title: "Title1", AirDateUTC: now}
		if change != nil {
			change(ep)
		}
		return []*series.Episode{ep}
	}
	// season returns five episodes of season 1 aired as air says.
	season := func(air func(i int) time.Time) []*series.Episode {
		var out []*series.Episode
		for i := range 5 {
			out = append(out, &series.Episode{SeasonNumber: 1, EpisodeNumber: i + 2, Title: "TBA", AirDateUTC: air(i)})
		}
		return out
	}
	differentDays := season(func(i int) time.Time { return now.AddDate(0, 0, -7*(i+1)) })
	sameDay := season(func(int) time.Time { return now })
	someWithoutAirDate := season(func(i int) time.Time {
		if i == 0 {
			return now
		}
		return time.Time{}
	})
	tba := func(ep *series.Episode) { ep.Title = "TBA" }
	for _, tt := range []struct {
		name     string
		required core.EpisodeTitleRequired
		naming   func(*organize.SeriesNaming)
		episodes []*series.Episode
		season   []*series.Episode
		want     string
	}{
		{name: "title is missing", episodes: episode(func(ep *series.Episode) { ep.Title = "" }), want: msgTitleMissing},
		{name: "title is TBA", episodes: episode(tba), want: msgTitleTBA},
		{name: "aired a week ago with a TBA title", episodes: episode(func(ep *series.Episode) { ep.Title, ep.AirDateUTC = "TBA", now.AddDate(0, 0, -7) })},
		{name: "title not in the format", episodes: episode(tba), naming: func(n *organize.SeriesNaming) { n.StandardEpisodeFormat = "S{season:00}E{episode:00}" }},
		{name: "not renaming", episodes: episode(tba), naming: func(n *organize.SeriesNaming) { n.RenameEpisodes = false }},
		{name: "never required", required: core.EpisodeTitleNever, episodes: episode(tba)},
		{name: "bulk season releases, not a bulk season", required: core.EpisodeTitleBulkSeasonReleases, episodes: episode(tba), season: differentDays},
		{name: "bulk season releases with a title", required: core.EpisodeTitleBulkSeasonReleases, episodes: episode(nil), season: sameDay},
		{name: "bulk season release without a title", required: core.EpisodeTitleBulkSeasonReleases, episodes: episode(tba), season: sameDay, want: msgTitleTBA},
		{name: "bulk season release, some without air dates", required: core.EpisodeTitleBulkSeasonReleases, episodes: episode(tba), season: someWithoutAirDate, want: msgTitleTBA},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n := naming
			if tt.naming != nil {
				tt.naming(&n)
			}
			required := tt.required
			if required == "" {
				required = core.EpisodeTitleAlways
			}
			all := append(append([]*series.Episode{}, tt.episodes...), tt.season...)
			if got := missingEpisodeTitle(required, n, standard, tt.episodes, all, now); got != tt.want {
				t.Errorf("missingEpisodeTitle() = %q, want = %q", got, tt.want)
			}
		})
	}
}

// Ports Sonarr's AbsoluteEpisodeNumberSpecificationFixture.
func TestMissingAbsoluteNumber(t *testing.T) {
	now := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	anime := organize.Series{Title: "30 Rock", Type: organize.SeriesAnime}
	naming := organize.DefaultSeriesNaming()
	naming.AnimeEpisodeFormat = "{Series Title} - {absolute:000} - {Episode Title}"
	for _, tt := range []struct {
		name     string
		absolute int
		aired    time.Time
		format   string
		series   organize.Series
		want     bool
	}{
		{name: "no absolute number", aired: now, want: true},
		{name: "aired a week ago", aired: now.AddDate(0, 0, -7)},
		{name: "absolute number not in the format", aired: now, format: "{Series Title} - S{season:00}E{episode:00}"},
		{name: "with an absolute number", absolute: 1, aired: now},
		{name: "not anime", aired: now, series: organize.Series{Title: "30 Rock", Type: organize.SeriesStandard}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			n, s := naming, anime
			if tt.format != "" {
				n.AnimeEpisodeFormat = tt.format
			}
			if tt.series.Title != "" {
				s = tt.series
			}
			eps := []*series.Episode{{SeasonNumber: 1, EpisodeNumber: 1, AbsoluteEpisodeNumber: tt.absolute, AirDateUTC: tt.aired}}
			if got := missingAbsoluteNumber(n, s, eps, now); got != tt.want {
				t.Errorf("missingAbsoluteNumber() = %v, want = %v", got, tt.want)
			}
		})
	}
}

// Ports Sonarr's UnverifiedSceneNumberingSpecification.
func TestUnverifiedSceneNumbering(t *testing.T) {
	if unverifiedSceneNumbering([]*series.Episode{{EpisodeNumber: 1}}) {
		t.Error("verified numbering rejected")
	}
	if !unverifiedSceneNumbering([]*series.Episode{{EpisodeNumber: 1}, {EpisodeNumber: 2, UnverifiedSceneNumbering: true}}) {
		t.Error("unverified numbering accepted")
	}
}

// Ports Sonarr's NotUnpackingSpecificationFixture.
func TestUnpacking(t *testing.T) {
	now := time.Now()
	dir := t.TempDir()
	file := func(rel string, modified time.Time) string {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, modified, modified); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if unpacking(file("Unsorted TV/30.rock/30.rock.s01e01.avi", now), now) {
		t.Error("file outside a working folder is being unpacked")
	}
	if !unpacking(file("Unsorted TV/_UNPACK_30.rock/someSubFolder/30.rock.s01e01.avi", now), now) {
		t.Error("file written just now in a working folder is not being unpacked")
	}
	old := file("Unsorted TV/_FAILED_30.rock/30.rock.s01e02.avi", now.Add(-time.Hour))
	if got, want := unpacking(old, now), runtime.GOOS != "windows"; got != want {
		t.Errorf("old file in a working folder being unpacked = %v, want = %v", got, want)
	}
}

// Ports Sonarr's GetSceneNameFixture.
func TestSceneNameOf(t *testing.T) {
	const (
		seasonName  = "series.title.s02.dvdrip.x264-ingot"
		episodeName = "series.title.s02e23.dvdrip.x264-ingot"
	)
	inSeries := "/test/tv/Series Title/Series Title - S02E23 - Episode Title.mkv"
	info := func(title string, fullSeason bool) *release.ParsedEpisodeInfo {
		return &release.ParsedEpisodeInfo{ReleaseTitle: title, FullSeason: fullSeason}
	}
	for _, tt := range []struct {
		name             string
		path             string
		single           bool
		download, folder *release.ParsedEpisodeInfo
		want             string
	}{
		{"download title", inSeries, true, info(episodeName, false), nil, episodeName},
		{"full season download", "/test/Unsorted TV/" + seasonName + "/" + episodeName, true, info(seasonName, true), nil, ""},
		{"download of other video files", "/test/Unsorted TV/" + seasonName + "/" + episodeName, false, info(seasonName, false), nil, ""},
		{"scene file name", "/test/Unsorted TV/" + seasonName + "/" + episodeName + ".mkv", true, nil, nil, episodeName},
		{"file name not a scene name", "/test/Unsorted TV/" + episodeName + "/aaaaa.mkv", true, nil, nil, ""},
		{"scene folder name", inSeries, true, nil, info(episodeName, false), episodeName},
		{"folder name not a scene name", "/test/Unsorted TV/" + episodeName + "/aaaaa.mkv", true, nil, info("aaaaa", false), ""},
		{"full season folder", "/test/Unsorted TV/" + episodeName + "/aaaaa.mkv", true, nil, info(seasonName, true), ""},
		{
			"batch", "/test/Unsorted TV/[HorribleSubs] Series Title (01-62) [1080p] (Batch)/[HorribleSubs] Series Title - 14 [1080p].mkv", false,
			info("[HorribleSubs] Series Title (01-62) [1080p] (Batch)", false), info(seasonName, false), "",
		},
		{"folder of other video files", "/test/Unsorted TV/" + episodeName + "/aaaaa.mkv", false, nil, info(seasonName, false), ""},
		{"download title with .mkv", inSeries, true, info(episodeName+".mkv", false), nil, episodeName},
		{"download title with .par2", inSeries, true, info(episodeName+".par2", false), nil, episodeName},
		{"download title with .nzb", inSeries, true, info(episodeName+".nzb", false), nil, episodeName},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := sceneNameOf(tt.path, tt.single, tt.download, tt.folder); got != tt.want {
				t.Errorf("sceneNameOf() = %q, want = %q", got, tt.want)
			}
		})
	}
}

// Ports Sonarr's MatchesFolderSpecificationFixture, with the episodes of
// the fixture series.
func TestUnexpectedEpisodes(t *testing.T) {
	e := newSeriesEnv(t)
	s := e.addSeries(81189, core.SeriesStandard)
	rn, err := e.rt.newRun(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	sh, err := rn.show(e.ctx, s.ID)
	if err != nil {
		t.Fatal(err)
	}
	ps, err := rn.parsing(e.ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name, folder, file, want string
	}{
		{name: "folder name not parsed", folder: "Some Folder", file: "Breaking.Bad.S01E01.720p.HDTV.x264-GRP"},
		{name: "file name not parsed", folder: "Breaking.Bad.S01E01.720p.HDTV.x264-GRP", file: "aaaaa"},
		{name: "full season", folder: "Breaking.Bad.S01.720p.HDTV.x264-GRP", file: "Breaking.Bad.S01E02.720p.HDTV.x264-GRP"},
		{name: "same episode", folder: "Breaking.Bad.S01E01.720p.HDTV.x264-GRP", file: "Breaking.Bad.S01E01.720p.HDTV.x264-GRP"},
		{name: "one episode of the folder", folder: "Breaking.Bad.S01E01E02.720p.HDTV.x264-GRP", file: "Breaking.Bad.S01E01.720p.HDTV.x264-GRP"},
		{
			name: "another episode", folder: "Breaking.Bad.S01E01.720p.HDTV.x264-GRP", file: "Breaking.Bad.S01E03.720p.HDTV.x264-GRP",
			want: "Episode 1x03 was unexpected considering the Breaking.Bad.S01E01.720p.HDTV.x264-GRP folder name",
		},
		{
			name: "other episodes", folder: "Breaking.Bad.S01E01.720p.HDTV.x264-GRP", file: "Breaking.Bad.S01E02E03.720p.HDTV.x264-GRP",
			want: "Episodes 1x02, 1x03 were unexpected considering the Breaking.Bad.S01E01.720p.HDTV.x264-GRP folder name",
		},
		{
			name: "another season", folder: "Breaking.Bad.S01.720p.HDTV.x264-GRP", file: "Breaking.Bad.S02E01.720p.HDTV.x264-GRP",
			want: "Episode 2x01 was unexpected considering the Breaking.Bad.S01.720p.HDTV.x264-GRP folder name",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got, err := unexpectedEpisodes(e.ctx, ps, sh.series, release.ParseSeriesTitle(tt.file), release.ParseSeriesTitle(tt.folder))
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Errorf("unexpectedEpisodes() = %q, want = %q", got, tt.want)
			}
		})
	}
}

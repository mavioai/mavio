package server_test

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/media/subsync"
	"github.com/mavioai/mavio/libs/media/vad"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/release"
	"github.com/mavioai/mavio/libs/subtitle"
	"github.com/mavioai/mavio/tools/fixtures"
)

// subtitlesPlugin is the subtitles plugin, for startAcquisition.
var subtitlesPlugin = processPlugin{name: "subtitles", source: subtitlesSource, build: subtitlesBuild}

// fakeSubsarr is a Subsarr whose searches list the subtitles offered for
// movies or for episodes, and which serves them.
type fakeSubsarr struct {
	*httptest.Server

	mu       sync.Mutex
	movies   []subsarrSubtitle
	episodes []subsarrSubtitle
	served   map[string]string
}

// subsarrSubtitle is a subtitle Subsarr lists.
type subsarrSubtitle struct {
	id, title, release, data string
}

func newFakeSubsarr(t *testing.T) *fakeSubsarr {
	f := &fakeSubsarr{served: map[string]string{}}
	f.Server = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeSubsarr) offer(episode bool, subs ...subsarrSubtitle) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, s := range subs {
		if episode {
			f.episodes = append(f.episodes, s)
		} else {
			f.movies = append(f.movies, s)
		}
		f.served["/download/"+s.id] = s.data
	}
}

// link is the ID the plugin gives a subtitle: its download link.
func (f *fakeSubsarr) link(s subsarrSubtitle) string { return f.URL + "/download/" + s.id }

func (f *fakeSubsarr) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r.URL.Path == "/api/v1/subtitles/search" {
		list := f.movies
		if r.URL.Query().Has("season") {
			list = f.episodes
		}
		var items []string
		for _, s := range list {
			items = append(items, fmt.Sprintf(`{"id": %q, "language": "french", "hi": false, "download_url": "/download/%s",
				"title": %q, "releases": [%q], "filename": "%s.srt"}`, s.id, s.id, s.title, s.release, s.release))
		}
		_, _ = io.WriteString(w, `{"items": [`+strings.Join(items, ",")+`]}`)
		return
	}
	data, ok := f.served[r.URL.Path]
	if !ok {
		http.NotFound(w, r)
		return
	}
	_, _ = io.WriteString(w, data)
}

// speechCues times cues as a subtitler might from the speech of a
// recording: runs above one half, pauses under 0.4 s bridged, rounded to
// 100 ms and shifted by shift.
func speechCues(s subsync.Speech, shift time.Duration) []subtitle.Cue {
	var cues []subtitle.Cue
	at := func(i int) time.Duration {
		return (time.Duration(i) * time.Second / subsync.SampleRate).Round(100*time.Millisecond) + shift
	}
	start, last := -1, -100
	for i, v := range s {
		if v <= 0.5 {
			continue
		}
		if start >= 0 && i-last > 40 {
			cues = append(cues, subtitle.Cue{Start: at(start), End: at(last + 1), Text: "Parole."})
			start = -1
		}
		if start < 0 {
			start = i
		}
		last = i
	}
	if start >= 0 {
		cues = append(cues, subtitle.Cue{Start: at(start), End: at(last + 1), Text: "Parole."})
	}
	return cues
}

// TestSubtitleFlows runs subtitle acquisition through imports with the
// subtitles plugin against a fake Subsarr on the loopback interface: an
// imported film gets French subtitles at once, the best of them, failing
// to decode, blocklisted and replaced by the next, which is out of sync
// with the film's speech and synchronized against its audio; an imported
// episode gets French subtitles at once, which the upgrade task replaces
// with better ones once a provider has them.
func TestSubtitleFlows(t *testing.T) {
	t.Parallel()
	speechWAV := fixtures.Require(t, "speech.wav")
	s := startAcquisition(t, seriesRoutes(t), true, subtitlesPlugin)
	ctx := t.Context()
	subs := s.subtitles

	// The film's audio is the speech recording, padded with silence to the
	// length of no sample of the film; subtitles timed by its speech are
	// 2.5 s late.
	film := filepath.Join(t.TempDir(), "film.mkv")
	run(t, s.ffmpeg, "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=1280x720:rate=1", "-i", speechWAV,
		"-af", "apad", "-t", "610", "-c:v", "libx264", "-preset", "ultrafast", "-c:a", "aac", "-b:a", "32k", film)
	settled(t, film)
	model, err := vad.NewModel()
	if err != nil {
		t.Fatal(err)
	}
	speech, err := subsync.AudioSpeech(ctx, s.ffmpeg, speechWAV, -1, model, 0)
	if err != nil {
		t.Fatalf("AudioSpeech: %v", err)
	}
	inSync := speechCues(speech, 0)
	late, err := subtitle.Write(&subtitle.Subtitle{Cues: speechCues(speech, 2500*time.Millisecond)}, "srt")
	if err != nil {
		t.Fatal(err)
	}
	episode := s.video("1280x720", 120)

	// Subsarr is enabled in the plugin; subtitles are downloaded whatever
	// their scores, those of movies synchronized whatever theirs, those of
	// episodes never.
	subsarr := newFakeSubsarr(t)
	if _, err := s.system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId:   new("org.mavio.subtitles"),
		ConfigJson: new(`{"providers": [{"name": "subsarr", "enabled": true, "settings": {"base_url": "` + subsarr.URL + `"}}]}`),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}
	waitFor(t, "Subsarr enabled", func() bool {
		resp, err := subs.ListSubtitleProviders(ctx, &acquisitionv1.ListSubtitleProvidersRequest{})
		if err != nil {
			return false
		}
		for _, p := range resp.GetProviders() {
			if p.GetId() == "org.mavio.subtitles/subsarr" {
				return p.GetEnabled()
			}
		}
		return false
	})
	got, err := subs.GetSubtitleSettings(ctx, &acquisitionv1.GetSubtitleSettingsRequest{})
	if err != nil {
		t.Fatalf("GetSubtitleSettings: %v", err)
	}
	settings := got.GetSettings()
	settings.SetMinimumScore(0)
	settings.SetMinimumScoreMovie(0)
	settings.GetSync().SetEnabled(true)
	settings.GetSync().SetUseThreshold(true)
	settings.GetSync().SetThreshold(0)
	if _, err := subs.UpdateSubtitleSettings(ctx, acquisitionv1.UpdateSubtitleSettingsRequest_builder{Settings: settings}.Build()); err != nil {
		t.Fatalf("UpdateSubtitleSettings: %v", err)
	}

	// The film and the series lie in libraries whose profile wants French
	// subtitles.
	movies, shows := t.TempDir(), t.TempDir()
	indexer := newFakeNewznab(t)
	sab := newFakeSABnzbd(t, t.TempDir())
	s.sabnzbd(indexer, sab)
	s.sceneMappings()
	moviesLib := s.library("Movies", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, movies)
	showsLib := s.library("Shows", libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, shows)
	profile, err := subs.CreateSubtitleProfile(ctx, acquisitionv1.CreateSubtitleProfileRequest_builder{
		Profile: acquisitionv1.SubtitleProfile_builder{Name: new("French"), Items: []*acquisitionv1.SubtitleProfileItem{
			acquisitionv1.SubtitleProfileItem_builder{Id: new(int32(1)), Language: new("fr")}.Build(),
		}}.Build(),
	}.Build())
	if err != nil {
		t.Fatalf("CreateSubtitleProfile: %v", err)
	}
	for _, lib := range []string{moviesLib, showsLib} {
		if _, err := subs.AssignSubtitleProfile(ctx, acquisitionv1.AssignSubtitleProfileRequest_builder{
			TargetId: new(lib), ProfileId: new(profile.GetProfile().GetId()),
		}.Build()); err != nil {
			t.Fatalf("AssignSubtitleProfile: %v", err)
		}
	}
	day := time.Now().Add(-48 * time.Hour)

	// history lists the subtitle events of an item, most recent first.
	history := func(itemID string, actions ...acquisitionv1.SubtitleAction) []*acquisitionv1.SubtitleHistoryEvent {
		t.Helper()
		resp, err := subs.ListSubtitleHistory(ctx, acquisitionv1.ListSubtitleHistoryRequest_builder{ItemIds: []string{itemID}, Actions: actions}.Build())
		if err != nil {
			t.Fatalf("ListSubtitleHistory: %v", err)
		}
		return resp.GetEvents()
	}
	describe := func(itemID string) func() any {
		return func() any { return history(itemID) }
	}

	// 1. The film is grabbed and imported. Subsarr's best subtitles for it
	// fail to decode: they are blocklisted, and the next downloaded and
	// synchronized against the film's speech.
	movieProfile := s.profile(release.Movie, release.WEBDL1080p, release.HDTV720p, release.WEBDL1080p)
	added, err := s.wanted.AddMovie(ctx, acquisitionv1.AddMovieRequest_builder{
		LibraryId: new(moviesLib), TmdbId: new(int32(603)), QualityProfileId: new(movieProfile),
	}.Build())
	if err != nil {
		t.Fatalf("AddMovie: %v", err)
	}
	movieID := added.GetItemId()
	broken := subsarrSubtitle{id: "10", title: "The Matrix", release: "The.Matrix.1999.720p.HDTV.x264-GRPA", data: "<html>Daily download limit reached.</html>\n"}
	offset := subsarrSubtitle{id: "11", title: "The Matrix", release: "The.Matrix.1999.1080p.BluRay.x264-OTHER", data: string(late)}
	subsarr.offer(false, broken, offset)
	hdtv := offered{id: "mv01", title: "The.Matrix.1999.720p.HDTV.x264-GRPA", size: 4 << 30, published: day}
	indexer.offer(hdtv)
	if resp, err := s.wanted.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: []string{movieID}}.Build()); err != nil || resp.GetQueued() != 1 {
		t.Fatalf("SearchWanted = %v, %v; want = the movie queued", resp, err)
	}
	s.sabImported(sab, hdtv, s.sabGrabbed(sab, hdtv, movieID, "HDTV-720p"), film)
	french := filepath.Join(movies, "The Matrix (1999)", "The Matrix (1999) HDTV-720p.fr.srt")
	waitFor(t, "the film's French subtitles synchronized", func() bool {
		return len(history(movieID, acquisitionv1.SubtitleAction_SUBTITLE_ACTION_SYNCED)) == 1
	}, describe(movieID))
	if h := history(movieID, acquisitionv1.SubtitleAction_SUBTITLE_ACTION_DOWNLOADED); len(h) != 1 ||
		h[0].GetSubtitleId() != subsarr.link(offset) || h[0].GetPath() != filepath.ToSlash(french) {
		t.Errorf("downloads of the film = %v, want = %s to %s", h, subsarr.link(offset), french)
	}
	blocked, err := subs.ListSubtitleBlocklist(ctx, acquisitionv1.ListSubtitleBlocklistRequest_builder{ItemId: new(movieID)}.Build())
	if err != nil || len(blocked.GetEntries()) != 1 || blocked.GetEntries()[0].GetSubtitleId() != subsarr.link(broken) {
		t.Errorf("ListSubtitleBlocklist = %v, %v; want = %s", blocked, err, subsarr.link(broken))
	}
	data, err := os.ReadFile(french)
	if err != nil {
		t.Fatal(err)
	}
	synced, err := subtitle.Parse(data, "srt")
	if err != nil {
		t.Fatalf("synchronized subtitles: %v", err)
	}
	if len(synced.Cues) != len(inSync) {
		t.Fatalf("synchronized subtitles have %d cues, want = %d", len(synced.Cues), len(inSync))
	}
	for i, c := range synced.Cues {
		if d := (c.Start - inSync[i].Start).Abs(); d > 100*time.Millisecond {
			t.Errorf("cue %d starts at %v, want = %v", i+1, c.Start, inSync[i].Start)
		}
	}

	// 2. An episode is grabbed and imported, and gets the French subtitles
	// Subsarr has at once.
	seriesProfile := s.profile(release.Series, release.WEBDL1080p, release.HDTV720p, release.WEBDL1080p)
	bb := s.addSeries(showsLib, seriesProfile, 81189, libraryv1.SeriesType_SERIES_TYPE_STANDARD, 3)
	pilot := s.episode(bb, 1, 1)
	weak := subsarrSubtitle{id: "20", title: "Breaking Bad", release: "Breaking.Bad.S01E01.1080p.WEB.h264-OTHER", data: "1\n00:00:01,000 --> 00:00:02,000\nBonjour.\n"}
	subsarr.offer(true, weak)
	pilotRelease := offered{
		id: "ep01", title: "Breaking.Bad.S01E01.720p.HDTV.x264-GRPA", size: 300 << 20, published: day,
		categories: []int{5000, 5040}, attrs: [][2]string{{"tvdbid", "81189"}},
	}
	indexer.offer(pilotRelease)
	if resp, err := s.wanted.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: []string{pilot.GetId()}}.Build()); err != nil || resp.GetQueued() != 1 {
		t.Fatalf("SearchWanted = %v, %v; want = the episode queued", resp, err)
	}
	s.sabImported(sab, pilotRelease, s.sabGrabbed(sab, pilotRelease, bb, "HDTV-720p"), episode)
	episodeFile := s.item(pilot.GetId()).GetPath()
	pilotFrench := strings.TrimSuffix(episodeFile, filepath.Ext(episodeFile)) + ".fr.srt"
	waitFor(t, "the episode's French subtitles", func() bool {
		data, err := os.ReadFile(pilotFrench)
		return err == nil && string(data) == weak.data
	}, describe(pilot.GetId()))
	downloads := history(pilot.GetId(), acquisitionv1.SubtitleAction_SUBTITLE_ACTION_DOWNLOADED)
	if len(downloads) != 1 || downloads[0].GetSubtitleId() != subsarr.link(weak) || downloads[0].GetScore() >= downloads[0].GetMaxScore() {
		t.Fatalf("downloads of the episode = %v, want = %s below its maximum score", downloads, subsarr.link(weak))
	}
	if h := history(pilot.GetId(), acquisitionv1.SubtitleAction_SUBTITLE_ACTION_SYNCED); len(h) != 0 {
		t.Errorf("episode's subtitles synchronized: %v", h)
	}

	// 3. Subsarr gets subtitles of the episode's release: the upgrade task
	// replaces the episode's with them.
	better := subsarrSubtitle{id: "21", title: "Breaking Bad", release: pilotRelease.title, data: "1\n00:00:01,000 --> 00:00:02,000\nBonjour, Walter.\n"}
	subsarr.offer(true, better)
	s.runTask("subtitles.upgrade")
	if data, err := os.ReadFile(pilotFrench); err != nil || string(data) != better.data {
		t.Errorf("upgraded subtitles = %q, %v, want = %q", data, err, better.data)
	}
	upgrades := history(pilot.GetId(), acquisitionv1.SubtitleAction_SUBTITLE_ACTION_UPGRADED)
	if len(upgrades) != 1 || upgrades[0].GetSubtitleId() != subsarr.link(better) || upgrades[0].GetUpgradedFrom() != downloads[0].GetId() ||
		upgrades[0].GetScore() <= downloads[0].GetScore() {
		t.Errorf("upgrades of the episode = %v, want = %s replacing %s with a higher score", upgrades, subsarr.link(better), downloads[0].GetId())
	}
	// The film's subtitles have no better: they stay.
	if h := history(movieID, acquisitionv1.SubtitleAction_SUBTITLE_ACTION_UPGRADED); len(h) != 0 {
		t.Errorf("upgrades of the film = %v, want none", h)
	}
}

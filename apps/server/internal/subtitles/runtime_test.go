package subtitles

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/store"
)

// fakePlugin is a subtitle provider plugin with one provider, "subs",
// whose subtitles are those of found.
type fakePlugin struct {
	mu       sync.Mutex
	found    []*pluginv1.RemoteSubtitle
	bad      map[string]bool
	searches []*pluginv1.SearchSubtitlesRequest
	gets     []string
}

func (p *fakePlugin) ListSubtitleProviders(context.Context, *pluginv1.ListSubtitleProvidersRequest) (*pluginv1.ListSubtitleProvidersResponse, error) {
	return pluginv1.ListSubtitleProvidersResponse_builder{Providers: []*pluginv1.SubtitleProviderInfo{
		pluginv1.SubtitleProviderInfo_builder{Name: proto.String("subs"), Enabled: proto.Bool(true)}.Build(),
	}}.Build(), nil
}

func (p *fakePlugin) SearchSubtitles(_ context.Context, req *pluginv1.SearchSubtitlesRequest) (*pluginv1.SearchSubtitlesResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.searches = append(p.searches, req)
	var out []*pluginv1.RemoteSubtitle
	for _, c := range p.found {
		if slices.ContainsFunc(req.GetLanguages(), func(l *pluginv1.SubtitleLanguage) bool { return l.GetLanguage() == c.GetLanguage().GetLanguage() }) {
			out = append(out, c)
		}
	}
	return pluginv1.SearchSubtitlesResponse_builder{Subtitles: out}.Build(), nil
}

func (p *fakePlugin) DownloadSubtitle(_ context.Context, req *pluginv1.DownloadSubtitleRequest) (*pluginv1.DownloadSubtitleResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.gets = append(p.gets, req.GetId())
	if p.bad[req.GetId()] {
		return pluginv1.DownloadSubtitleResponse_builder{Failure: pluginv1.SubtitleProviderFailure_builder{
			Provider: proto.String("subs"), Kind: pluginv1.SubtitleFailureKind_SUBTITLE_FAILURE_KIND_BAD_SUBTITLE.Enum(), Message: proto.String("empty"),
		}.Build()}.Build(), nil
	}
	return pluginv1.DownloadSubtitleResponse_builder{
		Data: []byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nBonjour " + req.GetId() + "\r\n"), Format: proto.String("srt"),
	}.Build(), nil
}

// candidate returns a subtitle of the provider matching the release
// name, if any, and matches.
func candidate(id, lang, name string, matches ...string) *pluginv1.RemoteSubtitle {
	var names []string
	if name != "" {
		names = []string{name}
	}
	return pluginv1.RemoteSubtitle_builder{
		Provider: proto.String("subs"), Id: proto.String(id), Language: pluginv1.SubtitleLanguage_builder{Language: proto.String(lang)}.Build(),
		ReleaseNames: names, Matches: matches, Format: proto.String("srt"),
	}.Build()
}

const release = "The.Matrix.1999.1080p.BluRay.x264-GROUP"

type env struct {
	t      *testing.T
	ctx    context.Context
	db     core.Store
	rt     *Runtime
	plugin *fakePlugin
	lib    core.Library
	movie  core.Item
	dir    string
	s      core.SubtitleSettings
}

func newEnv(t *testing.T) *env {
	t.Helper()
	ctx := t.Context()
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	e := &env{t: t, ctx: ctx, db: db, plugin: &fakePlugin{bad: map[string]bool{}}, s: core.DefaultSubtitleSettings()}
	root := t.TempDir()
	e.lib = core.Library{ID: core.NewID(), Name: "Movies", Kind: core.LibraryMovies, Paths: []string{root}}
	if err := db.Libraries().Create(ctx, &e.lib); err != nil {
		t.Fatal(err)
	}
	e.dir = filepath.Join(root, "The Matrix (1999)")
	video := filepath.Join(e.dir, "The Matrix (1999).mkv")
	if err := os.MkdirAll(e.dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(video, make([]byte, 128<<10), 0o644); err != nil {
		t.Fatal(err)
	}
	year := 1999
	e.movie = core.Item{
		ID: core.NewID(), LibraryID: e.lib.ID, Kind: core.KindMovie, Name: "The Matrix", ProductionYear: year,
		OriginalLanguage: "en", Path: video,
	}
	if err := db.Items().Upsert(ctx, e.movie); err != nil {
		t.Fatal(err)
	}
	src := core.MediaSource{
		ID: core.NewID(), ItemID: e.movie.ID, Path: video, Size: 128 << 10, Container: "matroska",
		Streams: []core.MediaStream{
			{Index: 0, Kind: core.StreamVideo, Codec: "h264", Width: 1920, Height: 1080},
			{Index: 1, Kind: core.StreamAudio, Codec: "aac", Language: "eng", Default: true},
			{Index: 2, Kind: core.StreamSubtitle, Codec: "subrip", Language: "eng"},
		},
	}
	if err := db.MediaSources().Replace(ctx, e.movie.ID, []core.MediaSource{src}); err != nil {
		t.Fatal(err)
	}
	p := core.SubtitleProfile{ID: core.NewID(), Name: "French and English", Items: []core.SubtitleProfileItem{
		{ID: 1, Language: "fr", HI: core.SubtitleHIAllowed}, {ID: 2, Language: "en", HI: core.SubtitleHIAllowed},
	}}
	if err := db.SubtitleProfiles().Put(ctx, &p); err != nil {
		t.Fatal(err)
	}
	if err := db.SubtitleProfileAssignments().Put(ctx, core.SubtitleProfileAssignment{TargetID: e.lib.ID, ProfileID: p.ID}); err != nil {
		t.Fatal(err)
	}
	e.rt = &Runtime{
		Store: db, Settings: func() core.SubtitleSettings { return e.s },
		Providers: func() []Provider { return []Provider{{ID: "plug", Client: e.plugin}} },
		Subtitles: &library.Subtitles{Store: db},
		Class:     func(context.Context, string) core.StorageClass { return core.StorageLocal },
		Now:       func() time.Time { return time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC) },
	}
	return e
}

func (e *env) history(actions ...core.SubtitleAction) []core.SubtitleHistoryEvent {
	e.t.Helper()
	page, err := e.db.SubtitleHistory().List(e.ctx, core.SubtitleHistoryQuery{Actions: actions})
	if err != nil {
		e.t.Fatal(err)
	}
	return page.Items
}

func (e *env) file(name string) string {
	e.t.Helper()
	data, err := os.ReadFile(filepath.Join(e.dir, name))
	if err != nil {
		e.t.Fatal(err)
	}
	return string(data)
}

func TestSearchItem(t *testing.T) {
	e := newEnv(t)
	e.plugin.found = []*pluginv1.RemoteSubtitle{
		candidate("weak", "fr", "", "title"),
		candidate("strong", "fr", release, "title", "year", "source", "resolution", "video_codec", "release_group"),
		candidate("german", "de", release, "title", "year"),
	}
	e.s.MinimumScoreMovie = 0
	got, err := e.rt.SearchItem(e.ctx, e.movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].Language.Language != "fr" || got[0].SubtitleID != "strong" || got[0].Provider != "plug/subs" {
		t.Fatalf("SearchItem = %+v, want the strong French subtitle", got)
	}
	// English is embedded, so only French is searched.
	if langs := e.plugin.searches[0].GetLanguages(); len(langs) != 1 || langs[0].GetLanguage() != "fr" {
		t.Errorf("searched languages = %v, want [fr]", langs)
	}
	if got, want := e.file("The Matrix (1999).fr.srt"), "1\r\n00:00:01,000 --> 00:00:02,000\r\nBonjour strong\r\n"; got != want {
		t.Errorf("subtitle = %q, want = %q", got, want)
	}
	if h := e.history(core.SubtitleDownloaded); len(h) != 1 || h[0].Path != filepath.Join(e.dir, "The Matrix (1999).fr.srt") {
		t.Errorf("history = %+v", h)
	}
	it, err := e.rt.Item(e.ctx, e.movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(it.Missing) != 0 || len(it.Streams) != 2 {
		t.Errorf("Item = %+v, want two subtitles and none missing", it)
	}
	// Nothing is missing any more.
	if wanted, total, err := e.rt.Wanted(e.ctx, core.KindMovie, 0, 0); err != nil || total != 0 || len(wanted) != 0 {
		t.Errorf("Wanted = %v, %d, %v, want none", wanted, total, err)
	}
}

func TestSearchItemMinimumScore(t *testing.T) {
	e := newEnv(t)
	e.plugin.found = []*pluginv1.RemoteSubtitle{candidate("weak", "fr", "", "title")}
	e.s.MinimumScoreMovie = 90
	got, err := e.rt.SearchItem(e.ctx, e.movie.ID)
	if err != nil || len(got) != 0 {
		t.Fatalf("SearchItem = %+v, %v, want nothing above the minimum score", got, err)
	}
	wanted, total, err := e.rt.Wanted(e.ctx, core.KindMovie, 0, 0)
	if err != nil || total != 1 || len(wanted[0].Missing) != 1 || wanted[0].Missing[0].Language != "fr" {
		t.Errorf("Wanted = %+v, %d, %v, want French missing", wanted, total, err)
	}
}

func TestSearchItemBadSubtitle(t *testing.T) {
	e := newEnv(t)
	e.plugin.found = []*pluginv1.RemoteSubtitle{
		candidate("broken", "fr", release, "title", "year", "source", "resolution", "video_codec", "release_group"),
		candidate("fine", "fr", release, "title", "year"),
	}
	e.plugin.bad["broken"] = true
	e.s.MinimumScoreMovie = 0
	got, err := e.rt.SearchItem(e.ctx, e.movie.ID)
	if err != nil || len(got) != 1 || got[0].SubtitleID != "fine" {
		t.Fatalf("SearchItem = %+v, %v, want the fine subtitle", got, err)
	}
	page, err := e.db.SubtitleBlocklist().List(e.ctx, e.movie.ID, 0, 0)
	if err != nil || len(page.Items) != 1 || page.Items[0].SubtitleID != "broken" {
		t.Errorf("blocklist = %+v, %v, want the broken subtitle", page.Items, err)
	}
}

func TestManual(t *testing.T) {
	e := newEnv(t)
	e.plugin.found = []*pluginv1.RemoteSubtitle{candidate("one", "fr", release, "title", "year")}
	found, failed, err := e.rt.ManualSearch(e.ctx, e.movie.ID, nil)
	if err != nil || len(failed) != 0 || len(found) != 1 || found[0].ID != "one" || found[0].Score == 0 {
		t.Fatalf("ManualSearch = %+v, %v, %v", found, failed, err)
	}
	ev, err := e.rt.ManualDownload(e.ctx, e.movie.ID, "plug/subs", "one", core.SubtitleLanguage{Language: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	if ev.Action != core.SubtitleManual || ev.Path != filepath.Join(e.dir, "The Matrix (1999).fr.srt") {
		t.Errorf("ManualDownload = %+v", ev)
	}
	// Blocklisting the download deletes it and searches again.
	if err := e.rt.BlocklistEvent(e.ctx, ev.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(ev.Path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("blocklisted subtitle still there: %v", err)
	}
	found, _, err = e.rt.ManualSearch(e.ctx, e.movie.ID, nil)
	if err != nil || len(found) != 1 || !found[0].Blocklisted {
		t.Errorf("ManualSearch after blocklisting = %+v, %v", found, err)
	}
	jobs, err := e.db.Jobs().List(e.ctx, core.JobQuery{Kinds: []string{JobSearch}})
	if err != nil || len(jobs.Items) != 1 {
		t.Errorf("search jobs = %+v, %v, want one", jobs.Items, err)
	}

	p, err := e.rt.Upload(e.ctx, e.movie.ID, []byte("WEBVTT\n\n00:01.000 --> 00:02.000\nSalut\n"), "vtt", core.SubtitleLanguage{Language: "fr", Forced: true})
	if err != nil {
		t.Fatal(err)
	}
	if p != filepath.Join(e.dir, "The Matrix (1999).fr.forced.srt") {
		t.Errorf("Upload path = %q", p)
	}
	if err := e.rt.Delete(e.ctx, e.movie.ID, p); err != nil {
		t.Fatal(err)
	}
	if h := e.history(core.SubtitleUploaded, core.SubtitleDeleted); len(h) != 3 {
		t.Errorf("history = %+v, want two deletions and an upload", h)
	}

	p, err = e.rt.Upload(e.ctx, e.movie.ID, []byte("1\n00:00:01,000 --> 00:00:02,000\n[DOOR SLAMS]\nWho is there?\n"), "srt", core.SubtitleLanguage{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.rt.Modify(e.ctx, e.movie.ID, p, []string{"remove_HI"}); err != nil {
		t.Fatal(err)
	}
	if got, want := e.file("The Matrix (1999).en.srt"), "1\n00:00:01,000 --> 00:00:02,000\nWho is there?\n\n"; got != want {
		t.Errorf("modified subtitle = %q, want = %q", got, want)
	}
	if err := e.rt.Modify(e.ctx, e.movie.ID, p, []string{"no_such_mod"}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("Modify with an unknown mod: err = %v, want ErrInvalid", err)
	}
}

func TestSyncCloud(t *testing.T) {
	e := newEnv(t)
	e.rt.Class = func(context.Context, string) core.StorageClass { return core.StorageCloud }
	e.rt.FFmpeg = "ffmpeg"
	p, err := e.rt.Upload(e.ctx, e.movie.ID, []byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n"), "srt", core.SubtitleLanguage{Language: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.rt.Sync(e.ctx, e.movie.ID, p); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Sync on a cloud drive: err = %v, want ErrUnavailable", err)
	}
}

func TestShouldSync(t *testing.T) {
	s := core.DefaultSubtitleSettings()
	s.Sync.Enabled, s.Sync.UseThreshold, s.Sync.Threshold = true, true, 90
	s.Sync.ExcludedProviders = []string{"yifysubtitles"}
	for _, c := range []struct {
		name    string
		e       core.SubtitleHistoryEvent
		episode bool
		want    bool
	}{
		{"below threshold", core.SubtitleHistoryEvent{Provider: "p/a", Score: 300, MaxScore: 360, Language: core.SubtitleLanguage{Language: "fr"}}, true, true},
		{"above threshold", core.SubtitleHistoryEvent{Provider: "p/a", Score: 355, MaxScore: 360, Language: core.SubtitleLanguage{Language: "fr"}}, true, false},
		{"movies without threshold", core.SubtitleHistoryEvent{Provider: "p/a", Score: 120, MaxScore: 120, Language: core.SubtitleLanguage{Language: "fr"}}, false, true},
		{"excluded provider", core.SubtitleHistoryEvent{Provider: "p/yifysubtitles", Score: 1, MaxScore: 120, Language: core.SubtitleLanguage{Language: "fr"}}, false, false},
	} {
		if got := shouldSync(&s, &c.e, c.episode); got != c.want {
			t.Errorf("%s: shouldSync = %v, want = %v", c.name, got, c.want)
		}
	}
}

func TestAudioStream(t *testing.T) {
	streams := []core.MediaStream{
		{Kind: core.StreamVideo},
		{Kind: core.StreamAudio, Language: "fre", Default: true},
		{Kind: core.StreamAudio, Language: "jpn"},
	}
	s := core.DefaultSubtitleSettings()
	if got := audioStream(streams, "ja", &s); got != 0 {
		t.Errorf("audioStream = %d, want = 0 (the default)", got)
	}
	s.Sync.OriginalLanguage = true
	if got := audioStream(streams, "ja", &s); got != 1 {
		t.Errorf("audioStream = %d, want = 1 (the original language)", got)
	}
}

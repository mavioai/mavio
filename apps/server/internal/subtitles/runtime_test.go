package subtitles

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
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
	root := filepath.ToSlash(t.TempDir()) // as the scanner records paths
	e.lib = core.Library{ID: core.NewID(), Name: "Movies", Kind: core.LibraryMovies, Paths: []string{root}}
	if err := db.Libraries().Create(ctx, &e.lib); err != nil {
		t.Fatal(err)
	}
	e.dir = path.Join(root, "The Matrix (1999)")
	video := path.Join(e.dir, "The Matrix (1999).mkv")
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
	data, err := os.ReadFile(path.Join(e.dir, name))
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
	// Repaired as ftfy does, which leaves only "\n" line breaks.
	if got, want := e.file("The Matrix (1999).fr.srt"), "1\n00:00:01,000 --> 00:00:02,000\nBonjour strong\n"; got != want {
		t.Errorf("subtitle = %q, want = %q", got, want)
	}
	if h := e.history(core.SubtitleDownloaded); len(h) != 1 || h[0].Path != path.Join(e.dir, "The Matrix (1999).fr.srt") {
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

func TestPrepareFixText(t *testing.T) {
	rt := &Runtime{}
	s := core.DefaultSubtitleSettings()
	s.UTF8 = false
	tests := []struct {
		name, data, want string
	}{
		{"mojibake", "1\n00:00:01,000 --> 00:00:02,000\nCafÃ© &amp; crÃ¨me\n", "1\n00:00:01,000 --> 00:00:02,000\nCafé & crème\n"},
		// Kept as the provider encoded it when there is nothing to repair.
		{"latin-1", "1\n00:00:01,000 --> 00:00:02,000\nCaf\xe9\n", "1\n00:00:01,000 --> 00:00:02,000\nCaf\xe9\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, format, err := rt.prepare([]byte(tt.data), "srt", "fr", false, &s)
			if err != nil || string(got) != tt.want || format != "srt" {
				t.Errorf("prepare got = %q, %q, %v, want = %q, srt, nil", got, format, err, tt.want)
			}
		})
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

func TestWantedKept(t *testing.T) {
	e := newEnv(t)
	missing := func() []string {
		t.Helper()
		wanted, total, err := e.rt.Wanted(e.ctx, core.KindMovie, 0, 0)
		if err != nil || total != len(wanted) {
			t.Fatalf("Wanted = %+v, %d, %v", wanted, total, err)
		}
		var out []string
		for _, w := range wanted {
			for _, l := range w.Missing {
				out = append(out, w.Item.Name+" "+l.String())
			}
		}
		return out
	}
	if got := missing(); len(got) != 0 {
		t.Errorf("Wanted before a refresh = %q, want none", got)
	}
	if err := e.rt.RefreshWanted(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got, want := missing(), []string{"The Matrix fr"}; !slices.Equal(got, want) {
		t.Errorf("Wanted after a refresh = %q, want = %q", got, want)
	}

	// Subtitles uploaded, deleted or extracted change it at once.
	p, err := e.rt.Upload(e.ctx, e.movie.ID, []byte("1\n00:00:01,000 --> 00:00:02,000\nSalut\n"), "srt", core.SubtitleLanguage{Language: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	if got := missing(); len(got) != 0 {
		t.Errorf("Wanted after an upload = %q, want none", got)
	}
	if err := e.rt.Delete(e.ctx, e.movie.ID, p); err != nil {
		t.Fatal(err)
	}
	if got, want := missing(), []string{"The Matrix fr"}; !slices.Equal(got, want) {
		t.Errorf("Wanted after a deletion = %q, want = %q", got, want)
	}

	// So does a file a scan found.
	src, err := e.db.MediaSources().ListForItem(e.ctx, e.movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	src[0].Streams = append(src[0].Streams, core.MediaStream{
		Index: 3, Kind: core.StreamSubtitle, Codec: "subrip", Language: "fre", ExternalPath: path.Join(e.dir, "The Matrix (1999).fr.srt"),
	})
	if err := e.db.MediaSources().Replace(e.ctx, e.movie.ID, src); err != nil {
		t.Fatal(err)
	}
	e.rt.Sidecars(e.ctx, e.lib, []core.ID{e.movie.ID})
	if got := missing(); len(got) != 0 {
		t.Errorf("Wanted after a scan found French = %q, want none", got)
	}

	// Profiles and settings changing take a refresh.
	e.s.UseEmbedded = false
	if err := e.rt.RefreshWanted(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got, want := missing(), []string{"The Matrix en"}; !slices.Equal(got, want) {
		t.Errorf("Wanted without embedded subtitles = %q, want = %q", got, want)
	}
	if err := e.db.SubtitleProfileAssignments().Delete(e.ctx, e.lib.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.rt.RefreshWanted(e.ctx); err != nil {
		t.Fatal(err)
	}
	if got := missing(); len(got) != 0 {
		t.Errorf("Wanted without a profile = %q, want none", got)
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
	if ev.Action != core.SubtitleManual || ev.Path != path.Join(e.dir, "The Matrix (1999).fr.srt") {
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
	if p != path.Join(e.dir, "The Matrix (1999).fr.forced.srt") {
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
	if _, err := e.rt.Sync(e.ctx, e.movie.ID, p, ""); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Sync on a cloud drive: err = %v, want ErrUnavailable", err)
	}

	// Against another subtitle, no video is read: a copy of the English
	// cues 2.5 seconds late is moved back.
	srt := func(shift time.Duration) []byte {
		var b strings.Builder
		at := 5 * time.Second
		for i := range 120 {
			d := time.Duration(1200+(i*7919)%2300) * time.Millisecond
			start, end := at+shift, at+shift+d
			ts := func(t time.Duration) string {
				return fmt.Sprintf("%02d:%02d:%02d,%03d", int(t.Hours()), int(t.Minutes())%60, int(t.Seconds())%60, t.Milliseconds()%1000)
			}
			fmt.Fprintf(&b, "%d\n%s --> %s\nLine %d\n\n", i+1, ts(start), ts(end), i)
			at += d + time.Duration(300+(i*104729)%4100)*time.Millisecond
		}
		return []byte(b.String())
	}
	en, err := e.rt.Upload(e.ctx, e.movie.ID, srt(0), "srt", core.SubtitleLanguage{Language: "en"})
	if err != nil {
		t.Fatal(err)
	}
	fr, err := e.rt.Upload(e.ctx, e.movie.ID, srt(2500*time.Millisecond), "srt", core.SubtitleLanguage{Language: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := e.rt.Sync(e.ctx, e.movie.ID, fr, en)
	if err != nil {
		t.Fatalf("Sync against a subtitle: %v", err)
	}
	if d := res.Offset + 2500*time.Millisecond; d < -20*time.Millisecond || d > 20*time.Millisecond || res.Scale != 1 {
		t.Errorf("Sync = %+v, want an offset of -2.5 s", res)
	}
	if data, err := os.ReadFile(fr); err != nil || !strings.Contains(string(data), "00:00:05,000 --> ") {
		t.Errorf("synchronized subtitle = %.80q, %v", data, err)
	}
	if _, err := e.rt.Sync(e.ctx, e.movie.ID, fr, fr); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("Sync against itself: err = %v, want ErrInvalid", err)
	}
	if _, err := e.rt.Sync(e.ctx, e.movie.ID, fr, "/elsewhere/x.srt"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Sync against an unknown file: err = %v, want ErrNotFound", err)
	}
}

func TestExtract(t *testing.T) {
	e := newEnv(t)
	if _, err := e.rt.Extract(e.ctx, e.movie.ID, 2, nil); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Extract without extraction: err = %v, want ErrUnavailable", err)
	}
	var calls int
	e.rt.Extracted = func(_ context.Context, itemID core.ID, _ *core.MediaSource, st *core.MediaStream) ([]byte, string, error) {
		calls++
		if itemID != e.movie.ID || st.Index != 2 {
			t.Errorf("Extracted(%s, stream %d), want stream 2 of the movie", itemID, st.Index)
		}
		return []byte("1\n00:00:01,000 --> 00:00:02,000\nHello\n"), "srt", nil
	}
	for _, c := range []struct {
		index int
		want  error
	}{{1, core.ErrInvalid}, {9, core.ErrNotFound}} {
		if _, err := e.rt.Extract(e.ctx, e.movie.ID, c.index, nil); !errors.Is(err, c.want) {
			t.Errorf("Extract(stream %d): err = %v, want %v", c.index, err, c.want)
		}
	}
	p, err := e.rt.Extract(e.ctx, e.movie.ID, 2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := path.Base(p), "The Matrix (1999).en.srt"; got != want {
		t.Errorf("Extract path = %s, want %s", got, want)
	}
	if got := e.file(path.Base(p)); !strings.Contains(got, "Hello") {
		t.Errorf("extracted subtitle = %q, want the stream's cues", got)
	}
	if h := e.history(core.SubtitleExtracted); len(h) != 1 || h[0].Path != p || h[0].Language.Language != "en" {
		t.Errorf("history = %+v, want one extraction of %s in en", h, p)
	}
	if _, err := e.rt.Extract(e.ctx, e.movie.ID, 2, &core.SubtitleLanguage{Language: "fr", Forced: true}); err != nil {
		t.Fatal(err)
	}

	// Videos on cloud drives are not read.
	e.rt.Class = func(context.Context, string) core.StorageClass { return core.StorageCloud }
	if _, err := e.rt.Extract(e.ctx, e.movie.ID, 2, nil); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Extract on a cloud drive: err = %v, want ErrUnavailable", err)
	}
	if calls != 2 {
		t.Errorf("Extracted called %d times, want 2", calls)
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

func TestRead(t *testing.T) {
	e := newEnv(t)
	if _, err := e.rt.Read(e.ctx, e.movie.ID, 2); !errors.Is(err, ErrUnavailable) {
		t.Errorf("Read without extraction: err = %v, want ErrUnavailable", err)
	}
	e.rt.Extracted = func(context.Context, core.ID, *core.MediaSource, *core.MediaStream) ([]byte, string, error) {
		return []byte("1\n00:00:01,000 --> 00:00:02,000\n<i>Hello</i>\n"), "srt", nil
	}
	p, err := e.rt.Upload(e.ctx, e.movie.ID, []byte("1\n00:00:03,000 --> 00:00:04,000\nBonjour\n"), "srt", core.SubtitleLanguage{Language: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	subs, err := e.rt.Item(e.ctx, e.movie.ID)
	if err != nil {
		t.Fatal(err)
	}
	file := -1
	for _, s := range subs.Streams {
		if s.Stream.ExternalPath == p {
			file = s.Stream.Index
		}
	}
	for _, c := range []struct {
		index int
		text  string
		want  error
	}{{2, "<i>Hello</i>", nil}, {file, "Bonjour", nil}, {1, "", core.ErrNotFound}, {9, "", core.ErrNotFound}} {
		sub, err := e.rt.Read(e.ctx, e.movie.ID, c.index)
		if !errors.Is(err, c.want) {
			t.Errorf("Read(stream %d): err = %v, want %v", c.index, err, c.want)
			continue
		}
		if c.want == nil && (len(sub.Cues) != 1 || sub.Cues[0].Text != c.text) {
			t.Errorf("Read(stream %d) = %+v, want one cue %q", c.index, sub.Cues, c.text)
		}
	}
}

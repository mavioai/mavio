package animetosho

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ulikunitz/xz"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// fake is Anime Tosho's feed and storage.
type fake struct {
	srv *httptest.Server

	mu       sync.Mutex
	feeds    map[string][]byte
	torrents map[string][]byte
	files    map[string][]byte
	requests []string
}

func newFake(t *testing.T) *fake {
	f := &fake{feeds: map[string][]byte{}, torrents: map[string][]byte{}, files: map[string][]byte{}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests = append(f.requests, r.URL.RequestURI())
		q := r.URL.Query()
		var body []byte
		var ok bool
		switch {
		case r.URL.Path == "/json" && q.Get("eid") != "":
			body, ok = f.feeds[q.Get("eid")]
		case r.URL.Path == "/json" && q.Get("show") == "torrent":
			body, ok = f.torrents[q.Get("id")]
		default:
			body, ok = f.files[r.URL.Path]
		}
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fake, settings string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

// episode is an episode whose AniDB IDs are known.
func episode(eid string) *provider.Video {
	return &provider.Video{
		Episode: true, Series: "Solo Leveling", Season: 1, Number: 12,
		SeriesExternalIDs: map[string]string{"anidb": "17495"}, ExternalIDs: map[string]string{"anidb": eid},
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// torrentResponse is a release with an attachment of each info, as the
// Python test's _torrent_response.
func torrentResponse(t *testing.T, infos ...map[string]any) []byte {
	var attachments []map[string]any
	for i, info := range infos {
		attachments = append(attachments, map[string]any{"id": i + 1, "filename": fmt.Sprintf("track%d.ass", i+1), "type": "subtitle", "info": info})
	}
	return mustJSON(t, map[string]any{"files": []map[string]any{{
		"filename": "[EMBER] Ore dake Level Up na Ken S01E12 [1080p] [HEVC WEBRip].mkv", "attachments": attachments,
	}}})
}

// varygResponse is the Brazilian Portuguese attachments of the VARYG
// release: a forced one and a normal one.
func varygResponse(t *testing.T, forced bool) []byte {
	attachments := []map[string]any{
		{"id": 2730010, "filename": "track25.ass", "type": "subtitle", "info": map[string]any{"codec": "ASS", "lang": "por", "name": "Brazilian", "forced": 0}},
	}
	if forced {
		attachments = append([]map[string]any{
			{"id": 2759029, "filename": "track24.ass", "type": "subtitle", "info": map[string]any{"codec": "ASS", "lang": "por", "name": "Brazilian (Forced)", "forced": 1}},
		}, attachments...)
	}
	return mustJSON(t, map[string]any{"files": []map[string]any{{
		"filename": "Frieren.Beyond.Journeys.End.S02E01.Shall.We.Go.Then.1080p.CR.WEB-DL.MULTi.AAC2.0.H.264-VARYG.mkv", "attachments": attachments,
	}}})
}

func (f *fake) feed(eid string, id int, title string, torrent []byte) {
	f.feeds[eid] = fmt.Appendf(nil, `[{"id":%d,"timestamp":1711853493,"status":"complete","title":%q}]`, id, title)
	f.torrents[fmt.Sprint(id)] = torrent
}

var allVariants = []provider.Language{
	{Tag: "en"}, {Tag: "en", Forced: true}, {Tag: "pt"}, {Tag: "pt", Forced: true}, {Tag: "pt-BR"}, {Tag: "pt-BR", Forced: true},
}

// exprRe reads the Python test's Language expressions.
var exprRe = regexp.MustCompile(`^Language\("(\w+)"(?:, "(\w+)")?(, forced=True)?\)$`)

func expected(t *testing.T, args map[string]json.RawMessage) provider.Language {
	t.Helper()
	e := ported.Arg[map[string]string](t, args, "expected")["$expr"]
	m := exprRe.FindStringSubmatch(e)
	if m == nil {
		t.Fatalf("expression %q", e)
	}
	tag := provider.FromCode(m[1])
	if m[2] != "" {
		tag += "-" + m[2]
	}
	return provider.Language{Tag: tag, Forced: m[3] != ""}
}

func TestPorted(t *testing.T) {
	languageOf := func(t *testing.T, args map[string]json.RawMessage, langs []provider.Language) provider.Language {
		f := newFake(t)
		f.feed("277518", 608526, "Solo Leveling - 12", torrentResponse(t, ported.Arg[map[string]any](t, args, "info")))
		subs, err := newProvider(t, f, `{"search_threshold":1}`).Search(t.Context(), episode("277518"), langs)
		if err != nil {
			t.Fatal(err)
		}
		if len(subs) != 1 {
			t.Fatalf("subtitles got = %d, want = 1", len(subs))
		}
		return subs[0].Language
	}
	ported.Run(t, "animetosho.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_portuguese_brazilian_detection": func(t *testing.T, args map[string]json.RawMessage) {
				got := languageOf(t, args, []provider.Language{{Tag: "pt"}, {Tag: "pt-BR"}})
				if want := expected(t, args); got != want {
					t.Errorf("language got = %+v, want = %+v", got, want)
				}
			},
			"test_forced_flag_is_propagated": func(t *testing.T, args map[string]json.RawMessage) {
				got := languageOf(t, args, allVariants)
				if want := expected(t, args); got != want {
					t.Errorf("language got = %+v, want = %+v", got, want)
				}
			},
		},
		Facts: map[string]string{
			"test_list_subtitles": "TestListSubtitles",
			"test_forced_track_is_not_offered_as_a_normal_subtitle":            "TestForcedTrack",
			"test_reused_attachment_across_releases_keeps_distinct_candidates": "TestReusedAttachment",
			"test_forced_languages_are_declared":                               "TestLanguagesDeclared",
		},
	})
}

func TestListSubtitles(t *testing.T) {
	f := newFake(t)
	f.feeds["277518"] = ported.Response(t, "animetosho_episode_response.json")
	series := ported.Response(t, "animetosho_series_response.json")
	f.torrents["608516"], f.torrents["608526"] = series, series
	subs, err := newProvider(t, f, `{"search_threshold":2}`).Search(t.Context(), episode("277518"), []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 2 {
		t.Fatalf("subtitles got = %d, want = 2", len(subs))
	}
	s := subs[0]
	link := f.srv.URL + "/storage/attach/001dee4b/1961547.xz"
	if s.ID != "608526:"+link || s.PageLink != link || s.Format != "ass" || s.Language != (provider.Language{Tag: "en"}) {
		t.Errorf("subtitle got = %+v", s)
	}
	if want := []string{"[EMBER] Ore dake Level Up na Ken S01E12 [1080p] [HEVC WEBRip] (Solo Leveling)", "[EMBER] Solo Leveling - 12.mkv"}; !slices.Equal(s.ReleaseNames, want) {
		t.Errorf("release names got = %q, want = %q", s.ReleaseNames, want)
	}
	if want := []string{"title", "series", "tvdb_id", "season", "episode"}; !slices.Equal(s.Matches, want) {
		t.Errorf("matches got = %v, want = %v", s.Matches, want)
	}
	if !strings.HasPrefix(subs[1].ID, "608516:") {
		t.Errorf("second subtitle got = %s, want = of 608516", subs[1].ID)
	}
}

func TestForcedTrack(t *testing.T) {
	f := newFake(t)
	f.feed("277518", 608526, "Solo Leveling - 12", varygResponse(t, true))
	p := newProvider(t, f, `{"search_threshold":1}`)
	for _, c := range []struct {
		lang provider.Language
		link string
	}{
		{provider.Language{Tag: "pt-BR"}, "/storage/attach/0029a81a/2730010.xz"},
		{provider.Language{Tag: "pt-BR", Forced: true}, "/storage/attach/002a1975/2759029.xz"},
	} {
		subs, err := p.Search(t.Context(), episode("277518"), []provider.Language{c.lang})
		if err != nil {
			t.Fatal(err)
		}
		if len(subs) != 1 || subs[0].Language != c.lang || subs[0].PageLink != f.srv.URL+c.link {
			t.Errorf("subtitles of %+v got = %+v, want = %s", c.lang, subs, c.link)
		}
	}
}

func TestReusedAttachment(t *testing.T) {
	f := newFake(t)
	f.feeds["306529"] = []byte(`[
		{"id": 592300, "timestamp": 100, "status": "complete", "title": "Earendur Frieren S02E01"},
		{"id": 592301, "timestamp": 200, "status": "complete", "title": "Frieren Beyond Journeys End S02E01 VARYG"},
		{"id": 592302, "timestamp": 300, "status": "skipped", "title": "Skipped"}]`)
	torrent := varygResponse(t, false)
	f.torrents["592300"], f.torrents["592301"] = torrent, torrent
	subs, err := newProvider(t, f, `{"search_threshold":2}`).Search(t.Context(), episode("306529"), []provider.Language{{Tag: "pt-BR"}})
	if err != nil {
		t.Fatal(err)
	}
	shared := f.srv.URL + "/storage/attach/0029a81a/2730010.xz"
	// The latest release comes first.
	var ids []string
	for _, s := range subs {
		ids = append(ids, s.ID)
	}
	if want := []string{"592301:" + shared, "592300:" + shared}; !slices.Equal(ids, want) {
		t.Errorf("IDs got = %v, want = %v", ids, want)
	}
}

func TestLanguagesDeclared(t *testing.T) {
	// Forced variants need no declaration: languages are tags.
	def := Definition()
	for _, tag := range []string{"en", "pt-BR"} {
		if !slices.Contains(def.Languages, tag) {
			t.Errorf("languages got = %v, want = with %s", def.Languages, tag)
		}
	}
	if def.Movies || !def.Episodes {
		t.Errorf("movies, episodes got = %v, %v, want = false, true", def.Movies, def.Episodes)
	}
}

func TestSearchWithoutEpisodeID(t *testing.T) {
	f := newFake(t)
	v := episode("")
	subs, err := newProvider(t, f, `{}`).Search(t.Context(), v, []provider.Language{{Tag: "en"}})
	if err != nil || len(subs) != 0 || len(f.requests) != 0 {
		t.Errorf("search got = %v, %v, %d requests, want = none", subs, err, len(f.requests))
	}
}

func compress(t *testing.T, data []byte) []byte {
	t.Helper()
	var b bytes.Buffer
	w, err := xz.NewWriter(&b)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	sub := []byte("[Script Info]\nTitle: x\n\n[Events]\nDialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hi\n")
	f.files["/storage/attach/00000001/1.xz"] = compress(t, sub)
	f.files["/storage/attach/00000002/2.xz"] = []byte("<html>not a subtitle</html>")
	p := newProvider(t, f, `{}`)
	got, err := p.Download(t.Context(), nil, "5:"+f.srv.URL+"/storage/attach/00000001/1.xz")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, sub) || got.Format != "ass" {
		t.Errorf("download got = %q, %s, want = %q, ass", got.Data, got.Format, sub)
	}
	for id, want := range map[string]provider.Kind{
		"5:" + f.srv.URL + "/storage/attach/00000002/2.xz": provider.ProviderError,
		"5:" + f.srv.URL + "/storage/attach/00000003/3.xz": provider.ProviderError,
		"5":              provider.BadSubtitle,
		"5:ftp://x/1.xz": provider.BadSubtitle,
	} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != want {
			t.Errorf("download of %s got = %v, want = %v", id, err, want)
		}
	}
}

func TestStatus(t *testing.T) {
	for code, want := range map[int]provider.Kind{
		http.StatusTooManyRequests: provider.TooManyRequests, http.StatusServiceUnavailable: provider.ServiceUnavailable,
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(code) }))
		p, err := New(provider.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(srv.URL)
		p.(*Provider).SetBaseURL(u)
		if _, err := p.Search(t.Context(), episode("1"), []provider.Language{{Tag: "en"}}); provider.KindOf(err) != want {
			t.Errorf("status %d got = %v, want = %v", code, err, want)
		}
		srv.Close()
	}
}

func TestNew(t *testing.T) {
	for _, s := range []string{`{"search_threshold":0}`, `{"search_threshold":16}`, `{"anidb_api_client_ver":10}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("New(%s) got = %v, want = ConfigurationError", s, err)
		}
	}
	p, err := New(provider.Env{HTTP: http.DefaultClient}, json.RawMessage(`{"anidb_api_client":"mavio","anidb_api_client_ver":2}`))
	if err != nil || p.(*Provider).threshold != 6 {
		t.Errorf("New got = %v, want = a threshold of 6", err)
	}
}

package animetoshoxyz

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
	"sync"
	"testing"
	"time"

	"github.com/ulikunitz/xz"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

// fake is AnimeTosho.xyz's feed and storage.
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
		if r.Header.Get("User-Agent") != userAgent {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		q := r.URL.Query()
		var body []byte
		var ok bool
		switch {
		case r.URL.Path == "/feed/json":
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

func newProvider(t *testing.T, f *fake) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func episode(eid string) *provider.Video {
	return &provider.Video{
		Episode: true, Series: "Crowned in a Hundred Days", Season: 1, Number: 8,
		SeriesExternalIDs: map[string]string{"anidb": "20151"}, ExternalIDs: map[string]string{"anidb": eid},
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

const releaseName = "[Erai-raws] Crowned in a Hundred Days - 08 (CA) [1080p CR WEB-DL AVC AAC][MultiSub][81FBD56D]"

// topLevelResponse is a release with its attachments at the top level.
func topLevelResponse(t *testing.T, infos ...map[string]any) []byte {
	var attachments []map[string]any
	for i, info := range infos {
		attachments = append(attachments, map[string]any{
			"id": i + 1, "type": "subtitle", "info": info,
			"url": fmt.Sprintf("https://storage.animetosho.xyz/releases/622245/subtitles/track%d.ass.xz", i+1),
		})
	}
	return mustJSON(t, map[string]any{
		"id": 622245, "torrent_name": releaseName, "attachments": attachments,
		"files": []map[string]any{{"id": 1, "filename": releaseName + ".mkv"}},
	})
}

func (f *fake) feed(eid string, id int, title string) {
	f.feeds[eid] = fmt.Appendf(nil, `[{"id":%d,"timestamp":1784290258,"status":"complete","title":%q}]`, id, title)
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
	ported.Run(t, "animetosho_xyz.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_forced_flag_is_propagated": func(t *testing.T, args map[string]json.RawMessage) {
				f := newFake(t)
				f.feed("313249", 622245, releaseName)
				f.torrents["622245"] = topLevelResponse(t, ported.Arg[map[string]any](t, args, "info"))
				subs, err := newProvider(t, f).Search(t.Context(), episode("313249"), allVariants)
				if err != nil {
					t.Fatal(err)
				}
				if want := expected(t, args); len(subs) != 1 || subs[0].Language != want {
					t.Errorf("subtitles got = %+v, want = one of %+v", subs, want)
				}
			},
		},
		Skip: map[string]string{
			"test_list_subtitles_with_episode_id_tuple": "the AniDB episode ID is a number of the video's external IDs; there is no tuple of IDs",
		},
		Facts: map[string]string{
			"test_list_subtitles":                                              "TestListSubtitles",
			"test_list_subtitles_with_portuguese_br":                           "TestListSubtitles",
			"test_list_subtitles_no_anidb_episode_id":                          "TestSearchWithoutEpisodeID",
			"test_nested_attachments_are_found":                                "TestNestedAttachments",
			"test_forced_track_is_not_offered_as_a_normal_subtitle":            "TestNestedAttachments",
			"test_attachments_are_not_listed_twice":                            "TestAttachmentsListedOnce",
			"test_reused_attachment_across_releases_keeps_distinct_candidates": "TestReusedAttachment",
			"test_forced_languages_are_declared":                               "TestLanguagesDeclared",
		},
	})
}

func TestListSubtitles(t *testing.T) {
	f := newFake(t)
	f.feeds["313249"] = ported.Response(t, "animetosho_xyz_episode_response.json")
	series := ported.Response(t, "animetosho_xyz_series_response.json")
	f.torrents["622245"], f.torrents["622243"] = series, series
	p := newProvider(t, f)
	for _, c := range []struct {
		lang provider.Language
		url  string
	}{
		{provider.Language{Tag: "en"}, "https://storage.animetosho.xyz/releases/622245/subtitles/eng.ass"},
		// Brazilian Portuguese is told by its name, "Portuguese[BR]".
		{provider.Language{Tag: "pt-BR"}, "https://storage.animetosho.xyz/releases/622245/subtitles/por.ass"},
	} {
		subs, err := p.Search(t.Context(), episode("313249"), []provider.Language{c.lang})
		if err != nil {
			t.Fatal(err)
		}
		if len(subs) != 2 {
			t.Fatalf("subtitles of %+v got = %d, want = 2", c.lang, len(subs))
		}
		for _, s := range subs {
			if s.Language != c.lang || s.PageLink != c.url || s.ID != "622245:"+c.url || s.Format != "ass" {
				t.Errorf("subtitle got = %+v, want = %+v at %s", s, c.lang, c.url)
			}
			if want := []string{"series", "season", "episode"}; !slices.Equal(s.Matches, want) {
				t.Errorf("matches got = %v, want = %v", s.Matches, want)
			}
		}
	}
	if subs, _ := p.Search(t.Context(), episode("313249"), []provider.Language{{Tag: "pt"}}); len(subs) != 0 {
		t.Errorf("Portuguese subtitles got = %d, want = 0", len(subs))
	}
}

func TestSearchWithoutEpisodeID(t *testing.T) {
	f := newFake(t)
	subs, err := newProvider(t, f).Search(t.Context(), episode(""), []provider.Language{{Tag: "en"}})
	if err != nil || len(subs) != 0 || len(f.requests) != 0 {
		t.Errorf("search got = %v, %v, %d requests, want = none", subs, err, len(f.requests))
	}
}

const varygTitle = "Frieren Beyond Journeys End S02E01 Shall We Go Then 1080p CR WEB-DL MULTi AAC2.0 H 264-VARYG (Sousou no Frieren, Multi-Audio, Multi-Subs)"

func TestNestedAttachments(t *testing.T) {
	f := newFake(t)
	f.feed("306529", 592301, varygTitle)
	f.torrents["592301"] = ported.Response(t, "animetosho_xyz_series_nested_attachments_response.json")
	p := newProvider(t, f)
	subs, err := p.Search(t.Context(), episode("306529"), allVariants)
	if err != nil {
		t.Fatal(err)
	}
	got := map[provider.Language]string{}
	for _, s := range subs {
		got[s.Language] = s.PageLink
	}
	// Without a language name, Brazilian Portuguese stays Portuguese.
	want := map[provider.Language]string{
		{Tag: "en"}:               "https://storage.animetosho.xyz/attachments/0029a/811.xz",
		{Tag: "en", Forced: true}: "https://storage.animetosho.xyz/attachments/002a1/6fb.xz",
		{Tag: "pt"}:               "https://storage.animetosho.xyz/attachments/0029a/81a.xz",
		{Tag: "pt", Forced: true}: "https://storage.animetosho.xyz/attachments/002a1/975.xz",
	}
	if len(got) != len(want) || len(subs) != len(want) {
		t.Fatalf("subtitles got = %v, want = %v", got, want)
	}
	for l, u := range want {
		if got[l] != u {
			t.Errorf("subtitle of %+v got = %s, want = %s", l, got[l], u)
		}
	}
	// The forced track does not answer a normal request, nor the normal
	// one a forced request.
	for _, l := range []provider.Language{{Tag: "pt"}, {Tag: "pt", Forced: true}} {
		subs, err := p.Search(t.Context(), episode("306529"), []provider.Language{l})
		if err != nil || len(subs) != 1 || subs[0].Language != l {
			t.Errorf("subtitles of %+v got = %+v, %v", l, subs, err)
		}
	}
}

func TestAttachmentsListedOnce(t *testing.T) {
	f := newFake(t)
	f.feed("313249", 622245, releaseName)
	a := map[string]any{
		"id": 127399, "type": "subtitle", "url": "https://storage.animetosho.xyz/releases/622245/subtitles/track1.eng.ass.xz",
		"info": map[string]any{"language": "English", "language_code": "eng", "forced": false},
	}
	f.torrents["622245"] = mustJSON(t, map[string]any{
		"id": 622245, "torrent_name": releaseName, "attachments": []any{a},
		"files": []any{map[string]any{"id": 1, "filename": releaseName + ".mkv", "attachments": []any{a}}},
	})
	subs, err := newProvider(t, f).Search(t.Context(), episode("313249"), []provider.Language{{Tag: "en"}})
	if err != nil || len(subs) != 1 {
		t.Errorf("subtitles got = %+v, %v, want = one", subs, err)
	}
}

func TestReusedAttachment(t *testing.T) {
	f := newFake(t)
	shared := "https://storage.animetosho.xyz/attachments/10002/3f70.xz"
	info := map[string]any{"language": "Portuguese[BR]", "language_code": "por", "forced": false}
	onalrie := "[Onalrie] Black Torch - S01E10 [1080p WEBRip AV1]"
	erai := "[Erai-raws] Black Torch - 10 [1080p CR WEB-DL AVC AAC][MultiSub][1595FA67]"
	f.feeds["315278"] = mustJSON(t, []any{
		map[string]any{"id": 685550, "timestamp": 100, "status": "complete", "title": erai},
		map[string]any{"id": 685592, "timestamp": 200, "status": "complete", "title": onalrie},
	})
	for id, title := range map[int]string{685592: onalrie, 685550: erai} {
		f.torrents[fmt.Sprint(id)] = mustJSON(t, map[string]any{
			"id": id, "torrent_name": title, "files": []any{},
			"attachments": []any{map[string]any{"id": 3, "type": "subtitle", "url": shared, "info": info}},
		})
	}
	subs, err := newProvider(t, f).Search(t.Context(), episode("315278"), []provider.Language{{Tag: "pt-BR"}})
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, s := range subs {
		ids = append(ids, s.ID)
	}
	if want := []string{"685592:" + shared, "685550:" + shared}; !slices.Equal(ids, want) {
		t.Errorf("IDs got = %v, want = %v", ids, want)
	}
	if want := []string{onalrie}; !slices.Equal(subs[0].ReleaseNames, want) {
		t.Errorf("release names got = %q, want = %q", subs[0].ReleaseNames, want)
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
	sub := []byte("1\n00:00:01,000 --> 00:00:02,000\nHi\n")
	f.files["/attachments/1.xz"] = compress(t, sub)
	f.files["/attachments/eng.ass"] = []byte("[Script Info]\n")
	p := newProvider(t, f)
	got, err := p.Download(t.Context(), nil, "1:"+f.srv.URL+"/attachments/1.xz")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, sub) || got.Format != "srt" {
		t.Errorf("download got = %q, %s, want = %q, srt", got.Data, got.Format, sub)
	}
	for id, want := range map[string]provider.Kind{
		"1:" + f.srv.URL + "/attachments/eng.ass": provider.ProviderError,
		"1:" + f.srv.URL + "/attachments/none.xz": provider.ProviderError,
		"1": provider.BadSubtitle,
	} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != want {
			t.Errorf("download of %s got = %v, want = %v", id, err, want)
		}
	}
}

func TestChecked(t *testing.T) {
	for code, want := range map[int]provider.Kind{
		http.StatusNotFound: provider.ProviderError, http.StatusTooManyRequests: provider.ProviderError, http.StatusBadGateway: provider.ProviderError,
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
	if _, err := New(provider.Env{}, json.RawMessage(`{"anidb_api_client_ver":0}`)); provider.KindOf(err) != provider.ConfigurationError {
		t.Errorf("New got = %v, want = ConfigurationError", err)
	}
}

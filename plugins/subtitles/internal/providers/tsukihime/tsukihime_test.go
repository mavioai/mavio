package tsukihime

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

// fake is TsukiHime's API and storage.
type fake struct {
	t   *testing.T
	srv *httptest.Server

	mu       sync.Mutex
	paths    map[string][]byte
	requests []string
}

func newFake(t *testing.T) *fake {
	f := &fake{t: t, paths: map[string][]byte{
		"/v1/animes/anidb/69":       []byte(`{"id": 2086, "release_year": 1999}`),
		"/v1/animes/anilist/195230": []byte(`{"id": 400, "release_year": 2025}`),
		"/anime-list-mini.json":     []byte(`[{"anidb_id": 1, "anilist_id": 195230, "imdb_id": "tt0000001"}]`),
	}}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests = append(f.requests, r.URL.Path)
		if r.UserAgent() != userAgent && r.URL.Path != "/anime-list-mini.json" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		body, ok := f.paths[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fake) set(path string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		f.t.Fatal(err)
	}
	f.mu.Lock()
	f.paths[path] = data
	f.mu.Unlock()
}

func newProvider(t *testing.T, f *fake) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	p.(*Provider).AniDB().SetURLs(f.srv.URL+"/anime-list.xml", f.srv.URL+"/anime-list-mini.json", f.srv.URL+"/httpapi")
	return p.(*Provider)
}

func episode() *provider.Video {
	return &provider.Video{
		Episode: true, Series: "One Piece", Season: 1, Number: 1171, Year: 1999,
		FileName: "One.Piece.S01E1171.1080p.NF.WEB-DL.AAC2.0.H.264-VARYG.mkv", SeriesExternalIDs: map[string]string{"anidb": "69"},
	}
}

func movie() *provider.Video {
	return &provider.Video{
		Title: "Summer Pockets Season 1: Omnibus", Year: 2025,
		FileName: "Summer.Pockets.Season.1.Omnibus.1080p.BluRay.mkv", ExternalIDs: map[string]string{"anilist": "195230"},
	}
}

func attachment(id int, lang, codec, name string, forced, cached int) map[string]any {
	return map[string]any{"id": id, "type": 1, "info": map[string]any{"cached": cached, "codec": codec, "lang": lang, "name": name, "forced": forced}}
}

// episodeRelease serves one release of the episode with attachments.
func (f *fake) episodeRelease(name string, attachments ...map[string]any) {
	f.set("/v1/animes/2086/episodes/1171", map[string]any{"results": []any{
		map[string]any{"id": 301, "name": name, "state": "completed", "sublangs": []string{"en"}, "source_date": 1},
	}})
	f.set("/v1/torrents/301", map[string]any{"files": []any{map[string]any{"filename": "One.Piece.S01E1171.mkv", "attachments": attachments}}})
}

// exprRe reads the Python test's Language expressions.
var exprRe = regexp.MustCompile(`^Language\("(\w+)"(?:, "(\w+)")?\)$`)

func TestPorted(t *testing.T) {
	ported.Run(t, "tsukihime.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_language_from_code": func(t *testing.T, args map[string]json.RawMessage) {
				want := ""
				if e := ported.Arg[map[string]string](t, args, "expected"); e != nil {
					m := exprRe.FindStringSubmatch(e["$expr"])
					if m == nil {
						t.Fatalf("expression %q", e["$expr"])
					}
					want = provider.FromCode(m[1])
					switch m[2] {
					case "":
					case "CN":
						want = "zh-Hans"
					case "TW":
						want = "zh-Hant"
					default:
						want += "-" + m[2]
					}
				}
				code := ported.Arg[string](t, args, "code")
				if got := languageFromCode(code); got != want {
					t.Errorf("languageFromCode(%q) got = %q, want = %q", code, got, want)
				}
			},
		},
		Skip: map[string]string{
			"test_streaming_service_is_exposed_in_release_info": "the server guesses the streaming service from the release names, which are not prefixed with it",
		},
		Facts: map[string]string{
			"test_list_episode_subtitles_uses_native_storage":                  "TestEpisode",
			"test_list_movie_subtitles_uses_animetosho_storage_and_best_file":  "TestMovie",
			"test_reused_attachment_across_releases_keeps_distinct_candidates": "TestReusedAttachment",
			"test_forced_track_is_propagated_to_the_language":                  "TestForcedAndHearingImpaired",
			"test_track_name_drives_forced_and_hi_classification":              "TestForcedAndHearingImpaired",
			"test_same_release_tracks_are_distinguished_in_release_info":       "TestTrackLabels",
			"test_uncached_native_subtitle_is_skipped":                         "TestUncached",
			"test_download_subtitle_decompresses_xz":                           "TestDownload",
			"test_download_subtitle_rejects_non_xz_response":                   "TestDownload",
			"test_list_subtitles_without_anime_id_returns_empty":               "TestNoAnime",
			"test_list_subtitles_missing_anime_404_returns_empty":              "TestNoAnime",
			"test_forced_and_hi_variants_are_declared":                         "TestLanguagesDeclared",
		},
	})
}

func TestEpisode(t *testing.T) {
	f := newFake(t)
	f.set("/v1/animes/2086/episodes/1171", map[string]any{"results": []any{
		map[string]any{"id": 301, "name": "One.Piece.S01E1171.1080p.NF.WEB-DL.AAC2.0.H.264-VARYG", "state": "completed", "sublangs": []string{"ar"}, "source_date": 1234},
		map[string]any{"id": 302, "name": "Other", "state": "completed", "sublangs": []string{"fr"}},
		map[string]any{"id": 303, "name": "Pending", "state": "pending", "sublangs": []string{"ar"}},
	}})
	f.set("/v1/torrents/301", map[string]any{"files": []any{
		map[string]any{"filename": "One.Piece.S01E1171.1080p.NF.WEB-DL.AAC2.0.H.264-VARYG.mkv", "attachments": []any{attachment(133226, "ar", "srt", "Arabic", 0, 1)}},
		map[string]any{"filename": "One.Piece.S01E1170.1080p.mkv", "attachments": []any{attachment(133227, "ar", "srt", "Arabic", 0, 1)}},
	}})
	subs, err := newProvider(t, f).Search(t.Context(), episode(), []provider.Language{{Tag: "ar"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("subtitles got = %+v, want = 1", subs)
	}
	s := subs[0]
	link := f.srv.URL + "/attach/0002086A/133226.xz"
	if s.Language != (provider.Language{Tag: "ar"}) || s.Format != "srt" || s.PageLink != link || s.ID != "301:"+link || s.Comment != "Netflix" {
		t.Errorf("subtitle got = %+v", s)
	}
	if want := []string{"series", "season", "episode", "year"}; !slices.Equal(s.Matches, want) {
		t.Errorf("matches got = %v, want = %v", s.Matches, want)
	}
	if want := []string{"One.Piece.S01E1171.1080p.NF.WEB-DL.AAC2.0.H.264-VARYG"}; !slices.Equal(s.ReleaseNames, want) {
		t.Errorf("release names got = %q, want = %q", s.ReleaseNames, want)
	}
	if slices.Contains(f.requests, "/v1/torrents/302") || slices.Contains(f.requests, "/v1/torrents/303") {
		t.Errorf("requests got = %v, want = none of releases without Arabic or incomplete", f.requests)
	}
}

func TestMovie(t *testing.T) {
	f := newFake(t)
	f.set("/v1/animes/400", map[string]any{"results": []any{map[string]any{
		"id": 401, "name": "Summer.Pockets.Season.1.Omnibus.1080p.BluRay", "state": "completed", "sublangs": []string{"en"}, "source_date": 4321, "animetosho": true,
	}}})
	f.set("/v1/torrents/401", map[string]any{"files": []any{
		map[string]any{"filename": "Summer.Pockets.Season.1.Omnibus.1080p.BluRay.mkv", "attachments": []any{attachment(65537, "en", "ass", "English", 0, 1)}},
		map[string]any{"filename": "Random.Extra.Track.1080p.mkv", "attachments": []any{attachment(65538, "en", "srt", "English", 0, 1)}},
	}})
	p := newProvider(t, f)
	// The AniList ID the movie carries, or the one of its IMDb ID.
	byIMDb := movie()
	byIMDb.ExternalIDs = map[string]string{"imdb": "tt0000001"}
	for _, v := range []*provider.Video{movie(), byIMDb} {
		subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}})
		if err != nil {
			t.Fatal(err)
		}
		if len(subs) != 1 || subs[0].Format != "ass" || subs[0].PageLink != f.srv.URL+"/tosho/attach/00010001/65537.xz" {
			t.Fatalf("subtitles got = %+v, want = the one of the best file", subs)
		}
		if want := []string{"title", "year"}; !slices.Equal(subs[0].Matches, want) {
			t.Errorf("matches got = %v, want = %v", subs[0].Matches, want)
		}
	}
}

func TestReusedAttachment(t *testing.T) {
	f := newFake(t)
	f.set("/v1/animes/400", map[string]any{"results": []any{
		map[string]any{"id": 1, "name": "Summer.Pockets", "state": "completed", "sublangs": []string{"en"}, "source_date": 1},
		map[string]any{"id": 2, "name": "Summer.Pockets", "state": "completed", "sublangs": []string{"en"}, "source_date": 2},
	}})
	for _, id := range []int{1, 2} {
		f.set(fmt.Sprintf("/v1/torrents/%d", id), map[string]any{"files": []any{
			map[string]any{"filename": "Summer.Pockets.mkv", "attachments": []any{attachment(10, "en", "srt", "English", 0, 1)}},
		}})
	}
	subs, err := newProvider(t, f).Search(t.Context(), movie(), []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	shared := f.srv.URL + "/attach/0000000A/10.xz"
	var ids []string
	for _, s := range subs {
		ids = append(ids, s.ID)
	}
	// Equally alike releases, the latest first.
	if want := []string{"2:" + shared, "1:" + shared}; !slices.Equal(ids, want) {
		t.Errorf("IDs got = %v, want = %v", ids, want)
	}
}

func TestForcedAndHearingImpaired(t *testing.T) {
	for _, c := range []struct {
		name        string
		attachments []map[string]any
		normal      []int
		forced      []int
		hi          []int
	}{
		{
			"flags", []map[string]any{
				attachment(1, "en", "srt", "Signs", 1, 1), attachment(2, "en", "srt", "English (CC)", 0, 1), attachment(3, "en", "srt", "English", 0, 1),
			}, []int{3}, []int{1}, []int{2},
		},
		{
			"names", []map[string]any{
				attachment(1, "en", "ass", "English [Signs]", 0, 1), attachment(2, "en", "ass", "English (Signs & Songs)", 0, 1),
				attachment(3, "en", "ass", "English", 0, 1), attachment(4, "en", "ass", "English [CC]", 0, 1),
			}, []int{3}, []int{1, 2}, []int{4},
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t)
			f.episodeRelease("One.Piece.S01E1171", c.attachments...)
			p := newProvider(t, f)
			for _, k := range []struct {
				lang provider.Language
				want []int
			}{
				{provider.Language{Tag: "en"}, c.normal},
				{provider.Language{Tag: "en", Forced: true}, c.forced},
				{provider.Language{Tag: "en", HearingImpaired: true}, c.hi},
			} {
				subs, err := p.Search(t.Context(), episode(), []provider.Language{k.lang})
				if err != nil {
					t.Fatal(err)
				}
				var got, want []string
				for _, s := range subs {
					if s.Language != k.lang {
						t.Errorf("language got = %+v, want = %+v", s.Language, k.lang)
					}
					got = append(got, s.PageLink)
				}
				for _, id := range k.want {
					want = append(want, fmt.Sprintf("%s/attach/%08X/%d.xz", f.srv.URL, id, id))
				}
				if !slices.Equal(got, want) {
					t.Errorf("subtitles of %+v got = %v, want = %v", k.lang, got, want)
				}
			}
		})
	}
}

func TestTrackLabels(t *testing.T) {
	f := newFake(t)
	f.episodeRelease("[Sonomama] One Piece - S01E1171",
		attachment(1, "en", "ass", "[Sonomama]", 0, 1), attachment(2, "en", "ass", "Crunchyroll", 0, 1), attachment(3, "en", "ass", "English", 0, 1))
	subs, err := newProvider(t, f).Search(t.Context(), episode(), []provider.Language{{Tag: "en"}})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range subs {
		got = append(got, s.Comment)
		if s.Uploader != "" || !slices.Equal(s.ReleaseNames, []string{"[Sonomama] One Piece - S01E1171"}) {
			t.Errorf("subtitle got = %+v", s)
		}
	}
	if want := []string{"[Sonomama]", "Crunchyroll", ""}; !slices.Equal(got, want) {
		t.Errorf("comments got = %q, want = %q", got, want)
	}
}

func TestStreamingService(t *testing.T) {
	for name, want := range map[string]string{
		"One.Piece.S01E1171.1080p.AMZN.WEB-DL.DDP2.0.H.264-VARYG": "Amazon Prime",
		"One.Piece.S01E1171.1080p.WEB-DL.H.264-VARYG":             "Crunchyroll",
	} {
		f := newFake(t)
		f.episodeRelease(name, attachment(1, "en", "ass", "Crunchyroll", 0, 1))
		subs, err := newProvider(t, f).Search(t.Context(), episode(), []provider.Language{{Tag: "en"}})
		if err != nil {
			t.Fatal(err)
		}
		// The service a release name names comes before its track's name.
		if len(subs) != 1 || subs[0].Comment != want {
			t.Errorf("%s: subtitles got = %+v, want one with comment %q", name, subs, want)
		}
	}
}

func TestUncached(t *testing.T) {
	f := newFake(t)
	f.episodeRelease("One.Piece.S01E1171", attachment(1, "en", "srt", "English", 0, 0), attachment(2, "en", "srt", "English", 0, 1))
	subs, err := newProvider(t, f).Search(t.Context(), episode(), []provider.Language{{Tag: "en"}})
	if err != nil || len(subs) != 1 || subs[0].PageLink != f.srv.URL+"/attach/00000002/2.xz" {
		t.Errorf("subtitles got = %+v, %v, want = the cached one", subs, err)
	}
}

func TestNoAnime(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f)
	v := movie()
	v.ExternalIDs = map[string]string{}
	if subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 || len(f.requests) != 0 {
		t.Errorf("without an AniList ID got = %v, %v, %v, want = none", subs, err, f.requests)
	}
	f.mu.Lock()
	delete(f.paths, "/v1/animes/anilist/195230")
	f.mu.Unlock()
	if subs, err := p.Search(t.Context(), movie(), []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
		t.Errorf("unknown anime got = %v, %v, want = none", subs, err)
	}
	ep := episode()
	ep.SeriesExternalIDs = map[string]string{}
	if subs, err := p.Search(t.Context(), ep, []provider.Language{{Tag: "en"}}); err != nil || len(subs) != 0 {
		t.Errorf("without an AniDB ID got = %v, %v, want = none", subs, err)
	}
}

func TestLanguagesDeclared(t *testing.T) {
	// Forced and hearing-impaired variants need no declaration:
	// languages are tags.
	def := Definition()
	for _, tag := range []string{"en", "pt-BR", "zh-Hans", "es-MX"} {
		if !slices.Contains(def.Languages, tag) {
			t.Errorf("languages got = %v, want = with %s", def.Languages, tag)
		}
	}
	if !def.Movies || !def.Episodes {
		t.Errorf("movies, episodes got = %v, %v, want = true, true", def.Movies, def.Episodes)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	content := []byte("1\n00:00:01,000 --> 00:00:02,000\nExample\n")
	var b bytes.Buffer
	w, err := xz.NewWriter(&b)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = w.Write(content)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	f.paths["/attach/00000001/1.xz"] = b.Bytes()
	f.paths["/attach/00000002/2.xz"] = []byte("<html>not a subtitle</html>")
	f.paths["/attach/00000003/3.xz"] = append([]byte("\xFD7zXZ\x00"), "broken"...)
	p := newProvider(t, f)
	got, err := p.Download(t.Context(), nil, "1:"+f.srv.URL+"/attach/00000001/1.xz")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Data, content) || got.Format != "srt" {
		t.Errorf("download got = %q, %s, want = %q, srt", got.Data, got.Format, content)
	}
	for id, want := range map[string]provider.Kind{
		"1:" + f.srv.URL + "/attach/00000002/2.xz": provider.ProviderError,
		"1:" + f.srv.URL + "/attach/00000003/3.xz": provider.ProviderError,
		"1:" + f.srv.URL + "/attach/00000004/4.xz": provider.ProviderError,
		"nothing": provider.BadSubtitle,
	} {
		if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != want {
			t.Errorf("download of %s got = %v, want = %v", id, err, want)
		}
	}
}

func TestStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer srv.Close()
	p, _ := New(provider.Env{HTTP: srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)
	if _, err := p.Search(t.Context(), movie(), []provider.Language{{Tag: "en"}}); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("status 503 got = %v, want = ProviderError", err)
	}
}

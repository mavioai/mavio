package animesubinfo

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPorted(t *testing.T) {
	ported.Run(t, "animesubinfo.json", ported.Cases{
		Facts: map[string]string{
			"test_list_subtitles_english_title":   "TestListEnglishTitle",
			"test_list_subtitles_japanese_title":  "TestListJapaneseTitle",
			"test_download_subtitle":              "TestDownloadCassette",
			"test_list_subtitles_polish_language": "TestListEnglishTitle",
			"test_subtitle_attributes":            "TestListJapaneseTitle",
			"test_provider_initialization":        "TestDefinition",
			"test_provider_languages":             "TestDefinition",
		},
	})
}

// orgTitles replays a cassette recorded when Bazarr searched the original
// title first: its searches of the English title are answered with the
// recorded searches of the original one.
type orgTitles struct{ rt http.RoundTripper }

func (o orgTitles) RoundTrip(req *http.Request) (*http.Response, error) {
	if q := req.URL.Query(); q.Get("pTitle") == "en" {
		q.Set("pTitle", "org")
		req = req.Clone(req.Context())
		req.URL.RawQuery = q.Encode()
	}
	return o.rt.RoundTrip(req)
}

func cassetteProvider(t *testing.T, name string, org bool) *Provider {
	t.Helper()
	rt := ported.Cassette(t, "test_animesubinfo/"+name)
	if org {
		rt = orgTitles{rt}
	}
	p, err := New(provider.Env{HTTP: &http.Client{Transport: rt}, Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return p.(*Provider)
}

func episode(series string) *provider.Video {
	return &provider.Video{Episode: true, Series: series, Season: 1, Number: 1, FileName: series + " - S01E01.mkv"}
}

var polish = []provider.Language{{Tag: "pl"}}

func TestListEnglishTitle(t *testing.T) {
	for _, name := range []string{"test_list_subtitles_english_title.yaml", "test_list_subtitles_polish_language.yaml"} {
		p := cassetteProvider(t, name, false)
		subs, err := p.Search(t.Context(), episode("Demon Slayer"), polish)
		if err != nil {
			t.Fatal(err)
		}
		for _, s := range subs {
			if s.Language.Tag != "pl" {
				t.Errorf("%s: language got = %q, want = pl", name, s.Language.Tag)
			}
		}
	}
}

func TestListJapaneseTitle(t *testing.T) {
	for name, hash := range map[string]string{
		"test_list_subtitles_japanese_title.yaml": "8e2c0dfbd6f4691f12bcf0088c6a021fdf98db33",
		"test_subtitle_attributes.yaml":           "49efec8753338e4b2dee9545600c803129fb8bb2",
	} {
		p := cassetteProvider(t, name, true)
		subs, err := p.Search(t.Context(), episode("Kimetsu no Yaiba"), polish)
		if err != nil {
			t.Fatal(err)
		}
		if len(subs) != 4 {
			t.Fatalf("%s: got = %d subtitles, want = 4", name, len(subs))
		}
		s := subs[0]
		id := url.Values{"id": {"68055"}, "sh": {hash}}.Encode()
		if s.ID != id || s.Language.Tag != "pl" || s.Uploader != "Askara" || s.PageLink != "http://animesub.info/" ||
			!slices.Equal(s.ReleaseNames, []string{"Kimetsu no Yaiba ep01 - Demon Slayer ep01"}) {
			t.Errorf("%s: got = %+v, want = %s by Askara", name, s, id)
		}
		want := []string{"title", "series", "episode", "season", "audio_codec"}
		if !slices.Equal(s.Matches, want) {
			t.Errorf("%s: matches got = %v, want = %v", name, s.Matches, want)
		}
	}
}

func TestDownloadCassette(t *testing.T) {
	p := cassetteProvider(t, "test_download_subtitle.yaml", true)
	subs, err := p.Search(t.Context(), episode("Kimetsu no Yaiba"), polish)
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 4 {
		t.Fatalf("got = %d subtitles, want = 4", len(subs))
	}
	f, err := p.Download(t.Context(), nil, subs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(f.Data) != 46866 || f.Format != "ass" || !strings.Contains(string(f.Data), "[Script Info]") || !strings.Contains(string(f.Data), "Dialogue:") {
		t.Errorf("got = %d bytes of %q, want = 46866 bytes of ass", len(f.Data), f.Format)
	}
}

func TestDefinition(t *testing.T) {
	d := Definition()
	if !slices.Equal(d.Languages, []string{"pl"}) || !d.Episodes || !d.Movies {
		t.Errorf("got = %v, episodes %v, movies %v, want = [pl], true, true", d.Languages, d.Episodes, d.Movies)
	}
}

func TestSearches(t *testing.T) {
	for _, c := range []struct {
		name  string
		video provider.Video
		want  []search
	}{
		{
			"episode",
			provider.Video{Episode: true, Series: "Show", AlternativeSeries: []string{"A", "B", "C"}, Number: 3, AbsoluteNumber: 27},
			[]search{{"en", "Show ep03"}, {"en", "Show ep27"}, {"org", "A ep03"}, {"pl", "A ep03"}, {"org", "A ep27"}, {"pl", "A ep27"}, {"org", "B ep03"}, {"pl", "B ep03"}, {"org", "B ep27"}, {"pl", "B ep27"}},
		},
		{"pack", provider.Video{Episode: true, Series: "Show"}, []search{{"en", "Show"}}},
		{"movie", provider.Video{Title: "Movie", AlternativeTitles: []string{"Eiga"}}, []search{{"en", "Movie"}, {"org", "Eiga"}, {"pl", "Eiga"}}},
	} {
		if got := searches(&c.video); !slices.Equal(got, c.want) {
			t.Errorf("%s: got = %v, want = %v", c.name, got, c.want)
		}
	}
}

const results = `<html><body>
<table class="Napisy" style="text-align:center">
<tr class="KNap"><td>Kimi no Na wa 2016</td><td>2017.01.01</td><td>&nbsp;</td><td>SubRip</td></tr>
<tr class="KNap"><td>Your Name</td><td>Someone</td><td><table><tr><td></td></tr></table></td><td>20kB</td></tr>
<tr class="KNap"><td>Twoje imię</td><td></td><td></td><td>12 razy</td></tr>
<tr class="KKom"><td><form method="POST" action="sciagnij.php"><input name="id" value="9"><input name="sh" value="abc"></form></td>
<td class="KNap" align="left">Synchro: [Erai-raws], VARYG (CR)</td></tr>
</table>
<table class="Napisy" style="text-align:center"><tr class="KNap"><td>No form</td></tr></table>
</body></html>`

func TestSearchMovie(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/szukaj.php":
			queries = append(queries, r.URL.Query().Get("pTitle")+":"+r.URL.Query().Get("szukane"))
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(results))
		case "/sciagnij.php":
			if err := r.ParseForm(); err != nil || r.Form.Get("id") != "9" || r.Form.Get("sh") != "abc" || r.Form.Get("single_file") == "" {
				t.Errorf("form got = %v, want = id 9, sh abc", r.Form)
			}
			_, _ = w.Write([]byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nCześć, to jest dość długi napis\r\n"))
		default:
			w.WriteHeader(http.StatusTooManyRequests)
		}
	}))
	defer srv.Close()
	p, _ := New(provider.Env{HTTP: srv.Client(), Now: time.Now}, nil)
	u, _ := url.Parse(srv.URL)
	p.(*Provider).SetBaseURL(u)

	if subs, err := p.Search(t.Context(), &provider.Video{Title: "Kimi no Na wa"}, []provider.Language{{Tag: "en"}}); err != nil || subs != nil {
		t.Errorf("english got = %v, %v, want = none", subs, err)
	}
	subs, err := p.Search(t.Context(), &provider.Video{Title: "Kimi no Na wa", Year: 2016, AlternativeTitles: []string{"Your Name"}}, polish)
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"en:Kimi no Na wa", "org:Your Name", "pl:Your Name"}; !slices.Equal(queries, want) {
		t.Errorf("queries got = %v, want = %v", queries, want)
	}
	if len(subs) != 1 {
		t.Fatalf("got = %d subtitles, want = 1 (deduplicated)", len(subs))
	}
	s := subs[0]
	if s.Downloads != 12 || s.Uploader != "Someone" || !slices.Equal(s.Matches, []string{"title", "year"}) {
		t.Errorf("got = %+v, want = 12 downloads by Someone matching title and year", s)
	}
	e := entry{description: "Synchro: [Erai-raws], VARYG (CR)"}
	if got, want := e.releaseGroups(), []string{"Erai-raws", "VARYG"}; !slices.Equal(got, want) {
		t.Errorf("release groups got = %v, want = %v", got, want)
	}
	f, err := p.Download(t.Context(), nil, s.ID)
	if err != nil || f.Format != "srt" || strings.Contains(string(f.Data), "\r") {
		t.Errorf("download got = %q, %v, want = an srt", f.Data, err)
	}

	p.(*Provider).SetBaseURL(&url.URL{Scheme: "http", Host: u.Host, Path: "/limited"})
	if _, err := p.Search(t.Context(), &provider.Video{Title: "X"}, polish); provider.KindOf(err) != provider.TooManyRequests {
		t.Errorf("got = %v, want = %v", err, provider.TooManyRequests)
	}
	if _, err := p.Download(t.Context(), nil, "%zz"); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("got = %v, want = %v", err, provider.BadSubtitle)
	}
}

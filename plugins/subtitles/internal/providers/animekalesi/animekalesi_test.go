package animekalesi

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const (
	listPage = `<html><body><table><tr>
<td id="bolumler"><a href="bolumler-one-piece.html">One Piece</a></td>
<td id="bolumler"><a href="bolumler-shingeki-no-kyojin.html">Shingeki no Kyojin</a></td>
<td id="bolumler"><a href="bolumler-shingeki-no-kyojin-final.html">Shingeki no Kyojin: Final Season</a></td>
<td id="bolumler"><a href="haber.html">Shingeki no Kyojin</a></td>
</tr></table></body></html>`
	episodesPage = `<html><body><table>
<tr><td id="ayazi_indir"><a href="indir_bolum-snk-1.html" title="Shingeki no Kyojin 1. Bölüm Türkçe Altyazısı">1</a></td></tr>
<tr><td id="ayazi_indir"><a href="indir_bolum-snk-2-2.html" title="Shingeki no Kyojin 2. Sezon 2. Bölüm Türkçe Altyazısı">2</a></td></tr>
<tr><td id="ayazi_indir"><a href="indir_bolum-snk-2.html" title="Shingeki no Kyojin 2. Bölüm Türkçe Altyazısı">2</a></td></tr>
<tr><td id="ayazi_indir"><a href="izle-snk-2.html" title="Shingeki no Kyojin 2. Bölüm İzle">2</a></td></tr>
</table></body></html>`
	episodePage = `<html><body>
<p><strong>Çevirmen notu:</strong> yok <strong>Altyazı/Çeviri:</strong> Kalesi Fansub <strong>Tarih:</strong> 2020</p>
<div id="altyazi_indir"><a href="dosyalar/snk-2.zip">İndir</a></div>
</body></html>`
	srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nMerhaba\r\n"
)

type fake struct {
	srv   *httptest.Server
	lists atomic.Int32
}

func newFake(t *testing.T, download []byte) *fake {
	f := &fake{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /tum-anime-serileri.html", func(w http.ResponseWriter, r *http.Request) {
		f.lists.Add(1)
		if r.Header.Get("Referer") == "" || r.Header.Get("Accept-Language") == "" {
			t.Errorf("headers = %v, want = a Referer and an Accept-Language", r.Header)
		}
		_, _ = w.Write([]byte(listPage))
	})
	mux.HandleFunc("GET /altyazib-shingeki-no-kyojin.html", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(episodesPage))
	})
	mux.HandleFunc("GET /indir_bolum-snk-2.html", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(episodePage))
	})
	mux.HandleFunc("GET /indir_bolum-snk-2-2.html", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<div id="altyazi_indir"><a href="dosyalar/snk-s2-2.srt">İndir</a></div>`))
	})
	mux.HandleFunc("GET /dosyalar/snk-2.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(download)
	})
	mux.HandleFunc("GET /dosyalar/broken.zip", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "down", http.StatusServiceUnavailable)
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fake) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range slices.Sorted(func(yield func(string) bool) {
		for n := range files {
			if !yield(n) {
				return
			}
		}
	}) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte(files[name]))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSearch(t *testing.T) {
	f := newFake(t, nil)
	p := newProvider(t, f)
	for _, c := range []struct {
		name    string
		video   provider.Video
		want    []string
		matches []string
	}{
		{
			"episode",
			provider.Video{Episode: true, Series: "Shingeki no Kyojin", Season: 1, Number: 2},
			[]string{"Shingeki no Kyojin - S01E02 by Kalesi Fansub"},
			[]string{"series", "season", "episode"},
		},
		{
			"normalized series",
			provider.Video{Episode: true, Series: "Shingeki no Kyojin!", AlternativeSeries: []string{"Attack"}, Season: 2, Number: 2},
			[]string{"Shingeki no Kyojin - S02E02"},
			[]string{"series", "season", "episode"},
		},
		{"no episode", provider.Video{Episode: true, Series: "Shingeki no Kyojin", Season: 1, Number: 9}, nil, nil},
		{"unknown series", provider.Video{Episode: true, Series: "Naruto", Season: 1, Number: 2}, nil, nil},
		{"movie", provider.Video{Title: "Shingeki no Kyojin"}, nil, nil},
	} {
		t.Run(c.name, func(t *testing.T) {
			subs, err := p.Search(t.Context(), &c.video, []provider.Language{{Tag: "tr"}})
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, s := range subs {
				got = append(got, s.ReleaseNames...)
				if s.Language.Tag != "tr" {
					t.Errorf("language got = %q, want = tr", s.Language.Tag)
				}
				if !slices.Equal(s.Matches, c.matches) {
					t.Errorf("matches got = %v, want = %v", s.Matches, c.matches)
				}
			}
			if !slices.Equal(got, c.want) {
				t.Errorf("got = %v, want = %v", got, c.want)
			}
		})
	}
	if got := f.lists.Load(); got != 3 {
		t.Errorf("list requests got = %d, want = 3 (cached by series)", got)
	}
}

func TestDownload(t *testing.T) {
	for _, c := range []struct {
		name     string
		download []byte
		want     string
		format   string
		kind     provider.Kind
	}{
		{"zip picks the version", zipOf(t, map[string]string{
			"a.srt": "1\n00:00:01,000 --> 00:00:02,000\nA\n",
			"shingeki no kyojin - s01e02 by kalesi fansub.ass": "[Script Info]\r\n",
		}), "[Script Info]\n", "ass", 0},
		{"zip first", zipOf(t, map[string]string{"b.srt": srt, "c.srt": "x", "notes.txt": "x"}), "1\n00:00:01,000 --> 00:00:02,000\nMerhaba\n", "srt", 0},
		{"plain", []byte(srt), "1\n00:00:01,000 --> 00:00:02,000\nMerhaba\n", "srt", 0},
		{"zip without subtitles", zipOf(t, map[string]string{"notes.txt": "x"}), "", "", provider.BadSubtitle},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := newFake(t, c.download)
			p := newProvider(t, f)
			subs, err := p.Search(t.Context(), &provider.Video{Episode: true, Series: "Shingeki no Kyojin", Season: 1, Number: 2}, nil)
			if err != nil || len(subs) != 1 {
				t.Fatalf("search got = %v, %v, want = 1 subtitle", subs, err)
			}
			file, err := p.Download(t.Context(), nil, subs[0].ID)
			if c.kind != 0 {
				if got := provider.KindOf(err); got != c.kind {
					t.Errorf("kind got = %v, want = %v", got, c.kind)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if string(file.Data) != c.want || file.Format != c.format || file.Language.Tag != "tr" {
				t.Errorf("got = %q, %q, %q, want = %q, %q, tr", file.Data, file.Format, file.Language.Tag, c.want, c.format)
			}
		})
	}
}

func TestDownloadStatus(t *testing.T) {
	f := newFake(t, nil)
	p := newProvider(t, f)
	id := url.Values{"u": {f.srv.URL + "/dosyalar/broken.zip"}}.Encode()
	if _, err := p.Download(t.Context(), nil, id); provider.KindOf(err) != provider.ServiceUnavailable {
		t.Errorf("got = %v, want = %v", err, provider.ServiceUnavailable)
	}
}

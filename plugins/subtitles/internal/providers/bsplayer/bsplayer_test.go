package bsplayer

import (
	"bytes"
	"compress/gzip"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

const soap = `<?xml version="1.0" encoding="UTF-8"?>
<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/" xmlns:ns1="http://api.bsplayer-subtitles.com/v1.php">
<SOAP-ENV:Body><ns1:%sResponse><return>%s</return></ns1:%sResponse></SOAP-ENV:Body></SOAP-ENV:Envelope>`

func soapAnswer(fn, ret string) string {
	return strings.Replace(strings.Replace(strings.Replace(soap, "%s", fn, 1), "%s", ret, 1), "%s", fn, 1)
}

// fakeServer is a BSPlayer subtitle server.
type fakeServer struct {
	srv *httptest.Server
	mu  sync.Mutex
	// login answers logins; search searches.
	login, search string
	logins        int
	bodies        []string
}

func newFake(t *testing.T) *fakeServer {
	f := &fakeServer{
		login: `<result>0</result><status>OK</status><data>tok&amp;1</data>`,
		search: `<result><result>200</result><status>OK</status></result><data>
			<item><subID>1</subID><subDownloadLink>%URL%/dl/1</subDownloadLink><subLang>pob</subLang><subName>Movie.2010.720p.BluRay.x264-GRP.srt</subName><subFormat>SRT</subFormat></item>
			<item><subID>2</subID><subDownloadLink>%URL%/dl/2</subDownloadLink><subLang>fre</subLang><subName>Movie.2010.DVDRip.srt</subName><subFormat>srt</subFormat></item>
		</data>`,
	}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/":
		case r.URL.Path == "/v1.php":
			if r.Header.Get("User-Agent") != "BSPlayer/2.x (1022.12360)" || r.Header.Get("Content-Type") != "text/xml; charset=utf-8" {
				t.Errorf("headers got = %v", r.Header)
			}
			body, _ := io.ReadAll(r.Body)
			f.mu.Lock()
			f.bodies = append(f.bodies, string(body))
			f.mu.Unlock()
			switch r.Header.Get("SOAPAction") {
			case `"http://api.bsplayer-subtitles.com/v1.php#logIn"`:
				f.mu.Lock()
				f.logins++
				f.mu.Unlock()
				_, _ = w.Write([]byte(soapAnswer("logIn", f.login)))
			case `"http://api.bsplayer-subtitles.com/v1.php#searchSubtitles"`:
				_, _ = w.Write([]byte(soapAnswer("searchSubtitles", strings.ReplaceAll(f.search, "%URL%", f.srv.URL))))
			default:
				t.Errorf("SOAPAction got = %q", r.Header.Get("SOAPAction"))
			}
		case r.URL.Path == "/dl/1":
			if r.Header.Get("User-Agent") != "Mozilla/4.0 (compatible; Synapse)" {
				t.Errorf("User-Agent got = %q", r.Header.Get("User-Agent"))
			}
			var b bytes.Buffer
			zw := gzip.NewWriter(&b)
			_, _ = zw.Write([]byte("1\r\n00:00:01,000 --> 00:00:02,000\r\nOlá\r\n"))
			_ = zw.Close()
			_, _ = w.Write(b.Bytes())
		case r.URL.Path == "/dl/500":
			_, _ = w.Write([]byte("500"))
		case r.URL.Path == "/dl/plain":
			_, _ = w.Write([]byte("plain"))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeServer) provider(t *testing.T) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, nil)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	p.(*Provider).throttle = 0
	return p.(*Provider)
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	p := f.provider(t)
	v := &provider.Video{Title: "Movie", Year: 2010, ExternalIDs: map[string]string{"imdb": "tt1234567"}, FileHash: "8e245d9679d31e12", FileSize: 12909756}
	langs := []provider.Language{{Tag: "pt-BR"}, {Tag: "fr", HearingImpaired: true}}
	for range 2 {
		got, err := p.Search(t.Context(), v, langs)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 {
			t.Fatalf("got = %d subtitles, want = 2", len(got))
		}
		if got[0].ID != f.srv.URL+"/dl/1" || got[0].Language != (provider.Language{Tag: "pt-BR"}) || got[0].Format != "srt" ||
			!slices.Equal(got[0].ReleaseNames, []string{"Movie.2010.720p.BluRay.x264-GRP.srt"}) || !slices.Equal(got[0].Matches, []string{"title", "year", "hash"}) {
			t.Errorf("subtitle got = %+v", got[0])
		}
		if got[1].Language != langs[1] {
			t.Errorf("language got = %+v, want = %+v", got[1].Language, langs[1])
		}
	}
	if f.logins != 1 {
		t.Errorf("logins got = %d, want = 1", f.logins)
	}
	search := f.bodies[len(f.bodies)-1]
	for _, want := range []string{
		"<ns1:searchSubtitles><handle>tok&amp;1</handle><movieHash>8e245d9679d31e12</movieHash><movieSize>12909756</movieSize><languageId>fre,pob</languageId><imdbId>tt1234567</imdbId></ns1:searchSubtitles>",
		`xmlns:ns1="` + f.srv.URL + `/v1.php"`,
	} {
		if !strings.Contains(search, want) {
			t.Errorf("search got = %s, want = with %s", search, want)
		}
	}

	ep := &provider.Video{Episode: true, Series: "Show", Season: 1, Number: 2, FileHash: "8e245d9679d31e12", FileSize: 1}
	got, err := p.Search(t.Context(), ep, langs)
	if err != nil || len(got) != 2 || !slices.Equal(got[0].Matches, []string{"title", "series", "season", "episode", "year", "hash"}) {
		t.Errorf("Search(episode) got = %+v, %v", got, err)
	}
	if !strings.Contains(f.bodies[len(f.bodies)-1], "<imdbId>*</imdbId>") {
		t.Errorf("search got = %s, want = any IMDb ID", f.bodies[len(f.bodies)-1])
	}
	if got, err := p.Search(t.Context(), &provider.Video{Title: "No hash"}, langs); err != nil || len(got) != 0 {
		t.Errorf("Search(no hash) got = %+v, %v, want = none", got, err)
	}
}

func TestSearchNone(t *testing.T) {
	for _, c := range []struct {
		name, login, search string
	}{
		{"login refused", `<status>FAIL</status>`, ""},
		{"no result", `<status>OK</status><data>t</data>`, `<result></result>`},
		{"bad status", `<status>OK</status><data>t</data>`, `<result><status>NOK</status></result>`},
	} {
		f := newFake(t)
		f.login, f.search = c.login, c.search
		got, err := f.provider(t).Search(t.Context(), &provider.Video{FileHash: "h"}, []provider.Language{{Tag: "en"}})
		if err != nil || len(got) != 0 {
			t.Errorf("%s: got = %+v, %v, want = none", c.name, got, err)
		}
	}
}

func TestSearchFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			return
		}
		_, _ = w.Write([]byte("not xml"))
	}))
	t.Cleanup(srv.Close)
	f := &fakeServer{srv: srv}
	_, err := f.provider(t).Search(t.Context(), &provider.Video{FileHash: "h"}, []provider.Language{{Tag: "en"}})
	if provider.KindOf(err) != provider.TooManyRequests {
		t.Errorf("Search got = %v, want = %v", err, provider.TooManyRequests)
	}

	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(down.Close)
	f = &fakeServer{srv: down}
	_, err = f.provider(t).Search(t.Context(), &provider.Video{FileHash: "h"}, []provider.Language{{Tag: "en"}})
	if provider.KindOf(err) != provider.ServiceUnavailable {
		t.Errorf("Search got = %v, want = %v", err, provider.ServiceUnavailable)
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := f.provider(t)
	got, err := p.Download(t.Context(), nil, f.srv.URL+"/dl/1")
	if err != nil {
		t.Fatal(err)
	}
	if want := "1\n00:00:01,000 --> 00:00:02,000\nOlá\n"; string(got.Data) != want || got.Format != "srt" {
		t.Errorf("Download got = %q %q, want = %q srt", got.Data, got.Format, want)
	}
	for _, c := range []struct {
		path string
		want provider.Kind
	}{
		{"/dl/500", provider.ServiceUnavailable},
		{"/dl/missing", provider.ServiceUnavailable},
		{"/dl/plain", provider.BadSubtitle},
	} {
		if _, err := p.Download(t.Context(), nil, f.srv.URL+c.path); provider.KindOf(err) != c.want {
			t.Errorf("Download(%s) got = %v, want = %v", c.path, err, c.want)
		}
	}
}

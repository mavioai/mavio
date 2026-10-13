package xsubs

import (
	"encoding/json"
	"fmt"
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

// fakeSite is XSubs with the account user/secret.
type fakeSite struct {
	srv     *httptest.Server
	lists   atomic.Int32
	failing atomic.Int32
}

func newFake(t *testing.T) *fakeSite {
	f := &fakeSite{}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /xforum/account/signin/", func(w http.ResponseWriter, r *http.Request) {
		http.SetCookie(w, &http.Cookie{Name: "csrftoken", Value: "tok", Path: "/"})
	})
	mux.HandleFunc("POST /xforum/account/signin/", func(w http.ResponseWriter, r *http.Request) {
		if r.PostFormValue("csrfmiddlewaretoken") != "tok" || r.PostFormValue("username") != "user" || r.PostFormValue("password") != "secret" {
			_, _ = fmt.Fprint(w, "<form>wrong</form>")
			return
		}
		http.Redirect(w, r, "/", http.StatusFound)
	})
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {})
	mux.HandleFunc("GET /series/all.xml", func(w http.ResponseWriter, r *http.Request) {
		f.lists.Add(1)
		_, _ = fmt.Fprint(w, `<?xml version="1.0" encoding="utf-8"?><root>
			<seriesl category="Ταινίες"><series srsid="1">The Show</series></seriesl>
			<seriesl category="Σειρές"><series srsid="7">Show, The [US]</series><series srsid="8">Office [2005]</series><series srsid="9">The Show</series></seriesl></root>`)
	})
	mux.HandleFunc("GET /series/9/main.xml", func(w http.ResponseWriter, r *http.Request) {
		if f.failing.Add(-1) >= 0 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		_, _ = fmt.Fprint(w, `<root><name>The Show (US)</name><series_group ssnnum="x" ssnid="1"/><series_group ssnnum="1" ssnid="31"></series_group><series_group ssnnum="2" ssnid="32"></series_group></root>`)
	})
	mux.HandleFunc("GET /series/9/31.xml", func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `<root>
			<subg><etitle number="1-2" title="Pilot"></etitle><sgt ssnnum="1" epsid="501"></sgt>
				<sr rlsid="900" published_on="2020-01-01"><fmt>720p.HDTV</fmt><team>LOL</team></sr>
				<sr rlsid="901" published_on=""><fmt>1080p</fmt><team>X</team></sr></subg>
			<subg><etitle number="3" title="Third"></etitle><sgt ssnnum="1" epsid="503"></sgt>
				<sr rlsid="903" published_on="2020-01-02"><fmt>WEB</fmt><team>NTb</team></sr></subg>
			<subg><sgt ssnnum="1" epsid="504"></sgt></subg>
			</root>`)
	})
	mux.HandleFunc("GET /xthru/getsub/{id}", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "900" {
			// "Γεια" in Windows-1253.
			_, _ = fmt.Fprint(w, "1\r\n00:00:01,000 --> 00:00:02,000\r\n\xc3\xe5\xe9\xe1\r\n")
		}
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func newProvider(t *testing.T, f *fakeSite, settings string) *Provider {
	t.Helper()
	p, err := New(provider.Env{HTTP: f.srv.Client(), Now: time.Now, Log: slog.New(slog.DiscardHandler)}, json.RawMessage(settings))
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(f.srv.URL)
	p.(*Provider).SetBaseURL(u)
	return p.(*Provider)
}

func TestSearch(t *testing.T) {
	f := newFake(t)
	f.failing.Store(1)
	p := newProvider(t, f, `{"username":"user","password":"secret"}`)
	v := &provider.Video{Episode: true, Series: "The Show", Season: 1, Number: 2, Title: "Pilot", Year: 2010, ReleaseGroup: "DIMENSION"}
	subs, err := p.Search(t.Context(), v, []provider.Language{{Tag: "el"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(subs) != 1 {
		t.Fatalf("subtitles got = %+v, want one", subs)
	}
	s := subs[0]
	if s.ID != f.srv.URL+"/xthru/getsub/900" || s.PageLink != f.srv.URL+"/ice/xsw.xml?srsid=9#31;1;501" || s.ReleaseNames[0] != "720p.HDTV LOL" ||
		!slices.Equal(s.Matches, []string{"series", "season", "episode", "title", "year", "release_group"}) {
		t.Errorf("subtitle got = %+v", s)
	}
	// The show IDs are kept.
	if _, err := p.Search(t.Context(), v, []provider.Language{{Tag: "el"}}); err != nil || f.lists.Load() != 1 {
		t.Errorf("show lists got = %d, %v, want = 1", f.lists.Load(), err)
	}
	for _, c := range []*provider.Video{
		{Episode: true, Series: "Another", Season: 1, Number: 2},
		{Episode: true, Series: "The Show", Season: 3, Number: 1},
		{Title: "The Show"},
	} {
		if subs, err := p.Search(t.Context(), c, []provider.Language{{Tag: "el"}}); err != nil || len(subs) != 0 {
			t.Errorf("%+v got = %v, %v, want none", c, subs, err)
		}
	}
}

func TestShowID(t *testing.T) {
	ids := map[string]int{"show the": 7, "office the 2005": 8, "lost [2004]": 10, "the show": 9}
	for _, c := range []struct {
		names []string
		year  int
		want  int
	}{
		{[]string{"The Show"}, 0, 9},
		{[]string{"The Office"}, 2005, 8},
		{[]string{"Lost"}, 2004, 10},
		{[]string{"Nothing", "Lost"}, 2004, 10},
		{[]string{"Lost"}, 0, 0},
	} {
		if got := showID(ids, c.names, c.year); got != c.want {
			t.Errorf("showID(%v, %d) got = %d, want = %d", c.names, c.year, got, c.want)
		}
	}
}

func TestDownload(t *testing.T) {
	f := newFake(t)
	p := newProvider(t, f, "")
	file, err := p.Download(t.Context(), nil, f.srv.URL+"/xthru/getsub/900")
	if want := "1\n00:00:01,000 --> 00:00:02,000\nΓεια\n"; err != nil || string(file.Data) != want || file.Format != "srt" {
		t.Errorf("download got = %q, %v, want = %q", file.Data, err, want)
	}
	if _, err := p.Download(t.Context(), nil, f.srv.URL+"/xthru/getsub/1"); provider.KindOf(err) != provider.ProviderError {
		t.Errorf("empty download got = %v, want = ProviderError", err)
	}
}

func TestLogin(t *testing.T) {
	f := newFake(t)
	v := &provider.Video{Episode: true, Series: "The Show", Season: 1, Number: 2}
	if _, err := newProvider(t, f, `{"username":"user","password":"wrong"}`).Search(t.Context(), v, []provider.Language{{Tag: "el"}}); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("wrong password got = %v, want = AuthenticationError", err)
	}
	for _, s := range []string{`{"username":"user"}`, `{"password":"x"}`, `[`} {
		if _, err := New(provider.Env{}, json.RawMessage(s)); provider.KindOf(err) != provider.ConfigurationError {
			t.Errorf("%q got = %v, want = ConfigurationError", s, err)
		}
	}
}

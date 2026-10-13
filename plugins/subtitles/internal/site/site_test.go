package site

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestSession(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/login":
			http.SetCookie(w, &http.Cookie{Name: "session", Value: "s1"})
		case "/page":
			if ck, err := r.Cookie("session"); err != nil || ck.Value != "s1" || r.UserAgent() != UserAgent {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=windows-1250")
			// "Čeština" in windows-1250.
			_, _ = io.WriteString(w, "<div class=\"lang\"><a href=\"/s/1\">\xc8e\x9atina</a></div>")
		}
	}))
	defer srv.Close()
	s := New(srv.Client())
	if _, err := s.Get(t.Context(), srv.URL+"/page", nil); provider.KindOf(err) != provider.AuthenticationError {
		t.Errorf("without session got = %v, want an authentication error", err)
	}
	if _, err := s.PostForm(t.Context(), srv.URL+"/login", nil); err != nil {
		t.Fatal(err)
	}
	r, err := s.Get(t.Context(), srv.URL+"/page", nil)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := r.HTML()
	if err != nil {
		t.Fatal(err)
	}
	a := doc.First("div.lang > a")
	if got, want := a.TrimmedText(), "Čeština"; got != want {
		t.Errorf("text got = %q, want = %q", got, want)
	}
	if got, want := a.Attr("href"), "/s/1"; got != want {
		t.Errorf("href got = %q, want = %q", got, want)
	}
	if doc.First("table").Valid() {
		t.Error("got a table, want none")
	}
}

func TestStatus(t *testing.T) {
	cases := map[int]provider.Kind{
		429: provider.TooManyRequests, 401: provider.AuthenticationError, 403: provider.ForbiddenError,
		503: provider.ServiceUnavailable, 404: provider.ProviderError,
	}
	for code, want := range cases {
		r := &Response{Status: code, URL: mustURL(t, "http://site.test/x")}
		if got := provider.KindOf(Status(r)); got != want {
			t.Errorf("Status(%d) got = %v, want = %v", code, got, want)
		}
	}
	if err := Status(&Response{Status: 204}); err != nil {
		t.Errorf("Status(204) got = %v, want = nil", err)
	}
}

func mustURL(t *testing.T, s string) *url.URL {
	t.Helper()
	u, err := url.Parse(s)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

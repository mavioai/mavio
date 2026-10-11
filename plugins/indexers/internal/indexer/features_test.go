package indexer

import (
	"context"
	"fmt"
	"net/http"
	"path/filepath"
	"slices"
	"testing"
	"testing/synctest"
	"time"
)

func TestFilterLimits(t *testing.T) {
	now := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	rel := func(title string, days int, size int64) *Release {
		r := &Release{Title: title, PublishDate: now.AddDate(0, 0, -days)}
		if size >= 0 {
			r.Size = &size
		}
		return r
	}
	all := func() []*Release {
		return []*Release{rel("new-small", 1, 100), rel("new-big", 1, 5000), rel("old-small", 30, 100), rel("old-big", 30, 5000), rel("unsized", 10, -1)}
	}
	tests := []struct {
		name string
		q    Query
		want []string
	}{
		{name: "no limits", want: []string{"new-small", "new-big", "old-small", "old-big", "unsized"}},
		{name: "zero limits", q: Query{MinAge: Ptr(0), MaxSize: Ptr[int64](0)}, want: []string{"new-small", "new-big", "old-small", "old-big", "unsized"}},
		{name: "min age", q: Query{MinAge: Ptr(5)}, want: []string{"old-small", "old-big", "unsized"}},
		{name: "max age", q: Query{MaxAge: Ptr(5)}, want: []string{"new-small", "new-big"}},
		{name: "min size", q: Query{MinSize: Ptr[int64](1000)}, want: []string{"new-big", "old-big"}},
		{name: "max size", q: Query{MaxSize: Ptr[int64](1000)}, want: []string{"new-small", "old-small", "unsized"}},
		{name: "all", q: Query{MinAge: Ptr(5), MaxAge: Ptr(60), MinSize: Ptr[int64](1000), MaxSize: Ptr[int64](10000)}, want: []string{"old-big"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := titles(filterLimits(all(), &tt.q, now)); !slices.Equal(got, tt.want) {
				t.Errorf("filterLimits() = %v, want = %v", got, tt.want)
			}
		})
	}
}

// cookieSite is a site with cookies of its own.
type cookieSite struct {
	*fakeSite
	cookies map[string]string
}

func (s cookieSite) RequestCookies(*Indexer) map[string]string { return s.cookies }

func TestRequestCookiesReplaceKept(t *testing.T) {
	var got []string
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var cs []string
		for _, c := range r.Cookies() {
			cs = append(cs, c.Name+"="+c.Value)
		}
		slices.Sort(cs)
		got = append(got, fmt.Sprint(cs))
		http.SetCookie(w, &http.Cookie{Name: "session", Value: "fromresponse"})
		fmt.Fprint(w, "a 1\n")
	})
	site := &fakeSite{info: Info{SupportsSearch: true, SupportsRSS: true}, chain: func(*Query) *Chain { return pages("http://site/search") }}
	for _, tt := range []struct {
		name string
		site Site
		want []string
	}{
		{name: "kept cookies", site: site, want: []string{"[]", "[session=fromresponse]"}},
		{name: "site cookies", site: cookieSite{site, map[string]string{"uid": "1"}}, want: []string{"[uid=1]", "[uid=1]"}},
		{name: "site without cookies", site: cookieSite{site, nil}, want: []string{"[]", "[session=fromresponse]"}},
	} {
		synctest.Test(t, func(t *testing.T) {
			got = nil
			ix := newIndexer(t, tt.site, h)
			for range 2 {
				if _, err := ix.Search(t.Context(), &Query{}); err != nil {
					t.Fatal(err)
				}
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("%s: cookies sent = %v, want = %v", tt.name, got, tt.want)
			}
		})
	}
}

// cachingSite keeps its pages for a minute.
type cachingSite struct{ *fakeSite }

func (cachingSite) PageCacheLife() time.Duration { return time.Minute }

func TestPageCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		fetched := 0
		h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fetched++
			fmt.Fprint(w, "a 1\n")
		})
		site := cachingSite{&fakeSite{info: Info{SupportsSearch: true, SupportsRSS: true}, chain: func(*Query) *Chain { return pages("http://site/search") }}}
		ix := newIndexer(t, site, h)
		search := func() {
			t.Helper()
			rs, err := ix.Search(t.Context(), &Query{})
			if err != nil {
				t.Fatal(err)
			}
			if len(rs) != 1 || rs[0].Title != "a" {
				t.Fatalf("Search() = %v", titles(rs))
			}
			rs[0].Title = "changed"
		}
		search()
		search()
		if fetched != 1 {
			t.Errorf("fetched = %d within the cache's life, want = 1", fetched)
		}
		time.Sleep(time.Minute)
		search()
		if fetched != 2 {
			t.Errorf("fetched = %d after the cache's life, want = 2", fetched)
		}
	})
}

// preparingSite forgets the indexer's cookies before tests.
type preparingSite struct{ *fakeSite }

func (preparingSite) PrepareTest(_ context.Context, ix *Indexer) error {
	ix.ClearCookies()
	return nil
}

func TestPrepareTest(t *testing.T) {
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := r.Cookie("old"); err == nil {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		fmt.Fprint(w, "a 1\n")
	})
	site := preparingSite{&fakeSite{info: Info{SupportsSearch: true, SupportsRSS: true}, chain: func(*Query) *Chain { return pages("http://site/search") }}}
	ix := newIndexer(t, site, h)
	ix.SetCookies(map[string]string{"old": "1"}, time.Time{})
	if err := ix.Test(t.Context()); err != nil {
		t.Fatalf("Test() = %v, want = nil", err)
	}
	if c := ix.Cookies(); c["old"] != "" {
		t.Errorf("cookies after test = %v, want = none named old", c)
	}
}

func TestStateKeptAcrossRestarts(t *testing.T) {
	file := filepath.Join(t.TempDir(), "status.json")
	st, err := NewStatuses(file, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	ix := &Indexer{ID: "1", Statuses: st}
	ix.SetState("token", "abc")
	st, err = NewStatuses(file, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	ix = &Indexer{ID: "1", Statuses: st}
	if got := ix.State("token"); got != "abc" {
		t.Errorf("State(token) after a restart = %q, want = %q", got, "abc")
	}
	ix.SetState("token", "")
	if got := st.Get("1").Values; len(got) != 0 {
		t.Errorf("values after forgetting = %v, want = none", got)
	}
}

package acquisition

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestAvailabilityFromReleaseDates(t *testing.T) {
	day := func(m time.Month, d int) *time.Time {
		t := time.Date(2026, m, d, 0, 0, 0, 0, time.UTC)
		return &t
	}
	released := core.Wanted{MinimumAvailability: core.AvailabilityReleased}
	for _, tc := range []struct {
		name string
		it   core.Item
		now  time.Time
		want bool
	}{
		{"in cinemas only, before 90 days", core.Item{PremiereDate: day(3, 1)}, *day(5, 1), false},
		{"in cinemas only, after 90 days", core.Item{PremiereDate: day(3, 1)}, *day(6, 1), true},
		{"digital release", core.Item{PremiereDate: day(3, 1), DigitalReleaseDate: day(4, 1)}, *day(4, 2), true},
		{"before the digital release", core.Item{PremiereDate: day(3, 1), DigitalReleaseDate: day(4, 1)}, *day(3, 20), false},
		{"physical release", core.Item{PremiereDate: day(3, 1), PhysicalReleaseDate: day(4, 10)}, *day(4, 10), true},
		{"earliest of digital and physical", core.Item{DigitalReleaseDate: day(5, 1), PhysicalReleaseDate: day(4, 10)}, *day(4, 20), true},
		{"no dates", core.Item{}, *day(4, 20), false},
	} {
		if got := availability(tc.it, released).Available(tc.now, 0); got != tc.want {
			t.Errorf("%s: available = %v, want = %v", tc.name, got, tc.want)
		}
	}
}

// TestOriginalLanguageWanted checks profiles wanting the original
// language take a movie's own, and any while it is unknown.
func TestOriginalLanguageWanted(t *testing.T) {
	german := "The.Matrix.1999.German.1080p.BluRay.x264-GROUP"
	for _, tc := range []struct {
		name, original string
		approved       []string
	}{
		{"original language English", "en", []string{title1080}},
		// A release without languages is in the original language.
		{"original language German", "de", []string{title1080, german}},
		{"original language unknown", "", []string{title1080, german}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			it := e.addMovie()
			it.OriginalLanguage = tc.original
			if err := e.db.Items().Upsert(e.ctx, it); err != nil {
				t.Fatal(err)
			}
			e.indexer.set([]*pluginv1.Release{
				torrent("g1080", title1080, 8*gb, hash1080, e.clock.Add(-time.Hour)),
				torrent("de", german, 8*gb, hash720, e.clock.Add(-time.Hour)),
			}, nil)
			found, err := e.rt.searcher().Search(e.ctx, releases.Search{ItemID: it.ID})
			if err != nil {
				t.Fatal(err)
			}
			decided, err := e.rt.Decide(e.ctx, it.ID, found.Releases)
			if err != nil {
				t.Fatal(err)
			}
			var approved []string
			for _, d := range decided {
				if len(d.Rejections) == 0 {
					approved = append(approved, d.Release.GetTitle())
				}
			}
			slices.Sort(approved)
			if !slices.Equal(approved, tc.approved) {
				t.Errorf("approved = %v, want = %v", approved, tc.approved)
			}
		})
	}
}

// TestRSSMatchesOtherTitles checks RSS releases match movies by their
// alternative titles, translations and secondary year.
func TestRSSMatchesOtherTitles(t *testing.T) {
	for _, tc := range []struct{ name, title string }{
		{"alternative title", "Matrix.Reloaded.Prequel.1999.1080p.BluRay.x264-GROUP"},
		{"translation", "Matrix.Die.Erste.1999.German.1080p.BluRay.x264-GROUP"},
		{"secondary year", "The.Matrix.1998.1080p.BluRay.x264-GROUP"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newEnv(t)
			it := e.addMovie()
			it.AlternativeTitles, it.TranslatedTitles, it.SecondaryYear = []string{"Matrix Reloaded Prequel"}, []string{"Matrix die Erste"}, 1998
			if err := e.db.Items().Upsert(e.ctx, it); err != nil {
				t.Fatal(err)
			}
			e.indexer.set(nil, []*pluginv1.Release{torrent("g", tc.title, 8*gb, hash1080, e.clock.Add(-time.Hour))})
			if err := e.rt.SyncRSS(e.ctx); err != nil {
				t.Fatal(err)
			}
			if got := e.client.titles(); !slices.Equal(got, []string{tc.title}) {
				t.Errorf("grabbed = %v, want = [%s]", got, tc.title)
			}
		})
	}
}

// TestRefreshMovies checks the daily refresh queues refreshes of the
// wanted movies due for one, such as those never identified.
func TestRefreshMovies(t *testing.T) {
	e := newEnv(t)
	day := func(n int) *time.Time {
		d := e.clock.AddDate(0, 0, n)
		return &d
	}
	add := func(name string, refreshed time.Time, premiere, physical *time.Time) core.ID {
		it := core.Item{
			ID: core.NewID(), LibraryID: e.lib.ID, Kind: core.KindMovie, Name: name, Virtual: true, Monitored: true, DateAdded: e.clock,
			MetadataRefreshedAt: refreshed, PremiereDate: premiere, PhysicalReleaseDate: physical,
		}
		if err := e.db.Items().Upsert(e.ctx, it); err != nil {
			t.Fatal(err)
		}
		w := core.Wanted{ItemID: it.ID, QualityProfileID: e.profile, MinimumAvailability: core.AvailabilityReleased, AddedAt: e.clock}
		if err := e.db.Wanted().Put(e.ctx, &w); err != nil {
			t.Fatal(err)
		}
		return it.ID
	}
	placeholder := add("TMDB 1", time.Time{}, nil, nil)
	upcoming := add("Upcoming", *day(-1), day(20), nil)
	add("Released long ago", *day(-1), day(-400), day(-300))
	add("Refreshed lately", e.clock.Add(-time.Hour), day(20), nil)
	stale := add("Stale", *day(-200), day(-400), day(-300))

	if err := e.rt.RefreshMovies(e.ctx); err != nil {
		t.Fatal(err)
	}
	var got []core.ID
	for _, j := range e.jobs(library.JobRefresh) {
		for _, id := range []core.ID{placeholder, upcoming, stale} {
			if strings.Contains(string(j.Payload), id.String()) {
				got = append(got, id)
			}
		}
	}
	if n := len(e.jobs(library.JobRefresh)); n != 3 || len(got) != 3 {
		t.Errorf("refresh jobs = %d (%d of the movies due), want = 3: the placeholder, the upcoming and the stale movie", n, len(got))
	}
	if !slices.ContainsFunc(e.rt.Tasks(), func(task Task) bool { return task.Kind == JobRefreshMovies && task.Interval == 24*time.Hour }) {
		t.Errorf("tasks lack the daily movie refresh")
	}
}

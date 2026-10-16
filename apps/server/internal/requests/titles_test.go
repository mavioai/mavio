package requests

import (
	"errors"
	"slices"
	"sync"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

func TestSearch(t *testing.T) {
	e := newEnv(t)
	movie := pluginv1.MediaKind_MEDIA_KIND_MOVIE
	d := &fakeDiscovery{titles: []*pluginv1.DiscoveredTitle{
		discovered(movie, 1, false), discovered(movie, 2, false), discovered(movie, 66, true),
	}}
	e.svc.Discovery = func() []DiscoveryProvider {
		return []DiscoveryProvider{{ID: "servarr", Client: &fakeDiscovery{noSearch: true}}, {ID: "tmdb", Client: d}}
	}
	pg := e.user("pg", func(u *core.User) { u.Access.MaxParentalRating = score(t, "PG") })
	req := pluginv1.SearchTitlesRequest_builder{Query: new("film"), IncludeAdult: new(true)}.Build()
	page, err := e.svc.Search(e.ctx, pg, "", req)
	if err != nil {
		t.Fatal(err)
	}
	if page.PluginID != "tmdb" || d.lastSearch.GetIncludeAdult() || d.lastSearch.GetQuery() != "film" {
		t.Errorf("Search() by %q asked %v", page.PluginID, d.lastSearch)
	}
	var got []int
	for _, d := range page.Titles {
		got = append(got, d.TmdbID)
	}
	if !slices.Equal(got, []int{1}) || page.Hidden != 2 || page.TotalResults != 58 || page.TotalPages != 3 {
		t.Errorf("Search() = %v, hidden %d of %d results in %d pages, want [1], 2 of 58 in 3", got, page.Hidden, page.TotalResults, page.TotalPages)
	}
	if !page.Titles[0].Requestable {
		t.Errorf("Search() title not requestable")
	}
	if _, err := e.svc.Search(e.ctx, pg, "servarr", req); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Search() of a provider that does not search: error = %v, want ErrNotFound", err)
	}
	if _, err := e.svc.Search(e.ctx, pg, "nope", req); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("Search() of an unknown provider: error = %v, want ErrNotFound", err)
	}
}

func TestGetTitle(t *testing.T) {
	e := newEnv(t)
	pg := e.user("pg", func(u *core.User) { u.Access.MaxParentalRating = score(t, "PG") })
	if _, err := e.svc.GetTitle(e.ctx, pg, core.KindMovie, 2, ""); !errors.Is(err, ErrRating) {
		t.Errorf("GetTitle() above the limit: error = %v, want ErrRating", err)
	}
	if _, err := e.svc.GetTitle(e.ctx, pg, core.KindMovie, 404, ""); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("GetTitle() of an unknown title: error = %v, want ErrNotFound", err)
	}
	m, err := e.svc.GetTitle(e.ctx, pg, core.KindMovie, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if m.Name != "Family Film" || m.Overview != "About Family Film" || m.PosterURL != "https://img/Family Film" ||
		m.ExternalIDs[core.ProviderIMDb] != "tt1" || m.Status != StatusNone || !m.Requestable || len(m.Seasons) != 0 {
		t.Errorf("GetTitle(movie) = %+v", m)
	}
	if m.OriginalLanguage != "en" || !slices.Equal(m.Studios, []string{"Studio Family Film"}) || m.Budget != 1000 ||
		len(m.WatchProviders) != 1 || m.WatchProviders[0].Name != "Stream" {
		t.Errorf("GetTitle(movie) facts = %+v", m)
	}
	if m.Tagline != "In en" || len(m.People) != 1 || m.People[0].Name != "Lead" {
		t.Errorf("GetTitle(movie) tagline, people = %q, %v, want = In en, [Lead]", m.Tagline, m.People)
	}
	if m.ProductionStatus != metadata.ProductionReleased || !slices.Equal(m.GenreRefs, []metadata.Ref{{ID: "18", Name: "Drama"}}) {
		t.Errorf("GetTitle(movie) status, genre refs = %q, %v, want = released, [18 Drama]", m.ProductionStatus, m.GenreRefs)
	}
	// A title's page has its whole cast and crew.
	if !e.meta.last.AllCredits {
		t.Errorf("lookup all credits = false, want = true")
	}
	// Another language is looked up apart.
	if fr, err := e.svc.GetTitle(e.ctx, pg, core.KindMovie, 1, "fr"); err != nil || fr.Tagline != "In fr" {
		t.Errorf("GetTitle(movie, fr) tagline = %q, %v, want = In fr", fr.Tagline, err)
	}

	// Season 1 is requested and available, season 2 requested by
	// another user and pending.
	r := e.must(e.svc.Create(e.ctx, e.admin, NewRequest{Kind: core.KindSeries, TmdbID: 9, Seasons: []int{1}}))
	e.giveFiles(r.ItemID, 1)
	pending := e.must(e.svc.Create(e.ctx, pg, NewRequest{Kind: core.KindSeries, TmdbID: 9, Seasons: []int{2}}))
	s, err := e.svc.GetTitle(e.ctx, pg, core.KindSeries, 9, "")
	if err != nil {
		t.Fatal(err)
	}
	type season struct {
		number, episodes int
		year             int
		status           MediaStatus
		request          core.ID
		requestable      bool
	}
	var got []season
	for _, ss := range s.Seasons {
		y := 0
		if ss.PremiereDate != nil {
			y = ss.PremiereDate.Year()
		}
		got = append(got, season{ss.Number, ss.Episodes, y, ss.Status, ss.RequestID, ss.Requestable})
	}
	want := []season{
		{1, 2, 2021, StatusAvailable, r.ID, false},
		{2, 2, 2022, StatusRequested, pending.ID, false},
		{3, 2, 2023, StatusNone, core.ID{}, true},
	}
	if !slices.Equal(got, want) {
		t.Errorf("GetTitle(series) seasons = %+v, want %+v", got, want)
	}
	if s.Status != StatusAvailable || !s.Requestable || s.ItemID != r.ItemID || len(s.Requests) != 2 {
		t.Errorf("GetTitle(series) = status %v, requestable %v, item %v, %d requests", s.Status, s.Requestable, s.ItemID, len(s.Requests))
	}
	// Specials are listed when the settings allow requesting them.
	e.settings.Specials = true
	if s, err = e.svc.GetTitle(e.ctx, pg, core.KindSeries, 9, ""); err != nil || s.Seasons[0].Number != 0 {
		t.Errorf("GetTitle(series) with specials = %+v, %v", s.Seasons, err)
	}
}

func TestBlocklist(t *testing.T) {
	e := newEnv(t)
	u := e.user("user", nil)
	if _, err := e.svc.Block(e.ctx, u, core.KindMovie, 5); !errors.Is(err, ErrPermission) {
		t.Errorf("Block() by a user: error = %v, want ErrPermission", err)
	}
	b, err := e.svc.Block(e.ctx, e.admin, core.KindMovie, 5)
	if err != nil || b.Title != "Fifth Film" || b.UserID != e.admin.ID {
		t.Fatalf("Block() = %+v, %v", b, err)
	}
	if _, err := e.svc.Block(e.ctx, e.admin, core.KindMovie, 5); !errors.Is(err, core.ErrConflict) {
		t.Errorf("Block() twice: error = %v, want ErrConflict", err)
	}
	// Titles no provider knows are blocked without a name.
	if b, err := e.svc.Block(e.ctx, e.admin, core.KindSeries, 404); err != nil || b.Title != "" {
		t.Errorf("Block(unknown) = %+v, %v", b, err)
	}
	if _, err := e.movie(u, 5); !errors.Is(err, ErrPermission) {
		t.Errorf("requesting a blocked title: error = %v, want ErrPermission", err)
	}
	if _, err := e.movie(e.admin, 5); !errors.Is(err, ErrPermission) {
		t.Errorf("an administrator requesting a blocked title: error = %v, want ErrPermission", err)
	}
	d, err := e.svc.GetTitle(e.ctx, u, core.KindMovie, 5, "")
	if err != nil || d.Status != StatusBlocklisted || d.Requestable {
		t.Errorf("GetTitle(blocked) = %v, requestable %v, %v", d.Status, d.Requestable, err)
	}
	if _, err := e.svc.ListBlocked(e.ctx, u, core.BlockedTitleQuery{}); !errors.Is(err, ErrPermission) {
		t.Errorf("ListBlocked() by a user: error = %v, want ErrPermission", err)
	}
	page, err := e.svc.ListBlocked(e.ctx, e.admin, core.BlockedTitleQuery{Search: "fifth"})
	if err != nil || page.Total != 1 || page.Items[0].TmdbID != 5 {
		t.Errorf("ListBlocked() = %+v, %v", page, err)
	}
	if err := e.svc.Unblock(e.ctx, u, core.KindMovie, 5); !errors.Is(err, ErrPermission) {
		t.Errorf("Unblock() by a user: error = %v, want ErrPermission", err)
	}
	for range 2 {
		if err := e.svc.Unblock(e.ctx, e.admin, core.KindMovie, 5); err != nil {
			t.Errorf("Unblock() = %v", err)
		}
	}
	e.must(e.movie(u, 5))
}

func TestDeletedTitles(t *testing.T) {
	e := newEnv(t)
	u := e.user("user", nil)
	movie := e.must(e.movie(e.admin, 1))
	e.giveFiles(movie.ItemID)
	series := e.must(e.svc.Create(e.ctx, e.admin, NewRequest{Kind: core.KindSeries, TmdbID: 9, Seasons: []int{1}}))
	e.giveFiles(series.ItemID, 1)
	if err := e.svc.Reconcile(e.ctx); err != nil {
		t.Fatal(err)
	}
	if e.get(movie.ID).Status != core.RequestCompleted || e.get(series.ID).Status != core.RequestCompleted {
		t.Fatal("requests not completed")
	}
	// Their items are removed from the libraries.
	if err := e.db.Items().Delete(e.ctx, movie.ItemID, series.ItemID); err != nil {
		t.Fatal(err)
	}
	m, err := e.svc.GetTitle(e.ctx, u, core.KindMovie, 1, "")
	if err != nil || m.Status != StatusDeleted || !m.Requestable {
		t.Errorf("GetTitle(deleted movie) = %v, requestable %v, %v", m.Status, m.Requestable, err)
	}
	s, err := e.svc.GetTitle(e.ctx, u, core.KindSeries, 9, "")
	if err != nil || s.Status != StatusDeleted || s.Seasons[0].Status != StatusDeleted || !s.Seasons[0].Requestable || s.Seasons[1].Status != StatusNone {
		t.Errorf("GetTitle(deleted series) = %v, seasons %+v, %v", s.Status, s.Seasons, err)
	}
	// Deleted titles are requested again.
	e.must(e.movie(u, 1))
	if m, err = e.svc.GetTitle(e.ctx, u, core.KindMovie, 1, ""); err != nil || m.Status != StatusRequested {
		t.Errorf("GetTitle(requested again) = %v, %v", m.Status, err)
	}
}

func TestAnimeRequests(t *testing.T) {
	e := newEnv(t)
	e.meta.titles[titleID(core.KindSeries, 20)] = title{name: "Anime Show", tvdb: 200, seasons: []int{1}, tags: []string{"Anime", "based on manga"}}
	anime := core.Library{ID: core.NewID(), Name: "Anime", Kind: core.LibraryShows, Paths: []string{t.TempDir()}}
	if err := e.db.Libraries().Create(e.ctx, &anime); err != nil {
		t.Fatal(err)
	}
	p := core.QualityProfile{ID: core.NewID(), Kind: core.KindSeries, Name: "anime", Cutoff: 7, Items: []core.QualityProfileItem{{Quality: 7, Allowed: true}}}
	if err := e.db.QualityProfiles().Put(e.ctx, &p); err != nil {
		t.Fatal(err)
	}
	e.settings.AnimeLibraryID, e.settings.AnimeQualityProfileID = anime.ID, p.ID
	e.must(e.svc.Create(e.ctx, e.admin, NewRequest{Kind: core.KindSeries, TmdbID: 20, Seasons: []int{1}}))
	e.must(e.svc.Create(e.ctx, e.admin, NewRequest{Kind: core.KindSeries, TmdbID: 9, Seasons: []int{1}}))
	if len(e.acq.series) != 2 {
		t.Fatalf("series added = %d, want 2", len(e.acq.series))
	}
	a, other := e.acq.series[0], e.acq.series[1]
	if a.LibraryID != anime.ID || a.QualityProfileID != p.ID || a.SeriesType == nil || *a.SeriesType != core.SeriesAnime {
		t.Errorf("anime series added to %v with %v as %v", a.LibraryID, a.QualityProfileID, a.SeriesType)
	}
	if other.LibraryID != e.shows.ID || other.QualityProfileID != e.settings.SeriesQualityProfileID || other.SeriesType != nil {
		t.Errorf("series added to %v with %v as %v", other.LibraryID, other.QualityProfileID, other.SeriesType)
	}
}

func TestMetadataRegion(t *testing.T) {
	e := newEnv(t)
	got, err := e.svc.Title(e.ctx, core.KindMovie, 1)
	if err != nil || got.Country != "US" || e.meta.last.Language != "en" {
		t.Errorf("Title() without settings in %q, %q, %v", got.Country, e.meta.last.Language, err)
	}
	e.svc.MetadataSettings = func() core.MetadataSettings { return core.MetadataSettings{PreferredLanguage: "de", Country: "DE"} }
	got, err = e.svc.Title(e.ctx, core.KindMovie, 1)
	if err != nil || got.Country != "DE" || e.meta.last.Country != "DE" || e.meta.last.Language != "de" {
		t.Errorf("Title() in %q, looked up in %q, %q, %v", got.Country, e.meta.last.Country, e.meta.last.Language, err)
	}
}

// pushed is a notification pushed to devices.
type pushed struct {
	typ           string
	user, admins  bool
	sameAsRecords bool
}

func TestNotificationEvents(t *testing.T) {
	e := newEnv(t)
	var mu sync.Mutex
	var got []pushed
	e.svc.Events = func(a core.Activity, user, admins bool) {
		mu.Lock()
		defer mu.Unlock()
		e.acts.mu.Lock()
		last := e.acts.acts[len(e.acts.acts)-1]
		e.acts.mu.Unlock()
		got = append(got, pushed{a.Type, user, admins, last.ID == a.ID && !a.ID.IsZero() && !a.Time.IsZero()})
	}
	u := e.user("user", nil)
	r := e.must(e.movie(u, 1))
	e.must(e.svc.Decline(e.ctx, e.admin, r.ID))
	want := []pushed{{TypePending, false, true, true}, {TypeDeclined, true, false, true}}
	if !slices.Equal(got, want) {
		t.Errorf("pushed = %+v, want %+v", got, want)
	}
}

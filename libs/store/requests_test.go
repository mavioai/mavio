package store_test

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func newUser(t *testing.T, s *store.Store, name string) core.User {
	t.Helper()
	u := core.User{ID: core.NewID(), Name: name, Kind: core.UserMember}
	if err := s.Users().Create(t.Context(), &u); err != nil {
		t.Fatal(err)
	}
	return u
}

func TestRequests(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		alice, bob := newUser(t, s, "alice"), newUser(t, s, "bob")
		repo := s.Requests()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		matrix := core.Request{
			Kind: core.KindMovie, TmdbID: 603, Title: "The Matrix", Year: 1999, Status: core.RequestPending,
			RequestedBy: alice.ID, CreatedAt: base, UpdatedAt: base,
		}
		breaking := core.Request{
			ID: core.NewID(), Kind: core.KindSeries, TmdbID: 1396, TvdbID: 81189, Title: "Breaking Bad",
			Seasons: []core.SeasonRequest{{Number: 1, Status: core.RequestApproved}, {Number: 2, Status: core.RequestApproved}},
			Status:  core.RequestApproved, RequestedBy: alice.ID, ModifiedBy: bob.ID, ItemID: core.NewID(),
			CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(3 * time.Hour),
		}
		declined := core.Request{
			Kind: core.KindSeries, TmdbID: 1399, Seasons: []core.SeasonRequest{{Number: 1, Status: core.RequestDeclined}},
			Status: core.RequestDeclined, RequestedBy: alice.ID, CreatedAt: base.Add(2 * time.Hour), UpdatedAt: base.Add(2 * time.Hour),
		}
		ignored := core.Request{
			Kind: core.KindMovie, TmdbID: 550, Status: core.RequestApproved, RequestedBy: alice.ID, IgnoreQuota: true,
			CreatedAt: base.Add(4 * time.Hour), UpdatedAt: base.Add(4 * time.Hour),
		}
		heat := core.Request{
			Kind: core.KindMovie, TmdbID: 949, Status: core.RequestPending, RequestedBy: bob.ID,
			CreatedAt: base.Add(5 * time.Hour), UpdatedAt: base.Add(5 * time.Hour),
		}
		for _, r := range []*core.Request{&matrix, &breaking, &declined, &ignored, &heat} {
			if err := repo.Put(ctx, r); err != nil {
				t.Fatal(err)
			}
		}
		if err := repo.Put(ctx, &core.Request{Kind: core.KindMovie, Status: core.RequestPending, RequestedBy: alice.ID}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put(no TMDB ID) = %v, want = ErrInvalid", err)
		}
		got, err := repo.Get(ctx, breaking.ID)
		if err != nil || !reflect.DeepEqual(got, breaking) {
			t.Fatalf("Get() = %+v, %v, want = %+v", got, err, breaking)
		}
		if _, err := repo.Get(ctx, core.NewID()); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get(unknown) = %v, want = ErrNotFound", err)
		}

		ids := func(p core.Page[core.Request]) []core.ID {
			out := make([]core.ID, len(p.Items))
			for i, r := range p.Items {
				out[i] = r.ID
			}
			return out
		}
		lists := []struct {
			name string
			q    core.RequestQuery
			want []core.ID
		}{
			{"all", core.RequestQuery{}, []core.ID{heat.ID, ignored.ID, declined.ID, breaking.ID, matrix.ID}},
			{"ascending", core.RequestQuery{Ascending: true, Limit: 2}, []core.ID{matrix.ID, breaking.ID}},
			{"by user", core.RequestQuery{RequestedBy: bob.ID}, []core.ID{heat.ID}},
			{"by status", core.RequestQuery{Statuses: []core.RequestStatus{core.RequestPending, core.RequestDeclined}}, []core.ID{heat.ID, declined.ID, matrix.ID}},
			{"by kind", core.RequestQuery{Kind: core.KindSeries}, []core.ID{declined.ID, breaking.ID}},
			{"by TMDB ID", core.RequestQuery{Kind: core.KindSeries, TmdbID: 1396}, []core.ID{breaking.ID}},
			{"by item", core.RequestQuery{ItemID: breaking.ItemID}, []core.ID{breaking.ID}},
			{"by modification", core.RequestQuery{SortByModified: true, Kind: core.KindSeries}, []core.ID{breaking.ID, declined.ID}},
			{"page", core.RequestQuery{Limit: 2, Offset: 1}, []core.ID{ignored.ID, declined.ID}},
		}
		for _, tt := range lists {
			p, err := repo.List(ctx, tt.q)
			if err != nil {
				t.Fatalf("%s: List() = %v", tt.name, err)
			}
			if got := ids(p); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s: List() = %v, want = %v", tt.name, got, tt.want)
			}
		}
		if p, _ := repo.List(ctx, core.RequestQuery{Limit: 1}); p.Total != 5 {
			t.Errorf("List().Total = %d, want = 5", p.Total)
		}

		quotas := []struct {
			name  string
			kind  core.ItemKind
			since time.Time
			want  int
		}{
			{"movies", core.KindMovie, time.Time{}, 1},
			{"seasons", core.KindSeries, time.Time{}, 2},
			{"seasons since", core.KindSeries, base.Add(time.Hour), 0},
			{"movies since", core.KindMovie, base.Add(-time.Minute), 1},
		}
		for _, tt := range quotas {
			if got, err := repo.QuotaUsed(ctx, alice.ID, tt.kind, tt.since); err != nil || got != tt.want {
				t.Errorf("%s: QuotaUsed() = %d, %v, want = %d", tt.name, got, err, tt.want)
			}
		}

		// Clearing the item and the modifier.
		breaking.ItemID, breaking.ModifiedBy, breaking.Status = core.ID{}, core.ID{}, core.RequestFailed
		breaking.Message = "no library"
		if err := repo.Put(ctx, &breaking); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.Get(ctx, breaking.ID); !reflect.DeepEqual(got, breaking) {
			t.Errorf("Get() after Put = %+v, want = %+v", got, breaking)
		}
		if err := repo.Delete(ctx, heat.ID); err != nil {
			t.Fatal(err)
		}
		if err := s.Users().Delete(ctx, alice.ID); err != nil {
			t.Fatal(err)
		}
		if p, _ := repo.List(ctx, core.RequestQuery{}); p.Total != 0 {
			t.Errorf("List() after deleting = %d requests, want none", p.Total)
		}
	})
}

func TestIssues(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		alice, bob := newUser(t, s, "alice"), newUser(t, s, "bob")
		lib := newLibrary(t, s, "/movies")
		movie := newItem(lib, core.KindMovie, "Heat")
		upsert(t, s, movie)
		repo := s.Issues()
		base := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
		audio := core.Issue{
			ItemID: movie.ID, Type: core.IssueAudio, Status: core.IssueOpen, CreatedBy: alice.ID,
			CreatedAt: base, UpdatedAt: base,
		}
		video := core.Issue{
			ItemID: movie.ID, Type: core.IssueVideo, Status: core.IssueResolved, CreatedBy: bob.ID, ModifiedBy: alice.ID,
			SeasonNumber: 1, EpisodeNumber: 2, CreatedAt: base.Add(time.Hour), UpdatedAt: base.Add(time.Hour),
		}
		for _, i := range []*core.Issue{&audio, &video} {
			if err := repo.Put(ctx, i); err != nil {
				t.Fatal(err)
			}
		}
		first := core.IssueComment{IssueID: audio.ID, UserID: alice.ID, Message: "No sound", CreatedAt: base, UpdatedAt: base}
		second := core.IssueComment{IssueID: audio.ID, UserID: bob.ID, Message: "Fixed?", CreatedAt: base.Add(time.Minute), UpdatedAt: base.Add(time.Minute)}
		for _, c := range []*core.IssueComment{&second, &first} {
			if err := repo.AddComment(ctx, c); err != nil {
				t.Fatal(err)
			}
		}
		if err := repo.AddComment(ctx, &core.IssueComment{IssueID: core.NewID(), UserID: alice.ID, Message: "x"}); !errors.Is(err, core.ErrConflict) && !errors.Is(err, core.ErrNotFound) {
			t.Errorf("AddComment(unknown issue) = %v, want = ErrConflict", err)
		}
		audio.Comments = []core.IssueComment{first, second}
		got, err := repo.Get(ctx, audio.ID)
		if err != nil || !reflect.DeepEqual(got, audio) {
			t.Fatalf("Get() = %+v, %v, want = %+v", got, err, audio)
		}
		// Put keeps the comments.
		audio.Status, audio.ModifiedBy, audio.UpdatedAt = core.IssueResolved, bob.ID, base.Add(2*time.Hour)
		if err := repo.Put(ctx, &audio); err != nil {
			t.Fatal(err)
		}
		if got, _ := repo.Get(ctx, audio.ID); !reflect.DeepEqual(got, audio) {
			t.Errorf("Get() after Put = %+v, want = %+v", got, audio)
		}
		lists := []struct {
			name string
			q    core.IssueQuery
			want []core.ID
		}{
			{"all", core.IssueQuery{}, []core.ID{video.ID, audio.ID}},
			{"by modification", core.IssueQuery{SortByModified: true}, []core.ID{audio.ID, video.ID}},
			{"by creator", core.IssueQuery{CreatedBy: bob.ID}, []core.ID{video.ID}},
			{"by item and status", core.IssueQuery{ItemID: movie.ID, Statuses: []core.IssueStatus{core.IssueOpen}}, nil},
		}
		for _, tt := range lists {
			p, err := repo.List(ctx, tt.q)
			if err != nil {
				t.Fatalf("%s: List() = %v", tt.name, err)
			}
			var got []core.ID
			for _, i := range p.Items {
				got = append(got, i.ID)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("%s: List() = %v, want = %v", tt.name, got, tt.want)
			}
		}
		if err := repo.Delete(ctx, audio.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Get(ctx, audio.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get(deleted) = %v, want = ErrNotFound", err)
		}
		if err := s.Items().Delete(ctx, movie.ID); err != nil {
			t.Fatal(err)
		}
		if p, _ := repo.List(ctx, core.IssueQuery{}); p.Total != 0 {
			t.Errorf("List() after deleting the item = %d issues, want none", p.Total)
		}
	})
}

func TestUserRequestPolicy(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		u := core.User{ID: core.NewID(), Name: "kid", Kind: core.UserMember, Policy: core.UserPolicy{
			Requests:   core.RequestPermissions{Movies: true, AutoApproveMovies: true, ReportIssues: true},
			MovieQuota: &core.Quota{Limit: 5, Days: 7},
		}}
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		got, err := s.Users().Get(ctx, u.ID)
		if err != nil || !reflect.DeepEqual(got.Policy, u.Policy) {
			t.Errorf("Get().Policy = %+v, %v, want = %+v", got.Policy, err, u.Policy)
		}
	})
}

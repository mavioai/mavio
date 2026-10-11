package core

import (
	"errors"
	"testing"
)

func TestRequestValidate(t *testing.T) {
	id := NewID()
	movie := func(change func(*Request)) validator {
		r := Request{ID: id, Kind: KindMovie, TmdbID: 603, Status: RequestPending, RequestedBy: id}
		change(&r)
		return &r
	}
	series := func(change func(*Request)) validator {
		r := Request{ID: id, Kind: KindSeries, TmdbID: 1396, Status: RequestPending, RequestedBy: id, Seasons: []SeasonRequest{{Number: 1, Status: RequestPending}}}
		change(&r)
		return &r
	}
	issue := func(change func(*Issue)) validator {
		i := Issue{ID: id, ItemID: id, CreatedBy: id, Type: IssueVideo, Status: IssueOpen}
		change(&i)
		return &i
	}
	settings := func(change func(*RequestSettings)) validator {
		s := DefaultRequestSettings()
		change(&s)
		return &s
	}
	tests := []struct {
		name string
		v    validator
		ok   bool
	}{
		{"movie", movie(func(*Request) {}), true},
		{"movie without a user", movie(func(r *Request) { r.RequestedBy = ID{} }), false},
		{"movie without a TMDB ID", movie(func(r *Request) { r.TmdbID = 0 }), false},
		{"movie with seasons", movie(func(r *Request) { r.Seasons = []SeasonRequest{{Number: 1, Status: RequestPending}} }), false},
		{"movie of unknown status", movie(func(r *Request) { r.Status = "queued" }), false},
		{"request of an episode", movie(func(r *Request) { r.Kind = KindEpisode }), false},
		{"series", series(func(*Request) {}), true},
		{"series without seasons", series(func(r *Request) { r.Seasons = nil }), false},
		{"series asking twice", series(func(r *Request) { r.Seasons = append(r.Seasons, r.Seasons[0]) }), false},
		{"series with a negative season", series(func(r *Request) { r.Seasons[0].Number = -1 }), false},
		{"season of unknown status", series(func(r *Request) { r.Seasons[0].Status = "" }), false},
		{"query", &RequestQuery{Statuses: []RequestStatus{RequestApproved}, Kind: KindSeries}, true},
		{"query of an unknown status", &RequestQuery{Statuses: []RequestStatus{"queued"}}, false},
		{"query of episodes", &RequestQuery{Kind: KindEpisode}, false},
		{"query past a page", &RequestQuery{Limit: MaxPageSize + 1}, false},
		{"issue", issue(func(*Issue) {}), true},
		{"issue without an item", issue(func(i *Issue) { i.ItemID = ID{} }), false},
		{"issue of unknown type", issue(func(i *Issue) { i.Type = "picture" }), false},
		{"issue of unknown status", issue(func(i *Issue) { i.Status = "closed" }), false},
		{"issue of a negative episode", issue(func(i *Issue) { i.EpisodeNumber = -1 }), false},
		{"comment", &IssueComment{ID: id, IssueID: id, UserID: id, Message: "no sound"}, true},
		{"empty comment", &IssueComment{ID: id, IssueID: id, UserID: id, Message: " "}, false},
		{"issue query of unknown status", &IssueQuery{Statuses: []IssueStatus{"closed"}}, false},
		{"request settings", settings(func(*RequestSettings) {}), true},
		{"negative quota", settings(func(s *RequestSettings) { s.SeriesQuota.Days = -1 }), false},
		{"unknown availability", settings(func(s *RequestSettings) { s.MinimumAvailability = "tba" }), false},
		{"unknown new item monitoring", settings(func(s *RequestSettings) { s.MonitorNewItems = "future" }), false},
		{"blocked movie", &BlockedTitle{Kind: KindMovie, TmdbID: 603}, true},
		{"blocked episode", &BlockedTitle{Kind: KindEpisode, TmdbID: 603}, false},
		{"blocked title without a TMDB ID", &BlockedTitle{Kind: KindSeries}, false},
		{"blocked title query past a page", &BlockedTitleQuery{Limit: MaxPageSize + 1}, false},
		{"user with a negative quota", &User{ID: id, Name: "u", PasswordHash: "x", Policy: UserPolicy{MovieQuota: &Quota{Limit: -1}}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.v.Validate()
			if (err == nil) != tt.ok || err != nil && !errors.Is(err, ErrInvalid) {
				t.Errorf("Validate() = %v, want ok = %v", err, tt.ok)
			}
		})
	}
}

func TestRequestStatusActive(t *testing.T) {
	for _, s := range RequestStatuses {
		want := s != RequestDeclined && s != RequestCompleted
		if got := s.Active(); got != want {
			t.Errorf("%s.Active() = %v, want = %v", s, got, want)
		}
	}
}

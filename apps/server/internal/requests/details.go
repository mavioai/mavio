package requests

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// TitleDetails is a title's page, as Seerr's GET /movie/:id and /tv/:id
// (server/routes/movie.ts, tv.ts): what the metadata providers tell of
// it, marked with what the server has of it and of each season.
type TitleDetails struct {
	Title
	Status MediaStatus
	// ItemID is its movie or series in a library, if any.
	ItemID core.ID
	// Requests are its active requests, newest first.
	Requests    []core.Request
	Requestable bool
	// Seasons are a series' seasons, by number; specials only when the
	// request settings allow them.
	Seasons []SeasonStatus
}

// SeasonStatus is what the server has of a season of a series, as
// Seerr's season statuses.
type SeasonStatus struct {
	Number, Episodes int
	// PremiereDate is its first episode's air date, if known.
	PremiereDate *time.Time
	Status       MediaStatus
	// RequestID is the active request asking for it, if any.
	RequestID     core.ID
	RequestStatus core.RequestStatus
	Requestable   bool
}

// GetTitle returns a title's page for a user: titles above the user's
// rating limit fail with ErrRating, as Seerr's movie and series routes
// hide them, and titles no metadata provider knows with core.ErrNotFound.
func (s *Service) GetTitle(ctx context.Context, caller core.User, kind core.ItemKind, tmdbID int) (TitleDetails, error) {
	t, err := s.Title(ctx, kind, tmdbID)
	if err != nil {
		return TitleDetails{}, err
	}
	if !RatingAllowed(&caller, t.Rating, t.Country) {
		return TitleDetails{}, fmt.Errorf("%w: %s is rated %q", ErrRating, t.Name, t.Rating)
	}
	x, err := s.index(ctx, kind)
	if err != nil {
		return TitleDetails{}, err
	}
	st, err := s.titleState(ctx, &caller, &x, kind, tmdbID, t.TvdbID)
	if err != nil {
		return TitleDetails{}, err
	}
	out := TitleDetails{Title: t, Status: st.status, ItemID: st.itemID, Requests: st.active, Requestable: st.requestable}
	if kind == core.KindSeries {
		if out.Seasons, err = s.seasonStatuses(ctx, &caller, t, st); err != nil {
			return TitleDetails{}, err
		}
		// A series known season by season is requestable while a season
		// is.
		if len(out.Seasons) > 0 {
			out.Requestable = slices.ContainsFunc(out.Seasons, func(ss SeasonStatus) bool { return ss.Requestable })
		}
	}
	return out, nil
}

// seasonStatuses marks a series' seasons, those the episode listers tell
// of and those the libraries have, as the series is marked: what the
// libraries have of it, requested when an active request asks for it,
// deleted when they have nothing and a completed request asked for it.
func (s *Service) seasonStatuses(ctx context.Context, u *core.User, t Title, st titleState) ([]SeasonStatus, error) {
	infos, err := s.seasonInfos(ctx, t)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return nil, err
	}
	byNumber := map[int]*SeasonStatus{}
	for _, si := range infos {
		byNumber[si.number] = &SeasonStatus{Number: si.number, Episodes: si.episodes, PremiereDate: si.premiere}
	}
	for n, ss := range st.series.seasons {
		if byNumber[n] == nil {
			byNumber[n] = &SeasonStatus{Number: n, Episodes: ss.episodes}
		}
	}
	specials := s.settings().Specials
	may := mayRequest(u, core.KindSeries)
	out := make([]SeasonStatus, 0, len(byNumber))
	for n, season := range byNumber {
		if n == 0 && !specials {
			continue
		}
		if ss := st.series.seasons[n]; ss != nil {
			switch {
			case ss.available():
				season.Status = StatusAvailable
			case ss.files > 0:
				season.Status = StatusPartial
			case ss.wanted:
				season.Status = StatusWanted
			}
		}
		asks := func(r core.Request) bool {
			return slices.ContainsFunc(r.Seasons, func(sr core.SeasonRequest) bool { return sr.Number == n })
		}
		if i := slices.IndexFunc(st.active, asks); i >= 0 {
			r := st.active[i]
			season.RequestID, season.RequestStatus = r.ID, r.Status
			if season.Status == StatusNone {
				season.Status = StatusRequested
			}
		}
		if season.Status == StatusNone && slices.ContainsFunc(st.all, func(r core.Request) bool {
			return r.Status == core.RequestCompleted && asks(r)
		}) {
			season.Status = StatusDeleted
		}
		if st.blocked {
			season.Status = StatusBlocklisted
		}
		season.Requestable = may && (season.Status == StatusNone || season.Status == StatusDeleted)
		out = append(out, *season)
	}
	slices.SortFunc(out, func(a, b SeasonStatus) int { return cmp.Compare(a.Number, b.Number) })
	return out, nil
}

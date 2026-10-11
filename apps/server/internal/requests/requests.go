package requests

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// NewRequest is a request to create.
type NewRequest struct {
	// Kind is core.KindMovie or core.KindSeries.
	Kind   core.ItemKind
	TmdbID int
	// Seasons are the seasons of a series requested, unless AllSeasons.
	Seasons    []int
	AllSeasons bool
	// For is the user requesting, when an administrator requests for
	// them; zero for the caller.
	For core.ID
	// IgnoreQuota leaves the request out of the user's quota;
	// administrators only.
	IgnoreQuota bool
}

// titleKey is the lock key of a title.
func titleKey(kind core.ItemKind, tmdbID int) string {
	return string(kind) + ":" + strconv.Itoa(tmdbID)
}

// mayRequest reports whether a user's policy allows requesting a kind.
func mayRequest(u *core.User, kind core.ItemKind) bool {
	p := u.Policy.Requests
	return u.Admin || kind == core.KindMovie && p.Movies || kind == core.KindSeries && p.Series
}

// autoApproves reports whether a user's requests of a kind are approved at
// once, as Seerr's AUTO_APPROVE permissions and MANAGE_REQUESTS.
func autoApproves(u *core.User, kind core.ItemKind) bool {
	p := u.Policy.Requests
	return u.Admin || kind == core.KindMovie && p.AutoApproveMovies || kind == core.KindSeries && p.AutoApproveSeries
}

// Create requests a movie or seasons of a series, as Seerr's
// MediaRequest.request (server/entity/MediaRequest.ts): the user's
// permission, then the title's rating against the user's limit, then the
// user's quota, then requests of the same title and what the libraries
// have of it; the request is approved at once when the user's requests
// are, which makes its movie or seasons wanted.
func (s *Service) Create(ctx context.Context, caller core.User, n NewRequest) (core.Request, error) {
	if n.Kind != core.KindMovie && n.Kind != core.KindSeries {
		return core.Request{}, fmt.Errorf("%w: request of kind %q", core.ErrInvalid, n.Kind)
	}
	if n.TmdbID <= 0 {
		return core.Request{}, fmt.Errorf("%w: TMDB ID %d", core.ErrInvalid, n.TmdbID)
	}
	user := caller
	if !n.For.IsZero() && n.For != caller.ID {
		if !caller.Admin {
			return core.Request{}, fmt.Errorf("%w: requesting for another user", ErrPermission)
		}
		u, err := s.Store.Users().Get(ctx, n.For)
		if err != nil {
			return core.Request{}, err
		}
		user = u
	}
	if n.IgnoreQuota && !caller.Admin {
		return core.Request{}, fmt.Errorf("%w: ignoring quotas", ErrPermission)
	}
	if !mayRequest(&user, n.Kind) {
		return core.Request{}, fmt.Errorf("%w: %s may not request %ss", ErrPermission, user.Name, n.Kind)
	}
	t, err := s.Title(ctx, n.Kind, n.TmdbID)
	if err != nil {
		return core.Request{}, err
	}
	if !RatingAllowed(&user, t.Rating, t.Country) {
		return core.Request{}, fmt.Errorf("%w: %s is rated %q", ErrRating, t.Name, t.Rating)
	}

	defer s.users.lock(user.ID.String())()
	defer s.titles.lock(titleKey(n.Kind, n.TmdbID))()
	quota, err := s.Quota(ctx, user, n.Kind)
	if err != nil {
		return core.Request{}, err
	}
	// Seerr lets ignoreQuota only lift a set limit.
	ignore := n.IgnoreQuota && quota.Limit > 0
	if !ignore && quota.Restricted {
		return core.Request{}, fmt.Errorf("%w: %s quota of %d in %d days", ErrQuota, n.Kind, quota.Limit, quota.Days)
	}
	active, err := s.activeRequests(ctx, n.Kind, n.TmdbID)
	if err != nil {
		return core.Request{}, err
	}
	m, err := s.findMedia(ctx, n.Kind, n.TmdbID, t.TvdbID)
	if err != nil {
		return core.Request{}, err
	}
	now := s.now()
	r := core.Request{
		ID: core.NewID(), Kind: n.Kind, TmdbID: n.TmdbID, TvdbID: t.TvdbID, Title: t.Name, Year: t.Year,
		Status: core.RequestPending, RequestedBy: user.ID, IgnoreQuota: ignore, CreatedAt: now, UpdatedAt: now,
	}
	if n.Kind == core.KindMovie {
		if len(active) > 0 {
			return core.Request{}, fmt.Errorf("%w: %s is requested already", core.ErrConflict, t.Name)
		}
		switch status, _, err := s.movieStatus(ctx, m); {
		case err != nil:
			return core.Request{}, err
		case status == StatusAvailable:
			return core.Request{}, fmt.Errorf("%w: %s is available already", core.ErrConflict, t.Name)
		case status == StatusWanted:
			return core.Request{}, fmt.Errorf("%w: %s is wanted already", core.ErrConflict, t.Name)
		}
	} else {
		seasons, err := s.requestedSeasons(ctx, t, n)
		if err != nil {
			return core.Request{}, err
		}
		seasons, err = s.freeSeasons(ctx, seasons, active, m, core.ID{})
		if err != nil {
			return core.Request{}, err
		}
		if len(seasons) == 0 {
			return core.Request{}, fmt.Errorf("%w: no seasons of %s left to request", core.ErrConflict, t.Name)
		}
		if !ignore && quota.Limit > 0 && len(seasons) > quota.Remaining {
			return core.Request{}, fmt.Errorf("%w: %d seasons past the %d left", ErrQuota, len(seasons), quota.Remaining)
		}
		for _, n := range seasons {
			r.Seasons = append(r.Seasons, core.SeasonRequest{Number: n, Status: core.RequestPending})
		}
	}
	if !autoApproves(&user, n.Kind) {
		if err := s.Store.Requests().Put(ctx, &r); err != nil {
			return core.Request{}, err
		}
		s.notify(ctx, TypePending, &r, user.ID, caller.ID)
		return r, nil
	}
	setStatus(&r, core.RequestApproved)
	r.ModifiedBy = caller.ID
	if err := s.Store.Requests().Put(ctx, &r); err != nil {
		return core.Request{}, err
	}
	return s.approved(ctx, r, caller.ID, TypeAutoApproved)
}

// requestedSeasons returns the seasons a series request asks for: those
// given, or with all those with episodes, but specials unless the
// settings allow them, as Seerr does.
func (s *Service) requestedSeasons(ctx context.Context, t Title, n NewRequest) ([]int, error) {
	seasons := n.Seasons
	if n.AllSeasons {
		all, err := s.Seasons(ctx, t)
		if err != nil {
			return nil, err
		}
		seasons = slices.DeleteFunc(all, func(n int) bool { return n == 0 })
	}
	if !s.settings().Specials {
		seasons = slices.DeleteFunc(slices.Clone(seasons), func(n int) bool { return n == 0 })
	}
	slices.Sort(seasons)
	return slices.Compact(seasons), nil
}

// freeSeasons leaves out of seasons those other active requests than
// except ask for, and those the libraries have wanted or with files.
func (s *Service) freeSeasons(ctx context.Context, seasons []int, active []core.Request, m media, except core.ID) ([]int, error) {
	st, err := s.seriesState(ctx, m)
	if err != nil {
		return nil, err
	}
	return slices.DeleteFunc(slices.Clone(seasons), func(n int) bool {
		for _, r := range active {
			if r.ID != except && slices.ContainsFunc(r.Seasons, func(sr core.SeasonRequest) bool { return sr.Number == n }) {
				return true
			}
		}
		ss := st.seasons[n]
		return ss != nil && ss.taken()
	}), nil
}

// activeRequests returns the pending, approved and failed requests of a
// title.
func (s *Service) activeRequests(ctx context.Context, kind core.ItemKind, tmdbID int) ([]core.Request, error) {
	page, err := s.Store.Requests().List(ctx, core.RequestQuery{
		Kind: kind, TmdbID: tmdbID, Statuses: []core.RequestStatus{core.RequestPending, core.RequestApproved, core.RequestFailed},
	})
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// setStatus sets a request's status and its seasons'.
func setStatus(r *core.Request, status core.RequestStatus) {
	r.Status = status
	for i := range r.Seasons {
		r.Seasons[i].Status = status
	}
}

// Get returns a request of the caller's, or anyone's for administrators.
func (s *Service) Get(ctx context.Context, caller core.User, id core.ID) (core.Request, error) {
	r, err := s.Store.Requests().Get(ctx, id)
	if err != nil {
		return core.Request{}, err
	}
	if !caller.Admin && r.RequestedBy != caller.ID {
		return core.Request{}, fmt.Errorf("%w: request of another user", ErrPermission)
	}
	return r, nil
}

// List lists the caller's requests, or anyone's for administrators, as
// Seerr's GET /request.
func (s *Service) List(ctx context.Context, caller core.User, q core.RequestQuery) (core.Page[core.Request], error) {
	if !caller.Admin {
		if !q.RequestedBy.IsZero() && q.RequestedBy != caller.ID {
			return core.Page[core.Request]{}, fmt.Errorf("%w: requests of another user", ErrPermission)
		}
		q.RequestedBy = caller.ID
	}
	return s.Store.Requests().List(ctx, q)
}

// Update changes the seasons of a pending series request, as Seerr's PUT
// /request/:id: its user or an administrator; seasons other requests ask
// for or the libraries have are left out, and those added must fit in
// the user's quota.
func (s *Service) Update(ctx context.Context, caller core.User, id core.ID, seasons []int) (core.Request, error) {
	r, err := s.Get(ctx, caller, id)
	if err != nil {
		return core.Request{}, err
	}
	if r.Kind != core.KindSeries {
		return core.Request{}, fmt.Errorf("%w: only the seasons of series requests change", core.ErrInvalid)
	}
	user := caller
	if r.RequestedBy != caller.ID {
		if user, err = s.Store.Users().Get(ctx, r.RequestedBy); err != nil {
			return core.Request{}, err
		}
	}
	defer s.users.lock(user.ID.String())()
	defer s.titles.lock(titleKey(r.Kind, r.TmdbID))()
	if r, err = s.Store.Requests().Get(ctx, id); err != nil {
		return core.Request{}, err
	}
	if r.Status != core.RequestPending {
		return core.Request{}, fmt.Errorf("%w: request %s is %s", ErrState, r.ID, r.Status)
	}
	if !s.settings().Specials {
		seasons = slices.DeleteFunc(slices.Clone(seasons), func(n int) bool { return n == 0 })
	}
	slices.Sort(seasons)
	seasons = slices.Compact(seasons)
	active, err := s.activeRequests(ctx, r.Kind, r.TmdbID)
	if err != nil {
		return core.Request{}, err
	}
	m, err := s.findMedia(ctx, r.Kind, r.TmdbID, r.TvdbID)
	if err != nil {
		return core.Request{}, err
	}
	old := r.SeasonNumbers()
	added := slices.DeleteFunc(slices.Clone(seasons), func(n int) bool { return slices.Contains(old, n) })
	free, err := s.freeSeasons(ctx, added, active, m, r.ID)
	if err != nil {
		return core.Request{}, err
	}
	seasons = slices.DeleteFunc(seasons, func(n int) bool { return slices.Contains(added, n) && !slices.Contains(free, n) })
	if len(seasons) == 0 {
		return core.Request{}, fmt.Errorf("%w: no seasons of %s left to request", core.ErrConflict, r.Title)
	}
	if !r.IgnoreQuota {
		// The edit pays for the seasons the quota has not counted yet, as
		// Seerr's PUT /request/:id.
		quota, err := s.Quota(ctx, user, r.Kind)
		if err != nil {
			return core.Request{}, err
		}
		prior := len(old)
		if quota.Days > 0 && !r.CreatedAt.After(s.now().AddDate(0, 0, -quota.Days)) {
			prior = 0
		}
		if required := len(seasons) - prior; quota.Limit > 0 && required > quota.Remaining {
			return core.Request{}, fmt.Errorf("%w: %d seasons past the %d left", ErrQuota, required, quota.Remaining)
		}
	}
	r.Seasons = r.Seasons[:0]
	for _, n := range seasons {
		r.Seasons = append(r.Seasons, core.SeasonRequest{Number: n, Status: core.RequestPending})
	}
	r.UpdatedAt = s.now()
	if err := s.Store.Requests().Put(ctx, &r); err != nil {
		return core.Request{}, err
	}
	return r, nil
}

// Delete deletes a request: administrators any, users their pending ones,
// as Seerr's DELETE /request/:id.
func (s *Service) Delete(ctx context.Context, caller core.User, id core.ID) error {
	r, err := s.Get(ctx, caller, id)
	if err != nil {
		return err
	}
	if !caller.Admin && r.Status != core.RequestPending {
		return fmt.Errorf("%w: only pending requests are deleted by their users", ErrPermission)
	}
	defer s.titles.lock(titleKey(r.Kind, r.TmdbID))()
	return s.Store.Requests().Delete(ctx, id)
}

// Approve approves a pending request, which makes its movie or seasons
// wanted, as Seerr's POST /request/:id/approve and its
// MediaRequestSubscriber. Administrators only.
func (s *Service) Approve(ctx context.Context, caller core.User, id core.ID) (core.Request, error) {
	return s.transition(ctx, caller, id, core.RequestPending, core.RequestApproved)
}

// Decline declines a pending request, which frees the quota it used.
// Administrators only.
func (s *Service) Decline(ctx context.Context, caller core.User, id core.ID) (core.Request, error) {
	return s.transition(ctx, caller, id, core.RequestPending, core.RequestDeclined)
}

// Retry approves a failed request again, as Seerr's POST
// /request/:id/retry. Administrators only.
func (s *Service) Retry(ctx context.Context, caller core.User, id core.ID) (core.Request, error) {
	return s.transition(ctx, caller, id, core.RequestFailed, core.RequestApproved)
}

func (s *Service) transition(ctx context.Context, caller core.User, id core.ID, from, to core.RequestStatus) (core.Request, error) {
	if !caller.Admin {
		return core.Request{}, fmt.Errorf("%w: only administrators manage requests", ErrPermission)
	}
	r, err := s.Store.Requests().Get(ctx, id)
	if err != nil {
		return core.Request{}, err
	}
	defer s.titles.lock(titleKey(r.Kind, r.TmdbID))()
	if r, err = s.Store.Requests().Get(ctx, id); err != nil {
		return core.Request{}, err
	}
	if r.Status != from {
		return core.Request{}, fmt.Errorf("%w: request %s is %s, not %s", ErrState, r.ID, r.Status, from)
	}
	setStatus(&r, to)
	r.ModifiedBy, r.Message, r.UpdatedAt = caller.ID, "", s.now()
	if err := s.Store.Requests().Put(ctx, &r); err != nil {
		return core.Request{}, err
	}
	if to == core.RequestDeclined {
		s.notify(ctx, TypeDeclined, &r, r.RequestedBy, caller.ID)
		return r, nil
	}
	return s.approved(ctx, r, caller.ID, TypeApproved)
}

// approved makes an approved request's movie or seasons wanted and
// tells its user, or that it is available already, as Seerr does; a
// failure fails the request, which may be retried.
func (s *Service) approved(ctx context.Context, r core.Request, actor core.ID, typ string) (core.Request, error) {
	if err := s.acquire(ctx, &r); err != nil {
		if ctx.Err() != nil {
			return core.Request{}, ctx.Err()
		}
		s.logger().WarnContext(ctx, "request failed", "request", r.ID, "title", r.Title, "err", err)
		setStatus(&r, core.RequestFailed)
		r.Message, r.UpdatedAt = err.Error(), s.now()
		if err := s.Store.Requests().Put(ctx, &r); err != nil {
			return core.Request{}, err
		}
		s.notify(ctx, TypeFailed, &r, r.RequestedBy, actor)
		return r, nil
	}
	r.UpdatedAt = s.now()
	if err := s.Store.Requests().Put(ctx, &r); err != nil {
		return core.Request{}, err
	}
	done, err := s.complete(ctx, &r)
	if err != nil {
		return core.Request{}, err
	}
	if !done {
		s.notify(ctx, typ, &r, r.RequestedBy, actor)
	}
	return r, nil
}

// Quota is how much of a quota a user has used.
type Quota struct {
	// Limit counts requests of movies, or seasons of series, in the last
	// Days days, all time when 0; 0 is unlimited.
	Limit, Days int
	Used        int
	// Remaining is 0 when unlimited.
	Remaining  int
	Restricted bool
}

// Quota returns how much of their quota of a kind a user has used, as
// Seerr's User.getQuota (server/entity/User.ts): administrators are
// unlimited; declined requests and those ignoring quotas do not count.
func (s *Service) Quota(ctx context.Context, u core.User, kind core.ItemKind) (Quota, error) {
	set := s.settings()
	q, own := set.MovieQuota, u.Policy.MovieQuota
	if kind == core.KindSeries {
		q, own = set.SeriesQuota, u.Policy.SeriesQuota
	}
	if own != nil {
		q = *own
	}
	out := Quota{Limit: q.Limit, Days: q.Days}
	if u.Admin {
		out.Limit = 0
	}
	if out.Limit == 0 {
		return out, nil
	}
	var since time.Time
	if q.Days > 0 {
		since = s.now().AddDate(0, 0, -q.Days)
	}
	used, err := s.Store.Requests().QuotaUsed(ctx, u.ID, kind, since)
	if err != nil {
		return Quota{}, err
	}
	out.Used = used
	out.Remaining = max(0, out.Limit-used)
	out.Restricted = out.Limit-used <= 0
	return out, nil
}

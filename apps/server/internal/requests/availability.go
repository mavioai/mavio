package requests

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
)

// JobAvailability is the kind of the recurring job completing the
// requests whose movie or seasons became available, as Seerr's
// availability sync.
const JobAvailability = "requests.availability"

// AvailabilityInterval is how often the availability job runs.
const AvailabilityInterval = 10 * time.Minute

// scheduledPayload is the payload of availability jobs.
type scheduledPayload struct {
	Scheduled bool `json:"scheduled,omitempty"`
}

// availabilityJob returns an availability job at a time; scheduled and
// requested runs have separate keys.
func availabilityJob(at time.Time, scheduled bool) core.Job {
	payload, _ := json.Marshal(scheduledPayload{Scheduled: scheduled})
	key := JobAvailability
	if scheduled {
		key += ":scheduled"
	}
	return core.Job{ID: core.NewID(), Kind: JobAvailability, Payload: payload, UniqueKey: key, MaxAttempts: 1, RunAt: at}
}

// Handlers returns the handler of the availability job, which schedules
// its next run.
func (s *Service) Handlers() map[string]library.Handler {
	return map[string]library.Handler{
		JobAvailability: func(ctx context.Context, job core.Job) ([]core.Job, error) {
			var p scheduledPayload
			if len(job.Payload) > 0 {
				if err := json.Unmarshal(job.Payload, &p); err != nil {
					return nil, fmt.Errorf("%s payload: %w", JobAvailability, err)
				}
			}
			err := s.Reconcile(ctx)
			if !p.Scheduled {
				return nil, err
			}
			return []core.Job{availabilityJob(s.now().Add(AvailabilityInterval), true)}, err
		},
	}
}

// Schedule enqueues the availability job unless it is pending.
func (s *Service) Schedule(ctx context.Context) error {
	job := availabilityJob(s.now(), true)
	if _, err := s.Store.Jobs().Enqueue(ctx, &job); err != nil {
		return fmt.Errorf("schedule %s: %w", JobAvailability, err)
	}
	return nil
}

// approvedStatuses are the statuses of requests that complete.
var approvedStatuses = []core.RequestStatus{core.RequestApproved, core.RequestFailed}

// Imported completes the requests of a movie or series a file was
// imported for, when it is available now: the acquisition runtime's
// Imported hook.
func (s *Service) Imported(ctx context.Context, itemID core.ID) {
	page, err := s.Store.Requests().List(ctx, core.RequestQuery{ItemID: itemID, Statuses: approvedStatuses})
	if err != nil {
		s.logger().WarnContext(ctx, "couldn't read requests of an import", "item", itemID, "err", err)
		return
	}
	for _, r := range page.Items {
		if err := s.completeLocked(ctx, r.ID); err != nil {
			s.logger().WarnContext(ctx, "couldn't complete request", "request", r.ID, "err", err)
		}
	}
}

// Reconcile completes every approved or failed request whose movie or
// seasons are available, whether imported or found by a scan, as Seerr's
// availability sync and MediaSubscriber.
func (s *Service) Reconcile(ctx context.Context) error {
	var ids []core.ID
	for offset := 0; ; offset += core.MaxPageSize {
		page, err := s.Store.Requests().List(ctx, core.RequestQuery{Statuses: approvedStatuses, Ascending: true, Limit: core.MaxPageSize, Offset: offset})
		if err != nil {
			return err
		}
		for _, r := range page.Items {
			ids = append(ids, r.ID)
		}
		if len(page.Items) < core.MaxPageSize {
			break
		}
	}
	for _, id := range ids {
		if err := s.completeLocked(ctx, id); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			s.logger().WarnContext(ctx, "couldn't complete request", "request", id, "err", err)
		}
	}
	return nil
}

// completeLocked completes a request under its title's lock.
func (s *Service) completeLocked(ctx context.Context, id core.ID) error {
	r, err := s.Store.Requests().Get(ctx, id)
	if err != nil {
		return err
	}
	defer s.titles.lock(titleKey(r.Kind, r.TmdbID))()
	if r, err = s.Store.Requests().Get(ctx, id); err != nil {
		return err
	}
	if r.Status != core.RequestApproved && r.Status != core.RequestFailed {
		return nil
	}
	_, err = s.complete(ctx, &r)
	return err
}

// complete completes an approved or failed request whose movie or
// seasons are available, and tells its user, as Seerr's MediaSubscriber:
// a movie is available when an item of it has its file; a season when its
// aired episodes all have files, and the request when all its seasons
// are. It reports whether the request completed.
func (s *Service) complete(ctx context.Context, r *core.Request) (bool, error) {
	if r.Kind == core.KindMovie {
		m, err := s.findMedia(ctx, core.KindMovie, r.TmdbID, 0)
		if err != nil {
			return false, err
		}
		status, itemID, err := s.movieStatus(ctx, m)
		if err != nil || status != StatusAvailable {
			return false, err
		}
		if r.ItemID.IsZero() {
			r.ItemID = itemID
		}
		setStatus(r, core.RequestCompleted)
	} else {
		m, err := s.findMedia(ctx, core.KindSeries, r.TmdbID, r.TvdbID)
		if err != nil {
			return false, err
		}
		st, err := s.seriesState(ctx, m)
		if err != nil {
			return false, err
		}
		changed, done := false, true
		for i := range r.Seasons {
			sr := &r.Seasons[i]
			if sr.Status != core.RequestCompleted {
				if ss := st.seasons[sr.Number]; ss != nil && ss.available() {
					sr.Status, changed = core.RequestCompleted, true
				}
			}
			done = done && sr.Status == core.RequestCompleted
		}
		if !changed {
			return false, nil
		}
		if r.ItemID.IsZero() {
			r.ItemID = st.itemID
		}
		if done {
			r.Status = core.RequestCompleted
		}
		if !done {
			r.UpdatedAt = s.now()
			return false, s.Store.Requests().Put(ctx, r)
		}
	}
	r.Message, r.UpdatedAt = "", s.now()
	if err := s.Store.Requests().Put(ctx, r); err != nil {
		return false, err
	}
	s.notify(ctx, TypeAvailable, r, r.RequestedBy, core.ID{})
	return true, nil
}

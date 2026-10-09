package events

import (
	"errors"
	"path/filepath"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	"github.com/mavioai/mavio/libs/store"
)

// next returns the next event of a stream, or nil when none is queued.
func next(s *Subscriber) *sessionv1.Event {
	select {
	case e := <-s.Events():
		return e
	default:
		return nil
	}
}

// drain discards the queued events of streams.
func drain(subs ...*Subscriber) {
	for _, s := range subs {
		for next(s) != nil {
		}
	}
}

func TestHub(t *testing.T) {
	s, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	ctx := t.Context()
	films := core.Library{Name: "Films", Kind: core.LibraryMovies, Paths: []string{t.TempDir()}}
	shows := core.Library{Name: "Shows", Kind: core.LibraryShows, Paths: []string{t.TempDir()}}
	for _, lib := range []*core.Library{&films, &shows} {
		if err := s.Libraries().Create(ctx, lib); err != nil {
			t.Fatal(err)
		}
	}
	admin := core.User{ID: core.NewID(), Name: "admin", Admin: true}
	kid := core.User{ID: core.NewID(), Name: "kid", PasswordHash: "x", Policy: core.UserPolicy{Libraries: []core.ID{shows.ID}, MaxParentalRating: new(10)}}

	synctest.Test(t, func(t *testing.T) {
		hub := New(Config{Store: s})
		db := Observe(s, hub)
		adminTV := hub.Subscribe(admin, core.NewID())
		drain(adminTV)
		kidTablet := hub.Subscribe(kid, core.NewID())
		// The administrator learns that the kid came online.
		if e := next(adminTV); e.GetSessionsChanged().GetSessionId() != kidTablet.SessionID.String() {
			t.Errorf("admin's event = %v, want the kid's session", e)
		}
		drain(adminTV, kidTablet)

		// Library changes wait for the delay, then reach whoever may see
		// them.
		film := core.Item{ID: core.NewID(), LibraryID: films.ID, Kind: core.KindMovie, Name: "Heat"}
		rated := core.Item{ID: core.NewID(), LibraryID: shows.ID, Kind: core.KindSeries, Name: "Lost", ParentalRating: new(14)}
		show := core.Item{ID: core.NewID(), LibraryID: shows.ID, Kind: core.KindSeries, Name: "Bluey", ParentalRating: new(0)}
		if err := db.Items().Upsert(ctx, film, rated); err != nil {
			t.Fatal(err)
		}
		// A transaction rolled back reports nothing; a committed one
		// reports once committed.
		boom := errors.New("boom")
		err := db.InTx(ctx, func(tx core.Store) error {
			if err := tx.Items().Upsert(ctx, core.Item{ID: core.NewID(), LibraryID: films.ID, Kind: core.KindMovie, Name: "Gone"}); err != nil {
				return err
			}
			return boom
		})
		if !errors.Is(err, boom) {
			t.Fatal(err)
		}
		if err := db.InTx(ctx, func(tx core.Store) error { return tx.Items().Upsert(ctx, show) }); err != nil {
			t.Fatal(err)
		}
		time.Sleep(4 * time.Second)
		synctest.Wait()
		if e := next(adminTV); e != nil {
			t.Errorf("event before the delay: %v", e)
		}
		time.Sleep(time.Second)
		synctest.Wait()
		ids := func(list ...core.ID) []string {
			out := []string{}
			for _, id := range list {
				out = append(out, id.String())
			}
			slices.Sort(out)
			return out
		}
		sorted := func(list []string) []string { return slices.Sorted(slices.Values(list)) }
		e := next(adminTV).GetLibraryChanged()
		if got := sorted(e.GetChangedItemIds()); !slices.Equal(got, ids(film.ID, rated.ID, show.ID)) || len(e.GetLibraryIds()) != 2 {
			t.Errorf("admin's library change = %v", e)
		}
		e = next(kidTablet).GetLibraryChanged()
		if got := e.GetChangedItemIds(); !slices.Equal(got, ids(show.ID)) || !slices.Equal(e.GetLibraryIds(), ids(shows.ID)) {
			t.Errorf("kid's library change = %v", e)
		}

		// Removed items reach those who may see their library.
		if err := db.Items().Delete(ctx, film.ID); err != nil {
			t.Fatal(err)
		}
		time.Sleep(5 * time.Second)
		synctest.Wait()
		if e := next(adminTV).GetLibraryChanged(); !slices.Equal(e.GetRemovedItemIds(), ids(film.ID)) {
			t.Errorf("admin's removal = %v", e)
		}
		if e := next(kidTablet); e != nil {
			t.Errorf("kid learned of a removal in a library it may not see: %v", e)
		}

		// User data reaches the user's devices at once.
		if err := s.Users().Create(ctx, &kid); err != nil {
			t.Fatal(err)
		}
		if err := db.UserData().Put(ctx, &core.UserData{UserID: kid.ID, ItemID: show.ID, Played: true}); err != nil {
			t.Fatal(err)
		}
		if e := next(kidTablet).GetUserDataChanged(); len(e.GetUserData()) != 1 || !e.GetUserData()[0].GetPlayed() {
			t.Errorf("kid's user data = %v", e)
		}
		if e := next(adminTV); e != nil {
			t.Errorf("admin got the kid's user data: %v", e)
		}

		// Administrators follow scans.
		job := library.ScanJob(shows, time.Now(), false)
		if _, err := db.Jobs().Enqueue(ctx, &job); err != nil {
			t.Fatal(err)
		}
		leased, err := db.Jobs().Lease(ctx, "test", []string{library.JobScan}, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.Jobs().Complete(ctx, leased.ID, "test"); err != nil {
			t.Fatal(err)
		}
		for _, want := range []sessionv1.JobState{sessionv1.JobState_JOB_STATE_RUNNING, sessionv1.JobState_JOB_STATE_SUCCEEDED} {
			if e := next(adminTV).GetJobChanged(); e.GetState() != want || e.GetLibraryId() != shows.ID.String() {
				t.Errorf("job event = %v, want %v", e, want)
			}
		}
		if e := next(kidTablet); e != nil {
			t.Errorf("kid got a job event: %v", e)
		}

		// A device subscribing again replaces its stream.
		again := hub.Subscribe(kid, kidTablet.SessionID)
		select {
		case <-kidTablet.Done():
		default:
			t.Error("the replaced stream is not done")
		}
		hub.Unsubscribe(kidTablet) // the old stream ending keeps the new one
		if !hub.Online(kidTablet.SessionID) {
			t.Error("the device went offline with its replaced stream")
		}
		hub.Unsubscribe(again)
		if hub.Online(kidTablet.SessionID) {
			t.Error("the device is online without a stream")
		}

		// A stream too far behind is dropped.
		for range buffer + 1 {
			hub.ToSession(adminTV.SessionID, sessionv1.Event_builder{Heartbeat: &sessionv1.Heartbeat{}}.Build())
		}
		select {
		case <-adminTV.Done():
		default:
			t.Error("a stream too far behind was kept")
		}
	})
}

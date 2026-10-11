package events

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/mavioai/mavio/libs/core"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

func TestNotify(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		hub := New(Config{})
		admin := core.User{ID: core.NewID(), Name: "admin", Admin: true}
		alice := core.User{ID: core.NewID(), Name: "alice"}
		bob := core.User{ID: core.NewID(), Name: "bob"}
		adminTV := hub.Subscribe(admin, core.NewID())
		alicePhone, aliceTV := hub.Subscribe(alice, core.NewID()), hub.Subscribe(alice, core.NewID())
		bobPhone := hub.Subscribe(bob, core.NewID())
		all := []*Subscriber{adminTV, alicePhone, aliceTV, bobPhone}
		drain(all...)

		a := core.Activity{
			ID: core.NewID(), Time: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), Type: "request.approved",
			Severity: core.SeverityWarning, Title: "Approved", UserID: alice.ID, Attributes: map[string]string{"tmdb_id": "603"},
		}
		tests := []struct {
			name         string
			user, admins bool
			want         []*Subscriber
		}{
			{"user", true, false, []*Subscriber{alicePhone, aliceTV}},
			{"admins", false, true, []*Subscriber{adminTV}},
			{"both", true, true, []*Subscriber{adminTV, alicePhone, aliceTV}},
		}
		for _, tt := range tests {
			hub.Notify(a, tt.user, tt.admins)
			for _, s := range all {
				e := next(s)
				wanted := false
				for _, w := range tt.want {
					wanted = wanted || w == s
				}
				if wanted != (e != nil) {
					t.Errorf("%s: %s got %v, want an event = %v", tt.name, s.User.Name, e, wanted)
					continue
				}
				if e == nil {
					continue
				}
				got := e.GetNotification().GetActivity()
				if got.GetId() != a.ID.String() || got.GetType() != a.Type || got.GetSeverity() != systemv1.Severity_SEVERITY_WARNING ||
					got.GetUserId() != alice.ID.String() || got.GetAttributes()["tmdb_id"] != "603" || !got.GetTime().AsTime().Equal(a.Time) {
					t.Errorf("%s: notification = %v", tt.name, got)
				}
			}
		}
	})
}

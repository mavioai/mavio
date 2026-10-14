package store_test

import (
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestAuthSessions(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		alice := core.User{Name: "alice", Kind: core.UserMember}
		if err := s.Users().Create(ctx, &alice); err != nil {
			t.Fatal(err)
		}
		key := func(device string) []byte { h := sha256.Sum256([]byte(device)); return h[:] }
		at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		phone := core.AuthSession{UserID: alice.ID, DeviceKey: key("phone"), DeviceID: "phone", DeviceName: "Pixel", Client: "Mavio Android", ClientVersion: "1.0", LastSeenAt: at}
		tv := core.AuthSession{UserID: alice.ID, DeviceKey: key("tv"), DeviceID: "tv", LastSeenAt: at.Add(time.Hour)}
		for _, sess := range []*core.AuthSession{&phone, &tv} {
			if err := s.AuthSessions().Create(ctx, sess); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.AuthSessions().Get(ctx, phone.ID)
		if err != nil || string(got.DeviceKey) != string(key("phone")) || got.DeviceName != "Pixel" || got.Client != "Mavio Android" || !got.LastSeenAt.Equal(at) {
			t.Errorf("Get = %+v, %v", got, err)
		}
		if _, err := s.AuthSessions().Get(ctx, core.NewID()); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("unknown session: %v, want ErrNotFound", err)
		}
		list, err := s.AuthSessions().ListForUser(ctx, alice.ID)
		if err != nil || len(list) != 2 || list[0].ID != tv.ID {
			t.Errorf("ListForUser = %+v, %v; want the TV first", list, err)
		}

		// Signing in again on the phone replaces its session.
		again := core.AuthSession{UserID: alice.ID, DeviceKey: key("phone again"), DeviceID: "phone"}
		if err := s.AuthSessions().Create(ctx, &again); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AuthSessions().Get(ctx, phone.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("replaced session: %v, want ErrNotFound", err)
		}
		if err := s.AuthSessions().Touch(ctx, again.ID, at.Add(2*time.Hour)); err != nil {
			t.Fatal(err)
		}
		if list, _ := s.AuthSessions().ListForUser(ctx, alice.ID); len(list) != 2 || list[0].ID != again.ID {
			t.Errorf("after Touch = %+v; want the phone first", list)
		}

		if err := s.AuthSessions().Delete(ctx, tv.ID); err != nil {
			t.Fatal(err)
		}
		if err := s.AuthSessions().Delete(ctx, tv.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("second Delete: %v, want ErrNotFound", err)
		}
		if err := s.AuthSessions().Create(ctx, &core.AuthSession{UserID: alice.ID, DeviceKey: []byte("x"), DeviceID: "d"}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("invalid session: %v, want ErrInvalid", err)
		}

		// Sessions unused since before the phone's activity are purged.
		idle := core.AuthSession{UserID: alice.ID, DeviceKey: key("old"), DeviceID: "old", LastSeenAt: at.Add(-time.Hour)}
		if err := s.AuthSessions().Create(ctx, &idle); err != nil {
			t.Fatal(err)
		}
		if n, err := s.AuthSessions().PurgeIdle(ctx, at); err != nil || n != 1 {
			t.Errorf("PurgeIdle = %d, %v, want = 1", n, err)
		}
		if _, err := s.AuthSessions().Get(ctx, idle.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("idle session: %v, want ErrNotFound", err)
		}

		// Switching changes the session's user and remembers the switcher.
		kid := core.User{Name: "kid", Kind: core.UserManaged}
		if err := s.Users().Create(ctx, &kid); err != nil {
			t.Fatal(err)
		}
		if err := s.AuthSessions().Switch(ctx, again.ID, kid.ID, alice.ID); err != nil {
			t.Fatal(err)
		}
		if got, err := s.AuthSessions().Get(ctx, again.ID); err != nil || got.UserID != kid.ID || got.SwitcherID != alice.ID {
			t.Errorf("after Switch = %+v, %v; want kid's, switched by alice", got, err)
		}
		if err := s.AuthSessions().Switch(ctx, core.NewID(), kid.ID, alice.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Switch of an unknown session: %v, want ErrNotFound", err)
		}
		if err := s.AuthSessions().Switch(ctx, again.ID, alice.ID, alice.ID); err != nil {
			t.Fatal(err)
		}

		// Deleting the user signs it out everywhere.
		if err := s.Users().Delete(ctx, alice.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AuthSessions().Get(ctx, again.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("session of a deleted user: %v, want ErrNotFound", err)
		}
	})
}

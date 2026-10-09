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
		alice := core.User{Name: "alice", PasswordHash: "$argon2id$x"}
		if err := s.Users().Create(ctx, &alice); err != nil {
			t.Fatal(err)
		}
		hash := func(token string) []byte { h := sha256.Sum256([]byte(token)); return h[:] }
		at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		phone := core.AuthSession{UserID: alice.ID, TokenHash: hash("one"), DeviceID: "phone", DeviceName: "Pixel", Client: "Mavio Android", ClientVersion: "1.0", LastSeenAt: at}
		tv := core.AuthSession{UserID: alice.ID, TokenHash: hash("two"), DeviceID: "tv", LastSeenAt: at.Add(time.Hour)}
		for _, sess := range []*core.AuthSession{&phone, &tv} {
			if err := s.AuthSessions().Create(ctx, sess); err != nil {
				t.Fatal(err)
			}
		}
		got, err := s.AuthSessions().GetByTokenHash(ctx, hash("one"))
		if err != nil || got.ID != phone.ID || got.DeviceName != "Pixel" || got.Client != "Mavio Android" || !got.LastSeenAt.Equal(at) {
			t.Errorf("GetByTokenHash = %+v, %v", got, err)
		}
		if _, err := s.AuthSessions().GetByTokenHash(ctx, hash("unknown")); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("unknown token: %v, want ErrNotFound", err)
		}
		list, err := s.AuthSessions().ListForUser(ctx, alice.ID)
		if err != nil || len(list) != 2 || list[0].ID != tv.ID {
			t.Errorf("ListForUser = %+v, %v; want the TV first", list, err)
		}

		// Signing in again on the phone replaces its session.
		again := core.AuthSession{UserID: alice.ID, TokenHash: hash("three"), DeviceID: "phone"}
		if err := s.AuthSessions().Create(ctx, &again); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AuthSessions().GetByTokenHash(ctx, hash("one")); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("replaced token: %v, want ErrNotFound", err)
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
		if err := s.AuthSessions().Create(ctx, &core.AuthSession{UserID: alice.ID, TokenHash: []byte("x"), DeviceID: "d"}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("invalid session: %v, want ErrInvalid", err)
		}

		// Deleting the user signs it out everywhere.
		if err := s.Users().Delete(ctx, alice.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.AuthSessions().GetByTokenHash(ctx, hash("three")); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("session of a deleted user: %v, want ErrNotFound", err)
		}
	})
}

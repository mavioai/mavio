package store_test

import (
	"crypto/sha256"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestInvitations(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		hash := func(token string) []byte { h := sha256.Sum256([]byte(token)); return h[:] }
		lib := core.NewID()
		family := core.Invitation{
			TokenHash: hash("family"), MaxUses: 3, ExpiresAt: at.Add(24 * time.Hour), CreatedAt: at,
			Access: core.LibraryAccess{Libraries: []core.ID{lib}, ContentLimits: core.ContentLimits{BlockedLabels: []string{"horror"}}},
		}
		if err := s.Invitations().Create(ctx, &family); err != nil {
			t.Fatal(err)
		}
		got, err := s.Invitations().GetByTokenHash(ctx, hash("family"))
		if err != nil || got.ID != family.ID || got.MaxUses != 3 || len(got.Access.Libraries) != 1 || got.Access.Libraries[0] != lib ||
			len(got.Access.BlockedLabels) != 1 || !got.ExpiresAt.Equal(family.ExpiresAt) || !got.UserID.IsZero() {
			t.Fatalf("GetByTokenHash = %+v, %v", got, err)
		}
		if _, err := s.Invitations().GetByTokenHash(ctx, hash("other")); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("unknown token: %v, want ErrNotFound", err)
		}

		// Concurrent uses never exceed the limit.
		var (
			wg   sync.WaitGroup
			mu   sync.Mutex
			used int
		)
		for range 6 {
			wg.Go(func() {
				err := s.Invitations().Use(ctx, family.ID, at)
				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					used++
				case !errors.Is(err, core.ErrNotFound):
					t.Errorf("Use: %v", err)
				}
			})
		}
		wg.Wait()
		if used != 3 {
			t.Errorf("successful uses = %d, want = 3", used)
		}
		if got, _ := s.Invitations().GetByTokenHash(ctx, hash("family")); got.Uses != 3 {
			t.Errorf("Uses = %d, want = 3", got.Uses)
		}

		// An invitation converting a managed user goes with that user.
		kid := core.User{Name: "kid", Kind: core.UserManaged}
		if err := s.Users().Create(ctx, &kid); err != nil {
			t.Fatal(err)
		}
		convert := core.Invitation{TokenHash: hash("kid"), UserID: kid.ID, MaxUses: 1, ExpiresAt: at.Add(time.Hour), CreatedAt: at.Add(time.Minute)}
		if err := s.Invitations().Create(ctx, &convert); err != nil {
			t.Fatal(err)
		}
		if err := s.Invitations().Use(ctx, convert.ID, at.Add(2*time.Hour)); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Use after expiry: %v, want ErrNotFound", err)
		}
		list, err := s.Invitations().List(ctx)
		if err != nil || len(list) != 2 || list[0].ID != convert.ID || list[0].UserID != kid.ID {
			t.Errorf("List = %+v, %v; want the conversion first", list, err)
		}
		if err := s.Users().Delete(ctx, kid.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Invitations().GetByTokenHash(ctx, hash("kid")); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("invitation of a deleted user: %v, want ErrNotFound", err)
		}

		if n, err := s.Invitations().PurgeExpired(ctx, at.Add(25*time.Hour)); err != nil || n != 1 {
			t.Errorf("PurgeExpired = %d, %v, want = 1", n, err)
		}
		if err := s.Invitations().Delete(ctx, family.ID); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Delete of a purged invitation: %v, want ErrNotFound", err)
		}
		if err := s.Invitations().Create(ctx, &core.Invitation{TokenHash: hash("x"), MaxUses: 0, ExpiresAt: at}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("invitation without uses: %v, want ErrInvalid", err)
		}
	})
}

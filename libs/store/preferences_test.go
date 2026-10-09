package store_test

import (
	"errors"
	"maps"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestDisplayPreferences(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		u := core.User{Name: "viewer", PasswordHash: "x"}
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DisplayPreferences().Get(ctx, u.ID, "web", "home"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("unset preferences: %v, want ErrNotFound", err)
		}
		home := core.DisplayPreferences{UserID: u.ID, Client: "web", View: "home", Values: map[string]string{"sections": "resume,nextup"}}
		tv := core.DisplayPreferences{UserID: u.ID, Client: "tv", View: "home", Values: map[string]string{"sections": "latest"}}
		for _, p := range []*core.DisplayPreferences{&home, &tv} {
			if err := s.DisplayPreferences().Put(ctx, p); err != nil {
				t.Fatal(err)
			}
		}
		// Putting again replaces the values.
		home.Values = map[string]string{"sort": "name"}
		if err := s.DisplayPreferences().Put(ctx, &home); err != nil {
			t.Fatal(err)
		}
		got, err := s.DisplayPreferences().Get(ctx, u.ID, "web", "home")
		if err != nil || !maps.Equal(got.Values, home.Values) || got.UpdatedAt.IsZero() {
			t.Errorf("web home = %+v, %v", got, err)
		}
		if got, err := s.DisplayPreferences().Get(ctx, u.ID, "tv", "home"); err != nil || got.Values["sections"] != "latest" {
			t.Errorf("tv home = %+v, %v", got, err)
		}
		if err := s.DisplayPreferences().Put(ctx, &core.DisplayPreferences{UserID: u.ID, View: "home"}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("no client: %v, want ErrInvalid", err)
		}
		// Deleting the user deletes their preferences.
		if err := s.Users().Delete(ctx, u.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.DisplayPreferences().Get(ctx, u.ID, "web", "home"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("after deleting the user: %v, want ErrNotFound", err)
		}
	})
}

package store_test

import (
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestSettings(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		got, err := s.Settings().Get(ctx)
		if err != nil || got.Transcoding.H264CRF != 23 || !got.UpdatedAt.IsZero() {
			t.Fatalf("Get before Put = %+v, %v; want the defaults", got, err)
		}
		set := core.DefaultServerSettings()
		set.Transcoding.EncoderPreset = "veryfast"
		set.Network.BaseURL = "/mavio"
		set.PluginCatalogs = []string{"https://plugins.example/catalog.json"}
		if err := s.Settings().Put(ctx, &set); err != nil {
			t.Fatal(err)
		}
		got, err = s.Settings().Get(ctx)
		if err != nil || got.Transcoding.EncoderPreset != "veryfast" || got.Network.BaseURL != "/mavio" ||
			len(got.PluginCatalogs) != 1 || got.UpdatedAt.IsZero() {
			t.Errorf("Get = %+v, %v", got, err)
		}
		bad := set
		bad.Transcoding.HardwareAcceleration = "magic"
		if err := s.Settings().Put(ctx, &bad); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("Put of bad settings: %v, want ErrInvalid", err)
		}
	})
}

func TestAPIKeys(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		u := core.User{ID: core.NewID(), Name: "admin", PasswordHash: "x", Admin: true}
		if err := s.Users().Create(ctx, &u); err != nil {
			t.Fatal(err)
		}
		hash := sha256.Sum256([]byte("token"))
		k := core.APIKey{UserID: u.ID, Name: "Home Assistant", TokenHash: hash[:]}
		if err := s.APIKeys().Create(ctx, &k); err != nil {
			t.Fatal(err)
		}
		got, err := s.APIKeys().GetByTokenHash(ctx, hash[:])
		if err != nil || got.ID != k.ID || got.Name != "Home Assistant" || got.LastUsedAt != nil {
			t.Fatalf("GetByTokenHash = %+v, %v", got, err)
		}
		at := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
		if err := s.APIKeys().Touch(ctx, k.ID, at); err != nil {
			t.Fatal(err)
		}
		list, err := s.APIKeys().List(ctx)
		if err != nil || len(list) != 1 || list[0].LastUsedAt == nil || !list[0].LastUsedAt.Equal(at) {
			t.Errorf("List = %+v, %v", list, err)
		}
		// Keys go with their user.
		if err := s.Users().Delete(ctx, u.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.APIKeys().GetByTokenHash(ctx, hash[:]); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("key of a deleted user: %v, want ErrNotFound", err)
		}
		if err := s.APIKeys().Create(ctx, &core.APIKey{UserID: u.ID, TokenHash: hash[:]}); !errors.Is(err, core.ErrInvalid) {
			t.Errorf("key without a name: %v, want ErrInvalid", err)
		}
	})
}

func TestActivities(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		user := core.NewID()
		base := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
		for i, a := range []core.Activity{
			{Type: "user.login", Severity: core.SeverityInfo, Title: "admin signed in", UserID: user},
			{Type: "user.login_failed", Severity: core.SeverityWarning, Title: "failed sign-in", Attributes: map[string]string{"name": "bob"}},
			{Type: "plugin.failed", Severity: core.SeverityError, Title: "plugin failed"},
		} {
			a.Time = base.Add(time.Duration(i) * time.Hour)
			if err := s.Activities().Add(ctx, &a); err != nil {
				t.Fatal(err)
			}
		}
		page, err := s.Activities().List(ctx, core.ActivityQuery{MinSeverity: core.SeverityWarning})
		if err != nil || page.Total != 2 || page.Items[0].Type != "plugin.failed" || page.Items[1].Attributes["name"] != "bob" {
			t.Errorf("List(warnings) = %+v, %v", page, err)
		}
		page, err = s.Activities().List(ctx, core.ActivityQuery{UserID: user})
		if err != nil || page.Total != 1 || page.Items[0].UserID != user || !page.Items[0].Time.Equal(base) {
			t.Errorf("List(user) = %+v, %v", page, err)
		}
		if n, err := s.Activities().Purge(ctx, base.Add(90*time.Minute)); err != nil || n != 2 {
			t.Errorf("Purge = %d, %v; want 2", n, err)
		}
	})
}

func TestJobListAndPurge(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		q := s.Jobs()
		for _, kind := range []string{"a", "b", "a"} {
			if _, err := q.Enqueue(ctx, &core.Job{Kind: kind, MaxAttempts: 1}); err != nil {
				t.Fatal(err)
			}
		}
		leased, err := q.Lease(ctx, "w", []string{"b"}, time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if err := q.Complete(ctx, leased.ID, "w"); err != nil {
			t.Fatal(err)
		}
		page, err := q.List(ctx, core.JobQuery{Kinds: []string{"a"}})
		if err != nil || page.Total != 2 {
			t.Errorf("List(a) = %+v, %v", page, err)
		}
		page, err = q.List(ctx, core.JobQuery{States: []core.JobState{core.JobSucceeded}})
		if err != nil || page.Total != 1 || page.Items[0].ID != leased.ID || page.Items[0].FinishedAt == nil {
			t.Errorf("List(succeeded) = %+v, %v", page, err)
		}
		if n, err := q.Purge(ctx, time.Now().Add(time.Minute)); err != nil || n != 1 {
			t.Errorf("Purge = %d, %v; want 1", n, err)
		}
		if page, _ := q.List(ctx, core.JobQuery{}); page.Total != 2 {
			t.Errorf("jobs after purging = %d, want 2", page.Total)
		}
	})
}

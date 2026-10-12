package store_test

import (
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestStorageJobs(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		repo := s.StorageJobs()
		if _, err := repo.Get(ctx, core.StorageJobOffline, "c/ABC"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get before Put: %v, want ErrNotFound", err)
		}
		offline := core.StorageJob{Kind: core.StorageJobOffline, Key: "c/ABC", Remote: "drive", Data: map[string]string{"task": "1"}}
		if err := repo.Put(ctx, &offline); err != nil {
			t.Fatal(err)
		}
		created := offline.CreatedAt
		if created.IsZero() {
			t.Error("Put left CreatedAt unset")
		}
		transfer := core.StorageJob{Kind: core.StorageJobTransfer, Key: "c/ABC", Remote: "drive", ExecuteID: "e1", JobID: 7}
		if err := repo.Put(ctx, &transfer); err != nil {
			t.Fatal(err)
		}

		// Put replaces the job of the same kind and key, keeping when it
		// was created.
		time.Sleep(10 * time.Millisecond)
		offline.Data = map[string]string{"task": "1", "status": "complete"}
		if err := repo.Put(ctx, &offline); err != nil {
			t.Fatal(err)
		}
		got, err := repo.Get(ctx, core.StorageJobOffline, "c/ABC")
		if err != nil {
			t.Fatal(err)
		}
		if !maps.Equal(got.Data, offline.Data) || !got.CreatedAt.Equal(created) || !got.UpdatedAt.After(created) {
			t.Errorf("Get = %+v, want = data %v created at %v", got, offline.Data, created)
		}
		got, err = repo.Get(ctx, core.StorageJobTransfer, "c/ABC")
		if err != nil || got.ExecuteID != "e1" || got.JobID != 7 || got.Remote != "drive" {
			t.Errorf("Get(transfer) = %+v, %v", got, err)
		}

		second := core.StorageJob{Kind: core.StorageJobOffline, Key: "c/DEF", Remote: "drive"}
		if err := repo.Put(ctx, &second); err != nil {
			t.Fatal(err)
		}
		list, err := repo.List(ctx, core.StorageJobOffline)
		if err != nil || len(list) != 2 || list[0].Key != "c/ABC" || list[1].Key != "c/DEF" {
			t.Errorf("List = %+v, %v; want c/ABC, c/DEF", list, err)
		}

		if err := repo.Delete(ctx, core.StorageJobOffline, "c/ABC"); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.Get(ctx, core.StorageJobOffline, "c/ABC"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get after Delete: %v, want ErrNotFound", err)
		}
		if _, err := repo.Get(ctx, core.StorageJobTransfer, "c/ABC"); err != nil {
			t.Errorf("Delete removed the transfer of the same key: %v", err)
		}
		if err := repo.Delete(ctx, core.StorageJobOffline, "none"); err != nil {
			t.Errorf("Delete of nothing: %v", err)
		}

		for _, bad := range []core.StorageJob{
			{Kind: "other", Key: "k", Remote: "r"},
			{Kind: core.StorageJobOffline, Remote: "r"},
			{Kind: core.StorageJobOffline, Key: "k"},
		} {
			if err := repo.Put(ctx, &bad); !errors.Is(err, core.ErrInvalid) {
				t.Errorf("Put(%+v) error = %v, want ErrInvalid", bad, err)
			}
		}
	})
}

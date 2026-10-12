package store_test

import (
	"reflect"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestLibraryStorageClasses(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		lib := core.Library{
			Name: "Films", Kind: core.LibraryMovies, Paths: []string{"/media/films", "/mnt/cloud/films/"},
			StorageClasses: map[string]core.StorageClass{"/mnt/cloud/films/": core.StorageCloud},
		}
		if err := s.Libraries().Create(ctx, &lib); err != nil {
			t.Fatal(err)
		}
		want := map[string]core.StorageClass{"/mnt/cloud/films": core.StorageCloud}
		if got, _ := s.Libraries().Get(ctx, lib.ID); !reflect.DeepEqual(got.StorageClasses, want) {
			t.Errorf("storage classes = %v, want = %v", got.StorageClasses, want)
		}
		lib.StorageClasses = nil
		if err := s.Libraries().Update(ctx, &lib); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.Libraries().Get(ctx, lib.ID); len(got.StorageClasses) != 0 {
			t.Errorf("storage classes after clearing = %v", got.StorageClasses)
		}
	})
}

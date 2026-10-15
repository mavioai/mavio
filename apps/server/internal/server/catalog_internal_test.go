package server

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestSeedCatalog(t *testing.T) {
	ctx := t.Context()
	db, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	set, err := settings.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	const official = "https://catalog.example/plugins.json"
	if err := seedCatalog(ctx, set, official); err != nil {
		t.Fatal(err)
	}
	if got := set.Get().PluginCatalogs; !slices.Equal(got, []string{official}) {
		t.Errorf("catalogs of a new server = %v, want [%s]", got, official)
	}
	// Once the settings are stored, the administrator's catalogs stay.
	if _, err := set.Change(ctx, func(s *core.ServerSettings) error {
		s.PluginCatalogs = nil
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	again, err := settings.Open(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	if err := seedCatalog(ctx, again, official); err != nil {
		t.Fatal(err)
	}
	if got := again.Get().PluginCatalogs; len(got) != 0 {
		t.Errorf("catalogs after the administrator removed them = %v, want none", got)
	}
}

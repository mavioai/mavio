package store_test

import (
	"errors"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestPluginConfigs(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		const id = "org.mavio.scraper-tmdb"
		if _, err := s.PluginConfigs().Get(ctx, id); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get before Put: %v, want ErrNotFound", err)
		}
		first := core.PluginConfig{PluginID: id, JSON: `{"api_key":"one"}`}
		if err := s.PluginConfigs().Put(ctx, &first); err != nil {
			t.Fatal(err)
		}
		// Put replaces the plugin's configuration.
		second := core.PluginConfig{PluginID: id, JSON: `{"api_key":"two","include_adult":true}`}
		if err := s.PluginConfigs().Put(ctx, &second); err != nil {
			t.Fatal(err)
		}
		got, err := s.PluginConfigs().Get(ctx, id)
		if err != nil || got.JSON != second.JSON || got.UpdatedAt.IsZero() {
			t.Errorf("Get = %+v, %v; want = %s", got, err, second.JSON)
		}
		other := core.PluginConfig{PluginID: "org.example.other", JSON: `{}`}
		if err := s.PluginConfigs().Put(ctx, &other); err != nil {
			t.Fatal(err)
		}
		if got, _ := s.PluginConfigs().Get(ctx, id); got.JSON != second.JSON {
			t.Errorf("another plugin's Put changed this one: got = %s", got.JSON)
		}

		if err := s.PluginConfigs().Delete(ctx, "org.example.other"); err != nil {
			t.Fatal(err)
		}
		if _, err := s.PluginConfigs().Get(ctx, "org.example.other"); !errors.Is(err, core.ErrNotFound) {
			t.Errorf("Get after Delete: %v, want ErrNotFound", err)
		}
		if err := s.PluginConfigs().Delete(ctx, "org.example.none"); err != nil {
			t.Errorf("Delete of nothing: %v", err)
		}

		for _, bad := range []core.PluginConfig{{PluginID: "", JSON: `{}`}, {PluginID: id, JSON: `{"api_key":`}} {
			if err := s.PluginConfigs().Put(ctx, &bad); !errors.Is(err, core.ErrInvalid) {
				t.Errorf("Put(%+v) error = %v, want ErrInvalid", bad, err)
			}
		}
	})
}

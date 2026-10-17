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
		const id = "dev.mavio.scraper-tmdb"
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

func TestPluginContainers(t *testing.T) {
	eachBackend(t, func(t *testing.T, s *store.Store) {
		ctx := t.Context()
		r := s.PluginContainers()
		const id = "dev.mavio.voice"
		if got, err := r.List(ctx, id); err != nil || len(got) != 0 {
			t.Errorf("List before Put = %+v, %v, want none", got, err)
		}
		puts := []core.PluginContainer{
			{PluginID: id, Name: "whisper", Mode: core.ContainerUnset, Secrets: map[string]string{"api-key": "s3cret"}},
			{PluginID: id, Name: "tts", Mode: core.ContainerManaged, UseGPU: true},
			{PluginID: "org.example.other", Name: "whisper", Mode: core.ContainerUnset},
		}
		for i := range puts {
			if err := r.Put(ctx, &puts[i]); err != nil {
				t.Fatal(err)
			}
		}
		// Put replaces the dependency of the name.
		ext := core.PluginContainer{PluginID: id, Name: "whisper", Mode: core.ContainerExternal, Address: "http://10.0.0.2:9000", Secrets: map[string]string{"api-key": "s3cret"}}
		if err := r.Put(ctx, &ext); err != nil {
			t.Fatal(err)
		}
		got, err := r.List(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(got) != 2 || got[0].Name != "tts" || got[1].Name != "whisper" {
			t.Fatalf("List = %+v, want tts and whisper", got)
		}
		if g := got[0]; g.Mode != core.ContainerManaged || !g.UseGPU || g.Address != "" || len(g.Secrets) != 0 || g.UpdatedAt.IsZero() {
			t.Errorf("tts = %+v, want managed with a GPU", g)
		}
		if g := got[1]; g.Mode != core.ContainerExternal || g.Address != ext.Address || g.Secrets["api-key"] != "s3cret" || g.UseGPU {
			t.Errorf("whisper = %+v, want = %+v", g, ext)
		}

		if err := r.Delete(ctx, id, "tts"); err != nil {
			t.Fatal(err)
		}
		if got, _ := r.List(ctx, id); len(got) != 1 || got[0].Name != "whisper" {
			t.Errorf("List after Delete of tts = %+v, want whisper", got)
		}
		if err := r.Delete(ctx, id, ""); err != nil {
			t.Fatal(err)
		}
		if got, _ := r.List(ctx, id); len(got) != 0 {
			t.Errorf("List after Delete of all = %+v, want none", got)
		}
		if got, _ := r.List(ctx, "org.example.other"); len(got) != 1 {
			t.Errorf("Delete removed another plugin's: List = %+v", got)
		}
		if err := r.Delete(ctx, "org.example.none", ""); err != nil {
			t.Errorf("Delete of nothing: %v", err)
		}

		for _, bad := range []core.PluginContainer{
			{Name: "whisper", Mode: core.ContainerUnset},
			{PluginID: id, Name: "whisper", Mode: core.ContainerExternal},
			{PluginID: id, Name: "whisper", Mode: "compose"},
		} {
			if err := r.Put(ctx, &bad); !errors.Is(err, core.ErrInvalid) {
				t.Errorf("Put(%+v) error = %v, want ErrInvalid", bad, err)
			}
		}
	})
}

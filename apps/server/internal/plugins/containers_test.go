package plugins_test

import (
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store"
)

func TestContainers(t *testing.T) {
	ctx := t.Context()
	s, err := store.Open(ctx, "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	const id = "dev.mavio.smoke"
	for _, name := range []string{"whisper", "gone"} {
		if err := s.PluginContainers().Put(ctx, &core.PluginContainer{PluginID: id, Name: name, Mode: core.ContainerUnset}); err != nil {
			t.Fatal(err)
		}
	}
	dir := t.TempDir()
	installWith(t, dir, "smoke", "{}", `"capabilities":["CAPABILITY_METADATA_PROVIDER"],
		"containers":[{"name":"whisper","image":"whisper:1@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef","port":9000}]`)
	m := open(t, dir, s)

	// Starting the plugin deletes what its manifest no longer declares.
	if got, err := s.PluginContainers().List(ctx, id); err != nil || len(got) != 1 || got[0].Name != "whisper" {
		t.Errorf("containers kept = %+v, %v, want whisper only", got, err)
	}
	if mf, ok := m.Manifest(id); !ok || len(mf.GetContainers()) != 1 {
		t.Errorf("Manifest = %v, %v, want the manifest with its container", mf, ok)
	}
	if err := m.Reconfigure(ctx, id); err != nil {
		t.Errorf("Reconfigure = %v", err)
	}

	if err := m.Uninstall(ctx, id); err != nil {
		t.Fatal(err)
	}
	if got, err := s.PluginContainers().List(ctx, id); err != nil || len(got) != 0 {
		t.Errorf("containers kept after Uninstall = %+v, %v, want none", got, err)
	}
	if _, ok := m.Manifest(id); ok {
		t.Error("Manifest after Uninstall found the plugin")
	}
}

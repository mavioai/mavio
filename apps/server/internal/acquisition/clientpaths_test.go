package acquisition

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
)

// TestClientPaths checks the folders of a client: unread ones get a
// remote of the client's host suggested, and mapped ones are read and
// hard link to libraries on their volume.
func TestClientPaths(t *testing.T) {
	e := newEnv(t)
	// The NAS the client runs on keeps its downloads in
	// /volume1/downloads/complete, which its SFTP remote sees below its
	// root.
	nas := t.TempDir()
	if err := os.MkdirAll(filepath.Join(nas, "downloads", "complete"), 0o755); err != nil {
		t.Fatal(err)
	}
	e.rt.Storages = &library.Storages{Remote: func(_ context.Context, _, dir string) (core.Storage, error) {
		return library.OpenLocalStorage(filepath.Join(nas, filepath.FromSlash(dir)))
	}}
	e.rt.RemoteHosts = func(context.Context) (map[string]string, error) {
		return map[string]string{"nas": "seedbox:22", "cloud": ""}, nil
	}
	e.client.roots = []string{"/volume1/downloads/complete", "/volume1/other"}

	paths, failure, err := e.rt.ClientPaths(e.ctx, "dl", "qb")
	if err != nil || failure != "" || len(paths) != 2 {
		t.Fatalf("ClientPaths = %+v, %q, %v; want = two folders", paths, failure, err)
	}
	if got := paths[0]; got.Readable || !got.RootFolder || got.SuggestedPath != "remote:nas:downloads/complete" {
		t.Errorf("paths[0] = %+v; want = unread, remote:nas:downloads/complete suggested", got)
	}
	if got := paths[1]; got.Readable || got.SuggestedPath != "" {
		t.Errorf("paths[1] = %+v; want = unread, nothing suggested", got)
	}

	paths, _, err = e.rt.SetClientPaths(e.ctx, "dl", "qb", []ClientPath{
		{RemotePath: "/volume1/", LocalPath: e.dlDir},
		{RemotePath: "/volume1/other"},
	})
	if err != nil || len(paths) != 1 {
		t.Fatalf("SetClientPaths = %+v, %v; want = the one mapping", paths, err)
	}
	got := paths[0]
	if !got.Readable || !got.RootFolder || got.LocalPath != e.dlDir {
		t.Errorf("paths[0] = %+v; want = %s read, holding the client's folders", got, e.dlDir)
	}
	if same, ok := sameDevice(e.dlDir, e.libDir); ok && same && len(got.LinkLibraryPaths) != 1 {
		t.Errorf("LinkLibraryPaths = %v, want = [%s]", got.LinkLibraryPaths, e.libDir)
	}

	if _, _, err := e.rt.ClientPaths(e.ctx, "dl", "nope"); err == nil {
		t.Error("ClientPaths of an unknown client: got = nil error")
	}
}

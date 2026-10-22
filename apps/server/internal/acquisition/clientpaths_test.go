package acquisition

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/downloads"
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

// TestClientPathsContainer checks that the folders of a client's managed
// container map to the server's folders mounted there, which stay out of
// the stored mappings.
func TestClientPathsContainer(t *testing.T) {
	e := newEnv(t)
	e.client.container = "qbittorrent"
	e.client.roots = []string{"/downloads/complete"}
	e.rt.ContainerFolders = func(_ context.Context, pluginID, name string) (map[string]string, error) {
		if pluginID != "dl" || name != "qbittorrent" {
			t.Errorf("ContainerFolders(%s, %s), want = (dl, qbittorrent)", pluginID, name)
		}
		return map[string]string{"/downloads": e.dlDir}, nil
	}
	want := ClientPath{RemotePath: "/downloads", LocalPath: e.dlDir, Container: true, RootFolder: true, Readable: true}
	check := func(what string, paths []ClientPath, err error) {
		t.Helper()
		if err != nil || len(paths) != 1 {
			t.Fatalf("%s = %+v, %v; want = the container's folder", what, paths, err)
		}
		got := paths[0]
		got.LinkLibraryPaths = nil
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: paths[0] = %+v; want = %+v", what, got, want)
		}
	}
	paths, _, err := e.rt.ClientPaths(e.ctx, "dl", "qb")
	check("ClientPaths", paths, err)
	paths, _, err = e.rt.SetClientPaths(e.ctx, "dl", "qb", paths)
	check("SetClientPaths", paths, err)
	added := append(paths, ClientPath{RemotePath: "/elsewhere", LocalPath: e.dlDir})
	if _, _, err := e.rt.SetClientPaths(e.ctx, "dl", "qb", added); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("SetClientPaths with a folder of its own: err = %v, want = %v", err, core.ErrInvalid)
	}
	if stored, err := e.rt.Store.DownloadPathMappings().List(e.ctx); err != nil || len(stored) != 0 {
		t.Errorf("stored mappings = %+v, %v; want = none", stored, err)
	}
}

// TestClientPathsOffline checks that the folder of a cloud drive's
// offline downloads moves files to the libraries on its remote, and takes
// no mappings.
func TestClientPathsOffline(t *testing.T) {
	e := newEnv(t)
	drive := t.TempDir()
	for _, dir := range []string{"downloads", "media/movies"} {
		if err := os.MkdirAll(filepath.Join(drive, filepath.FromSlash(dir)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	e.rt.Storages = &library.Storages{Remote: func(_ context.Context, _, dir string) (core.Storage, error) {
		return library.OpenLocalStorage(filepath.Join(drive, filepath.FromSlash(dir)))
	}}
	e.rt.Clients = func() []downloads.Provider { return []downloads.Provider{{ID: downloads.OfflineID, Client: e.client}} }
	e.client.roots = []string{"remote:pp:downloads"}
	drive2 := core.Library{ID: core.NewID(), Name: "Drive", Kind: core.LibraryMovies, Paths: []string{"remote:pp:media/movies", "remote:other:shows"}}
	if err := e.rt.Store.Libraries().Create(e.ctx, &drive2); err != nil {
		t.Fatal(err)
	}

	paths, failure, err := e.rt.ClientPaths(e.ctx, downloads.OfflineID, "qb")
	if err != nil || failure != "" || len(paths) != 1 {
		t.Fatalf("ClientPaths = %+v, %q, %v; want = the drive's folder", paths, failure, err)
	}
	want := ClientPath{RemotePath: "remote:pp:downloads", RootFolder: true, Readable: true, Moves: true, LinkLibraryPaths: []string{"remote:pp:media/movies"}}
	if got := paths[0]; !reflect.DeepEqual(got, want) {
		t.Errorf("paths[0] = %+v; want = %+v", got, want)
	}
	if _, _, err := e.rt.SetClientPaths(e.ctx, downloads.OfflineID, "qb", []ClientPath{{RemotePath: "remote:pp:downloads", LocalPath: e.dlDir}}); !errors.Is(err, core.ErrInvalid) {
		t.Errorf("SetClientPaths with a mapping: err = %v, want = %v", err, core.ErrInvalid)
	}
}

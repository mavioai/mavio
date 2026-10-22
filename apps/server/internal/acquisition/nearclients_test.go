package acquisition

import (
	"testing"

	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/libs/core"
)

func TestChooseClientPrefersLibraryStorage(t *testing.T) {
	e := newEnv(t)
	far := &fakeClient{}
	e.rt.Clients = func() []downloads.Provider {
		return []downloads.Provider{{ID: "dl", Client: e.client}, {ID: "far", Client: far}}
	}
	// dl's folder is on the library's file system, so imports hard link
	// from it; far's is not read by the server.
	if err := e.db.DownloadPathMappings().ReplaceClient(e.ctx, "dl", "qb", []core.DownloadPathMapping{{RemotePath: "/downloads/", LocalPath: e.dlDir}}); err != nil {
		t.Fatal(err)
	}
	if err := e.db.DownloadPathMappings().ReplaceClient(e.ctx, "far", "qb", []core.DownloadPathMapping{{RemotePath: "/downloads/", LocalPath: "/nonexistent/downloads"}}); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		libs []string
		want []string
	}{
		{"library", e.lib.Paths, []string{"dl/qb", "dl/qb", "dl/qb"}},
		{"no library", nil, []string{"far/qb", "dl/qb", "far/qb"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i, want := range tt.want {
				c, err := e.rt.chooseClient(e.ctx, clientChoice{protocol: core.ProtocolTorrent, libraryPaths: tt.libs})
				if err != nil {
					t.Fatal(err)
				}
				if got := c.Ref(); got != want {
					t.Errorf("choice %d = %s, want = %s", i, got, want)
				}
			}
		})
	}
}

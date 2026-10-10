package providers

import (
	"context"
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

type fakeResolver struct{ got *pluginv1.Folder }

func (f *fakeResolver) Ignore(_ context.Context, req *pluginv1.IgnoreRequest) (*pluginv1.IgnoreResponse, error) {
	f.got = req.GetFolder()
	return pluginv1.IgnoreResponse_builder{Names: []string{"junk"}}.Build(), nil
}

func (f *fakeResolver) Resolve(_ context.Context, req *pluginv1.ResolveRequest) (*pluginv1.ResolveResponse, error) {
	f.got = req.GetFolder()
	if req.GetFolder().GetPath() == "/media/other" {
		return &pluginv1.ResolveResponse{}, nil
	}
	return pluginv1.ResolveResponse_builder{
		Claimed:    proto.Bool(true),
		FolderItem: pluginv1.ResolvedItem_builder{Kind: pluginv1.MediaKind_MEDIA_KIND_SERIES.Enum(), Name: proto.String("Show"), Year: proto.Int32(2001)}.Build(),
		Items: []*pluginv1.ResolvedItem{pluginv1.ResolvedItem_builder{
			Kind: pluginv1.MediaKind_MEDIA_KIND_EPISODE.Enum(), Entry: proto.String("a.mkv"), Name: proto.String("Pilot"),
			ParentIndex: proto.Int32(1), Index: proto.Int32(1), Parts: []string{"b.mkv"},
			ExternalIds: map[string]string{"tvdb": "7"},
		}.Build()},
		Subfolders: []*pluginv1.Subfolder{pluginv1.Subfolder_builder{
			Name: proto.String("S2"), Parent: pluginv1.Container_CONTAINER_SERIES.Enum(), SeasonNumber: proto.Int32(2),
		}.Build()},
	}.Build(), nil
}

func TestResolverPlugin(t *testing.T) {
	f := &fakeResolver{}
	p := &ResolverPlugin{ID: "r", Client: f}
	season := 3
	scope := library.Scope{Kind: core.LibraryShows, Root: "/media", Parent: library.InSeason, SeasonNumber: &season}
	entries := []library.Entry{{Path: "/media/show/a.mkv"}, {Path: "/media/show/S2", IsDir: true}}

	ignored, err := p.Ignore(t.Context(), scope, "/media/show", entries)
	if err != nil || !slices.Equal(ignored, []string{"/media/show/junk"}) {
		t.Errorf("Ignore() = %v, %v, want [/media/show/junk]", ignored, err)
	}
	got := f.got
	if got.GetLibraryKind() != pluginv1.LibraryKind_LIBRARY_KIND_SHOWS || got.GetParent() != pluginv1.Container_CONTAINER_SEASON ||
		got.GetSeasonNumber() != 3 || got.GetRoot() != "/media" || len(got.GetEntries()) != 2 ||
		got.GetEntries()[1].GetName() != "S2" || !got.GetEntries()[1].GetIsDir() {
		t.Errorf("folder = %v", got)
	}

	res, err := p.Resolve(t.Context(), scope, "/media/show", entries)
	if err != nil || res == nil {
		t.Fatalf("Resolve() = %v, %v", res, err)
	}
	if it := res.Item; it == nil || it.Kind != core.KindSeries || it.Path != "/media/show" || it.Year == nil || *it.Year != 2001 {
		t.Errorf("folder item = %+v", res.Item)
	}
	if len(res.Items) != 1 {
		t.Fatalf("items = %+v", res.Items)
	}
	ep := res.Items[0]
	if ep.Kind != core.KindEpisode || ep.Path != "/media/show/a.mkv" || *ep.ParentIndex != 1 || *ep.Index != 1 || ep.Year != nil ||
		!slices.Equal(ep.Parts, []string{"/media/show/b.mkv"}) || ep.ExternalIDs["tvdb"] != "7" {
		t.Errorf("episode = %+v", ep)
	}
	want := library.Subfolder{Path: "/media/show/S2", Scope: library.Scope{Kind: core.LibraryShows, Root: "/media", Parent: library.InSeries, ParentPath: "/media/show"}}
	if len(res.Subfolders) != 1 || res.Subfolders[0].Path != want.Path || res.Subfolders[0].Scope.Parent != want.Scope.Parent ||
		res.Subfolders[0].Scope.ParentPath != want.Scope.ParentPath || *res.Subfolders[0].Scope.SeasonNumber != 2 {
		t.Errorf("subfolders = %+v, want %+v with season 2", res.Subfolders, want)
	}

	if res, err := p.Resolve(t.Context(), scope, "/media/other", nil); err != nil || res != nil {
		t.Errorf("Resolve(unclaimed) = %+v, %v, want nil", res, err)
	}
}

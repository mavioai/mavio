package providers

import (
	"context"
	"fmt"
	"path"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ResolverPlugin is a resolver plugin as a library.FolderResolver.
type ResolverPlugin struct {
	ID     string
	Client pluginv1connect.ResolverServiceClient
}

// Name returns the plugin's ID.
func (p *ResolverPlugin) Name() string { return p.ID }

// Ignore has the plugin name the entries of a folder to leave out.
func (p *ResolverPlugin) Ignore(ctx context.Context, scope library.Scope, dir string, entries []library.Entry) ([]string, error) {
	resp, err := p.Client.Ignore(ctx, pluginv1.IgnoreRequest_builder{Folder: toFolder(scope, dir, entries)}.Build())
	if err != nil {
		return nil, fmt.Errorf("ignore: %w", err)
	}
	out := make([]string, len(resp.GetNames()))
	for i, name := range resp.GetNames() {
		out[i] = path.Join(dir, name)
	}
	return out, nil
}

// Resolve has the plugin resolve a folder, when it claims it.
func (p *ResolverPlugin) Resolve(ctx context.Context, scope library.Scope, dir string, entries []library.Entry) (*library.Result, error) {
	resp, err := p.Client.Resolve(ctx, pluginv1.ResolveRequest_builder{Folder: toFolder(scope, dir, entries)}.Build())
	if err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
	}
	if !resp.GetClaimed() {
		return nil, nil
	}
	res := &library.Result{}
	if resp.HasFolderItem() {
		n, err := toNode(dir, resp.GetFolderItem())
		if err != nil {
			return nil, err
		}
		n.Path = dir
		res.Item = &n
	}
	for _, it := range resp.GetItems() {
		n, err := toNode(dir, it)
		if err != nil {
			return nil, err
		}
		res.Items = append(res.Items, n)
	}
	for _, s := range resp.GetSubfolders() {
		sub := library.Subfolder{
			Path: path.Join(dir, s.GetName()),
			Scope: library.Scope{
				Kind: scope.Kind, Root: scope.Root, Parent: containers[s.GetParent()], ParentPath: dir,
				SeasonNumber: optInt(s.GetSeasonNumber()),
			},
		}
		res.Subfolders = append(res.Subfolders, sub)
	}
	return res, nil
}

func toFolder(scope library.Scope, dir string, entries []library.Entry) *pluginv1.Folder {
	f := pluginv1.Folder_builder{
		LibraryKind: libraryKinds[scope.Kind].Enum(),
		Root:        proto.String(scope.Root),
		Path:        proto.String(dir),
		Parent:      containersToProto[scope.Parent].Enum(),
	}
	if scope.SeasonNumber != nil {
		f.SeasonNumber = proto.Int32(int32(*scope.SeasonNumber))
	}
	for _, e := range entries {
		f.Entries = append(f.Entries, pluginv1.FolderEntry_builder{Name: proto.String(e.Name()), IsDir: proto.Bool(e.IsDir)}.Build())
	}
	return f.Build()
}

// toNode returns an item a plugin found in dir; the library checks it
// further.
func toNode(dir string, it *pluginv1.ResolvedItem) (library.Node, error) {
	kind, ok := ItemKind(it.GetKind())
	if !ok {
		return library.Node{}, fmt.Errorf("item %q: kind %v", it.GetEntry(), it.GetKind())
	}
	n := library.Node{
		Kind: kind, Path: path.Join(dir, it.GetEntry()), Name: it.GetName(),
		Year: optInt(it.GetYear()), ParentIndex: optInt(it.GetParentIndex()), Index: optInt(it.GetIndex()),
		ExternalIDs: externalIDs(it.GetExternalIds()),
	}
	for _, part := range it.GetParts() {
		n.Parts = append(n.Parts, path.Join(dir, part))
	}
	return n, nil
}

// optInt returns a number, or nil for 0, which plugins send for unknown.
func optInt(v int32) *int {
	if v == 0 {
		return nil
	}
	n := int(v)
	return &n
}

var libraryKinds = map[core.LibraryKind]pluginv1.LibraryKind{
	core.LibraryMovies:      pluginv1.LibraryKind_LIBRARY_KIND_MOVIES,
	core.LibraryShows:       pluginv1.LibraryKind_LIBRARY_KIND_SHOWS,
	core.LibraryMusic:       pluginv1.LibraryKind_LIBRARY_KIND_MUSIC,
	core.LibraryMusicVideos: pluginv1.LibraryKind_LIBRARY_KIND_MUSIC_VIDEOS,
	core.LibraryHomeVideos:  pluginv1.LibraryKind_LIBRARY_KIND_HOME_VIDEOS,
	core.LibraryBooks:       pluginv1.LibraryKind_LIBRARY_KIND_BOOKS,
	core.LibraryPhotos:      pluginv1.LibraryKind_LIBRARY_KIND_PHOTOS,
	core.LibraryMixed:       pluginv1.LibraryKind_LIBRARY_KIND_MIXED,
}

var containers = map[pluginv1.Container]library.Container{
	pluginv1.Container_CONTAINER_FOLDER:       library.InFolder,
	pluginv1.Container_CONTAINER_SERIES:       library.InSeries,
	pluginv1.Container_CONTAINER_SEASON:       library.InSeason,
	pluginv1.Container_CONTAINER_MUSIC_ARTIST: library.InMusicArtist,
}

var containersToProto = reverse(containers)

var _ library.FolderResolver = (*ResolverPlugin)(nil)

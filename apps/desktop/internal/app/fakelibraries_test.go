package app

import (
	"context"
	"slices"

	"connectrpc.com/connect"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1/storagev1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
)

// fakeStorage serves an rclone that runs, with a remote of one folder.
type fakeStorage struct {
	storagev1connect.UnimplementedStorageServiceHandler
	f *fakeCore
}

// fakeLibraryList are the server's libraries: movies of two folders, a
// show of one, and music.
func fakeLibraryList() []*libraryv1.Library {
	lib := func(id, name string, kind libraryv1.LibraryKind, paths ...string) *libraryv1.Library {
		return libraryv1.Library_builder{Id: &id, Name: &name, Kind: &kind, Paths: paths}.Build()
	}
	movies := lib("movies", "Movies", libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, "/media/movies", "/media/4k")
	movies.SetEffectiveStorageClasses(map[string]libraryv1.StorageClass{
		"/media/movies": libraryv1.StorageClass_STORAGE_CLASS_LOCAL, "/media/4k": libraryv1.StorageClass_STORAGE_CLASS_NETWORK,
	})
	return []*libraryv1.Library{
		movies,
		lib("shows", "Shows", libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, "/media/shows"),
		lib("music", "Music", libraryv1.LibraryKind_LIBRARY_KIND_MUSIC, "/media/music"),
	}
}

// libraryOf returns a library of a spec.
func libraryOf(id string, s *libraryv1.LibrarySpec) *libraryv1.Library {
	return libraryv1.Library_builder{
		Id: &id, Name: new(s.GetName()), Kind: new(s.GetKind()), Paths: s.GetPaths(), ScanInterval: s.GetScanInterval(),
		PreferredLanguage: new(s.GetPreferredLanguage()), MetadataCountry: new(s.GetMetadataCountry()),
		SaveLocalMetadata: new(s.GetSaveLocalMetadata()), AutoCollections: new(s.GetAutoCollections()),
		ExtractTrickplay: new(s.GetExtractTrickplay()), ExtractChapterImages: new(s.GetExtractChapterImages()),
		VirtualEpisodes: s.GetVirtualEpisodes(), StorageClasses: s.GetStorageClasses(),
	}.Build()
}

func (l *fakeLibraries) CreateLibrary(_ context.Context, r *libraryv1.CreateLibraryRequest) (*libraryv1.CreateLibraryResponse, error) {
	l.f.mu.Lock()
	defer l.f.mu.Unlock()
	lib := libraryOf("new-lib", r.GetSpec())
	l.f.libraries = append(l.f.libraries, lib)
	return libraryv1.CreateLibraryResponse_builder{Library: lib}.Build(), nil
}

func (l *fakeLibraries) UpdateLibrary(_ context.Context, r *libraryv1.UpdateLibraryRequest) (*libraryv1.UpdateLibraryResponse, error) {
	l.f.mu.Lock()
	defer l.f.mu.Unlock()
	i := slices.IndexFunc(l.f.libraries, func(x *libraryv1.Library) bool { return x.GetId() == r.GetId() })
	if i < 0 {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	l.f.libraries[i] = libraryOf(r.GetId(), r.GetSpec())
	return libraryv1.UpdateLibraryResponse_builder{Library: l.f.libraries[i]}.Build(), nil
}

func (l *fakeLibraries) DeleteLibrary(_ context.Context, r *libraryv1.DeleteLibraryRequest) (*libraryv1.DeleteLibraryResponse, error) {
	l.f.mu.Lock()
	defer l.f.mu.Unlock()
	l.f.libraries = slices.DeleteFunc(l.f.libraries, func(x *libraryv1.Library) bool { return x.GetId() == r.GetId() })
	return &libraryv1.DeleteLibraryResponse{}, nil
}

func (s *fakeSystem) ListDirectory(_ context.Context, r *systemv1.ListDirectoryRequest) (*systemv1.ListDirectoryResponse, error) {
	dir := func(path, name string) *systemv1.DirectoryEntry {
		return systemv1.DirectoryEntry_builder{Name: &name, Path: &path, IsDir: new(true)}.Build()
	}
	switch r.GetPath() {
	case "":
		return systemv1.ListDirectoryResponse_builder{Entries: []*systemv1.DirectoryEntry{dir("/", "/")}}.Build(), nil
	case "/":
		return systemv1.ListDirectoryResponse_builder{Path: new("/"), Entries: []*systemv1.DirectoryEntry{dir("/media", "media")}}.Build(), nil
	case "/media":
		return systemv1.ListDirectoryResponse_builder{Path: new("/media"), Parent: new("/"), Entries: []*systemv1.DirectoryEntry{
			dir("/media/anime", "anime"), dir("/media/movies", "movies"),
		}}.Build(), nil
	}
	return systemv1.ListDirectoryResponse_builder{Path: new(r.GetPath()), Parent: new("/media")}.Build(), nil
}

func (s *fakeStorage) GetStatus(context.Context, *storagev1.GetStatusRequest) (*storagev1.GetStatusResponse, error) {
	return storagev1.GetStatusResponse_builder{Available: new(true)}.Build(), nil
}

func (s *fakeStorage) ListRemotes(context.Context, *storagev1.ListRemotesRequest) (*storagev1.ListRemotesResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	return storagev1.ListRemotesResponse_builder{Remotes: slices.Clone(s.f.remotes)}.Build(), nil
}

func (s *fakeStorage) ListFolders(_ context.Context, r *storagev1.ListFoldersRequest) (*storagev1.ListFoldersResponse, error) {
	if r.GetPath() == "remote:drive:" {
		return storagev1.ListFoldersResponse_builder{Folders: []string{"remote:drive:films"}}.Build(), nil
	}
	return &storagev1.ListFoldersResponse{}, nil
}

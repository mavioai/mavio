package app

import (
	"context"
	"net/http"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
)

// fakeLists serves the user's playlists, which it keeps with what was
// added to them, and adds to and renames collections.
type fakeLists struct {
	libraryv1connect.UnimplementedPlaylistServiceHandler
	libraryv1connect.UnimplementedCollectionServiceHandler
	f *fakeCore
}

// fakeList is a playlist or collection with its items.
type fakeList struct {
	id, name string
	items    []string
}

func (f *fakeCore) handleLists(mux *http.ServeMux) {
	l := &fakeLists{f: f}
	mux.Handle(libraryv1connect.NewPlaylistServiceHandler(l))
	mux.Handle(libraryv1connect.NewCollectionServiceHandler(l))
	f.playlists = []*fakeList{{id: "night-drives", name: "Night Drives"}}
	f.collections = map[string]*fakeList{
		"orbit-collection": {id: "orbit-collection", name: "The Orbit Collection"},
		"space-classics":   {id: "space-classics", name: "Space Classics"},
	}
}

func (l *fakeLists) ListPlaylists(context.Context, *libraryv1.ListPlaylistsRequest) (*libraryv1.ListPlaylistsResponse, error) {
	l.f.mu.Lock()
	defer l.f.mu.Unlock()
	var items []*libraryv1.Item
	for _, p := range l.f.playlists {
		items = append(items, libraryv1.Item_builder{Id: new(p.id), Name: new(p.name), Kind: new(libraryv1.ItemKind_ITEM_KIND_PLAYLIST)}.Build())
	}
	return libraryv1.ListPlaylistsResponse_builder{Playlists: items, Total: new(int32(len(items)))}.Build(), nil
}

func (l *fakeLists) AddToPlaylist(_ context.Context, r *libraryv1.AddToPlaylistRequest) (*libraryv1.AddToPlaylistResponse, error) {
	l.f.mu.Lock()
	defer l.f.mu.Unlock()
	for _, p := range l.f.playlists {
		if p.id == r.GetId() {
			p.items = append(p.items, r.GetItemIds()...)
		}
	}
	return &libraryv1.AddToPlaylistResponse{}, nil
}

func (l *fakeLists) CreatePlaylist(_ context.Context, r *libraryv1.CreatePlaylistRequest) (*libraryv1.CreatePlaylistResponse, error) {
	l.f.mu.Lock()
	defer l.f.mu.Unlock()
	p := &fakeList{id: "new-playlist", name: r.GetName(), items: r.GetItemIds()}
	l.f.playlists = append(l.f.playlists, p)
	return libraryv1.CreatePlaylistResponse_builder{Playlist: libraryv1.Item_builder{Id: new(p.id), Name: new(p.name)}.Build()}.Build(), nil
}

func (l *fakeLists) AddToCollection(_ context.Context, r *libraryv1.AddToCollectionRequest) (*libraryv1.AddToCollectionResponse, error) {
	l.f.mu.Lock()
	defer l.f.mu.Unlock()
	if col := l.f.collections[r.GetId()]; col != nil {
		col.items = append(col.items, r.GetItemIds()...)
	}
	return &libraryv1.AddToCollectionResponse{}, nil
}

func (s *fakeSessions) SendCommand(_ context.Context, r *sessionv1.SendCommandRequest) (*sessionv1.SendCommandResponse, error) {
	s.f.mu.Lock()
	defer s.f.mu.Unlock()
	s.f.commands = append(s.f.commands, r)
	return &sessionv1.SendCommandResponse{}, nil
}

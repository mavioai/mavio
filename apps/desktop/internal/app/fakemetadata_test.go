package app

import (
	"context"
	"net/http"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

// fakeMetadata serves the metadata of the server's items, keeping the
// changes asked of it.
type fakeMetadata struct {
	libraryv1connect.UnimplementedMetadataServiceHandler
	f *fakeCore
}

// metadataCalls are the changes asked of the fake's metadata.
type metadataCalls struct {
	updates    []*libraryv1.UpdateItemRequest
	identified []*libraryv1.IdentifyItemRequest
	refreshed  []*libraryv1.RefreshItemRequest
	images     []*libraryv1.SetItemImageRequest
}

func (f *fakeCore) handleMetadata(mux *http.ServeMux) {
	mux.Handle(libraryv1connect.NewMetadataServiceHandler(&fakeMetadata{f: f}))
}

func (m *fakeMetadata) ListExternalIdKinds(context.Context, *libraryv1.ListExternalIdKindsRequest) (*libraryv1.ListExternalIdKindsResponse, error) {
	return libraryv1.ListExternalIdKindsResponse_builder{Kinds: []*libraryv1.ExternalIdKind{
		libraryv1.ExternalIdKind_builder{Key: new("tmdb"), Name: new("TMDB")}.Build(),
		libraryv1.ExternalIdKind_builder{Key: new("imdb"), Name: new("IMDb")}.Build(),
	}}.Build(), nil
}

func (m *fakeMetadata) UpdateItem(_ context.Context, r *libraryv1.UpdateItemRequest) (*libraryv1.UpdateItemResponse, error) {
	m.f.mu.Lock()
	defer m.f.mu.Unlock()
	m.f.metadata.updates = append(m.f.metadata.updates, r)
	return &libraryv1.UpdateItemResponse{}, nil
}

func (m *fakeMetadata) SearchRemote(_ context.Context, r *libraryv1.SearchRemoteRequest) (*libraryv1.SearchRemoteResponse, error) {
	return libraryv1.SearchRemoteResponse_builder{Results: []*libraryv1.RemoteSearchResult{
		libraryv1.RemoteSearchResult_builder{
			Provider: new("TMDB"), Name: new(r.GetName()), Year: new(int32(2024)), Overview: new("A station at the edge of the system."),
			ExternalIds: map[string]string{"tmdb": "4242"},
		}.Build(),
	}}.Build(), nil
}

func (m *fakeMetadata) IdentifyItem(_ context.Context, r *libraryv1.IdentifyItemRequest) (*libraryv1.IdentifyItemResponse, error) {
	m.f.mu.Lock()
	defer m.f.mu.Unlock()
	m.f.metadata.identified = append(m.f.metadata.identified, r)
	return &libraryv1.IdentifyItemResponse{}, nil
}

func (m *fakeMetadata) RefreshItem(_ context.Context, r *libraryv1.RefreshItemRequest) (*libraryv1.RefreshItemResponse, error) {
	m.f.mu.Lock()
	defer m.f.mu.Unlock()
	m.f.metadata.refreshed = append(m.f.metadata.refreshed, r)
	return &libraryv1.RefreshItemResponse{}, nil
}

func (m *fakeMetadata) ListRemoteImages(_ context.Context, r *libraryv1.ListRemoteImagesRequest) (*libraryv1.ListRemoteImagesResponse, error) {
	return libraryv1.ListRemoteImagesResponse_builder{Images: []*libraryv1.RemoteImage{
		libraryv1.RemoteImage_builder{Provider: new("TMDB"), Kind: new(r.GetKind()), Url: new(m.f.images + "/remote-poster.jpg"), Width: new(int32(2000)), Height: new(int32(3000)), Language: new("en")}.Build(),
	}}.Build(), nil
}

func (m *fakeMetadata) SetItemImage(_ context.Context, r *libraryv1.SetItemImageRequest) (*libraryv1.SetItemImageResponse, error) {
	m.f.mu.Lock()
	defer m.f.mu.Unlock()
	m.f.metadata.images = append(m.f.metadata.images, r)
	return &libraryv1.SetItemImageResponse{}, nil
}

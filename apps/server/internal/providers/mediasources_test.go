package providers

import (
	"context"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

type fakeMediaSources struct {
	got *pluginv1.GetMediaSourcesRequest
}

func (f *fakeMediaSources) GetMediaSources(_ context.Context, req *pluginv1.GetMediaSourcesRequest) (*pluginv1.GetMediaSourcesResponse, error) {
	f.got = req
	return pluginv1.GetMediaSourcesResponse_builder{Sources: []*pluginv1.RemoteMediaSource{pluginv1.RemoteMediaSource_builder{
		Id: new("hd"), Name: new("HD"), Url: new("https://media.example/heat.mkv"),
	}.Build()}}.Build(), nil
}

func TestMediaSourcePlugin(t *testing.T) {
	f := &fakeMediaSources{}
	p := &MediaSourcePlugin{ID: "m", Client: f}
	it := core.Item{ID: core.NewID(), Kind: core.KindMovie, Name: "Heat", ExternalIDs: map[core.Provider]string{core.ProviderIMDb: "tt0113277"}}
	got, err := p.MediaSources(t.Context(), it)
	if err != nil || len(got) != 1 || got[0].ID != "hd" || got[0].Name != "HD" || got[0].URL != "https://media.example/heat.mkv" {
		t.Errorf("MediaSources() = %+v, %v", got, err)
	}
	if f.got.GetItemId() != it.ID.String() || f.got.GetKind() != pluginv1.MediaKind_MEDIA_KIND_MOVIE || f.got.GetName() != "Heat" ||
		f.got.GetExternalIds()[string(core.ProviderIMDb)] != "tt0113277" {
		t.Errorf("request = %v", f.got)
	}
}

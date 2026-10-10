package providers

import (
	"context"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

type fakeGenerator struct {
	got *pluginv1.GenerateImagesRequest
}

func (f *fakeGenerator) GenerateImages(_ context.Context, req *pluginv1.GenerateImagesRequest) (*pluginv1.GenerateImagesResponse, error) {
	f.got = req
	return pluginv1.GenerateImagesResponse_builder{Images: []*pluginv1.GeneratedImage{pluginv1.GeneratedImage_builder{
		Kind: pluginv1.ImageKind_IMAGE_KIND_LOGO.Enum(), Content: []byte("png"),
	}.Build()}}.Build(), nil
}

func TestGeneratorPlugin(t *testing.T) {
	f := &fakeGenerator{}
	p := &GeneratorPlugin{ID: "g", Client: f}
	it := core.Item{ID: core.NewID(), Kind: core.KindMovie, Name: "Heat"}
	got, err := p.GenerateImages(t.Context(), it, []core.ImageKind{core.ImageLogo, core.ImageBanner})
	if err != nil || len(got) != 1 || got[0].Kind != core.ImageLogo || string(got[0].Content) != "png" {
		t.Errorf("GenerateImages() = %+v, %v", got, err)
	}
	want := []pluginv1.ImageKind{pluginv1.ImageKind_IMAGE_KIND_LOGO, pluginv1.ImageKind_IMAGE_KIND_BANNER}
	if f.got.GetItemId() != it.ID.String() || f.got.GetKind() != pluginv1.MediaKind_MEDIA_KIND_MOVIE || f.got.GetName() != "Heat" ||
		!slices.Equal(f.got.GetImageKinds(), want) {
		t.Errorf("request = %v", f.got)
	}
}

package providers

import (
	"context"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

type fakeLocal struct {
	got  *pluginv1.ReadMetadataRequest
	save *pluginv1.SaveMetadataRequest
}

func (f *fakeLocal) ReadMetadata(_ context.Context, req *pluginv1.ReadMetadataRequest) (*pluginv1.ReadMetadataResponse, error) {
	f.got = req
	return pluginv1.ReadMetadataResponse_builder{
		Found: proto.Bool(true), Metadata: pluginv1.Metadata_builder{Name: proto.String(string(req.GetFiles()[0].GetContent()))}.Build(),
	}.Build(), nil
}

func (f *fakeLocal) SaveMetadata(_ context.Context, req *pluginv1.SaveMetadataRequest) (*pluginv1.SaveMetadataResponse, error) {
	f.save = req
	return pluginv1.SaveMetadataResponse_builder{Files: []*pluginv1.LocalFile{
		pluginv1.LocalFile_builder{Name: proto.String("movie.json"), Content: []byte("{}")}.Build(),
	}}.Build(), nil
}

func TestLocalPlugins(t *testing.T) {
	f := &fakeLocal{}
	reader := &LocalPlugin{ID: "yaml", Files: []string{"*.yaml"}, Client: f}
	res, err := reader.ReadLocal(t.Context(), library.Lookup{Kind: core.KindMovie, Name: "Heat"}, "Heat.mkv",
		[]library.LocalFile{{Name: "Heat.yaml", Content: []byte("Heat")}})
	if err != nil || res == nil || res.Item.Name != "Heat" {
		t.Errorf("ReadLocal() = %+v, %v", res, err)
	}
	if f.got.GetMediaName() != "Heat.mkv" || f.got.GetLookup().GetKind() != pluginv1.MediaKind_MEDIA_KIND_MOVIE || f.got.GetFiles()[0].GetName() != "Heat.yaml" {
		t.Errorf("request = %v", f.got)
	}

	saver := &SaverPlugin{ID: "json", Client: f}
	md := pluginv1.Metadata_builder{
		Name: proto.String("Heat"), Overview: proto.String("A crime saga."), ProductionYear: proto.Int32(1995),
		PremiereDate: proto.String("1995-12-15"), Runtime: durationpb.New(170 * time.Minute), CommunityRating: proto.Float64(8.3),
		Genres: []string{"Crime"}, ExternalIds: map[string]string{"tmdb": "949"}, SeriesStatus: pluginv1.SeriesStatus_SERIES_STATUS_ENDED.Enum(),
		People: []*pluginv1.PersonCredit{pluginv1.PersonCredit_builder{
			Name: proto.String("Al Pacino"), Kind: pluginv1.CreditKind_CREDIT_KIND_ACTOR.Enum(), Role: proto.String("Vincent Hanna"), Order: proto.Int32(0),
		}.Build()},
		Images: []*pluginv1.RemoteImage{pluginv1.RemoteImage_builder{
			Kind: pluginv1.ImageKind_IMAGE_KIND_PRIMARY.Enum(), Url: proto.String("https://image.example/heat.jpg"), Width: proto.Int32(1000),
		}.Build()},
	}.Build()
	in := toResult(md)
	in.Item.Kind = core.KindMovie
	files, err := saver.Save(t.Context(), "Heat.mkv", in)
	if err != nil || len(files) != 1 || files[0].Name != "movie.json" {
		t.Errorf("Save() = %+v, %v", files, err)
	}
	// What a provider gives comes back to the saver unchanged.
	if !proto.Equal(md, f.save.GetMetadata()) || f.save.GetKind() != pluginv1.MediaKind_MEDIA_KIND_MOVIE || f.save.GetMediaName() != "Heat.mkv" {
		t.Errorf("save request = %v, want metadata = %v", f.save, md)
	}
}

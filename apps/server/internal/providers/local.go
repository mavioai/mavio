package providers

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// LocalPlugin is a local metadata plugin as a library.LocalReader.
type LocalPlugin struct {
	ID string
	// Files are the manifest's local_metadata_files.
	Files  []string
	Client pluginv1connect.LocalMetadataServiceClient
}

// Name returns the plugin's ID.
func (p *LocalPlugin) Name() string { return p.ID }

// Patterns returns the names of the files the plugin reads.
func (p *LocalPlugin) Patterns() []string { return p.Files }

// ReadLocal has the plugin read an item's local files.
func (p *LocalPlugin) ReadLocal(ctx context.Context, l library.Lookup, media string, files []library.LocalFile) (*metadata.Result, error) {
	kind, ok := mediaKinds[l.Kind]
	if !ok {
		return nil, nil
	}
	resp, err := p.Client.ReadMetadata(ctx, pluginv1.ReadMetadataRequest_builder{
		Lookup: toLookup(kind, l), MediaName: &media, Files: toLocalFiles(files),
	}.Build())
	if err != nil {
		return nil, fmt.Errorf("read metadata: %w", err)
	}
	if !resp.GetFound() {
		return nil, nil
	}
	return toResult(resp.GetMetadata()), nil
}

// SaverPlugin is a metadata saver plugin as a library.Saver.
type SaverPlugin struct {
	ID     string
	Client pluginv1connect.MetadataSaverServiceClient
}

// Name returns the plugin's ID.
func (p *SaverPlugin) Name() string { return p.ID }

// Save has the plugin turn an item's metadata into files.
func (p *SaverPlugin) Save(ctx context.Context, media string, res *metadata.Result) ([]library.LocalFile, error) {
	kind, ok := mediaKinds[res.Item.Kind]
	if !ok {
		return nil, nil
	}
	resp, err := p.Client.SaveMetadata(ctx, pluginv1.SaveMetadataRequest_builder{
		Kind: kind.Enum(), MediaName: &media, Metadata: toMetadata(res),
	}.Build())
	if err != nil {
		return nil, fmt.Errorf("save metadata: %w", err)
	}
	out := make([]library.LocalFile, len(resp.GetFiles()))
	for i, f := range resp.GetFiles() {
		out[i] = library.LocalFile{Name: f.GetName(), Content: f.GetContent()}
	}
	return out, nil
}

func toLocalFiles(files []library.LocalFile) []*pluginv1.LocalFile {
	out := make([]*pluginv1.LocalFile, len(files))
	for i, f := range files {
		out[i] = pluginv1.LocalFile_builder{Name: proto.String(f.Name), Content: f.Content}.Build()
	}
	return out
}

// toMetadata is the reverse of toResult.
func toMetadata(res *metadata.Result) *pluginv1.Metadata {
	it := &res.Item
	md := pluginv1.Metadata_builder{
		Name:            set(it.Name),
		OriginalTitle:   set(it.OriginalTitle),
		SortName:        set(it.SortName),
		Overview:        set(it.Overview),
		Tagline:         set(it.Tagline),
		ProductionYear:  set(int32(it.ProductionYear)),
		PremiereDate:    set(isoDate(it.PremiereDate)),
		EndDate:         set(isoDate(it.EndDate)),
		OfficialRating:  set(it.OfficialRating),
		CommunityRating: set(it.CommunityRating),
		CriticRating:    set(it.CriticRating),
		Genres:          it.Genres,
		Tags:            it.Tags,
		Studios:         it.Studios,
		ExternalIds:     ids(it.ExternalIDs),
		Artists:         it.Artists,
		AlbumArtists:    it.AlbumArtists,
		CollectionName:  set(it.CollectionName),
	}
	if it.Runtime > 0 {
		md.Runtime = durationpb.New(it.Runtime)
	}
	if s, ok := seriesStatusesToProto[it.SeriesStatus]; ok {
		md.SeriesStatus = s.Enum()
	}
	for _, p := range res.People {
		pc := pluginv1.PersonCredit_builder{Name: set(p.Name), Role: set(p.Role), ImageUrl: set(p.ImageURL)}
		if k, ok := creditKindsToProto[p.Kind]; ok {
			pc.Kind = k.Enum()
		}
		if p.Order != nil {
			pc.Order = proto.Int32(int32(*p.Order))
		}
		md.People = append(md.People, pc.Build())
	}
	for _, img := range res.RemoteImages {
		if k, ok := imageKindsToProto[img.Kind]; ok {
			md.Images = append(md.Images, pluginv1.RemoteImage_builder{
				Kind: k.Enum(), Url: set(img.URL), Width: set(int32(img.Width)), Height: set(int32(img.Height)),
				Language: set(img.Language), Score: set(img.Score),
			}.Build())
		}
	}
	return md.Build()
}

// set returns a field's value, or nil for the zero value, which plugins
// read as unset.
func set[T comparable](v T) *T {
	var zero T
	if v == zero {
		return nil
	}
	return &v
}

// isoDate formats a date as toResult parses it; empty when nil.
func isoDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.DateOnly)
}

// reverse inverts a one-to-one map.
func reverse[K, V comparable](m map[K]V) map[V]K {
	out := make(map[V]K, len(m))
	for k, v := range m {
		out[v] = k
	}
	return out
}

var (
	seriesStatusesToProto = reverse(seriesStatuses)
	creditKindsToProto    = reverse(creditKinds)
	imageKindsToProto     = reverse(imageKinds)
)

var (
	_ library.LocalReader = (*LocalPlugin)(nil)
	_ library.Saver       = (*SaverPlugin)(nil)
)

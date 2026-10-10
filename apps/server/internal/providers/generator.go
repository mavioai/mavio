package providers

import (
	"context"
	"fmt"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// GeneratorPlugin is an image generator plugin as a library.ImageGenerator.
type GeneratorPlugin struct {
	ID     string
	Client pluginv1connect.ImageGeneratorServiceClient
}

// Name returns the plugin's ID.
func (p *GeneratorPlugin) Name() string { return p.ID }

// GenerateImages has the plugin make images of an item.
func (p *GeneratorPlugin) GenerateImages(ctx context.Context, it core.Item, kinds []core.ImageKind) ([]library.GeneratedImage, error) {
	req := pluginv1.GenerateImagesRequest_builder{ItemId: proto.String(it.ID.String()), Name: set(it.Name)}
	if kind, ok := mediaKinds[it.Kind]; ok {
		req.Kind = kind.Enum()
	}
	for _, k := range kinds {
		if pk, ok := imageKindsToProto[k]; ok {
			req.ImageKinds = append(req.ImageKinds, pk)
		}
	}
	if len(req.ImageKinds) == 0 {
		return nil, nil
	}
	resp, err := p.Client.GenerateImages(ctx, req.Build())
	if err != nil {
		return nil, fmt.Errorf("generate images: %w", err)
	}
	var out []library.GeneratedImage
	for _, img := range resp.GetImages() {
		kind, ok := imageKinds[img.GetKind()]
		if !ok {
			return nil, fmt.Errorf("generate images: image kind %v", img.GetKind())
		}
		out = append(out, library.GeneratedImage{Kind: kind, Content: img.GetContent()})
	}
	return out, nil
}

var _ library.ImageGenerator = (*GeneratorPlugin)(nil)

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

// IntroPlugin is an intro provider plugin as a library.IntroProvider.
type IntroPlugin struct {
	ID     string
	Client pluginv1connect.IntroProviderServiceClient
}

// Name returns the plugin's ID.
func (p *IntroPlugin) Name() string { return p.ID }

// Intros asks the plugin for the items to play before an item.
func (p *IntroPlugin) Intros(ctx context.Context, it core.Item, userID core.ID) ([]core.ID, error) {
	req := pluginv1.GetIntrosRequest_builder{
		ItemId: proto.String(it.ID.String()), Name: set(it.Name), UserId: proto.String(userID.String()),
	}
	if kind, ok := mediaKinds[it.Kind]; ok {
		req.Kind = kind.Enum()
	}
	resp, err := p.Client.GetIntros(ctx, req.Build())
	if err != nil {
		return nil, fmt.Errorf("get intros: %w", err)
	}
	out := make([]core.ID, 0, len(resp.GetItemIds()))
	for _, s := range resp.GetItemIds() {
		id, err := core.ParseID(s)
		if err != nil {
			return nil, fmt.Errorf("get intros: item ID %q: %w", s, err)
		}
		out = append(out, id)
	}
	return out, nil
}

var _ library.IntroProvider = (*IntroPlugin)(nil)

package rpc

import (
	"context"

	"github.com/mavioai/mavio/apps/server/internal/home"
	"github.com/mavioai/mavio/libs/core"
	homev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/home/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/home/v1/homev1connect"
)

// HomeService implements mavio.home.v1.HomeService.
type HomeService struct {
	store core.Store
	home  *home.Service
}

var _ homev1connect.HomeServiceHandler = (*HomeService)(nil)

// NewHomeService returns a HomeService building hubs with h.
func NewHomeService(store core.Store, h *home.Service) *HomeService {
	return &HomeService{store: store, home: h}
}

// ListHubs lists the hubs of Home, or of a library's Recommended tab.
func (s *HomeService) ListHubs(ctx context.Context, req *homev1.ListHubsRequest) (*homev1.ListHubsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	var hubs []home.Hub
	if req.HasLibraryId() {
		hubs, err = s.home.Recommended(ctx, &p.User, core.MustParseID(req.GetLibraryId()), int(req.GetItemLimit()))
	} else {
		hubs, err = s.home.Home(ctx, &p.User, int(req.GetItemLimit()))
	}
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*homev1.Hub, len(hubs))
	for i := range hubs {
		if out[i], err = s.hubToProto(ctx, &hubs[i], p.User.Admin); err != nil {
			return nil, connectError(ctx, err)
		}
	}
	return homev1.ListHubsResponse_builder{Hubs: out}.Build(), nil
}

// GetHub lists a page of a hub's items.
func (s *HomeService) GetHub(ctx context.Context, req *homev1.GetHubRequest) (*homev1.GetHubResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	h, err := s.home.Get(ctx, &p.User, req.GetId(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := s.hubToProto(ctx, &h, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return homev1.GetHubResponse_builder{Hub: out}.Build(), nil
}

func (s *HomeService) hubToProto(ctx context.Context, h *home.Hub, admin bool) (*homev1.Hub, error) {
	items, err := itemsToProto(ctx, s.store, h.Items, admin)
	if err != nil {
		return nil, err
	}
	b := homev1.Hub_builder{Id: &h.ID, Kind: new(hubKinds[h.Kind]), Items: items, More: &h.More}
	if h.Library != nil {
		b.LibraryId, b.LibraryName = new(h.Library.ID.String()), &h.Library.Name
	}
	b.SeedId = optionalID(h.SeedID)
	if h.SeedName != "" {
		b.SeedName = &h.SeedName
	}
	for i := range h.Titles {
		b.Titles = append(b.Titles, discoveredTitleToProto(&h.Titles[i]))
	}
	return b.Build(), nil
}

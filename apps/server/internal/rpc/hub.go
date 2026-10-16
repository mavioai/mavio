package rpc

import (
	"context"

	"github.com/mavioai/mavio/apps/server/internal/hub"
	"github.com/mavioai/mavio/libs/core"
	hubv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/hub/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/hub/v1/hubv1connect"
)

// HubService implements mavio.hub.v1.HubService.
type HubService struct {
	store core.Store
	hubs  *hub.Service
}

var _ hubv1connect.HubServiceHandler = (*HubService)(nil)

// NewHubService returns a HubService building hubs with h.
func NewHubService(store core.Store, h *hub.Service) *HubService {
	return &HubService{store: store, hubs: h}
}

// ListHubs lists the hubs of Home, or of a library's Recommended tab.
func (s *HubService) ListHubs(ctx context.Context, req *hubv1.ListHubsRequest) (*hubv1.ListHubsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	var hubs []hub.Hub
	if req.HasLibraryId() {
		hubs, err = s.hubs.Recommended(ctx, &p.User, core.MustParseID(req.GetLibraryId()), int(req.GetItemLimit()))
	} else {
		hubs, err = s.hubs.Home(ctx, &p.User, int(req.GetItemLimit()))
	}
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*hubv1.Hub, len(hubs))
	for i := range hubs {
		if out[i], err = s.hubToProto(ctx, &hubs[i], p.User.IsAdmin(), p.User.ID); err != nil {
			return nil, connectError(ctx, err)
		}
	}
	return hubv1.ListHubsResponse_builder{Hubs: out}.Build(), nil
}

// GetHub lists a page of a hub's items.
func (s *HubService) GetHub(ctx context.Context, req *hubv1.GetHubRequest) (*hubv1.GetHubResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	h, err := s.hubs.Get(ctx, &p.User, req.GetId(), int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := s.hubToProto(ctx, &h, p.User.IsAdmin(), p.User.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return hubv1.GetHubResponse_builder{Hub: out}.Build(), nil
}

func (s *HubService) hubToProto(ctx context.Context, h *hub.Hub, admin bool, viewer core.ID) (*hubv1.Hub, error) {
	items, err := itemsToProto(ctx, s.store, h.Items, admin, viewer)
	if err != nil {
		return nil, err
	}
	b := hubv1.Hub_builder{Id: &h.ID, Kind: new(hubKinds[h.Kind]), Items: items, More: &h.More}
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

// GetHomeLayout returns the caller's choice of what Home shows.
func (s *HubService) GetHomeLayout(ctx context.Context, _ *hubv1.GetHomeLayoutRequest) (*hubv1.GetHomeLayoutResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	return hubv1.GetHomeLayoutResponse_builder{Layout: homeLayoutToProto(&p.User.Home)}.Build(), nil
}

// UpdateHomeLayout replaces the caller's choice of what Home shows.
func (s *HubService) UpdateHomeLayout(ctx context.Context, req *hubv1.UpdateHomeLayoutRequest) (*hubv1.UpdateHomeLayoutResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	l := req.GetLayout()
	layout := core.HomeLayout{Pinned: l.GetPinned(), HiddenHubs: l.GetHiddenHubIds()}
	for _, id := range l.GetLibraryIds() {
		layout.Libraries = append(layout.Libraries, core.MustParseID(id))
	}
	if layout, err = s.hubs.SetLayout(ctx, p.User.ID, layout); err != nil {
		return nil, connectError(ctx, err)
	}
	return hubv1.UpdateHomeLayoutResponse_builder{Layout: homeLayoutToProto(&layout)}.Build(), nil
}

func homeLayoutToProto(l *core.HomeLayout) *hubv1.HomeLayout {
	b := hubv1.HomeLayout_builder{Pinned: &l.Pinned, HiddenHubIds: l.HiddenHubs}
	for _, id := range l.Libraries {
		b.LibraryIds = append(b.LibraryIds, id.String())
	}
	return b.Build()
}

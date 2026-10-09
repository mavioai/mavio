package rpc

import (
	"context"
	"errors"
	"fmt"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/apps/server/internal/syncplay"
	"github.com/mavioai/mavio/libs/core"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
)

// SyncPlayService implements mavio.session.v1.SyncPlayService.
type SyncPlayService struct {
	store  core.Store
	hub    *events.Hub
	groups *syncplay.Manager
}

var _ sessionv1connect.SyncPlayServiceHandler = (*SyncPlayService)(nil)

// NewSyncPlayService returns a SyncPlayService whose groups reach their
// members through hub; devices going offline leave their group.
func NewSyncPlayService(store core.Store, hub *events.Hub) *SyncPlayService {
	groups := syncplay.New(func(session core.ID, u *sessionv1.SyncPlayUpdate) {
		hub.ToSession(session, sessionv1.Event_builder{SyncPlay: u}.Build())
	})
	hub.OnOffline(groups.Leave)
	return &SyncPlayService{store: store, hub: hub, groups: groups}
}

// GetTime returns the server's time, for clients to sync their clocks.
func (s *SyncPlayService) GetTime(context.Context, *sessionv1.GetTimeRequest) (*sessionv1.GetTimeResponse, error) {
	received := timestamppb.Now()
	return sessionv1.GetTimeResponse_builder{ReceiveTime: received, SendTime: timestamppb.Now()}.Build(), nil
}

// ListGroups lists the groups whose queue the caller may play.
func (s *SyncPlayService) ListGroups(ctx context.Context, _ *sessionv1.ListGroupsRequest) (*sessionv1.ListGroupsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	var out []*sessionv1.SyncPlayGroup
	for _, g := range s.groups.Groups() {
		ok, err := s.mayPlayGroup(ctx, &p.User, core.MustParseID(g.GetId()))
		if err != nil {
			return nil, connectError(ctx, err)
		}
		if ok {
			out = append(out, g)
		}
	}
	return sessionv1.ListGroupsResponse_builder{Groups: out}.Build(), nil
}

// CreateGroup creates a group with the calling device in it.
func (s *SyncPlayService) CreateGroup(ctx context.Context, req *sessionv1.CreateGroupRequest) (*sessionv1.CreateGroupResponse, error) {
	mb, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	return sessionv1.CreateGroupResponse_builder{Group: s.groups.Create(mb, req.GetName())}.Build(), nil
}

// JoinGroup adds the calling device to a group whose queue it may play.
func (s *SyncPlayService) JoinGroup(ctx context.Context, req *sessionv1.JoinGroupRequest) (*sessionv1.JoinGroupResponse, error) {
	mb, err := s.member(ctx)
	if err != nil {
		return nil, err
	}
	id := core.MustParseID(req.GetGroupId())
	ok, err := s.mayPlayGroup(ctx, &mb.User, id)
	if err != nil || !ok {
		return nil, connectError(ctx, cmpOr(err, fmt.Errorf("syncplay group %s: %w", id, core.ErrNotFound)))
	}
	g, err := s.groups.Join(mb, id)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return sessionv1.JoinGroupResponse_builder{Group: g}.Build(), nil
}

// LeaveGroup removes the calling device from its group.
func (s *SyncPlayService) LeaveGroup(ctx context.Context, _ *sessionv1.LeaveGroupRequest) (*sessionv1.LeaveGroupResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	s.groups.Leave(p.Session.ID)
	return &sessionv1.LeaveGroupResponse{}, nil
}

// SetQueue plays items every member may play.
func (s *SyncPlayService) SetQueue(ctx context.Context, req *sessionv1.SetQueueRequest) (*sessionv1.SetQueueResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]core.ID, len(req.GetItemIds()))
	for i, id := range req.GetItemIds() {
		ids[i] = core.MustParseID(id)
	}
	for _, mb := range s.groups.Members(p.Session.ID) {
		if err := s.mayPlay(ctx, &mb.User, ids); err != nil {
			if mb.User.ID == p.User.ID {
				return nil, connectError(ctx, err)
			}
			return nil, connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("%s may not play every item", mb.User.Name))
		}
	}
	start := time.Duration(0)
	if req.HasStartPosition() {
		start = req.GetStartPosition().AsDuration()
	}
	return &sessionv1.SetQueueResponse{}, s.handle(ctx, syncplay.SetQueue(ids, int(req.GetStartIndex()), start))
}

// Pause pauses the caller's group.
func (s *SyncPlayService) Pause(ctx context.Context, _ *sessionv1.PauseRequest) (*sessionv1.PauseResponse, error) {
	return &sessionv1.PauseResponse{}, s.handle(ctx, syncplay.Pause())
}

// Unpause plays the caller's group.
func (s *SyncPlayService) Unpause(ctx context.Context, _ *sessionv1.UnpauseRequest) (*sessionv1.UnpauseResponse, error) {
	return &sessionv1.UnpauseResponse{}, s.handle(ctx, syncplay.Unpause())
}

// Seek moves the caller's group.
func (s *SyncPlayService) Seek(ctx context.Context, req *sessionv1.SeekRequest) (*sessionv1.SeekResponse, error) {
	return &sessionv1.SeekResponse{}, s.handle(ctx, syncplay.Seek(req.GetPosition().AsDuration()))
}

// Stop stops the caller's group.
func (s *SyncPlayService) Stop(ctx context.Context, _ *sessionv1.StopRequest) (*sessionv1.StopResponse, error) {
	return &sessionv1.StopResponse{}, s.handle(ctx, syncplay.Stop())
}

// NextItem plays the next entry of the caller's group.
func (s *SyncPlayService) NextItem(ctx context.Context, _ *sessionv1.NextItemRequest) (*sessionv1.NextItemResponse, error) {
	return &sessionv1.NextItemResponse{}, s.handle(ctx, syncplay.Step(1))
}

// PreviousItem plays the previous entry of the caller's group.
func (s *SyncPlayService) PreviousItem(ctx context.Context, _ *sessionv1.PreviousItemRequest) (*sessionv1.PreviousItemResponse, error) {
	return &sessionv1.PreviousItemResponse{}, s.handle(ctx, syncplay.Step(-1))
}

// ReportState records whether the calling device is ready or buffering.
func (s *SyncPlayService) ReportState(ctx context.Context, req *sessionv1.ReportStateRequest) (*sessionv1.ReportStateResponse, error) {
	r := syncplay.ReportState(core.MustParseID(req.GetEntryId()), req.GetPosition().AsDuration(), req.GetWhen().AsTime(),
		req.GetPlaying(), req.GetReady())
	return &sessionv1.ReportStateResponse{}, s.handle(ctx, r)
}

// ReportPing records the calling device's round trip.
func (s *SyncPlayService) ReportPing(ctx context.Context, req *sessionv1.ReportPingRequest) (*sessionv1.ReportPingResponse, error) {
	return &sessionv1.ReportPingResponse{}, s.handle(ctx, syncplay.ReportPing(req.GetPing().AsDuration()))
}

// handle applies a request to the caller's group.
func (s *SyncPlayService) handle(ctx context.Context, r syncplay.Request) error {
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	err = s.groups.Handle(p.Session.ID, r)
	switch {
	case errors.Is(err, syncplay.ErrNotInGroup), errors.Is(err, syncplay.ErrNoQueue):
		return connect.NewError(connect.CodeFailedPrecondition, err)
	case err != nil:
		return connectError(ctx, err)
	}
	return nil
}

// member describes the calling device, which must be online to learn of
// its group.
func (s *SyncPlayService) member(ctx context.Context) (syncplay.Member, error) {
	p, err := principal(ctx)
	if err != nil {
		return syncplay.Member{}, err
	}
	if !s.hub.Online(p.Session.ID) {
		return syncplay.Member{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("subscribe to events before joining a group"))
	}
	return memberOf(p), nil
}

func memberOf(p auth.Principal) syncplay.Member {
	return syncplay.Member{Session: p.Session.ID, User: p.User, DeviceName: p.Session.DeviceName}
}

// mayPlayGroup reports whether the user may play every item of a group's
// queue.
func (s *SyncPlayService) mayPlayGroup(ctx context.Context, u *core.User, groupID core.ID) (bool, error) {
	ids, err := s.groups.Queue(groupID)
	if err != nil {
		return false, err
	}
	err = s.mayPlay(ctx, u, ids)
	if errors.Is(err, core.ErrNotFound) {
		return false, nil
	}
	return err == nil, err
}

// mayPlay checks that the user may access every item, ErrNotFound if not.
func (s *SyncPlayService) mayPlay(ctx context.Context, u *core.User, ids []core.ID) error {
	for _, id := range ids {
		it, err := s.store.Items().Get(ctx, id)
		if err != nil {
			return err
		}
		if !u.CanAccess(&it) {
			return fmt.Errorf("item %s: %w", id, core.ErrNotFound)
		}
	}
	return nil
}

func cmpOr(err, def error) error {
	if err != nil {
		return err
	}
	return def
}

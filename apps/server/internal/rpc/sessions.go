package rpc

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/apps/server/internal/playback"
	"github.com/mavioai/mavio/libs/core"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
)

// heartbeatInterval keeps event streams alive through proxies.
const heartbeatInterval = 30 * time.Second

// EventService implements mavio.session.v1.EventService.
type EventService struct {
	hub *events.Hub
}

var _ sessionv1connect.EventServiceHandler = (*EventService)(nil)

// NewEventService returns an EventService streaming hub's events.
func NewEventService(hub *events.Hub) *EventService { return &EventService{hub: hub} }

// Subscribe streams the calling device's events until it disconnects or
// a newer stream replaces this one.
func (s *EventService) Subscribe(ctx context.Context, _ *sessionv1.SubscribeRequest, stream *connect.ServerStream[sessionv1.SubscribeResponse]) error {
	p, err := principal(ctx)
	if err != nil {
		return err
	}
	sub := s.hub.Subscribe(p.User, p.Session.ID)
	defer s.hub.Unsubscribe(sub)
	send := func(e *sessionv1.Event) error {
		if !e.HasTime() {
			e.SetTime(timestamppb.Now())
		}
		return stream.Send(sessionv1.SubscribeResponse_builder{Event: e}.Build())
	}
	if err := send(sessionv1.Event_builder{Connected: sessionv1.Connected_builder{SessionId: new(p.Session.ID.String())}.Build()}.Build()); err != nil {
		return err
	}
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-sub.Done():
			return nil
		case e := <-sub.Events():
			if err := send(e); err != nil {
				return err
			}
		case <-heartbeat.C:
			if err := send(sessionv1.Event_builder{Heartbeat: &sessionv1.Heartbeat{}}.Build()); err != nil {
				return err
			}
		}
	}
}

// SessionService implements mavio.session.v1.SessionService.
type SessionService struct {
	store     core.Store
	hub       *events.Hub
	playbacks *playback.Manager
}

var _ sessionv1connect.SessionServiceHandler = (*SessionService)(nil)

// NewSessionService returns a SessionService.
func NewSessionService(store core.Store, hub *events.Hub, playbacks *playback.Manager) *SessionService {
	return &SessionService{store: store, hub: hub, playbacks: playbacks}
}

// ListSessions lists signed-in devices with what they play.
func (s *SessionService) ListSessions(ctx context.Context, req *sessionv1.ListSessionsRequest) (*sessionv1.ListSessionsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	users := []core.User{p.User}
	if req.GetAllUsers() {
		if !p.User.Admin {
			return nil, connect.NewError(connect.CodePermissionDenied, errors.New("administrator required"))
		}
		if users, err = s.store.Users().List(ctx); err != nil {
			return nil, connectError(ctx, err)
		}
	}
	var out []*sessionv1.Session
	for _, u := range users {
		list, err := s.store.AuthSessions().ListForUser(ctx, u.ID)
		if err != nil {
			return nil, connectError(ctx, err)
		}
		for _, as := range list {
			sess, err := s.sessionToProto(ctx, &u, &as, p.User.Admin)
			if err != nil {
				return nil, connectError(ctx, err)
			}
			out = append(out, sess)
		}
	}
	// Online devices first, then the most recently seen.
	slices.SortStableFunc(out, func(a, b *sessionv1.Session) int {
		if a.GetOnline() != b.GetOnline() {
			if a.GetOnline() {
				return -1
			}
			return 1
		}
		return cmp.Compare(b.GetLastSeenTime().AsTime().UnixNano(), a.GetLastSeenTime().AsTime().UnixNano())
	})
	return sessionv1.ListSessionsResponse_builder{Sessions: out}.Build(), nil
}

func (s *SessionService) sessionToProto(ctx context.Context, u *core.User, as *core.AuthSession, admin bool) (*sessionv1.Session, error) {
	b := sessionv1.Session_builder{
		Id: new(as.ID.String()), UserId: new(u.ID.String()), UserName: &u.Name,
		DeviceId: &as.DeviceID, DeviceName: &as.DeviceName, Client: &as.Client, ClientVersion: &as.ClientVersion,
		LastSeenTime: timestamp(as.LastSeenAt), Online: new(s.hub.Online(as.ID)),
	}
	if pb := s.playbacks.NowPlaying(as.ID); pb != nil {
		items, err := itemsToProto(ctx, s.store, []core.Item{pb.Item}, admin)
		if err != nil {
			return nil, err
		}
		st := pb.State()
		b.NowPlaying = sessionv1.NowPlaying_builder{
			PlaybackId: &pb.ID, Item: items[0], Position: durationpb.New(st.Position), Paused: &st.Paused,
			Method: new(playMethods[pb.Method]),
		}.Build()
	}
	return b.Build(), nil
}

// SendCommand sends a command to an online device.
func (s *SessionService) SendCommand(ctx context.Context, req *sessionv1.SendCommandRequest) (*sessionv1.SendCommandResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	target := core.MustParseID(req.GetSessionId())
	u, online := s.hub.SessionUser(target)
	switch {
	case online && u.ID != p.User.ID && !p.User.Admin:
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
	case !online:
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the device is offline"))
	}
	e := sessionv1.Event_builder{Command: sessionv1.CommandReceived_builder{
		FromSessionId: new(p.Session.ID.String()), FromUserName: &p.User.Name, Command: req.GetCommand(),
	}.Build()}.Build()
	if !s.hub.ToSession(target, e) {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("the device is offline"))
	}
	return &sessionv1.SendCommandResponse{}, nil
}

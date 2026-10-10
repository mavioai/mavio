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
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
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
	// Devices lists plugins' remote devices and sends them commands; nil
	// means none.
	Devices DeviceController
}

// DeviceController keeps the remote devices of plugins; plugins.Manager is
// one.
type DeviceController interface {
	Devices() []plugins.Device
	IsDevice(session core.ID) bool
	SendCommand(ctx context.Context, session core.ID, from core.User, c *pluginv1.Command) error
	SetDevices(ctx context.Context, plugin string, list []*pluginv1.Device) error
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
	if s.Devices != nil {
		for _, d := range s.Devices.Devices() {
			sess, err := s.deviceToProto(ctx, &d, p.User.Admin)
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
	var err error
	b.NowPlaying, err = s.nowPlaying(ctx, as.ID, admin)
	return b.Build(), err
}

func (s *SessionService) deviceToProto(ctx context.Context, d *plugins.Device, admin bool) (*sessionv1.Session, error) {
	b := sessionv1.Session_builder{
		Id: new(d.Session.String()), DeviceId: &d.ID, DeviceName: &d.Name, Client: &d.Product,
		LastSeenTime: timestamp(d.Listed), Online: new(true), PluginId: &d.Plugin,
	}
	var err error
	b.NowPlaying, err = s.nowPlaying(ctx, d.Session, admin)
	return b.Build(), err
}

// nowPlaying describes what a session plays, or returns nil.
func (s *SessionService) nowPlaying(ctx context.Context, session core.ID, admin bool) (*sessionv1.NowPlaying, error) {
	pb := s.playbacks.NowPlaying(session)
	if pb == nil {
		return nil, nil
	}
	items, err := itemsToProto(ctx, s.store, []core.Item{pb.Item}, admin)
	if err != nil {
		return nil, err
	}
	st := pb.State()
	return sessionv1.NowPlaying_builder{
		PlaybackId: &pb.ID, Item: items[0], Position: durationpb.New(st.Position), Paused: &st.Paused,
		Method: new(playMethods[pb.Method]),
	}.Build(), nil
}

// SendCommand sends a command to an online device.
func (s *SessionService) SendCommand(ctx context.Context, req *sessionv1.SendCommandRequest) (*sessionv1.SendCommandResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	target := core.MustParseID(req.GetSessionId())
	if s.Devices != nil && s.Devices.IsDevice(target) {
		if err := s.Devices.SendCommand(ctx, target, p.User, commandToPlugin(req.GetCommand())); err != nil {
			if code := connect.CodeOf(err); code != connect.CodeUnknown {
				return nil, connect.NewError(code, err)
			}
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		return &sessionv1.SendCommandResponse{}, nil
	}
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

// commandToPlugin describes a command to the plugin controlling a device.
func commandToPlugin(c *sessionv1.Command) *pluginv1.Command {
	b := pluginv1.Command_builder{}
	switch {
	case c.HasPlay():
		p := c.GetPlay()
		b.Play = pluginv1.Play_builder{
			ItemIds: p.GetItemIds(), StartIndex: new(p.GetStartIndex()), StartPosition: p.GetStartPosition(),
		}.Build()
	case c.HasPlayState():
		b.PlayState = pluginv1.PlayState_builder{
			Command: new(pluginv1.PlayStateCommand(c.GetPlayState().GetCommand())),
		}.Build()
	case c.HasSeek():
		b.Seek = pluginv1.Seek_builder{Position: c.GetSeek().GetPosition()}.Build()
	case c.HasMessage():
		m := c.GetMessage()
		b.Message = pluginv1.ShowMessage_builder{Text: new(m.GetText()), Timeout: m.GetTimeout()}.Build()
	case c.HasVolume():
		v := c.GetVolume()
		vb := pluginv1.Volume_builder{}
		if v.HasLevel() {
			vb.Level = new(v.GetLevel())
		}
		if v.HasMuted() {
			vb.Muted = new(v.GetMuted())
		}
		b.Volume = vb.Build()
	}
	return b.Build()
}

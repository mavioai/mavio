package assistant

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/websocket"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// VoicePath is the route of voice sessions' WebSockets (§8.1).
const VoicePath = "/assistant/voice/{token}"

const (
	// voiceConnectTTL is how long a session waits for its client.
	voiceConnectTTL = time.Minute
	// voiceMaxTime is the longest a session lasts.
	voiceMaxTime = time.Hour
	// voiceCloseTimeout bounds closing a session in the plugin.
	voiceCloseTimeout = 10 * time.Second
	// The turn settings of sessions (§8.3, §8.4).
	voiceCommitDelay  = 700 * time.Millisecond
	voiceLongestPause = 3 * time.Second
	voiceIdle         = 8 * time.Second
)

// VoicePlugin is the plugin running voice sessions.
type VoicePlugin struct {
	ID     string
	Client pluginv1connect.VoiceSessionServiceClient
	// Dial connects to the plugin's socket, and Authorize adds the
	// plugin's credentials to the headers of a request on it.
	Dial      func(ctx context.Context) (net.Conn, error)
	Authorize func(h http.Header)
}

// voiceSession is a voice session the server opened in the plugin.
type voiceSession struct {
	id     string
	plugin VoicePlugin
	// device is the signed-in session of the user's device.
	device core.ID
	// url is the token of the session's URL, token the session token the
	// plugin asks the assistant with.
	url, token string
	// path is the session's WebSocket on the plugin's socket.
	path string
	// connected is set when the client connects; stop ends the relay.
	connected bool
	stop      context.CancelFunc
	once      sync.Once
}

// voices are the voice sessions open, by the tokens of their URLs and
// their session tokens, guarded by Service.mu.
type voices struct {
	byURL   map[string]*voiceSession
	byToken map[string]*voiceSession
}

func newToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

// StartVoiceSession opens a voice session in the VOICE_SESSION plugin and
// returns the URL of its WebSocket on the server, ending the device's
// session open (§8.1).
func (s *Service) StartVoiceSession(ctx context.Context, req *assistantv1.StartVoiceSessionRequest) (*assistantv1.StartVoiceSessionResponse, error) {
	p, set, err := s.caller(ctx)
	if err != nil {
		return nil, err
	}
	if p.Session.ID.IsZero() {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("a voice session needs a signed-in device"))
	}
	var vp VoicePlugin
	ok := s.cfg.VoiceSessions != nil
	if ok {
		vp, ok = s.cfg.VoiceSessions()
	}
	if !ok {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no voice session plugin is ready"))
	}
	s.mu.Lock()
	var old []*voiceSession
	for _, v := range s.voice.byURL {
		if v.device == p.Session.ID {
			old = append(old, v)
		}
	}
	s.mu.Unlock()
	for _, v := range old {
		s.endVoice(v)
	}

	v := &voiceSession{id: newToken()[:32], plugin: vp, device: p.Session.ID, url: newToken(), token: newToken()}
	lang := req.GetLanguage()
	open := pluginv1.OpenVoiceSessionRequest_builder{
		SessionId: &v.id, Language: &lang, Mode: req.GetMode().Enum(), SessionToken: &v.token,
		Recognizer: new(own(set.Recognizer, vp.ID)), Voice: new(own(voiceOf(set.Voices, lang), vp.ID)),
		Turns: pluginv1.VoiceTurnSettings_builder{
			CommitDelay: durationpb.New(voiceCommitDelay), LongestPause: durationpb.New(voiceLongestPause), Idle: durationpb.New(voiceIdle),
		}.Build(),
	}.Build()
	if set.Hints {
		open.SetHints(s.hints(ctx, p))
	}
	if req.HasContext() {
		b, err := protojson.Marshal(req.GetContext())
		if err != nil {
			return nil, fmt.Errorf("context: %w", err)
		}
		open.SetContextJson(string(b))
	}
	// The session is known before the plugin is asked, so that a request
	// it makes at once is answered.
	s.mu.Lock()
	s.voice.byURL[v.url] = v
	s.voice.byToken[v.token] = v
	s.mu.Unlock()
	resp, err := vp.Client.OpenVoiceSession(ctx, open)
	if err != nil {
		s.forgetVoice(v)
		return nil, fmt.Errorf("open voice session: %w", err)
	}
	v.path = resp.GetPath()
	if !strings.HasPrefix(v.path, "/_mavio/voice/") {
		s.endVoice(v)
		return nil, fmt.Errorf("open voice session: path %q is not under /_mavio/voice/", v.path)
	}
	time.AfterFunc(voiceConnectTTL, func() {
		s.mu.Lock()
		waiting := !v.connected
		s.mu.Unlock()
		if waiting {
			s.endVoice(v)
		}
	})
	return assistantv1.StartVoiceSessionResponse_builder{
		Url:        new(strings.Replace(VoicePath, "{token}", v.url, 1)),
		SessionId:  &v.id,
		ExpireTime: timestamppb.New(s.now().Add(voiceConnectTTL)),
	}.Build(), nil
}

// own returns the ID within plugin of a setting's "<plugin>/<id>", or
// empty for another plugin's, which leaves the choice to the plugin.
func own(ref, plugin string) string {
	if id, ok := strings.CutPrefix(ref, plugin+"/"); ok {
		return id
	}
	return ""
}

// voiceOf returns the default voice of a language or of its base.
func voiceOf(voices map[string]string, lang string) string {
	if v := voices[lang]; v != "" {
		return v
	}
	base, _, _ := strings.Cut(lang, "-")
	return voices[base]
}

// VoiceSession resolves a session token for the host API.
func (s *Service) VoiceSession(token string) (auth.VoiceSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.voice.byToken[token]
	if !ok {
		return auth.VoiceSession{}, false
	}
	return auth.VoiceSession{Plugin: v.plugin.ID, Session: v.device}, true
}

// forgetVoice revokes a session's tokens and ends its relay.
func (s *Service) forgetVoice(v *voiceSession) {
	s.mu.Lock()
	delete(s.voice.byURL, v.url)
	delete(s.voice.byToken, v.token)
	stop := v.stop
	s.mu.Unlock()
	if stop != nil {
		stop()
	}
}

// endVoice ends a session: its tokens are revoked, its relay ends and the
// plugin closes it.
func (s *Service) endVoice(v *voiceSession) {
	v.once.Do(func() {
		s.forgetVoice(v)
		ctx, cancel := context.WithTimeout(context.Background(), voiceCloseTimeout)
		defer cancel()
		req := pluginv1.CloseVoiceSessionRequest_builder{SessionId: &v.id}.Build()
		if _, err := v.plugin.Client.CloseVoiceSession(ctx, req); err != nil {
			s.log.WarnContext(ctx, "assistant: close voice session", "session", v.id, "err", err)
		}
	})
}

// endVoices ends every session, when the server stops.
func (s *Service) endVoices() {
	s.mu.Lock()
	var all []*voiceSession
	for _, v := range s.voice.byURL {
		all = append(all, v)
	}
	s.mu.Unlock()
	for _, v := range all {
		s.endVoice(v)
	}
}

// frame is a WebSocket message with its payload type, which rawFrames
// keeps so that text and binary messages are relayed as they came.
type frame struct {
	typ  byte
	data []byte
}

var rawFrames = websocket.Codec{
	Marshal: func(v any) ([]byte, byte, error) {
		f := v.(frame)
		return f.data, f.typ, nil
	},
	Unmarshal: func(data []byte, typ byte, v any) error {
		*v.(*frame) = frame{typ: typ, data: data}
		return nil
	},
}

// VoiceHandler serves the voice sessions' WebSockets: the URL's token,
// which serves once, authenticates, and the server relays the messages
// both ways, unchanged, to the session's WebSocket on the plugin's socket
// (§8.1).
func (s *Service) VoiceHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		v, ok := s.voice.byURL[r.PathValue("token")]
		if ok && v.connected {
			ok = false
		}
		var ctx context.Context
		if ok {
			v.connected = true
			ctx, v.stop = context.WithTimeout(context.WithoutCancel(r.Context()), voiceMaxTime)
		}
		s.mu.Unlock()
		if !ok {
			http.Error(w, "no voice session has this URL", http.StatusNotFound)
			return
		}
		defer s.endVoice(v)
		up, err := s.dialVoice(ctx, v)
		if err != nil {
			s.log.WarnContext(r.Context(), "assistant: connect to voice session", "session", v.id, "err", err)
			http.Error(w, "the voice session could not be reached", http.StatusBadGateway)
			return
		}
		defer up.Close()
		websocket.Server{
			// The URL's token authenticates, so the origin need not be
			// checked.
			Handshake: func(*websocket.Config, *http.Request) error { return nil },
			Handler: func(down *websocket.Conn) {
				defer down.Close()
				relay(ctx, down, up)
			},
		}.ServeHTTP(w, r)
	})
}

// dialVoice connects to a session's WebSocket on the plugin's socket.
func (s *Service) dialVoice(ctx context.Context, v *voiceSession) (*websocket.Conn, error) {
	conn, err := v.plugin.Dial(ctx)
	if err != nil {
		return nil, err
	}
	cfg, err := websocket.NewConfig("ws://plugin"+v.path, "http://plugin")
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	cfg.Header = http.Header{}
	v.plugin.Authorize(cfg.Header)
	// The handshake honors ctx through the connection's deadline.
	stop := context.AfterFunc(ctx, func() { _ = conn.SetDeadline(time.Now()) })
	ws, err := websocket.NewClient(cfg, conn)
	if !stop() {
		err = errors.Join(err, ctx.Err())
	}
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	return ws, nil
}

// relay copies messages both ways until either side ends or ctx does.
func relay(ctx context.Context, a, b *websocket.Conn) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, func() {
		_ = a.Close()
		_ = b.Close()
	})
	defer stop()
	var wg sync.WaitGroup
	for _, c := range [][2]*websocket.Conn{{a, b}, {b, a}} {
		wg.Go(func() {
			defer cancel()
			for {
				var f frame
				if err := rawFrames.Receive(c[0], &f); err != nil {
					return
				}
				if err := rawFrames.Send(c[1], f); err != nil {
					return
				}
			}
		})
	}
	wg.Wait()
}

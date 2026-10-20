package assistant

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
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
	// voiceOutputRate is the rate of the answers' speech when the
	// session's ready event gives none.
	voiceOutputRate = 24000
	// maxKeptSpeech bounds the speech of an answer that is kept.
	maxKeptSpeech = 10 * time.Minute
	// keepTimeout bounds encoding and keeping an answer's speech.
	keepTimeout = time.Minute
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
	// user is the session's user on the device; device is the signed-in
	// session of the user's device.
	user   auth.Principal
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

	v := &voiceSession{id: newToken()[:32], plugin: vp, user: p, device: p.Session.ID, url: newToken(), token: newToken()}
	lang := req.GetLanguage()
	sp := set.SpeechOf(lang)
	open := pluginv1.OpenVoiceSessionRequest_builder{
		SessionId: &v.id, Language: &lang, Mode: req.GetMode().Enum(), SessionToken: &v.token,
		Recognizer: new(own(sp.Recognizer, vp.ID)), Voice: new(own(sp.Voice, vp.ID)),
		Turns: pluginv1.VoiceTurnSettings_builder{
			LongestPause: durationpb.New(set.LongestPause), Idle: durationpb.New(set.SessionIdle),
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
		tap := &speechTap{s: s, v: v, rate: voiceOutputRate}
		defer tap.close()
		websocket.Server{
			// The URL's token authenticates, so the origin need not be
			// checked.
			Handshake: func(*websocket.Config, *http.Request) error { return nil },
			Handler: func(down *websocket.Conn) {
				defer down.Close()
				relay(ctx, down, up, tap.fromClient, tap.fromPlugin)
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

// relay copies messages both ways until either side ends or ctx does,
// showing those from a to fromA and those from b to fromB.
func relay(ctx context.Context, a, b *websocket.Conn, fromA, fromB func(frame)) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	stop := context.AfterFunc(ctx, func() {
		_ = a.Close()
		_ = b.Close()
	})
	defer stop()
	var wg sync.WaitGroup
	for _, c := range []struct {
		from, to *websocket.Conn
		see      func(frame)
	}{{a, b, fromA}, {b, a, fromB}} {
		wg.Go(func() {
			defer cancel()
			for {
				var f frame
				if err := rawFrames.Receive(c.from, &f); err != nil {
					return
				}
				if c.see != nil {
					c.see(f)
				}
				if err := rawFrames.Send(c.to, f); err != nil {
					return
				}
			}
		})
	}
	wg.Wait()
}

// speechTap collects the answers' speech a session's relay carries, to keep
// it with the answers in their conversations (§8.8): the audio from the
// plugin between a run's start and its end, cut where the client says it
// stopped playing after an interrupt.
type speechTap struct {
	s *Service
	v *voiceSession

	mu   sync.Mutex
	rate int
	// pcm is the speech of the run under way, or of the run ended that
	// waits for what the client played.
	pcm     []byte
	running bool
	// interrupted is set by an interrupt, until the next run; played is
	// how much the client played then, in bytes, -1 until it says.
	interrupted bool
	played      int
	// ended is the end of the run whose speech waits to be kept.
	ended *runEnd
}

// runEnd is what keeping a run's speech needs of its end event.
type runEnd struct {
	conversation, answer string
}

// tapEvent is the fields of the session's events the tap reads (§8.6).
type tapEvent struct {
	Type             string  `json:"type"`
	OutputSampleRate int     `json:"output_sample_rate"`
	Status           string  `json:"status"`
	ConversationID   string  `json:"conversation_id"`
	Answer           string  `json:"answer"`
	Ms               float64 `json:"ms"`
}

func (t *speechTap) fromPlugin(f frame) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if f.typ == websocket.BinaryFrame {
		if t.running && !t.interrupted && len(t.pcm)+len(f.data) <= t.rate*2*int(maxKeptSpeech/time.Second) {
			t.pcm = append(t.pcm, f.data...)
		}
		return
	}
	var e tapEvent
	if json.Unmarshal(f.data, &e) != nil {
		return
	}
	switch e.Type {
	case "ready":
		if e.OutputSampleRate > 0 {
			t.rate = e.OutputSampleRate
		}
	case "run":
		t.flush()
		t.running, t.interrupted, t.played = true, false, -1
	case "interrupt":
		if t.running && !t.interrupted {
			t.interrupted, t.played = true, -1
		}
	case "end":
		if !t.running {
			return
		}
		t.running = false
		if e.Status == "failed" || e.ConversationID == "" {
			t.pcm = nil
			return
		}
		t.ended = &runEnd{conversation: e.ConversationID, answer: e.Answer}
		if !t.interrupted || t.played >= 0 {
			t.flush()
		}
	}
}

func (t *speechTap) fromClient(f frame) {
	if f.typ != websocket.TextFrame {
		return
	}
	var e tapEvent
	if json.Unmarshal(f.data, &e) != nil || e.Type != "played" {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.interrupted || t.played >= 0 {
		return
	}
	t.played = max(0, int(e.Ms*float64(t.rate)/1000)*2)
	if t.ended != nil {
		t.flush()
	}
}

// close keeps the speech of a run that ended, when the relay ends.
func (t *speechTap) close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.flush()
}

// flush keeps the speech of the run that ended, cut at what the client
// played, and forgets it; t.mu is held.
func (t *speechTap) flush() {
	pcm, end := t.pcm, t.ended
	t.pcm, t.ended = nil, nil
	if end == nil || len(pcm) == 0 {
		return
	}
	if t.interrupted && t.played >= 0 && t.played < len(pcm) {
		pcm = pcm[:t.played]
	}
	rate, s, user := t.rate, t.s, t.v.user
	s.keeping.Go(func() {
		ctx, cancel := context.WithTimeout(context.Background(), keepTimeout)
		defer cancel()
		// The plugin names the conversation: it must be the user's.
		if _, _, _, err := s.own(auth.WithPrincipal(ctx, user), end.conversation); err != nil {
			s.log.WarnContext(ctx, "assistant: keep speech of another's conversation", "conversation", end.conversation, "err", err)
			return
		}
		if err := s.rec.keepPCM(ctx, end.conversation, end.answer, pcm, rate); err != nil {
			s.log.WarnContext(ctx, "assistant: keep speech", "conversation", end.conversation, "err", err)
		}
	})
}

package assistant

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/websocket"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// fakeVoicePlugin opens sessions and serves their WebSockets, echoing
// every message.
type fakeVoicePlugin struct {
	pluginv1connect.UnimplementedVoiceSessionServiceHandler
	srv *httptest.Server

	mu     sync.Mutex
	opened []*pluginv1.OpenVoiceSessionRequest
	closed []string
	// auth is the Authorization header of the last WebSocket.
	auth string
}

func newFakeVoicePlugin(t *testing.T) *fakeVoicePlugin {
	f := &fakeVoicePlugin{}
	mux := http.NewServeMux()
	mux.Handle("GET /_mavio/voice/{id}", websocket.Server{
		Handler: func(c *websocket.Conn) {
			f.mu.Lock()
			f.auth = c.Request().Header.Get("Authorization")
			f.mu.Unlock()
			for {
				var fr frame
				if err := rawFrames.Receive(c, &fr); err != nil {
					return
				}
				if err := rawFrames.Send(c, fr); err != nil {
					return
				}
			}
		},
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeVoicePlugin) OpenVoiceSession(_ context.Context, req *pluginv1.OpenVoiceSessionRequest) (*pluginv1.OpenVoiceSessionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, req)
	return pluginv1.OpenVoiceSessionResponse_builder{Path: new("/_mavio/voice/" + req.GetSessionId())}.Build(), nil
}

func (f *fakeVoicePlugin) CloseVoiceSession(_ context.Context, req *pluginv1.CloseVoiceSessionRequest) (*pluginv1.CloseVoiceSessionResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = append(f.closed, req.GetSessionId())
	return &pluginv1.CloseVoiceSessionResponse{}, nil
}

func (f *fakeVoicePlugin) closedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.closed...)
}

func (f *fakeVoicePlugin) plugin() VoicePlugin {
	return VoicePlugin{
		ID: "dev.mavio.pipecat", Client: f,
		Dial: func(ctx context.Context) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "tcp", f.srv.Listener.Addr().String())
		},
		Authorize: func(h http.Header) { h.Set("Authorization", "Bearer plugin-token") },
	}
}

func newVoiceService(f *fakeVoicePlugin) (*Service, *httptest.Server) {
	set := core.DefaultAssistantSettings()
	set.Hints = false
	set.Recognizer = "dev.mavio.pipecat/whisper-1"
	set.Voices = map[string]string{"en": "other.plugin/v"}
	s := New(Config{
		Settings: func() core.AssistantSettings { return set },
		VoiceSessions: func() (VoicePlugin, bool) {
			if f == nil {
				return VoicePlugin{}, false
			}
			return f.plugin(), true
		},
	})
	mux := http.NewServeMux()
	mux.Handle("GET "+VoicePath, s.VoiceHandler())
	return s, httptest.NewServer(mux)
}

func startVoice(t *testing.T, s *Service, p auth.Principal) *assistantv1.StartVoiceSessionResponse {
	t.Helper()
	req := assistantv1.StartVoiceSessionRequest_builder{
		Language: new("en"), Mode: pluginv1.VoiceMode_VOICE_MODE_CONVERSATION.Enum(),
		Context: assistantv1.AskContext_builder{Languages: []string{"en"}}.Build(),
	}.Build()
	resp, err := s.StartVoiceSession(auth.WithPrincipal(t.Context(), p), req)
	if err != nil {
		t.Fatalf("StartVoiceSession() = %v", err)
	}
	return resp
}

func dialVoiceURL(srv *httptest.Server, path string) (*websocket.Conn, error) {
	return websocket.Dial("ws"+strings.TrimPrefix(srv.URL, "http")+path, "", srv.URL)
}

func TestVoiceSessionRelays(t *testing.T) {
	f := newFakeVoicePlugin(t)
	s, srv := newVoiceService(f)
	defer srv.Close()
	resp := startVoice(t, s, signedIn)

	if len(f.opened) != 1 {
		t.Fatalf("opened = %d, want = 1", len(f.opened))
	}
	open := f.opened[0]
	if open.GetSessionId() != resp.GetSessionId() || open.GetLanguage() != "en" || open.GetMode() != pluginv1.VoiceMode_VOICE_MODE_CONVERSATION {
		t.Errorf("open = %v, want the session's ID, language and mode", open)
	}
	// The recognizer of the settings is the plugin's; the voice is not.
	if open.GetRecognizer() != "whisper-1" || open.GetVoice() != "" {
		t.Errorf("recognizer, voice = %q, %q, want = whisper-1, empty", open.GetRecognizer(), open.GetVoice())
	}
	if got := open.GetTurns().GetCommitDelay().AsDuration(); got != voiceCommitDelay {
		t.Errorf("commit delay = %v, want = %v", got, voiceCommitDelay)
	}
	if !strings.Contains(open.GetContextJson(), `"en"`) {
		t.Errorf("context json = %s, want the client's context", open.GetContextJson())
	}
	v, ok := s.VoiceSession(open.GetSessionToken())
	if !ok || v.Plugin != "dev.mavio.pipecat" || v.Session != signedIn.Session.ID {
		t.Errorf("VoiceSession(token) = %v, %v, want the plugin and the device", v, ok)
	}

	c, err := dialVoiceURL(srv, resp.GetUrl())
	if err != nil {
		t.Fatal(err)
	}
	// Text and binary messages come back as they went.
	for _, fr := range []frame{{websocket.BinaryFrame, []byte{1, 2, 3, 4}}, {websocket.TextFrame, []byte(`{"type":"ptt_start"}`)}} {
		if err := rawFrames.Send(c, fr); err != nil {
			t.Fatal(err)
		}
		var got frame
		if err := rawFrames.Receive(c, &got); err != nil {
			t.Fatal(err)
		}
		if got.typ != fr.typ || string(got.data) != string(fr.data) {
			t.Errorf("echo = %d %q, want = %d %q", got.typ, got.data, fr.typ, fr.data)
		}
	}
	f.mu.Lock()
	if f.auth != "Bearer plugin-token" {
		t.Errorf("plugin's Authorization = %q, want the plugin's token", f.auth)
	}
	f.mu.Unlock()
	// The URL serves once.
	if _, err := dialVoiceURL(srv, resp.GetUrl()); err == nil {
		t.Error("a second connection to the URL succeeded")
	}

	_ = c.Close()
	deadline := time.Now().Add(5 * time.Second)
	for len(f.closedIDs()) == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if got := f.closedIDs(); len(got) != 1 || got[0] != resp.GetSessionId() {
		t.Errorf("closed = %v, want = [%s]", got, resp.GetSessionId())
	}
	if _, ok := s.VoiceSession(open.GetSessionToken()); ok {
		t.Error("the session token still resolves after the session ended")
	}
}

func TestVoiceSessionOnePerDevice(t *testing.T) {
	f := newFakeVoicePlugin(t)
	s, srv := newVoiceService(f)
	defer srv.Close()
	first := startVoice(t, s, signedIn)
	other := signedIn
	other.Session = core.AuthSession{ID: core.NewID()}
	startVoice(t, s, other)
	if got := f.closedIDs(); len(got) != 0 {
		t.Fatalf("closed = %v, want none for another device", got)
	}
	startVoice(t, s, signedIn)
	if got := f.closedIDs(); len(got) != 1 || got[0] != first.GetSessionId() {
		t.Errorf("closed = %v, want the device's first session", got)
	}
	if _, err := dialVoiceURL(srv, first.GetUrl()); err == nil {
		t.Error("the ended session's URL still connects")
	}
}

func TestVoiceSessionNeeds(t *testing.T) {
	s, srv := newVoiceService(nil)
	srv.Close()
	for _, tc := range []struct {
		name string
		p    auth.Principal
		want connect.Code
	}{
		{"no plugin", signedIn, connect.CodeFailedPrecondition},
		{"no device", auth.Principal{User: signedIn.User}, connect.CodePermissionDenied},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := s.StartVoiceSession(auth.WithPrincipal(t.Context(), tc.p), &assistantv1.StartVoiceSessionRequest{})
			var ce *connect.Error
			if !errors.As(err, &ce) || ce.Code() != tc.want {
				t.Errorf("StartVoiceSession() = %v, want = %v", err, tc.want)
			}
		})
	}
}

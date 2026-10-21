package rpc

import (
	"context"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// echoSessions is a plugin answering each message with its text, in two parts.
type echoSessions struct {
	pluginv1connect.UnimplementedAgentSessionServiceHandler
}

func (echoSessions) StreamAgentMessage(_ context.Context, req *pluginv1.StreamAgentMessageRequest, stream *connect.ServerStream[pluginv1.StreamAgentMessageResponse]) error {
	text := req.GetMessage().GetText()
	for _, part := range []string{text[:1], text} {
		m := &pluginv1.StreamAgentMessageResponse{}
		m.SetText(part)
		if err := stream.Send(m); err != nil {
			return err
		}
	}
	m := &pluginv1.StreamAgentMessageResponse{}
	m.SetDone(pluginv1.SendAgentMessageResponse_builder{
		Answer: pluginv1.AgentMessage_builder{Role: pluginv1.AiRole_AI_ROLE_ASSISTANT.Enum(), Text: &text}.Build(),
	}.Build())
	return stream.Send(m)
}

func TestAiConversations(t *testing.T) {
	_, h := pluginv1connect.NewAgentSessionServiceHandler(echoSessions{})
	plugin := httptest.NewServer(h)
	t.Cleanup(plugin.Close)
	sessions := pluginv1connect.NewAgentSessionServiceClient(plugin.Client(), plugin.URL)

	serve := func(kind core.UserKind, svc *AiConversationService) systemv1connect.AiConversationServiceClient {
		_, h := systemv1connect.NewAiConversationServiceHandler(svc)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			h.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{User: core.User{Kind: kind}})))
		}))
		t.Cleanup(srv.Close)
		return systemv1connect.NewAiConversationServiceClient(srv.Client(), srv.URL)
	}
	running := &AiConversationService{Sessions: func() (pluginv1connect.AgentSessionServiceClient, bool) { return sessions, true }}
	send := func(c systemv1connect.AiConversationServiceClient) ([]string, error) {
		stream, err := c.SendAiMessage(t.Context(), systemv1.SendAiMessageRequest_builder{ConversationId: new("c"), Text: new("hello")}.Build())
		if err != nil {
			return nil, err
		}
		var got []string
		for stream.Receive() {
			if m := stream.Msg(); m.HasDone() {
				got = append(got, "done: "+m.GetDone().GetAnswer().GetText())
			} else {
				got = append(got, m.GetText())
			}
		}
		return got, stream.Err()
	}

	got, err := send(serve(core.UserAdmin, running))
	if want := []string{"h", "hello", "done: hello"}; err != nil || !slices.Equal(got, want) {
		t.Errorf("SendAiMessage() = %q, %v, want = %q", got, err, want)
	}
	for _, tt := range []struct {
		name string
		kind core.UserKind
		svc  *AiConversationService
		want connect.Code
	}{
		{"member", core.UserMember, running, connect.CodePermissionDenied},
		{"no plugin", core.UserAdmin, &AiConversationService{}, connect.CodeFailedPrecondition},
	} {
		if _, err := send(serve(tt.kind, tt.svc)); connect.CodeOf(err) != tt.want {
			t.Errorf("SendAiMessage() by %s: got = %v, want = %v", tt.name, err, tt.want)
		}
	}
}

package rpc

import (
	"context"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/assistant"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// echoSessions is a plugin answering each message with its text, in two
// parts, keeping the conversation "c" of the administrators and the
// assistant's conversations of its users, by ID.
type echoSessions struct {
	pluginv1connect.UnimplementedAgentSessionServiceHandler
	// assistant are the owners of the assistant's conversations, by ID.
	assistant map[string]string
}

func (e echoSessions) sessions() []*pluginv1.AgentSession {
	out := []*pluginv1.AgentSession{pluginv1.AgentSession_builder{Id: new("c")}.Build()}
	for _, id := range slices.Sorted(maps.Keys(e.assistant)) {
		out = append(out, pluginv1.AgentSession_builder{
			Id: new(id), Owner: new(e.assistant[id]), Labels: map[string]string{assistant.SessionLabel: "1"},
		}.Build())
	}
	return out
}

func (e echoSessions) ListAgentSessions(context.Context, *pluginv1.ListAgentSessionsRequest) (*pluginv1.ListAgentSessionsResponse, error) {
	return pluginv1.ListAgentSessionsResponse_builder{Sessions: e.sessions()}.Build(), nil
}

func (e echoSessions) GetAgentSession(_ context.Context, req *pluginv1.GetAgentSessionRequest) (*pluginv1.GetAgentSessionResponse, error) {
	for _, sess := range e.sessions() {
		if sess.GetId() == req.GetId() {
			return pluginv1.GetAgentSessionResponse_builder{Session: sess}.Build(), nil
		}
	}
	return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
}

func (echoSessions) DeleteAgentSession(context.Context, *pluginv1.DeleteAgentSessionRequest) (*pluginv1.DeleteAgentSessionResponse, error) {
	return &pluginv1.DeleteAgentSessionResponse{}, nil
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

func TestAiConversationsHideOthersAssistantConversations(t *testing.T) {
	me, other := core.NewID(), core.NewID()
	_, h := pluginv1connect.NewAgentSessionServiceHandler(echoSessions{assistant: map[string]string{
		"mine": me.String(), "theirs": other.String(),
	}})
	plugin := httptest.NewServer(h)
	t.Cleanup(plugin.Close)
	sessions := pluginv1connect.NewAgentSessionServiceClient(plugin.Client(), plugin.URL)
	_, handler := systemv1connect.NewAiConversationServiceHandler(&AiConversationService{
		Sessions: func() (pluginv1connect.AgentSessionServiceClient, bool) { return sessions, true },
	})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := auth.Principal{User: core.User{ID: me, Kind: core.UserAdmin}}
		handler.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
	}))
	t.Cleanup(srv.Close)
	c := systemv1connect.NewAiConversationServiceClient(srv.Client(), srv.URL)

	list, err := c.ListAiConversations(t.Context(), &systemv1.ListAiConversationsRequest{})
	var got []string
	for _, conv := range list.GetConversations() {
		got = append(got, conv.GetId())
	}
	if want := []string{"c", "mine"}; err != nil || !slices.Equal(got, want) {
		t.Errorf("ListAiConversations() = %q, %v, want = %q", got, err, want)
	}
	for _, tt := range []struct {
		id   string
		want connect.Code
	}{{"c", 0}, {"mine", 0}, {"theirs", connect.CodeNotFound}} {
		_, err := c.GetAiConversation(t.Context(), systemv1.GetAiConversationRequest_builder{Id: new(tt.id)}.Build())
		if got := connect.CodeOf(err); err != nil && got != tt.want || err == nil && tt.want != 0 {
			t.Errorf("GetAiConversation(%s) = %v, want = %v", tt.id, err, tt.want)
		}
		_, err = c.DeleteAiConversation(t.Context(), systemv1.DeleteAiConversationRequest_builder{Id: new(tt.id)}.Build())
		if got := connect.CodeOf(err); err != nil && got != tt.want || err == nil && tt.want != 0 {
			t.Errorf("DeleteAiConversation(%s) = %v, want = %v", tt.id, err, tt.want)
		}
		stream, err := c.SendAiMessage(t.Context(), systemv1.SendAiMessageRequest_builder{ConversationId: new(tt.id), Text: new("hi")}.Build())
		if err == nil {
			for stream.Receive() {
			}
			err = stream.Err()
		}
		if got := connect.CodeOf(err); err != nil && got != tt.want || err == nil && tt.want != 0 {
			t.Errorf("SendAiMessage(%s) = %v, want = %v", tt.id, err, tt.want)
		}
	}
}

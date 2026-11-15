package rpc

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"sync"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/settings"
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

func (echoSessions) GetAgentSession(_ context.Context, req *pluginv1.GetAgentSessionRequest) (*pluginv1.GetAgentSessionResponse, error) {
	return pluginv1.GetAgentSessionResponse_builder{Session: pluginv1.AgentSession_builder{Id: new(req.GetId())}.Build()}.Build(), nil
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

// keptSessions is a plugin keeping conversations in memory, by ID.
type keptSessions struct {
	pluginv1connect.UnimplementedAgentSessionServiceHandler
	mu       sync.Mutex
	sessions map[string]*pluginv1.AgentSession
	n        int
}

func (k *keptSessions) CreateAgentSession(_ context.Context, req *pluginv1.CreateAgentSessionRequest) (*pluginv1.CreateAgentSessionResponse, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.n++
	sess := pluginv1.AgentSession_builder{Id: new(fmt.Sprint("c", k.n)), Owner: new(req.GetOwner()), Labels: req.GetLabels()}.Build()
	k.sessions[sess.GetId()] = sess
	return pluginv1.CreateAgentSessionResponse_builder{Session: sess}.Build(), nil
}

func (k *keptSessions) ListAgentSessions(_ context.Context, req *pluginv1.ListAgentSessionsRequest) (*pluginv1.ListAgentSessionsResponse, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []*pluginv1.AgentSession
	for _, id := range slices.Sorted(maps.Keys(k.sessions)) {
		if sess := k.sessions[id]; req.GetOwner() == "" || sess.GetOwner() == req.GetOwner() {
			out = append(out, sess)
		}
	}
	return pluginv1.ListAgentSessionsResponse_builder{Sessions: out}.Build(), nil
}

func (k *keptSessions) GetAgentSession(_ context.Context, req *pluginv1.GetAgentSessionRequest) (*pluginv1.GetAgentSessionResponse, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	sess, ok := k.sessions[req.GetId()]
	if !ok {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("no such session"))
	}
	return pluginv1.GetAgentSessionResponse_builder{Session: sess}.Build(), nil
}

func (k *keptSessions) StreamAgentMessage(ctx context.Context, req *pluginv1.StreamAgentMessageRequest, stream *connect.ServerStream[pluginv1.StreamAgentMessageResponse]) error {
	return echoSessions{}.StreamAgentMessage(ctx, req, stream)
}

func (k *keptSessions) DeleteAgentSession(_ context.Context, req *pluginv1.DeleteAgentSessionRequest) (*pluginv1.DeleteAgentSessionResponse, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.sessions, req.GetId())
	return &pluginv1.DeleteAgentSessionResponse{}, nil
}

func TestAiConversationsAreTheirOwners(t *testing.T) {
	ctx := t.Context()
	m, err := settings.Open(ctx, openStore(t))
	if err != nil {
		t.Fatal(err)
	}
	admin, member, other := core.NewID(), core.NewID(), core.NewID()
	kept := &keptSessions{sessions: map[string]*pluginv1.AgentSession{
		// From before owners were recorded.
		"old": pluginv1.AgentSession_builder{Id: new("old")}.Build(),
		// The member's conversation with the assistant.
		"assistant": pluginv1.AgentSession_builder{Id: new("assistant"), Owner: new(member.String()), Labels: map[string]string{assistantLabel: "1"}}.Build(),
	}}
	_, h := pluginv1connect.NewAgentSessionServiceHandler(kept)
	plugin := httptest.NewServer(h)
	t.Cleanup(plugin.Close)
	sessions := pluginv1connect.NewAgentSessionServiceClient(plugin.Client(), plugin.URL)
	svc := &AiConversationService{Settings: m, Sessions: func() (pluginv1connect.AgentSessionServiceClient, bool) { return sessions, true }}
	_, handler := systemv1connect.NewAiConversationServiceHandler(svc)
	as := func(id core.ID, kind core.UserKind) systemv1connect.AiConversationServiceClient {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p := auth.Principal{User: core.User{ID: id, Kind: kind}}
			handler.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), p)))
		}))
		t.Cleanup(srv.Close)
		return systemv1connect.NewAiConversationServiceClient(srv.Client(), srv.URL)
	}
	asAdmin, asMember, asOther := as(admin, core.UserAdmin), as(member, core.UserMember), as(other, core.UserManaged)
	create := func(c systemv1connect.AiConversationServiceClient) (string, error) {
		resp, err := c.CreateAiConversation(ctx, systemv1.CreateAiConversationRequest_builder{Provider: new("p"), Model: new("m")}.Build())
		return resp.GetConversation().GetId(), err
	}
	list := func(c systemv1connect.AiConversationServiceClient) []string {
		t.Helper()
		resp, err := c.ListAiConversations(ctx, &systemv1.ListAiConversationsRequest{})
		if err != nil {
			t.Fatalf("ListAiConversations() = %v", err)
		}
		var ids []string
		for _, c := range resp.GetConversations() {
			ids = append(ids, c.GetId())
		}
		return ids
	}
	allowed := func(c systemv1connect.AiConversationServiceClient) bool {
		t.Helper()
		resp, err := c.GetAiConversationSettings(ctx, &systemv1.GetAiConversationSettingsRequest{})
		if err != nil {
			t.Fatalf("GetAiConversationSettings() = %v", err)
		}
		return resp.GetAllowed()
	}

	// By default only administrators keep conversations.
	mine, err := create(asAdmin)
	if err != nil || kept.sessions[mine].GetOwner() != admin.String() {
		t.Fatalf("CreateAiConversation() by an administrator = %q, %v; want = owned by them", kept.sessions[mine].GetOwner(), err)
	}
	if !allowed(asAdmin) || allowed(asMember) {
		t.Errorf("allowed = %v, %v; want = the administrator only", allowed(asAdmin), allowed(asMember))
	}
	if _, err := create(asMember); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("CreateAiConversation() by a member: got = %v, want = %v", err, connect.CodePermissionDenied)
	}
	if _, err := asMember.ListAiConversations(ctx, &systemv1.ListAiConversationsRequest{}); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("ListAiConversations() by a member: got = %v, want = %v", err, connect.CodePermissionDenied)
	}
	on := systemv1.UpdateAiConversationSettingsRequest_builder{Settings: systemv1.AiConversationSettings_builder{AllUsers: new(true)}.Build()}.Build()
	if _, err := asMember.UpdateAiConversationSettings(ctx, on); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("UpdateAiConversationSettings() by a member: got = %v, want = %v", err, connect.CodePermissionDenied)
	}
	if resp, err := asAdmin.UpdateAiConversationSettings(ctx, on); err != nil || !resp.GetSettings().GetAllUsers() {
		t.Fatalf("UpdateAiConversationSettings() = %v, %v", resp, err)
	}
	if !allowed(asMember) {
		t.Errorf("allowed for a member = false, want = true once conversations are for all users")
	}

	// Each user then keeps their own, and sees only those.
	theirs, err := create(asMember)
	if err != nil || kept.sessions[theirs].GetOwner() != member.String() {
		t.Fatalf("CreateAiConversation() by a member = %q, %v; want = owned by them", kept.sessions[theirs].GetOwner(), err)
	}
	others, err := create(asOther)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := list(asMember), []string{theirs}; !slices.Equal(got, want) {
		t.Errorf("ListAiConversations() by a member = %q, want = %q", got, want)
	}
	if got, want := list(asOther), []string{others}; !slices.Equal(got, want) {
		t.Errorf("ListAiConversations() by another = %q, want = %q", got, want)
	}
	// Administrators see every conversation.
	if got, want := list(asAdmin), slices.Sorted(slices.Values([]string{"assistant", "old", mine, theirs, others})); !slices.Equal(got, want) {
		t.Errorf("ListAiConversations() by an administrator = %q, want = %q", got, want)
	}
	for _, id := range []string{"old", "assistant", mine, others, "missing"} {
		if _, err := asMember.GetAiConversation(ctx, systemv1.GetAiConversationRequest_builder{Id: &id}.Build()); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("GetAiConversation(%s) by a member: got = %v, want = %v", id, err, connect.CodeNotFound)
		}
		if _, err := asMember.DeleteAiConversation(ctx, systemv1.DeleteAiConversationRequest_builder{Id: &id}.Build()); connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("DeleteAiConversation(%s) by a member: got = %v, want = %v", id, err, connect.CodeNotFound)
		}
		stream, err := asMember.SendAiMessage(ctx, systemv1.SendAiMessageRequest_builder{ConversationId: &id, Text: new("hi")}.Build())
		if err == nil {
			for stream.Receive() {
			}
			err = stream.Err()
		}
		if connect.CodeOf(err) != connect.CodeNotFound {
			t.Errorf("SendAiMessage(%s) by a member: got = %v, want = %v", id, err, connect.CodeNotFound)
		}
	}
	if _, ok := kept.sessions[others]; !ok {
		t.Errorf("another's conversation was deleted by the member")
	}
	if _, err := asMember.GetAiConversation(ctx, systemv1.GetAiConversationRequest_builder{Id: &theirs}.Build()); err != nil {
		t.Errorf("GetAiConversation() of their own = %v", err)
	}
	if _, err := asAdmin.GetAiConversation(ctx, systemv1.GetAiConversationRequest_builder{Id: &theirs}.Build()); err != nil {
		t.Errorf("GetAiConversation() of a member's by an administrator = %v", err)
	}
	if _, err := asMember.DeleteAiConversation(ctx, systemv1.DeleteAiConversationRequest_builder{Id: &theirs}.Build()); err != nil {
		t.Errorf("DeleteAiConversation() of their own = %v", err)
	}
	if got := list(asMember); len(got) != 0 {
		t.Errorf("ListAiConversations() by a member after deleting = %q, want = none", got)
	}
}

// fakeGateway is an AI gateway plugin with a provider and its model.
type fakeGateway struct {
	pluginv1connect.UnimplementedAiGatewayServiceHandler
}

func (fakeGateway) ListAiModels(_ context.Context, req *pluginv1.ListAiModelsRequest) (*pluginv1.ListAiModelsResponse, error) {
	if !req.GetAvailable() {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("want the available models"))
	}
	return pluginv1.ListAiModelsResponse_builder{Models: []*pluginv1.AiModel{pluginv1.AiModel_builder{Provider: new("p"), Id: new("m")}.Build()}}.Build(), nil
}

func (fakeGateway) ListAiProviders(context.Context, *pluginv1.ListAiProvidersRequest) (*pluginv1.ListAiProvidersResponse, error) {
	return pluginv1.ListAiProvidersResponse_builder{Providers: []*pluginv1.AiProvider{pluginv1.AiProvider_builder{Id: new("p"), Name: new("Provider")}.Build()}}.Build(), nil
}

func TestListAiConversationModels(t *testing.T) {
	_, h := pluginv1connect.NewAiGatewayServiceHandler(fakeGateway{})
	plugin := httptest.NewServer(h)
	t.Cleanup(plugin.Close)
	g := pluginv1connect.NewAiGatewayServiceClient(plugin.Client(), plugin.URL)
	_, handler := systemv1connect.NewAiConversationServiceHandler(&AiConversationService{
		Gateway: func() (string, pluginv1connect.AiGatewayServiceClient, bool) { return "gw", g, true },
	})
	for _, tt := range []struct {
		kind core.UserKind
		want error
	}{{core.UserAdmin, nil}, {core.UserMember, connect.NewError(connect.CodePermissionDenied, nil)}} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r.WithContext(auth.WithPrincipal(r.Context(), auth.Principal{User: core.User{ID: core.NewID(), Kind: tt.kind}})))
		}))
		t.Cleanup(srv.Close)
		resp, err := systemv1connect.NewAiConversationServiceClient(srv.Client(), srv.URL).ListAiConversationModels(t.Context(), &systemv1.ListAiConversationModelsRequest{})
		if (err == nil) != (tt.want == nil) || err != nil && connect.CodeOf(err) != connect.CodeOf(tt.want) {
			t.Errorf("ListAiConversationModels() by %s: got = %v, want = %v", tt.kind, err, tt.want)
		}
		if err == nil && (len(resp.GetModels()) != 1 || resp.GetProviderNames()["p"] != "Provider") {
			t.Errorf("ListAiConversationModels() = %v, want = the model and its provider's name", resp)
		}
	}
}

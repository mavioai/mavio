package rpc

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// AIGateway returns the started AI gateway plugin's ID and client, or
// false when none is started.
type AIGateway func() (string, pluginv1connect.AiGatewayServiceClient, bool)

// AgentSessions returns the started plugin keeping conversations with
// models, or false when none is started.
type AgentSessions func() (pluginv1connect.AgentSessionServiceClient, bool)

// AiProviderService implements systemv1connect.AiProviderServiceHandler
// over the AI gateway plugin.
type AiProviderService struct {
	// Gateway finds the AI gateway plugin; nil finds none.
	Gateway AIGateway
	// Activity records credential changes; nil records none.
	Activity *activity.Log
}

var _ systemv1connect.AiProviderServiceHandler = (*AiProviderService)(nil)

// AiConversationService implements
// systemv1connect.AiConversationServiceHandler over the plugin keeping
// conversations with models. A conversation is its owner's, the user who
// started it: administrators see every conversation, other users, while
// the settings allow them conversations, only their own.
type AiConversationService struct {
	// Sessions finds the plugin keeping conversations; nil finds none.
	Sessions AgentSessions
	// Gateway finds the AI gateway plugin, whose models conversations
	// start with; nil finds none.
	Gateway AIGateway
	// Settings keeps who may keep conversations; nil allows only
	// administrators and changes nothing.
	Settings *settings.Manager
	// Activity records changes of the settings; nil records none.
	Activity *activity.Log
}

// assistantLabel labels the assistant's conversations, as the assistant
// starts them.
const assistantLabel = "mavio.assistant"

var _ systemv1connect.AiConversationServiceHandler = (*AiConversationService)(nil)

// gateway checks the caller is an administrator and returns the gateway.
func (s *AiProviderService) gateway(ctx context.Context) (string, pluginv1connect.AiGatewayServiceClient, error) {
	if _, err := admin(ctx); err != nil {
		return "", nil, err
	}
	if s.Gateway != nil {
		if id, c, ok := s.Gateway(); ok {
			return id, c, nil
		}
	}
	return "", nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no AI gateway plugin is running"))
}

// gatewayError converts the plugin's failures: its unauthenticated and
// permission denied are about the provider, not the caller.
func gatewayError(ctx context.Context, err error) error {
	if c := connect.CodeOf(err); c == connect.CodeUnauthenticated || c == connect.CodePermissionDenied {
		return connect.NewError(connect.CodeFailedPrecondition, err)
	}
	return connectError(ctx, err)
}

func (s *AiProviderService) record(ctx context.Context, kind, title, provider string) {
	p, _ := principal(ctx)
	s.Activity.Record(ctx, core.Activity{
		Type: kind, UserID: p.User.ID, Title: fmt.Sprintf(title, p.User.Name, provider),
		Attributes: map[string]string{"provider": provider},
	})
}

// ListAiProviders lists the gateway's providers.
func (s *AiProviderService) ListAiProviders(ctx context.Context, _ *systemv1.ListAiProvidersRequest) (*systemv1.ListAiProvidersResponse, error) {
	id, g, err := s.gateway(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := g.ListAiProviders(ctx, &pluginv1.ListAiProvidersRequest{})
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	return systemv1.ListAiProvidersResponse_builder{PluginId: &id, Providers: resp.GetProviders()}.Build(), nil
}

// ListAiModels lists the gateway's models.
func (s *AiProviderService) ListAiModels(ctx context.Context, req *systemv1.ListAiModelsRequest) (*systemv1.ListAiModelsResponse, error) {
	_, g, err := s.gateway(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := g.ListAiModels(ctx, pluginv1.ListAiModelsRequest_builder{
		Provider: new(req.GetProvider()), Available: new(req.GetAvailable()),
	}.Build())
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	return systemv1.ListAiModelsResponse_builder{Models: resp.GetModels()}.Build(), nil
}

// SetAiApiKey stores or removes a provider's API key.
func (s *AiProviderService) SetAiApiKey(ctx context.Context, req *systemv1.SetAiApiKeyRequest) (*systemv1.SetAiApiKeyResponse, error) {
	_, g, err := s.gateway(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := g.SetAiApiKey(ctx, pluginv1.SetAiApiKeyRequest_builder{
		Provider: new(req.GetProvider()), Key: new(req.GetKey()),
	}.Build()); err != nil {
		return nil, gatewayError(ctx, err)
	}
	title := "%s set the API key of %s"
	if req.GetKey() == "" {
		title = "%s removed the API key of %s"
	}
	s.record(ctx, "ai.credential_changed", title, req.GetProvider())
	return &systemv1.SetAiApiKeyResponse{}, nil
}

// StartAiLogin starts a provider's OAuth login.
func (s *AiProviderService) StartAiLogin(ctx context.Context, req *systemv1.StartAiLoginRequest) (*systemv1.StartAiLoginResponse, error) {
	_, g, err := s.gateway(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := g.StartAiLogin(ctx, pluginv1.StartAiLoginRequest_builder{Provider: new(req.GetProvider())}.Build())
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	s.record(ctx, "ai.login_started", "%s started logging in to %s", req.GetProvider())
	return systemv1.StartAiLoginResponse_builder{LoginId: new(resp.GetLoginId())}.Build(), nil
}

// GetAiLogin returns a login's state.
func (s *AiProviderService) GetAiLogin(ctx context.Context, req *systemv1.GetAiLoginRequest) (*systemv1.GetAiLoginResponse, error) {
	_, g, err := s.gateway(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := g.GetAiLogin(ctx, pluginv1.GetAiLoginRequest_builder{LoginId: new(req.GetLoginId())}.Build())
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	return systemv1.GetAiLoginResponse_builder{
		State: resp.GetState().Enum(), Events: resp.GetEvents(), Prompt: resp.GetPrompt(), Error: new(resp.GetError()),
	}.Build(), nil
}

// WatchAiLogin streams a login's state from the gateway until it ends.
func (s *AiProviderService) WatchAiLogin(ctx context.Context, req *systemv1.WatchAiLoginRequest, stream *connect.ServerStream[systemv1.WatchAiLoginResponse]) error {
	_, g, err := s.gateway(ctx)
	if err != nil {
		return err
	}
	watch, err := g.WatchAiLogin(ctx, pluginv1.WatchAiLoginRequest_builder{LoginId: new(req.GetLoginId())}.Build())
	if err != nil {
		return gatewayError(ctx, err)
	}
	defer func() { _ = watch.Close() }()
	for watch.Receive() {
		m := watch.Msg()
		if err := stream.Send(systemv1.WatchAiLoginResponse_builder{
			State: m.GetState().Enum(), Events: m.GetEvents(), Prompt: m.GetPrompt(), Error: new(m.GetError()),
		}.Build()); err != nil {
			return err
		}
	}
	if err := watch.Err(); err != nil {
		return gatewayError(ctx, err)
	}
	return nil
}

// AnswerAiLogin answers a login's prompt.
func (s *AiProviderService) AnswerAiLogin(ctx context.Context, req *systemv1.AnswerAiLoginRequest) (*systemv1.AnswerAiLoginResponse, error) {
	_, g, err := s.gateway(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := g.AnswerAiLogin(ctx, pluginv1.AnswerAiLoginRequest_builder{
		LoginId: new(req.GetLoginId()), PromptId: new(req.GetPromptId()), Value: new(req.GetValue()),
	}.Build()); err != nil {
		return nil, gatewayError(ctx, err)
	}
	return &systemv1.AnswerAiLoginResponse{}, nil
}

// CancelAiLogin abandons a login.
func (s *AiProviderService) CancelAiLogin(ctx context.Context, req *systemv1.CancelAiLoginRequest) (*systemv1.CancelAiLoginResponse, error) {
	_, g, err := s.gateway(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := g.CancelAiLogin(ctx, pluginv1.CancelAiLoginRequest_builder{LoginId: new(req.GetLoginId())}.Build()); err != nil {
		return nil, gatewayError(ctx, err)
	}
	return &systemv1.CancelAiLoginResponse{}, nil
}

// LogoutAi removes a provider's stored credential.
func (s *AiProviderService) LogoutAi(ctx context.Context, req *systemv1.LogoutAiRequest) (*systemv1.LogoutAiResponse, error) {
	_, g, err := s.gateway(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := g.LogoutAi(ctx, pluginv1.LogoutAiRequest_builder{Provider: new(req.GetProvider())}.Build()); err != nil {
		return nil, gatewayError(ctx, err)
	}
	s.record(ctx, "ai.credential_changed", "%s logged out of %s", req.GetProvider())
	return &systemv1.LogoutAiResponse{}, nil
}

// caller checks the caller may keep conversations.
func (s *AiConversationService) caller(ctx context.Context) (auth.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	set := currentSettings(s.Settings)
	if !set.AI.AllowsConversations(&p.User) {
		return p, connect.NewError(connect.CodePermissionDenied, errors.New("conversations are for administrators only"))
	}
	return p, nil
}

// sessions checks the caller may keep conversations and returns them with
// the plugin keeping conversations.
func (s *AiConversationService) sessions(ctx context.Context) (auth.Principal, pluginv1connect.AgentSessionServiceClient, error) {
	p, err := s.caller(ctx)
	if err != nil {
		return p, nil, err
	}
	if s.Sessions != nil {
		if c, ok := s.Sessions(); ok {
			return p, c, nil
		}
	}
	return p, nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no AI agent session plugin is running"))
}

// sees tells whether p sees a conversation: an administrator every one,
// those without an owner from before owners were recorded among them;
// another user only their own, not their conversations with the assistant,
// which the assistant shows.
func sees(p *auth.Principal, sess *pluginv1.AgentSession) bool {
	if p.User.IsAdmin() {
		return true
	}
	return sess.GetOwner() != "" && sess.GetOwner() == p.User.ID.String() && sess.GetLabels()[assistantLabel] == ""
}

// conversation returns a conversation with its messages, if the caller
// sees it; NOT_FOUND otherwise, as for one that does not exist.
func (s *AiConversationService) conversation(ctx context.Context, id string) (pluginv1connect.AgentSessionServiceClient, *pluginv1.GetAgentSessionResponse, error) {
	p, c, err := s.sessions(ctx)
	if err != nil {
		return nil, nil, err
	}
	resp, err := c.GetAgentSession(ctx, pluginv1.GetAgentSessionRequest_builder{Id: &id}.Build())
	if err != nil {
		return nil, nil, gatewayError(ctx, err)
	}
	if !sees(&p, resp.GetSession()) {
		return nil, nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("conversation %s not found", id))
	}
	return c, resp, nil
}

func conversationSettingsToProto(a *core.AISettings) *systemv1.AiConversationSettings {
	return systemv1.AiConversationSettings_builder{AllUsers: &a.ConversationsForAll}.Build()
}

// GetAiConversationSettings returns who may keep conversations, and
// whether the caller may.
func (s *AiConversationService) GetAiConversationSettings(ctx context.Context, _ *systemv1.GetAiConversationSettingsRequest) (*systemv1.GetAiConversationSettingsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	set := currentSettings(s.Settings)
	return systemv1.GetAiConversationSettingsResponse_builder{
		Settings: conversationSettingsToProto(&set.AI), Allowed: new(set.AI.AllowsConversations(&p.User)),
	}.Build(), nil
}

// UpdateAiConversationSettings changes who may keep conversations.
func (s *AiConversationService) UpdateAiConversationSettings(ctx context.Context, req *systemv1.UpdateAiConversationSettingsRequest) (*systemv1.UpdateAiConversationSettingsResponse, error) {
	set, err := changeSettings(ctx, s.Settings, s.Activity, "the AI conversation settings", func(cur *core.ServerSettings) error {
		cur.AI.ConversationsForAll = req.GetSettings().GetAllUsers()
		return nil
	})
	if err != nil {
		return nil, err
	}
	return systemv1.UpdateAiConversationSettingsResponse_builder{Settings: conversationSettingsToProto(&set.AI)}.Build(), nil
}

// ListAiConversationModels lists the models of the providers that can be
// used, with their providers' names.
func (s *AiConversationService) ListAiConversationModels(ctx context.Context, _ *systemv1.ListAiConversationModelsRequest) (*systemv1.ListAiConversationModelsResponse, error) {
	if _, err := s.caller(ctx); err != nil {
		return nil, err
	}
	var g pluginv1connect.AiGatewayServiceClient
	if s.Gateway != nil {
		_, g, _ = s.Gateway()
	}
	if g == nil {
		return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no AI gateway plugin is running"))
	}
	models, err := g.ListAiModels(ctx, pluginv1.ListAiModelsRequest_builder{Available: new(true)}.Build())
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	providers, err := g.ListAiProviders(ctx, &pluginv1.ListAiProvidersRequest{})
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	names := make(map[string]string, len(providers.GetProviders()))
	for _, pr := range providers.GetProviders() {
		names[pr.GetId()] = pr.GetName()
	}
	return systemv1.ListAiConversationModelsResponse_builder{Models: models.GetModels(), ProviderNames: names}.Build(), nil
}

// CreateAiConversation starts a conversation with a model.
func (s *AiConversationService) CreateAiConversation(ctx context.Context, req *systemv1.CreateAiConversationRequest) (*systemv1.CreateAiConversationResponse, error) {
	p, c, err := s.sessions(ctx)
	if err != nil {
		return nil, err
	}
	// The user who starts it owns it; it keeps no memories.
	resp, err := c.CreateAgentSession(ctx, pluginv1.CreateAgentSessionRequest_builder{
		Provider: new(req.GetProvider()), Model: new(req.GetModel()), Instructions: new(req.GetInstructions()), Title: new(req.GetTitle()),
		Owner: new(p.User.ID.String()),
	}.Build())
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	return systemv1.CreateAiConversationResponse_builder{Conversation: resp.GetSession()}.Build(), nil
}

// ListAiConversations lists the conversations the caller sees, newest
// first.
func (s *AiConversationService) ListAiConversations(ctx context.Context, _ *systemv1.ListAiConversationsRequest) (*systemv1.ListAiConversationsResponse, error) {
	p, c, err := s.sessions(ctx)
	if err != nil {
		return nil, err
	}
	req := &pluginv1.ListAgentSessionsRequest{}
	if !p.User.IsAdmin() {
		req.SetOwner(p.User.ID.String())
	}
	resp, err := c.ListAgentSessions(ctx, req)
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	seen := slices.DeleteFunc(resp.GetSessions(), func(sess *pluginv1.AgentSession) bool { return !sees(&p, sess) })
	return systemv1.ListAiConversationsResponse_builder{Conversations: seen}.Build(), nil
}

// GetAiConversation returns a conversation with its messages.
func (s *AiConversationService) GetAiConversation(ctx context.Context, req *systemv1.GetAiConversationRequest) (*systemv1.GetAiConversationResponse, error) {
	_, resp, err := s.conversation(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	return systemv1.GetAiConversationResponse_builder{Conversation: resp.GetSession(), Messages: resp.GetMessages()}.Build(), nil
}

// SendAiMessage sends a message and streams the answer from the plugin.
func (s *AiConversationService) SendAiMessage(ctx context.Context, req *systemv1.SendAiMessageRequest, stream *connect.ServerStream[systemv1.SendAiMessageResponse]) error {
	c, _, err := s.conversation(ctx, req.GetConversationId())
	if err != nil {
		return err
	}
	answer, err := c.StreamAgentMessage(ctx, pluginv1.StreamAgentMessageRequest_builder{
		Message: pluginv1.SendAgentMessageRequest_builder{
			SessionId: new(req.GetConversationId()), Text: new(req.GetText()), RequestId: new(req.GetRequestId()),
		}.Build(),
	}.Build())
	if err != nil {
		return gatewayError(ctx, err)
	}
	defer func() { _ = answer.Close() }()
	for answer.Receive() {
		m := answer.Msg()
		out := &systemv1.SendAiMessageResponse{}
		if m.HasDone() {
			out.SetDone(m.GetDone())
		} else {
			out.SetText(m.GetText())
		}
		if err := stream.Send(out); err != nil {
			return err
		}
	}
	if err := answer.Err(); err != nil {
		return gatewayError(ctx, err)
	}
	return nil
}

// DeleteAiConversation deletes a conversation.
func (s *AiConversationService) DeleteAiConversation(ctx context.Context, req *systemv1.DeleteAiConversationRequest) (*systemv1.DeleteAiConversationResponse, error) {
	c, _, err := s.conversation(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if _, err := c.DeleteAgentSession(ctx, pluginv1.DeleteAgentSessionRequest_builder{Id: new(req.GetId())}.Build()); err != nil {
		return nil, gatewayError(ctx, err)
	}
	return &systemv1.DeleteAiConversationResponse{}, nil
}

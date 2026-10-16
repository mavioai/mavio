package rpc

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
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
// conversations with models.
type AiConversationService struct {
	// Sessions finds the plugin keeping conversations; nil finds none.
	Sessions AgentSessions
}

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

// sessions checks the caller is an administrator and returns the plugin
// keeping conversations.
func (s *AiConversationService) sessions(ctx context.Context) (pluginv1connect.AgentSessionServiceClient, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.Sessions != nil {
		if c, ok := s.Sessions(); ok {
			return c, nil
		}
	}
	return nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no AI agent session plugin is running"))
}

// CreateAiConversation starts a conversation with a model.
func (s *AiConversationService) CreateAiConversation(ctx context.Context, req *systemv1.CreateAiConversationRequest) (*systemv1.CreateAiConversationResponse, error) {
	c, err := s.sessions(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.CreateAgentSession(ctx, pluginv1.CreateAgentSessionRequest_builder{
		Provider: new(req.GetProvider()), Model: new(req.GetModel()), Instructions: new(req.GetInstructions()), Title: new(req.GetTitle()),
	}.Build())
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	return systemv1.CreateAiConversationResponse_builder{Conversation: resp.GetSession()}.Build(), nil
}

// ListAiConversations lists the conversations, newest first.
func (s *AiConversationService) ListAiConversations(ctx context.Context, _ *systemv1.ListAiConversationsRequest) (*systemv1.ListAiConversationsResponse, error) {
	c, err := s.sessions(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.ListAgentSessions(ctx, &pluginv1.ListAgentSessionsRequest{})
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	return systemv1.ListAiConversationsResponse_builder{Conversations: resp.GetSessions()}.Build(), nil
}

// GetAiConversation returns a conversation with its messages.
func (s *AiConversationService) GetAiConversation(ctx context.Context, req *systemv1.GetAiConversationRequest) (*systemv1.GetAiConversationResponse, error) {
	c, err := s.sessions(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := c.GetAgentSession(ctx, pluginv1.GetAgentSessionRequest_builder{Id: new(req.GetId())}.Build())
	if err != nil {
		return nil, gatewayError(ctx, err)
	}
	return systemv1.GetAiConversationResponse_builder{Conversation: resp.GetSession(), Messages: resp.GetMessages()}.Build(), nil
}

// SendAiMessage sends a message and streams the answer from the plugin.
func (s *AiConversationService) SendAiMessage(ctx context.Context, req *systemv1.SendAiMessageRequest, stream *connect.ServerStream[systemv1.SendAiMessageResponse]) error {
	c, err := s.sessions(ctx)
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
	c, err := s.sessions(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := c.DeleteAgentSession(ctx, pluginv1.DeleteAgentSessionRequest_builder{Id: new(req.GetId())}.Build()); err != nil {
		return nil, gatewayError(ctx, err)
	}
	return &systemv1.DeleteAiConversationResponse{}, nil
}

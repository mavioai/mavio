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
)

// AIGateway returns the started AI gateway plugin's ID and client, or
// false when none is started.
type AIGateway func() (string, pluginv1connect.AiGatewayServiceClient, bool)

// AiService implements systemv1connect.AiServiceHandler over the AI gateway
// plugin.
type AiService struct {
	// Gateway finds the AI gateway plugin; nil finds none.
	Gateway AIGateway
	// Activity records credential changes; nil records none.
	Activity *activity.Log
}

// gateway checks the caller is an administrator and returns the gateway.
func (s *AiService) gateway(ctx context.Context) (string, pluginv1connect.AiGatewayServiceClient, error) {
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

func (s *AiService) record(ctx context.Context, kind, title, provider string) {
	p, _ := principal(ctx)
	s.Activity.Record(ctx, core.Activity{
		Type: kind, UserID: p.User.ID, Title: fmt.Sprintf(title, p.User.Name, provider),
		Attributes: map[string]string{"provider": provider},
	})
}

// ListAiProviders lists the gateway's providers.
func (s *AiService) ListAiProviders(ctx context.Context, _ *systemv1.ListAiProvidersRequest) (*systemv1.ListAiProvidersResponse, error) {
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
func (s *AiService) ListAiModels(ctx context.Context, req *systemv1.ListAiModelsRequest) (*systemv1.ListAiModelsResponse, error) {
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
func (s *AiService) SetAiApiKey(ctx context.Context, req *systemv1.SetAiApiKeyRequest) (*systemv1.SetAiApiKeyResponse, error) {
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
func (s *AiService) StartAiLogin(ctx context.Context, req *systemv1.StartAiLoginRequest) (*systemv1.StartAiLoginResponse, error) {
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
func (s *AiService) GetAiLogin(ctx context.Context, req *systemv1.GetAiLoginRequest) (*systemv1.GetAiLoginResponse, error) {
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

// AnswerAiLogin answers a login's prompt.
func (s *AiService) AnswerAiLogin(ctx context.Context, req *systemv1.AnswerAiLoginRequest) (*systemv1.AnswerAiLoginResponse, error) {
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
func (s *AiService) CancelAiLogin(ctx context.Context, req *systemv1.CancelAiLoginRequest) (*systemv1.CancelAiLoginResponse, error) {
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
func (s *AiService) LogoutAi(ctx context.Context, req *systemv1.LogoutAiRequest) (*systemv1.LogoutAiResponse, error) {
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

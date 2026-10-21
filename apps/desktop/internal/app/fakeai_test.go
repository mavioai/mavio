package app

import (
	"context"
	"slices"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// fakeAI is a fake server's AI gateway: Anthropic with an API key, OpenAI
// signed in with an account, and OpenRouter without a credential; and the
// login of an account, which shows its page and asks for a code.
type fakeAI struct {
	systemv1connect.UnimplementedAiProviderServiceHandler

	mu        sync.Mutex
	providers []*pluginv1.AiProvider
	keys      map[string]string
	// answered is closed once the login's question is answered.
	answered chan struct{}
	answer   string
}

func newFakeAI() *fakeAI {
	return &fakeAI{
		providers: []*pluginv1.AiProvider{
			pluginv1.AiProvider_builder{
				Id: new("anthropic"), Name: new("Anthropic"), ApiKey: new(true), Oauth: new(true), OauthLabel: new("Claude Pro/Max"),
				Subscription: new(true), Credential: new(pluginv1.AiCredential_AI_CREDENTIAL_API_KEY), Available: new(true),
			}.Build(),
			pluginv1.AiProvider_builder{
				Id: new("openai-codex"), Name: new("ChatGPT"), Oauth: new(true), OauthLabel: new("ChatGPT Plus/Pro"),
				Credential: new(pluginv1.AiCredential_AI_CREDENTIAL_OAUTH), Available: new(true),
			}.Build(),
			pluginv1.AiProvider_builder{Id: new("openrouter"), Name: new("OpenRouter"), ApiKey: new(true)}.Build(),
		},
		keys:     map[string]string{"anthropic": "sk-ant"},
		answered: make(chan struct{}),
	}
}

var fakeModels = []*pluginv1.AiModel{
	pluginv1.AiModel_builder{
		Provider: new("anthropic"), Id: new("claude-sonnet-5-5"), Name: new("Claude Sonnet 5.5"), Reasoning: new(true),
		ContextWindow: new(int64(200000)), Cost: pluginv1.AiModelCost_builder{Input: new(3.0), Output: new(15.0)}.Build(),
	}.Build(),
	pluginv1.AiModel_builder{Provider: new("anthropic"), Id: new("claude-haiku-5-5"), Name: new("Claude Haiku 5.5")}.Build(),
	pluginv1.AiModel_builder{Provider: new("openai-codex"), Id: new("gpt-6"), Name: new("GPT-6"), Images: new(true)}.Build(),
	pluginv1.AiModel_builder{Provider: new("openrouter"), Id: new("qwen/qwen4"), Name: new("Qwen 4")}.Build(),
}

func (f *fakeAI) ListAiProviders(context.Context, *systemv1.ListAiProvidersRequest) (*systemv1.ListAiProvidersResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pluginv1.AiProvider
	for _, p := range f.providers {
		out = append(out, proto.CloneOf(p))
	}
	return systemv1.ListAiProvidersResponse_builder{Providers: out}.Build(), nil
}

func (f *fakeAI) available(provider string) bool {
	i := slices.IndexFunc(f.providers, func(p *pluginv1.AiProvider) bool { return p.GetId() == provider })
	return i >= 0 && f.providers[i].GetAvailable()
}

func (f *fakeAI) ListAiModels(_ context.Context, r *systemv1.ListAiModelsRequest) (*systemv1.ListAiModelsResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*pluginv1.AiModel
	for _, m := range fakeModels {
		if (r.GetProvider() == "" || m.GetProvider() == r.GetProvider()) && (!r.GetAvailable() || f.available(m.GetProvider())) {
			out = append(out, m)
		}
	}
	return systemv1.ListAiModelsResponse_builder{Models: out}.Build(), nil
}

func (f *fakeAI) provider(id string) *pluginv1.AiProvider {
	i := slices.IndexFunc(f.providers, func(p *pluginv1.AiProvider) bool { return p.GetId() == id })
	if i < 0 {
		return nil
	}
	return f.providers[i]
}

func (f *fakeAI) SetAiApiKey(_ context.Context, r *systemv1.SetAiApiKeyRequest) (*systemv1.SetAiApiKeyResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := f.provider(r.GetProvider())
	if p == nil {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	f.keys[p.GetId()] = r.GetKey()
	p.SetCredential(pluginv1.AiCredential_AI_CREDENTIAL_API_KEY)
	p.SetAvailable(true)
	return &systemv1.SetAiApiKeyResponse{}, nil
}

func (f *fakeAI) LogoutAi(_ context.Context, r *systemv1.LogoutAiRequest) (*systemv1.LogoutAiResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p := f.provider(r.GetProvider()); p != nil {
		delete(f.keys, p.GetId())
		p.SetCredential(pluginv1.AiCredential_AI_CREDENTIAL_UNSPECIFIED)
		p.SetAvailable(false)
	}
	return &systemv1.LogoutAiResponse{}, nil
}

func (f *fakeAI) StartAiLogin(context.Context, *systemv1.StartAiLoginRequest) (*systemv1.StartAiLoginResponse, error) {
	return systemv1.StartAiLoginResponse_builder{LoginId: new("login-1")}.Build(), nil
}

// WatchAiLogin shows the login's page and asks for its code, and once
// answered succeeds.
func (f *fakeAI) WatchAiLogin(ctx context.Context, _ *systemv1.WatchAiLoginRequest, stream *connect.ServerStream[systemv1.WatchAiLoginResponse]) error {
	events := []*pluginv1.AiLoginEvent{pluginv1.AiLoginEvent_builder{Url: pluginv1.AiLoginUrl_builder{
		Url: new("https://auth.example.test/authorize?client_id=mavio"), Instructions: new("Sign in on the page."),
	}.Build()}.Build()}
	err := stream.Send(systemv1.WatchAiLoginResponse_builder{
		State: new(pluginv1.AiLoginState_AI_LOGIN_STATE_RUNNING), Events: events,
		Prompt: pluginv1.AiLoginPrompt_builder{Id: new("code"), Kind: new(pluginv1.AiLoginPromptKind_AI_LOGIN_PROMPT_KIND_MANUAL_CODE), Message: new("Paste the code"), Placeholder: new("The code")}.Build(),
	}.Build())
	if err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-f.answered:
	case <-time.After(30 * time.Second):
		return connect.NewError(connect.CodeDeadlineExceeded, nil)
	}
	return stream.Send(systemv1.WatchAiLoginResponse_builder{State: new(pluginv1.AiLoginState_AI_LOGIN_STATE_SUCCEEDED), Events: events}.Build())
}

func (f *fakeAI) AnswerAiLogin(_ context.Context, r *systemv1.AnswerAiLoginRequest) (*systemv1.AnswerAiLoginResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.answer = r.GetValue()
	close(f.answered)
	return &systemv1.AnswerAiLoginResponse{}, nil
}

// fakeAssistantSettings are a fake server's assistant settings.
func fakeAssistantSettings() *assistantv1.AssistantSettings {
	return assistantv1.AssistantSettings_builder{
		Enabled: new(true), Model: new("ai/anthropic/claude-sonnet-5-5"), Memory: new(true),
		Speech:       []*assistantv1.AssistantSpeech{assistantv1.AssistantSpeech_builder{Language: new("en"), Recognizer: new("pipecat/whisper")}.Build()},
		LongestPause: durationpb.New(3 * time.Second),
	}.Build()
}

func (a *fakeAssistant) GetAssistantSettings(context.Context, *assistantv1.GetAssistantSettingsRequest) (*assistantv1.GetAssistantSettingsResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.settings == nil {
		a.settings = fakeAssistantSettings()
	}
	return assistantv1.GetAssistantSettingsResponse_builder{
		Settings:    proto.CloneOf(a.settings),
		Recognizers: []*assistantv1.VoiceEngine{assistantv1.VoiceEngine_builder{Id: new("pipecat/whisper"), Name: new("Whisper"), Ready: new(true)}.Build()},
		Voices:      []*assistantv1.VoiceEngine{assistantv1.VoiceEngine_builder{Id: new("pipecat/kokoro"), Name: new("Kokoro"), Ready: new(true)}.Build()},
	}.Build(), nil
}

func (a *fakeAssistant) UpdateAssistantSettings(_ context.Context, r *assistantv1.UpdateAssistantSettingsRequest) (*assistantv1.UpdateAssistantSettingsResponse, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings = proto.CloneOf(r.GetSettings())
	return assistantv1.UpdateAssistantSettingsResponse_builder{Settings: proto.CloneOf(a.settings)}.Build(), nil
}

package server_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"slices"
	"sync"
	"testing"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/server"
	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

var agentSource = filepath.Join("..", "..", "..", "..", "plugins", "agent")

// agentBuild compiles the agent plugin with Bun once per test binary.
var agentBuild = sync.OnceValues(func() (string, error) {
	out := filepath.Join(smokeRoot, "agent")
	build := exec.Command("bun", "build", "--compile", "src/main.ts", "--outfile", out)
	build.Dir = agentSource
	if msg, err := build.CombinedOutput(); err != nil {
		return "", fmt.Errorf("build the agent plugin: %w\n%s", err, msg)
	}
	if filepath.Separator == '\\' {
		out += ".exe"
	}
	return out, nil
})

// installAgent installs the agent plugin into the plugin folder root,
// skipping the test without Bun.
func installAgent(t *testing.T, root string) {
	t.Helper()
	if _, err := exec.LookPath("bun"); err != nil {
		t.Skip("integration test: bun not found on PATH")
	}
	installProcessPlugin(t, root, "agent", agentSource, agentBuild)
}

// fakeOpenAI serves an OpenAI-compatible chat completions API whose answer
// answer gives for a request's messages; it records the requests' bodies.
func fakeOpenAI(t *testing.T, answer func(messages []string) string) (*httptest.Server, func() []string) {
	t.Helper()
	var (
		mu     sync.Mutex
		bodies []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		bodies = append(bodies, string(body))
		mu.Unlock()
		var req struct {
			Messages []struct {
				Content json.RawMessage `json:"content"`
			} `json:"messages"`
		}
		_ = json.Unmarshal(body, &req)
		var messages []string
		for _, m := range req.Messages {
			var s string
			if json.Unmarshal(m.Content, &s) != nil {
				var parts []struct {
					Text string `json:"text"`
				}
				_ = json.Unmarshal(m.Content, &parts)
				for _, p := range parts {
					s += p.Text
				}
			}
			messages = append(messages, s)
		}
		text := answer(messages)
		if text == "" {
			http.Error(w, `{"error":{"message":"rate limited"}}`, http.StatusTooManyRequests)
			return
		}
		content, _ := json.Marshal(text)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprintf(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{\"role\":\"assistant\",\"content\":%s}}]}\n\n", content)
		fmt.Fprint(w, "data: {\"id\":\"1\",\"object\":\"chat.completion.chunk\",\"model\":\"m\",\"choices\":[{\"index\":0,\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":5,\"total_tokens\":15}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(bodies)
	}
}

// agentConfig configures the agent plugin with a custom provider "local"
// served at base.
func agentConfig(base string) string {
	return fmt.Sprintf(`{"providers":[{"id":"local","name":"Local","base_url":%q,"models":[{"id":"translator","name":"Translator"}]}]}`, base+"/v1")
}

// TestAI administers AI providers through the agent plugin: a provider
// served by an OpenAI-compatible API is configured, listed with its
// models, given a key and logged out of; only administrators may.
func TestAI(t *testing.T) {
	ctx := t.Context()
	pluginDir := t.TempDir()
	installAgent(t, pluginDir)
	openai, _ := fakeOpenAI(t, func([]string) string { return "ok" })
	srv := start(t, server.Config{
		Database: "sqlite:" + filepath.Join(t.TempDir(), "mavio.db"), TranscodeDir: t.TempDir(), CacheDir: t.TempDir(),
		MetadataDir: filepath.ToSlash(t.TempDir()), PluginDir: pluginDir, PluginDataDir: t.TempDir(),
	})
	first, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).CreateFirstUser(ctx, authv1.CreateFirstUserRequest_builder{
		Name: new("admin"), Password: new("secret"), Device: device("e2e"),
	}.Build())
	if err != nil {
		t.Fatalf("CreateFirstUser: %v", err)
	}
	token := withToken(first.GetAccessToken())
	system := systemv1connect.NewSystemServiceClient(http.DefaultClient, srv.url, token)
	ai := systemv1connect.NewAiServiceClient(http.DefaultClient, srv.url, token)
	waitFor(t, "the agent plugin started", func() bool {
		_, err := ai.ListAiProviders(ctx, &systemv1.ListAiProvidersRequest{})
		return err == nil
	})
	if _, err := system.SetPluginConfig(ctx, systemv1.SetPluginConfigRequest_builder{
		PluginId: new("dev.mavio.agent"), ConfigJson: new(agentConfig(openai.URL)),
	}.Build()); err != nil {
		t.Fatalf("SetPluginConfig: %v", err)
	}

	list, err := ai.ListAiProviders(ctx, &systemv1.ListAiProvidersRequest{})
	if err != nil {
		t.Fatalf("ListAiProviders: %v", err)
	}
	if list.GetPluginId() != "dev.mavio.agent" {
		t.Errorf("plugin = %q, want dev.mavio.agent", list.GetPluginId())
	}
	provider := func(id string) *pluginv1.AiProvider {
		list, err := ai.ListAiProviders(ctx, &systemv1.ListAiProvidersRequest{})
		if err != nil {
			t.Fatalf("ListAiProviders: %v", err)
		}
		for _, p := range list.GetProviders() {
			if p.GetId() == id {
				return p
			}
		}
		t.Fatalf("no provider %s among %d", id, len(list.GetProviders()))
		return nil
	}
	if p := provider("local"); !p.GetCustom() || !p.GetAvailable() || p.GetCredential() != pluginv1.AiCredential_AI_CREDENTIAL_UNSPECIFIED {
		t.Errorf("local = custom %v, available %v, credential %v, want true, true, unspecified", p.GetCustom(), p.GetAvailable(), p.GetCredential())
	}
	if p := provider("anthropic"); !p.GetOauth() || !p.GetSubscription() {
		t.Errorf("anthropic = oauth %v, subscription %v, want true, true", p.GetOauth(), p.GetSubscription())
	}
	models, err := ai.ListAiModels(ctx, systemv1.ListAiModelsRequest_builder{Provider: new("local")}.Build())
	if err != nil {
		t.Fatalf("ListAiModels: %v", err)
	}
	if len(models.GetModels()) != 1 || models.GetModels()[0].GetId() != "translator" {
		t.Errorf("models = %v, want translator", models.GetModels())
	}

	if _, err := ai.SetAiApiKey(ctx, systemv1.SetAiApiKeyRequest_builder{Provider: new("local"), Key: new("sk-test")}.Build()); err != nil {
		t.Fatalf("SetAiApiKey: %v", err)
	}
	if p := provider("local"); p.GetCredential() != pluginv1.AiCredential_AI_CREDENTIAL_API_KEY {
		t.Errorf("credential after SetAiApiKey = %v, want API key", p.GetCredential())
	}
	if _, err := ai.LogoutAi(ctx, systemv1.LogoutAiRequest_builder{Provider: new("local")}.Build()); err != nil {
		t.Fatalf("LogoutAi: %v", err)
	}
	if p := provider("local"); p.GetCredential() != pluginv1.AiCredential_AI_CREDENTIAL_UNSPECIFIED {
		t.Errorf("credential after LogoutAi = %v, want none", p.GetCredential())
	}
	if _, err := ai.StartAiLogin(ctx, systemv1.StartAiLoginRequest_builder{Provider: new("local")}.Build()); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("StartAiLogin(local) = %v, want failed precondition", err)
	}
	if _, err := ai.ListAiModels(ctx, systemv1.ListAiModelsRequest_builder{Provider: new("nowhere")}.Build()); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("ListAiModels(nowhere) = %v, want not found", err)
	}

	// An OAuth login runs in the plugin and asks how to sign in.
	login, err := ai.StartAiLogin(ctx, systemv1.StartAiLoginRequest_builder{Provider: new("anthropic")}.Build())
	if err != nil {
		t.Fatalf("StartAiLogin(anthropic): %v", err)
	}
	watch, err := ai.WatchAiLogin(ctx, systemv1.WatchAiLoginRequest_builder{LoginId: new(login.GetLoginId())}.Build())
	if err != nil {
		t.Fatalf("WatchAiLogin: %v", err)
	}
	defer func() { _ = watch.Close() }()
	var last *systemv1.WatchAiLoginResponse
	for last.GetPrompt() == nil && watch.Receive() {
		last = watch.Msg()
	}
	if last.GetPrompt() == nil {
		t.Fatalf("WatchAiLogin ended without a prompt: %v", watch.Err())
	}
	if _, err := ai.CancelAiLogin(ctx, systemv1.CancelAiLoginRequest_builder{LoginId: new(login.GetLoginId())}.Build()); err != nil {
		t.Fatalf("CancelAiLogin: %v", err)
	}
	for watch.Receive() {
		last = watch.Msg()
	}
	if err := watch.Err(); err != nil || last.GetState() != pluginv1.AiLoginState_AI_LOGIN_STATE_CANCELED {
		t.Errorf("WatchAiLogin ended with %v, %v; want canceled", last.GetState(), err)
	}
	if state, err := ai.GetAiLogin(ctx, systemv1.GetAiLoginRequest_builder{LoginId: new(login.GetLoginId())}.Build()); err != nil ||
		state.GetState() != pluginv1.AiLoginState_AI_LOGIN_STATE_CANCELED {
		t.Errorf("GetAiLogin = %v, %v; want canceled", state.GetState(), err)
	}

	// Others may not.
	users := userv1connect.NewUserServiceClient(http.DefaultClient, srv.url, token)
	if _, err := users.CreateUser(ctx, userv1.CreateUserRequest_builder{Name: new("guest"), Password: new("guest")}.Build()); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	guest, err := authv1connect.NewAuthServiceClient(http.DefaultClient, srv.url).Login(ctx, authv1.LoginRequest_builder{
		Name: new("guest"), Password: new("guest"), Device: device("guest"),
	}.Build())
	if err != nil {
		t.Fatalf("Login: %v", err)
	}
	asGuest := systemv1connect.NewAiServiceClient(http.DefaultClient, srv.url, withToken(guest.GetAccessToken()))
	if _, err := asGuest.ListAiProviders(ctx, &systemv1.ListAiProvidersRequest{}); connect.CodeOf(err) != connect.CodePermissionDenied {
		t.Errorf("ListAiProviders as a guest = %v, want permission denied", err)
	}
}

package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestAI(t *testing.T) {
	h := newHarness(t, true, 1280, 1200)
	h.tt.Key(ui.Cmd, ui.KeyComma)
	h.settle()
	open := func(label string) {
		t.Helper()
		h.click(label)
		h.settle()
	}
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}

	// The assistant's settings over the providers added, with their
	// credentials and models, and the others to add.
	open("AI")
	want("Assistant", "Turn On the Assistant", "Claude Sonnet 5.5", "Anthropic", "Who Uses AI", "Everyone",
		"Speech", "Whisper", "First ready", "Add Language", "Longest Pause (Seconds)",
		"Providers", "API key stored · 2 models", "ChatGPT", "Signed in with ChatGPT Plus/Pro · 1 model", "Add Provider", "Choose a provider")
	h.save(shots, "settings-ai")

	// The assistant's settings saved.
	open("Remembers what users tell it about themselves")
	open("Save")
	want("Saved.")
	if got := h.core.assistant.settings.GetMemory(); got {
		t.Errorf("memory = %v, want = false", got)
	}

	// Adding OpenRouter with an API key.
	open("Choose a provider")
	want("OpenRouter")
	h.save(shots, "settings-ai-add")
	open("OpenRouter")
	want("No credential", "API key", "The key is stored on the server and never shown again.", "Qwen 4",
		"The models can be used once the provider has a credential.")
	// The key typed: the field is labeled as its row and group are.
	h.app.settingsState.admin.ai.apiKey = "sk-or"
	h.settle()
	open("Save")
	want("API key stored", "Remove API key")
	if got := h.core.ai.keys["openrouter"]; got != "sk-or" {
		t.Errorf("OpenRouter's key = %q, want = sk-or", got)
	}

	// Anthropic's page: its models, and signing in with its account,
	// answered with a code.
	h.app.router.Push(aiPath("anthropic"))
	h.settle()
	want("Claude Sonnet 5.5", "claude-sonnet-5-5 · 200K context · Reasoning · $3.00 / $15.00 per 1M tokens",
		"Sign in with Claude Pro/Max", "This signs in with a subscription")
	open("Sign In")
	want("Sign in on the page.", "Open Sign-In Page", "https://auth.example.test/authorize?client_id=mavio", "Paste the code",
		"Waiting for the sign-in to finish…")
	h.save(shots, "settings-ai-login")
	// The field under the question.
	r, _ := h.tt.Find("Paste the code")
	h.tt.ClickAt(r.X+20, r.Y+r.H+24)
	h.settle()
	h.tt.Type("abc123")
	h.settle()
	open("Continue")
	want("Signed in.")
	if got := h.core.ai.answer; got != "abc123" {
		t.Errorf("answer = %q, want = abc123", got)
	}
}

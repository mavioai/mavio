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

func TestAIConversations(t *testing.T) {
	h := newHarness(t, true, 1280, 1200)
	h.app.router.Push("/settings/ai")
	h.settle()
	want := func(texts ...string) {
		t.Helper()
		for _, w := range texts {
			if !h.tt.HasText(w) {
				t.Errorf("no %q; texts = %q", w, h.tt.Texts())
			}
		}
	}

	// At the end of the AI settings: a new one, and those kept, an
	// assistant's saying whose it is and where it started.
	want("New Conversation", "Instructions", "Conversations", "Assistant conversation",
		"Kids · Living Room · Claude Sonnet 5.5 (Anthropic) · 2 hours ago · 12,345 tokens in, 678 out · $0.0421")

	// The assistant's conversation: Kids asked, the context line parted
	// from what was said.
	h.app.router.Push(conversationPath("c1"))
	h.settle()
	want("Assistant conversation", "Kids", "[screen: home]", "Play something funny", "Playing Paddington.", "Delete Conversation")
	h.save(shots, "settings-ai-conversation")

	// A new conversation started with its first message, which is answered.
	h.app.router.Push("/settings/ai")
	h.settle()
	h.app.settingsState.admin.ai.conv.instructions = "Be brief."
	h.app.settingsState.admin.ai.conv.first = "Hello there"
	h.settle()
	h.click("Start")
	h.settle()
	want("Hello there", "You", "You said: Hello there")
	if got := h.core.convs.instructions["c2"]; got != "Be brief." {
		t.Errorf("instructions = %q, want = Be brief.", got)
	}

	// Another message.
	h.app.settingsState.admin.ai.conv.text = "Again"
	h.settle()
	h.click("Send")
	h.settle()
	want("You said: Again")
	h.save(shots, "settings-ai-conversation-new")

	// Deleted, back to the AI settings.
	h.click("Delete Conversation")
	h.settle()
	want("Delete this conversation?")
	h.tt.Key(0, ui.KeyEnter)
	h.settle()
	if h.tt.HasText("Hello there") {
		t.Errorf("the conversation deleted still shows")
	}
	if n := len(h.core.convs.sessions); n != 1 {
		t.Errorf("conversations = %d, want = 1", n)
	}
}

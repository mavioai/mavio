package assistant

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/assistant/intent"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

// fakeItems serves a fixed set of items.
type fakeItems struct {
	libraryv1connect.UnimplementedItemServiceHandler
	items []*libraryv1.Item
}

func (f *fakeItems) ListItems(_ context.Context, req *libraryv1.ListItemsRequest) (*libraryv1.ListItemsResponse, error) {
	var out []*libraryv1.Item
	for _, it := range f.items {
		if ks := req.GetKinds(); len(ks) > 0 && !slices.Contains(ks, it.GetKind()) {
			continue
		}
		if q := strings.ToLower(req.GetSearch()); q != "" && !strings.Contains(strings.ToLower(it.GetName()), q) {
			continue
		}
		out = append(out, it)
	}
	return libraryv1.ListItemsResponse_builder{Items: out, Total: new(int32(len(out)))}.Build(), nil
}

func (f *fakeItems) GetItem(_ context.Context, req *libraryv1.GetItemRequest) (*libraryv1.GetItemResponse, error) {
	for _, it := range f.items {
		if it.GetId() == req.GetId() {
			return libraryv1.GetItemResponse_builder{Item: it}.Build(), nil
		}
	}
	return nil, errors.New("not found")
}

func movie(id, name string) *libraryv1.Item {
	return libraryv1.Item_builder{Id: new(id), Name: new(name), Kind: new(libraryv1.ItemKind_ITEM_KIND_MOVIE)}.Build()
}

func grammar(t *testing.T) *intent.Grammar {
	t.Helper()
	g, err := intent.Load(t.Context())
	if errors.Is(err, intent.ErrMissing) {
		t.Skip("the grammar is not built: run pnpm nx run intent:wasm")
	}
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = g.Close(context.Background()) })
	return g
}

func newTestService(t *testing.T, items ...*libraryv1.Item) *Service {
	return New(Config{
		Services: Services{Items: &fakeItems{items: items}},
		Grammar:  grammar(t),
		Key:      []byte("0123456789abcdef0123456789abcdef"),
	})
}

var signedIn = auth.Principal{
	User:    core.User{ID: core.NewID(), Name: "ana"},
	Session: core.AuthSession{ID: core.NewID()},
}

// ask runs a request without a model and returns what it sent.
func ask(t *testing.T, s *Service, req *assistantv1.AskRequest) []*assistantv1.AskResponse {
	t.Helper()
	var sent []*assistantv1.AskResponse
	ctx := auth.WithPrincipal(t.Context(), signedIn)
	set := core.DefaultAssistantSettings()
	tr := s.newTurn(ctx, signedIn, set, req, func(r *assistantv1.AskResponse) error {
		sent = append(sent, r)
		return nil
	})
	if err := tr.run(); err != nil {
		t.Fatalf("run(%q) = %v", req.GetText(), err)
	}
	return append(sent, assistantv1.AskResponse_builder{Done: tr.done}.Build())
}

func text(s string, langs ...string) *assistantv1.AskRequest {
	return assistantv1.AskRequest_builder{Text: new(s), Context: assistantv1.AskContext_builder{Languages: langs}.Build()}.Build()
}

func actions(sent []*assistantv1.AskResponse) []*assistantv1.ClientAction {
	var out []*assistantv1.ClientAction
	for _, r := range sent {
		if r.HasAction() {
			out = append(out, r.GetAction())
		}
	}
	return out
}

func TestAskPlays(t *testing.T) {
	s := newTestService(t, movie("m1", "Inception"), movie("m2", "盗梦空间"))
	for _, tc := range []struct {
		text, lang, want string
	}{
		{"play Inception", "en", "m1"},
		{"播放盗梦空间", "zh", "m2"},
		// A recognizer writing the title with other characters of the same
		// sound still finds it.
		{"播放道梦空间", "zh", "m2"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			sent := ask(t, s, text(tc.text, tc.lang))
			acts := actions(sent)
			if len(acts) != 1 || !acts[0].HasPlay() {
				t.Fatalf("actions = %v, want one play", acts)
			}
			if got := acts[0].GetPlay().GetItemIds(); !slices.Equal(got, []string{tc.want}) {
				t.Errorf("item ids = %v, want = [%s]", got, tc.want)
			}
			done := sent[len(sent)-1].GetDone()
			if done.GetAnswer() == "" || done.GetLanguage() != tc.lang {
				t.Errorf("done = %v, want an answer in %s", done, tc.lang)
			}
		})
	}
}

func TestAskControlsHere(t *testing.T) {
	s := newTestService(t)
	for _, phrase := range []string{"pause", "暂停", "Pause"} {
		acts := actions(ask(t, s, text(phrase)))
		if len(acts) != 1 || acts[0].GetControl().GetCommand() != assistantv1.ControlCommand_CONTROL_COMMAND_PAUSE {
			t.Errorf("%q: actions = %v, want pause", phrase, acts)
		}
	}
}

func TestAskChoosesAmongResults(t *testing.T) {
	s := newTestService(t, movie("a", "Alien"), movie("b", "Aliens"), movie("c", "Alien Covenant"))
	sent := ask(t, s, text("play ali", "en"))
	if got := sent[len(sent)-1].GetDone().GetResults(); got != 3 {
		t.Fatalf("results = %d, want = 3", got)
	}
	sent = ask(t, s, text("the second one", "en"))
	acts := actions(sent)
	if len(acts) != 1 || !slices.Equal(acts[0].GetPlay().GetItemIds(), []string{"b"}) {
		t.Errorf("actions = %v, want play of b", acts)
	}
}

func TestListenToken(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	s := New(Config{Key: []byte("key"), Now: func() time.Time { return now }})
	tok := s.sign(listenToken{User: "u", Expires: now.Add(time.Minute).Unix(), Nonce: "n1"})
	if got, err := s.verify(tok); err != nil || got.User != "u" {
		t.Fatalf("verify = %v, %v, want = u, nil", got, err)
	}
	if _, err := s.verify(tok); err == nil {
		t.Error("a token served twice")
	}
	if _, err := s.verify(strings.Replace(tok, ".", "x.", 1)); err == nil {
		t.Error("a tampered token verified")
	}
	late := s.sign(listenToken{User: "u", Expires: now.Add(-time.Second).Unix(), Nonce: "n2"})
	if _, err := s.verify(late); err == nil {
		t.Error("an expired token verified")
	}
}

func TestSettingsRoundTrip(t *testing.T) {
	want := core.DefaultAssistantSettings()
	want.Model = "ai/openai/gpt-5"
	want.AIAccess = core.AssistantAccessSelected
	want.AIUsers = []core.ID{core.NewID()}
	want.Voices = map[string]string{"en": "voice/kokoro/af_heart"}
	got, err := settingsFromProto(settingsToProto(want))
	if err != nil {
		t.Fatal(err)
	}
	if got.Model != want.Model || got.AIAccess != want.AIAccess || !slices.Equal(got.AIUsers, want.AIUsers) ||
		got.Voices["en"] != want.Voices["en"] || got.IdleTime != want.IdleTime || got.RetentionDays != want.RetentionDays {
		t.Errorf("settingsFromProto(settingsToProto(s)) = %+v, want = %+v", got, want)
	}
}

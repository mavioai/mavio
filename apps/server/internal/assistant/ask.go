package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	intentv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/intent/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
)

// confirmationTTL is how long a confirmation of the fast path waits.
const confirmationTTL = 5 * time.Minute

// pageSize is how many results are shown to choose from at a time.
const pageSize = 5

// dialog is a user's dialog state (docs/assistant.md §4.2, §7.1): the
// current conversation, a pending confirmation, the last results and item.
type dialog struct {
	mu           sync.Mutex
	conversation string
	active       time.Time
	pending      *pending
	results      []*libraryv1.Item
	offset       int
	// purpose is what choosing a result does: "play" or "open".
	purpose  string
	lastItem string
}

// pending is a call waiting for the user to confirm it: a tool of the fast
// path, or a model's call.
type pending struct {
	id, question, language string
	tool                   *tool
	args                   json.RawMessage
	// callID is the model's call; conversation its conversation.
	callID, conversation string
	expires              time.Time
}

func (s *Service) dialog(user core.ID) *dialog {
	s.mu.Lock()
	defer s.mu.Unlock()
	d, ok := s.dialogs[user]
	if !ok {
		d = &dialog{}
		s.dialogs[user] = d
	}
	return d
}

// turn is one Ask: its user, device, dialog and stream.
type turn struct {
	s      *Service
	p      auth.Principal
	set    core.AssistantSettings
	req    *assistantv1.AskRequest
	d      *dialog
	lang   string
	send   func(*assistantv1.AskResponse) error
	call   *call
	done   *assistantv1.Done
	sendMu sync.Mutex
}

func (t *turn) emit(r *assistantv1.AskResponse) {
	t.sendMu.Lock()
	defer t.sendMu.Unlock()
	if err := t.send(r); err != nil {
		t.s.log.DebugContext(t.call.ctx, "assistant: send", "err", err)
	}
}

// Ask carries a request out.
func (s *Service) Ask(ctx context.Context, req *assistantv1.AskRequest, stream *connect.ServerStream[assistantv1.AskResponse]) error {
	p, set, err := s.caller(ctx)
	if err != nil {
		return err
	}
	t := s.newTurn(ctx, p, set, req, stream.Send)
	if err := t.run(); err != nil {
		return err
	}
	t.emit(assistantv1.AskResponse_builder{Done: t.done}.Build())
	return nil
}

func (s *Service) newTurn(ctx context.Context, p auth.Principal, set core.AssistantSettings, req *assistantv1.AskRequest, send func(*assistantv1.AskResponse) error) *turn {
	t := &turn{s: s, p: p, set: set, req: req, d: s.dialog(p.User.ID), send: send, done: &assistantv1.Done{}}
	t.lang = answerLanguage("", req.GetContext())
	t.call = &call{s: s, ctx: ctx, user: p.User, ask: req.GetContext()}
	if !p.Session.ID.IsZero() {
		t.call.device = p.Session.ID.String()
		t.call.act = func(a *assistantv1.ClientAction) {
			t.emit(assistantv1.AskResponse_builder{Action: a}.Build())
		}
	}
	t.call.item = func(id string) {
		t.d.mu.Lock()
		t.d.lastItem = id
		t.d.mu.Unlock()
	}
	return t
}

func (t *turn) run() error {
	switch {
	case t.req.HasAnswer():
		return t.answer(t.req.GetAnswer().GetId(), t.req.GetAnswer().GetConfirmed())
	case t.req.GetChoice() != "":
		return t.choose(t.req.GetChoice())
	}
	text := strings.TrimSpace(t.req.GetText())
	intents, err := t.understand(text)
	if err != nil {
		return err
	}
	_, ai := t.s.agents(t.set, &t.p.User)
	for i, in := range intents {
		if in.GetMatch() == intentv1.IntentMatch_INTENT_MATCH_PARTIAL && ai {
			break
		}
		t.lang = answerLanguage(in.GetLanguage(), t.req.GetContext())
		if n := in.GetName(); n == "yes" || n == "no" {
			return t.answer("", n == "yes")
		}
		t.emit(assistantv1.AskResponse_builder{Understood: assistantv1.Understood_builder{Text: &text, Intent: new(in.GetName())}.Build()}.Build())
		o, err := t.fast(in)
		var ae *answerError
		if errors.As(err, &ae) && ae.key == "no_title" && (i+1 < len(intents) || ai) {
			continue
		}
		if err != nil {
			o = t.failed(err)
		}
		t.finish(o)
		t.remember(text)
		return nil
	}
	if ai && text != "" {
		return t.askAI(text)
	}
	if text != "" {
		t.withoutAI(text)
		return nil
	}
	t.finish(outcome{key: "not_understood", notUnderstood: true})
	return nil
}

// understand returns the request's intents: the client's, else the
// grammar's readings of the text.
func (t *turn) understand(text string) ([]*intentv1.Intent, error) {
	if t.req.HasIntent() {
		return []*intentv1.Intent{t.req.GetIntent()}, nil
	}
	g := t.s.cfg.Grammar
	if g == nil || text == "" {
		return nil, nil
	}
	t.d.mu.Lock()
	state := intentv1.DialogState_builder{
		Confirmation: new(t.d.pending != nil && t.s.now().Before(t.d.pending.expires)),
		Results:      new(int32(min(len(t.d.results)-t.d.offset, pageSize))),
		Playing:      new(t.req.GetContext().GetPlaying().GetItemId() != ""),
	}.Build()
	t.d.mu.Unlock()
	resp, err := g.Parse(t.call.ctx, intentv1.ParseIntentRequest_builder{
		Text: &text, Languages: t.req.GetContext().GetLanguages(), State: state,
	}.Build())
	if err != nil {
		t.s.log.WarnContext(t.call.ctx, "assistant: parse", "err", err)
		return nil, nil
	}
	return resp.GetIntents(), nil
}

// outcome is how the fast path answers: a sentence of the grammar, or a
// text of its own.
type outcome struct {
	key           string
	args          map[string]string
	text          string
	results       int
	confirmation  bool
	notUnderstood bool
}

func (t *turn) failed(err error) outcome {
	var ae *answerError
	if errors.As(err, &ae) {
		return outcome{key: ae.key, args: ae.args}
	}
	if c := connect.CodeOf(err); c == connect.CodePermissionDenied {
		return outcome{key: "not_allowed"}
	}
	t.s.log.WarnContext(t.call.ctx, "assistant: tool failed", "err", err)
	return outcome{key: "failed"}
}

// finish fills Done from an outcome.
func (t *turn) finish(o outcome) {
	text := o.text
	if text == "" && o.key != "" {
		text = t.s.reply(t.call.ctx, t.lang, o.key, o.args)
	}
	t.done.SetAnswer(text)
	t.done.SetLanguage(t.lang)
	t.done.SetConfirmation(o.confirmation)
	t.done.SetResults(int32(o.results))
	t.done.SetNotUnderstood(o.notUnderstood)
	t.d.mu.Lock()
	t.done.SetConversationId(t.d.conversation)
	t.d.mu.Unlock()
}

// reply writes a sentence of the grammar; without the grammar, its key.
func (s *Service) reply(ctx context.Context, lang, key string, args map[string]string) string {
	if s.cfg.Grammar == nil {
		return key
	}
	r, err := s.cfg.Grammar.Reply(ctx, intentv1.ReplyRequest_builder{Language: &lang, Key: &key, Args: args}.Build())
	if err != nil {
		s.log.WarnContext(ctx, "assistant: reply", "key", key, "err", err)
		return key
	}
	return r.GetText()
}

// withoutAI answers text the grammar did not match, without a model: as a
// title to find, else with examples (docs/assistant.md §4.5).
func (t *turn) withoutAI(text string) {
	items, err := t.call.resolveTitle(text, titleKinds)
	if err == nil && len(items) > 0 {
		t.finish(t.show(items, "open"))
		return
	}
	o := outcome{key: "no_ai", notUnderstood: true}
	if g := t.s.cfg.Grammar; g != nil {
		if ex, err := g.Examples(t.call.ctx, intentv1.ExamplesRequest_builder{Language: &t.lang}.Build()); err == nil {
			o.text = t.s.reply(t.call.ctx, t.lang, "no_ai", nil) + "\n" + strings.Join(ex.GetExamples(), "\n")
		}
	}
	t.finish(o)
}

// show offers results to choose from, a page at a time.
func (t *turn) show(items []*libraryv1.Item, purpose string) outcome {
	t.d.mu.Lock()
	t.d.results, t.d.offset, t.d.purpose = items, 0, purpose
	t.d.mu.Unlock()
	return t.page()
}

// page shows the page of results at the dialog's offset.
func (t *turn) page() outcome {
	t.d.mu.Lock()
	page := t.d.results[t.d.offset:min(len(t.d.results), t.d.offset+pageSize)]
	t.d.mu.Unlock()
	if t.call.act != nil {
		t.call.action(assistantv1.ClientAction_builder{ShowItems: assistantv1.ShowItemsAction_builder{Items: page, Choose: new(true)}.Build()}.Build())
	}
	names := make([]string, 0, len(page))
	for i, it := range page {
		names = append(names, fmt.Sprintf("%d. %s", i+1, describe(it)))
	}
	return outcome{key: "which", args: map[string]string{"choices": strings.Join(names, "; ")}, results: len(page)}
}

// describe names an item with what tells it apart from others of its
// name.
func describe(it *libraryv1.Item) string {
	if y := it.GetProductionYear(); y > 0 {
		return fmt.Sprintf("%s (%d)", title(it), y)
	}
	return title(it)
}

// choose acts on the result with an ID the client chose.
func (t *turn) choose(id string) error {
	t.d.mu.Lock()
	var it *libraryv1.Item
	for _, r := range t.d.results {
		if r.GetId() == id {
			it = r
		}
	}
	purpose := t.d.purpose
	t.d.mu.Unlock()
	if it == nil {
		var err error
		if it, err = t.call.fetch(id); err != nil {
			return err
		}
	}
	o, err := t.act(it, purpose)
	if err != nil {
		o = t.failed(err)
	}
	t.finish(o)
	return nil
}

// act plays or opens a chosen item.
func (t *turn) act(it *libraryv1.Item, purpose string) (outcome, error) {
	t.d.mu.Lock()
	t.d.results, t.d.offset = nil, 0
	t.d.mu.Unlock()
	if purpose == "play" {
		res, err := t.call.play(playArgs{ItemIDs: []string{it.GetId()}})
		if err != nil {
			return outcome{}, err
		}
		return outcome{key: "playing", args: map[string]string{"title": res.(playResult).Playing}}, nil
	}
	if _, err := t.call.navigate(navigateArgs{Target: "item", ID: it.GetId()}); err != nil {
		return outcome{}, err
	}
	return outcome{key: "opening", args: map[string]string{"title": title(it)}}, nil
}

// confirm asks the user to confirm a fast-path call of a tool.
func (t *turn) confirm(tl *tool, args any) (outcome, error) {
	raw, err := json.Marshal(args)
	if err != nil {
		return outcome{}, err
	}
	q, err := tl.confirm(t.call, raw, t.lang)
	if err != nil {
		return outcome{}, err
	}
	p := &pending{id: core.NewID().String(), question: q, language: t.lang, tool: tl, args: raw, expires: t.s.now().Add(confirmationTTL)}
	t.d.mu.Lock()
	t.d.pending = p
	t.d.mu.Unlock()
	t.emit(assistantv1.AskResponse_builder{Confirmation: assistantv1.Confirmation_builder{Id: &p.id, Text: &q}.Build()}.Build())
	return outcome{text: q, confirmation: true}, nil
}

// answer answers the pending confirmation; id empty answers whichever is
// pending.
func (t *turn) answer(id string, confirmed bool) error {
	t.d.mu.Lock()
	p := t.d.pending
	if p != nil && (id == "" || p.id == id) {
		t.d.pending = nil
	} else {
		p = nil
	}
	t.d.mu.Unlock()
	if p == nil || (p.callID == "" && t.s.now().After(p.expires)) {
		t.finish(outcome{key: "cancelled"})
		return nil
	}
	t.lang = p.language
	if p.callID != "" {
		return t.answerCall(p, confirmed)
	}
	if !confirmed {
		t.finish(outcome{key: "cancelled"})
		return nil
	}
	v, err := p.tool.run(t.call, p.args)
	if err != nil {
		t.finish(t.failed(err))
		return nil
	}
	b, _ := json.Marshal(v)
	t.finish(confirmedOutcome(p.tool, string(b)))
	t.remember(t.req.GetText())
	return nil
}

// confirmedOutcome answers a confirmed call by its result.
func confirmedOutcome(tl *tool, resultJSON string) outcome {
	var m map[string]any
	_ = json.Unmarshal([]byte(resultJSON), &m)
	if tl.name == "request_title" {
		return outcome{key: "requested", args: map[string]string{"title": fmt.Sprint(m["requested"])}}
	}
	return outcome{key: "resumed"}
}

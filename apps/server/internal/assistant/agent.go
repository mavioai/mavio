package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/libs/core"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// assistantLabel marks the plugin's sessions that are the assistant's
// conversations.
const assistantLabel = "mavio.assistant"

// callTTL is how long the results of the model's tool calls are kept.
const callTTL = 24 * time.Hour

// callResult is a kept result of a model's tool call.
type callResult struct {
	result *pluginv1.SubmitAgentToolResultRequest
	at     time.Time
}

// aiTurn is a message the model answers. It outlives the Ask that sent it
// when a tool call waits for confirmation: the Ask confirming it attaches
// to it and streams the rest.
type aiTurn struct {
	mu   sync.Mutex
	sink func(*assistantv1.AskResponse)
	call *call
	// waiting is signaled when a call waits for confirmation.
	waiting chan string
	done    chan struct{}
	final   *assistantv1.Done
}

func (a *aiTurn) emit(r *assistantv1.AskResponse) {
	a.mu.Lock()
	sink := a.sink
	a.mu.Unlock()
	if sink != nil {
		sink(r)
	}
}

// attach makes a turn's events and client actions go to t's stream.
func (a *aiTurn) attach(t *turn) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sink = t.emit
	a.call.act, a.call.ask, a.call.device = t.call.act, t.call.ask, t.call.device
}

func (a *aiTurn) detach() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.sink, a.call.act = nil, nil
}

// modelRef splits "ai/<provider>/<model>".
func modelRef(ref string) (provider, model string) {
	rest, _ := strings.CutPrefix(ref, "ai/")
	provider, model, _ = strings.Cut(rest, "/")
	return provider, model
}

// declarations declares the tools to the plugin.
func (s *Service) declarations() []*pluginv1.AgentTool {
	names := make([]string, 0, len(s.tool))
	for n := range s.tool {
		names = append(names, n)
	}
	slices.Sort(names)
	out := make([]*pluginv1.AgentTool, 0, len(names))
	for _, n := range names {
		t := s.tool[n]
		params, _ := json.Marshal(t.params)
		result, _ := json.Marshal(t.result)
		out = append(out, pluginv1.AgentTool_builder{
			Name: &t.name, Description: &t.description, ParametersJson: new(string(params)), ResultJson: new(string(result)),
		}.Build())
	}
	return out
}

// instructions are the model's instructions, stable across requests so
// that providers' prompt caches hold (docs/assistant.md §6.3).
func (s *Service) instructions(u *core.User) string {
	return fmt.Sprintf(`You are the assistant of %q, a Mavio media server, talking with its user %q through one of their devices.

Rules:
- Act through the tools; never claim to have done what no tool did. The tools act as the user, under the user's permissions.
- Find items with search_library before acting on them; when several match and the request does not tell them apart, list them briefly and ask which.
- Each request starts with its context in brackets: the device, what it plays, the screen, the time and the user's languages. "This" and "it" mean what plays, else what is shown.
- Never reveal what happens in a story after the position the user has reached.
- Answer in the user's language, briefly, in one or two sentences that read well aloud; results the tools showed need not be read out.
- Ask before acting when a request is ambiguous; a tool that needs the user's confirmation asks for it itself.
- Requests already carried out without you appear in the conversation as they happened.`, s.cfg.ServerName(), u.Name)
}

// conversation returns the user's current conversation, starting a new one
// after the idle time or on another day (docs/assistant.md §7.1).
func (t *turn) conversation(ag pluginv1connect.AgentSessionServiceClient) (string, error) {
	ctx, now := t.call.ctx, t.s.now()
	idle := t.set.IdleTime
	fresh := func(at time.Time) bool {
		y1, m1, d1 := at.Local().Date()
		y2, m2, d2 := now.Local().Date()
		return now.Sub(at) < idle && y1 == y2 && m1 == m2 && d1 == d2
	}
	t.d.mu.Lock()
	conv, active := t.d.conversation, t.d.active
	t.d.mu.Unlock()
	if conv != "" && fresh(active) {
		t.touch(conv)
		return conv, nil
	}
	if conv == "" {
		list, err := ag.ListAgentSessions(ctx, pluginv1.ListAgentSessionsRequest_builder{Owner: new(t.p.User.ID.String())}.Build())
		if err != nil {
			return "", err
		}
		for _, sess := range list.GetSessions() {
			if sess.GetLabels()[assistantLabel] != "" && !sess.GetEnded() && fresh(sess.GetUpdated().AsTime()) {
				t.touch(sess.GetId())
				return sess.GetId(), nil
			}
		}
	} else {
		// The plugin consolidates the memories of a conversation as it ends.
		go func(id string) {
			if _, err := ag.EndAgentSession(context.WithoutCancel(ctx), pluginv1.EndAgentSessionRequest_builder{Id: &id}.Build()); err != nil {
				t.s.log.WarnContext(ctx, "assistant: end conversation", "err", err)
			}
		}(conv)
	}
	provider, model := modelRef(t.set.Model)
	device := t.p.Session.DeviceName
	created, err := ag.CreateAgentSession(ctx, pluginv1.CreateAgentSessionRequest_builder{
		Provider: &provider, Model: &model, Instructions: new(t.s.instructions(&t.p.User)),
		Tools: t.s.declarations(), CodeMode: &t.set.CodeMode, Owner: new(t.p.User.ID.String()), Memory: &t.set.Memory,
		Labels: map[string]string{assistantLabel: "1", "device": device},
	}.Build())
	if err != nil {
		return "", err
	}
	id := created.GetSession().GetId()
	t.touch(id)
	return id, nil
}

func (t *turn) touch(conv string) {
	t.d.mu.Lock()
	t.d.conversation, t.d.active = conv, t.s.now()
	t.d.mu.Unlock()
}

// contextLine tells the model the request's context.
func (t *turn) contextLine() string {
	ctx := t.req.GetContext()
	parts := []string{fmt.Sprintf("device %q", t.p.Session.DeviceName)}
	if p := ctx.GetPlaying(); p.GetItemId() != "" {
		state := "playing"
		if p.GetPaused() {
			state = "paused"
		}
		name := p.GetItemId()
		if it, err := t.call.fetch(p.GetItemId()); err == nil {
			name = fmt.Sprintf("%q (id %s)", title(it), it.GetId())
		}
		parts = append(parts, fmt.Sprintf("%s %s at %s", state, name, clock(p.GetPosition().AsDuration().Seconds())))
	}
	if sc := ctx.GetScreen(); sc != "" {
		parts = append(parts, "screen "+sc)
	}
	if id := ctx.GetItemId(); id != "" {
		parts = append(parts, "item shown "+id)
	}
	if ids := ctx.GetSelectedItemIds(); len(ids) > 0 {
		parts = append(parts, "selected "+strings.Join(ids, ", "))
	}
	now := t.s.now()
	if loc, err := time.LoadLocation(ctx.GetTimeZone()); err == nil && ctx.GetTimeZone() != "" {
		now = now.In(loc)
	}
	parts = append(parts, "time "+now.Format("Mon 2006-01-02 15:04 MST"))
	if langs := ctx.GetLanguages(); len(langs) > 0 {
		parts = append(parts, "languages "+strings.Join(langs, ", "))
	}
	if t.req.GetSpoken() {
		parts = append(parts, "spoken")
	}
	if f := t.req.GetFailedActions(); len(f) > 0 {
		parts = append(parts, "the device could not carry out "+strings.Join(f, ", "))
	}
	return "[" + strings.Join(parts, "; ") + "]"
}

// askAI sends text to the user's conversation and streams the answer
// (docs/assistant.md §6).
func (t *turn) askAI(text string) error {
	ag, _ := t.s.agents(t.set, &t.p.User)
	t.emit(assistantv1.AskResponse_builder{Understood: assistantv1.Understood_builder{Text: &text, Ai: new(true)}.Build()}.Build())
	conv, err := t.conversation(ag)
	if err != nil {
		t.s.log.WarnContext(t.call.ctx, "assistant: conversation", "err", err)
		t.finish(outcome{key: "failed"})
		return nil
	}
	// The turn outlives the request when a call waits for confirmation.
	c := *t.call
	c.ctx = context.WithoutCancel(t.call.ctx)
	at := &aiTurn{sink: t.emit, call: &c, waiting: make(chan string, 1), done: make(chan struct{})}
	t.s.mu.Lock()
	t.s.turns[conv] = at
	t.s.mu.Unlock()
	message := t.contextLine() + "\n" + text
	go t.s.stream(ag, conv, message, t.lang, at)
	return t.await(at, conv)
}

// await streams a turn's events until it ends or waits for confirmation.
func (t *turn) await(at *aiTurn, conv string) error {
	select {
	case <-at.done:
		t.done = at.final
	case q := <-at.waiting:
		at.detach()
		t.finish(outcome{text: q, confirmation: true})
	case <-t.call.ctx.Done():
		at.detach()
		return t.call.ctx.Err()
	}
	if t.done.GetConversationId() == "" {
		t.done.SetConversationId(conv)
	}
	return nil
}

// stream runs a message of the model in the background.
func (s *Service) stream(ag pluginv1connect.AgentSessionServiceClient, conv, message, lang string, at *aiTurn) {
	ctx := at.call.ctx
	defer func() {
		s.mu.Lock()
		if s.turns[conv] == at {
			delete(s.turns, conv)
		}
		s.mu.Unlock()
		close(at.done)
	}()
	done := assistantv1.Done_builder{Language: &lang, ConversationId: &conv}.Build()
	at.final = done
	st, err := ag.StreamAgentMessage(ctx, pluginv1.StreamAgentMessageRequest_builder{
		Message: pluginv1.SendAgentMessageRequest_builder{SessionId: &conv, Text: &message, RequestId: new(core.NewID().String())}.Build(),
	}.Build())
	if err != nil {
		s.log.WarnContext(ctx, "assistant: message", "err", err)
		done.SetAnswer(s.reply(ctx, lang, "failed", nil))
		return
	}
	defer st.Close()
	var answer strings.Builder
	for st.Receive() {
		m := st.Msg()
		switch {
		case m.HasText():
			answer.WriteString(m.GetText())
			at.emit(assistantv1.AskResponse_builder{Answer: new(m.GetText())}.Build())
		case m.HasDone():
			if a := m.GetDone().GetAnswer().GetText(); a != "" {
				answer.Reset()
				answer.WriteString(a)
			}
		}
	}
	if err := st.Err(); err != nil {
		s.log.WarnContext(ctx, "assistant: message", "err", err)
		if answer.Len() == 0 {
			answer.WriteString(s.reply(ctx, lang, "failed", nil))
		}
	}
	done.SetAnswer(strings.TrimSpace(answer.String()))
}

// answerCall answers a model's call waiting for confirmation and streams
// the rest of its turn.
func (t *turn) answerCall(p *pending, confirmed bool) error {
	ag, ok := t.s.cfg.Agents()
	if !ok {
		t.finish(outcome{key: "failed"})
		return nil
	}
	t.s.mu.Lock()
	at := t.s.turns[p.conversation]
	t.s.mu.Unlock()
	c := t.call
	if at != nil {
		at.attach(t)
		c = at.call
	}
	result := &pluginv1.SubmitAgentToolResultRequest{}
	if confirmed {
		result = c.runTool(p.tool, p.args)
	} else {
		result.SetFailed(true)
		result.SetText("The user declined.")
	}
	t.s.submit(c.ctx, ag, p.callID, result)
	if at == nil {
		// The model's turn is gone, so the server answers for it.
		switch {
		case !confirmed:
			t.finish(outcome{key: "cancelled"})
		case result.GetFailed():
			t.finish(outcome{key: "failed"})
		default:
			t.finish(confirmedOutcome(p.tool, result.GetResultJson()))
		}
		return nil
	}
	return t.await(at, p.conversation)
}

// submit answers a call and keeps its result.
func (s *Service) submit(ctx context.Context, ag pluginv1connect.AgentSessionServiceClient, id string, r *pluginv1.SubmitAgentToolResultRequest) {
	r.SetCallId(id)
	s.mu.Lock()
	s.calls[id] = callResult{result: r, at: s.now()}
	s.mu.Unlock()
	if _, err := ag.SubmitAgentToolResult(ctx, r); err != nil {
		s.log.WarnContext(ctx, "assistant: submit tool result", "call", id, "err", err)
	}
}

// watchCalls runs the model's calls of the tools, reconnecting to the
// plugin as it restarts (docs/assistant.md §6.1).
func (s *Service) watchCalls(ctx context.Context) {
	running := map[string]bool{}
	var mu sync.Mutex
	for ctx.Err() == nil {
		ag, ok := pluginv1connect.AgentSessionServiceClient(nil), false
		if s.cfg.Agents != nil {
			ag, ok = s.cfg.Agents()
		}
		if ok {
			st, err := ag.WatchAgentToolCalls(ctx, &pluginv1.WatchAgentToolCallsRequest{})
			if err == nil {
				for st.Receive() {
					c := st.Msg().GetCall()
					mu.Lock()
					busy := running[c.GetId()]
					running[c.GetId()] = true
					mu.Unlock()
					if busy {
						continue
					}
					go func() {
						s.handleCall(ctx, ag, c)
						mu.Lock()
						delete(running, c.GetId())
						mu.Unlock()
					}()
				}
				err = st.Err()
				_ = st.Close()
			}
			if err != nil && ctx.Err() == nil && connect.CodeOf(err) != connect.CodeUnimplemented {
				s.log.DebugContext(ctx, "assistant: watch tool calls", "err", err)
			}
		}
		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
		}
	}
}

// handleCall runs a model's call as the conversation's user.
func (s *Service) handleCall(ctx context.Context, ag pluginv1connect.AgentSessionServiceClient, tc *pluginv1.AgentToolCall) {
	s.mu.Lock()
	kept, ok := s.calls[tc.GetId()]
	at := s.turns[tc.GetSessionId()]
	s.mu.Unlock()
	if ok {
		if _, err := ag.SubmitAgentToolResult(ctx, kept.result); err != nil {
			s.log.WarnContext(ctx, "assistant: submit tool result", "call", tc.GetId(), "err", err)
		}
		return
	}
	t, known := s.tool[tc.GetTool()]
	if !known {
		r := &pluginv1.SubmitAgentToolResultRequest{}
		r.SetFailed(true)
		r.SetText("no tool " + tc.GetTool())
		s.submit(ctx, ag, tc.GetId(), r)
		return
	}
	var c *call
	emit := func(*assistantv1.AskResponse) {}
	if at != nil {
		at.mu.Lock()
		c = at.call
		at.mu.Unlock()
		emit = at.emit
	} else {
		owner, err := core.ParseID(tc.GetOwner())
		if err != nil {
			s.log.WarnContext(ctx, "assistant: tool call without a user", "call", tc.GetId())
			return
		}
		u, err := s.cfg.Users.Get(ctx, owner)
		if err != nil {
			r := &pluginv1.SubmitAgentToolResultRequest{}
			r.SetFailed(true)
			r.SetText("the user is gone")
			s.submit(ctx, ag, tc.GetId(), r)
			return
		}
		d := s.dialog(u.ID)
		c = &call{
			s: s, ctx: auth.WithPrincipal(context.WithoutCancel(ctx), auth.Principal{User: u}), user: u,
			item: func(id string) { d.mu.Lock(); d.lastItem = id; d.mu.Unlock() },
		}
	}
	args := json.RawMessage(tc.GetArgumentsJson())
	summary := t.name
	if len(args) > 2 && len(args) < 200 {
		summary += " " + string(args)
	}
	activity := func(done, failed bool) {
		emit(assistantv1.AskResponse_builder{Tool: assistantv1.ToolActivity_builder{
			Id: new(tc.GetId()), Tool: &t.name, Summary: &summary, Done: &done, Failed: &failed,
		}.Build()}.Build())
	}
	activity(false, false)
	if t.confirm != nil {
		lang := "en"
		if ls := c.ask.GetLanguages(); len(ls) > 0 {
			lang = ls[0]
		}
		q, err := t.confirm(c, args, lang)
		if err != nil {
			r := &pluginv1.SubmitAgentToolResultRequest{}
			r.SetFailed(true)
			r.SetText(err.Error())
			s.submit(ctx, ag, tc.GetId(), r)
			activity(true, true)
			return
		}
		p := &pending{
			id: core.NewID().String(), question: q, language: lang, tool: t, args: args,
			callID: tc.GetId(), conversation: tc.GetSessionId(), expires: s.now().Add(callTTL),
		}
		d := s.dialog(c.user.ID)
		d.mu.Lock()
		d.pending = p
		d.mu.Unlock()
		emit(assistantv1.AskResponse_builder{Confirmation: assistantv1.Confirmation_builder{Id: &p.id, Text: &q}.Build()}.Build())
		if at != nil {
			select {
			case at.waiting <- q:
			default:
			}
		}
		return
	}
	r := c.runTool(t, args)
	s.submit(ctx, ag, tc.GetId(), r)
	activity(true, r.GetFailed())
}

// remember adds a fast-path request and its answer to the user's
// conversation, so that the model knows what was done (§7.1).
func (t *turn) remember(text string) {
	ag, ok := t.s.agents(t.set, &t.p.User)
	if !ok || text == "" || t.done.GetAnswer() == "" {
		return
	}
	ctx := context.WithoutCancel(t.call.ctx)
	answer, now := t.done.GetAnswer(), timestamppb.New(t.s.now())
	go func() {
		conv, err := t.conversation(ag)
		if err != nil {
			t.s.log.WarnContext(ctx, "assistant: conversation", "err", err)
			return
		}
		user, assistant := pluginv1.AiRole_AI_ROLE_USER, pluginv1.AiRole_AI_ROLE_ASSISTANT
		if _, err := ag.AddAgentMessages(ctx, pluginv1.AddAgentMessagesRequest_builder{
			SessionId: &conv, RequestId: new(core.NewID().String()),
			Messages: []*pluginv1.AgentMessage{
				pluginv1.AgentMessage_builder{Role: &user, Text: new(t.contextLine() + "\n" + text), Time: now}.Build(),
				pluginv1.AgentMessage_builder{Role: &assistant, Text: &answer, Time: now}.Build(),
			},
		}.Build()); err != nil {
			t.s.log.WarnContext(ctx, "assistant: add messages", "err", err)
		}
	}()
}

// sweep deletes conversations past the retention and those of deleted
// users with their memories, and forgets kept state that expired.
func (s *Service) sweep(ctx context.Context) {
	now := s.now()
	s.mu.Lock()
	for id, r := range s.calls {
		if now.Sub(r.at) > callTTL {
			delete(s.calls, id)
		}
	}
	for tok, exp := range s.listens {
		if now.After(exp) {
			delete(s.listens, tok)
		}
	}
	s.mu.Unlock()
	if s.cfg.Agents == nil || s.cfg.Users == nil {
		return
	}
	ag, ok := s.cfg.Agents()
	if !ok {
		return
	}
	list, err := ag.ListAgentSessions(ctx, &pluginv1.ListAgentSessionsRequest{})
	if err != nil {
		return
	}
	retention := time.Duration(s.cfg.Settings().RetentionDays) * 24 * time.Hour
	gone := map[string]bool{}
	for _, sess := range list.GetSessions() {
		if sess.GetLabels()[assistantLabel] == "" {
			continue
		}
		owner := sess.GetOwner()
		if _, seen := gone[owner]; !seen {
			id, err := core.ParseID(owner)
			if err == nil {
				_, err = s.cfg.Users.Get(ctx, id)
			}
			gone[owner] = errors.Is(err, core.ErrNotFound) || owner == ""
		}
		if gone[owner] || now.Sub(sess.GetUpdated().AsTime()) > retention {
			if _, err := ag.DeleteAgentSession(ctx, pluginv1.DeleteAgentSessionRequest_builder{Id: new(sess.GetId())}.Build()); err != nil {
				s.log.WarnContext(ctx, "assistant: delete conversation", "err", err)
			}
		}
	}
	for owner, isGone := range gone {
		if isGone && owner != "" {
			if _, err := ag.DeleteAgentMemories(ctx, pluginv1.DeleteAgentMemoriesRequest_builder{Owner: &owner}.Build()); err != nil {
				s.log.WarnContext(ctx, "assistant: delete memories", "err", err)
			}
		}
	}
}

// agentFor returns the agent plugin for the caller's conversations and
// memories.
func (s *Service) agentFor(ctx context.Context) (auth.Principal, pluginv1connect.AgentSessionServiceClient, error) {
	p, ok := auth.FromContext(ctx)
	if !ok || p.User.ID.IsZero() {
		return p, nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not signed in"))
	}
	if s.cfg.Agents != nil {
		if ag, ok := s.cfg.Agents(); ok {
			return p, ag, nil
		}
	}
	return p, nil, connect.NewError(connect.CodeFailedPrecondition, errors.New("no AGENT_SESSION plugin is running"))
}

func conversationToProto(sess *pluginv1.AgentSession) *assistantv1.AssistantConversation {
	return assistantv1.AssistantConversation_builder{
		Id: new(sess.GetId()), Title: new(sess.GetTitle()), DeviceName: new(sess.GetLabels()["device"]),
		CreateTime: sess.GetCreated(), UpdateTime: sess.GetUpdated(), Usage: sess.GetUsage(),
	}.Build()
}

// ListAssistantConversations lists the user's conversations, newest first.
func (s *Service) ListAssistantConversations(ctx context.Context, _ *assistantv1.ListAssistantConversationsRequest) (*assistantv1.ListAssistantConversationsResponse, error) {
	p, ag, err := s.agentFor(ctx)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeFailedPrecondition {
			return &assistantv1.ListAssistantConversationsResponse{}, nil
		}
		return nil, err
	}
	list, err := ag.ListAgentSessions(ctx, pluginv1.ListAgentSessionsRequest_builder{Owner: new(p.User.ID.String())}.Build())
	if err != nil {
		return nil, err
	}
	var out []*assistantv1.AssistantConversation
	for _, sess := range list.GetSessions() {
		if sess.GetLabels()[assistantLabel] != "" {
			out = append(out, conversationToProto(sess))
		}
	}
	slices.SortFunc(out, func(a, b *assistantv1.AssistantConversation) int {
		return b.GetUpdateTime().AsTime().Compare(a.GetUpdateTime().AsTime())
	})
	return assistantv1.ListAssistantConversationsResponse_builder{Conversations: out}.Build(), nil
}

// own returns one of the caller's conversations.
func (s *Service) own(ctx context.Context, id string) (auth.Principal, pluginv1connect.AgentSessionServiceClient, *pluginv1.GetAgentSessionResponse, error) {
	p, ag, err := s.agentFor(ctx)
	if err != nil {
		return p, nil, nil, err
	}
	resp, err := ag.GetAgentSession(ctx, pluginv1.GetAgentSessionRequest_builder{Id: &id}.Build())
	if err != nil {
		return p, nil, nil, err
	}
	if resp.GetSession().GetOwner() != p.User.ID.String() || resp.GetSession().GetLabels()[assistantLabel] == "" {
		return p, nil, nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no conversation %s", id))
	}
	return p, ag, resp, nil
}

// GetAssistantConversation returns one of the user's conversations.
func (s *Service) GetAssistantConversation(ctx context.Context, req *assistantv1.GetAssistantConversationRequest) (*assistantv1.GetAssistantConversationResponse, error) {
	_, _, resp, err := s.own(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	var msgs []*assistantv1.AssistantMessage
	for _, m := range resp.GetMessages() {
		text := m.GetText()
		// The context line is the model's, not what the user said.
		if m.GetRole() == pluginv1.AiRole_AI_ROLE_USER && strings.HasPrefix(text, "[") {
			if _, rest, ok := strings.Cut(text, "]\n"); ok {
				text = rest
			}
		}
		msgs = append(msgs, assistantv1.AssistantMessage_builder{
			User: new(m.GetRole() == pluginv1.AiRole_AI_ROLE_USER), Text: &text, Time: m.GetTime(),
		}.Build())
	}
	return assistantv1.GetAssistantConversationResponse_builder{Conversation: conversationToProto(resp.GetSession()), Messages: msgs}.Build(), nil
}

// DeleteAssistantConversation deletes one of the user's conversations.
func (s *Service) DeleteAssistantConversation(ctx context.Context, req *assistantv1.DeleteAssistantConversationRequest) (*assistantv1.DeleteAssistantConversationResponse, error) {
	p, ag, _, err := s.own(ctx, req.GetId())
	if err != nil {
		return nil, err
	}
	if _, err := ag.DeleteAgentSession(ctx, pluginv1.DeleteAgentSessionRequest_builder{Id: new(req.GetId())}.Build()); err != nil {
		return nil, err
	}
	d := s.dialog(p.User.ID)
	d.mu.Lock()
	if d.conversation == req.GetId() {
		d.conversation = ""
	}
	d.mu.Unlock()
	return &assistantv1.DeleteAssistantConversationResponse{}, nil
}

func memoryToProto(m *pluginv1.AgentMemory) *assistantv1.AssistantMemory {
	return assistantv1.AssistantMemory_builder{
		Id: new(m.GetId()), Text: new(m.GetText()), CreateTime: m.GetCreateTime(), UpdateTime: m.GetUpdateTime(), UseTime: m.GetUseTime(),
	}.Build()
}

// ListAssistantMemories lists what the assistant remembers of the user.
func (s *Service) ListAssistantMemories(ctx context.Context, _ *assistantv1.ListAssistantMemoriesRequest) (*assistantv1.ListAssistantMemoriesResponse, error) {
	p, ag, err := s.agentFor(ctx)
	if err != nil {
		if connect.CodeOf(err) == connect.CodeFailedPrecondition {
			return &assistantv1.ListAssistantMemoriesResponse{}, nil
		}
		return nil, err
	}
	list, err := ag.ListAgentMemories(ctx, pluginv1.ListAgentMemoriesRequest_builder{Owner: new(p.User.ID.String())}.Build())
	if err != nil {
		return nil, err
	}
	var out []*assistantv1.AssistantMemory
	for _, m := range list.GetMemories() {
		out = append(out, memoryToProto(m))
	}
	return assistantv1.ListAssistantMemoriesResponse_builder{Memories: out}.Build(), nil
}

// UpdateAssistantMemory changes or adds a memory.
func (s *Service) UpdateAssistantMemory(ctx context.Context, req *assistantv1.UpdateAssistantMemoryRequest) (*assistantv1.UpdateAssistantMemoryResponse, error) {
	p, ag, err := s.agentFor(ctx)
	if err != nil {
		return nil, err
	}
	resp, err := ag.PutAgentMemory(ctx, pluginv1.PutAgentMemoryRequest_builder{
		Owner: new(p.User.ID.String()), Id: new(req.GetId()), Text: new(req.GetText()),
	}.Build())
	if err != nil {
		return nil, err
	}
	return assistantv1.UpdateAssistantMemoryResponse_builder{Memory: memoryToProto(resp.GetMemory())}.Build(), nil
}

// DeleteAssistantMemories deletes memories, or all of the user's.
func (s *Service) DeleteAssistantMemories(ctx context.Context, req *assistantv1.DeleteAssistantMemoriesRequest) (*assistantv1.DeleteAssistantMemoriesResponse, error) {
	p, ag, err := s.agentFor(ctx)
	if err != nil {
		return nil, err
	}
	if _, err := ag.DeleteAgentMemories(ctx, pluginv1.DeleteAgentMemoriesRequest_builder{
		Owner: new(p.User.ID.String()), Ids: req.GetIds(),
	}.Build()); err != nil {
		return nil, err
	}
	return &assistantv1.DeleteAssistantMemoriesResponse{}, nil
}

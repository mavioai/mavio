package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// convState is the state of the AI settings' conversations (docs/clients.md
// §6.3.26): those kept, the one shown, a new one's form and the message
// being answered.
type convState struct {
	list task[[]*pluginv1.AgentSession]
	one  task[*systemv1.GetAiConversationResponse]
	// reload loads the conversations again, without the rest of the
	// settings.
	reload int

	// The new conversation's form.
	model, instructions, first string
	creating                   bool
	createErr                  error

	// The conversation's page: the message typed, the one being answered,
	// and its deleting.
	text       string
	exchange   *aiExchange
	deleting   bool
	deleteErr  error
	confirming bool
	// shown is what the page last scrolled to the end for: its messages
	// and the answer so far.
	shown string
}

// aiExchange is a message being answered: its text and the answer so far,
// or its error once failed, until it is sent again or another is.
type aiExchange struct {
	conversation, request string
	text, answer          string
	err                   error
}

func (a *App) aiConversationService(id string) systemv1connect.AiConversationServiceClient {
	return systemv1connect.NewAiConversationServiceClient(a.connectHTTP(), serverURL(id))
}

func conversationPath(id string) string {
	return "/settings/ai?" + url.Values{"conversation": {id}}.Encode()
}

// isAssistantConversation reports whether a conversation is one of the
// assistant's: the server labels them so.
func isAssistantConversation(s *pluginv1.AgentSession) bool {
	return s.GetLabels()["mavio.assistant"] != ""
}

// conversationTitle is a conversation's title; the assistant's have none,
// and are named so.
func conversationTitle(c *ui.Context, s *pluginv1.AgentSession) string {
	switch {
	case s.GetTitle() != "":
		return s.GetTitle()
	case isAssistantConversation(s):
		return style.T(c, "conversations.assistant")
	}
	return style.T(c, "conversations.untitled")
}

// grouped writes n with its thousands grouped.
func grouped(n int64) string {
	s := strconv.FormatInt(n, 10)
	for i := len(s) - 3; i > 0 && s[i-1] != '-'; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}

// usageDollars writes a cost in dollars to three significant digits, as
// the web UI's.
func usageDollars(v float64) string {
	if v == 0 {
		return "$0.00"
	}
	digits := max(2, 2-int(math.Floor(math.Log10(v))))
	return "$" + strconv.FormatFloat(v, 'f', digits, 64)
}

func usageOf(c *ui.Context, u *pluginv1.AiUsage) string {
	return style.T(c, "conversations.usage", "input", grouped(u.GetInputTokens()), "output", grouped(u.GetOutputTokens()),
		"cost", usageDollars(u.GetCost()))
}

// conversationModel is a conversation's model as the picker lists it, else
// by its IDs, as for a model its provider no longer has.
func conversationModel(options []modelOption, s *pluginv1.AgentSession) modelOption {
	value := s.GetProvider() + "/" + s.GetModel()
	for _, o := range options {
		if o.value == value {
			return o
		}
	}
	return modelOption{value: value, label: s.GetModel(), provider: s.GetProvider(), providerName: s.GetProvider()}
}

// modelName is a model's name and its provider's, on one line.
func modelName(o modelOption) string { return o.label + " (" + o.providerName + ")" }

// partedMessage parts a user's message to the assistant from the context
// line the assistant put before it for the model, as the assistant's own
// conversations leave it out (docs/assistant.md).
func partedMessage(assistant bool, text string) (context, rest string) {
	if !assistant || !strings.HasPrefix(text, "[") {
		return "", text
	}
	end := strings.Index(text, "]\n")
	if end < 0 {
		return "", text
	}
	return text[:end+1], text[end+2:]
}

func (a *App) conversationList() *task[[]*pluginv1.AgentSession] {
	st := &a.settingsState.admin.ai.conv
	return adminTask(a, &st.list, "ai-conversations", func(ctx context.Context, s string) ([]*pluginv1.AgentSession, error) {
		res, err := a.aiConversationService(s).ListAiConversations(ctx, &systemv1.ListAiConversationsRequest{})
		return res.GetConversations(), err
	}, st.reload)
}

// aiConversations builds the groups of the conversations at the end of the
// AI settings, as the web UI's: a new one, and those kept.
func (a *App) aiConversations(c *ui.Context) {
	list := a.conversationList()
	if isNoAiGateway(list.err) {
		group(c, "conversations-plugin", style.T(c, "settings.conversations"), style.T(c, "conversations.noPluginFooter"), func(g *settingsGroup) {
			if g.link("plugins", "plugins", ui.RGB(0xAF, 0x52, 0xDE), style.T(c, "conversations.noPlugin"), style.T(c, "ai.noGatewayDetail"), true) {
				a.router.Push(pluginsPath("", false))
			}
		})
		return
	}
	options := modelOptions(c, "", a.aiModels().value, a.providerNames())
	a.newConversation(c, options)
	names := map[string]string{}
	for _, u := range a.users().value {
		names[u.GetId()] = u.GetName()
	}
	group(c, "conversations", style.T(c, "conversations.list"), style.T(c, "conversations.message"), func(g *settingsGroup) {
		for _, s := range list.value {
			var detail []string
			if isAssistantConversation(s) {
				// An assistant's conversation says whose it is, and where it
				// started.
				detail = append(detail, names[s.GetOwner()], s.GetLabels()["device"])
			}
			detail = append(detail, modelName(conversationModel(options, s)))
			if s.HasUpdated() {
				detail = append(detail, fromNow(c, s.GetUpdated().AsTime()))
			}
			detail = append(detail, usageOf(c, s.GetUsage()))
			if g.link("conversation-"+s.GetId(), "", ui.Color{}, conversationTitle(c, s), joinDetail(detail...), true) {
				a.router.Push(conversationPath(s.GetId()))
			}
		}
		if list.loaded && list.err == nil && len(list.value) == 0 {
			g.note(style.T(c, "conversations.none"), false)
		}
		if list.err != nil {
			g.note(errorMessage(c, list.err), true)
		}
	})
}

// newConversation builds the form starting a conversation with a model,
// instructions and its first message, as the web UI's.
func (a *App) newConversation(c *ui.Context, options []modelOption) {
	st := &a.settingsState.admin.ai.conv
	colors := style.Colors(c)
	if a.aiModels().loaded && len(options) == 0 {
		group(c, "new-conversation", style.T(c, "conversations.new"), "", func(g *settingsGroup) {
			g.note(style.T(c, "conversations.noModels"), false)
		})
		return
	}
	chosen := st.model
	if chosen == "" && len(options) > 0 {
		chosen = options[0].value
	}
	label := ""
	for _, o := range options {
		if o.value == chosen {
			label = o.label
		}
	}
	group(c, "new-conversation", style.T(c, "conversations.new"), style.T(c, "conversations.newFooter"), func(g *settingsGroup) {
		g.row(style.T(c, "conversations.model"), "", nil, func() {
			if modelPicker(c, style.T(c, "conversations.model"), style.T(c, "conversations.model"), &chosen, options, false, 288) {
				st.model = chosen
			}
		})
		g.block(func() {
			ui.Column(c).Gap(12).AlignItems(ui.Stretch).Children(func() {
				instructions := style.T(c, "conversations.instructions")
				ui.TextArea(c, &st.instructions).FillWidth().MinHeight(64).Label(instructions).
					Placeholder(instructions + " (" + style.T(c, "conversations.instructionsPlaceholder") + ")")
				if a.composer(c, "new", &st.first, style.T(c, "conversations.placeholder", "model", label),
					style.T(c, "conversations.start"), st.creating, chosen == "") {
					a.startConversation(chosen)
				}
				if st.createErr != nil {
					style.Text(ui.Text(c, errorMessage(c, st.createErr)), style.Type(c).Callout).TextColor(style.Color(colors.Danger)).Selectable()
				}
			})
		})
	})
}

// startConversation starts a conversation with the model chosen, by
// "provider/model", and sends its first message on its page.
func (a *App) startConversation(model string) {
	st := &a.settingsState.admin.ai.conv
	message := strings.TrimSpace(st.first)
	provider, id, ok := strings.Cut(model, "/")
	if !ok || message == "" || st.creating {
		return
	}
	title, _, _ := strings.Cut(message, "\n")
	if utf8.RuneCountInString(title) > 80 {
		title = string([]rune(title)[:80])
	}
	instructions := strings.TrimSpace(st.instructions)
	st.creating, st.createErr = true, nil
	s := a.aiConversationService(a.chosenServer().GetId())
	go func() {
		res, err := s.CreateAiConversation(a.ctx, systemv1.CreateAiConversationRequest_builder{
			Provider: &provider, Model: &id, Instructions: &instructions, Title: &title,
		}.Build())
		a.opts.Update(func() {
			st.creating, st.createErr = false, err
			if err != nil {
				return
			}
			st.first, st.instructions, st.reload = "", "", st.reload+1
			conversation := res.GetConversation().GetId()
			a.router.Replace(conversationPath(conversation))
			a.sendAiMessage(conversation, message)
		})
	}()
}

// sendAiMessage sends a message to a conversation, with the answer shown
// as the model writes it; the conversation loads again with both once
// answered, or with what was kept of them once failed.
func (a *App) sendAiMessage(conversation, text string) {
	st := &a.settingsState.admin.ai.conv
	ex := &aiExchange{conversation: conversation, request: rand.Text(), text: text}
	st.exchange = ex
	mine := func() bool { return st.exchange == ex }
	s := a.aiConversationService(a.chosenServer().GetId())
	go func() {
		stream, err := s.SendAiMessage(a.ctx, systemv1.SendAiMessageRequest_builder{
			ConversationId: &conversation, Text: &text, RequestId: &ex.request,
		}.Build())
		if err == nil {
			for stream.Receive() {
				if m := stream.Msg(); m.HasText() {
					answer := m.GetText()
					a.opts.Update(func() {
						if mine() {
							ex.answer = answer
						}
					})
				}
			}
			err = stream.Err()
			stream.Close()
		}
		a.opts.Update(func() {
			st.reload++
			if !mine() {
				return
			}
			if err != nil {
				ex.answer, ex.err = "", err
				return
			}
			st.exchange = nil
		})
	}()
}

// composer builds a field for a message with its button sending it at its
// end, as the web UI's: the field grows with its text, and ⌘↩ sends it as
// the button does. It reports a send.
func (a *App) composer(c *ui.Context, key string, value *string, label, action string, busy, disabled bool) bool {
	colors := style.Colors(c)
	send := false
	empty := strings.TrimSpace(*value) == ""
	box := ui.Row(c.Key("composer-"+key)).Gap(8).Padding(6, 6, 6, 0).AlignItems(ui.End).Radius(design.RadiusMd).
		Border(1, style.Color(colors.Border)).Background(style.Color(colors.Background))
	box.Children(func() {
		f := ui.TextAreaBase(c, value).Grow(1).MinWidth(0).MinHeight(56).MaxHeight(240).Padding(2, 0, 0, 12).
			Placeholder(label).Label(label)
		if f.Focused() {
			box.Border(1, style.Color(colors.Accent))
		}
		if f.Shortcut(ui.Cmd, ui.KeyEnter) && !empty && !busy && !disabled {
			send = true
		}
		b := ui.ButtonBase(c.Key("send")).Size(28, 28).Radius(14).Center().Shrink(0).Label(action).Tooltip(action + " (⌘↩)").
			Disabled(disabled || empty || busy).Background(style.Color(colors.Button))
		if disabled || empty || busy {
			b.Opacity(0.4)
		} else if b.Hovered() {
			b.Opacity(0.85)
		}
		b.Children(func() {
			if busy {
				ui.Spinner(c).Size(16, 16).TextColor(style.Color(colors.OnButton))
			} else {
				ui.Icon(c, style.Icon("send", false)).Size(18, 18).TextColor(style.Color(colors.OnButton))
			}
		})
		if b.Clicked() && !disabled && !empty && !busy {
			send = true
		}
	})
	return send
}

func (a *App) conversation(id string) *task[*systemv1.GetAiConversationResponse] {
	st := &a.settingsState.admin.ai.conv
	return adminTask(a, &st.one, "ai-conversation", func(ctx context.Context, s string) (*systemv1.GetAiConversationResponse, error) {
		return a.aiConversationService(s).GetAiConversation(ctx, systemv1.GetAiConversationRequest_builder{Id: &id}.Build())
	}, id, st.reload)
}

// conversationPage builds a conversation's page, below the AI settings, as
// the web UI's: its messages, the user's on the right and the model's on
// the left under the model, the answer shown as the model writes it, then
// the field for the next message and Delete Conversation.
func (a *App) conversationPage(c *ui.Context, id string) {
	st := &a.settingsState.admin.ai.conv
	colors := style.Colors(c)
	a.backTo(c, style.T(c, "settings.ai"), aiPath(""))
	res := a.conversation(id)
	s := res.value.GetConversation()
	if res.value != nil && s.GetId() != id {
		s = nil
	}
	if s == nil {
		sectionHeader(c, style.T(c, "settings.conversations"), "")
		switch {
		case res.err != nil:
			dangerText(c, errorMessage(c, res.err))
		case res.loaded:
			secondaryText(c, style.T(c, "conversations.gone"))
		}
		return
	}
	options := modelOptions(c, "", a.aiModels().value, a.providerNames())
	model := conversationModel(options, s)
	sectionHeader(c, conversationTitle(c, s), modelName(model)+" · "+usageOf(c, s.GetUsage()))

	// The user's messages are the caller's, or those of the user whose
	// conversation it is: another's assistant conversation, or one another
	// administrator started.
	asker := style.T(c, "conversations.you")
	if owner := s.GetOwner(); owner != "" && owner != a.chosenServer().GetUserId() {
		asker = style.T(c, "conversations.user")
		for _, u := range a.users().value {
			if u.GetId() == owner {
				asker = u.GetName()
			}
		}
	}
	assistant := isAssistantConversation(s)
	ex := st.exchange
	if ex != nil && ex.conversation != id {
		ex = nil
	}
	messages := res.value.GetMessages()
	ui.Column(c).Gap(24).AlignItems(ui.Stretch).Children(func() {
		for i, m := range messages {
			user := m.GetRole() == pluginv1.AiRole_AI_ROLE_USER
			context, text := partedMessage(user && assistant, m.GetText())
			var who *modelOption
			if !user {
				who = &model
			}
			a.conversationBubble(c, strconv.Itoa(i), who, asker, context, text, false, m)
		}
		// A failed message shows as the conversation kept it.
		if ex != nil && ex.err == nil {
			a.conversationBubble(c, "sending", nil, asker, "", ex.text, false, nil)
			a.conversationBubble(c, "answer", &model, asker, "", ex.answer, true, nil)
		}
	})
	if ex != nil && ex.err != nil {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
			style.Text(ui.Text(c, errorMessage(c, ex.err)), style.Type(c).Callout).TextColor(style.Color(colors.Danger)).Selectable()
			if smallButton(c, style.T(c, "conversations.retry"), false) {
				a.sendAiMessage(id, ex.text)
			}
		})
	}
	sending := ex != nil && ex.err == nil
	end := ui.Column(c.Key("conversation-end")).AlignItems(ui.Stretch)
	end.Children(func() {
		if a.composer(c, "message", &st.text, style.T(c, "conversations.placeholder", "model", model.label),
			style.T(c, "conversations.send"), sending, false) {
			text := strings.TrimSpace(st.text)
			st.text = ""
			a.sendAiMessage(id, text)
		}
	})
	// The end in view as messages come and the answer grows.
	shown := fmt.Sprint(id, len(messages), ex != nil)
	if ex != nil {
		shown += ex.text + ex.answer
	}
	if shown != st.shown {
		st.shown = shown
		end.ScrollIntoView()
	}
	ui.Row(c).Children(func() {
		if dangerButton(c, style.T(c, "conversations.delete"), st.deleting) {
			st.confirming = true
		}
	})
	if st.deleteErr != nil {
		dangerText(c, errorMessage(c, st.deleteErr))
	}
	if ui.AlertDialog(c, &st.confirming, style.T(c, "conversations.deleteConfirm"), style.T(c, "conversations.deleteMessage"),
		style.T(c, "action.cancel"), style.T(c, "conversations.delete")) == 1 {
		st.deleting, st.deleteErr = true, nil
		svc := a.aiConversationService(a.chosenServer().GetId())
		go func() {
			_, err := svc.DeleteAiConversation(a.ctx, systemv1.DeleteAiConversationRequest_builder{Id: &id}.Build())
			a.opts.Update(func() {
				st.deleting, st.deleteErr = false, err
				if err == nil {
					st.reload++
					a.router.Replace(aiPath(""))
				}
			})
		}()
	}
}

// conversationBubble builds a message of a conversation, as the web UI's:
// the user's on the right under who asked, the model's on the left under
// the model as the picker's rows show it, each short of the other side;
// the context line of a user's message to the assistant above its text,
// and the bar copying it and telling its time under it, once answered.
func (a *App) conversationBubble(c *ui.Context, key string, who *modelOption, asker, context, text string, pending bool, m *pluginv1.AgentMessage) {
	colors := style.Colors(c)
	user := who == nil
	row := ui.Column(c.Key("message-" + key)).Gap(4).FillWidth()
	if user {
		row.AlignItems(ui.End).Padding(0, 0, 0, 96)
	} else {
		row.AlignItems(ui.Start).Padding(0, 96, 0, 0)
	}
	row.Children(func() {
		if who != nil {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(0, 4).MaxWidthPercent(100).Children(func() {
				modelLabel(c, *who, style.Color(colors.TextPrimary))
			})
		} else {
			style.Text(ui.Text(c, asker), style.Type(c).Caption).TextColor(style.Color(colors.TextTertiary)).Padding(0, 4)
		}
		b := ui.Column(c).MaxWidth(672).Padding(8, 12).Radius(design.RadiusLg).Background(style.Color(colors.Surface))
		if !user {
			b.Border(1, style.Color(colors.Accent).Alpha(0.5)).BorderStyle(ui.BorderDashed)
		}
		b.Children(func() {
			if context != "" {
				style.Text(ui.Text(c, context), style.Type(c).Caption).Font("SF Mono, Menlo, monospace").
					TextColor(style.Color(colors.TextTertiary)).Selectable()
				ui.Box(c).Height(1).FillWidth().Margin(6, 0).Background(style.Color(colors.Border))
			}
			shown := text
			if shown == "" && pending {
				shown = "…"
			}
			style.Text(ui.Text(c, shown), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
		})
		if !pending && text != "" {
			var at time.Time
			if m.HasTime() {
				at = m.GetTime().AsTime()
			}
			a.messageBar(c, "conversation-"+key, text, at)
		}
	})
}

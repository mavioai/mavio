package app

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// assistantPrefsState is the state of this device's Assistant section
// (docs/assistant.md §9.1): what the assistant of the server chosen
// remembers about the user, and the user's conversations with it.
type assistantPrefsState struct {
	info          task[*assistantv1.GetAssistantResponse]
	memories      task[[]*assistantv1.AssistantMemory]
	conversations task[[]*assistantv1.AssistantConversation]
	conversation  task[*assistantv1.GetAssistantConversationResponse]
	// reload loads the memories and conversations again after a change.
	reload int

	// shown is the conversation whose messages show.
	shown string
	// text is the memory typed to add.
	text   string
	busy   bool
	err    error
	asking bool
}

// assistantPrefs builds this device's Assistant section, as the web UI's:
// the assistant's memories of the user, and the user's conversations with
// it, on the server chosen. A voice session's microphone is the system's,
// which the desktop app has no voice sessions to choose for.
func (a *App) assistantPrefs(c *ui.Context) {
	st := &a.settingsState.assistant
	sectionHeader(c, style.T(c, "assistant.title"), style.T(c, "assistant.settingsMessage"))
	server := a.chosenServer().GetId()
	if server == "" {
		return
	}
	lang := c.Locale()
	info := load(a, &st.info, key("assistant-prefs", server, lang, a.generation), func(ctx context.Context) (*assistantv1.GetAssistantResponse, error) {
		return a.assistantService(server).GetAssistant(ctx, assistantv1.GetAssistantRequest_builder{Language: &lang}.Build())
	})
	switch {
	case info.err != nil:
		dangerText(c, errorMessage(c, info.err))
		return
	case info.value == nil:
		return
	case !info.value.GetEnabled():
		group(c, "assistant-off", "", "", func(g *settingsGroup) {
			g.row(style.T(c, "assistant.off"), "", nil, nil)
		})
		return
	}
	a.assistantMemories(c, server, info.value.GetAi(), info.value.GetMemory())
	if info.value.GetAi() {
		a.assistantConversations(c, server)
	}
}

// assistantChange runs a change of the memories or conversations, which
// load again once it ends.
func (a *App) assistantChange(server string, fn func(ctx context.Context, s string) error, done func()) {
	st := &a.settingsState.assistant
	st.busy, st.err = true, nil
	go func() {
		err := fn(a.ctx, server)
		a.opts.Update(func() {
			st.busy, st.err = false, err
			st.reload++
			if err == nil && done != nil {
				done()
			}
		})
	}()
}

func (a *App) assistantMemories(c *ui.Context, server string, ai, on bool) {
	st := &a.settingsState.assistant
	if !on {
		label := "assistant.memoryNoAi"
		if ai {
			label = "assistant.memoryOff"
		}
		group(c, "memories", style.T(c, "assistant.memory"), "", func(g *settingsGroup) {
			g.row(style.T(c, label), "", nil, nil)
		})
		return
	}
	memories := load(a, &st.memories, key("assistant-memories", server, a.generation, st.reload), func(ctx context.Context) ([]*assistantv1.AssistantMemory, error) {
		res, err := a.assistantService(server).ListAssistantMemories(ctx, &assistantv1.ListAssistantMemoriesRequest{})
		return res.GetMemories(), err
	})
	forget := func(ids ...string) {
		a.assistantChange(server, func(ctx context.Context, s string) error {
			_, err := a.assistantService(s).DeleteAssistantMemories(ctx, assistantv1.DeleteAssistantMemoriesRequest_builder{Ids: ids}.Build())
			return err
		}, func() { st.asking = false })
	}
	group(c, "memories", style.T(c, "assistant.memory"), style.T(c, "assistant.memoryFooter"), func(g *settingsGroup) {
		for _, m := range memories.value {
			detail := ""
			if m.HasUpdateTime() {
				detail = mediumDateTime(m.GetUpdateTime().AsTime())
			}
			g.row(m.GetText(), detail, nil, func() {
				moreMenu(c, "memory-"+m.GetId(), func(close func()) {
					if menuDangerItem(c, style.T(c, "assistant.forget")) {
						close()
						forget(m.GetId())
					}
				})
			})
		}
		if memories.loaded && memories.err == nil && len(memories.value) == 0 {
			g.row(style.T(c, "assistant.noMemories"), "", nil, nil)
		}
		g.divider()
		ui.Row(c.Key("new-memory")).Padding(10, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			label := style.T(c, "assistant.newMemory")
			f := ui.TextInput(c, &st.text).Grow(1).MinWidth(0).Placeholder(label).Label(label)
			text := strings.TrimSpace(st.text)
			add := smallButton(c, style.T(c, "assistant.remember"), text == "" || st.busy)
			if (add || f.Submitted()) && text != "" && !st.busy {
				a.assistantChange(server, func(ctx context.Context, s string) error {
					_, err := a.assistantService(s).UpdateAssistantMemory(ctx, assistantv1.UpdateAssistantMemoryRequest_builder{Text: &text}.Build())
					return err
				}, func() { st.text = "" })
			}
			if len(memories.value) > 0 {
				moreMenu(c, "memories", func(close func()) {
					if menuDangerItem(c, style.T(c, "assistant.forgetAll")) {
						close()
						st.asking = true
					}
				})
			}
		})
		for _, err := range []error{st.err, memories.err} {
			if err != nil {
				g.note(errorMessage(c, err), true)
				break
			}
		}
	})
	if ui.AlertDialog(c, &st.asking, style.T(c, "assistant.forgetAllConfirm"), "", style.T(c, "action.cancel"), style.T(c, "assistant.forgetAll")) == 1 {
		forget()
	}
}

func (a *App) assistantConversations(c *ui.Context, server string) {
	st := &a.settingsState.assistant
	colors := style.Colors(c)
	list := load(a, &st.conversations, key("assistant-conversations", server, a.generation, st.reload), func(ctx context.Context) ([]*assistantv1.AssistantConversation, error) {
		res, err := a.assistantService(server).ListAssistantConversations(ctx, &assistantv1.ListAssistantConversationsRequest{})
		return res.GetConversations(), err
	})
	group(c, "assistant-conversations", style.T(c, "assistant.conversations"), style.T(c, "assistant.conversationsFooter"), func(g *settingsGroup) {
		for _, conv := range list.value {
			// The assistant's conversations have no titles: they go by when
			// they were, and where.
			when := ""
			if conv.HasUpdateTime() {
				when = mediumDateTime(conv.GetUpdateTime().AsTime())
			}
			label, detail := conv.GetTitle(), []string{conv.GetDeviceName()}
			if label == "" {
				label = when
			} else {
				detail = append(detail, when)
			}
			id := conv.GetId()
			g.divider()
			b := ui.ButtonBase(c.Key("conversation-"+id)).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Label(label)
			if b.Hovered() {
				b.Background(style.Color(colors.Fill))
			}
			b.Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
					if d := joinDetail(detail...); d != "" {
						style.Text(ui.Text(c, d), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
					}
				})
			})
			if b.Clicked() {
				if st.shown == id {
					st.shown = ""
				} else {
					st.shown = id
				}
			}
			if st.shown == id {
				a.assistantMessages(c, server, id)
			}
		}
		if list.loaded && list.err == nil && len(list.value) == 0 {
			g.row(style.T(c, "assistant.noConversations"), "", nil, nil)
		}
		if list.err != nil {
			g.note(errorMessage(c, list.err), true)
		}
	})
}

// assistantMessages builds a conversation's messages under its row, as the
// web UI's: the user's on the right in the button's color, the answers on
// the left in the fill, each with the bar copying it and telling its time;
// then Delete Conversation.
func (a *App) assistantMessages(c *ui.Context, server, id string) {
	st := &a.settingsState.assistant
	colors := style.Colors(c)
	res := load(a, &st.conversation, key("assistant-conversation", server, id, a.generation), func(ctx context.Context) (*assistantv1.GetAssistantConversationResponse, error) {
		return a.assistantService(server).GetAssistantConversation(ctx, assistantv1.GetAssistantConversationRequest_builder{Id: &id}.Build())
	})
	ui.Column(c.Key("messages-"+id)).Padding(12, 16).Gap(16).AlignItems(ui.Stretch).Children(func() {
		if res.value.GetConversation().GetId() == id {
			for i, m := range res.value.GetMessages() {
				row := ui.Column(c.Key(i)).Gap(2).FillWidth()
				if m.GetUser() {
					row.AlignItems(ui.End).Padding(0, 0, 0, 96)
				} else {
					row.AlignItems(ui.Start).Padding(0, 96, 0, 0)
				}
				row.Children(func() {
					bg, fg := style.Color(colors.Fill), style.Color(colors.TextPrimary)
					if m.GetUser() {
						bg, fg = style.Color(colors.Button), style.Color(colors.OnButton)
					}
					ui.Box(c).Padding(8, 12).Radius(14).Background(bg).Children(func() {
						style.Text(ui.Text(c, m.GetText()), style.Type(c).Callout).TextColor(fg).Selectable()
					})
					var at time.Time
					if m.HasTime() {
						at = m.GetTime().AsTime()
					}
					a.messageBar(c, "assistant-"+id+"-"+strconv.Itoa(i), m.GetText(), at)
				})
			}
		}
		if res.err != nil {
			dangerText(c, errorMessage(c, res.err))
		}
		ui.Row(c).Children(func() {
			label := style.T(c, "assistant.deleteConversation")
			b := ui.ButtonBase(c.Key("delete")).MinHeight(28).Padding(0, 8).Radius(design.RadiusMd).AlignItems(ui.Center).Label(label).Disabled(st.busy)
			if b.Hovered() {
				b.Background(style.Color(colors.Fill))
			}
			b.Children(func() {
				style.Text(ui.Text(c, label), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.Danger))
			})
			if b.Clicked() && !st.busy {
				a.assistantChange(server, func(ctx context.Context, s string) error {
					_, err := a.assistantService(s).DeleteAssistantConversation(ctx, assistantv1.DeleteAssistantConversationRequest_builder{Id: &id}.Build())
					return err
				}, func() { st.shown = "" })
			}
		})
	})
}

// moreMenu builds a button of three dots, as the web UI's row menus,
// opening a menu which fill builds; close closes it.
func moreMenu(c *ui.Context, key string, fill func(close func())) {
	colors := style.Colors(c)
	label := style.T(c, "item.more")
	b := ui.ButtonBase(c.Key("more-"+key)).Size(32, 32).Radius(16).Center().Shrink(0).Label(label).Tooltip(label)
	open := ui.Local(b, "open", func() bool { return false })
	if *open || b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon("moreVertical", false)).Size(20, 20).TextColor(style.Color(colors.TextSecondary))
	})
	if b.Clicked() {
		*open = !*open
	}
	ui.PopoverBase(c, b, open, func(panel ui.Element) {
		menuPanel(c, panel, 192, 0.9, 24, func() {
			fill(func() { *open = false })
		})
	})
}

// menuDangerItem builds an item of a menu that removes something, its
// text the danger color, and reports whether it was chosen.
func menuDangerItem(c *ui.Context, text string) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key("item-"+text)).MinHeight(28).Padding(4, 8).Radius(6).AlignItems(ui.Center).Label(text)
	if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		style.Text(ui.Text(c, text), style.Type(c).Body).TextColor(style.Color(colors.Danger)).SingleLine().Grow(1)
	})
	return b.Clicked()
}

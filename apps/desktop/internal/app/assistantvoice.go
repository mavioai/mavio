package app

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/encoding/protojson"

	design "github.com/mavioai/mavio/libs/design/gen/go"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
	"github.com/mavioai/mavio/apps/desktop/internal/voice"
)

// The voice assistant (docs/assistant.md §9.1, §9.5;
// libs/design/components.md, Assistant), as the web UI's: an orb floating
// at the top right shows the voice session's state, and cards below it
// what the user said, the answer with its words lit as they are spoken,
// the tools run, results to choose from and a pending confirmation. The
// assistant button, the orb or V tapped starts a voice session and tapped
// again ends it; the button or the orb held talks while held. It goes away
// on its own a few seconds after the session ends, and by a click outside
// it or Escape.

// holdTime is how long the button must be held to talk while held, rather
// than start a conversation.
const holdTime = 400 * time.Millisecond

// leaveTime is how long the orb and cards take to fade out.
const leaveTime = design.DurationSlow

// assistantHold is a press of the assistant's button or orb: held once it
// lasted holdTime.
type assistantHold struct {
	held  bool
	timer *time.Timer
}

// voiceOn reports whether the app has voice sessions.
func (a *App) voiceOn() bool { return a.opts.Voice != nil }

// pressAssistant tells that the button, the orb or V went down in win,
// nil for the app's window.
func (a *App) pressAssistant(win Window, lang string) {
	st := &a.assistant
	v := st.voice
	// A conversation ends when pressed again.
	if v != nil && !v.pushToTalk {
		a.closeAssistant()
		return
	}
	h := &assistantHold{}
	st.hold = h
	h.timer = time.AfterFunc(holdTime, func() {
		a.opts.Update(func() {
			if st.hold != h {
				return
			}
			h.held = true
			switch s := st.voice; {
			case s != nil && s.pushToTalk:
				s.talk(true)
				st.talking = true
			case s != nil:
				a.connectVoice(true, lang)
			}
		})
	})
	if v == nil {
		a.captureVoice(win, lang)
	}
}

// releaseAssistant tells that the button, the orb or V went up.
func (a *App) releaseAssistant(lang string) {
	st := &a.assistant
	h := st.hold
	st.hold = nil
	if h == nil {
		return
	}
	h.timer.Stop()
	v := st.voice
	switch {
	case h.held:
		if v != nil {
			v.talk(false)
		}
		st.talking = false
	// A tap: a conversation starts, or a session held to talk ends.
	case v != nil && v.pushToTalk:
		a.closeAssistant()
	case v != nil:
		a.connectVoice(false, lang)
	}
}

// tapAssistant presses and releases the assistant, as a key does.
func (a *App) tapAssistant(win Window, lang string) {
	a.pressAssistant(win, lang)
	a.releaseAssistant(lang)
}

// captureVoice opens the microphone, and the voice session once it is
// known whether the user holds to talk.
func (a *App) captureVoice(win Window, lang string) {
	st := &a.assistant
	server := a.assistantServer()
	if server == "" {
		return
	}
	if st.cancel != nil {
		st.cancel()
	}
	st.run++
	st.leave++
	st.opening++
	n := st.opening
	st.open, st.win, st.server, st.leaving, st.lingering = true, win, server, false, 0
	st.heard, st.heardAt, st.answer, st.answerAt, st.activity, st.err = "", time.Time{}, "", time.Time{}, nil, ""
	st.working, st.confirmation, st.waits = false, nil, false
	if !st.choose {
		st.items = nil
	}
	if !st.info.value.GetVoice() {
		st.err = "assistant.noVoice"
		return
	}
	st.turn = ""
	st.listening, st.capturing, st.userSpeaking, st.speaking, st.voiced, st.pushToTalk, st.talking = true, false, false, false, false, false, false
	h := voiceHandlers{
		event: func(e voiceEvent) {
			if st.opening == n {
				a.voiceEvent(e)
			}
		},
		closed: func(err string) {
			if st.opening != n {
				return
			}
			st.voice = nil
			st.listening, st.capturing, st.speaking, st.userSpeaking, st.working = false, false, false, false, false
			if err != "" {
				st.err = err
				return
			}
			a.linger(st.run)
		},
		capturing: func() {
			if st.opening == n {
				st.capturing = true
			}
		},
		silent: func(device string) {
			if st.opening == n {
				st.err = "assistant.silentMicrophone:" + device
			}
		},
	}
	audio, mic := a.opts.Voice, a.prefs.Microphone
	go func() {
		v, err := openVoice(a.ctx, audio, mic, a.opts.Update, h)
		a.opts.Update(func() {
			if st.opening != n || !st.listening || st.leaving {
				if v != nil {
					v.close()
				}
				return
			}
			if err != nil {
				st.listening = false
				if errors.Is(err, voice.ErrDenied) {
					st.err = "assistant.microphoneDenied"
				} else {
					st.err = "assistant.noMicrophone:" + err.Error()
				}
				return
			}
			st.voice = v
			// Let go before the microphone opened: a tap.
			if st.hold == nil {
				a.connectVoice(false, lang)
			} else if st.hold.held {
				a.connectVoice(true, lang)
			}
		})
	}()
}

// connectVoice starts the session on the server: a conversation, or
// talking while held.
func (a *App) connectVoice(pushToTalk bool, lang string) {
	st := &a.assistant
	v := st.voice
	if v == nil || v.connecting {
		return
	}
	server := st.server
	mode := pluginv1.VoiceMode_VOICE_MODE_CONVERSATION
	if pushToTalk {
		mode = pluginv1.VoiceMode_VOICE_MODE_PUSH_TO_TALK
	}
	req := assistantv1.StartVoiceSessionRequest_builder{
		Language: new(lang), Mode: &mode, Context: a.askContext(slices.Compact([]string{lang, a.prefs.Language})),
	}.Build()
	v.connect(pushToTalk, func(ctx context.Context) (string, error) {
		res, err := a.assistantService(server).StartVoiceSession(ctx, req)
		if err != nil {
			if e := coreError(err); e != nil && e.Reason != "" {
				return "", errors.New(e.Reason)
			}
			return "", err
		}
		u := a.serverURLOf(server, res.GetUrl())
		return "ws" + strings.TrimPrefix(u, "http"), nil
	})
	st.pushToTalk, st.talking, st.sentPath = pushToTalk, pushToTalk, a.router.Path()
	if pushToTalk {
		v.talk(true)
	}
}

// voiceEvent carries out what the voice session tells (§8.6).
func (a *App) voiceEvent(e voiceEvent) {
	st := &a.assistant
	text := e.str("text")
	switch e.str("type") {
	case "user_started":
		st.lingering = 0
		st.userSpeaking = true
	case "user_stopped":
		st.userSpeaking = false
	case "interrupt":
		st.interrupts++
		st.speaking, st.userSpeaking = false, true
	case "transcript":
		said := func(s ...string) string {
			return strings.Join(slices.DeleteFunc(s, func(x string) bool { return x == "" }), " ")
		}
		if e.flag("final") {
			st.turn = said(st.turn, strings.TrimSpace(text))
			st.heard = st.turn
		} else {
			st.heard = said(st.turn, strings.TrimSpace(text))
		}
		st.heardAt = time.Time{}
	case "run":
		st.turn = ""
		if text != "" {
			st.heard = text
		}
		st.heardAt, st.working, st.userSpeaking, st.voiced = time.Now(), true, false, false
		st.activity, st.answer, st.answerAt, st.err, st.confirmation = nil, "", time.Time{}, "", nil
		if !st.choose {
			st.items = nil
		}
	case "intent":
		if text != "" {
			st.heard = text
		}
	case "tool":
		id := e.str("id")
		st.activity = slices.DeleteFunc(st.activity, func(l toolLine) bool { return l.id == id })
		st.activity = append(st.activity, toolLine{id, e.str("summary"), e.flag("done"), e.flag("failed")})
	case "action":
		b, err := json.Marshal(e["action"])
		if err != nil {
			break
		}
		act := &assistantv1.ClientAction{}
		if (protojson.UnmarshalOptions{DiscardUnknown: true}).Unmarshal(b, act) == nil {
			a.perform(st.server, act)
		}
	case "confirm":
		st.confirmation = assistantv1.Confirmation_builder{Id: new(e.str("id")), Text: new(text)}.Build()
	case "answer":
		st.answer += text
	case "end":
		st.working, st.answerAt = false, time.Now()
		if s := e.str("answer"); s != "" {
			st.answer = s
		}
		if e.str("status") == "failed" && st.answer == "" {
			st.err = "assistant.cantDo"
		}
	case "bot_started":
		st.speaking, st.voiced = true, true
	case "bot_stopped":
		st.speaking = false
	case "error":
		st.working = false
		st.err = text
		if m := e.str("message"); m != "" {
			st.err = m
		} else if st.err == "" {
			st.err = e.str("code")
		}
	}
}

// voiceContext sends the page's context to the session as it changes.
func (a *App) voiceContext(lang string) {
	v := a.assistant.voice
	if v == nil || !v.connecting {
		return
	}
	b, err := protojson.Marshal(a.askContext(slices.Compact([]string{lang, a.prefs.Language})))
	if err != nil {
		return
	}
	v.send(voiceEvent{"type": "context", "context": json.RawMessage(b)})
}

// orbMode is the orb's state, from the session's.
func (a *App) orbMode() orbMode {
	st := &a.assistant
	switch {
	case st.leaving:
		return orbEnded
	case st.err != "":
		return orbError
	case st.speaking:
		return orbSpeaking
	case st.userSpeaking:
		return orbHearing
	case st.working:
		return orbThinking
	case st.listening && !st.capturing:
		return orbOpening
	case st.listening && (!st.pushToTalk || st.talking):
		return orbIdle
	}
	return orbResting
}

// assistantMessage is the text of the assistant's error or note: a
// string's key, with its argument after a colon, or a message as it is.
func (a *App) assistantMessage(c *ui.Context, msg string) string {
	key, arg, _ := strings.Cut(msg, ":")
	switch key {
	case "assistant.cantDo", "assistant.noVoice", "assistant.microphoneDenied":
		return style.T(c, key)
	case "assistant.noMicrophone":
		return style.T(c, key, "error", arg)
	case "assistant.silentMicrophone":
		if arg == "" {
			arg = style.T(c, "assistant.microphone")
		}
		return style.T(c, key, "device", arg)
	}
	if msg == errUnreachable.Error() {
		name := ""
		for _, s := range a.servers {
			if s.GetId() == a.assistant.server {
				name = s.GetName()
			}
		}
		return style.T(c, "home.serverUnreachable", "server", name)
	}
	return msg
}

// voicePress builds the press of an element starting or ending a voice
// session, held or tapped: el is the assistant's button or the orb, which
// key names.
func (a *App) voicePress(c *ui.Context, el ui.Element, key string, win Window) {
	st := &a.assistant
	lang := c.Locale()
	pressed := el.Pressed()
	switch {
	case pressed && st.down == "":
		st.down = key
		a.pressAssistant(win, lang)
	case !pressed && st.down == key:
		st.down = ""
		a.releaseAssistant(lang)
	// A keyboard's press taps it.
	case el.Clicked() && st.down == "":
		a.tapAssistant(win, lang)
	}
}

// voiceOverlay builds the orb and the cards over the window win, nil for
// the app's.
func (a *App) voiceOverlay(c *ui.Context, win Window) {
	st := &a.assistant
	colors := style.Colors(c)
	lang := c.Locale()
	if c.Shortcut(0, ui.KeyEscape) {
		a.closeAssistant()
		return
	}
	_, h := c.Size()
	opacity := float32(1)
	if st.leaving {
		opacity = 0
	}
	material := style.Color(colors.SurfaceOverlay).Alpha(0.72)
	if !c.Theme().Dark {
		material = ui.RGB(255, 255, 255).Alpha(0.74)
	}
	card := func(key string) ui.Element {
		return ui.Column(c.Key(key)).Radius(design.RadiusXl).Border(1, style.Color(colors.Border)).Background(material).
			Material(glass.Blur{Radius: 28}).Shadow(0, 12, 32, 0, style.Color(design.ElevationLevel3.Color))
	}
	mode := a.orbMode()
	var status string
	switch {
	case mode == orbOpening:
		status = style.T(c, "state.loading")
	case mode == orbIdle && st.heard == "":
		status = style.T(c, "assistant.listening")
	}
	root := ui.Box(c.Key("voice")).Absolute().Top(0).Right(0).Bottom(0).Left(0).PassThrough()
	root.Opacity(root.Animate("fade", opacity, leaveTime))
	root.Children(func() {
		top := ui.Row(c.Key("voice-orb")).Absolute().Top(12).Right(12).Gap(8).AlignItems(ui.Center)
		top.Children(func() {
			if status != "" {
				style.Text(ui.Text(c, status), style.Type(c).Footnote).TextColor(style.Color(colors.TextPrimary)).
					Padding(4, 12).Radius(design.RadiusFull).Background(material).Border(1, style.Color(colors.Border)).
					Material(glass.Blur{Radius: 24}).SingleLine()
			}
			label := style.T(c, "assistant.talk")
			if st.listening {
				label = style.T(c, "assistant.stopListening")
			}
			scale := float32(1)
			if st.leaving {
				scale = 0.4
			}
			b := ui.ButtonBase(c.Key("orb-button")).Size(orbSize, orbSize).Radius(orbSize / 2).Label(label)
			k := b.Animate("scale", scale, leaveTime)
			b.Scale(k, k)
			b.Children(func() {
				orb(c, &st.orb, mode, a.orbLevel, st.interrupts)
			})
			a.voicePress(c, b, "orb", win)
		})
		cards := ui.Column(c.Key("voice-cards")).Absolute().Top(80).Right(16).Width(380).MaxHeight(h - 96).Gap(8).PassThrough()
		cards.Children(func() {
			ui.Scroll(c.Key("voice-scroll")).Shrink(1).Children(func() {
				ui.Column(c).Padding(4).Gap(8).Children(func() { a.voiceCards(c, card, lang) })
			})
		})
		// Over the cards, the assistant stays.
		hovered := cards.Hovered()
		switch {
		case hovered && !st.hovered:
			st.lingering = 0
		case !hovered && st.hovered && st.voice == nil && !st.working && !st.leaving:
			a.linger(st.run)
		}
		st.hovered = hovered
		// A click outside the orb and the cards dismisses them.
		if top.PressedOutside() && cards.PressedOutside() && st.down == "" && !st.leaving {
			a.closeAssistant()
		}
	})
}

// orbLevel is the audio the orb follows: the playback's while the answer
// is spoken, else the microphone's.
func (a *App) orbLevel() float32 {
	st := &a.assistant
	switch v := st.voice; {
	case v == nil:
		return 0
	case st.speaking:
		return v.outputLevel()
	default:
		return v.inputLevel()
	}
}

// voiceCards builds the cards: the off note, what was said and the
// answer, the tools, the results, a confirmation and an error.
func (a *App) voiceCards(c *ui.Context, card func(string) ui.Element, lang string) {
	st := &a.assistant
	colors := style.Colors(c)
	body := style.Type(c).Body
	if st.info.err == nil && st.info.value != nil && !st.info.value.GetEnabled() {
		card("card-off").Padding(14, 16).Children(func() {
			style.Text(ui.Text(c, style.T(c, "assistant.off")), body).TextColor(style.Color(colors.TextSecondary))
		})
	}
	if st.heard != "" || st.answer != "" {
		card("card-talk").Padding(14, 16).Gap(10).Children(func() {
			if st.heard != "" {
				spans := []ui.Span{{Text: st.heard, Color: style.Color(colors.TextSecondary)}}
				if !st.userSpeaking && !st.heardAt.IsZero() {
					spans = append(spans, ui.Span{Text: " ›", Color: style.Color(colors.TextTertiary)})
				}
				style.Text(ui.RichText(c, spans...), body).Selectable()
			}
			if st.answer != "" {
				a.spoken(c, st.answer, st.voiced && (st.speaking || st.working || st.answerAt.IsZero()))
			}
		})
	}
	if len(st.activity) > 0 {
		card("card-tools").Padding(12, 14).Gap(8).Children(func() {
			for _, l := range st.activity {
				ui.Row(c.Key(l.id)).Gap(10).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Size(26, 26).Shrink(0).Radius(design.RadiusSm).Center().Background(style.Color(colors.Accent).Alpha(0.16)).Children(func() {
						ui.Icon(c, style.Icon("search", false)).Size(14, 14).TextColor(style.Color(colors.Accent))
					})
					style.Text(ui.Text(c, l.summary), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Grow(1).MinWidth(0)
					switch {
					case l.failed:
						ui.Icon(c, style.Icon("close", false)).Size(16, 16).TextColor(style.Color(colors.Danger))
					case l.done:
						ui.Icon(c, style.Icon("check", false)).Size(16, 16).TextColor(style.Color(colors.Success))
					default:
						activityIndicator(c, 16, time.Time{})
					}
				})
			}
		})
	}
	if len(st.items) > 0 {
		card("card-results").Padding(10).Gap(2).Children(func() { a.voiceResults(c) })
	}
	if cf := st.confirmation; cf != nil {
		card("card-confirm").Padding(14, 16).Gap(12).Children(func() {
			style.Text(ui.Text(c, cf.GetText()), body).TextColor(style.Color(colors.TextPrimary))
			ui.Row(c).Gap(8).Children(func() {
				answer := func(ok bool) {
					a.ask(lang, assistantv1.AskRequest_builder{Answer: assistantv1.ConfirmationAnswer_builder{Id: new(cf.GetId()), Confirmed: new(ok)}.Build()}.Build())
				}
				if voiceButton(c, style.T(c, "action.cancel"), false) {
					answer(false)
				}
				if voiceButton(c, style.T(c, "assistant.confirm"), true) {
					answer(true)
				}
			})
		})
	}
	if st.err != "" {
		card("card-error").Padding(14, 16).Children(func() {
			style.Text(ui.Text(c, a.assistantMessage(c, st.err)), body).TextColor(style.Color(colors.Danger)).Selectable()
		})
	}
}

// voiceButton builds a button of a confirmation's card: Confirm the
// accent's, Cancel the fill's.
func voiceButton(c *ui.Context, label string, primary bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key("voice-" + label)).Grow(1).Basis(0).MinHeight(36).Radius(design.RadiusMd).Center().Label(label)
	bg, fg := style.Color(colors.Fill), style.Color(colors.TextPrimary)
	if primary {
		bg, fg = style.Color(colors.Accent), style.Color(colors.OnAccent)
	}
	switch {
	case primary && b.Hovered():
		b.Opacity(0.9)
	case b.Hovered():
		bg = style.Color(colors.FillSelected)
	}
	b.Background(bg).Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Callout).TextColor(fg)
	})
	return b.Clicked()
}

// voiceResults builds the results of an answer: to choose from,
// numbered, or to open.
func (a *App) voiceResults(c *ui.Context) {
	st := &a.assistant
	colors := style.Colors(c)
	for i, it := range st.items {
		card := a.cardOf(st.server, it)
		b := ui.ButtonBase(c.Key("result-" + it.GetId())).Padding(6).Gap(10).Radius(design.RadiusMd).AlignItems(ui.Center).Label(it.GetName())
		if b.Hovered() {
			b.Background(style.Color(colors.Fill))
		}
		b.Children(func() {
			if st.choose {
				style.Text(ui.Text(c, strconv.Itoa(i+1)), style.Type(c).Footnote).FontWeight(600).TextColor(style.Color(colors.TextTertiary)).Width(16).TextAlign(ui.Center).Shrink(0)
			}
			ui.Box(c).Width(34).Height(51).Radius(5).Clip().Shrink(0).Children(func() {
				a.artwork(c, card.GetPoster(), "film", card, 34, 51, false)
			})
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				style.Text(ui.Text(c, it.GetName()), style.Type(c).Callout).FontWeight(500).TextColor(style.Color(colors.TextPrimary)).SingleLine()
				if sub := card.GetSubtitle(); sub != "" {
					style.Text(ui.Text(c, sub), style.Type(c).Caption).TextColor(style.Color(colors.TextSecondary)).SingleLine()
				}
			})
		})
		if b.Clicked() {
			if st.choose {
				a.ask(c.Locale(), assistantv1.AskRequest_builder{Choice: new(it.GetId())}.Build())
			} else {
				a.leavePlayer()
				a.router.Push("/item/" + st.server + "/" + it.GetId())
			}
		}
	}
}

// sayTime is how long a piece of the answer takes to say, roughly: its
// words are lit by these against how much of its speech has played.
func sayTime(word string) time.Duration {
	var ms int
	for _, r := range word {
		switch {
		case unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul):
			ms += 230
		case unicode.IsLetter(r) || unicode.IsNumber(r):
			ms += 62
		case strings.ContainsRune(",.;:!?，。；：！？、", r):
			ms += 180
		}
	}
	return time.Duration(ms) * time.Millisecond
}

// answerWords splits text into its words, each CJK character one, and what is
// between them.
func answerWords(text string) []string {
	var out []string
	cjk := func(r rune) bool {
		return unicode.In(r, unicode.Han, unicode.Hiragana, unicode.Katakana, unicode.Hangul)
	}
	word := func(r rune) bool { return (unicode.IsLetter(r) || unicode.IsNumber(r) || r == '\'') && !cjk(r) }
	start := 0
	rs := []rune(text)
	for i := 1; i <= len(rs); i++ {
		if i < len(rs) && word(rs[i]) && word(rs[i-1]) {
			continue
		}
		out = append(out, string(rs[start:i]))
		start = i
	}
	return out
}

// spoken builds the answer, its words lit as its speech plays; all lit
// when it is not spoken.
func (a *App) spoken(c *ui.Context, text string, live bool) {
	colors := style.Colors(c)
	var played time.Duration
	if v := a.assistant.voice; live && v != nil {
		played, _ = v.dev.Progress()
	}
	var spans []ui.Span
	var at time.Duration
	for _, w := range answerWords(text) {
		at += sayTime(w)
		color := colors.TextPrimary
		if live && at > played+120*time.Millisecond {
			color = colors.TextTertiary
		}
		if n := len(spans); n > 0 && spans[n-1].Color == style.Color(color) {
			spans[n-1].Text += w
			continue
		}
		spans = append(spans, ui.Span{Text: w, Color: style.Color(color)})
	}
	style.Text(ui.RichText(c.Key("answer"), spans...), style.Type(c).Body).Selectable()
	if live {
		a.tick()
	}
}

// tick builds the view again in a moment, while the answer's words light
// up as they are spoken.
func (a *App) tick() {
	st := &a.assistant
	if st.ticking {
		return
	}
	st.ticking = true
	time.AfterFunc(50*time.Millisecond, func() { a.opts.Update(func() { st.ticking = false }) })
}

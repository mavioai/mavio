package app

import (
	"context"
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/durationpb"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"
	assistantv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/assistant/v1/assistantv1connect"
	intentv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/intent/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// lingerTime is how long an answer with nothing to choose stays before
// the panel closes.
const lingerTime = 6 * time.Second

// assistantState is the state of the assistant (docs/assistant.md §9.1):
// over any page, the player's included, it shows the request, the answer,
// results to choose from and a pending confirmation. Where the app has
// voice sessions it is the orb and cards of a voice session
// (assistantvoice.go); elsewhere a panel taking typed requests.
type assistantState struct {
	info task[*assistantv1.GetAssistantResponse]
	open bool
	// win is the window the panel shows in, nil for the app's.
	win Window
	// server is the server asked.
	server string
	// heard is the request, as the server understood it, and heardAt
	// when it was asked.
	heard    string
	heardAt  time.Time
	working  bool
	activity []toolLine
	// answer is the answer, and answerAt when it was done.
	answer   string
	answerAt time.Time
	// copied is the message just copied, "heard" or "answer".
	copied string
	items  []*libraryv1.Item
	choose bool
	// waits tells that the answer waits for the user: a confirmation or
	// results to choose from.
	waits        bool
	confirmation *assistantv1.Confirmation
	err          string
	// text is what is typed in the field.
	text string
	// failed are the actions the last answer could not carry out, which
	// the next request tells.
	failed []string
	// run counts the requests, of which the latest's events count; cancel
	// stops the one running.
	run    int
	cancel context.CancelFunc
	// lingering is the request whose answer closes the panel once it has
	// lingered.
	lingering int

	// The voice session (docs/assistant.md §9.1), where the app has voice
	// (Options.Voice), and its state, as the web UI's.
	voice *voiceSession
	// listening: a session runs; capturing: its microphone's audio came.
	listening, capturing bool
	// speaking: the answer is spoken; voiced: it comes with speech.
	speaking, voiced bool
	// userSpeaking: from user_started to user_stopped.
	userSpeaking bool
	// pushToTalk: the session listens only while held; talking: it is.
	pushToTalk, talking bool
	// interrupts counts the user's barge-ins, each flashing the orb.
	interrupts int
	// leaving: the orb and cards fade out.
	leaving bool
	// turn is the final transcripts of the user's turn, until its run.
	turn string
	// hold is the press of the button or the orb, down the key of the one
	// the pointer pressed last frame.
	hold *assistantHold
	down string
	// hovered: the pointer is over the cards, which holds them.
	hovered bool
	// ticking: the view is built again in a moment.
	ticking bool
	// sentPath is the page whose context the session was told.
	sentPath string
	// opening counts the microphones opened, of which the latest's
	// events count; leave counts the fades.
	opening, leave int
	orb            orbState
}

// toolLine is a tool the assistant runs or ran, with its summary.
type toolLine struct {
	id, summary  string
	done, failed bool
}

func (a *App) assistantService(id string) assistantv1connect.AssistantServiceClient {
	return assistantv1connect.NewAssistantServiceClient(a.connectHTTP(), serverURL(id))
}

// assistantServer returns the server the assistant asks: the page's, else
// the first signed in to; empty for none.
func (a *App) assistantServer() string {
	segs := strings.Split(strings.Trim(a.router.Path(), "/"), "/")
	if p := a.lastPlayer(); p != nil {
		return p.server
	}
	if len(segs) >= 2 {
		for _, s := range a.servers {
			if s.GetId() == segs[1] {
				return segs[1]
			}
		}
	}
	if len(a.servers) > 0 {
		return a.servers[0].GetId()
	}
	return ""
}

// lastPlayer returns the player last opened, nil when nothing plays.
func (a *App) lastPlayer() *player {
	if len(a.players) == 0 {
		return nil
	}
	return a.players[len(a.players)-1]
}

// assistantOn reports whether the asked server's assistant is on, loading
// it in the app's language.
func (a *App) assistantOn(c *ui.Context) bool {
	server := a.assistantServer()
	if server == "" {
		return false
	}
	lang := c.Locale()
	info := load(a, &a.assistant.info, key("assistant", server, lang, a.generation), func(ctx context.Context) (*assistantv1.GetAssistantResponse, error) {
		return a.assistantService(server).GetAssistant(ctx, assistantv1.GetAssistantRequest_builder{Language: new(lang)}.Build())
	}).value
	return info.GetEnabled()
}

// openAssistant opens the panel in win, nil for the app's window.
func (a *App) openAssistant(win Window) {
	st := &a.assistant
	st.open, st.win, st.lingering = true, win, 0
}

// closeAssistant closes the panel, stopping what runs; the orb and cards
// fade out first.
func (a *App) closeAssistant() {
	st := &a.assistant
	if st.cancel != nil {
		st.cancel()
	}
	if st.hold != nil {
		st.hold.timer.Stop()
		st.hold = nil
	}
	st.opening++
	if v := st.voice; v != nil {
		st.voice = nil
		v.close()
	}
	reset := func() {
		*st = assistantState{info: st.info, failed: st.failed, run: st.run + 1, opening: st.opening + 1, leave: st.leave + 1}
	}
	if !a.voiceOn() || !st.open {
		reset()
		return
	}
	if st.leaving {
		return
	}
	st.run++
	st.leave++
	leave := st.leave
	st.leaving, st.listening, st.speaking, st.userSpeaking, st.lingering = true, false, false, false, 0
	time.AfterFunc(leaveTime, func() {
		a.opts.Update(func() {
			if st.leave == leave {
				reset()
			}
		})
	})
}

// screenOf names the screen a path shows, as AskContext does.
func screenOf(path string) (screen, item, library string) {
	segs := strings.Split(strings.Trim(path, "/"), "/")
	switch segs[0] {
	case "item":
		if len(segs) >= 3 {
			item = segs[2]
		}
		return "item", item, ""
	case "library":
		if len(segs) >= 3 {
			library = segs[2]
		}
		return "library", "", library
	case "discover", "title":
		return "discover", "", ""
	}
	return segs[0], "", ""
}

// ask sends a request: the fast path first for typed text, which carries
// out a command for this app's player or a plain move of its page here,
// else the server's Ask, whose events show as they come.
func (a *App) ask(lang string, req *assistantv1.AskRequest) {
	st := &a.assistant
	server := a.assistantServer()
	if server == "" {
		return
	}
	if st.cancel != nil {
		st.cancel()
	}
	st.run++
	run := st.run
	ctx, cancel := context.WithCancel(a.ctx)
	st.cancel = cancel
	text := strings.TrimSpace(req.GetText())
	req.SetText(text)
	// The grammar reads "yes" or "the second" as the answer to what the
	// panel shows.
	state := intentv1.DialogState_builder{
		Confirmation: new(st.confirmation != nil), Playing: new(a.lastPlayer() != nil),
	}.Build()
	if st.choose {
		state.SetResults(int32(len(st.items)))
	}
	st.open, st.server, st.working, st.waits, st.voiced = true, server, true, false, false
	if st.leaving {
		st.leaving = false
		st.leave++
	}
	st.activity, st.answer, st.answerAt, st.err, st.confirmation, st.lingering = nil, "", time.Time{}, "", nil, 0
	if text != "" {
		st.heard, st.heardAt = text, time.Now()
	}
	if text != "" || req.GetChoice() != "" {
		st.items, st.choose = nil, false
	}
	languages := slices.Compact([]string{lang, a.prefs.Language})
	req.SetContext(a.askContext(languages))
	req.SetFailedActions(st.failed)
	st.failed = nil
	go func() {
		var intent *intentv1.Intent
		if text != "" {
			res := &intentv1.ParseIntentResponse{}
			parse := intentv1.ParseIntentRequest_builder{Text: new(text), Languages: languages, State: state}.Build()
			// A grammar that fails leaves the request to the server.
			if a.runIntent(ctx, "parse", parse, res) == nil && len(res.GetIntents()) > 0 {
				intent = res.GetIntents()[0]
			}
		}
		a.opts.Update(func() {
			if run != st.run {
				return
			}
			if intent != nil {
				if r, ok := a.runLocal(intent); ok {
					go a.replyLocal(ctx, run, intent.GetLanguage(), lang, r)
					return
				}
				req.SetIntent(intent)
			}
			go a.askServer(ctx, run, server, req)
		})
	}()
}

// askContext is what the request is asked in: the page, what plays, and
// the languages.
func (a *App) askContext(languages []string) *assistantv1.AskContext {
	screen, item, library := screenOf(a.router.Path())
	b := assistantv1.AskContext_builder{
		Languages: languages, Screen: new(screen), ItemId: new(item), LibraryId: new(library), TimeZone: new(time.Local.String()),
	}
	if p := a.lastPlayer(); p != nil {
		b.Screen = new("player")
		pb := assistantv1.AskPlaying_builder{
			ItemId: new(p.item), Position: durationpb.New(p.state.Position), Paused: new(p.state.Paused),
			Speed: new(p.state.Speed), Volume: new(int32(p.volume)), Muted: new(p.state.Muted),
		}
		if p.playback != nil {
			pb.AudioStreamIndex = new(p.playback.GetAudioStreamIndex())
			pb.SubtitleStreamIndex = new(p.playback.GetSubtitleStreamIndex())
		}
		b.Playing = pb.Build()
	}
	return b.Build()
}

// runIntent runs a function of the grammar through the core.
func (a *App) runIntent(ctx context.Context, function string, req, res proto.Message) error {
	b, err := proto.Marshal(req)
	if err != nil {
		return err
	}
	m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{RunIntent: capiv1.RunIntent_builder{Function: function, Request: b}.Build()}.Build())
	if err != nil {
		return err
	}
	return proto.Unmarshal(m.GetResponse().GetBody(), res)
}

// replyLocal shows the grammar's reply to an intent the fast path carried
// out, off the main thread.
func (a *App) replyLocal(ctx context.Context, run int, lang, fallback string, r localReply) {
	if lang == "" {
		lang = fallback
	}
	say := func(key string, args map[string]string) string {
		res := &intentv1.ReplyResponse{}
		if err := a.runIntent(ctx, "reply", intentv1.ReplyRequest_builder{Language: new(lang), Key: new(key), Args: args}.Build(), res); err != nil {
			return ""
		}
		return res.GetText()
	}
	args := r.args
	if r.duration >= 0 {
		args = map[string]string{"duration": spokenDuration(say, r.duration)}
	}
	answer := say(r.key, args)
	a.opts.Update(func() {
		st := &a.assistant
		if run != st.run {
			return
		}
		st.working, st.answer, st.answerAt = false, answer, time.Now()
		a.linger(run)
	})
}

// spokenDuration says a number of seconds with the grammar's units, as
// the server says it.
func spokenDuration(say func(string, map[string]string) string, secs float64) string {
	s := int(math.Round(math.Abs(secs)))
	var said []string
	for _, u := range []struct {
		n   int
		key string
	}{{s / 3600, "hours"}, {s / 60 % 60, "minutes"}, {s % 60, "seconds"}} {
		if u.n > 0 {
			said = append(said, say(u.key, map[string]string{"n": strconv.Itoa(u.n)}))
		}
	}
	if len(said) == 0 {
		return say("seconds", map[string]string{"n": "0"})
	}
	return strings.Join(said, " ")
}

// askServer sends a request to the server's Ask, off the main thread, and
// shows its events as they come.
func (a *App) askServer(ctx context.Context, run int, server string, req *assistantv1.AskRequest) {
	st := &a.assistant
	update := func(fn func()) {
		a.opts.Update(func() {
			if run == st.run {
				fn()
			}
		})
	}
	stream, err := a.assistantService(server).Ask(ctx, req)
	if err == nil {
		for stream.Receive() {
			ev := stream.Msg()
			update(func() { a.assistantEvent(server, ev) })
		}
		err = stream.Err()
		_ = stream.Close()
	}
	if errors.Is(err, context.Canceled) {
		return
	}
	update(func() {
		st.working, st.answerAt = false, time.Now()
		switch {
		case err != nil:
			st.err = err.Error()
			if e := coreError(err); e != nil && e.Reason != "" {
				st.err = e.Reason
			}
			return
		case len(st.failed) > 0 && st.answer == "":
			st.err = "assistant.cantDo"
		}
		if !st.waits {
			a.linger(run)
		}
	})
}

// assistantEvent shows an event of Ask, carrying out its client actions.
func (a *App) assistantEvent(server string, ev *assistantv1.AskResponse) {
	st := &a.assistant
	switch ev.WhichEvent() {
	case assistantv1.AskResponse_Understood_case:
		if t := ev.GetUnderstood().GetText(); t != "" {
			st.heard = t
		}
	case assistantv1.AskResponse_Tool_case:
		t := ev.GetTool()
		st.activity = slices.DeleteFunc(st.activity, func(l toolLine) bool { return l.id == t.GetId() })
		st.activity = append(st.activity, toolLine{t.GetId(), t.GetSummary(), t.GetDone(), t.GetFailed()})
	case assistantv1.AskResponse_Action_case:
		act := ev.GetAction()
		if !a.perform(server, act) {
			st.failed = append(st.failed, act.GetId())
		}
		if act.HasShowItems() && act.GetShowItems().GetChoose() {
			st.waits = true
		}
	case assistantv1.AskResponse_Confirmation_case:
		st.waits = true
		st.confirmation = ev.GetConfirmation()
	case assistantv1.AskResponse_Answer_case:
		st.answer += ev.GetAnswer()
	case assistantv1.AskResponse_Done_case:
		if t := ev.GetDone().GetAnswer(); t != "" {
			st.answer = t
		}
	}
}

// linger closes the panel a while after the answer of request run, when
// nothing is left to do.
func (a *App) linger(run int) {
	st := &a.assistant
	st.lingering = run
	time.AfterFunc(lingerTime, func() {
		a.opts.Update(func() {
			if st.lingering == run && st.run == run && !st.listening && !st.working && st.confirmation == nil && !st.choose && st.err == "" {
				a.closeAssistant()
			}
		})
	})
}

// perform carries out a client action; false when the app cannot.
func (a *App) perform(server string, act *assistantv1.ClientAction) bool {
	p := a.lastPlayer()
	switch act.WhichKind() {
	case assistantv1.ClientAction_Play_case:
		pl := act.GetPlay()
		ids := pl.GetItemIds()
		if len(ids) == 0 {
			return false
		}
		id := ids[0]
		if i := int(pl.GetStartIndex()); i > 0 && i < len(ids) {
			id = ids[i]
		}
		o := &capiv1.PlaybackOptions{}
		if pl.HasStartPosition() {
			o.SetStartPositionMs(pl.GetStartPosition().AsDuration().Milliseconds())
		}
		if pl.HasAudioStreamIndex() {
			o.SetAudioStreamIndex(pl.GetAudioStreamIndex())
		}
		if pl.HasSubtitleStreamIndex() {
			o.SetSubtitleStreamIndex(pl.GetSubtitleStreamIndex())
		}
		a.play(server, id, o)
		return true
	case assistantv1.ClientAction_Control_case:
		if p == nil {
			return false
		}
		return a.control(p, act.GetControl())
	case assistantv1.ClientAction_Streams_case:
		if p == nil {
			return false
		}
		s := act.GetStreams()
		if s.GetSubtitleOffset() != 0 {
			delay := p.state.SubtitleDelay + time.Duration(s.GetSubtitleOffset()*float64(time.Second))
			a.setVideo(p, "sub-delay", strconv.FormatFloat(delay.Seconds(), 'f', 3, 64))
		}
		if s.HasAudioStreamIndex() || s.HasSubtitleStreamIndex() {
			a.restart(p, func(o *capiv1.PlaybackOptions) {
				if s.HasAudioStreamIndex() {
					o.SetAudioStreamIndex(s.GetAudioStreamIndex())
				}
				if s.HasSubtitleStreamIndex() {
					o.SetSubtitleStreamIndex(s.GetSubtitleStreamIndex())
				}
			})
		}
		return true
	case assistantv1.ClientAction_Navigate_case:
		n := act.GetNavigate()
		switch n.GetTarget() {
		case assistantv1.NavigateTarget_NAVIGATE_TARGET_HOME:
			a.leavePlayer()
			a.openPlace("/home")
		case assistantv1.NavigateTarget_NAVIGATE_TARGET_BACK:
			if p != nil {
				a.closePlayer(p, true)
			} else {
				a.goBack()
			}
		case assistantv1.NavigateTarget_NAVIGATE_TARGET_ITEM:
			if n.GetId() == "" {
				return false
			}
			a.leavePlayer()
			a.router.Push("/item/" + server + "/" + n.GetId())
		case assistantv1.NavigateTarget_NAVIGATE_TARGET_LIBRARY:
			if n.GetId() == "" {
				return false
			}
			a.leavePlayer()
			a.router.Push("/library/" + server + "/" + n.GetId())
		case assistantv1.NavigateTarget_NAVIGATE_TARGET_SEARCH:
			a.leavePlayer()
			a.searchFor(strings.TrimSpace(n.GetQuery()))
		case assistantv1.NavigateTarget_NAVIGATE_TARGET_SETTINGS:
			a.leavePlayer()
			a.router.Push("/settings")
		default:
			return false
		}
		return true
	case assistantv1.ClientAction_ShowItems_case:
		a.assistant.items = act.GetShowItems().GetItems()
		a.assistant.choose = act.GetShowItems().GetChoose()
		return true
	}
	return false
}

// leavePlayer stops the player of the app's window, so that a page it
// covers shows.
func (a *App) leavePlayer() {
	if p := a.playerOf(nil); p != nil {
		a.closePlayer(p, true)
	}
}

// control carries out a command on player p; false when it cannot.
func (a *App) control(p *player, ctl *assistantv1.ControlAction) bool {
	v := ctl.GetValue()
	switch ctl.GetCommand() {
	case assistantv1.ControlCommand_CONTROL_COMMAND_PAUSE:
		a.setVideo(p, "pause", "yes")
	case assistantv1.ControlCommand_CONTROL_COMMAND_RESUME:
		a.setVideo(p, "pause", "no")
	case assistantv1.ControlCommand_CONTROL_COMMAND_STOP:
		a.closePlayer(p, true)
	case assistantv1.ControlCommand_CONTROL_COMMAND_SEEK:
		a.seekTo(p, seconds(v), true)
	case assistantv1.ControlCommand_CONTROL_COMMAND_SEEK_BY:
		a.seekBy(p, seconds(v))
	case assistantv1.ControlCommand_CONTROL_COMMAND_NEXT:
		if len(p.next) == 0 {
			return false
		}
		a.playNext(p)
	case assistantv1.ControlCommand_CONTROL_COMMAND_SPEED:
		a.setVideo(p, "speed", strconv.FormatFloat(v, 'f', -1, 64))
	case assistantv1.ControlCommand_CONTROL_COMMAND_VOLUME:
		a.setVolume(p, int(math.Round(v)))
	case assistantv1.ControlCommand_CONTROL_COMMAND_MUTE:
		a.setVideo(p, "mute", "yes")
	case assistantv1.ControlCommand_CONTROL_COMMAND_UNMUTE:
		a.setVideo(p, "mute", "no")
	case assistantv1.ControlCommand_CONTROL_COMMAND_FULLSCREEN:
		a.toggleFullScreen(p)
	default:
		return false
	}
	return true
}

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }

// setVolume sets player p's volume, from 0 to 100.
func (a *App) setVolume(p *player, level int) {
	p.volume = min(max(level, 0), 100)
	a.setVideo(p, "volume", strconv.Itoa(p.volume))
}

// localReply is the grammar's reply to an intent the fast path carried
// out: its key and arguments, or the seconds it says as a duration when
// not negative.
type localReply struct {
	key      string
	args     map[string]string
	duration float64
}

// runLocal carries out an intent about this app — its player's controls
// without a device named, or going home, back or to the settings — as
// the web UI's fast path does; false leaves it to the server.
func (a *App) runLocal(in *intentv1.Intent) (localReply, bool) {
	r := func(key string) (localReply, bool) { return localReply{key: key, duration: -1}, true }
	if in.GetMatch() != intentv1.IntentMatch_INTENT_MATCH_FULL || slotOf(in, "device") != nil {
		return localReply{}, false
	}
	switch in.GetName() {
	case "go_home":
		a.leavePlayer()
		a.openPlace("/home")
		return r("home")
	case "go_back":
		if p := a.lastPlayer(); p != nil {
			a.closePlayer(p, true)
		} else {
			a.goBack()
		}
		return r("back")
	case "open_settings":
		a.leavePlayer()
		a.router.Push("/settings")
		return r("settings")
	}
	p := a.lastPlayer()
	if p == nil {
		return localReply{}, false
	}
	switch in.GetName() {
	case "pause":
		a.setVideo(p, "pause", "yes")
		return r("paused")
	case "resume":
		a.setVideo(p, "pause", "no")
		return r("resumed")
	case "stop":
		a.closePlayer(p, true)
		return r("stopped")
	case "mute":
		a.setVideo(p, "mute", "yes")
		return r("muted")
	case "unmute":
		a.setVideo(p, "mute", "no")
		return r("unmuted")
	case "fullscreen":
		a.toggleFullScreen(p)
		return r("fullscreen")
	case "seek_back", "seek_forward":
		secs, ok := slotNumber(in, "seconds")
		if !ok {
			secs = 10
		}
		key := "seeked_forward"
		if in.GetName() == "seek_back" {
			secs, key = -secs, "seeked_back"
		}
		a.seekBy(p, seconds(secs))
		return localReply{key: key, duration: secs}, true
	case "seek_to":
		pos, _ := slotNumber(in, "position")
		a.seekTo(p, seconds(pos), true)
		return localReply{key: "seeked_to", args: map[string]string{"position": clock(seconds(pos))}, duration: -1}, true
	case "restart":
		a.seekTo(p, 0, true)
		return r("restarted")
	case "speed":
		speed, ok := slotNumber(in, "speed")
		if !ok {
			switch slotOf(in, "change").GetKeyword() {
			case "up":
				speed = min(p.state.Speed+0.25, 3)
			case "down":
				speed = max(p.state.Speed-0.25, 0.25)
			default:
				speed = 1
			}
		}
		a.setVideo(p, "speed", strconv.FormatFloat(speed, 'f', -1, 64))
		return localReply{key: "speed", args: map[string]string{"speed": strconv.FormatFloat(speed, 'f', -1, 64)}, duration: -1}, true
	case "volume":
		level, ok := slotNumber(in, "level")
		if !ok {
			level = float64(p.volume + 10)
			if slotOf(in, "change").GetKeyword() == "down" {
				level = float64(p.volume - 10)
			}
		}
		a.setVolume(p, int(math.Round(level)))
		return localReply{key: "volume", args: map[string]string{"level": strconv.Itoa(p.volume)}, duration: -1}, true
	case "subtitles_off":
		a.restart(p, func(o *capiv1.PlaybackOptions) { o.SetSubtitleStreamIndex(-1) })
		return r("subtitles_off")
	}
	return localReply{}, false
}

// slotOf returns an intent's slot, nil when it has none of the name.
func slotOf(in *intentv1.Intent, name string) *intentv1.Slot {
	for _, s := range in.GetSlots() {
		if s.GetName() == name {
			return s
		}
	}
	return nil
}

// slotNumber returns a slot's number, seconds or ordinal.
func slotNumber(in *intentv1.Intent, name string) (float64, bool) {
	s := slotOf(in, name)
	switch {
	case s.HasNumber():
		return s.GetNumber(), true
	case s.HasSeconds():
		return s.GetSeconds(), true
	case s.HasOrdinal():
		return float64(s.GetOrdinal()), true
	}
	return 0, false
}

// assistantButton builds the button opening the assistant's panel, shown
// when the server's assistant is on; size is the button's.
func (a *App) assistantButton(c *ui.Context, win Window, size float32) {
	if !a.assistantOn(c) {
		return
	}
	colors := style.Colors(c)
	label := style.T(c, "assistant.title")
	b := ui.ButtonBase(c.Key("assistant")).Size(size, size).Radius(design.RadiusMd).Center().Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon("microphone", a.assistantActive())).Size(18, 18).TextColor(style.Color(colors.TextSecondary))
	})
	switch {
	case a.voiceOn():
		a.voicePress(c, b, "button", win)
	case b.Clicked():
		a.openAssistant(win)
	}
}

// assistantActive reports whether the assistant listens, or its panel is
// open where the app has no voice.
func (a *App) assistantActive() bool {
	if a.voiceOn() {
		return a.assistant.listening
	}
	return a.assistant.open
}

// assistantPanel builds the assistant's panel over the window win, nil
// for the app's, as the web UI's: the request, what the server does, the
// answer, results to choose from, a confirmation and examples, and the
// field to type a request in.
func (a *App) assistantPanel(c *ui.Context, win Window) {
	st := &a.assistant
	if a.voiceOn() {
		// V taps the assistant outside text fields.
		if win == nil && c.Shortcut(0, ui.KeyV) && a.assistantOn(c) {
			a.tapAssistant(win, c.Locale())
		}
		if !st.open || st.win != win {
			return
		}
		// The page's context goes to the session as it changes.
		if path := a.router.Path(); path != st.sentPath {
			st.sentPath = path
			a.voiceContext(c.Locale())
		}
		a.voiceOverlay(c, win)
		return
	}
	if !st.open || st.win != win {
		return
	}
	colors := style.Colors(c)
	info := st.info.value
	lang := c.Locale()
	if c.Shortcut(0, ui.KeyEscape) {
		a.closeAssistant()
		return
	}
	w, _ := c.Size()
	width := min(w-24, 576)
	ui.Column(c).Absolute().Left(0).Right(0).Bottom(24).AlignItems(ui.Center).PassThrough().Children(func() {
		panel := ui.Column(c.Key("assistant-panel")).Width(width).MaxHeight(560).Radius(20).Clip().
			Background(style.Color(colors.Surface)).Border(1, style.Color(colors.Border)).
			Shadow(0, 12, 32, 0, style.Color(design.ElevationLevel3.Color)).Label(style.T(c, "assistant.title"))
		if panel.Pressed() {
			st.lingering = 0
		}
		panel.Children(func() {
			ui.Row(c).Padding(12, 16, 0).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, style.Icon("microphone", true)).Size(18, 18).TextColor(style.Color(colors.TextPrimary))
				style.Text(ui.Text(c, style.T(c, "assistant.title")), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).Grow(1)
				label := style.T(c, "action.close")
				b := ui.ButtonBase(c.Key("assistant-close")).Size(32, 32).Radius(16).Center().Label(label).Background(style.Color(colors.Fill))
				if b.Hovered() {
					b.Background(style.Color(colors.FillSelected))
				}
				b.Children(func() {
					ui.Icon(c, style.Icon("close", false)).Size(16, 16).TextColor(style.Color(colors.TextPrimary))
				})
				if b.Clicked() {
					a.closeAssistant()
				}
			})
			ui.Scroll(c.Key("assistant-scroll")).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Padding(12, 16).Gap(20).Children(func() { a.assistantBody(c, info, lang) })
			})
			ui.Box(c).Height(1).Background(style.Color(colors.Border))
			a.assistantField(c, lang)
		})
	})
}

// assistantBody builds the panel's request, activity, answer, results,
// confirmation and examples.
func (a *App) assistantBody(c *ui.Context, info *assistantv1.GetAssistantResponse, lang string) {
	st := &a.assistant
	colors := style.Colors(c)
	if st.info.err == nil && info != nil && !info.GetEnabled() {
		style.Text(ui.Text(c, style.T(c, "assistant.off")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
	}
	if st.heard != "" {
		// What the user said, in a bubble at the right under "You".
		ui.Column(c.Key("heard")).Gap(4).AlignItems(ui.End).Padding(0, 0, 0, 64).Children(func() {
			style.Text(ui.Text(c, style.T(c, "conversations.you")), style.Type(c).Caption).TextColor(style.Color(colors.TextTertiary)).Padding(0, 4)
			bubble(c, st.heard, true)
			a.messageBar(c, "heard", st.heard, st.heardAt)
		})
	}
	working := st.working && st.answer == ""
	if working || len(st.activity) > 0 || st.answer != "" {
		// The assistant's status, activity and answer, at the left under
		// its name, the answer in a dashed bubble.
		ui.Column(c.Key("answer")).Gap(4).AlignItems(ui.Start).Padding(0, 64, 0, 0).Children(func() {
			style.Text(ui.Text(c, style.T(c, "assistant.title")), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Padding(0, 4)
			if working {
				style.Text(ui.Text(c, style.T(c, "assistant.working")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).Padding(0, 4)
			}
			if len(st.activity) > 0 {
				ui.Column(c).Gap(4).Padding(0, 4).Children(func() {
					for _, l := range st.activity {
						icon, color := "pending", colors.TextSecondary
						switch {
						case l.failed:
							icon, color = "error", colors.Danger
						case l.done:
							icon = "check"
						}
						ui.Row(c.Key(l.id)).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, style.Icon(icon, false)).Size(14, 14).TextColor(style.Color(color))
							style.Text(ui.Text(c, l.summary), style.Type(c).Footnote).TextColor(style.Color(color))
						})
					}
				})
			}
			if st.answer != "" {
				bubble(c, st.answer, false)
				if !st.answerAt.IsZero() {
					a.messageBar(c, "answer", st.answer, st.answerAt)
				}
			}
		})
	}
	if len(st.items) > 0 {
		a.assistantItems(c)
	}
	if cf := st.confirmation; cf != nil {
		ui.Column(c).Padding(12).Gap(12).Radius(14).Background(style.Color(colors.Background)).Children(func() {
			style.Text(ui.Text(c, cf.GetText()), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
			ui.Row(c).Gap(8).Children(func() {
				answer := func(ok bool) {
					a.ask(lang, assistantv1.AskRequest_builder{Answer: assistantv1.ConfirmationAnswer_builder{Id: new(cf.GetId()), Confirmed: new(ok)}.Build()}.Build())
				}
				if confirmButton(c, style.T(c, "assistant.confirm"), true) {
					answer(true)
				}
				if confirmButton(c, style.T(c, "action.cancel"), false) {
					answer(false)
				}
			})
		})
	}
	if st.err != "" {
		msg := st.err
		if msg == "assistant.cantDo" {
			msg = style.T(c, msg)
		}
		style.Text(ui.Text(c, msg), style.Type(c).Callout).TextColor(style.Color(colors.Danger))
	}
	fresh := st.heard == "" && st.answer == "" && !st.working && st.err == "" && len(st.items) == 0
	if examples := info.GetExamples(); fresh && len(examples) > 0 {
		ui.Column(c).Gap(8).Children(func() {
			style.Text(ui.Text(c, style.T(c, "assistant.trySaying")), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for _, ex := range examples {
					b := ui.ButtonBase(c.Key("example-"+ex)).Padding(6, 12).Radius(999).Label(ex).Background(style.Color(colors.Fill))
					if b.Hovered() {
						b.Background(style.Color(colors.FillSelected))
					}
					b.Children(func() {
						style.Text(ui.Text(c, ex), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary))
					})
					if b.Clicked() {
						a.ask(lang, assistantv1.AskRequest_builder{Text: new(ex)}.Build())
					}
				}
			})
		})
	}
}

// bubble builds a message's bubble, as the web UI's: the user's and the
// answer's alike, the answer's in a dashed border of the accent.
func bubble(c *ui.Context, text string, user bool) {
	colors := style.Colors(c)
	b := ui.Box(c).MaxWidth(672).Padding(8, 12).Radius(design.RadiusLg).Background(style.Color(colors.Background))
	if !user {
		b.Border(1, style.Color(colors.Accent).Alpha(0.5)).BorderStyle(ui.BorderDashed)
	}
	b.Children(func() {
		style.Text(ui.Text(c, text), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
	})
}

// messageBar builds the bar under a message: a button copying it, then
// its time, the date too when not today.
func (a *App) messageBar(c *ui.Context, id, text string, at time.Time) {
	st := &a.assistant
	colors := style.Colors(c)
	ui.Row(c.Key("bar-" + id)).Gap(2).AlignItems(ui.Center).Children(func() {
		icon, label := "copy", style.T(c, "assistant.copyMessage")
		if st.copied == id {
			icon, label = "check", style.T(c, "assistant.copiedMessage")
		}
		b := ui.ButtonBase(c.Key("copy")).Size(28, 28).Radius(design.RadiusMd).Center().Label(label).Tooltip(label)
		color := colors.TextTertiary
		if b.Hovered() {
			b.Background(style.Color(colors.Fill))
			color = colors.TextPrimary
		}
		b.Children(func() {
			ui.Icon(c, style.Icon(icon, false)).Size(16, 16).TextColor(style.Color(color))
		})
		if b.Clicked() {
			c.WriteClipboard(text)
			st.copied = id
			time.AfterFunc(2*time.Second, func() {
				a.opts.Update(func() {
					if st.copied == id {
						st.copied = ""
					}
				})
			})
		}
		if at.IsZero() {
			return
		}
		when := at.Local().Format("3:04 PM")
		if y, m, d := at.Local().Date(); y != time.Now().Year() || m != time.Now().Month() || d != time.Now().Day() {
			when = mediumDateTime(at)
		}
		style.Text(ui.Text(c, when), style.Type(c).Caption).TextColor(style.Color(colors.TextTertiary)).Padding(0, 4).Tooltip(mediumDateTime(at))
	})
}

// confirmButton builds a button of a confirmation, as the web UI's: the
// primary one the button's color, the other the fill.
func confirmButton(c *ui.Context, label string, primary bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key("confirm-"+label)).MinHeight(28).Padding(0, 12).Radius(design.RadiusMd).Center().Label(label)
	bg, fg := style.Color(colors.Fill), style.Color(colors.TextPrimary)
	if primary {
		bg, fg = style.Color(colors.Button), style.Color(colors.OnButton)
	}
	switch {
	case primary && b.Hovered():
		b.Opacity(0.85)
	case b.Hovered():
		bg = style.Color(colors.FillSelected)
	}
	b.Background(bg).Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Headline).TextColor(fg)
	})
	return b.Clicked()
}

// assistantItems builds the results of an answer: to choose from,
// numbered, or to open.
func (a *App) assistantItems(c *ui.Context) {
	st := &a.assistant
	colors := style.Colors(c)
	ui.Column(c).Radius(14).Clip().Background(style.Color(colors.Background)).Children(func() {
		for i, it := range st.items {
			if i > 0 {
				ui.Box(c).Height(1).Background(style.Color(colors.Border))
			}
			card := a.cardOf(st.server, it)
			label := it.GetName()
			if st.choose {
				label = strconv.Itoa(i+1) + ". " + label
			}
			b := ui.ButtonBase(c.Key("result-"+it.GetId())).Padding(8, 12).Gap(12).AlignItems(ui.Center).Label(label)
			if b.Hovered() {
				b.Background(style.Color(colors.Fill))
			}
			b.Children(func() {
				if st.choose {
					style.Text(ui.Text(c, strconv.Itoa(i+1)), style.Type(c).Headline).TextColor(style.Color(colors.TextSecondary)).Width(20).TextAlign(ui.Center)
				}
				ui.Box(c).Width(40).Height(60).Radius(6).Clip().Shrink(0).Children(func() {
					a.artwork(c, card.GetPoster(), "film", card, 40, 60, false)
				})
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					style.Text(ui.Text(c, it.GetName()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
					if sub := card.GetSubtitle(); sub != "" {
						style.Text(ui.Text(c, sub), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine()
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
	})
}

// assistantField builds the field a request is typed in, with Send.
func (a *App) assistantField(c *ui.Context, lang string) {
	st := &a.assistant
	colors := style.Colors(c)
	ui.Row(c).Padding(10, 12).Gap(8).AlignItems(ui.Center).Children(func() {
		label := style.T(c, "assistant.messagePlaceholder")
		f := ui.TextInputBase(c, &st.text).Grow(1).MinWidth(0).Height(40).Padding(0, 16).Radius(20).
			Background(style.Color(colors.Fill)).Placeholder(label).Label(label).AutoFocus()
		if f.Changed() {
			st.lingering = 0
		}
		send := primaryPill(c, style.T(c, "assistant.send"), strings.TrimSpace(st.text) == "")
		if (send || f.Submitted()) && strings.TrimSpace(st.text) != "" {
			a.ask(lang, assistantv1.AskRequest_builder{Text: new(st.text)}.Build())
			st.text = ""
		}
	})
}

// primaryPill builds a round button of the primary color, reporting
// whether it was clicked.
func primaryPill(c *ui.Context, label string, disabled bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key("pill-"+label)).Height(40).Padding(0, 16).Radius(20).AlignItems(ui.Center).Label(label).Disabled(disabled).
		Background(style.Color(colors.Button))
	switch {
	case disabled:
		b.Opacity(0.4)
	case b.Hovered():
		b.Opacity(0.85)
	}
	b.Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Headline).TextColor(style.Color(colors.OnButton))
	})
	return b.Clicked() && !disabled
}

// tvAssistantButton builds the television's assistant button beside the
// user menu's dots, as the web UI's: frosted, white with focus.
func (a *App) tvAssistantButton(c *ui.Context) {
	if !a.assistantOn(c) {
		return
	}
	colors := style.Colors(c)
	label := style.T(c, "assistant.title")
	b := ui.ButtonBase(c.Key("tv-assistant")).Size(48, 48).Radius(design.RadiusFull).Label(label).Material(glass.Blur{Radius: 30})
	fg := style.Color(colors.TextSecondary)
	focused := a.tvFocus(c, b, false)
	b.Children(func() {
		inner := ui.Box(c).Size(48, 48).Radius(design.RadiusFull).Center().Background(style.Color(colors.SurfaceRaised).Alpha(0.6))
		if focused {
			inner.Background(ui.RGB(255, 255, 255))
			fg = ui.RGB(0, 0, 0)
		}
		inner.Children(func() {
			ui.Icon(c, style.Icon("microphone", a.assistantActive())).Size(26, 26).TextColor(fg)
		})
	})
	switch {
	case a.voiceOn():
		a.voicePress(c, b, "tv-button", nil)
	case b.Clicked():
		a.openAssistant(nil)
	}
}

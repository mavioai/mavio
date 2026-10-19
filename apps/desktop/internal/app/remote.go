package app

import (
	"cmp"
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/types/known/durationpb"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// remoteState is the state of the dialogs playing an item on another of
// the user's devices and controlling one.
type remoteState struct {
	open bool
	// session is the device controlled, empty while one is chosen to
	// play on.
	session  string
	sessions task[[]*sessionv1.Session]
	busy     bool
	err      error
	volume   float64
	// scrub is the position as the slider is dragged, in seconds.
	scrub float64
	text  string
	sent  bool
}

// playable reports whether a device plays an item of a kind by itself.
func playable(kind libraryv1.ItemKind) bool {
	switch kind {
	case libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_EPISODE, libraryv1.ItemKind_ITEM_KIND_VIDEO,
		libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO, libraryv1.ItemKind_ITEM_KIND_TRACK, libraryv1.ItemKind_ITEM_KIND_AUDIOBOOK:
		return true
	}
	return false
}

// deviceName returns a device's name, else its client's.
func deviceName(s *sessionv1.Session) string {
	return cmp.Or(s.GetDeviceName(), s.GetClient(), "—")
}

// remoteSessions loads the user's devices, again as they change.
func (a *App) remoteSessions(rs *remoteState, server string) *task[[]*sessionv1.Session] {
	return load(a, &rs.sessions, key("remote", server, a.generation, a.sessionsChanged), func(ctx context.Context) ([]*sessionv1.Session, error) {
		res, err := a.sessionService(server).ListSessions(ctx, &sessionv1.ListSessionsRequest{})
		return res.GetSessions(), err
	})
}

// sendCommand sends a device a command, then runs done when it went.
func (a *App) sendCommand(rs *remoteState, server, id string, cmd *sessionv1.Command, done func()) {
	rs.busy, rs.err = true, nil
	go func() {
		_, err := a.sessionService(server).SendCommand(a.ctx, sessionv1.SendCommandRequest_builder{SessionId: &id, Command: cmd}.Build())
		a.opts.Update(func() {
			rs.busy, rs.err = false, err
			if err == nil && done != nil {
				done()
			}
		})
	}()
}

// playOnDialog builds the dialog choosing one of the user's other online
// devices to play the item on, as the web UI's
// (apps/web/src/screens/remote.tsx), then controlling it.
func (a *App) playOnDialog(c *ui.Context, rs *remoteState, server string, it *libraryv1.Item) {
	if rs.session != "" {
		a.remoteDialog(c, rs, server)
		return
	}
	colors := style.Colors(c)
	sessions := a.remoteSessions(rs, server)
	ui.Modal(c, &rs.open, func() {
		ui.Column(c).Width(440).MaxHeight(600).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "remote.playOnTitle", "name", it.GetName())), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					targets := 0
					for _, s := range sessions.value {
						if !s.GetOnline() || s.GetCurrent() {
							continue
						}
						targets++
						b := ui.ButtonBase(c.Key(s.GetId())).MinHeight(48).Padding(6, 12).Gap(12).Radius(8).AlignItems(ui.Center).
							Label(deviceName(s)).Cursor(ui.CursorPointer).Disabled(rs.busy)
						if b.Hovered() || a.tvFocus(c, b, false) {
							b.Background(style.Color(colors.Fill))
						}
						icon := "devices"
						if s.GetPluginId() != "" {
							icon = "television"
						}
						detail := s.GetClient()
						if np := s.GetNowPlaying().GetItem(); np != nil {
							detail = style.T(c, "remote.playing", "name", np.GetName())
						}
						b.Children(func() {
							ui.Icon(c, style.Icon(icon, false)).Size(20, 20).TextColor(style.Color(colors.TextSecondary))
							ui.Column(c).Grow(1).MinWidth(0).Children(func() {
								style.Text(ui.Text(c, deviceName(s)), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).SingleLine()
								style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine()
							})
						})
						if b.Clicked() {
							play := sessionv1.Play_builder{ItemIds: []string{it.GetId()}}
							if u := it.GetUserData(); u.HasPosition() && !u.GetPlayed() {
								play.StartPosition = u.GetPosition()
							}
							id := s.GetId()
							a.sendCommand(rs, server, id, sessionv1.Command_builder{Play: play.Build()}.Build(), func() { rs.session = id })
						}
					}
					if sessions.loaded && sessions.err == nil && targets == 0 {
						style.Text(ui.Text(c, style.T(c, "remote.noDevices")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).Padding(0, 12)
					}
				})
			})
			if err := cmp.Or(rs.err, sessions.err); err != nil {
				style.Text(ui.Text(c, errorMessage(c, err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
			ui.Row(c).Justify(ui.End).Children(func() {
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					rs.open = false
				}
			})
		})
	})
}

// remoteDialog builds the control of a device: its playback, its volume,
// and a message to show on it.
func (a *App) remoteDialog(c *ui.Context, rs *remoteState, server string) {
	colors := style.Colors(c)
	sessions := a.remoteSessions(rs, server)
	var s *sessionv1.Session
	for _, x := range sessions.value {
		if x.GetId() == rs.session {
			s = x
		}
	}
	np := s.GetNowPlaying()
	it := np.GetItem()
	runtime := it.GetRuntime().AsDuration()
	position := np.GetPosition().AsDuration()
	state := func(cmd sessionv1.PlayStateCommand) {
		a.sendCommand(rs, server, rs.session, sessionv1.Command_builder{PlayState: sessionv1.PlayState_builder{Command: &cmd}.Build()}.Build(), nil)
	}
	seek := func(d time.Duration) {
		a.sendCommand(rs, server, rs.session, sessionv1.Command_builder{Seek: sessionv1.Seek_builder{Position: durationpb.New(max(d, 0))}.Build()}.Build(), nil)
	}
	title := style.T(c, "remote.title")
	if s != nil {
		title = deviceName(s)
	}
	ui.Modal(c, &rs.open, func() {
		ui.Column(c).Width(440).Gap(12).Children(func() {
			style.Text(ui.Text(c, title), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			secondary := func(text string) {
				style.Text(ui.Text(c, text), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
			}
			switch {
			case s != nil && !s.GetOnline():
				secondary(style.T(c, "remote.offline"))
			case s != nil && it == nil:
				secondary(style.T(c, "remote.idle"))
			}
			if it != nil {
				ui.Column(c).Gap(8).Children(func() {
					ui.Column(c).Children(func() {
						style.Text(ui.Text(c, it.GetName()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
						if sub := cardSubtitle(a.cardOf(server, it)); sub != "" {
							style.Text(ui.Text(c, sub), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine()
						}
					})
					if runtime > 0 {
						hi := runtime.Seconds()
						sl := ui.Slider(c, &rs.scrub, 0, hi).Step(1).Label(style.T(c, "remote.position"))
						if !sl.Dragging() && !sl.Changed() {
							rs.scrub = min(position.Seconds(), hi)
						}
						if sl.Changed() && !sl.Dragging() {
							seek(time.Duration(rs.scrub * float64(time.Second)))
						}
						ui.Row(c).Justify(ui.SpaceBetween).Children(func() {
							at := time.Duration(rs.scrub * float64(time.Second))
							style.Text(ui.Text(c, clock(at)), style.Type(c).Caption).FontFeatures("tnum").TextColor(style.Color(colors.TextSecondary))
							style.Text(ui.Text(c, clock(runtime)), style.Type(c).Caption).FontFeatures("tnum").TextColor(style.Color(colors.TextSecondary))
						})
					}
					ui.Row(c).Gap(4).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
						if a.remoteControl(c, "previous", style.T(c, "remote.previous"), false) {
							state(sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_PREVIOUS)
						}
						if a.remoteControl(c, "seekBack", style.T(c, "player.seekBack", "count", 10), false) {
							seek(position - 10*time.Second)
						}
						icon, label, cmd := "pause", style.T(c, "player.pause"), sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_PAUSE
						if np.GetPaused() {
							icon, label, cmd = "play", style.T(c, "player.play"), sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_UNPAUSE
						}
						if a.remoteControl(c, icon, label, true) {
							state(cmd)
						}
						if a.remoteControl(c, "seekForward", style.T(c, "player.seekForward", "count", 10), false) {
							seek(position + 10*time.Second)
						}
						if a.remoteControl(c, "next", style.T(c, "remote.next"), false) {
							state(sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_NEXT)
						}
					})
					ui.Row(c).Justify(ui.Center).Children(func() {
						if destructiveButton(c, style.T(c, "remote.stop")).Clicked() {
							state(sessionv1.PlayStateCommand_PLAY_STATE_COMMAND_STOP)
						}
					})
				})
			}
			if s.GetOnline() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					if a.remoteControl(c, "mute", style.T(c, "player.mute"), false) {
						a.sendCommand(rs, server, rs.session, sessionv1.Command_builder{Volume: sessionv1.Volume_builder{Muted: new(true)}.Build()}.Build(), nil)
					}
					v := ui.Slider(c, &rs.volume, 0, 100).Step(1).Grow(1).Label(style.T(c, "remote.volume"))
					if v.Changed() && !v.Dragging() {
						a.sendCommand(rs, server, rs.session, sessionv1.Command_builder{Volume: sessionv1.Volume_builder{Level: new(int32(rs.volume)), Muted: new(false)}.Build()}.Build(), nil)
					}
					style.Text(ui.Text(c, fmt.Sprint(int(rs.volume))), style.Type(c).Footnote).FontFeatures("tnum").TextColor(style.Color(colors.TextSecondary)).Width(32).TextAlign(ui.End)
				})
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					in := ui.TextInput(c, &rs.text).Grow(1).Label(style.T(c, "remote.message")).Placeholder(style.T(c, "remote.message"))
					if in.Changed() {
						rs.sent = false
					}
					text := strings.TrimSpace(rs.text)
					ok := text != "" && len(text) <= 1000 && !rs.busy
					label := style.T(c, "remote.send")
					if rs.sent {
						label = style.T(c, "remote.sent")
					}
					if secondaryButton(c, "", label).Disabled(!ok).Clicked() || in.Submitted() && ok {
						a.sendCommand(rs, server, rs.session, sessionv1.Command_builder{Message: sessionv1.Message_builder{Text: &text}.Build()}.Build(), func() {
							rs.text, rs.sent = "", true
						})
					}
				})
			}
			if err := cmp.Or(rs.err, sessions.err); err != nil {
				style.Text(ui.Text(c, errorMessage(c, err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.PrimaryButton(c, style.T(c, "keys.done")).Clicked() {
					rs.open = false
				}
			})
		})
	})
}

// remoteControl builds a round button of the remote control, larger and
// on the fill color for play and pause, and reports whether it was
// clicked.
func (a *App) remoteControl(c *ui.Context, icon, label string, large bool) bool {
	colors := style.Colors(c)
	size, glyph := float32(44), float32(22)
	if large {
		size, glyph = 56, 28
	}
	b := ui.ButtonBase(c.Key(icon)).Size(size, size).Radius(size / 2).Center().Label(label).Tooltip(label).Cursor(ui.CursorPointer)
	if large {
		b.Background(style.Color(colors.Fill))
	}
	if b.Hovered() || a.tvFocus(c, b, false) {
		b.Background(style.Color(colors.FillSelected))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, large)).Size(glyph, glyph).TextColor(style.Color(colors.TextPrimary))
	})
	return b.Clicked()
}

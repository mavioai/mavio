package app

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
	"google.golang.org/protobuf/proto"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"

	"github.com/mavioai/mavio/apps/desktop/internal/mpv"
	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// Video plays media into a native view of the window: libmpv's, made by
// Options.NewVideo.
type Video interface {
	// View returns the view the video shows in, for ui.Video; nil for none.
	View() ui.NativeVideoView
	// Load plays url from start, replacing what played.
	Load(url string, start time.Duration) error
	// Command runs a command of mpv's.
	Command(args ...string) error
	// Set sets a property of mpv's.
	Set(name, value string) error
	// Close stops the video and frees it, off the main thread.
	Close()
}

// Window is a window of the player's, made by Options.NewWindow.
type Window interface {
	Close()
	Focus()
	SetTitle(title string)
	ToggleFullScreen()
	IsFullScreen() bool
}

// panelTab is a tab of the player's side panel.
type panelTab int

const (
	tabNone panelTab = iota
	tabAudio
	tabSubtitles
	tabQuality
	tabInfo
)

// quality is a quality offered, as the web UI's, with the bitrate it
// allows.
type quality struct {
	key     string
	bitrate int64
}

// qualities are the qualities, in the order they are offered.
var qualities = []quality{
	{"original", 200_000_000},
	{"auto", 40_000_000},
	{"1080p", 10_000_000},
	{"720p", 4_000_000},
	{"480p", 1_500_000},
}

var speeds = []float64{0.5, 0.75, 1, 1.25, 1.5, 2}

// controlsTimeout is how long the controls show without input during
// playback.
const controlsTimeout = 3 * time.Second

// player is a playback and its screen.
type player struct {
	server, item string
	// next are the episodes that follow, played in a row.
	next []*capiv1.Card
	// options are what the next Play asks.
	options *capiv1.PlaybackOptions
	details task[*capiv1.Details]
	video   Video
	// playback is the server's session, once it answers.
	playback *capiv1.Playback
	// starting counts the Play calls, of which the latest answer wins.
	starting int
	err      error
	state    mpv.State
	// selected tells that the playback's streams were selected in mpv.
	selected bool
	// input is when the user last moved the pointer or pressed a key.
	input   time.Time
	pointer [2]float32
	panel   panelTab
	// scrub is the scrubber's value, in seconds.
	scrub float64
	// reported is when the position was last sent to the core.
	reported time.Time
	release  func()
	// win is the window the player shows in, nil for the app's, and title
	// the title it was given.
	win   Window
	title string
}

// playerOf returns the player showing in win, nil for the app's window.
func (a *App) playerOf(win Window) *player {
	for _, p := range a.players {
		if p.win == win {
			return p
		}
	}
	return nil
}

// playing reports whether p still plays.
func (a *App) playing(p *player) bool { return slices.Contains(a.players, p) }

// play plays an item of server, with options when not nil, followed by
// the episodes of next: in a window of the player's, a new one for each
// video when the preferences ask, else in place of what plays.
func (a *App) play(server, item string, options *capiv1.PlaybackOptions, next ...*capiv1.Card) {
	windows := a.opts.NewWindow != nil
	var win Window
	if !windows || !a.prefs.PlayerWindows {
		for _, p := range slices.Clone(a.players) {
			if windows && p.win != nil && win == nil {
				win = p.win
				a.endPlayer(p, true)
				continue
			}
			a.closePlayer(p, true)
		}
	}
	if windows && win == nil {
		var w Window
		w = a.opts.NewWindow(func(c *ui.Context) { a.playerWindowView(c, w) }, func() { a.windowClosed(w) })
		win = w
	}
	a.playIn(win, server, item, options, next...)
}

// playIn plays an item of server in win, the app's window when nil.
func (a *App) playIn(win Window, server, item string, options *capiv1.PlaybackOptions, next ...*capiv1.Card) {
	if options == nil {
		options = &capiv1.PlaybackOptions{}
	}
	p := &player{server: server, item: item, next: next, options: options, input: time.Now(), win: win}
	a.players = append(a.players, p)
	if win != nil {
		win.Focus()
	}
	if a.opts.NewVideo != nil {
		p.video, p.err = a.opts.NewVideo(win, func(s mpv.State) { a.videoChanged(p, s) })
	}
	if a.opts.KeepAwake != nil {
		p.release = a.opts.KeepAwake("Playing")
	}
	a.startPlayback(p)
}

// startPlayback asks the server how to play p's item with its options.
func (a *App) startPlayback(p *player) {
	p.starting++
	n := p.starting
	options := proto.CloneOf(p.options)
	options.SetMaxBitrate(a.quality().bitrate)
	caps, err := proto.Marshal(capabilities(options.GetMaxBitrate()))
	if err != nil {
		p.err = err
		return
	}
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{Play: capiv1.Play_builder{
			ServerId: p.server, ItemId: p.item, Capabilities: caps, Options: options,
		}.Build()}.Build())
		a.opts.Update(func() {
			if err != nil {
				if a.playing(p) && p.starting == n {
					p.err = err
				}
				return
			}
			pb := m.GetPlayback()
			if !a.playing(p) || p.starting != n {
				a.stopSession(pb, true, p.state.Position)
				return
			}
			p.playback, p.selected = pb, false
			if p.video == nil {
				return
			}
			p.err = nil
			start := time.Duration(options.GetStartPositionMs()) * time.Millisecond
			if pb.HasStartPositionMs() {
				start = time.Duration(pb.GetStartPositionMs()) * time.Millisecond
			}
			if err := p.video.Load(pb.GetUrl(), start); err != nil {
				p.err = err
			}
		})
	}()
}

// restart plays p again where it is, with other streams or another
// quality.
func (a *App) restart(p *player, change func(o *capiv1.PlaybackOptions)) {
	if p.playback != nil {
		p.options.SetMediaSourceId(p.playback.GetMediaSourceId())
		a.stopSession(p.playback, true, p.state.Position)
		p.playback = nil
	}
	p.options.SetStartPositionMs(p.state.Position.Milliseconds())
	change(p.options)
	a.startPlayback(p)
}

// stopSession ends a playback's session, where it is unless played to the
// end.
func (a *App) stopSession(pb *capiv1.Playback, stopped bool, at time.Duration) {
	stop := capiv1.StopPlayback_builder{Session: pb.GetSession()}
	if stopped {
		stop.PositionMs = new(at.Milliseconds())
	}
	go func() {
		if _, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{StopPlayback: stop.Build()}.Build()); err != nil {
			slog.ErrorContext(a.ctx, "stop the playback", "error", err)
		}
		a.opts.Update(func() { a.generation++ })
	}()
}

// closePlayer leaves player p, ending its session where it is when
// stopped, else as played to the end, and closes its window.
func (a *App) closePlayer(p *player, stopped bool) {
	a.endPlayer(p, stopped)
	if p.win != nil {
		p.win.Close()
	}
}

// windowClosed ends the player of win, which closed.
func (a *App) windowClosed(win Window) {
	if p := a.playerOf(win); p != nil && win != nil {
		a.endPlayer(p, true)
	}
}

// endPlayer ends player p, as closePlayer does, but leaves its window.
func (a *App) endPlayer(p *player, stopped bool) {
	if !a.playing(p) {
		return
	}
	a.players = slices.DeleteFunc(a.players, func(q *player) bool { return q == p })
	if p.playback != nil {
		a.stopSession(p.playback, stopped, p.state.Position)
	}
	if p.release != nil {
		p.release()
	}
	if p.video != nil {
		go p.video.Close()
	}
}

// videoChanged takes the video's state, from its goroutine; the position
// alone redraws four times a second.
func (a *App) videoChanged(p *player, s mpv.State) {
	a.opts.Update(func() {
		if !a.playing(p) {
			return
		}
		prev := p.state
		p.state = s
		a.playerStateChanged(p, prev)
	})
}

// playerStateChanged acts on a change of p's video from prev.
func (a *App) playerStateChanged(p *player, prev mpv.State) {
	s := p.state
	if s.Err != nil && prev.Err == nil {
		p.err = s.Err
	}
	if p.playback == nil {
		return
	}
	if s.Loaded && !prev.Loaded {
		a.addSubtitles(p)
	}
	if s.Loaded && !p.selected && p.video != nil {
		a.selectStreams(p)
	}
	if s.Loaded && (time.Since(p.reported) >= time.Second || s.Paused != prev.Paused) {
		p.reported = time.Now()
		update := capiv1.UpdatePlayback_builder{Session: p.playback.GetSession(), PositionMs: s.Position.Milliseconds(), Paused: s.Paused}.Build()
		go func() {
			if _, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{UpdatePlayback: update}.Build()); err != nil {
				slog.WarnContext(a.ctx, "report the playback", "error", err)
			}
		}()
	}
	if s.Paused != prev.Paused {
		p.input = time.Now()
	}
	if s.Ended && !prev.Ended {
		if len(p.next) > 0 && a.autoplayNext() {
			a.playNext(p)
		} else {
			a.closePlayer(p, false)
		}
	}
}

// playNext plays the episode after p's in its place, in its window.
func (a *App) playNext(p *player) {
	stopped := !p.state.Ended
	a.endPlayer(p, stopped)
	a.playIn(p.win, p.next[0].GetServerId(), p.next[0].GetItemId(), nil, p.next[1:]...)
}

// addSubtitles adds the playback's subtitles delivered as files of their
// own, selecting the one chosen.
func (a *App) addSubtitles(p *player) {
	for _, s := range p.playback.GetSubtitles() {
		if s.GetUrl() == "" {
			continue
		}
		flag := "auto"
		if s.GetStreamIndex() == p.playback.GetSubtitleStreamIndex() {
			flag = "select"
		}
		if err := p.video.Command("sub-add", s.GetUrl(), flag, subtitleName(s), s.GetLanguage()); err != nil {
			slog.WarnContext(a.ctx, "add a subtitle", "error", err)
		}
	}
}

// selectStreams selects the audio and subtitles the server chose, by
// their indexes in the file when it plays as it is.
func (a *App) selectStreams(p *player) {
	pb := p.playback
	direct := pb.GetKind() == capiv1.PlayKind_PLAY_KIND_UNSPECIFIED
	if direct && pb.HasAudioStreamIndex() {
		if t, ok := findTrack(p.state.Tracks, "audio", int64(pb.GetAudioStreamIndex()), ""); ok {
			_ = p.video.Set("aid", strconv.FormatInt(t.ID, 10))
		}
	}
	if !pb.HasSubtitleStreamIndex() {
		p.selected = true
		return
	}
	index := pb.GetSubtitleStreamIndex()
	if index < 0 {
		_ = p.video.Set("sid", "no")
		p.selected = true
		return
	}
	var url string
	for _, s := range pb.GetSubtitles() {
		if s.GetStreamIndex() == index {
			url = s.GetUrl()
		}
	}
	switch {
	case url != "":
		// sub-add selected it once it loaded.
		if _, ok := findTrack(p.state.Tracks, "sub", -1, url); !ok {
			return
		}
	case direct:
		if t, ok := findTrack(p.state.Tracks, "sub", int64(index), ""); ok {
			_ = p.video.Set("sid", strconv.FormatInt(t.ID, 10))
		}
	default:
		// HLS carries the one chosen, or burns it in.
		if t, ok := findTrack(p.state.Tracks, "sub", -2, ""); ok {
			_ = p.video.Set("sid", strconv.FormatInt(t.ID, 10))
		}
	}
	p.selected = true
}

// findTrack finds a track of kind: by its URL when external is not
// empty, by its index in the file when index is not -2, else the first.
func findTrack(tracks []mpv.Track, kind string, index int64, external string) (mpv.Track, bool) {
	i := slices.IndexFunc(tracks, func(t mpv.Track) bool {
		switch {
		case t.Type != kind:
			return false
		case external != "":
			return t.External == external
		case index != -2:
			return t.External == "" && t.Index == index
		}
		return true
	})
	if i < 0 {
		return mpv.Track{}, false
	}
	return tracks[i], true
}

// quality returns the quality the user chose.
func (a *App) quality() quality {
	for _, q := range qualities {
		if q.key == a.prefs.Quality {
			return q
		}
	}
	return qualities[1]
}

// clock formats d as "1:02:03" or "2:03".
func clock(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d / time.Second)
	if h := s / 3600; h > 0 {
		return fmt.Sprintf("%d:%02d:%02d", h, s/60%60, s%60)
	}
	return fmt.Sprintf("%d:%02d", s/60, s%60)
}

// languageName names a language code in the app's language.
func languageName(c *ui.Context, code string) string {
	if code == "" || code == "und" {
		return ""
	}
	tag, err := language.Parse(code)
	if err != nil {
		return code
	}
	// No namer speaks a language display does not know.
	namer := display.Tags(language.Make(c.Locale()))
	if namer == nil {
		namer = display.Tags(language.English)
	}
	if name := namer.Name(tag); name != "" {
		return name
	}
	return code
}

func subtitleName(s *capiv1.SubtitleTrack) string {
	return cmpOr(s.GetTitle(), s.GetLanguage(), fmt.Sprintf("#%d", s.GetStreamIndex()))
}

func cmpOr(s ...string) string {
	for _, v := range s {
		if v != "" {
			return v
		}
	}
	return ""
}

// playerKeys handles the player's keys: space and k, the arrows and j/l,
// f, m; Escape closes the panel, then leaves full screen, then the player.
func (a *App) playerKeys(c *ui.Context, p *player) {
	pressed := func(keys ...ui.Key) bool {
		for _, k := range keys {
			if c.Shortcut(0, k) {
				p.input = time.Now()
				return true
			}
		}
		return false
	}
	switch {
	case pressed(ui.KeyEscape):
		switch {
		case p.panel != tabNone:
			p.panel = tabNone
		case a.isFullScreen(p):
			a.toggleFullScreen(p)
		default:
			a.closePlayer(p, true)
		}
	case pressed(ui.KeySpace, ui.KeyK):
		a.togglePause(p)
	case pressed(ui.KeyLeft, ui.KeyJ):
		a.seekBy(p, -10*time.Second)
	case pressed(ui.KeyRight, ui.KeyL):
		a.seekBy(p, 10*time.Second)
	case pressed(ui.KeyF):
		a.toggleFullScreen(p)
	case pressed(ui.KeyM):
		a.setVideo(p, "mute", yesNo(!p.state.Muted))
	}
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

func (a *App) setVideo(p *player, name, value string) {
	if p.video == nil {
		return
	}
	if err := p.video.Set(name, value); err != nil {
		slog.WarnContext(a.ctx, "set the video", "name", name, "error", err)
	}
}

func (a *App) togglePause(p *player) { a.setVideo(p, "pause", yesNo(!p.state.Paused)) }

func (a *App) seekBy(p *player, d time.Duration) {
	a.seekTo(p, p.state.Position+d, true)
}

// seekTo seeks to at; exactly unless dragging the scrubber.
func (a *App) seekTo(p *player, at time.Duration, exact bool) {
	if p.video == nil {
		return
	}
	at = max(0, at)
	if p.state.Duration > 0 {
		at = min(at, p.state.Duration)
	}
	mode := "absolute+exact"
	if !exact {
		mode = "absolute+keyframes"
	}
	p.state.Position = at
	if err := p.video.Command("seek", strconv.FormatFloat(at.Seconds(), 'f', 3, 64), mode); err != nil {
		slog.WarnContext(a.ctx, "seek", "error", err)
	}
}

// toggleFullScreen enters or leaves full screen in p's window.
func (a *App) toggleFullScreen(p *player) {
	switch {
	case p.win != nil:
		p.win.ToggleFullScreen()
	case a.opts.ToggleFullScreen != nil:
		a.opts.ToggleFullScreen()
	}
}

// isFullScreen reports whether p's window is in full screen.
func (a *App) isFullScreen(p *player) bool {
	if p.win != nil {
		return p.win.IsFullScreen()
	}
	return a.opts.IsFullScreen != nil && a.opts.IsFullScreen()
}

// playerWindowView builds a window of the player's: the player showing in
// it, titled after what it plays.
func (a *App) playerWindowView(c *ui.Context, win Window) {
	c.SetTheme(style.Theme(true))
	a.windowKeys(c)
	p := a.playerOf(win)
	if win == nil || p == nil {
		ui.Box(c).Fill().Background(ui.RGB(0, 0, 0)).DragWindow()
		return
	}
	if title := p.details.value.GetCard().GetTitle(); title != "" && title != p.title {
		p.title = title
		win.SetTitle(title)
	}
	a.playerView(c, p)
}

// playerView builds player p: the video under the on-screen controller
// (docs/clients.md §6.5).
func (a *App) playerView(c *ui.Context, p *player) {
	c.SetTheme(style.Theme(true))
	a.playerKeys(c, p)
	if !a.playing(p) {
		return
	}
	load(a, &p.details, key("details", p.server, p.item), func(ctx context.Context) (*capiv1.Details, error) {
		return a.getDetails(ctx, p.server, p.item)
	})
	now := c.Now()
	idle := now.Sub(p.input)
	shown := p.panel != tabNone || p.state.Paused || !p.state.Loaded || idle < controlsTimeout || p.err != nil
	if shown && idle < controlsTimeout {
		// Hidden then, until the pointer moves or a key is pressed.
		c.After(controlsTimeout - idle)
	}
	black := ui.RGB(0, 0, 0)
	root := ui.Box(c).Fill().Background(black)
	if x, y, ok := root.PointerPosition(); ok && (x != p.pointer[0] || y != p.pointer[1]) {
		p.pointer = [2]float32{x, y}
		p.input = now
	}
	root.Children(func() {
		if p.video != nil && p.video.View() != nil {
			ui.Video(c, p.video.View()).Absolute().Top(0).Left(0).Right(0).Bottom(0)
		}
		surface := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).DragWindow()
		if surface.DoubleClicked() {
			a.toggleFullScreen(p)
		} else if surface.Clicked() {
			if p.panel != tabNone {
				p.panel = tabNone
			} else {
				a.togglePause(p)
			}
		}
		if p.playback == nil && p.err == nil || p.state.Buffering {
			ui.Spinner(c).Absolute().Top(0).Left(0).Right(0).Bottom(0)
		}
		controls := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough()
		controls.Opacity(controls.Animate("shown", boolf(shown), 300*time.Millisecond))
		if shown {
			controls.Children(func() {
				a.playerTopBar(c, p)
				a.playerBottomBar(c, p)
			})
		}
		if p.panel != tabNone {
			a.playerPanel(c, p)
		}
		if p.err != nil {
			a.playerError(c, p)
		}
	})
}

func boolf(b bool) float32 {
	if b {
		return 1
	}
	return 0
}

// playerButton builds a round button of the controller.
func playerButton(c *ui.Context, icon, label string, fill, active bool, size float32) ui.Element {
	white := ui.RGB(255, 255, 255)
	b := ui.ButtonBase(c).Size(size, size).Radius(size / 2).Center().Label(label).Tooltip(label)
	switch {
	case b.Hovered():
		b.Background(white.Alpha(0.15))
	case active:
		b.Background(white.Alpha(0.2))
	}
	if b.Pressed() {
		b.Opacity(0.8)
	}
	iconSize := float32(22)
	if size > 44 {
		iconSize = 30
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, fill)).Size(iconSize, iconSize).TextColor(white)
	})
	return b
}

// playerTopBar builds the title and, in the app's window, the way back;
// a window of the player's closes as windows do. It drags the window.
func (a *App) playerTopBar(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	card := p.details.value.GetCard()
	bar := ui.Row(c).Absolute().Top(0).Left(0).Right(0).Padding(28, 20, 48).Gap(12).AlignItems(ui.Center).DragWindow().
		LinearGradient(ui.LinearGradient{Angle: 180, From: ui.RGBA(0, 0, 0, 0.75), To: ui.RGBA(0, 0, 0, 0)})
	bar.Children(func() {
		if p.win == nil && playerButton(c, "back", style.T(c, "player.back"), false, false, 44).Clicked() {
			a.closePlayer(p, true)
			return
		}
		ui.Column(c).MinWidth(0).Grow(1).Children(func() {
			if card.GetKind() == capiv1.CardKind_CARD_KIND_EPISODE && card.GetSubtitle() != "" {
				style.Text(ui.Text(c, card.GetSubtitle()), style.Type.Footnote).TextColor(white.Alpha(0.7)).SingleLine()
			}
			style.Text(ui.Text(c, card.GetTitle()), style.Type.Title3).FontWeight(600).TextColor(white).SingleLine()
		})
	})
}

// playerBottomBar builds the scrubber and the controls under it.
func (a *App) playerBottomBar(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	total := p.state.Duration
	if total == 0 {
		total = time.Duration(p.details.value.GetCard().GetRuntimeMs()) * time.Millisecond
	}
	ui.Column(c).Absolute().Left(0).Right(0).Bottom(0).Padding(64, 20, 16).Gap(8).
		LinearGradient(ui.LinearGradient{Angle: 0, Stops: []ui.GradientStop{
			{At: 0, Color: ui.RGBA(0, 0, 0, 0.85)}, {At: 0.5, Color: ui.RGBA(0, 0, 0, 0.4)}, {At: 1, Color: ui.RGBA(0, 0, 0, 0)},
		}}).
		Children(func() {
			a.scrubber(c, p, total)
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				label, icon := style.T(c, "player.pause"), "pause"
				if p.state.Paused {
					label, icon = style.T(c, "player.play"), "play"
				}
				if playerButton(c, icon, label, true, false, 48).Clicked() {
					a.togglePause(p)
				}
				if playerButton(c, "seekBack", style.T(c, "player.seekBack", "count", 10), false, false, 44).Clicked() {
					a.seekBy(p, -10*time.Second)
				}
				if playerButton(c, "seekForward", style.T(c, "player.seekForward", "count", 10), false, false, 44).Clicked() {
					a.seekBy(p, 10*time.Second)
				}
				label, icon = style.T(c, "player.mute"), "volume"
				if p.state.Muted {
					label, icon = style.T(c, "player.unmute"), "mute"
				}
				if playerButton(c, icon, label, false, false, 44).Clicked() {
					a.setVideo(p, "mute", yesNo(!p.state.Muted))
				}
				style.Text(ui.Text(c, clock(p.state.Position)+" / "+clock(total)), style.Type.Footnote).
					TextColor(white.Alpha(0.85)).FontFeatures("tnum").Margin(0, 0, 0, 8).SingleLine()
				ui.Spacer(c)
				if len(p.next) > 0 {
					a.nextEpisodeButton(c, p)
				}
				if len(p.playback.GetSubtitles()) > 0 {
					on := p.playback.GetSubtitleStreamIndex() >= 0
					if playerButton(c, "subtitles", style.T(c, "player.subtitles"), on, p.panel == tabSubtitles, 44).Clicked() {
						p.panel = togglePanel(p.panel, tabSubtitles)
					}
				}
				options := p.panel != tabNone && p.panel != tabSubtitles
				if playerButton(c, "settings", style.T(c, "player.options"), false, options, 44).Clicked() {
					p.panel = togglePanel(p.panel, tabAudio)
				}
				icon = "fullScreen"
				if a.isFullScreen(p) {
					icon = "exitFullScreen"
				}
				if playerButton(c, icon, style.T(c, "player.fullScreen"), false, false, 44).Clicked() {
					a.toggleFullScreen(p)
				}
			})
		})
}

func togglePanel(open, tab panelTab) panelTab {
	if open == tab || (tab == tabAudio && open != tabNone && open != tabSubtitles) {
		return tabNone
	}
	return tab
}

// nextEpisodeButton builds the pill playing the next episode.
func (a *App) nextEpisodeButton(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	label := style.T(c, "player.nextEpisode")
	b := ui.ButtonBase(c).Height(36).Padding(0, 14).Gap(6).Radius(18).AlignItems(ui.Center).Margin(0, 4, 0, 0).
		Background(white.Alpha(0.15)).Label(label)
	if b.Hovered() {
		b.Background(white.Alpha(0.25))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon("next", true)).Size(16, 16).TextColor(white)
		style.Text(ui.Text(c, label), style.Type.Footnote).FontWeight(600).TextColor(white).SingleLine()
	})
	if b.Clicked() {
		a.playNext(p)
	}
}

// scrubber builds the timeline: a slim bar that thickens under the
// pointer.
func (a *App) scrubber(c *ui.Context, p *player, total time.Duration) {
	white := ui.RGB(255, 255, 255)
	hi := max(total.Seconds(), 1)
	s := ui.SliderBase(c, &p.scrub, 0, hi).Height(20).Padding(0, 8).Label(style.T(c, "player.position"))
	if !s.Dragging() {
		p.scrub = min(p.state.Position.Seconds(), hi)
	}
	if s.Changed() {
		a.seekTo(p, time.Duration(p.scrub*float64(time.Second)), !s.Dragging())
	}
	active := s.Hovered() || s.Dragging()
	s.Draw(func(pt *ui.Painter, r ui.Rect) {
		h := float32(4)
		if active {
			h = 6
		}
		x0, w := r.X+8, r.W-16
		y := r.Y + (r.H-h)/2
		at := w * float32(p.scrub/hi)
		pt.Fill(ui.Rect{X: x0, Y: y, W: w, H: h}, white.Alpha(0.3), h/2)
		pt.Fill(ui.Rect{X: x0, Y: y, W: at, H: h}, white, h/2)
		if active {
			pt.Fill(ui.Rect{X: x0 + at - 7, Y: r.Y + r.H/2 - 7, W: 14, H: 14}, white, 7)
		}
	})
}

// playerPanel builds the side panel: the audio and speed, the subtitles
// and their offset, the quality, and how the item plays.
func (a *App) playerPanel(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	_, h := c.Size()
	tabs := []panelTab{tabAudio, tabSubtitles, tabQuality, tabInfo}
	selected := slices.Index(tabs, p.panel)
	// The glass's tint goes on a column of its own over the blur, as an
	// element's background takes the place of its material.
	ui.Column(c).Absolute().Right(16).Bottom(96).Width(360).MaxHeight(max(h-120, 200)).Radius(18).Clip().
		Material(glass.Blur{Radius: 40}).AlignItems(ui.Stretch).Children(func() {
		ui.Column(c).Grow(1).MinHeight(0).Radius(18).Background(ui.RGBA(23, 23, 23, 0.75)).Border(1, white.Alpha(0.1)).
			Children(func() {
				seg := ui.Segmented(c, &selected, style.T(c, "player.audio"), style.T(c, "player.subtitles"), style.T(c, "player.quality"), style.T(c, "player.info")).
					Label(style.T(c, "player.options")).Margin(12)
				if seg.Changed() {
					p.panel = tabs[selected]
				}
				ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
					ui.Column(c).Padding(0, 8, 12).Children(func() {
						switch p.panel {
						case tabAudio:
							a.audioTab(c, p)
						case tabSubtitles:
							a.subtitlesTab(c, p)
						case tabQuality:
							for _, q := range qualities {
								if choice(c.Key(q.key), qualityName(c, q), "", q.key == a.quality().key).Clicked() && q.key != a.quality().key {
									a.prefs.Quality = q.key
									a.savePreferences()
									a.restart(p, func(*capiv1.PlaybackOptions) {})
								}
							}
						case tabInfo:
							a.infoTab(c, p)
						}
					})
				})
			})
	})
}

// source returns the source p plays, of its details.
func (p *player) source() *capiv1.Source {
	for _, s := range p.details.value.GetSources() {
		if s.GetServerId() == p.server && s.GetMediaSourceId() == p.playback.GetMediaSourceId() {
			return s
		}
	}
	return nil
}

func (a *App) audioTab(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	for _, s := range p.source().GetAudio() {
		name := cmpOr(s.GetTitle(), languageName(c, s.GetLanguage()), fmt.Sprintf("#%d", s.GetIndex()))
		var details []string
		details = append(details, strings.ToUpper(s.GetCodec()))
		switch ch := s.GetChannels(); ch {
		case 0:
		case 6:
			details = append(details, "5.1")
		case 8:
			details = append(details, "7.1")
		default:
			details = append(details, fmt.Sprintf("%d ch", ch))
		}
		checked := p.playback.HasAudioStreamIndex() && s.GetIndex() == p.playback.GetAudioStreamIndex()
		if choice(c.Key(key("audio", s.GetIndex())), name, strings.Join(details, " · "), checked).Clicked() && !checked {
			index := s.GetIndex()
			a.restart(p, func(o *capiv1.PlaybackOptions) { o.SetAudioStreamIndex(index) })
		}
	}
	panelHeading(c, style.T(c, "player.speed"))
	ui.Row(c).Gap(6).Padding(0, 10).Wrap().Children(func() {
		for _, r := range speeds {
			on := r == p.state.Speed
			label := strconv.FormatFloat(r, 'f', -1, 64) + "×"
			b := ui.ButtonBase(c.Key(key("speed", r))).Height(32).Padding(0, 12).Radius(16).Center().Label(label).Background(white.Alpha(0.1))
			color := white
			switch {
			case on:
				b.Background(white)
				color = ui.RGB(0, 0, 0)
			case b.Hovered():
				b.Background(white.Alpha(0.2))
			}
			b.Children(func() {
				style.Text(ui.Text(c, label), style.Type.Footnote).FontWeight(600).FontFeatures("tnum").TextColor(color)
			})
			if b.Clicked() {
				a.setVideo(p, "speed", strconv.FormatFloat(r, 'f', -1, 64))
			}
		}
	})
}

func (a *App) subtitlesTab(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	current := p.playback.GetSubtitleStreamIndex()
	if !p.playback.HasSubtitleStreamIndex() {
		current = -1
	}
	if choice(c.Key(key("sub", -1)), style.T(c, "player.subtitlesOff"), "", current < 0).Clicked() && current >= 0 {
		a.restart(p, func(o *capiv1.PlaybackOptions) { o.SetSubtitleStreamIndex(-1) })
	}
	for _, s := range p.playback.GetSubtitles() {
		name := cmpOr(s.GetTitle(), languageName(c, s.GetLanguage()), fmt.Sprintf("#%d", s.GetStreamIndex()))
		var details []string
		details = append(details, strings.ToUpper(s.GetFormat()))
		if s.GetForced() {
			details = append(details, "Forced")
		}
		checked := s.GetStreamIndex() == current
		if choice(c.Key(key("sub", s.GetStreamIndex())), name, strings.Join(details, " · "), checked).Clicked() && !checked {
			index := s.GetStreamIndex()
			a.restart(p, func(o *capiv1.PlaybackOptions) { o.SetSubtitleStreamIndex(index) })
		}
	}
	if current < 0 {
		return
	}
	panelHeading(c, style.T(c, "player.subtitleOffset"))
	ui.Row(c).Gap(8).Padding(0, 10).AlignItems(ui.Center).Children(func() {
		delay := p.state.SubtitleDelay
		step := func(label string, d time.Duration) {
			b := ui.ButtonBase(c.Key(key("delay", label))).Size(32, 32).Radius(16).Center().Label(label).Background(white.Alpha(0.1))
			if b.Hovered() {
				b.Background(white.Alpha(0.2))
			}
			b.Children(func() { style.Text(ui.Text(c, label), style.Type.Headline).TextColor(white) })
			if b.Clicked() {
				a.setVideo(p, "sub-delay", strconv.FormatFloat((delay+d).Seconds(), 'f', 3, 64))
			}
		}
		step("−", -100*time.Millisecond)
		style.Text(ui.Text(c, fmt.Sprintf("%+.1f s", delay.Seconds())), style.Type.Callout).FontFeatures("tnum").TextColor(white).
			MinWidth(64).TextAlign(ui.Center)
		step("+", 100*time.Millisecond)
	})
}

func (a *App) infoTab(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	fact := func(label, value string) {
		ui.Column(c.Key(key("fact", label))).Padding(4, 10).Gap(2).Children(func() {
			style.Text(ui.Text(c, label), style.Type.Footnote).FontWeight(600).TextColor(white.Alpha(0.6))
			style.Text(ui.Text(c, value), style.Type.Callout).TextColor(white)
		})
	}
	if p.playback == nil {
		return
	}
	method := style.T(c, "player.directPlay")
	switch p.playback.GetKind() {
	case capiv1.PlayKind_PLAY_KIND_DIRECT_STREAM:
		method = style.T(c, "player.directStream")
	case capiv1.PlayKind_PLAY_KIND_TRANSCODE:
		method = style.T(c, "player.transcode")
	}
	fact(style.T(c, "player.method"), method)
	if s := p.source(); s != nil {
		var parts []string
		if s.GetName() != "" {
			parts = append(parts, s.GetName())
		}
		if s.GetContainer() != "" {
			parts = append(parts, strings.ToUpper(s.GetContainer()))
		}
		fact(style.T(c, "player.source"), strings.Join(parts, " · "))
	}
}

func panelHeading(c *ui.Context, title string) {
	style.Text(ui.Text(c, title), style.Type.Footnote).FontWeight(600).TextColor(ui.RGB(255, 255, 255).Alpha(0.6)).Padding(16, 10, 8)
}

// choice builds a choice of a list in the panel, checked or not.
func choice(c *ui.Context, name, details string, checked bool) ui.Element {
	white := ui.RGB(255, 255, 255)
	b := ui.ButtonBase(c).FillWidth().Padding(8, 10).Gap(12).Radius(10).AlignItems(ui.Center).Label(name)
	if b.Hovered() {
		b.Background(white.Alpha(0.1))
	}
	b.Children(func() {
		ui.Box(c).Size(16, 16).Children(func() {
			if checked {
				ui.Icon(c, style.Icon("check", false)).Size(16, 16).TextColor(style.Color(design.DarkColors.Accent))
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			style.Text(ui.Text(c, name), style.Type.Callout).TextColor(white).SingleLine()
			if details != "" {
				style.Text(ui.Text(c, details), style.Type.Caption).TextColor(white.Alpha(0.6)).SingleLine()
			}
		})
	})
	return b
}

// playerError builds why the item cannot play, with the way back.
func (a *App) playerError(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(ui.RGBA(0, 0, 0, 0.7)).
		AlignItems(ui.Center).Justify(ui.Center).Gap(12).Padding(24).Children(func() {
		ui.Icon(c, style.Icon("error", false)).Size(48, 48).TextColor(white.Alpha(0.7))
		style.Text(ui.Text(c, errorMessage(c, p.err)), style.Type.Body).TextColor(white).TextAlign(ui.Center).MaxWidth(420)
		b := ui.ButtonBase(c).Height(36).Padding(0, 16).Radius(10).Center().Background(white.Alpha(0.15)).Label(style.T(c, "player.back"))
		if b.Hovered() {
			b.Background(white.Alpha(0.25))
		}
		b.Children(func() {
			style.Text(ui.Text(c, style.T(c, "player.back")), style.Type.Headline).TextColor(white)
		})
		if b.Clicked() {
			a.closePlayer(p, true)
		}
	})
}

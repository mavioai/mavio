package app

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"math"
	"os"
	"path/filepath"
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
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"

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
	// ContentSize and SetContentSize are the size of what the window
	// shows, and Position and SetPosition where its frame is, in DIPs.
	ContentSize() (width, height int)
	SetContentSize(width, height int)
	Position() (x, y int)
	SetPosition(x, y int)
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

// localAuto is the bitrate Auto allows from a server on the local
// network, where the internet's limits do not hold.
const localAuto = 100_000_000

// qualities are the qualities, in the order they are offered.
var qualities = []quality{
	{"original", 200_000_000},
	{"auto", 40_000_000},
	{"1080p", 10_000_000},
	{"720p", 4_000_000},
	{"480p", 1_500_000},
}

var speeds = []float64{0.5, 0.75, 1, 1.25, 1.5, 1.75, 2}

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
	// loading tells that a file loads until its first frame shows, which
	// mpv's count of restarts passing loaded tells.
	loading bool
	loaded  int
	// selected tells that the playback's streams were selected in mpv.
	selected bool
	// input is when the user last moved the pointer or pressed a key.
	input   time.Time
	pointer [2]float32
	panel   panelTab
	// scrub is the scrubber's value, in seconds, and scrubbing tells that
	// the pointer holds it, its seeks to keyframes until it lets go.
	scrub     float64
	scrubbing bool
	// reported is when the position was last sent to the core.
	reported time.Time
	release  func()
	// win is the window the player shows in, nil for the app's, and title
	// the title it was given.
	win   Window
	title string
	// hidden tells that the TV mode's back or play hid the controller,
	// until a key is pressed or the pointer moves (docs/tv.md §7).
	hidden bool
	// want is the button of the controller's row the TV mode focuses next,
	// its place in the row plus one, -1 taking focus off the row and 0
	// none; rowAt is the row's button with focus, -1 for none, rowLen how
	// many buttons the row has, and opener the one that opened the panel.
	// focusChoice is for how many frames more the panel focuses its
	// current choice as it opens.
	want, rowAt, rowLen, opener, focusChoice int
	// volume is mpv's, from 0 to 100, which the assistant, the slider,
	// the keys and the wheel set; volumeValue is the slider's.
	volume      int
	volumeValue float64
	// following is the episode after p's beyond its season, looked up
	// when next has none.
	following task[*capiv1.Card]
	// wheelAt is when the wheel or trackpad last turned, wheelSeek tells
	// that its gesture seeks, else it sets the volume, and wheelTarget is
	// where it seeks to.
	wheelAt     time.Time
	wheelSeek   bool
	wheelTarget time.Duration
	// notice is what the player tells a moment over the picture.
	notice playerNotice
	// fitted is the size of the picture the window was fitted to.
	fitted [2]int64
	// menu is the menu of the controller's row that is open.
	menu rowMenu
	// seek is the seek the picture waits for.
	seek playerSeek
	// subPos is where mpv places the subtitles, in percent of the height.
	subPos int
	// recovered tells that the video was loaded again after it failed.
	recovered bool
	// swallow tells that a press outside a menu closed it, which the
	// picture then ignores.
	swallow bool
}

// rowMenu is a menu of the controller's row, as the Mac app's.
type rowMenu int

const (
	menuNone rowMenu = iota
	menuSpeed
	menuSubtitles
	menuAudio
	menuQuality
	menuInfo
)

// playerSeek is a seek the picture waits for, as the web UI's: for a beat
// nothing changes, so that one within what is buffered does not flash, and
// after it the old frame hides behind the activity indicator until mpv
// starts again at the new time. The timeline shows target meanwhile.
type playerSeek struct {
	pending bool
	// at is when the seek began, restarts mpv's count of restarts then.
	at       time.Time
	target   time.Duration
	restarts int
}

// seekBeat is how long a seek shows the old frame.
const seekBeat = 200 * time.Millisecond

// playerNotice is what the player tells a moment over the picture, as the
// volume, the speed or where the wheel seeks to: an icon, a text and a
// level from 0 to 1, none when negative.
type playerNotice struct {
	icon, text string
	level      float64
	at         time.Time
}

// noticeTime is how long a notice shows, and noticeFade how long of that
// it fades out.
const noticeTime, noticeFade = 1500 * time.Millisecond, 500 * time.Millisecond

// thumbWidth is the width of the timeline's thumbnails, in DIPs.
const thumbWidth = 200

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
// video when the preferences ask, else in place of what plays. In the TV
// mode it plays in the app's window, which a remote stays in.
func (a *App) play(server, item string, options *capiv1.PlaybackOptions, next ...*capiv1.Card) {
	windows := a.opts.NewWindow != nil && !a.prefs.TV
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
	p := &player{server: server, item: item, next: next, options: options, win: win, volume: 100}
	if a.prefs.Volume != nil {
		p.volume = *a.prefs.Volume
	}
	a.players = append(a.players, p)
	if win != nil {
		win.Focus()
	}
	if a.opts.NewVideo != nil {
		p.video, p.err = a.opts.NewVideo(win, func(s mpv.State) { a.videoChanged(p, s) })
	}
	if p.video != nil {
		// The sound as the last video left it.
		if p.volume != 100 {
			a.setVideo(p, "volume", strconv.Itoa(p.volume))
		}
		if a.prefs.AudioDevice != "" {
			a.setVideo(p, "audio-device", a.prefs.AudioDevice)
		}
		if a.prefs.AudioDelay != 0 {
			a.setVideo(p, "audio-delay", strconv.FormatFloat(a.prefs.AudioDelay, 'f', 3, 64))
		}
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
	options.SetMaxBitrate(a.bitrate(p.server))
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
			if err := a.loadVideo(p, pb.GetUrl(), start); err != nil {
				p.err = err
			}
		})
	}()
}

// loadVideo loads url into p's video from start, loading until its first
// frame shows.
func (a *App) loadVideo(p *player, url string, start time.Duration) error {
	if err := p.video.Load(url, start); err != nil {
		return err
	}
	p.loading, p.loaded = true, p.state.Restarts
	return nil
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
	// The volume, which the next video takes.
	a.savePreferences()
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
		if p.loading && (s.Restarts > p.loaded || s.Err != nil) {
			p.loading = false
		}
		if p.seek.pending {
			if s.Restarts > p.seek.restarts || !s.Loaded || s.Err != nil {
				p.seek = playerSeek{}
			} else {
				p.state.Position = p.seek.target
			}
		}
		a.playerStateChanged(p, prev)
	})
}

// playerStateChanged acts on a change of p's video from prev.
func (a *App) playerStateChanged(p *player, prev mpv.State) {
	s := p.state
	if s.Err != nil && prev.Err == nil {
		// As the web UI rebuilds its pipeline after a media error, the
		// video loads once more where it was, decoded in software, as when
		// a hardware decoder fails on HEVC.
		if !p.recovered && p.playback != nil && p.video != nil && prev.Loaded {
			p.recovered, p.selected = true, false
			slog.WarnContext(a.ctx, "the video failed; load it again", "error", s.Err)
			a.setVideo(p, "hwdec", "no")
			if err := a.loadVideo(p, p.playback.GetUrl(), prev.Position); err == nil {
				return
			}
		}
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
	if s.Width > 0 && s.Height > 0 && [2]int64{s.Width, s.Height} != p.fitted {
		a.fitWindow(p)
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
		// The controller shows a while from the next frame.
		p.input = time.Time{}
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

// bitrate returns the bitrate the quality chosen allows from a server:
// Auto allows more from one on the local network.
func (a *App) bitrate(server string) int64 {
	q := a.quality()
	if q.key != "auto" {
		return q.bitrate
	}
	i := slices.IndexFunc(a.servers, func(s *capiv1.ServerRecord) bool { return s.GetId() == server })
	if i >= 0 && a.servers[i].GetLocal() {
		return localAuto
	}
	return q.bitrate
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

// playerKeys handles the player's keys, as the Mac app's: space and k
// play and pause, left and right and j and l seek, up and down set the
// volume and m mutes, [ and ] change the speed and \ resets it, s saves a
// screenshot in the Pictures folder, f enters full screen; Escape closes
// the menu or the panel, then leaves full screen, then the player.
func (a *App) playerKeys(c *ui.Context, p *player) {
	// The assistant's panel over the player takes the keys.
	if a.assistant.open && a.assistant.win == p.win {
		return
	}
	pressed := func(keys ...ui.Key) bool {
		for _, k := range keys {
			if c.Shortcut(0, k) {
				p.input, p.hidden = c.Now(), false
				return true
			}
		}
		return false
	}
	if style.TV(c) {
		a.tvPlayerKeys(c, p, pressed)
		return
	}
	switch {
	case pressed(ui.KeyEscape):
		switch {
		case p.menu != menuNone:
			p.menu = menuNone
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
		a.toggleMute(c.Now(), p)
	case pressed(ui.KeyUp):
		a.changeVolume(c.Now(), p, 5)
	case pressed(ui.KeyDown):
		a.changeVolume(c.Now(), p, -5)
	case pressed(ui.KeyBracketLeft):
		a.stepSpeed(c.Now(), p, -1)
	case pressed(ui.KeyBracketRight):
		a.stepSpeed(c.Now(), p, 1)
	case pressed(ui.KeyBackslash):
		a.stepSpeed(c.Now(), p, 0)
	case pressed(ui.KeyS):
		a.screenshot(c, p)
	}
}

// changeVolume changes p's volume by d and tells it.
func (a *App) changeVolume(now time.Time, p *player, d int) {
	level := p.volume
	if p.state.Muted {
		level = 0
		a.setVideo(p, "mute", "no")
		p.state.Muted = false
	}
	a.setVolume(p, level+d)
	volumeNotice(now, p)
}

// toggleMute mutes p or restores its sound, and tells it.
func (a *App) toggleMute(now time.Time, p *player) {
	p.state.Muted = !p.state.Muted
	a.setVideo(p, "mute", yesNo(p.state.Muted))
	volumeNotice(now, p)
}

func volumeNotice(now time.Time, p *player) {
	level := p.volume
	icon := "volume"
	if p.state.Muted || level == 0 {
		level, icon = 0, "mute"
	}
	p.notice = playerNotice{icon: icon, text: strconv.Itoa(level) + "%", level: float64(level) / 100, at: now}
}

// stepSpeed sets p's speed to the next of speeds up or down, by step's
// sign, or back to normal when step is 0, and tells it.
func (a *App) stepSpeed(now time.Time, p *player, step int) {
	speed := 1.0
	if step != 0 {
		i := slices.Index(speeds, p.state.Speed)
		if i < 0 {
			i = slices.Index(speeds, 1.0)
		}
		speed = speeds[min(max(i+step, 0), len(speeds)-1)]
	}
	p.state.Speed = speed
	label := strconv.FormatFloat(speed, 'f', -1, 64)
	a.setVideo(p, "speed", label)
	p.notice = playerNotice{icon: "speed", text: label + "×", level: -1, at: now}
}

// screenshot saves the picture, without the subtitles, as a PNG in the
// Pictures folder, named after what plays and where, and tells it.
func (a *App) screenshot(c *ui.Context, p *player) {
	home, err := os.UserHomeDir()
	if err != nil || p.video == nil {
		return
	}
	dir := filepath.Join(home, "Pictures")
	title := cmpOr(p.details.value.GetCard().GetTitle(), "Mavio")
	base := strings.NewReplacer("/", "-", ":", ".").Replace(title + " " + clock(p.state.Position))
	path := filepath.Join(dir, base+".png")
	for n := 2; ; n++ {
		if _, err := os.Stat(path); err != nil {
			break
		}
		path = filepath.Join(dir, fmt.Sprintf("%s %d.png", base, n))
	}
	if err := p.video.Command("screenshot-to-file", path, "video"); err != nil {
		p.notice = playerNotice{icon: "error", text: err.Error(), level: -1, at: c.Now()}
		return
	}
	p.notice = playerNotice{icon: "photos", text: style.T(c, "player.screenshotSaved"), level: -1, at: c.Now()}
}

// playerScrolled takes a turn of the wheel or trackpad over the picture,
// or over the controller when bar: across seeks and up and down set the
// volume, the axis held for the gesture, as the Mac app's; over the
// controller up and down seek too. MyGo tells not a gesture's phases, so
// one ends as the turns pause.
func (a *App) playerScrolled(p *player, ev ui.InputEvent, bar bool) {
	now := ev.Time
	p.input, p.hidden = now, false
	// Fingers to the right and up, with the system's natural scrolling.
	right, up := -float64(ev.DX), float64(ev.DY)
	if right == 0 && up == 0 {
		return
	}
	gesture := now.Sub(p.wheelAt) < 300*time.Millisecond
	p.wheelAt = now
	if !gesture {
		p.wheelSeek = bar || math.Abs(right) > math.Abs(up)
		p.wheelTarget = p.state.Position
	}
	// A wheel turns by lines of 40 DIPs: 5 seconds or 5% a line.
	perSecond, perLevel := 0.25, 0.5
	if !ev.Precise {
		perSecond, perLevel = 5.0/40, 5.0/40
	}
	if !p.wheelSeek {
		a.changeVolume(now, p, int(math.Round(up*perLevel)))
		return
	}
	delta := right
	if bar && math.Abs(up) > math.Abs(right) {
		delta = up
	}
	at := p.wheelTarget + seconds(delta*perSecond)
	at = max(at, 0)
	if p.state.Duration > 0 {
		at = min(at, p.state.Duration)
	}
	p.wheelTarget = at
	a.seekTo(p, at, true)
	icon := "seekForward"
	if delta < 0 {
		icon = "seekBack"
	}
	level := -1.0
	if p.state.Duration > 0 {
		level = at.Seconds() / p.state.Duration.Seconds()
	}
	p.notice = playerNotice{icon: icon, text: clock(at) + " / " + clock(p.state.Duration), level: level, at: now}
}

// playerMinWidth and playerMinHeight are the least size of a window of
// the player's.
const playerMinWidth, playerMinHeight = 480, 270

// fitWindow fits p's window to the aspect of its picture, keeping its area
// and its center, unless it is in full screen; MyGo cannot lock the
// window to the aspect as it is resized.
func (a *App) fitWindow(p *player) {
	s := p.state
	p.fitted = [2]int64{s.Width, s.Height}
	if p.win == nil || p.win.IsFullScreen() {
		return
	}
	w, h := p.win.ContentSize()
	if w <= 0 || h <= 0 {
		return
	}
	aspect := float64(s.Width) / float64(s.Height)
	tw := math.Sqrt(float64(w*h) * aspect)
	th := tw / aspect
	grow := max(1, playerMinWidth/tw, playerMinHeight/th)
	nw, nh := int(math.Round(tw*grow)), int(math.Round(th*grow))
	if nw == w && nh == h {
		return
	}
	x, y := p.win.Position()
	p.win.SetPosition(x+(w-nw)/2, y+(h-nh)/2)
	p.win.SetContentSize(nw, nh)
}

// tvPlayerKeys handles the player's keys in the TV mode, as a remote's
// (docs/tv.md §7): back closes the panel, then hides the controller, then
// leaves; down shows the controller, and again moves focus into its row,
// along which left and right move and which up leaves; elsewhere left and
// right seek, and select pauses, keeping the controller up, or plays,
// hiding it. The panel's choices take the arrows while it is open.
func (a *App) tvPlayerKeys(c *ui.Context, p *player, pressed func(...ui.Key) bool) {
	shown := p.controlsShown(c.Now())
	switch {
	case pressed(ui.KeyEscape, ui.KeyBackspace):
		switch {
		case p.panel != tabNone:
			p.panel, p.want = tabNone, p.opener+1
		case shown:
			p.hidden, p.want = true, -1
		default:
			a.closePlayer(p, true)
		}
	case p.panel != tabNone:
		c.SpatialFocus()
	case p.rowAt >= 0:
		switch {
		case pressed(ui.KeyUp):
			p.want = -1
		case pressed(ui.KeyLeft):
			if p.rowAt == 0 {
				c.ShakeFocus()
			} else {
				p.want = p.rowAt
			}
		case pressed(ui.KeyRight):
			if p.rowAt == p.rowLen-1 {
				c.ShakeFocus()
			} else {
				p.want = p.rowAt + 2
			}
		case pressed(ui.KeyDown):
			c.ShakeFocus()
		}
	case pressed(ui.KeyDown):
		if shown {
			p.want = 1
		}
	case pressed(ui.KeyLeft):
		a.seekBy(p, -10*time.Second)
	case pressed(ui.KeyRight):
		a.seekBy(p, 10*time.Second)
	case pressed(ui.KeyEnter, ui.KeySpace):
		p.hidden = p.state.Paused
		a.togglePause(p)
	}
}

// controlsShown reports whether the controller shows at now: while the
// panel or a menu is open or the item cannot play, and, unless hidden,
// while it is paused or loading and for a while after the last input.
func (p *player) controlsShown(now time.Time) bool {
	return p.panel != tabNone || p.menu != menuNone || p.err != nil ||
		!p.hidden && (p.state.Paused || !p.state.Loaded || now.Sub(p.input) < controlsTimeout)
}

// rowFocus makes b the next button of the controller's row in the TV
// mode, focused as the keys move focus, and reports whether it has focus.
func (p *player) rowFocus(b ui.Element) bool {
	i := p.rowLen
	p.rowLen++
	b.FocusRing(false)
	if p.want == i+1 {
		b.Focus()
		p.want = 0
	}
	if !b.Focused() {
		return false
	}
	p.rowAt = i
	return true
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
	} else if p.state.Loaded {
		// A seek while the old frame hides keeps it hidden.
		if !p.seek.pending {
			p.seek = playerSeek{pending: true, restarts: p.state.Restarts}
		}
		p.seek.target = at
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
	c.SetTheme(style.Theme(true, false))
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
	a.assistantPanel(c, win)
}

// playerView builds player p: the video under the on-screen controller
// (docs/clients.md §6.5).
func (a *App) playerView(c *ui.Context, p *player) {
	c.SetTheme(style.Theme(true, style.TV(c)))
	a.playerKeys(c, p)
	if !a.playing(p) {
		return
	}
	tv := style.TV(c)
	p.rowAt, p.rowLen = -1, 0
	load(a, &p.details, key("details", p.server, p.item), func(ctx context.Context) (*capiv1.Details, error) {
		return a.getDetails(ctx, p.server, p.item)
	})
	a.followingEpisode(p)
	now := c.Now()
	if p.input.IsZero() {
		// The controller shows as the player opens, and as it pauses or
		// plays on.
		p.input = now
	}
	idle := now.Sub(p.input)
	shown := p.controlsShown(now)
	if shown && idle < controlsTimeout {
		// Hidden then, until the pointer moves or a key is pressed.
		c.After(controlsTimeout - idle)
	}
	black := ui.RGB(0, 0, 0)
	root := ui.Box(c).Fill().Background(black)
	if x, y, ok := root.PointerPosition(); ok && (x != p.pointer[0] || y != p.pointer[1]) {
		p.pointer = [2]float32{x, y}
		p.input, p.hidden = now, false
	}
	root.Children(func() {
		if p.video != nil && p.video.View() != nil {
			ui.Video(c, p.video.View()).Absolute().Top(0).Left(0).Right(0).Bottom(0)
		}
		surface := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).DragWindow()
		if !shown && !tv {
			// The pointer hides with the controller.
			surface.Cursor(ui.CursorNone)
		}
		if !tv {
			surface.HandleInput(func(ev ui.InputEvent) bool {
				if ev.Kind != ui.InputScroll {
					return false
				}
				a.playerScrolled(p, ev, false)
				return true
			})
		}
		if tv && p.panel == tabNone {
			// Focus off the controller's row is on an element of its own,
			// which shows it nowhere and leaves select to the keys.
			sink := ui.Box(c).Absolute().Top(0).Left(0).Size(1, 1).PassThrough().Focusable().FocusRing(false)
			if p.want == -1 {
				sink.Focus()
				p.want = 0
			}
		}
		clicked := surface.Clicked()
		if p.swallow && (clicked || !surface.Pressed()) {
			p.swallow, clicked = false, false
		}
		if surface.DoubleClicked() {
			a.toggleFullScreen(p)
		} else if clicked {
			if p.panel != tabNone {
				p.panel = tabNone
			} else {
				a.togglePause(p)
			}
		}
		// After a seek's beat the old frame hides, the subtitles drawn on
		// it too, behind the activity indicator.
		hiding := false
		if p.seek.pending {
			if p.seek.at.IsZero() {
				p.seek.at = now
			}
			if since := now.Sub(p.seek.at); since >= seekBeat {
				hiding = true
			} else {
				c.After(seekBeat - since)
			}
		}
		if hiding {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(black).PassThrough()
		}
		if (p.playback == nil || p.loading) && p.err == nil || p.state.Buffering && !p.seek.pending || hiding {
			ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Center().PassThrough().Children(func() {
				ui.Spinner(c).Size(48, 48).TextColor(ui.RGBA(255, 255, 255, 0.7))
			})
		}
		controls := ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).PassThrough()
		controls.Opacity(controls.Animate("shown", boolf(shown), 300*time.Millisecond))
		lift := float32(0)
		if shown {
			controls.Children(func() {
				a.playerTopBar(c, p)
				// The subtitles 8 DIPs above the controls.
				lift = a.playerBottomBar(c, p) + 8
			})
		}
		a.liftSubtitles(c, p, lift)
		if p.panel != tabNone {
			a.playerPanel(c, p)
		}
		if since := now.Sub(p.notice.at); !p.notice.at.IsZero() && since < noticeTime {
			opacity := float32(1)
			if fade := since - (noticeTime - noticeFade); fade > 0 {
				opacity = 1 - float32(fade)/float32(noticeFade)
				c.After(16 * time.Millisecond)
			} else {
				c.After(-fade)
			}
			playerNoticeView(c, p.notice).Opacity(opacity)
		}
		if p.err != nil {
			a.playerError(c, p)
		}
	})
}

// playerNoticeView builds a notice at the picture's top leading corner, as
// the web UI's and the Mac app's: its icon and text, and its level as a
// bar after them.
func playerNoticeView(c *ui.Context, n playerNotice) ui.Element {
	white := ui.RGB(255, 255, 255)
	return ui.Row(c).Absolute().Top(80).Left(20).Padding(8, 16, 8, 14).Gap(12).Radius(10).AlignItems(ui.Center).
		Background(ui.RGBA(0, 0, 0, 0.6)).PassThrough().Children(func() {
		ui.Icon(c, style.Icon(n.icon, true)).Size(18, 18).TextColor(white)
		style.Text(ui.Text(c, n.text), style.Type(c).Headline).FontWeight(600).FontFeatures("tnum").TextColor(white).SingleLine()
		if n.level >= 0 {
			level := float32(min(n.level, 1))
			ui.Box(c).Size(100, 6).Draw(func(pt *ui.Painter, r ui.Rect) {
				pt.Fill(r, white.Alpha(0.3), 3)
				pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * level, H: r.H}, white, 3)
			})
		}
	})
}

// liftSubtitles places the subtitles above the controller while it shows,
// lift DIPs from the bottom, and back where they were as it hides, as the
// web UI's. mpv's place of 100 % keeps a margin of 22 of 720 under them.
func (a *App) liftSubtitles(c *ui.Context, p *player, lift float32) {
	_, h := c.Size()
	pos := 100
	if lift > 0 && h > 0 {
		pos = int(math.Round(100 * (1 - float64(max(lift-h*22/720, 0)/h))))
	}
	if pos != cmp.Or(p.subPos, 100) {
		p.subPos = pos
		a.setVideo(p, "sub-pos", strconv.Itoa(pos))
	}
}

// followingEpisode looks up the episode after p's beyond its season when
// no episode is to follow, as the web UI does: the series' episodes in
// order of their seasons and numbers, and the one after p's.
func (a *App) followingEpisode(p *player) {
	card := p.details.value.GetCard()
	if len(p.next) > 0 || card.GetKind() != capiv1.CardKind_CARD_KIND_EPISODE {
		return
	}
	t := load(a, &p.following, key("following", p.server, p.item), func(ctx context.Context) (*capiv1.Card, error) {
		items := a.itemService(p.server)
		it, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: &p.item}.Build())
		if err != nil {
			return nil, err
		}
		series := it.GetItem().GetSeriesId()
		if series == "" {
			return nil, nil
		}
		res, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{
			ParentId: &series, Recursive: new(true), Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_EPISODE},
		}.Build())
		if err != nil {
			return nil, err
		}
		episodes := slices.SortedStableFunc(slices.Values(res.GetItems()), func(x, y *libraryv1.Item) int {
			return cmp.Or(cmp.Compare(x.GetParentIndexNumber(), y.GetParentIndexNumber()), cmp.Compare(x.GetIndexNumber(), y.GetIndexNumber()))
		})
		i := slices.IndexFunc(episodes, func(e *libraryv1.Item) bool { return e.GetId() == p.item })
		if i < 0 || i+1 >= len(episodes) {
			return nil, nil
		}
		e := episodes[i+1]
		return capiv1.Card_builder{ServerId: p.server, ItemId: e.GetId(), Kind: capiv1.CardKind_CARD_KIND_EPISODE, Title: e.GetName()}.Build(), nil
	})
	if t.value != nil {
		p.next = []*capiv1.Card{t.value}
	}
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
		if p.win == nil && !style.TV(c) && playerButton(c, "back", style.T(c, "player.back"), false, false, 44).Clicked() {
			a.closePlayer(p, true)
			return
		}
		ui.Column(c).MinWidth(0).Grow(1).Children(func() {
			if card.GetKind() == capiv1.CardKind_CARD_KIND_EPISODE && card.GetSubtitle() != "" {
				style.Text(ui.Text(c, card.GetSubtitle()), style.Type(c).Footnote).TextColor(white.Alpha(0.7)).SingleLine()
			}
			style.Text(ui.Text(c, card.GetTitle()), style.Type(c).Title3).FontWeight(600).TextColor(white).SingleLine()
		})
	})
}

// playerBottomBar builds the scrubber and the controls under it, and
// returns how high above the bottom they reach, the subtitles' place over
// them.
func (a *App) playerBottomBar(c *ui.Context, p *player) float32 {
	white := ui.RGB(255, 255, 255)
	total := p.state.Duration
	if total == 0 {
		total = time.Duration(p.details.value.GetCard().GetRuntimeMs()) * time.Millisecond
	}
	// The gradient reaches over the picture, high above the controls.
	const fade = 64
	bar := ui.Column(c).Absolute().Left(0).Right(0).Bottom(0).Padding(fade, 20, 16).Gap(8)
	if !style.TV(c) {
		bar.HandleInput(func(ev ui.InputEvent) bool {
			if ev.Kind != ui.InputScroll {
				return false
			}
			a.playerScrolled(p, ev, true)
			return true
		})
	}
	bar.LinearGradient(ui.LinearGradient{Angle: 0, Stops: []ui.GradientStop{
		{At: 0, Color: ui.RGBA(0, 0, 0, 0.85)}, {At: 0.5, Color: ui.RGBA(0, 0, 0, 0.4)}, {At: 1, Color: ui.RGBA(0, 0, 0, 0)},
	}}).
		Children(func() {
			if style.TV(c) {
				a.tvPlayerRow(c, p, total)
				return
			}
			// The time played and the time left on either side of the
			// scrubber, as the Mac app's.
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				times := func(s string) {
					style.Text(ui.Text(c, s), style.Type(c).Footnote).TextColor(white.Alpha(0.85)).FontFeatures("tnum").SingleLine()
				}
				times(clock(p.state.Position))
				ui.Box(c).Grow(1).MinWidth(0).Children(func() { a.scrubber(c, p, total) })
				times("-" + clock(max(total-p.state.Position, 0)))
			})
			// As the Mac app's: playing and the sound on the leading side,
			// the menus and the window on the trailing one, with no buttons
			// seeking back and forward.
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				label, icon := style.T(c, "player.pause"), "pause"
				if p.state.Paused {
					label, icon = style.T(c, "player.play"), "play"
				}
				if playerButton(c, icon, label, true, false, 48).Clicked() {
					a.togglePause(p)
				}
				label, icon = style.T(c, "player.mute"), "volume"
				if p.state.Muted {
					label, icon = style.T(c, "player.unmute"), "mute"
				}
				if playerButton(c, icon, label, false, false, 44).Clicked() {
					a.toggleMute(c.Now(), p)
				}
				a.volumeSlider(c, p)
				if len(p.next) > 0 && playerButton(c, "next", style.T(c, "player.nextEpisode"), true, false, 44).Clicked() {
					a.playNext(p)
				}
				ui.Spacer(c)
				if a.assistantOn(c) {
					b := playerButton(c, "microphone", style.T(c, "assistant.title"), a.assistant.open, false, 44)
					switch {
					case a.voiceOn():
						a.voicePress(c, b, "player-button", p.win)
					case b.Clicked():
						a.openAssistant(p.win)
					}
				}
				a.rowMenu(c, p, menuSpeed, "speed", false, style.T(c, "player.speed"), func(close func()) { a.speedMenu(c, p, close) })
				if len(p.playback.GetSubtitles()) > 0 {
					on := p.playback.GetSubtitleStreamIndex() >= 0
					a.rowMenu(c, p, menuSubtitles, "subtitles", on, style.T(c, "player.subtitles"), func(close func()) { a.subtitlesMenu(c, p, close) })
				}
				a.rowMenu(c, p, menuAudio, "audio", false, style.T(c, "player.audio"), func(close func()) { a.audioMenu(c, p, close) })
				a.rowMenu(c, p, menuQuality, "quality", false, style.T(c, "player.quality"), func(close func()) { a.qualityMenu(c, p, close) })
				if p.playback != nil {
					a.rowMenu(c, p, menuInfo, "info", false, style.T(c, "player.info"), func(func()) { a.infoMenu(c, p) })
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
	return bar.LayoutBounds().H - fade
}

// tvPlayerRow builds the TV mode's timeline and, under it, the time and
// the row of buttons focus moves into: the next episode, the subtitles
// and the audio; the remote's keys play, pause and seek (docs/tv.md §7).
func (a *App) tvPlayerRow(c *ui.Context, p *player, total time.Duration) {
	white, black := ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
	at := float32(0)
	if total > 0 {
		at = min(1, float32(p.state.Position.Seconds()/total.Seconds()))
	}
	ui.Box(c).FillWidth().Height(6).Label(style.T(c, "player.position")).Draw(func(pt *ui.Painter, r ui.Rect) {
		pt.Fill(r, white.Alpha(0.25), 3)
		pt.Fill(ui.Rect{X: r.X, Y: r.Y, W: r.W * at, H: r.H}, white, 3)
	})
	ui.Row(c).Gap(12).AlignItems(ui.Center).Margin(4, 0, 0).Children(func() {
		style.Text(ui.Text(c, clock(p.state.Position)+" / "+clock(total)), style.Type(c).Callout).
			TextColor(white.Alpha(0.85)).FontFeatures("tnum").SingleLine()
		ui.Spacer(c)
		// While the panel is open, focus keeps within it.
		ui.Row(c).Gap(8).AlignItems(ui.Center).Disabled(p.panel != tabNone).Children(func() {
			open := func(tab panelTab) {
				p.input = c.Now()
				p.opener = p.rowAt
				p.panel = togglePanel(p.panel, tab)
				p.focusChoice = 0
				if p.panel != tabNone {
					p.focusChoice = 3
				}
			}
			if len(p.next) > 0 {
				label := style.T(c, "player.nextEpisode")
				b := ui.ButtonBase(c).Height(48).Padding(0, 20).Gap(8).Radius(24).AlignItems(ui.Center).
					Background(white.Alpha(0.15)).Label(label)
				color := white
				if p.rowFocus(b) {
					b.Background(white)
					color = black
				}
				b.Children(func() {
					ui.Icon(c, style.Icon("next", true)).Size(20, 20).TextColor(color)
					style.Text(ui.Text(c, label), style.Type(c).Callout).FontWeight(600).TextColor(color).SingleLine()
				})
				if b.Clicked() {
					a.playNext(p)
				}
			}
			if len(p.playback.GetSubtitles()) > 0 {
				on := p.playback.GetSubtitleStreamIndex() >= 0
				if tvPlayerButton(c, p, "subtitles", style.T(c, "player.subtitles"), on, p.panel == tabSubtitles) {
					open(tabSubtitles)
				}
			}
			if tvPlayerButton(c, p, "audio", style.T(c, "player.audio"), false, p.panel != tabNone && p.panel != tabSubtitles) {
				open(tabAudio)
			}
			if a.assistantOn(c) && tvPlayerButton(c, p, "microphone", style.T(c, "assistant.title"), a.assistant.open, false) {
				a.startAssistant(p.win, c.Locale())
			}
		})
	})
}

// tvPlayerButton builds a round button of the TV mode's row, white with
// focus, and reports whether it was pressed.
func tvPlayerButton(c *ui.Context, p *player, icon, label string, fill, active bool) bool {
	white := ui.RGB(255, 255, 255)
	b := ui.ButtonBase(c).Size(44, 44).Radius(22).Center().Label(label)
	color := white
	switch {
	case p.rowFocus(b):
		b.Background(white)
		color = ui.RGB(0, 0, 0)
	case active:
		b.Background(white.Alpha(0.2))
	case b.Hovered():
		b.Background(white.Alpha(0.15))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, fill)).Size(22, 22).TextColor(color)
	})
	return b.Clicked()
}

func togglePanel(open, tab panelTab) panelTab {
	if open == tab || (tab == tabAudio && open != tabNone && open != tabSubtitles) {
		return tabNone
	}
	return tab
}

// volumeSlider builds the slider of the volume beside the mute button.
func (a *App) volumeSlider(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	v := ui.SliderBase(c, &p.volumeValue, 0, 100).Width(96).Height(20).Padding(0, 6).Label(style.T(c, "player.volume"))
	held := v.Pressed() || v.Dragging()
	if v.Changed() {
		if p.state.Muted {
			p.state.Muted = false
			a.setVideo(p, "mute", "no")
		}
		a.setVolume(p, int(math.Round(p.volumeValue)))
	} else if !held {
		p.volumeValue = float64(p.volume)
		if p.state.Muted {
			p.volumeValue = 0
		}
	}
	active := v.Hovered() || held
	v.Draw(func(pt *ui.Painter, r ui.Rect) {
		h := float32(4)
		x0, w := r.X+6, r.W-12
		y := r.Y + (r.H-h)/2
		at := w * float32(p.volumeValue/100)
		pt.Fill(ui.Rect{X: x0, Y: y, W: w, H: h}, white.Alpha(0.3), h/2)
		pt.Fill(ui.Rect{X: x0, Y: y, W: at, H: h}, white, h/2)
		if active {
			pt.Fill(ui.Rect{X: x0 + at - 6, Y: r.Y + r.H/2 - 6, W: 12, H: 12}, white, 6)
		}
	})
}

// scrubber builds the timeline: a slim bar that thickens under the
// pointer, marked where the chapters start, and over the time under the
// pointer, as the Mac app's, its thumbnail, its chapter and the time.
func (a *App) scrubber(c *ui.Context, p *player, total time.Duration) {
	white := ui.RGB(255, 255, 255)
	hi := max(total.Seconds(), 1)
	chapters := p.source().GetChapters()
	ui.Box(c).FillWidth().Children(func() {
		s := ui.SliderBase(c, &p.scrub, 0, hi).FillWidth().Height(20).Padding(0, 8).Label(style.T(c, "player.position"))
		// The pointer's value, which Changed may have taken already in
		// this frame, is not overwritten by the position. A slider held
		// is not Dragging, which tells of Draggable's elements alone.
		held := s.Pressed() || s.Dragging()
		switch {
		case s.Changed():
			p.scrubbing = held
			a.seekTo(p, time.Duration(p.scrub*float64(time.Second)), !held)
		case held:
		case p.scrubbing:
			// Let go: exactly where it was let go.
			p.scrubbing = false
			a.seekTo(p, time.Duration(p.scrub*float64(time.Second)), true)
		default:
			p.scrub = min(p.state.Position.Seconds(), hi)
		}
		active := s.Hovered() || held
		s.Draw(func(pt *ui.Painter, r ui.Rect) {
			h := float32(4)
			if active {
				h = 6
			}
			// The track is inset by half the knob, so that the knob stays
			// within the bar at either end.
			x0, w := r.X+8, r.W-16
			y := r.Y + (r.H-h)/2
			at := w * float32(p.scrub/hi)
			// A gap of 2 DIPs where each chapter after the first starts,
			// cut out of one rounded track and its fill alike.
			cuts := []float32{x0}
			for _, ch := range chapters {
				if s := float64(ch.GetStartMs()) / 1000; s > 0 && s < hi {
					x := x0 + w*float32(s/hi)
					cuts = append(cuts, x-1, x+1)
				}
			}
			cuts = append(cuts, x0+w)
			pt.Clip(ui.Rect{X: x0, Y: y, W: w, H: h}, h/2, func() {
				for i := 0; i+1 < len(cuts); i += 2 {
					from, to := cuts[i], cuts[i+1]
					pt.Fill(ui.Rect{X: from, Y: y, W: to - from, H: h}, white.Alpha(0.3), 0)
					if fill := min(to, x0+at); fill > from {
						pt.Fill(ui.Rect{X: from, Y: y, W: fill - from, H: h}, white, 0)
					}
				}
			})
			if active {
				pt.Fill(ui.Rect{X: x0 + at - 7, Y: r.Y + r.H/2 - 7, W: 14, H: 14}, white, 7)
			}
		})
		width := s.LayoutBounds().W
		x, _, over := s.PointerPosition()
		if held {
			x, over = 8+(width-16)*float32(p.scrub/hi), true
		}
		if !over || width <= 16 || total <= 0 {
			return
		}
		at := time.Duration(min(max(float64((x-8)/(width-16)), 0), 1) * hi * float64(time.Second))
		left := min(max(x-thumbWidth/2, 0), width-thumbWidth)
		ui.Column(c).Absolute().Bottom(24).Left(left).Width(thumbWidth).Gap(4).AlignItems(ui.Center).PassThrough().Children(func() {
			a.trickplayThumbnail(c, p, at)
			if name := chapterAt(chapters, at); name != "" {
				style.Text(ui.Text(c, name), style.Type(c).Footnote).FontWeight(600).TextColor(white).SingleLine().MaxWidth(thumbWidth)
			}
			style.Text(ui.Text(c, clock(at)), style.Type(c).Footnote).FontWeight(600).FontFeatures("tnum").TextColor(white).SingleLine()
		})
	})
}

// chapterAt names the chapter playing at, empty for none.
func chapterAt(chapters []*capiv1.Chapter, at time.Duration) string {
	name := ""
	for _, ch := range chapters {
		if time.Duration(ch.GetStartMs())*time.Millisecond > at {
			break
		}
		name = ch.GetName()
	}
	return name
}

// chooseTrickplay returns the sheets whose thumbnails are at least wide
// pixels wide, the narrowest of those, else the widest; nil for none.
func chooseTrickplay(sheets []*capiv1.Trickplay, wide int32) *capiv1.Trickplay {
	var at, widest *capiv1.Trickplay
	for _, t := range sheets {
		if t.GetWidth() >= wide && (at == nil || t.GetWidth() < at.GetWidth()) {
			at = t
		}
		if widest == nil || t.GetWidth() > widest.GetWidth() {
			widest = t
		}
	}
	if at != nil {
		return at
	}
	return widest
}

// trickplayThumbnail builds the thumbnail of p's video at, cut from its
// sheet; nothing without sheets, and an empty frame while one loads.
func (a *App) trickplayThumbnail(c *ui.Context, p *player, at time.Duration) {
	t := chooseTrickplay(p.details.value.GetTrickplay(), int32(thumbWidth*scale))
	cols, rows, count, interval := t.GetColumns(), t.GetRows(), t.GetCount(), t.GetIntervalMs()
	if cols <= 0 || rows <= 0 || count <= 0 || interval <= 0 || t.GetWidth() <= 0 || t.GetHeight() <= 0 {
		return
	}
	index := min(max(int32(at.Milliseconds()/interval), 0), count-1)
	per := cols * rows
	bitmap := a.opts.Images.Get(fmt.Sprintf("%s/%d.jpg", t.GetUrl(), index/per), 0, 0)
	height := float32(thumbWidth * float64(t.GetHeight()) / float64(t.GetWidth()))
	box := ui.Box(c).Size(thumbWidth, height).Radius(6).Clip().Background(ui.RGB(0, 0, 0)).Border(1, ui.RGBA(255, 255, 255, 0.3))
	if bitmap == nil {
		return
	}
	// The sheet's own size, should it have been scaled.
	bw, bh := bitmap.Size()
	tileW, tileH := float32(bw)/float32(cols), float32(bh)/float32(rows)
	if tileW <= 0 || tileH <= 0 {
		return
	}
	k := thumbWidth / tileW
	tile := index % per
	box.Children(func() {
		ui.Image(c, bitmap).Absolute().Left(-float32(tile%cols)*thumbWidth).Top(-float32(tile/cols)*tileH*k).
			Size(float32(bw)*k, float32(bh)*k)
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
				seg := segmented(c, &selected, style.T(c, "player.audio"), style.T(c, "player.subtitles"), style.T(c, "player.quality"), style.T(c, "player.info")).
					Label(style.T(c, "player.options")).Margin(12)
				if seg.Changed() {
					p.panel = tabs[selected]
				}
				defer func() { p.focusChoice = max(p.focusChoice-1, 0) }()
				ui.Scroll(c).Grow(1).MinHeight(0).Children(func() {
					ui.Column(c).Padding(0, 8, 12).Children(func() {
						switch p.panel {
						case tabAudio:
							a.audioTab(c, p)
						case tabSubtitles:
							a.subtitlesTab(c, p)
						case tabQuality:
							for _, q := range qualities {
								if p.focusOpening(choice(c.Key(q.key), qualityName(c, q), "", q.key == a.quality().key), q.key == a.quality().key).Clicked() && q.key != a.quality().key {
									a.chooseQuality(p, q)
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
		checked := p.playback.HasAudioStreamIndex() && s.GetIndex() == p.playback.GetAudioStreamIndex()
		if p.focusOpening(choice(c.Key(key("audio", s.GetIndex())), name, audioDetails(s), checked), checked).Clicked() && !checked {
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
			case tvLit(c, b):
				color = ui.RGB(0, 0, 0)
			case on:
				b.Background(white.Alpha(0.3))
			case b.Hovered():
				b.Background(white.Alpha(0.2))
			}
			b.Children(func() {
				style.Text(ui.Text(c, label), style.Type(c).Footnote).FontWeight(600).FontFeatures("tnum").TextColor(color)
			})
			if b.Clicked() {
				a.setVideo(p, "speed", strconv.FormatFloat(r, 'f', -1, 64))
			}
		}
	})
	a.audioOffset(c, p)
	a.audioOutputs(c, p)
}

// audioOffset builds the offset of the sound, which the next videos keep.
func (a *App) audioOffset(c *ui.Context, p *player) {
	panelHeading(c, style.T(c, "player.audioOffset"))
	offsetRow(c, "audio", p.state.AudioDelay, func(d time.Duration) {
		a.prefs.AudioDelay = d.Seconds()
		a.setVideo(p, "audio-delay", strconv.FormatFloat(d.Seconds(), 'f', 3, 64))
	})
}

// audioOutputs builds the devices the sound can play to, the system's
// first, as the Mac app's Audio Output menu.
func (a *App) audioOutputs(c *ui.Context, p *player) {
	devices := p.state.AudioDevices
	if len(devices) < 2 {
		return
	}
	panelHeading(c, style.T(c, "player.audioDevice"))
	current := cmpOr(p.state.AudioDevice, "auto")
	for _, d := range devices {
		name := d.Description
		if d.Name == "auto" {
			name = style.T(c, "player.audioDeviceSystem")
		}
		checked := d.Name == current
		if p.focusOpening(choice(c.Key(key("output", d.Name)), cmpOr(name, d.Name), "", checked), checked).Clicked() && !checked {
			a.chooseOutput(p, d.Name)
		}
	}
}

// offsetRow builds the steps of an offset, a tenth of a second earlier or
// later, about the offset, set by set.
func offsetRow(c *ui.Context, id string, offset time.Duration, set func(time.Duration)) {
	white := ui.RGB(255, 255, 255)
	ui.Row(c.Key(key("offset", id))).Gap(8).Padding(0, 10).AlignItems(ui.Center).Children(func() {
		step := func(label string, d time.Duration) {
			b := ui.ButtonBase(c.Key(key("delay", id, label))).Size(32, 32).Radius(16).Center().Label(label).Background(white.Alpha(0.1))
			color := white
			switch {
			case tvLit(c, b):
				color = ui.RGB(0, 0, 0)
			case b.Hovered():
				b.Background(white.Alpha(0.2))
			}
			b.Children(func() { style.Text(ui.Text(c, label), style.Type(c).Headline).TextColor(color) })
			if b.Clicked() {
				set((offset + d).Round(time.Millisecond))
			}
		}
		step("−", -100*time.Millisecond)
		style.Text(ui.Text(c, fmt.Sprintf("%+.1f s", offset.Seconds())), style.Type(c).Callout).FontFeatures("tnum").TextColor(white).
			MinWidth(64).TextAlign(ui.Center)
		step("+", 100*time.Millisecond)
	})
}

func (a *App) subtitlesTab(c *ui.Context, p *player) {
	current := p.playback.GetSubtitleStreamIndex()
	if !p.playback.HasSubtitleStreamIndex() {
		current = -1
	}
	if p.focusOpening(choice(c.Key(key("sub", -1)), style.T(c, "player.subtitlesOff"), "", current < 0), current < 0).Clicked() && current >= 0 {
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
		if p.focusOpening(choice(c.Key(key("sub", s.GetStreamIndex())), name, strings.Join(details, " · "), checked), checked).Clicked() && !checked {
			index := s.GetStreamIndex()
			a.restart(p, func(o *capiv1.PlaybackOptions) { o.SetSubtitleStreamIndex(index) })
		}
	}
	if current < 0 {
		return
	}
	panelHeading(c, style.T(c, "player.subtitleOffset"))
	offsetRow(c, "sub", p.state.SubtitleDelay, func(d time.Duration) {
		a.setVideo(p, "sub-delay", strconv.FormatFloat(d.Seconds(), 'f', 3, 64))
	})
}

func (a *App) infoTab(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	fact := func(label, value string) {
		ui.Column(c.Key(key("fact", label))).Padding(4, 10).Gap(2).Children(func() {
			style.Text(ui.Text(c, label), style.Type(c).Footnote).FontWeight(600).TextColor(white.Alpha(0.6))
			style.Text(ui.Text(c, value), style.Type(c).Callout).TextColor(white)
		})
	}
	if p.playback == nil {
		return
	}
	fact(style.T(c, "player.method"), playMethod(c, p.playback))
	if s := p.source(); s != nil {
		fact(style.T(c, "player.source"), sourceFacts(s))
	}
}

// playMethod names how pb plays.
func playMethod(c *ui.Context, pb *capiv1.Playback) string {
	switch pb.GetKind() {
	case capiv1.PlayKind_PLAY_KIND_DIRECT_STREAM:
		return style.T(c, "player.directStream")
	case capiv1.PlayKind_PLAY_KIND_TRANSCODE:
		return style.T(c, "player.transcode")
	}
	return style.T(c, "player.directPlay")
}

// sourceFacts tells a source's name and container.
func sourceFacts(s *capiv1.Source) string {
	var parts []string
	if s.GetName() != "" {
		parts = append(parts, s.GetName())
	}
	if s.GetContainer() != "" {
		parts = append(parts, strings.ToUpper(s.GetContainer()))
	}
	return strings.Join(parts, " · ")
}

// audioDetails tells an audio stream's codec and channels.
func audioDetails(s *capiv1.AudioStream) string {
	details := []string{strings.ToUpper(s.GetCodec())}
	switch ch := s.GetChannels(); ch {
	case 0:
	case 6:
		details = append(details, "5.1")
	case 8:
		details = append(details, "7.1")
	default:
		details = append(details, fmt.Sprintf("%d ch", ch))
	}
	return strings.Join(details, " · ")
}

// chooseOutput plays p's sound to the device named, which the next videos
// keep.
func (a *App) chooseOutput(p *player, name string) {
	a.prefs.AudioDevice = name
	if name == "auto" {
		a.prefs.AudioDevice = ""
	}
	a.savePreferences()
	a.setVideo(p, "audio-device", name)
}

// chooseQuality plays p again where it is in quality q, which the next
// videos keep.
func (a *App) chooseQuality(p *player, q quality) {
	a.prefs.Quality = q.key
	a.savePreferences()
	a.restart(p, func(*capiv1.PlaybackOptions) {})
}

func panelHeading(c *ui.Context, title string) {
	style.Text(ui.Text(c, title), style.Type(c).Footnote).FontWeight(600).TextColor(ui.RGB(255, 255, 255).Alpha(0.6)).Padding(16, 10, 8)
}

// choice builds a choice of a list in the panel, checked or not.
func choice(c *ui.Context, name, details string, checked bool) ui.Element {
	white := ui.RGB(255, 255, 255)
	b := ui.ButtonBase(c).FillWidth().Padding(8, 10).Gap(12).Radius(10).AlignItems(ui.Center).Label(name)
	color := white
	switch {
	case tvLit(c, b):
		color = ui.RGB(0, 0, 0)
	case b.Hovered():
		b.Background(white.Alpha(0.1))
	}
	b.Children(func() {
		ui.Box(c).Size(16, 16).Children(func() {
			if checked {
				ui.Icon(c, style.Icon("check", false)).Size(16, 16).TextColor(style.Color(design.DarkColors.Accent))
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			style.Text(ui.Text(c, name), style.Type(c).Callout).TextColor(color).SingleLine()
			if details != "" {
				style.Text(ui.Text(c, details), style.Type(c).Caption).TextColor(color.Alpha(0.6)).SingleLine()
			}
		})
	})
	return b
}

// tvLit turns b white in the TV mode while it has focus, its content to
// be black (docs/tv.md §4), and reports whether it did.
func tvLit(c *ui.Context, b ui.Element) bool {
	if !style.TV(c) {
		return false
	}
	b.FocusRing(false)
	if !b.Focused() {
		return false
	}
	b.Background(ui.RGB(255, 255, 255))
	return true
}

// focusOpening gives b, the panel's current choice, focus as the panel
// opens in the TV mode (docs/tv.md §7).
func (p *player) focusOpening(b ui.Element, current bool) ui.Element {
	switch {
	case !current || p.focusChoice == 0:
	case b.Focused():
		p.focusChoice = 0
	default:
		b.Focus()
	}
	return b
}

// playerError builds why the item cannot play, with the way back.
func (a *App) playerError(c *ui.Context, p *player) {
	white := ui.RGB(255, 255, 255)
	ui.Column(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Background(ui.RGBA(0, 0, 0, 0.7)).
		AlignItems(ui.Center).Justify(ui.Center).Gap(12).Padding(24).Children(func() {
		ui.Icon(c, style.Icon("error", false)).Size(48, 48).TextColor(white.Alpha(0.7))
		style.Text(ui.Text(c, errorMessage(c, p.err)), style.Type(c).Body).TextColor(white).TextAlign(ui.Center).MaxWidth(420)
		b := ui.ButtonBase(c).Height(36).Padding(0, 16).Radius(10).Center().Background(white.Alpha(0.15)).Label(style.T(c, "player.back"))
		if b.Hovered() {
			b.Background(white.Alpha(0.25))
		}
		b.Children(func() {
			style.Text(ui.Text(c, style.T(c, "player.back")), style.Type(c).Headline).TextColor(white)
		})
		if b.Clicked() {
			a.closePlayer(p, true)
		}
	})
}

// rowMenu builds a button of the controller's row opening a menu of
// frosted glass above it, centered on it, as the Mac app's; fill builds its
// items, and close closes it. The press outside that closes it does not go
// on to the picture.
func (a *App) rowMenu(c *ui.Context, p *player, which rowMenu, icon string, fill bool, label string, items func(close func())) {
	b := playerButton(c.Key(key("menu", int(which))), icon, label, fill, p.menu == which, 44)
	if b.Clicked() {
		if p.menu == which {
			p.menu = menuNone
		} else {
			p.menu = which
		}
	}
	open := p.menu == which
	ui.PopoverBase(c, b, &open, func(panel ui.Element) {
		colors := style.Colors(c)
		shadow := design.ElevationLevel3
		panel.AttachTo(b, ui.AnchorTop, ui.AnchorBottom).Margin(0, 0, 8).Width(288).Radius(12).
			Material(glass.Blur{Radius: 64}).AlignItems(ui.Stretch).
			Shadow(shadow.X, shadow.Y, shadow.Blur, shadow.Spread, style.Color(shadow.Color))
		// The tint goes on a column of its own over the blur, as an
		// element's background takes the place of its material.
		ui.Column(c).Radius(12).Background(style.Color(colors.SurfaceOverlay).Alpha(0.45)).Border(1, style.Color(colors.Border)).
			Padding(6).AlignItems(ui.Stretch).Children(func() {
			items(func() { p.menu = menuNone })
		})
	})
	if !open && p.menu == which {
		p.menu, p.swallow = menuNone, true
		p.input = c.Now()
	}
}

// menuHeading builds a heading of a row's menu, lined up after the column
// of the checks.
func menuHeading(c *ui.Context, text string) {
	style.Text(ui.Text(c, text), style.Type(c).Caption).TextColor(style.Color(style.Colors(c).TextSecondary)).
		Padding(6, 10, 4, 32).SingleLine()
}

// menuFact builds a line of a row's menu that tells, not does, as the
// Mac's disabled items.
func menuFact(c *ui.Context, text string) {
	style.Text(ui.Text(c, text), style.Type(c).Callout).TextColor(style.Color(style.Colors(c).TextSecondary)).
		Padding(0, 10, 6, 32)
}

// menuChoice builds an item of a row's menu, checked or not, its details
// after its name, and reports whether it was chosen; no item has an icon.
func menuChoice(c *ui.Context, id, name, details string, checked, disabled bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(id)).MinHeight(28).Padding(4, 8).Gap(8).Radius(6).AlignItems(ui.Center).Label(name).Disabled(disabled)
	if b.Hovered() && !disabled {
		b.Background(style.Color(colors.Fill))
	}
	if disabled {
		b.Opacity(0.5)
	}
	b.Children(func() {
		ui.Box(c).Size(16, 16).Shrink(0).Children(func() {
			if checked {
				ui.Icon(c, style.Icon("check", false)).Size(16, 16).TextColor(style.Color(colors.AccentText))
			}
		})
		ui.Row(c).Grow(1).MinWidth(0).Gap(6).AlignItems(ui.Center).Children(func() {
			style.Text(ui.Text(c, name), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).SingleLine().Shrink(1)
			if details != "" {
				style.Text(ui.Text(c, details), style.Type(c).Footnote).TextColor(style.Color(colors.TextTertiary)).SingleLine().Shrink(1)
			}
		})
	})
	return b.Clicked()
}

func (a *App) speedMenu(c *ui.Context, p *player, close func()) {
	menuHeading(c, style.T(c, "player.speed"))
	for _, r := range speeds {
		label := strconv.FormatFloat(r, 'f', -1, 64)
		if menuChoice(c, key("speed", r), label+"×", "", r == p.state.Speed, false) {
			p.state.Speed = r
			a.setVideo(p, "speed", label)
			close()
		}
	}
}

func (a *App) subtitlesMenu(c *ui.Context, p *player, close func()) {
	current := p.playback.GetSubtitleStreamIndex()
	if !p.playback.HasSubtitleStreamIndex() {
		current = -1
	}
	menuHeading(c, style.T(c, "player.subtitles"))
	if menuChoice(c, key("sub", -1), style.T(c, "player.subtitlesOff"), "", current < 0, false) {
		close()
		if current >= 0 {
			a.restart(p, func(o *capiv1.PlaybackOptions) { o.SetSubtitleStreamIndex(-1) })
		}
	}
	for _, s := range p.playback.GetSubtitles() {
		name := cmpOr(s.GetTitle(), languageName(c, s.GetLanguage()), fmt.Sprintf("#%d", s.GetStreamIndex()))
		details := []string{strings.ToUpper(s.GetFormat())}
		if s.GetForced() {
			details = append(details, "Forced")
		}
		checked := s.GetStreamIndex() == current
		if menuChoice(c, key("sub", s.GetStreamIndex()), name, strings.Join(details, " · "), checked, false) {
			close()
			if !checked {
				index := s.GetStreamIndex()
				a.restart(p, func(o *capiv1.PlaybackOptions) { o.SetSubtitleStreamIndex(index) })
			}
		}
	}
	if current < 0 {
		return
	}
	menuSeparator(c)
	offsetItems(c, "sub", style.T(c, "player.subtitleOffset"), p.state.SubtitleDelay, func(d time.Duration) {
		p.state.SubtitleDelay = d
		a.setVideo(p, "sub-delay", strconv.FormatFloat(d.Seconds(), 'f', 3, 64))
	})
}

// offsetItems builds an offset's heading, telling it, and the items moving
// it a tenth of a second earlier or later and back, which keep the menu
// open, as the web UI's.
func offsetItems(c *ui.Context, id, title string, offset time.Duration, set func(time.Duration)) {
	menuHeading(c, fmt.Sprintf("%s: %+.1f s", title, offset.Seconds()))
	if menuChoice(c, key("offset", id, -1), style.T(c, "player.offsetEarlier"), "", false, false) {
		set((offset - 100*time.Millisecond).Round(time.Millisecond))
	}
	if menuChoice(c, key("offset", id, 1), style.T(c, "player.offsetLater"), "", false, false) {
		set((offset + 100*time.Millisecond).Round(time.Millisecond))
	}
	if menuChoice(c, key("offset", id, 0), style.T(c, "player.offsetReset"), "", false, offset == 0) {
		set(0)
	}
}

func (a *App) audioMenu(c *ui.Context, p *player, close func()) {
	menuHeading(c, style.T(c, "player.audio"))
	for _, s := range p.source().GetAudio() {
		name := cmpOr(s.GetTitle(), languageName(c, s.GetLanguage()), fmt.Sprintf("#%d", s.GetIndex()))
		checked := p.playback.HasAudioStreamIndex() && s.GetIndex() == p.playback.GetAudioStreamIndex()
		if menuChoice(c, key("audio", s.GetIndex()), name, audioDetails(s), checked, false) {
			close()
			if !checked {
				index := s.GetIndex()
				a.restart(p, func(o *capiv1.PlaybackOptions) { o.SetAudioStreamIndex(index) })
			}
		}
	}
	menuSeparator(c)
	offsetItems(c, "audio", style.T(c, "player.audioOffset"), p.state.AudioDelay, func(d time.Duration) {
		p.state.AudioDelay = d
		a.prefs.AudioDelay = d.Seconds()
		a.setVideo(p, "audio-delay", strconv.FormatFloat(d.Seconds(), 'f', 3, 64))
	})
	devices := p.state.AudioDevices
	if len(devices) < 2 {
		return
	}
	menuSeparator(c)
	menuHeading(c, style.T(c, "player.audioDevice"))
	current := cmpOr(p.state.AudioDevice, "auto")
	for _, d := range devices {
		name := d.Description
		if d.Name == "auto" {
			name = style.T(c, "player.audioDeviceSystem")
		}
		checked := d.Name == current
		if menuChoice(c, key("output", d.Name), cmpOr(name, d.Name), "", checked, false) {
			close()
			if !checked {
				a.chooseOutput(p, d.Name)
			}
		}
	}
}

func (a *App) qualityMenu(c *ui.Context, p *player, close func()) {
	menuHeading(c, style.T(c, "player.quality"))
	for _, q := range qualities {
		checked := q.key == a.quality().key
		if menuChoice(c, key("quality", q.key), qualityName(c, q), "", checked, false) {
			close()
			if !checked {
				a.chooseQuality(p, q)
			}
		}
	}
}

func (a *App) infoMenu(c *ui.Context, p *player) {
	menuHeading(c, style.T(c, "player.method"))
	menuFact(c, playMethod(c, p.playback))
	if s := p.source(); s != nil {
		menuHeading(c, style.T(c, "player.source"))
		menuFact(c, sourceFacts(s))
	}
}

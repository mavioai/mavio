package app

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/mpv"
)

// fakeVideo records what the player asks of mpv, whose state the test
// sends with emit.
type fakeVideo struct {
	changed func(mpv.State)

	mu       sync.Mutex
	loads    []string
	commands []string
	closed   bool
}

func (v *fakeVideo) View() ui.NativeVideoView { return nil }

func (v *fakeVideo) Load(url string, start time.Duration) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.loads = append(v.loads, url+"@"+start.String())
	return nil
}

func (v *fakeVideo) Command(args ...string) error {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.commands = append(v.commands, strings.Join(args, " "))
	return nil
}

func (v *fakeVideo) Set(name, value string) error { return v.Command("set", name, value) }

func (v *fakeVideo) Close() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.closed = true
}

func (v *fakeVideo) emit(s mpv.State) { v.changed(s) }

func (v *fakeVideo) has(command string) bool {
	v.mu.Lock()
	defer v.mu.Unlock()
	return slices.Contains(v.commands, command)
}

// tracks are the tracks of the fake core's sources as mpv lists them.
var tracks = []mpv.Track{
	{ID: 1, Type: "video", Index: 0, Selected: true},
	{ID: 1, Type: "audio", Index: 1, Language: "eng", Title: "English Stereo", Selected: true},
	{ID: 2, Type: "audio", Index: 2, Language: "jpn"},
	{ID: 1, Type: "sub", Index: 3, Language: "eng"},
	{ID: 2, Type: "sub", Index: 4, Language: "jpn"},
}

// requests returns the fake core's requests of a kind.
func (f *fakeCore) requestsOf(has func(*capiv1.Request) bool) []*capiv1.Request {
	f.mu.Lock()
	defer f.mu.Unlock()
	var rs []*capiv1.Request
	for _, r := range f.requests {
		if has(r) {
			rs = append(rs, r)
		}
	}
	return rs
}

func TestPlayer(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	server := h.core.s.Server.ID
	next := capiv1.Card_builder{ServerId: server, ItemId: "atlas-s2e4", Kind: capiv1.CardKind_CARD_KIND_EPISODE, Title: "Next"}.Build()
	start := int64(12 * time.Minute / time.Millisecond)
	h.app.play(server, "atlas-s2e3", capiv1.PlaybackOptions_builder{StartPositionMs: &start}.Build(), next)
	h.settle()
	v := h.video
	if got, want := v.loads, []string{"http://mavio.test/videos/atlas-s2e3@12m0s"}; !slices.Equal(got, want) {
		t.Fatalf("loads = %q, want = %q", got, want)
	}
	plays := h.core.requestsOf((*capiv1.Request).HasPlay)
	if len(plays) != 1 || plays[0].GetPlay().GetOptions().GetMaxBitrate() != 40_000_000 || len(plays[0].GetPlay().GetCapabilities()) == 0 {
		t.Fatalf("plays = %v, want = one with the auto quality and capabilities", plays)
	}

	playing := mpv.State{Loaded: true, Position: 12 * time.Minute, Duration: 42 * time.Minute, Speed: 1, Tracks: tracks}
	v.emit(playing)
	h.settle()
	for _, want := range []string{"set aid 1", "set sid 1"} {
		if !v.has(want) {
			t.Errorf("commands = %q, want %q", v.commands, want)
		}
	}
	if !h.tt.HasText("Atlas Station · S2:E3") {
		t.Errorf("texts = %q, want the episode's series", h.tt.Texts())
	}
	updates := h.core.requestsOf((*capiv1.Request).HasUpdatePlayback)
	if len(updates) != 1 || updates[0].GetUpdatePlayback().GetPositionMs() != start {
		t.Errorf("updates = %v, want = one at %d", updates, start)
	}
	h.save(shots, "player")

	h.tt.Key(0, ui.KeySpace)
	h.settle()
	if !v.has("set pause yes") {
		t.Errorf("commands = %q, want the pause", v.commands)
	}
	h.tt.Key(0, ui.KeyRight)
	h.settle()
	if !v.has("seek 730.000 absolute+exact") {
		t.Errorf("commands = %q, want a seek 10 s later", v.commands)
	}

	// The row's buttons open their menus above them, as the Mac app's,
	// with no buttons seeking back and forward.
	for _, label := range []string{"Back 10 seconds", "Forward 10 seconds"} {
		if _, ok := h.tt.Find(label); ok {
			t.Errorf("the controller shows %q", label)
		}
	}
	for _, menu := range []struct {
		button, name, text string
	}{
		{"Speed", "player-speed", "1.75×"},
		{"Audio", "player-audio", "English Stereo"},
		{"Subtitles", "player-subtitles", "Off"},
		{"Info", "player-info", "Playback"},
		{"Quality", "player-quality", "720p"},
	} {
		if err := h.tt.Click(menu.button); err != nil {
			t.Fatal(err)
		}
		h.settle()
		if !h.tt.HasText(menu.text) {
			t.Errorf("%s texts = %q, want %q", menu.name, h.tt.Texts(), menu.text)
		}
		h.save(shots, menu.name)
		if menu.button != "Quality" {
			h.tt.Key(0, ui.KeyEscape)
			h.settle()
			if p := h.app.playerOf(nil); p == nil || p.menu != menuNone {
				t.Fatalf("Escape did not close the %s menu alone", menu.button)
			}
		}
	}
	// A quality restarts the playback where it is.
	if err := h.tt.Click("720p"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	plays = h.core.requestsOf((*capiv1.Request).HasPlay)
	if len(plays) != 2 {
		t.Fatalf("plays = %d, want = 2", len(plays))
	}
	if o := plays[1].GetPlay().GetOptions(); o.GetMaxBitrate() != 4_000_000 || o.GetStartPositionMs() != (12*time.Minute+10*time.Second).Milliseconds() {
		t.Errorf("options = %v, want = 720p from 12:10", o)
	}
	if stops := h.core.requestsOf((*capiv1.Request).HasStopPlayback); len(stops) != 1 || !stops[0].GetStopPlayback().HasPositionMs() {
		t.Errorf("stops = %v, want = one at a position", stops)
	}

	// The end plays the next episode.
	v.emit(mpv.State{Ended: true, Position: 42 * time.Minute, Duration: 42 * time.Minute, Tracks: tracks})
	h.settle()
	stops := h.core.requestsOf((*capiv1.Request).HasStopPlayback)
	if len(stops) != 2 || stops[1].GetStopPlayback().HasPositionMs() {
		t.Errorf("stops = %v, want a second one played to the end", stops)
	}
	if !v.closed {
		t.Error("the ended video did not close")
	}
	plays = h.core.requestsOf((*capiv1.Request).HasPlay)
	if got := plays[len(plays)-1].GetPlay().GetItemId(); got != "atlas-s2e4" {
		t.Errorf("played = %s, want = atlas-s2e4", got)
	}

	// Escape leaves the player.
	h.tt.Key(0, ui.KeyEscape)
	h.settle()
	if h.app.playerOf(nil) != nil {
		t.Error("Escape did not close the player")
	}
}

func TestTVPlayer(t *testing.T) {
	h := newHarness(t, true, 1920, 1080)
	h.app.setTV(true)
	server := h.core.s.Server.ID
	next := capiv1.Card_builder{ServerId: server, ItemId: "atlas-s2e4", Kind: capiv1.CardKind_CARD_KIND_EPISODE, Title: "Next"}.Build()
	start := int64(12 * time.Minute / time.Millisecond)
	h.app.play(server, "atlas-s2e3", capiv1.PlaybackOptions_builder{StartPositionMs: &start}.Build(), next)
	h.settle()
	v := h.video
	playing := mpv.State{Loaded: true, Position: 12 * time.Minute, Duration: 42 * time.Minute, Speed: 1, Tracks: tracks}
	v.emit(playing)
	h.settle()
	key := func(k ui.Key) {
		t.Helper()
		h.tt.Key(0, k)
		h.settle()
	}
	focused := func(label, after string) {
		t.Helper()
		if !h.tt.Focused(label) {
			t.Fatalf("after %s: %q has no focus; texts = %q", after, label, h.tt.Texts())
		}
	}
	shown := func() bool {
		_, ok := h.tt.Find("Next Episode")
		return ok
	}
	// The controller has no way back, nor buttons to play, pause or seek.
	for _, label := range []string{"Back", "Pause", "Full Screen"} {
		if _, ok := h.tt.Find(label); ok {
			t.Errorf("the TV mode's controller shows %q", label)
		}
	}
	h.save(shots, "tv-player")

	// Down moves focus into the row, along which left and right move.
	key(ui.KeyDown)
	focused("Next Episode", "down")
	key(ui.KeyRight)
	focused("Subtitles", "right from Next Episode")
	key(ui.KeyRight)
	focused("Audio", "right from Subtitles")
	key(ui.KeyRight)
	focused("Assistant", "right from Audio")
	key(ui.KeyRight)
	focused("Assistant", "right from the row's last")
	key(ui.KeyLeft)
	focused("Audio", "left from Assistant")
	// The panel opens with focus on its current choice, and closing it
	// returns focus to its button.
	key(ui.KeyEnter)
	if p := h.app.playerOf(nil); p.panel != tabAudio {
		t.Fatalf("panel = %v, want = audio", p.panel)
	}
	focused("English Stereo", "opening the panel")
	h.save(shots, "tv-player-audio")
	// Focus keeps within the panel.
	key(ui.KeyDown)
	focused("Japanese", "down in the panel")
	key(ui.KeyDown)
	key(ui.KeyDown)
	if h.tt.Focused("Audio") || h.tt.Focused("Next Episode") {
		t.Fatal("focus left the panel")
	}
	key(ui.KeyEscape)
	if p := h.app.playerOf(nil); p == nil || p.panel != tabNone {
		t.Fatal("back did not close the panel alone")
	}
	focused("Audio", "closing the panel")

	// Up leaves the row; left and right seek, and select pauses, keeping
	// the controller up, and plays, hiding it.
	key(ui.KeyUp)
	if h.tt.Focused("Audio") {
		t.Fatal("up did not leave the row")
	}
	key(ui.KeyLeft)
	if !v.has("seek 710.000 absolute+exact") {
		t.Errorf("commands = %q, want a seek 10 s earlier", v.commands)
	}
	key(ui.KeyEnter)
	if !v.has("set pause yes") {
		t.Errorf("commands = %q, want the pause", v.commands)
	}
	paused := playing
	paused.Paused = true
	v.emit(paused)
	h.settle()
	if !shown() {
		t.Error("the controller hid while paused")
	}
	key(ui.KeyEnter)
	if !v.has("set pause no") {
		t.Errorf("commands = %q, want the play", v.commands)
	}
	v.emit(playing)
	h.settle()
	if shown() {
		t.Error("playing on did not hide the controller")
	}

	// Back hides the controller, then leaves the player.
	key(ui.KeyDown)
	if !shown() {
		t.Fatal("down did not show the controller")
	}
	key(ui.KeyEscape)
	if shown() || h.app.playerOf(nil) == nil {
		t.Fatal("back did not hide the controller alone")
	}
	key(ui.KeyEscape)
	if h.app.playerOf(nil) != nil {
		t.Error("back did not leave the player")
	}
}

func TestPlayerControls(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	server := h.core.s.Server.ID
	h.app.play(server, "atlas-s2e3", nil)
	h.settle()
	v := h.video
	v.emit(mpv.State{Loaded: true, Position: 12 * time.Minute, Duration: 42 * time.Minute, Speed: 1, Volume: 100, Tracks: tracks})
	h.settle()
	p := h.app.playerOf(nil)

	// The last of a season is followed by the next season's first.
	if len(p.next) != 1 || p.next[0].GetItemId() != "atlas-s3e1" {
		t.Errorf("next = %v, want = atlas-s3e1", p.next)
	}
	if _, ok := h.tt.Find("Next Episode"); !ok {
		t.Errorf("no Next Episode button; texts = %q", h.tt.Texts())
	}

	// Over the timeline: the chapter and the time under the pointer.
	r, ok := h.tt.Find("Position")
	if !ok {
		t.Fatalf("no timeline; texts = %q", h.tt.Texts())
	}
	h.tt.Move(r.X+8+(r.W-16)*0.5, r.Y+r.H/2)
	h.settle()
	for _, want := range []string{"The Station", "21:00"} {
		if !h.tt.HasText(want) {
			t.Errorf("texts over the timeline = %q, want %q", h.tt.Texts(), want)
		}
	}
	h.save(shots, "player-scrub")

	// The keys set the volume and the speed, and tell them; the settling
	// outlasts the notices.
	h.tt.Key(0, ui.KeyDown)
	h.settle()
	if !v.has("set volume 95") || p.notice.text != "95%" {
		t.Errorf("commands = %q, texts = %q, want the volume at 95", v.commands, h.tt.Texts())
	}
	h.tt.Key(0, ui.KeyBracketRight)
	h.settle()
	if !v.has("set speed 1.25") || p.notice.text != "1.25×" {
		t.Errorf("commands = %q, texts = %q, want the speed at 1.25", v.commands, h.tt.Texts())
	}

	// Across, the wheel seeks over the picture, 5 seconds a line; up and
	// down, a moment later, it sets the volume.
	h.tt.Scroll(640, 300, -40, 0)
	h.settle()
	if !v.has("seek 725.000 absolute+exact") {
		t.Errorf("commands = %q, want a seek 5 s later", v.commands)
	}
	h.tt.Advance(time.Second)
	h.tt.Scroll(640, 300, 0, -40)
	h.settle()
	if !v.has("set volume 90") {
		t.Errorf("commands = %q, want the volume at 90", v.commands)
	}
	if got := *h.app.prefs.Volume; got != 90 {
		t.Errorf("volume kept = %d, want = 90", got)
	}

	// The pointer hides with the controller.
	h.tt.Advance(controlsTimeout + time.Second)
	h.settle()
	if got := h.tt.Cursor(); got != ui.CursorNone {
		t.Errorf("cursor = %v, want none with the controller hidden", got)
	}

	// The audio's menu offers the offset, which keeps it open, and the
	// outputs.
	p.state.AudioDevices = []mpv.AudioDevice{{Name: "auto", Description: "Autoselect device"}, {Name: "coreaudio/speakers", Description: "MacBook Speakers"}}
	h.tt.Move(600, 320)
	h.settle()
	if err := h.tt.Click("Audio"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	for _, want := range []string{"Audio Offset: +0.0 s", "System Default", "MacBook Speakers"} {
		if !h.tt.HasText(want) {
			t.Errorf("texts = %q, want %q", h.tt.Texts(), want)
		}
	}
	if err := h.tt.Click("Later"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !v.has("set audio-delay 0.100") || p.menu != menuAudio {
		t.Errorf("commands = %q, menu = %v, want the sound 0.1 s later and the menu open", v.commands, p.menu)
	}
	if err := h.tt.Click("MacBook Speakers"); err != nil {
		t.Fatal(err)
	}
	h.settle()
	if !v.has("set audio-device coreaudio/speakers") || h.app.prefs.AudioDevice != "coreaudio/speakers" || p.menu != menuNone {
		t.Errorf("commands = %q, want the speakers chosen and the menu closed", v.commands)
	}

	// The subtitles stand above the controller while it shows.
	if p.subPos >= 100 || !v.has(fmt.Sprintf("set sub-pos %d", p.subPos)) {
		t.Errorf("sub-pos = %d, commands = %q, want the subtitles lifted", p.subPos, v.commands)
	}

	// S saves a screenshot.
	h.tt.Key(0, ui.KeyS)
	h.settle()
	if !slices.ContainsFunc(v.commands, func(c string) bool {
		return strings.HasPrefix(c, "screenshot-to-file ") && strings.Contains(c, "Pictures") && strings.HasSuffix(c, ".png video")
	}) {
		t.Errorf("commands = %q, want a screenshot", v.commands)
	}
}

func TestPlayerSeek(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	h.app.play(h.core.s.Server.ID, "atlas-s2e3", nil)
	h.settle()
	v := h.video
	playing := mpv.State{Loaded: true, Position: 12 * time.Minute, Duration: 42 * time.Minute, Speed: 1, Volume: 100, Tracks: tracks, Restarts: 1}
	v.emit(playing)
	h.settle()
	p := h.app.playerOf(nil)

	// A seek shows its time at once, keeps it while mpv still reports the
	// old one, and hides the old frame after a beat until mpv starts again.
	h.tt.Key(0, ui.KeyRight)
	h.tt.Frame()
	if !p.seek.pending || p.state.Position != 12*time.Minute+10*time.Second {
		t.Fatalf("seek = %+v, position = %v, want a pending seek to 12:10", p.seek, p.state.Position)
	}
	v.emit(playing)
	h.settle()
	if p.state.Position != 12*time.Minute+10*time.Second {
		t.Errorf("position = %v, want = 12:10 while seeking", p.state.Position)
	}
	landed := playing
	landed.Position, landed.Restarts = 12*time.Minute+10*time.Second, 2
	v.emit(landed)
	h.settle()
	if p.seek.pending || p.state.Position != landed.Position {
		t.Errorf("seek = %+v, want it over once mpv started again", p.seek)
	}

	// A failed video loads once more where it was, decoded in software;
	// failing again, it tells why.
	failed := landed
	failed.Loaded, failed.Err = false, errors.New("decoding failed")
	v.emit(failed)
	h.settle()
	if !v.has("set hwdec no") || len(v.loads) != 2 || !strings.HasSuffix(v.loads[1], "@12m10s") || p.err != nil {
		t.Errorf("loads = %q, commands = %q, err = %v, want the video loaded again at 12:10", v.loads, v.commands, p.err)
	}
	v.emit(landed)
	h.settle()
	v.emit(failed)
	h.settle()
	if p.err == nil || len(v.loads) != 2 {
		t.Errorf("loads = %q, err = %v, want the error told", v.loads, p.err)
	}
}

func TestPlayerScrub(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	h.app.play(h.core.s.Server.ID, "atlas-s2e3", nil)
	h.settle()
	v := h.video
	v.emit(mpv.State{Loaded: true, Position: 12 * time.Minute, Duration: 42 * time.Minute, Speed: 1, Volume: 100, Tracks: tracks, Restarts: 1})
	h.settle()
	r, ok := h.tt.Find("Position")
	if !ok {
		t.Fatalf("no timeline; texts = %q", h.tt.Texts())
	}
	y := r.Y + r.H/2
	x := func(f float32) float32 { return r.X + 8 + (r.W-16)*f }
	last := func() string {
		var seek string
		for _, c := range v.commands {
			if strings.HasPrefix(c, "seek ") {
				seek = c
			}
		}
		v.commands = nil
		return seek
	}

	// A click seeks exactly where it is.
	h.tt.ClickAt(x(0.25), y)
	h.settle()
	if got, want := last(), "seek 630.000 absolute+exact"; got != want {
		t.Errorf("click: seek = %q, want = %q", got, want)
	}

	// A drag seeks to keyframes as it goes, and exactly where it lets go.
	h.tt.Move(x(0.5), y)
	h.tt.Press(x(0.5), y)
	h.tt.Frame()
	h.tt.Move(x(0.6), y)
	h.tt.Frame()
	if got, want := last(), "seek 1512.000 absolute+keyframes"; got != want {
		t.Errorf("dragging: seek = %q, want = %q", got, want)
	}
	h.tt.Move(x(0.75), y)
	h.tt.Frame()
	h.tt.Release(x(0.75), y)
	h.settle()
	if got, want := last(), "seek 1890.000 absolute+exact"; got != want {
		t.Errorf("let go: seek = %q, want = %q", got, want)
	}
}

// The player loads, its activity indicator shown, until the first frame.
func TestPlayerLoading(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	h.app.play(h.core.s.Server.ID, "atlas-s2e3", nil)
	h.settle()
	p := h.app.playerOf(nil)
	if !p.loading {
		t.Fatal("loading = false after the load, want = true")
	}
	h.video.emit(mpv.State{Loaded: true, Duration: 42 * time.Minute, Speed: 1, Tracks: tracks})
	h.settle()
	if !p.loading {
		t.Error("loading = false once loaded, want = true until the first frame")
	}
	h.video.emit(mpv.State{Loaded: true, Duration: 42 * time.Minute, Speed: 1, Tracks: tracks, Restarts: 1})
	h.settle()
	if p.loading {
		t.Error("loading = true after the first frame, want = false")
	}
}

func TestFitWindow(t *testing.T) {
	a := &App{}
	for _, tt := range []struct {
		name          string
		size, picture [2]int
		want, at      [2]int
	}{
		{"wider", [2]int{1280, 720}, [2]int{1920, 800}, [2]int{1487, 620}, [2]int{-3, 150}},
		{"same", [2]int{1280, 720}, [2]int{1920, 1080}, [2]int{1280, 720}, [2]int{100, 100}},
		{"least", [2]int{480, 270}, [2]int{1080, 1920}, [2]int{480, 853}, [2]int{100, -191}},
	} {
		w := &fakeWindow{size: tt.size, at: [2]int{100, 100}}
		p := &player{win: w, state: mpv.State{Width: int64(tt.picture[0]), Height: int64(tt.picture[1])}}
		a.fitWindow(p)
		if w.size != tt.want || w.at != tt.at {
			t.Errorf("%s: size = %v at %v, want = %v at %v", tt.name, w.size, w.at, tt.want, tt.at)
		}
	}
}

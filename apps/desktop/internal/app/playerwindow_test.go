package app

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// fakeWindow is a window of the player's that only records what it is
// asked.
type fakeWindow struct {
	closed     func()
	isClosed   bool
	title      string
	fullScreen bool
	// size is the content's, and at where the frame is.
	size, at [2]int
}

func (w *fakeWindow) Close() {
	if !w.isClosed {
		w.isClosed = true
		w.closed()
	}
}
func (w *fakeWindow) Focus()                {}
func (w *fakeWindow) SetTitle(title string) { w.title = title }
func (w *fakeWindow) ToggleFullScreen()     { w.fullScreen = !w.fullScreen }
func (w *fakeWindow) IsFullScreen() bool    { return w.fullScreen }
func (w *fakeWindow) ContentSize() (int, int) {
	return w.size[0], w.size[1]
}
func (w *fakeWindow) SetContentSize(width, height int) { w.size = [2]int{width, height} }
func (w *fakeWindow) Position() (int, int)             { return w.at[0], w.at[1] }
func (w *fakeWindow) SetPosition(x, y int)             { w.at = [2]int{x, y} }

func TestPlayerWindows(t *testing.T) {
	h := newHarness(t, true, 1280, 800)
	server := h.core.s.Server.ID
	var windows []*fakeWindow
	h.app.opts.NewWindow = func(_ func(*ui.Context), closed func()) Window {
		w := &fakeWindow{closed: closed}
		windows = append(windows, w)
		return w
	}
	open := func() (n int) {
		for _, w := range windows {
			if !w.isClosed {
				n++
			}
		}
		return n
	}

	// One at a time by default, in one window of the player's.
	h.app.play(server, "orbit", nil)
	h.app.play(server, "harbor", nil)
	h.settle()
	if got := len(h.app.players); got != 1 || h.app.players[0].item != "harbor" {
		t.Fatalf("players = %d, want = harbor alone", got)
	}
	if got := len(windows); got != 1 {
		t.Errorf("windows opened = %d, want = 1", got)
	}
	if h.app.playerOf(nil) != nil {
		t.Errorf("a player shows in the app's window")
	}

	// A window for each video, as the preferences ask.
	h.app.prefs.PlayerWindows = true
	h.app.play(server, "orbit", nil)
	h.settle()
	if got, want := len(h.app.players), 2; got != want {
		t.Fatalf("players = %d, want = %d", got, want)
	}
	if got, want := open(), 2; got != want {
		t.Errorf("windows open = %d, want = %d", got, want)
	}

	// Closing a window ends its player alone.
	windows[0].Close()
	h.settle()
	if got := len(h.app.players); got != 1 || h.app.players[0].item != "orbit" {
		t.Errorf("after closing harbor's window: players = %d, want = orbit alone", got)
	}
	// Escape closes the player and its window.
	h.app.closePlayer(h.app.players[0], true)
	if open() != 0 || len(h.app.players) != 0 {
		t.Errorf("windows open = %d, players = %d, want = none", open(), len(h.app.players))
	}
}

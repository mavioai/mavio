package app

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// places are the sidebar's places (docs/clients.md §4.1), each keeping
// the pages opened from it, as the web UI's and the Apple TV app's do:
// choosing another place and coming back returns to the page last shown
// there, scrolled where it was left; choosing the place shown returns to
// its top. Back steps back through the place's pages alone.
type places struct {
	// stacks are each place's pages, from the place to the page last
	// shown, by the place's path.
	stacks map[string][]string
	// current is the place shown, shown the page seen last, and previous
	// the page before it in the router's history, if it is known.
	current, shown, previous string
	// replacing tells that the next page takes the place of the one shown.
	replacing bool
}

// isPlace reports whether path is a place's: a page the sidebar opens.
func isPlace(path string) bool {
	switch path {
	case "/home", "/search", "/discover":
		return true
	}
	parts := strings.Split(strings.Trim(path, "/"), "/")
	return len(parts) == 3 && parts[0] == "library"
}

// visitPlaces follows the router to the page shown, adding it to its
// place's pages.
func (a *App) visitPlaces() {
	p := &a.places
	path := a.router.Path()
	if path == p.shown {
		return
	}
	replacing := p.replacing
	p.replacing = false
	p.previous, p.shown = p.shown, path
	if replacing {
		p.previous = ""
	}
	defer a.prunePageScrolls()
	if p.stacks == nil {
		p.stacks = map[string][]string{}
	}
	if isPlace(path) {
		p.current = path
		p.stacks[path] = []string{path}
		return
	}
	stack, ok := p.stacks[p.current]
	if !ok {
		// A page opened directly is Home's.
		p.current = "/home"
		p.stacks[p.current] = []string{"/home", path}
		return
	}
	// A page among the place's ends them, as Back returns to it.
	switch i := slices.Index(stack, path); {
	case i >= 0:
		stack = stack[:i+1]
	case replacing:
		stack[len(stack)-1] = path
	default:
		stack = append(stack, path)
	}
	p.stacks[p.current] = stack
}

// openPlace opens the place of path: another place at the page last shown
// there, the place shown at its top.
func (a *App) openPlace(path string) {
	p := &a.places
	stack := p.stacks[path]
	if len(stack) == 0 {
		a.router.Push(path)
		return
	}
	page := stack[len(stack)-1]
	if p.current == path {
		page = stack[0]
	}
	if page == a.router.Path() {
		return
	}
	p.current = path
	a.router.Push(page)
}

// replace shows path in place of the page shown, in the router's history
// and in its place's pages.
func (a *App) replace(path string) {
	a.places.replacing = true
	a.router.Replace(path)
}

// goBack goes back from a page of a place to the page it was opened from,
// else to the place.
func (a *App) goBack() {
	p := &a.places
	stack := p.stacks[p.current]
	i := slices.Index(stack, a.router.Path())
	switch {
	case i > 0 && stack[i-1] == p.previous && a.router.CanGoBack():
		// The entry before this one in the history is the page's own.
		a.router.Back()
	case i > 0:
		a.router.Push(stack[i-1])
	case p.current != "":
		a.router.Push(p.current)
	default:
		a.router.Push("/home")
	}
}

// pageScroll returns the scroll state of the page at path, which keeps
// where the page was scrolled while it is among its place's pages.
func (a *App) pageScroll(path string) *ui.ScrollState {
	return a.shelfScroll("page:" + path)
}

// prunePageScrolls forgets where the pages no place keeps were scrolled.
func (a *App) prunePageScrolls() {
	for k := range a.scrolls {
		path, ok := strings.CutPrefix(k, "page:")
		if !ok {
			continue
		}
		kept := false
		for _, stack := range a.places.stacks {
			kept = kept || slices.Contains(stack, path)
		}
		if !kept {
			delete(a.scrolls, k)
		}
	}
}

// backRow is the height a page opened from another leaves at its top for
// Back, where it has no hero for Back to float over.
const backRow = 60

// pageTop returns the room a page leaves at its top: Back's row on a
// page opened from another, else the window's controls'.
func (a *App) pageTop(c *ui.Context) float32 {
	if !isPlace(a.router.Path()) {
		return backRow
	}
	return max(c.TitleBar().Height, 12)
}

// backButton builds Back over a page opened from another: a round button
// of glass 12 points from the content's top and left edges.
func (a *App) backButton(c *ui.Context) {
	if isPlace(a.router.Path()) {
		return
	}
	colors := style.Colors(c)
	glassColor, shadow := ui.RGBA(255, 255, 255, 0.35), ui.RGBA(0, 0, 0, 0.12)
	if c.Theme().Dark {
		glassColor, shadow = ui.RGBA(37, 37, 40, 0.38), ui.RGBA(0, 0, 0, 0.45)
	}
	label := style.T(c, "action.back")
	b := ui.ButtonBase(c).Absolute().Top(12).Left(12).Size(36, 36).Radius(design.RadiusFull).Center().
		Material(glass.Blur{Radius: 12}).Background(glassColor).Shadow(0, 10, 30, 0, shadow).Label(label).Tooltip(label)
	if b.Hovered() {
		b.Background(ui.RGBA(128, 128, 128, 0.1).Over(glassColor))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon("back", false)).Size(18, 18).TextColor(style.Color(colors.TextPrimary))
	})
	if b.Clicked() {
		a.goBack()
	}
}

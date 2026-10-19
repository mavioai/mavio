package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/images"
	"github.com/mavioai/mavio/apps/desktop/internal/mpv"
	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// Core is the shared core, as capi.Client calls it.
type Core interface {
	// Call sends a request and returns its answer.
	Call(ctx context.Context, request *capiv1.Request) (*capiv1.Message, error)
	// Stream is Call for a request answered in parts, given to part in
	// order.
	Stream(ctx context.Context, request *capiv1.Request, part func(*capiv1.Message)) (*capiv1.Message, error)
}

// Appearance is the user's choice of the light or dark theme.
type Appearance string

// The appearances.
const (
	AppearanceSystem Appearance = "system"
	AppearanceLight  Appearance = "light"
	AppearanceDark   Appearance = "dark"
)

// Options are what an App runs with.
type Options struct {
	Core Core
	// Images loads artwork.
	Images *images.Loader
	// Update runs a function on the main thread and draws the window
	// again; it is safe from any goroutine.
	Update func(func())
	// Preferences is the file of the user's preferences of this device.
	Preferences string
	// SetAppearance applies the appearance to the windows.
	SetAppearance func(Appearance)
	// SetLocale shows the app in a language, a BCP 47 tag, or in the
	// system's when empty.
	SetLocale func(string)
	// OpenURL opens a URL in the browser.
	OpenURL func(string) error
	// ChooseFile asks the user for a file of a kind, given by its name
	// and extensions, and returns its path, empty when none was chosen;
	// it blocks until the user answers. Nil chooses none.
	ChooseFile func(kind string, extensions []string) (string, error)
	// MavioDev is the API of the mavio.dev Mavio accounts sign in at.
	MavioDev string
	// NewWindow opens a window of the player's, which view builds, and
	// calls closed once it closes; nil plays in the app's window.
	NewWindow func(view func(*ui.Context), closed func()) Window
	// NewVideo makes a video to play in, in win, the app's window when
	// nil, which tells of its changes from its own goroutine; nil plays
	// nothing.
	NewVideo func(win Window, changed func(mpv.State)) (Video, error)
	// KeepAwake keeps the display awake, for a reason the system shows,
	// until release is called.
	KeepAwake func(reason string) (release func())
	// ToggleFullScreen enters or leaves full screen; IsFullScreen tells
	// which.
	ToggleFullScreen func()
	IsFullScreen     func() bool
	// NextWindow brings the app's next window to the front, the one
	// before when back.
	NextWindow func(back bool)
}

// phase is where the app is: starting, signing in, or showing the
// servers.
type phase int

const (
	phaseStarting phase = iota
	phaseSignIn
	phaseReady
)

// preferences are the user's choices on this device.
type preferences struct {
	Appearance       Appearance `json:"appearance,omitempty"`
	SidebarCollapsed bool       `json:"sidebarCollapsed,omitempty"`
	// SidebarWidth is the width the sidebar's edge was dragged to, shared
	// by the app's sidebar and the settings'; the layout's when zero.
	SidebarWidth float32 `json:"sidebarWidth,omitempty"`
	// TV is the TV mode (docs/tv.md).
	TV bool `json:"tv,omitempty"`
	// Quality is the key of the quality played, of qualities.
	Quality string `json:"quality,omitempty"`
	// NoAutoplayNext stops playing the next episode as one ends.
	NoAutoplayNext bool `json:"noAutoplayNext,omitempty"`
	// PlayerWindows opens a player's window for each video played, else
	// they play one at a time in one.
	PlayerWindows bool `json:"playerWindows,omitempty"`
	// Language is the language the app shows, a tag of design.Locales; the
	// system's when empty.
	Language string `json:"language,omitempty"`
}

// App is the app's state, owned by the main thread.
type App struct {
	ctx    context.Context
	opts   Options
	prefs  preferences
	router *ui.Router

	phase phase
	// players are what play: in windows of their own, or one in the
	// app's window.
	players []*player
	// account is the Mavio account signed in, if any.
	account *capiv1.Account
	// userMenuOpen tells whether the user menu is open.
	userMenuOpen bool
	// tvMenuOpen is the key of the TV mode's menu open other than the
	// user menu, "" for none (tvChoiceMenu).
	tvMenuOpen string
	servers    []*capiv1.ServerRecord
	// state is the state of each server's stream.
	state map[string]capiv1.StreamState_State
	// scans are how far the libraries' scans have come, by server and
	// library, as the servers tell administrators.
	scans map[string]map[string]*capiv1.ScanProgress
	// generation counts the changes of the servers' content, so that what
	// shows them loads again.
	generation int
	libraries  map[string]*task[*capiv1.Libraries]
	// sessionsChanged counts the changes of the user's devices, so that
	// the remote control shows them again.
	sessionsChanged int
	// itemServer, while set, is the server the downloads' rows act on in
	// place of the one chosen in the settings: an item's details'.
	itemServer string
	// issues are the issues reported with items, and their dialogs.
	issues issuesState
	// switching switches the standalone server's session to another user.
	switching switchState
	assistant assistantState

	places places
	// focus moves focus with the arrow keys in the TV mode.
	focus focusEngine
	// tvKeys is the TV mode's keyboard for fields.
	tvKeys tvKeyboard
	// pagePath is the path of the page being built.
	pagePath    string
	sidebarOpen bool // the sidebar shown over the content in medium width
	// sidebarResizing tells that the sidebar's edge is being dragged, to
	// keep the width it ends at.
	sidebarResizing bool
	// contentWidth is the width of the page beside the sidebar.
	contentWidth    float32
	signIn          signIn
	homeState       homeState
	searchState     searchState
	scrolls         map[string]*ui.ScrollState
	grids           map[string]*grid
	libraryStates   map[string]*libraryState
	detailStates    map[string]*detailsState
	personStates    map[string]*personState
	creditStates    map[string]*creditsState
	settingsState   settingsState
	discoverState   discoverState
	discoveredState discoveredState
	// bases are the servers' base URLs, for the Connect clients' responses.
	bases bases
}

// New returns the App of opts, which runs until ctx ends.
func New(ctx context.Context, opts Options) *App {
	a := &App{
		ctx:           ctx,
		opts:          opts,
		router:        ui.NewRouter("/home"),
		state:         map[string]capiv1.StreamState_State{},
		libraries:     map[string]*task[*capiv1.Libraries]{},
		scrolls:       map[string]*ui.ScrollState{},
		grids:         map[string]*grid{},
		libraryStates: map[string]*libraryState{},
		detailStates:  map[string]*detailsState{},
		personStates:  map[string]*personState{},
		creditStates:  map[string]*creditsState{},
	}
	a.loadPreferences()
	return a
}

// Start lists the Mavio account and the servers signed in to, showing
// them or the sign-in.
func (a *App) Start() {
	go func() {
		acc, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{GetAccount: &capiv1.GetAccount{}}.Build())
		if err != nil {
			slog.ErrorContext(a.ctx, "get the Mavio account", "error", err)
		}
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{ListServers: &capiv1.ListServers{}}.Build())
		a.opts.Update(func() {
			if err != nil {
				slog.ErrorContext(a.ctx, "list the servers", "error", err)
			}
			// Without a Mavio account, the core answers with an empty one.
			if acc.GetAccount().GetId() != "" {
				a.account = acc.GetAccount()
			}
			a.setServers(m.GetServers().GetServers())
		})
		if _, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{Start: &capiv1.Start{}}.Build()); err != nil {
			slog.ErrorContext(a.ctx, "start the event streams", "error", err)
		}
	}()
}

// Resume follows the servers again as the app comes to the front: their
// event streams, and the Mavio account's servers, among them those it could
// not reach before.
func (a *App) Resume() {
	go func() {
		if _, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{Start: &capiv1.Start{}}.Build()); err != nil {
			slog.ErrorContext(a.ctx, "follow the servers", "error", err)
		}
	}()
}

// setServers shows servers, or the sign-in when there are none and no
// Mavio account brings any.
func (a *App) setServers(servers []*capiv1.ServerRecord) {
	a.servers = servers
	if len(servers) == 0 && a.account == nil {
		a.phase = phaseSignIn
		a.signIn = signIn{}
		return
	}
	a.phase = phaseReady
	a.generation++
}

// Event takes an event of the core, from any goroutine.
func (a *App) Event(e *capiv1.Event) {
	if e.HasTokenChanged() {
		return
	}
	if e.HasServersChanged() {
		a.listServers()
		return
	}
	a.opts.Update(func() {
		id := e.GetServerId()
		switch {
		case e.HasState():
			a.state[id] = e.GetState().GetState()
		case e.HasConnected(), e.HasLibraryChanged(), e.HasUserDataChanged():
			a.generation++
		case e.HasSessionsChanged():
			a.sessionsChanged++
		case e.HasScanProgressChanged():
			a.scanChanged(id, e.GetScanProgressChanged())
		}
	})
}

// listServers lists the servers again, as the Mavio account's change.
func (a *App) listServers() {
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{ListServers: &capiv1.ListServers{}}.Build())
		if err != nil {
			slog.ErrorContext(a.ctx, "list the servers", "error", err)
			return
		}
		a.opts.Update(func() {
			if a.phase == phaseReady {
				a.setServers(m.GetServers().GetServers())
			}
		})
	}()
}

// signOut signs out of the Mavio account, or of every server.
func (a *App) signOut() {
	go func() {
		_, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{SignOut: &capiv1.SignOut{}}.Build())
		if err != nil {
			slog.ErrorContext(a.ctx, "sign out", "error", err)
			return
		}
		a.opts.Update(func() {
			a.account = nil
			for _, p := range slices.Clone(a.players) {
				a.closePlayer(p, true)
			}
			clear(a.libraries)
			a.setServers(nil)
			a.replace("/home")
		})
	}()
}

// windowKeys moves between the app's windows with ⌘` and ⇧⌘` on the Mac.
func (a *App) windowKeys(c *ui.Context) {
	if goruntime.GOOS != "darwin" || a.opts.NextWindow == nil {
		return
	}
	switch {
	case c.Shortcut(ui.Cmd, ui.KeyBackquote):
		a.opts.NextWindow(false)
	case c.Shortcut(ui.Cmd|ui.Shift, ui.KeyBackquote):
		a.opts.NextWindow(true)
	}
}

// tvShortcut reports whether the TV mode's shortcut was pressed: ⌃⌘T on
// the Mac, Shift+F11 elsewhere.
func tvShortcut(c *ui.Context) bool {
	if goruntime.GOOS == "darwin" {
		return c.Shortcut(ui.Ctrl|ui.Super, ui.KeyT)
	}
	return c.Shortcut(ui.Shift, ui.KeyF11)
}

// setTV turns the TV mode on or off and keeps it.
func (a *App) setTV(on bool) {
	a.prefs.TV = on
	a.tvKeys = tvKeyboard{}
	a.savePreferences()
}

// setAppearance applies and keeps the appearance.
func (a *App) setAppearance(appearance Appearance) {
	a.prefs.Appearance = appearance
	if a.opts.SetAppearance != nil {
		a.opts.SetAppearance(appearance)
	}
	a.savePreferences()
}

// setLanguage shows the app in a language and keeps it.
func (a *App) setLanguage(tag string) {
	a.prefs.Language = tag
	if a.opts.SetLocale != nil {
		a.opts.SetLocale(tag)
	}
	a.savePreferences()
}

// autoplayNext reports whether the next episode plays as one ends.
func (a *App) autoplayNext() bool { return !a.prefs.NoAutoplayNext }

// Appearance returns the appearance the user chose.
func (a *App) Appearance() Appearance {
	if a.prefs.Appearance == "" {
		return AppearanceSystem
	}
	return a.prefs.Appearance
}

func (a *App) loadPreferences() {
	if a.opts.Preferences == "" {
		return
	}
	b, err := os.ReadFile(a.opts.Preferences)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	if err == nil {
		err = json.Unmarshal(b, &a.prefs)
	}
	if err != nil {
		slog.WarnContext(a.ctx, "read the preferences", "error", err)
	}
}

func (a *App) savePreferences() {
	if a.opts.Preferences == "" {
		return
	}
	b, err := json.MarshalIndent(a.prefs, "", "  ")
	if err == nil {
		err = os.MkdirAll(filepath.Dir(a.opts.Preferences), 0o755)
	}
	if err == nil {
		err = os.WriteFile(a.opts.Preferences, b, 0o644)
	}
	if err != nil {
		slog.WarnContext(a.ctx, "write the preferences", "error", err)
	}
}

// View builds the window.
func (a *App) View(c *ui.Context) {
	c.SetTheme(style.Theme(c.Theme().Dark, a.prefs.TV && a.phase == phaseReady))
	a.windowKeys(c)
	if tvShortcut(c) {
		a.setTV(!a.prefs.TV)
	}
	switch a.phase {
	case phaseStarting:
		ui.Box(c).Fill().DragWindow()
	case phaseSignIn:
		a.signInView(c)
	default:
		// The PIN pad of the user switched to shows over every page.
		defer a.pinPad(c)
		defer a.assistantPanel(c, nil)
		if p := a.playerOf(nil); p != nil {
			a.playerView(c, p)
			return
		}
		if c.Shortcut(ui.Cmd, ui.KeyComma) && !strings.HasPrefix(a.router.Path(), "/settings") {
			a.router.Push("/settings")
		}
		// The settings have a layout of their own, as in the web UI.
		if strings.HasPrefix(a.router.Path(), "/settings") {
			if a.prefs.TV {
				a.tvSettingsView(c)
				return
			}
			a.router.View(c, func(r *ui.Route) { a.settings(c, r) })
			return
		}
		a.shell(c)
	}
}

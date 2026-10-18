package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	goruntime "runtime"
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
	// MavioDev is the API of the mavio.dev Mavio accounts sign in at.
	MavioDev string
	// NewVideo makes a video to play in, which tells of its changes from
	// its own goroutine; nil plays nothing.
	NewVideo func(changed func(mpv.State)) (Video, error)
	// KeepAwake keeps the display awake, for a reason the system shows,
	// until release is called.
	KeepAwake func(reason string) (release func())
	// ToggleFullScreen enters or leaves full screen; IsFullScreen tells
	// which.
	ToggleFullScreen func()
	IsFullScreen     func() bool
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
	// TV is the TV mode (docs/tv.md).
	TV bool `json:"tv,omitempty"`
	// Quality is the key of the quality played, of qualities.
	Quality string `json:"quality,omitempty"`
	// NoAutoplayNext stops playing the next episode as one ends.
	NoAutoplayNext bool `json:"noAutoplayNext,omitempty"`
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
	// player is what plays, if anything.
	player *player
	// account is the Mavio account signed in, if any.
	account *capiv1.Account
	servers []*capiv1.ServerRecord
	// state is the state of each server's stream.
	state map[string]capiv1.StreamState_State
	// generation counts the changes of the servers' content, so that what
	// shows them loads again.
	generation int
	libraries  map[string]*task[*capiv1.Libraries]

	sidebarOpen bool // the sidebar shown over the content in medium width
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
			clear(a.libraries)
			a.setServers(nil)
			a.router.Replace("/home")
		})
	}()
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
	c.SetTheme(style.Theme(c.Theme().Dark))
	if tvShortcut(c) {
		a.setTV(!a.prefs.TV)
	}
	switch a.phase {
	case phaseStarting:
		ui.Box(c).Fill().DragWindow()
	case phaseSignIn:
		a.signInView(c)
	default:
		if a.player != nil {
			a.playerView(c)
			return
		}
		if c.Shortcut(ui.Cmd, ui.KeyComma) && !strings.HasPrefix(a.router.Path(), "/settings") {
			a.router.Push("/settings/appearance")
		}
		// The settings have a layout of their own, as in the web UI.
		if strings.HasPrefix(a.router.Path(), "/settings") {
			a.router.View(c, func(r *ui.Route) { a.settings(c, r) })
			return
		}
		a.shell(c)
	}
}

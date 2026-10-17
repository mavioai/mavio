package app

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/images"
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
}

// App is the app's state, owned by the main thread.
type App struct {
	ctx    context.Context
	opts   Options
	prefs  preferences
	router *ui.Router

	phase   phase
	servers []*capiv1.ServerRecord
	// state is the state of each server's stream.
	state map[string]capiv1.StreamState_State
	// generation counts the changes of the servers' content, so that what
	// shows them loads again.
	generation int
	libraries  map[string]*task[*capiv1.Libraries]

	sidebarOpen bool // the sidebar shown over the content in medium width
	// contentWidth is the width of the page beside the sidebar.
	contentWidth  float32
	signIn        signIn
	homeState     homeState
	searchState   searchState
	scrolls       map[string]*ui.ScrollState
	grids         map[string]*grid
	libraryStates map[string]*libraryState
	detailStates  map[string]*detailsState
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
	}
	a.loadPreferences()
	return a
}

// Start lists the servers signed in to, showing them or the sign-in.
func (a *App) Start() {
	go func() {
		m, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{ListServers: &capiv1.ListServers{}}.Build())
		a.opts.Update(func() {
			if err != nil {
				slog.ErrorContext(a.ctx, "list the servers", "error", err)
			}
			a.setServers(m.GetServers().GetServers())
		})
		if _, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{Start: &capiv1.Start{}}.Build()); err != nil {
			slog.ErrorContext(a.ctx, "start the event streams", "error", err)
		}
	}()
}

// setServers shows servers, or the sign-in when there are none.
func (a *App) setServers(servers []*capiv1.ServerRecord) {
	a.servers = servers
	if len(servers) == 0 {
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

// signOut signs out of the server with id.
func (a *App) signOut(id string) {
	go func() {
		_, err := a.opts.Core.Call(a.ctx, capiv1.Request_builder{
			RemoveServer: capiv1.RemoveServer_builder{ServerId: id}.Build(),
		}.Build())
		if err != nil {
			slog.ErrorContext(a.ctx, "sign out", "server", id, "error", err)
			return
		}
		a.opts.Update(func() {
			a.setServers(slices.DeleteFunc(slices.Clone(a.servers), func(s *capiv1.ServerRecord) bool { return s.GetId() == id }))
			delete(a.libraries, id)
			a.router.Replace("/home")
		})
	}()
}

// setAppearance applies and keeps the appearance.
func (a *App) setAppearance(appearance Appearance) {
	a.prefs.Appearance = appearance
	if a.opts.SetAppearance != nil {
		a.opts.SetAppearance(appearance)
	}
	a.savePreferences()
}

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
	switch a.phase {
	case phaseStarting:
		ui.Box(c).Fill().DragWindow()
	case phaseSignIn:
		a.signInView(c)
	default:
		a.shell(c)
	}
}

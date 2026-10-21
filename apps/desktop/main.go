// Command mavio-desktop is Mavio's desktop app for macOS, Windows and Linux,
// built on MyGo's native UI with the shared core and libmpv.
package main

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/mavioai/mavio/libs/client-rust/capi"
	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"

	"github.com/mavioai/mavio/apps/desktop/internal/app"
	"github.com/mavioai/mavio/apps/desktop/internal/images"
	"github.com/mavioai/mavio/apps/desktop/internal/mpv"
	"github.com/mavioai/mavio/apps/desktop/internal/secrets"
	"github.com/mavioai/mavio/apps/desktop/internal/voice"
)

func main() {
	if err := run(); err != nil {
		slog.Error("mavio", "error", err)
		mygo.Dialog.Error("Mavio", err.Error())
		os.Exit(1)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	data, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		return err
	}
	cacheDir, err := mygo.App.Path(mygo.PathCache)
	if err != nil {
		return err
	}
	resources, err := mygo.App.Path(mygo.PathResources)
	if err != nil {
		return err
	}

	lib, err := capi.Load(library(resources, "maviocore"))
	if err != nil {
		return err
	}
	// A development build is signed anew each time, which the credential
	// store would ask about each launch: it keeps its secrets in a file.
	var backend secrets.Backend = mygo.Secrets
	if mygo.IsDev() {
		backend = secrets.NewFile(filepath.Join(data, "dev-secrets.json"), mygo.Secrets, mygo.ErrSecretNotFound)
	}
	store := secrets.New(backend, filepath.Join(data, "secrets.json"), mygo.ErrSecretNotFound)
	tokens, err := store.Load()
	if err != nil {
		return err
	}
	device, err := deviceID(filepath.Join(data, "device-id"))
	if err != nil {
		return err
	}
	host, _ := os.Hostname()

	// Tokens reach the credential store one after another, off the core's
	// threads.
	tokenChanges := make(chan *capiv1.Event, 16)
	go func() {
		for e := range tokenChanges {
			if err := store.Set(e.GetServerId(), e.GetTokenChanged().GetToken()); err != nil {
				slog.ErrorContext(ctx, "keep a token", "error", err)
			}
		}
	}()
	var a *app.App
	events := func(e *capiv1.Event) {
		if e.HasTokenChanged() {
			tokenChanges <- e
			return
		}
		if a != nil {
			a.Event(e)
		}
	}
	client, err := lib.Open(ctx, capiv1.Open_builder{
		DatabasePath: filepath.Join(data, "core.db"),
		Device: capiv1.Device_builder{
			Id:            device,
			Name:          strings.TrimSuffix(host, ".local"),
			Client:        "Mavio for " + platforms[runtime.GOOS],
			ClientVersion: mygo.App.Version(),
		}.Build(),
		Tokens: tokens,
	}.Build(), events)
	if err != nil {
		return err
	}

	var win *mygo.Window
	// players are the player's windows open, which draw again with the
	// app's: both on the main thread alone.
	players := map[*mygo.Window]bool{}
	// order is the app's windows in the order they opened, which ⌘` goes
	// through.
	var order []*mygo.Window
	update := func(fn func()) {
		if win != nil {
			win.Update(func() {
				fn()
				for w := range players {
					w.Update(func() {})
				}
			})
		}
	}
	// libmpv loads as the first video plays: on macOS after MoltenVK,
	// which it links.
	mpvLibrary := sync.OnceValues(func() (*mpv.Library, error) {
		var deps []string
		if runtime.GOOS == "darwin" {
			deps = append(deps, library(resources, "MoltenVK"))
		}
		return mpv.Load(library(resources, mpvName[runtime.GOOS]), deps...)
	})
	loader := images.New(ctx, &http.Client{Timeout: 30 * time.Second}, filepath.Join(cacheDir, "images"), 600, update)
	// The app is written in the design system's languages, which the
	// language it shows is chosen among.
	mygo.App.SetLanguages(design.Locales...)
	a = app.New(ctx, app.Options{
		Core:          client,
		Images:        loader,
		Update:        update,
		Preferences:   filepath.Join(data, "preferences.json"),
		SetAppearance: setAppearance,
		SetLocale:     mygo.App.SetLocale,
		OpenURL:       mygo.Shell.OpenExternal,
		ChooseFile: func(kind string, extensions []string) (string, error) {
			paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: win, Filters: []mygo.FileFilter{{Name: kind, Extensions: extensions}}})
			if err != nil || len(paths) == 0 {
				return "", err
			}
			return paths[0], nil
		},
		MavioDev: cmp.Or(os.Getenv("MAVIO_DEV_URL"), "https://mavio.dev/api"),
		NewWindow: func(view func(*ui.Context), closed func()) app.Window {
			opts := mygo.WindowOptions{
				Title:           "Mavio",
				Width:           1280,
				Height:          720,
				MinWidth:        480,
				MinHeight:       270,
				TitleBarStyle:   mygo.TitleBarHidden,
				BackgroundColor: "#000000",
				Content:         ui.View(view),
			}
			// The first keeps where it was left; the others open beside it.
			if len(players) == 0 {
				opts.StateKey = "player"
			}
			w := mygo.NewWindow(opts)
			players[w] = true
			order = append(order, w)
			w.OnClosed(func() {
				delete(players, w)
				order = slices.DeleteFunc(order, func(o *mygo.Window) bool { return o == w })
				update(closed)
			})
			return w
		},
		NewVideo: func(w app.Window, changed func(mpv.State)) (app.Video, error) {
			in := win
			if pw, ok := w.(*mygo.Window); ok {
				in = pw
			}
			return newVideo(in, mpvLibrary, changed)
		},
		KeepAwake: func(reason string) func() { return mygo.Power.KeepAwake(reason, true) },
		Voice:     voice.System(),
		ToggleFullScreen: func() {
			if win != nil {
				win.ToggleFullScreen()
			}
		},
		IsFullScreen: func() bool { return win != nil && win.IsFullScreen() },
		NextWindow: func(back bool) {
			all := append([]*mygo.Window{win}, order...)
			if len(all) < 2 {
				return
			}
			i := max(slices.Index(all, mygo.FocusedWindow()), 0)
			step := 1
			if back {
				step = len(all) - 1
			}
			all[(i+step)%len(all)].Focus()
		},
	})
	setAppearance(a.Appearance())

	mygo.App.WhenReady(func() {
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
			{Role: mygo.RoleViewMenu},
			{Role: mygo.RoleWindowMenu},
		}))
		win = mygo.NewWindow(mygo.WindowOptions{
			Title:  "Mavio",
			Width:  1280,
			Height: 800,
			// The app has the desktop layout alone: the settings' sidebar
			// beside a section still wide enough for its rows.
			MinWidth:      1024,
			MinHeight:     680,
			TitleBarStyle: mygo.TitleBarHidden,
			StateKey:      "main",
			Vibrancy:      mygo.VibrancySidebar,
			Content:       ui.View(a.View),
		})
		a.Start()
		mygo.App.OnDidBecomeActive(a.Resume)
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		cancel()
		client.Close()
	})
	return mygo.App.Run()
}

// mpvName is the name of libmpv's shared library, as library takes it.
var mpvName = map[string]string{"darwin": "mpv.2", "windows": "mpv-2", "linux": "mpv"}

// platforms names the operating systems.
var platforms = map[string]string{"darwin": "macOS", "windows": "Windows", "linux": "Linux"}

func setAppearance(appearance app.Appearance) {
	switch appearance {
	case app.AppearanceLight:
		mygo.Theme.SetSource(mygo.ThemeLight)
	case app.AppearanceDark:
		mygo.Theme.SetSource(mygo.ThemeDark)
	default:
		mygo.Theme.SetSource(mygo.ThemeSystem)
	}
}

// library returns the path of the shared library name the app ships with,
// in the lib directory of its resources: that of a packaged app, which
// merges those of the platform's directories, else, under go run, that of
// the platform's directory.
func library(resources, name string) string {
	switch runtime.GOOS {
	case "darwin":
		name = "lib" + name + ".dylib"
	case "windows":
		name += ".dll"
	default:
		name = "lib" + name + ".so"
	}
	dirs := []string{"", runtime.GOOS + "-" + runtime.GOARCH, runtime.GOOS}
	for _, d := range dirs {
		path := filepath.Join(resources, d, "lib", name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return filepath.Join(resources, "lib", name)
}

// loadMPV loads the libmpv the app ships with, once.

// video is a video view of the window that mpv draws into.
type video struct {
	*mpv.Player
	view *mygo.VideoView
}

func (v video) View() ui.NativeVideoView { return v.view }

// Close stops mpv, then closes its view.
func (v video) Close() {
	v.Player.Close()
	v.view.Destroy()
}

// newVideo makes a video view of win with mpv drawing into it.
func newVideo(win *mygo.Window, mpvLibrary func() (*mpv.Library, error), changed func(mpv.State)) (app.Video, error) {
	lib, err := mpvLibrary()
	if err != nil {
		return nil, err
	}
	view, err := win.NewVideoView()
	if err != nil {
		return nil, err
	}
	options := [][2]string{
		{"config", "no"},
		{"terminal", "no"},
		{"wid", strconv.FormatUint(uint64(view.Handle()), 10)},
		{"vo", "gpu-next"},
		{"hwdec", "auto-safe"},
		// HDR goes out as it is, through EDR on the Mac.
		{"target-colorspace-hint", "yes"},
		{"idle", "yes"},
		{"force-window", "yes"},
		{"keep-open", "no"},
		// The app's controller takes the pointer and the keyboard.
		{"input-default-bindings", "no"},
		{"input-vo-keyboard", "no"},
		{"input-cursor", "no"},
		{"cursor-autohide", "no"},
		{"osd-level", "0"},
		{"sub-auto", "no"},
		{"audio-display", "no"},
	}
	if runtime.GOOS == "darwin" {
		options = append(options, [2]string{"gpu-api", "vulkan"}, [2]string{"gpu-context", "macvk"})
	}
	p, err := lib.New(options, changed)
	if err != nil {
		view.Destroy()
		return nil, err
	}
	return video{Player: p, view: view}, nil
}

// deviceID returns the installation's ID, made the first time.
func deviceID(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err == nil && len(b) > 0 {
		return strings.TrimSpace(string(b)), nil
	}
	if !errors.Is(err, os.ErrNotExist) && err != nil {
		return "", fmt.Errorf("read the device ID: %w", err)
	}
	id := make([]byte, 16)
	rand.Read(id)
	s := hex.EncodeToString(id)
	if err := os.WriteFile(path, []byte(s), 0o600); err != nil {
		return "", fmt.Errorf("write the device ID: %w", err)
	}
	return s, nil
}

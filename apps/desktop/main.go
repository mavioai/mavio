// Command mavio-desktop is Mavio's desktop app for macOS, Windows and Linux,
// built on MyGo's native UI with the shared core and libmpv.
package main

import (
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
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"github.com/mavioai/mavio/libs/client-rust/capi"
	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/app"
	"github.com/mavioai/mavio/apps/desktop/internal/images"
	"github.com/mavioai/mavio/apps/desktop/internal/secrets"
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
	store := secrets.New(mygo.Secrets, filepath.Join(data, "secrets.json"), mygo.ErrSecretNotFound)
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
	update := func(fn func()) {
		if win != nil {
			win.Update(fn)
		}
	}
	loader := images.New(ctx, &http.Client{Timeout: 30 * time.Second}, filepath.Join(cacheDir, "images"), 600, update)
	a = app.New(ctx, app.Options{
		Core:          client,
		Images:        loader,
		Update:        update,
		Preferences:   filepath.Join(data, "preferences.json"),
		SetAppearance: setAppearance,
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
			Title:         "Mavio",
			Width:         1280,
			Height:        800,
			MinWidth:      600,
			MinHeight:     480,
			TitleBarStyle: mygo.TitleBarHidden,
			StateKey:      "main",
			Vibrancy:      mygo.VibrancySidebar,
			Content:       ui.View(a.View),
		})
		a.Start()
	})
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) {
		cancel()
		client.Close()
	})
	return mygo.App.Run()
}

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

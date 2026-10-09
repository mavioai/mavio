// Command mavio runs the Mavio media server.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mavioai/mavio/apps/server/internal/buildinfo"
	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/apps/server/internal/images"
	"github.com/mavioai/mavio/apps/server/internal/playback"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/apps/server/internal/providers"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/libs/store"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		slog.Error("mavio exited", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("mavio", flag.ContinueOnError)
	addr := fs.String("addr", ":8686", "HTTP listen address")
	database := fs.String("database", "sqlite:mavio.db", "database: sqlite:<path> or postgres://…")
	ffmpeg := fs.String("ffmpeg", "ffmpeg", "ffmpeg binary; without it media is only played directly")
	ffprobe := fs.String("ffprobe", "ffprobe", "ffprobe binary")
	transcodes := fs.String("transcode-dir", filepath.Join(os.TempDir(), "mavio-transcodes"), "directory for transcodes")
	cacheDir := fs.String("cache-dir", defaultCacheDir(), "directory for downloaded and resized images and compiled plugins")
	pluginDir := fs.String("plugin-dir", "", "directory holding one folder per plugin, each with its manifest.json")
	dev := fs.Bool("dev", false, "serve the development player at /dev/player")
	devLibrary := fs.String("dev-library", "", "add the Movies and Shows folders of this directory as libraries, e.g. .fixtures/dev-library")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}

	version := buildinfo.Version()
	if *showVersion {
		fmt.Println(version)
		return nil
	}

	db, err := store.Open(ctx, *database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			slog.ErrorContext(ctx, "close database", "err", err)
		}
	}()
	if *devLibrary != "" {
		if err := addDevLibraries(ctx, db, *devLibrary); err != nil {
			return err
		}
	}
	plugs, err := plugins.Open(ctx, plugins.Config{
		Dir: *pluginDir, CacheDir: filepath.Join(*cacheDir, "plugins"), Store: db, Logger: slog.Default(),
	})
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := plugs.Close(closeCtx); err != nil {
			slog.ErrorContext(ctx, "stop plugins", "err", err)
		}
	}()
	playbacks, ffmpegVersion := newPlaybacks(ctx, db, *ffmpeg, *ffprobe, *transcodes)
	imageServer := images.New(images.Config{Store: db, Dir: filepath.Join(*cacheDir, "images"), Logger: slog.Default()})
	h, err := httpserver.Handler(httpserver.Options{
		Version: version, Store: db, Database: db.Dialect(), FFmpegVersion: ffmpegVersion, Playbacks: playbacks,
		Images: imageServer, Plugins: plugs, Dev: *dev,
	})
	if err != nil {
		return err
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	slog.InfoContext(ctx, "starting mavio", "version", version)
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return playbacks.Run(ctx) })
	g.Go(func() error { return httpserver.Serve(ctx, ln, h) })
	if worker := newLibraryWorker(ctx, db, *ffprobe, imageServer, plugs.MetadataProviders()); worker != nil {
		g.Go(func() error {
			if err := worker.Run(ctx); !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		})
	}
	return g.Wait()
}

// addDevLibraries adds the Movies and Shows folders of dir as libraries,
// unless libraries already hold them; the startup scan then scans them.
func addDevLibraries(ctx context.Context, db core.Store, dir string) error {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for _, l := range []struct {
		name, folder string
		kind         core.LibraryKind
	}{{"Dev Movies", "Movies", core.LibraryMovies}, {"Dev Shows", "Shows", core.LibraryShows}} {
		path := filepath.Join(dir, l.folder)
		if info, err := os.Stat(path); err != nil || !info.IsDir() {
			return fmt.Errorf("dev library: %s is not a folder; generate it with pnpm nx run fixtures:dev-library", path)
		}
		lib := core.Library{Name: l.name, Kind: l.kind, Paths: []string{path}}
		switch err := db.Libraries().Create(ctx, &lib); {
		case errors.Is(err, core.ErrConflict):
			// Added before.
		case err != nil:
			return fmt.Errorf("dev library %s: %w", l.name, err)
		default:
			slog.InfoContext(ctx, "added dev library", "library", l.name, "path", path)
		}
	}
	return nil
}

// defaultCacheDir is the user's cache directory for Mavio, or one in the
// temporary directory.
func defaultCacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "mavio")
	}
	return filepath.Join(os.TempDir(), "mavio-cache")
}

// newLibraryWorker returns the worker running scans, probes, keyframe
// extractions, metadata refreshes and image placeholders, with a scan of every library queued,
// or nil when there is no ffprobe to probe media with.
func newLibraryWorker(ctx context.Context, db core.Store, ffprobe string, analyzer library.ImageAnalyzer, metadata []library.Provider) *library.Worker {
	path, err := exec.LookPath(ffprobe)
	if err != nil {
		slog.ErrorContext(ctx, "ffprobe unavailable; libraries are not scanned", "ffprobe", ffprobe, "err", err)
		return nil
	}
	jobs := &library.Jobs{
		Store:     db,
		Scanner:   &library.Scanner{Store: db, Resolver: library.NewResolver(), Logger: slog.Default()},
		Prober:    providers.FFprobe{Prober: &probe.Prober{FFprobe: path}},
		Keyframes: providers.Keyframes{Extractor: &keyframes.Extractor{FFprobe: path}},
		Refresher: &library.Refresher{Store: db, Providers: metadata, Logger: slog.Default()},
		Images:    analyzer,
		Logger:    slog.Default(),
	}
	if err := jobs.Schedule(ctx); err != nil {
		slog.ErrorContext(ctx, "schedule library scans", "err", err)
	}
	host, _ := os.Hostname()
	return &library.Worker{
		Queue: db.Jobs(), Owner: fmt.Sprintf("%s:%d", host, os.Getpid()), Handlers: jobs.Handlers(), Logger: slog.Default(),
	}
}

// newPlaybacks sets up playback with the ffmpeg found on this host, or
// for direct play only without one. It returns the ffmpeg version, empty
// without ffmpeg.
func newPlaybacks(ctx context.Context, db core.Store, ffmpeg, ffprobe, dir string) (*playback.Manager, string) {
	cfg := playback.Config{Store: db, Dir: dir, Logger: slog.Default()}
	v, err := cfg.UseFFmpeg(ctx, ffmpeg, ffprobe)
	if err != nil {
		slog.WarnContext(ctx, "ffmpeg unavailable; media plays directly only", "ffmpeg", ffmpeg, "err", err)
	} else {
		slog.InfoContext(ctx, "ffmpeg found", "version", v)
	}
	return playback.NewManager(cfg), v
}

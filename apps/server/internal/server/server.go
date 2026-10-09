package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/apps/server/internal/images"
	"github.com/mavioai/mavio/apps/server/internal/playback"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/apps/server/internal/providers"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/library/storage"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/libs/store"
)

// Config configures a server.
type Config struct {
	Version string
	// Database is "sqlite:<path>" or a PostgreSQL URL.
	Database string
	// FFmpeg and FFprobe are the binaries, names on PATH or paths; without
	// ffmpeg media plays directly only, without ffprobe libraries are not
	// scanned.
	FFmpeg, FFprobe string
	// TranscodeDir holds the segments of running transcodes.
	TranscodeDir string
	// CacheDir holds downloaded and resized images and compiled plugins.
	CacheDir string
	// PluginDir holds one folder per plugin; empty means no plugins.
	PluginDir string
	// Dev serves the development player at /dev/player.
	Dev bool
	// DevLibrary adds the Movies and Shows folders of the sample library
	// in this directory as libraries.
	DevLibrary string
	Logger     *slog.Logger
}

// Run serves on ln until ctx ends.
func Run(ctx context.Context, cfg Config, ln net.Listener) error {
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	raw, err := store.Open(ctx, cfg.Database)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() {
		if err := raw.Close(); err != nil {
			log.ErrorContext(ctx, "close database", "err", err)
		}
	}()
	// What is written reaches the devices' event streams.
	hub := events.New(events.Config{Store: raw, Logger: log})
	db := events.Observe(raw, hub)
	if cfg.DevLibrary != "" {
		if err := addDevLibraries(ctx, log, db, cfg.DevLibrary); err != nil {
			return err
		}
	}
	plugs, err := plugins.Open(ctx, plugins.Config{
		Dir: cfg.PluginDir, CacheDir: filepath.Join(cfg.CacheDir, "plugins"), Store: db, Logger: log,
	})
	if err != nil {
		return err
	}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := plugs.Close(closeCtx); err != nil {
			log.ErrorContext(ctx, "stop plugins", "err", err)
		}
	}()
	quietGate := storage.NewQuietGate()
	volumeLedger := storage.NewVolumeLedger(2*time.Second, 500*time.Millisecond)
	growthPolicy := storage.NewGrowthPolicy(10 * time.Second)

	playbacks, ffmpegVersion := newPlaybacks(ctx, log, db, hub, cfg, quietGate)
	imageServer := images.New(images.Config{Store: db, Dir: filepath.Join(cfg.CacheDir, "images"), Logger: log})
	h, err := httpserver.Handler(httpserver.Options{
		Version: cfg.Version, Store: db, Hub: hub, Database: raw.Dialect(), FFmpegVersion: ffmpegVersion, Playbacks: playbacks,
		Images: imageServer, Plugins: plugs, Dev: cfg.Dev,
	})
	if err != nil {
		return err
	}

	log.InfoContext(ctx, "starting mavio", "version", cfg.Version, "addr", ln.Addr().String())
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return playbacks.Run(ctx) })
	g.Go(func() error { return httpserver.Serve(ctx, ln, h) })
	if worker := newLibraryWorker(ctx, log, db, cfg.FFprobe, imageServer, plugs.MetadataProviders(), quietGate, volumeLedger, growthPolicy); worker != nil {
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
func addDevLibraries(ctx context.Context, log *slog.Logger, db core.Store, dir string) error {
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
		case err != nil:
			return fmt.Errorf("dev library %s: %w", l.name, err)
		default:
			log.InfoContext(ctx, "added dev library", "library", l.name, "path", path)
		}
	}
	return nil
}

// newLibraryWorker returns the worker running scans, probes, keyframe
// extractions, metadata refreshes and image placeholders, with a scan of
// every library queued, or nil when there is no ffprobe to probe media
// with.
func newLibraryWorker(ctx context.Context, log *slog.Logger, db core.Store, ffprobe string, analyzer library.ImageAnalyzer,
	metadata []library.Provider, quietGate *storage.QuietGate, volumeLedger *storage.VolumeLedger, growthPolicy *storage.GrowthPolicy,
) *library.Worker {
	path, err := exec.LookPath(ffprobe)
	if err != nil {
		log.ErrorContext(ctx, "ffprobe unavailable; libraries are not scanned", "ffprobe", ffprobe, "err", err)
		return nil
	}
	jobs := &library.Jobs{
		Store: db,
		Scanner: &library.Scanner{
			Store: db, Resolver: library.NewResolver(), Logger: log,
			QuietGate:    quietGate,
			VolumeLedger: volumeLedger,
			GrowthPolicy: growthPolicy,
		},
		Prober:    providers.FFprobe{Prober: &probe.Prober{FFprobe: path}},
		Keyframes: providers.Keyframes{Extractor: &keyframes.Extractor{FFprobe: path}},
		Refresher: &library.Refresher{Store: db, Providers: metadata, Logger: log},
		Images:    analyzer,
		Logger:    log,
	}
	if err := jobs.Schedule(ctx); err != nil {
		log.ErrorContext(ctx, "schedule library scans", "err", err)
	}
	host, _ := os.Hostname()
	return &library.Worker{
		Queue: db.Jobs(), Owner: fmt.Sprintf("%s:%d", host, os.Getpid()), Handlers: jobs.Handlers(), Logger: log,
	}
}

// newPlaybacks sets up playback with the configured ffmpeg, or for direct
// play only without one. It returns the ffmpeg version, empty without
// ffmpeg.
func newPlaybacks(ctx context.Context, log *slog.Logger, db core.Store, hub *events.Hub, cfg Config, quietGate *storage.QuietGate) (*playback.Manager, string) {
	pc := playback.Config{
		Store: db, Dir: cfg.TranscodeDir, OnChange: hub.SessionsChanged, QuietGate: quietGate, Logger: log,
	}
	v, err := pc.UseFFmpeg(ctx, cfg.FFmpeg, cfg.FFprobe)
	if err != nil {
		log.WarnContext(ctx, "ffmpeg unavailable; media plays directly only", "ffmpeg", cfg.FFmpeg, "err", err)
	} else {
		log.InfoContext(ctx, "ffmpeg found", "version", v)
	}
	return playback.NewManager(pc), v
}

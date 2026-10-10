package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/backup"
	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/apps/server/internal/images"
	"github.com/mavioai/mavio/apps/server/internal/logs"
	"github.com/mavioai/mavio/apps/server/internal/network"
	"github.com/mavioai/mavio/apps/server/internal/playback"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/apps/server/internal/providers"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/apps/server/internal/warming"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/library/storage"
	"github.com/mavioai/mavio/libs/media/borders"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/libs/media/thumbnails"
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
	// PluginDataDir holds the plugins' data folders; empty gives plugins
	// none.
	PluginDataDir string
	// MetadataDir holds the artwork chosen for items of libraries that do
	// not save metadata next to their media.
	MetadataDir string
	// BackupDir holds backups; empty makes none.
	BackupDir string
	// Restore, when set, is a backup restored into the empty database
	// before the server starts.
	Restore string
	// DiscoveryAddr is the UDP address local discovery answers on, such
	// as ":7359"; empty answers none.
	DiscoveryAddr string
	// HTTPSHost is the host HTTPS listens on; empty means every interface.
	HTTPSHost string
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
	// Administrators read the recent records through the API.
	ring := logs.NewRing(2000)
	log = slog.New(ring.Handler(log.Handler(), slog.LevelInfo))
	metadataDir, err := filepath.Abs(cfg.MetadataDir)
	if err != nil {
		return err
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
	if cfg.Restore != "" {
		if err := backup.Restore(ctx, cfg.Restore, raw, backup.Folders{
			Metadata: metadataDir, Plugins: cfg.PluginDir, PluginData: cfg.PluginDataDir,
		}); err != nil {
			return fmt.Errorf("restore %s: %w", cfg.Restore, err)
		}
		log.InfoContext(ctx, "restored backup", "backup", cfg.Restore)
	}
	// The activity log comes first: the hub and the plugins record and
	// publish events, which reach the plugins once they started.
	var started atomic.Pointer[plugins.Manager]
	activityLog := activity.New(activity.Config{
		Store: raw, Logger: log,
		Notifiers: func() []activity.Notifier {
			if m := started.Load(); m != nil {
				return m.Notifiers()
			}
			return nil
		},
		Events: func(a core.Activity) {
			if m := started.Load(); m != nil {
				m.Publish(a)
			}
		},
		Wants: func(eventType string) bool {
			m := started.Load()
			return m != nil && m.Wants(eventType)
		},
	})
	// What is written reaches the devices' event streams.
	hub := events.New(events.Config{Store: raw, Events: activityLog, Logger: log})
	db := events.Observe(raw, hub)
	if cfg.DevLibrary != "" {
		if err := addDevLibraries(ctx, log, db, cfg.DevLibrary); err != nil {
			return err
		}
	}
	set, err := settings.Open(ctx, db)
	if err != nil {
		return err
	}
	// Plugins start before the handler tree exists; their host API calls
	// wait for it.
	hostAPI := newLateHandler()
	plugs, err := plugins.Open(ctx, plugins.Config{
		Dir: cfg.PluginDir, CacheDir: filepath.Join(cfg.CacheDir, "plugins"), Store: db, Logger: log, HostAPI: hostAPI,
		DataDir: cfg.PluginDataDir, Activity: activityLog,
		Catalogs: func() []string { return set.Get().PluginCatalogs },
	})
	if err != nil {
		return err
	}
	started.Store(plugs)
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
		defer cancel()
		if err := plugs.Close(closeCtx); err != nil {
			log.ErrorContext(ctx, "stop plugins", "err", err)
		}
	}()
	quietGate := storage.NewQuietGate(0)
	volumeLedger := storage.NewVolumeLedger()
	devices := &storage.Detector{}
	growthPolicy := storage.NewGrowthPolicy()
	keeper := &storage.Keeper{Devices: devices}
	warmer := warming.New(warming.Config{Store: db, Keeper: keeper, Online: hub.OnlineSessions, Logger: log})

	playbacks, ffmpegVersion := newPlaybacks(ctx, log, db, hub, activityLog, cfg, quietGate, keeper)
	if err := set.Register(ctx, func(_ context.Context, s core.ServerSettings) error {
		return playbacks.SetTranscoding(s.Transcoding)
	}); err != nil {
		log.ErrorContext(ctx, "stored transcoding settings do not apply; change them", "err", err)
	}
	imageServer := images.New(images.Config{
		Store: db, Dir: filepath.Join(cfg.CacheDir, "images"), MetadataDir: metadataDir, Logger: log,
	})
	refresher := &library.Refresher{
		Store: db, Source: plugs.MetadataProviders, MetadataDir: filepath.ToSlash(metadataDir), Fetch: imageServer.Fetch, Logger: log,
	}
	var backups *backup.Manager
	if cfg.BackupDir != "" {
		backups = backup.New(backup.Config{
			Store: raw, Dir: cfg.BackupDir, Version: cfg.Version,
			Folders: backup.Folders{Metadata: metadataDir, Plugins: cfg.PluginDir, PluginData: cfg.PluginDataDir},
		})
	}
	h, err := httpserver.Handler(httpserver.Options{
		Version: cfg.Version, Store: db, Hub: hub, Database: raw.Dialect(), FFmpegVersion: ffmpegVersion, Playbacks: playbacks,
		Images: imageServer, Plugins: plugs, Refresher: refresher,
		Subtitles: &library.Subtitles{Store: db, Source: plugs.SubtitleProviders, Logger: log},
		Settings:  set, Accelerations: playbacks.Accelerations, Logs: ring, Activity: activityLog, Backups: backups,
		Authenticate: plugs.Authenticate, Wake: warmer.Wake, Dev: cfg.Dev,
	})
	if err != nil {
		return err
	}
	hostAPI.set(h)

	// The network settings decide the base URL, HTTPS and discovery.
	prefix := network.NewPrefix(h)
	https := &network.HTTPS{Handler: prefix, Host: cfg.HTTPSHost, Logger: log}
	port := 0
	if a, ok := ln.Addr().(*net.TCPAddr); ok {
		port = a.Port
	}
	discovery := &network.Discovery{Addr: cfg.DiscoveryAddr, Port: port, Logger: log, Reply: func() network.DiscoveryReply {
		name := set.Get().Network.ServerName
		if name == "" {
			name, _ = os.Hostname()
		}
		return network.DiscoveryReply{Name: name, Version: cfg.Version}
	}}
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = https.Close(closeCtx)
		_ = discovery.Close()
	}()
	if err := set.Register(ctx, func(ctx context.Context, s core.ServerSettings) error {
		if err := https.Apply(ctx, s.Network); err != nil {
			return err
		}
		prefix.Set(s.Network.BaseURL)
		return discovery.Apply(ctx, s.Network)
	}); err != nil {
		log.ErrorContext(ctx, "stored network settings do not apply; change them", "err", err)
	}

	log.InfoContext(ctx, "starting mavio", "version", cfg.Version, "addr", ln.Addr().String())
	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error { return playbacks.Run(ctx) })
	g.Go(func() error {
		if err := warmer.Run(ctx); !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	g.Go(func() error { return httpserver.Serve(ctx, ln, prefix) })
	g.Go(func() error {
		if err := activityLog.Run(ctx); !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	// Plugin tasks have a worker of their own, so that they neither wait
	// for library jobs nor need ffprobe.
	hostname, _ := os.Hostname()
	taskWorker := &library.Worker{
		Queue: db.Jobs(), Owner: fmt.Sprintf("%s:%d:plugins", hostname, os.Getpid()),
		Handlers: map[string]library.Handler{plugins.JobTask: plugs.RunTask}, Logger: log,
	}
	g.Go(func() error {
		if err := taskWorker.Run(ctx); !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	if worker, scanner := newLibraryWorker(ctx, log, db, cfg.FFprobe, cfg.FFmpeg, imageServer, refresher, plugs.SegmentProviders, quietGate, volumeLedger, devices, growthPolicy); worker != nil {
		g.Go(func() error {
			if err := worker.Run(ctx); !errors.Is(err, context.Canceled) {
				return err
			}
			return nil
		})
		g.Go(func() error {
			if err := scanner.RunDeferred(ctx, 0); !errors.Is(err, context.Canceled) {
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
// every library queued, and its scanner, or nil when there is no ffprobe
// to probe media with.
func newLibraryWorker(ctx context.Context, log *slog.Logger, db core.Store, ffprobe, ffmpeg string, analyzer library.ImageAnalyzer,
	refresher *library.Refresher, segments func() []library.SegmentProvider, quietGate *storage.QuietGate, volumeLedger *storage.VolumeLedger, devices *storage.Detector,
	growthPolicy *storage.GrowthPolicy,
) (*library.Worker, *library.Scanner) {
	path, err := exec.LookPath(ffprobe)
	if err != nil {
		log.ErrorContext(ctx, "ffprobe unavailable; libraries are not scanned", "ffprobe", ffprobe, "err", err)
		return nil, nil
	}
	jobs := &library.Jobs{
		Store: db,
		Scanner: &library.Scanner{
			Store: db, Resolver: library.NewResolver(), Logger: log,
			QuietGate:    quietGate,
			VolumeLedger: volumeLedger,
			Devices:      devices,
			GrowthPolicy: growthPolicy,
		},
		Prober:    providers.FFprobe{Prober: &probe.Prober{FFprobe: path}},
		Keyframes: providers.Keyframes{Extractor: &keyframes.Extractor{FFprobe: path}},
		Refresher: refresher,
		Borders:   bordersOf(ffmpeg),
		Segments:  segments, MetadataDir: refresher.MetadataDir,
		Fails:  library.NewFailNotes(24*time.Hour, 4096),
		Images: analyzer,
		Logger: log,
	}
	if ffmpeg, err := exec.LookPath(ffmpeg); err == nil {
		maker := &thumbnails.Maker{FFmpeg: ffmpeg}
		jobs.Thumbnails = providers.Thumbnails{Maker: maker, Options: thumbnails.DefaultOptions()}
		jobs.Loudness = maker
	}
	if err := jobs.Schedule(ctx); err != nil {
		log.ErrorContext(ctx, "schedule library scans", "err", err)
	}
	if err := jobs.ScheduleHousekeeping(ctx); err != nil {
		log.ErrorContext(ctx, "schedule housekeeping", "err", err)
	}
	host, _ := os.Hostname()
	return &library.Worker{
		Queue: db.Jobs(), Owner: fmt.Sprintf("%s:%d", host, os.Getpid()), Handlers: jobs.Handlers(), Logger: log,
	}, jobs.Scanner
}

// newPlaybacks sets up playback with the configured ffmpeg, or for direct
// play only without one. It returns the ffmpeg version, empty without
// ffmpeg.
func newPlaybacks(ctx context.Context, log *slog.Logger, db core.Store, hub *events.Hub, activityLog *activity.Log, cfg Config, quietGate *storage.QuietGate, keeper *storage.Keeper) (*playback.Manager, string) {
	pc := playback.Config{
		Store: db, Dir: cfg.TranscodeDir, OnChange: hub.SessionsChanged, Record: activityLog.Record, QuietGate: quietGate, Keeper: keeper, Logger: log,
	}
	v, err := pc.UseFFmpeg(ctx, cfg.FFmpeg, cfg.FFprobe)
	if err != nil {
		log.WarnContext(ctx, "ffmpeg unavailable; media plays directly only", "ffmpeg", cfg.FFmpeg, "err", err)
	} else {
		log.InfoContext(ctx, "ffmpeg found", "version", v)
	}
	return playback.NewManager(pc), v
}

// bordersOf returns the black border detector using ffmpeg, nil without
// ffmpeg.
func bordersOf(ffmpeg string) library.BorderDetector {
	path, err := exec.LookPath(ffmpeg)
	if ffmpeg == "" || err != nil {
		return nil
	}
	return providers.Borders{Detector: &borders.Detector{FFmpeg: path}}
}

// lateHandler serves a handler set after it is created; requests wait for
// it.
type lateHandler struct {
	ready chan struct{}
	h     http.Handler
}

func newLateHandler() *lateHandler { return &lateHandler{ready: make(chan struct{})} }

func (l *lateHandler) set(h http.Handler) {
	l.h = h
	close(l.ready)
}

func (l *lateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	select {
	case <-l.ready:
		l.h.ServeHTTP(w, r)
	case <-r.Context().Done():
		http.Error(w, "server starting", http.StatusServiceUnavailable)
	}
}

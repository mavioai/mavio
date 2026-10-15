package server

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/backup"
	"github.com/mavioai/mavio/apps/server/internal/certs"
	"github.com/mavioai/mavio/apps/server/internal/cloud"
	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/apps/server/internal/httpserver"
	"github.com/mavioai/mavio/apps/server/internal/identity"
	"github.com/mavioai/mavio/apps/server/internal/images"
	"github.com/mavioai/mavio/apps/server/internal/logs"
	"github.com/mavioai/mavio/apps/server/internal/network"
	"github.com/mavioai/mavio/apps/server/internal/playback"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/apps/server/internal/providers"
	"github.com/mavioai/mavio/apps/server/internal/relay"
	"github.com/mavioai/mavio/apps/server/internal/requests"
	"github.com/mavioai/mavio/apps/server/internal/rpc"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/apps/server/internal/subtitles"
	"github.com/mavioai/mavio/apps/server/internal/warming"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/library/storage"
	"github.com/mavioai/mavio/libs/media/borders"
	"github.com/mavioai/mavio/libs/media/keyframes"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/libs/media/thumbnails"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
	"github.com/mavioai/mavio/libs/store"
)

// Config configures a server.
type Config struct {
	Version string
	// Database is "sqlite:<path>" or a PostgreSQL URL.
	Database string
	// IdentityKey is the file of the server's identity key, created when
	// missing; empty uses a key of the run only, whose sessions end with
	// it.
	IdentityKey string
	// FFmpeg and FFprobe are the binaries, names on PATH or paths; without
	// ffmpeg media plays directly only, without ffprobe libraries are not
	// scanned.
	FFmpeg, FFprobe string
	// Rclone is the rclone binary the server runs as an rcd for libraries
	// on rclone remotes; empty, or not found, serves no remotes.
	Rclone string
	// RcloneConfig is the rclone.conf file of that rcd.
	RcloneConfig string
	// RcloneURL is the address of an rcd the server reaches instead of
	// running one, and RcloneUser and RclonePass its credentials.
	RcloneURL, RcloneUser, RclonePass string
	// RcloneCacheSize is the disk space, in bytes, the files of libraries
	// on cloud storage are cached in, under CacheDir; zero caches none.
	RcloneCacheSize int64
	// TranscodeDir holds the segments of running transcodes.
	TranscodeDir string
	// CacheDir holds downloaded and resized images and compiled plugins.
	CacheDir string
	// PluginCacheDir holds the compiled WASM plugins, which other servers
	// may share; empty keeps them under CacheDir.
	PluginCacheDir string
	// PluginDir holds one folder per plugin; empty means no plugins.
	PluginDir string
	// PluginSeedDir holds plugin folders installed into PluginDir once,
	// such as those a container image ships; empty seeds none.
	PluginSeedDir string
	// PluginDataDir holds the plugins' data folders; empty gives plugins
	// none.
	PluginDataDir string
	// PluginFetch performs the WASM plugins' outbound HTTP requests, which
	// their manifests allowed; nil uses a client with a 30-second timeout.
	PluginFetch *http.Client
	// MetadataDir holds the artwork chosen for items of libraries that do
	// not save metadata next to their media, and items' subtitles when
	// the subtitle settings keep them there.
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
	// MavioDevURL is the base URL of mavio.dev's API.
	MavioDevURL string
	// CertificateDir holds the *.<ID>.mavio.site certificate a claimed
	// server obtains; empty obtains none.
	CertificateDir string
	// ACMEDirectories are the ACME CAs the certificate is obtained from,
	// in order; empty uses Let's Encrypt, then Google Trust Services.
	ACMEDirectories []string
	// ClaimToken is a claim code from mavio.dev/claim, which claims the
	// server when it starts unclaimed; empty claims nothing.
	ClaimToken string
	// OpenBrowser opens the setup in the default browser when the server
	// starts without a mode or unclaimed, on a desktop.
	OpenBrowser bool
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
	key := identity.Generate()
	if cfg.IdentityKey != "" {
		if key, err = identity.Load(cfg.IdentityKey); err != nil {
			return err
		}
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
	hub := events.New(events.Config{
		Store: raw, Events: activityLog, Logger: log,
		// Plugins' devices are every user's.
		Shared: func(session core.ID) bool {
			m := started.Load()
			return m != nil && m.IsDevice(session)
		},
	})
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
	if cfg.PluginDir != "" && cfg.PluginSeedDir != "" {
		if err := plugins.Seed(ctx, cfg.PluginSeedDir, cfg.PluginDir, log); err != nil {
			log.WarnContext(ctx, "seed plugins", "err", err)
		}
	}
	pluginCache := cfg.PluginCacheDir
	if pluginCache == "" {
		pluginCache = filepath.Join(cfg.CacheDir, "plugins")
	}
	plugs, err := plugins.Open(ctx, plugins.Config{
		Dir: cfg.PluginDir, CacheDir: pluginCache, Store: db, Logger: log, HostAPI: hostAPI,
		DataDir: cfg.PluginDataDir, Fetch: cfg.PluginFetch, Activity: activityLog, DevicesChanged: hub.SharedSessionsChanged,
		Catalogs: func() []string { return set.Get().PluginCatalogs },
		Disabled: set.Get().DisabledPlugins,
		SaveDisabled: func(ctx context.Context, ids []string) error {
			_, err := set.Change(ctx, func(s *core.ServerSettings) error {
				s.DisabledPlugins = ids
				return nil
			})
			return err
		},
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
	classifier := storage.NewClassifier(devices)
	growthPolicy := storage.NewGrowthPolicy()
	// Paths are classified by the libraries holding them, whose storage
	// classes decide what the server reads on its own.
	pathClasses := &storage.PathClasses{Classifier: classifier, Libraries: db.Libraries().List}
	keeper := &storage.Keeper{Devices: devices, Class: func(p string) core.StorageClass {
		return pathClasses.Class(context.WithoutCancel(ctx), p)
	}}
	warmer := warming.New(warming.Config{Store: db, Keeper: keeper, Online: hub.OnlineSessions, Logger: log})

	// Library paths are local folders, or folders of rclone remotes.
	rc := startRclone(ctx, log, cfg, db.StorageJobs(), pathClasses.Class)
	defer rc.Close()
	storages := &library.Storages{}
	// Cloud drives of rclone remotes that download magnet links themselves
	// are download clients built into the server.
	offline := &downloads.Offline{Jobs: db.StorageJobs(), Configs: db.PluginConfigs()}
	clients := plugs.DownloadClientProviders
	if rc != nil {
		storages.Remote = rc.Remotes.Open
		offline.Remotes = rc.Remotes
		clients = func() []downloads.Provider {
			return append(plugs.DownloadClientProviders(), downloads.Provider{ID: downloads.OfflineID, Client: offline})
		}
	}
	playbacks, ffmpegVersion := newPlaybacks(ctx, log, db, hub, activityLog, plugs, cfg, quietGate, keeper, pathClasses, storages)
	if err := set.Register(ctx, func(_ context.Context, s core.ServerSettings) error {
		return playbacks.SetTranscoding(s.Transcoding)
	}); err != nil {
		log.ErrorContext(ctx, "stored transcoding settings do not apply; change them", "err", err)
	}
	imageServer := images.New(images.Config{
		Store: db, Dir: filepath.Join(cfg.CacheDir, "images"), MetadataDir: metadataDir, Storages: storages, Logger: log,
	})
	lyrics := &library.Lyrics{Store: db, Source: plugs.LyricsProviders, Storages: storages, Logger: log}
	refresher := &library.Refresher{
		Storages: storages,
		Store:    db, Source: plugs.MetadataProviders, Episodes: plugs.EpisodeListers, Images: plugs.ImageProviders, Local: plugs.LocalReaders, Savers: plugs.Savers,
		Processors: plugs.Processors, Generators: plugs.ImageGenerators, Lyrics: lyrics,
		MetadataDir: filepath.ToSlash(metadataDir), Fetch: imageServer.Fetch, Logger: log,
	}
	var backups *backup.Manager
	if cfg.BackupDir != "" {
		backups = backup.New(backup.Config{
			Store: raw, Dir: cfg.BackupDir, Version: cfg.Version,
			Folders: backup.Folders{Metadata: metadataDir, Plugins: cfg.PluginDir, PluginData: cfg.PluginDataDir},
		})
	}
	port := 0
	if a, ok := ln.Addr().(*net.TCPAddr); ok {
		port = a.Port
	}
	serverID := key.ID()
	serverName := func() string {
		if name := set.Get().Network.ServerName; name != "" {
			return name
		}
		name, _ := os.Hostname()
		return name
	}
	// In mavio.dev mode the server is claimed at mavio.dev and keeps a
	// link to it, which waits while the server is unclaimed.
	mavioDev := cloud.NewClient(cfg.MavioDevURL, key, nil)
	link := cloud.NewLink(db, mavioDev, log)
	link.Apply = (&cloud.Applier{Requests: func() core.RequestSettings { return set.Get().Requests }}).Apply
	assertions := cloud.NewVerifier(cfg.MavioDevURL, serverID, cloud.NewKeySet(cfg.MavioDevURL, nil))
	// A claimed server obtains its mavio.site certificate.
	var certificates *certs.Manager
	if cfg.CertificateDir != "" {
		certificates = certs.NewManager(cfg.CertificateDir, certs.Name(serverID), mavioDev, func(ctx context.Context) (bool, error) {
			s, err := db.Settings().Accounts(ctx)
			return s.Claimed(), err
		}, log)
		if len(cfg.ACMEDirectories) > 0 {
			certificates.CAs = cfg.ACMEDirectories
		}
	}
	// With its certificate, a claimed server is reachable through the
	// relay unless its settings turn that off.
	var relays *relay.Client
	if certificates != nil {
		relays = relay.NewClient(key, mavioDev, func(ctx context.Context) (bool, error) {
			s, err := db.Settings().Accounts(ctx)
			return s.Claimed() && set.Get().Network.Relay && certificates.Ready(), err
		}, log)
	}
	claimed := func() {
		link.Wake()
		if certificates != nil {
			certificates.Wake()
			relays.Wake()
		}
	}
	claimer := &cloud.Claimer{Store: db, Client: mavioDev, Name: serverName, Version: cfg.Version, Activity: activityLog, Claimed: claimed}
	acq := newAcquisition(log, db, set, plugs, clients, refresher, cfg.FFprobe, quietGate, volumeLedger, devices)
	reqs := &requests.Service{
		Store: db, Acquisition: acq, Settings: func() core.RequestSettings { return set.Get().Requests },
		Metadata: plugs.MetadataProviders, Episodes: plugs.EpisodeListers, Discovery: plugs.DiscoveryProviders,
		Activity: activityLog, Events: hub.Notify, Logger: log,
		MetadataSettings: func() core.MetadataSettings { return set.Get().Metadata },
		DropBlocklistedTags: func(ctx context.Context, tags []string) error {
			_, err := set.Change(ctx, func(s *core.ServerSettings) error {
				s.Requests.BlocklistedTags = slices.DeleteFunc(s.Requests.BlocklistedTags, func(t string) bool { return slices.Contains(tags, t) })
				return nil
			})
			return err
		},
	}
	subFiles := &library.Subtitles{Store: db, Source: plugs.SubtitleProviders, Storages: storages, Folder: subtitleFolder(set, metadataDir), Logger: log}
	subs := &subtitles.Runtime{
		Store: db, Settings: func() core.SubtitleSettings { return set.Get().Subtitles }, Providers: plugs.SubtitlePlugins,
		Translators: plugs.SubtitleTranslators,
		AI: func() (pluginv1connect.AiGatewayServiceClient, bool) {
			_, c, ok := plugs.AIGateway()
			return c, ok
		},
		Subtitles: subFiles, Storages: storages, Class: pathClasses.Class, FFmpeg: ffmpegPath(cfg.FFmpeg),
		Extracted: playbacks.ExtractedSubtitle, Activity: activityLog, Logger: log,
	}
	// Imports complete the requests of what they import; their scans
	// search for its subtitles once the files are probed.
	acq.Imported = reqs.Imported
	// The network settings decide the base URL, HTTPS and discovery.
	https := &network.HTTPS{Host: cfg.HTTPSHost, Logger: log}
	// mavio.dev gives the accounts that reach the server where it is.
	link.Connections = func() []string {
		return connections(serverID, localAddresses(), port, https.Port(), certificates != nil && certificates.Ready(), set.Get().Network.BaseURL)
	}
	h, err := httpserver.Handler(httpserver.Options{
		Version: cfg.Version, Store: db, Identity: key, Hub: hub, Database: raw.Dialect(), FFmpegVersion: ffmpegVersion, Playbacks: playbacks,
		Images: imageServer, Plugins: plugs, Devices: plugs, Refresher: refresher, Storage: classifier, Storages: storages, Remotes: rc.Service(offline),
		Subtitles: subFiles, SubtitleRuntime: subs,
		Lyrics:   lyrics,
		Intros:   &library.Intros{Store: db, Source: plugs.IntroProviders, Logger: log},
		Settings: set, Accelerations: playbacks.Accelerations, Logs: ring, Activity: activityLog, Backups: backups,
		ServerInfo: func() rpc.ServerInfo {
			n := set.Get().Network
			return rpc.ServerInfo{ID: serverID, Name: serverName(), Version: cfg.Version, BaseURL: n.BaseURL, HTTPPort: port, HTTPSPort: https.Port()}
		},
		SignIn: claimer, Assertions: assertions, MavioDevURL: cfg.MavioDevURL,
		Authenticate: plugs.Authenticate, ResetPlugin: func() string { return set.Get().PasswordResetPlugin }, StartReset: plugs.StartPasswordReset, PluginRoutes: plugs.Routes, Wake: warmer.Wake, ExternalIDKinds: plugs.ExternalIDKinds, Indexers: plugs.IndexerProviders,
		Downloads: plugs.DownloadClientProviders, Acquisition: acq, Requests: reqs, Dev: cfg.Dev,
	})
	if err != nil {
		return err
	}
	hostAPI.set(h)

	prefix := network.NewPrefix(h)
	https.Handler = prefix
	if certificates != nil {
		https.Managed = certificates
		// The first certificate starts HTTPS; later ones are served as
		// they come.
		certificates.Changed = func() {
			if err := https.Apply(ctx, set.Get().Network); err != nil {
				log.ErrorContext(ctx, "serve HTTPS with the mavio.site certificate", "err", err)
			}
			relays.Wake()
		}
	}
	discovery := &network.Discovery{Addr: cfg.DiscoveryAddr, Port: port, Logger: log, Reply: func() network.DiscoveryReply {
		return network.DiscoveryReply{ID: serverID, Name: serverName(), Version: cfg.Version}
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
		if relays != nil {
			relays.Wake()
		}
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
		link.Run(ctx)
		return nil
	})
	if certificates != nil {
		g.Go(func() error {
			certificates.Run(ctx)
			return nil
		})
		g.Go(func() error {
			relays.Run(ctx)
			return nil
		})
		// Relayed connections carry TLS for the mavio.site certificate.
		g.Go(func() error {
			cfg := &tls.Config{MinVersion: tls.VersionTLS12, GetCertificate: certificates.Certificate, NextProtos: []string{"http/1.1"}}
			return httpserver.Serve(ctx, tls.NewListener(relays.Listener(), cfg), prefix)
		})
	}
	if cfg.ClaimToken != "" {
		g.Go(func() error {
			claimOnStart(ctx, log, db, claimer, cfg.ClaimToken)
			return nil
		})
	} else if cfg.OpenBrowser {
		openSetup(ctx, log, db, port)
	}
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
	// Acquisition has a worker of its own too, so that searches and
	// imports neither wait for scans nor hold them up.
	// It also completes the requests whose titles became available.
	acqHandlers := acq.Handlers()
	maps.Copy(acqHandlers, reqs.Handlers())
	maps.Copy(acqHandlers, subs.Handlers())
	acqWorker := &library.Worker{
		Queue: db.Jobs(), Owner: fmt.Sprintf("%s:%d:acquisition", hostname, os.Getpid()), Handlers: acqHandlers, Logger: log,
	}
	// The recurring jobs are scheduled now and whenever the settings
	// change, as when RSS sync is turned on.
	if err := set.Register(ctx, func(ctx context.Context, s core.ServerSettings) error {
		if err := acq.Schedule(ctx, s.Acquisition); err != nil {
			return err
		}
		return subs.Schedule(ctx, s.Subtitles)
	}); err != nil {
		log.ErrorContext(ctx, "schedule acquisition jobs", "err", err)
	}
	if err := reqs.Schedule(ctx); err != nil {
		log.ErrorContext(ctx, "schedule request availability job", "err", err)
	}
	g.Go(func() error {
		if err := acqWorker.Run(ctx); !errors.Is(err, context.Canceled) {
			return err
		}
		return nil
	})
	if worker, scanner := newLibraryWorker(ctx, log, db, cfg.FFprobe, cfg.FFmpeg, imageServer, refresher, plugs.SegmentProviders, plugs.Resolvers, quietGate, volumeLedger, devices, growthPolicy, subs.Probed); worker != nil {
		// Seasons and episodes scans find in wanted series are monitored
		// as the series says, and episodes whose files go missing are
		// unmonitored when the settings say to.
		scanner.NewSeasonMonitored, scanner.OnMissing = acq.NewSeasonMonitored, acq.UnmonitorMissing
		scanner.SubtitleFolder = subtitleFolder(set, metadataDir)
		// Subtitle files a scan finds or loses change what is wanted.
		scanner.OnSidecars = subs.Sidecars
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

// claimOnStart claims an unclaimed server with the claim code of its
// configuration, logging failures, as nobody waits for the answer.
func claimOnStart(ctx context.Context, log *slog.Logger, db core.Store, claimer *cloud.Claimer, code string) {
	settings, err := db.Settings().Accounts(ctx)
	if err != nil {
		log.ErrorContext(ctx, "read the account settings", "err", err)
		return
	}
	if !cloud.Claimable(settings) {
		return
	}
	if _, err := claimer.Claim(ctx, code); err != nil {
		log.ErrorContext(ctx, "claim the server with claim-token", "err", err)
	}
}

// openSetup opens the web UI's setup in the default browser of a desktop
// when the server needs setting up, as a new server does.
func openSetup(ctx context.Context, log *slog.Logger, db core.Store, port int) {
	settings, err := db.Settings().Accounts(ctx)
	if err != nil || !cloud.Claimable(settings) || port == 0 {
		return
	}
	url := fmt.Sprintf("http://localhost:%d/setup", port)
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.CommandContext(ctx, "open", url)
	case "windows":
		cmd = exec.CommandContext(ctx, "rundll32", "url.dll,FileProtocolHandler", url)
	default:
		// Only a graphical session has a browser to open.
		if os.Getenv("DISPLAY") == "" && os.Getenv("WAYLAND_DISPLAY") == "" {
			return
		}
		cmd = exec.CommandContext(ctx, "xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		log.DebugContext(ctx, "open the setup in a browser", "err", err)
		return
	}
	log.InfoContext(ctx, "opened the setup in the browser", "url", url)
	go func() { _ = cmd.Wait() }()
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

// newAcquisition returns the movie and series acquisition runtime: the
// settings' acquisition settings, the plugins' indexers, download clients
// and scene mapping providers, and ffprobe to verify imports with when
// there is one.
func newAcquisition(log *slog.Logger, db core.Store, set *settings.Manager, plugs *plugins.Manager, clients func() []downloads.Provider, refresher *library.Refresher, ffprobe string,
	quietGate *storage.QuietGate, volumeLedger *storage.VolumeLedger, devices *storage.Detector,
) *acquisition.Runtime {
	rt := &acquisition.Runtime{
		Store: db, Settings: func() core.AcquisitionSettings { return set.Get().Acquisition },
		Indexers: plugs.IndexerProviders, Clients: clients, Identifier: refresher, Storages: refresher.Storages,
		VolumeLedger: volumeLedger, QuietGate: quietGate, Devices: devices, Logger: log,
		SceneProviders: func() []acquisition.SceneProvider {
			var out []acquisition.SceneProvider
			for _, p := range plugs.SceneMappingProviders() {
				out = append(out, acquisition.SceneProvider{ID: p.ID, Client: p.Client})
			}
			return out
		},
	}
	// Seasons and episodes refreshes add to wanted series are monitored as
	// the series says.
	refresher.NewSeasonMonitored = rt.NewSeasonMonitored
	if path, err := exec.LookPath(ffprobe); err == nil {
		rt.Prober = providers.FFprobe{Prober: &probe.Prober{FFprobe: path}}
	}
	return rt
}

// newLibraryWorker returns the worker running scans, probes, keyframe
// extractions, metadata refreshes and image placeholders, with a scan of
// every library queued, and its scanner, or nil when there is no ffprobe
// to probe media with.
func newLibraryWorker(ctx context.Context, log *slog.Logger, db core.Store, ffprobe, ffmpeg string, analyzer library.ImageAnalyzer,
	refresher *library.Refresher, segments func() []library.SegmentProvider, resolvers func() []library.FolderResolver,
	quietGate *storage.QuietGate, volumeLedger *storage.VolumeLedger, devices *storage.Detector,
	growthPolicy *storage.GrowthPolicy, probed func(ctx context.Context, lib core.Library, it core.Item),
) (*library.Worker, *library.Scanner) {
	path, err := exec.LookPath(ffprobe)
	if err != nil {
		log.ErrorContext(ctx, "ffprobe unavailable; libraries are not scanned", "ffprobe", ffprobe, "err", err)
		return nil, nil
	}
	jobs := &library.Jobs{
		Store: db, Storages: refresher.Storages, Probed: probed,
		Scanner: &library.Scanner{
			Store: db, Resolver: library.NewResolver(), Logger: log, Storages: refresher.Storages,
			Resolvers:    resolvers,
			QuietGate:    quietGate,
			VolumeLedger: volumeLedger,
			Devices:      devices,
			Classifier:   storage.NewClassifier(devices),
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
		jobs.Loudness = providers.Loudness{Maker: maker}
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
func newPlaybacks(ctx context.Context, log *slog.Logger, db core.Store, hub *events.Hub, activityLog *activity.Log, plugs *plugins.Manager, cfg Config, quietGate *storage.QuietGate, keeper *storage.Keeper, classes *storage.PathClasses, storages *library.Storages) (*playback.Manager, string) {
	pc := playback.Config{
		Store: db, Dir: cfg.TranscodeDir, OnChange: hub.SessionsChanged, Record: activityLog.Record, Device: plugs.IsDevice,
		QuietGate: quietGate, Keeper: keeper, Class: classes.Class, Storages: storages, Logger: log,
	}
	if dir, err := filepath.Abs(cfg.MetadataDir); err == nil && cfg.MetadataDir != "" {
		pc.MetadataDir = dir
	}
	if path, err := exec.LookPath(cfg.FFprobe); err == nil {
		remote := &library.RemoteSources{
			Source: plugs.MediaSourceProviders, Prober: providers.FFprobe{Prober: &probe.Prober{FFprobe: path}}, Logger: log,
		}
		pc.RemoteSources = remote.For
	}
	v, err := pc.UseFFmpeg(ctx, cfg.FFmpeg, cfg.FFprobe)
	if err != nil {
		log.WarnContext(ctx, "ffmpeg unavailable; media plays directly only", "ffmpeg", cfg.FFmpeg, "err", err)
	} else {
		log.InfoContext(ctx, "ffmpeg found", "version", v)
	}
	return playback.NewManager(pc), v
}

// ffmpegPath returns the path of ffmpeg, "" when it is not found.
func ffmpegPath(ffmpeg string) string {
	if ffmpeg == "" {
		return ""
	}
	p, err := exec.LookPath(ffmpeg)
	if err != nil {
		return ""
	}
	return p
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

// subtitleFolder gives where the settings save subtitle files.
func subtitleFolder(set *settings.Manager, metadataDir string) func() library.SubtitleFolder {
	return func() library.SubtitleFolder {
		s := set.Get().Subtitles
		return library.SubtitleFolder{Kind: s.Folder, Relative: s.CustomFolder, MetadataDir: metadataDir}
	}
}

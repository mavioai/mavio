// Package httpserver assembles the HTTP handler tree and runs the server.
package httpserver

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"connectrpc.com/connect"
	"connectrpc.com/grpcreflect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/apidocs"
	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/backup"
	"github.com/mavioai/mavio/apps/server/internal/devplayer"
	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/apps/server/internal/images"
	"github.com/mavioai/mavio/apps/server/internal/logs"
	"github.com/mavioai/mavio/apps/server/internal/playback"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/apps/server/internal/rpc"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1/playbackv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1/sessionv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// Options configures the handler tree.
type Options struct {
	Version string
	// Store is observed by Hub (events.Observe).
	Store core.Store
	// Hub keeps the devices' event streams.
	Hub *events.Hub
	// Database is "sqlite" or "postgres".
	Database string
	// FFmpegVersion is the version of the ffmpeg in use, if any.
	FFmpegVersion string
	// Playbacks runs playbacks; their media is served under /media/.
	Playbacks *playback.Manager
	// Images serves artwork under /images/.
	Images *images.Server
	// Plugins runs the server's plugins; nil means none.
	Plugins rpc.PluginManager
	// Refresher carries out metadata changes; nil uses one without
	// providers.
	Refresher *library.Refresher
	// Subtitles downloads subtitles; nil uses one without providers.
	Subtitles *library.Subtitles
	// Settings keeps the server settings; nil serves the defaults.
	Settings *settings.Manager
	// Accelerations lists the hardware accelerations available.
	Accelerations func() []core.HardwareAcceleration
	// Logs keeps the recent log records.
	Logs *logs.Ring
	// Activity records what happens; nil records nothing.
	Activity *activity.Log
	// Backups makes backups; nil makes none.
	Backups *backup.Manager
	// Authenticate checks the credentials of plugin users.
	Authenticate func(ctx context.Context, pluginID, name, password string) (plugins.AuthResult, error)
	// Wake is told of the media of each item a client opens; nil tells no
	// one.
	Wake func(sources []core.MediaSource)
	// Dev serves the development player at /dev/player.
	Dev bool
}

// Handler returns the root handler serving all Connect services.
func Handler(opts Options) (http.Handler, error) {
	validate, err := rpc.NewValidateInterceptor()
	if err != nil {
		return nil, err
	}
	public := append([]string{systemv1connect.SystemServiceGetHealthProcedure}, rpc.AuthPublicProcedures...)
	// Authentication runs first, so that invalid requests from strangers
	// learn nothing about the rules.
	authn := auth.NewInterceptor(opts.Store, public...).
		RequireUser(rpc.UserBoundProcedures...).
		RequireSession(rpc.SessionBoundProcedures...)
	interceptors := connect.WithInterceptors(authn, validate)

	mux := http.NewServeMux()
	mux.Handle(systemv1connect.NewSystemServiceHandler(&rpc.SystemService{
		Version: opts.Version, StartTime: time.Now(), Database: opts.Database, FFmpegVersion: opts.FFmpegVersion,
		Plugins: opts.Plugins, Hub: opts.Hub, Settings: opts.Settings, Accelerations: opts.Accelerations, Logs: opts.Logs,
		Activity: opts.Activity,
	}, interceptors))
	authService := rpc.NewAuthService(opts.Store)
	authService.Activity, authService.Authenticate = opts.Activity, opts.Authenticate
	mux.Handle(authv1connect.NewAuthServiceHandler(authService, interceptors))
	tasks := rpc.NewTaskService(opts.Store)
	if opts.Plugins != nil {
		tasks.Plugins = opts.Plugins.Tasks
	}
	mux.Handle(systemv1connect.NewTaskServiceHandler(tasks, interceptors))
	mux.Handle(systemv1connect.NewActivityServiceHandler(rpc.NewActivityService(opts.Store), interceptors))
	mux.Handle(systemv1connect.NewBackupServiceHandler(rpc.NewBackupService(opts.Backups, opts.Activity), interceptors))
	mux.Handle(systemv1connect.NewLocalizationServiceHandler(rpc.LocalizationService{}, interceptors))
	mux.Handle("GET /system/backups/{name}", backupDownload(authn, opts.Backups))
	mux.Handle(libraryv1connect.NewLibraryServiceHandler(rpc.NewLibraryService(opts.Store), interceptors))
	items := rpc.NewItemService(opts.Store)
	items.Wake = opts.Wake
	mux.Handle(libraryv1connect.NewItemServiceHandler(items, interceptors))
	mux.Handle(libraryv1connect.NewCollectionServiceHandler(rpc.NewCollectionService(opts.Store), interceptors))
	refresher, subtitles := opts.Refresher, opts.Subtitles
	if refresher == nil {
		refresher = &library.Refresher{Store: opts.Store}
	}
	if subtitles == nil {
		subtitles = &library.Subtitles{Store: opts.Store}
	}
	metadataService := rpc.NewMetadataService(opts.Store, refresher, subtitles, opts.Images.Fetch)
	metadataService.Activity = opts.Activity
	mux.Handle(libraryv1connect.NewMetadataServiceHandler(metadataService, interceptors))
	mux.Handle(libraryv1connect.NewPlaylistServiceHandler(rpc.NewPlaylistService(opts.Store), interceptors))
	users := rpc.NewUserService(opts.Store)
	users.Activity = opts.Activity
	mux.Handle(userv1connect.NewUserServiceHandler(users, interceptors))
	mux.Handle(userv1connect.NewUserDataServiceHandler(rpc.NewUserDataService(opts.Store), interceptors))
	mux.Handle(userv1connect.NewDisplayPreferencesServiceHandler(rpc.NewDisplayPreferencesService(opts.Store), interceptors))
	mux.Handle(playbackv1connect.NewPlaybackServiceHandler(rpc.NewPlaybackService(opts.Playbacks), interceptors))
	mux.Handle(sessionv1connect.NewEventServiceHandler(rpc.NewEventService(opts.Hub), interceptors))
	mux.Handle(sessionv1connect.NewSessionServiceHandler(rpc.NewSessionService(opts.Store, opts.Hub, opts.Playbacks), interceptors))
	mux.Handle(sessionv1connect.NewSyncPlayServiceHandler(rpc.NewSyncPlayService(opts.Store, opts.Hub), interceptors))
	mux.Handle("GET /media/", opts.Playbacks.Handler())
	mux.Handle("GET /images/", opts.Images.Handler())
	if opts.Dev {
		mux.Handle("GET /dev/", devplayer.Handler())
	}

	// The API describes itself to anyone: Connect reflection for tools such
	// as buf curl and grpcurl, and the OpenAPI document and its reference.
	reflector := grpcreflect.NewStaticReflector(services...)
	mux.Handle(grpcreflect.NewHandlerV1(reflector))
	mux.Handle(grpcreflect.NewHandlerV1Alpha(reflector))
	docs, err := apidocs.Handler(opts.Version, public)
	if err != nil {
		return nil, err
	}
	mux.Handle("GET /spec.json", docs)
	mux.Handle("GET /{$}", docs)
	return mux, nil
}

// connectHTTPStatus maps an authentication error to its HTTP status.
func connectHTTPStatus(err error) int {
	switch connect.CodeOf(err) {
	case connect.CodeUnauthenticated:
		return http.StatusUnauthorized
	case connect.CodePermissionDenied:
		return http.StatusForbidden
	case connect.CodeFailedPrecondition:
		return http.StatusPreconditionFailed
	default:
		return http.StatusInternalServerError
	}
}

// services are the Connect services the server serves.
var services = []string{
	authv1connect.AuthServiceName,
	libraryv1connect.LibraryServiceName, libraryv1connect.ItemServiceName, libraryv1connect.CollectionServiceName,
	libraryv1connect.MetadataServiceName, libraryv1connect.PlaylistServiceName,
	playbackv1connect.PlaybackServiceName,
	sessionv1connect.EventServiceName, sessionv1connect.SessionServiceName, sessionv1connect.SyncPlayServiceName,
	systemv1connect.SystemServiceName, systemv1connect.TaskServiceName, systemv1connect.ActivityServiceName,
	systemv1connect.BackupServiceName, systemv1connect.LocalizationServiceName,
	userv1connect.UserServiceName, userv1connect.UserDataServiceName, userv1connect.DisplayPreferencesServiceName,
}

// Serve serves h on ln until ctx is canceled, then shuts down gracefully.
func Serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Handler:           h,
		Protocols:         &protocols,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	slog.InfoContext(ctx, "listening", "addr", ln.Addr().String())

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}

	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// backupDownload serves a backup file to administrators, who send their
// bearer token as for the API, and to plugins with BackupService in their
// scopes.
func backupDownload(authn *auth.Interceptor, backups *backup.Manager) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := authn.ResolveRequest(r, "/"+systemv1connect.BackupServiceName+"/Download")
		if err != nil {
			http.Error(w, err.Error(), connectHTTPStatus(err))
			return
		}
		if !p.User.Admin {
			http.Error(w, "administrators only", http.StatusForbidden)
			return
		}
		if backups == nil {
			http.NotFound(w, r)
			return
		}
		f, err := backups.Open(r.PathValue("name"))
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.Error(w, "backup unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="`+info.Name()+`"`)
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)
	})
}

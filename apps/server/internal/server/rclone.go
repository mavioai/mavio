package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/apps/server/internal/rpc"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/rclone"
)

// rcloneRuntime is the rcd the server reaches for libraries on rclone
// remotes, and the proxy ffmpeg reads their media through.
type rcloneRuntime struct {
	Client  *rclone.Client
	Remotes *rclone.Remotes
	// Managed is set when the server runs the rcd itself.
	Managed bool
	proxy   *rclone.Proxy
	daemon  *rclone.Daemon
	blocks  *rclone.BlockCache
}

// startRclone starts the rcd cfg asks for, or reaches the one at
// cfg.RcloneURL. Without rclone the server serves no remotes, which nil
// tells, with the reason; an rcd that does not start, or runs another
// release than rclone.Version, is logged and leaves the server without
// them too.
// class gives the storage class of library paths, which decides whether
// the files of a folder are cached.
func startRclone(ctx context.Context, log *slog.Logger, cfg Config, jobs core.StorageJobRepository, class func(context.Context, string) core.StorageClass) (*rcloneRuntime, string) {
	rt := &rcloneRuntime{}
	switch {
	case cfg.RcloneURL != "":
		ep, err := externalEndpoint(cfg)
		if err != nil {
			log.ErrorContext(ctx, "libraries on rclone remotes are unavailable", "err", err)
			return nil, err.Error()
		}
		if rt.Client, err = rclone.NewClient(ep, nil); err != nil {
			log.ErrorContext(ctx, "libraries on rclone remotes are unavailable", "err", err)
			return nil, err.Error()
		}
		// An rcd that does not answer yet is checked when it first does.
		rt.Client.Require(rclone.Version)
		cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		err = rt.Client.CheckVersion(cctx)
		cancel()
		if errors.Is(err, rclone.ErrVersion) {
			log.ErrorContext(ctx, "libraries on rclone remotes are unavailable", "err", err)
			return nil, err.Error()
		}
		if err != nil {
			log.WarnContext(ctx, "rclone does not answer yet", "url", cfg.RcloneURL, "err", err)
		}
	case cfg.Rclone != "":
		bin, err := exec.LookPath(cfg.Rclone)
		if err != nil {
			log.InfoContext(ctx, "rclone not found; libraries on rclone remotes are unavailable", "rclone", cfg.Rclone)
			return nil, "rclone not found: " + err.Error()
		}
		pass, err := rclonePassword(cfg.RcloneConfig)
		if err != nil {
			log.ErrorContext(ctx, "libraries on rclone remotes are unavailable", "err", err)
			return nil, err.Error()
		}
		d, err := rclone.StartDaemon(ctx, rclone.DaemonOptions{
			Path: bin, Config: cfg.RcloneConfig, ConfigPass: pass,
			CacheDir: filepath.Join(cfg.CacheDir, "rclone"), Logger: log,
		})
		if err != nil {
			log.ErrorContext(ctx, "libraries on rclone remotes are unavailable", "err", err)
			return nil, err.Error()
		}
		rt.daemon, rt.Client, rt.Managed = d, d.Client(), true
	default:
		return nil, "the server is set to run no rclone"
	}
	rt.proxy = rclone.NewProxy(rt.Client, log)
	if err := rt.proxy.Listen(); err != nil {
		log.ErrorContext(ctx, "rclone media proxy", "err", err)
	}
	rt.Remotes = &rclone.Remotes{Client: rt.Client, MediaURL: rt.proxy.URL, Jobs: jobs, Log: log, Class: class}
	if cfg.RcloneCacheSize > 0 {
		bc, err := rclone.NewBlockCache(rt.Client, rclone.BlockCacheOptions{
			Dir: filepath.Join(cfg.CacheDir, "rclone", "blocks"), MaxSize: cfg.RcloneCacheSize, Logger: log,
		})
		if err != nil {
			log.ErrorContext(ctx, "files on cloud storage are read uncached", "err", err)
		} else {
			rt.blocks, rt.Remotes.Blocks = bc, bc
		}
	}
	go func() {
		if err := rt.Remotes.Sweep(ctx); err != nil {
			log.WarnContext(ctx, "sweep rclone transfers", "err", err)
		}
	}()
	return rt, ""
}

// Close stops the proxy and the rcd the server runs.
func (rt *rcloneRuntime) Close() {
	if rt == nil {
		return
	}
	_ = rt.proxy.Close()
	if rt.blocks != nil {
		_ = rt.blocks.Close()
	}
	if rt.daemon != nil {
		_ = rt.daemon.Close()
	}
}

// Service returns the StorageService of the rcd, keeping the offline
// download clients in offline; down is why the server reaches none.
func (rt *rcloneRuntime) Service(offline *downloads.Offline, down string) *rpc.StorageService {
	if rt == nil {
		return &rpc.StorageService{Offline: offline, Down: down}
	}
	return &rpc.StorageService{Client: rt.Client, Managed: rt.Managed, Changed: rt.Remotes.Forget, Offline: offline}
}

// externalEndpoint checks the address of an rcd the server reaches: its
// credentials travel only over TLS, a Unix socket or the loopback
// interface.
func externalEndpoint(cfg Config) (rclone.Endpoint, error) {
	u, err := url.Parse(cfg.RcloneURL)
	if err != nil {
		return rclone.Endpoint{}, fmt.Errorf("rclone-url: %w", err)
	}
	switch u.Scheme {
	case "https", "unix":
	case "http":
		host := u.Hostname()
		if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
			return rclone.Endpoint{}, fmt.Errorf("rclone-url %q: use https for an rcd on another machine", cfg.RcloneURL)
		}
	default:
		return rclone.Endpoint{}, fmt.Errorf("rclone-url %q: want https, http on the loopback interface or unix", cfg.RcloneURL)
	}
	return rclone.Endpoint{URL: cfg.RcloneURL, User: cfg.RcloneUser, Password: cfg.RclonePass}, nil
}

// rclonePassword returns the password the server encrypts its rclone.conf
// with, kept in rclone.pass beside it and made when the server creates the
// file. A configuration the server did not create has none: rclone reads
// it as it is, or with the RCLONE_CONFIG_PASS of the environment.
func rclonePassword(config string) (string, error) {
	file := filepath.Join(filepath.Dir(config), "rclone.pass")
	data, err := os.ReadFile(file)
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	if _, err := os.Stat(config); err == nil {
		return "", nil
	}
	if err := os.MkdirAll(filepath.Dir(config), 0o700); err != nil {
		return "", err
	}
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	pass := hex.EncodeToString(b)
	if err := os.WriteFile(file, []byte(pass+"\n"), 0o600); err != nil {
		return "", err
	}
	return pass, nil
}

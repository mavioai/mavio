// Command mavio runs the Mavio media server.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/mavioai/mavio/apps/server/internal/buildinfo"
	"github.com/mavioai/mavio/apps/server/internal/server"
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
	cfg := server.Config{Version: buildinfo.Version(), Logger: slog.Default()}
	fs.StringVar(&cfg.Database, "database", "sqlite:mavio.db", "database: sqlite:<path> or postgres://…")
	fs.StringVar(&cfg.FFmpeg, "ffmpeg", "ffmpeg", "ffmpeg binary; without it media is only played directly")
	fs.StringVar(&cfg.FFprobe, "ffprobe", "ffprobe", "ffprobe binary")
	fs.StringVar(&cfg.TranscodeDir, "transcode-dir", filepath.Join(os.TempDir(), "mavio-transcodes"), "directory for transcodes")
	fs.StringVar(&cfg.CacheDir, "cache-dir", defaultCacheDir(), "directory for downloaded and resized images and compiled plugins")
	fs.StringVar(&cfg.PluginDir, "plugin-dir", "", "directory holding one folder per plugin, each with its manifest.json")
	fs.BoolVar(&cfg.Dev, "dev", false, "serve the development player at /dev/player")
	fs.StringVar(&cfg.DevLibrary, "dev-library", "", "add the Movies and Shows folders of this directory as libraries, e.g. .fixtures/dev-library")
	showVersion := fs.Bool("version", false, "print version and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *showVersion {
		fmt.Println(cfg.Version)
		return nil
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	return server.Run(ctx, cfg, ln)
}

// defaultCacheDir is the user's cache directory for Mavio, or one in the
// temporary directory.
func defaultCacheDir() string {
	if dir, err := os.UserCacheDir(); err == nil {
		return filepath.Join(dir, "mavio")
	}
	return filepath.Join(os.TempDir(), "mavio-cache")
}

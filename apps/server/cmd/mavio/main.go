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
	"syscall"

	"github.com/mavioai/mavio/apps/server/internal/buildinfo"
	"github.com/mavioai/mavio/apps/server/internal/httpserver"
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
	h, err := httpserver.Handler(httpserver.Options{Version: version, Store: db, Database: db.Dialect()})
	if err != nil {
		return err
	}

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", *addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}
	slog.InfoContext(ctx, "starting mavio", "version", version)
	return httpserver.Serve(ctx, ln, h)
}

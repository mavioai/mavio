// Command mavio runs the Mavio media server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/mavioai/mavio/apps/server/internal/buildinfo"
	"github.com/mavioai/mavio/apps/server/internal/server"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := newCommand().ExecuteContext(ctx); err != nil {
		slog.Error("mavio exited", "err", err)
		os.Exit(1)
	}
}

// newCommand returns the mavio command: it serves, and its version
// subcommand prints the version.
func newCommand() *cobra.Command {
	var restore string
	cmd := &cobra.Command{
		Use:   "mavio",
		Short: "Mavio media server",
		Long: `Mavio serves media libraries.

Everything it keeps lives in its home, ~/.mavio unless --home or
MAVIO_HOME says otherwise: config.toml, the SQLite database mavio.db, and
the plugins, metadata, backups and cache folders. Each setting below may
also be given in config.toml under its flag's name, or as a MAVIO_*
environment variable (MAVIO_CACHE_DIR for --cache-dir); flags come first,
then variables, then config.toml. Relative paths in config.toml are
relative to the home.`,
		Version:       buildinfo.Version(),
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			o, err := load(cmd.Flags())
			if err != nil {
				return err
			}
			cfg := o.Server
			cfg.Version, cfg.Logger, cfg.Restore = buildinfo.Version(), slog.Default(), restore
			var lc net.ListenConfig
			ln, err := lc.Listen(cmd.Context(), "tcp", o.Addr)
			if err != nil {
				return fmt.Errorf("listen: %w", err)
			}
			slog.InfoContext(cmd.Context(), "mavio home", "path", o.Home)
			return server.Run(cmd.Context(), cfg, ln)
		},
	}
	defineFlags(cmd.Flags())
	cmd.Flags().StringVar(&restore, "restore", "", "restore this backup into the empty database before starting")
	cmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Println(buildinfo.Version())
		},
	})
	return cmd
}

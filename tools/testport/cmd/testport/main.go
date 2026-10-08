// Command testport extracts test cases and test assets from a Jellyfin
// checkout into Mavio's testdata directories.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	"github.com/mavioai/mavio/tools/testport"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		slog.Error("testport failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("testport", flag.ContinueOnError)
	jellyfin := fs.String("jellyfin", os.Getenv("MAVIO_JELLYFIN"), "Jellyfin repository root (default $MAVIO_JELLYFIN)")
	mapping := fs.String("mapping", "", "mapping file (default <repo>/tools/testport/mapping.json)")
	only := fs.String("only", "", "comma-separated target prefixes to port, e.g. libs/naming (default: all)")
	dryRun := fs.Bool("dry-run", false, "report what would be written without writing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *jellyfin == "" {
		return errors.New("-jellyfin or $MAVIO_JELLYFIN is required")
	}

	root, err := repoRoot()
	if err != nil {
		return err
	}
	if *mapping == "" {
		*mapping = filepath.Join(root, "tools", "testport", "mapping.json")
	}
	m, err := testport.LoadMapping(*mapping)
	if err != nil {
		return err
	}
	var prefixes []string
	if *only != "" {
		prefixes = strings.Split(*only, ",")
	}

	sum, err := testport.Port(ctx, testport.Options{Jellyfin: *jellyfin, Mavio: root, Mapping: m, Only: prefixes, DryRun: *dryRun})
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "testport done", "dry_run", *dryRun, "case_files", sum.Files, "theories", sum.Theories,
		"cases", sum.Cases, "facts", sum.Facts, "unsupported", sum.Unsupported, "assets", sum.Assets, "constants", sum.Constants)
	return nil
}

// repoRoot returns the directory containing go.work above the working
// directory.
func repoRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for dir := wd; ; dir = filepath.Dir(dir) {
		if _, err := os.Stat(filepath.Join(dir, "go.work")); err == nil {
			return dir, nil
		}
		if filepath.Dir(dir) == dir {
			return "", errors.New("no go.work found above " + wd)
		}
	}
}

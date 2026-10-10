// Command testport extracts test cases and test assets from checkouts of the
// source repositories in the mapping file (Jellyfin, Sonarr, Radarr, …) into
// Mavio's testdata directories. A source's checkout is given as -checkout
// name=dir or $MAVIO_<NAME> (e.g. $MAVIO_JELLYFIN); sources without one are
// skipped.
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
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
	checkouts := map[string]string{}
	fs.Func("checkout", "source checkout as name=dir, repeatable (default $MAVIO_<NAME>)", func(v string) error {
		name, dir, ok := strings.Cut(v, "=")
		if !ok || name == "" || dir == "" {
			return fmt.Errorf("want name=dir, got %q", v)
		}
		checkouts[strings.ToLower(name)] = dir
		return nil
	})
	mapping := fs.String("mapping", "", "mapping file (default <repo>/tools/testport/mapping.json)")
	only := fs.String("only", "", "comma-separated target prefixes to port, e.g. libs/naming (default: all)")
	dryRun := fs.Bool("dry-run", false, "report what would be written without writing")
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := repoRoot()
	if err != nil {
		return err
	}
	if *mapping == "" {
		*mapping = filepath.Join(root, "tools", "testport", "mapping.json")
	}
	ms, err := testport.LoadMappings(*mapping)
	if err != nil {
		return err
	}
	var prefixes []string
	if *only != "" {
		prefixes = strings.Split(*only, ",")
	}

	ported := 0
	for _, m := range ms.Sources {
		dir := cmp.Or(checkouts[strings.ToLower(m.Name)], os.Getenv("MAVIO_"+strings.ToUpper(m.Name)))
		if dir == "" {
			slog.InfoContext(ctx, "no checkout; skipping source", "source", m.Name)
			continue
		}
		sum, err := testport.Port(ctx, testport.Options{Checkout: dir, Mavio: root, Mapping: m, Only: prefixes, DryRun: *dryRun})
		if err != nil {
			return fmt.Errorf("%s: %w", m.Name, err)
		}
		ported++
		slog.InfoContext(ctx, "source ported", "source", m.Name, "dry_run", *dryRun, "case_files", sum.Files, "theories", sum.Theories,
			"cases", sum.Cases, "facts", sum.Facts, "unsupported", sum.Unsupported, "assets", sum.Assets, "constants", sum.Constants)
	}
	if ported == 0 {
		return errors.New("no source checkout given; use -checkout name=dir or $MAVIO_<NAME>")
	}
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

// Command fixtures generates the test media catalog with ffmpeg.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"strings"

	"github.com/mavioai/mavio/tools/fixtures"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	if err := run(ctx, os.Args[1:]); err != nil {
		slog.Error("fixtures failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("fixtures", flag.ContinueOnError)
	out := fs.String("out", "", "output directory (default: $"+fixtures.EnvDir+" or <repo>/.fixtures)")
	ffmpeg := fs.String("ffmpeg", "ffmpeg", "ffmpeg executable")
	only := fs.String("only", "", "comma-separated fixture names to generate (default: all)")
	force := fs.Bool("force", false, "regenerate fixtures even when up to date")
	list := fs.Bool("list", false, "list the catalog and exit")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *list {
		for _, s := range fixtures.Catalog() {
			fmt.Printf("%-28s %s\n", s.Name, s.Description)
		}
		return nil
	}

	dir := *out
	if dir == "" {
		var err error
		if dir, err = fixtures.Dir(); err != nil {
			return err
		}
	}
	var names []string
	if *only != "" {
		names = strings.Split(*only, ",")
	}

	res, err := fixtures.Generate(ctx, fixtures.Options{Dir: dir, FFmpeg: *ffmpeg, Only: names, Force: *force})
	if err != nil {
		return err
	}
	slog.InfoContext(ctx, "fixtures done", "dir", dir,
		"generated", len(res.Generated), "up_to_date", len(res.UpToDate), "skipped", len(res.Skipped))
	return nil
}

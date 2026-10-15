// Command plugindist packages the first-party plugins and writes their
// catalog.
//
// With -out it writes the packages and the catalog, whose URLs are
// relative to it, into one folder, as a release publishes them. Without
// it, each plugin's packages go into its dist folder and the catalog into
// plugins/catalog.json, with file URLs, for a development server.
//
// Usage: plugindist [-plugins dir] [-out dir] [-platforms all|os/arch,...]
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/mavioai/mavio/tools/plugindist"
)

func main() {
	plugins := flag.String("plugins", filepath.Join("..", "..", "plugins"), "the folder of the first-party plugins")
	out := flag.String("out", "", "folder to write the packages and their catalog.json into; empty packages each plugin into its dist folder and writes catalog.json into the plugins' folder")
	platforms := flag.String("platforms", "", `comma-separated os/arch the process plugins are built for, or "all"; by default all with -out and this platform without`)
	flag.Parse()

	o := plugindist.Options{Plugins: *plugins, Out: *out, Now: time.Now().UTC().Truncate(time.Second)}
	switch {
	case *platforms == "all", *platforms == "" && *out != "":
		o.Platforms = plugindist.Platforms
	case *platforms == "":
		o.Platforms = []string{runtime.GOOS + "/" + runtime.GOARCH}
	default:
		o.Platforms = strings.Split(*platforms, ",")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	file, err := plugindist.Dist(ctx, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "plugindist:", err)
		os.Exit(1)
	}
	fmt.Println(file)
}

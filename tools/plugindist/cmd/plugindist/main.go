// Command plugindist packages the first-party plugins and writes their
// catalog.
//
// With -out it writes the packages and the catalog, whose URLs are
// relative to it, into one folder, as a release publishes them. Without
// it, each plugin's packages go into its dist folder and the catalog into
// plugins/catalog.json, with file URLs, for a development server.
//
// With -python it builds a Python process plugin into a folder a server
// starts, as the container image ships it.
//
// Usage: plugindist [-plugins dir] [-out dir] [-platforms all|os/arch,...]
//
//	plugindist -python dir [-platform os/arch] [-folder dir]
package main

import (
	"cmp"
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
	python := flag.String("python", "", "a Python process plugin to build into -folder, instead of packaging the plugins")
	platform := flag.String("platform", runtime.GOOS+"/"+runtime.GOARCH, "the os/arch -python builds for")
	folder := flag.String("folder", "", "the folder -python builds into; by default dist/plugin of the plugin's")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *python != "" {
		if !plugindist.PythonPackaged(*platform) && !flagSet("platform") {
			// Building for this platform, as Nx does, skips one that has
			// no packages.
			fmt.Fprintf(os.Stderr, "plugindist: no Python plugin packages for %s; skipped\n", *platform)
			return
		}
		goos, arch, _ := strings.Cut(*platform, "/")
		out := cmp.Or(*folder, filepath.Join(*python, "dist", "plugin"))
		if err := plugindist.PythonFolder(ctx, *python, goos, arch, out); err != nil {
			fmt.Fprintln(os.Stderr, "plugindist:", err)
			os.Exit(1)
		}
		fmt.Println(out)
		return
	}

	o := plugindist.Options{Plugins: *plugins, Out: *out, Now: time.Now().UTC().Truncate(time.Second)}
	switch {
	case *platforms == "all", *platforms == "" && *out != "":
		o.Platforms = plugindist.Platforms
	case *platforms == "":
		o.Platforms = []string{runtime.GOOS + "/" + runtime.GOARCH}
	default:
		o.Platforms = strings.Split(*platforms, ",")
	}
	file, err := plugindist.Dist(ctx, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "plugindist:", err)
		os.Exit(1)
	}
	fmt.Println(file)
}

// flagSet reports whether the flag name was given.
func flagSet(name string) bool {
	set := false
	flag.Visit(func(f *flag.Flag) { set = set || f.Name == name })
	return set
}

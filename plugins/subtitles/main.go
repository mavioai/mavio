// Command subtitles is Mavio's subtitles plugin, a process plugin porting
// Bazarr's subtitle providers: it searches subtitle sites for subtitles of
// movies and episodes and downloads them, throttling the sites that fail,
// as the SUBTITLE_PROVIDER capability.
package main

import (
	"context"
	"log/slog"
	"os"
	_ "time/tzdata" // The sites' daily limits reset at midnight in their zones.

	"github.com/mavioai/mavio/libs/plugin/guest"
	"github.com/mavioai/mavio/libs/plugin/guest/process"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ID is the plugin's ID, as its manifest gives it.
const ID = "org.mavio.subtitles"

// Version is the plugin's version; builds set it with
// -ldflags "-X main.Version=…".
var Version = "0.1.0"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := newPlugin(log, Version, guest.DataDir(), nil)
	if err != nil {
		log.Error("start", "err", err)
		os.Exit(1)
	}
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewSubtitleProviderServiceHandler(p))
	if err := process.Serve(ctx, ID); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

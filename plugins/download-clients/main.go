// Command download-clients is Mavio's download clients plugin, a process
// plugin porting the download clients of Radarr and Sonarr: it sends
// releases to them and follows their downloads, as the
// DOWNLOAD_CLIENT_PROVIDER capability.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/mavioai/mavio/libs/plugin/guest"
	"github.com/mavioai/mavio/libs/plugin/guest/process"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ID is the plugin's ID, as its manifest gives it.
const ID = "dev.mavio.download-clients"

// Version is the plugin's version; builds set it with
// -ldflags "-X main.Version=…".
var Version = "0.1.0"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := newPlugin(log, Version, guest.DataDir(), newHTTPClient())
	if err != nil {
		log.Error("start", "err", err)
		os.Exit(1)
	}
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewDownloadClientProviderServiceHandler(p))
	if err := process.Serve(ctx, ID); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

// newHTTPClient returns the client the download clients are reached
// with, as Radarr's: requests time out after 100 seconds.
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 100 * time.Second}
}

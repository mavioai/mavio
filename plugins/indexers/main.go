// Command indexers is Mavio's indexers plugin, a process plugin porting
// Prowlarr: it searches Newznab and Torznab indexers, the sites of
// Cardigann definitions and the sites Prowlarr implements in code, as the
// INDEXER_PROVIDER capability.
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
const ID = "org.mavio.indexers"

// Version is the plugin's version; builds set it with
// -ldflags "-X main.Version=…".
var Version = "0.1.0"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := newPlugin(log, Version, guest.DataDir(), guest.HostOS())
	if err != nil {
		log.Error("start", "err", err)
		os.Exit(1)
	}
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewIndexerProviderServiceHandler(p))
	err = process.Serve(ctx, ID)
	p.halt()
	if err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

// newHTTPClient returns the client fetching definitions.
func newHTTPClient() *http.Client {
	return &http.Client{Timeout: 2 * time.Minute}
}

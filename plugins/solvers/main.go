// Command solvers is Mavio's challenge solvers plugin, a process plugin
// solving the challenges of sites behind Cloudflare or DDoS-Guard with
// FlareSolverr, run as a container dependency, as the CHALLENGE_SOLVER
// capability: other plugins reach it through HostService.SolveChallenge.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"

	"github.com/mavioai/mavio/libs/plugin/guest"
	"github.com/mavioai/mavio/libs/plugin/guest/process"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// ID is the plugin's ID, as its manifest gives it.
const ID = "dev.mavio.solvers"

// Version is the plugin's version; builds set it with
// -ldflags "-X main.Version=…".
var Version = "0.1.0"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p := newPlugin(ctx, log, Version, &http.Client{}, guest.HostClient)
	defer p.close()
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewChallengeSolverServiceHandler(p))
	if err := process.Serve(ctx, ID); err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

// Command dns serves the zone mavio.site from its name servers
// (docs/mavio-dev.md §4.1), over UDP and TCP.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/miekg/dns"

	"github.com/mavioai/mavio/apps/dns/internal/zone"
)

func main() {
	config := flag.String("config", "/etc/mavio-dns.json", "the zone's JSON configuration")
	addr := flag.String("addr", ":53", "address listened on, over UDP and TCP")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, *config, *addr); err != nil {
		slog.Error("dns exited", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, config, addr string) error {
	c, err := zone.Load(config)
	if err != nil {
		return err
	}
	h := zone.New(c)
	servers := []*dns.Server{
		{Addr: addr, Net: "udp", Handler: h, ReusePort: true},
		{Addr: addr, Net: "tcp", Handler: h, ReusePort: true},
	}
	errc := make(chan error, len(servers))
	for _, s := range servers {
		go func() { errc <- s.ListenAndServe() }()
	}
	slog.InfoContext(ctx, "serving the zone", "zone", c.Zone, "addr", addr)
	select {
	case err = <-errc:
	case <-ctx.Done():
	}
	for _, s := range servers {
		_ = s.ShutdownContext(context.WithoutCancel(ctx))
	}
	if errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

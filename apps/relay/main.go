// Command relay splices clients' connections to servers in mavio.dev mode
// that cannot be reached directly (docs/mavio-dev.md §5).
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mavioai/mavio/apps/relay/internal/relay"
)

func main() {
	var o options
	flag.StringVar(&o.region, "region", "", "the relay's region, such as eu")
	flag.StringVar(&o.zone, "zone", "mavio.site", "the domain of servers' names")
	flag.StringVar(&o.url, "url", "", "the base URL servers reach the relay at, such as https://eu.relay.mavio.dev")
	flag.StringVar(&o.secretFile, "ticket-secret-file", "", "file holding the ticket secret shared with mavio.dev")
	flag.StringVar(&o.addr, "addr", ":443", "address clients connect to")
	flag.StringVar(&o.serverAddr, "server-addr", ":8443", "address servers connect to")
	flag.StringVar(&o.cert, "cert", "", "certificate of the servers' address; plain HTTP without one")
	flag.StringVar(&o.key, "key", "", "private key of the certificate")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, o); err != nil {
		slog.Error("relay exited", "err", err)
		os.Exit(1)
	}
}

type options struct {
	region, zone, url, secretFile, addr, serverAddr, cert, key string
}

func run(ctx context.Context, o options) error {
	if o.region == "" || o.url == "" || o.secretFile == "" {
		return errors.New("--region, --url and --ticket-secret-file are required")
	}
	secret, err := os.ReadFile(o.secretFile)
	if err != nil {
		return fmt.Errorf("ticket secret: %w", err)
	}
	secret = bytes.TrimSpace(secret)
	if len(secret) < 32 {
		return errors.New("the ticket secret must be at least 32 bytes")
	}
	r := relay.New(o.region, o.zone, o.url, secret, slog.Default())

	var lc net.ListenConfig
	clients, err := lc.Listen(ctx, "tcp", o.addr)
	if err != nil {
		return err
	}
	servers, err := lc.Listen(ctx, "tcp", o.serverAddr)
	if err != nil {
		_ = clients.Close()
		return err
	}
	// Upgrades need HTTP/1.1.
	var http1 http.Protocols
	http1.SetHTTP1(true)
	hs := &http.Server{Handler: r.Handler(), ReadHeaderTimeout: 10 * time.Second, Protocols: &http1}
	errc := make(chan error, 2)
	go func() { errc <- r.ServeClients(ctx, clients) }()
	go func() {
		if o.cert != "" {
			errc <- hs.ServeTLS(servers, o.cert, o.key)
		} else {
			errc <- hs.Serve(servers)
		}
	}()
	slog.InfoContext(ctx, "relaying", "region", o.region, "addr", o.addr, "server-addr", o.serverAddr)
	select {
	case err = <-errc:
	case <-ctx.Done():
	}
	_ = clients.Close()
	_ = hs.Close()
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

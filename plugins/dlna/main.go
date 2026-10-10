// Command dlna is Mavio's DLNA plugin, a process plugin: a DLNA media
// server announcing itself over SSDP and serving a user's libraries
// through ContentDirectory, and Play To, which finds DLNA renderers and
// lists them as the server's remote devices.
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
const ID = "org.mavio.dlna"

// Version is the plugin's version; builds set it with
// -ldflags "-X main.Version=…".
var Version = "0.1.0"

func main() {
	log := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	p, err := newPlugin(ctx, log, Version)
	if err != nil {
		log.Error("start", "err", err)
		os.Exit(1)
	}
	guest.Handle(pluginv1connect.NewPluginServiceHandler(p))
	guest.Handle(pluginv1connect.NewEventConsumerServiceHandler(p))
	guest.Handle(pluginv1connect.NewDeviceControllerServiceHandler(p))
	guest.HandleHTTP(p.routes())
	err = process.Serve(ctx, ID)
	p.halt()
	if err != nil {
		log.Error("serve", "err", err)
		os.Exit(1)
	}
}

// headers adds headers to host API requests, those with values.
type headers map[string]string

func (h headers) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	for k, v := range h {
		if v != "" {
			r.Header.Set(k, v)
		}
	}
	return guest.HostClient().Transport.RoundTrip(r)
}

// hostClient returns a client of the host API acting as a user and, when
// device is set, one of the plugin's devices. It follows no redirects:
// those of the media routes lead away from the host.
func hostClient(user, device string) *http.Client {
	return &http.Client{
		Transport:     headers{guest.UserHeader: user, guest.DeviceHeader: device},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

// Package process is the guest entry point for child-process plugins: it
// serves the registered handlers over the Unix domain socket the host
// provides, authenticates the host's token, prints the handshake line and
// exits after the Shutdown RPC or a termination signal.
//
//	func main() {
//		if err := process.Serve(context.Background(), "org.example.my-plugin"); err != nil {
//			log.Fatal(err)
//		}
//	}
package process

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/mavioai/mavio/libs/plugin/guest"
	"github.com/mavioai/mavio/libs/plugin/internal/abi"
	"github.com/mavioai/mavio/libs/plugin/internal/proc"
)

func init() {
	socket, token := os.Getenv(proc.EnvHostSocket), os.Getenv(proc.EnvToken)
	if socket == "" {
		return
	}
	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	t := &http.Transport{
		Protocols: &protocols,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}
	guest.SetHostClient(&http.Client{Transport: roundTripper(func(req *http.Request) (*http.Response, error) {
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", proc.AuthScheme+token)
		return t.RoundTrip(req)
	})})
}

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// Serve runs the plugin until the host shuts it down. pluginID is reported
// in the handshake.
func Serve(ctx context.Context, pluginID string) error {
	socket, token := os.Getenv(proc.EnvSocket), os.Getenv(proc.EnvToken)
	if socket == "" || token == "" {
		return fmt.Errorf("%s and %s must be set; plugins are started by the Mavio host", proc.EnvSocket, proc.EnvToken)
	}
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "unix", socket)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", socket, err)
	}
	defer ln.Close()

	shutdown := make(chan struct{}, 1)
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	srv := &http.Server{
		Handler:           authenticate(token, shutdownAfter(guest.Handler(), shutdown)),
		Protocols:         &protocols,
		ReadHeaderTimeout: 10 * time.Second,
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	if err := json.NewEncoder(os.Stdout).Encode(proc.Handshake{Protocol: proc.ProtocolVersion, PluginID: pluginID}); err != nil {
		return err
	}

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	case <-shutdown:
	}
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	if err := <-errc; !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// authenticate requires the host's token: in the Authorization header of
// Connect calls, or in proc.TokenHeader of requests to HTTP routes, whose
// handler sees no token.
func authenticate(token string, next http.Handler) http.Handler {
	want := []byte(proc.AuthScheme + token)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, abi.HTTPPrefix+"/"):
			if subtle.ConstantTimeCompare([]byte(r.Header.Get(proc.TokenHeader)), []byte(token)) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			r.Header.Del(proc.TokenHeader)
		case subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1:
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// shutdownAfter closes done after a successful Shutdown RPC has been served.
func shutdownAfter(next http.Handler, done chan<- struct{}) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if r.URL.Path == proc.ShutdownPath {
			select {
			case done <- struct{}{}:
			default:
			}
		}
	})
}

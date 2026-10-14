package process

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/mavioai/mavio/libs/plugin/internal/proc"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// envTCPPlugin makes the test binary a plugin serving on loopback TCP, as
// plugins whose runtime has no Unix sockets on Windows do; its value is
// the address it reports.
const envTCPPlugin = "MAVIO_TEST_TCP_PLUGIN"

func TestMain(m *testing.M) {
	if addr, ok := os.LookupEnv(envTCPPlugin); ok {
		if err := serveTCP(addr); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// tcpPlugin reports whether the host API reached it on the loopback
// address the host gives on Windows.
type tcpPlugin struct {
	pluginv1connect.UnimplementedPluginServiceHandler
	done chan struct{}
}

func (p *tcpPlugin) Health(context.Context, *pluginv1.HealthRequest) (*pluginv1.HealthResponse, error) {
	addr := os.Getenv(proc.EnvHostAddress)
	return pluginv1.HealthResponse_builder{Healthy: proto.Bool(true), Message: proto.String("host api " + addr)}.Build(), nil
}

func (p *tcpPlugin) Shutdown(context.Context, *pluginv1.ShutdownRequest) (*pluginv1.ShutdownResponse, error) {
	close(p.done)
	return &pluginv1.ShutdownResponse{}, nil
}

// serveTCP serves the plugin on loopback TCP and reports addr, or the
// listener's address when addr is empty.
func serveTCP(addr string) error {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return err
	}
	p := &tcpPlugin{done: make(chan struct{})}
	path, h := pluginv1connect.NewPluginServiceHandler(p)
	token := proc.AuthScheme + os.Getenv(proc.EnvToken)
	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	srv := &http.Server{Protocols: &protocols, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != token || !strings.HasPrefix(r.URL.Path, path) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		h.ServeHTTP(w, r)
	})}
	go func() { _ = srv.Serve(ln) }()
	if addr == "" {
		addr = ln.Addr().String()
	}
	if err := json.NewEncoder(os.Stdout).Encode(proc.Handshake{Protocol: proc.ProtocolVersion, PluginID: "org.mavio.tcp", Address: addr}); err != nil {
		return err
	}
	<-p.done
	return nil
}

func start(t *testing.T, addr string) (*Plugin, error) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	m := pluginv1.Manifest_builder{Id: proto.String("org.mavio.tcp"), Runtime: pluginv1.Runtime_RUNTIME_PROCESS.Enum()}.Build()
	return Start(t.Context(), exe, m, Options{
		Env:     []string{envTCPPlugin + "=" + addr},
		HostAPI: http.NotFoundHandler(),
	})
}

func TestLoopbackAddress(t *testing.T) {
	p, err := start(t, "")
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close(context.WithoutCancel(t.Context()))
	h, err := p.Lifecycle().Health(t.Context(), &pluginv1.HealthRequest{})
	if err != nil {
		t.Fatalf("Health over loopback TCP: %v", err)
	}
	// The host API is on a loopback address too on Windows only.
	hostAddr := strings.TrimPrefix(h.GetMessage(), "host api ")
	if (hostAddr != "") != (runtime.GOOS == "windows") {
		t.Errorf("host API address = %q on %s", hostAddr, runtime.GOOS)
	}
	if hostAddr != "" {
		if err := loopback(hostAddr); err != nil {
			t.Error(err)
		}
	}
}

func TestLoopbackAddressRequired(t *testing.T) {
	for _, addr := range []string{"192.0.2.1:80", "example.com:80", "127.0.0.1:0", "127.0.0.1"} {
		if p, err := start(t, addr); err == nil {
			_ = p.Close(context.WithoutCancel(t.Context()))
			t.Errorf("Start with a plugin at %s: err = nil, want one", addr)
		}
	}
}

// Package process runs plugins as supervised child processes reached over a
// Unix domain socket.
//
// The host creates a private directory (mode 0700) holding the socket and
// passes its path and a one-time token in the environment; the plugin listens,
// prints a handshake line on stdout and requires the token on every request.
// The supervisor forwards the plugin's output to the logger, checks its
// health, restarts it with exponential backoff when it exits or stops
// responding, and shuts it down gracefully on Close.
package process

import (
	"bufio"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/plugin/internal/clients"
	"github.com/mavioai/mavio/libs/plugin/internal/proc"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Options configures the process runtime.
type Options struct {
	// StartTimeout bounds the wait for the handshake; zero means 10 s.
	StartTimeout time.Duration
	// HealthInterval is the period of health checks; zero means 30 s, and a
	// negative value disables them.
	HealthInterval time.Duration
	// MaxBackoff caps the delay between restarts; zero means one minute.
	MaxBackoff time.Duration
	// Env is added to the plugin's environment.
	Env []string
	// HostAPI serves the plugin's requests to the host API on a socket next
	// to the plugin's; nil serves none.
	HostAPI http.Handler
	// DataDir is the plugin's writable data folder, passed in
	// MAVIO_PLUGIN_DATA; empty passes none.
	DataDir string
	Logger  *slog.Logger
}

// Plugin is a supervised plugin process. It implements host.Plugin.
type Plugin struct {
	clients.Set
	manifest *pluginv1.Manifest
	path     string
	opts     Options
	log      *slog.Logger
	dir      string
	socket   string
	token    string
	host     *http.Server
	routes   http.Handler

	mu     sync.Mutex
	cmd    *exec.Cmd
	exited chan struct{} // closed when the current process exits
	ready  chan struct{} // closed when a process is serving; replaced on restart
	// stopping tells the supervisor not to restart; closing rejects calls.
	stopping, closing bool

	stop     context.CancelFunc
	done     chan struct{}
	restarts atomic.Int64
}

// Start launches the plugin executable at path and waits until it serves.
func Start(ctx context.Context, path string, m *pluginv1.Manifest, opts Options) (*Plugin, error) {
	if opts.StartTimeout == 0 {
		opts.StartTimeout = 10 * time.Second
	}
	if opts.HealthInterval == 0 {
		opts.HealthInterval = 30 * time.Second
	}
	if opts.MaxBackoff == 0 {
		opts.MaxBackoff = time.Minute
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	dir, err := os.MkdirTemp(socketBase(), "mavio-")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	token := make([]byte, 32)
	_, _ = rand.Read(token)
	p := &Plugin{
		manifest: m,
		path:     path,
		opts:     opts,
		log:      opts.Logger.With("plugin", m.GetId()),
		dir:      dir,
		socket:   filepath.Join(dir, "plugin.sock"),
		token:    hex.EncodeToString(token),
		ready:    make(chan struct{}),
		done:     make(chan struct{}),
	}
	if opts.HostAPI != nil {
		if err := p.serveHost(); err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
	}
	h2 := p.dialer()
	p.Set = clients.New(&http.Client{Transport: roundTripper(func(req *http.Request) (*http.Response, error) {
		req = req.Clone(req.Context())
		req.Header.Set("Authorization", proc.AuthScheme+p.token)
		return h2.RoundTrip(req)
	})}, "http://plugin", m, connect.WithInterceptors(p.waitReady()))
	if manifest.HasCapability(m, pluginv1.Capability_CAPABILITY_HTTP_HANDLER) {
		p.routes = clients.Routes(roundTripper(func(req *http.Request) (*http.Response, error) {
			req.Header.Set(proc.TokenHeader, p.token)
			return h2.RoundTrip(req)
		}), 0, p.log)
	}

	if err := p.launch(ctx); err != nil {
		p.closeHost()
		_ = os.RemoveAll(dir)
		return nil, err
	}
	superviseCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	p.stop = cancel
	go p.supervise(superviseCtx)
	return p, nil
}

// serveHost serves the host API on host.sock, requiring the plugin's token.
func (p *Plugin) serveHost() error {
	ln, err := net.Listen("unix", filepath.Join(p.dir, "host.sock"))
	if err != nil {
		return fmt.Errorf("host API socket: %w", err)
	}
	want := []byte(proc.AuthScheme + p.token)
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)
	p.host = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), want) != 1 {
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
			r.Header.Del("Authorization")
			r.RemoteAddr = "plugin:" + p.manifest.GetId()
			p.opts.HostAPI.ServeHTTP(w, r)
		}),
		Protocols:         &protocols,
		ReadHeaderTimeout: 10 * time.Second,
		ErrorLog:          slog.NewLogLogger(p.log.Handler(), slog.LevelWarn),
	}
	go func() { _ = p.host.Serve(ln) }()
	return nil
}

func (p *Plugin) closeHost() {
	if p.host != nil {
		_ = p.host.Close()
	}
}

// socketBase is a short directory for the socket: Unix socket paths are
// limited to about 100 bytes, which macOS's TMPDIR can exceed.
func socketBase() string {
	if runtime.GOOS != "windows" {
		if info, err := os.Stat("/tmp"); err == nil && info.IsDir() {
			return "/tmp"
		}
	}
	return os.TempDir()
}

// dialer is an HTTP/2 cleartext transport dialing the plugin's socket.
func (p *Plugin) dialer() *http.Transport {
	var protocols http.Protocols
	protocols.SetUnencryptedHTTP2(true)
	return &http.Transport{
		Protocols: &protocols,
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", p.socket)
		},
	}
}

// HTTP returns the handler of the plugin's HTTP routes, or nil; it
// streams both ways.
func (p *Plugin) HTTP() http.Handler { return p.routes }

type roundTripper func(*http.Request) (*http.Response, error)

func (f roundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// waitReady holds calls while the plugin is restarting.
func (p *Plugin) waitReady() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			p.mu.Lock()
			ready, closing := p.ready, p.closing
			p.mu.Unlock()
			if closing {
				return nil, connect.NewError(connect.CodeUnavailable, errors.New("plugin closed"))
			}
			select {
			case <-ready:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return next(ctx, req)
		}
	}
}

// launch starts one process and waits for its handshake.
func (p *Plugin) launch(ctx context.Context) error {
	_ = os.Remove(p.socket)
	cmd := exec.Command(p.path)
	cmd.Env = append(os.Environ(), p.opts.Env...)
	cmd.Env = append(cmd.Env, proc.EnvSocket+"="+p.socket, proc.EnvToken+"="+p.token)
	if p.host != nil {
		cmd.Env = append(cmd.Env, proc.EnvHostSocket+"="+filepath.Join(p.dir, "host.sock"))
	}
	if p.opts.DataDir != "" {
		cmd.Env = append(cmd.Env, proc.EnvData+"="+p.opts.DataDir)
	}
	cmd.Dir = filepath.Dir(p.path)
	configure(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start %s: %w", p.manifest.GetId(), err)
	}
	exited := make(chan struct{})
	go p.forward(stderr, slog.LevelInfo)

	handshake := make(chan error, 1)
	go func() {
		sc := bufio.NewScanner(stdout)
		if !sc.Scan() {
			handshake <- fmt.Errorf("plugin exited before the handshake")
			return
		}
		var hs proc.Handshake
		switch err := json.Unmarshal(sc.Bytes(), &hs); {
		case err != nil:
			handshake <- fmt.Errorf("bad handshake %q: %w", sc.Text(), err)
		case hs.Protocol != proc.ProtocolVersion:
			handshake <- fmt.Errorf("unsupported handshake protocol %d", hs.Protocol)
		case hs.PluginID != p.manifest.GetId():
			handshake <- fmt.Errorf("plugin reported id %q, manifest says %q", hs.PluginID, p.manifest.GetId())
		default:
			handshake <- nil
		}
		p.forward(io.MultiReader(stdoutRest(sc)), slog.LevelDebug)
	}()
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()

	timer := time.NewTimer(p.opts.StartTimeout)
	defer timer.Stop()
	select {
	case err := <-handshake:
		if err != nil {
			_ = cmd.Process.Kill()
			<-exited
			return fmt.Errorf("%s: %w", p.manifest.GetId(), err)
		}
	case <-exited:
		return fmt.Errorf("%s exited during startup", p.manifest.GetId())
	case <-timer.C:
		_ = cmd.Process.Kill()
		<-exited
		return fmt.Errorf("%s: no handshake within %s", p.manifest.GetId(), p.opts.StartTimeout)
	case <-ctx.Done():
		_ = cmd.Process.Kill()
		<-exited
		return ctx.Err()
	}

	p.mu.Lock()
	p.cmd, p.exited = cmd, exited
	close(p.ready)
	p.mu.Unlock()
	p.log.Info("plugin started", "pid", cmd.Process.Pid)
	return nil
}

// stdoutRest continues reading stdout after the handshake line.
func stdoutRest(sc *bufio.Scanner) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		for sc.Scan() {
			_, _ = pw.Write(append(sc.Bytes(), '\n'))
		}
		_ = pw.Close()
	}()
	return pr
}

func (p *Plugin) forward(r io.Reader, level slog.Level) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		p.log.Log(context.Background(), level, sc.Text())
	}
}

// supervise restarts the process when it exits or fails health checks.
func (p *Plugin) supervise(ctx context.Context) {
	defer close(p.done)
	backoff := time.Second
	for {
		p.mu.Lock()
		exited := p.exited
		p.mu.Unlock()

		if !p.watch(ctx, exited) {
			return
		}
		p.mu.Lock()
		if p.stopping {
			p.mu.Unlock()
			return
		}
		p.ready = make(chan struct{}) // hold calls until the restart completes
		p.mu.Unlock()

		for {
			p.log.Warn("plugin stopped; restarting", "in", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			err := p.launch(ctx)
			if err == nil {
				p.restarts.Add(1)
				backoff = time.Second
				break
			}
			p.log.Error("plugin restart failed", "err", err)
			backoff = min(2*backoff, p.opts.MaxBackoff)
		}
	}
}

// watch returns true when the process exited or became unhealthy, and false
// when supervision should stop.
func (p *Plugin) watch(ctx context.Context, exited <-chan struct{}) bool {
	var tick <-chan time.Time
	if p.opts.HealthInterval > 0 {
		t := time.NewTicker(p.opts.HealthInterval)
		defer t.Stop()
		tick = t.C
	}
	failures := 0
	for {
		select {
		case <-ctx.Done():
			return false
		case <-exited:
			return true
		case <-tick:
			hctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			resp, err := p.Lifecycle().Health(hctx, &pluginv1.HealthRequest{})
			cancel()
			if err == nil && resp.GetHealthy() {
				failures = 0
				continue
			}
			failures++
			p.log.Warn("plugin health check failed", "err", err, "message", resp.GetMessage(), "failures", failures)
			if failures >= 3 {
				p.mu.Lock()
				if p.cmd != nil {
					_ = p.cmd.Process.Kill()
				}
				p.mu.Unlock()
				<-exited
				return true
			}
		}
	}
}

// Restarts reports how many times the supervisor restarted the plugin.
func (p *Plugin) Restarts() int { return int(p.restarts.Load()) }

// Close asks the plugin to shut down, then terminates and finally kills it.
func (p *Plugin) Close(ctx context.Context) error {
	p.mu.Lock()
	if p.stopping {
		p.mu.Unlock()
		return nil
	}
	p.stopping = true
	cmd, exited := p.cmd, p.exited
	p.mu.Unlock()

	sctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	_, _ = p.Lifecycle().Shutdown(sctx, &pluginv1.ShutdownRequest{})
	cancel()

	p.mu.Lock()
	p.closing = true
	p.mu.Unlock()
	p.stop()

	if cmd != nil {
		select {
		case <-exited:
		case <-time.After(3 * time.Second):
			_ = terminate(cmd.Process)
			select {
			case <-exited:
			case <-time.After(3 * time.Second):
				_ = cmd.Process.Kill()
				<-exited
			}
		}
	}
	<-p.done
	p.closeHost()
	return os.RemoveAll(p.dir)
}

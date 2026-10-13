package rclone

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

// DaemonOptions configure an rcd the server runs itself.
type DaemonOptions struct {
	// Path is the rclone executable.
	Path string
	// Config is the rclone.conf file, created when missing, and ConfigPass
	// the password it is encrypted with; empty for a file not encrypted,
	// or encrypted with the RCLONE_CONFIG_PASS of the environment.
	Config, ConfigPass string
	// CacheDir is rclone's cache folder.
	CacheDir string
	// StartTimeout bounds the wait for the rcd to answer; 30 s when 0.
	StartTimeout time.Duration
	// MaxBackoff caps the delay between restarts; one minute when 0.
	MaxBackoff time.Duration
	Logger     *slog.Logger
}

// Daemon is an rcd the server runs, listening on a Unix socket in a
// private folder with random credentials. It restarts the rcd when it
// exits, waiting longer after each quick failure.
type Daemon struct {
	opts   DaemonOptions
	log    *slog.Logger
	dir    string
	ep     Endpoint
	client *Client

	mu       sync.Mutex
	cmd      *exec.Cmd
	exited   chan struct{}
	stopping bool

	stop context.CancelFunc
	done chan struct{}
}

// stableRun is how long an rcd runs before its restarts count afresh.
const stableRun = time.Minute

// StartDaemon starts an rcd and waits until it answers.
func StartDaemon(ctx context.Context, opts DaemonOptions) (*Daemon, error) {
	if opts.StartTimeout == 0 {
		opts.StartTimeout = 30 * time.Second
	}
	if opts.MaxBackoff == 0 {
		opts.MaxBackoff = time.Minute
	}
	if opts.Logger == nil {
		opts.Logger = slog.Default()
	}
	dir, err := os.MkdirTemp(socketBase(), "mavio-rclone-")
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	pass := make([]byte, 24)
	_, _ = rand.Read(pass)
	d := &Daemon{
		opts: opts,
		log:  opts.Logger.With("component", "rclone"),
		dir:  dir,
		ep:   Endpoint{URL: "unix://" + filepath.ToSlash(filepath.Join(dir, "rc.sock")), User: "mavio", Password: hex.EncodeToString(pass)},
		done: make(chan struct{}),
	}
	if d.client, err = NewClient(d.ep, nil); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if err := d.prepareConfig(ctx); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	if err := d.launch(ctx); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	sctx, stop := context.WithCancel(context.WithoutCancel(ctx))
	d.stop = stop
	go d.supervise(sctx)
	return d, nil
}

// socketBase is a folder whose paths are short enough for Unix sockets.
func socketBase() string {
	if runtime.GOOS != "windows" {
		if info, err := os.Stat("/tmp"); err == nil && info.IsDir() {
			return "/tmp"
		}
	}
	return os.TempDir()
}

// Endpoint returns where the rcd listens and its credentials.
func (d *Daemon) Endpoint() Endpoint { return d.ep }

// Client returns a client of the rcd.
func (d *Daemon) Client() *Client { return d.client }

// prepareConfig creates the configuration file when missing, encrypted
// with ConfigPass when set.
func (d *Daemon) prepareConfig(ctx context.Context) error {
	if _, err := os.Stat(d.opts.Config); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(d.opts.Config), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(d.opts.Config, nil, 0o600); err != nil {
		return err
	}
	if d.opts.ConfigPass == "" {
		return nil
	}
	// rclone reads the new password twice from standard input when it is
	// not a terminal.
	cmd := exec.CommandContext(ctx, d.opts.Path, "config", "encryption", "set", "--config", d.opts.Config)
	cmd.Stdin = strings.NewReader(d.opts.ConfigPass + "\n" + d.opts.ConfigPass + "\n")
	if out, err := cmd.CombinedOutput(); err != nil {
		_ = os.Remove(d.opts.Config)
		return fmt.Errorf("encrypt rclone configuration: %w: %s", err, out)
	}
	return nil
}

// launch starts the rcd and waits until it answers.
func (d *Daemon) launch(ctx context.Context) error {
	socket := filepath.Join(d.dir, "rc.sock")
	_ = os.Remove(socket)
	args := []string{
		"rcd",
		"--rc-addr", "unix://" + socket,
		"--rc-user", d.ep.User,
		"--rc-pass", d.ep.Password,
		"--rc-serve",
		// A remote's file system, with its folder IDs and the objects
		// --rc-serve found, outlives pauses of its reads.
		"--fs-cache-expire-duration", "1h",
		"--config", d.opts.Config,
		"--log-level", "NOTICE",
	}
	if d.opts.CacheDir != "" {
		args = append(args, "--cache-dir", d.opts.CacheDir)
	}
	cmd := exec.Command(d.opts.Path, args...)
	cmd.Env = append(os.Environ(), "RCLONE_ASK_PASSWORD=false")
	if d.opts.ConfigPass != "" {
		cmd.Env = append(cmd.Env, "RCLONE_CONFIG_PASS="+d.opts.ConfigPass)
	}
	configure(cmd)
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	cmd.Stdout = cmd.Stderr
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start rclone: %w", err)
	}
	exited := make(chan struct{})
	go d.forward(stderr)
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()

	wctx, cancel := context.WithTimeout(ctx, d.opts.StartTimeout)
	defer cancel()
	for {
		if err := d.client.Noop(wctx); err == nil {
			break
		}
		select {
		case <-exited:
			return errors.New("rclone exited while starting")
		case <-wctx.Done():
			_ = cmd.Process.Kill()
			<-exited
			return fmt.Errorf("rclone did not answer: %w", wctx.Err())
		case <-time.After(100 * time.Millisecond):
		}
	}
	d.mu.Lock()
	d.cmd, d.exited = cmd, exited
	d.mu.Unlock()
	return nil
}

// forward logs the rcd's output, "2006/01/02 15:04:05 LEVEL : message"
// lines, at their levels.
func (d *Daemon) forward(r io.Reader) {
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		level, msg := logLine(sc.Text())
		d.log.Log(context.Background(), level, msg)
	}
}

func logLine(line string) (slog.Level, string) {
	fields := strings.SplitN(line, " ", 4)
	if len(fields) < 4 || len(fields[0]) != len("2006/01/02") {
		return slog.LevelInfo, line
	}
	msg := strings.TrimPrefix(fields[3], ": ")
	if strings.Contains(msg, "Didn't finish writing GET request") {
		// ffmpeg and ffprobe stop reading where they need no more.
		return slog.LevelDebug, msg
	}
	switch fields[2] {
	case "CRITICAL", "ERROR":
		return slog.LevelError, msg
	case "DEBUG":
		return slog.LevelDebug, msg
	}
	return slog.LevelInfo, msg
}

// supervise restarts the rcd when it exits.
func (d *Daemon) supervise(ctx context.Context) {
	defer close(d.done)
	backoff := time.Second
	for {
		d.mu.Lock()
		exited := d.exited
		d.mu.Unlock()
		started := time.Now()
		select {
		case <-ctx.Done():
			return
		case <-exited:
		}
		d.mu.Lock()
		stopping := d.stopping
		d.mu.Unlock()
		if stopping {
			return
		}
		if time.Since(started) > stableRun {
			backoff = time.Second
		}
		for {
			d.log.Warn("rclone stopped; restarting", "in", backoff)
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return
			}
			err := d.launch(ctx)
			backoff = min(2*backoff, d.opts.MaxBackoff)
			if err == nil {
				break
			}
			d.log.Error("rclone restart failed", "err", err)
		}
	}
}

// Close stops the rcd: it is asked to quit, then terminated, then killed.
func (d *Daemon) Close() error {
	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		return nil
	}
	d.stopping = true
	cmd, exited := d.cmd, d.exited
	d.mu.Unlock()
	d.stop()
	if cmd != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		_ = d.client.Call(ctx, "core/quit", map[string]any{}, nil)
		cancel()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			_ = terminate(cmd.Process)
			select {
			case <-exited:
			case <-time.After(5 * time.Second):
				_ = cmd.Process.Kill()
				<-exited
			}
		}
	}
	<-d.done
	return os.RemoveAll(d.dir)
}

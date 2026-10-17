package procs

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// errGone reports that no process has an ID, or that it has exited.
var errGone = errors.New("no such process")

// errUnsupported reports that this system does not tell a process's
// start time and executable.
var errUnsupported = errors.New("processes are not identified on this system")

const (
	// serverGrace is how long a server left running has to stop: its
	// plugins and containers stop first.
	serverGrace = 30 * time.Second
	// childGrace is how long a process left behind has to stop.
	childGrace = 5 * time.Second
	// killWait bounds the wait for a killed process to be gone.
	killWait = 5 * time.Second
)

// record is a process as the server recorded it.
type record struct {
	PID int `json:"pid"`
	// Start and Path identify the process: when it started, as the
	// system tells it, and its executable.
	Start string `json:"start"`
	Path  string `json:"path"`
	// What started it, for the logs.
	What string `json:"what"`
}

// Registry records processes in a folder of the home.
type Registry struct {
	dir string
	log *slog.Logger
	// serverGrace and childGrace are how long a server and a process
	// left running have to stop before they are killed.
	serverGrace, childGrace time.Duration
}

// Open returns the registry in dir, created when missing.
func Open(dir string, log *slog.Logger) (*Registry, error) {
	if err := os.MkdirAll(filepath.Join(dir, "children"), 0o700); err != nil {
		return nil, err
	}
	return &Registry{dir: dir, log: cmp.Or(log, slog.New(slog.DiscardHandler)), serverGrace: serverGrace, childGrace: childGrace}, nil
}

// Claim ends a server recorded as running in this home that still runs,
// gracefully first, and records this one; release removes the record.
func (r *Registry) Claim(ctx context.Context) (release func(), err error) {
	file := filepath.Join(r.dir, "server.json")
	r.end(ctx, file, r.serverGrace)
	rec, err := identified(os.Getpid(), "server")
	if errors.Is(err, errUnsupported) {
		return func() {}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("identify the server's process: %w", err)
	}
	if err := write(file, rec); err != nil {
		return nil, err
	}
	return func() { _ = os.Remove(file) }, nil
}

// Reap ends the processes servers before started and left running,
// gracefully first.
func (r *Registry) Reap(ctx context.Context) {
	dir := filepath.Join(r.dir, "children")
	entries, err := os.ReadDir(dir)
	if err != nil {
		r.log.WarnContext(ctx, "read the processes left behind", "err", err)
		return
	}
	for _, e := range entries {
		r.end(ctx, filepath.Join(dir, e.Name()), r.childGrace)
	}
}

// Track returns a function recording each process what starts, by its
// ID, until the function it returns is called once the process exited.
func (r *Registry) Track(what string) func(pid int) (done func()) {
	return func(pid int) func() {
		rec, err := identified(pid, what)
		if err != nil {
			if !errors.Is(err, errUnsupported) {
				r.log.Warn("record a process", "what", what, "pid", pid, "err", err)
			}
			return func() {}
		}
		file := filepath.Join(r.dir, "children", strconv.Itoa(pid)+".json")
		if err := write(file, rec); err != nil {
			r.log.Warn("record a process", "what", what, "pid", pid, "err", err)
			return func() {}
		}
		return func() { _ = os.Remove(file) }
	}
}

// end ends the process file records when it still runs, and removes the
// record.
func (r *Registry) end(ctx context.Context, file string, grace time.Duration) {
	defer func() { _ = os.Remove(file) }()
	b, err := os.ReadFile(file)
	if errors.Is(err, os.ErrNotExist) {
		return
	}
	var rec record
	if err == nil {
		err = json.Unmarshal(b, &rec)
	}
	if err != nil {
		r.log.WarnContext(ctx, "read a process left behind", "file", file, "err", err)
		return
	}
	if !r.same(ctx, rec) {
		return
	}
	log := r.log.With("what", rec.What, "pid", rec.PID, "path", rec.Path)
	log.WarnContext(ctx, "ending a process left running")
	if err := terminate(rec.PID); err == nil && r.wait(ctx, rec, grace) {
		return
	}
	if err := kill(rec.PID); err != nil && !errors.Is(err, errGone) {
		log.WarnContext(ctx, "kill a process left running", "err", err)
		return
	}
	if !r.wait(ctx, rec, killWait) {
		log.WarnContext(ctx, "a process left running is still running after being killed")
	}
}

// same reports whether rec's process still runs.
func (r *Registry) same(ctx context.Context, rec record) bool {
	start, path, err := identify(rec.PID)
	switch {
	case errors.Is(err, errGone):
		return false
	case err != nil:
		r.log.WarnContext(ctx, "identify a process left behind", "what", rec.What, "pid", rec.PID, "err", err)
		return false
	}
	return start == rec.Start && path == rec.Path
}

// wait waits up to d for rec's process to be gone.
func (r *Registry) wait(ctx context.Context, rec record, d time.Duration) bool {
	deadline := time.Now().Add(d)
	for {
		if start, _, err := identify(rec.PID); errors.Is(err, errGone) || err == nil && start != rec.Start {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(50 * time.Millisecond):
		}
	}
}

func identified(pid int, what string) (record, error) {
	start, path, err := identify(pid)
	if err != nil {
		return record{}, err
	}
	return record{PID: pid, Start: start, Path: path, What: what}, nil
}

// write writes rec to file, whole or not at all.
func write(file string, rec record) error {
	b, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	tmp := file + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, file)
}

package procs

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"
	"testing"
	"time"
)

// envChild makes the test binary a process that runs until it is ended;
// "stubborn" ignores SIGTERM.
const envChild = "MAVIO_TEST_PROCS_CHILD"

func TestMain(m *testing.M) {
	if mode, ok := os.LookupEnv(envChild); ok {
		if mode == "stubborn" {
			signal.Ignore(syscall.SIGTERM)
		}
		time.Sleep(time.Minute)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// child starts the test binary as a process left running, which exits
// when the test ends.
func child(t *testing.T, mode string) (*exec.Cmd, <-chan struct{}) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(), envChild+"="+mode)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	exited := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(exited)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-exited
	})
	return cmd, exited
}

func open(t *testing.T) *Registry {
	t.Helper()
	r, err := Open(t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	r.childGrace, r.serverGrace = time.Second, time.Second
	return r
}

func supported(t *testing.T) {
	t.Helper()
	if _, _, err := identify(os.Getpid()); errors.Is(err, errUnsupported) {
		t.Skip("processes are not identified on " + runtime.GOOS)
	}
}

func exitedWithin(t *testing.T, exited <-chan struct{}, d time.Duration) bool {
	t.Helper()
	select {
	case <-exited:
		return true
	case <-time.After(d):
		return false
	}
}

func TestReap(t *testing.T) {
	supported(t)
	for _, mode := range []string{"plain", "stubborn"} {
		t.Run(mode, func(t *testing.T) {
			r := open(t)
			cmd, exited := child(t, mode)
			r.Track("test")(cmd.Process.Pid)
			// A server started again finds it.
			again, err := Open(r.dir, nil)
			if err != nil {
				t.Fatal(err)
			}
			again.childGrace = time.Second
			again.Reap(t.Context())
			if !exitedWithin(t, exited, 5*time.Second) {
				t.Fatal("the process left running still runs after Reap")
			}
			if entries, _ := os.ReadDir(filepath.Join(r.dir, "children")); len(entries) != 0 {
				t.Errorf("records after Reap = %d, want = 0", len(entries))
			}
		})
	}
}

func TestReapLeavesAnotherProcess(t *testing.T) {
	supported(t)
	r := open(t)
	cmd, exited := child(t, "plain")
	r.Track("test")(cmd.Process.Pid)
	// The ID now names another process, started at another time.
	file := filepath.Join(r.dir, "children", strconv.Itoa(cmd.Process.Pid)+".json")
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	var rec record
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	rec.Start += "0"
	if err := write(file, rec); err != nil {
		t.Fatal(err)
	}
	r.Reap(t.Context())
	if exitedWithin(t, exited, 500*time.Millisecond) {
		t.Fatal("Reap ended a process it did not start")
	}
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("record after Reap: err = %v, want = not exist", err)
	}
}

func TestTrackDone(t *testing.T) {
	supported(t)
	r := open(t)
	cmd, _ := child(t, "plain")
	done := r.Track("test")(cmd.Process.Pid)
	file := filepath.Join(r.dir, "children", strconv.Itoa(cmd.Process.Pid)+".json")
	if _, err := os.Stat(file); err != nil {
		t.Fatalf("record after Track: %v", err)
	}
	done()
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("record after done: err = %v, want = not exist", err)
	}
}

func TestClaim(t *testing.T) {
	supported(t)
	r := open(t)
	// A server left running in the home.
	cmd, exited := child(t, "plain")
	rec, err := identified(cmd.Process.Pid, "server")
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(r.dir, "server.json")
	if err := write(file, rec); err != nil {
		t.Fatal(err)
	}
	release, err := r.Claim(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !exitedWithin(t, exited, 5*time.Second) {
		t.Fatal("the server left running still runs after Claim")
	}
	b, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &rec); err != nil {
		t.Fatal(err)
	}
	if rec.PID != os.Getpid() {
		t.Errorf("claimed by %d, want = %d", rec.PID, os.Getpid())
	}
	release()
	if _, err := os.Stat(file); !os.IsNotExist(err) {
		t.Errorf("record after release: err = %v, want = not exist", err)
	}
}

package pgtest

import (
	"crypto/sha256"
	"encoding/hex"
	"net"
	"os"
	"path/filepath"
	"testing"
)

// writeContext records a Docker CLI context the way `docker context
// create` does.
func writeContext(t *testing.T, config, name, host string) {
	t.Helper()
	sum := sha256.Sum256([]byte(name))
	dir := filepath.Join(config, "contexts", "meta", hex.EncodeToString(sum[:]))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	meta := `{"Name":"` + name + `","Endpoints":{"docker":{"Host":"` + host + `"}}}`
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), []byte(meta), 0o644); err != nil {
		t.Fatal(err)
	}
}

// listen creates a Unix socket at path, as a Docker engine does.
func listen(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Skipf("unix sockets unavailable: %v", err)
	}
	t.Cleanup(func() { _ = ln.Close() })
}

func TestDockerHost(t *testing.T) {
	if _, err := os.Stat("/var/run/docker.sock"); err == nil {
		t.Skip("the default socket exists here and would be found first")
	}
	// Socket paths are limited to about 100 bytes; t.TempDir can be longer.
	home, err := os.MkdirTemp("", "pgt")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(home) })
	config := filepath.Join(home, ".docker")
	orb := filepath.Join(home, ".orbstack", "run", "docker.sock")
	colima := filepath.Join(home, ".colima", "default", "docker.sock")
	listen(t, orb)
	listen(t, colima)
	writeContext(t, config, "orbstack", "unix://"+orb)
	writeContext(t, config, "gone", "unix://"+filepath.Join(home, "gone.sock"))
	writeContext(t, config, "remote", "tcp://build.example:2376")

	tests := []struct {
		name    string
		current string // currentContext in config.json
		env     map[string]string
		want    string
	}{
		{"current context", "orbstack", nil, "unix://" + orb},
		{"DOCKER_CONTEXT wins", "orbstack", map[string]string{"DOCKER_CONTEXT": "remote"}, "tcp://build.example:2376"},
		{"context without its socket", "gone", nil, "unix://" + orb},
		{"default context", "default", nil, "unix://" + orb},
		{"no config", "", nil, "unix://" + orb},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_ = os.Remove(filepath.Join(config, "config.json"))
			if tt.current != "" {
				if err := os.WriteFile(filepath.Join(config, "config.json"), []byte(`{"currentContext":"`+tt.current+`"}`), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			getenv := func(k string) string { return tt.env[k] }
			if got := DockerHost(getenv, home); got != tt.want {
				t.Errorf("DockerHost() = %q, want = %q", got, tt.want)
			}
		})
	}

	// Without OrbStack, Colima's socket is found.
	if err := os.Remove(orb); err != nil {
		t.Fatal(err)
	}
	if got, want := DockerHost(func(string) string { return "" }, home), "unix://"+colima; got != want {
		t.Errorf("DockerHost() without OrbStack = %q, want = %q", got, want)
	}
}

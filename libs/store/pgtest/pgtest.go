// Package pgtest provides PostgreSQL to tests and development tools: the
// server MAVIO_TEST_POSTGRES_DSN names, or a throwaway container on the
// Docker engine the Docker CLI uses.
package pgtest

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// EnvDSN points tests at an existing PostgreSQL server, in a database they
// may create databases in.
const EnvDSN = "MAVIO_TEST_POSTGRES_DSN"

// Image is the PostgreSQL container image.
const Image = "postgres:17-alpine"

// Start returns the DSN of the server EnvDSN names, or of a new container
// and the function that removes it.
func Start(ctx context.Context) (dsn string, stop func(), err error) {
	if dsn := os.Getenv(EnvDSN); dsn != "" {
		return dsn, func() {}, nil
	}
	return Container(ctx)
}

// Container starts a PostgreSQL container and returns its DSN and the
// function that removes it. Unless DOCKER_HOST is set, the container runs
// on the engine of the Docker CLI's current context, or on the first
// engine found at a common socket path.
func Container(ctx context.Context) (dsn string, stop func(), err error) {
	if os.Getenv("DOCKER_HOST") == "" {
		home, _ := os.UserHomeDir()
		if host := DockerHost(os.Getenv, home); host != "" {
			// testcontainers reads no Docker CLI context; the environment
			// is how it is told about one.
			if err := os.Setenv("DOCKER_HOST", host); err != nil {
				return "", nil, err
			}
		}
	}
	c, err := tcpostgres.Run(ctx, Image, tcpostgres.BasicWaitStrategies())
	if err != nil {
		return "", nil, fmt.Errorf("start postgres: %w", err)
	}
	stop = func() { _ = testcontainers.TerminateContainer(c) }
	dsn, err = c.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		stop()
		return "", nil, err
	}
	return dsn, stop, nil
}

// DockerHost returns the Docker engine address the Docker CLI uses: the
// endpoint of its current context (DOCKER_CONTEXT, or currentContext in
// its config.json), else the first socket found among those Docker
// Desktop, OrbStack, Colima, Rancher Desktop and Podman create. It returns
// "" when there is none.
func DockerHost(getenv func(string) string, home string) string {
	config := getenv("DOCKER_CONFIG")
	if config == "" && home != "" {
		config = filepath.Join(home, ".docker")
	}
	if config != "" {
		if host := contextHost(config, getenv("DOCKER_CONTEXT")); host != "" {
			return host
		}
	}
	sockets := []string{"/var/run/docker.sock"}
	if home != "" {
		sockets = append(sockets,
			filepath.Join(home, ".docker", "run", "docker.sock"),
			filepath.Join(home, ".orbstack", "run", "docker.sock"),
			filepath.Join(home, ".colima", "default", "docker.sock"),
			filepath.Join(home, ".rd", "docker.sock"),
		)
	}
	if run := getenv("XDG_RUNTIME_DIR"); run != "" {
		sockets = append(sockets, filepath.Join(run, "docker.sock"), filepath.Join(run, "podman", "podman.sock"))
	}
	for _, s := range sockets {
		if info, err := os.Stat(s); err == nil && info.Mode().Type() == os.ModeSocket {
			return "unix://" + s
		}
	}
	return ""
}

// contextHost returns the Docker endpoint of a CLI context, by default the
// current one of config.json; "" for the default context or when unknown.
func contextHost(config, name string) string {
	if name == "" {
		data, err := os.ReadFile(filepath.Join(config, "config.json"))
		if err != nil {
			return ""
		}
		var cfg struct {
			CurrentContext string `json:"currentContext"`
		}
		if json.Unmarshal(data, &cfg) != nil {
			return ""
		}
		name = cfg.CurrentContext
	}
	if name == "" || name == "default" {
		return ""
	}
	sum := sha256.Sum256([]byte(name))
	data, err := os.ReadFile(filepath.Join(config, "contexts", "meta", hex.EncodeToString(sum[:]), "meta.json"))
	if err != nil {
		return ""
	}
	var meta struct {
		Endpoints map[string]struct {
			Host string `json:"Host"`
		} `json:"Endpoints"`
	}
	if json.Unmarshal(data, &meta) != nil {
		return ""
	}
	host := meta.Endpoints["docker"].Host
	if path, ok := strings.CutPrefix(host, "unix://"); ok {
		if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
			return ""
		}
	}
	return host
}

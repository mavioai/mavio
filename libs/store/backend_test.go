package store_test

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mavioai/mavio/libs/store"
)

// EnvPostgresDSN points the conformance tests at an existing PostgreSQL
// server (a database the tests may create databases in). Without it, tests
// start a container when Docker is available and skip PostgreSQL otherwise.
const EnvPostgresDSN = "MAVIO_TEST_POSTGRES_DSN"

var (
	pgAdminDSN string // empty when PostgreSQL is unavailable
	pgSkip     string
	dbCounter  atomic.Int64
)

func TestMain(m *testing.M) {
	flag.Parse()
	code := func() int {
		if dsn := os.Getenv(EnvPostgresDSN); dsn != "" {
			pgAdminDSN = dsn
			return m.Run()
		}
		if testing.Short() {
			pgSkip = "PostgreSQL skipped in -short mode"
			return m.Run()
		}
		ctx := context.Background()
		container, err := tcpostgres.Run(ctx, "postgres:17-alpine", tcpostgres.BasicWaitStrategies())
		if err != nil {
			pgSkip = fmt.Sprintf("PostgreSQL container unavailable: %v", err)
			return m.Run()
		}
		defer func() { _ = testcontainers.TerminateContainer(container) }()
		if pgAdminDSN, err = container.ConnectionString(ctx, "sslmode=disable"); err != nil {
			pgSkip = err.Error()
		}
		return m.Run()
	}()
	os.Exit(code)
}

// eachBackend runs fn against a fresh, migrated SQLite and PostgreSQL store.
func eachBackend(t *testing.T, fn func(t *testing.T, s *store.Store)) {
	t.Helper()
	t.Run("sqlite", func(t *testing.T) {
		t.Parallel()
		s, err := store.Open(t.Context(), "sqlite:"+filepath.Join(t.TempDir(), "mavio.db"))
		if err != nil {
			t.Fatalf("open sqlite: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })
		fn(t, s)
	})
	t.Run("postgres", func(t *testing.T) {
		t.Parallel()
		if pgAdminDSN == "" {
			t.Skip(pgSkip)
		}
		dsn := freshPostgresDB(t)
		s, err := store.Open(t.Context(), dsn)
		if err != nil {
			t.Fatalf("open postgres: %v", err)
		}
		t.Cleanup(func() { _ = s.Close() })
		fn(t, s)
	})
}

// freshPostgresDB creates an empty database and returns its DSN.
func freshPostgresDB(t *testing.T) string {
	t.Helper()
	name := fmt.Sprintf("mavio_test_%d_%d", os.Getpid(), dbCounter.Add(1))
	admin, err := sql.Open("pgx", pgAdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	if _, err := admin.ExecContext(t.Context(), "CREATE DATABASE "+name); err != nil {
		t.Fatalf("create database: %v", err)
	}
	t.Cleanup(func() {
		admin, err := sql.Open("pgx", pgAdminDSN)
		if err == nil {
			_, _ = admin.Exec("DROP DATABASE IF EXISTS " + name + " WITH (FORCE)")
			_ = admin.Close()
		}
	})
	return replaceDatabase(pgAdminDSN, name)
}

// replaceDatabase swaps the database name in a postgres:// URL.
func replaceDatabase(dsn, name string) string {
	rest, query, _ := strings.Cut(dsn, "?")
	slash := strings.LastIndex(rest, "/")
	out := rest[:slash+1] + name
	if query != "" {
		out += "?" + query
	}
	return out
}

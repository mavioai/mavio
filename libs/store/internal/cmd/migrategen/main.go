// Command migrategen writes versioned migrations from the ent schema.
//
// It replays the existing migration directory of a dialect on an empty
// development database, diffs the result against the ent schema and writes
// the difference as a new migration (Atlas format, with atlas.sum). SQLite
// uses an in-memory database; PostgreSQL uses a throwaway container.
//
//	go run ./internal/cmd/migrategen -dialect sqlite -name add_foo
//	go run ./internal/cmd/migrategen -dialect postgres -name add_foo
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"

	atlasmigrate "ariga.io/atlas/sql/migrate"
	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "github.com/ncruces/go-sqlite3/driver"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/mavioai/mavio/libs/store/internal/ent/migrate"
)

func main() {
	if err := run(context.Background()); err != nil {
		slog.Error("migrategen failed", "err", err)
		os.Exit(1)
	}
}

func run(ctx context.Context) error {
	dialectName := flag.String("dialect", "", `"sqlite" or "postgres"`)
	name := flag.String("name", "", "migration name, e.g. add_item_index")
	dir := flag.String("dir", "", "migration directory (default migrations/<dialect>)")
	flag.Parse()
	if *name == "" {
		return fmt.Errorf("-name is required")
	}
	if *dir == "" {
		*dir = "migrations/" + *dialectName
	}

	var (
		db  *sql.DB
		drv string
		err error
	)
	switch *dialectName {
	case "sqlite":
		drv = dialect.SQLite
		db, err = sql.Open("sqlite3", "file:dev?mode=memory&_pragma=foreign_keys(1)")
		if err != nil {
			return err
		}
		db.SetMaxOpenConns(1) // one connection keeps the in-memory database alive
	case "postgres":
		drv = dialect.Postgres
		container, err := tcpostgres.Run(ctx, "postgres:17-alpine", tcpostgres.BasicWaitStrategies())
		if err != nil {
			return fmt.Errorf("start postgres: %w", err)
		}
		defer func() { _ = testcontainers.TerminateContainer(container) }()
		dsn, err := container.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			return err
		}
		if db, err = sql.Open("pgx", dsn); err != nil {
			return err
		}
	default:
		return fmt.Errorf(`-dialect must be "sqlite" or "postgres"`)
	}
	defer db.Close()

	if err := os.MkdirAll(*dir, 0o755); err != nil {
		return err
	}
	local, err := atlasmigrate.NewLocalDir(*dir)
	if err != nil {
		return err
	}
	m, err := schema.NewMigrate(entsql.OpenDB(drv, db),
		schema.WithDir(local),
		schema.WithMigrationMode(schema.ModeReplay),
		schema.WithDialect(drv),
		schema.WithFormatter(formatter(drv)),
		schema.WithErrNoPlan(true),
	)
	if err != nil {
		return err
	}
	if err := m.NamedDiff(ctx, *name, migrate.Tables...); err != nil {
		if strings.Contains(err.Error(), "no changes") {
			slog.Info("schema is up to date; no migration written")
			return nil
		}
		return err
	}
	slog.Info("migration written", "dir", *dir, "name", *name)
	return nil
}

// formatter returns the migration file formatter. For SQLite it rewrites
// Atlas's backtick-quoted identifiers to standard double quotes, which sqlc's
// SQLite parser requires.
func formatter(drv string) atlasmigrate.Formatter {
	if drv != dialect.SQLite {
		return atlasmigrate.DefaultFormatter
	}
	return doubleQuoteFormatter{}
}

type doubleQuoteFormatter struct{}

func (doubleQuoteFormatter) Format(plan *atlasmigrate.Plan) ([]atlasmigrate.File, error) {
	changes := make([]*atlasmigrate.Change, len(plan.Changes))
	for i, c := range plan.Changes {
		cc := *c
		cc.Cmd = strings.ReplaceAll(c.Cmd, "`", `"`)
		cc.Comment = strings.ReplaceAll(c.Comment, "`", `"`)
		changes[i] = &cc
	}
	quoted := *plan
	quoted.Changes = changes
	return atlasmigrate.DefaultFormatter.Format(&quoted)
}

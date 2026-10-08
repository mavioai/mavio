// Package store implements the repository ports of libs/core on SQLite and
// PostgreSQL. ent handles CRUD, relationship traversal and dynamic item
// queries; sqlc handles the dialect-specific job queue SQL. Both share one
// *sql.Tx inside Store.InTx.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"runtime"
	"strings"

	entdialect "entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" driver
	_ "github.com/ncruces/go-sqlite3/driver"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
)

// Supported dialects.
const (
	DialectSQLite   = "sqlite"
	DialectPostgres = "postgres"
)

// Store implements core.Store.
type Store struct {
	dialect string
	// write runs writes and transactions; read runs queries. For SQLite they
	// are a single-connection writer pool and a reader pool; for PostgreSQL
	// they are the same pool.
	write, read *ent.Client
	// tx is set for a Store bound to a transaction.
	tx *sql.Tx
	// dbs are closed by Close; nil for transaction-bound stores.
	dbs []*sql.DB
}

var _ core.Store = (*Store)(nil)

// Open connects to the database described by dsn and applies pending
// migrations:
//
//	sqlite:/var/lib/mavio/mavio.db
//	postgres://user:pass@host:5432/mavio?sslmode=disable
func Open(ctx context.Context, dsn string) (*Store, error) {
	switch {
	case strings.HasPrefix(dsn, "sqlite:"):
		return openSQLite(ctx, strings.TrimPrefix(dsn, "sqlite:"))
	case strings.HasPrefix(dsn, "postgres://"), strings.HasPrefix(dsn, "postgresql://"):
		return openPostgres(ctx, dsn)
	default:
		return nil, fmt.Errorf("%w: unsupported database DSN %q (want sqlite:<path> or postgres://…)", core.ErrInvalid, dsn)
	}
}

func openSQLite(ctx context.Context, file string) (*Store, error) {
	if file == "" {
		return nil, fmt.Errorf("%w: empty SQLite path", core.ErrInvalid)
	}
	params := url.Values{}
	for _, p := range []string{"foreign_keys(1)", "journal_mode(wal)", "busy_timeout(10000)", "synchronous(normal)"} {
		params.Add("_pragma", p)
	}
	// Integer microseconds keep time columns ordered and zone-independent.
	params.Set("_timefmt", "unixepoch_micro")
	base := "file:" + (&url.URL{Path: file}).EscapedPath()

	writeParams := url.Values{}
	for k, v := range params {
		writeParams[k] = v
	}
	// Take the write lock when a transaction begins instead of upgrading
	// later, which avoids SQLITE_BUSY deadlocks between writers.
	writeParams.Set("_txlock", "immediate")

	wdb, err := sql.Open("sqlite3", base+"?"+writeParams.Encode())
	if err != nil {
		return nil, err
	}
	wdb.SetMaxOpenConns(1)
	rdb, err := sql.Open("sqlite3", base+"?"+params.Encode())
	if err != nil {
		wdb.Close()
		return nil, err
	}
	rdb.SetMaxOpenConns(max(4, runtime.NumCPU()))

	s := &Store{
		dialect: DialectSQLite,
		write:   ent.NewClient(ent.Driver(entsql.OpenDB(entdialect.SQLite, wdb))),
		read:    ent.NewClient(ent.Driver(entsql.OpenDB(entdialect.SQLite, rdb))),
		dbs:     []*sql.DB{wdb, rdb},
	}
	if err := s.migrate(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

func openPostgres(ctx context.Context, dsn string) (*Store, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, err
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}
	client := ent.NewClient(ent.Driver(entsql.OpenDB(entdialect.Postgres, db)))
	s := &Store{dialect: DialectPostgres, write: client, read: client, dbs: []*sql.DB{db}}
	if err := s.migrate(ctx); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// migrate applies pending schema migrations, then recomputes derived keys
// if their derivation changed.
func (s *Store) migrate(ctx context.Context) error {
	if err := migrate(ctx, s.dbs[0], s.dialect); err != nil {
		return err
	}
	return s.backfillKeys(ctx)
}

// Dialect returns DialectSQLite or DialectPostgres.
func (s *Store) Dialect() string { return s.dialect }

// Close closes the database connections.
func (s *Store) Close() error {
	var errs []error
	if s.dialect == DialectSQLite && len(s.dbs) > 0 {
		_, err := s.dbs[0].Exec("PRAGMA optimize")
		errs = append(errs, err)
	}
	for _, db := range s.dbs {
		errs = append(errs, db.Close())
	}
	return errors.Join(errs...)
}

// InTx runs fn with a Store bound to one transaction.
func (s *Store) InTx(ctx context.Context, fn func(tx core.Store) error) error {
	if s.tx != nil { // already in a transaction
		return fn(s)
	}
	db := s.dbs[0]
	sqlTx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	entDialect := entdialect.SQLite
	if s.dialect == DialectPostgres {
		entDialect = entdialect.Postgres
	}
	client := ent.NewClient(ent.Driver(txDriver{entsql.NewDriver(entDialect, entsql.Conn{ExecQuerier: sqlTx})}))
	txStore := &Store{dialect: s.dialect, write: client, read: client, tx: sqlTx}
	if err := fn(txStore); err != nil {
		return errors.Join(err, sqlTx.Rollback())
	}
	return sqlTx.Commit()
}

// txDriver is an ent driver bound to an open *sql.Tx. Operations that ent
// would wrap in their own transaction run inside the outer one instead.
type txDriver struct{ *entsql.Driver }

func (d txDriver) Tx(context.Context) (entdialect.Tx, error) { return entdialect.NopTx(d), nil }

func (d txDriver) BeginTx(ctx context.Context, _ *sql.TxOptions) (entdialect.Tx, error) {
	return d.Tx(ctx)
}

// Libraries returns the library repository.
func (s *Store) Libraries() core.LibraryRepository { return libraries{s} }

// Items returns the item repository.
func (s *Store) Items() core.ItemRepository { return items{s} }

// MediaSources returns the media source repository.
func (s *Store) MediaSources() core.MediaSourceRepository { return mediaSources{s} }

// Images returns the image repository.
func (s *Store) Images() core.ImageRepository { return images{s} }

// People returns the person repository.
func (s *Store) People() core.PersonRepository { return people{s} }

// Users returns the user repository.
func (s *Store) Users() core.UserRepository { return users{s} }

// UserData returns the user data repository.
func (s *Store) UserData() core.UserDataRepository { return userData{s} }

// Jobs returns the job queue.
func (s *Store) Jobs() core.JobQueue { return jobs{s} }

// writeTx runs fn in a transaction, reusing the current one if any.
func (s *Store) writeTx(ctx context.Context, fn func(tx *Store) error) error {
	return s.InTx(ctx, func(tx core.Store) error { return fn(tx.(*Store)) })
}

// mapErr translates ent errors into core sentinel errors.
func mapErr(err error, what string) error {
	switch {
	case err == nil:
		return nil
	case ent.IsNotFound(err):
		return fmt.Errorf("%s: %w", what, core.ErrNotFound)
	case ent.IsConstraintError(err):
		return fmt.Errorf("%s: %w: %w", what, core.ErrConflict, err)
	case ent.IsValidationError(err):
		return fmt.Errorf("%s: %w: %w", what, core.ErrInvalid, err)
	default:
		return fmt.Errorf("%s: %w", what, err)
	}
}

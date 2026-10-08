package store

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"
)

//go:embed migrations
var migrationFS embed.FS

// migrate applies the embedded migrations of the dialect that have not been
// applied yet, each in its own transaction, recording them in
// schema_migrations.
func migrate(ctx context.Context, db *sql.DB, dialect string) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version VARCHAR(32) NOT NULL PRIMARY KEY,
		applied_at BIGINT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	dir := path.Join("migrations", dialect)
	entries, err := fs.ReadDir(migrationFS, dir)
	if err != nil {
		return err
	}
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	slices.Sort(files)

	applied := map[string]bool{}
	rows, err := db.QueryContext(ctx, `SELECT version FROM schema_migrations`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			rows.Close()
			return err
		}
		applied[v] = true
	}
	if err := rows.Close(); err != nil {
		return err
	}

	for _, name := range files {
		version, _, _ := strings.Cut(name, "_")
		if applied[version] {
			continue
		}
		body, err := fs.ReadFile(migrationFS, path.Join(dir, name))
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, db, dialect, version, string(body)); err != nil {
			return fmt.Errorf("migration %s: %w", name, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, db *sql.DB, dialect, version, body string) error {
	conn, err := db.Conn(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// SQLite migrations that rebuild tables turn foreign keys off, but the
	// pragma is a no-op inside a transaction: dropping the old table would
	// cascade-delete every referencing row. Turn enforcement off on this
	// connection before the transaction, verify integrity before committing
	// and turn it back on afterwards.
	rebuild := dialect == DialectSQLite && strings.Contains(body, "PRAGMA foreign_keys = off")
	if rebuild {
		if _, err := conn.ExecContext(ctx, "PRAGMA foreign_keys = off"); err != nil {
			return err
		}
		defer func() { _, _ = conn.ExecContext(context.WithoutCancel(ctx), "PRAGMA foreign_keys = on") }()
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	// Both drivers execute multi-statement scripts when no arguments are
	// bound.
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	insert := `INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`
	if dialect == DialectPostgres {
		insert = `INSERT INTO schema_migrations (version, applied_at) VALUES ($1, $2)`
	}
	if _, err := tx.ExecContext(ctx, insert, version, time.Now().Unix()); err != nil {
		return err
	}
	if rebuild {
		if err := foreignKeyCheck(ctx, tx); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// foreignKeyCheck fails if any row violates a foreign key.
func foreignKeyCheck(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		return err
	}
	defer rows.Close()
	if rows.Next() {
		return fmt.Errorf("migration leaves rows violating foreign keys")
	}
	return rows.Err()
}

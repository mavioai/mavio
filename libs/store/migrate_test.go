package store

import (
	"database/sql"
	"path/filepath"
	"testing"
)

// TestSQLiteRebuildKeepsReferencingRows applies a migration rebuilding a
// table, as Atlas writes them for SQLite: dropping the old table must not
// cascade-delete the rows referencing it, and a rebuild leaving rows that
// violate foreign keys is rolled back.
func TestSQLiteRebuildKeepsReferencingRows(t *testing.T) {
	ctx := t.Context()
	db, err := sql.Open("sqlite3", "file:"+filepath.Join(t.TempDir(), "mavio.db")+"?_pragma=foreign_keys(1)")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `
		CREATE TABLE schema_migrations (version VARCHAR(32) NOT NULL PRIMARY KEY, applied_at BIGINT NOT NULL);
		CREATE TABLE parents (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE children (id INTEGER PRIMARY KEY, parent_id INTEGER NOT NULL REFERENCES parents (id) ON DELETE CASCADE);
		INSERT INTO parents (id, name) VALUES (1, 'a');
		INSERT INTO children (id, parent_id) VALUES (1, 1);`); err != nil {
		t.Fatal(err)
	}
	rebuild := func(keep string) string {
		return `PRAGMA foreign_keys = off;
			CREATE TABLE new_parents (id INTEGER PRIMARY KEY, name TEXT NOT NULL, note TEXT NULL);
			INSERT INTO new_parents (id, name) SELECT id, name FROM parents WHERE ` + keep + `;
			DROP TABLE parents;
			ALTER TABLE new_parents RENAME TO parents;
			PRAGMA foreign_keys = on;`
	}
	if err := applyMigration(ctx, db, DialectSQLite, "1", rebuild("true")); err != nil {
		t.Fatal(err)
	}
	var children int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM children`).Scan(&children); err != nil || children != 1 {
		t.Errorf("children after the rebuild = %d, %v; want = 1", children, err)
	}
	if err := applyMigration(ctx, db, DialectSQLite, "2", rebuild("false")); err == nil {
		t.Error("a rebuild orphaning children was applied")
	}
	var parents int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM parents`).Scan(&parents); err != nil || parents != 1 {
		t.Errorf("parents after the failed rebuild = %d, %v; want = 1", parents, err)
	}
}

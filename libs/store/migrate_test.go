package store

import (
	"database/sql"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// TestSQLiteUpgradeKeepsReferencingRows upgrades a database holding data
// in the initial schema: table rebuilds must not cascade-delete referencing
// rows, and the derived keys and inherited ratings must be backfilled.
func TestSQLiteUpgradeKeepsReferencingRows(t *testing.T) {
	ctx := t.Context()
	file := filepath.Join(t.TempDir(), "mavio.db")
	db, err := sql.Open("sqlite3", "file:"+file+"?_pragma=foreign_keys(1)&_timefmt=unixepoch_micro")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (version VARCHAR(32) NOT NULL PRIMARY KEY, applied_at BIGINT NOT NULL)`); err != nil {
		t.Fatal(err)
	}
	names, err := fs.Glob(migrationFS, "migrations/sqlite/*.sql")
	if err != nil || len(names) < 2 {
		t.Fatalf("migrations = %v, %v", names, err)
	}
	slices.Sort(names)
	initial, _ := fs.ReadFile(migrationFS, names[0])
	version, _, _ := strings.Cut(path.Base(names[0]), "_")
	if err := applyMigration(ctx, db, DialectSQLite, version, string(initial)); err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	libID, itemID, personID, trailerID := core.NewID(), core.NewID(), core.NewID(), core.NewID()
	for _, stmt := range []struct {
		query string
		args  []any
	}{
		{`INSERT INTO libraries (id, name, kind, paths, created_at, updated_at) VALUES (?, 'Films', 'movies', '[]', ?, ?)`, []any{libID, now, now}},
		{`INSERT INTO items (id, kind, name, sort_name, sort_key, original_title, date_added, library_id) VALUES (?, 'movie', 'The Spider-Man', 'The Spider-Man', 'the spider-man', 'Ｓｐｉｄｅｒ', ?, ?)`, []any{itemID, now, libID}},
		{`UPDATE items SET parental_rating = 13 WHERE id = ?`, []any{itemID}},
		{`INSERT INTO items (id, kind, name, sort_name, sort_key, date_added, library_id, parent_id) VALUES (?, 'video', 'Trailer', 'Trailer', 'trailer', ?, ?, ?)`, []any{trailerID, now, libID, itemID}},
		{`INSERT INTO item_values (kind, value, ord, item_id) VALUES ('genre', 'Comédie', 0, ?)`, []any{itemID}},
		{`INSERT INTO people (id, name, name_key) VALUES (?, 'Zoë Kravitz', 'zoë kravitz')`, []any{personID}},
		{`INSERT INTO credits (kind, item_id, person_id) VALUES ('actor', ?, ?)`, []any{itemID, personID}},
	} {
		if _, err := db.ExecContext(ctx, stmt.query, stmt.args...); err != nil {
			t.Fatalf("%s: %v", stmt.query, err)
		}
	}
	db.Close()

	s, err := Open(ctx, "sqlite:"+file)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	defer s.Close()

	values, err := s.Items().Values(ctx, core.ValueQuery{Kind: core.ValueGenre, Search: "comedie"})
	if err != nil || !slices.Equal(values, []string{"Comédie"}) {
		t.Errorf("genres after upgrade = %q, %v", values, err)
	}
	credits, err := s.People().CreditsForItem(ctx, itemID)
	if err != nil || len(credits) != 1 {
		t.Errorf("credits after upgrade = %v, %v", credits, err)
	}
	page, err := s.Items().Query(ctx, core.ItemQuery{Search: "spiderman"})
	if err != nil || page.Total != 1 {
		t.Errorf("sort-form search after upgrade = %d items, %v", page.Total, err)
	}
	page, err = s.Items().Query(ctx, core.ItemQuery{Search: "spider", Genres: []string{"comédie"}})
	if err != nil || page.Total != 1 {
		t.Errorf("search with genre after upgrade = %d items, %v", page.Total, err)
	}
	if child, err := s.Items().Get(ctx, trailerID); err != nil || child.InheritedRating != 13 {
		t.Errorf("inherited rating after upgrade = %d, %v; want 13", child.InheritedRating, err)
	}
	people, err := s.People().Search(ctx, core.PersonQuery{Search: "zoe"})
	if err != nil || len(people) != 1 {
		t.Errorf("person search after upgrade = %v, %v", people, err)
	}
}

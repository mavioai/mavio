package store

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	entschema "entgo.io/ent/dialect/sql/schema"

	"github.com/mavioai/mavio/libs/core"
	entmigrate "github.com/mavioai/mavio/libs/store/internal/ent/migrate"
)

// DumpFormat is the version of the dump format Dump writes.
const DumpFormat = 1

// DumpInfo describes a dump.
type DumpInfo struct {
	Format  int    `json:"format"`
	Dialect string `json:"dialect"`
	// Migrations are the schema migrations the database had.
	Migrations []string `json:"migrations"`
	// Rows counts the rows of each table.
	Rows map[string]int `json:"rows"`
}

// A dump holds a table's rows as JSON lines: the first line lists the
// columns, each further line a row's values, each value an object tagged
// with its type ({"s": "text"}, {"i": "42"}, {"f": 1.5}, {"b": true},
// {"x": "<base64>"}, {"t": "<RFC 3339>"}) or null.

// tables returns the tables in an order where every table comes after
// those its rows reference.
func tables() ([]*entschema.Table, error) {
	var out []*entschema.Table
	done := map[string]bool{}
	var visit func(t *entschema.Table, path []string) error
	visit = func(t *entschema.Table, path []string) error {
		if done[t.Name] {
			return nil
		}
		if slices.Contains(path, t.Name) {
			return fmt.Errorf("foreign keys form a cycle through %s", strings.Join(append(path, t.Name), " → "))
		}
		for _, fk := range t.ForeignKeys {
			if fk.RefTable != nil && fk.RefTable != t {
				if err := visit(fk.RefTable, append(path, t.Name)); err != nil {
					return err
				}
			}
		}
		done[t.Name] = true
		out = append(out, t)
		return nil
	}
	for _, t := range entmigrate.Tables {
		if err := visit(t, nil); err != nil {
			return nil, err
		}
	}
	return out, nil
}

// quote quotes an identifier, alike in both dialects.
func quote(name string) string { return `"` + strings.ReplaceAll(name, `"`, `""`) + `"` }

// Dump writes the rows of every table, from one consistent snapshot, to
// the writers create returns by file name ("<table>.jsonl").
func (s *Store) Dump(ctx context.Context, create func(name string) (io.Writer, error)) (DumpInfo, error) {
	order, err := tables()
	if err != nil {
		return DumpInfo{}, err
	}
	db := s.dbs[len(s.dbs)-1]
	opts := &sql.TxOptions{ReadOnly: true}
	if s.dialect == DialectPostgres {
		opts.Isolation = sql.LevelRepeatableRead
	}
	tx, err := db.BeginTx(ctx, opts)
	if err != nil {
		return DumpInfo{}, err
	}
	defer func() { _ = tx.Rollback() }()
	info := DumpInfo{Format: DumpFormat, Dialect: s.dialect, Rows: map[string]int{}}
	if info.Migrations, err = migrations(ctx, tx); err != nil {
		return DumpInfo{}, err
	}
	for _, t := range order {
		w, err := create(t.Name + ".jsonl")
		if err != nil {
			return DumpInfo{}, err
		}
		n, err := dumpTable(ctx, tx, t.Name, w)
		if err != nil {
			return DumpInfo{}, fmt.Errorf("dump %s: %w", t.Name, err)
		}
		info.Rows[t.Name] = n
	}
	return info, nil
}

// migrations lists the schema migrations applied, without the backfills
// recorded beside them.
func migrations(ctx context.Context, q interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
},
) ([]string, error) {
	rows, err := q.QueryContext(ctx, `SELECT version FROM schema_migrations ORDER BY version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		if strings.Trim(v, "0123456789") == "" {
			out = append(out, v)
		}
	}
	return out, rows.Err()
}

func dumpTable(ctx context.Context, tx *sql.Tx, table string, w io.Writer) (int, error) {
	rows, err := tx.QueryContext(ctx, "SELECT * FROM "+quote(table))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	cols, err := rows.ColumnTypes()
	if err != nil {
		return 0, err
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = c.Name()
	}
	enc := json.NewEncoder(w)
	if err := enc.Encode(names); err != nil {
		return 0, err
	}
	n := 0
	values := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range values {
		ptrs[i] = &values[i]
	}
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return n, err
		}
		row := make([]map[string]any, len(values))
		for i, v := range values {
			if row[i], err = encodeValue(v, cols[i].DatabaseTypeName()); err != nil {
				return n, fmt.Errorf("column %s: %w", names[i], err)
			}
		}
		if err := enc.Encode(row); err != nil {
			return n, err
		}
		n++
	}
	return n, rows.Err()
}

func encodeValue(v any, dbType string) (map[string]any, error) {
	textual := strings.Contains(strings.ToUpper(dbType), "JSON")
	switch v := v.(type) {
	case nil:
		return nil, nil
	case int64:
		return map[string]any{"i": strconv.FormatInt(v, 10)}, nil
	case float64:
		return map[string]any{"f": v}, nil
	case bool:
		return map[string]any{"b": v}, nil
	case string:
		return map[string]any{"s": v}, nil
	case []byte:
		if textual {
			return map[string]any{"s": string(v)}, nil
		}
		return map[string]any{"x": base64.StdEncoding.EncodeToString(v)}, nil
	case [16]byte: // UUIDs from PostgreSQL
		return map[string]any{"s": core.ID(v).String()}, nil
	case time.Time:
		return map[string]any{"t": v.Format(time.RFC3339Nano)}, nil
	}
	return nil, fmt.Errorf("unsupported value %T", v)
}

func decodeValue(m map[string]any) (any, error) {
	if m == nil {
		return nil, nil
	}
	for tag, v := range m {
		switch tag {
		case "i":
			s, _ := v.(string)
			return strconv.ParseInt(s, 10, 64)
		case "f", "b":
			return v, nil
		case "s":
			return v, nil
		case "x":
			s, _ := v.(string)
			return base64.StdEncoding.DecodeString(s)
		case "t":
			s, _ := v.(string)
			return time.Parse(time.RFC3339Nano, s)
		}
	}
	return nil, fmt.Errorf("untagged value %v", m)
}

// Restore loads a dump into this database, which must hold no users and no
// libraries, and whose schema must have every migration the dumped one
// had. open returns a table's file, or core.ErrNotFound for a table the
// dump lacks, which then stays empty. Derived keys are computed again.
func (s *Store) Restore(ctx context.Context, info DumpInfo, open func(name string) (io.ReadCloser, error)) error {
	switch {
	case info.Format != DumpFormat:
		return fmt.Errorf("%w: dump format %d, want %d", core.ErrInvalid, info.Format, DumpFormat)
	case info.Dialect != s.dialect:
		return fmt.Errorf("%w: a %s dump cannot be restored into %s", core.ErrInvalid, info.Dialect, s.dialect)
	}
	have, err := migrations(ctx, s.dbs[0])
	if err != nil {
		return err
	}
	for _, m := range info.Migrations {
		if !slices.Contains(have, m) {
			return fmt.Errorf("%w: the dump has migration %s, which this server lacks; restore it with a newer server", core.ErrInvalid, m)
		}
	}
	order, err := tables()
	if err != nil {
		return err
	}
	err = s.writeTx(ctx, func(tx *Store) error {
		for _, t := range []string{"users", "libraries"} {
			var n int
			if err := tx.tx.QueryRowContext(ctx, "SELECT COUNT(*) FROM "+quote(t)).Scan(&n); err != nil {
				return err
			}
			if n > 0 {
				return fmt.Errorf("%w: the database is not empty", core.ErrConflict)
			}
		}
		for _, t := range order {
			r, err := open(t.Name + ".jsonl")
			if errors.Is(err, core.ErrNotFound) {
				continue
			} else if err != nil {
				return err
			}
			err = tx.restoreTable(ctx, t, r)
			r.Close()
			if err != nil {
				return fmt.Errorf("restore %s: %w", t.Name, err)
			}
		}
		// The keys are derived again by the current rules.
		_, err := tx.tx.ExecContext(ctx, `DELETE FROM schema_migrations WHERE version LIKE 'keys-%' OR version LIKE 'ratings-%'`)
		return err
	})
	if err != nil {
		return err
	}
	return s.backfillKeys(ctx)
}

func (s *Store) restoreTable(ctx context.Context, t *entschema.Table, r io.Reader) error {
	dec := json.NewDecoder(bufio.NewReader(r))
	var names []string
	if err := dec.Decode(&names); err != nil {
		return err
	}
	for _, n := range names {
		if !slices.ContainsFunc(t.Columns, func(c *entschema.Column) bool { return c.Name == n }) {
			return fmt.Errorf("%w: unknown column %s", core.ErrInvalid, n)
		}
	}
	var rows [][]any
	for dec.More() {
		var raw []map[string]any
		if err := dec.Decode(&raw); err != nil {
			return err
		}
		if len(raw) != len(names) {
			return fmt.Errorf("%w: a row of %d values for %d columns", core.ErrInvalid, len(raw), len(names))
		}
		row := make([]any, len(raw))
		for i, v := range raw {
			var err error
			if row[i], err = decodeValue(v); err != nil {
				return err
			}
		}
		rows = append(rows, row)
	}
	rows = parentsFirst(t, names, rows)
	quoted := make([]string, len(names))
	params := make([]string, len(names))
	for i, n := range names {
		quoted[i] = quote(n)
		params[i] = "?"
		if s.dialect == DialectPostgres {
			params[i] = "$" + strconv.Itoa(i+1)
		}
	}
	insert := "INSERT INTO " + quote(t.Name) + " (" + strings.Join(quoted, ", ") + ") VALUES (" + strings.Join(params, ", ") + ")"
	stmt, err := s.tx.PrepareContext(ctx, insert)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, row := range rows {
		if _, err := stmt.ExecContext(ctx, row...); err != nil {
			return err
		}
	}
	return nil
}

// parentsFirst orders the rows of a table referencing itself, such as
// items and their parents, so that each comes after the rows it
// references.
func parentsFirst(t *entschema.Table, names []string, rows [][]any) [][]any {
	key := slices.Index(names, t.PrimaryKey[0].Name)
	var refs []int
	for _, fk := range t.ForeignKeys {
		if fk.RefTable == t {
			refs = append(refs, slices.Index(names, fk.Columns[0].Name))
		}
	}
	if key < 0 || len(refs) == 0 || slices.Contains(refs, -1) {
		return rows
	}
	id := func(v any) string {
		if b, ok := v.([]byte); ok {
			return string(b)
		}
		return fmt.Sprint(v)
	}
	byKey := make(map[string]int, len(rows))
	for i, r := range rows {
		byKey[id(r[key])] = i
	}
	placed := make([]bool, len(rows))
	out := make([][]any, 0, len(rows))
	var place func(i int, depth int)
	place = func(i, depth int) {
		if placed[i] || depth > len(rows) {
			return
		}
		placed[i] = true
		for _, c := range refs {
			if j, ok := byKey[id(rows[i][c])]; ok && rows[i][c] != nil {
				place(j, depth+1)
			}
		}
		out = append(out, rows[i])
	}
	for i := range rows {
		place(i, 0)
	}
	return out
}

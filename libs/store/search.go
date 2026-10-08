package store

import (
	"strings"

	entsql "entgo.io/ent/dialect/sql"
)

// search is a parsed search term, matched as Jellyfin's built-in search
// provider matches it:
//
//   - the clean form of the term (cleanValue) as a substring of the clean
//     form of the name;
//   - the raw term (trimmed, lower case, no diacritics) as a LIKE pattern
//     against the original title, so % and _ act as wildcards there;
//   - the sort form of the term (sortKey) as a LIKE pattern against the
//     sort name, which finds "Spider-Man" from "spiderman" and Chinese
//     titles from pinyin.
type search struct {
	// clean is the clean form of the term, which ranks results.
	clean string
	// cleanPattern, rawPattern and sortPattern are LIKE patterns;
	// sortPattern is empty when the term has no sort form.
	cleanPattern, rawPattern, sortPattern string
}

// parseSearch parses term; ok is false when its clean form is empty.
func parseSearch(term string) (search, bool) {
	raw := strings.TrimSpace(term)
	clean := cleanValue(raw)
	if clean == "" {
		return search{}, false
	}
	s := search{
		clean:        clean,
		cleanPattern: "%" + likeEscape(clean) + "%",
		rawPattern:   "%" + likeRaw(lowerFold(raw)) + "%",
	}
	if sk := sortKey(raw); sk != "" {
		s.sortPattern = "%" + likeRaw(sk) + "%"
	}
	return s, true
}

// match returns the search condition. cleanCol holds the clean form of the
// name; originalCol (optional) the lowerFold form of the original title;
// sortCol the sort key.
func (q search) match(cleanCol, originalCol, sortCol string) *entsql.Predicate {
	return entsql.P(func(b *entsql.Builder) {
		b.WriteByte('(')
		like(b, cleanCol, q.cleanPattern)
		if originalCol != "" {
			b.WriteString(" OR (").WriteString(originalCol).WriteString(" <> '' AND ")
			like(b, originalCol, q.rawPattern)
			b.WriteByte(')')
		}
		if q.sortPattern != "" {
			b.WriteString(" OR ")
			like(b, sortCol, q.sortPattern)
		}
		b.WriteByte(')')
	})
}

// rank returns the relevance of a row by its clean name, with Jellyfin's
// tiers: 0 for an exact match, 1 when the name starts with the term, 2 when
// it contains the term followed by a space (a word prefix), 3 otherwise.
// Lower ranks sort first.
func (q search) rank(cleanCol string) entsql.Querier {
	escaped := likeEscape(q.clean)
	return entsql.ExprFunc(func(b *entsql.Builder) {
		b.WriteString("CASE WHEN ").WriteString(cleanCol).WriteString(" = ").Arg(q.clean).WriteString(" THEN 0 WHEN ")
		like(b, cleanCol, escaped+"%")
		b.WriteString(" THEN 1 WHEN ")
		like(b, cleanCol, "%"+escaped+" %")
		b.WriteString(" THEN 2 ELSE 3 END")
	})
}

// like writes "col LIKE pattern" with backslash as the escape character.
func like(b *entsql.Builder, col, pattern string) {
	b.WriteString(col).WriteString(" LIKE ").Arg(pattern).WriteString(` ESCAPE '\'`)
}

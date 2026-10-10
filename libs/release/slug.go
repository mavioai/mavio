package release

import (
	"strings"
)

var (
	whitespaceRegex  = re(`\s`)
	invalidSlugChars = re(`[^a-z0-9\s-_]`)
)

// URLSlug returns value as a lowercase URL slug without accents. Invalid
// characters are removed, or replaced with dashes when invalidDash is set;
// trimChars are trimmed from both ends and runs of dedupeChars are
// collapsed.
func URLSlug(value string, invalidDash bool, trimChars, dedupeChars string) string {
	value = RemoveAccent(strings.ToLower(value))
	value = whitespaceRegex.replace(value, "-")
	replacement := ""
	if invalidDash {
		replacement = "-"
	}
	value = invalidSlugChars.replace(value, replacement)
	if trimChars != "" {
		value = strings.Trim(value, trimChars)
	}
	if dedupeChars != "" {
		value = re(`([`+dedupeChars+`]){2,}`).replace(value, "$1")
	}
	return value
}

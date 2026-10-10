package release

import (
	"slices"
	"strings"
	"testing"
)

// sameElements reports whether a and b hold the same elements in any order,
// like FluentAssertions' BeEquivalentTo.
func sameElements[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	rest := slices.Clone(b)
	for _, v := range a {
		i := slices.Index(rest, v)
		if i < 0 {
			return false
		}
		rest = slices.Delete(rest, i, i+1)
	}
	return true
}

// languageNamed converts a language name like Sonarr's explicit string
// operator.
func languageNamed(t *testing.T, name string) Language {
	t.Helper()
	for i, n := range languageNames {
		if strings.EqualFold(n, name) {
			return Language(i)
		}
	}
	t.Fatalf("unknown language %q", name)
	return LanguageUnknown
}

// languageSymbols maps Sonarr's and Radarr's Language members.
var languageSymbols = func() map[string]Language {
	m := map[string]Language{"Unknown": LanguageUnknown, "PortugueseBR": PortugueseBrazil}
	for i, n := range languageNames {
		m[strings.NewReplacer(" ", "", "(", "", ")", "").Replace(n)] = Language(i)
	}
	return m
}()

// languageOfMethod returns the language a should_parse_language_<name>
// theory expects.
func languageOfMethod(t *testing.T, method string) Language {
	t.Helper()
	name := strings.TrimPrefix(method, "should_parse_language_")
	var b strings.Builder
	for w := range strings.SplitSeq(name, "_") {
		b.WriteString(strings.ToUpper(w[:1]) + w[1:])
	}
	name = b.String()
	if name == "BrazilianPortuguese" {
		name = "PortugueseBrazil"
	}
	l, ok := languageSymbols[name]
	if !ok {
		t.Fatalf("no language for %s", method)
	}
	return l
}

// osAgnostic evaluates a test path expression such as
// @"C:\Test\file.mkv".AsOsAgnostic(), keeping Windows separators, which the
// parser accepts on every platform.
func osAgnostic(t *testing.T, a args, name string) string {
	t.Helper()
	if !strings.HasPrefix(string(a[name]), "{") {
		return a.str(t, name)
	}
	e := a.expr(t, name)
	if !strings.HasPrefix(e, `@"`) || !strings.HasSuffix(e, `".AsOsAgnostic()`) {
		t.Fatalf("unsupported expression %s", e)
	}
	return strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(e, `@"`), `".AsOsAgnostic()`), `""`, `"`)
}

func check[T comparable](t *testing.T, what string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got = %v, want = %v", what, got, want)
	}
}

func checkSame[T comparable](t *testing.T, what string, got, want []T) {
	t.Helper()
	if !sameElements(got, want) {
		t.Errorf("%s: got = %v, want = %v", what, got, want)
	}
}

package library

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
)

func (a args) optStr(t *testing.T, name string) string {
	t.Helper()
	if a.null(name) {
		return ""
	}
	return a.str(t, name)
}

func TestPathExtensionsCases(t *testing.T) {
	portedCases(t, "path_extensions.json", ported{
		run: map[string]func(t *testing.T, a args){
			"GetAttributeValue_ValidArgs_Correct": func(t *testing.T, a args) {
				got, err := AttributeValue(a.str(t, "input"), a.str(t, "attribute"))
				if want := a.optStr(t, "expectedResult"); err != nil || got != want {
					t.Errorf("got = %q, %v, want = %q", got, err, want)
				}
			},
			"GetAttributeValue_EmptyString_ThrowsArgumentException": func(t *testing.T, a args) {
				if _, err := AttributeValue(a.str(t, "input"), a.str(t, "attribute")); !errors.Is(err, ErrEmpty) {
					t.Errorf("got = %v, want = %v", err, ErrEmpty)
				}
			},
			"TryReplaceSubPath_ValidArgs_Correct": func(t *testing.T, a args) {
				got, ok := ReplaceSubPath(a.str(t, "path"), a.str(t, "subPath"), a.str(t, "newSubPath"))
				if want := a.str(t, "expectedResult"); !ok || got != want {
					t.Errorf("got = %q, %v, want = %q", got, ok, want)
				}
			},
			"TryReplaceSubPath_InvalidInput_ReturnsFalseAndNull": func(t *testing.T, a args) {
				if got, ok := ReplaceSubPath(a.optStr(t, "path"), a.optStr(t, "subPath"), a.optStr(t, "newSubPath")); ok || got != "" {
					t.Errorf("got = %q, %v, want = failure", got, ok)
				}
			},
			"NormalizePath_SpecifyingSeparator_Normalizes": func(t *testing.T, a args) {
				if got, want := NormalizePath(a.optStr(t, "path"), a.str(t, "separator")[0]), a.optStr(t, "expectedPath"); got != want {
					t.Errorf("got = %q, want = %q", got, want)
				}
			},
			"NormalizePath_NoArgs_UsesDirectorySeparatorChar": func(t *testing.T, a args) {
				path := a.str(t, "path")
				sep := string(filepath.Separator)
				want := strings.NewReplacer(`\`, sep, "/", sep).Replace(path)
				if got := NormalizePath(path, filepath.Separator); got != want {
					t.Errorf("got = %q, want = %q", got, want)
				}
			},
			"NormalizePath_OutVar_Correct": func(t *testing.T, a args) {
				path, want := a.str(t, "path"), a.str(t, "expectedSeparator")[0]
				got, sep := NormalizePathDetect(path)
				if sep != want || got != strings.NewReplacer(`\`, string(want), "/", string(want)).Replace(path) {
					t.Errorf("got = %q %q, want separator %q", got, sep, want)
				}
			},
		},
		facts: map[string]string{"NormalizePath_SpecifyInvalidSeparator_ThrowsException": "TestNormalizePathInvalidSeparator"},
	})
}

func TestNormalizePathInvalidSeparator(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("got = no panic")
		}
	}()
	NormalizePath("", 'a')
}

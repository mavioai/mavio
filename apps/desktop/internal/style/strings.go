package style

import (
	"sync"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"
)

var locales sync.Map // the app's language → the strings' locale

// T returns the string key in the language the app shows, with args given
// as names followed by their values.
func T(c *ui.Context, key string, args ...any) string {
	return Format(locale(c.Locale()), key, args...)
}

// Format returns the string key of locale, one of design.Locales, with args
// given as names followed by their values.
func Format(locale, key string, args ...any) string {
	var m map[string]any
	if len(args) > 0 {
		m = make(map[string]any, len(args)/2)
		for i := 0; i+1 < len(args); i += 2 {
			m[args[i].(string)] = args[i+1]
		}
	}
	return design.Format(locale, key, m)
}

func locale(language string) string {
	if l, ok := locales.Load(language); ok {
		return l.(string)
	}
	l := design.Match(language)
	locales.Store(language, l)
	return l
}

package style

import (
	"embed"
	"regexp"
	"sync"

	"github.com/egoist/mygo/ui"
)

//go:embed services/*.svg
var serviceFiles embed.FS

var (
	services sync.Map // logo name and whether mono → *ui.SVG
	fills    = regexp.MustCompile(`fill="[^"]*"`)
)

// Service returns the logo of a service elsewhere, as the web UI's
// (services/NOTICE): imdb, letterboxd, rt, rt-fresh, rt-rotten,
// rt-aud-fresh, rt-aud-rotten, simkl, tmdb, tmdb-wide, trakt or tvdb. Mono
// is the logo as a shape alone, in currentColor, to draw with ui.Icon and
// a text color, as the web UI masks the logos white alone. It panics for a
// name it does not have.
func Service(name string, mono bool) *ui.SVG {
	key := name
	if mono {
		key += ":mono"
	}
	if s, ok := services.Load(key); ok {
		return s.(*ui.SVG)
	}
	b, err := serviceFiles.ReadFile("services/" + name + ".svg")
	if err != nil {
		panic("style: no service " + name)
	}
	if mono {
		b = fills.ReplaceAll(b, []byte(`fill="currentColor"`))
	}
	s, _ := services.LoadOrStore(key, ui.MustParseSVG(b))
	return s.(*ui.SVG)
}

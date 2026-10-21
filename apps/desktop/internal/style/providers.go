package style

import (
	"sync"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"
)

var providers sync.Map // provider ID → providerLogo

type providerLogo struct {
	svg  *ui.SVG
	mono bool
}

// ProviderLogo returns an AI provider's logo, from libs/design/providers,
// and whether it draws in the text's color (currentColor), for ui.Icon,
// rather than its own, for ui.Image; nil for a provider without one.
func ProviderLogo(id string) (*ui.SVG, bool) {
	if l, ok := providers.Load(id); ok {
		return l.(providerLogo).svg, l.(providerLogo).mono
	}
	var l providerLogo
	if p, ok := design.ProviderLogoOf(id); ok {
		if s, err := ui.ParseSVG([]byte(p.SVG)); err == nil {
			l = providerLogo{s, p.Monochrome}
		}
	}
	providers.Store(id, l)
	return l.svg, l.mono
}

package style

import (
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"
)

// Layout returns the layout of the frame's form factor: television's in
// TV mode (TV), else the desktop's.
func Layout(c *ui.Context) *design.Layout {
	if t := c.Theme(); t == paneThemes()[0] || t == paneThemes()[1] {
		return &tvPaneLayout
	}
	if TV(c) {
		return &design.TvLayout
	}
	return &design.DesktopLayout
}

// Type returns the type scale of the frame's form factor: television's in
// TV mode, else the desktop's, with a body of 13 as on the Mac.
func Type(c *ui.Context) *design.Typography {
	if TV(c) {
		return &design.TvTypography
	}
	return &design.DesktopTypography
}

// TV reports whether the frame is in TV mode (docs/tv.md): built with a
// theme of Theme's for television.
func TV(c *ui.Context) bool {
	t := c.Theme()
	tv, pane := tvThemes(), paneThemes()
	return t == tv[0] || t == tv[1] || t == pane[0] || t == pane[1]
}

// tvPaneLayout is the layout of a pane of the TV mode beside another,
// whose margin keeps only a little room for a focused card to grow.
var tvPaneLayout = func() design.Layout {
	l := design.TvLayout
	l.Margin = 24
	return l
}()

// Pane returns the theme of a pane of the TV mode beside another, as
// Search's results beside its keyboard (docs/tv.md §6.4), the dark one
// when dark: a theme for television whose layout's margin is 24.
func Pane(dark bool) *ui.Theme {
	return paneThemes()[1-b2i(dark)]
}

// Color returns c as MyGo's color.
func Color(c design.Color) ui.Color {
	return ui.RGBA(c.R, c.G, c.B, float32(c.A)/255)
}

// Colors returns the design system's colors of the frame's theme.
func Colors(c *ui.Context) *design.Colors {
	dark := c.Theme().Dark
	if TV(c) {
		return tvColors()[b2i(dark)]
	}
	if dark {
		return &design.DarkColors
	}
	return &design.LightColors
}

// tvColors are the light and the dark colors of the TV mode, whose
// background is the television's (docs/tv.md §2).
var tvColors = sync.OnceValue(func() [2]*design.Colors {
	light, dark := design.LightColors, design.DarkColors
	light.Background, dark.Background = light.TvBackground, dark.TvBackground
	return [2]*design.Colors{&light, &dark}
})

func b2i(b bool) int {
	if b {
		return 1
	}
	return 0
}

var (
	themes   = sync.OnceValue(func() [2]*ui.Theme { return [2]*ui.Theme{theme(true, false), theme(false, false)} })
	tvThemes = sync.OnceValue(func() [2]*ui.Theme { return [2]*ui.Theme{theme(true, true), theme(false, true)} })
	// paneThemes are the TV mode's for a pane beside another (Pane).
	paneThemes = sync.OnceValue(func() [2]*ui.Theme { return [2]*ui.Theme{theme(true, true), theme(false, true)} })
)

// Theme returns MyGo's theme of the dark or the light colors, for
// television in TV mode, whose type and layout Type and Layout then
// return.
func Theme(dark, tv bool) *ui.Theme {
	t := themes()
	if tv {
		t = tvThemes()
	}
	if dark {
		return t[0]
	}
	return t[1]
}

func theme(dark, tv bool) *ui.Theme {
	t, p := ui.LightTheme(), &design.LightColors
	if dark {
		t, p = ui.DarkTheme(), &design.DarkColors
	}
	t.Background = Color(p.Background)
	t.Surface = Color(p.Fill)
	t.SurfaceHover = Color(p.FillSelected)
	t.SurfacePressed = Color(p.FillSelected)
	t.Border = Color(p.Border)
	t.Text = Color(p.TextPrimary)
	t.TextMuted = Color(p.TextSecondary)
	t.Accent = Color(p.Accent)
	t.AccentHover = Color(p.AccentPressed)
	t.AccentPressed = Color(p.AccentPressed)
	t.AccentText = Color(p.OnAccent)
	t.Danger = Color(p.Danger)
	t.Warning = Color(p.Warning)
	t.Success = Color(p.Success)
	t.Focus = Color(p.Focus)
	t.Selection = Color(p.Accent).Alpha(0.35)
	t.Radius = design.RadiusMd
	t.FontSize = design.DesktopTypography.Body.Size
	if tv {
		t.Background = Color(p.TvBackground)
		t.FontSize = design.TvTypography.Body.Size
	}
	return t
}

// Text styles e as s of the type scale.
func Text(e ui.Element, s design.TypeStyle) ui.Element {
	return e.FontSize(s.Size).FontWeight(s.Weight).FixedLineHeight(s.LineHeight)
}

var icons sync.Map // icon name and weight → *ui.SVG

// Icon returns the icon name of the design system, in its Fill weight when
// fill is true; it panics for a name the design system does not have.
func Icon(name string, fill bool) *ui.SVG {
	key := name
	if fill {
		key += ":fill"
	}
	if s, ok := icons.Load(key); ok {
		return s.(*ui.SVG)
	}
	icon, ok := design.Icons[name]
	if !ok {
		panic("style: no icon " + name)
	}
	paths := icon.Regular
	if fill {
		paths = icon.Fill
	}
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 256 256">`)
	for _, d := range paths {
		b.WriteString(`<path fill="currentColor" d="`)
		b.WriteString(d)
		b.WriteString(`"/>`)
	}
	b.WriteString(`</svg>`)
	s, _ := icons.LoadOrStore(key, ui.MustParseSVG([]byte(b.String())))
	return s.(*ui.SVG)
}

// Wordmark is Mavio's wordmark in its gradient, as libs/design/brand has
// it.
var Wordmark = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 244 64"><defs><linearGradient id="fill" gradientUnits="userSpaceOnUse" x1="0" y1="0" x2="244" y2="64"><stop offset="0" stop-color="#FFD34E"/><stop offset="0.45" stop-color="#FF9A2E"/><stop offset="1" stop-color="#F0412F"/></linearGradient></defs><path d="M0 64V0H14L30 30L46 0H60V64H48V24L34 50H26L12 24V64ZM80 20H114V64H80L70 54V30ZM86 32L82 36V48L86 52H102V32ZM124 20H137L146 44.7L155 20H168L152 64H140ZM178 20H190V64H178ZM178 2H190V14H178ZM210 20H234L244 30V54L234 64H210L200 54V30ZM216 32L212 36V48L216 52H228L232 48V36L228 32Z" fill="url(#fill)" fill-rule="evenodd"/></svg>`))

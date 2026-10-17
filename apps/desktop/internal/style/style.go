package style

import (
	"strings"
	"sync"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"
)

// Layout is the desktop form factor's layout.
var Layout = design.DesktopLayout

// Type is the type scale.
var Type = design.StandardTypography

// Color returns c as MyGo's color.
func Color(c design.Color) ui.Color {
	return ui.RGBA(c.R, c.G, c.B, float32(c.A)/255)
}

// Colors returns the design system's colors of the frame's theme.
func Colors(c *ui.Context) *design.Colors {
	if c.Theme().Dark {
		return &design.DarkColors
	}
	return &design.LightColors
}

var themes = sync.OnceValues(func() (*ui.Theme, *ui.Theme) {
	return theme(true), theme(false)
})

// Theme returns MyGo's theme of the dark or the light colors.
func Theme(dark bool) *ui.Theme {
	d, l := themes()
	if dark {
		return d
	}
	return l
}

func theme(dark bool) *ui.Theme {
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
	t.FontSize = Type.Body.Size
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

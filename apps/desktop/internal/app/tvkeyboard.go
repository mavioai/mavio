package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// tvKeyboard is the TV mode's keyboard for fields (docs/tv.md §5), open
// on a field Return was pressed on: what it types goes into the field as
// it is typed, Done keeps it and Back puts back what the field held.
type tvKeyboard struct {
	open  bool
	field ui.TextField
	// original is what the field held, text what the keyboard typed.
	original, text string
	// upper types capitals, more the symbols.
	upper, more bool
	// typed is what a computer's keyboard typed into it since the last
	// frame (tvTyping).
	typed string
}

const (
	tvLetters = "abcdefghijklmnopqrstuvwxyz"
	tvDigits  = "1234567890"
	tvSymbols = "-_.@/:;,?!#$%&*+=()[]{}<>'\"~|\\"
)

// tvKeyboardKeys opens the keyboard with Return on a field no keyboard
// typed into, as a remote's OK, but one whose suggestions show, where it
// chooses one, and builds it while open.
func (a *App) tvKeyboardKeys(c *ui.Context) {
	k := &a.tvKeys
	if !k.open {
		f, ok := c.FocusedTextField()
		if !ok || f.Expanded || !c.Shortcut(0, ui.KeyEnter) {
			return
		}
		*k = tvKeyboard{open: true, field: f, original: f.Text, text: f.Text}
	}
	if t := k.typed; t != "" {
		k.typed = ""
		if k.field.Number {
			t = strings.Map(func(r rune) rune {
				if r >= '0' && r <= '9' || r == '-' && k.field.Signed || r == '.' && k.field.Decimal {
					return r
				}
				return -1
			}, t)
		}
		a.tvType(c, func(s string) string { return s + t })
	}
	open := true
	ui.DialogBase(c, &open, func(back, panel ui.Element) {
		back.Background(ui.RGBA(0, 0, 0, 0.5))
		a.tvKeyboardPanel(c, panel)
	})
	if !open {
		// Escape, and Backspace with it (ui.Context.SpatialFocus), is the
		// keyboard's Back.
		a.tvBackKey(c)
	}
}

// tvKeyboardPanel builds the keyboard's panel: the field's label, what is
// typed, the keys, and the keys of what they type, Delete, Clear and Done.
func (a *App) tvKeyboardPanel(c *ui.Context, panel ui.Element) {
	k := &a.tvKeys
	colors := style.Colors(c)
	width := float32(820)
	if k.field.Number {
		width = 440
	}
	w, _ := c.Size()
	panel.Width(min(width, w-32)).Padding(24).Gap(16).Radius(design.RadiusLg * 2).Background(style.Color(colors.SurfaceRaised)).Label(k.field.Label)
	panel.Shadow(0, 10, 30, 0, ui.RGBA(0, 0, 0, 0.3))
	panel.Children(func() {
		if k.field.Label != "" {
			style.Text(ui.Text(c, k.field.Label), style.Type(c).Title3).TextColor(style.Color(colors.TextSecondary))
		}
		shown := k.text
		if k.field.Password {
			shown = strings.Repeat("•", len([]rune(k.text)))
		}
		ui.Row(c).MinHeight(64).Padding(0, 20).Radius(12).AlignItems(ui.Center).Background(style.Color(colors.Surface)).Children(func() {
			t := style.Text(ui.Text(c, shown), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).Shrink(1).MinWidth(0)
			if !k.field.Multiline {
				t.SingleLine()
			}
			ui.Box(c).Width(2).Height(28).Margin(0, 1).Radius(1).Shrink(0).Background(style.Color(colors.Accent))
		})
		var chars []string
		cols := 10
		switch {
		case k.field.Number:
			cols = 3
			chars = strings.Split("123456789", "")
			if k.field.Signed {
				chars = append(chars, "-")
			}
			chars = append(chars, "0")
			if k.field.Decimal {
				chars = append(chars, ".")
			}
		case k.more:
			chars = strings.Split(tvSymbols, "")
		default:
			chars = strings.Split(tvDigits+tvLetters+"-_.@", "")
		}
		ui.Column(c).Gap(8).Children(func() {
			for i := 0; i < len(chars); i += cols {
				ui.Row(c.Key(i)).Gap(8).Children(func() {
					for j := i; j < i+cols; j++ {
						if j >= len(chars) {
							ui.Box(c.Key(j)).Grow(1).Basis(0)
							continue
						}
						ch := chars[j]
						if k.upper {
							ch = strings.ToUpper(ch)
						}
						if a.tvTyping(tvKey(c, "k"+chars[j], ch, false, j == 0)).Grow(1).Basis(0).Clicked() {
							a.tvType(c, func(s string) string { return s + ch })
						}
					}
				})
			}
		})
		ui.Row(c).Gap(8).Children(func() {
			if !k.field.Number {
				label := "ABC"
				if k.upper {
					label = "abc"
				}
				if b := a.tvTyping(tvKey(c, "upper", label, false, false)).Width(96).Disabled(k.more); b.Clicked() {
					k.upper = !k.upper
				}
				label = "#+="
				if k.more {
					label = "abc"
				}
				if a.tvTyping(tvKey(c, "more", label, false, false)).Width(96).Clicked() {
					k.more = !k.more
				}
				if a.tvTyping(tvKey(c, "space", style.T(c, "search.space"), false, false)).Grow(1).Basis(0).Clicked() {
					a.tvType(c, func(s string) string { return s + " " })
				}
				if k.field.Multiline && a.tvTyping(tvKey(c, "newline", "↵", false, false)).Width(80).Clicked() {
					a.tvType(c, func(s string) string { return s + "\n" })
				}
			}
			if a.tvTyping(tvKey(c, "delete", style.T(c, "search.backspace"), false, false)).Grow(1).Basis(0).Clicked() {
				a.tvType(c, dropLast)
			}
			if a.tvTyping(tvKey(c, "clear", style.T(c, "search.clear"), false, false)).Grow(1).Basis(0).Clicked() {
				a.tvType(c, func(string) string { return "" })
			}
			if a.tvTyping(tvKey(c, "done", style.T(c, "action.done"), true, false)).Grow(1).Basis(0).Clicked() {
				a.tvKeyboardClose(c)
			}
		})
	})
}

// tvTyping has key b take what a computer's keyboard types into the
// keyboard while it has focus (docs/tv.md §4), the characters of the
// keyboard's layout, Space typing a space rather than pressing the key;
// Return still presses it, and Backspace is Back.
func (a *App) tvTyping(b ui.Element) ui.Element {
	k := &a.tvKeys
	return b.TextCaret(ui.Rect{W: 1, H: 1}).InputMethod(false).HandleInput(func(ev ui.InputEvent) bool {
		switch ev.Kind {
		case ui.InputKeyDown:
			return ev.Key == ui.KeySpace && ev.Mods == 0
		case ui.InputText:
			k.typed += ev.Text
			return true
		}
		return false
	})
}

// tvKey builds a key of the keyboard, the accent's when primary, white
// with focus, with focus as the keyboard opens when first.
func tvKey(c *ui.Context, key, label string, primary, first bool) ui.Element {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(key)).Height(56).Radius(10).Center().Label(label).FocusRing(false)
	if first {
		b.AutoFocus()
	}
	bg, fg := style.Color(colors.Fill), style.Color(colors.TextPrimary)
	if primary {
		bg, fg = style.Color(colors.Accent), style.Color(colors.OnAccent)
	}
	if b.Focused() {
		bg, fg = ui.RGB(255, 255, 255), ui.RGB(0, 0, 0)
		b.Shadow(0, 8, 24, 0, style.Color(colors.TvFocusShadow))
	}
	b.Background(bg).Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Headline).TextColor(fg)
	})
	return b
}

// tvType changes what the keyboard typed by f, into the field too.
func (a *App) tvType(c *ui.Context, f func(string) string) {
	k := &a.tvKeys
	k.text = f(k.text)
	c.SetFieldText(k.field, k.text)
}

// tvBackKey is Back on the keyboard: it deletes a character, and closes
// the keyboard once there is none, putting back what the field held.
func (a *App) tvBackKey(c *ui.Context) {
	k := &a.tvKeys
	if k.text != "" {
		a.tvType(c, dropLast)
		return
	}
	c.SetFieldText(k.field, k.original)
	a.tvKeyboardClose(c)
}

// tvKeyboardClose closes the keyboard, focus back on the field, to move
// on from or type into again.
func (a *App) tvKeyboardClose(c *ui.Context) {
	c.FocusField(a.tvKeys.field)
	a.tvKeys = tvKeyboard{}
}

// dropLast returns s without its last character.
func dropLast(s string) string {
	r := []rune(s)
	if len(r) == 0 {
		return s
	}
	return string(r[:len(r)-1])
}

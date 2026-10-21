package app

import (
	"strings"

	"github.com/egoist/mygo/ui"

	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// modelOption is a model to choose, as the web UI's model picker lists
// it: its value, its name, a line of its details, and its provider's ID
// and name; no provider for a choice of none.
type modelOption struct {
	value, label, detail   string
	provider, providerName string
}

// modelOptions are the models of the providers that can be used, by
// prefix + "provider/model", the providers named as names has them.
func modelOptions(c *ui.Context, prefix string, models []*pluginv1.AiModel, names map[string]string) []modelOption {
	out := make([]modelOption, 0, len(models))
	for _, m := range models {
		out = append(out, modelOption{
			value:        prefix + m.GetProvider() + "/" + m.GetId(),
			label:        cmpOr(m.GetName(), m.GetId()),
			detail:       modelDetail(c, m),
			provider:     m.GetProvider(),
			providerName: cmpOr(names[m.GetProvider()], m.GetProvider()),
		})
	}
	return out
}

// modelsShown is how many matching models the picker lists at most; more
// are found by typing.
const modelsShown = 200

// providerLogo builds an AI provider's logo, in its own colors or the
// text's for a monochrome one; a chip for a provider without.
func providerLogo(c *ui.Context, provider string, size float32) {
	colors := style.Colors(c)
	svg, mono := style.ProviderLogo(provider)
	switch {
	case svg == nil:
		ui.Icon(c, style.Icon("cpu", false)).Size(size, size).Shrink(0).TextColor(style.Color(colors.TextTertiary))
	case mono:
		ui.Icon(c, svg).Size(size, size).Shrink(0).TextColor(style.Color(colors.TextPrimary))
	default:
		ui.Image(c, svg).Size(size, size).Shrink(0)
	}
}

// modelLabel builds a model as the picker's rows show it first: its
// provider's logo, its name and, smaller and quieter, its provider's.
func modelLabel(c *ui.Context, o modelOption, fg ui.Color) {
	colors := style.Colors(c)
	if o.provider != "" {
		providerLogo(c, o.provider, 16)
	}
	style.Text(ui.Text(c, o.label), style.Type(c).Callout).TextColor(fg).SingleLine().Shrink(1).MinWidth(0)
	if o.providerName != "" {
		ui.Text(c, o.providerName).FontSize(11).TextColor(style.Color(colors.TextTertiary)).Opacity(0.75).SingleLine().Shrink(0)
	}
}

// modelMatches reports whether a model's name, value or provider holds
// what was typed.
func modelMatches(o modelOption, q string) bool {
	q = strings.ToLower(strings.TrimSpace(q))
	return q == "" || strings.Contains(strings.ToLower(o.label), q) || strings.Contains(strings.ToLower(o.value), q) ||
		strings.Contains(strings.ToLower(o.providerName), q)
}

// modelPicker builds a choice of *value among options, as the web UI's:
// a button showing the model chosen, or placeholder, whose popup searches
// them, in rows of the provider's logo, the model's name and its
// provider's, and the model's details under them. It reports a new
// choice.
func modelPicker(c *ui.Context, label, placeholder string, value *string, options []modelOption, disabled bool, width float32) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key("model-"+label)).MinHeight(28).Width(width).Padding(0, 6, 0, 8).Gap(6).Radius(6).AlignItems(ui.Center).
		Label(label).Disabled(disabled)
	type state struct {
		open      bool
		query     string
		highlight int
	}
	st := ui.Local(b, "picker", func() state { return state{} })
	if disabled {
		b.Opacity(0.5)
	}
	if st.open || (b.Hovered() && !disabled) {
		b.Background(style.Color(colors.FillSelected))
	} else {
		b.Background(style.Color(colors.Fill))
	}
	var chosen *modelOption
	for i := range options {
		if options[i].value == *value {
			chosen = &options[i]
		}
	}
	b.Children(func() {
		if chosen != nil {
			modelLabel(c, *chosen, style.Color(colors.TextPrimary))
		} else {
			style.Text(ui.Text(c, placeholder), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).SingleLine().Shrink(1).MinWidth(0)
		}
		ui.Box(c).Grow(1)
		ui.Icon(c, style.Icon("expand", false)).Size(14, 14).Shrink(0).TextColor(style.Color(colors.TextSecondary))
	})
	if b.Clicked() {
		st.open, st.query, st.highlight = !st.open, "", 0
	}
	changed := false
	ui.PopoverBase(c, b, &st.open, func(panel ui.Element) {
		menuPanel(c, panel, max(width, 256), 0.9, 24, func() {
			var shown []modelOption
			for _, o := range options {
				if modelMatches(o, st.query) {
					shown = append(shown, o)
				}
			}
			found := len(shown)
			shown = shown[:min(len(shown), modelsShown)]
			st.highlight = max(0, min(st.highlight, len(shown)-1))
			choose := func(o modelOption) {
				if o.value != *value {
					*value, changed = o.value, true
				}
				st.open = false
			}
			search := ui.SearchField(c, &st.query).Label(label).Placeholder(style.T(c, "nav.search")).AutoFocus().Margin(0, 0, 4, 0)
			if search.Changed() {
				st.highlight = 0
			}
			if search.Submitted() && len(shown) > 0 {
				choose(shown[st.highlight])
			}
			moved := false
			if c.Shortcut(0, ui.KeyDown) {
				st.highlight, moved = min(st.highlight+1, len(shown)-1), true
			}
			if c.Shortcut(0, ui.KeyUp) {
				st.highlight, moved = max(st.highlight-1, 0), true
			}
			if len(shown) == 0 {
				style.Text(ui.Text(c, style.T(c, "schema.noMatches")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).Padding(8, 10)
				return
			}
			ui.Scroll(c).MaxHeight(384).Children(func() {
				ui.Column(c).AlignItems(ui.Stretch).Children(func() {
					for i, o := range shown {
						row := ui.ButtonBase(c.Key("model-"+o.value)).Column().MinHeight(32).Padding(4, 10).Radius(8).
							AlignItems(ui.Start).Justify(ui.Center).Label(o.label)
						if row.Hovered() {
							st.highlight = i
						}
						switch {
						case i == st.highlight:
							row.Background(style.Color(colors.Fill))
							if moved {
								row.ScrollIntoView()
							}
						case o.value == *value:
							row.Background(style.Color(colors.FillSelected))
						}
						row.Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).MaxWidthPercent(100).Children(func() {
								modelLabel(c, o, style.Color(colors.TextPrimary))
							})
							if o.detail != "" {
								style.Text(ui.Text(c, o.detail), style.Type(c).Caption).TextColor(style.Color(colors.TextTertiary)).SingleLine().
									Padding(0, 0, 0, 24).MaxWidthPercent(100)
							}
						})
						if row.Clicked() {
							choose(o)
						}
					}
				})
			})
			if found > len(shown) {
				style.Text(ui.Text(c, style.T(c, "schema.matchesFirst", "shown", len(shown), "count", found)), style.Type(c).Caption).
					TextColor(style.Color(colors.TextTertiary)).Padding(6, 10, 2)
			}
		})
	})
	return changed
}

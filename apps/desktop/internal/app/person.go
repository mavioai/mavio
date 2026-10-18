package app

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// personState is a person's page's state.
type personState struct {
	person task[*libraryv1.Person]
	items  task[[]*capiv1.Card]
	more   bool // the whole biography shown
}

// person builds a person's page, opened from the cast on an item's
// details: their photo, dates, biography and links, and the movies,
// series and episodes of the server crediting them.
func (a *App) person(c *ui.Context, r *ui.Route, server, id string) {
	st := a.personStates[key(server, id)]
	if st == nil {
		st = &personState{}
		a.personStates[key(server, id)] = st
	}
	items := a.itemService(server)
	t := load(a, &st.person, key("person", server, id, a.generation), func(ctx context.Context) (*libraryv1.Person, error) {
		res, err := items.GetPerson(ctx, libraryv1.GetPersonRequest_builder{Id: &id}.Build())
		return res.GetPerson(), err
	})
	cards := load(a, &st.items, key("personItems", server, id, a.generation), func(ctx context.Context) ([]*capiv1.Card, error) {
		res, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{
			PersonId: &id, Recursive: new(true),
			Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_SERIES, libraryv1.ItemKind_ITEM_KIND_EPISODE},
			Sort: []*libraryv1.SortSpec{
				libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_PREMIERE_DATE), Descending: new(true)}.Build(),
			},
		}.Build())
		var cards []*capiv1.Card
		for _, it := range res.GetItems() {
			cards = append(cards, a.cardOf(server, it))
		}
		return cards, err
	})
	if taskStatus(c, t, func() { st.person.key = "" }, nil) {
		return
	}
	p := t.value
	r.Title(p.GetName())
	colors := style.Colors(c)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Gap(32).Padding(0, 0, 40).Children(func() {
			a.personHeader(c, st, server, p)
			of := func(kind capiv1.CardKind) []*capiv1.Card {
				var out []*capiv1.Card
				for _, card := range cards.value {
					if card.GetKind() == kind {
						out = append(out, card)
					}
				}
				return out
			}
			a.personTitles(c, style.T(c, "person.movies"), of(capiv1.CardKind_CARD_KIND_MOVIE), false)
			a.personTitles(c, style.T(c, "person.series"), of(capiv1.CardKind_CARD_KIND_SERIES), false)
			a.personTitles(c, style.T(c, "person.episodes"), of(capiv1.CardKind_CARD_KIND_EPISODE), true)
			switch {
			case cards.err != nil:
				style.Text(ui.Text(c, errorMessage(c, cards.err)), style.Type.Body).TextColor(style.Color(colors.Danger)).Margin(0, style.Layout.Margin)
			case cards.loaded && len(cards.value) == 0:
				style.Text(ui.Text(c, style.T(c, "person.none")), style.Type.Body).TextColor(style.Color(colors.TextSecondary)).Margin(0, style.Layout.Margin)
			}
		})
	})
}

// yearsBetween returns the whole years from one date to another.
func yearsBetween(from, to time.Time) int {
	years := to.Year() - from.Year()
	if to.Month() < from.Month() || (to.Month() == from.Month() && to.Day() < from.Day()) {
		years--
	}
	return years
}

func (a *App) personHeader(c *ui.Context, st *personState, server string, p *libraryv1.Person) {
	colors := style.Colors(c)
	bar := c.TitleBar()
	const w = 208
	ui.Row(c).Padding(max(bar.Height, 12)+20, style.Layout.Margin, 0).Gap(24).AlignItems(ui.Start).Wrap().Children(func() {
		ui.Box(c).Size(w, w*1.5).Shrink(0).Radius(14).Clip().Background(style.Color(colors.Fill)).Center().Children(func() {
			if bitmap := a.opts.Images.Get(a.imageOf(server, p.GetImages(), libraryv1.ImageKind_IMAGE_KIND_PRIMARY).GetUrl(), w, scale); bitmap != nil {
				ui.Image(c, bitmap).Fill().Fit(ui.Cover)
			} else {
				noArtwork(c, "profile", w)
			}
		})
		ui.Column(c).Basis(320).Grow(1).MaxWidth(768).Gap(12).Children(func() {
			style.Text(ui.Text(c, p.GetName()), style.Type.Title1).TextColor(style.Color(colors.TextPrimary)).Selectable()
			var facts []string
			if p.HasBirthDate() {
				born := p.GetBirthDate().AsTime()
				end := time.Now()
				if p.HasDeathDate() {
					end = p.GetDeathDate().AsTime()
				}
				facts = append(facts, style.T(c, "person.born", "date", longDate(born), "age", yearsBetween(born, end)))
			}
			if p.GetBirthPlace() != "" {
				facts = append(facts, p.GetBirthPlace())
			}
			if p.HasDeathDate() {
				facts = append(facts, style.T(c, "person.died", "date", longDate(p.GetDeathDate().AsTime())))
			}
			for _, f := range facts {
				style.Text(ui.Text(c, f), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
			}
			if bio := p.GetOverview(); bio != "" {
				ui.Column(c).Gap(4).AlignItems(ui.Start).Children(func() {
					text := style.Text(ui.Text(c, bio), style.Type.Body).TextColor(style.Color(colors.TextPrimary)).Selectable()
					if !st.more {
						text.MaxLines(5)
					}
					label := style.T(c, "person.more")
					if st.more {
						label = style.T(c, "person.less")
					}
					if linkButton(c, label).Clicked() {
						st.more = !st.more
					}
				})
			}
			if links := p.GetExternalUrls(); len(links) > 0 && a.opts.OpenURL != nil {
				ui.Row(c).Gap(8).Wrap().Padding(4, 0, 0).Children(func() {
					for i, l := range links {
						b := ui.ButtonBase(c.Key(i)).Height(30).Padding(0, 14).Gap(6).Radius(design.RadiusFull).AlignItems(ui.Center).
							Background(style.Color(colors.Fill)).Label(l.GetName())
						if b.Hovered() {
							b.Background(style.Color(colors.FillSelected))
						}
						b.Children(func() {
							style.Text(ui.Text(c, l.GetName()), style.Type.Footnote).FontWeight(600).TextColor(style.Color(colors.TextPrimary))
							ui.Icon(c, style.Icon("external", false)).Size(14, 14).TextColor(style.Color(colors.TextSecondary))
						})
						if b.Clicked() {
							_ = a.opts.OpenURL(l.GetUrl())
						}
					}
				})
			}
		})
	})
}

// linkButton builds a button of accent text.
func linkButton(c *ui.Context, label string) ui.Element {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Label(label)
	b.Children(func() {
		t := style.Text(ui.Text(c, label), style.Type.Callout).FontWeight(600).TextColor(style.Color(colors.AccentText))
		if b.Hovered() {
			t.Underline()
		}
	})
	return b
}

// personTitles builds a section of a person's titles, a grid of posters,
// or of thumbnails when thumb.
func (a *App) personTitles(c *ui.Context, title string, cards []*capiv1.Card, thumb bool) {
	if len(cards) == 0 {
		return
	}
	colors := style.Colors(c)
	width, _ := a.gridColumns(c)
	if thumb {
		width, _ = a.thumbColumns(c)
	}
	ui.Column(c).Gap(12).Children(func() {
		ui.Row(c).Padding(0, style.Layout.Margin).Gap(8).AlignItems(ui.End).Children(func() {
			style.Text(ui.Text(c, title), style.Type.Title3).TextColor(style.Color(colors.TextPrimary))
			style.Text(ui.Text(c, fmt.Sprint(len(cards))), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
		})
		ui.Row(c).Padding(0, style.Layout.Margin).Gap(style.Layout.Gutter).Wrap().Children(func() {
			for _, card := range cards {
				if thumb {
					a.thumbCard(c, card, width)
				} else {
					a.posterCard(c, card, width)
				}
			}
		})
	})
}

// thumbColumns returns the width of the thumbnail cards of a grid filling
// the page, and how many there are.
func (a *App) thumbColumns(c *ui.Context) (float32, int) {
	avail := a.contentWidth - 2*style.Layout.Margin
	gap := style.Layout.Gutter
	cols := max(int((avail+gap)/(style.Layout.Thumbnail+gap)), 1)
	return (avail - gap*float32(cols-1)) / float32(cols), cols
}

// longDate formats a date as the web UI's long date style does.
func longDate(t time.Time) string { return t.UTC().Format("January 2, 2006") }

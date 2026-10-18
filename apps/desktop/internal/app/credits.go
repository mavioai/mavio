package app

import (
	"context"
	"slices"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// creditEntry is a person credited on an item.
type creditEntry struct {
	personID, name, role string
	tmdb                 string // of a discovered title's credit, whose page is Discover's
	kind                 libraryv1.CreditKind
	order                int32
	photo                string
}

func (e creditEntry) cast() bool {
	return e.kind == libraryv1.CreditKind_CREDIT_KIND_ACTOR || e.kind == libraryv1.CreditKind_CREDIT_KIND_GUEST_STAR
}

// crewLabels are the crew's jobs in the order the credits list them.
var crewLabels = []struct {
	kind  libraryv1.CreditKind
	label string
}{
	{libraryv1.CreditKind_CREDIT_KIND_CREATOR, "item.creators"},
	{libraryv1.CreditKind_CREDIT_KIND_DIRECTOR, "item.director"},
	{libraryv1.CreditKind_CREDIT_KIND_WRITER, "item.writers"},
	{libraryv1.CreditKind_CREDIT_KIND_AUTHOR, "credits.authors"},
	{libraryv1.CreditKind_CREDIT_KIND_PRODUCER, "item.producers"},
	{libraryv1.CreditKind_CREDIT_KIND_COMPOSER, "item.music"},
	{libraryv1.CreditKind_CREDIT_KIND_CONDUCTOR, "credits.conductors"},
	{libraryv1.CreditKind_CREDIT_KIND_LYRICIST, "credits.lyricists"},
	{libraryv1.CreditKind_CREDIT_KIND_ARTIST, "credits.artists"},
	{libraryv1.CreditKind_CREDIT_KIND_NARRATOR, "credits.narrators"},
	{libraryv1.CreditKind_CREDIT_KIND_OTHER, "credits.other"},
}

// crewRank returns where a job comes among crewLabels, past them when it
// is not one of them.
func crewRank(k libraryv1.CreditKind) int {
	for i, l := range crewLabels {
		if l.kind == k {
			return i
		}
	}
	return len(crewLabels)
}

// libraryCredits returns an item's credits of people of the server. An
// actor credited for several characters is listed once, as the first of
// them.
func (a *App) libraryCredits(server string, credits []*libraryv1.Credit) []creditEntry {
	seen := map[string]bool{}
	var out []creditEntry
	for _, c := range credits {
		p := c.GetPerson()
		if p == nil {
			continue
		}
		e := creditEntry{
			personID: p.GetId(), name: p.GetName(), role: c.GetRole(), kind: c.GetKind(), order: c.GetOrder(),
			photo: a.imageOf(server, p.GetImages(), libraryv1.ImageKind_IMAGE_KIND_PRIMARY).GetUrl(),
		}
		if e.cast() {
			if seen[e.personID] {
				continue
			}
			seen[e.personID] = true
		}
		out = append(out, e)
	}
	return out
}

// coreCredits returns the credits of the core's details, for when the
// server's own cannot be had: the cast in billing order, then the crew,
// whose jobs the core does not tell.
func coreCredits(d *capiv1.Details) []creditEntry {
	var out []creditEntry
	for i, c := range d.GetCredits() {
		kind := libraryv1.CreditKind_CREDIT_KIND_OTHER
		if c.GetCast() {
			kind = libraryv1.CreditKind_CREDIT_KIND_ACTOR
		}
		out = append(out, creditEntry{
			personID: c.GetPersonId(), name: c.GetName(), role: c.GetRole(), kind: kind, order: int32(i), photo: c.GetPhoto().GetUrl(),
		})
	}
	return out
}

// shelfCrew are the jobs the cast shelf shows after the cast.
var shelfCrew = []libraryv1.CreditKind{
	libraryv1.CreditKind_CREDIT_KIND_CREATOR, libraryv1.CreditKind_CREDIT_KIND_DIRECTOR, libraryv1.CreditKind_CREDIT_KIND_WRITER,
}

// castShelf builds the cast in billing order, then the creators,
// directors and writers, with See All opening the whole cast and crew.
func (a *App) castShelf(c *ui.Context, server, id string, credits []creditEntry) {
	var shown []creditEntry
	for _, e := range credits {
		if e.cast() || slices.Contains(shelfCrew, e.kind) {
			shown = append(shown, e)
		}
	}
	if len(shown) == 0 {
		return
	}
	slices.SortStableFunc(shown, func(x, y creditEntry) int {
		if x.cast() != y.cast() {
			if x.cast() {
				return -1
			}
			return 1
		}
		return int(x.order - y.order)
	})
	label := func(e creditEntry) string {
		if e.cast() {
			return e.role
		}
		return style.T(c, crewLabels[crewRank(e.kind)].label)
	}
	all := func() { a.router.Push("/credits/" + server + "/" + id) }
	a.labeledShelf(c, key("cast", server, id), style.T(c, "item.cast"), all, style.T(c, "credits.all"), func() {
		for i, e := range shown {
			a.personTile(c.Key(i), server, e, label(e), 120)
		}
	})
}

// personTile builds a credited person width wide: their photo, else the
// placeholder of missing artwork, their name, and under it a label, such as the character they
// play or their job; it opens their page.
func (a *App) personTile(c *ui.Context, server string, e creditEntry, label string, width float32) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Cursor(ui.CursorPointer).Column().Width(width).Gap(4).AlignItems(ui.Center).Label(e.name)
	hovered := b.Hovered()
	if b.Pressed() {
		b.Opacity(0.8)
	}
	photo := width - 24
	b.Children(func() {
		circle := ui.Box(c).Size(photo, photo).Radius(photo / 2).Background(style.Color(colors.Fill))
		if hovered {
			circle.Shadow(0, 0, 0, 3, style.Color(colors.Accent))
		}
		circle.Children(func() {
			inner := ui.Box(c).Fill().Radius(photo / 2).Clip()
			if hovered {
				inner.Border(1, style.Color(colors.Background))
			}
			inner.Children(func() {
				if bitmap := a.opts.Images.Get(e.photo, photo, scale); bitmap != nil {
					ui.Image(c, bitmap).Fill().Fit(ui.Cover)
				} else {
					noArtwork(c, "profile", photo)
				}
			})
		})
		name := style.Text(ui.Text(c, e.name), style.Type.Callout).TextColor(style.Color(colors.TextPrimary)).TextAlign(ui.Center).MaxLines(2).Margin(4, 0, 0)
		if hovered {
			name.Underline()
		}
		style.Text(ui.Text(c, label), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).TextAlign(ui.Center).MaxLines(2)
	})
	if b.Clicked() {
		a.openPerson(server, e)
	}
}

// openPerson opens a credited person's page: the server's when it has
// them, else Discover's.
func (a *App) openPerson(server string, e creditEntry) {
	switch {
	case e.personID != "":
		a.router.Push("/person/" + server + "/" + e.personID)
	case e.tmdb != "":
		a.router.Push(discoveredPersonPath(server, e.tmdb))
	}
}

// creditsState is the state of an item's credits page.
type creditsState struct {
	item task[*libraryv1.GetItemResponse]
}

// creditsPage builds the whole cast and crew of an item: the cast in
// billing order, then the crew by job; the crew alone when crew.
func (a *App) creditsPage(c *ui.Context, r *ui.Route, server, id string, crew bool) {
	st := a.creditStates[key(server, id)]
	if st == nil {
		st = &creditsState{}
		a.creditStates[key(server, id)] = st
	}
	t := load(a, &st.item, key("item", server, id, a.generation), func(ctx context.Context) (*libraryv1.GetItemResponse, error) {
		return a.itemService(server).GetItem(ctx, libraryv1.GetItemRequest_builder{Id: &id}.Build())
	})
	if taskStatus(c, t, func() { st.item.key = "" }, nil) {
		return
	}
	title := style.T(c, "item.cast")
	if crew {
		title = style.T(c, "credits.fullCrew")
	}
	r.Title(title)
	credits := a.libraryCredits(server, t.value.GetCredits())
	if crew {
		credits = slices.DeleteFunc(credits, creditEntry.cast)
	}
	colors := style.Colors(c)
	of := func(kind libraryv1.CreditKind) []creditEntry {
		var out []creditEntry
		for _, e := range credits {
			if e.kind == kind {
				out = append(out, e)
			}
		}
		slices.SortStableFunc(out, func(x, y creditEntry) int { return int(x.order - y.order) })
		return out
	}
	ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Fill().Children(func() {
		ui.Column(c).Gap(24).Padding(0, 0, 40).Children(func() {
			a.pageHeader(c, title, func() {
				style.Text(ui.Text(c, t.value.GetItem().GetName()), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
			})
			if len(credits) == 0 {
				style.Text(ui.Text(c, style.T(c, "credits.none")), style.Type.Body).TextColor(style.Color(colors.TextSecondary)).Margin(0, style.Layout.Margin)
			}
			a.people(c, server, style.T(c, "credits.cast"), of(libraryv1.CreditKind_CREDIT_KIND_ACTOR))
			a.people(c, server, style.T(c, "credits.guestStars"), of(libraryv1.CreditKind_CREDIT_KIND_GUEST_STAR))
			for _, l := range crewLabels {
				a.people(c, server, style.T(c, l.label), of(l.kind))
			}
		})
	})
}

// people builds a heading over its people, in a grid.
func (a *App) people(c *ui.Context, server, title string, credits []creditEntry) {
	if len(credits) == 0 {
		return
	}
	colors := style.Colors(c)
	ui.Column(c.Key(title)).Gap(12).Children(func() {
		style.Text(ui.Text(c, title), style.Type.Title3).TextColor(style.Color(colors.TextPrimary)).Margin(0, style.Layout.Margin)
		ui.Row(c).Padding(0, style.Layout.Margin).GapX(style.Layout.Gutter).GapY(24).Wrap().Children(func() {
			for i, e := range credits {
				a.personTile(c.Key(i), server, e, e.role, 120)
			}
		})
	})
}

// crewList builds an item's leading crew, each their job over their
// name, with View Full Crew opening the whole cast and crew.
func (a *App) crewList(c *ui.Context, server, id string, credits []creditEntry) {
	var crew []creditEntry
	for _, e := range credits {
		if !e.cast() && crewRank(e.kind) < len(crewLabels) {
			crew = append(crew, e)
		}
	}
	if len(crew) == 0 {
		return
	}
	slices.SortStableFunc(crew, func(x, y creditEntry) int { return crewRank(x.kind) - crewRank(y.kind) })
	crew = crew[:min(len(crew), 6)]
	colors := style.Colors(c)
	ui.Column(c).Gap(12).AlignItems(ui.Start).Children(func() {
		ui.Row(c).GapX(24).GapY(16).Wrap().Children(func() {
			for i, e := range crew {
				job := style.T(c, crewLabels[crewRank(e.kind)].label)
				if e.kind == libraryv1.CreditKind_CREDIT_KIND_OTHER && e.role != "" {
					job = e.role
				}
				ui.Column(c.Key(i)).Width(160).AlignItems(ui.Start).Children(func() {
					style.Text(ui.Text(c, job), style.Type.Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
					b := ui.ButtonBase(c).Cursor(ui.CursorPointer).Label(e.name)
					b.Children(func() {
						name := style.Text(ui.Text(c, e.name), style.Type.Callout).TextColor(style.Color(colors.TextSecondary)).SingleLine()
						if b.Hovered() {
							name.TextColor(style.Color(colors.TextPrimary)).Underline()
						}
					})
					if b.Clicked() && e.personID != "" {
						a.router.Push("/person/" + server + "/" + e.personID)
					}
				})
			}
		})
		if linkButton(c, style.T(c, "discover.fullCrew")).Clicked() {
			a.router.Push("/crew/" + server + "/" + id)
		}
	})
}

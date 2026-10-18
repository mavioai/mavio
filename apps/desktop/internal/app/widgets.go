package app

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// scale is the display scale images are loaded for.
const scale = 2

// key joins parts into a key of a task or an element.
func key(parts ...any) string {
	s := make([]string, len(parts))
	for i, p := range parts {
		s[i] = fmt.Sprint(p)
	}
	return strings.Join(s, "/")
}

// errorMessage returns what to tell the user of err.
func errorMessage(c *ui.Context, err error) string {
	if e := coreError(err); e != nil {
		switch e.Code {
		case capiv1.Error_CODE_UNAUTHENTICATED:
			return style.T(c, "auth.wrongPassword")
		case capiv1.Error_CODE_UNREACHABLE:
			return style.T(c, "auth.unreachable")
		}
		if e.Reason != "" {
			return e.Reason
		}
	}
	return style.T(c, "state.error")
}

// secondaryButton builds a button on the fill color.
func secondaryButton(c *ui.Context, icon, label string) ui.Element {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Height(36).Padding(0, 16).Gap(8).Radius(18).AlignItems(ui.Center).Justify(ui.Center).
		Background(style.Color(colors.Fill)).Label(label)
	if b.Hovered() {
		b.Background(style.Color(colors.FillSelected))
	}
	if b.Pressed() {
		b.Opacity(0.8)
	}
	b.Children(func() {
		if icon != "" {
			ui.Icon(c, style.Icon(icon, false)).Size(18, 18).TextColor(style.Color(colors.TextPrimary))
		}
		style.Text(ui.Text(c, label), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
	})
	return b
}

// statusView builds the empty, error or offline state of a screen: an
// icon, a title, a message and a retry action, which retry runs when not
// nil.
func statusView(c *ui.Context, icon, title, message string, retry func()) {
	colors := style.Colors(c)
	ui.Column(c).Grow(1).Padding(48).Gap(8).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		if icon != "" {
			ui.Icon(c, style.Icon(icon, false)).Size(48, 48).TextColor(style.Color(colors.TextTertiary))
		}
		style.Text(ui.Text(c, title), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).TextAlign(ui.Center)
		if message != "" {
			style.Text(ui.Text(c, message), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary)).TextAlign(ui.Center).MaxWidth(420)
		}
		if retry != nil && secondaryButton(c, "retry", style.T(c, "action.retry")).Margin(8, 0, 0).Clicked() {
			retry()
		}
	})
}

// loading builds a page's content on its way, loading since since: the
// activity indicator, centered in the room the page is given.
func loading(c *ui.Context, since time.Time) {
	ui.Column(c).Grow(1).Padding(48).AlignItems(ui.Center).Justify(ui.Center).Children(func() {
		activityIndicator(c, 28, since)
	})
}

// activityIndicator builds the activity indicator size wide, for what
// started loading at since: eight spokes in the tertiary text's color,
// fading around the circle and turning a spoke a step, once in 0.9 s. It
// fades in from 0.4 s after since, so that what comes quickly shows
// nothing first.
func activityIndicator(c *ui.Context, size float32, since time.Time) {
	color := style.Color(style.Colors(c).TextTertiary)
	ui.Box(c).Size(size, size).Shrink(0).Role(ui.RoleProgress).Label(style.T(c, "state.loading")).Draw(func(p *ui.Painter, r ui.Rect) {
		const spokes, turn, delay, fade = 8, 900 * time.Millisecond, 400 * time.Millisecond, 300 * time.Millisecond
		now := p.Now()
		shown := now.Sub(since) - delay
		if shown < 0 {
			p.After(-shown)
			return
		}
		alpha := float32(1)
		if shown < fade {
			alpha = float32(shown) / float32(fade)
			p.AnimationFrame()
		}
		step := int(now.UnixMilli()%turn.Milliseconds()) * spokes / int(turn.Milliseconds())
		p.After(turn / spokes)
		// In a box of 24: spokes from 4 to 10 from the center, 2 wide.
		u := r.W / 24
		cx, cy := r.X+r.W/2, r.Y+r.H/2
		for i := range spokes {
			// The spoke i ahead of the one leading is i steps behind it.
			a := float64(i+step) * 2 * math.Pi / spokes
			sin, cos := float32(math.Sin(a)), float32(math.Cos(a))
			var path ui.Path
			path.MoveTo(cx+sin*5*u, cy-cos*5*u).LineTo(cx+sin*9*u, cy-cos*9*u)
			p.StrokePath(&path, 2*u, color.Alpha(alpha*(0.2+0.8*float32(i)/(spokes-1))))
		}
	})
}

// taskStatus builds the state of a task that has no value to show: its
// placeholder while it loads, or the activity indicator without one, else
// its error; it reports whether it built
// one.
func taskStatus[T any](c *ui.Context, t *task[T], retry func(), placeholder func()) bool {
	switch {
	case !t.loaded:
		if placeholder != nil {
			placeholder()
		} else {
			loading(c, t.started)
		}
		return true
	case t.err != nil:
		if e := coreError(t.err); e != nil && e.Code == capiv1.Error_CODE_UNREACHABLE {
			statusView(c, "offline", style.T(c, "state.offline"), "", retry)
		} else {
			statusView(c, "error", style.T(c, "state.error"), errorMessage(c, t.err), retry)
		}
		return true
	}
	return false
}

// artwork builds a card's artwork width × height: the image, else the
// placeholder of missing artwork with icon, or with the icon of card's
// kind when icon is empty, and the card's overlays.
func (a *App) artwork(c *ui.Context, art *capiv1.Artwork, icon string, card *capiv1.Card, width, height float32, hovered bool) {
	colors := style.Colors(c)
	box := ui.Box(c).Size(width, height).Shrink(0).Radius(design.RadiusMd).Background(style.Color(colors.SurfaceRaised))
	tv := style.TV(c)
	switch {
	case hovered && tv:
		// Focused on the television: a ring of the accent 1.5 points apart,
		// over a shadow (docs/tv.md §4).
		box.Shadow(0, 12, 32, 0, style.Color(colors.TvFocusShadow)).Shadow(0, 0, 0, 4.5, style.Color(colors.Accent)).
			Shadow(0, 0, 0, 1.5, style.Color(colors.Background))
	case hovered:
		box.Shadow(0, 0, 0, 3, style.Color(colors.Accent))
	}
	box.Children(func() {
		inner := ui.Box(c).Fill().Radius(design.RadiusMd).Clip()
		if hovered && !tv {
			inner.Border(1, style.Color(colors.Background))
		}
		inner.Children(func() {
			if bitmap := a.opts.Images.Get(art.GetUrl(), width, scale); bitmap != nil {
				ui.Image(c, bitmap).Fill().Fit(ui.Cover)
			} else if art.GetUrl() == "" || a.imageFailed(art.GetUrl(), width) {
				if icon == "" {
					icon = cardIcon(card.GetKind())
				}
				noArtwork(c, icon, width)
			}
			if card == nil {
				return
			}
			white := ui.RGB(255, 255, 255)
			if card.HasProgress() && card.GetProgress() > 0 && !card.GetPlayed() {
				ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(3).Background(style.Color(colors.Scrim)).Children(func() {
					ui.Box(c).Height(3).WidthPercent(min(card.GetProgress(), 1) * 100).Background(style.Color(colors.Progress))
				})
				if left := time.Duration(float64(card.GetRuntimeMs())*(1-float64(card.GetProgress()))) * time.Millisecond; left > time.Minute && width >= style.Layout(c).Thumbnail/2 && height < width {
					style.Text(ui.Text(c, style.T(c, "item.minutesLeft", "count", int(left.Minutes()))), style.Type(c).Caption).
						TextColor(white).Absolute().Left(8).Bottom(10).Padding(2, 6).Radius(design.RadiusSm).Background(style.Color(colors.Scrim))
				}
			}
			if card.GetPlayed() {
				ui.Box(c).Absolute().Top(6).Right(6).Size(22, 22).Radius(11).Background(style.Color(colors.Scrim)).Center().Children(func() {
					ui.Icon(c, style.Icon("played", true)).Size(16, 16).TextColor(white)
				})
			}
		})
	})
}

// noArtwork builds the placeholder of missing artwork, width wide: a quiet
// wash of gray from its top left, gone by 70% of the way, and icon.
func noArtwork(c *ui.Context, icon string, width float32) {
	colors := style.Colors(c)
	fill := style.Color(colors.Fill)
	size := min(width*0.26, 56)
	ui.Box(c).Fill().LinearGradient(ui.LinearGradient{From: fill, To: fill.Alpha(0), Angle: 160, End: 0.7}).Center().Children(func() {
		ui.Icon(c, style.Icon(icon, false)).Size(size, size).TextColor(style.Color(colors.TextTertiary)).Opacity(0.6)
	})
}

// cardIcon returns the icon of a card of kind, on its artwork's
// placeholder.
func cardIcon(kind capiv1.CardKind) string {
	switch kind {
	case capiv1.CardKind_CARD_KIND_SERIES, capiv1.CardKind_CARD_KIND_SEASON, capiv1.CardKind_CARD_KIND_EPISODE:
		return "shows"
	case capiv1.CardKind_CARD_KIND_ARTIST, capiv1.CardKind_CARD_KIND_ALBUM, capiv1.CardKind_CARD_KIND_TRACK, capiv1.CardKind_CARD_KIND_MUSIC_VIDEO:
		return "music"
	case capiv1.CardKind_CARD_KIND_AUDIOBOOK, capiv1.CardKind_CARD_KIND_BOOK:
		return "books"
	case capiv1.CardKind_CARD_KIND_PHOTO_ALBUM, capiv1.CardKind_CARD_KIND_PHOTO:
		return "photos"
	case capiv1.CardKind_CARD_KIND_VIDEO:
		return "homeVideos"
	case capiv1.CardKind_CARD_KIND_FOLDER:
		return "folder"
	case capiv1.CardKind_CARD_KIND_COLLECTION:
		return "collections"
	}
	return "movies"
}

// imageFailed reports whether the image of url shown width wide is not
// coming: it failed to load.
func (a *App) imageFailed(url string, width float32) bool {
	return a.opts.Images.Failed(url, width, scale)
}

// posterCard builds a 2:3 card of width.
func (a *App) posterCard(c *ui.Context, card *capiv1.Card, width float32) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(key(card.GetServerId(), card.GetItemId()))).Cursor(ui.CursorPointer).Column().Width(width).Gap(6).AlignItems(ui.Start).Label(card.GetTitle())
	hovered := a.cardFocus(c, b) || b.Hovered()
	if b.Pressed() {
		b.Opacity(0.8)
	}
	art := card.GetPoster()
	if art.GetUrl() == "" && card.GetKind() == capiv1.CardKind_CARD_KIND_EPISODE {
		art = card.GetThumb()
	}
	b.Children(func() {
		a.artwork(c, art, "", card, width, width*1.5, hovered)
		style.Text(ui.Text(c, card.GetTitle()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(width)
		if sub := cardSubtitle(card); sub != "" {
			style.Text(ui.Text(c, sub), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine().Width(width)
		}
	})
	if b.Clicked() {
		a.open(card)
	}
}

// thumbCard builds a 16:9 card of width.
func (a *App) thumbCard(c *ui.Context, card *capiv1.Card, width float32) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(key(card.GetServerId(), card.GetItemId()))).Cursor(ui.CursorPointer).Column().Width(width).Gap(6).AlignItems(ui.Start).Label(card.GetTitle())
	hovered := a.cardFocus(c, b) || b.Hovered()
	if b.Pressed() {
		b.Opacity(0.8)
	}
	b.Children(func() {
		a.artwork(c, card.GetThumb(), "", card, width, width*9/16, hovered)
		style.Text(ui.Text(c, card.GetTitle()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(width)
		if sub := card.GetSubtitle(); sub != "" {
			style.Text(ui.Text(c, sub), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine().Width(width)
		}
	})
	if b.Clicked() {
		a.open(card)
	}
}

// cardFocus makes a card one focus moves to in the TV mode, which grows
// by the layout's focus scale with focus while the other cards of its row
// fade to 75 % (docs/tv.md §4), and reports whether it has focus.
func (a *App) cardFocus(c *ui.Context, b ui.Element) bool {
	if !style.TV(c) {
		return false
	}
	focused := a.tvFocus(c, b, false)
	scale := float32(1)
	if focused {
		scale = style.Layout(c).FocusScale
	}
	s := b.Animate("focus", scale, design.DurationFast)
	b.TransformOrigin(0.5, 0.5).Scale(s, s)
	if a.focus.rowFocused && !focused {
		b.Opacity(0.75)
	}
	return focused
}

// cardSubtitle returns what a poster card shows under its title.
func cardSubtitle(card *capiv1.Card) string {
	if card.GetSubtitle() != "" {
		return card.GetSubtitle()
	}
	if card.GetYear() > 0 {
		return fmt.Sprint(card.GetYear())
	}
	return ""
}

// open opens a card's details.
func (a *App) open(card *capiv1.Card) {
	a.router.Push("/item/" + card.GetServerId() + "/" + card.GetItemId())
}

// wide reports whether a card shows as a thumbnail card.
func wide(card *capiv1.Card) bool {
	switch card.GetKind() {
	case capiv1.CardKind_CARD_KIND_EPISODE, capiv1.CardKind_CARD_KIND_VIDEO, capiv1.CardKind_CARD_KIND_MUSIC_VIDEO:
		return true
	}
	return false
}

// shelf builds a titled horizontal row of cards; more, when not nil, is
// its "more" action.
func (a *App) shelf(c *ui.Context, id, title string, more func(), cards func()) {
	a.labeledShelf(c, id, title, more, "", cards)
}

// labeledShelf builds a shelf whose "more" action also shows as a link
// of label at its end, when label is not empty.
func (a *App) labeledShelf(c *ui.Context, id, title string, more func(), label string, cards func()) {
	colors := style.Colors(c)
	scroll := a.shelfScroll(id)
	ui.Column(c.Key(id)).Gap(6).Children(func() {
		head := ui.Row(c).Padding(0, style.Layout(c).Margin).Gap(8).AlignItems(ui.Center)
		hovered := head.Hovered()
		head.Children(func() {
			t := style.Text(ui.Text(c, title), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).SingleLine()
			if more != nil {
				t.Cursor(ui.CursorPointer)
				if t.Clicked() {
					more()
				}
			}
			ui.Spacer(c)
			// On the television, See All is the row's last card instead.
			if label != "" && !style.TV(c) && linkButton(c, label).Clicked() {
				more()
			}
			// A remote has no arrows to scroll a row with.
			if scroll.MaxX <= 0 || style.TV(c) {
				return
			}
			w, _ := c.Size()
			step := max(w*0.8, 200)
			back := iconButton(c, "back", style.T(c, "action.scrollBack")).Disabled(scroll.X <= 0).Opacity(arrowOpacity(hovered, scroll.X <= 0))
			if back.Clicked() {
				scroll.ScrollTo(max(scroll.X-step, 0), 0, ui.Smooth)
			}
			fwd := iconButton(c, "forward", style.T(c, "action.scrollForward")).Disabled(scroll.X >= scroll.MaxX).Opacity(arrowOpacity(hovered, scroll.X >= scroll.MaxX))
			if fwd.Clicked() {
				scroll.ScrollTo(min(scroll.X+step, scroll.MaxX), 0, ui.Smooth)
			}
		})
		// The padding leaves room for a hovered card's ring, which the
		// scroll would clip; the column's gap is the less for it.
		row := ui.ScrollHorizontal(c).TrackScroll(scroll)
		gap, pad := style.Layout(c).Gutter*1.5, float32(4)
		if style.TV(c) {
			// Room above and below for a focused card to grow.
			gap, pad = style.Layout(c).Gutter, 20
		}
		row.Children(func() {
			f := &a.focus
			was := f.rowFocused
			f.rowFocused = style.TV(c) && row.FocusWithin()
			a.inScrollRow(row.Bounds(), scroll, func() {
				ui.Row(c).Padding(pad, style.Layout(c).Margin).Gap(gap).AlignItems(ui.Start).Children(func() {
					cards()
					if label != "" && more != nil && style.TV(c) {
						a.seeAllCard(c, label, more)
					}
				})
			})
			f.rowFocused = was
		})
	})
}

// seeAllCard builds the card ending a row on the television, which opens
// all of what the row shows (docs/tv.md §6.2, §6.3).
func (a *App) seeAllCard(c *ui.Context, label string, more func()) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key("see-all")).Cursor(ui.CursorPointer).Column().Width(120).Gap(8).AlignItems(ui.Center).Label(label)
	if a.tvFocusLit(c, b, false) {
		b.Padding(8).Margin(-8).Radius(design.RadiusLg)
		colors = &design.LightColors
	}
	b.Children(func() {
		ui.Box(c).Size(96, 96).Radius(48).Background(style.Color(colors.Fill)).Center().Children(func() {
			ui.Icon(c, style.Icon("forward", false)).Size(32, 32).TextColor(style.Color(colors.TextPrimary))
		})
		style.Text(ui.Text(c, label), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.TextPrimary)).TextAlign(ui.Center).MaxLines(2)
	})
	if b.Clicked() {
		more()
	}
}

func arrowOpacity(hovered, atEnd bool) float32 {
	switch {
	case atEnd:
		return 0.3
	case hovered:
		return 1
	}
	return 0.6
}

// shelfScroll returns the scroll state of the shelf id.
func (a *App) shelfScroll(id string) *ui.ScrollState {
	s, ok := a.scrolls[id]
	if !ok {
		s = &ui.ScrollState{}
		a.scrolls[id] = s
	}
	return s
}

// placeholderShelf builds a shelf's loading state.
func placeholderShelf(c *ui.Context, n int) {
	colors := style.Colors(c)
	ui.Column(c).Gap(10).Children(func() {
		ui.Box(c).Margin(0, style.Layout(c).Margin).Size(180, 22).Radius(design.RadiusSm).Background(style.Color(colors.SurfaceRaised))
		ui.Row(c).Padding(0, style.Layout(c).Margin).Gap(style.Layout(c).Gutter * 1.5).ClipX().Children(func() {
			for range n {
				ui.Box(c).Size(style.Layout(c).Poster, style.Layout(c).Poster*1.5).Shrink(0).Radius(design.RadiusMd).Background(style.Color(colors.SurfaceRaised))
			}
		})
	})
}

// header builds the top of a page: room for the window controls, which
// drags the window, and the title.
func header(c *ui.Context, title string) {
	colors := style.Colors(c)
	bar := c.TitleBar()
	ui.Row(c).MinHeight(max(bar.Height, 28)+24).Padding(max(bar.Height, 12), style.Layout(c).Margin, 12).AlignItems(ui.End).DragWindow().Children(func() {
		style.Text(ui.Text(c, title), style.Type(c).Title1).TextColor(style.Color(colors.TextPrimary)).SingleLine()
	})
}

package app

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"github.com/mavioai/mavio/libs/client-rust/capi"
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
	var e *capi.Error
	if errors.As(err, &e) {
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
		style.Text(ui.Text(c, label), style.Type.Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine()
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
		style.Text(ui.Text(c, title), style.Type.Title3).TextColor(style.Color(colors.TextPrimary)).TextAlign(ui.Center)
		if message != "" {
			style.Text(ui.Text(c, message), style.Type.Body).TextColor(style.Color(colors.TextSecondary)).TextAlign(ui.Center).MaxWidth(420)
		}
		if retry != nil && secondaryButton(c, "retry", style.T(c, "action.retry")).Margin(8, 0, 0).Clicked() {
			retry()
		}
	})
}

// taskStatus builds the state of a task that has no value to show: its
// placeholder while it loads, else its error; it reports whether it built
// one.
func taskStatus[T any](c *ui.Context, t *task[T], retry func(), placeholder func()) bool {
	switch {
	case !t.loaded:
		if placeholder != nil {
			placeholder()
		}
		return true
	case t.err != nil:
		var e *capi.Error
		if errors.As(t.err, &e) && e.Code == capiv1.Error_CODE_UNREACHABLE {
			statusView(c, "offline", style.T(c, "state.offline"), "", retry)
		} else {
			statusView(c, "error", style.T(c, "state.error"), errorMessage(c, t.err), retry)
		}
		return true
	}
	return false
}

// artwork builds a card's artwork width × height: the image, else the
// title on the raised surface, with the card's overlays.
func (a *App) artwork(c *ui.Context, art *capiv1.Artwork, title string, card *capiv1.Card, width, height float32, hovered bool) {
	colors := style.Colors(c)
	box := ui.Box(c).Size(width, height).Shrink(0).Radius(design.RadiusMd).Background(style.Color(colors.SurfaceRaised))
	if hovered {
		box.Shadow(0, 0, 0, 3, style.Color(colors.Accent))
	}
	box.Children(func() {
		inner := ui.Box(c).Fill().Radius(design.RadiusMd).Clip()
		if hovered {
			inner.Border(1, style.Color(colors.Background))
		}
		inner.Children(func() {
			if bitmap := a.opts.Images.Get(art.GetUrl(), width, scale); bitmap != nil {
				ui.Image(c, bitmap).Fill().Fit(ui.Cover)
			} else if title != "" && (art.GetUrl() == "" || a.imageFailed(art.GetUrl(), width)) {
				ui.Box(c).Fill().Padding(12).Center().Children(func() {
					style.Text(ui.Text(c, title), style.Type.Title3).TextColor(style.Color(colors.TextSecondary)).TextAlign(ui.Center).MaxLines(4)
				})
			}
			if card == nil {
				return
			}
			white := ui.RGB(255, 255, 255)
			if card.HasProgress() && card.GetProgress() > 0 && !card.GetPlayed() {
				ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(3).Background(style.Color(colors.Scrim)).Children(func() {
					ui.Box(c).Height(3).WidthPercent(min(card.GetProgress(), 1) * 100).Background(style.Color(colors.Progress))
				})
				if left := time.Duration(float64(card.GetRuntimeMs())*(1-float64(card.GetProgress()))) * time.Millisecond; left > time.Minute && width >= style.Layout.Thumbnail/2 && height < width {
					style.Text(ui.Text(c, style.T(c, "item.minutesLeft", "count", int(left.Minutes()))), style.Type.Caption).
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

// imageFailed reports whether the image of url shown width wide is not
// coming: it failed to load.
func (a *App) imageFailed(url string, width float32) bool {
	return a.opts.Images.Failed(url, width, scale)
}

// posterCard builds a 2:3 card of width.
func (a *App) posterCard(c *ui.Context, card *capiv1.Card, width float32) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(key(card.GetServerId(), card.GetItemId()))).Column().Width(width).Gap(6).AlignItems(ui.Start).Label(card.GetTitle())
	hovered := b.Hovered()
	if b.Pressed() {
		b.Opacity(0.8)
	}
	art := card.GetPoster()
	if art.GetUrl() == "" && card.GetKind() == capiv1.CardKind_CARD_KIND_EPISODE {
		art = card.GetThumb()
	}
	b.Children(func() {
		a.artwork(c, art, card.GetTitle(), card, width, width*1.5, hovered)
		style.Text(ui.Text(c, card.GetTitle()), style.Type.Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(width)
		if sub := cardSubtitle(card); sub != "" {
			style.Text(ui.Text(c, sub), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine().Width(width)
		}
	})
	if b.Clicked() {
		a.open(card)
	}
}

// thumbCard builds a 16:9 card of width.
func (a *App) thumbCard(c *ui.Context, card *capiv1.Card, width float32) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(key(card.GetServerId(), card.GetItemId()))).Column().Width(width).Gap(6).AlignItems(ui.Start).Label(card.GetTitle())
	hovered := b.Hovered()
	if b.Pressed() {
		b.Opacity(0.8)
	}
	b.Children(func() {
		a.artwork(c, card.GetThumb(), card.GetTitle(), card, width, width*9/16, hovered)
		style.Text(ui.Text(c, card.GetTitle()), style.Type.Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(width)
		if sub := card.GetSubtitle(); sub != "" {
			style.Text(ui.Text(c, sub), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine().Width(width)
		}
	})
	if b.Clicked() {
		a.open(card)
	}
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
	colors := style.Colors(c)
	scroll := a.shelfScroll(id)
	ui.Column(c.Key(id)).Gap(10).Children(func() {
		head := ui.Row(c).Padding(0, style.Layout.Margin).Gap(8).AlignItems(ui.Center)
		hovered := head.Hovered()
		head.Children(func() {
			t := style.Text(ui.Text(c, title), style.Type.Title3).TextColor(style.Color(colors.TextPrimary)).SingleLine()
			if more != nil {
				t.Cursor(ui.CursorPointer)
				if t.Clicked() {
					more()
				}
			}
			ui.Spacer(c)
			if scroll.MaxX <= 0 {
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
		ui.ScrollHorizontal(c).TrackScroll(scroll).Children(func() {
			ui.Row(c).Padding(0, style.Layout.Margin, 4).Gap(style.Layout.Gutter * 1.5).AlignItems(ui.Start).Children(cards)
		})
	})
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
		ui.Box(c).Margin(0, style.Layout.Margin).Size(180, 22).Radius(design.RadiusSm).Background(style.Color(colors.SurfaceRaised))
		ui.Row(c).Padding(0, style.Layout.Margin).Gap(style.Layout.Gutter * 1.5).ClipX().Children(func() {
			for range n {
				ui.Box(c).Size(style.Layout.Poster, style.Layout.Poster*1.5).Shrink(0).Radius(design.RadiusMd).Background(style.Color(colors.SurfaceRaised))
			}
		})
	})
}

// header builds the top of a page: room for the window controls, which
// drags the window, and the title.
func header(c *ui.Context, title string) {
	colors := style.Colors(c)
	bar := c.TitleBar()
	ui.Row(c).MinHeight(max(bar.Height, 28)+24).Padding(max(bar.Height, 12), style.Layout.Margin, 12).AlignItems(ui.End).DragWindow().Children(func() {
		style.Text(ui.Text(c, title), style.Type.Title1).TextColor(style.Color(colors.TextPrimary)).SingleLine()
	})
}

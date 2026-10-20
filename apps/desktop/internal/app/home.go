package app

import (
	"context"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// heroInterval is how long the hero shows each title.
const heroInterval = 9 * time.Second

// heroTitles is how many titles the hero shows at most.
const heroTitles = 5

// homeState is Home's state: its rows, all of them each time the core
// sends them as the servers answer.
type homeState struct {
	key     string
	rows    []*capiv1.Row
	err     error
	loading bool
	started time.Time // when it started loading
	// heroAt is the title the hero shows and heroSince when it began, or
	// how long it had shown while the pointer was over the hero, which
	// pauses it.
	heroAt     int
	heroSince  time.Time
	heroShown  time.Duration
	heroPaused bool
}

// loadHome loads Home again when the servers' content changed.
func (a *App) loadHome() {
	h := &a.homeState
	k := key("home", a.generation)
	if h.key == k {
		return
	}
	h.key, h.loading, h.started = k, true, time.Now()
	go func() {
		_, err := a.opts.Core.Stream(a.ctx, capiv1.Request_builder{LoadHome: &capiv1.LoadHome{}}.Build(), func(m *capiv1.Message) {
			rows := m.GetRows().GetRows()
			a.opts.Update(func() {
				if h.key == k {
					h.rows = rows
				}
			})
		})
		a.opts.Update(func() {
			if h.key == k {
				h.err, h.loading = err, false
			}
		})
	}()
}

// home builds Home: the hero, then the rows.
func (a *App) home(c *ui.Context) {
	a.loadHome()
	h := &a.homeState
	// Nothing yet from any server: the activity indicator alone.
	if len(h.rows) > 0 && !slices.ContainsFunc(h.rows, func(r *capiv1.Row) bool { return !r.HasLoading() }) {
		loading(c, h.started)
		return
	}
	if len(h.rows) == 0 {
		switch {
		case h.loading:
			loading(c, h.started)
		case h.err != nil:
			statusView(c, "error", style.T(c, "state.error"), errorMessage(c, h.err), func() { h.key = "" })
		default:
			statusView(c, "", style.T(c, "state.empty"), "", nil)
		}
		return
	}
	ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Fill().Children(func() {
		ui.Column(c).Gap(32).Padding(0, 0, 40).Children(func() {
			if hero := heroEntries(c, h.rows); len(hero) > 0 {
				a.hero(c, hero)
			} else {
				header(c, style.T(c, "nav.home"))
			}
			for _, r := range h.rows {
				a.row(c, r)
			}
		})
	})
}

// heroEntry is a title the hero shows, and the line above it: the title
// of the row it comes from.
type heroEntry struct {
	card    *capiv1.Card
	eyebrow string
}

// heroKinds are the kinds the hero shows.
var heroKinds = []capiv1.CardKind{
	capiv1.CardKind_CARD_KIND_MOVIE, capiv1.CardKind_CARD_KIND_SERIES,
	capiv1.CardKind_CARD_KIND_VIDEO, capiv1.CardKind_CARD_KIND_MUSIC_VIDEO,
}

// heroEntries returns up to five titles with backdrops from Home's rows,
// Continue Watching aside, as the web UI's.
func heroEntries(c *ui.Context, rows []*capiv1.Row) []heroEntry {
	var out []heroEntry
	seen := map[string]bool{}
	for _, r := range rows {
		if r.GetKind() == capiv1.RowKind_ROW_KIND_CONTINUE_WATCHING || !r.HasItems() {
			continue
		}
		for _, card := range r.GetItems().GetCards() {
			k := key(card.GetServerId(), card.GetItemId())
			if seen[k] || !slices.Contains(heroKinds, card.GetKind()) || card.GetBackdrop().GetUrl() == "" {
				continue
			}
			seen[k] = true
			out = append(out, heroEntry{card, rowTitle(c, r)})
			if len(out) == heroTitles {
				return out
			}
		}
	}
	return out
}

// hero builds the titles at the top of Home, one after another, as the
// web UI's: each with the row it comes from, its logo or title, its
// facts, overview and the buttons to play it and open its details. The
// pointer over it pauses it.
func (a *App) hero(c *ui.Context, entries []heroEntry) {
	h := &a.homeState
	now := c.Now()
	if h.heroSince.IsZero() {
		h.heroSince = now
	}
	shown := h.heroShown
	if !h.heroPaused {
		shown += now.Sub(h.heroSince)
	}
	if len(entries) > 1 && !h.heroPaused {
		if shown >= heroInterval {
			h.heroAt, h.heroSince, h.heroShown, shown = h.heroAt+1, now, 0, 0
		}
		c.After(heroInterval - shown)
	}
	at := h.heroAt % len(entries)
	card, eyebrow := entries[at].card, entries[at].eyebrow
	_, wh := c.Size()
	tv := style.TV(c)
	height := float32(470)
	if tv {
		// On the television, 78 % of the screen, behind the bar (docs/tv.md §6.1).
		height = wh * 0.78
	}
	white := ui.RGB(255, 255, 255)
	frame := ui.Box(c).FillWidth().Height(height).Clip().Background(style.Color(design.DarkColors.SurfaceRaised)).DragWindow()
	if paused := frame.Hovered() && !tv; paused != h.heroPaused {
		if paused {
			h.heroShown = shown
		}
		h.heroPaused, h.heroSince = paused, now
	}
	frame.Children(func() {
		w, _ := c.Size()
		if bitmap := a.opts.Images.Get(card.GetBackdrop().GetUrl(), w, scale); bitmap != nil {
			ui.Image(c.Key(card.GetItemId()), bitmap).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover)
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 90,
			Stops: []ui.GradientStop{{At: 0, Color: ui.RGBA(0, 0, 0, 0.78)}, {At: 0.45, Color: ui.RGBA(0, 0, 0, 0.35)}, {At: 0.75, Color: ui.RGBA(0, 0, 0, 0)}},
		})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 0,
			Stops: []ui.GradientStop{{At: 0, Color: ui.RGBA(0, 0, 0, 0.55)}, {At: 0.45, Color: ui.RGBA(0, 0, 0, 0)}},
		})
		text := ui.Column(c).Absolute().Left(40).Bottom(40).MaxWidth(560).Gap(12)
		if tv {
			text.Left(style.Layout(c).Margin).Bottom(wh * 0.14).MaxWidth(860)
		}
		text.Children(func() {
			style.Text(ui.Text(c, strings.ToUpper(eyebrow)), style.Type(c).Footnote).FontWeight(600).LetterSpacing(0.4).TextColor(white.Alpha(0.75)).SingleLine()
			logo := false
			if u := card.GetLogo().GetUrl(); u != "" {
				logoH := float32(96)
				if tv {
					logoH = 160
				}
				if bitmap := a.opts.Images.Get(u, 600, scale); bitmap != nil {
					ui.Image(c, bitmap).MaxWidth(448).MaxHeight(logoH).Fit(ui.Contain).AlignSelf(ui.Start).Label(card.GetTitle())
					logo = true
				}
			}
			if !logo {
				size := float32(52)
				if tv {
					size = 72
				}
				style.Text(ui.Text(c, card.GetTitle()), style.Type(c).Display).FontSize(size).FontWeight(700).LineHeight(1.05).TextColor(white).MaxLines(2)
			}
			facts := []string{}
			if card.GetYear() > 0 {
				facts = append(facts, strconv.Itoa(int(card.GetYear())))
			}
			if r := card.GetRuntimeMs(); r > 0 {
				facts = append(facts, runtime(time.Duration(r)*time.Millisecond))
			}
			facts = append(facts, card.GetGenres()[:min(2, len(card.GetGenres()))]...)
			if len(facts) > 0 || card.GetOfficialRating() != "" {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					if len(facts) > 0 {
						style.Text(ui.Text(c, strings.Join(facts, " · ")), style.Type(c).Callout).TextColor(white.Alpha(0.8))
					}
					if rating := card.GetOfficialRating(); rating != "" {
						ui.Box(c).Padding(0, 6).Radius(4).Border(1, white.Alpha(0.5)).Children(func() {
							style.Text(ui.Text(c, rating), style.Type(c).Caption).TextColor(white.Alpha(0.9))
						})
					}
				})
			}
			if o := card.GetOverview(); o != "" {
				style.Text(ui.Text(c, o), style.Type(c).Body).TextColor(white.Alpha(0.85)).MaxLines(3)
			}
			ui.Row(c).Gap(12).Margin(8, 0, 0).Children(func() {
				if card.GetKind() != capiv1.CardKind_CARD_KIND_SERIES {
					label := style.T(c, "item.play")
					if card.HasProgress() && card.GetProgress() > 0 {
						label = style.T(c, "item.resume")
					}
					if a.heroButton(c, "play", label, true).Clicked() {
						a.play(card.GetServerId(), card.GetItemId(), nil)
					}
				}
				if a.heroButton(c, "info", style.T(c, "item.details"), false).Clicked() {
					a.open(card)
				}
			})
		})
		if len(entries) > 1 {
			dots := ui.Row(c).Absolute().Right(32).Bottom(32).Gap(6).AlignItems(ui.Center)
			if tv {
				dots.Right(style.Layout(c).Margin).Bottom(wh * 0.14)
			}
			dots.Children(func() {
				for j, e := range entries {
					dot := ui.ButtonBase(c.Key(j)).Size(6, 6).Radius(3).Background(white.Alpha(0.45)).Label(e.card.GetTitle())
					switch {
					case j == at:
						dot.Width(20).Background(white)
					case dot.Hovered():
						dot.Background(white.Alpha(0.7))
					}
					if dot.Clicked() {
						h.heroAt, h.heroSince, h.heroShown = j, now, 0
					}
				}
			})
		}
	})
}

// heroButton builds a button over artwork: white with black content when
// primary, else translucent. On the television it is 64 points high, and
// white belongs to focus alone (docs/tv.md §4).
func (a *App) heroButton(c *ui.Context, icon, label string, primary bool) ui.Element {
	b := ui.ButtonBase(c.Key(icon)).Height(44).Padding(0, 24).Gap(8).Radius(10).AlignItems(ui.Center).Label(label)
	if style.TV(c) {
		b.Height(64).Padding(0, 40).Gap(12).Radius(10)
		primary = a.tvFocus(c, b, false)
	}
	fg, bg := ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)
	if !primary {
		fg, bg = ui.RGB(255, 255, 255), ui.RGBA(255, 255, 255, 0.2)
	}
	if b.Hovered() && !style.TV(c) {
		bg = ui.RGBA(255, 255, 255, 0.9)
		if !primary {
			bg = ui.RGBA(255, 255, 255, 0.3)
		}
	}
	b.Background(bg)
	if b.Pressed() {
		b.Opacity(0.8)
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, true)).Size(18, 18).TextColor(fg)
		style.Text(ui.Text(c, label), style.Type(c).Headline).TextColor(fg).SingleLine()
	})
	return b
}

// runtime returns a length as "1h 52m" or "48m".
func runtime(d time.Duration) string {
	h, m := int(d.Hours()), int(d.Minutes())%60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	return fmt.Sprintf("%dm", m)
}

func joinFacts(facts []string) string {
	s := ""
	for i, f := range facts {
		if i > 0 {
			s += " · "
		}
		s += f
	}
	return s
}

// rowTitle returns what names a row.
func rowTitle(c *ui.Context, r *capiv1.Row) string {
	lib, seed := r.GetLibraryName(), r.GetSeedName()
	switch r.GetKind() {
	case capiv1.RowKind_ROW_KIND_CONTINUE_WATCHING:
		return style.T(c, "home.continueWatching")
	case capiv1.RowKind_ROW_KIND_AVAILABLE_REQUESTS:
		return style.T(c, "home.availableRequests")
	case capiv1.RowKind_ROW_KIND_RECENTLY_ADDED:
		return style.T(c, "home.latest", "library", lib)
	case capiv1.RowKind_ROW_KIND_RECENTLY_RELEASED:
		return style.T(c, "home.recentlyReleased", "library", lib)
	case capiv1.RowKind_ROW_KIND_RECENTLY_AIRED:
		return style.T(c, "home.recentlyAired", "library", lib)
	case capiv1.RowKind_ROW_KIND_TOP_RATED:
		return style.T(c, "home.topRated", "library", lib)
	case capiv1.RowKind_ROW_KIND_TOP_UNPLAYED:
		return style.T(c, "home.topUnplayed", "library", lib)
	case capiv1.RowKind_ROW_KIND_START_WATCHING:
		return style.T(c, "home.startWatching", "library", lib)
	case capiv1.RowKind_ROW_KIND_REDISCOVER:
		return style.T(c, "home.rediscover", "library", lib)
	case capiv1.RowKind_ROW_KIND_BECAUSE_YOU_WATCHED:
		return style.T(c, "home.becauseYouWatched", "title", seed)
	case capiv1.RowKind_ROW_KIND_BECAUSE_YOU_LIKE:
		return style.T(c, "home.becauseYouLike", "title", seed)
	case capiv1.RowKind_ROW_KIND_MORE_IN_GENRE:
		return style.T(c, "home.moreInGenre", "genre", seed)
	case capiv1.RowKind_ROW_KIND_STARRING:
		return style.T(c, "home.starring", "name", seed)
	case capiv1.RowKind_ROW_KIND_DIRECTED_BY:
		return style.T(c, "home.directedBy", "name", seed)
	case capiv1.RowKind_ROW_KIND_TRENDING:
		return style.T(c, "home.trending")
	}
	return r.GetServerName()
}

// row builds a row of Home or of a Recommended tab.
func (a *App) row(c *ui.Context, r *capiv1.Row) {
	colors := style.Colors(c)
	switch {
	case r.HasLoading():
		placeholderShelf(c.Key(r.GetId()), 8)
		return
	case r.HasUnavailable():
		ui.Row(c.Key(r.GetId())).Padding(0, style.Layout(c).Margin).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, style.Icon("offline", false)).Size(18, 18).TextColor(style.Color(colors.TextTertiary))
			style.Text(ui.Text(c, style.T(c, "home.serverUnreachable", "server", r.GetServerName())), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
		})
		return
	}
	title := rowTitle(c, r)
	if len(a.servers) > 1 && r.GetServerName() != "" && r.GetKind() != capiv1.RowKind_ROW_KIND_UNSPECIFIED {
		title += " · " + r.GetServerName()
	}
	var more func()
	if r.GetMore() && r.GetHubId() != "" {
		more = func() {
			a.router.Push("/hub/" + r.GetServerId() + "/" + r.GetHubId() + "?title=" + url.QueryEscape(title))
		}
	}
	if titles := r.GetTitles().GetTitles(); len(titles) > 0 {
		a.shelf(c, r.GetId(), title, nil, func() {
			for _, t := range titles {
				a.titleCard(c, r.GetServerId(), t)
			}
		})
		return
	}
	cards := r.GetItems().GetCards()
	if len(cards) == 0 {
		return
	}
	thumbs := r.GetKind() == capiv1.RowKind_ROW_KIND_CONTINUE_WATCHING || allWide(cards)
	a.shelf(c, r.GetId(), title, more, func() {
		for _, card := range cards {
			if thumbs {
				a.thumbCard(c, card, style.Layout(c).Thumbnail*0.85)
			} else {
				a.posterCard(c, card, style.Layout(c).Poster)
			}
		}
	})
}

func allWide(cards []*capiv1.Card) bool {
	for _, card := range cards {
		if !wide(card) {
			return false
		}
	}
	return true
}

// titleCard builds a poster of a title discovery offers, which opens it
// as the server discovers it.
func (a *App) titleCard(c *ui.Context, server string, t *capiv1.TitleCard) {
	colors := style.Colors(c)
	w := style.Layout(c).Poster
	b := ui.ButtonBase(c.Key(t.GetTitle() + fmt.Sprint(t.GetYear()))).Cursor(ui.CursorPointer).Column().Width(w).Gap(6).AlignItems(ui.Start).Label(t.GetTitle())
	hovered := a.cardFocus(c, b) || b.Hovered()
	if b.Pressed() {
		b.Opacity(0.8)
	}
	b.Children(func() {
		a.artwork(c, capiv1.Artwork_builder{Url: t.GetPosterUrl()}.Build(), "", nil, w, w*1.5, hovered)
		style.Text(ui.Text(c, t.GetTitle()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(w)
		if t.GetYear() > 0 {
			style.Text(ui.Text(c, fmt.Sprint(t.GetYear())), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine()
		}
	})
	tmdb, _ := strconv.Atoi(t.GetExternalIds()["tmdb"])
	if b.Clicked() && tmdb > 0 && server != "" {
		kind := acquisitionv1.MediaKind_MEDIA_KIND_SERIES
		if t.GetMovie() {
			kind = acquisitionv1.MediaKind_MEDIA_KIND_MOVIE
		}
		a.router.Push(titlePath(server, kind, int32(tmdb)))
	}
}

// hubPage builds the page of a hub's items, from a row's "more".
func (a *App) hubPage(c *ui.Context, r *ui.Route) {
	server, hub, title := r.Param("server"), r.Param("hub"), r.Query("title")
	r.Title(title)
	a.cardGrid(c, key("hub", server, hub), title, nil, func(ctx context.Context, offset, limit uint32) (*capiv1.Page, error) {
		m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{GetHub: capiv1.GetHub_builder{ServerId: server, HubId: hub, Offset: offset, Limit: limit}.Build()}.Build())
		return m.GetPage(), err
	})
}

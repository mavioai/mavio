package app

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// searchDelay is how long typing pauses before the search runs.
const searchDelay = 300 * time.Millisecond

// searchState is the search's state.
type searchState struct {
	query   string
	typed   string // the query when it last changed
	changed time.Time
	results task[*capiv1.SearchResults]
	// titles are the titles discovery finds, in the TV mode.
	titles task[[]*acquisitionv1.DiscoveredTitle]
}

// search builds the search: a field, and what each server found.
func (a *App) search(c *ui.Context) {
	s := &a.searchState
	colors := style.Colors(c)
	now := c.Now()
	if s.query != s.typed {
		s.typed, s.changed = s.query, now
	}
	query := strings.TrimSpace(s.query)
	if wait := searchDelay - now.Sub(s.changed); wait > 0 && query != "" {
		c.After(wait)
	} else if query != "" {
		load(a, &s.results, key("search", query, a.generation), func(ctx context.Context) (*capiv1.SearchResults, error) {
			m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{Search: capiv1.Search_builder{Query: query}.Build()}.Build())
			return m.GetSearchResults(), err
		})
	}
	if style.TV(c) {
		a.tvSearch(c, query)
		return
	}
	ui.Column(c).Fill().Children(func() {
		a.pageHeader(c, style.T(c, "nav.search"), func() {
			ui.SearchField(c, &s.query).Placeholder(style.T(c, "search.placeholder")).AutoFocus().Grow(1).MaxWidth(560).Height(36)
		})
		if query == "" {
			return
		}
		t := &s.results
		if taskStatus(c, t, func() { t.key = "" }, nil) {
			return
		}
		found := false
		for _, r := range t.value.GetServers() {
			found = found || len(r.GetCards()) > 0
		}
		if !found {
			statusView(c, "search", style.T(c, "search.noResults"), "", nil)
			return
		}
		width, _ := a.gridColumns(c)
		ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Grow(1).Children(func() {
			ui.Column(c).Padding(0, style.Layout(c).Margin, 40).Gap(24).Children(func() {
				for _, r := range t.value.GetServers() {
					if len(r.GetCards()) == 0 && r.GetError() == "" {
						continue
					}
					ui.Column(c.Key(r.GetServerId())).Gap(12).Children(func() {
						if len(t.value.GetServers()) > 1 {
							style.Text(ui.Text(c, r.GetServerName()), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
						}
						if r.GetError() != "" {
							style.Text(ui.Text(c, style.T(c, "home.serverUnreachable", "server", r.GetServerName())), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
						}
						ui.Row(c).Wrap().Gap(style.Layout(c).Gutter).Children(func() {
							for _, card := range r.GetCards() {
								a.posterCard(c, card, width)
							}
						})
					})
				}
			})
		})
	})
}

// tvKeys are the keys of the TV mode's search, as the web UI's.
var tvKeys = strings.Split(tvLetters+tvDigits, "")

// tvSearch builds the TV mode's search (docs/tv.md §6.4): on the left,
// staying in place, the query, Space, Delete and Clear, and the keys of
// the letters and digits, with focus as the page opens; on the right,
// Home's rows while there is no query, else what each server found and
// the titles discovery finds, in grids.
func (a *App) tvSearch(c *ui.Context, query string) {
	s := &a.searchState
	w, _ := c.Size()
	layout := style.Layout(c)
	left := min(max(w*0.3, 300), 480)
	// A keyboard types as well; Back deletes (tvBack).
	for _, k := range tvKeys {
		if c.Shortcut(0, tvKeyOf(k)) {
			s.query += k
		}
	}
	ui.Row(c).Fill().Padding(48, 0, 0, layout.Margin).Gap(layout.Gutter).AlignItems(ui.Start).Children(func() {
		a.inRegion("search", func() {
			ui.Column(c).Width(left).Shrink(0).Gap(16).Label(style.T(c, "nav.search")).Children(func() {
				// The page has no tabs, so focus starts on the keyboard,
				// on its first letter.
				a.tvQuery(c)
				ui.Row(c).Gap(8).Children(func() {
					for _, k := range []struct {
						key, label string
						f          func(string) string
					}{
						{"space", style.T(c, "search.space"), func(q string) string { return q + " " }},
						{"delete", style.T(c, "search.backspace"), dropLast},
						{"clear", style.T(c, "search.clear"), func(string) string { return "" }},
					} {
						b := tvKey(c, k.key, k.label, false, false).Grow(1).Basis(0)
						a.tvFocus(c, b, false)
						if b.Clicked() {
							s.query = k.f(s.query)
						}
					}
				})
				size := (left - 5*8) / 6
				ui.Column(c).Gap(8).Children(func() {
					for r := 0; r < len(tvKeys); r += 6 {
						ui.Row(c.Key(r)).Gap(8).Children(func() {
							for i, k := range tvKeys[r : r+6] {
								b := tvKey(c, k, strings.ToUpper(k), false, false).Grow(1).Basis(0).Height(size)
								if r+i == 0 {
									a.initialFocus(func() { a.tvFocus(c, b, false) })
								} else {
									a.tvFocus(c, b, false)
								}
								if b.Clicked() {
									s.query += k
								}
							}
						})
					}
				})
			})
		})
		// Its rows and grids keep only a little room for a focused card
		// to grow.
		was, width := c.Theme(), a.contentWidth
		c.SetTheme(style.Pane(was.Dark))
		a.contentWidth = w - layout.Margin - left - layout.Gutter
		ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Grow(1).Basis(0).MinWidth(0).FillHeight().Children(func() {
			ui.Column(c).Padding(0, 0, 40).Gap(32).Children(func() {
				if query == "" {
					a.loadHome()
					for _, r := range a.homeState.rows {
						a.row(c, r)
					}
					return
				}
				a.tvResults(c, query)
			})
		})
		a.contentWidth = width
		c.SetTheme(was)
	})
}

// tvKeyOf returns the keyboard's key of the letter or digit k.
func tvKeyOf(k string) ui.Key {
	if k[0] >= 'a' {
		return ui.KeyA + ui.Key(k[0]-'a')
	}
	return ui.Key0 + ui.Key(k[0]-'0')
}

// tvQuery builds the TV mode search's query, with a caret blinking where
// the keys type, wherever focus is.
func (a *App) tvQuery(c *ui.Context) {
	s := &a.searchState
	colors := style.Colors(c)
	now := c.Now()
	blink := now.Sub(a.searchState.changed) % time.Second
	c.After(time.Second/2 - blink%(time.Second/2))
	ui.Row(c).MinHeight(64).Padding(0, 20).Gap(12).Radius(design.RadiusLg).AlignItems(ui.Center).Background(style.Color(colors.SurfaceRaised)).Children(func() {
		ui.Icon(c, style.Icon("search", false)).Size(20, 20).Shrink(0).TextColor(style.Color(colors.TextSecondary))
		ui.Row(c).Grow(1).MinWidth(0).AlignItems(ui.Center).Children(func() {
			if s.query != "" {
				style.Text(ui.Text(c, s.query), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).SingleLine().Shrink(1).MinWidth(0)
			}
			caret := ui.Box(c).Width(2).Height(style.Type(c).Title3.Size*1.1).Margin(0, 1).Radius(1).Shrink(0).Background(style.Color(colors.Accent))
			if blink >= time.Second/2 {
				caret.Opacity(0)
			}
			if s.query == "" {
				style.Text(ui.Text(c, style.T(c, "search.placeholder")), style.Type(c).Title3).TextColor(style.Color(colors.TextTertiary)).SingleLine().Shrink(1).MinWidth(0)
			}
		})
	})
}

// tvResults builds what the TV mode's search found for query: in a grid
// what each server found, under its name when there are several, then
// the titles the server that discovers finds, to request.
func (a *App) tvResults(c *ui.Context, query string) {
	s := &a.searchState
	colors := style.Colors(c)
	layout := style.Layout(c)
	t := &s.results
	several := len(a.servers) > 1
	width, _ := a.gridColumns(c)
	server, catalogs, pending := a.discovery(c)
	found := &s.titles
	var titles []*acquisitionv1.DiscoveredTitle
	// They are asked for once the query has settled, as the search.
	if server != "" && len(catalogs) > 0 && t.key == key("search", query, a.generation) {
		language := discoverLanguage(c)
		k := key("titles", server, query, language, a.generation)
		load(a, found, k, func(ctx context.Context) ([]*acquisitionv1.DiscoveredTitle, error) {
			res, err := a.discoverService(server).SearchTitles(ctx, acquisitionv1.SearchTitlesRequest_builder{Query: &query, Language: &language}.Build())
			return res.GetTitles(), err
		})
		if found.loaded && !found.loading {
			titles = found.value
		}
	} else {
		server = ""
	}
	grid := func(id, title string, cards func()) {
		ui.Column(c.Key(id)).Gap(8).Children(func() {
			if title != "" {
				style.Text(ui.Text(c, title), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).Margin(0, layout.Margin)
			}
			ui.Row(c).Wrap().Padding(0, layout.Margin).GapX(layout.Gutter).GapY(24).Children(cards)
		})
	}
	none := t.loaded && t.err == nil && !pending && (server == "" || !found.loading) && len(titles) == 0
	switch {
	case !t.loaded:
		placeholderShelf(c, 6)
	case t.err != nil:
		taskStatus(c, t, func() { t.key = "" }, nil)
		none = false
	default:
		for _, r := range t.value.GetServers() {
			switch {
			case r.GetError() != "":
				ui.Row(c.Key(r.GetServerId())).Padding(12, layout.Margin).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, style.Icon("offline", false)).Size(18, 18).TextColor(style.Color(colors.TextTertiary))
					style.Text(ui.Text(c, style.T(c, "home.serverUnreachable", "server", r.GetServerName())), style.Type(c).Callout).TextColor(style.Color(colors.TextTertiary))
				})
			case len(r.GetCards()) > 0:
				none = false
				title := ""
				if several {
					title = r.GetServerName()
				}
				grid(r.GetServerId(), title, func() {
					for _, card := range r.GetCards() {
						a.posterCard(c, card, width)
					}
				})
			}
		}
	}
	if server != "" && len(titles) > 0 {
		grid("discover", style.T(c, "search.discover"), func() {
			for _, title := range titles {
				a.discoveredCard(c, server, title, width, "")
			}
		})
	}
	if none {
		statusView(c, "search", style.T(c, "search.noResults"), "", nil)
	}
}

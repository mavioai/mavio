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
	query string
	// ran is the query confirmed, which the search runs outside the TV
	// mode.
	ran     string
	typed   string // the query when it last changed
	changed time.Time
	results task[searched]
	// titles are the titles discovery finds.
	titles task[[]*acquisitionv1.DiscoveredTitle]
}

// searched is what the servers found for a query.
type searched struct {
	query   string
	results *capiv1.SearchResults
}

// search builds the search: a field, confirmed with Return, and what each
// server found (docs/clients.md §5.4); in the TV mode it runs as the
// keyboard types.
func (a *App) search(c *ui.Context) {
	s := &a.searchState
	tv := style.TV(c)
	query := s.ran
	if tv {
		now := c.Now()
		if s.query != s.typed {
			s.typed, s.changed = s.query, now
		}
		query = strings.TrimSpace(s.query)
		if wait := searchDelay - now.Sub(s.changed); wait > 0 && query != "" {
			c.After(wait)
			a.tvSearch(c, query)
			return
		}
	}
	if query != "" {
		load(a, &s.results, key("search", query, a.generation), func(ctx context.Context) (searched, error) {
			m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{Search: capiv1.Search_builder{Query: query}.Build()}.Build())
			return searched{query, m.GetSearchResults()}, err
		})
	}
	if tv {
		a.tvSearch(c, query)
		return
	}
	ui.Column(c).Fill().Children(func() {
		a.pageHeader(c, style.T(c, "nav.search"), func() {
			if ui.SearchField(c, &s.query).Placeholder(style.T(c, "search.placeholder")).AutoFocus().Grow(1).MaxWidth(560).Height(36).Submitted() {
				// Results are kept, coming back to them; confirming the
				// query again searches again.
				if q := strings.TrimSpace(s.query); q == s.ran {
					s.results.key, s.titles.key = "", ""
				} else {
					s.ran = q
				}
			}
		})
		if query == "" {
			return
		}
		ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Grow(1).Children(func() {
			ui.Column(c).Padding(0, 0, 40).Gap(24).Children(func() {
				a.searchResults(c, query, false)
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
				a.searchResults(c, query, true)
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

// searchResults builds what the search found for query, as the web
// UI's: a row of each server that answered with results, in their order,
// titled by its name when there are several, a line for one that cannot
// be reached and nothing for one that found nothing, while it searches or
// after;
// then the titles the first server that discovers finds, to request
// (§6.3.28). The TV mode has grids in place of rows.
func (a *App) searchResults(c *ui.Context, query string, grid bool) {
	s := &a.searchState
	colors := style.Colors(c)
	layout := style.Layout(c)
	t := &s.results
	several := len(a.servers) > 1
	width, _ := a.gridColumns(c)
	if !grid {
		width = layout.Poster
	}
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
	section := func(id, title string, cards func()) {
		if !grid {
			a.shelf(c, "search/"+id, title, nil, cards)
			return
		}
		ui.Column(c.Key(id)).Gap(8).Children(func() {
			if title != "" {
				style.Text(ui.Text(c, title), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).Margin(0, layout.Margin)
			}
			ui.Row(c).Wrap().Padding(0, layout.Margin).GapX(layout.Gutter).GapY(24).Children(cards)
		})
	}
	title := func(name string) string {
		if several {
			return name
		}
		if grid {
			return ""
		}
		return style.T(c, "search.server")
	}
	searching := !t.loaded || t.value.query != query
	shown := len(titles) > 0
	if !searching && t.err == nil {
		for _, r := range t.value.results.GetServers() {
			shown = shown || r.GetError() != "" || len(r.GetCards()) > 0
		}
	}
	// One indicator while nothing is drawn yet, and No Results once all
	// answered with nothing.
	if !shown && (searching || t.err == nil) {
		if searching || pending || server != "" && (!found.loaded || found.loading) {
			loading(c, t.started)
		} else {
			statusView(c, "search", style.T(c, "search.noResults"), "", nil)
		}
		return
	}
	switch {
	case searching:
	case t.err != nil:
		taskStatus(c, t, func() { t.key = "" }, nil)
	default:
		// A server's row shows once it answers with results, never while
		// it searches.
		for _, r := range t.value.results.GetServers() {
			switch {
			case r.GetError() != "":
				ui.Row(c.Key(r.GetServerId())).Padding(12, layout.Margin).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, style.Icon("offline", false)).Size(18, 18).TextColor(style.Color(colors.TextTertiary))
					style.Text(ui.Text(c, style.T(c, "home.serverUnreachable", "server", r.GetServerName())), style.Type(c).Callout).TextColor(style.Color(colors.TextTertiary))
				})
			case len(r.GetCards()) > 0:
				section(r.GetServerId(), title(r.GetServerName()), func() {
					for _, card := range r.GetCards() {
						a.posterCard(c, card, width)
					}
				})
			}
		}
	}
	if server != "" && len(titles) > 0 {
		section("discover", style.T(c, "search.discover"), func() {
			for _, title := range titles {
				a.discoveredCard(c, server, title, width, "")
			}
		})
	}
}

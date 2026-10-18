package app

import (
	"context"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"

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

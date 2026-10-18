package app

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// wantedState is the state of the settings' Wanted: the monitored movies
// or episodes missing a file or below their profile's cutoff, as shown,
// and what was searched for.
type wantedState struct {
	list     task[*acquisitionv1.ListWantedResponse]
	filter   int // the index of wantedFilters shown
	kind     int // the index of wantedKinds shown
	limit    int
	queued   *int32 // how many searches searching all queued
	synced   bool
	busy     string
	err      error
	searched map[string]bool  // the items searched for
	errs     map[string]error // of searching or unmonitoring an item
}

var (
	wantedFilters = []acquisitionv1.WantedFilter{acquisitionv1.WantedFilter_WANTED_FILTER_MISSING, acquisitionv1.WantedFilter_WANTED_FILTER_CUTOFF_UNMET}
	wantedKinds   = []acquisitionv1.WantedKind{acquisitionv1.WantedKind_WANTED_KIND_MOVIE, acquisitionv1.WantedKind_WANTED_KIND_EPISODE}
)

// wantedPage is how many more of the wanted a page shows.
const wantedPage = 50

func (a *App) wantedService(id string) acquisitionv1connect.WantedServiceClient {
	return acquisitionv1connect.NewWantedServiceClient(a.connectHTTP(), serverURL(id))
}

// episodeCode returns an episode's numbers, as "S01E02".
func episodeCode(season, episode int32) string {
	return fmt.Sprintf("S%02dE%02d", season, episode)
}

func (a *App) wantedSection(c *ui.Context) {
	st := &a.settingsState.admin.wanted
	if st.limit == 0 {
		st.limit = wantedPage
	}
	sectionHeader(c, style.T(c, "settings.wanted"), style.T(c, "wanted.message"))
	filter, kind, limit := wantedFilters[st.filter], wantedKinds[st.kind], int32(st.limit)
	t := adminTask(a, &st.list, "wanted", func(ctx context.Context, s string) (*acquisitionv1.ListWantedResponse, error) {
		return a.wantedService(s).ListWanted(ctx, acquisitionv1.ListWantedRequest_builder{Filter: &filter, Kind: &kind, Limit: &limit}.Build())
	}, st.filter, st.kind, st.limit)
	total := t.value.GetTotal()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		changed := choose(c, style.T(c, "wanted.filter"), &st.filter, []string{style.T(c, "wanted.missing"), style.T(c, "wanted.cutoffUnmet")})
		if choose(c, style.T(c, "wanted.kind"), &st.kind, []string{style.T(c, "wanted.movies"), style.T(c, "wanted.episodes")}) || changed {
			st.queued, st.limit = nil, wantedPage
		}
		ui.Box(c).Grow(1)
		label := style.T(c, "wanted.rssSync")
		if st.busy == "rss" {
			label = "…"
		}
		if smallButton(c, label, st.busy == "rss") {
			a.wantedAction("rss", func(ctx context.Context, s string) error {
				_, err := a.wantedService(s).SyncRSS(ctx, &acquisitionv1.SyncRSSRequest{})
				return err
			}, func() { st.synced = true })
		}
		label = style.T(c, "wanted.searchAll")
		if st.busy == "search" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(st.busy == "search" || total == 0).Clicked() {
			var queued int32
			a.wantedAction("search", func(ctx context.Context, s string) error {
				res, err := a.wantedService(s).SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{Filter: &filter, Kind: &kind}.Build())
				queued = res.GetQueued()
				return err
			}, func() { st.queued = &queued })
		}
	})
	var notes []string
	if st.queued != nil {
		notes = append(notes, style.T(c, "wanted.queued", "count", *st.queued))
	}
	if st.synced {
		notes = append(notes, style.T(c, "wanted.rssQueued"))
	}
	if len(notes) > 0 {
		secondaryText(c, joinDetail(notes...))
	}
	if st.err != nil {
		dangerText(c, errorMessage(c, st.err))
	}
	footer := ""
	if t.loaded && t.err == nil {
		footer = style.T(c, "wanted.count", "count", total)
	}
	group(c, "wanted", "", footer, func(g *settingsGroup) {
		shown := 0
		if kind == acquisitionv1.WantedKind_WANTED_KIND_MOVIE {
			for _, m := range t.value.GetMovies() {
				label := m.GetName()
				if m.GetYear() > 0 {
					label = fmt.Sprintf("%s (%d)", m.GetName(), m.GetYear())
				}
				var detail []string
				if m.GetQuality() != "" {
					detail = append(detail, style.T(c, "wanted.has", "quality", m.GetQuality()))
				}
				if !m.GetAvailable() {
					detail = append(detail, style.T(c, "wanted.notAvailable"))
				}
				a.wantedRow(g, m.GetItemId(), true, label, joinDetail(detail...))
			}
			shown = len(t.value.GetMovies())
		} else {
			for _, e := range t.value.GetEpisodes() {
				label := e.GetSeriesName() + " · " + episodeCode(e.GetSeasonNumber(), e.GetEpisodeNumber())
				if e.GetName() != "" {
					label += " · " + e.GetName()
				}
				var detail []string
				if e.HasAirTime() {
					detail = append(detail, mediumDate(e.GetAirTime().AsTime()))
				}
				if e.GetAbsoluteNumber() > 0 {
					detail = append(detail, style.T(c, "wanted.absolute", "number", e.GetAbsoluteNumber()))
				}
				if e.GetQuality() != "" {
					detail = append(detail, style.T(c, "wanted.has", "quality", e.GetQuality()))
				}
				a.wantedRow(g, e.GetItemId(), false, label, joinDetail(detail...))
			}
			shown = len(t.value.GetEpisodes())
		}
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && total == 0:
			g.note(style.T(c, "wanted.none"), false)
		case int32(shown) < total && g.more(style.T(c, "activity.showMore")):
			st.limit += wantedPage
		}
	})
}

// wantedAction runs a change of the wanted in the background, what
// showing busy until it ends; done runs on the main thread if it
// succeeded.
func (a *App) wantedAction(what string, fn func(ctx context.Context, server string) error, done func()) {
	st := &a.settingsState.admin.wanted
	st.busy, st.err = what, nil
	server := a.chosenServer().GetId()
	go func() {
		err := fn(a.ctx, server)
		a.opts.Update(func() {
			st.busy, st.err = "", err
			if err == nil {
				a.settingsState.admin.reload++
				done()
			}
		})
	}()
}

// wantedRow adds a wanted movie's or episode's row: its name opening its
// details, what it has, and searching for it and unmonitoring it.
func (a *App) wantedRow(g *settingsGroup, id string, movie bool, label, detail string) {
	c := g.c
	st := &a.settingsState.admin.wanted
	colors := style.Colors(c)
	server := a.chosenServer().GetId()
	g.divider()
	ui.Row(c.Key(id)).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			a.itemLink(c, id, label, style.Type(c).Body)
			if detail != "" {
				style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
			}
			if err := st.errs[id]; err != nil {
				style.Text(ui.Text(c, errorMessage(c, err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
		})
		act := func(what string, fn func(ctx context.Context, w acquisitionv1connect.WantedServiceClient) error) {
			if st.errs == nil {
				st.errs = map[string]error{}
			}
			st.busy = what + id
			delete(st.errs, id)
			go func() {
				err := fn(a.ctx, a.wantedService(server))
				a.opts.Update(func() {
					st.busy = ""
					if err != nil {
						st.errs[id] = err
						return
					}
					if what == "search" {
						if st.searched == nil {
							st.searched = map[string]bool{}
						}
						st.searched[id] = true
					}
					a.settingsState.admin.reload++
				})
			}()
		}
		if linkButton(c, style.T(c, "wanted.unmonitor")).Disabled(st.busy == "unmonitor"+id).Clicked() && st.busy != "unmonitor"+id {
			act("unmonitor", func(ctx context.Context, w acquisitionv1connect.WantedServiceClient) error {
				if !movie {
					_, err := w.SetMonitored(ctx, acquisitionv1.SetMonitoredRequest_builder{
						Items: acquisitionv1.MonitoredItems_builder{ItemIds: []string{id}, Monitored: new(false)}.Build(),
					}.Build())
					return err
				}
				// Movies are monitored through their wanted.
				res, err := w.GetWanted(ctx, acquisitionv1.GetWantedRequest_builder{ItemId: &id}.Build())
				if err != nil {
					return err
				}
				next := proto.CloneOf(res.GetWanted())
				next.SetMonitored(false)
				_, err = w.UpdateWanted(ctx, acquisitionv1.UpdateWantedRequest_builder{Wanted: next}.Build())
				return err
			})
		}
		label := style.T(c, "wanted.search")
		if st.searched[id] {
			label = style.T(c, "wanted.searching")
		}
		if smallButton(c, label, st.searched[id] || st.busy == "search"+id) {
			act("search", func(ctx context.Context, w acquisitionv1connect.WantedServiceClient) error {
				_, err := w.SearchWanted(ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: []string{id}}.Build())
				return err
			})
		}
	})
}

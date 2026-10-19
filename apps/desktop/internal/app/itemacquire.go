package app

import (
	"context"
	"slices"
	"strings"

	"connectrpc.com/connect"
	"github.com/egoist/mygo/ui"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// acquireState is the state of an item's acquisition on its details.
type acquireState struct {
	wanted   task[*acquisitionv1.Wanted]
	queue    task[[]*acquisitionv1.QueueItem]
	history  task[[]*acquisitionv1.HistoryEvent]
	reload   int
	busy     bool
	err      error
	searched bool
	releases bool // the releases dialog is open
	found    task[*acquisitionv1.SearchReleasesResponse]
	search   int // bumped to search the indexers again
	grabs    map[string]*grab
}

// grab is a release grabbed from the releases dialog.
type grab struct {
	busy, done bool
	err        error
}

// acquirable reports whether an item's acquisition is shown: a movie, a
// series, a season or an episode.
func acquirable(k libraryv1.ItemKind) bool {
	switch k {
	case libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_SERIES, libraryv1.ItemKind_ITEM_KIND_SEASON, libraryv1.ItemKind_ITEM_KIND_EPISODE:
		return true
	}
	return false
}

// whole reports whether an item is wanted on its own, a movie or a
// series, rather than monitored as a part of one.
func whole(k libraryv1.ItemKind) bool {
	return k == libraryv1.ItemKind_ITEM_KIND_MOVIE || k == libraryv1.ItemKind_ITEM_KIND_SERIES
}

// asItemServer builds fn with the downloads' rows, which act on the
// server chosen in the settings, acting on server.
func (a *App) asItemServer(server string, fn func()) {
	prev := a.itemServer
	a.itemServer = server
	defer func() { a.itemServer = prev }()
	fn()
}

// itemQueue loads the downloads of an item not finished yet.
func (a *App) itemQueue(st *acquireState, server, id string) *task[[]*acquisitionv1.QueueItem] {
	return load(a, &st.queue, key("item-queue", server, id, a.generation, st.reload, a.settingsState.admin.reload), func(ctx context.Context) ([]*acquisitionv1.QueueItem, error) {
		res, err := a.downloadService(server).ListQueue(ctx, acquisitionv1.ListQueueRequest_builder{ItemId: &id}.Build())
		return res.GetItems(), err
	})
}

// downloadBadge builds how much of a movie or series is downloaded while
// it downloads, beside its header's badges.
func (a *App) downloadBadge(c *ui.Context, st *acquireState, server string, it *libraryv1.Item) {
	if !a.isAdmin(server) || !whole(it.GetKind()) {
		return
	}
	var size, left int64
	for _, d := range a.itemQueue(st, server, it.GetId()).value {
		if d.GetState() == acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING && d.GetSize() > 0 {
			size += d.GetSize()
			left += d.GetRemainingSize()
		}
	}
	if size == 0 {
		return
	}
	percent := int(max(0, min(1, 1-float64(left)/float64(size)))*100 + 0.5)
	style.Text(ui.Text(c, style.T(c, "discover.downloading", "percent", percent)), style.Type(c).Caption).FontWeight(600).
		TextColor(ui.RGB(255, 255, 255)).Padding(0, 6).Radius(4).Background(style.Color(style.Colors(c).Scrim))
}

// requestMore builds Request More of a series, as on its discovered
// title, while it has seasons to request.
func (a *App) requestMore(c *ui.Context, server string, it *libraryv1.Item) {
	tmdb := tmdbOfItem(it)
	if tmdb == 0 {
		return
	}
	title := a.discoveredTitle(c, server, acquisitionv1.MediaKind_MEDIA_KIND_SERIES, tmdb).value
	if !title.GetRequestable() || !slices.ContainsFunc(title.GetSeasons(), (*acquisitionv1.SeasonStatus).GetRequestable) {
		return
	}
	if a.moreLink(c.Key("request-more"), style.T(c, "discover.requestMore")).Clicked() && !a.discoverState.busy {
		a.askSeasons(title, nil)
	}
}

// serverUsers loads the users of a server, as the requests list them.
func (a *App) serverUsers(server string) map[string]string {
	st := &a.discoverState
	if st.users == nil {
		st.users = map[string]*task[[]*userv1.User]{}
	}
	u := st.users[server]
	if u == nil {
		u = &task[[]*userv1.User]{}
		st.users[server] = u
	}
	load(a, u, key("users", server, a.generation), func(ctx context.Context) ([]*userv1.User, error) {
		res, err := a.userService(server).ListUsers(ctx, &userv1.ListUsersRequest{})
		return res.GetUsers(), err
	})
	names := map[string]string{}
	for _, x := range u.value {
		names[x.GetId()] = x.GetName()
	}
	return names
}

// acquisition builds an item's acquisition on its details, as the web
// UI's, for administrators: whether it is monitored, its quality profile
// and last search, searching for it automatically or among the releases
// the indexers have, and its requests, downloads and history.
func (a *App) acquisition(c *ui.Context, st *acquireState, server string, it *libraryv1.Item) {
	if !a.isAdmin(server) || !acquirable(it.GetKind()) {
		return
	}
	colors := style.Colors(c)
	id, kind := it.GetId(), it.GetKind()
	isWhole := whole(kind)
	var wanted *task[*acquisitionv1.Wanted]
	if isWhole {
		wanted = load(a, &st.wanted, key("wanted", server, id, a.generation, st.reload), func(ctx context.Context) (*acquisitionv1.Wanted, error) {
			res, err := a.wantedService(server).GetWanted(ctx, acquisitionv1.GetWantedRequest_builder{ItemId: &id}.Build())
			if connect.CodeOf(err) == connect.CodeNotFound {
				return nil, nil
			}
			return res.GetWanted(), err
		})
	}
	queue := a.itemQueue(st, server, id)
	history := load(a, &st.history, key("item-history", server, id, a.generation, st.reload, a.settingsState.admin.reload), func(ctx context.Context) ([]*acquisitionv1.HistoryEvent, error) {
		limit := int32(10)
		res, err := a.downloadService(server).ListHistory(ctx, acquisitionv1.ListHistoryRequest_builder{ItemId: &id, Limit: &limit}.Build())
		return res.GetEvents(), err
	})
	monitored := it.GetMonitored()
	if isWhole {
		monitored = wanted.value.GetMonitored()
	}
	notWanted := isWhole && wanted.loaded && wanted.err == nil && wanted.value == nil
	var requests []*acquisitionv1.Request
	mediaKind := mediaKindOfItem(it)
	if isWhole && tmdbOfItem(it) > 0 {
		requests = a.discoveredTitle(c, server, mediaKind, tmdbOfItem(it)).value.GetRequests()
	}
	ui.Column(c).Padding(4, style.Layout(c).Margin, 0).Gap(16).Children(func() {
		style.Text(ui.Text(c, style.T(c, "acquire.title")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
		ui.Column(c).MaxWidth(768).Gap(24).Children(func() {
			if notWanted {
				group(c, "acquire", "", style.T(c, "acquire.notWantedFooter"), func(g *settingsGroup) {
					g.row(style.T(c, "acquire.notWanted"), "", nil, func() {
						// A release can be grabbed for a movie not wanted, not for a series.
						if kind == libraryv1.ItemKind_ITEM_KIND_MOVIE && smallButton(c, style.T(c, "acquire.interactive"), false) {
							st.releases = true
						}
					})
				})
			} else {
				group(c, "acquire", "", "", func(g *settingsGroup) {
					on := monitored
					if g.switchRow(style.T(c, "acquire.monitored"), "", &on) && !st.busy {
						a.setItemMonitored(st, server, it, wanted.value, on)
					}
					if isWhole && wanted.value != nil {
						a.profileRow(g, st, server, wanted.value, mediaKind)
						if wanted.value.HasLastSearchTime() {
							g.row(style.T(c, "acquire.lastSearch"), "", nil, func() {
								style.Text(ui.Text(c, fromNow(c, wanted.value.GetLastSearchTime().AsTime())), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
							})
						}
					}
					g.divider()
					ui.Row(c.Key("search")).Padding(12, 16).Gap(8).Wrap().Children(func() {
						label := style.T(c, "acquire.search")
						if st.searched {
							label = style.T(c, "wanted.searching")
						}
						if smallButton(c, label, st.busy || st.searched) {
							a.searchItem(st, server, id)
						}
						if smallButton(c, style.T(c, "acquire.interactive"), false) {
							st.releases = true
						}
					})
					if st.err != nil {
						g.note(errorMessage(c, st.err), true)
					}
					if isWhole && wanted.err != nil {
						g.note(errorMessage(c, wanted.err), true)
					}
				})
			}
			if len(requests) > 0 {
				names := a.serverUsers(server)
				group(c, "requests", style.T(c, "requests.title"), "", func(g *settingsGroup) {
					for _, r := range requests {
						g.divider()
						a.requestRow(c, server, r, true, names[r.GetRequestedBy()])
					}
				})
			}
			a.asItemServer(server, func() {
				if items := queue.value; len(items) > 0 {
					group(c, "queue", style.T(c, "downloads.queue"), "", func(g *settingsGroup) {
						for _, i := range items {
							g.divider()
							ui.Column(c.Key(i.GetId())).Padding(12, 16).Gap(6).Children(func() { a.queueRow(c, i) })
						}
					})
				}
				if events := history.value; len(events) > 0 {
					group(c, "history", style.T(c, "downloads.history"), "", func(g *settingsGroup) {
						for _, e := range events {
							g.divider()
							ui.Column(c.Key(e.GetId())).Padding(12, 16).Gap(6).Children(func() { a.historyRow(c, e) })
						}
					})
				}
			})
		})
	})
	a.releasesDialog(c, st, server, it)
}

// setItemMonitored monitors an item or stops: a movie or series through
// its wanted, a season or episode on its own.
func (a *App) setItemMonitored(st *acquireState, server string, it *libraryv1.Item, wanted *acquisitionv1.Wanted, on bool) {
	st.busy, st.err = true, nil
	id := it.GetId()
	go func() {
		var err error
		if wanted != nil {
			w := cloneWanted(wanted)
			w.SetMonitored(on)
			_, err = a.wantedService(server).UpdateWanted(a.ctx, acquisitionv1.UpdateWantedRequest_builder{Wanted: w}.Build())
		} else {
			_, err = a.wantedService(server).SetMonitored(a.ctx, acquisitionv1.SetMonitoredRequest_builder{
				Items: acquisitionv1.MonitoredItems_builder{ItemIds: []string{id}, Monitored: &on}.Build(),
			}.Build())
		}
		a.opts.Update(func() {
			st.busy, st.err = false, err
			st.reload++
			if wanted == nil && err == nil {
				a.generation++
			}
		})
	}()
}

// cloneWanted returns a copy of a wanted item to change.
func cloneWanted(w *acquisitionv1.Wanted) *acquisitionv1.Wanted {
	return acquisitionv1.Wanted_builder{
		ItemId: new(w.GetItemId()), Monitored: new(w.GetMonitored()), QualityProfileId: new(w.GetQualityProfileId()),
		MinimumAvailability: new(w.GetMinimumAvailability()), Tags: w.GetTags(), AddTime: w.GetAddTime(),
		LastSearchTime: w.GetLastSearchTime(), MonitorNewItems: new(w.GetMonitorNewItems()), SeasonFolder: new(w.GetSeasonFolder()),
	}.Build()
}

// profileRow adds the row choosing a wanted item's quality profile.
func (a *App) profileRow(g *settingsGroup, st *acquireState, server string, w *acquisitionv1.Wanted, kind acquisitionv1.MediaKind) {
	c := g.c
	var profiles []*acquisitionv1.QualityProfile
	a.asItemServer(server, func() {
		for _, p := range a.qualityProfiles().value {
			if p.GetKind() == kind {
				profiles = append(profiles, p)
			}
		}
	})
	g.row(style.T(c, "acquire.profile"), "", nil, func() {
		labels := make([]string, len(profiles))
		current := -1
		for i, p := range profiles {
			labels[i] = p.GetName()
			if p.GetId() == w.GetQualityProfileId() {
				current = i
			}
		}
		if len(labels) == 0 {
			return
		}
		chosen := ""
		if current >= 0 {
			chosen = labels[current]
		}
		if selectOf(c, &chosen, labels).MinWidth(200).Label(style.T(c, "acquire.profile")).Changed() && !st.busy {
			i := slices.Index(labels, chosen)
			if i < 0 || i == current {
				return
			}
			next := cloneWanted(w)
			next.SetQualityProfileId(profiles[i].GetId())
			st.busy, st.err = true, nil
			go func() {
				_, err := a.wantedService(server).UpdateWanted(a.ctx, acquisitionv1.UpdateWantedRequest_builder{Wanted: next}.Build())
				a.opts.Update(func() {
					st.busy, st.err = false, err
					st.reload++
				})
			}()
		}
	})
}

// searchItem searches for an item and grabs the best release.
func (a *App) searchItem(st *acquireState, server, id string) {
	st.busy, st.err = true, nil
	go func() {
		_, err := a.wantedService(server).SearchWanted(a.ctx, acquisitionv1.SearchWantedRequest_builder{ItemIds: []string{id}}.Build())
		a.opts.Update(func() {
			st.busy, st.err, st.searched = false, err, err == nil
		})
	}()
}

// releasesDialog builds the releases the indexers have of an item, each
// grabbed in place.
func (a *App) releasesDialog(c *ui.Context, st *acquireState, server string, it *libraryv1.Item) {
	if !st.releases {
		return
	}
	colors := style.Colors(c)
	id := it.GetId()
	found := load(a, &st.found, key("releases", server, id, st.search), func(ctx context.Context) (*acquisitionv1.SearchReleasesResponse, error) {
		return a.releaseService(server).SearchReleases(ctx, acquisitionv1.SearchReleasesRequest_builder{ItemId: &id}.Build())
	})
	title := style.T(c, "acquire.releasesTitle", "name", it.GetName())
	ui.Modal(c, &st.releases, func() {
		ui.Column(c).Width(min(960, a.contentWidth-64)).MaxHeight(720).Gap(12).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				style.Text(ui.Text(c, title), style.Type(c).Title2).TextColor(style.Color(colors.TextPrimary)).Grow(1).Shrink(1)
				label := style.T(c, "logs.refresh")
				if !found.loaded {
					label = "…"
				}
				if smallButton(c, label, !found.loaded) {
					st.search++
					st.grabs = nil
				}
				if linkButton(c, style.T(c, "keys.done")).Clicked() {
					st.releases = false
				}
			})
			for _, f := range found.value.GetFailures() {
				footnote(c.Key(f.GetPluginId()+"/"+f.GetIndexerId()), style.T(c, "acquire.indexerFailed", "indexer", f.GetIndexerId(), "message", f.GetMessage()), true)
			}
			ui.Box(c).Height(1).Background(style.Color(colors.Border))
			ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Children(func() {
					list := found.value.GetReleases()
					switch {
					case !found.loaded:
						secondaryText(c, style.T(c, "acquire.searchingIndexers"))
					case found.err != nil:
						footnote(c, errorMessage(c, found.err), true)
					case len(list) == 0:
						secondaryText(c, style.T(c, "acquire.noReleases"))
					}
					for i, r := range list {
						if i > 0 {
							ui.Box(c.Key(key("divider", i))).Height(1).Background(style.Color(colors.Border))
						}
						a.releaseRow(c.Key(r.GetIndexerId()+"/"+r.GetGuid()), st, server, id, r)
					}
				})
			})
		})
	})
}

// releaseRow builds a release: its title, quality, languages, size, age,
// peers and indexer, its custom formats and why it would be rejected, and
// grabbing it.
func (a *App) releaseRow(c *ui.Context, st *acquireState, server, id string, r *acquisitionv1.Release) {
	colors := style.Colors(c)
	k := r.GetIndexerId() + "/" + r.GetGuid()
	if st.grabs == nil {
		st.grabs = map[string]*grab{}
	}
	g := st.grabs[k]
	if g == nil {
		g = &grab{}
		st.grabs[k] = g
	}
	detail := []string{r.GetParsed().GetQuality().GetName(), strings.Join(r.GetParsed().GetLanguages(), ", ")}
	if r.GetSize() > 0 {
		detail = append(detail, sizeOf(r.GetSize()))
	}
	if r.HasPublishDate() {
		detail = append(detail, fromNow(c, r.GetPublishDate().AsTime()))
	}
	if r.GetProtocol() == acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT {
		detail = append(detail, style.T(c, "acquire.peers", "seeders", r.GetSeeders(), "peers", r.GetPeers()))
	} else {
		detail = append(detail, style.T(c, "acquire.usenet"))
	}
	detail = append(detail, r.GetIndexerName())
	formats := slices.Clone(r.GetCustomFormats())
	if len(formats) > 0 || r.GetCustomFormatScore() != 0 {
		formats = append(formats, style.T(c, "acquire.score", "score", r.GetCustomFormatScore()))
	}
	rejected := len(r.GetRejections()) > 0
	ui.Row(c).Padding(12, 0).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			fg := colors.TextPrimary
			if rejected {
				fg = colors.TextSecondary
			}
			style.Text(ui.Text(c, r.GetTitle()), style.Type(c).Callout).TextColor(style.Color(fg)).Selectable()
			footnote(c, joinDetail(detail...), false)
			if len(formats) > 0 {
				footnote(c, strings.Join(formats, " · "), false)
			}
			for i, x := range r.GetRejections() {
				footnote(c.Key(key("rejection", i)), cmpOr(x.GetMessage(), x.GetReason()), true)
			}
			if g.err != nil {
				footnote(c, errorMessage(c, g.err), true)
			}
		})
		label := style.T(c, "acquire.grab")
		switch {
		case g.done:
			label = style.T(c, "downloads.grabbed")
		case g.busy:
			label = "…"
		}
		var clicked bool
		if rejected {
			clicked = smallButton(c, label, g.busy || g.done)
		} else {
			clicked = ui.PrimaryButton(c, label).Disabled(g.busy || g.done).Clicked()
		}
		if clicked && !g.busy && !g.done {
			g.busy, g.err = true, nil
			go func() {
				_, err := a.downloadService(server).GrabRelease(a.ctx, acquisitionv1.GrabReleaseRequest_builder{ItemId: &id, Release: r}.Build())
				a.opts.Update(func() {
					g.busy, g.err, g.done = false, err, err == nil
					if err == nil {
						st.reload++
					}
				})
			}()
		}
	})
}

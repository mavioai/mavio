package app

import (
	"context"
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

// The indexer search, an advanced tool (docs/clients.md §6.3.21), as the
// web UI's: a search of the indexers by text, as Prowlarr's, whose
// releases are sent to a download client without a library item. With a
// Mavio account it searches every server the user administers, or one
// chosen, each server's releases a group of their own pages.

// tools are the advanced tools: the sections of the settings' Advanced
// group, for administrators, which can be pinned to the sidebar's
// Features group to open in the app's own layout.
var tools = []settingsSection{
	{id: "indexer-search", icon: "indexers", color: ui.RGB(0x0A, 0x84, 0xFF), title: "indexers.title", tool: true},
}

// toolPath returns the page of a tool pinned to the sidebar.
func toolPath(id string) string { return "/tools/" + id }

// pinned reports whether a tool is pinned to the sidebar.
func (a *App) pinned(id string) bool { return slices.Contains(a.prefs.Pinned, id) }

// togglePin pins a tool to the sidebar, or unpins it.
func (a *App) togglePin(id string) {
	if a.pinned(id) {
		a.prefs.Pinned = slices.DeleteFunc(slices.Clone(a.prefs.Pinned), func(p string) bool { return p == id })
	} else {
		a.prefs.Pinned = append(slices.Clone(a.prefs.Pinned), id)
	}
	a.savePreferences()
}

// pinnedTools returns the tools pinned to the sidebar, in the order
// pinned; they are an administrator's.
func (a *App) pinnedTools() []settingsSection {
	if admins, _ := a.adminServers(); len(admins) == 0 {
		return nil
	}
	var out []settingsSection
	for _, id := range a.prefs.Pinned {
		if i := slices.IndexFunc(tools, func(s settingsSection) bool { return s.id == id }); i >= 0 {
			out = append(out, tools[i])
		}
	}
	return out
}

// adminServers returns the servers the user administers; known tells
// whether that is known of every server yet.
func (a *App) adminServers() (list []*capiv1.ServerRecord, known bool) {
	known = true
	for _, s := range a.servers {
		u := a.currentUser(s.GetId())
		if !u.loaded {
			known = false
		}
		if u.value.GetAdmin() {
			list = append(list, s)
		}
	}
	return list, known
}

// toolPage builds a tool pinned to the sidebar, in the app's own layout.
func (a *App) toolPage(c *ui.Context, r *ui.Route) {
	if r.Param("tool") != "indexer-search" {
		a.home(c)
		return
	}
	r.Title(style.T(c, "indexers.title"))
	ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Fill().Children(func() {
		top := max(c.TitleBar().Height, 12)
		ui.Column(c).Padding(top+28, style.Layout(c).Margin, 40).AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(1200).Gap(28).Children(func() { a.indexerSearch(c) })
		})
	})
}

// indexerSearchState is the indexer search's: the form, the search run
// and each server's releases of it.
type indexerSearchState struct {
	// server is the ID of the server to search, every one when empty.
	server string
	query  string
	// kind, category and indexer are the choices', indices of their
	// options.
	kind, category, indexer int
	search                  *releaseSearch
	// searches counts the searches run, each starting the servers again
	// at their first page.
	searches int
	sort     releaseSort
	results  map[string]*serverReleases
	indexers map[string]*task[*acquisitionv1.ListIndexersResponse]
}

// releaseSearch is a search of the indexers run.
type releaseSearch struct {
	query      string
	kind       acquisitionv1.ReleaseSearchType
	categories []int32
	indexers   []*acquisitionv1.IndexerRef
}

// releaseSort is the column releases are sorted by, none when key is
// empty.
type releaseSort struct {
	key  string
	desc bool
}

// serverReleases are a server's releases of a search, the page shown,
// and what was done with each, by its key.
type serverReleases struct {
	found task[*acquisitionv1.SearchReleasesResponse]
	page  int
	sends map[string]*releaseSend
}

// releaseSend is a release sent to a download client, and its magnet
// link copied.
type releaseSend struct {
	busy   bool
	client string
	err    error
	copied bool
}

// releasePageSize is how many releases a page shows.
const releasePageSize = 50

var releaseKinds = []struct {
	kind  acquisitionv1.ReleaseSearchType
	label string
}{
	{acquisitionv1.ReleaseSearchType_RELEASE_SEARCH_TYPE_UNSPECIFIED, "indexers.typeAny"},
	{acquisitionv1.ReleaseSearchType_RELEASE_SEARCH_TYPE_MOVIE, "indexers.typeMovie"},
	{acquisitionv1.ReleaseSearchType_RELEASE_SEARCH_TYPE_TV, "indexers.typeTv"},
}

type newznabCategory struct {
	id    int32
	label string
}

// newznabCategories are Newznab's top categories, by their first number.
var newznabCategories = []newznabCategory{
	{1000, "indexers.categoryConsole"},
	{2000, "indexers.categoryMovies"},
	{3000, "indexers.categoryAudio"},
	{4000, "indexers.categoryPc"},
	{5000, "indexers.categoryTv"},
	{6000, "indexers.categoryXxx"},
	{7000, "indexers.categoryBooks"},
	{8000, "indexers.categoryOther"},
}

// categoryNames returns the top categories of a release's categories, by
// name.
func categoryNames(c *ui.Context, list []int32) string {
	var names []string
	var seen []int32
	for _, id := range list {
		top := id / 1000 * 1000
		if slices.Contains(seen, top) {
			continue
		}
		seen = append(seen, top)
		if i := slices.IndexFunc(newznabCategories, func(x newznabCategory) bool { return x.id == top }); i >= 0 {
			names = append(names, style.T(c, newznabCategories[i].label))
		}
	}
	return strings.Join(names, ", ")
}

// releaseAge tells how long ago a release was published: in days from a
// day, as Prowlarr tells.
func releaseAge(c *ui.Context, r *acquisitionv1.Release) string {
	if !r.HasPublishDate() {
		return ""
	}
	d := time.Since(r.GetPublishDate().AsTime())
	if d >= 24*time.Hour {
		d = d.Truncate(24 * time.Hour)
	}
	return lengthOf(c, d)
}

func isTorrent(r *acquisitionv1.Release) bool {
	return r.GetProtocol() == acquisitionv1.DownloadProtocol_DOWNLOAD_PROTOCOL_TORRENT
}

// pick builds a drop-down of labels choosing *value, an index of them,
// and reports a new choice.
func pick(c *ui.Context, label string, value *int, labels []string) bool {
	chosen := labels[max(0, min(*value, len(labels)-1))]
	if selectOf(c, &chosen, labels).Label(label).Changed() {
		if i := slices.Index(labels, chosen); i >= 0 && i != *value {
			*value = i
			return true
		}
	}
	return false
}

// indexerSearch builds the indexer search: the servers to search, the
// search, and each server's releases.
func (a *App) indexerSearch(c *ui.Context) {
	st := &a.settingsState.indexers
	sectionHeader(c, style.T(c, "indexers.title"), style.T(c, "indexers.message"))
	admins, _ := a.adminServers()
	var one *capiv1.ServerRecord
	if a.account != nil {
		if i := slices.IndexFunc(admins, func(s *capiv1.ServerRecord) bool { return s.GetId() == st.server }); i >= 0 {
			one = admins[i]
		}
	} else if len(admins) > 0 {
		one = admins[0]
	}
	searched := admins
	if one != nil {
		searched = []*capiv1.ServerRecord{one}
	}
	var enabled []*acquisitionv1.Indexer
	if one != nil {
		for _, x := range a.searchIndexers(one.GetId()).value.GetIndexers() {
			if x.GetEnabled() {
				enabled = append(enabled, x)
			}
		}
	}
	ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
		label := style.T(c, "indexers.query")
		f := ui.SearchField(c, &st.query).Grow(1).MinWidth(224).Placeholder(label).Label(label)
		submit := f.Submitted()
		if a.account != nil && len(admins) > 0 {
			labels := []string{style.T(c, "indexers.allServers")}
			chosen := 0
			for i, s := range admins {
				labels = append(labels, s.GetName())
				if s.GetId() == st.server {
					chosen = i + 1
				}
			}
			if pick(c.Key("server"), style.T(c, "settings.serverChoice"), &chosen, labels) {
				st.server, st.indexer = "", 0
				if chosen > 0 {
					st.server = admins[chosen-1].GetId()
				}
			}
		}
		kinds := make([]string, len(releaseKinds))
		for i, k := range releaseKinds {
			kinds[i] = style.T(c, k.label)
		}
		pick(c.Key("type"), style.T(c, "indexers.type"), &st.kind, kinds)
		categories := []string{style.T(c, "indexers.allCategories")}
		for _, x := range newznabCategories {
			categories = append(categories, style.T(c, x.label))
		}
		pick(c.Key("category"), style.T(c, "indexers.category"), &st.category, categories)
		if one != nil {
			indexers := []string{style.T(c, "indexers.allIndexers")}
			for _, x := range enabled {
				indexers = append(indexers, x.GetName())
			}
			pick(c.Key("indexer-"+one.GetId()), style.T(c, "indexers.indexer"), &st.indexer, indexers)
		}
		query := strings.TrimSpace(st.query)
		if ui.PrimaryButton(c, style.T(c, "indexers.search")).Disabled(query == "").Clicked() {
			submit = true
		}
		if submit && query != "" {
			s := &releaseSearch{query: query, kind: releaseKinds[st.kind].kind}
			if st.category > 0 {
				s.categories = []int32{newznabCategories[st.category-1].id}
			}
			if one != nil && st.indexer > 0 && st.indexer <= len(enabled) {
				x := enabled[st.indexer-1]
				s.indexers = []*acquisitionv1.IndexerRef{acquisitionv1.IndexerRef_builder{PluginId: new(x.GetPluginId()), IndexerId: new(x.GetId())}.Build()}
			}
			st.search = s
			st.searches++
			st.results = map[string]*serverReleases{}
		}
	})
	if st.search == nil {
		secondaryText(c, style.T(c, "indexers.prompt"))
		return
	}
	for _, s := range searched {
		a.serverReleases(c.Key("releases-"+s.GetId()), s, one == nil)
	}
}

// searchIndexers loads a server's indexers, for the choice of one.
func (a *App) searchIndexers(server string) *task[*acquisitionv1.ListIndexersResponse] {
	st := &a.settingsState.indexers
	if st.indexers == nil {
		st.indexers = map[string]*task[*acquisitionv1.ListIndexersResponse]{}
	}
	t := st.indexers[server]
	if t == nil {
		t = &task[*acquisitionv1.ListIndexersResponse]{}
		st.indexers[server] = t
	}
	return load(a, t, key("search-indexers", server, a.generation), func(ctx context.Context) (*acquisitionv1.ListIndexersResponse, error) {
		return a.releaseService(server).ListIndexers(ctx, &acquisitionv1.ListIndexersRequest{})
	})
}

// releaseKey returns the key of a release among a search's.
func releaseKey(r *acquisitionv1.Release) string {
	return r.GetPluginId() + "/" + r.GetIndexerId() + "/" + r.GetGuid()
}

// serverReleases builds a server's releases of the search, page by page,
// named by the server when every server is searched.
func (a *App) serverReleases(c *ui.Context, s *capiv1.ServerRecord, named bool) {
	st := &a.settingsState.indexers
	colors := style.Colors(c)
	id := s.GetId()
	sr := st.results[id]
	if sr == nil {
		sr = &serverReleases{sends: map[string]*releaseSend{}}
		st.results[id] = sr
	}
	search := st.search
	found := load(a, &sr.found, key("indexer-search", id, a.generation, st.searches), func(ctx context.Context) (*acquisitionv1.SearchReleasesResponse, error) {
		return a.releaseService(id).SearchReleases(ctx, acquisitionv1.SearchReleasesRequest_builder{
			Query: &search.query, Type: &search.kind, Categories: search.categories, Indexers: search.indexers,
		}.Build())
	})
	all := found.value.GetReleases()
	list := sortedReleases(all, st.sort)
	pages := max(1, (len(list)+releasePageSize-1)/releasePageSize)
	sr.page = min(sr.page, pages-1)
	shown := list[sr.page*releasePageSize : min(len(list), (sr.page+1)*releasePageSize)]
	names := map[string]string{}
	for _, r := range all {
		names[r.GetPluginId()+"/"+r.GetIndexerId()] = r.GetIndexerName()
	}
	title := ""
	var count func()
	if named {
		title = s.GetName()
		if found.loaded && found.err == nil {
			count = func() {
				style.Text(ui.Text(c, style.T(c, "indexers.results", "count", len(all))), style.Type(c).Callout).TextColor(style.Color(colors.TextTertiary))
			}
		}
	}
	compact := a.contentWidth < 860
	ui.Column(c).Gap(8).Children(func() {
		groupWith(c, "releases", title, count, "", func(g *settingsGroup) {
			line := func(key, text string, danger bool) {
				ui.Box(c.Key(key)).Padding(10, 16).Children(func() { footnote(c, text, danger) })
			}
			for _, f := range found.value.GetFailures() {
				k := f.GetPluginId() + "/" + f.GetIndexerId()
				line("failure-"+k, style.T(c, "acquire.indexerFailed", "indexer", cmpOr(names[k], f.GetIndexerId(), f.GetPluginId()), "message", f.GetMessage()), true)
			}
			switch {
			case !found.loaded:
				line("status", style.T(c, "acquire.searchingIndexers"), false)
			case found.err != nil:
				line("status", errorMessage(c, found.err), true)
			case len(all) == 0:
				line("status", style.T(c, "acquire.noReleases"), false)
			}
			if len(shown) == 0 {
				return
			}
			if compact {
				for i, r := range shown {
					if i > 0 {
						ui.Box(c.Key(key("divider", i))).Height(1).Background(style.Color(colors.Border))
					}
					a.releaseItem(c.Key(releaseKey(r)), id, sr, r)
				}
				return
			}
			a.releaseTable(c, id, sr, shown)
		})
		if pages > 1 {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.End).Padding(0, 4).Children(func() {
				footnote(c, style.T(c, "indexers.page", "page", sr.page+1, "pages", pages), false)
				if pageButton(c, "back", style.T(c, "indexers.previous"), sr.page == 0) {
					sr.page--
				}
				if pageButton(c, "forward", style.T(c, "indexers.next"), sr.page+1 >= pages) {
					sr.page++
				}
			})
		}
	})
}

// sortedReleases returns releases sorted by a column, as they came when
// by none.
func sortedReleases(list []*acquisitionv1.Release, s releaseSort) []*acquisitionv1.Release {
	if s.key == "" {
		return list
	}
	out := slices.Clone(list)
	slices.SortStableFunc(out, func(x, y *acquisitionv1.Release) int {
		var n int
		switch s.key {
		case "age":
			n = -x.GetPublishDate().AsTime().Compare(y.GetPublishDate().AsTime())
		case "title":
			n = strings.Compare(strings.ToLower(x.GetTitle()), strings.ToLower(y.GetTitle()))
		case "indexer":
			n = strings.Compare(strings.ToLower(x.GetIndexerName()), strings.ToLower(y.GetIndexerName()))
		case "size":
			n = cmpInt(x.GetSize(), y.GetSize())
		case "grabs":
			n = cmpInt(x.GetGrabs(), y.GetGrabs())
		case "peers":
			n = cmpInt(x.GetSeeders(), y.GetSeeders())
		}
		if s.desc {
			return -n
		}
		return n
	})
	return out
}

func cmpInt[T int32 | int64](x, y T) int {
	switch {
	case x < y:
		return -1
	case x > y:
		return 1
	}
	return 0
}

// pageButton builds a button of an icon turning a page, and reports a
// click.
func pageButton(c *ui.Context, icon, label string, disabled bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(icon)).Size(28, 28).Radius(6).Center().Label(label).Tooltip(label).Disabled(disabled)
	fg := colors.TextPrimary
	if disabled {
		fg = colors.TextDisabled
	} else if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, false)).Size(14, 14).TextColor(style.Color(fg))
	})
	return b.Clicked() && !disabled
}

// releaseColumn is a column of the releases' table: sorted by its key,
// when it has one; as wide as the room left when its width is zero.
type releaseColumn struct {
	label, key string
	width      float32
	end        bool
}

var releaseColumns = []releaseColumn{
	{label: "indexers.protocol", width: 72},
	{label: "indexers.age", key: "age", width: 64},
	{label: "indexers.releaseTitle", key: "title"},
	{label: "indexers.indexer", key: "indexer", width: 96},
	{label: "indexers.size", key: "size", width: 72, end: true},
	{label: "indexers.grabs", key: "grabs", width: 52, end: true},
	{label: "indexers.peers", key: "peers", width: 64, end: true},
	{label: "indexers.category", width: 88},
	{width: 88},
}

// releaseCell builds a cell of a column in the releases' table.
func releaseCell(c *ui.Context, col releaseColumn, fn func()) {
	cell := ui.Row(c).AlignItems(ui.Center).Shrink(0)
	if col.width == 0 {
		cell.Grow(1).Shrink(1).MinWidth(160)
	} else {
		cell.Width(col.width)
	}
	if col.end {
		cell.Justify(ui.End)
	}
	cell.Children(fn)
}

// releaseTable builds a page of releases as a table, as Prowlarr's search
// lists them; a column's heading sorts by it.
func (a *App) releaseTable(c *ui.Context, server string, sr *serverReleases, releases []*acquisitionv1.Release) {
	st := &a.settingsState.indexers
	colors := style.Colors(c)
	ui.Row(c.Key("head")).Padding(8, 16).Gap(8).AlignItems(ui.Center).Children(func() {
		for i, col := range releaseColumns {
			releaseCell(c.Key(i), col, func() {
				if col.label == "" {
					return
				}
				label := style.T(c, col.label)
				if col.key == "" {
					style.Text(ui.Text(c, label), style.Type(c).Footnote).FontWeight(500).TextColor(style.Color(colors.TextSecondary)).SingleLine()
					return
				}
				b := ui.ButtonBase(c).Gap(4).AlignItems(ui.Center).Label(label)
				fg := colors.TextSecondary
				if b.Hovered() {
					fg = colors.TextPrimary
				}
				b.Children(func() {
					style.Text(ui.Text(c, label), style.Type(c).Footnote).FontWeight(500).TextColor(style.Color(fg)).SingleLine()
					if st.sort.key == col.key {
						icon := ui.Icon(c, style.Icon("expand", false)).Size(12, 12).TextColor(style.Color(fg))
						if !st.sort.desc {
							icon.Rotate(180)
						}
					}
				})
				if b.Clicked() {
					desc := col.key == "size" || col.key == "grabs" || col.key == "peers"
					if st.sort.key == col.key {
						desc = !st.sort.desc
					}
					st.sort = releaseSort{key: col.key, desc: desc}
				}
			})
		}
	})
	for _, r := range releases {
		ui.Box(c.Key("divider-" + releaseKey(r))).Height(1).Background(style.Color(colors.Border))
		a.releaseLine(c.Key(releaseKey(r)), server, sr, r)
	}
}

// releaseLine builds a release's row of the table.
func (a *App) releaseLine(c *ui.Context, server string, sr *serverReleases, r *acquisitionv1.Release) {
	colors := style.Colors(c)
	torrent := isTorrent(r)
	text := func(s string, fg design.Color) {
		style.Text(ui.Text(c, s), style.Type(c).Footnote).TextColor(style.Color(fg)).SingleLine()
	}
	ui.Row(c).Padding(8, 16).Gap(8).AlignItems(ui.Center).Children(func() {
		for i, col := range releaseColumns {
			releaseCell(c.Key(i), col, func() {
				switch i {
				case 0:
					protocolBadge(c, torrent)
				case 1:
					text(releaseAge(c, r), colors.TextSecondary)
				case 2:
					style.Text(ui.Text(c, r.GetTitle()), style.Type(c).Footnote).TextColor(style.Color(colors.TextPrimary)).Selectable().Shrink(1).MinWidth(0)
				case 3:
					text(r.GetIndexerName(), colors.TextSecondary)
				case 4:
					if r.GetSize() > 0 {
						text(sizeOf(r.GetSize()), colors.TextPrimary)
					}
				case 5:
					if r.GetGrabs() > 0 {
						text(strconv.Itoa(int(r.GetGrabs())), colors.TextSecondary)
					}
				case 6:
					if torrent {
						text(strconv.Itoa(int(r.GetSeeders()))+" / "+strconv.Itoa(int(r.GetPeers())), colors.TextPrimary)
					}
				case 7:
					text(categoryNames(c, r.GetCategories()), colors.TextSecondary)
				case 8:
					a.releaseActions(c, server, sr, r)
				}
			})
		}
	})
}

// protocolBadge builds the badge of a release's protocol.
func protocolBadge(c *ui.Context, torrent bool) {
	colors := style.Colors(c)
	bg, fg, label := colors.Fill, colors.TextSecondary, style.T(c, "acquire.usenet")
	if torrent {
		bg, fg, label = colors.FillSelected, colors.TextPrimary, style.T(c, "indexers.torrent")
	}
	ui.Box(c).Padding(2, 6).Radius(4).Background(style.Color(bg)).Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Caption).FontWeight(500).TextColor(style.Color(fg)).SingleLine()
	})
}

// releaseItem builds a release where the table has no room: its title,
// and what the table's columns would tell, in a line under it.
func (a *App) releaseItem(c *ui.Context, server string, sr *serverReleases, r *acquisitionv1.Release) {
	colors := style.Colors(c)
	protocol := style.T(c, "acquire.usenet")
	var peers string
	if isTorrent(r) {
		protocol = style.T(c, "indexers.torrent")
		peers = style.T(c, "acquire.peers", "seeders", r.GetSeeders(), "peers", r.GetPeers())
	}
	var size string
	if r.GetSize() > 0 {
		size = sizeOf(r.GetSize())
	}
	ui.Row(c).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			style.Text(ui.Text(c, r.GetTitle()), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
			footnote(c, joinDetail(protocol, releaseAge(c, r), r.GetIndexerName(), size, peers, categoryNames(c, r.GetCategories())), false)
		})
		a.releaseActions(c, server, sr, r)
	})
}

// releaseActions builds the release's page on the indexer, opened in the
// browser, copying its magnet link, and sending it to a download client.
func (a *App) releaseActions(c *ui.Context, server string, sr *serverReleases, r *acquisitionv1.Release) {
	colors := style.Colors(c)
	k := releaseKey(r)
	s := sr.sends[k]
	if s == nil {
		s = &releaseSend{}
		sr.sends[k] = s
	}
	ui.Row(c).Gap(2).AlignItems(ui.Center).Justify(ui.End).FillWidth().Children(func() {
		if u := r.GetInfoUrl(); u != "" && a.opts.OpenURL != nil {
			if releaseAction(c, "info", "external", style.T(c, "indexers.info"), colors.TextSecondary, false) {
				_ = a.opts.OpenURL(u)
			}
		}
		if m := r.GetMagnetUrl(); m != "" {
			label, icon, fg := style.T(c, "indexers.magnet"), "magnet", colors.TextSecondary
			if s.copied {
				label, icon, fg = style.T(c, "indexers.magnetCopied"), "check", colors.Success
			}
			if releaseAction(c, "magnet", icon, label, fg, false) {
				c.WriteClipboard(m)
				s.copied = true
			}
		}
		label, icon, fg := style.T(c, "indexers.send"), "downloads", colors.AccentText
		switch {
		case s.client != "":
			label, icon, fg = style.T(c, "indexers.sent", "client", s.client), "check", colors.Success
		case s.err != nil:
			label, icon, fg = errorMessage(c, s.err), "error", colors.Danger
		case s.busy:
			fg = colors.TextTertiary
		}
		if releaseAction(c, "send", icon, label, fg, s.busy || s.client != "") {
			s.busy, s.err = true, nil
			go func() {
				res, err := a.downloadService(server).GrabRelease(a.ctx, acquisitionv1.GrabReleaseRequest_builder{Release: r}.Build())
				a.opts.Update(func() {
					s.busy, s.err = false, err
					if err == nil {
						s.client = cmpOr(res.GetClientName(), "?")
					}
				})
			}()
		}
	})
}

// releaseAction builds a button of an icon acting on a release, its label
// its tooltip, and reports a click.
func releaseAction(c *ui.Context, key, icon, label string, fg design.Color, disabled bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(key)).Size(28, 28).Radius(6).Center().Label(label).Tooltip(label).Disabled(disabled)
	if b.Hovered() && !disabled {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, false)).Size(16, 16).TextColor(style.Color(fg))
	})
	return b.Clicked() && !disabled
}

// toolLink builds a tool's link in the settings' sidebar, with its pin,
// shown while pinned or pointed at.
func (a *App) toolLink(c *ui.Context, s settingsSection, selected bool) {
	colors := style.Colors(c)
	row := ui.Box(c.Key("tool-" + s.id))
	row.Children(func() {
		a.sidebarLink(c, s, selected)
		on := a.pinned(s.id)
		if !on && !row.Hovered() {
			return
		}
		label := style.T(c, "settings.pin")
		fg := colors.TextTertiary
		if on {
			label, fg = style.T(c, "settings.unpin"), colors.AccentText
		}
		ui.Row(c).Absolute().Top(0).Bottom(0).Right(8).AlignItems(ui.Center).Children(func() {
			b := ui.ButtonBase(c.Key("pin")).Size(24, 24).Radius(6).Center().Label(label).Tooltip(label).Selected(on)
			if b.Hovered() && !on {
				fg = colors.TextPrimary
			}
			b.Children(func() {
				ui.Icon(c, style.Icon("pin", on)).Size(16, 16).TextColor(style.Color(fg))
			})
			if b.Clicked() {
				a.togglePin(s.id)
			}
		})
	})
}

// toolTile builds a tool's icon on its colored tile, size DIPs wide, in
// the app's sidebar.
func toolTile(c *ui.Context, icon string, color ui.Color, size float32) {
	ui.Box(c).Size(size, size).Shrink(0).Radius(size / 4).Background(color).Center().Children(func() {
		ui.Icon(c, style.Icon(icon, true)).Size(size*0.62, size*0.62).TextColor(ui.RGB(255, 255, 255))
	})
}

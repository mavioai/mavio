package app

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// discoverState is the state of Discover, its catalogs, titles and
// requests.
type discoverState struct {
	reload   int // counts the requests made, approved and changed
	catalogs map[string]*task[[]*acquisitionv1.Catalog]
	shelves  map[string]*task[*acquisitionv1.DiscoverResponse]
	grids    map[string]*titleGrid
	titles   map[string]*task[*acquisitionv1.Title]
	seasons  map[string]*task[[]*acquisitionv1.SeasonEpisode]
	open     map[string]bool // the seasons expanded, by title and number
	requests map[string]*requestList
	quotas   map[string]*task[*acquisitionv1.GetQuotaResponse]
	users    map[string]*task[[]*userv1.User]

	busy     bool  // a request is being made or changed
	err      error // of the last request made or changed
	dialog   bool  // the seasons' dialog is open
	asking   *acquisitionv1.Title
	editing  *acquisitionv1.Request // the request whose seasons the dialog changes
	chosen   map[int32]bool
	removing string // the request whose removal is being confirmed

	filters filtersState
}

// titleGrid is a catalog's titles loaded page by page.
type titleGrid struct {
	key     string
	titles  []*acquisitionv1.DiscoveredTitle
	seen    map[string]bool
	page    int32 // the last page loaded
	more    bool
	loading bool
	err     error
	state   ui.GridState
}

// requestList is the requests of a server of a status, loaded as far as
// limit.
type requestList struct {
	filter int
	limit  int32
	list   task[*acquisitionv1.ListRequestsResponse]
}

func (a *App) discoverService(id string) acquisitionv1connect.DiscoverServiceClient {
	return acquisitionv1connect.NewDiscoverServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) requestService(id string) acquisitionv1connect.RequestServiceClient {
	return acquisitionv1connect.NewRequestServiceClient(a.connectHTTP(), serverURL(id))
}

// discoverLanguage is the language titles are discovered in: the app's,
// without its region.
func discoverLanguage(c *ui.Context) string {
	l, _, _ := strings.Cut(c.Locale(), "-")
	return l
}

// tmdbImage matches the original size of a TMDB image.
var tmdbImage = regexp.MustCompile(`(image\.tmdb\.org/t/p/)original/`)

// sized returns the same TMDB image at a width, rather than its original
// size.
func sized(u string, width int) string {
	return tmdbImage.ReplaceAllString(u, fmt.Sprintf("${1}w%d/", width))
}

// kindParam is a title's kind in addresses.
func kindParam(k acquisitionv1.MediaKind) string {
	if k == acquisitionv1.MediaKind_MEDIA_KIND_SERIES {
		return "series"
	}
	return "movie"
}

func mediaKindOf(p string) acquisitionv1.MediaKind {
	switch p {
	case "series":
		return acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	case "movie":
		return acquisitionv1.MediaKind_MEDIA_KIND_MOVIE
	}
	return acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED
}

// titlePath is the address of a discovered title.
func titlePath(server string, kind acquisitionv1.MediaKind, tmdb int32) string {
	return "/title/" + server + "/" + kindParam(kind) + "/" + fmt.Sprint(tmdb)
}

func tmdbOf(t *acquisitionv1.DiscoveredTitle) int32 {
	n, _ := strconv.Atoi(t.GetExternalIds()["tmdb"])
	return int32(n)
}

// discovery returns the server Discover asks: the first signed-in server
// whose discovery providers offer catalogs, with its catalogs; pending
// while servers have not answered.
func (a *App) discovery(c *ui.Context) (server string, catalogs []*acquisitionv1.Catalog, pending bool) {
	st := &a.discoverState
	if st.catalogs == nil {
		st.catalogs = map[string]*task[[]*acquisitionv1.Catalog]{}
	}
	language := discoverLanguage(c)
	for _, s := range a.servers {
		id := s.GetId()
		t := st.catalogs[id]
		if t == nil {
			t = &task[[]*acquisitionv1.Catalog]{}
			st.catalogs[id] = t
		}
		load(a, t, key("catalogs", id, language, a.generation), func(ctx context.Context) ([]*acquisitionv1.Catalog, error) {
			res, err := a.discoverService(id).ListCatalogs(ctx, acquisitionv1.ListCatalogsRequest_builder{Language: &language}.Build())
			return res.GetCatalogs(), err
		})
		if !t.loaded {
			return "", nil, true
		}
		if len(t.value) > 0 {
			return id, t.value, false
		}
	}
	return "", nil, false
}

// catalogRow is a row of Discover: a catalog, of one kind of title unless
// it mixes them.
type catalogRow struct {
	catalog *acquisitionv1.Catalog
	kind    acquisitionv1.MediaKind
}

// rowKinds are the catalogs Discover shows, in its order; those that need
// a title to start from are left out.
var rowKinds = []pluginv1.CatalogKind{
	pluginv1.CatalogKind_CATALOG_KIND_TRENDING, pluginv1.CatalogKind_CATALOG_KIND_POPULAR, pluginv1.CatalogKind_CATALOG_KIND_UPCOMING,
	pluginv1.CatalogKind_CATALOG_KIND_TOP_RATED, pluginv1.CatalogKind_CATALOG_KIND_LIST,
}

// rowsOf returns the catalogs' rows: one, or one per kind of title it
// lists apart.
func rowsOf(catalogs []*acquisitionv1.Catalog) []catalogRow {
	var rows []catalogRow
	for _, kind := range rowKinds {
		for _, cat := range catalogs {
			if cat.GetKind() != kind {
				continue
			}
			kinds := cat.GetMediaKinds()
			switch {
			case cat.GetMixed():
				rows = append(rows, catalogRow{cat, acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED})
			case len(kinds) < 2:
				k := acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED
				if len(kinds) == 1 {
					k = kinds[0]
				}
				rows = append(rows, catalogRow{cat, k})
			default:
				for _, k := range []acquisitionv1.MediaKind{acquisitionv1.MediaKind_MEDIA_KIND_MOVIE, acquisitionv1.MediaKind_MEDIA_KIND_SERIES} {
					if slices.Contains(kinds, k) {
						rows = append(rows, catalogRow{cat, k})
					}
				}
			}
		}
	}
	return rows
}

// catalogRowTitle names a row of Discover by its catalog's kind and its
// kind of title.
func catalogRowTitle(c *ui.Context, r catalogRow) string {
	series := r.kind == acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	pick := func(movies, shows string) string {
		if series {
			return style.T(c, shows)
		}
		return style.T(c, movies)
	}
	switch r.catalog.GetKind() {
	case pluginv1.CatalogKind_CATALOG_KIND_TRENDING:
		if r.kind == acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED {
			return style.T(c, "discover.trending")
		}
	case pluginv1.CatalogKind_CATALOG_KIND_POPULAR:
		return pick("discover.popularMovies", "discover.popularSeries")
	case pluginv1.CatalogKind_CATALOG_KIND_UPCOMING:
		return pick("discover.upcomingMovies", "discover.upcomingSeries")
	case pluginv1.CatalogKind_CATALOG_KIND_TOP_RATED:
		return pick("discover.topRatedMovies", "discover.topRatedSeries")
	}
	return r.catalog.GetName()
}

// statusLabel returns the status a title's card and header show, if any.
func statusLabel(c *ui.Context, s acquisitionv1.MediaStatus) string {
	switch s {
	case acquisitionv1.MediaStatus_MEDIA_STATUS_AVAILABLE:
		return style.T(c, "discover.available")
	case acquisitionv1.MediaStatus_MEDIA_STATUS_PARTIALLY_AVAILABLE:
		return style.T(c, "discover.partiallyAvailable")
	case acquisitionv1.MediaStatus_MEDIA_STATUS_REQUESTED:
		return style.T(c, "discover.requested")
	case acquisitionv1.MediaStatus_MEDIA_STATUS_WANTED:
		return style.T(c, "discover.processing")
	case acquisitionv1.MediaStatus_MEDIA_STATUS_BLOCKLISTED:
		return style.T(c, "discover.blocklisted")
	}
	return ""
}

// discoverPage builds Discover: a row of each catalog of the first server
// that discovers.
func (a *App) discoverPage(c *ui.Context, r *ui.Route) {
	r.Title(style.T(c, "nav.discover"))
	server, catalogs, pending := a.discovery(c)
	ui.Column(c).Fill().Children(func() {
		pageHeader(c, style.T(c, "nav.discover"), func() {
			if server != "" && secondaryButton(c, "pending", style.T(c, "requests.title")).Clicked() {
				a.router.Push("/requests/" + server)
			}
		})
		switch {
		case pending:
			ui.Column(c).Gap(32).Children(func() { placeholderShelf(c, 8); placeholderShelf(c, 8) })
			return
		case server == "":
			statusView(c, "discover", style.T(c, "discover.notSetUp"), style.T(c, "discover.notSetUpMessage"), nil)
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Gap(32).Padding(0, 0, 40).Children(func() {
				for _, row := range rowsOf(catalogs) {
					a.catalogShelf(c, server, row, catalogRowTitle(c, row), 0)
				}
			})
		})
	})
}

// catalogShelf builds a row of a catalog's first page, with its titles
// all opening from its title; tmdb names the title of recommendations and
// similar titles.
func (a *App) catalogShelf(c *ui.Context, server string, row catalogRow, title string, tmdb int32) {
	st := &a.discoverState
	if st.shelves == nil {
		st.shelves = map[string]*task[*acquisitionv1.DiscoverResponse]{}
	}
	id, kind, language := row.catalog.GetId(), row.kind, discoverLanguage(c)
	k := key("shelf", server, id, kind, tmdb, language, a.generation, st.reload)
	t := st.shelves[key(server, id, kind, tmdb)]
	if t == nil {
		t = &task[*acquisitionv1.DiscoverResponse]{}
		st.shelves[key(server, id, kind, tmdb)] = t
	}
	load(a, t, k, func(ctx context.Context) (*acquisitionv1.DiscoverResponse, error) {
		return a.discoverService(server).Discover(ctx, acquisitionv1.DiscoverRequest_builder{
			Catalog: &id, MediaKind: &kind, TmdbId: &tmdb, Language: &language, Page: new(int32(1)),
		}.Build())
	})
	titles := t.value.GetTitles()
	if t.err != nil || (t.loaded && len(titles) == 0) {
		return
	}
	if !t.loaded {
		placeholderShelf(c.Key(k), 8)
		return
	}
	var more func()
	if t.value.GetPage() < t.value.GetTotalPages() {
		more = func() {
			q := url.Values{"title": {title}}
			if kind != acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED {
				q.Set("kind", kindParam(kind))
			}
			if tmdb != 0 {
				q.Set("tmdb", fmt.Sprint(tmdb))
			}
			a.router.Push("/catalog/" + server + "/" + id + "?" + q.Encode())
		}
	}
	a.shelf(c, key(server, id, kind, tmdb), title, more, func() {
		for _, title := range titles {
			a.discoveredCard(c, server, title, style.Layout.Poster, "")
		}
	})
}

// discoveredCard builds the card of a discovered title width wide, which
// opens it; under its year and kind, a person's role in it.
func (a *App) discoveredCard(c *ui.Context, server string, t *acquisitionv1.DiscoveredTitle, width float32, role string) {
	colors := style.Colors(c)
	tmdb := tmdbOf(t)
	b := ui.ButtonBase(c.Key(key(t.GetMediaKind(), tmdb, t.GetName()))).Column().Width(width).Gap(6).AlignItems(ui.Start).Label(t.GetName())
	hovered := b.Hovered()
	if b.Pressed() {
		b.Opacity(0.8)
	}
	kind := style.T(c, "discover.movie")
	if t.GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES {
		kind = style.T(c, "discover.series")
	}
	var facts []string
	if t.GetYear() > 0 {
		facts = append(facts, fmt.Sprint(t.GetYear()))
	}
	facts = append(facts, kind)
	b.Children(func() {
		ui.Box(c).Children(func() {
			a.artwork(c, artworkOf(sized(t.GetPosterUrl(), 342)), t.GetName(), nil, width, width*1.5, hovered)
			statusMark(c, t.GetStatus())
		})
		style.Text(ui.Text(c, t.GetName()), style.Type.Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(width)
		style.Text(ui.Text(c, strings.Join(facts, " · ")), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine().Width(width)
		if role != "" {
			style.Text(ui.Text(c, role), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine().Width(width)
		}
	})
	if b.Clicked() && tmdb > 0 {
		a.router.Push(titlePath(server, t.GetMediaKind(), tmdb))
	}
}

// statusMark builds the mark of a card's status: a check once available,
// a clock while requested or processing.
func statusMark(c *ui.Context, s acquisitionv1.MediaStatus) {
	colors := style.Colors(c)
	available := s == acquisitionv1.MediaStatus_MEDIA_STATUS_AVAILABLE || s == acquisitionv1.MediaStatus_MEDIA_STATUS_PARTIALLY_AVAILABLE
	waiting := s == acquisitionv1.MediaStatus_MEDIA_STATUS_REQUESTED || s == acquisitionv1.MediaStatus_MEDIA_STATUS_WANTED
	if !available && !waiting {
		return
	}
	icon, bg := "check", style.Color(colors.Accent)
	if waiting {
		icon, bg = "pending", style.Color(colors.Scrim)
	}
	m := ui.Box(c).Absolute().Top(6).Right(6).Size(22, 22).Radius(11).Background(bg).Center().Tooltip(statusLabel(c, s))
	if s == acquisitionv1.MediaStatus_MEDIA_STATUS_PARTIALLY_AVAILABLE {
		m.Opacity(0.7)
	}
	m.Children(func() {
		ui.Icon(c, style.Icon(icon, false)).Size(16, 16).TextColor(ui.RGB(255, 255, 255))
	})
}

// catalogPage builds all the titles of a catalog, page by page.
func (a *App) catalogPage(c *ui.Context, r *ui.Route) {
	server, id, title := r.Param("server"), r.Param("catalog"), r.Query("title")
	kind := mediaKindOf(r.Query("kind"))
	tmdb64, _ := strconv.ParseInt(r.Query("tmdb"), 10, 32)
	tmdb := int32(tmdb64)
	filter := parseFilter(r.Query("f"))
	language := discoverLanguage(c)
	r.Title(title)
	st := &a.discoverState
	if st.grids == nil {
		st.grids = map[string]*titleGrid{}
	}
	gk := key(server, id, kind, tmdb, filter.param())
	g := st.grids[gk]
	if g == nil {
		g = &titleGrid{}
		st.grids[gk] = g
	}
	k := key(gk, language, a.generation, st.reload)
	if g.key != k {
		*g = titleGrid{key: k, more: true, seen: map[string]bool{}, state: g.state}
	}
	next := func() {
		if g.loading || !g.more || g.err != nil {
			return
		}
		g.loading = true
		page := g.page + 1
		go func() {
			b := acquisitionv1.DiscoverRequest_builder{Catalog: &id, MediaKind: &kind, TmdbId: &tmdb, Language: &language, Page: &page}
			filter.request(&b)
			res, err := a.discoverService(server).Discover(a.ctx, b.Build())
			a.opts.Update(func() {
				if g.key != k {
					return
				}
				g.loading, g.err = false, err
				if err != nil {
					return
				}
				g.page = page
				for _, t := range res.GetTitles() {
					tk := key(t.GetMediaKind(), t.GetExternalIds()["tmdb"], t.GetName())
					if !g.seen[tk] {
						g.seen[tk] = true
						g.titles = append(g.titles, t)
					}
				}
				g.more = res.GetPage() < res.GetTotalPages() && len(res.GetTitles()) > 0
			})
		}()
	}
	if len(g.titles) == 0 {
		next()
	}
	colors := style.Colors(c)
	_, catalogs, _ := a.discovery(c)
	var found *acquisitionv1.Catalog
	for _, cat := range catalogs {
		if cat.GetId() == id {
			found = cat
		}
	}
	// Recommendations and similar titles take no filters.
	filterable := found != nil && tmdb == 0 && (len(found.GetFilters()) > 0 || len(found.GetSorts()) > 0)
	filtered := filter.count() > 0 || filter.Sort != pluginv1.DiscoverSort_DISCOVER_SORT_UNSPECIFIED
	ui.Column(c).Fill().Children(func() {
		var controls func()
		if filterable {
			controls = func() {
				label := style.T(c, "discover.filters")
				if n := filter.count(); n > 0 {
					label = fmt.Sprintf("%s (%d)", label, n)
				}
				var b ui.Element
				if filtered {
					b = ui.PrimaryButton(c, label)
				} else {
					b = secondaryButton(c, "filter", label)
				}
				if b.Clicked() {
					a.openFilters(filter)
				}
			}
		}
		pageHeader(c, title, controls)
		switch {
		case len(g.titles) == 0 && g.err != nil:
			statusView(c, "error", style.T(c, "state.error"), errorMessage(c, g.err), func() { g.err = nil; next() })
			return
		case len(g.titles) == 0 && !g.more && filtered:
			statusView(c, "search", style.T(c, "discover.noMatches"), "", nil)
			return
		case len(g.titles) == 0 && !g.more:
			statusView(c, "", style.T(c, "state.empty"), "", nil)
			return
		case len(g.titles) == 0:
			ui.Row(c).Padding(0, style.Layout.Margin).Gap(style.Layout.Gutter).Wrap().ClipY().Grow(1).Children(func() {
				for range 12 {
					ui.Box(c).Size(style.Layout.Poster, style.Layout.Poster*1.5).Radius(design.RadiusMd).Background(style.Color(colors.SurfaceRaised))
				}
			})
			return
		}
		width, _ := a.gridColumns(c)
		ui.GridView(c, &g.state, len(g.titles), width-1, width*1.5+58, func(i int) {
			if i >= len(g.titles)-10 {
				next()
			}
			a.discoveredCard(c, server, g.titles[i], width, "")
		}).Grow(1).Padding(0, style.Layout.Margin, 24).Gap(style.Layout.Gutter)
	})
	if found != nil {
		a.filtersDialog(c, server, found, kind, func(f catalogFilter) {
			a.router.Replace(catalogLocation(server, id, title, kind, tmdb, f))
		})
	}
}

// discoveredTitle loads a title of a server, with what the server has of
// it and of its seasons.
func (a *App) discoveredTitle(c *ui.Context, server string, kind acquisitionv1.MediaKind, tmdb int32) *task[*acquisitionv1.Title] {
	st := &a.discoverState
	if st.titles == nil {
		st.titles = map[string]*task[*acquisitionv1.Title]{}
	}
	t := st.titles[key(server, kind, tmdb)]
	if t == nil {
		t = &task[*acquisitionv1.Title]{}
		st.titles[key(server, kind, tmdb)] = t
	}
	language := discoverLanguage(c)
	return load(a, t, key("title", server, kind, tmdb, language, a.generation, st.reload), func(ctx context.Context) (*acquisitionv1.Title, error) {
		res, err := a.discoverService(server).GetTitle(ctx, acquisitionv1.GetTitleRequest_builder{MediaKind: &kind, TmdbId: &tmdb, Language: &language}.Build())
		return res.GetTitle(), err
	})
}

// isAdmin reports whether the user administers a server.
func (a *App) isAdmin(server string) bool {
	return a.currentUser(server).value.GetAdmin()
}

// titlePage builds a discovered title: to watch when a library has it and
// to request when none does.
func (a *App) titlePage(c *ui.Context, r *ui.Route) {
	server, kind := r.Param("server"), mediaKindOf(r.Param("kind"))
	tmdb64, _ := strconv.ParseInt(r.Param("tmdb"), 10, 32)
	tmdb := int32(tmdb64)
	t := a.discoveredTitle(c, server, kind, tmdb)
	if taskStatus(c, t, func() { a.discoverState.titles[key(server, kind, tmdb)].key = "" }, nil) {
		return
	}
	title := t.value
	if title == nil {
		statusView(c, "error", style.T(c, "state.notFound"), style.T(c, "state.notFoundMessage"), nil)
		return
	}
	r.Title(title.GetName())
	_, catalogs, _ := a.discovery(c)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Gap(32).Padding(0, 0, 48).Children(func() {
			a.titleHeader(c, server, title)
			a.titleOverview(c, server, title)
			if title.GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES && len(title.GetSeasons()) > 0 {
				a.titleSeasons(c, server, title)
			}
			a.titleCast(c, server, title)
			for _, cat := range catalogs {
				k := cat.GetKind()
				if (k != pluginv1.CatalogKind_CATALOG_KIND_RECOMMENDATIONS && k != pluginv1.CatalogKind_CATALOG_KIND_SIMILAR) || !slices.Contains(cat.GetMediaKinds(), title.GetMediaKind()) {
					continue
				}
				name := style.T(c, "discover.similar")
				if k == pluginv1.CatalogKind_CATALOG_KIND_RECOMMENDATIONS {
					name = style.T(c, "discover.recommendations")
				}
				a.catalogShelf(c, server, catalogRow{cat, title.GetMediaKind()}, name, title.GetTmdbId())
			}
		})
	})
	a.requestDialog(c, server)
}

// titleFacts returns the line of a title's facts, as Seerr's: the year,
// then its kind, rating, runtime and genres.
func titleFacts(c *ui.Context, t *acquisitionv1.Title) string {
	var facts []string
	if t.GetYear() > 0 {
		facts = append(facts, fmt.Sprint(t.GetYear()))
	}
	if t.GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES {
		facts = append(facts, style.T(c, "discover.series"))
	} else {
		facts = append(facts, style.T(c, "discover.movie"))
	}
	if r := t.GetOfficialRating(); r != "" {
		facts = append(facts, r)
	}
	if m := t.GetRuntimeMinutes(); m > 0 {
		facts = append(facts, style.T(c, "time.minutes", "count", int(m)))
	}
	genres := t.GetGenres()
	facts = append(facts, genres[:min(len(genres), 3)]...)
	return strings.Join(facts, " · ")
}

func (a *App) titleHeader(c *ui.Context, server string, title *acquisitionv1.Title) {
	st := &a.discoverState
	_, wh := c.Size()
	height := min(max(wh*0.7, 520), 700)
	white := ui.RGB(255, 255, 255)
	backdrop, blurred := sized(title.GetBackdropUrl(), 1280), false
	if backdrop == "" {
		backdrop, blurred = sized(title.GetPosterUrl(), 342), true
	}
	series := title.GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	admin := a.isAdmin(server)
	var pending []string
	for _, r := range title.GetRequests() {
		if r.GetStatus() == acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING {
			pending = append(pending, r.GetId())
		}
	}
	some := slices.ContainsFunc(title.GetSeasons(), func(s *acquisitionv1.SeasonStatus) bool {
		return s.GetStatus() != acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED && s.GetStatus() != acquisitionv1.MediaStatus_MEDIA_STATUS_DELETED
	})
	ui.Box(c).Margin(12).Height(height).Radius(18).Clip().Background(style.Color(design.DarkColors.SurfaceRaised)).DragWindow().Children(func() {
		if bitmap := a.opts.Images.Get(backdrop, a.contentWidth, scale); bitmap != nil {
			img := ui.Image(c, bitmap).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover)
			if blurred {
				img.Blur(40)
			}
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 55,
			Stops: []ui.GradientStop{{At: 0, Color: ui.RGBA(0, 0, 0, 0.9)}, {At: 0.5, Color: ui.RGBA(0, 0, 0, 0.45)}, {At: 1, Color: ui.RGBA(0, 0, 0, 0)}},
		})
		ui.Row(c).Absolute().Left(40).Right(40).Bottom(40).Gap(40).AlignItems(ui.End).Children(func() {
			if poster := a.opts.Images.Get(sized(title.GetPosterUrl(), 500), 200, scale); poster != nil {
				ui.Box(c).Width(200).Height(300).Shrink(0).Radius(8).Clip().Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.4)).Children(func() {
					ui.Image(c, poster).Fill().Fit(ui.Cover)
				})
			}
			ui.Column(c).MaxWidth(640).MinWidth(0).Gap(12).Children(func() {
				style.Text(ui.Text(c, title.GetName()), style.Type.Display).TextColor(white).MaxLines(3).Selectable()
				ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
					style.Text(ui.Text(c, titleFacts(c, title)), style.Type.Callout).TextColor(white.Alpha(0.8))
					if s := statusLabel(c, title.GetStatus()); s != "" {
						style.Text(ui.Text(c, s), style.Type.Caption).FontWeight(600).TextColor(white).Padding(1, 6).Radius(4).Background(white.Alpha(0.2))
					}
				})
				ui.Row(c).Gap(12).Wrap().Padding(8, 0, 0).Children(func() {
					primary := true
					if title.GetItemId() != "" {
						if heroButton(c, "play", style.T(c, "discover.watch"), true).Clicked() {
							a.router.Push("/item/" + server + "/" + title.GetItemId())
						}
						primary = false
					}
					if urls := title.GetTrailerUrls(); len(urls) > 0 && a.opts.OpenURL != nil {
						if heroButton(c, "movies", style.T(c, "item.trailer"), false).Clicked() {
							_ = a.opts.OpenURL(urls[0])
						}
					}
					if title.GetRequestable() {
						label := style.T(c, "discover.request")
						if series && some {
							label = style.T(c, "discover.requestMore")
						}
						b := heroButton(c, "add", label, primary)
						if st.busy {
							b.Opacity(0.5)
						}
						if b.Clicked() && !st.busy {
							if series {
								a.askSeasons(title, nil)
							} else {
								a.createRequest(server, title, nil)
							}
						}
					}
					if admin && len(pending) > 0 {
						if heroButton(c, "check", style.T(c, "requests.approve"), false).Clicked() && !st.busy {
							a.requestAction(server, pending, "approve")
						}
						if heroButton(c, "close", style.T(c, "requests.decline"), false).Clicked() && !st.busy {
							a.requestAction(server, pending, "decline")
						}
					}
				})
				if st.err != nil {
					style.Text(ui.Text(c, style.T(c, "discover.requestFailed", "reason", errorMessage(c, st.err))), style.Type.Callout).TextColor(white).Selectable()
				}
			})
		})
	})
}

// askSeasons opens the dialog choosing the seasons of a series to
// request, or, given a pending request, to change its seasons.
func (a *App) askSeasons(title *acquisitionv1.Title, editing *acquisitionv1.Request) {
	st := &a.discoverState
	st.asking, st.editing, st.dialog, st.chosen = title, editing, true, map[int32]bool{}
	if editing != nil {
		for _, s := range editing.GetSeasons() {
			st.chosen[s.GetSeasonNumber()] = true
		}
		return
	}
	for _, s := range title.GetSeasons() {
		if s.GetRequestable() {
			st.chosen[s.GetSeasonNumber()] = true
		}
	}
}

// createRequest requests a movie, or seasons of a series, all those not
// requested yet when seasons is nil.
func (a *App) createRequest(server string, title *acquisitionv1.Title, seasons []int32) {
	st := &a.discoverState
	st.busy, st.err = true, nil
	kind, tmdb := title.GetMediaKind(), title.GetTmdbId()
	all := kind == acquisitionv1.MediaKind_MEDIA_KIND_SERIES && seasons == nil
	go func() {
		_, err := a.requestService(server).CreateRequest(a.ctx, acquisitionv1.CreateRequestRequest_builder{
			MediaKind: &kind, TmdbId: &tmdb, Seasons: seasons, AllSeasons: &all,
		}.Build())
		a.opts.Update(func() {
			st.busy, st.err, st.dialog = false, err, false
			st.reload++
		})
	}()
}

// requestAction approves, declines, retries or removes requests.
func (a *App) requestAction(server string, ids []string, action string) {
	st := &a.discoverState
	st.busy, st.err = true, nil
	go func() {
		var err error
		svc := a.requestService(server)
		for _, id := range ids {
			switch action {
			case "approve":
				_, err = svc.ApproveRequest(a.ctx, acquisitionv1.ApproveRequestRequest_builder{Id: &id}.Build())
			case "decline":
				_, err = svc.DeclineRequest(a.ctx, acquisitionv1.DeclineRequestRequest_builder{Id: &id}.Build())
			case "retry":
				_, err = svc.RetryRequest(a.ctx, acquisitionv1.RetryRequestRequest_builder{Id: &id}.Build())
			case "remove":
				_, err = svc.DeleteRequest(a.ctx, acquisitionv1.DeleteRequestRequest_builder{Id: &id}.Build())
			}
			if err != nil {
				break
			}
		}
		a.opts.Update(func() {
			st.busy, st.err, st.removing = false, err, ""
			st.reload++
		})
	}()
}

// updateRequest changes the seasons of a pending request.
func (a *App) updateRequest(server, id string, seasons []int32) {
	st := &a.discoverState
	st.busy, st.err = true, nil
	go func() {
		_, err := a.requestService(server).UpdateRequest(a.ctx, acquisitionv1.UpdateRequestRequest_builder{Id: &id, Seasons: seasons}.Build())
		a.opts.Update(func() {
			st.busy, st.err, st.dialog = false, err, false
			st.reload++
		})
	}()
}

func seasonName(c *ui.Context, n int32) string {
	if n == 0 {
		return style.T(c, "discover.specials")
	}
	return style.T(c, "discover.season", "number", n)
}

// requestDialog builds the dialog choosing the seasons of a series to
// request: all those not requested yet, or some; or changing a pending
// request's seasons.
func (a *App) requestDialog(c *ui.Context, server string) {
	st := &a.discoverState
	if !st.dialog || st.asking == nil {
		return
	}
	colors := style.Colors(c)
	title, editing := st.asking, st.editing
	choosable := func(s *acquisitionv1.SeasonStatus) bool {
		return s.GetRequestable() || (editing != nil && slices.ContainsFunc(editing.GetSeasons(), func(r *acquisitionv1.SeasonRequest) bool {
			return r.GetSeasonNumber() == s.GetSeasonNumber()
		}))
	}
	var open []int32
	for _, s := range title.GetSeasons() {
		if choosable(s) {
			open = append(open, s.GetSeasonNumber())
		}
	}
	var chosen []int32
	for _, n := range open {
		if st.chosen[n] {
			chosen = append(chosen, n)
		}
	}
	all := len(chosen) == len(open)
	ui.Modal(c, &st.dialog, func() {
		ui.Column(c).Width(440).MaxHeight(600).Gap(12).Children(func() {
			heading := style.T(c, "discover.requestTitle", "name", title.GetName())
			if editing != nil {
				heading = style.T(c, "requests.editTitle", "name", title.GetName())
			}
			style.Text(ui.Text(c, heading), style.Type.Title3).TextColor(style.Color(colors.TextPrimary))
			ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					on := all
					if ui.Checkbox(c, &on, style.T(c, "discover.allSeasons")).MinHeight(40).Changed() {
						for _, n := range open {
							st.chosen[n] = on
						}
					}
					for _, s := range title.GetSeasons() {
						n := s.GetSeasonNumber()
						on := st.chosen[n]
						ui.Row(c.Key(n)).MinHeight(40).Gap(12).AlignItems(ui.Center).Children(func() {
							cb := ui.Checkbox(c, &on, seasonName(c, n)).Grow(1).Disabled(!choosable(s))
							if cb.Changed() {
								st.chosen[n] = on
							}
							if s.GetEpisodeCount() > 0 {
								style.Text(ui.Text(c, style.T(c, "item.episodeCount", "count", int(s.GetEpisodeCount()))), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
							}
							if l := statusLabel(c, s.GetStatus()); l != "" {
								style.Text(ui.Text(c, l), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary))
							}
						})
					}
				})
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					st.dialog = false
				}
				label := style.T(c, "discover.requestSeasons", "count", len(chosen))
				if editing != nil {
					label = style.T(c, "action.save")
				}
				if ui.PrimaryButton(c, label).Disabled(st.busy || len(chosen) == 0).Clicked() {
					switch {
					case editing != nil:
						a.updateRequest(server, editing.GetId(), chosen)
					case all:
						a.createRequest(server, title, nil)
					default:
						a.createRequest(server, title, chosen)
					}
				}
			})
		})
	})
}

// titleSeasons builds a series' seasons with what the server has of each,
// as Seerr lists them: the latest first, specials last, each expanding to
// its episodes.
func (a *App) titleSeasons(c *ui.Context, server string, title *acquisitionv1.Title) {
	colors := style.Colors(c)
	st := &a.discoverState
	if st.open == nil {
		st.open = map[string]bool{}
	}
	seasons := slices.Clone(title.GetSeasons())
	slices.SortFunc(seasons, func(x, y *acquisitionv1.SeasonStatus) int {
		xs, ys := x.GetSeasonNumber() == 0, y.GetSeasonNumber() == 0
		if xs != ys {
			if xs {
				return 1
			}
			return -1
		}
		return int(y.GetSeasonNumber() - x.GetSeasonNumber())
	})
	ui.Column(c).Padding(0, style.Layout.Margin).Gap(8).Children(func() {
		style.Text(ui.Text(c, style.T(c, "item.seasons")), style.Type.Title3).TextColor(style.Color(colors.TextPrimary))
		for _, s := range seasons {
			n := s.GetSeasonNumber()
			ok := key(server, title.GetTmdbId(), n)
			open := st.open[ok]
			ui.Column(c.Key(n)).Radius(10).Clip().Border(1, style.Color(colors.Border)).Background(style.Color(colors.SurfaceRaised)).Children(func() {
				b := ui.ButtonBase(c).MinHeight(48).Padding(8, 16).Gap(12).AlignItems(ui.Center).Label(seasonName(c, n))
				b.Children(func() {
					style.Text(ui.Text(c, seasonName(c, n)), style.Type.Headline).TextColor(style.Color(colors.TextPrimary)).Grow(1).SingleLine()
					if s.GetEpisodeCount() > 0 {
						style.Text(ui.Text(c, style.T(c, "item.episodeCount", "count", int(s.GetEpisodeCount()))), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
					}
					if l := statusLabel(c, s.GetStatus()); l != "" {
						style.Text(ui.Text(c, l), style.Type.Footnote).FontWeight(600).TextColor(style.Color(colors.TextPrimary)).Padding(2, 10).Radius(design.RadiusFull).Background(style.Color(colors.Fill))
					}
					chevron := ui.Icon(c, style.Icon("expand", false)).Size(16, 16).TextColor(style.Color(colors.TextSecondary))
					if open {
						chevron.Rotate(180)
					}
				})
				if b.Clicked() {
					st.open[ok] = !open
				}
				if open {
					a.seasonEpisodes(c, server, title.GetTmdbId(), n)
				}
			})
		}
	})
}

// seasonEpisodes builds the episodes of an expanded season, the latest
// first, as Seerr's.
func (a *App) seasonEpisodes(c *ui.Context, server string, tmdb, number int32) {
	colors := style.Colors(c)
	st := &a.discoverState
	if st.seasons == nil {
		st.seasons = map[string]*task[[]*acquisitionv1.SeasonEpisode]{}
	}
	sk := key(server, tmdb, number)
	t := st.seasons[sk]
	if t == nil {
		t = &task[[]*acquisitionv1.SeasonEpisode]{}
		st.seasons[sk] = t
	}
	language := discoverLanguage(c)
	load(a, t, key("season", sk, language), func(ctx context.Context) ([]*acquisitionv1.SeasonEpisode, error) {
		res, err := a.discoverService(server).GetSeason(ctx, acquisitionv1.GetSeasonRequest_builder{TmdbId: &tmdb, SeasonNumber: &number, Language: &language}.Build())
		return res.GetEpisodes(), err
	})
	note := func(text string) {
		style.Text(ui.Text(c, text), style.Type.Callout).TextColor(style.Color(colors.TextSecondary)).Padding(0, 16, 16)
	}
	switch {
	case !t.loaded:
		note(style.T(c, "state.loading"))
		return
	case t.err != nil:
		note(style.T(c, "state.error"))
		return
	case len(t.value) == 0:
		note(style.T(c, "discover.noEpisodes"))
		return
	}
	ui.Column(c).Margin(0, 16).Children(func() {
		for i := len(t.value) - 1; i >= 0; i-- {
			e := t.value[i]
			ui.Box(c.Key(i)).Height(1).Background(style.Color(colors.Border))
			ui.Row(c.Key(e.GetEpisodeNumber())).Padding(16, 0).Gap(16).AlignItems(ui.Start).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
						style.Text(ui.Text(c, style.T(c, "discover.episodeTitle", "number", e.GetEpisodeNumber(), "name", e.GetName())), style.Type.Headline).TextColor(style.Color(colors.TextPrimary))
						if d := e.GetAirDate(); d != "" {
							style.Text(ui.Text(c, dayText(d)), style.Type.Footnote).TextColor(style.Color(colors.TextPrimary)).Padding(2, 10).Radius(design.RadiusFull).Background(style.Color(colors.Fill))
						}
					})
					if o := e.GetOverview(); o != "" {
						style.Text(ui.Text(c, o), style.Type.Callout).TextColor(style.Color(colors.TextSecondary)).Selectable()
					}
				})
				if still := sized(e.GetStillUrl(), 300); still != "" {
					a.artwork(c, artworkOf(still), "", nil, 224, 126, false)
				}
			})
		}
	})
}

// dayText formats a date of the form 2006-01-02 as the web UI's dates.
func dayText(d string) string {
	if t, err := parseDay(d); err == nil {
		return longDate(t)
	}
	return d
}

// titleCast builds a title's cast in billing order.
func (a *App) titleCast(c *ui.Context, server string, title *acquisitionv1.Title) {
	var cast []creditEntry
	for _, e := range titleCredits(title) {
		if e.cast() {
			cast = append(cast, e)
		}
	}
	if len(cast) == 0 {
		return
	}
	slices.SortStableFunc(cast, func(x, y creditEntry) int { return int(x.order - y.order) })
	cast = cast[:min(len(cast), 20)]
	a.shelf(c, key("titleCast", server, title.GetTmdbId()), style.T(c, "item.cast"), nil, func() {
		for i, e := range cast {
			a.personTile(c.Key(i), server, e, e.role, 120)
		}
	})
}

// requestFilters are the statuses the requests page shows, all first.
var requestFilters = []struct {
	label    string
	statuses []acquisitionv1.RequestStatus
}{
	{"requests.all", nil},
	{"requests.pending", []acquisitionv1.RequestStatus{acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING}},
	{"requests.approved", []acquisitionv1.RequestStatus{acquisitionv1.RequestStatus_REQUEST_STATUS_APPROVED}},
	{"requests.available", []acquisitionv1.RequestStatus{acquisitionv1.RequestStatus_REQUEST_STATUS_COMPLETED}},
	{"requests.failed", []acquisitionv1.RequestStatus{acquisitionv1.RequestStatus_REQUEST_STATUS_FAILED}},
	{"requests.declined", []acquisitionv1.RequestStatus{acquisitionv1.RequestStatus_REQUEST_STATUS_DECLINED}},
}

// requestStatuses are how requests of each status show.
var requestStatuses = map[acquisitionv1.RequestStatus]struct {
	label string
	color func(colors *design.Colors) (bg, fg ui.Color)
}{
	acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING: {"requests.pending", func(k *design.Colors) (ui.Color, ui.Color) {
		return style.Color(k.Fill), style.Color(k.TextPrimary)
	}},
	acquisitionv1.RequestStatus_REQUEST_STATUS_APPROVED: {"requests.approved", func(k *design.Colors) (ui.Color, ui.Color) {
		return style.Color(k.Accent), style.Color(k.OnAccent)
	}},
	acquisitionv1.RequestStatus_REQUEST_STATUS_COMPLETED: {"requests.available", func(*design.Colors) (ui.Color, ui.Color) {
		return ui.RGB(0x34, 0xC7, 0x59), ui.RGB(255, 255, 255)
	}},
	acquisitionv1.RequestStatus_REQUEST_STATUS_FAILED: {"requests.failed", func(k *design.Colors) (ui.Color, ui.Color) {
		return style.Color(k.Danger), ui.RGB(255, 255, 255)
	}},
	acquisitionv1.RequestStatus_REQUEST_STATUS_DECLINED: {"requests.declined", func(k *design.Colors) (ui.Color, ui.Color) {
		return style.Color(k.Fill), style.Color(k.TextSecondary)
	}},
}

// requestsPage builds the requests of a server, the user's own or, for
// its administrators, everyone's, filtered by status.
func (a *App) requestsPage(c *ui.Context, r *ui.Route) {
	server := r.Param("server")
	r.Title(style.T(c, "requests.title"))
	colors := style.Colors(c)
	st := &a.discoverState
	if st.requests == nil {
		st.requests = map[string]*requestList{}
	}
	l := st.requests[server]
	if l == nil {
		l = &requestList{limit: 50}
		st.requests[server] = l
	}
	admin := a.isAdmin(server)
	statuses := requestFilters[l.filter].statuses
	limit := l.limit
	t := load(a, &l.list, key("requests", server, l.filter, limit, a.generation, st.reload), func(ctx context.Context) (*acquisitionv1.ListRequestsResponse, error) {
		return a.requestService(server).ListRequests(ctx, acquisitionv1.ListRequestsRequest_builder{Statuses: statuses, Limit: &limit}.Build())
	})
	names := map[string]string{}
	if admin {
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
		for _, x := range u.value {
			names[x.GetId()] = x.GetName()
		}
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Padding(0, 0, 48).Gap(16).Children(func() {
			pageHeader(c, style.T(c, "requests.title"), func() {
				labels := make([]string, len(requestFilters))
				for i, f := range requestFilters {
					labels[i] = style.T(c, f.label)
				}
				ui.Segmented(c, &l.filter, labels...)
			})
			if !admin {
				a.quotas(c, server)
			}
			ui.Column(c).Padding(0, style.Layout.Margin).Gap(8).Children(func() {
				if st.err != nil {
					style.Text(ui.Text(c, errorMessage(c, st.err)), style.Type.Callout).TextColor(style.Color(colors.Danger)).Selectable()
				}
				if taskStatus(c, t, func() { l.list.key = "" }, nil) {
					return
				}
				list := t.value.GetRequests()
				if len(list) == 0 {
					statusView(c, "discover", style.T(c, "requests.none"), style.T(c, "requests.noneMessage"), nil)
					return
				}
				group(c, "requests", "", "", func(g *settingsGroup) {
					for _, req := range list {
						g.divider()
						a.requestRow(c, server, req, admin, names[req.GetRequestedBy()])
					}
					if int(t.value.GetTotal()) > len(list) && g.more(style.T(c, "activity.showMore")) {
						l.limit += 50
					}
				})
			})
		})
	})
	if st.dialog {
		a.requestDialog(c, server)
	}
}

// quotaText tells how many more movies, or seasons, the user may request,
// when limited.
func quotaText(c *ui.Context, q *acquisitionv1.Quota, movies bool) string {
	if q.GetLimit() == 0 {
		return ""
	}
	args := []any{"remaining", q.GetRemaining(), "limit", q.GetLimit(), "days", q.GetDays()}
	k := "requests.seasonQuota"
	if movies {
		k = "requests.movieQuota"
	}
	if q.GetDays() > 0 {
		k += "Days"
	}
	return style.T(c, k, args...)
}

// quotas builds how many more movies and seasons the user may request,
// when limited.
func (a *App) quotas(c *ui.Context, server string) {
	st := &a.discoverState
	if st.quotas == nil {
		st.quotas = map[string]*task[*acquisitionv1.GetQuotaResponse]{}
	}
	t := st.quotas[server]
	if t == nil {
		t = &task[*acquisitionv1.GetQuotaResponse]{}
		st.quotas[server] = t
	}
	load(a, t, key("quota", server, a.generation, st.reload), func(ctx context.Context) (*acquisitionv1.GetQuotaResponse, error) {
		return a.requestService(server).GetQuota(ctx, &acquisitionv1.GetQuotaRequest{})
	})
	var lines []string
	for i, q := range []*acquisitionv1.Quota{t.value.GetMovie(), t.value.GetSeries()} {
		if l := quotaText(c, q, i == 0); l != "" {
			lines = append(lines, l)
		}
	}
	if len(lines) > 0 {
		style.Text(ui.Text(c, strings.Join(lines, " · ")), style.Type.Callout).TextColor(style.Color(style.Colors(c).TextSecondary)).Padding(0, style.Layout.Margin)
	}
}

func seasonsText(c *ui.Context, r *acquisitionv1.Request) string {
	var numbers []int32
	for _, s := range r.GetSeasons() {
		numbers = append(numbers, s.GetSeasonNumber())
	}
	slices.Sort(numbers)
	names := make([]string, len(numbers))
	for i, n := range numbers {
		names[i] = seasonName(c, n)
	}
	return strings.Join(names, ", ")
}

// requestRow builds a request with what the user may do to it.
func (a *App) requestRow(c *ui.Context, server string, r *acquisitionv1.Request, admin bool, userName string) {
	colors := style.Colors(c)
	st := &a.discoverState
	series := r.GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	title := a.discoveredTitle(c, server, r.GetMediaKind(), r.GetTmdbId())
	pending := r.GetStatus() == acquisitionv1.RequestStatus_REQUEST_STATUS_PENDING
	var detail []string
	if series {
		detail = append(detail, style.T(c, "discover.series"))
	} else {
		detail = append(detail, style.T(c, "discover.movie"))
	}
	if s := seasonsText(c, r); s != "" {
		detail = append(detail, s)
	}
	if userName != "" {
		detail = append(detail, style.T(c, "requests.by", "name", userName))
	}
	if r.HasCreateTime() {
		detail = append(detail, fromNow(c, r.GetCreateTime().AsTime()))
	}
	open := func() { a.router.Push(titlePath(server, r.GetMediaKind(), r.GetTmdbId())) }
	name := r.GetTitle()
	if name == "" {
		name = title.value.GetName()
	}
	if name == "" {
		name = fmt.Sprint(r.GetTmdbId())
	}
	ui.Row(c.Key(r.GetId())).Padding(12, 16).Gap(12).AlignItems(ui.Start).Children(func() {
		poster := ui.ButtonBase(c).Label(name)
		poster.Children(func() {
			a.artwork(c, artworkOf(sized(title.value.GetPosterUrl(), 154)), "", nil, 64, 96, poster.Hovered())
		})
		if poster.Clicked() {
			open()
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
				link := ui.ButtonBase(c).Label(name)
				link.Children(func() {
					t := style.Text(ui.Text(c, name), style.Type.Headline).TextColor(style.Color(colors.TextPrimary))
					if link.Hovered() {
						t.Underline()
					}
				})
				if link.Clicked() {
					open()
				}
				if r.GetYear() > 0 {
					style.Text(ui.Text(c, fmt.Sprint(r.GetYear())), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
				}
				if s, ok := requestStatuses[r.GetStatus()]; ok {
					bg, fg := s.color(colors)
					style.Text(ui.Text(c, style.T(c, s.label)), style.Type.Caption).FontWeight(600).TextColor(fg).Padding(2, 8).Radius(design.RadiusFull).Background(bg)
				}
			})
			style.Text(ui.Text(c, strings.Join(detail, " · ")), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary))
			if r.GetStatus() == acquisitionv1.RequestStatus_REQUEST_STATUS_FAILED && r.GetMessage() != "" {
				style.Text(ui.Text(c, r.GetMessage()), style.Type.Footnote).TextColor(style.Color(colors.Danger)).Selectable()
			}
			ui.Row(c).Gap(8).Wrap().Padding(4, 0, 0).AlignItems(ui.Center).Children(func() {
				id := r.GetId()
				if st.removing == id {
					style.Text(ui.Text(c, style.T(c, "requests.removeTitle")), style.Type.Callout).TextColor(style.Color(colors.TextPrimary))
					if smallButton(c.Key("cancel"), style.T(c, "action.cancel"), false) {
						st.removing = ""
					}
					if smallButton(c.Key("remove"), style.T(c, "requests.remove"), st.busy) {
						a.requestAction(server, []string{id}, "remove")
					}
					return
				}
				if admin && pending {
					if ui.PrimaryButton(c, style.T(c, "requests.approve")).Disabled(st.busy).Clicked() {
						a.requestAction(server, []string{id}, "approve")
					}
					if smallButton(c.Key("decline"), style.T(c, "requests.decline"), st.busy) {
						a.requestAction(server, []string{id}, "decline")
					}
				}
				if admin && r.GetStatus() == acquisitionv1.RequestStatus_REQUEST_STATUS_FAILED {
					if ui.PrimaryButton(c, style.T(c, "requests.retry")).Disabled(st.busy).Clicked() {
						a.requestAction(server, []string{id}, "retry")
					}
				}
				if pending && series && title.value != nil {
					if smallButton(c.Key("edit"), style.T(c, "requests.editSeasons"), st.busy) {
						a.askSeasons(title.value, r)
					}
				}
				if admin || pending {
					label := style.T(c, "requests.remove")
					b := ui.ButtonBase(c.Key("removeLink")).Label(label)
					b.Children(func() {
						t := style.Text(ui.Text(c, label), style.Type.Callout).FontWeight(600).TextColor(style.Color(colors.Danger))
						if b.Hovered() {
							t.Underline()
						}
					})
					if b.Clicked() {
						st.removing = id
					}
				}
			})
		})
	})
}

// artworkOf returns the artwork of an image's address.
func artworkOf(u string) *capiv1.Artwork {
	return capiv1.Artwork_builder{Url: u}.Build()
}

// parseDay parses a date of the form 2006-01-02.
func parseDay(d string) (time.Time, error) {
	return time.Parse(time.DateOnly, d)
}

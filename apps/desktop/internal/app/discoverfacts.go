package app

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"

	design "github.com/mavioai/mavio/libs/design/gen/go"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// productionStatuses name where a title is in its making and run.
var productionStatuses = map[pluginv1.ProductionStatus]string{
	pluginv1.ProductionStatus_PRODUCTION_STATUS_RUMORED:         "discover.statusRumored",
	pluginv1.ProductionStatus_PRODUCTION_STATUS_PLANNED:         "discover.statusPlanned",
	pluginv1.ProductionStatus_PRODUCTION_STATUS_IN_PRODUCTION:   "discover.statusInProduction",
	pluginv1.ProductionStatus_PRODUCTION_STATUS_POST_PRODUCTION: "discover.statusPostProduction",
	pluginv1.ProductionStatus_PRODUCTION_STATUS_RELEASED:        "discover.statusReleased",
	pluginv1.ProductionStatus_PRODUCTION_STATUS_CANCELED:        "discover.statusCanceled",
	pluginv1.ProductionStatus_PRODUCTION_STATUS_PILOT:           "discover.statusPilot",
	pluginv1.ProductionStatus_PRODUCTION_STATUS_RETURNING:       "discover.statusReturning",
	pluginv1.ProductionStatus_PRODUCTION_STATUS_ENDED:           "discover.statusEnded",
}

// crewJobs are the jobs Seerr shows first of a title's crew, in its order,
// after a series' creators.
var crewJobs = []string{"Director", "Creator", "Screenplay", "Writer", "Original Music Composer", "Composer", "Editor", "Producer", "Co-Producer", "Executive Producer", "Animation"}

// kindJobs are the jobs of credits without a more specific one.
var kindJobs = map[libraryv1.CreditKind]string{
	libraryv1.CreditKind_CREDIT_KIND_DIRECTOR: "Director",
	libraryv1.CreditKind_CREDIT_KIND_CREATOR:  "Creator",
	libraryv1.CreditKind_CREDIT_KIND_WRITER:   "Writer",
	libraryv1.CreditKind_CREDIT_KIND_COMPOSER: "Composer",
	libraryv1.CreditKind_CREDIT_KIND_PRODUCER: "Producer",
}

// jobRank returns where a crew credit comes among the leading crew, as
// Seerr orders it: a series' creators, then by crewJobs.
func jobRank(e creditEntry) int {
	if e.kind == libraryv1.CreditKind_CREDIT_KIND_CREATOR {
		return -1
	}
	job := e.role
	if job == "" {
		job = kindJobs[e.kind]
	}
	if i := slices.Index(crewJobs, job); i >= 0 {
		return i
	}
	return len(crewJobs)
}

// titleCredits returns a discovered title's credits, an actor credited
// for several characters once.
func titleCredits(title *acquisitionv1.Title) []creditEntry {
	seen := map[string]bool{}
	var out []creditEntry
	for _, cr := range title.GetCredits() {
		e := creditEntry{
			name: cr.GetName(), role: cr.GetRole(), kind: cr.GetKind(), order: cr.GetOrder(),
			photo: sized(cr.GetImageUrl(), 185), tmdb: cr.GetExternalIds()["tmdb"],
		}
		if e.cast() {
			id := e.tmdb + "|" + e.name
			if seen[id] {
				continue
			}
			seen[id] = true
		}
		out = append(out, e)
	}
	return out
}

// regionName names an ISO 3166-1 country in the app's language.
func regionName(c *ui.Context, code string) string {
	region, err := language.ParseRegion(code)
	if err != nil {
		return code
	}
	name := code
	if namer := display.Regions(language.Make(c.Locale())); namer != nil {
		if n := namer.Name(region); n != "" {
			name = n
		}
	} else if n := display.Regions(language.English).Name(region); n != "" {
		name = n
	}
	return name
}

// countryName names an ISO 3166-1 country in the app's language, after
// its flag.
func countryName(c *ui.Context, code string) string {
	name := regionName(c, code)
	if _, err := language.ParseRegion(code); err == nil && len(code) == 2 {
		var flag strings.Builder
		for _, r := range strings.ToUpper(code) {
			flag.WriteRune(0x1F1A5 + r)
		}
		return flag.String() + " " + name
	}
	return name
}

// moneyText formats US dollars as the web UI's, without cents.
func moneyText(dollars int64) string {
	if dollars <= 0 {
		return ""
	}
	s := strconv.FormatInt(dollars, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return "$" + b.String()
}

// titleOverview builds a discovered title's overview, leading crew and
// keywords beside its collection and facts, as Seerr's. On the
// television it is a region of the page's flow, its two columns side by
// side (docs/tv.md §6.3).
func (a *App) titleOverview(c *ui.Context, server string, title *acquisitionv1.Title) {
	a.inFlow(func() {
		ui.Row(c).Padding(0, style.Layout(c).Margin).GapX(32).GapY(32).AlignItems(ui.Start).Wrap().Children(func() {
			a.inColumn(func() { a.overviewColumn(c, server, title) })
			a.inColumn(func() {
				a.marked(focusMarks{beside: true}, func() {
					ui.Column(c).Basis(320).MaxWidth(380).Grow(1).MinWidth(0).Gap(24).Children(func() {
						a.collectionCard(c, server, title)
						a.titleFactBox(c, server, title)
					})
				})
			})
		})
	})
}

// overviewColumn builds a title's tagline, overview, leading crew and
// keywords.
func (a *App) overviewColumn(c *ui.Context, server string, title *acquisitionv1.Title) {
	colors := style.Colors(c)
	ui.Column(c).Basis(480).Grow(1).MinWidth(0).Gap(16).Children(func() {
		if t := title.GetTagline(); t != "" {
			style.Text(ui.Text(c, t), style.Type(c).Title3).FontWeight(400).Italic().TextColor(style.Color(colors.TextSecondary))
		}
		style.Text(ui.Text(c, style.T(c, "discover.overview")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
		overview := title.GetOverview()
		if overview == "" {
			overview = style.T(c, "discover.noOverview")
		}
		style.Text(ui.Text(c, overview), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Selectable()
		a.leadingCrew(c, server, titleCredits(title), func() {
			a.router.Push(titlePath(server, title.GetMediaKind(), title.GetTmdbId()) + "/crew")
		})
		a.marked(focusMarks{block: "keywords"}, func() {
			a.keywords(c, server, title)
		})
	})
}

// leadingCrew builds the leading crew, as Seerr's: six of them, a series'
// creators first, then by the jobs Seerr shows first, each opening their
// page; and under them the link to the whole crew.
func (a *App) leadingCrew(c *ui.Context, server string, credits []creditEntry, more func()) {
	type ranked struct {
		e    creditEntry
		rank int
	}
	var crew []ranked
	for _, e := range credits {
		if !e.cast() && crewRank(e.kind) < len(crewLabels) {
			crew = append(crew, ranked{e, jobRank(e)})
		}
	}
	if len(crew) == 0 {
		return
	}
	slices.SortStableFunc(crew, func(x, y ranked) int { return x.rank - y.rank })
	names := make([]creditEntry, 0, 6)
	for _, r := range crew[:min(len(crew), 6)] {
		names = append(names, r.e)
	}
	ui.Column(c).Gap(12).Padding(8, 0, 0).Children(func() {
		a.marked(focusMarks{block: "crew", beside: true}, func() {
			a.crewNames(c, server, names)
		})
		a.marked(focusMarks{block: "crew", beside: true, end: true}, func() {
			if a.crewMore(c).Clicked() {
				more()
			}
		})
	})
}

// keywords builds a title's keywords as pills, each opening the popular
// titles of the keyword where a catalog can filter by it.
func (a *App) keywords(c *ui.Context, server string, title *acquisitionv1.Title) {
	refs := title.GetKeywordRefs()
	if len(refs) == 0 {
		for _, k := range title.GetKeywords() {
			refs = append(refs, pluginv1.Reference_builder{Name: &k}.Build())
		}
	}
	if len(refs) == 0 {
		return
	}
	colors := style.Colors(c)
	ui.Row(c).Gap(8).Wrap().Padding(8, 0, 0).Children(func() {
		for i, k := range refs {
			linked := k.GetId() != "" && a.alikePath(c, server, title.GetMediaKind(), catalogFilter{Keywords: namedOf([]*pluginv1.Reference{k})}) != ""
			if !linked {
				style.Text(ui.Text(c.Key(i), k.GetName()), style.Type(c).Footnote).TextColor(style.Color(colors.TextPrimary)).
					Padding(4, 12).Radius(design.RadiusFull).Background(style.Color(colors.Fill))
				continue
			}
			b := ui.ButtonBase(c.Key(i)).Padding(4, 12).Radius(design.RadiusFull).Background(style.Color(colors.Fill)).Label(k.GetName())
			fg := style.Color(colors.TextPrimary)
			switch {
			case a.tvLink(c, b, design.RadiusFull):
				fg = ui.RGB(0, 0, 0)
			case b.Hovered():
				b.Background(style.Color(colors.FillSelected))
			}
			b.Children(func() {
				style.Text(ui.Text(c, k.GetName()), style.Type(c).Footnote).TextColor(fg)
			})
			if b.Clicked() {
				a.router.Push(a.alikePath(c, server, title.GetMediaKind(), catalogFilter{Keywords: namedOf([]*pluginv1.Reference{k})}))
			}
		}
	})
}

// collectionCard builds a movie's collection as Seerr's card: its
// backdrop and name, opening its page.
func (a *App) collectionCard(c *ui.Context, server string, title *acquisitionv1.Title) {
	id64, _ := strconv.ParseInt(title.GetExternalIds()["tmdb_collection"], 10, 32)
	id := int32(id64)
	if id <= 0 {
		return
	}
	col := a.collection(c, server, id).value
	name := col.GetName()
	if name == "" {
		name = title.GetCollectionName()
	}
	if name == "" {
		name = style.T(c, "item.collection")
	}
	white := ui.RGB(255, 255, 255)
	b := ui.ButtonBase(c).Height(80).Radius(14).Clip().AlignItems(ui.Center).
		Background(style.Color(design.DarkColors.SurfaceRaised)).Label(name)
	hovered := b.Hovered()
	if a.tvFocus(c, b, false) {
		hovered = true
		artRing(c, b, true)
	}
	b.Children(func() {
		backdrop := col.GetBackdropUrl()
		if backdrop == "" {
			backdrop = col.GetPosterUrl()
		}
		if bitmap := a.opts.Images.Get(sized(backdrop, 780), 380, scale); bitmap != nil {
			img := ui.Image(c, bitmap).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover)
			if hovered {
				img.Scale(1.05, 1.05)
			}
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 180,
			Stops: []ui.GradientStop{{At: 0, Color: ui.RGBA(0, 0, 0, 0.45)}, {At: 1, Color: ui.RGBA(0, 0, 0, 0.8)}},
		})
		// Above the backdrop and its gradient, which are drawn after the flow.
		ui.Row(c).Absolute().Top(0).Left(16).Right(16).Bottom(0).Gap(12).AlignItems(ui.Center).Children(func() {
			style.Text(ui.Text(c, name), style.Type(c).Headline).TextColor(white).SingleLine().Grow(1).MinWidth(0)
			style.Text(ui.Text(c, style.T(c, "collection.view")), style.Type(c).Footnote).FontWeight(600).TextColor(white).
				Padding(4, 12).Radius(design.RadiusFull).Background(white.Alpha(0.2)).Shrink(0)
		})
	})
	if b.Clicked() {
		a.router.Push(collectionPath(server, id))
	}
}

// ratings loads a title's ratings elsewhere.
func (a *App) ratings(server string, kind acquisitionv1.MediaKind, tmdb int32) *task[[]*pluginv1.Rating] {
	st := &a.discoveredState
	return load(a, taskIn(&st.ratings, key(server, kind, tmdb)), key("ratings", server, kind, tmdb, a.generation), func(ctx context.Context) ([]*pluginv1.Rating, error) {
		res, err := a.discoverService(server).GetRatings(ctx, acquisitionv1.GetRatingsRequest_builder{MediaKind: &kind, TmdbId: &tmdb}.Build())
		return res.GetRatings(), err
	})
}

// ratingOrder are the ratings shown beside TMDB's score, in Seerr's order.
var ratingOrder = []pluginv1.RatingSource{
	pluginv1.RatingSource_RATING_SOURCE_ROTTEN_TOMATOES_CRITICS,
	pluginv1.RatingSource_RATING_SOURCE_ROTTEN_TOMATOES_AUDIENCE,
	pluginv1.RatingSource_RATING_SOURCE_IMDB,
}

// titleRatings builds a title's ratings as Seerr's: Rotten Tomatoes' and
// IMDb's, then TMDB's score, each opening its page.
func (a *App) titleRatings(c *ui.Context, server string, title *acquisitionv1.Title) {
	kind, tmdb := title.GetMediaKind(), title.GetTmdbId()
	rs := a.ratings(server, kind, tmdb).value
	type shown struct {
		logo            string
		score, tip, url string
	}
	var all []shown
	for _, src := range ratingOrder {
		for _, r := range rs {
			if r.GetSource() != src {
				continue
			}
			fresh := r.GetScore() >= 60
			switch src {
			case pluginv1.RatingSource_RATING_SOURCE_IMDB:
				all = append(all, shown{"imdb", fmt.Sprintf("%.1f", r.GetScore()), style.T(c, "discover.imdbScore", "votes", compactCount(r.GetVotes())), r.GetUrl()})
			case pluginv1.RatingSource_RATING_SOURCE_ROTTEN_TOMATOES_CRITICS:
				tip := style.T(c, "discover.tomatometer")
				if r.GetCertified() {
					tip = style.T(c, "discover.tomatometerCertified")
				}
				logo := "rt-fresh"
				if !fresh {
					logo = "rt-rotten"
				}
				all = append(all, shown{logo, fmt.Sprintf("%.0f%%", r.GetScore()), tip, r.GetUrl()})
			default:
				logo := "rt-aud-fresh"
				if !fresh {
					logo = "rt-aud-rotten"
				}
				all = append(all, shown{logo, fmt.Sprintf("%.0f%%", r.GetScore()), style.T(c, "discover.audienceScore"), r.GetUrl()})
			}
		}
	}
	if title.GetVoteCount() > 0 {
		all = append(all, shown{"tmdb-wide", fmt.Sprintf("%.0f%%", title.GetCommunityRating()*10), style.T(c, "discover.tmdbScore"), tmdbURL(c, kind, tmdb)})
	}
	if len(all) == 0 {
		return
	}
	colors := style.Colors(c)
	ui.Row(c).Padding(10, 16).GapX(16).GapY(4).Justify(ui.Center).Wrap().Children(func() {
		for i, s := range all {
			b := ui.ButtonBase(c.Key(i)).Gap(6).AlignItems(ui.Center).Label(s.tip).Tooltip(s.tip)
			fg := style.Color(colors.TextPrimary)
			lit := a.tvLink(c, b, 6)
			if lit {
				fg = ui.RGB(0, 0, 0)
			}
			b.Children(func() {
				// The logos 20 high, TMDB's wide one 16, as the web UI's.
				logo, h := style.Service(s.logo, false), float32(20)
				if s.logo == "tmdb-wide" {
					h = 16
				}
				w, lh := logo.Size()
				ui.Image(c, logo).Size(h*w/lh, h).Label(s.tip)
				t := style.Text(ui.Text(c, s.score), style.Type(c).Callout).FontWeight(600).TextColor(fg)
				if b.Hovered() && !lit {
					t.Underline()
				}
			})
			if b.Clicked() && s.url != "" && a.opts.OpenURL != nil {
				_ = a.opts.OpenURL(s.url)
			}
		}
	})
}

// compactCount formats a count as the web UI's compact numbers, such as
// 1.2M.
func compactCount(n int64) string {
	f := float64(n)
	switch {
	case n >= 1_000_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", f/1e9), ".0") + "B"
	case n >= 1_000_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", f/1e6), ".0") + "M"
	case n >= 1_000:
		return strings.TrimSuffix(fmt.Sprintf("%.1f", f/1e3), ".0") + "K"
	}
	return strconv.FormatInt(n, 10)
}

// tmdbURL returns a title's page on TMDB in the app's language.
func tmdbURL(c *ui.Context, kind acquisitionv1.MediaKind, tmdb int32) string {
	k := "movie"
	if kind == acquisitionv1.MediaKind_MEDIA_KIND_SERIES {
		k = "tv"
	}
	return fmt.Sprintf("https://www.themoviedb.org/%s/%d?language=%s", k, tmdb, c.Locale())
}

// titleLinks returns the links of a title elsewhere, as Seerr's.
func titleLinks(c *ui.Context, title *acquisitionv1.Title, ratings []*pluginv1.Rating) []externalLink {
	series := title.GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	tmdb := title.GetTmdbId()
	imdb := title.GetExternalIds()["imdb"]
	tvdb := title.GetExternalIds()["tvdb"]
	if tvdb == "" && title.GetTvdbId() > 0 {
		tvdb = strconv.Itoa(int(title.GetTvdbId()))
	}
	links := []externalLink{{"TMDB", tmdbURL(c, title.GetMediaKind(), tmdb)}}
	if series && tvdb != "" {
		links = append(links, externalLink{"TVDB", "https://www.thetvdb.com/dereferrer/series/" + tvdb})
	}
	if imdb != "" {
		links = append(links, externalLink{"IMDb", "https://www.imdb.com/title/" + imdb})
	}
	for _, r := range ratings {
		if r.GetSource() != pluginv1.RatingSource_RATING_SOURCE_IMDB && r.GetUrl() != "" {
			links = append(links, externalLink{"Rotten Tomatoes", r.GetUrl()})
			break
		}
	}
	if imdb != "" {
		kind := "movies"
		if series {
			kind = "shows"
		}
		links = append(links,
			externalLink{"Trakt", "https://trakt.tv/" + kind + "/" + imdb},
			externalLink{"Simkl", "https://api.simkl.com/redirect?to=Simkl&imdb=" + imdb})
	}
	if !series {
		links = append(links, externalLink{"Letterboxd", fmt.Sprintf("https://letterboxd.com/tmdb/%d", tmdb)})
	}
	return links
}

// serviceLogos are the logos of titleLinks' links, by name, as Seerr's:
// each as wide as Seerr's, the mono ones white alone and drawn in the
// text's color.
var serviceLogos = map[string]struct {
	logo  string
	width float32
	mono  bool
}{
	"TMDB":            {"tmdb", 32, false},
	"TVDB":            {"tvdb", 36, true},
	"IMDb":            {"imdb", 32, false},
	"Rotten Tomatoes": {"rt", 56, false},
	"Trakt":           {"trakt", 32, false},
	"Simkl":           {"simkl", 32, true},
	"Letterboxd":      {"letterboxd", 32, false},
}

// logoLink builds a link elsewhere as its logo, which logo builds in the
// color given, faint until hovered or focused, as the web UI's.
func (a *App) logoLink(c *ui.Context, label string, logo func(fg ui.Color)) ui.Element {
	b := ui.ButtonBase(c).AlignItems(ui.Center).Cursor(ui.CursorPointer).Label(label).Tooltip(label)
	fg := style.Color(style.Colors(c).TextPrimary)
	switch {
	case a.tvLink(c, b, 6):
		fg = ui.RGB(0, 0, 0)
	case !b.Hovered():
		b.Opacity(0.5)
	}
	b.Children(func() { logo(fg) })
	return b
}

// minStudios are the studios shown before Show More.
const minStudios = 5

// titleFactBox builds a discovered title's facts in a bordered box, as
// Seerr's: its ratings, then each fact its label beside its value, its
// streaming services, and its links elsewhere.
func (a *App) titleFactBox(c *ui.Context, server string, title *acquisitionv1.Title) {
	colors := style.Colors(c)
	st := &a.discoveredState
	series := title.GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	kind := title.GetMediaKind()
	divider := func(i int) {
		if i > 0 {
			ui.Box(c.Key(key("divider", i))).Height(1).Background(style.Color(colors.Border))
		}
	}
	n := 0
	row := func(label string, values func()) {
		divider(n)
		n++
		ui.Row(c.Key(label)).Padding(10, 16).Gap(16).AlignItems(ui.Start).Justify(ui.SpaceBetween).Children(func() {
			style.Text(ui.Text(c, label), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.TextPrimary)).Shrink(0)
			ui.Column(c).MinWidth(0).AlignItems(ui.End).Children(values)
		})
	}
	value := func(text string) {
		style.Text(ui.Text(c.Key(text), text), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).TextAlign(ui.End).Selectable()
	}
	linked := func(text string, f catalogFilter) {
		path := a.alikePath(c, server, kind, f)
		if path == "" {
			value(text)
			return
		}
		b := ui.ButtonBase(c.Key(text)).Cursor(ui.CursorPointer).Label(text)
		lit := a.tvLink(c, b, 6)
		b.Children(func() {
			t := style.Text(ui.Text(c, text), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).TextAlign(ui.End)
			switch {
			case lit:
				t.TextColor(ui.RGB(0, 0, 0))
			case b.Hovered():
				t.Underline()
			}
		})
		if b.Clicked() {
			a.router.Push(path)
		}
	}
	rs := a.ratings(server, kind, title.GetTmdbId()).value
	ui.Column(c).Radius(14).Clip().Border(1, style.Color(colors.Border)).Children(func() {
		before := n
		a.titleRatings(c, server, title)
		if len(rs) > 0 || title.GetVoteCount() > 0 {
			n = before + 1
		}
		if o := title.GetOriginalTitle(); o != "" && title.GetOriginalLanguage() != discoverLanguage(c) {
			row(style.T(c, "item.originalTitle"), func() { value(o) })
		}
		if s, ok := productionStatuses[title.GetProductionStatus()]; ok {
			row(style.T(c, "item.status"), func() { value(style.T(c, s)) })
		}
		type release struct{ icon, label, date string }
		var releases []release
		if !series {
			for _, r := range []release{
				{"movies", style.T(c, "discover.theatricalRelease"), title.GetTheatricalReleaseDate()},
				{"devices", style.T(c, "item.digitalRelease"), title.GetDigitalReleaseDate()},
				{"collections", style.T(c, "item.physicalRelease"), title.GetPhysicalReleaseDate()},
			} {
				if r.date != "" {
					releases = append(releases, r)
				}
			}
		}
		switch {
		case len(releases) > 0:
			label := style.T(c, "discover.releaseDate")
			if len(releases) > 1 {
				label = style.T(c, "discover.releaseDates")
			}
			row(label, func() {
				// Each date with the icon of how it was released, named on
				// hover.
				for _, r := range releases {
					ui.Row(c.Key(r.label)).Gap(6).AlignItems(ui.Center).Tooltip(r.label).Children(func() {
						ui.Icon(c, style.Icon(r.icon, false)).Size(14, 14).TextColor(style.Color(colors.TextSecondary)).Label(r.label)
						style.Text(ui.Text(c, dayText(r.date)), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
					})
				}
			})
		case title.GetPremiereDate() != "":
			label := style.T(c, "item.released")
			if series {
				label = style.T(c, "item.firstAired")
			}
			row(label, func() { value(dayText(title.GetPremiereDate())) })
		}
		if series && title.GetEndDate() != "" && title.GetProductionStatus() == pluginv1.ProductionStatus_PRODUCTION_STATUS_ENDED {
			row(style.T(c, "item.lastAired"), func() { value(dayText(title.GetEndDate())) })
		}
		if series && title.GetNextAirDate() != "" {
			row(style.T(c, "item.nextAirDate"), func() { value(dayText(title.GetNextAirDate())) })
		}
		if m := title.GetRuntimeMinutes(); m > 0 {
			label := style.T(c, "item.runtime")
			if series {
				label = style.T(c, "discover.episodeRuntime")
			}
			row(label, func() { value(style.T(c, "time.minutes", "count", int(m))) })
		}
		if s := moneyText(title.GetRevenue()); s != "" {
			row(style.T(c, "item.revenue"), func() { value(s) })
		}
		if s := moneyText(title.GetBudget()); s != "" {
			row(style.T(c, "item.budget"), func() { value(s) })
		}
		if l := title.GetOriginalLanguage(); l != "" {
			row(style.T(c, "item.originalLanguage"), func() { linked(languageName(c, l), catalogFilter{OriginalLanguage: l}) })
		}
		countries := title.GetProductionLocations()
		if codes := title.GetProductionCountries(); len(codes) > 0 {
			countries = nil
			for _, code := range codes {
				countries = append(countries, countryName(c, code))
			}
		}
		if len(countries) > 0 {
			row(style.T(c, "item.country"), func() {
				for _, s := range countries {
					value(s)
				}
			})
		}
		studios := title.GetStudioRefs()
		if len(studios) == 0 {
			for _, s := range title.GetStudios() {
				studios = append(studios, pluginv1.Reference_builder{Name: &s}.Build())
			}
		}
		if len(studios) > 0 {
			label := style.T(c, "item.studio")
			if series {
				label = style.T(c, "item.networks")
			}
			row(label, func() {
				shown := studios
				if !st.allStudios {
					shown = studios[:min(len(studios), minStudios)]
				}
				for _, s := range shown {
					if s.GetId() == "" {
						value(s.GetName())
						continue
					}
					f := catalogFilter{Studios: namedOf([]*pluginv1.Reference{s})}
					if series {
						f = catalogFilter{Networks: namedOf([]*pluginv1.Reference{s})}
					}
					linked(s.GetName(), f)
				}
				if len(studios) > minStudios {
					label := style.T(c, "discover.showMore")
					if st.allStudios {
						label = style.T(c, "action.showLess")
					}
					b := ui.ButtonBase(c).Gap(4).AlignItems(ui.Center).Label(label)
					fg := style.Color(colors.TextSecondary)
					if a.tvLink(c, b, 6) {
						fg = ui.RGB(0, 0, 0)
					}
					b.Children(func() {
						style.Text(ui.Text(c, label), style.Type(c).Callout).TextColor(fg)
						chevron := ui.Icon(c, style.Icon("expand", false)).Size(14, 14).TextColor(fg)
						if st.allStudios {
							chevron.Rotate(180)
						}
					})
					if b.Clicked() {
						st.allStudios = !st.allStudios
					}
				}
			})
		}
		if ps := title.GetWatchProviders(); len(ps) > 0 {
			divider(n)
			n++
			ui.Column(c).Padding(10, 16).Gap(8).Children(func() {
				style.Text(ui.Text(c, style.T(c, "item.streaming")), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.TextPrimary))
				ui.Row(c).Gap(12).Wrap().Children(func() {
					for i, p := range ps {
						if bitmap := a.opts.Images.Get(p.GetLogoUrl(), 32, scale); bitmap != nil {
							ui.Box(c.Key(i)).Size(32, 32).Radius(6).Clip().Opacity(0.7).Tooltip(p.GetName()).Children(func() {
								ui.Image(c, bitmap).Fill().Fit(ui.Cover)
							})
						} else {
							style.Text(ui.Text(c.Key(i), p.GetName()), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
						}
					}
				})
			})
		}
		divider(n)
		ui.Column(c).Padding(12, 16).AlignItems(ui.Start).Children(func() {
			ui.Row(c).GapX(20).GapY(12).Justify(ui.Start).AlignItems(ui.Center).Wrap().Children(func() {
				// First Mavio's, opening the item when the server has it,
				// where Seerr links its media server.
				if id := title.GetItemId(); id != "" {
					if a.logoLink(c, style.T(c, "discover.openItem"), func(ui.Color) {
						ui.Image(c, style.Wordmark).Size(14*244.0/64, 14)
					}).Clicked() {
						a.router.Push("/item/" + server + "/" + id)
					}
				}
				if a.opts.OpenURL != nil {
					for i, l := range titleLinks(c, title, rs) {
						s := serviceLogos[l.name]
						if a.logoLink(c.Key(i), l.name, func(fg ui.Color) {
							logo := style.Service(s.logo, s.mono)
							w, h := logo.Size()
							if s.mono {
								ui.Icon(c, logo).Size(s.width, s.width*h/w).TextColor(fg)
							} else {
								ui.Image(c, logo).Size(s.width, s.width*h/w)
							}
						}).Clicked() {
							_ = a.opts.OpenURL(l.url)
						}
					}
				}
			})
		})
	})
}

// titleCreditsPage builds the whole cast and crew of a discovered title,
// which the cast's See All opens, or with crew its crew alone, by job,
// which View Full Crew opens, as Seerr's crew page.
func (a *App) titleCreditsPage(c *ui.Context, r *ui.Route, crew bool) {
	server, kind, tmdb := r.Param("server"), mediaKindOf(r.Param("kind")), tmdbParam(r)
	t := a.discoveredTitle(c, server, kind, tmdb)
	if taskStatus(c, t, func() { a.discoverState.titles[key(server, kind, tmdb)].key = "" }, nil) {
		return
	}
	title := style.T(c, "item.cast")
	credits := titleCredits(t.value)
	if crew {
		title = style.T(c, "credits.fullCrew")
		credits = slices.DeleteFunc(credits, creditEntry.cast)
	}
	r.Title(title)
	of := func(kind libraryv1.CreditKind) []creditEntry {
		var out []creditEntry
		for _, e := range credits {
			if e.kind == kind {
				out = append(out, e)
			}
		}
		slices.SortStableFunc(out, func(x, y creditEntry) int { return int(x.order - y.order) })
		return out
	}
	colors := style.Colors(c)
	ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Fill().Children(func() {
		ui.Column(c).Gap(24).Padding(0, 0, 40).Children(func() {
			a.pageHeader(c, title, func() {
				style.Text(ui.Text(c, t.value.GetName()), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
			})
			if len(credits) == 0 {
				style.Text(ui.Text(c, style.T(c, "credits.none")), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary)).Margin(0, style.Layout(c).Margin)
			}
			a.people(c, server, style.T(c, "credits.cast"), of(libraryv1.CreditKind_CREDIT_KIND_ACTOR))
			a.people(c, server, style.T(c, "credits.guestStars"), of(libraryv1.CreditKind_CREDIT_KIND_GUEST_STAR))
			for _, l := range crewLabels {
				a.people(c, server, style.T(c, l.label), of(l.kind))
			}
		})
	})
}

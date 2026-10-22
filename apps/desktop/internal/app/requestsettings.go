package app

import (
	"cmp"
	"context"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/proto"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// requestsForm is the Requests page's draft: the settings loaded and as
// edited, the keywords searched for and the names of those added.
type requestsForm struct {
	loaded, draft *acquisitionv1.RequestSettings
	settings      task[*acquisitionv1.RequestSettings]
	catalogs      task[[]*acquisitionv1.Catalog]
	found, named  task[[]*pluginv1.FilterValue]
	text, query   string
	added         []*pluginv1.FilterValue
}

var (
	availabilityChoices = []enumChoice[acquisitionv1.MinimumAvailability]{
		{acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_ANNOUNCED, "requestsettings.announced"},
		{acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_IN_CINEMAS, "requestsettings.inCinemas"},
		{acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_RELEASED, "requestsettings.released"},
	}
	monitorChoices = []enumChoice[acquisitionv1.NewItemMonitor]{
		{acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_ALL, "requestsettings.monitorAll"},
		{acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_NONE, "requestsettings.monitorNone"},
	}
)

// requestSettingsPage builds the Requests: where approved requests go —
// the library and quality profile of movies, of series and of anime —
// what new users may request and their quotas, and the keywords whose
// titles are blocklisted, as Seerr's Radarr and Sonarr servers, user
// defaults and blocklisted tags.
func (a *App) requestSettingsPage(c *ui.Context) {
	st := &a.settingsState.admin.acquisition
	f := &st.requests
	a.backTo(c, style.T(c, "settings.acquisition"), acquisitionPath("", ""))
	sectionHeader(c, style.T(c, "requestsettings.title"), style.T(c, "requestsettings.message"))
	t := adminTask(a, &f.settings, "request-settings", func(ctx context.Context, s string) (*acquisitionv1.RequestSettings, error) {
		res, err := a.requestService(s).GetRequestSettings(ctx, &acquisitionv1.GetRequestSettingsRequest{})
		return res.GetSettings(), err
	})
	if taskStatus(c, t, func() { f.settings.key = "" }, nil) {
		return
	}
	if t.value != f.loaded && (f.loaded == nil || proto.Equal(f.draft, f.loaded)) {
		f.loaded, f.draft = t.value, proto.CloneOf(t.value)
	}
	s := f.draft
	// The choices wait for the libraries and profiles, lest a choice show
	// its ID.
	lt, pt := a.adminLibraries(), a.qualityProfiles()
	if !lt.loaded || !pt.loaded {
		return
	}
	libraries, profiles := lt.value, pt.value
	type option struct{ id, name string }
	librariesOf := func(kind libraryv1.LibraryKind) []option {
		var out []option
		for _, l := range libraries {
			if l.GetKind() == kind {
				out = append(out, option{l.GetId(), l.GetName()})
			}
		}
		return out
	}
	profilesOf := func(kind acquisitionv1.MediaKind) []option {
		var out []option
		for _, p := range profiles {
			if p.GetKind() == kind {
				out = append(out, option{p.GetId(), p.GetName()})
			}
		}
		return out
	}
	choice := func(g *settingsGroup, label string, options []option, none string, get func() string, set func(string)) {
		g.row(label, "", nil, func() {
			v := get()
			// A choice no longer offered, such as a deleted library, stays
			// shown until changed.
			if v != "" && !slices.ContainsFunc(options, func(o option) bool { return o.id == v }) {
				options = append(options, option{v, v})
			}
			labels, i := []string{none}, 0
			for j, o := range options {
				labels = append(labels, o.name)
				if o.id == v {
					i = j + 1
				}
			}
			if choose(c, label, &i, labels) {
				set("")
				if i > 0 {
					set(options[i-1].id)
				}
			}
		})
	}
	toggle := func(g *settingsGroup, label, detail string, get func() bool, set func(bool)) {
		on := get()
		g.switchRow(label, detail, &on)
		if on != get() {
			set(on)
		}
	}
	quota := func(g *settingsGroup, label string, limit, days func() int32, setLimit, setDays func(int32)) {
		g.row(label, style.T(c, "requestsettings.quotaDetail"), nil, func() {
			ui.Row(c.Key(label)).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
				l, d := float64(limit()), float64(days())
				if ui.NumberInput(c.Key("limit"), &l, 0, 1000, 1).Label(style.T(c, "users.quotaLimit")).Changed() {
					setLimit(int32(l))
				}
				secondaryText(c, style.T(c, "users.quotaEvery"))
				if ui.NumberInput(c.Key("days"), &d, 0, 1000, 1).Label(style.T(c, "users.quotaDays")).Changed() {
					setDays(int32(d))
				}
				secondaryText(c, style.T(c, "users.quotaDaysUnit"))
			})
		})
	}
	none, same := style.T(c, "requestsettings.none"), style.T(c, "requestsettings.sameAsSeries")
	library, profile := style.T(c, "requestsettings.library"), style.T(c, "requestsettings.profile")
	movies, shows := libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, libraryv1.LibraryKind_LIBRARY_KIND_SHOWS
	movie, series := acquisitionv1.MediaKind_MEDIA_KIND_MOVIE, acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	group(c, "movies", style.T(c, "requestsettings.movies"), style.T(c, "requestsettings.moviesFooter"), func(g *settingsGroup) {
		choice(g, library, librariesOf(movies), none, s.GetMovieLibraryId, s.SetMovieLibraryId)
		choice(g, profile, profilesOf(movie), none, s.GetMovieQualityProfileId, s.SetMovieQualityProfileId)
		g.row(style.T(c, "requestsettings.minimumAvailability"), style.T(c, "requestsettings.minimumAvailabilityDetail"), nil, func() {
			v := s.GetMinimumAvailability()
			if chooseEnum(c, style.T(c, "requestsettings.minimumAvailability"), &v, acquisitionv1.MinimumAvailability_MINIMUM_AVAILABILITY_RELEASED, availabilityChoices) {
				s.SetMinimumAvailability(v)
			}
		})
	})
	group(c, "series", style.T(c, "requestsettings.series"), style.T(c, "requestsettings.seriesFooter"), func(g *settingsGroup) {
		choice(g, library, librariesOf(shows), none, s.GetSeriesLibraryId, s.SetSeriesLibraryId)
		choice(g, profile, profilesOf(series), none, s.GetSeriesQualityProfileId, s.SetSeriesQualityProfileId)
		toggle(g, style.T(c, "requestsettings.seasonFolder"), style.T(c, "requestsettings.seasonFolderDetail"), s.GetSeasonFolder, s.SetSeasonFolder)
		g.row(style.T(c, "requestsettings.monitorNew"), style.T(c, "requestsettings.monitorNewDetail"), nil, func() {
			v := s.GetMonitorNewItems()
			if chooseEnum(c, style.T(c, "requestsettings.monitorNew"), &v, acquisitionv1.NewItemMonitor_NEW_ITEM_MONITOR_ALL, monitorChoices) {
				s.SetMonitorNewItems(v)
			}
		})
		toggle(g, style.T(c, "requestsettings.specials"), style.T(c, "requestsettings.specialsDetail"), s.GetSpecials, s.SetSpecials)
	})
	group(c, "anime", style.T(c, "requestsettings.anime"), style.T(c, "requestsettings.animeFooter"), func(g *settingsGroup) {
		choice(g, library, librariesOf(shows), same, s.GetAnimeLibraryId, s.SetAnimeLibraryId)
		choice(g, profile, profilesOf(series), same, s.GetAnimeQualityProfileId, s.SetAnimeQualityProfileId)
	})
	group(c, "defaults", style.T(c, "requestsettings.defaults"), style.T(c, "requestsettings.defaultsFooter"), func(g *settingsGroup) {
		toggle(g, style.T(c, "users.requestMovies"), "", s.GetDefaultRequestMovies, s.SetDefaultRequestMovies)
		toggle(g, style.T(c, "users.autoApproveMovies"), "", s.GetDefaultAutoApproveMovies, s.SetDefaultAutoApproveMovies)
		toggle(g, style.T(c, "users.requestSeries"), "", s.GetDefaultRequestSeries, s.SetDefaultRequestSeries)
		toggle(g, style.T(c, "users.autoApproveSeries"), "", s.GetDefaultAutoApproveSeries, s.SetDefaultAutoApproveSeries)
		toggle(g, style.T(c, "users.reportIssues"), "", s.GetDefaultReportIssues, s.SetDefaultReportIssues)
		quota(g, style.T(c, "users.movieQuota"), s.GetMovieQuotaLimit, s.GetMovieQuotaDays, s.SetMovieQuotaLimit, s.SetMovieQuotaDays)
		quota(g, style.T(c, "users.seriesQuota"), s.GetSeriesQuotaLimit, s.GetSeriesQuotaDays, s.SetSeriesQuotaLimit, s.SetSeriesQuotaDays)
	})
	a.blocklistedTags(c, f, s)
	changed := !proto.Equal(s, f.loaded)
	a.acquisitionSave(c, style.T(c, "action.save"), changed, changed, func() {
		settings := proto.CloneOf(s)
		a.acquisitionAction("save", func(ctx context.Context, server string) error {
			_, err := a.requestService(server).UpdateRequestSettings(ctx, acquisitionv1.UpdateRequestSettingsRequest_builder{Settings: settings}.Build())
			return err
		}, func() {
			f.loaded, st.notice = nil, "acquisition.saved"
		})
	})
}

// blocklistedTags builds the blocklisted tags: keywords of a discovery
// provider, as "<plugin>/<keyword>", found by name in the first catalog
// filtering by keyword, and shown by their names.
func (a *App) blocklistedTags(c *ui.Context, f *requestsForm, s *acquisitionv1.RequestSettings) {
	colors := style.Colors(c)
	language := discoverLanguage(c)
	catalogs := adminTask(a, &f.catalogs, "catalogs", func(ctx context.Context, server string) ([]*acquisitionv1.Catalog, error) {
		res, err := a.discoverService(server).ListCatalogs(ctx, acquisitionv1.ListCatalogsRequest_builder{Language: &language}.Build())
		return res.GetCatalogs(), err
	}, language)
	keyword := pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD
	var catalog *acquisitionv1.Catalog
	for _, x := range catalogs.value {
		if catalog == nil && slices.Contains(x.GetFilters(), keyword) {
			catalog = x
		}
	}
	kind := acquisitionv1.MediaKind_MEDIA_KIND_MOVIE
	if k := catalog.GetMediaKinds(); len(k) > 0 {
		kind = k[0]
	}
	values := func(t *task[[]*pluginv1.FilterValue], name string, build func(b *acquisitionv1.ListFilterValuesRequest_builder)) *task[[]*pluginv1.FilterValue] {
		id := catalog.GetId()
		return adminTask(a, t, name, func(ctx context.Context, server string) ([]*pluginv1.FilterValue, error) {
			b := acquisitionv1.ListFilterValuesRequest_builder{Catalog: &id, Filter: &keyword, MediaKind: &kind, Language: &language}
			build(&b)
			res, err := a.discoverService(server).ListFilterValues(ctx, b.Build())
			return res.GetValues(), err
		})
	}
	tags := s.GetBlocklistedTags()
	prefix := catalog.GetPluginId() + "/"
	var ids []string
	for _, tag := range tags {
		if id, ok := strings.CutPrefix(tag, prefix); ok {
			ids = append(ids, id)
		}
	}
	var named []*pluginv1.FilterValue
	if catalog != nil && len(ids) > 0 {
		named = values(&f.named, "keyword-names/"+strings.Join(ids, ","), func(b *acquisitionv1.ListFilterValuesRequest_builder) { b.Ids = ids }).value
	}
	nameOf := func(tag string) string {
		if id, ok := strings.CutPrefix(tag, prefix); ok {
			for _, x := range slices.Concat(named, f.added) {
				if x.GetId() == id {
					return x.GetName()
				}
			}
		}
		return tag
	}
	group(c, "tags", style.T(c, "requestsettings.tags"), style.T(c, "requestsettings.tagsFooter"), func(g *settingsGroup) {
		if len(tags) > 0 {
			g.block(func() {
				ui.Row(c).Gap(8).Wrap().Children(func() {
					for _, tag := range tags {
						if a.toggleChip(c.Key(tag), nameOf(tag), "", true) {
							s.SetBlocklistedTags(slices.DeleteFunc(slices.Clone(tags), func(x string) bool { return x == tag }))
						}
					}
				})
			})
		}
		if catalog == nil {
			if catalogs.loaded {
				g.note(style.T(c, "requestsettings.noProvider"), false)
			}
		} else {
			g.block(func() {
				ui.Column(c).Gap(8).Children(func() {
					before := f.text
					ui.TextInput(c, &f.text).FillWidth().Label(style.T(c, "requestsettings.searchKeywords")).Placeholder(style.T(c, "requestsettings.searchKeywords"))
					if f.text != before {
						text := f.text
						time.AfterFunc(300*time.Millisecond, func() {
							a.opts.Update(func() {
								if f.text == text {
									f.query = strings.TrimSpace(text)
								}
							})
						})
					}
					if f.query == "" {
						return
					}
					query := f.query
					found := values(&f.found, "keywords/"+query, func(b *acquisitionv1.ListFilterValuesRequest_builder) { b.Query = &query })
					ui.Row(c).Gap(8).Wrap().Children(func() {
						n := 0
						for _, x := range found.value {
							if slices.Contains(tags, prefix+x.GetId()) || n == 20 {
								continue
							}
							n++
							if a.toggleChip(c.Key("found-"+x.GetId()), "+ "+x.GetName(), "", false) {
								f.added = append(f.added, x)
								s.SetBlocklistedTags(append(slices.Clone(tags), prefix+x.GetId()))
								f.text, f.query = "", ""
							}
						}
						if found.loaded && found.err == nil && len(found.value) == 0 {
							style.Text(ui.Text(c, style.T(c, "requestsettings.noKeywords")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
						}
					})
				})
			})
		}
		g.row(style.T(c, "requestsettings.tagsLimit"), style.T(c, "requestsettings.tagsLimitDetail"), nil, func() {
			v := float64(cmp.Or(s.GetBlocklistedTagsLimit(), 50))
			if ui.NumberInput(c, &v, 1, 250, 1).Width(128).Label(style.T(c, "requestsettings.tagsLimit")).Changed() {
				s.SetBlocklistedTagsLimit(int32(v))
			}
		})
	})
}

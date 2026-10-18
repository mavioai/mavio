package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// namedValue is a filter value chosen by its provider's ID, with its name
// to show.
type namedValue struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// catalogFilter is how a catalog's titles are filtered and sorted, kept
// in the catalog page's address as the web UI's; unset fields do not
// filter.
type catalogFilter struct {
	Sort             pluginv1.DiscoverSort `json:"sort,omitempty"`
	Ascending        bool                  `json:"ascending,omitempty"`
	Genres           []string              `json:"genres,omitempty"`
	Studios          []namedValue          `json:"studios,omitempty"`
	Networks         []namedValue          `json:"networks,omitempty"`
	Keywords         []namedValue          `json:"keywords,omitempty"`
	WatchProviders   []string              `json:"watchProviders,omitempty"`
	WatchRegion      string                `json:"watchRegion,omitempty"`
	OriginalLanguage string                `json:"originalLanguage,omitempty"`
	YearFrom         int32                 `json:"yearFrom,omitempty"`
	YearTo           int32                 `json:"yearTo,omitempty"`
	RuntimeMin       int32                 `json:"runtimeMin,omitempty"`
	RuntimeMax       int32                 `json:"runtimeMax,omitempty"`
	RatingMin        int32                 `json:"ratingMin,omitempty"`
	VotesMin         int32                 `json:"votesMin,omitempty"`
}

// namedOf returns references as named values.
func namedOf(refs []*pluginv1.Reference) []namedValue {
	var out []namedValue
	for _, r := range refs {
		out = append(out, namedValue{r.GetId(), r.GetName()})
	}
	return out
}

func ids(vs []namedValue) []string {
	var out []string
	for _, v := range vs {
		out = append(out, v.ID)
	}
	return out
}

// count returns how many filters are set, the sort aside.
func (f catalogFilter) count() int {
	n := 0
	for _, set := range []bool{
		len(f.Genres) > 0, len(f.Studios) > 0, len(f.Networks) > 0, len(f.Keywords) > 0, len(f.WatchProviders) > 0,
		f.OriginalLanguage != "", f.YearFrom > 0 || f.YearTo > 0, f.RuntimeMin > 0 || f.RuntimeMax > 0, f.RatingMin > 0, f.VotesMin > 0,
	} {
		if set {
			n++
		}
	}
	return n
}

// param returns the filter as the catalog page's address keeps it, empty
// when it neither filters nor sorts.
func (f catalogFilter) param() string {
	if f.count() == 0 && f.Sort == pluginv1.DiscoverSort_DISCOVER_SORT_UNSPECIFIED {
		return ""
	}
	if f.Sort == pluginv1.DiscoverSort_DISCOVER_SORT_UNSPECIFIED {
		f.Ascending = false
	}
	if len(f.WatchProviders) == 0 {
		f.WatchRegion = ""
	}
	b, _ := json.Marshal(f)
	return string(b)
}

// parseFilter returns the filter of a catalog page's address.
func parseFilter(s string) catalogFilter {
	var f catalogFilter
	if s != "" {
		_ = json.Unmarshal([]byte(s), &f)
	}
	return f
}

// request sets the fields of a discovery request that filter and sort.
func (f catalogFilter) request(b *acquisitionv1.DiscoverRequest_builder) {
	b.Sort, b.Ascending = &f.Sort, &f.Ascending
	b.Genres, b.Studios, b.Networks, b.Keywords = f.Genres, ids(f.Studios), ids(f.Networks), ids(f.Keywords)
	b.WatchProviders = f.WatchProviders
	if len(f.WatchProviders) > 0 {
		b.WatchRegion = &f.WatchRegion
	}
	b.OriginalLanguage = &f.OriginalLanguage
	if f.YearFrom > 0 {
		b.ReleasedFrom = new(fmt.Sprintf("%04d-01-01", f.YearFrom))
	}
	if f.YearTo > 0 {
		b.ReleasedTo = new(fmt.Sprintf("%04d-12-31", f.YearTo))
	}
	b.RuntimeMin, b.RuntimeMax, b.VotesMin = &f.RuntimeMin, &f.RuntimeMax, &f.VotesMin
	b.RatingMin = new(float64(f.RatingMin))
}

// catalogLocation returns the address of a catalog's page.
func catalogLocation(server, id, title string, kind acquisitionv1.MediaKind, tmdb int32, f catalogFilter) string {
	q := url.Values{}
	q.Set("title", title)
	if k := kindParam(kind); k != "" && kind != acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED {
		q.Set("kind", k)
	}
	if tmdb > 0 {
		q.Set("tmdb", strconv.Itoa(int(tmdb)))
	}
	if p := f.param(); p != "" {
		q.Set("f", p)
	}
	return "/catalog/" + server + "/" + id + "?" + q.Encode()
}

// alikePath returns the address of the popular titles of a kind filtered
// alike, empty without such a catalog.
func (a *App) alikePath(c *ui.Context, server string, kind acquisitionv1.MediaKind, f catalogFilter) string {
	_, catalogs, _ := a.discovery(c)
	for _, cat := range catalogs {
		if cat.GetKind() == pluginv1.CatalogKind_CATALOG_KIND_POPULAR && slices.Contains(cat.GetMediaKinds(), kind) {
			return catalogLocation(server, cat.GetId(), catalogRowTitle(c, catalogRow{cat, kind}), kind, 0, f)
		}
	}
	return ""
}

// filtersState is the state of a catalog's filters dialog.
type filtersState struct {
	open    bool
	draft   catalogFilter
	numbers map[string]*string // the text of the number fields, by name
	queries map[pluginv1.DiscoverFilter]*string
	settled map[pluginv1.DiscoverFilter]string // the queries once the typing paused
	values  map[string]*task[[]*pluginv1.FilterValue]

	languages map[string]*task[[]*systemv1.Language]
	countries map[string]*task[[]*systemv1.Country]
}

func (a *App) localizationService(id string) systemv1connect.LocalizationServiceClient {
	return systemv1connect.NewLocalizationServiceClient(a.connectHTTP(), serverURL(id))
}

// defaultRegion is the country of the streaming services until another is
// chosen, the one with the most of them.
const defaultRegion = "US"

// sortNames name the orders of discovered titles.
var sortNames = map[pluginv1.DiscoverSort]string{
	pluginv1.DiscoverSort_DISCOVER_SORT_UNSPECIFIED:  "discover.sortDefault",
	pluginv1.DiscoverSort_DISCOVER_SORT_POPULARITY:   "discover.sortPopularity",
	pluginv1.DiscoverSort_DISCOVER_SORT_RELEASE_DATE: "discover.sortReleaseDate",
	pluginv1.DiscoverSort_DISCOVER_SORT_RATING:       "discover.sortRating",
	pluginv1.DiscoverSort_DISCOVER_SORT_VOTE_COUNT:   "discover.sortVotes",
	pluginv1.DiscoverSort_DISCOVER_SORT_TITLE:        "discover.sortTitle",
}

// openFilters opens the filters dialog on a catalog's filter.
func (a *App) openFilters(f catalogFilter) {
	st := &a.discoverState.filters
	st.open, st.draft = true, f
	st.queries = map[pluginv1.DiscoverFilter]*string{}
	st.settled = map[pluginv1.DiscoverFilter]string{}
	st.numbers = map[string]*string{}
	for name, n := range map[string]int32{"yearFrom": f.YearFrom, "yearTo": f.YearTo, "runtimeMin": f.RuntimeMin, "runtimeMax": f.RuntimeMax, "votesMin": f.VotesMin} {
		s := ""
		if n > 0 {
			s = strconv.Itoa(int(n))
		}
		st.numbers[name] = &s
	}
}

// filterValues loads the values a catalog's filter takes, those matching
// query when given.
func (a *App) filterValues(c *ui.Context, server, catalog string, filter pluginv1.DiscoverFilter, kind acquisitionv1.MediaKind, query, region string) *task[[]*pluginv1.FilterValue] {
	st := &a.discoverState.filters
	language := discoverLanguage(c)
	k := key(server, catalog, filter, kind, query, region, language)
	return load(a, taskIn(&st.values, k), key("values", k, a.generation), func(ctx context.Context) ([]*pluginv1.FilterValue, error) {
		b := acquisitionv1.ListFilterValuesRequest_builder{Catalog: &catalog, Filter: &filter, MediaKind: &kind, Language: &language}
		if query != "" {
			b.Query = &query
		}
		if region != "" {
			b.Region = &region
		}
		res, err := a.discoverService(server).ListFilterValues(ctx, b.Build())
		return res.GetValues(), err
	})
}

// toggleChip builds a chip turned on or off, reporting a click.
func (a *App) toggleChip(c *ui.Context, label, image string, on bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Cursor(ui.CursorPointer).MinHeight(32).Padding(0, 12).Gap(6).Radius(design.RadiusFull).AlignItems(ui.Center).Label(label)
	switch {
	case on:
		b.Background(style.Color(colors.Accent))
	case b.Hovered():
		b.Background(style.Color(colors.FillSelected))
	default:
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		if bitmap := a.opts.Images.Get(image, 20, scale); bitmap != nil {
			ui.Box(c).Size(20, 20).Radius(4).Clip().Children(func() { ui.Image(c, bitmap).Fill().Fit(ui.Cover) })
		}
		tc := style.Color(colors.TextPrimary)
		if on {
			tc = style.Color(colors.OnAccent)
		}
		style.Text(ui.Text(c, label), style.Type(c).Footnote).FontWeight(600).TextColor(tc)
		if on && image == "" {
			ui.Icon(c, style.Icon("close", false)).Size(14, 14).TextColor(tc)
		}
	})
	return b.Clicked()
}

// filterSection builds a section of the filters dialog.
func filterSection(c *ui.Context, title string, fn func()) {
	colors := style.Colors(c)
	ui.Column(c.Key(title)).Gap(8).Children(func() {
		style.Text(ui.Text(c, title), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary))
		fn()
	})
}

// filtersDialog builds the dialog of a catalog's filters and sorts,
// offering those the catalog takes; its values are those of the catalog's
// kind of title, movies when it mixes them.
func (a *App) filtersDialog(c *ui.Context, server string, cat *acquisitionv1.Catalog, kind acquisitionv1.MediaKind, apply func(catalogFilter)) {
	st := &a.discoverState.filters
	if !st.open {
		return
	}
	f := &st.draft
	if kind == acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED {
		kind = acquisitionv1.MediaKind_MEDIA_KIND_MOVIE
	}
	takes := func(x pluginv1.DiscoverFilter) bool { return slices.Contains(cat.GetFilters(), x) }
	colors := style.Colors(c)
	ui.Modal(c, &st.open, func() {
		ui.Column(c).Width(640).MaxHeight(720).Gap(16).Children(func() {
			style.Text(ui.Text(c, style.T(c, "discover.filters")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Gap(20).Padding(0, 4, 0, 0).Children(func() {
					if sorts := cat.GetSorts(); len(sorts) > 0 {
						filterSection(c, style.T(c, "discover.sortBy"), func() {
							ui.Row(c).Gap(8).Children(func() {
								all := append([]pluginv1.DiscoverSort{pluginv1.DiscoverSort_DISCOVER_SORT_UNSPECIFIED}, sorts...)
								var labels []string
								for _, s := range all {
									labels = append(labels, style.T(c, sortNames[s]))
								}
								chosen := max(slices.Index(all, f.Sort), 0)
								if choose(c, style.T(c, "discover.sortBy"), &chosen, labels) {
									f.Sort = all[chosen]
								}
								if f.Sort != pluginv1.DiscoverSort_DISCOVER_SORT_UNSPECIFIED {
									order := 0
									if f.Ascending {
										order = 1
									}
									if choose(c, style.T(c, "discover.sortBy"), &order, []string{style.T(c, "discover.descending"), style.T(c, "discover.ascending")}) {
										f.Ascending = order == 1
									}
								}
							})
						})
					}
					if takes(pluginv1.DiscoverFilter_DISCOVER_FILTER_GENRE) {
						a.listedValues(c, server, cat.GetId(), pluginv1.DiscoverFilter_DISCOVER_FILTER_GENRE, kind, "", style.T(c, "discover.genres"), &f.Genres, nil)
					}
					if takes(pluginv1.DiscoverFilter_DISCOVER_FILTER_STUDIO) && kind == acquisitionv1.MediaKind_MEDIA_KIND_MOVIE {
						a.searchedValues(c, server, cat.GetId(), pluginv1.DiscoverFilter_DISCOVER_FILTER_STUDIO, kind, style.T(c, "discover.studios"), &f.Studios, true)
					}
					if kind == acquisitionv1.MediaKind_MEDIA_KIND_SERIES && len(f.Networks) > 0 {
						a.searchedValues(c, server, cat.GetId(), pluginv1.DiscoverFilter_DISCOVER_FILTER_NETWORK, kind, style.T(c, "discover.networks"), &f.Networks, false)
					}
					if takes(pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD) {
						a.searchedValues(c, server, cat.GetId(), pluginv1.DiscoverFilter_DISCOVER_FILTER_KEYWORD, kind, style.T(c, "discover.keywords"), &f.Keywords, true)
					}
					if takes(pluginv1.DiscoverFilter_DISCOVER_FILTER_WATCH_PROVIDER) {
						region := f.WatchRegion
						if region == "" {
							region = defaultRegion
						}
						a.listedValues(c, server, cat.GetId(), pluginv1.DiscoverFilter_DISCOVER_FILTER_WATCH_PROVIDER, kind, region, style.T(c, "discover.watchProviders"), &f.WatchProviders, func() {
							countries := a.countries(server)
							codes, labels := []string{}, []string{}
							for _, x := range countries {
								codes, labels = append(codes, x.GetCode()), append(labels, countryName(c, x.GetCode()))
							}
							if len(codes) == 0 {
								codes, labels = []string{region}, []string{countryName(c, region)}
							}
							chosen := max(slices.Index(codes, region), 0)
							if choose(c, style.T(c, "discover.region"), &chosen, labels) {
								f.WatchRegion, f.WatchProviders = codes[chosen], nil
							}
						})
						if len(f.WatchProviders) > 0 {
							f.WatchRegion = region
						}
					}
					if takes(pluginv1.DiscoverFilter_DISCOVER_FILTER_ORIGINAL_LANGUAGE) {
						filterSection(c, style.T(c, "discover.originalLanguage"), func() {
							codes, labels := []string{""}, []string{style.T(c, "discover.any")}
							for _, l := range a.languages(server) {
								if l.GetCode() != "" && !slices.Contains(codes, l.GetCode()) {
									codes, labels = append(codes, l.GetCode()), append(labels, languageName(c, l.GetCode()))
								}
							}
							if f.OriginalLanguage != "" && !slices.Contains(codes, f.OriginalLanguage) {
								codes, labels = append(codes, f.OriginalLanguage), append(labels, languageName(c, f.OriginalLanguage))
							}
							chosen := max(slices.Index(codes, f.OriginalLanguage), 0)
							if choose(c, style.T(c, "discover.originalLanguage"), &chosen, labels) {
								f.OriginalLanguage = codes[chosen]
							}
						})
					}
					if takes(pluginv1.DiscoverFilter_DISCOVER_FILTER_RELEASE_DATE) {
						filterSection(c, style.T(c, "discover.released"), func() {
							a.numberRange(c, "yearFrom", "yearTo", &f.YearFrom, &f.YearTo, 9999)
						})
					}
					if takes(pluginv1.DiscoverFilter_DISCOVER_FILTER_RUNTIME) {
						filterSection(c, style.T(c, "discover.runtime"), func() {
							a.numberRange(c, "runtimeMin", "runtimeMax", &f.RuntimeMin, &f.RuntimeMax, 1000)
						})
					}
					ui.Row(c).GapX(20).GapY(20).Wrap().Children(func() {
						if takes(pluginv1.DiscoverFilter_DISCOVER_FILTER_RATING) {
							filterSection(c, style.T(c, "discover.ratingMin"), func() {
								labels := []string{style.T(c, "discover.any")}
								for i := 1; i < 10; i++ {
									labels = append(labels, strconv.Itoa(i))
								}
								chosen := int(f.RatingMin)
								if choose(c, style.T(c, "discover.ratingMin"), &chosen, labels) {
									f.RatingMin = int32(chosen)
								}
							})
						}
						if takes(pluginv1.DiscoverFilter_DISCOVER_FILTER_VOTE_COUNT) {
							filterSection(c, style.T(c, "discover.votesMin"), func() {
								a.numberField(c, "votesMin", style.T(c, "discover.any"), &f.VotesMin, 1_000_000)
							})
						}
					})
				})
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if linkButton(c, style.T(c, "discover.reset")).Margin(0, 8, 0, 0).Clicked() {
					a.openFilters(catalogFilter{})
				}
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					st.open = false
				}
				if ui.PrimaryButton(c, style.T(c, "discover.apply")).Clicked() {
					st.open = false
					apply(st.draft)
				}
			})
		})
	})
}

// numberField builds a field of a whole number up to most, unset when
// empty.
func (a *App) numberField(c *ui.Context, name, placeholder string, n *int32, most int) {
	st := &a.discoverState.filters
	text := st.numbers[name]
	if text == nil {
		text = new("")
		st.numbers[name] = text
	}
	if ui.TextInput(c.Key(name), text).Placeholder(placeholder).Width(144).Label(placeholder).Changed() {
		v, err := strconv.Atoi(*text)
		if err != nil || v <= 0 || v > most {
			v = 0
		}
		*n = int32(v)
	}
}

// numberRange builds two number fields, from and to.
func (a *App) numberRange(c *ui.Context, fromName, toName string, from, to *int32, most int) {
	colors := style.Colors(c)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
		a.numberField(c, fromName, style.T(c, "discover.from"), from, most)
		style.Text(ui.Text(c, "–"), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary))
		a.numberField(c, toName, style.T(c, "discover.to"), to, most)
	})
}

// listedValues builds values listed whole, as genres and streaming
// services are, to pick from; control, when given, chooses the values
// listed, and the section shows with it even when none is listed.
func (a *App) listedValues(c *ui.Context, server, catalog string, filter pluginv1.DiscoverFilter, kind acquisitionv1.MediaKind, region, title string, value *[]string, control func()) {
	t := a.filterValues(c, server, catalog, filter, kind, "", region)
	if t.loaded && len(t.value) == 0 && control == nil {
		return
	}
	filterSection(c, title, func() {
		if control != nil {
			control()
		}
		list := func() {
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for _, v := range t.value {
					on := slices.Contains(*value, v.GetId())
					if a.toggleChip(c.Key(v.GetId()), v.GetName(), v.GetImageUrl(), on) {
						if on {
							*value = slices.DeleteFunc(slices.Clone(*value), func(x string) bool { return x == v.GetId() })
						} else {
							*value = append(slices.Clone(*value), v.GetId())
						}
					}
				}
			})
		}
		if len(t.value) > 30 {
			ui.Scroll(c).MaxHeight(160).Children(list)
		} else {
			list()
		}
	})
}

// searchedValues builds values found by name, as studios and keywords
// are: the chosen ones, a field, and what it finds; just the chosen ones
// unless searchable.
func (a *App) searchedValues(c *ui.Context, server, catalog string, filter pluginv1.DiscoverFilter, kind acquisitionv1.MediaKind, title string, value *[]namedValue, searchable bool) {
	st := &a.discoverState.filters
	text := st.queries[filter]
	if text == nil {
		text = new("")
		st.queries[filter] = text
	}
	filterSection(c, title, func() {
		if len(*value) > 0 {
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for _, v := range *value {
					if a.toggleChip(c.Key(v.ID), v.Name, "", true) {
						*value = slices.DeleteFunc(slices.Clone(*value), func(x namedValue) bool { return x.ID == v.ID })
					}
				}
			})
		}
		if !searchable {
			return
		}
		placeholder := style.T(c, "discover.searchValues", "filter", title)
		if ui.TextInput(c, text).Placeholder(placeholder).Label(placeholder).FillWidth().Changed() {
			typed := *text
			go func() {
				time.Sleep(300 * time.Millisecond)
				a.opts.Update(func() {
					if *text == typed {
						st.settled[filter] = strings.TrimSpace(typed)
					}
				})
			}()
		}
		query := st.settled[filter]
		if strings.TrimSpace(*text) == "" {
			query = ""
		}
		if query == "" {
			return
		}
		found := a.filterValues(c, server, catalog, filter, kind, query, "")
		ui.Scroll(c).MaxHeight(128).Children(func() {
			ui.Row(c).Gap(8).Wrap().Children(func() {
				n := 0
				for _, v := range found.value {
					if n == 20 || slices.ContainsFunc(*value, func(x namedValue) bool { return x.ID == v.GetId() }) {
						continue
					}
					n++
					if a.toggleChip(c.Key(v.GetId()), v.GetName(), "", false) {
						*value = append(slices.Clone(*value), namedValue{v.GetId(), v.GetName()})
						*text, st.settled[filter] = "", ""
					}
				}
			})
		})
	})
}

// languages returns the languages a server knows, loading them.
func (a *App) languages(server string) []*systemv1.Language {
	st := &a.discoverState.filters
	return load(a, taskIn(&st.languages, server), key("languages", server, a.generation), func(ctx context.Context) ([]*systemv1.Language, error) {
		res, err := a.localizationService(server).ListLanguages(ctx, &systemv1.ListLanguagesRequest{})
		return res.GetLanguages(), err
	}).value
}

// countries returns the countries a server knows, loading them.
func (a *App) countries(server string) []*systemv1.Country {
	st := &a.discoverState.filters
	return load(a, taskIn(&st.countries, server), key("countries", server, a.generation), func(ctx context.Context) ([]*systemv1.Country, error) {
		res, err := a.localizationService(server).ListCountries(ctx, &systemv1.ListCountriesRequest{})
		return res.GetCountries(), err
	}).value
}

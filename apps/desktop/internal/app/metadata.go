package app

import (
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// metadataState is the state of an item's metadata dialogs, as the web
// UI's (apps/web/src/screens/metadata.tsx).
type metadataState struct {
	busy bool
	err  error
	// before and form are the edit form's fields as the item had them and
	// as edited.
	before, form metadataForm
	idKinds      task[[]*libraryv1.ExternalIdKind]
	// name and year are the identify search's fields; query is what was
	// searched.
	name, year string
	query      string
	results    task[[]*libraryv1.RemoteSearchResult]
	replace    bool
	// imageKind is the kind of images shown, an index of imageKinds;
	// deleting is the URL of the image asked about deleting.
	imageKind int
	remote    task[[]*libraryv1.RemoteImage]
	deleting  string
}

// metadataForm is the edit form's fields: text by ItemMetadata's field
// name, external IDs, and locks.
type metadataForm struct {
	text         map[string]string
	status       int
	externalIDs  map[string]string
	locked       bool
	lockedFields []libraryv1.MetadataField
}

func (a *App) metadataService(id string) libraryv1connect.MetadataServiceClient {
	return libraryv1connect.NewMetadataServiceClient(a.connectHTTP(), serverURL(id))
}

// identifiable reports whether the providers identify an item of a kind.
func identifiable(kind libraryv1.ItemKind) bool {
	switch kind {
	case libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_SERIES, libraryv1.ItemKind_ITEM_KIND_MUSIC_ARTIST,
		libraryv1.ItemKind_ITEM_KIND_MUSIC_ALBUM, libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO, libraryv1.ItemKind_ITEM_KIND_BOOK,
		libraryv1.ItemKind_ITEM_KIND_AUDIOBOOK:
		return true
	}
	return false
}

// metadataDone ends a change of the item's metadata: the dialog closes
// as it went, and what shows the item loads again.
func (a *App) metadataDone(m *moreState, err error, close bool) {
	m.metadata.busy, m.metadata.err = false, err
	if err != nil {
		return
	}
	if close {
		m.open = false
	}
	a.generation++
}

// dayOf returns a time's day as "2006-01-02", empty for none.
func dayOf(ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return ts.AsTime().UTC().Format(time.DateOnly)
}

// numberOf returns a number as text, empty for 0.
func numberOf[T int32 | float64](n T) string {
	if n == 0 {
		return ""
	}
	return fmt.Sprint(n)
}

func formOf(it *libraryv1.Item) metadataForm {
	runtime := ""
	if it.HasRuntime() {
		runtime = strconv.Itoa(int(it.GetRuntime().AsDuration().Round(time.Minute).Minutes()))
	}
	list := func(v []string) string { return strings.Join(v, ", ") }
	return metadataForm{
		text: map[string]string{
			"name": it.GetName(), "original_title": it.GetOriginalTitle(), "sort_name": it.GetSortName(),
			"overview": it.GetOverview(), "tagline": it.GetTagline(), "production_year": numberOf(it.GetProductionYear()),
			"premiere_date": dayOf(it.GetPremiereDate()), "end_date": dayOf(it.GetEndDate()), "runtime": runtime,
			"official_rating": it.GetOfficialRating(), "custom_rating": it.GetCustomRating(),
			"community_rating": numberOf(it.GetCommunityRating()), "critic_rating": numberOf(it.GetCriticRating()),
			"genres": list(it.GetGenres()), "tags": list(it.GetTags()), "studios": list(it.GetStudios()),
			"production_locations": list(it.GetProductionLocations()), "artists": list(it.GetArtists()),
			"album_artists": list(it.GetAlbumArtists()), "index_number": numberOf(it.GetIndexNumber()),
			"parent_index_number": numberOf(it.GetParentIndexNumber()), "absolute_number": numberOf(it.GetAbsoluteNumber()),
			"metadata_language": it.GetMetadataLanguage(), "metadata_country": it.GetMetadataCountry(),
		},
		status:       int(it.GetSeriesStatus()),
		externalIDs:  maps.Clone(it.GetExternalIds()),
		locked:       it.GetLocked(),
		lockedFields: slices.Clone(it.GetLockedFields()),
	}
}

// unlist splits text separated by commas.
func unlist(v string) []string {
	var out []string
	for s := range strings.SplitSeq(v, ",") {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// changesOf returns the form's changed fields as ItemMetadata and its
// update mask's paths; bad is a field whose text is not a valid value.
func changesOf(before, after metadataForm) (md *libraryv1.ItemMetadata, paths []string, bad string) {
	t := after.text
	for _, k := range slices.Sorted(maps.Keys(t)) {
		if t[k] != before.text[k] {
			paths = append(paths, k)
		}
	}
	if after.status != before.status {
		paths = append(paths, "series_status")
	}
	ids := map[string]string{}
	for k, v := range after.externalIDs {
		if v = strings.TrimSpace(v); v != "" || before.externalIDs[k] != "" {
			ids[k] = v
		}
	}
	if !maps.Equal(ids, before.externalIDs) {
		paths = append(paths, "external_ids")
	}
	if after.locked != before.locked {
		paths = append(paths, "locked")
	}
	if !slices.Equal(after.lockedFields, before.lockedFields) {
		paths = append(paths, "locked_fields")
	}
	number := func(k string, lo, hi float64) (float64, bool) {
		s := strings.TrimSpace(t[k])
		if s == "" {
			return 0, true
		}
		n, err := strconv.ParseFloat(s, 64)
		if err != nil || n < lo || n > hi {
			bad = k
			return 0, false
		}
		return n, true
	}
	day := func(k string) *timestamppb.Timestamp {
		s := strings.TrimSpace(t[k])
		if s == "" {
			return nil
		}
		d, err := time.Parse(time.DateOnly, s)
		if err != nil {
			bad = k
			return nil
		}
		return timestamppb.New(d)
	}
	b := libraryv1.ItemMetadata_builder{
		Name: new(strings.TrimSpace(t["name"])), OriginalTitle: new(strings.TrimSpace(t["original_title"])),
		SortName: new(strings.TrimSpace(t["sort_name"])), Overview: new(strings.TrimSpace(t["overview"])),
		Tagline: new(strings.TrimSpace(t["tagline"])), PremiereDate: day("premiere_date"), EndDate: day("end_date"),
		OfficialRating: new(strings.TrimSpace(t["official_rating"])), CustomRating: new(strings.TrimSpace(t["custom_rating"])),
		Genres: unlist(t["genres"]), Tags: unlist(t["tags"]), Studios: unlist(t["studios"]),
		ProductionLocations: unlist(t["production_locations"]), Artists: unlist(t["artists"]), AlbumArtists: unlist(t["album_artists"]),
		SeriesStatus: new(libraryv1.SeriesStatus(after.status)), ExternalIds: ids,
		Locked: new(after.locked), LockedFields: after.lockedFields,
		MetadataLanguage: new(t["metadata_language"]), MetadataCountry: new(t["metadata_country"]),
	}
	if n, ok := number("production_year", 0, 9999); ok {
		b.ProductionYear = new(int32(n))
	}
	if n, ok := number("runtime", 0, 100000); ok && strings.TrimSpace(t["runtime"]) != "" {
		b.Runtime = durationpb.New(time.Duration(n * float64(time.Minute)))
	}
	if n, ok := number("community_rating", 0, 10); ok {
		b.CommunityRating = new(n)
	}
	if n, ok := number("critic_rating", 0, 100); ok {
		b.CriticRating = new(n)
	}
	for k, f := range map[string]**int32{"index_number": &b.IndexNumber, "parent_index_number": &b.ParentIndexNumber, "absolute_number": &b.AbsoluteNumber} {
		if n, ok := number(k, 0, 1e6); ok && strings.TrimSpace(t[k]) != "" {
			*f = new(int32(n))
		}
	}
	return b.Build(), paths, bad
}

// lockable are the fields locked one by one, with their labels' keys.
var lockable = []struct {
	field libraryv1.MetadataField
	label string
}{
	{libraryv1.MetadataField_METADATA_FIELD_NAME, "metadata.lockName"},
	{libraryv1.MetadataField_METADATA_FIELD_OVERVIEW, "metadata.overview"},
	{libraryv1.MetadataField_METADATA_FIELD_GENRES, "metadata.genres"},
	{libraryv1.MetadataField_METADATA_FIELD_TAGS, "metadata.tags"},
	{libraryv1.MetadataField_METADATA_FIELD_STUDIOS, "metadata.studios"},
	{libraryv1.MetadataField_METADATA_FIELD_PRODUCTION_LOCATIONS, "metadata.locations"},
	{libraryv1.MetadataField_METADATA_FIELD_OFFICIAL_RATING, "metadata.officialRating"},
	{libraryv1.MetadataField_METADATA_FIELD_RUNTIME, "metadata.runtime"},
	{libraryv1.MetadataField_METADATA_FIELD_CAST, "metadata.cast"},
}

// metadataField builds a labelled field of the edit form: half the
// form's width, or all of it when wide.
func metadataField(c *ui.Context, label string, wide bool, field func() ui.Element) {
	colors := style.Colors(c)
	col := ui.Column(c.Key(label)).Gap(4).MinWidth(0)
	if wide {
		col.WidthPercent(100)
	} else {
		col.Width(300)
	}
	col.Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
		field().FillWidth().Label(label)
	})
}

// switchRow builds a toggle with its label before it.
func switchRow(c *ui.Context, label string, on *bool, width float32) bool {
	changed := false
	ui.Row(c.Key(label)).Width(width).Gap(12).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Callout).TextColor(style.Color(style.Colors(c).TextPrimary))
		changed = ui.Switch(c, on).Label(label).Changed()
	})
	return changed
}

// metadataChoice builds the edit form's select of a field's code among
// those choices lists, the first following the library.
func (a *App) metadataChoice(c *ui.Context, server string, f *metadataForm, k, label string,
	choices func(c *ui.Context, server, none, current string) ([]string, []string),
) {
	metadataField(c, label, false, func() ui.Element {
		codes, labels := choices(c, server, style.T(c, "metadata.libraryDefault"), f.text[k])
		chosen := labels[max(slices.Index(codes, f.text[k]), 0)]
		s := selectOf(c, &chosen, labels)
		if s.Changed() {
			f.text[k] = codes[slices.Index(labels, chosen)]
		}
		return s
	})
}

// editDialog builds the dialog editing the item's fields and locks,
// which saves only those changed.
func (a *App) editDialog(c *ui.Context, m *moreState, server string, it *libraryv1.Item) {
	colors := style.Colors(c)
	md := &m.metadata
	f := &md.form
	kind := it.GetKind()
	kinds := load(a, &md.idKinds, key("idKinds", server, kind), func(ctx context.Context) ([]*libraryv1.ExternalIdKind, error) {
		res, err := a.metadataService(server).ListExternalIdKinds(ctx, libraryv1.ListExternalIdKindsRequest_builder{ItemKind: &kind}.Build())
		return res.GetKinds(), err
	})
	video := kind == libraryv1.ItemKind_ITEM_KIND_MOVIE || kind == libraryv1.ItemKind_ITEM_KIND_EPISODE ||
		kind == libraryv1.ItemKind_ITEM_KIND_VIDEO || kind == libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO
	music := kind == libraryv1.ItemKind_ITEM_KIND_MUSIC_ALBUM || kind == libraryv1.ItemKind_ITEM_KIND_TRACK || kind == libraryv1.ItemKind_ITEM_KIND_MUSIC_VIDEO
	series := kind == libraryv1.ItemKind_ITEM_KIND_SERIES
	episode := kind == libraryv1.ItemKind_ITEM_KIND_EPISODE
	metadata, paths, bad := changesOf(md.before, *f)
	save := func() {
		if len(paths) == 0 {
			m.open = false
			return
		}
		id := it.GetId()
		md.busy, md.err = true, nil
		go func() {
			_, err := a.metadataService(server).UpdateItem(a.ctx, libraryv1.UpdateItemRequest_builder{
				Id: &id, Metadata: metadata, UpdateMask: &fieldmaskpb.FieldMask{Paths: paths},
			}.Build())
			a.opts.Update(func() { a.metadataDone(m, err, true) })
		}()
	}
	text := func(k, label string, wide bool, placeholder string) {
		metadataField(c, label, wide, func() ui.Element {
			v := f.text[k]
			in := ui.TextInput(c, &v).Placeholder(placeholder)
			if in.Changed() {
				f.text[k] = v
			}
			return in
		})
	}
	commas := style.T(c, "metadata.commas")
	ui.Modal(c, &m.open, func() {
		ui.Column(c).Width(620).MaxHeight(720).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "metadata.editTitle", "name", it.GetName())), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Gap(12).Padding(0, 4, 4, 0).Children(func() {
					ui.Row(c).Wrap().GapX(16).GapY(12).Children(func() {
						text("name", style.T(c, "metadata.name"), true, "")
						text("original_title", style.T(c, "metadata.originalTitle"), false, "")
						text("sort_name", style.T(c, "metadata.sortName"), false, f.text["name"])
						metadataField(c, style.T(c, "metadata.overview"), true, func() ui.Element {
							v := f.text["overview"]
							in := ui.TextArea(c, &v).Height(112)
							if in.Changed() {
								f.text["overview"] = v
							}
							return in
						})
						if kind == libraryv1.ItemKind_ITEM_KIND_MOVIE || series {
							text("tagline", style.T(c, "metadata.tagline"), true, "")
						}
						if kind == libraryv1.ItemKind_ITEM_KIND_SEASON {
							text("index_number", style.T(c, "metadata.seasonNumber"), false, "")
						}
						if episode {
							text("parent_index_number", style.T(c, "metadata.seasonNumber"), false, "")
						}
						switch kind {
						case libraryv1.ItemKind_ITEM_KIND_EPISODE:
							text("index_number", style.T(c, "metadata.episodeNumber"), false, "")
							text("absolute_number", style.T(c, "metadata.absoluteNumber"), false, "")
						case libraryv1.ItemKind_ITEM_KIND_TRACK:
							text("index_number", style.T(c, "metadata.trackNumber"), false, "")
						}
						text("production_year", style.T(c, "metadata.year"), false, "")
						premiere := style.T(c, "metadata.releaseDate")
						if series {
							premiere = style.T(c, "metadata.firstAired")
						}
						text("premiere_date", premiere, false, "YYYY-MM-DD")
						if series {
							text("end_date", style.T(c, "metadata.endDate"), false, "YYYY-MM-DD")
							metadataField(c, style.T(c, "metadata.status"), false, func() ui.Element {
								labels := []string{
									style.T(c, "metadata.statusUnknown"), style.T(c, "metadata.continuing"),
									style.T(c, "metadata.ended"), style.T(c, "metadata.unreleased"),
								}
								chosen := labels[min(max(f.status, 0), len(labels)-1)]
								s := selectOf(c, &chosen, labels)
								if s.Changed() {
									f.status = slices.Index(labels, chosen)
								}
								return s
							})
						}
						if video {
							text("runtime", style.T(c, "metadata.runtimeMinutes"), false, "")
						}
						text("official_rating", style.T(c, "metadata.officialRating"), false, "")
						text("custom_rating", style.T(c, "metadata.customRating"), false, "")
						text("community_rating", style.T(c, "metadata.communityRating"), false, "")
						text("critic_rating", style.T(c, "metadata.criticRating"), false, "")
						if music {
							text("artists", style.T(c, "metadata.artists"), true, commas)
						}
						if kind == libraryv1.ItemKind_ITEM_KIND_MUSIC_ALBUM {
							text("album_artists", style.T(c, "metadata.albumArtists"), true, commas)
						}
						text("genres", style.T(c, "metadata.genres"), true, commas)
						text("tags", style.T(c, "metadata.tags"), true, commas)
						text("studios", style.T(c, "metadata.studios"), true, commas)
						text("production_locations", style.T(c, "metadata.locations"), true, commas)
						if identifiable(kind) {
							a.metadataChoice(c, server, f, "metadata_language", style.T(c, "libraries.language"), a.languageChoices)
							a.metadataChoice(c, server, f, "metadata_country", style.T(c, "libraries.country"), a.countryChoices)
						}
					})
					if len(kinds.value) > 0 {
						style.Text(ui.Text(c, style.T(c, "metadata.externalIds")), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).Margin(8, 0, 0)
						ui.Row(c).Wrap().GapX(16).GapY(12).Children(func() {
							for _, x := range kinds.value {
								metadataField(c, x.GetName(), false, func() ui.Element {
									v := f.externalIDs[x.GetKey()]
									in := ui.TextInput(c, &v)
									if in.Changed() {
										if f.externalIDs == nil {
											f.externalIDs = map[string]string{}
										}
										f.externalIDs[x.GetKey()] = v
									}
									return in
								})
							}
						})
					}
					style.Text(ui.Text(c, style.T(c, "metadata.locks")), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).Margin(8, 0, 0)
					switchRow(c, style.T(c, "metadata.lockAll"), &f.locked, 612)
					if !f.locked {
						ui.Row(c).Wrap().GapX(16).GapY(8).Children(func() {
							for _, l := range lockable {
								on := slices.Contains(f.lockedFields, l.field)
								if switchRow(c, style.T(c, l.label), &on, 300) {
									if on {
										f.lockedFields = append(f.lockedFields, l.field)
									} else {
										f.lockedFields = slices.DeleteFunc(slices.Clone(f.lockedFields), func(x libraryv1.MetadataField) bool { return x == l.field })
									}
								}
							}
						})
					}
					style.Text(ui.Text(c, style.T(c, "metadata.locksFooter")), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
				})
			})
			if md.err != nil {
				style.Text(ui.Text(c, errorMessage(c, md.err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					m.open = false
				}
				label := style.T(c, "metadata.save")
				if md.busy {
					label = "…"
				}
				if ui.PrimaryButton(c, label).Disabled(md.busy || strings.TrimSpace(f.text["name"]) == "" || bad != "").Clicked() {
					save()
				}
			})
		})
	})
}

// identifyDialog builds the dialog searching the providers for what the
// item is, and identifying it as a result.
func (a *App) identifyDialog(c *ui.Context, m *moreState, server string, it *libraryv1.Item) {
	colors := style.Colors(c)
	md := &m.metadata
	results := load(a, &md.results, key("identify", server, it.GetId(), md.query), func(ctx context.Context) ([]*libraryv1.RemoteSearchResult, error) {
		name, year, _ := strings.Cut(md.query, "\x00")
		b := libraryv1.SearchRemoteRequest_builder{ItemId: new(it.GetId()), Name: &name}
		if y, err := strconv.Atoi(year); err == nil && y > 0 {
			b.Year = new(int32(y))
		}
		res, err := a.metadataService(server).SearchRemote(ctx, b.Build())
		return res.GetResults(), err
	})
	search := func() {
		md.query = strings.TrimSpace(md.name) + "\x00" + strings.TrimSpace(md.year)
	}
	ui.Modal(c, &m.open, func() {
		ui.Column(c).Width(620).MaxHeight(680).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "metadata.identifyTitle", "name", it.GetName())), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				name := ui.TextInput(c, &md.name).Grow(1).Label(style.T(c, "metadata.name")).Placeholder(style.T(c, "metadata.name"))
				year := ui.TextInput(c, &md.year).Width(120).Label(style.T(c, "metadata.year")).Placeholder(style.T(c, "metadata.year"))
				ok := strings.TrimSpace(md.name) != ""
				if secondaryButton(c, "", style.T(c, "metadata.search")).Disabled(!ok).Clicked() || (name.Submitted() || year.Submitted()) && ok {
					search()
				}
			})
			ui.Scroll(c).Grow(1).Shrink(1).MinHeight(120).Children(func() {
				ui.Column(c).Children(func() {
					note := func(s string, color ui.Color) {
						style.Text(ui.Text(c, s), style.Type(c).Callout).TextColor(color).Padding(12, 0)
					}
					switch {
					case results.loading:
						note(style.T(c, "metadata.searching"), style.Color(colors.TextSecondary))
						return
					case results.err != nil:
						note(errorMessage(c, results.err), style.Color(colors.Danger))
						return
					case results.loaded && len(results.value) == 0:
						note(style.T(c, "metadata.noResults"), style.Color(colors.TextSecondary))
						return
					}
					for i, r := range results.value {
						a.resultRow(c, i, r, md.busy, func() {
							ids, id := r.GetExternalIds(), it.GetId()
							md.busy, md.err = true, nil
							go func() {
								_, err := a.metadataService(server).IdentifyItem(a.ctx, libraryv1.IdentifyItemRequest_builder{ItemId: &id, ExternalIds: ids}.Build())
								a.opts.Update(func() { a.metadataDone(m, err, true) })
							}()
						})
					}
				})
			})
			if md.err != nil {
				style.Text(ui.Text(c, errorMessage(c, md.err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
			ui.Row(c).Justify(ui.End).Children(func() {
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					m.open = false
				}
			})
		})
	})
}

// resultRow builds a provider's search result with its poster, its IDs
// and its overview, and the button identifying the item as it.
func (a *App) resultRow(c *ui.Context, i int, r *libraryv1.RemoteSearchResult, busy bool, identify func()) {
	colors := style.Colors(c)
	ui.Row(c.Key(i)).Gap(12).Padding(12, 0).AlignItems(ui.Start).BorderWidth(1, 0, 0, 0).BorderColor(style.Color(colors.Border)).Children(func() {
		ui.Box(c).Size(56, 84).Shrink(0).Radius(6).Clip().Background(style.Color(colors.Fill)).Children(func() {
			if bitmap := a.opts.Images.Get(r.GetImageUrl(), 56, scale); bitmap != nil {
				ui.Image(c, bitmap).Fill().Fit(ui.Cover)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			name := r.GetName()
			if r.GetYear() > 0 {
				name += fmt.Sprintf(" (%d)", r.GetYear())
			}
			style.Text(ui.Text(c, name), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary))
			facts := []string{r.GetProvider()}
			for _, k := range slices.Sorted(maps.Keys(r.GetExternalIds())) {
				facts = append(facts, k+" "+r.GetExternalIds()[k])
			}
			style.Text(ui.Text(c, strings.Join(facts, " · ")), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
			if r.GetOverview() != "" {
				style.Text(ui.Text(c, r.GetOverview()), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).MaxLines(2)
			}
		})
		if secondaryButton(c, "", style.T(c, "metadata.identify")).Disabled(busy).Clicked() {
			identify()
		}
	})
}

// refreshDialog builds the dialog refreshing the item's metadata, filling
// what is missing or replacing all of it.
func (a *App) refreshDialog(c *ui.Context, m *moreState, server string, it *libraryv1.Item) {
	colors := style.Colors(c)
	md := &m.metadata
	ui.Modal(c, &m.open, func() {
		ui.Column(c).Width(440).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "metadata.refreshTitle", "name", it.GetName())), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			switchRow(c, style.T(c, "metadata.replace"), &md.replace, 400)
			footer := style.T(c, "metadata.refreshFooter")
			if md.replace {
				footer = style.T(c, "metadata.replaceFooter")
			}
			style.Text(ui.Text(c, footer), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
			if md.err != nil {
				style.Text(ui.Text(c, errorMessage(c, md.err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					m.open = false
				}
				label := style.T(c, "metadata.refresh")
				if md.busy {
					label = style.T(c, "metadata.refreshing")
				}
				if ui.PrimaryButton(c, label).Disabled(md.busy).Clicked() {
					id, replace := it.GetId(), md.replace
					md.busy, md.err = true, nil
					go func() {
						_, err := a.metadataService(server).RefreshItem(a.ctx, libraryv1.RefreshItemRequest_builder{Id: &id, ReplaceMetadata: &replace}.Build())
						a.opts.Update(func() { a.metadataDone(m, err, true) })
					}()
				}
			})
		})
	})
}

// imageKinds are the kinds of images chosen, with their labels' keys.
var imageKinds = []struct {
	kind  libraryv1.ImageKind
	label string
}{
	{libraryv1.ImageKind_IMAGE_KIND_PRIMARY, "metadata.primary"},
	{libraryv1.ImageKind_IMAGE_KIND_BACKDROP, "metadata.backdrop"},
	{libraryv1.ImageKind_IMAGE_KIND_LOGO, "metadata.logo"},
	{libraryv1.ImageKind_IMAGE_KIND_THUMB, "metadata.thumb"},
	{libraryv1.ImageKind_IMAGE_KIND_BANNER, "metadata.banner"},
	{libraryv1.ImageKind_IMAGE_KIND_ART, "metadata.art"},
	{libraryv1.ImageKind_IMAGE_KIND_DISC, "metadata.disc"},
}

// setImage sets the item's image of a kind from a URL or from data.
func (a *App) setImage(m *moreState, server, item string, kind libraryv1.ImageKind, url string, data []byte) {
	md := &m.metadata
	md.busy, md.err = true, nil
	go func() {
		b := libraryv1.SetItemImageRequest_builder{ItemId: &item, Kind: &kind}
		if data != nil {
			b.Data = data
		} else {
			b.Url = &url
		}
		_, err := a.metadataService(server).SetItemImage(a.ctx, b.Build())
		a.opts.Update(func() { a.metadataDone(m, err, false) })
	}()
}

// imagesDialog builds the dialog of the item's images of a kind: deleted,
// replaced from the providers' or by a file.
func (a *App) imagesDialog(c *ui.Context, m *moreState, server string, it *libraryv1.Item) {
	colors := style.Colors(c)
	md := &m.metadata
	kind := imageKinds[md.imageKind].kind
	remote := load(a, &md.remote, key("images", server, it.GetId(), kind), func(ctx context.Context) ([]*libraryv1.RemoteImage, error) {
		res, err := a.metadataService(server).ListRemoteImages(ctx, libraryv1.ListRemoteImagesRequest_builder{ItemId: new(it.GetId()), Kind: &kind}.Build())
		return res.GetImages(), err
	})
	wide := kind != libraryv1.ImageKind_IMAGE_KIND_PRIMARY && kind != libraryv1.ImageKind_IMAGE_KIND_DISC
	width, height := float32(130), float32(195)
	if wide {
		width, height = 280, 158
	}
	var current []*libraryv1.Image
	for _, i := range it.GetImages() {
		if i.GetKind() == kind && i.GetUrl() != "" {
			current = append(current, i)
		}
	}
	upload := func() {
		go func() {
			path, err := a.opts.ChooseFile(style.T(c, "metadata.imageKind"), []string{"jpg", "jpeg", "png", "webp", "gif"})
			var data []byte
			if err == nil && path != "" {
				data, err = os.ReadFile(path)
			}
			a.opts.Update(func() {
				switch {
				case err != nil:
					md.err = err
				case data != nil:
					a.setImage(m, server, it.GetId(), kind, "", data)
				}
			})
		}()
	}
	ui.Modal(c, &m.open, func() {
		ui.Column(c).Width(620).MaxHeight(720).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "metadata.imagesTitle", "name", it.GetName())), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				labels := make([]string, len(imageKinds))
				for i, k := range imageKinds {
					labels[i] = style.T(c, k.label)
				}
				chosen := labels[md.imageKind]
				if selectOf(c, &chosen, labels).Label(style.T(c, "metadata.imageKind")).Changed() {
					md.imageKind, md.deleting = slices.Index(labels, chosen), ""
				}
				if a.opts.ChooseFile != nil && secondaryButton(c, "", style.T(c, "metadata.uploadImage")).Disabled(md.busy).Clicked() {
					upload()
				}
				if md.busy {
					style.Text(ui.Text(c, "…"), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
				}
			})
			if md.err != nil {
				style.Text(ui.Text(c, errorMessage(c, md.err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
			heading := func(s string) {
				style.Text(ui.Text(c, s), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary))
			}
			secondary := func(s string, color ui.Color) {
				style.Text(ui.Text(c, s), style.Type(c).Callout).TextColor(color)
			}
			ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Gap(16).Padding(0, 4, 4, 0).Children(func() {
					ui.Column(c).Gap(8).Children(func() {
						heading(style.T(c, "metadata.current"))
						if len(current) == 0 {
							secondary(style.T(c, "metadata.noImage"), style.Color(colors.TextSecondary))
						}
						ui.Row(c).Wrap().Gap(12).Children(func() {
							for _, i := range current {
								a.currentImage(c, m, server, it.GetId(), i, width, height)
							}
						})
					})
					ui.Column(c).Gap(8).Children(func() {
						heading(style.T(c, "metadata.fromProviders"))
						switch {
						case remote.loading:
							secondary(style.T(c, "metadata.searching"), style.Color(colors.TextSecondary))
							return
						case remote.err != nil:
							secondary(errorMessage(c, remote.err), style.Color(colors.Danger))
							return
						case remote.loaded && len(remote.value) == 0:
							secondary(style.T(c, "metadata.noRemoteImages"), style.Color(colors.TextSecondary))
							return
						}
						ui.Row(c).Wrap().Gap(12).Children(func() {
							for _, r := range remote.value {
								b := ui.ButtonBase(c.Key(r.GetUrl())).Column().Width(width).Gap(4).Label(r.GetProvider()).Cursor(ui.CursorPointer).Disabled(md.busy)
								b.Children(func() {
									art := ui.Box(c).Size(width, height).Radius(6).Clip().Background(style.Color(colors.Fill))
									if b.Hovered() || a.tvFocus(c, b, false) {
										art.Border(2, style.Color(colors.Accent))
									}
									art.Children(func() {
										if bitmap := a.opts.Images.Get(r.GetUrl(), width, scale); bitmap != nil {
											ui.Image(c, bitmap).Fill().Fit(ui.Cover)
										}
									})
									var facts []string
									for _, f := range []string{r.GetProvider(), dimensions(r.GetWidth(), r.GetHeight()), r.GetLanguage()} {
										if f != "" {
											facts = append(facts, f)
										}
									}
									style.Text(ui.Text(c, strings.Join(facts, " · ")), style.Type(c).Caption).TextColor(style.Color(colors.TextSecondary))
								})
								if b.Clicked() {
									a.setImage(m, server, it.GetId(), kind, r.GetUrl(), nil)
								}
							}
						})
					})
				})
			})
			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.PrimaryButton(c, style.T(c, "keys.done")).Clicked() {
					m.open = false
				}
			})
		})
	})
}

// dimensions returns an image's size as "1920×1080", empty when unknown.
func dimensions(w, h int32) string {
	if w == 0 || h == 0 {
		return ""
	}
	return fmt.Sprintf("%d×%d", w, h)
}

// currentImage builds one of the item's images, deleted after asking.
func (a *App) currentImage(c *ui.Context, m *moreState, server, item string, i *libraryv1.Image, width, height float32) {
	colors := style.Colors(c)
	md := &m.metadata
	link := func(label string, color ui.Color) bool {
		b := ui.ButtonBase(c.Key(label)).Label(label).Cursor(ui.CursorPointer)
		b.Children(func() {
			style.Text(ui.Text(c, label), style.Type(c).Footnote).FontWeight(600).TextColor(color)
		})
		return b.Clicked()
	}
	ui.Column(c.Key(i.GetId())).Width(width).Gap(4).Children(func() {
		ui.Box(c).Size(width, height).Radius(6).Clip().Background(style.Color(colors.Fill)).Children(func() {
			if bitmap := a.opts.Images.Get(a.serverURLOf(server, i.GetUrl()), width, scale); bitmap != nil {
				ui.Image(c, bitmap).Fill().Fit(ui.Cover)
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			style.Text(ui.Text(c, dimensions(i.GetWidth(), i.GetHeight())), style.Type(c).Caption).TextColor(style.Color(colors.TextSecondary))
			if md.deleting != i.GetUrl() {
				if link(style.T(c, "metadata.deleteImage"), style.Color(colors.Danger)) {
					md.deleting = i.GetUrl()
				}
				return
			}
			ui.Row(c).Gap(12).Children(func() {
				if link(style.T(c, "action.cancel"), style.Color(colors.TextPrimary)) {
					md.deleting = ""
				}
				if link(style.T(c, "metadata.confirmDelete"), style.Color(colors.Danger)) && !md.busy {
					id := i.GetId()
					md.busy, md.err, md.deleting = true, nil, ""
					go func() {
						_, err := a.metadataService(server).DeleteItemImage(a.ctx, libraryv1.DeleteItemImageRequest_builder{ItemId: &item, ImageId: &id}.Build())
						a.opts.Update(func() { a.metadataDone(m, err, false) })
					}()
				}
			})
		})
	})
}

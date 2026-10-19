package app

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// itemInfo is what the details show of the server's own item beyond the
// core's details: its credits by job, facts and files, its extras, and
// the collection it belongs to, and the series of an episode or season.
type itemInfo struct {
	item       *libraryv1.GetItemResponse
	extras     []*libraryv1.Item
	collection *libraryv1.Item
	series     *libraryv1.Item
}

// loadInfo loads an item's info from its server.
func (a *App) loadInfo(st *detailsState, server, id string) *task[*itemInfo] {
	return load(a, &st.info, key("info", server, id, a.generation), func(ctx context.Context) (*itemInfo, error) {
		items := a.itemService(server)
		res, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: &id}.Build())
		if err != nil {
			return nil, err
		}
		info := &itemInfo{item: res}
		extras, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{
			OwnerId: &id, Sort: []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_NAME)}.Build()},
		}.Build())
		if err != nil {
			return nil, err
		}
		// Theme songs and videos play behind the screen elsewhere; the
		// other extras are watched.
		for _, e := range extras.GetItems() {
			if e.GetExtra() != libraryv1.ExtraKind_EXTRA_KIND_THEME_SONG && e.GetExtra() != libraryv1.ExtraKind_EXTRA_KIND_THEME_VIDEO {
				info.extras = append(info.extras, e)
			}
		}
		if sid := res.GetItem().GetSeriesId(); sid != "" {
			series, err := items.GetItem(ctx, libraryv1.GetItemRequest_builder{Id: &sid}.Build())
			if err != nil {
				return nil, err
			}
			info.series = series.GetItem()
		}
		if it := res.GetItem(); it.GetCollectionName() != "" {
			cols, err := items.ListItems(ctx, libraryv1.ListItemsRequest_builder{
				Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_COLLECTION},
			}.Build())
			if err != nil {
				return nil, err
			}
			id, name := it.GetExternalIds()["tmdb_collection"], strings.ToLower(it.GetCollectionName())
			for _, c := range cols.GetItems() {
				if (id != "" && c.GetExternalIds()["tmdb_collection"] == id) || (id == "" && strings.ToLower(c.GetName()) == name) {
					info.collection = c
					break
				}
			}
		}
		return info, nil
	})
}

var extraLabels = map[libraryv1.ExtraKind]string{
	libraryv1.ExtraKind_EXTRA_KIND_TRAILER:           "extra.trailer",
	libraryv1.ExtraKind_EXTRA_KIND_CLIP:              "extra.clip",
	libraryv1.ExtraKind_EXTRA_KIND_BEHIND_THE_SCENES: "extra.behindTheScenes",
	libraryv1.ExtraKind_EXTRA_KIND_DELETED_SCENE:     "extra.deletedScene",
	libraryv1.ExtraKind_EXTRA_KIND_INTERVIEW:         "extra.interview",
	libraryv1.ExtraKind_EXTRA_KIND_SCENE:             "extra.scene",
	libraryv1.ExtraKind_EXTRA_KIND_SAMPLE:            "extra.sample",
	libraryv1.ExtraKind_EXTRA_KIND_FEATURETTE:        "extra.featurette",
	libraryv1.ExtraKind_EXTRA_KIND_SHORT:             "extra.short",
}

// extrasShelf builds an item's trailers, featurettes and other extras,
// each played by itself; those without a thumbnail wear the item's
// backdrop.
func (a *App) extrasShelf(c *ui.Context, server string, d *capiv1.Details, extras []*libraryv1.Item) {
	if len(extras) == 0 {
		return
	}
	colors := style.Colors(c)
	white := ui.RGB(255, 255, 255)
	width := style.Layout(c).Thumbnail
	a.shelf(c, key("extras", server, d.GetCard().GetItemId()), style.T(c, "item.extras"), nil, func() {
		for _, e := range extras {
			card := a.cardOf(server, e)
			if card.GetThumb() == nil {
				card.SetThumb(d.GetBackdrop())
			}
			b := ui.ButtonBase(c.Key(e.GetId())).Cursor(ui.CursorPointer).Column().Width(width).Gap(6).AlignItems(ui.Start).Label(e.GetName())
			hovered := a.cardFocus(c, b) || b.Hovered()
			if b.Pressed() {
				b.Opacity(0.8)
			}
			b.Children(func() {
				ui.Box(c).Children(func() {
					a.artwork(c, card.GetThumb(), "shows", nil, width, width*9/16, hovered)
					ui.Box(c).Absolute().Left(8).Bottom(8).Size(26, 26).Radius(13).Background(style.Color(colors.Scrim)).Center().Children(func() {
						ui.Icon(c, style.Icon("play", true)).Size(14, 14).TextColor(white)
					})
				})
				style.Text(ui.Text(c, e.GetName()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(width)
				label := extraLabels[e.GetExtra()]
				if label == "" {
					label = "extra.other"
				}
				sub := []string{style.T(c, label)}
				if e.HasRuntime() {
					sub = append(sub, runtime(e.GetRuntime().AsDuration()))
				}
				style.Text(ui.Text(c, joinFacts(sub)), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine().Width(width)
			})
			if b.Clicked() {
				a.play(server, e.GetId(), nil)
			}
		}
	})
}

// collectionBanner builds the collection a movie belongs to over its
// artwork, else the movie's.
func (a *App) collectionBanner(c *ui.Context, server string, d *capiv1.Details, col *libraryv1.Item) {
	if col == nil {
		return
	}
	white := ui.RGB(255, 255, 255)
	image := a.imageOf(server, col.GetImages(), libraryv1.ImageKind_IMAGE_KIND_BACKDROP).GetUrl()
	if image == "" {
		image = d.GetBackdrop().GetUrl()
	}
	// On the television it is taller, its name larger (docs/tv.md §6.2).
	tv := style.TV(c)
	height, padX, name := float32(112), float32(24), style.Type(c).Title3
	if tv {
		height, padX, name = 160, 40, style.Type(c).Title1
	}
	b := ui.ButtonBase(c).Margin(0, style.Layout(c).Margin).Height(height).Radius(14).Clip().
		Background(style.Color(style.Colors(c).SurfaceRaised)).Label(col.GetName())
	hovered := b.Hovered()
	if a.tvFocus(c, b, false) {
		artRing(c, b, true)
		hovered = true
	}
	b.Children(func() {
		if bitmap := a.opts.Images.Get(image, a.contentWidth, scale); bitmap != nil {
			img := ui.Image(c, bitmap).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover)
			if hovered {
				img.Scale(1.05, 1.05)
			}
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 90,
			Stops: []ui.GradientStop{{At: 0, Color: ui.RGBA(0, 0, 0, 0.8)}, {At: 0.6, Color: ui.RGBA(0, 0, 0, 0.55)}, {At: 1, Color: ui.RGBA(0, 0, 0, 0.3)}},
		})
		// Over the artwork, which is positioned as well.
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(20, padX).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				style.Text(ui.Text(c, style.T(c, "item.collection")), style.Type(c).Footnote).FontWeight(600).TextColor(white.Alpha(0.7))
				style.Text(ui.Text(c, col.GetName()), name).TextColor(white).SingleLine()
			})
			pill := ui.Row(c).Height(36).Padding(0, 16).Gap(4).Radius(18).AlignItems(ui.Center).Background(white.Alpha(0.2))
			if hovered {
				pill.Background(white.Alpha(0.3))
			}
			pill.Children(func() {
				style.Text(ui.Text(c, style.T(c, "item.viewCollection")), style.Type(c).Callout).FontWeight(600).TextColor(white)
				ui.Icon(c, style.Icon("forward", false)).Size(14, 14).TextColor(white)
			})
		})
	})
	if b.Clicked() {
		a.router.Push("/item/" + server + "/" + col.GetId())
	}
}

// fact is a caption over its value.
type fact struct{ caption, value string }

// about builds the details at the foot, as the web UI's: the overview
// with the tagline, the leading crew, tags and links beside the ratings,
// the facts, where the title streams and the media of the version chosen.
// What the metadata providers tell of a movie or series beyond what the
// library keeps, as Seerr shows, fills in its status, releases, next
// episode, budget and takings. On the television it is a region in the
// page's flow, its two columns crossed from the leading crew (docs/tv.md
// §6.2).
func (a *App) about(c *ui.Context, st *detailsState, d *capiv1.Details, info *itemInfo, credits []creditEntry) {
	colors := style.Colors(c)
	card := d.GetCard()
	server := card.GetServerId()
	it := info.item.GetItem()
	tv := style.TV(c)
	kind, tmdb := mediaKindOfItem(it), tmdbOfItem(it)
	titled := kind != acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED && tmdb > 0
	var title *acquisitionv1.Title
	if titled {
		title = a.discoveredTitle(c, server, kind, tmdb).value
	}
	var facts []fact
	add := func(caption, value string) {
		if value != "" {
			facts = append(facts, fact{style.T(c, caption), value})
		}
	}
	day := func(t time.Time) string { return t.Local().Format("Jan 2, 2006") }
	// A movie's releases together, what the library keeps first.
	var releases []string
	if it.GetKind() == libraryv1.ItemKind_ITEM_KIND_MOVIE {
		for _, r := range []struct {
			ts  interface{ AsTime() time.Time }
			has bool
			alt string
		}{
			{it.GetPremiereDate(), it.HasPremiereDate(), title.GetTheatricalReleaseDate()},
			{it.GetDigitalReleaseDate(), it.HasDigitalReleaseDate(), title.GetDigitalReleaseDate()},
			{it.GetPhysicalReleaseDate(), it.HasPhysicalReleaseDate(), title.GetPhysicalReleaseDate()},
		} {
			switch {
			case r.has:
				releases = append(releases, day(r.ts.AsTime()))
			case r.alt != "":
				releases = append(releases, dayText(r.alt))
			}
		}
	}
	switch {
	case len(releases) > 1:
		add("discover.releaseDates", strings.Join(releases, "\n"))
	case len(releases) == 1:
		add("discover.releaseDate", releases[0])
	case d.HasPremiereMs():
		caption := "item.released"
		switch card.GetKind() {
		case capiv1.CardKind_CARD_KIND_SERIES:
			caption = "item.firstAired"
		case capiv1.CardKind_CARD_KIND_EPISODE:
			caption = "item.aired"
		}
		add(caption, day(time.UnixMilli(d.GetPremiereMs())))
	case it.HasPremiereDate():
		add("item.released", day(it.GetPremiereDate().AsTime()))
	}
	series := it.GetKind() == libraryv1.ItemKind_ITEM_KIND_SERIES
	switch {
	case series && it.GetSeriesStatus() == libraryv1.SeriesStatus_SERIES_STATUS_CONTINUING:
		add("item.status", style.T(c, "item.continuing"))
	case series && it.GetSeriesStatus() == libraryv1.SeriesStatus_SERIES_STATUS_ENDED:
		add("item.status", style.T(c, "item.ended"))
	default:
		if s, ok := productionStatuses[title.GetProductionStatus()]; ok {
			add("item.status", style.T(c, s))
		}
	}
	if series && it.GetSeriesStatus() == libraryv1.SeriesStatus_SERIES_STATUS_ENDED && it.HasEndDate() {
		add("item.lastAired", day(it.GetEndDate().AsTime()))
	}
	if series && title.GetNextAirDate() != "" {
		add("item.nextAirDate", dayText(title.GetNextAirDate()))
	}
	if ms := card.GetRuntimeMs(); ms > 0 {
		add("item.runtime", runtime(time.Duration(ms)*time.Millisecond))
	}
	add("item.rated", d.GetOfficialRating())
	// An episode or season shows its series' genres and studios.
	genres, studios := it.GetGenres(), it.GetStudios()
	if len(genres) == 0 {
		genres = info.series.GetGenres()
	}
	if len(studios) == 0 {
		studios = info.series.GetStudios()
	}
	if len(genres) == 0 {
		genres = d.GetGenres()
	}
	add("item.genres", strings.Join(genres, ", "))
	studioCaption := "item.studio"
	switch it.GetKind() {
	case libraryv1.ItemKind_ITEM_KIND_SERIES, libraryv1.ItemKind_ITEM_KIND_SEASON, libraryv1.ItemKind_ITEM_KIND_EPISODE:
		studioCaption = "item.networks"
	}
	add(studioCaption, strings.Join(studios, ", "))
	countries := it.GetProductionLocations()
	if len(countries) == 0 {
		for _, code := range title.GetProductionCountries() {
			countries = append(countries, countryName(c, code))
		}
	}
	add("item.country", strings.Join(countries, "\n"))
	add("item.originalLanguage", languageName(c, cmpOr(it.GetOriginalLanguage(), title.GetOriginalLanguage())))
	if it.GetOriginalTitle() != it.GetName() {
		add("item.originalTitle", it.GetOriginalTitle())
	}
	add("item.budget", moneyText(title.GetBudget()))
	add("item.revenue", moneyText(title.GetRevenue()))
	if it.HasDateAdded() && !it.GetVirtual() {
		add("item.dateAdded", day(it.GetDateAdded().AsTime()))
	}
	media := a.mediaFacts(c, st, d, info)
	var tags []string
	if it.GetKind() != libraryv1.ItemKind_ITEM_KIND_EPISODE && it.GetKind() != libraryv1.ItemKind_ITEM_KIND_SEASON {
		tags = it.GetTags()
	}
	// A television opens no web pages (docs/tv.md §1); a movie or series
	// links elsewhere as its discovered title does.
	var links []externalLink
	if a.opts.OpenURL != nil && !tv {
		if titled {
			if title != nil {
				links = titleLinks(c, title, a.ratings(server, kind, tmdb).value)
			}
		} else {
			for _, l := range it.GetExternalUrls() {
				links = append(links, externalLink{l.GetName(), l.GetUrl()})
			}
		}
	}
	scored := title != nil && (title.GetVoteCount() > 0 || len(a.ratings(server, kind, tmdb).value) > 0)
	ratings := scored || it.GetCommunityRating() > 0 || it.GetCriticRating() > 0
	leading := slices.ContainsFunc(credits, func(e creditEntry) bool { return !e.cast() })
	streaming := title.GetWatchProviders()
	text := d.GetOverview()
	if text == "" && !ratings && !leading && len(tags) == 0 && len(facts) == 0 && len(media) == 0 && len(links) == 0 && len(streaming) == 0 {
		return
	}
	// Side by side where there is room, else one under the other.
	wide := a.contentWidth >= 1024
	ui.Column(c).Padding(4, style.Layout(c).Margin, 0).Gap(16).Children(func() {
		style.Text(ui.Text(c, style.T(c, "item.details")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
		a.inFlow(func() {
			box := ui.Column(c).Gap(32)
			if wide {
				box = ui.Row(c).Gap(64).AlignItems(ui.Start)
			}
			box.Children(func() {
				if text != "" || leading || len(tags) > 0 || len(links) > 0 {
					a.inColumn(func() {
						col := ui.Column(c).MinWidth(0).Gap(20)
						if wide {
							col.WidthPercent(44).Shrink(0)
						}
						col.Children(func() {
							if text != "" {
								ui.Column(c).Gap(8).Children(func() {
									if d.GetTagline() != "" {
										style.Text(ui.Text(c, d.GetTagline()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary))
									}
									style.Text(ui.Text(c, text), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary)).Selectable()
								})
							}
							a.crewList(c, server, card.GetItemId(), credits)
							if len(tags) > 0 {
								ui.Column(c).Gap(8).Children(func() {
									style.Text(ui.Text(c, style.T(c, "item.tags")), style.Type(c).Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
									ui.Row(c).Gap(8).Wrap().Children(func() {
										for _, tag := range tags {
											style.Text(ui.Text(c.Key(tag), tag), style.Type(c).Footnote).TextColor(style.Color(colors.TextPrimary)).
												Padding(4, 12).Radius(design.RadiusFull).Border(1, style.Color(colors.Border))
										}
									})
								})
							}
							if len(links) > 0 {
								ui.Row(c).Gap(8).Wrap().Children(func() {
									for i, l := range links {
										if a.linkChip(c.Key(i), l.name).Clicked() {
											_ = a.opts.OpenURL(l.url)
										}
									}
								})
							}
						})
					})
				}
				a.inColumn(func() {
					a.marked(focusMarks{beside: true}, func() {
						ui.Column(c).Grow(1).Basis(0).MinWidth(0).Gap(32).Children(func() {
							switch {
							case scored:
								ui.Row(c).Justify(ui.Start).Children(func() { a.titleRatings(c, server, title) })
							case ratings:
								ratingTiles(c, it)
							}
							factGrid(c, facts)
							watchProviders(c, a, streaming)
							factGrid(c, media)
						})
					})
				})
			})
		})
	})
}

// watchProviders builds where a title streams: the services' logos, else
// their names.
func watchProviders(c *ui.Context, a *App, ps []*pluginv1.WatchProvider) {
	if len(ps) == 0 {
		return
	}
	colors := style.Colors(c)
	ui.Column(c).Gap(8).Children(func() {
		style.Text(ui.Text(c, style.T(c, "item.streaming")), style.Type(c).Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
		ui.Row(c).Gap(12).Wrap().Children(func() {
			for i, p := range ps {
				if bitmap := a.opts.Images.Get(p.GetLogoUrl(), 40, scale); bitmap != nil {
					ui.Box(c.Key(i)).Size(40, 40).Radius(8).Clip().Tooltip(p.GetName()).Children(func() {
						ui.Image(c, bitmap).Fill().Fit(ui.Cover)
					})
				} else {
					style.Text(ui.Text(c.Key(i), p.GetName()), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary))
				}
			}
		})
	})
}

// linkChip builds a pill opening a page elsewhere.
func (a *App) linkChip(c *ui.Context, label string) ui.Element {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Height(30).Padding(0, 14).Gap(6).Radius(design.RadiusFull).AlignItems(ui.Center).
		Background(style.Color(colors.Fill)).Label(label)
	fg, sub := style.Color(colors.TextPrimary), style.Color(colors.TextSecondary)
	switch {
	case a.tvLink(c, b, design.RadiusFull):
		fg, sub = ui.RGB(0, 0, 0), style.Color(design.LightColors.TextSecondary)
	case b.Hovered():
		b.Background(style.Color(colors.FillSelected))
	}
	b.Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Footnote).FontWeight(600).TextColor(fg)
		ui.Icon(c, style.Icon("external", false)).Size(14, 14).TextColor(sub)
	})
	return b
}

// factGrid builds facts in columns at least 200 wide.
func factGrid(c *ui.Context, facts []fact) {
	if len(facts) == 0 {
		return
	}
	colors := style.Colors(c)
	ui.Row(c).GapX(32).GapY(20).Wrap().Children(func() {
		for _, f := range facts {
			ui.Column(c.Key(f.caption)).Basis(200).Grow(1).MinWidth(0).Gap(4).Children(func() {
				style.Text(ui.Text(c, f.caption), style.Type(c).Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
				style.Text(ui.Text(c, f.value), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
			})
		}
	})
}

// ratingTiles builds the audience's rating out of 10 and the critics'
// score out of 100, large.
func ratingTiles(c *ui.Context, it *libraryv1.Item) {
	colors := style.Colors(c)
	tile := func(label string, value func(), scale string) {
		ui.Column(c.Key(label)).MinWidth(144).Gap(4).Padding(12, 16).Radius(12).Background(style.Color(colors.Fill)).Children(func() {
			style.Text(ui.Text(c, label), style.Type(c).Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				value()
				if scale != "" {
					style.Text(ui.Text(c, scale), style.Type(c).Callout).TextColor(style.Color(colors.TextTertiary))
				}
			})
		})
	}
	ui.Row(c).Gap(12).Wrap().Children(func() {
		if r := it.GetCommunityRating(); r > 0 {
			tile(style.T(c, "item.communityRating"), func() {
				ui.Icon(c, style.Icon("rating", true)).Size(22, 22).TextColor(ui.RGB(0xFF, 0xD6, 0x0A))
				style.Text(ui.Text(c, fmt.Sprintf("%.1f", r)), style.Type(c).Title2).FontWeight(700).TextColor(style.Color(colors.TextPrimary))
			}, "/ 10")
		}
		if r := it.GetCriticRating(); r > 0 {
			tile(style.T(c, "item.criticRating"), func() {
				style.Text(ui.Text(c, fmt.Sprintf("%.0f%%", r)), style.Type(c).Title2).FontWeight(700).TextColor(style.Color(colors.TextPrimary))
			}, "")
		}
	})
}

// mediaFacts returns the media of the version chosen: its video, audio and
// subtitle streams, and its file.
func (a *App) mediaFacts(c *ui.Context, st *detailsState, d *capiv1.Details, info *itemInfo) []fact {
	sources := d.GetSources()
	if st.source >= len(sources) || sources[st.source].GetServerId() != d.GetCard().GetServerId() {
		return nil
	}
	id := sources[st.source].GetMediaSourceId()
	i := slices.IndexFunc(info.item.GetMediaSources(), func(s *libraryv1.MediaSource) bool { return s.GetId() == id })
	if i < 0 {
		return nil
	}
	s := info.item.GetMediaSources()[i]
	var facts []fact
	add := func(caption, value string) {
		if value != "" {
			facts = append(facts, fact{style.T(c, caption), value})
		}
	}
	streams := func(kind libraryv1.StreamKind) []string {
		var out []string
		for _, m := range s.GetStreams() {
			if m.GetKind() == kind {
				out = append(out, streamText(c, m))
			}
		}
		return out
	}
	if v := videoOf(s); v != nil {
		parts := []string{resolutionOf(s), strings.ToUpper(v.GetCodec()), rangeNames[v.GetRange()]}
		add("item.video", joinFacts(slices.DeleteFunc(parts, func(p string) bool { return p == "" })))
	}
	add("item.audioTracks", strings.Join(streams(libraryv1.StreamKind_STREAM_KIND_AUDIO), "\n"))
	add("item.subtitleTracks", strings.Join(streams(libraryv1.StreamKind_STREAM_KIND_SUBTITLE), "\n"))
	file := []string{path.Base(s.GetPath())}
	if s.GetSizeBytes() > 0 {
		file = append(file, size(uint64(s.GetSizeBytes())))
	}
	if s.GetPath() != "" {
		add("item.file", strings.Join(file, "\n"))
	}
	return facts
}

func videoOf(s *libraryv1.MediaSource) *libraryv1.MediaStream {
	for _, m := range s.GetStreams() {
		if m.GetKind() == libraryv1.StreamKind_STREAM_KIND_VIDEO {
			return m
		}
	}
	return nil
}

// resolutionOf names the resolution of a source's video, as "4K".
func resolutionOf(s *libraryv1.MediaSource) string {
	v := videoOf(s)
	switch {
	case v == nil || v.GetHeight() == 0:
		return ""
	case v.GetWidth() >= 3200 || v.GetHeight() >= 2000:
		return "4K"
	case v.GetWidth() >= 1800 || v.GetHeight() >= 1000:
		return "1080p"
	case v.GetWidth() >= 1200 || v.GetHeight() >= 700:
		return "720p"
	}
	return fmt.Sprintf("%dp", v.GetHeight())
}

var rangeNames = map[libraryv1.VideoRange]string{
	libraryv1.VideoRange_VIDEO_RANGE_HDR10:        "HDR10",
	libraryv1.VideoRange_VIDEO_RANGE_HDR10_PLUS:   "HDR10+",
	libraryv1.VideoRange_VIDEO_RANGE_HLG:          "HLG",
	libraryv1.VideoRange_VIDEO_RANGE_DOLBY_VISION: "Dolby Vision",
}

// streamText returns a stream's language or title, codec and channels,
// as the details list them.
func streamText(c *ui.Context, m *libraryv1.MediaStream) string {
	var channels string
	switch n := m.GetChannels(); {
	case n == 6:
		channels = "5.1"
	case n == 8:
		channels = "7.1"
	case n > 0:
		channels = fmt.Sprintf("%d ch", n)
	}
	parts := []string{cmpOr(m.GetTitle(), languageName(c, m.GetLanguage())), strings.ToUpper(m.GetCodec()), channels}
	return joinFacts(slices.DeleteFunc(parts, func(p string) bool { return p == "" }))
}

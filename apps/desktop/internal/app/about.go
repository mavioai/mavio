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
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// itemInfo is what the details show of the server's own item beyond the
// core's details: its credits by job, facts and files, its extras, and
// the collection it belongs to.
type itemInfo struct {
	item       *libraryv1.GetItemResponse
	extras     []*libraryv1.Item
	collection *libraryv1.Item
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
	width := style.Layout.Thumbnail
	a.shelf(c, key("extras", server, d.GetCard().GetItemId()), style.T(c, "item.extras"), nil, func() {
		for _, e := range extras {
			card := a.cardOf(server, e)
			if card.GetThumb() == nil {
				card.SetThumb(d.GetBackdrop())
			}
			b := ui.ButtonBase(c.Key(e.GetId())).Column().Width(width).Gap(6).AlignItems(ui.Start).Label(e.GetName())
			hovered := b.Hovered()
			if b.Pressed() {
				b.Opacity(0.8)
			}
			b.Children(func() {
				ui.Box(c).Children(func() {
					a.artwork(c, card.GetThumb(), e.GetName(), nil, width, width*9/16, hovered)
					ui.Box(c).Absolute().Left(8).Bottom(8).Size(26, 26).Radius(13).Background(style.Color(colors.Scrim)).Center().Children(func() {
						ui.Icon(c, style.Icon("play", true)).Size(14, 14).TextColor(white)
					})
				})
				style.Text(ui.Text(c, e.GetName()), style.Type.Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(width)
				label := extraLabels[e.GetExtra()]
				if label == "" {
					label = "extra.other"
				}
				sub := []string{style.T(c, label)}
				if e.HasRuntime() {
					sub = append(sub, runtime(e.GetRuntime().AsDuration()))
				}
				style.Text(ui.Text(c, joinFacts(sub)), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine().Width(width)
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
	b := ui.ButtonBase(c).Margin(0, style.Layout.Margin).Height(112).Radius(14).Clip().
		Background(style.Color(style.Colors(c).SurfaceRaised)).Label(col.GetName())
	hovered := b.Hovered()
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
		ui.Row(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).Padding(20, 24).Gap(16).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Children(func() {
				style.Text(ui.Text(c, style.T(c, "item.collection")), style.Type.Footnote).FontWeight(600).TextColor(white.Alpha(0.7))
				style.Text(ui.Text(c, col.GetName()), style.Type.Title3).TextColor(white).SingleLine()
			})
			pill := ui.Row(c).Height(36).Padding(0, 16).Gap(4).Radius(18).AlignItems(ui.Center).Background(white.Alpha(0.2))
			if hovered {
				pill.Background(white.Alpha(0.3))
			}
			pill.Children(func() {
				style.Text(ui.Text(c, style.T(c, "item.viewCollection")), style.Type.Callout).FontWeight(600).TextColor(white)
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

// about builds the details at the foot: the overview with the tagline,
// the leading crew, keywords and links beside the ratings and facts, and
// under them the media of the version chosen.
func (a *App) about(c *ui.Context, st *detailsState, d *capiv1.Details, info *itemInfo, credits []creditEntry) {
	colors := style.Colors(c)
	card := d.GetCard()
	server := card.GetServerId()
	it := info.item.GetItem()
	var facts []fact
	add := func(caption, value string) {
		if value != "" {
			facts = append(facts, fact{style.T(c, caption), value})
		}
	}
	day := func(t time.Time) string { return t.Local().Format("Jan 2, 2006") }
	switch {
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
	if it.GetKind() == libraryv1.ItemKind_ITEM_KIND_SERIES {
		switch it.GetSeriesStatus() {
		case libraryv1.SeriesStatus_SERIES_STATUS_CONTINUING:
			add("item.status", style.T(c, "item.continuing"))
		case libraryv1.SeriesStatus_SERIES_STATUS_ENDED:
			add("item.status", style.T(c, "item.ended"))
			if it.HasEndDate() {
				add("item.lastAired", day(it.GetEndDate().AsTime()))
			}
		}
	}
	if ms := card.GetRuntimeMs(); ms > 0 {
		add("item.runtime", runtime(time.Duration(ms)*time.Millisecond))
	}
	add("item.rated", d.GetOfficialRating())
	add("item.genres", strings.Join(d.GetGenres(), ", "))
	studios := "item.studio"
	switch it.GetKind() {
	case libraryv1.ItemKind_ITEM_KIND_SERIES, libraryv1.ItemKind_ITEM_KIND_SEASON, libraryv1.ItemKind_ITEM_KIND_EPISODE:
		studios = "item.networks"
	}
	add(studios, strings.Join(it.GetStudios(), ", "))
	add("item.country", strings.Join(it.GetProductionLocations(), "\n"))
	add("item.originalLanguage", languageName(c, it.GetOriginalLanguage()))
	if it.GetOriginalTitle() != it.GetName() {
		add("item.originalTitle", it.GetOriginalTitle())
	}
	if it.HasDateAdded() && !it.GetVirtual() {
		add("item.dateAdded", day(it.GetDateAdded().AsTime()))
	}
	media := a.mediaFacts(c, st, d, info)
	var tags []string
	if it.GetKind() != libraryv1.ItemKind_ITEM_KIND_EPISODE && it.GetKind() != libraryv1.ItemKind_ITEM_KIND_SEASON {
		tags = it.GetTags()
	}
	links := it.GetExternalUrls()
	if a.opts.OpenURL == nil {
		links = nil
	}
	ratings := it.GetCommunityRating() > 0 || it.GetCriticRating() > 0
	leading := slices.ContainsFunc(credits, func(e creditEntry) bool { return !e.cast() })
	text := d.GetOverview()
	if text == "" && !ratings && !leading && len(tags) == 0 && len(facts) == 0 && len(media) == 0 && len(links) == 0 {
		return
	}
	ui.Column(c).Padding(0, style.Layout.Margin).Gap(16).Children(func() {
		style.Text(ui.Text(c, style.T(c, "item.details")), style.Type.Title3).TextColor(style.Color(colors.TextPrimary))
		ui.Row(c).GapX(64).GapY(32).AlignItems(ui.Start).Wrap().Children(func() {
			if text != "" || leading || len(tags) > 0 || len(links) > 0 {
				ui.Column(c).Basis(360).Grow(1).MaxWidth(640).Gap(20).Children(func() {
					if text != "" {
						ui.Column(c).Gap(8).Children(func() {
							if d.GetTagline() != "" {
								style.Text(ui.Text(c, d.GetTagline()), style.Type.Headline).TextColor(style.Color(colors.TextPrimary))
							}
							style.Text(ui.Text(c, text), style.Type.Body).TextColor(style.Color(colors.TextSecondary)).Selectable()
						})
					}
					a.crewList(c, server, card.GetItemId(), credits)
					if len(tags) > 0 {
						ui.Column(c).Gap(8).Children(func() {
							style.Text(ui.Text(c, style.T(c, "item.tags")), style.Type.Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
							ui.Row(c).Gap(8).Wrap().Children(func() {
								for _, tag := range tags {
									style.Text(ui.Text(c.Key(tag), tag), style.Type.Footnote).TextColor(style.Color(colors.TextPrimary)).
										Padding(4, 12).Radius(design.RadiusFull).Border(1, style.Color(colors.Border))
								}
							})
						})
					}
					if len(links) > 0 {
						ui.Row(c).Gap(8).Wrap().Children(func() {
							for i, l := range links {
								if linkChip(c.Key(i), l.GetName()).Clicked() {
									_ = a.opts.OpenURL(l.GetUrl())
								}
							}
						})
					}
				})
			}
			ui.Column(c).Basis(360).Grow(1).MinWidth(0).Gap(32).Children(func() {
				if ratings {
					ratingTiles(c, it)
				}
				factGrid(c, facts)
				factGrid(c, media)
			})
		})
	})
}

// linkChip builds a pill opening a page elsewhere.
func linkChip(c *ui.Context, label string) ui.Element {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Height(30).Padding(0, 14).Gap(6).Radius(design.RadiusFull).AlignItems(ui.Center).
		Background(style.Color(colors.Fill)).Label(label)
	if b.Hovered() {
		b.Background(style.Color(colors.FillSelected))
	}
	b.Children(func() {
		style.Text(ui.Text(c, label), style.Type.Footnote).FontWeight(600).TextColor(style.Color(colors.TextPrimary))
		ui.Icon(c, style.Icon("external", false)).Size(14, 14).TextColor(style.Color(colors.TextSecondary))
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
				style.Text(ui.Text(c, f.caption), style.Type.Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
				style.Text(ui.Text(c, f.value), style.Type.Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
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
			style.Text(ui.Text(c, label), style.Type.Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				value()
				if scale != "" {
					style.Text(ui.Text(c, scale), style.Type.Callout).TextColor(style.Color(colors.TextTertiary))
				}
			})
		})
	}
	ui.Row(c).Gap(12).Wrap().Children(func() {
		if r := it.GetCommunityRating(); r > 0 {
			tile(style.T(c, "item.communityRating"), func() {
				ui.Icon(c, style.Icon("rating", true)).Size(22, 22).TextColor(ui.RGB(0xFF, 0xD6, 0x0A))
				style.Text(ui.Text(c, fmt.Sprintf("%.1f", r)), style.Type.Title2).FontWeight(700).TextColor(style.Color(colors.TextPrimary))
			}, "/ 10")
		}
		if r := it.GetCriticRating(); r > 0 {
			tile(style.T(c, "item.criticRating"), func() {
				style.Text(ui.Text(c, fmt.Sprintf("%.0f%%", r)), style.Type.Title2).FontWeight(700).TextColor(style.Color(colors.TextPrimary))
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

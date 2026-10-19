package app

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strconv"
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

// detailsState is an item's details page's state.
type detailsState struct {
	details task[*capiv1.Details]
	// season is the season of a series shown, its index among the
	// seasons, -1 until the next episode's season is known.
	season       int
	seasonDetail task[*capiv1.Details]
	source       int
	overviewOpen bool
	info         task[*itemInfo]
	// episodeInfo are the server's own episodes of the season shown;
	// scrolledTo tells that an episode's season scrolled to it.
	episodeInfo task[map[string]*libraryv1.Item]
	scrolledTo  bool
	more        moreState
}

// details builds an item's details, as the web UI's
// (apps/web/src/screens/details.tsx): the header with the actions, the
// episodes, the cast, the extras, the collection, similar items, the
// recommendations, and the details at the foot.
func (a *App) details(c *ui.Context, r *ui.Route, server, id string) {
	st := a.detailStates[key(server, id)]
	if st == nil {
		st = &detailsState{season: -1}
		a.detailStates[key(server, id)] = st
	}
	t := load(a, &st.details, key("details", server, id, a.generation), func(ctx context.Context) (*capiv1.Details, error) {
		return a.getDetails(ctx, server, id)
	})
	if taskStatus(c, t, func() { st.details.key = "" }, nil) {
		return
	}
	d := t.value
	r.Title(d.GetCard().GetTitle())
	// Offline, or before the server answers, the core's details tell what
	// they can.
	info := a.loadInfo(st, server, id)
	credits := coreCredits(d)
	if info.value != nil {
		credits = a.libraryCredits(server, info.value.item.GetCredits())
	}
	ui.Scroll(c).TrackScroll(a.pageScroll(a.pagePath)).Fill().Children(func() {
		ui.Column(c).Padding(0, 0, 40).Children(func() {
			a.detailsHeader(c, st, d, info.value)
			ui.Column(c).Gap(32).Padding(28, 0, 0).Children(func() {
				a.episodes(c, st, d, info.value)
				a.castShelf(c, server, id, credits)
				if info.value != nil {
					a.extrasShelf(c, server, d, info.value.extras)
					a.collectionBanner(c, server, d, info.value.collection)
				}
				if similar := d.GetSimilar(); len(similar) > 0 {
					a.shelf(c, key("similar", server, id), style.T(c, "item.similar"), nil, func() {
						for _, card := range similar {
							a.posterCard(c, card, style.Layout(c).Poster)
						}
					})
				}
				if info.value != nil {
					a.recommendations(c, info.value.item.GetItem())
					a.about(c, st, d, info.value, credits)
				} else {
					a.facts(c, st, d)
				}
			})
		})
	})
}

// mediaKindOfItem returns the kind of title discovery knows a movie or
// series as, unspecified for anything else.
func mediaKindOfItem(it *libraryv1.Item) acquisitionv1.MediaKind {
	switch it.GetKind() {
	case libraryv1.ItemKind_ITEM_KIND_MOVIE:
		return acquisitionv1.MediaKind_MEDIA_KIND_MOVIE
	case libraryv1.ItemKind_ITEM_KIND_SERIES:
		return acquisitionv1.MediaKind_MEDIA_KIND_SERIES
	}
	return acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED
}

// tmdbOfItem returns an item's TMDB ID, 0 for none.
func tmdbOfItem(it *libraryv1.Item) int32 {
	n, _ := strconv.ParseInt(it.GetExternalIds()["tmdb"], 10, 32)
	return int32(max(n, 0))
}

// recommendations builds the titles the discovery providers recommend to
// those who liked a movie or series, those in a library opening their
// details there.
func (a *App) recommendations(c *ui.Context, it *libraryv1.Item) {
	kind, tmdb := mediaKindOfItem(it), tmdbOfItem(it)
	if kind == acquisitionv1.MediaKind_MEDIA_KIND_UNSPECIFIED || tmdb == 0 {
		return
	}
	server, catalogs, _ := a.discovery(c)
	if server == "" {
		return
	}
	i := slices.IndexFunc(catalogs, func(cat *acquisitionv1.Catalog) bool {
		return cat.GetKind() == pluginv1.CatalogKind_CATALOG_KIND_RECOMMENDATIONS && slices.Contains(cat.GetMediaKinds(), kind)
	})
	if i < 0 {
		return
	}
	a.catalogShelf(c, server, catalogRow{catalogs[i], kind}, style.T(c, "discover.recommendations"), tmdb)
}

func (a *App) getDetails(ctx context.Context, server, id string) (*capiv1.Details, error) {
	m, err := a.opts.Core.Call(ctx, capiv1.Request_builder{GetDetails: capiv1.GetDetails_builder{ServerId: server, ItemId: id}.Build()}.Build())
	return m.GetDetails(), err
}

var episodeNumbers = regexp.MustCompile(`S(\d+):E(\d+)`)

// episodeName returns how an action names an episode: "S1 E2", else its
// title.
func episodeName(c *ui.Context, card *capiv1.Card) string {
	if m := episodeNumbers.FindStringSubmatch(card.GetSubtitle()); m != nil {
		return style.T(c, "item.episodeShort", "season", m[1], "episode", m[2])
	}
	return card.GetTitle()
}

// nextEpisode returns the episode to watch next of episodes: the first
// unfinished one, else the first unplayed one, else the first.
func nextEpisode(episodes []*capiv1.Card) *capiv1.Card {
	if len(episodes) == 0 {
		return nil
	}
	for _, e := range episodes {
		if e.HasProgress() && e.GetProgress() > 0 && !e.GetPlayed() {
			return e
		}
	}
	for _, e := range episodes {
		if !e.GetPlayed() {
			return e
		}
	}
	return episodes[0]
}

// episodesOf returns the episodes shown of a series or season: a season's
// own, or those of the series' season shown.
func (a *App) episodesOf(st *detailsState, d *capiv1.Details) []*capiv1.Card {
	switch d.GetCard().GetKind() {
	case capiv1.CardKind_CARD_KIND_SEASON:
		return d.GetChildren()
	case capiv1.CardKind_CARD_KIND_SERIES:
		seasons := d.GetChildren()
		if len(seasons) == 0 {
			return nil
		}
		if st.season < 0 || st.season >= len(seasons) {
			st.season = 0
			for i, s := range seasons {
				if !s.GetPlayed() {
					st.season = i
					break
				}
			}
		}
		season := seasons[st.season]
		t := load(a, &st.seasonDetail, key("season", season.GetServerId(), season.GetItemId(), a.generation), func(ctx context.Context) (*capiv1.Details, error) {
			return a.getDetails(ctx, season.GetServerId(), season.GetItemId())
		})
		if t.loaded && t.err == nil && t.value.GetCard().GetItemId() == season.GetItemId() {
			return t.value.GetChildren()
		}
	}
	return nil
}

// detailsHeader builds the top of the details, as the web UI's: the
// artwork, and over its lower left the series an episode or season
// belongs to, the logo or title, the facts and badges, the overview and
// the actions. On the television the artwork fills 86 % of the screen,
// under the title at 72 points, two lines of overview and no tagline, the
// text at most 960 wide (docs/tv.md §6.2).
func (a *App) detailsHeader(c *ui.Context, st *detailsState, d *capiv1.Details, info *itemInfo) {
	card := d.GetCard()
	server := card.GetServerId()
	kind := card.GetKind()
	_, wh := c.Size()
	tv := style.TV(c)
	height := min(max(wh*0.7, 520), 700)
	textWidth, posterW, logoH, seriesLogoH, titleSize := float32(640), float32(200), float32(112), float32(48), float32(52)
	if tv {
		height, textWidth, posterW, logoH, seriesLogoH, titleSize = wh*0.86, 960, 300, 160, 80, 72
	}
	textWidth = min(textWidth, a.contentWidth-2*style.Layout(c).Margin)
	white := ui.RGB(255, 255, 255)
	inSeries := kind == capiv1.CardKind_CARD_KIND_EPISODE || kind == capiv1.CardKind_CARD_KIND_SEASON
	var it, series *libraryv1.Item
	if info != nil {
		it, series = info.item.GetItem(), info.series
	}
	// An episode or season wears its series' artwork where it has none.
	image := d.GetBackdrop().GetUrl()
	if image == "" && series != nil {
		image = a.imageOf(server, series.GetImages(), libraryv1.ImageKind_IMAGE_KIND_BACKDROP).GetUrl()
	}
	if image == "" && kind == capiv1.CardKind_CARD_KIND_EPISODE {
		image = card.GetThumb().GetUrl()
	}
	poster := ""
	if image == "" {
		poster = card.GetPoster().GetUrl()
	}
	logo := d.GetLogo().GetUrl()
	if inSeries {
		logo = ""
	}
	ui.Box(c).Height(height).Clip().Background(style.Color(design.DarkColors.Surface)).DragWindow().Children(func() {
		if image != "" {
			if bitmap := a.opts.Images.Get(image, a.contentWidth, scale); bitmap != nil {
				ui.Image(c, bitmap).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover)
			}
		} else if bitmap := a.opts.Images.Get(poster, 400, scale); bitmap != nil {
			// Without a backdrop the poster, blurred, colors the header.
			ui.Image(c, bitmap).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover).Scale(1.25, 1.25).Opacity(0.7).Blur(48)
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 90,
			Stops: []ui.GradientStop{{At: 0, Color: ui.RGBA(0, 0, 0, 0.82)}, {At: 0.45, Color: ui.RGBA(0, 0, 0, 0.45)}, {At: 0.75, Color: ui.RGBA(0, 0, 0, 0)}},
		})
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 0,
			Stops: []ui.GradientStop{{At: 0, Color: ui.RGBA(0, 0, 0, 0.75)}, {At: 0.55, Color: ui.RGBA(0, 0, 0, 0)}},
		})
		block := ui.Row(c).Absolute().Left(0).Right(0).Bottom(0).Padding(40).Gap(40).AlignItems(ui.End)
		if tv {
			block.Padding(0, style.Layout(c).Margin, wh*0.1)
		}
		block.Children(func() {
			if poster != "" {
				if bitmap := a.opts.Images.Get(poster, posterW, scale); bitmap != nil {
					ui.Box(c).Width(posterW).Height(posterW*1.5).Shrink(0).Radius(design.RadiusLg).Clip().
						Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.4)).Children(func() {
						ui.Image(c, bitmap).Fill().Fit(ui.Cover)
					})
				}
			}
			ui.Column(c).MinWidth(0).MaxWidth(textWidth).Gap(12).Children(func() {
				if inSeries && it.GetSeriesId() != "" {
					a.seriesLink(c, server, it, series, seriesLogoH)
				}
				if kind == capiv1.CardKind_CARD_KIND_EPISODE && it.GetIndexNumber() > 0 {
					style.Text(ui.Text(c, style.T(c, "item.episodeNumber", "season", it.GetParentIndexNumber(), "episode", it.GetIndexNumber())), style.Type(c).Headline).
						TextColor(white.Alpha(0.85))
				}
				if bitmap := a.opts.Images.Get(logo, 800, scale); bitmap != nil {
					ui.Image(c, bitmap).MaxWidth(textWidth * 0.8).MaxHeight(logoH).Fit(ui.Contain).AlignSelf(ui.Start)
				} else {
					style.Text(ui.Text(c, card.GetTitle()), style.Type(c).Display).FontSize(titleSize).FontWeight(700).LineHeight(1.05).
						TextColor(white).MaxLines(3)
				}
				if facts := detailFacts(c, d, it); facts != "" {
					style.Text(ui.Text(c, facts), style.Type(c).Callout).TextColor(white.Alpha(0.8))
				}
				badges := detailBadges(d, a.chosenSource(st, d, info))
				several := len(a.servers) > 1 && d.GetServerName() != ""
				if len(badges) > 0 || several {
					ui.Row(c).GapX(12).GapY(6).Wrap().AlignItems(ui.Center).Children(func() {
						for _, b := range badges {
							style.Text(ui.Text(c.Key(b), b), style.Type(c).Caption).FontWeight(600).TextColor(white.Alpha(0.9)).
								Padding(0, 6).Radius(4).Border(1, white.Alpha(0.5))
						}
						if several {
							style.Text(ui.Text(c, d.GetServerName()), style.Type(c).Caption).TextColor(white.Alpha(0.8)).
								Padding(0, 6).Radius(4).Background(white.Alpha(0.2))
						}
					})
				}
				if d.GetTagline() != "" && !tv {
					style.Text(ui.Text(c, d.GetTagline()), style.Type(c).Headline).TextColor(white.Alpha(0.9))
				}
				a.headerOverview(c, st, d)
				a.actions(c, st, d, info)
			})
		})
	})
}

// seriesLink builds the series an episode or season belongs to, its logo
// else its name, opening it.
func (a *App) seriesLink(c *ui.Context, server string, it, series *libraryv1.Item, logoH float32) {
	white := ui.RGB(255, 255, 255)
	name := it.GetSeriesName()
	b := ui.ButtonBase(c.Key("series")).Cursor(ui.CursorPointer).AlignSelf(ui.Start).Label(name)
	lit := a.tvLink(c, b, design.RadiusSm)
	logo := a.imageOf(server, series.GetImages(), libraryv1.ImageKind_IMAGE_KIND_LOGO).GetUrl()
	b.Children(func() {
		if bitmap := a.opts.Images.Get(logo, 400, scale); bitmap != nil {
			ui.Image(c, bitmap).MaxWidth(400).MaxHeight(logoH).Fit(ui.Contain)
			return
		}
		fg := white.Alpha(0.85)
		switch {
		case lit:
			fg = ui.RGB(0, 0, 0)
		case b.Hovered():
			fg = white
		}
		style.Text(ui.Text(c, name), style.Type(c).Headline).TextColor(fg).SingleLine()
	})
	if b.Clicked() {
		a.router.Push("/item/" + server + "/" + it.GetSeriesId())
	}
}

// headerOverview builds the overview over the artwork: three lines, two on
// the television, which a click opens whole.
func (a *App) headerOverview(c *ui.Context, st *detailsState, d *capiv1.Details) {
	if d.GetOverview() == "" {
		return
	}
	white := ui.RGB(255, 255, 255)
	o := style.Text(ui.Text(c, d.GetOverview()), style.Type(c).Body).TextColor(white.Alpha(0.85))
	switch {
	case style.TV(c):
		o.MaxLines(2)
	case !st.overviewOpen:
		o.MaxLines(3).Cursor(ui.CursorPointer).Tooltip(style.T(c, "item.more"))
	}
	if o.Clicked() {
		st.overviewOpen = !st.overviewOpen
	}
}

// detailFacts returns the line of an item's facts, as the web UI's: a
// series' years and seasons, an episode's air date, a season's episodes,
// else the year; the runtime but a series'; and the genres but an
// episode's.
func detailFacts(c *ui.Context, d *capiv1.Details, it *libraryv1.Item) string {
	card := d.GetCard()
	var facts []string
	switch card.GetKind() {
	case capiv1.CardKind_CARD_KIND_SERIES:
		if y := card.GetYear(); y > 0 {
			to := 0
			if it.HasEndDate() {
				to = it.GetEndDate().AsTime().Year()
			}
			switch {
			case to > 0 && to != int(y):
				facts = append(facts, fmt.Sprintf("%d–%d", y, to))
			case it.GetSeriesStatus() == libraryv1.SeriesStatus_SERIES_STATUS_CONTINUING || it == nil:
				facts = append(facts, fmt.Sprintf("%d–", y))
			default:
				facts = append(facts, fmt.Sprint(y))
			}
		}
		if n := len(d.GetChildren()); n > 0 {
			facts = append(facts, style.T(c, "item.seasonCount", "count", n))
		}
	case capiv1.CardKind_CARD_KIND_SEASON:
		if card.GetYear() > 0 {
			facts = append(facts, fmt.Sprint(card.GetYear()))
		}
		if n := len(d.GetChildren()); n > 0 {
			facts = append(facts, style.T(c, "item.episodeCount", "count", n))
		}
	case capiv1.CardKind_CARD_KIND_EPISODE:
		if d.HasPremiereMs() {
			facts = append(facts, time.UnixMilli(d.GetPremiereMs()).Format("Jan 2, 2006"))
		}
	default:
		if card.GetYear() > 0 {
			facts = append(facts, fmt.Sprint(card.GetYear()))
		}
	}
	if card.GetRuntimeMs() > 0 && card.GetKind() != capiv1.CardKind_CARD_KIND_SERIES {
		facts = append(facts, runtime(time.Duration(card.GetRuntimeMs())*time.Millisecond))
	}
	if genres := d.GetGenres(); len(genres) > 0 && card.GetKind() != capiv1.CardKind_CARD_KIND_EPISODE {
		facts = append(facts, strings.Join(genres[:min(len(genres), 3)], " / "))
	}
	return joinFacts(facts)
}

// detailBadges returns the badges of an item and its version chosen: its
// content rating, the resolution, range and surround sound of the version,
// and CC where it has subtitles.
func detailBadges(d *capiv1.Details, s *libraryv1.MediaSource) []string {
	badges := []string{d.GetOfficialRating(), resolutionOf(s)}
	if v := videoOf(s); v != nil {
		badges = append(badges, rangeNames[v.GetRange()])
	}
	most := int32(0)
	subtitles := false
	for _, m := range s.GetStreams() {
		switch m.GetKind() {
		case libraryv1.StreamKind_STREAM_KIND_AUDIO:
			most = max(most, m.GetChannels())
		case libraryv1.StreamKind_STREAM_KIND_SUBTITLE:
			subtitles = true
		}
	}
	switch {
	case most >= 8:
		badges = append(badges, "7.1")
	case most >= 6:
		badges = append(badges, "5.1")
	}
	if subtitles {
		badges = append(badges, "CC")
	}
	return slices.DeleteFunc(badges, func(b string) bool { return b == "" })
}

// chosenSource returns the server's media source of the version chosen,
// nil until the item's info is loaded or for a copy on another server.
func (a *App) chosenSource(st *detailsState, d *capiv1.Details, info *itemInfo) *libraryv1.MediaSource {
	sources := d.GetSources()
	if info == nil || st.source >= len(sources) || sources[st.source].GetServerId() != d.GetCard().GetServerId() {
		return nil
	}
	id := sources[st.source].GetMediaSourceId()
	i := slices.IndexFunc(info.item.GetMediaSources(), func(s *libraryv1.MediaSource) bool { return s.GetId() == id })
	if i < 0 {
		return nil
	}
	return info.item.GetMediaSources()[i]
}

// actions builds the details' actions, as the web UI's: Play or Resume,
// of the item or of the episode to watch next, from the beginning, the
// trailer, favorite, played and the version to play; and under them the
// status line, with the progress, the episode's name, the time left and
// when it would end. On the television focus starts on Play.
func (a *App) actions(c *ui.Context, st *detailsState, d *capiv1.Details, info *itemInfo) {
	card := d.GetCard()
	white := ui.RGB(255, 255, 255)
	tv := style.TV(c)
	target, label, position := card, "", d.GetPositionMs()
	// The episodes after the target, which play in a row.
	var following []*capiv1.Card
	switch card.GetKind() {
	case capiv1.CardKind_CARD_KIND_SERIES, capiv1.CardKind_CARD_KIND_SEASON:
		episodes := a.episodesOf(st, d)
		target = nextEpisode(episodes)
		if i := slices.Index(episodes, target); i >= 0 {
			following = episodes[i+1:]
		}
		position = 0
		if target != nil {
			name := episodeName(c, target)
			label = style.T(c, "item.playEpisode", "episode", name)
			if target.HasProgress() && target.GetProgress() > 0 && !target.GetPlayed() {
				label = style.T(c, "item.resumeEpisode", "episode", name)
				position = int64(float64(target.GetRuntimeMs()) * float64(target.GetProgress()))
			}
		}
	case capiv1.CardKind_CARD_KIND_MOVIE, capiv1.CardKind_CARD_KIND_EPISODE, capiv1.CardKind_CARD_KIND_VIDEO, capiv1.CardKind_CARD_KIND_MUSIC_VIDEO:
		label = style.T(c, "item.play")
		if position > 0 {
			label = style.T(c, "item.resume")
		}
		if len(d.GetSources()) == 0 {
			target = nil
		}
	default:
		target = nil
	}
	own := target == card
	sources := d.GetSources()
	source := ""
	if st.source < len(sources) && sources[st.source].GetServerId() == card.GetServerId() {
		source = sources[st.source].GetMediaSourceId()
	}
	var trailer *libraryv1.Item
	var trailerURL string
	if info != nil {
		if i := slices.IndexFunc(info.extras, func(e *libraryv1.Item) bool { return e.GetExtra() == libraryv1.ExtraKind_EXTRA_KIND_TRAILER }); i >= 0 {
			trailer = info.extras[i]
		}
		// A television opens no web pages (docs/tv.md §1).
		if t := info.item.GetItem().GetRemoteTrailers(); len(t) > 0 && !tv && a.opts.OpenURL != nil {
			trailerURL = t[0]
		}
	}
	ui.Column(c).Gap(12).Margin(4, 0, 0).Children(func() {
		ui.Row(c).Gap(12).Wrap().AlignItems(ui.Center).Children(func() {
			if target != nil {
				var b ui.Element
				a.initialFocus(func() { b = a.playButton(c, label) })
				if b.Clicked() {
					opts := capiv1.PlaybackOptions_builder{}
					if own {
						opts.MediaSourceId = source
					}
					if position > 0 {
						opts.StartPositionMs = new(position)
					}
					a.play(target.GetServerId(), target.GetItemId(), opts.Build(), following...)
				}
				if position > 0 && a.overButton(c, "previous", style.T(c, "item.fromStart"), false).Clicked() {
					opts := capiv1.PlaybackOptions_builder{StartPositionMs: new(int64(0))}
					if own {
						opts.MediaSourceId = source
					}
					a.play(target.GetServerId(), target.GetItemId(), opts.Build(), following...)
				}
			}
			switch {
			case trailer != nil:
				if a.overButton(c, "movies", style.T(c, "item.trailer"), false).Clicked() {
					a.play(card.GetServerId(), trailer.GetId(), nil)
				}
			case trailerURL != "":
				if a.overButton(c, "movies", style.T(c, "item.trailer"), false).Clicked() {
					_ = a.opts.OpenURL(trailerURL)
				}
			}
			if a.overButton(c, "favorite", style.T(c, "item.favorite"), card.GetFavorite()).Clicked() {
				card.SetFavorite(!card.GetFavorite())
				a.setUserData(card, capiv1.Request_builder{SetFavorite: capiv1.SetFavorite_builder{ServerId: card.GetServerId(), ItemId: card.GetItemId(), Favorite: card.GetFavorite()}.Build()})
			}
			playedLabel := style.T(c, "item.markPlayed")
			if card.GetPlayed() {
				playedLabel = style.T(c, "item.markUnplayed")
			}
			if a.overButton(c, "played", playedLabel, card.GetPlayed()).Clicked() {
				card.SetPlayed(!card.GetPlayed())
				a.setUserData(card, capiv1.Request_builder{SetPlayed: capiv1.SetPlayed_builder{ServerId: card.GetServerId(), ItemId: card.GetItemId(), Played: card.GetPlayed()}.Build()})
			}
			if len(sources) > 1 {
				a.versionMenu(c, st, d)
			}
			if info != nil {
				a.moreMenu(c, st, card.GetServerId(), info.item.GetItem())
			}
		})
		if target == nil {
			return
		}
		var status []string
		if !own {
			status = append(status, target.GetTitle())
		}
		runtimeMs := target.GetRuntimeMs()
		if position > 0 && runtimeMs > 0 {
			status = append(status, style.T(c, "item.timeLeft", "time", runtime(time.Duration(runtimeMs-position)*time.Millisecond)))
		}
		if left := runtimeMs - position; runtimeMs > 0 && left > 0 {
			status = append(status, style.T(c, "item.endsAt", "time", c.Now().Add(time.Duration(left)*time.Millisecond).Format("15:04")))
		}
		ui.Row(c).GapX(12).GapY(4).Wrap().AlignItems(ui.Center).Children(func() {
			if position > 0 && runtimeMs > 0 {
				ui.Box(c).Width(160).Height(4).Radius(2).Clip().Background(white.Alpha(0.35)).Children(func() {
					ui.Box(c).Height(4).Radius(2).WidthPercent(min(float32(position)/float32(runtimeMs), 1) * 100).Background(white)
				})
			}
			style.Text(ui.Text(c, joinFacts(status)), style.Type(c).Footnote).TextColor(white.Alpha(0.8))
		})
	})
}

// playButton builds Play over artwork: white with black content, and on
// the television glass like the other buttons until it has focus, white
// belonging to focus (docs/tv.md §4).
func (a *App) playButton(c *ui.Context, label string) ui.Element {
	b := ui.ButtonBase(c.Key("play")).Height(44).Padding(0, 24).Gap(8).Radius(10).AlignItems(ui.Center).Label(label)
	icon := float32(20)
	fg, bg := ui.RGB(0, 0, 0), ui.RGB(255, 255, 255)
	if style.TV(c) {
		b.Height(64).Padding(0, 40)
		icon = 28
		if !a.tvFocus(c, b, false) {
			fg, bg = ui.RGB(255, 255, 255), ui.RGBA(255, 255, 255, 0.2)
		}
	} else if b.Hovered() {
		bg = bg.Alpha(0.9)
	}
	b.Background(bg)
	if b.Pressed() {
		b.Opacity(0.8)
	}
	b.Children(func() {
		ui.Icon(c, style.Icon("play", true)).Size(icon, icon).TextColor(fg)
		style.Text(ui.Text(c, label), style.Type(c).Headline).FontWeight(600).TextColor(fg).SingleLine()
	})
	return b
}

// setUserData sends a change of the user's data of card.
func (a *App) setUserData(card *capiv1.Card, req capiv1.Request_builder) {
	go func() {
		if _, err := a.opts.Core.Call(a.ctx, req.Build()); err != nil {
			a.opts.Update(func() { a.generation++ })
		}
	}()
}

// overButton builds a round button over artwork, 44 points wide, its icon
// filled while on, a favorite's red; on the television 64 points wide,
// white with black content with focus.
func (a *App) overButton(c *ui.Context, icon, label string, on bool) ui.Element {
	white := ui.RGB(255, 255, 255)
	b := ui.ButtonBase(c.Key(icon)).Size(44, 44).Radius(22).Center().Background(white.Alpha(0.2)).Label(label).Tooltip(label)
	fg, size := white, float32(20)
	if style.TV(c) {
		b.Size(64, 64).Radius(32)
		size = 28
		if a.tvFocus(c, b, false) {
			b.Background(white)
			fg = ui.RGB(0, 0, 0)
		}
	} else if b.Hovered() {
		b.Background(white.Alpha(0.3))
	}
	if b.Pressed() {
		b.Opacity(0.8)
	}
	if icon == "favorite" && on {
		fg = ui.RGB(0xFF, 0x45, 0x3A)
	}
	b.Children(func() {
		ui.Icon(c, style.Icon(icon, on)).Size(size, size).TextColor(fg)
	})
	return b
}

// versionMenu builds the pill choosing the version played.
func (a *App) versionMenu(c *ui.Context, st *detailsState, d *capiv1.Details) {
	white := ui.RGB(255, 255, 255)
	sources := d.GetSources()
	name := func(s *capiv1.Source) string {
		if s.GetServerId() != d.GetCard().GetServerId() {
			return style.T(c, "item.onServer", "server", s.GetServerName())
		}
		parts := []string{s.GetName()}
		if s.GetContainer() != "" {
			parts = append(parts, strings.ToUpper(s.GetContainer()))
		}
		if s.GetSize() > 0 {
			parts = append(parts, size(s.GetSize()))
		}
		return joinFacts(parts)
	}
	st.source = min(st.source, len(sources)-1)
	b := ui.ButtonBase(c.Key("version")).Height(44).Padding(0, 16).Gap(8).Radius(22).AlignItems(ui.Center).Background(white.Alpha(0.2)).Label(style.T(c, "item.versions"))
	font := style.Type(c).Callout
	fg, dim := white, white.Alpha(0.7)
	if style.TV(c) {
		b.Height(64).Padding(0, 24).Radius(32)
		font = style.Type(c).Headline
		if a.tvFocus(c, b, false) {
			b.Background(white)
			fg, dim = ui.RGB(0, 0, 0), ui.RGBA(0, 0, 0, 0.6)
		}
	} else if b.Hovered() {
		b.Background(white.Alpha(0.3))
	}
	b.Children(func() {
		style.Text(ui.Text(c, style.T(c, "item.version")), font).FontWeight(600).TextColor(dim).SingleLine()
		style.Text(ui.Text(c, sources[st.source].GetName()), font).FontWeight(600).TextColor(fg).SingleLine().MaxWidth(240)
		ui.Icon(c, style.Icon("expand", false)).Size(14, 14).TextColor(fg)
	})
	b.Menu(func(m *ui.Menu) {
		for i, s := range sources {
			if m.Item(name(s)).Checked(i == st.source).Chosen() {
				if s.GetServerId() != d.GetCard().GetServerId() {
					a.router.Push("/item/" + s.GetServerId() + "/" + s.GetItemId())
				} else {
					st.source = i
				}
			}
		}
	})
}

// size returns a size in bytes as "4.2 GB".
func size(b uint64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "kMGTPE"[exp])
}

// episodeItems returns the server's own episodes of a season by their
// IDs, which tell their overviews and air dates; none while they load.
func (a *App) episodeItems(st *detailsState, server, season string) map[string]*libraryv1.Item {
	if season == "" {
		return nil
	}
	t := load(a, &st.episodeInfo, key("episodes", server, season, a.generation), func(ctx context.Context) (map[string]*libraryv1.Item, error) {
		res, err := a.itemService(server).ListItems(ctx, libraryv1.ListItemsRequest_builder{
			ParentId: &season, Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_EPISODE},
		}.Build())
		if err != nil {
			return nil, err
		}
		items := map[string]*libraryv1.Item{}
		for _, it := range res.GetItems() {
			items[it.GetId()] = it
		}
		return items, nil
	})
	return t.value
}

// episodes builds the episodes of the item, as the web UI's: a series'
// seasons to choose from beside their title, and the chosen one's
// episodes in a row; a season's one under another; and the other
// episodes of an episode's season, from it on.
func (a *App) episodes(c *ui.Context, st *detailsState, d *capiv1.Details, info *itemInfo) {
	card := d.GetCard()
	server := card.GetServerId()
	colors := style.Colors(c)
	margin := style.Layout(c).Margin
	switch card.GetKind() {
	case capiv1.CardKind_CARD_KIND_SERIES:
		seasons := d.GetChildren()
		if len(seasons) == 0 {
			return
		}
		episodes := a.episodesOf(st, d)
		next := nextEpisode(episodes)
		season := seasons[st.season]
		items := a.episodeItems(st, server, season.GetItemId())
		ui.Column(c).Gap(12).Children(func() {
			ui.Row(c).Padding(0, margin).Gap(16).AlignItems(ui.Center).Children(func() {
				style.Text(ui.Text(c, style.T(c, "item.episodes")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).SingleLine().Shrink(0)
				if len(seasons) > 1 {
					a.seasonPills(c, st, card.GetItemId(), seasons)
				} else {
					style.Text(ui.Text(c, season.GetTitle()), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).SingleLine()
				}
				ui.Spacer(c)
				if a.moreLink(c, style.T(c, "home.more")).Clicked() {
					a.open(season)
				}
			})
			if len(episodes) == 0 {
				placeholderShelf(c, 4)
				return
			}
			a.cardRow(c, a.shelfScroll(key("episodes", season.GetItemId())), func() {
				for _, e := range episodes {
					a.episodeCard(c, e, items[e.GetItemId()], e == next)
				}
			})
		})
	case capiv1.CardKind_CARD_KIND_SEASON:
		episodes := d.GetChildren()
		if len(episodes) == 0 {
			return
		}
		next := nextEpisode(episodes)
		items := a.episodeItems(st, server, card.GetItemId())
		ui.Column(c).Gap(4).Children(func() {
			style.Text(ui.Text(c, style.T(c, "item.episodes")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).Margin(0, margin, 4)
			for _, e := range episodes {
				a.episodeRow(c, e, items[e.GetItemId()], e == next)
			}
		})
	case capiv1.CardKind_CARD_KIND_EPISODE:
		if info == nil {
			return
		}
		it := info.item.GetItem()
		season := it.GetSeasonId()
		if season == "" {
			return
		}
		t := load(a, &st.seasonDetail, key("season", server, season, a.generation), func(ctx context.Context) (*capiv1.Details, error) {
			return a.getDetails(ctx, server, season)
		})
		episodes := t.value.GetChildren()
		if len(episodes) < 2 {
			return
		}
		items := a.episodeItems(st, server, season)
		name := it.GetSeasonName()
		if name == "" {
			name = fmt.Sprint(it.GetParentIndexNumber())
		}
		scroll := a.shelfScroll(key("season", server, season, card.GetItemId()))
		ui.Column(c).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "item.moreFrom", "season", name)), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary)).Margin(0, margin)
			a.cardRow(c, scroll, func() {
				for _, e := range episodes {
					a.episodeCard(c, e, items[e.GetItemId()], e.GetItemId() == card.GetItemId())
				}
			})
		})
		// The row starts at the episode shown, as a season's list would.
		if !st.scrolledTo && scroll.MaxX > 0 {
			st.scrolledTo = true
			if i := slices.IndexFunc(episodes, func(e *capiv1.Card) bool { return e.GetItemId() == card.GetItemId() }); i > 0 {
				step := style.Layout(c).Thumbnail + style.Layout(c).Gutter*1.5
				if style.TV(c) {
					step = style.Layout(c).Thumbnail + style.Layout(c).Gutter
				}
				scroll.ScrollTo(min(float32(i)*step, scroll.MaxX), 0, ui.Animation{})
			}
		}
	}
}

// seasonPills builds a series' seasons to choose from, as pills in a row
// that scrolls without bars; on the television focus on a season's pill
// chooses it (docs/tv.md §6.2).
func (a *App) seasonPills(c *ui.Context, st *detailsState, id string, seasons []*capiv1.Card) {
	colors := style.Colors(c)
	scroll := a.shelfScroll(key("seasons", id))
	row := ui.ScrollHorizontal(c).TrackScroll(scroll).HideScrollbars().Shrink(1).MinWidth(0)
	row.Children(func() {
		a.inScrollRow(row.Bounds(), scroll, func() {
			a.inChoice(func() {
				ui.Row(c).Padding(4, 0).Gap(8).Role(ui.RoleTabList).Label(style.T(c, "item.seasons")).Children(func() {
					for i, s := range seasons {
						pill := ui.ButtonBase(c.Key(s.GetItemId())).Cursor(ui.CursorPointer).Label(s.GetTitle()).Selected(i == st.season)
						if style.TV(c) {
							focused := a.tvFocus(c, pill, i == st.season)
							pillLook(c, pill, s.GetTitle(), focused, i == st.season)
							if pill.Clicked() || focused {
								st.season = i
							}
							continue
						}
						pill.Height(32).Padding(0, 14).Radius(16).Center()
						fg := colors.TextPrimary
						switch {
						case i == st.season:
							pill.Background(style.Color(colors.Button))
							fg = colors.OnButton
						case pill.Hovered():
							pill.Background(style.Color(colors.FillSelected))
						default:
							pill.Background(style.Color(colors.Fill))
						}
						pill.Children(func() {
							style.Text(ui.Text(c, s.GetTitle()), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(fg)).SingleLine()
						})
						if pill.Clicked() {
							st.season = i
						}
					}
				})
			})
		})
	})
}

// moreLink builds a link of accent text at the end of a section's title,
// white with black text with focus on the television.
func (a *App) moreLink(c *ui.Context, label string) ui.Element {
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(label)).Shrink(0).Label(label)
	lit := a.tvLink(c, b, 6)
	b.Children(func() {
		t := style.Text(ui.Text(c, label), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.AccentText))
		switch {
		case lit:
			t.TextColor(ui.RGB(0, 0, 0))
		case b.Hovered():
			t.Underline()
		}
	})
	return b
}

// episodeCard builds an episode in a row, as the web UI's: its still with
// the progress and played mark, its numbers, the current one's accented,
// with its runtime and air date, its name, and three lines of overview.
func (a *App) episodeCard(c *ui.Context, e *capiv1.Card, it *libraryv1.Item, current bool) {
	colors := style.Colors(c)
	w := style.Layout(c).Thumbnail
	b := ui.ButtonBase(c.Key(e.GetItemId())).Cursor(ui.CursorPointer).Column().Width(w).Gap(2).AlignItems(ui.Start).Label(e.GetTitle())
	hovered := a.cardFocus(c, b) || b.Hovered()
	if b.Pressed() {
		b.Opacity(0.8)
	}
	b.Children(func() {
		a.episodeStill(c, e, w, hovered)
		ui.Row(c).Gap(8).Margin(6, 0, 0).Width(w).AlignItems(ui.Center).Children(func() {
			if number := episodeNumber(c, e, it); number != "" {
				n := style.Text(ui.Text(c, number), style.Type(c).Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary)).Shrink(0)
				if current {
					n.TextColor(style.Color(colors.AccentText))
				}
			}
			style.Text(ui.Text(c, episodeFacts(e, it)), style.Type(c).Footnote).TextColor(style.Color(colors.TextTertiary)).SingleLine().MinWidth(0)
		})
		style.Text(ui.Text(c, e.GetTitle()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(w)
		if o := it.GetOverview(); o != "" {
			style.Text(ui.Text(c, o), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).MaxLines(3).Width(w).Margin(2, 0, 0)
		}
	})
	if b.Clicked() {
		a.open(e)
	}
}

// episodeStill builds an episode's still width wide, with a bar of its
// progress inset at its foot and the mark of a played one.
func (a *App) episodeStill(c *ui.Context, e *capiv1.Card, width float32, hovered bool) {
	white := ui.RGB(255, 255, 255)
	ui.Box(c).Children(func() {
		a.artwork(c, e.GetThumb(), "shows", nil, width, width*9/16, hovered)
		if e.HasProgress() && e.GetProgress() > 0 && !e.GetPlayed() {
			ui.Box(c).Absolute().Left(10).Right(10).Bottom(10).Height(4).Radius(2).Clip().Background(white.Alpha(0.35)).Children(func() {
				ui.Box(c).Height(4).Radius(2).WidthPercent(min(e.GetProgress(), 1) * 100).Background(white)
			})
		}
		if e.GetPlayed() {
			ui.Box(c).Absolute().Top(6).Right(6).Size(22, 22).Radius(11).Background(style.Color(style.Colors(c).Scrim)).Center().Children(func() {
				ui.Icon(c, style.Icon("played", true)).Size(18, 18).TextColor(white)
			})
		}
	})
}

// episodeNumber returns an episode's short numbers, as "S1 E2".
func episodeNumber(c *ui.Context, e *capiv1.Card, it *libraryv1.Item) string {
	if it.GetIndexNumber() > 0 {
		return style.T(c, "item.episodeShort", "season", it.GetParentIndexNumber(), "episode", it.GetIndexNumber())
	}
	if m := episodeNumbers.FindStringSubmatch(e.GetSubtitle()); m != nil {
		return style.T(c, "item.episodeShort", "season", m[1], "episode", m[2])
	}
	return ""
}

// episodeFacts returns an episode's runtime and air date.
func episodeFacts(e *capiv1.Card, it *libraryv1.Item) string {
	var facts []string
	if e.GetRuntimeMs() > 0 {
		facts = append(facts, runtime(time.Duration(e.GetRuntimeMs())*time.Millisecond))
	}
	if it.HasPremiereDate() {
		facts = append(facts, it.GetPremiereDate().AsTime().Format("Jan 2, 2006"))
	}
	return joinFacts(facts)
}

// episodeRow builds an episode in a season's list, as the web UI's: its
// still, then its numbers, name, runtime and air date, and three lines of
// overview.
func (a *App) episodeRow(c *ui.Context, e *capiv1.Card, it *libraryv1.Item, current bool) {
	colors := style.Colors(c)
	margin := style.Layout(c).Margin
	w := min(max(a.contentWidth*0.32, 140), style.Layout(c).Thumbnail)
	b := ui.ButtonBase(c.Key(e.GetItemId())).Cursor(ui.CursorPointer).Gap(20).Padding(12, margin).AlignItems(ui.Start).Label(e.GetTitle())
	if a.tvFocusLit(c, b, false) {
		b.Margin(0, margin/2).Padding(12, margin/2).Radius(design.RadiusLg)
		colors = &design.LightColors
	} else if b.Hovered() {
		b.Background(style.Color(colors.Surface))
	}
	b.Children(func() {
		a.episodeStill(c, e, w, false)
		ui.Column(c).Grow(1).MinWidth(0).Padding(4, 0).Children(func() {
			n := style.Text(ui.Text(c, episodeNumber(c, e, it)), style.Type(c).Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
			if current {
				n.TextColor(style.Color(colors.AccentText))
			}
			style.Text(ui.Text(c, e.GetTitle()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary))
			style.Text(ui.Text(c, episodeFacts(e, it)), style.Type(c).Footnote).TextColor(style.Color(colors.TextTertiary))
			if o := it.GetOverview(); o != "" {
				style.Text(ui.Text(c, o), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).MaxLines(3).MaxWidth(760).Margin(6, 0, 0)
			}
		})
	})
	if b.Clicked() {
		a.open(e)
	}
}

func initials(name string) string {
	var s []rune
	for _, f := range strings.Fields(name) {
		s = append(s, []rune(strings.ToUpper(f))[0])
		if len(s) == 2 {
			break
		}
	}
	return string(s)
}

// facts builds the details at the foot: the overview, then the facts and
// the chosen version's file, each a caption over its value.
func (a *App) facts(c *ui.Context, st *detailsState, d *capiv1.Details) {
	colors := style.Colors(c)
	type fact struct{ caption, value string }
	var facts []fact
	if d.HasPremiereMs() {
		caption := "item.released"
		if d.GetCard().GetKind() == capiv1.CardKind_CARD_KIND_SERIES {
			caption = "item.firstAired"
		}
		facts = append(facts, fact{style.T(c, caption), time.UnixMilli(d.GetPremiereMs()).Format("January 2, 2006")})
	}
	if ms := d.GetCard().GetRuntimeMs(); ms > 0 {
		facts = append(facts, fact{style.T(c, "item.runtime"), runtime(time.Duration(ms) * time.Millisecond)})
	}
	if r := d.GetOfficialRating(); r != "" {
		facts = append(facts, fact{style.T(c, "item.rated"), r})
	}
	if g := d.GetGenres(); len(g) > 0 {
		facts = append(facts, fact{style.T(c, "item.genres"), strings.Join(g, ", ")})
	}
	if sources := d.GetSources(); st.source < len(sources) && sources[st.source].GetServerId() == d.GetCard().GetServerId() {
		s := sources[st.source]
		if s.GetContainer() != "" || s.GetSize() > 0 {
			var parts []string
			if s.GetContainer() != "" {
				parts = append(parts, strings.ToUpper(s.GetContainer()))
			}
			if s.GetSize() > 0 {
				parts = append(parts, size(s.GetSize()))
			}
			facts = append(facts, fact{style.T(c, "item.file"), joinFacts(parts)})
		}
		var audio []string
		for _, as := range s.GetAudio() {
			parts := []string{}
			for _, p := range []string{as.GetTitle(), as.GetLanguage(), strings.ToUpper(as.GetCodec())} {
				if p != "" {
					parts = append(parts, p)
				}
			}
			audio = append(audio, joinFacts(parts))
		}
		if len(audio) > 0 {
			facts = append(facts, fact{style.T(c, "item.audioTracks"), strings.Join(audio, "\n")})
		}
	}
	if len(facts) == 0 && d.GetOverview() == "" {
		return
	}
	ui.Column(c).Padding(0, style.Layout(c).Margin).Gap(16).Children(func() {
		style.Text(ui.Text(c, style.T(c, "item.details")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
		ui.Row(c).Gap(48).AlignItems(ui.Start).Wrap().Children(func() {
			if d.GetOverview() != "" {
				ui.Column(c).Basis(320).Grow(1).MaxWidth(640).Gap(8).Children(func() {
					if d.GetTagline() != "" {
						style.Text(ui.Text(c, d.GetTagline()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary))
					}
					style.Text(ui.Text(c, d.GetOverview()), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary)).Selectable()
				})
			}
			ui.Column(c).Basis(240).Grow(1).Gap(12).Children(func() {
				for _, f := range facts {
					ui.Column(c).Gap(2).Children(func() {
						style.Text(ui.Text(c, f.caption), style.Type(c).Caption).TextColor(style.Color(colors.TextSecondary))
						style.Text(ui.Text(c, f.value), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
					})
				}
			})
		})
	})
}

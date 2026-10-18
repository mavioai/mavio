package app

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	design "github.com/mavioai/mavio/libs/design/gen/go"

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
}

// details builds an item's details.
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
		ui.Column(c).Gap(32).Padding(0, 0, 40).Children(func() {
			a.detailsHeader(c, st, d)
			a.episodes(c, st, d)
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
				a.about(c, st, d, info.value, credits)
			} else {
				a.facts(c, st, d)
			}
		})
	})
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

// detailsHeader builds the top of the details: the artwork with the
// title, facts, overview and actions over its lower left.
func (a *App) detailsHeader(c *ui.Context, st *detailsState, d *capiv1.Details) {
	card := d.GetCard()
	_, wh := c.Size()
	height := min(max(wh*0.7, 520), 700)
	// On the television, the backdrop fills 86 % of the screen, under the
	// name at 72 points, two lines of overview and no tagline, the text at
	// most 960 wide (docs/tv.md §6.2).
	tv := style.TV(c)
	textWidth, logoW, logoH := min(640, a.contentWidth*0.6), float32(400), float32(120)
	if tv {
		height, textWidth, logoW, logoH = wh*0.86, min(960, a.contentWidth*0.6), 600, 180
	}
	white := ui.RGB(255, 255, 255)
	backdrop := d.GetBackdrop()
	if backdrop.GetUrl() == "" {
		backdrop = card.GetThumb()
	}
	blurred := backdrop.GetUrl() == ""
	if blurred {
		backdrop = card.GetPoster()
	}
	ui.Box(c).Height(height).Clip().Background(style.Color(design.DarkColors.SurfaceRaised)).DragWindow().Children(func() {
		if bitmap := a.opts.Images.Get(backdrop.GetUrl(), a.contentWidth, scale); bitmap != nil {
			img := ui.Image(c, bitmap).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover)
			if blurred {
				img.Blur(40)
			}
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 55,
			Stops: []ui.GradientStop{{At: 0, Color: ui.RGBA(0, 0, 0, 0.9)}, {At: 0.5, Color: ui.RGBA(0, 0, 0, 0.45)}, {At: 1, Color: ui.RGBA(0, 0, 0, 0)}},
		})
		ui.Column(c).Absolute().Left(style.Layout(c).Margin).Bottom(32).MaxWidth(textWidth).Gap(10).Children(func() {
			if card.GetKind() == capiv1.CardKind_CARD_KIND_EPISODE && card.GetSubtitle() != "" {
				style.Text(ui.Text(c, card.GetSubtitle()), style.Type(c).Headline).TextColor(white.Alpha(0.85)).SingleLine()
			}
			if logo := a.opts.Images.Get(d.GetLogo().GetUrl(), logoW, scale); logo != nil {
				ui.Image(c, logo).MaxWidth(logoW).MaxHeight(logoH).Fit(ui.Contain).AlignSelf(ui.Start)
			} else {
				title := style.Text(ui.Text(c, card.GetTitle()), style.Type(c).Display).TextColor(white).MaxLines(2)
				if tv {
					title.FontSize(72).LineHeight(1.1)
				}
			}
			if facts := detailFacts(c, d); facts != "" {
				style.Text(ui.Text(c, facts), style.Type(c).Callout).TextColor(white.Alpha(0.85))
			}
			ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
				if d.HasCommunityRating() {
					ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, style.Icon("rating", true)).Size(14, 14).TextColor(style.Color(design.DarkColors.Warning))
						style.Text(ui.Text(c, fmt.Sprintf("%.1f", d.GetCommunityRating())), style.Type(c).Callout).TextColor(white)
					})
				}
				if r := d.GetOfficialRating(); r != "" {
					style.Text(ui.Text(c, r), style.Type(c).Caption).TextColor(white).Padding(1, 6).Radius(design.RadiusSm).Border(1, white.Alpha(0.8))
				}
			})
			if d.GetTagline() != "" && !tv {
				style.Text(ui.Text(c, d.GetTagline()), style.Type(c).Headline).TextColor(white)
			}
			if d.GetOverview() != "" {
				o := style.Text(ui.Text(c, d.GetOverview()), style.Type(c).Body).TextColor(white.Alpha(0.9))
				if tv {
					o.MaxLines(2)
				} else if !st.overviewOpen {
					o.MaxLines(3).Cursor(ui.CursorPointer).Tooltip(style.T(c, "item.more"))
				}
				if o.Clicked() {
					st.overviewOpen = !st.overviewOpen
				}
			}
			a.actions(c, st, d)
		})
	})
}

// detailFacts returns the line of an item's facts.
func detailFacts(c *ui.Context, d *capiv1.Details) string {
	card := d.GetCard()
	var facts []string
	switch card.GetKind() {
	case capiv1.CardKind_CARD_KIND_SERIES:
		if card.GetYear() > 0 {
			facts = append(facts, fmt.Sprintf("%d–", card.GetYear()))
		}
		if n := len(d.GetChildren()); n > 0 {
			facts = append(facts, style.T(c, "item.seasonCount", "count", n))
		}
	case capiv1.CardKind_CARD_KIND_SEASON:
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
	if card.GetRuntimeMs() > 0 {
		facts = append(facts, runtime(time.Duration(card.GetRuntimeMs())*time.Millisecond))
	}
	genres := d.GetGenres()
	facts = append(facts, genres[:min(len(genres), 3)]...)
	return joinFacts(facts)
}

// actions builds the details' actions and the status line under them.
func (a *App) actions(c *ui.Context, st *detailsState, d *capiv1.Details) {
	card := d.GetCard()
	white := ui.RGB(255, 255, 255)
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
			if target.HasProgress() && target.GetProgress() > 0 {
				label = style.T(c, "item.resumeEpisode", "episode", name)
				position = int64(float64(target.GetRuntimeMs()) * float64(target.GetProgress()))
			}
		}
	case capiv1.CardKind_CARD_KIND_MOVIE, capiv1.CardKind_CARD_KIND_EPISODE, capiv1.CardKind_CARD_KIND_VIDEO, capiv1.CardKind_CARD_KIND_MUSIC_VIDEO:
		label = style.T(c, "item.play")
		if position > 0 {
			label = style.T(c, "item.resume")
		}
	default:
		target = nil
	}
	sources := d.GetSources()
	source := ""
	if st.source < len(sources) && sources[st.source].GetServerId() == card.GetServerId() {
		source = sources[st.source].GetMediaSourceId()
	}
	ui.Row(c).Gap(12).Margin(8, 0, 0).AlignItems(ui.Center).Children(func() {
		if target != nil {
			if a.heroButton(c, "play", label, true).Clicked() {
				opts := capiv1.PlaybackOptions_builder{MediaSourceId: source}
				if target == card {
					opts.StartPositionMs = new(position)
				}
				a.play(target.GetServerId(), target.GetItemId(), opts.Build(), following...)
			}
			if position > 0 && target == card && a.overButton(c, "retry", style.T(c, "item.fromStart"), false).Clicked() {
				a.play(card.GetServerId(), card.GetItemId(), capiv1.PlaybackOptions_builder{MediaSourceId: source, StartPositionMs: new(int64(0))}.Build())
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
	})
	if target == card && position > 0 && card.GetRuntimeMs() > 0 {
		left := time.Duration(card.GetRuntimeMs()-position) * time.Millisecond
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Width(120).Height(4).Radius(2).Background(white.Alpha(0.3)).Children(func() {
				ui.Box(c).Height(4).Radius(2).WidthPercent(float32(position) / float32(card.GetRuntimeMs()) * 100).Background(style.Color(design.DarkColors.Progress))
			})
			style.Text(ui.Text(c, style.T(c, "item.timeLeft", "time", runtime(left))+" · "+style.T(c, "item.endsAt", "time", c.Now().Add(left).Format("15:04"))), style.Type(c).Footnote).TextColor(white.Alpha(0.85))
		})
	}
}

// setUserData sends a change of the user's data of card.
func (a *App) setUserData(card *capiv1.Card, req capiv1.Request_builder) {
	go func() {
		if _, err := a.opts.Core.Call(a.ctx, req.Build()); err != nil {
			a.opts.Update(func() { a.generation++ })
		}
	}()
}

// overButton builds a round button over artwork; on the television 64
// points wide, white with black content with focus.
func (a *App) overButton(c *ui.Context, icon, label string, on bool) ui.Element {
	white := ui.RGB(255, 255, 255)
	b := ui.ButtonBase(c.Key(icon)).Size(40, 40).Radius(20).Center().Background(white.Alpha(0.22)).Label(label).Tooltip(label)
	fg, size := white, float32(20)
	if style.TV(c) {
		b.Size(64, 64).Radius(32)
		size = 28
		if a.tvFocus(c, b, false) {
			b.Background(white)
			fg = ui.RGB(0, 0, 0)
		}
	} else if b.Hovered() {
		b.Background(white.Alpha(0.32))
	}
	if b.Pressed() {
		b.Opacity(0.8)
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
	b := ui.ButtonBase(c).Height(40).Padding(0, 16).Gap(6).Radius(20).AlignItems(ui.Center).Background(white.Alpha(0.22)).Label(style.T(c, "item.versions"))
	b.Children(func() {
		style.Text(ui.Text(c, style.T(c, "item.version")+" "+sources[st.source].GetName()), style.Type(c).Callout).TextColor(white).SingleLine().MaxWidth(200)
		ui.Icon(c, style.Icon("selector", false)).Size(14, 14).TextColor(white)
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

// episodes builds a series' seasons and the episodes of the one shown, or
// a season's episodes.
func (a *App) episodes(c *ui.Context, st *detailsState, d *capiv1.Details) {
	card := d.GetCard()
	colors := style.Colors(c)
	switch card.GetKind() {
	case capiv1.CardKind_CARD_KIND_SERIES:
		seasons := d.GetChildren()
		if len(seasons) == 0 {
			return
		}
		episodes := a.episodesOf(st, d)
		next := nextEpisode(episodes)
		ui.Column(c).Gap(12).Children(func() {
			ui.ScrollHorizontal(c).Children(func() {
				a.inChoice(func() {
					ui.Row(c).Padding(0, style.Layout(c).Margin).Gap(8).Children(func() {
						for i, s := range seasons {
							pill := ui.ButtonBase(c.Key(s.GetItemId())).Cursor(ui.CursorPointer).Label(s.GetTitle())
							// On the television, focus on a season's pill
							// chooses it (docs/tv.md §6.2).
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
			if len(episodes) == 0 {
				placeholderShelf(c, 4)
				return
			}
			a.shelf(c, key("episodes", seasons[st.season].GetItemId()), style.T(c, "item.episodes"), func() { a.open(seasons[st.season]) }, func() {
				for _, e := range episodes {
					a.episodeCard(c, e, e == next)
				}
			})
		})
	case capiv1.CardKind_CARD_KIND_SEASON:
		episodes := d.GetChildren()
		next := nextEpisode(episodes)
		ui.Column(c).Padding(0, style.Layout(c).Margin).Gap(16).Children(func() {
			style.Text(ui.Text(c, style.T(c, "item.episodes")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			for _, e := range episodes {
				a.episodeRow(c, e, e == next)
			}
		})
	}
}

// episodeCard builds an episode in a series' row.
func (a *App) episodeCard(c *ui.Context, e *capiv1.Card, next bool) {
	colors := style.Colors(c)
	w := style.Layout(c).Thumbnail * 0.85
	b := ui.ButtonBase(c.Key(e.GetItemId())).Cursor(ui.CursorPointer).Column().Width(w).Gap(4).AlignItems(ui.Start).Label(e.GetTitle())
	hovered := a.cardFocus(c, b) || b.Hovered()
	b.Children(func() {
		a.artwork(c, e.GetThumb(), "", e, w, w*9/16, hovered)
		number := episodeName(c, e)
		if e.GetRuntimeMs() > 0 {
			number += " · " + runtime(time.Duration(e.GetRuntimeMs())*time.Millisecond)
		}
		n := style.Text(ui.Text(c, number), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary)).Margin(4, 0, 0)
		if next {
			n.TextColor(style.Color(colors.AccentText))
		}
		style.Text(ui.Text(c, e.GetTitle()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary)).SingleLine().Width(w)
	})
	if b.Clicked() {
		a.open(e)
	}
}

// episodeRow builds an episode in a season's list.
func (a *App) episodeRow(c *ui.Context, e *capiv1.Card, next bool) {
	colors := style.Colors(c)
	w := min(max(a.contentWidth/3, 140), style.Layout(c).Thumbnail)
	b := ui.ButtonBase(c.Key(e.GetItemId())).Cursor(ui.CursorPointer).Gap(16).Padding(8).Radius(design.RadiusLg).AlignItems(ui.Start).Label(e.GetTitle())
	if a.tvFocusLit(c, b, false) {
		colors = &design.LightColors
	} else if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		a.artwork(c, e.GetThumb(), "", e, w, w*9/16, false)
		ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
			n := style.Text(ui.Text(c, episodeName(c, e)), style.Type(c).Footnote).FontWeight(600).TextColor(style.Color(colors.TextSecondary))
			if next {
				n.TextColor(style.Color(colors.AccentText))
			}
			style.Text(ui.Text(c, e.GetTitle()), style.Type(c).Headline).TextColor(style.Color(colors.TextPrimary))
			if e.GetRuntimeMs() > 0 {
				style.Text(ui.Text(c, runtime(time.Duration(e.GetRuntimeMs())*time.Millisecond)), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
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

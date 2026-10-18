package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// discoveredState is the state of the pages of discovered people and
// collections.
type discoveredState struct {
	people      map[string]*task[*acquisitionv1.Person]
	collections map[string]*task[*acquisitionv1.Collection]
	ratings     map[string]*task[[]*pluginv1.Rating]
	allStudios  bool // a title's studios shown past minStudios
	kinds       int  // the kinds of a person's titles shown: all, movies or series
	more        bool // a person's whole biography shown

	asking   bool // the dialog choosing a collection's movies is open
	chosen   map[int32]bool
	blocking bool
	err      error
}

// taskIn returns the task of m under k, made when there is none.
func taskIn[T any](m *map[string]*task[T], k string) *task[T] {
	if *m == nil {
		*m = map[string]*task[T]{}
	}
	t := (*m)[k]
	if t == nil {
		t = &task[T]{}
		(*m)[k] = t
	}
	return t
}

// discoveredPersonPath returns the path of a discovered person's page.
func discoveredPersonPath(server, tmdb string) string {
	return "/discover/person/" + server + "/" + tmdb
}

// collectionPath returns the path of a discovered collection's page.
func collectionPath(server string, tmdb int32) string {
	return "/discover/collection/" + server + "/" + strconv.Itoa(int(tmdb))
}

// tmdbParam returns a route's TMDB ID.
func tmdbParam(r *ui.Route) int32 {
	n, _ := strconv.ParseInt(r.Param("tmdb"), 10, 32)
	return int32(n)
}

// discoveredPersonPage builds a discovered person's page, as Seerr's: their
// photo, dates, other names, biography and links, then the titles they
// play in and work on, the department they are known for first.
func (a *App) discoveredPersonPage(c *ui.Context, r *ui.Route) {
	server, tmdb := r.Param("server"), tmdbParam(r)
	st := &a.discoveredState
	language := discoverLanguage(c)
	t := load(a, taskIn(&st.people, key(server, tmdb)), key("dperson", server, tmdb, language, a.generation, a.discoverState.reload), func(ctx context.Context) (*acquisitionv1.Person, error) {
		res, err := a.discoverService(server).GetPerson(ctx, acquisitionv1.GetPersonRequest_builder{TmdbId: &tmdb, Language: &language}.Build())
		return res.GetPerson(), err
	})
	if taskStatus(c, t, func() { st.people[key(server, tmdb)].key = "" }, nil) {
		return
	}
	p := t.value
	if p == nil {
		statusView(c, "error", style.T(c, "state.notFound"), style.T(c, "state.notFoundMessage"), nil)
		return
	}
	r.Title(p.GetName())
	of := func(ts []*acquisitionv1.PersonTitle) []*acquisitionv1.PersonTitle {
		var out []*acquisitionv1.PersonTitle
		for _, pt := range ts {
			series := pt.GetTitle().GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES
			if pt.GetTitle() != nil && (st.kinds == 0 || series == (st.kinds == 2)) {
				out = append(out, pt)
			}
		}
		return out
	}
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Gap(32).Padding(0, 0, 40).Children(func() {
			a.discoveredPersonHeader(c, p)
			cast := func() { a.personTitleGrid(c, server, style.T(c, "person.appearances"), of(p.GetCast())) }
			crew := func() { a.personTitleGrid(c, server, style.T(c, "person.crew"), of(p.GetCrew())) }
			if d := p.GetKnownForDepartment(); d == "" || d == "Acting" {
				cast()
				crew()
			} else {
				crew()
				cast()
			}
		})
	})
}

func (a *App) discoveredPersonHeader(c *ui.Context, p *acquisitionv1.Person) {
	colors := style.Colors(c)
	st := &a.discoveredState
	bar := c.TitleBar()
	const w = 176
	ui.Row(c).Padding(max(bar.Height, 12)+20, style.Layout.Margin, 0).Gap(24).AlignItems(ui.Start).Children(func() {
		ui.Box(c).Size(w, w).Shrink(0).Radius(w / 2).Clip().Background(style.Color(colors.Fill)).Center().Children(func() {
			if bitmap := a.opts.Images.Get(sized(p.GetImageUrl(), 342), w, scale); bitmap != nil {
				ui.Image(c, bitmap).Fill().Fit(ui.Cover)
			} else {
				noArtwork(c, "profile", w)
			}
		})
		ui.Column(c).Grow(1).MinWidth(0).MaxWidth(768).Gap(12).AlignItems(ui.Start).Children(func() {
			style.Text(ui.Text(c, p.GetName()), style.Type.Title1).TextColor(style.Color(colors.TextPrimary)).Selectable()
			var facts []string
			if born, err := parseDay(p.GetBirthday()); err == nil {
				end := time.Now()
				if died, err := parseDay(p.GetDeathday()); err == nil {
					end = died
				}
				facts = append(facts, style.T(c, "person.born", "date", longDate(born), "age", yearsBetween(born, end)))
			}
			if place := p.GetPlaceOfBirth(); place != "" {
				facts = append(facts, place)
			}
			if died, err := parseDay(p.GetDeathday()); err == nil {
				facts = append(facts, style.T(c, "person.died", "date", longDate(died)))
			}
			if len(facts) > 0 {
				style.Text(ui.Text(c, strings.Join(facts, " | ")), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
			}
			if names := p.GetAlsoKnownAs(); len(names) > 0 {
				style.Text(ui.Text(c, style.T(c, "person.alsoKnownAs", "names", strings.Join(names, ", "))), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
			}
			var links []externalLink
			if id := p.GetExternalIds()["tmdb"]; id != "" {
				links = append(links, externalLink{"TMDB", "https://www.themoviedb.org/person/" + id})
			}
			if id := p.GetExternalIds()["imdb"]; id != "" {
				links = append(links, externalLink{"IMDb", "https://www.imdb.com/name/" + id})
			}
			a.externalLinks(c, links)
			if bio := p.GetBiography(); bio != "" {
				ui.Column(c).Gap(4).AlignItems(ui.Start).Children(func() {
					text := style.Text(ui.Text(c, bio), style.Type.Body).TextColor(style.Color(colors.TextPrimary)).Selectable()
					if !st.more {
						text.MaxLines(6)
					}
					label := style.T(c, "person.more")
					if st.more {
						label = style.T(c, "person.less")
					}
					if linkButton(c, label).Clicked() {
						st.more = !st.more
					}
				})
			}
			ui.Segmented(c, &st.kinds, style.T(c, "person.allKinds"), style.T(c, "person.movies"), style.T(c, "person.series")).Width(300)
		})
	})
}

// externalLink is a link to a title or person elsewhere.
type externalLink struct{ name, url string }

// externalLinks builds links elsewhere as pills, as the library person's.
func (a *App) externalLinks(c *ui.Context, links []externalLink) {
	if len(links) == 0 || a.opts.OpenURL == nil {
		return
	}
	colors := style.Colors(c)
	ui.Row(c).Gap(8).Wrap().Children(func() {
		for i, l := range links {
			b := ui.ButtonBase(c.Key(i)).Height(30).Padding(0, 14).Gap(6).Radius(design.RadiusFull).AlignItems(ui.Center).
				Background(style.Color(colors.Fill)).Label(l.name)
			if b.Hovered() {
				b.Background(style.Color(colors.FillSelected))
			}
			b.Children(func() {
				style.Text(ui.Text(c, l.name), style.Type.Footnote).FontWeight(600).TextColor(style.Color(colors.TextPrimary))
				ui.Icon(c, style.Icon("external", false)).Size(14, 14).TextColor(style.Color(colors.TextSecondary))
			})
			if b.Clicked() {
				_ = a.opts.OpenURL(l.url)
			}
		}
	})
}

// personTitleGrid builds a section of a discovered person's titles, each
// with the role they have in it.
func (a *App) personTitleGrid(c *ui.Context, server, title string, titles []*acquisitionv1.PersonTitle) {
	if len(titles) == 0 {
		return
	}
	colors := style.Colors(c)
	width, _ := a.gridColumns(c)
	ui.Column(c.Key(title)).Gap(12).Children(func() {
		ui.Row(c).Padding(0, style.Layout.Margin).Gap(8).AlignItems(ui.End).Children(func() {
			style.Text(ui.Text(c, title), style.Type.Title3).TextColor(style.Color(colors.TextPrimary))
			style.Text(ui.Text(c, fmt.Sprint(len(titles))), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
		})
		ui.Row(c).Padding(0, style.Layout.Margin).GapX(style.Layout.Gutter).GapY(24).Wrap().Children(func() {
			for i, pt := range titles {
				a.discoveredCard(c.Key(i), server, pt.GetTitle(), width, pt.GetRole())
			}
		})
	})
}

// collectionStatus returns what the server has of a collection, as Seerr
// marks it: available when it has every movie, partially available when
// some, requested when every other movie is requested, blocklisted when
// every movie is.
func collectionStatus(parts []*acquisitionv1.DiscoveredTitle) acquisitionv1.MediaStatus {
	if len(parts) == 0 {
		return acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED
	}
	every := func(f func(s acquisitionv1.MediaStatus) bool) bool {
		for _, p := range parts {
			if !f(p.GetStatus()) {
				return false
			}
		}
		return true
	}
	is := func(want ...acquisitionv1.MediaStatus) func(acquisitionv1.MediaStatus) bool {
		return func(s acquisitionv1.MediaStatus) bool {
			for _, w := range want {
				if s == w {
					return true
				}
			}
			return false
		}
	}
	has := is(acquisitionv1.MediaStatus_MEDIA_STATUS_AVAILABLE, acquisitionv1.MediaStatus_MEDIA_STATUS_PARTIALLY_AVAILABLE)
	switch {
	case every(is(acquisitionv1.MediaStatus_MEDIA_STATUS_BLOCKLISTED)):
		return acquisitionv1.MediaStatus_MEDIA_STATUS_BLOCKLISTED
	case every(is(acquisitionv1.MediaStatus_MEDIA_STATUS_AVAILABLE)):
		return acquisitionv1.MediaStatus_MEDIA_STATUS_AVAILABLE
	case !every(func(s acquisitionv1.MediaStatus) bool { return !has(s) }):
		return acquisitionv1.MediaStatus_MEDIA_STATUS_PARTIALLY_AVAILABLE
	case every(is(acquisitionv1.MediaStatus_MEDIA_STATUS_REQUESTED, acquisitionv1.MediaStatus_MEDIA_STATUS_WANTED)):
		return acquisitionv1.MediaStatus_MEDIA_STATUS_REQUESTED
	}
	return acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED
}

// collection loads a collection of a server.
func (a *App) collection(c *ui.Context, server string, tmdb int32) *task[*acquisitionv1.Collection] {
	st := &a.discoveredState
	language := discoverLanguage(c)
	return load(a, taskIn(&st.collections, key(server, tmdb)), key("collection", server, tmdb, language, a.generation, a.discoverState.reload), func(ctx context.Context) (*acquisitionv1.Collection, error) {
		res, err := a.discoverService(server).GetCollection(ctx, acquisitionv1.GetCollectionRequest_builder{TmdbCollectionId: &tmdb, Language: &language}.Build())
		return res.GetCollection(), err
	})
}

// collectionPage builds a discovered collection, as Seerr's: its backdrop,
// poster, name and overview, Request asking for the movies of it chosen,
// and for administrators Blocklist, then its movies.
func (a *App) collectionPage(c *ui.Context, r *ui.Route) {
	server, tmdb := r.Param("server"), tmdbParam(r)
	t := a.collection(c, server, tmdb)
	if taskStatus(c, t, func() { a.discoveredState.collections[key(server, tmdb)].key = "" }, nil) {
		return
	}
	col := t.value
	if col == nil {
		statusView(c, "error", style.T(c, "state.notFound"), style.T(c, "state.notFoundMessage"), nil)
		return
	}
	r.Title(col.GetName())
	colors := style.Colors(c)
	width, _ := a.gridColumns(c)
	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Gap(32).Padding(0, 0, 40).Children(func() {
			a.collectionHeader(c, server, tmdb, col)
			ui.Column(c).Gap(12).Children(func() {
				style.Text(ui.Text(c, style.T(c, "collection.movies")), style.Type.Title3).TextColor(style.Color(colors.TextPrimary)).Margin(0, style.Layout.Margin)
				ui.Row(c).Padding(0, style.Layout.Margin).GapX(style.Layout.Gutter).GapY(24).Wrap().Children(func() {
					for i, p := range col.GetParts() {
						a.discoveredCard(c.Key(i), server, p, width, "")
					}
				})
			})
		})
	})
	a.collectionDialog(c, server, col)
}

func (a *App) collectionHeader(c *ui.Context, server string, tmdb int32, col *acquisitionv1.Collection) {
	st := &a.discoveredState
	_, wh := c.Size()
	height := min(max(wh*0.6, 440), 600)
	white := ui.RGB(255, 255, 255)
	status := collectionStatus(col.GetParts())
	blocked := status == acquisitionv1.MediaStatus_MEDIA_STATUS_BLOCKLISTED
	requestable := false
	for _, p := range col.GetParts() {
		requestable = requestable || p.GetRequestable()
	}
	ui.Box(c).Height(height).Clip().Background(style.Color(design.DarkColors.SurfaceRaised)).DragWindow().Children(func() {
		if bitmap := a.opts.Images.Get(sized(col.GetBackdropUrl(), 1280), a.contentWidth, scale); bitmap != nil {
			ui.Image(c, bitmap).Absolute().Top(0).Left(0).Right(0).Bottom(0).Fit(ui.Cover)
		}
		ui.Box(c).Absolute().Top(0).Left(0).Right(0).Bottom(0).LinearGradient(ui.LinearGradient{
			Angle: 55,
			Stops: []ui.GradientStop{{At: 0, Color: ui.RGBA(0, 0, 0, 0.9)}, {At: 0.5, Color: ui.RGBA(0, 0, 0, 0.45)}, {At: 1, Color: ui.RGBA(0, 0, 0, 0)}},
		})
		ui.Row(c).Absolute().Left(40).Right(40).Bottom(40).Gap(40).AlignItems(ui.End).Children(func() {
			if poster := a.opts.Images.Get(sized(col.GetPosterUrl(), 500), 200, scale); poster != nil {
				ui.Box(c).Width(200).Height(300).Shrink(0).Radius(8).Clip().Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.4)).Children(func() {
					ui.Image(c, poster).Fill().Fit(ui.Cover)
				})
			}
			ui.Column(c).MaxWidth(640).MinWidth(0).Gap(12).Children(func() {
				style.Text(ui.Text(c, col.GetName()), style.Type.Display).TextColor(white).MaxLines(3).Selectable()
				ui.Row(c).Gap(12).AlignItems(ui.Center).Wrap().Children(func() {
					style.Text(ui.Text(c, style.T(c, "collection.movieCount", "count", len(col.GetParts()))), style.Type.Callout).TextColor(white.Alpha(0.8))
					if s := statusLabel(c, status); s != "" {
						style.Text(ui.Text(c, s), style.Type.Caption).FontWeight(600).TextColor(white).Padding(1, 6).Radius(4).Background(white.Alpha(0.2))
					}
				})
				if o := col.GetOverview(); o != "" {
					style.Text(ui.Text(c, o), style.Type.Body).TextColor(white.Alpha(0.85)).MaxLines(4)
				}
				ui.Row(c).Gap(12).Wrap().Padding(8, 0, 0).Children(func() {
					if requestable && heroButton(c, "add", style.T(c, "collection.request"), true).Clicked() {
						st.asking, st.err, st.chosen = true, nil, map[int32]bool{}
						for _, p := range col.GetParts() {
							if p.GetRequestable() {
								st.chosen[tmdbOf(p)] = true
							}
						}
					}
					if a.isAdmin(server) {
						icon, label := "close", style.T(c, "collection.block")
						if blocked {
							icon, label = "check", style.T(c, "collection.unblock")
						}
						b := heroButton(c, icon, label, false)
						if st.blocking {
							b.Opacity(0.5)
						}
						if b.Clicked() && !st.blocking {
							a.blockCollection(server, tmdb, !blocked)
						}
					}
				})
				if st.err != nil && !st.asking {
					style.Text(ui.Text(c, errorMessage(c, st.err)), style.Type.Callout).TextColor(white).Selectable()
				}
			})
		})
	})
}

// blockCollection blocklists a collection's movies, or takes them off the
// blocklist.
func (a *App) blockCollection(server string, tmdb int32, block bool) {
	st := &a.discoveredState
	st.blocking, st.err = true, nil
	go func() {
		svc := a.requestService(server)
		var err error
		if block {
			_, err = svc.BlockCollection(a.ctx, acquisitionv1.BlockCollectionRequest_builder{TmdbCollectionId: &tmdb}.Build())
		} else {
			_, err = svc.UnblockCollection(a.ctx, acquisitionv1.UnblockCollectionRequest_builder{TmdbCollectionId: &tmdb}.Build())
		}
		a.opts.Update(func() {
			st.blocking, st.err = false, err
			a.discoverState.reload++
		})
	}()
}

// collectionDialog builds the dialog choosing the movies of a collection
// to request, all those not requested yet at first.
func (a *App) collectionDialog(c *ui.Context, server string, col *acquisitionv1.Collection) {
	st := &a.discoveredState
	if !st.asking {
		return
	}
	colors := style.Colors(c)
	var open, chosen []int32
	for _, p := range col.GetParts() {
		if p.GetRequestable() {
			open = append(open, tmdbOf(p))
			if st.chosen[tmdbOf(p)] {
				chosen = append(chosen, tmdbOf(p))
			}
		}
	}
	ui.Modal(c, &st.asking, func() {
		ui.Column(c).Width(440).MaxHeight(600).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "discover.requestTitle", "name", col.GetName())), style.Type.Title3).TextColor(style.Color(colors.TextPrimary))
			ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					all := len(chosen) == len(open)
					if ui.Checkbox(c, &all, style.T(c, "collection.allMovies")).MinHeight(40).Changed() {
						for _, n := range open {
							st.chosen[n] = all
						}
					}
					for _, p := range col.GetParts() {
						id := tmdbOf(p)
						on := st.chosen[id]
						ui.Row(c.Key(id)).MinHeight(40).Gap(12).AlignItems(ui.Center).Children(func() {
							if ui.Checkbox(c, &on, p.GetName()).Grow(1).Disabled(!p.GetRequestable()).Changed() {
								st.chosen[id] = on
							}
							if p.GetYear() > 0 {
								style.Text(ui.Text(c, fmt.Sprint(p.GetYear())), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
							}
							if l := statusLabel(c, p.GetStatus()); l != "" {
								style.Text(ui.Text(c, l), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary))
							}
						})
					}
				})
			})
			if st.err != nil {
				style.Text(ui.Text(c, style.T(c, "discover.requestFailed", "reason", errorMessage(c, st.err))), style.Type.Callout).TextColor(style.Color(colors.Danger))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					st.asking = false
				}
				if ui.PrimaryButton(c, style.T(c, "collection.requestMovies", "count", len(chosen))).Disabled(st.blocking || len(chosen) == 0).Clicked() {
					a.requestMovies(server, chosen)
				}
			})
		})
	})
}

// requestMovies requests movies one by one, stopping at the first that
// fails.
func (a *App) requestMovies(server string, ids []int32) {
	st := &a.discoveredState
	st.blocking, st.err = true, nil
	go func() {
		kind := acquisitionv1.MediaKind_MEDIA_KIND_MOVIE
		var err error
		for _, id := range ids {
			if _, err = a.requestService(server).CreateRequest(a.ctx, acquisitionv1.CreateRequestRequest_builder{MediaKind: &kind, TmdbId: &id}.Build()); err != nil {
				break
			}
		}
		a.opts.Update(func() {
			st.blocking, st.err = false, err
			if err == nil {
				st.asking = false
			}
			a.discoverState.reload++
		})
	}()
}

package app

import (
	"context"
	"strings"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"
	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// issuesState is the state of the issues reported with items, as the web
// UI's (apps/web/src/screens/issues.tsx): the open issues of the items
// shown, the issue open in its dialog, and the problem being reported.
type issuesState struct {
	lists  map[string]*task[[]*acquisitionv1.Issue] // by server and item
	reload int                                      // counts the changes made

	// The issue's dialog: the issue of a server, and the name of its item.
	open             bool
	server, id, name string
	issue            task[*acquisitionv1.Issue]
	comment          string
	deleting, busy   bool
	err              error

	// The settings' Issues: the statuses shown, among issueFilters, how
	// many, and the names of the issues' items, by server and item.
	filter int
	limit  int
	all    task[*acquisitionv1.ListIssuesResponse]
	names  map[string]*task[string]

	// The report's dialog, of an item.
	reporting       bool
	item            *libraryv1.Item
	kind            int // among issueTypes
	season, episode float64
	message         string
}

// issueTypes are the kinds of problem reported, with their names.
var issueTypes = []struct {
	value acquisitionv1.IssueType
	label string
}{
	{acquisitionv1.IssueType_ISSUE_TYPE_VIDEO, "issues.video"},
	{acquisitionv1.IssueType_ISSUE_TYPE_AUDIO, "issues.audio"},
	{acquisitionv1.IssueType_ISSUE_TYPE_SUBTITLES, "issues.subtitles"},
	{acquisitionv1.IssueType_ISSUE_TYPE_OTHER, "issues.other"},
}

func (a *App) issueService(id string) acquisitionv1connect.IssueServiceClient {
	return acquisitionv1connect.NewIssueServiceClient(a.connectHTTP(), serverURL(id))
}

// reportable reports whether problems are reported with items of a kind.
func reportable(kind libraryv1.ItemKind) bool {
	switch kind {
	case libraryv1.ItemKind_ITEM_KIND_MOVIE, libraryv1.ItemKind_ITEM_KIND_SERIES, libraryv1.ItemKind_ITEM_KIND_SEASON, libraryv1.ItemKind_ITEM_KIND_EPISODE:
		return true
	}
	return false
}

// mayReport reports whether the user may report problems on a server:
// administrators, and users allowed to.
func (a *App) mayReport(server string) bool {
	me := a.currentUser(server).value
	return me.GetAdmin() || me.GetPolicy().GetReportIssues()
}

// issueTypeName names the kind of an issue.
func issueTypeName(c *ui.Context, t acquisitionv1.IssueType) string {
	for _, x := range issueTypes {
		if x.value == t {
			return style.T(c, x.label)
		}
	}
	return ""
}

// issueNumbers tells the season or episode of a series an issue is about.
func issueNumbers(c *ui.Context, i *acquisitionv1.Issue) string {
	switch {
	case i.GetSeasonNumber() <= 0:
		return ""
	case i.GetEpisodeNumber() > 0:
		return episodeCode(i.GetSeasonNumber(), i.GetEpisodeNumber())
	}
	return style.T(c, "discover.season", "number", i.GetSeasonNumber())
}

// issueUserName names a user of a server on its issues: everyone for
// administrators, else the user and "Administrator".
func (a *App) issueUserName(c *ui.Context, server, id string) string {
	me := a.currentUser(server).value
	if id == me.GetId() {
		return me.GetName()
	}
	if me.GetAdmin() {
		if name := a.serverUsers(server)[id]; name != "" {
			return name
		}
	}
	return style.T(c, "issues.administrator")
}

// openIssues loads the open issues of an item.
func (a *App) openIssues(server, item string) *task[[]*acquisitionv1.Issue] {
	st := &a.issues
	if st.lists == nil {
		st.lists = map[string]*task[[]*acquisitionv1.Issue]{}
	}
	t := st.lists[key(server, item)]
	if t == nil {
		t = &task[[]*acquisitionv1.Issue]{}
		st.lists[key(server, item)] = t
	}
	return load(a, t, key("issues", server, item, a.generation, st.reload), func(ctx context.Context) ([]*acquisitionv1.Issue, error) {
		res, err := a.issueService(server).ListIssues(ctx, acquisitionv1.ListIssuesRequest_builder{
			ItemId: &item, Statuses: []acquisitionv1.IssueStatus{acquisitionv1.IssueStatus_ISSUE_STATUS_OPEN},
		}.Build())
		return res.GetIssues(), err
	})
}

// openIssue opens an issue of a server in its dialog; name is its item's.
func (a *App) openIssue(server, id, name string) {
	st := &a.issues
	st.open, st.server, st.id, st.name = true, server, id, name
	st.issue, st.comment, st.deleting, st.busy, st.err = task[*acquisitionv1.Issue]{}, "", false, false, nil
}

// report opens the dialog reporting a problem with an item.
func (a *App) report(it *libraryv1.Item) {
	st := &a.issues
	st.reporting, st.item, st.kind, st.season, st.episode, st.message, st.busy, st.err = true, it, 0, 0, 0, "", false, nil
}

// itemIssues builds the open issues of an item on its details, for whoever
// may see them, each opening its dialog.
func (a *App) itemIssues(c *ui.Context, server string, it *libraryv1.Item) {
	issues := a.openIssues(server, it.GetId()).value
	if len(issues) == 0 {
		return
	}
	colors := style.Colors(c)
	ui.Column(c).Padding(4, style.Layout(c).Margin, 0).Gap(16).Children(func() {
		style.Text(ui.Text(c, style.T(c, "issues.itemTitle")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
		ui.Column(c).MaxWidth(768).Children(func() { a.issueList(c, server, issues, "", it.GetName()) })
	})
}

// issueList builds issues in a group under a title, each opening its
// dialog; item is the name of their item.
func (a *App) issueList(c *ui.Context, server string, issues []*acquisitionv1.Issue, title, item string) {
	group(c, "issues", title, "", func(g *settingsGroup) {
		for k, i := range issues {
			if k > 0 {
				g.divider()
			}
			a.issueRow(c, server, i, item, false)
		}
	})
}

// issueRow builds an issue in a list: its item when showItem, its kind
// and numbers, what it says, who reported it, when it last changed and
// its status; it opens the issue's dialog.
func (a *App) issueRow(c *ui.Context, server string, i *acquisitionv1.Issue, item string, showItem bool) {
	colors := style.Colors(c)
	label := joinDetail(issueTypeName(c, i.GetType()), issueNumbers(c, i))
	if showItem {
		label = joinDetail(item, label)
	}
	var first string
	if cs := i.GetComments(); len(cs) > 0 {
		first = strings.ReplaceAll(cs[0].GetMessage(), "\n", " ")
	}
	var changed string
	if i.HasUpdateTime() {
		changed = fromNow(c, i.GetUpdateTime().AsTime())
	}
	b := ui.ButtonBase(c.Key(i.GetId())).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Label(label)
	if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).SingleLine()
			style.Text(ui.Text(c, joinDetail(first, a.issueUserName(c, server, i.GetCreatedBy()), changed)), style.Type(c).Footnote).
				TextColor(style.Color(colors.TextSecondary)).SingleLine()
		})
		statusBadge(c, i.GetStatus())
	})
	if b.Clicked() {
		a.openIssue(server, i.GetId(), item)
	}
}

// choiceChip builds a chip of a single choice, filled when chosen,
// reporting a click.
func choiceChip(c *ui.Context, label string, on bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Cursor(ui.CursorPointer).MinHeight(32).Padding(0, 12).Radius(design.RadiusFull).AlignItems(ui.Center).Label(label)
	tc := style.Color(colors.TextPrimary)
	switch {
	case on:
		b.Background(style.Color(colors.Accent))
		tc = style.Color(colors.OnAccent)
	case b.Hovered():
		b.Background(style.Color(colors.FillSelected))
	default:
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Footnote).FontWeight(600).TextColor(tc)
	})
	return b.Clicked()
}

// statusBadge builds an issue's status: open, or resolved in green.
func statusBadge(c *ui.Context, s acquisitionv1.IssueStatus) {
	colors := style.Colors(c)
	label, bg, fg := style.T(c, "issues.open"), style.Color(colors.Button), style.Color(colors.OnButton)
	if s != acquisitionv1.IssueStatus_ISSUE_STATUS_OPEN {
		label, bg, fg = style.T(c, "issues.resolved"), ui.RGB(0x34, 0xC7, 0x59), ui.RGB(0xFF, 0xFF, 0xFF)
	}
	ui.Box(c).Shrink(0).Padding(2, 8).Radius(999).Background(bg).Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Caption).FontWeight(600).TextColor(fg)
	})
}

// issueDialogs builds the dialog reporting a problem and the dialog of an
// issue, whichever is open.
func (a *App) issueDialogs(c *ui.Context, server string) {
	st := &a.issues
	if st.reporting {
		a.reportDialog(c, server)
	}
	if st.open {
		a.issueDialog(c)
	}
}

// issueAct changes an issue in the background, its dialog busy until it
// ends; done runs on the main thread with the issue changed, if it was.
func (a *App) issueAct(fn func(ctx context.Context, s acquisitionv1connect.IssueServiceClient) (*acquisitionv1.Issue, error), done func(*acquisitionv1.Issue)) {
	st := &a.issues
	st.busy, st.err = true, nil
	s := a.issueService(st.server)
	go func() {
		i, err := fn(a.ctx, s)
		a.opts.Update(func() {
			st.busy, st.err = false, err
			if err != nil {
				return
			}
			st.reload++
			if done != nil {
				done(i)
			}
		})
	}()
}

// reportDialog builds reporting a problem with an item: its kind, for a
// series the season and episode, and what is wrong; the issue reported
// then opens.
func (a *App) reportDialog(c *ui.Context, server string) {
	st := &a.issues
	colors := style.Colors(c)
	it := st.item
	series := it.GetKind() == libraryv1.ItemKind_ITEM_KIND_SERIES
	ok := strings.TrimSpace(st.message) != "" && !st.busy
	send := func() {
		id, kind, message := it.GetId(), issueTypes[st.kind].value, strings.TrimSpace(st.message)
		var season, episode int32
		if series {
			season = int32(st.season)
			if season > 0 {
				episode = int32(st.episode)
			}
		}
		st.server = server
		a.issueAct(func(ctx context.Context, s acquisitionv1connect.IssueServiceClient) (*acquisitionv1.Issue, error) {
			res, err := s.CreateIssue(ctx, acquisitionv1.CreateIssueRequest_builder{
				ItemId: &id, Type: &kind, SeasonNumber: &season, EpisodeNumber: &episode, Message: &message,
			}.Build())
			return res.GetIssue(), err
		}, func(i *acquisitionv1.Issue) {
			st.reporting = false
			if i != nil {
				a.openIssue(server, i.GetId(), it.GetName())
			}
		})
	}
	ui.Modal(c, &st.reporting, func() {
		ui.Column(c).Width(min(520, a.contentWidth-64)).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "issues.reportTitle", "name", it.GetName())), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			ui.Row(c).Gap(8).Wrap().Children(func() {
				for i, x := range issueTypes {
					if choiceChip(c.Key(x.label), style.T(c, x.label), st.kind == i) {
						st.kind = i
					}
				}
			})
			if series {
				ui.Row(c).Gap(12).Children(func() {
					for _, f := range []struct {
						label string
						value *float64
					}{{style.T(c, "issues.season"), &st.season}, {style.T(c, "issues.episode"), &st.episode}} {
						ui.Column(c.Key(f.label)).Grow(1).Basis(0).Gap(4).Children(func() {
							footnote(c, f.label, false)
							in := ui.NumberInput(c, f.value, 0, 9999, 1).FillWidth().Label(f.label)
							if f.value == &st.episode && st.season == 0 {
								st.episode = 0
								in.Disabled(true)
							}
						})
					}
				})
				footnote(c, style.T(c, "issues.allNumbers"), false)
			}
			ui.TextArea(c, &st.message).FillWidth().Height(112).Placeholder(style.T(c, "issues.message")).Label(style.T(c, "issues.message")).AutoFocus()
			if st.err != nil {
				footnote(c, errorMessage(c, st.err), true)
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					st.reporting = false
				}
				if ui.PrimaryButton(c, style.T(c, "issues.send")).Disabled(!ok).Clicked() && ok {
					send()
				}
			})
		})
	})
}

// issueDialog builds an issue: its status, item, numbers and who reported
// it, its comments, a comment added, and resolving, reopening or deleting
// it.
func (a *App) issueDialog(c *ui.Context) {
	st := &a.issues
	colors := style.Colors(c)
	server, id := st.server, st.id
	t := load(a, &st.issue, key("issue", server, id, a.generation), func(ctx context.Context) (*acquisitionv1.Issue, error) {
		res, err := a.issueService(server).GetIssue(ctx, acquisitionv1.GetIssueRequest_builder{Id: &id}.Build())
		return res.GetIssue(), err
	})
	i := t.value
	set := func(changed *acquisitionv1.Issue) {
		if changed != nil {
			st.issue.value = changed
		}
	}
	title := style.T(c, "issues.title")
	if i != nil {
		title = joinDetail(issueTypeName(c, i.GetType()), st.name)
	}
	open := i.GetStatus() == acquisitionv1.IssueStatus_ISSUE_STATUS_OPEN
	ui.Modal(c, &st.open, func() {
		ui.Column(c).Width(min(640, a.contentWidth-64)).MaxHeight(640).Gap(12).Children(func() {
			style.Text(ui.Text(c, title), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			if i != nil {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					statusBadge(c, i.GetStatus())
					if st.name != "" && linkButton(c, st.name).Clicked() {
						st.open = false
						a.router.Push("/item/" + server + "/" + i.GetItemId())
					}
					if n := issueNumbers(c, i); n != "" {
						secondaryText(c, n)
					}
					secondaryText(c, style.T(c, "issues.reportedBy", "name", a.issueUserName(c, server, i.GetCreatedBy())))
				})
				ui.Scroll(c).Shrink(1).Children(func() {
					ui.Column(c).Gap(12).Children(func() {
						for _, m := range i.GetComments() {
							var when string
							if m.HasCreateTime() {
								when = fromNow(c, m.GetCreateTime().AsTime())
							}
							ui.Column(c.Key(m.GetId())).Padding(10, 14).Gap(2).Radius(12).Background(style.Color(colors.Fill)).Children(func() {
								footnote(c, joinDetail(a.issueUserName(c, server, m.GetUserId()), when), false)
								style.Text(ui.Text(c, m.GetMessage()), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Selectable()
							})
						}
					})
				})
				ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
					ui.TextArea(c, &st.comment).Grow(1).Height(56).Placeholder(style.T(c, "issues.comment")).Label(style.T(c, "issues.comment"))
					comment := strings.TrimSpace(st.comment)
					if secondaryButton(c, "", style.T(c, "issues.commentSend")).Disabled(comment == "" || st.busy).Clicked() && comment != "" && !st.busy {
						a.issueAct(func(ctx context.Context, s acquisitionv1connect.IssueServiceClient) (*acquisitionv1.Issue, error) {
							res, err := s.AddComment(ctx, acquisitionv1.AddCommentRequest_builder{Id: &id, Message: &comment}.Build())
							return res.GetIssue(), err
						}, func(changed *acquisitionv1.Issue) {
							st.comment = ""
							set(changed)
						})
					}
				})
			}
			for _, err := range []error{t.err, st.err} {
				if err != nil {
					footnote(c, errorMessage(c, err), true)
				}
			}
			if st.deleting {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					style.Text(ui.Text(c, style.T(c, "issues.confirmDelete")), style.Type(c).Callout).TextColor(style.Color(colors.TextPrimary)).Grow(1)
					if linkButton(c, style.T(c, "action.cancel")).Clicked() {
						st.deleting = false
					}
					if dangerButton(c, style.T(c, "issues.delete"), st.busy) && !st.busy {
						a.issueAct(func(ctx context.Context, s acquisitionv1connect.IssueServiceClient) (*acquisitionv1.Issue, error) {
							_, err := s.DeleteIssue(ctx, acquisitionv1.DeleteIssueRequest_builder{Id: &id}.Build())
							return nil, err
						}, func(*acquisitionv1.Issue) { st.open = false })
					}
				})
				return
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if i != nil {
					b := ui.ButtonBase(c.Key("delete")).Height(32).Padding(0, 8).AlignItems(ui.Center).Label(style.T(c, "issues.delete"))
					b.Children(func() {
						style.Text(ui.Text(c, style.T(c, "issues.delete")), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.Danger))
					})
					if b.Clicked() {
						st.deleting = true
					}
				}
				ui.Box(c).Grow(1)
				if i != nil {
					label := style.T(c, "issues.reopen")
					if open {
						label = style.T(c, "issues.resolve")
					}
					if secondaryButton(c, "", label).Disabled(st.busy).Clicked() && !st.busy {
						a.issueAct(func(ctx context.Context, s acquisitionv1connect.IssueServiceClient) (*acquisitionv1.Issue, error) {
							if open {
								res, err := s.ResolveIssue(ctx, acquisitionv1.ResolveIssueRequest_builder{Id: &id}.Build())
								return res.GetIssue(), err
							}
							res, err := s.ReopenIssue(ctx, acquisitionv1.ReopenIssueRequest_builder{Id: &id}.Build())
							return res.GetIssue(), err
						}, set)
					}
				}
				if ui.PrimaryButton(c, style.T(c, "keys.done")).Clicked() {
					st.open = false
				}
			})
		})
	})
}

// manageDialog builds what an administrator manages of a discovered
// title, as Seerr's manage panel: its requests, downloads and open
// issues, and the blocklist.
func (a *App) manageDialog(c *ui.Context, server string, title *acquisitionv1.Title) {
	st := &a.discoverState
	colors := style.Colors(c)
	item := title.GetItemId()
	blocked := title.GetStatus() == acquisitionv1.MediaStatus_MEDIA_STATUS_BLOCKLISTED
	blockable := title.GetStatus() == acquisitionv1.MediaStatus_MEDIA_STATUS_UNSPECIFIED && len(title.GetRequests()) == 0
	ui.Modal(c, &st.managing, func() {
		ui.Column(c).Width(min(720, a.contentWidth-64)).MaxHeight(720).Gap(12).Children(func() {
			ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
				style.Text(ui.Text(c, style.T(c, "discover.manageTitle", "name", title.GetName())), style.Type(c).Title3).
					TextColor(style.Color(colors.TextPrimary)).Grow(1).Shrink(1)
				if linkButton(c, style.T(c, "keys.done")).Clicked() {
					st.managing = false
				}
			})
			ui.Scroll(c).Shrink(1).Children(func() {
				ui.Column(c).Gap(24).Padding(0, 0, 4).Children(func() {
					users := a.serverUsers(server)
					group(c, "requests", style.T(c, "requests.title"), "", func(g *settingsGroup) {
						for k, r := range title.GetRequests() {
							if k > 0 {
								g.divider()
							}
							ui.Column(c.Key(r.GetId())).Padding(12, 16).Children(func() {
								a.requestRow(c, server, r, true, users[r.GetRequestedBy()])
							})
						}
						if len(title.GetRequests()) == 0 {
							g.note(style.T(c, "discover.noRequests"), false)
						}
					})
					if item == "" {
						return
					}
					if queue := a.itemQueue(&st.manage, server, item).value; len(queue) > 0 {
						group(c, "queue", style.T(c, "downloads.queue"), "", func(g *settingsGroup) {
							a.asItemServer(server, func() {
								for k, d := range queue {
									if k > 0 {
										g.divider()
									}
									ui.Column(c.Key(d.GetId())).Padding(12, 16).Gap(6).Children(func() { a.queueRow(c, d) })
								}
							})
						})
					}
					if issues := a.openIssues(server, item).value; len(issues) > 0 {
						a.issueList(c, server, issues, style.T(c, "issues.itemTitle"), title.GetName())
					}
					ui.Row(c).Children(func() {
						if linkButton(c, style.T(c, "discover.openItem")).Clicked() {
							st.managing = false
							a.router.Push("/item/" + server + "/" + item)
						}
					})
				})
				if !blocked && !blockable {
					return
				}
				footer := style.T(c, "discover.blockFooter")
				if blocked {
					footer = ""
				}
				group(c, "block", "", footer, func(g *settingsGroup) {
					ui.Row(c).Padding(12, 16).Children(func() {
						label := style.T(c, "collection.block")
						if blocked {
							label = style.T(c, "collection.unblock")
						}
						var clicked bool
						if blocked {
							clicked = smallButton(c, label, st.busy)
						} else {
							clicked = dangerButton(c, label, st.busy)
						}
						if clicked && !st.busy {
							a.blockTitle(server, title, !blocked)
						}
					})
				})
			})
		})
	})
}

// blockTitle blocklists a title, or takes it off the blocklist.
func (a *App) blockTitle(server string, title *acquisitionv1.Title, block bool) {
	st := &a.discoverState
	st.busy, st.err = true, nil
	kind, tmdb := title.GetMediaKind(), title.GetTmdbId()
	go func() {
		svc := a.requestService(server)
		var err error
		if block {
			_, err = svc.BlockTitle(a.ctx, acquisitionv1.BlockTitleRequest_builder{MediaKind: &kind, TmdbId: &tmdb}.Build())
		} else {
			_, err = svc.UnblockTitle(a.ctx, acquisitionv1.UnblockTitleRequest_builder{MediaKind: &kind, TmdbId: &tmdb}.Build())
		}
		a.opts.Update(func() {
			st.busy, st.err = false, err
			st.reload++
		})
	}()
}

// titleItem is the library's item of a discovered title, as reporting a
// problem with it needs.
func titleItem(title *acquisitionv1.Title) *libraryv1.Item {
	kind := libraryv1.ItemKind_ITEM_KIND_MOVIE
	if title.GetMediaKind() == acquisitionv1.MediaKind_MEDIA_KIND_SERIES {
		kind = libraryv1.ItemKind_ITEM_KIND_SERIES
	}
	return libraryv1.Item_builder{Id: new(title.GetItemId()), Name: new(title.GetName()), Kind: &kind}.Build()
}

// issueFilters are the statuses the settings' Issues show, with their names.
var issueFilters = []struct {
	label    string
	statuses []acquisitionv1.IssueStatus
}{
	{"issues.open", []acquisitionv1.IssueStatus{acquisitionv1.IssueStatus_ISSUE_STATUS_OPEN}},
	{"issues.resolved", []acquisitionv1.IssueStatus{acquisitionv1.IssueStatus_ISSUE_STATUS_RESOLVED}},
	{"issues.all", nil},
}

// issuesSection builds the settings' Issues, as the web UI's: the issues
// reported on the server chosen, the user's own or everyone's for
// administrators, by status, each with its item and opening its dialog.
func (a *App) issuesSection(c *ui.Context) {
	st := &a.issues
	server := a.chosenServer().GetId()
	sectionHeader(c, style.T(c, "settings.issues"), style.T(c, "issues.intro"))
	filters := make([]string, len(issueFilters))
	for i, f := range issueFilters {
		filters[i] = style.T(c, f.label)
	}
	if choose(c.Key("filter"), style.T(c, "issues.filter"), &st.filter, filters) {
		st.limit = 0
	}
	limit := max(st.limit, 50)
	statuses := issueFilters[st.filter].statuses
	t := adminTask(a, &st.all, "all-issues", func(ctx context.Context, s string) (*acquisitionv1.ListIssuesResponse, error) {
		return a.issueService(s).ListIssues(ctx, acquisitionv1.ListIssuesRequest_builder{Statuses: statuses, Limit: new(int32(limit))}.Build())
	}, st.filter, limit, st.reload)
	footer := ""
	if t.loaded && t.err == nil {
		footer = style.T(c, "issues.count", "count", int(t.value.GetTotal()))
	}
	issues := t.value.GetIssues()
	group(c, "all-issues", "", footer, func(g *settingsGroup) {
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(issues) == 0:
			g.note(style.T(c, "issues.none"), false)
		}
		for k, i := range issues {
			if k > 0 {
				g.divider()
			}
			a.issueRow(c, server, i, a.issueItemName(server, i.GetItemId()), true)
		}
		if int(t.value.GetTotal()) > len(issues) && g.more(style.T(c, "activity.showMore")) {
			st.limit = limit + 50
		}
	})
	a.issueDialogs(c, server)
}

// issueItemName loads the name of an issue's item.
func (a *App) issueItemName(server, item string) string {
	st := &a.issues
	if st.names == nil {
		st.names = map[string]*task[string]{}
	}
	k := key(server, item)
	t := st.names[k]
	if t == nil {
		t = &task[string]{}
		st.names[k] = t
	}
	return load(a, t, key("issue-item", server, item, a.generation), func(ctx context.Context) (string, error) {
		res, err := a.itemService(server).GetItem(ctx, libraryv1.GetItemRequest_builder{Id: &item}.Build())
		return res.GetItem().GetName(), err
	}).value
}

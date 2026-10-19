package app

import (
	"cmp"
	"context"
	"fmt"
	"net/url"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/types/known/durationpb"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// usersState is the state of the settings' Users: the user added, the
// user edited and the invitation links.
type usersState struct {
	ratings     task[[]*systemv1.Rating]
	invitations task[[]*userv1.Invitation]
	fresh       newUserForm
	form        userForm
	invites     invitesForm
	busy        string // the change being made
	copied      string // the link copied last
}

// newUserForm is the form of a user being added.
type newUserForm struct {
	managed             bool
	name, password, pin string
	restriction         int
	err                 error
}

// quota is a user's own quota of requests.
type quota struct{ limit, days float64 }

// userDraft is a user as edited: its name, state, restriction profile
// and policy.
type userDraft struct {
	name         string
	disabled     bool
	restriction  int
	allLibraries bool
	libraryIDs   []string
	// maxRating is the highest rating score allowed, when hasMax.
	hasMax                  bool
	maxRating               int32
	blockUnrated            bool
	allowedLabels           string
	blockedLabels           string
	allowDownload           bool
	allowTranscoding        bool
	maxBitrate              int64
	maxSessions             float64
	requestMovies           bool
	requestSeries           bool
	autoApproveMovies       bool
	autoApproveSeries       bool
	reportIssues            bool
	movieQuota, seriesQuota *quota
}

// userForm is the page of a user: the draft of the user as loaded and as
// edited, and its sign-ins' fields.
type userForm struct {
	id               string
	draft, saved     userDraft
	err              error
	notice           string
	confirming       bool
	password, pin    string
	signNotice, link string
	signErr          error
}

// invitesForm is the form of an invitation link being made, and the
// link made last.
type invitesForm struct {
	making        bool
	uses, days    float64
	all, download bool
	ids           []string
	madeID, link  string
	err           error
}

// restrictions are the restriction profiles, by their number.
var restrictions = []string{"users.restrictionNone", "users.restrictionYoungerChild", "users.restrictionOlderChild", "users.restrictionTeen"}

// bitrates are the streaming limits offered, in Mbps; 0 for no limit.
var bitrates = []int64{0, 120, 80, 40, 20, 10, 8, 4, 2, 1}

// adultScore is the rating score of adult content.
const adultScore = 1000

func bitrateTitle(c *ui.Context, b int64) string {
	if b == 0 {
		return style.T(c, "users.unlimited")
	}
	return style.T(c, "users.mbps", "mbps", b/1_000_000)
}

// accessible tells the libraries access is given to: not those of
// collections and playlists.
func accessible(l *libraryv1.Library) bool {
	k := l.GetKind()
	return k != libraryv1.LibraryKind_LIBRARY_KIND_COLLECTIONS && k != libraryv1.LibraryKind_LIBRARY_KIND_PLAYLISTS
}

// mediumDate formats a date as the web UI's medium date.
func mediumDate(t time.Time) string { return t.Local().Format("Jan 2, 2006") }

func usersPath(user string) string {
	if user == "" {
		return "/settings/users"
	}
	return "/settings/users?user=" + url.QueryEscape(user)
}

func draftOf(u *userv1.User) userDraft {
	p := u.GetPolicy()
	d := userDraft{
		name: u.GetName(), disabled: u.GetDisabled(), restriction: int(u.GetRestriction()),
		allLibraries: p.GetAllLibraries(), libraryIDs: slices.Clone(p.GetLibraryIds()),
		hasMax: p.HasMaxParentalRating(), maxRating: p.GetMaxParentalRating(), blockUnrated: p.GetBlockUnrated(),
		allowedLabels: strings.Join(p.GetAllowedLabels(), ", "), blockedLabels: strings.Join(p.GetBlockedLabels(), ", "),
		allowDownload: p.GetAllowDownload(), allowTranscoding: p.GetAllowTranscoding(),
		maxBitrate: p.GetMaxStreamingBitrate(), maxSessions: float64(p.GetMaxSessions()),
		requestMovies: p.GetRequestMovies(), requestSeries: p.GetRequestSeries(),
		autoApproveMovies: p.GetAutoApproveMovies(), autoApproveSeries: p.GetAutoApproveSeries(), reportIssues: p.GetReportIssues(),
	}
	if q := p.GetMovieQuota(); q != nil {
		d.movieQuota = &quota{float64(q.GetLimit()), float64(q.GetDays())}
	}
	if q := p.GetSeriesQuota(); q != nil {
		d.seriesQuota = &quota{float64(q.GetLimit()), float64(q.GetDays())}
	}
	if len(d.libraryIDs) == 0 {
		d.libraryIDs = nil
	}
	return d
}

func (d userDraft) equal(e userDraft) bool {
	if len(d.libraryIDs) == 0 {
		d.libraryIDs = nil
	}
	if len(e.libraryIDs) == 0 {
		e.libraryIDs = nil
	}
	return reflect.DeepEqual(d, e)
}

// labelsOf splits labels separated by commas.
func labelsOf(s string) []string {
	var labels []string
	for l := range strings.SplitSeq(s, ",") {
		if l = strings.TrimSpace(l); l != "" && !slices.Contains(labels, l) {
			labels = append(labels, l)
		}
	}
	return labels
}

func quotaOf(q *quota) *userv1.RequestQuota {
	if q == nil {
		return nil
	}
	return userv1.RequestQuota_builder{Limit: new(int32(q.limit)), Days: new(int32(q.days))}.Build()
}

func (d userDraft) policy() *userv1.UserPolicy {
	b := userv1.UserPolicy_builder{
		AllLibraries: new(d.allLibraries), BlockUnrated: new(d.blockUnrated),
		AllowedLabels: labelsOf(d.allowedLabels), BlockedLabels: labelsOf(d.blockedLabels),
		AllowDownload: new(d.allowDownload), AllowTranscoding: new(d.allowTranscoding),
		MaxStreamingBitrate: new(d.maxBitrate), MaxSessions: new(int32(d.maxSessions)),
		RequestMovies: new(d.requestMovies), RequestSeries: new(d.requestSeries),
		AutoApproveMovies: new(d.autoApproveMovies), AutoApproveSeries: new(d.autoApproveSeries), ReportIssues: new(d.reportIssues),
		MovieQuota: quotaOf(d.movieQuota), SeriesQuota: quotaOf(d.seriesQuota),
	}
	if !d.allLibraries {
		b.LibraryIds = d.libraryIDs
	}
	if d.hasMax {
		b.MaxParentalRating = new(d.maxRating)
	}
	return b.Build()
}

// checkRow adds a row of a checkbox and reports a change of *on.
func (g *settingsGroup) checkRow(label string, on *bool, disabled bool) bool {
	g.divider()
	return ui.Checkbox(g.c.Key(g.rows), on, label).MinHeight(44).Padding(8, 16).Disabled(disabled).Changed()
}

// toggleRow adds a row of a switch, or of its state when readOnly.
func (g *settingsGroup) toggleRow(label, detail string, on *bool, readOnly bool) {
	if !readOnly {
		g.switchRow(label, detail, on)
		return
	}
	c := g.c
	state := style.T(c, "users.off")
	if *on {
		state = style.T(c, "users.on")
	}
	g.row(label, detail, nil, func() { secondaryText(c, state) })
}

// addRow adds the last row of a group, adding to what it lists.
func (g *settingsGroup) addRow(label string) bool {
	c := g.c
	g.divider()
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(g.rows)).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Justify(ui.Start).Label(label)
	if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		ui.Icon(c, style.Icon("add", false)).Size(20, 20).TextColor(style.Color(colors.AccentText))
		style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.AccentText))
	})
	return b.Clicked()
}

func secondaryText(c *ui.Context, text string) {
	style.Text(ui.Text(c, text), style.Type(c).Callout).TextColor(style.Color(style.Colors(c).TextSecondary))
}

func dangerText(c *ui.Context, text string) {
	style.Text(ui.Text(c, text), style.Type(c).Callout).TextColor(style.Color(style.Colors(c).Danger)).Selectable()
}

// dangerButton builds a small button of the secondary look whose label
// is of the danger color.
func dangerButton(c *ui.Context, label string, disabled bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Height(28).Padding(0, 12).Radius(8).AlignItems(ui.Center).Label(label).Disabled(disabled).
		Background(style.Color(colors.Fill))
	if b.Hovered() && !disabled {
		b.Background(style.Color(colors.FillSelected))
	}
	if disabled {
		b.Opacity(0.5)
	}
	b.Children(func() {
		style.Text(ui.Text(c, label), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.Danger))
	})
	return b.Clicked() && !disabled
}

// backTo builds the link back to a section.
func (a *App) backTo(c *ui.Context, label, path string) {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).AlignSelf(ui.Start).Gap(4).AlignItems(ui.Center).Label(label)
	b.Children(func() {
		ui.Icon(c, style.Icon("back", false)).Size(16, 16).TextColor(style.Color(colors.AccentText))
		t := style.Text(ui.Text(c, label), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.AccentText))
		if b.Hovered() {
			t.Underline()
		}
	})
	if b.Clicked() {
		a.router.Push(path)
	}
}

func badges(c *ui.Context, u *userv1.User) {
	colors := style.Colors(c)
	var list []string
	if u.GetAdmin() {
		list = append(list, style.T(c, "users.admin"))
	}
	if u.GetManaged() {
		list = append(list, style.T(c, "users.managed"))
	}
	if u.GetDisabled() {
		list = append(list, style.T(c, "users.disabled"))
	}
	ui.Row(c.Key(u.GetId())).Gap(6).AlignItems(ui.Center).Children(func() {
		for _, b := range list {
			style.Text(ui.Text(c.Key(b), b), style.Type(c).Caption).TextColor(style.Color(colors.TextSecondary)).
				Padding(1, 6).Radius(4).Background(style.Color(colors.Fill))
		}
	})
}

// userAction runs a change of the users in the background, keyed what;
// done takes its error on the main thread.
func (a *App) userAction(what string, fn func(ctx context.Context, s userv1connect.UserServiceClient) error, done func(err error)) {
	us := &a.settingsState.admin.people
	us.busy = what
	client := a.userService(a.chosenServer().GetId())
	go func() {
		err := fn(a.ctx, client)
		a.opts.Update(func() {
			us.busy = ""
			if err == nil {
				a.settingsState.admin.reload++
			}
			done(err)
		})
	}()
}

func (a *App) usersSection(c *ui.Context, r *ui.Route) {
	switch user := r.Query("user"); user {
	case "":
		a.userList(c)
	case "new":
		a.newUser(c)
	default:
		a.userPage(c, user)
	}
}

func (a *App) users() *task[[]*userv1.User] {
	st := &a.settingsState.admin
	return adminTask(a, &st.users, "users", func(ctx context.Context, s string) ([]*userv1.User, error) {
		res, err := a.userService(s).ListUsers(ctx, &userv1.ListUsersRequest{})
		return res.GetUsers(), err
	})
}

func (a *App) adminLibraries() *task[[]*libraryv1.Library] {
	st := &a.settingsState.admin
	return adminTask(a, &st.libs, "libraries", func(ctx context.Context, s string) ([]*libraryv1.Library, error) {
		res, err := a.libraryService(s).ListLibraries(ctx, &libraryv1.ListLibrariesRequest{})
		if err == nil {
			a.opts.Update(func() { a.listedScans(s, res.GetLibraries()) })
		}
		return res.GetLibraries(), err
	})
}

func (a *App) userList(c *ui.Context) {
	st := &a.settingsState.admin
	standalone := a.account == nil
	message := style.T(c, "users.message")
	if !standalone {
		message = style.T(c, "users.messageMavioDev")
	}
	sectionHeader(c, style.T(c, "settings.users"), message)
	t := a.users()
	if taskStatus(c, t, func() { st.users.key = "" }, nil) {
		return
	}
	users := slices.Clone(t.value)
	slices.SortFunc(users, func(x, y *userv1.User) int {
		if x.GetAdmin() != y.GetAdmin() {
			if x.GetAdmin() {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(x.GetName()), strings.ToLower(y.GetName()))
	})
	colors := style.Colors(c)
	group(c, "users", "", "", func(g *settingsGroup) {
		for _, u := range users {
			g.divider()
			b := ui.ButtonBase(c.Key(u.GetId())).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Label(u.GetName())
			if b.Hovered() {
				b.Background(style.Color(colors.Fill))
			}
			b.Children(func() {
				avatar(c, u.GetName(), 32)
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						style.Text(ui.Text(c, u.GetName()), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
						badges(c, u)
					})
					detail := style.T(c, "users.neverSignedIn")
					if u.HasLastLoginTime() {
						detail = style.T(c, "users.lastSignIn", "date", mediumDate(u.GetLastLoginTime().AsTime()))
					}
					style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
				})
				ui.Icon(c, style.Icon("forward", false)).Size(16, 16).TextColor(style.Color(colors.TextTertiary))
			})
			if b.Clicked() {
				a.router.Push(usersPath(u.GetId()))
			}
		}
		if standalone && g.addRow(style.T(c, "users.add")) {
			a.settingsState.admin.people.fresh = newUserForm{}
			a.router.Push(usersPath("new"))
		}
	})
	if standalone {
		a.invitationsGroup(c, users)
	}
}

// newUser adds a member with a password, or a managed user with a PIN
// and a restriction profile.
func (a *App) newUser(c *ui.Context) {
	us := &a.settingsState.admin.people
	f := &us.fresh
	a.backTo(c, style.T(c, "settings.users"), usersPath(""))
	sectionHeader(c, style.T(c, "users.add"), "")
	footer := style.T(c, "users.memberFooter")
	if f.managed {
		footer = style.T(c, "users.managedFooter")
	}
	group(c, "new", "", footer, func(g *settingsGroup) {
		g.row(style.T(c, "users.kind"), "", nil, func() {
			kind := 0
			if f.managed {
				kind = 1
			}
			if choose(c, style.T(c, "users.kind"), &kind, []string{style.T(c, "users.member"), style.T(c, "users.managed")}) {
				f.managed = kind == 1
			}
		})
		g.row(style.T(c, "users.name"), "", nil, func() {
			ui.TextInput(c, &f.name).Width(288).Label(style.T(c, "users.name"))
		})
		if f.managed {
			g.row(style.T(c, "users.pin"), style.T(c, "users.pinDetail"), nil, func() { pinInput(c, &f.pin, "") })
			g.row(style.T(c, "users.restriction"), style.T(c, "users.restrictionDetail"), nil, func() {
				choose(c, style.T(c, "users.restriction"), &f.restriction, translated(c, restrictions))
			})
		} else {
			g.row(style.T(c, "auth.password"), "", nil, func() {
				ui.TextInput(c, &f.password).Width(288).Password().Label(style.T(c, "auth.password"))
			})
		}
	})
	name := strings.TrimSpace(f.name)
	valid := name != "" && (f.managed && (f.pin == "" || len(f.pin) == 4) || !f.managed && f.password != "")
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		label := style.T(c, "users.create")
		if us.busy == "create" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(!valid || us.busy != "").Clicked() {
			b := userv1.CreateUserRequest_builder{Name: &name}
			if f.managed {
				b.Managed, b.Restriction = new(true), new(userv1.RestrictionProfile(f.restriction))
				if f.pin != "" {
					b.Pin = new(f.pin)
				}
			} else {
				b.Password = new(f.password)
			}
			var created string
			f.err = nil
			a.userAction("create", func(ctx context.Context, s userv1connect.UserServiceClient) error {
				res, err := s.CreateUser(ctx, b.Build())
				created = res.GetUser().GetId()
				return err
			}, func(err error) {
				f.err = err
				if err == nil {
					a.replace(usersPath(created))
				}
			})
		}
		if f.err != nil {
			dangerText(c, errorMessage(c, f.err))
		}
	})
}

// pinInput builds the field of a four-digit PIN.
func pinInput(c *ui.Context, pin *string, placeholder string) {
	in := ui.TextInput(c, pin).Width(112).Label(style.T(c, "users.pin"))
	if placeholder != "" {
		in.Placeholder(placeholder)
	}
	if in.Changed() {
		digits := strings.Map(func(r rune) rune {
			if r >= '0' && r <= '9' {
				return r
			}
			return -1
		}, *pin)
		*pin = digits[:min(len(digits), 4)]
	}
}

func translated(c *ui.Context, keys []string) []string {
	labels := make([]string, len(keys))
	for i, k := range keys {
		labels[i] = style.T(c, k)
	}
	return labels
}

func (a *App) userPage(c *ui.Context, id string) {
	st := &a.settingsState.admin
	a.backTo(c, style.T(c, "settings.users"), usersPath(""))
	t := a.users()
	if taskStatus(c, t, func() { st.users.key = "" }, nil) {
		return
	}
	i := slices.IndexFunc(t.value, func(u *userv1.User) bool { return u.GetId() == id })
	if i < 0 {
		secondaryText(c, style.T(c, "users.notFound"))
		return
	}
	a.userForm(c, t.value[i])
}

func (a *App) userForm(c *ui.Context, user *userv1.User) {
	us := &a.settingsState.admin.people
	f := &us.form
	if f.id != user.GetId() {
		*f = userForm{id: user.GetId(), draft: draftOf(user)}
		f.saved = draftOf(user)
	}
	colors := style.Colors(c)
	d := &f.draft
	standalone := a.account == nil
	own := standalone
	ui.Column(c).Gap(4).Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
			avatar(c, user.GetName(), 40)
			style.Text(ui.Text(c, user.GetName()), style.Type(c).Title1).TextColor(style.Color(colors.TextPrimary))
			badges(c, user)
		})
		if !standalone {
			style.Text(ui.Text(c, style.T(c, "users.mavioDevNote")), style.Type(c).Body).TextColor(style.Color(colors.TextSecondary))
		}
	})
	group(c, "user", "", "", func(g *settingsGroup) {
		g.row(style.T(c, "users.name"), "", nil, func() {
			if own {
				ui.TextInput(c, &d.name).Width(288).Label(style.T(c, "users.name"))
			} else {
				secondaryText(c, user.GetName())
			}
		})
		if !user.GetAdmin() {
			g.toggleRow(style.T(c, "users.disable"), style.T(c, "users.disableDetail"), &d.disabled, !own)
		}
		if user.GetManaged() {
			g.row(style.T(c, "users.restriction"), style.T(c, "users.restrictionDetail"), nil, func() {
				choose(c, style.T(c, "users.restriction"), &d.restriction, translated(c, restrictions))
			})
		}
	})
	if !user.GetAdmin() {
		a.accessGroup(c, d, own)
	}
	group(c, "playback", style.T(c, "users.playback"), "", func(g *settingsGroup) {
		g.switchRow(style.T(c, "users.allowTranscoding"), style.T(c, "users.allowTranscodingDetail"), &d.allowTranscoding)
		g.row(style.T(c, "users.maxBitrate"), "", nil, func() {
			values := slices.Clone(bitrates)
			for j := range values {
				values[j] *= 1_000_000
			}
			if !slices.Contains(values, d.maxBitrate) {
				values = append(values, d.maxBitrate)
			}
			labels := make([]string, len(values))
			for j, v := range values {
				labels[j] = bitrateTitle(c, v)
			}
			j := slices.Index(values, d.maxBitrate)
			if choose(c, style.T(c, "users.maxBitrate"), &j, labels) {
				d.maxBitrate = values[j]
			}
		})
		g.row(style.T(c, "users.maxSessions"), style.T(c, "users.maxSessionsDetail"), nil, func() {
			ui.NumberInput(c, &d.maxSessions, 0, 100, 1).Label(style.T(c, "users.maxSessions"))
		})
	})
	if !user.GetAdmin() {
		group(c, "requests", style.T(c, "users.requests"), "", func(g *settingsGroup) {
			g.switchRow(style.T(c, "users.requestMovies"), "", &d.requestMovies)
			if d.requestMovies {
				g.switchRow(style.T(c, "users.autoApproveMovies"), "", &d.autoApproveMovies)
				quotaRow(g, "movie", style.T(c, "users.movieQuota"), &d.movieQuota)
			}
			g.switchRow(style.T(c, "users.requestSeries"), "", &d.requestSeries)
			if d.requestSeries {
				g.switchRow(style.T(c, "users.autoApproveSeries"), "", &d.autoApproveSeries)
				quotaRow(g, "series", style.T(c, "users.seriesQuota"), &d.seriesQuota)
			}
			g.switchRow(style.T(c, "users.reportIssues"), "", &d.reportIssues)
		})
	}
	changed := !d.equal(f.saved)
	if changed {
		f.notice = ""
	}
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		label := style.T(c, "action.save")
		if us.busy == "save" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(!changed || strings.TrimSpace(d.name) == "" || us.busy != "").Clicked() {
			draft := *d
			name := strings.TrimSpace(draft.name)
			req := userv1.UpdateUserRequest_builder{
				Id: new(user.GetId()), Name: &name, Disabled: new(draft.disabled),
				Restriction: new(userv1.RestrictionProfile(draft.restriction)), Policy: draft.policy(),
			}.Build()
			f.err = nil
			a.userAction("save", func(ctx context.Context, s userv1connect.UserServiceClient) error {
				_, err := s.UpdateUser(ctx, req)
				return err
			}, func(err error) {
				f.err = err
				if err == nil {
					f.saved, f.notice = draft, style.T(c, "users.saved")
				}
			})
		}
		if f.notice != "" && !changed {
			secondaryText(c, f.notice)
		}
		if f.err != nil {
			dangerText(c, errorMessage(c, f.err))
		}
	})
	if standalone {
		a.signIns(c, user)
	}
	if !user.GetAdmin() && (standalone || user.GetDisabled()) {
		ui.Row(c).Children(func() {
			if dangerButton(c, style.T(c, "action.delete"), us.busy != "") {
				f.confirming = true
			}
		})
	}
	title := style.T(c, "users.deleteTitle", "name", user.GetName())
	if ui.AlertDialog(c, &f.confirming, title, style.T(c, "users.deleteMessage"), style.T(c, "action.cancel"), style.T(c, "action.delete")) == 1 {
		id := user.GetId()
		a.userAction("delete", func(ctx context.Context, s userv1connect.UserServiceClient) error {
			_, err := s.DeleteUser(ctx, userv1.DeleteUserRequest_builder{Id: &id}.Build())
			return err
		}, func(err error) {
			f.err = err
			if err == nil {
				a.replace(usersPath(""))
			}
		})
	}
}

// accessGroup builds what a user may see: the libraries, the ratings and
// labels, and downloads.
func (a *App) accessGroup(c *ui.Context, d *userDraft, own bool) {
	us := &a.settingsState.admin.people
	footer := ""
	if !own {
		footer = style.T(c, "users.accessMavioDev")
	}
	libs := a.adminLibraries()
	ratings := adminTask(a, &us.ratings, "ratings", func(ctx context.Context, s string) ([]*systemv1.Rating, error) {
		res, err := a.localizationService(s).ListRatings(ctx, &systemv1.ListRatingsRequest{})
		return res.GetRatings(), err
	})
	group(c, "access", style.T(c, "users.access"), footer, func(g *settingsGroup) {
		g.toggleRow(style.T(c, "users.allLibraries"), "", &d.allLibraries, !own)
		if !d.allLibraries {
			for _, l := range libs.value {
				if !accessible(l) {
					continue
				}
				on := slices.Contains(d.libraryIDs, l.GetId())
				if g.checkRow(l.GetName(), &on, !own) {
					if on {
						d.libraryIDs = append(d.libraryIDs, l.GetId())
					} else {
						d.libraryIDs = slices.DeleteFunc(slices.Clone(d.libraryIDs), func(x string) bool { return x == l.GetId() })
					}
				}
			}
		}
		g.row(style.T(c, "users.maxRating"), style.T(c, "users.maxRatingDetail"), nil, func() {
			// Ratings by the age they stand for, a few of their names
			// beside it; those above adult content are left to no limit.
			var scores []int32
			for _, r := range ratings.value {
				if r.GetScore() <= adultScore && !slices.Contains(scores, r.GetScore()) {
					scores = append(scores, r.GetScore())
				}
			}
			if d.hasMax && !slices.Contains(scores, d.maxRating) {
				scores = append(scores, d.maxRating)
			}
			slices.Sort(scores)
			labels := []string{style.T(c, "users.noLimit")}
			for _, s := range scores {
				var names []string
				for _, r := range ratings.value {
					if r.GetScore() == s {
						names = append(names, r.GetName())
					}
				}
				age := style.T(c, "users.ratingAge", "age", s)
				switch {
				case s == 0:
					age = style.T(c, "users.ratingAll")
				case s >= adultScore:
					age = style.T(c, "users.ratingAdult")
				}
				if len(names) > 0 && len(names) <= 3 {
					age = fmt.Sprintf("%s (%s)", age, strings.Join(names, ", "))
				}
				labels = append(labels, age)
			}
			i := 0
			if d.hasMax {
				i = slices.Index(scores, d.maxRating) + 1
			}
			if choose(c, style.T(c, "users.maxRating"), &i, labels) && own {
				d.hasMax = i > 0
				if d.hasMax {
					d.maxRating = scores[i-1]
				}
			}
		})
		g.toggleRow(style.T(c, "users.blockUnrated"), "", &d.blockUnrated, !own)
		g.row(style.T(c, "users.allowedLabels"), style.T(c, "users.allowedLabelsDetail"), nil, func() {
			ui.TextInput(c, &d.allowedLabels).Width(288).Disabled(!own).Label(style.T(c, "users.allowedLabels"))
		})
		g.row(style.T(c, "users.blockedLabels"), style.T(c, "users.blockedLabelsDetail"), nil, func() {
			ui.TextInput(c, &d.blockedLabels).Width(288).Disabled(!own).Label(style.T(c, "users.blockedLabels"))
		})
		g.toggleRow(style.T(c, "users.allowDownload"), "", &d.allowDownload, !own)
	})
}

// quotaRow adds a row of a user's quota of requests: the server's, or
// its own, of a number of requests every so many days.
func quotaRow(g *settingsGroup, name, label string, q **quota) {
	c := g.c
	detail := ""
	if *q == nil {
		detail = style.T(c, "users.quotaServer")
	}
	g.row(label, detail, nil, func() {
		ui.Row(c.Key(name)).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
			if v := *q; v != nil {
				ui.NumberInput(c.Key("limit"), &v.limit, 0, 1000, 1).Label(style.T(c, "users.quotaLimit"))
				secondaryText(c, style.T(c, "users.quotaEvery"))
				ui.NumberInput(c.Key("days"), &v.days, 0, 1000, 1).Label(style.T(c, "users.quotaDays"))
				secondaryText(c, style.T(c, "users.quotaDaysUnit"))
			}
			on := *q != nil
			if ui.Switch(c, &on).Label(style.T(c, "users.quotaCustom")).Changed() {
				*q = nil
				if on {
					*q = &quota{limit: 10, days: 7}
				}
			}
		})
	})
}

// signIns builds a user's sign-ins: its password, PIN, authenticator and
// passkeys, and an invitation that converts a managed user.
func (a *App) signIns(c *ui.Context, user *userv1.User) {
	us := &a.settingsState.admin.people
	f := &us.form
	id := user.GetId()
	self := id == a.chosenServer().GetUserId()
	done := func(notice string) func(err error) {
		return func(err error) {
			f.signErr = err
			if err == nil {
				f.signNotice = style.T(c, notice)
			}
		}
	}
	footer := f.signNotice
	if f.signErr != nil {
		footer = errorMessage(c, f.signErr)
	}
	settable := user.GetAuthProvider() == "" && !self
	if !settable && !user.GetManaged() && !user.GetHasTotp() && len(user.GetPasskeys()) == 0 {
		return
	}
	group(c, "signIns", style.T(c, "users.signIns"), footer, func(g *settingsGroup) {
		// Users change their own password, with the current one, in
		// their account's settings.
		if settable {
			label, detail, notice := style.T(c, "users.setPassword"), style.T(c, "users.setPasswordDetail"), "users.passwordSet"
			if user.GetManaged() {
				label, detail, notice = style.T(c, "users.givePassword"), style.T(c, "users.givePasswordDetail"), "users.converted"
			}
			g.row(label, detail, nil, func() {
				ui.Row(c.Key("password")).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.TextInput(c, &f.password).Width(224).Password().Placeholder(style.T(c, "auth.password")).Label(style.T(c, "auth.password"))
					if smallButton(c, style.T(c, "users.set"), f.password == "" || us.busy != "") {
						password := f.password
						a.userAction("password", func(ctx context.Context, s userv1connect.UserServiceClient) error {
							_, err := s.SetPassword(ctx, userv1.SetPasswordRequest_builder{UserId: &id, NewPassword: &password}.Build())
							return err
						}, func(err error) {
							if err == nil {
								f.password = ""
							}
							done(notice)(err)
						})
					}
				})
			})
		}
		if user.GetManaged() {
			detail := style.T(c, "users.pinOff")
			if user.GetHasPin() {
				detail = style.T(c, "users.pinOn")
			}
			setPin := func(pin string) {
				notice := "users.pinCleared"
				if pin != "" {
					notice = "users.pinSet"
				}
				a.userAction("pin", func(ctx context.Context, s userv1connect.UserServiceClient) error {
					_, err := s.SetPin(ctx, userv1.SetPinRequest_builder{UserId: &id, Pin: &pin}.Build())
					return err
				}, func(err error) {
					if err == nil {
						f.pin = ""
					}
					done(notice)(err)
				})
			}
			g.row(style.T(c, "users.pin"), detail, nil, func() {
				ui.Row(c.Key("pin")).Gap(8).AlignItems(ui.Center).Children(func() {
					pinInput(c, &f.pin, "0000")
					if smallButton(c, style.T(c, "users.set"), len(f.pin) != 4 || us.busy != "") {
						setPin(f.pin)
					}
					if user.GetHasPin() && linkButton(c.Key("clear"), style.T(c, "users.clear")).Clicked() && us.busy == "" {
						setPin("")
					}
				})
			})
			detail = style.T(c, "users.inviteManagedDetail")
			if f.link != "" {
				detail = f.link
			}
			g.row(style.T(c, "users.inviteManaged"), detail, nil, func() {
				if f.link != "" {
					a.copyButton(c, f.link)
					return
				}
				if smallButton(c, style.T(c, "users.createLink"), us.busy != "") {
					var token string
					a.userAction("invite", func(ctx context.Context, s userv1connect.UserServiceClient) error {
						res, err := s.CreateInvitation(ctx, userv1.CreateInvitationRequest_builder{
							UserId: &id, MaxUses: new(int32(1)), ValidFor: durationpb.New(7 * 24 * time.Hour),
						}.Build())
						token = res.GetToken()
						return err
					}, func(err error) {
						f.signErr = err
						if err == nil {
							f.link = a.invitationLink(token)
						}
					})
				}
			})
		}
		if user.GetHasTotp() {
			g.row(style.T(c, "users.totp"), style.T(c, "users.totpDetail"), nil, func() {
				if dangerButton(c, style.T(c, "users.turnOff"), us.busy != "") {
					a.userAction("totp", func(ctx context.Context, s userv1connect.UserServiceClient) error {
						_, err := s.DeleteTotp(ctx, userv1.DeleteTotpRequest_builder{UserId: &id}.Build())
						return err
					}, func(err error) { f.signErr = err })
				}
			})
		}
		for _, k := range user.GetPasskeys() {
			var detail []string
			if k.GetRpId() != "" {
				detail = append(detail, k.GetRpId())
			}
			if k.HasLastUseTime() {
				detail = append(detail, style.T(c, "users.lastUsed", "date", mediumDate(k.GetLastUseTime().AsTime())))
			}
			name := k.GetName()
			if name == "" {
				name = style.T(c, "users.passkey")
			}
			g.row(name, strings.Join(detail, " · "), nil, func() {
				if dangerButton(c.Key(k.GetId()), style.T(c, "action.delete"), us.busy != "") {
					key := k.GetId()
					a.userAction("passkey", func(ctx context.Context, s userv1connect.UserServiceClient) error {
						_, err := s.DeletePasskey(ctx, userv1.DeletePasskeyRequest_builder{UserId: &id, Id: &key}.Build())
						return err
					}, func(err error) { f.signErr = err })
				}
			})
		}
	})
}

// invitationLink returns the link of an invitation's token on the server
// chosen.
func (a *App) invitationLink(token string) string {
	return strings.TrimSuffix(a.chosenServer().GetAddress(), "/") + "/invite#" + url.PathEscape(token)
}

// copyButton builds a button copying a link.
func (a *App) copyButton(c *ui.Context, link string) {
	us := &a.settingsState.admin.people
	label := style.T(c, "users.copyLink")
	if us.copied == link {
		label = style.T(c, "users.copied")
	}
	if smallButton(c.Key(link), label, false) {
		c.WriteClipboard(link)
		us.copied = link
	}
}

// invitationsGroup builds the server's invitation links: making one,
// which shows its link once, and revoking them; and the links made for
// managed users.
func (a *App) invitationsGroup(c *ui.Context, users []*userv1.User) {
	us := &a.settingsState.admin.people
	f := &us.invites
	t := adminTask(a, &us.invitations, "invitations", func(ctx context.Context, s string) ([]*userv1.Invitation, error) {
		res, err := a.userService(s).ListInvitations(ctx, &userv1.ListInvitationsRequest{})
		return res.GetInvitations(), err
	})
	// Links used up can no longer be accepted.
	var list, conversions []*userv1.Invitation
	for _, i := range t.value {
		switch {
		case i.GetUses() >= i.GetMaxUses():
		case i.GetUserId() != "":
			conversions = append(conversions, i)
		default:
			list = append(list, i)
		}
	}
	// The link made is shown until its invitation is revoked or used up.
	link := ""
	if f.link != "" && slices.ContainsFunc(list, func(i *userv1.Invitation) bool { return i.GetId() == f.madeID }) {
		link = f.link
	}
	footer := style.T(c, "users.invitationsFooter")
	if err := cmp.Or(t.err, f.err); err != nil {
		footer = errorMessage(c, err)
	}
	control := func() {
		if f.making {
			return
		}
		b := ui.ButtonBase(c).Gap(4).AlignItems(ui.Center).Label(style.T(c, "users.invite"))
		colors := style.Colors(c)
		b.Children(func() {
			ui.Icon(c, style.Icon("add", false)).Size(18, 18).TextColor(style.Color(colors.AccentText))
			style.Text(ui.Text(c, style.T(c, "users.invite")), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.AccentText))
		})
		if b.Clicked() {
			*f = invitesForm{making: true, uses: 1, days: 7, all: true, download: true}
		}
	}
	revoke := func(id string) {
		a.userAction("revoke", func(ctx context.Context, s userv1connect.UserServiceClient) error {
			_, err := s.DeleteInvitation(ctx, userv1.DeleteInvitationRequest_builder{Id: &id}.Build())
			return err
		}, func(err error) { f.err = err })
	}
	groupWith(c, "invitations", style.T(c, "users.invitations"), control, footer, func(g *settingsGroup) {
		if link != "" {
			g.row(style.T(c, "users.linkMade"), link, nil, func() { a.copyButton(c, link) })
		}
		if f.making {
			g.row(style.T(c, "users.maxUses"), "", nil, func() {
				ui.NumberInput(c, &f.uses, 1, 100, 1).Label(style.T(c, "users.maxUses"))
			})
			g.row(style.T(c, "users.validDays"), "", nil, func() {
				ui.NumberInput(c, &f.days, 1, 30, 1).Label(style.T(c, "users.validDays"))
			})
			g.switchRow(style.T(c, "users.allLibraries"), "", &f.all)
			if !f.all {
				for _, l := range a.adminLibraries().value {
					if !accessible(l) {
						continue
					}
					on := slices.Contains(f.ids, l.GetId())
					if g.checkRow(l.GetName(), &on, false) {
						if on {
							f.ids = append(f.ids, l.GetId())
						} else {
							f.ids = slices.DeleteFunc(f.ids, func(x string) bool { return x == l.GetId() })
						}
					}
				}
			}
			g.switchRow(style.T(c, "users.allowDownload"), "", &f.download)
			g.block(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					if ui.PrimaryButton(c, style.T(c, "users.createLink")).Disabled(us.busy != "").Clicked() {
						access := userv1.UserPolicy_builder{AllLibraries: new(f.all), AllowDownload: new(f.download)}
						if !f.all {
							access.LibraryIds = slices.Clone(f.ids)
						}
						req := userv1.CreateInvitationRequest_builder{
							MaxUses: new(int32(f.uses)), ValidFor: durationpb.New(time.Duration(f.days) * 24 * time.Hour), Access: access.Build(),
						}.Build()
						var res *userv1.CreateInvitationResponse
						a.userAction("invite", func(ctx context.Context, s userv1connect.UserServiceClient) (err error) {
							res, err = s.CreateInvitation(ctx, req)
							return err
						}, func(err error) {
							f.err = err
							if err == nil {
								f.making, f.madeID, f.link = false, res.GetInvitation().GetId(), a.invitationLink(res.GetToken())
							}
						})
					}
					if linkButton(c, style.T(c, "action.cancel")).Clicked() {
						f.making = false
					}
				})
			})
		}
		for _, i := range list {
			label := style.T(c, "users.invitationUses", "uses", i.GetUses(), "max", i.GetMaxUses())
			a.invitationRow(c, g, i, label, revoke)
		}
		if t.loaded && t.err == nil && len(list) == 0 && !f.making && link == "" {
			g.note(style.T(c, "users.noInvitations"), false)
		}
	})
	if len(conversions) > 0 {
		group(c, "conversions", style.T(c, "users.conversionLinks"), "", func(g *settingsGroup) {
			for _, i := range conversions {
				label := i.GetUserId()
				if j := slices.IndexFunc(users, func(u *userv1.User) bool { return u.GetId() == i.GetUserId() }); j >= 0 {
					label = users[j].GetName()
				}
				a.invitationRow(c, g, i, label, revoke)
			}
		})
	}
}

func (a *App) invitationRow(c *ui.Context, g *settingsGroup, i *userv1.Invitation, label string, revoke func(id string)) {
	detail := style.T(c, "users.expires", "date", mediumDate(i.GetExpireTime().AsTime()))
	if i.HasCreateTime() {
		detail += " · " + style.T(c, "users.created", "date", mediumDate(i.GetCreateTime().AsTime()))
	}
	g.row(label, detail, nil, func() {
		if dangerButton(c.Key(i.GetId()), style.T(c, "users.revoke"), a.settingsState.admin.people.busy != "") {
			revoke(i.GetId())
		}
	})
}

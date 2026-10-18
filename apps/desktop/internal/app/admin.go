package app

import (
	"cmp"
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	playbackv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/playback/v1"
	sessionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/session/v1"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// adminState is the state of the sections of a server's administration.
type adminState struct {
	reload int // counts the changes the sections made

	info        task[*systemv1.GetSystemInfoResponse]
	health      task[*systemv1.GetHealthResponse]
	users       task[[]*userv1.User]
	people      usersState
	server      serverSettingsState
	proxies     proxiesState
	storage     storageState
	acquisition acquisitionState
	wanted      wantedState
	downloads   downloadsState
	subtitles   subtitlesState
	libraries   librariesState
	libs        task[[]*libraryv1.Library]
	sessions    task[[]*sessionv1.Session]

	activities    task[*systemv1.ListActivitiesResponse]
	severity      int // the index of the severities shown
	activityLimit int
	tasks         task[[]*systemv1.Task]
	jobs          task[*systemv1.ListJobsResponse]
	jobFilter     int
	jobLimit      int
	logs          task[[]*systemv1.LogRecord]
	logLevel      int
	logPeriod     int
	logFilter     string
	logRefresh    int
	notices       map[string]string // of tasks run and libraries scanned
	running       string            // the task or library being started
}

func (a *App) systemService(id string) systemv1connect.SystemServiceClient {
	return systemv1connect.NewSystemServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) taskService(id string) systemv1connect.TaskServiceClient {
	return systemv1connect.NewTaskServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) activityService(id string) systemv1connect.ActivityServiceClient {
	return systemv1connect.NewActivityServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) libraryService(id string) libraryv1connect.LibraryServiceClient {
	return libraryv1connect.NewLibraryServiceClient(a.connectHTTP(), serverURL(id))
}

// fromNow tells how long ago, or how soon, a time is, as the web UI's
// relative times.
func fromNow(c *ui.Context, t time.Time) string {
	d := time.Until(t)
	past := d < 0
	if past {
		d = -d
	}
	var unit string
	var n int
	switch {
	case d < time.Minute:
		return style.T(c, "time.justNow")
	case d < time.Hour:
		unit, n = "Minutes", int((d+30*time.Second)/time.Minute)
	case d < 24*time.Hour:
		unit, n = "Hours", int((d+30*time.Minute)/time.Hour)
	default:
		unit, n = "Days", int((d+12*time.Hour)/(24*time.Hour))
	}
	if past {
		return style.T(c, "time."+strings.ToLower(unit)+"Ago", "count", n)
	}
	return style.T(c, "time.in"+unit, "count", n)
}

// lengthOf tells a length of time as its largest whole unit or two, as
// "2 hr 5 min" or "3 days".
func lengthOf(c *ui.Context, d time.Duration) string {
	f := func(unit string, n int) string { return style.T(c, "time."+unit, "count", n) }
	m := int((d + 30*time.Second) / time.Minute)
	switch {
	case m < 1:
		return f("seconds", max(0, int(d/time.Second)))
	case m < 60:
		return f("minutes", m)
	}
	h := m / 60
	if h < 24 {
		if m%60 != 0 {
			return f("hours", h) + " " + f("minutes", m%60)
		}
		return f("hours", h)
	}
	if h%24 != 0 {
		return f("days", h/24) + " " + f("hours", h%24)
	}
	return f("days", h/24)
}

// dot builds a small round mark of a color.
func dot(c *ui.Context, color ui.Color) {
	ui.Box(c).Size(8, 8).Shrink(0).Radius(4).Background(color)
}

// note adds a row of a line of text to a group: an empty list's, or an
// error's when danger.
func (g *settingsGroup) note(text string, danger bool) {
	c := g.c
	g.divider()
	colors := style.Colors(c)
	color := colors.TextSecondary
	if danger {
		color = colors.Danger
	}
	style.Text(ui.Text(c.Key(g.rows), text), style.Type.Callout).TextColor(style.Color(color)).Padding(12, 16).Selectable()
}

// more adds a row showing more of a list.
func (g *settingsGroup) more(label string) bool {
	c := g.c
	g.divider()
	colors := style.Colors(c)
	b := ui.ButtonBase(c.Key(g.rows)).MinHeight(44).Padding(0, 16).AlignItems(ui.Center).Label(label)
	if b.Hovered() {
		b.Background(style.Color(colors.Fill))
	}
	b.Children(func() {
		style.Text(ui.Text(c, label), style.Type.Body).TextColor(style.Color(colors.AccentText))
	})
	return b.Clicked()
}

// smallButton builds a small button of the secondary look, as the web UI's.
func smallButton(c *ui.Context, label string, disabled bool) bool {
	colors := style.Colors(c)
	b := ui.ButtonBase(c).Height(32).Padding(0, 14).Radius(8).AlignItems(ui.Center).Label(label).Disabled(disabled).
		Background(style.Color(colors.Fill))
	if b.Hovered() && !disabled {
		b.Background(style.Color(colors.FillSelected))
	}
	if disabled {
		b.Opacity(0.5)
	}
	b.Children(func() {
		style.Text(ui.Text(c, label), style.Type.Callout).FontWeight(600).TextColor(style.Color(colors.TextPrimary))
	})
	return b.Clicked() && !disabled
}

// adminTask loads a value of the server chosen for a section, again as
// the sections change it.
func adminTask[T any](a *App, t *task[T], name string, fn func(ctx context.Context, server string) (T, error), parts ...any) *task[T] {
	server := a.chosenServer().GetId()
	k := key(append([]any{name, server, a.generation, a.settingsState.admin.reload}, parts...)...)
	return load(a, t, k, func(ctx context.Context) (T, error) { return fn(ctx, server) })
}

// startAction runs a change of a section in the background: what is
// keyed id, which shows busy until it ends; done takes its result on the
// main thread.
func (a *App) startAction(id string, fn func(ctx context.Context) (string, error), done func(notice string, err error)) {
	st := &a.settingsState.admin
	st.running = id
	go func() {
		notice, err := fn(a.ctx)
		a.opts.Update(func() {
			st.running = ""
			done(notice, err)
		})
	}()
}

func (a *App) overviewSection(c *ui.Context) {
	colors := style.Colors(c)
	st := &a.settingsState.admin
	sectionHeader(c, style.T(c, "settings.overview"), "")
	info := adminTask(a, &st.info, "info", func(ctx context.Context, s string) (*systemv1.GetSystemInfoResponse, error) {
		return a.systemService(s).GetSystemInfo(ctx, &systemv1.GetSystemInfoRequest{})
	})
	health := adminTask(a, &st.health, "health", func(ctx context.Context, s string) (*systemv1.GetHealthResponse, error) {
		return a.systemService(s).GetHealth(ctx, &systemv1.GetHealthRequest{})
	})
	group(c, "server", style.T(c, "overview.server"), "", func(g *settingsGroup) {
		g.row(style.T(c, "overview.status"), "", nil, func() {
			serving := health.value.GetStatus() == systemv1.GetHealthResponse_STATUS_SERVING
			color, text := style.Color(colors.Danger), style.T(c, "overview.notServing")
			switch {
			case !health.loaded:
				color, text = style.Color(colors.Fill), "…"
			case health.err != nil:
				text = errorMessage(c, health.err)
			case serving:
				color, text = ui.RGB(0x34, 0xC7, 0x59), style.T(c, "overview.serving")
			}
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				dot(c, color)
				style.Text(ui.Text(c, text), style.Type.Callout).TextColor(style.Color(colors.TextSecondary))
			})
		})
		if !info.loaded {
			return
		}
		if info.err != nil {
			g.note(errorMessage(c, info.err), true)
			return
		}
		i := info.value
		database := map[string]string{"postgres": "PostgreSQL", "sqlite": "SQLite"}[i.GetDatabase()]
		ffmpeg := cmp.Or(i.GetFfmpegVersion(), style.T(c, "overview.ffmpegMissing"))
		facts := [][2]string{
			{style.T(c, "overview.version"), i.GetVersion()},
			{style.T(c, "overview.platform"), i.GetOs() + "/" + i.GetArch()},
			{style.T(c, "overview.database"), cmp.Or(database, i.GetDatabase())},
			{style.T(c, "overview.ffmpeg"), ffmpeg},
		}
		if i.HasStartTime() {
			facts = append(facts, [2]string{style.T(c, "overview.uptime"), lengthOf(c, time.Since(i.GetStartTime().AsTime()))})
		}
		for _, f := range facts {
			if f[1] == "" {
				continue
			}
			g.row(f[0], "", nil, func() {
				style.Text(ui.Text(c, f[1]), style.Type.Callout).TextColor(style.Color(colors.TextSecondary)).TextAlign(ui.End).MaxWidth(480).Selectable()
			})
		}
	})
}

// libraryKinds are how libraries of each kind show.
var libraryKinds = map[libraryv1.LibraryKind]struct {
	title, icon string
	color       ui.Color
}{
	libraryv1.LibraryKind_LIBRARY_KIND_MOVIES:       {"libraries.kindMovies", "movies", ui.RGB(0xFF, 0x95, 0x00)},
	libraryv1.LibraryKind_LIBRARY_KIND_SHOWS:        {"libraries.kindShows", "shows", ui.RGB(0x00, 0x7A, 0xFF)},
	libraryv1.LibraryKind_LIBRARY_KIND_MUSIC:        {"libraries.kindMusic", "music", ui.RGB(0xFF, 0x2D, 0x55)},
	libraryv1.LibraryKind_LIBRARY_KIND_MUSIC_VIDEOS: {"libraries.kindMusicVideos", "music", ui.RGB(0xAF, 0x52, 0xDE)},
	libraryv1.LibraryKind_LIBRARY_KIND_HOME_VIDEOS:  {"libraries.kindHomeVideos", "homeVideos", ui.RGB(0x34, 0xC7, 0x59)},
	libraryv1.LibraryKind_LIBRARY_KIND_BOOKS:        {"libraries.kindBooks", "books", ui.RGB(0xA2, 0x84, 0x5E)},
	libraryv1.LibraryKind_LIBRARY_KIND_PHOTOS:       {"libraries.kindPhotos", "photos", ui.RGB(0x32, 0xAD, 0xE6)},
	libraryv1.LibraryKind_LIBRARY_KIND_MIXED:        {"libraries.kindMixed", "folder", ui.RGB(0x8E, 0x8E, 0x93)},
}

// notice keeps the notice of an action on what is keyed id: the string
// notice, or the error.
func (a *App) notice(c *ui.Context, id, notice string, err error) {
	st := &a.settingsState.admin
	if st.notices == nil {
		st.notices = map[string]string{}
	}
	if err != nil {
		st.notices[id] = errorMessage(c, err)
		return
	}
	st.notices[id] = style.T(c, notice)
	st.reload++
}

// playMethods name how a session plays.
var playMethods = map[playbackv1.PlayMethod]string{
	playbackv1.PlayMethod_PLAY_METHOD_DIRECT_PLAY:   "activity.directPlay",
	playbackv1.PlayMethod_PLAY_METHOD_DIRECT_STREAM: "activity.directStream",
	playbackv1.PlayMethod_PLAY_METHOD_TRANSCODE:     "activity.transcode",
}

// severities are the activities shown: all, or warnings and errors, or
// errors.
var severities = []struct {
	min   systemv1.Severity
	label string
}{
	{systemv1.Severity_SEVERITY_UNSPECIFIED, "activity.activityAll"},
	{systemv1.Severity_SEVERITY_WARNING, "activity.activityWarnings"},
	{systemv1.Severity_SEVERITY_ERROR, "activity.activityErrors"},
}

func severityColor(c *ui.Context, s systemv1.Severity) ui.Color {
	switch s {
	case systemv1.Severity_SEVERITY_WARNING:
		return ui.RGB(0xFF, 0x95, 0x00)
	case systemv1.Severity_SEVERITY_ERROR:
		return ui.RGB(0xFF, 0x3B, 0x30)
	}
	return style.Color(style.Colors(c).TextTertiary)
}

func (a *App) activitySection(c *ui.Context) {
	colors := style.Colors(c)
	st := &a.settingsState.admin
	sectionHeader(c, style.T(c, "settings.activity"), style.T(c, "settings.activityMessage"))
	sessions := adminTask(a, &st.sessions, "allSessions", func(ctx context.Context, s string) ([]*sessionv1.Session, error) {
		res, err := a.sessionService(s).ListSessions(ctx, sessionv1.ListSessionsRequest_builder{AllUsers: new(true)}.Build())
		return res.GetSessions(), err
	})
	group(c, "playing", style.T(c, "activity.nowPlaying"), "", func(g *settingsGroup) {
		var playing []*sessionv1.Session
		for _, s := range sessions.value {
			if s.GetNowPlaying().GetItem() != nil {
				playing = append(playing, s)
			}
		}
		switch {
		case sessions.err != nil:
			g.note(errorMessage(c, sessions.err), true)
		case sessions.loaded && len(playing) == 0:
			g.note(style.T(c, "activity.nothingPlaying"), false)
		}
		for _, s := range playing {
			np := s.GetNowPlaying()
			item := np.GetItem()
			g.block(func() {
				ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
					icon, color := "play", ui.RGB(0x34, 0xC7, 0x59)
					if np.GetPaused() {
						icon, color = "pause", ui.RGB(0x8E, 0x8E, 0x93)
					}
					sectionTile(c, icon, color)
					ui.Column(c).Grow(1).MinWidth(0).Children(func() {
						title := item.GetName()
						if sub := a.cardOf(a.chosenServer().GetId(), item).GetSubtitle(); sub != "" {
							title += " · " + sub
						}
						style.Text(ui.Text(c, title), style.Type.Body).TextColor(style.Color(colors.TextPrimary)).SingleLine()
						var who []string
						for _, w := range []string{cmp.Or(s.GetUserName(), s.GetPluginId()), s.GetDeviceName(), s.GetClient()} {
							if w != "" {
								who = append(who, w)
							}
						}
						style.Text(ui.Text(c, strings.Join(who, " · ")), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).SingleLine()
					})
					if m, ok := playMethods[np.GetMethod()]; ok {
						style.Text(ui.Text(c, style.T(c, m)), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary))
					}
				})
				runtime, position := item.GetRuntime().AsDuration(), np.GetPosition().AsDuration()
				if runtime > 0 {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Padding(0, 0, 0, 40).Children(func() {
						ui.Box(c).Grow(1).Height(4).Radius(2).Clip().Background(style.Color(colors.Fill)).Children(func() {
							ui.Box(c).Height(4).Width(0).Basis(0).Grow(float32(min(1, position.Seconds()/runtime.Seconds()))).Radius(2).Background(style.Color(colors.Accent))
							ui.Box(c).Basis(0).Grow(float32(max(0, 1-position.Seconds()/runtime.Seconds())))
						}).Row()
						style.Text(ui.Text(c, clock(position)+" / "+clock(runtime)), style.Type.Caption).TextColor(style.Color(colors.TextSecondary))
					})
				}
			})
		}
	})
	if st.activityLimit == 0 {
		st.activityLimit = 20
	}
	min := severities[st.severity].min
	limit := int32(st.activityLimit)
	t := adminTask(a, &st.activities, "activities", func(ctx context.Context, s string) (*systemv1.ListActivitiesResponse, error) {
		return a.activityService(s).ListActivities(ctx, systemv1.ListActivitiesRequest_builder{MinSeverity: &min, Limit: &limit}.Build())
	}, st.severity, st.activityLimit)
	filter := func() {
		labels := make([]string, len(severities))
		for i, s := range severities {
			labels[i] = style.T(c, s.label)
		}
		choose(c, style.T(c, "activity.activityShown"), &st.severity, labels)
	}
	groupWith(c, "activities", style.T(c, "activity.recent"), filter, "", func(g *settingsGroup) {
		list := t.value.GetActivities()
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(list) == 0:
			g.note(style.T(c, "activity.noActivity"), false)
		}
		for _, act := range list {
			g.divider()
			ui.Row(c.Key(act.GetId())).Padding(10, 16).Gap(12).AlignItems(ui.Start).Children(func() {
				ui.Box(c).Padding(6, 0, 0).Children(func() { dot(c, severityColor(c, act.GetSeverity())) })
				ui.Column(c).Grow(1).MinWidth(0).Children(func() {
					style.Text(ui.Text(c, act.GetTitle()), style.Type.Body).TextColor(style.Color(colors.TextPrimary))
					if act.GetMessage() != "" {
						style.Text(ui.Text(c, act.GetMessage()), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).Selectable()
					}
				})
				if act.HasTime() {
					style.Text(ui.Text(c, fromNow(c, act.GetTime().AsTime())), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary)).
						Tooltip(mediumDateTime(act.GetTime().AsTime()))
				}
			})
		}
		if int(t.value.GetTotal()) > len(list) && g.more(style.T(c, "activity.showMore")) {
			st.activityLimit += 50
		}
	})
}

// jobStates are how jobs of each state show.
var jobStates = map[systemv1.JobState]struct {
	title string
	color ui.Color
}{
	systemv1.JobState_JOB_STATE_PENDING:   {"tasks.pending", ui.RGB(0x8E, 0x8E, 0x93)},
	systemv1.JobState_JOB_STATE_RUNNING:   {"tasks.running", ui.RGB(0x00, 0x7A, 0xFF)},
	systemv1.JobState_JOB_STATE_SUCCEEDED: {"tasks.succeeded", ui.RGB(0x34, 0xC7, 0x59)},
	systemv1.JobState_JOB_STATE_FAILED:    {"tasks.failed", ui.RGB(0xFF, 0x3B, 0x30)},
}

// jobFilters are the jobs shown, by their states.
var jobFilters = []struct {
	label  string
	states []systemv1.JobState
}{
	{"tasks.jobsAll", nil},
	{"tasks.jobsActive", []systemv1.JobState{systemv1.JobState_JOB_STATE_PENDING, systemv1.JobState_JOB_STATE_RUNNING}},
	{"tasks.jobsFailed", []systemv1.JobState{systemv1.JobState_JOB_STATE_FAILED}},
	{"tasks.jobsSucceeded", []systemv1.JobState{systemv1.JobState_JOB_STATE_SUCCEEDED}},
}

func ago(c *ui.Context, ts *timestamppb.Timestamp) string {
	if ts == nil {
		return ""
	}
	return fromNow(c, ts.AsTime())
}

func (a *App) tasksSection(c *ui.Context) {
	st := &a.settingsState.admin
	sectionHeader(c, style.T(c, "settings.tasks"), style.T(c, "tasks.message"))
	t := adminTask(a, &st.tasks, "tasks", func(ctx context.Context, s string) ([]*systemv1.Task, error) {
		res, err := a.taskService(s).ListTasks(ctx, &systemv1.ListTasksRequest{})
		return res.GetTasks(), err
	})
	if t.err != nil {
		style.Text(ui.Text(c, errorMessage(c, t.err)), style.Type.Callout).TextColor(style.Color(style.Colors(c).Danger))
	}
	var own, plugins []*systemv1.Task
	for _, task := range t.value {
		if task.GetPluginId() == "" {
			own = append(own, task)
		} else {
			plugins = append(plugins, task)
		}
	}
	byName := func(x, y *systemv1.Task) int {
		return strings.Compare(strings.ToLower(x.GetName()), strings.ToLower(y.GetName()))
	}
	slices.SortFunc(own, byName)
	slices.SortFunc(plugins, byName)
	a.taskGroup(c, "own", style.T(c, "tasks.server"), own)
	a.taskGroup(c, "plugins", style.T(c, "settings.plugins"), plugins)
	a.jobs(c)
}

func (a *App) taskGroup(c *ui.Context, k, title string, tasks []*systemv1.Task) {
	if len(tasks) == 0 {
		return
	}
	colors := style.Colors(c)
	st := &a.settingsState.admin
	server := a.chosenServer().GetId()
	group(c, k, title, "", func(g *settingsGroup) {
		for _, task := range tasks {
			last := task.GetLastRun()
			status := style.T(c, "tasks.neverRun")
			color := style.Color(colors.Fill)
			switch {
			case task.GetRunning():
				status, color = style.T(c, "tasks.runningNow"), jobStates[systemv1.JobState_JOB_STATE_RUNNING].color
			case last != nil:
				s := jobStates[last.GetState()]
				status, color = strings.TrimSpace(style.T(c, s.title)+" "+ago(c, cmp.Or(last.GetFinishTime(), last.GetRunTime()))), s.color
			}
			if n, ok := st.notices[task.GetId()]; ok && !task.GetRunning() {
				status = n
			}
			every := style.T(c, "tasks.onDemand")
			if task.HasInterval() {
				every = style.T(c, "tasks.every", "length", lengthOf(c, task.GetInterval().AsDuration()))
			}
			line := status + " · " + every
			if task.HasNextRunTime() && !task.GetRunning() {
				line += " · " + style.T(c, "tasks.next", "when", ago(c, task.GetNextRunTime()))
			}
			g.divider()
			ui.Row(c.Key(task.GetId())).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					style.Text(ui.Text(c, task.GetName()), style.Type.Body).TextColor(style.Color(colors.TextPrimary))
					if d := task.GetDescription(); d != "" {
						style.Text(ui.Text(c, d), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary))
					}
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						dot(c, color)
						style.Text(ui.Text(c, line), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary))
					})
					if last.GetState() == systemv1.JobState_JOB_STATE_FAILED && last.GetLastError() != "" && !task.GetRunning() {
						style.Text(ui.Text(c, last.GetLastError()), style.Type.Footnote).TextColor(style.Color(colors.Danger)).Selectable()
					}
				})
				id := task.GetId()
				label := style.T(c, "tasks.run")
				if st.running == id {
					label = "…"
				}
				if smallButton(c, label, st.running != "" || task.GetRunning()) {
					a.startAction(id, func(ctx context.Context) (string, error) {
						res, err := a.taskService(server).RunTask(ctx, systemv1.RunTaskRequest_builder{Id: &id}.Build())
						if res.GetEnqueued() {
							return "tasks.queued", err
						}
						return "tasks.alreadyQueued", err
					}, func(notice string, err error) { a.notice(c, id, notice, err) })
				}
			})
		}
	})
}

func (a *App) jobs(c *ui.Context) {
	colors := style.Colors(c)
	st := &a.settingsState.admin
	if st.jobLimit == 0 {
		st.jobLimit = 50
	}
	states := jobFilters[st.jobFilter].states
	limit := int32(st.jobLimit)
	t := adminTask(a, &st.jobs, "jobs", func(ctx context.Context, s string) (*systemv1.ListJobsResponse, error) {
		return a.taskService(s).ListJobs(ctx, systemv1.ListJobsRequest_builder{States: states, Limit: &limit}.Build())
	}, st.jobFilter, st.jobLimit)
	filter := func() {
		labels := make([]string, len(jobFilters))
		for i, f := range jobFilters {
			labels[i] = style.T(c, f.label)
		}
		choose(c, style.T(c, "tasks.jobsShown"), &st.jobFilter, labels)
	}
	groupWith(c, "jobs", style.T(c, "tasks.jobs"), filter, style.T(c, "tasks.jobsFooter"), func(g *settingsGroup) {
		list := t.value.GetJobs()
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(list) == 0:
			g.note(style.T(c, "tasks.noJobs"), false)
		}
		for _, j := range list {
			s := jobStates[j.GetState()]
			var when string
			switch {
			case j.GetState() == systemv1.JobState_JOB_STATE_PENDING:
				if j.HasRunTime() {
					when = style.T(c, "tasks.runs", "when", ago(c, j.GetRunTime()))
				}
			case j.HasFinishTime():
				when = ago(c, j.GetFinishTime())
			default:
				when = ago(c, cmp.Or(j.GetRunTime(), j.GetCreateTime()))
			}
			parts := []string{style.T(c, s.title)}
			if j.HasProgress() {
				parts = append(parts, fmt.Sprintf("%d%%", int(j.GetProgress()*100)))
			}
			if when != "" {
				parts = append(parts, when)
			}
			if j.GetAttempts() > 1 {
				parts = append(parts, style.T(c, "tasks.attempts", "attempts", j.GetAttempts(), "max", j.GetMaxAttempts()))
			}
			g.divider()
			ui.Column(c.Key(j.GetId())).Padding(10, 16).Gap(2).Children(func() {
				style.Text(ui.Text(c, j.GetKind()), style.Type.Body).TextColor(style.Color(colors.TextPrimary))
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					dot(c, s.color)
					style.Text(ui.Text(c, strings.Join(parts, " · ")), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary))
				})
				if j.GetLastError() != "" {
					style.Text(ui.Text(c, j.GetLastError()), style.Type.Footnote).TextColor(style.Color(colors.Danger)).Selectable()
				}
			})
		}
		if int(t.value.GetTotal()) > len(list) && g.more(style.T(c, "tasks.showMore")) {
			st.jobLimit += 100
		}
	})
}

// logLevels are the levels of the logs shown, the least severe first.
var logLevels = []struct {
	level systemv1.LogLevel
	label string
}{
	{systemv1.LogLevel_LOG_LEVEL_DEBUG, "logs.debug"},
	{systemv1.LogLevel_LOG_LEVEL_INFO, "logs.info"},
	{systemv1.LogLevel_LOG_LEVEL_WARN, "logs.warn"},
	{systemv1.LogLevel_LOG_LEVEL_ERROR, "logs.error"},
}

// logPeriods are how far back the logs go: an hour, a day, or all.
var logPeriods = []struct {
	label string
	age   time.Duration
}{
	{"logs.lastHour", time.Hour},
	{"logs.lastDay", 24 * time.Hour},
	{"logs.all", 0},
}

func logColor(c *ui.Context, l systemv1.LogLevel) ui.Color {
	colors := style.Colors(c)
	switch l {
	case systemv1.LogLevel_LOG_LEVEL_DEBUG:
		return style.Color(colors.TextTertiary)
	case systemv1.LogLevel_LOG_LEVEL_WARN:
		return ui.RGB(0xFF, 0x9F, 0x0A)
	case systemv1.LogLevel_LOG_LEVEL_ERROR:
		return style.Color(colors.Danger)
	}
	return style.Color(colors.TextSecondary)
}

func (a *App) logsSection(c *ui.Context) {
	colors := style.Colors(c)
	st := &a.settingsState.admin
	sectionHeader(c, style.T(c, "settings.logs"), style.T(c, "logs.message"))
	if st.logLevel == 0 && st.logPeriod == 0 && st.logs.key == "" {
		st.logLevel, st.logPeriod = 1, 1 // info, the last day
	}
	level := logLevels[st.logLevel].level
	b := systemv1.ListLogsRequest_builder{MinLevel: &level}
	if age := logPeriods[st.logPeriod].age; age > 0 {
		b.MaxAge = durationpb.New(age)
	}
	req := b.Build()
	t := adminTask(a, &st.logs, "logs", func(ctx context.Context, s string) ([]*systemv1.LogRecord, error) {
		res, err := a.systemService(s).ListLogs(ctx, req)
		return res.GetRecords(), err
	}, st.logLevel, st.logPeriod, st.logRefresh)
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		levels := make([]string, len(logLevels))
		for i, l := range logLevels {
			levels[i] = style.T(c, l.label)
		}
		choose(c.Key("level"), style.T(c, "logs.level"), &st.logLevel, levels)
		periods := make([]string, len(logPeriods))
		for i, p := range logPeriods {
			periods[i] = style.T(c, p.label)
		}
		choose(c.Key("period"), style.T(c, "logs.period"), &st.logPeriod, periods)
		ui.TextInput(c, &st.logFilter).Grow(1).MinWidth(160).Placeholder(style.T(c, "logs.filter")).Label(style.T(c, "logs.filter"))
		label := style.T(c, "logs.refresh")
		if t.loading {
			label = "…"
		}
		if smallButton(c, label, t.loading) {
			st.logRefresh++
		}
	})
	words := strings.Fields(strings.ToLower(st.logFilter))
	var shown []*systemv1.LogRecord
	for _, r := range t.value {
		text := strings.ToLower(r.GetMessage())
		for k, v := range r.GetAttributes() {
			text += " " + strings.ToLower(k) + " " + strings.ToLower(v)
		}
		if !slices.ContainsFunc(words, func(w string) bool { return !strings.Contains(text, w) }) {
			shown = append(shown, r)
		}
	}
	footer := ""
	if t.loaded && t.err == nil {
		footer = style.T(c, "logs.count", "count", len(shown))
	}
	group(c, "logs", "", footer, func(g *settingsGroup) {
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(shown) == 0:
			g.note(style.T(c, "logs.none"), false)
		}
		for i, r := range shown {
			g.divider()
			ui.Column(c.Key(i)).Padding(8, 16).Gap(2).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.End).Children(func() {
					if r.HasTime() {
						style.Text(ui.Text(c, r.GetTime().AsTime().Local().Format("1/2/06, 3:04:05 PM")), style.Type.Footnote).TextColor(style.Color(colors.TextSecondary))
					}
					name := ""
					for _, l := range logLevels {
						if l.level == r.GetLevel() {
							name = style.T(c, l.label)
						}
					}
					style.Text(ui.Text(c, name), style.Type.Footnote).FontWeight(600).TextColor(logColor(c, r.GetLevel()))
				})
				style.Text(ui.Text(c, r.GetMessage()), style.Type.Callout).TextColor(style.Color(colors.TextPrimary)).Selectable()
				if attrs := r.GetAttributes(); len(attrs) > 0 {
					var parts []string
					for _, k := range slices.Sorted(maps.Keys(attrs)) {
						parts = append(parts, k+"="+attrs[k])
					}
					style.Text(ui.Text(c, strings.Join(parts, "   ")), style.Type.Caption).Font("SF Mono, Menlo, monospace").TextColor(style.Color(colors.TextSecondary)).Selectable()
				}
			})
		}
	})
}

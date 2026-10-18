package app

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	design "github.com/mavioai/mavio/libs/design/gen/go"

	acquisitionv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/acquisition/v1/acquisitionv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// downloadsState is the state of the settings' Downloads: the tab shown,
// its lists, the panels open on the queue's downloads, and the files of
// manual imports.
type downloadsState struct {
	tab       int // the index of the tabs: queue, history, blocklist, import
	finished  bool
	queue     task[*acquisitionv1.ListQueueResponse]
	history   task[*acquisitionv1.ListHistoryResponse]
	blocklist task[*acquisitionv1.ListBlocklistResponse]
	limits    [3]int            // how many of the queue, history and blocklist to list
	event     int               // the index of historyTypes shown, 0 for all
	panels    map[string]string // a download's open panel: "remove" or "import"
	removals  map[string]*removal
	files     map[string]*task[*acquisitionv1.ListManualImportResponse] // by importTarget's key
	modes     map[string]int                                            // the index of importModes, by importTarget's key
	imported  map[string]bool                                           // by importTarget's key and path
	busy      string
	errs      map[string]error // of what was done to a download, an event, an entry or a file
	confirm   string           // the grabbed event marking failed is confirmed for
	path      string           // the folder typed
	folder    string           // the folder scanned
}

// removal is how a download is removed from the queue.
type removal struct{ fromClient, blocklist, search bool }

// importTarget is a tracked download or a folder whose files are imported.
type importTarget struct{ id, folder string }

func (t importTarget) key() string { return t.id + "\x00" + t.folder }

var (
	historyTypes = []acquisitionv1.HistoryEventType{
		acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_GRABBED,
		acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IMPORTED,
		acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_DOWNLOAD_FAILED,
		acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_FILE_DELETED,
		acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_FILE_RENAMED,
		acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IGNORED,
	}
	importModes = []acquisitionv1.ImportMode{
		acquisitionv1.ImportMode_IMPORT_MODE_UNSPECIFIED,
		acquisitionv1.ImportMode_IMPORT_MODE_MOVE,
		acquisitionv1.ImportMode_IMPORT_MODE_COPY,
	}
)

// downloadsPage is how many more of a list a page shows.
const downloadsPage = 50

func downloadStateName(c *ui.Context, s acquisitionv1.DownloadState) string {
	switch s {
	case acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING:
		return style.T(c, "downloads.downloading")
	case acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORT_BLOCKED:
		return style.T(c, "downloads.importBlocked")
	case acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORT_PENDING:
		return style.T(c, "downloads.importPending")
	case acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTING:
		return style.T(c, "downloads.importing")
	case acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTED:
		return style.T(c, "downloads.imported")
	case acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED_PENDING, acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED:
		return style.T(c, "downloads.failed")
	case acquisitionv1.DownloadState_DOWNLOAD_STATE_IGNORED:
		return style.T(c, "downloads.ignored")
	}
	return ""
}

func downloadStatusName(c *ui.Context, s acquisitionv1.DownloadStatus) string {
	switch s {
	case acquisitionv1.DownloadStatus_DOWNLOAD_STATUS_QUEUED:
		return style.T(c, "downloads.queued")
	case acquisitionv1.DownloadStatus_DOWNLOAD_STATUS_PAUSED:
		return style.T(c, "downloads.paused")
	case acquisitionv1.DownloadStatus_DOWNLOAD_STATUS_WARNING:
		return style.T(c, "downloads.warning")
	}
	return ""
}

func historyName(c *ui.Context, t acquisitionv1.HistoryEventType) string {
	switch t {
	case acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_GRABBED:
		return style.T(c, "downloads.grabbed")
	case acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IMPORTED:
		return style.T(c, "downloads.imported")
	case acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_DOWNLOAD_FAILED:
		return style.T(c, "downloads.failed")
	case acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_FILE_DELETED:
		return style.T(c, "downloads.fileDeleted")
	case acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_FILE_RENAMED:
		return style.T(c, "downloads.fileRenamed")
	case acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_IGNORED:
		return style.T(c, "downloads.ignored")
	}
	return ""
}

// sizeOf returns a size in bytes in gigabytes or megabytes, as the web
// UI's.
func sizeOf(b int64) string {
	n, unit := float64(b)/1e6, "MB"
	if b >= 1e9 {
		n, unit = float64(b)/1e9, "GB"
	}
	return strings.TrimSuffix(fmt.Sprintf("%.1f", n), ".0") + " " + unit
}

func (a *App) downloadsSection(c *ui.Context) {
	st := &a.settingsState.admin.downloads
	sectionHeader(c, style.T(c, "settings.downloads"), style.T(c, "downloads.message"))
	ui.Segmented(c, &st.tab, style.T(c, "downloads.queue"), style.T(c, "downloads.history"), style.T(c, "downloads.blocklist"), style.T(c, "downloads.import")).
		AlignSelf(ui.Start).Label(style.T(c, "settings.downloads"))
	for i := range st.limits {
		if st.limits[i] == 0 {
			st.limits[i] = downloadsPage
		}
	}
	switch st.tab {
	case 0:
		a.downloadQueue(c)
	case 1:
		a.downloadHistory(c)
	case 2:
		a.downloadBlocklist(c)
	default:
		a.folderImport(c)
	}
}

// downloadAction runs a change of downloads in the background, keyed
// what, which shows busy until it ends and keeps its error under key;
// done runs on the main thread if it succeeded.
func (a *App) downloadAction(what, key string, fn func(ctx context.Context, d acquisitionv1connect.DownloadServiceClient) error, done func()) {
	st := &a.settingsState.admin.downloads
	if st.errs == nil {
		st.errs = map[string]error{}
	}
	st.busy = what
	delete(st.errs, key)
	d := a.downloadService(a.chosenServer().GetId())
	go func() {
		err := fn(a.ctx, d)
		a.opts.Update(func() {
			st.busy = ""
			if err != nil {
				st.errs[key] = err
				return
			}
			a.settingsState.admin.reload++
			if done != nil {
				done()
			}
		})
	}()
}

// itemLink builds an item's name opening its details, or the name alone
// without an item, of a type.
func (a *App) itemLink(c *ui.Context, id, name string, ts design.TypeStyle) {
	colors := style.Colors(c)
	if id == "" {
		style.Text(ui.Text(c, name), ts).TextColor(style.Color(colors.TextPrimary))
		return
	}
	if name == "" {
		name = style.T(c, "downloads.item")
	}
	server := a.chosenServer().GetId()
	b := ui.ButtonBase(c).AlignSelf(ui.Start).Label(name)
	b.Children(func() {
		t := style.Text(ui.Text(c, name), ts).TextColor(style.Color(colors.TextPrimary))
		if b.Hovered() {
			t.Underline()
		}
	})
	if b.Clicked() {
		a.router.Push("/item/" + server + "/" + id)
	}
}

// pill builds a state's rounded label, of the danger color when danger.
func pill(c *ui.Context, label string, danger bool) {
	colors := style.Colors(c)
	bg, fg := style.Color(colors.Fill), style.Color(colors.TextPrimary)
	if danger {
		bg, fg = style.Color(colors.Danger), ui.RGB(0xFF, 0xFF, 0xFF)
	}
	ui.Box(c).Padding(2, 8).Radius(999).Background(bg).Children(func() {
		style.Text(ui.Text(c, label), style.Type.Caption).FontWeight(600).TextColor(fg)
	})
}

func footnote(c *ui.Context, text string, danger bool) {
	colors := style.Colors(c)
	color := colors.TextSecondary
	if danger {
		color = colors.Danger
	}
	style.Text(ui.Text(c, text), style.Type.Footnote).TextColor(style.Color(color)).Selectable()
}

func (a *App) downloadQueue(c *ui.Context) {
	st := &a.settingsState.admin.downloads
	finished, limit := st.finished, int32(st.limits[0])
	t := adminTask(a, &st.queue, "queue", func(ctx context.Context, s string) (*acquisitionv1.ListQueueResponse, error) {
		return a.downloadService(s).ListQueue(ctx, acquisitionv1.ListQueueRequest_builder{IncludeFinished: &finished, Limit: &limit}.Build())
	}, finished, limit)
	footer := ""
	if t.loaded && t.err == nil {
		footer = style.T(c, "downloads.count", "count", t.value.GetTotal())
	}
	group(c, "queue", "", footer, func(g *settingsGroup) {
		if g.switchRow(style.T(c, "downloads.showFinished"), "", &st.finished) {
			st.limits[0] = downloadsPage
		}
		items := t.value.GetItems()
		for _, i := range items {
			g.divider()
			ui.Column(c.Key(i.GetId())).Padding(12, 16).Gap(6).Children(func() { a.queueRow(c, i) })
		}
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(items) == 0:
			g.note(style.T(c, "downloads.queueEmpty"), false)
		case int32(len(items)) < t.value.GetTotal() && g.more(style.T(c, "activity.showMore")):
			st.limits[0] += downloadsPage
		}
	})
}

// queueRow builds a tracked download: its item, state, release, progress
// and messages, and removing and importing it.
func (a *App) queueRow(c *ui.Context, i *acquisitionv1.QueueItem) {
	st := &a.settingsState.admin.downloads
	colors := style.Colors(c)
	id, state := i.GetId(), i.GetState()
	downloading := state == acquisitionv1.DownloadState_DOWNLOAD_STATE_DOWNLOADING
	size := i.GetSize()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		a.itemLink(c, i.GetItemId(), i.GetItemName(), style.Type.Headline)
		pill(c, downloadStateName(c, state), state == acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORT_BLOCKED ||
			state == acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED || state == acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED_PENDING)
		if s := downloadStatusName(c, i.GetStatus()); s != "" {
			footnote(c, s, false)
		}
	})
	style.Text(ui.Text(c, i.GetTitle()), style.Type.Footnote).TextColor(style.Color(colors.TextPrimary)).Selectable()
	detail := []string{i.GetQuality(), strings.Join(i.GetLanguages(), ", ")}
	if size > 0 {
		detail = append(detail, sizeOf(size))
	}
	detail = append(detail, i.GetIndexerName())
	if i.HasRemainingTime() && downloading {
		detail = append(detail, style.T(c, "downloads.left", "time", lengthOf(c, i.GetRemainingTime().AsDuration())))
	}
	if i.HasGrabTime() {
		detail = append(detail, fromNow(c, i.GetGrabTime().AsTime()))
	}
	footnote(c, joinDetail(detail...), false)
	if size > 0 && downloading {
		done := max(0, min(1, 1-float64(i.GetRemainingSize())/float64(size)))
		ui.Box(c).Height(4).Radius(2).Background(style.Color(colors.Fill)).Children(func() {
			ui.Box(c).Height(4).Radius(2).WidthPercent(float32(done * 100)).Background(style.Color(colors.Accent))
		})
	}
	for k, m := range i.GetMessages() {
		style.Text(ui.Text(c.Key(k), m), style.Type.Footnote).TextColor(style.Color(colors.Danger)).Selectable()
	}
	finished := state == acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORTED || state == acquisitionv1.DownloadState_DOWNLOAD_STATE_FAILED ||
		state == acquisitionv1.DownloadState_DOWNLOAD_STATE_IGNORED
	if finished {
		return
	}
	importable := state == acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORT_BLOCKED || state == acquisitionv1.DownloadState_DOWNLOAD_STATE_IMPORT_PENDING
	toggle := func(panel string) {
		if st.panels == nil {
			st.panels = map[string]string{}
		}
		if st.panels[id] == panel {
			delete(st.panels, id)
		} else {
			st.panels[id] = panel
		}
	}
	ui.Row(c).Gap(8).Wrap().Padding(4, 0, 0, 0).Children(func() {
		if importable && smallButton(c, style.T(c, "downloads.manualImport"), false) {
			toggle("import")
		}
		if ui.ButtonBase(c).Height(28).Padding(0, 8).AlignItems(ui.Center).Label(style.T(c, "downloads.remove")).Children(func() {
			style.Text(ui.Text(c, style.T(c, "downloads.remove")), style.Type.Callout).FontWeight(600).TextColor(style.Color(colors.Danger))
		}).Clicked() {
			toggle("remove")
		}
	})
	switch st.panels[id] {
	case "remove":
		a.removePanel(c, id)
	case "import":
		a.importFiles(c, importTarget{id: id})
	}
}

// panel builds a raised box of a panel's content.
func panel(c *ui.Context, fn func()) {
	ui.Column(c).Padding(8, 12).Gap(4).Radius(8).Background(style.Color(style.Colors(c).SurfaceRaised)).Children(fn)
}

// switchLine builds a line of a panel's switch, which a click anywhere on
// the line toggles, and reports a change of *on.
func switchLine(c *ui.Context, label string, on *bool) bool {
	changed := false
	b := ui.ButtonBase(c.Key(label)).MinHeight(44).Gap(12).AlignItems(ui.Center).Label(label)
	b.Children(func() {
		style.Text(ui.Text(c, label), style.Type.Callout).TextColor(style.Color(style.Colors(c).TextPrimary)).Grow(1)
		changed = ui.Switch(c, on).Label(label).Changed()
	})
	if b.Clicked() && !changed {
		*on, changed = !*on, true
	}
	return changed
}

// removePanel builds how a download is removed: from the client or not,
// its release blocklisted or not, and searched for again or not.
func (a *App) removePanel(c *ui.Context, id string) {
	st := &a.settingsState.admin.downloads
	if st.removals == nil {
		st.removals = map[string]*removal{}
	}
	r := st.removals[id]
	if r == nil {
		r = &removal{fromClient: true}
		st.removals[id] = r
	}
	panel(c, func() {
		switchLine(c, style.T(c, "downloads.removeFromClient"), &r.fromClient)
		if switchLine(c, style.T(c, "downloads.blocklistRelease"), &r.blocklist) && !r.blocklist {
			r.search = false
		}
		if r.blocklist {
			switchLine(c, style.T(c, "downloads.searchAgain"), &r.search)
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Padding(4, 0, 0, 0).Children(func() {
			if linkButton(c, style.T(c, "action.cancel")).Clicked() {
				delete(st.panels, id)
			}
			if dangerButton(c, style.T(c, "downloads.remove"), st.busy == "remove"+id) {
				req := acquisitionv1.RemoveFromQueueRequest_builder{Id: &id, RemoveFromClient: &r.fromClient, Blocklist: &r.blocklist, Search: &r.search}.Build()
				a.downloadAction("remove"+id, id, func(ctx context.Context, d acquisitionv1connect.DownloadServiceClient) error {
					_, err := d.RemoveFromQueue(ctx, req)
					return err
				}, func() {
					delete(st.panels, id)
					delete(st.removals, id)
				})
			}
			if err := st.errs[id]; err != nil {
				footnote(c, errorMessage(c, err), true)
			}
		})
	})
}

// importFiles builds the files a manual import would take from a
// download or a folder, each imported on its own.
func (a *App) importFiles(c *ui.Context, target importTarget) {
	st := &a.settingsState.admin.downloads
	k := target.key()
	if st.modes == nil {
		st.modes = map[string]int{}
	}
	mode := st.modes[k]
	t := adminTask(a, taskIn(&st.files, k), "manual-import", func(ctx context.Context, s string) (*acquisitionv1.ListManualImportResponse, error) {
		req := acquisitionv1.ListManualImportRequest_builder{}
		if target.id != "" {
			req.Id = &target.id
		} else {
			req.Folder = &target.folder
		}
		return a.downloadService(s).ListManualImport(ctx, req.Build())
	}, k)
	panel(c, func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
			style.Text(ui.Text(c, style.T(c, "downloads.files")), style.Type.Callout).FontWeight(600).TextColor(style.Color(style.Colors(c).TextPrimary)).Grow(1)
			if choose(c, style.T(c, "downloads.mode"), &mode, []string{style.T(c, "downloads.modeAuto"), style.T(c, "downloads.modeMove"), style.T(c, "downloads.modeCopy")}) {
				st.modes[k] = mode
			}
		})
		switch {
		case !t.loaded:
			secondaryText(c, style.T(c, "state.loading"))
		case t.err != nil:
			footnote(c, errorMessage(c, t.err), true)
		case len(t.value.GetFiles()) == 0:
			secondaryText(c, style.T(c, "downloads.noFiles"))
		}
		for _, f := range t.value.GetFiles() {
			a.importFile(c, target, f, importModes[mode])
		}
	})
}

func (a *App) importFile(c *ui.Context, target importTarget, f *acquisitionv1.ManualImportFile, mode acquisitionv1.ImportMode) {
	st := &a.settingsState.admin.downloads
	colors := style.Colors(c)
	key := target.key() + "\x00" + f.GetPath()
	ui.Box(c.Key("divider" + f.GetPath())).Height(1).Background(style.Color(colors.Border))
	ui.Row(c.Key(f.GetPath())).Gap(12).AlignItems(ui.Center).Padding(4, 0).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			name := f.GetRelativePath()
			if name == "" {
				name = f.GetPath()
			}
			style.Text(ui.Text(c, name), style.Type.Footnote).TextColor(style.Color(colors.TextPrimary)).Selectable()
			footnote(c, joinDetail(sizeOf(f.GetSize()), f.GetQuality(), strings.Join(f.GetLanguages(), ", "), f.GetReleaseGroup()), false)
			for k, r := range f.GetRejections() {
				style.Text(ui.Text(c.Key(k), r), style.Type.Footnote).TextColor(style.Color(colors.Danger)).Selectable()
			}
			if err := st.errs[key]; err != nil {
				footnote(c, errorMessage(c, err), true)
			}
		})
		label := style.T(c, "downloads.importFile")
		if st.imported[key] {
			label = style.T(c, "downloads.importedFile")
		}
		if smallButton(c, label, f.GetMovieId() == "" || st.busy == key || st.imported[key]) {
			req := acquisitionv1.ManualImportRequest_builder{Path: new(f.GetPath()), Mode: &mode}
			if target.id != "" {
				req.Id = &target.id
			} else {
				req.Folder = &target.folder
			}
			a.downloadAction(key, key, func(ctx context.Context, d acquisitionv1connect.DownloadServiceClient) error {
				_, err := d.ManualImport(ctx, req.Build())
				return err
			}, func() {
				if st.imported == nil {
					st.imported = map[string]bool{}
				}
				st.imported[key] = true
			})
		}
	})
}

func (a *App) downloadHistory(c *ui.Context) {
	st := &a.settingsState.admin.downloads
	labels := []string{style.T(c, "downloads.allEvents")}
	for _, x := range historyTypes {
		labels = append(labels, historyName(c, x))
	}
	if choose(c, style.T(c, "downloads.eventType"), &st.event, labels) {
		st.limits[1] = downloadsPage
	}
	var types []acquisitionv1.HistoryEventType
	if st.event > 0 {
		types = historyTypes[st.event-1 : st.event]
	}
	limit := int32(st.limits[1])
	t := adminTask(a, &st.history, "history", func(ctx context.Context, s string) (*acquisitionv1.ListHistoryResponse, error) {
		return a.downloadService(s).ListHistory(ctx, acquisitionv1.ListHistoryRequest_builder{Types: types, Limit: &limit}.Build())
	}, st.event, limit)
	footer := ""
	if t.loaded && t.err == nil {
		footer = style.T(c, "downloads.eventCount", "count", t.value.GetTotal())
	}
	group(c, "history", "", footer, func(g *settingsGroup) {
		events := t.value.GetEvents()
		for _, e := range events {
			g.divider()
			ui.Column(c.Key(e.GetId())).Padding(12, 16).Gap(4).Children(func() { a.historyRow(c, e) })
		}
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(events) == 0:
			g.note(style.T(c, "downloads.historyEmpty"), false)
		case int32(len(events)) < t.value.GetTotal() && g.more(style.T(c, "activity.showMore")):
			st.limits[1] += downloadsPage
		}
	})
}

// historyRow builds an event: what happened, of which release and item,
// its details, and marking a grab failed.
func (a *App) historyRow(c *ui.Context, e *acquisitionv1.HistoryEvent) {
	st := &a.settingsState.admin.downloads
	colors := style.Colors(c)
	id := e.GetId()
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
		pill(c, historyName(c, e.GetType()), e.GetType() == acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_DOWNLOAD_FAILED)
		var when string
		if e.HasTime() {
			when = fromNow(c, e.GetTime().AsTime())
		}
		footnote(c, joinDetail(e.GetQuality(), strings.Join(e.GetLanguages(), ", "), when), false)
	})
	style.Text(ui.Text(c, e.GetSourceTitle()), style.Type.Footnote).TextColor(style.Color(colors.TextPrimary)).Selectable()
	if e.GetItemId() != "" {
		a.itemLink(c, e.GetItemId(), style.T(c, "downloads.item"), style.Type.Footnote)
	}
	if data := e.GetData(); len(data) > 0 {
		var parts []string
		for _, k := range slices.Sorted(maps.Keys(data)) {
			parts = append(parts, k+"="+data[k])
		}
		style.Text(ui.Text(c, strings.Join(parts, "   ")), style.Type.Caption).Font("SF Mono, Menlo, monospace").TextColor(style.Color(colors.TextSecondary)).Selectable()
	}
	if e.GetType() != acquisitionv1.HistoryEventType_HISTORY_EVENT_TYPE_GRABBED {
		return
	}
	ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Padding(4, 0, 0, 0).Children(func() {
		if st.confirm == id {
			style.Text(ui.Text(c, style.T(c, "downloads.markFailedTitle")), style.Type.Callout).TextColor(style.Color(colors.TextPrimary))
			if linkButton(c, style.T(c, "action.cancel")).Clicked() {
				st.confirm = ""
			}
			if dangerButton(c, style.T(c, "downloads.markFailed"), st.busy == "fail"+id) {
				a.downloadAction("fail"+id, id, func(ctx context.Context, d acquisitionv1connect.DownloadServiceClient) error {
					_, err := d.MarkFailed(ctx, acquisitionv1.MarkFailedRequest_builder{HistoryEventId: &id}.Build())
					return err
				}, func() { st.confirm = "" })
			}
		} else if ui.ButtonBase(c).Height(28).AlignItems(ui.Center).Label(style.T(c, "downloads.markFailed")).Children(func() {
			style.Text(ui.Text(c, style.T(c, "downloads.markFailed")), style.Type.Callout).FontWeight(600).TextColor(style.Color(colors.Danger))
		}).Clicked() {
			st.confirm = id
		}
		if err := st.errs[id]; err != nil {
			footnote(c, errorMessage(c, err), true)
		}
	})
}

func (a *App) downloadBlocklist(c *ui.Context) {
	st := &a.settingsState.admin.downloads
	limit := int32(st.limits[2])
	t := adminTask(a, &st.blocklist, "blocklist", func(ctx context.Context, s string) (*acquisitionv1.ListBlocklistResponse, error) {
		return a.downloadService(s).ListBlocklist(ctx, acquisitionv1.ListBlocklistRequest_builder{Limit: &limit}.Build())
	}, limit)
	group(c, "blocklist", "", style.T(c, "downloads.blocklistFooter"), func(g *settingsGroup) {
		entries := t.value.GetEntries()
		for _, e := range entries {
			g.divider()
			a.blocklistRow(c, e)
		}
		switch {
		case t.err != nil:
			g.note(errorMessage(c, t.err), true)
		case t.loaded && len(entries) == 0:
			g.note(style.T(c, "downloads.blocklistEmpty"), false)
		case int32(len(entries)) < t.value.GetTotal() && g.more(style.T(c, "activity.showMore")):
			st.limits[2] += downloadsPage
		}
	})
}

func (a *App) blocklistRow(c *ui.Context, e *acquisitionv1.BlocklistEntry) {
	st := &a.settingsState.admin.downloads
	colors := style.Colors(c)
	id := e.GetId()
	ui.Row(c.Key(id)).Padding(12, 16).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			style.Text(ui.Text(c, e.GetSourceTitle()), style.Type.Footnote).TextColor(style.Color(colors.TextPrimary)).Selectable()
			detail := []string{e.GetQuality(), strings.Join(e.GetLanguages(), ", ")}
			if e.GetSize() > 0 {
				detail = append(detail, sizeOf(e.GetSize()))
			}
			detail = append(detail, e.GetIndexerName())
			if e.HasTime() {
				detail = append(detail, fromNow(c, e.GetTime().AsTime()))
			}
			footnote(c, joinDetail(detail...), false)
			if e.GetMessage() != "" {
				footnote(c, e.GetMessage(), false)
			}
			if e.GetItemId() != "" {
				a.itemLink(c, e.GetItemId(), style.T(c, "downloads.item"), style.Type.Footnote)
			}
			if err := st.errs[id]; err != nil {
				footnote(c, errorMessage(c, err), true)
			}
		})
		if dangerButton(c, style.T(c, "downloads.unblock"), st.busy == "unblock"+id) {
			a.downloadAction("unblock"+id, id, func(ctx context.Context, d acquisitionv1connect.DownloadServiceClient) error {
				_, err := d.DeleteBlocklist(ctx, acquisitionv1.DeleteBlocklistRequest_builder{Ids: []string{id}}.Build())
				return err
			}, nil)
		}
	})
}

// folderImport builds a folder's files, imported for the wanted movies
// their names match.
func (a *App) folderImport(c *ui.Context) {
	st := &a.settingsState.admin.downloads
	group(c, "import", "", style.T(c, "downloads.importFooter"), func(g *settingsGroup) {
		g.divider()
		ui.Row(c.Key("folder")).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
			label := style.T(c, "downloads.folder")
			in := ui.TextInput(c, &st.path).Grow(1).MinWidth(224).Font("SF Mono, Menlo, monospace").Placeholder(label).Label(label)
			empty := strings.TrimSpace(st.path) == ""
			if (ui.PrimaryButton(c, style.T(c, "downloads.scan")).Disabled(empty).Clicked() || in.Submitted()) && !empty {
				st.folder = strings.TrimSpace(st.path)
			}
		})
		if st.folder != "" {
			ui.Column(c.Key("files")).Padding(0, 16, 12, 16).Children(func() {
				a.importFiles(c.Key(st.folder), importTarget{folder: st.folder})
			})
		}
	})
}

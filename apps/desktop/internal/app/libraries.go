package app

import (
	"context"
	"maps"
	"net/url"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"google.golang.org/protobuf/types/known/durationpb"

	capiv1 "github.com/mavioai/mavio/libs/client-rust/capi/gen/mavio/capi/v1"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1/storagev1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// librariesState is the state of the settings' Libraries: the library
// edited and the folders browsed for it.
type librariesState struct {
	form    libraryForm
	dirs    map[string]*task[*systemv1.ListDirectoryResponse]
	folders map[string]*task[[]string]
	status  task[*storagev1.GetStatusResponse]
	remotes task[[]*storagev1.Remote]
	busy    string // the change being made
}

// libraryDraft is a library as edited: the fields of LibrarySpec the
// page shows.
type libraryDraft struct {
	name                                                 string
	kind                                                 libraryv1.LibraryKind
	paths                                                []string
	classes                                              map[string]libraryv1.StorageClass
	scan                                                 time.Duration
	language, country                                    string
	saveLocal, autoCollections, trickplay, chapterImages bool
	loudness, lyrics, missing, unaired, specials         bool
	graceDays                                            float64
}

// libraryForm is the page of a library: the draft of it as loaded and as
// edited, and the folder picker's place.
type libraryForm struct {
	id           string // the library's, or "new"
	draft, saved libraryDraft
	picking      bool
	path, typed  string
	err          error
	notice       string
	confirming   bool
}

// libraryKindOrder is the order the kinds are offered in.
var libraryKindOrder = []libraryv1.LibraryKind{
	libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, libraryv1.LibraryKind_LIBRARY_KIND_MUSIC,
	libraryv1.LibraryKind_LIBRARY_KIND_MUSIC_VIDEOS, libraryv1.LibraryKind_LIBRARY_KIND_HOME_VIDEOS, libraryv1.LibraryKind_LIBRARY_KIND_BOOKS,
	libraryv1.LibraryKind_LIBRARY_KIND_PHOTOS, libraryv1.LibraryKind_LIBRARY_KIND_MIXED,
}

// libraryKindOf returns how a kind shows, mixed content's for a kind not
// offered.
func libraryKindOf(k libraryv1.LibraryKind) struct {
	title, icon string
	color       ui.Color
} {
	if x, ok := libraryKinds[k]; ok {
		return x
	}
	return libraryKinds[libraryv1.LibraryKind_LIBRARY_KIND_MIXED]
}

// kindsOf tells the kinds a library's options apply to.
func kindsOf(kinds ...libraryv1.LibraryKind) func(libraryv1.LibraryKind) bool {
	return func(k libraryv1.LibraryKind) bool { return slices.Contains(kinds, k) }
}

var (
	videoKinds = kindsOf(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, libraryv1.LibraryKind_LIBRARY_KIND_SHOWS,
		libraryv1.LibraryKind_LIBRARY_KIND_MUSIC_VIDEOS, libraryv1.LibraryKind_LIBRARY_KIND_HOME_VIDEOS, libraryv1.LibraryKind_LIBRARY_KIND_MIXED)
	musicKinds = kindsOf(libraryv1.LibraryKind_LIBRARY_KIND_MUSIC, libraryv1.LibraryKind_LIBRARY_KIND_MIXED)
	movieKinds = kindsOf(libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, libraryv1.LibraryKind_LIBRARY_KIND_MIXED)
	showKinds  = kindsOf(libraryv1.LibraryKind_LIBRARY_KIND_SHOWS, libraryv1.LibraryKind_LIBRARY_KIND_MIXED)
)

// scanIntervals are the intervals of automatic scans offered; 0 for none.
var scanIntervals = []time.Duration{0, time.Hour, 6 * time.Hour, 12 * time.Hour, 24 * time.Hour, 7 * 24 * time.Hour}

func scanIntervalTitle(c *ui.Context, d time.Duration) string {
	switch d {
	case 0:
		return style.T(c, "libraries.scanNever")
	case time.Hour:
		return style.T(c, "libraries.scanHourly")
	case 24 * time.Hour:
		return style.T(c, "libraries.scanDaily")
	case 7 * 24 * time.Hour:
		return style.T(c, "libraries.scanWeekly")
	}
	return style.T(c, "libraries.scanEveryHours", "hours", int(d.Hours()))
}

// storageClasses are the storage classes a folder is given, by title.
var storageClasses = []struct {
	class libraryv1.StorageClass
	title string
}{
	{libraryv1.StorageClass_STORAGE_CLASS_LOCAL, "libraries.storageLocal"},
	{libraryv1.StorageClass_STORAGE_CLASS_NETWORK, "libraries.storageNetwork"},
	{libraryv1.StorageClass_STORAGE_CLASS_CLOUD, "libraries.storageCloud"},
}

func (a *App) storageService(id string) storagev1connect.StorageServiceClient {
	return storagev1connect.NewStorageServiceClient(a.connectHTTP(), serverURL(id))
}

func librariesPath(library string) string {
	if library == "" {
		return "/settings/libraries"
	}
	return "/settings/libraries?library=" + url.QueryEscape(library)
}

func libraryDraftOf(l *libraryv1.Library) libraryDraft {
	if l == nil {
		return libraryDraft{kind: libraryv1.LibraryKind_LIBRARY_KIND_MOVIES, paths: []string{}, classes: map[string]libraryv1.StorageClass{}, scan: 12 * time.Hour}
	}
	v := l.GetVirtualEpisodes()
	d := libraryDraft{
		name: l.GetName(), kind: l.GetKind(), paths: append([]string{}, l.GetPaths()...), classes: map[string]libraryv1.StorageClass{},
		language: l.GetPreferredLanguage(), country: l.GetMetadataCountry(),
		saveLocal: l.GetSaveLocalMetadata(), autoCollections: l.GetAutoCollections(), trickplay: l.GetExtractTrickplay(),
		chapterImages: l.GetExtractChapterImages(), loudness: l.GetAnalyzeLoudness(), lyrics: l.GetDownloadLyrics(),
		missing: v.GetMissing(), unaired: v.GetUnaired(), specials: v.GetSpecials(), graceDays: float64(v.GetGraceDays()),
	}
	maps.Copy(d.classes, l.GetStorageClasses())
	if l.HasScanInterval() {
		d.scan = l.GetScanInterval().AsDuration()
	}
	return d
}

// spec returns the spec of a draft, keeping what the page does not show
// of the library l.
func (d libraryDraft) spec(l *libraryv1.Library) *libraryv1.LibrarySpec {
	classes := map[string]libraryv1.StorageClass{}
	for p, class := range d.classes {
		if slices.Contains(d.paths, p) && class != libraryv1.StorageClass_STORAGE_CLASS_UNSPECIFIED {
			classes[p] = class
		}
	}
	b := libraryv1.LibrarySpec_builder{
		Name: new(strings.TrimSpace(d.name)), Kind: new(d.kind), Paths: d.paths,
		PreferredLanguage: new(d.language), MetadataCountry: new(d.country),
		SaveLocalMetadata: new(d.saveLocal), AutoCollections: new(d.autoCollections),
		ExtractTrickplay: new(d.trickplay), ExtractChapterImages: new(d.chapterImages),
		AnalyzeLoudness: new(d.loudness), DownloadLyrics: new(d.lyrics),
		ProviderOrder: l.GetProviderOrder(), Hubs: l.GetHubs(), StorageClasses: classes,
		VirtualEpisodes: libraryv1.VirtualEpisodes_builder{
			Missing: new(d.missing), Unaired: new(d.unaired), Specials: new(d.specials), GraceDays: new(int32(d.graceDays)),
		}.Build(),
	}
	if d.scan > 0 {
		b.ScanInterval = durationpb.New(d.scan)
	}
	return b.Build()
}

func (a *App) librariesSection(c *ui.Context, r *ui.Route) {
	if id := r.Query("library"); id != "" {
		a.libraryPage(c, id)
		return
	}
	a.libraryList(c)
}

func (a *App) libraryList(c *ui.Context) {
	st := &a.settingsState.admin
	sectionHeader(c, style.T(c, "settings.libraries"), style.T(c, "libraries.message"))
	t := a.adminLibraries()
	if taskStatus(c, t, func() { st.libs.key = "" }, nil) {
		return
	}
	var libs []*libraryv1.Library
	for _, l := range t.value {
		if accessible(l) {
			libs = append(libs, l)
		}
	}
	slices.SortFunc(libs, func(x, y *libraryv1.Library) int {
		return strings.Compare(strings.ToLower(x.GetName()), strings.ToLower(y.GetName()))
	})
	colors := style.Colors(c)
	group(c, "libraries", "", "", func(g *settingsGroup) {
		if len(libs) == 0 {
			g.note(style.T(c, "libraries.none"), false)
		}
		for _, l := range libs {
			k := libraryKindOf(l.GetKind())
			detail := style.T(c, k.title) + " · " + style.T(c, "libraries.folderCount", "count", len(l.GetPaths()))
			id := l.GetId()
			progress := a.scans[a.chosenServer().GetId()][id]
			switch n, ok := st.notices[id]; {
			case progress != nil:
				detail = scanDetail(c, progress)
			case st.scanned[id]:
				detail = style.T(c, "libraries.scanDone")
			case ok:
				detail = n
			}
			g.divider()
			ui.Row(c.Key(id)).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
				b := ui.ButtonBase(c).Grow(1).MinWidth(0).Gap(12).AlignItems(ui.Center).Justify(ui.Start).Label(l.GetName())
				b.Children(func() {
					sectionTile(c, k.icon, k.color)
					ui.Column(c).Grow(1).MinWidth(0).Children(func() {
						t := style.Text(ui.Text(c, l.GetName()), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary))
						if b.Hovered() {
							t.Underline()
						}
						style.Text(ui.Text(c, detail), style.Type(c).Footnote).TextColor(style.Color(colors.TextSecondary))
						if progress != nil {
							ui.Box(c).Height(4).Radius(2).Margin(4, 0, 0, 0).Background(style.Color(colors.Fill)).Role(ui.RoleProgress).Children(func() {
								ui.Box(c).Height(4).Radius(2).WidthPercent(float32(min(max(progress.GetFraction(), 0), 1) * 100)).Background(style.Color(colors.Accent))
							})
						}
					})
				})
				if b.Clicked() {
					a.router.Push(librariesPath(id))
				}
				a.scanButton(c, id, func(notice string, err error) { a.notice(c, id, notice, err) })
			})
		}
		if g.addRow(style.T(c, "libraries.add")) {
			a.router.Push(librariesPath("new"))
		}
	})
}

// scanButton builds the button scanning a library; done takes the
// notice of the scan, or its error.
func (a *App) scanButton(c *ui.Context, id string, done func(notice string, err error)) {
	st := &a.settingsState.admin
	server := a.chosenServer().GetId()
	label := style.T(c, "libraries.scan")
	if st.running == id {
		label = "…"
	}
	if smallButton(c.Key("scan"+id), label, st.running != "") {
		delete(st.scanned, id)
		a.startAction(id, func(ctx context.Context) (string, error) {
			res, err := a.libraryService(server).ScanLibrary(ctx, libraryv1.ScanLibraryRequest_builder{Id: &id}.Build())
			if res.GetEnqueued() {
				return "libraries.scanQueued", err
			}
			return "libraries.scanPending", err
		}, done)
	}
}

// scanDetail tells what a library's scan does, and how far it has come.
func scanDetail(c *ui.Context, p *capiv1.ScanProgress) string {
	if p.GetReadingFolders() {
		return style.T(c, "libraries.scanFolders", "done", p.GetFoldersDone(), "total", p.GetFoldersTotal())
	}
	return style.T(c, "libraries.scanItems", "done", p.GetItemsDone(), "total", p.GetItemsTotal())
}

// listedScans takes the scans of a server's libraries as listed, which
// the server's events then carry on.
func (a *App) listedScans(server string, libs []*libraryv1.Library) {
	scans := map[string]*capiv1.ScanProgress{}
	for _, l := range libs {
		if !l.HasScanProgress() {
			continue
		}
		p := l.GetScanProgress()
		scans[l.GetId()] = capiv1.ScanProgress_builder{
			ReadingFolders: p.GetPhase() == libraryv1.ScanPhase_SCAN_PHASE_FOLDERS,
			FoldersDone:    p.GetFoldersDone(),
			FoldersTotal:   p.GetFoldersTotal(),
			ItemsDone:      p.GetItemsDone(),
			ItemsTotal:     p.GetItemsTotal(),
			Fraction:       p.GetFraction(),
		}.Build()
	}
	if a.scans == nil {
		a.scans = map[string]map[string]*capiv1.ScanProgress{}
	}
	a.scans[server] = scans
}

// scanChanged takes how far a library's scan has come, or that it ended.
func (a *App) scanChanged(server string, e *capiv1.ScanProgressChanged) {
	if a.scans == nil {
		a.scans = map[string]map[string]*capiv1.ScanProgress{}
	}
	if a.scans[server] == nil {
		a.scans[server] = map[string]*capiv1.ScanProgress{}
	}
	st := &a.settingsState.admin
	if e.HasProgress() {
		a.scans[server][e.GetLibraryId()] = e.GetProgress()
		return
	}
	delete(a.scans[server], e.GetLibraryId())
	if st.scanned == nil {
		st.scanned = map[string]bool{}
	}
	st.scanned[e.GetLibraryId()] = true
}

// libraryPage builds a library's page, or that of a new one when id is
// "new".
func (a *App) libraryPage(c *ui.Context, id string) {
	st := &a.settingsState.admin
	a.backTo(c, style.T(c, "settings.libraries"), librariesPath(""))
	t := a.adminLibraries()
	if taskStatus(c, t, func() { st.libs.key = "" }, nil) {
		return
	}
	var library *libraryv1.Library
	if id != "new" {
		i := slices.IndexFunc(t.value, func(l *libraryv1.Library) bool { return l.GetId() == id })
		if i < 0 {
			secondaryText(c, style.T(c, "libraries.notFound"))
			return
		}
		library = t.value[i]
	}
	a.libraryForm(c, id, library)
}

// trailingSlashes are the slashes ending a path, but its first.
var trailingSlashes = regexp.MustCompile(`(.)/+$`)

func (a *App) libraryForm(c *ui.Context, id string, library *libraryv1.Library) {
	ls := &a.settingsState.admin.libraries
	f := &ls.form
	if f.id != id {
		*f = libraryForm{id: id, draft: libraryDraftOf(library), saved: libraryDraftOf(library), picking: library == nil}
	}
	d := &f.draft
	server := a.chosenServer().GetId()
	colors := style.Colors(c)
	k := libraryKindOf(d.kind)
	title := style.T(c, "libraries.add")
	if library != nil {
		title = library.GetName()
	}
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(40, 40).Shrink(0).Radius(10).Background(k.color).Center().Children(func() {
			ui.Icon(c, style.Icon(k.icon, true)).Size(24, 24).TextColor(ui.RGB(0xFF, 0xFF, 0xFF))
		})
		style.Text(ui.Text(c, title), style.Type(c).Title1).TextColor(style.Color(colors.TextPrimary))
	})
	group(c, "library", "", "", func(g *settingsGroup) {
		g.row(style.T(c, "libraries.name"), "", nil, func() {
			ui.TextInput(c, &d.name).Width(288).Label(style.T(c, "libraries.name"))
		})
		g.row(style.T(c, "libraries.kind"), "", nil, func() {
			labels := make([]string, len(libraryKindOrder))
			for i, x := range libraryKindOrder {
				labels[i] = style.T(c, libraryKinds[x].title)
			}
			i := max(slices.Index(libraryKindOrder, d.kind), 0)
			if choose(c, style.T(c, "libraries.kind"), &i, labels) {
				d.kind = libraryKindOrder[i]
			}
		})
	})
	a.foldersGroup(c, server, library)
	group(c, "metadata", style.T(c, "libraries.metadata"), "", func(g *settingsGroup) {
		g.row(style.T(c, "libraries.language"), "", nil, func() {
			codes, labels := []string{""}, []string{style.T(c, "libraries.serverDefault")}
			for _, l := range a.languages(server) {
				if l.GetCode() != "" && !slices.Contains(codes, l.GetCode()) {
					codes, labels = append(codes, l.GetCode()), append(labels, cmpOr(languageName(c, l.GetCode()), l.GetName()))
				}
			}
			if !slices.Contains(codes, d.language) {
				codes, labels = append(codes, d.language), append(labels, languageName(c, d.language))
			}
			i := slices.Index(codes, d.language)
			if choose(c, style.T(c, "libraries.language"), &i, labels) {
				d.language = codes[i]
			}
		})
		g.row(style.T(c, "libraries.country"), "", nil, func() {
			codes, labels := []string{""}, []string{style.T(c, "libraries.serverDefault")}
			for _, x := range a.countries(server) {
				if x.GetCode() != "" && !slices.Contains(codes, x.GetCode()) {
					codes, labels = append(codes, x.GetCode()), append(labels, regionName(c, x.GetCode()))
				}
			}
			if !slices.Contains(codes, d.country) {
				codes, labels = append(codes, d.country), append(labels, regionName(c, d.country))
			}
			i := slices.Index(codes, d.country)
			if choose(c, style.T(c, "libraries.country"), &i, labels) {
				d.country = codes[i]
			}
		})
		g.switchRow(style.T(c, "libraries.saveLocalMetadata"), style.T(c, "libraries.saveLocalMetadataDetail"), &d.saveLocal)
		if movieKinds(d.kind) {
			g.switchRow(style.T(c, "libraries.autoCollections"), style.T(c, "libraries.autoCollectionsDetail"), &d.autoCollections)
		}
		if musicKinds(d.kind) {
			g.switchRow(style.T(c, "libraries.downloadLyrics"), "", &d.lyrics)
		}
	})
	group(c, "scanning", style.T(c, "libraries.scanning"), "", func(g *settingsGroup) {
		g.row(style.T(c, "libraries.scanInterval"), "", nil, func() {
			values := slices.Clone(scanIntervals)
			if !slices.Contains(values, d.scan) {
				values = append(values, d.scan)
			}
			labels := make([]string, len(values))
			for i, v := range values {
				labels[i] = scanIntervalTitle(c, v)
			}
			i := slices.Index(values, d.scan)
			if choose(c, style.T(c, "libraries.scanInterval"), &i, labels) {
				d.scan = values[i]
			}
		})
		if videoKinds(d.kind) {
			g.switchRow(style.T(c, "libraries.trickplay"), style.T(c, "libraries.trickplayDetail"), &d.trickplay)
			g.switchRow(style.T(c, "libraries.chapterImages"), "", &d.chapterImages)
		}
		if musicKinds(d.kind) {
			g.switchRow(style.T(c, "libraries.loudness"), style.T(c, "libraries.loudnessDetail"), &d.loudness)
		}
	})
	if showKinds(d.kind) {
		group(c, "missing", style.T(c, "libraries.missingEpisodes"), style.T(c, "libraries.missingEpisodesFooter"), func(g *settingsGroup) {
			g.switchRow(style.T(c, "libraries.missingAired"), "", &d.missing)
			g.switchRow(style.T(c, "libraries.missingUnaired"), "", &d.unaired)
			g.switchRow(style.T(c, "libraries.missingSpecials"), "", &d.specials)
			if !d.missing {
				g.row(style.T(c, "libraries.graceDays"), style.T(c, "libraries.graceDaysDetail"), nil, func() {
					ui.NumberInput(c, &d.graceDays, 0, 3650, 1).Label(style.T(c, "libraries.graceDays"))
				})
			}
		})
	}
	changed := !reflect.DeepEqual(*d, f.saved)
	valid := strings.TrimSpace(d.name) != "" && len(d.paths) > 0
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		label := style.T(c, "action.save")
		if library == nil {
			label = style.T(c, "libraries.create")
		}
		if ls.busy == "save" {
			label = "…"
		}
		if ui.PrimaryButton(c, label).Disabled(!changed || !valid || ls.busy != "").Clicked() {
			a.saveLibrary(library)
		}
		if library != nil {
			a.scanButton(c, library.GetId(), func(notice string, err error) {
				f.err = err
				if err == nil {
					f.notice = notice
				}
			})
		}
		if f.notice != "" && !changed {
			secondaryText(c, style.T(c, f.notice))
		}
		if f.err != nil {
			dangerText(c, errorMessage(c, f.err))
		}
	})
	if library == nil {
		return
	}
	ui.Row(c).Children(func() {
		if dangerButton(c, style.T(c, "action.delete"), ls.busy != "") {
			f.confirming = true
		}
	})
	title = style.T(c, "libraries.deleteTitle", "name", library.GetName())
	if ui.AlertDialog(c, &f.confirming, title, style.T(c, "libraries.deleteMessage"), style.T(c, "action.cancel"), style.T(c, "action.delete")) == 1 {
		id := library.GetId()
		a.libraryAction("delete", func(ctx context.Context) error {
			_, err := a.libraryService(server).DeleteLibrary(ctx, libraryv1.DeleteLibraryRequest_builder{Id: &id}.Build())
			return err
		}, func(err error) {
			f.err = err
			if err == nil {
				a.replace(librariesPath(""))
			}
		})
	}
}

// saveLibrary adds the library drafted, or saves the changes of library.
func (a *App) saveLibrary(library *libraryv1.Library) {
	f := &a.settingsState.admin.libraries.form
	server := a.chosenServer().GetId()
	draft := f.draft
	spec := draft.spec(library)
	f.err = nil
	if library == nil {
		var created string
		a.libraryAction("save", func(ctx context.Context) error {
			res, err := a.libraryService(server).CreateLibrary(ctx, libraryv1.CreateLibraryRequest_builder{Spec: spec}.Build())
			created = res.GetLibrary().GetId()
			return err
		}, func(err error) {
			f.err = err
			if err == nil {
				f.id = created
				f.saved = draft
				a.replace(librariesPath(created))
			}
		})
		return
	}
	id := library.GetId()
	a.libraryAction("save", func(ctx context.Context) error {
		_, err := a.libraryService(server).UpdateLibrary(ctx, libraryv1.UpdateLibraryRequest_builder{Id: &id, Spec: spec}.Build())
		return err
	}, func(err error) {
		f.err = err
		if err == nil {
			f.saved, f.notice = draft, "libraries.saved"
		}
	})
}

// libraryAction runs a change of the libraries in the background, keyed
// what; done takes its error on the main thread.
func (a *App) libraryAction(what string, fn func(ctx context.Context) error, done func(err error)) {
	ls := &a.settingsState.admin.libraries
	ls.busy = what
	go func() {
		err := fn(a.ctx)
		a.opts.Update(func() {
			ls.busy = ""
			if err == nil {
				a.settingsState.admin.reload++
			}
			done(err)
		})
	}()
}

// foldersGroup builds a library's folders, with their storage classes,
// and adding folders typed or chosen from the server's.
func (a *App) foldersGroup(c *ui.Context, server string, library *libraryv1.Library) {
	f := &a.settingsState.admin.libraries.form
	d := &f.draft
	colors := style.Colors(c)
	addPath := func(p string) {
		p = trailingSlashes.ReplaceAllString(strings.TrimSpace(p), "$1")
		if p != "" && !slices.Contains(d.paths, p) {
			d.paths = append(slices.Clone(d.paths), p)
		}
	}
	group(c, "folders", style.T(c, "libraries.folders"), style.T(c, "libraries.foldersFooter"), func(g *settingsGroup) {
		for _, p := range d.paths {
			g.divider()
			ui.Row(c.Key(p)).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
				style.Text(ui.Text(c, p), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).Grow(1).MinWidth(0).Selectable()
				auto := style.T(c, "libraries.storageAuto")
				for _, s := range storageClasses {
					if s.class == library.GetEffectiveStorageClasses()[p] {
						auto = style.T(c, "libraries.storageAutoIs", "storage", style.T(c, s.title))
					}
				}
				labels := []string{auto}
				values := []libraryv1.StorageClass{libraryv1.StorageClass_STORAGE_CLASS_UNSPECIFIED}
				for _, s := range storageClasses {
					labels, values = append(labels, style.T(c, s.title)), append(values, s.class)
				}
				i := max(slices.Index(values, d.classes[p]), 0)
				if choose(c, style.T(c, "libraries.storageClass"), &i, labels) {
					classes := maps.Clone(d.classes)
					classes[p] = values[i]
					d.classes = classes
				}
				if iconButton(c, "close", style.T(c, "libraries.removeFolder")).Size(32, 32).Clicked() {
					d.paths = slices.DeleteFunc(slices.Clone(d.paths), func(x string) bool { return x == p })
				}
			})
		}
		g.divider()
		ui.Row(c.Key("add")).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
			in := ui.TextInput(c, &f.typed).Grow(1).MinWidth(224).Placeholder(style.T(c, "libraries.folderPath")).Label(style.T(c, "libraries.folderPath"))
			if smallButton(c, style.T(c, "libraries.addFolder"), strings.TrimSpace(f.typed) == "") || in.Submitted() {
				addPath(f.typed)
				f.typed = ""
			}
			b := ui.ButtonBase(c).Height(28).Padding(0, 10).Radius(8).Gap(6).AlignItems(ui.Center).Label(style.T(c, "libraries.browse"))
			if f.picking || b.Hovered() {
				b.Background(style.Color(colors.Fill))
			}
			b.Children(func() {
				ui.Icon(c, style.Icon("folder", false)).Size(18, 18).TextColor(style.Color(colors.AccentText))
				style.Text(ui.Text(c, style.T(c, "libraries.browse")), style.Type(c).Callout).FontWeight(600).TextColor(style.Color(colors.AccentText))
			})
			if b.Clicked() {
				f.picking = !f.picking
			}
		})
		if f.picking {
			g.divider()
			a.folderPicker(c, server, d.paths, addPath)
		}
	})
}

// folderPicker builds the folders of the server's file system and of its
// rclone remotes, to walk and choose from.
func (a *App) folderPicker(c *ui.Context, server string, chosen []string, choose func(path string)) {
	ls := &a.settingsState.admin.libraries
	f := &ls.form
	colors := style.Colors(c)
	path := f.path
	type folder struct{ path, name string }
	var (
		folders []folder
		here    = path
		parent  string
		loaded  bool
		err     error
	)
	if strings.HasPrefix(path, "rclone:") {
		t := load(a, taskIn(&ls.folders, path), key("folders", server, path, a.generation), func(ctx context.Context) ([]string, error) {
			res, err := a.storageService(server).ListFolders(ctx, storagev1.ListFoldersRequest_builder{Path: &path}.Build())
			return res.GetFolders(), err
		})
		loaded, err = t.loaded, t.err
		for _, p := range t.value {
			sep := ":"
			if strings.Contains(p, "/") {
				sep = "/"
			}
			folders = append(folders, folder{p, p[strings.LastIndex(p, sep)+1:]})
		}
		switch {
		case regexp.MustCompile(`^rclone:[^:]*:$`).MatchString(path):
			parent = ""
		case strings.Contains(path, "/"):
			parent = path[:strings.LastIndex(path, "/")]
		default:
			parent = path[:strings.Index(path[7:], ":")+8]
		}
	} else {
		t := load(a, taskIn(&ls.dirs, path), key("dir", server, path, a.generation), func(ctx context.Context) (*systemv1.ListDirectoryResponse, error) {
			return a.systemService(server).ListDirectory(ctx, systemv1.ListDirectoryRequest_builder{Path: &path}.Build())
		})
		loaded, err = t.loaded, t.err
		for _, e := range t.value.GetEntries() {
			if e.GetIsDir() {
				folders = append(folders, folder{e.GetPath(), e.GetName()})
			}
		}
		if t.value != nil {
			here, parent = t.value.GetPath(), t.value.GetParent()
		}
		if path == "" {
			status := load(a, &ls.status, key("storage", server, a.generation), func(ctx context.Context) (*storagev1.GetStatusResponse, error) {
				return a.storageService(server).GetStatus(ctx, &storagev1.GetStatusRequest{})
			})
			if status.value.GetAvailable() {
				remotes := load(a, &ls.remotes, key("remotes", server, a.generation), func(ctx context.Context) ([]*storagev1.Remote, error) {
					res, err := a.storageService(server).ListRemotes(ctx, &storagev1.ListRemotesRequest{})
					return res.GetRemotes(), err
				})
				for _, r := range remotes.value {
					p := "rclone:" + r.GetName() + ":"
					folders = append(folders, folder{p, p})
				}
			}
		}
	}
	ui.Column(c.Key("picker")).Background(style.Color(colors.Background)).Children(func() {
		ui.Row(c).MinHeight(52).Padding(10, 16).Gap(12).AlignItems(ui.Center).Children(func() {
			style.Text(ui.Text(c, cmpOr(here, style.T(c, "libraries.roots"))), style.Type(c).Callout).
				TextColor(style.Color(colors.TextSecondary)).Grow(1).MinWidth(0).Selectable()
			if smallButton(c, style.T(c, "libraries.chooseFolder"), here == "" || slices.Contains(chosen, here)) {
				choose(here)
			}
		})
		ui.Box(c).Height(1).FillWidth().Background(style.Color(colors.Border))
		ui.Scroll(c.Key(path)).MaxHeight(320).Children(func() {
			ui.Column(c).Children(func() {
				row := func(key, label, icon, to string) {
					b := ui.ButtonBase(c.Key(key)).MinHeight(44).Padding(0, 16).Gap(12).AlignItems(ui.Center).Justify(ui.Start).Label(label)
					if b.Hovered() {
						b.Background(style.Color(colors.Fill))
					}
					b.Children(func() {
						ui.Icon(c, style.Icon(icon, false)).Size(18, 18).Shrink(0).TextColor(style.Color(colors.TextSecondary))
						style.Text(ui.Text(c, label), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).MaxLines(1).Grow(1).MinWidth(0)
					})
					if b.Clicked() {
						f.path = to
					}
				}
				if here != "" {
					row("..", style.T(c, "libraries.parentFolder"), "back", parent)
				}
				for _, x := range folders {
					row(x.path, x.name, "folder", x.path)
				}
				switch {
				case err != nil:
					style.Text(ui.Text(c, errorMessage(c, err)), style.Type(c).Callout).TextColor(style.Color(colors.Danger)).Padding(12, 16)
				case loaded && len(folders) == 0:
					style.Text(ui.Text(c, style.T(c, "libraries.noFolders")), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary)).Padding(12, 16)
				}
			})
		})
	})
}

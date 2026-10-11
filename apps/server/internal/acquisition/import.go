package acquisition

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/organize"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/naming"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
	"github.com/mavioai/mavio/libs/release"
)

// The warnings of imports, Radarr's.
const (
	msgNoOutputPath    = "Download doesn't contain intermediate path, Skipping."
	msgNotLocalPath    = "[%s] is not a valid local path. You may need a Remote Path Mapping."
	msgNoMovie         = "Unable to parse download, automatic import is not possible."
	msgNoFiles         = "No files found are eligible for import in %s"
	msgNotAllImported  = "One or more movies expected in this release were not imported or missing"
	msgSample          = "Sample"
	msgSampleUnknown   = "Unable to determine if file is a sample"
	msgNoAudio         = "No audio tracks detected"
	msgUnreadable      = "Unable to read file: %v"
	msgNoFreeSpace     = "Not enough free space"
	msgNoLibraryFolder = "Movie folder is not in a folder of the library"
)

// importFile is a video file of a download.
type importFile struct {
	// path is the server's path; rel is that below the download's.
	path, rel string
	size      int64
	// inFolder reports whether the download is a folder.
	inFolder bool
}

// importDownload imports the best video file of a completed download into
// its movie's folder, as Radarr's CompletedDownloadService.Import and
// ImportApprovedMovie: the largest file no import specification rejects
// is imported, upgrading the movie's file; when none is, the download is
// blocked with why.
func (r *Runtime) importDownload(ctx context.Context, rn *run, d *core.Download, p polled, mappings []acquire.RemotePathMapping) error {
	out := p.item.GetOutputPath()
	if strings.TrimSpace(out) == "" {
		return r.warn(ctx, d, msgNoOutputPath)
	}
	local := acquire.RemapRemoteToLocal(mappings, p.client.GetHost(), out)
	if !filepath.IsAbs(local) {
		return r.warn(ctx, d, fmt.Sprintf(msgNotLocalPath, local))
	}
	local = filepath.Clean(local)
	if isSeriesDownload(*d) {
		return r.importEpisodeDownload(ctx, rn, d, p, local, mappings)
	}
	if done, err := r.alreadyImported(ctx, d); err != nil || done {
		if done {
			return r.completeImport(ctx, d, p)
		}
		return err
	}
	m, err := rn.movie(ctx, d.ItemID)
	if errors.Is(err, core.ErrNotFound) {
		d.State = core.DownloadImportBlocked
		return r.warn(ctx, d, msgNoMovie)
	}
	if err != nil {
		return err
	}
	d.State = core.DownloadImporting
	if err := r.putDownload(ctx, d); err != nil {
		return err
	}
	files, err := importFiles(ctx, local)
	if err != nil {
		r.logger().WarnContext(ctx, "couldn't read download", "path", local, "err", err)
	}
	if len(files) == 0 {
		d.State = core.DownloadImportPending
		return r.warn(ctx, d, fmt.Sprintf(msgNoFiles, local))
	}
	msgs := []string{msgNotAllImported}
	for _, f := range files {
		reasons, err := r.importOne(ctx, rn, m, d, p, f)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			reasons = append(reasons, err.Error())
		}
		if len(reasons) == 0 {
			if p.item.GetCanMoveFiles() {
				r.cleanDownloadFolder(ctx, m, local, mappings)
			}
			return r.completeImport(ctx, d, p)
		}
		msgs = append(msgs, filepath.Base(f.path)+": "+strings.Join(reasons, "; "))
	}
	d.State = core.DownloadImportBlocked
	return r.warn(ctx, d, msgs...)
}

// alreadyImported reports whether a download's movie was imported from
// it, as Radarr's TrackedDownloadAlreadyImported.
func (r *Runtime) alreadyImported(ctx context.Context, d *core.Download) (bool, error) {
	page, err := r.Store.History().List(ctx, core.HistoryQuery{
		ItemIDs: []core.ID{d.ItemID}, Types: []core.HistoryEventType{core.HistoryImported}, DownloadID: d.DownloadID, Limit: 1,
	})
	if err != nil {
		return false, err
	}
	return len(page.Items) > 0, nil
}

// completeImport marks a download imported, tells its client and removes
// it from the client when the client is set to, as Radarr's
// DownloadEventHub on DownloadCompletedEvent.
func (r *Runtime) completeImport(ctx context.Context, d *core.Download, p polled) error {
	d.State, d.Messages = core.DownloadImported, nil
	if err := r.putDownload(ctx, d); err != nil {
		return err
	}
	if api := r.clientAPI(d.PluginID); api != nil {
		_, err := api.MarkDownloadImported(ctx, pluginv1.MarkDownloadImportedRequest_builder{ClientId: new(d.ClientID), DownloadId: new(d.DownloadID)}.Build())
		if err != nil && connect.CodeOf(err) != connect.CodeUnimplemented {
			r.logger().ErrorContext(ctx, "couldn't mark download as imported", "download", d.Title, "err", err)
		}
	}
	return r.removeImported(ctx, d, p)
}

// importFiles returns the video files of a download, the largest first,
// leaving out those of extras, samples and hidden folders, as Radarr's
// DiskScanService.
func importFiles(ctx context.Context, local string) ([]importFile, error) {
	parser := naming.Default()
	fi, err := os.Stat(local)
	if err != nil {
		return nil, err
	}
	if !fi.IsDir() {
		if !parser.IsVideoFile(local) {
			return nil, nil
		}
		return []importFile{{path: local, rel: filepath.Base(local), size: fi.Size()}}, nil
	}
	sizes := map[string]int64{}
	var paths []string
	err = filepath.WalkDir(local, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() || !parser.IsVideoFile(p) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		paths = append(paths, p)
		sizes[p] = info.Size()
		return nil
	})
	if err != nil {
		return nil, err
	}
	var out []importFile
	for _, p := range acquire.FilterImportPaths(local, paths) {
		rel, _ := filepath.Rel(local, p)
		out = append(out, importFile{path: p, rel: rel, size: sizes[p], inFolder: true})
	}
	slices.SortStableFunc(out, func(a, b importFile) int { return cmp.Compare(b.size, a.size) })
	return out, nil
}

// importPlan is what an import of a file was decided with.
type importPlan struct {
	probed          library.ProbeResult
	q               acquire.QualityModel
	langs           []release.Language
	formats         []acquire.CustomFormat
	libPath, folder string
}

// importOne imports a file of a download for a movie unless an import
// specification rejects it, returning why; errors are of the transfer.
func (r *Runtime) importOne(ctx context.Context, rn *run, m *movie, d *core.Download, p polled, f importFile) ([]string, error) {
	plan, reasons, err := r.planImport(ctx, rn, m, d, f, nil, false)
	if err != nil || len(reasons) > 0 {
		return reasons, err
	}
	return nil, r.importPlanned(ctx, rn, m, d, p, f, plan)
}

// planImport decides the import of a file of a download for a movie,
// returning why the import specifications reject it: the first reasons
// found, or every reason when all is set. quality, when set, is the
// file's quality rather than that read from it and its names. The plan
// has no library folder when the movie has none.
func (r *Runtime) planImport(ctx context.Context, rn *run, m *movie, d *core.Download, f importFile, quality *acquire.QualityModel, all bool) (importPlan, []string, error) {
	var plan importPlan
	probed, reasons, err := r.verify(ctx, m, f)
	if err != nil {
		return plan, nil, err
	}
	if len(reasons) > 0 && !all {
		return plan, reasons, nil
	}
	plan.probed = probed
	plan.q = importQuality(d, f, probed)
	if quality != nil {
		plan.q = *quality
	}
	plan.langs = languagesOf(d.Languages)
	plan.formats = rn.formats(d.Title, plan.q, plan.langs, f.size, nil)
	upgrades := acquire.Upgrades{Propers: rn.env.Config.Propers}
	if reason, msg := upgrades.ImportUpgrade(m.target.Profile, m.acq.File, plan.q, plan.formats); reason != "" {
		if reasons = append(reasons, msg); !all {
			return plan, reasons, nil
		}
	}

	libPath, folder, ok := libraryFolder(m)
	if !ok {
		return plan, append(reasons, msgNoLibraryFolder), nil
	}
	plan.libPath, plan.folder = libPath, folder
	s := r.settings()
	if !s.SkipFreeSpaceCheck {
		// The volume of the movie's folder, which may be another than the
		// library folder's, or of its nearest parent until it exists, as
		// Radarr's FreeSpaceSpecification.
		dir := filepath.Join(libPath, filepath.FromSlash(folder))
		if free, known := r.freeSpace(dir); known && free < f.size+int64(s.MinimumFreeSpace)*1024*1024 {
			reasons = append(reasons, msgNoFreeSpace)
		}
	}
	return plan, reasons, nil
}

// importPlanned imports a file of a download for a movie as planned.
func (r *Runtime) importPlanned(ctx context.Context, rn *run, m *movie, d *core.Download, p polled, f importFile, plan importPlan) error {
	probed, q, langs, formats := plan.probed, plan.q, plan.langs, plan.formats
	libPath, folder := plan.libPath, plan.folder
	s := r.settings()

	parsed := release.ParseMovieTitle(d.Title, false)
	if parsed == nil {
		parsed = release.ParseMoviePath(f.path)
	}
	file := organize.File{
		Quality: rn.qualityTitle(q.Quality), Revision: organize.Revision{Version: q.Revision.Version, Real: q.Revision.Real},
		SceneName: d.Title, RelativePath: filepath.ToSlash(f.rel), Path: filepath.ToSlash(f.path),
	}
	if parsed != nil {
		file.Edition, file.ReleaseGroup = parsed.Edition, parsed.ReleaseGroup
	}
	file.MediaInfo = mediaInfoOf(probed, d.Title)
	for _, cf := range formats {
		file.CustomFormats = append(file.CustomFormats, organize.CustomFormat{Name: cf.Name, IncludeWhenRenaming: cf.IncludeWhenRenaming})
	}
	name, err := organize.BuildFileName(rn.naming(), organizeMovie(m.item), file)
	if err != nil {
		return fmt.Errorf("name file: %w", err)
	}
	rel := filepath.FromSlash(path.Join(folder, name)) + filepath.Ext(f.path)
	if !filepath.IsLocal(rel) {
		return fmt.Errorf("file name %q leaves the library folder", rel)
	}
	root, err := os.OpenRoot(libPath)
	if err != nil {
		return fmt.Errorf("library folder: %w", err)
	}
	defer func() { _ = root.Close() }()
	if err := root.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		return fmt.Errorf("movie folder: %w", err)
	}

	mode := transferCopy
	switch {
	case p.item.GetCanMoveFiles():
		mode = transferMove
	case s.CopyUsingHardlinks:
		mode = transferHardLinkOrCopy
	}
	dest := filepath.Join(libPath, rel)
	done, err := r.pace(ctx, dest)
	if err != nil {
		return err
	}
	defer done()
	if m.file != "" {
		old := filepath.FromSlash(m.file)
		if err := r.recycle(ctx, old, path.Base(path.Dir(m.file))); err != nil {
			return err
		}
		r.recycleExtras(ctx, old, path.Base(path.Dir(m.file)), s.ExtraFileExtensions)
		if err := r.Store.History().Add(ctx, core.HistoryEvent{
			ID: core.NewID(), Type: core.HistoryFileDeleted, ItemID: m.item.ID, Date: r.now(), SourceTitle: m.file,
			Quality: qualityRef(m.acq.File.Quality), Languages: languageNames(m.acq.File.Languages), Data: map[string]string{dataReason: "Upgrade"},
		}); err != nil {
			return err
		}
	}
	if err := transfer(root, f.path, rel, mode); err != nil {
		return fmt.Errorf("%s file: %w", mode, err)
	}
	r.logger().InfoContext(ctx, "imported movie file", "item", m.item.ID, "from", f.path, "to", dest, "mode", mode.String())
	if s.ImportExtraFiles {
		r.importMovieExtras(ctx, root, m, f, rel, mode, s.ExtraFileExtensions)
	}

	imported := filepath.ToSlash(dest)
	it, err := r.Store.Items().Get(ctx, m.item.ID)
	if err != nil {
		return err
	}
	it.Path, it.Virtual = imported, false
	if err := r.Store.Items().Upsert(ctx, it); err != nil {
		return err
	}
	// A scan of the movie's folder alone, rather than of its library.
	job := library.ScanFolderJob(m.lib, filepath.Dir(dest), r.now())
	if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
		return fmt.Errorf("queue scan: %w", err)
	}
	data := map[string]string{
		dataImportedPath: imported, dataDroppedPath: filepath.ToSlash(f.path),
		dataDownloadClient: p.client.Ref(), dataDownloadClientName: p.client.GetName(),
		dataSceneName: d.Title, dataSize: strconv.FormatInt(f.size, 10), dataIndexer: d.Indexer,
		dataCustomFormatScore: strconv.Itoa(m.target.Profile.FormatScore(formats)),
	}
	if file.ReleaseGroup != "" {
		data[dataReleaseGroup] = file.ReleaseGroup
	}
	if err := r.Store.History().Add(ctx, core.HistoryEvent{
		ID: core.NewID(), Type: core.HistoryImported, ItemID: m.item.ID, Date: r.now(), SourceTitle: d.Title,
		Quality: qualityRef(q), Languages: languageNames(langs), DownloadID: d.DownloadID, Data: data,
	}); err != nil {
		return err
	}
	// The movie has its file now: later decisions in this run see it.
	delete(rn.movies, m.item.ID)
	r.imported(ctx, m.item.ID)
	return nil
}

// verify reads a file and rejects samples and files without audio, as
// Radarr's NotSampleSpecification and HasAudioTrackSpecification; a file
// that cannot be read is rejected too.
func (r *Runtime) verify(ctx context.Context, m *movie, f importFile) (library.ProbeResult, []string, error) {
	res, reasons, err := r.probeFile(ctx, f)
	if res == nil {
		return library.ProbeResult{}, reasons, err
	}
	return *res, checkProbe(f, res, m.acq.Runtime, false), nil
}

// probeFile reads a file; nil with why when it cannot be read.
func (r *Runtime) probeFile(ctx context.Context, f importFile) (*library.ProbeResult, []string, error) {
	if r.Prober == nil {
		return nil, []string{fmt.Sprintf(msgUnreadable, "no prober")}, nil
	}
	done, err := r.pace(ctx, f.path)
	if err != nil {
		return nil, nil, err
	}
	res, err := r.Prober.Probe(ctx, f.path, false)
	done()
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, ctx.Err()
		}
		return nil, []string{fmt.Sprintf(msgUnreadable, err)}, nil
	}
	return &res, nil, nil
}

// checkProbe rejects samples of a runtime in minutes and files without
// audio among files read; specials are not checked for samples.
func checkProbe(f importFile, res *library.ProbeResult, runtime int, special bool) []string {
	var reasons []string
	if !special {
		switch acquire.IsSample(f.path, &res.Source.Duration, runtime) {
		case acquire.SampleYes:
			reasons = append(reasons, msgSample)
		case acquire.SampleIndeterminate:
			reasons = append(reasons, msgSampleUnknown)
		}
	}
	if !slices.ContainsFunc(res.Source.Streams, func(s core.MediaStream) bool { return s.Kind == core.StreamAudio }) {
		reasons = append(reasons, msgNoAudio)
	}
	return reasons
}

// libraryFolder returns the library folder a movie's folder is in and the
// movie folder below it.
func libraryFolder(m *movie) (string, string, bool) {
	folder := m.target.Path
	for _, p := range m.lib.Paths {
		base := filepath.ToSlash(filepath.Clean(p))
		if rel, ok := strings.CutPrefix(folder, strings.TrimSuffix(base, "/")+"/"); ok && rel != "" {
			return filepath.Clean(p), rel, true
		}
	}
	return "", "", false
}

// qualityTitle returns a quality's title, as the definitions name it.
func (rn *run) qualityTitle(q release.Quality) string {
	for _, d := range rn.env.Definitions {
		if d.Quality == q && d.Title != "" {
			return d.Title
		}
	}
	return q.String()
}

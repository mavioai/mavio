package acquisition

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/acquire/organize"
	"github.com/mavioai/mavio/libs/acquire/series"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/release"
)

// The warnings of episode imports, Sonarr's.
const (
	msgNoSeries               = "Unable to parse download, automatic import is not possible."
	msgNotAllEpisodesImported = "One or more episodes expected in this release were not imported or missing from the release"
	msgUnknownEpisodes        = "Invalid season or episode"
	msgFullSeason             = "Single episode file contains all episodes in seasons. Review file name or manually import"
	msgSplitEpisode           = "Single episode split into multiple files"
	msgExistingHasMore        = "Episode file on disk contains more episodes than this file contains"
	msgNotInRelease           = "Episode %s was not found in the grabbed release: %s"
	msgEpisodeImported        = "Episode file already imported at %s"
	msgNoSeriesFolder         = "Series folder is not in a folder of the library"
)

// slot is an episode by season and number.
type slot struct{ season, episode int }

func slotOf(ep *series.Episode) slot { return slot{ep.SeasonNumber, ep.EpisodeNumber} }

func (s slot) String() string { return fmt.Sprintf("%dx%02d", s.season, s.episode) }

// localEpisode is a video file of a download with the episodes it holds,
// as Sonarr's LocalEpisode, and why it is not imported.
type localEpisode struct {
	file     importFile
	parsed   *release.ParsedEpisodeInfo
	episodes []*series.Episode
	quality  acquire.QualityModel
	langs    []release.Language
	formats  []acquire.CustomFormat
	// sceneName is the release name of the file, as Sonarr's
	// SceneNameCalculator; empty for its file name.
	sceneName string
	// folderInfo is what the name of the download's folder holds, as
	// Sonarr's FolderEpisodeInfo; nil for a lone file.
	folderInfo *release.ParsedEpisodeInfo
	reasons    []string
}

// importEpisodeDownload imports the video files of a completed episode
// download into its series' folder, as Sonarr's
// CompletedDownloadService.Import, ImportDecisionMaker and
// ImportApprovedEpisodes: each file maps to the episodes its name holds,
// the files holding the most episodes, then the largest, first; a file
// whose episodes another one brought is skipped. The download is
// imported once every episode it was grabbed for is; else it is blocked
// with why. A folder the files were moved out of is deleted once only
// samples are left in it.
func (r *Runtime) importEpisodeDownload(ctx context.Context, rn *run, d *core.Download, p polled, local string, mappings []acquire.RemotePathMapping) error {
	sh, err := rn.show(ctx, d.ItemID)
	if errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrInvalid) {
		d.State = core.DownloadImportBlocked
		return r.warn(ctx, d, msgNoSeries)
	}
	if err != nil {
		return err
	}
	expected := expectedSlots(sh, d)
	imported, err := r.importedSlots(ctx, sh, d)
	if err != nil {
		return err
	}
	if len(imported) > 0 && allImported(expected, imported) {
		return r.completeImport(ctx, d, p)
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
	locals, err := r.episodeDecisions(ctx, rn, sh, d, local, files, expected)
	if err != nil {
		return err
	}
	// The files holding the most episodes first, then the largest.
	slices.SortStableFunc(locals, func(a, b *localEpisode) int {
		return cmp.Or(cmp.Compare(len(b.episodes), len(a.episodes)), cmp.Compare(b.file.size, a.file.size))
	})
	var msgs []string
	moved := false
	for _, le := range locals {
		if len(le.reasons) == 0 {
			if i := slices.IndexFunc(le.episodes, func(ep *series.Episode) bool { return imported[slotOf(ep)] != "" }); i >= 0 {
				le.reasons = append(le.reasons, fmt.Sprintf(msgEpisodeImported, imported[slotOf(le.episodes[i])]))
			}
		}
		if len(le.reasons) == 0 {
			dest, reasons, err := r.importEpisodeFile(ctx, rn, sh, d, p, le)
			if err != nil {
				if ctx.Err() != nil {
					return ctx.Err()
				}
				reasons = append(reasons, err.Error())
			}
			le.reasons = reasons
			if len(reasons) == 0 {
				moved = moved || p.item.GetCanMoveFiles()
				for _, ep := range le.episodes {
					imported[slotOf(ep)] = dest
				}
				// The episodes have their files now: later decisions in this
				// run read the series again.
				delete(rn.shows, sh.item.ID)
				rn.showList, rn.showsRead, rn.parser = nil, false, nil
				continue
			}
		}
		msgs = append(msgs, filepath.Base(le.file.path)+": "+strings.Join(le.reasons, "; "))
	}
	if moved {
		r.cleanEpisodeDownloadFolder(ctx, sh, local, mappings)
	}
	if len(imported) > 0 && allImported(expected, imported) {
		return r.completeImport(ctx, d, p)
	}
	d.State = core.DownloadImportBlocked
	return r.warn(ctx, d, append([]string{msgNotAllEpisodesImported}, msgs...)...)
}

// expectedSlots returns the episodes a download was grabbed for, by
// season and number: its item IDs are episodes of the series, or of
// episodes multi-episode files cover. Episodes gone since are left out.
func expectedSlots(sh *show, d *core.Download) []slot {
	var out []slot
	for _, id := range d.ItemIDs {
		if ep, ok := sh.byID[id]; ok && !slices.Contains(out, slotOf(ep)) {
			out = append(out, slotOf(ep))
		}
	}
	return out
}

// allImported reports whether every expected episode was imported.
func allImported(expected []slot, imported map[slot]string) bool {
	return !slices.ContainsFunc(expected, func(s slot) bool { return imported[s] == "" })
}

// importedSlots returns the episodes of a series imported from a
// download, with the paths they were imported to, as Sonarr's
// TrackedDownloadAlreadyImported reads the history: those of the items
// with an import of it.
func (r *Runtime) importedSlots(ctx context.Context, sh *show, d *core.Download) (map[slot]string, error) {
	page, err := r.Store.History().List(ctx, core.HistoryQuery{
		ItemIDs: []core.ID{sh.item.ID}, Types: []core.HistoryEventType{core.HistoryImported}, DownloadID: d.DownloadID, Limit: core.MaxPageSize,
	})
	if err != nil {
		return nil, err
	}
	out := map[slot]string{}
	for _, h := range page.Items {
		for _, ep := range sh.episodes {
			if sh.items[ep.ID].ID == h.ItemID {
				out[slotOf(ep)] = cmp.Or(h.Data[dataImportedPath], h.SourceTitle)
			}
		}
	}
	return out, nil
}

// episodeDecisions maps the files of a download, at local, to their
// episodes and runs Sonarr's import specifications on each: the episodes
// must be known, grabbed, expected by the name of the download's folder,
// with titles and absolute numbers when the file names need them, and of
// confirmed scene numbering; the file no sample, with audio, not being
// unpacked, fitting the free space, no full season nor split episode,
// and an upgrade of the files it replaces, none holding more episodes.
func (r *Runtime) episodeDecisions(ctx context.Context, rn *run, sh *show, d *core.Download, local string, files []importFile, expected []slot) ([]*localEpisode, error) {
	ps, err := rn.parsing(ctx)
	if err != nil {
		return nil, err
	}
	// The download's title stands for a lone file whose name does not
	// parse, as Sonarr's download client info does.
	downloadInfo := release.ParseSeriesTitle(d.Title)
	grabbedQuality := release.ParseQuality(d.Title, release.Series)
	// The name of the download's folder, Sonarr's folder info.
	var folderInfo *release.ParsedEpisodeInfo
	if len(files) > 0 && files[0].path != local {
		folderInfo = release.ParseSeriesTitle(filepath.Base(local))
	}
	now := r.now()
	naming, oseries := rn.seriesNaming(), organizeSeries(sh)
	langs := languagesOf(d.Languages)
	upgrades := acquire.Upgrades{Propers: rn.env.Config.Propers}
	s := r.settings()
	// The free space of the series folder's volume, which may be another
	// than the library folder's, or of its nearest parent until it exists,
	// as Sonarr's FreeSpaceSpecification.
	free, freeKnown := int64(0), false
	if libPath, seriesRel, ok := seriesLibraryFolder(sh); ok && !s.SkipFreeSpaceCheck {
		free, freeKnown = r.freeSpace(filepath.Join(libPath, filepath.FromSlash(seriesRel)))
	}
	out := make([]*localEpisode, 0, len(files))
	for _, f := range files {
		le := &localEpisode{file: f, langs: langs, folderInfo: folderInfo}
		out = append(out, le)
		le.parsed = release.ParseSeriesPath(f.path)
		fileInfo := le.parsed
		single := len(files) == 1
		if le.parsed == nil && single {
			le.parsed = downloadInfo
		}
		switch {
		case le.parsed == nil:
			le.reasons = append(le.reasons, msgUnknownEpisodes)
			continue
		case le.parsed.FullSeason:
			le.reasons = append(le.reasons, msgFullSeason)
			continue
		case le.parsed.IsSplitEpisode:
			le.reasons = append(le.reasons, msgSplitEpisode)
			continue
		}
		m, err := ps.Map(ctx, le.parsed, 0, 0, "", sh.series, nil)
		if err != nil {
			return nil, err
		}
		if m == nil || len(m.Episodes) == 0 {
			le.reasons = append(le.reasons, msgUnknownEpisodes)
			continue
		}
		le.episodes = m.Episodes
		probe, unreadable, err := r.probeFile(ctx, f)
		if err != nil {
			return nil, err
		}
		le.quality = acquire.ModelOf(aggregateQuality(fileInfo, folderInfo, downloadInfo, probe, grabbedQuality))
		if len(le.langs) == 0 {
			le.langs = le.parsed.Languages
		}
		le.sceneName = sceneNameOf(f.path, single, downloadInfo, folderInfo)
		le.formats = rn.seriesFormats(cmp.Or(le.sceneName, strings.TrimSuffix(filepath.Base(f.path), filepath.Ext(f.path))), le.quality, le.langs, f.size, nil)

		// MatchesGrabSpecification.
		for _, ep := range le.episodes {
			if len(expected) > 0 && !slices.Contains(expected, slotOf(ep)) {
				le.reasons = append(le.reasons, fmt.Sprintf(msgNotInRelease, slotOf(ep), d.Title))
			}
		}
		// SameEpisodesImportSpecification and UpgradeSpecification.
		checked := map[core.ID]bool{}
		for _, ep := range le.episodes {
			if ep.File == nil || checked[ep.File.ID] {
				continue
			}
			checked[ep.File.ID] = true
			for _, other := range sh.episodes {
				if other.File != nil && other.File.ID == ep.File.ID && !slices.Contains(le.episodes, other) {
					le.reasons = append(le.reasons, msgExistingHasMore)
					break
				}
			}
			if reason, msg := upgrades.ImportUpgrade(sh.series.Profile, ep.File, le.quality, le.formats); reason != "" {
				le.reasons = append(le.reasons, msg)
			}
		}
		if freeKnown && free < f.size+int64(s.MinimumFreeSpace)*1024*1024 {
			le.reasons = append(le.reasons, msgNoFreeSpace)
		}
		// NotUnpackingSpecification, MatchesFolderSpecification,
		// EpisodeTitleSpecification, UnverifiedSceneNumberingSpecification
		// and AbsoluteEpisodeNumberSpecification.
		if unpacking(f.path, now) {
			le.reasons = append(le.reasons, msgUnpacking)
		}
		msg, err := unexpectedEpisodes(ctx, ps, sh.series, le.parsed, folderInfo)
		if err != nil {
			return nil, err
		}
		if msg != "" {
			le.reasons = append(le.reasons, msg)
		}
		if msg := missingEpisodeTitle(s.EpisodeTitleRequired, naming, oseries, le.episodes, sh.episodes, now); msg != "" {
			le.reasons = append(le.reasons, msg)
		}
		if unverifiedSceneNumbering(le.episodes) {
			le.reasons = append(le.reasons, msgUnverifiedScene)
		}
		if missingAbsoluteNumber(naming, oseries, le.episodes, now) {
			le.reasons = append(le.reasons, msgNoAbsoluteNumber)
		}
		if probe == nil {
			le.reasons = append(le.reasons, unreadable...)
			continue
		}
		runtime := 0
		special := false
		for _, ep := range le.episodes {
			runtime += cmp.Or(ep.Runtime, sh.series.Runtime)
			special = special || ep.SeasonNumber == 0
		}
		le.reasons = append(le.reasons, checkProbe(f, probe, cmp.Or(runtime, 45), special)...)
	}
	return out, nil
}

// aggregateQuality returns the quality of an episode file, as Sonarr's
// AggregateQuality with its augmenters in order: the names of the file,
// of the download's folder and of the download, the frame size of the
// file's video, and the name of the release grabbed.
func aggregateQuality(fileInfo, folderInfo, downloadInfo *release.ParsedEpisodeInfo, probe *library.ProbeResult, grabbed release.ParsedQuality) release.ParsedQuality {
	var augments []release.QualityAugment
	for _, info := range []*release.ParsedEpisodeInfo{fileInfo, folderInfo, downloadInfo} {
		if info != nil {
			augments = append(augments, release.AugmentFromName(info.Quality))
		}
	}
	if probe != nil {
		for _, s := range probe.Source.Streams {
			if s.Kind != core.StreamVideo {
				continue
			}
			if a, ok := release.AugmentFromVideo(s.Width, s.Height); ok {
				augments = append(augments, a)
			}
			break
		}
	}
	augments = append(augments, release.AugmentFromName(grabbed))
	return release.AggregateSeriesQuality(augments...)
}

// seriesLibraryFolder returns the library folder a series' folder is in
// and the series folder below it.
func seriesLibraryFolder(sh *show) (string, string, bool) {
	folder := sh.series.Path
	for _, p := range sh.lib.Paths {
		base := filepath.ToSlash(filepath.Clean(p))
		if rel, ok := strings.CutPrefix(folder, strings.TrimSuffix(base, "/")+"/"); ok && rel != "" {
			return filepath.Clean(p), rel, true
		}
	}
	return "", "", false
}

// importEpisodeFile moves, links or copies a file into its season's
// folder, named after its episodes, as Sonarr's
// UpgradeMediaFileService and EpisodeFileMovingService: the files it
// replaces go to the recycle bin. The episode item of its first episode
// takes the file, holding the others; the series' and season's items
// become physical. It returns where the file went, or why it was not
// imported; errors are of the transfer.
func (r *Runtime) importEpisodeFile(ctx context.Context, rn *run, sh *show, d *core.Download, p polled, le *localEpisode) (string, []string, error) {
	libPath, seriesRel, ok := seriesLibraryFolder(sh)
	if !ok {
		return "", []string{msgNoSeriesFolder}, nil
	}
	episodes := slices.Clone(le.episodes)
	slices.SortFunc(episodes, func(a, b *series.Episode) int {
		return cmp.Or(cmp.Compare(a.SeasonNumber, b.SeasonNumber), cmp.Compare(a.EpisodeNumber, b.EpisodeNumber))
	})
	q := le.quality
	file := organize.EpisodeFile{
		Quality: rn.qualityTitle(q.Quality), Revision: organize.Revision{Version: q.Revision.Version, Real: q.Revision.Real},
		SceneName: le.sceneName, RelativePath: filepath.ToSlash(le.file.rel), Path: filepath.ToSlash(le.file.path),
	}
	if le.parsed != nil {
		file.ReleaseGroup, file.ReleaseHash = le.parsed.ReleaseGroup, le.parsed.ReleaseHash
	}
	for _, cf := range le.formats {
		file.CustomFormats = append(file.CustomFormats, organize.CustomFormat{Name: cf.Name, IncludeWhenRenaming: cf.IncludeWhenRenaming})
	}
	dest, err := organize.BuildEpisodeFilePath(rn.seriesNaming(), organizeSeries(sh), organizeEpisodes(episodes), file, filepath.Ext(le.file.path))
	if err != nil {
		return "", nil, fmt.Errorf("name file: %w", err)
	}
	seriesFolder := strings.TrimSuffix(sh.series.Path, "/") + "/"
	relToSeries, ok := strings.CutPrefix(dest, seriesFolder)
	if !ok {
		return "", nil, fmt.Errorf("file %q is not in the series folder", dest)
	}
	rel := filepath.FromSlash(path.Join(seriesRel, relToSeries))
	if !filepath.IsLocal(rel) {
		return "", nil, fmt.Errorf("file name %q leaves the library folder", rel)
	}
	return r.placeEpisodeFile(ctx, rn, sh, d, p, le, episodes, libPath, rel, q)
}

// placeEpisodeFile transfers an episode file to rel in a library folder
// and records it.
func (r *Runtime) placeEpisodeFile(ctx context.Context, rn *run, sh *show, d *core.Download, p polled, le *localEpisode,
	episodes []*series.Episode, libPath, rel string, q acquire.QualityModel,
) (string, []string, error) {
	root, err := os.OpenRoot(libPath)
	if err != nil {
		return "", nil, fmt.Errorf("library folder: %w", err)
	}
	defer func() { _ = root.Close() }()
	if err := root.MkdirAll(filepath.Dir(rel), 0o755); err != nil {
		return "", nil, fmt.Errorf("season folder: %w", err)
	}
	s := r.settings()
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
		return "", nil, err
	}
	defer done()

	// The files the episodes had go, each once.
	var old []core.Item
	for _, ep := range episodes {
		it := sh.items[ep.ID]
		if ep.File != nil && hasFile(it) && !slices.ContainsFunc(old, func(o core.Item) bool { return o.ID == it.ID }) {
			old = append(old, it)
		}
	}
	for _, it := range old {
		f := sh.byID[it.ID].File
		oldPath := filepath.FromSlash(it.Path)
		if err := r.recycle(ctx, oldPath, path.Base(sh.series.Path)); err != nil {
			return "", nil, err
		}
		r.recycleExtras(ctx, oldPath, path.Base(sh.series.Path), s.ExtraFileExtensions)
		if err := r.Store.History().Add(ctx, core.HistoryEvent{
			ID: core.NewID(), Type: core.HistoryFileDeleted, ItemID: it.ID, ParentID: sh.item.ID, Date: r.now(), SourceTitle: it.Path,
			Quality: qualityRef(f.Quality), Languages: languageNames(f.Languages), Data: map[string]string{dataReason: "Upgrade"},
		}); err != nil {
			return "", nil, err
		}
	}
	if err := transfer(root, le.file.path, rel, mode); err != nil {
		return "", nil, fmt.Errorf("%s file: %w", mode, err)
	}
	r.logger().InfoContext(ctx, "imported episode file", "series", sh.item.Name, "from", le.file.path, "to", dest, "mode", mode.String())
	if s.ImportExtraFiles {
		r.importEpisodeExtras(ctx, root, sh, le, le.folderInfo != nil, rel, mode, s.ExtraFileExtensions)
	}
	imported := filepath.ToSlash(dest)
	holder, err := r.recordEpisodeItems(ctx, sh, episodes, imported)
	if err != nil {
		return "", nil, err
	}
	// A scan of the series' folder alone, rather than of its library.
	seriesDir := libPath
	if _, seriesRel, ok := seriesLibraryFolder(sh); ok {
		seriesDir = filepath.Join(libPath, filepath.FromSlash(seriesRel))
	}
	job := library.ScanFolderJob(sh.lib, seriesDir, r.now())
	if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
		return "", nil, fmt.Errorf("queue scan: %w", err)
	}
	data := map[string]string{
		dataImportedPath: imported, dataDroppedPath: filepath.ToSlash(le.file.path),
		dataDownloadClient: p.client.Ref(), dataDownloadClientName: p.client.GetName(),
		dataSceneName: le.sceneName, dataSize: strconv.FormatInt(le.file.size, 10), dataIndexer: d.Indexer,
		dataCustomFormatScore: strconv.Itoa(sh.series.Profile.FormatScore(le.formats)),
	}
	if le.parsed != nil && le.parsed.ReleaseGroup != "" {
		data[dataReleaseGroup] = le.parsed.ReleaseGroup
	}
	for k, v := range data {
		if v == "" {
			delete(data, k)
		}
	}
	if err := r.Store.History().Add(ctx, core.HistoryEvent{
		ID: core.NewID(), Type: core.HistoryImported, ItemID: holder, ParentID: sh.item.ID, Date: r.now(),
		SourceTitle: cmp.Or(le.sceneName, d.Title), Quality: qualityRef(q), Languages: languageNames(le.langs), DownloadID: d.DownloadID, Data: data,
	}); err != nil {
		return "", nil, err
	}
	r.imported(ctx, sh.item.ID)
	return imported, nil, nil
}

// recordEpisodeItems gives an imported file to the item of its first
// episode, which holds the others up to its last, and deletes the items
// of the others, as the library does with virtual episodes a file
// covers; the series' item and the season's become physical, so that
// scans adopt them. It returns the holding item's ID.
func (r *Runtime) recordEpisodeItems(ctx context.Context, sh *show, episodes []*series.Episode, imported string) (core.ID, error) {
	st := r.Store.Items()
	first := sh.items[episodes[0].ID]
	it, err := st.Get(ctx, first.ID)
	if err != nil {
		return core.NilID, err
	}
	it.Path, it.Virtual, it.MissingSince, it.IndexNumberEnd = imported, false, nil, nil
	if last := episodes[len(episodes)-1]; len(episodes) > 1 && last.SeasonNumber == episodes[0].SeasonNumber && last.EpisodeNumber > *it.IndexNumber {
		it.IndexNumberEnd = new(last.EpisodeNumber)
	}
	if err := st.Upsert(ctx, it); err != nil {
		return core.NilID, err
	}
	var gone []core.ID
	for _, ep := range episodes[1:] {
		if o := sh.items[ep.ID]; o.ID == ep.ID && o.ID != it.ID && !slices.Contains(gone, o.ID) {
			gone = append(gone, o.ID)
		}
	}
	if len(gone) > 0 {
		// The items of the other episodes go, their history kept by the
		// file's.
		if err := r.Store.History().MoveItems(ctx, it.ID, gone...); err != nil {
			return core.NilID, err
		}
		if err := st.Delete(ctx, gone...); err != nil {
			return core.NilID, err
		}
	}
	if sh.item.Virtual || sh.item.Path == "" {
		s, err := st.Get(ctx, sh.item.ID)
		if err != nil {
			return core.NilID, err
		}
		s.Path, s.Virtual = sh.series.Path, false
		if err := st.Upsert(ctx, s); err != nil {
			return core.NilID, err
		}
		sh.item = s
	}
	if season, ok := sh.seasons[episodes[0].SeasonNumber]; ok && season.Virtual && sh.wanted.SeasonFolder {
		s, err := st.Get(ctx, season.ID)
		if err != nil {
			return core.NilID, err
		}
		s.Path, s.Virtual = path.Dir(imported), false
		if err := st.Upsert(ctx, s); err != nil {
			return core.NilID, err
		}
	}
	return it.ID, nil
}

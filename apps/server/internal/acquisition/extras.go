package acquisition

import (
	"cmp"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/naming"
	"github.com/mavioai/mavio/libs/release"
)

// isSubtitleExtension reports whether an extension, with its dot, is one
// of a subtitle file, as Radarr's SubtitleFileExtensions.
func isSubtitleExtension(ext string) bool {
	switch strings.ToLower(ext) {
	case ".aqt", ".ass", ".idx", ".jss", ".psb", ".rt", ".smi", ".srt", ".ssa", ".sub", ".sup", ".txt", ".utf", ".utf8", ".utf-8", ".vtt":
		return true
	}
	return false
}

// wantedExtensions returns the set of extra file extensions, with their
// dots and in lower case, of the extensions setting.
func wantedExtensions(extensions []string) map[string]bool {
	wanted := map[string]bool{}
	for _, e := range extensions {
		if e = strings.ToLower(strings.Trim(strings.TrimSpace(e), ".")); e != "" {
			wanted["."+e] = true
		}
	}
	return wanted
}

// importMovieExtras imports the extra files of an imported video whose
// extensions are wanted, as Radarr's ExtraService.ImportMovie with its
// SubtitleService and OtherExtraService: the files of the video's folder,
// and of those below it when the download is a folder, named as the video
// or as its movie, or any when the video is the folder's only one besides
// a sample. Subtitles are named as the imported video with their language,
// tags and, among those alike, a copy number, such as ".1.en.forced.srt";
// other extras keep what follows the video's name in theirs. It returns
// the extras imported, for recordExtras. Failures are logged.
func (r *Runtime) importMovieExtras(ctx context.Context, root *os.Root, m *movie, f importFile, rel string, mode transferMode, extensions []string) []core.ExtraFile {
	parsed := release.ParseMoviePath(f.path)
	sameMovie := func(p string) bool {
		if parsed == nil || parsed.PrimaryMovieTitle() == "" {
			return false
		}
		info := release.ParseMoviePath(p)
		return info != nil && info.Year == parsed.Year && strings.EqualFold(info.PrimaryMovieTitle(), parsed.PrimaryMovieTitle())
	}
	isSample := func(p string) bool { return r.isSample(ctx, p, m.acq.Runtime) }
	return r.importExtras(ctx, root, f.path, f.inFolder, rel, mode, extensions, release.Movie, sameMovie, isSample)
}

// importEpisodeExtras imports the extra files of an imported episode
// file, as Sonarr's ExtraService.ImportEpisode with its SubtitleService
// and OtherExtraService: as importMovieExtras, but the files below the
// video's folder are looked at when the download's folder name parsed,
// and extras match by the season and episodes their names hold.
func (r *Runtime) importEpisodeExtras(ctx context.Context, root *os.Root, sh *show, le *localEpisode, recurse bool, rel string, mode transferMode, extensions []string) []core.ExtraFile {
	fileInfo := release.ParseSeriesPath(le.file.path)
	sameEpisodes := func(p string) bool {
		if fileInfo == nil {
			return false
		}
		info := release.ParseSeriesPath(p)
		return info != nil && len(info.EpisodeNumbers) > 0 &&
			info.SeasonNumber == fileInfo.SeasonNumber && slices.Equal(info.EpisodeNumbers, fileInfo.EpisodeNumbers)
	}
	isSample := func(p string) bool { return r.isSample(ctx, p, cmp.Or(sh.series.Runtime, 45)) }
	return r.importExtras(ctx, root, le.file.path, recurse, rel, mode, extensions, release.Series, sameEpisodes, isSample)
}

// importExtras imports the extras of an imported video to beside rel:
// those named as the video or that matches accepts, or any when the
// video is its folder's only one besides a sample; recurse looks below
// the video's folder too. It returns the extras imported, without their
// item.
func (r *Runtime) importExtras(ctx context.Context, root *os.Root, video string, recurse bool, rel string, mode transferMode, extensions []string,
	kind release.Kind, matches, isSample func(path string) bool,
) []core.ExtraFile {
	wanted := wantedExtensions(extensions)
	if len(wanted) == 0 {
		return nil
	}
	files := r.extraFiles(ctx, video, recurse, wanted, matches, isSample)
	destBase := strings.TrimSuffix(rel, filepath.Ext(rel))
	var out []core.ExtraFile
	for _, e := range nameExtras(video, files, kind) {
		dest := destBase + e.suffix
		if err := transfer(root, e.src, dest, mode); err != nil {
			r.logger().WarnContext(ctx, "couldn't import extra file", "path", e.src, "err", err)
			continue
		}
		out = append(out, core.ExtraFile{
			Kind: e.kind, Path: filepath.ToSlash(filepath.Join(root.Name(), dest)), Extension: strings.ToLower(filepath.Ext(dest)),
			Language: e.language, LanguageTags: e.tags,
		})
	}
	return out
}

// recordExtras records the extras imported with the file of a movie or
// episode item, as Radarr's and Sonarr's ExtraFileManager.ImportFiles
// add their SubtitleFile and OtherExtraFile rows. Failures are logged.
func (r *Runtime) recordExtras(ctx context.Context, itemID core.ID, extras []core.ExtraFile) {
	now := r.now()
	for i := range extras {
		e := &extras[i]
		e.ItemID, e.Added, e.LastUpdated = itemID, now, now
		if err := r.Store.ExtraFiles().Put(ctx, e); err != nil {
			r.logger().WarnContext(ctx, "couldn't record extra file", "path", e.Path, "err", err)
		}
	}
}

// extraFiles lists the extra files of an imported video to import, as
// Radarr's and Sonarr's SubtitleService.ImportFiles match them: those
// named as the video, those matches accepts, or any when the video is
// its folder's only one besides a sample.
func (r *Runtime) extraFiles(ctx context.Context, video string, recurse bool, wanted map[string]bool, matches, isSample func(path string) bool) []string {
	dir := filepath.Dir(video)
	parser := naming.Default()
	var candidates, videos []string
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if p != dir && (!recurse || strings.HasPrefix(name, ".")) {
				return fs.SkipDir
			}
			return nil
		}
		switch {
		case strings.HasPrefix(name, "."):
		case parser.IsVideoFile(p):
			videos = append(videos, p)
		case wanted[strings.ToLower(filepath.Ext(name))]:
			candidates = append(candidates, p)
		}
		return nil
	})
	if err != nil {
		r.logger().WarnContext(ctx, "couldn't list extra files", "path", dir, "err", err)
		return nil
	}

	videoBase := strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))
	var matching []string
	for _, p := range candidates {
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if hasPrefixFold(stem, videoBase) || matches(p) {
			matching = append(matching, p)
		}
	}
	// Any extra when the video is the folder's only one but for a sample.
	if len(matching) == 0 && len(candidates) > 0 && len(videos) <= 2 {
		videos = slices.DeleteFunc(videos, func(p string) bool { return p != video && isSample(p) })
		if len(videos) == 1 {
			r.logger().WarnContext(ctx, "importing any extra file of the video's folder", "video", video, "path", dir)
			matching = candidates
		}
	}
	return matching
}

// hasPrefixFold reports whether s begins with prefix, ignoring case.
func hasPrefixFold(s, prefix string) bool {
	return len(s) >= len(prefix) && strings.EqualFold(s[:len(prefix)], prefix)
}

// extraName is an extra file to import and what follows the imported
// video's name, without its extension, in its name; a subtitle's language
// name, unless unknown, and tags.
type extraName struct {
	src, suffix string
	kind        core.ExtraFileKind
	language    string
	tags        []string
}

// nameExtras names the extra files of a video. Subtitles alike in
// language, tags and extension are numbered, as Radarr's
// SubtitleService.ImportFiles and GetSuffix.
func nameExtras(video string, files []string, kind release.Kind) []extraName {
	videoBase := strings.TrimSuffix(filepath.Base(video), filepath.Ext(video))
	type subtitle struct {
		src, ext string
		language release.Language
		tags     []string
	}
	var (
		out    []extraName
		keys   []string
		groups = map[string][]subtitle{}
	)
	for _, p := range files {
		name := filepath.Base(p)
		ext := filepath.Ext(name)
		if !isSubtitleExtension(ext) {
			stem := strings.TrimSuffix(name, ext)
			suffix := ""
			if len(stem) > len(videoBase) && hasPrefixFold(stem, videoBase) {
				suffix = stem[len(videoBase):]
			}
			out = append(out, extraName{src: p, suffix: suffix + ext, kind: core.ExtraFileOther})
			continue
		}
		s := subtitle{src: p, ext: ext, language: release.ParseSubtitleLanguage(p, kind), tags: release.ParseLanguageTags(p)}
		key := s.language.String() + "\x00" + strings.Join(s.tags, ".") + "\x00" + ext
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], s)
	}
	for _, key := range keys {
		group := groups[key]
		for i, s := range group {
			e := extraName{
				src: s.src, suffix: subtitleSuffix(s.language, kind, i+1, s.tags, len(group) > 1) + s.ext,
				kind: core.ExtraFileSubtitle, tags: s.tags,
			}
			if s.language != release.LanguageUnknown {
				e.language = s.language.String()
			}
			out = append(out, e)
		}
	}
	return out
}

// subtitleSuffix returns what follows a video's name in that of a
// subtitle file, as Radarr's SubtitleService.GetSuffix: a copy number when
// several are alike, the language's two-letter code unless it is unknown,
// and the tags.
func subtitleSuffix(language release.Language, kind release.Kind, copyNumber int, tags []string, multipleCopies bool) string {
	var b strings.Builder
	if multipleCopies {
		b.WriteString("." + strconv.Itoa(copyNumber))
	}
	if language != release.LanguageUnknown {
		if iso, ok := release.IsoLanguageOf(language, kind); ok && iso.TwoLetterCode != "" {
			b.WriteString("." + iso.TwoLetterCode)
		}
	}
	if len(tags) > 0 {
		b.WriteString("." + strings.Join(tags, "."))
	}
	return b.String()
}

// recycleExtras recycles the extra files recorded with the file of a
// movie or episode item an upgrade replaces, and deletes their records,
// as Radarr's and Sonarr's ExtraFileService do on MovieFileDeletedEvent
// and EpisodeFileDeletedEvent; extras not recorded stay. Failures are
// logged.
func (r *Runtime) recycleExtras(ctx context.Context, itemID core.ID, sub string) {
	extras, err := r.Store.ExtraFiles().List(ctx, itemID)
	if err != nil {
		r.logger().WarnContext(ctx, "couldn't list extra files of replaced file", "item", itemID, "err", err)
		return
	}
	var gone []core.ID
	for _, e := range extras {
		if err := r.recycle(ctx, filepath.FromSlash(e.Path), sub); err != nil {
			r.logger().WarnContext(ctx, "couldn't recycle extra file of replaced file", "path", e.Path, "err", err)
			continue
		}
		gone = append(gone, e.ID)
	}
	if err := r.Store.ExtraFiles().Delete(ctx, gone...); err != nil {
		r.logger().WarnContext(ctx, "couldn't delete records of recycled extra files", "item", itemID, "err", err)
	}
}

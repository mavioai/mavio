package acquisition

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

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
// other extras keep what follows the video's name in theirs. Failures are
// logged.
func (r *Runtime) importMovieExtras(ctx context.Context, root *os.Root, m *movie, f importFile, rel string, mode transferMode, extensions []string) {
	wanted := wantedExtensions(extensions)
	if len(wanted) == 0 {
		return
	}
	files := r.extraFiles(ctx, m, f, wanted)
	destBase := strings.TrimSuffix(rel, filepath.Ext(rel))
	for _, e := range nameExtras(f.path, files) {
		if err := transfer(root, e.src, destBase+e.suffix, mode); err != nil {
			r.logger().WarnContext(ctx, "couldn't import extra file", "path", e.src, "err", err)
		}
	}
}

// extraFiles lists the extra files of an imported video to import, as
// Radarr's SubtitleService.ImportFiles matches them.
func (r *Runtime) extraFiles(ctx context.Context, m *movie, f importFile, wanted map[string]bool) []string {
	dir := filepath.Dir(f.path)
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
			if p != dir && (!f.inFolder || strings.HasPrefix(name, ".")) {
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

	videoBase := strings.TrimSuffix(filepath.Base(f.path), filepath.Ext(f.path))
	parsed := release.ParseMoviePath(f.path)
	var matching []string
	for _, p := range candidates {
		stem := strings.TrimSuffix(filepath.Base(p), filepath.Ext(p))
		if hasPrefixFold(stem, videoBase) {
			matching = append(matching, p)
			continue
		}
		if parsed == nil || parsed.PrimaryMovieTitle() == "" {
			continue
		}
		if info := release.ParseMoviePath(p); info != nil && info.Year == parsed.Year &&
			strings.EqualFold(info.PrimaryMovieTitle(), parsed.PrimaryMovieTitle()) {
			matching = append(matching, p)
		}
	}
	// Any extra when the video is the folder's only one but for a sample.
	if len(matching) == 0 && len(candidates) > 0 && len(videos) <= 2 {
		videos = slices.DeleteFunc(videos, func(p string) bool { return p != f.path && r.isSample(ctx, m, p) })
		if len(videos) == 1 {
			r.logger().WarnContext(ctx, "importing any extra file of the movie's folder", "item", m.item.ID, "path", dir)
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
// video's name, without its extension, in its name.
type extraName struct {
	src, suffix string
}

// nameExtras names the extra files of a video. Subtitles alike in
// language, tags and extension are numbered, as Radarr's
// SubtitleService.ImportFiles and GetSuffix.
func nameExtras(video string, files []string) []extraName {
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
			out = append(out, extraName{src: p, suffix: suffix + ext})
			continue
		}
		s := subtitle{src: p, ext: ext, language: release.ParseSubtitleLanguage(p, release.Movie), tags: release.ParseLanguageTags(p)}
		key := s.language.String() + "\x00" + strings.Join(s.tags, ".") + "\x00" + ext
		if _, ok := groups[key]; !ok {
			keys = append(keys, key)
		}
		groups[key] = append(groups[key], s)
	}
	for _, key := range keys {
		group := groups[key]
		for i, s := range group {
			out = append(out, extraName{src: s.src, suffix: subtitleSuffix(s.language, i+1, s.tags, len(group) > 1) + s.ext})
		}
	}
	return out
}

// subtitleSuffix returns what follows a video's name in that of a
// subtitle file, as Radarr's SubtitleService.GetSuffix: a copy number when
// several are alike, the language's two-letter code unless it is unknown,
// and the tags.
func subtitleSuffix(language release.Language, copyNumber int, tags []string, multipleCopies bool) string {
	var b strings.Builder
	if multipleCopies {
		b.WriteString("." + strconv.Itoa(copyNumber))
	}
	if language != release.LanguageUnknown {
		if iso, ok := release.IsoLanguageOf(language, release.Movie); ok && iso.TwoLetterCode != "" {
			b.WriteString("." + iso.TwoLetterCode)
		}
	}
	if len(tags) > 0 {
		b.WriteString("." + strings.Join(tags, "."))
	}
	return b.String()
}

// recycleExtras recycles the extra files of a movie file an upgrade
// replaces, as Radarr's ExtraService does on MovieFileDeletedEvent: the
// subtitles and the files with wanted extensions of its folder named as it
// with a suffix. Failures are logged.
func (r *Runtime) recycleExtras(ctx context.Context, old, sub string, extensions []string) {
	wanted := wantedExtensions(extensions)
	dir := filepath.Dir(old)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			r.logger().WarnContext(ctx, "couldn't list extra files of replaced file", "path", dir, "err", err)
		}
		return
	}
	oldBase := strings.TrimSuffix(filepath.Base(old), filepath.Ext(old))
	parser := naming.Default()
	for _, e := range entries {
		name := e.Name()
		ext := filepath.Ext(name)
		if e.IsDir() || name == filepath.Base(old) || parser.IsVideoFile(name) ||
			(!isSubtitleExtension(ext) && !wanted[strings.ToLower(ext)]) ||
			!hasPrefixFold(name, oldBase+".") {
			continue
		}
		p := filepath.Join(dir, name)
		if err := r.recycle(ctx, p, sub); err != nil {
			r.logger().WarnContext(ctx, "couldn't recycle extra file of replaced file", "path", p, "err", err)
		}
	}
}

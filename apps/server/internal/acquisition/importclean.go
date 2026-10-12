package acquisition

import (
	"cmp"
	"context"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/naming"
	"github.com/mavioai/mavio/libs/release"
)

// rarLeftOver is the size from which a RAR file left in a download folder
// keeps it, for the user to clean up.
const rarLeftOver = 10 << 20

// cleanDownloadFolder deletes a download's folder that an import moved
// its movie out of, when nothing but samples and small RAR files are
// left, as Radarr's DownloadedMovieImportService.ProcessFolder and
// ShouldDeleteFolder.
func (r *Runtime) cleanDownloadFolder(ctx context.Context, m *movie, folder string, mappings []acquire.RemotePathMapping) {
	r.deleteDownloadFolder(ctx, folder, mappings, func(p string) bool {
		return release.ParseMovieTitle(filepath.Base(p), false) != nil && r.isSample(ctx, p, m.acq.Runtime)
	})
}

// cleanEpisodeDownloadFolder deletes a download's folder that an import
// moved episodes of a series out of, when nothing but samples and small
// RAR files are left, as Sonarr's
// DownloadedEpisodesImportService.ProcessFolder and ShouldDeleteFolder:
// a video whose name does not parse, or that may be a special, keeps it.
func (r *Runtime) cleanEpisodeDownloadFolder(ctx context.Context, sh *show, folder string, mappings []acquire.RemotePathMapping) {
	r.deleteDownloadFolder(ctx, folder, mappings, func(p string) bool {
		info := release.ParseSeriesTitle(filepath.Base(p))
		if info == nil || info.IsPossibleSpecialEpisode() {
			return false
		}
		return r.isSample(ctx, p, cmp.Or(sh.series.Runtime, 45))
	})
}

// deleteDownloadFolder deletes a download's folder when every video file
// left in it is a sample and no RAR file of 10 MiB or more is. Folders of
// libraries and of remote path mappings, and those holding them, and
// folders in libraries are never deleted. Failures are logged.
func (r *Runtime) deleteDownloadFolder(ctx context.Context, folder string, mappings []acquire.RemotePathMapping, sample func(path string) bool) {
	if fi, err := r.statDownload(ctx, folder); err != nil || !fi.IsDir() {
		return
	}
	for _, mp := range mappings {
		if p := strings.TrimSpace(mp.LocalPath); p != "" && within(p, folder) {
			return
		}
	}
	libs, err := r.Store.Libraries().List(ctx)
	if err != nil {
		r.logger().WarnContext(ctx, "couldn't list libraries before deleting download folder", "path", folder, "err", err)
		return
	}
	for _, lib := range libs {
		for _, p := range lib.Paths {
			if p = strings.TrimSpace(p); p != "" && (within(folder, p) || within(p, folder)) {
				return
			}
		}
	}
	if !r.onlySamplesLeft(ctx, folder, sample) {
		return
	}
	if err := r.removeDownload(ctx, folder); err != nil {
		r.logger().WarnContext(ctx, "couldn't delete download folder after importing", "path", folder, "err", err)
		return
	}
	r.logger().InfoContext(ctx, "deleted download folder after importing", "path", folder)
}

// within reports whether path is dir or below it.
func within(path, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(dir), filepath.Clean(path))
	return err == nil && (rel == "." || filepath.IsLocal(rel))
}

// onlySamplesLeft reports whether every video file left in a folder is a
// sample, and no RAR file of 10 MiB or more is left.
func (r *Runtime) onlySamplesLeft(ctx context.Context, folder string, sample func(path string) bool) bool {
	parser := naming.Default()
	ok := true
	err := r.walkDownload(ctx, folder, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".rar") {
			if fi, err := d.Info(); err != nil || fi.Size() > rarLeftOver {
				ok = false
				return fs.SkipAll
			}
			return nil
		}
		if !parser.IsVideoFile(p) {
			return nil
		}
		if !sample(p) {
			ok = false
			return fs.SkipAll
		}
		return nil
	})
	return err == nil && ok
}

// isSample reports whether a video file is a sample of a title of a
// runtime in minutes, as Radarr's and Sonarr's DetectSample.
// probe probes a file to import.
func (r *Runtime) probe(ctx context.Context, path string) (library.ProbeResult, error) {
	f, err := r.Storages.Media(ctx, path)
	if err != nil {
		return library.ProbeResult{}, err
	}
	defer f.Close()
	return r.Prober.Probe(ctx, f, false)
}

func (r *Runtime) isSample(ctx context.Context, path string, runtime int) bool {
	if r.Prober == nil {
		return false
	}
	done, err := r.pace(ctx, path)
	if err != nil {
		return false
	}
	res, err := r.probe(ctx, path)
	done()
	if err != nil {
		return false
	}
	return acquire.IsSample(path, &res.Source.Duration, runtime) == acquire.SampleYes
}

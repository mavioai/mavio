package acquisition

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/naming"
	"github.com/mavioai/mavio/libs/release"
)

// rarLeftOver is the size from which a RAR file left in a download folder
// keeps it, for the user to clean up.
const rarLeftOver = 10 << 20

// cleanDownloadFolder deletes a download's folder that an import moved
// its movie out of, when nothing but samples and small RAR files are
// left, as Radarr's DownloadedMovieImportService.ProcessFolder and
// ShouldDeleteFolder. Folders of libraries and of remote path mappings,
// and those holding them, and folders in libraries are never deleted.
// Failures are logged.
func (r *Runtime) cleanDownloadFolder(ctx context.Context, m *movie, folder string, mappings []acquire.RemotePathMapping) {
	if fi, err := os.Stat(folder); err != nil || !fi.IsDir() {
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
	if !r.onlySamplesLeft(ctx, m, folder) {
		return
	}
	if err := os.RemoveAll(folder); err != nil {
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
// sample of the movie, and no RAR file of 10 MiB or more is left.
func (r *Runtime) onlySamplesLeft(ctx context.Context, m *movie, folder string) bool {
	parser := naming.Default()
	ok := true
	err := filepath.WalkDir(folder, func(p string, d fs.DirEntry, err error) error {
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
		if release.ParseMovieTitle(filepath.Base(p), false) == nil || !r.isSample(ctx, m, p) {
			ok = false
			return fs.SkipAll
		}
		return nil
	})
	return err == nil && ok
}

// isSample reports whether a video file is a sample of the movie by its
// runtime, as Radarr's DetectSample.
func (r *Runtime) isSample(ctx context.Context, m *movie, path string) bool {
	if r.Prober == nil {
		return false
	}
	done, err := r.pace(ctx, path)
	if err != nil {
		return false
	}
	res, err := r.Prober.Probe(ctx, path, false)
	done()
	if err != nil {
		return false
	}
	return acquire.IsSample(path, &res.Source.Duration, m.acq.Runtime) == acquire.SampleYes
}

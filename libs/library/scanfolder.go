package library

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
)

// JobScanFolder scans one folder of a library, such as one a file was
// just written to; the payload is a FolderPayload.
const JobScanFolder = "library.scan_folder"

// FolderPayload is the payload of JobScanFolder jobs.
type FolderPayload struct {
	LibraryID core.ID `json:"library_id"`
	// Path is the folder, in one of the library's folders.
	Path string `json:"path"`
}

// ScanFolderJob returns a job scanning the folder dir of lib at the given
// time.
func ScanFolderJob(lib core.Library, dir string, at time.Time) core.Job {
	dir = filepath.ToSlash(filepath.Clean(dir))
	payload, _ := json.Marshal(FolderPayload{LibraryID: lib.ID, Path: dir})
	return core.Job{
		ID: core.NewID(), Kind: JobScanFolder, Payload: payload, UniqueKey: JobScanFolder + ":" + lib.ID.String() + ":" + dir,
		MaxAttempts: 3, RunAt: at, Priority: 10,
	}
}

// ScanFolder reconciles one folder of lib, and the folders below it, with
// the disk, as Scan does for the whole library: the folders from the
// library folder down to it are scanned too, for its items to have their
// parents, but none beside them. Items whose files are gone are left for
// the next scan of the library to mark missing.
func (s *Scanner) ScanFolder(ctx context.Context, lib core.Library, dir string) (ScanStats, error) {
	dir = filepath.ToSlash(filepath.Clean(dir))
	base := ""
	for _, p := range lib.Paths {
		b := filepath.ToSlash(filepath.Clean(p))
		if dir == b || strings.HasPrefix(dir, strings.TrimSuffix(b, "/")+"/") {
			base = b
			break
		}
	}
	if base == "" {
		return ScanStats{}, fmt.Errorf("%w: %s is not in a folder of library %s", core.ErrInvalid, dir, lib.Name)
	}
	gen, err := s.Store.Scans().NextGeneration(ctx, lib.ID)
	if err != nil {
		return ScanStats{}, err
	}
	sc := &scan{Scanner: s, lib: lib, gen: gen, devices: s.Devices, stats: ScanStats{Generation: gen}}
	if sc.devices == nil {
		sc.devices = &storage.Detector{}
	}
	r, err := os.OpenRoot(filepath.FromSlash(base))
	if err != nil {
		return sc.stats, fmt.Errorf("library folder: %w", err)
	}
	defer r.Close()
	rfs := rootFS{root: r, base: base}
	ignores := &IgnoreFiles{FS: r.FS()}
	t := task{dir: base, scope: Scope{Kind: lib.Kind, Root: base}}
	for t.dir != dir {
		subs, err := sc.folder(ctx, rfs, ignores, t)
		if err != nil {
			return sc.stats, err
		}
		i := slices.IndexFunc(subs, func(sub task) bool { return sub.dir == dir || strings.HasPrefix(dir, sub.dir+"/") })
		if i < 0 {
			// The folder was resolved with the one above, or is ignored.
			s.logger().InfoContext(ctx, "library folder scanned", "library", lib.Name, "path", t.dir, "generation", gen, "saved", sc.stats.Saved)
			return sc.stats, nil
		}
		t = subs[i]
	}
	dev := sc.devices.Device(filepath.Join(filepath.FromSlash(dir), "."))
	if err := sc.walk(ctx, rfs, ignores, sc.walkers(dev, sc.policy(dir)), t); err != nil {
		return sc.stats, err
	}
	s.logger().InfoContext(ctx, "library folder scanned", "library", lib.Name, "path", dir, "generation", gen,
		"listed", sc.stats.Listed, "pruned", sc.stats.Pruned, "saved", sc.stats.Saved)
	return sc.stats, nil
}

// scanFolder runs a JobScanFolder job, apart from the library's other
// scans.
func (j *Jobs) scanFolder(ctx context.Context, job core.Job) ([]core.Job, error) {
	var p FolderPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("scan folder payload: %w", err)
	}
	lib, err := j.Store.Libraries().Get(ctx, p.LibraryID)
	if err != nil || lib.Kind.Curated() {
		return nil, ignoreGone(err)
	}
	lock := j.libraryLock(lib.ID)
	lock.Lock()
	defer lock.Unlock()
	_, err = j.Scanner.ScanFolder(ctx, lib, p.Path)
	return nil, err
}

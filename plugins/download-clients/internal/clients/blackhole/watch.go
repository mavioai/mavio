package blackhole

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/plugins/download-clients/internal/client"
	"github.com/mavioai/mavio/plugins/download-clients/internal/clients/blackhole/disk"
)

// watchItem is a download found in a watch folder, as Radarr's
// WatchFolderItem.
type watchItem struct {
	ID            string
	Title         string
	TotalSize     int64
	RemainingTime *time.Duration
	OutputPath    string
	Status        client.Status

	// LastChanged is when Hash last changed; zero once the item was left
	// unchanged for an hour and is no longer hashed.
	LastChanged time.Time
	Hash        string
}

// The times of Radarr's ScanWatchFolder: how long the items of a scan are
// kept for the next, and after how long unchanged an item is no longer
// read again.
const (
	cacheTTL     = 5 * time.Minute
	stableExpiry = time.Hour
)

// scanner lists the downloads in watch folders, as Radarr's
// ScanWatchFolder: each folder and video file of a watch folder is one,
// still downloading while its files are locked or until they are left
// unchanged for a wait period.
type scanner struct {
	now func() time.Time
	log *slog.Logger
	// locked reports whether another process holds a file.
	locked func(path string) bool

	mu    sync.Mutex
	cache map[string]scan
}

// scan is the items of a watch folder's last scan.
type scan struct {
	items   map[string]watchItem
	expires time.Time
}

func newScanner(env client.Env) *scanner {
	return &scanner{now: env.Now, log: env.Log, locked: disk.IsLocked, cache: map[string]scan{}}
}

// The files and folders Radarr's DiskScanService.FilterPaths leaves out,
// by name; its patterns of subfolders never match the entries of the watch
// folder itself.
var (
	excludedFiles      = regexp.MustCompile(`(?i)^\.(_|unmanic|DS_Store$)|^Thumbs\.db$`)
	excludedExtraFiles = regexp.MustCompile(`(?i)(-(trailer|other|behindthescenes|deleted|featurette|interview|scene|short)\.[^.]+$)`)
)

// specialFolders are the folders systems keep, as Radarr's SpecialFolders.
var specialFolders = []string{
	"boot", "bootmgr", "cache", "msocache", "recovery", "$recycle.bin", "recycler",
	"system volume information", "temporary internet files", "windows",
	".fseventd", ".spotlight", ".trashes", ".vol", "cachedmessages", "caches", "trash",
	".@__thumb",
	"@eadir", "#recycle",
}

// videoExtensions are Radarr's MediaFileExtensions.
var videoExtensions = []string{
	".webm", ".m4v", ".3gp", ".nsv", ".ty", ".strm", ".rm", ".rmvb", ".m3u", ".ifo", ".mov", ".qt",
	".divx", ".xvid", ".bivx", ".nrg", ".pva", ".wmv", ".asf", ".asx", ".ogm", ".ogv", ".m2v", ".avi",
	".bin", ".dat", ".dvr-ms", ".mpg", ".mpeg", ".mp4", ".avc", ".vp3", ".svq3", ".nuv", ".viv", ".dv",
	".fli", ".flv", ".wpl", ".img", ".iso", ".vob", ".mkv", ".mk3d", ".ts", ".wtv", ".m2ts",
}

func filtered(name string) bool {
	return excludedFiles.MatchString(name) || excludedExtraFiles.MatchString(name)
}

func isVideo(name string) bool {
	return slices.Contains(videoExtensions, strings.ToLower(filepath.Ext(name)))
}

// items lists the downloads in a watch folder; those changed within wait
// are still downloading.
func (s *scanner) items(ctx context.Context, watchFolder string, wait time.Duration) ([]watchItem, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	last := map[string]watchItem{}
	if c, ok := s.cache[watchFolder]; ok && c.expires.After(s.now()) {
		last = c.items
	}
	root, err := os.OpenRoot(watchFolder)
	if err != nil {
		return nil, fmt.Errorf("open watch folder: %w", err)
	}
	defer func() { _ = root.Close() }()
	entries, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return nil, fmt.Errorf("read watch folder: %w", err)
	}
	var folders, files []string
	for _, e := range entries {
		name := e.Name()
		fi, err := root.Stat(name)
		if err != nil || filtered(name) {
			continue
		}
		switch {
		case fi.IsDir():
			if !slices.Contains(specialFolders, strings.ToLower(name)) {
				folders = append(folders, name)
			}
		case fi.Mode().IsRegular() && isVideo(name):
			files = append(files, name)
		}
	}

	var out []watchItem
	next := map[string]watchItem{}
	add := func(it watchItem) {
		out = append(out, it)
		next[it.ID] = it
	}
	for _, name := range folders {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		fi, err := root.Stat(name)
		if err != nil {
			continue
		}
		it := s.newItem(watchFolder, name, disk.CreationTime(fi))
		old, ok := last[it.ID]
		if s.preCheck(&it, old, ok) {
			contents := s.folderFiles(ctx, root, name)
			for _, f := range contents {
				it.TotalSize += f.Size()
			}
			it.Hash = s.folderHash(name, fi, contents)
			if slices.ContainsFunc(contents, func(f fileInfo) bool { return s.locked(filepath.Join(watchFolder, filepath.FromSlash(f.path))) }) {
				it.Status = client.Downloading
				it.RemainingTime = nil
			}
			s.updateExpiry(&it, old, ok, wait)
		}
		add(it)
	}
	for _, name := range files {
		fi, err := root.Stat(name)
		if err != nil {
			continue
		}
		it := s.newItem(watchFolder, name, fi.ModTime())
		old, ok := last[it.ID]
		if s.preCheck(&it, old, ok) {
			it.TotalSize = fi.Size()
			it.Hash = fileHash(name, fi)
			if s.locked(it.OutputPath) {
				it.Status = client.Downloading
			}
			s.updateExpiry(&it, old, ok, wait)
		}
		add(it)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s.cache[watchFolder] = scan{items: next, expires: s.now().Add(cacheTTL)}
	return out, nil
}

// newItem is a completed download of a folder or file of the watch
// folder, its ID the name and the time as Radarr's ticks.
func (s *scanner) newItem(watchFolder, name string, t time.Time) watchItem {
	zero := time.Duration(0)
	return watchItem{
		ID:            name + "_" + strconv.FormatInt(disk.Ticks(t), 10),
		Title:         client.CleanFileName(name),
		OutputPath:    filepath.Join(watchFolder, name),
		Status:        client.Completed,
		RemainingTime: &zero,
	}
}

// preCheck reports whether an item is to be read again: one new, or
// changed within the hour; otherwise it keeps what was read of it.
func (s *scanner) preCheck(it *watchItem, old watchItem, ok bool) bool {
	if !ok || old.LastChanged.Add(stableExpiry).After(s.now()) {
		return true
	}
	it.TotalSize, it.Hash = old.TotalSize, old.Hash
	return false
}

// updateExpiry keeps when an unchanged item last changed, and holds an
// item changed within wait as downloading for the rest of it.
func (s *scanner) updateExpiry(it *watchItem, old watchItem, ok bool, wait time.Duration) {
	now := s.now()
	if ok && it.Hash == old.Hash {
		it.LastChanged = old.LastChanged
	} else {
		it.LastChanged = now
	}
	if remaining := wait - now.Sub(it.LastChanged); remaining > 0 {
		it.RemainingTime = &remaining
		it.Status = client.Downloading
	}
}

// fileInfo is a file of a folder, by its slash-separated path in the
// watch folder.
type fileInfo struct {
	fs.FileInfo
	path string
}

// folderFiles returns the files in a folder and its subfolders; those
// that cannot be read are left out.
func (s *scanner) folderFiles(ctx context.Context, root *os.Root, folder string) []fileInfo {
	var files []fileInfo
	_ = fs.WalkDir(root.FS(), folder, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			s.log.DebugContext(ctx, "skipping unreadable path in watch folder", "path", p, "err", err)
			return nil
		}
		if d.IsDir() {
			return ctx.Err()
		}
		fi, err := root.Stat(filepath.FromSlash(p))
		if err != nil || !fi.Mode().IsRegular() {
			return nil
		}
		files = append(files, fileInfo{FileInfo: fi, path: p})
		return nil
	})
	slices.SortFunc(files, func(a, b fileInfo) int { return strings.Compare(a.path, b.path) })
	return files
}

// folderHash hashes a folder by its path, its last write and those of
// its files, as Radarr's GetHash.
func (s *scanner) folderHash(name string, fi fs.FileInfo, files []fileInfo) string {
	lastWrite := fi.ModTime()
	if len(files) > 0 {
		lastWrite = time.Time{}
		for _, f := range files {
			if f.ModTime().After(lastWrite) {
				lastWrite = f.ModTime()
			}
		}
	}
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s%d", name, lastWrite.UnixNano())
	for _, f := range files {
		h.Write([]byte(fileHash(f.path, f)))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// fileHash hashes a file by its path, last write and size.
func fileHash(p string, fi fs.FileInfo) string {
	sum := sha256.Sum256(fmt.Appendf(nil, "%s%d%d", path.Clean(p), fi.ModTime().UnixNano(), fi.Size()))
	return hex.EncodeToString(sum[:])
}

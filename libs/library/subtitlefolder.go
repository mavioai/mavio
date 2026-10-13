package library

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"slices"
	"strings"

	"github.com/mavioai/mavio/libs/core"
)

// SubtitleFolder is where subtitle files are saved and found besides
// beside their videos, as core.SubtitleSettings' Folder and CustomFolder.
// The zero value keeps them beside their videos only.
type SubtitleFolder struct {
	Kind core.SubtitleFolder
	// Custom is the name of the folder within each video's folder for
	// core.SubtitleFolderRelative, and the folder, a local path or a
	// remote path, for core.SubtitleFolderAbsolute.
	Custom string
}

// relative returns the name of the subtitle folder within videos'
// folders, empty when there is none.
func (f SubtitleFolder) relative() string {
	if f.Kind == core.SubtitleFolderRelative {
		return f.Custom
	}
	return ""
}

// absolute returns the absolute subtitle folder, empty when there is
// none.
func (f SubtitleFolder) absolute() string {
	if f.Kind == core.SubtitleFolderAbsolute && f.Custom != "" {
		return cleanRoot(f.Custom)
	}
	return ""
}

// dirs returns the folders the subtitle files of a video are found in:
// the video's folder, then the subtitle folder.
func (f SubtitleFolder) dirs(video string) []string {
	dir := path.Dir(video)
	switch {
	case f.relative() != "":
		return []string{dir, path.Join(dir, f.relative())}
	case f.absolute() != "":
		return []string{dir, f.absolute()}
	}
	return []string{dir}
}

// sidecarIndex holds the files of the absolute subtitle folder for a
// scan, by lowercase name, so that each video finds those named after it
// without going through all of them.
type sidecarIndex struct {
	dir   string
	names []string // lowercase, sorted
	files map[string][]string
}

// readSidecarIndex lists the absolute subtitle folder dir.
func readSidecarIndex(ctx context.Context, s *Storages, dir string) (*sidecarIndex, error) {
	st, err := s.Open(ctx, dir)
	if err != nil {
		return nil, err
	}
	defer st.Close()
	des, err := st.ReadDir(ctx, ".")
	if err != nil {
		return nil, err
	}
	x := &sidecarIndex{dir: dir, files: map[string][]string{}}
	for _, de := range des {
		if de.IsDir() {
			continue
		}
		lower := strings.ToLower(de.Name())
		if _, ok := x.files[lower]; !ok {
			x.names = append(x.names, lower)
		}
		x.files[lower] = append(x.files[lower], de.Name())
	}
	slices.Sort(x.names)
	return x, nil
}

// add adds the files named after video's to entries.
func (x *sidecarIndex) add(video string, entries map[string]core.FolderEntry) {
	if x == nil {
		return
	}
	stem := strings.ToLower(fileNameWithoutExt(video))
	i, _ := slices.BinarySearch(x.names, stem)
	for ; i < len(x.names) && strings.HasPrefix(x.names[i], stem); i++ {
		for _, name := range x.files[x.names[i]] {
			entries[path.Join(x.dir, name)] = core.FolderEntry{Name: name}
		}
	}
}

// readSidecarDirs lists the folders a video's subtitle files are found
// in, as sidecars takes them; the absolute folder only for the files
// named after the video. Missing folders are left out.
func (s *Subtitles) readSidecarDirs(ctx context.Context, lib core.Library, video string) (map[string]core.FolderEntry, error) {
	entries := map[string]core.FolderEntry{}
	folder := s.folder()
	for i, dir := range folder.dirs(video) {
		if i > 0 && dir == folder.absolute() {
			x, err := readSidecarIndex(ctx, s.Storages, dir)
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			if err != nil {
				return nil, err
			}
			x.add(video, entries)
			continue
		}
		root, name, ok := libraryRoot(lib, path.Join(dir, "x"))
		if !ok {
			continue
		}
		st, err := s.Storages.Open(ctx, root)
		if err != nil {
			return nil, err
		}
		des, err := st.ReadDir(ctx, path.Dir(name))
		_ = st.Close()
		if i > 0 && errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		for _, de := range des {
			entries[path.Join(dir, de.Name())] = core.FolderEntry{Name: de.Name(), IsDir: de.IsDir()}
		}
	}
	return entries, nil
}

// folder returns where subtitle files are saved and found.
func (s *Subtitles) folder() SubtitleFolder {
	if s.Folder == nil {
		return SubtitleFolder{}
	}
	return s.Folder()
}

package library

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"path/filepath"

	"github.com/mavioai/mavio/libs/core"
)

// SubtitleFolder is where subtitle files are saved and found besides
// beside their videos, as core.SubtitleSettings' Folder and CustomFolder.
// The zero value keeps them beside their videos only.
type SubtitleFolder struct {
	Kind core.SubtitleFolder
	// Relative is the name of the folder within each video's folder, for
	// core.SubtitleFolderRelative.
	Relative string
	// MetadataDir is the server's metadata folder, holding each item's
	// subtitles in its folder there, for core.SubtitleFolderMetadata.
	MetadataDir string
}

// relative returns the name of the subtitle folder within videos'
// folders, empty when there is none.
func (f SubtitleFolder) relative() string {
	if f.Kind == core.SubtitleFolderRelative {
		return f.Relative
	}
	return ""
}

// metadata returns the folder of an item's subtitles within the metadata
// folder, empty when there is none.
func (f SubtitleFolder) metadata(itemID core.ID) string {
	if f.Kind != core.SubtitleFolderMetadata || f.MetadataDir == "" {
		return ""
	}
	return SubtitleDir(f.MetadataDir, itemID)
}

// SubtitleDir returns the folder of an item's subtitles within the
// metadata folder, as a slash path.
func SubtitleDir(metadataDir string, itemID core.ID) string {
	return filepath.ToSlash(extrasFolder(metadataDir, itemID, "subtitles"))
}

// dirs returns the folders the subtitle files of an item's video are
// found in: the video's folder, then the subtitle folder.
func (f SubtitleFolder) dirs(video string, itemID core.ID) []string {
	dir := path.Dir(video)
	switch {
	case f.relative() != "":
		return []string{dir, path.Join(dir, f.relative())}
	case f.metadata(itemID) != "":
		return []string{dir, f.metadata(itemID)}
	}
	return []string{dir}
}

// readSidecarDir lists a folder of subtitle files outside the library
// into entries, as sidecars takes them; a missing folder adds nothing.
func readSidecarDir(ctx context.Context, s *Storages, dir string, entries map[string]core.FolderEntry) error {
	st, err := s.Open(ctx, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer st.Close()
	des, err := st.ReadDir(ctx, ".")
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, de := range des {
		entries[path.Join(dir, de.Name())] = core.FolderEntry{Name: de.Name(), IsDir: de.IsDir()}
	}
	return nil
}

// readSidecarDirs lists the folders the subtitle files of an item's video
// are found in, as sidecars takes them. Missing subtitle folders are left
// out.
func (s *Subtitles) readSidecarDirs(ctx context.Context, lib core.Library, itemID core.ID, video string) (map[string]core.FolderEntry, error) {
	entries := map[string]core.FolderEntry{}
	folder := s.folder()
	for i, dir := range folder.dirs(video, itemID) {
		root, name, ok := libraryRoot(lib, path.Join(dir, "x"))
		if !ok {
			if err := readSidecarDir(ctx, s.Storages, dir, entries); err != nil {
				return nil, err
			}
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

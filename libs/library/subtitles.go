package library

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/naming"
)

// Subtitles saves subtitle files of videos beside the videos,
// in a folder of theirs or in the metadata folder, where they become
// subtitle streams.
type Subtitles struct {
	Store core.Store
	// Storages opens the storage of library paths; nil opens local
	// folders only.
	Storages *Storages
	// Folder gives where subtitle files are saved and found; nil keeps
	// them beside their videos.
	Folder func() SubtitleFolder
}

// video returns an item of lib and one of its media sources, the first
// when sourceID is zero.
func (s *Subtitles) video(ctx context.Context, itemID, sourceID core.ID) (core.Item, []core.MediaSource, int, error) {
	it, err := s.Store.Items().Get(ctx, itemID)
	if err != nil {
		return it, nil, 0, err
	}
	sources, err := s.Store.MediaSources().ListForItem(ctx, itemID)
	if err != nil {
		return it, nil, 0, err
	}
	i := slices.IndexFunc(sources, func(src core.MediaSource) bool { return sourceID.IsZero() || src.ID == sourceID })
	if i < 0 || sources[i].Disc != "" {
		return it, nil, 0, fmt.Errorf("%w: no video file to find subtitles for", core.ErrNotFound)
	}
	return it, sources, i, nil
}

// Save writes a subtitle file of an item's video, beside it or in the
// subtitle folder, and adds it to the video's streams, which it returns.
// name names the file from stem, the video's name without its extension
// in the folder saved to, given the folder's files; both are relative to
// the folder's storage.
func (s *Subtitles) Save(ctx context.Context, lib core.Library, itemID, sourceID core.ID, name func(fsys fs.FS, stem string) (string, error), data []byte) (core.MediaStream, error) {
	_, sources, si, err := s.video(ctx, itemID, sourceID)
	if err != nil {
		return core.MediaStream{}, err
	}
	src := &sources[si]
	root, rel, ok := libraryRoot(lib, src.Path)
	if !ok {
		return core.MediaStream{}, fmt.Errorf("%w: %s is outside the library's folders", core.ErrInvalid, src.Path)
	}
	folder := s.folder()
	dir := path.Dir(rel)
	switch {
	case folder.relative() != "":
		dir = path.Join(dir, folder.relative())
	case folder.metadata(itemID) != "":
		root, dir = folder.metadata(itemID), "."
		if err := os.MkdirAll(filepath.FromSlash(root), 0o755); err != nil {
			return core.MediaStream{}, fmt.Errorf("save subtitle: %w", err)
		}
	}
	st, err := s.Storages.Open(ctx, root)
	if err != nil {
		return core.MediaStream{}, err
	}
	defer st.Close()
	stem := path.Join(dir, fileNameWithoutExt(rel))
	file, err := name(core.StorageFS(ctx, st), stem)
	if err != nil {
		return core.MediaStream{}, err
	}
	if path.Dir(file) != dir {
		return core.MediaStream{}, fmt.Errorf("%w: subtitle %s is not in the subtitle folder %s", core.ErrInvalid, file, dir)
	}
	if dir != path.Dir(rel) && dir != "." {
		if err := st.MkdirAll(ctx, dir); err != nil {
			return core.MediaStream{}, fmt.Errorf("save subtitle: %w", err)
		}
	}
	if err := st.WriteFile(ctx, file, data); err != nil {
		return core.MediaStream{}, fmt.Errorf("save subtitle: %w", err)
	}
	if err := s.refreshSidecars(ctx, lib, itemID, sources, si); err != nil {
		return core.MediaStream{}, err
	}
	saved := path.Join(root, file)
	for _, st := range src.Streams {
		if st.ExternalPath == saved {
			return st, nil
		}
	}
	return core.MediaStream{}, fmt.Errorf("saved subtitle %s is not named after its video", saved)
}

// Remove deletes a subtitle file of an item's video, the ExternalPath of
// one of its streams, and drops it from the video's streams.
func (s *Subtitles) Remove(ctx context.Context, lib core.Library, itemID core.ID, file string) error {
	sources, si, err := s.sourceWith(ctx, itemID, file)
	if err != nil {
		return err
	}
	st, err := s.Storages.Open(ctx, cleanRoot(path.Dir(file)))
	if err != nil {
		return err
	}
	err = st.Remove(ctx, path.Base(file))
	_ = st.Close()
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("delete subtitle: %w", err)
	}
	return s.refreshSidecars(ctx, lib, itemID, sources, si)
}

// sourceWith returns an item's media sources and which has the subtitle
// file.
func (s *Subtitles) sourceWith(ctx context.Context, itemID core.ID, file string) ([]core.MediaSource, int, error) {
	sources, err := s.Store.MediaSources().ListForItem(ctx, itemID)
	if err != nil {
		return nil, 0, err
	}
	for i, src := range sources {
		if slices.ContainsFunc(src.Streams, func(st core.MediaStream) bool { return st.ExternalPath == file }) {
			return sources, i, nil
		}
	}
	return nil, 0, fmt.Errorf("%w: %s is not a subtitle of item %s", core.ErrNotFound, file, itemID)
}

// ReadFile reads a subtitle file of an item's video, the ExternalPath of
// one of its streams.
func (s *Subtitles) ReadFile(ctx context.Context, itemID core.ID, file string) ([]byte, error) {
	if _, _, err := s.sourceWith(ctx, itemID, file); err != nil {
		return nil, err
	}
	f, err := s.Storages.OpenPath(ctx, file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, maxSubtitleSize))
}

// WriteFile replaces a subtitle file of an item's video, the ExternalPath
// of one of its streams, as after modifying or synchronizing it.
func (s *Subtitles) WriteFile(ctx context.Context, itemID core.ID, file string, data []byte) error {
	if _, _, err := s.sourceWith(ctx, itemID, file); err != nil {
		return err
	}
	st, err := s.Storages.Open(ctx, cleanRoot(path.Dir(file)))
	if err != nil {
		return err
	}
	defer st.Close()
	if err := st.WriteFile(ctx, path.Base(file), data); err != nil {
		return fmt.Errorf("save subtitle: %w", err)
	}
	return nil
}

// maxSubtitleSize bounds the subtitle files read.
const maxSubtitleSize = 64 << 20

// refreshSidecars reads the subtitle files of a video again, as a scan
// sees them, into the streams of sources[si], and stores the sources.
func (s *Subtitles) refreshSidecars(ctx context.Context, lib core.Library, itemID core.ID, sources []core.MediaSource, si int) error {
	src := &sources[si]
	files, err := s.readSidecarDirs(ctx, lib, itemID, src.Path)
	if err != nil {
		return err
	}
	src.Streams = withSidecars(src.Streams, sidecars(naming.Default(), src.Path, s.folder().dirs(src.Path, itemID), files))
	return s.Store.MediaSources().Replace(ctx, itemID, sources)
}

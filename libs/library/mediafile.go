package library

import (
	"context"
	"path"

	"github.com/mavioai/mavio/libs/core"
)

// MediaFile is a media file of a library as the tools reading it, such as
// ffprobe and ffmpeg, take it. Close releases it.
type MediaFile struct {
	// Path is the file's path as items record it.
	Path string
	// URL is the input ffmpeg and ffprobe read: a file: URL on local
	// storage, an HTTP URL otherwise.
	URL string
	// Local is the file's OS path on local storage; "" otherwise.
	Local string

	storage core.Storage
	name    string
}

// Remote reports whether the file is read over the network, so that tools
// read as little of it as they can.
func (m MediaFile) Remote() bool { return m.Local == "" }

// Open opens the file for reads at offsets.
func (m MediaFile) Open(ctx context.Context) (core.File, error) {
	return m.storage.Open(ctx, m.name)
}

// Close releases the file's storage.
func (m MediaFile) Close() error { return m.storage.Close() }

// Media opens the media file at p, a path items record, through the
// storage of its folder; the caller closes it.
func (s *Storages) Media(ctx context.Context, p string) (MediaFile, error) {
	st, err := s.Open(ctx, cleanRoot(path.Dir(p)))
	if err != nil {
		return MediaFile{}, err
	}
	name := path.Base(p)
	u, err := st.MediaURL(ctx, name)
	if err != nil {
		_ = st.Close()
		return MediaFile{}, err
	}
	m := MediaFile{Path: p, URL: u, storage: st, name: name}
	if local, ok := core.LocalPath(st, name); ok {
		m.Local = local
	}
	return m, nil
}

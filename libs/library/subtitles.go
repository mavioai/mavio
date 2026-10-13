package library

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/naming"
	"github.com/mavioai/mavio/libs/subtitle"
)

// SubtitleProvider finds and downloads subtitles from an external source,
// such as a subtitle plugin.
type SubtitleProvider interface {
	Name() string
	SearchSubtitles(ctx context.Context, q SubtitleQuery) ([]RemoteSubtitle, error)
	DownloadSubtitle(ctx context.Context, id string) (DownloadedSubtitle, error)
}

// SubtitleQuery describes the video subtitles are searched for.
type SubtitleQuery struct {
	Kind        core.ItemKind
	Name        string
	Year        int
	ExternalIDs map[core.Provider]string
	// SeriesName, SeriesExternalIDs, SeasonNumber and EpisodeNumber place
	// episodes.
	SeriesName                  string
	SeriesExternalIDs           map[core.Provider]string
	SeasonNumber, EpisodeNumber *int
	// Language is an ISO 639-1 code, or "zh-Hans" or "zh-Hant".
	Language string
	// FileName is the video's file name without folders; FileHash its
	// OpenSubtitles hash (subtitle.FileHash), empty when unknown.
	FileName        string
	FileSize        int64
	FileHash        string
	HearingImpaired bool
	Forced          bool
}

// RemoteSubtitle is a subtitle a provider offers.
type RemoteSubtitle struct {
	Provider        string
	ID              string
	Name            string
	Language        string
	Format          string
	HearingImpaired bool
	Forced          bool
	Score           float64
	DownloadCount   int
	HashMatch       bool
}

// DownloadedSubtitle is a subtitle file from a provider.
type DownloadedSubtitle struct {
	Data            []byte
	Format          string
	Language        string
	HearingImpaired bool
	Forced          bool
}

// subtitleFormats are the formats saved, by extension.
var subtitleFormats = []string{"srt", "ass", "ssa", "vtt", "sub"}

// Subtitles finds subtitles for videos and saves them beside the videos,
// or in the subtitle folder, where they become subtitle streams.
type Subtitles struct {
	Store     core.Store
	Providers []SubtitleProvider
	// Source, when set, gives the providers instead of Providers.
	Source func() []SubtitleProvider
	// Storages opens the storage of library paths; nil opens local
	// folders only.
	Storages *Storages
	// Folder gives where subtitle files are saved and found; nil keeps
	// them beside their videos.
	Folder func() SubtitleFolder
	Logger *slog.Logger
}

// providers returns the subtitle providers a library uses, in its order.
func (s *Subtitles) providers(lib core.Library) []SubtitleProvider {
	all := s.Providers
	if s.Source != nil {
		all = s.Source()
	}
	return ordered(all, lib.Providers.Subtitles)
}

func (s *Subtitles) logger() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.DiscardHandler)
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

// Search asks every provider for subtitles of an item's video, in their
// order, each provider's best first. A provider failing leaves the
// others' results.
func (s *Subtitles) Search(ctx context.Context, lib core.Library, itemID, sourceID core.ID, language string, hearingImpaired, forced bool) ([]RemoteSubtitle, error) {
	it, sources, i, err := s.video(ctx, itemID, sourceID)
	if err != nil {
		return nil, err
	}
	src := sources[i]
	q := SubtitleQuery{
		Kind: it.Kind, Name: it.Name, Year: it.ProductionYear, ExternalIDs: it.ExternalIDs,
		Language: language, FileName: path.Base(src.Path), FileSize: src.Size,
		HearingImpaired: hearingImpaired, Forced: forced,
	}
	if it.Kind == core.KindEpisode {
		q.SeasonNumber, q.EpisodeNumber = it.ParentIndexNumber, it.IndexNumber
		for id := it.ParentID; !id.IsZero(); {
			p, err := s.Store.Items().Get(ctx, id)
			if err != nil {
				break
			}
			if p.Kind == core.KindSeries {
				q.SeriesName, q.SeriesExternalIDs = p.Name, p.ExternalIDs
				break
			}
			id = p.ParentID
		}
	}
	if hash, err := s.fileHash(ctx, lib, src.Path); err == nil {
		q.FileHash = hash
	} else {
		s.logger().DebugContext(ctx, "no hash for subtitle search", "path", src.Path, "err", err)
	}
	var out []RemoteSubtitle
	for _, p := range s.providers(lib) {
		found, err := p.SearchSubtitles(ctx, q)
		if err != nil {
			s.logger().WarnContext(ctx, "subtitle search failed", "provider", p.Name(), "item", it.ID, "err", err)
			continue
		}
		for _, f := range found {
			f.Provider = p.Name()
			out = append(out, f)
		}
	}
	return out, nil
}

// fileHash hashes a video within its library folder.
func (s *Subtitles) fileHash(ctx context.Context, lib core.Library, p string) (string, error) {
	root, rel, ok := libraryRoot(lib, p)
	if !ok {
		return "", fmt.Errorf("%s is outside the library's folders", p)
	}
	st, err := s.Storages.Open(ctx, root)
	if err != nil {
		return "", err
	}
	defer st.Close()
	f, err := st.Open(ctx, rel)
	if err != nil {
		return "", err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	return subtitle.FileHash(f, info.Size())
}

// Download saves a provider's subtitle beside an item's video, named
// "<video>.<language>[.sdh][.forced].<format>", and adds it to the video's
// streams, which it returns.
func (s *Subtitles) Download(ctx context.Context, lib core.Library, itemID, sourceID core.ID, provider, id string) (core.MediaStream, error) {
	providers := s.providers(lib)
	i := slices.IndexFunc(providers, func(p SubtitleProvider) bool { return p.Name() == provider })
	if i < 0 {
		return core.MediaStream{}, fmt.Errorf("subtitle provider %s: %w", provider, core.ErrNotFound)
	}
	_, sources, si, err := s.video(ctx, itemID, sourceID)
	if err != nil {
		return core.MediaStream{}, err
	}
	sub, err := providers[i].DownloadSubtitle(ctx, id)
	if err != nil {
		return core.MediaStream{}, fmt.Errorf("download subtitle: %w", err)
	}
	format := strings.ToLower(sub.Format)
	if !slices.Contains(subtitleFormats, format) {
		return core.MediaStream{}, fmt.Errorf("download subtitle: unknown format %q", sub.Format)
	}
	data := sub.Data
	if format != "sub" {
		if data, _, err = subtitle.ToUTF8(data); err != nil {
			return core.MediaStream{}, fmt.Errorf("download subtitle: %w", err)
		}
	}
	return s.Save(ctx, lib, itemID, sources[si].ID, func(fsys fs.FS, stem string) (string, error) {
		return subtitleName(fsys, stem, sub, format), nil
	}, data)
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
	case folder.absolute() != "":
		root, dir = folder.absolute(), "."
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
	files, err := s.readSidecarDirs(ctx, lib, src.Path)
	if err != nil {
		return err
	}
	src.Streams = withSidecars(src.Streams, sidecars(naming.Default(), src.Path, s.folder().dirs(src.Path), files))
	return s.Store.MediaSources().Replace(ctx, itemID, sources)
}

// subtitleName returns a free name for a subtitle of the video named
// stem, without its extension.
func subtitleName(fsys fs.FS, stem string, sub DownloadedSubtitle, format string) string {
	base := stem + "." + cmp.Or(sub.Language, "und")
	if sub.HearingImpaired {
		base += ".sdh"
	}
	if sub.Forced {
		base += ".forced"
	}
	name := base + "." + format
	for n := 2; ; n++ {
		if _, err := fs.Stat(fsys, name); err != nil {
			return name
		}
		name = base + "." + strconv.Itoa(n) + "." + format
	}
}

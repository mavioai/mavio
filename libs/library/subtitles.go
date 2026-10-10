package library

import (
	"cmp"
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
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
// where they become subtitle streams.
type Subtitles struct {
	Store     core.Store
	Providers []SubtitleProvider
	// Source, when set, gives the providers instead of Providers.
	Source func() []SubtitleProvider
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
	if hash, err := fileHash(lib, src.Path); err == nil {
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
func fileHash(lib core.Library, p string) (string, error) {
	root, rel, ok := libraryRoot(lib, p)
	if !ok {
		return "", fmt.Errorf("%s is outside the library's folders", p)
	}
	f, err := os.OpenInRoot(filepath.FromSlash(root), rel)
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
	src := &sources[si]
	root, rel, ok := libraryRoot(lib, src.Path)
	if !ok {
		return core.MediaStream{}, fmt.Errorf("%w: %s is outside the library's folders", core.ErrInvalid, src.Path)
	}
	rt, err := os.OpenRoot(filepath.FromSlash(root))
	if err != nil {
		return core.MediaStream{}, err
	}
	defer rt.Close()
	name := subtitleName(rt.FS(), rel, sub, format)
	if err := writeFileIn(rt, name, data); err != nil {
		return core.MediaStream{}, fmt.Errorf("save subtitle: %w", err)
	}

	// The folder's subtitle files, as a scan sees them.
	dir := path.Dir(rel)
	entries, err := fs.ReadDir(rt.FS(), dir)
	if err != nil {
		return core.MediaStream{}, err
	}
	files := map[string]core.FolderEntry{}
	for _, e := range entries {
		if info, err := e.Info(); err == nil {
			files[path.Join(root, dir, e.Name())] = core.FolderEntry{Name: e.Name(), IsDir: e.IsDir(), Size: info.Size(), ModTime: info.ModTime()}
		}
	}
	src.Streams = withSidecars(src.Streams, sidecars(naming.Default(), src.Path, files))
	if err := s.Store.MediaSources().Replace(ctx, itemID, sources); err != nil {
		return core.MediaStream{}, err
	}
	saved := path.Join(root, name)
	for _, st := range src.Streams {
		if st.ExternalPath == saved {
			return st, nil
		}
	}
	return core.MediaStream{}, fmt.Errorf("saved subtitle %s is not named after its video", saved)
}

// subtitleName returns a free name for a subtitle beside a video, both
// relative to the library folder.
func subtitleName(fsys fs.FS, video string, sub DownloadedSubtitle, format string) string {
	stem := strings.TrimSuffix(video, path.Ext(video))
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

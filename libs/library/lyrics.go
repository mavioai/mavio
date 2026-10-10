package library

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/metadata"
)

// MaxLyricsSize bounds downloaded lyrics.
const MaxLyricsSize = 1 << 20

// LyricsProvider finds lyrics of tracks in an external source, such as a
// lyrics provider plugin.
type LyricsProvider interface {
	Name() string
	SearchLyrics(ctx context.Context, q LyricsQuery) ([]RemoteLyrics, error)
	// DownloadLyrics returns lyrics SearchLyrics found: LRC when synced,
	// else plain text.
	DownloadLyrics(ctx context.Context, id string) (content string, synced bool, err error)
}

// LyricsQuery describes the track lyrics are searched for.
type LyricsQuery struct {
	Name        string
	Artists     []string
	Album       string
	Duration    time.Duration
	ExternalIDs map[core.Provider]string
}

// RemoteLyrics are lyrics a provider has.
type RemoteLyrics struct {
	// Provider names the provider; ID is its ID of the lyrics.
	Provider, ID string
	Name         string
	Artists      []string
	Album        string
	Duration     time.Duration
	Synced       bool
	Score        float64
}

// Lyrics finds lyrics of tracks through the lyrics providers and saves
// them beside the tracks.
type Lyrics struct {
	Store core.Store
	// Source gives the providers, in order.
	Source func() []LyricsProvider
	Logger *slog.Logger
}

func (l *Lyrics) providers() []LyricsProvider {
	if l.Source == nil {
		return nil
	}
	return l.Source()
}

func (l *Lyrics) logger() *slog.Logger {
	if l.Logger != nil {
		return l.Logger
	}
	return slog.New(slog.DiscardHandler)
}

// track returns a track.
func (l *Lyrics) track(ctx context.Context, itemID core.ID) (core.Item, error) {
	it, err := l.Store.Items().Get(ctx, itemID)
	if err != nil {
		return it, err
	}
	if it.Kind != core.KindTrack {
		return it, fmt.Errorf("%w: only tracks have lyrics", core.ErrInvalid)
	}
	return it, nil
}

// Search asks every provider for lyrics of a track, in their order, each
// provider's best first. A provider failing leaves the others' results.
func (l *Lyrics) Search(ctx context.Context, itemID core.ID) ([]RemoteLyrics, error) {
	it, err := l.track(ctx, itemID)
	if err != nil {
		return nil, err
	}
	q := LyricsQuery{Name: it.Name, Artists: it.Artists, Album: it.Album, Duration: it.Runtime, ExternalIDs: it.ExternalIDs}
	if q.Duration == 0 {
		if sources, err := l.Store.MediaSources().ListForItem(ctx, it.ID); err == nil && len(sources) > 0 {
			q.Duration = sources[0].Duration
		}
	}
	var out []RemoteLyrics
	for _, p := range l.providers() {
		found, err := p.SearchLyrics(ctx, q)
		if err != nil {
			l.logger().WarnContext(ctx, "lyrics search failed", "provider", p.Name(), "item", it.ID, "err", err)
			continue
		}
		for _, f := range found {
			f.Provider = p.Name()
			out = append(out, f)
		}
	}
	return out, nil
}

// Download saves a provider's lyrics beside a track, as "<track>.lrc"
// when synced and "<track>.txt" otherwise, replacing the track's other
// lyric files.
func (l *Lyrics) Download(ctx context.Context, lib core.Library, itemID core.ID, provider, id string) error {
	providers := l.providers()
	i := slices.IndexFunc(providers, func(p LyricsProvider) bool { return p.Name() == provider })
	if i < 0 {
		return fmt.Errorf("lyrics provider %s: %w", provider, core.ErrNotFound)
	}
	it, err := l.track(ctx, itemID)
	if err != nil {
		return err
	}
	content, synced, err := providers[i].DownloadLyrics(ctx, id)
	if err != nil {
		return fmt.Errorf("download lyrics: %w", err)
	}
	ext := ".txt"
	if synced {
		ext = ".lrc"
	}
	if len(content) > MaxLyricsSize {
		return fmt.Errorf("download lyrics: %d bytes; at most %d", len(content), MaxLyricsSize)
	}
	if parsed, ok := metadata.ParseLyrics("lyrics"+ext, content); !ok || synced && !parsed.Metadata.Synced ||
		!slices.ContainsFunc(parsed.Lines, func(line metadata.LyricLine) bool { return line.Text != "" }) {
		return errors.New("download lyrics: no lyrics in what the provider gave")
	}
	root, rel, ok := libraryRoot(lib, it.Path)
	if !ok {
		return fmt.Errorf("%w: %s is outside the library's folders", core.ErrInvalid, it.Path)
	}
	rt, err := os.OpenRoot(filepath.FromSlash(root))
	if err != nil {
		return err
	}
	defer rt.Close()
	stem := strings.TrimSuffix(rel, path.Ext(rel))
	if err := writeFileIn(rt, stem+ext, []byte(content)); err != nil {
		return fmt.Errorf("save lyrics: %w", err)
	}
	for _, other := range metadata.LyricExtensions {
		if other != ext {
			if err := rt.Remove(stem + other); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("remove old lyrics: %w", err)
			}
		}
	}
	return nil
}

package subtitles

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"path"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/subsync"
	"github.com/mavioai/mavio/libs/subtitle"
)

// readable reports whether the server reads a video's file on its own:
// on local disks and network shares, but not on cloud drives, whose reads
// are counted.
func (r *Runtime) readable(ctx context.Context, p string) bool {
	if r.Class != nil {
		return r.Class(ctx, p) != core.StorageCloud
	}
	return !core.IsRemotePath(p)
}

// napiprojektSize is how much of a video NapiProjekt hashes.
const napiprojektSize = 10 << 20

// napiprojektHash returns the MD5 of a video's first 10 MiB, as
// subliminal's hash_napiprojekt, for videos larger than that.
func (r *Runtime) napiprojektHash(ctx context.Context, lib core.Library, src *core.MediaSource) (string, error) {
	if src.Size <= napiprojektSize {
		return "", errors.New("video too small for a NapiProjekt hash")
	}
	root, name, ok := library.RootOf(lib.Paths, src.Path)
	if !ok {
		return "", fmt.Errorf("%s is outside the library's folders", src.Path)
	}
	f, err := r.Storages.OpenFile(ctx, root, name)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, io.NewSectionReader(f, 0, napiprojektSize)); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// shouldSync reports whether the settings synchronize a download, as
// Bazarr's sync_subtitles decides.
func shouldSync(s *core.SubtitleSettings, e *core.SubtitleHistoryEvent, episode bool) bool {
	y := &s.Sync
	if !y.Enabled || slices.Contains(y.ExcludedLanguages, e.Language.Language) {
		return false
	}
	provider := e.Provider[strings.LastIndex(e.Provider, "/")+1:]
	if slices.Contains(y.ExcludedProviders, provider) || slices.Contains(y.ExcludedProviders, e.Provider) {
		return false
	}
	pct := percent(e.Score, e.MaxScore)
	switch {
	case episode && y.UseThreshold:
		return pct < y.Threshold
	case !episode && y.UseMovieThreshold:
		return pct < y.MovieThreshold
	}
	return true
}

// syncDownloaded synchronizes a download when the settings say to and
// its video is read where it lies; failures are logged.
func (r *Runtime) syncDownloaded(ctx context.Context, t *target, e *core.SubtitleHistoryEvent, s *core.SubtitleSettings) {
	if !shouldSync(s, e, t.episode()) || !r.readable(ctx, t.source().Path) || r.FFmpeg == "" {
		return
	}
	if _, err := r.sync(ctx, t, e.Path, e.Language, s); err != nil {
		r.logger().WarnContext(ctx, "subtitle synchronization failed", "item", t.item.ID, "path", e.Path, "err", err)
	}
}

// SyncResult is how a subtitle was moved.
type SyncResult struct {
	Offset time.Duration
	Scale  float64
}

// errNotLocal is the error of synchronizing subtitles of videos on cloud
// drives, whose audio would have to be read whole.
var errNotLocal = fmt.Errorf("%w: the video is on a cloud drive; its audio is not read to synchronize subtitles", ErrUnavailable)

// sync aligns a subtitle file of the target to its video's audio, as
// Bazarr's subsync with ffsubsync, rewrites it and records it.
func (r *Runtime) sync(ctx context.Context, t *target, file string, l core.SubtitleLanguage, s *core.SubtitleSettings) (SyncResult, error) {
	src := t.source()
	if !r.readable(ctx, src.Path) {
		return SyncResult{}, errNotLocal
	}
	if r.FFmpeg == "" {
		return SyncResult{}, fmt.Errorf("%w: synchronizing subtitles needs ffmpeg", ErrUnavailable)
	}
	format := strings.TrimPrefix(strings.ToLower(path.Ext(file)), ".")
	if !subtitle.CanParse(format) {
		return SyncResult{}, fmt.Errorf("%w: %s subtitles are not synchronized", ErrUnavailable, format)
	}
	data, err := r.Subtitles.ReadFile(ctx, t.lib, t.item.ID, file)
	if err != nil {
		return SyncResult{}, err
	}
	text, _, err := subtitle.ToUTF8(data)
	if err != nil {
		return SyncResult{}, err
	}
	sub, err := subtitle.Parse(text, format)
	if err != nil {
		return SyncResult{}, err
	}
	root, name, ok := library.RootOf(t.lib.Paths, src.Path)
	if !ok {
		return SyncResult{}, fmt.Errorf("%w: %s is outside the library's folders", core.ErrInvalid, src.Path)
	}
	st, err := r.Storages.Open(ctx, root)
	if err != nil {
		return SyncResult{}, err
	}
	defer st.Close()
	input, err := st.MediaURL(core.WithReadIntent(ctx, core.ReadIntent{Kind: core.ReadBackground}), name)
	if err != nil {
		return SyncResult{}, err
	}
	model, err := r.vad()
	if err != nil {
		return SyncResult{}, err
	}
	ref, err := subsync.AudioSpeech(ctx, r.FFmpeg, input, audioStream(src.Streams, t.owner.OriginalLanguage, s), model, runtime.GOMAXPROCS(0))
	if err != nil {
		return SyncResult{}, err
	}
	y := &s.Sync
	res, err := subsync.Align(ctx, ref, sub.Cues, subsync.Options{MaxOffset: y.MaxOffset, FixFramerate: y.FixFramerate, GoldenSection: y.GoldenSection})
	if err != nil {
		return SyncResult{}, err
	}
	if y.FixFramerate {
		if problems := res.Problems(subsync.Quality{MinScore: y.MinScore, MaxOffset: y.MaxQualityOffset, MaxScaleDeviation: y.MaxScaleDeviation}); len(problems) > 0 {
			return SyncResult{}, fmt.Errorf("alignment rejected: %s", strings.Join(problems, ", "))
		}
	}
	sub.Cues = res.Apply(sub.Cues)
	out, err := subtitle.Write(sub, format)
	if err != nil {
		return SyncResult{}, err
	}
	if err := r.Subtitles.WriteFile(ctx, t.lib, t.item.ID, file, out); err != nil {
		return SyncResult{}, err
	}
	e := core.SubtitleHistoryEvent{
		ID: core.NewID(), Action: core.SubtitleSynced, ItemID: t.item.ID, Date: r.now(), Language: l,
		Path: file, VideoPath: src.Path,
		Description: fmt.Sprintf("%s subtitles synchronized with an offset of %.3f seconds and a framerate scale of %.4f.", l, res.Offset.Seconds(), res.Scale),
	}
	if t.series != nil {
		e.ParentID = t.series.ID
	}
	if err := r.record(ctx, t, &e); err != nil {
		return SyncResult{}, err
	}
	return SyncResult{Offset: res.Offset, Scale: res.Scale}, nil
}

// audioStream returns which audio stream to synchronize against, among
// the video's audio streams: that in the original language when the
// settings ask for it and there is one, else the default, else ffmpeg's
// choice (-1).
func audioStream(streams []core.MediaStream, original string, s *core.SubtitleSettings) int {
	original = Tag(original)
	n, def := 0, -1
	for _, st := range streams {
		if st.Kind != core.StreamAudio || st.ExternalPath != "" {
			continue
		}
		if s.Sync.OriginalLanguage && original != "" && Tag(st.Language) == original {
			return n
		}
		if st.Default && def < 0 {
			def = n
		}
		n++
	}
	return def
}

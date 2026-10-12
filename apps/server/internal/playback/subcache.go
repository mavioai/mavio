package playback

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/subtitle"
)

// stampFile records, in a source's subtitle folder, the size and
// modification time of the file they were extracted from.
const stampFile = "source"

// sourceLocks serializes the extraction of each source's subtitles.
type sourceLocks struct {
	mu    sync.Mutex
	locks map[core.ID]*sourceLock
}

type sourceLock struct {
	sem   chan struct{}
	users int
}

// lock waits for a source's lock; unlock releases it.
func (l *sourceLocks) lock(ctx context.Context, id core.ID) (unlock func(), err error) {
	l.mu.Lock()
	if l.locks == nil {
		l.locks = map[core.ID]*sourceLock{}
	}
	sl := l.locks[id]
	if sl == nil {
		sl = &sourceLock{sem: make(chan struct{}, 1)}
		l.locks[id] = sl
	}
	sl.users++
	l.mu.Unlock()
	release := func() {
		l.mu.Lock()
		if sl.users--; sl.users == 0 {
			delete(l.locks, id)
		}
		l.mu.Unlock()
	}
	select {
	case sl.sem <- struct{}{}:
		return func() { <-sl.sem; release() }, nil
	case <-ctx.Done():
		release()
		return nil, context.Cause(ctx)
	}
}

// extracted returns an embedded text subtitle stream of a source, as
// sourceFormat reads it, from the metadata folder. The first request
// extracts every embedded text stream of the source in one ffmpeg run,
// which reads the file once; later playbacks read them from there until
// the file's size or modification time changes.
func (m *Manager) extracted(ctx context.Context, itemID core.ID, ms *core.MediaSource, st *core.MediaStream) ([]byte, error) {
	if m.cfg.FFmpegPath == "" || m.cfg.MetadataDir == "" {
		return nil, errNoSubtitle
	}
	dir := library.SubtitlesDir(m.cfg.MetadataDir, itemID, ms.ID)
	unlock, err := m.subLocks.lock(ctx, ms.ID)
	if err != nil {
		return nil, err
	}
	defer unlock()
	stamp := sourceStamp(ms)
	if b, err := os.ReadFile(filepath.Join(dir, stampFile)); err != nil || string(b) != stamp {
		if err := extractAll(ctx, m.cfg.FFmpegPath, ms, dir, stamp); err != nil {
			return nil, err
		}
	}
	data, err := os.ReadFile(filepath.Join(dir, extractedName(st)))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, errNoSubtitle
	}
	return data, err
}

// sourceStamp identifies the version of a source's file.
func sourceStamp(ms *core.MediaSource) string {
	return strconv.FormatInt(ms.Size, 10) + " " + strconv.FormatInt(ms.Modified.UnixNano(), 10) + "\n"
}

// extractedName is the file an embedded stream is extracted to.
func extractedName(st *core.MediaStream) string {
	return strconv.Itoa(st.Index) + "." + sourceFormat(st)
}

// extractAll extracts every embedded text subtitle stream of a source into
// dir, replacing what it held: ASS keeps its styling, other text becomes
// SRT. Cues are timed from the source's start, as HLS outputs are.
func extractAll(ctx context.Context, ffmpeg string, ms *core.MediaSource, dir, stamp string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp(filepath.Dir(dir), ".subtitles-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-i", "file:" + ms.Path}
	n := 0
	for i := range ms.Streams {
		st := &ms.Streams[i]
		if st.Kind != core.StreamSubtitle || st.ExternalPath != "" || !st.IsTextSubtitle() {
			continue
		}
		codec := "srt"
		if sourceFormat(st) == subtitle.ASS {
			codec = "ass"
		}
		args = append(args, "-map", "0:"+strconv.Itoa(st.Index), "-c:s", codec, "-f", codec, filepath.Join(tmp, extractedName(st)))
		n++
	}
	if n > 0 {
		cmd := exec.CommandContext(ctx, ffmpeg, args...)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("extract subtitles of %s: %w: %s", ms.Path, err, bytes.TrimSpace(stderr.Bytes()))
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, stampFile), []byte(stamp), 0o644); err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	return os.Rename(tmp, dir)
}

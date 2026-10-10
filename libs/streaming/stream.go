package streaming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/mavioai/mavio/libs/media/planner"
	"github.com/mavioai/mavio/libs/media/supervisor"
)

// InitSegment is the index of the fMP4 initialization segment.
const InitSegment = -1

// ErrSegmentRange is returned for segments the stream does not have.
var ErrSegmentRange = errors.New("streaming: segment out of range")

// Config configures a stream.
type Config struct {
	// Dir holds the stream's files; it is created, and removed on Close.
	Dir    string
	Layout Layout
	// Container is the segment container, "mp4" (fMP4) or "ts".
	Container string
	// Args returns the ffmpeg arguments producing the stream from start
	// on, with the output as given; typically planner.HLSArgs for a job
	// starting at start.
	Args  func(start time.Duration, out planner.HLSOutput) []string
	Start supervisor.Starter
	// Supervision configures each ffmpeg run; its Args are replaced.
	Supervision supervisor.Config
	// MaxGap is how far ahead of what ffmpeg has written a request may
	// be before ffmpeg is restarted at the requested segment; default 24
	// seconds, as Jellyfin.
	MaxGap time.Duration
	// Poll is how often files are checked while waiting; default 100
	// milliseconds.
	Poll   time.Duration
	Logger *slog.Logger
}

// Stream produces the segments of one HLS rendition on demand. Segments
// are written by one ffmpeg run at a time, which is restarted at the
// requested segment when a client seeks before what it has written or far
// beyond; segments already written are kept and served again. Requests
// are served one at a time, as one client plays a stream.
type Stream struct {
	cfg Config
	sem chan struct{}

	// Guarded by sem.
	job    *job
	closed bool
	// begin is the segment the stream begins at, where the
	// initialization segment starts ffmpeg.
	begin int
}

type job struct {
	session *supervisor.Session
	// first is the number of the first file the run writes.
	first int
}

// NewStream creates a stream; ffmpeg starts with Prepare or the first
// request.
func NewStream(cfg Config) (*Stream, error) {
	if cfg.MaxGap <= 0 {
		cfg.MaxGap = 24 * time.Second
	}
	if cfg.Poll <= 0 {
		cfg.Poll = 100 * time.Millisecond
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.DiscardHandler)
	}
	if cfg.Container == "" {
		cfg.Container = "mp4"
	}
	if err := os.MkdirAll(cfg.Dir, 0o755); err != nil {
		return nil, fmt.Errorf("stream directory: %w", err)
	}
	return &Stream{cfg: cfg, sem: make(chan struct{}, 1)}, nil
}

func (s *Stream) lock(ctx context.Context) error {
	select {
	case s.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return context.Cause(ctx)
	}
}

func (s *Stream) unlock() { <-s.sem }

// file is the path of file n: a segment, or a keyframe chunk.
func (s *Stream) file(n int) string { return s.filePattern(strconv.Itoa(n)) }

func (s *Stream) filePattern(n string) string {
	prefix := "s"
	if s.cfg.Layout.Chunked() {
		prefix = "k"
	}
	return filepath.Join(s.cfg.Dir, prefix+n+"."+s.cfg.Container)
}

// initFile is the initialization segment served; every run writes its
// own, all alike, to jobInit, and the first is kept.
func (s *Stream) initFile() string { return filepath.Join(s.cfg.Dir, "init."+s.cfg.Container) }

const jobInit = "run-init.mp4"

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Segment opens segment index, or the initialization segment for
// [InitSegment], waiting for ffmpeg to write it and starting or
// restarting ffmpeg as needed. The reader holds the segment's files open,
// so later restarts do not change what it reads.
func (s *Stream) Segment(ctx context.Context, index int) (io.ReadCloser, int64, error) {
	l := s.cfg.Layout
	if index < InitSegment || index >= len(l.Segments) || (index == InitSegment && s.cfg.Container == "ts") {
		return nil, 0, ErrSegmentRange
	}
	if err := s.lock(ctx); err != nil {
		return nil, 0, err
	}
	defer s.unlock()
	if s.closed {
		return nil, 0, errors.New("streaming: stream closed")
	}
	if index == InitSegment {
		return s.init(ctx)
	}
	first, last := l.files(index)
	if !s.written(first, last) {
		if s.restart(index) {
			if err := s.startAt(ctx, index); err != nil {
				return nil, 0, err
			}
		} else {
			s.job.session.Touch()
		}
		if err := s.wait(ctx, func() bool { return s.written(first, last) }); err != nil {
			return nil, 0, fmt.Errorf("segment %d: %w", index, err)
		}
	}
	if s.job != nil {
		s.job.session.ReportSegment(index, l.End(index))
	}
	paths := make([]string, 0, last-first+1)
	for n := first; n <= last; n++ {
		paths = append(paths, s.file(n))
	}
	return openAll(paths)
}

// Prepare starts ffmpeg at the segment playing at, where the client
// begins, unless a run is going: the first run reads from there instead
// of from the beginning, so that a playback resuming further on does not
// start ffmpeg twice, and segments are being written while the client
// fetches its playlists.
func (s *Stream) Prepare(ctx context.Context, at time.Duration) error {
	if err := s.lock(ctx); err != nil {
		return err
	}
	defer s.unlock()
	if s.closed {
		return errors.New("streaming: stream closed")
	}
	s.begin = s.cfg.Layout.Index(at)
	if s.job != nil && !s.ended() {
		return nil
	}
	return s.startAt(ctx, s.begin)
}

// init returns the initialization segment, starting ffmpeg where the
// stream begins if none was written yet.
func (s *Stream) init(ctx context.Context) (io.ReadCloser, int64, error) {
	if !exists(s.initFile()) {
		if s.job == nil || s.ended() {
			if err := s.startAt(ctx, s.begin); err != nil {
				return nil, 0, err
			}
		}
		// A run's initialization segment is complete once its first
		// file is.
		if err := s.wait(ctx, func() bool { return exists(s.file(s.job.first)) }); err != nil {
			return nil, 0, fmt.Errorf("initialization segment: %w", err)
		}
		if err := copyFile(filepath.Join(s.cfg.Dir, jobInit), s.initFile()); err != nil {
			return nil, 0, err
		}
	}
	return openAll([]string{s.initFile()})
}

// written reports whether files first to last are complete.
func (s *Stream) written(first, last int) bool {
	for n := first; n <= last; n++ {
		if !exists(s.file(n)) {
			return false
		}
	}
	return true
}

func (s *Stream) ended() bool {
	select {
	case <-s.job.session.Done():
		return true
	default:
		return false
	}
}

// restart decides whether ffmpeg must start anew for segment index: when
// none runs, when the segment lies before where the run started, or when
// it lies further ahead of what the run has written than MaxGap.
func (s *Stream) restart(index int) bool {
	if s.job == nil || s.ended() {
		return true
	}
	l := s.cfg.Layout
	first, _ := l.files(index)
	if first < s.job.first {
		return true
	}
	// The run is at the end of the files it wrote in a row, or at its
	// start.
	n := s.job.first
	for exists(s.file(n)) {
		n++
	}
	reached := s.runStart()
	if n > s.job.first {
		reached = l.fileEnd(n - 1)
	}
	return l.Start(index)-reached > s.cfg.MaxGap
}

// runStart is the time the current run started at.
func (s *Stream) runStart() time.Duration {
	l := s.cfg.Layout
	if !l.Chunked() {
		return l.Start(s.job.first)
	}
	if s.job.first == 0 {
		return 0
	}
	return l.chunks[s.job.first]
}

// startAt stops the current run, removes its unfinished files and starts
// ffmpeg at segment index.
func (s *Stream) startAt(ctx context.Context, index int) error {
	s.stopJob()
	start, first := s.cfg.Layout.seek(index)
	out := planner.HLSOutput{
		Playlist:       filepath.Join(s.cfg.Dir, "run.m3u8"),
		Segments:       s.filePattern("%d"),
		Init:           jobInit,
		StartNumber:    first,
		KeyframeChunks: s.cfg.Layout.Chunked(),
	}
	cfg := s.cfg.Supervision
	cfg.Args = s.cfg.Args(start, out)
	if cfg.Logger == nil {
		cfg.Logger = s.cfg.Logger
	}
	session, err := supervisor.Start(context.WithoutCancel(ctx), s.cfg.Start, cfg)
	if err != nil {
		return err
	}
	s.cfg.Logger.DebugContext(ctx, "transcode started", "segment", index, "start", start)
	s.job = &job{session: session, first: first}
	return nil
}

// stopJob ends the current run and removes the files it left unfinished.
func (s *Stream) stopJob() {
	if s.job == nil {
		return
	}
	_ = s.job.session.Stop()
	s.job = nil
	tmp, _ := filepath.Glob(filepath.Join(s.cfg.Dir, "*.tmp"))
	for _, f := range tmp {
		_ = os.Remove(f)
	}
}

// wait polls until done reports true, the run ends without it, or ctx is
// canceled.
func (s *Stream) wait(ctx context.Context, done func() bool) error {
	t := time.NewTicker(s.cfg.Poll)
	defer t.Stop()
	for !done() {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case <-s.job.session.Done():
			if done() {
				return nil
			}
			if err := s.job.session.Err(); err != nil {
				return fmt.Errorf("transcode failed: %w", err)
			}
			return errors.New("transcode ended before writing it")
		case <-t.C:
		}
	}
	return nil
}

// ReportPosition records the client's playback position, which throttles
// ffmpeg and counts as activity.
func (s *Stream) ReportPosition(ctx context.Context, pos time.Duration) error {
	if err := s.lock(ctx); err != nil {
		return err
	}
	defer s.unlock()
	if s.job != nil {
		s.job.session.ReportPosition(pos)
	}
	return nil
}

// Close stops ffmpeg and removes the stream's files.
func (s *Stream) Close() error {
	s.sem <- struct{}{}
	defer s.unlock()
	s.stopJob()
	s.closed = true
	return os.RemoveAll(s.cfg.Dir)
}

func copyFile(from, to string) error {
	data, err := os.ReadFile(from)
	if err != nil {
		return fmt.Errorf("initialization segment: %w", err)
	}
	tmp := to + ".copy"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, to)
}

// openAll opens files to be read one after another.
func openAll(paths []string) (io.ReadCloser, int64, error) {
	var (
		files   []*os.File
		readers []io.Reader
		size    int64
	)
	for _, p := range paths {
		f, err := os.Open(p)
		if err != nil {
			for _, f := range files {
				_ = f.Close()
			}
			return nil, 0, err
		}
		fi, err := f.Stat()
		if err != nil {
			_ = f.Close()
			return nil, 0, err
		}
		files, readers, size = append(files, f), append(readers, f), size+fi.Size()
	}
	return &multiFile{Reader: io.MultiReader(readers...), files: files}, size, nil
}

type multiFile struct {
	io.Reader
	files []*os.File
}

func (m *multiFile) Close() error {
	var errs []error
	for _, f := range m.files {
		errs = append(errs, f.Close())
	}
	return errors.Join(errs...)
}

package streaming

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/media/planner"
	"github.com/mavioai/mavio/libs/media/supervisor"
)

// fakeFFmpeg writes numbered files as ffmpeg's HLS muxer does: each under
// a temporary name first, one every interval, until it has written files
// up to last or is told to quit. With failAt set, every run fails instead
// of writing that file, as on corrupt input.
type fakeFFmpeg struct {
	last     int
	interval time.Duration
	failAt   int

	mu   sync.Mutex
	runs []run
}

type run struct {
	start time.Duration
	first int
}

func (f *fakeFFmpeg) runsSoFar() []run {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.runs)
}

type fakeProcess struct {
	stdin    io.WriteCloser
	quit     io.Reader
	progress *io.PipeReader
	done     chan error
}

func (p *fakeProcess) Stdin() io.Writer    { return p.stdin }
func (p *fakeProcess) Progress() io.Reader { return p.progress }
func (p *fakeProcess) Wait() error         { return <-p.done }
func (p *fakeProcess) Kill() error         { return p.stdin.Close() }

func (f *fakeFFmpeg) start(_ context.Context, args []string) (supervisor.Process, error) {
	arg := func(name string) string { return args[slices.Index(args, name)+1] }
	first, _ := strconv.Atoi(arg("-start_number"))
	pattern := arg("-hls_segment_filename")
	dir := filepath.Dir(arg("-y"))
	init := filepath.Join(dir, arg("-hls_fmp4_init_filename"))
	f.mu.Lock()
	f.runs = append(f.runs, run{start: time.Duration(seconds(arg("-output_ts_offset"))), first: first})
	f.mu.Unlock()

	quitR, quitW := io.Pipe()
	progR, progW := io.Pipe()
	p := &fakeProcess{stdin: quitW, quit: quitR, progress: progR, done: make(chan error, 1)}
	quit := make(chan struct{})
	go func() {
		// Any key or a closed stdin stops the run.
		_, _ = quitR.Read(make([]byte, 1))
		close(quit)
	}()
	go func() {
		defer progW.Close()
		write := func(path, content string) {
			_ = os.WriteFile(path+".tmp", []byte(content), 0o644)
			_ = os.Rename(path+".tmp", path)
		}
		write(init, "init")
		for n := first; n <= f.last; n++ {
			// The file being written when told to quit stays unfinished.
			path := fmt.Sprintf(pattern, n)
			_ = os.WriteFile(path+".tmp", []byte("partial"), 0o644)
			select {
			case <-quit:
				p.done <- nil
				return
			case <-time.After(f.interval):
			}
			if f.failAt > 0 && n == f.failAt {
				p.done <- errors.New("exit status 1: Invalid data found when processing input")
				return
			}
			write(path, "file"+strconv.Itoa(n))
		}
		p.done <- nil
	}()
	return p, nil
}

// seconds parses ffmpeg's -output_ts_offset; absent means zero.
func seconds(s string) float64 {
	if !strings.HasPrefix(s, "-") {
		v, _ := strconv.ParseFloat(s, 64)
		return v * float64(time.Second)
	}
	return 0
}

// testArgs stands in for the planner: the HLS options the fake reads.
func testArgs(start time.Duration, out planner.HLSOutput) []string {
	return []string{
		"-output_ts_offset", strconv.FormatFloat(start.Seconds(), 'f', -1, 64),
		"-start_number", strconv.Itoa(out.StartNumber), "-hls_fmp4_init_filename", out.Init,
		"-hls_segment_filename", out.Segments, "-y", out.Playlist,
	}
}

func newTestStream(t *testing.T, l Layout, f *fakeFFmpeg) *Stream {
	t.Helper()
	s, err := NewStream(Config{
		Dir: filepath.Join(t.TempDir(), "stream"), Layout: l, Args: testArgs, Start: f.start,
		Supervision: supervisor.Config{CheckInterval: time.Hour, StopGrace: time.Second},
		MaxGap:      3 * time.Second, Poll: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func read(t *testing.T, s *Stream, index int) string {
	t.Helper()
	r, size, err := s.Segment(t.Context(), index)
	if err != nil {
		t.Fatalf("Segment(%d): %v", index, err)
	}
	defer r.Close()
	data, err := io.ReadAll(r)
	if err != nil || int64(len(data)) != size {
		t.Fatalf("Segment(%d) read %d of %d bytes: %v", index, len(data), size, err)
	}
	return string(data)
}

func encoded(t *testing.T, n int) Layout {
	t.Helper()
	l, err := EncodedLayout(time.Second, time.Duration(n)*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestStreamPlaysThrough(t *testing.T) {
	f := &fakeFFmpeg{last: 9, interval: time.Millisecond}
	s := newTestStream(t, encoded(t, 10), f)
	if got := read(t, s, InitSegment); got != "init" {
		t.Errorf("init = %q", got)
	}
	for i := range 10 {
		if got, want := read(t, s, i), "file"+strconv.Itoa(i); got != want {
			t.Errorf("segment %d = %q, want = %q", i, got, want)
		}
	}
	if runs := f.runsSoFar(); len(runs) != 1 || runs[0] != (run{0, 0}) {
		t.Errorf("runs = %+v, want = one from the start", runs)
	}
}

func TestStreamSeeks(t *testing.T) {
	f := &fakeFFmpeg{last: 29, interval: 5 * time.Millisecond}
	s := newTestStream(t, encoded(t, 30), f)
	read(t, s, 0)
	read(t, s, 1)
	// Within reach: the run goes on.
	read(t, s, 3)
	// Far ahead: a new run starts there.
	if got := read(t, s, 20); got != "file20" {
		t.Errorf("segment 20 = %q", got)
	}
	// Back to written segments: served as they are.
	read(t, s, 1)
	// Back before the run, to a segment not written: a new run.
	read(t, s, 15)
	runs := f.runsSoFar()
	want := []run{{0, 0}, {20 * time.Second, 20}, {15 * time.Second, 15}}
	if !slices.Equal(runs, want) {
		t.Errorf("runs = %+v, want = %+v", runs, want)
	}
	tmp, _ := filepath.Glob(filepath.Join(s.cfg.Dir, "*.tmp"))
	if len(tmp) > 1 {
		t.Errorf("unfinished files = %v, want = at most the current run's", tmp)
	}
}

func TestStreamPrepare(t *testing.T) {
	f := &fakeFFmpeg{last: 29, interval: time.Millisecond}
	s := newTestStream(t, encoded(t, 30), f)
	// Resuming at 20.5 s starts the one run at segment 20, before any
	// request; the initialization segment and segment 20 come from it.
	if err := s.Prepare(t.Context(), 20500*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if got := read(t, s, InitSegment); got != "init" {
		t.Errorf("init = %q", got)
	}
	if got := read(t, s, 20); got != "file20" {
		t.Errorf("segment 20 = %q", got)
	}
	if err := s.Prepare(t.Context(), 21*time.Second); err != nil {
		t.Fatal(err)
	}
	if runs, want := f.runsSoFar(), []run{{20 * time.Second, 20}}; !slices.Equal(runs, want) {
		t.Errorf("runs = %+v, want = %+v", runs, want)
	}
}

func TestLayoutIndex(t *testing.T) {
	l := encoded(t, 10)
	cases := []struct {
		at   time.Duration
		want int
	}{{0, 0}, {999 * time.Millisecond, 0}, {time.Second, 1}, {9500 * time.Millisecond, 9}, {time.Hour, 9}, {-time.Second, 0}}
	for _, c := range cases {
		if got := l.Index(c.at); got != c.want {
			t.Errorf("Index(%v) = %d, want = %d", c.at, got, c.want)
		}
	}
}

func TestStreamChunked(t *testing.T) {
	// Keyframes every 1.5 s: 4 s segments cut at 4.5, 9, 12, 16.5, …
	var kf []time.Duration
	for k := time.Duration(0); k < 40*time.Second; k += 1500 * time.Millisecond {
		kf = append(kf, k)
	}
	l := CopiedLayout(kf, 40*time.Second, 4*time.Second)
	if got, want := l.Segments[:4], []time.Duration{4500 * time.Millisecond, 4500 * time.Millisecond, 3 * time.Second, 4500 * time.Millisecond}; !slices.Equal(got, want) {
		t.Fatalf("segments = %v, want = %v", got, want)
	}
	f := &fakeFFmpeg{last: len(kf) - 1, interval: time.Millisecond}
	s := newTestStream(t, l, f)
	// Segment 2 spans 9 s to 12 s: chunks 6 and 7, read from 9.75 s.
	if got := read(t, s, 6); got != "file16file17file18" {
		t.Errorf("segment 6 = %q", got)
	}
	if got := read(t, s, 2); got != "file6file7" {
		t.Errorf("segment 2 = %q", got)
	}
	runs := f.runsSoFar()
	if len(runs) != 2 || runs[1] != (run{9750 * time.Millisecond, 6}) {
		t.Errorf("runs = %+v, want = the second from chunk 6 at 9.75 s", runs)
	}
}

func TestStreamServesCachedSegments(t *testing.T) {
	f := &fakeFFmpeg{last: 4, interval: time.Millisecond}
	s := newTestStream(t, encoded(t, 5), f)
	for i := range 5 {
		read(t, s, i)
	}
	// The run has ended; written segments need none.
	read(t, s, 2)
	if runs := f.runsSoFar(); len(runs) != 1 {
		t.Errorf("runs = %+v, want = one", runs)
	}
}

func TestStreamSerializesRequests(t *testing.T) {
	f := &fakeFFmpeg{last: 9, interval: 10 * time.Millisecond}
	s := newTestStream(t, encoded(t, 10), f)
	// Without serialization each would find no run and start one.
	var wg sync.WaitGroup
	got := make([]string, 4)
	for i := range got {
		wg.Go(func() { got[i] = read(t, s, 0) })
	}
	wg.Wait()
	for i, g := range got {
		if g != "file0" {
			t.Errorf("request %d = %q", i, g)
		}
	}
	if runs := f.runsSoFar(); len(runs) != 1 {
		t.Errorf("runs = %+v, want = one", runs)
	}
}

func TestStreamRequestCanceled(t *testing.T) {
	f := &fakeFFmpeg{last: 9, interval: time.Hour}
	s := newTestStream(t, encoded(t, 10), f)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Millisecond)
	defer cancel()
	if _, _, err := s.Segment(ctx, 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Segment() error = %v, want = %v", err, context.DeadlineExceeded)
	}
	// A request waiting its turn gives up too.
	if err := s.lock(t.Context()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel = context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if _, _, err := s.Segment(ctx, 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("queued Segment() error = %v, want = %v", err, context.DeadlineExceeded)
	}
	s.unlock()
}

func TestStreamTranscodeFails(t *testing.T) {
	// Segment 1 fails whether its request waits for the first run or,
	// once that run ended, starts another.
	f := &fakeFFmpeg{last: 9, interval: time.Millisecond, failAt: 1}
	s := newTestStream(t, encoded(t, 10), f)
	read(t, s, 0)
	if _, _, err := s.Segment(t.Context(), 1); err == nil || !strings.Contains(err.Error(), "Invalid data") {
		t.Errorf("Segment() error = %v, want = the transcode's failure", err)
	}
}

func TestStreamRange(t *testing.T) {
	s := newTestStream(t, encoded(t, 3), &fakeFFmpeg{last: 2, interval: time.Millisecond})
	for _, i := range []int{-2, 3} {
		if _, _, err := s.Segment(t.Context(), i); !errors.Is(err, ErrSegmentRange) {
			t.Errorf("Segment(%d) error = %v, want = %v", i, err, ErrSegmentRange)
		}
	}
}

func TestStreamClose(t *testing.T) {
	f := &fakeFFmpeg{last: 9, interval: 5 * time.Millisecond}
	s := newTestStream(t, encoded(t, 10), f)
	read(t, s, 0)
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(s.cfg.Dir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("directory after Close: %v", err)
	}
	if _, _, err := s.Segment(t.Context(), 1); err == nil {
		t.Error("Segment() after Close = nil error")
	}
}

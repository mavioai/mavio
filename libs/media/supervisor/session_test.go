package supervisor

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"testing/synctest"
	"time"
)

// fakeProcess stands in for ffmpeg: it records the keys it receives,
// exits on "q" unless stubborn, and exits killed on Kill.
type fakeProcess struct {
	mu       sync.Mutex
	keys     strings.Builder
	stubborn bool
	exit     chan error
	once     sync.Once
	pr       *io.PipeReader
	pw       *io.PipeWriter
}

func newFake(stubborn bool) *fakeProcess {
	pr, pw := io.Pipe()
	return &fakeProcess{stubborn: stubborn, exit: make(chan error, 1), pr: pr, pw: pw}
}

var errKilled = errors.New("signal: killed")

func (f *fakeProcess) end(err error) {
	f.once.Do(func() {
		f.pw.Close()
		f.exit <- err
	})
}

func (f *fakeProcess) Stdin() io.Writer    { return fakeStdin{f} }
func (f *fakeProcess) Progress() io.Reader { return f.pr }
func (f *fakeProcess) Wait() error         { return <-f.exit }
func (f *fakeProcess) Kill() error         { f.end(errKilled); return nil }

func (f *fakeProcess) Keys() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.keys.String()
}

type fakeStdin struct{ f *fakeProcess }

func (s fakeStdin) Write(p []byte) (int, error) {
	s.f.mu.Lock()
	s.f.keys.Write(p)
	s.f.mu.Unlock()
	if strings.Contains(string(p), "q") && !s.f.stubborn {
		s.f.end(nil)
	}
	return len(p), nil
}

func (f *fakeProcess) report(t *testing.T, block string) {
	t.Helper()
	if _, err := io.WriteString(f.pw, block); err != nil {
		t.Fatal(err)
	}
}

func starter(f *fakeProcess) Starter {
	return func(context.Context, []string) (Process, error) { return f, nil }
}

func TestServedSegments(t *testing.T) {
	// Ports TranscodingJob's GetHighestServedSegmentIndexEndingAtOrBefore
	// facts.
	session := func() *Session { return &Session{segments: map[int]time.Duration{}} }
	t.Run("no segments served", func(t *testing.T) {
		if _, ok := session().HighestServedSegmentEndingBy(time.Hour); ok {
			t.Error("got = a segment, want = none")
		}
	})
	t.Run("init segment ignored", func(t *testing.T) {
		s := session()
		s.ReportSegment(-1, 0)
		if _, ok := s.HighestServedSegmentEndingBy(time.Hour); ok {
			t.Error("got = a segment, want = none")
		}
	})
	t.Run("segment ending exactly at the position", func(t *testing.T) {
		s := session()
		s.ReportSegment(0, 6*time.Second)
		s.ReportSegment(1, 12*time.Second)
		if i, ok := s.HighestServedSegmentEndingBy(12 * time.Second); !ok || i != 1 {
			t.Errorf("got = %d %v, want = 1", i, ok)
		}
		if i, ok := s.HighestServedSegmentEndingBy(11900 * time.Millisecond); !ok || i != 0 {
			t.Errorf("got = %d %v, want = 0", i, ok)
		}
	})
	t.Run("segments longer than desired", func(t *testing.T) {
		// Keyframe-based playlists average longer segments than 6 s.
		const segment = 6600 * time.Millisecond
		s := session()
		for i := range 520 {
			s.ReportSegment(i, time.Duration(i+1)*segment)
		}
		if i, ok := s.HighestServedSegmentEndingBy(520*segment - 120*time.Second); !ok || i != 500 {
			t.Errorf("got = %d %v, want = 500", i, ok)
		}
	})
}

func TestParseProgress(t *testing.T) {
	in := "frame=48\nfps=24.00\nout_time_us=2000000\ntotal_size=1024\nspeed=1.5x\nprogress=continue\n" +
		"frame=96\nout_time_us=4000000\nspeed=N/A\nprogress=end\n"
	var got []Progress
	parseProgress(strings.NewReader(in), func(p Progress) { got = append(got, p) })
	want := []Progress{
		{Position: 2 * time.Second, Frame: 48, FPS: 24, Speed: 1.5, Size: 1024},
		{Position: 4 * time.Second, Frame: 96, FPS: 24, Speed: 0, Size: 1024, Ended: true},
	}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("got = %+v, want = %+v", got, want)
	}
}

func TestFinishes(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake(false)
		s, err := Start(t.Context(), starter(f), Config{})
		if err != nil {
			t.Fatal(err)
		}
		f.report(t, "out_time_us=1000000\nprogress=end\n")
		synctest.Wait()
		if p := s.Progress(); !p.Ended || p.Position != time.Second {
			t.Errorf("progress: got = %+v", p)
		}
		f.end(nil)
		<-s.Done()
		if s.Err() != nil {
			t.Errorf("got = %v, want = nil", s.Err())
		}
	})
}

func TestFailureReported(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake(false)
		s, _ := Start(t.Context(), starter(f), Config{})
		f.end(errors.New("exit status 1"))
		<-s.Done()
		if s.Err() == nil {
			t.Error("got = nil, want = the failure")
		}
	})
}

func TestIdleReaping(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake(false)
		begin := time.Now()
		s, _ := Start(t.Context(), starter(f), Config{IdleTimeout: 30 * time.Second})
		time.Sleep(20 * time.Second)
		s.Touch()
		<-s.Done()
		// Touched at 20 s, idle from 50 s, seen at the 50 s check.
		if got := time.Since(begin); got != 50*time.Second {
			t.Errorf("reaped after: got = %v, want = 50s", got)
		}
		if !errors.Is(s.Err(), ErrIdle) || f.Keys() != "q" {
			t.Errorf("got = %v, keys %q", s.Err(), f.Keys())
		}
	})
}

func TestKillAfterGrace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake(true)
		s, _ := Start(t.Context(), starter(f), Config{StopGrace: 3 * time.Second})
		begin := time.Now()
		if err := s.Stop(); err != nil {
			t.Errorf("Stop: got = %v, want = nil", err)
		}
		if got := time.Since(begin); got != 3*time.Second {
			t.Errorf("killed after: got = %v, want = 3s", got)
		}
	})
}

func TestContextCancel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := newFake(false)
		ctx, cancel := context.WithCancelCause(t.Context())
		s, _ := Start(ctx, starter(f), Config{})
		cause := errors.New("client gone")
		cancel(cause)
		<-s.Done()
		if !errors.Is(s.Err(), cause) {
			t.Errorf("got = %v, want = %v", s.Err(), cause)
		}
	})
}

func TestThrottle(t *testing.T) {
	for _, pauseKey := range []bool{true, false} {
		synctest.Test(t, func(t *testing.T) {
			f := newFake(false)
			s, _ := Start(t.Context(), starter(f), Config{ThrottleAhead: 60 * time.Second, PauseKey: pauseKey})
			f.report(t, "out_time_us=100000000\nprogress=continue\n")
			s.ReportPosition(10 * time.Second)
			time.Sleep(5 * time.Second)
			synctest.Wait()
			if !s.Paused() {
				t.Fatal("paused: got = false")
			}
			s.ReportSegment(3, 90*time.Second)
			time.Sleep(5 * time.Second)
			synctest.Wait()
			if s.Paused() {
				t.Fatal("paused after catching up: got = true")
			}
			// A paused ffmpeg is resumed before it is asked to quit.
			f.report(t, "out_time_us=200000000\nprogress=continue\n")
			time.Sleep(5 * time.Second)
			synctest.Wait()
			if err := s.Stop(); err != nil {
				t.Fatal(err)
			}
			want := "pupuq"
			if !pauseKey {
				want = "c\nc\nq"
			}
			if f.Keys() != want {
				t.Errorf("keys: got = %q, want = %q", f.Keys(), want)
			}
		})
	}
}

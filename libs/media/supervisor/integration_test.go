package supervisor

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/tools/fixtures"
)

func TestExecFFmpeg(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	in := fixtures.Require(t, "h264_aac.mp4")
	t.Run("finishes", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.mkv")
		s, err := Start(t.Context(), Exec(ffmpeg), Config{Args: []string{"-v", "error", "-i", in, "-c", "copy", "-y", out}})
		if err != nil {
			t.Fatal(err)
		}
		<-s.Done()
		if s.Err() != nil {
			t.Fatal(s.Err())
		}
		if p := s.Progress(); !p.Ended || p.Position <= 0 {
			t.Errorf("progress: got = %+v", p)
		}
		if _, err := os.Stat(out); err != nil {
			t.Error(err)
		}
	})
	t.Run("stops gracefully", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "out.mkv")
		// -re reads at native speed, so the transcode is still running.
		s, err := Start(t.Context(), Exec(ffmpeg), Config{
			Args:      []string{"-v", "error", "-re", "-stream_loop", "-1", "-i", in, "-c", "copy", "-y", out},
			StopGrace: 5 * time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(500 * time.Millisecond)
		begin := time.Now()
		if err := s.Stop(); err != nil {
			t.Fatalf("Stop: got = %v, want = nil", err)
		}
		if d := time.Since(begin); d > 4*time.Second {
			t.Errorf("stopped after %v: ffmpeg ignored q", d)
		}
	})
	t.Run("fails", func(t *testing.T) {
		s, err := Start(t.Context(), Exec(ffmpeg), Config{Args: []string{"-v", "error", "-i", filepath.Join(t.TempDir(), "missing.mkv"), "-f", "null", "-"}})
		if err != nil {
			t.Fatal(err)
		}
		<-s.Done()
		if s.Err() == nil {
			t.Error("got = nil, want = an error")
		}
	})
}

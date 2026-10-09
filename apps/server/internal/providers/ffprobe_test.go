package providers

import (
	"os/exec"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/media/probe"
	"github.com/mavioai/mavio/tools/fixtures"
)

func TestFFprobe(t *testing.T) {
	ffprobe, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("ffprobe not found on PATH")
	}
	path := fixtures.Require(t, "chapters.mkv")
	res, err := FFprobe{Prober: &probe.Prober{FFprobe: ffprobe}}.Probe(t.Context(), path, false)
	if err != nil {
		t.Fatal(err)
	}
	src := res.Source
	if src.Container == "" || src.Duration <= 0 || len(src.Chapters) == 0 {
		t.Errorf("source = %+v, want = container, duration and chapters", src)
	}
	if len(src.Streams) == 0 || src.Streams[0].Kind != core.StreamVideo {
		t.Errorf("streams = %+v, want = a video stream first", src.Streams)
	}
}

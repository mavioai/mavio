package probe

import (
	"os/exec"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/tools/fixtures"
)

// prober returns a prober for the ffprobe on PATH, skipping the test when
// there is none.
func prober(t *testing.T) *Prober {
	t.Helper()
	path, err := exec.LookPath("ffprobe")
	if err != nil {
		t.Skip("integration test: ffprobe not found on PATH")
	}
	return &Prober{FFprobe: path}
}

func TestProbeFixtures(t *testing.T) {
	p := prober(t)
	ctx := t.Context()
	tests := []struct {
		fixture string
		audio   bool
		check   func(t *testing.T, r Result)
	}{
		{"h264_aac.mp4", false, func(t *testing.T, r Result) {
			v := video(t, r)
			check(t, "video", [3]any{v.Codec, v.Width, v.Height}, [3]any{"h264", 640, 360})
			check(t, "range", v.VideoRangeType(), core.RangeTypeSDR)
			check(t, "audio", r.Source.Streams[1].Codec, "aac")
			if r.Source.Duration < 1900*time.Millisecond || r.Source.Bitrate <= 0 {
				t.Errorf("source: got = %+v", r.Source)
			}
		}},
		{"hevc_main10_hdr10.mkv", false, func(t *testing.T, r Result) {
			v := video(t, r)
			check(t, "container", r.Source.Container, "mkv")
			check(t, "bit depth", v.BitDepth, 10)
			check(t, "range", v.VideoRangeType(), core.RangeTypeHDR10)
			check(t, "channels", r.Source.Streams[1].Channels, 6)
		}},
		{"mpeg2_interlaced.ts", false, func(t *testing.T, r Result) {
			check(t, "container", r.Source.Container, "ts")
			check(t, "interlaced", video(t, r).Interlaced, true)
		}},
		{"multi_track.mkv", false, func(t *testing.T, r Result) {
			s := r.Source.Streams
			check(t, "streams", len(s), 5)
			check(t, "audio titles", [2]string{s[1].Title, s[2].Title}, [2]string{"English Stereo", "Japanese 5.1"})
			check(t, "forced subtitle", s[4].Forced, true)
			check(t, "text subtitles", s[3].IsTextSubtitle() && s[4].IsTextSubtitle(), true)
		}},
		{"ntsc_film_odd_duration.mp4", false, func(t *testing.T, r Result) {
			check(t, "frame rate", video(t, r).RealFrameRate, core.Rational{Num: 24000, Den: 1001})
		}},
		{"chapters.mkv", false, func(t *testing.T, r Result) {
			check(t, "chapters", len(r.Source.Chapters), 2)
			if len(r.Source.Chapters) == 2 && (r.Source.Chapters[0].Title == "" || r.Source.Chapters[1].Start <= 0) {
				t.Errorf("chapters: got = %+v", r.Source.Chapters)
			}
		}},
		{"tagged.flac", true, func(t *testing.T, r Result) {
			md := r.Metadata
			check(t, "name", md.Name, "Test Tone")
			check(t, "album", md.Album, "Fixtures")
			check(t, "year", md.ProductionYear, 2026)
			if md.IndexNumber == nil || *md.IndexNumber != 1 || len(md.AlbumArtists) != 1 {
				t.Errorf("tags: got = %+v", md)
			}
			check(t, "bitrate", r.Source.Streams[0].Bitrate > 0, true)
		}},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			path := fixtures.Require(t, tt.fixture)
			r, err := p.Probe(ctx, Request{Path: path, Audio: tt.audio, Chapters: true})
			if err != nil {
				t.Fatal(err)
			}
			tt.check(t, r)
		})
	}
}

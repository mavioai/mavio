package voice

import (
	"math"
	"testing"
	"time"
)

func TestCapture(t *testing.T) {
	tests := []struct {
		name   string
		rate   float64
		in     int
		frames int
	}{
		{"16 kHz", 16000, 16000, 50},
		{"48 kHz", 48000, 48000, 50},
		{"44.1 kHz", 44100, 44100, 50},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var frames [][]int16
			c := newCapture(tt.rate, func(f []int16) { frames = append(frames, f) })
			// A 440 Hz tone at half scale, written in uneven pieces.
			in := make([]float32, tt.in)
			for i := range in {
				in[i] = float32(0.5 * math.Sin(2*math.Pi*440*float64(i)/tt.rate))
			}
			for len(in) > 0 {
				n := min(len(in), 471)
				c.write(in[:n])
				in = in[n:]
			}
			if got := len(frames); got < tt.frames-1 || got > tt.frames {
				t.Fatalf("frames = %d, want = %d", got, tt.frames)
			}
			peak := 0
			for _, f := range frames {
				if len(f) != FrameSamples {
					t.Fatalf("frame samples = %d, want = %d", len(f), FrameSamples)
				}
				for _, s := range f {
					peak = max(peak, int(s), -int(s))
				}
			}
			if peak < 15000 || peak > 16500 {
				t.Errorf("peak = %d, want = about %d", peak, 0x7fff/2)
			}
			if l := c.meter.level(); l < 0.5 || l > 1 {
				t.Errorf("level = %v, want = between 0.5 and 1", l)
			}
		})
	}
}

func TestPlayback(t *testing.T) {
	var p playback
	// A second at 24 kHz plays as one at 48 kHz.
	pcm := make([]int16, 2400)
	for i := range pcm {
		pcm[i] = 8000
	}
	for range 10 {
		p.push(pcm, 24000)
	}
	if played, total := p.progress(); played != 0 || total < 999*time.Millisecond || total > time.Second {
		t.Fatalf("progress = %v, %v, want = 0, 1s", played, total)
	}
	out := make([]float32, 12000)
	p.read(out)
	if got, want := out[len(out)-1], float32(8000)/0x8000; math.Abs(float64(got-want)) > 1e-4 {
		t.Errorf("sample = %v, want = %v", got, want)
	}
	if played, _ := p.progress(); played != 250*time.Millisecond {
		t.Errorf("played = %v, want = 250ms", played)
	}
	if p.meter.level() == 0 {
		t.Errorf("level = 0, want > 0")
	}
	// An interrupt drops the rest, telling how much played.
	if got := p.drop(); got != 250*time.Millisecond {
		t.Errorf("drop = %v, want = 250ms", got)
	}
	p.read(out)
	if out[0] != 0 || p.meter.level() != 0 {
		t.Errorf("after drop: sample = %v, level = %v, want = silence", out[0], p.meter.level())
	}
	// A run counts from its start.
	p.push(pcm, 48000)
	p.read(out[:1200])
	p.run()
	if played, total := p.progress(); played != 0 || total != 25*time.Millisecond {
		t.Errorf("progress = %v, %v, want = 0, 25ms", played, total)
	}
}

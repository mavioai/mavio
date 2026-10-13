package subsync

import (
	"encoding/json"
	"errors"
	"math"
	"math/cmplx"
	"math/rand/v2"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/media/vad"
	"github.com/mavioai/mavio/libs/subtitle"
	"github.com/mavioai/mavio/tools/fixtures"
)

func readCues(t testing.TB, path string) []subtitle.Cue {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := subtitle.Parse(data, subtitle.SRT)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return s.Cues
}

// TestMatchesFfsubsync compares Align with the alignments ffsubsync finds
// with Bazarr's arguments (testdata/golden.py).
func TestMatchesFfsubsync(t *testing.T) {
	data, err := os.ReadFile("testdata/cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name    string   `json:"name"`
		Args    []string `json:"args"`
		Success bool     `json:"success"`
		Offset  float64  `json:"offset"`
		Scale   float64  `json:"scale"`
		Score   float64  `json:"score"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			ref := FromCues(readCues(t, filepath.Join("testdata/cases", tc.Name+".ref.srt")), 1)
			cues := readCues(t, filepath.Join("testdata/cases", tc.Name+".srt"))
			opt := Options{
				MaxOffset:      time.Minute,
				FixFramerate:   !slices.Contains(tc.Args, "--no-fix-framerate"),
				GoldenSection:  slices.Contains(tc.Args, "--gss"),
				InferFramerate: true,
			}
			got, err := Align(t.Context(), ref, cues, opt)
			if err != nil {
				t.Fatalf("Align: %v", err)
			}
			if want := time.Duration(math.Round(tc.Offset*SampleRate)) * time.Second / SampleRate; got.Offset != want {
				t.Errorf("offset: got = %v, want = %v", got.Offset, want)
			}
			if math.Abs(got.Scale-tc.Scale) > 1e-9 {
				t.Errorf("scale: got = %v, want = %v", got.Scale, tc.Scale)
			}
			if math.Abs(got.Score-tc.Score) > 1e-6*math.Abs(tc.Score) {
				t.Errorf("score: got = %v, want = %v", got.Score, tc.Score)
			}
			quality := Quality{MinScore: 0}
			if opt.FixFramerate {
				quality = DefaultQuality
			}
			if ok := len(got.Problems(quality)) == 0; ok != tc.Success {
				t.Errorf("success: got = %v (%v), want = %v", ok, got.Problems(quality), tc.Success)
			}
		})
	}
}

func TestAlignWithoutAlignment(t *testing.T) {
	cues := []subtitle.Cue{{Start: 100 * time.Second, End: 102 * time.Second, Text: "Hello."}}
	ref := FromCues([]subtitle.Cue{{Start: 10 * time.Second, End: 12 * time.Second, Text: "Hello."}}, 1)
	for _, tc := range []struct {
		name string
		ref  Speech
		cues []subtitle.Cue
	}{
		{"no reference", nil, cues},
		{"no cues", ref, nil},
	} {
		if _, err := Align(t.Context(), tc.ref, tc.cues, Options{}); !errors.Is(err, ErrNoAlignment) {
			t.Errorf("%s: err = %v, want = %v", tc.name, err, ErrNoAlignment)
		}
	}
	// Within the bounds the speech never meets, yet an offset is found.
	got, err := Align(t.Context(), ref, cues, Options{MaxOffset: time.Minute})
	if err != nil {
		t.Fatalf("Align: %v", err)
	}
	if got.Offset.Abs() > time.Minute {
		t.Errorf("offset: got = %v, want at most a minute", got.Offset)
	}
	got, err = Align(t.Context(), ref, cues, Options{})
	if err != nil || got.Offset != -90*time.Second {
		t.Errorf("unbounded: got = %v, %v, want = -1m30s", got.Offset, err)
	}
}

func TestFFT(t *testing.T) {
	rnd := rand.New(rand.NewPCG(1, 2))
	for _, n := range []int{1, 2, 8, 64, 1024} {
		x := make([]complex128, n)
		for i := range x {
			x[i] = complex(rnd.Float64()-0.5, rnd.Float64()-0.5)
		}
		got := slices.Clone(x)
		fft(got, false)
		for k := range n {
			var want complex128
			for j, v := range x {
				want += v * cmplx.Rect(1, -2*math.Pi*float64(j*k)/float64(n))
			}
			if cmplx.Abs(got[k]-want) > 1e-9 {
				t.Fatalf("n=%d, k=%d: got = %v, want = %v", n, k, got[k], want)
			}
		}
		fft(got, true)
		for i := range got {
			if cmplx.Abs(got[i]/complex(float64(n), 0)-x[i]) > 1e-12 {
				t.Fatalf("n=%d: inverse at %d: got = %v, want = %v", n, i, got[i]/complex(float64(n), 0), x[i])
			}
		}
	}
}

func TestCorrelate(t *testing.T) {
	rnd := rand.New(rand.NewPCG(3, 4))
	for _, size := range [][2]int{{1, 1}, {7, 3}, {50, 80}, {300, 120}} {
		ref, sub := make(Speech, size[0]), make(Speech, size[1])
		for i := range ref {
			ref[i] = rnd.Float32()
		}
		for i := range sub {
			sub[i] = rnd.Float32()
		}
		c := correlate(ref, sub)
		for o := -(len(sub) - 1); o < len(ref); o++ {
			var want float64
			for t := range ref {
				if j := t - o; j >= 0 && j < len(sub) {
					want += (2*float64(ref[t]) - 1) * (2*float64(sub[j]) - 1)
				}
			}
			if got := c.at(o); math.Abs(got-want) > 1e-9 {
				t.Fatalf("%v, offset %d: got = %v, want = %v", size, o, got, want)
			}
		}
	}
}

func TestIsMetadata(t *testing.T) {
	for _, tc := range []struct {
		text        string
		firstOrLast bool
		want        bool
	}{
		{"", false, true},
		{"  <i> </i> ", false, true},
		{"[MUSIC PLAYING]", false, true},
		{"<i>[door slams]</i>", false, true},
		{"（笑声）", false, true},
		{"【音乐】", false, true},
		{"♪ ♪", false, true},
		{"♪ La la la ♪", false, false},
		{"<i>Hello?</i>", false, false},
		{"(whispers) Come here.", false, false},
		{"{\\an8}Hello.", false, false},
		{"English subtitles by someone", true, true},
		{"English subtitles by someone", false, false},
		{"Synced by X - www.example.org", true, true},
		{"Synced by X - www.example.org", false, false},
		{"I speak English.", true, true},
	} {
		if got := isMetadata(tc.text, tc.firstOrLast); got != tc.want {
			t.Errorf("isMetadata(%q, %v): got = %v, want = %v", tc.text, tc.firstOrLast, got, tc.want)
		}
	}
}

func TestFromCues(t *testing.T) {
	cues := []subtitle.Cue{
		{Start: 0, End: 2 * time.Second, Text: "Subtitles by X - English"},
		{Start: 1 * time.Second, End: 1050 * time.Millisecond, Text: "Hi."},
		{Start: 3 * time.Second, End: 30 * time.Second, Text: "A long speech."},
		{Start: 14 * time.Second, End: 15 * time.Second, Text: "[applause]"},
		{Start: 16 * time.Second, End: 16500 * time.Millisecond, Text: "The end."},
	}
	got := FromCues(cues, 1)
	// The long speech counts for 10 s, so the last cue ends the speech.
	if want := 1650 + 2; len(got) != want {
		t.Fatalf("length: got = %d, want = %d", len(got), want)
	}
	want := make(Speech, len(got))
	for _, r := range [][2]int{{100, 105}, {300, 1300}, {1600, 1650}} {
		for i := r[0]; i < r[1]; i++ {
			want[i] = 1
		}
	}
	if !slices.Equal(got, want) {
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("sample %d: got = %v, want = %v", i, got[i], want[i])
			}
		}
	}
	// Scaled up, speech is longer and weighs less.
	scaled := FromCues(cues[1:2], 2)
	if len(scaled) != 212 || scaled[200] != 0.5 || scaled[209] != 0.5 || scaled[210] != 0 {
		t.Errorf("scaled by 2: got = %v", scaled[195:])
	}
}

func TestFromProbabilities(t *testing.T) {
	got := FromProbabilities([]float32{0.1, 0.9, 0.3})
	// 3 windows of 32 ms are 96 ms: 10 samples, each the window of its
	// middle.
	want := Speech{0.1, 0.1, 0.1, 0.9, 0.9, 0.9, 0.3, 0.3, 0.3, 0.3}
	if !slices.Equal(got, want) {
		t.Errorf("got = %v, want = %v", got, want)
	}
}

func TestProblems(t *testing.T) {
	for _, tc := range []struct {
		r    Result
		want int
	}{
		{Result{Offset: 3 * time.Second, Scale: 1, Score: 10}, 0},
		{Result{Offset: -3 * time.Second, Scale: 1, Score: -1}, 1},
		{Result{Offset: -31 * time.Second, Scale: 1.2, Score: -1}, 3},
	} {
		if got := tc.r.Problems(DefaultQuality); len(got) != tc.want {
			t.Errorf("%+v: got = %v, want %d problems", tc.r, got, tc.want)
		}
	}
	if got := (Result{Offset: time.Hour, Scale: 2}).Problems(Quality{}); len(got) != 0 {
		t.Errorf("without bounds: got = %v, want none", got)
	}
}

func TestApply(t *testing.T) {
	cues := []subtitle.Cue{
		{Index: 1, Start: time.Second, End: 2 * time.Second, Text: "a"},
		{Index: 2, Start: 10 * time.Second, End: 12 * time.Second, Text: "b"},
	}
	got := Result{Offset: -1500 * time.Millisecond, Scale: 25 / 23.976}.Apply(cues)
	want := []subtitle.Cue{
		{Index: 1, Start: 0, End: 585419 * time.Microsecond, Text: "a"},
		{Index: 2, Start: 8927094 * time.Microsecond, End: 11012513 * time.Microsecond, Text: "b"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got = %v, want = %v", got, want)
	}
	if cues[0].Start != time.Second {
		t.Error("Apply changed its input")
	}
}

// TestAudioSpeech needs ffmpeg and the speech fixture: subtitles timed by
// the recording's speech, late by 2.5 s, are put back in place.
func TestAudioSpeech(t *testing.T) {
	ffmpeg, err := exec.LookPath("ffmpeg")
	if err != nil {
		t.Skip("integration test: ffmpeg not found on PATH")
	}
	input := fixtures.Require(t, "speech.wav")
	m, err := vad.NewModel()
	if err != nil {
		t.Fatal(err)
	}
	speech, err := AudioSpeech(t.Context(), ffmpeg, input, -1, m, 0)
	if err != nil {
		t.Fatalf("AudioSpeech: %v", err)
	}
	if want := 60 * SampleRate; math.Abs(float64(len(speech)-want)) > 4 {
		t.Fatalf("length: got = %d samples, want about %d", len(speech), want)
	}
	cues := cuesOf(speech, 2500*time.Millisecond)
	if len(cues) < 10 {
		t.Fatalf("got %d cues from the speech, want at least 10", len(cues))
	}
	got, err := Align(t.Context(), speech, cues, Options{MaxOffset: time.Minute})
	if err != nil {
		t.Fatalf("Align: %v", err)
	}
	if want := -2500 * time.Millisecond; (got.Offset-want).Abs() > 20*time.Millisecond || got.Scale != 1 {
		t.Errorf("got = %+v, want offset %v, scale 1", got, want)
	}
	if len(got.Problems(DefaultQuality)) != 0 {
		t.Errorf("problems: %v", got.Problems(DefaultQuality))
	}
	if _, err := AudioSpeech(t.Context(), ffmpeg, filepath.Join(t.TempDir(), "missing.wav"), -1, m, 0); err == nil {
		t.Error("AudioSpeech of a missing file: err = nil, want an error")
	}
}

// cuesOf times cues as a subtitler might from speech: runs above one half,
// pauses under 0.4 s bridged, rounded to 100 ms and shifted by shift.
func cuesOf(s Speech, shift time.Duration) []subtitle.Cue {
	var cues []subtitle.Cue
	at := func(i int) time.Duration {
		return (time.Duration(i) * time.Second / SampleRate).Round(100*time.Millisecond) + shift
	}
	start, last := -1, -100
	for i, v := range s {
		if v <= 0.5 {
			continue
		}
		if start >= 0 && i-last > 40 {
			cues = append(cues, subtitle.Cue{Start: at(start), End: at(last + 1), Text: "Speech."})
			start = -1
		}
		if start < 0 {
			start = i
		}
		last = i
	}
	if start >= 0 {
		cues = append(cues, subtitle.Cue{Start: at(start), End: at(last + 1), Text: "Speech."})
	}
	return cues
}

func BenchmarkAlignFilm(b *testing.B) {
	// Two hours of reference against 1,600 cues, at the three ratios
	// Bazarr's defaults try at most.
	rnd := rand.New(rand.NewPCG(5, 6))
	var cues []subtitle.Cue
	at := 10 * time.Second
	for range 1600 {
		d := time.Duration(800+rnd.IntN(4000)) * time.Millisecond
		cues = append(cues, subtitle.Cue{Start: at, End: at + d, Text: "Line."})
		at += d + time.Duration(200+rnd.IntN(4000))*time.Millisecond
	}
	ref := FromCues(Result{Offset: 7 * time.Second, Scale: 1}.Apply(cues), 1)
	for b.Loop() {
		if _, err := Align(b.Context(), ref, cues, Options{MaxOffset: time.Minute, InferFramerate: true}); err != nil {
			b.Fatal(err)
		}
	}
}

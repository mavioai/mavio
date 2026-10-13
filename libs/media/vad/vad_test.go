package vad

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math"
	"os"
	"testing"

	"github.com/mavioai/mavio/tools/fixtures"
)

// golden holds the probabilities onnxruntime gives with Silero's model
// (testdata/golden.py).
type golden struct {
	Noise  []float32 `json:"noise"`
	Speech []float32 `json:"speech"`
}

func readGolden(t testing.TB) golden {
	t.Helper()
	data, err := os.ReadFile("testdata/probabilities.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

// noise returns n samples of the noise golden.py's noise generates: an LCG
// at four levels changing every half second.
func noise(n int) []int16 {
	s := uint32(1)
	out := make([]int16, n)
	for i := range out {
		s = s*1664525 + 1013904223
		v := int32(s>>16) - 32768
		out[i] = int16(v >> (1 + (i/8000)%4))
	}
	return out
}

// speech returns the samples of the speech fixture, Silero's test
// recording.
func speech(t testing.TB) []int16 {
	t.Helper()
	data, err := os.ReadFile(fixtures.Require(t, "speech.wav"))
	if err != nil {
		t.Fatal(err)
	}
	i := bytes.Index(data, []byte("data"))
	if i < 0 {
		t.Fatal("speech.wav has no data chunk")
	}
	data = data[i+8:]
	out := make([]int16, len(data)/2)
	for j := range out {
		out[j] = int16(binary.LittleEndian.Uint16(data[2*j:]))
	}
	return out
}

func pcm(samples []int16) []byte {
	out := make([]byte, 2*len(samples))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(out[2*i:], uint16(s))
	}
	return out
}

// tolerance is how far the probabilities may be from onnxruntime's, whose
// float32 sums run in another order.
const tolerance = 1e-5

func compare(t *testing.T, got, want []float32) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d probabilities, want %d", len(got), len(want))
	}
	worst := 0.0
	for i := range got {
		d := math.Abs(float64(got[i] - want[i]))
		worst = max(worst, d)
		if d > tolerance {
			t.Errorf("window %d: got = %v, want = %v", i, got[i], want[i])
		}
	}
	t.Logf("largest difference %.2g over %d windows", worst, len(got))
}

func newModel(t testing.TB) *Model {
	t.Helper()
	m, err := NewModel()
	if err != nil {
		t.Fatalf("NewModel: %v", err)
	}
	return m
}

func TestDetectorMatchesOnnxruntime(t *testing.T) {
	m := newModel(t)
	g := readGolden(t)
	for _, tc := range []struct {
		name    string
		samples func(testing.TB) []int16
		want    []float32
	}{
		{"noise", func(testing.TB) []int16 { return noise(40000) }, g.Noise},
		{"speech", speech, g.Speech},
	} {
		t.Run(tc.name, func(t *testing.T) {
			samples := tc.samples(t)
			d := m.NewDetector()
			var got []float32
			window := make([]float32, WindowSize)
			for i := 0; i < len(samples); i += WindowSize {
				clear(window)
				for j := 0; j < WindowSize && i+j < len(samples); j++ {
					window[j] = float32(samples[i+j]) / 32768
				}
				p, err := d.Process(window)
				if err != nil {
					t.Fatal(err)
				}
				got = append(got, p)
			}
			compare(t, got, tc.want)
		})
	}
}

func TestDetectMatchesOnnxruntime(t *testing.T) {
	m := newModel(t)
	g := readGolden(t)
	for _, workers := range []int{1, 3} {
		got, err := m.Detect(t.Context(), bytes.NewReader(pcm(noise(40000))), workers)
		if err != nil {
			t.Fatalf("Detect: %v", err)
		}
		compare(t, got, g.Noise)
	}
	// More than one block, so that the context and state cross blocks.
	samples := speech(t)
	got, err := m.Detect(t.Context(), bytes.NewReader(pcm(samples)), 0)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	if len(samples) <= blockWindows*WindowSize {
		t.Fatalf("speech fixture of %d samples fits in one block", len(samples))
	}
	compare(t, got, g.Speech)
}

func TestDetectorRejectsWrongWindows(t *testing.T) {
	d := newModel(t).NewDetector()
	if _, err := d.Process(make([]float32, WindowSize-1)); err == nil {
		t.Error("Process of a short window: err = nil, want an error")
	}
}

func TestDetectorReset(t *testing.T) {
	m := newModel(t)
	d := m.NewDetector()
	window := make([]float32, WindowSize)
	for i, s := range noise(WindowSize) {
		window[i] = float32(s) / 32768
	}
	first, _ := d.Process(window)
	_, _ = d.Process(window)
	d.Reset()
	again, _ := d.Process(window)
	if again != first {
		t.Errorf("after Reset got = %v, want = %v", again, first)
	}
}

func TestParseWeightsRejectsBadData(t *testing.T) {
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{"empty", nil},
		{"magic", []byte("XXXX\x0f\x00\x00\x00")},
		{"truncated", weights[:len(weights)/2]},
		{"trailing", append(bytes.Clone(weights), 0)},
	} {
		if _, err := parseWeights(tc.data); err == nil {
			t.Errorf("%s: err = nil, want an error", tc.name)
		}
	}
}

func BenchmarkDetect(b *testing.B) {
	m := newModel(b)
	data := pcm(noise(SampleRate * 60))
	for b.Loop() {
		if _, err := m.Detect(b.Context(), bytes.NewReader(data), 0); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Seconds())/float64(b.N)/60, "s/audio-s")
}

func BenchmarkDetectOneWorker(b *testing.B) {
	m := newModel(b)
	data := pcm(noise(SampleRate * 60))
	for b.Loop() {
		if _, err := m.Detect(b.Context(), bytes.NewReader(data), 1); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(b.Elapsed().Seconds())/float64(b.N)/60, "s/audio-s")
}

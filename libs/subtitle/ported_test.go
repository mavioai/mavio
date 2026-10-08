package subtitle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/unicode"
)

// Test cases ported from Jellyfin by tools/testport live in
// testdata/cases, test files in testdata/subtitles; see docs/testing.md §2.

type caseFile struct {
	Theories []struct {
		Method string `json:"method"`
		Cases  []struct {
			ID   string                     `json:"id"`
			Skip string                     `json:"skip"`
			Args map[string]json.RawMessage `json:"args"`
		} `json:"cases"`
	} `json:"theories"`
	Facts []struct {
		Method string `json:"method"`
	} `json:"facts"`
	Unsupported []struct {
		Method string `json:"method"`
	} `json:"unsupported"`
}

type ported struct {
	run   map[string]func(t *testing.T, args map[string]json.RawMessage)
	skip  map[string]string
	facts map[string]string // fact or unsupported theory → Go test porting it
}

func portedCases(t *testing.T, name string, p ported) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "cases", name))
	if err != nil {
		t.Fatal(err)
	}
	var f caseFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	for _, th := range f.Theories {
		check, ok := p.run[th.Method]
		reason, skipped := p.skip[th.Method]
		if !ok && !skipped {
			t.Errorf("%s: theory %s is neither run nor skipped", name, th.Method)
			continue
		}
		for _, c := range th.Cases {
			t.Run(c.ID, func(t *testing.T) {
				switch {
				case c.Skip != "":
					t.Skip("skipped in Jellyfin: " + c.Skip)
				case skipped:
					t.Skip(reason)
				}
				check(t, c.Args)
			})
		}
	}
	for _, m := range append(f.Facts, f.Unsupported...) {
		if p.facts[m.Method] == "" && p.skip[m.Method] == "" {
			t.Errorf("%s: %s is neither ported nor skipped", name, m.Method)
		}
	}
}

func parseFile(t *testing.T, name, format string) *Subtitle {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "subtitles", name))
	if err != nil {
		t.Fatal(err)
	}
	s, err := Parse(data, format)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// dur parses a .NET TimeSpan such as "00:02:17.440".
func dur(t *testing.T, s string) time.Duration {
	t.Helper()
	var h, m int
	var sec float64
	if _, err := fmt.Sscanf(s, "%d:%d:%f", &h, &m, &sec); err != nil {
		t.Fatal(err)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute + time.Duration(sec*1000+0.5)*time.Millisecond
}

func checkCue(t *testing.T, got Cue, index int, start, end time.Duration, text string) {
	t.Helper()
	if got.Index != index || got.Start != start || got.End != end || got.Text != text {
		t.Errorf("cue: got = %d %v–%v %q, want = %d %v–%v %q", got.Index, got.Start, got.End, got.Text, index, start, end, text)
	}
}

func TestPortedAssParser(t *testing.T) {
	portedCases(t, "ass_parser.json", ported{facts: map[string]string{"Parse_Valid_Success": "TestParseExampleASS"}})
}

func TestParseExampleASS(t *testing.T) {
	s := parseFile(t, "example.ass", "ass")
	if len(s.Cues) != 1 {
		t.Fatalf("cues: got = %d, want = 1", len(s.Cues))
	}
	checkCue(t, s.Cues[0], 1, dur(t, "00:00:01.18"), dur(t, "00:00:06.85"),
		"{\\pos(400,570)}Like an Angel with pity on nobody\nThe second line in subtitle")
}

func TestPortedSrtParser(t *testing.T) {
	portedCases(t, "srt_parser.json", ported{facts: map[string]string{
		"Parse_Valid_Success":                   "TestParseExampleSRT",
		"Parse_EmptyNewlineBetweenText_Success": "TestParseExampleSRT",
	}})
}

func TestParseExampleSRT(t *testing.T) {
	s := parseFile(t, "example.srt", "srt")
	if len(s.Cues) != 2 {
		t.Fatalf("cues: got = %d, want = 2", len(s.Cues))
	}
	checkCue(t, s.Cues[0], 1, dur(t, "00:02:17.440"), dur(t, "00:02:20.375"), "Senator, we're making\nour final approach into Coruscant.")
	checkCue(t, s.Cues[1], 2, dur(t, "00:02:20.476"), dur(t, "00:02:22.501"), "Very good, Lieutenant.")

	// Blank lines inside the text do not end a cue.
	s = parseFile(t, "example2.srt", "srt")
	if len(s.Cues) != 2 {
		t.Fatalf("cues: got = %d, want = 2", len(s.Cues))
	}
	checkCue(t, s.Cues[0], 311, dur(t, "00:16:46.465"), dur(t, "00:16:49.009"), "Una vez que la gente se entere\n\nde que ustedes están aquí,")
	checkCue(t, s.Cues[1], 312, dur(t, "00:16:49.092"), dur(t, "00:16:51.470"), "este lugar se convertirá\n\nen un maldito zoológico.")
}

func TestPortedSsaParser(t *testing.T) {
	portedCases(t, "ssa_parser.json", ported{
		run: map[string]func(*testing.T, map[string]json.RawMessage){
			"Parse_MultipleDialogues_Success": func(t *testing.T, args map[string]json.RawMessage) {
				var ssa string
				var want struct {
					Items []struct {
						Args []string `json:"args"`
						Init struct {
							StartPositionTicks int64
							EndPositionTicks   int64
						} `json:"init"`
					} `json:"items"`
				}
				if err := json.Unmarshal(args["ssa"], &ssa); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(args["expectedSubtitleTrackEvents"], &want); err != nil {
					t.Fatal(err)
				}
				s, err := Parse([]byte(ssa), "ssa")
				if err != nil {
					t.Fatal(err)
				}
				if len(s.Cues) != len(want.Items) {
					t.Fatalf("cues: got = %d, want = %d", len(s.Cues), len(want.Items))
				}
				for i, w := range want.Items {
					c := s.Cues[i]
					if fmt.Sprint(c.Index) != w.Args[0] || c.Text != w.Args[1] ||
						ticks(c.Start) != w.Init.StartPositionTicks || ticks(c.End) != w.Init.EndPositionTicks {
						t.Errorf("cue %d: got = %+v, want = %+v", i, c, w)
					}
				}
			},
		},
		facts: map[string]string{"Parse_Valid_Success": "TestParseExampleSSA"},
	})
}

func TestParseExampleSSA(t *testing.T) {
	s := parseFile(t, "example.ssa", "ssa")
	if len(s.Cues) != 1 {
		t.Fatalf("cues: got = %d, want = 1", len(s.Cues))
	}
	checkCue(t, s.Cues[0], 1, dur(t, "00:00:01.18"), dur(t, "00:00:06.85"), "{\\pos(400,570)}Like an angel with pity on nobody")
}

func TestPortedSubtitleEncoder(t *testing.T) {
	portedCases(t, "subtitle_encoder.json", ported{
		skip: map[string]string{
			"GetReadableFile_Valid_Success": "locates and extracts subtitle files of a media source; that is the media pipeline's job, not this pure library's",
		},
		facts: map[string]string{
			"GetSubtitleStream_NonUtf8LocalFile_ConvertedToUtf8":       "TestToUTF8",
			"GetSubtitleStream_Utf8LocalFile_PreservesContent":         "TestToUTF8",
			"ConvertSubtitles_SequentialCalls_AreDeterministic":        "TestConvertDeterministic",
			"ConvertSubtitles_ConcurrentCalls_MatchSequentialBaseline": "TestConvertDeterministic",
		},
	})
}

// greekText needs a legacy encoding; its accented letters have the same
// code points in windows-1253 and iso-8859-7, so confusing the two still
// round-trips.
const greekText = "Καλημέρα κόσμε, αυτό είναι ένας υπότιτλος."

func greekSRT() string {
	var b strings.Builder
	for i := 1; i <= 8; i++ {
		fmt.Fprintf(&b, "%d\n00:00:0%d,000 --> 00:00:0%d,000\n%s\nΗ γρήγορη καφέ αλεπού πηδάει πάνω από το τεμπέλικο σκυλί.\n\n", i, i, i+1, greekText)
	}
	return b.String()
}

func TestToUTF8(t *testing.T) {
	src := greekSRT()
	for name, enc := range map[string]encoding.Encoding{
		"windows-1253": charmap.Windows1253,
		"iso-8859-7":   charmap.ISO8859_7,
		"utf-16le BOM": unicode.UTF16(unicode.LittleEndian, unicode.UseBOM),
	} {
		data, err := enc.NewEncoder().Bytes([]byte(src))
		if err != nil {
			t.Fatal(err)
		}
		out, charset, err := ToUTF8(data)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if text := string(out); !strings.Contains(text, greekText) || strings.ContainsAny(text, "�?") {
			t.Errorf("%s (detected %s): got = %q", name, charset, text[:min(len(text), 120)])
		}
	}
	// UTF-8 is returned unchanged.
	out, charset, err := ToUTF8([]byte(src))
	if err != nil || charset != "utf-8" || !bytes.Equal(out, []byte(src)) {
		t.Errorf("utf-8: got = %s %v, changed = %v", charset, err, !bytes.Equal(out, []byte(src)))
	}
}

const (
	streamCount = 8
	cueCount    = 500
)

func generatedSRT(stream int) []byte {
	var b strings.Builder
	for i := range cueCount {
		start := time.Duration(i*4) * time.Second
		fmt.Fprintf(&b, "%d\n%s --> %s\nS%dC%d\n\n", i+1, timestamp(start, ","), timestamp(start+2*time.Second, ","), stream, i)
	}
	return []byte(b.String())
}

func TestConvertDeterministic(t *testing.T) {
	convert := func(stream int) string {
		out, err := Convert(generatedSRT(stream), "srt", "vtt", 0, 0, false)
		if err != nil {
			t.Error(err)
		}
		return string(out)
	}
	baseline := make([]string, streamCount)
	for i := range streamCount {
		baseline[i] = convert(i)
		if !strings.Contains(baseline[i], fmt.Sprintf("S%dC%d", i, cueCount-1)) {
			t.Errorf("stream %d: last cue missing", i)
		}
		if again := convert(i); again != baseline[i] {
			t.Errorf("stream %d: sequential conversions differ", i)
		}
	}
	for range 10 {
		var wg sync.WaitGroup
		results := make([]string, streamCount)
		for i := range streamCount {
			wg.Go(func() { results[i] = convert(i) })
		}
		wg.Wait()
		for i := range streamCount {
			if results[i] != baseline[i] {
				t.Errorf("stream %d: concurrent conversion differs (%d vs %d bytes)", i, len(results[i]), len(baseline[i]))
			}
		}
	}
}

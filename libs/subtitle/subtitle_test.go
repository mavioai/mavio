package subtitle

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

func ms(n int) time.Duration { return time.Duration(n) * time.Millisecond }

func TestParseVTT(t *testing.T) {
	src := "WEBVTT - title\n\nNOTE a comment\nspanning lines\n\nSTYLE\n::cue { color: red }\n\n" +
		"intro\n00:01.000 --> 00:04.500 align:start\n<i>Hello</i>\nworld\n\n" +
		"7\n01:00:00.250 --> 01:00:01.000\nLast\n"
	s, err := Parse([]byte(src), "webvtt")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Cues) != 2 {
		t.Fatalf("cues: got = %d, want = 2", len(s.Cues))
	}
	checkCue(t, s.Cues[0], 1, ms(1000), ms(4500), "<i>Hello</i>\nworld")
	checkCue(t, s.Cues[1], 7, time.Hour+ms(250), time.Hour+ms(1000), "Last")
}

func TestParseSRTVariants(t *testing.T) {
	// A byte order mark, CRLF endings, a missing cue number and a dot
	// before the milliseconds.
	src := "\ufeff1\r\n00:00:01,000 --> 00:00:02,000\r\nOne\r\n\r\n00:00:03.000 --> 00:00:04,000 X1:0\r\nTwo\r\n\r\n\r\n"
	s, err := Parse([]byte(src), ".SRT")
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Cues) != 2 {
		t.Fatalf("cues: got = %d, want = 2", len(s.Cues))
	}
	checkCue(t, s.Cues[0], 1, ms(1000), ms(2000), "One")
	checkCue(t, s.Cues[1], 2, ms(3000), ms(4000), "Two")
}

func TestParseErrors(t *testing.T) {
	if _, err := Parse([]byte("x"), "sub"); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("format: got = %v, want = ErrUnsupportedFormat", err)
	}
	if _, err := Parse([]byte("not a subtitle"), "srt"); !errors.Is(err, ErrNoCues) {
		t.Errorf("empty: got = %v, want = ErrNoCues", err)
	}
	if _, err := Write(&Subtitle{}, "sub"); !errors.Is(err, ErrUnsupportedFormat) {
		t.Errorf("write: got = %v, want = ErrUnsupportedFormat", err)
	}
}

var sample = &Subtitle{Cues: []Cue{
	{Index: 1, Start: ms(1180), End: ms(6850), Text: "{\\pos(400,570)}Like an angel\nwith pity on nobody"},
	{Index: 2, Start: time.Hour + ms(5), End: time.Hour + ms(2500), Text: "<i>A & B</i>"},
}}

func TestWrite(t *testing.T) {
	tests := []struct {
		format, want string
	}{
		{"srt", "1\n00:00:01,180 --> 00:00:06,850\nLike an angel\nwith pity on nobody\n\n2\n01:00:00,005 --> 01:00:02,500\n<i>A & B</i>\n\n"},
		{"vtt", "WEBVTT\n\n00:00:01.180 --> 00:00:06.850\nLike an angel\nwith pity on nobody\n\n01:00:00.005 --> 01:00:02.500\n<i>A & B</i>\n\n"},
	}
	for _, tt := range tests {
		got, err := Write(sample, tt.format)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != tt.want {
			t.Errorf("%s: got = %q, want = %q", tt.format, got, tt.want)
		}
	}
	data, err := Write(sample, "json")
	if err != nil {
		t.Fatal(err)
	}
	var track jsonTrack
	if err := json.Unmarshal(data, &track); err != nil {
		t.Fatal(err)
	}
	want := jsonTrack{TrackEvents: []jsonEvent{
		{ID: "1", Text: sample.Cues[0].Text, StartPositionTicks: 11_800_000, EndPositionTicks: 68_500_000},
		{ID: "2", Text: sample.Cues[1].Text, StartPositionTicks: 36_000_050_000, EndPositionTicks: 36_025_000_000},
	}}
	if !reflect.DeepEqual(track, want) {
		t.Errorf("json: got = %+v, want = %+v", track, want)
	}
	ttml, _ := Write(sample, "ttml")
	if !strings.Contains(string(ttml), `<p begin="00:00:01.180" end="00:00:06.850">Like an angel<br />with pity on nobody</p>`) ||
		!strings.Contains(string(ttml), `>A &amp; B</p>`) {
		t.Errorf("ttml: got = %s", ttml)
	}
}

// TestRoundTrip writes each format that can be read back and parses it.
func TestRoundTrip(t *testing.T) {
	for _, format := range []string{"srt", "vtt", "ssa", "ass"} {
		data, err := Write(sample, format)
		if err != nil {
			t.Fatal(err)
		}
		s, err := Parse(data, format)
		if err != nil {
			t.Fatalf("%s: %v\n%s", format, err, data)
		}
		precision := time.Millisecond
		if format == "ssa" || format == "ass" {
			precision = 10 * time.Millisecond // centiseconds
		}
		if len(s.Cues) != 2 || s.Cues[1].Start != sample.Cues[1].Start.Truncate(precision) || s.Cues[1].End != sample.Cues[1].End {
			t.Errorf("%s: got = %+v", format, s.Cues)
		}
		wantText := "Like an angel\nwith pity on nobody"
		if format == "ssa" || format == "ass" {
			wantText = sample.Cues[0].Text // overrides survive
		}
		if s.Cues[0].Text != wantText {
			t.Errorf("%s text: got = %q, want = %q", format, s.Cues[0].Text, wantText)
		}
	}
}

func TestFilter(t *testing.T) {
	cues := func() *Subtitle {
		return &Subtitle{Cues: []Cue{
			{Index: 1, Start: ms(0), End: ms(1000)},
			{Index: 2, Start: ms(1500), End: ms(3000)}, // spans the start
			{Index: 3, Start: ms(4000), End: ms(5000)},
			{Index: 4, Start: ms(9000), End: ms(9500)},
		}}
	}
	s := cues()
	s.Filter(ms(2000), ms(8000), false)
	if len(s.Cues) != 2 || s.Cues[0].Index != 2 || s.Cues[0].Start != 0 || s.Cues[0].End != ms(1000) || s.Cues[1].Start != ms(2000) {
		t.Errorf("shifted: got = %+v", s.Cues)
	}
	s = cues()
	s.Filter(ms(2000), 0, true)
	if len(s.Cues) != 3 || s.Cues[0].Start != ms(1500) {
		t.Errorf("preserved: got = %+v", s.Cues)
	}
}

func TestToUTF8Chinese(t *testing.T) {
	for name, tt := range map[string]struct {
		enc  encoding.Encoding
		text string
	}{
		"gbk":  {simplifiedchinese.GBK, "这是一个中文字幕文件，用于测试编码检测。我们希望它能正确识别。"},
		"big5": {traditionalchinese.Big5, "這是一個中文字幕檔案，用於測試編碼偵測。我們希望它能正確識別。"},
	} {
		var b strings.Builder
		for i := 1; i <= 6; i++ {
			b.WriteString("1\n00:00:01,000 --> 00:00:02,000\n" + tt.text + "\n\n")
		}
		data, err := tt.enc.NewEncoder().Bytes([]byte(b.String()))
		if err != nil {
			t.Fatal(err)
		}
		out, charset, err := ToUTF8(data)
		if err != nil || !strings.Contains(string(out), tt.text) {
			t.Errorf("%s: got = %s %v %q", name, charset, err, out[:min(len(out), 80)])
		}
	}
}

func TestDetectCharsetBOM(t *testing.T) {
	for want, data := range map[string][]byte{
		"utf-8":    {0xEF, 0xBB, 0xBF, 'a'},
		"utf-16le": {0xFF, 0xFE, 'a', 0},
		"utf-16be": {0xFE, 0xFF, 0, 'a'},
		"utf-32le": {0xFF, 0xFE, 0, 0, 'a', 0, 0, 0},
	} {
		if got := DetectCharset(data); got != want {
			t.Errorf("DetectCharset(% x): got = %q, want = %q", data, got, want)
		}
	}
}

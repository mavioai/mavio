package parse

import (
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/ported"
)

var now = time.Date(2026, time.October, 11, 9, 30, 15, 0, time.UTC)

func TestParseUtil(t *testing.T) {
	ported.Run(t, "parser/parse_util.json", ported.Cases{Run: map[string]func(*testing.T, ported.Args){
		"should_parse_size": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "stringSize")
			want, _ := a.Int(t, "size")
			if got, err := Bytes(s); err != nil || got != want {
				t.Errorf("Bytes(%q) = %d, %v, want = %d", s, got, err, want)
			}
		},
		"should_normalize_multiple_spaces": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "original")
			want, _ := a.Str(t, "newString")
			if got := NormalizeSpaces(s); got != want {
				t.Errorf("NormalizeSpaces(%q) got = %q, want = %q", s, got, want)
			}
		},
		"should_parse_int_from_string": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "original")
			want, _ := a.Int(t, "parsedInt")
			if got, err := Int(s); err != nil || got != want {
				t.Errorf("Int(%q) = %d, %v, want = %d", s, got, err, want)
			}
		},
		"should_parse_double_from_string": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "original")
			want := a.Float(t, "parsedInt")
			if got, err := Float(s); err != nil || math.Abs(got-want) > 1e-9 {
				t.Errorf("Float(%q) = %v, %v, want = %v", s, got, err, want)
			}
		},
		"should_parse_long_from_string": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "original")
			want, wantOK := a.Int(t, "parsedInt")
			if got, ok := LeadingInt(s); got != want || ok != wantOK {
				t.Errorf("LeadingInt(%q) got = %d, %t, want = %d, %t", s, got, ok, want, wantOK)
			}
		},
		"should_parse_full_imdb_id_from_string": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "input")
			want, _ := a.Str(t, "expected")
			if got := FullIMDbID(s); got != want {
				t.Errorf("FullIMDbID(%q) got = %q, want = %q", s, got, want)
			}
		},
	}})
}

func TestDateTimeUtil(t *testing.T) {
	ported.Run(t, "parser/date_time_util.json", ported.Cases{Run: map[string]func(*testing.T, ported.Args){
		// Go formats dates the same in every culture.
		"should_format_date_invariant": func(t *testing.T, a ported.Args) {
			s := now.Format(time.DateTime)
			got, err := Unknown(now, s, "")
			if err != nil || !got.Equal(now) {
				t.Errorf("Unknown(%q) = %v, %v, want = %v", s, got, err, now)
			}
		},
		"parse_datetime_golang": func(t *testing.T, a ported.Args) {
			s, _ := a.Str(t, "dateInput")
			layout, _ := a.Str(t, "format")
			standard, _ := a.Str(t, "standardFormat")
			want, _ := a.Str(t, "expectedDate")
			d, err := Layout(now, s, layout)
			if err != nil {
				t.Fatal(err)
			}
			if got := d.UTC().Format(DotNetLayout(standard)); got != want {
				t.Errorf("Layout(%q, %q) got = %q, want = %q", s, layout, got, want)
			}
		},
	}})
}

var (
	testCaseString = regexp.MustCompile(`@"((?:[^"]|"")*)"`)
	testCaseDate   = regexp.MustCompile(`new DateTime\(([^)]*)\)`)
)

func TestDateTimeRoutines(t *testing.T) {
	ported.Run(t, "parser/date_time_routines.json", ported.Cases{Run: map[string]func(*testing.T, ported.Args){
		"should_parse_date": func(t *testing.T, a ported.Args) {
			// new TestCaseData(@"…").Returns(new DateTime(y, M, d, h, m, s))
			e := a.Expr(t, "date")
			s := strings.ReplaceAll(testCaseString.FindStringSubmatch(e)[1], `""`, `"`)
			var parts []int
			for p := range strings.SplitSeq(testCaseDate.FindStringSubmatch(e)[1], ",") {
				switch p = strings.TrimSpace(p); p {
				case "DateTime.Now.Year":
					parts = append(parts, now.Year())
				case "DateTime.Now.Month":
					parts = append(parts, int(now.Month()))
				case "DateTime.Now.Day":
					parts = append(parts, now.Day())
				default:
					n, err := strconv.Atoi(p)
					if err != nil {
						t.Fatalf("unsupported expression %s", e)
					}
					parts = append(parts, n)
				}
			}
			want := time.Date(parts[0], time.Month(parts[1]), parts[2], parts[3], parts[4], parts[5], 0, time.UTC)
			if got, ok := FuzzyTime(now, s, MonthFirst); !ok || !got.Equal(want) {
				t.Errorf("FuzzyTime(%q) = %v, %t, want = %v", s, got, ok, want)
			}
		},
	}})
}

func TestUnknown(t *testing.T) {
	tests := []struct {
		in   string
		want time.Time
	}{
		{"Sat, 10 Oct 2026 12:00:00 +0000", time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)},
		{"1791504000", time.Unix(1791504000, 0).UTC()},
		{"just now", now},
		{"2 hours 1 day ago", now.Add(-26 * time.Hour)},
		{"1.5 weeks ago", now.Add(-252 * time.Hour)},
		{"Today, 14:22", time.Date(2026, 10, 11, 14, 22, 0, 0, time.UTC)},
		{"Yesterday at 2:30 PM", time.Date(2026, 10, 10, 14, 30, 0, 0, time.UTC)},
		{"tomorrow", time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)},
		// The 11th is a Sunday.
		{"Friday at 09:00", time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)},
		{"03-14 10:00", time.Date(2026, 3, 14, 10, 0, 0, 0, time.UTC)},
		{"1 Jan 10:30", time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)},
		{"2024-02-29 23:59:59", time.Date(2024, 2, 29, 23, 59, 59, 0, time.UTC)},
	}
	for _, tt := range tests {
		got, err := Unknown(now, tt.in, "")
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("Unknown(%q) = %v, %v, want = %v", tt.in, got, err, tt.want)
		}
	}
	if _, err := Unknown(now, "5 fortnights ago", ""); err == nil {
		t.Errorf("Unknown(5 fortnights ago) got = nil error, want = an error")
	}
}

func TestLayout(t *testing.T) {
	tests := []struct {
		in, layout string
		want       time.Time
	}{
		{"14:05", "15:04", time.Date(2026, 10, 11, 14, 5, 0, 0, time.UTC)},
		{"Mar 3 14:05", "Jan 2 15:04", time.Date(2026, 3, 3, 14, 5, 0, 0, time.UTC)},
		{"03.03.2021 14:05", "dd.MM.yyyy HH:mm", time.Date(2021, 3, 3, 14, 5, 0, 0, time.UTC)},
		{"Wednesday, 03 March 2021", "dddd, dd MMMM yyyy", time.Date(2021, 3, 3, 0, 0, 0, 0, time.UTC)},
		{"2021-03-03 14:05:06.123", "yyyy-MM-dd HH:mm:ss.fff", time.Date(2021, 3, 3, 14, 5, 6, 123e6, time.UTC)},
	}
	for _, tt := range tests {
		got, err := Layout(now, tt.in, tt.layout)
		if err != nil || !got.Equal(tt.want) {
			t.Errorf("Layout(%q, %q) = %v, %v, want = %v", tt.in, tt.layout, got, err, tt.want)
		}
	}
}

func TestQueryArg(t *testing.T) {
	if got := QueryArg("https://x/download.php?id=12&name=a+b#top", "name"); got != "a b" {
		t.Errorf("QueryArg got = %q, want = %q", got, "a b")
	}
}

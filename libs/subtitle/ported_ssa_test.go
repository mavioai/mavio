package subtitle

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Test cases ported from Lingarr by tools/testport live in
// testdata/cases/ssa; see docs/testing.md §2.

// minimalASS is the header of SsaParserTests' scripts.
const minimalASS = "[Script Info]\nTitle: Test\nScriptType: v4.00+\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\n" +
	"Style: Default,Arial,20\n[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"

const wrapStyle = "libs/subtitle reads \\n as a line break whatever the script's WrapStyle, as pysubs2 does for Bazarr's mods, " +
	"which it ports; the Script Info is kept as read"

func parseASS(t *testing.T, script string) *Subtitle {
	t.Helper()
	s, err := Parse([]byte(script), ASS)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// spoken returns the spoken text of each cue.
func spoken(s *Subtitle) []string {
	out := make([]string, len(s.Cues))
	for i, c := range s.Cues {
		out[i] = SpokenText(c.Text)
	}
	return out
}

func TestPortedLingarrSsaParser(t *testing.T) {
	portedCases(t, "ssa/ssa_parser.json", ported{
		skip: map[string]string{
			"ParseStream_NonNoneWrapStyles_SplitOnBackslashN_Only":               wrapStyle,
			"ParseStream_ParsesWrapStyleFromScriptInfo":                          wrapStyle,
			"ParseStream_NoneWrapStyle_SplitsOnBothBackslashNAndBackslashLowerN": wrapStyle,
			"ParseStream_NonSeekableStream_Throws":                               "Parse takes the subtitle's bytes, not a stream",
		},
		facts: map[string]string{
			"ParseStream_ParsesDialogueTimingTextAndDialogueFields":          "TestLingarrSsaParser",
			"ParseStream_MultipleDialogueLines_ReturnsAll":                   "TestLingarrSsaParser",
			"ParseStream_DialogueWithAssTags_StripsTagsFromPlaintext":        "TestLingarrSsaParser",
			"ParseStream_TextContainingCommas_NotSplit":                      "TestLingarrSsaParser",
			"ParseStream_CapturesV4PlusStylesSectionAndHeader":               "TestLingarrSsaScriptKept",
			"ParseStream_CapturesV4StylesSectionAndHeader":                   "TestLingarrSsaScriptKept",
			"ParseStream_CapturesEventsSectionHeader":                        "TestLingarrSsaScriptKept",
			"ParseStream_BareVectorDrawingLine_KeepsEntryWithEmptyPlaintext": "TestLingarrSsaDrawings",
			"ParseStream_DrawingModeVectorLine_KeepsEntryWithEmptyPlaintext": "TestLingarrSsaDrawings",
			"ParseStream_MixedVectorAndText_KeepsText":                       "TestLingarrSsaDrawings",
			"ParseStream_AllLinesPureVector_KeepsEntryWithEmptyPlaintext":    "TestLingarrSsaDrawings",
			"ParseStream_LayerOmittedDialogue_Parses":                        "TestLingarrSsaLayerOmitted",
			"ParseStream_NoDialogueLines_Throws":                             "TestLingarrSsaParser",
		},
	})
}

func TestLingarrSsaParser(t *testing.T) {
	s := parseASS(t, minimalASS+"Dialogue: 0,0:00:01.50,0:00:03.25,Default,Bob,0,0,0,,Hello world\n")
	if len(s.Cues) != 1 || s.Cues[0].Start != 1500*time.Millisecond || s.Cues[0].End != 3250*time.Millisecond || s.Cues[0].Text != "Hello world" {
		t.Errorf("cues = %+v, want Hello world from 1.5s to 3.25s", s.Cues)
	}
	// The dialogue's other fields are kept when its text changes.
	s.Cues[0].Text = "Bonjour"
	if out, _ := Write(s, ASS); !strings.Contains(string(out), "Dialogue: 0,0:00:01.50,0:00:03.25,Default,Bob,0,0,0,,Bonjour\n") {
		t.Errorf("written = %q, want the style and name kept", out)
	}

	s = parseASS(t, minimalASS+"Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,First\n"+
		"Dialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,Second\nDialogue: 0,0:00:09.00,0:00:12.00,Default,,0,0,0,,Third\n")
	if got := strings.Join(spoken(s), "|"); got != "First|Second|Third" {
		t.Errorf("cues = %q, want = First|Second|Third", got)
	}
	s = parseASS(t, minimalASS+`Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,{\b}Bold{\b0} text`+"\n")
	if got := spoken(s)[0]; got != "Bold text" {
		t.Errorf("spoken text = %q, want = Bold text", got)
	}
	s = parseASS(t, minimalASS+"Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hello, world, and more\n")
	if got := s.Cues[0].Text; got != "Hello, world, and more" {
		t.Errorf("text = %q, want the commas kept", got)
	}
	if _, err := Parse([]byte(minimalASS), ASS); !errors.Is(err, ErrNoCues) {
		t.Errorf("Parse without dialogue: err = %v, want ErrNoCues", err)
	}
}

func TestLingarrSsaScriptKept(t *testing.T) {
	for _, c := range []struct {
		name, format, script string
		want                 []string
	}{
		{
			"V4+", ASS, minimalASS + "Dialogue: 0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hi\n",
			[]string{"[V4+ Styles]", "Format: Name, Fontname, Fontsize", "Style: Default,Arial,20", "[Events]", "Format: Layer, Start"},
		},
		{
			"V4", SSA, "[Script Info]\nTitle: Legacy\n[V4 Styles]\nFormat: Name, Fontname\nStyle: Default,Arial\n[Events]\n" +
				"Format: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n" +
				"Dialogue: Marked=0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Hi\n",
			[]string{"[V4 Styles]", "Style: Default,Arial", "Dialogue: Marked=0,0:00:01.00,0:00:02.00,Default,,0,0,0,,Salut"},
		},
	} {
		s, err := Parse([]byte(c.script), c.format)
		if err != nil {
			t.Fatal(err)
		}
		s.Cues[0].Text = "Salut"
		out, err := Write(s, c.format)
		if err != nil {
			t.Fatal(err)
		}
		for _, w := range c.want {
			if !strings.Contains(string(out), w) {
				t.Errorf("%s: written = %q, want it to keep %q", c.name, out, w)
			}
		}
	}
}

func TestLingarrSsaDrawings(t *testing.T) {
	for _, c := range []struct {
		name, dialogue string
		want           string
	}{
		{"bare vector", "Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,m 0 0 l 0 -403 l 716 -403 l 716 0", ""},
		{"drawing mode", `Dialogue: 6,0:00:00.00,0:00:02.00,Signs,,0,0,0,,{\p1}m 0 0 l 716 0 l 716 403 l 0 403{\p0}`, ""},
		{"mixed", `Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,{\p1}m 0 0{\p0} Meeting at 9 AM`, "Meeting at 9 AM"},
		{"all lines vector", `Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,m 0 0 l 10 10\Nm 20 20 l 30 30`, "\n"},
	} {
		s := parseASS(t, minimalASS+c.dialogue+"\nDialogue: 0,0:00:05.00,0:00:08.00,Default,,0,0,0,,Real dialogue text\n")
		if got := spoken(s); len(got) != 2 || got[0] != c.want || got[1] != "Real dialogue text" {
			t.Errorf("%s: spoken text = %q, want = [%q Real dialogue text]", c.name, got, c.want)
		}
	}
}

func TestLingarrSsaLayerOmitted(t *testing.T) {
	s := parseASS(t, "[Script Info]\nTitle: Test\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize\nStyle: Default,Arial,20\n[Events]\n"+
		"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"+
		"Dialogue: 0:00:01.00,0:00:04.00,Default,,0,0,0,,Hello from legacy Aegisub, again\n")
	if len(s.Cues) != 1 || s.Cues[0].Text != "Hello from legacy Aegisub, again" || s.Cues[0].Start != time.Second {
		t.Fatalf("cues = %+v, want the dialogue without its layer", s.Cues)
	}
	s.Cues[0].Text = "Bonjour"
	if out, _ := Write(s, ASS); !strings.Contains(string(out), "Dialogue: 0, 0:00:01.00,0:00:04.00,Default,,0,0,0,,Bonjour\n") {
		t.Errorf("written = %q, want the dialogue in layer 0", out)
	}
}

const strippedSRT = "with formatting stripped, Mavio writes translations as SubRip, so no SSA style is written"

func TestPortedLingarrSsaWriter(t *testing.T) {
	portedCases(t, "ssa/ssa_writer.json", ported{
		skip: map[string]string{
			"WriteStreamAsync_StripFormatting_EmitsDefaultStyleEvenWithEmptyStyles": strippedSRT,
			"WriteStreamAsync_StripFormatting_PreservesSourceDefaultStyle":          strippedSRT,
			"WriteStreamAsync_StripFormatting_ScalesFallbackFontsizeToPlayResY":     strippedSRT,
		},
		facts: map[string]string{
			"WriteStreamAsync_PreservesOriginalStyles_WhenNotStripping":      "TestLingarrSsaWriter",
			"WriteStreamAsync_EmptyTranslatedLines_FallsBackToOriginalLines": "TestLingarrSsaWriter",
		},
	})
}

func TestLingarrSsaWriter(t *testing.T) {
	s := parseASS(t, "[Script Info]\n[V4+ Styles]\nFormat: Name, Fontname, Fontsize, PrimaryColour\nStyle: MyCustom,Arial,20,&H00FFFFFF\n[Events]\n"+
		"Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n"+
		`Dialogue: 0,0:00:00.00,0:00:01.00,MyCustom,,0,0,0,,{\p1}m 0 0 l 10 10{\p0}`+"\n"+
		"Dialogue: 0,0:00:00.00,0:00:02.00,MyCustom,,0,0,0,,Hi\n")
	// The server translates only cues with spoken text, so the drawing
	// keeps its path.
	for i, c := range s.Cues {
		if SpokenText(c.Text) != "" {
			s.Cues[i].Text = "مرحبا"
		}
	}
	out, err := Write(s, ASS)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range []string{"Style: MyCustom,Arial,20,&H00FFFFFF", `{\p1}m 0 0 l 10 10{\p0}`, ",,مرحبا\n"} {
		if !strings.Contains(string(out), w) {
			t.Errorf("written = %q, want it to hold %q", out, w)
		}
	}
}

func TestPortedLingarrSubtitleFormatter(t *testing.T) {
	facts := map[string]string{}
	for _, c := range removeMarkupCases {
		facts[c.fact] = "TestRemoveMarkup"
	}
	portedCases(t, "ssa/subtitle_formatter_service.json", ported{
		skip: map[string]string{
			"RemoveMarkup_WithTab_ConvertsToSpace": `"\t" outside override blocks is no SSA escape; libs/subtitle keeps it as pysubs2 does`,
			"RemoveMarkup_Null_ReturnsEmpty":       "Go strings are never null",
		},
		facts: facts,
	})
}

// removeMarkupCases are RemoveMarkup's cases; libs/subtitle keeps the
// line breaks, which the server splits cues at, so they are compared with
// whitespace collapsed as RemoveMarkup does.
var removeMarkupCases = []struct{ fact, in, want string }{
	{"RemoveMarkup_PlainText_ReturnsUnchanged", "Hello world", "Hello world"},
	{"RemoveMarkup_WithAssTags_StripsTags", `Hello {\b}world{\b0}`, "Hello world"},
	{"RemoveMarkup_WithHtmlTags_StripsTags", "Hello <b>world</b>", "Hello world"},
	{"RemoveMarkup_WithLineBreaks_ConvertsToSpaces", `Hello\Nworld\nsecond line`, "Hello world second line"},
	{"RemoveMarkup_WithHardSpace_ConvertsToSpace", `Hello\hworld`, "Hello world"},
	{"RemoveMarkup_WithMultipleWhitespace_CollapsesToSingleSpace", `Hello   world\n\n  test`, "Hello world test"},
	{"RemoveMarkup_EmptyString_ReturnsEmpty", "", ""},
	{"RemoveMarkup_WhitespaceOnly_ReturnsEmpty", "   ", ""},
	{"RemoveMarkup_WithDrawingModeTags_StripsTagAndVectorCommands", `{\p1}m 0 0 l 100 100{\p0}`, ""},
	{"RemoveMarkup_DrawingModeTagInMiddle_StripsAndReturnsText", `Hello {\p1}m 0 0{\p0} world`, "Hello world"},
	{"RemoveMarkup_NestedTags_StripsCorrectly", `{\blur5}{\p1}m 0 0{\p0}{\blur0}Text`, "Text"},
	{"RemoveMarkup_BareVectorPath_mCommand_ReturnsEmpty", "m 0 0", ""},
	{"RemoveMarkup_BareVectorPath_mWithCoordinates_ReturnsEmpty", "m 0 0 l 100 200 l -50 75", ""},
	{"RemoveMarkup_BareVectorPath_MixedCaseCommand_ReturnsEmpty", "M 0 0 L 100 200", ""},
	{"RemoveMarkup_BareVectorPath_MixedCaseCommand_ReturnsEmpty", "c 10 20 30 40 50 60", ""},
	{"RemoveMarkup_BareVectorPath_MixedCaseCommand_ReturnsEmpty", "B 1 2 3 4 5 6", ""},
	{"RemoveMarkup_BareVectorPath_WithDecimalCoords_ReturnsEmpty", "m 0.5 -10.25 l 100.0 200.5", ""},
	{"RemoveMarkup_BareVectorPath_WithScientificNotation_ReturnsEmpty", "m 1e2 2E-3 l 3.5e1 -4.2E+2", ""},
	{"RemoveMarkup_BareVectorPath_ChainedCommands_ReturnsEmpty", "m 9 23 b 8 22 b 4 18 b 6 25 b 8 12", ""},
	{"RemoveMarkup_TextStartingWithVectorCommand_NotEmpty", "m 9:00 morning meeting", "m 9:00 morning meeting"},
	{"RemoveMarkup_CleanText_ReturnsText", "This is clean dialogue", "This is clean dialogue"},
	{"RemoveMarkup_VectorCommandsWithTextAfter_ReturnsText", `{\p1}m 0 0 l 100 100{\p0} Meeting`, "Meeting"},
	{"RemoveMarkup_DrawingModeWithMultiplePaths_StripsEntireBlock", `{\p1}m 0 0 0 100{\p0} Title {\p1}m 200 200 200 300{\p0}`, "Title"},
	{"RemoveMarkup_CaseInsensitiveDrawingTags_StripsCorrectly", `{\P1}m 0 0{\P0} text {\p2}m 10 10{\p2} more`, "text more"},
	{"RemoveMarkup_UnclosedDrawingTag_StripsVectorPrefix", `{\p1}m 0 0 text without close`, ""},
}

func TestRemoveMarkup(t *testing.T) {
	for _, c := range removeMarkupCases {
		// Cues hold SSA's line breaks as "\n".
		in := strings.NewReplacer(`\N`, "\n", `\n`, "\n").Replace(c.in)
		if got := strings.Join(strings.Fields(SpokenText(in)), " "); got != c.want {
			t.Errorf("%s: SpokenText(%q) = %q, want = %q", c.fact, c.in, got, c.want)
		}
	}
}

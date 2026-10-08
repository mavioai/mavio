package subtitle

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

// defaultEventFormat is the ASS event format assumed when a file has no
// Format line.
var defaultEventFormat = []string{"layer", "start", "end", "style", "name", "marginl", "marginr", "marginv", "effect", "text"}

// parseSSA reads the Dialogue events of SubStation Alpha (SSA) and Advanced
// SubStation Alpha (ASS) scripts, in file order. "\N" and "\n" become line
// breaks; override blocks are kept.
func parseSSA(lines []string) []Cue {
	var cues []Cue
	inEvents := false
	format := defaultEventFormat
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inEvents = strings.EqualFold(line, "[Events]")
			continue
		}
		if !inEvents {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(key)) {
		case "format":
			format = nil
			for _, f := range strings.Split(value, ",") {
				format = append(format, strings.ToLower(strings.TrimSpace(f)))
			}
		case "dialogue":
			if c, ok := parseDialogue(value, format); ok {
				c.Index = len(cues) + 1
				cues = append(cues, c)
			}
		}
	}
	return cues
}

func parseDialogue(value string, format []string) (Cue, bool) {
	fields := strings.SplitN(value, ",", len(format))
	if len(fields) < len(format) {
		return Cue{}, false
	}
	var c Cue
	var okStart, okEnd bool
	for i, name := range format {
		v := fields[i]
		switch name {
		case "start":
			c.Start, okStart = parseSSATime(v)
		case "end":
			c.End, okEnd = parseSSATime(v)
		case "text":
			c.Text = strings.NewReplacer(`\N`, "\n", `\n`, "\n").Replace(v)
		}
	}
	return c, okStart && okEnd
}

var ssaTime = regexp.MustCompile(`^\s*(\d+):(\d{1,2}):(\d{1,2})\.(\d{1,3})\s*$`)

// parseSSATime parses H:MM:SS.cc.
func parseSSATime(s string) (time.Duration, bool) {
	m := ssaTime.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}
	return clock(m[1], m[2], m[3], m[4]), true
}

// ssaTimestamp formats d as H:MM:SS.cc.
func ssaTimestamp(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	cs := d.Milliseconds() / 10
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360_000, cs/6000%60, cs/100%60, cs%100)
}

func writeSSA(s *Subtitle, advanced bool) []byte {
	var b strings.Builder
	b.WriteString("[Script Info]\n")
	if advanced {
		b.WriteString("ScriptType: v4.00+\nPlayResX: 384\nPlayResY: 288\n\n[V4+ Styles]\n")
		b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
		b.WriteString("Style: Default,Arial,20,&H00FFFFFF,&H0300FFFF,&H00000000,&H02000000,0,0,0,0,100,100,0,0,1,2,1,2,10,10,10,1\n\n")
		b.WriteString("[Events]\nFormat: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	} else {
		b.WriteString("ScriptType: v4.00\nPlayResX: 384\nPlayResY: 288\n\n[V4 Styles]\n")
		b.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, TertiaryColour, BackColour, Bold, Italic, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, AlphaLevel, Encoding\n")
		b.WriteString("Style: Default,Arial,20,16777215,65535,65535,-2147483640,0,0,1,2,1,2,10,10,10,0,1\n\n")
		b.WriteString("[Events]\nFormat: Marked, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")
	}
	first := "0"
	if !advanced {
		first = "Marked=0"
	}
	for _, c := range s.Cues {
		text := strings.ReplaceAll(c.Text, "\n", `\N`)
		fmt.Fprintf(&b, "Dialogue: %s,%s,%s,Default,,0,0,0,,%s\n", first, ssaTimestamp(c.Start), ssaTimestamp(c.End), text)
	}
	return []byte(b.String())
}

var ssaOverride = regexp.MustCompile(`\{\\[^}]*\}`)

// plainText removes SSA override blocks such as {\pos(400,570)} and turns
// hard spaces into spaces, for formats that cannot show them.
func plainText(text string) string {
	return strings.ReplaceAll(ssaOverride.ReplaceAllString(text, ""), `\h`, " ")
}

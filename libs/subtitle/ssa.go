package subtitle

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

// defaultEventFormat is the ASS event format assumed when a file has no
// Format line.
var defaultEventFormat = []string{"layer", "start", "end", "style", "name", "marginl", "marginr", "marginv", "effect", "text"}

// ssaScript is what an SSA or ASS file holds besides the timing and text
// of its dialogue: its script info, styles, comments, other sections and
// each event's other fields, kept as read, so that writing the file in
// its own format changes only what was changed of its cues, as pysubs2
// keeps them for Subzero.
type ssaScript struct {
	// advanced is set for ASS, with V4+ styles.
	advanced bool
	// head is the lines before the events, through their Format line.
	head []string
	// format is the events' fields, lower-case.
	format []string
	events []ssaEvent
	// tail is the lines of the sections after the events.
	tail []string
}

// ssaEvent is a line of the events section.
type ssaEvent struct {
	// line is the line as read, written back as it was unless the event
	// is a dialogue.
	line string
	// fields are a dialogue's fields as read; nil for comments and other
	// lines.
	fields []string
	// cue is the dialogue as parsed, to tell what changed.
	cue Cue
}

// parseSSA reads the Dialogue events of SubStation Alpha (SSA) and Advanced
// SubStation Alpha (ASS) scripts, in file order, and the rest of the
// script. "\N" and "\n" become line breaks; override blocks are kept. The
// script is ASS when it has V4+ styles, SSA when it has V4 styles, and
// otherwise as advanced says.
func parseSSA(lines []string, advanced bool) ([]Cue, *ssaScript) {
	var cues []Cue
	script := &ssaScript{advanced: advanced, format: defaultEventFormat}
	section := ""
	formatRead := false
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		// The line break ending the file.
		lines = lines[:len(lines)-1]
	}
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.ToLower(line)
			switch section {
			case "[v4+ styles]":
				script.advanced = true
			case "[v4 styles]":
				script.advanced = false
			}
		}
		switch {
		case section != "[events]" && len(script.events) == 0 && !formatRead:
			script.head = append(script.head, raw)
			continue
		case section != "[events]":
			script.tail = append(script.tail, raw)
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		kind := strings.ToLower(strings.TrimSpace(key))
		if !formatRead && (!ok || kind != "dialogue" && kind != "comment") {
			if ok && kind == "format" {
				script.format = nil
				for _, f := range strings.Split(value, ",") {
					script.format = append(script.format, strings.ToLower(strings.TrimSpace(f)))
				}
				formatRead = true
			}
			script.head = append(script.head, raw)
			continue
		}
		formatRead = true
		e := ssaEvent{line: raw}
		if ok && kind == "dialogue" {
			_, value, _ := strings.Cut(raw, ":")
			if c, fields, ok := parseDialogue(value, script.format); ok {
				c.Index = len(cues) + 1
				c.event = len(script.events) + 1
				e.fields, e.cue = fields, c
				cues = append(cues, c)
			}
		}
		script.events = append(script.events, e)
	}
	return cues, script
}

func parseDialogue(value string, format []string) (Cue, []string, bool) {
	c, fields, ok := dialogueFields(value, format)
	if !ok && len(format) > 1 && format[0] == "layer" {
		// Older Aegisub and FFmpeg write dialogue without its layer.
		if c, fields, ok := dialogueFields(value, format[1:]); ok {
			return c, append([]string{" 0"}, fields...), true
		}
	}
	return c, fields, ok
}

// dialogueFields parses a dialogue's fields in the format.
func dialogueFields(value string, format []string) (Cue, []string, bool) {
	fields := strings.SplitN(value, ",", len(format))
	if len(fields) < len(format) {
		return Cue{}, nil, false
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
			c.Text = ssaText(v)
		}
	}
	return c, fields, okStart && okEnd
}

// ssaText returns the text of an event with line breaks as "\n".
func ssaText(v string) string {
	return strings.NewReplacer(`\N`, "\n", `\n`, "\n").Replace(strings.TrimRightFunc(v, unicode.IsSpace))
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
	if s.script != nil && s.script.advanced == advanced {
		return s.script.write(s.Cues)
	}
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

// write writes the script with its dialogue replaced by cues: each event
// as it was read, a dialogue with the timing and text of the cues read
// from it, where they changed, and without it when they were dropped.
// Cues read from no event of the script follow the events, in the
// Default style.
func (sc *ssaScript) write(cues []Cue) []byte {
	byEvent := map[int][]Cue{}
	var added []Cue
	for _, c := range cues {
		if c.event > 0 && c.event <= len(sc.events) && sc.events[c.event-1].fields != nil {
			byEvent[c.event] = append(byEvent[c.event], c)
		} else {
			added = append(added, c)
		}
	}
	var b strings.Builder
	for _, l := range sc.head {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	// Added cues follow the last event that is not a blank line.
	last := len(sc.events) - 1
	for last >= 0 && strings.TrimSpace(sc.events[last].line) == "" {
		last--
	}
	for i, e := range sc.events {
		if i == last+1 {
			sc.writeAdded(&b, added)
			added = nil
		}
		if e.fields == nil {
			b.WriteString(e.line)
			b.WriteByte('\n')
			continue
		}
		for _, c := range byEvent[i+1] {
			if c.Start == e.cue.Start && c.End == e.cue.End && c.Text == e.cue.Text {
				b.WriteString(e.line)
				b.WriteByte('\n')
				continue
			}
			sc.writeDialogue(&b, e.fields, c, e.cue)
		}
	}
	sc.writeAdded(&b, added)
	for _, l := range sc.tail {
		b.WriteString(l)
		b.WriteByte('\n')
	}
	return []byte(b.String())
}

// writeAdded writes cues read from no event as dialogue in the Default
// style.
func (sc *ssaScript) writeAdded(b *strings.Builder, added []Cue) {
	if len(added) > 0 {
		blank := make([]string, len(sc.format))
		for i, name := range sc.format {
			switch name {
			case "layer":
				blank[i] = " 0"
			case "marked":
				blank[i] = " Marked=0"
			case "style":
				blank[i] = "Default"
			case "marginl", "marginr", "marginv":
				blank[i] = "0"
			}
		}
		for _, c := range added {
			sc.writeDialogue(b, blank, c, Cue{Start: -1, End: -1})
		}
	}
}

// writeDialogue writes a dialogue line of fields with the cue's timing and
// text in place of those of was.
func (sc *ssaScript) writeDialogue(b *strings.Builder, fields []string, c, was Cue) {
	out := slices.Clone(fields)
	for i, name := range sc.format {
		switch {
		case name == "start" && c.Start != was.Start:
			out[i] = ssaTimestamp(c.Start)
		case name == "end" && c.End != was.End:
			out[i] = ssaTimestamp(c.End)
		case name == "text" && c.Text != was.Text:
			out[i] = strings.ReplaceAll(c.Text, "\n", `\N`)
		}
	}
	b.WriteString("Dialogue:")
	b.WriteString(strings.Join(out, ","))
	b.WriteByte('\n')
}

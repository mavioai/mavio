package subtitle

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"
)

func writeTTML(s *Subtitle) []byte {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<tt xmlns="http://www.w3.org/ns/ttml" xml:lang="">` + "\n<body>\n<div>\n")
	for _, c := range s.Cues {
		var lines []string
		for line := range strings.SplitSeq(stripTags(plainText(c.Text)), "\n") {
			lines = append(lines, html.EscapeString(line))
		}
		fmt.Fprintf(&b, `<p begin="%s" end="%s">%s</p>`+"\n", timestamp(c.Start, "."), timestamp(c.End, "."), strings.Join(lines, "<br />"))
	}
	b.WriteString("</div>\n</body>\n</tt>\n")
	return []byte(b.String())
}

// stripTags removes HTML-like tags such as <i> and <font color="…">.
func stripTags(text string) string {
	var b strings.Builder
	depth := 0
	for _, r := range text {
		switch {
		case r == '<':
			depth++
		case r == '>' && depth > 0:
			depth--
		case depth == 0:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// jsonTrack is Jellyfin's JSON subtitle format; positions are in ticks of
// 100 ns.
type jsonTrack struct {
	TrackEvents []jsonEvent `json:"TrackEvents"`
}

type jsonEvent struct {
	ID                 string `json:"Id"`
	Text               string `json:"Text"`
	StartPositionTicks int64  `json:"StartPositionTicks"`
	EndPositionTicks   int64  `json:"EndPositionTicks"`
}

func ticks(d time.Duration) int64 { return int64(d / 100) }

func writeJSON(s *Subtitle) ([]byte, error) {
	t := jsonTrack{TrackEvents: make([]jsonEvent, len(s.Cues))}
	for i, c := range s.Cues {
		t.TrackEvents[i] = jsonEvent{
			ID:                 fmt.Sprint(c.Index),
			Text:               c.Text,
			StartPositionTicks: ticks(c.Start),
			EndPositionTicks:   ticks(c.End),
		}
	}
	return json.Marshal(t)
}

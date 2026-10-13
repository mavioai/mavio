package subsync

import (
	"math"
	"regexp"
	"strings"
	"time"
	"unicode"

	"github.com/mavioai/mavio/libs/media/vad"
	"github.com/mavioai/mavio/libs/subtitle"
)

// SampleRate is the number of speech samples a second.
const SampleRate = 100

// maxCue is the longest a cue counts as speech, as ffsubsync's
// --max-subtitle-seconds default.
const maxCue = 10 * time.Second

// Speech holds, for every 1/SampleRate of a second from the start, how
// likely someone speaks, from 0 to 1.
type Speech []float32

// FromCues returns the speech of a subtitle whose cue times are multiplied
// by ratio: cues show speech, at most maxCue of each, except cues that are
// sound descriptions, music or credits. Where ratio is above 1, the speech
// is weighted 1/ratio, as ffsubsync weighs it.
func FromCues(cues []subtitle.Cue, ratio float64) Speech {
	var maxTime float64
	type span struct{ start, end float64 }
	spans := make([]span, len(cues))
	for i, c := range cues {
		end := min(c.End, c.Start+maxCue)
		s, e := scaled(c.Start, ratio), scaled(end, ratio)
		spans[i] = span{s, e}
		maxTime = max(maxTime, e)
	}
	out := make(Speech, int(maxTime*SampleRate)+2)
	weight := float32(min(1/ratio, 1))
	for i, c := range cues {
		if isMetadata(c.Text, i == 0 || i+1 == len(cues)) {
			continue
		}
		start := int(math.RoundToEven(spans[i].start * SampleRate))
		end := start + int(math.RoundToEven((spans[i].end-spans[i].start)*SampleRate))
		for j := max(start, 0); j < min(end, len(out)); j++ {
			out[j] = weight
		}
	}
	return out
}

// scaled returns d × ratio in seconds, rounded to the microsecond as
// Python's timedelta is.
func scaled(d time.Duration, ratio float64) float64 {
	return math.RoundToEven(d.Seconds()*ratio*1e6) / 1e6
}

// FromProbabilities returns the speech of Silero VAD's probabilities, one
// for each window of vad.WindowSize samples at vad.SampleRate.
func FromProbabilities(p []float32) Speech {
	const step = vad.SampleRate / SampleRate // samples of audio a speech sample spans
	n := (len(p)*vad.WindowSize + step - 1) / step
	out := make(Speech, n)
	for k := range out {
		out[k] = p[min((k*step+step/2)/vad.WindowSize, len(p)-1)]
	}
	return out
}

// span returns how many samples lie from the first to the last above one
// half, as ffsubsync's num_frames; false when none does.
func (s Speech) span() (int, bool) {
	first, last := -1, -1
	for i, v := range s {
		if v > 0.5 {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	return last - first, first >= 0
}

var markup = regexp.MustCompile(`<[^>]+>`)

// nesters pairs the brackets a sound description or a note is wrapped in.
var nesters = map[rune]rune{'(': ')', '{': '}', '[': ']', '（': '）', '【': '】', '「': '」'}

// isMetadata reports whether a cue's text is no speech: empty, wrapped in
// brackets, only music symbols, or, first or last, a credit naming English
// or holding " - ".
func isMetadata(text string, firstOrLast bool) bool {
	text = strings.TrimFunc(markup.ReplaceAllString(text, ""), unicode.IsSpace)
	if text == "" {
		return true
	}
	runes := []rune(text)
	if closing, ok := nesters[runes[0]]; ok && runes[len(runes)-1] == closing {
		return true
	}
	if !strings.ContainsFunc(text, func(r rune) bool { return !unicode.IsSpace(r) && !strings.ContainsRune("♪♫♬♩🎵🎶", r) }) {
		return true
	}
	if firstOrLast {
		if strings.Contains(strings.ToLower(text), "english") || strings.Contains(text, " - ") {
			return true
		}
	}
	return false
}

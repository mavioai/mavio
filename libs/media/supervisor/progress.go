package supervisor

import (
	"bufio"
	"io"
	"strconv"
	"strings"
	"time"
)

// Progress is the state ffmpeg reports with -progress.
type Progress struct {
	// Position is the output time written so far.
	Position time.Duration
	Frame    int64
	FPS      float64
	// Speed is the transcoding speed relative to real time.
	Speed float64
	// Size is the output size in bytes.
	Size int64
	// Ended is set by the final report.
	Ended bool
}

// parseProgress reads -progress blocks, which end with a "progress=" line,
// and calls report for each.
func parseProgress(r io.Reader, report func(Progress)) {
	var p Progress
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		key, value, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok {
			continue
		}
		switch key {
		case "out_time_us", "out_time_ms": // both are microseconds
			if v, err := strconv.ParseInt(value, 10, 64); err == nil && v >= 0 {
				p.Position = time.Duration(v) * time.Microsecond
			}
		case "frame":
			p.Frame, _ = strconv.ParseInt(value, 10, 64)
		case "fps":
			p.FPS, _ = strconv.ParseFloat(value, 64)
		case "speed":
			p.Speed, _ = strconv.ParseFloat(strings.TrimSuffix(strings.TrimSpace(value), "x"), 64)
		case "total_size":
			p.Size, _ = strconv.ParseInt(value, 10, 64)
		case "progress":
			p.Ended = value == "end"
			report(p)
		}
	}
}

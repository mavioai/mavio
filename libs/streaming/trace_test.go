package streaming

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestStreamTracesRequests(t *testing.T) {
	// Files far enough apart that segment 2 is not written when asked for.
	f := &fakeFFmpeg{last: 29, interval: 20 * time.Millisecond}
	s := newTestStream(t, encoded(t, 30), f)
	var buf bytes.Buffer
	s.cfg.Logger = slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	read(t, s, 0)
	read(t, s, 2)
	read(t, s, 20)
	read(t, s, 2)
	var got []string
	for line := range strings.Lines(buf.String()) {
		if !strings.Contains(line, `msg="stream request"`) {
			continue
		}
		fields := strings.Fields(line)
		var seg, decision string
		for _, f := range fields {
			if v, ok := strings.CutPrefix(f, "segment="); ok {
				seg = v
			}
			if v, ok := strings.CutPrefix(f, "decision="); ok {
				decision = v
			}
		}
		got = append(got, seg+":"+decision)
	}
	want := []string{"0:restart", "2:wait", "20:restart", "2:written"}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("traces = %v, want = %v\n%s", got, want, buf.String())
	}
}

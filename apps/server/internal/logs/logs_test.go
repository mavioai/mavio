package logs

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestRing(t *testing.T) {
	var out bytes.Buffer
	ring := NewRing(3)
	log := slog.New(ring.Handler(slog.NewTextHandler(&out, &slog.HandlerOptions{Level: slog.LevelWarn}), slog.LevelInfo))
	log.Debug("dropped")
	log.With("plugin", "tmdb").WithGroup("req").Info("one", "n", 1)
	log.Warn("two")
	log.Error("three")
	log.Info("four")
	got := ring.Records(slog.LevelInfo, time.Time{}, 10)
	var msgs []string
	for _, r := range got {
		msgs = append(msgs, r.Message)
	}
	if strings.Join(msgs, ",") != "four,three,two" {
		t.Errorf("records = %v, want = four,three,two", msgs)
	}
	if got := ring.Records(slog.LevelError, time.Time{}, 10); len(got) != 1 || got[0].Message != "three" {
		t.Errorf("errors = %+v", got)
	}
	// Only warnings and errors reach the server's log.
	if s := out.String(); strings.Contains(s, "one") || !strings.Contains(s, "three") {
		t.Errorf("log = %s", s)
	}
	ring = NewRing(5)
	slog.New(ring.Handler(slog.DiscardHandler, slog.LevelInfo)).With("plugin", "tmdb").WithGroup("req").Info("one", "n", 1)
	if a := ring.Records(slog.LevelInfo, time.Time{}, 1)[0].Attributes; a["plugin"] != "tmdb" || a["req.n"] != "1" {
		t.Errorf("attributes = %v", a)
	}
}

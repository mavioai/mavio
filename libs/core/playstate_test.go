package core

import (
	"testing"
	"time"
)

func TestRecordPosition(t *testing.T) {
	now := time.Date(2026, 10, 9, 20, 0, 0, 0, time.UTC)
	const hour = time.Hour
	tests := []struct {
		name      string
		kind      ItemKind
		runtime   time.Duration
		pos       time.Duration
		wantPos   time.Duration
		completed bool
		lastSeen  bool // LastPlayedAt set
	}{
		{"movie start", KindMovie, 2 * hour, 3 * time.Minute, 0, false, false},
		{"movie middle", KindMovie, 2 * hour, hour, hour, false, true},
		{"movie past 90%", KindMovie, 2 * hour, 109 * time.Minute, 0, true, true},
		{"movie at 90%", KindMovie, 100 * time.Minute, 90 * time.Minute, 90 * time.Minute, false, true},
		{"short video middle", KindVideo, 4 * time.Minute, 2 * time.Minute, 0, true, true},
		{"no position", KindEpisode, hour, 0, 0, false, false},
		{"no runtime", KindEpisode, 0, 10 * time.Minute, 0, true, true},
		{"audiobook start", KindAudioBook, 10 * hour, 4 * time.Minute, 0, false, false},
		{"audiobook middle", KindAudioBook, 10 * hour, 9 * hour, 9 * hour, false, true},
		{"audiobook end", KindAudioBook, 10 * hour, 10*hour - 4*time.Minute, 0, true, true},
		{"book", KindBook, hour, 2 * time.Minute, 2 * time.Minute, false, true},
		{"track middle keeps no position", KindTrack, 4 * time.Minute, 2 * time.Minute, 0, true, true},
		{"track start", KindTrack, 4 * time.Minute, 5 * time.Second, 0, false, false},
		{"photo", KindPhoto, 0, 0, 0, false, false},
	}
	for _, tt := range tests {
		d := UserData{Position: 42 * time.Second}
		item := Item{Kind: tt.kind, Runtime: tt.runtime}
		completed := d.RecordPosition(&item, tt.pos, now)
		if completed != tt.completed || d.Position != tt.wantPos || d.Played != tt.completed || (d.LastPlayedAt != nil) != tt.lastSeen {
			t.Errorf("%s: got = completed %v, position %v, played %v, last played %v; want = %v, %v, %v, set %v",
				tt.name, completed, d.Position, d.Played, d.LastPlayedAt, tt.completed, tt.wantPos, tt.completed, tt.lastSeen)
		}
	}

	// A played item stays played when watched again from the start.
	d := UserData{Played: true}
	d.RecordPosition(&Item{Kind: KindMovie, Runtime: 2 * hour}, time.Minute, now)
	if !d.Played {
		t.Error("rewatch: got = unplayed, want = still played")
	}
}

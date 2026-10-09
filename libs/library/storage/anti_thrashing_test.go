package storage

import (
	"testing"
	"time"
)

func TestVolumeLedger_PathClaim(t *testing.T) {
	ledger := NewVolumeLedger(30*time.Second, 20*time.Second)

	p1 := "/media/movies/movie1.mkv"
	if !ledger.ClaimPath(p1) {
		t.Fatalf("first ClaimPath(%q) = false, want true", p1)
	}
	if ledger.ClaimPath(p1) {
		t.Fatalf("second ClaimPath(%q) = true, want false while in-flight", p1)
	}

	ledger.FinishPath(p1)
	if !ledger.ClaimPath(p1) {
		t.Fatalf("ClaimPath(%q) after FinishPath = false, want true", p1)
	}
	ledger.FinishPath(p1)
}

func TestVolumeLedger_VolumeSerialization(t *testing.T) {
	ledger := NewVolumeLedger(30*time.Second, 20*time.Second)
	vol := "dev:12345"

	// Initial read slot claim
	if !ledger.ClaimVolumeRead(vol, 1) {
		t.Fatalf("ClaimVolumeRead(%q, 1) = false, want true", vol)
	}
	if !ledger.IsVolumeReadInFlight(vol) {
		t.Fatalf("IsVolumeReadInFlight(%q) = false, want true", vol)
	}

	// Secondary read slot claim while busy must fail
	if ledger.ClaimVolumeRead(vol, 2) {
		t.Fatalf("second ClaimVolumeRead(%q, 2) = true, want false while busy", vol)
	}

	// Register a pending candidate for the busy volume
	p2 := "/media/movies/movie2.mkv"
	ledger.NotePendingEntry(vol, p2, 2)

	// HasNewerCandidate should report true for another file with higher seq
	p1 := "/media/movies/movie1.mkv"
	if !ledger.HasNewerCandidate(vol, p1) {
		t.Fatalf("HasNewerCandidate(%q, %q) = false, want true", vol, p1)
	}

	// Release active read and hand off to next candidate
	cand, ok := ledger.ReleaseVolumeRead(vol)
	if !ok || cand.Path != p2 || cand.Seq != 2 {
		t.Fatalf("ReleaseVolumeRead(%q) = (%+v, %v), want (%q, 2, true)", vol, cand, ok, p2)
	}

	if ledger.IsVolumeReadInFlight(vol) {
		t.Fatalf("IsVolumeReadInFlight after release = true, want false")
	}
}

func TestVolumeLedger_WakeCooldown(t *testing.T) {
	cooldown := 100 * time.Millisecond
	ledger := NewVolumeLedger(cooldown, cooldown)
	vol := "dev:remote1"

	t0 := time.Now()
	if !ledger.ClaimVolumeWake(vol, t0) {
		t.Fatalf("first ClaimVolumeWake = false, want true")
	}

	// Repeated claim within cooldown must be suppressed
	if ledger.ClaimVolumeWake(vol, t0.Add(50*time.Millisecond)) {
		t.Fatalf("ClaimVolumeWake within cooldown = true, want false")
	}

	// Claim after cooldown expires must succeed
	if !ledger.ClaimVolumeWake(vol, t0.Add(150*time.Millisecond)) {
		t.Fatalf("ClaimVolumeWake after cooldown = false, want true")
	}

	// Revoke allows immediate retry
	ledger.RevokeVolumeWake(vol)
	if !ledger.ClaimVolumeWake(vol, t0.Add(160*time.Millisecond)) {
		t.Fatalf("ClaimVolumeWake after revoke = false, want true")
	}
}

func TestVolumeLedger_IntentSequencing(t *testing.T) {
	ledger := NewVolumeLedger(time.Minute, time.Minute)
	path := "/media/show/ep1.mkv"

	s1 := ledger.NoteIntent(path)
	if s1 != 1 {
		t.Errorf("first NoteIntent = %d, want 1", s1)
	}
	s2 := ledger.NoteIntent(path)
	if s2 != 2 {
		t.Errorf("second NoteIntent = %d, want 2", s2)
	}

	if got := ledger.IntentSeq(path); got != 2 {
		t.Errorf("IntentSeq = %d, want 2", got)
	}
}

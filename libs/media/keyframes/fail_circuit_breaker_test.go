package keyframes

import (
	"testing"
	"time"
)

func TestFailCircuitBreaker_SuppressionAndTTL(t *testing.T) {
	cb := NewFailCircuitBreaker(5*time.Second, 4)
	t0 := time.Now()

	target := 10 * time.Second
	gop := 2 * time.Second // window [8s, 12s]

	if cb.IsSuppressed(target, t0) {
		t.Fatalf("expected target not to be suppressed initially")
	}

	cb.RecordFailure(target, gop, t0)

	// Within failure window:
	if !cb.IsSuppressed(target, t0.Add(1*time.Second)) {
		t.Errorf("expected target 10s to be suppressed at t0+1s")
	}
	if !cb.IsSuppressed(8*time.Second, t0.Add(1*time.Second)) {
		t.Errorf("expected target 8s to be suppressed at t0+1s")
	}
	if !cb.IsSuppressed(12*time.Second, t0.Add(1*time.Second)) {
		t.Errorf("expected target 12s to be suppressed at t0+1s")
	}

	// Outside failure window:
	if cb.IsSuppressed(7*time.Second, t0.Add(1*time.Second)) {
		t.Errorf("expected target 7s not to be suppressed")
	}
	if cb.IsSuppressed(13*time.Second, t0.Add(1*time.Second)) {
		t.Errorf("expected target 13s not to be suppressed")
	}

	// After TTL:
	if cb.IsSuppressed(target, t0.Add(6*time.Second)) {
		t.Errorf("expected target to no longer be suppressed after TTL (6s > 5s)")
	}
}

func TestFailCircuitBreaker_CapacityEviction(t *testing.T) {
	cap := 3
	cb := NewFailCircuitBreaker(10*time.Second, cap)
	t0 := time.Now()

	for i := 1; i <= 5; i++ {
		cb.RecordFailure(time.Duration(i*10)*time.Second, 1*time.Second, t0)
	}

	// Cap is 3, so items 1 (10s) and 2 (20s) should have been evicted
	if cb.IsSuppressed(10*time.Second, t0) {
		t.Errorf("expected 10s to be evicted")
	}
	if cb.IsSuppressed(20*time.Second, t0) {
		t.Errorf("expected 20s to be evicted")
	}

	// Items 3, 4, 5 (30s, 40s, 50s) should remain
	if !cb.IsSuppressed(30*time.Second, t0) {
		t.Errorf("expected 30s to be retained")
	}
	if !cb.IsSuppressed(40*time.Second, t0) {
		t.Errorf("expected 40s to be retained")
	}
	if !cb.IsSuppressed(50*time.Second, t0) {
		t.Errorf("expected 50s to be retained")
	}
}

func TestKeyedFailCircuitBreaker(t *testing.T) {
	kb := NewKeyedFailCircuitBreaker(5*time.Second, 4)
	t0 := time.Now()

	keyA := "movie_a.mkv"
	keyB := "movie_b.mkv"
	target := 10 * time.Second

	kb.RecordFailure(keyA, target, 0, t0) // default 2.5s window: [7.5s, 12.5s]

	if !kb.IsSuppressed(keyA, target, t0) {
		t.Errorf("keyA at 10s should be suppressed")
	}
	if kb.IsSuppressed(keyB, target, t0) {
		t.Errorf("keyB at 10s should not be suppressed")
	}

	kb.Clear(keyA)
	if kb.IsSuppressed(keyA, target, t0) {
		t.Errorf("keyA should not be suppressed after Clear")
	}
}

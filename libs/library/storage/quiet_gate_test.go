package storage

import (
	"context"
	"testing"
	"time"
)

func TestQuietGate_Basic(t *testing.T) {
	gate := NewQuietGate()

	if gate.IsActive() {
		t.Errorf("initially IsActive = true, want false")
	}

	gate.NoteActivity(200 * time.Millisecond)
	if !gate.IsActive() {
		t.Errorf("after NoteActivity, IsActive = false, want true")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	err := gate.Wait(ctx)
	if err != nil {
		t.Fatalf("Wait failed: %v", err)
	}

	elapsed := time.Since(start)
	if elapsed < 150*time.Millisecond {
		t.Errorf("Wait returned after %v, want >= 150ms", elapsed)
	}

	if gate.IsActive() {
		t.Errorf("after wait, IsActive = true, want false")
	}
}

func TestQuietGate_CoalesceExtension(t *testing.T) {
	gate := NewQuietGate()

	gate.NoteActivity(100 * time.Millisecond)
	d1 := gate.QuietUntil()

	time.Sleep(20 * time.Millisecond)
	gate.NoteActivity(200 * time.Millisecond)
	d2 := gate.QuietUntil()

	if !d2.After(d1) {
		t.Errorf("subsequent NoteActivity did not extend quietUntil: d1=%v, d2=%v", d1, d2)
	}
}

func TestQuietGate_WaitContextCancelled(t *testing.T) {
	gate := NewQuietGate()
	gate.NoteActivity(5 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := gate.Wait(ctx)
	if err == nil {
		t.Fatalf("Wait on cancelled context got nil, want context error")
	}
}

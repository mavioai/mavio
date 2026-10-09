package storage

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPrefetchHeadTail(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "test_prefetch.bin")

	// Create 2MB test file with dummy data
	data := make([]byte, 2<<20)
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	// Prefetch with default budgets
	if err := PrefetchHeadTail(filePath, 0, 0); err != nil {
		t.Fatalf("PrefetchHeadTail(default) failed: %v", err)
	}

	// Prefetch with custom small budgets
	if err := PrefetchHeadTail(filePath, 64<<10, 32<<10); err != nil {
		t.Fatalf("PrefetchHeadTail(custom) failed: %v", err)
	}
}

func TestPrefetchHeadTail_SmallFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "small.bin")

	// 50KB file (smaller than head budget)
	data := make([]byte, 50<<10)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := PrefetchHeadTail(filePath, 1<<20, 256<<10); err != nil {
		t.Fatalf("PrefetchHeadTail on small file failed: %v", err)
	}
}

func TestPrefetchHeadTail_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "empty.bin")

	if err := os.WriteFile(filePath, nil, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := PrefetchHeadTail(filePath, 1<<20, 256<<10); err != nil {
		t.Fatalf("PrefetchHeadTail on empty file failed: %v", err)
	}
}

func TestVolumeHeartbeat(t *testing.T) {
	tempDir := t.TempDir()
	filePath := filepath.Join(tempDir, "heartbeat.bin")

	data := make([]byte, 64<<10)
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	if err := VolumeHeartbeat(filePath); err != nil {
		t.Fatalf("VolumeHeartbeat failed: %v", err)
	}
}

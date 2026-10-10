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

func TestPrefetchRange(t *testing.T) {
	p := filepath.Join(t.TempDir(), "range.bin")
	if err := os.WriteFile(p, make([]byte, 1<<20), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, r := range [][2]int64{{0, 4096}, {512 << 10, 1 << 20}, {2 << 20, 10}, {-5, 10}} {
		if err := PrefetchRange(p, r[0], r[1]); err != nil {
			t.Errorf("PrefetchRange(%d, %d) = %v, want nil", r[0], r[1], err)
		}
	}
}

package subtitle

import (
	"bytes"
	"testing"
)

func TestFileHash(t *testing.T) {
	// A file of 128 KiB whose words are all one: the size plus 16384 ones.
	data := bytes.Repeat([]byte{1, 0, 0, 0, 0, 0, 0, 0}, 2*hashChunk/8)
	got, err := FileHash(bytes.NewReader(data), int64(len(data)))
	if want := "0000000000024000"; err != nil || got != want {
		t.Errorf("FileHash = %q, %v, want = %q", got, err, want)
	}
	// The sum wraps around: 64 KiB less 16384.
	data = bytes.Repeat([]byte{0xff}, hashChunk)
	got, err = FileHash(bytes.NewReader(data), int64(len(data)))
	if want := "000000000000c000"; err != nil || got != want {
		t.Errorf("FileHash of ones = %q, %v, want = %q", got, err, want)
	}
	if _, err := FileHash(bytes.NewReader(nil), 10); err == nil {
		t.Error("FileHash of a small file: no error")
	}
}

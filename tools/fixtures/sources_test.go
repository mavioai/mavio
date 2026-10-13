package fixtures

import (
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFetchSources(t *testing.T) {
	body := "recorded speech"
	sum := sha256.Sum256([]byte(body))
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		requests++
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	dir := t.TempDir()
	good := Spec{Name: "a.wav", Sources: map[string]Source{"in.wav": {URL: srv.URL, SHA256: hex.EncodeToString(sum[:])}}}
	folder, err := fetchSources(t.Context(), dir, good, slog.Default())
	if err != nil {
		t.Fatalf("fetchSources: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(folder, hex.EncodeToString(sum[:])))
	if err != nil || string(data) != body {
		t.Fatalf("fetched = %q, %v, want %q", data, err, body)
	}
	if _, err := fetchSources(t.Context(), dir, good, slog.Default()); err != nil || requests != 1 {
		t.Errorf("second fetchSources: err = %v, requests = %d, want nil, 1", err, requests)
	}
	args := ffmpegArgs(Spec{Sources: good.Sources, Args: []string{"-i", "{source:in.wav}"}}, "", folder, "out.wav")
	if got, want := args[6], filepath.Join(folder, hex.EncodeToString(sum[:])); got != want {
		t.Errorf("source argument = %q, want %q", got, want)
	}

	bad := Spec{Name: "b.wav", Sources: map[string]Source{"in.wav": {URL: srv.URL, SHA256: strings.Repeat("0", 64)}}}
	if _, err := fetchSources(t.Context(), dir, bad, slog.Default()); err == nil || !strings.Contains(err.Error(), "sha256") {
		t.Errorf("fetchSources with a wrong hash: err = %v, want a sha256 mismatch", err)
	}
	if _, err := os.Stat(filepath.Join(folder, strings.Repeat("0", 64))); err == nil {
		t.Error("a source with a wrong hash was kept")
	}
}

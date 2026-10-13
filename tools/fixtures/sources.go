package fixtures

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// sourcesDir is the folder of the fixtures directory keeping fetched
// sources, each named by its SHA-256.
const sourcesDir = ".sources"

// fetchSources fetches the sources spec needs that are not kept yet and
// returns the folder keeping them.
func fetchSources(ctx context.Context, dir string, spec Spec, log *slog.Logger) (string, error) {
	folder := filepath.Join(dir, sourcesDir)
	for name, src := range spec.Sources {
		path := filepath.Join(folder, src.SHA256)
		if sum, err := fileSHA256(path); err == nil && sum == src.SHA256 {
			continue
		}
		log.InfoContext(ctx, "fetching fixture source", "name", name, "url", src.URL)
		if err := fetch(ctx, src, path); err != nil {
			return "", fmt.Errorf("source %s: %w", name, err)
		}
	}
	return folder, nil
}

// fetch downloads src to path, failing when its hash differs.
func fetch(ctx context.Context, src Source, path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.URL, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: %s", src.URL, resp.Status)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".fetch-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("GET %s: %w", src.URL, err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if sum := hex.EncodeToString(h.Sum(nil)); sum != src.SHA256 {
		return fmt.Errorf("GET %s: sha256 %s, want %s", src.URL, sum, src.SHA256)
	}
	return os.Rename(tmp.Name(), path)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

package main

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/mavioai/mavio/plugins/indexers/internal/sites/cardigann"
)

// The definitions Prowlarr reads, from the branch and version of the
// definition format it takes.
const (
	definitionsURL = "https://indexers.prowlarr.com/master/11"
	// definitionsInterval is how often the definitions are updated, as
	// Prowlarr's IndexerDefinitionUpdateCommand runs.
	definitionsInterval = 24 * time.Hour
	// maxPackage bounds the definitions' package.
	maxPackage = 64 << 20
)

// prowlarrVersion is the version of Prowlarr ported, whose User-Agent
// requests for definitions carry.
const prowlarrVersion = "2.6.5.5620"

// definitionStore keeps Cardigann definitions in a directory: those of
// indexers.prowlarr.com, updated in place, and the administrator's own in
// its Custom directory.
type definitionStore struct {
	dir string
	// url is that of the definitions, without "/package.zip".
	url       string
	userAgent string
	client    *http.Client
	log       *slog.Logger
}

// Update downloads the definitions' package and extracts it into the
// directory, as Prowlarr's UpdateLocalDefinitions does.
func (s *definitionStore) Update(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(s.url, "/")+"/package.zip", nil)
	if err != nil {
		return fmt.Errorf("update definitions: %w", err)
	}
	req.Header.Set("User-Agent", s.userAgent)
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("update definitions: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("update definitions: HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxPackage+1))
	if err != nil {
		return fmt.Errorf("update definitions: %w", err)
	}
	if len(data) > maxPackage {
		return errors.New("update definitions: the package is too large")
	}
	if err := s.extract(data); err != nil {
		return fmt.Errorf("update definitions: %w", err)
	}
	s.log.DebugContext(ctx, "updated indexer definitions")
	return nil
}

// extract writes the package's definitions into the directory, replacing
// those there; files that would land outside it are left out.
func (s *definitionStore) extract(data []byte) error {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	root, err := os.OpenRoot(s.dir)
	if err != nil {
		return err
	}
	defer root.Close()
	for _, f := range zr.File {
		name := path.Clean(f.Name)
		if f.FileInfo().IsDir() || !strings.HasSuffix(name, ".yml") {
			continue
		}
		if !filepath.IsLocal(filepath.FromSlash(name)) {
			s.log.Warn("definition outside the definitions' directory left out", "file", f.Name)
			continue
		}
		if dir := path.Dir(name); dir != "." {
			if err := root.MkdirAll(filepath.FromSlash(dir), 0o755); err != nil {
				return err
			}
		}
		if err := writeFile(root, filepath.FromSlash(name), f); err != nil {
			return fmt.Errorf("%s: %w", f.Name, err)
		}
	}
	return nil
}

func writeFile(root *os.Root, name string, f *zip.File) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close()
	w, err := root.Create(name)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, r); err != nil {
		w.Close()
		return err
	}
	return w.Close()
}

// Load reads the definitions in the directory and its Custom directory;
// none before the first update.
func (s *definitionStore) Load() ([]*cardigann.Definition, error) {
	if _, err := os.Stat(s.dir); errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	return cardigann.LoadDefinitions(os.DirFS(s.dir), s.log)
}

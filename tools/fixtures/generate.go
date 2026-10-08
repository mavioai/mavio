package fixtures

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// manifestName is the file in the fixtures directory that records how each
// fixture was generated.
const manifestName = "manifest.json"

// Options configures Generate.
type Options struct {
	// Dir is the output directory.
	Dir string
	// FFmpeg is the ffmpeg executable; defaults to "ffmpeg".
	FFmpeg string
	// Only restricts generation to the named fixtures; empty means all.
	Only []string
	// Force regenerates fixtures even when they are up to date.
	Force bool
	// Logger receives progress messages; defaults to slog.Default().
	Logger *slog.Logger
}

// Result reports what Generate did with each fixture.
type Result struct {
	Generated []string
	UpToDate  []string
	// Skipped maps fixture names to the reason they were skipped.
	Skipped map[string]string
}

type manifest struct {
	FFmpeg   string                   `json:"ffmpeg"`
	Fixtures map[string]manifestEntry `json:"fixtures"`
}

type manifestEntry struct {
	Key         string `json:"key"`
	Description string `json:"description"`
}

// Generate creates the fixtures in the catalog that are missing or out of
// date. A fixture is up to date when its file exists and the manifest records
// the same key, a hash of the ffmpeg version, arguments and auxiliary files.
func Generate(ctx context.Context, opts Options) (Result, error) {
	if opts.FFmpeg == "" {
		opts.FFmpeg = "ffmpeg"
	}
	log := opts.Logger
	if log == nil {
		log = slog.Default()
	}
	res := Result{Skipped: map[string]string{}}

	specs, err := selectSpecs(Catalog(), opts.Only)
	if err != nil {
		return res, err
	}
	version, encoders, err := probeFFmpeg(ctx, opts.FFmpeg)
	if err != nil {
		return res, err
	}
	if err := os.MkdirAll(opts.Dir, 0o755); err != nil {
		return res, err
	}
	man := readManifest(filepath.Join(opts.Dir, manifestName))
	man.FFmpeg = version

	for _, spec := range specs {
		if missing := missingEncoders(spec, encoders); len(missing) > 0 {
			reason := "ffmpeg lacks encoders: " + strings.Join(missing, ", ")
			res.Skipped[spec.Name] = reason
			log.WarnContext(ctx, "skipping fixture", "name", spec.Name, "reason", reason)
			continue
		}
		key := specKey(version, spec)
		out := filepath.Join(opts.Dir, spec.Name)
		if !opts.Force && man.Fixtures[spec.Name].Key == key && fileExists(out) {
			res.UpToDate = append(res.UpToDate, spec.Name)
			continue
		}
		log.InfoContext(ctx, "generating fixture", "name", spec.Name)
		if err := generateOne(ctx, opts.FFmpeg, opts.Dir, spec); err != nil {
			return res, fmt.Errorf("fixture %s: %w", spec.Name, err)
		}
		man.Fixtures[spec.Name] = manifestEntry{Key: key, Description: spec.Description}
		res.Generated = append(res.Generated, spec.Name)
	}
	return res, writeManifest(filepath.Join(opts.Dir, manifestName), man)
}

func selectSpecs(all []Spec, only []string) ([]Spec, error) {
	if len(only) == 0 {
		return all, nil
	}
	var out []Spec
	for _, name := range only {
		i := slices.IndexFunc(all, func(s Spec) bool { return s.Name == name })
		if i < 0 {
			return nil, fmt.Errorf("unknown fixture %q", name)
		}
		out = append(out, all[i])
	}
	return out, nil
}

// probeFFmpeg returns the first line of `ffmpeg -version` and the set of
// available encoders.
func probeFFmpeg(ctx context.Context, ffmpeg string) (string, map[string]bool, error) {
	out, err := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-version").Output()
	if err != nil {
		return "", nil, fmt.Errorf("run %s -version: %w", ffmpeg, err)
	}
	version, _, _ := strings.Cut(string(out), "\n")

	out, err = exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-encoders").Output()
	if err != nil {
		return "", nil, fmt.Errorf("run %s -encoders: %w", ffmpeg, err)
	}
	return strings.TrimSpace(version), parseEncoders(out), nil
}

// parseEncoders parses `ffmpeg -encoders` output, whose entries look like
// " V....D libx264              libx264 H.264 / AVC ...".
func parseEncoders(out []byte) map[string]bool {
	encoders := map[string]bool{}
	sc := bufio.NewScanner(bytes.NewReader(out))
	inList := false
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 1 && fields[0] == "------" {
			inList = true
			continue
		}
		if inList && len(fields) >= 2 && len(fields[0]) == 6 {
			encoders[fields[1]] = true
		}
	}
	return encoders
}

func missingEncoders(spec Spec, available map[string]bool) []string {
	var missing []string
	for _, e := range spec.Encoders {
		if !available[e] {
			missing = append(missing, e)
		}
	}
	return missing
}

func specKey(version string, spec Spec) string {
	h := sha256.New()
	fmt.Fprintln(h, version)
	for _, a := range ffmpegArgs(spec, "", "") {
		fmt.Fprintln(h, a)
	}
	for _, name := range slices.Sorted(maps.Keys(spec.Files)) {
		fmt.Fprintln(h, name)
		fmt.Fprintln(h, spec.Files[name])
	}
	return hex.EncodeToString(h.Sum(nil))
}

// ffmpegArgs returns the full argument list for spec, with auxiliary file
// references resolved against auxDir and the output written to out.
func ffmpegArgs(spec Spec, auxDir, out string) []string {
	args := []string{"-hide_banner", "-nostdin", "-loglevel", "error", "-y"}
	for _, a := range spec.Args {
		if name, ok := strings.CutPrefix(a, "{file:"); ok {
			a = filepath.Join(auxDir, strings.TrimSuffix(name, "}"))
		}
		args = append(args, a)
	}
	// Bit-exact muxing and single-threaded encoding keep output reproducible
	// for a given ffmpeg build. Codec-level -flags are left to the specs, since
	// a later -flags option would replace theirs.
	return append(args, "-threads", "1", "-fflags", "+bitexact", out)
}

func generateOne(ctx context.Context, ffmpeg, dir string, spec Spec) error {
	auxDir, err := os.MkdirTemp("", "mavio-fixture-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(auxDir)
	for name, content := range spec.Files {
		if err := os.WriteFile(filepath.Join(auxDir, name), []byte(content), 0o644); err != nil {
			return err
		}
	}

	// Write next to the destination with the same extension (ffmpeg picks the
	// muxer from it), then rename atomically.
	tmp := filepath.Join(dir, ".tmp-"+spec.Name)
	cmd := exec.CommandContext(ctx, ffmpeg, ffmpegArgs(spec, auxDir, tmp)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(stderr.String()))
	}
	return os.Rename(tmp, filepath.Join(dir, spec.Name))
}

func readManifest(path string) manifest {
	man := manifest{Fixtures: map[string]manifestEntry{}}
	data, err := os.ReadFile(path)
	if err != nil {
		return man
	}
	if json.Unmarshal(data, &man) != nil || man.Fixtures == nil {
		return manifest{Fixtures: map[string]manifestEntry{}}
	}
	return man
}

func writeManifest(path string, man manifest) error {
	data, err := json.MarshalIndent(man, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

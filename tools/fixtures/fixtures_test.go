package fixtures

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCatalog(t *testing.T) {
	seen := map[string]bool{}
	for _, s := range Catalog() {
		if seen[s.Name] {
			t.Errorf("duplicate fixture name %q", s.Name)
		}
		seen[s.Name] = true
		if filepath.Ext(s.Name) == "" {
			t.Errorf("%s: name has no extension; ffmpeg needs it to pick a muxer", s.Name)
		}
		if s.Description == "" || len(s.Encoders) == 0 {
			t.Errorf("%s: description and encoders are required", s.Name)
		}
		for _, a := range s.Args {
			if name, ok := strings.CutPrefix(a, "{file:"); ok {
				if _, ok := s.Files[strings.TrimSuffix(name, "}")]; !ok {
					t.Errorf("%s: argument %q references an undefined file", s.Name, a)
				}
			}
		}
	}
}

func TestParseEncoders(t *testing.T) {
	out := []byte(`Encoders:
 V..... = Video
 A..... = Audio
 ------
 V....D libx264              libx264 H.264 / AVC / MPEG-4 AVC / MPEG-4 part 10 (codec h264)
 A....D aac                  AAC (Advanced Audio Coding)
 S..... srt                  SubRip subtitle (codec subrip)
`)
	got := parseEncoders(out)
	for _, want := range []string{"libx264", "aac", "srt"} {
		if !got[want] {
			t.Errorf("parseEncoders missing %q; got %v", want, got)
		}
	}
	if got["="] || got["Video"] || len(got) != 3 {
		t.Errorf("parseEncoders = %v, want exactly libx264, aac, srt", got)
	}
}

func TestSpecKeyChangesWithInputs(t *testing.T) {
	spec := Catalog()[0]
	base := specKey("ffmpeg version 1", spec)
	if specKey("ffmpeg version 2", spec) == base {
		t.Error("key does not depend on the ffmpeg version")
	}
	changed := spec
	changed.Args = append(slices.Clone(spec.Args), "-an")
	if specKey("ffmpeg version 1", changed) == base {
		t.Error("key does not depend on the arguments")
	}
}

func TestGenerate(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not found in PATH")
	}
	dir := t.TempDir()
	opts := Options{Dir: dir, Only: []string{"tagged.flac"}}

	res, err := Generate(t.Context(), opts)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if reason, ok := res.Skipped["tagged.flac"]; ok {
		t.Skipf("fixture skipped: %s", reason)
	}
	if got, want := res.Generated, []string{"tagged.flac"}; !slices.Equal(got, want) {
		t.Fatalf("Generated = %v, want %v", got, want)
	}
	if info, err := os.Stat(filepath.Join(dir, "tagged.flac")); err != nil || info.Size() == 0 {
		t.Fatalf("fixture not written: %v", err)
	}

	res, err = Generate(t.Context(), opts)
	if err != nil {
		t.Fatalf("second Generate: %v", err)
	}
	if got, want := res.UpToDate, []string{"tagged.flac"}; !slices.Equal(got, want) {
		t.Errorf("second run UpToDate = %v, want %v", got, want)
	}
}

func TestRequireSkipsMissingFixture(t *testing.T) {
	t.Setenv(EnvDir, t.TempDir())
	ok := t.Run("missing", func(t *testing.T) {
		Require(t, "does-not-exist.mkv")
		t.Error("Require did not skip")
	})
	if !ok {
		t.Error("subtest failed instead of skipping")
	}
}

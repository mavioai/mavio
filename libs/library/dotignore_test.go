package library

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/fstest"
	"time"
)

var (
	rule1 = []string{"SPs"}
	rule2 = []string{"SPs", "!thebestshot.mkv"}
	rule3 = []string{"*.txt", `{\colortbl;\red255\green255\blue255;}`, "videos/", `\invalid\escape\sequence`, "*.mkv"}
	rule4 = []string{`{\colortbl;\red255\green255\blue255;}`, `\invalid\escape\sequence`}
)

// TestCheckIgnoreRules ports the TheoryData of CheckIgnoreRules_ReturnsExpectedResult
// and CheckIgnoreRules_WithWindowsPaths_NormalizesBackslashes, which
// testport cannot extract.
func TestCheckIgnoreRules(t *testing.T) {
	tests := []struct {
		rules     []string
		path      string
		normalize bool
		want      bool
	}{
		// Basic matching.
		{rule1, "f:/cd/sps/ffffff.mkv", false, true},
		{rule1, "cd/sps/ffffff.mkv", false, true},
		{rule1, "/cd/sps/ffffff.mkv", false, true},
		// Negation.
		{rule2, "f:/cd/sps/ffffff.mkv", false, true},
		{rule2, "cd/sps/ffffff.mkv", false, true},
		{rule2, "/cd/sps/ffffff.mkv", false, true},
		{rule2, "f:/cd/sps/thebestshot.mkv", false, false},
		{rule2, "cd/sps/thebestshot.mkv", false, false},
		{rule2, "/cd/sps/thebestshot.mkv", false, false},
		// Invalid patterns are skipped.
		{rule3, "test.txt", false, true},
		{rule3, "videos/movie.mp4", false, true},
		{rule3, "movie.mkv", false, true},
		{rule3, "test.mp3", false, false},
		// Only invalid patterns ignore everything.
		{rule4, "any-file.txt", false, true},
		{rule4, "any/path/to/file.mkv", false, true},
		// Windows paths.
		{rule1, `C:\cd\sps\ffffff.mkv`, true, true},
		{rule1, `D:\media\sps\movie.mkv`, true, true},
		{rule1, `\\server\share\sps\file.mkv`, true, true},
		{rule2, `C:\cd\sps\ffffff.mkv`, true, true},
		{rule2, `C:\cd\sps\thebestshot.mkv`, true, false},
		{rule3, `C:\videos\movie.mp4`, true, true},
		{rule3, `D:\documents\test.txt`, true, true},
		{rule3, `E:\music\song.mp3`, true, false},
	}
	for _, tt := range tests {
		if got := CheckIgnoreRules(tt.path, tt.rules, false, tt.normalize); got != tt.want {
			t.Errorf("%v %s: got = %v, want = %v", tt.rules, tt.path, got, tt.want)
		}
	}
}

func TestDotIgnoreCases(t *testing.T) {
	portedCases(t, "dot_ignore_ignore_rule.json", ported{
		run: map[string]func(t *testing.T, a args){
			"CheckIgnoreRules_WithWindowsPaths_WithoutNormalization_DoesNotMatch": func(t *testing.T, a args) {
				if CheckIgnoreRules(a.str(t, "path"), rule1, false, false) {
					t.Error("got = ignored, want = not")
				}
			},
		},
		facts: map[string]string{
			"CacheHit_RepeatedCallsDoNotRereadFiles":            "TestIgnoreFiles",
			"CacheInvalidation_ModifyIgnoreFile_Reparses":       "TestIgnoreFiles",
			"EmptyIgnoreFile_IgnoresEverything":                 "TestIgnoreFiles",
			"WhitespaceOnlyIgnoreFile_IgnoresEverything":        "TestIgnoreFiles",
			"NoIgnoreFile_DoesNotIgnore":                        "TestIgnoreFiles",
			"ConcurrentAccess_ThreadSafe":                       "TestIgnoreFilesConcurrent",
			"ClearCache_ClearsAllCachedData":                    "TestIgnoreFiles",
			"IgnoreFileDeleted_HandlesGracefully":               "TestIgnoreFiles",
			"ParentDirectoryIgnoreFile_AppliesToSubdirectories": "TestIgnoreFiles",
			"DirectoryMatching_TrailingSlashPattern":            "TestIgnoreFiles",
		},
	})
}

func ignored(t *testing.T, f *IgnoreFiles, p string, isDir bool) bool {
	t.Helper()
	got, err := f.Ignored(p, isDir)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestIgnoreFiles(t *testing.T) {
	t.Run("repeated lookups", func(t *testing.T) {
		f := &IgnoreFiles{FS: fstest.MapFS{".ignore": {Data: []byte("*.tmp")}, "subdir/x": {}}}
		for _, c := range []struct {
			p    string
			want bool
		}{{"subdir/test.tmp", true}, {"subdir/test.tmp", true}, {"subdir/other.tmp", true}, {"subdir/other.txt", false}} {
			if got := ignored(t, f, c.p, false); got != c.want {
				t.Errorf("%s: got = %v, want = %v", c.p, got, c.want)
			}
		}
	})
	t.Run("modified file is reparsed", func(t *testing.T) {
		dir := t.TempDir()
		name := filepath.Join(dir, ".ignore")
		write(t, name, "*.tmp")
		f := &IgnoreFiles{FS: os.DirFS(dir)}
		if !ignored(t, f, "test.tmp", false) {
			t.Fatal("tmp: got = not ignored")
		}
		write(t, name, "*.txt")
		later := time.Now().Add(time.Second)
		if err := os.Chtimes(name, later, later); err != nil {
			t.Fatal(err)
		}
		if ignored(t, f, "test.tmp", false) || !ignored(t, f, "test.txt", false) {
			t.Error("after edit: got = stale rules")
		}
	})
	t.Run("empty and blank files ignore everything", func(t *testing.T) {
		for _, content := range []string{"", "   \n\t\n   "} {
			f := &IgnoreFiles{FS: fstest.MapFS{".ignore": {Data: []byte(content)}}}
			if !ignored(t, f, "anyfile.mkv", false) {
				t.Errorf("%q: got = not ignored", content)
			}
		}
	})
	t.Run("no file ignores nothing", func(t *testing.T) {
		if ignored(t, &IgnoreFiles{FS: fstest.MapFS{}}, "anyfile.mkv", false) {
			t.Error("got = ignored")
		}
	})
	t.Run("deleted file", func(t *testing.T) {
		dir := t.TempDir()
		name := filepath.Join(dir, ".ignore")
		write(t, name, "*.tmp")
		f := &IgnoreFiles{FS: os.DirFS(dir)}
		if !ignored(t, f, "test.tmp", false) {
			t.Fatal("got = not ignored")
		}
		if err := os.Remove(name); err != nil {
			t.Fatal(err)
		}
		if ignored(t, f, "test.tmp", false) {
			t.Error("after delete: got = ignored")
		}
	})
	t.Run("parent file applies to subfolders", func(t *testing.T) {
		f := &IgnoreFiles{FS: fstest.MapFS{".ignore": {Data: []byte("*.tmp")}, "sub1/sub2/x": {}}}
		if !ignored(t, f, "sub1/sub2/test.tmp", false) || !ignored(t, f, "sub1/test.tmp", false) {
			t.Error("got = not ignored")
		}
	})
	t.Run("trailing slash matches folders", func(t *testing.T) {
		f := &IgnoreFiles{FS: fstest.MapFS{".ignore": {Data: []byte("videos/")}, "videos/x": {}}}
		if !ignored(t, f, "videos", true) {
			t.Error("folder: got = not ignored")
		}
		if ignored(t, f, "videos", false) {
			t.Error("file: got = ignored")
		}
	})
}

func TestIgnoreFilesConcurrent(t *testing.T) {
	f := &IgnoreFiles{FS: fstest.MapFS{".ignore": {Data: []byte("*.tmp")}}}
	var wg sync.WaitGroup
	for i := range 100 {
		wg.Go(func() {
			if got, err := f.Ignored("test.tmp", false); err != nil || !got {
				t.Errorf("%d tmp: got = %v, %v", i, got, err)
			}
			if got, err := f.Ignored("test.txt", false); err != nil || got {
				t.Errorf("%d txt: got = %v, %v", i, got, err)
			}
		})
	}
	wg.Wait()
}

func write(t *testing.T, name, content string) {
	t.Helper()
	if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

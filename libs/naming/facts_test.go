package naming

import "testing"

// Facts ported by hand from Jellyfin's naming tests.

func TestPortedAudioBookFileInfo(t *testing.T) {
	portedCases(t, "audio_book/audio_book_file_info.json", ported{
		facts: map[string]string{
			"CompareTo_Same_Success":  "TestAudioBookFileCompare",
			"CompareTo_Empty_Success": "TestAudioBookFileCompare",
		},
		skip: map[string]string{
			"CompareTo_Null_Success": "compares with a null reference; AudioBookFile is a value in Go",
		},
	})
}

func TestAudioBookFileCompare(t *testing.T) {
	a, b := AudioBookFile{}, AudioBookFile{}
	if got := a.Compare(a); got != 0 {
		t.Errorf("same: got = %d, want = 0", got)
	}
	if got := a.Compare(b); got != 0 {
		t.Errorf("empty: got = %d, want = 0", got)
	}
}

func TestPortedNamingOptions(t *testing.T) {
	portedCases(t, "common/naming_options.json", ported{facts: map[string]string{
		"TestNamingOptionsCompile":            "TestNamingOptionsCompile",
		"TestNamingOptionsEpisodeExpressions": "TestNamingOptionsCompile",
	}})
}

func TestNamingOptionsCompile(t *testing.T) {
	p := Default()
	if len(p.cleanDateTimes) == 0 || len(p.cleanStrings) == 0 {
		t.Error("clean expressions: got = none, want = compiled")
	}
	for _, expr := range []string{"", "test"} {
		o := DefaultOptions()
		o.EpisodeExpressions = []EpisodeExpression{{Expression: expr}}
		if _, err := New(o); err != nil {
			t.Errorf("episode expression %q: %v", expr, err)
		}
	}
	o := DefaultOptions()
	o.EpisodeExpressions = []EpisodeExpression{{Expression: "(unclosed"}}
	if _, err := New(o); err == nil {
		t.Error("invalid expression: got = no error, want = error")
	}
}

func TestPortedFormat3D(t *testing.T) {
	portedCases(t, "video/format_3d.json", ported{facts: map[string]string{
		"TestKodiFormat3D":        "TestFormat3D",
		"TestFormat3DAtEndOfPath": "TestFormat3D",
		"TestExpandedFormat3D":    "TestFormat3D",
		"TestResolveDirectory3D":  "TestFormat3DResolve",
		"Test3DName":              "TestFormat3DResolve",
	}})
}

func TestFormat3D(t *testing.T) {
	for path, want := range map[string]string{
		// TestKodiFormat3D
		"Super movie.3d.mp4": "", "Super movie.3d.hsbs.mp4": "hsbs", "Super movie.3d.sbs.mp4": "sbs",
		"Super movie.3d.htab.mp4": "htab", "Super movie.3d.tab.mp4": "tab", "Super movie 3d hsbs.mp4": "hsbs",
		// TestFormat3DAtEndOfPath: folder rips have no extension.
		"Super movie (2009) 3d hsbs": "hsbs", "Super movie (2009).3d.sbs": "sbs", "Super movie (2009) 3d htab": "htab",
		"Super movie (2009).hsbs": "hsbs", "Super movie (2009) 3d": "",
		// TestExpandedFormat3D: Media Browser 3 conventions.
		"Super movie.hsbs.mp4": "hsbs", "Super movie.sbs.mp4": "sbs", "Super movie.htab.mp4": "htab",
		"Super movie.tab.mp4": "tab", "Super movie.sbs3d.mp4": "sbs3d", "Super movie.3d.mvc.mp4": "mvc",
		"Super movie [3d].mp4": "", "Super movie [hsbs].mp4": "hsbs", "Super movie [fsbs].mp4": "fsbs",
		"Super movie [ftab].mp4": "ftab", "Super movie [htab].mp4": "htab", "Super movie [sbs3d].mp4": "sbs3d",
	} {
		is3D, format := parser.Parse3D(path)
		if is3D != (want != "") || format != want {
			t.Errorf("Parse3D(%q): got = %v %q, want = %q", path, is3D, format, want)
		}
	}
}

func TestFormat3DResolve(t *testing.T) {
	dir, ok := parser.ResolveVideo("/movies/Oblivion (2013) 3d hsbs", true, true, "")
	if !ok || !dir.Is3D || dir.Format3D != "hsbs" {
		t.Errorf("folder: got = %+v", dir)
	}
	file, _ := parser.ResolveVideo("C:/Users/media/Desktop/Video Test/Movies/Oblivion/Oblivion.3d.hsbs.mkv", false, true, "")
	if file.Format3D != "hsbs" || file.Name != "Oblivion" {
		t.Errorf("file: got = %+v", file)
	}
}

func TestPortedStub(t *testing.T) {
	portedCases(t, "video/stub.json", ported{facts: map[string]string{
		"TestStubs":    "TestStubs",
		"TestStubName": "TestStubs",
	}})
}

func TestStubs(t *testing.T) {
	for _, tt := range []struct {
		path     string
		isStub   bool
		stubType string
	}{
		{"video.mkv", false, ""},
		{"video.disc", true, ""},
		{"video.dvd.disc", true, "dvd"},
		{"video.hddvd.disc", true, "hddvd"},
		{"video.bluray.disc", true, "bluray"},
		{"video.brrip.disc", true, "bluray"},
		{"video.bd25.disc", true, "bluray"},
		{"video.bd50.disc", true, "bluray"},
		{"video.vhs.disc", true, "vhs"},
		{"video.hdtv.disc", true, "tv"},
		{"video.pdtv.disc", true, "tv"},
		{"video.dsr.disc", true, "tv"},
		{"", false, ""},
	} {
		stubType, ok := parser.ResolveStub(tt.path)
		if ok != tt.isStub || stubType != tt.stubType {
			t.Errorf("ResolveStub(%q): got = %q %v, want = %q %v", tt.path, stubType, ok, tt.stubType, tt.isStub)
		}
	}
	v, _ := parser.ResolveVideo("C:/Users/media/Desktop/Video Test/Movies/Oblivion/Oblivion.dvd.disc", false, true, "")
	checkString(t, "stub name", v.Name, "Oblivion")
}

func TestResolveVideoDirectory(t *testing.T) {
	for path, want := range map[string]bool{"/Server/Iron Man": true, "Batman": true, "": false} {
		v, ok := parser.ResolveVideo(path, true, true, "")
		if ok != want || v.Container != "" {
			t.Errorf("ResolveVideo(%q): got = %+v %v, want resolved = %v", path, v, ok, want)
		}
	}
}

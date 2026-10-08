package naming

import (
	"encoding/json"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func TestPortedCleanDateTime(t *testing.T) {
	portedCases(t, "video/clean_date_time.json", ported{run: map[string]func(*testing.T, args){
		"CleanDateTimeTest": func(t *testing.T, a args) {
			name, year := parser.CleanDateTime(fileName(a.str(t, "input")))
			checkFold(t, "name", name, a.str(t, "expectedName"))
			checkInt(t, "year", year, a.optInt(t, "expectedYear"))
		},
	}})
}

func TestPortedCleanString(t *testing.T) {
	portedCases(t, "video/clean_string.json", ported{run: map[string]func(*testing.T, args){
		"CleanStringTest_NeedsCleaning_Success": func(t *testing.T, a args) {
			got, ok := parser.CleanString(a.str(t, "input"))
			if !ok {
				t.Fatal("cleaned: got = false, want = true")
			}
			checkString(t, "name", got, a.str(t, "expectedName"))
		},
		"CleanStringTest_DoesntNeedCleaning_False": func(t *testing.T, a args) {
			if got, ok := parser.CleanString(a.str(t, "input")); ok || got != "" {
				t.Errorf("got = %q %v, want = not cleaned", got, ok)
			}
		},
	}})
}

func checkExtra(t *testing.T, path, libraryRoot string, want core.ExtraKind) {
	t.Helper()
	if got, _ := parser.ExtraInfo(path, libraryRoot); got != want {
		t.Errorf("ExtraInfo(%q, %q): got = %q, want = %q", path, libraryRoot, got, want)
	}
}

func TestPortedExtra(t *testing.T) {
	dirExtras := func(ext1, ext2 string) func(t *testing.T, a args) {
		return func(t *testing.T, a args) {
			kind, dir := extraKinds[a.symbol(t, "type")], a.str(t, "dirName")
			checkExtra(t, dir+"/300"+ext1, "", kind)
			checkExtra(t, "300/"+dir+"/something"+ext2, "", kind)
			checkExtra(t, "/data/something/Movies/300/"+dir+"/whoknows"+ext1, "", kind)
		}
	}
	topLevel := func(ext1, ext2 string) func(t *testing.T, a args) {
		return func(t *testing.T, a args) {
			kind, dir := extraKinds[a.symbol(t, "typicalType")], a.str(t, "dirName")
			root := "/data/something/" + dir
			checkExtra(t, root+"/300"+ext1, root, "")
			checkExtra(t, root+"/300/"+dir+"/something"+ext2, root, kind)
		}
	}
	portedCases(t, "video/extra.json", ported{
		run: map[string]func(*testing.T, args){
			"TestDirectoriesAudioExtras": dirExtras(".mp3", ".mp3"),
			"TestDirectoriesVideoExtras": dirExtras(".mp4", ".mkv"),
			"TestNonExtraDirectories": func(t *testing.T, a args) {
				dir := a.str(t, "dirName")
				checkExtra(t, dir+"/300.mp4", "", "")
				checkExtra(t, "300/"+dir+"/something.mkv", "", "")
				checkExtra(t, "/data/something/Movies/300/"+dir+"/whoknows.mp4", "", "")
				checkExtra(t, "/data/something/Movies/"+dir+"/"+dir+".mp4", "", "")
			},
			"TestTopLevelDirectoriesWithAudioExtraNames": topLevel(".mp3", ".mp3"),
			"TestTopLevelDirectoriesWithVideoExtraNames": topLevel(".mp4", ".mkv"),
		},
		facts: map[string]string{
			"TestKodiExtras":                "TestExtraFileNames",
			"TestExpandedExtras":            "TestExtraFileNames",
			"TestSample":                    "TestExtraFileNames",
			"TestSuffixPartOfTitle":         "TestExtraFileNames",
			"TestExtraInfo_InvalidRuleType": "TestExtraRegexRule",
		},
	})
}

func TestExtraFileNames(t *testing.T) {
	for path, want := range map[string]core.ExtraKind{
		// TestKodiExtras
		"trailer.mp4": core.ExtraTrailer, "300-trailer.mp4": core.ExtraTrailer, "300.trailer.mp4": core.ExtraTrailer,
		"300_trailer.mp4": core.ExtraTrailer, "300 - trailer.mp4": core.ExtraTrailer, "theme.mp3": core.ExtraThemeSong,
		// TestExpandedExtras
		"trailer2.mp4": core.ExtraTrailer, "trailer.mp3": "", "stuff trailerthings.mkv": "", "theme.mkv": "",
		"300-scene.mp4": core.ExtraScene, "300-scene2.mp4": core.ExtraScene, "300-clip.mp4": core.ExtraClip,
		"300-deleted.mp4": core.ExtraDeletedScene, "300-deletedscene.mp4": core.ExtraDeletedScene,
		"300-interview.mp4": core.ExtraInterview, "300-behindthescenes.mp4": core.ExtraBehindTheScene,
		"300-featurette.mp4": core.ExtraFeaturette, "300-short.mp4": core.ExtraShort,
		"300-extra.mp4": core.ExtraOther, "300-other.mp4": core.ExtraOther,
		// TestSample
		"sample.mp4": core.ExtraSample, "300-sample.mp4": core.ExtraSample, "300.sample.mp4": core.ExtraSample,
		"300_sample.mp4": core.ExtraSample, "300 - sample.mp4": core.ExtraSample, "sample1.mp4": core.ExtraSample,
		"Sample2.mkv": core.ExtraSample,
		// TestSuffixPartOfTitle
		"I Live In A Trailer.mp4": "", "The DNA Sample.mp4": "",
	} {
		checkExtra(t, path, "", want)
	}
}

func TestExtraRegexRule(t *testing.T) {
	o := DefaultOptions()
	o.VideoExtraRules = []ExtraRule{{Kind: core.ExtraOther, Type: ExtraRegex, Token: `([eE]x(tra)?\.\w+)`, Media: MediaVideo}}
	p, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	if _, rule := p.ExtraInfo("extra.mp4", ""); rule == nil || *rule != o.VideoExtraRules[0] {
		t.Errorf("rule: got = %v, want = %v", rule, o.VideoExtraRules[0])
	}
}

func TestPortedVideoResolver(t *testing.T) {
	portedCases(t, "video/video_resolver.json", ported{
		run: map[string]func(*testing.T, args){
			"ResolveFile_ValidFileName_Success": func(t *testing.T, a args) {
				want, _ := a.object(t, "expectedResult")
				path := want.str(t, "path")
				got, ok := parser.ResolveVideo(path, false, true, "")
				if !ok {
					t.Fatal("got = not resolved")
				}
				checkString(t, "container", got.Container, want.str(t, "container"))
				checkString(t, "name", got.Name, want.str(t, "name"))
				checkInt(t, "year", got.Year, want.optInt(t, "year"))
				if kind := extraKinds[symbolOf(t, want, "extraType")]; got.Extra != kind {
					t.Errorf("extra: got = %q, want = %q", got.Extra, kind)
				}
				checkString(t, "3D format", got.Format3D, want.str(t, "format3D"))
				if got.Is3D != want.boolean(t, "is3D") || got.IsStub != want.boolean(t, "isStub") || got.IsDir {
					t.Errorf("flags: got = %+v", got)
				}
				checkString(t, "stub type", got.StubType, want.str(t, "stubType"))
			},
		},
		facts: map[string]string{
			"ResolveFile_EmptyPath": "TestResolveVideoEmptyPath",
			"ResolveDirectoryTest":  "TestResolveVideoDirectory",
		},
	})
}

// symbolOf returns an optional enum argument of a constructor call.
func symbolOf(t *testing.T, a args, name string) string {
	t.Helper()
	if _, ok := a[name]; !ok {
		return ""
	}
	return a.symbol(t, name)
}

func TestResolveVideoEmptyPath(t *testing.T) {
	if _, ok := parser.ResolveVideo("", false, true, ""); ok {
		t.Error("got = resolved, want = not resolved")
	}
}

func TestPortedMultiDiscAlbum(t *testing.T) {
	portedCases(t, "music/multi_disc_album.json", ported{run: map[string]func(*testing.T, args){
		"AlbumParser_MultidiscPath_Identifies": func(t *testing.T, a args) {
			path := a.str(t, "path")
			if got, want := parser.IsMultiPartAlbum(path), a.boolean(t, "result"); got != want {
				t.Errorf("IsMultiPartAlbum(%q): got = %v, want = %v", path, got, want)
			}
		},
	}})
}

func TestPortedAudioBookResolver(t *testing.T) {
	portedCases(t, "audio_book/audio_book_resolver.json", ported{
		run: map[string]func(*testing.T, args){
			"Resolve_ValidFileName_Success": func(t *testing.T, a args) {
				named, positional := a.object(t, "expectedResult")
				var path, container string
				_ = json.Unmarshal(positional[0], &path)
				_ = json.Unmarshal(positional[1], &container)
				got, ok := parser.ResolveAudioBookFile(path)
				if !ok {
					t.Fatal("got = not resolved")
				}
				checkString(t, "container", got.Container, container)
				checkInt(t, "chapter", got.ChapterNumber, named.optInt(t, "chapterNumber"))
				checkInt(t, "part", got.PartNumber, named.optInt(t, "partNumber"))
			},
		},
		facts: map[string]string{
			"Resolve_InvalidExtension": "TestResolveAudioBookFileInvalid",
			"Resolve_EmptyFileName":    "TestResolveAudioBookFileInvalid",
		},
	})
}

func TestResolveAudioBookFileInvalid(t *testing.T) {
	for _, path := range []string{"/server/AudioBooks/Larry Potter/Larry Potter.mp9", ""} {
		if _, ok := parser.ResolveAudioBookFile(path); ok {
			t.Errorf("%q: got = resolved, want = not resolved", path)
		}
	}
}

func TestPortedBookResolver(t *testing.T) {
	portedCases(t, "book/book_resolver.json", ported{run: map[string]func(*testing.T, args){
		"Resolve_Books": func(t *testing.T, a args) {
			r := ParseBookFileName(a.str(t, "input"))
			checkString(t, "name", r.Name, a.str(t, "name"))
			checkString(t, "series", r.SeriesName, a.str(t, "series"))
			checkInt(t, "index", r.Index, a.optInt(t, "index"))
			checkInt(t, "year", r.Year, a.optInt(t, "year"))
		},
		"Resolve_Comics": func(t *testing.T, a args) {
			r := ParseBookFileName(a.str(t, "input"))
			checkString(t, "name", r.Name, a.str(t, "name"))
			checkString(t, "series", r.SeriesName, a.str(t, "series"))
			checkInt(t, "chapter", r.Index, a.optInt(t, "chapter"))
			checkInt(t, "volume", r.ParentIndex, a.optInt(t, "volume"))
			checkInt(t, "year", r.Year, a.optInt(t, "year"))
		},
	}})
}

// testLanguages mirrors the localization mock of Jellyfin's tests: tokens
// starting with "en", "fr" or "hi" are English, French or Hindi.
type testLanguages struct{}

func (testLanguages) FindLanguage(token string) (Language, bool) {
	for prefix, l := range map[string]Language{
		"en": {Name: "en", ThreeLetter: "eng"},
		"fr": {Name: "fr", ThreeLetter: "fre"},
		"hi": {Name: "hi", ThreeLetter: "hin"},
	} {
		if containsStringFold(token, prefix) {
			return l, true
		}
	}
	return Language{}, false
}

func TestPortedExternalPathParser(t *testing.T) {
	notMatched := func(kind ExternalKind) func(t *testing.T, a args) {
		return func(t *testing.T, a args) {
			if got, ok := parser.ParseExternalFile(kind, testLanguages{}, a.str(t, "path"), ""); ok {
				t.Errorf("got = %+v, want = not matched", got)
			}
		}
	}
	matched := func(kind ExternalKind) func(t *testing.T, a args) {
		return func(t *testing.T, a args) {
			path := a.str(t, "path")
			got, ok := parser.ParseExternalFile(kind, testLanguages{}, path, "")
			if !ok {
				t.Fatal("got = not matched")
			}
			checkString(t, "path", got.Path, path)
		}
	}
	portedCases(t, "external_files/external_path_parser.json", ported{run: map[string]func(*testing.T, args){
		"ParseFile_AudioExtensionsNotMatched_ReturnsNull":    notMatched(ExternalAudio),
		"ParseFile_AudioExtensionsMatched_ReturnsPath":       matched(ExternalAudio),
		"ParseFile_SubtitleExtensionsNotMatched_ReturnsNull": notMatched(ExternalSubtitle),
		"ParseFile_SubtitleExtensionsMatched_ReturnsPath":    matched(ExternalSubtitle),
		"ParseFile_ExtraTokens_ParseToValues": func(t *testing.T, a args) {
			tokens := a.str(t, "tokens")
			got, ok := parser.ParseExternalFile(ExternalSubtitle, testLanguages{}, "My.Video"+tokens+".srt", tokens)
			if !ok {
				t.Fatal("got = not matched")
			}
			checkString(t, "title", got.Title, a.str(t, "title"))
			checkString(t, "language", got.Language, a.str(t, "language"))
			if got.IsDefault != a.boolean(t, "isDefault") || got.IsForced != a.boolean(t, "isForced") ||
				got.IsHearingImpaired != a.boolean(t, "isHearingImpaired") {
				t.Errorf("flags: got = %+v", got)
			}
		},
	}})
}

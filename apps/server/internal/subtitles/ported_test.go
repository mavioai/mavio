package subtitles

import (
	"encoding/json"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// Test cases ported from Bazarr by tools/testport live in testdata/cases;
// see docs/testing.md §2.

type caseFile struct {
	Theories []struct {
		Method string `json:"method"`
		Cases  []struct {
			ID   string                     `json:"id"`
			Skip string                     `json:"skip"`
			Args map[string]json.RawMessage `json:"args"`
		} `json:"cases"`
	} `json:"theories"`
	Facts []struct {
		Method string `json:"method"`
	} `json:"facts"`
}

type ported struct {
	run   map[string]func(t *testing.T, args map[string]json.RawMessage)
	skip  map[string]string
	facts map[string]string // fact → Go test porting it
}

func portedCases(t *testing.T, name string, p ported) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "cases", name))
	if err != nil {
		t.Fatal(err)
	}
	var f caseFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	for _, th := range f.Theories {
		check, ok := p.run[th.Method]
		reason, skipped := p.skip[th.Method]
		if !ok && !skipped {
			t.Errorf("%s: theory %s is neither run nor skipped", name, th.Method)
			continue
		}
		for _, c := range th.Cases {
			t.Run(c.ID, func(t *testing.T) {
				switch {
				case c.Skip != "":
					t.Skip(c.Skip)
				case skipped:
					t.Skip(reason)
				}
				check(t, c.Args)
			})
		}
	}
	for _, m := range f.Facts {
		if p.facts[m.Method] == "" && p.skip[m.Method] == "" {
			t.Errorf("%s: %s is neither ported nor skipped", name, m.Method)
		}
	}
}

const (
	itemIDs     = "every Mavio movie and episode has an ID, so none is left out of the blocklist"
	oneByOne    = "Mavio downloads and saves subtitles one at a time, so the unusable one is always known"
	structured  = "Mavio records languages as core.SubtitleLanguage, not as suffixed strings"
	oneTable    = "Mavio keeps one subtitle blocklist for movies and episodes, keyed by item"
	notExtract  = "embedded subtitles are not extracted to be downloaded; they count as present from the scan's streams"
	scanStreams = "the streams come from the library scan's ffprobe, whose parsing libs/media tests"
)

func TestPortedDownloadBlocklist(t *testing.T) {
	portedCases(t, "download_blacklist_on_failure.json", ported{
		run: map[string]func(*testing.T, map[string]json.RawMessage){
			// Plugins report subtitles they cannot decode as bad
			// subtitles; the server blocklists them.
			"test_undecodable_subtitle_is_blacklisted": func(t *testing.T, _ map[string]json.RawMessage) {
				e := newEnv(t)
				e.plugin.found = []*pluginv1.RemoteSubtitle{candidate("broken", "fr", release, "title", "year")}
				e.plugin.bad["broken"] = true
				if _, err := e.rt.SearchItem(e.ctx, e.movie.ID); err != nil {
					t.Fatal(err)
				}
				page, err := e.db.SubtitleBlocklist().List(e.ctx, e.movie.ID, 0, 0)
				if err != nil || len(page.Items) != 1 {
					t.Errorf("blocklist = %+v, %v, want the broken subtitle", page.Items, err)
				}
			},
			// Failing to save is the server's failure, not the
			// subtitle's.
			"test_environmental_failure_is_not_blacklisted": func(t *testing.T, _ map[string]json.RawMessage) {
				e := newEnv(t)
				e.plugin.found = []*pluginv1.RemoteSubtitle{candidate("fine", "fr", release, "title", "year")}
				// The subtitle folder is a file, which fails on every
				// platform; a read-only folder does not on Windows.
				e.rt.Subtitles.Folder = func() library.SubtitleFolder {
					return library.SubtitleFolder{Kind: core.SubtitleFolderRelative, Relative: "Subs"}
				}
				if err := os.WriteFile(path.Join(e.dir, "Subs"), nil, 0o644); err != nil {
					t.Fatal(err)
				}
				if _, err := e.rt.SearchItem(e.ctx, e.movie.ID); err == nil {
					t.Error("SearchItem into a folder that cannot be made: no error")
				}
				page, err := e.db.SubtitleBlocklist().List(e.ctx, e.movie.ID, 0, 0)
				if err != nil || len(page.Items) != 0 {
					t.Errorf("blocklist = %+v, %v, want none", page.Items, err)
				}
			},
		},
		facts: map[string]string{
			"test_episode_subtitle_is_blacklisted":         "TestSearchItemBadSubtitle",
			"test_movie_subtitle_is_blacklisted":           "TestSearchItemBadSubtitle",
			"test_forced_language_is_recorded":             "TestSearchItemBadSubtitle",
			"test_only_the_tagged_subtitle_is_blacklisted": "TestSearchItemBadSubtitle",
		},
		skip: map[string]string{
			"test_every_subtitle_in_the_batch_is_blacklisted": oneByOne,
			"test_untagged_falls_back_to_the_whole_batch":     oneByOne,
			"test_save_subtitles_tags_the_failing_subtitle":   oneByOne,
			"test_no_id_video_is_not_blacklisted":             itemIDs,
			"test_no_id_video_is_logged":                      itemIDs,
		},
	})
}

func TestPortedProvidersBlocklist(t *testing.T) {
	portedCases(t, "get_providers_blacklist.json", ported{
		facts: map[string]string{
			"test_series_is_blacklisted": "TestManual",
			"test_movie_is_blacklisted":  "TestManual",
		},
		skip: map[string]string{
			"test_series_without_known_ids_is_not_blacklisted": itemIDs,
			"test_movie_without_known_id_is_not_blacklisted":   itemIDs,
			"test_no_ids_at_all_is_not_blacklisted":            itemIDs,
			"test_skipped_series_blacklisting_is_logged":       itemIDs,
			"test_skipped_movie_blacklisting_is_logged":        itemIDs,
			"test_successful_blacklisting_logs_no_skip":        itemIDs,
			"test_series_does_not_touch_the_movie_blacklist":   oneTable,
			"test_plain_language_is_recorded_bare":             structured,
			"test_forced_language_is_suffixed":                 structured,
			"test_hi_language_is_suffixed":                     structured,
		},
	})
}

func TestPortedManual(t *testing.T) {
	portedCases(t, "subtitles_manual.json", ported{facts: map[string]string{
		"test_build_release_info_plain":                      "TestReleaseNames",
		"test_build_release_info_multiple_comma_separated":   "TestReleaseNames",
		"test_build_release_info_with_extra_release_info":    "TestReleaseNames",
		"test_build_release_info_without_extra_release_info": "TestReleaseNames",
		"test_build_release_info_none":                       "TestReleaseNames",
	}})
}

func TestPortedPool(t *testing.T) {
	const pool = "the provider pool runs in plugins/subtitles, whose tests port this"
	portedCases(t, "subtitles_pool.json", ported{skip: map[string]string{"test_init_pool": pool, "test_pool_update": pool}})
}

func TestPortedSync(t *testing.T) {
	const settings = "Mavio reads the settings anew on every synchronization and passes them typed, with no options left to preserve"
	portedCases(t, "subtitles_sync.json", ported{skip: map[string]string{
		"test_sync_subtitles_reads_default_settings_at_runtime": settings,
		"test_sync_subtitles_preserves_explicit_sync_options":   settings,
	}})
}

func TestPortedVideoAnalyzer(t *testing.T) {
	portedCases(t, "utilities_video_analyzer.json", ported{
		facts: map[string]string{"test_handle_alpha3_accepts_string_language": "TestTag"},
		skip: map[string]string{
			"test_embedded_subs_reader":                          scanStreams,
			"test_embedded_audio_reader":                         scanStreams,
			"test_embedded_audio_reader_accepts_string_language": scanStreams,
		},
	})
}

func TestPortedEmbedded(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "cases", "embeddedsubtitles.json"))
	if err != nil {
		t.Fatal(err)
	}
	var f caseFile
	if err := json.Unmarshal(data, &f); err != nil {
		t.Fatal(err)
	}
	skip := map[string]string{}
	for _, th := range f.Theories {
		skip[th.Method] = notExtract
	}
	for _, m := range f.Facts {
		skip[m.Method] = notExtract
	}
	portedCases(t, "embeddedsubtitles.json", ported{skip: skip})
}

func TestPortedWhisper(t *testing.T) {
	const ffprobe = "the server encodes audio for recognizers with its own ffmpeg; recognizers take PCM, never an ffprobe path"
	portedCases(t, "whisperai.json", ported{skip: map[string]string{
		"test_ffprobe_path_uses_provided_value":          ffprobe,
		"test_ffprobe_path_falls_back_when_not_provided": ffprobe,
	}})
}

func TestReleaseNames(t *testing.T) {
	for _, c := range []struct {
		in, want []string
	}{
		{[]string{"Movie.en.srt"}, []string{"Movie.en.srt"}},
		{[]string{"Movie.en.srt,Some.Other.Release"}, []string{"Movie.en.srt", "Some.Other.Release"}},
		// Plugins send the extra release information as further names.
		{[]string{"Movie.en.srt", "Duration: 1:42:18"}, []string{"Movie.en.srt", "Duration: 1:42:18"}},
		{nil, nil},
	} {
		if got := ReleaseNames(c.in); !slices.Equal(got, c.want) {
			t.Errorf("ReleaseNames(%q) = %q, want = %q", c.in, got, c.want)
		}
	}
}

func TestTag(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"jpn", "ja"}, {"eng", "en"}, {"fre", "fr"}, {"por", "pt"}, {"zh-Hant", "zh-Hant"}, {"", ""}, {"und", ""},
	} {
		if got := Tag(c.in); got != c.want {
			t.Errorf("Tag(%q) = %q, want = %q", c.in, got, c.want)
		}
	}
}

package subtitles

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/mavioai/mavio/libs/core"
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
		Skip   string `json:"skip"`
	} `json:"facts"`
	Unsupported []struct {
		Method string `json:"method"`
	} `json:"unsupported"`
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
	for _, m := range f.Unsupported {
		if p.facts[m.Method] == "" && p.skip[m.Method] == "" {
			t.Errorf("%s: %s is neither ported nor skipped", name, m.Method)
		}
	}
}

// bazarrLanguage matches the core.Language("spa", "MX") expressions of
// Bazarr's tests.
var bazarrLanguage = regexp.MustCompile(`^core\.Language\("(\w+)"(?:, "(\w+)")?\)$`)

// languageOf converts a Bazarr language expression to a subtitle language.
func languageOf(t *testing.T, raw json.RawMessage) core.SubtitleLanguage {
	t.Helper()
	var v struct {
		Expr string `json:"$expr"`
	}
	if err := json.Unmarshal(raw, &v); err != nil {
		t.Fatal(err)
	}
	m := bazarrLanguage.FindStringSubmatch(v.Expr)
	if m == nil {
		t.Fatalf("language %q: not supported", v.Expr)
	}
	alpha2 := map[string]string{"spa": "es", "por": "pt", "zho": "zh", "eng": "en"}[m[1]]
	if alpha2 == "" {
		t.Fatalf("language %q: not supported", v.Expr)
	}
	if m[2] != "" {
		alpha2 += "-" + m[2]
	}
	return core.SubtitleLanguage{Language: alpha2}
}

const (
	poolInPlugin    = "the provider pool runs in plugins/subtitles, whose tests port this"
	settingsTyped   = "Mavio's settings hold language equivalences as structured languages, not Bazarr's \"from:to\" strings"
	providersPlugin = "provider configuration and throttling live in plugins/subtitles, whose tests port this"
)

func TestPortedScore(t *testing.T) {
	portedCases(t, "score.json", ported{
		facts: map[string]string{
			"test_compute_score_set_var":           "TestScoreUnverifiableHash",
			"test_compute_score_set_var_w_episode": "TestScoreUnverifiableHash",
			"test_compute_score_defaults":          "TestDefaultScores",
		},
		skip: map[string]string{
			"test_compute_score_custom_invalid": "Mavio scores only with Bazarr's default weights",
			"test_compute_score_custom_valid":   "Mavio scores only with Bazarr's default weights",
		},
	})
}

func TestPortedCore(t *testing.T) {
	portedCases(t, "core.json", ported{
		run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_language_equals_check_set": func(t *testing.T, args map[string]json.RawMessage) {
				var pairs [][]json.RawMessage
				if err := json.Unmarshal(args["langs"], &pairs); err != nil {
					t.Fatal(err)
				}
				var eq LanguageEquals
				var want []core.SubtitleLanguage
				for _, p := range pairs {
					from, to := languageOf(t, p[0]), languageOf(t, p[1])
					eq = append(eq, core.SubtitleLanguageEquivalence{From: from, To: to})
					want = append(want, from, to)
				}
				got := eq.CheckSet([]core.SubtitleLanguage{eq[0].From})
				if !sameLanguages(got, want) {
					t.Errorf("CheckSet: got = %v, want = %v", got, want)
				}
			},
		},
		facts: map[string]string{
			"test_scan_video_movie":                                              "TestParseGuess",
			"test_scan_video_episode":                                            "TestParseGuess",
			"test_language_equals_init":                                          "TestLanguageEquals",
			"test_language_equals_init_empty_list_gracefully":                    "TestLanguageEquals",
			"test_language_equals_check_set_do_nothing":                          "TestLanguageEquals",
			"test_language_equals_check_set_do_nothing_w_forced":                 "TestLanguageEquals",
			"test_language_equals_pool_intance_list_subtitles":                   "TestFilterLanguageEquals",
			"test_language_equals_pool_intance_list_subtitles_reversed":          "TestFilterLanguageEquals",
			"test_language_equals_pool_intance_list_subtitles_empty_lang_equals": "TestFilterLanguageEquals",
			"test_language_equals_pool_intance_list_subtitles_return_nothing":    "TestFilterLanguageEquals",
		},
		skip: map[string]string{
			"test_pool_update_w_nothing":             poolInPlugin,
			"test_pool_update_w_multiple_providers":  poolInPlugin,
			"test_pool_update_discarded_providers":   poolInPlugin,
			"test_pool_update_discarded_providers_2": poolInPlugin,
			"test_language_equals_init_invalid":      "an equivalence is a typed pair, so it cannot have one language",
		},
	})
}

func TestPortedAppGetProviders(t *testing.T) {
	portedCases(t, "app_get_providers.json", ported{
		skip: map[string]string{
			"test_get_language_equals_injected_settings_custom_lang_alpha3": settingsTyped,
			"test_get_language_equals_default_settings":                     settingsTyped,
			"test_get_language_equals_injected_settings_invalid":            settingsTyped,
			"test_get_language_equals_injected_settings_valid":              settingsTyped,
			"test_get_language_equals_injected_settings_multiple":           settingsTyped,
			"test_get_language_equals_injected_settings_valid_multiple":     settingsTyped,
			"test_get_language_equals_injected_settings_hi":                 settingsTyped,
			"test_get_providers_auth":                                       providersPlugin,
			"test_get_providers_auth_with_provider_registry":                providersPlugin,
			"test_get_providers_auth_embeddedsubtitles":                     providersPlugin,
			"test_get_providers_auth_karagarga":                             providersPlugin,
			"test_throttled_count_grants_new_retries_after_throttling":      providersPlugin,
			"test_throttled_count_ignores_failures_older_than_the_window":   providersPlugin,
			"test_throttled_count_tolerates_a_concurrent_removal":           providersPlugin,
			"test_reset_throttled_providers_also_resets_the_count":          providersPlugin,
			"test_get_traceback_info":                                       "formats Python tracebacks for Bazarr's logs",
		},
	})
}

func sameLanguages(a, b []core.SubtitleLanguage) bool {
	key := func(x, y core.SubtitleLanguage) int {
		if x.String() < y.String() {
			return -1
		} else if x.String() > y.String() {
			return 1
		}
		return 0
	}
	a, b = slices.Clone(a), slices.Clone(b)
	slices.SortFunc(a, key)
	slices.SortFunc(b, key)
	return slices.Equal(a, b)
}

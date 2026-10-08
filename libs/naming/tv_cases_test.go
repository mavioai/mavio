package naming

import (
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

func checkInt(t *testing.T, what string, got, want *int) {
	t.Helper()
	if !eqInt(got, want) {
		t.Errorf("%s: got = %v, want = %v", what, fmtInt(got), fmtInt(want))
	}
}

func checkFold(t *testing.T, what, got, want string) {
	t.Helper()
	if !strings.EqualFold(got, want) {
		t.Errorf("%s: got = %q, want = %q", what, got, want)
	}
}

func checkString(t *testing.T, what, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: got = %q, want = %q", what, got, want)
	}
}

func resolve(t *testing.T, path string, o EpisodeOptions) *Episode {
	t.Helper()
	if ep, ok := parser.ResolveEpisode(path, false, o); ok {
		return &ep
	}
	return nil
}

// episodeField returns a field of an optional episode, nil when absent.
func episodeField(ep *Episode, f func(*Episode) *int) *int {
	if ep == nil {
		return nil
	}
	return f(ep)
}

func TestPortedAbsoluteEpisodeNumber(t *testing.T) {
	portedCases(t, "tv/absolute_episode_number.json", ported{run: map[string]func(*testing.T, args){
		"GetEpisodeNumberFromFileTest": func(t *testing.T, a args) {
			ep := resolve(t, a.str(t, "path"), EpisodeOptions{SupportsAbsoluteNumbers: ptr(true)})
			want := a.integer(t, "episodeNumber")
			checkInt(t, "episode", episodeField(ep, func(e *Episode) *int { return e.EpisodeNumber }), &want)
		},
	}})
}

func TestPortedDailyEpisode(t *testing.T) {
	portedCases(t, "tv/daily_episode.json", ported{run: map[string]func(*testing.T, args){
		"Test": func(t *testing.T, a args) {
			ep := resolve(t, a.str(t, "path"), EpisodeOptions{})
			field := func(f func(*Episode) *int) *int { return episodeField(ep, f) }
			checkInt(t, "season", field(func(e *Episode) *int { return e.SeasonNumber }), nil)
			checkInt(t, "episode", field(func(e *Episode) *int { return e.EpisodeNumber }), nil)
			checkInt(t, "year", field(func(e *Episode) *int { return e.Year }), a.optInt(t, "year"))
			checkInt(t, "month", field(func(e *Episode) *int { return e.Month }), a.optInt(t, "month"))
			checkInt(t, "day", field(func(e *Episode) *int { return e.Day }), a.optInt(t, "day"))
			series := ""
			if ep != nil {
				series = ep.SeriesName
			}
			checkFold(t, "series", series, a.str(t, "seriesName"))
		},
	}})
}

func TestPortedEpisodeNumber(t *testing.T) {
	portedCases(t, "tv/episode_number.json", ported{run: map[string]func(*testing.T, args){
		"GetEpisodeNumberFromFileTest": func(t *testing.T, a args) {
			r := parser.ParseEpisodePath(a.str(t, "path"), false, EpisodeOptions{})
			checkInt(t, "episode", r.EpisodeNumber, a.optInt(t, "expected"))
		},
	}})
}

func TestPortedEpisodeNumberWithoutSeason(t *testing.T) {
	portedCases(t, "tv/episode_number_without_season.json", ported{run: map[string]func(*testing.T, args){
		"GetEpisodeNumberFromFileTest": func(t *testing.T, a args) {
			ep := resolve(t, a.str(t, "path"), EpisodeOptions{})
			want := a.integer(t, "episodeNumber")
			checkInt(t, "episode", episodeField(ep, func(e *Episode) *int { return e.EpisodeNumber }), &want)
		},
	}})
}

func TestPortedEpisodePathParser(t *testing.T) {
	portedCases(t, "tv/episode_path_parser.json", ported{
		run: map[string]func(*testing.T, args){
			"ParseEpisodesCorrectly": func(t *testing.T, a args) {
				r := parser.ParseEpisodePath(a.str(t, "path"), a.boolean(t, "isDirectory"), EpisodeOptions{})
				if !r.Success {
					t.Fatal("success: got = false, want = true")
				}
				checkString(t, "series", r.SeriesName, a.str(t, "name"))
				season, episode := a.integer(t, "season"), a.integer(t, "episode")
				checkInt(t, "season", r.SeasonNumber, &season)
				checkInt(t, "episode", r.EpisodeNumber, &episode)
			},
			"EpisodePathParserTest_DifferentExpressionsParameters": func(t *testing.T, a args) {
				r := parser.ParseEpisodePath(a.str(t, "path"), false, EpisodeOptions{Named: a.optBool(t, "isNamed"), Optimistic: a.optBool(t, "isOptimistic")})
				if !r.Success {
					t.Error("success: got = false, want = true")
				}
			},
		},
		facts: map[string]string{
			"EpisodePathParserTest_FalsePositivePixelRate": "TestEpisodePathFalsePositivePixelRate",
			"EpisodeResolverTest_WrongExtension":           "TestEpisodeResolverExtensions",
			"EpisodeResolverTest_WrongExtensionStub":       "TestEpisodeResolverExtensions",
			"EpisodePathParserTest_EmptyDateParsers":       "TestEpisodePathEmptyDateFormats",
		},
	})
}

func TestEpisodePathFalsePositivePixelRate(t *testing.T) {
	if r := parser.ParseEpisodePath("Series Special (1920x1080).mkv", false, EpisodeOptions{}); r.Success {
		t.Errorf("success: got = true, want = false (%+v)", r)
	}
}

func TestEpisodeResolverExtensions(t *testing.T) {
	if _, ok := parser.ResolveEpisode("test.mp3", false, EpisodeOptions{}); ok {
		t.Error("test.mp3: got = resolved, want = not resolved")
	}
	ep, ok := parser.ResolveEpisode("dvd.disc", false, EpisodeOptions{})
	if !ok || !ep.IsStub {
		t.Errorf("dvd.disc: got = %+v %v, want = stub", ep, ok)
	}
}

// TestEpisodePathEmptyDateFormats covers a date expression without formats.
func TestEpisodePathEmptyDateFormats(t *testing.T) {
	o := DefaultOptions()
	o.EpisodeExpressions = []EpisodeExpression{{Expression: "(([0-9]{4})-([0-9]{2})-([0-9]{2}) [0-9]{2}:[0-9]{2}:[0-9]{2})", ByDate: true}}
	p, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	if r := p.ParseEpisodePath("ABC_2019_10_21 11:00:00", false, EpisodeOptions{}); !r.Success {
		t.Error("success: got = false, want = true")
	}
}

func TestPortedMultiEpisode(t *testing.T) {
	portedCases(t, "tv/multi_episode.json", ported{run: map[string]func(*testing.T, args){
		"TestGetEndingEpisodeNumberFromFile": func(t *testing.T, a args) {
			r := parser.ParseEpisodePath(a.str(t, "filename"), false, EpisodeOptions{})
			checkInt(t, "ending episode", r.EndingEpisodeNumber, a.optInt(t, "endingEpisodeNumber"))
		},
	}})
}

func TestPortedSeasonNumber(t *testing.T) {
	portedCases(t, "tv/season_number.json", ported{run: map[string]func(*testing.T, args){
		"GetSeasonNumberFromEpisodeFileTest": func(t *testing.T, a args) {
			ep := resolve(t, a.str(t, "path"), EpisodeOptions{})
			checkInt(t, "season", episodeField(ep, func(e *Episode) *int { return e.SeasonNumber }), a.optInt(t, "expected"))
		},
	}})
}

func TestPortedSeasonPathParser(t *testing.T) {
	check := func(aliases bool) func(t *testing.T, a args) {
		return func(t *testing.T, a args) {
			r := ParseSeasonPath(a.str(t, "path"), a.str(t, "parentPath"), aliases, aliases)
			if r.Success != (r.SeasonNumber != nil) {
				t.Errorf("success: got = %v with season %v", r.Success, fmtInt(r.SeasonNumber))
			}
			checkInt(t, "season", r.SeasonNumber, a.optInt(t, "seasonNumber"))
			if want := a.boolean(t, "isSeasonDirectory"); r.IsSeasonFolder != want {
				t.Errorf("season folder: got = %v, want = %v", r.IsSeasonFolder, want)
			}
		}
	}
	portedCases(t, "tv/season_path_parser.json", ported{run: map[string]func(*testing.T, args){
		"GetSeasonNumberFromPathTest":             check(true),
		"GetSeasonNumberFromPathMixedLibraryTest": check(false),
	}})
}

func TestPortedSeriesPathParser(t *testing.T) {
	portedCases(t, "tv/series_path_parser.json", ported{run: map[string]func(*testing.T, args){
		"SeriesPathParserParseTest": func(t *testing.T, a args) {
			name, ok := parser.ParseSeriesPath(a.str(t, "path"))
			if !ok {
				t.Error("success: got = false, want = true")
			}
			checkString(t, "series", name, a.str(t, "name"))
		},
		"SeriesPathParser_ResolutionPatternIsNotASeries": func(t *testing.T, a args) {
			if name, ok := parser.ParseSeriesPath(a.str(t, "path")); ok {
				t.Errorf("success: got = true (%q), want = false", name)
			}
		},
	}})
}

func TestPortedSeriesResolver(t *testing.T) {
	portedCases(t, "tv/series_resolver.json", ported{run: map[string]func(*testing.T, args){
		"SeriesResolverResolveTest": func(t *testing.T, a args) {
			checkString(t, "name", parser.ResolveSeries(a.str(t, "path")).Name, a.str(t, "name"))
		},
	}})
}

func TestPortedSimpleEpisode(t *testing.T) {
	portedCases(t, "tv/simple_episode.json", ported{run: map[string]func(*testing.T, args){
		"TestSimple": func(t *testing.T, a args) {
			path := a.str(t, "path")
			ep := resolve(t, path, EpisodeOptions{})
			if ep == nil {
				t.Fatal("got = not resolved")
			}
			checkInt(t, "season", ep.SeasonNumber, a.optInt(t, "seasonNumber"))
			checkInt(t, "episode", ep.EpisodeNumber, a.optInt(t, "episodeNumber"))
			checkFold(t, "series", ep.SeriesName, a.str(t, "seriesName"))
			checkString(t, "path", ep.Path, path)
			checkString(t, "container", ep.Container, extension(path)[1:])
			if ep.Is3D || ep.Format3D != "" || ep.IsStub || ep.StubType != "" || ep.IsByDate {
				t.Errorf("flags: got = %+v", ep)
			}
			checkInt(t, "ending episode", ep.EndingEpisodeNumber, a.optInt(t, "episodeEndNumber"))
		},
	}})
}

func TestPortedTvParserHelpers(t *testing.T) {
	statuses := map[string]core.SeriesStatus{"Continuing": core.SeriesContinuing, "Ended": core.SeriesEnded, "Unreleased": core.SeriesUnreleased}
	portedCases(t, "tv/tv_parser_helpers.json", ported{run: map[string]func(*testing.T, args){
		"SeriesStatusParserTest_Valid": func(t *testing.T, a args) {
			got, ok := ParseSeriesStatus(a.str(t, "statusString"))
			if want := statuses[a.symbol(t, "status")]; !ok || got != want {
				t.Errorf("got = %q %v, want = %q", got, ok, want)
			}
		},
		"SeriesStatusParserTest_InValid": func(t *testing.T, a args) {
			if got, ok := ParseSeriesStatus(a.str(t, "statusString")); ok {
				t.Errorf("got = %q, want = no status", got)
			}
		},
	}})
}

package metadata

import "testing"

func TestPortedProviderIDParser(t *testing.T) {
	valid := func(find func(string) (string, bool)) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			if got, ok := find(a.str(t, "text")); !ok || got != a.str(t, "expected") {
				t.Errorf("got = %q %v, want = %q", got, ok, a.str(t, "expected"))
			}
		}
	}
	invalid := func(find func(string) (string, bool)) func(*testing.T, args) {
		return func(t *testing.T, a args) {
			if got, ok := find(a.str(t, "text")); ok {
				t.Errorf("got = %q, want = not found", got)
			}
		}
	}
	portedCases(t, "nfo/provider_id_parser.json", ported{run: map[string]func(*testing.T, args){
		"FindImdbId_Valid_Success":         valid(FindIMDbID),
		"FindImdbId_Invalid_Success":       invalid(FindIMDbID),
		"FindTmdbMovieId_Valid_Success":    valid(FindTMDBMovieID),
		"FindTmdbMovieId_Invalid_Success":  invalid(FindTMDBMovieID),
		"FindTmdbSeriesId_Valid_Success":   valid(FindTMDBSeriesID),
		"FindTmdbSeriesId_Invalid_Success": invalid(FindTMDBSeriesID),
		"FindTvdbId_Valid_Success":         valid(FindTVDBID),
		"FindTvdbId_Invalid_Success":       invalid(FindTVDBID),
	}})
}

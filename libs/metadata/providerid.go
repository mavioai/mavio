package metadata

import "strings"

// FindIMDbID finds an IMDb ID ("tt" followed by 7 or 8 digits) in text,
// such as a URL.
func FindIMDbID(text string) (string, bool) {
	for len(text) >= 2+7 {
		i := strings.Index(text, "tt")
		if i < 0 {
			return "", false
		}
		text = text[i:]
		n := 2
		for n < min(len(text), 2+8) && isDigit(text[n]) {
			n++
		}
		if n >= 2+7 {
			return text[:n], true
		}
		text = text[n:]
	}
	return "", false
}

// FindTMDBMovieID finds the ID in a themoviedb.org movie URL.
func FindTMDBMovieID(text string) (string, bool) { return findID(text, "themoviedb.org/movie/") }

// FindTMDBSeriesID finds the ID in a themoviedb.org TV URL.
func FindTMDBSeriesID(text string) (string, bool) { return findID(text, "themoviedb.org/tv/") }

// FindTVDBID finds the ID in a thetvdb.com series URL.
func FindTVDBID(text string) (string, bool) { return findID(text, "thetvdb.com/?tab=series&id=") }

func findID(text, prefix string) (string, bool) {
	i := strings.Index(text, prefix)
	if i < 0 {
		return "", false
	}
	text = text[i+len(prefix):]
	n := 0
	for n < len(text) && isDigit(text[n]) {
		n++
	}
	return text[:n], n > 0
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

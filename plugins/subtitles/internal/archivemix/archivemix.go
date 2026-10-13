// Package archivemix picks subtitles from archives as Bazarr's
// ProviderSubtitleArchiveMixin, and reads the season and episodes of
// release and file names as the guessit properties Bazarr's providers use.
package archivemix

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mavioai/mavio/plugins/subtitles/internal/archive"
)

// Guess is what a name tells of its episode.
type Guess struct {
	// Season is 0 when the name has none.
	Season int
	// Episodes are the name's episodes, a range expanded; none when it has
	// none.
	Episodes []int
}

var (
	// seasonEpisode is "S01E02", "s01.e02", "S01E02E03" or "S01E02-E05".
	seasonEpisode = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])s(\d{1,3})[\s._-]*e(\d{1,4})`)
	// more is a further episode after one: "E03", "-E05" or "-05".
	more = regexp.MustCompile(`(?i)^[\s._]*(-[\s._]*e?|e)(\d{1,4})`)
	// crossed is "1x02" or "1x02-1x05".
	crossed = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(\d{1,2})x(\d{1,3})(?:-(?:\d{1,2}x)?(\d{1,3}))?(?:[^0-9a-z]|$)`)
	// episodeWord is "Episode 5", "EP0264-0336" or "E05".
	episodeWord = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:episode|ep|e)[\s._-]*(\d{1,4})(?:[\s._]*-[\s._]*(?:ep?)?(\d{1,4}))?(?:[^0-9a-z]|$)`)
	// seasonWord is "Season 2" or "S02".
	seasonWord = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:season[\s._-]*|s)(\d{1,3})(?:[^0-9a-z]|$)`)
	// extension is that of a subtitle or video file.
	extension = regexp.MustCompile(`(?i)\.(?:srt|sub|ssa|ass|vtt|idx|txt|mkv|mp4|avi|zip|rar)$`)
)

// maxRange bounds an expanded range of episodes.
const maxRange = 2000

// Parse reads the season and episodes of a name, as guessit's season and
// episode properties of an episode.
func Parse(name string) Guess {
	stem := extension.ReplaceAllString(name, "")
	if m := seasonEpisode.FindStringSubmatchIndex(stem); m != nil {
		g := Guess{Season: atoi(stem[m[2]:m[3]]), Episodes: []int{atoi(stem[m[4]:m[5]])}}
		rest := stem[m[1]:]
		for {
			n := more.FindStringSubmatchIndex(rest)
			if n == nil {
				break
			}
			after := rest[n[1]:]
			if after != "" && isAlnum(after[0]) && after[0] != 'e' && after[0] != 'E' {
				break
			}
			ep := atoi(rest[n[4]:n[5]])
			if strings.Contains(rest[n[2]:n[3]], "-") {
				g.Episodes = expand(g.Episodes[len(g.Episodes)-1], ep, g.Episodes[:len(g.Episodes)-1])
			} else {
				g.Episodes = append(g.Episodes, ep)
			}
			rest = after
		}
		return g
	}
	if m := crossed.FindStringSubmatch(stem); m != nil {
		g := Guess{Season: atoi(m[1]), Episodes: []int{atoi(m[2])}}
		if m[3] != "" {
			g.Episodes = expand(g.Episodes[0], atoi(m[3]), nil)
		}
		return g
	}
	var g Guess
	if m := seasonWord.FindStringSubmatch(stem); m != nil {
		g.Season = atoi(m[1])
	}
	if m := episodeWord.FindStringSubmatch(stem); m != nil {
		g.Episodes = []int{atoi(m[1])}
		if m[2] != "" {
			g.Episodes = expand(g.Episodes[0], atoi(m[2]), nil)
		}
		return g
	}
	if g.Season == 0 {
		if n, ok := archive.EpisodeNumber(name); ok {
			g.Episodes = []int{n}
		}
	}
	return g
}

// expand appends the episodes from first to last to before; first alone
// when the range is backwards or too long.
func expand(first, last int, before []int) []int {
	if last <= first || last-first > maxRange {
		return append(before, first)
	}
	for e := first; e <= last; e++ {
		before = append(before, e)
	}
	return before
}

// Single returns the one episode a name is of, as guessit reads it; false
// for none and for ranges.
func Single(name string) (int, bool) {
	g := Parse(name)
	if len(g.Episodes) != 1 {
		return 0, false
	}
	return g.Episodes[0], true
}

func isAlnum(c byte) bool {
	return c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func atoi(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// Want is the subtitle wanted from an archive, as the fields of Bazarr's
// subtitles the mixin reads.
type Want struct {
	// Forced subtitles are wanted; files named forced are otherwise
	// skipped.
	Forced bool
	// Season and Episode are the subtitle's; Episode is 0 for a pack.
	Season, Episode int
	// AskedForEpisode is the episode searched for, 0 for a movie.
	AskedForEpisode int
	// Pack is a subtitle of several episodes.
	Pack bool
	// ReleaseGroup is the video's release group, checked against the
	// names of the files of packs; "" for none.
	ReleaseGroup string
}

// subtitleExtensions are those of the files picked.
var subtitleExtensions = []string{".srt", ".sub", ".ssa", ".ass"}

// Pick returns the name of the wanted subtitle of an archive's files, as
// the mixin's get_subtitle_from_archive; "" when none is.
func Pick(names []string, w Want) string {
	var subs []string
	for _, name := range names {
		for _, ext := range subtitleExtensions {
			if strings.HasSuffix(name, ext) {
				subs = append(subs, name)
			}
		}
	}
	if len(subs) == 1 {
		return subs[0]
	}
	var fallback []string
	for _, name := range subs {
		lower := strings.ToLower(name)
		if !w.Forced && strings.HasSuffix(strings.TrimSuffix(lower, path.Ext(lower)), "forced") {
			continue
		}
		g := Parse(name)
		episode := w.AskedForEpisode != 0
		inEpisode := slices.Contains(g.Episodes, w.Episode) || w.Pack && slices.Contains(g.Episodes, w.AskedForEpisode)
		if episode && (!inEpisode || g.Season != w.Season) {
			continue
		}
		groupMatches := true
		if w.Pack && w.ReleaseGroup != "" {
			groupMatches = strings.Contains(lower, strings.ToLower(w.ReleaseGroup))
		}
		switch {
		case !episode && groupMatches:
			return name
		case episode && w.Episode != 0 && slices.Contains(g.Episodes, w.Episode):
			return name
		}
		fallback = append(fallback, name)
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}

// Subtitle returns the wanted subtitle of an archive with Unix line
// endings, as the mixin's get_subtitle_from_archive; nil when none is.
func Subtitle(a *archive.Archive, w Want) ([]byte, string, error) {
	name := Pick(a.Names(), w)
	if name == "" {
		return nil, "", nil
	}
	data, err := a.Read(name)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", name, err)
	}
	return archive.FixLineEndings(data), name, nil
}

// Package archive opens the archives subtitle sites serve subtitles in
// (zip, RAR and 7z) and picks the subtitle of a movie or episode from
// them, as Bazarr's provider utilities.
package archive

import (
	"archive/zip"
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/nwaples/rardecode/v2"
)

// maxFile bounds a file read from an archive.
const maxFile = 10 << 20

// extensions are those of the subtitle files picked from archives.
var extensions = []string{".srt", ".sub", ".ssa", ".ass"}

// ErrNotArchive is content that is neither an archive nor a subtitle.
var ErrNotArchive = errors.New("not an archive or a subtitle")

// Archive is an opened archive.
type Archive struct {
	names []string
	read  func(name string) ([]byte, error)
}

// Names returns the names of the archive's files, folders included.
func (a *Archive) Names() []string { return a.names }

// Read returns a file of the archive.
func (a *Archive) Read(name string) ([]byte, error) { return a.read(name) }

var (
	rarMagic      = []byte("Rar!\x1a\x07")
	sevenZipMagic = []byte("7z\xbc\xaf\x27\x1c")
)

// Open opens an archive, as Bazarr's get_archive_from_bytes: RAR, zip and
// 7z, or a subtitle file, as an archive of that file alone.
func Open(data []byte) (*Archive, error) {
	switch {
	case bytes.HasPrefix(data, rarMagic):
		return openRAR(data)
	case bytes.HasPrefix(data, sevenZipMagic):
		return open7z(data)
	}
	if zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data))); err == nil {
		return openZip(zr), nil
	}
	if format := Detect(data); format != "" {
		name := "subtitle." + format
		return &Archive{names: []string{name}, read: func(n string) ([]byte, error) {
			if n != name {
				return nil, fmt.Errorf("%s: not in the archive", n)
			}
			return data, nil
		}}, nil
	}
	return nil, ErrNotArchive
}

// formats tell subtitle formats by their content, as pysubs2's
// autodetection, in any single-byte or UTF-8 encoding.
var formats = []struct {
	name string
	re   *regexp.Regexp
}{
	{"vtt", regexp.MustCompile(`\A(?:\x{FEFF})?WEBVTT`)},
	{"ass", regexp.MustCompile(`(?m)^\[(?:Script Info|V4\+? Styles|Events)\]`)},
	{"srt", regexp.MustCompile(`(?m)^\s*\d{1,2}:\d{1,2}:\d{1,2}[,.:]\d{1,3}\s*-->\s*\d{1,2}:\d{1,2}:\d{1,2}[,.:]\d{1,3}`)},
	{"sub", regexp.MustCompile(`(?m)^\{\d+\}\{\d*\}`)},
}

// Detect returns the format of a subtitle file, "srt", "ass", "vtt" or
// "sub" (MicroDVD), or "" for content that is none of them.
func Detect(data []byte) string {
	for _, f := range formats {
		if f.re.Match(data) {
			return f.name
		}
	}
	return ""
}

func openZip(zr *zip.Reader) *Archive {
	a := &Archive{}
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		a.names = append(a.names, f.Name)
		files[f.Name] = f
	}
	a.read = func(name string) ([]byte, error) {
		f := files[name]
		if f == nil {
			return nil, fmt.Errorf("%s: not in the archive", name)
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return readLimited(r)
	}
	return a
}

func openRAR(data []byte) (*Archive, error) {
	list := func() (*rardecode.Reader, error) { return rardecode.NewReader(bytes.NewReader(data)) }
	r, err := list()
	if err != nil {
		return nil, fmt.Errorf("rar: %w", err)
	}
	a := &Archive{}
	for {
		h, err := r.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("rar: %w", err)
		}
		a.names = append(a.names, h.Name)
	}
	a.read = func(name string) ([]byte, error) {
		r, err := list()
		if err != nil {
			return nil, err
		}
		for {
			h, err := r.Next()
			if errors.Is(err, io.EOF) {
				return nil, fmt.Errorf("%s: not in the archive", name)
			}
			if err != nil {
				return nil, fmt.Errorf("rar: %w", err)
			}
			if h.Name == name {
				return readLimited(r)
			}
		}
	}
	return a, nil
}

func open7z(data []byte) (*Archive, error) {
	zr, err := sevenzip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("7z: %w", err)
	}
	a := &Archive{}
	files := map[string]*sevenzip.File{}
	for _, f := range zr.File {
		a.names = append(a.names, f.Name)
		files[f.Name] = f
	}
	a.read = func(name string) ([]byte, error) {
		f := files[name]
		if f == nil {
			return nil, fmt.Errorf("%s: not in the archive", name)
		}
		r, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return readLimited(r)
	}
	return a, nil
}

func readLimited(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxFile+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFile {
		return nil, fmt.Errorf("larger than %d bytes", maxFile)
	}
	return data, nil
}

// Want is which subtitle of an archive is wanted.
type Want struct {
	// Forced subtitles are wanted; files named forced are otherwise
	// skipped.
	Forced bool
	// Episode is the episode's number, 0 for a movie.
	Episode int
	// EpisodeTitle is the episode's title, matched against file names
	// when not "".
	EpisodeTitle string
	// First takes the first subtitle file.
	First bool
}

// Subtitle returns the wanted subtitle of an archive, its name and its
// data with Unix line endings, as Bazarr's get_subtitle_from_archive; ""
// when none is.
func Subtitle(a *Archive, w Want) (string, []byte, error) {
	var subs []string
	for _, name := range a.Names() {
		if slices.ContainsFunc(extensions, func(ext string) bool { return strings.HasSuffix(name, ext) }) {
			subs = append(subs, name)
		}
	}
	var name string
	switch {
	case len(subs) == 0:
		return "", nil, nil
	case len(subs) == 1 || w.First:
		name = subs[0]
	default:
		name = Matching(subs, w)
		if name == "" {
			return "", nil, nil
		}
	}
	data, err := a.Read(name)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", name, err)
	}
	return name, FixLineEndings(data), nil
}

// FixLineEndings replaces Windows' and old Macs' line endings with
// Unix', as subliminal's fix_line_ending.
func FixLineEndings(data []byte) []byte {
	return bytes.ReplaceAll(data, []byte("\r\n"), []byte("\n"))
}

type matchingSub struct {
	name     string
	priority int
}

// Matching picks the subtitle of a movie or episode from file names, as
// Bazarr's _get_matching_sub: the first of a movie, else the one of the
// episode's title, then of its number, then a title nearly matching; ""
// for none.
func Matching(names []string, w Want) string {
	var found []matchingSub
	for _, name := range names {
		if !w.Forced && strings.HasSuffix(strings.ToLower(strings.TrimSuffix(name, path.Ext(name))), "forced") {
			continue
		}
		if w.Episode == 0 && w.EpisodeTitle == "" {
			found = append(found, matchingSub{name, 2})
			break
		}
		if w.EpisodeTitle != "" {
			if m, ok := byTitle(name, w.EpisodeTitle); ok {
				found = append(found, m)
			}
		}
		if n, ok := EpisodeNumber(name); w.Episode != 0 && ok && n == w.Episode {
			found = append(found, matchingSub{name, 2})
		}
	}
	if len(found) == 0 {
		return ""
	}
	slices.SortStableFunc(found, func(a, b matchingSub) int { return cmp.Compare(b.priority, a.priority) })
	return found[0].name
}

var titleSplit = regexp.MustCompile(`[\s_.+]?[.-][\s_.+]?`)

// byTitle matches a file name to an episode's title, as Bazarr's
// _analize_sub_name.
func byTitle(name, title string) (matchingSub, bool) {
	for _, part := range titleSplit.Split(strings.TrimSuffix(name, path.Ext(name)), -1) {
		r := ratio(strings.ToLower(strings.TrimSpace(part)), strings.ToLower(title))
		if r <= 0.85 {
			continue
		}
		// Short titles match by chance.
		if len([]rune(title)) > 4 && r >= 0.98 {
			return matchingSub{name, 3}, true
		}
		return matchingSub{name, 1}, true
	}
	return matchingSub{}, false
}

// IsEpisode reports whether a name is an episode's, as Bazarr's
// is_episode.
func IsEpisode(name string) bool {
	_, ok := EpisodeNumber(name)
	return ok
}

var (
	// seasonEpisode is "S01E02", "s01 e02" or "1x02".
	seasonEpisode = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:s(\d{1,3})[\s._-]*e(\d{1,4})|(\d{1,2})x(\d{1,3}))(?:[^0-9]|$)`)
	// episodeWord is "Episode 2", "Ep.02" or "E02".
	episodeWord = regexp.MustCompile(`(?i)(?:^|[^a-z0-9])(?:episode|ep|e)[\s._-]*(\d{1,4})(?:[^0-9a-z]|$)`)
	// leadingNumber is a number at the start of a name: "104 - Pilot",
	// "35. Mondo Magic".
	leadingNumber = regexp.MustCompile(`^\s*(\d{1,4})(?:[\s._-]|$)`)
	// dashedNumber is a number between dashes or ending a name after one:
	// "Show - 05 - Title", "Show - 05".
	dashedNumber = regexp.MustCompile(`\s-\s(\d{1,3})(?:\s-\s|\s*$)`)
	// subtitleExtension is the extension of a subtitle or video file.
	subtitleExtension = regexp.MustCompile(`(?i)\.(?:srt|sub|ssa|ass|vtt|idx|txt|mkv|mp4|avi)$`)
)

// EpisodeNumber returns the episode number of a file name, as guessit
// reads one: from "S01E02" and "1x02", "Episode 2", a number between
// dashes, or a number leading the name, whose last two digits are the
// episode's when it has three or four ("104" is season 1, episode 4),
// but which is a year from 1900 to 2099.
func EpisodeNumber(name string) (int, bool) {
	stem := subtitleExtension.ReplaceAllString(name, "")
	if m := seasonEpisode.FindStringSubmatch(stem); m != nil {
		return atoi(cmp.Or(m[2], m[4])), true
	}
	if m := episodeWord.FindStringSubmatch(stem); m != nil {
		return atoi(m[1]), true
	}
	if m := dashedNumber.FindStringSubmatch(stem); m != nil {
		return atoi(m[1]), true
	}
	if m := leadingNumber.FindStringSubmatch(stem); m != nil {
		n := atoi(m[1])
		switch {
		case len(m[1]) == 4 && n >= 1900 && n < 2100:
			return 0, false
		case len(m[1]) >= 3:
			return n % 100, true
		}
		return n, true
	}
	return 0, false
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		n = n*10 + int(c-'0')
	}
	return n
}

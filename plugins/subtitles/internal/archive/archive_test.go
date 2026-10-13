package archive

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"hash/crc32"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/mavioai/mavio/plugins/subtitles/internal/ported"
	"github.com/mavioai/mavio/plugins/subtitles/internal/provider"
)

func TestPortedUtils(t *testing.T) {
	ported.Run(t, "utils.json", ported.Cases{
		Run: map[string]func(*testing.T, map[string]json.RawMessage){
			"test_get_matching_sub": func(t *testing.T, args map[string]json.RawMessage) {
				names := ported.Arg[[]string](t, args, "sub_names")
				w := Want{Forced: ported.Arg[bool](t, args, "forced"), Episode: ported.Arg[int](t, args, "episode")}
				want := ported.Arg[string](t, args, "expected")
				if got := Matching(names, w); got != want {
					t.Errorf("Matching(%q, %+v) = %q, want = %q", names, w, got, want)
				}
			},
			"test_get_archive_from_bytes_zip": func(t *testing.T, args map[string]json.RawMessage) {
				name := ported.Arg[string](t, args, "filename")
				if _, err := Open(ported.Response(t, name)); err != nil {
					t.Errorf("Open(%s): %v", name, err)
				}
			},
			"test_is_episode": func(t *testing.T, args map[string]json.RawMessage) {
				content, want := ported.Arg[string](t, args, "content"), ported.Arg[bool](t, args, "expected")
				if got := IsEpisode(content); got != want {
					t.Errorf("IsEpisode(%q) = %v, want = %v", content, got, want)
				}
			},
		},
		Facts: map[string]string{
			"test_get_matching_sub_complex_season_pack":        "TestComplexSeasonPack",
			"test_get_matching_sub_complete_series_pack_mixed": "TestCompleteSeriesPackMixed",
			"test_get_subtitle_from_archive_movie":             "TestSubtitleFromArchive",
			"test_get_subtitle_from_archive_season_pack":       "TestSubtitleFromArchive",
			"test_get_archive_from_bytes_none":                 "TestOpenNothing",
		},
		Skip: map[string]string{
			"test_update_matches":          "the server matches release names, in libs/acquire/subtitles' CandidateMatches",
			"test_update_matches_iterable": "the server matches release names, in libs/acquire/subtitles' CandidateMatches",
		},
	})
}

func TestComplexSeasonPack(t *testing.T) {
	files := []string{
		"30. Hard Drive Courage. The Ride Of The Valkyries.srt",
		"S03E15.srt",
		"S03E16.srt",
		"S03E17.srt",
		"28. Campsite Of Terror. The Record Deal.srt",
		"33. Feast Of The Bullfrogs. Tulip's Worm.srt",
		"37. Dome Of Doom. Snowman's Revenge.srt",
		"35. Mondo Magic. Watch The Birdies.srt",
		"29. Stormy Weather. The Sandman Sleeps.srt",
		"38. The Quilt Club. Swindlin' Wind.srt",
	}
	// Courage the Cowardly Dog S03E17 "Mondo Magic".
	if got, want := Matching(files, Want{Episode: 17, EpisodeTitle: "Mondo Magic"}), "35. Mondo Magic. Watch The Birdies.srt"; got != want {
		t.Errorf("Matching = %q, want = %q", got, want)
	}
}

func TestCompleteSeriesPackMixed(t *testing.T) {
	files := []string{
		"11b - Little Bigfoot.srt", "11a - Kiss Kiss, Bang Bang.srt", "10b - The Invaders.srt", "09a - The Tell Tale Tail.srt",
		"01 - The Thing That Wouldn't Stop It.srt", "03b - They Came from Down There.srt", "03a - Bad Day on the Moon.srt",
		"04a - The Friend for Life.srt", "08b - The Glazed McGuffin Affair.srt", "07b - It's Dangly Deever Time.srt",
		"04b - Dysfunction of the Gods.srt", "05b - A Glitch in Time.srt", "12b - Sam & Max vs. the Uglions.srt",
		"08a - Aaiiieee Robot.srt", "02b - Max's Big Day.srt", "05a - Big Trouble at the Earth's Core.srt",
		"13 - The Final Episode.srt", "02a - The Second Show Ever.srt", "10a - Tonight We Love.srt",
		"12a - Fools Die on Friday.srt", "09b - The Trouble with Gary.srt", "06b - We Drop at Dawn.srt",
		"07a - Christmas Bloody Christmas.srt", "06a - That Darn Gator.srt",
	}
	// Bazarr's _gen_subs: the title after the dash, with Python's
	// rstrip(".srt") taking any of those letters off its end.
	for episode, file := range map[int]string{
		15: "11b - Little Bigfoot.srt", 14: "11a - Kiss Kiss, Bang Bang.srt", 17: "10b - The Invaders.srt",
		18: "09a - The Tell Tale Tail.srt", 1: "01 - The Thing That Wouldn't Stop It.srt",
		5: "03b - They Came from Down There.srt", 4: "03a - Bad Day on the Moon.srt", 8: "04a - The Friend for Life.srt",
		21: "08b - The Glazed McGuffin Affair.srt", 13: "07b - It's Dangly Deever Time.srt", 9: "04b - Dysfunction of the Gods.srt",
	} {
		title := strings.TrimRight(strings.TrimSpace(strings.Split(file, "-")[1]), ".srt")
		if got := Matching(files, Want{Episode: episode, EpisodeTitle: title}); got != file {
			t.Errorf("Matching(%d, %q) = %q, want = %q", episode, title, got, file)
		}
	}
}

func TestSubtitleFromArchive(t *testing.T) {
	for _, c := range []struct {
		file string
		want Want
		name string
	}{
		{"archive_1.zip", Want{}, "Safety.Last.1923.720p.BluRay.x264-PublicHD-GABE.Spanish.srt"},
		{"archive_2.rar", Want{Episode: 4}, ""},
	} {
		a, err := Open(ported.Response(t, c.file))
		if err != nil {
			t.Fatal(err)
		}
		name, data, err := Subtitle(a, c.want)
		if err != nil || len(data) == 0 || c.name != "" && name != c.name {
			t.Errorf("Subtitle(%s) = %q, %d bytes, %v, want = %q", c.file, name, len(data), err, c.name)
		}
		if bytes.Contains(data, []byte("\r\n")) {
			t.Errorf("Subtitle(%s) has Windows line endings", c.file)
		}
		t.Logf("%s: %q of %q", c.file, name, a.Names())
	}
}

func TestOpenNothing(t *testing.T) {
	for _, data := range [][]byte{nil, []byte("<html>not found</html>")} {
		if _, err := Open(data); !errors.Is(err, ErrNotArchive) {
			t.Errorf("Open(%q): err = %v, want = %v", data, err, ErrNotArchive)
		}
	}
}

const srt = "1\r\n00:00:01,000 --> 00:00:02,000\r\nHello\r\n"

func TestOpenSubtitle(t *testing.T) {
	a, err := Open([]byte(srt))
	if err != nil {
		t.Fatal(err)
	}
	name, data, err := Subtitle(a, Want{Episode: 3})
	if err != nil || name != "subtitle.srt" || string(data) != strings.ReplaceAll(srt, "\r\n", "\n") {
		t.Errorf("Subtitle = %q, %q, %v", name, data, err)
	}
}

func TestOpen7z(t *testing.T) {
	a, err := Open(stored7z(t, "Show.S01E02.srt", []byte(srt)))
	if err != nil {
		t.Fatal(err)
	}
	name, data, err := Subtitle(a, Want{Episode: 2})
	if err != nil || name != "Show.S01E02.srt" || !strings.Contains(string(data), "Hello") {
		t.Errorf("Subtitle = %q, %q, %v", name, data, err)
	}
}

// stored7z builds a 7z archive of one file stored with the copy method.
func stored7z(t *testing.T, name string, data []byte) []byte {
	t.Helper()
	num := func(n int) byte {
		if n >= 0x80 {
			t.Fatalf("number %d too large for this archive", n)
		}
		return byte(n)
	}
	var h bytes.Buffer
	h.Write([]byte{0x01, 0x04})                                   // header, main streams info
	h.Write([]byte{0x06, 0x00, 0x01, 0x09, num(len(data)), 0x00}) // pack info: position 0, one stream of its size
	h.Write([]byte{0x07, 0x0b, 0x01, 0x00})                       // unpack info: one folder, not external
	h.Write([]byte{0x01, 0x01, 0x00})                             // one coder: a one-byte ID, copy
	h.Write([]byte{0x0c, num(len(data)), 0x00})                   // its unpacked size; end of unpack info
	h.Write([]byte{0x00})                                         // end of main streams info
	var utf16Name bytes.Buffer
	for _, u := range utf16.Encode([]rune(name)) {
		_ = binary.Write(&utf16Name, binary.LittleEndian, u)
	}
	utf16Name.Write([]byte{0, 0})
	h.Write([]byte{0x05, 0x01, 0x11, num(utf16Name.Len() + 1), 0x00}) // files info: one file's name, not external
	h.Write(utf16Name.Bytes())
	h.Write([]byte{0x00, 0x00}) // end of files info, end of header

	var start [20]byte
	binary.LittleEndian.PutUint64(start[0:], uint64(len(data)))
	binary.LittleEndian.PutUint64(start[8:], uint64(h.Len()))
	binary.LittleEndian.PutUint32(start[16:], crc32.ChecksumIEEE(h.Bytes()))
	var out bytes.Buffer
	out.Write(sevenZipMagic)
	out.Write([]byte{0, 4})
	_ = binary.Write(&out, binary.LittleEndian, crc32.ChecksumIEEE(start[:]))
	out.Write(start[:])
	out.Write(data)
	out.Write(h.Bytes())
	return out.Bytes()
}

func TestEpisodeNumber(t *testing.T) {
	for _, c := range []struct {
		name string
		want int
		ok   bool
	}{
		{"Show.S01E02.720p.srt", 2, true},
		{"show s1 e12.srt", 12, true},
		{"Show 3x07.srt", 7, true},
		{"Show Episode 5.srt", 5, true},
		{"Show - 05 - Title.srt", 5, true},
		{"Show - 112.ass", 112, true},
		{"104 - Cancer Man.srt", 4, true},
		{"1012 - Title.srt", 12, true},
		{"35. Mondo Magic. Watch The Birdies.srt", 35, true},
		{"09a - The Tell Tale Tail.srt", 0, false},
		{"1999 - Title.srt", 0, false},
		{"taxi driver 1976", 0, false},
		{"Safety.Last.1923.720p.BluRay.x264-PublicHD.srt", 0, false},
	} {
		if got, ok := EpisodeNumber(c.name); got != c.want || ok != c.ok {
			t.Errorf("EpisodeNumber(%q) = %d, %v, want = %d, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}

func TestDetect(t *testing.T) {
	for _, c := range []struct{ data, want string }{
		{srt, "srt"},
		{"\ufeffWEBVTT\n\n00:01.000 --> 00:02.000\nHi\n", "vtt"},
		{"[Script Info]\nTitle: x\n", "ass"},
		{"{1}{25}Hello\n", "sub"},
		{"<html></html>", ""},
	} {
		if got := Detect([]byte(c.data)); got != c.want {
			t.Errorf("Detect(%q) = %q, want = %q", c.data, got, c.want)
		}
	}
}

func TestExtract(t *testing.T) {
	v := &provider.Video{Episode: true, Number: 4}
	l := provider.Language{Tag: "en"}
	f, err := Extract(ported.Response(t, "archive_2.rar"), WantFor(v, l), l)
	if err != nil || f.Format != "srt" || f.Language != l || len(f.Data) == 0 {
		t.Errorf("Extract = %+v, %v", f, err)
	}
	if _, err := Extract(ported.Response(t, "archive_2.rar"), Want{Episode: 9}, l); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("Extract of a missing episode: err = %v, want = BadSubtitle", err)
	}
	if _, err := Extract([]byte("<html>"), Want{}, l); provider.KindOf(err) != provider.BadSubtitle {
		t.Errorf("Extract of a page: err = %v, want = BadSubtitle", err)
	}
	f, err = Extract([]byte("{1}{25}Hello\n"), Want{}, l)
	if err != nil || f.Format != "sub" {
		t.Errorf("Extract of MicroDVD = %+v, %v", f, err)
	}
}

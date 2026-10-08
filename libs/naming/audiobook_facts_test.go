package naming

import (
	"slices"
	"testing"
)

func TestPortedAudioBookListResolver(t *testing.T) {
	portedCases(t, "audio_book/audio_book_list_resolver.json", ported{facts: map[string]string{
		"TestStackAndExtras":      "TestAudioBooksStackAndExtras",
		"TestAlternativeVersions": "TestAudioBooksAlternativeVersions",
		"TestNameYearExtraction":  "TestAudioBooksNameYear",
		"TestWithMetadata":        "TestAudioBooksSingle",
		"TestWithExtra":           "TestAudioBooksSingle",
		"TestWithoutFolder":       "TestAudioBooksSingle",
		"TestEmpty":               "TestAudioBooksSingle",
	}})
}

func TestAudioBooksStackAndExtras(t *testing.T) {
	books := parser.ResolveAudioBooks([]string{
		"Harry Potter and the Deathly Hallows/Part 1.mp3",
		"Harry Potter and the Deathly Hallows/Part 2.mp3",
		"Harry Potter and the Deathly Hallows/Extra.mp3",
		"Batman/Chapter 1.mp3",
		"Batman/Chapter 2.mp3",
		"Batman/Chapter 3.mp3",
		"Badman/audiobook.mp3",
		"Badman/extra.mp3",
		"Superman (2020)/Part 1.mp3",
		"Superman (2020)/extra.mp3",
		"Ready Player One (2020)/audiobook.mp3",
		"Ready Player One (2020)/extra.mp3",
		".mp3",
	})
	want := []struct {
		name          string
		files, extras int
	}{
		{"Harry Potter and the Deathly Hallows", 2, 1},
		{"Batman", 3, 0},
		{"Badman", 1, 1},
		{"Superman", 1, 1},
		{"Ready Player One", 1, 1},
	}
	if len(books) != len(want) {
		t.Fatalf("books: got = %d, want = %d", len(books), len(want))
	}
	for i, w := range want {
		b := books[i]
		if b.Name != w.name || len(b.Files) != w.files || len(b.Extras) != w.extras {
			t.Errorf("book %d: got = %q %d files %d extras, want = %q %d files %d extras",
				i, b.Name, len(b.Files), len(b.Extras), w.name, w.files, w.extras)
		}
	}
}

func TestAudioBooksAlternativeVersions(t *testing.T) {
	books := parser.ResolveAudioBooks([]string{
		"Harry Potter and the Deathly Hallows/Chapter 1.ogg",
		"Harry Potter and the Deathly Hallows/Chapter 1.mp3",
		"Deadpool.mp3",
		"Deadpool [HQ].mp3",
		"Superman/audiobook.mp3",
		"Superman/Superman.mp3",
		"Superman/Superman [HQ].mp3",
		"Superman/extra.mp3",
		"Batman/ Chapter 1 .mp3",
		"Batman/Chapter 1[loss-less].mp3",
	})
	if len(books) != 5 {
		t.Fatalf("books: got = %d, want = 5", len(books))
	}
	// Harry Potter: same name, either file is the alternative. The two
	// Deadpools have no folder and are not grouped.
	for i, want := range []int{1, 0, 0, 2, 1} {
		if got := len(books[i].AlternateVersions); got != want {
			t.Errorf("book %d alternates: got = %d, want = %d", i, got, want)
		}
	}
	// Superman: the file named after the book is the main one, then
	// "audiobook", then names with modifiers.
	var paths []string
	for _, f := range books[3].AlternateVersions {
		paths = append(paths, f.Path)
	}
	if !slices.Contains(paths, "Superman/audiobook.mp3") || !slices.Contains(paths, "Superman/Superman [HQ].mp3") {
		t.Errorf("Superman alternates: got = %q", paths)
	}
}

func TestAudioBooksNameYear(t *testing.T) {
	data := []struct {
		name, path string
		year       *int
	}{
		{"Harry Potter and the Deathly Hallows", "Harry Potter and the Deathly Hallows (2007)/Chapter 1.ogg", ptr(2007)},
		{"Batman", "Batman (2020).ogg", ptr(2020)},
		{"Batman", "Batman( 2021 ).mp3", ptr(2021)},
		{"Batman(*2021*)", "Batman(*2021*).mp3", nil},
		{"Batman", "Batman.mp3", nil},
		{"+ Batman .", " + Batman . .mp3", nil},
		{" ", " .mp3", nil},
	}
	var paths []string
	for _, d := range data {
		paths = append(paths, d.path)
	}
	books := parser.ResolveAudioBooks(paths)
	if len(books) != len(data) {
		t.Fatalf("books: got = %d, want = %d", len(books), len(data))
	}
	for i, d := range data {
		checkString(t, d.path, books[i].Name, d.name)
		checkInt(t, d.path, books[i].Year, d.year)
	}
}

func TestAudioBooksSingle(t *testing.T) {
	for _, tt := range []struct {
		files []string
		want  int
	}{
		{[]string{"Harry Potter and the Deathly Hallows/Chapter 1.ogg", "Harry Potter and the Deathly Hallows/Harry Potter and the Deathly Hallows.nfo"}, 1},
		{[]string{"Harry Potter and the Deathly Hallows/Chapter 1.mp3", "Harry Potter and the Deathly Hallows/Harry Potter and the Deathly Hallows trailer.mp3"}, 1},
		{[]string{"Harry Potter and the Deathly Hallows trailer.mp3"}, 1},
		{nil, 0},
	} {
		if got := len(parser.ResolveAudioBooks(tt.files)); got != tt.want {
			t.Errorf("%q: got = %d books, want = %d", tt.files, got, tt.want)
		}
	}
}

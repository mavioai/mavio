package library

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/mavioai/mavio/libs/metadata"
)

// titles reads and writes ".title" files: an overview and a tagline, one
// per line.
type titles struct {
	read  [][]string
	media []string
	// save is what Save returns; nil writes "<media>.title".
	save []LocalFile
}

func (*titles) Name() string { return "titles" }

func (*titles) Patterns() []string { return []string{"*.title", "extra.txt"} }

func (t *titles) ReadLocal(_ context.Context, _ Lookup, media string, files []LocalFile) (*metadata.Result, error) {
	var names []string
	for _, f := range files {
		names = append(names, f.Name)
	}
	t.read, t.media = append(t.read, names), append(t.media, media)
	overview, tagline, _ := strings.Cut(string(files[0].Content), "\n")
	res := &metadata.Result{}
	res.Item.Overview, res.Item.Tagline = overview, tagline
	return res, nil
}

func (t *titles) Save(_ context.Context, media string, res *metadata.Result) ([]LocalFile, error) {
	if t.save != nil {
		return t.save, nil
	}
	return []LocalFile{{Name: media + ".title", Content: []byte(res.Item.Overview + "\n" + res.Item.Tagline)}}, nil
}

func writeBytes(t *testing.T, root, rel string, data []byte) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, filepath.FromSlash(rel)), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLocalReaders(t *testing.T) {
	f := newManage(t, false)
	r := &titles{}
	f.r.Local = func() []LocalReader { return []LocalReader{r} }
	writeBytes(t, f.root, "Heat (1995)/Heat (1995).mkv.title", []byte("A thief and a detective.\nA Los Angeles crime saga."))
	writeBytes(t, f.root, "Heat (1995)/big.title", make([]byte, MaxLocalFileSize+1))
	writeBytes(t, f.root, "Heat (1995)/notes.txt", []byte("not read"))

	// The reader's metadata wins over the provider's.
	it := f.refresh(RefreshOptions{})
	if it.Overview != "A thief and a detective." || it.Tagline != "A Los Angeles crime saga." {
		t.Errorf("after reading: overview = %q, tagline = %q", it.Overview, it.Tagline)
	}
	if want := [][]string{{"Heat (1995).mkv.title"}}; !slices.EqualFunc(r.read, want, slices.Equal) || r.media[0] != "Heat (1995).mkv" {
		t.Errorf("read %q for %q, want = %q", r.read, r.media, want)
	}

	// The NFO file wins over the reader.
	writeBytes(t, f.root, "Heat (1995)/Heat (1995).nfo", []byte("<movie><plot>From the NFO.</plot></movie>"))
	if it = f.refresh(RefreshOptions{}); it.Overview != "From the NFO." || it.Tagline != "A Los Angeles crime saga." {
		t.Errorf("with an NFO: overview = %q, tagline = %q", it.Overview, it.Tagline)
	}

	// Replacing metadata ignores local files.
	if it = f.refresh(RefreshOptions{ReplaceMetadata: true}); it.Overview != "A Las Vegas bodyguard." {
		t.Errorf("replaced: overview = %q", it.Overview)
	}
}

func TestSavers(t *testing.T) {
	f := newManage(t, true)
	s := &titles{}
	f.r.Savers = func() []Saver { return []Saver{s} }
	f.refresh(RefreshOptions{})
	if got := f.file("Heat (1995)/Heat (1995).mkv.title"); got != "A Las Vegas bodyguard.\n" {
		t.Errorf("saved = %q", got)
	}

	// Files outside the item's folder, too big, or replacing the media are
	// refused, with all the saver's other files.
	for _, bad := range []LocalFile{
		{Name: "../escape.title"},
		{Name: "/abs.title"},
		{Name: "Heat (1995).mkv", Content: []byte("gone")},
		{Name: "huge.title", Content: make([]byte, MaxLocalFileSize+1)},
	} {
		s.save = []LocalFile{{Name: "ok.title", Content: []byte("ok")}, bad}
		f.refresh(RefreshOptions{})
		if exists(f.root, "Heat (1995)/ok.title") || exists(f.root, "escape.title") || f.file("Heat (1995)/Heat (1995).mkv") == "gone" {
			t.Errorf("saver returning %q wrote files", bad.Name)
		}
	}
}

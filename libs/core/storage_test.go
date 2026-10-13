package core

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"
	"testing/fstest"
	"time"
)

func TestValidLibraryPath(t *testing.T) {
	for _, tt := range []struct {
		path string
		ok   bool
	}{
		{"/media/movies", true},
		{`C:\Media`, true},
		{"c:/media", true},
		{`\\nas\media`, true},
		{"media/movies", false},
		{"rclone:gdrive:Movies", true},
		{"rclone:gdrive:Media/Movies", true},
		{"rclone:gdrive:", false},
		{"rclone::Movies", false},
		{"rclone:gdrive", false},
		{"rclone:gdrive:/Movies", false},
		{"rclone:gdrive:Movies/", false},
		{"rclone:gdrive:../Movies", false},
		{"rclone:gdrive:Media/../Movies", false},
		{"rclone:g/drive:Movies", false},
	} {
		if err := validLibraryPath(tt.path); (err == nil) != tt.ok {
			t.Errorf("validLibraryPath(%q) = %v, want ok = %v", tt.path, err, tt.ok)
		}
	}
}

func TestSplitRemotePath(t *testing.T) {
	remote, rest, ok := SplitRemotePath("rclone:gdrive:Media/Movies")
	if !ok || remote != "gdrive" || rest != "Media/Movies" {
		t.Errorf("SplitRemotePath() = %q, %q, %v, want = gdrive, Media/Movies, true", remote, rest, ok)
	}
	if _, _, ok := SplitRemotePath("/media"); ok {
		t.Error("SplitRemotePath(/media) is remote")
	}
}

func TestStorageName(t *testing.T) {
	for _, tt := range []struct {
		root, path, name string
		ok               bool
	}{
		{"/media", "/media", ".", true},
		{"/media", "/media/a/b.mkv", "a/b.mkv", true},
		{"/media/", "/media/a.mkv", "a.mkv", true},
		{"/media", "/mediafiles/a.mkv", "", false},
		{"/media", "/media/../etc/passwd", "", false},
		{`C:\Media`, "C:/Media/a.mkv", "a.mkv", true},
		{"C:/Media", `C:\Media\a.mkv`, "a.mkv", true},
		{`C:\Media\`, "C:/Media", ".", true},
		{`C:\Media`, `C:\Media2\a.mkv`, "", false},
		{`C:\Media`, `C:\Media\..\a.mkv`, "", false},
		{"C:/Media", "C:/Media/a/b.mkv", "a/b.mkv", true},
		{`C:\Media`, `C:\Media\a\b.mkv`, "a/b.mkv", true},
		{"rclone:gdrive:Movies", "rclone:gdrive:Movies/a/b.mkv", "a/b.mkv", true},
		{"rclone:gdrive:Movies", "rclone:gdrive:Movies 2/a.mkv", "", false},
	} {
		name, ok := StorageName(tt.root, tt.path)
		if name != tt.name || ok != tt.ok {
			t.Errorf("StorageName(%q, %q) = %q, %v, want = %q, %v", tt.root, tt.path, name, ok, tt.name, tt.ok)
		}
		if !ok {
			continue
		}
		p := StoragePath(tt.root, name)
		if back, _ := StorageName(tt.root, p); back != name {
			t.Errorf("StorageName(%q, StoragePath() = %q) = %q, want = %q", tt.root, p, back, name)
		}
	}
}

func TestInFolder(t *testing.T) {
	for _, tt := range []struct {
		path, folder string
		want         bool
	}{
		{"/media", "/media", true},
		{"/media/a.mkv", "/media/", true},
		{"/mediafiles", "/media", false},
		{"/media", "/media/a", false},
		{"/anything", "/", true},
		{"rclone:gdrive:Movies", "/", false},
		{`C:\Media\a.mkv`, "C:/Media", true},
		{"C:/Media/a.mkv", `C:\Media`, true},
		{`C:\Media2`, `C:\Media`, false},
		{"rclone:gdrive:Movies/a.mkv", "rclone:gdrive:Movies", true},
		{"rclone:gdrive:Movies 2", "rclone:gdrive:Movies", false},
	} {
		if got := InFolder(tt.path, tt.folder); got != tt.want {
			t.Errorf("InFolder(%q, %q) = %v, want = %v", tt.path, tt.folder, got, tt.want)
		}
	}
}

// mapStorage is a read-only storage of an fstest.MapFS.
type mapStorage struct{ fs fstest.MapFS }

func (m mapStorage) Stat(_ context.Context, name string) (fs.FileInfo, error) {
	return m.fs.Stat(name)
}

func (m mapStorage) ReadDir(_ context.Context, name string) ([]fs.DirEntry, error) {
	return m.fs.ReadDir(name)
}

func (m mapStorage) Open(_ context.Context, name string) (File, error) {
	f, err := m.fs.Open(name)
	if err != nil {
		return nil, err
	}
	if file, ok := f.(File); ok {
		return file, nil
	}
	_ = f.Close()
	return nil, errors.New("not a file")
}

func (mapStorage) WriteFile(context.Context, string, []byte) error { return ErrUnsupported }
func (mapStorage) Create(context.Context, string) (io.WriteCloser, error) {
	return nil, ErrUnsupported
}
func (mapStorage) MkdirAll(context.Context, string) error           { return ErrUnsupported }
func (mapStorage) Rename(context.Context, string, string) error     { return ErrUnsupported }
func (mapStorage) Copy(context.Context, string, string) error       { return ErrUnsupported }
func (mapStorage) Link(context.Context, string, string) error       { return ErrUnsupported }
func (mapStorage) Remove(context.Context, string) error             { return ErrUnsupported }
func (mapStorage) Chtimes(context.Context, string, time.Time) error { return ErrUnsupported }
func (mapStorage) MediaURL(context.Context, string) (string, error) { return "", ErrUnsupported }
func (mapStorage) Capabilities() StorageCapabilities                { return StorageCapabilities{} }
func (mapStorage) Close() error                                     { return nil }

func TestStorageFS(t *testing.T) {
	s := mapStorage{fstest.MapFS{
		"movie.nfo":         {Data: []byte("<movie/>")},
		"Movie/movie.mkv":   {Data: []byte("matroska")},
		"Movie/poster.jpg":  {Data: []byte("jpeg")},
		"Show/S01/e01.mkv":  {Data: []byte("episode")},
		"Show/tvshow.nfo":   {Data: []byte("<tvshow/>")},
		"Show/S01/e01.srt":  {Data: []byte("1\n")},
		"Show/S01/.ignore":  {Data: nil},
		"Show/extras/a.mkv": {Data: []byte("extra")},
	}}
	fsys := StorageFS(t.Context(), s)
	if err := fstest.TestFS(fsys, "movie.nfo", "Movie/movie.mkv", "Show/S01/e01.srt", "Show/extras/a.mkv"); err != nil {
		t.Fatal(err)
	}
	if _, err := fs.Stat(fsys, "../x"); !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("Stat(../x) error = %v, want = %v", err, fs.ErrInvalid)
	}
}

func TestReadIntent(t *testing.T) {
	if got := ReadIntentOf(context.Background()); got != (ReadIntent{}) {
		t.Errorf("ReadIntentOf(background) = %+v, want = ReadAny", got)
	}
	want := ReadIntent{Kind: ReadPlayback, Bitrate: 8_000_000}
	if got := ReadIntentOf(WithReadIntent(context.Background(), want)); got != want {
		t.Errorf("ReadIntentOf() = %+v, want = %+v", got, want)
	}
}

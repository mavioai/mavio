package library

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// chapteredProber probes videos with a stream, borders and chapters, and
// tracks as audio.
type chapteredProber struct{}

func (chapteredProber) Probe(_ context.Context, _ string, audio bool) (ProbeResult, error) {
	if audio {
		return ProbeResult{Source: core.MediaSource{
			Container: "flac", Duration: 3 * time.Minute,
			Streams: []core.MediaStream{{Index: 0, Kind: core.StreamAudio, Codec: "flac"}},
		}}, nil
	}
	return ProbeResult{Source: core.MediaSource{
		Container: "mkv", Duration: 30 * time.Minute,
		Streams:  []core.MediaStream{{Index: 0, Kind: core.StreamVideo, Codec: "h264", Width: 1920, Height: 1080, Crop: &core.Crop{Top: 140, Bottom: 140}}},
		Chapters: []core.Chapter{{Start: 0, Title: "One"}, {Start: 10 * time.Minute, Title: "Two"}},
	}}, nil
}

// fakeThumbs writes empty images, failing for files named in fail.
type fakeThumbs struct {
	trickplays, chapters int
	at                   []time.Duration
	frames               []VideoFrame
	fail                 bool
}

func (f *fakeThumbs) Trickplay(_ context.Context, _ string, v VideoFrame, dir string) (core.Trickplay, error) {
	f.trickplays++
	f.frames = append(f.frames, v)
	if f.fail {
		return core.Trickplay{}, errors.New("broken GOP")
	}
	if err := os.WriteFile(filepath.Join(dir, "0.jpg"), []byte("jpg"), 0o644); err != nil {
		return core.Trickplay{}, err
	}
	return core.Trickplay{Width: 320, Height: 133, TileWidth: 10, TileHeight: 10, ThumbnailCount: 180, Interval: 10 * time.Second}, nil
}

func (f *fakeThumbs) ChapterImage(_ context.Context, _ string, at time.Duration, _ VideoFrame, file string) error {
	f.chapters++
	f.at = append(f.at, at)
	return os.WriteFile(file, []byte("jpg"), 0o644)
}

type fakeLoudness map[string]float64

func (f fakeLoudness) Loudness(_ context.Context, path string) (float64, error) {
	return f[filepath.Base(path)], nil
}

type fakeSegments struct{ name string }

func (f fakeSegments) Name() string { return f.name }

func (f fakeSegments) Segments(_ context.Context, q SegmentQuery) ([]core.MediaSegment, error) {
	return []core.MediaSegment{
		{Kind: core.SegmentIntro, Start: 0, End: time.Minute},
		{Kind: core.SegmentOutro, Start: q.Duration - time.Minute, End: q.Duration},
	}, nil
}

func TestMediaExtras(t *testing.T) {
	f := newScan(t, core.LibraryMovies)
	f.lib.ExtractTrickplay, f.lib.ExtractChapterImages = true, true
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	tree(t, f.root, "Up (2009)/Up (2009).mkv")
	thumbs := &fakeThumbs{}
	meta := t.TempDir()
	jobs := &Jobs{
		Store: f.store, Scanner: f.sc, Prober: chapteredProber{}, Now: func() time.Time { return f.clock },
		Refresher: &Refresher{Store: f.store}, Thumbnails: thumbs, MetadataDir: meta,
		Segments: func() []SegmentProvider { return []SegmentProvider{fakeSegments{"a"}, fakeSegments{"b"}} },
		Fails:    NewFailNotes(time.Hour, 10),
	}
	f.store.clock = func() time.Time { return f.clock }
	w := &Worker{Queue: f.store.Jobs(), Owner: "test", Handlers: jobs.Handlers()}
	f.scan()
	drain(t, w)
	up := f.item("Up (2009)/Up (2009).mkv")
	if thumbs.trickplays != 1 || thumbs.frames[0].Crop.Top != 140 {
		t.Errorf("trickplays = %d, frames %+v", thumbs.trickplays, thumbs.frames)
	}
	if tp := f.store.tricks[up.ID]; len(tp) != 1 || tp[0].ItemID != up.ID {
		t.Errorf("trickplay = %+v", tp)
	}
	if _, err := os.Stat(filepath.Join(TrickplayDir(meta, up.ID, 320), "0.jpg")); err != nil {
		t.Errorf("sheet: %v", err)
	}
	// The first chapter's image is taken a little in, past the black.
	if thumbs.chapters != 2 || thumbs.at[0] != 10*time.Second || thumbs.at[1] != 10*time.Minute {
		t.Errorf("chapter images at %v", thumbs.at)
	}
	if ch := f.store.sources[up.ID][0].Chapters; ch[0].ImagePath == "" || ch[1].ImagePath == "" {
		t.Errorf("chapters = %+v", ch)
	}
	// Both providers found the same segments; the first one's are kept.
	if segs := f.store.segs[up.ID]; len(segs) != 2 || segs[0].Provider != "a" || segs[1].End != 30*time.Minute {
		t.Errorf("segments = %+v", segs)
	}

	// A failing video is not read again while the note lasts.
	thumbs.fail = true
	job := ItemJob(JobTrickplay, up.ID, f.clock)
	if _, err := jobs.trickplay(t.Context(), job); err == nil {
		t.Fatal("failed trickplay: no error")
	}
	if _, err := jobs.trickplay(t.Context(), job); err != nil || thumbs.trickplays != 2 {
		t.Errorf("retry within the note: err %v, trickplays %d, want 2", err, thumbs.trickplays)
	}
}

func TestLoudness(t *testing.T) {
	f := newScan(t, core.LibraryMusic)
	f.lib.AnalyzeLoudness = true
	if err := f.store.Libraries().Create(t.Context(), &f.lib); err != nil {
		t.Fatal(err)
	}
	tree(t, f.root, "Band/Album/01 - One.flac", "Band/Album/02 - Two.flac")
	jobs := &Jobs{
		Store: f.store, Scanner: f.sc, Prober: chapteredProber{}, Now: func() time.Time { return f.clock },
		Refresher: &Refresher{Store: f.store}, Loudness: fakeLoudness{"01 - One.flac": -10, "02 - Two.flac": -20},
	}
	f.store.clock = func() time.Time { return f.clock }
	w := &Worker{Queue: f.store.Jobs(), Owner: "test", Handlers: jobs.Handlers()}
	f.scan()
	drain(t, w)
	one := f.item("Band/Album/01 - One.flac")
	if one.Loudness == nil || *one.Loudness != -10 || *one.NormalizationGain() != -8 {
		t.Fatalf("track loudness = %v", one.Loudness)
	}
	album := f.store.items[one.ParentID]
	// Equal durations: the mean of 10^-1 and 10^-2.
	want := 10 * math.Log10((0.1+0.01)/2)
	if album.Loudness == nil || math.Abs(*album.Loudness-want) > 1e-9 {
		t.Errorf("album loudness = %v, want %v", album.Loudness, want)
	}
}

func TestFailNotes(t *testing.T) {
	now := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	f := NewFailNotes(time.Hour, 2)
	f.Now = func() time.Time { return now }
	f.Note("a")
	now = now.Add(time.Minute)
	f.Note("b")
	f.Note("c") // forgets a, the oldest
	if f.Failed("a") || !f.Failed("b") || !f.Failed("c") {
		t.Error("capacity")
	}
	now = now.Add(time.Hour)
	if f.Failed("b") {
		t.Error("a note outlived its TTL")
	}
	var none *FailNotes
	none.Note("x")
	if none.Failed("x") {
		t.Error("nil notes failed")
	}
}

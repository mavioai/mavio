package library

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// Jobs making what media has beyond its streams.
const (
	// JobTrickplay makes a video's trickplay sheets.
	JobTrickplay = "media.trickplay"
	// JobChapterImages takes an image of each chapter of a video.
	JobChapterImages = "media.chapters"
	// JobLoudness measures the loudness of a track.
	JobLoudness = "media.loudness"
	// JobSegments asks the segment providers for a video's segments.
	JobSegments = "media.segments"
	// JobExtras queues the extras jobs of every item of a library.
	JobExtras = "library.extras"
)

// ExtrasPriority runs extras after everything else but border detection,
// whose crop trickplay and chapter images use.
const ExtrasPriority = BordersPriority - 10

// Thumbnailer takes images of videos, such as with ffmpeg.
type Thumbnailer interface {
	// Trickplay writes the sheets of a video into dir as 0.jpg, 1.jpg, …
	// and describes them.
	Trickplay(ctx context.Context, path string, v VideoFrame, dir string) (core.Trickplay, error)
	// ChapterImage writes the frame at a time into file.
	ChapterImage(ctx context.Context, path string, at time.Duration, v VideoFrame, file string) error
}

// VideoFrame describes the picture of a video.
type VideoFrame struct {
	Duration      time.Duration
	Width, Height int
	Crop          core.Crop
}

// LoudnessMeter measures the integrated loudness of audio, in LUFS.
type LoudnessMeter interface {
	Loudness(ctx context.Context, path string) (float64, error)
}

// SegmentProvider finds the segments of videos, such as a segment plugin.
type SegmentProvider interface {
	Name() string
	Segments(ctx context.Context, q SegmentQuery) ([]core.MediaSegment, error)
}

// SegmentQuery describes the video segments are looked for in.
type SegmentQuery struct {
	Kind              core.ItemKind
	Name              string
	ExternalIDs       map[core.Provider]string
	SeriesExternalIDs map[core.Provider]string
	// SeasonNumber and EpisodeNumber place episodes.
	SeasonNumber, EpisodeNumber *int
	Path                        string
	Duration                    time.Duration
	Chapters                    []core.Chapter
}

// FailNotes remember the files whose images failed, so that broken media
// is not read again and again while a note lasts.
type FailNotes struct {
	// TTL is how long a note lasts; Capacity how many are kept, the
	// oldest forgotten first.
	TTL      time.Duration
	Capacity int
	Now      func() time.Time

	mu    sync.Mutex
	notes map[string]time.Time
}

// NewFailNotes keeps up to capacity notes for ttl.
func NewFailNotes(ttl time.Duration, capacity int) *FailNotes {
	return &FailNotes{TTL: ttl, Capacity: capacity}
}

func (f *FailNotes) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

// failKey names a file as it is now.
func failKey(kind string, src *core.MediaSource) string {
	return kind + "\x00" + src.Path + "\x00" + strconv.FormatInt(src.Size, 10) + "\x00" + src.Modified.UTC().Format(time.RFC3339Nano)
}

// Failed reports whether a note for key lasts.
func (f *FailNotes) Failed(key string) bool {
	if f == nil {
		return false
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	at, ok := f.notes[key]
	if ok && f.now().Sub(at) >= f.TTL {
		delete(f.notes, key)
		return false
	}
	return ok
}

// Note records a failure.
func (f *FailNotes) Note(key string) {
	if f == nil {
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.notes == nil {
		f.notes = map[string]time.Time{}
	}
	if len(f.notes) >= f.Capacity {
		oldest, oldestAt := "", time.Time{}
		for k, at := range f.notes {
			if oldest == "" || at.Before(oldestAt) {
				oldest, oldestAt = k, at
			}
		}
		delete(f.notes, oldest)
	}
	f.notes[key] = f.now()
}

// ItemJob returns a job of kind about an item, once per item while queued.
func ItemJob(kind string, itemID core.ID, at time.Time) core.Job {
	payload, _ := json.Marshal(ItemPayload{ItemID: itemID})
	return core.Job{
		ID: core.NewID(), Kind: kind, Payload: payload, UniqueKey: kind + ":" + itemID.String(),
		MaxAttempts: 2, RunAt: at, Priority: ExtrasPriority,
	}
}

// ExtrasJob returns the job queueing the extras of every item of a
// library.
func ExtrasJob(lib core.Library, at time.Time) core.Job {
	payload, _ := json.Marshal(LibraryPayload{LibraryID: lib.ID})
	return core.Job{
		ID: core.NewID(), Kind: JobExtras, Payload: payload, UniqueKey: JobExtras + ":" + lib.ID.String(),
		MaxAttempts: 3, RunAt: at, Priority: ExtrasPriority,
	}
}

// extrasJobs returns the extras jobs of a probed item: those its library
// asks for and the server can do.
func (j *Jobs) extrasJobs(lib core.Library, it core.Item, video bool, chapters bool) []core.Job {
	now := j.now()
	var out []core.Job
	switch {
	case video:
		if j.Thumbnails != nil && lib.ExtractTrickplay {
			out = append(out, ItemJob(JobTrickplay, it.ID, now))
		}
		if j.Thumbnails != nil && lib.ExtractChapterImages && chapters {
			out = append(out, ItemJob(JobChapterImages, it.ID, now))
		}
		if j.Segments != nil && it.Extra == "" && (it.Kind == core.KindMovie || it.Kind == core.KindEpisode) {
			out = append(out, ItemJob(JobSegments, it.ID, now))
		}
	case it.Kind == core.KindTrack:
		if j.Loudness != nil && lib.AnalyzeLoudness {
			out = append(out, ItemJob(JobLoudness, it.ID, now))
		}
	}
	return out
}

// extrasFolder is an item's folder within the metadata folder.
func extrasFolder(metadataDir string, id core.ID, parts ...string) string {
	return filepath.Join(append([]string{metadataDir, id.String()[:2], id.String()}, parts...)...)
}

// TrickplayDir returns the folder of an item's trickplay sheets at a
// width within the metadata folder.
func TrickplayDir(metadataDir string, itemID core.ID, width int) string {
	return extrasFolder(metadataDir, itemID, "trickplay", strconv.Itoa(width))
}

// ChapterImagePath returns the file of a chapter's image of a media
// source of an item within the metadata folder.
func ChapterImagePath(metadataDir string, itemID, sourceID core.ID, index int) string {
	return filepath.Join(extrasFolder(metadataDir, itemID, "chapters", sourceID.String()), strconv.Itoa(index)+".jpg")
}

// videoSource returns an item's first probed video file and its video
// stream, nil when it has none.
func videoSource(sources []core.MediaSource) (*core.MediaSource, *core.MediaStream) {
	for i := range sources {
		src := &sources[i]
		if src.ProbedAt.IsZero() || src.Disc != "" || src.Duration <= 0 {
			continue
		}
		for k := range src.Streams {
			st := &src.Streams[k]
			if st.Kind == core.StreamVideo && st.ExternalPath == "" && st.Width > 0 && st.Height > 0 && !st.IsDolbyVisionEnhancement() {
				return src, st
			}
		}
	}
	return nil, nil
}

func frameOf(src *core.MediaSource, st *core.MediaStream) VideoFrame {
	v := VideoFrame{Duration: src.Duration, Width: st.Width, Height: st.Height}
	if st.Crop != nil {
		v.Crop = *st.Crop
	}
	return v
}

// itemAndLibrary loads a job's item and its library.
func (j *Jobs) itemAndLibrary(ctx context.Context, job core.Job) (core.Item, core.Library, error) {
	var p ItemPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return core.Item{}, core.Library{}, fmt.Errorf("%s payload: %w", job.Kind, err)
	}
	it, err := j.Store.Items().Get(ctx, p.ItemID)
	if err != nil {
		return it, core.Library{}, err
	}
	lib, err := j.Store.Libraries().Get(ctx, it.LibraryID)
	return it, lib, err
}

// quiet waits for foreground I/O to pause.
func (j *Jobs) quiet(ctx context.Context) error {
	if j.Scanner != nil && j.Scanner.QuietGate != nil {
		return j.Scanner.QuietGate.PauseOrCancel(ctx)
	}
	return nil
}

func (j *Jobs) trickplay(ctx context.Context, job core.Job) ([]core.Job, error) {
	if j.Thumbnails == nil || j.MetadataDir == "" {
		return nil, nil
	}
	it, lib, err := j.itemAndLibrary(ctx, job)
	if err != nil || !lib.ExtractTrickplay {
		return nil, ignoreGone(err)
	}
	sources, err := j.Store.MediaSources().ListForItem(ctx, it.ID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	src, st := videoSource(sources)
	if src == nil {
		return nil, nil
	}
	key := failKey(JobTrickplay, src)
	if j.Fails.Failed(key) {
		return nil, nil
	}
	if err := j.quiet(ctx); err != nil {
		return nil, err
	}
	// The sheets are written aside and replace the old ones at once.
	parent := extrasFolder(j.MetadataDir, it.ID, "trickplay")
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return nil, err
	}
	tmp, err := os.MkdirTemp(parent, ".new-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)
	t, err := j.Thumbnails.Trickplay(ctx, src.Path, frameOf(src, st), tmp)
	if err != nil {
		if ctx.Err() == nil {
			j.Fails.Note(key)
		}
		return nil, err
	}
	t.ItemID = it.ID
	dir := TrickplayDir(j.MetadataDir, it.ID, t.Width)
	if err := os.RemoveAll(dir); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, dir); err != nil {
		return nil, err
	}
	return nil, ignoreGone(j.Store.Trickplay().Put(ctx, &t))
}

// chapterTime is where a chapter's image is taken: its start, or a little
// after the start of a video, which is often black.
func chapterTime(chapters []core.Chapter, i int, duration time.Duration) time.Duration {
	at := chapters[i].Start
	if at == 0 {
		end := duration
		if i+1 < len(chapters) {
			end = chapters[i+1].Start
		}
		at = min(10*time.Second, end/10)
	}
	return at
}

func (j *Jobs) chapterImages(ctx context.Context, job core.Job) ([]core.Job, error) {
	if j.Thumbnails == nil || j.MetadataDir == "" {
		return nil, nil
	}
	it, lib, err := j.itemAndLibrary(ctx, job)
	if err != nil || !lib.ExtractChapterImages {
		return nil, ignoreGone(err)
	}
	sources, err := j.Store.MediaSources().ListForItem(ctx, it.ID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	src, st := videoSource(sources)
	if src == nil || len(src.Chapters) == 0 {
		return nil, nil
	}
	key := failKey(JobChapterImages, src)
	if j.Fails.Failed(key) {
		return nil, nil
	}
	dir := extrasFolder(j.MetadataDir, it.ID, "chapters", src.ID.String())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	images := map[int]string{}
	for i := range src.Chapters {
		if src.Chapters[i].ImagePath != "" {
			continue
		}
		if err := j.quiet(ctx); err != nil {
			return nil, err
		}
		file := ChapterImagePath(j.MetadataDir, it.ID, src.ID, i)
		if err := j.Thumbnails.ChapterImage(ctx, src.Path, chapterTime(src.Chapters, i, src.Duration), frameOf(src, st), file); err != nil {
			if ctx.Err() != nil {
				return nil, err
			}
			j.Fails.Note(key)
			j.logger().WarnContext(ctx, "chapter image failed", "item", it.ID, "chapter", i, "err", err)
			break
		}
		images[i] = file
	}
	if len(images) == 0 {
		return nil, nil
	}
	err = j.Store.InTx(ctx, func(tx core.Store) error {
		current, err := tx.MediaSources().ListForItem(ctx, it.ID)
		if err != nil {
			return err
		}
		i := slices.IndexFunc(current, func(s core.MediaSource) bool { return s.ID == src.ID })
		if i < 0 || len(current[i].Chapters) != len(src.Chapters) {
			return nil
		}
		for n, file := range images {
			current[i].Chapters[n].ImagePath = filepath.ToSlash(file)
		}
		return tx.MediaSources().Replace(ctx, it.ID, current)
	})
	return nil, ignoreGone(err)
}

func (j *Jobs) loudness(ctx context.Context, job core.Job) ([]core.Job, error) {
	if j.Loudness == nil {
		return nil, nil
	}
	it, lib, err := j.itemAndLibrary(ctx, job)
	if err != nil || !lib.AnalyzeLoudness || it.Kind != core.KindTrack {
		return nil, ignoreGone(err)
	}
	sources, err := j.Store.MediaSources().ListForItem(ctx, it.ID)
	if err != nil || len(sources) == 0 {
		return nil, ignoreGone(err)
	}
	key := failKey(JobLoudness, &sources[0])
	if j.Fails.Failed(key) {
		return nil, nil
	}
	if err := j.quiet(ctx); err != nil {
		return nil, err
	}
	lufs, err := j.Loudness.Loudness(ctx, sources[0].Path)
	if err != nil {
		if ctx.Err() == nil {
			j.Fails.Note(key)
		}
		return nil, err
	}
	err = j.Store.InTx(ctx, func(tx core.Store) error {
		it, err := tx.Items().Get(ctx, it.ID)
		if err != nil {
			return err
		}
		it.Loudness = &lufs
		if err := tx.Items().Upsert(ctx, it); err != nil {
			return err
		}
		return albumLoudness(ctx, tx, it.ParentID)
	})
	return nil, ignoreGone(err)
}

// albumLoudness sets an album's loudness once all its tracks have theirs:
// the mean of their energies, weighted by duration.
func albumLoudness(ctx context.Context, tx core.Store, albumID core.ID) error {
	if albumID.IsZero() {
		return nil
	}
	album, err := tx.Items().Get(ctx, albumID)
	if err != nil || album.Kind != core.KindMusicAlbum {
		return err
	}
	var energy, total float64
	for tr, err := range tx.Items().Walk(ctx, core.ItemQuery{ParentID: albumID, Kinds: []core.ItemKind{core.KindTrack}}) {
		if err != nil {
			return err
		}
		if tr.Loudness == nil {
			return nil
		}
		w := max(tr.Runtime.Seconds(), 1)
		energy += w * math.Pow(10, *tr.Loudness/10)
		total += w
	}
	if total == 0 {
		return nil
	}
	l := 10 * math.Log10(energy/total)
	album.Loudness = &l
	return tx.Items().Upsert(ctx, album)
}

func (j *Jobs) segments(ctx context.Context, job core.Job) ([]core.Job, error) {
	if j.Segments == nil {
		return nil, nil
	}
	it, _, err := j.itemAndLibrary(ctx, job)
	if err != nil {
		return nil, ignoreGone(err)
	}
	sources, err := j.Store.MediaSources().ListForItem(ctx, it.ID)
	if err != nil || len(sources) == 0 {
		return nil, ignoreGone(err)
	}
	q := SegmentQuery{
		Kind: it.Kind, Name: it.Name, ExternalIDs: it.ExternalIDs, Path: sources[0].Path,
		Duration: sources[0].Duration, Chapters: sources[0].Chapters,
	}
	if it.Kind == core.KindEpisode {
		q.SeasonNumber, q.EpisodeNumber = it.ParentIndexNumber, it.IndexNumber
		for id := it.ParentID; !id.IsZero(); {
			p, err := j.Store.Items().Get(ctx, id)
			if err != nil {
				break
			}
			if p.Kind == core.KindSeries {
				q.SeriesExternalIDs = p.ExternalIDs
				break
			}
			id = p.ParentID
		}
	}
	var all []core.MediaSegment
	for _, p := range j.Segments() {
		found, err := p.Segments(ctx, q)
		if err != nil {
			j.logger().WarnContext(ctx, "segment provider failed", "provider", p.Name(), "item", it.ID, "err", err)
			continue
		}
		for _, s := range found {
			s.ID, s.ItemID, s.Provider = core.NewID(), it.ID, p.Name()
			if s.Validate() == nil && !slices.ContainsFunc(all, func(o core.MediaSegment) bool { return o.Kind == s.Kind && overlap(o, s) }) {
				all = append(all, s)
			}
		}
	}
	return nil, ignoreGone(j.Store.MediaSegments().Replace(ctx, it.ID, all))
}

// overlap reports whether two segments overlap; the first provider's
// segment of a kind wins.
func overlap(a, b core.MediaSegment) bool { return a.Start < b.End && b.Start < a.End }

// extras queues the extras jobs of every probed item of a library.
func (j *Jobs) extras(ctx context.Context, job core.Job) ([]core.Job, error) {
	var p LibraryPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("extras payload: %w", err)
	}
	lib, err := j.Store.Libraries().Get(ctx, p.LibraryID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	var next []core.Job
	kinds := []core.ItemKind{core.KindMovie, core.KindEpisode, core.KindVideo, core.KindMusicVideo, core.KindTrack}
	for it, err := range j.Store.Items().Walk(ctx, core.ItemQuery{LibraryIDs: []core.ID{lib.ID}, Kinds: kinds, IncludeExtras: true}) {
		if err != nil {
			return nil, err
		}
		sources, err := j.Store.MediaSources().ListForItem(ctx, it.ID)
		if err != nil {
			return nil, err
		}
		src, _ := videoSource(sources)
		next = append(next, j.extrasJobs(lib, it, src != nil, src != nil && len(src.Chapters) > 0)...)
	}
	return next, nil
}

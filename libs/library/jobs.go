package library

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library/storage"
)

// Job kinds besides JobProbe.
const (
	// JobScan scans a library; the payload is a LibraryPayload.
	JobScan = "library.scan"
	// JobRefresh refreshes an item's metadata; the payload is an
	// ItemPayload.
	JobRefresh = "item.refresh"
)

// LibraryPayload is the payload of JobScan jobs.
type LibraryPayload struct {
	LibraryID core.ID `json:"library_id"`
	// Scheduled scans enqueue the next one after the library's scan
	// interval.
	Scheduled bool `json:"scheduled,omitempty"`
}

// ItemPayload is the payload of JobRefresh jobs.
type ItemPayload struct {
	ItemID core.ID `json:"item_id"`
}

// ScanJob returns a job scanning lib at the given time. Scheduled and
// requested scans have separate keys, so that asking for a scan does not
// wait for the next scheduled one.
func ScanJob(lib core.Library, at time.Time, scheduled bool) core.Job {
	payload, _ := json.Marshal(LibraryPayload{LibraryID: lib.ID, Scheduled: scheduled})
	key := JobScan + ":" + lib.ID.String()
	if scheduled {
		key += ":scheduled"
	}
	return core.Job{ID: core.NewID(), Kind: JobScan, Payload: payload, UniqueKey: key, MaxAttempts: 3, RunAt: at, Priority: 10}
}

// RefreshJob returns a job refreshing an item's metadata.
func RefreshJob(itemID core.ID, at time.Time) core.Job {
	payload, _ := json.Marshal(ItemPayload{ItemID: itemID})
	return core.Job{ID: core.NewID(), Kind: JobRefresh, Payload: payload, UniqueKey: JobRefresh + ":" + itemID.String(), MaxAttempts: 3, RunAt: at}
}

// ProbeResult is what probing a media file found.
type ProbeResult struct {
	// Source carries the container facts, streams and chapters.
	Source core.MediaSource
	// Tags are the metadata embedded in audio files, such as title,
	// artists and track number.
	Tags core.Item
}

// Prober reads the facts of a media file, such as with ffprobe.
type Prober interface {
	Probe(ctx context.Context, path string, audio bool) (ProbeResult, error)
}

// Jobs runs the library's background jobs: scans, probes of new and changed
// media files, and metadata refreshes.
type Jobs struct {
	Store   core.Store
	Scanner *Scanner
	Prober  Prober
	// Keyframes extracts the keyframes of probed video files; nil skips
	// them.
	Keyframes KeyframeExtractor
	Refresher *Refresher
	// Images measures items' images and computes their placeholders after
	// each refresh; nil skips them.
	Images ImageAnalyzer
	Now    func() time.Time
	Logger *slog.Logger

	mu    sync.Mutex
	scans map[core.ID]*sync.Mutex
}

func (j *Jobs) logger() *slog.Logger {
	if j.Logger != nil {
		return j.Logger
	}
	return slog.New(slog.DiscardHandler)
}

func (j *Jobs) now() time.Time {
	if j.Now != nil {
		return j.Now()
	}
	return time.Now()
}

// Handlers returns the handlers for a Worker.
func (j *Jobs) Handlers() map[string]Handler {
	return map[string]Handler{
		JobScan:         j.scan,
		JobProbe:        j.probe,
		JobRefresh:      j.refresh,
		JobKeyframes:    j.keyframes,
		JobPlaceholders: j.placeholders,
	}
}

// Schedule enqueues a scan of every library with folders now, as on server
// startup, and for libraries with a scan interval the scheduled scan after
// it, unless one is pending; each scheduled scan enqueues the next.
func (j *Jobs) Schedule(ctx context.Context) error {
	libs, err := j.Store.Libraries().List(ctx)
	if err != nil {
		return err
	}
	now := j.now()
	for _, lib := range libs {
		if lib.Kind.Curated() {
			continue
		}
		jobs := []core.Job{ScanJob(lib, now, false)}
		if lib.ScanInterval > 0 {
			jobs = append(jobs, ScanJob(lib, now.Add(lib.ScanInterval), true))
		}
		for i := range jobs {
			if _, err := j.Store.Jobs().Enqueue(ctx, &jobs[i]); err != nil {
				return err
			}
		}
	}
	return nil
}

// libraryLock serializes the scans of one library, which a scheduled and a
// requested scan could otherwise run at once.
func (j *Jobs) libraryLock(id core.ID) *sync.Mutex {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.scans == nil {
		j.scans = map[core.ID]*sync.Mutex{}
	}
	if j.scans[id] == nil {
		j.scans[id] = &sync.Mutex{}
	}
	return j.scans[id]
}

func (j *Jobs) scan(ctx context.Context, job core.Job) ([]core.Job, error) {
	var p LibraryPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("scan payload: %w", err)
	}
	lib, err := j.Store.Libraries().Get(ctx, p.LibraryID)
	if err != nil || lib.Kind.Curated() {
		return nil, err
	}
	lock := j.libraryLock(lib.ID)
	lock.Lock()
	defer lock.Unlock()
	if _, err := j.Scanner.Scan(ctx, lib); err != nil {
		return nil, err
	}
	if p.Scheduled && lib.ScanInterval > 0 {
		return []core.Job{ScanJob(lib, j.now().Add(lib.ScanInterval), true)}, nil
	}
	return nil, nil
}

// probe probes an item's media sources that have not been probed since
// their file changed, and queues a metadata refresh.
func (j *Jobs) probe(ctx context.Context, job core.Job) ([]core.Job, error) {
	var p ProbePayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("probe payload: %w", err)
	}
	it, err := j.Store.Items().Get(ctx, p.ItemID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	sources, err := j.Store.MediaSources().ListForItem(ctx, it.ID)
	if err != nil {
		return nil, err
	}
	audio := it.Kind == core.KindTrack || it.Kind == core.KindAudioBook
	probed := map[string]ProbeResult{}
	for _, src := range sources {
		if !src.ProbedAt.IsZero() || src.Disc != "" {
			continue
		}
		if j.Scanner != nil && j.Scanner.QuietGate != nil {
			if err := j.Scanner.QuietGate.PauseOrCancel(ctx); err != nil {
				return nil, err
			}
		}
		if j.Scanner != nil && j.Scanner.VolumeLedger != nil {
			dev, _ := storage.DetectDevice(src.Path)
			if dev.ID != "" {
				rel, err := j.Scanner.VolumeLedger.Acquire(ctx, dev.ID)
				if err != nil {
					return nil, err
				}
				defer rel()
			}
		}
		_ = storage.PrefetchHeadTail(src.Path, 1024*1024, 256*1024)
		res, err := j.Prober.Probe(ctx, src.Path, audio)
		if err != nil {
			return nil, fmt.Errorf("probe %s: %w", src.Path, err)
		}
		probed[src.Path] = res
	}
	if len(probed) > 0 {
		err = j.Store.InTx(ctx, func(tx core.Store) error {
			// A scan may have replaced the sources meanwhile; only results
			// for files still known as they were probed are kept.
			current, err := tx.MediaSources().ListForItem(ctx, it.ID)
			if err != nil {
				return err
			}
			for i, src := range current {
				res, ok := probed[src.Path]
				if !ok || !slices.ContainsFunc(sources, func(s core.MediaSource) bool {
					return s.Path == src.Path && s.Size == src.Size && s.Modified.Equal(src.Modified)
				}) {
					continue
				}
				r := res.Source
				src.Container, src.Duration, src.Bitrate = r.Container, r.Duration, r.Bitrate
				src.Streams, src.Chapters = r.Streams, r.Chapters
				src.ProbedAt = j.now()
				current[i] = src
			}
			if err := tx.MediaSources().Replace(ctx, it.ID, current); err != nil {
				return err
			}
			it, err := tx.Items().Get(ctx, it.ID)
			if err != nil {
				return err
			}
			if len(current) > 0 && current[0].Duration > 0 && !slices.Contains(it.LockedFields, core.FieldRuntime) {
				it.Runtime = current[0].Duration
			}
			if audio {
				if res, ok := probed[it.Path]; ok {
					applyMetadata(&it, res.Tags)
				}
			}
			return tx.Items().Upsert(ctx, it)
		})
		if err != nil {
			return nil, ignoreGone(err)
		}
	}
	next := []core.Job{RefreshJob(it.ID, j.now())}
	if j.Keyframes != nil && !audio && len(probed) > 0 {
		next = append(next, KeyframesJob(it.ID, j.now(), KeyframesBackground))
	}
	return next, nil
}

func (j *Jobs) refresh(ctx context.Context, job core.Job) ([]core.Job, error) {
	var p ItemPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("refresh payload: %w", err)
	}
	it, err := j.Store.Items().Get(ctx, p.ItemID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	lib, err := j.Store.Libraries().Get(ctx, it.LibraryID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	if err := j.Refresher.Refresh(ctx, lib, it.ID); err != nil {
		return nil, ignoreGone(err)
	}
	if j.Images == nil {
		return nil, nil
	}
	return []core.Job{PlaceholdersJob(it.ID, j.now())}, nil
}

// ignoreGone treats items deleted since the job was queued as done.
func ignoreGone(err error) error {
	if err != nil && isNotFound(err) {
		return nil
	}
	return err
}

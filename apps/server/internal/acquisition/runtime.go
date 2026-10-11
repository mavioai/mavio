package acquisition

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mavioai/mavio/apps/server/internal/downloads"
	"github.com/mavioai/mavio/apps/server/internal/releases"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/library/storage"
)

// The kinds of the acquisition jobs.
const (
	// JobSearch searches for a movie and grabs the best release; the
	// payload is a SearchPayload.
	JobSearch = "acquisition.search"
	// JobRSS reads the indexers' recent releases and grabs those of
	// wanted movies and episodes, every RSSSyncInterval minutes.
	JobRSS = "acquisition.rss"
	// JobQueue checks the tracked downloads in their clients, imports the
	// completed and handles the failed, every CheckFinishedInterval
	// minutes.
	JobQueue = "acquisition.queue"
	// JobRecycleBin deletes what has been in the recycle bin longer than
	// RecycleBinCleanupDays, daily.
	JobRecycleBin = "acquisition.recycle_bin"
)

// SearchPayload is the payload of JobSearch jobs.
type SearchPayload struct {
	ItemID core.ID `json:"item_id"`
	// UserInvoked is set for searches a user asked for, which skip the
	// delay profiles as Radarr's do.
	UserInvoked bool `json:"user_invoked,omitempty"`
}

// ScheduledPayload is the payload of the recurring jobs.
type ScheduledPayload struct {
	// Scheduled runs enqueue the next one.
	Scheduled bool `json:"scheduled,omitempty"`
}

// Identifier identifies an item by external IDs and refreshes its
// metadata, as library.Refresher does.
type Identifier interface {
	Identify(ctx context.Context, lib core.Library, itemID core.ID, ids map[core.Provider]string) (core.Item, error)
}

// Runtime acquires movies and episodes. Its zero value is not usable; Store and
// Settings are required.
type Runtime struct {
	Store core.Store
	// Settings returns the current acquisition settings.
	Settings func() core.AcquisitionSettings
	// Indexers and Clients return the ready indexer and download client
	// provider plugins; nil returns none.
	Indexers func() []releases.Provider
	Clients  func() []downloads.Provider
	// SceneProviders returns the ready scene mapping provider plugins;
	// nil returns none.
	SceneProviders func() []SceneProvider
	// Identifier identifies added movies; nil leaves them as added.
	Identifier Identifier
	// Prober reads imported files; without it no file is imported, as
	// none can be told from a sample.
	Prober library.Prober
	// VolumeLedger, QuietGate and Devices pace imports on rotational and
	// remote volumes, as for scans; nil leaves imports unpaced.
	VolumeLedger *storage.VolumeLedger
	QuietGate    *storage.QuietGate
	Devices      *storage.Detector
	// Imported is told, after an import is stored, of the movie or series
	// a file was imported for; nil tells none. It runs without the
	// runtime's locks held.
	Imported func(ctx context.Context, itemID core.ID)
	Now      func() time.Time
	Logger   *slog.Logger

	mu sync.Mutex
	// statuses are what the clients last told of the tracked downloads,
	// by download ID.
	statuses map[core.ID]ClientStatus
	// devices detects volumes when Devices is nil.
	devices *storage.Detector
	// scene caches the scene mapping providers' mappings.
	scene sceneCache
}

func (r *Runtime) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// imported tells Imported of an import.
func (r *Runtime) imported(ctx context.Context, itemID core.ID) {
	if r.Imported != nil {
		r.Imported(ctx, itemID)
	}
}

func (r *Runtime) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.New(slog.DiscardHandler)
}

func (r *Runtime) settings() core.AcquisitionSettings {
	if r.Settings == nil {
		return core.DefaultAcquisitionSettings()
	}
	return r.Settings()
}

func (r *Runtime) indexers() []releases.Provider {
	if r.Indexers == nil {
		return nil
	}
	return r.Indexers()
}

func (r *Runtime) clients() []downloads.Provider {
	if r.Clients == nil {
		return nil
	}
	return r.Clients()
}

func (r *Runtime) searcher() *releases.Searcher {
	return &releases.Searcher{Providers: r.Indexers, Items: r.Store.Items()}
}

// Handlers returns the handlers of the acquisition jobs for a
// library.Worker.
func (r *Runtime) Handlers() map[string]library.Handler {
	return map[string]library.Handler{
		JobSearch: func(ctx context.Context, job core.Job) ([]core.Job, error) {
			var p SearchPayload
			if err := json.Unmarshal(job.Payload, &p); err != nil {
				return nil, fmt.Errorf("search payload: %w", err)
			}
			return nil, r.SearchMovie(ctx, p.ItemID, p.UserInvoked)
		},
		JobRSS: r.recurring(JobRSS, func(s core.AcquisitionSettings) time.Duration {
			return time.Duration(s.RSSSyncInterval) * time.Minute
		}, r.SyncRSS),
		JobQueue: r.recurring(JobQueue, func(s core.AcquisitionSettings) time.Duration {
			return time.Duration(max(s.CheckFinishedInterval, 1)) * time.Minute
		}, r.CheckQueue),
		JobRecycleBin: r.recurring(JobRecycleBin, func(core.AcquisitionSettings) time.Duration { return 24 * time.Hour }, r.CleanRecycleBin),
		JobEpisodeSearch: func(ctx context.Context, job core.Job) ([]core.Job, error) {
			var p EpisodeSearchPayload
			if err := json.Unmarshal(job.Payload, &p); err != nil {
				return nil, fmt.Errorf("episode search payload: %w", err)
			}
			return nil, r.SearchEpisodes(ctx, p)
		},
		JobSceneMappings: r.recurring(JobSceneMappings, func(core.AcquisitionSettings) time.Duration { return SceneMappingInterval }, r.UpdateSceneMappings),
	}
}

// recurring returns the handler of a recurring job, which runs run and
// schedules the next run after the interval the settings give; an
// interval of 0 schedules none.
func (r *Runtime) recurring(kind string, interval func(core.AcquisitionSettings) time.Duration, run func(context.Context) error) library.Handler {
	return func(ctx context.Context, job core.Job) ([]core.Job, error) {
		var p ScheduledPayload
		if len(job.Payload) > 0 {
			if err := json.Unmarshal(job.Payload, &p); err != nil {
				return nil, fmt.Errorf("%s payload: %w", kind, err)
			}
		}
		err := run(ctx)
		if !p.Scheduled {
			return nil, err
		}
		if d := interval(r.settings()); d > 0 {
			return []core.Job{scheduledJob(kind, r.now().Add(d), true)}, err
		}
		return nil, err
	}
}

// scheduledJob returns a recurring job at a time. Scheduled and
// requested runs have separate keys, so that asking for a run does not
// wait for the next scheduled one.
func scheduledJob(kind string, at time.Time, scheduled bool) core.Job {
	payload, _ := json.Marshal(ScheduledPayload{Scheduled: scheduled})
	key := kind
	if scheduled {
		key += ":scheduled"
	}
	return core.Job{ID: core.NewID(), Kind: kind, Payload: payload, UniqueKey: key, MaxAttempts: 1, RunAt: at}
}

// SearchJob returns a job searching for a movie.
func SearchJob(itemID core.ID, userInvoked bool, at time.Time) core.Job {
	payload, _ := json.Marshal(SearchPayload{ItemID: itemID, UserInvoked: userInvoked})
	return core.Job{ID: core.NewID(), Kind: JobSearch, Payload: payload, UniqueKey: JobSearch + ":" + itemID.String(), MaxAttempts: 1, RunAt: at}
}

// Schedule enqueues the recurring jobs that are not pending: the queue
// check, the recycle bin cleanup and the scene mappings update, and the
// RSS sync unless s disables
// it. It is called at startup and whenever the settings change, with the
// settings applied: settings appliers run under the settings lock, so it
// must not read the settings through Settings.
func (r *Runtime) Schedule(ctx context.Context, s core.AcquisitionSettings) error {
	now := r.now()
	jobs := []core.Job{scheduledJob(JobQueue, now, true), scheduledJob(JobRecycleBin, now, true), scheduledJob(JobSceneMappings, now, true)}
	if s.RSSSyncInterval > 0 {
		jobs = append(jobs, scheduledJob(JobRSS, now, true))
	}
	for i := range jobs {
		if _, err := r.Store.Jobs().Enqueue(ctx, &jobs[i]); err != nil {
			return fmt.Errorf("schedule %s: %w", jobs[i].Kind, err)
		}
	}
	return nil
}

// QueueSearch enqueues searches for movies and reports how many were
// queued; a movie whose search is pending counts as queued.
func (r *Runtime) QueueSearch(ctx context.Context, itemIDs []core.ID, userInvoked bool) (int, error) {
	now := r.now()
	for _, id := range itemIDs {
		job := SearchJob(id, userInvoked, now)
		if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
			return 0, fmt.Errorf("queue search: %w", err)
		}
	}
	return len(itemIDs), nil
}

// Task is a recurring acquisition job, as administrators see and run it
// among the server's tasks.
type Task struct {
	// Kind is the job kind it runs, which is also its task ID.
	Kind, Name, Description string
	// Interval is how often it runs; 0 when it runs on demand only.
	Interval time.Duration
}

// Tasks returns the recurring jobs with the intervals the settings give:
// RSS syncs, queue checks, recycle bin cleanups and scene mapping
// updates, as Radarr's and Sonarr's tasks.
func (r *Runtime) Tasks() []Task {
	s := r.settings()
	return []Task{
		{
			Kind: JobRSS, Name: "RSS sync", Interval: time.Duration(s.RSSSyncInterval) * time.Minute,
			Description: "Reads the indexers' recent releases and grabs those of wanted movies and episodes.",
		},
		{
			Kind: JobQueue, Name: "Refresh monitored downloads", Interval: time.Duration(max(s.CheckFinishedInterval, 1)) * time.Minute,
			Description: "Checks the tracked downloads in their clients, imports the completed ones and handles the failed ones.",
		},
		{
			Kind: JobRecycleBin, Name: "Clean up recycle bin", Interval: 24 * time.Hour,
			Description: "Deletes what has been in the recycle bin longer than the settings keep it.",
		},
		{
			Kind: JobSceneMappings, Name: "Update scene mapping", Interval: SceneMappingInterval,
			Description: "Reads the scene mappings and daily series of the scene mapping providers.",
		},
	}
}

// RunJob returns a job running a recurring job now, apart from its
// scheduled runs.
func RunJob(kind string, at time.Time) core.Job { return scheduledJob(kind, at, false) }

// QueueRSS enqueues an RSS sync.
func (r *Runtime) QueueRSS(ctx context.Context) error {
	job := RunJob(JobRSS, r.now())
	if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
		return fmt.Errorf("queue RSS sync: %w", err)
	}
	return nil
}

package subtitles

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/media/vad"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1/pluginv1connect"
)

// The kinds of the subtitle jobs.
const (
	// JobSearch searches for the missing subtitles of a movie or an
	// episode; the payload is a SearchPayload.
	JobSearch = "subtitles.search"
	// JobWantedMovies and JobWantedEpisodes search for the missing
	// subtitles of every movie and episode, every
	// WantedSearchIntervalMovie and WantedSearchInterval.
	JobWantedMovies   = "subtitles.wanted_movies"
	JobWantedEpisodes = "subtitles.wanted_episodes"
	// JobUpgrade searches for better subtitles than those downloaded
	// lately, every UpgradeInterval.
	JobUpgrade = "subtitles.upgrade"
)

// ErrUnavailable is the error of what cannot be done now, such as
// synchronizing subtitles without ffmpeg or of videos on cloud drives.
var ErrUnavailable = errors.New("unavailable")

// SearchPayload is the payload of JobSearch jobs.
type SearchPayload struct {
	ItemID core.ID `json:"item_id"`
}

// scheduledPayload is the payload of the recurring jobs.
type scheduledPayload struct {
	// Scheduled runs enqueue the next one.
	Scheduled bool `json:"scheduled,omitempty"`
}

// Provider is a subtitle provider plugin.
type Provider struct {
	ID     string
	Client pluginv1connect.SubtitleProviderServiceClient
}

// Activity records what happened.
type Activity interface {
	Publish(a core.Activity)
}

// Runtime acquires subtitles. Store and Subtitles are required.
type Runtime struct {
	Store core.Store
	// Settings returns the current subtitle settings; nil uses the
	// defaults.
	Settings func() core.SubtitleSettings
	// Providers returns the ready subtitle provider plugins; nil returns
	// none.
	Providers func() []Provider
	// Subtitles reads and writes the subtitle files beside videos.
	Subtitles *library.Subtitles
	// Storages opens the storage of library paths; nil opens local
	// folders only.
	Storages *library.Storages
	// Class returns the storage class of a path, which decides what the
	// server reads on its own; nil takes rclone remotes for cloud drives
	// and other paths for local disks.
	Class func(ctx context.Context, path string) core.StorageClass
	// FFmpeg is the ffmpeg synchronization reads audio with; "" does not
	// synchronize against audio.
	FFmpeg string
	// Activity records downloads; nil records none.
	Activity Activity
	Now      func() time.Time
	Logger   *slog.Logger

	vadOnce  sync.Once
	vadModel *vad.Model
	vadErr   error
}

func (r *Runtime) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

func (r *Runtime) logger() *slog.Logger {
	if r.Logger != nil {
		return r.Logger
	}
	return slog.New(slog.DiscardHandler)
}

func (r *Runtime) settings() core.SubtitleSettings {
	if r.Settings == nil {
		return core.DefaultSubtitleSettings()
	}
	return r.Settings()
}

func (r *Runtime) providers() []Provider {
	if r.Providers == nil {
		return nil
	}
	return r.Providers()
}

func (r *Runtime) publish(a core.Activity) {
	if r.Activity != nil {
		r.Activity.Publish(a)
	}
}

// vad returns the voice activity detector, loaded once.
func (r *Runtime) vad() (*vad.Model, error) {
	r.vadOnce.Do(func() { r.vadModel, r.vadErr = vad.NewModel() })
	return r.vadModel, r.vadErr
}

// Handlers returns the handlers of the subtitle jobs for a
// library.Worker.
func (r *Runtime) Handlers() map[string]library.Handler {
	return map[string]library.Handler{
		JobSearch: func(ctx context.Context, job core.Job) ([]core.Job, error) {
			var p SearchPayload
			if err := json.Unmarshal(job.Payload, &p); err != nil {
				return nil, fmt.Errorf("subtitle search payload: %w", err)
			}
			_, err := r.SearchItem(ctx, p.ItemID)
			return nil, err
		},
		JobWantedMovies: r.recurring(JobWantedMovies, func(s core.SubtitleSettings) time.Duration { return s.WantedSearchIntervalMovie },
			func(ctx context.Context) error { return r.SearchWanted(ctx, core.KindMovie) }),
		JobWantedEpisodes: r.recurring(JobWantedEpisodes, func(s core.SubtitleSettings) time.Duration { return s.WantedSearchInterval },
			func(ctx context.Context) error { return r.SearchWanted(ctx, core.KindEpisode) }),
		JobUpgrade: r.recurring(JobUpgrade, func(s core.SubtitleSettings) time.Duration {
			if !s.Upgrade {
				return 0
			}
			return s.UpgradeInterval
		}, r.Upgrade),
	}
}

// recurring returns the handler of a recurring job, which runs run and
// schedules the next run after the interval the settings give; an
// interval of 0 schedules none.
func (r *Runtime) recurring(kind string, interval func(core.SubtitleSettings) time.Duration, run func(context.Context) error) library.Handler {
	return func(ctx context.Context, job core.Job) ([]core.Job, error) {
		var p scheduledPayload
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
	payload, _ := json.Marshal(scheduledPayload{Scheduled: scheduled})
	key := kind
	if scheduled {
		key += ":scheduled"
	}
	return core.Job{ID: core.NewID(), Kind: kind, Payload: payload, UniqueKey: key, MaxAttempts: 1, RunAt: at}
}

// RunJob returns a job running a recurring job now, apart from its
// scheduled runs.
func RunJob(kind string, at time.Time) core.Job { return scheduledJob(kind, at, false) }

// SearchJob returns a job searching for an item's missing subtitles.
func SearchJob(itemID core.ID, at time.Time) core.Job {
	payload, _ := json.Marshal(SearchPayload{ItemID: itemID})
	return core.Job{ID: core.NewID(), Kind: JobSearch, Payload: payload, UniqueKey: JobSearch + ":" + itemID.String(), MaxAttempts: 1, RunAt: at}
}

// Schedule enqueues the recurring jobs s turns on that are not pending.
// It is called at startup and whenever the settings change, with the
// settings applied: settings appliers run under the settings lock, so it
// must not read the settings through Settings.
func (r *Runtime) Schedule(ctx context.Context, s core.SubtitleSettings) error {
	now := r.now()
	var jobs []core.Job
	if s.WantedSearchIntervalMovie > 0 {
		jobs = append(jobs, scheduledJob(JobWantedMovies, now.Add(s.WantedSearchIntervalMovie), true))
	}
	if s.WantedSearchInterval > 0 {
		jobs = append(jobs, scheduledJob(JobWantedEpisodes, now.Add(s.WantedSearchInterval), true))
	}
	if s.Upgrade && s.UpgradeInterval > 0 {
		jobs = append(jobs, scheduledJob(JobUpgrade, now.Add(s.UpgradeInterval), true))
	}
	for i := range jobs {
		if _, err := r.Store.Jobs().Enqueue(ctx, &jobs[i]); err != nil {
			return fmt.Errorf("schedule %s: %w", jobs[i].Kind, err)
		}
	}
	return nil
}

// QueueSearch enqueues searches for the missing subtitles of items and
// reports how many were queued; an item whose search is pending counts
// as queued.
func (r *Runtime) QueueSearch(ctx context.Context, itemIDs []core.ID) (int, error) {
	now := r.now()
	for _, id := range itemIDs {
		job := SearchJob(id, now)
		if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
			return 0, fmt.Errorf("queue subtitle search: %w", err)
		}
	}
	return len(itemIDs), nil
}

// Queue enqueues a run of a recurring job.
func (r *Runtime) Queue(ctx context.Context, kind string) error {
	job := RunJob(kind, r.now())
	if _, err := r.Store.Jobs().Enqueue(ctx, &job); err != nil {
		return fmt.Errorf("queue %s: %w", kind, err)
	}
	return nil
}

// Task is a recurring subtitle job, as administrators see and run it
// among the server's tasks.
type Task struct {
	// Kind is the job kind it runs, which is also its task ID.
	Kind, Name, Description string
	// Interval is how often it runs; 0 when it runs on demand only.
	Interval time.Duration
}

// Tasks returns the recurring jobs with the intervals the settings give,
// as Bazarr's tasks.
func (r *Runtime) Tasks() []Task {
	s := r.settings()
	upgrade := s.UpgradeInterval
	if !s.Upgrade {
		upgrade = 0
	}
	return []Task{
		{
			Kind: JobWantedMovies, Name: "Search for missing movies subtitles", Interval: s.WantedSearchIntervalMovie,
			Description: "Searches the subtitle providers for the subtitles movies are missing.",
		},
		{
			Kind: JobWantedEpisodes, Name: "Search for missing series subtitles", Interval: s.WantedSearchInterval,
			Description: "Searches the subtitle providers for the subtitles episodes are missing.",
		},
		{
			Kind: JobUpgrade, Name: "Upgrade previously downloaded subtitles", Interval: upgrade,
			Description: "Searches for better subtitles than those downloaded lately below the best score.",
		},
	}
}

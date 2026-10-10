package rpc

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/backup"
	"github.com/mavioai/mavio/apps/server/internal/plugins"
	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
	"github.com/mavioai/mavio/libs/metadata"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

var jobStates = map[core.JobState]systemv1.JobState{
	core.JobPending: systemv1.JobState_JOB_STATE_PENDING, core.JobRunning: systemv1.JobState_JOB_STATE_RUNNING,
	core.JobSucceeded: systemv1.JobState_JOB_STATE_SUCCEEDED, core.JobFailed: systemv1.JobState_JOB_STATE_FAILED,
}

func jobToProto(j *core.Job) *systemv1.Job {
	out := systemv1.Job_builder{
		Id: new(j.ID.String()), Kind: &j.Kind, State: new(jobStates[j.State]), Priority: new(int32(j.Priority)),
		Attempts: new(int32(j.Attempts)), MaxAttempts: new(int32(j.MaxAttempts)), RunTime: timestamppb.New(j.RunAt),
		CreateTime: timestamppb.New(j.CreatedAt), LastError: &j.LastError, PayloadJson: new(string(j.Payload)),
	}.Build()
	if j.FinishedAt != nil {
		out.SetFinishTime(timestamppb.New(*j.FinishedAt))
	}
	return out
}

// TaskService implements mavio.system.v1.TaskService.
type TaskService struct {
	store core.Store
	now   func() time.Time
	// Plugins lists the plugins' tasks; nil lists none.
	Plugins func() []plugins.Task
}

var _ systemv1connect.TaskServiceHandler = (*TaskService)(nil)

// NewTaskService returns a TaskService backed by store.
func NewTaskService(store core.Store) *TaskService { return &TaskService{store: store, now: time.Now} }

// task is a recurring task with how to find and queue its runs.
type task struct {
	proto *systemv1.Task
	// matches reports whether a job is a run of the task.
	matches func(j *core.Job) bool
	// run returns the job running the task now.
	run func(now time.Time) core.Job
}

func (s *TaskService) tasks(ctx context.Context) ([]task, error) {
	libs, err := s.store.Libraries().List(ctx)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(libs, func(a, b core.Library) int { return cmp.Compare(a.Name, b.Name) })
	var out []task
	for _, lib := range libs {
		if lib.Kind.Curated() {
			continue
		}
		t := systemv1.Task_builder{
			Id: new(library.JobScan + ":" + lib.ID.String()), Name: new("Scan " + lib.Name), Kind: new(library.JobScan),
			Description: new("Finds what was added, changed or removed in the library's folders."),
		}.Build()
		if lib.ScanInterval > 0 {
			t.SetInterval(durationpb.New(lib.ScanInterval))
		}
		out = append(out, task{
			proto: t,
			matches: func(j *core.Job) bool {
				var p library.LibraryPayload
				return j.Kind == library.JobScan && json.Unmarshal(j.Payload, &p) == nil && p.LibraryID == lib.ID
			},
			run: func(now time.Time) core.Job { return library.ScanJob(lib, now, false) },
		})
		if lib.ExtractTrickplay || lib.ExtractChapterImages || lib.AnalyzeLoudness {
			out = append(out, task{
				proto: systemv1.Task_builder{
					Id: new(library.JobExtras + ":" + lib.ID.String()), Name: new("Make media extras of " + lib.Name),
					Kind:        new(library.JobExtras),
					Description: new("Makes the trickplay sheets, chapter images and loudness measurements the library asks for, of every item."),
				}.Build(),
				matches: func(j *core.Job) bool {
					var p library.LibraryPayload
					return j.Kind == library.JobExtras && json.Unmarshal(j.Payload, &p) == nil && p.LibraryID == lib.ID
				},
				run: func(now time.Time) core.Job { return library.ExtrasJob(lib, now) },
			})
		}
	}
	if s.Plugins != nil {
		for _, pt := range s.Plugins() {
			t := systemv1.Task_builder{
				Id: new(plugins.TaskID(pt.Plugin, pt.Task.GetId())), Name: new(pt.Task.GetName()), Kind: new(plugins.JobTask),
				Description: new(pt.Task.GetDescription()), PluginId: new(pt.Plugin),
			}.Build()
			if pt.Task.HasInterval() {
				t.SetInterval(pt.Task.GetInterval())
			}
			out = append(out, task{
				proto: t,
				matches: func(j *core.Job) bool {
					var p plugins.TaskPayload
					return j.Kind == plugins.JobTask && json.Unmarshal(j.Payload, &p) == nil && p.Plugin == pt.Plugin && p.Task == pt.Task.GetId()
				},
				run: func(now time.Time) core.Job { return plugins.TaskJob(pt.Plugin, pt.Task.GetId(), now, false) },
			})
		}
	}
	out = append(out, task{
		proto: systemv1.Task_builder{
			Id: new(library.JobCleanup), Name: new("Clean up jobs"), Kind: new(library.JobCleanup),
			Description: new("Deletes the jobs that finished more than a week ago."), Interval: durationpb.New(library.CleanupInterval),
		}.Build(),
		matches: func(j *core.Job) bool { return j.Kind == library.JobCleanup },
		run:     func(now time.Time) core.Job { return library.CleanupJob(now, false) },
	})
	return out, nil
}

// ListTasks lists the tasks with their last and next runs.
func (s *TaskService) ListTasks(ctx context.Context, _ *systemv1.ListTasksRequest) (*systemv1.ListTasksResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	tasks, err := s.tasks(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	kinds := []string{library.JobScan, library.JobCleanup, library.JobExtras, plugins.JobTask}
	active, err := s.store.Jobs().List(ctx, core.JobQuery{Kinds: kinds, States: []core.JobState{core.JobPending, core.JobRunning}})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	finished, err := s.store.Jobs().List(ctx, core.JobQuery{Kinds: kinds, States: []core.JobState{core.JobSucceeded, core.JobFailed}, Limit: 500})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*systemv1.Task, len(tasks))
	for i, t := range tasks {
		out[i] = t.proto
		var next *time.Time
		for j := range active.Items {
			job := &active.Items[j]
			if !t.matches(job) {
				continue
			}
			if job.State == core.JobRunning {
				out[i].SetRunning(true)
			} else if next == nil || job.RunAt.Before(*next) {
				next = &job.RunAt
			}
		}
		if next != nil {
			out[i].SetNextRunTime(timestamppb.New(*next))
		}
		var last *core.Job
		for j := range finished.Items {
			job := &finished.Items[j]
			if t.matches(job) && job.FinishedAt != nil && (last == nil || job.FinishedAt.After(*last.FinishedAt)) {
				last = job
			}
		}
		if last != nil {
			out[i].SetLastRun(jobToProto(last))
		}
	}
	return systemv1.ListTasksResponse_builder{Tasks: out}.Build(), nil
}

// RunTask queues a task now.
func (s *TaskService) RunTask(ctx context.Context, req *systemv1.RunTaskRequest) (*systemv1.RunTaskResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	tasks, err := s.tasks(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	i := slices.IndexFunc(tasks, func(t task) bool { return t.proto.GetId() == req.GetId() })
	if i < 0 {
		return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no task %s", req.GetId()))
	}
	job := tasks[i].run(s.now())
	added, err := s.store.Jobs().Enqueue(ctx, &job)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return systemv1.RunTaskResponse_builder{JobId: new(job.ID.String()), Enqueued: &added}.Build(), nil
}

// ListJobs lists the job queue.
func (s *TaskService) ListJobs(ctx context.Context, req *systemv1.ListJobsRequest) (*systemv1.ListJobsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := core.JobQuery{Kinds: req.GetKinds(), Limit: cmp.Or(int(req.GetLimit()), 100), Offset: int(req.GetOffset())}
	for _, st := range req.GetStates() {
		for k, v := range jobStates {
			if v == st {
				q.States = append(q.States, k)
			}
		}
	}
	page, err := s.store.Jobs().List(ctx, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*systemv1.Job, len(page.Items))
	for i := range page.Items {
		out[i] = jobToProto(&page.Items[i])
	}
	return systemv1.ListJobsResponse_builder{Jobs: out, Total: new(int32(page.Total))}.Build(), nil
}

var severities = map[core.Severity]systemv1.Severity{
	core.SeverityInfo: systemv1.Severity_SEVERITY_INFO, core.SeverityWarning: systemv1.Severity_SEVERITY_WARNING,
	core.SeverityError: systemv1.Severity_SEVERITY_ERROR,
}

// ActivityService implements mavio.system.v1.ActivityService.
type ActivityService struct{ store core.Store }

var _ systemv1connect.ActivityServiceHandler = (*ActivityService)(nil)

// NewActivityService returns an ActivityService backed by store.
func NewActivityService(store core.Store) *ActivityService { return &ActivityService{store: store} }

// ListActivities lists the activity log.
func (s *ActivityService) ListActivities(ctx context.Context, req *systemv1.ListActivitiesRequest) (*systemv1.ListActivitiesResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	q := core.ActivityQuery{Limit: cmp.Or(int(req.GetLimit()), 100), Offset: int(req.GetOffset())}
	if req.HasSince() {
		q.Since = req.GetSince().AsTime()
	}
	if req.GetUserId() != "" {
		q.UserID = core.MustParseID(req.GetUserId())
	}
	for k, v := range severities {
		if v == req.GetMinSeverity() {
			q.MinSeverity = k
		}
	}
	page, err := s.store.Activities().List(ctx, q)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*systemv1.Activity, len(page.Items))
	for i, a := range page.Items {
		out[i] = systemv1.Activity_builder{
			Id: new(a.ID.String()), Time: timestamppb.New(a.Time), Type: &a.Type, Severity: new(severities[a.Severity]),
			Title: &a.Title, Message: &a.Message, Attributes: a.Attributes,
		}.Build()
		if !a.UserID.IsZero() {
			out[i].SetUserId(a.UserID.String())
		}
		if !a.ItemID.IsZero() {
			out[i].SetItemId(a.ItemID.String())
		}
	}
	return systemv1.ListActivitiesResponse_builder{Activities: out, Total: new(int32(page.Total))}.Build(), nil
}

// BackupService implements mavio.system.v1.BackupService.
type BackupService struct {
	backups  *backup.Manager
	activity *activity.Log
}

var _ systemv1connect.BackupServiceHandler = (*BackupService)(nil)

// NewBackupService returns a BackupService; a nil manager makes no
// backups.
func NewBackupService(backups *backup.Manager, log *activity.Log) *BackupService {
	return &BackupService{backups: backups, activity: log}
}

func backupToProto(b *backup.Backup) *systemv1.Backup {
	return systemv1.Backup_builder{
		Name: &b.Name, SizeBytes: &b.Size, CreateTime: timestamppb.New(b.Created), Version: &b.Version,
	}.Build()
}

func (s *BackupService) available() error {
	if s.backups == nil {
		return connect.NewError(connect.CodeFailedPrecondition, fmt.Errorf("this server makes no backups"))
	}
	return nil
}

// CreateBackup backs the server up.
func (s *BackupService) CreateBackup(ctx context.Context, _ *systemv1.CreateBackupRequest) (*systemv1.CreateBackupResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.available(); err != nil {
		return nil, err
	}
	b, err := s.backups.Create(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	s.activity.Record(ctx, core.Activity{
		Type: "backup.created", UserID: p.User.ID, Title: p.User.Name + " backed the server up", Attributes: map[string]string{"backup": b.Name},
	})
	return systemv1.CreateBackupResponse_builder{Backup: backupToProto(&b)}.Build(), nil
}

// ListBackups lists the backups.
func (s *BackupService) ListBackups(ctx context.Context, _ *systemv1.ListBackupsRequest) (*systemv1.ListBackupsResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if s.backups == nil {
		return &systemv1.ListBackupsResponse{}, nil
	}
	list, err := s.backups.List()
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*systemv1.Backup, len(list))
	for i := range list {
		out[i] = backupToProto(&list[i])
	}
	return systemv1.ListBackupsResponse_builder{Backups: out}.Build(), nil
}

// DeleteBackup deletes a backup.
func (s *BackupService) DeleteBackup(ctx context.Context, req *systemv1.DeleteBackupRequest) (*systemv1.DeleteBackupResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	if err := s.available(); err != nil {
		return nil, err
	}
	if err := s.backups.Delete(req.GetName()); err != nil {
		return nil, connectError(ctx, err)
	}
	return &systemv1.DeleteBackupResponse{}, nil
}

// LocalizationService implements mavio.system.v1.LocalizationService.
type LocalizationService struct{}

var _ systemv1connect.LocalizationServiceHandler = LocalizationService{}

// ListCountries lists the countries by name.
func (LocalizationService) ListCountries(context.Context, *systemv1.ListCountriesRequest) (*systemv1.ListCountriesResponse, error) {
	list := metadata.Countries()
	out := make([]*systemv1.Country, len(list))
	for i, c := range list {
		out[i] = systemv1.Country_builder{Code: &c.Code, ThreeLetterCode: &c.ThreeLetterCode, Name: &c.Name}.Build()
	}
	return systemv1.ListCountriesResponse_builder{Countries: out}.Build(), nil
}

// ListLanguages lists the languages with an ISO 639-1 code, by name.
func (LocalizationService) ListLanguages(context.Context, *systemv1.ListLanguagesRequest) (*systemv1.ListLanguagesResponse, error) {
	var out []*systemv1.Language
	for _, l := range metadata.Languages() {
		if len(l.TwoLetterCode) != 2 {
			continue
		}
		three := l.ThreeLetterCodes[len(l.ThreeLetterCodes)-1] // bibliographic where it differs
		name, _ := metadata.LanguageDisplayName(l.TwoLetterCode)
		out = append(out, systemv1.Language_builder{Code: &l.TwoLetterCode, ThreeLetterCode: &three, Name: &name}.Build())
	}
	slices.SortFunc(out, func(a, b *systemv1.Language) int { return strings.Compare(a.GetName(), b.GetName()) })
	return systemv1.ListLanguagesResponse_builder{Languages: out}.Build(), nil
}

// ListRatings lists a country's ratings, or every country's.
func (LocalizationService) ListRatings(_ context.Context, req *systemv1.ListRatingsRequest) (*systemv1.ListRatingsResponse, error) {
	countries := []string{req.GetCountry()}
	if req.GetCountry() == "" {
		countries = nil
		for _, c := range metadata.Countries() {
			countries = append(countries, c.Code)
		}
	}
	var out []*systemv1.Rating
	seen := map[string]bool{}
	for _, c := range countries {
		for _, r := range metadata.ParentalRatings(c) {
			if r.Score == nil || seen[r.Name] {
				continue
			}
			seen[r.Name] = true
			out = append(out, systemv1.Rating_builder{Name: &r.Name, Score: new(int32(*r.Score))}.Build())
		}
	}
	slices.SortStableFunc(out, func(a, b *systemv1.Rating) int {
		return cmp.Or(cmp.Compare(a.GetScore(), b.GetScore()), cmp.Compare(a.GetName(), b.GetName()))
	})
	return systemv1.ListRatingsResponse_builder{Ratings: out}.Build(), nil
}

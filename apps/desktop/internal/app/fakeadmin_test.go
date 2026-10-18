package app

import (
	"context"
	"net/http"
	"slices"
	"time"

	"google.golang.org/protobuf/types/known/durationpb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1/authv1connect"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
	storagev1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/storage/v1/storagev1connect"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

// fakeSystem serves the server's system information, health and logs.
type fakeSystem struct {
	systemv1connect.UnimplementedSystemServiceHandler
	f *fakeCore
}

// fakeTasks serves the server's tasks and jobs, whose runs it counts.
type fakeTasks struct {
	systemv1connect.UnimplementedTaskServiceHandler
	f *fakeCore
}

// fakeActivity serves the server's activity log.
type fakeActivity struct {
	systemv1connect.UnimplementedActivityServiceHandler
	f *fakeCore
}

// fakeLibraries serves the server's libraries, whose scans it counts.
type fakeLibraries struct {
	libraryv1connect.UnimplementedLibraryServiceHandler
	f *fakeCore
}

// handleAdmin adds the handlers of the server's administration to mux.
func (f *fakeCore) handleAdmin(mux *http.ServeMux) {
	mux.Handle(systemv1connect.NewSystemServiceHandler(&fakeSystem{f: f}))
	mux.Handle(systemv1connect.NewTaskServiceHandler(&fakeTasks{f: f}))
	mux.Handle(systemv1connect.NewActivityServiceHandler(&fakeActivity{f: f}))
	mux.Handle(libraryv1connect.NewLibraryServiceHandler(&fakeLibraries{f: f}))
	mux.Handle(storagev1connect.NewStorageServiceHandler(&fakeStorage{f: f}))
	mux.Handle(systemv1connect.NewPluginManagementServiceHandler(fakePlugins{}))
	mux.Handle(authv1connect.NewAuthServiceHandler(&fakeAuth{f: f}))
	mux.Handle(systemv1connect.NewProxyServiceHandler(&fakeProxies{f: f}))
	f.libraries = fakeLibraryList()
	f.serverSettings = fakeServerSettings()
	f.apiKeys = fakeApiKeyList()
	f.proxies = fakeProxyList()
	f.remotes = []*storagev1.Remote{storagev1.Remote_builder{Name: new("drive"), Type: new("drive")}.Build()}
}

// tsAgo returns a time a length of time before now, as a timestamp.
func tsAgo(d time.Duration) *timestamppb.Timestamp { return timestamppb.New(time.Now().Add(-d)) }

func (s *fakeSystem) GetHealth(context.Context, *systemv1.GetHealthRequest) (*systemv1.GetHealthResponse, error) {
	return systemv1.GetHealthResponse_builder{Status: new(systemv1.GetHealthResponse_STATUS_SERVING), Version: new("0.9.0")}.Build(), nil
}

func (s *fakeSystem) GetSystemInfo(context.Context, *systemv1.GetSystemInfoRequest) (*systemv1.GetSystemInfoResponse, error) {
	return systemv1.GetSystemInfoResponse_builder{
		Version: new("0.9.0"), Os: new("linux"), Arch: new("amd64"), Database: new("sqlite"),
		FfmpegVersion: new("7.1.2-Jellyfin"), StartTime: tsAgo(26*time.Hour + 5*time.Minute),
	}.Build(), nil
}

func (s *fakeSystem) ListLogs(context.Context, *systemv1.ListLogsRequest) (*systemv1.ListLogsResponse, error) {
	rec := func(d time.Duration, l systemv1.LogLevel, msg string, attrs map[string]string) *systemv1.LogRecord {
		return systemv1.LogRecord_builder{Time: tsAgo(d), Level: &l, Message: &msg, Attributes: attrs}.Build()
	}
	return systemv1.ListLogsResponse_builder{Records: []*systemv1.LogRecord{
		rec(2*time.Minute, systemv1.LogLevel_LOG_LEVEL_INFO, "scan finished", map[string]string{"library": "Movies", "items": "412"}),
		rec(9*time.Minute, systemv1.LogLevel_LOG_LEVEL_WARN, "probe slow", map[string]string{"path": "/media/movies/The Long Orbit (2024)/The Long Orbit.mkv"}),
		rec(40*time.Minute, systemv1.LogLevel_LOG_LEVEL_ERROR, "fetch metadata", map[string]string{"provider": "tmdb", "error": "context deadline exceeded"}),
		rec(3*time.Hour, systemv1.LogLevel_LOG_LEVEL_INFO, "server started", map[string]string{"addr": ":8096"}),
	}}.Build(), nil
}

func (t *fakeTasks) ListTasks(context.Context, *systemv1.ListTasksRequest) (*systemv1.ListTasksResponse, error) {
	t.f.mu.Lock()
	defer t.f.mu.Unlock()
	return systemv1.ListTasksResponse_builder{Tasks: []*systemv1.Task{
		systemv1.Task_builder{
			Id: new("library.scan"), Name: new("Scan Media Libraries"), Description: new("Finds new, changed and removed media in every library."),
			Interval: durationpb.New(12 * time.Hour), NextRunTime: timestamppb.New(time.Now().Add(3*time.Hour + 10*time.Minute)),
			LastRun: systemv1.Job_builder{State: new(systemv1.JobState_JOB_STATE_SUCCEEDED), FinishTime: tsAgo(2 * time.Hour)}.Build(),
		}.Build(),
		systemv1.Task_builder{
			Id: new("trickplay"), Name: new("Extract Trickplay Images"), Description: new("Makes the scrubbing previews of videos."),
			Interval: durationpb.New(24 * time.Hour), Running: new(t.f.taskRuns > 0),
			LastRun: systemv1.Job_builder{State: new(systemv1.JobState_JOB_STATE_FAILED), FinishTime: tsAgo(5 * time.Hour), LastError: new("ffmpeg exited with status 1")}.Build(),
		}.Build(),
		systemv1.Task_builder{Id: new("cache.clean"), Name: new("Clean the Cache"), Description: new("Removes old images and transcodes.")}.Build(),
		systemv1.Task_builder{
			Id: new("subtitles.search"), Name: new("Search for Missing Subtitles"), PluginId: new("subtitles"),
			Interval: durationpb.New(6 * time.Hour), NextRunTime: timestamppb.New(time.Now().Add(40 * time.Minute)),
		}.Build(),
	}}.Build(), nil
}

func (t *fakeTasks) RunTask(context.Context, *systemv1.RunTaskRequest) (*systemv1.RunTaskResponse, error) {
	t.f.mu.Lock()
	defer t.f.mu.Unlock()
	t.f.taskRuns++
	return systemv1.RunTaskResponse_builder{JobId: new("j-run"), Enqueued: new(true)}.Build(), nil
}

func (t *fakeTasks) ListJobs(context.Context, *systemv1.ListJobsRequest) (*systemv1.ListJobsResponse, error) {
	jobs := []*systemv1.Job{
		systemv1.Job_builder{Id: new("j1"), Kind: new("library.scan"), State: new(systemv1.JobState_JOB_STATE_RUNNING), Progress: new(0.42), RunTime: tsAgo(time.Minute), Attempts: new(int32(1)), MaxAttempts: new(int32(3))}.Build(),
		systemv1.Job_builder{Id: new("j2"), Kind: new("trickplay.extract"), State: new(systemv1.JobState_JOB_STATE_FAILED), FinishTime: tsAgo(5 * time.Hour), LastError: new("ffmpeg exited with status 1"), Attempts: new(int32(3)), MaxAttempts: new(int32(3))}.Build(),
		systemv1.Job_builder{Id: new("j3"), Kind: new("metadata.refresh"), State: new(systemv1.JobState_JOB_STATE_SUCCEEDED), FinishTime: tsAgo(2 * time.Hour), Attempts: new(int32(1)), MaxAttempts: new(int32(3))}.Build(),
	}
	return systemv1.ListJobsResponse_builder{Jobs: jobs, Total: new(int32(len(jobs)))}.Build(), nil
}

func (a *fakeActivity) ListActivities(context.Context, *systemv1.ListActivitiesRequest) (*systemv1.ListActivitiesResponse, error) {
	act := func(id string, d time.Duration, s systemv1.Severity, title, msg string) *systemv1.Activity {
		return systemv1.Activity_builder{Id: &id, Time: tsAgo(d), Severity: &s, Title: &title, Message: &msg}.Build()
	}
	list := []*systemv1.Activity{
		act("a1", 3*time.Minute, systemv1.Severity_SEVERITY_INFO, "Anna started playing The Long Orbit", "Living Room TV"),
		act("a2", 40*time.Minute, systemv1.Severity_SEVERITY_ERROR, "Metadata could not be fetched", "TMDB did not answer in time."),
		act("a3", 3*time.Hour, systemv1.Severity_SEVERITY_WARNING, "Sign-in failed for Guest", "From 192.168.1.23"),
		act("a4", 26*time.Hour, systemv1.Severity_SEVERITY_INFO, "Anna signed in", "Mavio for Android TV"),
	}
	return systemv1.ListActivitiesResponse_builder{Activities: list, Total: new(int32(30))}.Build(), nil
}

func (l *fakeLibraries) ListLibraries(context.Context, *libraryv1.ListLibrariesRequest) (*libraryv1.ListLibrariesResponse, error) {
	l.f.mu.Lock()
	defer l.f.mu.Unlock()
	return libraryv1.ListLibrariesResponse_builder{Libraries: slices.Clone(l.f.libraries)}.Build(), nil
}

func (l *fakeLibraries) ScanLibrary(context.Context, *libraryv1.ScanLibraryRequest) (*libraryv1.ScanLibraryResponse, error) {
	l.f.mu.Lock()
	defer l.f.mu.Unlock()
	l.f.scans++
	return libraryv1.ScanLibraryResponse_builder{JobId: new("j-scan"), Enqueued: new(true)}.Build(), nil
}

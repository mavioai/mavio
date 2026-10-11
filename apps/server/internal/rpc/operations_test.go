package rpc

import (
	"net/http"
	"testing"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/acquisition"
	"github.com/mavioai/mavio/apps/server/internal/requests"
	"github.com/mavioai/mavio/libs/core"
	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1/systemv1connect"
)

func TestTaskServiceAcquisition(t *testing.T) {
	ctx := t.Context()
	db := openStore(t)
	settings := core.DefaultAcquisitionSettings()
	settings.RSSSyncInterval, settings.CheckFinishedInterval = 0, 0
	svc := NewTaskService(db)
	svc.Acquisition = &acquisition.Runtime{Store: db, Settings: func() core.AcquisitionSettings { return settings }}
	url := testHandlers(t, func(opts connect.HandlerOption) (string, http.Handler) {
		return systemv1connect.NewTaskServiceHandler(svc, opts)
	})
	admin := systemv1connect.NewTaskServiceClient(http.DefaultClient, url, as("admin"))
	user := systemv1connect.NewTaskServiceClient(http.DefaultClient, url, as("user"))

	resp, err := admin.ListTasks(ctx, &systemv1.ListTasksRequest{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	intervals := map[string]time.Duration{}
	for _, task := range resp.GetTasks() {
		if task.GetKind() != task.GetId() {
			continue
		}
		intervals[task.GetId()] = -1
		if task.HasInterval() {
			intervals[task.GetId()] = task.GetInterval().AsDuration()
		}
	}
	for _, tt := range []struct {
		id   string
		want time.Duration
	}{
		{acquisition.JobRSS, -1}, // RSS sync on demand only
		{acquisition.JobQueue, time.Minute},
		{acquisition.JobRecycleBin, 24 * time.Hour},
		{acquisition.JobSceneMappings, 3 * time.Hour},
	} {
		if got, ok := intervals[tt.id]; !ok || got != tt.want {
			t.Errorf("task %s interval = %v (listed %t), want = %v", tt.id, got, ok, tt.want)
		}
	}

	// Running a task queues its job now, once.
	for i, want := range []bool{true, false} {
		run, err := admin.RunTask(ctx, systemv1.RunTaskRequest_builder{Id: new(acquisition.JobQueue)}.Build())
		if err != nil {
			t.Fatalf("RunTask %d: %v", i, err)
		}
		if run.GetEnqueued() != want {
			t.Errorf("RunTask %d enqueued = %t, want = %t", i, run.GetEnqueued(), want)
		}
	}
	jobs, err := db.Jobs().List(ctx, core.JobQuery{Kinds: []string{acquisition.JobQueue}})
	if err != nil || len(jobs.Items) != 1 || jobs.Items[0].State != core.JobPending {
		t.Errorf("queue check jobs = %+v, %v, want = 1 pending", jobs, err)
	}
	resp, err = admin.ListTasks(ctx, &systemv1.ListTasksRequest{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	for _, task := range resp.GetTasks() {
		if task.GetId() == acquisition.JobQueue && !task.HasNextRunTime() {
			t.Errorf("task %s has no next run after RunTask", task.GetId())
		}
	}
	_, err = user.RunTask(ctx, systemv1.RunTaskRequest_builder{Id: new(acquisition.JobRSS)}.Build())
	wantCode(t, "RunTask as user", err, connect.CodePermissionDenied)

	// Without the runtime, no acquisition task is listed.
	svc.Acquisition = nil
	_, err = admin.RunTask(ctx, systemv1.RunTaskRequest_builder{Id: new(acquisition.JobRSS)}.Build())
	wantCode(t, "RunTask without acquisition", err, connect.CodeNotFound)
}

func TestTaskServiceRequests(t *testing.T) {
	ctx := t.Context()
	db := openStore(t)
	svc := NewTaskService(db)
	url := testHandlers(t, func(opts connect.HandlerOption) (string, http.Handler) {
		return systemv1connect.NewTaskServiceHandler(svc, opts)
	})
	admin := systemv1connect.NewTaskServiceClient(http.DefaultClient, url, as("admin"))
	run := systemv1.RunTaskRequest_builder{Id: new(requests.JobAvailability)}.Build()

	// Without requests, the task is not listed.
	_, err := admin.RunTask(ctx, run)
	wantCode(t, "RunTask without requests", err, connect.CodeNotFound)

	svc.Requests = true
	resp, err := admin.ListTasks(ctx, &systemv1.ListTasksRequest{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	listed := false
	for _, task := range resp.GetTasks() {
		if task.GetId() == requests.JobAvailability {
			listed = true
			if got := task.GetInterval().AsDuration(); got != requests.AvailabilityInterval {
				t.Errorf("availability interval = %v, want = %v", got, requests.AvailabilityInterval)
			}
		}
	}
	if !listed {
		t.Fatalf("tasks = %v, want = %s listed", resp.GetTasks(), requests.JobAvailability)
	}
	got, err := admin.RunTask(ctx, run)
	if err != nil || !got.GetEnqueued() {
		t.Fatalf("RunTask = %v, %v, want = enqueued", got, err)
	}
	jobs, err := db.Jobs().List(ctx, core.JobQuery{Kinds: []string{requests.JobAvailability}})
	if err != nil || len(jobs.Items) != 1 || jobs.Items[0].State != core.JobPending {
		t.Errorf("availability jobs = %+v, %v, want = 1 pending", jobs, err)
	}
}

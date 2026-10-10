package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/plugin/host"
	"github.com/mavioai/mavio/libs/plugin/manifest"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// JobTask runs a plugin's task; the payload is a TaskPayload.
const JobTask = "plugin.task"

// TaskPayload is the payload of JobTask jobs.
type TaskPayload struct {
	Plugin string `json:"plugin"`
	Task   string `json:"task"`
	// Scheduled runs enqueue the next one after the task's interval.
	Scheduled bool `json:"scheduled,omitempty"`
}

// Task is a task of a started plugin.
type Task struct {
	Plugin string
	Task   *pluginv1.Task
}

// TaskID returns the ID TaskService gives a plugin's task.
func TaskID(plugin, task string) string { return "plugin:" + plugin + ":" + task }

// TaskJob returns a job running a plugin's task at the given time.
// Scheduled and requested runs have separate keys, as library scans do.
func TaskJob(plugin, task string, at time.Time, scheduled bool) core.Job {
	payload, _ := json.Marshal(TaskPayload{Plugin: plugin, Task: task, Scheduled: scheduled})
	key := JobTask + ":" + plugin + ":" + task
	if scheduled {
		key += ":scheduled"
	}
	return core.Job{ID: core.NewID(), Kind: JobTask, Payload: payload, UniqueKey: key, MaxAttempts: 1, RunAt: at}
}

// Tasks lists the tasks of the started plugins, by plugin ID.
func (m *Manager) Tasks() []Task {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []Task
	for _, e := range m.plugins {
		if e.plugin == nil {
			continue
		}
		for _, t := range e.manifest.GetTasks() {
			out = append(out, Task{Plugin: e.manifest.GetId(), Task: t})
		}
	}
	return out
}

// task returns a started plugin's task.
func (m *Manager) task(plugin, id string) (*pluginv1.Task, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	e, err := m.find(plugin)
	if err != nil || e.plugin == nil {
		return nil, false
	}
	i := slices.IndexFunc(e.manifest.GetTasks(), func(t *pluginv1.Task) bool { return t.GetId() == id })
	if i < 0 {
		return nil, false
	}
	return e.manifest.GetTasks()[i], true
}

// scheduleTasks enqueues the scheduled run of each of a started plugin's
// tasks that has an interval, unless one is pending; each run enqueues the
// next.
func (m *Manager) scheduleTasks(ctx context.Context, man *pluginv1.Manifest) {
	for _, t := range man.GetTasks() {
		if !t.HasInterval() {
			continue
		}
		job := TaskJob(man.GetId(), t.GetId(), m.now().Add(t.GetInterval().AsDuration()), true)
		if _, err := m.store.Jobs().Enqueue(ctx, &job); err != nil {
			m.log.WarnContext(ctx, "schedule plugin task", "plugin", man.GetId(), "task", t.GetId(), "err", err)
		}
	}
}

// RunTask is the handler of JobTask jobs. Runs of tasks that are gone are
// dropped; a plugin that is not ready fails the run.
func (m *Manager) RunTask(ctx context.Context, job core.Job) ([]core.Job, error) {
	var p TaskPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("task payload: %w", err)
	}
	t, ok := m.task(p.Plugin, p.Task)
	if !ok {
		m.log.InfoContext(ctx, "dropped the run of a task that is gone", "plugin", p.Plugin, "task", p.Task)
		return nil, nil
	}
	var next []core.Job
	if p.Scheduled && t.HasInterval() {
		next = append(next, TaskJob(p.Plugin, p.Task, m.now().Add(t.GetInterval().AsDuration()), true))
	}
	plugin, ok := m.running(p.Plugin)
	switch {
	case !ok && p.Scheduled:
		m.log.InfoContext(ctx, "skipped a scheduled task of a plugin that is not ready", "plugin", p.Plugin, "task", p.Task)
		return next, nil
	case !ok:
		return next, fmt.Errorf("plugin %s is not ready", p.Plugin)
	}
	timeout := manifest.TaskTimeout(t)
	ctx, cancel := context.WithTimeout(host.WithCallTimeout(ctx, timeout), timeout)
	defer cancel()
	req := &pluginv1.RunTaskRequest{}
	req.SetTaskId(p.Task)
	resp, err := plugin.Tasks().RunTask(ctx, req)
	if err != nil {
		return next, fmt.Errorf("plugin %s task %s: %w", p.Plugin, p.Task, err)
	}
	m.log.InfoContext(ctx, "plugin task done", "plugin", p.Plugin, "task", p.Task, "message", resp.GetMessage())
	return next, nil
}

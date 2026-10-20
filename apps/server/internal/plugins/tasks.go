package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
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
	// Locales are the translations of the plugin's strings.
	Locales manifest.Locales
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
			out = append(out, Task{Plugin: e.manifest.GetId(), Task: t, Locales: e.locales})
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
	m.runs.start(p.Plugin, p.Task, job.ID)
	resp, err := plugin.Tasks().RunTask(ctx, req)
	m.runs.end(p.Plugin, p.Task)
	attrs := map[string]string{"task": TaskID(p.Plugin, p.Task), "plugin": p.Plugin}
	if err != nil {
		err = fmt.Errorf("plugin %s task %s: %w", p.Plugin, p.Task, err)
		m.cfg.Activity.Record(ctx, core.Activity{
			Type: "task.failed", Severity: core.SeverityError, Title: t.GetName() + " failed", Message: err.Error(), Attributes: attrs,
		})
		return next, err
	}
	m.log.InfoContext(ctx, "plugin task done", "plugin", p.Plugin, "task", p.Task, "message", resp.GetMessage())
	library.SetResult(ctx, resp.GetMessage())
	m.cfg.Activity.Publish(core.Activity{Type: "task.completed", Title: t.GetName() + " completed", Message: resp.GetMessage(), Attributes: attrs})
	return next, nil
}

// taskRuns are the runs of the plugins' tasks in progress, by plugin and
// task, and how far each has come and what it is doing.
type taskRuns struct {
	mu       sync.Mutex
	jobs     map[[2]string]core.ID
	progress map[core.ID]float64
	status   map[core.ID]string
}

func (r *taskRuns) start(plugin, task string, job core.ID) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.jobs == nil {
		r.jobs, r.progress, r.status = map[[2]string]core.ID{}, map[core.ID]float64{}, map[core.ID]string{}
	}
	r.jobs[[2]string{plugin, task}] = job
}

func (r *taskRuns) end(plugin, task string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	key := [2]string{plugin, task}
	delete(r.progress, r.jobs[key])
	delete(r.status, r.jobs[key])
	delete(r.jobs, key)
}

// ReportTaskProgress keeps how far the run of a plugin's task has come, a
// fraction from 0 to 1, and what it is doing, empty for nothing to tell;
// it reports whether the task is running.
func (m *Manager) ReportTaskProgress(plugin, task string, progress float64, status string) bool {
	m.runs.mu.Lock()
	defer m.runs.mu.Unlock()
	job, ok := m.runs.jobs[[2]string{plugin, task}]
	if ok {
		m.runs.progress[job] = min(max(progress, 0), 1)
		if status == "" {
			delete(m.runs.status, job)
		} else {
			m.runs.status[job] = status
		}
	}
	return ok
}

// JobStatus returns what a running JobTask job is doing, as its plugin
// last reported it.
func (m *Manager) JobStatus(id core.ID) (string, bool) {
	m.runs.mu.Lock()
	defer m.runs.mu.Unlock()
	s, ok := m.runs.status[id]
	return s, ok
}

// taskStarts are the runs the plugins asked of their own tasks
// (StartTask), by plugin and task, until their plugins are ready.
type taskStarts struct {
	mu    sync.Mutex
	asked map[[2]string]bool
	// wake tells queueStarts to look again: a run was asked, or a plugin
	// became ready.
	wake chan struct{}
}

// StartTask asks for a run of one of a plugin's tasks, queued as an
// administrator's run is once the plugin is ready, joining a run already
// waiting. It waits on neither the plugins nor the store, for the plugin
// may ask while it is being configured. A task the plugin's manifest does
// not declare is not found.
func (m *Manager) StartTask(plugin, task string) error {
	man, ok := m.Manifest(plugin)
	if !ok || !slices.ContainsFunc(man.GetTasks(), func(t *pluginv1.Task) bool { return t.GetId() == task }) {
		return fmt.Errorf("plugin %s declares no task %s: %w", plugin, task, core.ErrNotFound)
	}
	m.starts.mu.Lock()
	if m.starts.asked == nil {
		m.starts.asked = map[[2]string]bool{}
	}
	m.starts.asked[[2]string{plugin, task}] = true
	m.starts.mu.Unlock()
	m.wakeStarts()
	return nil
}

func (m *Manager) wakeStarts() {
	select {
	case m.starts.wake <- struct{}{}:
	default:
	}
}

// queueStarts queues the runs asked of the tasks of the plugins that are
// ready, whenever a run is asked or a plugin becomes ready, until the
// Manager closes; a run that cannot be queued is tried again then.
func (m *Manager) queueStarts() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-m.starts.wake:
		}
		m.starts.mu.Lock()
		asked := slices.Collect(maps.Keys(m.starts.asked))
		m.starts.mu.Unlock()
		for _, k := range asked {
			if _, ok := m.running(k[0]); !ok {
				continue
			}
			job := TaskJob(k[0], k[1], m.now(), false)
			if _, err := m.store.Jobs().Enqueue(m.ctx, &job); err != nil {
				m.log.WarnContext(m.ctx, "queue a task the plugin started", "plugin", k[0], "task", k[1], "err", err)
				continue
			}
			m.starts.mu.Lock()
			delete(m.starts.asked, k)
			m.starts.mu.Unlock()
		}
	}
}

// JobProgress returns the fraction done of a running JobTask job, as its
// plugin last reported it.
func (m *Manager) JobProgress(id core.ID) (float64, bool) {
	m.runs.mu.Lock()
	defer m.runs.mu.Unlock()
	f, ok := m.runs.progress[id]
	return f, ok
}

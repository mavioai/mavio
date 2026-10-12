package rclone

import (
	"context"
	"errors"
	"io/fs"
	"sync"
	"testing"

	"github.com/mavioai/mavio/libs/core"
)

// memJobs is a StorageJobRepository in memory.
type memJobs struct {
	mu   sync.Mutex
	jobs map[string]core.StorageJob
}

func (m *memJobs) Get(_ context.Context, kind core.StorageJobKind, key string) (core.StorageJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	j, ok := m.jobs[string(kind)+"/"+key]
	if !ok {
		return core.StorageJob{}, core.ErrNotFound
	}
	return j, nil
}

func (m *memJobs) List(_ context.Context, kind core.StorageJobKind) ([]core.StorageJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []core.StorageJob
	for _, j := range m.jobs {
		if j.Kind == kind {
			out = append(out, j)
		}
	}
	return out, nil
}

func (m *memJobs) Put(_ context.Context, j *core.StorageJob) error {
	if err := j.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.jobs == nil {
		m.jobs = map[string]core.StorageJob{}
	}
	m.jobs[string(j.Kind)+"/"+j.Key] = *j
	return nil
}

func (m *memJobs) Delete(_ context.Context, kind core.StorageJobKind, key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.jobs, string(kind)+"/"+key)
	return nil
}

func (m *memJobs) len() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.jobs)
}

func TestTransferLedger(t *testing.T) {
	d, _ := startDaemon(t)
	ctx := t.Context()
	c := d.Client()
	if err := c.Call(ctx, "operations/mkdir", map[string]any{"fs": "mem:", "remote": "lib"}, nil); err != nil {
		t.Fatal(err)
	}
	jobs := &memJobs{}
	r := &Remotes{Client: c, Jobs: jobs}
	st, err := r.Open(ctx, "mem", "lib")
	if err != nil {
		t.Fatal(err)
	}
	if !st.Capabilities().ServerCopy {
		t.Skip("the memory remote copies no files itself")
	}
	if err := st.WriteFile(ctx, "a.mkv", []byte("abc")); err != nil {
		t.Fatal(err)
	}

	// A transfer the rcd forgot starts again.
	in := map[string]any{"srcFs": "mem:lib", "srcRemote": "a.mkv", "dstFs": "mem:lib", "dstRemote": "b.mkv"}
	lost := core.StorageJob{Kind: core.StorageJobTransfer, Key: transferKey("operations/copyfile", in), Remote: "mem", ExecuteID: "gone", JobID: 9999}
	if err := jobs.Put(ctx, &lost); err != nil {
		t.Fatal(err)
	}
	if err := st.Copy(ctx, "a.mkv", "b.mkv"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Stat(ctx, "b.mkv"); err != nil {
		t.Errorf("Stat(copied) error = %v", err)
	}
	if n := jobs.len(); n != 0 {
		t.Errorf("ledger keeps %d transfers after the copy, want = 0", n)
	}

	// A transfer the rcd finished is not run again: the copy removed
	// since stays removed.
	in["dstRemote"] = "c.mkv"
	job, err := c.Start(ctx, "operations/copyfile", in)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Wait(ctx, job); err != nil {
		t.Fatal(err)
	}
	if err := st.Remove(ctx, "c.mkv"); err != nil {
		t.Fatal(err)
	}
	done := core.StorageJob{Kind: core.StorageJobTransfer, Key: transferKey("operations/copyfile", in), Remote: "mem", ExecuteID: job.ExecuteID, JobID: job.ID}
	if err := jobs.Put(ctx, &done); err != nil {
		t.Fatal(err)
	}
	if err := st.Copy(ctx, "a.mkv", "c.mkv"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Stat(ctx, "c.mkv"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Stat(c.mkv) error = %v, want = %v: the finished copy ran again", err, fs.ErrNotExist)
	}

	// Sweep drops finished and forgotten transfers.
	for _, j := range []core.StorageJob{done, lost} {
		if err := jobs.Put(ctx, &j); err != nil {
			t.Fatal(err)
		}
	}
	if err := r.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if n := jobs.len(); n != 0 {
		t.Errorf("ledger keeps %d transfers after Sweep, want = 0", n)
	}
}

package library

import (
	"context"
	"iter"
	"strings"
	"sync"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// memStore is an in-memory core.Store with what scans use; the store's own
// tests cover the real implementations on SQLite and PostgreSQL.
type memStore struct {
	mu      sync.Mutex
	items   map[core.ID]core.Item
	sources map[core.ID][]core.MediaSource
	folders map[string]core.FolderState
	gens    map[core.ID]int64
	jobs    []core.Job
}

func newMemStore() *memStore {
	return &memStore{items: map[core.ID]core.Item{}, sources: map[core.ID][]core.MediaSource{}, folders: map[string]core.FolderState{}, gens: map[core.ID]int64{}}
}

func (m *memStore) Libraries() core.LibraryRepository                       { panic("unused") }
func (m *memStore) Items() core.ItemRepository                              { return memItems{m} }
func (m *memStore) MediaSources() core.MediaSourceRepository                { return memSources{m} }
func (m *memStore) Images() core.ImageRepository                            { panic("unused") }
func (m *memStore) People() core.PersonRepository                           { panic("unused") }
func (m *memStore) Users() core.UserRepository                              { panic("unused") }
func (m *memStore) UserData() core.UserDataRepository                       { panic("unused") }
func (m *memStore) Jobs() core.JobQueue                                     { return memJobs{m} }
func (m *memStore) Scans() core.ScanRepository                              { return memScans{m} }
func (m *memStore) InTx(_ context.Context, fn func(core.Store) error) error { return fn(m) }

// present returns the items not missing, by path.
func (m *memStore) present() map[string]core.Item {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]core.Item{}
	for _, it := range m.items {
		if it.MissingSince == nil && it.Path != "" {
			out[it.Path] = it
		}
	}
	return out
}

type memItems struct{ m *memStore }

func (r memItems) Get(_ context.Context, id core.ID) (core.Item, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	it, ok := r.m.items[id]
	if !ok {
		return core.Item{}, core.ErrNotFound
	}
	return it, nil
}

func (r memItems) GetByPath(_ context.Context, lib core.ID, path string) (core.Item, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, it := range r.m.items {
		if it.LibraryID == lib && it.Path == path && path != "" {
			return it, nil
		}
	}
	return core.Item{}, core.ErrNotFound
}

func (r memItems) Query(context.Context, core.ItemQuery) (core.Page[core.Item], error) {
	panic("unused")
}

func (r memItems) Walk(context.Context, core.ItemQuery) iter.Seq2[core.Item, error] { panic("unused") }

func (r memItems) Values(context.Context, core.ValueQuery) ([]string, error) { panic("unused") }

func (r memItems) Upsert(_ context.Context, items ...core.Item) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, it := range items {
		if err := it.Validate(); err != nil {
			return err
		}
		r.m.items[it.ID] = it
	}
	return nil
}

func (r memItems) Delete(_ context.Context, ids ...core.ID) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, id := range ids {
		delete(r.m.items, id)
		delete(r.m.sources, id)
		for cid, it := range r.m.items {
			if it.ParentID == id || it.OwnerID == id {
				delete(r.m.items, cid)
			}
		}
	}
	return nil
}

func (r memItems) update(lib core.ID, match func(core.Item) bool, f func(*core.Item)) int {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	n := 0
	for id, it := range r.m.items {
		if it.LibraryID == lib && it.Path != "" && match(it) {
			f(&it)
			r.m.items[id] = it
			n++
		}
	}
	return n
}

func (r memItems) MarkSeen(_ context.Context, lib core.ID, prefix string, gen int64) error {
	prefix = strings.TrimSuffix(prefix, "/")
	r.update(lib, func(it core.Item) bool { return it.Path == prefix || strings.HasPrefix(it.Path, prefix+"/") },
		func(it *core.Item) { it.ScanGeneration, it.MissingSince = gen, nil })
	return nil
}

func (r memItems) Touch(_ context.Context, lib core.ID, gen int64, paths ...string) error {
	set := map[string]bool{}
	for _, p := range paths {
		set[p] = true
	}
	r.update(lib, func(it core.Item) bool { return set[it.Path] }, func(it *core.Item) { it.ScanGeneration, it.MissingSince = gen, nil })
	return nil
}

func (r memItems) MarkMissing(_ context.Context, lib core.ID, gen int64, now time.Time) (int, error) {
	return r.update(lib, func(it core.Item) bool { return it.ScanGeneration < gen && it.MissingSince == nil },
		func(it *core.Item) { t := now; it.MissingSince = &t }), nil
}

func (r memItems) PurgeMissing(ctx context.Context, lib core.ID, before time.Time) ([]core.ID, error) {
	r.m.mu.Lock()
	var ids []core.ID
	for id, it := range r.m.items {
		if it.LibraryID == lib && it.MissingSince != nil && it.MissingSince.Before(before) {
			ids = append(ids, id)
		}
	}
	r.m.mu.Unlock()
	return ids, r.Delete(ctx, ids...)
}

type memSources struct{ m *memStore }

func (r memSources) ListForItem(_ context.Context, id core.ID) ([]core.MediaSource, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	return append([]core.MediaSource(nil), r.m.sources[id]...), nil
}

func (r memSources) Replace(_ context.Context, id core.ID, sources []core.MediaSource) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.m.sources[id] = append([]core.MediaSource(nil), sources...)
	return nil
}

type memScans struct{ m *memStore }

func (r memScans) NextGeneration(_ context.Context, lib core.ID) (int64, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.m.gens[lib]++
	return r.m.gens[lib], nil
}

func (r memScans) Folder(_ context.Context, lib core.ID, path string) (core.FolderState, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	f, ok := r.m.folders[lib.String()+path]
	if !ok {
		return core.FolderState{}, core.ErrNotFound
	}
	return f, nil
}

func (r memScans) PutFolders(_ context.Context, folders ...core.FolderState) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, f := range folders {
		r.m.folders[f.LibraryID.String()+f.Path] = f
	}
	return nil
}

func (r memScans) DeleteFolders(_ context.Context, lib core.ID, before int64) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for k, f := range r.m.folders {
		if f.LibraryID == lib && f.Generation < before {
			delete(r.m.folders, k)
		}
	}
	return nil
}

type memJobs struct{ m *memStore }

func (r memJobs) Enqueue(_ context.Context, j *core.Job) (bool, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, k := range r.m.jobs {
		if j.UniqueKey != "" && k.UniqueKey == j.UniqueKey && k.State != core.JobSucceeded {
			return false, nil
		}
	}
	r.m.jobs = append(r.m.jobs, *j)
	return true, nil
}

func (r memJobs) Lease(context.Context, string, []string, time.Duration) (core.Job, error) {
	panic("unused")
}

func (r memJobs) Extend(context.Context, core.ID, string, time.Duration) error { panic("unused") }

func (r memJobs) Complete(context.Context, core.ID, string) error { panic("unused") }

func (r memJobs) Fail(context.Context, core.ID, string, error) error { panic("unused") }

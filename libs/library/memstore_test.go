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
	libs    map[core.ID]core.Library
	people  map[core.ID]core.Person
	credits map[core.ID][]core.Credit
	images  map[core.ID][]core.Image
	clock   func() time.Time
}

func newMemStore() *memStore {
	return &memStore{
		items: map[core.ID]core.Item{}, sources: map[core.ID][]core.MediaSource{}, folders: map[string]core.FolderState{},
		gens: map[core.ID]int64{}, libs: map[core.ID]core.Library{}, people: map[core.ID]core.Person{},
		credits: map[core.ID][]core.Credit{}, images: map[core.ID][]core.Image{}, clock: time.Now,
	}
}

func (m *memStore) Libraries() core.LibraryRepository                       { return memLibs{m} }
func (m *memStore) Items() core.ItemRepository                              { return memItems{m} }
func (m *memStore) MediaSources() core.MediaSourceRepository                { return memSources{m} }
func (m *memStore) Images() core.ImageRepository                            { return memImages{m} }
func (m *memStore) People() core.PersonRepository                           { return memPeople{m} }
func (m *memStore) Users() core.UserRepository                              { panic("unused") }
func (m *memStore) UserData() core.UserDataRepository                       { panic("unused") }
func (m *memStore) Jobs() core.JobQueue                                     { return memJobs{m} }
func (m *memStore) Scans() core.ScanRepository                              { return memScans{m} }
func (m *memStore) AuthSessions() core.AuthSessionRepository                { panic("unused") }
func (m *memStore) PluginConfigs() core.PluginConfigRepository              { panic("unused") }
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

// byPath returns the item at a path, missing or not.
func (m *memStore) byPath(p string) core.Item {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, it := range m.items {
		if it.Path == p {
			return it
		}
	}
	return core.Item{}
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

func (r memItems) Values(context.Context, core.ValueQuery) ([]core.ValueCount, error) {
	panic("unused")
}

func (r memItems) Upsert(_ context.Context, items ...core.Item) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, it := range items {
		if err := it.Validate(); err != nil {
			return err
		}
		it.FileModified = stored(it.FileModified)
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
	r.update(lib, func(it core.Item) bool {
		return it.MissingSince == nil && (it.Path == prefix || strings.HasPrefix(it.Path, prefix+"/"))
	}, func(it *core.Item) { it.ScanGeneration = gen })
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
	for i := range r.m.sources[id] {
		r.m.sources[id][i].Modified = stored(r.m.sources[id][i].Modified)
	}
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
		f.ModTime = stored(f.ModTime)
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
		active := k.State == "" || k.State == core.JobPending || k.State == core.JobRunning
		if j.UniqueKey != "" && k.UniqueKey == j.UniqueKey && active {
			return false, nil
		}
	}
	r.m.jobs = append(r.m.jobs, *j)
	return true, nil
}

func (r memJobs) Lease(_ context.Context, owner string, kinds []string, ttl time.Duration) (core.Job, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	now := r.m.clock()
	best := -1
	for i, j := range r.m.jobs {
		due := (j.State == "" || j.State == core.JobPending) && !j.RunAt.After(now)
		expired := j.State == core.JobRunning && now.After(j.LeaseExpiresAt)
		if (due || expired) && slicesContains(kinds, j.Kind) && (best < 0 || j.Priority > r.m.jobs[best].Priority) {
			best = i
		}
	}
	if best < 0 {
		return core.Job{}, core.ErrNotFound
	}
	j := &r.m.jobs[best]
	j.State, j.LeaseOwner, j.LeaseExpiresAt = core.JobRunning, owner, now.Add(ttl)
	j.Attempts++
	return *j, nil
}

func (r memJobs) find(id core.ID, owner string) (*core.Job, error) {
	for i := range r.m.jobs {
		if j := &r.m.jobs[i]; j.ID == id {
			if j.State != core.JobRunning || j.LeaseOwner != owner {
				return nil, core.ErrConflict
			}
			return j, nil
		}
	}
	return nil, core.ErrNotFound
}

func (r memJobs) Extend(_ context.Context, id core.ID, owner string, ttl time.Duration) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	j, err := r.find(id, owner)
	if err == nil {
		j.LeaseExpiresAt = r.m.clock().Add(ttl)
	}
	return err
}

func (r memJobs) Complete(_ context.Context, id core.ID, owner string) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	j, err := r.find(id, owner)
	if err == nil {
		j.State = core.JobSucceeded
	}
	return err
}

func (r memJobs) Fail(_ context.Context, id core.ID, owner string, cause error) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	j, err := r.find(id, owner)
	if err != nil {
		return err
	}
	j.LastError = cause.Error()
	if j.Attempts >= j.MaxAttempts {
		j.State = core.JobFailed
	} else {
		j.State, j.RunAt = core.JobPending, r.m.clock().Add(core.RetryDelay(j.Attempts))
	}
	return nil
}

func slicesContains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

type memLibs struct{ m *memStore }

func (r memLibs) Get(_ context.Context, id core.ID) (core.Library, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	l, ok := r.m.libs[id]
	if !ok {
		return core.Library{}, core.ErrNotFound
	}
	return l, nil
}

func (r memLibs) List(context.Context) ([]core.Library, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	var out []core.Library
	for _, l := range r.m.libs {
		out = append(out, l)
	}
	return out, nil
}

func (r memLibs) Create(_ context.Context, l *core.Library) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	if l.ID.IsZero() {
		l.ID = core.NewID()
	}
	r.m.libs[l.ID] = *l
	return nil
}

func (r memLibs) Update(ctx context.Context, l *core.Library) error { return r.Create(ctx, l) }
func (r memLibs) Delete(context.Context, core.ID) error             { panic("unused") }

type memPeople struct{ m *memStore }

func (r memPeople) Get(_ context.Context, id core.ID) (core.Person, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	return r.m.people[id], nil
}

func (r memPeople) FindByName(_ context.Context, name string) (core.Person, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, p := range r.m.people {
		if strings.EqualFold(p.Name, name) {
			return p, nil
		}
	}
	return core.Person{}, core.ErrNotFound
}

func (r memPeople) Upsert(_ context.Context, people ...core.Person) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	for _, p := range people {
		r.m.people[p.ID] = p
	}
	return nil
}

func (r memPeople) Search(context.Context, core.PersonQuery) ([]core.PersonCount, error) {
	panic("unused")
}

func (r memPeople) CreditsForItem(_ context.Context, id core.ID) ([]core.Credit, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	return r.m.credits[id], nil
}

func (r memPeople) ReplaceCredits(_ context.Context, id core.ID, credits []core.Credit) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.m.credits[id] = credits
	return nil
}

type memImages struct{ m *memStore }

func (r memImages) ListForOwner(_ context.Context, id core.ID) ([]core.Image, error) {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	return r.m.images[id], nil
}

func (r memImages) Get(context.Context, core.ID) (core.Image, error) { panic("unused") }

func (r memImages) ListForOwners(context.Context, []core.ID) (map[core.ID][]core.Image, error) {
	panic("unused")
}

func (r memImages) Replace(_ context.Context, id core.ID, images []core.Image) error {
	r.m.mu.Lock()
	defer r.m.mu.Unlock()
	r.m.images[id] = images
	return nil
}

// stored returns a time as libs/store returns it: to the microsecond.
func stored(t time.Time) time.Time { return t.Truncate(time.Microsecond) }

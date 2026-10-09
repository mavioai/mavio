package events

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
)

// Observe returns store with its writes of libraries, items, images, user
// data and jobs reported to the hub, once their transaction committed.
func Observe(store core.Store, hub *Hub) core.Store {
	return &observed{Store: store, hub: hub}
}

type observed struct {
	core.Store
	hub *Hub
	// pending holds the reports of an open transaction; nil outside one.
	pending *[]func()
}

// report reports now, or when the open transaction commits.
func (o *observed) report(f func()) {
	if o.pending != nil {
		*o.pending = append(*o.pending, f)
		return
	}
	f()
}

func (o *observed) InTx(ctx context.Context, fn func(tx core.Store) error) error {
	if o.pending != nil { // already in a transaction
		return o.Store.InTx(ctx, func(tx core.Store) error {
			return fn(&observed{Store: tx, hub: o.hub, pending: o.pending})
		})
	}
	var pending []func()
	err := o.Store.InTx(ctx, func(tx core.Store) error {
		pending = nil
		return fn(&observed{Store: tx, hub: o.hub, pending: &pending})
	})
	if err == nil {
		for _, f := range pending {
			f()
		}
	}
	return err
}

func (o *observed) Libraries() core.LibraryRepository {
	return observedLibraries{o.Store.Libraries(), o}
}
func (o *observed) Items() core.ItemRepository   { return observedItems{o.Store.Items(), o} }
func (o *observed) Images() core.ImageRepository { return observedImages{o.Store.Images(), o} }

func (o *observed) UserData() core.UserDataRepository { return observedUserData{o.Store.UserData(), o} }
func (o *observed) Jobs() core.JobQueue               { return observedJobs{o.Store.Jobs(), o} }

type observedLibraries struct {
	core.LibraryRepository
	o *observed
}

func (r observedLibraries) libraryChanged(id core.ID) {
	r.o.report(func() { r.o.hub.ItemsChanged([]core.ID{id}, nil, nil) })
}

func (r observedLibraries) Create(ctx context.Context, lib *core.Library) error {
	err := r.LibraryRepository.Create(ctx, lib)
	if err == nil {
		r.libraryChanged(lib.ID)
	}
	return err
}

func (r observedLibraries) Update(ctx context.Context, lib *core.Library) error {
	err := r.LibraryRepository.Update(ctx, lib)
	if err == nil {
		r.libraryChanged(lib.ID)
	}
	return err
}

func (r observedLibraries) Delete(ctx context.Context, id core.ID) error {
	err := r.LibraryRepository.Delete(ctx, id)
	if err == nil {
		r.libraryChanged(id)
	}
	return err
}

type observedItems struct {
	core.ItemRepository
	o *observed
}

func (r observedItems) changed(ids ...core.ID) {
	r.o.report(func() { r.o.hub.ItemsChanged(nil, ids, nil) })
}

func (r observedItems) Upsert(ctx context.Context, items ...core.Item) error {
	err := r.ItemRepository.Upsert(ctx, items...)
	if err == nil {
		ids := make([]core.ID, len(items))
		for i := range items {
			ids[i] = items[i].ID
		}
		r.changed(ids...)
	}
	return err
}

func (r observedItems) Delete(ctx context.Context, ids ...core.ID) error {
	// The libraries of deleted items tell who may learn of them.
	removed := map[core.ID]core.ID{}
	for _, id := range ids {
		if it, err := r.Get(ctx, id); err == nil {
			removed[id] = it.LibraryID
		} else if !errors.Is(err, core.ErrNotFound) {
			return err
		}
	}
	err := r.ItemRepository.Delete(ctx, ids...)
	if err == nil {
		r.o.report(func() { r.o.hub.ItemsChanged(nil, nil, removed) })
	}
	return err
}

func (r observedItems) MarkMissing(ctx context.Context, libraryID core.ID, generation int64, now time.Time) (int, error) {
	n, err := r.ItemRepository.MarkMissing(ctx, libraryID, generation, now)
	if err == nil && n > 0 {
		r.o.report(func() { r.o.hub.ItemsChanged([]core.ID{libraryID}, nil, nil) })
	}
	return n, err
}

func (r observedItems) PurgeMissing(ctx context.Context, libraryID core.ID, before time.Time) ([]core.ID, error) {
	ids, err := r.ItemRepository.PurgeMissing(ctx, libraryID, before)
	if err == nil && len(ids) > 0 {
		removed := make(map[core.ID]core.ID, len(ids))
		for _, id := range ids {
			removed[id] = libraryID
		}
		r.o.report(func() { r.o.hub.ItemsChanged(nil, nil, removed) })
	}
	return ids, err
}

func (r observedItems) ReplaceLinks(ctx context.Context, containerID core.ID, links []core.Link) error {
	err := r.ItemRepository.ReplaceLinks(ctx, containerID, links)
	if err == nil {
		r.changed(containerID)
	}
	return err
}

type observedImages struct {
	core.ImageRepository
	o *observed
}

func (r observedImages) Replace(ctx context.Context, ownerID core.ID, images []core.Image) error {
	err := r.ImageRepository.Replace(ctx, ownerID, images)
	if err == nil {
		// People own images too; the hub ignores IDs that are no items.
		r.o.report(func() { r.o.hub.ItemsChanged(nil, []core.ID{ownerID}, nil) })
	}
	return err
}

type observedUserData struct {
	core.UserDataRepository
	o *observed
}

func (r observedUserData) Put(ctx context.Context, d *core.UserData) error {
	err := r.UserDataRepository.Put(ctx, d)
	if err == nil {
		saved := *d
		r.o.report(func() { r.o.hub.UserDataChanged(saved) })
	}
	return err
}

type observedJobs struct {
	core.JobQueue
	o *observed
}

func (q observedJobs) Lease(ctx context.Context, owner string, kinds []string, ttl time.Duration) (core.Job, error) {
	job, err := q.JobQueue.Lease(ctx, owner, kinds, ttl)
	if err == nil {
		q.o.report(func() { q.o.hub.JobLeased(job) })
	}
	return job, err
}

func (q observedJobs) Complete(ctx context.Context, id core.ID, owner string) error {
	err := q.JobQueue.Complete(ctx, id, owner)
	if err == nil {
		q.o.report(func() { q.o.hub.JobEnded(id, false) })
	}
	return err
}

func (q observedJobs) Fail(ctx context.Context, id core.ID, owner string, cause error) error {
	err := q.JobQueue.Fail(ctx, id, owner, cause)
	if err == nil {
		q.o.report(func() { q.o.hub.JobEnded(id, true) })
	}
	return err
}

// libraryOfJob returns the library a scan job scans.
func libraryOfJob(job core.Job) core.ID {
	var p library.LibraryPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return core.NilID
	}
	return p.LibraryID
}

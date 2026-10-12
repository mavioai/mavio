package store

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/storagejob"
)

type storageJobs struct{ s *Store }

// StorageJobs returns the long operations of storage backends the server
// follows.
func (s *Store) StorageJobs() core.StorageJobRepository { return storageJobs{s} }

func storageJobFromEnt(j *ent.StorageJob) core.StorageJob {
	return core.StorageJob{
		Kind: core.StorageJobKind(j.Kind), Key: j.Key, Remote: j.Remote,
		ExecuteID: j.ExecuteID, JobID: j.JobID, Data: j.Data,
		CreatedAt: j.CreatedAt, UpdatedAt: j.UpdatedAt,
	}
}

func (r storageJobs) Get(ctx context.Context, kind core.StorageJobKind, key string) (core.StorageJob, error) {
	j, err := r.s.read.StorageJob.Query().Where(storagejob.Kind(string(kind)), storagejob.Key(key)).Only(ctx)
	if err != nil {
		return core.StorageJob{}, mapErr(err, "get storage job")
	}
	return storageJobFromEnt(j), nil
}

func (r storageJobs) List(ctx context.Context, kind core.StorageJobKind) ([]core.StorageJob, error) {
	jobs, err := r.s.read.StorageJob.Query().
		Where(storagejob.Kind(string(kind))).
		Order(ent.Asc(storagejob.FieldCreatedAt), ent.Asc(storagejob.FieldKey)).
		All(ctx)
	if err != nil {
		return nil, mapErr(err, "list storage jobs")
	}
	out := make([]core.StorageJob, len(jobs))
	for i, j := range jobs {
		out[i] = storageJobFromEnt(j)
	}
	return out, nil
}

func (r storageJobs) Put(ctx context.Context, j *core.StorageJob) error {
	if err := j.Validate(); err != nil {
		return err
	}
	now := time.Now()
	err := r.s.write.StorageJob.Create().
		SetKind(string(j.Kind)).
		SetKey(j.Key).
		SetRemote(j.Remote).
		SetExecuteID(j.ExecuteID).
		SetJobID(j.JobID).
		SetData(j.Data).
		SetCreatedAt(now).
		SetUpdatedAt(now).
		OnConflictColumns(storagejob.FieldKind, storagejob.FieldKey).
		UpdateNewValues().
		Exec(ctx)
	if err != nil {
		return mapErr(err, "put storage job")
	}
	stored, err := r.Get(ctx, j.Kind, j.Key)
	if err != nil {
		return err
	}
	j.CreatedAt, j.UpdatedAt = stored.CreatedAt, stored.UpdatedAt
	return nil
}

func (r storageJobs) Delete(ctx context.Context, kind core.StorageJobKind, key string) error {
	_, err := r.s.write.StorageJob.Delete().Where(storagejob.Kind(string(kind)), storagejob.Key(key)).Exec(ctx)
	return mapErr(err, "delete storage job")
}

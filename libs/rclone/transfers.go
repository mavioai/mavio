package rclone

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"

	"github.com/mavioai/mavio/libs/core"
)

// run runs a transfer, a movefile or copyfile, as a job of the rcd. With
// a ledger the job is recorded while it runs, so that the same transfer
// asked for again, after the server restarted, waits for the job still
// running rather than starting another beside it.
func (r *Remotes) run(ctx context.Context, remote, method string, in map[string]any) error {
	if r.Jobs == nil {
		return r.Client.Run(ctx, method, in)
	}
	key := transferKey(method, in)
	if j, err := r.Jobs.Get(ctx, core.StorageJobTransfer, key); err == nil {
		err := r.Client.Wait(ctx, JobStatus{ID: j.JobID, ExecuteID: j.ExecuteID})
		if ctx.Err() != nil {
			return err
		}
		r.drop(ctx, key)
		if err == nil {
			return nil
		}
		// The job failed, or the rcd forgot it: start the transfer again.
	} else if !errors.Is(err, core.ErrNotFound) {
		r.log().WarnContext(ctx, "read rclone transfer", "method", method, "err", err)
	}

	job, err := r.Client.Start(ctx, method, in)
	if err != nil {
		return err
	}
	rec := core.StorageJob{
		Kind: core.StorageJobTransfer, Key: key, Remote: remote,
		ExecuteID: job.ExecuteID, JobID: job.ID,
		Data: map[string]string{
			"method": method,
			"src":    fmt.Sprint(in["srcFs"], in["srcRemote"]),
			"dst":    fmt.Sprint(in["dstFs"], in["dstRemote"]),
		},
	}
	if err := r.Jobs.Put(ctx, &rec); err != nil {
		r.log().WarnContext(ctx, "record rclone transfer", "method", method, "err", err)
	}
	err = r.Client.Wait(ctx, job)
	if ctx.Err() == nil || errors.Is(err, ErrJobLost) {
		r.drop(ctx, key)
	}
	return err
}

// drop removes a transfer from the ledger.
func (r *Remotes) drop(ctx context.Context, key string) {
	if err := r.Jobs.Delete(context.WithoutCancel(ctx), core.StorageJobTransfer, key); err != nil {
		r.log().WarnContext(ctx, "forget rclone transfer", "err", err)
	}
}

// Sweep removes the transfers of the ledger whose jobs ended, or that the
// rcd forgot; transfers still running are kept for whoever asks for them
// again.
func (r *Remotes) Sweep(ctx context.Context) error {
	if r.Jobs == nil {
		return nil
	}
	jobs, err := r.Jobs.List(ctx, core.StorageJobTransfer)
	if err != nil {
		return err
	}
	for _, j := range jobs {
		st, err := r.Client.Job(ctx, j.JobID)
		switch {
		case jobNotFound(err) || (err == nil && (st.Finished || st.ExecuteID != j.ExecuteID)):
			r.drop(ctx, j.Key)
		case err != nil:
			return err
		}
	}
	return nil
}

// transferKey identifies a transfer by its method, source and target.
func transferKey(method string, in map[string]any) string {
	h := sha256.New()
	for _, v := range []any{method, in["srcFs"], in["srcRemote"], in["dstFs"], in["dstRemote"]} {
		fmt.Fprintf(h, "%v\x00", v)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (r *Remotes) log() *slog.Logger {
	if r.Log == nil {
		return slog.Default()
	}
	return r.Log
}

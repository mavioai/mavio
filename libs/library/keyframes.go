package library

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// JobKeyframes is the kind of jobs that extract the keyframes of an item's
// video files, which HLS needs to cut copied video; the payload is an
// ItemPayload.
const JobKeyframes = "media.keyframes"

// Keyframe job priorities: after scans and probes, unless a playback
// waits for them.
const (
	KeyframesBackground = -10
	KeyframesUrgent     = 20
)

// KeyframeExtractor reads the keyframe times of a video file's first video
// stream, such as with ffprobe.
type KeyframeExtractor interface {
	Keyframes(ctx context.Context, path string) ([]time.Duration, error)
}

// KeyframesJob returns a job extracting the keyframes of an item's video
// files.
func KeyframesJob(itemID core.ID, at time.Time, priority int) core.Job {
	payload, _ := json.Marshal(ItemPayload{ItemID: itemID})
	return core.Job{
		ID: core.NewID(), Kind: JobKeyframes, Payload: payload, UniqueKey: JobKeyframes + ":" + itemID.String(),
		MaxAttempts: 3, RunAt: at, Priority: priority,
	}
}

// NeedsKeyframes reports whether a media source is a probed video file
// without keyframes.
func NeedsKeyframes(src *core.MediaSource) bool {
	return src.Keyframes == nil && !src.ProbedAt.IsZero() && src.Disc == "" &&
		slices.ContainsFunc(src.Streams, func(st core.MediaStream) bool { return st.Kind == core.StreamVideo })
}

func (j *Jobs) keyframes(ctx context.Context, job core.Job) ([]core.Job, error) {
	var p ItemPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("keyframes payload: %w", err)
	}
	sources, err := j.Store.MediaSources().ListForItem(ctx, p.ItemID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	extracted := map[string][]time.Duration{}
	for i := range sources {
		src := &sources[i]
		if !NeedsKeyframes(src) {
			continue
		}
		kf, err := j.Keyframes.Keyframes(ctx, src.Path)
		if err != nil {
			return nil, fmt.Errorf("keyframes of %s: %w", src.Path, err)
		}
		if kf == nil {
			// Stored as extracted, so that the job does not run again.
			kf = []time.Duration{}
		}
		extracted[src.Path] = kf
	}
	if len(extracted) == 0 {
		return nil, nil
	}
	err = j.Store.InTx(ctx, func(tx core.Store) error {
		// Only files still known as they were read keep the result.
		current, err := tx.MediaSources().ListForItem(ctx, p.ItemID)
		if err != nil {
			return err
		}
		for i := range current {
			src := &current[i]
			kf, ok := extracted[src.Path]
			if ok && slices.ContainsFunc(sources, func(s core.MediaSource) bool {
				return s.Path == src.Path && s.Size == src.Size && s.Modified.Equal(src.Modified)
			}) {
				src.Keyframes = kf
			}
		}
		return tx.MediaSources().Replace(ctx, p.ItemID, current)
	})
	return nil, ignoreGone(err)
}

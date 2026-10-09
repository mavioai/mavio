package library

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// JobBorders is the kind of jobs that find the black borders of an item's
// video files, which transcodes crop; the payload is an ItemPayload.
const JobBorders = "media.borders"

// BordersPriority runs border detection after everything else.
const BordersPriority = -20

// BorderDetector finds the black borders of a video file's frames, such as
// by sampling them with ffmpeg.
type BorderDetector interface {
	Borders(ctx context.Context, path string, duration time.Duration, width, height int) (core.Crop, error)
}

// BordersJob returns a job finding the black borders of an item's video
// files.
func BordersJob(itemID core.ID, at time.Time) core.Job {
	payload, _ := json.Marshal(ItemPayload{ItemID: itemID})
	return core.Job{
		ID: core.NewID(), Kind: JobBorders, Payload: payload, UniqueKey: JobBorders + ":" + itemID.String(),
		MaxAttempts: 2, RunAt: at, Priority: BordersPriority,
	}
}

// bordersStream returns the video stream of a probed file whose borders
// were not looked for, nil when there is none.
func bordersStream(src *core.MediaSource) *core.MediaStream {
	if src.ProbedAt.IsZero() || src.Disc != "" || src.Duration <= 0 {
		return nil
	}
	i := slices.IndexFunc(src.Streams, func(st core.MediaStream) bool {
		return st.Kind == core.StreamVideo && st.ExternalPath == "" && st.Width > 0 && st.Height > 0 && !st.IsDolbyVisionEnhancement()
	})
	if i < 0 || src.Streams[i].Crop != nil {
		return nil
	}
	return &src.Streams[i]
}

func (j *Jobs) borders(ctx context.Context, job core.Job) ([]core.Job, error) {
	if j.Borders == nil {
		return nil, nil
	}
	var p ItemPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("borders payload: %w", err)
	}
	sources, err := j.Store.MediaSources().ListForItem(ctx, p.ItemID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	found := map[string]core.Crop{}
	for i := range sources {
		src := &sources[i]
		st := bordersStream(src)
		if st == nil {
			continue
		}
		if j.Scanner != nil && j.Scanner.QuietGate != nil {
			if err := j.Scanner.QuietGate.PauseOrCancel(ctx); err != nil {
				return nil, err
			}
		}
		crop, err := j.Borders.Borders(ctx, src.Path, src.Duration, st.Width, st.Height)
		if err != nil {
			return nil, fmt.Errorf("borders of %s: %w", src.Path, err)
		}
		found[src.Path] = crop
	}
	if len(found) == 0 {
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
			crop, ok := found[src.Path]
			if !ok || !slices.ContainsFunc(sources, func(s core.MediaSource) bool {
				return s.Path == src.Path && s.Size == src.Size && s.Modified.Equal(src.Modified)
			}) {
				continue
			}
			if st := bordersStream(src); st != nil {
				st.Crop = &crop
			}
		}
		return tx.MediaSources().Replace(ctx, p.ItemID, current)
	})
	return nil, ignoreGone(err)
}

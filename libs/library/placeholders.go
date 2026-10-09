package library

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// JobPlaceholders is the kind of jobs that measure an item's images and
// compute their placeholders; the payload is an ItemPayload.
const JobPlaceholders = "image.placeholders"

// ImageFacts are what analyzing an image found.
type ImageFacts struct {
	Width, Height int
	Blurhash      string
	Thumbhash     []byte
}

// ImageAnalyzer reads an image, local or downloaded from its provider, and
// measures it. Images it cannot measure, such as SVGs, give zero facts.
type ImageAnalyzer interface {
	Analyze(ctx context.Context, img core.Image) (ImageFacts, error)
}

// PlaceholdersJob returns a job analyzing an item's images.
func PlaceholdersJob(itemID core.ID, at time.Time) core.Job {
	payload, _ := json.Marshal(ItemPayload{ItemID: itemID})
	return core.Job{
		ID: core.NewID(), Kind: JobPlaceholders, Payload: payload, UniqueKey: JobPlaceholders + ":" + itemID.String(),
		MaxAttempts: 3, RunAt: at, Priority: -5,
	}
}

func (j *Jobs) placeholders(ctx context.Context, job core.Job) ([]core.Job, error) {
	var p ItemPayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("placeholders payload: %w", err)
	}
	images, err := j.Store.Images().ListForOwner(ctx, p.ItemID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	facts := map[core.ID]ImageFacts{}
	for _, img := range images {
		if img.Blurhash != "" {
			continue
		}
		f, err := j.Images.Analyze(ctx, img)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			// One unreadable image, such as a provider's broken link,
			// leaves the others.
			j.logger().WarnContext(ctx, "analyze image", "item", p.ItemID, "image", img.ID, "err", err)
			continue
		}
		if f.Blurhash != "" || f.Width > 0 {
			facts[img.ID] = f
		}
	}
	if len(facts) == 0 {
		return nil, nil
	}
	err = j.Store.InTx(ctx, func(tx core.Store) error {
		// A refresh may have replaced the images meanwhile; only those
		// still there take their facts.
		current, err := tx.Images().ListForOwner(ctx, p.ItemID)
		if err != nil {
			return err
		}
		for i := range current {
			if f, ok := facts[current[i].ID]; ok {
				current[i].Width, current[i].Height, current[i].Blurhash, current[i].Thumbhash = f.Width, f.Height, f.Blurhash, f.Thumbhash
			}
		}
		return tx.Images().Replace(ctx, p.ItemID, current)
	})
	return nil, ignoreGone(err)
}

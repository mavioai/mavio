package acquisition

import (
	"context"
	"fmt"

	"github.com/mavioai/mavio/libs/acquire"
	"github.com/mavioai/mavio/libs/core"
)

// AddDefaultProfiles adds the default quality profiles of movies and of
// series when there are none of the kind, as Radarr's and Sonarr's
// QualityProfileService do on ApplicationStartedEvent: Any, SD, HD-720p,
// HD-1080p, Ultra-HD and HD - 720p/1080p, scoring every custom format 0.
func (r *Runtime) AddDefaultProfiles(ctx context.Context) error {
	stored, err := r.Store.CustomFormats().List(ctx)
	if err != nil {
		return fmt.Errorf("list custom formats: %w", err)
	}
	formats := make([]acquire.CustomFormat, 0, len(stored))
	for _, f := range stored {
		cf, err := acquire.CustomFormatFromCore(f)
		if err != nil {
			return err
		}
		formats = append(formats, cf)
	}
	for _, kind := range []core.ItemKind{core.KindMovie, core.KindSeries} {
		existing, err := r.Store.QualityProfiles().List(ctx, kind)
		if err != nil {
			return fmt.Errorf("list %s quality profiles: %w", kind, err)
		}
		if len(existing) > 0 {
			continue
		}
		k, err := acquire.KindFromCore(kind)
		if err != nil {
			return err
		}
		r.logger().InfoContext(ctx, "setting up default quality profiles", "kind", kind)
		for _, p := range acquire.DefaultProfiles(k, formats) {
			cp, err := acquire.QualityProfileToCore(p, kind)
			if err != nil {
				return err
			}
			cp.ID = core.NewID()
			if err := r.Store.QualityProfiles().Put(ctx, &cp); err != nil {
				return fmt.Errorf("add quality profile %q: %w", cp.Name, err)
			}
		}
	}
	return nil
}

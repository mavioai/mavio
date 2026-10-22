package library

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/mavioai/mavio/libs/core"
)

// JobLocalize fetches an item's name, overview and tagline in a language
// its clients read; the payload is a LocalizePayload.
const JobLocalize = "item.localize"

// LocalizePayload is the payload of JobLocalize jobs.
type LocalizePayload struct {
	ItemID   core.ID `json:"item_id"`
	Language string  `json:"language"`
}

// LocalizeJob returns a job fetching an item's metadata in a language. It
// runs before refreshes, for a client waits for it.
func LocalizeJob(itemID core.ID, language string, at time.Time) core.Job {
	payload, _ := json.Marshal(LocalizePayload{ItemID: itemID, Language: language})
	return core.Job{
		ID: core.NewID(), Kind: JobLocalize, Payload: payload, UniqueKey: JobLocalize + ":" + itemID.String() + ":" + language,
		MaxAttempts: 3, RunAt: at, Priority: 5,
	}
}

// Localizable tells whether items of a kind are kept in the languages
// their clients read: those the providers describe in many languages.
func Localizable(kind core.ItemKind) bool {
	return slices.Contains([]core.ItemKind{core.KindMovie, core.KindSeries, core.KindSeason, core.KindEpisode}, kind)
}

// Localize fetches an item's name, overview and tagline in a language from
// the library's metadata providers and keeps them as its localization,
// the first provider giving each field giving it. Locked fields keep what
// the localization has. Items not following their clients' languages, and
// the language they are kept in, are left alone.
func (r *Refresher) Localize(ctx context.Context, lib core.Library, itemID core.ID, language string) error {
	it, err := r.Store.Items().Get(ctx, itemID)
	if err != nil {
		return err
	}
	if it.Locked || it.Extra != "" || !Localizable(it.Kind) || r.Metadata == nil {
		return nil
	}
	lookup, err := r.lookup(ctx, lib, it)
	if err != nil {
		return err
	}
	var series *core.Item
	if it.Kind == core.KindSeason || it.Kind == core.KindEpisode {
		if series, err = r.seriesOf(ctx, it); err != nil {
			return err
		}
	}
	kept, follows := MetadataLanguage(&it, series, lib, r.metadata())
	if !follows || language == kept {
		return nil
	}
	have, err := r.Store.Localizations().Get(ctx, it.ID, language)
	if err != nil && !errors.Is(err, core.ErrNotFound) {
		return err
	}
	lookup.Language = language
	l := core.ItemLocalization{ItemID: it.ID, Language: language, FetchedAt: r.now()}
	answered := false
	for _, p := range r.providers(lib) {
		res, err := p.Metadata(ctx, lookup)
		switch {
		case errors.Is(err, ErrProviderUnavailable):
			continue
		case err != nil:
			r.logger().WarnContext(ctx, "metadata provider failed", "provider", p.Name(), "item", it.ID, "language", language, "err", err)
			continue
		}
		answered = true
		if res != nil {
			l.Name, l.Overview, l.Tagline = cmp.Or(l.Name, res.Item.Name), cmp.Or(l.Overview, res.Item.Overview), cmp.Or(l.Tagline, res.Item.Tagline)
		}
	}
	if !answered {
		return fmt.Errorf("localize %s in %s: %w", it.ID, language, ErrNoProvider)
	}
	if slices.Contains(it.LockedFields, core.FieldName) {
		l.Name = have.Name
	}
	if slices.Contains(it.LockedFields, core.FieldOverview) {
		l.Overview, l.Tagline = have.Overview, have.Tagline
	}
	return r.Store.Localizations().Put(ctx, &l)
}

// relocalize fetches again the languages an item has, after a refresh.
func (r *Refresher) relocalize(ctx context.Context, lib core.Library, it core.Item) error {
	if !Localizable(it.Kind) {
		return nil
	}
	langs, err := r.Store.Localizations().Languages(ctx, it.ID)
	if err != nil {
		return err
	}
	var errs []error
	for _, lang := range langs {
		errs = append(errs, r.Localize(ctx, lib, it.ID, lang))
	}
	return errors.Join(errs...)
}

// localize runs a JobLocalize job.
func (j *Jobs) localize(ctx context.Context, job core.Job) ([]core.Job, error) {
	var p LocalizePayload
	if err := json.Unmarshal(job.Payload, &p); err != nil {
		return nil, fmt.Errorf("localize payload: %w", err)
	}
	it, err := j.Store.Items().Get(ctx, p.ItemID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	lib, err := j.Store.Libraries().Get(ctx, it.LibraryID)
	if err != nil {
		return nil, ignoreGone(err)
	}
	return nil, ignoreGone(j.Refresher.Localize(ctx, lib, it.ID, p.Language))
}

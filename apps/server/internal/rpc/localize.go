package rpc

import (
	"context"
	"errors"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/library"
)

// clientLanguage returns the language (ISO 639-1) of the caller's UI, the
// first its Accept-Language header names; empty when it names none.
func clientLanguage(ctx context.Context) string {
	tags := wantedLanguages(ctx)
	if len(tags) == 0 {
		return ""
	}
	base, _ := tags[0].Base()
	if code := base.String(); core.ValidLanguage(code) {
		return code
	}
	return ""
}

// localizer presents items in the caller's language: those following
// their clients' languages get their localizations in it, and the items
// lacking one are queued to fetch it (docs/architecture.md §9).
type localizer struct {
	store    core.Store
	language string
	settings core.MetadataSettings
	libs     map[core.ID]*core.Library
}

// newLocalizer returns the caller's localizer; nil when nothing is to be
// localized, as when the server keeps metadata in a language of its own
// or the caller reads the language items are kept in.
func newLocalizer(ctx context.Context, store core.Store) (*localizer, error) {
	lang := clientLanguage(ctx)
	if lang == "" || lang == core.FallbackMetadataLanguage {
		return nil, nil
	}
	settings, err := store.Settings().Get(ctx)
	if err != nil {
		return nil, err
	}
	if !settings.Metadata.FollowsClients() {
		return nil, nil
	}
	return &localizer{store: store, language: lang, settings: settings.Metadata, libs: map[core.ID]*core.Library{}}, nil
}

// follows tells whether an item is presented in the caller's language.
func (l *localizer) follows(ctx context.Context, it *core.Item, anc *ancestry) (bool, error) {
	if l == nil || it.Locked || it.Extra != "" || !library.Localizable(it.Kind) {
		return false, nil
	}
	lib, ok := l.libs[it.LibraryID]
	if !ok {
		got, err := l.store.Libraries().Get(ctx, it.LibraryID)
		switch {
		case errors.Is(err, core.ErrNotFound):
		case err != nil:
			return false, err
		default:
			lib = &got
		}
		l.libs[it.LibraryID] = lib
	}
	if lib == nil {
		return false, nil
	}
	var series *core.Item
	if it.Kind == core.KindSeason || it.Kind == core.KindEpisode {
		var err error
		if series, err = anc.series(ctx, it); err != nil {
			return false, err
		}
	}
	_, follows := library.MetadataLanguage(it, series, *lib, l.settings)
	return follows, nil
}

// apply localizes items and the series and seasons anc holds of them in
// place, and queues fetching the localizations they lack.
func (l *localizer) apply(ctx context.Context, items []*core.Item, anc *ancestry) error {
	if l == nil {
		return nil
	}
	for _, it := range items {
		if it.Kind != core.KindSeason && it.Kind != core.KindEpisode {
			continue
		}
		for id := it.ParentID; !id.IsZero(); {
			p, err := anc.get(ctx, id)
			if err != nil {
				return err
			}
			if p == nil {
				break
			}
			items = append(items, p)
			id = p.ParentID
		}
	}
	var following []*core.Item
	var ids []core.ID
	seen := map[core.ID]bool{}
	for _, it := range items {
		if seen[it.ID] {
			continue
		}
		seen[it.ID] = true
		ok, err := l.follows(ctx, it, anc)
		if err != nil {
			return err
		}
		if ok {
			following, ids = append(following, it), append(ids, it.ID)
		}
	}
	if len(following) == 0 {
		return nil
	}
	have, err := l.store.Localizations().GetMany(ctx, ids, l.language)
	if err != nil {
		return err
	}
	for _, it := range following {
		if loc, ok := have[it.ID]; ok {
			loc.Apply(it)
			continue
		}
		job := library.LocalizeJob(it.ID, l.language, time.Now())
		if _, err := l.store.Jobs().Enqueue(ctx, &job); err != nil {
			return err
		}
	}
	return nil
}

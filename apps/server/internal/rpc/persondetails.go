package rpc

import (
	"cmp"
	"context"
	"log/slog"
	"maps"
	"time"

	"github.com/mavioai/mavio/libs/core"
	pluginv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/plugin/v1"
)

// PersonDetails tells of a person by their external IDs in a language
// (ISO 639-1); nil when no one knows them.
type PersonDetails func(ctx context.Context, ids map[core.Provider]string, language string) (*pluginv1.Person, error)

const (
	// personDetailsAge is how long a person's biography is kept before it
	// is asked for again.
	personDetailsAge = 30 * 24 * time.Hour
	// personDetailsTimeout bounds the wait of the client opening a person
	// whose biography is asked for.
	personDetailsTimeout = 5 * time.Second
)

// fillPerson asks for the biography, dates and headshot of a person known
// by an external ID when they were never asked for or are stale, and keeps
// them. It fails only when the store does: a provider failing leaves the
// person as stored, to be asked for again on their next opening.
func (s *ItemService) fillPerson(ctx context.Context, person *core.Person, images *[]core.Image) error {
	now := time.Now()
	if s.PersonDetails == nil || len(person.ExternalIDs) == 0 ||
		person.DetailsFetchedAt != nil && now.Sub(*person.DetailsFetchedAt) < personDetailsAge {
		return nil
	}
	settings, err := s.store.Settings().Get(ctx)
	if err != nil {
		return err
	}
	language := settings.Metadata.PreferredLanguage
	if language == "" {
		language = cmp.Or(clientLanguage(ctx), core.FallbackMetadataLanguage)
	}
	fetchCtx, cancel := context.WithTimeout(ctx, personDetailsTimeout)
	defer cancel()
	found, err := s.PersonDetails(fetchCtx, person.ExternalIDs, language)
	if err != nil {
		slog.WarnContext(ctx, "fetch person details", "person", person.ID, "error", err)
		return nil
	}
	p := *person
	p.DetailsFetchedAt = &now
	if found != nil {
		p.Overview = cmp.Or(found.GetBiography(), p.Overview)
		p.BirthDate = cmp.Or(calendarDate(found.GetBirthday()), p.BirthDate)
		p.DeathDate = cmp.Or(calendarDate(found.GetDeathday()), p.DeathDate)
		p.BirthPlace = cmp.Or(found.GetPlaceOfBirth(), p.BirthPlace)
		p.ExternalIDs = maps.Clone(p.ExternalIDs)
		for k, v := range found.GetExternalIds() {
			if v != "" {
				p.ExternalIDs[core.Provider(k)] = v
			}
		}
	}
	err = s.store.InTx(ctx, func(tx core.Store) error {
		if err := tx.People().Upsert(ctx, p); err != nil {
			return err
		}
		if found.GetImageUrl() == "" || len(*images) > 0 {
			return nil
		}
		*images = []core.Image{{ID: core.NewID(), OwnerID: p.ID, Kind: core.ImagePrimary, RemoteURL: found.GetImageUrl()}}
		return tx.Images().Replace(ctx, p.ID, *images)
	})
	if err != nil {
		return err
	}
	*person = p
	return nil
}

// calendarDate parses an ISO 8601 calendar date; nil when empty or
// malformed.
func calendarDate(s string) *time.Time {
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil
	}
	return &t
}

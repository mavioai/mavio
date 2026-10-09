package store

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent/displaypreferences"
)

type displayPreferences struct{ s *Store }

// DisplayPreferences returns the users' display preferences.
func (s *Store) DisplayPreferences() core.DisplayPreferencesRepository {
	return displayPreferences{s}
}

func (r displayPreferences) Get(ctx context.Context, userID core.ID, client, view string) (core.DisplayPreferences, error) {
	p, err := r.s.read.DisplayPreferences.Query().
		Where(displaypreferences.UserID(userID), displaypreferences.Client(client), displaypreferences.View(view)).
		Only(ctx)
	if err != nil {
		return core.DisplayPreferences{}, mapErr(err, "get display preferences")
	}
	return core.DisplayPreferences{UserID: p.UserID, Client: p.Client, View: p.View, Values: p.Values, UpdatedAt: p.UpdatedAt.UTC()}, nil
}

func (r displayPreferences) Put(ctx context.Context, p *core.DisplayPreferences) error {
	if err := p.Validate(); err != nil {
		return err
	}
	p.UpdatedAt = time.Now().UTC()
	err := r.s.write.DisplayPreferences.Create().
		SetUserID(p.UserID).
		SetClient(p.Client).
		SetView(p.View).
		SetValues(p.Values).
		SetUpdatedAt(p.UpdatedAt).
		OnConflictColumns(displaypreferences.FieldUserID, displaypreferences.FieldClient, displaypreferences.FieldView).
		UpdateNewValues().
		Exec(ctx)
	return mapErr(err, "put display preferences")
}

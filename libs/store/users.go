package store

import (
	"context"
	"strings"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/credential"
	"github.com/mavioai/mavio/libs/store/internal/ent/predicate"
	"github.com/mavioai/mavio/libs/store/internal/ent/user"
	"github.com/mavioai/mavio/libs/store/internal/ent/userdata"
)

type users struct{ s *Store }

func (r users) Get(ctx context.Context, id core.ID) (core.User, error) {
	u, err := r.s.read.User.Get(ctx, id)
	if err != nil {
		return core.User{}, mapErr(err, "get user "+id.String())
	}
	return toUser(u), nil
}

func (r users) GetByName(ctx context.Context, name string) (core.User, error) {
	u, err := r.s.read.User.Query().Where(user.NameKey(strings.ToLower(name))).Only(ctx)
	if err != nil {
		return core.User{}, mapErr(err, "get user "+name)
	}
	return toUser(u), nil
}

func (r users) List(ctx context.Context) ([]core.User, error) {
	list, err := r.s.read.User.Query().Order(ent.Asc(user.FieldNameKey)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list users")
	}
	out := make([]core.User, len(list))
	for i, u := range list {
		out[i] = toUser(u)
	}
	return out, nil
}

func (r users) Create(ctx context.Context, u *core.User) error {
	if u.ID.IsZero() {
		u.ID = core.NewID()
	}
	if err := u.Validate(); err != nil {
		return err
	}
	created, err := r.s.write.User.Create().
		SetID(u.ID).
		SetName(u.Name).
		SetNameKey(strings.ToLower(u.Name)).
		SetKind(u.Kind).
		SetAuthProvider(u.AuthProvider).
		SetRestriction(u.Restriction).
		SetDisabled(u.Disabled).
		SetPolicy(u.Policy).
		SetPreferences(u.Preferences).
		SetCreatedAt(orNow(u.CreatedAt)).
		SetNillableLastLoginAt(u.LastLoginAt).
		Save(ctx)
	if err != nil {
		return mapErr(err, "create user "+u.Name)
	}
	*u = toUser(created)
	return nil
}

func (r users) Update(ctx context.Context, u *core.User) error {
	if err := u.Validate(); err != nil {
		return err
	}
	update := r.s.write.User.UpdateOneID(u.ID).
		SetName(u.Name).
		SetNameKey(strings.ToLower(u.Name)).
		SetKind(u.Kind).
		SetAuthProvider(u.AuthProvider).
		SetRestriction(u.Restriction).
		SetDisabled(u.Disabled).
		SetPolicy(u.Policy).
		SetPreferences(u.Preferences)
	if u.LastLoginAt != nil {
		update.SetLastLoginAt(*u.LastLoginAt)
	} else {
		update.ClearLastLoginAt()
	}
	updated, err := update.Save(ctx)
	if err != nil {
		return mapErr(err, "update user "+u.ID.String())
	}
	*u = toUser(updated)
	return nil
}

func (r users) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.User.DeleteOneID(id).Exec(ctx), "delete user "+id.String())
}

func toUser(u *ent.User) core.User {
	return core.User{
		ID:           u.ID,
		Name:         u.Name,
		Kind:         u.Kind,
		AuthProvider: u.AuthProvider,
		Restriction:  u.Restriction,
		Disabled:     u.Disabled,
		Policy:       u.Policy,
		Preferences:  u.Preferences,
		CreatedAt:    u.CreatedAt.UTC(),
		LastLoginAt:  utcPtr(u.LastLoginAt),
	}
}

type credentials struct{ s *Store }

func (r credentials) List(ctx context.Context, userID core.ID) ([]core.Credential, error) {
	list, err := r.s.read.Credential.Query().Where(credential.UserID(userID)).
		Order(ent.Asc(credential.FieldCreatedAt), ent.Asc(credential.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list credentials")
	}
	out := make([]core.Credential, len(list))
	for i, c := range list {
		out[i] = toCredential(c)
	}
	return out, nil
}

func (r credentials) Get(ctx context.Context, userID core.ID, kind core.CredentialKind) (core.Credential, error) {
	c, err := r.s.read.Credential.Query().Where(credential.UserID(userID), credential.Kind(kind)).First(ctx)
	if err != nil {
		return core.Credential{}, mapErr(err, "get "+string(kind)+" credential")
	}
	return toCredential(c), nil
}

func (r credentials) Put(ctx context.Context, c *core.Credential) error {
	if c.ID.IsZero() {
		c.ID = core.NewID()
	}
	if err := c.Validate(); err != nil {
		return err
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		old := []predicate.Credential{credential.ID(c.ID)}
		if c.Kind.Single() {
			old = append(old, credential.And(credential.UserID(c.UserID), credential.Kind(c.Kind)))
		}
		if _, err := tx.write.Credential.Delete().Where(credential.Or(old...)).Exec(ctx); err != nil {
			return mapErr(err, "replace "+string(c.Kind)+" credential")
		}
		created, err := tx.write.Credential.Create().
			SetID(c.ID).
			SetUserID(c.UserID).
			SetKind(c.Kind).
			SetName(c.Name).
			SetData(c.Data).
			SetCreatedAt(orNow(c.CreatedAt)).
			SetNillableLastUsedAt(c.LastUsedAt).
			Save(ctx)
		if err != nil {
			return mapErr(err, "put "+string(c.Kind)+" credential")
		}
		*c = toCredential(created)
		return nil
	})
}

func (r credentials) Delete(ctx context.Context, userID core.ID, kind core.CredentialKind) error {
	_, err := r.s.write.Credential.Delete().Where(credential.UserID(userID), credential.Kind(kind)).Exec(ctx)
	return mapErr(err, "delete "+string(kind)+" credentials")
}

func (r credentials) Remove(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.Credential.DeleteOneID(id).Exec(ctx), "remove credential "+id.String())
}

func toCredential(c *ent.Credential) core.Credential {
	return core.Credential{
		ID:         c.ID,
		UserID:     c.UserID,
		Kind:       c.Kind,
		Name:       c.Name,
		Data:       c.Data,
		CreatedAt:  c.CreatedAt.UTC(),
		LastUsedAt: utcPtr(c.LastUsedAt),
	}
}

type userData struct{ s *Store }

func (r userData) Get(ctx context.Context, userID, itemID core.ID) (core.UserData, error) {
	d, err := r.s.read.UserData.Query().Where(userdata.UserID(userID), userdata.ItemID(itemID)).Only(ctx)
	if err != nil {
		return core.UserData{}, mapErr(err, "get user data")
	}
	return toUserData(d), nil
}

func (r userData) GetMany(ctx context.Context, userID core.ID, itemIDs []core.ID) (map[core.ID]core.UserData, error) {
	out := make(map[core.ID]core.UserData, len(itemIDs))
	if len(itemIDs) == 0 {
		return out, nil
	}
	list, err := r.s.read.UserData.Query().Where(userdata.UserID(userID), userdata.ItemIDIn(itemIDs...)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "get user data")
	}
	for _, d := range list {
		out[d.ItemID] = toUserData(d)
	}
	return out, nil
}

func (r userData) Put(ctx context.Context, d *core.UserData) error {
	if err := d.Validate(); err != nil {
		return err
	}
	d.UpdatedAt = time.Now()
	err := r.s.write.UserData.Create().
		SetUserID(d.UserID).
		SetItemID(d.ItemID).
		SetPlayed(d.Played).
		SetPlayCount(d.PlayCount).
		SetPosition(d.Position).
		SetNillableAudioStream(d.AudioStream).
		SetNillableSubtitleStream(d.SubtitleStream).
		SetFavorite(d.Favorite).
		SetNillableRating(d.Rating).
		SetNillableLastPlayedAt(d.LastPlayedAt).
		SetUpdatedAt(d.UpdatedAt).
		OnConflictColumns(userdata.FieldUserID, userdata.FieldItemID).
		UpdateNewValues().
		Update(func(u *ent.UserDataUpsert) {
			// Put replaces the state: unset optional fields are cleared,
			// which UpdateNewValues alone leaves as they were.
			if d.AudioStream == nil {
				u.ClearAudioStream()
			}
			if d.SubtitleStream == nil {
				u.ClearSubtitleStream()
			}
			if d.Rating == nil {
				u.ClearRating()
			}
			if d.LastPlayedAt == nil {
				u.ClearLastPlayedAt()
			}
		}).
		Exec(ctx)
	return mapErr(err, "put user data")
}

func toUserData(d *ent.UserData) core.UserData {
	return core.UserData{
		UserID:         d.UserID,
		ItemID:         d.ItemID,
		Played:         d.Played,
		PlayCount:      d.PlayCount,
		Position:       d.Position,
		AudioStream:    d.AudioStream,
		SubtitleStream: d.SubtitleStream,
		Favorite:       d.Favorite,
		Rating:         d.Rating,
		LastPlayedAt:   utcPtr(d.LastPlayedAt),
		UpdatedAt:      d.UpdatedAt.UTC(),
	}
}

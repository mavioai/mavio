package store

import (
	"context"
	"time"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/authsession"
)

type authSessions struct{ s *Store }

// AuthSessions returns the signed-in clients.
func (s *Store) AuthSessions() core.AuthSessionRepository { return authSessions{s} }

func (r authSessions) Create(ctx context.Context, sess *core.AuthSession) error {
	if sess.ID.IsZero() {
		sess.ID = core.NewID()
	}
	if err := sess.Validate(); err != nil {
		return err
	}
	return r.s.writeTx(ctx, func(tx *Store) error {
		_, err := tx.write.AuthSession.Delete().
			Where(authsession.UserID(sess.UserID), authsession.DeviceID(sess.DeviceID)).
			Exec(ctx)
		if err != nil {
			return mapErr(err, "replace auth session")
		}
		now := time.Now()
		created, err := tx.write.AuthSession.Create().
			SetID(sess.ID).
			SetUserID(sess.UserID).
			SetTokenHash(sess.TokenHash).
			SetDeviceID(sess.DeviceID).
			SetDeviceName(sess.DeviceName).
			SetClient(sess.Client).
			SetClientVersion(sess.ClientVersion).
			SetCreatedAt(orNow(sess.CreatedAt)).
			SetLastSeenAt(orTime(sess.LastSeenAt, now)).
			Save(ctx)
		if err != nil {
			return mapErr(err, "create auth session")
		}
		*sess = toAuthSession(created)
		return nil
	})
}

func orTime(t, def time.Time) time.Time {
	if t.IsZero() {
		return def
	}
	return t
}

func (r authSessions) GetByTokenHash(ctx context.Context, hash []byte) (core.AuthSession, error) {
	s, err := r.s.read.AuthSession.Query().Where(authsession.TokenHash(hash)).Only(ctx)
	if err != nil {
		return core.AuthSession{}, mapErr(err, "get auth session")
	}
	return toAuthSession(s), nil
}

func (r authSessions) ListForUser(ctx context.Context, userID core.ID) ([]core.AuthSession, error) {
	list, err := r.s.read.AuthSession.Query().
		Where(authsession.UserID(userID)).
		Order(ent.Desc(authsession.FieldLastSeenAt), ent.Asc(authsession.FieldID)).
		All(ctx)
	if err != nil {
		return nil, mapErr(err, "list auth sessions")
	}
	out := make([]core.AuthSession, len(list))
	for i, s := range list {
		out[i] = toAuthSession(s)
	}
	return out, nil
}

func (r authSessions) Touch(ctx context.Context, id core.ID, at time.Time) error {
	err := r.s.write.AuthSession.UpdateOneID(id).SetLastSeenAt(at).Exec(ctx)
	return mapErr(err, "touch auth session "+id.String())
}

func (r authSessions) Delete(ctx context.Context, id core.ID) error {
	err := r.s.write.AuthSession.DeleteOneID(id).Exec(ctx)
	return mapErr(err, "delete auth session "+id.String())
}

func toAuthSession(s *ent.AuthSession) core.AuthSession {
	return core.AuthSession{
		ID: s.ID, UserID: s.UserID, TokenHash: s.TokenHash,
		DeviceID: s.DeviceID, DeviceName: s.DeviceName, Client: s.Client, ClientVersion: s.ClientVersion,
		CreatedAt: s.CreatedAt.UTC(), LastSeenAt: s.LastSeenAt.UTC(),
	}
}

package store

import (
	"context"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	"github.com/mavioai/mavio/libs/core"
	"github.com/mavioai/mavio/libs/store/internal/ent"
	"github.com/mavioai/mavio/libs/store/internal/ent/invitation"
)

type invitations struct{ s *Store }

// Invitations returns the invitation links.
func (s *Store) Invitations() core.InvitationRepository { return invitations{s} }

func (r invitations) Create(ctx context.Context, inv *core.Invitation) error {
	if inv.ID.IsZero() {
		inv.ID = core.NewID()
	}
	if err := inv.Validate(); err != nil {
		return err
	}
	create := r.s.write.Invitation.Create().
		SetID(inv.ID).
		SetTokenHash(inv.TokenHash).
		SetAccess(inv.Access).
		SetMaxUses(inv.MaxUses).
		SetUses(inv.Uses).
		SetExpiresAt(inv.ExpiresAt).
		SetCreatedAt(orNow(inv.CreatedAt))
	if !inv.UserID.IsZero() {
		create.SetUserID(inv.UserID)
	}
	created, err := create.Save(ctx)
	if err != nil {
		return mapErr(err, "create invitation")
	}
	*inv = toInvitation(created)
	return nil
}

func (r invitations) GetByTokenHash(ctx context.Context, hash []byte) (core.Invitation, error) {
	inv, err := r.s.read.Invitation.Query().Where(invitation.TokenHash(hash)).Only(ctx)
	if err != nil {
		return core.Invitation{}, mapErr(err, "get invitation")
	}
	return toInvitation(inv), nil
}

func (r invitations) List(ctx context.Context) ([]core.Invitation, error) {
	list, err := r.s.read.Invitation.Query().Order(ent.Desc(invitation.FieldCreatedAt), ent.Asc(invitation.FieldID)).All(ctx)
	if err != nil {
		return nil, mapErr(err, "list invitations")
	}
	out := make([]core.Invitation, len(list))
	for i, inv := range list {
		out[i] = toInvitation(inv)
	}
	return out, nil
}

func (r invitations) Use(ctx context.Context, id core.ID, now time.Time) error {
	// One conditional update, so that concurrent uses cannot exceed the
	// limit.
	n, err := r.s.write.Invitation.Update().
		Where(invitation.ID(id), invitation.ExpiresAtGT(now), func(s *entsql.Selector) {
			s.Where(entsql.ColumnsLT(s.C(invitation.FieldUses), s.C(invitation.FieldMaxUses)))
		}).
		AddUses(1).
		Save(ctx)
	if err != nil {
		return mapErr(err, "use invitation")
	}
	if n == 0 {
		return core.ErrNotFound
	}
	return nil
}

func (r invitations) Delete(ctx context.Context, id core.ID) error {
	return mapErr(r.s.write.Invitation.DeleteOneID(id).Exec(ctx), "delete invitation")
}

func (r invitations) PurgeExpired(ctx context.Context, before time.Time) (int, error) {
	n, err := r.s.write.Invitation.Delete().Where(invitation.ExpiresAtLT(before)).Exec(ctx)
	return n, mapErr(err, "purge expired invitations")
}

func toInvitation(inv *ent.Invitation) core.Invitation {
	out := core.Invitation{
		ID: inv.ID, TokenHash: inv.TokenHash, Access: inv.Access,
		MaxUses: inv.MaxUses, Uses: inv.Uses, ExpiresAt: inv.ExpiresAt.UTC(), CreatedAt: inv.CreatedAt.UTC(),
	}
	if inv.UserID != nil {
		out.UserID = *inv.UserID
	}
	return out
}

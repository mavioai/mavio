package rpc

import (
	"context"
	"errors"
	"slices"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/auth"
	"github.com/mavioai/mavio/apps/server/internal/events"
	"github.com/mavioai/mavio/libs/core"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// UserService implements mavio.user.v1.UserService.
type UserService struct {
	store core.Store
}

var _ userv1connect.UserServiceHandler = (*UserService)(nil)

// NewUserService returns a UserService backed by store.
func NewUserService(store core.Store) *UserService { return &UserService{store: store} }

// admin returns the caller when they are an administrator.
func admin(ctx context.Context) (auth.Principal, error) {
	p, err := principal(ctx)
	if err != nil {
		return p, err
	}
	if !p.User.Admin {
		return p, connect.NewError(connect.CodePermissionDenied, errors.New("administrator required"))
	}
	return p, nil
}

// ListUsers lists all users.
func (s *UserService) ListUsers(ctx context.Context, _ *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	users, err := s.store.Users().List(ctx)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*userv1.User, len(users))
	for i := range users {
		out[i] = userToProto(&users[i])
	}
	return userv1.ListUsersResponse_builder{Users: out}.Build(), nil
}

// GetUser returns one user.
func (s *UserService) GetUser(ctx context.Context, req *userv1.GetUserRequest) (*userv1.GetUserResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	u, err := s.store.Users().Get(ctx, core.MustParseID(req.GetId()))
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return userv1.GetUserResponse_builder{User: userToProto(&u)}.Build(), nil
}

// GetCurrentUser returns the caller.
func (s *UserService) GetCurrentUser(ctx context.Context, _ *userv1.GetCurrentUserRequest) (*userv1.GetCurrentUserResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	return userv1.GetCurrentUserResponse_builder{User: userToProto(&p.User)}.Build(), nil
}

// CreateUser creates an account.
func (s *UserService) CreateUser(ctx context.Context, req *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	u := core.User{
		Name:         req.GetName(),
		AuthProvider: req.GetAuthProvider(),
		Admin:        req.GetAdmin(),
		Policy:       core.UserPolicy{AllowTranscoding: true, AllowDownload: true},
	}
	switch {
	case u.AuthProvider == "" && req.GetPassword() == "":
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("a password is required"))
	case u.AuthProvider == "":
		u.PasswordHash = auth.HashPassword(req.GetPassword())
	}
	if req.HasPolicy() {
		u.Policy = policyFromProto(req.GetPolicy())
	}
	if req.HasPreferences() {
		u.Preferences = preferencesFromProto(req.GetPreferences())
	}
	if err := s.store.Users().Create(ctx, &u); err != nil {
		return nil, connectError(ctx, err)
	}
	return userv1.CreateUserResponse_builder{User: userToProto(&u)}.Build(), nil
}

// UpdateUser changes a user's name, rights, state and policy.
func (s *UserService) UpdateUser(ctx context.Context, req *userv1.UpdateUserRequest) (*userv1.UpdateUserResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	var u core.User
	err := s.store.InTx(ctx, func(tx core.Store) error {
		var err error
		if u, err = tx.Users().Get(ctx, core.MustParseID(req.GetId())); err != nil {
			return err
		}
		u.Name, u.Admin, u.Disabled = req.GetName(), req.GetAdmin(), req.GetDisabled()
		if req.HasPolicy() {
			u.Policy = policyFromProto(req.GetPolicy())
		}
		if err := tx.Users().Update(ctx, &u); err != nil {
			return err
		}
		return adminRemains(ctx, tx)
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return userv1.UpdateUserResponse_builder{User: userToProto(&u)}.Build(), nil
}

// DeleteUser removes an account with its state and sessions.
func (s *UserService) DeleteUser(ctx context.Context, req *userv1.DeleteUserRequest) (*userv1.DeleteUserResponse, error) {
	if _, err := admin(ctx); err != nil {
		return nil, err
	}
	err := s.store.InTx(ctx, func(tx core.Store) error {
		if err := tx.Users().Delete(ctx, core.MustParseID(req.GetId())); err != nil {
			return err
		}
		return adminRemains(ctx, tx)
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return &userv1.DeleteUserResponse{}, nil
}

// adminRemains fails when no enabled administrator is left.
func adminRemains(ctx context.Context, tx core.Store) error {
	users, err := tx.Users().List(ctx)
	if err != nil {
		return err
	}
	if !slices.ContainsFunc(users, func(u core.User) bool { return u.Admin && !u.Disabled }) {
		return connect.NewError(connect.CodeFailedPrecondition, errors.New("the server needs an enabled administrator"))
	}
	return nil
}

// SetPassword sets a user's password: administrators set anyone's, users
// change their own with the current one. The user's other sessions end.
func (s *UserService) SetPassword(ctx context.Context, req *userv1.SetPasswordRequest) (*userv1.SetPasswordResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	id := core.MustParseID(req.GetUserId())
	self := id == p.User.ID
	if !self && !p.User.Admin {
		return nil, connect.NewError(connect.CodePermissionDenied, errors.New("administrator required"))
	}
	err = s.store.InTx(ctx, func(tx core.Store) error {
		u, err := tx.Users().Get(ctx, id)
		if err != nil {
			return err
		}
		if u.AuthProvider != "" {
			return connect.NewError(connect.CodeFailedPrecondition, errors.New("the user signs in through "+u.AuthProvider))
		}
		if self {
			if ok, _, err := auth.VerifyPassword(u.PasswordHash, req.GetCurrentPassword()); err != nil || !ok {
				return connect.NewError(connect.CodePermissionDenied, errors.New("wrong current password"))
			}
		}
		u.PasswordHash = auth.HashPassword(req.GetNewPassword())
		if err := tx.Users().Update(ctx, &u); err != nil {
			return err
		}
		sessions, err := tx.AuthSessions().ListForUser(ctx, id)
		if err != nil {
			return err
		}
		for _, sess := range sessions {
			if sess.ID != p.Session.ID {
				if err := tx.AuthSessions().Delete(ctx, sess.ID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return &userv1.SetPasswordResponse{}, nil
}

// UpdatePreferences changes the caller's playback preferences.
func (s *UserService) UpdatePreferences(ctx context.Context, req *userv1.UpdatePreferencesRequest) (*userv1.UpdatePreferencesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	var u core.User
	err = s.store.InTx(ctx, func(tx core.Store) error {
		if u, err = tx.Users().Get(ctx, p.User.ID); err != nil {
			return err
		}
		u.Preferences = preferencesFromProto(req.GetPreferences())
		return tx.Users().Update(ctx, &u)
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return userv1.UpdatePreferencesResponse_builder{Preferences: userToProto(&u).GetPreferences()}.Build(), nil
}

// UserDataService implements mavio.user.v1.UserDataService.
type UserDataService struct {
	store core.Store
	now   func() time.Time
}

var _ userv1connect.UserDataServiceHandler = (*UserDataService)(nil)

// NewUserDataService returns a UserDataService backed by store.
func NewUserDataService(store core.Store) *UserDataService {
	return &UserDataService{store: store, now: time.Now}
}

// GetUserData returns the caller's state of the items that have some.
func (s *UserDataService) GetUserData(ctx context.Context, req *userv1.GetUserDataRequest) (*userv1.GetUserDataResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]core.ID, len(req.GetItemIds()))
	for i, id := range req.GetItemIds() {
		ids[i] = core.MustParseID(id)
	}
	data, err := s.store.UserData().GetMany(ctx, p.User.ID, ids)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out := make([]*userv1.UserData, 0, len(data))
	for _, id := range ids {
		if d, ok := data[id]; ok {
			out = append(out, events.UserDataToProto(&d))
		}
	}
	return userv1.GetUserDataResponse_builder{UserData: out}.Build(), nil
}

// UpdateUserData changes the set fields of the caller's state of an item.
// Marking a folder such as a series played or unplayed marks its playable
// descendants.
func (s *UserDataService) UpdateUserData(ctx context.Context, req *userv1.UpdateUserDataRequest) (*userv1.UpdateUserDataResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	var data core.UserData
	err = s.store.InTx(ctx, func(tx core.Store) error {
		item, err := tx.Items().Get(ctx, core.MustParseID(req.GetItemId()))
		if err != nil {
			return err
		}
		if !p.User.CanAccess(&item) {
			return core.ErrNotFound
		}
		update := func(it *core.Item, fields bool) (core.UserData, error) {
			d, err := tx.UserData().Get(ctx, p.User.ID, it.ID)
			if errors.Is(err, core.ErrNotFound) {
				d = core.UserData{UserID: p.User.ID, ItemID: it.ID}
			} else if err != nil {
				return d, err
			}
			if req.HasPlayed() && it.Kind.SupportsPlayed() {
				s.setPlayed(&d, req.GetPlayed())
			}
			if fields {
				if req.HasFavorite() {
					d.Favorite = req.GetFavorite()
				}
				if req.HasPosition() && it.Kind.SupportsResume() {
					d.Position = req.GetPosition().AsDuration()
				}
				if req.HasRating() {
					d.Rating = new(req.GetRating())
				}
			}
			return d, tx.UserData().Put(ctx, &d)
		}
		if data, err = update(&item, true); err != nil {
			return err
		}
		if !req.HasPlayed() || item.Kind.SupportsPlayed() {
			return nil
		}
		// Read the descendants before writing within the transaction.
		var children []core.Item
		for child, err := range tx.Items().Walk(ctx, core.ItemQuery{ParentID: item.ID, Recursive: true}) {
			if err != nil {
				return err
			}
			if child.Kind.SupportsPlayed() {
				children = append(children, child)
			}
		}
		for i := range children {
			if _, err := update(&children[i], false); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return userv1.UpdateUserDataResponse_builder{UserData: events.UserDataToProto(&data)}.Build(), nil
}

// setPlayed marks an item played or unplayed as Jellyfin does: played
// counts at least one play and clears the resume position; unplayed
// resets the play state.
func (s *UserDataService) setPlayed(d *core.UserData, played bool) {
	if played {
		d.Played, d.Position, d.PlayCount = true, 0, max(d.PlayCount, 1)
		if d.LastPlayedAt == nil {
			d.LastPlayedAt = new(s.now())
		}
		return
	}
	d.Played, d.Position, d.PlayCount, d.LastPlayedAt = false, 0, 0, nil
}

func policyFromProto(p *userv1.UserPolicy) core.UserPolicy {
	out := core.UserPolicy{
		BlockUnrated:        p.GetBlockUnrated(),
		AllowTranscoding:    p.GetAllowTranscoding(),
		AllowDownload:       p.GetAllowDownload(),
		MaxStreamingBitrate: p.GetMaxStreamingBitrate(),
		MaxSessions:         int(p.GetMaxSessions()),
	}
	if p.HasMaxParentalRating() {
		out.MaxParentalRating = new(int(p.GetMaxParentalRating()))
	}
	if !p.GetAllLibraries() {
		// Non-nil: only the listed libraries, possibly none.
		out.Libraries = make([]core.ID, 0, len(p.GetLibraryIds()))
		for _, id := range p.GetLibraryIds() {
			out.Libraries = append(out.Libraries, core.MustParseID(id))
		}
	}
	return out
}

func preferencesFromProto(p *userv1.UserPreferences) core.UserPreferences {
	out := core.UserPreferences{
		AudioLanguages:        p.GetAudioLanguages(),
		SubtitleLanguages:     p.GetSubtitleLanguages(),
		PlayDefaultAudioTrack: p.GetPlayDefaultAudioTrack(),
	}
	for mode, v := range subtitleModes {
		if v == p.GetSubtitleMode() {
			out.SubtitleMode = mode
		}
	}
	return out
}

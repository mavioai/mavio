package app

import (
	"context"
	"slices"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/types/known/timestamppb"

	systemv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/system/v1"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
)

// fakeUserList are the server's users: its administrator, a disabled
// guest, and a managed user with a PIN.
func fakeUserList() []*userv1.User {
	user := func(id, name string, admin, managed, disabled bool, last time.Duration) *userv1.User {
		b := userv1.User_builder{Id: &id, Name: &name, Admin: &admin, Managed: &managed, Disabled: &disabled}
		if last > 0 {
			b.LastLoginTime = tsAgo(last)
		}
		if managed {
			b.HasPin = new(true)
			b.Restriction = new(userv1.RestrictionProfile_RESTRICTION_PROFILE_YOUNGER_CHILD)
			b.Policy = userv1.UserPolicy_builder{AllLibraries: new(true), AllowTranscoding: new(true)}.Build()
		}
		return b.Build()
	}
	return []*userv1.User{
		user("u3", "Kids", false, true, false, 0),
		user("u2", "Guest", false, false, true, 30*24*time.Hour),
		user("u1", "Anna", true, false, false, time.Hour),
	}
}

func (u *fakeUsers) ListUsers(context.Context, *userv1.ListUsersRequest) (*userv1.ListUsersResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	return userv1.ListUsersResponse_builder{Users: slices.Clone(u.f.users)}.Build(), nil
}

func (u *fakeUsers) find(id string) (*userv1.User, error) {
	i := slices.IndexFunc(u.f.users, func(x *userv1.User) bool { return x.GetId() == id })
	if i < 0 {
		return nil, connect.NewError(connect.CodeNotFound, nil)
	}
	return u.f.users[i], nil
}

func (u *fakeUsers) CreateUser(_ context.Context, r *userv1.CreateUserRequest) (*userv1.CreateUserResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	user := userv1.User_builder{
		Id: new("u-new"), Name: new(r.GetName()), Managed: new(r.GetManaged()), Restriction: new(r.GetRestriction()),
		HasPin: new(r.GetPin() != ""), Policy: userv1.UserPolicy_builder{AllLibraries: new(true)}.Build(),
	}.Build()
	u.f.users = append(u.f.users, user)
	return userv1.CreateUserResponse_builder{User: user}.Build(), nil
}

func (u *fakeUsers) UpdateUser(_ context.Context, r *userv1.UpdateUserRequest) (*userv1.UpdateUserResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	user, err := u.find(r.GetId())
	if err != nil {
		return nil, err
	}
	user.SetName(r.GetName())
	user.SetDisabled(r.GetDisabled())
	user.SetRestriction(r.GetRestriction())
	user.SetPolicy(r.GetPolicy())
	return userv1.UpdateUserResponse_builder{User: user}.Build(), nil
}

func (u *fakeUsers) DeleteUser(_ context.Context, r *userv1.DeleteUserRequest) (*userv1.DeleteUserResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	u.f.users = slices.DeleteFunc(u.f.users, func(x *userv1.User) bool { return x.GetId() == r.GetId() })
	return &userv1.DeleteUserResponse{}, nil
}

func (u *fakeUsers) SetPassword(_ context.Context, r *userv1.SetPasswordRequest) (*userv1.SetPasswordResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	user, err := u.find(r.GetUserId())
	if err != nil {
		return nil, err
	}
	if u.f.passwords == nil {
		u.f.passwords = map[string]string{}
	}
	u.f.passwords[user.GetId()] = r.GetNewPassword()
	user.SetManaged(false)
	return &userv1.SetPasswordResponse{}, nil
}

func (u *fakeUsers) SetPin(_ context.Context, r *userv1.SetPinRequest) (*userv1.SetPinResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	user, err := u.find(r.GetUserId())
	if err != nil {
		return nil, err
	}
	user.SetHasPin(r.GetPin() != "")
	return &userv1.SetPinResponse{}, nil
}

func (u *fakeUsers) ListInvitations(context.Context, *userv1.ListInvitationsRequest) (*userv1.ListInvitationsResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	return userv1.ListInvitationsResponse_builder{Invitations: slices.Clone(u.f.invitations)}.Build(), nil
}

func (u *fakeUsers) CreateInvitation(_ context.Context, r *userv1.CreateInvitationRequest) (*userv1.CreateInvitationResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	now := time.Now()
	id := "i" + string(rune('1'+len(u.f.invitations)))
	inv := userv1.Invitation_builder{
		Id: &id, UserId: new(r.GetUserId()), Access: r.GetAccess(), MaxUses: new(r.GetMaxUses()), Uses: new(int32(0)),
		CreateTime: timestamppb.New(now), ExpireTime: timestamppb.New(now.Add(r.GetValidFor().AsDuration())),
	}.Build()
	u.f.invitations = append(u.f.invitations, inv)
	return userv1.CreateInvitationResponse_builder{Invitation: inv, Token: new("tok-" + id)}.Build(), nil
}

func (u *fakeUsers) DeleteInvitation(_ context.Context, r *userv1.DeleteInvitationRequest) (*userv1.DeleteInvitationResponse, error) {
	u.f.mu.Lock()
	defer u.f.mu.Unlock()
	u.f.invitations = slices.DeleteFunc(u.f.invitations, func(i *userv1.Invitation) bool { return i.GetId() == r.GetId() })
	return &userv1.DeleteInvitationResponse{}, nil
}

func (fakeLocalization) ListRatings(context.Context, *systemv1.ListRatingsRequest) (*systemv1.ListRatingsResponse, error) {
	rating := func(name string, score int32) *systemv1.Rating {
		return systemv1.Rating_builder{Name: &name, Score: &score}.Build()
	}
	return systemv1.ListRatingsResponse_builder{Ratings: []*systemv1.Rating{
		rating("G", 0), rating("PG", 10), rating("PG-13", 13), rating("R", 17), rating("NC-17", 18), rating("XXX", 1000),
	}}.Build(), nil
}

package app

import (
	"context"

	authv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/auth/v1"
)

// switchPin is the PIN of Kids, whom the session switches to with it.
const switchPin = "1234"

// ListSwitchableUsers lists the user signed in and the server's enabled
// users.
func (a *fakeAuth) ListSwitchableUsers(context.Context, *authv1.ListSwitchableUsersRequest) (*authv1.ListSwitchableUsersResponse, error) {
	a.f.mu.Lock()
	defer a.f.mu.Unlock()
	me := a.f.record()
	users := []*authv1.SwitchableUser{authv1.SwitchableUser_builder{Id: new(me.GetUserId()), Name: new(me.GetUserName()), Admin: new(true), Current: new(true)}.Build()}
	for _, u := range a.f.users {
		if u.GetDisabled() || u.GetId() == me.GetUserId() {
			continue
		}
		users = append(users, authv1.SwitchableUser_builder{
			Id: new(u.GetId()), Name: new(u.GetName()), Admin: new(u.GetAdmin()), Managed: new(u.GetManaged()), HasPin: new(u.GetHasPin()),
		}.Build())
	}
	return authv1.ListSwitchableUsersResponse_builder{Users: users}.Build(), nil
}

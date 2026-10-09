package rpc

import (
	"context"
	"errors"

	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/mavioai/mavio/libs/core"
	userv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/user/v1/userv1connect"
)

// DisplayPreferencesService implements
// mavio.user.v1.DisplayPreferencesService for the calling user.
type DisplayPreferencesService struct {
	store core.Store
}

var _ userv1connect.DisplayPreferencesServiceHandler = (*DisplayPreferencesService)(nil)

// NewDisplayPreferencesService returns a DisplayPreferencesService backed
// by store.
func NewDisplayPreferencesService(store core.Store) *DisplayPreferencesService {
	return &DisplayPreferencesService{store: store}
}

// GetDisplayPreferences returns the preferences of a client's view.
func (s *DisplayPreferencesService) GetDisplayPreferences(ctx context.Context, req *userv1.GetDisplayPreferencesRequest) (*userv1.GetDisplayPreferencesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	prefs, err := s.store.DisplayPreferences().Get(ctx, p.User.ID, req.GetClient(), req.GetView())
	switch {
	case errors.Is(err, core.ErrNotFound):
		prefs = core.DisplayPreferences{Client: req.GetClient(), View: req.GetView()}
	case err != nil:
		return nil, connectError(ctx, err)
	}
	return userv1.GetDisplayPreferencesResponse_builder{Preferences: displayPreferencesToProto(&prefs)}.Build(), nil
}

// SetDisplayPreferences replaces the preferences of a client's view.
func (s *DisplayPreferencesService) SetDisplayPreferences(ctx context.Context, req *userv1.SetDisplayPreferencesRequest) (*userv1.SetDisplayPreferencesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	prefs := core.DisplayPreferences{UserID: p.User.ID, Client: req.GetClient(), View: req.GetView(), Values: req.GetValues()}
	if err := s.store.DisplayPreferences().Put(ctx, &prefs); err != nil {
		return nil, connectError(ctx, err)
	}
	return userv1.SetDisplayPreferencesResponse_builder{Preferences: displayPreferencesToProto(&prefs)}.Build(), nil
}

func displayPreferencesToProto(p *core.DisplayPreferences) *userv1.DisplayPreferences {
	b := userv1.DisplayPreferences_builder{Client: &p.Client, View: &p.View, Values: p.Values}
	if !p.UpdatedAt.IsZero() {
		b.UpdateTime = timestamppb.New(p.UpdatedAt)
	}
	return b.Build()
}

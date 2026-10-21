package rpc

import (
	"context"
	"errors"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/apps/server/internal/activity"
	"github.com/mavioai/mavio/apps/server/internal/settings"
	"github.com/mavioai/mavio/libs/core"
)

// currentSettings returns the settings m keeps, the defaults when m is
// nil.
func currentSettings(m *settings.Manager) core.ServerSettings {
	if m == nil {
		return core.DefaultServerSettings()
	}
	return m.Get()
}

// changeSettings has an administrator change a group of the settings m
// keeps, which applies them at once, and records it in log as changing
// what, such as "the transcoding settings".
func changeSettings(ctx context.Context, m *settings.Manager, log *activity.Log, what string, change func(s *core.ServerSettings) error) (core.ServerSettings, error) {
	p, err := admin(ctx)
	if err != nil {
		return core.ServerSettings{}, err
	}
	if m == nil {
		return core.ServerSettings{}, connect.NewError(connect.CodeFailedPrecondition, errors.New("settings cannot be changed"))
	}
	set, err := m.Change(ctx, change)
	if err != nil {
		return core.ServerSettings{}, connectError(ctx, err)
	}
	if log != nil {
		log.Record(ctx, core.Activity{Type: "settings.updated", UserID: p.User.ID, Title: p.User.Name + " changed " + what})
	}
	return set, nil
}

// keyOf returns the key of v in m, the zero key when there is none.
func keyOf[K, V comparable](m map[K]V, v V) K {
	for k, mv := range m {
		if mv == v {
			return k
		}
	}
	var zero K
	return zero
}

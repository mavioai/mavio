package library

import (
	"context"
	"errors"
	"log/slog"
	"slices"

	"github.com/mavioai/mavio/libs/core"
)

// IntroProvider picks items to play before an item, such as an intro
// provider plugin.
type IntroProvider interface {
	Name() string
	// Intros returns the IDs of the items to play before it for a user, in
	// order.
	Intros(ctx context.Context, it core.Item, userID core.ID) ([]core.ID, error)
}

// MaxIntros bounds the items played before an item.
const MaxIntros = 20

// Intros finds the items to play before items through the intro
// providers.
type Intros struct {
	Store core.Store
	// Source gives the providers, in order.
	Source func() []IntroProvider
	Logger *slog.Logger
}

// List returns the items to play before an item for a user: the
// providers' picks in their order, each once, leaving out items the user
// cannot access, items that do not play and the item itself. A provider
// failing leaves the others' picks.
func (in *Intros) List(ctx context.Context, user *core.User, itemID core.ID) ([]core.Item, error) {
	it, err := in.Store.Items().Get(ctx, itemID)
	if err != nil {
		return nil, err
	}
	if !user.CanAccess(&it) {
		return nil, core.ErrNotFound
	}
	if in.Source == nil {
		return nil, nil
	}
	seen := map[core.ID]bool{it.ID: true}
	var out []core.Item
	for _, p := range in.Source() {
		ids, err := p.Intros(ctx, it, user.ID)
		if err != nil {
			in.logger().WarnContext(ctx, "intro provider failed", "provider", p.Name(), "item", it.ID, "err", err)
			continue
		}
		for _, id := range ids {
			if len(out) == MaxIntros {
				return out, nil
			}
			if seen[id] {
				continue
			}
			seen[id] = true
			intro, err := in.Store.Items().Get(ctx, id)
			switch {
			case errors.Is(err, core.ErrNotFound):
				continue
			case err != nil:
				return nil, err
			}
			if user.CanAccess(&intro) && slices.Contains(playableKinds, intro.Kind) {
				out = append(out, intro)
			}
		}
	}
	return out, nil
}

// playableKinds are the kinds of items that play as intros.
var playableKinds = []core.ItemKind{core.KindMovie, core.KindEpisode, core.KindVideo, core.KindMusicVideo, core.KindTrack, core.KindAudioBook}

func (in *Intros) logger() *slog.Logger {
	if in.Logger != nil {
		return in.Logger
	}
	return slog.New(slog.DiscardHandler)
}

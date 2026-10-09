package rpc

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"connectrpc.com/connect"

	"github.com/mavioai/mavio/libs/core"
	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"
)

// curatedLibrary returns the library of a curated kind, creating it when
// first needed.
func curatedLibrary(ctx context.Context, store core.Store, kind core.LibraryKind, name string) (core.Library, error) {
	find := func() (core.Library, bool, error) {
		libs, err := store.Libraries().List(ctx)
		if err != nil {
			return core.Library{}, false, err
		}
		for _, lib := range libs {
			if lib.Kind == kind {
				return lib, true, nil
			}
		}
		return core.Library{}, false, nil
	}
	if lib, ok, err := find(); err != nil || ok {
		return lib, err
	}
	lib := core.Library{Name: name, Kind: kind}
	err := store.Libraries().Create(ctx, &lib)
	if errors.Is(err, core.ErrConflict) { // created concurrently
		lib, _, err = find()
	}
	return lib, err
}

// curatedItem returns a collection or playlist by ID, or not_found when
// it is none or the caller may not see it.
func curatedItem(ctx context.Context, store core.Store, u *core.User, id string, kind core.ItemKind) (core.Item, error) {
	it, err := store.Items().Get(ctx, core.MustParseID(id))
	if errors.Is(err, core.ErrNotFound) || err == nil && (it.Kind != kind || !u.CanAccess(&it)) {
		return it, connect.NewError(connect.CodeNotFound, fmt.Errorf("no such %s", kind))
	}
	return it, err
}

// rename renames a collection or playlist, whose sort name follows its
// name, and reads it back.
func rename(ctx context.Context, store core.Store, it *core.Item, name string) error {
	it.Name, it.SortName = name, ""
	if err := store.Items().Upsert(ctx, *it); err != nil {
		return err
	}
	var err error
	*it, err = store.Items().Get(ctx, it.ID)
	return err
}

// CollectionService implements mavio.library.v1.CollectionService.
// Administrators curate collections.
type CollectionService struct {
	store core.Store
	now   func() time.Time
}

var _ libraryv1connect.CollectionServiceHandler = (*CollectionService)(nil)

// NewCollectionService returns a CollectionService backed by store.
func NewCollectionService(store core.Store) *CollectionService {
	return &CollectionService{store: store, now: time.Now}
}

// CreateCollection creates a collection of items.
func (s *CollectionService) CreateCollection(ctx context.Context, req *libraryv1.CreateCollectionRequest) (*libraryv1.CreateCollectionResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	members, err := s.members(ctx, req.GetItemIds())
	if err != nil {
		return nil, err
	}
	lib, err := curatedLibrary(ctx, s.store, core.LibraryCollections, "Collections")
	if err != nil {
		return nil, connectError(ctx, err)
	}
	c := core.Item{ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindCollection, Name: req.GetName(), DateAdded: s.now()}
	err = s.store.InTx(ctx, func(tx core.Store) error {
		if err := tx.Items().Upsert(ctx, c); err != nil {
			return err
		}
		return tx.Items().ReplaceLinks(ctx, c.ID, linksTo(members))
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := itemsToProto(ctx, s.store, []core.Item{c}, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.CreateCollectionResponse_builder{Collection: out[0]}.Build(), nil
}

// UpdateCollection renames a collection.
func (s *CollectionService) UpdateCollection(ctx context.Context, req *libraryv1.UpdateCollectionRequest) (*libraryv1.UpdateCollectionResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	c, err := curatedItem(ctx, s.store, &p.User, req.GetId(), core.KindCollection)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := rename(ctx, s.store, &c, req.GetName()); err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := itemsToProto(ctx, s.store, []core.Item{c}, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.UpdateCollectionResponse_builder{Collection: out[0]}.Build(), nil
}

// DeleteCollection deletes a collection; its items stay.
func (s *CollectionService) DeleteCollection(ctx context.Context, req *libraryv1.DeleteCollectionRequest) (*libraryv1.DeleteCollectionResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	c, err := curatedItem(ctx, s.store, &p.User, req.GetId(), core.KindCollection)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := s.store.Items().Delete(ctx, c.ID); err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.DeleteCollectionResponse{}, nil
}

// AddToCollection appends the items not yet in a collection.
func (s *CollectionService) AddToCollection(ctx context.Context, req *libraryv1.AddToCollectionRequest) (*libraryv1.AddToCollectionResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	members, err := s.members(ctx, req.GetItemIds())
	if err != nil {
		return nil, err
	}
	err = s.edit(ctx, &p.User, req.GetId(), func(links []core.Link) []core.Link {
		for _, l := range linksTo(members) {
			if !slices.ContainsFunc(links, func(m core.Link) bool { return m.ItemID == l.ItemID }) {
				links = append(links, l)
			}
		}
		return links
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.AddToCollectionResponse{}, nil
}

// RemoveFromCollection removes items from a collection.
func (s *CollectionService) RemoveFromCollection(ctx context.Context, req *libraryv1.RemoveFromCollectionRequest) (*libraryv1.RemoveFromCollectionResponse, error) {
	p, err := admin(ctx)
	if err != nil {
		return nil, err
	}
	err = s.edit(ctx, &p.User, req.GetId(), func(links []core.Link) []core.Link {
		return slices.DeleteFunc(links, func(l core.Link) bool { return slices.Contains(req.GetItemIds(), l.ItemID.String()) })
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.RemoveFromCollectionResponse{}, nil
}

// edit changes a collection's entries in one transaction.
func (s *CollectionService) edit(ctx context.Context, u *core.User, id string, change func([]core.Link) []core.Link) error {
	return s.store.InTx(ctx, func(tx core.Store) error {
		c, err := curatedItem(ctx, tx, u, id, core.KindCollection)
		if err != nil {
			return err
		}
		links, err := tx.Items().Links(ctx, c.ID)
		if err != nil {
			return err
		}
		return tx.Items().ReplaceLinks(ctx, c.ID, change(links))
	})
}

// members returns the items to put in a collection, each once: any items
// but collections and playlists.
func (s *CollectionService) members(ctx context.Context, ids []string) ([]core.Item, error) {
	var out []core.Item
	for _, id := range ids {
		it, err := s.store.Items().Get(ctx, core.MustParseID(id))
		switch {
		case errors.Is(err, core.ErrNotFound):
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no item %s", id))
		case err != nil:
			return nil, connectError(ctx, err)
		case it.Kind.IsCurated():
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("a %s cannot be in a collection", it.Kind))
		}
		if !slices.ContainsFunc(out, func(m core.Item) bool { return m.ID == it.ID }) {
			out = append(out, it)
		}
	}
	return out, nil
}

// linksTo returns new entries for items.
func linksTo(items []core.Item) []core.Link {
	out := make([]core.Link, len(items))
	for i, it := range items {
		out[i] = core.Link{ID: core.NewID(), ItemID: it.ID}
	}
	return out
}

// PlaylistService implements mavio.library.v1.PlaylistService. Users
// manage their own playlists.
type PlaylistService struct {
	store core.Store
	now   func() time.Time
}

var _ libraryv1connect.PlaylistServiceHandler = (*PlaylistService)(nil)

// NewPlaylistService returns a PlaylistService backed by store.
func NewPlaylistService(store core.Store) *PlaylistService {
	return &PlaylistService{store: store, now: time.Now}
}

// ListPlaylists lists the caller's playlists by name.
func (s *PlaylistService) ListPlaylists(ctx context.Context, req *libraryv1.ListPlaylistsRequest) (*libraryv1.ListPlaylistsResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	page, err := s.store.Items().Query(ctx, core.ItemQuery{
		Kinds:  []core.ItemKind{core.KindPlaylist},
		UserID: p.User.ID,
		Sort:   []core.SortSpec{{Field: core.SortName}},
		Limit:  int(req.GetLimit()),
		Offset: int(req.GetOffset()),
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := itemsToProto(ctx, s.store, page.Items, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.ListPlaylistsResponse_builder{Playlists: out, Total: new(int32(page.Total))}.Build(), nil
}

// ListPlaylistEntries lists a playlist's entries the caller may access.
func (s *PlaylistService) ListPlaylistEntries(ctx context.Context, req *libraryv1.ListPlaylistEntriesRequest) (*libraryv1.ListPlaylistEntriesResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	pl, err := curatedItem(ctx, s.store, &p.User, req.GetId(), core.KindPlaylist)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	links, err := s.store.Items().Links(ctx, pl.ID)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	items := map[core.ID]core.Item{}
	if q, ok := userItems(&p.User); ok {
		q.MemberOf = pl.ID
		for it, err := range s.store.Items().Walk(ctx, q) {
			if err != nil {
				return nil, connectError(ctx, err)
			}
			items[it.ID] = it
		}
	}
	links = slices.DeleteFunc(links, func(l core.Link) bool { _, ok := items[l.ItemID]; return !ok })
	total := len(links)
	limit := cmp.Or(int(req.GetLimit()), core.MaxPageSize)
	links = links[min(len(links), int(req.GetOffset())):]
	links = links[:min(len(links), limit)]

	list := make([]core.Item, len(links))
	for i, l := range links {
		list[i] = items[l.ItemID]
	}
	described, err := itemsToProto(ctx, s.store, list, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	entries := make([]*libraryv1.PlaylistEntry, len(links))
	for i, l := range links {
		entries[i] = libraryv1.PlaylistEntry_builder{Id: new(l.ID.String()), Item: described[i]}.Build()
	}
	return libraryv1.ListPlaylistEntriesResponse_builder{Entries: entries, Total: new(int32(total))}.Build(), nil
}

// CreatePlaylist creates a playlist of the caller.
func (s *PlaylistService) CreatePlaylist(ctx context.Context, req *libraryv1.CreatePlaylistRequest) (*libraryv1.CreatePlaylistResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.playable(ctx, &p.User, req.GetItemIds())
	if err != nil {
		return nil, err
	}
	lib, err := curatedLibrary(ctx, s.store, core.LibraryPlaylists, "Playlists")
	if err != nil {
		return nil, connectError(ctx, err)
	}
	pl := core.Item{
		ID: core.NewID(), LibraryID: lib.ID, Kind: core.KindPlaylist, Name: req.GetName(),
		UserID: p.User.ID, DateAdded: s.now(),
	}
	err = s.store.InTx(ctx, func(tx core.Store) error {
		if err := tx.Items().Upsert(ctx, pl); err != nil {
			return err
		}
		return tx.Items().ReplaceLinks(ctx, pl.ID, linksTo(items))
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := itemsToProto(ctx, s.store, []core.Item{pl}, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.CreatePlaylistResponse_builder{Playlist: out[0]}.Build(), nil
}

// UpdatePlaylist renames a playlist.
func (s *PlaylistService) UpdatePlaylist(ctx context.Context, req *libraryv1.UpdatePlaylistRequest) (*libraryv1.UpdatePlaylistResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	pl, err := curatedItem(ctx, s.store, &p.User, req.GetId(), core.KindPlaylist)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := rename(ctx, s.store, &pl, req.GetName()); err != nil {
		return nil, connectError(ctx, err)
	}
	out, err := itemsToProto(ctx, s.store, []core.Item{pl}, p.User.Admin)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return libraryv1.UpdatePlaylistResponse_builder{Playlist: out[0]}.Build(), nil
}

// DeletePlaylist deletes a playlist.
func (s *PlaylistService) DeletePlaylist(ctx context.Context, req *libraryv1.DeletePlaylistRequest) (*libraryv1.DeletePlaylistResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	pl, err := curatedItem(ctx, s.store, &p.User, req.GetId(), core.KindPlaylist)
	if err != nil {
		return nil, connectError(ctx, err)
	}
	if err := s.store.Items().Delete(ctx, pl.ID); err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.DeletePlaylistResponse{}, nil
}

// AddToPlaylist inserts entries for items, appending unless a position is
// given.
func (s *PlaylistService) AddToPlaylist(ctx context.Context, req *libraryv1.AddToPlaylistRequest) (*libraryv1.AddToPlaylistResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	items, err := s.playable(ctx, &p.User, req.GetItemIds())
	if err != nil {
		return nil, err
	}
	added := linksTo(items)
	err = s.edit(ctx, &p.User, req.GetId(), func(links []core.Link) ([]core.Link, error) {
		at := len(links)
		if req.HasPosition() {
			at = min(at, int(req.GetPosition()))
		}
		return slices.Insert(links, at, added...), nil
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	ids := make([]string, len(added))
	for i, l := range added {
		ids[i] = l.ID.String()
	}
	return libraryv1.AddToPlaylistResponse_builder{EntryIds: ids}.Build(), nil
}

// RemoveFromPlaylist removes entries from a playlist.
func (s *PlaylistService) RemoveFromPlaylist(ctx context.Context, req *libraryv1.RemoveFromPlaylistRequest) (*libraryv1.RemoveFromPlaylistResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	err = s.edit(ctx, &p.User, req.GetId(), func(links []core.Link) ([]core.Link, error) {
		return slices.DeleteFunc(links, func(l core.Link) bool { return slices.Contains(req.GetEntryIds(), l.ID.String()) }), nil
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.RemoveFromPlaylistResponse{}, nil
}

// MovePlaylistEntry moves an entry to another position.
func (s *PlaylistService) MovePlaylistEntry(ctx context.Context, req *libraryv1.MovePlaylistEntryRequest) (*libraryv1.MovePlaylistEntryResponse, error) {
	p, err := principal(ctx)
	if err != nil {
		return nil, err
	}
	err = s.edit(ctx, &p.User, req.GetId(), func(links []core.Link) ([]core.Link, error) {
		i := slices.IndexFunc(links, func(l core.Link) bool { return l.ID.String() == req.GetEntryId() })
		if i < 0 {
			return nil, connect.NewError(connect.CodeNotFound, errors.New("no such playlist entry"))
		}
		entry := links[i]
		links = slices.Delete(links, i, i+1)
		return slices.Insert(links, min(len(links), int(req.GetPosition())), entry), nil
	})
	if err != nil {
		return nil, connectError(ctx, err)
	}
	return &libraryv1.MovePlaylistEntryResponse{}, nil
}

// edit changes the entries of a playlist of the caller in one
// transaction.
func (s *PlaylistService) edit(ctx context.Context, u *core.User, id string, change func([]core.Link) ([]core.Link, error)) error {
	return s.store.InTx(ctx, func(tx core.Store) error {
		pl, err := curatedItem(ctx, tx, u, id, core.KindPlaylist)
		if err != nil {
			return err
		}
		links, err := tx.Items().Links(ctx, pl.ID)
		if err != nil {
			return err
		}
		if links, err = change(links); err != nil {
			return err
		}
		return tx.Items().ReplaceLinks(ctx, pl.ID, links)
	})
}

// playable resolves items for a playlist: playable items as they are, and
// the playable items of containers and collections in their order; the
// caller must be able to access them.
func (s *PlaylistService) playable(ctx context.Context, u *core.User, ids []string) ([]core.Item, error) {
	var media []core.ItemKind
	for _, k := range core.ItemKinds {
		if k.HasMedia() {
			media = append(media, k)
		}
	}
	var out []core.Item
	for _, id := range ids {
		it, err := s.store.Items().Get(ctx, core.MustParseID(id))
		switch {
		case errors.Is(err, core.ErrNotFound) || err == nil && !u.CanAccess(&it):
			return nil, connect.NewError(connect.CodeNotFound, fmt.Errorf("no item %s", id))
		case err != nil:
			return nil, connectError(ctx, err)
		case it.Kind.HasMedia():
			out = append(out, it)
			continue
		case !it.Kind.IsContainer() || it.Kind == core.KindPlaylist:
			return nil, connect.NewError(connect.CodeInvalidArgument, fmt.Errorf("a %s cannot be in a playlist", it.Kind))
		}
		// The caller may access the container, so some library.
		q, _ := userItems(u)
		q.Kinds = media
		if it.Kind == core.KindCollection {
			q.MemberOf, q.Sort = it.ID, []core.SortSpec{{Field: core.SortListOrder}}
		} else {
			q.ParentID, q.Recursive, q.Sort = it.ID, true, []core.SortSpec{{Field: core.SortIndex}, {Field: core.SortName}}
		}
		for q.Offset = 0; ; q.Offset += core.MaxPageSize {
			page, err := s.store.Items().Query(ctx, q)
			if err != nil {
				return nil, connectError(ctx, err)
			}
			out = append(out, page.Items...)
			if len(page.Items) < core.MaxPageSize {
				break
			}
		}
	}
	return out, nil
}

// userItems returns a query for the items the user may access; it
// reports false when the user may access no library.
func userItems(u *core.User) (core.ItemQuery, bool) {
	libs, ok := libraryScope(&u.Policy, nil)
	return core.ItemQuery{
		LibraryIDs:  libs,
		MaxRating:   u.Policy.MaxParentalRating,
		SkipUnrated: u.Policy.BlockUnrated,
		UserID:      u.ID,
	}, ok
}

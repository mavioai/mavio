package app

import (
	"context"

	"github.com/egoist/mygo/ui"

	libraryv1 "github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1"
	"github.com/mavioai/mavio/libs/proto/gen/go/mavio/library/v1/libraryv1connect"

	"github.com/mavioai/mavio/apps/desktop/internal/style"
)

// moreDialog names a dialog of an item's More menu.
type moreDialog int

const (
	moreNone moreDialog = iota
	morePlayOn
	morePlaylist
	moreCollection
	moreEdit
	moreIdentify
	moreRefresh
	moreImages
	moreRename
	moreDelete
)

// moreState is the state of an item's More menu's dialogs.
type moreState struct {
	dialog moreDialog
	open   bool // the dialog shows
	// lists are the playlists or collections offered to add to; name is
	// a new one's name or the new name, added the list added to.
	lists task[[]*libraryv1.Item]
	name  string
	added string
	busy  bool
	err   error
	// remote is Play On's.
	remote remoteState
	// metadata is the metadata dialogs'.
	metadata metadataState
}

func (a *App) playlistService(id string) libraryv1connect.PlaylistServiceClient {
	return libraryv1connect.NewPlaylistServiceClient(a.connectHTTP(), serverURL(id))
}

func (a *App) collectionService(id string) libraryv1connect.CollectionServiceClient {
	return libraryv1connect.NewCollectionServiceClient(a.connectHTTP(), serverURL(id))
}

// listable reports whether an item of a kind can be put in a playlist or
// a collection.
func listable(kind libraryv1.ItemKind) bool {
	switch kind {
	case libraryv1.ItemKind_ITEM_KIND_PLAYLIST, libraryv1.ItemKind_ITEM_KIND_COLLECTION, libraryv1.ItemKind_ITEM_KIND_FOLDER:
		return false
	}
	return true
}

// moreMenu builds the More button of an item's actions, as the web UI's
// (apps/web/src/screens/lists.tsx): adding the item to a playlist, or for
// administrators to a collection, and renaming or deleting a playlist or
// collection; and the dialogs it opens.
func (a *App) moreMenu(c *ui.Context, st *detailsState, server string, it *libraryv1.Item) {
	admin := a.isAdmin(server)
	kind := it.GetKind()
	own := kind == libraryv1.ItemKind_ITEM_KIND_PLAYLIST || kind == libraryv1.ItemKind_ITEM_KIND_COLLECTION && admin
	type entry struct {
		dialog moreDialog
		label  string
	}
	var entries []entry
	if playable(kind) {
		entries = append(entries, entry{morePlayOn, style.T(c, "remote.playOn")})
	}
	if listable(kind) {
		entries = append(entries, entry{morePlaylist, style.T(c, "lists.addToPlaylist")})
		if admin {
			entries = append(entries, entry{moreCollection, style.T(c, "lists.addToCollection")})
		}
	}
	if admin && kind != libraryv1.ItemKind_ITEM_KIND_PLAYLIST {
		entries = append(entries, entry{moreEdit, style.T(c, "metadata.edit")})
		if identifiable(kind) {
			entries = append(entries, entry{moreIdentify, style.T(c, "metadata.identify")})
		}
		if kind != libraryv1.ItemKind_ITEM_KIND_COLLECTION {
			entries = append(entries, entry{moreRefresh, style.T(c, "metadata.refresh")})
		}
		entries = append(entries, entry{moreImages, style.T(c, "metadata.images")})
	}
	if own {
		entries = append(entries, entry{moreRename, style.T(c, "lists.rename")}, entry{moreDelete, style.T(c, "lists.delete")})
	}
	if len(entries) == 0 {
		return
	}
	m := &st.more
	a.overButton(c, "more", style.T(c, "lists.more"), false).Menu(func(menu *ui.Menu) {
		for _, e := range entries {
			if menu.Item(e.label).Chosen() {
				*m = moreState{dialog: e.dialog, open: true, remote: remoteState{open: true, volume: 100}}
				switch e.dialog {
				case moreRename:
					m.name = it.GetName()
				case moreEdit:
					m.metadata.before, m.metadata.form = formOf(it), formOf(it)
				case moreIdentify:
					m.metadata.name, m.metadata.year = it.GetName(), numberOf(it.GetProductionYear())
					m.metadata.query = m.metadata.name + "\x00" + m.metadata.year
				}
			}
		}
	})
	if !m.open {
		m.dialog = moreNone
		return
	}
	switch m.dialog {
	case morePlayOn:
		a.playOnDialog(c, &m.remote, server, it)
		m.open = m.remote.open
	case morePlaylist, moreCollection:
		a.addListDialog(c, m, server, it)
	case moreEdit:
		a.editDialog(c, m, server, it)
	case moreIdentify:
		a.identifyDialog(c, m, server, it)
	case moreRefresh:
		a.refreshDialog(c, m, server, it)
	case moreImages:
		a.imagesDialog(c, m, server, it)
	case moreRename:
		a.renameDialog(c, m, server, it)
	case moreDelete:
		a.deleteListDialog(c, m, server, it)
	}
}

// addListDialog builds the dialog adding the item to a playlist or
// collection, or to a new one.
func (a *App) addListDialog(c *ui.Context, m *moreState, server string, it *libraryv1.Item) {
	colors := style.Colors(c)
	playlist := m.dialog == morePlaylist
	lists := load(a, &m.lists, key("lists", server, playlist, a.generation), func(ctx context.Context) ([]*libraryv1.Item, error) {
		if playlist {
			res, err := a.playlistService(server).ListPlaylists(ctx, &libraryv1.ListPlaylistsRequest{})
			return res.GetPlaylists(), err
		}
		res, err := a.itemService(server).ListItems(ctx, libraryv1.ListItemsRequest_builder{
			Kinds: []libraryv1.ItemKind{libraryv1.ItemKind_ITEM_KIND_COLLECTION},
			Sort:  []*libraryv1.SortSpec{libraryv1.SortSpec_builder{Field: new(libraryv1.SortField_SORT_FIELD_NAME)}.Build()},
		}.Build())
		return res.GetItems(), err
	})
	title, icon, placeholder := style.T(c, "lists.addToCollection"), "collections", style.T(c, "lists.newCollection")
	if playlist {
		title, icon, placeholder = style.T(c, "lists.addToPlaylist"), "playlists", style.T(c, "lists.newPlaylist")
	}
	add := func(id, name string) {
		m.busy, m.err = true, nil
		go func() {
			var err error
			if playlist {
				_, err = a.playlistService(server).AddToPlaylist(a.ctx, libraryv1.AddToPlaylistRequest_builder{Id: &id, ItemIds: []string{it.GetId()}}.Build())
			} else {
				_, err = a.collectionService(server).AddToCollection(a.ctx, libraryv1.AddToCollectionRequest_builder{Id: &id, ItemIds: []string{it.GetId()}}.Build())
			}
			a.opts.Update(func() {
				m.busy, m.err = false, err
				if err == nil {
					m.added = name
				}
			})
		}()
	}
	create := func(name string) {
		m.busy, m.err = true, nil
		go func() {
			var err error
			if playlist {
				_, err = a.playlistService(server).CreatePlaylist(a.ctx, libraryv1.CreatePlaylistRequest_builder{Name: &name, ItemIds: []string{it.GetId()}}.Build())
			} else {
				_, err = a.collectionService(server).CreateCollection(a.ctx, libraryv1.CreateCollectionRequest_builder{Name: &name, ItemIds: []string{it.GetId()}}.Build())
			}
			a.opts.Update(func() {
				m.busy, m.err = false, err
				if err == nil {
					m.added, m.name = name, ""
					m.lists.key = ""
				}
			})
		}()
	}
	ui.Modal(c, &m.open, func() {
		ui.Column(c).Width(440).MaxHeight(600).Gap(12).Children(func() {
			style.Text(ui.Text(c, title), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			note := it.GetName()
			if m.added != "" {
				note = style.T(c, "lists.added", "name", it.GetName(), "list", m.added)
			}
			style.Text(ui.Text(c, note), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
			ui.Scroll(c).Grow(1).Shrink(1).Children(func() {
				ui.Column(c).Gap(2).Children(func() {
					for _, l := range lists.value {
						b := ui.ButtonBase(c.Key(l.GetId())).MinHeight(44).Padding(0, 12).Gap(12).Radius(8).AlignItems(ui.Center).
							Label(l.GetName()).Cursor(ui.CursorPointer).Disabled(m.busy)
						if b.Hovered() || a.tvFocus(c, b, false) {
							b.Background(style.Color(colors.Fill))
						}
						b.Children(func() {
							ui.Icon(c, style.Icon(icon, false)).Size(18, 18).TextColor(style.Color(colors.TextSecondary))
							style.Text(ui.Text(c, l.GetName()), style.Type(c).Body).TextColor(style.Color(colors.TextPrimary)).SingleLine().Grow(1)
							if m.added == l.GetName() {
								ui.Icon(c, style.Icon("check", false)).Size(16, 16).TextColor(style.Color(colors.Accent))
							}
						})
						if b.Clicked() {
							add(l.GetId(), l.GetName())
						}
					}
				})
			})
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				in := ui.TextInput(c, &m.name).Grow(1).Label(placeholder).Placeholder(placeholder)
				ok := len(m.name) > 0 && len(m.name) <= 200 && !m.busy
				if secondaryButton(c, "", style.T(c, "lists.create")).Disabled(!ok).Clicked() || in.Submitted() && ok {
					create(m.name)
				}
			})
			if err := m.err; err != nil {
				style.Text(ui.Text(c, errorMessage(c, err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			} else if err := lists.err; err != nil {
				style.Text(ui.Text(c, errorMessage(c, err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
			ui.Row(c).Justify(ui.End).Children(func() {
				if ui.PrimaryButton(c, style.T(c, "keys.done")).Clicked() {
					m.open = false
				}
			})
		})
	})
}

// renameDialog builds the dialog renaming a playlist or collection.
func (a *App) renameDialog(c *ui.Context, m *moreState, server string, it *libraryv1.Item) {
	colors := style.Colors(c)
	save := func() {
		id, name := it.GetId(), m.name
		m.busy, m.err = true, nil
		go func() {
			var err error
			if it.GetKind() == libraryv1.ItemKind_ITEM_KIND_PLAYLIST {
				_, err = a.playlistService(server).UpdatePlaylist(a.ctx, libraryv1.UpdatePlaylistRequest_builder{Id: &id, Name: &name}.Build())
			} else {
				_, err = a.collectionService(server).UpdateCollection(a.ctx, libraryv1.UpdateCollectionRequest_builder{Id: &id, Name: &name}.Build())
			}
			a.opts.Update(func() {
				m.busy, m.err = false, err
				if err == nil {
					m.open = false
					a.generation++
				}
			})
		}()
	}
	ui.Modal(c, &m.open, func() {
		ui.Column(c).Width(400).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "lists.rename")), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			ok := len(m.name) > 0 && len(m.name) <= 200 && !m.busy
			if ui.TextInput(c, &m.name).Label(style.T(c, "lists.name")).AutoFocus().Submitted() && ok {
				save()
			}
			if m.err != nil {
				style.Text(ui.Text(c, errorMessage(c, m.err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					m.open = false
				}
				if ui.PrimaryButton(c, style.T(c, "lists.save")).Disabled(!ok).Clicked() {
					save()
				}
			})
		})
	})
}

// deleteListDialog builds the dialog asking before deleting a playlist or
// collection, whose page is then left for its library.
func (a *App) deleteListDialog(c *ui.Context, m *moreState, server string, it *libraryv1.Item) {
	colors := style.Colors(c)
	playlist := it.GetKind() == libraryv1.ItemKind_ITEM_KIND_PLAYLIST
	detail := style.T(c, "lists.deleteCollection")
	if playlist {
		detail = style.T(c, "lists.deletePlaylist")
	}
	ui.Modal(c, &m.open, func() {
		ui.Column(c).Width(400).Gap(12).Children(func() {
			style.Text(ui.Text(c, style.T(c, "lists.deleteTitle", "name", it.GetName())), style.Type(c).Title3).TextColor(style.Color(colors.TextPrimary))
			style.Text(ui.Text(c, detail), style.Type(c).Callout).TextColor(style.Color(colors.TextSecondary))
			if m.err != nil {
				style.Text(ui.Text(c, errorMessage(c, m.err)), style.Type(c).Footnote).TextColor(style.Color(colors.Danger))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if secondaryButton(c, "", style.T(c, "action.cancel")).Clicked() {
					m.open = false
				}
				if !destructiveButton(c, style.T(c, "lists.delete")).Disabled(m.busy).Clicked() {
					return
				}
				id := it.GetId()
				m.busy, m.err = true, nil
				go func() {
					var err error
					if playlist {
						_, err = a.playlistService(server).DeletePlaylist(a.ctx, libraryv1.DeletePlaylistRequest_builder{Id: &id}.Build())
					} else {
						_, err = a.collectionService(server).DeleteCollection(a.ctx, libraryv1.DeleteCollectionRequest_builder{Id: &id}.Build())
					}
					a.opts.Update(func() {
						m.busy, m.err = false, err
						if err != nil {
							return
						}
						m.open = false
						a.generation++
						if lib := it.GetLibraryId(); lib != "" {
							a.router.Replace("/library/" + server + "/" + lib)
						} else {
							a.router.Replace("/libraries")
						}
					})
				}()
			})
		})
	})
}

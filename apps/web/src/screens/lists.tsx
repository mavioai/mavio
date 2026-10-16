// Playlists and collections on an item's details (docs/clients.md §6.3.20):
// the More menu adding the item to a playlist, or for administrators to a
// collection, and renaming or deleting a playlist or collection; a
// playlist's entries in order, moved and removed; a collection's members
// removed by its administrators. The menu also plays the item on another
// device (§6.3.23), watches it together (§6.3.24), reports a problem with it
// (§6.3.25) and, for administrators, opens its metadata (§6.3.22).
import {
  errorMessage,
  type ServerEntry,
  thumbOf,
  useAdminServers,
  useCollectionActions,
  useCollections,
  usePlaylistActions,
  usePlaylistEntries,
  usePlaylists,
} from "@mavio/client-ts";
import { type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import type { PlaylistEntry } from "@mavio/proto/mavio/library/v1/playlist_pb";
import { Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Artwork, Button, Grid, ItemCard, runtimeOf, subtitleOf } from "../ui.tsx";
import { Menu, MenuItem } from "../controls/menu.tsx";
import { Tip } from "../controls/tooltip.tsx";
import { EditDialog, IdentifyDialog, identifiable, ImagesDialog, RefreshDialog } from "./metadata.tsx";
import { field } from "../controls/styles.ts";
import { Modal } from "../controls/dialog.tsx";
import { playable, PlayOnDialog } from "./remote.tsx";
import { ReportDialog, reportable, useMayReport } from "./issues.tsx";
import { SyncPlayDialog } from "./syncplay.tsx";

type Dialog = "playOn" | "together" | "report" | "playlist" | "collection" | "edit" | "identify" | "refresh" | "images" | "rename" | "delete";

/** Kinds that can be put in a playlist or a collection. */
const listable = (kind: ItemKind) => kind !== ItemKind.PLAYLIST && kind !== ItemKind.COLLECTION && kind !== ItemKind.FOLDER;

/** The More menu of an item's actions, with the dialogs it opens. */
export function MoreMenu({ server, item, className, iconSize, phone }: { server: ServerEntry; item: Item; className: string; iconSize: number; phone: boolean }) {
  const admin = useAdminServers().some((s) => s.id === server.id);
  const mayReport = useMayReport(server);
  const [dialog, setDialog] = useState<Dialog>();
  const own = item.kind === ItemKind.PLAYLIST || (item.kind === ItemKind.COLLECTION && admin);
  const entries: [Dialog, string][] = [
    ...(playable(item.kind)
      ? ([
          ["playOn", t("remote.playOn")],
          ["together", t("syncplay.watchTogether")],
        ] as [Dialog, string][])
      : []),
    ...(listable(item.kind) ? ([["playlist", t("lists.addToPlaylist")]] as [Dialog, string][]) : []),
    ...(reportable(item.kind) && mayReport ? ([["report", t("issues.report")]] as [Dialog, string][]) : []),
    ...(listable(item.kind) && admin ? ([["collection", t("lists.addToCollection")]] as [Dialog, string][]) : []),
    ...(admin && item.kind !== ItemKind.PLAYLIST
      ? ([
          ["edit", t("metadata.edit")],
          ...(identifiable(item.kind) ? [["identify", t("metadata.identify")]] : []),
          ...(item.kind !== ItemKind.COLLECTION ? [["refresh", t("metadata.refresh")]] : []),
          ["images", t("metadata.images")],
        ] as [Dialog, string][])
      : []),
    ...(own
      ? ([
          ["rename", t("lists.rename")],
          ["delete", t("lists.delete")],
        ] as [Dialog, string][])
      : []),
  ];
  if (entries.length === 0) return null;
  return (
    <>
      <Menu
        side="top"
        align={phone ? "end" : "start"}
        width="w-[260px]"
        trigger={
          <Tip label={t("lists.more")}>
            <button type="button" aria-label={t("lists.more")} className={className}>
              <Icon name="more" size={iconSize} />
            </button>
          </Tip>
        }
      >
        {entries.map(([d, label]) => (
          <MenuItem key={d} danger={d === "delete"} onClick={() => setDialog(d)}>
            {label}
          </MenuItem>
        ))}
      </Menu>
      {(dialog === "playlist" || dialog === "collection") && <AddDialog server={server} item={item} kind={dialog} onClose={() => setDialog(undefined)} />}
      {dialog === "report" && <ReportDialog server={server} item={item} onClose={() => setDialog(undefined)} />}
      {dialog === "together" && <SyncPlayDialog server={server} item={item} onClose={() => setDialog(undefined)} />}
      {dialog === "playOn" && <PlayOnDialog server={server} item={item} onClose={() => setDialog(undefined)} />}
      {dialog === "edit" && <EditDialog server={server} item={item} onClose={() => setDialog(undefined)} />}
      {dialog === "identify" && <IdentifyDialog server={server} item={item} onClose={() => setDialog(undefined)} />}
      {dialog === "refresh" && <RefreshDialog server={server} item={item} onClose={() => setDialog(undefined)} />}
      {dialog === "images" && <ImagesDialog server={server} item={item} onClose={() => setDialog(undefined)} />}
      {dialog === "rename" && <RenameDialog server={server} item={item} onClose={() => setDialog(undefined)} />}
      {dialog === "delete" && <DeleteDialog server={server} item={item} onClose={() => setDialog(undefined)} />}
    </>
  );
}

/** Adds the item to a playlist or collection, or to a new one. */
function AddDialog({ server, item, kind, onClose }: { server: ServerEntry; item: Item; kind: "playlist" | "collection"; onClose: () => void }) {
  const playlists = usePlaylists(server, kind === "playlist");
  const collections = useCollections(server, kind === "collection");
  const playlistActions = usePlaylistActions(server);
  const collectionActions = useCollectionActions(server);
  const lists = (kind === "playlist" ? playlists.data : collections.data) ?? [];
  const add = kind === "playlist" ? playlistActions.add : collectionActions.add;
  const create = kind === "playlist" ? playlistActions.create : collectionActions.create;
  const [name, setName] = useState("");
  const [added, setAdded] = useState<string>();
  const error = add.error ?? create.error;
  return (
    <Modal title={kind === "playlist" ? t("lists.addToPlaylist") : t("lists.addToCollection")} onClose={onClose}>
      <p className="type-callout text-text-secondary">{added ? t("lists.added", { name: item.name, list: added }) : item.name}</p>
      <div className="-mx-3 flex min-h-0 flex-col overflow-y-auto">
        {lists.map((l) => (
          <button
            key={l.id}
            type="button"
            disabled={add.isPending}
            onClick={() => add.mutate({ id: l.id, itemIds: [item.id] }, { onSuccess: () => setAdded(l.name) })}
            className="flex min-h-11 w-full items-center gap-3 rounded-lg px-3 text-left type-body hover:bg-fill"
          >
            <Icon name={kind === "playlist" ? "playlists" : "collections"} size={18} className="text-text-secondary" />
            <span className="min-w-0 flex-1 truncate">{l.name}</span>
            {added === l.name && <Icon name="check" size={16} className="text-accent" />}
          </button>
        ))}
      </div>
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          const n = name.trim();
          if (n) create.mutate({ name: n, itemIds: [item.id] }, { onSuccess: () => (setAdded(n), setName("")) });
        }}
      >
        <input
          className={field}
          aria-label={kind === "playlist" ? t("lists.newPlaylist") : t("lists.newCollection")}
          placeholder={kind === "playlist" ? t("lists.newPlaylist") : t("lists.newCollection")}
          maxLength={200}
          value={name}
          onChange={(e) => setName(e.target.value)}
        />
        <Button type="submit" variant="secondary" disabled={!name.trim() || create.isPending}>
          {t("lists.create")}
        </Button>
      </form>
      {error && <p className="type-footnote break-words text-danger">{errorMessage(error)}</p>}
      <div className="flex justify-end">
        <Button onClick={onClose}>{t("keys.done")}</Button>
      </div>
    </Modal>
  );
}

function RenameDialog({ server, item, onClose }: { server: ServerEntry; item: Item; onClose: () => void }) {
  const playlist = usePlaylistActions(server).rename;
  const collection = useCollectionActions(server).rename;
  const rename = item.kind === ItemKind.PLAYLIST ? playlist : collection;
  const [name, setName] = useState(item.name);
  return (
    <Modal title={t("lists.rename")} onClose={onClose}>
      <form
        className="flex flex-col gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          if (name.trim()) rename.mutate({ id: item.id, name: name.trim() }, { onSuccess: onClose });
        }}
      >
        <input
          className={field}
          aria-label={t("lists.name")}
          maxLength={200}
          value={name}
          autoFocus
          onFocus={(e) => e.target.select()}
          onChange={(e) => setName(e.target.value)}
        />
        {rename.isError && <p className="type-footnote break-words text-danger">{errorMessage(rename.error)}</p>}
        <div className="flex justify-end gap-2">
          <Button variant="plain" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={!name.trim() || rename.isPending}>
            {t("lists.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

function DeleteDialog({ server, item, onClose }: { server: ServerEntry; item: Item; onClose: () => void }) {
  const playlist = usePlaylistActions(server).remove;
  const collection = useCollectionActions(server).remove;
  const remove = item.kind === ItemKind.PLAYLIST ? playlist : collection;
  const navigate = useNavigate();
  // The deleted list's page is left for its library.
  const done = () => {
    onClose();
    void navigate(
      item.libraryId
        ? { to: "/s/$serverId/library/$libraryId", params: { serverId: server.id, libraryId: item.libraryId }, replace: true }
        : { to: "/libraries", replace: true },
    );
  };
  return (
    <Modal title={t("lists.deleteTitle", { name: item.name })} onClose={onClose}>
      <p className="type-callout text-text-secondary">{item.kind === ItemKind.PLAYLIST ? t("lists.deletePlaylist") : t("lists.deleteCollection")}</p>
      {remove.isError && <p className="type-footnote break-words text-danger">{errorMessage(remove.error)}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="plain" onClick={onClose}>
          {t("action.cancel")}
        </Button>
        <Button variant="secondary" className="text-danger" disabled={remove.isPending} onClick={() => remove.mutate(item.id, { onSuccess: done })}>
          {t("lists.delete")}
        </Button>
      </div>
    </Modal>
  );
}

/** A playlist's entries in order, each played, moved or removed in place. */
export function PlaylistEntries({ server, playlist }: { server: ServerEntry; playlist: Item }) {
  const entries = usePlaylistEntries(server, playlist.id);
  const list = entries.data ?? [];
  return (
    <section className="px-margin py-4">
      {entries.isError && <p className="type-callout break-words text-danger">{errorMessage(entries.error)}</p>}
      {entries.isSuccess && list.length === 0 && <p className="type-body text-text-secondary">{t("lists.emptyPlaylist")}</p>}
      <ol className="flex flex-col">
        {list.map((e, i) => (
          <EntryRow key={e.id} server={server} playlistId={playlist.id} entry={e} index={i} last={i === list.length - 1} />
        ))}
      </ol>
    </section>
  );
}

function EntryRow({ server, playlistId, entry: e, index, last }: { server: ServerEntry; playlistId: string; entry: PlaylistEntry; index: number; last: boolean }) {
  const { move, removeEntries } = usePlaylistActions(server);
  const item = e.item;
  if (!item) return null;
  const busy = move.isPending || removeEntries.isPending;
  const small = "inline-flex size-9 shrink-0 items-center justify-center rounded-full text-text-secondary hover:bg-fill disabled:opacity-30";
  return (
    <li className="flex items-center gap-3 rounded-md px-2 py-2 hover:tint">
      <span className="w-6 text-right type-callout text-text-tertiary tabular-nums">{index + 1}</span>
      <Link to="/s/$serverId/play/$itemId" params={{ serverId: server.id, itemId: item.id }} className="flex min-w-0 flex-1 items-center gap-3">
        <Artwork src={thumbOf(server, item, 240)?.url} alt="" className="aspect-video w-24 shrink-0 rounded-md" />
        <span className="min-w-0 flex-1">
          <span className="block truncate type-body">{item.name}</span>
          <span className="block truncate type-footnote text-text-secondary">{[subtitleOf(item), runtimeOf(item)].filter(Boolean).join(" · ")}</span>
        </span>
      </Link>
      <Tip label={t("lists.moveUp")}>
        <button type="button" aria-label={t("lists.moveUp")} disabled={busy || index === 0} onClick={() => move.mutate({ id: playlistId, entryId: e.id, position: index - 1 })} className={small}>
          <Icon name="expand" size={16} className="rotate-180" />
        </button>
      </Tip>
      <Tip label={t("lists.moveDown")}>
        <button type="button" aria-label={t("lists.moveDown")} disabled={busy || last} onClick={() => move.mutate({ id: playlistId, entryId: e.id, position: index + 1 })} className={small}>
          <Icon name="expand" size={16} />
        </button>
      </Tip>
      <Tip label={t("lists.remove")}>
        <button type="button" aria-label={t("lists.remove")} disabled={busy} onClick={() => removeEntries.mutate({ id: playlistId, entryIds: [e.id] })} className={small}>
          <Icon name="close" size={16} />
        </button>
      </Tip>
    </li>
  );
}

/** A collection's members, which its administrators can remove. */
export function CollectionMembers({ server, collection, items }: { server: ServerEntry; collection: Item; items: Item[] }) {
  const admin = useAdminServers().some((s) => s.id === server.id);
  const { removeItems } = useCollectionActions(server);
  const [editing, setEditing] = useState(false);
  if (items.length === 0) return <p className="px-margin py-4 type-body text-text-secondary">{t("lists.emptyCollection")}</p>;
  return (
    <section className="py-4">
      {admin && (
        <div className="flex justify-end px-margin pb-3">
          <Button variant="secondary" onClick={() => setEditing(!editing)}>
            {editing ? t("keys.done") : t("lists.edit")}
          </Button>
        </div>
      )}
      {removeItems.isError && <p className="px-margin pb-3 type-callout break-words text-danger">{errorMessage(removeItems.error)}</p>}
      <Grid>
        {items.map((i) => (
          <div key={i.id} className="relative">
            <ItemCard server={server} serverId={server.id} item={i} fluid />
            {editing && (
              <button
                type="button"
                aria-label={t("lists.removeNamed", { name: i.name })}
                disabled={removeItems.isPending}
                onClick={() => removeItems.mutate({ id: collection.id, itemIds: [i.id] })}
                className="absolute -top-2 -right-2 inline-flex size-8 items-center justify-center rounded-full bg-danger text-white shadow-level2"
              >
                <Icon name="close" size={16} />
              </button>
            )}
          </div>
        ))}
      </Grid>
    </section>
  );
}

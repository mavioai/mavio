// SyncPlay (docs/clients.md §6.3.24): the dialog creating, joining and
// leaving a group and playing an item in it, the chip of the player
// following a group, and the indicator the screens show while in one.
import { errorMessage, groupPositionAt, type ServerEntry, useItem, useServersStore, useSyncPlayGroups } from "@mavio/client-ts";
import type { Item } from "@mavio/proto/mavio/library/v1/item_pb";
import { type SyncPlayGroup, SyncPlayService, SyncPlayState } from "@mavio/proto/mavio/session/v1/syncplay_pb";
import { useLocation, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { useWidthClass } from "../prefs.ts";
import { useSyncPlay } from "../syncplay.tsx";
import { Button } from "../ui.tsx";
import { field } from "../controls/styles.ts";
import { Modal } from "../controls/dialog.tsx";

/** Who a group waits for. */
function waitingFor(group: SyncPlayGroup): string {
  const names = group.members.filter((m) => m.buffering).map((m) => m.deviceName || m.userName);
  return t("syncplay.waitingFor", { names: names.join(", ") });
}

/** What a group does: its state and the name of its entry. */
function GroupState({ server, group }: { server: ServerEntry; group: SyncPlayGroup }) {
  const entry = group.queue[group.currentIndex];
  const item = useItem(entry ? server : undefined, entry?.itemId ?? "");
  const name = item.data?.item?.name ?? "…";
  const text = {
    [SyncPlayState.UNSPECIFIED]: t("syncplay.idle"),
    [SyncPlayState.IDLE]: t("syncplay.idle"),
    [SyncPlayState.WAITING]: waitingFor(group),
    [SyncPlayState.PAUSED]: t("syncplay.paused", { name }),
    [SyncPlayState.PLAYING]: t("syncplay.playing", { name }),
  }[group.state];
  return <p className="type-callout text-text-secondary">{text}</p>;
}

/**
 * The browser's group, or the server's groups to join and a new one; in a
 * group on the item's server, the item plays in it from its resume
 * position.
 */
export function SyncPlayDialog({ server, item, onClose }: { server: ServerEntry; item?: Item; onClose: () => void }) {
  const servers = useServersStore();
  const sp = useSyncPlay();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const joined = sp?.joined;
  const groups = useSyncPlayGroups(server, !joined);
  const [name, setName] = useState(() => t("syncplay.defaultName", { name: server.userName }));
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string>();
  if (!sp) return null;
  const rpc = (s: ServerEntry) => servers.client(SyncPlayService, s);
  const run = async (f: () => Promise<unknown>) => {
    setBusy(true);
    setError(undefined);
    try {
      await f();
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };

  if (joined) {
    const { server: at, group } = joined;
    const entry = group.queue[group.currentIndex];
    const watching = entry && group.state !== SyncPlayState.IDLE && pathname !== `/s/${at.id}/play/${entry.itemId}`;
    const position = item?.userData?.position && !item.userData.played ? item.userData.position : undefined;
    return (
      <Modal title={group.name} onClose={onClose}>
        <GroupState server={at} group={group} />
        <ul className="-mx-3 flex min-h-0 flex-col overflow-y-auto" aria-label={t("syncplay.members")}>
          {group.members.map((m) => (
            <li key={m.sessionId} className="flex min-h-11 items-center gap-3 px-3">
              <Icon name="devices" size={20} className="text-text-secondary" />
              <span className="min-w-0 flex-1">
                <span className="block truncate type-body">{m.userName}</span>
                <span className="block truncate type-footnote text-text-secondary">
                  {[m.deviceName, m.sessionId === at.sessionId ? t("syncplay.thisBrowser") : ""].filter(Boolean).join(" · ")}
                </span>
              </span>
              {m.buffering && <span className="type-footnote text-text-secondary">{t("syncplay.loading")}</span>}
            </li>
          ))}
        </ul>
        {error && <p className="type-footnote break-words text-danger">{error}</p>}
        <div className="flex flex-wrap justify-end gap-2">
          <Button variant="plain" className="text-danger" disabled={busy} onClick={() => void run(async () => (await sp.leave(), onClose()))}>
            {t("syncplay.leave")}
          </Button>
          {watching && (
            <Button
              variant="secondary"
              onClick={() => {
                const start = Math.round(groupPositionAt(group, sp.serverNow()));
                void navigate({ to: "/s/$serverId/play/$itemId", params: { serverId: at.id, itemId: entry.itemId }, search: { start: start || undefined } });
                onClose();
              }}
            >
              {t("syncplay.watch")}
            </Button>
          )}
          {item && at.id === server.id ? (
            <Button
              disabled={busy}
              onClick={() => void run(async () => (await rpc(at).setQueue({ itemIds: [item.id], startPosition: position }), onClose()))}
            >
              {t("syncplay.playHere", { name: item.name })}
            </Button>
          ) : (
            <Button onClick={onClose}>{t("keys.done")}</Button>
          )}
        </div>
      </Modal>
    );
  }

  return (
    <Modal title={t("syncplay.title")} onClose={onClose}>
      <p className="type-callout text-text-secondary">{t("syncplay.intro")}</p>
      <div className="-mx-3 flex min-h-0 flex-col overflow-y-auto">
        {(groups.data ?? []).map((g) => (
          <div key={g.id} className="flex min-h-12 items-center gap-3 px-3 py-1.5">
            <Icon name="users" size={20} className="text-text-secondary" />
            <span className="min-w-0 flex-1">
              <span className="block truncate type-body">{g.name}</span>
              <span className="block truncate type-footnote text-text-secondary">{g.members.map((m) => m.userName).join(", ")}</span>
            </span>
            <Button
              variant="secondary"
              disabled={busy}
              onClick={() =>
                void run(async () => {
                  const r = await rpc(server).joinGroup({ groupId: g.id });
                  if (r.group) sp.enter(server, r.group);
                })
              }
            >
              {t("syncplay.join")}
            </Button>
          </div>
        ))}
        {groups.isSuccess && groups.data.length === 0 && <p className="px-3 type-callout text-text-secondary">{t("syncplay.noGroups")}</p>}
      </div>
      <form
        className="flex gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          void run(async () => {
            const r = await rpc(server).createGroup({ name: name.trim() });
            if (r.group) sp.enter(server, r.group);
          });
        }}
      >
        <input className={field} aria-label={t("syncplay.groupName")} maxLength={200} value={name} onChange={(e) => setName(e.target.value)} />
        <Button type="submit" variant="secondary" disabled={busy || !name.trim()}>
          {t("syncplay.create")}
        </Button>
      </form>
      {(error || groups.error) && <p className="type-footnote break-words text-danger">{error ?? errorMessage(groups.error)}</p>}
      <div className="flex justify-end">
        <Button variant="secondary" onClick={onClose}>
          {t("action.cancel")}
        </Button>
      </div>
    </Modal>
  );
}

/** The player's chip of the group it follows: who it waits for, else the group's name and size; it opens the group. */
export function GroupChip({ server, group }: { server: ServerEntry; group: SyncPlayGroup }) {
  const [open, setOpen] = useState(false);
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="ml-auto flex h-9 min-w-0 items-center gap-1.5 rounded-full bg-white/15 px-3.5 type-footnote font-semibold backdrop-blur-xl hover:bg-white/25"
      >
        <Icon name="users" fill size={16} />
        <span className="truncate">
          {group.state === SyncPlayState.WAITING ? waitingFor(group) : t("syncplay.chip", { name: group.name, count: group.members.length })}
        </span>
      </button>
      {open && <SyncPlayDialog server={server} onClose={() => setOpen(false)} />}
    </>
  );
}

/** The screens' indicator of the browser's group, opening it. */
export function GroupIndicator() {
  const sp = useSyncPlay();
  const { pathname } = useLocation();
  const compact = useWidthClass() === "compact";
  const [open, setOpen] = useState(false);
  const joined = sp?.joined;
  if (!joined || pathname.includes("/play/")) return null;
  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={`fixed right-4 z-30 flex h-10 max-w-[calc(100vw-32px)] items-center gap-2 rounded-full border border-border bg-surface-overlay/90 px-4 type-footnote font-semibold text-on-background shadow-level2 backdrop-blur-xl hover:bg-surface-raised ${compact ? "bottom-[calc(68px+env(safe-area-inset-bottom))]" : "bottom-4"}`}
      >
        <Icon name="users" fill size={18} className="text-accent-text" />
        <span className="truncate">{t("syncplay.chip", { name: joined.group.name, count: joined.group.members.length })}</span>
      </button>
      {open && <SyncPlayDialog server={joined.server} onClose={() => setOpen(false)} />}
    </>
  );
}

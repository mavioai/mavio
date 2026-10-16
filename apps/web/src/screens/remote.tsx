// Remote control (docs/clients.md §6.3.23): the commands other devices send
// this browser — playing items, and messages shown over any screen, the
// player carrying out the rest — and the dialogs that play an item on
// another device of the user and control it.
import { durationFromMs, durationMs } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useSendCommand, useSessions } from "@mavio/client-ts";
import { type Item, ItemKind } from "@mavio/proto/mavio/library/v1/item_pb";
import { PlayStateCommand, type Session } from "@mavio/proto/mavio/session/v1/session_pb";
import { useNavigate } from "@tanstack/react-router";
import { useEffect, useState } from "react";

import { Slider } from "../controls/inputs.tsx";
import { Tip } from "../controls/tooltip.tsx";
import { useServerEvents } from "../events.tsx";
import { t } from "../i18n.ts";
import { Icon, type IconName } from "../icon.tsx";
import { Button, subtitleOf } from "../ui.tsx";
import { field } from "../controls/styles.ts";
import { Modal } from "../controls/dialog.tsx";

/** Kinds a device plays by themselves. */
export const playable = (kind: ItemKind) =>
  [ItemKind.MOVIE, ItemKind.EPISODE, ItemKind.VIDEO, ItemKind.MUSIC_VIDEO, ItemKind.TRACK, ItemKind.AUDIOBOOK].includes(kind);

interface Shown {
  from: string;
  text: string;
  timeoutMs?: number;
}

/**
 * Carries out what other devices ask of this browser outside the player:
 * playing an item opens the player at its position, and a message shows
 * until dismissed or for as long as it asks.
 */
export function RemoteCommands() {
  const navigate = useNavigate();
  const [message, setMessage] = useState<Shown>();
  useServerEvents((server, e) => {
    if (e.kind.case !== "command") return;
    const { fromUserName, command } = e.kind.value;
    const c = command?.kind;
    if (c?.case === "play") {
      const id = c.value.itemIds[c.value.startIndex] ?? c.value.itemIds[0];
      if (!id) return;
      const start = c.value.startPosition ? Math.round(durationMs(c.value.startPosition)) : undefined;
      void navigate({ to: "/s/$serverId/play/$itemId", params: { serverId: server.id, itemId: id }, search: { start: start || undefined } });
    } else if (c?.case === "message") {
      setMessage({ from: fromUserName, text: c.value.text, timeoutMs: c.value.timeout ? durationMs(c.value.timeout) : undefined });
    }
  });
  useEffect(() => {
    if (!message?.timeoutMs) return;
    const timer = setTimeout(() => setMessage(undefined), message.timeoutMs);
    return () => clearTimeout(timer);
  }, [message]);
  if (!message) return null;
  return (
    <div role="status" className="fixed inset-x-0 top-4 z-[60] flex justify-center px-4">
      <div className="flex max-w-lg items-start gap-3 rounded-[14px] border border-border bg-surface-overlay/95 px-4 py-3 text-on-background shadow-level3 backdrop-blur-xl">
        <div className="min-w-0 flex-1">
          {message.from && <p className="type-footnote text-text-secondary">{t("remote.messageFrom", { name: message.from })}</p>}
          <p className="type-body break-words whitespace-pre-line">{message.text}</p>
        </div>
        <button type="button" aria-label={t("remote.dismiss")} onClick={() => setMessage(undefined)} className="rounded-full p-1 hover:bg-fill">
          <Icon name="close" size={18} />
        </button>
      </div>
    </div>
  );
}

/** A device's name, else its client's. */
function deviceName(s: Session): string {
  return s.deviceName || s.client || "—";
}

/** The other online devices of the user that play items: theirs and the plugins' remote devices. */
function useTargets(server: ServerEntry) {
  const sessions = useSessions(server);
  const targets = (sessions.data ?? []).filter((s) => s.online && s.id !== server.sessionId);
  return { sessions, targets };
}

/** Picks a device to play the item on, then controls it. */
export function PlayOnDialog({ server, item, onClose }: { server: ServerEntry; item: Item; onClose: () => void }) {
  const { sessions, targets } = useTargets(server);
  const send = useSendCommand(server);
  const [controlling, setControlling] = useState<string>();
  if (controlling) return <RemoteDialog server={server} sessionId={controlling} onClose={onClose} />;
  const position = item.userData?.position && !item.userData.played ? item.userData.position : undefined;
  const play = (s: Session) =>
    send.mutate(
      { sessionId: s.id, command: { kind: { case: "play", value: { itemIds: [item.id], startPosition: position } } } },
      { onSuccess: () => setControlling(s.id) },
    );
  return (
    <Modal title={t("remote.playOnTitle", { name: item.name })} onClose={onClose}>
      <div className="-mx-3 flex min-h-0 flex-col overflow-y-auto">
        {targets.map((s) => (
          <button
            key={s.id}
            type="button"
            disabled={send.isPending}
            onClick={() => play(s)}
            className="flex min-h-12 w-full items-center gap-3 rounded-lg px-3 py-1.5 text-left hover:bg-fill"
          >
            <Icon name={s.pluginId ? "television" : "devices"} size={20} className="text-text-secondary" />
            <span className="min-w-0 flex-1">
              <span className="block truncate type-body">{deviceName(s)}</span>
              <span className="block truncate type-footnote text-text-secondary">
                {s.nowPlaying?.item ? t("remote.playing", { name: s.nowPlaying.item.name }) : s.client}
              </span>
            </span>
          </button>
        ))}
        {sessions.isSuccess && targets.length === 0 && <p className="px-3 type-callout text-text-secondary">{t("remote.noDevices")}</p>}
      </div>
      {(sessions.error || send.error) && <p className="type-footnote break-words text-danger">{errorMessage(sessions.error ?? send.error)}</p>}
      <div className="flex justify-end">
        <Button variant="secondary" onClick={onClose}>
          {t("action.cancel")}
        </Button>
      </div>
    </Modal>
  );
}

/** Seconds as "1:02:03" or "2:03". */
function clock(ms: number): string {
  const s = Math.max(0, Math.floor(ms / 1000));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const pad = (n: number) => String(n).padStart(2, "0");
  return h ? `${h}:${pad(m)}:${pad(s % 60)}` : `${m}:${pad(s % 60)}`;
}

/** Controls a device: its playback, volume, and a message to show. */
export function RemoteDialog({ server, sessionId, onClose }: { server: ServerEntry; sessionId: string; onClose: () => void }) {
  const sessions = useSessions(server);
  const send = useSendCommand(server);
  const [volume, setVolume] = useState(100);
  const [text, setText] = useState("");
  const [sent, setSent] = useState(false);
  const s = sessions.data?.find((x) => x.id === sessionId);
  const np = s?.nowPlaying;
  const item = np?.item;
  const runtime = item?.runtime ? durationMs(item.runtime) : 0;
  const position = np?.position ? durationMs(np.position) : 0;
  const command = (kind: Parameters<typeof send.mutate>[0]["command"]["kind"]) => send.mutate({ sessionId, command: { kind } });
  const state = (c: PlayStateCommand) => command({ case: "playState", value: { command: c } });
  // The position as the slider is dragged, sent once it is let go.
  const [dragged, setDragged] = useState<number>();
  const seek = (ms: number) => command({ case: "seek", value: { position: durationFromMs(Math.max(0, Math.round(ms))) } });
  return (
    <Modal title={s ? deviceName(s) : t("remote.title")} onClose={onClose}>
      {s && !s.online && <p className="type-callout text-text-secondary">{t("remote.offline")}</p>}
      {s?.online && !item && <p className="type-callout text-text-secondary">{t("remote.idle")}</p>}
      {item && (
        <div className="flex flex-col gap-2">
          <div>
            <p className="truncate type-headline">{item.name}</p>
            {subtitleOf(item) && <p className="truncate type-footnote text-text-secondary">{subtitleOf(item)}</p>}
          </div>
          {runtime > 0 && (
            <div className="flex flex-col gap-1">
              <Slider
                min={0}
                max={runtime}
                step={1000}
                value={Math.min(dragged ?? position, runtime)}
                label={t("remote.position")}
                valueText={clock}
                onChange={setDragged}
                onCommit={(v) => (seek(v), setDragged(undefined))}
              />
              <div className="flex justify-between type-caption text-text-secondary tabular-nums">
                <span>{clock(dragged ?? position)}</span>
                <span>{clock(runtime)}</span>
              </div>
            </div>
          )}
          <div className="flex items-center justify-center gap-1">
            <Control icon="previous" label={t("remote.previous")} onClick={() => state(PlayStateCommand.PREVIOUS)} />
            <Control icon="seekBack" label={t("player.seekBack", { count: 10 })} onClick={() => seek(position - 10_000)} />
            <Control
              icon={np.paused ? "play" : "pause"}
              label={np.paused ? t("player.play") : t("player.pause")}
              large
              onClick={() => state(np.paused ? PlayStateCommand.UNPAUSE : PlayStateCommand.PAUSE)}
            />
            <Control icon="seekForward" label={t("player.seekForward", { count: 10 })} onClick={() => seek(position + 10_000)} />
            <Control icon="next" label={t("remote.next")} onClick={() => state(PlayStateCommand.NEXT)} />
          </div>
          <div className="flex justify-center">
            <Button variant="plain" className="text-danger" onClick={() => state(PlayStateCommand.STOP)}>
              {t("remote.stop")}
            </Button>
          </div>
        </div>
      )}
      {s?.online && (
        <>
          <div className="flex items-center gap-2">
            <Control icon="mute" label={t("player.mute")} onClick={() => command({ case: "volume", value: { muted: true } })} />
            <Slider
              min={0}
              max={100}
              value={volume}
              label={t("remote.volume")}
              className="flex-1"
              onChange={setVolume}
              onCommit={(level) => command({ case: "volume", value: { level, muted: false } })}
            />
            <span className="w-10 text-right type-footnote text-text-secondary tabular-nums">{volume}</span>
          </div>
          <form
            className="flex gap-2"
            onSubmit={(e) => {
              e.preventDefault();
              const m = text.trim();
              if (m) send.mutate({ sessionId, command: { kind: { case: "message", value: { text: m } } } }, { onSuccess: () => (setText(""), setSent(true)) });
            }}
          >
            <input
              className={field}
              aria-label={t("remote.message")}
              placeholder={t("remote.message")}
              maxLength={1000}
              value={text}
              onChange={(e) => (setText(e.target.value), setSent(false))}
            />
            <Button type="submit" variant="secondary" disabled={!text.trim() || send.isPending}>
              {sent ? t("remote.sent") : t("remote.send")}
            </Button>
          </form>
        </>
      )}
      {(sessions.error || send.error) && <p className="type-footnote break-words text-danger">{errorMessage(sessions.error ?? send.error)}</p>}
      <div className="flex justify-end">
        <Button onClick={onClose}>{t("keys.done")}</Button>
      </div>
    </Modal>
  );
}

function Control({ icon, label, large = false, onClick }: { icon: IconName; label: string; large?: boolean; onClick: () => void }) {
  return (
    <Tip label={label}>
      <button
        type="button"
        aria-label={label}
        onClick={onClick}
        className={`flex shrink-0 items-center justify-center rounded-full hover:bg-fill ${large ? "h-14 w-14 bg-fill" : "h-11 w-11"}`}
      >
        <Icon name={icon} fill={large} size={large ? 28 : 22} />
      </button>
    </Tip>
  );
}

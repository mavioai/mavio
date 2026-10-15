// The settings' Libraries (docs/clients.md §6.3.4): the libraries of the
// server chosen, to scan, and a library's page to add, change and delete
// it: its name, kind and folders, chosen from the server's file system,
// with their storage classes, the language of its metadata, how often it
// is scanned and what scans and refreshes make.
import { create } from "@bufbuild/protobuf";
import { durationFromMs, durationMs } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  useDirectory,
  useRemoteFolders,
  useRemotes,
  useStorageStatus,
  useLibraryActions,
  useLocalization,
  useServerLibraries,
} from "@mavio/client-ts";
import { type Library, LibraryKind, LibrarySpecSchema, StorageClass, VirtualEpisodesSchema } from "@mavio/proto/mavio/library/v1/library_pb";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Icon } from "../../icon.tsx";
import { Button, Group, libraryIcon, Row, Select, Toggle } from "../../ui.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { BackLink, ColorTile, Confirm, countryOptions, input, languageOptions, Page } from "./common.tsx";

const kinds: { kind: LibraryKind; title: () => string; color: string }[] = [
  { kind: LibraryKind.MOVIES, title: () => t("libraries.kindMovies"), color: "#FF9500" },
  { kind: LibraryKind.SHOWS, title: () => t("libraries.kindShows"), color: "#007AFF" },
  { kind: LibraryKind.MUSIC, title: () => t("libraries.kindMusic"), color: "#FF2D55" },
  { kind: LibraryKind.MUSIC_VIDEOS, title: () => t("libraries.kindMusicVideos"), color: "#AF52DE" },
  { kind: LibraryKind.HOME_VIDEOS, title: () => t("libraries.kindHomeVideos"), color: "#34C759" },
  { kind: LibraryKind.BOOKS, title: () => t("libraries.kindBooks"), color: "#A2845E" },
  { kind: LibraryKind.PHOTOS, title: () => t("libraries.kindPhotos"), color: "#32ADE6" },
  { kind: LibraryKind.MIXED, title: () => t("libraries.kindMixed"), color: "#8E8E93" },
];

const kindOf = (k: LibraryKind) => kinds.find((x) => x.kind === k) ?? kinds[kinds.length - 1];

const video = new Set([LibraryKind.MOVIES, LibraryKind.SHOWS, LibraryKind.MUSIC_VIDEOS, LibraryKind.HOME_VIDEOS, LibraryKind.MIXED]);
const music = new Set([LibraryKind.MUSIC, LibraryKind.MIXED]);
const movies = new Set([LibraryKind.MOVIES, LibraryKind.MIXED]);
const shows = new Set([LibraryKind.SHOWS, LibraryKind.MIXED]);

/** The libraries administrators manage: not those of collections and playlists. */
const managed = (l: Library) => l.kind !== LibraryKind.COLLECTIONS && l.kind !== LibraryKind.PLAYLISTS;

const hour = 3_600_000;
const intervals: { ms: number; title: () => string }[] = [
  { ms: 0, title: () => t("libraries.scanNever") },
  { ms: hour, title: () => t("libraries.scanHourly") },
  { ms: 6 * hour, title: () => t("libraries.scanEveryHours", { hours: 6 }) },
  { ms: 12 * hour, title: () => t("libraries.scanEveryHours", { hours: 12 }) },
  { ms: 24 * hour, title: () => t("libraries.scanDaily") },
  { ms: 7 * 24 * hour, title: () => t("libraries.scanWeekly") },
];

const storageClasses: { value: StorageClass; title: () => string }[] = [
  { value: StorageClass.LOCAL, title: () => t("libraries.storageLocal") },
  { value: StorageClass.NETWORK, title: () => t("libraries.storageNetwork") },
  { value: StorageClass.CLOUD, title: () => t("libraries.storageCloud") },
];

/** The section of the server chosen in the settings. */
export function LibrariesSection({ server }: { server: ServerEntry }) {
  const search = useSearch({ strict: false }) as SettingsSearch;
  if (search.library) return <LibraryPage key={search.library} server={server} id={search.library} />;
  return <LibraryList server={server} />;
}

function LibraryList({ server }: { server: ServerEntry }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const libraries = useServerLibraries(server);
  const list = (libraries.data ?? []).filter(managed).sort((a, b) => a.name.localeCompare(b.name, locale));
  return (
    <Page
      title={t("settings.libraries")}
      message={t("libraries.message")}
      actions={
        <Link
          to="/settings/$section"
          params={{ section: "libraries" }}
          search={{ server: cur.server, library: "new" }}
          className="inline-flex min-h-11 items-center gap-2 rounded-md bg-button px-4 type-headline text-on-button hover:opacity-85"
        >
          <Icon name="add" size={18} />
          {t("libraries.add")}
        </Link>
      }
    >
      {libraries.isError && <p className="type-callout text-danger">{errorMessage(libraries.error)}</p>}
      {libraries.isSuccess && list.length === 0 && <p className="type-body text-text-secondary">{t("libraries.none")}</p>}
      {list.length > 0 && (
        <Group>
          {list.map((l) => (
            <LibraryRow key={l.id} server={server} library={l} />
          ))}
        </Group>
      )}
    </Page>
  );
}

function LibraryRow({ server, library: l }: { server: ServerEntry; library: Library }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const actions = useLibraryActions(server);
  const [scanned, setScanned] = useState<string>();
  const k = kindOf(l.kind);
  const scanning = actions.scan.isPending && actions.scan.variables === l.id;
  return (
    <div className="flex min-h-13 items-center gap-3 px-4 py-2.5">
      <Link
        to="/settings/$section"
        params={{ section: "libraries" }}
        search={{ server: cur.server, library: l.id }}
        className="flex min-w-0 flex-1 items-center gap-3"
      >
        <ColorTile icon={libraryIcon(l.kind)} color={k.color} />
        <div className="min-w-0 flex-1">
          <div className="truncate type-body">{l.name}</div>
          <div className="truncate type-footnote text-text-secondary">
            {scanned ?? [k.title(), t("libraries.folderCount", { count: l.paths.length })].join(" · ")}
          </div>
        </div>
      </Link>
      <Button
        variant="secondary"
        disabled={scanning}
        onClick={() =>
          actions.scan.mutate(l.id, {
            onSuccess: (r) => setScanned(r.enqueued ? t("libraries.scanQueued") : t("libraries.scanPending")),
            onError: (e) => setScanned(errorMessage(e)),
          })
        }
      >
        {scanning ? "…" : t("libraries.scan")}
      </Button>
    </div>
  );
}

/** A library as edited: the fields of LibrarySpec the page shows. */
interface Draft {
  name: string;
  kind: LibraryKind;
  paths: string[];
  storageClasses: Record<string, StorageClass>;
  scanMs: number;
  language: string;
  country: string;
  saveLocalMetadata: boolean;
  autoCollections: boolean;
  extractTrickplay: boolean;
  extractChapterImages: boolean;
  analyzeLoudness: boolean;
  downloadLyrics: boolean;
  missing: boolean;
  unaired: boolean;
  specials: boolean;
  graceDays: number;
}

function draftOf(l?: Library): Draft {
  return {
    name: l?.name ?? "",
    kind: l?.kind ?? LibraryKind.MOVIES,
    paths: l?.paths ?? [],
    storageClasses: { ...l?.storageClasses },
    scanMs: l ? (l.scanInterval ? durationMs(l.scanInterval) : 0) : 12 * hour,
    language: l?.preferredLanguage ?? "",
    country: l?.metadataCountry ?? "",
    saveLocalMetadata: l?.saveLocalMetadata ?? false,
    autoCollections: l?.autoCollections ?? false,
    extractTrickplay: l?.extractTrickplay ?? false,
    extractChapterImages: l?.extractChapterImages ?? false,
    analyzeLoudness: l?.analyzeLoudness ?? false,
    downloadLyrics: l?.downloadLyrics ?? false,
    missing: l?.virtualEpisodes?.missing ?? false,
    unaired: l?.virtualEpisodes?.unaired ?? false,
    specials: l?.virtualEpisodes?.specials ?? false,
    graceDays: l?.virtualEpisodes?.graceDays ?? 0,
  };
}

/** The spec of a draft, keeping what the page does not show of the library. */
function specOf(d: Draft, l?: Library) {
  const classes = Object.fromEntries(Object.entries(d.storageClasses).filter(([p, c]) => d.paths.includes(p) && c !== StorageClass.UNSPECIFIED));
  return create(LibrarySpecSchema, {
    name: d.name.trim(),
    kind: d.kind,
    paths: d.paths,
    scanInterval: d.scanMs > 0 ? durationFromMs(d.scanMs) : undefined,
    preferredLanguage: d.language,
    metadataCountry: d.country,
    saveLocalMetadata: d.saveLocalMetadata,
    autoCollections: d.autoCollections,
    extractTrickplay: d.extractTrickplay,
    extractChapterImages: d.extractChapterImages,
    analyzeLoudness: d.analyzeLoudness,
    downloadLyrics: d.downloadLyrics,
    providerOrder: l?.providerOrder,
    virtualEpisodes: create(VirtualEpisodesSchema, { missing: d.missing, unaired: d.unaired, specials: d.specials, graceDays: d.graceDays }),
    hubs: l?.hubs,
    storageClasses: classes,
  });
}

/** A library's page, or that of a new one when id is "new". */
function LibraryPage({ server, id }: { server: ServerEntry; id: string }) {
  const libraries = useServerLibraries(server);
  const library = libraries.data?.find((l) => l.id === id);
  if (id !== "new" && !library) {
    return (
      <div className="flex flex-col gap-7">
        <BackLink section="libraries" label={t("settings.libraries")} />
        {libraries.isSuccess && <p className="type-body text-text-secondary">{t("libraries.notFound")}</p>}
        {libraries.isError && <p className="type-callout text-danger">{errorMessage(libraries.error)}</p>}
      </div>
    );
  }
  return <LibraryForm server={server} library={library} />;
}

function LibraryForm({ server, library }: { server: ServerEntry; library?: Library }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const navigate = useNavigate();
  const actions = useLibraryActions(server);
  const { languages, countries } = useLocalization(server);
  const [draft, setDraft] = useState(() => draftOf(library));
  const [picking, setPicking] = useState(library === undefined);
  const [typed, setTyped] = useState("");
  const [confirming, setConfirming] = useState(false);
  const [notice, setNotice] = useState<string>();
  const set = (patch: Partial<Draft>) => (setDraft((d) => ({ ...d, ...patch })), setNotice(undefined));
  const changed = JSON.stringify(draft) !== JSON.stringify(draftOf(library));
  const valid = draft.name.trim() !== "" && draft.paths.length > 0;
  const saving = actions.create.isPending || actions.update.isPending;
  const failure = [actions.create, actions.update, actions.remove, actions.scan].find((x) => x.isError)?.error;
  const addPath = (p: string) => {
    const path = p.trim().replace(/(.)\/+$/, "$1");
    if (path && !draft.paths.includes(path)) set({ paths: [...draft.paths, path] });
  };
  const save = () => {
    if (!library) {
      actions.create.mutate(specOf(draft), {
        onSuccess: (l) =>
          l && void navigate({ to: "/settings/$section", params: { section: "libraries" }, search: { server: cur.server, library: l.id } }),
      });
      return;
    }
    actions.update.mutate({ id: library.id, spec: specOf(draft, library) }, { onSuccess: () => setNotice(t("libraries.saved")) });
  };
  const toggle = (key: keyof Draft, label: string, detail?: string) => (
    <Row key={key} label={label} detail={detail}>
      <Toggle on={draft[key] as boolean} label={label} onChange={(on) => set({ [key]: on })} />
    </Row>
  );
  const k = kindOf(draft.kind);
  return (
    <Page
      back={<BackLink section="libraries" label={t("settings.libraries")} />}
      title={
        <span className="flex items-center gap-3">
          <ColorTile icon={libraryIcon(draft.kind)} color={k.color} size={40} />
          {library ? library.name : t("libraries.add")}
        </span>
      }
      actions={
        library && (
          <>
            <Button
              variant="secondary"
              disabled={actions.scan.isPending}
              onClick={() => actions.scan.mutate(library.id, { onSuccess: (r) => setNotice(r.enqueued ? t("libraries.scanQueued") : t("libraries.scanPending")) })}
            >
              {t("libraries.scan")}
            </Button>
            <Button variant="secondary" className="text-danger" onClick={() => setConfirming(true)}>
              {t("action.delete")}
            </Button>
          </>
        )
      }
    >
      {confirming && library && (
        <Confirm
          title={t("libraries.deleteTitle", { name: library.name })}
          message={t("libraries.deleteMessage")}
          action={t("action.delete")}
          busy={actions.remove.isPending}
          onCancel={() => setConfirming(false)}
          onConfirm={() =>
            actions.remove.mutate(library.id, {
              onSuccess: () => void navigate({ to: "/settings/$section", params: { section: "libraries" }, search: { server: cur.server } }),
              onSettled: () => setConfirming(false),
            })
          }
        />
      )}
      <Group>
        <Row label={t("libraries.name")}>
          <input
            className={`${input} sm:w-72`}
            aria-label={t("libraries.name")}
            value={draft.name}
            onChange={(e) => set({ name: e.target.value })}
          />
        </Row>
        <Row label={t("libraries.kind")}>
          <Select
            value={String(draft.kind)}
            label={t("libraries.kind")}
            options={kinds.map((x) => ({ value: String(x.kind), label: x.title() }))}
            onChange={(v) => set({ kind: Number(v) as LibraryKind })}
          />
        </Row>
      </Group>
      <Group title={t("libraries.folders")} footer={t("libraries.foldersFooter")}>
        {draft.paths.map((p) => {
          const effective = library?.effectiveStorageClasses[p];
          const auto = storageClasses.find((c) => c.value === effective);
          return (
            <Row key={p} label={<span className="break-all">{p}</span>}>
              <Select
                value={String(draft.storageClasses[p] ?? StorageClass.UNSPECIFIED)}
                label={t("libraries.storageClass")}
                options={[
                  {
                    value: String(StorageClass.UNSPECIFIED),
                    label: auto ? t("libraries.storageAutoIs", { storage: auto.title() }) : t("libraries.storageAuto"),
                  },
                  ...storageClasses.map((c) => ({ value: String(c.value), label: c.title() })),
                ]}
                onChange={(v) => set({ storageClasses: { ...draft.storageClasses, [p]: Number(v) as StorageClass } })}
              />
              <button
                type="button"
                aria-label={t("libraries.removeFolder")}
                title={t("libraries.removeFolder")}
                onClick={() => set({ paths: draft.paths.filter((o) => o !== p) })}
                className="flex size-8 items-center justify-center rounded-full text-text-secondary hover:bg-fill hover:text-danger"
              >
                <Icon name="close" size={16} />
              </button>
            </Row>
          );
        })}
        <form
          className="flex min-h-13 flex-wrap items-center gap-3 px-4 py-2.5"
          onSubmit={(e) => (e.preventDefault(), addPath(typed), setTyped(""))}
        >
          <input
            className={`${input} flex-1 basis-56`}
            aria-label={t("libraries.folderPath")}
            placeholder={t("libraries.folderPath")}
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
          />
          <Button type="submit" variant="secondary" disabled={!typed.trim()}>
            {t("libraries.addFolder")}
          </Button>
          <Button variant="plain" onClick={() => setPicking(!picking)}>
            <Icon name="folder" size={18} />
            {t("libraries.browse")}
          </Button>
        </form>
        {picking && <FolderPicker server={server} chosen={draft.paths} onChoose={(p) => addPath(p)} />}
      </Group>
      <Group title={t("libraries.metadata")}>
        <Row label={t("libraries.language")}>
          <Select
            value={draft.language}
            label={t("libraries.language")}
            className="w-48"
            options={[{ value: "", label: t("libraries.serverDefault") }, ...languageOptions(languages)]}
            onChange={(language) => set({ language })}
          />
        </Row>
        <Row label={t("libraries.country")}>
          <Select
            value={draft.country}
            label={t("libraries.country")}
            className="w-48"
            options={[{ value: "", label: t("libraries.serverDefault") }, ...countryOptions(countries)]}
            onChange={(country) => set({ country })}
          />
        </Row>
        {toggle("saveLocalMetadata", t("libraries.saveLocalMetadata"), t("libraries.saveLocalMetadataDetail"))}
        {movies.has(draft.kind) && toggle("autoCollections", t("libraries.autoCollections"), t("libraries.autoCollectionsDetail"))}
        {music.has(draft.kind) && toggle("downloadLyrics", t("libraries.downloadLyrics"))}
      </Group>
      <Group title={t("libraries.scanning")}>
        <Row label={t("libraries.scanInterval")}>
          <Select
            value={String(draft.scanMs)}
            label={t("libraries.scanInterval")}
            options={[
              ...intervals.map((i) => ({ value: String(i.ms), label: i.title() })),
              ...(intervals.some((i) => i.ms === draft.scanMs) ? [] : [{ value: String(draft.scanMs), label: t("libraries.scanEveryHours", { hours: draft.scanMs / hour }) }]),
            ]}
            onChange={(v) => set({ scanMs: Number(v) })}
          />
        </Row>
        {video.has(draft.kind) && toggle("extractTrickplay", t("libraries.trickplay"), t("libraries.trickplayDetail"))}
        {video.has(draft.kind) && toggle("extractChapterImages", t("libraries.chapterImages"))}
        {music.has(draft.kind) && toggle("analyzeLoudness", t("libraries.loudness"), t("libraries.loudnessDetail"))}
      </Group>
      {shows.has(draft.kind) && (
        <Group title={t("libraries.missingEpisodes")} footer={t("libraries.missingEpisodesFooter")}>
          {toggle("missing", t("libraries.missingAired"))}
          {toggle("unaired", t("libraries.missingUnaired"))}
          {toggle("specials", t("libraries.missingSpecials"))}
          {!draft.missing && (
            <Row label={t("libraries.graceDays")} detail={t("libraries.graceDaysDetail")}>
              <input
                type="number"
                min={0}
                max={3650}
                className={`${input} w-24`}
                aria-label={t("libraries.graceDays")}
                value={draft.graceDays}
                onChange={(e) => set({ graceDays: Math.max(0, Math.min(3650, Math.trunc(Number(e.target.value) || 0))) })}
              />
            </Row>
          )}
        </Group>
      )}
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={!changed || !valid || saving} onClick={save}>
          {saving ? "…" : library ? t("action.save") : t("libraries.create")}
        </Button>
        {notice && !changed && <span className="type-callout text-text-secondary">{notice}</span>}
        {failure && <span className="type-callout text-danger">{errorMessage(failure)}</span>}
      </div>
    </Page>
  );
}

/** The folders of the server's file system and of its rclone remotes, to walk and choose from. */
function FolderPicker({ server, chosen, onChoose }: { server: ServerEntry; chosen: string[]; onChoose: (path: string) => void }) {
  const [path, setPath] = useState("");
  const remote = path.startsWith("rclone:");
  const dir = useDirectory(server, path, !remote);
  const storage = useStorageStatus(server);
  const remotes = useRemotes(server, path === "" && (storage.data?.available ?? false));
  const remoteFolders = useRemoteFolders(server, path, remote);
  const listing = remote ? remoteFolders : dir;
  const folders = remote
    ? (remoteFolders.data ?? []).map((f) => ({ path: f, name: f.slice(f.lastIndexOf(f.includes("/") ? "/" : ":") + 1) }))
    : [
        ...(dir.data?.entries ?? []).filter((e) => e.isDir),
        ...(path === "" ? (remotes.data ?? []).map((r) => ({ path: `rclone:${r.name}:`, name: `rclone:${r.name}:` })) : []),
      ];
  const here = remote ? path : (dir.data?.path ?? path);
  const parent = remote ? (/^rclone:[^:]*:$/.test(path) ? "" : path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : path.slice(0, path.indexOf(":", 7) + 1)) : (dir.data?.parent ?? "");
  const row = (key: string, label: ReactNode, onClick: () => void, icon: "back" | "folder") => (
    <button key={key} type="button" onClick={onClick} className="flex min-h-11 w-full items-center gap-3 px-4 text-left hover:bg-surface-raised">
      <Icon name={icon} size={18} className="shrink-0 text-text-secondary" />
      <span className="min-w-0 flex-1 truncate type-body">{label}</span>
    </button>
  );
  return (
    <div className="flex flex-col bg-background/40">
      <div className="flex min-h-13 flex-wrap items-center gap-3 px-4 py-2.5">
        <span className="min-w-0 flex-1 break-all type-callout text-text-secondary">{here || t("libraries.roots")}</span>
        <Button variant="secondary" disabled={!here || chosen.includes(here)} onClick={() => onChoose(here)}>
          {t("libraries.chooseFolder")}
        </Button>
      </div>
      <div className="max-h-80 divide-y divide-border overflow-y-auto border-t border-border">
        {here && row("..", t("libraries.parentFolder"), () => setPath(parent), "back")}
        {folders.map((f) => row(f.path, f.name, () => setPath(f.path), "folder"))}
        {listing.isSuccess && folders.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("libraries.noFolders")}</p>}
        {listing.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(listing.error)}</p>}
      </div>
    </div>
  );
}

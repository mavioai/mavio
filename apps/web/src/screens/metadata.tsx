// An item's metadata from its More menu, for administrators (docs/clients.md
// §6.3.22): its fields edited and locked, the item identified anew from the
// providers' search results, its metadata refreshed, and its images chosen
// from the providers' or uploaded.
import { durationFromMs, durationMs, timestampDate, timestampFromDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  imageUrl,
  type ServerEntry,
  useExternalIdKinds,
  useMetadataActions,
  useRemoteImages,
  useRemoteSearch,
} from "@mavio/client-ts";
import { type Image, ImageKind, type Item, ItemKind, MetadataField, SeriesStatus } from "@mavio/proto/mavio/library/v1/item_pb";
import type { RemoteSearchResult } from "@mavio/proto/mavio/library/v1/metadata_pb";
import { type ReactNode, useState } from "react";

import { t } from "../i18n.ts";
import { Button } from "../ui.tsx";
import { NumberField, Switch } from "../controls/inputs.tsx";
import { Select } from "../controls/select.tsx";
import { field } from "../controls/styles.ts";
import { Modal } from "../controls/dialog.tsx";

/** Kinds the providers identify. */
export const identifiable = (kind: ItemKind) =>
  [ItemKind.MOVIE, ItemKind.SERIES, ItemKind.MUSIC_ARTIST, ItemKind.MUSIC_ALBUM, ItemKind.MUSIC_VIDEO, ItemKind.BOOK, ItemKind.AUDIOBOOK].includes(kind);

const music = (kind: ItemKind) => [ItemKind.MUSIC_ALBUM, ItemKind.TRACK, ItemKind.MUSIC_VIDEO].includes(kind);

/** The editable fields as the form holds them: text, and toggles. */
interface Form {
  name: string;
  original_title: string;
  sort_name: string;
  overview: string;
  tagline: string;
  production_year: string;
  premiere_date: string;
  end_date: string;
  runtime: string;
  official_rating: string;
  custom_rating: string;
  community_rating: string;
  critic_rating: string;
  genres: string;
  tags: string;
  studios: string;
  production_locations: string;
  artists: string;
  album_artists: string;
  index_number: string;
  parent_index_number: string;
  absolute_number: string;
  series_status: string;
  external_ids: Record<string, string>;
  locked: boolean;
  locked_fields: MetadataField[];
}

const list = (v: string[]) => v.join(", ");
const unlist = (v: string) =>
  v
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
const day = (ts?: Parameters<typeof timestampDate>[0]) => (ts ? timestampDate(ts).toISOString().slice(0, 10) : "");
const num = (n: number) => (n ? String(n) : "");

function formOf(item: Item): Form {
  return {
    name: item.name,
    original_title: item.originalTitle,
    sort_name: item.sortName,
    overview: item.overview,
    tagline: item.tagline,
    production_year: num(item.productionYear),
    premiere_date: day(item.premiereDate),
    end_date: day(item.endDate),
    runtime: item.runtime ? String(Math.round(durationMs(item.runtime) / 60000)) : "",
    official_rating: item.officialRating,
    custom_rating: item.customRating,
    community_rating: num(item.communityRating),
    critic_rating: num(item.criticRating),
    genres: list(item.genres),
    tags: list(item.tags),
    studios: list(item.studios),
    production_locations: list(item.productionLocations),
    artists: list(item.artists),
    album_artists: list(item.albumArtists),
    index_number: item.indexNumber ? String(item.indexNumber) : "",
    parent_index_number: item.parentIndexNumber ? String(item.parentIndexNumber) : "",
    absolute_number: item.absoluteNumber ? String(item.absoluteNumber) : "",
    series_status: String(item.seriesStatus),
    external_ids: { ...item.externalIds },
    locked: item.locked,
    locked_fields: [...item.lockedFields],
  };
}

const optionalInt = (v: string) => (v.trim() === "" ? undefined : Number.parseInt(v, 10));
const date = (v: string) => (v ? timestampFromDate(new Date(`${v}T00:00:00Z`)) : undefined);

/** The form's changed fields as ItemMetadata and its update mask. */
function changesOf(before: Form, after: Form) {
  const paths = (Object.keys(after) as (keyof Form)[]).filter((k) => JSON.stringify(before[k]) !== JSON.stringify(after[k]));
  const metadata = {
    name: after.name.trim(),
    originalTitle: after.original_title.trim(),
    sortName: after.sort_name.trim(),
    overview: after.overview.trim(),
    tagline: after.tagline.trim(),
    productionYear: Number.parseInt(after.production_year, 10) || 0,
    premiereDate: date(after.premiere_date),
    endDate: date(after.end_date),
    runtime: after.runtime ? durationFromMs(Number(after.runtime) * 60000) : undefined,
    officialRating: after.official_rating.trim(),
    customRating: after.custom_rating.trim(),
    communityRating: Number(after.community_rating) || 0,
    criticRating: Number(after.critic_rating) || 0,
    genres: unlist(after.genres),
    tags: unlist(after.tags),
    studios: unlist(after.studios),
    productionLocations: unlist(after.production_locations),
    artists: unlist(after.artists),
    albumArtists: unlist(after.album_artists),
    indexNumber: optionalInt(after.index_number),
    parentIndexNumber: optionalInt(after.parent_index_number),
    absoluteNumber: optionalInt(after.absolute_number),
    seriesStatus: Number(after.series_status) as SeriesStatus,
    externalIds: Object.fromEntries(Object.entries(after.external_ids).map(([k, v]) => [k, v.trim()])),
    locked: after.locked,
    lockedFields: after.locked_fields,
  };
  return { metadata, paths };
}

const lockable: [MetadataField, () => string][] = [
  [MetadataField.NAME, () => t("metadata.lockName")],
  [MetadataField.OVERVIEW, () => t("metadata.overview")],
  [MetadataField.GENRES, () => t("metadata.genres")],
  [MetadataField.TAGS, () => t("metadata.tags")],
  [MetadataField.STUDIOS, () => t("metadata.studios")],
  [MetadataField.PRODUCTION_LOCATIONS, () => t("metadata.locations")],
  [MetadataField.OFFICIAL_RATING, () => t("metadata.officialRating")],
  [MetadataField.RUNTIME, () => t("metadata.runtime")],
  [MetadataField.CAST, () => t("metadata.cast")],
];

/** A labelled field of the edit form. */
function Labelled({ label, wide = false, children }: { label: string; wide?: boolean; children: ReactNode }) {
  return (
    <label className={`flex min-w-0 flex-col gap-1 ${wide ? "sm:col-span-2" : ""}`}>
      <span className="type-footnote text-text-secondary">{label}</span>
      {children}
    </label>
  );
}

/** Edits the item's fields and locks; saves only those changed. */
export function EditDialog({ server, item, onClose }: { server: ServerEntry; item: Item; onClose: () => void }) {
  const { update } = useMetadataActions(server, item.id);
  const kinds = useExternalIdKinds(server, item.kind);
  const [before] = useState(() => formOf(item));
  const [form, setForm] = useState(before);
  const set = <K extends keyof Form>(k: K, v: Form[K]) => setForm((f) => ({ ...f, [k]: v }));
  const text = (k: keyof Form, label: string, opts: { wide?: boolean; type?: string; placeholder?: string } = {}) => (
    <Labelled label={label} wide={opts.wide}>
      <input
        className={field}
        type={opts.type ?? "text"}
        placeholder={opts.placeholder}
        value={form[k] as string}
        onChange={(e) => set(k, e.target.value as never)}
      />
    </Labelled>
  );
  const k = item.kind;
  const video = k === ItemKind.MOVIE || k === ItemKind.EPISODE || k === ItemKind.VIDEO || k === ItemKind.MUSIC_VIDEO;
  const { metadata, paths } = changesOf(before, form);
  return (
    <Modal title={t("metadata.editTitle", { name: item.name })} wide onClose={onClose}>
      <form
        className="-mx-6 flex min-h-0 flex-col gap-3 overflow-y-auto px-6"
        onSubmit={(e) => {
          e.preventDefault();
          if (paths.length === 0) onClose();
          else update.mutate({ metadata, paths }, { onSuccess: onClose });
        }}
      >
        <div className="grid gap-3 sm:grid-cols-2">
          {text("name", t("metadata.name"), { wide: true })}
          {text("original_title", t("metadata.originalTitle"))}
          {text("sort_name", t("metadata.sortName"), { placeholder: form.name })}
          <Labelled label={t("metadata.overview")} wide>
            <textarea className={`${field} min-h-28 py-2`} value={form.overview} onChange={(e) => set("overview", e.target.value)} />
          </Labelled>
          {(k === ItemKind.MOVIE || k === ItemKind.SERIES) && text("tagline", t("metadata.tagline"), { wide: true })}
          {k === ItemKind.SEASON && text("index_number", t("metadata.seasonNumber"), { type: "number" })}
          {k === ItemKind.EPISODE && text("parent_index_number", t("metadata.seasonNumber"), { type: "number" })}
          {(k === ItemKind.EPISODE || k === ItemKind.TRACK) && text("index_number", k === ItemKind.TRACK ? t("metadata.trackNumber") : t("metadata.episodeNumber"), { type: "number" })}
          {k === ItemKind.EPISODE && text("absolute_number", t("metadata.absoluteNumber"), { type: "number" })}
          {text("production_year", t("metadata.year"), { type: "number" })}
          {text("premiere_date", k === ItemKind.SERIES ? t("metadata.firstAired") : t("metadata.releaseDate"), { type: "date" })}
          {k === ItemKind.SERIES && text("end_date", t("metadata.endDate"), { type: "date" })}
          {k === ItemKind.SERIES && (
            <Labelled label={t("metadata.status")}>
              <Select
                value={form.series_status}
                label={t("metadata.status")}
                options={[
                  { value: String(SeriesStatus.UNSPECIFIED), label: t("metadata.statusUnknown") },
                  { value: String(SeriesStatus.CONTINUING), label: t("metadata.continuing") },
                  { value: String(SeriesStatus.ENDED), label: t("metadata.ended") },
                  { value: String(SeriesStatus.UNRELEASED), label: t("metadata.unreleased") },
                ]}
                onChange={(v) => set("series_status", v)}
              />
            </Labelled>
          )}
          {video && text("runtime", t("metadata.runtimeMinutes"), { type: "number" })}
          {text("official_rating", t("metadata.officialRating"))}
          {text("custom_rating", t("metadata.customRating"))}
          {text("community_rating", t("metadata.communityRating"), { type: "number" })}
          {text("critic_rating", t("metadata.criticRating"), { type: "number" })}
          {music(k) && text("artists", t("metadata.artists"), { wide: true, placeholder: t("metadata.commas") })}
          {k === ItemKind.MUSIC_ALBUM && text("album_artists", t("metadata.albumArtists"), { wide: true, placeholder: t("metadata.commas") })}
          {text("genres", t("metadata.genres"), { wide: true, placeholder: t("metadata.commas") })}
          {text("tags", t("metadata.tags"), { wide: true, placeholder: t("metadata.commas") })}
          {text("studios", t("metadata.studios"), { wide: true, placeholder: t("metadata.commas") })}
          {text("production_locations", t("metadata.locations"), { wide: true, placeholder: t("metadata.commas") })}
        </div>
        {(kinds.data ?? []).length > 0 && (
          <>
            <h3 className="pt-2 type-headline">{t("metadata.externalIds")}</h3>
            <div className="grid gap-3 sm:grid-cols-2">
              {(kinds.data ?? []).map((x) => (
                <Labelled key={x.key} label={x.name}>
                  <input
                    className={field}
                    value={form.external_ids[x.key] ?? ""}
                    onChange={(e) => set("external_ids", { ...form.external_ids, [x.key]: e.target.value })}
                  />
                </Labelled>
              ))}
            </div>
          </>
        )}
        <h3 className="pt-2 type-headline">{t("metadata.locks")}</h3>
        <div className="flex items-center justify-between gap-3">
          <span className="type-callout">{t("metadata.lockAll")}</span>
          <Switch on={form.locked} onChange={(on) => set("locked", on)} label={t("metadata.lockAll")} />
        </div>
        {!form.locked && (
          <div className="grid gap-x-6 gap-y-2 sm:grid-cols-2">
            {lockable.map(([f, label]) => (
              <div key={f} className="flex items-center justify-between gap-3">
                <span className="type-callout">{label()}</span>
                <Switch
                  on={form.locked_fields.includes(f)}
                  onChange={(on) => set("locked_fields", on ? [...form.locked_fields, f] : form.locked_fields.filter((x) => x !== f))}
                  label={label()}
                />
              </div>
            ))}
          </div>
        )}
        <p className="type-footnote text-text-secondary">{t("metadata.locksFooter")}</p>
        {update.isError && <p className="type-footnote break-words text-danger">{errorMessage(update.error)}</p>}
        <div className="sticky bottom-0 flex justify-end gap-2 bg-surface-overlay pt-2">
          <Button variant="secondary" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button type="submit" disabled={update.isPending || !form.name.trim()}>
            {update.isPending ? "…" : t("metadata.save")}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

/** Searches the providers for what the item is, and identifies it as a result. */
export function IdentifyDialog({ server, item, onClose }: { server: ServerEntry; item: Item; onClose: () => void }) {
  const { identify } = useMetadataActions(server, item.id);
  const [name, setName] = useState(item.name);
  const [year, setYear] = useState(item.productionYear ? String(item.productionYear) : "");
  const [query, setQuery] = useState<{ name: string; year?: number }>({ name: item.name, year: item.productionYear || undefined });
  const search = useRemoteSearch(server, item.id, query);
  const results = search.data ?? [];
  return (
    <Modal title={t("metadata.identifyTitle", { name: item.name })} wide onClose={onClose}>
      <form
        className="flex flex-wrap gap-2"
        onSubmit={(e) => {
          e.preventDefault();
          setQuery({ name: name.trim(), year: Number.parseInt(year, 10) || undefined });
        }}
      >
        <input className={`${field} basis-60`} aria-label={t("metadata.name")} placeholder={t("metadata.name")} value={name} onChange={(e) => setName(e.target.value)} />
        <NumberField
          className="w-36 flex-none"
          label={t("metadata.year")}
          placeholder={t("metadata.year")}
          min={1800}
          max={3000}
          value={Number.parseInt(year, 10) || undefined}
          onChange={(v) => setYear(v === undefined ? "" : String(v))}
        />
        <Button type="submit" variant="secondary" disabled={!name.trim()}>
          {t("metadata.search")}
        </Button>
      </form>
      <div className="-mx-6 min-h-0 flex-1 divide-y divide-border overflow-y-auto border-t border-border">
        {search.isFetching && <p className="px-6 py-4 type-callout text-text-secondary">{t("metadata.searching")}</p>}
        {search.isError && <p className="px-6 py-4 type-callout break-words text-danger">{errorMessage(search.error)}</p>}
        {search.isSuccess && !search.isFetching && results.length === 0 && (
          <p className="px-6 py-4 type-callout text-text-secondary">{t("metadata.noResults")}</p>
        )}
        {!search.isFetching &&
          results.map((r, i) => (
            <ResultRow
              key={`${r.provider}-${i}`}
              result={r}
              busy={identify.isPending}
              onIdentify={() => identify.mutate(r.externalIds, { onSuccess: onClose })}
            />
          ))}
      </div>
      {identify.isError && <p className="type-footnote break-words text-danger">{errorMessage(identify.error)}</p>}
      <div className="flex justify-end">
        <Button variant="secondary" onClick={onClose}>
          {t("action.cancel")}
        </Button>
      </div>
    </Modal>
  );
}

function ResultRow({ result: r, busy, onIdentify }: { result: RemoteSearchResult; busy: boolean; onIdentify: () => void }) {
  const ids = Object.entries(r.externalIds).map(([k, v]) => `${k} ${v}`);
  return (
    <div className="flex items-start gap-3 px-6 py-3">
      <div className="aspect-[2/3] w-14 shrink-0 overflow-hidden rounded-md bg-fill">
        {r.imageUrl && <img src={r.imageUrl} alt="" loading="lazy" className="h-full w-full object-cover" />}
      </div>
      <div className="flex min-w-0 flex-1 flex-col gap-0.5">
        <p className="type-headline">
          {r.name} {r.year > 0 && <span className="type-callout text-text-secondary">({r.year})</span>}
        </p>
        <p className="type-footnote break-all text-text-secondary">{[r.provider, ...ids].join(" · ")}</p>
        {r.overview && <p className="type-footnote line-clamp-2 text-text-secondary">{r.overview}</p>}
      </div>
      <Button variant="secondary" disabled={busy} onClick={onIdentify}>
        {t("metadata.identify")}
      </Button>
    </div>
  );
}

/** Refreshes the item's metadata, filling what is missing or replacing all of it. */
export function RefreshDialog({ server, item, onClose }: { server: ServerEntry; item: Item; onClose: () => void }) {
  const { refresh } = useMetadataActions(server, item.id);
  const [replace, setReplace] = useState(false);
  return (
    <Modal title={t("metadata.refreshTitle", { name: item.name })} onClose={onClose}>
      <div className="flex items-center justify-between gap-3">
        <span className="type-callout">{t("metadata.replace")}</span>
        <Switch on={replace} onChange={setReplace} label={t("metadata.replace")} />
      </div>
      <p className="type-footnote text-text-secondary">{replace ? t("metadata.replaceFooter") : t("metadata.refreshFooter")}</p>
      {refresh.isError && <p className="type-footnote break-words text-danger">{errorMessage(refresh.error)}</p>}
      <div className="flex justify-end gap-2">
        <Button variant="secondary" onClick={onClose}>
          {t("action.cancel")}
        </Button>
        <Button disabled={refresh.isPending} onClick={() => refresh.mutate(replace, { onSuccess: onClose })}>
          {refresh.isPending ? t("metadata.refreshing") : t("metadata.refresh")}
        </Button>
      </div>
    </Modal>
  );
}

const imageKinds: [ImageKind, () => string][] = [
  [ImageKind.PRIMARY, () => t("metadata.primary")],
  [ImageKind.BACKDROP, () => t("metadata.backdrop")],
  [ImageKind.LOGO, () => t("metadata.logo")],
  [ImageKind.THUMB, () => t("metadata.thumb")],
  [ImageKind.BANNER, () => t("metadata.banner")],
  [ImageKind.ART, () => t("metadata.art")],
  [ImageKind.DISC, () => t("metadata.disc")],
];

/** The item's images of a kind, deleted, replaced from the providers' or by an upload. */
export function ImagesDialog({ server, item, onClose }: { server: ServerEntry; item: Item; onClose: () => void }) {
  const [kind, setKind] = useState(ImageKind.PRIMARY);
  const actions = useMetadataActions(server, item.id);
  const remote = useRemoteImages(server, item.id, kind);
  const current = item.images.filter((i) => i.kind === kind && i.url);
  const wide = kind === ImageKind.BACKDROP || kind === ImageKind.THUMB || kind === ImageKind.BANNER || kind === ImageKind.LOGO || kind === ImageKind.ART;
  const error = actions.setImage.error ?? actions.deleteImage.error;
  const upload = async (file?: File) => {
    if (file) actions.setImage.mutate({ kind, source: { case: "data", value: new Uint8Array(await file.arrayBuffer()) } });
  };
  return (
    <Modal title={t("metadata.imagesTitle", { name: item.name })} wide onClose={onClose}>
      <div className="flex flex-wrap items-center gap-2">
        <Select value={String(kind)} label={t("metadata.imageKind")} options={imageKinds.map(([k, l]) => ({ value: String(k), label: l() }))} onChange={(v) => setKind(Number(v))} />
        <label className="inline-flex min-h-11 cursor-pointer items-center rounded-md bg-fill px-4 type-headline hover:bg-fill-selected has-focus-visible:outline-2 has-focus-visible:outline-focus">
          {t("metadata.uploadImage")}
          <input type="file" accept="image/jpeg,image/png,image/webp,image/gif" className="sr-only" onChange={(e) => void upload(e.target.files?.[0])} />
        </label>
        {actions.setImage.isPending && <span className="type-footnote text-text-secondary">…</span>}
      </div>
      {error && <p className="type-footnote break-words text-danger">{errorMessage(error)}</p>}
      <div className="-mx-6 flex min-h-0 flex-col gap-4 overflow-y-auto px-6">
        <section className="flex flex-col gap-2">
          <h3 className="type-headline">{t("metadata.current")}</h3>
          {current.length === 0 && <p className="type-callout text-text-secondary">{t("metadata.noImage")}</p>}
          <div className={`grid gap-3 ${wide ? "grid-cols-2" : "grid-cols-3 sm:grid-cols-4"}`}>
            {current.map((i) => (
              <CurrentImage key={i.id} server={server} image={i} wide={wide} busy={actions.deleteImage.isPending} onDelete={() => actions.deleteImage.mutate(i.id)} />
            ))}
          </div>
        </section>
        <section className="flex flex-col gap-2 pb-1">
          <h3 className="type-headline">{t("metadata.fromProviders")}</h3>
          {remote.isFetching && <p className="type-callout text-text-secondary">{t("metadata.searching")}</p>}
          {remote.isError && <p className="type-callout break-words text-danger">{errorMessage(remote.error)}</p>}
          {remote.isSuccess && remote.data.length === 0 && <p className="type-callout text-text-secondary">{t("metadata.noRemoteImages")}</p>}
          <div className={`grid gap-3 ${wide ? "grid-cols-2" : "grid-cols-3 sm:grid-cols-4"}`}>
            {(remote.data ?? []).map((r) => (
              <button
                key={r.url}
                type="button"
                disabled={actions.setImage.isPending}
                onClick={() => actions.setImage.mutate({ kind, source: { case: "url", value: r.url } })}
                className="group flex flex-col gap-1 text-left"
              >
                <span className={`overflow-hidden rounded-md bg-fill ring-accent group-hover:ring-2 ${wide ? "aspect-video" : "aspect-[2/3]"}`}>
                  <img src={r.url} alt="" loading="lazy" className="h-full w-full object-cover" />
                </span>
                <span className="type-caption text-text-secondary">
                  {[r.provider, r.width && r.height ? `${r.width}×${r.height}` : "", r.language].filter(Boolean).join(" · ")}
                </span>
              </button>
            ))}
          </div>
        </section>
      </div>
      <div className="flex justify-end">
        <Button onClick={onClose}>{t("keys.done")}</Button>
      </div>
    </Modal>
  );
}

function CurrentImage({ server, image, wide, busy, onDelete }: { server: ServerEntry; image: Image; wide: boolean; busy: boolean; onDelete: () => void }) {
  const [confirming, setConfirming] = useState(false);
  return (
    <div className="flex flex-col gap-1">
      <span className={`overflow-hidden rounded-md bg-fill ${wide ? "aspect-video" : "aspect-[2/3]"}`}>
        <img src={imageUrl(server, image.url, 320)} alt="" className="h-full w-full object-cover" />
      </span>
      <div className="flex items-center justify-between gap-2">
        <span className="type-caption text-text-secondary">{image.width && image.height ? `${image.width}×${image.height}` : ""}</span>
        {confirming ? (
          <span className="flex gap-3">
            <button type="button" onClick={() => setConfirming(false)} className="type-footnote font-semibold hover:underline">
              {t("action.cancel")}
            </button>
            <button type="button" disabled={busy} onClick={onDelete} className="type-footnote font-semibold text-danger hover:underline">
              {t("metadata.confirmDelete")}
            </button>
          </span>
        ) : (
          <button type="button" onClick={() => setConfirming(true)} className="type-footnote font-semibold text-danger hover:underline">
            {t("metadata.deleteImage")}
          </button>
        )}
      </div>
    </div>
  );
}

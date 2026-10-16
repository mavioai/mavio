// Acquisition's qualities (docs/clients.md §6.3.11): the sizes of each
// kind's qualities, and a quality profile — the qualities it allows, best
// first, where upgrades stop, the language of movies, and the scores of
// custom formats.
import { clone, create } from "@bufbuild/protobuf";
import { errorMessage, type ServerEntry, useCustomFormats, useQualityDefinitions, useQualityProfiles } from "@mavio/client-ts";
import {
  FormatScoreSchema,
  MediaKind,
  type QualityDefinition,
  QualityDefinitionSchema,
  type QualityProfile,
  type QualityProfileItem,
  QualityProfileItemSchema,
  QualityProfileSchema,
} from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Icon } from "../../icon.tsx";
import { Button, Group, Row } from "../../ui.tsx";
import { NumberField, Switch } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { mediaKinds } from "./acquisition.tsx";
import { BackLink, Page, patched, useChoice } from "./common.tsx";
import { input } from "../../controls/styles.ts";
import { Confirm } from "../../controls/dialog.tsx";
import { useUnsavedGuard } from "../../controls/unsaved.tsx";

/** The languages releases are parsed into, by their English names, as libs/release names them. */
export const releaseLanguages = [
  "English", "French", "Spanish", "German", "Italian", "Danish", "Dutch", "Japanese", "Icelandic", "Chinese", "Russian",
  "Polish", "Vietnamese", "Swedish", "Norwegian", "Finnish", "Turkish", "Portuguese", "Flemish", "Greek", "Korean",
  "Hungarian", "Hebrew", "Lithuanian", "Czech", "Arabic", "Hindi", "Bulgarian", "Malayalam", "Ukrainian", "Slovak", "Thai",
  "Portuguese (Brazil)", "Spanish (Latino)", "Romanian", "Latvian", "Persian", "Catalan", "Croatian", "Serbian", "Bosnian",
  "Estonian", "Tamil", "Indonesian", "Macedonian", "Slovenian", "Bengali", "Telugu", "Kannada", "Albanian", "Afrikaans",
  "Marathi", "Tagalog", "Urdu", "Romansh", "Mongolian", "Georgian",
];

/** A number field clamped to a range, with buttons to step it. */
export function NumberInput({ value, onChange, label, min = 0, max, step = 1, className = "w-32" }: {
  value: number;
  onChange: (v: number) => void;
  label: string;
  min?: number;
  max?: number;
  step?: number;
  className?: string;
}) {
  return (
    <NumberField
      min={min}
      max={max}
      step={step}
      integer={Number.isInteger(step)}
      className={className}
      label={label}
      value={value}
      onChange={(v) => v !== undefined && onChange(v)}
    />
  );
}

/** Acquisition's page of the sizes of each kind's qualities. */
export function QualityDefinitionsPage({ server }: { server: ServerEntry }) {
  const [kind, setKind] = useChoice("view", mediaKinds.map((k) => k.kind));
  return (
    <Page
      title={t("acquisition.qualities")}
      message={t("acquisition.qualitiesMessage")}
      back={<BackLink section="acquisition" label={t("settings.acquisition")} />}
      actions={
        <Select
          value={String(kind)}
          label={t("acquisition.kind")}
          options={mediaKinds.map((k) => ({ value: String(k.kind), label: k.title() }))}
          onChange={(v) => setKind(Number(v) as MediaKind)}
        />
      }
    >
      <Definitions key={kind} server={server} kind={kind} />
    </Page>
  );
}

function Definitions({ server, kind }: { server: ServerEntry; kind: MediaKind }) {
  const { list, update } = useQualityDefinitions(server, kind);
  const [edits, setEdits] = useState<Record<number, QualityDefinition>>({});
  const [notice, setNotice] = useState<string>();
  const changed = Object.values(edits).filter((d) => {
    const saved = list.data?.find((x) => x.quality === d.quality);
    return saved && (saved.minSize !== d.minSize || saved.preferredSize !== d.preferredSize || saved.maxSize !== d.maxSize);
  });
  const guard = useUnsavedGuard(changed.length > 0);
  const set = (d: QualityDefinition, patch: Partial<QualityDefinition>) => {
    setEdits((e) => ({ ...e, [d.quality]: patched(QualityDefinitionSchema, e[d.quality] ?? d, patch) }));
    setNotice(undefined);
  };
  return (
    <>
      {guard.dialog}
      <Group footer={t("acquisition.qualitiesFooter")}>
        <div className="hidden min-h-11 items-center gap-3 px-4 type-footnote text-text-secondary sm:flex">
          <span className="flex-1">{t("acquisition.quality")}</span>
          <span className="w-24">{t("acquisition.minSize")}</span>
          <span className="w-24">{t("acquisition.preferredSize")}</span>
          <span className="w-24">{t("acquisition.maxSize")}</span>
        </div>
        {(list.data ?? []).map((saved) => {
          const d = edits[saved.quality] ?? saved;
          return (
            <div key={d.quality} className="flex min-h-13 flex-wrap items-center gap-3 px-4 py-2.5">
              <span className="min-w-0 flex-1 basis-40 type-body">{d.title}</span>
              <NumberInput value={d.minSize} step={0.1} label={`${d.title} ${t("acquisition.minSize")}`} onChange={(minSize) => set(d, { minSize })} />
              <NumberInput value={d.preferredSize} step={0.1} label={`${d.title} ${t("acquisition.preferredSize")}`} onChange={(preferredSize) => set(d, { preferredSize })} />
              <NumberInput value={d.maxSize} step={0.1} label={`${d.title} ${t("acquisition.maxSize")}`} onChange={(maxSize) => set(d, { maxSize })} />
            </div>
          );
        })}
        {list.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(list.error)}</p>}
      </Group>
      <div className="flex flex-wrap items-center gap-3">
        <Button
          disabled={changed.length === 0 || update.isPending}
          onClick={() => update.mutate(changed, { onSuccess: () => (setEdits({}), setNotice(t("acquisition.saved"))) })}
        >
          {update.isPending ? "…" : t("action.save")}
        </Button>
        {notice && changed.length === 0 && <span className="type-callout text-text-secondary">{notice}</span>}
        {update.isError && <span className="type-callout text-danger">{errorMessage(update.error)}</span>}
      </div>
    </>
  );
}

/** The ID an item is ranked by: a group's, else its quality's. */
const itemId = (it: QualityProfileItem) => (it.id || it.quality);

/** Acquisition's page of a quality profile; "new-movie" or "new-series" for one to add. */
export function QualityProfilePage({ server, id }: { server: ServerEntry; id: string }) {
  const profiles = useQualityProfiles(server);
  const isNew = id.startsWith("new-");
  const kind = isNew ? (id === "new-series" ? MediaKind.SERIES : MediaKind.MOVIE) : profiles.list.data?.find((p) => p.id === id)?.kind;
  const { list: definitions } = useQualityDefinitions(server, kind ?? MediaKind.MOVIE);
  const saved = isNew ? undefined : profiles.list.data?.find((p) => p.id === id);
  const back = <BackLink section="acquisition" label={t("settings.acquisition")} />;
  if (!isNew && profiles.list.isSuccess && !saved)
    return (
      <Page title={t("acquisition.profile")} back={back}>
        <p className="type-callout text-text-secondary">{t("acquisition.noProfile")}</p>
      </Page>
    );
  const blank =
    isNew && definitions.data
      ? create(QualityProfileSchema, {
          kind,
          items: definitions.data.map((d) => create(QualityProfileItemSchema, { quality: d.quality, allowed: false })),
          upgradeAllowed: true,
        })
      : undefined;
  const profile = saved ?? blank;
  return (
    <Page title={saved?.name || t("acquisition.newProfile")} message={mediaKinds.find((k) => k.kind === kind)?.title()} back={back}>
      {(profiles.list.isError || definitions.isError) && <p className="type-callout text-danger">{errorMessage(profiles.list.error ?? definitions.error)}</p>}
      {profile && definitions.data && <ProfileForm server={server} profile={profile} titles={new Map(definitions.data.map((d) => [d.quality, d.title]))} />}
    </Page>
  );
}

function ProfileForm({ server, profile, titles }: { server: ServerEntry; profile: QualityProfile; titles: Map<number, string> }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const navigate = useNavigate();
  const profiles = useQualityProfiles(server);
  const formats = useCustomFormats(server);
  const [draft, setDraft] = useState(() => clone(QualityProfileSchema, profile));
  const [notice, setNotice] = useState<string>();
  const [removing, setRemoving] = useState(false);
  const isNew = !profile.id;
  const set = (patch: Partial<QualityProfile>) => (setDraft((d) => patched(QualityProfileSchema, d, patch)), setNotice(undefined));
  const changed = JSON.stringify(draft) !== JSON.stringify(profile);
  const guard = useUnsavedGuard(changed);
  const nameOf = (it: QualityProfileItem) => (it.id ? it.name : (titles.get(it.quality) ?? String(it.quality)));
  // Best first, as Radarr shows them; the profile keeps them lowest first.
  const best = draft.items.toReversed();
  const allowed = draft.items.filter((it) => it.allowed);
  const setItems = (items: QualityProfileItem[]) => {
    const cutoffAllowed = items.some((it) => it.allowed && itemId(it) === draft.cutoff);
    const top = items.findLast((it) => it.allowed);
    set({ items, cutoff: cutoffAllowed || !top ? draft.cutoff : itemId(top) });
  };
  const toggle = (i: number) => setItems(draft.items.map((it, j) => (j === i ? patched(QualityProfileItemSchema, it, { allowed: !it.allowed, items: it.items.map((q) => patched(QualityProfileItemSchema, q, { allowed: !it.allowed })) }) : it)));
  const move = (i: number, by: number) => {
    const items = [...draft.items];
    const [it] = items.splice(i, 1);
    items.splice(i + by, 0, it);
    setItems(items);
  };
  const score = (formatId: string) => draft.formatScores.find((s) => s.formatId === formatId)?.score ?? 0;
  const setScore = (formatId: string, value: number) =>
    set({
      formatScores: [...draft.formatScores.filter((s) => s.formatId !== formatId), ...(value ? [create(FormatScoreSchema, { formatId, score: value })] : [])],
    });
  const save = () => {
    const done = { onSuccess: () => setNotice(t("acquisition.saved")) };
    const p = patched(QualityProfileSchema, draft, { name: draft.name.trim() });
    if (isNew)
      profiles.create.mutate(p, {
        onSuccess: (r) => (guard.leave(), void navigate({ replace: true, to: "/settings/$section", params: { section: "acquisition" }, search: { server: cur.server, acquisition: "profile", id: r.profile?.id } })),
      });
    else profiles.update.mutate(p, done);
  };
  const pending = profiles.create.isPending || profiles.update.isPending;
  const failure = profiles.create.error ?? profiles.update.error;
  return (
    <>
      {guard.dialog}
      <Group>
        <Row label={t("acquisition.profileName")}>
          <input className={`${input} sm:w-72`} aria-label={t("acquisition.profileName")} value={draft.name} onChange={(e) => set({ name: e.target.value })} />
        </Row>
        <Row label={t("acquisition.upgradeAllowed")} detail={t("acquisition.upgradeAllowedDetail")}>
          <Switch on={draft.upgradeAllowed} label={t("acquisition.upgradeAllowed")} onChange={(upgradeAllowed) => set({ upgradeAllowed })} />
        </Row>
        {draft.upgradeAllowed && (
          <Row label={t("acquisition.cutoff")} detail={t("acquisition.cutoffDetail")}>
            <Select
              value={String(draft.cutoff)}
              label={t("acquisition.cutoff")}
              options={[
                ...(allowed.some((it) => itemId(it) === draft.cutoff) ? [] : [{ value: String(draft.cutoff), label: "—" }]),
                ...allowed.toReversed().map((it) => ({ value: String(itemId(it)), label: nameOf(it) })),
              ]}
              onChange={(v) => set({ cutoff: Number(v) })}
            />
          </Row>
        )}
        {draft.kind === MediaKind.MOVIE && (
          <Row label={t("acquisition.language")} detail={t("acquisition.languageDetail")}>
            <Select
              value={draft.language === "Any" ? "" : draft.language}
              label={t("acquisition.language")}
              className="w-48"
              options={[
                { value: "", label: t("acquisition.languageAny") },
                { value: "Original", label: t("acquisition.languageOriginal") },
                { value: "Unknown", label: t("acquisition.languageUnknown") },
                ...releaseLanguages.toSorted((a, b) => a.localeCompare(b, locale)).map((l) => ({ value: l, label: l })),
              ]}
              onChange={(language) => set({ language })}
            />
          </Row>
        )}
      </Group>
      <Group title={t("acquisition.profileQualities")} footer={t("acquisition.profileQualitiesFooter")}>
        {best.map((it) => {
          const i = draft.items.indexOf(it);
          return (
            <div key={itemId(it)} className="flex min-h-11 items-center gap-3 px-4 py-1.5">
              <Switch on={it.allowed} label={nameOf(it)} onChange={() => toggle(i)} />
              <div className="min-w-0 flex-1">
                <div className={`type-body ${it.allowed ? "" : "text-text-secondary"}`}>{nameOf(it)}</div>
                {it.id > 0 && <div className="type-footnote text-text-secondary">{it.items.map(nameOf).join(", ")}</div>}
              </div>
              <button type="button" aria-label={t("acquisition.moveUp")} disabled={i === draft.items.length - 1} onClick={() => move(i, 1)} className="rounded-md p-1.5 text-text-secondary hover:bg-fill disabled:opacity-30">
                <Icon name="back" size={16} className="rotate-90" />
              </button>
              <button type="button" aria-label={t("acquisition.moveDown")} disabled={i === 0} onClick={() => move(i, -1)} className="rounded-md p-1.5 text-text-secondary hover:bg-fill disabled:opacity-30">
                <Icon name="back" size={16} className="-rotate-90" />
              </button>
            </div>
          );
        })}
      </Group>
      <Group title={t("acquisition.formatScores")} footer={t("acquisition.formatScoresFooter")}>
        {(formats.list.data ?? []).map((f) => (
          <Row key={f.id} label={f.name}>
            <NumberInput value={score(f.id)} min={-100000} max={100000} label={f.name} onChange={(v) => setScore(f.id, Math.trunc(v))} />
          </Row>
        ))}
        {formats.list.isSuccess && formats.list.data.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("acquisition.noFormats")}</p>}
        <Row label={t("acquisition.minFormatScore")} detail={t("acquisition.minFormatScoreDetail")}>
          <NumberInput value={draft.minFormatScore} min={-100000} max={100000} label={t("acquisition.minFormatScore")} onChange={(v) => set({ minFormatScore: Math.trunc(v) })} />
        </Row>
        {draft.upgradeAllowed && (
          <>
            <Row label={t("acquisition.cutoffFormatScore")} detail={t("acquisition.cutoffFormatScoreDetail")}>
              <NumberInput value={draft.cutoffFormatScore} min={-100000} max={100000} label={t("acquisition.cutoffFormatScore")} onChange={(v) => set({ cutoffFormatScore: Math.trunc(v) })} />
            </Row>
            <Row label={t("acquisition.minUpgradeFormatScore")} detail={t("acquisition.minUpgradeFormatScoreDetail")}>
              <NumberInput value={draft.minUpgradeFormatScore} min={1} max={100000} label={t("acquisition.minUpgradeFormatScore")} onChange={(v) => set({ minUpgradeFormatScore: Math.trunc(v) })} />
            </Row>
          </>
        )}
      </Group>
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={!changed || !draft.name.trim() || allowed.length === 0 || pending} onClick={save}>
          {pending ? "…" : isNew ? t("acquisition.create") : t("action.save")}
        </Button>
        {notice && !changed && <span className="type-callout text-text-secondary">{notice}</span>}
        {failure && <span className="type-callout text-danger">{errorMessage(failure)}</span>}
      </div>
      {!isNew &&
        (removing ? (
          <Confirm
            title={t("acquisition.removeProfileConfirm", { name: profile.name })}
            message={profiles.remove.isError ? errorMessage(profiles.remove.error) : t("acquisition.removeProfileDetail")}
            action={t("acquisition.removeProfile")}
            busy={profiles.remove.isPending}
            onCancel={() => setRemoving(false)}
            onConfirm={() =>
              profiles.remove.mutate(profile.id, {
                onSuccess: () => (guard.leave(), void navigate({ replace: true, to: "/settings/$section", params: { section: "acquisition" }, search: { server: cur.server } })),
              })
            }
          />
        ) : (
          <div>
            <Button variant="secondary" className="text-danger" onClick={() => setRemoving(true)}>
              {t("acquisition.removeProfile")}
            </Button>
          </div>
        ))}
    </>
  );
}

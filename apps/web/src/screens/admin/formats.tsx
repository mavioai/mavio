// Acquisition's custom formats (docs/clients.md §6.3.11): named sets of
// conditions on releases — on their titles, editions, languages, indexer
// flags, sources, resolutions, modifiers, sizes, groups, years and types —
// which quality profiles score.
import { clone, create } from "@bufbuild/protobuf";
import { errorMessage, type ServerEntry, useCustomFormats } from "@mavio/client-ts";
import {
  type CustomFormat,
  CustomFormatSchema,
  type CustomFormatSpec,
  CustomFormatSpecKind as Kind,
  CustomFormatSpecSchema,
} from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Button, Group, Row, Select, Toggle } from "../../ui.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { BackLink, Confirm, input, Page, PageLink, patched } from "./common.tsx";
import { NumberInput, releaseLanguages } from "./quality.tsx";

const kinds: { kind: Kind; title: () => string }[] = [
  { kind: Kind.RELEASE_TITLE, title: () => t("formats.releaseTitle") },
  { kind: Kind.EDITION, title: () => t("formats.edition") },
  { kind: Kind.LANGUAGE, title: () => t("formats.language") },
  { kind: Kind.INDEXER_FLAG, title: () => t("formats.indexerFlag") },
  { kind: Kind.SOURCE, title: () => t("formats.source") },
  { kind: Kind.RESOLUTION, title: () => t("formats.resolution") },
  { kind: Kind.QUALITY_MODIFIER, title: () => t("formats.modifier") },
  { kind: Kind.SIZE, title: () => t("formats.size") },
  { kind: Kind.RELEASE_GROUP, title: () => t("formats.releaseGroup") },
  { kind: Kind.YEAR, title: () => t("formats.year") },
  { kind: Kind.RELEASE_TYPE, title: () => t("formats.releaseType") },
];

/** The languages a condition can name; a function, as quality.tsx and this module import each other through acquisition.tsx. */
function languageChoices() {
  return ["Original", "Unknown", ...releaseLanguages].map((v) => ({ value: v, label: v }));
}

/** The values of the conditions that take one of a list, by the names libs/acquire gives them. */
const choices: Partial<Record<Kind, { value: string; label: string }[]>> = {
  [Kind.INDEXER_FLAG]: ["freeleech", "halfleech", "doubleupload", "internal", "scene", "exclusive", "nuked", "neutralleech"].map((v) => ({ value: v, label: v })),
  [Kind.SOURCE]: [
    ["cam", "CAM"],
    ["telesync", "Telesync"],
    ["telecine", "Telecine"],
    ["workprint", "Workprint"],
    ["dvd", "DVD"],
    ["tv", "TV"],
    ["webdl", "WEB-DL"],
    ["webrip", "WEBRip"],
    ["bluray", "Blu-ray"],
    ["unknown", "Unknown"],
  ].map(([value, label]) => ({ value, label })),
  [Kind.RESOLUTION]: ["360", "480", "540", "576", "720", "1080", "2160"].map((v) => ({ value: v, label: `${v}p` })),
  [Kind.QUALITY_MODIFIER]: [
    ["none", "None"],
    ["regional", "Regional"],
    ["screener", "Screener"],
    ["rawhd", "Raw-HD"],
    ["brdisk", "BR-DISK"],
    ["remux", "Remux"],
  ].map(([value, label]) => ({ value, label })),
  [Kind.RELEASE_TYPE]: [
    ["single_episode", "Single episode"],
    ["multi_episode", "Multi-episode"],
    ["season_pack", "Season pack"],
    ["unknown", "Unknown"],
  ].map(([value, label]) => ({ value, label })),
};

const patterned = (k: Kind) => k === Kind.RELEASE_TITLE || k === Kind.EDITION || k === Kind.RELEASE_GROUP;
const ranged = (k: Kind) => k === Kind.SIZE || k === Kind.YEAR;

/** Acquisition's page of the custom formats. */
export function CustomFormatsPage({ server }: { server: ServerEntry }) {
  const formats = useCustomFormats(server);
  const kindTitle = (k: Kind) => kinds.find((x) => x.kind === k)?.title() ?? "";
  return (
    <Page title={t("acquisition.formats")} message={t("formats.message")} back={<BackLink section="acquisition" label={t("settings.acquisition")} />}>
      <Group>
        {(formats.list.data ?? [])
          .toSorted((a, b) => a.name.localeCompare(b.name, locale))
          .map((f) => (
            <PageLink
              key={f.id}
              search={{ acquisition: "format", id: f.id }}
              label={f.name}
              detail={[...new Set(f.specifications.map((s) => kindTitle(s.kind)))].join(", ") || undefined}
            />
          ))}
        <PageLink search={{ acquisition: "format", id: "new" }} label={<span className="text-accent-text">{t("formats.add")}</span>} />
        {formats.list.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(formats.list.error)}</p>}
      </Group>
    </Page>
  );
}

/** Acquisition's page of a custom format; "new" for one to add. */
export function CustomFormatPage({ server, id }: { server: ServerEntry; id: string }) {
  const formats = useCustomFormats(server);
  const saved = formats.list.data?.find((f) => f.id === id);
  const back = <BackLink section="acquisition" label={t("settings.acquisition")} />;
  if (id !== "new" && formats.list.isSuccess && !saved)
    return (
      <Page title={t("acquisition.formats")} back={back}>
        <p className="type-callout text-text-secondary">{t("formats.none")}</p>
      </Page>
    );
  const format = id === "new" ? create(CustomFormatSchema) : saved;
  return (
    <Page title={saved?.name || t("formats.new")} back={back}>
      {formats.list.isError && <p className="type-callout text-danger">{errorMessage(formats.list.error)}</p>}
      {format && <FormatForm server={server} format={format} />}
    </Page>
  );
}

function FormatForm({ server, format }: { server: ServerEntry; format: CustomFormat }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const navigate = useNavigate();
  const formats = useCustomFormats(server);
  const [draft, setDraft] = useState(() => clone(CustomFormatSchema, format));
  const [notice, setNotice] = useState<string>();
  const [removing, setRemoving] = useState(false);
  const isNew = !format.id;
  const set = (patch: Partial<CustomFormat>) => (setDraft((d) => patched(CustomFormatSchema, d, patch)), setNotice(undefined));
  const setSpec = (i: number, patch: Partial<CustomFormatSpec>) =>
    set({ specifications: draft.specifications.map((s, j) => (j === i ? patched(CustomFormatSpecSchema, s, patch) : s)) });
  const changed = JSON.stringify(draft) !== JSON.stringify(format);
  const complete = draft.specifications.every((s) => s.name.trim() && (ranged(s.kind) ? s.max > s.min : s.value.trim()));
  const list = () => void navigate({ to: "/settings/$section", params: { section: "acquisition" }, search: { server: cur.server, acquisition: "formats" } });
  const save = () => {
    const f = patched(CustomFormatSchema, draft, { name: draft.name.trim() });
    if (isNew) formats.create.mutate(f, { onSuccess: list });
    else formats.update.mutate(f, { onSuccess: () => setNotice(t("acquisition.saved")) });
  };
  const pending = formats.create.isPending || formats.update.isPending;
  const failure = formats.create.error ?? formats.update.error;
  return (
    <>
      <Group>
        <Row label={t("formats.name")}>
          <input className={`${input} sm:w-72`} aria-label={t("formats.name")} value={draft.name} onChange={(e) => set({ name: e.target.value })} />
        </Row>
        <Row label={t("formats.rename")} detail={t("formats.renameDetail")}>
          <Toggle on={draft.includeWhenRenaming} label={t("formats.rename")} onChange={(includeWhenRenaming) => set({ includeWhenRenaming })} />
        </Row>
      </Group>
      <Group
        title={
          <span className="flex items-center justify-between gap-3">
            {t("formats.conditions")}
            <Button
              variant="plain"
              onClick={() => set({ specifications: [...draft.specifications, create(CustomFormatSpecSchema, { kind: Kind.RELEASE_TITLE })] })}
            >
              {t("formats.addCondition")}
            </Button>
          </span>
        }
        footer={t("formats.conditionsFooter")}
      >
        {draft.specifications.map((s, i) => (
          <SpecForm
            key={i}
            spec={s}
            onChange={(patch) => setSpec(i, patch)}
            onRemove={() => set({ specifications: draft.specifications.filter((_, j) => j !== i) })}
          />
        ))}
        {draft.specifications.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("formats.noConditions")}</p>}
      </Group>
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={!changed || !draft.name.trim() || !complete || pending} onClick={save}>
          {pending ? "…" : isNew ? t("acquisition.create") : t("action.save")}
        </Button>
        {notice && !changed && <span className="type-callout text-text-secondary">{notice}</span>}
        {failure && <span className="type-callout break-words text-danger">{errorMessage(failure)}</span>}
      </div>
      {!isNew &&
        (removing ? (
          <Confirm
            title={t("formats.removeConfirm", { name: format.name })}
            message={formats.remove.isError ? errorMessage(formats.remove.error) : t("formats.removeDetail")}
            action={t("formats.remove")}
            busy={formats.remove.isPending}
            onCancel={() => setRemoving(false)}
            onConfirm={() => formats.remove.mutate(format.id, { onSuccess: list })}
          />
        ) : (
          <div>
            <Button variant="secondary" className="text-danger" onClick={() => setRemoving(true)}>
              {t("formats.remove")}
            </Button>
          </div>
        ))}
    </>
  );
}

function SpecForm({ spec: s, onChange, onRemove }: { spec: CustomFormatSpec; onChange: (patch: Partial<CustomFormatSpec>) => void; onRemove: () => void }) {
  const list = s.kind === Kind.LANGUAGE ? languageChoices() : choices[s.kind];
  return (
    <div className="flex flex-col gap-2 px-4 py-3">
      <div className="flex flex-wrap items-center gap-2">
        <Select
          value={String(s.kind)}
          label={t("formats.kind")}
          options={kinds.map((k) => ({ value: String(k.kind), label: k.title() }))}
          onChange={(v) => onChange({ kind: Number(v) as Kind, value: "", min: 0, max: 0, exceptLanguage: false })}
        />
        <input
          className={`${input} min-w-40 flex-1`}
          aria-label={t("formats.conditionName")}
          placeholder={t("formats.conditionName")}
          value={s.name}
          onChange={(e) => onChange({ name: e.target.value })}
        />
        <Button variant="plain" className="text-danger" onClick={onRemove}>
          {t("formats.removeCondition")}
        </Button>
      </div>
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
        {patterned(s.kind) && (
          <input
            className={`${input} min-w-60 flex-1 font-mono`}
            aria-label={t("formats.pattern")}
            placeholder={t("formats.pattern")}
            value={s.value}
            onChange={(e) => onChange({ value: e.target.value })}
          />
        )}
        {list && (
          <Select
            value={s.value}
            label={t("formats.value")}
            options={[...(list.some((c) => c.value === s.value) ? [] : [{ value: s.value, label: s.value || t("formats.choose") }]), ...list]}
            onChange={(value) => onChange({ value })}
          />
        )}
        {ranged(s.kind) && (
          <span className="flex items-center gap-2 type-callout text-text-secondary">
            <NumberInput value={s.min} step={s.kind === Kind.SIZE ? 0.1 : 1} label={t("formats.min")} onChange={(min) => onChange({ min })} />
            –
            <NumberInput value={s.max} step={s.kind === Kind.SIZE ? 0.1 : 1} label={t("formats.max")} onChange={(max) => onChange({ max })} />
            {s.kind === Kind.SIZE && t("formats.gigabytes")}
          </span>
        )}
        {s.kind === Kind.LANGUAGE && <Check on={s.exceptLanguage} label={t("formats.exceptLanguage")} onChange={(exceptLanguage) => onChange({ exceptLanguage })} />}
        <Check on={s.negate} label={t("formats.negate")} onChange={(negate) => onChange({ negate })} />
        <Check on={s.required} label={t("formats.required")} onChange={(required) => onChange({ required })} />
      </div>
    </div>
  );
}

/** A checkbox with its label. */
export function Check({ on, label, onChange }: { on: boolean; label: string; onChange: (on: boolean) => void }) {
  return (
    <label className="inline-flex items-center gap-2 type-callout">
      <input type="checkbox" className="size-4 accent-[#34C759]" checked={on} onChange={(e) => onChange(e.target.checked)} />
      {label}
    </label>
  );
}

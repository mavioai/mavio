// Subtitles' settings (docs/clients.md §6.3.12), one page each: searching
// and saving — scores, intervals, embedded streams, upgrades, where
// subtitles are saved, the mods applied, equivalent languages and tags;
// synchronization; and translation, with its validation and automation.
import { clone, create, equals } from "@bufbuild/protobuf";
import { errorMessage, type ServerEntry, useAiModels, useSubtitleProviders, useSubtitleSettings, useSubtitleTranslators } from "@mavio/client-ts";
import {
  SubtitleFolder,
  type SubtitleLanguageEquivalence,
  SubtitleLanguageEquivalenceSchema,
  type SubtitleLanguageSpec,
  SubtitleLanguageSpecSchema,
  type SubtitleSettings,
  SubtitleSettingsSchema,
  type SubtitleSyncSettings,
  SubtitleSyncSettingsSchema,
  type SubtitleTranslationAutomation,
  SubtitleTranslationAutomationSchema,
  type SubtitleTranslationSettings,
  SubtitleTranslationSettingsSchema,
  type SubtitleTranslationValidation,
  SubtitleTranslationValidationSchema,
} from "@mavio/proto/mavio/acquisition/v1/subtitle_pb";
import { type ReactNode, useState } from "react";

import { t } from "../../i18n.ts";
import { Icon } from "../../icon.tsx";
import { Button, Group, Row } from "../../ui.tsx";
import { Switch } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import { BackLink, Page, patched } from "./common.tsx";
import { input } from "../../controls/styles.ts";
import { NumberInput } from "./quality.tsx";
import { languageName, useSubtitleLanguages } from "./subtitles.tsx";
import { useUnsavedGuard } from "../../controls/unsaved.tsx";

/** A page of the subtitle settings. */
export function SubtitleSettingsPage({ server, page }: { server: ServerEntry; page: "settings" | "sync" | "translation" }) {
  const settings = useSubtitleSettings(server);
  const title = { settings: t("subtitles.searching"), sync: t("subtitles.sync"), translation: t("subtitles.translation") }[page];
  const message = { settings: t("subtitles.searchingDetail"), sync: t("subtitles.syncDetail"), translation: t("subtitles.translationDetail") }[page];
  return (
    <Page title={title} message={message} back={<BackLink section="subtitles" label={t("settings.subtitles")} />}>
      {settings.get.isError && <p className="type-callout text-danger">{errorMessage(settings.get.error)}</p>}
      {settings.get.data && <SettingsForm key={page} server={server} page={page} settings={settings.get.data} />}
    </Page>
  );
}

/** The mods a download can have applied, as Bazarr names them; those taking arguments are kept as they are. */
export const mods: [string, () => string][] = [
  ["OCR_fixes", () => t("subtitles.modOcr")],
  ["remove_HI", () => t("subtitles.modRemoveHi")],
  ["common", () => t("subtitles.modCommon")],
  ["remove_tags", () => t("subtitles.modRemoveTags")],
  ["fix_uppercase", () => t("subtitles.modUppercase")],
  ["reverse_rtl", () => t("subtitles.modRtl")],
  ["emoji", () => t("subtitles.modEmoji")],
];

const words = (s: string) => s.split(/[\s,]+/).filter(Boolean);

/** The draft of the settings and its setters, a field's or a nested message's. */
function SettingsForm({ server, page, settings }: { server: ServerEntry; page: "settings" | "sync" | "translation"; settings: SubtitleSettings }) {
  const update = useSubtitleSettings(server).update;
  const [draft, setDraft] = useState(() => clone(SubtitleSettingsSchema, settings));
  const [notice, setNotice] = useState<string>();
  const set = (patch: Partial<SubtitleSettings>) => (setDraft((d) => patched(SubtitleSettingsSchema, d, patch)), setNotice(undefined));
  const changed = !equals(SubtitleSettingsSchema, draft, settings);
  const guard = useUnsavedGuard(changed);
  const save = () => update.mutate(draft, { onSuccess: () => setNotice(t("subtitles.saved")) });
  const props = { server, draft, set };
  return (
    <>
      {guard.dialog}
      {page === "settings" && <SearchingForm {...props} />}
      {page === "sync" && <SyncForm {...props} />}
      {page === "translation" && <TranslationForm {...props} />}
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={!changed || update.isPending} onClick={save}>
          {update.isPending ? "…" : t("action.save")}
        </Button>
        {changed && (
          <Button variant="plain" onClick={() => setDraft(clone(SubtitleSettingsSchema, settings))}>
            {t("action.cancel")}
          </Button>
        )}
        {notice && !changed && <span className="type-callout text-text-secondary">{notice}</span>}
        {update.isError && <span className="type-callout break-words text-danger">{errorMessage(update.error)}</span>}
      </div>
    </>
  );
}

interface FormProps {
  server: ServerEntry;
  draft: SubtitleSettings;
  set: (patch: Partial<SubtitleSettings>) => void;
}

/** A row with a number. */
function NumberRow({ label, detail, value, onChange, min = 0, max, step }: { label: string; detail?: string; value: number; onChange: (v: number) => void; min?: number; max?: number; step?: number }) {
  return (
    <Row label={label} detail={detail}>
      <NumberInput label={label} value={value} min={min} max={max} step={step} onChange={(v) => onChange(step && step < 1 ? v : Math.trunc(v))} />
    </Row>
  );
}

/** A row with a switch. */
function ToggleRow({ label, detail, on, onChange }: { label: string; detail?: string; on: boolean; onChange: (on: boolean) => void }) {
  return (
    <Row label={label} detail={detail}>
      <Switch on={on} label={label} onChange={onChange} />
    </Row>
  );
}

/** A row with a list of words, edited as text and set when it loses focus. */
function WordsRow({ label, detail, value, onChange }: { label: string; detail?: string; value: string[]; onChange: (v: string[]) => void }) {
  const [text, setText] = useState(value.join(", "));
  return (
    <Row label={label} detail={detail}>
      <input
        className={`${input} sm:w-72`}
        aria-label={label}
        value={text}
        onChange={(e) => (setText(e.target.value), onChange(words(e.target.value)))}
      />
    </Row>
  );
}

function SearchingForm({ server, draft, set }: FormProps) {
  const languages = useSubtitleLanguages(server, draft.defaultAudioLanguage, draft.defaultEmbeddedLanguage);
  const unset = [{ value: "", label: t("subtitles.languageUnset") }, ...languages];
  const hasMod = (m: string) => draft.mods.includes(m);
  const toggleMod = (m: string, on: boolean) => set({ mods: on ? [...draft.mods, m] : draft.mods.filter((x) => x !== m) });
  return (
    <>
      <Group title={t("subtitles.scores")} footer={t("subtitles.scoresFooter")}>
        <NumberRow label={t("subtitles.minimumScore")} value={draft.minimumScore} max={100} onChange={(minimumScore) => set({ minimumScore })} />
        <NumberRow label={t("subtitles.minimumScoreMovie")} value={draft.minimumScoreMovie} max={100} onChange={(minimumScoreMovie) => set({ minimumScoreMovie })} />
      </Group>
      <Group title={t("subtitles.searches")}>
        <NumberRow
          label={t("subtitles.wantedInterval")}
          detail={t("subtitles.hoursNever")}
          value={draft.wantedSearchIntervalHours}
          onChange={(wantedSearchIntervalHours) => set({ wantedSearchIntervalHours })}
        />
        <NumberRow
          label={t("subtitles.wantedIntervalMovie")}
          detail={t("subtitles.hoursNever")}
          value={draft.wantedSearchIntervalMovieHours}
          onChange={(wantedSearchIntervalMovieHours) => set({ wantedSearchIntervalMovieHours })}
        />
        <ToggleRow label={t("subtitles.adaptive")} detail={t("subtitles.adaptiveDetail")} on={draft.adaptiveSearching} onChange={(adaptiveSearching) => set({ adaptiveSearching })} />
        {draft.adaptiveSearching && (
          <>
            <NumberRow label={t("subtitles.adaptiveDelay")} value={draft.adaptiveDelayDays} onChange={(adaptiveDelayDays) => set({ adaptiveDelayDays })} />
            <NumberRow label={t("subtitles.adaptiveDelta")} value={draft.adaptiveDeltaDays} onChange={(adaptiveDeltaDays) => set({ adaptiveDeltaDays })} />
          </>
        )}
        <ToggleRow label={t("subtitles.sceneName")} detail={t("subtitles.sceneNameDetail")} on={draft.useSceneName} onChange={(useSceneName) => set({ useSceneName })} />
        <ToggleRow label={t("subtitles.skipHashing")} detail={t("subtitles.skipHashingDetail")} on={draft.skipHashing} onChange={(skipHashing) => set({ skipHashing })} />
      </Group>
      <Group title={t("subtitles.embedded")}>
        <ToggleRow label={t("subtitles.useEmbedded")} detail={t("subtitles.useEmbeddedDetail")} on={draft.useEmbedded} onChange={(useEmbedded) => set({ useEmbedded })} />
        {draft.useEmbedded && (
          <>
            <ToggleRow label={t("subtitles.ignorePgs")} on={draft.ignorePgs} onChange={(ignorePgs) => set({ ignorePgs })} />
            <ToggleRow label={t("subtitles.ignoreVobsub")} on={draft.ignoreVobsub} onChange={(ignoreVobsub) => set({ ignoreVobsub })} />
            <ToggleRow label={t("subtitles.ignoreAss")} on={draft.ignoreAss} onChange={(ignoreAss) => set({ ignoreAss })} />
          </>
        )}
        <Row label={t("subtitles.defaultAudio")} detail={t("subtitles.defaultAudioDetail")}>
          <Select value={draft.defaultAudioLanguage} label={t("subtitles.defaultAudio")} className="w-48" options={unset} onChange={(defaultAudioLanguage) => set({ defaultAudioLanguage })} />
        </Row>
        <Row label={t("subtitles.defaultEmbedded")}>
          <Select
            value={draft.defaultEmbeddedLanguage}
            label={t("subtitles.defaultEmbedded")}
            className="w-48"
            options={unset}
            onChange={(defaultEmbeddedLanguage) => set({ defaultEmbeddedLanguage })}
          />
        </Row>
      </Group>
      <Group title={t("subtitles.upgrades")}>
        <ToggleRow label={t("subtitles.upgrade")} detail={t("subtitles.upgradeDetail")} on={draft.upgrade} onChange={(upgrade) => set({ upgrade })} />
        {draft.upgrade && (
          <>
            <NumberRow label={t("subtitles.upgradeInterval")} value={draft.upgradeIntervalHours} min={1} onChange={(upgradeIntervalHours) => set({ upgradeIntervalHours })} />
            <NumberRow label={t("subtitles.upgradeDays")} value={draft.upgradeDays} max={30} onChange={(upgradeDays) => set({ upgradeDays })} />
            <ToggleRow label={t("subtitles.upgradeManual")} on={draft.upgradeManual} onChange={(upgradeManual) => set({ upgradeManual })} />
          </>
        )}
      </Group>
      <Group title={t("subtitles.saving")}>
        <Row label={t("subtitles.folder")}>
          <Select
            value={String(draft.folder)}
            label={t("subtitles.folder")}
            className="w-48"
            options={[
              { value: String(SubtitleFolder.UNSPECIFIED), label: t("subtitles.folderVideo") },
              { value: String(SubtitleFolder.RELATIVE), label: t("subtitles.folderRelative") },
              { value: String(SubtitleFolder.METADATA), label: t("subtitles.folderMetadata") },
            ]}
            onChange={(v) => {
              const folder = Number(v) as SubtitleFolder;
              set({ folder, customFolder: folder === SubtitleFolder.RELATIVE ? draft.customFolder || "Subs" : "" });
            }}
          />
        </Row>
        {draft.folder === SubtitleFolder.RELATIVE && (
          <Row label={t("subtitles.customFolder")}>
            <input className={`${input} sm:w-48`} aria-label={t("subtitles.customFolder")} value={draft.customFolder} onChange={(e) => set({ customFolder: e.target.value.trim() })} />
          </Row>
        )}
        <Row label={t("subtitles.hiExtension")} detail={t("subtitles.hiExtensionDetail")}>
          <Select value={draft.hiExtension || "hi"} label={t("subtitles.hiExtension")} options={["hi", "cc", "sdh"].map((v) => ({ value: v, label: v }))} onChange={(hiExtension) => set({ hiExtension })} />
        </Row>
        <ToggleRow label={t("subtitles.singleLanguage")} detail={t("subtitles.singleLanguageDetail")} on={draft.singleLanguage} onChange={(singleLanguage) => set({ singleLanguage })} />
        <ToggleRow label={t("subtitles.utf")} on={draft.utf8} onChange={(utf8) => set({ utf8 })} />
      </Group>
      <Group title={t("subtitles.mods")} footer={t("subtitles.modsFooter")}>
        {mods.map(([m, label]) => (
          <ToggleRow key={m} label={label()} on={hasMod(m)} onChange={(on) => toggleMod(m, on)} />
        ))}
      </Group>
      <Equivalences server={server} value={draft.languageEquals} onChange={(languageEquals) => set({ languageEquals })} />
      <Group title={t("subtitles.tags")} footer={t("subtitles.tagsFooter")}>
        <ToggleRow label={t("subtitles.movieTags")} on={draft.movieProfileTags} onChange={(movieProfileTags) => set({ movieProfileTags })} />
        <ToggleRow label={t("subtitles.seriesTags")} on={draft.seriesProfileTags} onChange={(seriesProfileTags) => set({ seriesProfileTags })} />
        <WordsRow label={t("subtitles.noProfileTags")} detail={t("subtitles.noProfileTagsDetail")} value={draft.noProfileTags} onChange={(noProfileTags) => set({ noProfileTags })} />
      </Group>
    </>
  );
}

/** A language spec's kind, as one value. */
const kindOf = (s?: SubtitleLanguageSpec) => (s?.forced ? "forced" : s?.hearingImpaired ? "hi" : "");

/** The languages whose subtitles count as another's. */
function Equivalences({ server, value, onChange }: { server: ServerEntry; value: SubtitleLanguageEquivalence[]; onChange: (v: SubtitleLanguageEquivalence[]) => void }) {
  const languages = useSubtitleLanguages(server, ...value.flatMap((e) => [e.from?.language ?? "", e.to?.language ?? ""]));
  const kinds = [
    { value: "", label: t("subtitles.kindNormal") },
    { value: "forced", label: t("subtitles.forced") },
    { value: "hi", label: t("subtitles.hiShort") },
  ];
  const spec = (s: SubtitleLanguageSpec | undefined, patch: { language?: string; kind?: string }) => {
    const kind = patch.kind ?? kindOf(s);
    return create(SubtitleLanguageSpecSchema, { language: patch.language ?? s?.language ?? "en", forced: kind === "forced", hearingImpaired: kind === "hi" });
  };
  const setAt = (i: number, side: "from" | "to", patch: { language?: string; kind?: string }) =>
    onChange(value.map((e, j) => (j === i ? patched(SubtitleLanguageEquivalenceSchema, e, { [side]: spec(e[side], patch) }) : e)));
  const side = (e: SubtitleLanguageEquivalence, i: number, which: "from" | "to") => (
    <>
      <Select value={e[which]?.language ?? ""} label={t("subtitles.language")} className="w-40" options={languages} onChange={(language) => setAt(i, which, { language })} />
      <Select value={kindOf(e[which])} label={t("subtitles.kind")} className="w-28" options={kinds} onChange={(kind) => setAt(i, which, { kind })} />
    </>
  );
  return (
    <Group
      title={
        <span className="flex items-center justify-between gap-3">
          {t("subtitles.equivalences")}
          <Button
            variant="plain"
            onClick={() => onChange([...value, create(SubtitleLanguageEquivalenceSchema, { from: spec(undefined, { language: "pt-BR" }), to: spec(undefined, { language: "pt" }) })])}
          >
            {t("rules.add")}
          </Button>
        </span>
      }
      footer={t("subtitles.equivalencesFooter")}
    >
      {value.map((e, i) => (
        <div key={i} className="flex flex-wrap items-center gap-2 px-4 py-2.5">
          {side(e, i, "from")}
          <span className="type-body text-text-secondary">→</span>
          {side(e, i, "to")}
          <span className="flex-1" />
          <button
            type="button"
            className="rounded-md p-1.5 text-text-secondary hover:tint"
            aria-label={t("rules.remove")}
            onClick={() => onChange(value.filter((_, j) => j !== i))}
          >
            <Icon name="close" size={16} />
          </button>
        </div>
      ))}
      {value.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("subtitles.noEquivalences")}</p>}
    </Group>
  );
}

function SyncForm({ server, draft, set }: FormProps) {
  const providers = useSubtitleProviders(server);
  const y = draft.sync ?? create(SubtitleSyncSettingsSchema);
  const setY = (patch: Partial<SubtitleSyncSettings>) => set({ sync: patched(SubtitleSyncSettingsSchema, y, patch) });
  const providerNames = (providers.data?.providers ?? []).map((p) => p.name);
  return (
    <>
      <Group footer={t("subtitles.syncFooter")}>
        <ToggleRow label={t("subtitles.syncEnabled")} on={y.enabled} onChange={(enabled) => setY({ enabled })} />
        {y.enabled && (
          <>
            <ToggleRow label={t("subtitles.syncThreshold")} detail={t("subtitles.syncThresholdDetail")} on={y.useThreshold} onChange={(useThreshold) => setY({ useThreshold })} />
            {y.useThreshold && <NumberRow label={t("subtitles.threshold")} value={y.threshold} max={100} onChange={(threshold) => setY({ threshold })} />}
            <ToggleRow label={t("subtitles.syncMovieThreshold")} on={y.useMovieThreshold} onChange={(useMovieThreshold) => setY({ useMovieThreshold })} />
            {y.useMovieThreshold && <NumberRow label={t("subtitles.threshold")} value={y.movieThreshold} max={100} onChange={(movieThreshold) => setY({ movieThreshold })} />}
            <ToggleRow label={t("subtitles.forceAudio")} detail={t("subtitles.forceAudioDetail")} on={y.forceAudio} onChange={(forceAudio) => setY({ forceAudio })} />
            <ToggleRow label={t("subtitles.originalLanguage")} detail={t("subtitles.originalLanguageDetail")} on={y.originalLanguage} onChange={(originalLanguage) => setY({ originalLanguage })} />
            <WordsRow
              label={t("subtitles.excludedProviders")}
              detail={providerNames.length ? t("subtitles.excludedProvidersDetail", { names: providerNames.join(", ") }) : undefined}
              value={y.excludedProviders}
              onChange={(excludedProviders) => setY({ excludedProviders })}
            />
            <WordsRow label={t("subtitles.excludedLanguages")} detail={t("subtitles.excludedLanguagesDetail")} value={y.excludedLanguages} onChange={(excludedLanguages) => setY({ excludedLanguages })} />
          </>
        )}
      </Group>
      {y.enabled && (
        <Group title={t("subtitles.alignment")}>
          <NumberRow label={t("subtitles.maxOffset")} value={y.maxOffsetSeconds} min={1} onChange={(maxOffsetSeconds) => setY({ maxOffsetSeconds })} />
          <ToggleRow label={t("subtitles.fixFramerate")} detail={t("subtitles.fixFramerateDetail")} on={y.fixFramerate} onChange={(fixFramerate) => setY({ fixFramerate })} />
          <ToggleRow label={t("subtitles.goldenSection")} detail={t("subtitles.goldenSectionDetail")} on={y.goldenSection} onChange={(goldenSection) => setY({ goldenSection })} />
          <NumberRow label={t("subtitles.minScore")} value={y.minScore} step={0.01} onChange={(minScore) => setY({ minScore })} />
          <NumberRow label={t("subtitles.maxQualityOffset")} value={y.maxQualityOffsetSeconds} onChange={(maxQualityOffsetSeconds) => setY({ maxQualityOffsetSeconds })} />
          <NumberRow label={t("subtitles.maxScaleDeviation")} value={y.maxScaleDeviation} step={0.001} onChange={(maxScaleDeviation) => setY({ maxScaleDeviation })} />
        </Group>
      )}
    </>
  );
}

/** A block of text, such as a prompt, with its placeholders. */
function TextBlock({ label, detail, value, onChange, rows = 4 }: { label: string; detail?: ReactNode; value: string; onChange: (v: string) => void; rows?: number }) {
  return (
    <div className="flex flex-col gap-1.5 px-4 py-2.5">
      <div className="type-body">{label}</div>
      {detail && <div className="type-footnote break-words text-text-secondary">{detail}</div>}
      <textarea rows={rows} className={`${input} py-2 font-mono`} aria-label={label} value={value} onChange={(e) => onChange(e.target.value)} />
    </div>
  );
}

function TranslationForm({ server, draft, set }: FormProps) {
  const translators = useSubtitleTranslators(server);
  const models = useAiModels(server, true);
  const tr = draft.translation ?? create(SubtitleTranslationSettingsSchema);
  const setTr = (patch: Partial<SubtitleTranslationSettings>) => set({ translation: patched(SubtitleTranslationSettingsSchema, tr, patch) });
  const v = tr.validation ?? create(SubtitleTranslationValidationSchema);
  const setV = (patch: Partial<SubtitleTranslationValidation>) => setTr({ validation: patched(SubtitleTranslationValidationSchema, v, patch) });
  const a = tr.automation ?? create(SubtitleTranslationAutomationSchema);
  const setA = (patch: Partial<SubtitleTranslationAutomation>) => setTr({ automation: patched(SubtitleTranslationAutomationSchema, a, patch) });
  const languages = useSubtitleLanguages(server, ...a.sourceLanguages);
  const options = [
    ...(translators.data?.translators ?? []).map((x) => ({ value: x.id, label: x.displayName || x.name })),
    ...(models.data ?? []).map((m) => ({ value: `ai/${m.provider}/${m.id}`, label: `${m.name || m.id} (${m.provider})` })),
  ];
  const named = (id: string) => options.find((o) => o.value === id)?.label ?? id;
  const withMissing = (ids: string[]) => [...options, ...ids.filter((id) => id && !options.some((o) => o.value === id)).map((id) => ({ value: id, label: id }))];
  const unused = options.filter((o) => o.value !== tr.translator && !tr.fallbackTranslators.includes(o.value));
  const placeholders = "{sourceLanguage}, {targetLanguage}, {lineToTranslate}, {contextBefore}, {contextAfter}";
  return (
    <>
      <Group title={t("subtitles.translators")} footer={t("subtitles.translatorFooter")}>
        <Row label={t("subtitles.translator")}>
          <Select
            value={tr.translator}
            label={t("subtitles.translator")}
            className="w-56"
            options={[{ value: "", label: t("subtitles.translatorFirst") }, ...withMissing([tr.translator])]}
            onChange={(translator) => setTr({ translator, fallbackTranslators: tr.fallbackTranslators.filter((x) => x !== translator) })}
          />
        </Row>
        {tr.fallbackTranslators.map((id, i) => (
          <Row key={id} label={named(id)} detail={t("subtitles.fallbackOf", { position: i + 1 })}>
            <button
              type="button"
              className="rounded-md p-1.5 text-text-secondary hover:tint"
              aria-label={t("rules.remove")}
              onClick={() => setTr({ fallbackTranslators: tr.fallbackTranslators.filter((x) => x !== id) })}
            >
              <Icon name="close" size={16} />
            </button>
          </Row>
        ))}
        {unused.length > 0 && (
          <Row label={t("subtitles.addFallback")}>
            <Select
              value=""
              label={t("subtitles.addFallback")}
              className="w-56"
              options={[{ value: "", label: t("subtitles.choose") }, ...unused]}
              onChange={(id) => id && setTr({ fallbackTranslators: [...tr.fallbackTranslators, id] })}
            />
          </Row>
        )}
        {(translators.isError || models.isError) && <p className="px-4 py-3 type-callout text-danger">{errorMessage(translators.error ?? models.error)}</p>}
      </Group>
      <Group title={t("subtitles.output")}>
        <NumberRow label={t("subtitles.defaultScore")} detail={t("subtitles.defaultScoreDetail")} value={tr.defaultScore} max={100} onChange={(defaultScore) => setTr({ defaultScore })} />
        <ToggleRow label={t("subtitles.credit")} detail={t("subtitles.creditDetail")} on={tr.credit} onChange={(credit) => setTr({ credit })} />
        <ToggleRow label={t("subtitles.stripFormatting")} detail={t("subtitles.stripFormattingDetail")} on={tr.stripFormatting} onChange={(stripFormatting) => setTr({ stripFormatting })} />
        <ToggleRow label={t("subtitles.preserveLineBreaks")} on={tr.preserveLineBreaks} onChange={(preserveLineBreaks) => setTr({ preserveLineBreaks })} />
        <ToggleRow label={t("subtitles.fixOverlaps")} on={tr.fixOverlaps} onChange={(fixOverlaps) => setTr({ fixOverlaps })} />
      </Group>
      <Group title={t("subtitles.models")} footer={t("subtitles.modelsFooter")}>
        <NumberRow label={t("subtitles.contextBefore")} value={tr.contextBefore} max={50} onChange={(contextBefore) => setTr({ contextBefore })} />
        <NumberRow label={t("subtitles.contextAfter")} value={tr.contextAfter} max={50} onChange={(contextAfter) => setTr({ contextAfter })} />
        <ToggleRow label={t("subtitles.translatedContext")} detail={t("subtitles.translatedContextDetail")} on={tr.translatedContext} onChange={(translatedContext) => setTr({ translatedContext })} />
        <ToggleRow label={t("subtitles.batch")} detail={t("subtitles.batchDetail")} on={tr.batch} onChange={(batch) => setTr({ batch })} />
        {tr.batch && <NumberRow label={t("subtitles.maxBatchSize")} detail={t("subtitles.maxBatchSizeDetail")} value={tr.maxBatchSize} onChange={(maxBatchSize) => setTr({ maxBatchSize })} />}
        <ToggleRow label={t("subtitles.languageCodes")} on={tr.languageCodes} onChange={(languageCodes) => setTr({ languageCodes })} />
        <TextBlock label={t("subtitles.prompt")} detail={placeholders} value={tr.prompt} onChange={(prompt) => setTr({ prompt })} rows={6} />
        <TextBlock label={t("subtitles.userPrompt")} value={tr.userPrompt} onChange={(userPrompt) => setTr({ userPrompt })} />
        <TextBlock label={t("subtitles.proofreadPrompt")} detail="{sourceLanguage}, {targetLanguage}, {sourceLine}, {translatedLine}" value={tr.proofreadPrompt} onChange={(proofreadPrompt) => setTr({ proofreadPrompt })} rows={6} />
        <TextBlock label={t("subtitles.proofreadUserPrompt")} value={tr.proofreadUserPrompt} onChange={(proofreadUserPrompt) => setTr({ proofreadUserPrompt })} />
      </Group>
      <Group title={t("subtitles.retries")}>
        <NumberRow label={t("subtitles.maxRetries")} value={tr.maxRetries} min={1} max={20} onChange={(maxRetries) => setTr({ maxRetries })} />
        <NumberRow label={t("subtitles.retryDelay")} value={tr.retryDelaySeconds} max={600} onChange={(retryDelaySeconds) => setTr({ retryDelaySeconds })} />
        <NumberRow label={t("subtitles.retryMultiplier")} value={tr.retryMultiplier} min={1} max={10} onChange={(retryMultiplier) => setTr({ retryMultiplier })} />
        <NumberRow label={t("subtitles.requestTimeout")} value={tr.requestTimeoutSeconds} min={1} max={3600} onChange={(requestTimeoutSeconds) => setTr({ requestTimeoutSeconds })} />
      </Group>
      <Group title={t("subtitles.validation")} footer={t("subtitles.validationFooter")}>
        <ToggleRow label={t("subtitles.validationEnabled")} on={v.enabled} onChange={(enabled) => setV({ enabled })} />
        {v.enabled && (
          <>
            <NumberRow
              label={t("subtitles.maxFileSize")}
              value={Math.round(Number(v.maxFileSize) / 1024)}
              min={1}
              onChange={(kb) => setV({ maxFileSize: BigInt(Math.max(1, kb)) * 1024n })}
            />
            <NumberRow label={t("subtitles.maxCueLength")} value={v.maxCueLength} min={1} onChange={(maxCueLength) => setV({ maxCueLength })} />
            <NumberRow label={t("subtitles.minCueLength")} value={v.minCueLength} onChange={(minCueLength) => setV({ minCueLength })} />
            <NumberRow label={t("subtitles.minDuration")} value={v.minDurationMs} onChange={(minDurationMs) => setV({ minDurationMs })} />
            <NumberRow label={t("subtitles.maxDuration")} value={v.maxDurationSeconds} min={1} onChange={(maxDurationSeconds) => setV({ maxDurationSeconds })} />
          </>
        )}
      </Group>
      <Group title={t("subtitles.automation")} footer={t("subtitles.automationFooter")}>
        <ToggleRow label={t("subtitles.automationEnabled")} on={a.enabled} onChange={(enabled) => setA({ enabled })} />
        {a.enabled && (
          <>
            <NumberRow label={t("subtitles.automationInterval")} value={a.intervalHours} min={1} max={8760} onChange={(intervalHours) => setA({ intervalHours })} />
            {a.sourceLanguages.map((l) => (
              <Row key={l} label={languageName(l)} detail={t("subtitles.sourceLanguage")}>
                <button
                  type="button"
                  className="rounded-md p-1.5 text-text-secondary hover:tint"
                  aria-label={t("rules.remove")}
                  onClick={() => setA({ sourceLanguages: a.sourceLanguages.filter((x) => x !== l) })}
                >
                  <Icon name="close" size={16} />
                </button>
              </Row>
            ))}
            <Row label={t("subtitles.addSourceLanguage")}>
              <Select
                value=""
                label={t("subtitles.addSourceLanguage")}
                className="w-48"
                options={[{ value: "", label: t("subtitles.choose") }, ...languages.filter((l) => !a.sourceLanguages.includes(l.value))]}
                onChange={(l) => l && setA({ sourceLanguages: [...a.sourceLanguages, l] })}
              />
            </Row>
            <NumberRow label={t("subtitles.maxPerRun")} value={a.maxPerRun} min={1} max={1000} onChange={(maxPerRun) => setA({ maxPerRun })} />
            <NumberRow label={t("subtitles.movieAge")} value={a.movieAgeHours} onChange={(movieAgeHours) => setA({ movieAgeHours })} />
            <NumberRow label={t("subtitles.episodeAge")} value={a.episodeAgeHours} onChange={(episodeAgeHours) => setA({ episodeAgeHours })} />
          </>
        )}
      </Group>
    </>
  );
}

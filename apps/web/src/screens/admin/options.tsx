// The Acquisition settings' Options (docs/clients.md §6.3.11): how the
// server names what it imports, how it imports finished downloads, how
// often it checks download clients, which releases it takes, and the
// recycle bin replaced and deleted files go to.
import { errorMessage, type ServerEntry, useAcquisitionSettings } from "@mavio/client-ts";
import {
  type AcquisitionSettings,
  AcquisitionSettingsSchema,
  ColonReplacement,
  EpisodeTitleRequired,
  MultiEpisodeStyle,
  type NamingSettings,
  NamingSettingsSchema,
  ProperDownload,
} from "@mavio/proto/mavio/acquisition/v1/settings_pb";
import { useState } from "react";

import { t } from "../../i18n.ts";
import { Button, Group, Row } from "../../ui.tsx";
import { NumberField, Switch } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import { input } from "../../controls/styles.ts";
import { useUnsavedGuard } from "../../controls/unsaved.tsx";
import { BackLink, Page, patched } from "./common.tsx";

/** The episode and folder templates' defaults, which empty ones use. */
const defaults = {
  standardEpisodeFormat: "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
  dailyEpisodeFormat: "{Series Title} - {Air-Date} - {Episode Title} {Quality Full}",
  animeEpisodeFormat: "{Series Title} - S{season:00}E{episode:00} - {Episode Title} {Quality Full}",
  seriesFolderFormat: "{Series Title}",
  seasonFolderFormat: "Season {season}",
  specialsFolderFormat: "Specials",
};

const colons = (): [ColonReplacement, string][] => [
  [ColonReplacement.SMART, t("options.colonSmart")],
  [ColonReplacement.DELETE, t("options.colonDelete")],
  [ColonReplacement.DASH, t("options.colonDash")],
  [ColonReplacement.SPACE_DASH, t("options.colonSpaceDash")],
  [ColonReplacement.SPACE_DASH_SPACE, t("options.colonSpaceDashSpace")],
];

/** The multi-episode styles, shown for episodes 6 to 8 of season 15. */
const multiEpisodeStyles: [MultiEpisodeStyle, string][] = [
  [MultiEpisodeStyle.PREFIXED_RANGE, "S15E06-E08"],
  [MultiEpisodeStyle.RANGE, "S15E06-08"],
  [MultiEpisodeStyle.EXTEND, "S15E06-07-08"],
  [MultiEpisodeStyle.SCENE, "S15E06-E07-E08"],
  [MultiEpisodeStyle.REPEAT, "S15E06E07E08"],
  [MultiEpisodeStyle.DUPLICATE, "S15E06 - S15E07 - S15E08"],
];

const titleRequirements = (): [EpisodeTitleRequired, string][] => [
  [EpisodeTitleRequired.ALWAYS, t("options.titleAlways")],
  [EpisodeTitleRequired.BULK_SEASON_RELEASES, t("options.titleBulk")],
  [EpisodeTitleRequired.NEVER, t("options.titleNever")],
];

const propers = (): [ProperDownload, string][] => [
  [ProperDownload.PREFER_AND_UPGRADE, t("options.propersUpgrade")],
  [ProperDownload.DO_NOT_UPGRADE, t("options.propersNoUpgrade")],
  [ProperDownload.DO_NOT_PREFER, t("options.propersNoPrefer")],
];

/** An enumeration's choices as a select's options. */
const choices = (list: [number, string][]) => list.map(([v, label]) => ({ value: String(v), label }));

/** The options as edited, the lists as the text typed. */
interface Draft {
  s: AcquisitionSettings;
  extensions: string;
  whitelisted: string;
}

const draftOf = (s: AcquisitionSettings): Draft => ({
  s,
  extensions: s.extraFileExtensions.join(", "),
  whitelisted: s.whitelistedHardcodedSubs.join(", "),
});

const list = (text: string) =>
  text
    .split(",")
    .map((v) => v.trim())
    .filter(Boolean);

/** The page of the options. */
export function AcquisitionOptionsPage({ server }: { server: ServerEntry }) {
  const settings = useAcquisitionSettings(server).get;
  return (
    <Page
      title={t("options.options")}
      message={t("options.optionsMessage")}
      back={<BackLink section="acquisition" label={t("settings.acquisition")} />}
    >
      {settings.isError && <p className="type-callout text-danger">{errorMessage(settings.error)}</p>}
      {settings.data && <OptionsForm server={server} settings={settings.data} />}
    </Page>
  );
}

type Flag =
  | "completedDownloadHandling"
  | "copyUsingHardlinks"
  | "importExtraFiles"
  | "deleteEmptyFolders"
  | "skipFreeSpaceCheck"
  | "autoUnmonitorPreviouslyDownloadedEpisodes"
  | "autoRedownloadFailed"
  | "autoRedownloadFailedFromInteractiveSearch"
  | "preferIndexerFlags"
  | "allowHardcodedSubs";
type Count =
  | "minimumFreeSpace"
  | "checkFinishedInterval"
  | "maximumSize"
  | "minimumAge"
  | "retention"
  | "rssSyncInterval"
  | "availabilityDelay"
  | "recycleBinCleanupDays";
type Template = keyof typeof defaults | "movieFolderFormat" | "standardMovieFormat";

function OptionsForm({ server, settings }: { server: ServerEntry; settings: AcquisitionSettings }) {
  const update = useAcquisitionSettings(server).update;
  const [draft, setDraft] = useState(() => draftOf(settings));
  const [notice, setNotice] = useState<string>();
  const { s } = draft;
  const naming = s.naming!;
  const edit = (d: Partial<Draft>) => (setDraft((x) => ({ ...x, ...d })), setNotice(undefined));
  const set = (patch: Partial<AcquisitionSettings>) => edit({ s: patched(AcquisitionSettingsSchema, s, patch) });
  const name = (patch: Partial<NamingSettings>) => set({ naming: patched(NamingSettingsSchema, naming, patch) });
  const saved = () =>
    patched(AcquisitionSettingsSchema, s, {
      extraFileExtensions: list(draft.extensions),
      whitelistedHardcodedSubs: list(draft.whitelisted),
      recycleBin: s.recycleBin.trim(),
      downloadClientWorkingFolders: s.downloadClientWorkingFolders.trim(),
    });
  const changed = JSON.stringify(saved()) !== JSON.stringify(settings);
  const guard = useUnsavedGuard(changed);
  const save = () => update.mutate(saved(), { onSuccess: (r) => (r && setDraft(draftOf(r)), setNotice(t("acquisition.saved"))) });

  const toggle = (key: Flag, label: string, detail?: string) => (
    <Row label={label} detail={detail}>
      <Switch on={s[key]} label={label} onChange={(on) => set({ [key]: on })} />
    </Row>
  );
  const number = (key: Count, label: string, detail: string | undefined, min: number, max?: number) => (
    <Row label={label} detail={detail}>
      <NumberField min={min} max={max} className="w-36" label={label} value={s[key]} onChange={(v) => set({ [key]: v ?? 0 })} />
    </Row>
  );
  const template = (key: Template, label: string) => (
    <Row label={label}>
      <input
        className={`${input} sm:w-96`}
        aria-label={label}
        placeholder={key in defaults ? defaults[key as keyof typeof defaults] : undefined}
        value={naming[key]}
        onChange={(e) => name({ [key]: e.target.value })}
      />
    </Row>
  );
  const text = (value: string, label: string, detail: string, onChange: (v: string) => void, placeholder?: string) => (
    <Row label={label} detail={detail}>
      <input className={`${input} sm:w-72`} aria-label={label} placeholder={placeholder} value={value} onChange={(e) => onChange(e.target.value)} />
    </Row>
  );
  const select = <E extends number>(value: E, fallback: E, label: string, options: [E, string][], onChange: (v: E) => void, detail?: string) => (
    <Row label={label} detail={detail}>
      <Select
        value={String(value || fallback)}
        label={label}
        className="w-60"
        options={choices(options)}
        onChange={(v) => onChange(Number(v) as E)}
      />
    </Row>
  );

  return (
    <>
      {guard.dialog}
      <Group title={t("options.naming")}>
        {namingSwitch(naming.replaceIllegalCharacters, t("options.replaceIllegal"), t("options.replaceIllegalDetail"), (on) => name({ replaceIllegalCharacters: on }))}
        {select(naming.colonReplacement, ColonReplacement.SMART, t("options.colon"), colons(), (colonReplacement) => name({ colonReplacement }))}
      </Group>
      <Group title={t("options.movieNaming")}>
        {namingSwitch(naming.renameMovies, t("options.renameMovies"), t("options.renameMoviesDetail"), (on) => name({ renameMovies: on }))}
        {template("movieFolderFormat", t("options.movieFolder"))}
        {template("standardMovieFormat", t("options.movieFile"))}
      </Group>
      <Group title={t("options.episodeNaming")} footer={t("options.namingFooter")}>
        {namingSwitch(naming.renameEpisodes, t("options.renameEpisodes"), t("options.renameMoviesDetail"), (on) => name({ renameEpisodes: on }))}
        {template("standardEpisodeFormat", t("options.standardEpisode"))}
        {template("dailyEpisodeFormat", t("options.dailyEpisode"))}
        {template("animeEpisodeFormat", t("options.animeEpisode"))}
        {template("seriesFolderFormat", t("options.seriesFolder"))}
        {template("seasonFolderFormat", t("options.seasonFolder"))}
        {template("specialsFolderFormat", t("options.specialsFolder"))}
        {select(naming.multiEpisodeStyle, MultiEpisodeStyle.PREFIXED_RANGE, t("options.multiEpisode"), multiEpisodeStyles, (multiEpisodeStyle) =>
          name({ multiEpisodeStyle }),
        )}
      </Group>
      <Group title={t("options.importing")}>
        {toggle("completedDownloadHandling", t("options.completedHandling"), t("options.completedHandlingDetail"))}
        {toggle("copyUsingHardlinks", t("options.hardlinks"), t("options.hardlinksDetail"))}
        {toggle("importExtraFiles", t("options.extraFiles"))}
        {s.importExtraFiles &&
          text(draft.extensions, t("options.extensions"), t("options.extensionsDetail"), (extensions) => edit({ extensions }), "srt")}
        {toggle("deleteEmptyFolders", t("options.deleteEmpty"))}
        {toggle("skipFreeSpaceCheck", t("options.skipFreeSpace"))}
        {!s.skipFreeSpaceCheck && number("minimumFreeSpace", t("options.minimumFreeSpace"), t("options.minimumFreeSpaceDetail"), 0)}
        {select(s.episodeTitleRequired, EpisodeTitleRequired.ALWAYS, t("options.episodeTitle"), titleRequirements(), (episodeTitleRequired) => set({ episodeTitleRequired }), t("options.episodeTitleDetail"))}
        {toggle("autoUnmonitorPreviouslyDownloadedEpisodes", t("options.unmonitorMissing"), t("options.unmonitorMissingDetail"))}
        {text(s.downloadClientWorkingFolders, t("options.workingFolders"), t("options.workingFoldersDetail"), (downloadClientWorkingFolders) => set({ downloadClientWorkingFolders }), "_UNPACK_|_FAILED_")}
      </Group>
      <Group title={t("options.downloadClients")}>
        {number("checkFinishedInterval", t("options.checkInterval"), t("options.checkIntervalDetail"), 1, 120)}
        {toggle("autoRedownloadFailed", t("options.redownload"))}
        {s.autoRedownloadFailed && toggle("autoRedownloadFailedFromInteractiveSearch", t("options.redownloadInteractive"))}
      </Group>
      <Group title={t("options.releasesGroup")}>
        {select(s.propers, ProperDownload.PREFER_AND_UPGRADE, t("options.propers"), propers(), (p) => set({ propers: p }))}
        {toggle("preferIndexerFlags", t("options.indexerFlags"))}
        {number("rssSyncInterval", t("options.rssInterval"), t("options.rssIntervalDetail"), 0, 120)}
        {number("maximumSize", t("options.maximumSize"), t("options.maximumSizeDetail"), 0)}
        {number("minimumAge", t("options.minimumAge"), t("options.minimumAgeDetail"), 0)}
        {number("retention", t("options.retention"), t("options.retentionDetail"), 0)}
        {number("availabilityDelay", t("options.availabilityDelay"), t("options.availabilityDelayDetail"), -365, 365)}
        {toggle("allowHardcodedSubs", t("options.hardcodedSubs"))}
        {!s.allowHardcodedSubs &&
          text(draft.whitelisted, t("options.whitelistedSubs"), t("options.whitelistedSubsDetail"), (whitelisted) => edit({ whitelisted }), "kor, chi")}
      </Group>
      <Group title={t("options.recycleBin")}>
        {text(s.recycleBin, t("options.recycleBin"), t("options.recycleBinDetail"), (recycleBin) => set({ recycleBin }), "/data/recycle")}
        {s.recycleBin.trim() !== "" && number("recycleBinCleanupDays", t("options.cleanupDays"), t("options.cleanupDaysDetail"), 0)}
      </Group>
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={!changed || update.isPending} onClick={save}>
          {update.isPending ? "…" : t("action.save")}
        </Button>
        {notice && !changed && <span className="type-callout text-text-secondary">{notice}</span>}
        {update.isError && <span className="type-callout text-danger">{errorMessage(update.error)}</span>}
      </div>
    </>
  );
}

/** A switch of a naming option. */
function namingSwitch(on: boolean, label: string, detail: string, onChange: (on: boolean) => void) {
  return (
    <Row label={label} detail={detail}>
      <Switch on={on} label={label} onChange={onChange} />
    </Row>
  );
}

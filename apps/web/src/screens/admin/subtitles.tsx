// The settings' Subtitles (docs/clients.md §6.3.12): the providers and
// translators the server's plugins have configured, the subtitle profiles
// and the libraries they are assigned to, and the pages of the settings —
// searching and saving, synchronization and translation.
import { create, equals } from "@bufbuild/protobuf";
import { timestampDate } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  type ServerEntry,
  useLocalization,
  usePlugins,
  useServerLibraries,
  useSubtitleAssignments,
  useSubtitleProfiles,
  useSubtitleProviders,
  useSubtitleTranslators,
} from "@mavio/client-ts";
import {
  SubtitleHearingImpaired,
  type SubtitleProfile,
  type SubtitleProfileItem,
  SubtitleProfileItemSchema,
  SubtitleProfileSchema,
} from "@mavio/proto/mavio/acquisition/v1/subtitle_pb";
import { useNavigate, useSearch } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Icon } from "../../icon.tsx";
import { Button, Group, Row } from "../../ui.tsx";
import { Switch } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { BackLink, fromNow, languageOptions, Page, PageLink, patched } from "./common.tsx";
import { input } from "../../controls/styles.ts";
import { Confirm } from "../../controls/dialog.tsx";
import { Check } from "./formats.tsx";
import { SubtitleSettingsPage } from "./subtitle-settings.tsx";

/** The section of the server chosen in the settings. */
export function SubtitlesSection({ server }: { server: ServerEntry }) {
  const { subtitles: page, id } = useSearch({ strict: false }) as SettingsSearch;
  switch (page) {
    case "profile":
      return <SubtitleProfilePage key={id} server={server} id={id ?? ""} />;
    case "settings":
    case "sync":
    case "translation":
      return <SubtitleSettingsPage server={server} page={page} />;
  }
  return <SubtitlesList server={server} />;
}

/** Languages a subtitle can have beyond the server's, which name only ISO 639-1 codes. */
const extraLanguages = ["pt-BR", "zh-Hans", "zh-Hant"];

/** The options of subtitle languages, named in the UI's language, with any of values the server's list lacks. */
export function useSubtitleLanguages(server: ServerEntry, ...values: string[]) {
  const { languages } = useLocalization(server);
  const known = [...languages, ...extraLanguages.map((code) => ({ code, name: code }))];
  const missing = values.filter((v) => v && !known.some((l) => l.code === v));
  return languageOptions([...known, ...missing.map((code) => ({ code, name: code }))]);
}

function SubtitlesList({ server }: { server: ServerEntry }) {
  const providers = useSubtitleProviders(server);
  const translators = useSubtitleTranslators(server);
  const profiles = useSubtitleProfiles(server);
  const plugins = usePlugins(server);
  const pluginName = (id: string) => plugins.data?.find((p) => p.manifest?.id === id)?.manifest?.name || id;
  const throttled = (until?: Date) => until && until.getTime() > Date.now();
  const sorted = (profiles.list.data ?? []).toSorted((a, b) => a.name.localeCompare(b.name, locale));
  return (
    <Page title={t("settings.subtitles")} message={t("subtitles.message")}>
      <Group title={t("subtitles.providers")} footer={t("subtitles.providersFooter")}>
        {(providers.data?.providers ?? []).map((p) => {
          const until = p.throttledUntil && timestampDate(p.throttledUntil);
          return (
            <PageLink
              key={p.id}
              search={{ plugin: p.pluginId }}
              icon="subtitles"
              color={!p.enabled ? "#8E8E93" : throttled(until) ? "#FF9500" : "#34C759"}
              label={p.displayName || p.name}
              detail={[
                pluginName(p.pluginId),
                [p.movies && t("subtitles.movies"), p.episodes && t("subtitles.episodes")].filter(Boolean).join(" + "),
                !p.enabled && t("acquisition.off"),
                until && throttled(until) && t("subtitles.throttled", { when: fromNow(until), reason: p.throttleReason }),
              ]
                .filter(Boolean)
                .join(" · ")}
            />
          );
        })}
        {(providers.data?.failedPlugins ?? []).map((id) => (
          <p key={id} className="px-4 py-3 type-callout break-words text-danger">
            {t("subtitles.pluginFailed", { plugin: pluginName(id) })}
          </p>
        ))}
        {providers.isSuccess && providers.data.providers.length === 0 && providers.data.failedPlugins.length === 0 && (
          <p className="px-4 py-3 type-callout text-text-secondary">{t("subtitles.noProviders")}</p>
        )}
        {providers.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(providers.error)}</p>}
      </Group>
      <Group title={t("subtitles.translators")} footer={t("subtitles.translatorsFooter")}>
        {(translators.data?.translators ?? []).map((x) => (
          <PageLink
            key={x.id}
            search={{ plugin: x.pluginId }}
            icon="subtitles"
            color={x.enabled ? "#007AFF" : "#8E8E93"}
            label={x.displayName || x.name}
            detail={[pluginName(x.pluginId), !x.enabled && t("acquisition.off")].filter(Boolean).join(" · ")}
          />
        ))}
        {(translators.data?.failedPlugins ?? []).map((id) => (
          <p key={id} className="px-4 py-3 type-callout break-words text-danger">
            {t("subtitles.pluginFailed", { plugin: pluginName(id) })}
          </p>
        ))}
        {translators.isSuccess && translators.data.translators.length === 0 && translators.data.failedPlugins.length === 0 && (
          <p className="px-4 py-3 type-callout text-text-secondary">{t("subtitles.noTranslators")}</p>
        )}
        {translators.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(translators.error)}</p>}
      </Group>
      <Group title={t("subtitles.profiles")} footer={t("subtitles.profilesFooter")}>
        {sorted.map((p) => (
          <PageLink key={p.id} search={{ subtitles: "profile", id: p.id }} label={p.name} detail={p.items.map(itemName).join(", ")} />
        ))}
        <PageLink search={{ subtitles: "profile", id: "new" }} label={<span className="text-accent-text">{t("subtitles.addProfile")}</span>} />
        {profiles.list.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(profiles.list.error)}</p>}
      </Group>
      <LibraryProfiles server={server} profiles={sorted} />
      <Group title={t("subtitles.settings")}>
        <PageLink search={{ subtitles: "settings" }} icon="search" color="#5856D6" label={t("subtitles.searching")} detail={t("subtitles.searchingDetail")} />
        <PageLink search={{ subtitles: "sync" }} icon="pending" color="#FF9500" label={t("subtitles.sync")} detail={t("subtitles.syncDetail")} />
        <PageLink search={{ subtitles: "translation" }} icon="subtitles" color="#007AFF" label={t("subtitles.translation")} detail={t("subtitles.translationDetail")} />
      </Group>
    </Page>
  );
}

const languageNames = new Intl.DisplayNames([locale], { type: "language", fallback: "code" });

/** A language's name in the UI's language, or its tag. */
export function languageName(tag: string) {
  try {
    return languageNames.of(tag) ?? tag;
  } catch {
    return tag;
  }
}

/** A profile's language, as "French (forced)". */
function itemName(it: SubtitleProfileItem) {
  const kinds = [
    it.forced && t("subtitles.forced"),
    it.hearingImpaired === SubtitleHearingImpaired.REQUIRED && t("subtitles.hiShort"),
    it.hearingImpaired === SubtitleHearingImpaired.EXCLUDED && t("subtitles.noHiShort"),
  ].filter(Boolean);
  return kinds.length ? `${languageName(it.language)} (${kinds.join(", ")})` : languageName(it.language);
}

/** Each library with the profile assigned to it. */
function LibraryProfiles({ server, profiles }: { server: ServerEntry; profiles: SubtitleProfile[] }) {
  const libraries = useServerLibraries(server);
  const assignments = useSubtitleAssignments(server);
  const options = [{ value: "", label: t("subtitles.noProfile") }, ...profiles.map((p) => ({ value: p.id, label: p.name }))];
  return (
    <Group title={t("subtitles.libraries")} footer={t("subtitles.librariesFooter")}>
      {(libraries.data ?? []).map((l) => (
        <Row key={l.id} label={l.name}>
          <Select
            value={assignments.list.data?.get(l.id) ?? ""}
            label={l.name}
            className="w-48"
            options={options}
            onChange={(profileId) => assignments.assign.mutate({ targetId: l.id, profileId })}
          />
        </Row>
      ))}
      {libraries.isSuccess && libraries.data.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("subtitles.noLibraries")}</p>}
      {(assignments.list.isError || assignments.assign.isError) && (
        <p className="px-4 py-3 type-callout text-danger">{errorMessage(assignments.list.error ?? assignments.assign.error)}</p>
      )}
    </Group>
  );
}

/** A subtitle profile, "new" for one to add. */
function SubtitleProfilePage({ server, id }: { server: ServerEntry; id: string }) {
  const profiles = useSubtitleProfiles(server);
  const back = <BackLink section="subtitles" label={t("settings.subtitles")} />;
  if (id === "new") return <ProfileForm server={server} profile={create(SubtitleProfileSchema, { items: [create(SubtitleProfileItemSchema, { id: 1, language: "en" })] })} back={back} />;
  const saved = profiles.list.data?.find((p) => p.id === id);
  if (!saved)
    return (
      <Page title={t("subtitles.profile")} back={back}>
        {profiles.list.isError && <p className="type-callout text-danger">{errorMessage(profiles.list.error)}</p>}
        {profiles.list.isSuccess && <p className="type-callout text-text-secondary">{t("subtitles.profileGone")}</p>}
      </Page>
    );
  return <ProfileForm key={saved.id} server={server} profile={saved} back={back} />;
}

/** The cutoff that ends the search once any language has subtitles. */
const cutoffAny = 65535;

const lines = (s: string) =>
  s
    .split("\n")
    .map((x) => x.trim())
    .filter(Boolean);

function ProfileForm({ server, profile, back }: { server: ServerEntry; profile: SubtitleProfile; back: ReactNode }) {
  const profiles = useSubtitleProfiles(server);
  const navigate = useNavigate();
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const [draft, setDraft] = useState(profile);
  const [mustContain, setMustContain] = useState(profile.mustContain.join("\n"));
  const [mustNotContain, setMustNotContain] = useState(profile.mustNotContain.join("\n"));
  const [removing, setRemoving] = useState(false);
  const languages = useSubtitleLanguages(server, ...draft.items.map((it) => it.language));
  const isNew = !profile.id;
  const set = (patch: Partial<SubtitleProfile>) => setDraft((d) => patched(SubtitleProfileSchema, d, patch));
  const setItem = (i: number, patch: Partial<SubtitleProfileItem>) =>
    set({ items: draft.items.map((it, j) => (j === i ? patched(SubtitleProfileItemSchema, it, patch) : it)) });
  const result = patched(SubtitleProfileSchema, draft, { name: draft.name.trim(), tag: draft.tag.trim(), mustContain: lines(mustContain), mustNotContain: lines(mustNotContain) });
  const changed = isNew || !equals(SubtitleProfileSchema, result, profile);
  const valid = result.name !== "" && result.items.length > 0 && !/\s/.test(result.tag);
  const addItem = () => {
    const next = Math.max(0, ...draft.items.map((it) => it.id)) + 1;
    const used = new Set(draft.items.map((it) => it.language));
    const language = [locale.split("-")[0], "en", ...languages.map((l) => l.value)].find((l) => !used.has(l)) ?? "en";
    set({ items: [...draft.items, create(SubtitleProfileItemSchema, { id: next, language })] });
  };
  const removeItem = (i: number) => {
    const gone = draft.items[i];
    set({ items: draft.items.filter((_, j) => j !== i), cutoff: draft.cutoff === gone.id ? 0 : draft.cutoff });
  };
  const toList = () => navigate({ to: "/settings/$section", params: { section: "subtitles" }, search: { server: cur.server } });
  const save = () =>
    isNew
      ? profiles.create.mutate(result, {
          onSuccess: (r) => r.profile && navigate({ replace: true, to: "/settings/$section", params: { section: "subtitles" }, search: { server: cur.server, subtitles: "profile", id: r.profile.id } }),
        })
      : profiles.update.mutate(result);
  const hiOptions = [
    { value: String(SubtitleHearingImpaired.UNSPECIFIED), label: t("subtitles.hiAllowed") },
    { value: String(SubtitleHearingImpaired.REQUIRED), label: t("subtitles.hiRequired") },
    { value: String(SubtitleHearingImpaired.EXCLUDED), label: t("subtitles.hiExcluded") },
  ];
  const error = profiles.create.error ?? profiles.update.error ?? profiles.remove.error;
  return (
    <Page title={profile.name || t("subtitles.newProfile")} back={back}>
      <Group>
        <Row label={t("rules.name")}>
          <input className={`${input} sm:w-72`} aria-label={t("rules.name")} value={draft.name} onChange={(e) => set({ name: e.target.value })} />
        </Row>
      </Group>
      <Group
        title={
          <span className="flex items-center justify-between gap-3">
            {t("subtitles.languages")}
            <Button variant="plain" onClick={addItem}>
              {t("rules.add")}
            </Button>
          </span>
        }
        footer={t("subtitles.languagesFooter")}
      >
        {draft.items.map((it, i) => (
          <div key={it.id} className="flex flex-col gap-2 px-4 py-2.5">
            <div className="flex flex-wrap items-center gap-2">
              <Select
                value={it.language}
                label={t("subtitles.language")}
                className="w-48"
                options={languages}
                onChange={(language) => setItem(i, { language })}
              />
              <Select
                value={String(it.hearingImpaired)}
                label={t("subtitles.hearingImpaired")}
                className="w-48"
                options={hiOptions}
                onChange={(v) => setItem(i, { hearingImpaired: Number(v) as SubtitleHearingImpaired })}
              />
              <span className="flex-1" />
              <button
                type="button"
                className="rounded-md p-1.5 text-text-secondary enabled:hover:tint disabled:opacity-30"
                aria-label={t("rules.remove")}
                disabled={draft.items.length === 1}
                onClick={() => removeItem(i)}
              >
                <Icon name="close" size={16} />
              </button>
            </div>
            <div className="flex flex-wrap gap-x-4 gap-y-1">
              <Check on={it.forced} label={t("subtitles.forcedOnly")} onChange={(forced) => setItem(i, { forced })} />
              <Check on={it.audioExclude} label={t("subtitles.audioExclude")} onChange={(audioExclude) => setItem(i, { audioExclude, audioOnlyInclude: audioExclude ? false : it.audioOnlyInclude })} />
              <Check on={it.audioOnlyInclude} label={t("subtitles.audioOnlyInclude")} onChange={(audioOnlyInclude) => setItem(i, { audioOnlyInclude, audioExclude: audioOnlyInclude ? false : it.audioExclude })} />
            </div>
          </div>
        ))}
      </Group>
      <Group>
        <Row label={t("subtitles.cutoff")} detail={t("subtitles.cutoffDetail")}>
          <Select
            value={String(draft.cutoff)}
            label={t("subtitles.cutoff")}
            className="w-48"
            options={[
              { value: "0", label: t("subtitles.cutoffNone") },
              { value: String(cutoffAny), label: t("subtitles.cutoffAny") },
              ...draft.items.map((it) => ({ value: String(it.id), label: itemName(it) })),
            ]}
            onChange={(v) => set({ cutoff: Number(v) })}
          />
        </Row>
        <Row label={t("subtitles.originalFormat")} detail={t("subtitles.originalFormatDetail")}>
          <Switch on={draft.originalFormat} label={t("subtitles.originalFormat")} onChange={(originalFormat) => set({ originalFormat })} />
        </Row>
        <Row label={t("subtitles.tag")} detail={t("subtitles.tagDetail")}>
          <input className={`${input} sm:w-48`} aria-label={t("subtitles.tag")} value={draft.tag} onChange={(e) => set({ tag: e.target.value })} />
        </Row>
      </Group>
      <Group title={t("subtitles.releaseNames")} footer={t("subtitles.releaseNamesFooter")}>
        <div className="flex flex-col gap-1.5 px-4 py-2.5">
          <div className="type-body">{t("subtitles.mustContain")}</div>
          <textarea rows={2} className={`${input} py-2 font-mono`} aria-label={t("subtitles.mustContain")} value={mustContain} onChange={(e) => setMustContain(e.target.value)} />
        </div>
        <div className="flex flex-col gap-1.5 px-4 py-2.5">
          <div className="type-body">{t("subtitles.mustNotContain")}</div>
          <textarea rows={2} className={`${input} py-2 font-mono`} aria-label={t("subtitles.mustNotContain")} value={mustNotContain} onChange={(e) => setMustNotContain(e.target.value)} />
        </div>
      </Group>
      {removing ? (
        <Confirm
          title={t("subtitles.removeTitle", { name: profile.name })}
          message={t("subtitles.removeMessage")}
          action={t("subtitles.remove")}
          busy={profiles.remove.isPending}
          onCancel={() => setRemoving(false)}
          onConfirm={() => profiles.remove.mutate(profile.id, { onSuccess: toList })}
        />
      ) : (
        <div className="flex flex-wrap items-center gap-3">
          <Button disabled={!changed || !valid || profiles.create.isPending || profiles.update.isPending} onClick={save}>
            {profiles.create.isPending || profiles.update.isPending ? "…" : t("action.save")}
          </Button>
          {!isNew && (
            <Button variant="plain" className="text-danger" onClick={() => setRemoving(true)}>
              {t("subtitles.remove")}
            </Button>
          )}
          {error && <span className="type-callout break-words text-danger">{errorMessage(error)}</span>}
        </div>
      )}
    </Page>
  );
}

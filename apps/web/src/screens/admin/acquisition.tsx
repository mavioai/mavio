// The settings' Acquisition (docs/clients.md §6.3.11): the indexers and
// download clients the server's plugins have configured, the quality
// profiles of movies and series, and the pages of what else decides
// which releases are grabbed — quality definitions, custom formats, delay
// and release profiles and remote path mappings.
import { type Timestamp, timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useDownloadClients, useIndexers, usePlugins, useQualityProfiles } from "@mavio/client-ts";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { DownloadProtocol, IndexerPrivacy } from "@mavio/proto/mavio/acquisition/v1/release_pb";
import { Link, useSearch } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { locale, t } from "../../i18n.ts";
import { Icon, type IconName } from "../../icon.tsx";
import { Group } from "../../ui.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { ColorTile, fromNow, Page } from "./common.tsx";
import { CustomFormatPage, CustomFormatsPage } from "./formats.tsx";
import { QualityDefinitionsPage, QualityProfilePage } from "./quality.tsx";
import { DelayProfilesPage, ReleaseProfilesPage, RemotePathMappingsPage } from "./rules.tsx";

/** The section of the server chosen in the settings. */
export function AcquisitionSection({ server }: { server: ServerEntry }) {
  const { acquisition: page, id } = useSearch({ strict: false }) as SettingsSearch;
  switch (page) {
    case "profile":
      return <QualityProfilePage key={id} server={server} id={id ?? ""} />;
    case "qualities":
      return <QualityDefinitionsPage server={server} />;
    case "formats":
      return <CustomFormatsPage server={server} />;
    case "format":
      return <CustomFormatPage key={id} server={server} id={id ?? ""} />;
    case "delays":
      return <DelayProfilesPage server={server} />;
    case "releases":
      return <ReleaseProfilesPage server={server} />;
    case "mappings":
      return <RemotePathMappingsPage server={server} />;
  }
  return <AcquisitionList server={server} />;
}

/** The kinds of quality profiles and definitions, named. */
export const mediaKinds = [
  { kind: MediaKind.MOVIE, title: () => t("acquisition.movies") },
  { kind: MediaKind.SERIES, title: () => t("acquisition.series") },
];

/** A row linking to a page of the section. */
export function PageLink({
  search,
  icon,
  color,
  label,
  detail,
}: {
  search: SettingsSearch;
  icon?: IconName;
  color?: string;
  label: ReactNode;
  detail?: ReactNode;
}) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  return (
    <Link
      to="/settings/$section"
      params={{ section: search.plugin ? "plugins" : "acquisition" }}
      search={{ server: cur.server, ...search }}
      className="flex min-h-13 items-center gap-3 px-4 py-2.5 hover:bg-surface-raised"
    >
      {icon && <ColorTile icon={icon} color={color ?? "#8E8E93"} />}
      <div className="min-w-0 flex-1">
        <div className="truncate type-body">{label}</div>
        {detail && <div className="type-footnote break-words text-text-secondary">{detail}</div>}
      </div>
      <Icon name="forward" size={16} className="shrink-0 text-text-tertiary" />
    </Link>
  );
}

const protocols: Record<DownloadProtocol, () => string> = {
  [DownloadProtocol.UNSPECIFIED]: () => "",
  [DownloadProtocol.TORRENT]: () => t("acquisition.torrent"),
  [DownloadProtocol.USENET]: () => t("acquisition.usenet"),
};

const privacies: Record<IndexerPrivacy, () => string> = {
  [IndexerPrivacy.UNSPECIFIED]: () => "",
  [IndexerPrivacy.PUBLIC]: () => t("acquisition.public"),
  [IndexerPrivacy.SEMI_PRIVATE]: () => t("acquisition.semiPrivate"),
  [IndexerPrivacy.PRIVATE]: () => t("acquisition.private"),
};

/** Why an indexer or client is left out: its failure, until when. */
function failureOf(x: { enabled: boolean; failure: string; disabledUntil?: Timestamp }) {
  if (!x.enabled) return t("acquisition.off");
  if (!x.disabledUntil) return undefined;
  const until = timestampDate(x.disabledUntil);
  if (until.getTime() < Date.now()) return undefined;
  return t("acquisition.leftOut", { when: fromNow(until), failure: x.failure });
}

function AcquisitionList({ server }: { server: ServerEntry }) {
  const indexers = useIndexers(server);
  const clients = useDownloadClients(server);
  const profiles = useQualityProfiles(server);
  const plugins = usePlugins(server);
  const pluginName = (id: string) => plugins.data?.find((p) => p.manifest?.id === id)?.manifest?.name || id;
  const failedIndexers = indexers.data?.failures ?? [];
  const failedClients = Object.entries(clients.data?.failures ?? {});
  return (
    <Page title={t("settings.acquisition")} message={t("acquisition.message")}>
      <Group title={t("acquisition.indexers")} footer={t("acquisition.indexersFooter")}>
        {(indexers.data?.indexers ?? []).map((x) => (
          <PageLink
            key={`${x.pluginId}/${x.id}`}
            search={{ plugin: x.pluginId }}
            icon="search"
            color={!x.enabled ? "#8E8E93" : failureOf(x) ? "#FF9500" : "#5856D6"}
            label={x.name}
            detail={[pluginName(x.pluginId), protocols[x.protocol](), privacies[x.privacy](), t("acquisition.priority", { priority: x.priority }), failureOf(x)]
              .filter(Boolean)
              .join(" · ")}
          />
        ))}
        {failedIndexers.map((f) => (
          <p key={`${f.pluginId}/${f.indexerId}`} className="px-4 py-3 type-callout break-words text-danger">
            {pluginName(f.pluginId)}: {f.message}
          </p>
        ))}
        {indexers.isSuccess && indexers.data.indexers.length === 0 && failedIndexers.length === 0 && (
          <p className="px-4 py-3 type-callout text-text-secondary">{t("acquisition.noIndexers")}</p>
        )}
        {indexers.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(indexers.error)}</p>}
      </Group>
      <Group title={t("acquisition.clients")} footer={t("acquisition.clientsFooter")}>
        {(clients.data?.clients ?? []).map((x) => (
          <PageLink
            key={`${x.pluginId}/${x.id}`}
            search={{ plugin: x.pluginId }}
            icon="downloads"
            color={!x.enabled ? "#8E8E93" : failureOf(x) ? "#FF9500" : "#34C759"}
            label={x.name}
            detail={[pluginName(x.pluginId), protocols[x.protocol](), t("acquisition.priority", { priority: x.priority }), failureOf(x)].filter(Boolean).join(" · ")}
          />
        ))}
        {failedClients.map(([plugin, error]) => (
          <p key={plugin} className="px-4 py-3 type-callout break-words text-danger">
            {pluginName(plugin)}: {error}
          </p>
        ))}
        {clients.isSuccess && clients.data.clients.length === 0 && failedClients.length === 0 && (
          <p className="px-4 py-3 type-callout text-text-secondary">{t("acquisition.noClients")}</p>
        )}
        {clients.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(clients.error)}</p>}
      </Group>
      {mediaKinds.map(({ kind, title }) => (
        <Group key={kind} title={t("acquisition.profilesOf", { kind: title() })}>
          {(profiles.list.data ?? [])
            .filter((p) => p.kind === kind)
            .toSorted((a, b) => a.name.localeCompare(b.name, locale))
            .map((p) => (
              <PageLink key={p.id} search={{ acquisition: "profile", id: p.id }} label={p.name} detail={p.upgradeAllowed ? t("acquisition.upgrades") : undefined} />
            ))}
          <PageLink search={{ acquisition: "profile", id: `new-${kind === MediaKind.MOVIE ? "movie" : "series"}` }} label={<span className="text-accent-text">{t("acquisition.addProfile")}</span>} />
          {profiles.list.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(profiles.list.error)}</p>}
        </Group>
      ))}
      <Group title={t("acquisition.rules")}>
        <PageLink search={{ acquisition: "qualities" }} icon="quality" color="#FF2D55" label={t("acquisition.qualities")} detail={t("acquisition.qualitiesDetail")} />
        <PageLink search={{ acquisition: "formats" }} icon="filter" color="#AF52DE" label={t("acquisition.formats")} detail={t("acquisition.formatsDetail")} />
        <PageLink search={{ acquisition: "delays" }} icon="pending" color="#FF9500" label={t("acquisition.delays")} detail={t("acquisition.delaysDetail")} />
        <PageLink search={{ acquisition: "releases" }} icon="sort" color="#007AFF" label={t("acquisition.releases")} detail={t("acquisition.releasesDetail")} />
        <PageLink search={{ acquisition: "mappings" }} icon="folder" color="#30B0C7" label={t("acquisition.mappings")} detail={t("acquisition.mappingsDetail")} />
      </Group>
    </Page>
  );
}

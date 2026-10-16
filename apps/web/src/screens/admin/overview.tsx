// The settings' Overview (docs/clients.md §6.3.6): the server chosen —
// its version and what it runs on, whether it serves and since when.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useSystemInfo } from "@mavio/client-ts";
import { GetHealthResponse_Status } from "@mavio/proto/mavio/system/v1/system_pb";

import { t } from "../../i18n.ts";
import { Group, Row } from "../../ui.tsx";
import { lengthOf, Page } from "./common.tsx";

/** The section of the server chosen in the settings. */
export function OverviewSection({ server }: { server: ServerEntry }) {
  return (
    <Page title={t("settings.overview")} message={server.name}>
      <ServerInfo server={server} />
    </Page>
  );
}

function ServerInfo({ server }: { server: ServerEntry }) {
  const { info, health } = useSystemInfo(server);
  const i = info.data;
  const serving = health.data?.status === GetHealthResponse_Status.SERVING;
  const fact = (label: string, value?: string) =>
    value && (
      <Row key={label} label={label}>
        <span className="min-w-0 break-all text-right type-callout text-text-secondary">{value}</span>
      </Row>
    );
  return (
    <Group title={t("overview.server")}>
      <Row label={t("overview.status")}>
        <span className="flex items-center gap-2 type-callout text-text-secondary">
          <span className={`size-2 rounded-full ${health.isPending ? "bg-fill" : serving ? "bg-[#34C759]" : "bg-danger"}`} />
          {health.isPending ? "…" : serving ? t("overview.serving") : health.isError ? errorMessage(health.error) : t("overview.notServing")}
        </span>
      </Row>
      {fact(t("overview.version"), i?.version)}
      {fact(t("overview.platform"), i && `${i.os}/${i.arch}`)}
      {fact(t("overview.database"), i && (i.database === "postgres" ? "PostgreSQL" : i.database === "sqlite" ? "SQLite" : i.database))}
      {fact(t("overview.ffmpeg"), i && (i.ffmpegVersion || t("overview.ffmpegMissing")))}
      {fact(t("overview.uptime"), i?.startTime && lengthOf(Date.now() - timestampDate(i.startTime).getTime()))}
      {info.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(info.error)}</p>}
    </Group>
  );
}

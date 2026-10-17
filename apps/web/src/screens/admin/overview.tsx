// The settings' Overview (docs/clients.md §6.3.6): the server chosen —
// its version and what it runs on, whether it serves and since when —, the
// load of its computer, and the plugins' containers.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, useContainerRuntime, useContainers, usePlugins, useSystemInfo } from "@mavio/client-ts";
import { ContainerMode, ContainerState } from "@mavio/proto/mavio/plugin/v1/container_pb";
import { GetHealthResponse_Status } from "@mavio/proto/mavio/system/v1/system_pb";

import { t } from "../../i18n.ts";
import { Group, Row } from "../../ui.tsx";
import { lengthOf, Page } from "./common.tsx";
import { Load } from "./load.tsx";
import { ContainerCommand, containerStates } from "../plugins.tsx";

/** The section of the server chosen in the settings. */
export function OverviewSection({ server }: { server: ServerEntry }) {
  return (
    <Page title={t("settings.overview")} message={server.name}>
      <ServerInfo server={server} />
      <Load server={server} />
      <Containers server={server} />
    </Page>
  );
}

function ServerInfo({ server }: { server: ServerEntry }) {
  const { info, health } = useSystemInfo(server);
  const i = info.data;
  const rt = useContainerRuntime(server).data;
  const engineName = (name: string) => ({ docker: "Docker", podman: "Podman" })[name] ?? name;
  const engine =
    rt &&
    (rt.available
      ? [`${engineName(rt.engine)} ${rt.version}`, `API ${rt.apiVersion}`, rt.rootless && t("overview.containerEngineRootless")].filter(Boolean).join(" · ")
      : t("overview.containerEngineNone", { reason: rt.reason }));
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
      {engine && (
        <Row label={t("overview.containerEngine")}>
          <span className="flex min-w-0 flex-col items-end text-right type-callout text-text-secondary">
            <span className="break-all">{engine}</span>
            {rt?.available && <span className="break-all type-footnote">{rt.endpoint}</span>}
          </span>
        </Row>
      )}
      {fact(t("overview.uptime"), i?.startTime && lengthOf(Date.now() - timestampDate(i.startTime).getTime()))}
      {info.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(info.error)}</p>}
    </Group>
  );
}

/** The containers of the plugins: which plugin each is for, its state, and the command it runs as. */
function Containers({ server }: { server: ServerEntry }) {
  const list = useContainers(server);
  const plugins = usePlugins(server);
  const pluginName = (id: string) => plugins.data?.find((p) => p.manifest?.id === id)?.manifest?.name || id;
  return (
    <Group title={t("overview.containers")}>
      {list.data?.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("overview.containersNone")}</p>}
      {list.data?.map((c) => {
        const st = c.status;
        const external = st?.mode === ContainerMode.EXTERNAL;
        const state =
          st &&
          (external
            ? t("overview.containerExternal", { address: st.address })
            : st.state === ContainerState.PULLING && st.progress > 0
              ? `${containerStates[st.state]()} ${Math.floor(st.progress * 100)}%`
              : containerStates[st.state]());
        const command = external ? (c.useGpu ? c.gpuCommands : c.commands)?.dockerRun : c.command;
        return (
          <div key={`${c.pluginId}/${c.dependency?.name}`} className="flex flex-col">
            <Row
              label={c.dependency?.name ?? ""}
              detail={[t("overview.containerOf", { plugin: pluginName(c.pluginId) }), st?.error ? `${state}: ${st.error}` : state].filter(Boolean).join(" · ")}
            />
            {command && <ContainerCommand command={command} />}
          </div>
        );
      })}
      {list.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(list.error)}</p>}
    </Group>
  );
}

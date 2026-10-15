// The settings' Plugins (docs/clients.md §6.3.3): the plugins of a server
// the user administers, to enable, disable, update and uninstall, those
// its catalogs offer, to install, and the catalogs.
import {
  errorMessage,
  type ServerEntry,
  useAdminServers,
  useCatalogPlugins,
  usePluginActions,
  usePluginCatalogs,
  usePlugins,
} from "@mavio/client-ts";
import { type CatalogPlugin, type Plugin, PluginState } from "@mavio/proto/mavio/system/v1/system_pb";
import { type FormEvent, useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Button, Group, Row, Select, Toggle } from "../ui.tsx";

const states: Record<PluginState, () => string> = {
  [PluginState.UNSPECIFIED]: () => "",
  [PluginState.READY]: () => t("plugins.stateReady"),
  [PluginState.UNCONFIGURED]: () => t("plugins.stateUnconfigured"),
  [PluginState.FAILED]: () => t("plugins.stateFailed"),
  [PluginState.DISABLED]: () => t("plugins.stateDisabled"),
};

/** The section, of the first server the user administers or the one chosen. */
export function PluginsSection() {
  const admin = useAdminServers();
  const [serverId, setServerId] = useState<string>();
  const server = admin.find((s) => s.id === serverId) ?? admin[0];
  if (!server) return null;
  return (
    <div className="flex flex-col gap-7">
      <header>
        <h2 className="type-title1">{t("settings.plugins")}</h2>
        <p className="mt-1 type-body text-text-secondary">{t("settings.pluginsMessage", { server: server.name })}</p>
      </header>
      {admin.length > 1 && (
        <Select value={server.id} label={t("auth.server")} options={admin.map((s) => ({ value: s.id, label: s.name }))} onChange={setServerId} />
      )}
      <ServerPlugins key={server.id} server={server} />
    </div>
  );
}

function ServerPlugins({ server }: { server: ServerEntry }) {
  const installed = usePlugins(server);
  const catalog = useCatalogPlugins(server);
  const actions = usePluginActions(server);
  const offered = new Map((catalog.data ?? []).map((p) => [p.id, p]));
  const available = (catalog.data ?? []).filter((p) => !p.installedVersion && p.versions.length > 0);
  const failure = [actions.install, actions.uninstall, actions.setEnabled, actions.setCatalogs].find((m) => m.isError)?.error;
  return (
    <>
      <Group title={t("plugins.installed")}>
        {installed.data?.length === 0 && <Row label={<span className="text-text-secondary">{t("plugins.noneInstalled")}</span>} />}
        {(installed.data ?? []).map((p) => (
          <InstalledRow key={p.manifest?.id} plugin={p} offered={offered.get(p.manifest?.id ?? "")} actions={actions} />
        ))}
      </Group>
      {failure && <p className="type-callout text-danger">{errorMessage(failure)}</p>}
      <Group title={t("plugins.available")}>
        {catalog.isError ? (
          <Row label={<span className="text-danger">{t("plugins.catalogsUnreachable", { reason: errorMessage(catalog.error) })}</span>} />
        ) : (
          catalog.isSuccess && available.length === 0 && <Row label={<span className="text-text-secondary">{t("plugins.noneAvailable")}</span>} />
        )}
        {available.map((p) => (
          <Row key={p.id} label={p.name || p.id} detail={describe(p)} leading={<PluginTile />}>
            <Button
              variant="secondary"
              disabled={actions.install.isPending}
              onClick={() => actions.install.mutate({ id: p.id })}
            >
              {actions.install.isPending && actions.install.variables?.id === p.id ? "…" : t("plugins.install")}
            </Button>
          </Row>
        ))}
      </Group>
      <Catalogs server={server} actions={actions} />
    </>
  );
}

/** A catalog plugin's version, author and description. */
function describe(p: CatalogPlugin): string {
  return [p.versions[0]?.version, p.author && t("plugins.by", { author: p.author }), p.description].filter(Boolean).join(" · ");
}

function PluginTile() {
  return (
    <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[9px] bg-fill">
      <Icon name="plugins" size={20} />
    </span>
  );
}

type Actions = ReturnType<typeof usePluginActions>;

/** An installed plugin: its state, its switch, an update when its catalog has a newer version, and uninstalling. */
function InstalledRow({ plugin, offered, actions }: { plugin: Plugin; offered?: CatalogPlugin; actions: Actions }) {
  const [confirming, setConfirming] = useState(false);
  const m = plugin.manifest;
  const id = m?.id ?? "";
  const name = m?.name || id;
  const enabled = plugin.state !== PluginState.DISABLED;
  const newest = offered?.versions[0]?.version;
  const update = newest && m?.version && newest !== m.version ? newest : undefined;
  const busy = (x: { isPending: boolean; variables?: unknown }) =>
    x.isPending && (x.variables === id || (x.variables as { id?: string } | undefined)?.id === id);
  const state = states[plugin.state]();
  const detail = [m?.version, plugin.state === PluginState.FAILED && plugin.error ? `${state}: ${plugin.error}` : state, m?.description]
    .filter(Boolean)
    .join(" · ");
  if (confirming) {
    return (
      <Row label={t("plugins.uninstallTitle", { name })} detail={t("plugins.uninstallMessage")} leading={<PluginTile />}>
        <Button variant="plain" onClick={() => setConfirming(false)}>
          {t("action.cancel")}
        </Button>
        <Button
          variant="secondary"
          className="text-danger"
          disabled={actions.uninstall.isPending}
          onClick={() => actions.uninstall.mutate(id, { onSettled: () => setConfirming(false) })}
        >
          {t("plugins.uninstall")}
        </Button>
      </Row>
    );
  }
  return (
    <Row
      label={
        <span className={plugin.state === PluginState.FAILED ? "text-danger" : enabled ? "" : "text-text-secondary"}>{name}</span>
      }
      detail={detail}
      leading={<PluginTile />}
    >
      {update && (
        <Button variant="secondary" disabled={actions.install.isPending} onClick={() => actions.install.mutate({ id, version: update })}>
          {busy(actions.install) ? "…" : t("plugins.update", { version: update })}
        </Button>
      )}
      <Button variant="plain" className="text-danger" onClick={() => setConfirming(true)}>
        {t("plugins.uninstall")}
      </Button>
      <Toggle
        on={busy(actions.setEnabled) ? !enabled : enabled}
        label={t("plugins.enabled")}
        onChange={(on) => !actions.setEnabled.isPending && actions.setEnabled.mutate({ id, enabled: on })}
      />
    </Row>
  );
}

/** The catalogs' URLs, to add to and remove from. */
function Catalogs({ server, actions }: { server: ServerEntry; actions: Actions }) {
  const catalogs = usePluginCatalogs(server);
  const [url, setUrl] = useState("");
  const list = catalogs.data ?? [];
  const add = (e: FormEvent) => {
    e.preventDefault();
    const u = url.trim();
    if (!u || list.includes(u)) return;
    actions.setCatalogs.mutate([...list, u], { onSuccess: () => setUrl("") });
  };
  return (
    <Group title={t("plugins.catalogs")} footer={t("plugins.catalogsFooter")}>
      {list.map((c) => (
        <Row key={c} label={<span className="break-all">{c}</span>}>
          <Button
            variant="plain"
            className="text-danger"
            disabled={actions.setCatalogs.isPending}
            onClick={() => actions.setCatalogs.mutate(list.filter((o) => o !== c))}
          >
            {t("plugins.removeCatalog")}
          </Button>
        </Row>
      ))}
      <form onSubmit={add} className="flex min-h-13 items-center gap-3 px-4 py-2.5">
        <input
          type="url"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          placeholder={t("plugins.catalogAddress")}
          aria-label={t("plugins.catalogAddress")}
          className="min-w-0 flex-1 bg-transparent type-body outline-none placeholder:text-text-tertiary"
        />
        <Button type="submit" variant="secondary" disabled={!url.trim() || actions.setCatalogs.isPending}>
          {t("plugins.addCatalog")}
        </Button>
      </form>
    </Group>
  );
}

// A download client's page in Acquisition (docs/clients.md §6.3.11): a
// link to the plugin that set it up, and the folders the client puts
// downloads in, with where the server reads each, whether it can, and the
// libraries imports from it hard link to.
import { create } from "@bufbuild/protobuf";
import { errorMessage, type ServerEntry, useDownloadClientPaths, useDownloadClients, usePlugins, useUpdateDownloadClientPaths } from "@mavio/client-ts";
import { type DownloadClientPath, DownloadClientPathSchema } from "@mavio/proto/mavio/acquisition/v1/download_pb";
import { useEffect, useState } from "react";

import { t } from "../../i18n.ts";
import { Button, Group } from "../../ui.tsx";
import { input } from "../../controls/styles.ts";
import { useUnsavedGuard } from "../../controls/unsaved.tsx";
import { AddRow, BackLink, Page, PageLink } from "./common.tsx";

/** A folder being edited: added ones name the client's folder too. */
interface Draft {
  remote: string;
  local: string;
  added?: boolean;
  /** Mounted in the client's managed container, which is not edited here. */
  container?: boolean;
}

const draftsOf = (paths: DownloadClientPath[]): Draft[] =>
  paths.map((p) => ({ remote: p.remotePath, local: p.localPath, ...(p.container ? { container: true } : {}) }));

/** Where the server may read a folder: empty, an absolute path or a remote's folder. */
const validLocal = (s: string) => {
  const v = s.trim();
  return v === "" || /^(\/|[A-Za-z]:[\\/]|\\\\)/.test(v) || /^remote:[^/\\:]+:(?!\/)/.test(v);
};

/** Acquisition's page of a download client, by "<plugin>/<client>". */
export function DownloadClientPage({ server, id }: { server: ServerEntry; id: string }) {
  const slash = id.indexOf("/");
  const pluginId = slash < 0 ? "" : id.slice(0, slash);
  const clientId = slash < 0 ? "" : id.slice(slash + 1);
  const clients = useDownloadClients(server);
  const plugins = usePlugins(server);
  const paths = useDownloadClientPaths(server, pluginId, clientId);
  const update = useUpdateDownloadClientPaths(server, pluginId, clientId);
  const client = clients.data?.clients.find((c) => c.pluginId === pluginId && c.id === clientId);
  const pluginName = plugins.data?.find((p) => p.manifest?.id === pluginId)?.manifest?.name || pluginId;
  const saved = paths.data?.paths;
  const [d, setD] = useState<Draft[]>();
  useEffect(() => setD(saved && draftsOf(saved)), [saved]);
  const changed = saved !== undefined && d !== undefined && JSON.stringify(d) !== JSON.stringify(draftsOf(saved));
  const guard = useUnsavedGuard(changed);
  const valid = (d ?? []).every((x) => x.remote.trim() !== "" && validLocal(x.local));
  const set = (i: number, patch: Partial<Draft>) => setD((xs) => xs?.map((x, j) => (j === i ? { ...x, ...patch } : x)));
  const save = () =>
    d &&
    update.mutate(
      d.filter((x) => !x.container && x.local.trim() !== "").map((x) => create(DownloadClientPathSchema, { remotePath: x.remote.trim(), localPath: x.local.trim() })),
    );
  return (
    <Page title={client?.name ?? clientId} message={t("client.message")} back={<BackLink section="acquisition" label={t("settings.acquisition")} />}>
      <Group>
        <PageLink search={{ plugin: pluginId }} icon="plugins" color="#5856D6" label={t("client.settings")} detail={pluginName} />
      </Group>
      <Group title={t("client.folders")} footer={t("client.foldersFooter")}>
        {(d ?? []).map((x, i) => {
          const p = !x.added ? saved?.find((s) => s.remotePath === x.remote) : undefined;
          const unchanged = p !== undefined && x.local === p.localPath;
          return (
            <div key={x.added ? `added-${i}` : x.remote} className="flex flex-col gap-2 px-4 py-3">
              {x.added ? (
                <input
                  className={input}
                  aria-label={t("client.clientFolder")}
                  placeholder={t("client.clientFolder")}
                  value={x.remote}
                  onChange={(e) => set(i, { remote: e.target.value })}
                />
              ) : (
                <span className="type-body break-all font-mono">{x.remote}</span>
              )}
              <div className="flex items-center gap-2">
                <span aria-hidden className="text-text-secondary">
                  →
                </span>
                {x.container ? (
                  <span className="type-body break-all font-mono">{x.local}</span>
                ) : (
                  <input
                    className={`${input} flex-1`}
                    aria-label={t("client.serverFolder")}
                    placeholder={t("client.readAt")}
                    value={x.local}
                    aria-invalid={!validLocal(x.local)}
                    onChange={(e) => set(i, { local: e.target.value })}
                  />
                )}
                {!x.container && (x.added || (p && !p.rootFolder)) && (
                  <Button variant="plain" className="text-danger" onClick={() => setD((xs) => xs?.filter((_, j) => j !== i))}>
                    {t("rules.remove")}
                  </Button>
                )}
              </div>
              {p && unchanged && (
                <p className="type-footnote break-words text-text-secondary">
                  {x.container && (
                    <>
                      {t("client.containerFolder")}
                      {" · "}
                    </>
                  )}
                  {p.readable ? (
                    <>
                      <span className="text-success">{t("client.readable")}</span>
                      {" · "}
                      {p.linkLibraryPaths.length ? t("client.links", { paths: p.linkLibraryPaths.join(", ") }) : t("client.copies")}
                    </>
                  ) : (
                    <span className="text-danger">{p.error}</span>
                  )}
                </p>
              )}
              {p && unchanged && !p.readable && p.suggestedPath && x.local === "" && (
                <div>
                  <Button variant="secondary" onClick={() => set(i, { local: p.suggestedPath })}>
                    {t("client.use", { path: p.suggestedPath })}
                  </Button>
                </div>
              )}
            </div>
          );
        })}
        {paths.isSuccess && d?.length === 0 && !paths.data.failure && <p className="px-4 py-3 type-callout text-text-secondary">{t("client.noFolders")}</p>}
        {paths.data?.failure && <p className="line-clamp-4 px-4 py-3 type-callout break-words text-danger" title={paths.data.failure}>{t("client.unreported", { failure: paths.data.failure })}</p>}
        {paths.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(paths.error)}</p>}
        {d && <AddRow label={t("client.addFolder")} onClick={() => setD((xs) => [...(xs ?? []), { remote: "", local: "", added: true }])} />}
      </Group>
      {d && (
        <div className="flex flex-wrap items-center gap-3">
          <Button disabled={!changed || !valid || update.isPending} onClick={save}>
            {update.isPending ? "…" : t("action.save")}
          </Button>
          {update.isError && <span className="type-callout text-danger">{errorMessage(update.error)}</span>}
        </div>
      )}
      {guard.dialog}
    </Page>
  );
}

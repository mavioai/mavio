// All the items of a hub, from its shelf's "See All".
import { useHubItems, useServer, useSignedIn } from "@mavio/client-ts";
import type { ServerEntry } from "@mavio/client-ts";
import { HubKind } from "@mavio/proto/mavio/library/v1/library_pb";
import { getRouteApi } from "@tanstack/react-router";

import { t } from "../i18n.ts";
import { Grid, ItemCard, LoadMore, Loading, ScreenTitle, StatusView } from "../ui.tsx";
import { sectionTitle } from "./home.tsx";

const route = getRouteApi("/s/$serverId/hub/$hubId");

export function HubScreen() {
  const { serverId, hubId } = route.useParams();
  const server = useServer(serverId);
  if (!server?.token) return <StatusView icon="error" title={t("auth.unreachable")} />;
  return <Hub server={server} hubId={hubId} several={useSignedIn().length > 1} />;
}

function Hub({ server, hubId, several }: { server: ServerEntry; hubId: string; several: boolean }) {
  const q = useHubItems(server, hubId);
  if (q.isPending) return <Loading />;
  if (q.isError) return <StatusView icon="error" title={t("state.error")} action={{ label: t("action.retry"), onClick: () => void q.refetch() }} />;
  const hub = q.data.pages[0]?.hub;
  const items = q.data.pages.flatMap((p) => p.hub?.items ?? []);
  const title = hub
    ? sectionTitle({
        id: hub.id,
        kind: hub.kind,
        libraryName: hub.libraryName,
        seedName: hub.seedName,
        serverName: server.name,
        more: false,
        offline: false,
        content: { type: "loading" },
      })
    : "";
  return (
    <div>
      <ScreenTitle>
        {title}
        {several && hub?.kind !== HubKind.CONTINUE_WATCHING && (
          <span className="ml-3 type-callout text-text-secondary">{server.name}</span>
        )}
      </ScreenTitle>
      <div className="pt-2">
        <Grid>
          {items.map((item) => (
            <ItemCard key={item.id} server={server} serverId={server.id} item={item} fluid />
          ))}
        </Grid>
        <LoadMore active={q.hasNextPage && !q.isFetchingNextPage} onVisible={() => void q.fetchNextPage()} />
      </div>
    </div>
  );
}

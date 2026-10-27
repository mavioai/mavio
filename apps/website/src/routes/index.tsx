import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import { accounts } from "../lib/api.ts";
import { describe, useRich, useT } from "../lib/locale.tsx";
import { useAccount } from "../lib/session.ts";
import { Button, Empty, ErrorText, Group, Icon, Loading, Name, Page, Row } from "../ui.tsx";

export const Route = createFileRoute("/")({ component: Servers });

function Servers() {
  const t = useT();
  const rich = useRich();
  const me = useAccount();
  const qc = useQueryClient();
  const servers = useQuery({ queryKey: ["servers"], queryFn: () => accounts.listServers({}), enabled: !!me });
  const invitations = useQuery({ queryKey: ["invitations"], queryFn: () => accounts.listInvitations({}), enabled: !!me });
  const answer = useMutation({
    mutationFn: async ({ serverId, accept }: { serverId: string; accept: boolean }) => {
      if (accept) await accounts.acceptInvitation({ serverId });
      else await accounts.declineInvitation({ serverId });
    },
    onSuccess: () => qc.invalidateQueries(),
  });
  if (!me || servers.isPending) return <Loading />;

  const pending = invitations.data?.invitations ?? [];
  return (
    <Page title={t("site.nav.servers")}>
      {pending.length > 0 && (
        <Group title={t("site.invitations.title")} footer={<ErrorText error={describe(t, answer.error)} />}>
          {pending.map((i) => (
            <Row
              key={i.serverId}
              leading={<Icon name="server" size={28} className="text-accent" />}
              label={rich("site.invitations.server", {
                name: i.administrator?.displayName || i.administrator?.userName,
                server: <Name>{i.serverName || i.serverId}</Name>,
              })}
            >
              <Button onClick={() => answer.mutate({ serverId: i.serverId, accept: true })}>{t("site.action.accept")}</Button>
              <Button variant="secondary" onClick={() => answer.mutate({ serverId: i.serverId, accept: false })}>
                {t("site.action.decline")}
              </Button>
            </Row>
          ))}
        </Group>
      )}
      <ErrorText error={describe(t, servers.error)} />
      {servers.data?.servers.length === 0 ? (
        <Empty icon="server" title={t("site.servers.empty")} message={t("site.servers.emptyBody")}>
          <Link to="/claim" className="mt-2 inline-flex min-h-9 items-center rounded-md bg-button px-4 type-headline text-on-button hover:opacity-85">
            {t("site.nav.claim")}
          </Link>
        </Empty>
      ) : (
        servers.data && (
          <Group>
            {servers.data.servers.map((s) => (
              <Link key={s.id} to="/servers/$id" params={{ id: s.id }} className="transition-colors hover:bg-fill">
                <Row
                  leading={
                    <span className="flex size-10 items-center justify-center rounded-[10px] bg-fill text-accent">
                      <Icon name="server" size={22} />
                    </span>
                  }
                  label={<span className="font-semibold">{s.name || s.id}</span>}
                  detail={[
                    s.administrator?.id === me.id ? t("site.servers.administer") : t("site.servers.administeredBy", { name: s.administrator?.displayName || s.administrator?.userName || "" }),
                    t("site.servers.libraries", { count: s.libraries.length }),
                  ].join(" · ")}
                >
                  <Icon name="forward" size={18} className="text-text-tertiary" />
                </Row>
              </Link>
            ))}
          </Group>
        )
      )}
    </Page>
  );
}

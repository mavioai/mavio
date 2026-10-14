import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link } from "@tanstack/react-router";

import { accounts, message } from "../lib/api.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Muted, Page } from "../ui.tsx";

export const Route = createFileRoute("/")({ component: Servers });

function Servers() {
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
  if (!me) return null;

  return (
    <Page title="Servers">
      {invitations.data?.invitations.map((i) => (
        <Card key={i.serverId} title={`Invitation to ${i.serverName || i.serverId}`}>
          <Muted>
            {i.administrator?.displayName || i.administrator?.userName} invites you to their server.
          </Muted>
          <div className="flex gap-3">
            <Button onClick={() => answer.mutate({ serverId: i.serverId, accept: true })}>Accept</Button>
            <Button kind="secondary" onClick={() => answer.mutate({ serverId: i.serverId, accept: false })}>
              Decline
            </Button>
          </div>
        </Card>
      ))}
      <ErrorText error={answer.error ? message(answer.error) : undefined} />
      <ErrorText error={servers.error ? message(servers.error) : undefined} />
      {servers.data?.servers.length === 0 && (
        <Card>
          <Muted>You have no servers yet. Claim one you run, or ask a server's administrator for access.</Muted>
          <Link to="/claim" className="type-callout text-accent">
            Claim a server
          </Link>
        </Card>
      )}
      {servers.data?.servers.map((s) => (
        <Link key={s.id} to="/servers/$id" params={{ id: s.id }}>
          <Card title={s.name || s.id}>
            <Muted>
              {s.administrator?.id === me.id ? "You administer this server" : `Administered by ${s.administrator?.userName}`}
              {" · "}
              {s.libraries.length} {s.libraries.length === 1 ? "library" : "libraries"}
            </Muted>
          </Card>
        </Link>
      ))}
    </Page>
  );
}

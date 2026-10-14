import { create } from "@bufbuild/protobuf";
import { type Access, AccessSchema, type Library } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useNavigate } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { accounts, message } from "../../lib/api.ts";
import { useAccount } from "../../lib/session.ts";
import { Button, Card, Check, ErrorText, Field, Muted, Page } from "../../ui.tsx";

export const Route = createFileRoute("/servers/$id")({ component: ServerPage });

function ServerPage() {
  const { id } = Route.useParams();
  const me = useAccount();
  const qc = useQueryClient();
  const navigate = useNavigate();
  const servers = useQuery({ queryKey: ["servers"], queryFn: () => accounts.listServers({}), enabled: !!me });
  const server = servers.data?.servers.find((s) => s.id === id);
  const admin = !!me && server?.administrator?.id === me.id;
  const leave = useMutation({
    mutationFn: async () => {
      if (admin) await accounts.removeServer({ serverId: id });
      else await accounts.leaveServer({ serverId: id });
    },
    onSuccess: async () => {
      await qc.invalidateQueries();
      await navigate({ to: "/" });
    },
  });
  if (!me || !servers.data) return null;
  if (!server) return <Page title="Server">{<Muted>You have no access to this server.</Muted>}</Page>;

  return (
    <Page title={server.name || server.id}>
      <Card title="Libraries">
        {server.libraries.length === 0 && <Muted>The server has published no libraries yet.</Muted>}
        <ul className="type-body flex flex-col gap-1">
          {server.libraries.map((l) => (
            <li key={l.id}>
              {l.name} <span className="text-text-tertiary">· {l.kind}</span>
            </li>
          ))}
        </ul>
      </Card>
      {admin && <AccessRecords serverId={id} libraries={server.libraries} />}
      <Card>
        <Muted>
          {admin
            ? "Removing the server from mavio.dev deletes its access records; the server keeps running until you change its mode."
            : "Leaving the server removes your access; its administrator can invite you again."}
        </Muted>
        <Button kind="danger" onClick={() => confirm(admin ? "Remove this server?" : "Leave this server?") && leave.mutate()}>
          {admin ? "Remove server" : "Leave server"}
        </Button>
        <ErrorText error={leave.error ? message(leave.error) : undefined} />
      </Card>
    </Page>
  );
}

function AccessRecords({ serverId, libraries }: { serverId: string; libraries: Library[] }) {
  const qc = useQueryClient();
  const records = useQuery({ queryKey: ["access", serverId], queryFn: () => accounts.listAccess({ serverId }) });
  const [editing, setEditing] = useState<{ userName: string; access: Access }>();
  const revoke = useMutation({
    mutationFn: (accountId: string) => accounts.revokeAccess({ serverId, accountId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["access", serverId] }),
  });
  return (
    <Card title="Access">
      {records.data?.records.length === 0 && <Muted>Nobody else has access yet.</Muted>}
      <ul className="flex flex-col gap-3">
        {records.data?.records.map((r) => (
          <li key={r.account?.id} className="flex items-center gap-3">
            <span className="type-body flex-1">
              {r.account?.displayName} <span className="text-text-tertiary">@{r.account?.userName}</span>
              {r.invitation && <span className="type-footnote text-text-secondary"> · invited</span>}
            </span>
            <Button kind="secondary" onClick={() => setEditing({ userName: r.account?.userName ?? "", access: r.access ?? create(AccessSchema) })}>
              Edit
            </Button>
            <Button kind="danger" onClick={() => r.account && revoke.mutate(r.account.id)}>
              {r.invitation ? "Withdraw" : "Revoke"}
            </Button>
          </li>
        ))}
      </ul>
      <ErrorText error={revoke.error ? message(revoke.error) : undefined} />
      <Grant
        key={editing?.userName ?? ""}
        serverId={serverId}
        libraries={libraries}
        initial={editing}
        onDone={() => setEditing(undefined)}
      />
    </Card>
  );
}

function Grant({
  serverId,
  libraries,
  initial,
  onDone,
}: {
  serverId: string;
  libraries: Library[];
  initial?: { userName: string; access: Access };
  onDone: () => void;
}) {
  const qc = useQueryClient();
  const a = initial?.access;
  const [all, setAll] = useState(a ? a.allLibraries : true);
  const grant = useMutation({
    mutationFn: (req: { userName: string; access: Access }) => accounts.grantAccess({ serverId, ...req }),
    onSuccess: async () => {
      await qc.invalidateQueries({ queryKey: ["access", serverId] });
      onDone();
    },
  });

  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    const f = new FormData(e.currentTarget);
    const rating = String(f.get("rating"));
    const labels = (name: string) =>
      String(f.get(name))
        .split(",")
        .map((l) => l.trim())
        .filter(Boolean);
    grant.mutate({
      userName: String(f.get("userName")),
      access: create(AccessSchema, {
        allLibraries: all,
        libraryIds: all ? [] : f.getAll("library").map(String),
        maxParentalRating: rating === "" ? undefined : Number(rating),
        blockUnrated: f.get("blockUnrated") === "on",
        allowedLabels: labels("allowedLabels"),
        blockedLabels: labels("blockedLabels"),
        allowDownload: f.get("allowDownload") === "on",
      }),
    });
  }

  return (
    <form className="flex flex-col gap-3 border-t border-border pt-4" onSubmit={submit}>
      <h3 className="type-headline">{initial ? `Access of @${initial.userName}` : "Invite an account"}</h3>
      <Field label="User name" name="userName" defaultValue={initial?.userName} readOnly={!!initial} required />
      <Check label="Every library" checked={all} onChange={(e) => setAll(e.target.checked)} />
      {!all &&
        libraries.map((l) => (
          <Check key={l.id} label={l.name} name="library" value={l.id} defaultChecked={a?.libraryIds.includes(l.id)} />
        ))}
      <Field
        label="Highest rating score shown (empty for every rating)"
        name="rating"
        type="number"
        min={0}
        defaultValue={a?.maxParentalRating}
      />
      <Check label="Hide unrated items" name="blockUnrated" defaultChecked={a?.blockUnrated} />
      <Field label="Show only items with these labels" name="allowedLabels" defaultValue={a?.allowedLabels.join(", ")} />
      <Field label="Hide items with these labels" name="blockedLabels" defaultValue={a?.blockedLabels.join(", ")} />
      <Check label="Allow downloads" name="allowDownload" defaultChecked={a?.allowDownload} />
      <ErrorText error={grant.error ? message(grant.error) : undefined} />
      <div className="flex gap-3">
        <Button type="submit" disabled={grant.isPending}>
          {initial ? "Save" : "Invite"}
        </Button>
        {initial && (
          <Button kind="secondary" onClick={onDone}>
            Cancel
          </Button>
        )}
      </div>
    </form>
  );
}

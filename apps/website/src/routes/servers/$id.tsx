import { create } from "@bufbuild/protobuf";
import { type Access, AccessSchema, type Library } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, Link, useNavigate } from "@tanstack/react-router";
import { useState } from "react";

import { accounts } from "../../lib/api.ts";
import { describe, useT } from "../../lib/locale.tsx";
import { useAccount } from "../../lib/session.ts";
import { Actions, Avatar, Block, Button, Check, Empty, ErrorText, Field, Group, Icon, Loading, Muted, Page, Row, useConfirm } from "../../ui.tsx";

export const Route = createFileRoute("/servers/$id")({ component: ServerPage });

function ServerPage() {
  const t = useT();
  const confirm = useConfirm();
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
  if (!me || !servers.data) return <Loading />;
  if (!server) {
    return (
      <Page title={t("site.nav.servers")}>
        <Empty icon="server" title={t("site.server.unknown")} />
      </Page>
    );
  }
  const name = server.name || server.id;

  return (
    <Page
      title={
        <span className="flex flex-col gap-2">
          <Link to="/" className="inline-flex items-center gap-0.5 self-start type-body font-normal text-accent-text hover:opacity-80">
            <Icon name="back" size={18} />
            {t("site.nav.servers")}
          </Link>
          {name}
        </span>
      }
    >
      <Group title={t("site.server.libraries")}>
        {server.libraries.length === 0 ? (
          <Block>
            <Muted>{t("site.server.noLibraries")}</Muted>
          </Block>
        ) : (
          server.libraries.map((l) => <Row key={l.id} leading={<Icon name="folder" size={22} className="text-text-secondary" />} label={l.name} detail={l.kind} />)
        )}
      </Group>
      {admin && <AccessRecords serverId={id} libraries={server.libraries} />}
      <Group footer={t(admin ? "site.server.removeBody" : "site.server.leaveBody")}>
        <Block>
          <Actions>
            <Button
              variant="danger"
              onClick={async () => {
                if (await confirm({ title: t(admin ? "site.server.removeConfirm" : "site.server.leaveConfirm", { name }), action: t(admin ? "site.server.remove" : "site.server.leave"), destructive: true }))
                  leave.mutate();
              }}
            >
              {t(admin ? "site.server.remove" : "site.server.leave")}
            </Button>
          </Actions>
          <ErrorText error={describe(t, leave.error)} />
        </Block>
      </Group>
    </Page>
  );
}

function AccessRecords({ serverId, libraries }: { serverId: string; libraries: Library[] }) {
  const t = useT();
  const qc = useQueryClient();
  const records = useQuery({ queryKey: ["access", serverId], queryFn: () => accounts.listAccess({ serverId }) });
  const [editing, setEditing] = useState<{ userName: string; access: Access }>();
  const revoke = useMutation({
    mutationFn: (accountId: string) => accounts.revokeAccess({ serverId, accountId }),
    onSuccess: () => qc.invalidateQueries({ queryKey: ["access", serverId] }),
  });
  return (
    <>
      <Group title={t("site.server.access")} footer={<ErrorText error={describe(t, revoke.error)} />}>
        {records.data?.records.length === 0 && (
          <Block>
            <Muted>{t("site.server.noAccess")}</Muted>
          </Block>
        )}
        {records.data?.records.map((r) => (
          <Row
            key={r.account?.id}
            leading={<Avatar name={r.account?.displayName || r.account?.userName || "?"} />}
            label={r.account?.displayName || r.account?.userName}
            detail={[`@${r.account?.userName}`, r.invitation && t("site.server.invited")].filter(Boolean).join(" · ")}
          >
            <Button variant="secondary" onClick={() => setEditing({ userName: r.account?.userName ?? "", access: r.access ?? create(AccessSchema) })}>
              {t("site.action.edit")}
            </Button>
            <Button variant="danger" onClick={() => r.account && revoke.mutate(r.account.id)}>
              {t(r.invitation ? "site.action.withdraw" : "site.server.revoke")}
            </Button>
          </Row>
        ))}
      </Group>
      <Grant key={editing?.userName ?? ""} serverId={serverId} libraries={libraries} initial={editing} onDone={() => setEditing(undefined)} />
    </>
  );
}

function Grant({ serverId, libraries, initial, onDone }: { serverId: string; libraries: Library[]; initial?: { userName: string; access: Access }; onDone: () => void }) {
  const t = useT();
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

  function submit(f: FormData, form: HTMLFormElement) {
    const rating = String(f.get("rating"));
    const labels = (name: string) =>
      String(f.get(name))
        .split(",")
        .map((l) => l.trim())
        .filter(Boolean);
    grant.mutate(
      {
        userName: String(f.get("userName")).trim(),
        access: create(AccessSchema, {
          allLibraries: all,
          libraryIds: all ? [] : f.getAll("library").map(String),
          maxParentalRating: rating === "" ? undefined : Number(rating),
          blockUnrated: f.get("blockUnrated") === "on",
          allowedLabels: labels("allowedLabels"),
          blockedLabels: labels("blockedLabels"),
          allowDownload: f.get("allowDownload") === "on",
        }),
      },
      { onSuccess: () => !initial && form.reset() },
    );
  }

  return (
    <Group title={initial ? t("site.server.accessOf", { name: initial.userName }) : t("site.server.invite")}>
      <Block>
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            submit(new FormData(e.currentTarget), e.currentTarget);
          }}
        >
          <Field label={t("site.field.userName")} name="userName" defaultValue={initial?.userName} readOnly={!!initial} autoCapitalize="none" spellCheck={false} required />
          <div className="flex flex-col gap-1">
            <Check label={t("site.server.everyLibrary")} checked={all} onChange={(e) => setAll(e.target.checked)} />
            {!all && (
              <div className="flex flex-col gap-1 pl-7">
                {libraries.map((l) => (
                  <Check key={l.id} label={l.name} name="library" value={l.id} defaultChecked={a?.libraryIds.includes(l.id)} />
                ))}
              </div>
            )}
          </div>
          <Field label={t("site.server.rating")} name="rating" type="number" min={0} placeholder={t("site.server.ratingHint")} defaultValue={a?.maxParentalRating} />
          <div className="flex flex-col gap-4 sm:flex-row">
            <Field label={t("site.server.allowedLabels")} name="allowedLabels" placeholder={t("site.server.labelsHint")} defaultValue={a?.allowedLabels.join(", ")} />
            <Field label={t("site.server.blockedLabels")} name="blockedLabels" placeholder={t("site.server.labelsHint")} defaultValue={a?.blockedLabels.join(", ")} />
          </div>
          <div className="flex flex-col gap-1">
            <Check label={t("site.server.blockUnrated")} name="blockUnrated" defaultChecked={a?.blockUnrated} />
            <Check label={t("site.server.allowDownload")} name="allowDownload" defaultChecked={a?.allowDownload} />
          </div>
          <ErrorText error={describe(t, grant.error)} />
          <Actions>
            <Button type="submit" disabled={grant.isPending}>
              {initial ? t("site.action.save") : t("site.action.invite")}
            </Button>
            {initial && (
              <Button variant="secondary" onClick={onDone}>
                {t("site.action.cancel")}
              </Button>
            )}
          </Actions>
        </form>
      </Block>
    </Group>
  );
}

import { create } from "@bufbuild/protobuf";
import { AccessSchema, AccountKind } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import type { Home, HomeInvitation, HomeMember } from "@mavio/proto/mavio/cloud/v1/home_pb";
import { RestrictionProfile } from "@mavio/proto/mavio/user/v1/user_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useRouter } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { accounts, homes, switchTo } from "../lib/api.ts";
import type { MessageKey } from "../lib/i18n.ts";
import { describe, useRich, useT } from "../lib/locale.tsx";
import { useAccount } from "../lib/session.ts";
import { Actions, Avatar, Block, Button, Check, ErrorText, Field, Group, Icon, Loading, Muted, Name, Page, Row, Select, useConfirm } from "../ui.tsx";

export const Route = createFileRoute("/home")({ component: HomePage });

const restrictions: [RestrictionProfile, MessageKey][] = [
  [RestrictionProfile.UNSPECIFIED, "site.restriction.none"],
  [RestrictionProfile.YOUNGER_CHILD, "site.restriction.youngerChild"],
  [RestrictionProfile.OLDER_CHILD, "site.restriction.olderChild"],
  [RestrictionProfile.TEEN, "site.restriction.teen"],
];

const nameOf = (m?: { displayName: string; userName: string }) => m?.displayName || m?.userName || "";

function HomePage() {
  const t = useT();
  const me = useAccount();
  const data = useQuery({ queryKey: ["home"], queryFn: () => homes.getHome({}), enabled: !!me });
  if (!me || data.isPending) return <Loading />;
  if (!data.data) {
    return (
      <Page title={t("site.nav.home")}>
        <ErrorText error={describe(t, data.error)} />
      </Page>
    );
  }
  const { home, invitations } = data.data;
  return (
    <Page title={home?.name || t("site.nav.home")}>
      {invitations.length > 0 && (
        <Group title={t("site.invitations.title")}>
          {invitations.map((i) => (
            <Invitation key={i.homeId} invitation={i} />
          ))}
        </Group>
      )}
      {home ? <Members home={home} me={me.id} /> : <CreateHome />}
    </Page>
  );
}

function useDone() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries();
}

function Invitation({ invitation: i }: { invitation: HomeInvitation }) {
  const t = useT();
  const rich = useRich();
  const done = useDone();
  const answer = useMutation({
    mutationFn: async (accept: boolean) => {
      if (accept) await homes.acceptHomeInvitation({ homeId: i.homeId });
      else await homes.declineHomeInvitation({ homeId: i.homeId });
    },
    onSuccess: done,
  });
  const servers = i.offers.map((o) => o.serverName || o.serverId).join(", ");
  return (
    <Block>
      <div className="flex items-start gap-3">
        <Icon name="home" size={28} className="shrink-0 text-accent" />
        <div className="flex flex-col gap-1">
          <p className="type-body">{rich("site.home.invitation", { name: nameOf(i.administrator), home: <Name>{i.homeName}</Name> })}</p>
          <Muted>{[servers && t("site.home.invitationServers", { servers }), t("site.home.joinLeaves")].filter(Boolean).join(" ")}</Muted>
        </div>
      </div>
      <Actions>
        <Button onClick={() => answer.mutate(true)}>{t("site.home.join")}</Button>
        <Button variant="secondary" onClick={() => answer.mutate(false)}>
          {t("site.action.decline")}
        </Button>
      </Actions>
      <ErrorText error={describe(t, answer.error)} />
    </Block>
  );
}

function CreateHome() {
  const t = useT();
  const done = useDone();
  const create = useMutation({ mutationFn: (name: string) => homes.createHome({ name }), onSuccess: done });
  return (
    <Group title={t("site.home.create")} footer={t("site.home.createBody")}>
      <Block>
        <form
          className="flex flex-col gap-3 sm:flex-row sm:items-end"
          onSubmit={(e) => {
            e.preventDefault();
            create.mutate(String(new FormData(e.currentTarget).get("name")).trim());
          }}
        >
          <Field label={t("site.field.name")} name="name" defaultValue={t("site.home.defaultName")} required maxLength={64} />
          <Button type="submit" className="min-h-11" disabled={create.isPending}>
            {t("site.action.create")}
          </Button>
        </form>
        <ErrorText error={describe(t, create.error)} />
      </Block>
    </Group>
  );
}

function Members({ home, me }: { home: Home; me: string }) {
  const t = useT();
  const confirm = useConfirm();
  const admin = home.administrator?.id === me;
  const done = useDone();
  const router = useRouter();
  const leave = useMutation({
    mutationFn: async () => {
      if (admin) await homes.dissolveHome({});
      else await homes.leaveHome({});
    },
    onSuccess: done,
  });
  return (
    <>
      {home.frozen && (
        <p className="flex items-start gap-2.5 rounded-[14px] bg-fill px-4 py-3 type-callout text-text-secondary">
          <Icon name="info" size={20} className="mt-px shrink-0 text-warning" />
          {t(admin ? "site.home.frozenMine" : "site.home.frozenTheirs")}
        </p>
      )}
      <Group title={t("site.home.members")}>
        {home.members.map((m) => (
          <Member key={m.account?.id} member={m} me={me} admin={admin} administrator={m.account?.id === home.administrator?.id} onSwitched={() => router.navigate({ to: "/" })} />
        ))}
      </Group>
      {admin && <CreateManaged />}
      {admin && <Invite home={home} />}
      <Group footer={t(admin ? "site.home.dissolveBody" : "site.home.leaveBody")}>
        <Block>
          <Actions>
            <Button
              variant="danger"
              onClick={async () => {
                if (await confirm({ title: t(admin ? "site.home.dissolveConfirm" : "site.home.leaveConfirm", { home: home.name }), action: t(admin ? "site.home.dissolve" : "site.home.leave"), destructive: true }))
                  leave.mutate();
              }}
            >
              {t(admin ? "site.home.dissolve" : "site.home.leave")}
            </Button>
          </Actions>
          <ErrorText error={describe(t, leave.error)} />
        </Block>
      </Group>
    </>
  );
}

/** What opens under a member's row: switching with a PIN or password, editing a managed account, making it a full one. */
type Mode = "pin" | "password" | "edit" | "convert";

function Member({ member: m, me, admin, administrator, onSwitched }: { member: HomeMember; me: string; admin: boolean; administrator: boolean; onSwitched: () => void }) {
  const t = useT();
  const confirm = useConfirm();
  const account = m.account!;
  const isManaged = account.kind === AccountKind.MANAGED;
  const done = useDone();
  const [mode, setMode] = useState<Mode>();
  const [error, setError] = useState<string>();
  const remove = useMutation({ mutationFn: () => homes.removeMember({ accountId: account.id }), onSuccess: done });

  async function switchWith(proof: { pin?: string; password?: string }) {
    try {
      await switchTo({ accountId: account.id, ...proof });
      await done();
      onSwitched();
    } catch (e) {
      const text = describe(t, e) ?? "";
      setError(text);
      if (/password/.test(text)) setMode("password");
      else if (/PIN/.test(text)) setMode("pin");
    }
  }

  async function removeMember() {
    const yes = await confirm(
      isManaged
        ? { title: t("site.home.removeManagedConfirm", { name: nameOf(account) }), message: t("site.home.removeManagedBody"), action: t("site.action.remove"), destructive: true }
        : { title: t("site.home.removeMemberConfirm", { name: nameOf(account) }), message: t("site.home.removeMemberBody"), action: t("site.action.remove"), destructive: true },
    );
    if (yes) remove.mutate();
  }

  const restriction = restrictions.find(([r]) => r === account.restriction)?.[1];
  const tags = [
    `@${account.userName}`,
    account.id === me && t("site.home.you"),
    administrator && t("site.home.administrator"),
    isManaged && t("site.home.managed"),
    isManaged && restriction && t(restriction),
    m.hasPin && t("site.field.pin"),
  ].filter(Boolean);
  const toggle = (next: Mode) => setMode(mode === next ? undefined : next);
  let below: ReactNode;
  if (mode === "pin" || mode === "password") {
    below = (
      <form
        className="flex flex-col gap-3 sm:flex-row sm:items-end"
        onSubmit={(e) => {
          e.preventDefault();
          const v = String(new FormData(e.currentTarget).get("secret"));
          void switchWith(mode === "pin" ? { pin: v } : { password: v });
        }}
      >
        {mode === "pin" ? (
          <Field label={t("site.home.switchPin", { name: nameOf(account) })} name="secret" type="password" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} autoFocus required />
        ) : (
          <Field label={t("site.home.switchPassword", { name: nameOf(account) })} name="secret" type="password" autoFocus required />
        )}
        <Button type="submit" className="min-h-11">
          {t("site.home.switch")}
        </Button>
      </form>
    );
  } else if (mode === "edit") below = <EditManaged member={m} onConvert={() => setMode("convert")} onDone={() => setMode(undefined)} />;
  else if (mode === "convert") below = <Convert accountId={account.id} />;
  const failed = error ?? describe(t, remove.error);
  return (
    <div>
      <Row leading={<Avatar name={nameOf(account) || "?"} size={36} />} label={nameOf(account)} detail={tags.join(" · ")}>
        {account.id !== me && (
          <Button variant="secondary" onClick={() => (m.hasPin ? toggle("pin") : void switchWith({}))}>
            {t("site.home.switch")}
          </Button>
        )}
        {admin && isManaged && (
          <Button variant="secondary" onClick={() => toggle("edit")}>
            {t("site.action.edit")}
          </Button>
        )}
        {admin && account.id !== me && (
          <Button variant="danger" onClick={() => void removeMember()}>
            {t("site.action.remove")}
          </Button>
        )}
      </Row>
      {(below || failed) && (
        <div className="flex flex-col gap-3 px-4 pb-4">
          {below}
          <ErrorText error={failed} />
        </div>
      )}
    </div>
  );
}

function RestrictionSelect({ value }: { value?: RestrictionProfile }) {
  const t = useT();
  return <Select label={t("site.field.restriction")} name="restriction" defaultValue={value ?? RestrictionProfile.OLDER_CHILD} options={restrictions.map(([r, key]) => [r, t(key)])} />;
}

function EditManaged({ member: m, onConvert, onDone }: { member: HomeMember; onConvert: () => void; onDone: () => void }) {
  const t = useT();
  const account = m.account!;
  const done = useDone();
  const save = useMutation({
    mutationFn: async (form: FormData) => {
      await homes.updateManagedAccount({
        accountId: account.id,
        displayName: String(form.get("name")).trim(),
        restriction: Number(form.get("restriction")) as RestrictionProfile,
      });
      const pin = String(form.get("pin"));
      if (pin || form.get("clearPin")) await homes.setPin({ accountId: account.id, pin });
    },
    onSuccess: async () => {
      await done();
      onDone();
    },
  });
  return (
    <form
      className="flex flex-col gap-4 rounded-[12px] bg-fill p-4"
      onSubmit={(e) => {
        e.preventDefault();
        save.mutate(new FormData(e.currentTarget));
      }}
    >
      <div className="flex flex-col gap-4 sm:flex-row">
        <Field label={t("site.field.displayName")} name="name" defaultValue={account.displayName} required maxLength={64} />
        <RestrictionSelect value={account.restriction} />
      </div>
      <Field label={m.hasPin ? t("site.field.newPin") : t("site.field.pin")} name="pin" type="password" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} />
      {m.hasPin && <Check name="clearPin" label={t("site.home.removePin")} />}
      <ErrorText error={describe(t, save.error)} />
      <Actions>
        <Button type="submit" disabled={save.isPending}>
          {t("site.action.save")}
        </Button>
        <Button variant="secondary" onClick={onConvert}>
          {t("site.home.makeFull")}
        </Button>
      </Actions>
    </form>
  );
}

function Convert({ accountId }: { accountId: string }) {
  const t = useT();
  const done = useDone();
  const convert = useMutation({
    mutationFn: (form: FormData) => homes.convertManagedAccount({ accountId, email: String(form.get("email")).trim(), password: String(form.get("password")) }),
    onSuccess: done,
  });
  return (
    <form
      className="flex flex-col gap-4 rounded-[12px] bg-fill p-4"
      onSubmit={(e) => {
        e.preventDefault();
        convert.mutate(new FormData(e.currentTarget));
      }}
    >
      <Muted>{t("site.home.convertBody")}</Muted>
      <div className="flex flex-col gap-4 sm:flex-row">
        <Field label={t("site.field.email")} name="email" type="email" required />
        <Field label={t("site.field.password")} name="password" type="password" autoComplete="new-password" placeholder={t("site.field.passwordHint")} minLength={8} required />
      </div>
      <ErrorText error={describe(t, convert.error)} />
      <Actions>
        <Button type="submit" disabled={convert.isPending}>
          {t("site.home.makeFull")}
        </Button>
      </Actions>
    </form>
  );
}

function CreateManaged() {
  const t = useT();
  const done = useDone();
  const add = useMutation({
    mutationFn: (form: FormData) =>
      homes.createManagedAccount({
        userName: String(form.get("userName")).trim(),
        displayName: String(form.get("displayName")).trim(),
        restriction: Number(form.get("restriction")) as RestrictionProfile,
        pin: String(form.get("pin")) || undefined,
      }),
    onSuccess: done,
  });
  return (
    <Group title={t("site.home.addManaged")} footer={t("site.home.addManagedBody")}>
      <Block>
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            const form = e.currentTarget;
            add.mutate(new FormData(form), { onSuccess: () => form.reset() });
          }}
        >
          <div className="flex flex-col gap-4 sm:flex-row">
            <Field label={t("site.field.displayName")} name="displayName" required maxLength={64} />
            <Field label={t("site.field.userName")} name="userName" autoCapitalize="none" spellCheck={false} required minLength={3} maxLength={32} pattern="[A-Za-z0-9_.]+" />
          </div>
          <div className="flex flex-col gap-4 sm:flex-row">
            <RestrictionSelect />
            <Field label={t("site.field.pinOptional")} name="pin" type="password" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} />
          </div>
          <ErrorText error={describe(t, add.error)} />
          <Actions>
            <Button type="submit" disabled={add.isPending}>
              {t("site.action.add")}
            </Button>
          </Actions>
        </form>
      </Block>
    </Group>
  );
}

function Invite({ home }: { home: Home }) {
  const t = useT();
  const done = useDone();
  const servers = useQuery({ queryKey: ["servers"], queryFn: () => accounts.listServers({}) });
  const mine = servers.data?.servers.filter((s) => s.administrator?.id === home.administrator?.id) ?? [];
  const invite = useMutation({
    mutationFn: (form: FormData) =>
      homes.inviteToHome({
        userName: String(form.get("userName")).trim(),
        offers: form.getAll("server").map((id) => ({ serverId: String(id), access: create(AccessSchema, { allLibraries: true }) })),
      }),
    onSuccess: done,
  });
  const cancel = useMutation({ mutationFn: (accountId: string) => homes.cancelHomeInvitation({ accountId }), onSuccess: done });
  return (
    <Group title={t("site.home.invite")} footer={t("site.home.inviteBody")}>
      <Block>
        <form
          className="flex flex-col gap-4"
          onSubmit={(e) => {
            e.preventDefault();
            const form = e.currentTarget;
            invite.mutate(new FormData(form), { onSuccess: () => form.reset() });
          }}
        >
          <Field label={t("site.field.userName")} name="userName" autoCapitalize="none" spellCheck={false} required />
          {mine.length > 0 && (
            <div className="flex flex-col gap-1">
              {mine.map((s) => (
                <Check key={s.id} name="server" value={s.id} defaultChecked label={t("site.home.inviteServer", { server: s.name || s.id })} />
              ))}
            </div>
          )}
          <ErrorText error={describe(t, invite.error ?? cancel.error)} />
          <Actions>
            <Button type="submit" disabled={home.frozen || invite.isPending}>
              {t("site.action.invite")}
            </Button>
          </Actions>
        </form>
      </Block>
      {home.invitations.map((i) => (
        <Row key={i.account?.id} leading={<Avatar name={i.account?.userName || "?"} />} label={`@${i.account?.userName}`} detail={t("site.home.invited")}>
          <Button variant="secondary" onClick={() => cancel.mutate(i.account!.id)}>
            {t("site.action.withdraw")}
          </Button>
        </Row>
      ))}
    </Group>
  );
}

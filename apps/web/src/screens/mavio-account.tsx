// The Mavio account's matters inside the web UI (docs/clients.md §4.2,
// docs/accounts.md §3): its Mavio Pass, PIN and invitations to servers in
// Account, and the sections of its Home, its friends and the sharing of
// its servers, called with the device's session at mavio.dev. The
// password, passkeys and two-step verification stay on mavio.dev's pages.
import { create } from "@bufbuild/protobuf";
import { type Access, AccessSchema, AccountKind, type Library } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import type { Server } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { Friend_State, type Home, type HomeInvitation, type HomeMember, ServerAccessSchema } from "@mavio/proto/mavio/cloud/v1/home_pb";
import { RestrictionProfile } from "@mavio/proto/mavio/user/v1/user_pb";
import {
  errorMessage,
  serversByRole,
  useAccessRecords,
  useAccountAction,
  useFriends,
  useMavioAccount,
  useMavioHome,
  useMavioServers,
  useServerInvitations,
  useServersStore,
  useSignIn,
} from "@mavio/client-ts";
import { type FormEvent, type ReactNode, useState } from "react";

import { Confirm } from "../controls/dialog.tsx";
import { Checkbox } from "../controls/inputs.tsx";
import { Select } from "../controls/select.tsx";
import { type MessageKey } from "@mavio/design/strings";
import { locale, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Avatar, Button, Field, Group, Row } from "../ui.tsx";

const nameOf = (a?: { displayName: string; userName: string }) => a?.displayName || a?.userName || "";

const restrictions: [RestrictionProfile, MessageKey][] = [
  [RestrictionProfile.UNSPECIFIED, "site.restriction.none"],
  [RestrictionProfile.YOUNGER_CHILD, "site.restriction.youngerChild"],
  [RestrictionProfile.OLDER_CHILD, "site.restriction.olderChild"],
  [RestrictionProfile.TEEN, "site.restriction.teen"],
];

/** Whether a Mavio account is signed in, whose matters these sections are. */
export function useHasMavioAccount(): boolean {
  return useSignIn()?.kind === "mavioDev";
}

function Failed({ error }: { error: unknown }) {
  return error ? <p className="type-callout text-danger">{errorMessage(error)}</p> : null;
}

/** A group's content padded as its rows, such as a form. */
function Block({ children }: { children: ReactNode }) {
  return <div className="flex flex-col gap-3 p-4">{children}</div>;
}

/** A form whose fields are read on submit. */
function Form({ onSubmit, className, children }: { onSubmit: (f: FormData, form: HTMLFormElement) => void; className?: string; children: ReactNode }) {
  return (
    <form
      className={className ?? "flex flex-col gap-4"}
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        onSubmit(new FormData(e.currentTarget), e.currentTarget);
      }}
    >
      {children}
    </form>
  );
}

/** A destructive action asked about first. */
function useConfirm() {
  const [asking, setAsking] = useState<{ title: string; message?: string; action: string; run: () => Promise<unknown> }>();
  const [busy, setBusy] = useState(false);
  const dialog = asking && (
    <Confirm
      title={asking.title}
      message={asking.message}
      action={asking.action}
      busy={busy}
      onCancel={() => setAsking(undefined)}
      onConfirm={() => {
        setBusy(true);
        void asking.run().finally(() => (setBusy(false), setAsking(undefined)));
      }}
    />
  );
  return { ask: setAsking, dialog };
}

/** The Account section's groups of a Mavio account: Mavio Pass, the PIN and the invitations to servers. */
export function MavioAccountGroups() {
  const account = useMavioAccount();
  const site = useServersStore().mavioDevSite();
  const [pinOpen, setPinOpen] = useState(false);
  const a = account.data;
  if (!a) return <Failed error={account.error} />;
  const expires = a.passExpireTime && new Date(Number(a.passExpireTime.seconds) * 1000);
  return (
    <>
      <Group title={t("site.pass.title")}>
        <Row
          leading={<Icon name="key" size={22} className={expires || a.effectivePass ? "text-accent" : "text-text-tertiary"} />}
          label={expires ? t("site.pass.until", { date: expires.toLocaleDateString(locale, { dateStyle: "long" }) }) : a.effectivePass ? t("site.pass.shared") : t("site.pass.none")}
          wrap
        >
          {site && (
            <a href={`${site}/pass`} target="_blank" rel="noreferrer" className="inline-flex items-center gap-1.5 type-body text-accent-text">
              {t("site.nav.pass")}
              <Icon name="external" size={16} />
            </a>
          )}
        </Row>
      </Group>
      <Group>
        <Row label={t("site.field.pin")} detail={t(a.hasPin ? "site.account.pinSet" : "site.account.pinNone")} wrap>
          <Button variant="secondary" onClick={() => setPinOpen(!pinOpen)}>
            {pinOpen ? t("action.cancel") : t("site.action.set")}
          </Button>
        </Row>
        {pinOpen && a.account && <Pin accountId={a.account.id} has={a.hasPin} onDone={() => setPinOpen(false)} />}
      </Group>
      <ServerInvitations />
    </>
  );
}

function Pin({ accountId, has, onDone }: { accountId: string; has: boolean; onDone: () => void }) {
  const set = useAccountAction("setPin");
  return (
    <Block>
      <Form className="flex flex-wrap items-end gap-3" onSubmit={(f) => set.mutate({ accountId, pin: String(f.get("pin")) }, { onSuccess: onDone })}>
        <div className="w-40">
          <Field label={t(has ? "site.field.newPin" : "site.field.pin")} name="pin" type="password" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} autoFocus required />
        </div>
        <Button type="submit" disabled={set.isPending}>
          {t("site.action.set")}
        </Button>
        {has && (
          <Button variant="secondary" className="text-danger" onClick={() => set.mutate({ accountId, pin: "" }, { onSuccess: onDone })}>
            {t("site.action.remove")}
          </Button>
        )}
      </Form>
      <Failed error={set.error} />
    </Block>
  );
}

/** The invitations to servers the account has not answered. */
function ServerInvitations() {
  const invitations = useServerInvitations();
  const accept = useAccountAction("acceptInvitation");
  const decline = useAccountAction("declineInvitation");
  if (!invitations.data?.length) return null;
  return (
    <Group title={t("site.invitations.title")} footer={<Failed error={accept.error ?? decline.error} />}>
      {invitations.data.map((i) => (
        <Row key={i.serverId} leading={<Icon name="server" size={22} className="text-accent" />} label={t("site.invitations.server", { name: nameOf(i.administrator), server: i.serverName || i.serverId })} wrap>
          <Button disabled={accept.isPending} onClick={() => accept.mutate(i.serverId)}>
            {t("site.action.accept")}
          </Button>
          <Button variant="secondary" disabled={decline.isPending} onClick={() => decline.mutate(i.serverId)}>
            {t("site.action.decline")}
          </Button>
        </Row>
      ))}
    </Group>
  );
}

/** The account's friends: adding one, the requests received and sent, and removing one. */
export function FriendsGroups() {
  const friends = useFriends();
  const add = useAccountAction("addFriend");
  const accept = useAccountAction("acceptFriend");
  const remove = useAccountAction("removeFriend");
  const { ask, dialog } = useConfirm();
  const state = {
    [Friend_State.FRIEND]: t("site.friends.friend"),
    [Friend_State.RECEIVED]: t("site.friends.received"),
    [Friend_State.SENT]: t("site.friends.sent"),
    [Friend_State.UNSPECIFIED]: "",
  };
  return (
    <>
      <Group title={t("site.friends.add")} footer={t("site.friends.addHint")}>
        <Block>
          <Form className="flex flex-wrap items-end gap-3" onSubmit={(f, form) => add.mutate(String(f.get("name")).trim(), { onSuccess: () => form.reset() })}>
            <div className="min-w-56 flex-1">
              <Field label={t("site.friends.addField")} name="name" autoCapitalize="none" spellCheck={false} required />
            </div>
            <Button type="submit" disabled={add.isPending}>
              {t("site.friends.send")}
            </Button>
          </Form>
          <Failed error={add.error} />
        </Block>
      </Group>
      <Group title={t("site.nav.friends")} footer={<Failed error={friends.error ?? accept.error ?? remove.error} />}>
        {friends.data?.length === 0 && (
          <Block>
            <p className="type-callout text-text-secondary">{t("site.friends.empty")}</p>
          </Block>
        )}
        {friends.data?.map((f) => {
          const a = f.account!;
          return (
            <Row key={a.id} leading={<Avatar name={nameOf(a) || "?"} size={36} />} label={nameOf(a)} detail={`@${a.userName} · ${state[f.state]}`}>
              {f.state === Friend_State.RECEIVED && (
                <Button disabled={accept.isPending} onClick={() => accept.mutate(a.id)}>
                  {t("site.action.accept")}
                </Button>
              )}
              <Button
                variant="secondary"
                className={f.state === Friend_State.FRIEND ? "text-danger" : undefined}
                onClick={() =>
                  f.state === Friend_State.FRIEND
                    ? ask({ title: t("friends.removeConfirm", { name: nameOf(a) }), message: t("friends.removeBody"), action: t("site.action.remove"), run: () => remove.mutateAsync(a.id) })
                    : remove.mutate(a.id)
                }
              >
                {t(f.state === Friend_State.RECEIVED ? "site.action.decline" : f.state === Friend_State.SENT ? "site.action.withdraw" : "site.action.remove")}
              </Button>
            </Row>
          );
        })}
      </Group>
      {dialog}
    </>
  );
}

/** The account's Home: the invitations to Homes, creating one, its members, managed accounts and invitations, and leaving it. */
export function HomeGroups() {
  const home = useMavioHome();
  const me = useSignIn()?.account?.id ?? "";
  if (!home.data) return <Failed error={home.error} />;
  const { home: h, invitations } = home.data;
  return (
    <>
      {invitations.length > 0 && (
        <Group title={t("site.invitations.title")}>
          {invitations.map((i) => (
            <HomeInvitationRow key={i.homeId} invitation={i} />
          ))}
        </Group>
      )}
      {h ? <Members home={h} me={me} /> : <CreateHome />}
    </>
  );
}

function HomeInvitationRow({ invitation: i }: { invitation: HomeInvitation }) {
  const accept = useAccountAction("acceptHomeInvitation");
  const decline = useAccountAction("declineHomeInvitation");
  const servers = i.offers.map((o) => o.serverName || o.serverId).join(", ");
  return (
    <div>
      <Row
        leading={<Icon name="home" size={22} className="text-accent" />}
        label={t("site.home.invitation", { name: nameOf(i.administrator), home: i.homeName })}
        detail={[servers && t("site.home.invitationServers", { servers }), t("site.home.joinLeaves")].filter(Boolean).join(" ")}
        wrap
      >
        <Button disabled={accept.isPending} onClick={() => accept.mutate(i.homeId)}>
          {t("site.home.join")}
        </Button>
        <Button variant="secondary" disabled={decline.isPending} onClick={() => decline.mutate(i.homeId)}>
          {t("site.action.decline")}
        </Button>
      </Row>
      <div className="px-4 empty:hidden">
        <Failed error={accept.error ?? decline.error} />
      </div>
    </div>
  );
}

function CreateHome() {
  const createHome = useAccountAction("createHome");
  return (
    <Group title={t("site.home.create")} footer={t("site.home.createBody")}>
      <Block>
        <Form className="flex flex-wrap items-end gap-3" onSubmit={(f) => createHome.mutate(String(f.get("name")).trim())}>
          <div className="min-w-56 flex-1">
            <Field label={t("site.field.name")} name="name" defaultValue={t("site.home.defaultName")} required maxLength={64} />
          </div>
          <Button type="submit" disabled={createHome.isPending}>
            {t("site.action.create")}
          </Button>
        </Form>
        <Failed error={createHome.error} />
      </Block>
    </Group>
  );
}

function Members({ home, me }: { home: Home; me: string }) {
  const admin = home.administrator?.id === me;
  const dissolve = useAccountAction("dissolveHome");
  const leave = useAccountAction("leaveHome");
  const { ask, dialog } = useConfirm();
  return (
    <>
      {home.frozen && (
        <p className="flex items-start gap-2.5 rounded-[14px] bg-fill px-4 py-3 type-callout text-text-secondary">
          <Icon name="info" size={20} className="mt-px shrink-0" />
          {t(admin ? "site.home.frozenMine" : "site.home.frozenTheirs")}
        </p>
      )}
      <Group title={t("site.home.members")}>
        {home.members.map((m) => (
          <Member key={m.account?.id} member={m} me={me} admin={admin} administrator={m.account?.id === home.administrator?.id} />
        ))}
      </Group>
      {admin && <CreateManaged />}
      {admin && <Invite home={home} />}
      <Group footer={t(admin ? "site.home.dissolveBody" : "site.home.leaveBody")}>
        <Row label={t(admin ? "site.home.dissolve" : "site.home.leave")}>
          <Button
            variant="secondary"
            className="text-danger"
            onClick={() =>
              ask({
                title: t(admin ? "site.home.dissolveConfirm" : "site.home.leaveConfirm", { home: home.name }),
                message: t(admin ? "site.home.dissolveBody" : "site.home.leaveBody"),
                action: t(admin ? "site.home.dissolve" : "site.home.leave"),
                run: () => (admin ? dissolve.mutateAsync() : leave.mutateAsync()),
              })
            }
          >
            {t(admin ? "site.home.dissolve" : "site.home.leave")}
          </Button>
        </Row>
        <div className="px-4 empty:hidden">
          <Failed error={dissolve.error ?? leave.error} />
        </div>
      </Group>
      {dialog}
    </>
  );
}

function Member({ member: m, me, admin, administrator }: { member: HomeMember; me: string; admin: boolean; administrator: boolean }) {
  const account = m.account!;
  const managed = account.kind === AccountKind.MANAGED;
  const remove = useAccountAction("removeMember");
  const { ask, dialog } = useConfirm();
  const [mode, setMode] = useState<"edit" | "convert">();
  const restriction = restrictions.find(([r]) => r === account.restriction)?.[1];
  const tags = [
    `@${account.userName}`,
    account.id === me && t("site.home.you"),
    administrator && t("site.home.administrator"),
    managed && t("site.home.managed"),
    managed && restriction && t(restriction),
    m.hasPin && t("site.field.pin"),
  ].filter(Boolean);
  return (
    <div>
      <Row leading={<Avatar name={nameOf(account) || "?"} size={36} />} label={nameOf(account)} detail={tags.join(" · ")} wrap>
        {admin && managed && (
          <Button variant="secondary" onClick={() => setMode(mode ? undefined : "edit")}>
            {mode ? t("action.cancel") : t("site.action.edit")}
          </Button>
        )}
        {admin && account.id !== me && (
          <Button
            variant="secondary"
            className="text-danger"
            onClick={() =>
              ask(
                managed
                  ? { title: t("site.home.removeManagedConfirm", { name: nameOf(account) }), message: t("site.home.removeManagedBody"), action: t("site.action.remove"), run: () => remove.mutateAsync(account.id) }
                  : { title: t("site.home.removeMemberConfirm", { name: nameOf(account) }), message: t("site.home.removeMemberBody"), action: t("site.action.remove"), run: () => remove.mutateAsync(account.id) },
              )
            }
          >
            {t("site.action.remove")}
          </Button>
        )}
      </Row>
      {mode === "edit" && <EditManaged member={m} onConvert={() => setMode("convert")} onDone={() => setMode(undefined)} />}
      {mode === "convert" && <Convert accountId={account.id} onDone={() => setMode(undefined)} />}
      <div className="px-4 empty:hidden">
        <Failed error={remove.error} />
      </div>
      {dialog}
    </div>
  );
}

function RestrictionSelect({ value, onChange }: { value: RestrictionProfile; onChange: (r: RestrictionProfile) => void }) {
  return (
    <label className="flex flex-col gap-1.5">
      <span className="type-callout text-text-secondary">{t("site.field.restriction")}</span>
      <Select
        label={t("site.field.restriction")}
        value={String(value)}
        options={restrictions.map(([r, key]) => ({ value: String(r), label: t(key) }))}
        onChange={(v) => onChange(Number(v) as RestrictionProfile)}
        className="min-h-11"
      />
    </label>
  );
}

function EditManaged({ member: m, onConvert, onDone }: { member: HomeMember; onConvert: () => void; onDone: () => void }) {
  const account = m.account!;
  const save = useAccountAction("updateManagedAccount");
  const [restriction, setRestriction] = useState(account.restriction);
  const [clearPin, setClearPin] = useState(false);
  return (
    <div className="px-4 pb-4">
      <Form
        className="flex flex-col gap-4 rounded-[12px] bg-fill p-4"
        onSubmit={(f) => {
          const pin = String(f.get("pin"));
          save.mutate({ accountId: account.id, displayName: String(f.get("name")).trim(), restriction, pin: pin || (clearPin ? "" : undefined) }, { onSuccess: onDone });
        }}
      >
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("site.field.displayName")} name="name" defaultValue={account.displayName} required maxLength={64} />
          <RestrictionSelect value={restriction} onChange={setRestriction} />
        </div>
        <Field label={m.hasPin ? t("site.field.newPin") : t("site.field.pin")} name="pin" type="password" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} />
        {m.hasPin && (
          <Checkbox checked={clearPin} onChange={setClearPin}>
            {t("site.home.removePin")}
          </Checkbox>
        )}
        <Failed error={save.error} />
        <div className="flex flex-wrap gap-2">
          <Button type="submit" disabled={save.isPending}>
            {t("action.save")}
          </Button>
          <Button variant="secondary" onClick={onConvert}>
            {t("site.home.makeFull")}
          </Button>
        </div>
      </Form>
    </div>
  );
}

function Convert({ accountId, onDone }: { accountId: string; onDone: () => void }) {
  const convert = useAccountAction("convertManagedAccount");
  return (
    <div className="px-4 pb-4">
      <Form
        className="flex flex-col gap-4 rounded-[12px] bg-fill p-4"
        onSubmit={(f) => convert.mutate({ accountId, email: String(f.get("email")).trim(), password: String(f.get("password")) }, { onSuccess: onDone })}
      >
        <p className="type-callout text-text-secondary">{t("site.home.convertBody")}</p>
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("site.field.email")} name="email" type="email" required />
          <Field label={t("site.field.password")} name="password" type="password" autoComplete="new-password" placeholder={t("site.field.passwordHint")} minLength={8} required />
        </div>
        <Failed error={convert.error} />
        <div className="flex gap-2">
          <Button type="submit" disabled={convert.isPending}>
            {t("site.home.makeFull")}
          </Button>
        </div>
      </Form>
    </div>
  );
}

function CreateManaged() {
  const add = useAccountAction("createManagedAccount");
  const [restriction, setRestriction] = useState(RestrictionProfile.OLDER_CHILD);
  return (
    <Group title={t("site.home.addManaged")} footer={t("site.home.addManagedBody")}>
      <Block>
        <Form
          onSubmit={(f, form) =>
            add.mutate(
              { userName: String(f.get("userName")).trim(), displayName: String(f.get("displayName")).trim(), restriction, pin: String(f.get("pin")) || undefined },
              { onSuccess: () => form.reset() },
            )
          }
        >
          <div className="grid gap-4 sm:grid-cols-2">
            <Field label={t("site.field.displayName")} name="displayName" required maxLength={64} />
            <Field label={t("site.field.userName")} name="userName" autoCapitalize="none" spellCheck={false} required minLength={3} maxLength={32} pattern="[A-Za-z0-9_.]+" />
            <RestrictionSelect value={restriction} onChange={setRestriction} />
            <Field label={t("site.field.pinOptional")} name="pin" type="password" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} />
          </div>
          <Failed error={add.error} />
          <div className="flex gap-2">
            <Button type="submit" disabled={add.isPending}>
              {t("site.action.add")}
            </Button>
          </div>
        </Form>
      </Block>
    </Group>
  );
}

function Invite({ home }: { home: Home }) {
  const servers = useMavioServers();
  const invite = useAccountAction("inviteToHome");
  const cancel = useAccountAction("cancelHomeInvitation");
  const mine = servers.data?.filter((s) => s.administrator?.id === home.administrator?.id) ?? [];
  const [offered, setOffered] = useState<Set<string>>();
  const chosen = offered ?? new Set(mine.map((s) => s.id));
  return (
    <Group title={t("site.home.invite")} footer={t("site.home.inviteBody")}>
      <Block>
        <Form
          onSubmit={(f, form) =>
            invite.mutate(
              { userName: String(f.get("userName")).trim(), offers: [...chosen].map((serverId) => create(ServerAccessSchema, { serverId, access: create(AccessSchema, { allLibraries: true }) })) },
              { onSuccess: () => (form.reset(), setOffered(undefined)) },
            )
          }
        >
          <Field label={t("site.field.userName")} name="userName" autoCapitalize="none" spellCheck={false} required />
          {mine.map((s) => (
            <Checkbox
              key={s.id}
              checked={chosen.has(s.id)}
              onChange={(on) => {
                const next = new Set(chosen);
                if (on) next.add(s.id);
                else next.delete(s.id);
                setOffered(next);
              }}
            >
              {t("site.home.inviteServer", { server: s.name || s.id })}
            </Checkbox>
          ))}
          <Failed error={invite.error ?? cancel.error} />
          <div className="flex gap-2">
            <Button type="submit" disabled={home.frozen || invite.isPending}>
              {t("site.action.invite")}
            </Button>
          </div>
        </Form>
      </Block>
      {home.invitations.map((i) => (
        <Row key={i.account?.id} leading={<Avatar name={i.account?.userName || "?"} />} label={`@${i.account?.userName}`} detail={t("site.home.invited")}>
          <Button variant="secondary" disabled={cancel.isPending} onClick={() => cancel.mutate(i.account!.id)}>
            {t("site.action.withdraw")}
          </Button>
        </Row>
      ))}
    </Group>
  );
}

/** Who reaches the servers the account administers, and the servers shared with it, which it may leave. */
export function SharingGroups() {
  const servers = useMavioServers();
  const me = useSignIn()?.account?.id ?? "";
  if (!servers.data) return <Failed error={servers.error} />;
  const { administered, shared } = serversByRole(servers.data, me);
  if (administered.length === 0 && shared.length === 0) {
    return <p className="type-callout text-text-secondary">{t("sharing.nothing")}</p>;
  }
  return (
    <>
      {administered.map((s) => (
        <AccessRecords key={s.id} server={s} />
      ))}
      {shared.length > 0 && <SharedWithYou servers={shared} />}
    </>
  );
}

function accessLine(a: Access | undefined, libraries: Library[]): string {
  if (!a) return "";
  const parts = [
    a.allLibraries ? t("site.server.everyLibrary") : libraries.filter((l) => a.libraryIds.includes(l.id)).map((l) => l.name).join(", "),
    a.allowDownload && t("sharing.downloads"),
  ];
  return parts.filter(Boolean).join(" · ");
}

function AccessRecords({ server }: { server: Server }) {
  const records = useAccessRecords(server.id);
  const revoke = useAccountAction("revokeAccess");
  const { ask, dialog } = useConfirm();
  const [editing, setEditing] = useState<{ userName: string; access?: Access }>();
  const name = server.name || server.id;
  return (
    <Group title={name} footer={<Failed error={records.error ?? revoke.error} />}>
      {records.data?.length === 0 && (
        <Block>
          <p className="type-callout text-text-secondary">{t("site.server.noAccess")}</p>
        </Block>
      )}
      {records.data?.map((r) => {
        const a = r.account!;
        const open = editing?.userName === a.userName;
        return (
          <div key={a.id}>
            <Row
              leading={<Avatar name={nameOf(a) || "?"} size={36} />}
              label={nameOf(a)}
              detail={[`@${a.userName}`, r.invitation && t("site.server.invited"), accessLine(r.access, server.libraries)].filter(Boolean).join(" · ")}
              wrap
            >
              <Button variant="secondary" onClick={() => setEditing(open ? undefined : { userName: a.userName, access: r.access })}>
                {open ? t("action.cancel") : t("site.action.edit")}
              </Button>
              <Button
                variant="secondary"
                className="text-danger"
                onClick={() =>
                  r.invitation
                    ? revoke.mutate({ serverId: server.id, accountId: a.id })
                    : ask({
                        title: t("sharing.revokeConfirm", { name: nameOf(a), server: name }),
                        message: t("sharing.revokeBody"),
                        action: t("site.server.revoke"),
                        run: () => revoke.mutateAsync({ serverId: server.id, accountId: a.id }),
                      })
                }
              >
                {t(r.invitation ? "site.action.withdraw" : "site.server.revoke")}
              </Button>
            </Row>
            {open && <Grant server={server} initial={editing} onDone={() => setEditing(undefined)} />}
          </div>
        );
      })}
      {editing?.userName === "" ? (
        <Grant server={server} initial={editing} onDone={() => setEditing(undefined)} />
      ) : (
        <Row label={t("sharing.grant")} leading={<Icon name="add" size={20} className="text-accent" />}>
          <Button variant="secondary" onClick={() => setEditing({ userName: "" })}>
            {t("site.action.invite")}
          </Button>
        </Row>
      )}
      {dialog}
    </Group>
  );
}

/** The form giving an account access to a server's libraries, or changing it. */
function Grant({ server, initial, onDone }: { server: Server; initial: { userName: string; access?: Access }; onDone: () => void }) {
  const grant = useAccountAction("grantAccess");
  const a = initial.access;
  const editing = initial.userName !== "";
  const [all, setAll] = useState(a ? a.allLibraries : true);
  const [libraries, setLibraries] = useState(new Set(a?.libraryIds ?? []));
  const [blockUnrated, setBlockUnrated] = useState(a?.blockUnrated ?? false);
  const [allowDownload, setAllowDownload] = useState(a?.allowDownload ?? false);
  const labels = (f: FormData, name: string) =>
    String(f.get(name))
      .split(",")
      .map((l) => l.trim())
      .filter(Boolean);
  return (
    <div className="px-4 pb-4">
      <Form
        className="flex flex-col gap-4 rounded-[12px] bg-fill p-4"
        onSubmit={(f) => {
          const rating = String(f.get("rating"));
          grant.mutate(
            {
              serverId: server.id,
              userName: String(f.get("userName")).trim(),
              access: create(AccessSchema, {
                allLibraries: all,
                libraryIds: all ? [] : [...libraries],
                maxParentalRating: rating === "" ? undefined : Number(rating),
                blockUnrated,
                allowedLabels: labels(f, "allowedLabels"),
                blockedLabels: labels(f, "blockedLabels"),
                allowDownload,
              }),
            },
            { onSuccess: onDone },
          );
        }}
      >
        <Field label={t("site.field.userName")} name="userName" defaultValue={initial.userName} readOnly={editing} autoCapitalize="none" spellCheck={false} required autoFocus={!editing} />
        <div className="flex flex-col gap-2">
          <Checkbox checked={all} onChange={setAll}>
            {t("site.server.everyLibrary")}
          </Checkbox>
          {!all &&
            server.libraries.map((l) => (
              <Checkbox
                key={l.id}
                className="ps-7"
                checked={libraries.has(l.id)}
                onChange={(on) => {
                  const next = new Set(libraries);
                  if (on) next.add(l.id);
                  else next.delete(l.id);
                  setLibraries(next);
                }}
              >
                {l.name}
              </Checkbox>
            ))}
        </div>
        <Field label={t("site.server.rating")} name="rating" type="number" min={0} placeholder={t("site.server.ratingHint")} defaultValue={a?.maxParentalRating} />
        <div className="grid gap-4 sm:grid-cols-2">
          <Field label={t("site.server.allowedLabels")} name="allowedLabels" placeholder={t("site.server.labelsHint")} defaultValue={a?.allowedLabels.join(", ")} />
          <Field label={t("site.server.blockedLabels")} name="blockedLabels" placeholder={t("site.server.labelsHint")} defaultValue={a?.blockedLabels.join(", ")} />
        </div>
        <Checkbox checked={blockUnrated} onChange={setBlockUnrated}>
          {t("site.server.blockUnrated")}
        </Checkbox>
        <Checkbox checked={allowDownload} onChange={setAllowDownload}>
          {t("site.server.allowDownload")}
        </Checkbox>
        <Failed error={grant.error} />
        <div className="flex gap-2">
          <Button type="submit" disabled={grant.isPending}>
            {editing ? t("action.save") : t("site.action.invite")}
          </Button>
          <Button variant="secondary" onClick={onDone}>
            {t("action.cancel")}
          </Button>
        </div>
      </Form>
    </div>
  );
}

/** The servers others share with the account, each of which it may leave. */
function SharedWithYou({ servers }: { servers: Server[] }) {
  const leave = useAccountAction("leaveServer");
  const { ask, dialog } = useConfirm();
  return (
    <Group title={t("sharing.sharedWithYou")} footer={<Failed error={leave.error} />}>
      {servers.map((s) => {
        const name = s.name || s.id;
        return (
          <Row
            key={s.id}
            leading={<Icon name="server" size={22} className="text-text-secondary" />}
            label={name}
            detail={[t("site.servers.administeredBy", { name: nameOf(s.administrator) }), accessLine(s.access, s.libraries)].filter(Boolean).join(" · ")}
            wrap
          >
            <Button
              variant="secondary"
              className="text-danger"
              onClick={() => ask({ title: t("site.server.leaveConfirm", { name }), message: t("site.server.leaveBody"), action: t("site.action.leave"), run: () => leave.mutateAsync(s.id) })}
            >
              {t("site.server.leave")}
            </Button>
          </Row>
        );
      })}
      {dialog}
    </Group>
  );
}

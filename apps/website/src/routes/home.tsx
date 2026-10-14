import { create } from "@bufbuild/protobuf";
import { AccessSchema, AccountKind } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import type { Home, HomeInvitation, HomeMember } from "@mavio/proto/mavio/cloud/v1/home_pb";
import { RestrictionProfile } from "@mavio/proto/mavio/user/v1/user_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { createFileRoute, useRouter } from "@tanstack/react-router";
import { type FormEvent, useState } from "react";

import { accounts, homes, message, switchTo } from "../lib/api.ts";
import { useAccount } from "../lib/session.ts";
import { Button, Card, ErrorText, Field, Muted, Page } from "../ui.tsx";

export const Route = createFileRoute("/home")({ component: HomePage });

const restrictions: [RestrictionProfile, string][] = [
  [RestrictionProfile.UNSPECIFIED, "No restriction"],
  [RestrictionProfile.YOUNGER_CHILD, "Younger child"],
  [RestrictionProfile.OLDER_CHILD, "Older child"],
  [RestrictionProfile.TEEN, "Teenager"],
];

const nameOf = (m?: { displayName: string; userName: string }) => m?.displayName || m?.userName || "";

function HomePage() {
  const me = useAccount();
  const data = useQuery({ queryKey: ["home"], queryFn: () => homes.getHome({}), enabled: !!me });
  if (!me || !data.data) return <ErrorText error={data.error ? message(data.error) : undefined} />;
  const { home, invitations } = data.data;
  return (
    <Page title={home?.name || "Home"}>
      {invitations.map((i) => (
        <Invitation key={i.homeId} invitation={i} />
      ))}
      {home ? <Members home={home} me={me.id} /> : <CreateHome />}
    </Page>
  );
}

function useDone() {
  const qc = useQueryClient();
  return () => qc.invalidateQueries();
}

function Invitation({ invitation: i }: { invitation: HomeInvitation }) {
  const done = useDone();
  const answer = useMutation({
    mutationFn: async (accept: boolean) => {
      if (accept) await homes.acceptHomeInvitation({ homeId: i.homeId });
      else await homes.declineHomeInvitation({ homeId: i.homeId });
    },
    onSuccess: done,
  });
  return (
    <Card title={`Invitation to ${i.homeName}`}>
      <Muted>
        {nameOf(i.administrator)} invites you to their Home
        {i.offers.length > 0 && `, with access to ${i.offers.map((o) => o.serverName || o.serverId).join(", ")}`}. Joining leaves
        the Home you are in.
      </Muted>
      <ErrorText error={answer.error ? message(answer.error) : undefined} />
      <div className="flex gap-3">
        <Button onClick={() => answer.mutate(true)}>Join</Button>
        <Button kind="secondary" onClick={() => answer.mutate(false)}>
          Decline
        </Button>
      </div>
    </Card>
  );
}

function CreateHome() {
  const done = useDone();
  const create = useMutation({ mutationFn: (name: string) => homes.createHome({ name }), onSuccess: done });
  function submit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    create.mutate(String(new FormData(e.currentTarget).get("name")));
  }
  return (
    <Card title="Create a Home">
      <Muted>
        A Home is up to 15 people who share devices: they switch between each other, and children get managed accounts with
        restriction profiles.
      </Muted>
      <form className="flex flex-col gap-3" onSubmit={submit}>
        <Field label="Name" name="name" defaultValue="Home" required maxLength={64} />
        <ErrorText error={create.error ? message(create.error) : undefined} />
        <Button type="submit">Create</Button>
      </form>
    </Card>
  );
}

function Members({ home, me }: { home: Home; me: string }) {
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
        <Card>
          <Muted>
            This Home is frozen: {admin ? "your" : "its administrator's"} Mavio Pass lapsed, so no one can be invited and the Pass is
            not shared. Switching and managed accounts keep working.
          </Muted>
        </Card>
      )}
      <Card title="Members">
        <ul className="flex flex-col gap-4">
          {home.members.map((m) => (
            <Member key={m.account?.id} member={m} me={me} admin={admin} onSwitched={() => router.navigate({ to: "/" })} />
          ))}
        </ul>
      </Card>
      {admin && <CreateManaged />}
      {admin && <Invite home={home} />}
      <Card title={admin ? "Dissolve the Home" : "Leave the Home"}>
        <Muted>
          {admin
            ? "Full members become your friends and keep their access. Managed accounts are deleted, with their data on every server."
            : "You become the administrator's friend and keep your access."}
        </Muted>
        <ErrorText error={leave.error ? message(leave.error) : undefined} />
        <Button kind="danger" onClick={() => confirm(admin ? "Dissolve the Home?" : "Leave the Home?") && leave.mutate()}>
          {admin ? "Dissolve" : "Leave"}
        </Button>
      </Card>
    </>
  );
}

function Member({ member: m, me, admin, onSwitched }: { member: HomeMember; me: string; admin: boolean; onSwitched: () => void }) {
  const account = m.account!;
  const isManaged = account.kind === AccountKind.MANAGED;
  const done = useDone();
  const [mode, setMode] = useState<"pin" | "password" | "edit" | "convert">();
  const [error, setError] = useState<string>();
  const remove = useMutation({ mutationFn: () => homes.removeMember({ accountId: account.id }), onSuccess: done });

  async function switchWith(proof: { pin?: string; password?: string }) {
    try {
      await switchTo({ accountId: account.id, ...proof });
      await done();
      onSwitched();
    } catch (e) {
      const text = message(e);
      setError(text);
      if (/password/.test(text)) setMode("password");
      else if (/PIN/.test(text)) setMode("pin");
    }
  }

  function removeMember() {
    const lost = m.servers.map((s) => s.name || s.id);
    const what = isManaged
      ? `Delete ${nameOf(account)}? The account is deleted with its watch history and playlists${lost.length ? ` on ${lost.join(", ")}` : ""}.`
      : `Remove ${nameOf(account)} from the Home? They become your friend and keep their access.`;
    if (confirm(what)) remove.mutate();
  }

  return (
    <li className="flex flex-col gap-2">
      <div className="flex items-center gap-3">
        <div className="flex flex-1 flex-col">
          <span className="type-body">{nameOf(account)}</span>
          <span className="type-footnote text-text-secondary">
            @{account.userName}
            {isManaged && ` · managed · ${restrictions.find(([r]) => r === account.restriction)?.[1]}`}
            {m.hasPin && " · PIN"}
          </span>
        </div>
        {account.id !== me && (
          <Button kind="secondary" onClick={() => (m.hasPin ? setMode("pin") : switchWith({}))}>
            Switch
          </Button>
        )}
        {admin && isManaged && (
          <Button kind="secondary" onClick={() => setMode(mode === "edit" ? undefined : "edit")}>
            Edit
          </Button>
        )}
        {admin && account.id !== me && (
          <Button kind="danger" onClick={removeMember}>
            Remove
          </Button>
        )}
      </div>
      {(mode === "pin" || mode === "password") && (
        <form
          className="flex items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            const v = String(new FormData(e.currentTarget).get("secret"));
            void switchWith(mode === "pin" ? { pin: v } : { password: v });
          }}
        >
          {mode === "pin" ? (
            <Field label="PIN" name="secret" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} autoFocus required />
          ) : (
            <Field label={`${nameOf(account)}'s password`} name="secret" type="password" autoFocus required />
          )}
          <Button type="submit">Switch</Button>
        </form>
      )}
      {mode === "edit" && <EditManaged member={m} onConvert={() => setMode("convert")} />}
      {mode === "convert" && <Convert accountId={account.id} />}
      <ErrorText error={error ?? (remove.error ? message(remove.error) : undefined)} />
    </li>
  );
}

function RestrictionSelect({ value }: { value?: RestrictionProfile }) {
  return (
    <label className="flex flex-col gap-1">
      <span className="type-footnote text-text-secondary">Restriction profile</span>
      <select name="restriction" defaultValue={value ?? RestrictionProfile.OLDER_CHILD} className="type-body rounded-md border border-border bg-background px-3 py-2">
        {restrictions.map(([r, label]) => (
          <option key={r} value={r}>
            {label}
          </option>
        ))}
      </select>
    </label>
  );
}

function EditManaged({ member: m, onConvert }: { member: HomeMember; onConvert: () => void }) {
  const account = m.account!;
  const done = useDone();
  const save = useMutation({
    mutationFn: async (form: FormData) => {
      await homes.updateManagedAccount({
        accountId: account.id,
        displayName: String(form.get("name")),
        restriction: Number(form.get("restriction")) as RestrictionProfile,
      });
      const pin = String(form.get("pin"));
      if (pin || form.get("clearPin")) await homes.setPin({ accountId: account.id, pin });
    },
    onSuccess: done,
  });
  return (
    <form
      className="flex flex-col gap-3 rounded-md border border-border p-4"
      onSubmit={(e) => {
        e.preventDefault();
        save.mutate(new FormData(e.currentTarget));
      }}
    >
      <Field label="Display name" name="name" defaultValue={account.displayName} required maxLength={64} />
      <RestrictionSelect value={account.restriction} />
      <Field label={m.hasPin ? "New PIN" : "PIN"} name="pin" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} />
      {m.hasPin && (
        <label className="type-callout flex items-center gap-2">
          <input type="checkbox" name="clearPin" /> Remove the PIN
        </label>
      )}
      <ErrorText error={save.error ? message(save.error) : undefined} />
      <div className="flex gap-3">
        <Button type="submit">Save</Button>
        <Button kind="secondary" onClick={onConvert}>
          Make a full account
        </Button>
      </div>
    </form>
  );
}

function Convert({ accountId }: { accountId: string }) {
  const done = useDone();
  const convert = useMutation({
    mutationFn: (form: FormData) =>
      homes.convertManagedAccount({ accountId, email: String(form.get("email")), password: String(form.get("password")) }),
    onSuccess: done,
  });
  return (
    <form
      className="flex flex-col gap-3 rounded-md border border-border p-4"
      onSubmit={(e) => {
        e.preventDefault();
        convert.mutate(new FormData(e.currentTarget));
      }}
    >
      <Muted>
        The account signs in with its own e-mail address and password from now on, and keeps its data, access and place in the
        Home; its restriction profile is removed.
      </Muted>
      <Field label="E-mail address" name="email" type="email" required />
      <Field label="Password" name="password" type="password" minLength={8} required />
      <ErrorText error={convert.error ? message(convert.error) : undefined} />
      <Button type="submit">Make a full account</Button>
    </form>
  );
}

function CreateManaged() {
  const done = useDone();
  const add = useMutation({
    mutationFn: (form: FormData) =>
      homes.createManagedAccount({
        userName: String(form.get("userName")),
        displayName: String(form.get("displayName")),
        restriction: Number(form.get("restriction")) as RestrictionProfile,
        pin: String(form.get("pin")) || undefined,
      }),
    onSuccess: done,
  });
  return (
    <Card title="Add a managed account">
      <Muted>A managed account has no sign-in of its own; it is reached by switching in the Home, and sees only your servers.</Muted>
      <form
        className="flex flex-col gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          const form = e.currentTarget;
          add.mutate(new FormData(form), { onSuccess: () => form.reset() });
        }}
      >
        <Field label="Display name" name="displayName" required maxLength={64} />
        <Field label="User name" name="userName" required minLength={3} maxLength={32} pattern="[A-Za-z0-9_.]+" />
        <RestrictionSelect />
        <Field label="PIN (optional)" name="pin" inputMode="numeric" pattern="[0-9]{4}" maxLength={4} />
        <ErrorText error={add.error ? message(add.error) : undefined} />
        <Button type="submit">Add</Button>
      </form>
    </Card>
  );
}

function Invite({ home }: { home: Home }) {
  const done = useDone();
  const servers = useQuery({ queryKey: ["servers"], queryFn: () => accounts.listServers({}) });
  const mine = servers.data?.servers.filter((s) => s.administrator?.id === home.administrator?.id) ?? [];
  const invite = useMutation({
    mutationFn: (form: FormData) =>
      homes.inviteToHome({
        userName: String(form.get("userName")),
        offers: form.getAll("server").map((id) => ({ serverId: String(id), access: create(AccessSchema, { allLibraries: true }) })),
      }),
    onSuccess: done,
  });
  const cancel = useMutation({ mutationFn: (accountId: string) => homes.cancelHomeInvitation({ accountId }), onSuccess: done });
  return (
    <Card title="Invite someone">
      <Muted>Inviting a full account needs your Mavio Pass, which the Home then shares.</Muted>
      <form
        className="flex flex-col gap-3"
        onSubmit={(e) => {
          e.preventDefault();
          const form = e.currentTarget;
          invite.mutate(new FormData(form), { onSuccess: () => form.reset() });
        }}
      >
        <Field label="User name" name="userName" required />
        {mine.map((s) => (
          <label key={s.id} className="type-callout flex items-center gap-2">
            <input type="checkbox" name="server" value={s.id} defaultChecked /> Every library of {s.name || s.id}
          </label>
        ))}
        <ErrorText error={invite.error ? message(invite.error) : undefined} />
        <Button type="submit" disabled={home.frozen}>
          Invite
        </Button>
      </form>
      {home.invitations.length > 0 && (
        <ul className="flex flex-col gap-2">
          {home.invitations.map((i) => (
            <li key={i.account?.id} className="flex items-center gap-3">
              <span className="type-body flex-1">Invited @{i.account?.userName}</span>
              <Button kind="secondary" onClick={() => cancel.mutate(i.account!.id)}>
                Withdraw
              </Button>
            </li>
          ))}
        </ul>
      )}
    </Card>
  );
}

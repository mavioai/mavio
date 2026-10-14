// The profile's sections of one standalone server's sign-in: switching to
// another of its users (docs/accounts.md §4.4), and the signed-in user's
// passkeys, authenticator app and PIN.
import { timestampDate } from "@bufbuild/protobuf/wkt";
import { errorMessage, type ServerEntry, serverKey, useServersStore } from "@mavio/client-ts";
import type { SwitchableUser } from "@mavio/proto/mavio/auth/v1/auth_pb";
import { UserService } from "@mavio/proto/mavio/user/v1/user_pb";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { t } from "../i18n.ts";
import { Button, Field } from "../ui.tsx";
import { passkeysHere } from "./sign-in.tsx";

/** The users the server's session may switch to; nothing for sessions that may not. */
export function SwitchUser({ server }: { server: ServerEntry }) {
  const servers = useServersStore();
  const queryClient = useQueryClient();
  const users = useQuery({
    queryKey: serverKey(server.id, "switchable", server.userId),
    queryFn: () => servers.switchableUsers(server),
    retry: false,
  });
  const [asking, setAsking] = useState<SwitchableUser>();
  const [pin, setPin] = useState("");
  const [error, setError] = useState<string>();
  const switchTo = useMutation({
    mutationFn: ({ user, pin }: { user: SwitchableUser; pin?: string }) => servers.switchUser(server, user.id, pin),
    onSuccess: () => {
      setAsking(undefined);
      setPin("");
      setError(undefined);
      // Everything the server answered was the previous user's.
      queryClient.removeQueries({ queryKey: serverKey(server.id) });
    },
    onError: (err) => setError(errorMessage(err)),
  });
  if (!users.data || users.data.length < 2) return null;
  return (
    <div className="flex flex-col gap-3 rounded-lg bg-surface p-4">
      <h3 className="type-headline">{t("auth.switchUser")}</h3>
      <div className="flex flex-wrap gap-2">
        {users.data.map((u) => (
          <Button
            key={u.id}
            variant={u.current ? "primary" : "secondary"}
            disabled={u.current || switchTo.isPending}
            onClick={() => {
              setError(undefined);
              if (u.hasPin) setAsking(u);
              else switchTo.mutate({ user: u });
            }}
          >
            {u.name}
            {u.current && ` · ${t("auth.current")}`}
          </Button>
        ))}
      </div>
      {asking && (
        <form
          className="flex flex-wrap items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            switchTo.mutate({ user: asking, pin });
          }}
        >
          <div className="w-40">
            <Field
              label={t("auth.enterPin", { name: asking.name })}
              type="password"
              value={pin}
              onChange={(e) => setPin(e.target.value.replace(/\D/g, "").slice(0, 4))}
              inputMode="numeric"
              autoComplete="off"
              autoFocus
            />
          </div>
          <Button type="submit" disabled={pin.length !== 4 || switchTo.isPending}>
            {t("auth.switchUser")}
          </Button>
          <Button variant="plain" onClick={() => (setAsking(undefined), setPin(""))}>
            {t("action.cancel")}
          </Button>
        </form>
      )}
      {error && <p className="type-callout text-danger">{error}</p>}
    </div>
  );
}

/** The signed-in user's passkeys, authenticator app and PIN on a standalone server. */
export function Security({ server }: { server: ServerEntry }) {
  const servers = useServersStore();
  const queryClient = useQueryClient();
  const key = serverKey(server.id, "currentUser", server.userId);
  const me = useQuery({
    queryKey: key,
    queryFn: async () => (await servers.client(UserService, server).getCurrentUser({})).user,
  });
  const [error, setError] = useState<string>();
  const users = servers.client(UserService, server);
  const change = useMutation({
    mutationFn: (f: () => Promise<unknown>) => f(),
    onSuccess: () => (setError(undefined), queryClient.invalidateQueries({ queryKey: key })),
    onError: (err) => setError(errorMessage(err)),
  });
  const user = me.data;
  // Managed users have no credentials of their own.
  if (!user || user.managed) return null;

  const addPasskey = () =>
    change.mutate(async () => {
      const start = await users.startPasskeyRegistration({});
      const publicKey = PublicKeyCredential.parseCreationOptionsFromJSON(JSON.parse(start.optionsJson));
      const credential = (await navigator.credentials.create({ publicKey })) as PublicKeyCredential | null;
      if (!credential) return;
      await users.finishPasskeyRegistration({
        ceremonyId: start.ceremonyId,
        name: browserName(),
        credentialJson: JSON.stringify(credential.toJSON()),
      });
    });

  return (
    <div className="flex flex-col divide-y divide-border rounded-lg bg-surface">
      <div className="flex flex-col gap-2 p-4">
        <div className="flex items-center">
          <h3 className="type-headline">{t("auth.passkeys")}</h3>
          {passkeysHere(server.url) && (
            <Button variant="plain" className="ml-auto text-accent-text" disabled={change.isPending} onClick={addPasskey}>
              {t("auth.addPasskey")}
            </Button>
          )}
        </div>
        {user.passkeys.length === 0 && <p className="type-callout text-text-secondary">{t("auth.noPasskeys")}</p>}
        {user.passkeys.map((p) => (
          <div key={p.id} className="flex items-center gap-3">
            <div className="min-w-0 flex-1">
              <div className="type-body">{p.name}</div>
              <div className="type-footnote text-text-secondary">
                {t("auth.passkeyHost", { host: p.rpId })}
                {p.createTime && ` · ${timestampDate(p.createTime).toLocaleDateString()}`}
              </div>
            </div>
            <Button
              variant="plain"
              className="text-danger"
              disabled={change.isPending}
              onClick={() => change.mutate(() => users.deletePasskey({ userId: user.id, id: p.id }))}
            >
              {t("action.delete")}
            </Button>
          </div>
        ))}
        {!passkeysHere(server.url) && new URL(server.url).origin === location.origin && (
          <p className="type-footnote text-text-tertiary">{t("auth.passkeysUnavailable")}</p>
        )}
      </div>
      <Totp
        on={user.hasTotp}
        busy={change.isPending}
        start={() => change.mutateAsync(() => users.startTotp({})) as Promise<{ secret: string; uri: string }>}
        confirm={(code) => change.mutateAsync(() => users.confirmTotp({ code }))}
        turnOff={() => change.mutate(() => users.deleteTotp({ userId: user.id }))}
      />
      <Pin
        on={user.hasPin}
        busy={change.isPending}
        save={(pin) => change.mutateAsync(() => users.setPin({ userId: user.id, pin }))}
      />
      {error && <p className="p-4 type-callout text-danger">{error}</p>}
    </div>
  );
}

function Totp({
  on,
  busy,
  start,
  confirm,
  turnOff,
}: {
  on: boolean;
  busy: boolean;
  start: () => Promise<{ secret: string; uri: string }>;
  confirm: (code: string) => Promise<unknown>;
  turnOff: () => void;
}) {
  const [setup, setSetup] = useState<{ secret: string; uri: string }>();
  const [code, setCode] = useState("");
  return (
    <div className="flex flex-col gap-2 p-4">
      <div className="flex items-center gap-3">
        <div className="min-w-0 flex-1">
          <h3 className="type-headline">{t("auth.totp")}</h3>
          <div className="type-footnote text-text-secondary">{on ? t("auth.totpOn") : t("auth.totpOff")}</div>
        </div>
        {on ? (
          <Button variant="plain" className="text-danger" disabled={busy} onClick={turnOff}>
            {t("auth.turnOff")}
          </Button>
        ) : (
          !setup && (
            <Button variant="plain" className="text-accent-text" disabled={busy} onClick={() => void start().then(setSetup, () => {})}>
              {t("auth.setUpTotp")}
            </Button>
          )
        )}
      </div>
      {setup && !on && (
        <form
          className="flex flex-col gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            void confirm(code).then(
              () => (setSetup(undefined), setCode("")),
              () => {},
            );
          }}
        >
          <p className="type-callout break-all text-text-secondary">{t("auth.totpSecret", { secret: setup.secret })}</p>
          <a href={setup.uri} className="type-callout text-accent-text hover:underline">
            {t("auth.openAuthenticator")}
          </a>
          <div className="flex flex-wrap items-end gap-3">
            <div className="w-40">
              <Field
                label={t("auth.totpCode")}
                value={code}
                onChange={(e) => setCode(e.target.value.replace(/\D/g, "").slice(0, 6))}
                inputMode="numeric"
                autoComplete="one-time-code"
              />
            </div>
            <Button type="submit" disabled={busy || code.length !== 6}>
              {t("action.save")}
            </Button>
            <Button variant="plain" onClick={() => (setSetup(undefined), setCode(""))}>
              {t("action.cancel")}
            </Button>
          </div>
        </form>
      )}
    </div>
  );
}

function Pin({ on, busy, save }: { on: boolean; busy: boolean; save: (pin: string) => Promise<unknown> }) {
  const [editing, setEditing] = useState(false);
  const [pin, setPin] = useState("");
  return (
    <div className="flex flex-col gap-2 p-4">
      <div className="flex items-center gap-3">
        <div className="min-w-0 flex-1">
          <h3 className="type-headline">{t("auth.pin")}</h3>
          <div className="type-footnote text-text-secondary">
            {on ? t("auth.pinOn") : t("auth.totpOff")} · {t("auth.pinMessage")}
          </div>
        </div>
        {on && (
          <Button variant="plain" className="text-danger" disabled={busy} onClick={() => void save("").catch(() => {})}>
            {t("auth.removePin")}
          </Button>
        )}
        {!editing && (
          <Button variant="plain" className="text-accent-text" disabled={busy} onClick={() => setEditing(true)}>
            {t("auth.setPin")}
          </Button>
        )}
      </div>
      {editing && (
        <form
          className="flex flex-wrap items-end gap-3"
          onSubmit={(e) => {
            e.preventDefault();
            void save(pin).then(
              () => (setEditing(false), setPin("")),
              () => {},
            );
          }}
        >
          <div className="w-40">
            <Field
              label={t("auth.pin")}
              type="password"
              value={pin}
              onChange={(e) => setPin(e.target.value.replace(/\D/g, "").slice(0, 4))}
              inputMode="numeric"
              autoComplete="new-password"
              autoFocus
            />
          </div>
          <Button type="submit" disabled={busy || pin.length !== 4}>
            {t("action.save")}
          </Button>
          <Button variant="plain" onClick={() => (setEditing(false), setPin(""))}>
            {t("action.cancel")}
          </Button>
        </form>
      )}
    </div>
  );
}

/** What a new passkey is called: the browser it was made in. */
function browserName(): string {
  const ua = navigator.userAgent;
  if (/Firefox\//.test(ua)) return "Firefox";
  if (/Edg\//.test(ua)) return "Edge";
  if (/Chrome\//.test(ua)) return "Chrome";
  if (/Safari\//.test(ua)) return "Safari";
  return "Browser";
}

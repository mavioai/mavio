// The settings' Users (docs/clients.md §6.3.5): the users of the server
// chosen and its invitation links, and a user's page: its name, whether it
// is disabled, its restriction profile, the libraries and ratings it may
// see, how it may play and what it may request, and its sign-ins. In
// mavio.dev mode a user's name, access and state are mavio.dev's, and the
// server's own credentials and invitations are not used.
import { create, isFieldSet } from "@bufbuild/protobuf";
import { durationFromMs, timestampDate, type Timestamp } from "@bufbuild/protobuf/wkt";
import {
  errorMessage,
  invitationLink,
  type ServerEntry,
  useInvitationActions,
  useInvitations,
  useRatings,
  useServerLibraries,
  useUserActions,
  useUsers,
} from "@mavio/client-ts";
import { LibraryKind } from "@mavio/proto/mavio/library/v1/library_pb";
import {
  type Invitation,
  RequestQuotaSchema,
  RestrictionProfile,
  type User,
  type UserPolicy,
  UserPolicySchema,
} from "@mavio/proto/mavio/user/v1/user_pb";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Icon } from "../../icon.tsx";
import { Avatar, Button, Group, Row } from "../../ui.tsx";
import { Checkbox, NumberField, Switch } from "../../controls/inputs.tsx";
import { Select } from "../../controls/select.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { BackLink, Page } from "./common.tsx";
import { input } from "../../controls/styles.ts";
import { Confirm } from "../../controls/dialog.tsx";

const restrictions: { value: RestrictionProfile; title: () => string }[] = [
  { value: RestrictionProfile.UNSPECIFIED, title: () => t("users.restrictionNone") },
  { value: RestrictionProfile.YOUNGER_CHILD, title: () => t("users.restrictionYoungerChild") },
  { value: RestrictionProfile.OLDER_CHILD, title: () => t("users.restrictionOlderChild") },
  { value: RestrictionProfile.TEEN, title: () => t("users.restrictionTeen") },
];

const mbps = 1_000_000n;
const bitrates = [0n, 120n, 80n, 40n, 20n, 10n, 8n, 4n, 2n, 1n].map((m) => m * mbps);
const bitrateTitle = (b: bigint) => (b === 0n ? t("users.unlimited") : t("users.mbps", { mbps: Number(b / mbps) }));

const day = 86_400_000;
/** The rating score of adult content. */
const adult = 1000;
const dates = new Intl.DateTimeFormat(locale, { dateStyle: "medium" });
const dateOf = (ts?: Timestamp) => (ts ? dates.format(timestampDate(ts)) : undefined);

/** The libraries access is given to: not those of collections and playlists. */
const accessible = (kind: LibraryKind) => kind !== LibraryKind.COLLECTIONS && kind !== LibraryKind.PLAYLISTS;

/** The section of the server chosen in the settings. */
export function UsersSection({ server }: { server: ServerEntry }) {
  const search = useSearch({ strict: false }) as SettingsSearch;
  if (search.user === "new") return <NewUser server={server} />;
  if (search.user) return <UserPage key={search.user} server={server} id={search.user} />;
  return <UserList server={server} />;
}

function LinkButton({ search, icon, children }: { search: SettingsSearch; icon: "add"; children: string }) {
  return (
    <Link
      to="/settings/$section"
      params={{ section: "users" }}
      search={search}
      className="inline-flex min-h-11 items-center gap-2 rounded-md bg-button px-4 type-headline text-on-button hover:opacity-85"
    >
      <Icon name={icon} size={18} />
      {children}
    </Link>
  );
}

function UserList({ server }: { server: ServerEntry }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const users = useUsers(server);
  const standalone = server.mode !== "mavioDev";
  const list = [...(users.data ?? [])].sort((a, b) => Number(b.admin) - Number(a.admin) || a.name.localeCompare(b.name, locale));
  return (
    <Page
      title={t("settings.users")}
      message={standalone ? t("users.message") : t("users.messageMavioDev")}
      actions={standalone && <LinkButton search={{ server: cur.server, user: "new" }} icon="add">{t("users.add")}</LinkButton>}
    >
      {users.isError && <p className="type-callout text-danger">{errorMessage(users.error)}</p>}
      {list.length > 0 && (
        <Group>
          {list.map((u) => (
            <Link
              key={u.id}
              to="/settings/$section"
              params={{ section: "users" }}
              search={{ server: cur.server, user: u.id }}
              className="flex min-h-13 items-center gap-3 px-4 py-2.5 hover:tint"
            >
              <Avatar name={u.name} size={32} />
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2 truncate type-body">
                  {u.name}
                  <Badges user={u} />
                </div>
                <div className="truncate type-footnote text-text-secondary">
                  {u.lastLoginTime ? t("users.lastSignIn", { date: dateOf(u.lastLoginTime) ?? "" }) : t("users.neverSignedIn")}
                </div>
              </div>
              <Icon name="forward" size={16} className="text-text-tertiary" />
            </Link>
          ))}
        </Group>
      )}
      {standalone && <Invitations server={server} users={users.data ?? []} />}
    </Page>
  );
}

function Badges({ user: u }: { user: User }) {
  const badges = [u.admin && t("users.admin"), u.managed && t("users.managed"), u.disabled && t("users.disabled")].filter(
    (b): b is string => !!b,
  );
  return badges.map((b) => (
    <span key={b} className="rounded-sm bg-fill px-1.5 type-caption font-normal text-text-secondary">
      {b}
    </span>
  ));
}

/** Adds a member with a password, or a managed user with a PIN and a restriction profile. */
function NewUser({ server }: { server: ServerEntry }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const navigate = useNavigate();
  const actions = useUserActions(server);
  const [managed, setManaged] = useState(false);
  const [name, setName] = useState("");
  const [password, setPassword] = useState("");
  const [pin, setPin] = useState("");
  const [restriction, setRestriction] = useState(RestrictionProfile.UNSPECIFIED);
  const valid = name.trim() !== "" && (managed ? pin === "" || /^\d{4}$/.test(pin) : password !== "");
  const save = () =>
    actions.create.mutate(
      managed ? { name: name.trim(), managed, restriction, ...(pin ? { pin } : {}) } : { name: name.trim(), password },
      {
        onSuccess: (u) =>
          u && void navigate({ to: "/settings/$section", params: { section: "users" }, search: { server: cur.server, user: u.id } }),
      },
    );
  return (
    <Page back={<BackLink section="users" label={t("settings.users")} />} title={t("users.add")}>
      <form className="flex flex-col gap-7" onSubmit={(e) => (e.preventDefault(), valid && save())}>
        <Group footer={managed ? t("users.managedFooter") : t("users.memberFooter")}>
          <Row label={t("users.kind")}>
            <Select
              value={managed ? "managed" : "member"}
              label={t("users.kind")}
              options={[
                { value: "member", label: t("users.member") },
                { value: "managed", label: t("users.managed") },
              ]}
              onChange={(v) => setManaged(v === "managed")}
            />
          </Row>
          <Row label={t("users.name")}>
            <input className={`${input} sm:w-72`} aria-label={t("users.name")} value={name} onChange={(e) => setName(e.target.value)} required />
          </Row>
          {managed ? (
            <>
              <Row label={t("users.pin")} detail={t("users.pinDetail")}>
                <input
                  className={`${input} w-28`}
                  aria-label={t("users.pin")}
                  inputMode="numeric"
                  maxLength={4}
                  value={pin}
                  onChange={(e) => setPin(e.target.value.replace(/\D/g, ""))}
                />
              </Row>
              <Row label={t("users.restriction")} detail={t("users.restrictionDetail")}>
                <Select
                  value={String(restriction)}
                  label={t("users.restriction")}
                  options={restrictions.map((r) => ({ value: String(r.value), label: r.title() }))}
                  onChange={(v) => setRestriction(Number(v) as RestrictionProfile)}
                />
              </Row>
            </>
          ) : (
            <Row label={t("auth.password")}>
              <input
                type="password"
                autoComplete="new-password"
                className={`${input} sm:w-72`}
                aria-label={t("auth.password")}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
            </Row>
          )}
        </Group>
        <div className="flex flex-wrap items-center gap-3">
          <Button type="submit" disabled={!valid || actions.create.isPending}>
            {actions.create.isPending ? "…" : t("users.create")}
          </Button>
          {actions.create.isError && <span className="type-callout text-danger">{errorMessage(actions.create.error)}</span>}
        </div>
      </form>
    </Page>
  );
}

/** A user as edited: its name, state, restriction profile and policy. */
interface Draft {
  name: string;
  disabled: boolean;
  restriction: RestrictionProfile;
  allLibraries: boolean;
  libraryIds: string[];
  /** The highest rating score allowed; empty when unrestricted. */
  maxRating: string;
  blockUnrated: boolean;
  allowedLabels: string;
  blockedLabels: string;
  allowDownload: boolean;
  allowTranscoding: boolean;
  maxBitrate: bigint;
  maxSessions: number;
  requestMovies: boolean;
  requestSeries: boolean;
  autoApproveMovies: boolean;
  autoApproveSeries: boolean;
  reportIssues: boolean;
  movieQuota?: { limit: number; days: number };
  seriesQuota?: { limit: number; days: number };
}

const labelsOf = (s: string) => [...new Set(s.split(",").map((l) => l.trim()).filter(Boolean))];

function draftOf(u: User): Draft {
  const p = u.policy ?? create(UserPolicySchema);
  return {
    name: u.name,
    disabled: u.disabled,
    restriction: u.restriction,
    allLibraries: p.allLibraries,
    libraryIds: p.libraryIds,
    maxRating: isFieldSet(p, UserPolicySchema.field.maxParentalRating) ? String(p.maxParentalRating) : "",
    blockUnrated: p.blockUnrated,
    allowedLabels: p.allowedLabels.join(", "),
    blockedLabels: p.blockedLabels.join(", "),
    allowDownload: p.allowDownload,
    allowTranscoding: p.allowTranscoding,
    maxBitrate: p.maxStreamingBitrate,
    maxSessions: p.maxSessions,
    requestMovies: p.requestMovies,
    requestSeries: p.requestSeries,
    autoApproveMovies: p.autoApproveMovies,
    autoApproveSeries: p.autoApproveSeries,
    reportIssues: p.reportIssues,
    movieQuota: p.movieQuota && { limit: p.movieQuota.limit, days: p.movieQuota.days },
    seriesQuota: p.seriesQuota && { limit: p.seriesQuota.limit, days: p.seriesQuota.days },
  };
}

function policyOf(d: Draft): UserPolicy {
  return create(UserPolicySchema, {
    allLibraries: d.allLibraries,
    libraryIds: d.allLibraries ? [] : d.libraryIds,
    ...(d.maxRating !== "" ? { maxParentalRating: Number(d.maxRating) } : {}),
    blockUnrated: d.blockUnrated,
    allowedLabels: labelsOf(d.allowedLabels),
    blockedLabels: labelsOf(d.blockedLabels),
    allowDownload: d.allowDownload,
    allowTranscoding: d.allowTranscoding,
    maxStreamingBitrate: d.maxBitrate,
    maxSessions: d.maxSessions,
    requestMovies: d.requestMovies,
    requestSeries: d.requestSeries,
    autoApproveMovies: d.autoApproveMovies,
    autoApproveSeries: d.autoApproveSeries,
    reportIssues: d.reportIssues,
    movieQuota: d.movieQuota && create(RequestQuotaSchema, d.movieQuota),
    seriesQuota: d.seriesQuota && create(RequestQuotaSchema, d.seriesQuota),
  });
}

function UserPage({ server, id }: { server: ServerEntry; id: string }) {
  const users = useUsers(server);
  const user = users.data?.find((u) => u.id === id);
  if (!user) {
    return (
      <div className="flex flex-col gap-7">
        <BackLink section="users" label={t("settings.users")} />
        {users.isSuccess && <p className="type-body text-text-secondary">{t("users.notFound")}</p>}
        {users.isError && <p className="type-callout text-danger">{errorMessage(users.error)}</p>}
      </div>
    );
  }
  return <UserForm server={server} user={user} />;
}

function UserForm({ server, user }: { server: ServerEntry; user: User }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const navigate = useNavigate();
  const actions = useUserActions(server);
  const libraries = useServerLibraries(server);
  const ratings = useRatings(server);
  const standalone = server.mode !== "mavioDev";
  const [draft, setDraft] = useState(() => draftOf(user));
  const [confirming, setConfirming] = useState(false);
  const [notice, setNotice] = useState<string>();
  const set = (patch: Partial<Draft>) => (setDraft((d) => ({ ...d, ...patch })), setNotice(undefined));
  const changed = JSON.stringify(draft, bigints) !== JSON.stringify(draftOf(user), bigints);
  const valid = draft.name.trim() !== "";
  // The account's name, state and access are mavio.dev's in its mode.
  const own = standalone;
  const save = () =>
    actions.update.mutate(
      { id: user.id, name: draft.name.trim(), disabled: draft.disabled, restriction: draft.restriction, policy: policyOf(draft) },
      { onSuccess: () => setNotice(t("users.saved")) },
    );
  const toggle = (key: keyof Draft, label: string, detail?: string, disabled = false) => (
    <Row key={key} label={label} detail={detail}>
      {disabled ? (
        <span className="type-callout text-text-secondary">{draft[key] ? t("users.on") : t("users.off")}</span>
      ) : (
        <Switch on={draft[key] as boolean} label={label} onChange={(on) => set({ [key]: on })} />
      )}
    </Row>
  );
  const deletable = !user.admin && (standalone || user.disabled);
  // Ratings by the age they stand for, a few of their names beside it;
  // those above adult content are left to no limit.
  const scores = [...new Set((ratings.data ?? []).map((r) => r.score).filter((s) => s <= adult))];
  if (draft.maxRating !== "" && !scores.includes(Number(draft.maxRating))) scores.push(Number(draft.maxRating));
  const ratingOptions = [
    { value: "", label: t("users.noLimit") },
    ...scores
      .sort((a, b) => a - b)
      .map((s) => {
        const names = (ratings.data ?? []).filter((r) => r.score === s).map((r) => r.name);
        const age = s === 0 ? t("users.ratingAll") : s >= adult ? t("users.ratingAdult") : t("users.ratingAge", { age: s });
        return { value: String(s), label: names.length > 0 && names.length <= 3 ? `${age} (${names.join(", ")})` : age };
      }),
  ];
  const quota = (key: "movieQuota" | "seriesQuota", label: string) => {
    const q = draft[key];
    return (
      <Row label={label} detail={q ? undefined : t("users.quotaServer")}>
        {q && (
          <span className="flex items-center gap-2 type-callout text-text-secondary">
            <NumberField min={0} className="w-28" label={t("users.quotaLimit")} value={q.limit} onChange={(v) => set({ [key]: { ...q, limit: v ?? 0 } })} />
            {t("users.quotaEvery")}
            <NumberField min={0} className="w-28" label={t("users.quotaDays")} value={q.days} onChange={(v) => set({ [key]: { ...q, days: v ?? 0 } })} />
            {t("users.quotaDaysUnit")}
          </span>
        )}
        <Switch on={!!q} label={t("users.quotaCustom")} onChange={(on) => set({ [key]: on ? { limit: 10, days: 7 } : undefined })} />
      </Row>
    );
  };
  return (
    <Page
      back={<BackLink section="users" label={t("settings.users")} />}
      title={
        <span className="flex items-center gap-3">
          <Avatar name={user.name} size={40} />
          {user.name}
          <Badges user={user} />
        </span>
      }
      message={!standalone && t("users.mavioDevNote")}
      actions={
        deletable && (
          <Button variant="secondary" className="text-danger" onClick={() => setConfirming(true)}>
            {t("action.delete")}
          </Button>
        )
      }
    >
      {confirming && (
        <Confirm
          title={t("users.deleteTitle", { name: user.name })}
          message={t("users.deleteMessage")}
          action={t("action.delete")}
          busy={actions.remove.isPending}
          onCancel={() => setConfirming(false)}
          onConfirm={() =>
            actions.remove.mutate(user.id, {
              onSuccess: () => void navigate({ to: "/settings/$section", params: { section: "users" }, search: { server: cur.server } }),
              onSettled: () => setConfirming(false),
            })
          }
        />
      )}
      <Group>
        <Row label={t("users.name")}>
          {own ? (
            <input className={`${input} sm:w-72`} aria-label={t("users.name")} value={draft.name} onChange={(e) => set({ name: e.target.value })} />
          ) : (
            <span className="type-callout text-text-secondary">{user.name}</span>
          )}
        </Row>
        {!user.admin && toggle("disabled", t("users.disable"), t("users.disableDetail"), !own)}
        {user.managed && (
          <Row label={t("users.restriction")} detail={t("users.restrictionDetail")}>
            <Select
              value={String(draft.restriction)}
              label={t("users.restriction")}
              options={restrictions.map((r) => ({ value: String(r.value), label: r.title() }))}
              onChange={(v) => set({ restriction: Number(v) as RestrictionProfile })}
            />
          </Row>
        )}
      </Group>
      {!user.admin && (
        <Group title={t("users.access")} footer={own ? undefined : t("users.accessMavioDev")}>
          {toggle("allLibraries", t("users.allLibraries"), undefined, !own)}
          {!draft.allLibraries &&
            (libraries.data ?? [])
              .filter((l) => accessible(l.kind))
              .map((l) => (
                <Checkbox
                  key={l.id}
                  className="min-h-11 px-4 py-2"
                  disabled={!own}
                  checked={draft.libraryIds.includes(l.id)}
                  onChange={(on) => set({ libraryIds: on ? [...draft.libraryIds, l.id] : draft.libraryIds.filter((x) => x !== l.id) })}
                >
                  {l.name}
                </Checkbox>
              ))}
          <Row label={t("users.maxRating")} detail={t("users.maxRatingDetail")}>
            <Select
              value={draft.maxRating}
              label={t("users.maxRating")}
              className="w-48"
              options={ratingOptions}
              onChange={(maxRating) => own && set({ maxRating })}
            />
          </Row>
          {toggle("blockUnrated", t("users.blockUnrated"), undefined, !own)}
          <Row label={t("users.allowedLabels")} detail={t("users.allowedLabelsDetail")}>
            <input
              className={`${input} sm:w-72`}
              aria-label={t("users.allowedLabels")}
              disabled={!own}
              value={draft.allowedLabels}
              onChange={(e) => set({ allowedLabels: e.target.value })}
            />
          </Row>
          <Row label={t("users.blockedLabels")} detail={t("users.blockedLabelsDetail")}>
            <input
              className={`${input} sm:w-72`}
              aria-label={t("users.blockedLabels")}
              disabled={!own}
              value={draft.blockedLabels}
              onChange={(e) => set({ blockedLabels: e.target.value })}
            />
          </Row>
          {toggle("allowDownload", t("users.allowDownload"), undefined, !own)}
        </Group>
      )}
      <Group title={t("users.playback")}>
        {toggle("allowTranscoding", t("users.allowTranscoding"), t("users.allowTranscodingDetail"))}
        <Row label={t("users.maxBitrate")}>
          <Select
            value={String(draft.maxBitrate)}
            label={t("users.maxBitrate")}
            options={[
              ...bitrates.map((b) => ({ value: String(b), label: bitrateTitle(b) })),
              ...(bitrates.includes(draft.maxBitrate) ? [] : [{ value: String(draft.maxBitrate), label: bitrateTitle(draft.maxBitrate) }]),
            ]}
            onChange={(v) => set({ maxBitrate: BigInt(v) })}
          />
        </Row>
        <Row label={t("users.maxSessions")} detail={t("users.maxSessionsDetail")}>
          <NumberField min={0} max={100} className="w-32" label={t("users.maxSessions")} value={draft.maxSessions} onChange={(v) => set({ maxSessions: v ?? 0 })} />
        </Row>
      </Group>
      {!user.admin && (
        <Group title={t("users.requests")}>
          {toggle("requestMovies", t("users.requestMovies"))}
          {draft.requestMovies && toggle("autoApproveMovies", t("users.autoApproveMovies"))}
          {draft.requestMovies && quota("movieQuota", t("users.movieQuota"))}
          {toggle("requestSeries", t("users.requestSeries"))}
          {draft.requestSeries && toggle("autoApproveSeries", t("users.autoApproveSeries"))}
          {draft.requestSeries && quota("seriesQuota", t("users.seriesQuota"))}
          {toggle("reportIssues", t("users.reportIssues"))}
        </Group>
      )}
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={!changed || !valid || actions.update.isPending} onClick={save}>
          {actions.update.isPending ? "…" : t("action.save")}
        </Button>
        {notice && !changed && <span className="type-callout text-text-secondary">{notice}</span>}
        {actions.update.isError && <span className="type-callout text-danger">{errorMessage(actions.update.error)}</span>}
      </div>
      {standalone && <SignIns server={server} user={user} />}
    </Page>
  );
}

const bigints = (_: string, v: unknown) => (typeof v === "bigint" ? v.toString() : v);

/** A user's sign-ins: its password, PIN, authenticator and passkeys, and an invitation that converts a managed user. */
function SignIns({ server, user }: { server: ServerEntry; user: User }) {
  const actions = useUserActions(server);
  const invite = useInvitationActions(server);
  const [password, setPassword] = useState("");
  const [pin, setPin] = useState("");
  const [notice, setNotice] = useState<string>();
  const [link, setLink] = useState<string>();
  const failure = [actions.setPassword, actions.setPin, actions.deleteTotp, actions.deletePasskey, invite.create].find((x) => x.isError)?.error;
  const self = user.id === server.userId;
  return (
    <Group title={t("users.signIns")} footer={notice ?? (failure && <span className="text-danger">{errorMessage(failure)}</span>)}>
      {!user.authProvider && (
        <form
          className="flex min-h-13 flex-wrap items-center gap-3 px-4 py-2.5"
          onSubmit={(e) => {
            e.preventDefault();
            actions.setPassword.mutate(
              { id: user.id, password },
              { onSuccess: () => (setPassword(""), setNotice(user.managed ? t("users.converted") : t("users.passwordSet"))) },
            );
          }}
        >
          <div className="min-w-0 flex-1 basis-48">
            <div className="type-body">{user.managed ? t("users.givePassword") : t("users.setPassword")}</div>
            <div className="type-footnote text-text-secondary">
              {user.managed ? t("users.givePasswordDetail") : self ? t("users.setPasswordSelf") : t("users.setPasswordDetail")}
            </div>
          </div>
          <input
            type="password"
            autoComplete="new-password"
            className={`${input} w-56`}
            aria-label={t("auth.password")}
            placeholder={t("auth.password")}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
          />
          <Button type="submit" variant="secondary" disabled={!password || actions.setPassword.isPending}>
            {t("users.set")}
          </Button>
        </form>
      )}
      {user.managed && (
        <form
          className="flex min-h-13 flex-wrap items-center gap-3 px-4 py-2.5"
          onSubmit={(e) => {
            e.preventDefault();
            actions.setPin.mutate({ id: user.id, pin }, { onSuccess: () => (setPin(""), setNotice(pin ? t("users.pinSet") : t("users.pinCleared"))) });
          }}
        >
          <div className="min-w-0 flex-1 basis-48">
            <div className="type-body">{t("users.pin")}</div>
            <div className="type-footnote text-text-secondary">{user.hasPin ? t("users.pinOn") : t("users.pinOff")}</div>
          </div>
          <input
            className={`${input} w-28`}
            aria-label={t("users.pin")}
            inputMode="numeric"
            maxLength={4}
            placeholder="0000"
            value={pin}
            onChange={(e) => setPin(e.target.value.replace(/\D/g, ""))}
          />
          <Button type="submit" variant="secondary" disabled={pin.length !== 4 || actions.setPin.isPending}>
            {t("users.set")}
          </Button>
          {user.hasPin && (
            <Button variant="plain" disabled={actions.setPin.isPending} onClick={() => actions.setPin.mutate({ id: user.id, pin: "" }, { onSuccess: () => setNotice(t("users.pinCleared")) })}>
              {t("users.clear")}
            </Button>
          )}
        </form>
      )}
      {user.managed && (
        <Row label={t("users.inviteManaged")} detail={link ?? t("users.inviteManagedDetail")}>
          {link ? (
            <CopyButton text={link} />
          ) : (
            <Button
              variant="secondary"
              disabled={invite.create.isPending}
              onClick={() =>
                invite.create.mutate(
                  { userId: user.id, maxUses: 1, validFor: durationFromMs(7 * day) },
                  { onSuccess: (r) => setLink(invitationLink(server, r.token)) },
                )
              }
            >
              {t("users.createLink")}
            </Button>
          )}
        </Row>
      )}
      {user.hasTotp && (
        <Row label={t("users.totp")} detail={t("users.totpDetail")}>
          <Button variant="secondary" className="text-danger" disabled={actions.deleteTotp.isPending} onClick={() => actions.deleteTotp.mutate(user.id)}>
            {t("users.turnOff")}
          </Button>
        </Row>
      )}
      {user.passkeys.map((k) => (
        <Row
          key={k.id}
          label={k.name || t("users.passkey")}
          detail={[k.rpId, k.lastUseTime && t("users.lastUsed", { date: dateOf(k.lastUseTime) ?? "" })].filter(Boolean).join(" · ")}
        >
          <Button
            variant="secondary"
            className="text-danger"
            disabled={actions.deletePasskey.isPending}
            onClick={() => actions.deletePasskey.mutate({ id: user.id, passkeyId: k.id })}
          >
            {t("action.delete")}
          </Button>
        </Row>
      ))}
    </Group>
  );
}

function CopyButton({ text }: { text: string }) {
  const [copied, setCopied] = useState(false);
  return (
    <Button
      variant="secondary"
      onClick={() =>
        void navigator.clipboard.writeText(text).then(
          () => setCopied(true),
          () => {},
        )
      }
    >
      {copied ? t("users.copied") : t("users.copyLink")}
    </Button>
  );
}

/** The server's invitation links: making one, which shows its link once, and revoking them. */
function Invitations({ server, users }: { server: ServerEntry; users: User[] }) {
  const invitations = useInvitations(server);
  const actions = useInvitationActions(server);
  const libraries = useServerLibraries(server);
  const [making, setMaking] = useState(false);
  const [uses, setUses] = useState(1);
  const [days, setDays] = useState(7);
  const [all, setAll] = useState(true);
  const [ids, setIds] = useState<string[]>([]);
  const [download, setDownload] = useState(true);
  const [made, setMade] = useState<{ id: string; link: string }>();
  // Links used up can no longer be accepted.
  const open = (invitations.data ?? []).filter((i) => i.uses < i.maxUses);
  const list = open.filter((i) => !i.userId);
  const make = () =>
    actions.create.mutate(
      {
        maxUses: uses,
        validFor: durationFromMs(days * day),
        access: create(UserPolicySchema, { allLibraries: all, libraryIds: all ? [] : ids, allowDownload: download }),
      },
      { onSuccess: (r) => (setMade({ id: r.invitation?.id ?? "", link: invitationLink(server, r.token) }), setMaking(false)) },
    );
  // The link made is shown until its invitation is revoked or used up.
  const link = made && list.some((i) => i.id === made.id) ? made.link : undefined;
  const failure = [invitations, actions.create, actions.remove].find((x) => x.isError)?.error;
  return (
    <>
      <Group
        title={
          <span className="flex items-center justify-between gap-3">
            {t("users.invitations")}
            {!making && (
              <Button variant="plain" onClick={() => (setMaking(true), setMade(undefined))}>
                <Icon name="add" size={18} />
                {t("users.invite")}
              </Button>
            )}
          </span>
        }
        footer={failure ? <span className="text-danger">{errorMessage(failure)}</span> : t("users.invitationsFooter")}
      >
        {link && (
          <Row label={t("users.linkMade")} detail={<span className="break-all whitespace-normal">{link}</span>}>
            <CopyButton text={link} />
          </Row>
        )}
        {making && (
          <form className="flex flex-col divide-y divide-border bg-background/40" onSubmit={(e) => (e.preventDefault(), make())}>
            <Row label={t("users.maxUses")}>
              <NumberField min={1} max={100} className="w-32" label={t("users.maxUses")} value={uses} onChange={(v) => setUses(v ?? 1)} />
            </Row>
            <Row label={t("users.validDays")}>
              <NumberField min={1} max={30} className="w-32" label={t("users.validDays")} value={days} onChange={(v) => setDays(v ?? 1)} />
            </Row>
            <Row label={t("users.allLibraries")}>
              <Switch on={all} label={t("users.allLibraries")} onChange={setAll} />
            </Row>
            {!all &&
              (libraries.data ?? [])
                .filter((l) => accessible(l.kind))
                .map((l) => (
                  <Checkbox
                    key={l.id}
                    className="min-h-11 px-4 py-2"
                    checked={ids.includes(l.id)}
                    onChange={(on) => setIds(on ? [...ids, l.id] : ids.filter((x) => x !== l.id))}
                  >
                    {l.name}
                  </Checkbox>
                ))}
            <Row label={t("users.allowDownload")}>
              <Switch on={download} label={t("users.allowDownload")} onChange={setDownload} />
            </Row>
            <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
              <Button type="submit" disabled={actions.create.isPending}>
                {t("users.createLink")}
              </Button>
              <Button variant="plain" onClick={() => setMaking(false)}>
                {t("action.cancel")}
              </Button>
            </div>
          </form>
        )}
        {list.map((i) => (
          <InvitationRow key={i.id} invitation={i} onRevoke={() => actions.remove.mutate(i.id)} busy={actions.remove.isPending} />
        ))}
        {invitations.isSuccess && list.length === 0 && !making && !link && (
          <p className="px-4 py-3 type-callout text-text-secondary">{t("users.noInvitations")}</p>
        )}
      </Group>
      <ConversionLinks invitations={open.filter((i) => i.userId)} users={users} onRevoke={(id) => actions.remove.mutate(id)} />
    </>
  );
}

function InvitationRow({ invitation: i, label, onRevoke, busy }: { invitation: Invitation; label?: string; onRevoke: () => void; busy: boolean }) {
  return (
    <Row
      label={label ?? t("users.invitationUses", { uses: i.uses, max: i.maxUses })}
      detail={[t("users.expires", { date: dateOf(i.expireTime) ?? "" }), dateOf(i.createTime) && t("users.created", { date: dateOf(i.createTime) ?? "" })].join(" · ")}
    >
      <Button variant="secondary" className="text-danger" disabled={busy} onClick={onRevoke}>
        {t("users.revoke")}
      </Button>
    </Row>
  );
}

/** The links made for managed users, each converting one into a member. */
function ConversionLinks({ invitations, users, onRevoke }: { invitations: Invitation[]; users: User[]; onRevoke: (id: string) => void }) {
  if (invitations.length === 0) return null;
  return (
    <Group title={t("users.conversionLinks")}>
      {invitations.map((i) => (
        <InvitationRow
          key={i.id}
          invitation={i}
          label={users.find((u) => u.id === i.userId)?.name ?? i.userId}
          onRevoke={() => onRevoke(i.id)}
          busy={false}
        />
      ))}
    </Group>
  );
}

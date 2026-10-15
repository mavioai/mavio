// The settings' Storage (docs/clients.md §6.3.10): the rclone the server
// runs or reaches, its remotes — added with a kind's settings and the
// questions their configuration asks, such as authorizing a cloud drive —
// their folders, and the download clients of cloud drives' offline
// downloads.
import { create } from "@bufbuild/protobuf";
import {
  errorMessage,
  type ServerEntry,
  useOfflineClients,
  useRemoteActions,
  useRemoteFolders,
  useRemotes,
  useSetOfflineClients,
  useStorageProviders,
  useStorageStatus,
} from "@mavio/client-ts";
import {
  type ConfigQuestion,
  type OfflineClient,
  OfflineClientSchema,
  type Option,
  type Provider,
  type Remote,
} from "@mavio/proto/mavio/storage/v1/storage_pb";
import { Link, useNavigate, useSearch } from "@tanstack/react-router";
import { type ReactNode, useState } from "react";

import { locale, t } from "../../i18n.ts";
import { Icon } from "../../icon.tsx";
import { Button, Group, Row, Select, Toggle } from "../../ui.tsx";
import type { SettingsSearch } from "../settings.tsx";
import { BackLink, ColorTile, Confirm, input, Page } from "./common.tsx";

/** The section of the server chosen in the settings. */
export function StorageSection({ server }: { server: ServerEntry }) {
  const search = useSearch({ strict: false }) as SettingsSearch;
  if (search.remote === "new") return <NewRemote server={server} />;
  if (search.remote) return <RemotePage key={search.remote} server={server} name={search.remote} />;
  return <StorageList server={server} />;
}

function StorageList({ server }: { server: ServerEntry }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const status = useStorageStatus(server);
  const available = status.data?.available ?? false;
  const remotes = useRemotes(server, available);
  const providers = useStorageProviders(server, available);
  const s = status.data;
  return (
    <Page
      title={t("settings.storage")}
      message={t("storage.message")}
      actions={
        available && (
          <Link
            to="/settings/$section"
            params={{ section: "storage" }}
            search={{ server: cur.server, remote: "new" }}
            className="inline-flex min-h-11 items-center gap-2 rounded-md bg-button px-4 type-headline text-on-button hover:opacity-85"
          >
            <Icon name="add" size={18} />
            {t("storage.add")}
          </Link>
        )
      }
    >
      <Group title={t("storage.rclone")}>
        <Row label={t("storage.status")}>
          <span className="flex min-w-0 items-center gap-2 type-callout text-text-secondary">
            <span className={`size-2 shrink-0 rounded-full ${status.isPending ? "bg-fill" : available ? "bg-[#34C759]" : "bg-danger"}`} />
            <span className="min-w-0 break-words">
              {status.isPending ? "…" : status.isError ? errorMessage(status.error) : available ? t("storage.available") : s?.error || t("storage.unavailable")}
            </span>
          </span>
        </Row>
        {s?.version && (
          <Row label={t("storage.version")}>
            <span className="type-callout text-text-secondary">{s.version}</span>
          </Row>
        )}
        {s && (
          <Row label={t("storage.rcd")} detail={s.managed ? t("storage.managedDetail") : t("storage.externalDetail")}>
            <span className="type-callout text-text-secondary">{s.managed ? t("storage.managed") : t("storage.external")}</span>
          </Row>
        )}
      </Group>
      {available && (
        <Group title={t("storage.remotes")} footer={t("storage.remotesFooter")}>
          {(remotes.data ?? [])
            .toSorted((a, b) => a.name.localeCompare(b.name, locale))
            .map((r) => (
              <Link
                key={r.name}
                to="/settings/$section"
                params={{ section: "storage" }}
                search={{ server: cur.server, remote: r.name }}
                className="flex min-h-13 items-center gap-3 px-4 py-2.5 hover:bg-surface-raised"
              >
                <ColorTile icon="folder" color="#30B0C7" />
                <div className="min-w-0 flex-1">
                  <div className="truncate type-body">{r.name}</div>
                  <div className="truncate type-footnote text-text-secondary">{kindOf(providers.data, r.type)}</div>
                </div>
                <Icon name="forward" size={16} className="shrink-0 text-text-tertiary" />
              </Link>
            ))}
          {remotes.isSuccess && remotes.data.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("storage.noRemotes")}</p>}
          {remotes.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(remotes.error)}</p>}
        </Group>
      )}
      <OfflineClients server={server} remotes={available ? (remotes.data ?? []) : []} />
    </Page>
  );
}

/** A kind of remote's name, as its description says it. */
function kindOf(providers: Provider[] | undefined, type: string) {
  return providers?.find((p) => p.name === type)?.description || type;
}

/** Whether an rclone provider filter, "AWS,Ceph" or "!AWS,Ceph", takes a provider. */
export function providerMatches(filter: string, provider: string): boolean {
  if (!filter) return true;
  const negated = filter.startsWith("!");
  const list = (negated ? filter.slice(1) : filter).split(",").map((s) => s.trim());
  return list.includes(provider) !== negated;
}

/** The options of a kind shown for the values set: those its provider takes. */
function shownOptions(options: Option[], values: Record<string, string>, advanced: boolean) {
  const provider = values.provider ?? options.find((o) => o.name === "provider")?.defaultValue ?? "";
  return options.filter((o) => (advanced || !o.advanced || o.required) && providerMatches(o.provider, provider));
}

/** The first line of an option's help, and the rest. */
function helpOf(o: Option): [string, string] {
  const [first = "", ...rest] = o.help.trim().split("\n");
  return [first, rest.join("\n").trim()];
}

/** The control of an option's value. */
function OptionField({
  option: o,
  value,
  provider,
  placeholder,
  onChange,
}: {
  option: Option;
  value: string | undefined;
  provider: string;
  placeholder?: string;
  onChange: (v: string) => void;
}) {
  const [first, rest] = helpOf(o);
  const examples = o.examples.filter((e) => providerMatches(e.provider, provider));
  const current = value ?? o.defaultValue;
  let control: ReactNode;
  if (o.type === "bool") {
    control = <Toggle on={current === "true"} label={o.name} onChange={(on) => onChange(String(on))} />;
  } else if (examples.length && o.exclusive) {
    const options = examples.map((e) => ({ value: e.value, label: e.help ? `${e.value} — ${e.help.split("\n")[0]}` : e.value }));
    if (!options.some((x) => x.value === current)) options.unshift({ value: current, label: current || "—" });
    control = <Select value={current} label={o.name} className="w-full sm:w-72" options={options} onChange={onChange} />;
  } else {
    const list = examples.length ? `options-${o.name}` : undefined;
    control = (
      <>
        <input
          type={o.password ? "password" : "text"}
          autoComplete={o.password ? "new-password" : "off"}
          className={`${input} sm:w-72`}
          aria-label={o.name}
          list={list}
          placeholder={placeholder ?? o.defaultValue}
          value={value ?? ""}
          onChange={(e) => onChange(e.target.value)}
        />
        {list && (
          <datalist id={list}>
            {examples.map((e) => (
              <option key={e.value} value={e.value}>
                {e.help.split("\n")[0]}
              </option>
            ))}
          </datalist>
        )}
      </>
    );
  }
  return (
    <div className="flex min-h-13 flex-wrap items-center gap-3 px-4 py-2.5">
      <div className="min-w-0 flex-1 basis-60">
        <div className="type-body">
          <code>{o.name}</code>
          {o.required && <span className="text-danger"> *</span>}
        </div>
        {first && <div className="type-footnote text-text-secondary">{first}</div>}
        {rest && (
          <details className="type-footnote text-text-secondary">
            <summary className="cursor-pointer text-accent-text">{t("storage.more")}</summary>
            <p className="mt-1 whitespace-pre-wrap">{rest}</p>
          </details>
        )}
      </div>
      {control}
    </div>
  );
}

/** A remote's settings: its kind's options, with those rarely changed behind a switch. */
function OptionsGroup({
  options,
  values,
  placeholders,
  onChange,
}: {
  options: Option[];
  values: Record<string, string>;
  placeholders?: (o: Option) => string | undefined;
  onChange: (name: string, v: string) => void;
}) {
  const [advanced, setAdvanced] = useState(false);
  const shown = shownOptions(options, values, advanced);
  const provider = values.provider ?? options.find((o) => o.name === "provider")?.defaultValue ?? "";
  return (
    <Group title={t("storage.settings")} footer={t("storage.settingsFooter")}>
      {shown.map((o) => (
        <OptionField key={o.name} option={o} value={values[o.name]} provider={provider} placeholder={placeholders?.(o)} onChange={(v) => onChange(o.name, v)} />
      ))}
      {options.some((o) => o.advanced && !o.required) && (
        <Row label={t("storage.advanced")}>
          <Toggle on={advanced} label={t("storage.advanced")} onChange={setAdvanced} />
        </Row>
      )}
    </Group>
  );
}

/** The values to send: those set, differing from their defaults. */
function changedValues(options: Option[], values: Record<string, string>) {
  return Object.fromEntries(
    Object.entries(values).filter(([k, v]) => {
      const o = options.find((x) => x.name === k);
      return o ? v !== o.defaultValue && (v !== "" || o.defaultValue === "") && v.trim() !== "" : v !== "";
    }),
  );
}

function NewRemote({ server }: { server: ServerEntry }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const navigate = useNavigate();
  const providers = useStorageProviders(server);
  const remotes = useRemotes(server);
  const actions = useRemoteActions(server);
  const [name, setName] = useState("");
  const [type, setType] = useState("");
  const [values, setValues] = useState<Record<string, string>>({});
  const [question, setQuestion] = useState<ConfigQuestion>();
  const sorted = (providers.data ?? []).toSorted((a, b) => (a.description || a.name).localeCompare(b.description || b.name, locale));
  const provider = sorted.find((p) => p.name === type);
  const taken = remotes.data?.some((r) => r.name === name.trim());
  const valid = /^[A-Za-z0-9_][A-Za-z0-9_.+@-]*$/.test(name.trim()) && !taken;
  const missing = provider ? shownOptions(provider.options, values, false).some((o) => o.required && !(values[o.name] ?? o.defaultValue)) : true;
  const open = () => void navigate({ to: "/settings/$section", params: { section: "storage" }, search: { server: cur.server, remote: name.trim() } });
  const back = <BackLink section="storage" label={t("settings.storage")} />;
  if (question)
    return (
      <Page title={name.trim()} message={t("storage.questionMessage")} back={back}>
        <QuestionStep server={server} name={name.trim()} question={question} onDone={open} />
      </Page>
    );
  return (
    <Page title={t("storage.new")} back={back}>
      <Group>
        <Row label={t("storage.name")} detail={taken ? t("storage.nameTaken") : t("storage.nameDetail")}>
          <input className={`${input} sm:w-72`} aria-label={t("storage.name")} value={name} onChange={(e) => setName(e.target.value)} />
        </Row>
        <Row label={t("storage.kind")}>
          <Select
            value={type}
            label={t("storage.kind")}
            className="w-full sm:w-72"
            options={[{ value: "", label: t("storage.chooseKind") }, ...sorted.map((p) => ({ value: p.name, label: p.description ? `${p.description} (${p.name})` : p.name }))]}
            onChange={(v) => (setType(v), setValues({}))}
          />
        </Row>
        {providers.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(providers.error)}</p>}
      </Group>
      {provider && <OptionsGroup key={provider.name} options={provider.options} values={values} onChange={(k, v) => setValues((x) => ({ ...x, [k]: v }))} />}
      <div className="flex flex-wrap items-center gap-3">
        <Button
          disabled={!valid || !provider || missing || actions.create.isPending}
          onClick={() =>
            actions.create.mutate(
              { name: name.trim(), type, parameters: changedValues(provider!.options, values) },
              { onSuccess: (r) => (r.question ? setQuestion(r.question) : open()) },
            )
          }
        >
          {actions.create.isPending ? "…" : t("storage.create")}
        </Button>
        {actions.create.isError && <span className="type-callout text-danger">{errorMessage(actions.create.error)}</span>}
      </div>
    </Page>
  );
}

/**
 * A question of a remote's configuration — a choice, a value, or the token
 * of authorizing a cloud drive — answered until none is left.
 */
function QuestionStep({ server, name, question, onDone }: { server: ServerEntry; name: string; question: ConfigQuestion; onDone: () => void }) {
  const actions = useRemoteActions(server);
  const [q, setQ] = useState(question);
  const o = q.option;
  const [answer, setAnswer] = useState(o?.defaultValue ?? "");
  const next = (r: { question?: ConfigQuestion }) => {
    if (!r.question) return onDone();
    setQ(r.question);
    setAnswer(r.question.option?.defaultValue ?? "");
  };
  return (
    <>
      <Group footer={t("storage.questionFooter")}>
        {o ? (
          <OptionField key={q.state} option={o} value={answer} provider="" onChange={setAnswer} />
        ) : (
          <p className="px-4 py-3 type-callout text-text-secondary">{t("storage.questionNone")}</p>
        )}
        {q.error && <p className="px-4 py-3 type-callout text-danger">{q.error}</p>}
      </Group>
      <div className="flex flex-wrap items-center gap-3">
        <Button disabled={actions.answer.isPending} onClick={() => actions.answer.mutate({ name, state: q.state, answer }, { onSuccess: next })}>
          {actions.answer.isPending ? "…" : t("storage.continue")}
        </Button>
        <Button variant="plain" onClick={onDone}>
          {t("storage.later")}
        </Button>
        {actions.answer.isError && <span className="type-callout text-danger">{errorMessage(actions.answer.error)}</span>}
      </div>
    </>
  );
}

function RemotePage({ server, name }: { server: ServerEntry; name: string }) {
  const remotes = useRemotes(server);
  const providers = useStorageProviders(server);
  const remote = remotes.data?.find((r) => r.name === name);
  const provider = providers.data?.find((p) => p.name === remote?.type);
  const back = <BackLink section="storage" label={t("settings.storage")} />;
  if (remotes.isSuccess && !remote)
    return (
      <Page title={name} back={back}>
        <p className="type-callout text-text-secondary">{t("storage.noRemote")}</p>
      </Page>
    );
  return (
    <Page title={name} message={remote && kindOf(providers.data, remote.type)} back={back}>
      {(remotes.isError || providers.isError) && <p className="type-callout text-danger">{errorMessage(remotes.error ?? providers.error)}</p>}
      {remote && provider && <RemoteForm server={server} remote={remote} provider={provider} />}
      {remote && <Folders server={server} remote={remote.name} />}
      {remote && <RemoveRemote server={server} name={remote.name} />}
    </Page>
  );
}

function RemoteForm({ server, remote, provider }: { server: ServerEntry; remote: Remote; provider: Provider }) {
  const actions = useRemoteActions(server);
  const [values, setValues] = useState<Record<string, string>>(() => ({ ...remote.parameters }));
  const [question, setQuestion] = useState<ConfigQuestion>();
  const [notice, setNotice] = useState<string>();
  // Secrets are never listed: an empty one keeps what is set.
  const secret = (o: Option) => o.password || o.sensitive;
  const changes = Object.fromEntries(
    Object.entries(values).filter(([k, v]) => {
      const o = provider.options.find((x) => x.name === k);
      return o && secret(o) ? v !== "" : v !== (remote.parameters[k] ?? "");
    }),
  );
  const changed = Object.keys(changes).length > 0;
  if (question)
    return (
      <QuestionStep
        server={server}
        name={remote.name}
        question={question}
        onDone={() => (setQuestion(undefined), setNotice(t("storage.saved")), setValues({ ...remote.parameters }))}
      />
    );
  return (
    <>
      <OptionsGroup
        options={provider.options}
        values={values}
        placeholders={(o) => (secret(o) ? t("storage.secretKept") : undefined)}
        onChange={(k, v) => (setValues((x) => ({ ...x, [k]: v })), setNotice(undefined))}
      />
      <div className="flex flex-wrap items-center gap-3">
        <Button
          disabled={!changed || actions.update.isPending}
          onClick={() =>
            actions.update.mutate(
              { name: remote.name, parameters: changes },
              {
                onSuccess: (r) => {
                  if (r.question) return setQuestion(r.question);
                  setNotice(t("storage.saved"));
                  setValues({ ...(r.remote?.parameters ?? remote.parameters) });
                },
              },
            )
          }
        >
          {actions.update.isPending ? "…" : t("action.save")}
        </Button>
        {notice && !changed && <span className="type-callout text-text-secondary">{notice}</span>}
        {actions.update.isError && <span className="type-callout text-danger">{errorMessage(actions.update.error)}</span>}
      </div>
    </>
  );
}

/** A remote's folders, to walk and to see that it is reached. */
function Folders({ server, remote }: { server: ServerEntry; remote: string }) {
  const root = `rclone:${remote}:`;
  const [path, setPath] = useState(root);
  const folders = useRemoteFolders(server, path);
  const parent = path === root ? undefined : path.includes("/") ? path.slice(0, path.lastIndexOf("/")) : root;
  const row = (key: string, label: ReactNode, onClick: () => void, icon: "back" | "folder") => (
    <button key={key} type="button" onClick={onClick} className="flex min-h-11 w-full items-center gap-3 px-4 text-left hover:bg-surface-raised">
      <Icon name={icon} size={18} className="shrink-0 text-text-secondary" />
      <span className="min-w-0 flex-1 truncate type-body">{label}</span>
    </button>
  );
  return (
    <Group title={t("storage.folders")} footer={t("storage.foldersFooter")}>
      <p className="px-4 py-2.5 break-all type-callout text-text-secondary">{path}</p>
      <div className="max-h-80 divide-y divide-border overflow-y-auto">
        {parent !== undefined && row("..", t("libraries.parentFolder"), () => setPath(parent), "back")}
        {(folders.data ?? []).map((f) => row(f, f.slice(f.lastIndexOf(f.includes("/") ? "/" : ":") + 1), () => setPath(f), "folder"))}
        {folders.isPending && <p className="px-4 py-3 type-callout text-text-secondary">…</p>}
        {folders.isSuccess && folders.data.length === 0 && <p className="px-4 py-3 type-callout text-text-secondary">{t("libraries.noFolders")}</p>}
        {folders.isError && <p className="px-4 py-3 type-callout break-words text-danger">{errorMessage(folders.error)}</p>}
      </div>
    </Group>
  );
}

function RemoveRemote({ server, name }: { server: ServerEntry; name: string }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const navigate = useNavigate();
  const actions = useRemoteActions(server);
  const [confirming, setConfirming] = useState(false);
  if (confirming)
    return (
      <Confirm
        title={t("storage.removeConfirm", { name })}
        message={actions.remove.isError ? errorMessage(actions.remove.error) : t("storage.removeDetail")}
        action={t("storage.remove")}
        busy={actions.remove.isPending}
        onCancel={() => setConfirming(false)}
        onConfirm={() =>
          actions.remove.mutate(name, {
            onSuccess: () => void navigate({ to: "/settings/$section", params: { section: "storage" }, search: { server: cur.server } }),
          })
        }
      />
    );
  return (
    <div>
      <Button variant="secondary" className="text-danger" onClick={() => setConfirming(true)}>
        {t("storage.remove")}
      </Button>
    </div>
  );
}

/** A download client as edited; tags as typed. */
interface ClientDraft {
  id: string;
  name: string;
  enabled: boolean;
  priority: number;
  folder: string;
  tags: string;
  removeCompleted: boolean;
  removeFailed: boolean;
}

const draftOf = (c: OfflineClient): ClientDraft => ({
  id: c.id,
  name: c.name,
  enabled: c.enabled,
  priority: c.priority,
  folder: c.folder,
  tags: c.tags.join(", "),
  removeCompleted: c.removeCompletedDownloads,
  removeFailed: c.removeFailedDownloads,
});

const clientOf = (d: ClientDraft): OfflineClient =>
  create(OfflineClientSchema, {
    id: d.id.trim(),
    name: d.name.trim(),
    enabled: d.enabled,
    priority: d.priority,
    folder: d.folder.trim(),
    tags: d.tags
      .split(",")
      .map((s) => s.trim())
      .filter(Boolean),
    removeCompletedDownloads: d.removeCompleted,
    removeFailedDownloads: d.removeFailed,
  });

/** An id for a client named so, unique among the others. */
function idFor(name: string, taken: string[]) {
  const base =
    name
      .toLowerCase()
      .replace(/[^a-z0-9_-]+/g, "-")
      .replace(/^-+|-+$/g, "") || "offline";
  let id = base;
  for (let i = 2; taken.includes(id); i++) id = `${base}-${i}`;
  return id;
}

/** The download clients of offline downloads: cloud drives that download magnet links into a folder of their remote. */
function OfflineClients({ server, remotes }: { server: ServerEntry; remotes: Remote[] }) {
  const clients = useOfflineClients(server);
  const set = useSetOfflineClients(server);
  // The client edited, by its index; the list's length for a new one.
  const [editing, setEditing] = useState<number>();
  const list = clients.data ?? [];
  const save = (next: OfflineClient[]) => set.mutate(next, { onSuccess: () => setEditing(undefined) });
  return (
    <Group
      title={
        <span className="flex items-center justify-between gap-3">
          {t("storage.offline")}
          {editing === undefined && clients.isSuccess && (
            <Button variant="plain" onClick={() => (set.reset(), setEditing(list.length))}>
              {t("storage.addOffline")}
            </Button>
          )}
        </span>
      }
      footer={t("storage.offlineFooter")}
    >
      {list.map((c, i) =>
        editing === i ? (
          <ClientForm
            key={c.id}
            client={draftOf(c)}
            remotes={remotes}
            taken={list.filter((_, j) => j !== i).map((x) => x.id)}
            busy={set.isPending}
            error={set.isError ? errorMessage(set.error) : undefined}
            onCancel={() => setEditing(undefined)}
            onSave={(d) => save(list.map((x, j) => (j === i ? clientOf(d) : x)))}
            onRemove={() => save(list.filter((_, j) => j !== i))}
          />
        ) : (
          <button
            key={c.id}
            type="button"
            disabled={editing !== undefined}
            onClick={() => (set.reset(), setEditing(i))}
            className="flex min-h-13 w-full items-center gap-3 px-4 py-2.5 text-left enabled:hover:bg-surface-raised"
          >
            <ColorTile icon="downloads" color={c.enabled ? "#34C759" : "#8E8E93"} />
            <div className="min-w-0 flex-1">
              <div className="truncate type-body">{c.name}</div>
              <div className="truncate type-footnote text-text-secondary">
                {[c.folder, c.enabled ? undefined : t("storage.disabled"), c.tags.length ? c.tags.join(", ") : undefined].filter(Boolean).join(" · ")}
              </div>
            </div>
          </button>
        ),
      )}
      {editing === list.length && (
        <ClientForm
          client={{ id: "", name: "", enabled: true, priority: 1, folder: "", tags: "", removeCompleted: true, removeFailed: false }}
          remotes={remotes}
          taken={list.map((x) => x.id)}
          isNew
          busy={set.isPending}
          error={set.isError ? errorMessage(set.error) : undefined}
          onCancel={() => setEditing(undefined)}
          onSave={(d) => save([...list, clientOf(d)])}
        />
      )}
      {clients.isSuccess && list.length === 0 && editing === undefined && <p className="px-4 py-3 type-callout text-text-secondary">{t("storage.noOffline")}</p>}
      {clients.isError && <p className="px-4 py-3 type-callout text-danger">{errorMessage(clients.error)}</p>}
    </Group>
  );
}

function ClientForm({
  client,
  remotes,
  taken,
  isNew,
  busy,
  error,
  onCancel,
  onSave,
  onRemove,
}: {
  client: ClientDraft;
  remotes: Remote[];
  taken: string[];
  isNew?: boolean;
  busy: boolean;
  error?: string;
  onCancel: () => void;
  onSave: (d: ClientDraft) => void;
  onRemove?: () => void;
}) {
  const [d, setD] = useState(client);
  const [removing, setRemoving] = useState(false);
  const set = (patch: Partial<ClientDraft>) => setD((x) => ({ ...x, ...patch }));
  const id = isNew ? idFor(d.name, taken) : d.id;
  const valid = d.name.trim() !== "" && /^rclone:[^:/\\]+:/.test(d.folder.trim());
  const text = (key: "name" | "folder" | "tags", label: string, detail?: string, placeholder?: string) => (
    <Row label={label} detail={detail}>
      <input className={`${input} sm:w-72`} aria-label={label} placeholder={placeholder} value={d[key]} onChange={(e) => set({ [key]: e.target.value })} />
    </Row>
  );
  return (
    <div className="flex flex-col divide-y divide-border bg-background/40">
      {text("name", t("storage.offlineName"), undefined, "PikPak")}
      {text("folder", t("storage.offlineFolder"), t("storage.offlineFolderDetail"), remotes[0] ? `rclone:${remotes[0].name}:downloads` : "rclone:pikpak:downloads")}
      {text("tags", t("storage.offlineTags"), t("storage.offlineTagsDetail"))}
      <Row label={t("storage.offlinePriority")} detail={t("storage.offlinePriorityDetail")}>
        <input
          type="number"
          min={1}
          max={50}
          className={`${input} w-24`}
          aria-label={t("storage.offlinePriority")}
          value={d.priority || 1}
          onChange={(e) => set({ priority: Math.max(1, Math.min(50, Math.trunc(Number(e.target.value) || 1))) })}
        />
      </Row>
      <Row label={t("storage.offlineEnabled")}>
        <Toggle on={d.enabled} label={t("storage.offlineEnabled")} onChange={(enabled) => set({ enabled })} />
      </Row>
      <Row label={t("storage.removeCompleted")}>
        <Toggle on={d.removeCompleted} label={t("storage.removeCompleted")} onChange={(removeCompleted) => set({ removeCompleted })} />
      </Row>
      <Row label={t("storage.removeFailed")}>
        <Toggle on={d.removeFailed} label={t("storage.removeFailed")} onChange={(removeFailed) => set({ removeFailed })} />
      </Row>
      <div className="flex flex-wrap items-center gap-3 px-4 py-2.5">
        <Button disabled={!valid || busy} onClick={() => onSave({ ...d, id })}>
          {busy ? "…" : t("action.save")}
        </Button>
        <Button variant="plain" onClick={onCancel}>
          {t("action.cancel")}
        </Button>
        {onRemove &&
          (removing ? (
            <Button variant="secondary" className="text-danger" disabled={busy} onClick={onRemove}>
              {t("storage.removeOfflineConfirm")}
            </Button>
          ) : (
            <Button variant="plain" className="text-danger" onClick={() => setRemoving(true)}>
              {t("storage.removeOffline")}
            </Button>
          ))}
        {error && <span className="type-callout text-danger">{error}</span>}
      </div>
    </div>
  );
}

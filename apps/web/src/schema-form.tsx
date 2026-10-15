// A form drawn from a JSON Schema, as plugins describe their configuration
// (docs/plugins.md §4): objects as groups of rows, arrays of objects as
// cards to add and remove, lists of strings as lines, enumerations as
// choices, and what has no shape the form knows, such as an object of
// arbitrary keys, as JSON.
import { type ReactNode, useState } from "react";

import { t } from "./i18n.ts";
import { Icon } from "./icon.tsx";
import { Button, Toggle } from "./ui.tsx";

/** The subset of JSON Schema the form draws. */
export interface Schema {
  type?: string | string[];
  title?: string;
  description?: string;
  properties?: Record<string, Schema>;
  required?: string[];
  items?: Schema;
  enum?: unknown[];
  default?: unknown;
  format?: string;
  minimum?: number;
  maximum?: number;
  writeOnly?: boolean;
}

type Json = unknown;
type Obj = Record<string, Json>;

const isObj = (v: Json): v is Obj => typeof v === "object" && v !== null && !Array.isArray(v);

function typeOf(s: Schema): string | undefined {
  if (Array.isArray(s.type)) return s.type.find((x) => x !== "null");
  if (s.type) return s.type;
  if (s.enum) return "enum";
  if (s.properties) return "object";
  return undefined;
}

/** A key as words: api_key becomes "Api key". */
export function labelOf(key: string, s?: Schema): string {
  if (s?.title) return s.title;
  const words = key
    .replace(/[_-]+/g, " ")
    .replace(/([a-z])([A-Z])/g, "$1 $2")
    .toLowerCase()
    .replace(/\b(api|url|id|ip|ssdp|http|https|dns|tls|rss|nzb|tmdb|imdb|tvdb)\b/g, (w) => w.toUpperCase());
  return words.charAt(0).toUpperCase() + words.slice(1);
}

/** Whether a property holds a secret, to draw as a password. */
function secret(key: string, s: Schema): boolean {
  return s.writeOnly === true || s.format === "password" || /(^|_)(password|secret|token|api_key|key|passkey)$/i.test(key);
}

/** An empty object of a schema, with its required properties' defaults. */
function initial(s: Schema): Json {
  switch (typeOf(s)) {
    case "object": {
      const o: Obj = {};
      for (const [k, p] of Object.entries(s.properties ?? {})) {
        if (s.required?.includes(k) && p.default !== undefined) o[k] = p.default;
      }
      return o;
    }
    case "array":
      return [];
    default:
      return s.default;
  }
}

const input =
  "min-h-9 w-full min-w-0 rounded-md border border-border bg-surface px-3 type-body text-on-background placeholder:text-text-tertiary focus:border-focus focus:outline-none";

/** The form of a configuration object. */
export function SchemaForm({ schema, value, onChange }: { schema: Schema; value: Json; onChange: (v: Json) => void }) {
  return <ObjectFields schema={schema} value={isObj(value) ? value : {}} onChange={onChange} />;
}

/** An object's properties, as the rows of a group. */
function ObjectFields({ schema, value, onChange }: { schema: Schema; value: Obj; onChange: (v: Obj) => void }) {
  const props = Object.entries(schema.properties ?? {});
  if (props.length === 0) return <JsonField value={value} onChange={(v) => onChange(isObj(v) ? v : {})} />;
  const set = (k: string, v: Json) => {
    const next = { ...value };
    if (v === undefined) delete next[k];
    else next[k] = v;
    onChange(next);
  };
  return (
    <div className="flex flex-col divide-y divide-border overflow-hidden rounded-[14px] bg-surface">
      {props.map(([k, p]) => (
        <Property key={k} name={k} schema={p} required={schema.required?.includes(k) ?? false} value={value[k]} onChange={(v) => set(k, v)} />
      ))}
    </div>
  );
}

/** A property: its label and description, and its control beside or below them. */
function Property({
  name,
  schema: s,
  required,
  value,
  onChange,
}: {
  name: string;
  schema: Schema;
  required: boolean;
  value: Json;
  onChange: (v: Json) => void;
}) {
  const type = typeOf(s);
  const label = (
    <span className="type-body">
      {labelOf(name, s)}
      {required && <span className="text-danger"> *</span>}
    </span>
  );
  const head = (control?: ReactNode) => (
    <div className="flex min-h-13 flex-wrap items-center gap-x-4 gap-y-2 px-4 py-2.5">
      <div className="min-w-0 flex-1 basis-56">
        {label}
        {s.description && <div className="type-footnote text-text-secondary">{s.description}</div>}
      </div>
      {control}
    </div>
  );
  const below = (control: ReactNode) => (
    <div className="pb-3">
      {head()}
      <div className="px-4">{control}</div>
    </div>
  );
  switch (type) {
    case "boolean":
      return head(
        <Toggle on={typeof value === "boolean" ? value : s.default === true} label={labelOf(name, s)} onChange={(on) => onChange(on)} />,
      );
    case "enum":
    case "string":
      if (s.enum) return head(<EnumField label={labelOf(name, s)} schema={s} required={required} value={value} onChange={onChange} />);
      return head(
        <div className="w-full sm:w-80">
          <TextField
            label={labelOf(name, s)}
            required={required}
            secret={secret(name, s)}
            type={s.format === "uri" ? "url" : "text"}
            value={typeof value === "string" ? value : ""}
            placeholder={typeof s.default === "string" ? s.default : undefined}
            onChange={(v) => onChange(v === "" && !required ? undefined : v)}
          />
        </div>,
      );
    case "integer":
    case "number":
      return head(
        <input
          type="number"
          className={`${input} sm:w-40`}
          aria-label={labelOf(name, s)}
          required={required}
          min={s.minimum}
          max={s.maximum}
          step={type === "integer" ? 1 : "any"}
          value={typeof value === "number" ? value : ""}
          placeholder={s.default !== undefined ? String(s.default) : undefined}
          onChange={(e) => onChange(e.target.value === "" ? undefined : Number(e.target.value))}
        />,
      );
    case "object": {
      if (!s.properties) return below(<JsonField value={value ?? {}} onChange={onChange} />);
      // An optional object is set or left out as a whole.
      const set = isObj(value);
      return (
        <div className="pb-3">
          {head(!required && <Toggle on={set} label={labelOf(name, s)} onChange={(on) => onChange(on ? initial(s) : undefined)} />)}
          {(set || required) && (
            <div className="mx-4 rounded-[12px] border border-border">
              <ObjectFields schema={s} value={set ? value : {}} onChange={onChange} />
            </div>
          )}
        </div>
      );
    }
    case "array": {
      const items = s.items ?? {};
      const list = Array.isArray(value) ? value : [];
      const itemType = typeOf(items);
      if (itemType === "object" && items.properties) return below(<ObjectList schema={items} value={list} onChange={onChange} />);
      if (items.enum) return below(<EnumSet schema={items} value={list} onChange={(v) => onChange(v.length ? v : undefined)} />);
      if (itemType === "string" || itemType === "integer" || itemType === "number")
        return below(<Lines label={labelOf(name, s)} numeric={itemType !== "string"} value={list} onChange={(v) => onChange(v.length ? v : undefined)} />);
      return below(<JsonField value={value ?? []} onChange={onChange} />);
    }
    default:
      return below(<JsonField value={value} onChange={onChange} />);
  }
}

/** A text input, revealed by a button when it holds a secret. */
function TextField({
  label,
  required,
  secret,
  type,
  value,
  placeholder,
  onChange,
}: {
  label: string;
  required: boolean;
  secret: boolean;
  type: string;
  value: string;
  placeholder?: string;
  onChange: (v: string) => void;
}) {
  const [shown, setShown] = useState(false);
  return (
    <div className="relative">
      <input
        type={secret && !shown ? "password" : type}
        aria-label={label}
        required={required}
        autoComplete={secret ? "new-password" : "off"}
        className={`${input} ${secret ? "pr-10" : ""}`}
        value={value}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value)}
      />
      {secret && (
        <button
          type="button"
          aria-label={shown ? t("schema.hide") : t("schema.show")}
          title={shown ? t("schema.hide") : t("schema.show")}
          onClick={() => setShown(!shown)}
          className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-text-secondary hover:text-on-background"
        >
          <Icon name={shown ? "lock" : "key"} size={16} />
        </button>
      )}
    </div>
  );
}

function EnumField({
  label,
  schema: s,
  required,
  value,
  onChange,
}: {
  label: string;
  schema: Schema;
  required: boolean;
  value: Json;
  onChange: (v: Json) => void;
}) {
  const options = s.enum ?? [];
  const index = options.findIndex((o) => o === value);
  return (
    <select
      aria-label={label}
      className="min-h-9 max-w-full rounded-md bg-fill px-2 type-callout text-on-background"
      value={index < 0 ? "" : String(index)}
      onChange={(e) => onChange(e.target.value === "" ? undefined : options[Number(e.target.value)])}
    >
      {(!required || index < 0) && (
        <option value="">{s.default !== undefined ? t("schema.default", { value: String(s.default) }) : "—"}</option>
      )}
      {options.map((o, i) => (
        <option key={String(o)} value={String(i)}>
          {String(o)}
        </option>
      ))}
    </select>
  );
}

/** Values chosen among an enumeration, as checkboxes. */
function EnumSet({ schema: s, value, onChange }: { schema: Schema; value: Json[]; onChange: (v: Json[]) => void }) {
  return (
    <div className="flex flex-wrap gap-x-5 gap-y-2">
      {(s.enum ?? []).map((o) => (
        <label key={String(o)} className="inline-flex items-center gap-2 type-body">
          <input
            type="checkbox"
            className="size-4 accent-accent"
            checked={value.includes(o)}
            onChange={(e) => onChange(e.target.checked ? [...value, o] : value.filter((x) => x !== o))}
          />
          {String(o)}
        </label>
      ))}
    </div>
  );
}

/** A list of strings or numbers, one per line. */
function Lines({ label, numeric, value, onChange }: { label: string; numeric: boolean; value: Json[]; onChange: (v: Json[]) => void }) {
  const [text, setText] = useState(value.map(String).join("\n"));
  return (
    <textarea
      aria-label={label}
      className={`${input} min-h-20 py-2 font-mono`}
      rows={Math.min(8, Math.max(2, value.length + 1))}
      placeholder={t("schema.onePerLine")}
      value={text}
      onChange={(e) => {
        setText(e.target.value);
        const lines = e.target.value
          .split("\n")
          .map((l) => l.trim())
          .filter(Boolean);
        onChange(numeric ? lines.map(Number).filter((n) => !Number.isNaN(n)) : lines);
      }}
    />
  );
}

/** A value of no shape the form knows, edited as JSON. */
function JsonField({ value, onChange }: { value: Json; onChange: (v: Json) => void }) {
  const [text, setText] = useState(value === undefined ? "" : JSON.stringify(value, null, 2));
  const [bad, setBad] = useState(false);
  return (
    <div className="flex flex-col gap-1">
      <textarea
        className={`${input} min-h-24 py-2 font-mono type-footnote ${bad ? "border-danger" : ""}`}
        rows={Math.min(14, Math.max(3, text.split("\n").length))}
        spellCheck={false}
        value={text}
        onChange={(e) => {
          setText(e.target.value);
          if (e.target.value.trim() === "") {
            setBad(false);
            onChange(undefined);
            return;
          }
          try {
            onChange(JSON.parse(e.target.value));
            setBad(false);
          } catch {
            setBad(true);
          }
        }}
      />
      {bad && <span className="type-footnote text-danger">{t("schema.invalidJson")}</span>}
    </div>
  );
}

/** The title of an object in a list: its name or ID, else its place. */
function itemTitle(v: Json, i: number): string {
  if (isObj(v)) for (const k of ["name", "id", "definition", "kind", "host"]) if (typeof v[k] === "string" && v[k]) return v[k];
  return t("schema.item", { number: i + 1 });
}

/** Objects of a list, as cards that open to their fields. */
function ObjectList({ schema, value, onChange }: { schema: Schema; value: Json[]; onChange: (v: Json[] | undefined) => void }) {
  const [open, setOpen] = useState<number | undefined>();
  const set = (i: number, v: Json) => onChange(value.map((x, j) => (j === i ? v : x)));
  return (
    <div className="flex flex-col gap-2">
      {value.map((v, i) => (
        // Items have no identity of their own: their place and the list's
        // length key them, so that removing one draws the rest anew.
        <div key={`${i}/${value.length}`} className="rounded-[12px] border border-border">
          <div className="flex items-center gap-2 px-3 py-1.5">
            <button
              type="button"
              aria-expanded={open === i}
              onClick={() => setOpen(open === i ? undefined : i)}
              className="flex min-h-9 min-w-0 flex-1 items-center gap-2 text-left"
            >
              <Icon name="expand" size={16} className={`shrink-0 transition-transform ${open === i ? "" : "-rotate-90"}`} />
              <span className="truncate type-headline">{itemTitle(v, i)}</span>
            </button>
            <Button
              variant="plain"
              className="min-h-9 text-danger"
              onClick={() => {
                const next = value.filter((_, j) => j !== i);
                setOpen(undefined);
                onChange(next.length ? next : undefined);
              }}
            >
              {t("schema.remove")}
            </Button>
          </div>
          {open === i && (
            <div className="border-t border-border">
              <ObjectFields schema={schema} value={isObj(v) ? v : {}} onChange={(o) => set(i, o)} />
            </div>
          )}
        </div>
      ))}
      <Button
        variant="secondary"
        className="self-start"
        onClick={() => {
          setOpen(value.length);
          onChange([...value, initial(schema)]);
        }}
      >
        <Icon name="add" size={18} />
        {t("schema.add")}
      </Button>
    </div>
  );
}

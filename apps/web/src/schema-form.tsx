// A form drawn from a JSON Schema, as plugins describe their configuration
// (docs/clients.md §6.3.3): objects as groups of rows, arrays of objects as
// cards to add and remove, lists of strings as lines, enumerations as
// choices, values of a source the schema names (x-mavio-options) as a
// searchable choice whose option gives its settings' schema, and what has
// no shape the form knows, such as an object of arbitrary keys, as JSON.
// Properties the server generates are left out.
import { createContext, type ReactNode, useContext, useState } from "react";

import { Tip } from "./controls/tooltip.tsx";
import { locale, t } from "./i18n.ts";
import { Icon } from "./icon.tsx";
import { Button } from "./ui.tsx";
import { Checkbox, NumberField, Switch } from "./controls/inputs.tsx";
import { Picker, type PickerFilter, type PickerOption, type PickerTone, Select } from "./controls/select.tsx";

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
  readOnly?: boolean;
  oneOf?: Schema[];
  const?: unknown;
  "x-mavio-options"?: OptionsSpec;
}

/** Where a property's values come from, and the sibling properties a chosen option fills. */
interface OptionsSpec {
  source: string;
  /** The property taking the chosen option's settings, by their schema. */
  settings?: string;
  /** The property given the chosen option's name while it has none of its own. */
  name?: string;
}

/** A value of a source of options, with the schema of the settings it takes. */
export interface SchemaOption {
  value: string;
  name: string;
  description: string;
  tags: string[];
  settingsSchema: string;
}

/** The options of a source, undefined while they load or when they cannot be had. */
type Options = (source: string) => SchemaOption[] | undefined;

const OptionsContext = createContext<Options>(() => undefined);

/** The sources of options a schema names. */
export function optionSources(s: Schema): string[] {
  const out = new Set<string>();
  const walk = (s: Schema) => {
    if (s["x-mavio-options"]) out.add(s["x-mavio-options"].source);
    Object.values(s.properties ?? {}).forEach(walk);
    if (s.items) walk(s.items);
  };
  walk(s);
  return [...out];
}

/** Whether the server generates a property: a readOnly UUID. */
const generated = (s: Schema) => s.readOnly === true && s.format === "uuid";

/** The choices of an enumeration, of enum or of oneOf's constants with their titles. */
function choices(s: Schema): { value: unknown; label: string }[] | undefined {
  if (s.enum) return s.enum.map((v) => ({ value: v, label: String(v) }));
  if (s.oneOf?.length && s.oneOf.every((c) => "const" in c)) return s.oneOf.map((c) => ({ value: c.const, label: c.title ?? String(c.const) }));
  return undefined;
}

/** The tags of options a picker filters by, in groups, with their words and tones. */
const tagGroups: { key: string; label: () => string; tone: PickerTone }[][] = [
  [
    { key: "torrent", label: () => t("schema.tagTorrent"), tone: "neutral" },
    { key: "usenet", label: () => t("schema.tagUsenet"), tone: "neutral" },
  ],
  [
    { key: "public", label: () => t("schema.tagPublic"), tone: "success" },
    { key: "semi-private", label: () => t("schema.tagSemiPrivate"), tone: "warning" },
    { key: "private", label: () => t("schema.tagPrivate"), tone: "danger" },
  ],
];

const knownTags = new Map(tagGroups.flat().map((g) => [g.key, g]));

/** A language's name without its region, the tag itself when it has none. */
function languageName(tag: string): string {
  try {
    return new Intl.DisplayNames([locale], { type: "language" }).of(new Intl.Locale(tag).language) ?? tag;
  } catch {
    return tag;
  }
}

/** Options as a picker lists them, by name: protocol and privacy as badges, the language beside them. */
function pickerOptions(list: SchemaOption[]): PickerOption[] {
  const collator = new Intl.Collator(locale, { sensitivity: "base", numeric: true });
  return list.toSorted((a, b) => collator.compare(a.name, b.name)).map((o) => ({
    value: o.value,
    label: o.name,
    detail: o.description,
    meta: o.tags.filter((tag) => !knownTags.has(tag)).map(languageName).join(" · ") || undefined,
    badges: o.tags.flatMap((tag) => {
      const known = knownTags.get(tag);
      return known ? [{ label: known.label(), tone: known.tone }] : [];
    }),
    keys: o.tags,
  }));
}

/**
 * The server's proxies as a picker lists them: none first, then the system
 * proxy, then the others by name, each with its address.
 */
function proxyOptions(list: SchemaOption[]): PickerOption[] {
  const collator = new Intl.Collator(locale, { sensitivity: "base", numeric: true });
  const system = list.find((o) => o.value === systemProxy);
  return [
    { value: "", label: t("proxies.none"), detail: t("proxies.noneDetail") },
    ...(system ? [{ value: systemProxy, label: t("proxies.system"), detail: system.description || t("proxies.systemUnset") }] : []),
    ...list
      .filter((o) => o.value !== systemProxy)
      .toSorted((a, b) => collator.compare(a.name, b.name))
      .map((o) => ({ value: o.value, label: o.name, detail: o.description })),
  ];
}

/** The system proxy's ID. */
const systemProxy = "system";

/** The groups of filters a list of options can be told apart by, those of two tags or more. */
function pickerFilters(list: SchemaOption[]): PickerFilter[][] {
  const present = new Set(list.flatMap((o) => o.tags));
  return tagGroups
    .map((group) => group.filter((g) => present.has(g.key)).map((g) => ({ key: g.key, label: g.label() })))
    .filter((group) => group.length > 1);
}

/** A schema of JSON, undefined when it does not parse. */
function parseSchema(text: string): Schema | undefined {
  try {
    return JSON.parse(text) as Schema;
  } catch {
    return undefined;
  }
}

type Json = unknown;
type Obj = Record<string, Json>;

const isObj = (v: Json): v is Obj => typeof v === "object" && v !== null && !Array.isArray(v);

function typeOf(s: Schema): string | undefined {
  if (Array.isArray(s.type)) return s.type.find((x) => x !== "null");
  if (s.type) return s.type;
  if (choices(s)) return "enum";
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

/** The form of a configuration object; options gives the values of the sources its schema names. */
export function SchemaForm({ schema, value, onChange, options }: { schema: Schema; value: Json; onChange: (v: Json) => void; options?: Options }) {
  return (
    <OptionsContext value={options ?? (() => undefined)}>
      <ObjectFields schema={schema} value={isObj(value) ? value : {}} onChange={onChange} />
    </OptionsContext>
  );
}

/** An object's properties, as the rows of a group. */
function ObjectFields({ schema, value, onChange }: { schema: Schema; value: Obj; onChange: (v: Obj) => void }) {
  const options = useContext(OptionsContext);
  const props = Object.entries(schema.properties ?? {}).filter(([, p]) => !generated(p));
  if (props.length === 0) return <JsonField value={value} onChange={(v) => onChange(isObj(v) ? v : {})} />;
  const set = (k: string, v: Json) => {
    const next = { ...value };
    if (v === undefined) delete next[k];
    else next[k] = v;
    onChange(next);
  };
  // The properties whose schema a chosen option gives, null when it takes
  // no settings.
  const given = new Map<string, Schema | null>();
  for (const [k, p] of props) {
    const spec = p["x-mavio-options"];
    const chosen = spec?.settings ? options(spec.source)?.find((o) => o.value === value[k]) : undefined;
    if (spec?.settings && chosen) given.set(spec.settings, (chosen.settingsSchema && parseSchema(chosen.settingsSchema)) || null);
  }
  // Choosing another option leaves the settings of the one before, and
  // names the object after it while it bears that one's name or none.
  const choose = (k: string, spec: OptionsSpec, list: SchemaOption[], v: string | undefined) => {
    const before = list.find((o) => o.value === value[k]);
    const after = list.find((o) => o.value === v);
    const next = { ...value };
    if (v === undefined) delete next[k];
    else next[k] = v;
    if (spec.settings && before?.value !== after?.value) delete next[spec.settings];
    if (spec.name && after && (!next[spec.name] || next[spec.name] === before?.name)) next[spec.name] = after.name;
    onChange(next);
  };
  return (
    <div className="flex flex-col divide-y divide-border overflow-hidden rounded-[14px] bg-surface">
      {props.map(([k, p]) => {
        const required = schema.required?.includes(k) ?? false;
        if (given.has(k)) {
          const s = given.get(k);
          if (!s?.properties || Object.keys(s.properties).length === 0) return null;
          return <Property key={k} name={k} schema={{ ...s, title: p.title, description: p.description }} required value={value[k]} onChange={(v) => set(k, v)} />;
        }
        const spec = p["x-mavio-options"];
        const list = spec && options(spec.source);
        if (spec && list)
          return <Property key={k} name={k} schema={p} required={required} value={value[k]} onChange={(v) => set(k, v)} choices={list} onChoose={(v) => choose(k, spec, list, v)} />;
        return <Property key={k} name={k} schema={p} required={required} value={value[k]} onChange={(v) => set(k, v)} />;
      })}
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
  choices: list,
  onChoose,
}: {
  name: string;
  schema: Schema;
  required: boolean;
  value: Json;
  onChange: (v: Json) => void;
  /** The options of the source the property's schema names, chosen by onChoose. */
  choices?: SchemaOption[];
  onChoose?: (v: string | undefined) => void;
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
  if (list && onChoose)
    return head(
      <Picker
        className="w-full sm:w-80"
        label={labelOf(name, s)}
        value={typeof value === "string" ? value : typeof s.default === "string" ? s.default : undefined}
        options={s["x-mavio-options"]?.source === "proxies" ? proxyOptions(list) : pickerOptions(list)}
        filters={pickerFilters(list)}
        labels={{
          placeholder: t("schema.choose"),
          empty: t("schema.noMatches"),
          all: t("schema.all"),
          count: (count) => t("schema.matches", { count }),
          first: (shown, count) => t("schema.matchesFirst", { shown, count }),
        }}
        onChange={onChoose}
      />,
    );
  // Text shown to administrators, which takes no value.
  if (s.readOnly && !type) return head();
  switch (type) {
    case "boolean":
      return head(
        <Switch on={typeof value === "boolean" ? value : s.default === true} label={labelOf(name, s)} onChange={(on) => onChange(on)} />,
      );
    case "enum":
    case "string":
      if (choices(s)) return head(<EnumField label={labelOf(name, s)} schema={s} required={required} value={value} onChange={onChange} />);
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
        <NumberField
          className="sm:w-40"
          label={labelOf(name, s)}
          min={s.minimum}
          max={s.maximum}
          integer={type === "integer"}
          value={typeof value === "number" ? value : undefined}
          placeholder={s.default !== undefined ? String(s.default) : undefined}
          onChange={onChange}
        />,
      );
    case "object": {
      if (!s.properties) return below(<JsonField value={value ?? {}} onChange={onChange} />);
      // An optional object is set or left out as a whole.
      const set = isObj(value);
      return (
        <div className="pb-3">
          {head(!required && <Switch on={set} label={labelOf(name, s)} onChange={(on) => onChange(on ? initial(s) : undefined)} />)}
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
      if (choices(items)) return below(<EnumSet schema={items} value={list} onChange={(v) => onChange(v.length ? v : undefined)} />);
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
        <Tip label={shown ? t("schema.hide") : t("schema.show")}>
          <button
            type="button"
            aria-label={shown ? t("schema.hide") : t("schema.show")}
            onClick={() => setShown(!shown)}
            className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-text-secondary hover:text-on-background"
          >
            <Icon name={shown ? "lock" : "key"} size={16} />
          </button>
        </Tip>
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
  const options = choices(s) ?? [];
  const index = options.findIndex((o) => o.value === value);
  const fallback = s.default !== undefined && (options.find((o) => o.value === s.default)?.label ?? String(s.default));
  return (
    <Select
      label={label}
      value={index < 0 ? "" : String(index)}
      options={[
        ...(!required || index < 0 ? [{ value: "", label: fallback ? t("schema.default", { value: fallback }) : "—" }] : []),
        ...options.map((o, i) => ({ value: String(i), label: o.label })),
      ]}
      onChange={(v) => onChange(v === "" ? undefined : options[Number(v)]?.value)}
    />
  );
}

/** Values chosen among an enumeration, as checkboxes. */
function EnumSet({ schema: s, value, onChange }: { schema: Schema; value: Json[]; onChange: (v: Json[]) => void }) {
  return (
    <div className="flex flex-wrap gap-x-5 gap-y-2">
      {(choices(s) ?? []).map((o) => (
        <Checkbox key={String(o.value)} checked={value.includes(o.value)} onChange={(on) => onChange(on ? [...value, o.value] : value.filter((x) => x !== o.value))}>
          {o.label}
        </Checkbox>
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

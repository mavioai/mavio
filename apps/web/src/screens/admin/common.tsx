// What the pages of a server's administration share: their header, the
// link back from a page to its section's list, colored tiles, the
// confirmation of what cannot be undone, and times and their lengths.
import { clone, type DescMessage, type MessageShape } from "@bufbuild/protobuf";
import { Link, useNavigate, useParams, useSearch } from "@tanstack/react-router";
import type { ReactNode } from "react";

import { locale } from "../../i18n.ts";
import { Icon, type IconName } from "../../icon.tsx";
import type { SettingsSearch } from "../settings.tsx";

/** A page's title, the line under it, the buttons beside it, and its groups. */
export function Page({
  title,
  message,
  actions,
  back,
  children,
}: {
  title: ReactNode;
  message?: ReactNode;
  actions?: ReactNode;
  back?: ReactNode;
  children: ReactNode;
}) {
  return (
    <div className="flex flex-col gap-7">
      {back}
      <header className="flex flex-wrap items-center gap-4">
        <div className="min-w-0 flex-1">
          <h2 className="type-title1">{title}</h2>
          {message && <p className="mt-1 type-body text-text-secondary">{message}</p>}
        </div>
        {actions && <div className="flex flex-wrap items-center gap-3">{actions}</div>}
      </header>
      {children}
    </div>
  );
}

/**
 * A page's choice kept in the address, which Back and a reload return to:
 * one of values, the first when the address names none; choosing replaces
 * the page in the history rather than adding to it.
 */
export function useChoice<T extends string | number>(key: "view" | "filter", values: readonly T[]): [T, (v: T) => void] {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const navigate = useNavigate();
  const value = values.find((v) => String(v) === cur[key]) ?? (values[0] as T);
  const set = (v: T) =>
    void navigate({
      to: ".",
      search: (s: SettingsSearch) => ({ ...s, [key]: v === values[0] ? undefined : String(v) }),
      replace: true,
      resetScroll: false,
    } as never);
  return [value, set];
}

/** A link back to a section's list, keeping the server chosen. */
export function BackLink({ section, label }: { section: string; label: string }) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  return (
    <Link
      to="/settings/$section"
      params={{ section }}
      search={{ server: cur.server }}
      className="inline-flex items-center gap-1 self-start type-body text-accent-text"
    >
      <Icon name="back" size={18} />
      {label}
    </Link>
  );
}

/** A row linking to a page of the section shown, or of another section, or of Plugins for a plugin. */
export function PageLink({
  search,
  section: to,
  icon,
  color,
  label,
  detail,
}: {
  search: SettingsSearch;
  section?: string;
  icon?: IconName;
  color?: string;
  label: ReactNode;
  detail?: ReactNode;
}) {
  const cur = useSearch({ strict: false }) as SettingsSearch;
  const { section } = useParams({ strict: false });
  return (
    <Link
      to="/settings/$section"
      params={{ section: to ?? (search.plugin ? "plugins" : (section ?? "")) }}
      search={{ server: cur.server, ...search }}
      className="flex min-h-13 items-center gap-3 px-4 py-2.5 hover:tint"
    >
      {icon && <ColorTile icon={icon} color={color ?? "#8E8E93"} />}
      <div className="min-w-0 flex-1">
        <div className="truncate type-body">{label}</div>
        {detail && <div className="type-footnote break-words text-text-secondary">{detail}</div>}
      </div>
      <Icon name="forward" size={16} className="shrink-0 text-text-tertiary" />
    </Link>
  );
}

/** An icon on a colored tile. */
export function ColorTile({ icon, color, size = 32 }: { icon: IconName; color: string; size?: number }) {
  return (
    <span
      className="flex shrink-0 items-center justify-center rounded-[22%] text-white"
      style={{ background: color, width: size, height: size }}
    >
      <Icon name={icon} fill size={Math.round(size * 0.58)} />
    </span>
  );
}


/**
 * A copy of a message with some fields changed. Spreading a message would
 * lose its unset fields, whose defaults live on its prototype.
 */
export function patched<Desc extends DescMessage>(schema: Desc, message: MessageShape<Desc>, changes: Partial<MessageShape<Desc>>): MessageShape<Desc> {
  return Object.assign(clone(schema, message), changes);
}


const languageNames = new Intl.DisplayNames([locale], { type: "language", fallback: "none" });
const regionNames = new Intl.DisplayNames([locale], { type: "region", fallback: "none" });

/** The options of languages, named in the UI's language where it can, in order. */
export function languageOptions(languages: { code: string; name: string }[]) {
  return named(languages, (code) => languageNames.of(code));
}

/** The options of countries, named in the UI's language where it can, in order. */
export function countryOptions(countries: { code: string; name: string }[]) {
  return named(countries, (code) => regionNames.of(code));
}

function named(list: { code: string; name: string }[], of: (code: string) => string | undefined) {
  const seen = new Set<string>();
  return list
    .filter((x) => x.code && !seen.has(x.code) && seen.add(x.code))
    .map((x) => {
      let label = x.name;
      try {
        label = of(x.code) || x.name;
      } catch {
        // A code Intl does not take keeps the server's name.
      }
      return { value: x.code, label };
    })
    .sort((a, b) => a.label.localeCompare(b.label, locale));
}

const relative = new Intl.RelativeTimeFormat([locale], { numeric: "auto" });
const units: [Intl.RelativeTimeFormatUnit, number][] = [
  ["day", 86_400_000],
  ["hour", 3_600_000],
  ["minute", 60_000],
];

const megabytes = new Intl.NumberFormat([locale], { style: "unit", unit: "megabyte", maximumFractionDigits: 1 });
const gigabytes = new Intl.NumberFormat([locale], { style: "unit", unit: "gigabyte", maximumFractionDigits: 2 });

/** A file's size in megabytes, or gigabytes from a thousand megabytes. */
export function sizeOf(bytes: bigint | number): string {
  const n = Number(bytes);
  return n >= 1e9 ? gigabytes.format(n / 1e9) : megabytes.format(n / 1e6);
}

const capacities = (["terabyte", "gigabyte", "megabyte"] as const).map((unit) => new Intl.NumberFormat([locale], { style: "unit", unit, maximumFractionDigits: 1 }));

/** A disk's or a remote's size, in terabytes, gigabytes or megabytes as it is large, as file managers tell them. */
export function capacityOf(bytes: bigint | number): string {
  const n = Number(bytes);
  return n >= 1e12 ? capacities[0].format(n / 1e12) : n >= 1e9 ? capacities[1].format(n / 1e9) : capacities[2].format(n / 1e6);
}

/** When a time is from now, as "5 minutes ago" or "in 2 hours", in the UI's language. */
export function fromNow(date: Date, now = Date.now()): string {
  const ms = date.getTime() - now;
  for (const [unit, size] of units) {
    if (Math.abs(ms) >= size) return relative.format(Math.round(ms / size), unit);
  }
  return relative.format(Math.round(ms / 1000), "second");
}

const lengthUnits = Object.fromEntries(
  (["day", "hour", "minute", "second"] as const).map((unit) => [unit, new Intl.NumberFormat([locale], { style: "unit", unit, unitDisplay: "short" })]),
);

/** A length of time as its largest whole unit or two, as "2 hr 5 min" or "3 days", in the UI's language. */
export function lengthOf(ms: number): string {
  const f = (unit: keyof typeof lengthUnits, n: number) => lengthUnits[unit].format(n);
  const m = Math.round(ms / 60_000);
  if (m < 1) return f("second", Math.max(0, Math.round(ms / 1000)));
  if (m < 60) return f("minute", m);
  const h = Math.floor(m / 60);
  if (h < 24) return m % 60 ? `${f("hour", h)} ${f("minute", m % 60)}` : f("hour", h);
  const d = Math.floor(h / 24);
  return h % 24 ? `${f("day", d)} ${f("hour", h % 24)}` : f("day", d);
}

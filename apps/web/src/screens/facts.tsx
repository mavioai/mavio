// The facts of a title's details, on an item's details and on a discovered
// title (libs/design/components.md, Details Screen): captions over values,
// and the services streaming it.
import type { WatchProvider } from "@mavio/proto/mavio/plugin/v1/metadata_pb";
import type { ReactNode } from "react";

import { locale, t } from "../i18n.ts";

const dayFormat = new Intl.DateTimeFormat(locale, { dateStyle: "medium", timeZone: "UTC" });
const moneyFormat = new Intl.NumberFormat(locale, { style: "currency", currency: "USD", maximumFractionDigits: 0 });
const languages = new Intl.DisplayNames(locale, { type: "language" });

/** An ISO 8601 calendar date, as "Nov 2, 2026"; undefined when empty. */
export function dayText(date: string): string | undefined {
  return date ? dayFormat.format(new Date(`${date}T00:00:00Z`)) : undefined;
}

/** An amount of US dollars; undefined when 0, which stands for unknown. */
export function moneyText(dollars: bigint): string | undefined {
  return dollars > 0n ? moneyFormat.format(dollars) : undefined;
}

/** The name of an ISO 639-1 language in the UI's language. */
export function languageText(code: string): string | undefined {
  if (!code) return undefined;
  try {
    return languages.of(code) ?? code;
  } catch {
    return code;
  }
}

/** Facts, each a caption over its value, in as many columns as fit. */
export function FactList({ rows }: { rows: [string, ReactNode][] }) {
  if (rows.length === 0) return null;
  return (
    <dl className="grid gap-x-8 gap-y-5" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))" }}>
      {rows.map(([k, v]) => (
        <div key={k} className="min-w-0">
          <dt className="type-footnote font-semibold text-text-secondary">{k}</dt>
          <dd className="mt-1 type-callout break-words whitespace-pre-line">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

/** The services streaming a title by subscription in the server's country, each its logo and name. */
export function WatchProviders({ providers }: { providers: WatchProvider[] }) {
  if (providers.length === 0) return null;
  return (
    <div className="flex flex-col gap-2">
      <h3 className="type-footnote font-semibold text-text-secondary">{t("item.streaming")}</h3>
      <ul className="flex flex-wrap gap-2">
        {providers.map((p) => (
          <li key={p.name} className="inline-flex items-center gap-2 rounded-full bg-fill py-1 pr-3.5 pl-1 type-footnote font-semibold">
            {p.logoUrl ? (
              <img src={p.logoUrl} alt="" loading="lazy" className="size-6 rounded-full object-cover" />
            ) : (
              <span className="size-6" />
            )}
            {p.name}
          </li>
        ))}
      </ul>
    </div>
  );
}

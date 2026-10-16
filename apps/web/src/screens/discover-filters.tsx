// A catalog's filters and sorts (docs/clients.md §6.3.28): the dialog
// choosing the order of its titles and the filters it takes — genres,
// studios, keywords and streaming services by their values, the original
// language, and ranges of years, runtime, rating and votes.
import { type CatalogFilters, type NamedValue, type ServerEntry, useFilterValues, useLocalization } from "@mavio/client-ts";
import type { Catalog } from "@mavio/proto/mavio/acquisition/v1/discover_pb";
import { MediaKind } from "@mavio/proto/mavio/acquisition/v1/profile_pb";
import { DiscoverFilter, DiscoverSort } from "@mavio/proto/mavio/plugin/v1/discovery_pb";
import { type ReactNode, useEffect, useState } from "react";

import { discoverLanguage, locale, t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { Button, Select } from "../ui.tsx";
import { countryOptions, languageOptions } from "./admin/common.tsx";
import { field, Modal } from "./modal.tsx";

const sortNames: Record<DiscoverSort, () => string> = {
  [DiscoverSort.UNSPECIFIED]: () => t("discover.sortDefault"),
  [DiscoverSort.POPULARITY]: () => t("discover.sortPopularity"),
  [DiscoverSort.RELEASE_DATE]: () => t("discover.sortReleaseDate"),
  [DiscoverSort.RATING]: () => t("discover.sortRating"),
  [DiscoverSort.VOTE_COUNT]: () => t("discover.sortVotes"),
  [DiscoverSort.TITLE]: () => t("discover.sortTitle"),
};

/** The country of the streaming services until another is chosen, the one with the most of them. */
const defaultRegion = "US";

const strings = (v: unknown): string[] | undefined => {
  const a = Array.isArray(v) ? v.filter((x): x is string => typeof x === "string" && x !== "").slice(0, 50) : [];
  return a.length > 0 ? a : undefined;
};
const named = (v: unknown): NamedValue[] | undefined => {
  const a = Array.isArray(v)
    ? v
        .filter((x): x is NamedValue => typeof x?.id === "string" && x.id !== "" && typeof x?.name === "string")
        .slice(0, 50)
        .map((x) => ({ id: x.id, name: x.name }))
    : [];
  return a.length > 0 ? a : undefined;
};
const count = (v: unknown, max: number): number | undefined => {
  const n = Number(v);
  return Number.isInteger(n) && n > 0 && n <= max ? n : undefined;
};
const code = (v: unknown): string | undefined => (typeof v === "string" && /^[A-Za-z]{2,3}$/.test(v) ? v : undefined);

/** The filters in a catalog page's address, those that are well formed; none when none is set. */
export function parseFilters(raw: unknown): CatalogFilters | undefined {
  if (typeof raw !== "object" || raw === null) return undefined;
  const r = raw as Record<string, unknown>;
  const sort = count(r.sort, DiscoverSort.TITLE) as DiscoverSort | undefined;
  const f: CatalogFilters = {
    sort,
    ascending: sort && r.ascending === true ? true : undefined,
    genres: strings(r.genres),
    studios: named(r.studios),
    keywords: named(r.keywords),
    watchProviders: strings(r.watchProviders),
    watchRegion: code(r.watchRegion),
    originalLanguage: code(r.originalLanguage),
    yearFrom: count(r.yearFrom, 9999),
    yearTo: count(r.yearTo, 9999),
    runtimeMin: count(r.runtimeMin, 1000),
    runtimeMax: count(r.runtimeMax, 1000),
    ratingMin: count(r.ratingMin, 10),
    votesMin: count(r.votesMin, 1_000_000),
  };
  for (const k of Object.keys(f) as (keyof CatalogFilters)[]) if (f[k] === undefined) delete f[k];
  return Object.keys(f).length > 0 ? f : undefined;
}

function Section({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      <h3 className="type-headline">{title}</h3>
      {children}
    </section>
  );
}

/** A value to pick, with its logo when it has one. */
function Chip({ on, label, image, onClick }: { on: boolean; label: string; image?: string; onClick: () => void }) {
  return (
    <button
      type="button"
      aria-pressed={on}
      onClick={onClick}
      className={`inline-flex min-h-8 items-center gap-1.5 rounded-full px-3 type-footnote font-semibold ${on ? "bg-accent text-on-accent" : "bg-fill hover:bg-fill-selected"}`}
    >
      {image && <img src={image} alt="" className="size-5 rounded-[5px]" loading="lazy" />}
      {label}
    </button>
  );
}

/** A number field for a range's end; empty is unset. */
function NumberField({
  label,
  value,
  min,
  max,
  wide = false,
  onChange,
}: {
  label: string;
  value?: number;
  min: number;
  max: number;
  wide?: boolean;
  onChange: (v?: number) => void;
}) {
  return (
    <input
      type="number"
      inputMode="numeric"
      className={`${field} flex-none ${wide ? "w-44" : "w-28"}`}
      aria-label={label}
      placeholder={label}
      min={min}
      max={max}
      value={value ?? ""}
      onChange={(e) => {
        const n = e.target.valueAsNumber;
        onChange(Number.isFinite(n) && n > 0 ? Math.min(max, Math.max(min, Math.round(n))) : undefined);
      }}
    />
  );
}

/** Values found by name, as studios and keywords are: the chosen ones, a field, and what it finds. */
function SearchedValues({
  server,
  catalog,
  filter,
  mediaKind,
  title,
  value,
  onChange,
}: {
  server: ServerEntry;
  catalog: string;
  filter: DiscoverFilter;
  mediaKind: MediaKind;
  title: string;
  value: NamedValue[];
  onChange: (v: NamedValue[]) => void;
}) {
  const [text, setText] = useState("");
  const [query, setQuery] = useState("");
  useEffect(() => {
    const timer = setTimeout(() => setQuery(text.trim()), 300);
    return () => clearTimeout(timer);
  }, [text]);
  const found = useFilterValues(server, { catalog, filter, mediaKind, query, language: discoverLanguage }, query !== "");
  const chosen = new Set(value.map((v) => v.id));
  return (
    <Section title={title}>
      {value.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {value.map((v) => (
            <button
              key={v.id}
              type="button"
              onClick={() => onChange(value.filter((x) => x.id !== v.id))}
              className="inline-flex min-h-8 items-center gap-1 rounded-full bg-accent px-3 type-footnote font-semibold text-on-accent"
            >
              {v.name}
              <Icon name="close" size={14} />
            </button>
          ))}
        </div>
      )}
      <input
        type="search"
        className={field}
        aria-label={t("discover.searchValues", { filter: title })}
        placeholder={t("discover.searchValues", { filter: title })}
        value={text}
        onChange={(e) => setText(e.target.value)}
      />
      {query !== "" && (
        <div className="flex max-h-32 flex-wrap gap-2 overflow-y-auto">
          {(found.data ?? [])
            .filter((v) => !chosen.has(v.id))
            .slice(0, 20)
            .map((v) => (
              <Chip key={v.id} on={false} label={v.name} onClick={() => {
                  onChange([...value, { id: v.id, name: v.name }]);
                  setText("");
                }} />
            ))}
        </div>
      )}
    </Section>
  );
}

/** Values listed whole, as genres and streaming services are, to pick from. */
function ListedValues({
  server,
  catalog,
  filter,
  mediaKind,
  region,
  title,
  value,
  onChange,
  control,
}: {
  server: ServerEntry;
  catalog: string;
  filter: DiscoverFilter;
  mediaKind: MediaKind;
  region?: string;
  title: string;
  value: string[];
  onChange: (v: string[]) => void;
  /** What chooses the values listed, as the country of streaming services does; the section shows with it even when none is listed. */
  control?: ReactNode;
}) {
  const values = useFilterValues(server, { catalog, filter, mediaKind, language: discoverLanguage, region });
  const list = values.data ?? [];
  if (values.isSuccess && list.length === 0 && !control) return null;
  return (
    <Section title={title}>
      {control}
      <div className={`flex flex-wrap gap-2 ${list.length > 30 ? "max-h-40 overflow-y-auto" : ""}`}>
        {list.map((v) => {
          const on = value.includes(v.id);
          return (
            <Chip
              key={v.id}
              on={on}
              label={v.name}
              image={v.imageUrl || undefined}
              onClick={() => onChange(on ? value.filter((x) => x !== v.id) : [...value, v.id])}
            />
          );
        })}
      </div>
    </Section>
  );
}

/**
 * The dialog of a catalog's filters and sorts, offering those the catalog
 * takes; its values are those of the catalog's kind of title, movies when
 * it mixes them.
 */
export function FiltersDialog({
  server,
  catalog,
  mediaKind: kind,
  value,
  onApply,
  onClose,
}: {
  server: ServerEntry;
  catalog: Catalog;
  mediaKind: MediaKind;
  value: CatalogFilters;
  onApply: (f: CatalogFilters) => void;
  onClose: () => void;
}) {
  const [f, setF] = useState(value);
  const set = (patch: Partial<CatalogFilters>) => setF((cur) => ({ ...cur, ...patch }));
  const takes = (x: DiscoverFilter) => catalog.filters.includes(x);
  const mediaKind = kind === MediaKind.UNSPECIFIED ? MediaKind.MOVIE : kind;
  const { languages, countries } = useLocalization(server);
  const region = f.watchRegion ?? defaultRegion;
  const year = new Date().getFullYear() + 5;
  const common = { server, catalog: catalog.id, mediaKind };
  return (
    <Modal title={t("discover.filters")} wide onClose={onClose}>
      <div className="-mx-1 flex min-h-0 flex-col gap-5 overflow-y-auto px-1">
        {catalog.sorts.length > 0 && (
          <Section title={t("discover.sortBy")}>
            <div className="flex flex-wrap gap-2">
              <Select
                value={String(f.sort ?? DiscoverSort.UNSPECIFIED)}
                label={t("discover.sortBy")}
                options={[DiscoverSort.UNSPECIFIED, ...catalog.sorts].map((s) => ({ value: String(s), label: sortNames[s]() }))}
                onChange={(v) => set({ sort: Number(v) as DiscoverSort })}
              />
              {!!f.sort && (
                <Select
                  value={f.ascending ? "asc" : "desc"}
                  label={t("discover.sortBy")}
                  options={[
                    { value: "desc", label: t("discover.descending") },
                    { value: "asc", label: t("discover.ascending") },
                  ]}
                  onChange={(v) => set({ ascending: v === "asc" })}
                />
              )}
            </div>
          </Section>
        )}
        {takes(DiscoverFilter.GENRE) && (
          <ListedValues {...common} filter={DiscoverFilter.GENRE} title={t("discover.genres")} value={f.genres ?? []} onChange={(genres) => set({ genres })} />
        )}
        {takes(DiscoverFilter.STUDIO) && mediaKind === MediaKind.MOVIE && (
          <SearchedValues {...common} filter={DiscoverFilter.STUDIO} title={t("discover.studios")} value={f.studios ?? []} onChange={(studios) => set({ studios })} />
        )}
        {takes(DiscoverFilter.KEYWORD) && (
          <SearchedValues {...common} filter={DiscoverFilter.KEYWORD} title={t("discover.keywords")} value={f.keywords ?? []} onChange={(keywords) => set({ keywords })} />
        )}
        {takes(DiscoverFilter.WATCH_PROVIDER) && (
          <ListedValues
            {...common}
            filter={DiscoverFilter.WATCH_PROVIDER}
            region={region}
            title={t("discover.watchProviders")}
            value={f.watchProviders ?? []}
            onChange={(watchProviders) => set({ watchProviders, watchRegion: region })}
            control={
              <Select
                value={region}
                label={t("discover.region")}
                options={countryOptions(countries)}
                onChange={(watchRegion) => set({ watchRegion, watchProviders: undefined })}
              />
            }
          />
        )}
        {takes(DiscoverFilter.ORIGINAL_LANGUAGE) && (
          <Section title={t("discover.originalLanguage")}>
            <Select
              value={f.originalLanguage ?? ""}
              label={t("discover.originalLanguage")}
              options={[{ value: "", label: t("discover.any") }, ...languageOptions(languages)]}
              onChange={(v) => set({ originalLanguage: v || undefined })}
            />
          </Section>
        )}
        {takes(DiscoverFilter.RELEASE_DATE) && (
          <Section title={t("discover.released")}>
            <div className="flex items-center gap-2">
              <NumberField label={t("discover.from")} value={f.yearFrom} min={1870} max={year} onChange={(yearFrom) => set({ yearFrom })} />
              <span className="text-text-secondary">–</span>
              <NumberField label={t("discover.to")} value={f.yearTo} min={1870} max={year} onChange={(yearTo) => set({ yearTo })} />
            </div>
          </Section>
        )}
        {takes(DiscoverFilter.RUNTIME) && (
          <Section title={t("discover.runtime")}>
            <div className="flex items-center gap-2">
              <NumberField label={t("discover.from")} value={f.runtimeMin} min={1} max={1000} onChange={(runtimeMin) => set({ runtimeMin })} />
              <span className="text-text-secondary">–</span>
              <NumberField label={t("discover.to")} value={f.runtimeMax} min={1} max={1000} onChange={(runtimeMax) => set({ runtimeMax })} />
            </div>
          </Section>
        )}
        {(takes(DiscoverFilter.RATING) || takes(DiscoverFilter.VOTE_COUNT)) && (
          <div className="flex flex-wrap gap-5">
            {takes(DiscoverFilter.RATING) && (
              <Section title={t("discover.ratingMin")}>
                <Select
                  value={String(f.ratingMin ?? 0)}
                  label={t("discover.ratingMin")}
                  options={Array.from({ length: 10 }, (_, i) => ({ value: String(i), label: i ? i.toLocaleString(locale) : t("discover.any") }))}
                  onChange={(v) => set({ ratingMin: Number(v) || undefined })}
                />
              </Section>
            )}
            {takes(DiscoverFilter.VOTE_COUNT) && (
              <Section title={t("discover.votesMin")}>
                <NumberField wide label={t("discover.votesMin")} value={f.votesMin} min={1} max={1_000_000} onChange={(votesMin) => set({ votesMin })} />
              </Section>
            )}
          </div>
        )}
      </div>
      <div className="flex justify-end gap-2">
        <Button variant="plain" onClick={() => setF({})}>
          {t("discover.reset")}
        </Button>
        <Button variant="secondary" onClick={onClose}>
          {t("action.cancel")}
        </Button>
        <Button onClick={() => onApply(f)}>{t("discover.apply")}</Button>
      </div>
    </Modal>
  );
}

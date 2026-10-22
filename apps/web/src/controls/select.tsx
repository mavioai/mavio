// A choice among options (docs/clients.md §6.4): Base UI's select, its
// popup drawn as the menus are, the chosen option checked; past a screenful
// of options, its popup searches them; and among many, Base UI's combobox,
// whose field searches them.
import { Combobox } from "@base-ui/react/combobox";
import { Select as Base } from "@base-ui/react/select";
import { useState } from "react";

import { t } from "../i18n.ts";
import { Icon } from "../icon.tsx";
import { usePrefs } from "../prefs.ts";
import { input, popup, popupItem } from "./styles.ts";

/** How many options a select lists without a field searching them. */
const selectSearchFrom = 16;

/**
 * A choice among options, labelled for assistive technology by label; quiet,
 * small and without a fill until pointed at, beside a title. On the
 * television its options open as a menu's do (menu.tsx): below it, on the
 * chosen one, as frosted glass with larger items, and only as it is pressed.
 * Elsewhere, of selectSearchFrom options or more, its popup opens below it
 * with a field searching them.
 */
export function Select<T extends string>({
  value,
  options,
  onChange,
  label,
  disabled,
  quiet = false,
  className,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
  label: string;
  disabled?: boolean;
  quiet?: boolean;
  className?: string;
}) {
  const { tv } = usePrefs();
  const look = quiet
    ? "min-h-7 gap-1 ps-2 pe-1.5 type-footnote desk:min-h-6 text-text-secondary hover:bg-fill data-popup-open:bg-fill"
    : "min-h-9 gap-2 bg-fill ps-2.5 pe-2.5 type-callout desk:min-h-7 desk:gap-1.5 desk:ps-2 desk:pe-1.5 text-on-background hover:bg-fill-selected data-popup-open:bg-fill-selected";
  if (!tv && options.length >= selectSearchFrom) {
    return <SearchSelect value={value} options={options} onChange={onChange} label={label} disabled={disabled} look={look} className={className} />;
  }
  return (
    <Base.Root items={options} value={value} disabled={disabled} onValueChange={(v) => v !== null && onChange(v)}>
      <Base.Trigger
        aria-label={label}
        onKeyDown={tv ? (e) => (e.key === "ArrowDown" || e.key === "ArrowUp") && e.preventBaseUIHandler() : undefined}
        className={`flex max-w-full min-w-0 items-center justify-between rounded-md text-start data-disabled:opacity-50 ${look} ${className ?? ""}`}
      >
        <Base.Value className="min-w-0 truncate" />
        <Base.Icon className="flex shrink-0 text-text-secondary">
          <Icon name="expand" size={14} />
        </Base.Icon>
      </Base.Trigger>
      <Base.Portal>
        <Base.Positioner alignItemWithTrigger={!tv} sideOffset={tv ? 8 : 4} className="z-50 outline-none">
          <Base.Popup
            data-tv-menu={tv || undefined}
            className={`${popup} ${tv ? "bg-surface-overlay/45 backdrop-blur-3xl backdrop-saturate-150" : ""} min-w-[var(--anchor-width)]`}
          >
            <Base.List className="max-h-[min(var(--available-height),24rem)] overflow-y-auto">
              {options.map((o) => (
                <Base.Item key={o.value} value={o.value} className={`${popupItem} grid grid-cols-[1rem_1fr] gap-2 ps-2 ${tv ? "min-h-12 type-headline" : ""}`}>
                  <Base.ItemIndicator className="col-start-1 flex text-accent">
                    <Icon name="check" size={16} />
                  </Base.ItemIndicator>
                  <Base.ItemText className="col-start-2">{o.label}</Base.ItemText>
                </Base.Item>
              ))}
            </Base.List>
          </Base.Popup>
        </Base.Positioner>
      </Base.Portal>
    </Base.Root>
  );
}

/** Whether an option's label holds what was typed. */
function optionMatch(o: { label: string }, query: string): boolean {
  const q = query.trim().toLocaleLowerCase();
  return !q || o.label.toLocaleLowerCase().includes(q);
}

/**
 * A select of many options: Base UI's combobox as a button looking as the
 * select does, its popup below it, at least as wide, with a field searching
 * the options by label above them, the chosen one checked.
 */
function SearchSelect<T extends string>({
  value,
  options,
  onChange,
  label,
  disabled,
  look,
  className,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
  label: string;
  disabled?: boolean;
  look: string;
  className?: string;
}) {
  type Option = { value: T; label: string };
  const [query, setQuery] = useState("");
  const selected = options.find((o) => o.value === value) ?? null;
  return (
    <Combobox.Root
      items={options}
      value={selected}
      disabled={disabled}
      onValueChange={(o: Option | null) => o && onChange(o.value)}
      inputValue={query}
      onInputValueChange={(q, { reason }) => setQuery(["input-change", "input-clear", "input-paste"].includes(reason) ? q : "")}
      onOpenChange={(open) => !open && setQuery("")}
      itemToStringLabel={(o: Option) => o.label}
      isItemEqualToValue={(a: Option, b: Option) => a.value === b.value}
      filter={(o: Option) => optionMatch(o, query)}
      autoHighlight
    >
      <Combobox.Trigger
        aria-label={label}
        className={`flex max-w-full min-w-0 items-center justify-between rounded-md text-start data-disabled:opacity-50 ${look} ${className ?? ""}`}
      >
        <span className="min-w-0 truncate">{selected?.label}</span>
        <Icon name="expand" size={14} className="shrink-0 text-text-secondary" />
      </Combobox.Trigger>
      <Combobox.Portal>
        <Combobox.Positioner sideOffset={4} align="start" className="z-50 outline-none">
          <Combobox.Popup className={`${popup} flex w-[max(var(--anchor-width),16rem)] max-w-[calc(100vw-32px)] flex-col`}>
            <div className="mb-1 px-1 pt-0.5">
              <div className="relative">
                <Icon name="search" size={16} className="pointer-events-none absolute inset-y-0 start-2.5 my-auto text-text-secondary" />
                <Combobox.Input aria-label={label} placeholder={t("nav.search")} className={`${input} ps-8`} />
              </div>
            </div>
            <Combobox.Empty className="px-2.5 py-2 type-callout text-text-secondary empty:hidden">{t("schema.noMatches")}</Combobox.Empty>
            <Combobox.List className="max-h-[min(calc(var(--available-height)-3rem),24rem)] overflow-y-auto">
              {(o: Option) => (
                <Combobox.Item key={o.value} value={o} className={`${popupItem} grid grid-cols-[1rem_1fr] gap-2 ps-2`}>
                  <Combobox.ItemIndicator className="col-start-1 flex text-accent">
                    <Icon name="check" size={16} />
                  </Combobox.ItemIndicator>
                  <span className="col-start-2">{o.label}</span>
                </Combobox.Item>
              )}
            </Combobox.List>
          </Combobox.Popup>
        </Combobox.Positioner>
      </Combobox.Portal>
    </Combobox.Root>
  );
}

/** The tones of a picker's badges. */
export type PickerTone = "neutral" | "success" | "warning" | "danger";

const tones: Record<PickerTone, string> = {
  neutral: "bg-fill text-text-secondary",
  success: "bg-success/15 text-success",
  warning: "bg-warning/15 text-warning",
  danger: "bg-danger/15 text-danger",
};

/** An option of a picker: its label, and what tells it apart. */
export interface PickerOption {
  value: string;
  label: string;
  /** A line under the label, cut to fit. */
  detail?: string;
  /** Quiet text beside the label, such as a language. */
  meta?: string;
  badges?: { label: string; tone?: PickerTone }[];
  /** The keys the filters match. */
  keys?: string[];
}

/** A filter of a picker's options: those with its key. */
export interface PickerFilter {
  key: string;
  label: string;
}

/** How many options a picker lists at once; typing finds the others. */
const pickerLimit = 100;

/** Whether an option holds what was typed, in its label, detail, meta or badges. */
function pickerMatch(o: PickerOption, query: string): boolean {
  const q = query.trim().toLocaleLowerCase();
  return !q || [o.label, o.detail ?? "", o.meta ?? "", ...(o.badges ?? []).map((b) => b.label)].some((s) => s.toLocaleLowerCase().includes(q));
}

/**
 * A choice among many options, found by typing into the field: its popup
 * lists the options holding what was typed, one compact row each, under
 * filters chosen one per group and the count of the options matching.
 * With labels.search, it is a button as a select is, showing the option
 * chosen or the placeholder, and the field searching is in its popup, as
 * wide as the line holding the button.
 */
export function Picker({
  value,
  options,
  onChange,
  label,
  filters = [],
  labels,
  className,
}: {
  value: string | undefined;
  options: PickerOption[];
  onChange: (v: string | undefined) => void;
  label: string;
  /** Groups of filters, one of each group chosen, or none for all. */
  filters?: PickerFilter[][];
  labels: {
    placeholder: string;
    /** The placeholder of the search field in the popup, which makes the picker a button. */
    search?: string;
    empty: string;
    all: string;
    count: (n: number) => string;
    first: (shown: number, n: number) => string;
  };
  className?: string;
}) {
  const { tv } = usePrefs();
  const [open, setOpen] = useState(false);
  // The line holding a button, which its popup spans.
  const [line, setLine] = useState<HTMLElement | null>(null);
  const selected = options.find((o) => o.value === value) ?? null;
  // What is typed while the popup is open; closed, the field shows the option chosen.
  const [query, setQuery] = useState<string | null>(null);
  const [chosen, setChosen] = useState<(string | undefined)[]>([]);
  const items = options.filter((o) => chosen.every((k) => !k || o.keys?.includes(k)));
  const searched = query ?? "";
  const count = items.filter((o) => pickerMatch(o, searched)).length;
  const choose = (group: number, key: string | undefined) => setChosen((c) => Object.assign([...c], { [group]: key }));
  const chip = (on: boolean) =>
    `min-h-7 rounded-full px-2.5 type-caption desk:min-h-6 desk:px-2 font-medium ${on ? "bg-fill-selected text-on-background" : "text-text-secondary hover:bg-fill"}`;
  return (
    <Combobox.Root
      items={items}
      value={selected}
      onValueChange={(o: PickerOption | null) => onChange(o?.value)}
      inputValue={query ?? (labels.search === undefined ? (selected?.label ?? "") : "")}
      onInputValueChange={(q, { reason }) => setQuery(["input-change", "input-clear", "input-paste"].includes(reason) ? q : null)}
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (!o) setQuery(null);
      }}
      itemToStringLabel={(o: PickerOption) => o.label}
      isItemEqualToValue={(a: PickerOption, b: PickerOption) => a.value === b.value}
      filter={(o: PickerOption) => pickerMatch(o, searched)}
      limit={pickerLimit}
      autoHighlight
    >
      {labels.search !== undefined ? (
        <Combobox.Trigger
          ref={(el: HTMLElement | null) => setLine(el?.parentElement ?? null)}
          aria-label={label}
          className={`flex min-h-9 max-w-full min-w-0 items-center justify-between gap-2 rounded-md bg-fill ps-2.5 pe-2.5 text-start type-callout text-on-background hover:bg-fill-selected data-popup-open:bg-fill-selected desk:min-h-7 desk:gap-1.5 desk:ps-2 desk:pe-1.5 ${className ?? ""}`}
        >
          <span className={`min-w-0 truncate ${selected ? "" : "text-text-secondary"}`}>{selected?.label ?? labels.placeholder}</span>
          <Icon name="expand" size={14} className="shrink-0 text-text-secondary" />
        </Combobox.Trigger>
      ) : (
      <div className={`relative ${className ?? ""}`}>
        {/* On the television OK opens its options, as a select's, and up
            and down move focus on while they are closed. */}
        <Combobox.Input
          aria-label={label}
          placeholder={labels.placeholder}
          data-tv-list={tv || undefined}
          onKeyDown={
            tv
              ? (e) => {
                  if (open) return;
                  if (e.key === "ArrowDown" || e.key === "ArrowUp") e.preventBaseUIHandler();
                  else if (e.key === "Enter") {
                    e.preventDefault();
                    setOpen(true);
                  }
                }
              : undefined
          }
          className={`${input} pe-9`}
        />
        <Combobox.Trigger aria-label={label} className="absolute inset-y-0 end-0 flex w-9 items-center justify-center text-text-secondary">
          <Icon name="expand" size={14} />
        </Combobox.Trigger>
      </div>
      )}
      <Combobox.Portal>
        <Combobox.Positioner anchor={labels.search === undefined ? undefined : line} sideOffset={4} align="start" className="z-50 outline-none">
          <Combobox.Popup
            className={`${popup} flex flex-col ${labels.search === undefined ? "w-[min(36rem,calc(100vw-32px))]" : "w-[var(--anchor-width)] max-w-[calc(100vw-32px)]"}`}
          >
            {labels.search !== undefined && (
              <div className="mb-1 px-1 pt-0.5">
                <div className="relative">
                  <Icon name="search" size={16} className="pointer-events-none absolute inset-y-0 start-2.5 my-auto text-text-secondary" />
                  <Combobox.Input aria-label={label} placeholder={labels.search} className={`${input} ps-8`} />
                </div>
              </div>
            )}
            {/* A button's popup counts its options only when filtered or cut short. */}
            {(labels.search === undefined || filters.length > 0 || count > pickerLimit) && (
            <div className="mb-1 flex flex-wrap items-center gap-x-3 gap-y-1 border-b border-border px-1 pb-1.5">
              {filters.map((group, g) => (
                <div key={g} className="flex items-center gap-0.5">
                  {[undefined, ...group.map((f) => f.key)].map((key) => (
                    // A press keeps the focus in the field, the popup open.
                    <button key={key ?? ""} type="button" className={chip(chosen[g] === key)} onMouseDown={(e) => e.preventDefault()} onClick={() => choose(g, key)}>
                      {key === undefined ? labels.all : group.find((f) => f.key === key)?.label}
                    </button>
                  ))}
                </div>
              ))}
              <span className="ms-auto px-1 type-caption text-text-tertiary">
                {count > pickerLimit ? labels.first(pickerLimit, count) : labels.count(count)}
              </span>
            </div>
            )}
            <Combobox.Empty className="px-2.5 py-2 type-callout text-text-secondary empty:hidden">{labels.empty}</Combobox.Empty>
            <Combobox.List className="max-h-[min(calc(var(--available-height)-3rem),26rem)] overflow-y-auto">
              {(o: PickerOption) => (
                <Combobox.Item
                  key={o.value}
                  value={o}
                  className="grid w-full cursor-default grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 rounded-lg px-2.5 py-1.5 text-start outline-none select-none data-highlighted:bg-fill data-selected:bg-fill-selected"
                >
                  <span className="truncate type-callout font-medium">{o.label}</span>
                  <span className="flex items-center gap-1.5">
                    {o.meta && <span className="type-caption text-text-tertiary">{o.meta}</span>}
                    {o.badges?.map((b) => (
                      <span key={b.label} className={`rounded-[5px] px-1.5 type-caption font-medium ${tones[b.tone ?? "neutral"]}`}>
                        {b.label}
                      </span>
                    ))}
                  </span>
                  {o.detail && <span className="col-span-2 truncate type-caption text-text-secondary">{o.detail}</span>}
                </Combobox.Item>
              )}
            </Combobox.List>
          </Combobox.Popup>
        </Combobox.Positioner>
      </Combobox.Portal>
    </Combobox.Root>
  );
}

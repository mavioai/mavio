// A choice of an AI model (docs/clients.md §6.4): a button as a select is,
// its popup searching the models, which can number in the thousands, as a
// virtual list (TanStack Virtual) of rows of the provider's logo, the
// model's name and, smaller and quieter, the provider's.
import { Combobox } from "@base-ui/react/combobox";
import { useVirtualizer, type Virtualizer } from "@tanstack/react-virtual";
import { useRef, useState } from "react";

import { Icon } from "../icon.tsx";
import { ProviderLogo } from "../provider-logo.tsx";
import { input, popup } from "./styles.ts";

/** A model to choose: its value, its name, its details and its provider's ID and name; no provider for a choice of none. */
export interface ModelOption {
  value: string;
  label: string;
  /** A line of details under the name, in the list. */
  detail?: string;
  provider?: string;
  providerName?: string;
}

/** The height of a row of the list: one line, or two with details. */
const rowHeight = (o: ModelOption | undefined) => (o?.detail ? 46 : 32);

/** Whether a model's name, value or provider holds what was typed. */
function matches(o: ModelOption, query: string): boolean {
  const q = query.trim().toLocaleLowerCase();
  return !q || [o.label, o.value, o.providerName ?? ""].some((s) => s.toLocaleLowerCase().includes(q));
}

/** A model as the list's rows show it first: its provider's logo, its name and, smaller and quieter, its provider's name. */
export function ModelLabel({ option }: { option: ModelOption }) {
  return (
    <>
      {option.provider && <ProviderLogo provider={option.provider} />}
      <span className="min-w-0 truncate">{option.label}</span>
      {option.providerName && <span className="shrink-0 truncate text-[11px] text-text-tertiary opacity-75">{option.providerName}</span>}
    </>
  );
}

/**
 * A choice among options, labelled for assistive technology by label: a
 * button showing the model chosen, whose popup, as wide as the line holding
 * it, searches them.
 */
export function ModelPicker({
  value,
  options,
  onChange,
  label,
  disabled,
  labels,
  className,
}: {
  value: string;
  options: ModelOption[];
  onChange: (value: string) => void;
  label: string;
  disabled?: boolean;
  labels: { placeholder: string; search: string; empty: string };
  className?: string;
}) {
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState("");
  // The line holding the button, which the popup spans.
  const [line, setLine] = useState<HTMLElement | null>(null);
  const list = useRef<Virtualizer<HTMLDivElement, Element> | null>(null);
  const selected = options.find((o) => o.value === value) ?? null;
  return (
    <Combobox.Root
      virtualized
      items={options}
      value={selected}
      disabled={disabled}
      onValueChange={(o: ModelOption | null) => o && onChange(o.value)}
      inputValue={query}
      onInputValueChange={(q) => setQuery(q)}
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        if (!o) setQuery("");
      }}
      itemToStringLabel={(o: ModelOption) => o.label}
      isItemEqualToValue={(a: ModelOption, b: ModelOption) => a.value === b.value}
      filter={(o: ModelOption) => matches(o, query)}
      onItemHighlighted={(o, { reason, index }) => {
        if (o && reason === "keyboard" && index >= 0) list.current?.scrollToIndex(index, { align: "auto" });
      }}
      autoHighlight
    >
      <Combobox.Trigger
        ref={(el: HTMLElement | null) => setLine(el?.parentElement ?? null)}
        aria-label={label}
        className={`flex min-h-9 max-w-full min-w-0 items-center gap-2 rounded-md bg-fill ps-2.5 pe-2.5 text-start type-callout text-on-background hover:bg-fill-selected data-disabled:opacity-50 data-popup-open:bg-fill-selected desk:min-h-7 desk:gap-1.5 desk:ps-2 desk:pe-1.5 ${className ?? ""}`}
      >
        {selected ? <ModelLabel option={selected} /> : <span className="min-w-0 truncate text-text-secondary">{labels.placeholder}</span>}
        <Icon name="expand" size={14} className="ms-auto shrink-0 text-text-secondary" />
      </Combobox.Trigger>
      <Combobox.Portal>
        <Combobox.Positioner anchor={line} sideOffset={4} align="start" className="z-50 outline-none">
          <Combobox.Popup className={`${popup} flex w-[var(--anchor-width)] max-w-[calc(100vw-32px)] flex-col`}>
            <div className="relative mb-1 px-1 pt-0.5">
              <Icon name="search" size={16} className="pointer-events-none absolute inset-y-0 start-3.5 my-auto text-text-secondary" />
              <Combobox.Input aria-label={label} placeholder={labels.search} className={`${input} ps-8 desk:ps-8`} />
            </div>
            <Combobox.Empty className="px-2.5 py-2 type-callout text-text-secondary empty:hidden">{labels.empty}</Combobox.Empty>
            <Rows list={list} />
          </Combobox.Popup>
        </Combobox.Positioner>
      </Combobox.Portal>
    </Combobox.Root>
  );
}

/** The options matching what was typed, only those scrolled into view drawn. */
function Rows({ list }: { list: React.RefObject<Virtualizer<HTMLDivElement, Element> | null> }) {
  const items = Combobox.useFilteredItems<ModelOption>();
  const scroller = useRef<HTMLDivElement | null>(null);
  const virtualizer = useVirtualizer({
    count: items.length,
    getScrollElement: () => scroller.current,
    estimateSize: (i) => rowHeight(items[i]),
    overscan: 10,
  });
  list.current = virtualizer;
  if (items.length === 0) return null;
  return (
    <Combobox.List className="outline-none">
      <div role="presentation" ref={scroller} className="max-h-[min(calc(var(--available-height)-3.5rem),24rem)] overflow-y-auto">
        <div role="presentation" className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
          {virtualizer.getVirtualItems().map((row) => {
            const o = items[row.index];
            if (!o) return null;
            return (
              <Combobox.Item
                key={row.key}
                index={row.index}
                value={o}
                aria-setsize={items.length}
                aria-posinset={row.index + 1}
                className="absolute start-0 top-0 flex w-full cursor-default flex-col justify-center rounded-lg px-2.5 text-start type-callout outline-none select-none data-highlighted:bg-fill data-selected:bg-fill-selected"
                style={{ height: row.size, transform: `translateY(${row.start}px)` }}
              >
                <span className="flex min-w-0 items-center gap-2">
                  <ModelLabel option={o} />
                </span>
                {o.detail && <span className="truncate ps-6 type-caption font-normal text-text-tertiary">{o.detail}</span>}
              </Combobox.Item>
            );
          })}
        </div>
      </div>
    </Combobox.List>
  );
}

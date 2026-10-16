// A choice among options (docs/clients.md §6.4): Base UI's select, its
// popup drawn as the menus are, the chosen option checked.
import { Select as Base } from "@base-ui/react/select";

import { Icon } from "../icon.tsx";
import { popup, popupItem } from "./styles.ts";

/** A choice among options, labelled for assistive technology by label. */
export function Select<T extends string>({
  value,
  options,
  onChange,
  label,
  disabled,
  className,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
  label: string;
  disabled?: boolean;
  className?: string;
}) {
  return (
    <Base.Root items={options} value={value} disabled={disabled} onValueChange={(v) => v !== null && onChange(v)}>
      <Base.Trigger
        aria-label={label}
        className={`flex min-h-9 max-w-full min-w-0 items-center justify-between gap-2 rounded-md bg-fill ps-2.5 pe-2.5 text-start type-callout text-on-background hover:bg-fill-selected data-disabled:opacity-50 data-popup-open:bg-fill-selected ${className ?? ""}`}
      >
        <Base.Value className="min-w-0 truncate" />
        <Base.Icon className="flex shrink-0 text-text-secondary">
          <Icon name="expand" size={14} />
        </Base.Icon>
      </Base.Trigger>
      <Base.Portal>
        <Base.Positioner sideOffset={4} className="z-50 outline-none">
          <Base.Popup className={`${popup} min-w-[var(--anchor-width)]`}>
            <Base.List className="max-h-[min(var(--available-height),24rem)] overflow-y-auto">
              {options.map((o) => (
                <Base.Item key={o.value} value={o.value} className={`${popupItem} grid grid-cols-[1rem_1fr] gap-2 ps-2`}>
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

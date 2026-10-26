// Menus (docs/clients.md §6.4): Base UI's menu opened by a button, its
// items moved through with the arrow keys and typed to, closed by Escape
// or a press outside; on the television, frosted glass with larger
// items, opening on the item chosen.
import { Menu as Base } from "@base-ui/react/menu";
import { Link, type LinkProps } from "@tanstack/react-router";
import { type KeyboardEvent, type ReactElement, type ReactNode, type RefObject, useRef } from "react";

import { Icon, type IconName } from "../icon.tsx";
import { usePrefs } from "../prefs.ts";
import { popup } from "./styles.ts";

/**
 * A menu opened by trigger, a button, placed on side of it and aligned
 * with its start, center or end; width is the popup's width class. It opens in
 * container where given, as an element in full screen, else the page.
 */
export function Menu({
  trigger,
  side = "bottom",
  align = "start",
  width = "w-64",
  label,
  open,
  onOpenChange,
  start = "[aria-checked=true]",
  container,
  children,
}: {
  trigger: ReactElement;
  side?: "top" | "bottom";
  align?: "start" | "center" | "end";
  width?: string;
  label?: string;
  open?: boolean;
  onOpenChange?: (open: boolean) => void;
  /** The item a menu opens on on the television: its chosen one, else its first. */
  start?: string;
  container?: RefObject<HTMLElement | null>;
  children: ReactNode;
}) {
  const { tv } = usePrefs();
  const ref = useRef<HTMLDivElement>(null);
  // On the television the menu opens on its start: focus first entering
  // it, on whichever item Base UI chose, moves there before it is drawn.
  const started = useRef(false);
  const toStart = () => {
    if (!tv || started.current) return;
    started.current = true;
    const el = ref.current?.querySelector<HTMLElement>(start);
    if (el && document.activeElement !== el) el.focus();
  };
  return (
    <Base.Root
      open={open}
      onOpenChange={(o) => {
        started.current = false;
        onOpenChange?.(o);
      }}
      onOpenChangeComplete={(o) => o && toStart()}
    >
      {/* On the television a menu opens as its button is pressed alone:
          the arrows move focus on, as they do from any button. */}
      <Base.Trigger
        render={trigger}
        onKeyDown={tv ? (e) => (e.key === "ArrowDown" || e.key === "ArrowUp") && e.preventBaseUIHandler() : undefined}
      />
      <Base.Portal container={container}>
        <Base.Positioner side={side} align={align} sideOffset={8} className="z-50 outline-none">
          <Base.Popup
            ref={ref}
            onFocus={toStart}
            aria-label={label}
            data-tv-menu={tv || undefined}
            className={`${popup} ${tv ? "bg-surface-overlay/45 backdrop-blur-3xl backdrop-saturate-150" : ""} ${width} max-h-[var(--available-height)] overflow-y-auto`}
          >
            {children}
          </Base.Popup>
        </Base.Positioner>
      </Base.Portal>
    </Base.Root>
  );
}

const item =
  "flex w-full cursor-default items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-start outline-none select-none data-disabled:opacity-50 data-highlighted:bg-fill";

/** An item's class, larger on the television. */
function useItem() {
  const { tv } = usePrefs();
  return `${item} ${tv ? "min-h-12 type-headline" : "min-h-10 type-body desk:min-h-7 desk:gap-2 desk:rounded-md desk:px-2 desk:py-1"}`;
}

/** The parts of an item: its icon, its text, and the keys of its shortcut. */
function Parts({ icon, shortcut, end, children }: { icon?: IconName; shortcut?: string; end?: ReactNode; children: ReactNode }) {
  return (
    <>
      {icon && <Icon name={icon} size={18} className="shrink-0 text-text-secondary" />}
      <span className="min-w-0 flex-1">{children}</span>
      {shortcut && <kbd className="font-sans type-footnote text-text-tertiary">{shortcut}</kbd>}
      {end}
    </>
  );
}

/** An item doing something; a destructive one is red. */
export function MenuItem({
  start,
  icon,
  shortcut,
  end,
  danger = false,
  disabled,
  closeOnClick,
  onClick,
  children,
}: {
  icon?: IconName;
  shortcut?: string;
  end?: ReactNode;
  danger?: boolean;
  disabled?: boolean;
  closeOnClick?: boolean;
  /** Marks the item a menu opens on on the television. */
  start?: boolean;
  onClick: () => void;
  children: ReactNode;
}) {
  const item = useItem();
  return (
    <Base.Item data-menu-start={start || undefined} disabled={disabled} closeOnClick={closeOnClick} onClick={onClick} className={`${item} ${danger ? "text-danger" : ""}`}>
      <Parts icon={icon} shortcut={shortcut} end={end}>
        {children}
      </Parts>
    </Base.Item>
  );
}

/** An item opening a page of the web UI. */
export function MenuLink({
  start,
  icon,
  shortcut,
  end,
  children,
  ...link
}: Omit<LinkProps, "children" | "className"> & { start?: boolean; icon?: IconName; shortcut?: string; end?: ReactNode; children: ReactNode }) {
  const item = useItem();
  return (
    <Base.LinkItem data-menu-start={start || undefined} closeOnClick render={<Link {...link} />} className={item}>
      <Parts icon={icon} shortcut={shortcut} end={end}>
        {children}
      </Parts>
    </Base.LinkItem>
  );
}

/** An item opening a page elsewhere, in a new tab. */
export function MenuExternal({ icon, href, children }: { icon?: IconName; href: string; children: ReactNode }) {
  const item = useItem();
  return (
    <Base.LinkItem closeOnClick href={href} target="_blank" rel="noreferrer" className={item}>
      <Parts icon={icon}>{children}</Parts>
    </Base.LinkItem>
  );
}

/**
 * Items of which one is chosen, each checked when it is. Items side by
 * side (horizontal), as MenuIconOption's, are moved through with Left and
 * Right, which the menu's own keys, Up and Down, leave alone.
 */
export function MenuChoice<T extends string>({
  value,
  onChange,
  label,
  horizontal = false,
  className,
  children,
}: {
  value: T;
  onChange: (v: T) => void;
  label?: string;
  horizontal?: boolean;
  className?: string;
  children: ReactNode;
}) {
  const sideways = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== "ArrowLeft" && e.key !== "ArrowRight") return;
    const items = [...e.currentTarget.querySelectorAll<HTMLElement>("[role=menuitemradio]")];
    const i = items.indexOf(document.activeElement as HTMLElement);
    if (i < 0) return;
    const rtl = getComputedStyle(e.currentTarget).direction === "rtl";
    const next = items[i + ((e.key === "ArrowRight") !== rtl ? 1 : -1)];
    e.preventDefault();
    e.stopPropagation();
    next?.focus();
  };
  return (
    <Base.RadioGroup
      value={value}
      onValueChange={(v: T) => onChange(v)}
      aria-label={label}
      onKeyDown={horizontal ? sideways : undefined}
      className={className}
    >
      {children}
    </Base.RadioGroup>
  );
}

/** An item of a choice. */
export function MenuOption({ value, children }: { value: string; children: ReactNode }) {
  const item = useItem();
  return (
    <Base.RadioItem value={value} closeOnClick className={item}>
      <span className="flex w-4 shrink-0 text-accent-text">
        <Base.RadioItemIndicator>
          <Icon name="check" size={16} />
        </Base.RadioItemIndicator>
      </span>
      <span className="min-w-0 flex-1">{children}</span>
    </Base.RadioItem>
  );
}

/** An item of a choice drawn as an icon, beside the others as segments are. */
export function MenuIconOption({ value, icon, label }: { value: string; icon: IconName; label: string }) {
  const { tv } = usePrefs();
  return (
    <Base.RadioItem
      value={value}
      closeOnClick={false}
      aria-label={label}
      className={`flex items-center justify-center rounded-full text-text-secondary outline-none data-checked:bg-surface-raised data-checked:text-on-background data-checked:shadow-level1 data-highlighted:text-on-background data-highlighted:ring-2 data-highlighted:ring-focus data-highlighted:ring-inset ${tv ? "size-9" : "size-7"}`}
    >
      <Icon name={icon} size={tv ? 18 : 16} />
    </Base.RadioItem>
  );
}

/** A heading over the items after it. */
export function MenuLabel({ children }: { children: ReactNode }) {
  return <div className="px-2.5 pt-1.5 pb-1 type-caption text-text-secondary">{children}</div>;
}

/** A line between groups of items. */
export function MenuSeparator() {
  return <Base.Separator className="mx-2.5 my-1 h-px bg-border" />;
}

// The form controls (docs/clients.md §6.4) on Base UI: switches,
// checkboxes, sliders, number fields, a field with suggestions, choices
// drawn as segments or chips, tabs, and progress bars; each drawn the same
// in every browser.
import { Autocomplete } from "@base-ui/react/autocomplete";
import { Checkbox as BaseCheckbox } from "@base-ui/react/checkbox";
import { Collapsible } from "@base-ui/react/collapsible";
import { NumberField as BaseNumberField } from "@base-ui/react/number-field";
import { Progress as BaseProgress } from "@base-ui/react/progress";
import { Radio } from "@base-ui/react/radio";
import { RadioGroup } from "@base-ui/react/radio-group";
import { Slider as BaseSlider } from "@base-ui/react/slider";
import { Switch as BaseSwitch } from "@base-ui/react/switch";
import { Tabs as BaseTabs } from "@base-ui/react/tabs";
import { Toggle } from "@base-ui/react/toggle";
import type { HTMLAttributes, ReactNode, Ref } from "react";

import { Icon, type IconName } from "../icon.tsx";
import { locale } from "../i18n.ts";
import { popup, popupItem } from "./styles.ts";

/** A switch, green when on as Apple's are. */
export function Switch({ on, onChange, label, disabled }: { on: boolean; onChange: (on: boolean) => void; label: string; disabled?: boolean }) {
  return (
    <BaseSwitch.Root
      checked={on}
      disabled={disabled}
      aria-label={label}
      onCheckedChange={onChange}
      className="relative h-[31px] w-[51px] shrink-0 rounded-full bg-fill-selected transition-colors data-checked:bg-[#34C759] data-disabled:opacity-50"
    >
      <BaseSwitch.Thumb className="absolute top-[2px] left-[2px] size-[27px] rounded-full bg-white shadow-level1 transition-transform data-checked:translate-x-5" />
    </BaseSwitch.Root>
  );
}

/** A checkbox, with its label beside it when it has children; className lays out the label. */
export function Checkbox({
  checked,
  onChange,
  label,
  disabled,
  className,
  children,
}: {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label?: string;
  disabled?: boolean;
  className?: string;
  children?: ReactNode;
}) {
  const box = (
    <BaseCheckbox.Root
      checked={checked}
      disabled={disabled}
      aria-label={children ? undefined : label}
      onCheckedChange={onChange}
      className="flex size-5 shrink-0 items-center justify-center rounded-[6px] border-2 border-text-tertiary data-checked:border-accent data-checked:bg-accent data-disabled:opacity-50"
    >
      <BaseCheckbox.Indicator className="flex text-on-accent">
        <Icon name="check" size={14} />
      </BaseCheckbox.Indicator>
    </BaseCheckbox.Root>
  );
  if (!children) return box;
  return (
    <label className={`flex cursor-pointer items-center gap-2.5 type-body ${disabled ? "cursor-default opacity-50" : ""} ${className ?? ""}`}>
      {box}
      <span className="flex min-w-0 flex-1 items-center gap-3">{children}</span>
    </label>
  );
}

/**
 * A slider over a range; onChange follows the thumb, onCommit is called
 * once it is let go.
 */
export function Slider({
  value,
  min = 0,
  max,
  step = 1,
  label,
  valueText,
  onChange,
  onCommit,
  light = false,
  className,
}: {
  value: number;
  min?: number;
  max: number;
  step?: number;
  label: string;
  valueText?: (v: number) => string;
  onChange?: (v: number) => void;
  onCommit?: (v: number) => void;
  /** White over video, as the player's timeline is: a slim bar thickening under the pointer, its thumb shown then. */
  light?: boolean;
  className?: string;
}) {
  return (
    <BaseSlider.Root
      value={value}
      min={min}
      max={max}
      step={step}
      onValueChange={(v) => onChange?.(v)}
      onValueCommitted={(v) => onCommit?.(v)}
      className={`flex min-w-0 touch-none items-center ${className ?? ""}`}
    >
      <BaseSlider.Control className={`group flex w-full cursor-pointer items-center ${light ? "h-5" : "h-6"}`}>
        <BaseSlider.Track className={light ? "h-1 w-full rounded-full bg-white/30 transition-[height] group-hover:h-1.5" : "h-1.5 w-full rounded-full bg-fill-selected"}>
          <BaseSlider.Indicator className={`rounded-full ${light ? "bg-white" : "bg-accent"}`} />
          <BaseSlider.Thumb
            aria-label={label}
            getAriaValueText={valueText ? (_, v) => valueText(v) : undefined}
            className={`rounded-full bg-white shadow-level1 outline-none focus-visible:ring-2 focus-visible:ring-focus ${
              light ? "size-3.5 opacity-0 transition-opacity group-hover:opacity-100 focus-visible:opacity-100 data-dragging:opacity-100" : "size-5"
            }`}
          />
        </BaseSlider.Track>
      </BaseSlider.Control>
    </BaseSlider.Root>
  );
}

/**
 * A field of a number between min and max, whole unless integer is false,
 * with buttons to step it; empty is undefined.
 */
export function NumberField({
  value,
  onChange,
  min,
  max,
  step = 1,
  integer = true,
  label,
  placeholder,
  disabled,
  className,
}: {
  value: number | undefined;
  onChange: (v: number | undefined) => void;
  min?: number;
  max?: number;
  step?: number;
  integer?: boolean;
  label: string;
  placeholder?: string;
  disabled?: boolean;
  className?: string;
}) {
  const button =
    "flex w-8 shrink-0 items-center justify-center text-text-secondary hover:bg-fill hover:text-on-background data-disabled:opacity-40 data-disabled:hover:bg-transparent";
  return (
    <BaseNumberField.Root
      value={value ?? null}
      min={min}
      max={max}
      step={step}
      disabled={disabled}
      locale={locale}
      format={{ useGrouping: false, maximumFractionDigits: integer ? 0 : 6 }}
      onValueChange={(v) => onChange(v ?? undefined)}
      className={`min-w-0 ${className ?? ""}`}
    >
      <BaseNumberField.Group className="flex min-h-9 w-full overflow-hidden rounded-md border border-border bg-surface focus-within:border-focus data-disabled:opacity-50">
        <BaseNumberField.Decrement aria-label={`${label} −`} className={button}>
          <span aria-hidden className="type-body">−</span>
        </BaseNumberField.Decrement>
        <BaseNumberField.Input
          aria-label={label}
          placeholder={placeholder}
          className="w-full min-w-0 bg-transparent px-1 text-center type-body text-on-background tabular-nums outline-none placeholder:text-text-tertiary"
        />
        <BaseNumberField.Increment aria-label={`${label} +`} className={button}>
          <Icon name="add" size={14} />
        </BaseNumberField.Increment>
      </BaseNumberField.Group>
    </BaseNumberField.Root>
  );
}

/**
 * A text field suggesting values as it is typed, which it filters by what
 * is typed; any other value may be typed too.
 */
export function SuggestField({
  value,
  onChange,
  suggestions,
  label,
  placeholder,
  className,
}: {
  value: string;
  onChange: (v: string) => void;
  suggestions: { value: string; label?: string }[];
  label: string;
  placeholder?: string;
  className: string;
}) {
  return (
    <Autocomplete.Root items={suggestions} value={value} onValueChange={onChange} itemToStringValue={(s) => s.value}>
      <Autocomplete.Input aria-label={label} placeholder={placeholder} className={className} />
      <Autocomplete.Portal>
        <Autocomplete.Positioner sideOffset={4} align="start" className="z-50 outline-none">
          <Autocomplete.Popup className={`${popup} max-h-[min(var(--available-height),20rem)] w-[var(--anchor-width)] overflow-y-auto data-empty:hidden`}>
            <Autocomplete.List>
              {(s: { value: string; label?: string }) => (
                <Autocomplete.Item key={s.value} value={s} className={`${popupItem} flex-col items-start justify-center gap-0`}>
                  <span className="type-body">{s.value}</span>
                  {s.label && <span className="type-caption text-text-secondary">{s.label}</span>}
                </Autocomplete.Item>
              )}
            </Autocomplete.List>
          </Autocomplete.Popup>
        </Autocomplete.Positioner>
      </Autocomplete.Portal>
    </Autocomplete.Root>
  );
}

/**
 * One of a few options in a row of segments, as iOS draws them; an option
 * with an icon and no text is labelled by its label.
 */
export function Segmented<T extends string>({
  value,
  options,
  onChange,
  label,
  size = "medium",
  className,
}: {
  value: T;
  options: { value: T; label: string; icon?: IconName; iconOnly?: boolean }[];
  onChange: (v: T) => void;
  label: string;
  size?: "small" | "medium";
  className?: string;
}) {
  return (
    <RadioGroup
      value={value}
      onValueChange={(v) => onChange(v as T)}
      aria-label={label}
      data-focus-choice
      className={`flex shrink-0 rounded-full bg-fill p-0.5 ${className ?? ""}`}
    >
      {options.map((o) => (
        <Radio.Root
          key={o.value}
          value={o.value}
          aria-label={o.iconOnly ? o.label : undefined}
          className={`flex items-center justify-center gap-1.5 rounded-full text-text-secondary hover:text-on-background data-checked:bg-surface-raised data-checked:text-on-background data-checked:shadow-level1 ${
            o.iconOnly ? (size === "small" ? "size-7" : "size-9") : size === "small" ? "h-7 px-3 type-footnote font-semibold" : "h-9 px-4 type-callout font-semibold"
          }`}
        >
          {o.icon && <Icon name={o.icon} fill={o.value === value} size={size === "small" ? 16 : 18} />}
          {!o.iconOnly && o.label}
        </Radio.Root>
      ))}
    </RadioGroup>
  );
}

/** One of several options as chips, the chosen one filled. */
export function Chips<T extends string>({
  value,
  options,
  onChange,
  label,
  className,
}: {
  value: T;
  options: { value: T; label: string }[];
  onChange: (v: T) => void;
  label: string;
  className?: string;
}) {
  return (
    <RadioGroup value={value} onValueChange={(v) => onChange(v as T)} aria-label={label} data-focus-choice className={`flex flex-wrap gap-2 ${className ?? ""}`}>
      {options.map((o) => (
        <Radio.Root key={o.value} value={o.value} className={pill}>
          {o.label}
        </Radio.Root>
      ))}
    </RadioGroup>
  );
}

/** One of several options each drawn by draw, as pictures of themselves are. */
export function Pictures<T extends string>({
  value,
  options,
  onChange,
  label,
  draw,
  className,
}: {
  value: T;
  options: T[];
  onChange: (v: T) => void;
  label: string;
  draw: (option: T, on: boolean) => ReactNode;
  className?: string;
}) {
  return (
    <RadioGroup value={value} onValueChange={(v) => onChange(v as T)} aria-label={label} data-focus-choice className={className}>
      {options.map((o) => (
        <Radio.Root key={o} value={o} className="flex flex-col items-center gap-2 rounded-[12px] outline-none focus-visible:ring-2 focus-visible:ring-focus">
          {draw(o, o === value)}
        </Radio.Root>
      ))}
    </RadioGroup>
  );
}

/** A chosen one of several, filled as a button is. */
const pill =
  "min-h-9 rounded-full bg-fill px-4 type-callout font-semibold hover:bg-fill-selected data-active:bg-button data-active:text-on-button data-checked:bg-button data-checked:text-on-button";

const chip =
  "inline-flex min-h-8 items-center gap-1.5 rounded-full bg-fill px-3 type-footnote font-semibold hover:bg-fill-selected data-checked:bg-accent data-checked:text-on-accent data-pressed:bg-accent data-pressed:text-on-accent";

/** A chip turned on and off, as a filter's value; with its logo when it has one. */
export function ToggleChip({ on, onChange, label, image }: { on: boolean; onChange: (on: boolean) => void; label: string; image?: string }) {
  return (
    <Toggle pressed={on} onPressedChange={onChange} className={chip}>
      {image && <img src={image} alt="" className="size-5 rounded-[5px]" loading="lazy" />}
      {label}
    </Toggle>
  );
}

/**
 * A button turned on and off, such as Favorite; its look is the caller's,
 * data-pressed when on. Other props go on to the button, so that a
 * tooltip can name it.
 */
export function ToggleButton({
  on,
  onChange,
  label,
  className,
  children,
  ...props
}: {
  on: boolean;
  onChange: (on: boolean) => void;
  label?: string;
  className: string;
  children: ReactNode;
} & Omit<HTMLAttributes<HTMLButtonElement>, "onChange" | "className" | "children"> & { ref?: Ref<HTMLButtonElement> }) {
  return (
    <Toggle {...props} pressed={on} onPressedChange={onChange} aria-label={label} className={className}>
      {children}
    </Toggle>
  );
}

/**
 * Tabs choosing what the page below them shows; the panels are the
 * caller's. Focus moving onto a tab chooses it, as on the television.
 */
export function Tabs<T extends string>({
  value,
  tabs,
  onChange,
  label,
  look = "chips",
  className,
}: {
  value: T;
  tabs: { value: T; label: ReactNode }[];
  onChange: (v: T) => void;
  label: string;
  look?: "chips" | "segments" | "dark" | "pills";
  className?: string;
}) {
  const list = {
    chips: "flex flex-wrap gap-2",
    segments: "flex w-fit rounded-[9px] bg-fill p-0.5",
    dark: "flex rounded-[9px] bg-white/10 p-0.5",
    pills: "flex gap-1",
  }[look];
  const tab = {
    chips: pill,
    segments:
      "flex-1 rounded-[7px] px-4 py-1 type-callout font-semibold whitespace-nowrap text-text-secondary hover:text-on-background data-active:bg-surface-overlay data-active:text-on-background data-active:shadow-level1",
    pills:
      "tv-pill shrink-0 rounded-full px-4 py-1.5 type-callout font-semibold text-text-secondary hover:bg-fill data-active:bg-fill-selected data-active:text-on-background",
    dark: "flex-1 rounded-[7px] px-2 py-1 type-footnote font-semibold whitespace-nowrap text-white/70 hover:text-white data-active:bg-white/25 data-active:text-white",
  }[look];
  return (
    <BaseTabs.Root value={value} onValueChange={(v) => onChange(v as T)}>
      <BaseTabs.List aria-label={label} data-focus-choice activateOnFocus loopFocus={false} className={`${list} ${className ?? ""}`}>
        {tabs.map((tb) => (
          <BaseTabs.Tab key={tb.value} value={tb.value} className={tab}>
            {tb.label}
          </BaseTabs.Tab>
        ))}
      </BaseTabs.List>
    </BaseTabs.Root>
  );
}

/** A bar showing how far something has gone, from 0 to 1. */
export function Progress({ value, label, className }: { value: number; label: string; className?: string }) {
  return (
    <BaseProgress.Root value={Math.round(value * 100)} aria-label={label} className={className}>
      <BaseProgress.Track className="h-1.5 overflow-hidden rounded-full bg-fill">
        <BaseProgress.Indicator className="h-full rounded-full bg-accent transition-[width]" />
      </BaseProgress.Track>
    </BaseProgress.Root>
  );
}

/** Text shown only once label, a link, is pressed, as a long help's rest is. */
export function More({ label, children }: { label: string; children: ReactNode }) {
  return (
    <Collapsible.Root>
      <Collapsible.Trigger className="group inline-flex items-center gap-1 text-accent-text">
        {label}
        <Icon name="expand" size={12} className="transition-transform group-data-panel-open:rotate-180" />
      </Collapsible.Trigger>
      <Collapsible.Panel>{children}</Collapsible.Panel>
    </Collapsible.Root>
  );
}

/** A button choosing a file, the name of the one chosen beside it. */
export function FileField({ label, accept, file, onChange }: { label: string; accept: string; file?: File; onChange: (file?: File) => void }) {
  return (
    <div className="flex min-w-0 items-center gap-3">
      <label className="inline-flex min-h-9 shrink-0 cursor-pointer items-center rounded-[8px] bg-fill px-3 type-callout font-semibold hover:bg-fill-selected has-focus-visible:outline-2 has-focus-visible:outline-focus">
        {label}
        <input type="file" accept={accept} className="sr-only" onChange={(e) => onChange(e.target.files?.[0])} />
      </label>
      {file && <span className="min-w-0 truncate type-callout text-text-secondary">{file.name}</span>}
    </div>
  );
}

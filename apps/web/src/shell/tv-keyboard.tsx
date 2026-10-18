// The television's keyboard for fields (docs/tv.md §5): OK on a text or
// number field opens a keyboard of keys reached with the arrows, as
// Search's is, over the page; what it types goes into the field as it is
// typed, Done keeps it and Back puts back what the field held. A field a
// keyboard has typed into keeps its Enter.
import { Dialog } from "@base-ui/react/dialog";
import { type KeyboardEvent as ReactKeyboardEvent, useEffect, useRef, useState } from "react";

import { useDialogKeys } from "../controls/dialog.tsx";
import { backdrop, dialogFrame } from "../controls/styles.ts";
import { t } from "../i18n.ts";

/** A field the keyboard types into. */
type Field = HTMLInputElement | HTMLTextAreaElement;

/** The inputs typed into; others, as checkboxes and ranges, have their own keys. */
const typed = ["text", "search", "url", "email", "password", "number", "tel"];

/**
 * The field OK opens the keyboard for: one typed into, not read-only, and
 * not a list of options to choose from (select.tsx's picker), nor one
 * whose suggestions are open, where OK chooses one.
 */
function fieldOf(el: Element | null): Field | undefined {
  if (!(el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement)) return undefined;
  if (el instanceof HTMLInputElement && !typed.includes(el.type)) return undefined;
  if (el.readOnly || el.disabled || el.closest("[data-tv-list]") || el.getAttribute("aria-expanded") === "true") return undefined;
  return el;
}

/** Sets a field's value as typing would, so that React and Base UI see it. */
function setValue(el: Field, value: string) {
  const proto = el instanceof HTMLInputElement ? HTMLInputElement.prototype : HTMLTextAreaElement.prototype;
  Object.getOwnPropertyDescriptor(proto, "value")?.set?.call(el, value);
  el.dispatchEvent(new Event("input", { bubbles: true }));
}

/** What names a field: its label, else its placeholder. */
function labelOf(el: Field): string {
  return el.getAttribute("aria-label") ?? el.labels?.[0]?.textContent?.trim() ?? el.placeholder;
}

/** Whether a field takes only a number, and whether it may have a fraction. */
function numberOf(el: Field): "integer" | "decimal" | undefined {
  if (el.inputMode === "numeric") return "integer";
  if (el.inputMode === "decimal" || (el instanceof HTMLInputElement && el.type === "number")) return "decimal";
  return undefined;
}

const letters = [..."abcdefghijklmnopqrstuvwxyz"];
const digits = [..."1234567890"];
const symbols = [..."-_.@/:;,?!#$%&*+=()[]{}<>'\"~|\\"];

/**
 * The fields a keyboard has typed into since they were focused, whose Enter
 * and Backspace are the field's own (back.ts), as sending what was typed.
 */
const typedIn = new WeakSet<EventTarget>();

/** Whether a keyboard has typed into el since it was focused. */
export function typedInto(el: Element | null): boolean {
  return !!el && typedIn.has(el);
}

/** Opens the keyboard with OK on a field while on. */
export function TvKeyboard() {
  const [field, setField] = useState<Field>();
  useEffect(() => {
    const focus = (e: FocusEvent) => e.target && typedIn.delete(e.target);
    const input = (e: Event) => e.isTrusted && e.target && typedIn.add(e.target);
    // Ahead of the field's own keys, as a form's submitting.
    const key = (e: KeyboardEvent) => {
      if (e.key !== "Enter" || e.metaKey || e.ctrlKey || e.altKey || e.shiftKey || e.isComposing) return;
      const el = fieldOf(document.activeElement);
      if (!el || typedIn.has(el)) return;
      e.preventDefault();
      e.stopPropagation();
      setField(el);
    };
    document.addEventListener("focusin", focus);
    document.addEventListener("input", input, true);
    document.addEventListener("keydown", key, true);
    return () => {
      document.removeEventListener("focusin", focus);
      document.removeEventListener("input", input, true);
      document.removeEventListener("keydown", key, true);
    };
  }, []);
  const close = () => {
    setField(undefined);
    // Back on the field, to move on from or type into again.
    requestAnimationFrame(() => field?.focus({ preventScroll: true }));
  };
  return field && <Keyboard field={field} onClose={close} />;
}

function Keyboard({ field, onClose }: { field: Field; onClose: () => void }) {
  const [original] = useState(field.value);
  const [text, setText] = useState(field.value);
  const [upper, setUpper] = useState(false);
  const [more, setMore] = useState(false);
  const first = useRef<HTMLButtonElement>(null);
  const keys = useDialogKeys();
  const number = numberOf(field);
  const secret = field instanceof HTMLInputElement && field.type === "password";
  const lines = field instanceof HTMLTextAreaElement;
  const change = (f: (s: string) => string) =>
    setText((s) => {
      const next = f(s);
      setValue(field, next);
      return next;
    });
  const type = (c: string) => change((s) => s + c);
  const erase = () => change((s) => [...s].slice(0, -1).join(""));
  const cancel = () => {
    setValue(field, original);
    onClose();
  };
  // A keyboard types as well; Back deletes a character, and closes the
  // keyboard once there is none, as Search's does.
  const onKeyDown = (e: ReactKeyboardEvent & { preventBaseUIHandler: () => void }) => {
    keys?.(e);
    if (e.metaKey || e.ctrlKey || e.altKey) return;
    if (e.key === "Backspace" && text !== "") erase();
    else if (e.key.length === 1 && !(e.key === " " && e.target instanceof HTMLButtonElement)) type(e.key);
    else return;
    e.preventDefault();
    e.stopPropagation();
  };
  const key = "tv-pill flex h-14 items-center justify-center rounded-lg bg-fill type-headline";
  const chars = number ? [..."123456789", "-", "0", ...(number === "decimal" ? ["."] : [])] : more ? symbols : [...digits, ...letters, ..."-_.@"];
  return (
    <Dialog.Root
      open
      onOpenChange={(open, details) => {
        if (open) return;
        // Escape is Back, deleting first.
        if (details.reason === "escape-key" && text !== "") {
          details.cancel();
          erase();
        } else cancel();
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className={backdrop} />
        <Dialog.Popup
          aria-modal
          aria-label={labelOf(field)}
          initialFocus={first}
          finalFocus={false}
          onKeyDown={onKeyDown}
          className={`${dialogFrame} ${number ? "w-[min(440px,calc(100vw-32px))]" : "w-[min(820px,calc(100vw-32px))]"} gap-4 p-6`}
        >
          <Dialog.Title className="type-title3 text-text-secondary">{labelOf(field)}</Dialog.Title>
          <div aria-live="polite" className="flex min-h-16 items-center rounded-lg bg-surface-raised px-5 type-title3">
            {/* The caret shows where the keys type, wherever focus is. */}
            <span className={`min-w-0 ${lines ? "max-h-40 overflow-y-auto break-all whitespace-pre-wrap" : "truncate whitespace-pre"}`}>
              {secret ? "•".repeat([...text].length) : text}
              <span aria-hidden className="caret mx-px inline-block h-[1.1em] w-0.5 rounded-full bg-accent align-text-bottom" />
            </span>
          </div>
          <div className={`grid gap-2 ${number ? "grid-cols-3" : "grid-cols-10"}`}>
            {chars.map((c, i) => (
              <button key={c} ref={i === 0 ? first : undefined} type="button" onClick={() => type(upper ? c.toUpperCase() : c)} className={key}>
                {upper ? c.toUpperCase() : c}
              </button>
            ))}
          </div>
          <div className="flex gap-2">
            {!number && (
              <>
                <button type="button" aria-pressed={upper} onClick={() => setUpper((u) => !u)} disabled={more} className={`${key} w-24 disabled:opacity-40`}>
                  {upper ? "abc" : "ABC"}
                </button>
                <button type="button" aria-pressed={more} onClick={() => setMore((m) => !m)} className={`${key} w-24`}>
                  {more ? "abc" : "#+="}
                </button>
                <button type="button" onClick={() => type(" ")} className={`${key} flex-1`}>
                  {t("search.space")}
                </button>
                {lines && (
                  <button type="button" onClick={() => type("\n")} className={`${key} w-20`}>
                    ↵
                  </button>
                )}
              </>
            )}
            <button type="button" onClick={erase} className={`${key} flex-1`}>
              {t("search.backspace")}
            </button>
            <button type="button" onClick={() => change(() => "")} className={`${key} flex-1`}>
              {t("search.clear")}
            </button>
            <button type="button" onClick={onClose} className={`${key.replace("bg-fill", "bg-accent")} flex-1 text-on-accent`}>
              {t("action.done")}
            </button>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

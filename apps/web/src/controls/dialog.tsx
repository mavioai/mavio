// Dialogs (docs/clients.md §6.4): Base UI's modal dialog over a dimmed
// page, focus kept inside it and returned where it was, closed by Escape
// or a press outside; and the alert asking to confirm what cannot be
// undone, which only its buttons close.
import { AlertDialog } from "@base-ui/react/alert-dialog";
import { Dialog } from "@base-ui/react/dialog";
import { useBlocker } from "@tanstack/react-router";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { t } from "../i18n.ts";
import { prefs } from "../prefs.ts";
import { Icon } from "../icon.tsx";
import { Button } from "../ui.tsx";
import { backdrop, dialogFrame } from "./styles.ts";

/** The widths of dialogs. */
const widths = {
  pin: "w-[300px] rounded-[22px]",
  narrow: "w-[min(440px,calc(100vw-32px))]",
  medium: "w-[min(640px,calc(100vw-32px))]",
  wide: "w-[min(720px,calc(100vw-32px))]",
  full: "w-[min(960px,calc(100vw-32px))]",
};

/**
 * Where focus goes when a dialog closes: the element that had it as the
 * dialog opened, or, for an item of a menu, the button that opened the menu.
 */
function useOpener() {
  const opener = useRef<HTMLElement | null>(null);
  if (opener.current === null) {
    const active = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const menu = active?.closest("[role=menu]");
    opener.current = (menu?.id && document.querySelector<HTMLElement>(`[aria-controls="${CSS.escape(menu.id)}"]`)) || active;
  }
  return opener;
}

/** The ways the dialogs open close, the last opened last. */
const open: { current: () => void }[] = [];

/**
 * Back closes the dialog opened last rather than leaving the page under
 * it; the history stays as it was. Not on the television, whose Back is a
 * key the dialog takes as Escape.
 */
function useBackCloses(close: () => void) {
  const [entry] = useState(() => ({ current: close }));
  useEffect(() => {
    entry.current = close;
  });
  useEffect(() => {
    open.push(entry);
    return () => {
      open.splice(open.indexOf(entry), 1);
    };
  }, [entry]);
  useBlocker({
    shouldBlockFn: ({ action }) => {
      if (action !== "BACK" || prefs().tv) return false;
      if (open.at(-1) === entry) entry.current();
      return true;
    },
    enableBeforeUnload: false,
  });
}

/**
 * A modal dialog shown while it is rendered, with its title above its
 * content; onClose is called on Escape and a press outside it, which ask
 * first while changed holds changes not saved.
 */
export function Modal({
  title,
  wide = false,
  width,
  changed = false,
  onClose,
  children,
}: {
  title: ReactNode;
  wide?: boolean;
  width?: keyof typeof widths;
  changed?: boolean;
  onClose: () => void;
  children: ReactNode;
}) {
  const opener = useOpener();
  const [asking, setAsking] = useState(false);
  const close = () => (changed ? setAsking(true) : onClose());
  useBackCloses(close);
  return (
    <Dialog.Root open onOpenChange={(open) => !open && close()}>
      {asking && (
        <Confirm
          title={t("unsaved.title")}
          message={t("unsaved.message")}
          action={t("unsaved.discard")}
          busy={false}
          onCancel={() => setAsking(false)}
          onConfirm={onClose}
        />
      )}
      <Dialog.Portal>
        <Dialog.Backdrop className={backdrop} />
        <Dialog.Popup aria-modal finalFocus={opener} className={`${dialogFrame} ${widths[width ?? (wide ? "wide" : "narrow")]}`}>
          <div className="flex max-h-[85vh] min-h-0 flex-col gap-3 p-6">
            <Dialog.Title className="type-title2">{title}</Dialog.Title>
            {children}
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/**
 * A modal dialog without a title of its own, for content laid out
 * otherwise; label names it for assistive technology.
 */
export function Sheet({ label, width = "narrow", className, onClose, children }: { label: string; width?: keyof typeof widths; className?: string; onClose: () => void; children: ReactNode }) {
  const opener = useOpener();
  useBackCloses(onClose);
  return (
    <Dialog.Root open onOpenChange={(open) => !open && onClose()}>
      <Dialog.Portal>
        <Dialog.Backdrop className={backdrop} />
        <Dialog.Popup aria-modal aria-label={label} finalFocus={opener} className={`${dialogFrame} ${widths[width]} ${className ?? ""}`}>
          {children}
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}

/** The round button closing the dialog it is in. */
export function Dismiss() {
  return (
    <Dialog.Close
      aria-label={t("action.close")}
      className="flex size-8 shrink-0 items-center justify-center rounded-full bg-fill hover:bg-fill-selected"
    >
      <Icon name="close" size={16} />
    </Dialog.Close>
  );
}

/**
 * Asks to confirm what cannot be undone, as an alert: the title, what it
 * does, and the red action beside Cancel.
 */
export function Confirm({
  title,
  message,
  action,
  busy,
  onCancel,
  onConfirm,
}: {
  title: string;
  message?: ReactNode;
  action: string;
  busy: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  const opener = useOpener();
  useBackCloses(onCancel);
  return (
    <AlertDialog.Root open onOpenChange={(open) => !open && onCancel()}>
      <AlertDialog.Portal>
        <AlertDialog.Backdrop className={backdrop} />
        <AlertDialog.Popup aria-modal finalFocus={opener} className={`${dialogFrame} ${widths.narrow} gap-3 p-6`}>
          <AlertDialog.Title className="type-title3">{title}</AlertDialog.Title>
          {message && <AlertDialog.Description className="type-callout break-words text-text-secondary">{message}</AlertDialog.Description>}
          <div className="mt-2 flex justify-end gap-2">
            <AlertDialog.Close render={<Button variant="secondary" />}>{t("action.cancel")}</AlertDialog.Close>
            <Button variant="secondary" className="text-danger" disabled={busy} onClick={onConfirm}>
              {busy ? "…" : action}
            </Button>
          </div>
        </AlertDialog.Popup>
      </AlertDialog.Portal>
    </AlertDialog.Root>
  );
}

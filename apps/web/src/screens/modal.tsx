// The modal dialog of the item screens, and its text fields.
import { type KeyboardEvent, type ReactNode, useEffect, useId, useRef, useState } from "react";

/** A modal dialog, closed by Escape or a click outside it. */
export function Modal({ title, wide = false, onClose, children }: { title: string; wide?: boolean; onClose: () => void; children: ReactNode }) {
  const id = useId();
  const ref = useRef<HTMLDialogElement>(null);
  const [opener] = useState(() => (document.activeElement instanceof HTMLElement ? document.activeElement : undefined));
  useEffect(() => {
    if (!ref.current?.open) ref.current?.showModal();
    return () => opener?.focus({ preventScroll: true });
  }, [opener]);
  const onKeyDown = (e: KeyboardEvent) => {
    if (e.key !== "Escape") return;
    e.preventDefault();
    e.stopPropagation();
    onClose();
  };
  return (
    <dialog
      ref={ref}
      aria-modal
      aria-labelledby={id}
      onClose={onClose}
      onKeyDown={onKeyDown}
      onClick={(e) => e.target === ref.current && onClose()}
      className={`m-auto max-h-[85vh] ${wide ? "w-[min(720px,calc(100vw-32px))]" : "w-[min(440px,calc(100vw-32px))]"} rounded-[18px] border border-border bg-surface-overlay p-0 text-on-background shadow-level3 backdrop:bg-black/60 backdrop:backdrop-blur-sm`}
    >
      <div className="flex max-h-[85vh] flex-col gap-3 p-6">
        <h2 id={id} className="type-title2">
          {title}
        </h2>
        {children}
      </div>
    </dialog>
  );
}

/** A text field's style in a dialog. */
export const field =
  "min-h-11 min-w-0 flex-1 rounded-[10px] border border-border bg-surface px-3 type-body placeholder:text-text-tertiary focus:border-focus focus:outline-none";

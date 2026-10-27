// The website's components, drawn as the web UI draws them (docs/clients.md
// §6, libs/design/components.md): its buttons, fields, the settings' groups
// of rows, avatars, the activity indicator and the icons of libs/design.
import { type IconName, iconViewBox, icons } from "@mavio/design/icons";
import { createContext, type ReactNode, useCallback, useContext, useRef, useState } from "react";

import { useT } from "./lib/locale.tsx";

export type { IconName };

/** An icon of libs/design: Regular when inactive, Fill when active. Decorative unless labeled. */
export function Icon({ name, fill = false, size = 24, label, className }: { name: IconName; fill?: boolean; size?: number; label?: string; className?: string }) {
  return (
    <svg
      viewBox={`0 0 ${iconViewBox} ${iconViewBox}`}
      width={size}
      height={size}
      fill="currentColor"
      className={className}
      role={label ? "img" : undefined}
      aria-label={label}
      aria-hidden={label ? undefined : true}
    >
      {icons[name][fill ? "fill" : "regular"].map((d) => (
        <path key={d} d={d} />
      ))}
    </svg>
  );
}

/** A page of the signed-in account: its title and content, as wide as reads well. */
export function Page({ title, children }: { title: ReactNode; children: ReactNode }) {
  return (
    <main className="mx-auto flex w-full max-w-[720px] flex-col gap-8 px-margin pt-6 pb-16">
      <h1 className="type-title1 [overflow-wrap:anywhere]">{title}</h1>
      {children}
    </main>
  );
}

/** A group of rows on a raised surface, as in the web UI's settings. */
export function Group({ title, footer, children }: { title?: ReactNode; footer?: ReactNode; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-2">
      {title && <h2 className="px-1 type-headline">{title}</h2>}
      <div className="flex flex-col divide-y divide-border overflow-hidden rounded-[14px] bg-surface">{children}</div>
      {footer && <div className="px-1 type-footnote text-text-secondary">{footer}</div>}
    </section>
  );
}

/** A row of a group: what leads it, a label, a line under it and its controls. */
export function Row({ label, detail, leading, children }: { label: ReactNode; detail?: ReactNode; leading?: ReactNode; children?: ReactNode }) {
  return (
    <div className="flex min-h-13 flex-wrap items-center gap-x-3 gap-y-2 px-4 py-2.5">
      {leading}
      <div className="min-w-0 flex-1">
        <div className="type-body [overflow-wrap:anywhere]">{label}</div>
        {detail && <div className="type-footnote text-text-secondary [overflow-wrap:anywhere]">{detail}</div>}
      </div>
      {children && <div className="flex shrink-0 items-center gap-2">{children}</div>}
    </div>
  );
}

/** A group's content that is not a row: text, a form. */
export function Block({ children }: { children: ReactNode }) {
  return <div className="flex flex-col gap-4 px-4 py-4">{children}</div>;
}

type Variant = "primary" | "secondary" | "plain" | "danger";

/** A button of the primary, secondary or plain kind, or a destructive one. */
export function Button({ variant = "primary", className, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant }) {
  const styles = {
    primary: "bg-button text-on-button hover:opacity-85",
    secondary: "bg-fill text-on-background hover:bg-fill-selected",
    plain: "text-accent-text hover:bg-fill",
    danger: "bg-fill text-danger hover:bg-fill-selected",
  }[variant];
  return (
    <button
      type="button"
      {...props}
      className={`inline-flex min-h-9 items-center justify-center gap-2 rounded-md px-4 type-headline transition active:opacity-80 disabled:opacity-50 ${styles} ${className ?? ""}`}
    />
  );
}

const inputStyle =
  "min-h-11 w-full min-w-0 rounded-[10px] border border-border bg-background px-3 type-body text-on-background placeholder:text-text-tertiary focus:border-focus focus:outline-none read-only:text-text-secondary";

/** A text field with its label above. */
export function Field({ label, hint, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { label: string; hint?: string }) {
  return (
    <label className="flex min-w-0 flex-1 flex-col gap-1.5">
      <span className="type-footnote font-semibold text-text-secondary">{label}</span>
      <input {...props} className={inputStyle} />
      {hint && <span className="type-footnote text-text-tertiary">{hint}</span>}
    </label>
  );
}

/** A choice of values with its label above. */
export function Select({ label, options, ...props }: React.SelectHTMLAttributes<HTMLSelectElement> & { label: string; options: [string | number, string][] }) {
  return (
    <label className="flex min-w-0 flex-1 flex-col gap-1.5">
      <span className="type-footnote font-semibold text-text-secondary">{label}</span>
      <span className="relative">
        <select {...props} className={`${inputStyle} appearance-none pr-9`}>
          {options.map(([value, text]) => (
            <option key={value} value={value}>
              {text}
            </option>
          ))}
        </select>
        <Icon name="selector" size={16} className="pointer-events-none absolute top-1/2 right-3 -translate-y-1/2 text-text-tertiary" />
      </span>
    </label>
  );
}

/** A checkbox with its label, in the accent. */
export function Check({ label, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { label: ReactNode }) {
  return (
    <label className="flex min-h-7 items-center gap-2.5 type-body">
      <input type="checkbox" {...props} className="size-[18px] shrink-0 accent-[var(--mavio-color-accent)]" />
      {label}
    </label>
  );
}

/** The buttons ending a form, from its start. */
export function Actions({ children }: { children: ReactNode }) {
  return <div className="flex flex-wrap items-center gap-2">{children}</div>;
}

/** An error, in the danger color. */
export function ErrorText({ error }: { error?: string }) {
  return error ? (
    <p role="alert" className="flex items-start gap-2 type-callout text-danger">
      <Icon name="error" size={18} className="mt-px shrink-0" />
      <span className="[overflow-wrap:anywhere]">{error}</span>
    </p>
  ) : null;
}

/** Text in the secondary color. */
export function Muted({ children }: { children: ReactNode }) {
  return <p className="type-callout text-text-secondary [overflow-wrap:anywhere]">{children}</p>;
}

/** A name in the type of a name: a server's, an address. */
export function Name({ children }: { children: ReactNode }) {
  return <span className="font-semibold text-on-background [overflow-wrap:anywhere]">{children}</span>;
}

/** A person's initial on a quiet fill (libs/design/components.md, Avatar). */
export function Avatar({ name, size = 32 }: { name: string; size?: number }) {
  return (
    <span
      aria-hidden
      className="inline-flex shrink-0 items-center justify-center rounded-full bg-fill-selected font-semibold text-text-secondary"
      style={{ width: size, height: size, fontSize: size * 0.42 }}
    >
      {[...name][0]?.toUpperCase()}
    </span>
  );
}

/** The activity indicator: eight spokes turning, shown after a moment. */
export function Spinner({ size = 28 }: { size?: number }) {
  const t = useT();
  return (
    <span role="status" aria-label={t("site.loading")} className="inline-flex animate-[fade_300ms_ease-out_400ms_backwards] text-text-tertiary">
      <svg viewBox="0 0 24 24" width={size} height={size} aria-hidden className="animate-[spinner_0.9s_steps(8)_infinite] motion-reduce:animate-none">
        {Array.from({ length: 8 }, (_, i) => (
          <rect key={i} x="11" y="2" width="2" height="6" rx="1" fill="currentColor" opacity={0.2 + (0.8 * i) / 7} transform={`rotate(${i * 45} 12 12)`} />
        ))}
      </svg>
    </span>
  );
}

/** A page's content on its way. */
export function Loading() {
  return (
    <div className="flex flex-1 items-center justify-center py-24">
      <Spinner />
    </div>
  );
}

/** The empty state of a list: an icon, a title and what to do. */
export function Empty({ icon, title, message, children }: { icon: IconName; title: string; message?: string; children?: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-3 rounded-[14px] bg-surface px-6 py-12 text-center">
      <Icon name={icon} size={44} className="text-text-tertiary" />
      <h2 className="type-title3">{title}</h2>
      {message && <p className="max-w-md type-body text-text-secondary">{message}</p>}
      {children}
    </div>
  );
}

/** What a confirmation asks. */
export interface Question {
  title: string;
  message?: string;
  action: string;
  destructive?: boolean;
}

const ConfirmContext = createContext<(q: Question) => Promise<boolean>>(async () => false);

/** useConfirm returns a function asking a question in a dialog, resolving to the answer. */
export const useConfirm = () => useContext(ConfirmContext);

/** ConfirmProvider draws the dialog of useConfirm, as the web UI's alerts. */
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const t = useT();
  const ref = useRef<HTMLDialogElement>(null);
  const [question, setQuestion] = useState<Question>();
  const answer = useRef<(yes: boolean) => void>(undefined);
  const ask = useCallback((q: Question) => {
    setQuestion(q);
    queueMicrotask(() => ref.current?.showModal());
    return new Promise<boolean>((resolve) => {
      answer.current = resolve;
    });
  }, []);
  const close = (yes: boolean) => {
    ref.current?.close();
    answer.current?.(yes);
    answer.current = undefined;
  };
  return (
    <ConfirmContext value={ask}>
      {children}
      <dialog
        ref={ref}
        onCancel={() => close(false)}
        className="m-auto w-[min(400px,calc(100vw-32px))] rounded-[18px] border border-border bg-surface-overlay p-6 text-on-background shadow-level3"
      >
        {question && (
          <div className="flex flex-col gap-5">
            <div className="flex flex-col gap-2">
              <h2 className="type-title3 [overflow-wrap:anywhere]">{question.title}</h2>
              {question.message && <p className="type-body text-text-secondary">{question.message}</p>}
            </div>
            <div className="flex justify-end gap-2">
              <Button variant="secondary" onClick={() => close(false)}>
                {t("site.action.cancel")}
              </Button>
              <Button variant={question.destructive ? "danger" : "primary"} onClick={() => close(true)} autoFocus>
                {question.action}
              </Button>
            </div>
          </div>
        )}
      </dialog>
    </ConfirmContext>
  );
}

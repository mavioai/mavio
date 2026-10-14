// The website's few components, drawn with the design tokens.
import type { ButtonHTMLAttributes, InputHTMLAttributes, ReactNode } from "react";

export function Page({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="mx-auto flex max-w-2xl flex-col gap-6 px-4 py-10">
      <h1 className="type-title1">{title}</h1>
      {children}
    </main>
  );
}

export function Card({ title, children }: { title?: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-4 rounded-lg border border-border bg-surface p-5">
      {title && <h2 className="type-headline">{title}</h2>}
      {children}
    </section>
  );
}

export function Button({
  kind = "primary",
  className = "",
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { kind?: "primary" | "secondary" | "danger" }) {
  const look = {
    primary: "bg-accent text-on-accent hover:bg-accent-pressed",
    secondary: "border border-border bg-surface-raised hover:bg-surface-overlay",
    danger: "border border-danger text-danger hover:bg-surface-overlay",
  }[kind];
  return (
    <button
      type="button"
      className={`type-callout rounded-md px-4 py-2 font-semibold disabled:opacity-50 ${look} ${className}`}
      {...props}
    />
  );
}

export function Field({ label, ...props }: InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  return (
    <label className="flex flex-col gap-1">
      <span className="type-footnote text-text-secondary">{label}</span>
      <input className="type-body rounded-md border border-border bg-background px-3 py-2" {...props} />
    </label>
  );
}

export function Check({ label, ...props }: InputHTMLAttributes<HTMLInputElement> & { label: string }) {
  return (
    <label className="type-callout flex items-center gap-2">
      <input type="checkbox" {...props} />
      {label}
    </label>
  );
}

export function ErrorText({ error }: { error?: string }) {
  return error ? (
    <p role="alert" className="type-callout text-danger">
      {error}
    </p>
  ) : null;
}

export function Muted({ children }: { children: ReactNode }) {
  return <p className="type-callout text-text-secondary">{children}</p>;
}

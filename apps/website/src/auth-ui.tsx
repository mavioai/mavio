// The pages of signing in, as the web UI's sign-in screen (docs/clients.md
// §4.1): a card of glass on a wall of posters, the wordmark above its
// title, labeled fields and buttons as wide as the card.
import type { ReactNode } from "react";

import { authClient } from "./lib/auth-client.ts";
import { useT } from "./lib/locale.tsx";
import { Wordmark } from "./logo.tsx";
import { Avatar, Icon, type IconName } from "./ui.tsx";

/** The colors of the posters, top to bottom: warm, cool and dusk, as films are. */
const tones = [
  ["#FF9F0A", "#6B2400"],
  ["#2A6F97", "#0A1A2E"],
  ["#8E5CF7", "#24104D"],
  ["#F0412F", "#3A0A0A"],
  ["#30B07A", "#0A2E20"],
  ["#FFD34E", "#7A3E00"],
  ["#5AC8FA", "#10325A"],
  ["#FF6482", "#420D26"],
  ["#C7A17A", "#33241A"],
  ["#64D2FF", "#1A2A66"],
  ["#E07A5F", "#2E1A3A"],
];

/** The screen behind the card, and the front page's: a wall of posters drawn from gradients, tilted and blurred under a veil. */
export function PosterWall() {
  return (
    <div aria-hidden className="pointer-events-none absolute inset-0 overflow-hidden">
      <div className="absolute -inset-[25%] flex gap-[1%]" style={{ transform: "rotate(-8deg) scale(1.15)", filter: "blur(2px)", opacity: "var(--signin-wall)" }}>
        {Array.from({ length: 8 }, (_, c) => (
          <div key={c} className="flex flex-1 flex-col gap-[1%]" style={{ marginTop: c % 2 ? "-6%" : "0" }}>
            {Array.from({ length: 9 }, (_, r) => {
              const [a, b] = tones[(c * 4 + r * 7) % tones.length];
              return (
                <div
                  key={r}
                  className="aspect-[2/3] w-full shrink-0 rounded-[8px]"
                  style={{ background: `radial-gradient(120% 70% at 25% 12%, rgb(255 255 255 / 0.28), transparent 55%), linear-gradient(165deg, ${a}, ${b})` }}
                />
              );
            })}
          </div>
        ))}
      </div>
      <div className="absolute inset-0" style={{ background: "var(--signin-veil)" }} />
    </div>
  );
}

/**
 * A page of signing in: the card on the wall of posters, with the
 * wordmark, a back link, the title and the line under it above its content.
 */
export function AuthScreen({ title, subtitle, onBack, wide = false, children }: { title: ReactNode; subtitle?: ReactNode; onBack?: () => void; wide?: boolean; children: ReactNode }) {
  const t = useT();
  return (
    <div className="relative flex min-h-dvh items-center justify-center overflow-hidden bg-background px-4 py-10">
      <PosterWall />
      <main
        className={`relative flex w-full flex-col gap-[22px] rounded-[26px] border border-(--signin-separator) bg-(--signin-card) px-6 pt-8 pb-7 shadow-(--signin-shadow) backdrop-blur-[40px] backdrop-saturate-150 sm:px-10 sm:pt-10 sm:pb-[34px] ${wide ? "max-w-[520px]" : "max-w-[460px]"}`}
      >
        <a href="/" className="self-start" aria-label="Mavio">
          <Wordmark height={26} />
        </a>
        {onBack && (
          <button type="button" onClick={onBack} className="-ml-1 inline-flex items-center gap-0.5 self-start text-[15px] text-accent-text hover:opacity-80">
            <Icon name="back" size={18} />
            {t("site.action.back")}
          </button>
        )}
        <div className="flex flex-col gap-1.5">
          <h1 className="text-[28px] leading-[34px] font-bold tracking-[-0.01em] [overflow-wrap:anywhere]">{title}</h1>
          {subtitle && <p className="text-[15px] leading-[22px] text-text-secondary [overflow-wrap:anywhere]">{subtitle}</p>}
        </div>
        {children}
      </main>
    </div>
  );
}

/** A form of a card's fields and buttons. */
export function AuthForm({ onSubmit, children }: { onSubmit: (form: FormData, el: HTMLFormElement) => void; children: ReactNode }) {
  return (
    <form
      className="flex flex-col gap-[18px]"
      onSubmit={(e) => {
        e.preventDefault();
        onSubmit(new FormData(e.currentTarget), e.currentTarget);
      }}
    >
      {children}
    </form>
  );
}

/** A field of a card, labeled above, with what sits at the end of its label. */
export function AuthField({ label, aside, className, ...props }: React.InputHTMLAttributes<HTMLInputElement> & { label: string; aside?: ReactNode }) {
  return (
    <label className="flex flex-col gap-2">
      <span className="flex items-baseline justify-between gap-3 text-[13px] font-semibold text-text-secondary">
        {label}
        {aside}
      </span>
      <input
        {...props}
        className={`h-11 rounded-[10px] border border-(--signin-separator) bg-(--signin-field) px-3.5 text-[15px] text-on-background placeholder:text-text-tertiary focus:border-accent focus:ring-1 focus:ring-accent focus:outline-none focus-visible:outline-none ${className ?? ""}`}
      />
    </label>
  );
}

/** The classes of a button of a card, as wide as it, for a link that looks as one. */
export function actionClass(variant: "primary" | "secondary" = "primary"): string {
  const styles = variant === "primary" ? "bg-button text-on-button hover:opacity-90" : "bg-fill text-on-background hover:bg-fill-selected";
  return `inline-flex min-h-12 w-full items-center justify-center gap-2 rounded-[12px] px-5 text-[16px] font-semibold transition active:opacity-80 disabled:opacity-50 ${styles}`;
}

/** A button of a card, as wide as it. */
export function Action({ variant = "primary", icon, className, children, ...props }: React.ButtonHTMLAttributes<HTMLButtonElement> & { variant?: "primary" | "secondary"; icon?: IconName }) {
  return (
    <button type="button" {...props} className={`${actionClass(variant)} ${className ?? ""}`}>
      {icon && <Icon name={icon} size={18} />}
      {children}
    </button>
  );
}

/** A line between ways of going on, broken by "or". */
export function Or() {
  const t = useT();
  return (
    <div className="flex w-full items-center gap-3 text-[13px] text-text-tertiary">
      <span className="h-px flex-1 bg-(--signin-separator)" />
      {t("site.or")}
      <span className="h-px flex-1 bg-(--signin-separator)" />
    </div>
  );
}

/** The card's body text. */
export function AuthText({ children }: { children: ReactNode }) {
  return <p className="text-[15px] leading-[22px] text-text-secondary [overflow-wrap:anywhere]">{children}</p>;
}

/** The line at the foot of a card, such as the way to the other of signing in and up. */
export function AuthFoot({ children }: { children: ReactNode }) {
  return <p className="text-center text-[14px] text-text-secondary">{children}</p>;
}

/** A link of a card, in the accent's text color. */
export const authLink = "font-semibold text-accent-text hover:underline";

/** A notice of a card: what went wrong, or what happened. */
export function Notice({ tone = "danger", children }: { tone?: "danger" | "success"; children?: ReactNode }) {
  if (!children) return null;
  return (
    <p role={tone === "danger" ? "alert" : "status"} className={`flex items-start gap-2 text-[14px] leading-5 ${tone === "danger" ? "text-danger" : "text-success"}`}>
      <Icon name={tone === "danger" ? "error" : "check"} size={18} className="mt-px shrink-0" />
      <span className="[overflow-wrap:anywhere]">{children}</span>
    </p>
  );
}

/** The account being signed in, with a link to sign in another. */
export function SignedInAs({ me }: { me: { name: string; email: string; username?: string | null } }) {
  const t = useT();
  // A managed account's e-mail address is a placeholder, never shown.
  const email = me.email.startsWith("managed:") ? undefined : me.email;
  const detail = me.username && me.username !== me.name ? `@${me.username}` : email;
  const switchAccount = async () => {
    const back = window.location.pathname + window.location.search;
    await authClient.signOut();
    window.location.href = `/sign-in?${new URLSearchParams({ next: back })}`;
  };
  return (
    <div className="flex items-center gap-3 rounded-[12px] border border-(--signin-separator) bg-(--signin-field) px-3.5 py-3">
      <Avatar name={me.name || me.username || "?"} size={36} />
      <span className="flex min-w-0 flex-1 flex-col">
        <span className="truncate text-[15px] font-semibold">{me.name || me.username}</span>
        {detail && <span className="truncate text-[13px] text-text-secondary">{detail}</span>}
      </span>
      <button type="button" className="shrink-0 text-[13px] font-semibold text-accent-text hover:underline" onClick={() => void switchAccount()}>
        {t("site.authorize.different")}
      </button>
    </div>
  );
}

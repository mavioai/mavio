// mavio.dev's e-mails (docs/mavio-dev.md §2), each a link sent through
// Cloudflare Email Service: verifying an address and resetting a password,
// written in the language of the request that asked for it; and telling an
// account of another's invitation to a server, friend request or invitation
// to a Home, written in the language the account last signed in with.
import { asLocale, type Locale, type T, translator } from "../lib/i18n.ts";

/** Mail is an e-mail to one address, as plain text and HTML. */
export interface Mail {
  to: string;
  subject: string;
  text: string;
  html: string;
}

/** Mailer sends e-mails. */
export interface Mailer {
  send(mail: Mail): Promise<void>;
}

/** The address mavio.dev's e-mails come from. */
export const sender = { name: "Mavio", email: "noreply@mavio.dev" };

/** EmailBinding is what a Worker's binding of Cloudflare Email Service is used for. */
export interface EmailBinding {
  send(message: { from: { name: string; email: string }; to: string; subject: string; text: string; html: string }): Promise<unknown>;
}

/** emailService returns a Mailer sending through a Worker's binding of Cloudflare Email Service. */
export function emailService(binding: EmailBinding): Mailer {
  return {
    send: async (mail) => {
      await binding.send({ from: sender, to: mail.to, subject: mail.subject, text: mail.text, html: mail.html });
    },
  };
}

const escapes: Record<string, string> = { "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" };

function escape(s: string): string {
  return s.replace(/[&<>"']/g, (c) => escapes[c]);
}

/**
 * A message with one action: a greeting, what it is about, the button and
 * its link spelled out, and what to do when the reader did not ask for it.
 */
function actionMail(to: string, subject: string, t: T, parts: { greeting: string; body: string; action: string; url: string; ignore: string }): Mail {
  const text = [parts.greeting, parts.body, `${parts.action}\n${parts.url}`, parts.ignore, "— Mavio"].join("\n\n");
  // Inline styles and a table: what e-mail clients draw alike. The colors
  // are the design's accent and grays (libs/design/tokens).
  const html = `<!doctype html>
<html><body style="margin:0;padding:0;background:#f2f2f7">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="background:#f2f2f7;padding:32px 16px">
<tr><td align="center">
<table role="presentation" width="100%" cellspacing="0" cellpadding="0" style="max-width:480px;background:#ffffff;border-radius:18px;padding:36px 32px;font-family:Inter,-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;color:#1c1c1e">
<tr><td style="font-size:26px;font-weight:800;letter-spacing:-0.02em;color:#ff9a2e;padding-bottom:24px">Mavio</td></tr>
<tr><td style="font-size:17px;line-height:24px;padding-bottom:12px">${escape(parts.greeting)}</td></tr>
<tr><td style="font-size:15px;line-height:22px;color:#3c3c43;padding-bottom:28px">${escape(parts.body)}</td></tr>
<tr><td style="padding-bottom:28px"><a href="${escape(parts.url)}" style="display:inline-block;background:#ff9a2e;color:#ffffff;font-size:15px;font-weight:600;text-decoration:none;border-radius:10px;padding:12px 22px">${escape(parts.action)}</a></td></tr>
<tr><td style="font-size:13px;line-height:19px;color:#8e8e93;padding-bottom:8px">${escape(t("mail.linkFallback"))}</td></tr>
<tr><td style="font-size:13px;line-height:19px;word-break:break-all;padding-bottom:28px"><a href="${escape(parts.url)}" style="color:#8e8e93">${escape(parts.url)}</a></td></tr>
<tr><td style="font-size:13px;line-height:19px;color:#8e8e93;border-top:1px solid #e5e5ea;padding-top:20px">${escape(parts.ignore)}</td></tr>
</table>
</td></tr>
</table>
</body></html>`;
  return { to, subject, text, html };
}

/** The account an e-mail is written to. */
export interface Recipient {
  email: string;
  name: string;
}

/** verificationMail asks an account to confirm its e-mail address at url. */
export function verificationMail(user: Recipient, url: string, locale: Locale): Mail {
  const t = translator(locale);
  return actionMail(user.email, t("mail.verifySubject"), t, {
    greeting: t("mail.greeting", { name: user.name }),
    body: t("mail.verifyBody"),
    action: t("mail.verifyAction"),
    url,
    ignore: t("mail.verifyIgnore"),
  });
}

/** resetMail gives an account the link to set a new password at url. */
export function resetMail(user: Recipient, url: string, locale: Locale): Mail {
  const t = translator(locale);
  return actionMail(user.email, t("mail.resetSubject"), t, {
    greeting: t("mail.greeting", { name: user.name }),
    body: t("mail.resetBody"),
    action: t("mail.resetAction"),
    url,
    ignore: t("mail.resetIgnore"),
  });
}

/** The account whose request an e-mail tells of: its display name and user name. */
export interface Inviter {
  name: string;
  userName: string;
}

/** inviterOf returns an account as the e-mails telling of its requests name it. */
export function inviterOf(u: { name: string; username?: string | null; displayUsername?: string | null }): Inviter {
  return { name: u.name, userName: u.displayUsername || u.username || "" };
}

/**
 * What the e-mails telling of other accounts' requests share: who asked,
 * the link to answer it, and what to do when the asker is unknown. Names
 * are other accounts' words, kept on one line in the subject.
 */
function requestMail(to: Recipient, t: T, from: Inviter, parts: { subject: string; body: string; action: string; url: string }): Mail {
  return actionMail(to.email, parts.subject.replace(/\s+/g, " "), t, {
    greeting: t("mail.greeting", { name: to.name }),
    body: parts.body,
    action: parts.action,
    url: parts.url,
    ignore: t("mail.requestIgnore", { name: from.name }),
  });
}

/** invitationMail tells an account that from invited it to the server named server, answered at url. */
export function invitationMail(to: Recipient, from: Inviter, server: string, url: string, locale: Locale): Mail {
  const t = translator(locale);
  return requestMail(to, t, from, {
    subject: t("mail.invitationSubject", { name: from.name, server }),
    body: t("mail.invitationBody", { name: from.name, userName: from.userName, server }),
    action: t("mail.invitationAction"),
    url,
  });
}

/** friendRequestMail tells an account that from asked to be its friend, answered at url. */
export function friendRequestMail(to: Recipient, from: Inviter, url: string, locale: Locale): Mail {
  const t = translator(locale);
  return requestMail(to, t, from, {
    subject: t("mail.friendSubject", { name: from.name }),
    body: t("mail.friendBody", { name: from.name, userName: from.userName }),
    action: t("mail.friendAction"),
    url,
  });
}

/** homeInvitationMail tells an account that from invited it to the Home named home, answered at url. */
export function homeInvitationMail(to: Recipient, from: Inviter, home: string, url: string, locale: Locale): Mail {
  const t = translator(locale);
  return requestMail(to, t, from, {
    subject: t("mail.homeSubject", { name: from.name }),
    body: t("mail.homeBody", { name: from.name, userName: from.userName, home }),
    action: t("mail.invitationAction"),
    url,
  });
}

/** Addressee is what mavio.dev knows of an account it may e-mail. */
export interface Addressee extends Recipient {
  emailVerified: boolean;
  kind?: string | null;
  locale?: string | null;
}

/**
 * mailTo returns the recipient and language of an e-mail to an account
 * telling of another account's request, undefined when it has no address
 * to send to: a managed account, or one whose address is not verified.
 * The language is the one the account last signed up or in with.
 */
export function mailTo(u: Addressee): { to: Recipient; locale: Locale } | undefined {
  if (u.kind === "managed" || !u.emailVerified) return undefined;
  return { to: { email: u.email, name: u.name }, locale: asLocale(u.locale ?? undefined) };
}

/**
 * deliver sends an e-mail telling of a request that already succeeded:
 * one that cannot be sent is logged, not failed, as the request stands.
 */
export async function deliver(mailer: Mailer | undefined, mail: Mail | undefined): Promise<void> {
  if (!mailer || !mail) return;
  try {
    await mailer.send(mail);
  } catch (e) {
    console.error("sending an e-mail failed", { subject: mail.subject, error: e instanceof Error ? e.message : String(e) });
  }
}

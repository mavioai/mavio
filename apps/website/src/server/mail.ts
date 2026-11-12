// mavio.dev's e-mails (docs/mavio-dev.md §2): verifying an address and
// resetting a password, each a link, written in the language of the
// request that asked for it and sent through Cloudflare Email Service.
import { type Locale, type T, translator } from "../lib/i18n.ts";

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

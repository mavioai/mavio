// Link codes (RFC 8628, docs/accounts.md §6): a television shows a code an
// account approves at mavio.dev/link, while it polls the token endpoint
// with the device code that goes with it.
import { newClaimCode } from "./claim-codes.ts";

/** linkCodeLifetime is how long a link code is valid, in milliseconds. */
export const linkCodeLifetime = 10 * 60_000;

/** linkPollInterval is how often a device polls, in seconds. */
export const linkPollInterval = 5;

/** newUserCode returns a random code to show, as XXXX-XXXX. */
export const newUserCode = newClaimCode;

/** userCodeKey returns how a user code is stored: upper case, without spaces and dashes. */
export function userCodeKey(code: string): string {
  return code.toUpperCase().replace(/[\s-]/g, "");
}

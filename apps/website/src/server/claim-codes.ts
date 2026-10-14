// Claim codes (docs/accounts.md §3.2): eight characters an account types
// into its server's setup or passes in MAVIO_CLAIM_TOKEN, kept as hashes.

const alphabet = "ABCDEFGHJKMNPQRSTUVWXYZ23456789";

/** ClaimCodeLifetime is how long a claim code is valid, in milliseconds. */
export const claimCodeLifetime = 10 * 60 * 1000;

/** newClaimCode returns a random code, shown as XXXX-XXXX. */
export function newClaimCode(): string {
  const bytes = crypto.getRandomValues(new Uint8Array(8));
  let code = "";
  for (const b of bytes) code += alphabet[b % alphabet.length];
  return `${code.slice(0, 4)}-${code.slice(4)}`;
}

/** claimCodeHash returns the stored hash of a code, ignoring case, spaces and dashes. */
export async function claimCodeHash(code: string): Promise<string> {
  const normalized = code.toUpperCase().replace(/[\s-]/g, "");
  const digest = await crypto.subtle.digest("SHA-256", new TextEncoder().encode(normalized));
  return Buffer.from(digest).toString("hex");
}

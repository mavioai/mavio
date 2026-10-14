// The origins a server's web interface is expected at (docs/accounts.md
// §6): its own names under mavio.site, and addresses on the local network.

/** expectedOrigin reports whether a redirect URI is at one of the server's names or on the local network. */
export function expectedOrigin(redirectUri: string, serverId: string): boolean {
  let host: string;
  try {
    host = new URL(redirectUri).hostname.toLowerCase();
  } catch {
    return false;
  }
  if (host === `${serverId}.mavio.site` || host.endsWith(`.${serverId}.mavio.site`)) return true;
  if (host === "localhost" || host.endsWith(".localhost") || host.endsWith(".local")) return true;
  const v4 = host.match(/^(\d+)\.(\d+)\.(\d+)\.(\d+)$/)?.slice(1).map(Number);
  if (v4) {
    const [a, b] = v4;
    return a === 10 || a === 127 || (a === 172 && b >= 16 && b < 32) || (a === 192 && b === 168) || (a === 169 && b === 254);
  }
  if (host.startsWith("[")) {
    const v6 = host.slice(1, -1);
    return v6 === "::1" || /^f[cd]/.test(v6) || /^fe[89ab]/.test(v6);
  }
  return false;
}

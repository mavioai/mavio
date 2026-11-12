// Serving the API on Workers: one PostgreSQL client per request, reached
// through Hyperdrive, and the notices of the servers' Durable Objects
// (docs/mavio-dev.md §2, §3).
import { env } from "cloudflare:workers";
import { drizzle } from "drizzle-orm/node-postgres";
import pg from "pg";

import { type Acme, cloudflareDns, googleBindings } from "./acme.ts";
import { handleApi } from "./api.ts";
import { createAuth } from "./auth.ts";
import { Stripe } from "./billing.ts";
import { type Database, schema } from "./db/index.ts";
import { bindingLimit } from "./limits.ts";
import { emailService } from "./mail.ts";
import type { NoticeHub } from "./notice-hub.ts";
import { parseRelays } from "./relay.ts";

/** serveApi serves a request under /api; notices go to the servers' Durable Objects once it is done. */
export async function serveApi(req: Request, ctx: ExecutionContext): Promise<Response> {
  // The binding names its Worker, so its type does not know the class.
  const notices = env.NOTICES as DurableObjectNamespace<NoticeHub>;
  const hub = (id: string) => notices.get(notices.idFromName(id));
  const client = new pg.Client({ connectionString: env.HYPERDRIVE.connectionString });
  await client.connect();
  try {
    const db: Database = drizzle(client, { schema });
    const stripe = stripeOf();
    const mailer = emailService(env.EMAIL);
    const auth = createAuth(db, { baseURL: env.BASE_URL, secret: env.BETTER_AUTH_SECRET, mailer, stripe });
    const res = await handleApi(req, {
      db,
      auth,
      mailer,
      baseURL: env.BASE_URL,
      appURL: env.APP_URL,
      notify: (cursors) => ctx.waitUntil(Promise.all([...cursors].map(([id, cursor]) => hub(id).notify(cursor)))),
      connectNotices: (id, r) => hub(id).fetch(r),
      acme: acme(),
      relays: env.RELAYS && env.RELAY_TICKET_SECRET ? parseRelays(env.RELAYS, env.RELAY_TICKET_SECRET) : undefined,
      stripe,
      limit: bindingLimit({ credentials: env.CREDENTIALS_LIMIT, mail: env.MAIL_LIMIT, codes: env.CODES_LIMIT }),
    });
    return res ?? new Response("Not Found", { status: 404 });
  } finally {
    await client.end();
  }
}

/** stripeOf returns the Stripe account Mavio Pass is sold through, undefined when not configured. */
function stripeOf(): Stripe | undefined {
  if (!env.STRIPE_SECRET_KEY || !env.STRIPE_WEBHOOK_SECRET || !env.STRIPE_PRICE_MONTH || !env.STRIPE_PRICE_YEAR) return undefined;
  return new Stripe({
    secretKey: env.STRIPE_SECRET_KEY,
    webhookSecret: env.STRIPE_WEBHOOK_SECRET,
    prices: { month: env.STRIPE_PRICE_MONTH, year: env.STRIPE_PRICE_YEAR },
  });
}

/** acme returns how servers' certificate requests are answered, undefined when not configured. */
function acme(): Acme | undefined {
  if (!env.ACME_DOMAIN || !env.CLOUDFLARE_ZONE_ID || !env.CLOUDFLARE_API_TOKEN) return undefined;
  return {
    domain: env.ACME_DOMAIN,
    dns: cloudflareDns(env.CLOUDFLARE_API_TOKEN, env.CLOUDFLARE_ZONE_ID),
    bindings:
      env.GOOGLE_CA_PROJECT && env.GOOGLE_CA_CREDENTIALS ? googleBindings(env.GOOGLE_CA_CREDENTIALS, env.GOOGLE_CA_PROJECT) : undefined,
  };
}

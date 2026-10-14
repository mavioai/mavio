// Serving the API on Workers: one PostgreSQL client per request, reached
// through Hyperdrive, and the notices of the servers' Durable Objects
// (docs/mavio-dev.md §2, §3).
import { env } from "cloudflare:workers";
import { drizzle } from "drizzle-orm/node-postgres";
import pg from "pg";

import { handleApi } from "./api.ts";
import { createAuth } from "./auth.ts";
import { type Database, schema } from "./db/index.ts";

/** serveApi serves a request under /api; notices go to the servers' Durable Objects once it is done. */
export async function serveApi(req: Request, ctx: ExecutionContext): Promise<Response> {
  const hub = (id: string) => env.NOTICES.get(env.NOTICES.idFromName(id));
  const client = new pg.Client({ connectionString: env.HYPERDRIVE.connectionString });
  await client.connect();
  try {
    const db: Database = drizzle(client, { schema });
    const auth = createAuth(db, { baseURL: env.BASE_URL, secret: env.BETTER_AUTH_SECRET });
    const res = await handleApi(req, {
      db,
      auth,
      baseURL: env.BASE_URL,
      notify: (cursors) => ctx.waitUntil(Promise.all([...cursors].map(([id, cursor]) => hub(id).notify(cursor)))),
      connectNotices: (id, r) => hub(id).fetch(r),
    });
    return res ?? new Response("Not Found", { status: 404 });
  } finally {
    await client.end();
  }
}

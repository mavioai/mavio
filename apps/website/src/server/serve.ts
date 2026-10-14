// Serving the API on Workers: one PostgreSQL client per request, reached
// through Hyperdrive (docs/mavio-dev.md §2).
import { env } from "cloudflare:workers";
import { drizzle } from "drizzle-orm/node-postgres";
import pg from "pg";

import { handleApi } from "./api.ts";
import { createAuth } from "./auth.ts";
import { type Database, schema } from "./db/index.ts";

/** serveApi serves a request under /api. */
export async function serveApi(req: Request): Promise<Response> {
  const client = new pg.Client({ connectionString: env.HYPERDRIVE.connectionString });
  await client.connect();
  try {
    const db: Database = drizzle(client, { schema });
    const auth = createAuth(db, { baseURL: env.BASE_URL, secret: env.BETTER_AUTH_SECRET });
    return (await handleApi(req, { db, auth, baseURL: env.BASE_URL })) ?? new Response("Not Found", { status: 404 });
  } finally {
    await client.end();
  }
}

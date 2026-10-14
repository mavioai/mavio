// A mavio.dev for tests: the API over PGlite, called in process.
import { fileURLToPath } from "node:url";

import { PGlite } from "@electric-sql/pglite";
import type { DescService } from "@bufbuild/protobuf";
import { type Client, createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { ServerService } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { drizzle } from "drizzle-orm/pglite";
import { migrate } from "drizzle-orm/pglite/migrator";

import { handleApi } from "../src/server/api.ts";
import { createAuth } from "../src/server/auth.ts";
import { type Database, schema } from "../src/server/db/index.ts";
import { base64url, serverID } from "../src/server/ids.ts";

export const baseURL = "http://localhost:3000";
export const apiURL = `${baseURL}/api`;

/** newSite returns a mavio.dev over an empty database. */
export async function newSite() {
  const pg = new PGlite();
  const db = drizzle(pg, { schema }) as unknown as Database;
  await migrate(drizzle(pg), { migrationsFolder: fileURLToPath(new URL("../drizzle", import.meta.url)) });
  const auth = createAuth(db, { baseURL, secret: "a test secret that is long enough to pass" });
  const site = {
    db,
    auth,
    fetch: async (req: Request) => (await handleApi(req, { db, auth, baseURL })) ?? new Response("Not Found", { status: 404 }),
    /** client returns a Connect client of service sending headers. */
    client<T extends DescService>(service: T, headers: Record<string, string> = {}, interceptors: Interceptor[] = []): Client<T> {
      return createClient(
        service,
        createConnectTransport({
          baseUrl: apiURL,
          interceptors,
          fetch: (input, init) => {
            const req = new Request(input, init);
            for (const [k, v] of Object.entries(headers)) req.headers.set(k, v);
            return site.fetch(req);
          },
        }),
      );
    },
    /** serverClient returns a ServerService client signing every request as server. */
    serverClient(server: { token: () => Promise<string> }): Client<typeof ServerService> {
      const sign: Interceptor = (next) => async (req) => {
        req.header.set("authorization", `Bearer ${await server.token()}`);
        return next(req);
      };
      return site.client(ServerService, {}, [sign]);
    },
    /** signUp creates an account and returns its bearer header. */
    async signUp(username: string) {
      const res = await auth.api.signUpEmail({
        body: { email: `${username}@example.com`, password: `${username} password`, name: username, username },
      });
      return { id: res.user.id, headers: { authorization: `Bearer ${res.token}` } };
    },
  };
  return site;
}

/** newServer returns a server's identity key and a signer of its JWTs, as the server signs them. */
export async function newServer() {
  const keys = (await crypto.subtle.generateKey({ name: "Ed25519" }, false, ["sign", "verify"])) as CryptoKeyPair;
  const publicKey = new Uint8Array(await crypto.subtle.exportKey("raw", keys.publicKey));
  const id = await serverID(publicKey);
  const enc = (v: unknown) => base64url(new TextEncoder().encode(JSON.stringify(v)));
  return {
    id,
    publicKey,
    async token(claims: Record<string, unknown> = {}) {
      const now = Math.floor(Date.now() / 1000);
      const signed = `${enc({ alg: "EdDSA", typ: "JWT", kid: id })}.${enc({
        iss: id,
        sub: id,
        aud: apiURL,
        iat: now,
        exp: now + 60,
        jti: crypto.randomUUID(),
        ...claims,
      })}`;
      const sig = new Uint8Array(await crypto.subtle.sign({ name: "Ed25519" }, keys.privateKey, new TextEncoder().encode(signed)));
      return `${signed}.${base64url(sig)}`;
    },
  };
}

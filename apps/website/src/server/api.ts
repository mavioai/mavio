// mavio.dev's API under /api (docs/mavio-dev.md §2, §3): better-auth at
// /api/auth, the Connect services of mavio.cloud.v1, the OAuth endpoints
// of devices at /api/oauth, the pages' configuration at /api/config, and
// servers' WebSockets at /api/servers/notices. Web UIs call the accounts'
// services and the OAuth endpoints from their own origins.
import { ConnectError, type ConnectRouter, createConnectRouter } from "@connectrpc/connect";
import { createFetchHandler } from "@connectrpc/connect/protocol";
import { createValidateInterceptor } from "@connectrpc/validate";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { ServerService } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { FriendService, HomeService } from "@mavio/proto/mavio/cloud/v1/home_pb";

import { accountService } from "./account-service.ts";
import type { Acme } from "./acme.ts";
import { type Auth, apiURL } from "./auth.ts";
import { cors, handleOAuth, preflight } from "./authorization.ts";
import type { Database } from "./db/index.ts";
import { friendService } from "./friend-service.ts";
import { homeService } from "./home-service.ts";
import { collectNotices } from "./notices.ts";
import type { Relays } from "./relay.ts";
import { authenticateServer, type Deps, serverService } from "./server-service.ts";

/** ApiOptions are what the API serves from. */
export interface ApiOptions {
  db: Database;
  auth: Auth;
  /** The site's origin, such as https://mavio.dev. */
  baseURL: string;
  /** app.mavio.dev's origin. */
  appURL?: string;
  now?: () => Date;
  /** notify tells servers that their changes reached cursors, once a request recorded changes. */
  notify?: (cursors: Map<string, number>) => void;
  /** acme answers servers' certificate requests. */
  acme?: Acme;
  /** relays are the relays servers connect to and clients are given tickets for. */
  relays?: Relays;
  /** connectNotices hands an authenticated server's WebSocket request to its Durable Object. */
  connectNotices?: (serverId: string, req: Request) => Promise<Response>;
}

/** noticesPath is where servers open their WebSockets (docs/mavio-dev.md §3). */
export const noticesPath = "/api/servers/notices";

/** configPath is where the pages read the site's configuration. */
export const configPath = "/api/config";

/** defaultAppURL is app.mavio.dev's origin when none is configured. */
export const defaultAppURL = "https://app.mavio.dev";

// The services web UIs call across origins, with devices' access tokens.
const crossOrigin = ["/api/mavio.cloud.v1.AccountService/", "/api/mavio.cloud.v1.FriendService/", "/api/mavio.cloud.v1.HomeService/"];

/** handleApi serves a request under /api, or returns undefined for other paths. */
export async function handleApi(req: Request, options: ApiOptions): Promise<Response | undefined> {
  const { pathname } = new URL(req.url);
  const deps = { db: options.db, apiURL: apiURL(options.baseURL), now: options.now, acme: options.acme, relays: options.relays };
  if (pathname === noticesPath) {
    let server;
    try {
      server = await authenticateServer(deps, req.headers);
    } catch (e) {
      return new Response(ConnectError.from(e).rawMessage, { status: 401 });
    }
    if (req.headers.get("upgrade")?.toLowerCase() !== "websocket") return new Response("Expected a WebSocket", { status: 426 });
    return options.connectNotices?.(server.id, req) ?? new Response("Not Implemented", { status: 501 });
  }
  const [res, cursors] = await collectNotices(() => route(req, pathname, options, deps));
  if (cursors.size > 0) options.notify?.(cursors);
  return res;
}

async function route(req: Request, pathname: string, options: ApiOptions, deps: Deps): Promise<Response | undefined> {
  if (pathname.startsWith("/api/auth/")) return options.auth.handler(req);
  if (pathname === configPath) return Response.json({ appURL: options.appURL ?? defaultAppURL });
  const oauth = await handleOAuth(req, pathname, {
    db: options.db,
    auth: options.auth,
    baseURL: options.baseURL,
    now: options.now ?? (() => new Date()),
  });
  if (oauth) return oauth;
  if (!pathname.startsWith("/api/mavio.")) return undefined;
  if (crossOrigin.some((p) => pathname.startsWith(p))) {
    if (req.method === "OPTIONS") return preflight(req);
    const res = await serve(req, pathname, options, deps);
    for (const [k, v] of Object.entries(cors(req))) res.headers.set(k, v);
    return res;
  }
  return serve(req, pathname, options, deps);
}

async function serve(req: Request, pathname: string, options: ApiOptions, deps: Deps): Promise<Response> {
  const router: ConnectRouter = createConnectRouter({ interceptors: [createValidateInterceptor()] });
  router.service(ServerService, serverService(deps));
  router.service(AccountService, accountService({ ...deps, auth: options.auth }));
  router.service(FriendService, friendService({ ...deps, auth: options.auth }));
  router.service(HomeService, homeService({ ...deps, auth: options.auth }));
  const handler = router.handlers.find((h) => `/api${h.requestPath}` === pathname);
  if (!handler) return new Response("Not Found", { status: 404 });
  return createFetchHandler(handler)(req);
}

// mavio.dev's API under /api (docs/mavio-dev.md §2, §3): better-auth at
// /api/auth, the Connect services of mavio.cloud.v1, and servers'
// WebSockets at /api/servers/notices.
import { ConnectError, type ConnectRouter, createConnectRouter } from "@connectrpc/connect";
import { createFetchHandler } from "@connectrpc/connect/protocol";
import { createValidateInterceptor } from "@connectrpc/validate";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { ServerService } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { FriendService, HomeService } from "@mavio/proto/mavio/cloud/v1/home_pb";

import { accountService } from "./account-service.ts";
import type { Acme } from "./acme.ts";
import { type Auth, apiURL } from "./auth.ts";
import type { Database } from "./db/index.ts";
import { friendService } from "./friend-service.ts";
import { homeService } from "./home-service.ts";
import { collectNotices } from "./notices.ts";
import { authenticateServer, type Deps, serverService } from "./server-service.ts";

/** ApiOptions are what the API serves from. */
export interface ApiOptions {
  db: Database;
  auth: Auth;
  /** The site's origin, such as https://mavio.dev. */
  baseURL: string;
  now?: () => Date;
  /** notify tells servers that their changes reached cursors, once a request recorded changes. */
  notify?: (cursors: Map<string, number>) => void;
  /** acme answers servers' certificate requests. */
  acme?: Acme;
  /** connectNotices hands an authenticated server's WebSocket request to its Durable Object. */
  connectNotices?: (serverId: string, req: Request) => Promise<Response>;
}

/** noticesPath is where servers open their WebSockets (docs/mavio-dev.md §3). */
export const noticesPath = "/api/servers/notices";

/** handleApi serves a request under /api, or returns undefined for other paths. */
export async function handleApi(req: Request, options: ApiOptions): Promise<Response | undefined> {
  const { pathname } = new URL(req.url);
  const deps = { db: options.db, apiURL: apiURL(options.baseURL), now: options.now, acme: options.acme };
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
  if (!pathname.startsWith("/api/mavio.")) return undefined;
  const router: ConnectRouter = createConnectRouter({ interceptors: [createValidateInterceptor()] });
  router.service(ServerService, serverService(deps));
  router.service(AccountService, accountService({ ...deps, auth: options.auth }));
  router.service(FriendService, friendService({ ...deps, auth: options.auth }));
  router.service(HomeService, homeService({ ...deps, auth: options.auth }));
  const handler = router.handlers.find((h) => `/api${h.requestPath}` === pathname);
  if (!handler) return new Response("Not Found", { status: 404 });
  return createFetchHandler(handler)(req);
}

// mavio.dev's API under /api (docs/mavio-dev.md §2): better-auth at
// /api/auth, and the Connect services of mavio.cloud.v1.
import { ConnectRouter, createConnectRouter } from "@connectrpc/connect";
import { createFetchHandler } from "@connectrpc/connect/protocol";
import { createValidateInterceptor } from "@connectrpc/validate";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { ServerService } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { FriendService, HomeService } from "@mavio/proto/mavio/cloud/v1/home_pb";

import { accountService } from "./account-service.ts";
import { type Auth, apiURL } from "./auth.ts";
import type { Database } from "./db/index.ts";
import { friendService } from "./friend-service.ts";
import { homeService } from "./home-service.ts";
import { serverService } from "./server-service.ts";

/** ApiOptions are what the API serves from. */
export interface ApiOptions {
  db: Database;
  auth: Auth;
  /** The site's origin, such as https://mavio.dev. */
  baseURL: string;
  now?: () => Date;
}

/** handleApi serves a request under /api, or returns undefined for other paths. */
export async function handleApi(req: Request, options: ApiOptions): Promise<Response | undefined> {
  const { pathname } = new URL(req.url);
  if (pathname.startsWith("/api/auth/")) return options.auth.handler(req);
  if (!pathname.startsWith("/api/mavio.")) return undefined;
  const router: ConnectRouter = createConnectRouter({ interceptors: [createValidateInterceptor()] });
  const deps = { db: options.db, apiURL: apiURL(options.baseURL), now: options.now };
  router.service(ServerService, serverService(deps));
  router.service(AccountService, accountService({ ...deps, auth: options.auth }));
  router.service(FriendService, friendService({ ...deps, auth: options.auth }));
  router.service(HomeService, homeService({ ...deps, auth: options.auth }));
  const handler = router.handlers.find((h) => `/api${h.requestPath}` === pathname);
  if (!handler) return new Response("Not Found", { status: 404 });
  return createFetchHandler(handler)(req);
}

// mavio.dev's Worker: the API under /api, the pages of TanStack Start, and
// the Durable Objects of the push to servers (docs/mavio-dev.md §2, §3).
import handler from "@tanstack/react-start/server-entry";

import { serveApi } from "./server/serve.ts";

export { NoticeHub } from "./server/notice-hub.ts";

export default {
  async fetch(req: Request, _env: unknown, ctx: ExecutionContext): Promise<Response> {
    if (new URL(req.url).pathname.startsWith("/api/")) return serveApi(req, ctx);
    return handler.fetch(req);
  },
};

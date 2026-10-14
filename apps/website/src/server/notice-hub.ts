// The Durable Object of each server in mavio.dev mode (docs/mavio-dev.md
// §3): it holds the server's WebSocket, hibernating between notices, and
// tells the server that its changes reached a cursor. The runtime answers
// the server's protocol pings without waking it.
import { create, toBinary } from "@bufbuild/protobuf";
import { NoticeSchema } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { DurableObject } from "cloudflare:workers";

/** NoticeHub holds one server's WebSocket. */
export class NoticeHub extends DurableObject {
  /** fetch accepts the server's WebSocket, which the Worker has authenticated, replacing an older one. */
  async fetch(req: Request): Promise<Response> {
    if (req.headers.get("upgrade")?.toLowerCase() !== "websocket") return new Response("Expected a WebSocket", { status: 426 });
    for (const ws of this.ctx.getWebSockets()) ws.close(1000, "replaced by a new connection");
    const pair = new WebSocketPair();
    this.ctx.acceptWebSocket(pair[1]);
    return new Response(null, { status: 101, webSocket: pair[0] });
  }

  /** notify tells the server that its changes reached cursor. */
  notify(cursor: number): void {
    const frame = toBinary(NoticeSchema, create(NoticeSchema, { cursor: BigInt(cursor) }));
    for (const ws of this.ctx.getWebSockets()) {
      try {
        ws.send(frame);
      } catch {
        // A closed connection; the server fetches its changes when it reconnects.
      }
    }
  }

  /** webSocketMessage ignores messages: servers send only pings. */
  webSocketMessage(): void {}

  webSocketClose(ws: WebSocket, code: number, reason: string): void {
    ws.close(code, reason);
  }
}

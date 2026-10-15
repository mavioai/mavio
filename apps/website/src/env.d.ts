// The bindings of mavio.dev's Worker (wrangler.jsonc), and the parts of the
// Workers runtime it uses.
declare module "cloudflare:workers" {
  export const env: {
    HYPERDRIVE: { connectionString: string };
    BASE_URL: string;
    /** app.mavio.dev's origin, by default https://app.mavio.dev. */
    APP_URL?: string;
    BETTER_AUTH_SECRET: string;
    /** The domain of servers' ACME challenges' TXT records, such as acme.mavio.dev. */
    ACME_DOMAIN?: string;
    /** The Cloudflare zone holding ACME_DOMAIN, and a token allowed to edit its DNS. */
    CLOUDFLARE_ZONE_ID?: string;
    CLOUDFLARE_API_TOKEN?: string;
    /** The Google Cloud project minting Google Trust Services' bindings, and its service account's key file. */
    GOOGLE_CA_PROJECT?: string;
    GOOGLE_CA_CREDENTIALS?: string;
    /** The relays' base URLs by region as a JSON object, such as {"eu":"https://relay-eu.mavio.dev"}. */
    RELAYS?: string;
    /** The secret of relay tickets, shared with the relays. */
    RELAY_TICKET_SECRET?: string;
    NOTICES: DurableObjectNamespace<import("./server/notice-hub.ts").NoticeHub>;
  };

  /** DurableObject is the base class of Durable Objects. */
  export abstract class DurableObject<Env = unknown> {
    protected ctx: DurableObjectState;
    protected env: Env;
    constructor(ctx: DurableObjectState, env: Env);
  }
}

interface DurableObjectNamespace<T> {
  idFromName(name: string): DurableObjectId;
  get(id: DurableObjectId): DurableObjectStub<T>;
}

interface DurableObjectId {
  toString(): string;
}

/** DurableObjectStub calls a Durable Object's methods over RPC. */
type DurableObjectStub<T> = {
  [K in keyof T]: T[K] extends (...args: infer A) => infer R ? (...args: A) => Promise<Awaited<R>> : never;
} & { fetch(req: Request): Promise<Response> };

interface DurableObjectState {
  acceptWebSocket(ws: WebSocket, tags?: string[]): void;
  getWebSockets(tag?: string): WebSocket[];
}

declare class WebSocketPair {
  0: WebSocket;
  1: WebSocket;
}

interface ResponseInit {
  webSocket?: WebSocket | null;
}

interface ExecutionContext {
  waitUntil(promise: Promise<unknown>): void;
}

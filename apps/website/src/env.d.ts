// The bindings of mavio.dev's Worker (wrangler.jsonc), and the parts of the
// Workers runtime it uses.
declare module "cloudflare:workers" {
  export const env: {
    HYPERDRIVE: { connectionString: string };
    BASE_URL: string;
    BETTER_AUTH_SECRET: string;
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

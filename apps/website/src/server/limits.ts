// Rate limits of the API (docs/mavio-dev.md §2): what guesses passwords,
// codes or PINs, and what sends e-mails, is limited per client address,
// through the Worker's rate limiting bindings.
import { cors } from "./authorization.ts";

/** Bucket names what a request is limited as. */
export type Bucket = "credentials" | "mail" | "codes";

/** Limit tells whether a client's request of a bucket may go on. */
export type Limit = (bucket: Bucket, client: string) => Promise<boolean>;

// better-auth's endpoints taking a password, a code or a PIN.
const credentials = [
  "/api/auth/sign-in/",
  "/api/auth/sign-up/",
  "/api/auth/two-factor/verify-",
  "/api/auth/home/switch",
  "/api/auth/reset-password",
  "/api/auth/change-password",
];

// Those sending e-mails.
const mail = ["/api/auth/request-password-reset", "/api/auth/send-verification-email"];

// Link codes and devices' sessions, guessed or polled for.
const codes = [
  "/api/oauth/device",
  "/api/oauth/token",
  "/api/mavio.cloud.v1.AccountService/GetLinkCode",
  "/api/mavio.cloud.v1.AccountService/AnswerLinkCode",
];

/** bucketOf returns the bucket a request is limited as, undefined when it is not. */
export function bucketOf(method: string, pathname: string): Bucket | undefined {
  if (method !== "POST") return undefined;
  const under = (prefixes: string[]) => prefixes.some((p) => pathname.startsWith(p));
  if (under(mail)) return "mail";
  if (under(credentials)) return "credentials";
  if (under(codes)) return "codes";
  return undefined;
}

/** clientOf returns the address a request came from, as Cloudflare saw it. */
export function clientOf(req: Request): string {
  return req.headers.get("cf-connecting-ip") ?? "unknown";
}

/** limited answers a request over its limit in the form its caller reads errors in. */
export function limited(req: Request, pathname: string): Response {
  const headers = { ...cors(req), "retry-after": "60" };
  if (pathname.startsWith("/api/mavio.")) {
    return Response.json({ code: "resource_exhausted", message: "too many requests" }, { status: 429, headers });
  }
  if (pathname.startsWith("/api/oauth/")) {
    return Response.json({ error: "slow_down", error_description: "too many requests" }, { status: 429, headers });
  }
  return Response.json({ code: "TOO_MANY_REQUESTS", message: "Too many requests. Try again in a minute." }, { status: 429, headers });
}

/** RateLimiter is what a Worker's rate limiting binding is used for. */
export interface RateLimiter {
  limit(options: { key: string }): Promise<{ success: boolean }>;
}

/** RateLimiters are the Worker's rate limiting bindings, one a bucket. */
export type RateLimiters = Record<Bucket, RateLimiter>;

/** bindingLimit returns a Limit over the Worker's bindings. */
export function bindingLimit(limiters: RateLimiters): Limit {
  return async (bucket, client) => (await limiters[bucket].limit({ key: `${bucket}:${client}` })).success;
}

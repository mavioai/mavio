// The bindings of mavio.dev's Worker (wrangler.jsonc).
declare module "cloudflare:workers" {
  export const env: {
    HYPERDRIVE: { connectionString: string };
    BASE_URL: string;
    BETTER_AUTH_SECRET: string;
  };
}

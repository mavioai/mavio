// The Mavio plugin SDK for TypeScript: child-process plugins running on
// Node or Bun serve the services of mavio.plugin.v1 (from @mavio/proto)
// with serve and reach the host API through hostTransport.
export { dataDir, env, hostOS, hostTransport, hostURL, httpPrefix, loopback, protocolVersion, serve } from "./process.ts";
export type { ServeOptions } from "./process.ts";

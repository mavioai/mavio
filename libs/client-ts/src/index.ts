// The web UI's client of the Mavio API (docs/clients.md §4.1): Connect
// clients over connect-es, one per server the user is signed in to, and
// the TanStack Query hooks the screens use.
export { type Server, transport } from "./server.ts";

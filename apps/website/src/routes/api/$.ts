import { createFileRoute } from "@tanstack/react-router";

import { serveApi } from "../../server/serve.ts";

const handler = ({ request }: { request: Request }) => serveApi(request);

export const Route = createFileRoute("/api/$")({
  server: { handlers: { GET: handler, POST: handler, OPTIONS: handler } },
});

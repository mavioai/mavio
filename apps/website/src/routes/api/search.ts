import { createFileRoute } from "@tanstack/react-router";
import { createFromSource } from "fumadocs-core/search/server";

import { source } from "../../lib/docs.ts";

// The documentation's search: fumadocs' built-in search over every language.
// A page's languages share its URL, so each is indexed under its language too.
const search = createFromSource(source, {
  buildIndex: async (page) => {
    const { structuredData } = await page.data.load();
    return { id: `${page.locale}:${page.url}`, url: page.url, title: page.data.title ?? page.url, description: page.data.description, structuredData };
  },
});

export const Route = createFileRoute("/api/search")({
  server: { handlers: { GET: ({ request }) => search.GET(request) } },
});

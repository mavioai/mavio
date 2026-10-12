// A fake Mavio server for the snapshot tests: the browser's requests to
// http://mavio.test are answered by the test, the built web UI from
// apps/server/internal/webui/dist and every call from the fixed data of
// the screens, so that a screen draws the same on every run.
import { readFile } from "node:fs/promises";
import { extname, join } from "node:path";

import { create, type DescMessage, type DescService, type MessageInitShape, toJsonString } from "@bufbuild/protobuf";
import type { Page, Route } from "@playwright/test";

/** The origin the web UI is served from and every server call goes to. */
export const origin = "http://mavio.test";

const dist = join(import.meta.dirname, "../../server/internal/webui/dist");

const types: Record<string, string> = {
  ".html": "text/html",
  ".js": "text/javascript",
  ".css": "text/css",
  ".svg": "image/svg+xml",
  ".woff2": "font/woff2",
  ".png": "image/png",
  ".ico": "image/x-icon",
  ".json": "application/json",
};

type Answer = (request: unknown) => MessageInitShape<DescMessage>;

/** The answers of a fake server, by service and method. */
export class Fixtures {
  private answers = new Map<string, Answer>();

  /** Answers the calls of a method with what answer returns for each. */
  on<S extends DescService, M extends keyof S["method"] & string>(
    service: S,
    method: M,
    answer: (
      request: S["method"][M] extends { input: infer I extends DescMessage } ? MessageInitShape<I> : never,
    ) => S["method"][M] extends { output: infer O extends DescMessage } ? MessageInitShape<O> : never,
  ): this {
    const m = service.method[method] as { name: string };
    this.answers.set(`${service.typeName}/${m.name}`, answer as Answer);
    return this;
  }

  /** The JSON of the answer to a call; an empty message for those without one. */
  answer(service: DescService, method: string, request: unknown): string {
    const desc = service.methods.find((m) => m.name === method);
    if (!desc) throw new Error(`no method ${service.typeName}/${method}`);
    const init = this.answers.get(`${service.typeName}/${method}`)?.(request) ?? {};
    return toJsonString(desc.output, create(desc.output, init));
  }
}

/** Every service the web UI calls, by type name. */
export type Services = Record<string, DescService>;

/**
 * Serves the web UI and the fixtures to page. Server streams end at once,
 * so that the event stream reconnects in the background as it would after
 * the server restarts; nothing is drawn from it.
 */
export async function serve(page: Page, services: Services, fixtures: Fixtures) {
  await page.route(`${origin}/**`, async (route: Route) => {
    const url = new URL(route.request().url());
    const [, service, method] = url.pathname.split("/");
    const svc = services[service];
    if (svc && method) {
      const desc = svc.methods.find((m) => m.name === method);
      if (desc?.methodKind === "server_streaming") {
        // An end-of-stream envelope alone: flags 2, then its length.
        const end = Buffer.from("{}");
        const head = Buffer.from([2, 0, 0, 0, end.length]);
        return route.fulfill({ contentType: "application/connect+json", body: Buffer.concat([head, end]) });
      }
      const request = route.request().postDataJSON() as unknown;
      return route.fulfill({ contentType: "application/json", body: fixtures.answer(svc, method, request) });
    }
    if (url.pathname.startsWith("/images/")) {
      return route.fulfill({ contentType: "image/svg+xml", body: artwork(url.pathname.slice("/images/".length)) });
    }
    const file = extname(url.pathname) ? url.pathname : "/index.html";
    try {
      const body = await readFile(join(dist, file));
      return route.fulfill({ contentType: types[extname(file)] ?? "application/octet-stream", body });
    } catch {
      return route.fulfill({ status: 404 });
    }
  });
}

/** An image of an ID: a gradient whose hues follow from the ID. */
function artwork(id: string): string {
  let h = 0;
  for (const c of id) h = (h * 31 + c.charCodeAt(0)) % 360;
  return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 2 3" preserveAspectRatio="none">
<defs><linearGradient id="g" x1="0" y1="0" x2="1" y2="1">
<stop offset="0" stop-color="hsl(${h} 55% 45%)"/><stop offset="1" stop-color="hsl(${(h + 60) % 360} 55% 25%)"/>
</linearGradient></defs><rect width="2" height="3" fill="url(#g)"/></svg>`;
}

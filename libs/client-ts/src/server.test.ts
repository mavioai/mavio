import { createClient } from "@connectrpc/connect";
import { SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
import { afterEach, expect, test, vi } from "vitest";

import { transport } from "./server.ts";

afterEach(() => vi.unstubAllGlobals());

test("calls the server with the user's token", async () => {
  const fetch = vi.fn(async (_url: string, _init: RequestInit) => Response.json({}));
  vi.stubGlobal("fetch", fetch);
  const client = createClient(SystemService, transport({ url: "https://media.example.com", token: "secret" }));
  await client.getHealth({});
  const [url, init] = fetch.mock.calls[0];
  expect(url).toBe("https://media.example.com/mavio.system.v1.SystemService/GetHealth");
  expect(new Headers(init.headers).get("authorization")).toBe("Bearer secret");
});

test("asks for the UI's language", async () => {
  const fetch = vi.fn(async (_url: string, _init: RequestInit) => Response.json({}));
  vi.stubGlobal("fetch", fetch);
  await createClient(SystemService, transport({ url: "https://media.example.com" }, undefined, "zh-Hans")).getHealth({});
  await createClient(SystemService, transport({ url: "https://media.example.com" })).getHealth({});
  expect(new Headers(fetch.mock.calls[0][1].headers).get("accept-language")).toBe("zh-Hans");
  expect(new Headers(fetch.mock.calls[1][1].headers).get("accept-language")).toBeNull();
});

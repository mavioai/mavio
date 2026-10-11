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

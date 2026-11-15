import { createClient } from "@connectrpc/connect";
import { SystemService } from "@mavio/proto/mavio/system/v1/system_pb";
import { afterEach, expect, test, vi } from "vitest";

import { isLocalAddress, transport } from "./server.ts";

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

// The cases of the core's local_addresses test (libs/client-rust/core/src/connection.rs).
test.each([
  ["192.168.31.147:8688", true],
  ["https://10.0.0.2", true],
  ["http://172.20.1.1:8686", true],
  ["127.0.0.1", true],
  ["169.254.3.4", true],
  ["http://[::1]:8686", true],
  ["http://[fd12:3456::1]:8686", true],
  ["http://[fe80::1]:8686", true],
  ["http://[::ffff:192.168.1.2]", true],
  ["localhost:8686", true],
  ["media.local", true],
  ["https://192-168-1-10.abcdefghijklmnopqrstuvwxyz234567.s.mavio.fyi", true],
  ["https://fd12-3456--1.abcdefghijklmnopqrstuvwxyz234567.s.mavio.fyi", true],
  ["https://203-0-113-5.abcdefghijklmnopqrstuvwxyz234567.s.mavio.fyi", false],
  ["https://relay-eu-abc.abcdefghijklmnopqrstuvwxyz234567.s.mavio.fyi", false],
  ["https://media.example.com", false],
  ["203.0.113.5", false],
  ["100.64.0.1", false],
  ["172.32.0.1", false],
  ["http://[2001:db8::1]:8686", false],
  ["http://[::ffff:8.8.8.8]", false],
  ["ftp://192.168.1.2", false],
  ["", false],
])("isLocalAddress(%j) = %s", (address, want) => {
  expect(isLocalAddress(address)).toBe(want);
});

import { describe, expect, test } from "vitest";

import { claimCodeHash, newClaimCode } from "../src/server/claim-codes.ts";
import { cookieDomain, requestLocale } from "../src/lib/i18n.ts";
import { expectedOrigin } from "../src/lib/origins.ts";
import { base32, serverID } from "../src/server/ids.ts";
import * as password from "../src/server/password.ts";

describe("password", () => {
  test("hashes in PHC format and verifies", async () => {
    const h = await password.hash("correct horse");
    expect(h).toMatch(/^\$scrypt\$ln=14,r=16,p=1\$[A-Za-z0-9+/]{22}\$[A-Za-z0-9+/]{86}$/);
    expect(await password.verify({ hash: h, password: "correct horse" })).toBe(true);
    expect(await password.verify({ hash: h, password: "wrong horse" })).toBe(false);
    expect(password.outdated(h)).toBe(false);
  });
  test("verifies older parameters and reports them outdated", async () => {
    const old = (await password.hash("pw")).replace("ln=14", "ln=10");
    expect(password.outdated(old)).toBe(true);
    expect(await password.verify({ hash: "$scrypt$garbage", password: "pw" })).toBe(false);
  });
});

describe("server IDs", () => {
  test("encode base32 as RFC 4648 in lowercase", () => {
    const enc = (s: string) => base32(new TextEncoder().encode(s));
    expect(["", "f", "fo", "foo", "foob", "fooba", "foobar"].map(enc)).toEqual([
      "",
      "my",
      "mzxq",
      "mzxw6",
      "mzxw6yq",
      "mzxw6ytb",
      "mzxw6ytboi",
    ]);
  });
  test("are the first 20 bytes of the key's SHA-256", async () => {
    // As the server computes it (apps/server/internal/identity).
    expect(await serverID(new Uint8Array(32))).toBe("mzuhvlpymk6xo3epygfy5h4oeaejofef");
  });
});

test("claim codes ignore case, spaces and dashes", async () => {
  const code = newClaimCode();
  expect(code).toMatch(/^[A-Z2-9]{4}-[A-Z2-9]{4}$/);
  expect(await claimCodeHash(code.toLowerCase().replace("-", " "))).toBe(await claimCodeHash(code));
});

describe("expectedOrigin", () => {
  const id = "abcdefghijklmnopqrstuvwxyz234567";
  test.each([
    [`https://relay-eu-ticket.${id}.s.mavio.fyi/sign-in`, true],
    [`https://lan-192-168-1-2.${id}.s.mavio.fyi`, true],
    [`https://evil.example/${id}.s.mavio.fyi`, false],
    [`https://x.other.s.mavio.fyi`, false],
    ["http://localhost:8686", true],
    ["http://nas.local:8686", true],
    ["http://192.168.1.20:8686", true],
    ["http://10.0.0.2", true],
    ["http://172.20.0.2", true],
    ["http://172.32.0.2", false],
    ["http://[::1]:8686", true],
    ["http://[fd00::2]:8686", true],
    ["http://[2001:db8::1]", false],
    ["https://media.example.com", false],
    ["not a url", false],
  ])("%s is %s", (uri, want) => {
    expect(expectedOrigin(uri, id)).toBe(want);
  });
});

describe("requestLocale", () => {
  test.each([
    { name: "Accept-Language alone", req: { acceptLanguage: "fr, zh-TW;q=0.8" }, want: "zh-Hans" },
    { name: "the cookie over Accept-Language", req: { cookies: "a=1; mavio-locale=en", acceptLanguage: "zh-CN" }, want: "en" },
    { name: "ui_locales over the cookie", req: { uiLocales: "fr zh-Hans", cookies: "mavio-locale=en" }, want: "zh-Hans" },
    { name: "ui_locales the strings lack", req: { uiLocales: "fr", cookies: "mavio-locale=zh-Hans" }, want: "zh-Hans" },
    { name: "a cookie the strings lack", req: { cookies: "mavio-locale=fr", acceptLanguage: "zh" }, want: "zh-Hans" },
    { name: "nothing", req: {}, want: "en" },
  ])("$name", ({ req, want }) => {
    expect(requestLocale(req)).toBe(want);
  });
});

describe("cookieDomain", () => {
  test.each([
    ["mavio.dev", "mavio.dev"],
    ["app.mavio.dev", "mavio.dev"],
    ["localhost:5173", undefined],
    ["192.168.2.6:5175", undefined],
    ["[::1]:5173", undefined],
  ])("%s", (host, want) => {
    expect(cookieDomain(host)).toBe(want);
  });
});

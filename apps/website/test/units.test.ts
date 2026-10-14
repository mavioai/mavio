import { describe, expect, test } from "vitest";

import { claimCodeHash, newClaimCode } from "../src/server/claim-codes.ts";
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

import { Code } from "@connectrpc/connect";
import { describe, expect, test } from "vitest";

import { cloudflareDns, googleBindings } from "../src/server/acme.ts";
import { claimed, code, newServer } from "./harness.ts";

const value = "a".repeat(43);

describe("ACME", () => {
  test("challenges are published for the server's own ID only", async () => {
    const { site, server, api } = await claimed();
    await api.setAcmeChallenge({ values: [value] });
    expect([...site.txt]).toEqual([[`${server.id}.acme.mavio.dev`, [value]]]);
    await api.clearAcmeChallenge({});
    expect(site.txt.size).toBe(0);
    expect(await code(api.setAcmeChallenge({ values: ["not a digest"] }))).toBe(Code.InvalidArgument);
    expect(await code(site.serverClient(await newServer()).setAcmeChallenge({ values: [value] }))).toBe(Code.Unauthenticated);
  });

  test("bindings are minted for claimed servers", async () => {
    const { api } = await claimed();
    const binding = await api.createAcmeBinding({});
    expect([binding.keyId, [...binding.hmacKey]]).toEqual(["kid-1", [1, 2, 3]]);
  });

  test("Cloudflare's TXT records are replaced", async () => {
    const calls: string[] = [];
    const dns = cloudflareDns("token", "zone", async (input, init) => {
      const url = new URL(input.toString());
      calls.push(`${init?.method ?? "GET"} ${url.pathname}${url.search} ${init?.body ?? ""}`.trim());
      expect(new Headers(init?.headers).get("authorization")).toBe("Bearer token");
      const result = init?.method ? {} : [{ id: "old" }];
      return Response.json({ success: true, result });
    });
    await dns.set("id.acme.mavio.dev", [value]);
    expect(calls).toEqual([
      "GET /client/v4/zones/zone/dns_records?type=TXT&name.exact=id.acme.mavio.dev",
      "DELETE /client/v4/zones/zone/dns_records/old",
      `POST /client/v4/zones/zone/dns_records {"type":"TXT","name":"id.acme.mavio.dev","content":"\\"${value}\\"","ttl":60}`,
    ]);
  });

  test("Google's bindings are created with a service account's token", async () => {
    const keys = (await crypto.subtle.generateKey(
      { name: "RSASSA-PKCS1-v1_5", modulusLength: 2048, publicExponent: new Uint8Array([1, 0, 1]), hash: "SHA-256" },
      true,
      ["sign", "verify"],
    )) as CryptoKeyPair;
    const pkcs8 = Buffer.from(await crypto.subtle.exportKey("pkcs8", keys.privateKey)).toString("base64");
    const credentials = JSON.stringify({
      client_email: "ca@project.iam.gserviceaccount.com",
      private_key: `-----BEGIN PRIVATE KEY-----\n${pkcs8}\n-----END PRIVATE KEY-----\n`,
    });
    const bindings = googleBindings(credentials, "project", async (input, init) => {
      const url = input.toString();
      if (url === "https://oauth2.googleapis.com/token") {
        const assertion = new URLSearchParams(init?.body as URLSearchParams).get("assertion")!;
        const [header, claims, sig] = assertion.split(".");
        const ok = await crypto.subtle.verify(
          "RSASSA-PKCS1-v1_5",
          keys.publicKey,
          Buffer.from(sig, "base64url"),
          new TextEncoder().encode(`${header}.${claims}`),
        );
        expect([ok, JSON.parse(Buffer.from(claims, "base64url").toString()).iss]).toEqual([true, "ca@project.iam.gserviceaccount.com"]);
        return Response.json({ access_token: "access" });
      }
      expect([url, new Headers(init?.headers).get("authorization")]).toEqual([
        "https://publicca.googleapis.com/v1/projects/project/locations/global/externalAccountKeys",
        "Bearer access",
      ]);
      return Response.json({ keyId: "kid", b64MacKey: Buffer.from([9, 8, 7]).toString("base64") });
    });
    const binding = await bindings.create();
    expect([binding.keyId, [...binding.hmacKey]]).toEqual(["kid", [9, 8, 7]]);
  });
});

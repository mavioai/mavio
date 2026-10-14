import { Code } from "@connectrpc/connect";
import { AccountService } from "@mavio/proto/mavio/cloud/v1/account_pb";
import { describe, expect, test } from "vitest";

import { s256 } from "../src/server/authorization.ts";
import { apiURL, claimed, code } from "./harness.ts";

const redirect = "http://192.168.1.20:8686/sign-in/mavio-dev";
const verifier = "a verifier of the web interface, long enough to be one";

describe("authorization codes", () => {
  test("are exchanged once with the verifier for an assertion of the device key", async () => {
    const { site, alice, asAlice, server } = await claimed();
    const device = new Uint8Array(32).fill(7);
    const challenge = await s256(verifier);
    const { code: authCode } = await asAlice.createAuthorizationCode({
      serverId: server.id,
      redirectUri: redirect,
      codeChallenge: challenge,
      deviceKey: device,
    });
    const exchange = (fields: Record<string, string>, origin = "http://192.168.1.20:8686") =>
      site.fetch(
        new Request(`${apiURL}/oauth/token`, {
          method: "POST",
          headers: { origin },
          body: new URLSearchParams({
            grant_type: "authorization_code",
            code: authCode,
            client_id: server.id,
            redirect_uri: redirect,
            code_verifier: verifier,
            ...fields,
          }),
        }),
      );

    // The code of another origin, client or verifier is refused, and spent.
    let res = await exchange({ code_verifier: "another verifier" });
    expect(res.status).toBe(400);
    expect((await res.json()).error).toBe("invalid_grant");
    res = await exchange({});
    expect(res.status).toBe(400);

    const { code: second } = await asAlice.createAuthorizationCode({
      serverId: server.id,
      redirectUri: redirect,
      codeChallenge: challenge,
      deviceKey: device,
    });
    res = await exchange({ code: second }, "https://evil.example");
    expect(res.status).toBe(400);

    const { code: third } = await asAlice.createAuthorizationCode({
      serverId: server.id,
      redirectUri: redirect,
      codeChallenge: challenge,
      deviceKey: device,
    });
    res = await exchange({ code: third });
    expect(res.status).toBe(200);
    expect(res.headers.get("access-control-allow-origin")).toBe("http://192.168.1.20:8686");
    const body = await res.json();
    expect(body.account).toMatchObject({ id: alice.id, userName: "alice" });
    expect(body.expires_in).toBe(120);
    const claims = JSON.parse(Buffer.from(body.assertion.split(".")[1], "base64url").toString());
    expect(claims).toMatchObject({ aud: server.id, sub: alice.id, device_key: "BwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwcHBwc" });
    expect((await exchange({ code: third })).status).toBe(400);
  });

  test("need access to the server and a web redirect URI", async () => {
    const { site, asAlice, server } = await claimed();
    const mallory = await site.signUp("mallory");
    const asMallory = site.client(AccountService, mallory.headers);
    const req = { serverId: server.id, redirectUri: redirect, codeChallenge: await s256(verifier), deviceKey: new Uint8Array(32) };
    expect(await code(asMallory.createAuthorizationCode(req))).toBe(Code.PermissionDenied);
    expect(await code(site.client(AccountService).createAuthorizationCode(req))).toBe(Code.Unauthenticated);
    expect(await code(asAlice.createAuthorizationCode({ ...req, redirectUri: "javascript:alert(1)" }))).toBe(Code.InvalidArgument);
    expect(await code(asAlice.createAuthorizationCode({ ...req, codeChallenge: "short" }))).toBe(Code.InvalidArgument);
  });
});

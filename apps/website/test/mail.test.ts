import { Code } from "@connectrpc/connect";
import { FriendService } from "@mavio/proto/mavio/cloud/v1/home_pb";
import { describe, expect, test } from "vitest";

import { bucketOf } from "../src/server/limits.ts";
import { resetMail, verificationMail } from "../src/server/mail.ts";
import { apiURL, baseURL, code, newSite } from "./harness.ts";

type Site = Awaited<ReturnType<typeof newSite>>;

function post(site: Site, path: string, body: unknown, headers: Record<string, string> = {}) {
  return site.fetch(
    new Request(`${apiURL}${path}`, { method: "POST", headers: { "content-type": "application/json", origin: baseURL, ...headers }, body: JSON.stringify(body) }),
  );
}

const carol = { email: "carol@example.com", password: "carol password", name: "Carol", username: "carol" };

describe("e-mail addresses", () => {
  test("sign in once verified by the link sent", async () => {
    const site = await newSite();
    expect((await post(site, "/auth/sign-up/email", { ...carol, callbackURL: "/" }, { "accept-language": "zh-CN,en;q=0.5" })).status).toBe(200);
    expect(site.mails.map((m) => [m.to, m.subject])).toEqual([[carol.email, "验证你的 Mavio 电子邮件地址"]]);
    // Not yet: signing in sends the link again.
    expect((await post(site, "/auth/sign-in/email", { email: carol.email, password: carol.password })).status).toBe(403);
    expect(site.mails).toHaveLength(2);
    const link = /https?:\/\/\S+verify-email\S+/.exec(site.mails[1].text)![0];
    const verified = await site.fetch(new Request(link));
    expect(verified.status).toBe(302);
    expect(verified.headers.get("set-cookie")).toMatch(/session_token/);
    expect((await post(site, "/auth/sign-in/username", { username: "carol", password: carol.password })).status).toBe(200);
  });

  test("find friends only once verified", async () => {
    const site = await newSite();
    const alice = await site.signUp("alice");
    await post(site, "/auth/sign-up/email", carol);
    const asAlice = site.client(FriendService, alice.headers);
    expect(await code(asAlice.addFriend({ name: carol.email }))).toBe(Code.NotFound);
    expect((await asAlice.addFriend({ name: "carol" })).friend?.account?.userName).toBe("carol");
  });
});

describe("passwords", () => {
  test("reset by the link sent, which ends the account's sessions", async () => {
    const site = await newSite();
    const alice = await site.signUp("alice");
    expect((await post(site, "/auth/request-password-reset", { email: "alice@example.com", redirectTo: `${baseURL}/reset-password` })).status).toBe(200);
    // An unknown address answers alike, sending nothing.
    expect((await post(site, "/auth/request-password-reset", { email: "nobody@example.com", redirectTo: `${baseURL}/reset-password` })).status).toBe(200);
    expect(site.mails.map((m) => [m.to, m.subject])).toEqual([["alice@example.com", "Reset your Mavio password"]]);
    const link = await site.fetch(new Request(/https?:\/\/\S+reset-password\/\S+/.exec(site.mails[0].text)![0]));
    const token = new URL(link.headers.get("location")!, baseURL).searchParams.get("token")!;
    expect((await post(site, "/auth/reset-password", { token, newPassword: "a new password" })).status).toBe(200);
    expect((await post(site, "/auth/reset-password", { token, newPassword: "another password" })).status).toBe(400);
    expect((await site.fetch(new Request(`${apiURL}/auth/get-session`, { headers: alice.headers }))).status).toBe(200);
    expect(await (await site.fetch(new Request(`${apiURL}/auth/get-session`, { headers: alice.headers }))).json()).toBeNull();
    expect((await post(site, "/auth/sign-in/email", { email: "alice@example.com", password: "a new password" })).status).toBe(200);
  });
});

describe("rate limits", () => {
  test("limit guessing and e-mails per client", async () => {
    const site = await newSite();
    site.allowance.set("credentials", 1);
    const signIn = () => post(site, "/auth/sign-in/username", { username: "nobody", password: "wrong" });
    expect((await signIn()).status).toBe(401);
    const res = await signIn();
    expect([res.status, res.headers.get("retry-after")]).toEqual([429, "60"]);
    // A Connect call is refused as Connect clients read it.
    site.allowance.set("codes", 0);
    const tv = await site.fetch(
      new Request(`${apiURL}/mavio.cloud.v1.AccountService/GetLinkCode`, { method: "POST", headers: { "content-type": "application/json" }, body: "{}" }),
    );
    expect([tv.status, (await tv.json()).code]).toEqual([429, "resource_exhausted"]);
  });

  test("bucket the requests that guess, poll or send", () => {
    const cases: [string, string, string | undefined][] = [
      ["POST", "/api/auth/sign-in/email", "credentials"],
      ["POST", "/api/auth/two-factor/verify-totp", "credentials"],
      ["POST", "/api/auth/home/switch", "credentials"],
      ["POST", "/api/auth/request-password-reset", "mail"],
      ["POST", "/api/auth/send-verification-email", "mail"],
      ["POST", "/api/oauth/token", "codes"],
      ["POST", "/api/mavio.cloud.v1.AccountService/AnswerLinkCode", "codes"],
      ["GET", "/api/auth/get-session", undefined],
      ["POST", "/api/mavio.cloud.v1.ServerService/ListChanges", undefined],
    ];
    for (const [method, path, want] of cases) expect(bucketOf(method, path), `${method} ${path}`).toBe(want);
  });
});

describe("mails", () => {
  test("are written in the request's language, their text escaped in HTML", () => {
    const m = verificationMail({ email: "a@example.com", name: "<b>Ann</b>" }, "https://mavio.dev/api/auth/verify-email?token=x&callbackURL=/", "en-US");
    expect(m.subject).toBe("Verify your e-mail address for Mavio");
    expect(m.text).toContain("Hi <b>Ann</b>,");
    expect(m.html).toContain("Hi &lt;b&gt;Ann&lt;/b&gt;,");
    expect(m.html).toContain('href="https://mavio.dev/api/auth/verify-email?token=x&amp;callbackURL=/"');
    expect(resetMail({ email: "a@example.com", name: "Ann" }, "https://x", "fr, zh-TW;q=0.8").subject).toBe("重设你的 Mavio 密码");
  });
});

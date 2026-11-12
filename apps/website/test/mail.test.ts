import { create } from "@bufbuild/protobuf";
import { Code } from "@connectrpc/connect";
import { AccessSchema } from "@mavio/proto/mavio/cloud/v1/cloud_pb";
import { Friend_State, FriendService, HomeService } from "@mavio/proto/mavio/cloud/v1/home_pb";
import { RestrictionProfile } from "@mavio/proto/mavio/user/v1/user_pb";
import { eq } from "drizzle-orm";
import { describe, expect, test, vi } from "vitest";

import { localeOf } from "../src/lib/i18n.ts";
import { schema } from "../src/server/db/index.ts";
import { bucketOf } from "../src/server/limits.ts";
import { friendRequestMail, homeInvitationMail, invitationMail, mailTo, resetMail, verificationMail } from "../src/server/mail.ts";
import { apiURL, baseURL, claimed, code, libraryA, newSite } from "./harness.ts";

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
    const m = verificationMail({ email: "a@example.com", name: "<b>Ann</b>" }, "https://mavio.dev/api/auth/verify-email?token=x&callbackURL=/", localeOf("en-US"));
    expect(m.subject).toBe("Verify your e-mail address for Mavio");
    expect(m.text).toContain("Hi <b>Ann</b>,");
    expect(m.text).toContain("Verify E-mail Address\nhttps://mavio.dev/api/auth/verify-email?token=x&callbackURL=/");
    expect(m.html).toContain("Hi &lt;b&gt;Ann&lt;/b&gt;,");
    expect(m.html).toContain('href="https://mavio.dev/api/auth/verify-email?token=x&amp;callbackURL=/"');
    expect(resetMail({ email: "a@example.com", name: "Ann" }, "https://x", localeOf("fr, zh-TW;q=0.8")).subject).toBe("重设你的 Mavio 密码");
  });

  test("tell of others' requests in the recipient's language, naming only who asked to what", () => {
    const ann = { email: "ann@example.com", name: "Ann" };
    const from = { name: "<i>Bo</i>\nBo", userName: "bo" };
    const cases = [
      {
        name: "an invitation to a server",
        mail: invitationMail(ann, from, "Den", "https://mavio.dev/", "en"),
        subject: "<i>Bo</i> Bo invited you to Den on Mavio",
        body: "(bo) invited you to their Mavio server, Den.",
        action: "See the Invitation\nhttps://mavio.dev/",
      },
      {
        name: "a friend request",
        mail: friendRequestMail(ann, from, "https://mavio.dev/friends", "zh-Hans"),
        subject: "<i>Bo</i> Bo 想在 Mavio 上加你为好友",
        body: "（bo）想在 Mavio 上加你为好友。",
        action: "查看请求\nhttps://mavio.dev/friends",
      },
      {
        name: "an invitation to a Home",
        mail: homeInvitationMail(ann, from, "Family", "https://mavio.dev/home", "en"),
        subject: "<i>Bo</i> Bo invited you to their Home on Mavio",
        body: "(bo) invited you to join their Home, Family, on Mavio.",
        action: "See the Invitation\nhttps://mavio.dev/home",
      },
    ];
    for (const c of cases) {
      expect(c.mail.to, c.name).toBe("ann@example.com");
      expect(c.mail.subject, c.name).toBe(c.subject);
      expect(c.mail.text, c.name).toContain(c.body);
      expect(c.mail.text, c.name).toContain(c.action);
      expect(c.mail.html, c.name).toContain("&lt;i&gt;Bo&lt;/i&gt;");
      expect(c.mail.html, c.name).not.toContain("<i>");
    }
  });

  test("go to full accounts with verified addresses, in the language they signed in with", () => {
    const u = { email: "ann@example.com", name: "Ann", emailVerified: true, kind: "full", locale: "zh-Hans" };
    expect(mailTo(u)).toEqual({ to: { email: "ann@example.com", name: "Ann" }, locale: "zh-Hans" });
    expect(mailTo({ ...u, locale: null })?.locale).toBe("en");
    expect(mailTo({ ...u, locale: "fr" })?.locale).toBe("en");
    expect(mailTo({ ...u, emailVerified: false })).toBeUndefined();
    expect(mailTo({ ...u, email: "managed:x", emailVerified: false, kind: "managed" })).toBeUndefined();
  });
});

describe("accounts' languages", () => {
  test("are those of the requests signing them up and in", async () => {
    const site = await newSite();
    const locale = async () => (await site.db.query.user.findFirst({ where: eq(schema.user.username, "carol") }))?.locale;
    await post(site, "/auth/sign-up/email", carol, { "accept-language": "zh-CN,en;q=0.5" });
    expect(await locale()).toBe("zh-Hans");
    await site.db.update(schema.user).set({ emailVerified: true });
    // The language chosen on the site comes before the browser's.
    const res = await post(site, "/auth/sign-in/username", { username: "carol", password: carol.password }, { "accept-language": "zh-CN", cookie: "mavio-locale=en" });
    expect(res.status).toBe(200);
    expect(await locale()).toBe("en");
  });
});

describe("telling accounts of requests", () => {
  const all = create(AccessSchema, { allLibraries: true });

  test("e-mails a new invitation to a server once, without its access", async () => {
    const { site, asAlice, server } = await claimed();
    await site.signUp("bob", { "accept-language": "zh-CN" });
    await asAlice.grantAccess({ serverId: server.id, userName: "bob", access: create(AccessSchema, { libraryIds: [libraryA] }) });
    expect(site.mails.map((m) => [m.to, m.subject])).toEqual([["bob@example.com", "alice 邀请你使用 Mavio 上的 Home"]]);
    expect(site.mails[0].text).toContain(`查看邀请\n${baseURL}/\n`);
    expect(site.mails[0].text).not.toContain("Movies");
    // Changing the invitation's access, or the friend request sent along, sends nothing more.
    await asAlice.grantAccess({ serverId: server.id, userName: "bob", access: all });
    expect(site.mails).toHaveLength(1);
  });

  test("e-mails a new friend request once", async () => {
    const site = await newSite();
    const [alice, bob] = [await site.signUp("alice"), await site.signUp("bob", { "accept-language": "en-GB" })];
    const [asAlice, asBob] = [site.client(FriendService, alice.headers), site.client(FriendService, bob.headers)];
    await asAlice.addFriend({ name: "bob" });
    await asAlice.addFriend({ name: "bob" });
    expect(site.mails.map((m) => [m.to, m.subject])).toEqual([["bob@example.com", "alice wants to be your friend on Mavio"]]);
    expect(site.mails[0].text).toContain(`See the Request\n${baseURL}/friends\n`);
    // Adding back accepts, telling no one.
    await asBob.addFriend({ name: "alice" });
    expect(site.mails).toHaveLength(1);
  });

  test("e-mails a new invitation to a Home once", async () => {
    const { site, alice, server } = await claimed();
    await site.signUp("bob", { "accept-language": "zh-CN" });
    await site.db.insert(schema.passes).values({ accountId: alice.id, expiresAt: new Date(Date.now() + 86_400_000) });
    const home = site.client(HomeService, alice.headers);
    await home.createHome({ name: "Family" });
    await home.inviteToHome({ userName: "bob", offers: [] });
    await home.inviteToHome({ userName: "bob", offers: [{ serverId: server.id, access: all }] });
    expect(site.mails.map((m) => [m.to, m.subject])).toEqual([["bob@example.com", "alice 邀请你加入其 Mavio 家庭"]]);
    expect(site.mails[0].text).toContain("“Family”");
    expect(site.mails[0].text).toContain(`查看邀请\n${baseURL}/home\n`);
  });

  test("e-mails no account without a verified address", async () => {
    const { site, alice, asAlice, server } = await claimed();
    await post(site, "/auth/sign-up/email", carol);
    site.mails.length = 0;
    await site.client(FriendService, alice.headers).addFriend({ name: "carol" });
    await asAlice.grantAccess({ serverId: server.id, userName: "carol", access: all });
    // Nor a managed account, whose grants take effect at once.
    const home = site.client(HomeService, alice.headers);
    await home.createHome({ name: "Family" });
    await home.createManagedAccount({ userName: "kid", displayName: "Kid", restriction: RestrictionProfile.YOUNGER_CHILD });
    await asAlice.grantAccess({ serverId: server.id, userName: "kid", access: all });
    expect(site.mails).toEqual([]);
  });

  test("answer the request when the e-mail cannot be sent", async () => {
    const site = await newSite();
    const alice = await site.signUp("alice");
    await site.signUp("bob");
    site.mailFails = true;
    const logged = vi.spyOn(console, "error").mockImplementation(() => {});
    try {
      const res = await site.client(FriendService, alice.headers).addFriend({ name: "bob" });
      expect(res.friend?.state).toBe(Friend_State.SENT);
      expect(logged).toHaveBeenCalledOnce();
    } finally {
      logged.mockRestore();
    }
  });
});

// mavio.dev's privacy policy and terms of service, in each language of the
// site: sections of paragraphs and lists, drawn as the site's pages.
import type { Locale } from "./lib/i18n.ts";
import { useLocale, useT } from "./lib/locale.tsx";

/** A section of a policy: its title, and paragraphs and lists. */
interface Section {
  title: string;
  body: (string | string[])[];
}

/** A policy: when it last changed and its sections. */
interface Policy {
  updated: string;
  sections: Section[];
}

/** The address users write to. */
const support = "support@mavio.dev";

const privacy: Record<Locale, Policy> = {
  en: {
    updated: "2026-10-27",
    sections: [
      {
        title: "Who we are",
        body: [
          `mavio.dev is run by an individual, the developer of Mavio ("we"). This policy explains what mavio.dev's website and services collect and why. Write to ${support} with any question about it.`,
          "Mavio's servers are software you run yourself. What a server stores — your libraries, your watch history, the media itself — stays on that server and is not sent to us; this policy covers only what mavio.dev handles.",
        ],
      },
      {
        title: "What we collect",
        body: [
          [
            "Your account: e-mail address, user name, display name, a hash of your password, your passkeys' public keys, your two-factor secret, a hash of your PIN, and the profiles of your Home's managed accounts.",
            "Sessions: for each signed-in browser or device, when it signed in, its IP address and its user agent, and the public keys of devices' sessions.",
            "Your servers: each server's identity public key, name and version, the IDs, names and kinds of its libraries, the addresses your clients reach it at, and the access you grant others to it.",
            "Your relationships: your friends, your Home and its members, and invitations.",
            "Mavio Pass: when your Pass runs until, and your subscription's state, plan and period. Payments are taken by Stripe; we receive a customer ID and the subscription's state, never your card's details.",
            "Requests: your IP address, used to limit how often passwords, codes and e-mail can be tried, for a minute at a time.",
          ],
        ],
      },
      {
        title: "How we use it",
        body: [
          [
            "To sign you in, to your servers and on your devices, and to keep your account secure.",
            "To tell your servers who may reach them and when access changes, an account is suspended or deleted.",
            "To issue certificates and names for your servers, and to relay connections to them when they cannot be reached directly.",
            "To send you e-mail about your account, such as verifying your address or resetting your password. We send no marketing e-mail.",
            "To sell and renew Mavio Pass.",
          ],
          "We do not sell your data, show advertising or use analytics or tracking cookies. The only cookie is the one that keeps you signed in.",
        ],
      },
      {
        title: "The relay",
        body: [
          "When a connection goes through mavio.dev's relay, it stays encrypted end to end between your device and your server. The relay sees which server is reached, the IP addresses at both ends and how much data passes, and keeps none of it after the connection ends; it cannot see what is watched.",
        ],
      },
      {
        title: "Who processes it for us",
        body: [
          [
            "Cloudflare runs mavio.dev's website, database connections, e-mail and DNS.",
            "Stripe takes payments for Mavio Pass, under its own privacy policy.",
            "Let's Encrypt and Google Trust Services issue your servers' certificates, which list their names publicly in Certificate Transparency logs.",
            "Virtual machines we rent run the relays and the name servers of s.mavio.fyi.",
          ],
          "These providers may process data in countries other than yours.",
        ],
      },
      {
        title: "Keeping and deleting it",
        body: [
          "We keep your data while your account exists. Deleting your account on its page deletes it, with your sessions, friendships, Home membership and access records, and tells your servers to remove you; a Pass subscription ends with it. Records that Stripe keeps for tax and accounting stay with Stripe.",
          `You can see and change most of your data on your account's pages. To get a copy of it or to have it corrected or deleted, write to ${support}.`,
        ],
      },
      {
        title: "Children",
        body: [
          "mavio.dev is not meant for children to sign up on their own. A Home's administrator may create managed accounts for their family; they have no e-mail address or password, and the administrator is responsible for them.",
        ],
      },
      {
        title: "Changes",
        body: ["When this policy changes, the date at its top changes too, and we tell account holders of significant changes by e-mail before they apply."],
      },
    ],
  },
  "zh-Hans": {
    updated: "2026-10-27",
    sections: [
      {
        title: "我们是谁",
        body: [
          `mavio.dev 由 Mavio 的开发者个人运营（下称“我们”）。本政策说明 mavio.dev 的网站和服务收集哪些信息以及原因。如有任何疑问，请写信至 ${support}。`,
          "Mavio 服务器是你自己运行的软件。服务器存储的内容——你的媒体库、观看记录以及媒体本身——都留在该服务器上，不会发送给我们；本政策只涉及 mavio.dev 处理的信息。",
        ],
      },
      {
        title: "我们收集的信息",
        body: [
          [
            "你的账户：电子邮件地址、用户名、显示名称、密码的哈希值、通行密钥的公钥、两步验证密钥、PIN 的哈希值，以及你的 Home 中受管理账户的资料。",
            "会话：每个已登录的浏览器或设备的登录时间、IP 地址和用户代理，以及设备会话的公钥。",
            "你的服务器：每台服务器的身份公钥、名称和版本，其媒体库的 ID、名称和类型，客户端访问它所用的地址，以及你授予他人的访问权限。",
            "你的关系：你的好友、你的 Home 及其成员，以及邀请。",
            "Mavio Pass：你的 Pass 的到期时间，以及订阅的状态、方案和周期。付款由 Stripe 处理；我们只收到客户 ID 和订阅状态，从不接触你的银行卡信息。",
            "请求：你的 IP 地址，用于限制密码、验证码和电子邮件的尝试频率，每次仅保留一分钟。",
          ],
        ],
      },
      {
        title: "我们如何使用这些信息",
        body: [
          [
            "让你登录，包括登录你的服务器和设备，并保障账户安全。",
            "告知你的服务器谁可以访问它，以及访问权限变更、账户被停用或删除的情况。",
            "为你的服务器签发证书和域名，并在无法直接访问时中继连接。",
            "向你发送与账户相关的电子邮件，例如验证地址或重置密码。我们不发送营销邮件。",
            "销售和续订 Mavio Pass。",
          ],
          "我们不出售你的数据，不展示广告，也不使用分析或跟踪 Cookie。唯一的 Cookie 用于保持你的登录状态。",
        ],
      },
      {
        title: "中继",
        body: [
          "通过 mavio.dev 中继的连接在你的设备与服务器之间始终保持端到端加密。中继能看到所访问的服务器、两端的 IP 地址和传输的数据量，连接结束后不保留任何记录；它无法看到观看的内容。",
        ],
      },
      {
        title: "为我们处理信息的服务商",
        body: [
          [
            "Cloudflare 运行 mavio.dev 的网站、数据库连接、电子邮件和 DNS。",
            "Stripe 处理 Mavio Pass 的付款，并适用其自己的隐私政策。",
            "Let's Encrypt 和 Google Trust Services 为你的服务器签发证书，证书中的域名会公开记录在证书透明度日志中。",
            "我们租用的虚拟机运行中继和 s.mavio.fyi 的域名服务器。",
          ],
          "这些服务商可能在你所在国家或地区以外处理数据。",
        ],
      },
      {
        title: "保留与删除",
        body: [
          "在你的账户存在期间，我们会保留你的数据。在账户页面删除账户即会删除这些数据，包括会话、好友关系、Home 成员身份和访问记录，并通知你的服务器移除你；Pass 订阅也会随之结束。Stripe 出于税务和会计目的保留的记录由 Stripe 保管。",
          `你可以在账户页面查看和更改大部分数据。如需获取数据副本、更正或删除数据，请写信至 ${support}。`,
        ],
      },
      {
        title: "儿童",
        body: ["mavio.dev 不面向儿童自行注册。Home 管理员可以为家人创建受管理账户；这些账户没有电子邮件地址和密码，由管理员负责。"],
      },
      {
        title: "变更",
        body: ["本政策变更时，顶部的日期也会随之更新；如有重大变更，我们会在生效前通过电子邮件通知账户持有人。"],
      },
    ],
  },
};

const terms: Record<Locale, Policy> = {
  en: {
    updated: "2026-10-27",
    sections: [
      {
        title: "These terms",
        body: [
          `These terms govern your use of mavio.dev's website and services: accounts, signing in to servers, certificates and names, the relay and Mavio Pass. mavio.dev is run by an individual, the developer of Mavio ("we"). By creating an account you agree to them; write to ${support} with any question.`,
          "Mavio's server and apps are free software, licensed under the GNU General Public License version 3; that license, not these terms, governs the software.",
        ],
      },
      {
        title: "Your account",
        body: [
          [
            "Give a working e-mail address and keep your password and devices safe; you are responsible for what is done with your account.",
            "You may create managed accounts for the people in your Home and are responsible for them.",
            "One person may not create accounts to get around a suspension or a limit.",
          ],
        ],
      },
      {
        title: "Your servers and media",
        body: [
          "You run your servers and choose what they hold and whom they share it with. You are responsible for having the rights to the media you store and share, and for following the law where you and the people you share with are.",
        ],
      },
      {
        title: "Acceptable use",
        body: [
          [
            "Do not use mavio.dev to break the law or to infringe others' rights.",
            "Do not attack, overload or probe mavio.dev, its relays or its name servers, or get around their limits.",
            "Do not use the relay or the names of s.mavio.fyi for anything but reaching your own Mavio servers and those shared with you.",
          ],
          "We may suspend an account that breaks these terms. A suspended account cannot sign in, and its servers end its sessions; write to us if you think a suspension is a mistake.",
        ],
      },
      {
        title: "Mavio Pass",
        body: [
          [
            "Mavio Pass is a subscription, paid each month or each year through Stripe at the price shown when you subscribe, plus any tax.",
            "It renews automatically until you cancel. You can cancel at any time from the Mavio Pass page; your Pass then lasts until the end of the period paid for.",
            "Payments are not refunded for the rest of a period, except where the law requires.",
            "If we change the price, we tell you by e-mail at least 30 days before your next renewal at the new price.",
            "What Mavio Pass includes is described on its page. Your servers and apps never depend on it.",
          ],
        ],
      },
      {
        title: "The service",
        body: [
          "We work to keep mavio.dev available but do not promise that it will always be, and we may change, add or remove features. If we stop running mavio.dev, we will tell account holders at least 90 days in advance and refund the unused part of Mavio Pass subscriptions; servers keep working in standalone mode.",
          `You can stop using mavio.dev and delete your account at any time.`,
        ],
      },
      {
        title: "Disclaimer and liability",
        body: [
          'mavio.dev is provided "as is", without warranties of any kind, to the extent the law allows. To the extent the law allows, we are not liable for indirect or consequential losses, or for loss of data, and our total liability to you is limited to what you paid us in the twelve months before the claim. Nothing in these terms limits liability that the law does not allow to be limited.',
        ],
      },
      {
        title: "Changes",
        body: ["When these terms change, the date at their top changes too, and we tell account holders of significant changes by e-mail before they apply. Using mavio.dev after they apply means you accept them."],
      },
    ],
  },
  "zh-Hans": {
    updated: "2026-10-27",
    sections: [
      {
        title: "关于本条款",
        body: [
          `本条款适用于你对 mavio.dev 网站和服务的使用，包括账户、登录服务器、证书和域名、中继以及 Mavio Pass。mavio.dev 由 Mavio 的开发者个人运营（下称“我们”）。创建账户即表示你同意本条款；如有疑问，请写信至 ${support}。`,
          "Mavio 的服务器和应用是自由软件，以 GNU 通用公共许可证第 3 版授权；软件受该许可证约束，而非本条款。",
        ],
      },
      {
        title: "你的账户",
        body: [
          [
            "请提供可用的电子邮件地址，并妥善保管密码和设备；你需对使用你账户进行的一切行为负责。",
            "你可以为 Home 中的家人创建受管理账户，并对其负责。",
            "不得为规避停用或限制而创建账户。",
          ],
        ],
      },
      {
        title: "你的服务器与媒体",
        body: ["你运行自己的服务器，并决定其中存放的内容以及与谁分享。你需确保对所存储和分享的媒体拥有相应权利，并遵守你和分享对象所在地的法律。"],
      },
      {
        title: "可接受的使用",
        body: [
          [
            "不得利用 mavio.dev 违反法律或侵犯他人权利。",
            "不得攻击、过载或探测 mavio.dev、其中继或域名服务器，也不得规避其限制。",
            "中继和 s.mavio.fyi 的域名仅可用于访问你自己的以及他人与你分享的 Mavio 服务器。",
          ],
          "对于违反本条款的账户，我们可以将其停用。被停用的账户无法登录，其在各服务器上的会话也会结束；如果你认为停用有误，请与我们联系。",
        ],
      },
      {
        title: "Mavio Pass",
        body: [
          [
            "Mavio Pass 是一项订阅服务，按月或按年通过 Stripe 付款，价格以订阅时显示的为准，另加适用的税费。",
            "订阅会自动续订，直到你取消为止。你可以随时在 Mavio Pass 页面取消；取消后，Pass 会持续到已付费周期结束。",
            "除法律另有规定外，当前周期的剩余部分不予退款。",
            "如果我们调整价格，会在你按新价格续订前至少 30 天通过电子邮件通知你。",
            "Mavio Pass 包含的内容以其页面所述为准。你的服务器和应用从不依赖它。",
          ],
        ],
      },
      {
        title: "服务",
        body: [
          "我们努力保持 mavio.dev 的可用性，但不保证其始终可用，并可能更改、新增或移除功能。如果我们停止运营 mavio.dev，会至少提前 90 天通知账户持有人，并退还 Mavio Pass 订阅未使用部分的费用；服务器可以继续以独立模式工作。",
          "你可以随时停止使用 mavio.dev 并删除账户。",
        ],
      },
      {
        title: "免责声明与责任限制",
        body: [
          "在法律允许的范围内，mavio.dev 按“现状”提供，不作任何形式的保证。在法律允许的范围内，我们不对间接或附带损失以及数据丢失承担责任，且我们对你的全部责任以你在索赔前十二个月内向我们支付的金额为限。本条款中的任何内容均不限制法律不允许限制的责任。",
        ],
      },
      {
        title: "变更",
        body: ["本条款变更时，顶部的日期也会随之更新；如有重大变更，我们会在生效前通过电子邮件通知账户持有人。变更生效后继续使用 mavio.dev 即表示你接受变更后的条款。"],
      },
    ],
  },
};

/** The privacy policy or the terms of service, in the page's language. */
export function LegalPage({ policy }: { policy: "privacy" | "terms" }) {
  const t = useT();
  const locale = useLocale();
  const p = (policy === "privacy" ? privacy : terms)[locale];
  const updated = new Date(`${p.updated}T00:00:00Z`).toLocaleDateString(locale, { dateStyle: "long", timeZone: "UTC" });
  return (
    <main className="mx-auto flex w-full max-w-[720px] flex-col gap-8 px-margin pt-6 pb-16">
      <header className="flex flex-col gap-2">
        <h1 className="type-title1">{t(policy === "privacy" ? "site.footer.privacy" : "site.footer.terms")}</h1>
        <p className="type-footnote text-text-secondary">{t("site.legal.updated", { date: updated })}</p>
      </header>
      {p.sections.map((s) => (
        <section key={s.title} className="flex flex-col gap-3">
          <h2 className="type-title3">{s.title}</h2>
          {s.body.map((b, i) =>
            typeof b === "string" ? (
              <p key={i} className="type-body text-text-secondary">
                {b}
              </p>
            ) : (
              <ul key={i} className="flex list-disc flex-col gap-2 pl-5 type-body text-text-secondary marker:text-text-tertiary">
                {b.map((item) => (
                  <li key={item}>{item}</li>
                ))}
              </ul>
            ),
          )}
        </section>
      ))}
    </main>
  );
}

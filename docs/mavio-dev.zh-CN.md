# mavio.dev

> [English](mavio-dev.md) | 简体中文

> 相关文档：[账户](accounts.zh-CN.md) · [系统架构](architecture.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> mavio.dev 模式服务端的中心服务：它运行什么、如何触达服务端、服务端如何获得名称和证书、中转，以及各部分部署在哪里。它保存什么以及它所服务的账户模型，见[账户](accounts.zh-CN.md)。在[路线图](roadmap.zh-CN.md)的 P28 阶段实现。

---

## 1. 概述

mavio.dev 是 mavio.dev 模式服务端的 OpenID Connect 签发方（[账户 §3.2](accounts.zh-CN.md#32-服务端与认领)）：它保存这些服务端的访问记录并向它们推送变更和吊销，用代码登录电视，并为它们提供中转、证书和发现。它由三个应用组成：

| 应用 | 内容 |
| :--- | :--- |
| `apps/website` | 网站及其 API：账户、服务端、访问记录、好友、Home、Mavio Pass、向服务端的推送和 ACME 质询（§2、§3） |
| `apps/dns` | `s.mavio.fyi` 的名称服务器（§4） |
| `apps/relay` | 供无法直接访问的服务端使用的中转（§5） |

## 2. 网站与 API（`apps/website`）
* 一个运行在 Cloudflare Workers 上的 TanStack Start 应用，用 Cloudflare 的 `cf` 命令行部署。页面使用 TanStack Query；数据保存在 PostgreSQL 中，通过 Hyperdrive 用基于 `pg` 的 Drizzle 访问，每个请求一个数据库客户端。模式在 `src/server/db` 中：better-auth 的表由它的 `auth` 命令行生成（`pnpm --filter @mavio/website generate-auth-schema`），另有 Mavio 自己的表；drizzle-kit 把它们的迁移写入 `drizzle/`（`generate-migration`）。API 位于 `/api` 下：better-auth 在 `/api/auth`，设备会话的 OAuth 端点在 `/api/oauth/token`、`/api/oauth/device` 和 `/api/oauth/revoke`（[账户 §6](accounts.zh-CN.md#6-登录)），站点配置在 `/api/config`，Connect 服务在 `/api/mavio.cloud.v1.*`，其请求用 protovalidate 检查。OAuth 端点、`/api/config`、`AccountService`、`FriendService` 和 `HomeService` 接受任意来源的请求，因为 app.mavio.dev 和服务端的 Web 界面会调用它们。
* 账户基于 better-auth：邮箱地址或用户名加密码、通行密钥、作为第二因素的 TOTP，bearer 会话，以及 JWT 插件，服务端获取其密钥集来检查登录断言。
* **邮件**通过 Cloudflare Email Service 从 `noreply@mavio.dev` 发送，语言按页面的方式取请求的语言，`ui_locales` 除外（英文或简体中文），同时有 HTML 与纯文本，字符串在 `libs/design/strings` 中（`mail.*`）。邮箱地址须验证后才能登录：注册时发送链接，以尚未验证的地址登录时拒绝并重新发送；打开链接即验证地址、登录账户并进入 `/verified`。`/forgot-password` 发送指向 `/reset-password` 的链接，后者设置新密码并结束账户的所有会话，包括设备会话。链接有效一小时，请求链接的页面不透露该地址是否有账户。新的服务端邀请、好友请求或 Home 邀请会以邮件告知被邀请的账户（其地址须已验证），语言为该账户最近一次注册或登录时的语言（记录在账户上），只写明是谁发出的以及（对邀请）哪个服务端或 Home，并附上答复它的页面的链接；发送失败只记入日志，请求照常生效。发往 `support@mavio.dev` 的邮件由 Cloudflare Email Routing 转发。
* **限流**按客户端地址，用 Worker 的限流绑定分三个桶：`credentials`，接受密码、TOTP 码或 PIN 的端点（登录与注册、验证 TOTP、切换、重置与修改密码）每分钟十次；`mail`，发送邮件的端点每分钟三次；`codes`，`/api/oauth/device`、`/api/oauth/token` 以及 `GetLinkCode` 与 `AnswerLinkCode` 每分钟三十次。超出限额的请求以 429 和 `Retry-After: 60` 回应，形式按调用方而定：Connect 为 `resource_exhausted`，OAuth 端点为 `slow_down`，其余为 better-auth 的 `TOO_MANY_REQUESTS`。
* **页面**与 Web UI 的绘制方式一致（[客户端 §6](clients.zh-CN.md#6-设计系统)）：登录页与设备应答页是海报墙上的一张卡片，账户的页面是带有服务端、Home、好友导航与账户菜单的页头，其下为成组的行，使用 libs/design 的颜色、字体与图标。字符串是 `libs/design/strings` 中 `website` 平台的字符串（`site.*`），语言取请求的语言：地址中的 OpenID Connect `ui_locales`（客户端以它把自己的语言传给 `/authorize`，随后由语言 cookie 保留）；否则为 `mavio-locale` cookie 中选定的语言（由页脚与卡片上的菜单设置，保留一年，选“跟随系统”即删除，并在网站的域名上与 app.mavio.dev 共享）；否则为其 `Accept-Language`。切换语言时页面重新渲染，不重新加载。
* **站点自身的页面**：`/` 对访客是介绍 Mavio 与 Mavio Pass 的首页，对已登录的账户是其服务端；`/privacy` 与 `/terms` 是隐私政策与服务条款，提供英文与简体中文（`src/legal.tsx`）；`/pass` 是 Mavio Pass。除卡片页与文档外，每个页面的末尾都有页脚，链接到它们、文档和 `support@mavio.dev`。站点只介绍账户、服务端、应用和 Mavio Pass，不介绍应用在应用商店中不能提供的内容。
* **文档**位于 `/docs`，用 fumadocs 构建：其页面是 `content/docs` 中的 MDX，英文为 `<name>.mdx`，简体中文为 `<name>.zh-Hans.mdx`，按 `meta.json` 与 `meta.zh-Hans.json` 排序，由 fumadocs-mdx 的 Vite 插件编译；与其他页面一样，语言取请求的语言，URL 中不含语言，fumadocs 的语言菜单与网站的菜单一样切换它。搜索是 fumadocs 自带的基于 ZBSearch 的搜索，位于 `/api/search`，按语言分别建立索引，并用 `Intl.Segmenter` 分词。fumadocs 的颜色即 libs/design 的颜色（`src/styles.css`）。
* **Mavio Pass 通过 Stripe Billing 购买**，每月 4 美元或每年 35 美元。Mavio 的 better-auth 插件提供 `GET /api/auth/pass/subscription`，即账户的订阅（Stripe 的状态、方案、已付周期的结束时间及是否正在取消）以及能否购买 Pass；`POST /api/auth/pass/checkout`，返回为账户订阅某方案的 Stripe Checkout 页面，账户已有订阅时拒绝；以及 `POST /api/auth/pass/portal`，返回账户在 Stripe 客户门户中的页面，在那里更改或取消方案。Stripe 把 webhook 发送到 `/api/billing/stripe`，带签名并使用 API 版本 `2025-09-30.clover`，签名不属于该端点或早于五分钟的事件被拒绝：`checkout.session.completed` 把账户关联到其 Stripe 客户，`customer.subscription.created`、`updated` 和 `deleted` 把订阅记录在 `billing` 表中，`invoice.paid` 把账户的 Pass 延长到已付周期结束后一天，从不缩短。删除账户会取消其订阅。缺少 Stripe 配置（§6）时这些端点返回 501。`/pass` 显示 Pass 提供的内容、账户自己的 Pass 及方案，对受管账户不提供方案。
* **工作人员**是角色为 `staff` 的账户，该角色只在数据库中设置。他们通过 better-auth 的 admin 插件（其角色不允许他们做其他事）按名称或电子邮件地址查找账户，并停用和恢复账户；工作人员不能被停用。停用账户会结束其会话（包括设备会话），向其服务端发送原因为停用的 `AccountRevoked`，并在恢复前拒绝其登录（`BANNED_USER`）。`GET /api/auth/staff/account` 给出账户的 Pass、订阅、Home、服务端和设备会话，`POST /api/auth/staff/pass` 设置或结束其 Pass。`/staff` 是他们的页面。
* **设备会话**（[账户 §6](accounts.zh-CN.md#6-登录)）由 Mavio 自己实现：每个会话一行，记录其账户、客户端 ID 和设备密钥。访问令牌是 `mds1.<声明>.<签名>`，声明写明会话、账户和十分钟后的过期时间，用 better-auth 的密钥以 HMAC-SHA256 签名；每个请求都检查其会话是否仍然存在，因此结束会话立即生效。Connect 服务既接受这种令牌，也接受 better-auth 的会话。
  * `/api/oauth/token` 为授权码开启会话，其 `client_id` 是服务端的 ID（账户不再管理该服务端时拒绝）、`app` 或 Mavio 的某个应用（`dev.mavio.*`）；为已批准的链接码开启会话，使用授权类型 `urn:ietf:params:oauth:grant-type:device_code`，等待期间按 RFC 8628 应答；并用授权类型 `urn:mavio:params:oauth:grant-type:device-signature` 为会话续期，参数是会话 ID、晚于上次续期且与 mavio.dev 时间相差不超过两分钟的毫秒时间，以及设备密钥对 `mavio-dev-renew-v1`、一个零字节、会话 ID、一个零字节和该时间的 Ed25519 签名。应答包含访问令牌、其有效期、会话 ID 和账户。
  * `/api/oauth/device` 为 Mavio 自己的客户端（`app`、`dev.mavio.*`）或服务端 Web 界面（其 ID）的设备密钥给出链接码：在 `/link` 输入的用户码 `XXXX-XXXX`，以及用于轮询的设备码，有效十分钟，轮询间隔至少五秒。服务端的链接码只能由其管理员批准。
  * `/api/oauth/revoke` 结束一个访问令牌（无论是否过期）的会话。
* 密码通过 better-auth 的 `password.hash` 和 `password.verify`，用 `node:crypto` 的 scrypt（N = 16384、r = 16、p = 1、64 字节密钥）哈希，以 PHC 格式保存，参数变化时在登录时重新哈希。
* 托管账户是 `HomeService` 直接插入的 better-auth 用户，类型为 `managed`，带有其限制配置，邮箱值为 `managed:<id>`。只有公开路由要求有效的邮箱地址，因此任何登录都不接受这个值；它没有凭据，Mavio 的 better-auth 插件中的钩子拒绝它调用修改名称、邮箱地址、密码、通行密钥和 TOTP 或删除账户的端点。转为完整账户时设置真实的邮箱地址和密码，并把变更发送给它的服务端。
* 页面还调用 `mavio.cloud.v1.FriendService` 和 `mavio.cloud.v1.HomeService`。好友按用户名或已验证的邮箱地址请求，对方回加或接受即成为好友；向非好友账户授予访问时会同时发出好友请求。一个 Home 至多 15 个账户（含管理员），未答复的邀请也计入其中。邀请需要管理员的 Mavio Pass，并附带管理员服务端上的访问记录，接受后生效；加入另一个 Home 会离开当前的 Home，Home 的管理员须先解散它。离开或被移除的完整成员成为管理员的好友；被移除或所在 Home 被解散的托管账户被删除，其服务端收到原因为删除的 `AccountRevoked`；管理着 Home 的账户不能删除。管理员向本 Home 托管账户授予的访问立即生效，且只限其自己的服务端。
* 切换是 Mavio 的 better-auth 插件提供的 `POST /api/auth/home/switch`，带目标的账户 ID 及其 PIN，或在 PIN 错误过多后带其密码：它为目标创建会话，设置其 cookie（以及供原生客户端使用的 bearer 令牌），并删除设备之前的会话。PIN 与密码一样哈希；从第三次错误起，切换到该账户需等待 30 秒，每次加倍，至多 15 分钟；完整账户的 PIN 错误五次后，切换到它需要其密码。Mavio Pass 是按账户保存到期时间的表，`GetAccount` 报告它以及账户的 Pass 是否有效；管理员没有 Pass 时 Home 冻结。
* Mavio 自己实现的部分：认领与 `private_key_jwt`、登录断言、媒体库访问记录及其邀请、好友、Home、用 PIN 切换、Mavio Pass、向服务端的推送、§4.2 的 ACME 质询，以及中转票据（§5）。
* 服务端调用 API 基础 URL 下的 Connect 服务 `mavio.cloud.v1.ServerService`（`libs/proto`），基础 URL 为 `https://mavio.dev/api`，除非服务端的 `mavio-dev-url` 指定了别的地址：`CreateAuthorizationRequest` 仿照 RFC 9126 记录服务端为其 Web 界面推送的授权请求，连同其重定向 URI、PKCE 质询、设备密钥和状态，服务端要求时认领该服务端，否则仅限已注册的服务端，并返回其页面 `/authorize?request_uri=…`；`ClaimServer` 在账户批准后返回认领请求的结果，或把服务端的身份公钥注册到认领码所属账户名下，返回该账户和服务端当前的游标；`ListChanges` 给出已认领服务端在某游标之后的变更；`PublishLibraries` 替换 mavio.dev 保存的该服务端媒体库的 ID、名称和类型，`PublishConnections` 替换客户端访问该服务端的 URL，`ListServers` 把它们提供给服务端的账户。一项变更是：授予访问记录，附带账户和记录；撤销访问；账户改名；或账户被停用、删除或报告被盗。每个请求都以 bearer 令牌携带服务端用身份密钥签名的 JWT（`private_key_jwt`）：EdDSA，以服务端 ID（[账户 §5.1](accounts.zh-CN.md#51-服务端身份)）为 `kid`、`iss` 和 `sub`，以基础 URL 为 `aud`，带随机的 `jti`，签发一分钟后过期。
* 登录断言（[账户 §5.5](accounts.zh-CN.md#55-登录断言)）是用 better-auth JWT 插件的密钥签名的 EdDSA JWT，密钥集发布在 API 基础 URL 下的 `/auth/jwks`：`iss` 为基础 URL，`aud` 为服务端 ID，`sub` 为账户 ID，`iat` 与 `exp` 相差不超过两分钟，`device_key` 为设备公钥的无填充 base64url，`nonce` 为随机值。设备对 `mavio-sign-in-assertion-v1`、一个零字节和断言签名。服务端按 `Cache-Control` 的 max-age 缓存密钥集（没有时为一小时），遇到未知的 `kid` 时重新获取，至多每分钟一次。
* 网站的页面以账户的会话调用 Connect 服务 `mavio.cloud.v1.AccountService`：`GetAuthorizationRequest` 和 `AnswerAuthorizationRequest` 在 `/authorize` 显示服务端的授权请求并批准或取消它，批准后浏览器带着授权码返回：认领请求需要完整账户，并把服务端注册到其名下，其他请求需要管理该服务端的账户；`CreateClaimCode` 为没有浏览器的服务端给出认领码，即不含易混字符的八个字符 `XXXX-XXXX`，有效十分钟，以哈希保存；`GetLinkCode` 和 `AnswerLinkCode` 在 `/link` 显示链接码并批准或拒绝它；`ListServers` 和 `RemoveServer` 列出账户管理或可访问的服务端并移除其一；`CreateAssertion` 为账户有访问权的服务端和一个设备密钥签发登录断言；授权码有效一分钟，以哈希与重定向 URI、PKCE 质询和设备密钥一起保存，`/api/oauth/token` 用它换取一次会话；`CreateAuthorizationCode` 由 `/authorize` 调用，重定向 URI 位于 app.mavio.dev 地址时立即为其给出授权码，为以客户端 ID 指明的 Mavio 应用则在账户批准后给出，其重定向 URI 须为回环地址（`http://127.0.0.1` 或 `http://[::1]`，任意端口），或以客户端 ID 命名的私有 scheme，例如 `dev.mavio.app:/callback`（RFC 8252）；`ListAccess`、`GrantAccess` 和 `RevokeAccess` 供服务端管理员管理其访问记录；`ListInvitations`、`AcceptInvitation`、`DeclineInvitation` 和 `LeaveServer` 供被邀请的账户使用；`ReportCompromised` 结束账户的所有会话，包括设备会话。
* 向账户授予访问会创建一个邀请；账户接受后访问记录才生效，其服务端才收到变更。修改已接受的记录会再次发送它，撤销或退出则发送其撤销。服务端被另一个账户认领时，发送对前任管理员访问的撤销。账户改名时把变更发送给它能访问的服务端；删除账户发送其撤销，并在它仍管理服务端时被拒绝；报告被盗发送其专属类型的撤销。

## 3. 向服务端推送
* mavio.dev 模式的每台服务端对应一个 Durable Object，持有该服务端的 WebSocket 连接。连接在消息之间休眠，并用协议层的 ping 保活，ping 不会唤醒该对象。
* 每项针对某台服务端的变更（[访问记录](accounts.zh-CN.md#33-媒体库访问)、吊销、Home 与账户的删除）都带一个递增的游标保存。Durable Object 只通知服务端它的变更已到游标 N；服务端随后通过 HTTPS，用身份密钥签名的请求，取回自己游标之后的变更。
* 服务端用与请求相同的 JWT 打开 API 基础 URL 下 `/servers/notices` 的 WebSocket；每条通知是一个装有 `mavio.cloud.v1.Notice` 的二进制帧。服务端每 30 秒发一次 ping，ping 失败即结束连接。
* 服务端每次重新连接时（按指数退避加随机抖动）以及每六小时也会取回变更，因此丢失的通知只会推迟变更，不会丢失变更。
* Durable Object 是类 `NoticeHub`，由 Worker 的入口 `src/server.ts` 与 TanStack Start 的处理器一同导出，以服务端 ID 命名。Worker 先检查服务端的 JWT，再把 WebSocket 交给该对象，对象会替换同一服务端较旧的连接。请求记录的每项变更都会被记下，请求完成后 Worker 把每个服务端的最新游标告诉其对象（`waitUntil`），因此通知绝不会早于记录该变更的事务。

## 4. 证书与子域名

### 4.1 名称
* 服务端的名称位于 `s.mavio.fyi` 下。该域名列入公共后缀列表，使浏览器把不同服务端的 Cookie 和存储隔开。
* mavio.dev 模式的服务端为它的每个地址拥有一个名称 `<地址>.<ID>.s.mavio.fyi`：IPv4 地址用短横线书写（`192-168-1-10`），IPv6 地址用短横线代替冒号（`2001-db8--1`）。
* `apps/dns` 是基于 `miekg/dns` 的无状态 Go 服务，作为 `s.mavio.fyi` 的两台名称服务器，`mavio.fyi` 用 NS 记录把它委派给它们。它从名称计算 A 和 AAAA 应答，用对应区域的中转地址应答中转名称（§5），并提供静态的 SOA 和 NS、`_psl` TXT 记录，以及允许 Let's Encrypt 和 Google Trust Services 的 CAA 记录。对 `_acme-challenge.<ID>.s.mavio.fyi`，它应答指向 `<ID>.acme.mavio.dev` 的 CNAME。
* ID 是由 32 个小写 base32 字符组成的标签；在它之下，既不是用短横线书写的 IPv4 地址、也不是用短横线代替冒号的 IPv6 地址的地址标签，未知区域的中转名称，以及其他所有名称，都应答为不存在，并附上 SOA。该服务通过 UDP 和 TCP 监听（`--addr`，默认 `:53`），从 JSON 文件读取配置（`--config`）：区域、名称服务器及其地址、SOA 的邮箱和序列号、ACME 域名、`_psl` 的值、CAA 记录的 CA、按区域划分的中转地址，以及 TTL：计算出的记录和静态记录为一小时，中转名称为五分钟。

### 4.2 证书
* 每台服务端有一张通配符证书 `*.<ID>.s.mavio.fyi`，由服务端自己用 `golang.org/x/crypto/acme` 通过 ACME 的 DNS-01 质询从 Let's Encrypt 获取。证书的私钥在服务端上生成，从不离开服务端。
* 服务端用身份密钥签名的请求把质询的 TXT 值发给 mavio.dev（`ServerService.SetAcmeChallenge`）；mavio.dev 只为该服务端自己的 ID 接受它，通过 Cloudflare API 在自己的 Cloudflare 区域中写入 TXT 记录 `<ID>.acme.mavio.dev`，质询完成后删除（`ClearAcmeChallenge`）。服务端发布该值后等待十秒，再请 CA 检查。
* 服务端在证书有效期的三分之二时续期，或在 ACME 续期信息（ARI）建议的时间窗口内随机选一个更早的时间续期；Let's Encrypt 失败时改用 Google Trust Services。Google Trust Services 注册服务端的 ACME 账户需要外部账户绑定，由 mavio.dev 以其 Google Cloud 项目的服务账号通过 Google 的 Public CA API 签发（`ServerService.CreateAcmeBinding`）。失败的尝试在十分钟后重试，间隔逐次加倍，最长六小时。
* 已认领的服务端把证书、证书私钥以及每个 CA 一个的 ACME 账户密钥保存在 `certificate-dir`（其主目录下的 `certificates/`），按顺序尝试 `acme-directories` 中的 CA；当网络设置未指定自己的证书文件时，它在网络设置的 HTTPS 端口（默认 8920）上用该证书提供 HTTPS。证书通过 `tls.Config.GetCertificate` 提供，续期无需重启即生效。

### 4.3 连接
浏览器通过 DNS 用名称访问服务端。原生客户端直接连接地址，以该地址的名称作为 TLS 服务器名，因此在过滤私有地址 DNS 应答的网络中也能工作，并且还会检查服务端的身份密钥（[账户 §5.1](accounts.zh-CN.md#51-服务端身份)）。

## 5. 中转（`apps/relay`）
* 一个运行在虚拟机上的 Go 服务，部署在欧洲、美国和新加坡，供无法直接访问的服务端使用。允许使用中转的服务端与其所在区域的中转保持一条连接。
* 客户端通过 `relay-<区域>-<票据>.<ID>.s.mavio.fyi` 经中转访问服务端。票据由 mavio.dev 签发，几分钟内有效，只在观看者或服务端管理员拥有有效 Mavio Pass（[账户 §3.7](accounts.zh-CN.md#37-mavio-pass)）时签发。
* 中转从 TLS ClientHello 中读取名称，检查票据，然后在不终止 TLS 的情况下把客户端的连接与服务端的连接拼接起来，因此加密保持端到端，中转只看到加密后的字节。

### 5.1 票据
* 客户端用 `AccountService.CreateRelayTicket` 请求票据，应答是要连接的主机名及其到期时间。只有当账户管理该服务端或持有已接受的访问记录、服务端已记录中转区域、并且观看者或管理员拥有有效 Mavio Pass 时，mavio.dev 才签发票据。
* 票据是到期时间（五分钟后，以 Unix 秒表示的四个大端字节），后接以 mavio.dev 与各中转共享的机密计算的 HMAC-SHA256 的前 16 个字节，其输入是 `mavio relay ticket`、服务端 ID、区域和这四个字节，以零字节分隔；以不带填充的小写 base32 书写，共 32 个字符。中转用名称中的 ID 和自己的区域检查票据，因此一张票据只允许经一个中转连接一个服务端；到期前打开的连接保持打开。

### 5.2 服务端的连接
* 服务端在已被认领、已拥有证书且网络设置允许中转（默认开启）时与一个中转保持连接。它用 `ServerService.ListRelays` 列出中转，选择最先接受 TCP 连接的那个，用 `ServerService.SetRelayRegion` 记录其区域，并在停止时清除；mavio.dev 只为已记录的区域签发票据。
* 中转在一个端口（443）上服务客户端，在另一个端口上以 HTTPS 服务服务端。服务端以升级到 `mavio-relay` 协议的 `GET /servers/connect` 打开控制连接，在 `Mavio-Identity-Key` 中携带其身份公钥，并携带一个像对 mavio.dev 那样（§2）为中转 URL 签名的 JWT；中转检查该公钥的哈希即为 ID 并验证 JWT，因此不需要数据库。同一服务端较新的控制连接会替换较旧的。
* 中转在控制连接上写入 16 字节的帧：每个客户端一个随机的连接 ID，或每 30 秒一次的全零心跳；服务端 90 秒内未收到任何内容即重新连接。对每个 ID，服务端以 `GET /servers/accept` 并在 `Mavio-Relay-Connection` 中携带该 ID 打开连接，只有被通知的服务端才能取用；中转把客户端的 ClientHello 发给它，再双向复制字节，服务端在这条连接上用其证书提供 TLS 和它的 HTTP 处理器。
* 中转是一个无状态的 Go 程序（`--region`；`--url`，即服务端 JWT 中写明的基础 URL；`--ticket-secret-file`；面向客户端的 `--addr`；面向服务端的 `--server-addr` 及 `--cert` 和 `--key`；`--zone`）。连接失败后在五秒后重试，间隔加倍直至十分钟。

## 6. 部署

| 位置 | 内容 |
| :--- | :--- |
| Cloudflare | Workers 上的 `apps/website`；§3 的 Durable Object；`mavio.dev` 的 DNS 区域；通过 Cloudflare Email Service 发送邮件；限流 |
| Stripe | Mavio Pass 的价格、Checkout、客户门户和 §2 的 webhook |
| 虚拟机 | PostgreSQL，带一个流复制副本，WAL 归档到 R2；`apps/relay`；`apps/dns` |

Worker 用 Cloudflare 的 `cf` 命令行（`apps/website` 的开发依赖）构建、在开发时运行和部署，其配置是 `apps/website/cloudflare.config.ts`：`BASE_URL` 变量、app.mavio.dev 的地址 `APP_URL` 变量（未设置时为 `https://app.mavio.dev`）、Hyperdrive 绑定（其 ID 是为数据库创建的配置）、`NoticeHub` Durable Object 的 `NOTICES` 绑定（以 SQLite 存储导出）、`ACME_DOMAIN` 变量（`acme.mavio.dev`）、从 `noreply@mavio.dev` 发信的 `EMAIL` 绑定（其域名须已接入 Email Sending）、§2 的限流绑定 `CREDENTIALS_LIMIT`、`MAIL_LIMIT` 与 `CODES_LIMIT`，以及机密 `BETTER_AUTH_SECRET`、`CLOUDFLARE_ZONE_ID` 与 `CLOUDFLARE_API_TOKEN`（可编辑该区域 DNS 记录的令牌）、`GOOGLE_CA_PROJECT` 与 `GOOGLE_CA_CREDENTIALS`（可创建外部账户密钥的服务账号的密钥文件），以及 `RELAY_TICKET_SECRET`（中转的票据机密），每个机密都用 `cf workers secrets update` 设置；缺少 Cloudflare 机密时，ACME 方法返回未实现。`RELAYS` 绑定是中转运行后在配置中列出的 JSON 绑定，按区域给出各中转的基础 URL，每个 URL 即其中转的 `--url`；缺少它或票据机密时没有中转，票据方法返回未实现。Mavio Pass 通过 `STRIPE_PRICE_MONTH` 与 `STRIPE_PRICE_YEAR`（其定期价格的 ID）以及机密 `STRIPE_SECRET_KEY` 与 `STRIPE_WEBHOOK_SECRET`（webhook 端点 `https://mavio.dev/api/billing/stripe` 的签名机密，该端点以 §2 的 API 版本发送 §2 的事件）销售；缺少它们时不销售。其入口是 `src/server.ts`。Worker 绑定与 Workers 运行时的类型由 `cf workers types`（`typecheck` 会运行它）和构建从配置生成到 `.cloudflare/types`。`pnpm --filter @mavio/website build`（`cf build`）把 Worker 写入 `.cloudflare/output`，`deploy`（`cf deploy`）构建并部署它。开发时，`pnpm --filter @mavio/website dev`（`cf dev`）通过 Vite 在 workerd 中运行 Worker，连接绑定的开发连接字符串所指的 PostgreSQL（例如端口 5432 上用户、密码和数据库均为 `mavio` 的 `postgres:17-alpine` 容器，用 `DATABASE_URL=postgres://mavio:mavio@localhost:5432/mavio drizzle-kit migrate` 迁移），开发模式下 `BASE_URL` 为 `http://localhost:5173`，机密放在不提交的 `apps/website/.env.local` 中，邮件不发送而是写入 `.cloudflare/tmp/email`；服务端以 `--mavio-dev-url http://localhost:5173/api` 连接它。要从网络中的其他设备访问它（其浏览器只在安全上下文中登录），`DEV_TLS_CERT` 与 `DEV_TLS_KEY`（为本机地址签发的 PEM 证书与密钥，例如由那些设备信任的 mkcert 生成）让它在所有网络接口上以 HTTPS 提供服务，`DEV_ORIGIN`（例如 `https://192.168.1.10:5173`）把 `BASE_URL` 设为该地址；服务端随后以该地址的 `/api` 作为 `--mavio-dev-url` 启动。

## 7. 安全说明

* **mavio.dev 控制 `s.mavio.fyi`**，可以为服务端的名称申请证书；原生客户端还会检查服务端的身份密钥，而 mavio.dev 没有它的私钥。
* **中转**能看到客户端访问哪台服务端以及传输了多少数据，看不到内容。
* mavio.dev 被攻破时能对服务端做什么，见[账户 §10](accounts.zh-CN.md#10-安全说明)。

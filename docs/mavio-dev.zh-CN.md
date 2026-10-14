# mavio.dev

> [English](mavio-dev.md) | 简体中文

> 相关文档：[账户](accounts.zh-CN.md) · [系统架构](architecture.zh-CN.md) · [路线图](roadmap.zh-CN.md) · [AGENTS.zh-CN.md](../AGENTS.zh-CN.md)

> mavio.dev 模式服务端的中心服务：它运行什么、如何触达服务端、服务端如何获得名称和证书、中转，以及各部分部署在哪里。它保存什么以及它所服务的账户模型，见[账户](accounts.zh-CN.md)。在[路线图](roadmap.zh-CN.md)的 P27 阶段实现。

---

## 1. 概述

mavio.dev 是 mavio.dev 模式服务端的 OpenID Connect 签发方（[账户 §3.2](accounts.zh-CN.md#32-服务端与认领)）：它保存这些服务端的访问记录并向它们推送变更和吊销，用代码登录电视，并为它们提供中转、证书和发现。它由三个应用组成：

| 应用 | 内容 |
| :--- | :--- |
| `apps/website` | 网站及其 API：账户、服务端、访问记录、好友、Home、Mavio Pass、向服务端的推送和 ACME 质询（§2、§3） |
| `apps/dns` | `mavio.site` 的名称服务器（§4） |
| `apps/relay` | 供无法直接访问的服务端使用的中转（§5） |

## 2. 网站与 API（`apps/website`）
* 一个运行在 Cloudflare Workers 上的 TanStack Start 应用，用 Cloudflare 的 `cf` 命令行部署。页面使用 TanStack Query；数据保存在 PostgreSQL 中，通过 Hyperdrive 用基于 `pg` 的 Drizzle 访问，每个请求一个数据库客户端。模式在 `src/server/db` 中：better-auth 的表由它的 `auth` 命令行生成（`pnpm --filter @mavio/website generate-auth-schema`），另有 Mavio 自己的表；drizzle-kit 把它们的迁移写入 `drizzle/`（`generate-migration`）。API 位于 `/api` 下：better-auth 在 `/api/auth`，服务端 Web 界面的令牌端点在 `/api/oauth/token`（[账户 §6](accounts.zh-CN.md#6-登录)），Connect 服务在 `/api/mavio.cloud.v1.*`，其请求用 protovalidate 检查。
* 账户基于 better-auth：邮箱地址或用户名加密码、通行密钥、作为第二因素的 TOTP，用于电视代码的设备授权插件（RFC 8628）——代码在 `/link` 输入，只接受 Mavio 自己的客户端（`dev.mavio.*`）——原生客户端的 bearer 会话，以及 JWT 插件，服务端获取其密钥集来检查登录断言。
* 密码通过 better-auth 的 `password.hash` 和 `password.verify`，用 `node:crypto` 的 scrypt（N = 16384、r = 16、p = 1、64 字节密钥）哈希，以 PHC 格式保存，参数变化时在登录时重新哈希。
* 托管账户是 `HomeService` 直接插入的 better-auth 用户，类型为 `managed`，带有其限制配置，邮箱值为 `managed:<id>`。只有公开路由要求有效的邮箱地址，因此任何登录都不接受这个值；它没有凭据，Mavio 的 better-auth 插件中的钩子拒绝它调用修改名称、邮箱地址、密码、通行密钥和 TOTP 或删除账户的端点。转为完整账户时设置真实的邮箱地址和密码，并把变更发送给它的服务端。
* 页面还调用 `mavio.cloud.v1.FriendService` 和 `mavio.cloud.v1.HomeService`。好友按用户名或邮箱地址请求，对方回加或接受即成为好友；向非好友账户授予访问时会同时发出好友请求。一个 Home 至多 15 个账户（含管理员），未答复的邀请也计入其中。邀请需要管理员的 Mavio Pass，并附带管理员服务端上的访问记录，接受后生效；加入另一个 Home 会离开当前的 Home，Home 的管理员须先解散它。离开或被移除的完整成员成为管理员的好友；被移除或所在 Home 被解散的托管账户被删除，其服务端收到原因为删除的 `AccountRevoked`；管理着 Home 的账户不能删除。管理员向本 Home 托管账户授予的访问立即生效，且只限其自己的服务端。
* 切换是 Mavio 的 better-auth 插件提供的 `POST /api/auth/home/switch`，带目标的账户 ID 及其 PIN，或在 PIN 错误过多后带其密码：它为目标创建会话，设置其 cookie（以及供原生客户端使用的 bearer 令牌），并删除设备之前的会话。PIN 与密码一样哈希；从第三次错误起，切换到该账户需等待 30 秒，每次加倍，至多 15 分钟；完整账户的 PIN 错误五次后，切换到它需要其密码。Mavio Pass 是按账户保存到期时间的表，`GetAccount` 报告它以及账户的 Pass 是否有效；管理员没有 Pass 时 Home 冻结。
* Mavio 自己实现的部分：认领与 `private_key_jwt`、登录断言、媒体库访问记录及其邀请、好友、Home、用 PIN 切换、Mavio Pass、向服务端的推送、§4.2 的 ACME 质询，以及中转票据（§5）。
* 服务端调用 API 基础 URL 下的 Connect 服务 `mavio.cloud.v1.ServerService`（`libs/proto`），基础 URL 为 `https://mavio.dev/api`，除非服务端的 `mavio-dev-url` 指定了别的地址：`ClaimServer` 把服务端的身份公钥注册到认领码所属账户名下，返回该账户和服务端当前的游标；`ListChanges` 给出已认领服务端在某游标之后的变更；`PublishLibraries` 替换 mavio.dev 保存的该服务端媒体库的 ID、名称和类型。一项变更是：授予访问记录，附带账户和记录；撤销访问；账户改名；或账户被停用、删除或报告被盗。每个请求都以 bearer 令牌携带服务端用身份密钥签名的 JWT（`private_key_jwt`）：EdDSA，以服务端 ID（[账户 §5.1](accounts.zh-CN.md#51-服务端身份)）为 `kid`、`iss` 和 `sub`，以基础 URL 为 `aud`，带随机的 `jti`，签发一分钟后过期。
* 登录断言（[账户 §5.5](accounts.zh-CN.md#55-登录断言)）是用 better-auth JWT 插件的密钥签名的 EdDSA JWT，密钥集发布在 API 基础 URL 下的 `/auth/jwks`：`iss` 为基础 URL，`aud` 为服务端 ID，`sub` 为账户 ID，`iat` 与 `exp` 相差不超过两分钟，`device_key` 为设备公钥的无填充 base64url，`nonce` 为随机值。设备对 `mavio-sign-in-assertion-v1`、一个零字节和断言签名。服务端按 `Cache-Control` 的 max-age 缓存密钥集（没有时为一小时），遇到未知的 `kid` 时重新获取，至多每分钟一次。
* 网站的页面以账户的会话调用 Connect 服务 `mavio.cloud.v1.AccountService`：`CreateClaimCode` 给出认领码，即不含易混字符的八个字符 `XXXX-XXXX`，有效十分钟，以哈希保存；`ListServers` 和 `RemoveServer` 列出账户管理或可访问的服务端并移除其一；`CreateAssertion` 为账户有访问权的服务端和一个设备密钥签发登录断言；`CreateAuthorizationCode` 由 `/authorize` 在账户同意后调用，给服务端的 Web 界面一个有效一分钟的授权码，以哈希与重定向 URI、PKCE 质询和设备密钥一起保存，`/api/oauth/token` 用它换取一次这样的断言；`ListAccess`、`GrantAccess` 和 `RevokeAccess` 供服务端管理员管理其访问记录；`ListInvitations`、`AcceptInvitation`、`DeclineInvitation` 和 `LeaveServer` 供被邀请的账户使用；`ReportCompromised` 结束账户的所有会话。
* 向账户授予访问会创建一个邀请；账户接受后访问记录才生效，其服务端才收到变更。修改已接受的记录会再次发送它，撤销或退出则发送其撤销。服务端被另一个账户认领时，发送对前任管理员访问的撤销。账户改名时把变更发送给它能访问的服务端；删除账户发送其撤销，并在它仍管理服务端时被拒绝；报告被盗发送其专属类型的撤销。

## 3. 向服务端推送
* mavio.dev 模式的每台服务端对应一个 Durable Object，持有该服务端的 WebSocket 连接。连接在消息之间休眠，并用协议层的 ping 保活，ping 不会唤醒该对象。
* 每项针对某台服务端的变更（[访问记录](accounts.zh-CN.md#33-媒体库访问)、吊销、Home 与账户的删除）都带一个递增的游标保存。Durable Object 只通知服务端它的变更已到游标 N；服务端随后通过 HTTPS，用身份密钥签名的请求，取回自己游标之后的变更。
* 服务端用与请求相同的 JWT 打开 API 基础 URL 下 `/servers/notices` 的 WebSocket；每条通知是一个装有 `mavio.cloud.v1.Notice` 的二进制帧。服务端每 30 秒发一次 ping，ping 失败即结束连接。
* 服务端每次重新连接时（按指数退避加随机抖动）以及每六小时也会取回变更，因此丢失的通知只会推迟变更，不会丢失变更。
* Durable Object 是类 `NoticeHub`，由 Worker 的入口 `src/server.ts` 与 TanStack Start 的处理器一同导出，以服务端 ID 命名。Worker 先检查服务端的 JWT，再把 WebSocket 交给该对象，对象会替换同一服务端较旧的连接。请求记录的每项变更都会被记下，请求完成后 Worker 把每个服务端的最新游标告诉其对象（`waitUntil`），因此通知绝不会早于记录该变更的事务。

## 4. 证书与子域名

### 4.1 名称
* 服务端的名称位于 `mavio.site` 下。该域名列入公共后缀列表，使浏览器把不同服务端的 Cookie 和存储隔开。
* mavio.dev 模式的服务端为它的每个地址拥有一个名称 `<地址>.<ID>.mavio.site`：IPv4 地址用短横线书写（`192-168-1-10`），IPv6 地址用短横线代替冒号（`2001-db8--1`）。
* `apps/dns` 是基于 `miekg/dns` 的无状态 Go 服务，在两台虚拟机上作为 `mavio.site` 的名称服务器。它从名称计算 A 和 AAAA 应答，用对应区域的中转地址应答中转名称（§5），并提供静态的 SOA 和 NS、`_psl` TXT 记录，以及允许 Let's Encrypt 和 Google Trust Services 的 CAA 记录。对 `_acme-challenge.<ID>.mavio.site`，它应答指向 `<ID>.acme.mavio.dev` 的 CNAME。
* ID 是由 32 个小写 base32 字符组成的标签；在它之下，既不是用短横线书写的 IPv4 地址、也不是用短横线代替冒号的 IPv6 地址的地址标签，未知区域的中转名称，以及其他所有名称，都应答为不存在，并附上 SOA。该服务通过 UDP 和 TCP 监听（`--addr`，默认 `:53`），从 JSON 文件读取配置（`--config`）：区域、名称服务器及其地址、SOA 的邮箱和序列号、ACME 域名、`_psl` 的值、CAA 记录的 CA、按区域划分的中转地址，以及 TTL：计算出的记录和静态记录为一小时，中转名称为五分钟。

### 4.2 证书
* 每台服务端有一张通配符证书 `*.<ID>.mavio.site`，由服务端自己用 `golang.org/x/crypto/acme` 通过 ACME 的 DNS-01 质询从 Let's Encrypt 获取。证书的私钥在服务端上生成，从不离开服务端。
* 服务端用身份密钥签名的请求把质询的 TXT 值发给 mavio.dev（`ServerService.SetAcmeChallenge`）；mavio.dev 只为该服务端自己的 ID 接受它，通过 Cloudflare API 在自己的 Cloudflare 区域中写入 TXT 记录 `<ID>.acme.mavio.dev`，质询完成后删除（`ClearAcmeChallenge`）。服务端发布该值后等待十秒，再请 CA 检查。
* 服务端在证书有效期的三分之二时续期，或在 ACME 续期信息（ARI）建议的时间窗口内随机选一个更早的时间续期；Let's Encrypt 失败时改用 Google Trust Services。Google Trust Services 注册服务端的 ACME 账户需要外部账户绑定，由 mavio.dev 以其 Google Cloud 项目的服务账号通过 Google 的 Public CA API 签发（`ServerService.CreateAcmeBinding`）。失败的尝试在十分钟后重试，间隔逐次加倍，最长六小时。
* 已认领的服务端把证书、证书私钥以及每个 CA 一个的 ACME 账户密钥保存在 `certificate-dir`（其主目录下的 `certificates/`），按顺序尝试 `acme-directories` 中的 CA；当网络设置未指定自己的证书文件时，它在网络设置的 HTTPS 端口（默认 8920）上用该证书提供 HTTPS。证书通过 `tls.Config.GetCertificate` 提供，续期无需重启即生效。

### 4.3 连接
浏览器通过 DNS 用名称访问服务端。原生客户端直接连接地址，以该地址的名称作为 TLS 服务器名，因此在过滤私有地址 DNS 应答的网络中也能工作，并且还会检查服务端的身份密钥（[账户 §5.1](accounts.zh-CN.md#51-服务端身份)）。

## 5. 中转（`apps/relay`）
* 一个运行在虚拟机上的 Go 服务，部署在欧洲、美国和新加坡，供无法直接访问的服务端使用。允许使用中转的服务端与其所在区域的中转保持一条连接。
* 客户端通过 `relay-<区域>-<票据>.<ID>.mavio.site` 经中转访问服务端。票据由 mavio.dev 签发，几分钟内有效，只在观看者或服务端管理员拥有有效 Mavio Pass（[账户 §3.7](accounts.zh-CN.md#37-mavio-pass)）时签发。
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
| Cloudflare | Workers 上的 `apps/website`；§3 的 Durable Object；`mavio.dev` 的 DNS 区域；用于 Mavio 发布下载的 R2；用 Workers 的出站 TCP 连接检查服务端公布地址的可达性；通过 Cloudflare Email Service 发送邮件 |
| 虚拟机 | PostgreSQL，带一个流复制副本，WAL 归档到 R2；`apps/relay`；`apps/dns` |

Worker 的配置是 `apps/website/wrangler.jsonc`：`BASE_URL` 变量、Hyperdrive 绑定（其 ID 是为数据库创建的配置）、`NoticeHub` Durable Object 的 `NOTICES` 绑定及其以 SQLite 为后端的类迁移、`ACME_DOMAIN` 变量（`acme.mavio.dev`），以及机密 `BETTER_AUTH_SECRET`、`CLOUDFLARE_ZONE_ID` 与 `CLOUDFLARE_API_TOKEN`（可编辑该区域 DNS 记录的令牌）、`GOOGLE_CA_PROJECT` 与 `GOOGLE_CA_CREDENTIALS`（可创建外部账户密钥的服务账号的密钥文件），以及 `RELAY_TICKET_SECRET`（中转的票据机密）；缺少 Cloudflare 机密时，ACME 方法返回未实现。`RELAYS` 变量是一个按区域列出各中转基础 URL 的 JSON 对象，每个 URL 即其中转的 `--url`；缺少它或票据机密时没有中转，票据方法返回未实现。其入口是 `src/server.ts`。开发时，`pnpm --filter @mavio/website dev` 通过 Vite 在 workerd 中运行 Worker，连接绑定的 `localConnectionString` 所指的 PostgreSQL（用 `drizzle-kit migrate` 迁移），机密和值为 `http://localhost:5173` 的 `BASE_URL` 放在不提交的 `apps/website/.dev.vars` 中；服务端以 `--mavio-dev-url http://localhost:5173/api` 连接它。

## 7. 安全说明

* **mavio.dev 控制 `mavio.site`**，可以为服务端的名称申请证书；原生客户端还会检查服务端的身份密钥，而 mavio.dev 没有它的私钥。
* **中转**能看到客户端访问哪台服务端以及传输了多少数据，看不到内容。
* mavio.dev 被攻破时能对服务端做什么，见[账户 §10](accounts.zh-CN.md#10-安全说明)。

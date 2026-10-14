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
* 一个运行在 Cloudflare Workers 上的 TanStack Start 应用，用 Cloudflare 的 `cf` 命令行部署。页面使用 TanStack Query；数据保存在 PostgreSQL 中，通过 Hyperdrive 用 Drizzle 访问，每个请求一个数据库客户端。
* 账户基于 better-auth：邮箱加密码、通行密钥、TOTP，用于电视代码的设备授权插件（RFC 8628），以及 JWT 插件，服务端获取其密钥集来检查登录断言。
* 密码通过 better-auth 的 `password.hash` 和 `password.verify`，用 `node:crypto` 的 scrypt（N = 16384、r = 16、p = 1、64 字节密钥）哈希，以 PHC 格式保存，参数变化时在登录时重新哈希。
* 托管账户是通过服务端适配器创建的 better-auth 用户，邮箱值为 `managed:<id>`。只有公开路由要求有效的邮箱地址，因此任何登录都不接受这个值。转为完整账户时设置真实的邮箱地址和密码。
* Mavio 自己实现的部分：认领与 `private_key_jwt`、登录断言、媒体库访问记录及其邀请、好友、Home、用 PIN 切换、Mavio Pass、向服务端的推送、§4.2 的 ACME 质询，以及中转票据（§5）。
* 服务端调用 API 基础 URL 下的 Connect 服务 `mavio.cloud.v1.ServerService`（`libs/proto`），基础 URL 为 `https://mavio.dev/api`，除非服务端的 `mavio-dev-url` 指定了别的地址：`ClaimServer` 把服务端的身份公钥注册到认领码所属账户名下，返回该账户和服务端当前的游标；`ListChanges` 给出已认领服务端在某游标之后的变更。每个请求都以 bearer 令牌携带服务端用身份密钥签名的 JWT（`private_key_jwt`）：EdDSA，以服务端 ID（[账户 §5.1](accounts.zh-CN.md#51-服务端身份)）为 `kid`、`iss` 和 `sub`，以基础 URL 为 `aud`，带随机的 `jti`，签发一分钟后过期。

## 3. 向服务端推送
* mavio.dev 模式的每台服务端对应一个 Durable Object，持有该服务端的 WebSocket 连接。连接在消息之间休眠，并用协议层的 ping 保活，ping 不会唤醒该对象。
* 每项针对某台服务端的变更（[访问记录](accounts.zh-CN.md#33-媒体库访问)、吊销、Home 与账户的删除）都带一个递增的游标保存。Durable Object 只通知服务端它的变更已到游标 N；服务端随后通过 HTTPS，用身份密钥签名的请求，取回自己游标之后的变更。
* 服务端用与请求相同的 JWT 打开 API 基础 URL 下 `/servers/notices` 的 WebSocket；每条通知是一个装有 `mavio.cloud.v1.Notice` 的二进制帧。服务端每 30 秒发一次 ping，ping 失败即结束连接。
* 服务端每次重新连接时（按指数退避加随机抖动）以及每六小时也会取回变更，因此丢失的通知只会推迟变更，不会丢失变更。

## 4. 证书与子域名

### 4.1 名称
* 服务端的名称位于 `mavio.site` 下。该域名列入公共后缀列表，使浏览器把不同服务端的 Cookie 和存储隔开。
* mavio.dev 模式的服务端为它的每个地址拥有一个名称 `<地址>.<ID>.mavio.site`：IPv4 地址用短横线书写（`192-168-1-10`），IPv6 地址用短横线代替冒号（`2001-db8--1`）。
* `apps/dns` 是基于 `miekg/dns` 的无状态 Go 服务，在两台 Hetzner 虚拟机上作为 `mavio.site` 的名称服务器。它从名称计算 A 和 AAAA 应答，用对应区域的中转地址应答中转名称（§5），并提供静态的 SOA 和 NS、`_psl` TXT 记录，以及允许 Let's Encrypt 和 Google Trust Services 的 CAA 记录。对 `_acme-challenge.<ID>.mavio.site`，它应答指向 `<ID>.acme.mavio.dev` 的 CNAME。

### 4.2 证书
* 每台服务端有一张通配符证书 `*.<ID>.mavio.site`，由服务端自己用 `golang.org/x/crypto/acme` 通过 ACME 的 DNS-01 质询从 Let's Encrypt 获取。证书的私钥在服务端上生成，从不离开服务端。
* 服务端用身份密钥签名的请求把质询的 TXT 值发给 mavio.dev；mavio.dev 只为该服务端自己的 ID 接受它，通过 Cloudflare API 在自己的 Cloudflare 区域中写入 TXT 记录 `<ID>.acme.mavio.dev`，质询完成后删除。
* 服务端在证书有效期的三分之二时续期，或按 ACME 续期信息（ARI）的建议续期；Let's Encrypt 失败时改用 Google Trust Services。证书通过 `tls.Config.GetCertificate` 提供，续期无需重启即生效。

### 4.3 连接
浏览器通过 DNS 用名称访问服务端。原生客户端直接连接地址，以该地址的名称作为 TLS 服务器名，因此在过滤私有地址 DNS 应答的网络中也能工作，并且还会检查服务端的身份密钥（[账户 §5.1](accounts.zh-CN.md#51-服务端身份)）。

## 5. 中转（`apps/relay`）
* 一个运行在 Hetzner 上的 Go 服务，部署在欧洲、美国和新加坡，供无法直接访问的服务端使用。允许使用中转的服务端与其所在区域的中转保持一条连接。
* 客户端通过 `relay-<区域>-<票据>.<ID>.mavio.site` 经中转访问服务端。票据由 mavio.dev 签发，几分钟内有效，只在观看者或服务端管理员拥有有效 Mavio Pass（[账户 §3.7](accounts.zh-CN.md#37-mavio-pass)）时签发。
* 中转从 TLS ClientHello 中读取名称，检查票据，然后在不终止 TLS 的情况下把客户端的连接与服务端的连接拼接起来，因此加密保持端到端，中转只看到加密后的字节。

## 6. 部署

| 位置 | 内容 |
| :--- | :--- |
| Cloudflare | Workers 上的 `apps/website`；§3 的 Durable Object；`mavio.dev` 的 DNS 区域；用于 Mavio 发布下载的 R2；用 Workers 的出站 TCP 连接检查服务端公布地址的可达性；通过 Cloudflare Email Service 发送邮件 |
| Hetzner | PostgreSQL，带一个流复制副本，WAL 归档到 R2；`apps/relay`；`apps/dns` |

## 7. 安全说明

* **mavio.dev 控制 `mavio.site`**，可以为服务端的名称申请证书；原生客户端还会检查服务端的身份密钥，而 mavio.dev 没有它的私钥。
* **中转**能看到客户端访问哪台服务端以及传输了多少数据，看不到内容。
* mavio.dev 被攻破时能对服务端做什么，见[账户 §10](accounts.zh-CN.md#10-安全说明)。

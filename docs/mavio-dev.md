# mavio.dev

> English | [简体中文](mavio-dev.zh-CN.md)

> Related: [Accounts](accounts.md) · [Architecture](architecture.md) · [Roadmap](roadmap.md) · [AGENTS.md](../AGENTS.md)

> The central service of servers in mavio.dev mode: what it runs, how it reaches servers, how servers get names and certificates, the relay, and where each part is deployed. What it holds and the account model it serves are described in [Accounts](accounts.md). It is built in phase P27 of the [Roadmap](roadmap.md).

---

## 1. Overview

mavio.dev is the OpenID Connect issuer of servers in mavio.dev mode ([Accounts §3.2](accounts.md#32-servers-and-claiming)): it keeps their access records and pushes changes and revocations to them, signs in televisions with codes, and runs the relay, certificates and discovery for those servers. It is made of three applications:

| Application | What it is |
| :--- | :--- |
| `apps/website` | The website and its API: accounts, servers, access records, friends, Homes, Mavio Pass, the push to servers and ACME challenges (§2, §3) |
| `apps/dns` | The name servers of `mavio.site` (§4) |
| `apps/relay` | The relay for servers that cannot be reached directly (§5) |

## 2. Website and API (`apps/website`)
* A TanStack Start application on Cloudflare Workers, deployed with Cloudflare's `cf` command line. The pages use TanStack Query; the data is in PostgreSQL, reached through Hyperdrive with Drizzle, with one database client per request.
* Accounts are built on better-auth: e-mail and password, passkeys, TOTP, its device authorization plugin (RFC 8628) for television codes, and its JWT plugin, whose key set servers fetch to check sign-in assertions.
* Passwords are hashed through better-auth's `password.hash` and `password.verify` with scrypt from `node:crypto` (N = 16384, r = 16, p = 1, a 64-byte key), stored in PHC format, and hashed again at sign-in when the parameters change.
* A managed account is a better-auth user created through the server-side adapter with the e-mail value `managed:<id>`, which no sign-in accepts, since only the public routes require a valid e-mail address. Converting it into a full account sets a real e-mail address and a password.
* Mavio's own parts: claiming and `private_key_jwt`, sign-in assertions, library access records and invitations, friends, Homes, switching with PINs, Mavio Pass, the push to servers, the ACME challenges of §4.2 and relay tickets (§5).
* Servers call the Connect service `mavio.cloud.v1.ServerService` (`libs/proto`) under the API's base URL, `https://mavio.dev/api` unless a server's `mavio-dev-url` names another: `ClaimServer` registers a server's identity public key under the account of a claim code and returns that account with the server's current cursor, and `ListChanges` gives a claimed server's changes after a cursor. Every request carries a JWT the server signs with its identity key (`private_key_jwt`) as a bearer token: EdDSA, with the server's ID ([Accounts §5.1](accounts.md#51-server-identity)) as its `kid`, `iss` and `sub`, the base URL as its `aud`, a random `jti` and an expiry a minute after it was issued.

## 3. Push to Servers
* Each server in mavio.dev mode has a Durable Object holding the server's WebSocket connection. The connection hibernates between messages and is kept alive with protocol pings, which do not wake the object.
* Every change for a server ([access records](accounts.md#33-library-access), revocations, Home and account deletions) is stored with an increasing cursor. The Durable Object only notifies the server that its changes reached cursor N; the server then fetches the changes after its own cursor over HTTPS, with a request signed by its identity key.
* The server opens the WebSocket at `/servers/notices` under the API's base URL with the same JWT as its requests; each notice is a binary frame holding a `mavio.cloud.v1.Notice`. The server pings every 30 seconds, and a failed ping ends the connection.
* The server also fetches its changes whenever it reconnects, with exponential backoff and jitter, and every six hours, so that a lost notice delays a change but never loses it.

## 4. Certificates and Subdomains

### 4.1 Names
* Servers' names are under `mavio.site`, which is on the Public Suffix List, so that browsers keep the cookies and storage of different servers apart.
* A server in mavio.dev mode has the names `<address>.<ID>.mavio.site`, one for each of its addresses: IPv4 addresses are written with dashes (`192-168-1-10`), IPv6 addresses with dashes for colons (`2001-db8--1`).
* `apps/dns` is a stateless Go service built on `miekg/dns` that serves `mavio.site` from two Hetzner VMs, its name servers. It computes A and AAAA answers from the name, answers relay names with their region's relay addresses (§5), and serves a static SOA and NS, the `_psl` TXT record and CAA records allowing Let's Encrypt and Google Trust Services. It answers `_acme-challenge.<ID>.mavio.site` with a CNAME to `<ID>.acme.mavio.dev`.

### 4.2 Certificates
* Each server has one wildcard certificate, `*.<ID>.mavio.site`, from Let's Encrypt through ACME's DNS-01 challenge, which the server runs itself with `golang.org/x/crypto/acme`. The certificate's key is generated on the server and never leaves it.
* The server sends the challenge's TXT value to mavio.dev in a request signed with its identity key; mavio.dev accepts it for that server's own ID only, writes the TXT record `<ID>.acme.mavio.dev` in its Cloudflare zone through the Cloudflare API, and deletes it once the challenge is done.
* The server renews the certificate at two thirds of its lifetime, or when ACME Renewal Information suggests, and turns to Google Trust Services when Let's Encrypt fails. It serves the certificate through `tls.Config.GetCertificate`, so a renewal takes effect without a restart.

### 4.3 Connecting
Browsers reach a server by its names through DNS. Native clients connect to an address directly, with the address's name as the TLS server name, so that they also work where DNS answers with private addresses are filtered, and they check the server's identity key as well ([Accounts §5.1](accounts.md#51-server-identity)).

## 5. Relay (`apps/relay`)
* A Go service on Hetzner, in Europe, the United States and Singapore, for servers that cannot be reached directly. A server whose relay use is allowed keeps a connection to the relay of its region.
* A client reaches the server through the relay at `relay-<region>-<ticket>.<ID>.mavio.site`. The ticket is issued by mavio.dev, valid for a few minutes, and only when the viewer or the server's administrator has an effective Mavio Pass ([Accounts §3.7](accounts.md#37-mavio-pass)).
* The relay reads the name from the TLS ClientHello, checks the ticket and splices the client's connection to the server's without terminating TLS, so encryption stays end to end and the relay sees only encrypted bytes.

## 6. Deployment

| Where | What |
| :--- | :--- |
| Cloudflare | `apps/website` on Workers; the Durable Objects of §3; the `mavio.dev` DNS zone; R2 for downloads of Mavio releases; reachability checks of servers' published addresses with Workers' outbound TCP connections; e-mail through Cloudflare Email Service |
| Hetzner | PostgreSQL, with a streaming replica and its WAL archived to R2; `apps/relay`; `apps/dns` |

## 7. Security Notes

* **mavio.dev controls `mavio.site`** and could have certificates issued for a server's names; native clients also check the server's identity key, which mavio.dev does not hold the private half of.
* **The relay** sees which server a client reaches and how much it transfers, never the content.
* What a compromised mavio.dev could do to servers is described in [Accounts §10](accounts.md#10-security-notes).

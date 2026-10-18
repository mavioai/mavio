//! The Mavio account (`docs/accounts.md` §6): signing it in at mavio.dev,
//! in the browser through a loopback address or with a link code, the
//! session of the device key there, and following the account's servers,
//! each signed in to with an assertion of mavio.dev.

use std::collections::HashMap;
use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use aws_lc_rs::digest::{SHA256, digest};
use bytes::Bytes;
use connectrpc::ConnectError;
use connectrpc::client::{BoxFuture, ClientBody, ClientConfig, ClientTransport, HttpClient};
use http::header::{AUTHORIZATION, CONTENT_TYPE};
use http::{Request, Response, Uri};
use http_body_util::{BodyExt, Full};
use mavio_api::connect::mavio::cloud::v1::AccountServiceClient;
use mavio_api::proto::mavio::auth::v1::SignInWithAssertionRequest;
use mavio_api::proto::mavio::cloud::v1::{CreateAssertionRequest, ListServersRequest};
use serde::{Deserialize, Serialize};
use tokio::io::{AsyncReadExt as _, AsyncWriteExt as _};
use tokio::net::TcpListener;
use tokio::sync::Mutex;

use crate::connection::{Connection, tls_config};
use crate::error::{Error, Result};
use crate::servers::{Servers, TokenStore, probe};
use crate::session::{Credential, DeviceKey, expiry};

/// The key the account's session is kept under in the [`TokenStore`].
pub(crate) const ACCOUNT_ID: &str = "mavio-account";

/// mavio.dev's API, unless a server or the app names another.
pub const MAVIO_DEV_API: &str = "https://mavio.dev/api";

/// How long before its expiry the account's access token is renewed.
const RENEW_MARGIN: Duration = Duration::from_secs(60);

/// How long each of a server's URLs is tried.
const REACH_TIMEOUT: Duration = Duration::from_secs(4);

/// How long a call of mavio.dev may take.
const CALL_TIMEOUT: Duration = Duration::from_secs(20);

const AUTHORIZATION_CODE_GRANT: &str = "authorization_code";
const DEVICE_CODE_GRANT: &str = "urn:ietf:params:oauth:grant-type:device_code";
const RENEW_GRANT: &str = "urn:mavio:params:oauth:grant-type:device-signature";

/// The Mavio account signed in.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct Account {
    /// The account's ID.
    pub id: String,
    /// Its display name, or its user name without one.
    pub name: String,
    /// The API of the mavio.dev it is kept by.
    pub api: String,
}

/// What the device is signed in with, which decides whether an app shows
/// its sign-in.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum SignInState {
    /// Nothing: the app shows its sign-in.
    SignedOut,
    /// A Mavio account, signed in while none of its servers answers.
    Account(Account),
    /// A standalone server's user, or the users of servers added by their
    /// addresses.
    Standalone,
}

/// A link code: the device shows it, with the address to enter it at and
/// a QR code of the address with the code filled in, and the account
/// approves it there.
#[derive(Debug, Clone)]
pub struct LinkCode {
    /// The code to show, such as `WDJB-MJHT`.
    pub user_code: String,
    /// Where to enter it.
    pub verification_uri: String,
    /// The same with the code filled in, for the QR code.
    pub verification_uri_complete: String,
    /// When the code expires.
    pub expires: SystemTime,
    /// How often to ask whether it was approved.
    pub interval: Duration,
    device_code: String,
    api: String,
    client_id: String,
}

/// A sign-in in the browser: the app opens url, mavio.dev sends the
/// browser back to a loopback address of this device (RFC 8252), and
/// [`Servers::finish_browser_sign_in`] completes it.
pub struct BrowserSignIn {
    /// The page of mavio.dev to open.
    pub url: String,
    listener: Mutex<TcpListener>,
    redirect_uri: String,
    state: String,
    verifier: String,
    api: String,
    client_id: String,
}

/// The account's session at mavio.dev, as the [`TokenStore`] keeps it.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
struct AccountSession {
    api: String,
    client_id: String,
    session_id: String,
    token: String,
    /// When the token expires, in seconds since the Unix epoch.
    expires: u64,
    account_id: String,
    account_name: String,
}

impl AccountSession {
    fn load(tokens: &dyn TokenStore) -> Option<Self> {
        serde_json::from_str(&tokens.token(ACCOUNT_ID)?).ok()
    }

    fn save(&self, tokens: &dyn TokenStore) {
        let json = serde_json::to_string(self).expect("sessions serialize");
        tokens.set_token(ACCOUNT_ID, &json);
    }

    fn account(&self) -> Account {
        Account {
            id: self.account_id.clone(),
            name: self.account_name.clone(),
            api: self.api.clone(),
        }
    }
}

/// The answer of mavio.dev's token endpoint.
#[derive(Deserialize)]
struct TokenAnswer {
    access_token: String,
    expires_in: u64,
    #[serde(default)]
    session_id: String,
    #[serde(default)]
    account: Option<AccountAnswer>,
}

#[derive(Deserialize)]
struct AccountAnswer {
    id: String,
    #[serde(default, rename = "userName")]
    user_name: String,
    #[serde(default, rename = "displayName")]
    display_name: String,
}

#[derive(Deserialize)]
struct DeviceAnswer {
    device_code: String,
    user_code: String,
    verification_uri: String,
    verification_uri_complete: String,
    expires_in: u64,
    interval: u64,
}

/// An OAuth endpoint's failure: its error code, or the call's.
#[derive(Debug)]
enum OAuthError {
    Code(String, String),
    Call(Error),
}

impl From<OAuthError> for Error {
    fn from(e: OAuthError) -> Self {
        match e {
            OAuthError::Call(e) => e,
            OAuthError::Code(code, description) => {
                let m = format!("{code}: {description}");
                match code.as_str() {
                    "invalid_grant" | "invalid_client" | "unauthorized_client" => {
                        Error::Unauthenticated(m)
                    }
                    "access_denied" => Error::PermissionDenied(m),
                    "expired_token" => Error::NotFound(m),
                    "invalid_request" | "unsupported_grant_type" => Error::Invalid(m),
                    "temporarily_unavailable" => Error::Unreachable(m),
                    _ => Error::Server(m),
                }
            }
        }
    }
}

/// The account's part of [`Servers`]: the device key's session at
/// mavio.dev, renewed one at a time, and the servers followed one sync at
/// a time.
pub(crate) struct MavioDev {
    tokens: Arc<dyn TokenStore>,
    key: Arc<DeviceKey>,
    renewing: Mutex<()>,
    syncing: Mutex<()>,
}

impl MavioDev {
    pub(crate) fn new(tokens: Arc<dyn TokenStore>, key: Arc<DeviceKey>) -> Self {
        MavioDev {
            tokens,
            key,
            renewing: Mutex::default(),
            syncing: Mutex::default(),
        }
    }

    /// The account's access token, renewed with a signature of the device
    /// key when it is about to expire; none when signed out, or once
    /// mavio.dev ended the session.
    async fn token(&self) -> Result<Option<String>> {
        let _renewing = self.renewing.lock().await;
        let Some(mut s) = AccountSession::load(self.tokens.as_ref()) else {
            return Ok(None);
        };
        if SystemTime::now() + RENEW_MARGIN < UNIX_EPOCH + Duration::from_secs(s.expires) {
            return Ok(Some(s.token));
        }
        let timestamp = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_default()
            .as_millis()
            .to_string();
        let message = format!("mavio-dev-renew-v1\0{}\0{timestamp}", s.session_id);
        let signature = base64url(&self.key.sign(message.as_bytes()));
        let answer = oauth(
            &s.api,
            "token",
            &[
                ("grant_type", RENEW_GRANT),
                ("session_id", &s.session_id),
                ("timestamp", &timestamp),
                ("signature", &signature),
            ],
        )
        .await;
        let t: TokenAnswer = match answer {
            Ok(t) => t,
            // A session mavio.dev ended signs the account out.
            Err(OAuthError::Code(code, _)) if code == "invalid_grant" => {
                self.tokens.delete_token(ACCOUNT_ID);
                return Ok(None);
            }
            Err(e) => return Err(e.into()),
        };
        s.token = t.access_token;
        s.expires = seconds_from_now(t.expires_in);
        s.save(self.tokens.as_ref());
        Ok(Some(s.token))
    }

    fn account_service(
        self: &Arc<Self>,
        api: &str,
    ) -> Result<AccountServiceClient<AccountTransport>> {
        let uri: Uri = api
            .parse()
            .map_err(|e| Error::Invalid(format!("mavio.dev's address {api:?}: {e}")))?;
        let transport = AccountTransport {
            http: http_client(api),
            mavio_dev: self.clone(),
        };
        Ok(AccountServiceClient::new(
            transport,
            ClientConfig::new(uri).with_default_timeout(CALL_TIMEOUT),
        ))
    }
}

/// The transport of mavio.dev's services: the HTTP client, sending the
/// account's access token.
#[derive(Clone)]
pub(crate) struct AccountTransport {
    http: HttpClient,
    mavio_dev: Arc<MavioDev>,
}

impl ClientTransport for AccountTransport {
    type ResponseBody = hyper::body::Incoming;
    type Error = ConnectError;

    fn send(
        &self,
        mut request: Request<ClientBody>,
    ) -> BoxFuture<'static, std::result::Result<Response<Self::ResponseBody>, Self::Error>> {
        let http = self.http.clone();
        let mavio_dev = self.mavio_dev.clone();
        Box::pin(async move {
            let token = match mavio_dev.token().await {
                Ok(Some(t)) => t,
                Ok(None) => return Err(ConnectError::unauthenticated("signed out of mavio.dev")),
                Err(e) if e.is_transient() => return Err(ConnectError::unavailable(e.to_string())),
                Err(e) => return Err(ConnectError::internal(e.to_string())),
            };
            let value = http::HeaderValue::from_str(&format!("Bearer {token}"))
                .map_err(|_| ConnectError::internal("the access token is not a header value"))?;
            request.headers_mut().insert(AUTHORIZATION, value);
            http.send(request).await
        })
    }
}

fn http_client(address: &str) -> HttpClient {
    if address.starts_with("https:") {
        HttpClient::with_tls(tls_config())
    } else {
        HttpClient::plaintext()
    }
}

fn seconds_from_now(seconds: u64) -> u64 {
    (SystemTime::now() + Duration::from_secs(seconds))
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs()
}

/// The site of mavio.dev whose API is api, where its pages are.
fn site_of(api: &str) -> &str {
    let api = api.trim_end_matches('/');
    api.strip_suffix("/api").unwrap_or(api)
}

/// Posts fields to an OAuth endpoint of mavio.dev's API and decodes its
/// answer.
async fn oauth<T: for<'de> Deserialize<'de>>(
    api: &str,
    endpoint: &str,
    fields: &[(&str, &str)],
) -> std::result::Result<T, OAuthError> {
    let url = format!("{}/oauth/{endpoint}", api.trim_end_matches('/'));
    let body: ClientBody = Full::new(Bytes::from(form(fields)))
        .map_err(|e| match e {})
        .boxed();
    let request = Request::post(&url)
        .header(CONTENT_TYPE, "application/x-www-form-urlencoded")
        .body(body)
        .map_err(|e| OAuthError::Call(Error::Invalid(format!("{url}: {e}"))))?;
    let call = async {
        let response = http_client(api).send(request).await?;
        let status = response.status();
        let body = response
            .into_body()
            .collect()
            .await
            .map_err(|e| ConnectError::unavailable(e.to_string()))?
            .to_bytes();
        Ok::<_, ConnectError>((status, body))
    };
    let (status, body) = tokio::time::timeout(CALL_TIMEOUT, call)
        .await
        .map_err(|_| OAuthError::Call(Error::Unreachable(format!("{url} did not answer"))))?
        .map_err(|e| OAuthError::Call(e.into()))?;
    if !status.is_success() {
        #[derive(Deserialize)]
        struct Failure {
            error: Option<String>,
            error_description: Option<String>,
        }
        let f: Option<Failure> = serde_json::from_slice(&body).ok();
        let (code, description) = f
            .map(|f| (f.error, f.error_description))
            .unwrap_or_default();
        return Err(OAuthError::Code(
            code.unwrap_or_else(|| "server_error".into()),
            description.unwrap_or_else(|| status.to_string()),
        ));
    }
    serde_json::from_slice(&body)
        .map_err(|e| OAuthError::Call(Error::Server(format!("{url} answered: {e}"))))
}

impl Servers {
    /// The Mavio account signed in, if any.
    pub fn account(&self) -> Option<Account> {
        AccountSession::load(self.tokens().as_ref()).map(|s| s.account())
    }

    /// What the device is signed in with.
    pub fn sign_in_state(&self) -> Result<SignInState> {
        if let Some(a) = self.account() {
            return Ok(SignInState::Account(a));
        }
        Ok(if self.list()?.is_empty() {
            SignInState::SignedOut
        } else {
            SignInState::Standalone
        })
    }

    /// Starts signing a Mavio account in with a link code, at the mavio.dev
    /// whose API is api, as client_id, such as `dev.mavio.desktop`.
    pub async fn start_link(&self, api: &str, client_id: &str) -> Result<LinkCode> {
        let key = base64url(&self.device_key().public_key());
        let d: DeviceAnswer = oauth(
            api,
            "device",
            &[("client_id", client_id), ("device_key", &key)],
        )
        .await?;
        Ok(LinkCode {
            user_code: d.user_code,
            verification_uri: d.verification_uri,
            verification_uri_complete: d.verification_uri_complete,
            expires: SystemTime::now() + Duration::from_secs(d.expires_in),
            interval: Duration::from_secs(d.interval.max(1)),
            device_code: d.device_code,
            api: api.to_owned(),
            client_id: client_id.to_owned(),
        })
    }

    /// Asks once whether a link code was approved, signing the account in
    /// when it was; the app calls it every interval while it shows the
    /// code. An expired code is `NotFound`, a denied one
    /// `PermissionDenied`. The account's servers follow with
    /// [`Servers::sync_account`].
    pub async fn poll_link(&self, link: &LinkCode) -> Result<Option<Account>> {
        let answer = oauth(
            &link.api,
            "token",
            &[
                ("grant_type", DEVICE_CODE_GRANT),
                ("device_code", &link.device_code),
                ("client_id", &link.client_id),
            ],
        )
        .await;
        match answer {
            Ok(t) => self
                .start_account(&link.api, &link.client_id, t)
                .await
                .map(Some),
            Err(OAuthError::Code(code, _))
                if code == "authorization_pending" || code == "slow_down" =>
            {
                Ok(None)
            }
            Err(e) => Err(e.into()),
        }
    }

    /// Starts signing a Mavio account in in the browser, at the mavio.dev
    /// whose API is api, as client_id: listens on a loopback address for
    /// the browser to come back to.
    pub async fn start_browser_sign_in(&self, api: &str, client_id: &str) -> Result<BrowserSignIn> {
        let listener = TcpListener::bind(("127.0.0.1", 0))
            .await
            .map_err(|e| Error::Unreachable(format!("listen on a loopback address: {e}")))?;
        let port = listener
            .local_addr()
            .map_err(|e| Error::Unreachable(e.to_string()))?
            .port();
        let redirect_uri = format!("http://127.0.0.1:{port}/callback");
        let state = random_string();
        let verifier = random_string();
        let challenge = base64url(digest(&SHA256, verifier.as_bytes()).as_ref());
        let key = base64url(&self.device_key().public_key());
        let url = format!(
            "{}/authorize?{}",
            site_of(api),
            form(&[
                ("client_id", client_id),
                ("redirect_uri", &redirect_uri),
                ("state", &state),
                ("code_challenge", &challenge),
                ("code_challenge_method", "S256"),
                ("device_key", &key),
            ])
        );
        Ok(BrowserSignIn {
            url,
            listener: Mutex::new(listener),
            redirect_uri,
            state,
            verifier,
            api: api.to_owned(),
            client_id: client_id.to_owned(),
        })
    }

    /// Waits for the browser to come back from a sign-in and signs the
    /// account in; the app cancels it by dropping the future. A sign-in the
    /// account declined is `PermissionDenied`.
    pub async fn finish_browser_sign_in(&self, sign_in: &BrowserSignIn) -> Result<Account> {
        let params = {
            let listener = sign_in.listener.lock().await;
            loop {
                let (mut stream, _) = listener
                    .accept()
                    .await
                    .map_err(|e| Error::Unreachable(format!("the browser's callback: {e}")))?;
                let Some(target) = read_request_target(&mut stream).await else {
                    continue;
                };
                let (path, query) = target.split_once('?').unwrap_or((&target, ""));
                if path != "/callback" {
                    let _ = respond(&mut stream, "404 Not Found", "").await;
                    continue;
                }
                let params = parse_query(query);
                if params.get("state") != Some(&sign_in.state) {
                    let _ = respond(&mut stream, "400 Bad Request", CALLBACK_PAGE_FAILED).await;
                    continue;
                }
                let page = if params.contains_key("error") {
                    CALLBACK_PAGE_FAILED
                } else {
                    CALLBACK_PAGE
                };
                let _ = respond(&mut stream, "200 OK", page).await;
                break params;
            }
        };
        if let Some(error) = params.get("error") {
            let description = params.get("error_description").unwrap_or(error);
            return Err(OAuthError::Code(error.clone(), description.clone()).into());
        }
        let code = params.get("code").map(String::as_str).unwrap_or_default();
        let t: TokenAnswer = oauth(
            &sign_in.api,
            "token",
            &[
                ("grant_type", AUTHORIZATION_CODE_GRANT),
                ("code", code),
                ("client_id", &sign_in.client_id),
                ("redirect_uri", &sign_in.redirect_uri),
                ("code_verifier", &sign_in.verifier),
            ],
        )
        .await?;
        self.start_account(&sign_in.api, &sign_in.client_id, t)
            .await
    }

    async fn start_account(&self, api: &str, client_id: &str, t: TokenAnswer) -> Result<Account> {
        let a = t
            .account
            .ok_or_else(|| Error::Server("mavio.dev did not tell the account".into()))?;
        // Another account's servers go with its sign-in.
        if self.account().is_some_and(|was| was.id != a.id) {
            self.sign_out_account().await?;
        }
        let name = if a.display_name.is_empty() {
            a.user_name
        } else {
            a.display_name
        };
        let s = AccountSession {
            api: api.to_owned(),
            client_id: client_id.to_owned(),
            session_id: t.session_id,
            token: t.access_token,
            expires: seconds_from_now(t.expires_in),
            account_id: a.id,
            account_name: name,
        };
        s.save(self.tokens().as_ref());
        Ok(s.account())
    }

    /// Follows the account's servers: signs in to those it newly reaches,
    /// at the first of their URLs that answers as the server, and removes
    /// those it no longer reaches, with the servers signed in to without
    /// it. Returns whether the servers changed; one sync runs at a time.
    pub async fn sync_account(&self) -> Result<bool> {
        let mavio_dev = self.mavio_dev();
        let _syncing = mavio_dev.syncing.lock().await;
        let Some(session) = AccountSession::load(self.tokens().as_ref()) else {
            return Ok(false);
        };
        let accounts = mavio_dev.account_service(&session.api)?;
        let listed = accounts
            .list_servers(ListServersRequest::default())
            .await?
            .into_owned()
            .servers;
        let mut changed = false;
        for s in self.list()? {
            if !listed
                .iter()
                .any(|l| l.id.as_deref() == Some(s.id.as_str()))
            {
                self.remove(&s.id).await?;
                changed = true;
            }
        }
        for s in listed {
            let id = s.id.unwrap_or_default();
            if id.is_empty() || self.signed_in(&id) {
                continue;
            }
            // An unreachable server is tried again with the next sync.
            let Some((conn, info)) = reach(&id, &s.connections).await else {
                continue;
            };
            let assertion = accounts
                .create_assertion(CreateAssertionRequest {
                    server_id: Some(id.clone()),
                    device_key: Some(self.device_key().public_key()),
                    ..Default::default()
                })
                .await?
                .into_owned()
                .assertion
                .unwrap_or_default();
            let message = format!("mavio-sign-in-assertion-v1\0{assertion}");
            let resp = conn
                .auth()
                .sign_in_with_assertion(SignInWithAssertionRequest {
                    signature: Some(self.device_key().sign(message.as_bytes())),
                    assertion: Some(assertion),
                    device: self.device_proto().into(),
                    ..Default::default()
                })
                .await?
                .into_owned();
            self.signed_in_as(
                &conn,
                &info,
                Credential {
                    session: resp
                        .session
                        .into_option()
                        .and_then(|s| s.id)
                        .unwrap_or_default(),
                    token: resp.access_token.unwrap_or_default(),
                    expires: expiry(resp.access_token_expire_time.into_option()),
                },
                resp.user.into_option(),
            )?;
            changed = true;
        }
        Ok(changed)
    }

    /// Signs the account out: ends every server's session and the
    /// account's session at mavio.dev, and removes the servers.
    pub async fn sign_out_account(&self) -> Result<()> {
        let session = AccountSession::load(self.tokens().as_ref());
        self.tokens().delete_token(ACCOUNT_ID);
        for s in self.list()? {
            self.remove(&s.id).await?;
        }
        if let Some(s) = session {
            // An unrevoked token expires unused.
            let _ = oauth::<serde_json::Value>(&s.api, "revoke", &[("token", &s.token)]).await;
        }
        Ok(())
    }
}

/// The first of a server's URLs that answers as the server with ID id.
async fn reach(id: &str, urls: &[String]) -> Option<(Connection, crate::servers::ServerInfo)> {
    for url in urls {
        let Ok(conn) = Connection::new(url) else {
            continue;
        };
        if let Ok(Ok(info)) = tokio::time::timeout(REACH_TIMEOUT, probe(&conn)).await
            && info.id == id
        {
            return Some((conn, info));
        }
    }
    None
}

const CALLBACK_PAGE: &str = "<!doctype html><meta charset=utf-8><title>Mavio</title>\
<body style=\"font:16px system-ui;text-align:center;padding:4em\">\
<p>You are signed in. Return to Mavio.</p>";

const CALLBACK_PAGE_FAILED: &str = "<!doctype html><meta charset=utf-8><title>Mavio</title>\
<body style=\"font:16px system-ui;text-align:center;padding:4em\">\
<p>The sign-in did not complete. Return to Mavio.</p>";

/// The target of an HTTP request's first line, such as `/callback?code=x`.
async fn read_request_target(stream: &mut tokio::net::TcpStream) -> Option<String> {
    let mut head = Vec::new();
    let mut buf = [0u8; 1024];
    while !head.windows(4).any(|w| w == b"\r\n\r\n") {
        let n = tokio::time::timeout(Duration::from_secs(5), stream.read(&mut buf))
            .await
            .ok()?
            .ok()?;
        if n == 0 || head.len() > 16 * 1024 {
            return None;
        }
        head.extend_from_slice(&buf[..n]);
    }
    let line = head.split(|&b| b == b'\r').next()?;
    let line = std::str::from_utf8(line).ok()?;
    let mut parts = line.split(' ');
    (parts.next()? == "GET").then_some(())?;
    parts.next().map(str::to_owned)
}

async fn respond(
    stream: &mut tokio::net::TcpStream,
    status: &str,
    body: &str,
) -> std::io::Result<()> {
    let response = format!(
        "HTTP/1.1 {status}\r\nContent-Type: text/html; charset=utf-8\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
        body.len()
    );
    stream.write_all(response.as_bytes()).await?;
    stream.shutdown().await
}

fn random_string() -> String {
    let mut b = [0u8; 32];
    aws_lc_rs::rand::fill(&mut b).expect("the system makes random numbers");
    base64url(&b)
}

/// Unpadded base64url.
pub(crate) fn base64url(b: &[u8]) -> String {
    const ALPHABET: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_";
    let mut out = String::with_capacity(b.len().div_ceil(3) * 4);
    for chunk in b.chunks(3) {
        let n = chunk
            .iter()
            .enumerate()
            .fold(0u32, |n, (i, &b)| n | u32::from(b) << (16 - 8 * i));
        for i in 0..=chunk.len() {
            out.push(char::from(ALPHABET[(n >> (18 - 6 * i) & 63) as usize]));
        }
    }
    out
}

/// Fields encoded as `application/x-www-form-urlencoded`, which is also a
/// URL's query.
fn form(fields: &[(&str, &str)]) -> String {
    fields
        .iter()
        .map(|(k, v)| format!("{}={}", percent_encode(k), percent_encode(v)))
        .collect::<Vec<_>>()
        .join("&")
}

fn percent_encode(s: &str) -> String {
    let mut out = String::with_capacity(s.len());
    for b in s.bytes() {
        if b.is_ascii_alphanumeric() || matches!(b, b'-' | b'.' | b'_' | b'~') {
            out.push(char::from(b));
        } else {
            out.push_str(&format!("%{b:02X}"));
        }
    }
    out
}

/// The fields of a URL's query.
fn parse_query(query: &str) -> HashMap<String, String> {
    query
        .split('&')
        .filter(|p| !p.is_empty())
        .map(|p| {
            let (k, v) = p.split_once('=').unwrap_or((p, ""));
            (percent_decode(k), percent_decode(v))
        })
        .collect()
}

fn percent_decode(s: &str) -> String {
    let b = s.as_bytes();
    let mut out = Vec::with_capacity(b.len());
    let mut i = 0;
    while i < b.len() {
        match b[i] {
            b'+' => out.push(b' '),
            b'%' => match s
                .get(i + 1..i + 3)
                .and_then(|h| u8::from_str_radix(h, 16).ok())
            {
                Some(v) => {
                    out.push(v);
                    i += 3;
                    continue;
                }
                None => out.push(b'%'),
            },
            c => out.push(c),
        }
        i += 1;
    }
    String::from_utf8_lossy(&out).into_owned()
}

#[cfg(test)]
mod tests {
    use std::sync::Mutex as StdMutex;

    use http::StatusCode;
    use mavio_api::proto::mavio::auth::v1::{Session as SessionProto, SignInWithAssertionResponse};
    use mavio_api::proto::mavio::cloud::v1::{
        CreateAssertionResponse, ListServersResponse, Server,
    };
    use tokio::net::TcpStream;

    use super::*;
    use crate::fake::{Fake, error};
    use crate::servers::tests::{SESSION, fake_server, servers};

    fn verify(key: &[u8], message: &[u8], signature: &[u8]) -> bool {
        aws_lc_rs::signature::UnparsedPublicKey::new(&aws_lc_rs::signature::ED25519, key)
            .verify(message, signature)
            .is_ok()
    }

    fn from_base64url(s: &str) -> Vec<u8> {
        let value = |c: u8| match c {
            b'A'..=b'Z' => c - b'A',
            b'a'..=b'z' => c - b'a' + 26,
            b'0'..=b'9' => c - b'0' + 52,
            b'-' => 62,
            _ => 63,
        };
        let bits: Vec<u8> = s.bytes().map(value).collect();
        bits.chunks(4)
            .flat_map(|c| {
                let n = c
                    .iter()
                    .enumerate()
                    .fold(0u32, |n, (i, &v)| n | u32::from(v) << (18 - 6 * i));
                (0..c.len() - 1).map(move |i| (n >> (16 - 8 * i)) as u8)
            })
            .collect()
    }

    /// What the fake mavio.dev saw and decides.
    #[derive(Default)]
    struct State {
        approved: bool,
        /// The device key the account's session is bound to.
        device_key: Vec<u8>,
        /// The lifetime of the tokens it gives.
        expires_in: u64,
        renewed: u32,
        ended: bool,
        revoked: Vec<String>,
        /// The servers the account reaches.
        servers: Vec<(String, Vec<String>)>,
    }

    fn json(status: StatusCode, v: serde_json::Value) -> (StatusCode, String) {
        (status, v.to_string())
    }

    fn fail(code: &str) -> (StatusCode, String) {
        json(
            StatusCode::BAD_REQUEST,
            serde_json::json!({"error": code, "error_description": code}),
        )
    }

    /// A fake mavio.dev; its API is at `<address>/api`.
    async fn mavio_dev(state: Arc<StdMutex<State>>) -> String {
        let fake = Fake::default();
        let token = |s: &State, token: &str| {
            json(
                StatusCode::OK,
                serde_json::json!({
                    "access_token": token,
                    "token_type": "Bearer",
                    "expires_in": s.expires_in,
                    "session_id": "ses-1",
                    "account": {"id": "acc", "userName": "ann", "displayName": "Ann"},
                }),
            )
        };
        let st = state.clone();
        fake.http("/api/oauth/device", move |_, body| {
            let f = parse_query(std::str::from_utf8(&body).unwrap());
            assert_eq!(f["client_id"], "dev.mavio.test");
            st.lock().unwrap().device_key = from_base64url(&f["device_key"]);
            json(
                StatusCode::OK,
                serde_json::json!({
                    "device_code": "dc", "user_code": "WDJB-MJHT",
                    "verification_uri": "https://mavio.dev/link",
                    "verification_uri_complete": "https://mavio.dev/link?code=WDJB-MJHT",
                    "expires_in": 600, "interval": 5,
                }),
            )
        });
        let st = state.clone();
        fake.http("/api/oauth/token", move |_, body| {
            let f = parse_query(std::str::from_utf8(&body).unwrap());
            let mut s = st.lock().unwrap();
            match f["grant_type"].as_str() {
                DEVICE_CODE_GRANT if f["device_code"] == "dc" => {
                    if s.approved {
                        token(&s, "a-0")
                    } else {
                        fail("authorization_pending")
                    }
                }
                AUTHORIZATION_CODE_GRANT => {
                    if f["code"] != "c1" || f["code_verifier"].len() < 43 {
                        return fail("invalid_grant");
                    }
                    token(&s, "a-0")
                }
                RENEW_GRANT => {
                    let message = format!(
                        "mavio-dev-renew-v1\0{}\0{}",
                        f["session_id"], f["timestamp"]
                    );
                    if s.ended
                        || !verify(
                            &s.device_key,
                            message.as_bytes(),
                            &from_base64url(&f["signature"]),
                        )
                    {
                        return fail("invalid_grant");
                    }
                    s.renewed += 1;
                    s.expires_in = 600;
                    token(&s, &format!("a-{}", s.renewed))
                }
                _ => fail("invalid_grant"),
            }
        });
        let st = state.clone();
        fake.http("/api/oauth/revoke", move |_, body| {
            let f = parse_query(std::str::from_utf8(&body).unwrap());
            st.lock().unwrap().revoked.push(f["token"].clone());
            json(StatusCode::OK, serde_json::json!({}))
        });
        let st = state.clone();
        fake.unary(
            "/api/mavio.cloud.v1.AccountService/ListServers",
            move |head, _: ListServersRequest| {
                let s = st.lock().unwrap();
                let want = format!("Bearer a-{}", s.renewed);
                if head.headers()["authorization"] != want.as_str() {
                    return Err(error("unauthenticated"));
                }
                Ok(ListServersResponse {
                    servers: s
                        .servers
                        .iter()
                        .map(|(id, urls)| Server {
                            id: Some(id.clone()),
                            name: Some(id.clone()),
                            connections: urls.clone(),
                            ..Default::default()
                        })
                        .collect(),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/api/mavio.cloud.v1.AccountService/CreateAssertion",
            move |_, req: CreateAssertionRequest| {
                Ok(CreateAssertionResponse {
                    assertion: Some(format!("as-{}", req.server_id.unwrap_or_default())),
                    ..Default::default()
                })
            },
        );
        format!("{}/api", fake.serve().await)
    }

    /// A fake server with ID id that signs in the device holding the
    /// assertion mavio.dev made for it.
    async fn account_server(id: &'static str) -> String {
        let fake = fake_server(id, "Home");
        fake.unary(
            "/mavio.auth.v1.AuthService/SignInWithAssertion",
            move |_, req: SignInWithAssertionRequest| {
                let assertion = req.assertion.clone().unwrap_or_default();
                let message = format!("mavio-sign-in-assertion-v1\0{assertion}");
                if assertion != format!("as-{id}")
                    || !verify(
                        req.device.public_key.as_deref().unwrap_or_default(),
                        message.as_bytes(),
                        req.signature.as_deref().unwrap_or_default(),
                    )
                {
                    return Err(error("unauthenticated"));
                }
                Ok(SignInWithAssertionResponse {
                    access_token: Some(format!("t-{id}")),
                    session: SessionProto {
                        id: Some(SESSION.into()),
                        ..Default::default()
                    }
                    .into(),
                    ..Default::default()
                })
            },
        );
        fake.serve().await
    }

    fn state(servers: &[(&str, &[&str])]) -> Arc<StdMutex<State>> {
        Arc::new(StdMutex::new(State {
            expires_in: 600,
            servers: servers
                .iter()
                .map(|(id, urls)| (id.to_string(), urls.iter().map(|u| u.to_string()).collect()))
                .collect(),
            ..Default::default()
        }))
    }

    fn ids(servers: &Servers) -> Vec<String> {
        servers.list().unwrap().into_iter().map(|s| s.id).collect()
    }

    #[tokio::test]
    async fn link_codes_sign_the_account_in_to_its_servers() {
        let home = account_server("sa").await;
        let st = state(&[
            ("sa", &["http://127.0.0.1:1", &home]),
            ("gone", &["http://127.0.0.1:1"]),
        ]);
        let api = mavio_dev(st.clone()).await;
        let (servers, _) = servers();

        let link = servers.start_link(&api, "dev.mavio.test").await.unwrap();
        assert_eq!(link.user_code, "WDJB-MJHT");
        assert_eq!(link.interval, Duration::from_secs(5));
        assert_eq!(
            st.lock().unwrap().device_key,
            servers.device_key().public_key()
        );
        assert_eq!(servers.poll_link(&link).await.unwrap(), None);
        assert!(servers.account().is_none());
        assert_eq!(servers.sign_in_state().unwrap(), SignInState::SignedOut);

        st.lock().unwrap().approved = true;
        let got = servers.poll_link(&link).await.unwrap().unwrap();
        assert_eq!(
            got,
            Account {
                id: "acc".into(),
                name: "Ann".into(),
                api: api.clone()
            }
        );
        assert_eq!(servers.account(), Some(got.clone()));
        assert_eq!(
            servers.sign_in_state().unwrap(),
            SignInState::Account(got.clone()),
            "signed in before any of its servers answers"
        );

        assert!(servers.sync_account().await.unwrap(), "the servers changed");
        assert_eq!(ids(&servers), ["sa"], "the unreachable server waits");
        let s = servers.get("sa").unwrap();
        assert_eq!(s.address, home, "reached at the URL that answered");
        assert_eq!(
            servers
                .connection("sa")
                .unwrap()
                .token()
                .await
                .unwrap()
                .as_deref(),
            Some("t-sa")
        );
        assert!(!servers.sync_account().await.unwrap(), "nothing changed");

        // A server the account no longer reaches is removed.
        st.lock().unwrap().servers.clear();
        assert!(servers.sync_account().await.unwrap());
        assert!(ids(&servers).is_empty());
        assert_eq!(
            servers.sign_in_state().unwrap(),
            SignInState::Account(got),
            "the account stays signed in without servers"
        );
    }

    #[tokio::test]
    async fn sessions_renew_with_the_device_key() {
        let st = state(&[]);
        st.lock().unwrap().approved = true;
        st.lock().unwrap().expires_in = 0;
        let api = mavio_dev(st.clone()).await;
        let (servers, tokens) = servers();
        let link = servers.start_link(&api, "dev.mavio.test").await.unwrap();
        servers.poll_link(&link).await.unwrap().unwrap();

        servers.sync_account().await.unwrap();
        servers.sync_account().await.unwrap();
        assert_eq!(st.lock().unwrap().renewed, 1, "renewed once, as it expired");
        assert_eq!(AccountSession::load(tokens.as_ref()).unwrap().token, "a-1");

        // A session mavio.dev ended signs the account out.
        let mut s = AccountSession::load(tokens.as_ref()).unwrap();
        s.expires = 0;
        s.save(tokens.as_ref());
        st.lock().unwrap().ended = true;
        let got = servers.sync_account().await;
        assert!(
            matches!(got, Err(Error::Unauthenticated(_))),
            "got = {got:?}"
        );
        assert!(servers.account().is_none());
    }

    #[tokio::test]
    async fn signing_out_ends_every_session() {
        let home = account_server("sa").await;
        let st = state(&[("sa", &[&home])]);
        st.lock().unwrap().approved = true;
        let api = mavio_dev(st.clone()).await;
        let (servers, tokens) = servers();
        let link = servers.start_link(&api, "dev.mavio.test").await.unwrap();
        servers.poll_link(&link).await.unwrap().unwrap();
        servers.sync_account().await.unwrap();

        servers.sign_out_account().await.unwrap();
        assert!(servers.account().is_none());
        assert!(ids(&servers).is_empty());
        assert!(tokens.token("sa").is_none());
        assert_eq!(st.lock().unwrap().revoked, ["a-0"]);
    }

    /// Sends the browser's request for target to the sign-in's loopback
    /// address and returns the status line of the answer.
    async fn visit(redirect_uri: &str, target: &str) -> String {
        let authority = redirect_uri
            .strip_prefix("http://")
            .and_then(|r| r.split('/').next())
            .unwrap();
        let mut stream = TcpStream::connect(authority).await.unwrap();
        stream
            .write_all(format!("GET {target} HTTP/1.1\r\nHost: {authority}\r\n\r\n").as_bytes())
            .await
            .unwrap();
        let mut answer = String::new();
        stream.read_to_string(&mut answer).await.unwrap();
        answer.lines().next().unwrap_or_default().to_owned()
    }

    #[tokio::test]
    async fn browsers_come_back_to_a_loopback_address() {
        let st = state(&[]);
        let api = mavio_dev(st.clone()).await;
        let (servers, _) = servers();

        let sign_in = servers
            .start_browser_sign_in(&api, "dev.mavio.test")
            .await
            .unwrap();
        let (page, query) = sign_in.url.split_once('?').unwrap();
        assert_eq!(
            page,
            format!("{}/authorize", api.strip_suffix("/api").unwrap())
        );
        let q = parse_query(query);
        assert_eq!(q["client_id"], "dev.mavio.test");
        assert_eq!(q["code_challenge_method"], "S256");
        assert_eq!(
            q["code_challenge"],
            base64url(digest(&SHA256, sign_in.verifier.as_bytes()).as_ref())
        );
        assert_eq!(
            from_base64url(&q["device_key"]),
            servers.device_key().public_key()
        );
        let redirect = q["redirect_uri"].clone();
        assert!(
            redirect.starts_with("http://127.0.0.1:"),
            "got = {redirect}"
        );

        let state = q["state"].clone();
        let browser = async {
            assert_eq!(
                visit(&redirect, "/favicon.ico").await,
                "HTTP/1.1 404 Not Found"
            );
            assert_eq!(
                visit(&redirect, "/callback?code=c1&state=forged").await,
                "HTTP/1.1 400 Bad Request"
            );
            assert_eq!(
                visit(&redirect, &format!("/callback?code=c1&state={state}")).await,
                "HTTP/1.1 200 OK"
            );
        };
        let (got, ()) = tokio::join!(servers.finish_browser_sign_in(&sign_in), browser);
        assert_eq!(got.unwrap().name, "Ann");

        // A sign-in the account declined.
        let sign_in = servers
            .start_browser_sign_in(&api, "dev.mavio.test")
            .await
            .unwrap();
        let q = parse_query(sign_in.url.split_once('?').unwrap().1);
        let target = format!("/callback?error=access_denied&state={}", q["state"]);
        let (got, _) = tokio::join!(
            servers.finish_browser_sign_in(&sign_in),
            visit(&q["redirect_uri"], &target)
        );
        assert!(
            matches!(got, Err(Error::PermissionDenied(_))),
            "got = {got:?}"
        );
    }

    #[test]
    fn encodings() {
        let cases: [(&[u8], &str); 5] = [
            (b"", ""),
            (b"f", "Zg"),
            (b"fo", "Zm8"),
            (b"foo", "Zm9v"),
            (&[0xfb, 0xff, 0xbf], "-_-_"),
        ];
        for (input, want) in cases {
            assert_eq!(base64url(input), want, "base64url({input:?})");
            assert_eq!(from_base64url(want), input);
        }
        let encoded = form(&[("a b", "x/y?z&=é")]);
        assert_eq!(encoded, "a%20b=x%2Fy%3Fz%26%3D%C3%A9");
        assert_eq!(parse_query(&encoded)["a b"], "x/y?z&=é");
        assert_eq!(parse_query("a=1+2&b")["a"], "1 2");
        assert_eq!(site_of("https://mavio.dev/api/"), "https://mavio.dev");
        assert_eq!(site_of("http://localhost:5173"), "http://localhost:5173");
    }
}

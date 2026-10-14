//! Sessions (`docs/accounts.md` §5.3): the installation's Ed25519 device
//! key, the credentials of a server's session, and the transport that calls
//! a server with the session's access token, refreshing it by signing the
//! server's nonce as it expires.

use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use aws_lc_rs::signature::{Ed25519KeyPair, KeyPair as _};
use bytes::Bytes;
use connectrpc::ConnectError;
use connectrpc::client::{BoxFuture, ClientBody, ClientConfig, ClientTransport, HttpClient};
use http::{HeaderValue, Request, Response, StatusCode, header::AUTHORIZATION};
use http_body_util::{BodyExt, Full};
use mavio_api::connect::mavio::auth::v1::AuthServiceClient;
use mavio_api::proto::mavio::auth::v1::{RefreshSessionRequest, StartRefreshRequest};
use serde::{Deserialize, Serialize};
use tokio::sync::Mutex;

use crate::error::{Error, Result};
use crate::servers::TokenStore;

/// The key the device key's seed is kept under in the [`TokenStore`].
pub(crate) const DEVICE_KEY_ID: &str = "device-key";

/// How long before its expiry an access token is refreshed.
const REFRESH_MARGIN: Duration = Duration::from_secs(5 * 60);

/// What a device signs to refresh its session, before the session ID and
/// the nonce.
const REFRESH_CONTEXT: &[u8] = b"mavio-session-refresh-v1\0";

/// The installation's Ed25519 key, whose public half the servers bind its
/// sessions to.
pub(crate) struct DeviceKey(Ed25519KeyPair);

impl DeviceKey {
    /// The key kept in tokens, made and kept the first time.
    pub(crate) fn load(tokens: &dyn TokenStore) -> Self {
        if let Some(key) = tokens
            .token(DEVICE_KEY_ID)
            .and_then(|hex| from_hex(&hex))
            .and_then(|seed| Ed25519KeyPair::from_seed_unchecked(&seed).ok())
        {
            return DeviceKey(key);
        }
        let mut seed = [0u8; 32];
        aws_lc_rs::rand::fill(&mut seed).expect("the system makes random numbers");
        tokens.set_token(DEVICE_KEY_ID, &to_hex(&seed));
        DeviceKey(Ed25519KeyPair::from_seed_unchecked(&seed).expect("32-byte seeds make keys"))
    }

    /// The 32-byte public key.
    pub(crate) fn public_key(&self) -> Vec<u8> {
        self.0.public_key().as_ref().to_vec()
    }

    /// The signature refreshing session_id with nonce.
    pub(crate) fn sign_refresh(&self, session_id: &str, nonce: &[u8]) -> Result<Vec<u8>> {
        let id = from_hex(&session_id.replace('-', ""))
            .filter(|id| id.len() == 16)
            .ok_or_else(|| Error::Invalid(format!("session ID {session_id:?}")))?;
        let mut message = Vec::with_capacity(REFRESH_CONTEXT.len() + id.len() + nonce.len());
        message.extend_from_slice(REFRESH_CONTEXT);
        message.extend_from_slice(&id);
        message.extend_from_slice(nonce);
        Ok(self.0.sign(&message).as_ref().to_vec())
    }
}

fn to_hex(b: &[u8]) -> String {
    b.iter().map(|b| format!("{b:02x}")).collect()
}

fn from_hex(s: &str) -> Option<Vec<u8>> {
    if !s.len().is_multiple_of(2) {
        return None;
    }
    (0..s.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(s.get(i..i + 2)?, 16).ok())
        .collect()
}

/// A server's session as the [`TokenStore`] keeps it.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize)]
pub(crate) struct Credential {
    /// The session's ID.
    pub session: String,
    /// Its access token.
    pub token: String,
    /// When the token expires, in seconds since the Unix epoch; 0 when
    /// unknown.
    pub expires: u64,
}

impl Credential {
    /// The credential kept for a server, if any.
    pub(crate) fn load(tokens: &dyn TokenStore, server_id: &str) -> Option<Self> {
        serde_json::from_str(&tokens.token(server_id)?).ok()
    }

    pub(crate) fn save(&self, tokens: &dyn TokenStore, server_id: &str) {
        let json = serde_json::to_string(self).expect("credentials serialize");
        tokens.set_token(server_id, &json);
    }

    fn expires_soon(&self) -> bool {
        self.expires != 0
            && SystemTime::now() + REFRESH_MARGIN >= UNIX_EPOCH + Duration::from_secs(self.expires)
    }
}

/// The seconds since the Unix epoch of a response's expiry; 0 when unset.
pub(crate) fn expiry<T: TryInto<SystemTime>>(t: Option<T>) -> u64 {
    t.and_then(|t| t.try_into().ok())
        .and_then(|t| t.duration_since(UNIX_EPOCH).ok())
        .map_or(0, |d| d.as_secs())
}

/// A server's session, which refreshes its access token.
pub(crate) struct Session {
    server_id: String,
    tokens: Arc<dyn TokenStore>,
    key: Arc<DeviceKey>,
    /// The signed-out client of the server, which refreshes the session.
    auth: AuthServiceClient<HttpClient>,
    /// Held while refreshing, so that one refresh runs at a time.
    credential: Mutex<Credential>,
}

impl Session {
    pub(crate) fn new(
        server_id: &str,
        credential: Credential,
        tokens: Arc<dyn TokenStore>,
        key: Arc<DeviceKey>,
        http: HttpClient,
        config: ClientConfig,
    ) -> Self {
        Session {
            server_id: server_id.to_owned(),
            tokens,
            key,
            auth: AuthServiceClient::new(http, config),
            credential: Mutex::new(credential),
        }
    }

    /// Refreshes the access token now.
    pub(crate) async fn refresh_now(&self) -> Result<()> {
        let mut c = self.credential.lock().await;
        self.refresh(&mut c).await
    }

    /// The access token, refreshed first when it is about to expire.
    pub(crate) async fn token(&self) -> Result<String> {
        let mut c = self.credential.lock().await;
        if c.expires_soon() {
            self.refresh(&mut c).await?;
        }
        Ok(c.token.clone())
    }

    /// A new access token after the server rejected `rejected`, unless
    /// another call refreshed it meanwhile. A session the server no longer
    /// refreshes is `Unauthenticated`.
    pub(crate) async fn refreshed(&self, rejected: &str) -> Result<String> {
        let mut c = self.credential.lock().await;
        if c.token == rejected {
            self.refresh(&mut c).await?;
        }
        Ok(c.token.clone())
    }

    async fn refresh(&self, c: &mut Credential) -> Result<()> {
        let start = self
            .auth
            .start_refresh(StartRefreshRequest {
                session_id: Some(c.session.clone()),
                ..Default::default()
            })
            .await?
            .into_owned();
        let nonce = start.nonce.unwrap_or_default();
        let signature = self.key.sign_refresh(&c.session, &nonce)?;
        let resp = self
            .auth
            .refresh_session(RefreshSessionRequest {
                session_id: Some(c.session.clone()),
                nonce: Some(nonce),
                signature: Some(signature),
                ..Default::default()
            })
            .await?
            .into_owned();
        c.token = resp.access_token.unwrap_or_default();
        c.expires = expiry(resp.access_token_expire_time.into_option());
        c.save(self.tokens.as_ref(), &self.server_id);
        Ok(())
    }
}

/// The transport of a connection: the HTTP client, sending the session's
/// access token when signed in. A unary call the server rejects for the
/// token is made again once with a refreshed one.
#[derive(Clone)]
pub struct Transport {
    pub(crate) http: HttpClient,
    pub(crate) session: Option<Arc<Session>>,
}

fn bearer(token: &str) -> std::result::Result<HeaderValue, ConnectError> {
    HeaderValue::from_str(&format!("Bearer {token}"))
        .map_err(|_| ConnectError::internal("the access token is not a header value"))
}

fn connect_error(e: Error) -> ConnectError {
    match e {
        Error::Unauthenticated(m) => ConnectError::unauthenticated(m),
        e if e.is_transient() => ConnectError::unavailable(e.to_string()),
        e => ConnectError::internal(e.to_string()),
    }
}

impl ClientTransport for Transport {
    type ResponseBody = hyper::body::Incoming;
    type Error = ConnectError;

    fn send(
        &self,
        request: Request<ClientBody>,
    ) -> BoxFuture<'static, std::result::Result<Response<Self::ResponseBody>, Self::Error>> {
        let http = self.http.clone();
        let Some(session) = self.session.clone() else {
            return http.send(request);
        };
        Box::pin(async move {
            let token = session.token().await.map_err(connect_error)?;
            // The body is kept to send it again; requests are small.
            let (mut parts, body) = request.into_parts();
            let body: Bytes = body.collect().await?.to_bytes();
            let with = |parts: &http::request::Parts, token: &str| {
                let mut req = Request::from_parts(
                    parts.clone(),
                    Full::new(body.clone()).map_err(|e| match e {}).boxed(),
                );
                bearer(token).map(|v| {
                    req.headers_mut().insert(AUTHORIZATION, v);
                    req
                })
            };
            parts.headers.remove(AUTHORIZATION);
            let resp = http.send(with(&parts, &token)?).await?;
            // Streams fail within a 200 response; their callers refresh.
            if resp.status() != StatusCode::UNAUTHORIZED {
                return Ok(resp);
            }
            let fresh = session.refreshed(&token).await.map_err(connect_error)?;
            http.send(with(&parts, &fresh)?).await
        })
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn hex_round_trips() {
        assert_eq!(
            from_hex(&to_hex(&[0, 1, 0xab, 0xff])),
            Some(vec![0, 1, 0xab, 0xff])
        );
        assert_eq!(from_hex("abc"), None);
        assert_eq!(from_hex("zz"), None);
    }

    #[test]
    fn refresh_signatures_verify() {
        let tokens = crate::servers::tests::MemoryTokens::default();
        let key = DeviceKey::load(&tokens);
        let again = DeviceKey::load(&tokens);
        assert_eq!(key.public_key(), again.public_key(), "the key is kept");

        let nonce = [7u8; 32];
        let sig = key
            .sign_refresh("01890a5d-ac96-774b-bcce-b302099a8057", &nonce)
            .unwrap();
        let mut message = REFRESH_CONTEXT.to_vec();
        message.extend_from_slice(&from_hex("01890a5dac96774bbcceb302099a8057").unwrap());
        message.extend_from_slice(&nonce);
        let public = aws_lc_rs::signature::UnparsedPublicKey::new(
            &aws_lc_rs::signature::ED25519,
            key.public_key(),
        );
        assert!(public.verify(&message, &sig).is_ok());
        assert!(key.sign_refresh("not an ID", &nonce).is_err());
    }
}

//! The servers a client is signed in to (`docs/clients.md` §5.4): adding
//! them by address, signing in with a password or Quick Connect, their
//! order, and their connections.

use std::collections::HashMap;
use std::sync::{Arc, RwLock};
use std::time::SystemTime;

use mavio_api::proto::mavio::auth::v1::{
    AuthorizeQuickConnectRequest, CreateFirstUserRequest, Device, GetAuthInfoRequest,
    GetQuickConnectStateRequest, LoginRequest, LoginWithQuickConnectRequest, LogoutRequest,
    StartQuickConnectRequest,
};
use mavio_api::proto::mavio::user::v1::User;

use crate::connection::Connection;
use crate::error::{Error, Result};
use crate::session::{Credential, DeviceKey, Session, expiry};
use crate::store::{ServerRecord, Store};

/// Keeps secrets in the platform's secure storage: the Keychain, the
/// Android Keystore, the Windows Credential Manager or the Secret Service.
/// The core keeps there the credentials of each server's session, under
/// the server's ID, and the device key, under `device-key`; the app keeps
/// what it is given as it is. The app implements it.
pub trait TokenStore: Send + Sync {
    /// What is kept under an ID, if anything.
    fn token(&self, server_id: &str) -> Option<String>;
    /// Keeps a value under an ID, replacing an earlier one.
    fn set_token(&self, server_id: &str, token: &str);
    /// Forgets what is kept under an ID.
    fn delete_token(&self, server_id: &str);
}

/// The installation, as servers list it among the user's devices.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct DeviceInfo {
    /// Stable for the installation; signing in again replaces the session.
    pub id: String,
    /// Shown to the user, such as "Living room TV".
    pub name: String,
    /// The app, such as "Mavio for iOS".
    pub client: String,
    /// The app's version.
    pub client_version: String,
}

impl DeviceInfo {
    fn to_proto(&self, key: &DeviceKey) -> Device {
        Device {
            id: Some(self.id.clone()),
            name: Some(self.name.clone()),
            client: Some(self.client.clone()),
            client_version: Some(self.client_version.clone()),
            public_key: Some(key.public_key()),
            ..Default::default()
        }
    }
}

/// What a server tells before anyone signs in.
#[derive(Debug, Clone, PartialEq, Eq)]
pub struct ServerInfo {
    /// The server's own ID, the same at every address.
    pub id: String,
    /// The name the server shows.
    pub name: String,
    /// The server's version.
    pub version: String,
    /// The address it answered at, as the core writes it.
    pub address: String,
    /// Whether the server has no user yet, and the first one is created
    /// instead of signing in.
    pub setup_required: bool,
}

/// A Quick Connect request: the device shows the code, the user enters it
/// on a device signed in to the server, and this device then signs in.
#[derive(Clone)]
pub struct QuickConnect {
    /// Six digits to show.
    pub code: String,
    /// When the request expires.
    pub expires: SystemTime,
    /// The server signed in to.
    pub server: ServerInfo,
    secret: String,
    conn: Connection,
}

/// The servers the user added, in their order, and the connections of
/// those signed in.
pub struct Servers {
    store: Arc<Store>,
    tokens: Arc<dyn TokenStore>,
    device: DeviceInfo,
    key: Arc<DeviceKey>,
    connections: RwLock<HashMap<String, Connection>>,
}

impl Servers {
    /// The servers kept in store, signed in with the sessions kept in
    /// tokens, with the device key kept there too.
    pub fn new(store: Arc<Store>, tokens: Arc<dyn TokenStore>, device: DeviceInfo) -> Self {
        let key = Arc::new(DeviceKey::load(tokens.as_ref()));
        Servers {
            store,
            tokens,
            device,
            key,
            connections: RwLock::default(),
        }
    }

    /// The device signing in.
    pub fn device(&self) -> &DeviceInfo {
        &self.device
    }

    /// The servers in the user's order.
    pub fn list(&self) -> Result<Vec<ServerRecord>> {
        self.store.servers()
    }

    /// One server.
    pub fn get(&self, id: &str) -> Result<ServerRecord> {
        self.list()?
            .into_iter()
            .find(|s| s.id == id)
            .ok_or_else(|| Error::NotFound(format!("server {id}")))
    }

    /// Whether the device holds a session of the server; without one the
    /// user signs in again.
    pub fn signed_in(&self, id: &str) -> bool {
        Credential::load(self.tokens.as_ref(), id).is_some()
    }

    /// The connection of a signed-in server.
    pub fn connection(&self, id: &str) -> Result<Connection> {
        if let Some(c) = self
            .connections
            .read()
            .unwrap_or_else(|e| e.into_inner())
            .get(id)
        {
            return Ok(c.clone());
        }
        let server = self.get(id)?;
        let credential = Credential::load(self.tokens.as_ref(), id)
            .ok_or_else(|| Error::Unauthenticated(format!("signed out of {}", server.name)))?;
        let conn = Connection::new(&server.address)?;
        let conn = conn.with_session(self.session(&conn, id, credential));
        self.connections
            .write()
            .unwrap_or_else(|e| e.into_inner())
            .insert(id.to_owned(), conn.clone());
        Ok(conn)
    }

    fn session(&self, conn: &Connection, id: &str, credential: Credential) -> Session {
        let (http, config) = conn.signed_out();
        Session::new(
            id,
            credential,
            self.tokens.clone(),
            self.key.clone(),
            http,
            config,
        )
    }

    /// Asks the server at address who it is.
    pub async fn probe(&self, address: &str) -> Result<ServerInfo> {
        let conn = Connection::new(address)?;
        probe(&conn).await
    }

    /// Signs in to the server at address with a name and password, adding
    /// it after the others, or updating its address when it was added.
    pub async fn sign_in(&self, address: &str, name: &str, password: &str) -> Result<ServerRecord> {
        let conn = Connection::new(address)?;
        let info = probe(&conn).await?;
        let resp = conn
            .auth()
            .login(LoginRequest {
                name: Some(name.to_owned()),
                password: Some(password.to_owned()),
                device: self.device.to_proto(&self.key).into(),
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
        )
    }

    /// Creates the first user of a new server, an administrator, and signs
    /// in as it.
    pub async fn create_first_user(
        &self,
        address: &str,
        name: &str,
        password: &str,
    ) -> Result<ServerRecord> {
        let conn = Connection::new(address)?;
        let info = probe(&conn).await?;
        let resp = conn
            .auth()
            .create_first_user(CreateFirstUserRequest {
                name: Some(name.to_owned()),
                password: Some(password.to_owned()),
                device: self.device.to_proto(&self.key).into(),
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
        )
    }

    /// Starts signing in to the server at address with Quick Connect.
    pub async fn start_quick_connect(&self, address: &str) -> Result<QuickConnect> {
        let conn = Connection::new(address)?;
        let server = probe(&conn).await?;
        let resp = conn
            .auth()
            .start_quick_connect(StartQuickConnectRequest {
                device: self.device.to_proto(&self.key).into(),
                ..Default::default()
            })
            .await?
            .into_owned();
        let expires = resp
            .expire_time
            .into_option()
            .and_then(|t| SystemTime::try_from(t).ok())
            .unwrap_or_else(SystemTime::now);
        Ok(QuickConnect {
            code: resp.code.unwrap_or_default(),
            expires,
            server,
            secret: resp.secret.unwrap_or_default(),
            conn,
        })
    }

    /// Checks a Quick Connect request once, signing in when it was
    /// authorized; the app calls it every few seconds while it shows the
    /// code. An expired request is `NotFound`.
    pub async fn poll_quick_connect(&self, qc: &QuickConnect) -> Result<Option<ServerRecord>> {
        let state = qc
            .conn
            .auth()
            .get_quick_connect_state(GetQuickConnectStateRequest {
                secret: Some(qc.secret.clone()),
                ..Default::default()
            })
            .await?
            .into_owned();
        if !state.authorized.unwrap_or_default() {
            return Ok(None);
        }
        let resp = qc
            .conn
            .auth()
            .login_with_quick_connect(LoginWithQuickConnectRequest {
                secret: Some(qc.secret.clone()),
                ..Default::default()
            })
            .await?
            .into_owned();
        self.signed_in_as(
            &qc.conn,
            &qc.server,
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
        )
        .map(Some)
    }

    /// Signs in the device showing code, as the user signed in to the
    /// server here.
    pub async fn authorize_quick_connect(&self, server_id: &str, code: &str) -> Result<()> {
        self.connection(server_id)?
            .auth()
            .authorize_quick_connect(AuthorizeQuickConnectRequest {
                code: Some(code.to_owned()),
                ..Default::default()
            })
            .await?;
        Ok(())
    }

    /// Signs out of a server and removes it with everything kept of it.
    /// The server is told when it can be reached.
    pub async fn remove(&self, id: &str) -> Result<()> {
        if let Ok(conn) = self.connection(id) {
            // Signing out of an unreachable server leaves its session to
            // expire there.
            let _ = conn.auth().logout(LogoutRequest::default()).await;
        }
        self.forget_token(id);
        self.store.remove_server(id)
    }

    /// Forgets the session of a server that ended it, so that the user
    /// signs in again; the server stays with what is kept of it.
    pub fn forget_token(&self, id: &str) {
        self.tokens.delete_token(id);
        self.connections
            .write()
            .unwrap_or_else(|e| e.into_inner())
            .remove(id);
    }

    /// Puts the servers in the user's order.
    pub fn order(&self, ids: &[String]) -> Result<()> {
        self.store.order_servers(ids)
    }

    fn signed_in_as(
        &self,
        conn: &Connection,
        info: &ServerInfo,
        credential: Credential,
        user: Option<User>,
    ) -> Result<ServerRecord> {
        if credential.token.is_empty() || credential.session.is_empty() {
            return Err(Error::Server("no session".into()));
        }
        let user = user.unwrap_or_default();
        let record = ServerRecord {
            id: info.id.clone(),
            address: info.address.clone(),
            name: info.name.clone(),
            version: info.version.clone(),
            user_id: user.id.unwrap_or_default(),
            user_name: user.name.unwrap_or_default(),
        };
        self.store.put_server(&record)?;
        credential.save(self.tokens.as_ref(), &record.id);
        let conn = conn.with_session(self.session(conn, &record.id, credential));
        self.connections
            .write()
            .unwrap_or_else(|e| e.into_inner())
            .insert(record.id.clone(), conn);
        Ok(record)
    }
}

async fn probe(conn: &Connection) -> Result<ServerInfo> {
    let info = conn
        .auth()
        .get_auth_info(GetAuthInfoRequest::default())
        .await?
        .into_owned();
    let id = info.server_id.unwrap_or_default();
    if id.is_empty() {
        return Err(Error::Invalid(
            "the server does not tell its ID; update it".into(),
        ));
    }
    let address = conn.base().to_string();
    Ok(ServerInfo {
        id,
        name: info.server_name.unwrap_or_default(),
        version: info.version.unwrap_or_default(),
        address: address.trim_end_matches('/').to_owned(),
        setup_required: info.setup_required.unwrap_or_default(),
    })
}

#[cfg(test)]
pub(crate) mod tests {
    use std::sync::Mutex;

    use buffa_types::google::protobuf::Timestamp;
    use mavio_api::proto::mavio::auth::v1::{
        AuthorizeQuickConnectResponse, GetAuthInfoResponse, GetQuickConnectStateResponse,
        LoginResponse, LoginWithQuickConnectResponse, LogoutResponse, RefreshSessionRequest,
        RefreshSessionResponse, Session as SessionProto, StartQuickConnectResponse,
        StartRefreshRequest, StartRefreshResponse,
    };
    use mavio_api::proto::mavio::user::v1::{GetCurrentUserRequest, GetCurrentUserResponse};

    use super::*;
    use crate::fake::{Fake, error};

    /// Tokens kept in memory.
    #[derive(Default)]
    pub(crate) struct MemoryTokens(pub(crate) Mutex<HashMap<String, String>>);

    impl TokenStore for MemoryTokens {
        fn token(&self, server_id: &str) -> Option<String> {
            self.0.lock().unwrap().get(server_id).cloned()
        }
        fn set_token(&self, server_id: &str, token: &str) {
            self.0
                .lock()
                .unwrap()
                .insert(server_id.into(), token.into());
        }
        fn delete_token(&self, server_id: &str) {
            self.0.lock().unwrap().remove(server_id);
        }
    }

    /// The ID of the fake servers' sessions.
    pub(crate) const SESSION: &str = "01890a5d-ac96-774b-bcce-b302099a8057";

    /// The access token kept for a server.
    fn token_of(tokens: &MemoryTokens, id: &str) -> Option<String> {
        Credential::load(tokens, id).map(|c| c.token)
    }

    fn session() -> SessionProto {
        SessionProto {
            id: Some(SESSION.into()),
            ..Default::default()
        }
    }

    pub(crate) fn device() -> DeviceInfo {
        DeviceInfo {
            id: "dev".into(),
            name: "Test".into(),
            client: "Mavio tests".into(),
            client_version: "0".into(),
        }
    }

    pub(crate) fn servers() -> (Servers, Arc<MemoryTokens>) {
        let (s, tokens, _) = servers_with_store();
        (s, tokens)
    }

    pub(crate) fn servers_with_store() -> (Servers, Arc<MemoryTokens>, Arc<Store>) {
        let tokens = Arc::new(MemoryTokens::default());
        let store = Arc::new(Store::open_in_memory().unwrap());
        let s = Servers::new(store.clone(), tokens.clone(), device());
        (s, tokens, store)
    }

    /// A fake server named name, with ID id, that signs in "me" with
    /// "secret" as user "u" and answers with token "t-<id>".
    pub(crate) fn fake_server(id: &'static str, name: &'static str) -> Fake {
        let fake = Fake::default();
        fake.unary(
            "/mavio.auth.v1.AuthService/GetAuthInfo",
            move |_, _: GetAuthInfoRequest| {
                Ok(GetAuthInfoResponse {
                    server_id: Some(id.into()),
                    server_name: Some(name.into()),
                    version: Some("v1".into()),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.auth.v1.AuthService/Login",
            move |_, req: LoginRequest| {
                if req.name.as_deref() != Some("me") || req.password.as_deref() != Some("secret") {
                    return Err(error("unauthenticated"));
                }
                assert_eq!(req.device.id.as_deref(), Some("dev"), "the device signs in");
                assert_eq!(
                    req.device.public_key.as_ref().map(Vec::len),
                    Some(32),
                    "with its key"
                );
                Ok(LoginResponse {
                    access_token: Some(format!("t-{id}")),
                    session: session().into(),
                    user: User {
                        id: Some("u".into()),
                        name: Some("me".into()),
                        ..Default::default()
                    }
                    .into(),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.auth.v1.AuthService/Logout",
            |head, _: LogoutRequest| {
                assert!(head.headers().contains_key("authorization"));
                Ok(LogoutResponse::default())
            },
        );
        fake
    }

    #[tokio::test]
    async fn sign_in_adds_servers_in_order() {
        let (servers, tokens) = servers();
        let a = fake_server("sa", "Home").serve().await;
        let b = fake_server("sb", "Friend").serve().await;

        let wrong = servers.sign_in(&a, "me", "wrong").await;
        assert!(
            matches!(wrong, Err(Error::Unauthenticated(_))),
            "got = {wrong:?}"
        );
        assert!(servers.list().unwrap().is_empty());

        let got = servers.sign_in(&a, "me", "secret").await.unwrap();
        assert_eq!(
            (got.id.as_str(), got.name.as_str(), got.user_name.as_str()),
            ("sa", "Home", "me")
        );
        servers.sign_in(&b, "me", "secret").await.unwrap();
        let ids: Vec<_> = servers.list().unwrap().into_iter().map(|s| s.id).collect();
        assert_eq!(ids, ["sa", "sb"]);
        assert_eq!(token_of(&tokens, "sb").as_deref(), Some("t-sb"));
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

        servers.order(&["sb".into()]).unwrap();
        let ids: Vec<_> = servers.list().unwrap().into_iter().map(|s| s.id).collect();
        assert_eq!(ids, ["sb", "sa"]);

        servers.remove("sb").await.unwrap();
        assert!(token_of(&tokens, "sb").is_none());
        assert!(matches!(servers.connection("sb"), Err(Error::NotFound(_))));
    }

    #[tokio::test]
    async fn the_same_server_at_another_address_is_one() {
        let (servers, _) = servers();
        let fake = fake_server("sa", "Home");
        let first = fake.serve().await;
        let second = fake.serve().await;
        servers.sign_in(&first, "me", "secret").await.unwrap();
        servers.sign_in(&second, "me", "secret").await.unwrap();
        let list = servers.list().unwrap();
        assert_eq!(list.len(), 1);
        assert_eq!(list[0].address, second);
    }

    #[tokio::test]
    async fn forgotten_tokens_need_signing_in() {
        let (servers, _) = servers();
        let a = fake_server("sa", "Home").serve().await;
        servers.sign_in(&a, "me", "secret").await.unwrap();
        servers.forget_token("sa");
        assert!(!servers.signed_in("sa"));
        assert!(matches!(
            servers.connection("sa"),
            Err(Error::Unauthenticated(_))
        ));
        assert_eq!(servers.list().unwrap().len(), 1, "the server stays");
    }

    #[tokio::test]
    async fn unreachable_servers() {
        let (servers, _) = servers();
        let got = servers.probe("http://127.0.0.1:1").await;
        assert!(
            matches!(got, Err(ref e) if e.is_transient()),
            "got = {got:?}"
        );
    }

    #[tokio::test]
    async fn quick_connect() {
        let (servers, tokens) = servers();
        let authorized = Arc::new(Mutex::new(false));
        let fake = fake_server("sa", "Home");
        let state = authorized.clone();
        fake.unary(
            "/mavio.auth.v1.AuthService/StartQuickConnect",
            |_, _: StartQuickConnectRequest| {
                Ok(StartQuickConnectResponse {
                    secret: Some("s3cret".into()),
                    code: Some("123456".into()),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.auth.v1.AuthService/GetQuickConnectState",
            move |_, req: GetQuickConnectStateRequest| {
                assert_eq!(req.secret.as_deref(), Some("s3cret"));
                Ok(GetQuickConnectStateResponse {
                    authorized: Some(*state.lock().unwrap()),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.auth.v1.AuthService/LoginWithQuickConnect",
            |_, _: LoginWithQuickConnectRequest| {
                Ok(LoginWithQuickConnectResponse {
                    access_token: Some("t-qc".into()),
                    session: session().into(),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.auth.v1.AuthService/AuthorizeQuickConnect",
            |head, req: AuthorizeQuickConnectRequest| {
                assert_eq!(head.headers()["authorization"], "Bearer t-sa");
                assert_eq!(req.code.as_deref(), Some("123456"));
                Ok(AuthorizeQuickConnectResponse::default())
            },
        );
        let addr = fake.serve().await;

        let qc = servers.start_quick_connect(&addr).await.unwrap();
        assert_eq!(qc.code, "123456");
        assert!(servers.poll_quick_connect(&qc).await.unwrap().is_none());
        *authorized.lock().unwrap() = true;
        let got = servers.poll_quick_connect(&qc).await.unwrap().unwrap();
        assert_eq!(got.id, "sa");
        assert_eq!(token_of(&tokens, "sa").as_deref(), Some("t-qc"));

        // A signed-in device authorizes another.
        let (other, _) = self::servers();
        other.sign_in(&addr, "me", "secret").await.unwrap();
        other.authorize_quick_connect("sa", "123456").await.unwrap();
    }

    /// A fake server whose sign-in token expires at once, and which
    /// refreshes the session to "t-<n>" for the device key that signed in.
    fn refreshing_server(refreshes: Arc<Mutex<u32>>) -> Fake {
        let fake = fake_server("sa", "Home");
        let public = Arc::new(Mutex::new(Vec::new()));
        let nonce = [9u8; 32];
        let key = public.clone();
        fake.unary(
            "/mavio.auth.v1.AuthService/Login",
            move |_, req: LoginRequest| {
                *key.lock().unwrap() = req.device.public_key.clone().unwrap_or_default();
                Ok(LoginResponse {
                    access_token: Some("t-0".into()),
                    access_token_expire_time: Timestamp::from(SystemTime::now()).into(),
                    session: session().into(),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.auth.v1.AuthService/StartRefresh",
            move |head, req: StartRefreshRequest| {
                assert!(
                    !head.headers().contains_key("authorization"),
                    "refreshes are signed out"
                );
                assert_eq!(req.session_id.as_deref(), Some(SESSION));
                Ok(StartRefreshResponse {
                    nonce: Some(nonce.to_vec()),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.auth.v1.AuthService/RefreshSession",
            move |_, req: RefreshSessionRequest| {
                let mut message = b"mavio-session-refresh-v1\0".to_vec();
                message.extend_from_slice(&[
                    0x01, 0x89, 0x0a, 0x5d, 0xac, 0x96, 0x77, 0x4b, 0xbc, 0xce, 0xb3, 0x02, 0x09,
                    0x9a, 0x80, 0x57,
                ]);
                message.extend_from_slice(&nonce);
                let key = public.lock().unwrap().clone();
                let public = aws_lc_rs::signature::UnparsedPublicKey::new(
                    &aws_lc_rs::signature::ED25519,
                    key,
                );
                if public
                    .verify(&message, req.signature.as_deref().unwrap_or_default())
                    .is_err()
                {
                    return Err(error("unauthenticated"));
                }
                let mut n = refreshes.lock().unwrap();
                *n += 1;
                Ok(RefreshSessionResponse {
                    access_token: Some(format!("t-{n}")),
                    ..Default::default()
                })
            },
        );
        fake
    }

    #[tokio::test]
    async fn expiring_tokens_are_refreshed_with_the_device_key() {
        let (servers, tokens) = servers();
        let refreshes = Arc::new(Mutex::new(0));
        let fake = refreshing_server(refreshes.clone());
        let seen = Arc::new(Mutex::new(Vec::new()));
        let headers = seen.clone();
        fake.unary(
            "/mavio.user.v1.UserService/GetCurrentUser",
            move |head, _: GetCurrentUserRequest| {
                let auth = head.headers()["authorization"].to_str().unwrap().to_owned();
                headers.lock().unwrap().push(auth);
                Ok(GetCurrentUserResponse::default())
            },
        );
        let addr = fake.serve().await;
        servers.sign_in(&addr, "me", "secret").await.unwrap();
        let conn = servers.connection("sa").unwrap();
        for _ in 0..2 {
            conn.users()
                .get_current_user(GetCurrentUserRequest::default())
                .await
                .unwrap();
        }
        assert_eq!(*seen.lock().unwrap(), ["Bearer t-1", "Bearer t-1"]);
        assert_eq!(
            token_of(&tokens, "sa").as_deref(),
            Some("t-1"),
            "the new token is kept"
        );
    }

    #[tokio::test]
    async fn rejected_tokens_are_refreshed_once() {
        let (servers, tokens) = servers();
        let refreshes = Arc::new(Mutex::new(0));
        let fake = refreshing_server(refreshes.clone());
        let ended = Arc::new(Mutex::new(false));
        let session_ended = ended.clone();
        fake.unary(
            "/mavio.user.v1.UserService/GetCurrentUser",
            move |head, _: GetCurrentUserRequest| {
                let auth = head.headers()["authorization"].to_str().unwrap();
                if *session_ended.lock().unwrap() || auth != "Bearer t-2" {
                    return Err(error("unauthenticated"));
                }
                Ok(GetCurrentUserResponse::default())
            },
        );
        let addr = fake.serve().await;
        servers.sign_in(&addr, "me", "secret").await.unwrap();
        let conn = servers.connection("sa").unwrap();
        conn.users()
            .get_current_user(GetCurrentUserRequest::default())
            .await
            .unwrap();
        assert_eq!(
            *refreshes.lock().unwrap(),
            2,
            "refreshed as it expired, then as it was rejected"
        );
        assert_eq!(token_of(&tokens, "sa").as_deref(), Some("t-2"));

        // Signing in on another device binds the session to its key, so
        // this one's refreshes fail.
        let (other, _) = self::servers();
        other.sign_in(&addr, "me", "secret").await.unwrap();
        *ended.lock().unwrap() = true;
        let got = conn
            .users()
            .get_current_user(GetCurrentUserRequest::default())
            .await
            .map_err(Error::from);
        assert!(
            matches!(got, Err(Error::Unauthenticated(_))),
            "got = {got:?}"
        );
    }
}

//! The Connect clients of one server, over HTTP/1.1 and HTTP/2, with TLS
//! for `https` addresses.

use std::sync::Arc;
use std::time::Duration;

use bytes::Bytes;
use connectrpc::client::{ClientConfig, ClientTransport, HttpClient};
use connectrpc::{ConnectError, ErrorCode};
use http::header::CONTENT_TYPE;
use http::{Request, StatusCode, Uri};
use http_body_util::{BodyExt, Full};
use mavio_api::connect::mavio::auth::v1::AuthServiceClient;
use mavio_api::connect::mavio::hub::v1::HubServiceClient;
use mavio_api::connect::mavio::library::v1::{ItemServiceClient, LibraryServiceClient};
use mavio_api::connect::mavio::playback::v1::PlaybackServiceClient;
use mavio_api::connect::mavio::session::v1::EventServiceClient;
use mavio_api::connect::mavio::user::v1::{UserDataServiceClient, UserServiceClient};

use crate::error::{Error, Result};
use crate::session::{Session, Transport};

/// The port a Mavio server listens on unless configured otherwise.
pub const DEFAULT_PORT: u16 = 8686;

/// How long a unary call may take before the server counts as
/// unreachable.
const CALL_TIMEOUT: Duration = Duration::from_secs(20);

/// Parses an address the user typed: a URL, or a host with an optional
/// port, which gets `http://` and, without a port, the default port. A path
/// is kept, for servers behind a reverse proxy at a base URL.
pub fn parse_address(input: &str) -> Result<Uri> {
    let input = input.trim().trim_end_matches('/');
    if input.is_empty() {
        return Err(Error::Invalid("empty address".into()));
    }
    let with_scheme = if input.contains("://") {
        input.to_owned()
    } else {
        format!("http://{input}")
    };
    let uri: Uri = with_scheme
        .parse()
        .map_err(|e| Error::Invalid(format!("address {input:?}: {e}")))?;
    let (Some(scheme), Some(authority)) = (uri.scheme_str(), uri.authority()) else {
        return Err(Error::Invalid(format!("address {input:?} has no host")));
    };
    if scheme != "http" && scheme != "https" {
        return Err(Error::Invalid(format!(
            "address {input:?} is not http or https"
        )));
    }
    let authority = match (authority.port_u16(), input.contains("://")) {
        (None, false) => format!("{authority}:{DEFAULT_PORT}"),
        _ => authority.to_string(),
    };
    let path = uri.path().trim_end_matches('/');
    format!("{scheme}://{authority}{path}")
        .parse()
        .map_err(|e| Error::Invalid(format!("address {input:?}: {e}")))
}

/// The TLS configuration of `https` servers: the Mozilla root
/// certificates, the same on every platform.
pub(crate) fn tls_config() -> Arc<rustls::ClientConfig> {
    let roots = rustls::RootCertStore {
        roots: webpki_roots::TLS_SERVER_ROOTS.to_vec(),
    };
    let provider = Arc::new(rustls::crypto::aws_lc_rs::default_provider());
    let config = rustls::ClientConfig::builder_with_provider(provider)
        .with_safe_default_protocol_versions()
        .expect("the provider supports the default protocol versions")
        .with_root_certificates(roots)
        .with_no_client_auth();
    Arc::new(config)
}

/// The clients of one server, signed in when it has a session. Cloning is
/// cheap and shares the connections.
#[derive(Clone)]
pub struct Connection {
    base: Uri,
    http: HttpClient,
    session: Option<Arc<Session>>,
}

impl Connection {
    /// Connects to the server at address, signed out.
    pub fn new(address: &str) -> Result<Self> {
        let base = parse_address(address)?;
        let http = if base.scheme_str() == Some("https") {
            HttpClient::with_tls(tls_config())
        } else {
            HttpClient::plaintext()
        };
        Ok(Connection {
            base,
            http,
            session: None,
        })
    }

    /// The same server, calling as a session.
    pub(crate) fn with_session(&self, session: Session) -> Self {
        Connection {
            session: Some(Arc::new(session)),
            ..self.clone()
        }
    }

    /// The signed-out client and configuration of a session's refreshes.
    pub(crate) fn signed_out(&self) -> (HttpClient, ClientConfig) {
        (self.http.clone(), self.unary())
    }

    /// The server's base URL, which media and image paths are relative to.
    pub fn base(&self) -> &Uri {
        &self.base
    }

    /// The access token, refreshed first when it is about to expire; none
    /// when signed out.
    pub async fn token(&self) -> Result<Option<String>> {
        match &self.session {
            Some(s) => s.token().await.map(Some),
            None => Ok(None),
        }
    }

    /// Refreshes the session's access token now, as after a stream the
    /// server rejected; a session the server no longer refreshes is
    /// `Unauthenticated`.
    pub async fn refresh(&self) -> Result<()> {
        match &self.session {
            Some(s) => s.refresh_now().await,
            None => Err(Error::Unauthenticated("signed out".into())),
        }
    }

    fn transport(&self) -> Transport {
        Transport {
            http: self.http.clone(),
            session: self.session.clone(),
        }
    }

    /// Resolves a path the server returned, such as a playback's URL,
    /// against its base URL.
    pub fn url(&self, path: &str) -> String {
        if path.contains("://") {
            return path.to_owned();
        }
        let base = self.base.to_string();
        format!(
            "{}/{}",
            base.trim_end_matches('/'),
            path.trim_start_matches('/')
        )
    }

    /// Calls a unary procedure with a request encoded as protobuf, such as
    /// `/mavio.library.v1.ItemService/GetItem`, answering with the
    /// response encoded the same way: the calls of the screens the core
    /// does not model.
    pub async fn call(&self, procedure: &str, request: Vec<u8>) -> Result<Vec<u8>> {
        if !procedure.starts_with("/mavio.") || procedure.matches('/').count() != 2 {
            return Err(Error::Invalid(format!("procedure {procedure:?}")));
        }
        let uri: Uri = self
            .url(procedure)
            .parse()
            .map_err(|e| Error::Invalid(format!("procedure {procedure:?}: {e}")))?;
        let req = Request::post(uri)
            .header(CONTENT_TYPE, "application/proto")
            .header("connect-protocol-version", "1")
            .header("connect-timeout-ms", CALL_TIMEOUT.as_millis().to_string())
            .body(
                Full::new(Bytes::from(request))
                    .map_err(|e| match e {})
                    .boxed(),
            )
            .map_err(|e| Error::Invalid(e.to_string()))?;
        let call = async {
            let resp = self.transport().send(req).await?;
            let status = resp.status();
            let body = resp
                .into_body()
                .collect()
                .await
                .map_err(|e| Error::Unreachable(e.to_string()))?
                .to_bytes();
            if status.is_success() {
                return Ok(body.to_vec());
            }
            Err(call_error(status, &body))
        };
        tokio::time::timeout(CALL_TIMEOUT, call)
            .await
            .map_err(|_| Error::Unreachable(format!("{procedure} timed out")))?
    }

    fn config(&self, timeout: Option<Duration>) -> ClientConfig {
        let mut config = ClientConfig::new(self.base.clone());
        if let Some(timeout) = timeout {
            config = config.with_default_timeout(timeout);
        }
        config
    }

    fn unary(&self) -> ClientConfig {
        self.config(Some(CALL_TIMEOUT))
    }

    /// `AuthService`.
    pub fn auth(&self) -> AuthServiceClient<Transport> {
        AuthServiceClient::new(self.transport(), self.unary())
    }

    /// `UserService`.
    pub fn users(&self) -> UserServiceClient<Transport> {
        UserServiceClient::new(self.transport(), self.unary())
    }

    /// `UserDataService`.
    pub fn user_data(&self) -> UserDataServiceClient<Transport> {
        UserDataServiceClient::new(self.transport(), self.unary())
    }

    /// `HubService`.
    pub fn hub(&self) -> HubServiceClient<Transport> {
        HubServiceClient::new(self.transport(), self.unary())
    }

    /// `LibraryService`.
    pub fn libraries(&self) -> LibraryServiceClient<Transport> {
        LibraryServiceClient::new(self.transport(), self.unary())
    }

    /// `ItemService`.
    pub fn items(&self) -> ItemServiceClient<Transport> {
        ItemServiceClient::new(self.transport(), self.unary())
    }

    /// `PlaybackService`.
    pub fn playback(&self) -> PlaybackServiceClient<Transport> {
        PlaybackServiceClient::new(self.transport(), self.unary())
    }

    /// `EventService`, whose stream lasts as long as the device listens.
    pub fn events(&self) -> EventServiceClient<Transport> {
        EventServiceClient::new(self.transport(), self.config(None))
    }
}

/// The error of a unary call that failed: Connect's JSON error, or the
/// HTTP status's code without one.
fn call_error(status: StatusCode, body: &[u8]) -> Error {
    #[derive(serde::Deserialize)]
    struct Body {
        code: String,
        #[serde(default)]
        message: String,
    }
    let (code, message) = match serde_json::from_slice::<Body>(body) {
        Ok(b) => (b.code.parse().unwrap_or(ErrorCode::Unknown), b.message),
        Err(_) => (
            match status {
                StatusCode::UNAUTHORIZED => ErrorCode::Unauthenticated,
                StatusCode::FORBIDDEN => ErrorCode::PermissionDenied,
                StatusCode::NOT_FOUND => ErrorCode::Unimplemented,
                StatusCode::BAD_GATEWAY
                | StatusCode::SERVICE_UNAVAILABLE
                | StatusCode::GATEWAY_TIMEOUT => ErrorCode::Unavailable,
                _ => ErrorCode::Unknown,
            },
            status.to_string(),
        ),
    };
    ConnectError::new(code, message).into()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn addresses() {
        let cases = [
            ("192.168.1.5", Ok("http://192.168.1.5:8686/")),
            ("media.local:9000", Ok("http://media.local:9000/")),
            (
                "https://media.example.com",
                Ok("https://media.example.com/"),
            ),
            (
                "https://example.com/mavio/",
                Ok("https://example.com/mavio"),
            ),
            ("http://[::1]:8686", Ok("http://[::1]:8686/")),
            ("  ", Err(())),
            ("ftp://example.com", Err(())),
        ];
        for (input, want) in cases {
            let got = parse_address(input).map(|u| u.to_string()).map_err(|_| ());
            assert_eq!(
                got.as_deref().map_err(|_| ()),
                want,
                "parse_address({input:?})"
            );
        }
    }

    #[test]
    fn urls_resolve_against_the_base() {
        let c = Connection::new("https://example.com/mavio").unwrap();
        assert_eq!(
            c.url("/media/p/stream.m3u8"),
            "https://example.com/mavio/media/p/stream.m3u8"
        );
        assert_eq!(c.url("http://cdn/x"), "http://cdn/x");
        let c = Connection::new("10.0.0.2").unwrap();
        assert_eq!(c.url("images/a"), "http://10.0.0.2:8686/images/a");
    }
}

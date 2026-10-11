//! The errors of the core, as the apps show them.

use std::fmt;

use connectrpc::ErrorCode;

/// What went wrong in a call of the core.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum Error {
    /// The server could not be reached, or did not answer in time.
    Unreachable(String),
    /// The server rejected the access token or the credentials; the user
    /// signs in again.
    Unauthenticated(String),
    /// The user may not do this.
    PermissionDenied(String),
    /// The item, server or download does not exist.
    NotFound(String),
    /// The request is invalid, such as an address that is not a URL.
    Invalid(String),
    /// The server refused the request in its present state, such as
    /// creating the first user once one exists.
    FailedPrecondition(String),
    /// The server failed.
    Server(String),
    /// The offline store failed.
    Store(String),
}

/// The result of a call of the core.
pub type Result<T, E = Error> = std::result::Result<T, E>;

impl Error {
    /// Whether the call may succeed later without anything changing on the
    /// client, so that it is retried or queued for later.
    pub fn is_transient(&self) -> bool {
        matches!(self, Error::Unreachable(_))
    }
}

impl fmt::Display for Error {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let (what, msg) = match self {
            Error::Unreachable(m) => ("unreachable", m),
            Error::Unauthenticated(m) => ("unauthenticated", m),
            Error::PermissionDenied(m) => ("permission denied", m),
            Error::NotFound(m) => ("not found", m),
            Error::Invalid(m) => ("invalid", m),
            Error::FailedPrecondition(m) => ("failed precondition", m),
            Error::Server(m) => ("server error", m),
            Error::Store(m) => ("offline store", m),
        };
        if msg.is_empty() {
            f.write_str(what)
        } else {
            write!(f, "{what}: {msg}")
        }
    }
}

impl std::error::Error for Error {}

impl From<connectrpc::ConnectError> for Error {
    fn from(err: connectrpc::ConnectError) -> Self {
        let msg = err.message.clone().unwrap_or_default();
        match err.code {
            // Failures to connect are unavailable.
            ErrorCode::Unavailable | ErrorCode::DeadlineExceeded | ErrorCode::Canceled => {
                Error::Unreachable(msg)
            }
            ErrorCode::Unauthenticated => Error::Unauthenticated(msg),
            ErrorCode::PermissionDenied => Error::PermissionDenied(msg),
            ErrorCode::NotFound => Error::NotFound(msg),
            ErrorCode::InvalidArgument | ErrorCode::OutOfRange | ErrorCode::AlreadyExists => {
                Error::Invalid(msg)
            }
            ErrorCode::FailedPrecondition | ErrorCode::Aborted => Error::FailedPrecondition(msg),
            _ => Error::Server(msg),
        }
    }
}

impl From<rusqlite::Error> for Error {
    fn from(err: rusqlite::Error) -> Self {
        Error::Store(err.to_string())
    }
}

impl From<buffa::DecodeError> for Error {
    fn from(err: buffa::DecodeError) -> Self {
        Error::Store(err.to_string())
    }
}

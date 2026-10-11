//! The shared core of the Mavio clients (`docs/clients.md` §5): the servers
//! a client is signed in to, the event stream, the offline store, playback
//! reporting, downloads and the view models of the screens. It draws
//! nothing and decodes nothing; the apps reach it through `mavio-uniffi`.

mod connection;
mod error;
mod events;
#[cfg(test)]
mod fake;
mod home;
mod reporting;
mod servers;
mod store;

pub use connection::{Connection, DEFAULT_PORT, parse_address};
pub use error::{Error, Result};
pub use events::{Backoff, EventSink, StreamState, spawn as stream_events};
pub use home::{Content, Entry, Hubs, Section, ServerHubs, Trending, Version, merge as merge_home};
pub use reporting::{Playback, PlaybackOptions, REPORT_INTERVAL, Reporter, Session};
pub use servers::{DeviceInfo, QuickConnect, ServerInfo, Servers, TokenStore};
pub use store::{DownloadRecord, DownloadState, PendingProgress, ServerRecord, Store};

/// The version of the core, which the apps show and send as their client
/// version.
pub const VERSION: &str = env!("CARGO_PKG_VERSION");

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version() {
        assert_eq!(VERSION, "0.0.0", "got = {VERSION}, want = 0.0.0");
    }
}

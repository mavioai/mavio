//! The shared core of the Mavio clients (`docs/clients.md` §5): the servers
//! a client is signed in to, the event stream, the offline store, playback
//! reporting, downloads and the view models of the screens. It draws
//! nothing and decodes nothing; the apps reach it through `mavio-uniffi`.

mod account;
mod client;
mod connection;
mod details;
mod discovery;
mod downloads;
mod error;
mod events;
#[cfg(test)]
mod fake;
mod home;
mod reporting;
mod servers;
mod session;
mod settings;
mod store;
mod view;

pub use account::{Account, BrowserSignIn, LinkCode, MAVIO_DEV_API, SignInState};
pub use client::{
    AudioStream, Core, CreditEntry, Details, DownloadEntry, Filter, HomeObserver, Libraries,
    LibraryEntry, LibraryType, Listener, Page, PersonDetails, SearchResults, Sort, Source,
};
pub use connection::{Connection, DEFAULT_PORT, is_local_address, parse_address};
pub use details::{
    CollectionLink, CreditJob, Episode, ExternalLink, Extra, ExtraKind, SeriesLink, SeriesStatus,
    SubtitleStream, VideoRange, VideoStream,
};
pub use discovery::{DISCOVERY_PORT, FoundServer, discover};
pub use downloads::{Downloads, Transfer};
pub use error::{Error, Result};
pub use events::{Backoff, EventSink, StreamState, spawn as stream_events};
pub use home::{Content, Entry, Hubs, Section, ServerHubs, Trending, Version, merge as merge_home};
pub use reporting::{Playback, PlaybackOptions, REPORT_INTERVAL, Reporter, Session};
pub use servers::{AccountMode, DeviceInfo, QuickConnect, ServerInfo, Servers, TokenStore};
pub use session::Transport;
pub use settings::{
    AccountSecurity, DeviceSession, Passkey, PlaybackPreferences, RemoteCommand, RemotePlayback,
    SubtitleMode, SwitchableUser, TotpSetup,
};
pub use store::{DownloadRecord, DownloadState, PendingProgress, ServerRecord, Store};
pub use view::{Artwork, Card, CardKind, Row, RowContent, RowKind, TitleCard};

/// The version of the core, which the apps show: a release's, else 0.0.0
/// and the commit it was built at, else dev (build.rs).
pub const VERSION: &str = env!("MAVIO_CORE_VERSION");

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn version() {
        let built = option_env!("MAVIO_VERSION")
            .filter(|v| !v.is_empty())
            .map(|v| v.trim_start_matches('v'));
        let ok = match built {
            Some(release) => VERSION == release,
            None => {
                VERSION == "dev"
                    || VERSION
                        .strip_prefix("0.0.0-")
                        .is_some_and(|r| r.len() == 12)
            }
        };
        assert!(
            ok,
            "got = {VERSION}, want = a release's, 0.0.0-<commit> or dev"
        );
    }
}

//! The Swift and Kotlin API of the shared core, exported with UniFFI
//! (`docs/clients.md` §5.3). The `uniffi-bindgen` crate generates their
//! sources from the built library.
//!
//! The core runs on one Tokio runtime of the process; the async functions
//! run their work there and cancel it when the app drops their future.

mod caps;
mod types;

use std::fmt;
use std::future::Future;
use std::sync::{Arc, OnceLock};
use std::time::{Duration, SystemTime};

use mavio_api::proto::mavio::playback::v1::{PlayMethod, SubtitleMethod};
use mavio_api::proto::mavio::session::v1::__buffa::oneof::event::Kind;
use mavio_api::proto::mavio::session::v1::Event;
use mavio_core::{
    Core, Details, DeviceInfo, DownloadEntry, Filter, FoundServer, Libraries, Page,
    PlaybackOptions, Row, SearchResults, ServerInfo, ServerRecord, Sort, Store, StreamState,
    Transfer,
};
use tokio::runtime::Runtime;

pub use caps::*;

uniffi::setup_scaffolding!();

/// The version of the core.
#[uniffi::export]
pub fn core_version() -> String {
    mavio_core::VERSION.to_owned()
}

/// The runtime of the core's tasks.
fn runtime() -> &'static Runtime {
    static RUNTIME: OnceLock<Runtime> = OnceLock::new();
    RUNTIME.get_or_init(|| {
        tokio::runtime::Builder::new_multi_thread()
            .worker_threads(2)
            .thread_name("mavio-core")
            .enable_all()
            .build()
            .expect("the core's runtime starts")
    })
}

/// Aborts a task when the future awaiting it is dropped.
struct AbortOnDrop(tokio::task::AbortHandle);

impl Drop for AbortOnDrop {
    fn drop(&mut self) {
        self.0.abort();
    }
}

/// Runs work on the core's runtime, whichever executor awaits it.
async fn run<T: Send + 'static>(work: impl Future<Output = T> + Send + 'static) -> T {
    let task = runtime().spawn(work);
    let _abort = AbortOnDrop(task.abort_handle());
    match task.await {
        Ok(v) => v,
        Err(e) if e.is_panic() => std::panic::resume_unwind(e.into_panic()),
        Err(_) => unreachable!("the task is aborted only with its future"),
    }
}

/// Why a call failed.
#[derive(Debug, uniffi::Error)]
pub enum CoreError {
    /// The server did not answer; the call may be retried.
    Unreachable {
        /// What happened.
        reason: String,
    },
    /// The server does not know the user or token; they sign in again.
    Unauthenticated {
        /// What happened.
        reason: String,
    },
    /// The user may not do this.
    PermissionDenied {
        /// What happened.
        reason: String,
    },
    /// What was asked for does not exist.
    NotFound {
        /// What happened.
        reason: String,
    },
    /// The request was wrong, such as a malformed address.
    Invalid {
        /// What happened.
        reason: String,
    },
    /// The server is not in a state to do this.
    FailedPrecondition {
        /// What happened.
        reason: String,
    },
    /// The server failed.
    Server {
        /// What happened.
        reason: String,
    },
    /// The offline store failed.
    Store {
        /// What happened.
        reason: String,
    },
}

impl fmt::Display for CoreError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let (CoreError::Unreachable { reason }
        | CoreError::Unauthenticated { reason }
        | CoreError::PermissionDenied { reason }
        | CoreError::NotFound { reason }
        | CoreError::Invalid { reason }
        | CoreError::FailedPrecondition { reason }
        | CoreError::Server { reason }
        | CoreError::Store { reason }) = self;
        f.write_str(reason)
    }
}

impl std::error::Error for CoreError {}

impl From<mavio_core::Error> for CoreError {
    fn from(e: mavio_core::Error) -> Self {
        use mavio_core::Error as E;
        match e {
            E::Unreachable(reason) => CoreError::Unreachable { reason },
            E::Unauthenticated(reason) => CoreError::Unauthenticated { reason },
            E::PermissionDenied(reason) => CoreError::PermissionDenied { reason },
            E::NotFound(reason) => CoreError::NotFound { reason },
            E::Invalid(reason) => CoreError::Invalid { reason },
            E::FailedPrecondition(reason) => CoreError::FailedPrecondition { reason },
            E::Server(reason) => CoreError::Server { reason },
            E::Store(reason) => CoreError::Store { reason },
        }
    }
}

type Result<T, E = CoreError> = std::result::Result<T, E>;

/// Keeps the core's secrets in the platform's secure storage: the Keychain
/// or the Android Keystore. They are the credentials of each server's
/// session, under the server's ID, and the device key, under
/// `device-key`, kept as they are. The app implements it.
#[uniffi::export(with_foreign)]
pub trait SecureTokens: Send + Sync {
    /// What is kept under an ID, if anything.
    fn token(&self, server_id: String) -> Option<String>;
    /// Keeps a value under an ID, replacing an earlier one.
    fn set_token(&self, server_id: String, token: String);
    /// Forgets what is kept under an ID.
    fn delete_token(&self, server_id: String);
}

struct Tokens(Arc<dyn SecureTokens>);

impl mavio_core::TokenStore for Tokens {
    fn token(&self, server_id: &str) -> Option<String> {
        self.0.token(server_id.to_owned())
    }
    fn set_token(&self, server_id: &str, token: &str) {
        self.0.set_token(server_id.to_owned(), token.to_owned());
    }
    fn delete_token(&self, server_id: &str) {
        self.0.delete_token(server_id.to_owned());
    }
}

/// What a server pushed, after which the app reloads what it shows of it.
#[derive(Debug, Clone, uniffi::Enum)]
pub enum ServerEvent {
    /// The stream came up; the app reloads what it shows of the server.
    Connected,
    /// Items of libraries changed.
    LibraryChanged {
        /// The libraries changed.
        library_ids: Vec<String>,
        /// The items added or changed.
        changed_item_ids: Vec<String>,
        /// The items removed.
        removed_item_ids: Vec<String>,
    },
    /// The user's data of items changed, from this device or another.
    UserDataChanged {
        /// The items.
        item_ids: Vec<String>,
    },
    /// The sessions of the user's devices changed.
    SessionsChanged,
}

impl ServerEvent {
    fn from_event(event: &Event) -> Option<Self> {
        Some(match event.kind.as_ref()? {
            Kind::Connected(_) => ServerEvent::Connected,
            Kind::LibraryChanged(c) => ServerEvent::LibraryChanged {
                library_ids: c.library_ids.clone(),
                changed_item_ids: c.changed_item_ids.clone(),
                removed_item_ids: c.removed_item_ids.clone(),
            },
            Kind::UserDataChanged(c) => ServerEvent::UserDataChanged {
                item_ids: c
                    .user_data
                    .iter()
                    .filter_map(|u| u.item_id.clone())
                    .collect(),
            },
            Kind::SessionsChanged(_) => ServerEvent::SessionsChanged,
            _ => return None,
        })
    }
}

/// Receives what the servers push. The app implements it.
#[uniffi::export(with_foreign)]
pub trait CoreListener: Send + Sync {
    /// A server's stream came up or went down, or the server signed the
    /// device out.
    fn server_state(&self, server_id: String, state: StreamState);
    /// A server sent an event.
    fn event(&self, server_id: String, event: ServerEvent);
}

struct Listener(Arc<dyn CoreListener>);

impl mavio_core::Listener for Listener {
    fn server_state(&self, server_id: &str, state: StreamState) {
        self.0.server_state(server_id.to_owned(), state);
    }
    fn event(&self, server_id: &str, event: &Event) {
        if let Some(event) = ServerEvent::from_event(event) {
            self.0.event(server_id.to_owned(), event);
        }
    }
}

/// Receives Home as it fills in. The app implements it.
#[uniffi::export(with_foreign)]
pub trait HomeListener: Send + Sync {
    /// The rows of Home, all of them each time.
    fn rows(&self, rows: Vec<Row>);
}

struct Home(Arc<dyn HomeListener>);

impl mavio_core::HomeObserver for Home {
    fn rows(&self, rows: Vec<Row>) {
        self.0.rows(rows);
    }
}

/// A Quick Connect request: the device shows the code, the user enters it
/// on a device signed in to the server, and this device then signs in.
#[derive(uniffi::Object)]
pub struct QuickConnectRequest(mavio_core::QuickConnect);

#[uniffi::export]
impl QuickConnectRequest {
    /// Six digits to show.
    pub fn code(&self) -> String {
        self.0.code.clone()
    }

    /// When the request expires.
    pub fn expires(&self) -> SystemTime {
        self.0.expires
    }

    /// The server signed in to.
    pub fn server(&self) -> ServerInfo {
        self.0.server.clone()
    }
}

/// How the server plays an item.
#[derive(Debug, Clone, Copy, uniffi::Enum)]
pub enum PlayKind {
    /// The file as it is.
    DirectPlay,
    /// The streams copied into a container the client plays.
    DirectStream,
    /// Re-encoded.
    Transcode,
}

/// A subtitle stream of a playback.
#[derive(Debug, Clone, uniffi::Record)]
pub struct SubtitleTrack {
    /// The stream's index in the media source, to select it.
    pub stream_index: i32,
    /// Its language.
    pub language: Option<String>,
    /// Its title.
    pub title: Option<String>,
    /// Whether it is the default.
    pub default: bool,
    /// Whether it is forced.
    pub forced: bool,
    /// The delivered format, such as "vtt".
    pub format: String,
    /// The file of a subtitle delivered on its own; unset when it is in
    /// the media.
    pub url: Option<String>,
}

/// A playback the server started.
#[derive(Debug, Clone, uniffi::Record)]
pub struct PlaybackInfo {
    /// The media to play: the file, an HLS playlist or a progressive
    /// stream.
    pub url: String,
    /// How the server plays it.
    pub kind: PlayKind,
    /// The media source played.
    pub media_source_id: String,
    /// Where the media begins; the player starts there.
    pub start_position: Option<Duration>,
    /// The audio stream selected.
    pub audio_stream_index: Option<i32>,
    /// The subtitle stream selected, -1 for none.
    pub subtitle_stream_index: Option<i32>,
    /// The subtitle streams.
    pub subtitles: Vec<SubtitleTrack>,
}

/// A playing item, which reports its progress.
#[derive(uniffi::Object)]
pub struct PlaybackSession {
    session: Arc<mavio_core::Session>,
    info: PlaybackInfo,
}

#[uniffi::export]
impl PlaybackSession {
    /// What to play.
    pub fn info(&self) -> PlaybackInfo {
        self.info.clone()
    }

    /// The player's position and whether it is paused; called on every
    /// tick, it reports every ten seconds and on pause and seek.
    pub async fn update(&self, position: Duration, paused: bool) -> Result<()> {
        let session = self.session.clone();
        Ok(run(async move { session.update(position, paused).await }).await?)
    }

    /// Playback stopped at position, unset when played to the end.
    pub async fn stop(&self, position: Option<Duration>) -> Result<()> {
        let session = self.session.clone();
        Ok(run(async move { session.stop(position).await }).await?)
    }
}

fn playback_info(conn_url: impl Fn(&str) -> String, p: &mavio_core::Playback) -> PlaybackInfo {
    let r = &p.response;
    let text = |s: &Option<String>| s.clone().filter(|s| !s.is_empty());
    PlaybackInfo {
        url: p.url.clone(),
        kind: match r.method.as_ref().and_then(|m| m.as_known()) {
            Some(PlayMethod::PLAY_METHOD_TRANSCODE) => PlayKind::Transcode,
            Some(PlayMethod::PLAY_METHOD_DIRECT_STREAM) => PlayKind::DirectStream,
            _ => PlayKind::DirectPlay,
        },
        media_source_id: r.media_source_id.clone().unwrap_or_default(),
        start_position: r
            .start_position
            .as_option()
            .and_then(|d| Duration::try_from(d.clone()).ok()),
        audio_stream_index: r.audio_stream_index,
        subtitle_stream_index: r.subtitle_stream_index,
        subtitles: r
            .subtitles
            .iter()
            .map(|s| SubtitleTrack {
                stream_index: s.stream_index.unwrap_or_default(),
                language: text(&s.language),
                title: text(&s.title),
                default: s.default.unwrap_or_default(),
                forced: s.forced.unwrap_or_default(),
                format: s.format.clone().unwrap_or_default(),
                url: (s.method.as_ref().and_then(|m| m.as_known())
                    == Some(SubtitleMethod::SUBTITLE_METHOD_EXTERNAL))
                .then(|| text(&s.url).map(|u| conn_url(&u)))
                .flatten(),
            })
            .collect(),
    }
}

/// The core of an app: its servers, their event streams, the data of its
/// screens, playback and downloads.
#[derive(uniffi::Object)]
pub struct Client(Arc<Core>);

impl Client {
    /// Runs a call of the core on its runtime.
    async fn call<T, F>(&self, f: impl FnOnce(Arc<Core>) -> F) -> Result<T>
    where
        T: Send + 'static,
        F: Future<Output = mavio_core::Result<T>> + Send + 'static,
    {
        Ok(run(f(self.0.clone())).await?)
    }
}

#[uniffi::export]
impl Client {
    /// The core with its offline store at database_path, signed in with
    /// the tokens of tokens, as device; what the servers push goes to
    /// listener.
    #[uniffi::constructor]
    pub fn new(
        database_path: String,
        device: DeviceInfo,
        tokens: Arc<dyn SecureTokens>,
        listener: Arc<dyn CoreListener>,
    ) -> Result<Arc<Self>> {
        let store = Arc::new(Store::open(database_path)?);
        let core = Core::new(
            store,
            Arc::new(Tokens(tokens)),
            device,
            Arc::new(Listener(listener)),
        );
        Ok(Arc::new(Client(Arc::new(core))))
    }

    /// Follows the event streams of every signed-in server; the app calls
    /// it when it comes to the foreground.
    pub fn start(&self) -> Result<()> {
        let _rt = runtime().enter();
        Ok(self.0.start()?)
    }

    /// Stops the event streams; the app calls it when it goes to the
    /// background.
    pub fn stop(&self) {
        self.0.stop();
    }

    /// The servers, in the user's order.
    pub fn servers(&self) -> Result<Vec<ServerRecord>> {
        Ok(self.0.servers().list()?)
    }

    /// Whether the device holds a token of a server; without one the user
    /// signs in again.
    pub fn signed_in(&self, server_id: String) -> bool {
        self.0.servers().signed_in(&server_id)
    }

    /// The state of a server's stream, unknown before it first connects.
    pub fn server_state(&self, server_id: String) -> Option<StreamState> {
        self.0.server_state(&server_id)
    }

    /// Orders the servers as the user arranged them.
    pub fn order_servers(&self, server_ids: Vec<String>) -> Result<()> {
        Ok(self.0.servers().order(&server_ids)?)
    }

    /// Finds the servers on the local network that answer within wait.
    pub async fn discover(&self, wait: Duration) -> Result<Vec<FoundServer>> {
        Ok(run(mavio_core::discover(wait)).await?)
    }

    /// Asks the server at an address who it is.
    pub async fn probe(&self, address: String) -> Result<ServerInfo> {
        self.call(|c| async move { c.servers().probe(&address).await })
            .await
    }

    /// Signs in to a server with a name and password, adding it.
    pub async fn sign_in(
        &self,
        address: String,
        name: String,
        password: String,
    ) -> Result<ServerRecord> {
        self.call(|c| async move { c.sign_in(&address, &name, &password).await })
            .await
    }

    /// Creates the first user of a new server and signs in as them.
    pub async fn create_first_user(
        &self,
        address: String,
        name: String,
        password: String,
    ) -> Result<ServerRecord> {
        self.call(|c| async move { c.create_first_user(&address, &name, &password).await })
            .await
    }

    /// Starts signing in to a server with Quick Connect.
    pub async fn start_quick_connect(&self, address: String) -> Result<Arc<QuickConnectRequest>> {
        let qc = self
            .call(|c| async move { c.servers().start_quick_connect(&address).await })
            .await?;
        Ok(Arc::new(QuickConnectRequest(qc)))
    }

    /// Asks whether a Quick Connect request was authorized, signing in when
    /// it was.
    pub async fn poll_quick_connect(
        &self,
        request: Arc<QuickConnectRequest>,
    ) -> Result<Option<ServerRecord>> {
        self.call(|c| async move { c.poll_quick_connect(&request.0).await })
            .await
    }

    /// Authorizes another device's Quick Connect code on a server this
    /// device is signed in to.
    pub async fn authorize_quick_connect(&self, server_id: String, code: String) -> Result<()> {
        self.call(|c| async move { c.servers().authorize_quick_connect(&server_id, &code).await })
            .await
    }

    /// Signs out of a server and forgets it.
    pub async fn remove_server(&self, server_id: String) -> Result<()> {
        self.call(|c| async move { c.remove(&server_id).await })
            .await
    }

    /// Loads Home from every server at once, giving listener the rows
    /// each time a server answers.
    pub async fn load_home(&self, listener: Arc<dyn HomeListener>) -> Result<()> {
        self.call(|c| async move { c.home(&Home(listener)).await })
            .await
    }

    /// More items of a row, from offset.
    pub async fn hub(
        &self,
        server_id: String,
        hub_id: String,
        offset: u32,
        limit: u32,
    ) -> Result<Page> {
        self.call(|c| async move { c.hub(&server_id, &hub_id, offset, limit).await })
            .await
    }

    /// A library's Recommended tab.
    pub async fn recommended(&self, server_id: String, library_id: String) -> Result<Vec<Row>> {
        self.call(|c| async move { c.recommended(&server_id, &library_id).await })
            .await
    }

    /// Searches every server at once, the results in the user's order of
    /// servers.
    pub async fn search(&self, query: String) -> Result<Vec<SearchResults>> {
        self.call(|c| async move { c.search(&query).await }).await
    }

    /// A server's libraries.
    pub async fn libraries(&self, server_id: String) -> Result<Libraries> {
        self.call(|c| async move { c.libraries(&server_id).await })
            .await
    }

    /// A page of a library's grid.
    pub async fn library_items(
        &self,
        server_id: String,
        library_id: String,
        sort: Sort,
        filter: Filter,
        offset: u32,
        limit: u32,
    ) -> Result<Page> {
        self.call(|c| async move {
            c.library_items(&server_id, &library_id, sort, &filter, offset, limit)
                .await
        })
        .await
    }

    /// An item's details.
    pub async fn details(&self, server_id: String, item_id: String) -> Result<Details> {
        self.call(|c| async move { c.details(&server_id, &item_id).await })
            .await
    }

    /// Marks an item played or unplayed.
    pub async fn set_played(&self, server_id: String, item_id: String, played: bool) -> Result<()> {
        self.call(|c| async move { c.set_played(&server_id, &item_id, played).await })
            .await
    }

    /// Marks an item a favorite or not.
    pub async fn set_favorite(
        &self,
        server_id: String,
        item_id: String,
        favorite: bool,
    ) -> Result<()> {
        self.call(|c| async move { c.set_favorite(&server_id, &item_id, favorite).await })
            .await
    }

    /// Starts playing an item with what the client plays.
    pub async fn play(
        &self,
        server_id: String,
        item_id: String,
        capabilities: Capabilities,
        options: PlaybackOptions,
    ) -> Result<Arc<PlaybackSession>> {
        let session = self
            .call(|c| async move {
                c.reporter()
                    .start(&server_id, &item_id, capabilities.into(), options)
                    .await
            })
            .await?;
        let conn = self.0.servers().connection(&session.playback().server_id)?;
        let info = playback_info(|u| conn.url(u), session.playback());
        Ok(Arc::new(PlaybackSession {
            session: Arc::new(session),
            info,
        }))
    }

    /// Records progress made without the server, such as of a downloaded
    /// item played offline, sent when the server answers; position is
    /// unset when it was played to the end.
    pub async fn played_offline(
        &self,
        server_id: String,
        item_id: String,
        position: Option<Duration>,
    ) -> Result<()> {
        self.call(|c| async move {
            c.reporter()
                .played_offline(&server_id, &item_id, position)
                .await
        })
        .await
    }

    /// The downloads, in the order they were queued.
    pub fn downloads(&self) -> Result<Vec<DownloadEntry>> {
        Ok(self.0.download_entries()?)
    }

    /// Queues an item for download and returns the transfer to start.
    pub async fn add_download(
        &self,
        server_id: String,
        item_id: String,
        capabilities: Capabilities,
    ) -> Result<Transfer> {
        self.call(|c| async move {
            c.downloads()
                .add(&server_id, &item_id, capabilities.into())
                .await
        })
        .await
    }

    /// The transfer of a paused or failed download.
    pub async fn resume_download(
        &self,
        server_id: String,
        item_id: String,
        capabilities: Capabilities,
    ) -> Result<Transfer> {
        self.call(|c| async move {
            c.downloads()
                .resume(&server_id, &item_id, capabilities.into())
                .await
        })
        .await
    }

    /// A transfer started or moved on: the file, relative to the app's
    /// downloads folder, and the bytes so far.
    pub fn download_progress(
        &self,
        server_id: String,
        item_id: String,
        file: String,
        received: u64,
        total: u64,
    ) -> Result<()> {
        self.0
            .downloads()
            .progress(&server_id, &item_id, &file, received, total)?;
        Ok(())
    }

    /// A transfer completed.
    pub fn download_finished(&self, server_id: String, item_id: String) -> Result<()> {
        self.0.downloads().finished(&server_id, &item_id)?;
        Ok(())
    }

    /// A transfer failed.
    pub fn download_failed(&self, server_id: String, item_id: String, error: String) -> Result<()> {
        self.0.downloads().failed(&server_id, &item_id, &error)?;
        Ok(())
    }

    /// The user paused a transfer.
    pub fn download_paused(&self, server_id: String, item_id: String) -> Result<()> {
        self.0.downloads().paused(&server_id, &item_id)?;
        Ok(())
    }

    /// Forgets a download, returning its file for the app to delete.
    pub fn remove_download(&self, server_id: String, item_id: String) -> Result<String> {
        Ok(self.0.downloads().remove(&server_id, &item_id)?)
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Mutex;

    use super::*;

    #[derive(Default)]
    struct Memory(Mutex<Vec<(String, String)>>);

    impl SecureTokens for Memory {
        fn token(&self, server_id: String) -> Option<String> {
            let tokens = self.0.lock().unwrap();
            tokens
                .iter()
                .find(|(s, _)| *s == server_id)
                .map(|(_, t)| t.clone())
        }
        fn set_token(&self, server_id: String, token: String) {
            self.0.lock().unwrap().push((server_id, token));
        }
        fn delete_token(&self, server_id: String) {
            self.0.lock().unwrap().retain(|(s, _)| *s != server_id);
        }
    }

    struct Quiet;

    impl CoreListener for Quiet {
        fn server_state(&self, _: String, _: StreamState) {}
        fn event(&self, _: String, _: ServerEvent) {}
    }

    #[test]
    fn calls_run_on_the_cores_runtime_from_any_executor() {
        let dir = std::env::temp_dir().join(format!("mavio-uniffi-{}", std::process::id()));
        std::fs::create_dir_all(&dir).unwrap();
        let core = Client::new(
            dir.join("core.db").to_string_lossy().into_owned(),
            DeviceInfo {
                id: "dev".into(),
                name: "Test".into(),
                client: "test".into(),
                client_version: "0".into(),
            },
            Arc::new(Memory::default()),
            Arc::new(Quiet),
        )
        .unwrap();
        core.start().unwrap();
        assert!(core.servers().unwrap().is_empty());
        // A plain executor without a Tokio runtime, as the apps' are.
        let found = block_on(core.search("x".into())).unwrap();
        assert!(found.is_empty());
        let wrong = block_on(core.probe("ftp://example.com".into()));
        assert!(
            matches!(wrong, Err(CoreError::Invalid { .. })),
            "got = {wrong:?}"
        );
        // Windows keeps the directory while the database is open.
        core.stop();
        drop(core);
        std::fs::remove_dir_all(dir).unwrap();
    }

    /// Polls a future on this thread until it is ready.
    fn block_on<T>(f: impl Future<Output = T>) -> T {
        use std::task::{Context, Poll, Wake, Waker};

        struct Thread(std::thread::Thread);
        impl Wake for Thread {
            fn wake(self: Arc<Self>) {
                self.0.unpark();
            }
        }
        let waker = Waker::from(Arc::new(Thread(std::thread::current())));
        let mut cx = Context::from_waker(&waker);
        let mut f = std::pin::pin!(f);
        loop {
            if let Poll::Ready(v) = f.as_mut().poll(&mut cx) {
                return v;
            }
            std::thread::park();
        }
    }
}

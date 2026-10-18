//! The core as the apps use it (`docs/clients.md` §5): the servers and
//! their event streams, and the data of each screen, from the servers when
//! they answer and from the offline store when they do not.

use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::{Duration, SystemTime};

use mavio_api::proto::mavio::hub::v1::{GetHubRequest, ListHubsRequest, ListHubsResponse};
use mavio_api::proto::mavio::library::v1::{
    GetItemRequest, GetItemResponse, GetPersonRequest, ImageKind, Item, ItemKind, LibraryKind,
    ListItemsRequest, ListLibrariesRequest, ListLibrariesResponse, ListSimilarItemsRequest,
    SortField, SortSpec, StreamKind,
};
use mavio_api::proto::mavio::session::v1::__buffa::oneof::event::Kind;
use mavio_api::proto::mavio::session::v1::Event;
use mavio_api::proto::mavio::user::v1::{
    GetCurrentUserRequest, UpdatePlaybackPreferencesRequest, UpdateUserDataRequest,
};
use tokio::task::{JoinHandle, JoinSet};

use crate::account::{Account, BrowserSignIn, LinkCode, SignInState};
use crate::connection::Connection;
use crate::details::{
    self, CollectionLink, CreditJob, Episode, ExternalLink, Extra, SeriesLink, SeriesStatus,
    SubtitleStream, VideoStream,
};
use crate::downloads::Downloads;
use crate::error::{Error, Result};
use crate::events::{self, EventSink, StreamState};
use crate::home::{self, Hubs, ServerHubs, Version};
use crate::reporting::Reporter;
use crate::servers::{DeviceInfo, QuickConnect, Servers, TokenStore};
use crate::settings::{PlaybackPreferences, SwitchableUser};
use crate::store::{DownloadState, ServerRecord, Store};
use crate::view::{self, Artwork, Card, Row};

/// The items of a search asked of each server.
const SEARCH_LIMIT: i32 = 30;
/// The kinds search finds: titles, not the seasons and folders inside them.
const SEARCH_KINDS: [ItemKind; 13] = [
    ItemKind::ITEM_KIND_MOVIE,
    ItemKind::ITEM_KIND_SERIES,
    ItemKind::ITEM_KIND_EPISODE,
    ItemKind::ITEM_KIND_VIDEO,
    ItemKind::ITEM_KIND_MUSIC_ARTIST,
    ItemKind::ITEM_KIND_MUSIC_ALBUM,
    ItemKind::ITEM_KIND_TRACK,
    ItemKind::ITEM_KIND_MUSIC_VIDEO,
    ItemKind::ITEM_KIND_AUDIOBOOK,
    ItemKind::ITEM_KIND_BOOK,
    ItemKind::ITEM_KIND_PHOTO_ALBUM,
    ItemKind::ITEM_KIND_COLLECTION,
    ItemKind::ITEM_KIND_PLAYLIST,
];

/// Receives what the servers push.
pub trait Listener: Send + Sync {
    /// A server's stream came up or went down, or the server signed the
    /// device out.
    fn server_state(&self, server_id: &str, state: StreamState);
    /// A server sent an event, such as a change of the user's data, after
    /// which the app reloads what it shows of it.
    fn event(&self, server_id: &str, event: &Event);
    /// The servers changed as the Mavio account's servers were followed,
    /// after which the app reloads them.
    fn servers_changed(&self) {}
}

/// Receives Home as it fills in.
pub trait HomeObserver: Send + Sync {
    /// The rows of Home, all of them each time.
    fn rows(&self, rows: Vec<Row>);
}

/// A page of items.
#[derive(Debug, Clone, PartialEq)]
pub struct Page {
    /// The items.
    pub cards: Vec<Card>,
    /// The number of items across the pages, when the server counts them.
    pub total: Option<u32>,
    /// Whether there are items after these.
    pub more: bool,
}

/// What a library holds.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum LibraryType {
    /// Movies.
    Movies,
    /// Series.
    Shows,
    /// Music.
    Music,
    /// Music videos.
    MusicVideos,
    /// Home videos.
    HomeVideos,
    /// Books.
    Books,
    /// Photos.
    Photos,
    /// Collections.
    Collections,
    /// Playlists.
    Playlists,
    /// Anything else.
    Mixed,
}

impl From<Option<LibraryKind>> for LibraryType {
    fn from(k: Option<LibraryKind>) -> Self {
        match k {
            Some(LibraryKind::LIBRARY_KIND_MOVIES) => LibraryType::Movies,
            Some(LibraryKind::LIBRARY_KIND_SHOWS) => LibraryType::Shows,
            Some(LibraryKind::LIBRARY_KIND_MUSIC) => LibraryType::Music,
            Some(LibraryKind::LIBRARY_KIND_MUSIC_VIDEOS) => LibraryType::MusicVideos,
            Some(LibraryKind::LIBRARY_KIND_HOME_VIDEOS) => LibraryType::HomeVideos,
            Some(LibraryKind::LIBRARY_KIND_BOOKS) => LibraryType::Books,
            Some(LibraryKind::LIBRARY_KIND_PHOTOS) => LibraryType::Photos,
            Some(LibraryKind::LIBRARY_KIND_COLLECTIONS) => LibraryType::Collections,
            Some(LibraryKind::LIBRARY_KIND_PLAYLISTS) => LibraryType::Playlists,
            _ => LibraryType::Mixed,
        }
    }
}

/// A library of a server.
#[derive(Debug, Clone, PartialEq)]
pub struct LibraryEntry {
    /// The server.
    pub server_id: String,
    /// The library.
    pub id: String,
    /// Its name.
    pub name: String,
    /// What it holds.
    pub kind: LibraryType,
    /// Its collage.
    pub image_url: String,
}

/// The libraries of a server.
#[derive(Debug, Clone, PartialEq)]
pub struct Libraries {
    /// The libraries.
    pub libraries: Vec<LibraryEntry>,
    /// Whether they come from the offline store.
    pub offline: bool,
}

/// How a library's grid is sorted.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum Sort {
    /// By name.
    Name,
    /// Last added first.
    DateAdded,
    /// Last released first.
    Released,
    /// Best rated first.
    Rating,
    /// Last played first.
    LastPlayed,
    /// At random.
    Random,
}

/// A library's grid filter.
#[derive(Debug, Clone, Default, PartialEq, Eq)]
pub struct Filter {
    /// Only played (true) or unplayed (false) items.
    pub played: Option<bool>,
    /// Only favorites.
    pub favorite: bool,
    /// Only items of this genre.
    pub genre: Option<String>,
    /// Only items with this keyword.
    pub tag: Option<String>,
    /// Only items of this studio or network.
    pub studio: Option<String>,
}

/// A person credited on an item.
#[derive(Debug, Clone, PartialEq)]
pub struct CreditEntry {
    /// The person.
    pub person_id: String,
    /// Their name.
    pub name: String,
    /// The character, or their job.
    pub role: Option<String>,
    /// Whether they act in it; else crew.
    pub cast: bool,
    /// What they did.
    pub job: CreditJob,
    /// Their photo.
    pub photo: Option<Artwork>,
}

/// One version of a title: on this server, a file such as "4K HDR"; on
/// another server, its copy.
#[derive(Debug, Clone, PartialEq)]
pub struct Source {
    /// The server of the version.
    pub server_id: String,
    /// That server's name.
    pub server_name: String,
    /// The item on that server.
    pub item_id: String,
    /// The media source on that server, for a file of this server.
    pub media_source_id: Option<String>,
    /// What tells it apart, such as "4K HDR".
    pub name: Option<String>,
    /// The file's size.
    pub size: Option<u64>,
    /// The file's container, such as "mkv", for a file of this server.
    pub container: Option<String>,
    /// The file's audio streams, for a file of this server.
    pub audio: Vec<AudioStream>,
    /// The file's name, for a file of this server.
    pub file_name: Option<String>,
    /// The file's video, for a file of this server.
    pub video: Option<VideoStream>,
    /// The file's subtitles, in it and beside it, for a file of this server.
    pub subtitles: Vec<SubtitleStream>,
}

/// An audio stream of a file, to choose it.
#[derive(Debug, Clone, PartialEq)]
pub struct AudioStream {
    /// The stream's index in the file.
    pub index: i32,
    /// Its language.
    pub language: Option<String>,
    /// Its title.
    pub title: Option<String>,
    /// Its codec, such as "aac".
    pub codec: String,
    /// Its channels.
    pub channels: i32,
    /// Whether it is the default.
    pub default: bool,
}

/// A person of a server's library, with what the library credits them in.
#[derive(Debug, Clone, PartialEq)]
pub struct PersonDetails {
    /// Their name.
    pub name: String,
    /// Their biography.
    pub overview: Option<String>,
    /// When they were born.
    pub born: Option<SystemTime>,
    /// When they died.
    pub died: Option<SystemTime>,
    /// Where they were born.
    pub birth_place: Option<String>,
    /// Their photo.
    pub photo: Option<Artwork>,
    /// Pages about them on other sites.
    pub links: Vec<ExternalLink>,
    /// The movies, series and episodes crediting them, last released first.
    pub items: Vec<Card>,
}

/// An item's details.
#[derive(Debug, Clone, PartialEq)]
pub struct Details {
    /// The item, as its card.
    pub card: Card,
    /// The library holding it.
    pub library_id: Option<String>,
    /// The name of its server.
    pub server_name: String,
    /// What it is about.
    pub overview: Option<String>,
    /// Its tagline.
    pub tagline: Option<String>,
    /// Its genres.
    pub genres: Vec<String>,
    /// Its content rating, such as "PG-13".
    pub official_rating: Option<String>,
    /// Its community rating, from 0 to 10.
    pub community_rating: Option<f32>,
    /// When it was first released or aired.
    pub premiere: Option<SystemTime>,
    /// Its backdrop.
    pub backdrop: Option<Artwork>,
    /// Its logo.
    pub logo: Option<Artwork>,
    /// Where it resumes.
    pub position: Option<Duration>,
    /// The people credited, cast first.
    pub credits: Vec<CreditEntry>,
    /// Its files on this server, then its copies on the other servers.
    pub sources: Vec<Source>,
    /// Its seasons, episodes or tracks.
    pub children: Vec<Card>,
    /// Items like it.
    pub similar: Vec<Card>,
    /// Whether it comes from the offline store.
    pub offline: bool,
    /// An episode's season number.
    pub season_number: Option<i32>,
    /// An episode's number.
    pub episode_number: Option<i32>,
    /// An episode's season's name.
    pub season_name: Option<String>,
    /// The series an episode or season belongs to.
    pub series: Option<SeriesLink>,
    /// Whether a series still airs.
    pub series_status: Option<SeriesStatus>,
    /// When a series ended.
    pub end_date: Option<SystemTime>,
    /// The critics' score, from 0 to 100.
    pub critic_rating: Option<f32>,
    /// Its keywords.
    pub tags: Vec<String>,
    /// Its studios or networks; an episode's or season's are its series'.
    pub studios: Vec<String>,
    /// The countries it was made in.
    pub countries: Vec<String>,
    /// Its title in its original language, when that differs.
    pub original_title: Option<String>,
    /// Its original language, as a language code.
    pub original_language: Option<String>,
    /// When a movie was released digitally.
    pub digital_release: Option<SystemTime>,
    /// When a movie was released on discs.
    pub physical_release: Option<SystemTime>,
    /// When the library added it.
    pub date_added: Option<SystemTime>,
    /// Pages about it on other sites.
    pub links: Vec<ExternalLink>,
    /// A trailer on the web, for when it has none among its extras.
    pub remote_trailer: Option<String>,
    /// A series' episodes, a season's, or an episode's season's, in order.
    pub episodes: Vec<Episode>,
    /// The episode a series or season plays next; after an episode, the
    /// one that follows it in the series.
    pub next: Option<Episode>,
    /// Its trailers, featurettes and other extras.
    pub extras: Vec<Extra>,
    /// The collection a movie belongs to.
    pub collection: Option<CollectionLink>,
}

/// What the details fetch beyond the item.
#[derive(Default)]
struct More {
    series: Option<Item>,
    episodes: Vec<Item>,
    extras: Vec<Item>,
    collections: Vec<Item>,
}

/// The results of a search on one server.
#[derive(Debug, Clone, PartialEq)]
pub struct SearchResults {
    /// The server.
    pub server_id: String,
    /// Its name.
    pub server_name: String,
    /// What it found.
    pub cards: Vec<Card>,
    /// Why it could not search.
    pub error: Option<String>,
}

/// A download, as the Downloads screen shows it.
#[derive(Debug, Clone, PartialEq)]
pub struct DownloadEntry {
    /// The item, as its card.
    pub card: Card,
    /// The name of its server.
    pub server_name: String,
    /// How far it is.
    pub state: DownloadState,
    /// Where the app keeps the file, relative to its downloads folder.
    pub file: String,
    /// The bytes received.
    pub received: u64,
    /// The bytes in all; zero until the transfer starts.
    pub total: u64,
    /// Why it failed.
    pub error: Option<String>,
    /// When it was queued.
    pub added: SystemTime,
}

/// The core of an app: its servers, their event streams, playback
/// reporting, downloads and the data of its screens.
pub struct Core {
    store: Arc<Store>,
    servers: Arc<Servers>,
    reporter: Reporter,
    downloads: Downloads,
    relay: Arc<Relay>,
    streams: Arc<Streams>,
}

/// The event streams followed, by server.
type Streams = Mutex<HashMap<String, JoinHandle<()>>>;

/// Follows a server's event stream unless it is followed already.
fn follow(streams: &Streams, servers: &Arc<Servers>, relay: &Arc<Relay>, server_id: &str) {
    let mut streams = streams.lock().unwrap_or_else(|e| e.into_inner());
    if streams.get(server_id).is_some_and(|t| !t.is_finished()) {
        return;
    }
    let task = events::spawn(servers.clone(), server_id.to_owned(), relay.clone());
    streams.insert(server_id.to_owned(), task);
}

/// Follows the Mavio account's servers, then their event streams, stopping
/// those of the servers removed.
async fn sync_account(streams: &Streams, servers: &Arc<Servers>, relay: &Arc<Relay>) -> Result<()> {
    if !servers.sync_account().await? {
        return Ok(());
    }
    let list = servers.list()?;
    streams
        .lock()
        .unwrap_or_else(|e| e.into_inner())
        .retain(|id, task| {
            let keep = list.iter().any(|s| &s.id == id);
            if !keep {
                task.abort();
            }
            keep
        });
    for s in &list {
        if servers.signed_in(&s.id) {
            follow(streams, servers, relay, &s.id);
        }
    }
    relay.listener.servers_changed();
    Ok(())
}

/// Passes the streams' events on to the app, replaying missed progress
/// when a server comes back.
struct Relay {
    listener: Arc<dyn Listener>,
    reporter: Reporter,
    states: Mutex<HashMap<String, StreamState>>,
}

impl EventSink for Relay {
    fn state(&self, server_id: &str, state: StreamState) {
        self.states
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .insert(server_id.to_owned(), state.clone());
        self.listener.server_state(server_id, state);
    }

    fn event(&self, server_id: &str, event: &Event) {
        if matches!(event.kind, Some(Kind::Connected(_))) {
            let reporter = self.reporter.clone();
            let server_id = server_id.to_owned();
            tokio::spawn(async move {
                // A failure leaves the progress for the next connection.
                let _ = reporter.replay(&server_id).await;
            });
        }
        self.listener.event(server_id, event);
    }
}

impl Drop for Core {
    fn drop(&mut self) {
        self.stop();
    }
}

fn is_signed_out(e: &Error) -> bool {
    matches!(e, Error::Unauthenticated(_))
}

impl Core {
    /// The core of the servers kept in store, signed in with the tokens of
    /// tokens, as device; what the servers push goes to listener. Call it
    /// within a Tokio runtime.
    pub fn new(
        store: Arc<Store>,
        tokens: Arc<dyn TokenStore>,
        device: DeviceInfo,
        listener: Arc<dyn Listener>,
    ) -> Self {
        let servers = Arc::new(Servers::new(store.clone(), tokens, device));
        let reporter = Reporter::new(servers.clone(), store.clone());
        Core {
            downloads: Downloads::new(servers.clone(), store.clone(), reporter.clone()),
            relay: Arc::new(Relay {
                listener,
                reporter: reporter.clone(),
                states: Mutex::default(),
            }),
            store,
            servers,
            reporter,
            streams: Arc::default(),
        }
    }

    /// The servers.
    pub fn servers(&self) -> &Arc<Servers> {
        &self.servers
    }

    /// Playback and its reports.
    pub fn reporter(&self) -> &Reporter {
        &self.reporter
    }

    /// The downloads.
    pub fn downloads(&self) -> &Downloads {
        &self.downloads
    }

    /// The downloads of every server, in the order they were queued.
    pub fn download_entries(&self) -> Result<Vec<DownloadEntry>> {
        let servers: HashMap<String, ServerRecord> = self
            .servers
            .list()?
            .into_iter()
            .map(|s| (s.id.clone(), s))
            .collect();
        Ok(self
            .downloads
            .list()?
            .into_iter()
            .filter_map(|d| {
                let server = servers.get(&d.server_id)?;
                let base = self.base(server)?;
                Some(DownloadEntry {
                    card: Card::new(&base, &d.server_id, &d.item),
                    server_name: server.name.clone(),
                    state: d.state,
                    file: d.file,
                    received: d.received,
                    total: d.total,
                    error: Some(d.error).filter(|e| !e.is_empty()),
                    added: d.added,
                })
            })
            .collect())
    }

    /// Follows the event streams of every signed-in server, and in the
    /// background the Mavio account's servers; the app calls it when it
    /// comes to the foreground.
    pub fn start(&self) -> Result<()> {
        for s in self.servers.list()? {
            if self.servers.signed_in(&s.id) {
                self.follow(&s.id);
            }
        }
        if self.servers.account().is_some() {
            let (streams, servers, relay) = (
                self.streams.clone(),
                self.servers.clone(),
                self.relay.clone(),
            );
            tokio::spawn(async move {
                // A failure leaves the servers for the next start.
                let _ = sync_account(&streams, &servers, &relay).await;
            });
        }
        Ok(())
    }

    /// Stops the event streams; the app calls it when it goes to the
    /// background.
    pub fn stop(&self) {
        for (_, task) in self
            .streams
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .drain()
        {
            task.abort();
        }
    }

    fn follow(&self, server_id: &str) {
        follow(&self.streams, &self.servers, &self.relay, server_id);
    }

    /// The state of a server's stream, unknown before it first connects.
    pub fn server_state(&self, server_id: &str) -> Option<StreamState> {
        self.relay
            .states
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .get(server_id)
            .cloned()
    }

    /// Signs in to a server with a name and password.
    pub async fn sign_in(&self, address: &str, name: &str, password: &str) -> Result<ServerRecord> {
        let s = self.servers.sign_in(address, name, password).await?;
        self.follow(&s.id);
        Ok(s)
    }

    /// Creates the first user of a new server and signs in as them.
    pub async fn create_first_user(
        &self,
        address: &str,
        name: &str,
        password: &str,
    ) -> Result<ServerRecord> {
        let s = self
            .servers
            .create_first_user(address, name, password)
            .await?;
        self.follow(&s.id);
        Ok(s)
    }

    /// Asks whether a Quick Connect request was authorized, signing in when
    /// it was.
    pub async fn poll_quick_connect(&self, qc: &QuickConnect) -> Result<Option<ServerRecord>> {
        let s = self.servers.poll_quick_connect(qc).await?;
        if let Some(s) = &s {
            self.follow(&s.id);
        }
        Ok(s)
    }

    /// Signs out of a server and forgets it, with what it kept offline.
    pub async fn remove(&self, server_id: &str) -> Result<()> {
        if let Some(task) = self
            .streams
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .remove(server_id)
        {
            task.abort();
        }
        self.servers.remove(server_id).await
    }

    /// The users of a standalone server whom its session can switch to.
    pub async fn switchable_users(&self, server_id: &str) -> Result<Vec<SwitchableUser>> {
        self.servers.switchable_users(server_id).await
    }

    /// Switches a standalone server's session to another of its users, with
    /// their PIN when they have one, and follows its event stream again,
    /// since a stream keeps the user it was opened by.
    pub async fn switch_user(
        &self,
        server_id: &str,
        user_id: &str,
        pin: Option<&str>,
    ) -> Result<ServerRecord> {
        // Progress kept offline is the present user's, so it goes first;
        // a server that cannot take it cannot switch either.
        let _ = self.reporter.replay(server_id).await;
        let s = self.servers.switch_user(server_id, user_id, pin).await?;
        if let Some(task) = self
            .streams
            .lock()
            .unwrap_or_else(|e| e.into_inner())
            .remove(server_id)
        {
            task.abort();
            self.follow(server_id);
        }
        Ok(s)
    }

    /// The user's playback preferences, as the first server signed in to
    /// keeps them.
    pub async fn playback_preferences(&self) -> Result<PlaybackPreferences> {
        let id = self.signed_in_ids()?.into_iter().next();
        let id = id.ok_or_else(|| Error::NotFound("a server signed in to".into()))?;
        let r = self
            .servers
            .connection(&id)?
            .users()
            .get_current_user(GetCurrentUserRequest::default())
            .await?
            .into_owned();
        let user = r.user.into_option().unwrap_or_default();
        Ok(PlaybackPreferences::from_proto(
            &user.playback_preferences.into_option().unwrap_or_default(),
        ))
    }

    /// Saves the user's playback preferences to every server signed in to,
    /// failing with the first server that refuses them.
    pub async fn set_playback_preferences(&self, prefs: &PlaybackPreferences) -> Result<()> {
        let mut calls = JoinSet::new();
        for id in self.signed_in_ids()? {
            let conn = self.servers.connection(&id)?;
            let req = UpdatePlaybackPreferencesRequest {
                playback_preferences: prefs.to_proto().into(),
                ..Default::default()
            };
            calls.spawn(async move { conn.users().update_playback_preferences(req).await });
        }
        while let Some(done) = calls.join_next().await {
            if let Ok(r) = done {
                r?;
            }
        }
        Ok(())
    }

    /// The servers signed in to, in the user's order.
    fn signed_in_ids(&self) -> Result<Vec<String>> {
        Ok(self
            .servers
            .list()?
            .into_iter()
            .filter(|s| self.servers.signed_in(&s.id))
            .map(|s| s.id)
            .collect())
    }

    /// The Mavio account signed in, if any.
    pub fn account(&self) -> Option<Account> {
        self.servers.account()
    }

    /// What the device is signed in with, which decides whether the app
    /// shows its sign-in.
    pub fn sign_in_state(&self) -> Result<SignInState> {
        self.servers.sign_in_state()
    }

    /// Starts signing a Mavio account in with a link code.
    pub async fn start_link(&self, api: &str, client_id: &str) -> Result<LinkCode> {
        self.servers.start_link(api, client_id).await
    }

    /// Asks whether a link code was approved; once it was, signs the
    /// account in and follows its servers.
    pub async fn poll_link(&self, link: &LinkCode) -> Result<Option<Account>> {
        let a = self.servers.poll_link(link).await?;
        if a.is_some() {
            self.sync_account().await?;
        }
        Ok(a)
    }

    /// Starts signing a Mavio account in in the browser.
    pub async fn start_browser_sign_in(&self, api: &str, client_id: &str) -> Result<BrowserSignIn> {
        self.servers.start_browser_sign_in(api, client_id).await
    }

    /// Waits for the browser to come back, signs the account in and
    /// follows its servers.
    pub async fn finish_browser_sign_in(&self, sign_in: &BrowserSignIn) -> Result<Account> {
        let a = self.servers.finish_browser_sign_in(sign_in).await?;
        self.sync_account().await?;
        Ok(a)
    }

    /// Starts signing a Mavio account in in the browser, for the app to
    /// receive the browser at redirect_uri itself.
    pub fn start_app_sign_in(
        &self,
        api: &str,
        client_id: &str,
        redirect_uri: &str,
    ) -> BrowserSignIn {
        self.servers.start_app_sign_in(api, client_id, redirect_uri)
    }

    /// Signs the account in from the URL the browser came back to and
    /// follows its servers.
    pub async fn finish_app_sign_in(
        &self,
        sign_in: &BrowserSignIn,
        callback: &str,
    ) -> Result<Account> {
        let a = self.servers.finish_app_sign_in(sign_in, callback).await?;
        self.sync_account().await?;
        Ok(a)
    }

    /// Calls a unary procedure of a server signed in to as its session,
    /// with a request encoded as protobuf, answering with the response
    /// encoded the same way and the base URL it reached the server at.
    pub async fn call(
        &self,
        server_id: &str,
        procedure: &str,
        request: Vec<u8>,
    ) -> Result<(Vec<u8>, String)> {
        let conn = self.servers.connection(server_id)?;
        let body = conn.call(procedure, request).await?;
        Ok((body, conn.url("")))
    }

    /// Resolves a plain HTTP path of a server signed in to, answering its
    /// URL and the session's access token to send it with.
    pub async fn authorize(&self, server_id: &str, path: &str) -> Result<(String, String)> {
        let conn = self.servers.connection(server_id)?;
        let token = conn
            .token()
            .await?
            .ok_or_else(|| Error::Unauthenticated("signed out".into()))?;
        Ok((conn.url(path), token))
    }

    /// Follows the Mavio account's servers now.
    pub async fn sync_account(&self) -> Result<()> {
        sync_account(&self.streams, &self.servers, &self.relay).await
    }

    /// Signs out: of the Mavio account and its servers, or of every server
    /// signed in to without one, forgetting them with what they kept
    /// offline.
    pub async fn sign_out(&self) -> Result<()> {
        self.stop();
        if self.servers.account().is_some() {
            return self.servers.sign_out_account().await;
        }
        for s in self.servers.list()? {
            self.servers.remove(&s.id).await?;
        }
        Ok(())
    }

    /// The connection to resolve a server's paths, signed out when the
    /// device has no token of it.
    fn base(&self, server: &ServerRecord) -> Option<Connection> {
        self.servers
            .connection(&server.id)
            .or_else(|_| Connection::new(&server.address))
            .ok()
    }

    /// Loads Home from every server at once, telling observer the rows
    /// each time a server answers: first what the offline store kept, then
    /// each server's hubs as they arrive.
    pub async fn home(&self, observer: &dyn HomeObserver) -> Result<()> {
        let list = self.servers.list()?;
        let bases: HashMap<String, Connection> = list
            .iter()
            .filter_map(|s| Some((s.id.clone(), self.base(s)?)))
            .collect();
        let draw = |slots: &[ServerHubs]| {
            observer.rows(view::rows(home::merge(slots), |id| bases.get(id).cloned()));
        };

        let mut slots = Vec::with_capacity(list.len());
        let mut calls = JoinSet::new();
        for (i, s) in list.iter().enumerate() {
            let cached = self.store.cached::<ListHubsResponse>(&s.id, "home")?;
            let hubs = match (self.servers.connection(&s.id), cached) {
                (Ok(conn), cached) => {
                    calls.spawn(async move {
                        let r = conn.hub().list_hubs(ListHubsRequest::default()).await;
                        (i, r.map(|r| r.into_owned()).map_err(Error::from))
                    });
                    match cached {
                        Some((r, at)) => Hubs::Cached(r.hubs, at),
                        None => Hubs::Loading,
                    }
                }
                (Err(e), Some((r, at))) if !is_signed_out(&e) => Hubs::Cached(r.hubs, at),
                (Err(e), _) => Hubs::Unavailable(e.to_string()),
            };
            slots.push(ServerHubs {
                server_id: s.id.clone(),
                server_name: s.name.clone(),
                hubs,
            });
        }
        draw(&slots);

        while let Some(done) = calls.join_next().await {
            let Ok((i, result)) = done else { continue };
            let slot = &mut slots[i];
            slot.hubs = match (result, std::mem::replace(&mut slot.hubs, Hubs::Loading)) {
                (Ok(r), _) => {
                    self.store.put_cached(&slot.server_id, "home", &r)?;
                    Hubs::Fresh(r.hubs)
                }
                (Err(_), kept @ Hubs::Cached(..)) => kept,
                (Err(e), _) => Hubs::Unavailable(e.to_string()),
            };
            draw(&slots);
        }
        Ok(())
    }

    /// More items of a server's hub, from offset.
    pub async fn hub(
        &self,
        server_id: &str,
        hub_id: &str,
        offset: u32,
        limit: u32,
    ) -> Result<Page> {
        let conn = self.servers.connection(server_id)?;
        let hub = conn
            .hub()
            .get_hub(GetHubRequest {
                id: Some(hub_id.to_owned()),
                offset: Some(offset as i32),
                limit: Some(limit as i32),
                ..Default::default()
            })
            .await?
            .into_owned()
            .hub
            .into_option()
            .unwrap_or_default();
        let cards: Vec<Card> = hub
            .items
            .iter()
            .map(|i| Card::new(&conn, server_id, i))
            .collect();
        Ok(Page {
            cards,
            total: None,
            more: hub.more.unwrap_or_default(),
        })
    }

    /// A library's Recommended tab: its hubs, as rows of its server.
    pub async fn recommended(&self, server_id: &str, library_id: &str) -> Result<Vec<Row>> {
        let server = self.servers.get(server_id)?;
        let conn = self.servers.connection(server_id)?;
        let r = conn
            .hub()
            .list_hubs(ListHubsRequest {
                library_id: Some(library_id.to_owned()),
                ..Default::default()
            })
            .await?
            .into_owned();
        let slot = ServerHubs {
            server_id: server.id,
            server_name: server.name,
            hubs: Hubs::Fresh(r.hubs),
        };
        Ok(view::rows(home::merge(std::slice::from_ref(&slot)), |_| {
            Some(conn.clone())
        }))
    }

    /// Searches every signed-in server at once, the results in the user's
    /// order of servers.
    pub async fn search(&self, query: &str) -> Result<Vec<SearchResults>> {
        let query = query.trim();
        let list = self.servers.list()?;
        let mut calls = JoinSet::new();
        let mut out: Vec<SearchResults> = Vec::with_capacity(list.len());
        for (i, s) in list.into_iter().enumerate() {
            let error = match self.servers.connection(&s.id) {
                Ok(conn) => {
                    let req = ListItemsRequest {
                        search: Some(query.to_owned()),
                        kinds: SEARCH_KINDS.iter().map(|&k| k.into()).collect(),
                        limit: Some(SEARCH_LIMIT),
                        ..Default::default()
                    };
                    let server_id = s.id.clone();
                    calls.spawn(async move {
                        let r = conn.items().list_items(req).await.map_err(Error::from);
                        let cards = r.map(|r| {
                            let r = r.into_owned();
                            r.items
                                .iter()
                                .map(|i| Card::new(&conn, &server_id, i))
                                .collect()
                        });
                        (i, cards)
                    });
                    None
                }
                Err(e) => Some(e.to_string()),
            };
            out.push(SearchResults {
                server_id: s.id,
                server_name: s.name,
                cards: Vec::new(),
                error,
            });
        }
        while let Some(done) = calls.join_next().await {
            let Ok((i, result)) = done else { continue };
            match result {
                Ok(cards) => out[i].cards = cards,
                Err(e) => out[i].error = Some(e.to_string()),
            }
        }
        Ok(out)
    }

    /// A server's libraries, from the offline store when it is unreachable.
    pub async fn libraries(&self, server_id: &str) -> Result<Libraries> {
        let server = self.servers.get(server_id)?;
        let fetched = match self.servers.connection(server_id) {
            Ok(conn) => conn
                .libraries()
                .list_libraries(ListLibrariesRequest::default())
                .await
                .map(|r| r.into_owned())
                .map_err(Error::from),
            Err(e) => Err(e),
        };
        let (r, offline) = match fetched {
            Ok(r) => {
                self.store.put_cached(server_id, "libraries", &r)?;
                (r, false)
            }
            Err(e) if e.is_transient() => {
                match self
                    .store
                    .cached::<ListLibrariesResponse>(server_id, "libraries")?
                {
                    Some((r, _)) => (r, true),
                    None => return Err(e),
                }
            }
            Err(e) => return Err(e),
        };
        let base = self.base(&server);
        Ok(Libraries {
            libraries: r
                .libraries
                .iter()
                .map(|l| {
                    let id = l.id.clone().unwrap_or_default();
                    LibraryEntry {
                        server_id: server_id.to_owned(),
                        image_url: base
                            .as_ref()
                            .zip(l.image_url.as_deref().filter(|u| !u.is_empty()))
                            .map(|(b, u)| b.url(u))
                            .unwrap_or_default(),
                        id,
                        name: l.name.clone().unwrap_or_default(),
                        kind: l.kind.as_ref().and_then(|k| k.as_known()).into(),
                    }
                })
                .collect(),
            offline,
        })
    }

    /// A page of a library's grid: the items at its top, sorted and
    /// filtered.
    pub async fn library_items(
        &self,
        server_id: &str,
        library_id: &str,
        sort: Sort,
        filter: &Filter,
        offset: u32,
        limit: u32,
    ) -> Result<Page> {
        let conn = self.servers.connection(server_id)?;
        let (field, descending) = match sort {
            Sort::Name => (SortField::SORT_FIELD_NAME, false),
            Sort::DateAdded => (SortField::SORT_FIELD_DATE_ADDED, true),
            Sort::Released => (SortField::SORT_FIELD_PREMIERE_DATE, true),
            Sort::Rating => (SortField::SORT_FIELD_COMMUNITY_RATING, true),
            Sort::LastPlayed => (SortField::SORT_FIELD_LAST_PLAYED, true),
            Sort::Random => (SortField::SORT_FIELD_RANDOM, false),
        };
        let r = conn
            .items()
            .list_items(ListItemsRequest {
                library_ids: vec![library_id.to_owned()],
                top_level: Some(true),
                sort: vec![SortSpec {
                    field: Some(field.into()),
                    descending: descending.then_some(true),
                    ..Default::default()
                }],
                played: filter.played,
                favorite: filter.favorite.then_some(true),
                genres: filter.genre.iter().cloned().collect(),
                tags: filter.tag.iter().cloned().collect(),
                studios: filter.studio.iter().cloned().collect(),
                offset: Some(offset as i32),
                limit: Some(limit as i32),
                ..Default::default()
            })
            .await?
            .into_owned();
        let total = r.total.unwrap_or_default().max(0) as u32;
        Ok(Page {
            more: offset as usize + r.items.len() < total as usize,
            cards: r
                .items
                .iter()
                .map(|i| Card::new(&conn, server_id, i))
                .collect(),
            total: Some(total),
        })
    }

    /// A person of a server's library and the movies, series and episodes
    /// crediting them.
    pub async fn person(&self, server_id: &str, person_id: &str) -> Result<PersonDetails> {
        let server = self.servers.get(server_id)?;
        let conn = self.servers.connection(server_id)?;
        let base = self
            .base(&server)
            .ok_or_else(|| Error::Invalid(format!("address of {}", server.name)))?;
        let items = conn.items();
        let (person, credited) = tokio::join!(
            items.get_person(GetPersonRequest {
                id: Some(person_id.to_owned()),
                ..Default::default()
            }),
            items.list_items(ListItemsRequest {
                person_id: Some(person_id.to_owned()),
                kinds: [
                    ItemKind::ITEM_KIND_MOVIE,
                    ItemKind::ITEM_KIND_SERIES,
                    ItemKind::ITEM_KIND_EPISODE
                ]
                .into_iter()
                .map(Into::into)
                .collect(),
                sort: vec![SortSpec {
                    field: Some(SortField::SORT_FIELD_PREMIERE_DATE.into()),
                    descending: Some(true),
                    ..Default::default()
                }],
                ..Default::default()
            }),
        );
        let p = person?
            .into_owned()
            .person
            .as_option()
            .cloned()
            .unwrap_or_default();
        let photo = Item {
            images: p.images.clone(),
            ..Default::default()
        };
        Ok(PersonDetails {
            name: p.name.clone().unwrap_or_default(),
            overview: details::non_empty(&p.overview),
            born: details::time(p.birth_date.as_option()),
            died: details::time(p.death_date.as_option()),
            birth_place: details::non_empty(&p.birth_place),
            photo: view::artwork(&base, &photo, ImageKind::IMAGE_KIND_PRIMARY),
            links: p
                .external_urls
                .iter()
                .filter_map(|l| {
                    Some(ExternalLink {
                        name: details::non_empty(&l.name)?,
                        url: details::non_empty(&l.url)?,
                    })
                })
                .collect(),
            items: credited?
                .into_owned()
                .items
                .iter()
                .map(|i| Card::new(&conn, server_id, i))
                .collect(),
        })
    }

    /// An item's details with its seasons, episodes or tracks, items like
    /// it, and its copies on the user's other servers; from the offline
    /// store when its server is unreachable.
    pub async fn details(&self, server_id: &str, item_id: &str) -> Result<Details> {
        let server = self.servers.get(server_id)?;
        let key = format!("item/{item_id}");
        let conn = self.servers.connection(server_id);
        let fetched = match &conn {
            Ok(conn) => conn
                .items()
                .get_item(GetItemRequest {
                    id: Some(item_id.to_owned()),
                    ..Default::default()
                })
                .await
                .map(|r| r.into_owned())
                .map_err(Error::from),
            Err(e) => Err(e.clone()),
        };
        let (r, offline) = match fetched {
            Ok(r) => {
                self.store.put_cached(server_id, &key, &r)?;
                (r, false)
            }
            Err(e) if e.is_transient() || is_signed_out(&e) => {
                match self.store.cached::<GetItemResponse>(server_id, &key)? {
                    Some((r, _)) => (r, true),
                    None => return Err(e),
                }
            }
            Err(e) => return Err(e),
        };
        let base = self
            .base(&server)
            .ok_or_else(|| Error::Invalid(format!("address of {}", server.name)))?;
        let item = r.item.as_option().cloned().unwrap_or_default();

        let kind = item.kind.as_ref().and_then(|k| k.as_known());
        let in_series = matches!(
            kind,
            Some(ItemKind::ITEM_KIND_EPISODE | ItemKind::ITEM_KIND_SEASON)
        );
        let series_id = match kind {
            Some(ItemKind::ITEM_KIND_SERIES) => item.id.clone(),
            _ if in_series => details::non_empty(&item.series_id),
            _ => None,
        };

        let (children, similar, others, more) = match (&conn, offline) {
            (Ok(conn), false) => tokio::join!(
                self.children(conn, server_id, &item),
                self.similar(conn, server_id, item_id),
                self.other_copies(server_id, &item),
                self.more(conn, &item, series_id.as_deref(), in_series),
            ),
            _ => (Vec::new(), Vec::new(), Vec::new(), More::default()),
        };

        let mut card = Card::new(&base, server_id, &item);
        card.versions = others
            .iter()
            .map(|s| Version {
                server_id: s.server_id.clone(),
                item_id: s.item_id.clone(),
            })
            .collect();
        let mut sources: Vec<Source> = r
            .media_sources
            .iter()
            .map(|m| Source {
                server_id: server_id.to_owned(),
                server_name: server.name.clone(),
                item_id: item_id.to_owned(),
                media_source_id: m.id.clone(),
                name: m.name.clone().filter(|n| !n.is_empty()),
                size: m.size_bytes.filter(|s| *s > 0).map(|s| s as u64),
                container: m.container.clone().filter(|c| !c.is_empty()),
                audio: m
                    .streams
                    .iter()
                    .filter(|s| {
                        s.kind.as_ref().and_then(|k| k.as_known())
                            == Some(StreamKind::STREAM_KIND_AUDIO)
                    })
                    .map(|s| AudioStream {
                        index: s.index.unwrap_or_default(),
                        language: s.language.clone().filter(|l| !l.is_empty()),
                        title: s.title.clone().filter(|t| !t.is_empty()),
                        codec: s.codec.clone().unwrap_or_default(),
                        channels: s.channels.unwrap_or_default(),
                        default: s.default.unwrap_or_default(),
                    })
                    .collect(),
                file_name: m
                    .path
                    .as_deref()
                    .and_then(|p| p.rsplit(['/', '\\']).next())
                    .filter(|n| !n.is_empty())
                    .map(str::to_owned),
                video: details::video(&m.streams),
                subtitles: details::subtitles(&m.streams),
            })
            .collect();
        sources.extend(others);

        // The cast in billing order, each actor once, then the crew.
        let mut seen = std::collections::HashSet::new();
        let mut credits: Vec<(bool, i32, CreditEntry)> = r
            .credits
            .iter()
            .filter_map(|c| {
                let person = c.person.as_option()?;
                let job = CreditJob::from(c.kind.as_ref().and_then(|k| k.as_known()));
                let cast = matches!(job, CreditJob::Actor | CreditJob::GuestStar);
                let person_id = person.id.clone().unwrap_or_default();
                if cast && !seen.insert(person_id.clone()) {
                    return None;
                }
                let photo = Item {
                    images: person.images.clone(),
                    ..Default::default()
                };
                Some((
                    !cast,
                    c.order.unwrap_or_default(),
                    CreditEntry {
                        person_id,
                        name: person.name.clone().unwrap_or_default(),
                        role: c.role.clone().filter(|r| !r.is_empty()),
                        cast,
                        job,
                        photo: view::artwork(&base, &photo, ImageKind::IMAGE_KIND_PRIMARY),
                    },
                ))
            })
            .collect();
        credits.sort_by_key(|(crew, order, _)| (*crew, *order));

        // A series' episodes, a season's, or an episode's season's; a
        // series or season plays the one to watch next.
        let own_season = match kind {
            Some(ItemKind::ITEM_KIND_SEASON) => item.id.clone(),
            Some(ItemKind::ITEM_KIND_EPISODE) => details::non_empty(&item.season_id),
            _ => None,
        };
        let shown: Vec<&Item> = more
            .episodes
            .iter()
            .filter(|e| own_season.is_none() || e.season_id == own_season)
            .collect();
        let next = match kind {
            Some(ItemKind::ITEM_KIND_SERIES | ItemKind::ITEM_KIND_SEASON) => {
                let list: Vec<Item> = shown.iter().map(|e| (*e).clone()).collect();
                details::next_up(&list).map(|e| Episode::new(&base, server_id, e))
            }
            // An episode, the one after it in the series, which the
            // player plays at its end.
            Some(ItemKind::ITEM_KIND_EPISODE) => more
                .episodes
                .iter()
                .skip_while(|e| e.id != item.id)
                .nth(1)
                .map(|e| Episode::new(&base, server_id, e)),
            _ => None,
        };
        let episodes = shown
            .into_iter()
            .map(|e| Episode::new(&base, server_id, e))
            .collect();
        let series = more.series.as_ref();
        let from_series = |own: &Vec<String>, theirs: fn(&Item) -> &Vec<String>| {
            if own.is_empty() {
                series.map(theirs).cloned().unwrap_or_default()
            } else {
                own.clone()
            }
        };

        Ok(Details {
            library_id: details::non_empty(&item.library_id),
            server_name: server.name.clone(),
            overview: item.overview.clone().filter(|s| !s.is_empty()),
            tagline: item.tagline.clone().filter(|s| !s.is_empty()),
            genres: from_series(&item.genres, |s| &s.genres),
            official_rating: item.official_rating.clone().filter(|s| !s.is_empty()),
            community_rating: item.community_rating.filter(|r| *r > 0.0).map(|r| r as f32),
            premiere: details::time(item.premiere_date.as_option()),
            backdrop: view::artwork(&base, &item, ImageKind::IMAGE_KIND_BACKDROP).or_else(|| {
                series.and_then(|s| view::artwork(&base, s, ImageKind::IMAGE_KIND_BACKDROP))
            }),
            logo: view::artwork(&base, &item, ImageKind::IMAGE_KIND_LOGO),
            position: details::position(&item),
            credits: credits.into_iter().map(|(_, _, c)| c).collect(),
            sources,
            children,
            similar,
            offline,
            season_number: item
                .parent_index_number
                .filter(|_| kind == Some(ItemKind::ITEM_KIND_EPISODE)),
            episode_number: item
                .index_number
                .filter(|n| *n > 0 && kind == Some(ItemKind::ITEM_KIND_EPISODE)),
            season_name: details::non_empty(&item.season_name)
                .filter(|_| kind == Some(ItemKind::ITEM_KIND_EPISODE)),
            series: series.map(|s| details::series_link(&base, s)),
            series_status: details::series_status(&item),
            end_date: details::time(item.end_date.as_option()),
            critic_rating: item.critic_rating.filter(|r| *r > 0.0).map(|r| r as f32),
            tags: if in_series {
                Vec::new()
            } else {
                item.tags.clone()
            },
            studios: from_series(&item.studios, |s| &s.studios),
            countries: item.production_locations.clone(),
            original_title: details::non_empty(&item.original_title)
                .filter(|t| item.name.as_ref() != Some(t)),
            original_language: details::non_empty(&item.original_language),
            digital_release: details::time(item.digital_release_date.as_option()),
            physical_release: details::time(item.physical_release_date.as_option()),
            date_added: details::time(item.date_added.as_option())
                .filter(|_| !item.r#virtual.unwrap_or_default()),
            links: details::links(&item),
            remote_trailer: item.remote_trailers.iter().find(|t| !t.is_empty()).cloned(),
            episodes,
            next,
            extras: more
                .extras
                .iter()
                .filter_map(|e| details::extra(&base, server_id, e))
                .collect(),
            collection: details::collection_link(&base, &item, &more.collections),
            card,
        })
    }

    /// What the details show beyond the item: its series, the series'
    /// episodes, its extras and the collections it may belong to.
    async fn more(
        &self,
        conn: &Connection,
        item: &Item,
        series_id: Option<&str>,
        in_series: bool,
    ) -> More {
        let list = |req: ListItemsRequest| async move {
            conn.items()
                .list_items(req)
                .await
                .map(|r| r.into_owned().items)
                .unwrap_or_default()
        };
        let series = async {
            let id = series_id.filter(|_| in_series)?;
            let r = conn
                .items()
                .get_item(GetItemRequest {
                    id: Some(id.to_owned()),
                    ..Default::default()
                })
                .await
                .ok()?;
            r.into_owned().item.into_option()
        };
        let episodes = async {
            let Some(id) = series_id else {
                return Vec::new();
            };
            let mut items = list(ListItemsRequest {
                parent_id: Some(id.to_owned()),
                recursive: Some(true),
                kinds: vec![ItemKind::ITEM_KIND_EPISODE.into()],
                ..Default::default()
            })
            .await;
            details::sort_episodes(&mut items);
            items
        };
        let extras = list(ListItemsRequest {
            owner_id: item.id.clone(),
            sort: vec![SortSpec {
                field: Some(SortField::SORT_FIELD_NAME.into()),
                ..Default::default()
            }],
            ..Default::default()
        });
        let collections = async {
            if item.collection_name.as_deref().is_none_or(str::is_empty) {
                return Vec::new();
            }
            list(ListItemsRequest {
                kinds: vec![ItemKind::ITEM_KIND_COLLECTION.into()],
                ..Default::default()
            })
            .await
        };
        let (series, episodes, extras, collections) =
            tokio::join!(series, episodes, extras, collections);
        More {
            series,
            episodes,
            extras,
            collections,
        }
    }

    async fn children(&self, conn: &Connection, server_id: &str, item: &Item) -> Vec<Card> {
        let Some(id) = item.id.clone() else {
            return Vec::new();
        };
        let req = ListItemsRequest {
            parent_id: Some(id),
            sort: vec![SortSpec {
                field: Some(SortField::SORT_FIELD_INDEX.into()),
                ..Default::default()
            }],
            ..Default::default()
        };
        match conn.items().list_items(req).await {
            Ok(r) => r
                .into_owned()
                .items
                .iter()
                .map(|i| Card::new(conn, server_id, i))
                .collect(),
            Err(_) => Vec::new(),
        }
    }

    async fn similar(&self, conn: &Connection, server_id: &str, item_id: &str) -> Vec<Card> {
        let req = ListSimilarItemsRequest {
            id: Some(item_id.to_owned()),
            ..Default::default()
        };
        match conn.items().list_similar_items(req).await {
            Ok(r) => r
                .into_owned()
                .items
                .iter()
                .map(|i| Card::new(conn, server_id, i))
                .collect(),
            Err(_) => Vec::new(),
        }
    }

    /// The same title on the user's other servers, matched by its external
    /// IDs, in the user's order of servers.
    async fn other_copies(&self, server_id: &str, item: &Item) -> Vec<Source> {
        let ids: HashMap<String, String> = item
            .external_ids
            .iter()
            .filter(|(p, v)| matches!(p.as_str(), "tmdb" | "tvdb" | "imdb") && !v.is_empty())
            .map(|(p, v)| (p.clone(), v.clone()))
            .collect();
        if ids.is_empty() {
            return Vec::new();
        }
        let Ok(list) = self.servers.list() else {
            return Vec::new();
        };
        let mut calls = JoinSet::new();
        for (i, s) in list.into_iter().enumerate() {
            if s.id == server_id {
                continue;
            }
            let Ok(conn) = self.servers.connection(&s.id) else {
                continue;
            };
            let req = ListItemsRequest {
                kinds: item.kind.iter().cloned().collect(),
                recursive: Some(true),
                external_ids: ids.iter().map(|(k, v)| (k.clone(), v.clone())).collect(),
                limit: Some(1),
                ..Default::default()
            };
            calls.spawn(async move {
                let found = conn.items().list_items(req).await.ok()?.into_owned();
                let item = found.items.into_iter().next()?;
                Some((
                    i,
                    Source {
                        server_id: s.id,
                        server_name: s.name,
                        item_id: item.id.unwrap_or_default(),
                        media_source_id: None,
                        name: None,
                        size: None,
                        container: None,
                        audio: Vec::new(),
                        file_name: None,
                        video: None,
                        subtitles: Vec::new(),
                    },
                ))
            });
        }
        let mut found: Vec<(usize, Source)> =
            calls.join_all().await.into_iter().flatten().collect();
        found.sort_by_key(|(i, _)| *i);
        found.into_iter().map(|(_, s)| s).collect()
    }

    /// Marks an item played or unplayed.
    pub async fn set_played(&self, server_id: &str, item_id: &str, played: bool) -> Result<()> {
        self.update_user_data(
            server_id,
            UpdateUserDataRequest {
                item_id: Some(item_id.to_owned()),
                played: Some(played),
                ..Default::default()
            },
        )
        .await
    }

    /// Marks an item a favorite or not.
    pub async fn set_favorite(&self, server_id: &str, item_id: &str, favorite: bool) -> Result<()> {
        self.update_user_data(
            server_id,
            UpdateUserDataRequest {
                item_id: Some(item_id.to_owned()),
                favorite: Some(favorite),
                ..Default::default()
            },
        )
        .await
    }

    async fn update_user_data(&self, server_id: &str, req: UpdateUserDataRequest) -> Result<()> {
        let conn = self.servers.connection(server_id)?;
        conn.user_data().update_user_data(req).await?;
        Ok(())
    }
}

#[cfg(test)]
mod tests {
    use mavio_api::proto::mavio::hub::v1::Hub;
    use mavio_api::proto::mavio::library::v1::{
        Credit, CreditKind, ExternalUrl, ExtraKind, GetPersonResponse, HubKind, ItemKind,
        ListItemsResponse, ListSimilarItemsResponse, MediaSource, MediaStream, Person,
    };

    use super::*;
    use crate::servers::tests::{MemoryTokens, device, fake_server};
    use crate::view::RowContent;

    struct Quiet;

    impl Listener for Quiet {
        fn server_state(&self, _: &str, _: StreamState) {}
        fn event(&self, _: &str, _: &Event) {}
    }

    #[derive(Default)]
    struct Frames(Mutex<Vec<Vec<String>>>);

    impl HomeObserver for Frames {
        fn rows(&self, rows: Vec<Row>) {
            let frame = rows
                .iter()
                .map(|r| {
                    let content = match &r.content {
                        RowContent::Items(c) => c
                            .iter()
                            .map(|c| format!("{}:{}", c.server_id, c.item_id))
                            .collect::<Vec<_>>()
                            .join(" "),
                        RowContent::Titles(_) => "titles".into(),
                        RowContent::Loading => "loading".into(),
                        RowContent::Unavailable(_) => "unavailable".into(),
                    };
                    let offline = if r.offline { " (offline)" } else { "" };
                    format!("{}{offline}: {content}", r.id)
                })
                .collect();
            self.0.lock().unwrap().push(frame);
        }
    }

    fn core() -> (Core, Arc<Store>) {
        let store = Arc::new(Store::open_in_memory().unwrap());
        let core = Core::new(
            store.clone(),
            Arc::new(MemoryTokens::default()),
            device(),
            Arc::new(Quiet),
        );
        (core, store)
    }

    fn movie(id: &str, tmdb: &str) -> Item {
        Item {
            id: Some(id.into()),
            name: Some(id.into()),
            kind: Some(ItemKind::ITEM_KIND_MOVIE.into()),
            external_ids: [("tmdb".to_string(), tmdb.to_string())]
                .into_iter()
                .collect(),
            ..Default::default()
        }
    }

    fn hubs(items: Vec<Item>) -> ListHubsResponse {
        ListHubsResponse {
            hubs: vec![Hub {
                id: Some("recently-added/l".into()),
                kind: Some(HubKind::HUB_KIND_RECENTLY_ADDED.into()),
                items,
                ..Default::default()
            }],
            ..Default::default()
        }
    }

    #[tokio::test]
    async fn home_draws_kept_hubs_then_each_server_as_it_answers() {
        let (core, store) = core();
        let a = fake_server("sa", "Home");
        a.unary(
            "/mavio.hub.v1.HubService/ListHubs",
            |_, _: ListHubsRequest| Ok(hubs(vec![movie("a1", "1")])),
        );
        let b = fake_server("sb", "Friend");
        b.unary(
            "/mavio.hub.v1.HubService/ListHubs",
            |_, _: ListHubsRequest| Err::<ListHubsResponse, _>(crate::fake::error("unavailable")),
        );
        let (a, b) = (a.serve().await, b.serve().await);
        core.servers.sign_in(&a, "me", "secret").await.unwrap();
        core.servers.sign_in(&b, "me", "secret").await.unwrap();
        // b answered before, and its hubs were kept.
        store
            .put_cached("sb", "home", &hubs(vec![movie("b1", "2")]))
            .unwrap();

        let frames = Frames::default();
        core.home(&frames).await.unwrap();
        let frames = frames.0.into_inner().unwrap();
        assert_eq!(
            frames[0],
            ["sa/: loading", "sb/recently-added/l (offline): sb:b1"],
        );
        assert_eq!(
            frames.last().unwrap(),
            &[
                "sa/recently-added/l: sa:a1",
                "sb/recently-added/l (offline): sb:b1"
            ],
        );
        assert!(
            store
                .cached::<ListHubsResponse>("sa", "home")
                .unwrap()
                .is_some()
        );
    }

    #[tokio::test]
    async fn servers_added_by_address_are_a_standalone_sign_in() {
        let (core, _) = core();
        assert_eq!(core.sign_in_state().unwrap(), SignInState::SignedOut);
        let a = fake_server("sa", "Home").serve().await;
        core.servers.sign_in(&a, "me", "secret").await.unwrap();
        assert_eq!(core.sign_in_state().unwrap(), SignInState::Standalone);
    }

    #[tokio::test]
    async fn a_person_carries_the_items_crediting_them() {
        let (core, _) = core();
        let a = fake_server("sa", "Home");
        a.unary(
            "/mavio.library.v1.ItemService/GetPerson",
            |_, req: GetPersonRequest| {
                assert_eq!(req.id.as_deref(), Some("p1"));
                Ok(GetPersonResponse {
                    person: Person {
                        id: Some("p1".into()),
                        name: Some("Ada".into()),
                        overview: Some(String::new()),
                        birth_place: Some("London".into()),
                        external_urls: vec![ExternalUrl {
                            name: Some("IMDb".into()),
                            url: Some("https://www.imdb.com/name/nm1".into()),
                            ..Default::default()
                        }],
                        ..Default::default()
                    }
                    .into(),
                    ..Default::default()
                })
            },
        );
        a.unary(
            "/mavio.library.v1.ItemService/ListItems",
            |_, req: ListItemsRequest| {
                assert_eq!(req.person_id.as_deref(), Some("p1"));
                assert_eq!(req.kinds.len(), 3, "got = {:?}", req.kinds);
                Ok(ListItemsResponse {
                    items: vec![movie("m1", "1")],
                    ..Default::default()
                })
            },
        );
        let a = a.serve().await;
        core.servers.sign_in(&a, "me", "secret").await.unwrap();

        let got = core.person("sa", "p1").await.unwrap();
        assert_eq!(got.name, "Ada");
        assert_eq!(got.overview, None, "got = {:?}, want = None", got.overview);
        assert_eq!(got.birth_place.as_deref(), Some("London"));
        assert_eq!(got.links.len(), 1, "got = {:?}", got.links);
        assert_eq!(got.items.len(), 1, "got = {:?}", got.items);
        assert_eq!(got.items[0].item_id, "m1");
    }

    #[tokio::test]
    async fn a_library_is_narrowed_to_a_keyword_or_studio() {
        let (core, _) = core();
        let a = fake_server("sa", "Home");
        a.unary(
            "/mavio.library.v1.ItemService/ListItems",
            |_, req: ListItemsRequest| {
                assert_eq!(req.tags, vec!["heist".to_string()]);
                assert_eq!(req.studios, vec!["A24".to_string()]);
                assert!(req.genres.is_empty(), "got = {:?}, want = []", req.genres);
                Ok(ListItemsResponse::default())
            },
        );
        let a = a.serve().await;
        core.servers.sign_in(&a, "me", "secret").await.unwrap();

        let filter = Filter {
            tag: Some("heist".into()),
            studio: Some("A24".into()),
            ..Default::default()
        };
        core.library_items("sa", "lib", Sort::Name, &filter, 0, 50)
            .await
            .unwrap();
    }

    #[tokio::test]
    async fn search_asks_every_server() {
        let (core, _) = core();
        let a = fake_server("sa", "Home");
        a.unary(
            "/mavio.library.v1.ItemService/ListItems",
            |_, req: ListItemsRequest| {
                assert_eq!(req.search.as_deref(), Some("brazil"));
                // A search spans the whole server, which takes no recursive flag.
                assert_eq!(req.recursive, None);
                assert!(!req.kinds.is_empty());
                Ok(ListItemsResponse {
                    items: vec![movie("a1", "68")],
                    ..Default::default()
                })
            },
        );
        let b = fake_server("sb", "Friend");
        b.unary(
            "/mavio.library.v1.ItemService/ListItems",
            |_, _: ListItemsRequest| Err::<ListItemsResponse, _>(crate::fake::error("internal")),
        );
        let (a, b) = (a.serve().await, b.serve().await);
        core.servers.sign_in(&a, "me", "secret").await.unwrap();
        core.servers.sign_in(&b, "me", "secret").await.unwrap();

        let got = core.search(" brazil ").await.unwrap();
        assert_eq!(got.len(), 2);
        assert_eq!(got[0].cards[0].item_id, "a1", "got = {got:?}");
        assert!(got[1].error.is_some(), "got = {:?}", got[1]);
    }

    #[tokio::test]
    async fn series_details_carry_episodes_the_next_one_and_extras() {
        let (core, _) = core();
        let a = fake_server("sa", "Home");
        let ep = |id: &str, season: &str, s: i32, e: i32, played: bool| Item {
            id: Some(id.into()),
            name: Some(id.into()),
            kind: Some(ItemKind::ITEM_KIND_EPISODE.into()),
            season_id: Some(season.into()),
            parent_index_number: Some(s),
            index_number: Some(e),
            user_data: mavio_api::proto::mavio::user::v1::UserData {
                played: Some(played),
                ..Default::default()
            }
            .into(),
            ..Default::default()
        };
        a.unary(
            "/mavio.library.v1.ItemService/GetItem",
            |_, req: GetItemRequest| {
                Ok(GetItemResponse {
                    item: Item {
                        id: req.id.clone(),
                        name: Some("Show".into()),
                        kind: Some(ItemKind::ITEM_KIND_SERIES.into()),
                        collection_name: Some("Saga".into()),
                        ..Default::default()
                    }
                    .into(),
                    credits: vec![
                        Credit {
                            person: Person {
                                id: Some("p".into()),
                                name: Some("Pat".into()),
                                ..Default::default()
                            }
                            .into(),
                            kind: Some(CreditKind::CREDIT_KIND_ACTOR.into()),
                            role: Some("A".into()),
                            ..Default::default()
                        },
                        Credit {
                            person: Person {
                                id: Some("p".into()),
                                name: Some("Pat".into()),
                                ..Default::default()
                            }
                            .into(),
                            kind: Some(CreditKind::CREDIT_KIND_ACTOR.into()),
                            role: Some("B".into()),
                            ..Default::default()
                        },
                        Credit {
                            person: Person {
                                id: Some("d".into()),
                                name: Some("Dee".into()),
                                ..Default::default()
                            }
                            .into(),
                            kind: Some(CreditKind::CREDIT_KIND_DIRECTOR.into()),
                            ..Default::default()
                        },
                    ],
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.library.v1.ItemService/ListItems",
            move |_, req: ListItemsRequest| {
                let items = if req.owner_id.is_some() {
                    vec![
                        Item {
                            id: Some("t".into()),
                            extra: Some(ExtraKind::EXTRA_KIND_TRAILER.into()),
                            ..Default::default()
                        },
                        Item {
                            id: Some("theme".into()),
                            extra: Some(ExtraKind::EXTRA_KIND_THEME_SONG.into()),
                            ..Default::default()
                        },
                    ]
                } else if req.recursive == Some(true) {
                    vec![
                        ep("e3", "s2", 2, 1, false),
                        ep("e2", "s1", 1, 2, false),
                        ep("e1", "s1", 1, 1, true),
                    ]
                } else if req
                    .kinds
                    .iter()
                    .any(|k| k.as_known() == Some(ItemKind::ITEM_KIND_COLLECTION))
                {
                    vec![Item {
                        id: Some("c".into()),
                        name: Some("saga".into()),
                        ..Default::default()
                    }]
                } else {
                    Vec::new()
                };
                Ok(ListItemsResponse {
                    items,
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.library.v1.ItemService/ListSimilarItems",
            |_, _: ListSimilarItemsRequest| Ok(ListSimilarItemsResponse::default()),
        );
        let addr = a.serve().await;
        core.servers.sign_in(&addr, "me", "secret").await.unwrap();

        let d = core.details("sa", "show").await.unwrap();
        let ids: Vec<&str> = d.episodes.iter().map(|e| e.card.item_id.as_str()).collect();
        assert_eq!(
            ids,
            ["e1", "e2", "e3"],
            "got = {ids:?}, want = the episodes in order"
        );
        assert_eq!(d.next.as_ref().map(|e| e.card.item_id.as_str()), Some("e2"));
        assert_eq!(d.episodes[1].number, Some(2));
        let extras: Vec<_> = d
            .extras
            .iter()
            .map(|e| (e.card.item_id.as_str(), e.kind))
            .collect();
        assert_eq!(extras, [("t", crate::details::ExtraKind::Trailer)]);
        assert_eq!(d.collection.as_ref().map(|c| c.item_id.as_str()), Some("c"));
        let credits: Vec<_> = d.credits.iter().map(|c| (c.name.as_str(), c.job)).collect();
        assert_eq!(
            credits,
            [("Pat", CreditJob::Actor), ("Dee", CreditJob::Director)]
        );
    }

    #[tokio::test]
    async fn an_episodes_details_carry_the_one_after_it_across_seasons() {
        let (core, _) = core();
        let a = fake_server("sa", "Home");
        let ep = |id: &str, season: &str, s: i32, e: i32| Item {
            id: Some(id.into()),
            name: Some(id.into()),
            kind: Some(ItemKind::ITEM_KIND_EPISODE.into()),
            series_id: Some("show".into()),
            season_id: Some(season.into()),
            parent_index_number: Some(s),
            index_number: Some(e),
            ..Default::default()
        };
        a.unary(
            "/mavio.library.v1.ItemService/GetItem",
            move |_, req: GetItemRequest| {
                let item = match req.id.as_deref() {
                    Some("show") => Item {
                        id: Some("show".into()),
                        kind: Some(ItemKind::ITEM_KIND_SERIES.into()),
                        ..Default::default()
                    },
                    Some(id) => ep(id, "s1", 1, 2),
                    None => Item::default(),
                };
                Ok(GetItemResponse {
                    item: item.into(),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.library.v1.ItemService/ListItems",
            move |_, req: ListItemsRequest| {
                let items = if req.recursive == Some(true) {
                    vec![
                        ep("e3", "s2", 2, 1),
                        ep("e2", "s1", 1, 2),
                        ep("e1", "s1", 1, 1),
                    ]
                } else {
                    Vec::new()
                };
                Ok(ListItemsResponse {
                    items,
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.library.v1.ItemService/ListSimilarItems",
            |_, _: ListSimilarItemsRequest| Ok(ListSimilarItemsResponse::default()),
        );
        let addr = a.serve().await;
        core.servers.sign_in(&addr, "me", "secret").await.unwrap();

        let tests = [("e2", Some("e3")), ("e3", None)];
        for (id, want) in tests {
            let d = core.details("sa", id).await.unwrap();
            let got = d.next.as_ref().map(|e| e.card.item_id.as_str());
            assert_eq!(got, want, "{id}: got = {got:?}, want = {want:?}");
        }
        let d = core.details("sa", "e2").await.unwrap();
        let ids: Vec<&str> = d.episodes.iter().map(|e| e.card.item_id.as_str()).collect();
        assert_eq!(
            ids,
            ["e1", "e2"],
            "got = {ids:?}, want = the season's episodes"
        );
    }

    #[tokio::test]
    async fn details_offer_the_other_servers_copies_and_work_offline() {
        let (core, _) = core();
        let a = fake_server("sa", "Home");
        a.unary(
            "/mavio.library.v1.ItemService/GetItem",
            |_, req: GetItemRequest| {
                let stream = |index, kind: StreamKind, codec: &str| MediaStream {
                    index: Some(index),
                    kind: Some(kind.into()),
                    codec: Some(codec.into()),
                    ..Default::default()
                };
                Ok(GetItemResponse {
                    item: movie(req.id.as_deref().unwrap(), "68").into(),
                    media_sources: vec![MediaSource {
                        id: Some("ms1".into()),
                        container: Some("mkv".into()),
                        streams: vec![
                            stream(0, StreamKind::STREAM_KIND_VIDEO, "hevc"),
                            MediaStream {
                                language: Some("jpn".into()),
                                channels: Some(6),
                                ..stream(1, StreamKind::STREAM_KIND_AUDIO, "ac3")
                            },
                        ],
                        ..Default::default()
                    }],
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.library.v1.ItemService/ListItems",
            |_, _: ListItemsRequest| Ok(ListItemsResponse::default()),
        )
        .unary(
            "/mavio.library.v1.ItemService/ListSimilarItems",
            |_, _: ListSimilarItemsRequest| {
                Err::<ListSimilarItemsResponse, _>(crate::fake::error("internal"))
            },
        );
        let b = fake_server("sb", "Friend");
        b.unary(
            "/mavio.library.v1.ItemService/ListItems",
            |_, req: ListItemsRequest| {
                assert_eq!(req.external_ids.get("tmdb").map(String::as_str), Some("68"));
                Ok(ListItemsResponse {
                    items: vec![movie("b-brazil", "68")],
                    ..Default::default()
                })
            },
        );
        let (a_addr, b_addr) = (a.serve().await, b.serve().await);
        core.servers.sign_in(&a_addr, "me", "secret").await.unwrap();
        core.servers.sign_in(&b_addr, "me", "secret").await.unwrap();

        let d = core.details("sa", "a-brazil").await.unwrap();
        assert!(!d.offline);
        assert_eq!(
            d.card.versions,
            [Version {
                server_id: "sb".into(),
                item_id: "b-brazil".into()
            }]
        );
        assert_eq!(d.sources.last().unwrap().server_name, "Friend");
        assert_eq!(d.sources[0].container.as_deref(), Some("mkv"));
        assert_eq!(
            d.sources[0].audio,
            [AudioStream {
                index: 1,
                language: Some("jpn".into()),
                title: None,
                codec: "ac3".into(),
                channels: 6,
                default: false,
            }]
        );

        // a goes away: its details come from the offline store.
        a.unary(
            "/mavio.library.v1.ItemService/GetItem",
            |_, _: GetItemRequest| Err::<GetItemResponse, _>(crate::fake::error("unavailable")),
        );
        let d = core.details("sa", "a-brazil").await.unwrap();
        assert!(d.offline);
        assert_eq!(d.card.title, "a-brazil");
    }

    #[tokio::test]
    async fn switching_users_keeps_the_new_user_and_forgets_the_old_ones_answers() {
        use mavio_api::proto::mavio::auth::v1::{
            ListSwitchableUsersRequest, ListSwitchableUsersResponse, SwitchUserRequest,
            SwitchUserResponse, SwitchableUser as SwitchableUserProto,
        };
        use mavio_api::proto::mavio::user::v1::User;

        let (core, store) = core();
        let a = fake_server("sa", "Home");
        a.unary(
            "/mavio.auth.v1.AuthService/ListSwitchableUsers",
            |_, _: ListSwitchableUsersRequest| {
                Ok(ListSwitchableUsersResponse {
                    users: vec![
                        SwitchableUserProto {
                            id: Some("u".into()),
                            name: Some("me".into()),
                            current: Some(true),
                            ..Default::default()
                        },
                        SwitchableUserProto {
                            id: Some("kid".into()),
                            name: Some("Kid".into()),
                            managed: Some(true),
                            has_pin: Some(true),
                            ..Default::default()
                        },
                    ],
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.auth.v1.AuthService/SwitchUser",
            |_, req: SwitchUserRequest| {
                if req.pin.as_deref() != Some("1234") {
                    return Err(crate::fake::error("permission_denied"));
                }
                Ok(SwitchUserResponse {
                    user: User {
                        id: req.user_id.clone(),
                        name: Some("Kid".into()),
                        ..Default::default()
                    }
                    .into(),
                    ..Default::default()
                })
            },
        );
        let a = a.serve().await;
        core.servers.sign_in(&a, "me", "secret").await.unwrap();
        store.put_cached("sa", "home", &hubs(vec![])).unwrap();

        let users = core.switchable_users("sa").await.unwrap();
        let got: Vec<_> = users
            .iter()
            .map(|u| (u.name.as_str(), u.has_pin, u.current))
            .collect();
        assert_eq!(got, [("me", false, true), ("Kid", true, false)]);

        let wrong = core.switch_user("sa", "kid", Some("0000")).await;
        assert!(
            matches!(wrong, Err(Error::PermissionDenied(_))),
            "got = {wrong:?}, want = PermissionDenied"
        );
        let s = core.switch_user("sa", "kid", Some("1234")).await.unwrap();
        assert_eq!((s.user_id.as_str(), s.user_name.as_str()), ("kid", "Kid"));
        let kept = core.servers.get("sa").unwrap();
        assert_eq!(
            kept.user_name, "Kid",
            "got = {}, want = Kid",
            kept.user_name
        );
        let cached = store.cached::<ListHubsResponse>("sa", "home").unwrap();
        assert!(cached.is_none(), "got = {cached:?}, want = None");
    }

    #[tokio::test]
    async fn playback_preferences_come_from_the_first_server_and_go_to_every_one() {
        use mavio_api::proto::mavio::user::v1::{
            GetCurrentUserResponse, PlaybackPreferences as PlaybackPreferencesProto,
            UpdatePlaybackPreferencesResponse, User,
        };

        use crate::settings::SubtitleMode;

        let (core, _) = core();
        let saved = Arc::new(Mutex::new(Vec::new()));
        let mut addresses = Vec::new();
        for (id, lang) in [("sa", "jpn"), ("sb", "fre")] {
            let f = fake_server(id, id);
            f.unary(
                "/mavio.user.v1.UserService/GetCurrentUser",
                move |_, _: GetCurrentUserRequest| {
                    Ok(GetCurrentUserResponse {
                        user: User {
                            playback_preferences: PlaybackPreferencesProto {
                                audio_languages: vec![lang.into()],
                                ..Default::default()
                            }
                            .into(),
                            ..Default::default()
                        }
                        .into(),
                        ..Default::default()
                    })
                },
            );
            let saved = saved.clone();
            f.unary(
                "/mavio.user.v1.UserService/UpdatePlaybackPreferences",
                move |_, req: UpdatePlaybackPreferencesRequest| {
                    let p = req.playback_preferences.into_option().unwrap_or_default();
                    saved
                        .lock()
                        .unwrap()
                        .push((id, PlaybackPreferences::from_proto(&p)));
                    Ok(UpdatePlaybackPreferencesResponse::default())
                },
            );
            addresses.push(f.serve().await);
        }
        for a in &addresses {
            core.servers.sign_in(a, "me", "secret").await.unwrap();
        }

        let mut p = core.playback_preferences().await.unwrap();
        assert_eq!(
            p.audio_languages,
            ["jpn"],
            "got = {:?}, want = [jpn]",
            p.audio_languages
        );
        p.subtitle_languages = vec!["chi".into()];
        p.subtitle_mode = SubtitleMode::Smart;
        core.set_playback_preferences(&p).await.unwrap();
        let mut got = saved.lock().unwrap().clone();
        got.sort_by_key(|(id, _)| *id);
        assert_eq!(got, [("sa", p.clone()), ("sb", p)]);
    }
}

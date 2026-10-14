//! The core as the apps use it (`docs/clients.md` §5): the servers and
//! their event streams, and the data of each screen, from the servers when
//! they answer and from the offline store when they do not.

use std::collections::HashMap;
use std::sync::{Arc, Mutex};
use std::time::{Duration, SystemTime};

use mavio_api::proto::mavio::home::v1::{GetHubRequest, ListHubsRequest, ListHubsResponse};
use mavio_api::proto::mavio::library::v1::{
    CreditKind, GetItemRequest, GetItemResponse, ImageKind, Item, ItemKind, LibraryKind,
    ListItemsRequest, ListLibrariesRequest, ListLibrariesResponse, ListSimilarItemsRequest,
    SortField, SortSpec, StreamKind,
};
use mavio_api::proto::mavio::session::v1::__buffa::oneof::event::Kind;
use mavio_api::proto::mavio::session::v1::Event;
use mavio_api::proto::mavio::user::v1::UpdateUserDataRequest;
use tokio::task::{JoinHandle, JoinSet};

use crate::connection::Connection;
use crate::downloads::Downloads;
use crate::error::{Error, Result};
use crate::events::{self, EventSink, StreamState};
use crate::home::{self, Hubs, ServerHubs, Version};
use crate::reporting::Reporter;
use crate::servers::{DeviceInfo, QuickConnect, Servers, TokenStore};
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

/// An item's details.
#[derive(Debug, Clone, PartialEq)]
pub struct Details {
    /// The item, as its card.
    pub card: Card,
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
    streams: Mutex<HashMap<String, JoinHandle<()>>>,
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
        for (_, task) in self
            .streams
            .get_mut()
            .unwrap_or_else(|e| e.into_inner())
            .drain()
        {
            task.abort();
        }
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
            streams: Mutex::default(),
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

    /// Follows the event streams of every signed-in server; the app calls
    /// it when it comes to the foreground.
    pub fn start(&self) -> Result<()> {
        for s in self.servers.list()? {
            if self.servers.signed_in(&s.id) {
                self.follow(&s.id);
            }
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
        let mut streams = self.streams.lock().unwrap_or_else(|e| e.into_inner());
        if streams.get(server_id).is_some_and(|t| !t.is_finished()) {
            return;
        }
        let task = events::spawn(
            self.servers.clone(),
            server_id.to_owned(),
            self.relay.clone(),
        );
        streams.insert(server_id.to_owned(), task);
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
                        let r = conn.home().list_hubs(ListHubsRequest::default()).await;
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
            .home()
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
            .home()
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

        let (children, similar, others) = match (&conn, offline) {
            (Ok(conn), false) => tokio::join!(
                self.children(conn, server_id, &item),
                self.similar(conn, server_id, item_id),
                self.other_copies(server_id, &item),
            ),
            _ => (Vec::new(), Vec::new(), Vec::new()),
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
            })
            .collect();
        sources.extend(others);

        let mut credits: Vec<(bool, i32, CreditEntry)> = r
            .credits
            .iter()
            .filter_map(|c| {
                let person = c.person.as_option()?;
                let cast = matches!(
                    c.kind.as_ref().and_then(|k| k.as_known()),
                    Some(CreditKind::CREDIT_KIND_ACTOR | CreditKind::CREDIT_KIND_GUEST_STAR)
                );
                let photo = Item {
                    images: person.images.clone(),
                    ..Default::default()
                };
                Some((
                    !cast,
                    c.order.unwrap_or_default(),
                    CreditEntry {
                        person_id: person.id.clone().unwrap_or_default(),
                        name: person.name.clone().unwrap_or_default(),
                        role: c.role.clone().filter(|r| !r.is_empty()),
                        cast,
                        photo: view::artwork(&base, &photo, ImageKind::IMAGE_KIND_PRIMARY),
                    },
                ))
            })
            .collect();
        credits.sort_by_key(|(crew, order, _)| (*crew, *order));

        Ok(Details {
            server_name: server.name.clone(),
            overview: item.overview.clone().filter(|s| !s.is_empty()),
            tagline: item.tagline.clone().filter(|s| !s.is_empty()),
            genres: item.genres.clone(),
            official_rating: item.official_rating.clone().filter(|s| !s.is_empty()),
            community_rating: item.community_rating.filter(|r| *r > 0.0).map(|r| r as f32),
            premiere: item
                .premiere_date
                .as_option()
                .and_then(|t| SystemTime::try_from(t.clone()).ok()),
            backdrop: view::artwork(&base, &item, ImageKind::IMAGE_KIND_BACKDROP),
            logo: view::artwork(&base, &item, ImageKind::IMAGE_KIND_LOGO),
            position: item
                .user_data
                .as_option()
                .and_then(|u| u.position.as_option())
                .and_then(|d| Duration::try_from(d.clone()).ok()),
            credits: credits.into_iter().map(|(_, _, c)| c).collect(),
            sources,
            children,
            similar,
            offline,
            card,
        })
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
    use mavio_api::proto::mavio::home::v1::Hub;
    use mavio_api::proto::mavio::library::v1::{
        HubKind, ItemKind, ListItemsResponse, ListSimilarItemsResponse, MediaSource, MediaStream,
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
            "/mavio.home.v1.HomeService/ListHubs",
            |_, _: ListHubsRequest| Ok(hubs(vec![movie("a1", "1")])),
        );
        let b = fake_server("sb", "Friend");
        b.unary(
            "/mavio.home.v1.HomeService/ListHubs",
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
}

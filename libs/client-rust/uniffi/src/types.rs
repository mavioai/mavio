//! The core's records and enums, exported as they are.

use std::collections::HashMap;
use std::time::{Duration, SystemTime};

use mavio_core::{
    Artwork, Card, CardKind, CreditEntry, Details, DeviceInfo, DownloadEntry, DownloadState,
    Filter, FoundServer, Libraries, LibraryEntry, LibraryType, Page, PlaybackOptions, Row,
    RowContent, RowKind, SearchResults, ServerInfo, ServerRecord, Sort, Source, StreamState,
    TitleCard, Transfer, Version,
};

#[uniffi::remote(Record)]
pub struct DeviceInfo {
    pub id: String,
    pub name: String,
    pub client: String,
    pub client_version: String,
}

#[uniffi::remote(Record)]
pub struct ServerInfo {
    pub id: String,
    pub name: String,
    pub version: String,
    pub address: String,
    pub setup_required: bool,
}

#[uniffi::remote(Record)]
pub struct FoundServer {
    pub id: String,
    pub name: String,
    pub address: String,
    pub version: String,
}

#[uniffi::remote(Record)]
pub struct ServerRecord {
    pub id: String,
    pub address: String,
    pub name: String,
    pub version: String,
    pub user_id: String,
    pub user_name: String,
}

#[uniffi::remote(Enum)]
pub enum StreamState {
    Offline(String),
    Online,
    SignedOut,
}

#[uniffi::remote(Record)]
pub struct Version {
    pub server_id: String,
    pub item_id: String,
}

#[uniffi::remote(Enum)]
pub enum CardKind {
    Movie,
    Series,
    Season,
    Episode,
    Video,
    Artist,
    Album,
    Track,
    MusicVideo,
    Audiobook,
    Book,
    PhotoAlbum,
    Photo,
    Folder,
    Collection,
    Playlist,
    Other,
}

#[uniffi::remote(Record)]
pub struct Artwork {
    pub url: String,
    pub aspect: f32,
    pub blurhash: Option<String>,
}

#[uniffi::remote(Record)]
pub struct Card {
    pub server_id: String,
    pub item_id: String,
    pub kind: CardKind,
    pub title: String,
    pub subtitle: Option<String>,
    pub year: Option<i32>,
    pub poster: Option<Artwork>,
    pub thumb: Option<Artwork>,
    pub progress: Option<f32>,
    pub played: bool,
    pub favorite: bool,
    pub runtime: Option<Duration>,
    pub versions: Vec<Version>,
}

#[uniffi::remote(Record)]
pub struct TitleCard {
    pub movie: bool,
    pub title: String,
    pub year: Option<i32>,
    pub overview: Option<String>,
    pub poster_url: Option<String>,
    pub backdrop_url: Option<String>,
    pub external_ids: HashMap<String, String>,
    pub requestable: bool,
    pub requested: bool,
    pub available: Vec<Version>,
}

#[uniffi::remote(Enum)]
pub enum RowKind {
    ContinueWatching,
    AvailableRequests,
    RecentlyAdded,
    RecentlyReleased,
    RecentlyAired,
    TopRated,
    TopUnplayed,
    StartWatching,
    Rediscover,
    BecauseYouWatched,
    BecauseYouLike,
    MoreInGenre,
    Starring,
    DirectedBy,
    Trending,
    Server,
}

#[uniffi::remote(Enum)]
pub enum RowContent {
    Items(Vec<Card>),
    Titles(Vec<TitleCard>),
    Loading,
    Unavailable(String),
}

#[uniffi::remote(Record)]
pub struct Row {
    pub id: String,
    pub kind: RowKind,
    pub server_id: Option<String>,
    pub server_name: Option<String>,
    pub hub_id: Option<String>,
    pub library_name: Option<String>,
    pub seed_name: Option<String>,
    pub more: bool,
    pub offline: bool,
    pub content: RowContent,
}

#[uniffi::remote(Record)]
pub struct Page {
    pub cards: Vec<Card>,
    pub total: Option<u32>,
    pub more: bool,
}

#[uniffi::remote(Enum)]
pub enum LibraryType {
    Movies,
    Shows,
    Music,
    MusicVideos,
    HomeVideos,
    Books,
    Photos,
    Collections,
    Playlists,
    Mixed,
}

#[uniffi::remote(Record)]
pub struct LibraryEntry {
    pub server_id: String,
    pub id: String,
    pub name: String,
    pub kind: LibraryType,
    pub image_url: String,
}

#[uniffi::remote(Record)]
pub struct Libraries {
    pub libraries: Vec<LibraryEntry>,
    pub offline: bool,
}

#[uniffi::remote(Enum)]
pub enum Sort {
    Name,
    DateAdded,
    Released,
    Rating,
    LastPlayed,
    Random,
}

#[uniffi::remote(Record)]
pub struct Filter {
    pub played: Option<bool>,
    pub favorite: bool,
    pub genre: Option<String>,
}

#[uniffi::remote(Record)]
pub struct CreditEntry {
    pub person_id: String,
    pub name: String,
    pub role: Option<String>,
    pub cast: bool,
    pub photo: Option<Artwork>,
}

#[uniffi::remote(Record)]
pub struct Source {
    pub server_id: String,
    pub server_name: String,
    pub item_id: String,
    pub media_source_id: Option<String>,
    pub name: Option<String>,
    pub size: Option<u64>,
}

#[uniffi::remote(Record)]
pub struct Details {
    pub card: Card,
    pub server_name: String,
    pub overview: Option<String>,
    pub tagline: Option<String>,
    pub genres: Vec<String>,
    pub official_rating: Option<String>,
    pub community_rating: Option<f32>,
    pub premiere: Option<SystemTime>,
    pub backdrop: Option<Artwork>,
    pub logo: Option<Artwork>,
    pub position: Option<Duration>,
    pub credits: Vec<CreditEntry>,
    pub sources: Vec<Source>,
    pub children: Vec<Card>,
    pub similar: Vec<Card>,
    pub offline: bool,
}

#[uniffi::remote(Record)]
pub struct SearchResults {
    pub server_id: String,
    pub server_name: String,
    pub cards: Vec<Card>,
    pub error: Option<String>,
}

#[uniffi::remote(Enum)]
pub enum DownloadState {
    Queued,
    Running,
    Paused,
    Done,
    Failed,
}

#[uniffi::remote(Record)]
pub struct DownloadEntry {
    pub card: Card,
    pub server_name: String,
    pub state: DownloadState,
    pub file: String,
    pub received: u64,
    pub total: u64,
    pub error: Option<String>,
    pub added: SystemTime,
}

#[uniffi::remote(Record)]
pub struct Transfer {
    pub server_id: String,
    pub item_id: String,
    pub url: String,
}

#[uniffi::remote(Record)]
pub struct PlaybackOptions {
    pub media_source_id: Option<String>,
    pub audio_stream_index: Option<i32>,
    pub subtitle_stream_index: Option<i32>,
    pub max_bitrate: Option<i64>,
    pub start_position: Option<Duration>,
    pub download: bool,
}

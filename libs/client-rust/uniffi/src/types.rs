//! The core's records and enums, exported as they are.

use std::collections::HashMap;
use std::time::{Duration, SystemTime};

use mavio_core::{
    Account, AccountMode, Artwork, AudioStream, Card, CardKind, CreditEntry, Details, DeviceInfo,
    DownloadEntry, DownloadState, Filter, FoundServer, Libraries, LibraryEntry, LibraryType, Page,
    PersonDetails, PlaybackOptions, Row, RowContent, RowKind, SearchResults, ServerInfo,
    ServerRecord, SignInState, Sort, Source, StreamState, TitleCard, Transfer, Version,
};
use mavio_core::{
    AccountSecurity, DeviceSession, Passkey, PlaybackPreferences, RemoteCommand, RemotePlayback,
    SubtitleMode, SwitchableUser, TotpSetup,
};
use mavio_core::{
    CollectionLink, CreditJob, Episode, ExternalLink, Extra, ExtraKind, SeriesLink, SeriesStatus,
    SubtitleStream, VideoRange, VideoStream,
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
    pub mode: AccountMode,
    pub mavio_dev_url: String,
}

#[uniffi::remote(Record)]
pub struct Account {
    pub id: String,
    pub name: String,
    pub api: String,
}

#[uniffi::remote(Enum)]
pub enum AccountMode {
    Unspecified,
    Standalone,
    MavioDev,
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
pub enum SignInState {
    SignedOut,
    Account(Account),
    Standalone,
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
    pub backdrop: Option<Artwork>,
    pub logo: Option<Artwork>,
    pub overview: Option<String>,
    pub genres: Vec<String>,
    pub official_rating: Option<String>,
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
    #[uniffi(default = None)]
    pub tag: Option<String>,
    #[uniffi(default = None)]
    pub studio: Option<String>,
}

#[uniffi::remote(Record)]
pub struct CreditEntry {
    pub person_id: String,
    pub name: String,
    pub role: Option<String>,
    pub cast: bool,
    pub job: CreditJob,
    pub photo: Option<Artwork>,
}

#[uniffi::remote(Enum)]
pub enum CreditJob {
    Actor,
    GuestStar,
    Creator,
    Director,
    Writer,
    Author,
    Producer,
    Composer,
    Conductor,
    Lyricist,
    Artist,
    Narrator,
    Other,
}

#[uniffi::remote(Record)]
pub struct Source {
    pub server_id: String,
    pub server_name: String,
    pub item_id: String,
    pub media_source_id: Option<String>,
    pub name: Option<String>,
    pub size: Option<u64>,
    pub container: Option<String>,
    pub audio: Vec<AudioStream>,
    pub file_name: Option<String>,
    pub video: Option<VideoStream>,
    pub subtitles: Vec<SubtitleStream>,
}

#[uniffi::remote(Enum)]
pub enum VideoRange {
    Sdr,
    Hdr10,
    Hdr10Plus,
    Hlg,
    DolbyVision,
}

#[uniffi::remote(Record)]
pub struct VideoStream {
    pub width: i32,
    pub height: i32,
    pub codec: String,
    pub range: VideoRange,
}

#[uniffi::remote(Record)]
pub struct SubtitleStream {
    pub index: i32,
    pub language: Option<String>,
    pub title: Option<String>,
    pub codec: String,
    pub forced: bool,
    pub hearing_impaired: bool,
    pub external: bool,
}

#[uniffi::remote(Record)]
pub struct AudioStream {
    pub index: i32,
    pub language: Option<String>,
    pub title: Option<String>,
    pub codec: String,
    pub channels: i32,
    pub default: bool,
}

#[uniffi::remote(Record)]
pub struct Details {
    pub card: Card,
    pub library_id: Option<String>,
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
    pub season_number: Option<i32>,
    pub episode_number: Option<i32>,
    pub season_name: Option<String>,
    pub series: Option<SeriesLink>,
    pub series_status: Option<SeriesStatus>,
    pub end_date: Option<SystemTime>,
    pub critic_rating: Option<f32>,
    pub tags: Vec<String>,
    pub studios: Vec<String>,
    pub countries: Vec<String>,
    pub original_title: Option<String>,
    pub original_language: Option<String>,
    pub digital_release: Option<SystemTime>,
    pub physical_release: Option<SystemTime>,
    pub date_added: Option<SystemTime>,
    pub links: Vec<ExternalLink>,
    pub remote_trailer: Option<String>,
    pub episodes: Vec<Episode>,
    pub next: Option<Episode>,
    pub extras: Vec<Extra>,
    pub collection: Option<CollectionLink>,
}

#[uniffi::remote(Record)]
pub struct PersonDetails {
    pub name: String,
    pub overview: Option<String>,
    pub born: Option<SystemTime>,
    pub died: Option<SystemTime>,
    pub birth_place: Option<String>,
    pub photo: Option<Artwork>,
    pub links: Vec<ExternalLink>,
    pub items: Vec<Card>,
}

#[uniffi::remote(Enum)]
pub enum SeriesStatus {
    Continuing,
    Ended,
    Unreleased,
}

#[uniffi::remote(Record)]
pub struct SeriesLink {
    pub item_id: String,
    pub name: String,
    pub logo: Option<Artwork>,
}

#[uniffi::remote(Record)]
pub struct CollectionLink {
    pub item_id: String,
    pub name: String,
    pub backdrop: Option<Artwork>,
}

#[uniffi::remote(Record)]
pub struct ExternalLink {
    pub name: String,
    pub url: String,
}

#[uniffi::remote(Enum)]
pub enum ExtraKind {
    Trailer,
    Clip,
    BehindTheScenes,
    DeletedScene,
    Interview,
    Scene,
    Sample,
    Featurette,
    Short,
    Other,
}

#[uniffi::remote(Record)]
pub struct Extra {
    pub card: Card,
    pub kind: ExtraKind,
}

#[uniffi::remote(Record)]
pub struct Episode {
    pub card: Card,
    pub season_id: Option<String>,
    pub season: Option<i32>,
    pub number: Option<i32>,
    pub overview: Option<String>,
    pub premiere: Option<SystemTime>,
    pub position: Option<Duration>,
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

#[uniffi::remote(Enum)]
pub enum SubtitleMode {
    Default,
    Always,
    Foreign,
    Forced,
    Never,
    Smart,
}

#[uniffi::remote(Record)]
pub struct PlaybackPreferences {
    pub audio_languages: Vec<String>,
    pub subtitle_languages: Vec<String>,
    pub subtitle_mode: SubtitleMode,
    pub play_default_audio_track: bool,
}

#[uniffi::remote(Record)]
pub struct SwitchableUser {
    pub id: String,
    pub name: String,
    pub admin: bool,
    pub managed: bool,
    pub has_pin: bool,
    pub current: bool,
}

#[uniffi::remote(Record)]
pub struct AccountSecurity {
    pub managed: bool,
    pub has_password: bool,
    pub passkeys: Vec<Passkey>,
    pub has_totp: bool,
    pub has_pin: bool,
}

#[uniffi::remote(Record)]
pub struct Passkey {
    pub id: String,
    pub name: String,
    pub host: String,
    pub created: Option<SystemTime>,
}

#[uniffi::remote(Record)]
pub struct TotpSetup {
    pub secret: String,
    pub uri: String,
}

#[uniffi::remote(Record)]
pub struct DeviceSession {
    pub id: String,
    pub device_name: String,
    pub client: String,
    pub online: bool,
    pub current: bool,
    pub last_seen: Option<SystemTime>,
    pub playing: Option<RemotePlayback>,
}

#[uniffi::remote(Record)]
pub struct RemotePlayback {
    pub card: Card,
    pub position: Duration,
    pub paused: bool,
}

#[uniffi::remote(Enum)]
pub enum RemoteCommand {
    Play {
        item_ids: Vec<String>,
        start_index: u32,
        start_position: Option<Duration>,
    },
    Pause,
    Unpause,
    Stop,
    Next,
    Previous,
    Seek {
        position: Duration,
    },
    Volume {
        level: Option<u32>,
        muted: Option<bool>,
    },
    Message {
        text: String,
    },
}

//! What the screens draw (`docs/clients.md` §5.1): plain records made from
//! the servers' messages, with image URLs resolved, so that every app draws
//! the same thing and none reads a message itself.

use std::time::Duration;

use mavio_api::proto::mavio::acquisition::v1::{DiscoveredTitle, MediaKind};
use mavio_api::proto::mavio::library::v1::{HubKind, ImageKind, Item, ItemKind};

use crate::connection::Connection;
use crate::home::{self, Version};

/// What a card stands for.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CardKind {
    /// A movie.
    Movie,
    /// A series.
    Series,
    /// A season of a series.
    Season,
    /// An episode of a series.
    Episode,
    /// A video that is none of the others.
    Video,
    /// A music artist.
    Artist,
    /// A music album.
    Album,
    /// A music track.
    Track,
    /// A music video.
    MusicVideo,
    /// An audiobook.
    Audiobook,
    /// A book.
    Book,
    /// A photo album.
    PhotoAlbum,
    /// A photo.
    Photo,
    /// A folder.
    Folder,
    /// A collection.
    Collection,
    /// A playlist.
    Playlist,
    /// Unknown to this version of the core.
    Other,
}

impl From<Option<ItemKind>> for CardKind {
    fn from(k: Option<ItemKind>) -> Self {
        match k {
            Some(ItemKind::ITEM_KIND_MOVIE) => CardKind::Movie,
            Some(ItemKind::ITEM_KIND_SERIES) => CardKind::Series,
            Some(ItemKind::ITEM_KIND_SEASON) => CardKind::Season,
            Some(ItemKind::ITEM_KIND_EPISODE) => CardKind::Episode,
            Some(ItemKind::ITEM_KIND_VIDEO) => CardKind::Video,
            Some(ItemKind::ITEM_KIND_MUSIC_ARTIST) => CardKind::Artist,
            Some(ItemKind::ITEM_KIND_MUSIC_ALBUM) => CardKind::Album,
            Some(ItemKind::ITEM_KIND_TRACK) => CardKind::Track,
            Some(ItemKind::ITEM_KIND_MUSIC_VIDEO) => CardKind::MusicVideo,
            Some(ItemKind::ITEM_KIND_AUDIOBOOK) => CardKind::Audiobook,
            Some(ItemKind::ITEM_KIND_BOOK) => CardKind::Book,
            Some(ItemKind::ITEM_KIND_PHOTO_ALBUM) => CardKind::PhotoAlbum,
            Some(ItemKind::ITEM_KIND_PHOTO) => CardKind::Photo,
            Some(ItemKind::ITEM_KIND_FOLDER) => CardKind::Folder,
            Some(ItemKind::ITEM_KIND_COLLECTION) => CardKind::Collection,
            Some(ItemKind::ITEM_KIND_PLAYLIST) => CardKind::Playlist,
            _ => CardKind::Other,
        }
    }
}

/// An image of an item.
#[derive(Debug, Clone, PartialEq)]
pub struct Artwork {
    /// The image, to which the app adds `?maxWidth=` and `maxHeight=` for
    /// the size it draws.
    pub url: String,
    /// Width over height; zero when unknown.
    pub aspect: f32,
    /// A BlurHash to draw while it loads.
    pub blurhash: Option<String>,
}

/// An item as a card in a row or grid.
#[derive(Debug, Clone, PartialEq)]
pub struct Card {
    /// The server of the item.
    pub server_id: String,
    /// The item on that server.
    pub item_id: String,
    /// What it is.
    pub kind: CardKind,
    /// Its name; an episode's own name.
    pub title: String,
    /// What tells it apart: an episode's series with its numbers, an
    /// album's artists.
    pub subtitle: Option<String>,
    /// Its year.
    pub year: Option<i32>,
    /// The poster, or a square cover.
    pub poster: Option<Artwork>,
    /// A wide image: the thumbnail, else the first backdrop.
    pub thumb: Option<Artwork>,
    /// How far the user watched it, from 0 to 1.
    pub progress: Option<f32>,
    /// Whether the user played it.
    pub played: bool,
    /// Whether the user marked it a favorite.
    pub favorite: bool,
    /// Its length.
    pub runtime: Option<Duration>,
    /// The same title on the user's other servers.
    pub versions: Vec<Version>,
}

fn non_empty(s: &Option<String>) -> Option<String> {
    s.clone().filter(|s| !s.is_empty())
}

/// The first image of a kind.
pub fn artwork(conn: &Connection, item: &Item, kind: ImageKind) -> Option<Artwork> {
    let image = item
        .images
        .iter()
        .filter(|i| i.kind.as_ref().and_then(|k| k.as_known()) == Some(kind))
        .min_by_key(|i| i.index.unwrap_or_default())?;
    let id = image.id.as_deref().filter(|id| !id.is_empty())?;
    let (w, h) = (image.width.unwrap_or(0), image.height.unwrap_or(0));
    Some(Artwork {
        url: conn.url(&format!("images/{id}")),
        aspect: if h > 0 { w as f32 / h as f32 } else { 0.0 },
        blurhash: non_empty(&image.blurhash),
    })
}

/// The subtitle of a card.
fn subtitle(item: &Item) -> Option<String> {
    match CardKind::from(item.kind.as_ref().and_then(|k| k.as_known())) {
        CardKind::Episode => {
            let numbers = match (item.parent_index_number, item.index_number) {
                (Some(s), Some(e)) => Some(format!("S{s}:E{e}")),
                (None, Some(e)) => Some(format!("E{e}")),
                _ => None,
            };
            let parts: Vec<String> = [non_empty(&item.series_name), numbers]
                .into_iter()
                .flatten()
                .collect();
            (!parts.is_empty()).then(|| parts.join(" · "))
        }
        CardKind::Season => non_empty(&item.series_name),
        CardKind::Album | CardKind::Track => {
            let artists = if item.album_artists.is_empty() {
                &item.artists
            } else {
                &item.album_artists
            };
            (!artists.is_empty()).then(|| artists.join(", "))
        }
        _ => None,
    }
}

impl Card {
    /// The card of an item of a server.
    pub fn new(conn: &Connection, server_id: &str, item: &Item) -> Self {
        let user = item.user_data.as_option();
        let runtime = item
            .runtime
            .as_option()
            .and_then(|d| Duration::try_from(d.clone()).ok())
            .filter(|d| !d.is_zero());
        let position = user
            .and_then(|u| u.position.as_option())
            .and_then(|d| Duration::try_from(d.clone()).ok());
        let progress = match (position, runtime) {
            (Some(p), Some(r)) if !p.is_zero() => {
                Some((p.as_secs_f32() / r.as_secs_f32()).min(1.0))
            }
            _ => None,
        };
        Card {
            server_id: server_id.to_owned(),
            item_id: item.id.clone().unwrap_or_default(),
            kind: item.kind.as_ref().and_then(|k| k.as_known()).into(),
            title: item.name.clone().unwrap_or_default(),
            subtitle: subtitle(item),
            year: item.production_year.filter(|y| *y > 0),
            poster: artwork(conn, item, ImageKind::IMAGE_KIND_PRIMARY),
            thumb: artwork(conn, item, ImageKind::IMAGE_KIND_THUMB)
                .or_else(|| artwork(conn, item, ImageKind::IMAGE_KIND_BACKDROP)),
            progress,
            played: user.and_then(|u| u.played).unwrap_or_default(),
            favorite: user.and_then(|u| u.favorite).unwrap_or_default(),
            runtime,
            versions: Vec::new(),
        }
    }
}

/// A title discovery offers, which no server or only some have.
#[derive(Debug, Clone, PartialEq)]
pub struct TitleCard {
    /// Whether it is a movie; else a series.
    pub movie: bool,
    /// Its name.
    pub title: String,
    /// Its year.
    pub year: Option<i32>,
    /// What it is about.
    pub overview: Option<String>,
    /// The poster, from the metadata provider.
    pub poster_url: Option<String>,
    /// A backdrop, from the metadata provider.
    pub backdrop_url: Option<String>,
    /// Its external IDs, which name it to requests.
    pub external_ids: Vec<(String, String)>,
    /// Whether the user may request it from the section's server.
    pub requestable: bool,
    /// Whether the section's server has requested it.
    pub requested: bool,
    /// The servers that have it, whose items play instead.
    pub available: Vec<Version>,
}

impl TitleCard {
    fn new(title: &DiscoveredTitle, available: Vec<Version>) -> Self {
        let mut external_ids: Vec<(String, String)> = title
            .external_ids
            .iter()
            .map(|(k, v)| (k.clone(), v.clone()))
            .collect();
        external_ids.sort();
        TitleCard {
            movie: title.media_kind.as_ref().and_then(|k| k.as_known())
                == Some(MediaKind::MEDIA_KIND_MOVIE),
            title: title.name.clone().unwrap_or_default(),
            year: title.year.filter(|y| *y > 0),
            overview: non_empty(&title.overview),
            poster_url: non_empty(&title.poster_url),
            backdrop_url: non_empty(&title.backdrop_url),
            external_ids,
            requestable: title.requestable.unwrap_or_default(),
            requested: title.request_id.as_deref().is_some_and(|r| !r.is_empty()),
            available,
        }
    }
}

/// The kind of a row of Home or of a library's Recommended tab, which
/// names it.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum RowKind {
    /// Items being watched and next episodes.
    ContinueWatching,
    /// Requests that became available.
    AvailableRequests,
    /// Recently added to a library.
    RecentlyAdded,
    /// Recently released.
    RecentlyReleased,
    /// Episodes recently aired.
    RecentlyAired,
    /// Rated highest.
    TopRated,
    /// Rated highest, not played.
    TopUnplayed,
    /// Series not begun.
    StartWatching,
    /// Played long ago.
    Rediscover,
    /// Like an item the user played; the seed names it.
    BecauseYouWatched,
    /// Like an item the user favored; the seed names it.
    BecauseYouLike,
    /// More in a genre; the seed names it.
    MoreInGenre,
    /// Starring a person; the seed names them.
    Starring,
    /// Directed by a person; the seed names them.
    DirectedBy,
    /// Titles discovery offers.
    Trending,
    /// A server's place while it loads or is unreachable.
    Server,
}

impl From<HubKind> for RowKind {
    fn from(k: HubKind) -> Self {
        match k {
            HubKind::HUB_KIND_CONTINUE_WATCHING => RowKind::ContinueWatching,
            HubKind::HUB_KIND_AVAILABLE_REQUESTS => RowKind::AvailableRequests,
            HubKind::HUB_KIND_RECENTLY_ADDED => RowKind::RecentlyAdded,
            HubKind::HUB_KIND_RECENTLY_RELEASED => RowKind::RecentlyReleased,
            HubKind::HUB_KIND_RECENTLY_AIRED => RowKind::RecentlyAired,
            HubKind::HUB_KIND_TOP_RATED => RowKind::TopRated,
            HubKind::HUB_KIND_TOP_UNPLAYED => RowKind::TopUnplayed,
            HubKind::HUB_KIND_START_WATCHING => RowKind::StartWatching,
            HubKind::HUB_KIND_REDISCOVER => RowKind::Rediscover,
            HubKind::HUB_KIND_BECAUSE_YOU_WATCHED => RowKind::BecauseYouWatched,
            HubKind::HUB_KIND_BECAUSE_YOU_LIKE => RowKind::BecauseYouLike,
            HubKind::HUB_KIND_MORE_IN_GENRE => RowKind::MoreInGenre,
            HubKind::HUB_KIND_STARRING => RowKind::Starring,
            HubKind::HUB_KIND_DIRECTED_BY => RowKind::DirectedBy,
            HubKind::HUB_KIND_TRENDING => RowKind::Trending,
            HubKind::HUB_KIND_UNSPECIFIED => RowKind::Server,
        }
    }
}

/// What a row shows.
#[derive(Debug, Clone, PartialEq)]
pub enum RowContent {
    /// Items.
    Items(Vec<Card>),
    /// Titles of discovery.
    Titles(Vec<TitleCard>),
    /// Its server has not answered yet.
    Loading,
    /// Its server is unreachable, with why.
    Unavailable(String),
}

/// A row of Home or of a library's Recommended tab.
#[derive(Debug, Clone, PartialEq)]
pub struct Row {
    /// Stable across updates, to keep the row's scroll position.
    pub id: String,
    /// What it is.
    pub kind: RowKind,
    /// Its server; unset for Continue Watching across servers.
    pub server_id: Option<String>,
    /// That server's name, shown when several servers are signed in.
    pub server_name: Option<String>,
    /// The hub on its server, which `Core::hub` pages.
    pub hub_id: Option<String>,
    /// Its library.
    pub library_name: Option<String>,
    /// What a seeded row is drawn from: an item, a genre, a person.
    pub seed_name: Option<String>,
    /// Whether it has more than shown.
    pub more: bool,
    /// Whether it comes from the offline store.
    pub offline: bool,
    /// What it shows.
    pub content: RowContent,
}

/// The rows of merged sections, with the connections of their servers.
pub(crate) fn rows(
    sections: Vec<home::Section>,
    conn: impl Fn(&str) -> Option<Connection>,
) -> Vec<Row> {
    sections
        .into_iter()
        .map(|s| {
            let content = match s.content {
                home::Content::Items(entries) => RowContent::Items(
                    entries
                        .into_iter()
                        .filter_map(|e| {
                            let mut card = Card::new(&conn(&e.server_id)?, &e.server_id, &e.item);
                            card.versions = e.versions;
                            Some(card)
                        })
                        .collect(),
                ),
                home::Content::Titles(titles) => RowContent::Titles(
                    titles
                        .into_iter()
                        .map(|t| TitleCard::new(&t.title, t.available))
                        .collect(),
                ),
                home::Content::Loading => RowContent::Loading,
                home::Content::Unavailable(why) => RowContent::Unavailable(why),
            };
            Row {
                id: s.id,
                kind: s.kind.into(),
                server_id: s.server_id,
                server_name: s.server_name,
                hub_id: s.hub_id,
                library_name: s.library_name,
                seed_name: s.seed_name,
                more: s.more,
                offline: s.offline,
                content,
            }
        })
        .collect()
}

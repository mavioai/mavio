//! The core's records as messages of `mavio.capi.v1`, and the requests'
//! options as the core's.

use std::time::{Duration, SystemTime, UNIX_EPOCH};

use mavio_api::proto::mavio::playback::v1::{PlayMethod, SubtitleMethod};
use mavio_core::{
    Artwork, Card, CardKind, Connection, Details, Filter, Libraries, LibraryType, Page, Playback,
    PlaybackOptions, Row, RowContent, RowKind, SearchResults, Sort, TitleCard, Version,
};

use crate::proto::mavio::capi::v1 as pb;
use crate::proto::mavio::capi::v1::__buffa::oneof::row::Content;

fn millis(d: Duration) -> i64 {
    i64::try_from(d.as_millis()).unwrap_or(i64::MAX)
}

fn since_epoch(t: SystemTime) -> i64 {
    t.duration_since(UNIX_EPOCH).map_or(0, millis)
}

/// A duration of milliseconds, negative ones as zero.
pub(crate) fn duration(ms: i64) -> Duration {
    Duration::from_millis(u64::try_from(ms).unwrap_or_default())
}

pub(crate) fn time(t: SystemTime) -> i64 {
    since_epoch(t)
}

fn artwork(a: Artwork) -> pb::Artwork {
    pb::Artwork {
        url: a.url,
        aspect: a.aspect,
        blurhash: a.blurhash.unwrap_or_default(),
        ..Default::default()
    }
}

fn card_kind(k: CardKind) -> pb::CardKind {
    use pb::CardKind as K;
    match k {
        CardKind::Movie => K::CARD_KIND_MOVIE,
        CardKind::Series => K::CARD_KIND_SERIES,
        CardKind::Season => K::CARD_KIND_SEASON,
        CardKind::Episode => K::CARD_KIND_EPISODE,
        CardKind::Video => K::CARD_KIND_VIDEO,
        CardKind::Artist => K::CARD_KIND_ARTIST,
        CardKind::Album => K::CARD_KIND_ALBUM,
        CardKind::Track => K::CARD_KIND_TRACK,
        CardKind::MusicVideo => K::CARD_KIND_MUSIC_VIDEO,
        CardKind::Audiobook => K::CARD_KIND_AUDIOBOOK,
        CardKind::Book => K::CARD_KIND_BOOK,
        CardKind::PhotoAlbum => K::CARD_KIND_PHOTO_ALBUM,
        CardKind::Photo => K::CARD_KIND_PHOTO,
        CardKind::Folder => K::CARD_KIND_FOLDER,
        CardKind::Collection => K::CARD_KIND_COLLECTION,
        CardKind::Playlist => K::CARD_KIND_PLAYLIST,
        CardKind::Other => K::CARD_KIND_UNSPECIFIED,
    }
}

fn version(v: Version) -> pb::Version {
    pb::Version {
        server_id: v.server_id,
        item_id: v.item_id,
        ..Default::default()
    }
}

pub(crate) fn card(c: Card) -> pb::Card {
    pb::Card {
        server_id: c.server_id,
        item_id: c.item_id,
        kind: card_kind(c.kind).into(),
        title: c.title,
        subtitle: c.subtitle.unwrap_or_default(),
        year: c.year.unwrap_or_default(),
        poster: c.poster.map(artwork).into(),
        thumb: c.thumb.map(artwork).into(),
        progress: c.progress,
        played: c.played,
        favorite: c.favorite,
        runtime_ms: c.runtime.map(millis).unwrap_or_default(),
        versions: c.versions.into_iter().map(version).collect(),
        backdrop: c.backdrop.map(artwork).into(),
        logo: c.logo.map(artwork).into(),
        overview: c.overview.unwrap_or_default(),
        genres: c.genres,
        official_rating: c.official_rating.unwrap_or_default(),
        ..Default::default()
    }
}

fn cards(cards: Vec<Card>) -> Vec<pb::Card> {
    cards.into_iter().map(card).collect()
}

fn title_card(t: TitleCard) -> pb::TitleCard {
    pb::TitleCard {
        movie: t.movie,
        title: t.title,
        year: t.year.unwrap_or_default(),
        overview: t.overview.unwrap_or_default(),
        poster_url: t.poster_url.unwrap_or_default(),
        backdrop_url: t.backdrop_url.unwrap_or_default(),
        external_ids: t.external_ids.into_iter().collect(),
        requestable: t.requestable,
        requested: t.requested,
        available: t.available.into_iter().map(version).collect(),
        ..Default::default()
    }
}

fn row_kind(k: RowKind) -> pb::RowKind {
    use pb::RowKind as K;
    match k {
        RowKind::ContinueWatching => K::ROW_KIND_CONTINUE_WATCHING,
        RowKind::AvailableRequests => K::ROW_KIND_AVAILABLE_REQUESTS,
        RowKind::RecentlyAdded => K::ROW_KIND_RECENTLY_ADDED,
        RowKind::RecentlyReleased => K::ROW_KIND_RECENTLY_RELEASED,
        RowKind::RecentlyAired => K::ROW_KIND_RECENTLY_AIRED,
        RowKind::TopRated => K::ROW_KIND_TOP_RATED,
        RowKind::TopUnplayed => K::ROW_KIND_TOP_UNPLAYED,
        RowKind::StartWatching => K::ROW_KIND_START_WATCHING,
        RowKind::Rediscover => K::ROW_KIND_REDISCOVER,
        RowKind::BecauseYouWatched => K::ROW_KIND_BECAUSE_YOU_WATCHED,
        RowKind::BecauseYouLike => K::ROW_KIND_BECAUSE_YOU_LIKE,
        RowKind::MoreInGenre => K::ROW_KIND_MORE_IN_GENRE,
        RowKind::Starring => K::ROW_KIND_STARRING,
        RowKind::DirectedBy => K::ROW_KIND_DIRECTED_BY,
        RowKind::Trending => K::ROW_KIND_TRENDING,
        RowKind::Server => K::ROW_KIND_UNSPECIFIED,
    }
}

fn row(r: Row) -> pb::Row {
    let content = match r.content {
        RowContent::Items(items) => Content::from(pb::Cards {
            cards: cards(items),
            ..Default::default()
        }),
        RowContent::Titles(titles) => Content::from(pb::TitleCards {
            titles: titles.into_iter().map(title_card).collect(),
            ..Default::default()
        }),
        RowContent::Loading => Content::from(pb::Loading::default()),
        RowContent::Unavailable(why) => Content::Unavailable(why),
    };
    pb::Row {
        id: r.id,
        kind: row_kind(r.kind).into(),
        server_id: r.server_id.unwrap_or_default(),
        server_name: r.server_name.unwrap_or_default(),
        hub_id: r.hub_id.unwrap_or_default(),
        library_name: r.library_name.unwrap_or_default(),
        seed_name: r.seed_name.unwrap_or_default(),
        more: r.more,
        offline: r.offline,
        content: Some(content),
        ..Default::default()
    }
}

pub(crate) fn rows(rows: Vec<Row>) -> pb::Rows {
    pb::Rows {
        rows: rows.into_iter().map(row).collect(),
        ..Default::default()
    }
}

pub(crate) fn page(p: Page) -> pb::Page {
    pb::Page {
        cards: cards(p.cards),
        total: p.total,
        more: p.more,
        ..Default::default()
    }
}

fn library_type(k: LibraryType) -> pb::LibraryType {
    use pb::LibraryType as K;
    match k {
        LibraryType::Movies => K::LIBRARY_TYPE_MOVIES,
        LibraryType::Shows => K::LIBRARY_TYPE_SHOWS,
        LibraryType::Music => K::LIBRARY_TYPE_MUSIC,
        LibraryType::MusicVideos => K::LIBRARY_TYPE_MUSIC_VIDEOS,
        LibraryType::HomeVideos => K::LIBRARY_TYPE_HOME_VIDEOS,
        LibraryType::Books => K::LIBRARY_TYPE_BOOKS,
        LibraryType::Photos => K::LIBRARY_TYPE_PHOTOS,
        LibraryType::Collections => K::LIBRARY_TYPE_COLLECTIONS,
        LibraryType::Playlists => K::LIBRARY_TYPE_PLAYLISTS,
        LibraryType::Mixed => K::LIBRARY_TYPE_UNSPECIFIED,
    }
}

pub(crate) fn libraries(l: Libraries) -> pb::Libraries {
    pb::Libraries {
        libraries: l
            .libraries
            .into_iter()
            .map(|l| pb::Library {
                server_id: l.server_id,
                id: l.id,
                name: l.name,
                kind: library_type(l.kind).into(),
                image_url: l.image_url,
                ..Default::default()
            })
            .collect(),
        offline: l.offline,
        ..Default::default()
    }
}

pub(crate) fn details(d: Details) -> pb::Details {
    pb::Details {
        card: card(d.card).into(),
        server_name: d.server_name,
        overview: d.overview.unwrap_or_default(),
        tagline: d.tagline.unwrap_or_default(),
        genres: d.genres,
        official_rating: d.official_rating.unwrap_or_default(),
        community_rating: d.community_rating,
        premiere_ms: d.premiere.map(since_epoch),
        backdrop: d.backdrop.map(artwork).into(),
        logo: d.logo.map(artwork).into(),
        position_ms: d.position.map(millis).unwrap_or_default(),
        credits: d
            .credits
            .into_iter()
            .map(|c| pb::Credit {
                person_id: c.person_id,
                name: c.name,
                role: c.role.unwrap_or_default(),
                cast: c.cast,
                photo: c.photo.map(artwork).into(),
                ..Default::default()
            })
            .collect(),
        sources: d
            .sources
            .into_iter()
            .map(|s| pb::Source {
                server_id: s.server_id,
                server_name: s.server_name,
                item_id: s.item_id,
                media_source_id: s.media_source_id.unwrap_or_default(),
                name: s.name.unwrap_or_default(),
                size: s.size.unwrap_or_default(),
                container: s.container.unwrap_or_default(),
                audio: s
                    .audio
                    .into_iter()
                    .map(|a| pb::AudioStream {
                        index: a.index,
                        language: a.language.unwrap_or_default(),
                        title: a.title.unwrap_or_default(),
                        codec: a.codec,
                        channels: a.channels,
                        default: a.default,
                        ..Default::default()
                    })
                    .collect(),
                chapters: s
                    .chapters
                    .into_iter()
                    .map(|c| pb::Chapter {
                        start_ms: millis(c.start),
                        name: c.name.unwrap_or_default(),
                        ..Default::default()
                    })
                    .collect(),
                ..Default::default()
            })
            .collect(),
        trickplay: d
            .trickplay
            .into_iter()
            .map(|t| pb::Trickplay {
                width: t.width,
                height: t.height,
                columns: t.columns,
                rows: t.rows,
                count: t.count,
                interval_ms: millis(t.interval),
                url: t.url,
                ..Default::default()
            })
            .collect(),
        children: cards(d.children),
        similar: cards(d.similar),
        offline: d.offline,
        ..Default::default()
    }
}

pub(crate) fn search_results(results: Vec<SearchResults>) -> pb::SearchResults {
    pb::SearchResults {
        servers: results
            .into_iter()
            .map(|r| pb::ServerResults {
                server_id: r.server_id,
                server_name: r.server_name,
                cards: cards(r.cards),
                error: r.error.unwrap_or_default(),
                ..Default::default()
            })
            .collect(),
        ..Default::default()
    }
}

pub(crate) fn sort(s: Option<pb::Sort>) -> Sort {
    match s {
        Some(pb::Sort::SORT_DATE_ADDED) => Sort::DateAdded,
        Some(pb::Sort::SORT_RELEASED) => Sort::Released,
        Some(pb::Sort::SORT_RATING) => Sort::Rating,
        Some(pb::Sort::SORT_LAST_PLAYED) => Sort::LastPlayed,
        Some(pb::Sort::SORT_RANDOM) => Sort::Random,
        _ => Sort::Name,
    }
}

pub(crate) fn filter(f: Option<&pb::Filter>) -> Filter {
    let Some(f) = f else {
        return Filter::default();
    };
    Filter {
        played: f.played,
        favorite: f.favorite,
        genre: Some(f.genre.clone()).filter(|g| !g.is_empty()),
        tag: Some(f.tag.clone()).filter(|t| !t.is_empty()),
        studio: Some(f.studio.clone()).filter(|s| !s.is_empty()),
    }
}

pub(crate) fn playback_options(o: Option<&pb::PlaybackOptions>) -> PlaybackOptions {
    let Some(o) = o else {
        return PlaybackOptions::default();
    };
    PlaybackOptions {
        media_source_id: Some(o.media_source_id.clone()).filter(|s| !s.is_empty()),
        audio_stream_index: o.audio_stream_index,
        subtitle_stream_index: o.subtitle_stream_index,
        max_bitrate: Some(o.max_bitrate).filter(|b| *b > 0),
        start_position: o.start_position_ms.map(duration),
        download: false,
    }
}

/// The playback of session, its subtitles' URLs resolved against conn.
pub(crate) fn playback(session: u64, conn: &Connection, p: &Playback) -> pb::Playback {
    let r = &p.response;
    let text = |s: &Option<String>| s.clone().unwrap_or_default();
    pb::Playback {
        session,
        url: p.url.clone(),
        kind: match r.method.as_ref().and_then(|m| m.as_known()) {
            Some(PlayMethod::PLAY_METHOD_TRANSCODE) => pb::PlayKind::PLAY_KIND_TRANSCODE,
            Some(PlayMethod::PLAY_METHOD_DIRECT_STREAM) => pb::PlayKind::PLAY_KIND_DIRECT_STREAM,
            _ => pb::PlayKind::PLAY_KIND_UNSPECIFIED,
        }
        .into(),
        media_source_id: text(&r.media_source_id),
        start_position_ms: r
            .start_position
            .as_option()
            .and_then(|d| Duration::try_from(d.clone()).ok())
            .map(millis),
        audio_stream_index: r.audio_stream_index,
        subtitle_stream_index: r.subtitle_stream_index,
        subtitles: r
            .subtitles
            .iter()
            .map(|s| {
                let external = s.method.as_ref().and_then(|m| m.as_known())
                    == Some(SubtitleMethod::SUBTITLE_METHOD_EXTERNAL);
                pb::SubtitleTrack {
                    stream_index: s.stream_index.unwrap_or_default(),
                    language: text(&s.language),
                    title: text(&s.title),
                    default: s.default.unwrap_or_default(),
                    forced: s.forced.unwrap_or_default(),
                    format: text(&s.format),
                    url: s
                        .url
                        .as_deref()
                        .filter(|u| external && !u.is_empty())
                        .map(|u| conn.url(u))
                        .unwrap_or_default(),
                    ..Default::default()
                }
            })
            .collect(),
        ..Default::default()
    }
}

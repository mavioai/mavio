//! What an item's details screen shows beyond the item's card
//! (`libs/design/components.md`, Details Header and Details Screen): a
//! series' episodes and the one to watch next, the extras, the collection,
//! the series an episode belongs to, and what the files hold.

use std::time::{Duration, SystemTime};

use mavio_api::proto::mavio::library::v1::{
    CreditKind, ExtraKind as ProtoExtraKind, ImageKind, Item, MediaStream,
    SeriesStatus as ProtoSeriesStatus, StreamKind, VideoRange as ProtoVideoRange,
};

use crate::connection::Connection;
use crate::view::{self, Artwork, Card};

/// What a credited person did.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum CreditJob {
    /// Acts in it.
    Actor,
    /// Acts in an episode as a guest.
    GuestStar,
    /// Created the series.
    Creator,
    /// Directed it.
    Director,
    /// Wrote it.
    Writer,
    /// Wrote the book.
    Author,
    /// Produced it.
    Producer,
    /// Wrote the music.
    Composer,
    /// Conducted the music.
    Conductor,
    /// Wrote the lyrics.
    Lyricist,
    /// Performs the music.
    Artist,
    /// Reads the audiobook.
    Narrator,
    /// Another job, named by the credit's role.
    Other,
}

impl From<Option<CreditKind>> for CreditJob {
    fn from(kind: Option<CreditKind>) -> Self {
        match kind {
            Some(CreditKind::CREDIT_KIND_ACTOR) => Self::Actor,
            Some(CreditKind::CREDIT_KIND_GUEST_STAR) => Self::GuestStar,
            Some(CreditKind::CREDIT_KIND_CREATOR) => Self::Creator,
            Some(CreditKind::CREDIT_KIND_DIRECTOR) => Self::Director,
            Some(CreditKind::CREDIT_KIND_WRITER) => Self::Writer,
            Some(CreditKind::CREDIT_KIND_AUTHOR) => Self::Author,
            Some(CreditKind::CREDIT_KIND_PRODUCER) => Self::Producer,
            Some(CreditKind::CREDIT_KIND_COMPOSER) => Self::Composer,
            Some(CreditKind::CREDIT_KIND_CONDUCTOR) => Self::Conductor,
            Some(CreditKind::CREDIT_KIND_LYRICIST) => Self::Lyricist,
            Some(CreditKind::CREDIT_KIND_ARTIST) => Self::Artist,
            Some(CreditKind::CREDIT_KIND_NARRATOR) => Self::Narrator,
            _ => Self::Other,
        }
    }
}

/// The dynamic range of a video.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum VideoRange {
    /// Standard.
    Sdr,
    /// HDR10.
    Hdr10,
    /// HDR10+.
    Hdr10Plus,
    /// Hybrid log-gamma.
    Hlg,
    /// Dolby Vision.
    DolbyVision,
}

/// A file's video.
#[derive(Debug, Clone, PartialEq)]
pub struct VideoStream {
    /// Its width in pixels.
    pub width: i32,
    /// Its height in pixels.
    pub height: i32,
    /// Its codec, such as "hevc".
    pub codec: String,
    /// Its dynamic range.
    pub range: VideoRange,
}

/// A subtitle stream of a file, in it or beside it.
#[derive(Debug, Clone, PartialEq)]
pub struct SubtitleStream {
    /// The stream's index.
    pub index: i32,
    /// Its language.
    pub language: Option<String>,
    /// Its title.
    pub title: Option<String>,
    /// Its codec, such as "subrip".
    pub codec: String,
    /// Whether it holds only what is not spoken in the audio's language.
    pub forced: bool,
    /// Whether it describes sounds for the hearing impaired.
    pub hearing_impaired: bool,
    /// Whether it is a file beside the video.
    pub external: bool,
}

/// Whether a series still airs.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum SeriesStatus {
    /// It airs.
    Continuing,
    /// It ended.
    Ended,
    /// It has not aired yet.
    Unreleased,
}

/// The series an episode or season belongs to.
#[derive(Debug, Clone, PartialEq)]
pub struct SeriesLink {
    /// The series on the same server.
    pub item_id: String,
    /// Its name.
    pub name: String,
    /// Its logo.
    pub logo: Option<Artwork>,
}

/// The collection a movie belongs to.
#[derive(Debug, Clone, PartialEq)]
pub struct CollectionLink {
    /// The collection on the same server.
    pub item_id: String,
    /// Its name.
    pub name: String,
    /// Its backdrop.
    pub backdrop: Option<Artwork>,
}

/// A page about the item on another site.
#[derive(Debug, Clone, PartialEq)]
pub struct ExternalLink {
    /// The site's name.
    pub name: String,
    /// The page.
    pub url: String,
}

/// What an extra is.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub enum ExtraKind {
    /// A trailer.
    Trailer,
    /// A clip.
    Clip,
    /// Behind the scenes.
    BehindTheScenes,
    /// A deleted scene.
    DeletedScene,
    /// An interview.
    Interview,
    /// A scene.
    Scene,
    /// A sample.
    Sample,
    /// A featurette.
    Featurette,
    /// A short.
    Short,
    /// Anything else.
    Other,
}

/// A trailer, featurette or other extra of an item, played by itself.
#[derive(Debug, Clone, PartialEq)]
pub struct Extra {
    /// The extra, as its card.
    pub card: Card,
    /// What it is.
    pub kind: ExtraKind,
}

/// An episode, as the details list it.
#[derive(Debug, Clone, PartialEq)]
pub struct Episode {
    /// The episode, as its card.
    pub card: Card,
    /// Its season.
    pub season_id: Option<String>,
    /// Its season's number; 0 for specials.
    pub season: Option<i32>,
    /// Its number in the season.
    pub number: Option<i32>,
    /// What happens in it.
    pub overview: Option<String>,
    /// When it aired.
    pub premiere: Option<SystemTime>,
    /// Where it resumes.
    pub position: Option<Duration>,
}

impl Episode {
    /// The episode of an item of a server.
    pub fn new(conn: &Connection, server_id: &str, item: &Item) -> Self {
        Episode {
            card: Card::new(conn, server_id, item),
            season_id: non_empty(&item.season_id),
            season: item.parent_index_number,
            number: item.index_number.filter(|n| *n > 0),
            overview: non_empty(&item.overview),
            premiere: time(item.premiere_date.as_option()),
            position: position(item),
        }
    }
}

/// The extra of an item of a server; none for theme songs and videos, which
/// play behind the screen elsewhere.
pub fn extra(conn: &Connection, server_id: &str, item: &Item) -> Option<Extra> {
    let kind = match item.extra.as_ref().and_then(|k| k.as_known()) {
        Some(ProtoExtraKind::EXTRA_KIND_THEME_SONG | ProtoExtraKind::EXTRA_KIND_THEME_VIDEO) => {
            return None;
        }
        Some(ProtoExtraKind::EXTRA_KIND_TRAILER) => ExtraKind::Trailer,
        Some(ProtoExtraKind::EXTRA_KIND_CLIP) => ExtraKind::Clip,
        Some(ProtoExtraKind::EXTRA_KIND_BEHIND_THE_SCENES) => ExtraKind::BehindTheScenes,
        Some(ProtoExtraKind::EXTRA_KIND_DELETED_SCENE) => ExtraKind::DeletedScene,
        Some(ProtoExtraKind::EXTRA_KIND_INTERVIEW) => ExtraKind::Interview,
        Some(ProtoExtraKind::EXTRA_KIND_SCENE) => ExtraKind::Scene,
        Some(ProtoExtraKind::EXTRA_KIND_SAMPLE) => ExtraKind::Sample,
        Some(ProtoExtraKind::EXTRA_KIND_FEATURETTE) => ExtraKind::Featurette,
        Some(ProtoExtraKind::EXTRA_KIND_SHORT) => ExtraKind::Short,
        _ => ExtraKind::Other,
    };
    Some(Extra {
        card: Card::new(conn, server_id, item),
        kind,
    })
}

/// The series of an episode or season.
pub fn series_link(conn: &Connection, series: &Item) -> SeriesLink {
    SeriesLink {
        item_id: series.id.clone().unwrap_or_default(),
        name: series.name.clone().unwrap_or_default(),
        logo: view::artwork(conn, series, ImageKind::IMAGE_KIND_LOGO),
    }
}

/// Of the collections of a server, the one a movie belongs to: by its TMDB
/// collection, else by name.
pub fn collection_link(
    conn: &Connection,
    item: &Item,
    collections: &[Item],
) -> Option<CollectionLink> {
    let name = item
        .collection_name
        .as_deref()
        .filter(|n| !n.is_empty())?
        .to_lowercase();
    let tmdb = item
        .external_ids
        .get("tmdb_collection")
        .filter(|id| !id.is_empty());
    let found = collections.iter().find(|c| match tmdb {
        Some(id) => c.external_ids.get("tmdb_collection") == Some(id),
        None => c.name.as_deref().is_some_and(|n| n.to_lowercase() == name),
    })?;
    Some(CollectionLink {
        item_id: found.id.clone().unwrap_or_default(),
        name: found.name.clone().unwrap_or_default(),
        backdrop: view::artwork(conn, found, ImageKind::IMAGE_KIND_BACKDROP),
    })
}

/// Episodes in order of season and number.
pub fn sort_episodes(items: &mut [Item]) {
    items.sort_by_key(|e| {
        (
            e.parent_index_number.unwrap_or(0),
            e.index_number.unwrap_or(0),
        )
    });
}

/// Of a series' episodes in order, ignoring specials unless there are only
/// specials, the one to watch next: the one played last if it is
/// unfinished, else the first unplayed one after it, else the first
/// unplayed one, else the first.
pub fn next_up(episodes: &[Item]) -> Option<&Item> {
    let regular: Vec<&Item> = episodes
        .iter()
        .filter(|e| e.parent_index_number.unwrap_or(0) > 0)
        .collect();
    let order: Vec<&Item> = if regular.is_empty() {
        episodes.iter().collect()
    } else {
        regular
    };
    let user = |e: &Item| e.user_data.as_option().cloned().unwrap_or_default();
    let played = |e: &Item| user(e).played.unwrap_or_default();
    let last_played = |e: &Item| time(user(e).last_played_time.as_option());
    let mut last: Option<usize> = None;
    for (i, e) in order.iter().enumerate() {
        if let Some(t) = last_played(e)
            && last.is_none_or(|l| last_played(order[l]).is_none_or(|lt| t >= lt))
        {
            last = Some(i);
        }
    }
    if let Some(l) = last {
        if !played(order[l]) && position(order[l]).is_some() {
            return Some(order[l]);
        }
        if let Some(after) = order[l + 1..].iter().find(|e| !played(e)) {
            return Some(after);
        }
    }
    order.iter().find(|e| !played(e)).or(order.first()).copied()
}

/// The first video stream of a file.
pub fn video(streams: &[MediaStream]) -> Option<VideoStream> {
    let v = streams.iter().find(|s| {
        s.kind.as_ref().and_then(|k| k.as_known()) == Some(StreamKind::STREAM_KIND_VIDEO)
    })?;
    Some(VideoStream {
        width: v.width.unwrap_or_default(),
        height: v.height.unwrap_or_default(),
        codec: v.codec.clone().unwrap_or_default(),
        range: match v.range.as_ref().and_then(|r| r.as_known()) {
            Some(ProtoVideoRange::VIDEO_RANGE_HDR10) => VideoRange::Hdr10,
            Some(ProtoVideoRange::VIDEO_RANGE_HDR10_PLUS) => VideoRange::Hdr10Plus,
            Some(ProtoVideoRange::VIDEO_RANGE_HLG) => VideoRange::Hlg,
            Some(ProtoVideoRange::VIDEO_RANGE_DOLBY_VISION) => VideoRange::DolbyVision,
            _ => VideoRange::Sdr,
        },
    })
}

/// The subtitle streams of a file.
pub fn subtitles(streams: &[MediaStream]) -> Vec<SubtitleStream> {
    streams
        .iter()
        .filter(|s| {
            s.kind.as_ref().and_then(|k| k.as_known()) == Some(StreamKind::STREAM_KIND_SUBTITLE)
        })
        .map(|s| SubtitleStream {
            index: s.index.unwrap_or_default(),
            language: non_empty(&s.language),
            title: non_empty(&s.title),
            codec: s.codec.clone().unwrap_or_default(),
            forced: s.forced.unwrap_or_default(),
            hearing_impaired: s.hearing_impaired.unwrap_or_default(),
            external: s.external_path.as_deref().is_some_and(|p| !p.is_empty()),
        })
        .collect()
}

/// A series' status.
pub fn series_status(item: &Item) -> Option<SeriesStatus> {
    match item.series_status.as_ref().and_then(|s| s.as_known()) {
        Some(ProtoSeriesStatus::SERIES_STATUS_CONTINUING) => Some(SeriesStatus::Continuing),
        Some(ProtoSeriesStatus::SERIES_STATUS_ENDED) => Some(SeriesStatus::Ended),
        Some(ProtoSeriesStatus::SERIES_STATUS_UNRELEASED) => Some(SeriesStatus::Unreleased),
        _ => None,
    }
}

/// The pages about an item on other sites.
pub fn links(item: &Item) -> Vec<ExternalLink> {
    item.external_urls
        .iter()
        .filter_map(|l| {
            Some(ExternalLink {
                name: non_empty(&l.name)?,
                url: non_empty(&l.url)?,
            })
        })
        .collect()
}

/// A point in time the server sent.
pub fn time<T: Clone>(t: Option<&T>) -> Option<SystemTime>
where
    SystemTime: TryFrom<T>,
{
    t.and_then(|t| SystemTime::try_from(t.clone()).ok())
}

/// Where the user stopped watching an item.
pub fn position(item: &Item) -> Option<Duration> {
    item.user_data
        .as_option()
        .and_then(|u| u.position.as_option())
        .and_then(|d| Duration::try_from(d.clone()).ok())
        .filter(|d| !d.is_zero())
}

/// A string the server sent, when it is not empty.
pub fn non_empty(s: &Option<String>) -> Option<String> {
    s.clone().filter(|s| !s.is_empty())
}

#[cfg(test)]
mod tests {
    use buffa_types::google::protobuf::Timestamp;
    use mavio_api::proto::mavio::user::v1::UserData;

    use super::*;

    fn episode(
        id: &str,
        season: i32,
        number: i32,
        played: bool,
        last: Option<i64>,
        at: Option<u64>,
    ) -> Item {
        Item {
            id: Some(id.into()),
            parent_index_number: Some(season),
            index_number: Some(number),
            user_data: UserData {
                played: Some(played),
                last_played_time: last
                    .map(|s| Timestamp {
                        seconds: s,
                        ..Default::default()
                    })
                    .into(),
                position: at
                    .map(|s| buffa_types::google::protobuf::Duration {
                        seconds: s as i64,
                        ..Default::default()
                    })
                    .into(),
                ..Default::default()
            }
            .into(),
            ..Default::default()
        }
    }

    #[test]
    fn next_up_follows_what_was_played_last() {
        let id = |e: Option<&Item>| e.and_then(|e| e.id.clone());
        let cases: Vec<(&str, Vec<Item>, Option<&str>)> = vec![
            (
                "nothing played starts at the first",
                vec![
                    episode("e1", 1, 1, false, None, None),
                    episode("e2", 1, 2, false, None, None),
                ],
                Some("e1"),
            ),
            (
                "the last played resumes while unfinished",
                vec![
                    episode("e1", 1, 1, true, Some(10), None),
                    episode("e2", 1, 2, false, Some(20), Some(300)),
                    episode("e3", 1, 3, false, None, None),
                ],
                Some("e2"),
            ),
            (
                "after the last played, the next unplayed",
                vec![
                    episode("e1", 1, 1, true, Some(30), None),
                    episode("e2", 1, 2, true, Some(20), None),
                    episode("e3", 1, 3, false, None, None),
                ],
                Some("e3"),
            ),
            (
                "specials are skipped",
                vec![
                    episode("s1", 0, 1, false, None, None),
                    episode("e1", 1, 1, false, None, None),
                ],
                Some("e1"),
            ),
            (
                "only specials",
                vec![episode("s1", 0, 1, false, None, None)],
                Some("s1"),
            ),
            (
                "all played starts again at the first",
                vec![
                    episode("e1", 1, 1, true, Some(10), None),
                    episode("e2", 1, 2, true, Some(20), None),
                ],
                Some("e1"),
            ),
        ];
        for (name, episodes, want) in cases {
            let got = id(next_up(&episodes));
            assert_eq!(
                got.as_deref(),
                want,
                "{name}: got = {got:?}, want = {want:?}"
            );
        }
    }
}

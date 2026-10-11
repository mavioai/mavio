//! Playback reporting (`docs/clients.md` §3): starting playbacks, the
//! progress reports every ten seconds and on pause, seek and stop, and the
//! progress made while the server was unreachable, kept in the offline
//! store and replayed once it answers.

use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant, SystemTime};

use mavio_api::proto::mavio::playback::v1::{
    ClientCapabilities, ReportProgressRequest, StartPlaybackRequest, StartPlaybackResponse,
    StopPlaybackRequest,
};
use mavio_api::proto::mavio::user::v1::UpdateUserDataRequest;

use crate::error::{Error, Result};
use crate::servers::Servers;
use crate::store::{PendingProgress, Store};

/// How often a playing item reports its position.
pub const REPORT_INTERVAL: Duration = Duration::from_secs(10);
/// A position further than this from where the last report and the time
/// since put it is a seek, reported at once.
const SEEK_THRESHOLD: Duration = Duration::from_secs(5);

/// What to play and how.
#[derive(Debug, Clone, Default)]
pub struct PlaybackOptions {
    /// The media source; unset picks the best one for the client.
    pub media_source_id: Option<String>,
    /// The audio stream; unset plays the user's or the source's default.
    pub audio_stream_index: Option<i32>,
    /// The subtitle stream, -1 for none; unset plays the user's or the
    /// source's default.
    pub subtitle_stream_index: Option<i32>,
    /// Caps the bitrate, in bits per second.
    pub max_bitrate: Option<i64>,
    /// Where to begin, usually the resume position.
    pub start_position: Option<Duration>,
    /// Asks for a file to keep instead of a stream.
    pub download: bool,
}

/// A playback the server started.
#[derive(Debug, Clone)]
pub struct Playback {
    /// The server playing.
    pub server_id: String,
    /// The item played.
    pub item_id: String,
    /// The server's answer, with the URLs relative to its base URL.
    pub response: StartPlaybackResponse,
    /// The media to play, resolved against the server's base URL.
    pub url: String,
}

/// Starts playbacks, reports their progress, and replays the progress the
/// servers missed.
#[derive(Clone)]
pub struct Reporter {
    servers: Arc<Servers>,
    store: Arc<Store>,
}

impl Reporter {
    /// Reports to the servers of servers, keeping missed progress in
    /// store.
    pub fn new(servers: Arc<Servers>, store: Arc<Store>) -> Self {
        Reporter { servers, store }
    }

    /// Starts playing an item with what the client can play.
    pub async fn start(
        &self,
        server_id: &str,
        item_id: &str,
        capabilities: ClientCapabilities,
        options: PlaybackOptions,
    ) -> Result<Session> {
        let conn = self.servers.connection(server_id)?;
        let response = conn
            .playback()
            .start_playback(StartPlaybackRequest {
                item_id: Some(item_id.to_owned()),
                media_source_id: options.media_source_id,
                capabilities: capabilities.into(),
                audio_stream_index: options.audio_stream_index,
                subtitle_stream_index: options.subtitle_stream_index,
                max_bitrate: options.max_bitrate,
                start_position: options.start_position.map(Into::into).into(),
                download: options.download.then_some(true),
                ..Default::default()
            })
            .await?
            .into_owned();
        let url = conn.url(response.url.as_deref().unwrap_or_default());
        let playback = Playback {
            server_id: server_id.to_owned(),
            item_id: item_id.to_owned(),
            response,
            url,
        };
        Ok(Session {
            reporter: self.clone(),
            playback,
            last: Mutex::new(None),
        })
    }

    /// Records progress made without the server, such as of a downloaded
    /// item played offline, and replays it when the server answers.
    pub async fn played_offline(
        &self,
        server_id: &str,
        item_id: &str,
        position: Option<Duration>,
    ) -> Result<()> {
        self.queue(server_id, item_id, position)?;
        match self.replay(server_id).await {
            Err(e) if e.is_transient() => Ok(()),
            other => other.map(|_| ()),
        }
    }

    /// Sends a server the progress it missed, oldest first, and returns how
    /// many reports it took. Progress of items the server no longer has is
    /// dropped; an unreachable server keeps the rest for later.
    pub async fn replay(&self, server_id: &str) -> Result<usize> {
        let pending = self.store.pending_progress(server_id)?;
        if pending.is_empty() {
            return Ok(0);
        }
        let conn = self.servers.connection(server_id)?;
        let mut sent = 0;
        for p in pending {
            let req = UpdateUserDataRequest {
                item_id: Some(p.item_id.clone()),
                played: p.played.then_some(true),
                position: p.position.map(Into::into).into(),
                ..Default::default()
            };
            match conn
                .user_data()
                .update_user_data(req)
                .await
                .map_err(Error::from)
            {
                Ok(_) => sent += 1,
                Err(Error::NotFound(_) | Error::Invalid(_) | Error::PermissionDenied(_)) => {}
                Err(e) => return Err(e),
            }
            self.store.remove_progress(&p)?;
        }
        Ok(sent)
    }

    fn queue(&self, server_id: &str, item_id: &str, position: Option<Duration>) -> Result<()> {
        self.store.queue_progress(&PendingProgress {
            server_id: server_id.to_owned(),
            item_id: item_id.to_owned(),
            position,
            played: position.is_none(),
            at: SystemTime::now(),
        })
    }
}

/// Whether a position is reported, given the last report, if any, with when
/// it was sent, its position and pause: the first is, then one every ten
/// seconds, and one at once on pause, resume and seek.
fn due(
    last: Option<(Instant, Duration, bool)>,
    now: Instant,
    position: Duration,
    paused: bool,
) -> bool {
    let Some((at, last_position, last_paused)) = last else {
        return true;
    };
    let elapsed = now.duration_since(at);
    let expected = if last_paused {
        last_position
    } else {
        last_position + elapsed
    };
    paused != last_paused
        || elapsed >= REPORT_INTERVAL
        || position.abs_diff(expected) > SEEK_THRESHOLD
}

/// A playback in progress. The app tells it the player's position as
/// often as it likes; it reports every ten seconds, and at once on pause,
/// resume and seek.
pub struct Session {
    reporter: Reporter,
    playback: Playback,
    /// When the last report was sent, with its position and pause.
    last: Mutex<Option<(Instant, Duration, bool)>>,
}

impl Session {
    /// The playback.
    pub fn playback(&self) -> &Playback {
        &self.playback
    }

    /// Takes the player's position, reporting it when due. Reports the
    /// server misses are kept to replay.
    pub async fn update(&self, position: Duration, paused: bool) -> Result<()> {
        let now = Instant::now();
        let last = *self.last.lock().unwrap_or_else(|e| e.into_inner());
        if !due(last, now, position, paused) {
            return Ok(());
        }
        *self.last.lock().unwrap_or_else(|e| e.into_inner()) = Some((now, position, paused));
        let req = ReportProgressRequest {
            playback_id: self.playback.response.playback_id.clone(),
            position: Some(position.into()).into(),
            paused: Some(paused),
            ..Default::default()
        };
        let result = match self.reporter.servers.connection(&self.playback.server_id) {
            Ok(conn) => conn
                .playback()
                .report_progress(req)
                .await
                .map(|_| ())
                .map_err(Error::from),
            Err(e) => Err(e),
        };
        self.keep_missed(result, Some(position))
    }

    /// Ends the playback at position, or played to the end when unset.
    pub async fn stop(&self, position: Option<Duration>) -> Result<()> {
        let req = StopPlaybackRequest {
            playback_id: self.playback.response.playback_id.clone(),
            position: position.map(Into::into).into(),
            ..Default::default()
        };
        let result = match self.reporter.servers.connection(&self.playback.server_id) {
            Ok(conn) => conn
                .playback()
                .stop_playback(req)
                .await
                .map(|_| ())
                .map_err(Error::from),
            Err(e) => Err(e),
        };
        self.keep_missed(result, position)
    }

    /// Keeps the progress of a report the server missed: one it could not
    /// be reached for, or of a playback it already ended.
    fn keep_missed(&self, result: Result<()>, position: Option<Duration>) -> Result<()> {
        match result {
            Err(e) if e.is_transient() || matches!(e, Error::NotFound(_)) => {
                self.reporter
                    .queue(&self.playback.server_id, &self.playback.item_id, position)
            }
            other => other,
        }
    }
}

#[cfg(test)]
mod tests {
    use mavio_api::proto::mavio::playback::v1::{
        ReportProgressResponse, StartPlaybackResponse, StopPlaybackResponse,
    };
    use mavio_api::proto::mavio::user::v1::UpdateUserDataResponse;

    use super::*;
    use crate::fake::error;
    use crate::servers::tests::{MemoryTokens, device, fake_server};

    fn secs(s: u64) -> Duration {
        Duration::from_secs(s)
    }

    async fn setup() -> (
        Reporter,
        Arc<Store>,
        crate::fake::Fake,
        Arc<Mutex<Vec<String>>>,
    ) {
        let store = Arc::new(Store::open_in_memory().unwrap());
        let servers = Arc::new(Servers::new(
            store.clone(),
            Arc::new(MemoryTokens::default()),
            device(),
        ));
        let fake = fake_server("sa", "Home");
        let log = Arc::new(Mutex::new(Vec::new()));
        let l = log.clone();
        fake.unary(
            "/mavio.playback.v1.PlaybackService/StartPlayback",
            |_, req: StartPlaybackRequest| {
                assert_eq!(req.start_position.as_option().map(|d| d.seconds), Some(30));
                Ok(StartPlaybackResponse {
                    playback_id: Some("p1".into()),
                    url: Some("media/p1/master.m3u8".into()),
                    ..Default::default()
                })
            },
        )
        .unary(
            "/mavio.playback.v1.PlaybackService/ReportProgress",
            move |_, req: ReportProgressRequest| {
                let pos = req.position.as_option().map_or(0, |d| d.seconds);
                l.lock()
                    .unwrap()
                    .push(format!("report {pos} {}", req.paused.unwrap_or_default()));
                Ok(ReportProgressResponse::default())
            },
        );
        let addr = fake.serve().await;
        servers.sign_in(&addr, "me", "secret").await.unwrap();
        (Reporter::new(servers, store.clone()), store, fake, log)
    }

    #[test]
    fn reports_every_ten_seconds_and_on_pause_and_seek() {
        let t0 = Instant::now();
        let at = |s| t0 + secs(s);
        let playing = |s, p| Some((at(s), secs(p), false));
        let cases = [
            ("first", None, at(0), 30, false, true),
            (
                "within ten seconds",
                playing(0, 30),
                at(4),
                34,
                false,
                false,
            ),
            ("ten seconds", playing(0, 30), at(10), 40, false, true),
            ("paused", playing(10, 40), at(10), 40, true, true),
            (
                "still paused",
                Some((at(10), secs(40), true)),
                at(13),
                40,
                true,
                false,
            ),
            (
                "seek while paused",
                Some((at(10), secs(40), true)),
                at(13),
                100,
                true,
                true,
            ),
            ("seek back", playing(0, 30), at(4), 10, false, true),
            (
                "resumed",
                Some((at(10), secs(40), true)),
                at(12),
                40,
                false,
                true,
            ),
        ];
        for (name, last, now, position, paused, want) in cases {
            assert_eq!(due(last, now, secs(position), paused), want, "{name}");
        }
    }

    #[tokio::test]
    async fn sessions_start_and_report() {
        let (reporter, _, _, log) = setup().await;
        let options = PlaybackOptions {
            start_position: Some(secs(30)),
            ..Default::default()
        };
        let session = reporter
            .start("sa", "i", ClientCapabilities::default(), options)
            .await
            .unwrap();
        assert!(
            session.playback().url.ends_with("/media/p1/master.m3u8"),
            "url = {}",
            session.playback().url
        );
        session.update(secs(30), false).await.unwrap();
        session.update(secs(30), true).await.unwrap();
        assert_eq!(
            log.lock().unwrap().as_slice(),
            ["report 30 false", "report 30 true"]
        );
    }

    #[tokio::test]
    async fn missed_progress_is_replayed() {
        let (reporter, store, fake, _) = setup().await;
        let session = reporter
            .start(
                "sa",
                "i",
                ClientCapabilities::default(),
                PlaybackOptions {
                    start_position: Some(secs(30)),
                    ..Default::default()
                },
            )
            .await
            .unwrap();
        // The server has forgotten the playback.
        fake.unary(
            "/mavio.playback.v1.PlaybackService/StopPlayback",
            |_, _: StopPlaybackRequest| Err::<StopPlaybackResponse, _>(error("not_found")),
        );
        session.stop(Some(secs(600))).await.unwrap();
        let pending = store.pending_progress("sa").unwrap();
        assert_eq!(pending.len(), 1);
        assert_eq!(pending[0].position, Some(secs(600)));

        let updates = Arc::new(Mutex::new(Vec::new()));
        let u = updates.clone();
        fake.unary(
            "/mavio.user.v1.UserDataService/UpdateUserData",
            move |_, req: UpdateUserDataRequest| {
                let id = req.item_id.clone().unwrap_or_default();
                if id == "gone" {
                    return Err(error("not_found"));
                }
                u.lock().unwrap().push(format!(
                    "{id} {:?} {}",
                    req.position.as_option().map(|d| d.seconds),
                    req.played.unwrap_or_default()
                ));
                Ok(UpdateUserDataResponse::default())
            },
        );
        reporter.played_offline("sa", "gone", None).await.unwrap();
        reporter.played_offline("sa", "d", None).await.unwrap();
        assert_eq!(
            updates.lock().unwrap().as_slice(),
            ["i Some(600) false", "d None true"]
        );
        assert!(store.pending_progress("sa").unwrap().is_empty());
    }

    #[tokio::test]
    async fn unreachable_servers_keep_progress() {
        let store = Arc::new(Store::open_in_memory().unwrap());
        let tokens = Arc::new(MemoryTokens::default());
        crate::TokenStore::set_token(tokens.as_ref(), "sa", "t");
        store
            .put_server(&crate::ServerRecord {
                id: "sa".into(),
                address: "http://127.0.0.1:1".into(),
                name: "Home".into(),
                version: "v1".into(),
                user_id: "u".into(),
                user_name: "me".into(),
            })
            .unwrap();
        let servers = Arc::new(Servers::new(store.clone(), tokens, device()));
        let reporter = Reporter::new(servers, store.clone());
        let session = Session {
            reporter: reporter.clone(),
            playback: Playback {
                server_id: "sa".into(),
                item_id: "i".into(),
                response: StartPlaybackResponse::default(),
                url: String::new(),
            },
            last: Mutex::new(None),
        };
        session.update(secs(5), false).await.unwrap();
        assert_eq!(
            store.pending_progress("sa").unwrap()[0].position,
            Some(secs(5))
        );
        session.stop(None).await.unwrap();
        let pending = store.pending_progress("sa").unwrap();
        assert!(
            pending.len() == 1 && pending[0].played,
            "pending = {pending:?}"
        );

        let replayed = reporter.replay("sa").await;
        assert!(
            matches!(replayed, Err(ref e) if e.is_transient()),
            "got = {replayed:?}"
        );
        assert_eq!(store.pending_progress("sa").unwrap().len(), 1);
    }
}

//! One `EventService` stream per signed-in server, reconnected with
//! backoff (`docs/clients.md` §3): the events go to the core and the app,
//! and the stream's state tells whether the server is reachable.

use std::sync::Arc;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use mavio_api::proto::mavio::session::v1::__buffa::oneof::event::Kind;
use mavio_api::proto::mavio::session::v1::{Event, SubscribeRequest, SubscribeResponse};
use tokio::task::JoinHandle;

use crate::error::{Error, Result};
use crate::servers::Servers;

/// The first wait before reconnecting, doubled after each failure.
const FIRST_DELAY: Duration = Duration::from_secs(1);
/// The longest wait before reconnecting.
const MAX_DELAY: Duration = Duration::from_secs(60);
/// The server sends a heartbeat every 30 seconds; a stream silent for
/// longer than this is taken for broken.
const SILENCE: Duration = Duration::from_secs(75);

/// Whether a server's stream is up.
#[derive(Debug, Clone, PartialEq, Eq)]
pub enum StreamState {
    /// Connecting, or waiting to reconnect after an attempt failed with
    /// the reason.
    Offline(String),
    /// The stream is up: the server is reachable and the device online.
    Online,
    /// The server rejected the token; the user signs in again.
    SignedOut,
}

/// Receives the events of the servers' streams.
pub trait EventSink: Send + Sync {
    /// The state of a server's stream changed.
    fn state(&self, server_id: &str, state: StreamState);
    /// A server sent an event, heartbeats excepted.
    fn event(&self, server_id: &str, event: &Event);
}

/// The waits between attempts to reconnect: doubling from one second up to
/// a minute, each shortened by up to a fifth at random so that clients a
/// restarted server dropped do not return at once.
#[derive(Debug, Default)]
pub struct Backoff {
    failures: u32,
}

impl Backoff {
    /// The wait before the next attempt.
    pub fn next_delay(&mut self) -> Duration {
        let delay = FIRST_DELAY
            .saturating_mul(1 << self.failures.min(16))
            .min(MAX_DELAY);
        self.failures = self.failures.saturating_add(1);
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map_or(0, |d| d.subsec_nanos());
        delay - delay.mul_f64(f64::from(nanos % 200) / 1000.0)
    }

    /// Starts over after a stream came up.
    pub fn reset(&mut self) {
        self.failures = 0;
    }
}

/// Streams a server's events to sink until the task is aborted or the
/// server ends the session. A stream the server rejects is followed again
/// once the session is refreshed.
pub fn spawn(servers: Arc<Servers>, server_id: String, sink: Arc<dyn EventSink>) -> JoinHandle<()> {
    tokio::spawn(async move {
        let mut backoff = Backoff::default();
        loop {
            let reason = match subscribe(&servers, &server_id, sink.as_ref(), &mut backoff).await {
                Err(Error::Unauthenticated(m)) => match refresh(&servers, &server_id).await {
                    Ok(()) => m,
                    Err(e) if e.is_transient() => e.to_string(),
                    Err(_) => {
                        servers.forget_token(&server_id);
                        sink.state(&server_id, StreamState::SignedOut);
                        return;
                    }
                },
                Err(Error::NotFound(_)) if servers.get(&server_id).is_err() => return,
                Err(e) => e.to_string(),
                Ok(()) => "the stream ended".to_owned(),
            };
            sink.state(&server_id, StreamState::Offline(reason));
            tokio::time::sleep(backoff.next_delay()).await;
        }
    })
}

async fn refresh(servers: &Servers, server_id: &str) -> Result<()> {
    servers.connection(server_id)?.refresh().await
}

/// Follows one stream until it ends or fails.
async fn subscribe(
    servers: &Servers,
    server_id: &str,
    sink: &dyn EventSink,
    backoff: &mut Backoff,
) -> Result<(), Error> {
    let conn = servers.connection(server_id)?;
    let mut stream = conn.events().subscribe(SubscribeRequest::default()).await?;
    loop {
        let msg = tokio::time::timeout(SILENCE, stream.message::<SubscribeResponse>())
            .await
            .map_err(|_| Error::Unreachable("the stream went silent".into()))??;
        let Some(msg) = msg else {
            return Ok(());
        };
        let event = msg
            .to_owned_message()
            .event
            .into_option()
            .unwrap_or_default();
        match event.kind {
            Some(Kind::Heartbeat(_)) | None => {}
            Some(Kind::Connected(_)) => {
                backoff.reset();
                sink.state(server_id, StreamState::Online);
                sink.event(server_id, &event);
            }
            Some(_) => sink.event(server_id, &event),
        }
    }
}

#[cfg(test)]
mod tests {
    use std::sync::Mutex;

    use mavio_api::proto::mavio::auth::v1::{StartRefreshRequest, StartRefreshResponse};
    use mavio_api::proto::mavio::session::v1::{Connected, Heartbeat, LibraryChanged};
    use tokio::sync::mpsc;

    use super::*;
    use crate::fake::error;
    use crate::servers::tests::{fake_server, servers};

    #[test]
    fn backoff_doubles_up_to_a_minute() {
        let mut b = Backoff::default();
        let waits: Vec<_> = (0..9).map(|_| b.next_delay()).collect();
        for (got, want) in waits.iter().zip([1, 2, 4, 8, 16, 32, 60, 60, 60]) {
            let want = Duration::from_secs(want);
            assert!(
                *got <= want && *got >= want.mul_f64(0.8),
                "got = {got:?}, want = about {want:?}"
            );
        }
        b.reset();
        assert!(b.next_delay() <= FIRST_DELAY);
    }

    #[derive(Default)]
    struct Recorder(Mutex<Vec<String>>);

    impl EventSink for Recorder {
        fn state(&self, server_id: &str, state: StreamState) {
            let state = match state {
                StreamState::Offline(_) => "offline",
                StreamState::Online => "online",
                StreamState::SignedOut => "signed out",
            };
            self.0.lock().unwrap().push(format!("{server_id} {state}"));
        }
        fn event(&self, server_id: &str, event: &Event) {
            let kind = match &event.kind {
                Some(Kind::Connected(_)) => "connected",
                Some(Kind::LibraryChanged(_)) => "library",
                _ => "other",
            };
            self.0.lock().unwrap().push(format!("{server_id} {kind}"));
        }
    }

    fn event(kind: impl Into<Option<Kind>>) -> SubscribeResponse {
        SubscribeResponse {
            event: Event {
                kind: kind.into(),
                ..Default::default()
            }
            .into(),
            ..Default::default()
        }
    }

    async fn until(rec: &Recorder, n: usize) -> Vec<String> {
        for _ in 0..500 {
            if rec.0.lock().unwrap().len() >= n {
                break;
            }
            tokio::time::sleep(Duration::from_millis(10)).await;
        }
        rec.0.lock().unwrap().clone()
    }

    #[tokio::test]
    async fn streams_reconnect_after_ending() {
        let (servers, _) = servers();
        let fake = fake_server("sa", "Home");
        let (streams_tx, streams_rx) =
            mpsc::unbounded_channel::<mpsc::UnboundedReceiver<SubscribeResponse>>();
        let streams_rx = Mutex::new(streams_rx);
        fake.stream("/mavio.session.v1.EventService/Subscribe", move || {
            streams_rx
                .lock()
                .unwrap()
                .try_recv()
                .unwrap_or_else(|_| mpsc::unbounded_channel().1)
        });
        let addr = fake.serve().await;
        servers.sign_in(&addr, "me", "secret").await.unwrap();

        // The first stream sends three events and ends; the second stays.
        let (first, rx) = mpsc::unbounded_channel();
        streams_tx.send(rx).unwrap();
        first.send(event(Connected::default())).unwrap();
        first.send(event(Heartbeat::default())).unwrap();
        first.send(event(LibraryChanged::default())).unwrap();
        drop(first);
        let (second, rx) = mpsc::unbounded_channel();
        streams_tx.send(rx).unwrap();
        second.send(event(Connected::default())).unwrap();

        let rec = Arc::new(Recorder::default());
        let task = spawn(Arc::new(servers), "sa".into(), rec.clone());
        let got = until(&rec, 6).await;
        task.abort();
        assert_eq!(
            got[..6],
            [
                "sa online",
                "sa connected",
                "sa library",
                "sa offline",
                "sa online",
                "sa connected"
            ],
        );
    }

    #[tokio::test]
    async fn rejected_tokens_sign_out() {
        let (servers, tokens) = servers();
        let fake = fake_server("sa", "Home");
        let addr = fake.serve().await;
        servers.sign_in(&addr, "me", "secret").await.unwrap();
        fake.stream_error(
            "/mavio.session.v1.EventService/Subscribe",
            "unauthenticated",
        )
        .unary(
            "/mavio.auth.v1.AuthService/StartRefresh",
            |_, _: StartRefreshRequest| -> Result<StartRefreshResponse, _> {
                Err(error("unauthenticated"))
            },
        );
        let rec = Arc::new(Recorder::default());
        spawn(Arc::new(servers), "sa".into(), rec.clone())
            .await
            .unwrap();
        assert_eq!(rec.0.lock().unwrap().as_slice(), ["sa signed out"]);
        assert!(!tokens.0.lock().unwrap().contains_key("sa"));
    }
}

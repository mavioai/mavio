//! The C ABI of the shared core, through which the desktop app
//! calls it from Go with purego (`docs/clients.md` §5.3).
//!
//! Requests and answers are messages of `mavio.capi.v1` (`libs/client-rust/
//! proto`) in byte buffers. The app opens a client with the one callback of
//! the process and a context of its own, sends each request with a call
//! number of its choosing, and receives the answer, a `Message` with that
//! number, through the callback, which also carries the servers' events
//! under call number zero. The callback runs on the core's threads; the
//! buffer it receives is valid only during the call.
#![allow(unsafe_code)]

/// The messages of `mavio.capi.v1` (buffa), generated into `src/gen` by
/// `pnpm nx run capi:generate`.
#[rustfmt::skip]
#[allow(missing_docs, clippy::all)]
#[path = "gen/mod.rs"]
mod proto;

use std::collections::HashMap;
use std::future::Future;
use std::sync::{Arc, Mutex, OnceLock};

use buffa::Message as _;
use mavio_api::proto::mavio::session::v1::__buffa::oneof::event::Kind as EventKind;
use mavio_api::proto::mavio::session::v1::Event;
use mavio_core::{Core, DeviceInfo, Store, StreamState};
use proto::mavio::capi::v1 as pb;
use proto::mavio::capi::v1::__buffa::oneof::{message::Kind as Answer, request::Kind as Call};
use tokio::runtime::Runtime;

/// The callback of the app: its context and a `Message` of `len` bytes at
/// `message`.
pub type Callback = extern "C" fn(context: usize, message: *const u8, len: usize);

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

/// Sends messages to the app.
#[derive(Clone, Copy)]
struct Sender {
    callback: Callback,
    context: usize,
}

impl Sender {
    fn send(self, call: u64, kind: Answer) {
        let bytes = pb::Message {
            call,
            kind: Some(kind),
            ..Default::default()
        }
        .encode_to_vec();
        (self.callback)(self.context, bytes.as_ptr(), bytes.len());
    }

    fn event(self, server_id: &str, kind: pb::__buffa::oneof::event::Kind) {
        let event = pb::Event {
            server_id: server_id.to_owned(),
            kind: Some(kind),
            ..Default::default()
        };
        self.send(0, Answer::Event(Box::new(event)));
    }
}

/// The secrets the app keeps, sessions' credentials and the device key,
/// given when it opens the core and kept up to date through
/// `TokenChanged`.
struct Tokens {
    tokens: Mutex<HashMap<String, String>>,
    sender: Sender,
}

impl mavio_core::TokenStore for Tokens {
    fn token(&self, server_id: &str) -> Option<String> {
        let tokens = self.tokens.lock().unwrap_or_else(|e| e.into_inner());
        tokens.get(server_id).cloned()
    }
    fn set_token(&self, server_id: &str, token: &str) {
        let mut tokens = self.tokens.lock().unwrap_or_else(|e| e.into_inner());
        tokens.insert(server_id.to_owned(), token.to_owned());
        self.sender.event(server_id, changed_token(token));
    }
    fn delete_token(&self, server_id: &str) {
        let mut tokens = self.tokens.lock().unwrap_or_else(|e| e.into_inner());
        tokens.remove(server_id);
        self.sender.event(server_id, changed_token(""));
    }
}

fn changed_token(token: &str) -> pb::__buffa::oneof::event::Kind {
    pb::TokenChanged {
        token: token.to_owned(),
        ..Default::default()
    }
    .into()
}

/// Relays the servers' streams to the app.
struct Listener(Sender);

impl mavio_core::Listener for Listener {
    fn server_state(&self, server_id: &str, state: StreamState) {
        use pb::stream_state::State;
        let (state, reason) = match state {
            StreamState::Offline(reason) => (State::STATE_OFFLINE, reason),
            StreamState::Online => (State::STATE_ONLINE, String::new()),
            StreamState::SignedOut => (State::STATE_SIGNED_OUT, String::new()),
        };
        let state = pb::StreamState {
            state: state.into(),
            reason,
            ..Default::default()
        };
        self.0.event(server_id, state.into());
    }

    fn event(&self, server_id: &str, event: &Event) {
        let kind = match event.kind.as_ref() {
            Some(EventKind::Connected(_)) => pb::Connected::default().into(),
            Some(EventKind::LibraryChanged(c)) => pb::LibraryChanged {
                library_ids: c.library_ids.clone(),
                changed_item_ids: c.changed_item_ids.clone(),
                removed_item_ids: c.removed_item_ids.clone(),
                ..Default::default()
            }
            .into(),
            Some(EventKind::UserDataChanged(c)) => pb::UserDataChanged {
                item_ids: c
                    .user_data
                    .iter()
                    .filter_map(|u| u.item_id.clone())
                    .collect(),
                ..Default::default()
            }
            .into(),
            Some(EventKind::SessionsChanged(_)) => pb::SessionsChanged::default().into(),
            _ => return,
        };
        self.0.event(server_id, kind);
    }
}

/// A client of the core, which the app opens with `mavio_open`.
pub struct Client {
    sender: Sender,
    core: OnceLock<Arc<Core>>,
}

fn error(code: pb::error::Code, reason: impl Into<String>) -> Answer {
    Answer::Error(Box::new(pb::Error {
        code: code.into(),
        reason: reason.into(),
        ..Default::default()
    }))
}

impl From<mavio_core::Error> for Answer {
    fn from(e: mavio_core::Error) -> Self {
        use mavio_core::Error as E;
        use pb::error::Code;
        let (code, reason) = match e {
            E::Unreachable(r) => (Code::CODE_UNREACHABLE, r),
            E::Unauthenticated(r) => (Code::CODE_UNAUTHENTICATED, r),
            E::PermissionDenied(r) => (Code::CODE_PERMISSION_DENIED, r),
            E::NotFound(r) => (Code::CODE_NOT_FOUND, r),
            E::Invalid(r) => (Code::CODE_INVALID, r),
            E::FailedPrecondition(r) => (Code::CODE_FAILED_PRECONDITION, r),
            E::Server(r) => (Code::CODE_SERVER, r),
            E::Store(r) => (Code::CODE_STORE, r),
        };
        error(code, reason)
    }
}

fn answer<T>(result: mavio_core::Result<T>, f: impl FnOnce(T) -> Answer) -> Answer {
    result.map_or_else(Answer::from, f)
}

fn record(s: mavio_core::ServerRecord) -> pb::ServerRecord {
    pb::ServerRecord {
        id: s.id,
        address: s.address,
        name: s.name,
        version: s.version,
        user_id: s.user_id,
        user_name: s.user_name,
        ..Default::default()
    }
}

impl Client {
    fn core(&self) -> Result<Arc<Core>, Answer> {
        self.core.get().cloned().ok_or_else(|| {
            error(
                pb::error::Code::CODE_FAILED_PRECONDITION,
                "the core is not open",
            )
        })
    }

    fn open(&self, open: pb::Open) -> Answer {
        let device = open.device.as_option().cloned().unwrap_or_default();
        let device = DeviceInfo {
            id: device.id,
            name: device.name,
            client: device.client,
            client_version: device.client_version,
        };
        let store = match Store::open(open.database_path) {
            Ok(store) => Arc::new(store),
            Err(e) => return e.into(),
        };
        let tokens = Tokens {
            tokens: Mutex::new(open.tokens.into_iter().collect()),
            sender: self.sender,
        };
        let core = Core::new(
            store,
            Arc::new(tokens),
            device,
            Arc::new(Listener(self.sender)),
        );
        if self.core.set(Arc::new(core)).is_err() {
            return error(
                pb::error::Code::CODE_FAILED_PRECONDITION,
                "the core is open",
            );
        }
        pb::Done::default().into()
    }

    async fn handle(self: Arc<Self>, call: Call) -> Answer {
        if let Call::Open(open) = call {
            return self.open(*open);
        }
        let core = match self.core() {
            Ok(core) => core,
            Err(answer) => return answer,
        };
        match call {
            Call::Open(_) => unreachable!("handled above"),
            Call::GetVersion(_) => Answer::Version(mavio_core::VERSION.to_owned()),
            Call::Probe(p) => answer(core.servers().probe(&p.address).await, |s| {
                pb::ServerInfo {
                    id: s.id,
                    name: s.name,
                    version: s.version,
                    address: s.address,
                    setup_required: s.setup_required,
                    ..Default::default()
                }
                .into()
            }),
            Call::SignIn(s) => answer(core.sign_in(&s.address, &s.name, &s.password).await, |s| {
                Answer::Server(Box::new(record(s)))
            }),
            Call::Start(_) => answer(core.start(), |()| pb::Done::default().into()),
            Call::Stop(_) => {
                core.stop();
                pb::Done::default().into()
            }
            Call::ListServers(_) => answer(core.servers().list(), |list| {
                pb::Servers {
                    servers: list.into_iter().map(record).collect(),
                    ..Default::default()
                }
                .into()
            }),
        }
    }
}

/// Runs work on the core's runtime, its panics answered as server errors.
fn spawn(sender: Sender, call: u64, work: impl Future<Output = Answer> + Send + 'static) {
    let task = runtime().spawn(work);
    runtime().spawn(async move {
        let answer = task.await.unwrap_or_else(|e| {
            error(
                pb::error::Code::CODE_SERVER,
                format!("the core failed: {e}"),
            )
        });
        sender.send(call, answer);
    });
}

/// Opens a client whose messages go to callback with context; the app
/// sends it `Open` first. The callback stays callable for as long as the
/// process runs, as answers to calls made before `mavio_close` may follow
/// it.
#[unsafe(no_mangle)]
pub extern "C" fn mavio_open(callback: Callback, context: usize) -> *mut Client {
    let client = Arc::new(Client {
        sender: Sender { callback, context },
        core: OnceLock::new(),
    });
    Arc::into_raw(client).cast_mut()
}

/// Sends a `Request` of `len` bytes at `request` to client, answered with
/// a `Message` of call number call, which is not zero.
///
/// # Safety
///
/// client comes from `mavio_open` and is not closed; request points to
/// `len` readable bytes, which the function copies before it returns.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn mavio_call(
    client: *const Client,
    call: u64,
    request: *const u8,
    len: usize,
) {
    // SAFETY: the caller passes a client from mavio_open that is still open.
    let client = unsafe {
        Arc::increment_strong_count(client);
        Arc::from_raw(client)
    };
    let sender = client.sender;
    let bytes = if len == 0 {
        &[][..]
    } else {
        // SAFETY: the caller passes len readable bytes at request.
        unsafe { std::slice::from_raw_parts(request, len) }
    };
    match pb::Request::decode_from_slice(bytes) {
        Ok(pb::Request {
            kind: Some(kind), ..
        }) => spawn(sender, call, client.handle(kind)),
        Ok(_) => spawn(sender, call, async {
            error(pb::error::Code::CODE_INVALID, "the request is empty")
        }),
        Err(e) => {
            let reason = format!("the request is not a message: {e}");
            spawn(sender, call, async move {
                error(pb::error::Code::CODE_INVALID, reason)
            });
        }
    }
}

/// Closes a client, stopping its event streams.
///
/// # Safety
///
/// client comes from `mavio_open` and is not closed already; it is not
/// used after.
#[unsafe(no_mangle)]
pub unsafe extern "C" fn mavio_close(client: *mut Client) {
    // SAFETY: the caller passes a client from mavio_open, once.
    let client = unsafe { Arc::from_raw(client.cast_const()) };
    if let Some(core) = client.core.get() {
        core.stop();
    }
}

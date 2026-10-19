//! The C ABI of the shared core, through which the desktop app
//! calls it from Go with purego (`docs/clients.md` §5.3).
//!
//! Requests and answers are messages of `mavio.capi.v1` (`libs/client-rust/
//! proto`) in byte buffers. The app opens a client with the one callback of
//! the process and a context of its own, sends each request with a call
//! number of its choosing, and receives the answer, a `Message` with that
//! number, through the callback, which also carries the servers' events
//! under call number zero. A call answered in parts sends messages marked
//! `more` before its last. The callback runs on the core's threads; the
//! buffer it receives is valid only during the call.
#![allow(unsafe_code)]

/// The messages of `mavio.capi.v1` (buffa), generated into `src/gen` by
/// `pnpm nx run capi:generate`.
#[rustfmt::skip]
#[allow(missing_docs, clippy::all)]
#[path = "gen/mod.rs"]
mod proto;

mod convert;

use std::collections::HashMap;
use std::future::Future;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::{Arc, Mutex, OnceLock};

use buffa::Message as _;
use mavio_api::proto::mavio::library::v1::ScanPhase;
use mavio_api::proto::mavio::playback::v1::ClientCapabilities;
use mavio_api::proto::mavio::session::v1::__buffa::oneof::event::Kind as EventKind;
use mavio_api::proto::mavio::session::v1::Event;
use mavio_core::{
    BrowserSignIn, Core, DeviceInfo, HomeObserver, LinkCode, QuickConnect, Session, Store,
    StreamState,
};
use proto::mavio::capi::v1 as pb;
use proto::mavio::capi::v1::__buffa::oneof::{message::Kind as Answer, request::Kind as Call};
use tokio::runtime::Runtime;
use tokio::sync::Notify;

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
        self.message(call, kind, false);
    }

    /// Sends a part of an answer, which more parts follow.
    fn part(self, call: u64, kind: Answer) {
        self.message(call, kind, true);
    }

    fn message(self, call: u64, kind: Answer, more: bool) {
        let bytes = pb::Message {
            call,
            kind: Some(kind),
            more,
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
            Some(EventKind::ScanProgressChanged(c)) => pb::ScanProgressChanged {
                library_id: c.library_id.clone().unwrap_or_default(),
                progress: c
                    .progress
                    .as_option()
                    .map(|p| pb::ScanProgress {
                        reading_folders: p.phase.as_ref().and_then(|p| p.as_known())
                            == Some(ScanPhase::SCAN_PHASE_FOLDERS),
                        folders_done: p.folders_done.unwrap_or_default(),
                        folders_total: p.folders_total.unwrap_or_default(),
                        items_done: p.items_done.unwrap_or_default(),
                        items_total: p.items_total.unwrap_or_default(),
                        fraction: p.fraction.unwrap_or_default(),
                        ..Default::default()
                    })
                    .into(),
                ..Default::default()
            }
            .into(),
            _ => return,
        };
        self.0.event(server_id, kind);
    }

    fn servers_changed(&self) {
        self.0.event("", pb::ServersChanged::default().into());
    }
}

/// Sends Home's rows as parts of the answer to a call.
struct Home {
    sender: Sender,
    call: u64,
}

impl HomeObserver for Home {
    fn rows(&self, rows: Vec<mavio_core::Row>) {
        self.sender
            .part(self.call, Answer::Rows(Box::new(convert::rows(rows))));
    }
}

/// A client of the core, which the app opens with `mavio_open`.
pub struct Client {
    sender: Sender,
    core: OnceLock<Arc<Core>>,
    /// The number of the last Quick Connect request or playback session.
    handles: AtomicU64,
    quick_connects: Mutex<HashMap<u64, QuickConnect>>,
    /// Sign-ins in the browser, each with what cancels it.
    sign_ins: Mutex<HashMap<u64, Arc<(BrowserSignIn, Notify)>>>,
    links: Mutex<HashMap<u64, LinkCode>>,
    sessions: Mutex<HashMap<u64, Arc<Session>>>,
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

fn done() -> Answer {
    pb::Done::default().into()
}

fn server_info(s: mavio_core::ServerInfo) -> pb::ServerInfo {
    pb::ServerInfo {
        id: s.id,
        name: s.name,
        version: s.version,
        address: s.address,
        setup_required: s.setup_required,
        mode: match s.mode {
            mavio_core::AccountMode::Unspecified => pb::AccountMode::ACCOUNT_MODE_UNSPECIFIED,
            mavio_core::AccountMode::Standalone => pb::AccountMode::ACCOUNT_MODE_STANDALONE,
            mavio_core::AccountMode::MavioDev => pb::AccountMode::ACCOUNT_MODE_MAVIO_DEV,
        }
        .into(),
        mavio_dev_url: s.mavio_dev_url,
        ..Default::default()
    }
}

fn account(a: Option<mavio_core::Account>) -> Answer {
    let a = a.unwrap_or(mavio_core::Account {
        id: String::new(),
        name: String::new(),
        api: String::new(),
    });
    Answer::Account(Box::new(pb::Account {
        id: a.id,
        name: a.name,
        api: a.api,
        ..Default::default()
    }))
}

fn record(s: mavio_core::ServerRecord) -> pb::ServerRecord {
    pb::ServerRecord {
        id: s.id,
        local: mavio_core::is_local_address(&s.address),
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
        done()
    }

    fn handle_number(&self) -> u64 {
        self.handles.fetch_add(1, Ordering::Relaxed) + 1
    }

    fn quick_connect(&self, request: u64) -> Result<QuickConnect, Answer> {
        let requests = self
            .quick_connects
            .lock()
            .unwrap_or_else(|e| e.into_inner());
        requests.get(&request).cloned().ok_or_else(|| {
            error(
                pb::error::Code::CODE_NOT_FOUND,
                "no such Quick Connect request",
            )
        })
    }

    fn session(&self, session: u64) -> Result<Arc<Session>, Answer> {
        let sessions = self.sessions.lock().unwrap_or_else(|e| e.into_inner());
        sessions
            .get(&session)
            .cloned()
            .ok_or_else(|| error(pb::error::Code::CODE_NOT_FOUND, "no such playback session"))
    }

    async fn play(&self, core: &Core, p: pb::Play) -> Answer {
        let capabilities = match ClientCapabilities::decode_from_slice(&p.capabilities) {
            Ok(c) => c,
            Err(e) => {
                return error(
                    pb::error::Code::CODE_INVALID,
                    format!("the capabilities are not a message: {e}"),
                );
            }
        };
        let options = convert::playback_options(p.options.as_option());
        let session = match core
            .reporter()
            .start(&p.server_id, &p.item_id, capabilities, options)
            .await
        {
            Ok(s) => s,
            Err(e) => return e.into(),
        };
        let conn = match core.servers().connection(&session.playback().server_id) {
            Ok(c) => c,
            Err(e) => return e.into(),
        };
        let number = self.handle_number();
        let playback = convert::playback(number, &conn, session.playback());
        let mut sessions = self.sessions.lock().unwrap_or_else(|e| e.into_inner());
        sessions.insert(number, Arc::new(session));
        Answer::Playback(Box::new(playback))
    }

    async fn handle(self: Arc<Self>, number: u64, call: Call) -> Answer {
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
                server_info(s).into()
            }),
            Call::SignIn(s) => answer(core.sign_in(&s.address, &s.name, &s.password).await, |s| {
                Answer::Server(Box::new(record(s)))
            }),
            Call::Start(_) => answer(core.start(), |()| done()),
            Call::Stop(_) => {
                core.stop();
                done()
            }
            Call::ListServers(_) => answer(core.servers().list(), |list| {
                pb::Servers {
                    servers: list.into_iter().map(record).collect(),
                    ..Default::default()
                }
                .into()
            }),
            Call::CreateFirstUser(c) => answer(
                core.create_first_user(&c.address, &c.name, &c.password)
                    .await,
                |s| Answer::Server(Box::new(record(s))),
            ),
            Call::Discover(d) => {
                let wait = std::time::Duration::from_millis(d.wait_ms.into());
                answer(mavio_core::discover(wait).await, |found| {
                    pb::FoundServers {
                        servers: found
                            .into_iter()
                            .map(|f| pb::FoundServer {
                                id: f.id,
                                name: f.name,
                                address: f.address,
                                version: f.version,
                                ..Default::default()
                            })
                            .collect(),
                        ..Default::default()
                    }
                    .into()
                })
            }
            Call::RemoveServer(r) => answer(core.remove(&r.server_id).await, |()| done()),
            Call::OrderServers(o) => answer(core.servers().order(&o.server_ids), |()| done()),
            Call::StartQuickConnect(s) => {
                let qc = match core.servers().start_quick_connect(&s.address).await {
                    Ok(qc) => qc,
                    Err(e) => return e.into(),
                };
                let request = self.handle_number();
                let answer = pb::QuickConnect {
                    request,
                    code: qc.code.clone(),
                    expires_ms: convert::time(qc.expires),
                    server: server_info(qc.server.clone()).into(),
                    ..Default::default()
                };
                let mut requests = self
                    .quick_connects
                    .lock()
                    .unwrap_or_else(|e| e.into_inner());
                requests.insert(request, qc);
                answer.into()
            }
            Call::PollQuickConnect(p) => {
                let qc = match self.quick_connect(p.request) {
                    Ok(qc) => qc,
                    Err(answer) => return answer,
                };
                match core.poll_quick_connect(&qc).await {
                    Ok(Some(server)) => {
                        let mut requests = self
                            .quick_connects
                            .lock()
                            .unwrap_or_else(|e| e.into_inner());
                        requests.remove(&p.request);
                        Answer::Server(Box::new(record(server)))
                    }
                    Ok(None) => done(),
                    Err(e) => e.into(),
                }
            }
            Call::CancelQuickConnect(c) => {
                let mut requests = self
                    .quick_connects
                    .lock()
                    .unwrap_or_else(|e| e.into_inner());
                requests.remove(&c.request);
                done()
            }
            Call::AuthorizeQuickConnect(a) => answer(
                core.servers()
                    .authorize_quick_connect(&a.server_id, &a.code)
                    .await,
                |()| done(),
            ),
            Call::StartAccountSignIn(s) => {
                let sign_in = match core.start_browser_sign_in(&s.api, &s.client_id).await {
                    Ok(s) => s,
                    Err(e) => return e.into(),
                };
                let request = self.handle_number();
                let answer = pb::AccountSignIn {
                    request,
                    url: sign_in.url.clone(),
                    ..Default::default()
                };
                let mut sign_ins = self.sign_ins.lock().unwrap_or_else(|e| e.into_inner());
                sign_ins.insert(request, Arc::new((sign_in, Notify::new())));
                answer.into()
            }
            Call::FinishAccountSignIn(f) => {
                let sign_in = {
                    let sign_ins = self.sign_ins.lock().unwrap_or_else(|e| e.into_inner());
                    sign_ins.get(&f.request).cloned()
                };
                let Some(sign_in) = sign_in else {
                    return error(pb::error::Code::CODE_NOT_FOUND, "no such sign-in");
                };
                let (browser, cancel) = &*sign_in;
                let result = tokio::select! {
                    r = core.finish_browser_sign_in(browser) => r,
                    () = cancel.notified() => Err(mavio_core::Error::NotFound("the sign-in was cancelled".into())),
                };
                let mut sign_ins = self.sign_ins.lock().unwrap_or_else(|e| e.into_inner());
                sign_ins.remove(&f.request);
                answer(result, |a| account(Some(a)))
            }
            Call::CancelAccountSignIn(c) => {
                let mut sign_ins = self.sign_ins.lock().unwrap_or_else(|e| e.into_inner());
                if let Some(s) = sign_ins.remove(&c.request) {
                    s.1.notify_one();
                }
                done()
            }
            Call::StartLink(s) => {
                let link = match core.start_link(&s.api, &s.client_id).await {
                    Ok(l) => l,
                    Err(e) => return e.into(),
                };
                let request = self.handle_number();
                let answer = pb::LinkCode {
                    request,
                    user_code: link.user_code.clone(),
                    verification_uri: link.verification_uri.clone(),
                    verification_uri_complete: link.verification_uri_complete.clone(),
                    expires_ms: convert::time(link.expires),
                    interval_ms: u32::try_from(link.interval.as_millis()).unwrap_or(u32::MAX),
                    ..Default::default()
                };
                let mut links = self.links.lock().unwrap_or_else(|e| e.into_inner());
                links.insert(request, link);
                answer.into()
            }
            Call::PollLink(p) => {
                let link = {
                    let links = self.links.lock().unwrap_or_else(|e| e.into_inner());
                    links.get(&p.request).cloned()
                };
                let Some(link) = link else {
                    return error(pb::error::Code::CODE_NOT_FOUND, "no such link code");
                };
                match core.poll_link(&link).await {
                    Ok(Some(a)) => {
                        let mut links = self.links.lock().unwrap_or_else(|e| e.into_inner());
                        links.remove(&p.request);
                        account(Some(a))
                    }
                    Ok(None) => done(),
                    Err(e) => e.into(),
                }
            }
            Call::CancelLink(c) => {
                let mut links = self.links.lock().unwrap_or_else(|e| e.into_inner());
                links.remove(&c.request);
                done()
            }
            Call::GetAccount(_) => account(core.account()),
            Call::CallServer(c) => answer(
                core.call(&c.server_id, &c.procedure, c.request.clone())
                    .await,
                |(body, base_url)| {
                    Answer::Response(Box::new(pb::ServerResponse {
                        body,
                        base_url,
                        ..Default::default()
                    }))
                },
            ),
            Call::SyncAccount(_) => answer(core.sync_account().await, |()| done()),
            Call::SignOut(_) => answer(core.sign_out().await, |()| done()),
            Call::LoadHome(_) => {
                let home = Home {
                    sender: self.sender,
                    call: number,
                };
                answer(core.home(&home).await, |()| done())
            }
            Call::GetHub(h) => answer(
                core.hub(&h.server_id, &h.hub_id, h.offset, h.limit).await,
                |p| convert::page(p).into(),
            ),
            Call::GetRecommended(r) => answer(
                core.recommended(&r.server_id, &r.library_id).await,
                |rows| convert::rows(rows).into(),
            ),
            Call::ListLibraries(l) => answer(core.libraries(&l.server_id).await, |l| {
                convert::libraries(l).into()
            }),
            Call::ListLibraryItems(l) => {
                let sort = convert::sort(l.sort.as_known());
                let filter = convert::filter(l.filter.as_option());
                answer(
                    core.library_items(
                        &l.server_id,
                        &l.library_id,
                        sort,
                        &filter,
                        l.offset,
                        l.limit,
                    )
                    .await,
                    |p| convert::page(p).into(),
                )
            }
            Call::GetDetails(d) => answer(core.details(&d.server_id, &d.item_id).await, |d| {
                convert::details(d).into()
            }),
            Call::SetPlayed(s) => answer(
                core.set_played(&s.server_id, &s.item_id, s.played).await,
                |()| done(),
            ),
            Call::SetFavorite(s) => answer(
                core.set_favorite(&s.server_id, &s.item_id, s.favorite)
                    .await,
                |()| done(),
            ),
            Call::Search(s) => answer(core.search(&s.query).await, |r| {
                convert::search_results(r).into()
            }),
            Call::SearchServer(s) => {
                answer(core.search_server(&s.server_id, &s.query).await, |r| {
                    convert::search_results(vec![r]).into()
                })
            }
            Call::Play(p) => self.play(&core, *p).await,
            Call::UpdatePlayback(u) => {
                let session = match self.session(u.session) {
                    Ok(s) => s,
                    Err(answer) => return answer,
                };
                let position = convert::duration(u.position_ms);
                answer(session.update(position, u.paused).await, |()| done())
            }
            Call::StopPlayback(s) => {
                let session = {
                    let mut sessions = self.sessions.lock().unwrap_or_else(|e| e.into_inner());
                    sessions.remove(&s.session)
                };
                let Some(session) = session else {
                    return error(pb::error::Code::CODE_NOT_FOUND, "no such playback session");
                };
                let position = s.position_ms.map(convert::duration);
                answer(session.stop(position).await, |()| done())
            }
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
        handles: AtomicU64::new(0),
        quick_connects: Mutex::new(HashMap::new()),
        sign_ins: Mutex::new(HashMap::new()),
        links: Mutex::new(HashMap::new()),
        sessions: Mutex::new(HashMap::new()),
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
        }) => spawn(sender, call, client.handle(call, kind)),
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

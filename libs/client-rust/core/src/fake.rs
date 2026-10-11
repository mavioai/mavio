//! A fake Mavio server for the core's tests: it answers the Connect
//! procedures a test registers, unary ones with a function of the request
//! and server streams with the messages a test sends through a channel.

use std::collections::HashMap;
use std::convert::Infallible;
use std::net::SocketAddr;
use std::pin::Pin;
use std::sync::{Arc, Mutex};
use std::task::{Context, Poll};

use buffa::Message;
use bytes::Bytes;
use http_body_util::{BodyExt, Full};
use hyper::body::{Frame, Incoming};
use hyper::{Request, Response, StatusCode};
use hyper_util::rt::TokioIo;
use tokio::net::TcpListener;
use tokio::sync::mpsc;

type Unary =
    Arc<dyn Fn(&Request<()>, Bytes) -> Result<Bytes, (StatusCode, &'static str)> + Send + Sync>;
type Stream = Arc<dyn Fn() -> Result<mpsc::UnboundedReceiver<Bytes>, &'static str> + Send + Sync>;

/// A fake server; it stops when dropped.
#[derive(Clone, Default)]
pub struct Fake {
    unary: Arc<Mutex<HashMap<String, Unary>>>,
    streams: Arc<Mutex<HashMap<String, Stream>>>,
}

/// The status and code of a Connect error.
pub fn error(code: &'static str) -> (StatusCode, &'static str) {
    let status = match code {
        "unauthenticated" => StatusCode::UNAUTHORIZED,
        "not_found" => StatusCode::NOT_FOUND,
        "permission_denied" => StatusCode::FORBIDDEN,
        "invalid_argument" | "failed_precondition" => StatusCode::BAD_REQUEST,
        "unavailable" => StatusCode::SERVICE_UNAVAILABLE,
        _ => StatusCode::INTERNAL_SERVER_ERROR,
    };
    (status, code)
}

impl Fake {
    /// Answers a unary procedure, such as
    /// `/mavio.auth.v1.AuthService/Login`, with f of the request.
    pub fn unary<Req, Resp>(
        &self,
        procedure: &str,
        f: impl Fn(&Request<()>, Req) -> Result<Resp, (StatusCode, &'static str)>
        + Send
        + Sync
        + 'static,
    ) -> &Self
    where
        Req: Message,
        Resp: Message,
    {
        let handler: Unary = Arc::new(move |head, body| {
            let req = Req::decode_from_slice(&body).map_err(|_| error("invalid_argument"))?;
            f(head, req).map(|resp| Bytes::from(resp.encode_to_vec()))
        });
        self.unary
            .lock()
            .unwrap()
            .insert(procedure.to_owned(), handler);
        self
    }

    /// Streams to each call of a server-streaming procedure what the
    /// channel f returns carries, ending the stream when it closes.
    pub fn stream<Resp: Message>(
        &self,
        procedure: &str,
        f: impl Fn() -> mpsc::UnboundedReceiver<Resp> + Send + Sync + 'static,
    ) -> &Self {
        let handler: Stream = Arc::new(move || {
            let mut rx = f();
            let (tx, out) = mpsc::unbounded_channel();
            tokio::spawn(async move {
                while let Some(msg) = rx.recv().await {
                    if tx.send(Bytes::from(msg.encode_to_vec())).is_err() {
                        break;
                    }
                }
            });
            Ok(out)
        });
        self.streams
            .lock()
            .unwrap()
            .insert(procedure.to_owned(), handler);
        self
    }

    /// Ends each call of a server-streaming procedure with an error code,
    /// such as "unauthenticated".
    pub fn stream_error(&self, procedure: &str, code: &'static str) -> &Self {
        let handler: Stream = Arc::new(move || Err(code));
        self.streams
            .lock()
            .unwrap()
            .insert(procedure.to_owned(), handler);
        self
    }

    /// Serves on a free local port and returns the server's address.
    pub async fn serve(&self) -> String {
        let listener = TcpListener::bind(SocketAddr::from(([127, 0, 0, 1], 0)))
            .await
            .unwrap();
        let addr = listener.local_addr().unwrap();
        let fake = self.clone();
        tokio::spawn(async move {
            loop {
                let Ok((conn, _)) = listener.accept().await else {
                    return;
                };
                let fake = fake.clone();
                tokio::spawn(async move {
                    let service = hyper::service::service_fn(move |req| {
                        let fake = fake.clone();
                        async move { Ok::<_, Infallible>(fake.handle(req).await) }
                    });
                    let _ = hyper::server::conn::http1::Builder::new()
                        .serve_connection(TokioIo::new(conn), service)
                        .await;
                });
            }
        });
        format!("http://{addr}")
    }

    async fn handle(
        &self,
        req: Request<Incoming>,
    ) -> Response<http_body_util::combinators::BoxBody<Bytes, Infallible>> {
        let (parts, body) = req.into_parts();
        let path = parts.uri.path().to_owned();
        let body = body
            .collect()
            .await
            .map(|b| b.to_bytes())
            .unwrap_or_default();
        let head = Request::from_parts(parts, ());

        let stream = self.streams.lock().unwrap().get(&path).cloned();
        if let Some(stream) = stream {
            let frames = match stream() {
                Ok(rx) => Frames {
                    rx: Some(rx),
                    end: Bytes::from_static(b"{}"),
                    done: false,
                },
                Err(code) => Frames {
                    rx: None,
                    end: Bytes::from(format!(r#"{{"error":{{"code":"{code}"}}}}"#)),
                    done: false,
                },
            };
            return Response::builder()
                .header("content-type", "application/connect+proto")
                .body(BodyExt::boxed(frames))
                .unwrap();
        }
        let unary = self.unary.lock().unwrap().get(&path).cloned();
        let result = match unary {
            Some(f) => f(&head, body),
            None => Err(error("unimplemented")),
        };
        match result {
            Ok(bytes) => Response::builder()
                .header("content-type", "application/proto")
                .body(BodyExt::boxed(Full::new(bytes)))
                .unwrap(),
            Err((status, code)) => Response::builder()
                .status(if code == "unimplemented" {
                    StatusCode::NOT_IMPLEMENTED
                } else {
                    status
                })
                .header("content-type", "application/json")
                .body(BodyExt::boxed(Full::new(Bytes::from(format!(
                    r#"{{"code":"{code}"}}"#
                )))))
                .unwrap(),
        }
    }
}

/// The messages of a channel framed as a Connect stream, ended with an
/// end-of-stream message when the channel closes, or at once without a
/// channel.
struct Frames {
    rx: Option<mpsc::UnboundedReceiver<Bytes>>,
    end: Bytes,
    done: bool,
}

impl hyper::body::Body for Frames {
    type Data = Bytes;
    type Error = Infallible;

    fn poll_frame(
        mut self: Pin<&mut Self>,
        cx: &mut Context<'_>,
    ) -> Poll<Option<Result<Frame<Bytes>, Infallible>>> {
        if self.done {
            return Poll::Ready(None);
        }
        let next = match self.rx.as_mut() {
            Some(rx) => rx.poll_recv(cx),
            None => Poll::Ready(None),
        };
        match next {
            Poll::Ready(Some(msg)) => Poll::Ready(Some(Ok(Frame::data(envelope(0, &msg))))),
            Poll::Ready(None) => {
                self.done = true;
                Poll::Ready(Some(Ok(Frame::data(envelope(2, &self.end)))))
            }
            Poll::Pending => Poll::Pending,
        }
    }
}

fn envelope(flags: u8, data: &[u8]) -> Bytes {
    let mut out = Vec::with_capacity(5 + data.len());
    out.push(flags);
    out.extend_from_slice(&u32::try_from(data.len()).unwrap().to_be_bytes());
    out.extend_from_slice(data);
    Bytes::from(out)
}

///Shorthand for `OwnedView<ClaimServerRequestView<'static>>`.
pub type OwnedClaimServerRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerRequestView<'static>,
>;
///Shorthand for `OwnedView<ClaimServerResponseView<'static>>`.
pub type OwnedClaimServerResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<'static>,
>;
///Shorthand for `OwnedView<ListChangesRequestView<'static>>`.
pub type OwnedListChangesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ListChangesRequestView<'static>,
>;
///Shorthand for `OwnedView<ListChangesResponseView<'static>>`.
pub type OwnedListChangesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<'static>,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ClaimServerResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ClaimServerResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<'static>,
> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self.reborrow(), codec)
    }
    /// An `OwnedView` still holds the buffer it was decoded from, so
    /// its large fields can be handed to the response body by
    /// reference count instead of copied. The bare view impl above
    /// cannot do this: it has borrows but no buffer to name.
    fn encode_segments(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::connectrpc::EncodedBody, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body_segments(
            self.reborrow(),
            self.bytes(),
            codec,
        )
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ListChangesResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ListChangesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<'static>,
> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self.reborrow(), codec)
    }
    /// An `OwnedView` still holds the buffer it was decoded from, so
    /// its large fields can be handed to the response body by
    /// reference count instead of copied. The bare view impl above
    /// cannot do this: it has borrows but no buffer to name.
    fn encode_segments(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::connectrpc::EncodedBody, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body_segments(
            self.reborrow(),
            self.bytes(),
            codec,
        )
    }
}
/// Full service name for this service.
pub const SERVER_SERVICE_SERVICE_NAME: &str = "mavio.cloud.v1.ServerService";
/// Static [`Spec`](::connectrpc::Spec) for the `ClaimServer` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_CLAIM_SERVER_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/ClaimServer",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListChanges` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_LIST_CHANGES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/ListChanges",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// ServerService is mavio.dev's API for servers in mavio.dev mode
/// (docs/accounts.md §3.2, docs/mavio-dev.md §3). Every request is
/// authenticated by a JWT the server signs with its identity key
/// (private_key_jwt): EdDSA, with the server's ID as its kid, iss and sub,
/// mavio.dev's API base URL as its aud, a random jti, and an expiry at
/// most a minute after it was issued, carried as a bearer token. Claiming
/// registers the key; the other methods need a claimed server.
///
/// # Implementing handlers
///
/// Implement methods with plain `async fn`; the returned future satisfies
/// the `Send` bound automatically.
///
/// **Unary and server-streaming requests** arrive as
/// [`ServiceRequest<'_, Req>`](::connectrpc::ServiceRequest): a zero-copy
/// view of the request plus its body, valid for the duration of the call.
/// Fields are read directly (`request.name` is a `&str` into the decoded
/// buffer) and the borrow may be held across `.await` points. Anything
/// that must outlive the call — `tokio::spawn`, channels, server state,
/// or data captured by a returned response stream — takes owned data:
/// call `request.to_owned_message()` (or copy the specific fields)
/// first.
///
/// **Client-streaming and bidi requests** arrive as
/// [`InboundStream<Req>`](::connectrpc::InboundStream) — a
/// `ServiceStream` of [`StreamMessage`](::connectrpc::StreamMessage)s.
/// Each item owns its decoded buffer and is `Send + 'static`, so items
/// can be buffered or moved into spawned tasks; read fields zero-copy
/// through the generated accessor methods (`item.name()`) or `.view()`,
/// convert with `.to_owned_message()`, or yield an item back unchanged —
/// `StreamMessage<M>` implements `Encodable<M>`.
///
/// Request types resolved through `extern_path` (e.g. well-known types
/// from another crate) use the same wrappers; the crate that owns the
/// type must be generated with buffa ≥ 0.9.0 and views enabled so the
/// backing `HasMessageView` impl exists.
///
/// The `impl Encodable<Out>` return bound accepts the owned `Out`, the
/// generated `OutView<'_>` / `OwnedOutView`,
/// [`MaybeBorrowed`](::connectrpc::MaybeBorrowed), or
/// [`PreEncoded`](::connectrpc::PreEncoded) for handlers that encode a
/// non-`'static` view internally and pass the bytes across the handler
/// boundary. View bodies are not emitted for output types mapped via
/// `extern_path` (the impl would be an orphan); return owned for
/// WKT/extern outputs.
///
/// Server-streaming and bidi-streaming methods return
/// `ServiceStream<impl Encodable<Out> + Send + use<Self>>`. The
/// `use<Self>` precise-capturing clause excludes `&self`'s lifetime and
/// the request's lifetime (unary methods use `use<'a, Self>` and may
/// borrow from `&self`), so stream items must be `'static` and cannot
/// borrow from the request. To stream view-encoded data, encode each
/// item inside the stream body and yield
/// [`PreEncoded`](::connectrpc::PreEncoded) — see its `# Streaming
/// example` doc.
#[allow(clippy::type_complexity)]
pub trait ServerService: Send + Sync + 'static {
    /// ClaimServer registers the server under the account that created the
    /// claim code at mavio.dev/claim, which becomes its administrator. An
    /// unknown, used or expired code fails with INVALID_ARGUMENT. Claiming a
    /// server again, after its command line unclaimed it, keeps its changes.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn claim_server<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::ClaimServerRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::ClaimServerResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListChanges gives the server's changes after a cursor, oldest first.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_changes<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::ListChangesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::ListChangesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
}
/// Extension trait for registering a service implementation with a Router.
///
/// This trait is automatically implemented for all types that implement the service trait.
/// Prefer [`Router::add_service`](::connectrpc::Router::add_service) for
/// top-down registration; `register` remains available for compatibility
/// and cases where the service-first call shape is more convenient.
///
/// # Example
///
/// ```rust,ignore
/// use std::sync::Arc;
///
/// let service = Arc::new(MyServiceImpl);
/// let router = service.register(Router::new());
/// ```
pub trait ServerServiceExt: ServerService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: ServerService> ServerServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view(
                SERVER_SERVICE_SERVICE_NAME,
                "ClaimServer",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::ClaimServerRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.claim_server(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::ClaimServerResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_CLAIM_SERVER_SPEC)
            .route_view_idempotent(
                SERVER_SERVICE_SERVICE_NAME,
                "ListChanges",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::ListChangesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::ListChangesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_changes(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::ListChangesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_LIST_CHANGES_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct ServerServiceRegisterMarker;
impl<S: ServerService> ::connectrpc::ServiceRegister<ServerServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as ServerServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `ServerService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = ServerServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct ServerServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: ServerService> ServerServiceServer<T> {
    /// Wrap a service implementation in a monomorphic dispatcher.
    pub fn new(service: T) -> Self {
        Self {
            inner: ::std::sync::Arc::new(service),
        }
    }
    /// Wrap an already-`Arc`'d service implementation.
    pub fn from_arc(inner: ::std::sync::Arc<T>) -> Self {
        Self { inner }
    }
}
impl<T> Clone for ServerServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: ServerService> ::connectrpc::Dispatcher for ServerServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.cloud.v1.ServerService/")?;
        match method {
            "ClaimServer" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SERVER_SERVICE_CLAIM_SERVER_SPEC),
                )
            }
            "ListChanges" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SERVER_SERVICE_LIST_CHANGES_SPEC),
                )
            }
            _ => None,
        }
    }
    fn call_unary(
        &self,
        path: &str,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::Payload,
        format: ::connectrpc::CodecFormat,
    ) -> ::connectrpc::dispatcher::codegen::UnaryResult {
        let Some(method) = path.strip_prefix("mavio.cloud.v1.ServerService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "ClaimServer" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::ClaimServerRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::ClaimServerRequest,
                    >::from_parts(&req, &body);
                    svc.claim_server(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::ClaimServerResponse,
                        >(format)
                })
            }
            "ListChanges" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::ListChangesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::ListChangesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::ListChangesRequest,
                    >::from_parts(&req, &body);
                    svc.list_changes(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::ListChangesResponse,
                        >(format)
                })
            }
            _ => ::connectrpc::dispatcher::codegen::unimplemented_unary(path),
        }
    }
    fn call_server_streaming(
        &self,
        path: &str,
        ctx: ::connectrpc::RequestContext,
        request: ::buffa::bytes::Bytes,
        format: ::connectrpc::CodecFormat,
    ) -> ::connectrpc::dispatcher::codegen::StreamingResult {
        let Some(method) = path.strip_prefix("mavio.cloud.v1.ServerService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_streaming(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            _ => ::connectrpc::dispatcher::codegen::unimplemented_streaming(path),
        }
    }
    fn call_client_streaming(
        &self,
        path: &str,
        ctx: ::connectrpc::RequestContext,
        requests: ::connectrpc::dispatcher::codegen::RequestStream,
        format: ::connectrpc::CodecFormat,
    ) -> ::connectrpc::dispatcher::codegen::UnaryResult {
        let Some(method) = path.strip_prefix("mavio.cloud.v1.ServerService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &requests, &format);
        match method {
            _ => ::connectrpc::dispatcher::codegen::unimplemented_unary(path),
        }
    }
    fn call_bidi_streaming(
        &self,
        path: &str,
        ctx: ::connectrpc::RequestContext,
        requests: ::connectrpc::dispatcher::codegen::RequestStream,
        format: ::connectrpc::CodecFormat,
    ) -> ::connectrpc::dispatcher::codegen::StreamingResult {
        let Some(method) = path.strip_prefix("mavio.cloud.v1.ServerService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_streaming(path);
        };
        let _ = (&ctx, &requests, &format);
        match method {
            _ => ::connectrpc::dispatcher::codegen::unimplemented_streaming(path),
        }
    }
}
/// Client for this service.
///
/// Generic over `T: ClientTransport`. For **gRPC** (HTTP/2), use
/// `Http2Connection` — it has honest `poll_ready` and composes with
/// `tower::balance` for multi-connection load balancing. For **Connect
/// over HTTP/1.1** (or unknown protocol), use `HttpClient`.
///
/// # Example (gRPC / HTTP/2)
///
/// ```rust,ignore
/// use connectrpc::client::{Http2Connection, ClientConfig};
/// use connectrpc::Protocol;
///
/// let uri: http::Uri = "http://localhost:8080".parse()?;
/// let conn = Http2Connection::connect_plaintext(uri.clone()).await?.shared(1024);
/// let config = ClientConfig::new(uri).with_protocol(Protocol::Grpc);
///
/// let client = ServerServiceClient::new(conn, config);
/// let response = client.claim_server(request).await?;
/// ```
///
/// # Example (Connect / HTTP/1.1 or ALPN)
///
/// ```rust,ignore
/// use connectrpc::client::{HttpClient, ClientConfig};
///
/// let http = HttpClient::plaintext();  // cleartext http:// only
/// let config = ClientConfig::new("http://localhost:8080".parse()?);
///
/// let client = ServerServiceClient::new(http, config);
/// let response = client.claim_server(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.claim_server(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.claim_server(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct ServerServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> ServerServiceClient<T>
where
    T: ::connectrpc::client::ClientTransport,
    <T::ResponseBody as ::connectrpc::http_body::Body>::Error: ::std::fmt::Display,
{
    /// Create a new client with the given transport and configuration.
    pub fn new(transport: T, config: ::connectrpc::client::ClientConfig) -> Self {
        Self { transport, config }
    }
    /// Get the client configuration.
    pub fn config(&self) -> &::connectrpc::client::ClientConfig {
        &self.config
    }
    /// Get a mutable reference to the client configuration.
    pub fn config_mut(&mut self) -> &mut ::connectrpc::client::ClientConfig {
        &mut self.config
    }
    /// Call the ClaimServer RPC. Sends a request to /mavio.cloud.v1.ServerService/ClaimServer.
    pub async fn claim_server(
        &self,
        request: crate::proto::mavio::cloud::v1::ClaimServerRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.claim_server_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ClaimServer RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn claim_server_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::ClaimServerRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_CLAIM_SERVER_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListChanges RPC. Sends a request to /mavio.cloud.v1.ServerService/ListChanges.
    pub async fn list_changes(
        &self,
        request: crate::proto::mavio::cloud::v1::ListChangesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_changes_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListChanges RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_changes_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::ListChangesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_LIST_CHANGES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

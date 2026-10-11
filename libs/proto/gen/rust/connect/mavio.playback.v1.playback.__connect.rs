///Shorthand for `OwnedView<StartPlaybackRequestView<'static>>`.
pub type OwnedStartPlaybackRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::StartPlaybackRequestView<'static>,
>;
///Shorthand for `OwnedView<StartPlaybackResponseView<'static>>`.
pub type OwnedStartPlaybackResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::StartPlaybackResponseView<'static>,
>;
///Shorthand for `OwnedView<ReportProgressRequestView<'static>>`.
pub type OwnedReportProgressRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::ReportProgressRequestView<'static>,
>;
///Shorthand for `OwnedView<ReportProgressResponseView<'static>>`.
pub type OwnedReportProgressResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::ReportProgressResponseView<'static>,
>;
///Shorthand for `OwnedView<StopPlaybackRequestView<'static>>`.
pub type OwnedStopPlaybackRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::StopPlaybackRequestView<'static>,
>;
///Shorthand for `OwnedView<StopPlaybackResponseView<'static>>`.
pub type OwnedStopPlaybackResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::StopPlaybackResponseView<'static>,
>;
///Shorthand for `OwnedView<ListIntrosRequestView<'static>>`.
pub type OwnedListIntrosRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::ListIntrosRequestView<'static>,
>;
///Shorthand for `OwnedView<ListIntrosResponseView<'static>>`.
pub type OwnedListIntrosResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::ListIntrosResponseView<'static>,
>;
///Shorthand for `OwnedView<ListMediaSourcesRequestView<'static>>`.
pub type OwnedListMediaSourcesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::ListMediaSourcesRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListMediaSourcesResponseView<'static>>`.
pub type OwnedListMediaSourcesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::ListMediaSourcesResponseView<
        'static,
    >,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::StartPlaybackResponse>
for crate::proto::mavio::playback::v1::__buffa::view::StartPlaybackResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::StartPlaybackResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::StartPlaybackResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::ReportProgressResponse>
for crate::proto::mavio::playback::v1::__buffa::view::ReportProgressResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::ReportProgressResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::ReportProgressResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::StopPlaybackResponse>
for crate::proto::mavio::playback::v1::__buffa::view::StopPlaybackResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::StopPlaybackResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::StopPlaybackResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::ListIntrosResponse>
for crate::proto::mavio::playback::v1::__buffa::view::ListIntrosResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::ListIntrosResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::ListIntrosResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::ListMediaSourcesResponse>
for crate::proto::mavio::playback::v1::__buffa::view::ListMediaSourcesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::playback::v1::ListMediaSourcesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::playback::v1::__buffa::view::ListMediaSourcesResponseView<
        'static,
    >,
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
pub const PLAYBACK_SERVICE_SERVICE_NAME: &str = "mavio.playback.v1.PlaybackService";
/// Static [`Spec`](::connectrpc::Spec) for the `StartPlayback` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYBACK_SERVICE_START_PLAYBACK_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.playback.v1.PlaybackService/StartPlayback",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ReportProgress` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYBACK_SERVICE_REPORT_PROGRESS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.playback.v1.PlaybackService/ReportProgress",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `StopPlayback` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYBACK_SERVICE_STOP_PLAYBACK_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.playback.v1.PlaybackService/StopPlayback",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListIntros` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYBACK_SERVICE_LIST_INTROS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.playback.v1.PlaybackService/ListIntros",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListMediaSources` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYBACK_SERVICE_LIST_MEDIA_SOURCES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.playback.v1.PlaybackService/ListMediaSources",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// PlaybackService decides how a client plays an item and tracks the
/// playback. All methods act for the calling user.
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
pub trait PlaybackService: Send + Sync + 'static {
    /// StartPlayback picks a media source and streams, decides whether the
    /// client plays it directly, remuxed or transcoded, and returns the URLs to
    /// play it from. The URLs need no token and work until the playback stops,
    /// expires or its sign-in session ends.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn start_playback<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::playback::v1::StartPlaybackRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::playback::v1::StartPlaybackResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ReportProgress records the position of a playback. Playbacks without
    /// reports or media requests for a while expire.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn report_progress<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::playback::v1::ReportProgressRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::playback::v1::ReportProgressResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// StopPlayback ends a playback and saves the resume position or marks the
    /// item played.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn stop_playback<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::playback::v1::StopPlaybackRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::playback::v1::StopPlaybackResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListIntros returns the items to play before an item, such as its
    /// trailers, as the intro provider plugins pick them; only items the user
    /// can play.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_intros<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::playback::v1::ListIntrosRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::playback::v1::ListIntrosResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListMediaSources returns the media sources an item plays from: its
    /// files, as GetItem lists them, then those the media source provider
    /// plugins give, probed. StartPlayback plays one by its ID.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_media_sources<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::playback::v1::ListMediaSourcesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::playback::v1::ListMediaSourcesResponse,
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
pub trait PlaybackServiceExt: PlaybackService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: PlaybackService> PlaybackServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view(
                PLAYBACK_SERVICE_SERVICE_NAME,
                "StartPlayback",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::playback::v1::__buffa::view::StartPlaybackRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::playback::v1::StartPlaybackRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.start_playback(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::playback::v1::StartPlaybackResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYBACK_SERVICE_START_PLAYBACK_SPEC)
            .route_view(
                PLAYBACK_SERVICE_SERVICE_NAME,
                "ReportProgress",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::playback::v1::__buffa::view::ReportProgressRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::playback::v1::ReportProgressRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.report_progress(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::playback::v1::ReportProgressResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYBACK_SERVICE_REPORT_PROGRESS_SPEC)
            .route_view(
                PLAYBACK_SERVICE_SERVICE_NAME,
                "StopPlayback",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::playback::v1::__buffa::view::StopPlaybackRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::playback::v1::StopPlaybackRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.stop_playback(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::playback::v1::StopPlaybackResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYBACK_SERVICE_STOP_PLAYBACK_SPEC)
            .route_view_idempotent(
                PLAYBACK_SERVICE_SERVICE_NAME,
                "ListIntros",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::playback::v1::__buffa::view::ListIntrosRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::playback::v1::ListIntrosRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_intros(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::playback::v1::ListIntrosResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYBACK_SERVICE_LIST_INTROS_SPEC)
            .route_view_idempotent(
                PLAYBACK_SERVICE_SERVICE_NAME,
                "ListMediaSources",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::playback::v1::__buffa::view::ListMediaSourcesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::playback::v1::ListMediaSourcesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_media_sources(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::playback::v1::ListMediaSourcesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYBACK_SERVICE_LIST_MEDIA_SOURCES_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct PlaybackServiceRegisterMarker;
impl<S: PlaybackService> ::connectrpc::ServiceRegister<PlaybackServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as PlaybackServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `PlaybackService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = PlaybackServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct PlaybackServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: PlaybackService> PlaybackServiceServer<T> {
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
impl<T> Clone for PlaybackServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: PlaybackService> ::connectrpc::Dispatcher for PlaybackServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.playback.v1.PlaybackService/")?;
        match method {
            "StartPlayback" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(PLAYBACK_SERVICE_START_PLAYBACK_SPEC),
                )
            }
            "ReportProgress" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(PLAYBACK_SERVICE_REPORT_PROGRESS_SPEC),
                )
            }
            "StopPlayback" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(PLAYBACK_SERVICE_STOP_PLAYBACK_SPEC),
                )
            }
            "ListIntros" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(PLAYBACK_SERVICE_LIST_INTROS_SPEC),
                )
            }
            "ListMediaSources" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(PLAYBACK_SERVICE_LIST_MEDIA_SOURCES_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.playback.v1.PlaybackService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "StartPlayback" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::playback::v1::StartPlaybackRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::playback::v1::__buffa::view::StartPlaybackRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::playback::v1::StartPlaybackRequest,
                    >::from_parts(&req, &body);
                    svc.start_playback(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::playback::v1::StartPlaybackResponse,
                        >(format)
                })
            }
            "ReportProgress" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::playback::v1::ReportProgressRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::playback::v1::__buffa::view::ReportProgressRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::playback::v1::ReportProgressRequest,
                    >::from_parts(&req, &body);
                    svc.report_progress(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::playback::v1::ReportProgressResponse,
                        >(format)
                })
            }
            "StopPlayback" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::playback::v1::StopPlaybackRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::playback::v1::__buffa::view::StopPlaybackRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::playback::v1::StopPlaybackRequest,
                    >::from_parts(&req, &body);
                    svc.stop_playback(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::playback::v1::StopPlaybackResponse,
                        >(format)
                })
            }
            "ListIntros" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::playback::v1::ListIntrosRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::playback::v1::__buffa::view::ListIntrosRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::playback::v1::ListIntrosRequest,
                    >::from_parts(&req, &body);
                    svc.list_intros(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::playback::v1::ListIntrosResponse,
                        >(format)
                })
            }
            "ListMediaSources" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::playback::v1::ListMediaSourcesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::playback::v1::__buffa::view::ListMediaSourcesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::playback::v1::ListMediaSourcesRequest,
                    >::from_parts(&req, &body);
                    svc.list_media_sources(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::playback::v1::ListMediaSourcesResponse,
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
        let Some(method) = path.strip_prefix("mavio.playback.v1.PlaybackService/") else {
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
        let Some(method) = path.strip_prefix("mavio.playback.v1.PlaybackService/") else {
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
        let Some(method) = path.strip_prefix("mavio.playback.v1.PlaybackService/") else {
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
/// let client = PlaybackServiceClient::new(conn, config);
/// let response = client.start_playback(request).await?;
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
/// let client = PlaybackServiceClient::new(http, config);
/// let response = client.start_playback(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.start_playback(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.start_playback(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct PlaybackServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> PlaybackServiceClient<T>
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
    /// Call the StartPlayback RPC. Sends a request to /mavio.playback.v1.PlaybackService/StartPlayback.
    pub async fn start_playback(
        &self,
        request: crate::proto::mavio::playback::v1::StartPlaybackRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::StartPlaybackResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.start_playback_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the StartPlayback RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn start_playback_with_options(
        &self,
        request: crate::proto::mavio::playback::v1::StartPlaybackRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::StartPlaybackResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYBACK_SERVICE_START_PLAYBACK_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ReportProgress RPC. Sends a request to /mavio.playback.v1.PlaybackService/ReportProgress.
    pub async fn report_progress(
        &self,
        request: crate::proto::mavio::playback::v1::ReportProgressRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::ReportProgressResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.report_progress_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ReportProgress RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn report_progress_with_options(
        &self,
        request: crate::proto::mavio::playback::v1::ReportProgressRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::ReportProgressResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYBACK_SERVICE_REPORT_PROGRESS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the StopPlayback RPC. Sends a request to /mavio.playback.v1.PlaybackService/StopPlayback.
    pub async fn stop_playback(
        &self,
        request: crate::proto::mavio::playback::v1::StopPlaybackRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::StopPlaybackResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.stop_playback_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the StopPlayback RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn stop_playback_with_options(
        &self,
        request: crate::proto::mavio::playback::v1::StopPlaybackRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::StopPlaybackResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYBACK_SERVICE_STOP_PLAYBACK_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListIntros RPC. Sends a request to /mavio.playback.v1.PlaybackService/ListIntros.
    pub async fn list_intros(
        &self,
        request: crate::proto::mavio::playback::v1::ListIntrosRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::ListIntrosResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_intros_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListIntros RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_intros_with_options(
        &self,
        request: crate::proto::mavio::playback::v1::ListIntrosRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::ListIntrosResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYBACK_SERVICE_LIST_INTROS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListMediaSources RPC. Sends a request to /mavio.playback.v1.PlaybackService/ListMediaSources.
    pub async fn list_media_sources(
        &self,
        request: crate::proto::mavio::playback::v1::ListMediaSourcesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::ListMediaSourcesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_media_sources_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListMediaSources RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_media_sources_with_options(
        &self,
        request: crate::proto::mavio::playback::v1::ListMediaSourcesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::playback::v1::__buffa::view::ListMediaSourcesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYBACK_SERVICE_LIST_MEDIA_SOURCES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

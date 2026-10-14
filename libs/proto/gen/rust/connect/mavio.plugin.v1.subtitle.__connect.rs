///Shorthand for `OwnedView<ListSubtitleProvidersRequestView<'static>>`.
pub type OwnedListSubtitleProvidersRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleProvidersRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListSubtitleProvidersResponseView<'static>>`.
pub type OwnedListSubtitleProvidersResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleProvidersResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<SearchSubtitlesRequestView<'static>>`.
pub type OwnedSearchSubtitlesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::SearchSubtitlesRequestView<'static>,
>;
///Shorthand for `OwnedView<SearchSubtitlesResponseView<'static>>`.
pub type OwnedSearchSubtitlesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::SearchSubtitlesResponseView<'static>,
>;
///Shorthand for `OwnedView<DownloadSubtitleRequestView<'static>>`.
pub type OwnedDownloadSubtitleRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::DownloadSubtitleRequestView<'static>,
>;
///Shorthand for `OwnedView<DownloadSubtitleResponseView<'static>>`.
pub type OwnedDownloadSubtitleResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::DownloadSubtitleResponseView<'static>,
>;
///Shorthand for `OwnedView<ListSubtitleTranslatorsRequestView<'static>>`.
pub type OwnedListSubtitleTranslatorsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleTranslatorsRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListSubtitleTranslatorsResponseView<'static>>`.
pub type OwnedListSubtitleTranslatorsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleTranslatorsResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<TranslateSubtitleLinesRequestView<'static>>`.
pub type OwnedTranslateSubtitleLinesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::TranslateSubtitleLinesRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<TranslateSubtitleLinesResponseView<'static>>`.
pub type OwnedTranslateSubtitleLinesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::TranslateSubtitleLinesResponseView<
        'static,
    >,
>;
impl ::connectrpc::Encodable<
    crate::proto::mavio::plugin::v1::ListSubtitleProvidersResponse,
>
for crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleProvidersResponseView<
    '_,
> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<
    crate::proto::mavio::plugin::v1::ListSubtitleProvidersResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleProvidersResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::SearchSubtitlesResponse>
for crate::proto::mavio::plugin::v1::__buffa::view::SearchSubtitlesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::SearchSubtitlesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::SearchSubtitlesResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::DownloadSubtitleResponse>
for crate::proto::mavio::plugin::v1::__buffa::view::DownloadSubtitleResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::DownloadSubtitleResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::DownloadSubtitleResponseView<'static>,
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
impl ::connectrpc::Encodable<
    crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsResponse,
>
for crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleTranslatorsResponseView<
    '_,
> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<
    crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleTranslatorsResponseView<
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
impl ::connectrpc::Encodable<
    crate::proto::mavio::plugin::v1::TranslateSubtitleLinesResponse,
>
for crate::proto::mavio::plugin::v1::__buffa::view::TranslateSubtitleLinesResponseView<
    '_,
> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<
    crate::proto::mavio::plugin::v1::TranslateSubtitleLinesResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::TranslateSubtitleLinesResponseView<
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
pub const SUBTITLE_PROVIDER_SERVICE_SERVICE_NAME: &str = "mavio.plugin.v1.SubtitleProviderService";
/// Static [`Spec`](::connectrpc::Spec) for the `ListSubtitleProviders` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SUBTITLE_PROVIDER_SERVICE_LIST_SUBTITLE_PROVIDERS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.SubtitleProviderService/ListSubtitleProviders",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `SearchSubtitles` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SUBTITLE_PROVIDER_SERVICE_SEARCH_SUBTITLES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.SubtitleProviderService/SearchSubtitles",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `DownloadSubtitle` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SUBTITLE_PROVIDER_SERVICE_DOWNLOAD_SUBTITLE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.SubtitleProviderService/DownloadSubtitle",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// SubtitleProviderService finds and downloads subtitles from the subtitle
/// sites a plugin implements, as Bazarr's providers do. The server scores
/// the candidates it returns from the properties they match.
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
pub trait SubtitleProviderService: Send + Sync + 'static {
    /// ListSubtitleProviders lists the providers the plugin implements, with
    /// their settings and state.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_subtitle_providers<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::ListSubtitleProvidersRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::ListSubtitleProvidersResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SearchSubtitles asks the enabled providers for subtitles of a video in
    /// several languages.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn search_subtitles<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::SearchSubtitlesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::SearchSubtitlesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// DownloadSubtitle returns a subtitle found by SearchSubtitles,
    /// extracted from its archive.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn download_subtitle<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::DownloadSubtitleRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::DownloadSubtitleResponse,
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
pub trait SubtitleProviderServiceExt: SubtitleProviderService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: SubtitleProviderService> SubtitleProviderServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                SUBTITLE_PROVIDER_SERVICE_SERVICE_NAME,
                "ListSubtitleProviders",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleProvidersRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::ListSubtitleProvidersRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_subtitle_providers(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::ListSubtitleProvidersResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SUBTITLE_PROVIDER_SERVICE_LIST_SUBTITLE_PROVIDERS_SPEC)
            .route_view_idempotent(
                SUBTITLE_PROVIDER_SERVICE_SERVICE_NAME,
                "SearchSubtitles",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::SearchSubtitlesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::SearchSubtitlesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.search_subtitles(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::SearchSubtitlesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SUBTITLE_PROVIDER_SERVICE_SEARCH_SUBTITLES_SPEC)
            .route_view(
                SUBTITLE_PROVIDER_SERVICE_SERVICE_NAME,
                "DownloadSubtitle",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::DownloadSubtitleRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::DownloadSubtitleRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.download_subtitle(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::DownloadSubtitleResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SUBTITLE_PROVIDER_SERVICE_DOWNLOAD_SUBTITLE_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct SubtitleProviderServiceRegisterMarker;
impl<
    S: SubtitleProviderService,
> ::connectrpc::ServiceRegister<SubtitleProviderServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as SubtitleProviderServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `SubtitleProviderService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = SubtitleProviderServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct SubtitleProviderServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: SubtitleProviderService> SubtitleProviderServiceServer<T> {
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
impl<T> Clone for SubtitleProviderServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: SubtitleProviderService> ::connectrpc::Dispatcher
for SubtitleProviderServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.plugin.v1.SubtitleProviderService/")?;
        match method {
            "ListSubtitleProviders" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(
                            SUBTITLE_PROVIDER_SERVICE_LIST_SUBTITLE_PROVIDERS_SPEC,
                        ),
                )
            }
            "SearchSubtitles" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SUBTITLE_PROVIDER_SERVICE_SEARCH_SUBTITLES_SPEC),
                )
            }
            "DownloadSubtitle" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SUBTITLE_PROVIDER_SERVICE_DOWNLOAD_SUBTITLE_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.SubtitleProviderService/")
        else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "ListSubtitleProviders" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::ListSubtitleProvidersRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleProvidersRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::ListSubtitleProvidersRequest,
                    >::from_parts(&req, &body);
                    svc.list_subtitle_providers(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::ListSubtitleProvidersResponse,
                        >(format)
                })
            }
            "SearchSubtitles" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::SearchSubtitlesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::SearchSubtitlesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::SearchSubtitlesRequest,
                    >::from_parts(&req, &body);
                    svc.search_subtitles(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::SearchSubtitlesResponse,
                        >(format)
                })
            }
            "DownloadSubtitle" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::DownloadSubtitleRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::DownloadSubtitleRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::DownloadSubtitleRequest,
                    >::from_parts(&req, &body);
                    svc.download_subtitle(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::DownloadSubtitleResponse,
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.SubtitleProviderService/")
        else {
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.SubtitleProviderService/")
        else {
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.SubtitleProviderService/")
        else {
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
/// let client = SubtitleProviderServiceClient::new(conn, config);
/// let response = client.list_subtitle_providers(request).await?;
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
/// let client = SubtitleProviderServiceClient::new(http, config);
/// let response = client.list_subtitle_providers(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.list_subtitle_providers(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.list_subtitle_providers(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct SubtitleProviderServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> SubtitleProviderServiceClient<T>
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
    /// Call the ListSubtitleProviders RPC. Sends a request to /mavio.plugin.v1.SubtitleProviderService/ListSubtitleProviders.
    pub async fn list_subtitle_providers(
        &self,
        request: crate::proto::mavio::plugin::v1::ListSubtitleProvidersRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleProvidersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_subtitle_providers_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListSubtitleProviders RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_subtitle_providers_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::ListSubtitleProvidersRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleProvidersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SUBTITLE_PROVIDER_SERVICE_LIST_SUBTITLE_PROVIDERS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SearchSubtitles RPC. Sends a request to /mavio.plugin.v1.SubtitleProviderService/SearchSubtitles.
    pub async fn search_subtitles(
        &self,
        request: crate::proto::mavio::plugin::v1::SearchSubtitlesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::SearchSubtitlesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.search_subtitles_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SearchSubtitles RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn search_subtitles_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::SearchSubtitlesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::SearchSubtitlesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SUBTITLE_PROVIDER_SERVICE_SEARCH_SUBTITLES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DownloadSubtitle RPC. Sends a request to /mavio.plugin.v1.SubtitleProviderService/DownloadSubtitle.
    pub async fn download_subtitle(
        &self,
        request: crate::proto::mavio::plugin::v1::DownloadSubtitleRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::DownloadSubtitleResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.download_subtitle_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DownloadSubtitle RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn download_subtitle_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::DownloadSubtitleRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::DownloadSubtitleResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SUBTITLE_PROVIDER_SERVICE_DOWNLOAD_SUBTITLE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}
/// Full service name for this service.
pub const SUBTITLE_TRANSLATOR_SERVICE_SERVICE_NAME: &str = "mavio.plugin.v1.SubtitleTranslatorService";
/// Static [`Spec`](::connectrpc::Spec) for the `ListSubtitleTranslators` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SUBTITLE_TRANSLATOR_SERVICE_LIST_SUBTITLE_TRANSLATORS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.SubtitleTranslatorService/ListSubtitleTranslators",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `TranslateSubtitleLines` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SUBTITLE_TRANSLATOR_SERVICE_TRANSLATE_SUBTITLE_LINES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.SubtitleTranslatorService/TranslateSubtitleLines",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// SubtitleTranslatorService translates subtitles' cues with the
/// translation services a plugin implements, as Bazarr's translators do.
/// The server reads, times and writes the subtitles; translators take the
/// cues' text.
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
pub trait SubtitleTranslatorService: Send + Sync + 'static {
    /// ListSubtitleTranslators lists the translators the plugin implements,
    /// with their settings and state.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_subtitle_translators<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// TranslateSubtitleLines translates the text of cues of a subtitle.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn translate_subtitle_lines<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::TranslateSubtitleLinesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::TranslateSubtitleLinesResponse,
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
pub trait SubtitleTranslatorServiceExt: SubtitleTranslatorService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: SubtitleTranslatorService> SubtitleTranslatorServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                SUBTITLE_TRANSLATOR_SERVICE_SERVICE_NAME,
                "ListSubtitleTranslators",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleTranslatorsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_subtitle_translators(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SUBTITLE_TRANSLATOR_SERVICE_LIST_SUBTITLE_TRANSLATORS_SPEC)
            .route_view(
                SUBTITLE_TRANSLATOR_SERVICE_SERVICE_NAME,
                "TranslateSubtitleLines",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::TranslateSubtitleLinesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::TranslateSubtitleLinesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.translate_subtitle_lines(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::TranslateSubtitleLinesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SUBTITLE_TRANSLATOR_SERVICE_TRANSLATE_SUBTITLE_LINES_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct SubtitleTranslatorServiceRegisterMarker;
impl<
    S: SubtitleTranslatorService,
> ::connectrpc::ServiceRegister<SubtitleTranslatorServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as SubtitleTranslatorServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `SubtitleTranslatorService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = SubtitleTranslatorServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct SubtitleTranslatorServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: SubtitleTranslatorService> SubtitleTranslatorServiceServer<T> {
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
impl<T> Clone for SubtitleTranslatorServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: SubtitleTranslatorService> ::connectrpc::Dispatcher
for SubtitleTranslatorServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.plugin.v1.SubtitleTranslatorService/")?;
        match method {
            "ListSubtitleTranslators" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(
                            SUBTITLE_TRANSLATOR_SERVICE_LIST_SUBTITLE_TRANSLATORS_SPEC,
                        ),
                )
            }
            "TranslateSubtitleLines" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(
                            SUBTITLE_TRANSLATOR_SERVICE_TRANSLATE_SUBTITLE_LINES_SPEC,
                        ),
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
        let Some(method) = path
            .strip_prefix("mavio.plugin.v1.SubtitleTranslatorService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "ListSubtitleTranslators" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleTranslatorsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsRequest,
                    >::from_parts(&req, &body);
                    svc.list_subtitle_translators(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsResponse,
                        >(format)
                })
            }
            "TranslateSubtitleLines" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::TranslateSubtitleLinesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::TranslateSubtitleLinesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::TranslateSubtitleLinesRequest,
                    >::from_parts(&req, &body);
                    svc.translate_subtitle_lines(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::TranslateSubtitleLinesResponse,
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
        let Some(method) = path
            .strip_prefix("mavio.plugin.v1.SubtitleTranslatorService/") else {
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
        let Some(method) = path
            .strip_prefix("mavio.plugin.v1.SubtitleTranslatorService/") else {
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
        let Some(method) = path
            .strip_prefix("mavio.plugin.v1.SubtitleTranslatorService/") else {
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
/// let client = SubtitleTranslatorServiceClient::new(conn, config);
/// let response = client.list_subtitle_translators(request).await?;
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
/// let client = SubtitleTranslatorServiceClient::new(http, config);
/// let response = client.list_subtitle_translators(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.list_subtitle_translators(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.list_subtitle_translators(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct SubtitleTranslatorServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> SubtitleTranslatorServiceClient<T>
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
    /// Call the ListSubtitleTranslators RPC. Sends a request to /mavio.plugin.v1.SubtitleTranslatorService/ListSubtitleTranslators.
    pub async fn list_subtitle_translators(
        &self,
        request: crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleTranslatorsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_subtitle_translators_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListSubtitleTranslators RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_subtitle_translators_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::ListSubtitleTranslatorsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListSubtitleTranslatorsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SUBTITLE_TRANSLATOR_SERVICE_LIST_SUBTITLE_TRANSLATORS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the TranslateSubtitleLines RPC. Sends a request to /mavio.plugin.v1.SubtitleTranslatorService/TranslateSubtitleLines.
    pub async fn translate_subtitle_lines(
        &self,
        request: crate::proto::mavio::plugin::v1::TranslateSubtitleLinesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::TranslateSubtitleLinesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.translate_subtitle_lines_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the TranslateSubtitleLines RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn translate_subtitle_lines_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::TranslateSubtitleLinesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::TranslateSubtitleLinesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SUBTITLE_TRANSLATOR_SERVICE_TRANSLATE_SUBTITLE_LINES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

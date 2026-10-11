///Shorthand for `OwnedView<ListDefinitionsRequestView<'static>>`.
pub type OwnedListDefinitionsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListDefinitionsRequestView<'static>,
>;
///Shorthand for `OwnedView<ListDefinitionsResponseView<'static>>`.
pub type OwnedListDefinitionsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListDefinitionsResponseView<'static>,
>;
///Shorthand for `OwnedView<ListIndexersRequestView<'static>>`.
pub type OwnedListIndexersRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListIndexersRequestView<'static>,
>;
///Shorthand for `OwnedView<ListIndexersResponseView<'static>>`.
pub type OwnedListIndexersResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListIndexersResponseView<'static>,
>;
///Shorthand for `OwnedView<TestIndexerRequestView<'static>>`.
pub type OwnedTestIndexerRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::TestIndexerRequestView<'static>,
>;
///Shorthand for `OwnedView<TestIndexerResponseView<'static>>`.
pub type OwnedTestIndexerResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::TestIndexerResponseView<'static>,
>;
///Shorthand for `OwnedView<SearchReleasesRequestView<'static>>`.
pub type OwnedSearchReleasesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::SearchReleasesRequestView<'static>,
>;
///Shorthand for `OwnedView<SearchReleasesResponseView<'static>>`.
pub type OwnedSearchReleasesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::SearchReleasesResponseView<'static>,
>;
///Shorthand for `OwnedView<GetReleaseFileRequestView<'static>>`.
pub type OwnedGetReleaseFileRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::GetReleaseFileRequestView<'static>,
>;
///Shorthand for `OwnedView<GetReleaseFileResponseView<'static>>`.
pub type OwnedGetReleaseFileResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::GetReleaseFileResponseView<'static>,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::ListDefinitionsResponse>
for crate::proto::mavio::plugin::v1::__buffa::view::ListDefinitionsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::ListDefinitionsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListDefinitionsResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::ListIndexersResponse>
for crate::proto::mavio::plugin::v1::__buffa::view::ListIndexersResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::ListIndexersResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListIndexersResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::TestIndexerResponse>
for crate::proto::mavio::plugin::v1::__buffa::view::TestIndexerResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::TestIndexerResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::TestIndexerResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::SearchReleasesResponse>
for crate::proto::mavio::plugin::v1::__buffa::view::SearchReleasesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::SearchReleasesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::SearchReleasesResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::GetReleaseFileResponse>
for crate::proto::mavio::plugin::v1::__buffa::view::GetReleaseFileResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::GetReleaseFileResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::GetReleaseFileResponseView<'static>,
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
pub const INDEXER_PROVIDER_SERVICE_SERVICE_NAME: &str = "mavio.plugin.v1.IndexerProviderService";
/// Static [`Spec`](::connectrpc::Spec) for the `ListDefinitions` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const INDEXER_PROVIDER_SERVICE_LIST_DEFINITIONS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.IndexerProviderService/ListDefinitions",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListIndexers` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const INDEXER_PROVIDER_SERVICE_LIST_INDEXERS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.IndexerProviderService/ListIndexers",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `TestIndexer` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const INDEXER_PROVIDER_SERVICE_TEST_INDEXER_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.IndexerProviderService/TestIndexer",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `SearchReleases` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const INDEXER_PROVIDER_SERVICE_SEARCH_RELEASES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.IndexerProviderService/SearchReleases",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `GetReleaseFile` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const INDEXER_PROVIDER_SERVICE_GET_RELEASE_FILE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.IndexerProviderService/GetReleaseFile",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// IndexerProviderService searches indexers, the sites that list releases
/// as torrents or NZBs. The provider implements kinds of indexers, its
/// definitions, and the plugin's configuration lists the indexers
/// configured from them.
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
pub trait IndexerProviderService: Send + Sync + 'static {
    /// ListDefinitions lists the kinds of indexers the provider implements,
    /// with the settings each takes.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_definitions<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::ListDefinitionsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::ListDefinitionsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListIndexers lists the configured indexers with their capabilities.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_indexers<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::ListIndexersRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::ListIndexersResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// TestIndexer checks that an indexer, configured or not, answers a
    /// search with the settings given, signing in when it needs to.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn test_indexer<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::TestIndexerRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::TestIndexerResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SearchReleases searches indexers, or reads their recent releases when the
    /// request has no query, IDs, season or episode.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn search_releases<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::SearchReleasesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::SearchReleasesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetReleaseFile fetches a release's torrent file or NZB from its
    /// indexer, or the magnet link its download link leads to.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_release_file<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::GetReleaseFileRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::GetReleaseFileResponse,
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
pub trait IndexerProviderServiceExt: IndexerProviderService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: IndexerProviderService> IndexerProviderServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                INDEXER_PROVIDER_SERVICE_SERVICE_NAME,
                "ListDefinitions",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::ListDefinitionsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::ListDefinitionsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_definitions(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::ListDefinitionsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(INDEXER_PROVIDER_SERVICE_LIST_DEFINITIONS_SPEC)
            .route_view_idempotent(
                INDEXER_PROVIDER_SERVICE_SERVICE_NAME,
                "ListIndexers",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::ListIndexersRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::ListIndexersRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_indexers(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::ListIndexersResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(INDEXER_PROVIDER_SERVICE_LIST_INDEXERS_SPEC)
            .route_view(
                INDEXER_PROVIDER_SERVICE_SERVICE_NAME,
                "TestIndexer",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::TestIndexerRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::TestIndexerRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.test_indexer(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::TestIndexerResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(INDEXER_PROVIDER_SERVICE_TEST_INDEXER_SPEC)
            .route_view_idempotent(
                INDEXER_PROVIDER_SERVICE_SERVICE_NAME,
                "SearchReleases",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::SearchReleasesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::SearchReleasesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.search_releases(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::SearchReleasesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(INDEXER_PROVIDER_SERVICE_SEARCH_RELEASES_SPEC)
            .route_view(
                INDEXER_PROVIDER_SERVICE_SERVICE_NAME,
                "GetReleaseFile",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::GetReleaseFileRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::GetReleaseFileRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_release_file(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::GetReleaseFileResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(INDEXER_PROVIDER_SERVICE_GET_RELEASE_FILE_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct IndexerProviderServiceRegisterMarker;
impl<
    S: IndexerProviderService,
> ::connectrpc::ServiceRegister<IndexerProviderServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as IndexerProviderServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `IndexerProviderService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = IndexerProviderServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct IndexerProviderServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: IndexerProviderService> IndexerProviderServiceServer<T> {
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
impl<T> Clone for IndexerProviderServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: IndexerProviderService> ::connectrpc::Dispatcher
for IndexerProviderServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.plugin.v1.IndexerProviderService/")?;
        match method {
            "ListDefinitions" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(INDEXER_PROVIDER_SERVICE_LIST_DEFINITIONS_SPEC),
                )
            }
            "ListIndexers" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(INDEXER_PROVIDER_SERVICE_LIST_INDEXERS_SPEC),
                )
            }
            "TestIndexer" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(INDEXER_PROVIDER_SERVICE_TEST_INDEXER_SPEC),
                )
            }
            "SearchReleases" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(INDEXER_PROVIDER_SERVICE_SEARCH_RELEASES_SPEC),
                )
            }
            "GetReleaseFile" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(INDEXER_PROVIDER_SERVICE_GET_RELEASE_FILE_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.IndexerProviderService/")
        else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "ListDefinitions" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::ListDefinitionsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::ListDefinitionsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::ListDefinitionsRequest,
                    >::from_parts(&req, &body);
                    svc.list_definitions(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::ListDefinitionsResponse,
                        >(format)
                })
            }
            "ListIndexers" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::ListIndexersRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::ListIndexersRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::ListIndexersRequest,
                    >::from_parts(&req, &body);
                    svc.list_indexers(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::ListIndexersResponse,
                        >(format)
                })
            }
            "TestIndexer" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::TestIndexerRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::TestIndexerRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::TestIndexerRequest,
                    >::from_parts(&req, &body);
                    svc.test_indexer(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::TestIndexerResponse,
                        >(format)
                })
            }
            "SearchReleases" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::SearchReleasesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::SearchReleasesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::SearchReleasesRequest,
                    >::from_parts(&req, &body);
                    svc.search_releases(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::SearchReleasesResponse,
                        >(format)
                })
            }
            "GetReleaseFile" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::GetReleaseFileRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::GetReleaseFileRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::GetReleaseFileRequest,
                    >::from_parts(&req, &body);
                    svc.get_release_file(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::GetReleaseFileResponse,
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.IndexerProviderService/")
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.IndexerProviderService/")
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.IndexerProviderService/")
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
/// let client = IndexerProviderServiceClient::new(conn, config);
/// let response = client.list_definitions(request).await?;
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
/// let client = IndexerProviderServiceClient::new(http, config);
/// let response = client.list_definitions(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.list_definitions(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.list_definitions(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct IndexerProviderServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> IndexerProviderServiceClient<T>
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
    /// Call the ListDefinitions RPC. Sends a request to /mavio.plugin.v1.IndexerProviderService/ListDefinitions.
    pub async fn list_definitions(
        &self,
        request: crate::proto::mavio::plugin::v1::ListDefinitionsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListDefinitionsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_definitions_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListDefinitions RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_definitions_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::ListDefinitionsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListDefinitionsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                INDEXER_PROVIDER_SERVICE_LIST_DEFINITIONS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListIndexers RPC. Sends a request to /mavio.plugin.v1.IndexerProviderService/ListIndexers.
    pub async fn list_indexers(
        &self,
        request: crate::proto::mavio::plugin::v1::ListIndexersRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListIndexersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_indexers_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListIndexers RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_indexers_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::ListIndexersRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListIndexersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                INDEXER_PROVIDER_SERVICE_LIST_INDEXERS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the TestIndexer RPC. Sends a request to /mavio.plugin.v1.IndexerProviderService/TestIndexer.
    pub async fn test_indexer(
        &self,
        request: crate::proto::mavio::plugin::v1::TestIndexerRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::TestIndexerResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.test_indexer_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the TestIndexer RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn test_indexer_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::TestIndexerRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::TestIndexerResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                INDEXER_PROVIDER_SERVICE_TEST_INDEXER_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SearchReleases RPC. Sends a request to /mavio.plugin.v1.IndexerProviderService/SearchReleases.
    pub async fn search_releases(
        &self,
        request: crate::proto::mavio::plugin::v1::SearchReleasesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::SearchReleasesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.search_releases_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SearchReleases RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn search_releases_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::SearchReleasesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::SearchReleasesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                INDEXER_PROVIDER_SERVICE_SEARCH_RELEASES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetReleaseFile RPC. Sends a request to /mavio.plugin.v1.IndexerProviderService/GetReleaseFile.
    pub async fn get_release_file(
        &self,
        request: crate::proto::mavio::plugin::v1::GetReleaseFileRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::GetReleaseFileResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_release_file_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetReleaseFile RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_release_file_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::GetReleaseFileRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::GetReleaseFileResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                INDEXER_PROVIDER_SERVICE_GET_RELEASE_FILE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

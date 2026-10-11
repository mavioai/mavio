///Shorthand for `OwnedView<CreateCollectionRequestView<'static>>`.
pub type OwnedCreateCollectionRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::CreateCollectionRequestView<'static>,
>;
///Shorthand for `OwnedView<CreateCollectionResponseView<'static>>`.
pub type OwnedCreateCollectionResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::CreateCollectionResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<UpdateCollectionRequestView<'static>>`.
pub type OwnedUpdateCollectionRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::UpdateCollectionRequestView<'static>,
>;
///Shorthand for `OwnedView<UpdateCollectionResponseView<'static>>`.
pub type OwnedUpdateCollectionResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::UpdateCollectionResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DeleteCollectionRequestView<'static>>`.
pub type OwnedDeleteCollectionRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DeleteCollectionRequestView<'static>,
>;
///Shorthand for `OwnedView<DeleteCollectionResponseView<'static>>`.
pub type OwnedDeleteCollectionResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DeleteCollectionResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<AddToCollectionRequestView<'static>>`.
pub type OwnedAddToCollectionRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::AddToCollectionRequestView<'static>,
>;
///Shorthand for `OwnedView<AddToCollectionResponseView<'static>>`.
pub type OwnedAddToCollectionResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::AddToCollectionResponseView<'static>,
>;
///Shorthand for `OwnedView<RemoveFromCollectionRequestView<'static>>`.
pub type OwnedRemoveFromCollectionRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::RemoveFromCollectionRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<RemoveFromCollectionResponseView<'static>>`.
pub type OwnedRemoveFromCollectionResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::RemoveFromCollectionResponseView<
        'static,
    >,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::CreateCollectionResponse>
for crate::proto::mavio::library::v1::__buffa::view::CreateCollectionResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::CreateCollectionResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::CreateCollectionResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::UpdateCollectionResponse>
for crate::proto::mavio::library::v1::__buffa::view::UpdateCollectionResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::UpdateCollectionResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::UpdateCollectionResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::DeleteCollectionResponse>
for crate::proto::mavio::library::v1::__buffa::view::DeleteCollectionResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::DeleteCollectionResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DeleteCollectionResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::AddToCollectionResponse>
for crate::proto::mavio::library::v1::__buffa::view::AddToCollectionResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::AddToCollectionResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::AddToCollectionResponseView<'static>,
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
    crate::proto::mavio::library::v1::RemoveFromCollectionResponse,
>
for crate::proto::mavio::library::v1::__buffa::view::RemoveFromCollectionResponseView<
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
    crate::proto::mavio::library::v1::RemoveFromCollectionResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::RemoveFromCollectionResponseView<
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
pub const COLLECTION_SERVICE_SERVICE_NAME: &str = "mavio.library.v1.CollectionService";
/// Static [`Spec`](::connectrpc::Spec) for the `CreateCollection` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const COLLECTION_SERVICE_CREATE_COLLECTION_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.CollectionService/CreateCollection",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `UpdateCollection` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const COLLECTION_SERVICE_UPDATE_COLLECTION_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.CollectionService/UpdateCollection",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `DeleteCollection` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const COLLECTION_SERVICE_DELETE_COLLECTION_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.CollectionService/DeleteCollection",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `AddToCollection` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const COLLECTION_SERVICE_ADD_TO_COLLECTION_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.CollectionService/AddToCollection",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `RemoveFromCollection` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const COLLECTION_SERVICE_REMOVE_FROM_COLLECTION_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.CollectionService/RemoveFromCollection",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// CollectionService lets administrators curate collections: named sets of
/// items across libraries, such as a film series. Collections are items of
/// the collections library; users with access to it list them with
/// ItemService.ListItems and their members with its parent_id, seeing the
/// members their policy allows.
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
pub trait CollectionService: Send + Sync + 'static {
    /// Handle the CreateCollection RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn create_collection<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::CreateCollectionRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::CreateCollectionResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the UpdateCollection RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn update_collection<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::UpdateCollectionRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::UpdateCollectionResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the DeleteCollection RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn delete_collection<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::DeleteCollectionRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::DeleteCollectionResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// AddToCollection appends items not yet in the collection.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn add_to_collection<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::AddToCollectionRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::AddToCollectionResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the RemoveFromCollection RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn remove_from_collection<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::RemoveFromCollectionRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::RemoveFromCollectionResponse,
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
pub trait CollectionServiceExt: CollectionService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: CollectionService> CollectionServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view(
                COLLECTION_SERVICE_SERVICE_NAME,
                "CreateCollection",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::CreateCollectionRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::CreateCollectionRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.create_collection(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::CreateCollectionResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(COLLECTION_SERVICE_CREATE_COLLECTION_SPEC)
            .route_view(
                COLLECTION_SERVICE_SERVICE_NAME,
                "UpdateCollection",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::UpdateCollectionRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::UpdateCollectionRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.update_collection(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::UpdateCollectionResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(COLLECTION_SERVICE_UPDATE_COLLECTION_SPEC)
            .route_view(
                COLLECTION_SERVICE_SERVICE_NAME,
                "DeleteCollection",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::DeleteCollectionRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::DeleteCollectionRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.delete_collection(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::DeleteCollectionResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(COLLECTION_SERVICE_DELETE_COLLECTION_SPEC)
            .route_view(
                COLLECTION_SERVICE_SERVICE_NAME,
                "AddToCollection",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::AddToCollectionRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::AddToCollectionRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.add_to_collection(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::AddToCollectionResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(COLLECTION_SERVICE_ADD_TO_COLLECTION_SPEC)
            .route_view(
                COLLECTION_SERVICE_SERVICE_NAME,
                "RemoveFromCollection",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::RemoveFromCollectionRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::RemoveFromCollectionRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.remove_from_collection(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::RemoveFromCollectionResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(COLLECTION_SERVICE_REMOVE_FROM_COLLECTION_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct CollectionServiceRegisterMarker;
impl<S: CollectionService> ::connectrpc::ServiceRegister<CollectionServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as CollectionServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `CollectionService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = CollectionServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct CollectionServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: CollectionService> CollectionServiceServer<T> {
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
impl<T> Clone for CollectionServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: CollectionService> ::connectrpc::Dispatcher for CollectionServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.library.v1.CollectionService/")?;
        match method {
            "CreateCollection" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(COLLECTION_SERVICE_CREATE_COLLECTION_SPEC),
                )
            }
            "UpdateCollection" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(COLLECTION_SERVICE_UPDATE_COLLECTION_SPEC),
                )
            }
            "DeleteCollection" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(COLLECTION_SERVICE_DELETE_COLLECTION_SPEC),
                )
            }
            "AddToCollection" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(COLLECTION_SERVICE_ADD_TO_COLLECTION_SPEC),
                )
            }
            "RemoveFromCollection" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(COLLECTION_SERVICE_REMOVE_FROM_COLLECTION_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.library.v1.CollectionService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "CreateCollection" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::CreateCollectionRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::CreateCollectionRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::CreateCollectionRequest,
                    >::from_parts(&req, &body);
                    svc.create_collection(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::CreateCollectionResponse,
                        >(format)
                })
            }
            "UpdateCollection" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::UpdateCollectionRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::UpdateCollectionRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::UpdateCollectionRequest,
                    >::from_parts(&req, &body);
                    svc.update_collection(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::UpdateCollectionResponse,
                        >(format)
                })
            }
            "DeleteCollection" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::DeleteCollectionRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::DeleteCollectionRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::DeleteCollectionRequest,
                    >::from_parts(&req, &body);
                    svc.delete_collection(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::DeleteCollectionResponse,
                        >(format)
                })
            }
            "AddToCollection" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::AddToCollectionRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::AddToCollectionRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::AddToCollectionRequest,
                    >::from_parts(&req, &body);
                    svc.add_to_collection(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::AddToCollectionResponse,
                        >(format)
                })
            }
            "RemoveFromCollection" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::RemoveFromCollectionRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::RemoveFromCollectionRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::RemoveFromCollectionRequest,
                    >::from_parts(&req, &body);
                    svc.remove_from_collection(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::RemoveFromCollectionResponse,
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
        let Some(method) = path.strip_prefix("mavio.library.v1.CollectionService/") else {
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
        let Some(method) = path.strip_prefix("mavio.library.v1.CollectionService/") else {
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
        let Some(method) = path.strip_prefix("mavio.library.v1.CollectionService/") else {
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
/// let client = CollectionServiceClient::new(conn, config);
/// let response = client.create_collection(request).await?;
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
/// let client = CollectionServiceClient::new(http, config);
/// let response = client.create_collection(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.create_collection(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.create_collection(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct CollectionServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> CollectionServiceClient<T>
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
    /// Call the CreateCollection RPC. Sends a request to /mavio.library.v1.CollectionService/CreateCollection.
    pub async fn create_collection(
        &self,
        request: crate::proto::mavio::library::v1::CreateCollectionRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::CreateCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.create_collection_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the CreateCollection RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn create_collection_with_options(
        &self,
        request: crate::proto::mavio::library::v1::CreateCollectionRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::CreateCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                COLLECTION_SERVICE_CREATE_COLLECTION_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the UpdateCollection RPC. Sends a request to /mavio.library.v1.CollectionService/UpdateCollection.
    pub async fn update_collection(
        &self,
        request: crate::proto::mavio::library::v1::UpdateCollectionRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::UpdateCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.update_collection_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the UpdateCollection RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn update_collection_with_options(
        &self,
        request: crate::proto::mavio::library::v1::UpdateCollectionRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::UpdateCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                COLLECTION_SERVICE_UPDATE_COLLECTION_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DeleteCollection RPC. Sends a request to /mavio.library.v1.CollectionService/DeleteCollection.
    pub async fn delete_collection(
        &self,
        request: crate::proto::mavio::library::v1::DeleteCollectionRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DeleteCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.delete_collection_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DeleteCollection RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn delete_collection_with_options(
        &self,
        request: crate::proto::mavio::library::v1::DeleteCollectionRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DeleteCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                COLLECTION_SERVICE_DELETE_COLLECTION_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the AddToCollection RPC. Sends a request to /mavio.library.v1.CollectionService/AddToCollection.
    pub async fn add_to_collection(
        &self,
        request: crate::proto::mavio::library::v1::AddToCollectionRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::AddToCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.add_to_collection_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the AddToCollection RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn add_to_collection_with_options(
        &self,
        request: crate::proto::mavio::library::v1::AddToCollectionRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::AddToCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                COLLECTION_SERVICE_ADD_TO_COLLECTION_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the RemoveFromCollection RPC. Sends a request to /mavio.library.v1.CollectionService/RemoveFromCollection.
    pub async fn remove_from_collection(
        &self,
        request: crate::proto::mavio::library::v1::RemoveFromCollectionRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::RemoveFromCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.remove_from_collection_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the RemoveFromCollection RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn remove_from_collection_with_options(
        &self,
        request: crate::proto::mavio::library::v1::RemoveFromCollectionRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::RemoveFromCollectionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                COLLECTION_SERVICE_REMOVE_FROM_COLLECTION_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

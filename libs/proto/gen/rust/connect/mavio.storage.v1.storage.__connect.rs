///Shorthand for `OwnedView<GetStatusRequestView<'static>>`.
pub type OwnedGetStatusRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::GetStatusRequestView<'static>,
>;
///Shorthand for `OwnedView<GetStatusResponseView<'static>>`.
pub type OwnedGetStatusResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::GetStatusResponseView<'static>,
>;
///Shorthand for `OwnedView<ListProvidersRequestView<'static>>`.
pub type OwnedListProvidersRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ListProvidersRequestView<'static>,
>;
///Shorthand for `OwnedView<ListProvidersResponseView<'static>>`.
pub type OwnedListProvidersResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ListProvidersResponseView<'static>,
>;
///Shorthand for `OwnedView<ListRemotesRequestView<'static>>`.
pub type OwnedListRemotesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ListRemotesRequestView<'static>,
>;
///Shorthand for `OwnedView<ListRemotesResponseView<'static>>`.
pub type OwnedListRemotesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ListRemotesResponseView<'static>,
>;
///Shorthand for `OwnedView<CreateRemoteRequestView<'static>>`.
pub type OwnedCreateRemoteRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::CreateRemoteRequestView<'static>,
>;
///Shorthand for `OwnedView<CreateRemoteResponseView<'static>>`.
pub type OwnedCreateRemoteResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::CreateRemoteResponseView<'static>,
>;
///Shorthand for `OwnedView<UpdateRemoteRequestView<'static>>`.
pub type OwnedUpdateRemoteRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::UpdateRemoteRequestView<'static>,
>;
///Shorthand for `OwnedView<UpdateRemoteResponseView<'static>>`.
pub type OwnedUpdateRemoteResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::UpdateRemoteResponseView<'static>,
>;
///Shorthand for `OwnedView<ContinueRemoteRequestView<'static>>`.
pub type OwnedContinueRemoteRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ContinueRemoteRequestView<'static>,
>;
///Shorthand for `OwnedView<ContinueRemoteResponseView<'static>>`.
pub type OwnedContinueRemoteResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ContinueRemoteResponseView<'static>,
>;
///Shorthand for `OwnedView<DeleteRemoteRequestView<'static>>`.
pub type OwnedDeleteRemoteRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::DeleteRemoteRequestView<'static>,
>;
///Shorthand for `OwnedView<DeleteRemoteResponseView<'static>>`.
pub type OwnedDeleteRemoteResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::DeleteRemoteResponseView<'static>,
>;
///Shorthand for `OwnedView<ListFoldersRequestView<'static>>`.
pub type OwnedListFoldersRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ListFoldersRequestView<'static>,
>;
///Shorthand for `OwnedView<ListFoldersResponseView<'static>>`.
pub type OwnedListFoldersResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ListFoldersResponseView<'static>,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::GetStatusResponse>
for crate::proto::mavio::storage::v1::__buffa::view::GetStatusResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::GetStatusResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::GetStatusResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::ListProvidersResponse>
for crate::proto::mavio::storage::v1::__buffa::view::ListProvidersResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::ListProvidersResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ListProvidersResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::ListRemotesResponse>
for crate::proto::mavio::storage::v1::__buffa::view::ListRemotesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::ListRemotesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ListRemotesResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::CreateRemoteResponse>
for crate::proto::mavio::storage::v1::__buffa::view::CreateRemoteResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::CreateRemoteResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::CreateRemoteResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::UpdateRemoteResponse>
for crate::proto::mavio::storage::v1::__buffa::view::UpdateRemoteResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::UpdateRemoteResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::UpdateRemoteResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::ContinueRemoteResponse>
for crate::proto::mavio::storage::v1::__buffa::view::ContinueRemoteResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::ContinueRemoteResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ContinueRemoteResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::DeleteRemoteResponse>
for crate::proto::mavio::storage::v1::__buffa::view::DeleteRemoteResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::DeleteRemoteResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::DeleteRemoteResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::ListFoldersResponse>
for crate::proto::mavio::storage::v1::__buffa::view::ListFoldersResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::storage::v1::ListFoldersResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::storage::v1::__buffa::view::ListFoldersResponseView<'static>,
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
pub const STORAGE_SERVICE_SERVICE_NAME: &str = "mavio.storage.v1.StorageService";
/// Static [`Spec`](::connectrpc::Spec) for the `GetStatus` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const STORAGE_SERVICE_GET_STATUS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.storage.v1.StorageService/GetStatus",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListProviders` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const STORAGE_SERVICE_LIST_PROVIDERS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.storage.v1.StorageService/ListProviders",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListRemotes` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const STORAGE_SERVICE_LIST_REMOTES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.storage.v1.StorageService/ListRemotes",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `CreateRemote` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const STORAGE_SERVICE_CREATE_REMOTE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.storage.v1.StorageService/CreateRemote",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `UpdateRemote` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const STORAGE_SERVICE_UPDATE_REMOTE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.storage.v1.StorageService/UpdateRemote",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ContinueRemote` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const STORAGE_SERVICE_CONTINUE_REMOTE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.storage.v1.StorageService/ContinueRemote",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `DeleteRemote` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const STORAGE_SERVICE_DELETE_REMOTE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.storage.v1.StorageService/DeleteRemote",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListFolders` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const STORAGE_SERVICE_LIST_FOLDERS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.storage.v1.StorageService/ListFolders",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// StorageService configures the rclone remotes that library folders may lie
/// on, as "rclone:\<remote\>:\<path\>", through the rclone rcd the server runs or
/// reaches. Administrators only.
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
pub trait StorageService: Send + Sync + 'static {
    /// GetStatus tells whether the server reaches an rcd.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_status<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::storage::v1::GetStatusRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::storage::v1::GetStatusResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListProviders lists the kinds of remotes and the settings each takes,
    /// from which clients build their forms.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_providers<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::storage::v1::ListProvidersRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::storage::v1::ListProvidersResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListRemotes lists the configured remotes.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_remotes<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::storage::v1::ListRemotesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::storage::v1::ListRemotesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// CreateRemote configures a remote. Providers that need more, such as an
    /// OAuth token, ask a question, which ContinueRemote answers.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn create_remote<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::storage::v1::CreateRemoteRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::storage::v1::CreateRemoteResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// UpdateRemote changes settings of a remote.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn update_remote<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::storage::v1::UpdateRemoteRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::storage::v1::UpdateRemoteResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ContinueRemote answers the question of a remote's configuration.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn continue_remote<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::storage::v1::ContinueRemoteRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::storage::v1::ContinueRemoteResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// DeleteRemote removes a remote's configuration. Libraries on it become
    /// unreadable; their items are kept until a scan finds it again.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn delete_remote<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::storage::v1::DeleteRemoteRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::storage::v1::DeleteRemoteResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListFolders lists the folders of a folder of a remote, for choosing
    /// library folders.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_folders<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::storage::v1::ListFoldersRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::storage::v1::ListFoldersResponse,
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
pub trait StorageServiceExt: StorageService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: StorageService> StorageServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                STORAGE_SERVICE_SERVICE_NAME,
                "GetStatus",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::storage::v1::__buffa::view::GetStatusRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::storage::v1::GetStatusRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_status(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::storage::v1::GetStatusResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(STORAGE_SERVICE_GET_STATUS_SPEC)
            .route_view_idempotent(
                STORAGE_SERVICE_SERVICE_NAME,
                "ListProviders",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::storage::v1::__buffa::view::ListProvidersRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::storage::v1::ListProvidersRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_providers(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::storage::v1::ListProvidersResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(STORAGE_SERVICE_LIST_PROVIDERS_SPEC)
            .route_view_idempotent(
                STORAGE_SERVICE_SERVICE_NAME,
                "ListRemotes",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::storage::v1::__buffa::view::ListRemotesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::storage::v1::ListRemotesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_remotes(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::storage::v1::ListRemotesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(STORAGE_SERVICE_LIST_REMOTES_SPEC)
            .route_view(
                STORAGE_SERVICE_SERVICE_NAME,
                "CreateRemote",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::storage::v1::__buffa::view::CreateRemoteRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::storage::v1::CreateRemoteRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.create_remote(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::storage::v1::CreateRemoteResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(STORAGE_SERVICE_CREATE_REMOTE_SPEC)
            .route_view(
                STORAGE_SERVICE_SERVICE_NAME,
                "UpdateRemote",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::storage::v1::__buffa::view::UpdateRemoteRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::storage::v1::UpdateRemoteRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.update_remote(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::storage::v1::UpdateRemoteResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(STORAGE_SERVICE_UPDATE_REMOTE_SPEC)
            .route_view(
                STORAGE_SERVICE_SERVICE_NAME,
                "ContinueRemote",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::storage::v1::__buffa::view::ContinueRemoteRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::storage::v1::ContinueRemoteRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.continue_remote(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::storage::v1::ContinueRemoteResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(STORAGE_SERVICE_CONTINUE_REMOTE_SPEC)
            .route_view(
                STORAGE_SERVICE_SERVICE_NAME,
                "DeleteRemote",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::storage::v1::__buffa::view::DeleteRemoteRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::storage::v1::DeleteRemoteRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.delete_remote(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::storage::v1::DeleteRemoteResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(STORAGE_SERVICE_DELETE_REMOTE_SPEC)
            .route_view_idempotent(
                STORAGE_SERVICE_SERVICE_NAME,
                "ListFolders",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::storage::v1::__buffa::view::ListFoldersRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::storage::v1::ListFoldersRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_folders(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::storage::v1::ListFoldersResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(STORAGE_SERVICE_LIST_FOLDERS_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct StorageServiceRegisterMarker;
impl<S: StorageService> ::connectrpc::ServiceRegister<StorageServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as StorageServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `StorageService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = StorageServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct StorageServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: StorageService> StorageServiceServer<T> {
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
impl<T> Clone for StorageServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: StorageService> ::connectrpc::Dispatcher for StorageServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.storage.v1.StorageService/")?;
        match method {
            "GetStatus" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(STORAGE_SERVICE_GET_STATUS_SPEC),
                )
            }
            "ListProviders" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(STORAGE_SERVICE_LIST_PROVIDERS_SPEC),
                )
            }
            "ListRemotes" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(STORAGE_SERVICE_LIST_REMOTES_SPEC),
                )
            }
            "CreateRemote" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(STORAGE_SERVICE_CREATE_REMOTE_SPEC),
                )
            }
            "UpdateRemote" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(STORAGE_SERVICE_UPDATE_REMOTE_SPEC),
                )
            }
            "ContinueRemote" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(STORAGE_SERVICE_CONTINUE_REMOTE_SPEC),
                )
            }
            "DeleteRemote" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(STORAGE_SERVICE_DELETE_REMOTE_SPEC),
                )
            }
            "ListFolders" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(STORAGE_SERVICE_LIST_FOLDERS_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.storage.v1.StorageService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "GetStatus" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::storage::v1::GetStatusRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::storage::v1::__buffa::view::GetStatusRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::storage::v1::GetStatusRequest,
                    >::from_parts(&req, &body);
                    svc.get_status(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::storage::v1::GetStatusResponse,
                        >(format)
                })
            }
            "ListProviders" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::storage::v1::ListProvidersRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::storage::v1::__buffa::view::ListProvidersRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::storage::v1::ListProvidersRequest,
                    >::from_parts(&req, &body);
                    svc.list_providers(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::storage::v1::ListProvidersResponse,
                        >(format)
                })
            }
            "ListRemotes" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::storage::v1::ListRemotesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::storage::v1::__buffa::view::ListRemotesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::storage::v1::ListRemotesRequest,
                    >::from_parts(&req, &body);
                    svc.list_remotes(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::storage::v1::ListRemotesResponse,
                        >(format)
                })
            }
            "CreateRemote" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::storage::v1::CreateRemoteRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::storage::v1::__buffa::view::CreateRemoteRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::storage::v1::CreateRemoteRequest,
                    >::from_parts(&req, &body);
                    svc.create_remote(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::storage::v1::CreateRemoteResponse,
                        >(format)
                })
            }
            "UpdateRemote" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::storage::v1::UpdateRemoteRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::storage::v1::__buffa::view::UpdateRemoteRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::storage::v1::UpdateRemoteRequest,
                    >::from_parts(&req, &body);
                    svc.update_remote(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::storage::v1::UpdateRemoteResponse,
                        >(format)
                })
            }
            "ContinueRemote" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::storage::v1::ContinueRemoteRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::storage::v1::__buffa::view::ContinueRemoteRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::storage::v1::ContinueRemoteRequest,
                    >::from_parts(&req, &body);
                    svc.continue_remote(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::storage::v1::ContinueRemoteResponse,
                        >(format)
                })
            }
            "DeleteRemote" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::storage::v1::DeleteRemoteRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::storage::v1::__buffa::view::DeleteRemoteRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::storage::v1::DeleteRemoteRequest,
                    >::from_parts(&req, &body);
                    svc.delete_remote(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::storage::v1::DeleteRemoteResponse,
                        >(format)
                })
            }
            "ListFolders" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::storage::v1::ListFoldersRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::storage::v1::__buffa::view::ListFoldersRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::storage::v1::ListFoldersRequest,
                    >::from_parts(&req, &body);
                    svc.list_folders(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::storage::v1::ListFoldersResponse,
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
        let Some(method) = path.strip_prefix("mavio.storage.v1.StorageService/") else {
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
        let Some(method) = path.strip_prefix("mavio.storage.v1.StorageService/") else {
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
        let Some(method) = path.strip_prefix("mavio.storage.v1.StorageService/") else {
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
/// let client = StorageServiceClient::new(conn, config);
/// let response = client.get_status(request).await?;
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
/// let client = StorageServiceClient::new(http, config);
/// let response = client.get_status(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.get_status(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.get_status(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct StorageServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> StorageServiceClient<T>
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
    /// Call the GetStatus RPC. Sends a request to /mavio.storage.v1.StorageService/GetStatus.
    pub async fn get_status(
        &self,
        request: crate::proto::mavio::storage::v1::GetStatusRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::GetStatusResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_status_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetStatus RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_status_with_options(
        &self,
        request: crate::proto::mavio::storage::v1::GetStatusRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::GetStatusResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                STORAGE_SERVICE_GET_STATUS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListProviders RPC. Sends a request to /mavio.storage.v1.StorageService/ListProviders.
    pub async fn list_providers(
        &self,
        request: crate::proto::mavio::storage::v1::ListProvidersRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::ListProvidersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_providers_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListProviders RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_providers_with_options(
        &self,
        request: crate::proto::mavio::storage::v1::ListProvidersRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::ListProvidersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                STORAGE_SERVICE_LIST_PROVIDERS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListRemotes RPC. Sends a request to /mavio.storage.v1.StorageService/ListRemotes.
    pub async fn list_remotes(
        &self,
        request: crate::proto::mavio::storage::v1::ListRemotesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::ListRemotesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_remotes_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListRemotes RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_remotes_with_options(
        &self,
        request: crate::proto::mavio::storage::v1::ListRemotesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::ListRemotesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                STORAGE_SERVICE_LIST_REMOTES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the CreateRemote RPC. Sends a request to /mavio.storage.v1.StorageService/CreateRemote.
    pub async fn create_remote(
        &self,
        request: crate::proto::mavio::storage::v1::CreateRemoteRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::CreateRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.create_remote_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the CreateRemote RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn create_remote_with_options(
        &self,
        request: crate::proto::mavio::storage::v1::CreateRemoteRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::CreateRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                STORAGE_SERVICE_CREATE_REMOTE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the UpdateRemote RPC. Sends a request to /mavio.storage.v1.StorageService/UpdateRemote.
    pub async fn update_remote(
        &self,
        request: crate::proto::mavio::storage::v1::UpdateRemoteRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::UpdateRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.update_remote_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the UpdateRemote RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn update_remote_with_options(
        &self,
        request: crate::proto::mavio::storage::v1::UpdateRemoteRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::UpdateRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                STORAGE_SERVICE_UPDATE_REMOTE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ContinueRemote RPC. Sends a request to /mavio.storage.v1.StorageService/ContinueRemote.
    pub async fn continue_remote(
        &self,
        request: crate::proto::mavio::storage::v1::ContinueRemoteRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::ContinueRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.continue_remote_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ContinueRemote RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn continue_remote_with_options(
        &self,
        request: crate::proto::mavio::storage::v1::ContinueRemoteRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::ContinueRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                STORAGE_SERVICE_CONTINUE_REMOTE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DeleteRemote RPC. Sends a request to /mavio.storage.v1.StorageService/DeleteRemote.
    pub async fn delete_remote(
        &self,
        request: crate::proto::mavio::storage::v1::DeleteRemoteRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::DeleteRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.delete_remote_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DeleteRemote RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn delete_remote_with_options(
        &self,
        request: crate::proto::mavio::storage::v1::DeleteRemoteRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::DeleteRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                STORAGE_SERVICE_DELETE_REMOTE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListFolders RPC. Sends a request to /mavio.storage.v1.StorageService/ListFolders.
    pub async fn list_folders(
        &self,
        request: crate::proto::mavio::storage::v1::ListFoldersRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::ListFoldersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_folders_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListFolders RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_folders_with_options(
        &self,
        request: crate::proto::mavio::storage::v1::ListFoldersRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::storage::v1::__buffa::view::ListFoldersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                STORAGE_SERVICE_LIST_FOLDERS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

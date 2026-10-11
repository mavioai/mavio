///Shorthand for `OwnedView<CreateRequestRequestView<'static>>`.
pub type OwnedCreateRequestRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::CreateRequestRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<CreateRequestResponseView<'static>>`.
pub type OwnedCreateRequestResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::CreateRequestResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListRequestsRequestView<'static>>`.
pub type OwnedListRequestsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListRequestsRequestView<'static>,
>;
///Shorthand for `OwnedView<ListRequestsResponseView<'static>>`.
pub type OwnedListRequestsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListRequestsResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<GetRequestRequestView<'static>>`.
pub type OwnedGetRequestRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetRequestRequestView<'static>,
>;
///Shorthand for `OwnedView<GetRequestResponseView<'static>>`.
pub type OwnedGetRequestResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetRequestResponseView<'static>,
>;
///Shorthand for `OwnedView<UpdateRequestRequestView<'static>>`.
pub type OwnedUpdateRequestRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::UpdateRequestRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<UpdateRequestResponseView<'static>>`.
pub type OwnedUpdateRequestResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::UpdateRequestResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DeleteRequestRequestView<'static>>`.
pub type OwnedDeleteRequestRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeleteRequestRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DeleteRequestResponseView<'static>>`.
pub type OwnedDeleteRequestResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeleteRequestResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ApproveRequestRequestView<'static>>`.
pub type OwnedApproveRequestRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ApproveRequestRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ApproveRequestResponseView<'static>>`.
pub type OwnedApproveRequestResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ApproveRequestResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DeclineRequestRequestView<'static>>`.
pub type OwnedDeclineRequestRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeclineRequestRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DeclineRequestResponseView<'static>>`.
pub type OwnedDeclineRequestResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeclineRequestResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<RetryRequestRequestView<'static>>`.
pub type OwnedRetryRequestRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::RetryRequestRequestView<'static>,
>;
///Shorthand for `OwnedView<RetryRequestResponseView<'static>>`.
pub type OwnedRetryRequestResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::RetryRequestResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<GetQuotaRequestView<'static>>`.
pub type OwnedGetQuotaRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetQuotaRequestView<'static>,
>;
///Shorthand for `OwnedView<GetQuotaResponseView<'static>>`.
pub type OwnedGetQuotaResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetQuotaResponseView<'static>,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::CreateRequestResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::CreateRequestResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::CreateRequestResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::CreateRequestResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListRequestsResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::ListRequestsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListRequestsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListRequestsResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GetRequestResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::GetRequestResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GetRequestResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetRequestResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::UpdateRequestResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::UpdateRequestResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::UpdateRequestResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::UpdateRequestResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::DeleteRequestResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::DeleteRequestResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::DeleteRequestResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeleteRequestResponseView<
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
    crate::proto::mavio::acquisition::v1::ApproveRequestResponse,
>
for crate::proto::mavio::acquisition::v1::__buffa::view::ApproveRequestResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<
    crate::proto::mavio::acquisition::v1::ApproveRequestResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ApproveRequestResponseView<
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
    crate::proto::mavio::acquisition::v1::DeclineRequestResponse,
>
for crate::proto::mavio::acquisition::v1::__buffa::view::DeclineRequestResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<
    crate::proto::mavio::acquisition::v1::DeclineRequestResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeclineRequestResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::RetryRequestResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::RetryRequestResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::RetryRequestResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::RetryRequestResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GetQuotaResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::GetQuotaResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GetQuotaResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetQuotaResponseView<'static>,
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
pub const REQUEST_SERVICE_SERVICE_NAME: &str = "mavio.acquisition.v1.RequestService";
/// Static [`Spec`](::connectrpc::Spec) for the `CreateRequest` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const REQUEST_SERVICE_CREATE_REQUEST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.RequestService/CreateRequest",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListRequests` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const REQUEST_SERVICE_LIST_REQUESTS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.RequestService/ListRequests",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `GetRequest` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const REQUEST_SERVICE_GET_REQUEST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.RequestService/GetRequest",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `UpdateRequest` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const REQUEST_SERVICE_UPDATE_REQUEST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.RequestService/UpdateRequest",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `DeleteRequest` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const REQUEST_SERVICE_DELETE_REQUEST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.RequestService/DeleteRequest",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `ApproveRequest` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const REQUEST_SERVICE_APPROVE_REQUEST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.RequestService/ApproveRequest",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `DeclineRequest` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const REQUEST_SERVICE_DECLINE_REQUEST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.RequestService/DeclineRequest",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `RetryRequest` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const REQUEST_SERVICE_RETRY_REQUEST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.RequestService/RetryRequest",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `GetQuota` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const REQUEST_SERVICE_GET_QUOTA_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.RequestService/GetQuota",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// RequestService takes users' requests of movies and seasons of series,
/// as Seerr's: administrators approve or decline them, unless the user's
/// requests are approved at once, and approved requests are acquired.
/// Signed-in users request what their policy allows, within their quotas
/// and their rating limit, and see their own requests; administrators
/// manage all.
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
pub trait RequestService: Send + Sync + 'static {
    /// CreateRequest requests a movie, or seasons of a series, by its TMDB
    /// ID. It fails with PERMISSION_DENIED when the user may not request it
    /// (its kind, or its rating above the user's limit), RESOURCE_EXHAUSTED
    /// past the user's quota, and ALREADY_EXISTS when it is requested,
    /// wanted or available already.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn create_request<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::CreateRequestRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::CreateRequestResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListRequests lists the caller's requests, or anyone's for
    /// administrators.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_requests<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ListRequestsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ListRequestsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetRequest returns a request of the caller's, or anyone's for
    /// administrators.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_request<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::GetRequestRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::GetRequestResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// UpdateRequest changes the seasons of a pending series request; its
    /// user or an administrator.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn update_request<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::UpdateRequestRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::UpdateRequestResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// DeleteRequest deletes a request: administrators any, users their
    /// pending ones.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn delete_request<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::DeleteRequestRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::DeleteRequestResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ApproveRequest approves a pending request, which adds its movie or
    /// seasons to acquisition. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn approve_request<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ApproveRequestRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ApproveRequestResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// DeclineRequest declines a pending request, which frees the quota it
    /// used. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn decline_request<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::DeclineRequestRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::DeclineRequestResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// RetryRequest approves a failed request again. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn retry_request<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::RetryRequestRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::RetryRequestResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetQuota returns how much of their quotas a user has used: the
    /// caller's own, or anyone's for administrators.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_quota<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::GetQuotaRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::GetQuotaResponse,
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
pub trait RequestServiceExt: RequestService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: RequestService> RequestServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view(
                REQUEST_SERVICE_SERVICE_NAME,
                "CreateRequest",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::CreateRequestRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::CreateRequestRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.create_request(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::CreateRequestResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(REQUEST_SERVICE_CREATE_REQUEST_SPEC)
            .route_view_idempotent(
                REQUEST_SERVICE_SERVICE_NAME,
                "ListRequests",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ListRequestsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ListRequestsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_requests(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ListRequestsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(REQUEST_SERVICE_LIST_REQUESTS_SPEC)
            .route_view_idempotent(
                REQUEST_SERVICE_SERVICE_NAME,
                "GetRequest",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::GetRequestRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::GetRequestRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_request(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::GetRequestResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(REQUEST_SERVICE_GET_REQUEST_SPEC)
            .route_view(
                REQUEST_SERVICE_SERVICE_NAME,
                "UpdateRequest",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::UpdateRequestRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::UpdateRequestRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.update_request(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::UpdateRequestResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(REQUEST_SERVICE_UPDATE_REQUEST_SPEC)
            .route_view(
                REQUEST_SERVICE_SERVICE_NAME,
                "DeleteRequest",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::DeleteRequestRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::DeleteRequestRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.delete_request(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::DeleteRequestResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(REQUEST_SERVICE_DELETE_REQUEST_SPEC)
            .route_view(
                REQUEST_SERVICE_SERVICE_NAME,
                "ApproveRequest",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ApproveRequestRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ApproveRequestRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.approve_request(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ApproveRequestResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(REQUEST_SERVICE_APPROVE_REQUEST_SPEC)
            .route_view(
                REQUEST_SERVICE_SERVICE_NAME,
                "DeclineRequest",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::DeclineRequestRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::DeclineRequestRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.decline_request(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::DeclineRequestResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(REQUEST_SERVICE_DECLINE_REQUEST_SPEC)
            .route_view(
                REQUEST_SERVICE_SERVICE_NAME,
                "RetryRequest",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::RetryRequestRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::RetryRequestRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.retry_request(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::RetryRequestResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(REQUEST_SERVICE_RETRY_REQUEST_SPEC)
            .route_view_idempotent(
                REQUEST_SERVICE_SERVICE_NAME,
                "GetQuota",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::GetQuotaRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::GetQuotaRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_quota(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::GetQuotaResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(REQUEST_SERVICE_GET_QUOTA_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct RequestServiceRegisterMarker;
impl<S: RequestService> ::connectrpc::ServiceRegister<RequestServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as RequestServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `RequestService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = RequestServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct RequestServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: RequestService> RequestServiceServer<T> {
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
impl<T> Clone for RequestServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: RequestService> ::connectrpc::Dispatcher for RequestServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.acquisition.v1.RequestService/")?;
        match method {
            "CreateRequest" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(REQUEST_SERVICE_CREATE_REQUEST_SPEC),
                )
            }
            "ListRequests" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(REQUEST_SERVICE_LIST_REQUESTS_SPEC),
                )
            }
            "GetRequest" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(REQUEST_SERVICE_GET_REQUEST_SPEC),
                )
            }
            "UpdateRequest" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(REQUEST_SERVICE_UPDATE_REQUEST_SPEC),
                )
            }
            "DeleteRequest" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(REQUEST_SERVICE_DELETE_REQUEST_SPEC),
                )
            }
            "ApproveRequest" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(REQUEST_SERVICE_APPROVE_REQUEST_SPEC),
                )
            }
            "DeclineRequest" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(REQUEST_SERVICE_DECLINE_REQUEST_SPEC),
                )
            }
            "RetryRequest" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(REQUEST_SERVICE_RETRY_REQUEST_SPEC),
                )
            }
            "GetQuota" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(REQUEST_SERVICE_GET_QUOTA_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.RequestService/")
        else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "CreateRequest" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::CreateRequestRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::CreateRequestRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::CreateRequestRequest,
                    >::from_parts(&req, &body);
                    svc.create_request(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::CreateRequestResponse,
                        >(format)
                })
            }
            "ListRequests" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ListRequestsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ListRequestsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ListRequestsRequest,
                    >::from_parts(&req, &body);
                    svc.list_requests(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ListRequestsResponse,
                        >(format)
                })
            }
            "GetRequest" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::GetRequestRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::GetRequestRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::GetRequestRequest,
                    >::from_parts(&req, &body);
                    svc.get_request(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::GetRequestResponse,
                        >(format)
                })
            }
            "UpdateRequest" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::UpdateRequestRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::UpdateRequestRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::UpdateRequestRequest,
                    >::from_parts(&req, &body);
                    svc.update_request(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::UpdateRequestResponse,
                        >(format)
                })
            }
            "DeleteRequest" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::DeleteRequestRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::DeleteRequestRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::DeleteRequestRequest,
                    >::from_parts(&req, &body);
                    svc.delete_request(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::DeleteRequestResponse,
                        >(format)
                })
            }
            "ApproveRequest" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ApproveRequestRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ApproveRequestRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ApproveRequestRequest,
                    >::from_parts(&req, &body);
                    svc.approve_request(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ApproveRequestResponse,
                        >(format)
                })
            }
            "DeclineRequest" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::DeclineRequestRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::DeclineRequestRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::DeclineRequestRequest,
                    >::from_parts(&req, &body);
                    svc.decline_request(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::DeclineRequestResponse,
                        >(format)
                })
            }
            "RetryRequest" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::RetryRequestRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::RetryRequestRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::RetryRequestRequest,
                    >::from_parts(&req, &body);
                    svc.retry_request(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::RetryRequestResponse,
                        >(format)
                })
            }
            "GetQuota" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::GetQuotaRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::GetQuotaRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::GetQuotaRequest,
                    >::from_parts(&req, &body);
                    svc.get_quota(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::GetQuotaResponse,
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.RequestService/")
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.RequestService/")
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.RequestService/")
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
/// let client = RequestServiceClient::new(conn, config);
/// let response = client.create_request(request).await?;
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
/// let client = RequestServiceClient::new(http, config);
/// let response = client.create_request(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.create_request(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.create_request(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct RequestServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> RequestServiceClient<T>
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
    /// Call the CreateRequest RPC. Sends a request to /mavio.acquisition.v1.RequestService/CreateRequest.
    pub async fn create_request(
        &self,
        request: crate::proto::mavio::acquisition::v1::CreateRequestRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::CreateRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.create_request_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the CreateRequest RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn create_request_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::CreateRequestRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::CreateRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                REQUEST_SERVICE_CREATE_REQUEST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListRequests RPC. Sends a request to /mavio.acquisition.v1.RequestService/ListRequests.
    pub async fn list_requests(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListRequestsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListRequestsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_requests_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListRequests RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_requests_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListRequestsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListRequestsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                REQUEST_SERVICE_LIST_REQUESTS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetRequest RPC. Sends a request to /mavio.acquisition.v1.RequestService/GetRequest.
    pub async fn get_request(
        &self,
        request: crate::proto::mavio::acquisition::v1::GetRequestRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GetRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_request_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetRequest RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_request_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::GetRequestRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GetRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                REQUEST_SERVICE_GET_REQUEST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the UpdateRequest RPC. Sends a request to /mavio.acquisition.v1.RequestService/UpdateRequest.
    pub async fn update_request(
        &self,
        request: crate::proto::mavio::acquisition::v1::UpdateRequestRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::UpdateRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.update_request_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the UpdateRequest RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn update_request_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::UpdateRequestRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::UpdateRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                REQUEST_SERVICE_UPDATE_REQUEST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DeleteRequest RPC. Sends a request to /mavio.acquisition.v1.RequestService/DeleteRequest.
    pub async fn delete_request(
        &self,
        request: crate::proto::mavio::acquisition::v1::DeleteRequestRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::DeleteRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.delete_request_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DeleteRequest RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn delete_request_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::DeleteRequestRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::DeleteRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                REQUEST_SERVICE_DELETE_REQUEST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ApproveRequest RPC. Sends a request to /mavio.acquisition.v1.RequestService/ApproveRequest.
    pub async fn approve_request(
        &self,
        request: crate::proto::mavio::acquisition::v1::ApproveRequestRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ApproveRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.approve_request_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ApproveRequest RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn approve_request_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ApproveRequestRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ApproveRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                REQUEST_SERVICE_APPROVE_REQUEST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DeclineRequest RPC. Sends a request to /mavio.acquisition.v1.RequestService/DeclineRequest.
    pub async fn decline_request(
        &self,
        request: crate::proto::mavio::acquisition::v1::DeclineRequestRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::DeclineRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.decline_request_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DeclineRequest RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn decline_request_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::DeclineRequestRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::DeclineRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                REQUEST_SERVICE_DECLINE_REQUEST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the RetryRequest RPC. Sends a request to /mavio.acquisition.v1.RequestService/RetryRequest.
    pub async fn retry_request(
        &self,
        request: crate::proto::mavio::acquisition::v1::RetryRequestRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::RetryRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.retry_request_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the RetryRequest RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn retry_request_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::RetryRequestRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::RetryRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                REQUEST_SERVICE_RETRY_REQUEST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetQuota RPC. Sends a request to /mavio.acquisition.v1.RequestService/GetQuota.
    pub async fn get_quota(
        &self,
        request: crate::proto::mavio::acquisition::v1::GetQuotaRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GetQuotaResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_quota_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetQuota RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_quota_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::GetQuotaRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GetQuotaResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                REQUEST_SERVICE_GET_QUOTA_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

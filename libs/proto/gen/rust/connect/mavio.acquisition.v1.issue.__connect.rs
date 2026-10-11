///Shorthand for `OwnedView<CreateIssueRequestView<'static>>`.
pub type OwnedCreateIssueRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::CreateIssueRequestView<'static>,
>;
///Shorthand for `OwnedView<CreateIssueResponseView<'static>>`.
pub type OwnedCreateIssueResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::CreateIssueResponseView<'static>,
>;
///Shorthand for `OwnedView<ListIssuesRequestView<'static>>`.
pub type OwnedListIssuesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListIssuesRequestView<'static>,
>;
///Shorthand for `OwnedView<ListIssuesResponseView<'static>>`.
pub type OwnedListIssuesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListIssuesResponseView<'static>,
>;
///Shorthand for `OwnedView<GetIssueRequestView<'static>>`.
pub type OwnedGetIssueRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetIssueRequestView<'static>,
>;
///Shorthand for `OwnedView<GetIssueResponseView<'static>>`.
pub type OwnedGetIssueResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetIssueResponseView<'static>,
>;
///Shorthand for `OwnedView<AddCommentRequestView<'static>>`.
pub type OwnedAddCommentRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::AddCommentRequestView<'static>,
>;
///Shorthand for `OwnedView<AddCommentResponseView<'static>>`.
pub type OwnedAddCommentResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::AddCommentResponseView<'static>,
>;
///Shorthand for `OwnedView<ResolveIssueRequestView<'static>>`.
pub type OwnedResolveIssueRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ResolveIssueRequestView<'static>,
>;
///Shorthand for `OwnedView<ResolveIssueResponseView<'static>>`.
pub type OwnedResolveIssueResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ResolveIssueResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ReopenIssueRequestView<'static>>`.
pub type OwnedReopenIssueRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ReopenIssueRequestView<'static>,
>;
///Shorthand for `OwnedView<ReopenIssueResponseView<'static>>`.
pub type OwnedReopenIssueResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ReopenIssueResponseView<'static>,
>;
///Shorthand for `OwnedView<DeleteIssueRequestView<'static>>`.
pub type OwnedDeleteIssueRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeleteIssueRequestView<'static>,
>;
///Shorthand for `OwnedView<DeleteIssueResponseView<'static>>`.
pub type OwnedDeleteIssueResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeleteIssueResponseView<'static>,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::CreateIssueResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::CreateIssueResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::CreateIssueResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::CreateIssueResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListIssuesResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::ListIssuesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListIssuesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListIssuesResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GetIssueResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::GetIssueResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GetIssueResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetIssueResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::AddCommentResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::AddCommentResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::AddCommentResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::AddCommentResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ResolveIssueResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::ResolveIssueResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ResolveIssueResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ResolveIssueResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ReopenIssueResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::ReopenIssueResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ReopenIssueResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ReopenIssueResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::DeleteIssueResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::DeleteIssueResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::DeleteIssueResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeleteIssueResponseView<'static>,
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
pub const ISSUE_SERVICE_SERVICE_NAME: &str = "mavio.acquisition.v1.IssueService";
/// Static [`Spec`](::connectrpc::Spec) for the `CreateIssue` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ISSUE_SERVICE_CREATE_ISSUE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.IssueService/CreateIssue",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListIssues` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ISSUE_SERVICE_LIST_ISSUES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.IssueService/ListIssues",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `GetIssue` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ISSUE_SERVICE_GET_ISSUE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.IssueService/GetIssue",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `AddComment` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ISSUE_SERVICE_ADD_COMMENT_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.IssueService/AddComment",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ResolveIssue` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ISSUE_SERVICE_RESOLVE_ISSUE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.IssueService/ResolveIssue",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `ReopenIssue` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ISSUE_SERVICE_REOPEN_ISSUE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.IssueService/ReopenIssue",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `DeleteIssue` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ISSUE_SERVICE_DELETE_ISSUE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.IssueService/DeleteIssue",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// IssueService takes the problems users report with items, as Seerr's
/// issues: users who may report issues report them and comment on their
/// own; administrators see, comment on, resolve and delete all.
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
pub trait IssueService: Send + Sync + 'static {
    /// CreateIssue reports a problem with an item the caller can access;
    /// its message is the first comment.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn create_issue<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::CreateIssueRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::CreateIssueResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListIssues lists the caller's issues, or anyone's for administrators.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_issues<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ListIssuesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ListIssuesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetIssue returns an issue of the caller's, or anyone's for
    /// administrators.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_issue<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::GetIssueRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::GetIssueResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// AddComment comments on an issue: its reporter or an administrator.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn add_comment<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::AddCommentRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::AddCommentResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ResolveIssue resolves an issue: its reporter or an administrator.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn resolve_issue<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ResolveIssueRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ResolveIssueResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ReopenIssue reopens an issue: its reporter or an administrator.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn reopen_issue<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ReopenIssueRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ReopenIssueResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// DeleteIssue deletes an issue: administrators any, its reporter while
    /// nobody else commented on it.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn delete_issue<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::DeleteIssueRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::DeleteIssueResponse,
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
pub trait IssueServiceExt: IssueService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: IssueService> IssueServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view(
                ISSUE_SERVICE_SERVICE_NAME,
                "CreateIssue",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::CreateIssueRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::CreateIssueRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.create_issue(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::CreateIssueResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ISSUE_SERVICE_CREATE_ISSUE_SPEC)
            .route_view_idempotent(
                ISSUE_SERVICE_SERVICE_NAME,
                "ListIssues",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ListIssuesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ListIssuesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_issues(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ListIssuesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ISSUE_SERVICE_LIST_ISSUES_SPEC)
            .route_view_idempotent(
                ISSUE_SERVICE_SERVICE_NAME,
                "GetIssue",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::GetIssueRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::GetIssueRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_issue(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::GetIssueResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ISSUE_SERVICE_GET_ISSUE_SPEC)
            .route_view(
                ISSUE_SERVICE_SERVICE_NAME,
                "AddComment",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::AddCommentRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::AddCommentRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.add_comment(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::AddCommentResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ISSUE_SERVICE_ADD_COMMENT_SPEC)
            .route_view(
                ISSUE_SERVICE_SERVICE_NAME,
                "ResolveIssue",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ResolveIssueRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ResolveIssueRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.resolve_issue(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ResolveIssueResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ISSUE_SERVICE_RESOLVE_ISSUE_SPEC)
            .route_view(
                ISSUE_SERVICE_SERVICE_NAME,
                "ReopenIssue",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ReopenIssueRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ReopenIssueRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.reopen_issue(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ReopenIssueResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ISSUE_SERVICE_REOPEN_ISSUE_SPEC)
            .route_view(
                ISSUE_SERVICE_SERVICE_NAME,
                "DeleteIssue",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::DeleteIssueRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::DeleteIssueRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.delete_issue(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::DeleteIssueResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ISSUE_SERVICE_DELETE_ISSUE_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct IssueServiceRegisterMarker;
impl<S: IssueService> ::connectrpc::ServiceRegister<IssueServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as IssueServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `IssueService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = IssueServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct IssueServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: IssueService> IssueServiceServer<T> {
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
impl<T> Clone for IssueServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: IssueService> ::connectrpc::Dispatcher for IssueServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.acquisition.v1.IssueService/")?;
        match method {
            "CreateIssue" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(ISSUE_SERVICE_CREATE_ISSUE_SPEC),
                )
            }
            "ListIssues" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ISSUE_SERVICE_LIST_ISSUES_SPEC),
                )
            }
            "GetIssue" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ISSUE_SERVICE_GET_ISSUE_SPEC),
                )
            }
            "AddComment" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(ISSUE_SERVICE_ADD_COMMENT_SPEC),
                )
            }
            "ResolveIssue" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(ISSUE_SERVICE_RESOLVE_ISSUE_SPEC),
                )
            }
            "ReopenIssue" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(ISSUE_SERVICE_REOPEN_ISSUE_SPEC),
                )
            }
            "DeleteIssue" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(ISSUE_SERVICE_DELETE_ISSUE_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.IssueService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "CreateIssue" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::CreateIssueRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::CreateIssueRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::CreateIssueRequest,
                    >::from_parts(&req, &body);
                    svc.create_issue(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::CreateIssueResponse,
                        >(format)
                })
            }
            "ListIssues" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ListIssuesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ListIssuesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ListIssuesRequest,
                    >::from_parts(&req, &body);
                    svc.list_issues(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ListIssuesResponse,
                        >(format)
                })
            }
            "GetIssue" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::GetIssueRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::GetIssueRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::GetIssueRequest,
                    >::from_parts(&req, &body);
                    svc.get_issue(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::GetIssueResponse,
                        >(format)
                })
            }
            "AddComment" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::AddCommentRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::AddCommentRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::AddCommentRequest,
                    >::from_parts(&req, &body);
                    svc.add_comment(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::AddCommentResponse,
                        >(format)
                })
            }
            "ResolveIssue" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ResolveIssueRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ResolveIssueRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ResolveIssueRequest,
                    >::from_parts(&req, &body);
                    svc.resolve_issue(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ResolveIssueResponse,
                        >(format)
                })
            }
            "ReopenIssue" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ReopenIssueRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ReopenIssueRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ReopenIssueRequest,
                    >::from_parts(&req, &body);
                    svc.reopen_issue(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ReopenIssueResponse,
                        >(format)
                })
            }
            "DeleteIssue" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::DeleteIssueRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::DeleteIssueRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::DeleteIssueRequest,
                    >::from_parts(&req, &body);
                    svc.delete_issue(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::DeleteIssueResponse,
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.IssueService/") else {
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.IssueService/") else {
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.IssueService/") else {
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
/// let client = IssueServiceClient::new(conn, config);
/// let response = client.create_issue(request).await?;
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
/// let client = IssueServiceClient::new(http, config);
/// let response = client.create_issue(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.create_issue(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.create_issue(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct IssueServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> IssueServiceClient<T>
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
    /// Call the CreateIssue RPC. Sends a request to /mavio.acquisition.v1.IssueService/CreateIssue.
    pub async fn create_issue(
        &self,
        request: crate::proto::mavio::acquisition::v1::CreateIssueRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::CreateIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.create_issue_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the CreateIssue RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn create_issue_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::CreateIssueRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::CreateIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ISSUE_SERVICE_CREATE_ISSUE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListIssues RPC. Sends a request to /mavio.acquisition.v1.IssueService/ListIssues.
    pub async fn list_issues(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListIssuesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListIssuesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_issues_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListIssues RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_issues_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListIssuesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListIssuesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ISSUE_SERVICE_LIST_ISSUES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetIssue RPC. Sends a request to /mavio.acquisition.v1.IssueService/GetIssue.
    pub async fn get_issue(
        &self,
        request: crate::proto::mavio::acquisition::v1::GetIssueRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GetIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_issue_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetIssue RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_issue_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::GetIssueRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GetIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ISSUE_SERVICE_GET_ISSUE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the AddComment RPC. Sends a request to /mavio.acquisition.v1.IssueService/AddComment.
    pub async fn add_comment(
        &self,
        request: crate::proto::mavio::acquisition::v1::AddCommentRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::AddCommentResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.add_comment_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the AddComment RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn add_comment_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::AddCommentRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::AddCommentResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ISSUE_SERVICE_ADD_COMMENT_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ResolveIssue RPC. Sends a request to /mavio.acquisition.v1.IssueService/ResolveIssue.
    pub async fn resolve_issue(
        &self,
        request: crate::proto::mavio::acquisition::v1::ResolveIssueRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ResolveIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.resolve_issue_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ResolveIssue RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn resolve_issue_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ResolveIssueRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ResolveIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ISSUE_SERVICE_RESOLVE_ISSUE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ReopenIssue RPC. Sends a request to /mavio.acquisition.v1.IssueService/ReopenIssue.
    pub async fn reopen_issue(
        &self,
        request: crate::proto::mavio::acquisition::v1::ReopenIssueRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ReopenIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.reopen_issue_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ReopenIssue RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn reopen_issue_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ReopenIssueRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ReopenIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ISSUE_SERVICE_REOPEN_ISSUE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DeleteIssue RPC. Sends a request to /mavio.acquisition.v1.IssueService/DeleteIssue.
    pub async fn delete_issue(
        &self,
        request: crate::proto::mavio::acquisition::v1::DeleteIssueRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::DeleteIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.delete_issue_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DeleteIssue RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn delete_issue_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::DeleteIssueRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::DeleteIssueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ISSUE_SERVICE_DELETE_ISSUE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

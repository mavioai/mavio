///Shorthand for `OwnedView<ListAiProvidersRequestView<'static>>`.
pub type OwnedListAiProvidersRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListAiProvidersRequestView<'static>,
>;
///Shorthand for `OwnedView<ListAiProvidersResponseView<'static>>`.
pub type OwnedListAiProvidersResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListAiProvidersResponseView<'static>,
>;
///Shorthand for `OwnedView<ListAiModelsRequestView<'static>>`.
pub type OwnedListAiModelsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListAiModelsRequestView<'static>,
>;
///Shorthand for `OwnedView<ListAiModelsResponseView<'static>>`.
pub type OwnedListAiModelsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListAiModelsResponseView<'static>,
>;
///Shorthand for `OwnedView<SetAiApiKeyRequestView<'static>>`.
pub type OwnedSetAiApiKeyRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::SetAiApiKeyRequestView<'static>,
>;
///Shorthand for `OwnedView<SetAiApiKeyResponseView<'static>>`.
pub type OwnedSetAiApiKeyResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::SetAiApiKeyResponseView<'static>,
>;
///Shorthand for `OwnedView<StartAiLoginRequestView<'static>>`.
pub type OwnedStartAiLoginRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::StartAiLoginRequestView<'static>,
>;
///Shorthand for `OwnedView<StartAiLoginResponseView<'static>>`.
pub type OwnedStartAiLoginResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::StartAiLoginResponseView<'static>,
>;
///Shorthand for `OwnedView<GetAiLoginRequestView<'static>>`.
pub type OwnedGetAiLoginRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetAiLoginRequestView<'static>,
>;
///Shorthand for `OwnedView<GetAiLoginResponseView<'static>>`.
pub type OwnedGetAiLoginResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetAiLoginResponseView<'static>,
>;
///Shorthand for `OwnedView<AnswerAiLoginRequestView<'static>>`.
pub type OwnedAnswerAiLoginRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::AnswerAiLoginRequestView<'static>,
>;
///Shorthand for `OwnedView<AnswerAiLoginResponseView<'static>>`.
pub type OwnedAnswerAiLoginResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::AnswerAiLoginResponseView<'static>,
>;
///Shorthand for `OwnedView<CancelAiLoginRequestView<'static>>`.
pub type OwnedCancelAiLoginRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::CancelAiLoginRequestView<'static>,
>;
///Shorthand for `OwnedView<CancelAiLoginResponseView<'static>>`.
pub type OwnedCancelAiLoginResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::CancelAiLoginResponseView<'static>,
>;
///Shorthand for `OwnedView<LogoutAiRequestView<'static>>`.
pub type OwnedLogoutAiRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::LogoutAiRequestView<'static>,
>;
///Shorthand for `OwnedView<LogoutAiResponseView<'static>>`.
pub type OwnedLogoutAiResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::LogoutAiResponseView<'static>,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListAiProvidersResponse>
for crate::proto::mavio::system::v1::__buffa::view::ListAiProvidersResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListAiProvidersResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListAiProvidersResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListAiModelsResponse>
for crate::proto::mavio::system::v1::__buffa::view::ListAiModelsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListAiModelsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListAiModelsResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::SetAiApiKeyResponse>
for crate::proto::mavio::system::v1::__buffa::view::SetAiApiKeyResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::SetAiApiKeyResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::SetAiApiKeyResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::StartAiLoginResponse>
for crate::proto::mavio::system::v1::__buffa::view::StartAiLoginResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::StartAiLoginResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::StartAiLoginResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetAiLoginResponse>
for crate::proto::mavio::system::v1::__buffa::view::GetAiLoginResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetAiLoginResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetAiLoginResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::AnswerAiLoginResponse>
for crate::proto::mavio::system::v1::__buffa::view::AnswerAiLoginResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::AnswerAiLoginResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::AnswerAiLoginResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::CancelAiLoginResponse>
for crate::proto::mavio::system::v1::__buffa::view::CancelAiLoginResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::CancelAiLoginResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::CancelAiLoginResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::LogoutAiResponse>
for crate::proto::mavio::system::v1::__buffa::view::LogoutAiResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::LogoutAiResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::LogoutAiResponseView<'static>,
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
pub const AI_SERVICE_SERVICE_NAME: &str = "mavio.system.v1.AiService";
/// Static [`Spec`](::connectrpc::Spec) for the `ListAiProviders` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const AI_SERVICE_LIST_AI_PROVIDERS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.AiService/ListAiProviders",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListAiModels` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const AI_SERVICE_LIST_AI_MODELS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.AiService/ListAiModels",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `SetAiApiKey` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const AI_SERVICE_SET_AI_API_KEY_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.AiService/SetAiApiKey",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `StartAiLogin` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const AI_SERVICE_START_AI_LOGIN_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.AiService/StartAiLogin",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `GetAiLogin` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const AI_SERVICE_GET_AI_LOGIN_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.AiService/GetAiLogin",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `AnswerAiLogin` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const AI_SERVICE_ANSWER_AI_LOGIN_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.AiService/AnswerAiLogin",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `CancelAiLogin` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const AI_SERVICE_CANCEL_AI_LOGIN_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.AiService/CancelAiLogin",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `LogoutAi` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const AI_SERVICE_LOGOUT_AI_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.AiService/LogoutAi",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// AiService administers the AI model providers the server's features use,
/// through the plugin with the AI_GATEWAY capability, which keeps their
/// credentials: API keys, and OAuth logins relayed to the administrator.
/// Every RPC requires an administrator; without a started AI gateway
/// plugin they fail with failed precondition.
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
pub trait AiService: Send + Sync + 'static {
    /// ListAiProviders lists the providers with how they authenticate and
    /// whether they can be used.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_ai_providers<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::ListAiProvidersRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::ListAiProvidersResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListAiModels lists the providers' models.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_ai_models<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::ListAiModelsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::ListAiModelsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SetAiApiKey stores a provider's API key, or removes it.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn set_ai_api_key<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::SetAiApiKeyRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::SetAiApiKeyResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// StartAiLogin starts a provider's OAuth login. Logins to subscriptions
    /// (AiProvider.subscription) may break the provider's terms; clients
    /// warn of it and leave the choice to the administrator.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn start_ai_login<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::StartAiLoginRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::StartAiLoginResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetAiLogin returns a login's state: what to show, what it asks and
    /// whether it ended. Clients poll it while the login runs.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_ai_login<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::GetAiLoginRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::GetAiLoginResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// AnswerAiLogin answers a login's prompt.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn answer_ai_login<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::AnswerAiLoginRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::AnswerAiLoginResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// CancelAiLogin abandons a login.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn cancel_ai_login<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::CancelAiLoginRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::CancelAiLoginResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// LogoutAi removes a provider's stored credential.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn logout_ai<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::LogoutAiRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::LogoutAiResponse,
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
pub trait AiServiceExt: AiService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: AiService> AiServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                AI_SERVICE_SERVICE_NAME,
                "ListAiProviders",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::ListAiProvidersRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::ListAiProvidersRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_ai_providers(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::ListAiProvidersResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(AI_SERVICE_LIST_AI_PROVIDERS_SPEC)
            .route_view_idempotent(
                AI_SERVICE_SERVICE_NAME,
                "ListAiModels",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::ListAiModelsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::ListAiModelsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_ai_models(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::ListAiModelsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(AI_SERVICE_LIST_AI_MODELS_SPEC)
            .route_view(
                AI_SERVICE_SERVICE_NAME,
                "SetAiApiKey",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::SetAiApiKeyRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::SetAiApiKeyRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.set_ai_api_key(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::SetAiApiKeyResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(AI_SERVICE_SET_AI_API_KEY_SPEC)
            .route_view(
                AI_SERVICE_SERVICE_NAME,
                "StartAiLogin",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::StartAiLoginRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::StartAiLoginRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.start_ai_login(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::StartAiLoginResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(AI_SERVICE_START_AI_LOGIN_SPEC)
            .route_view_idempotent(
                AI_SERVICE_SERVICE_NAME,
                "GetAiLogin",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::GetAiLoginRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::GetAiLoginRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_ai_login(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::GetAiLoginResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(AI_SERVICE_GET_AI_LOGIN_SPEC)
            .route_view(
                AI_SERVICE_SERVICE_NAME,
                "AnswerAiLogin",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::AnswerAiLoginRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::AnswerAiLoginRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.answer_ai_login(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::AnswerAiLoginResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(AI_SERVICE_ANSWER_AI_LOGIN_SPEC)
            .route_view(
                AI_SERVICE_SERVICE_NAME,
                "CancelAiLogin",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::CancelAiLoginRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::CancelAiLoginRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.cancel_ai_login(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::CancelAiLoginResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(AI_SERVICE_CANCEL_AI_LOGIN_SPEC)
            .route_view(
                AI_SERVICE_SERVICE_NAME,
                "LogoutAi",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::LogoutAiRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::LogoutAiRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.logout_ai(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::LogoutAiResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(AI_SERVICE_LOGOUT_AI_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct AiServiceRegisterMarker;
impl<S: AiService> ::connectrpc::ServiceRegister<AiServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as AiServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `AiService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = AiServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct AiServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: AiService> AiServiceServer<T> {
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
impl<T> Clone for AiServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: AiService> ::connectrpc::Dispatcher for AiServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.system.v1.AiService/")?;
        match method {
            "ListAiProviders" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(AI_SERVICE_LIST_AI_PROVIDERS_SPEC),
                )
            }
            "ListAiModels" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(AI_SERVICE_LIST_AI_MODELS_SPEC),
                )
            }
            "SetAiApiKey" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(AI_SERVICE_SET_AI_API_KEY_SPEC),
                )
            }
            "StartAiLogin" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(AI_SERVICE_START_AI_LOGIN_SPEC),
                )
            }
            "GetAiLogin" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(AI_SERVICE_GET_AI_LOGIN_SPEC),
                )
            }
            "AnswerAiLogin" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(AI_SERVICE_ANSWER_AI_LOGIN_SPEC),
                )
            }
            "CancelAiLogin" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(AI_SERVICE_CANCEL_AI_LOGIN_SPEC),
                )
            }
            "LogoutAi" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(AI_SERVICE_LOGOUT_AI_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.system.v1.AiService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "ListAiProviders" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::ListAiProvidersRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::ListAiProvidersRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::ListAiProvidersRequest,
                    >::from_parts(&req, &body);
                    svc.list_ai_providers(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::ListAiProvidersResponse,
                        >(format)
                })
            }
            "ListAiModels" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::ListAiModelsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::ListAiModelsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::ListAiModelsRequest,
                    >::from_parts(&req, &body);
                    svc.list_ai_models(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::ListAiModelsResponse,
                        >(format)
                })
            }
            "SetAiApiKey" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::SetAiApiKeyRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::SetAiApiKeyRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::SetAiApiKeyRequest,
                    >::from_parts(&req, &body);
                    svc.set_ai_api_key(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::SetAiApiKeyResponse,
                        >(format)
                })
            }
            "StartAiLogin" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::StartAiLoginRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::StartAiLoginRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::StartAiLoginRequest,
                    >::from_parts(&req, &body);
                    svc.start_ai_login(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::StartAiLoginResponse,
                        >(format)
                })
            }
            "GetAiLogin" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::GetAiLoginRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::GetAiLoginRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::GetAiLoginRequest,
                    >::from_parts(&req, &body);
                    svc.get_ai_login(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::GetAiLoginResponse,
                        >(format)
                })
            }
            "AnswerAiLogin" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::AnswerAiLoginRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::AnswerAiLoginRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::AnswerAiLoginRequest,
                    >::from_parts(&req, &body);
                    svc.answer_ai_login(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::AnswerAiLoginResponse,
                        >(format)
                })
            }
            "CancelAiLogin" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::CancelAiLoginRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::CancelAiLoginRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::CancelAiLoginRequest,
                    >::from_parts(&req, &body);
                    svc.cancel_ai_login(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::CancelAiLoginResponse,
                        >(format)
                })
            }
            "LogoutAi" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::LogoutAiRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::LogoutAiRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::LogoutAiRequest,
                    >::from_parts(&req, &body);
                    svc.logout_ai(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::LogoutAiResponse,
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
        let Some(method) = path.strip_prefix("mavio.system.v1.AiService/") else {
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
        let Some(method) = path.strip_prefix("mavio.system.v1.AiService/") else {
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
        let Some(method) = path.strip_prefix("mavio.system.v1.AiService/") else {
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
/// let client = AiServiceClient::new(conn, config);
/// let response = client.list_ai_providers(request).await?;
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
/// let client = AiServiceClient::new(http, config);
/// let response = client.list_ai_providers(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.list_ai_providers(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.list_ai_providers(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct AiServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> AiServiceClient<T>
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
    /// Call the ListAiProviders RPC. Sends a request to /mavio.system.v1.AiService/ListAiProviders.
    pub async fn list_ai_providers(
        &self,
        request: crate::proto::mavio::system::v1::ListAiProvidersRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListAiProvidersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_ai_providers_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListAiProviders RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_ai_providers_with_options(
        &self,
        request: crate::proto::mavio::system::v1::ListAiProvidersRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListAiProvidersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                AI_SERVICE_LIST_AI_PROVIDERS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListAiModels RPC. Sends a request to /mavio.system.v1.AiService/ListAiModels.
    pub async fn list_ai_models(
        &self,
        request: crate::proto::mavio::system::v1::ListAiModelsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListAiModelsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_ai_models_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListAiModels RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_ai_models_with_options(
        &self,
        request: crate::proto::mavio::system::v1::ListAiModelsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListAiModelsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                AI_SERVICE_LIST_AI_MODELS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SetAiApiKey RPC. Sends a request to /mavio.system.v1.AiService/SetAiApiKey.
    pub async fn set_ai_api_key(
        &self,
        request: crate::proto::mavio::system::v1::SetAiApiKeyRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::SetAiApiKeyResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.set_ai_api_key_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SetAiApiKey RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn set_ai_api_key_with_options(
        &self,
        request: crate::proto::mavio::system::v1::SetAiApiKeyRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::SetAiApiKeyResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                AI_SERVICE_SET_AI_API_KEY_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the StartAiLogin RPC. Sends a request to /mavio.system.v1.AiService/StartAiLogin.
    pub async fn start_ai_login(
        &self,
        request: crate::proto::mavio::system::v1::StartAiLoginRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::StartAiLoginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.start_ai_login_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the StartAiLogin RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn start_ai_login_with_options(
        &self,
        request: crate::proto::mavio::system::v1::StartAiLoginRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::StartAiLoginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                AI_SERVICE_START_AI_LOGIN_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetAiLogin RPC. Sends a request to /mavio.system.v1.AiService/GetAiLogin.
    pub async fn get_ai_login(
        &self,
        request: crate::proto::mavio::system::v1::GetAiLoginRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetAiLoginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_ai_login_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetAiLogin RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_ai_login_with_options(
        &self,
        request: crate::proto::mavio::system::v1::GetAiLoginRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetAiLoginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                AI_SERVICE_GET_AI_LOGIN_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the AnswerAiLogin RPC. Sends a request to /mavio.system.v1.AiService/AnswerAiLogin.
    pub async fn answer_ai_login(
        &self,
        request: crate::proto::mavio::system::v1::AnswerAiLoginRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::AnswerAiLoginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.answer_ai_login_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the AnswerAiLogin RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn answer_ai_login_with_options(
        &self,
        request: crate::proto::mavio::system::v1::AnswerAiLoginRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::AnswerAiLoginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                AI_SERVICE_ANSWER_AI_LOGIN_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the CancelAiLogin RPC. Sends a request to /mavio.system.v1.AiService/CancelAiLogin.
    pub async fn cancel_ai_login(
        &self,
        request: crate::proto::mavio::system::v1::CancelAiLoginRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::CancelAiLoginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.cancel_ai_login_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the CancelAiLogin RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn cancel_ai_login_with_options(
        &self,
        request: crate::proto::mavio::system::v1::CancelAiLoginRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::CancelAiLoginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                AI_SERVICE_CANCEL_AI_LOGIN_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the LogoutAi RPC. Sends a request to /mavio.system.v1.AiService/LogoutAi.
    pub async fn logout_ai(
        &self,
        request: crate::proto::mavio::system::v1::LogoutAiRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::LogoutAiResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.logout_ai_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the LogoutAi RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn logout_ai_with_options(
        &self,
        request: crate::proto::mavio::system::v1::LogoutAiRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::LogoutAiResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                AI_SERVICE_LOGOUT_AI_SPEC.with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

///Shorthand for `OwnedView<ListTranscribersRequestView<'static>>`.
pub type OwnedListTranscribersRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListTranscribersRequestView<'static>,
>;
///Shorthand for `OwnedView<ListTranscribersResponseView<'static>>`.
pub type OwnedListTranscribersResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListTranscribersResponseView<'static>,
>;
///Shorthand for `OwnedView<TranscribeSpeechRequestView<'static>>`.
pub type OwnedTranscribeSpeechRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::TranscribeSpeechRequestView<'static>,
>;
///Shorthand for `OwnedView<TranscribeSpeechResponseView<'static>>`.
pub type OwnedTranscribeSpeechResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::TranscribeSpeechResponseView<'static>,
>;
///Shorthand for `OwnedView<DetectSpeechLanguageRequestView<'static>>`.
pub type OwnedDetectSpeechLanguageRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::DetectSpeechLanguageRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DetectSpeechLanguageResponseView<'static>>`.
pub type OwnedDetectSpeechLanguageResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::DetectSpeechLanguageResponseView<
        'static,
    >,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::ListTranscribersResponse>
for crate::proto::mavio::plugin::v1::__buffa::view::ListTranscribersResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::ListTranscribersResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::ListTranscribersResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::TranscribeSpeechResponse>
for crate::proto::mavio::plugin::v1::__buffa::view::TranscribeSpeechResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::plugin::v1::TranscribeSpeechResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::TranscribeSpeechResponseView<'static>,
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
    crate::proto::mavio::plugin::v1::DetectSpeechLanguageResponse,
>
for crate::proto::mavio::plugin::v1::__buffa::view::DetectSpeechLanguageResponseView<
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
    crate::proto::mavio::plugin::v1::DetectSpeechLanguageResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::plugin::v1::__buffa::view::DetectSpeechLanguageResponseView<
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
pub const SPEECH_TO_TEXT_SERVICE_SERVICE_NAME: &str = "mavio.plugin.v1.SpeechToTextService";
/// Static [`Spec`](::connectrpc::Spec) for the `ListTranscribers` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SPEECH_TO_TEXT_SERVICE_LIST_TRANSCRIBERS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.SpeechToTextService/ListTranscribers",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `TranscribeSpeech` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SPEECH_TO_TEXT_SERVICE_TRANSCRIBE_SPEECH_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.SpeechToTextService/TranscribeSpeech",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `DetectSpeechLanguage` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SPEECH_TO_TEXT_SERVICE_DETECT_SPEECH_LANGUAGE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.plugin.v1.SpeechToTextService/DetectSpeechLanguage",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// SpeechToTextService transcribes the audio of videos into subtitles with
/// the transcription services a plugin implements (docs/architecture.md
/// §14.5). What users say to the assistant is heard by voice sessions
/// (VoiceSessionService).
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
pub trait SpeechToTextService: Send + Sync + 'static {
    /// ListTranscribers lists the transcription services the plugin
    /// implements, with their settings, and the transcribers configured from
    /// them, with their state.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_transcribers<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::ListTranscribersRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::ListTranscribersResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// TranscribeSpeech transcribes audio into segments with their times.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn transcribe_speech<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::TranscribeSpeechRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::TranscribeSpeechResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// DetectSpeechLanguage tells the language spoken in the first 30
    /// seconds of audio.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn detect_speech_language<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::plugin::v1::DetectSpeechLanguageRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::plugin::v1::DetectSpeechLanguageResponse,
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
pub trait SpeechToTextServiceExt: SpeechToTextService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: SpeechToTextService> SpeechToTextServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                SPEECH_TO_TEXT_SERVICE_SERVICE_NAME,
                "ListTranscribers",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::ListTranscribersRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::ListTranscribersRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_transcribers(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::ListTranscribersResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SPEECH_TO_TEXT_SERVICE_LIST_TRANSCRIBERS_SPEC)
            .route_view(
                SPEECH_TO_TEXT_SERVICE_SERVICE_NAME,
                "TranscribeSpeech",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::TranscribeSpeechRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::TranscribeSpeechRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.transcribe_speech(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::TranscribeSpeechResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SPEECH_TO_TEXT_SERVICE_TRANSCRIBE_SPEECH_SPEC)
            .route_view(
                SPEECH_TO_TEXT_SERVICE_SERVICE_NAME,
                "DetectSpeechLanguage",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::plugin::v1::__buffa::view::DetectSpeechLanguageRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::plugin::v1::DetectSpeechLanguageRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.detect_speech_language(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::plugin::v1::DetectSpeechLanguageResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SPEECH_TO_TEXT_SERVICE_DETECT_SPEECH_LANGUAGE_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct SpeechToTextServiceRegisterMarker;
impl<
    S: SpeechToTextService,
> ::connectrpc::ServiceRegister<SpeechToTextServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as SpeechToTextServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `SpeechToTextService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = SpeechToTextServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct SpeechToTextServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: SpeechToTextService> SpeechToTextServiceServer<T> {
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
impl<T> Clone for SpeechToTextServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: SpeechToTextService> ::connectrpc::Dispatcher for SpeechToTextServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.plugin.v1.SpeechToTextService/")?;
        match method {
            "ListTranscribers" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SPEECH_TO_TEXT_SERVICE_LIST_TRANSCRIBERS_SPEC),
                )
            }
            "TranscribeSpeech" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SPEECH_TO_TEXT_SERVICE_TRANSCRIBE_SPEECH_SPEC),
                )
            }
            "DetectSpeechLanguage" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SPEECH_TO_TEXT_SERVICE_DETECT_SPEECH_LANGUAGE_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.SpeechToTextService/")
        else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "ListTranscribers" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::ListTranscribersRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::ListTranscribersRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::ListTranscribersRequest,
                    >::from_parts(&req, &body);
                    svc.list_transcribers(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::ListTranscribersResponse,
                        >(format)
                })
            }
            "TranscribeSpeech" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::TranscribeSpeechRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::TranscribeSpeechRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::TranscribeSpeechRequest,
                    >::from_parts(&req, &body);
                    svc.transcribe_speech(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::TranscribeSpeechResponse,
                        >(format)
                })
            }
            "DetectSpeechLanguage" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::plugin::v1::DetectSpeechLanguageRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::plugin::v1::__buffa::view::DetectSpeechLanguageRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::plugin::v1::DetectSpeechLanguageRequest,
                    >::from_parts(&req, &body);
                    svc.detect_speech_language(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::plugin::v1::DetectSpeechLanguageResponse,
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.SpeechToTextService/")
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.SpeechToTextService/")
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
        let Some(method) = path.strip_prefix("mavio.plugin.v1.SpeechToTextService/")
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
/// let client = SpeechToTextServiceClient::new(conn, config);
/// let response = client.list_transcribers(request).await?;
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
/// let client = SpeechToTextServiceClient::new(http, config);
/// let response = client.list_transcribers(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.list_transcribers(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.list_transcribers(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct SpeechToTextServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> SpeechToTextServiceClient<T>
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
    /// Call the ListTranscribers RPC. Sends a request to /mavio.plugin.v1.SpeechToTextService/ListTranscribers.
    pub async fn list_transcribers(
        &self,
        request: crate::proto::mavio::plugin::v1::ListTranscribersRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListTranscribersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_transcribers_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListTranscribers RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_transcribers_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::ListTranscribersRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::ListTranscribersResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SPEECH_TO_TEXT_SERVICE_LIST_TRANSCRIBERS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the TranscribeSpeech RPC. Sends a request to /mavio.plugin.v1.SpeechToTextService/TranscribeSpeech.
    pub async fn transcribe_speech(
        &self,
        request: crate::proto::mavio::plugin::v1::TranscribeSpeechRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::TranscribeSpeechResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.transcribe_speech_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the TranscribeSpeech RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn transcribe_speech_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::TranscribeSpeechRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::TranscribeSpeechResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SPEECH_TO_TEXT_SERVICE_TRANSCRIBE_SPEECH_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DetectSpeechLanguage RPC. Sends a request to /mavio.plugin.v1.SpeechToTextService/DetectSpeechLanguage.
    pub async fn detect_speech_language(
        &self,
        request: crate::proto::mavio::plugin::v1::DetectSpeechLanguageRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::DetectSpeechLanguageResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.detect_speech_language_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DetectSpeechLanguage RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn detect_speech_language_with_options(
        &self,
        request: crate::proto::mavio::plugin::v1::DetectSpeechLanguageRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::plugin::v1::__buffa::view::DetectSpeechLanguageResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SPEECH_TO_TEXT_SERVICE_DETECT_SPEECH_LANGUAGE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

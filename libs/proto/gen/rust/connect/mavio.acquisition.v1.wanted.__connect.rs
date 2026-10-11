///Shorthand for `OwnedView<AddMovieRequestView<'static>>`.
pub type OwnedAddMovieRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::AddMovieRequestView<'static>,
>;
///Shorthand for `OwnedView<AddMovieResponseView<'static>>`.
pub type OwnedAddMovieResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::AddMovieResponseView<'static>,
>;
///Shorthand for `OwnedView<AddSeriesRequestView<'static>>`.
pub type OwnedAddSeriesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::AddSeriesRequestView<'static>,
>;
///Shorthand for `OwnedView<AddSeriesResponseView<'static>>`.
pub type OwnedAddSeriesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::AddSeriesResponseView<'static>,
>;
///Shorthand for `OwnedView<MonitorSeriesRequestView<'static>>`.
pub type OwnedMonitorSeriesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::MonitorSeriesRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<MonitorSeriesResponseView<'static>>`.
pub type OwnedMonitorSeriesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::MonitorSeriesResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<SetMonitoredRequestView<'static>>`.
pub type OwnedSetMonitoredRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::SetMonitoredRequestView<'static>,
>;
///Shorthand for `OwnedView<SetMonitoredResponseView<'static>>`.
pub type OwnedSetMonitoredResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::SetMonitoredResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<GetWantedRequestView<'static>>`.
pub type OwnedGetWantedRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetWantedRequestView<'static>,
>;
///Shorthand for `OwnedView<GetWantedResponseView<'static>>`.
pub type OwnedGetWantedResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetWantedResponseView<'static>,
>;
///Shorthand for `OwnedView<UpdateWantedRequestView<'static>>`.
pub type OwnedUpdateWantedRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::UpdateWantedRequestView<'static>,
>;
///Shorthand for `OwnedView<UpdateWantedResponseView<'static>>`.
pub type OwnedUpdateWantedResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::UpdateWantedResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListWantedRequestView<'static>>`.
pub type OwnedListWantedRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListWantedRequestView<'static>,
>;
///Shorthand for `OwnedView<ListWantedResponseView<'static>>`.
pub type OwnedListWantedResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListWantedResponseView<'static>,
>;
///Shorthand for `OwnedView<SearchWantedRequestView<'static>>`.
pub type OwnedSearchWantedRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::SearchWantedRequestView<'static>,
>;
///Shorthand for `OwnedView<SearchWantedResponseView<'static>>`.
pub type OwnedSearchWantedResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::SearchWantedResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<SyncRssRequestView<'static>>`.
pub type OwnedSyncRssRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::SyncRSSRequestView<'static>,
>;
///Shorthand for `OwnedView<SyncRssResponseView<'static>>`.
pub type OwnedSyncRssResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::SyncRSSResponseView<'static>,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::AddMovieResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::AddMovieResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::AddMovieResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::AddMovieResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::AddSeriesResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::AddSeriesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::AddSeriesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::AddSeriesResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::MonitorSeriesResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::MonitorSeriesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::MonitorSeriesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::MonitorSeriesResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::SetMonitoredResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::SetMonitoredResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::SetMonitoredResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::SetMonitoredResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GetWantedResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::GetWantedResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GetWantedResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GetWantedResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::UpdateWantedResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::UpdateWantedResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::UpdateWantedResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::UpdateWantedResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListWantedResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::ListWantedResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListWantedResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListWantedResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::SearchWantedResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::SearchWantedResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::SearchWantedResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::SearchWantedResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::SyncRSSResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::SyncRSSResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::SyncRSSResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::SyncRSSResponseView<'static>,
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
pub const WANTED_SERVICE_SERVICE_NAME: &str = "mavio.acquisition.v1.WantedService";
/// Static [`Spec`](::connectrpc::Spec) for the `AddMovie` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const WANTED_SERVICE_ADD_MOVIE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.WantedService/AddMovie",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `AddSeries` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const WANTED_SERVICE_ADD_SERIES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.WantedService/AddSeries",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `MonitorSeries` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const WANTED_SERVICE_MONITOR_SERIES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.WantedService/MonitorSeries",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `SetMonitored` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const WANTED_SERVICE_SET_MONITORED_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.WantedService/SetMonitored",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `GetWanted` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const WANTED_SERVICE_GET_WANTED_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.WantedService/GetWanted",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `UpdateWanted` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const WANTED_SERVICE_UPDATE_WANTED_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.WantedService/UpdateWanted",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `ListWanted` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const WANTED_SERVICE_LIST_WANTED_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.WantedService/ListWanted",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `SearchWanted` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const WANTED_SERVICE_SEARCH_WANTED_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.WantedService/SearchWanted",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `SyncRSS` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const WANTED_SERVICE_SYNC_RSS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.WantedService/SyncRSS",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// WantedService adds the movies and series to acquire, and searches for
/// them. Administrators only.
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
pub trait WantedService: Send + Sync + 'static {
    /// AddMovie adds a movie to a movie library by its TMDB ID, as a virtual
    /// item identified by the library's metadata providers, monitored with
    /// a quality profile; it fails with ALREADY_EXISTS when the library has
    /// the movie.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn add_movie<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::AddMovieRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::AddMovieResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// AddSeries adds a series to a show library by its TVDB ID, as a
    /// virtual item identified by the library's metadata providers, which
    /// list its episodes, monitored with a quality profile; it fails with
    /// ALREADY_EXISTS when the library has the series.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn add_series<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::AddSeriesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::AddSeriesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// MonitorSeries applies a monitoring option to the seasons and episodes
    /// of wanted series, as Sonarr's season pass.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn monitor_series<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::MonitorSeriesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::MonitorSeriesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SetMonitored monitors or unmonitors seasons and episodes; a season
    /// takes its episodes along.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn set_monitored<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::SetMonitoredRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::SetMonitoredResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetWanted returns how a movie or series is acquired.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_wanted<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::GetWantedRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::GetWantedResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// UpdateWanted changes how a movie or series is acquired.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn update_wanted<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::UpdateWantedRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::UpdateWantedResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListWanted lists the monitored movies or episodes that lack a file,
    /// or whose file has not met their profile's cutoff.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_wanted<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ListWantedRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ListWantedResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SearchWanted queues searches for monitored movies or episodes, which
    /// grab the best releases they find.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn search_wanted<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::SearchWantedRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::SearchWantedResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SyncRSS queues an RSS sync: the indexers' recent releases are checked
    /// against every monitored movie and episode.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn sync_rss<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::SyncRSSRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::SyncRSSResponse,
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
pub trait WantedServiceExt: WantedService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: WantedService> WantedServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view(
                WANTED_SERVICE_SERVICE_NAME,
                "AddMovie",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::AddMovieRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::AddMovieRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.add_movie(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::AddMovieResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(WANTED_SERVICE_ADD_MOVIE_SPEC)
            .route_view(
                WANTED_SERVICE_SERVICE_NAME,
                "AddSeries",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::AddSeriesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::AddSeriesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.add_series(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::AddSeriesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(WANTED_SERVICE_ADD_SERIES_SPEC)
            .route_view(
                WANTED_SERVICE_SERVICE_NAME,
                "MonitorSeries",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::MonitorSeriesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::MonitorSeriesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.monitor_series(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::MonitorSeriesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(WANTED_SERVICE_MONITOR_SERIES_SPEC)
            .route_view(
                WANTED_SERVICE_SERVICE_NAME,
                "SetMonitored",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::SetMonitoredRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::SetMonitoredRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.set_monitored(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::SetMonitoredResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(WANTED_SERVICE_SET_MONITORED_SPEC)
            .route_view_idempotent(
                WANTED_SERVICE_SERVICE_NAME,
                "GetWanted",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::GetWantedRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::GetWantedRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_wanted(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::GetWantedResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(WANTED_SERVICE_GET_WANTED_SPEC)
            .route_view(
                WANTED_SERVICE_SERVICE_NAME,
                "UpdateWanted",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::UpdateWantedRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::UpdateWantedRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.update_wanted(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::UpdateWantedResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(WANTED_SERVICE_UPDATE_WANTED_SPEC)
            .route_view_idempotent(
                WANTED_SERVICE_SERVICE_NAME,
                "ListWanted",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ListWantedRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ListWantedRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_wanted(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ListWantedResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(WANTED_SERVICE_LIST_WANTED_SPEC)
            .route_view(
                WANTED_SERVICE_SERVICE_NAME,
                "SearchWanted",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::SearchWantedRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::SearchWantedRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.search_wanted(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::SearchWantedResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(WANTED_SERVICE_SEARCH_WANTED_SPEC)
            .route_view(
                WANTED_SERVICE_SERVICE_NAME,
                "SyncRSS",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::SyncRSSRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::SyncRSSRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.sync_rss(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::SyncRSSResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(WANTED_SERVICE_SYNC_RSS_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct WantedServiceRegisterMarker;
impl<S: WantedService> ::connectrpc::ServiceRegister<WantedServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as WantedServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `WantedService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = WantedServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct WantedServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: WantedService> WantedServiceServer<T> {
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
impl<T> Clone for WantedServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: WantedService> ::connectrpc::Dispatcher for WantedServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.acquisition.v1.WantedService/")?;
        match method {
            "AddMovie" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(WANTED_SERVICE_ADD_MOVIE_SPEC),
                )
            }
            "AddSeries" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(WANTED_SERVICE_ADD_SERIES_SPEC),
                )
            }
            "MonitorSeries" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(WANTED_SERVICE_MONITOR_SERIES_SPEC),
                )
            }
            "SetMonitored" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(WANTED_SERVICE_SET_MONITORED_SPEC),
                )
            }
            "GetWanted" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(WANTED_SERVICE_GET_WANTED_SPEC),
                )
            }
            "UpdateWanted" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(WANTED_SERVICE_UPDATE_WANTED_SPEC),
                )
            }
            "ListWanted" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(WANTED_SERVICE_LIST_WANTED_SPEC),
                )
            }
            "SearchWanted" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(WANTED_SERVICE_SEARCH_WANTED_SPEC),
                )
            }
            "SyncRSS" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(WANTED_SERVICE_SYNC_RSS_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.WantedService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "AddMovie" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::AddMovieRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::AddMovieRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::AddMovieRequest,
                    >::from_parts(&req, &body);
                    svc.add_movie(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::AddMovieResponse,
                        >(format)
                })
            }
            "AddSeries" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::AddSeriesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::AddSeriesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::AddSeriesRequest,
                    >::from_parts(&req, &body);
                    svc.add_series(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::AddSeriesResponse,
                        >(format)
                })
            }
            "MonitorSeries" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::MonitorSeriesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::MonitorSeriesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::MonitorSeriesRequest,
                    >::from_parts(&req, &body);
                    svc.monitor_series(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::MonitorSeriesResponse,
                        >(format)
                })
            }
            "SetMonitored" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::SetMonitoredRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::SetMonitoredRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::SetMonitoredRequest,
                    >::from_parts(&req, &body);
                    svc.set_monitored(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::SetMonitoredResponse,
                        >(format)
                })
            }
            "GetWanted" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::GetWantedRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::GetWantedRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::GetWantedRequest,
                    >::from_parts(&req, &body);
                    svc.get_wanted(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::GetWantedResponse,
                        >(format)
                })
            }
            "UpdateWanted" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::UpdateWantedRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::UpdateWantedRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::UpdateWantedRequest,
                    >::from_parts(&req, &body);
                    svc.update_wanted(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::UpdateWantedResponse,
                        >(format)
                })
            }
            "ListWanted" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ListWantedRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ListWantedRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ListWantedRequest,
                    >::from_parts(&req, &body);
                    svc.list_wanted(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ListWantedResponse,
                        >(format)
                })
            }
            "SearchWanted" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::SearchWantedRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::SearchWantedRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::SearchWantedRequest,
                    >::from_parts(&req, &body);
                    svc.search_wanted(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::SearchWantedResponse,
                        >(format)
                })
            }
            "SyncRSS" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::SyncRSSRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::SyncRSSRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::SyncRSSRequest,
                    >::from_parts(&req, &body);
                    svc.sync_rss(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::SyncRSSResponse,
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.WantedService/") else {
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.WantedService/") else {
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.WantedService/") else {
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
/// let client = WantedServiceClient::new(conn, config);
/// let response = client.add_movie(request).await?;
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
/// let client = WantedServiceClient::new(http, config);
/// let response = client.add_movie(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.add_movie(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.add_movie(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct WantedServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> WantedServiceClient<T>
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
    /// Call the AddMovie RPC. Sends a request to /mavio.acquisition.v1.WantedService/AddMovie.
    pub async fn add_movie(
        &self,
        request: crate::proto::mavio::acquisition::v1::AddMovieRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::AddMovieResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.add_movie_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the AddMovie RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn add_movie_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::AddMovieRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::AddMovieResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                WANTED_SERVICE_ADD_MOVIE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the AddSeries RPC. Sends a request to /mavio.acquisition.v1.WantedService/AddSeries.
    pub async fn add_series(
        &self,
        request: crate::proto::mavio::acquisition::v1::AddSeriesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::AddSeriesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.add_series_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the AddSeries RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn add_series_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::AddSeriesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::AddSeriesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                WANTED_SERVICE_ADD_SERIES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the MonitorSeries RPC. Sends a request to /mavio.acquisition.v1.WantedService/MonitorSeries.
    pub async fn monitor_series(
        &self,
        request: crate::proto::mavio::acquisition::v1::MonitorSeriesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::MonitorSeriesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.monitor_series_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the MonitorSeries RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn monitor_series_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::MonitorSeriesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::MonitorSeriesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                WANTED_SERVICE_MONITOR_SERIES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SetMonitored RPC. Sends a request to /mavio.acquisition.v1.WantedService/SetMonitored.
    pub async fn set_monitored(
        &self,
        request: crate::proto::mavio::acquisition::v1::SetMonitoredRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::SetMonitoredResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.set_monitored_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SetMonitored RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn set_monitored_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::SetMonitoredRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::SetMonitoredResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                WANTED_SERVICE_SET_MONITORED_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetWanted RPC. Sends a request to /mavio.acquisition.v1.WantedService/GetWanted.
    pub async fn get_wanted(
        &self,
        request: crate::proto::mavio::acquisition::v1::GetWantedRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GetWantedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_wanted_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetWanted RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_wanted_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::GetWantedRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GetWantedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                WANTED_SERVICE_GET_WANTED_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the UpdateWanted RPC. Sends a request to /mavio.acquisition.v1.WantedService/UpdateWanted.
    pub async fn update_wanted(
        &self,
        request: crate::proto::mavio::acquisition::v1::UpdateWantedRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::UpdateWantedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.update_wanted_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the UpdateWanted RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn update_wanted_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::UpdateWantedRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::UpdateWantedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                WANTED_SERVICE_UPDATE_WANTED_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListWanted RPC. Sends a request to /mavio.acquisition.v1.WantedService/ListWanted.
    pub async fn list_wanted(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListWantedRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListWantedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_wanted_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListWanted RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_wanted_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListWantedRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListWantedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                WANTED_SERVICE_LIST_WANTED_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SearchWanted RPC. Sends a request to /mavio.acquisition.v1.WantedService/SearchWanted.
    pub async fn search_wanted(
        &self,
        request: crate::proto::mavio::acquisition::v1::SearchWantedRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::SearchWantedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.search_wanted_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SearchWanted RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn search_wanted_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::SearchWantedRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::SearchWantedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                WANTED_SERVICE_SEARCH_WANTED_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SyncRSS RPC. Sends a request to /mavio.acquisition.v1.WantedService/SyncRSS.
    pub async fn sync_rss(
        &self,
        request: crate::proto::mavio::acquisition::v1::SyncRSSRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::SyncRSSResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.sync_rss_with_options(request, ::connectrpc::client::CallOptions::default())
            .await
    }
    /// Call the SyncRSS RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn sync_rss_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::SyncRSSRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::SyncRSSResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                WANTED_SERVICE_SYNC_RSS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

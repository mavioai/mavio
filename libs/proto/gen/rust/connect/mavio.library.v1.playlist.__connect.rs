///Shorthand for `OwnedView<ListPlaylistsRequestView<'static>>`.
pub type OwnedListPlaylistsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListPlaylistsRequestView<'static>,
>;
///Shorthand for `OwnedView<ListPlaylistsResponseView<'static>>`.
pub type OwnedListPlaylistsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListPlaylistsResponseView<'static>,
>;
///Shorthand for `OwnedView<ListPlaylistEntriesRequestView<'static>>`.
pub type OwnedListPlaylistEntriesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListPlaylistEntriesRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListPlaylistEntriesResponseView<'static>>`.
pub type OwnedListPlaylistEntriesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListPlaylistEntriesResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<CreatePlaylistRequestView<'static>>`.
pub type OwnedCreatePlaylistRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::CreatePlaylistRequestView<'static>,
>;
///Shorthand for `OwnedView<CreatePlaylistResponseView<'static>>`.
pub type OwnedCreatePlaylistResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::CreatePlaylistResponseView<'static>,
>;
///Shorthand for `OwnedView<UpdatePlaylistRequestView<'static>>`.
pub type OwnedUpdatePlaylistRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::UpdatePlaylistRequestView<'static>,
>;
///Shorthand for `OwnedView<UpdatePlaylistResponseView<'static>>`.
pub type OwnedUpdatePlaylistResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::UpdatePlaylistResponseView<'static>,
>;
///Shorthand for `OwnedView<DeletePlaylistRequestView<'static>>`.
pub type OwnedDeletePlaylistRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DeletePlaylistRequestView<'static>,
>;
///Shorthand for `OwnedView<DeletePlaylistResponseView<'static>>`.
pub type OwnedDeletePlaylistResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DeletePlaylistResponseView<'static>,
>;
///Shorthand for `OwnedView<AddToPlaylistRequestView<'static>>`.
pub type OwnedAddToPlaylistRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::AddToPlaylistRequestView<'static>,
>;
///Shorthand for `OwnedView<AddToPlaylistResponseView<'static>>`.
pub type OwnedAddToPlaylistResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::AddToPlaylistResponseView<'static>,
>;
///Shorthand for `OwnedView<RemoveFromPlaylistRequestView<'static>>`.
pub type OwnedRemoveFromPlaylistRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::RemoveFromPlaylistRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<RemoveFromPlaylistResponseView<'static>>`.
pub type OwnedRemoveFromPlaylistResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::RemoveFromPlaylistResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<MovePlaylistEntryRequestView<'static>>`.
pub type OwnedMovePlaylistEntryRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::MovePlaylistEntryRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<MovePlaylistEntryResponseView<'static>>`.
pub type OwnedMovePlaylistEntryResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::MovePlaylistEntryResponseView<
        'static,
    >,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListPlaylistsResponse>
for crate::proto::mavio::library::v1::__buffa::view::ListPlaylistsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListPlaylistsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListPlaylistsResponseView<'static>,
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
    crate::proto::mavio::library::v1::ListPlaylistEntriesResponse,
>
for crate::proto::mavio::library::v1::__buffa::view::ListPlaylistEntriesResponseView<
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
    crate::proto::mavio::library::v1::ListPlaylistEntriesResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListPlaylistEntriesResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::CreatePlaylistResponse>
for crate::proto::mavio::library::v1::__buffa::view::CreatePlaylistResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::CreatePlaylistResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::CreatePlaylistResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::UpdatePlaylistResponse>
for crate::proto::mavio::library::v1::__buffa::view::UpdatePlaylistResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::UpdatePlaylistResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::UpdatePlaylistResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::DeletePlaylistResponse>
for crate::proto::mavio::library::v1::__buffa::view::DeletePlaylistResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::DeletePlaylistResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DeletePlaylistResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::AddToPlaylistResponse>
for crate::proto::mavio::library::v1::__buffa::view::AddToPlaylistResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::AddToPlaylistResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::AddToPlaylistResponseView<'static>,
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
    crate::proto::mavio::library::v1::RemoveFromPlaylistResponse,
>
for crate::proto::mavio::library::v1::__buffa::view::RemoveFromPlaylistResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<
    crate::proto::mavio::library::v1::RemoveFromPlaylistResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::RemoveFromPlaylistResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::MovePlaylistEntryResponse>
for crate::proto::mavio::library::v1::__buffa::view::MovePlaylistEntryResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::MovePlaylistEntryResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::MovePlaylistEntryResponseView<
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
pub const PLAYLIST_SERVICE_SERVICE_NAME: &str = "mavio.library.v1.PlaylistService";
/// Static [`Spec`](::connectrpc::Spec) for the `ListPlaylists` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYLIST_SERVICE_LIST_PLAYLISTS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.PlaylistService/ListPlaylists",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListPlaylistEntries` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYLIST_SERVICE_LIST_PLAYLIST_ENTRIES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.PlaylistService/ListPlaylistEntries",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `CreatePlaylist` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYLIST_SERVICE_CREATE_PLAYLIST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.PlaylistService/CreatePlaylist",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `UpdatePlaylist` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYLIST_SERVICE_UPDATE_PLAYLIST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.PlaylistService/UpdatePlaylist",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `DeletePlaylist` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYLIST_SERVICE_DELETE_PLAYLIST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.PlaylistService/DeletePlaylist",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `AddToPlaylist` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYLIST_SERVICE_ADD_TO_PLAYLIST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.PlaylistService/AddToPlaylist",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `RemoveFromPlaylist` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYLIST_SERVICE_REMOVE_FROM_PLAYLIST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.PlaylistService/RemoveFromPlaylist",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `MovePlaylistEntry` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const PLAYLIST_SERVICE_MOVE_PLAYLIST_ENTRY_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.PlaylistService/MovePlaylistEntry",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// PlaylistService manages the caller's playlists: ordered lists of media
/// items in which an item may appear several times, each entry with its own
/// ID. Playlists are items of the playlists library that only their owner
/// sees; ItemService.GetItem returns them and ListItems with their ID as
/// parent_id lists their items once each.
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
pub trait PlaylistService: Send + Sync + 'static {
    /// Handle the ListPlaylists RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_playlists<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::ListPlaylistsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::ListPlaylistsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListPlaylistEntries lists a playlist's entries in order, leaving out
    /// items the caller may no longer access.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_playlist_entries<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::ListPlaylistEntriesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::ListPlaylistEntriesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the CreatePlaylist RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn create_playlist<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::CreatePlaylistRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::CreatePlaylistResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the UpdatePlaylist RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn update_playlist<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::UpdatePlaylistRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::UpdatePlaylistResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the DeletePlaylist RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn delete_playlist<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::DeletePlaylistRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::DeletePlaylistResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// AddToPlaylist inserts entries for items; a series, season, album or
    /// other container adds its playable items in order.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn add_to_playlist<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::AddToPlaylistRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::AddToPlaylistResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the RemoveFromPlaylist RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn remove_from_playlist<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::RemoveFromPlaylistRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::RemoveFromPlaylistResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// MovePlaylistEntry moves an entry to another position.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn move_playlist_entry<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::MovePlaylistEntryRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::MovePlaylistEntryResponse,
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
pub trait PlaylistServiceExt: PlaylistService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: PlaylistService> PlaylistServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                PLAYLIST_SERVICE_SERVICE_NAME,
                "ListPlaylists",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::ListPlaylistsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::ListPlaylistsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_playlists(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::ListPlaylistsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYLIST_SERVICE_LIST_PLAYLISTS_SPEC)
            .route_view_idempotent(
                PLAYLIST_SERVICE_SERVICE_NAME,
                "ListPlaylistEntries",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::ListPlaylistEntriesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::ListPlaylistEntriesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_playlist_entries(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::ListPlaylistEntriesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYLIST_SERVICE_LIST_PLAYLIST_ENTRIES_SPEC)
            .route_view(
                PLAYLIST_SERVICE_SERVICE_NAME,
                "CreatePlaylist",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::CreatePlaylistRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::CreatePlaylistRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.create_playlist(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::CreatePlaylistResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYLIST_SERVICE_CREATE_PLAYLIST_SPEC)
            .route_view(
                PLAYLIST_SERVICE_SERVICE_NAME,
                "UpdatePlaylist",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::UpdatePlaylistRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::UpdatePlaylistRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.update_playlist(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::UpdatePlaylistResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYLIST_SERVICE_UPDATE_PLAYLIST_SPEC)
            .route_view(
                PLAYLIST_SERVICE_SERVICE_NAME,
                "DeletePlaylist",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::DeletePlaylistRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::DeletePlaylistRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.delete_playlist(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::DeletePlaylistResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYLIST_SERVICE_DELETE_PLAYLIST_SPEC)
            .route_view(
                PLAYLIST_SERVICE_SERVICE_NAME,
                "AddToPlaylist",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::AddToPlaylistRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::AddToPlaylistRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.add_to_playlist(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::AddToPlaylistResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYLIST_SERVICE_ADD_TO_PLAYLIST_SPEC)
            .route_view(
                PLAYLIST_SERVICE_SERVICE_NAME,
                "RemoveFromPlaylist",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::RemoveFromPlaylistRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::RemoveFromPlaylistRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.remove_from_playlist(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::RemoveFromPlaylistResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYLIST_SERVICE_REMOVE_FROM_PLAYLIST_SPEC)
            .route_view(
                PLAYLIST_SERVICE_SERVICE_NAME,
                "MovePlaylistEntry",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::MovePlaylistEntryRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::MovePlaylistEntryRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.move_playlist_entry(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::MovePlaylistEntryResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(PLAYLIST_SERVICE_MOVE_PLAYLIST_ENTRY_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct PlaylistServiceRegisterMarker;
impl<S: PlaylistService> ::connectrpc::ServiceRegister<PlaylistServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as PlaylistServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `PlaylistService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = PlaylistServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct PlaylistServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: PlaylistService> PlaylistServiceServer<T> {
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
impl<T> Clone for PlaylistServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: PlaylistService> ::connectrpc::Dispatcher for PlaylistServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.library.v1.PlaylistService/")?;
        match method {
            "ListPlaylists" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(PLAYLIST_SERVICE_LIST_PLAYLISTS_SPEC),
                )
            }
            "ListPlaylistEntries" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(PLAYLIST_SERVICE_LIST_PLAYLIST_ENTRIES_SPEC),
                )
            }
            "CreatePlaylist" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(PLAYLIST_SERVICE_CREATE_PLAYLIST_SPEC),
                )
            }
            "UpdatePlaylist" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(PLAYLIST_SERVICE_UPDATE_PLAYLIST_SPEC),
                )
            }
            "DeletePlaylist" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(PLAYLIST_SERVICE_DELETE_PLAYLIST_SPEC),
                )
            }
            "AddToPlaylist" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(PLAYLIST_SERVICE_ADD_TO_PLAYLIST_SPEC),
                )
            }
            "RemoveFromPlaylist" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(PLAYLIST_SERVICE_REMOVE_FROM_PLAYLIST_SPEC),
                )
            }
            "MovePlaylistEntry" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(PLAYLIST_SERVICE_MOVE_PLAYLIST_ENTRY_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.library.v1.PlaylistService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "ListPlaylists" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::ListPlaylistsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::ListPlaylistsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::ListPlaylistsRequest,
                    >::from_parts(&req, &body);
                    svc.list_playlists(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::ListPlaylistsResponse,
                        >(format)
                })
            }
            "ListPlaylistEntries" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::ListPlaylistEntriesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::ListPlaylistEntriesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::ListPlaylistEntriesRequest,
                    >::from_parts(&req, &body);
                    svc.list_playlist_entries(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::ListPlaylistEntriesResponse,
                        >(format)
                })
            }
            "CreatePlaylist" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::CreatePlaylistRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::CreatePlaylistRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::CreatePlaylistRequest,
                    >::from_parts(&req, &body);
                    svc.create_playlist(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::CreatePlaylistResponse,
                        >(format)
                })
            }
            "UpdatePlaylist" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::UpdatePlaylistRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::UpdatePlaylistRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::UpdatePlaylistRequest,
                    >::from_parts(&req, &body);
                    svc.update_playlist(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::UpdatePlaylistResponse,
                        >(format)
                })
            }
            "DeletePlaylist" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::DeletePlaylistRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::DeletePlaylistRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::DeletePlaylistRequest,
                    >::from_parts(&req, &body);
                    svc.delete_playlist(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::DeletePlaylistResponse,
                        >(format)
                })
            }
            "AddToPlaylist" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::AddToPlaylistRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::AddToPlaylistRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::AddToPlaylistRequest,
                    >::from_parts(&req, &body);
                    svc.add_to_playlist(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::AddToPlaylistResponse,
                        >(format)
                })
            }
            "RemoveFromPlaylist" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::RemoveFromPlaylistRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::RemoveFromPlaylistRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::RemoveFromPlaylistRequest,
                    >::from_parts(&req, &body);
                    svc.remove_from_playlist(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::RemoveFromPlaylistResponse,
                        >(format)
                })
            }
            "MovePlaylistEntry" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::MovePlaylistEntryRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::MovePlaylistEntryRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::MovePlaylistEntryRequest,
                    >::from_parts(&req, &body);
                    svc.move_playlist_entry(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::MovePlaylistEntryResponse,
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
        let Some(method) = path.strip_prefix("mavio.library.v1.PlaylistService/") else {
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
        let Some(method) = path.strip_prefix("mavio.library.v1.PlaylistService/") else {
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
        let Some(method) = path.strip_prefix("mavio.library.v1.PlaylistService/") else {
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
/// let client = PlaylistServiceClient::new(conn, config);
/// let response = client.list_playlists(request).await?;
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
/// let client = PlaylistServiceClient::new(http, config);
/// let response = client.list_playlists(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.list_playlists(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.list_playlists(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct PlaylistServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> PlaylistServiceClient<T>
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
    /// Call the ListPlaylists RPC. Sends a request to /mavio.library.v1.PlaylistService/ListPlaylists.
    pub async fn list_playlists(
        &self,
        request: crate::proto::mavio::library::v1::ListPlaylistsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListPlaylistsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_playlists_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListPlaylists RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_playlists_with_options(
        &self,
        request: crate::proto::mavio::library::v1::ListPlaylistsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListPlaylistsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYLIST_SERVICE_LIST_PLAYLISTS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListPlaylistEntries RPC. Sends a request to /mavio.library.v1.PlaylistService/ListPlaylistEntries.
    pub async fn list_playlist_entries(
        &self,
        request: crate::proto::mavio::library::v1::ListPlaylistEntriesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListPlaylistEntriesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_playlist_entries_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListPlaylistEntries RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_playlist_entries_with_options(
        &self,
        request: crate::proto::mavio::library::v1::ListPlaylistEntriesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListPlaylistEntriesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYLIST_SERVICE_LIST_PLAYLIST_ENTRIES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the CreatePlaylist RPC. Sends a request to /mavio.library.v1.PlaylistService/CreatePlaylist.
    pub async fn create_playlist(
        &self,
        request: crate::proto::mavio::library::v1::CreatePlaylistRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::CreatePlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.create_playlist_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the CreatePlaylist RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn create_playlist_with_options(
        &self,
        request: crate::proto::mavio::library::v1::CreatePlaylistRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::CreatePlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYLIST_SERVICE_CREATE_PLAYLIST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the UpdatePlaylist RPC. Sends a request to /mavio.library.v1.PlaylistService/UpdatePlaylist.
    pub async fn update_playlist(
        &self,
        request: crate::proto::mavio::library::v1::UpdatePlaylistRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::UpdatePlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.update_playlist_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the UpdatePlaylist RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn update_playlist_with_options(
        &self,
        request: crate::proto::mavio::library::v1::UpdatePlaylistRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::UpdatePlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYLIST_SERVICE_UPDATE_PLAYLIST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DeletePlaylist RPC. Sends a request to /mavio.library.v1.PlaylistService/DeletePlaylist.
    pub async fn delete_playlist(
        &self,
        request: crate::proto::mavio::library::v1::DeletePlaylistRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DeletePlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.delete_playlist_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DeletePlaylist RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn delete_playlist_with_options(
        &self,
        request: crate::proto::mavio::library::v1::DeletePlaylistRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DeletePlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYLIST_SERVICE_DELETE_PLAYLIST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the AddToPlaylist RPC. Sends a request to /mavio.library.v1.PlaylistService/AddToPlaylist.
    pub async fn add_to_playlist(
        &self,
        request: crate::proto::mavio::library::v1::AddToPlaylistRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::AddToPlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.add_to_playlist_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the AddToPlaylist RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn add_to_playlist_with_options(
        &self,
        request: crate::proto::mavio::library::v1::AddToPlaylistRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::AddToPlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYLIST_SERVICE_ADD_TO_PLAYLIST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the RemoveFromPlaylist RPC. Sends a request to /mavio.library.v1.PlaylistService/RemoveFromPlaylist.
    pub async fn remove_from_playlist(
        &self,
        request: crate::proto::mavio::library::v1::RemoveFromPlaylistRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::RemoveFromPlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.remove_from_playlist_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the RemoveFromPlaylist RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn remove_from_playlist_with_options(
        &self,
        request: crate::proto::mavio::library::v1::RemoveFromPlaylistRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::RemoveFromPlaylistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYLIST_SERVICE_REMOVE_FROM_PLAYLIST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the MovePlaylistEntry RPC. Sends a request to /mavio.library.v1.PlaylistService/MovePlaylistEntry.
    pub async fn move_playlist_entry(
        &self,
        request: crate::proto::mavio::library::v1::MovePlaylistEntryRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::MovePlaylistEntryResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.move_playlist_entry_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the MovePlaylistEntry RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn move_playlist_entry_with_options(
        &self,
        request: crate::proto::mavio::library::v1::MovePlaylistEntryRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::MovePlaylistEntryResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                PLAYLIST_SERVICE_MOVE_PLAYLIST_ENTRY_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

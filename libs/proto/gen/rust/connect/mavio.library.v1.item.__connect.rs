///Shorthand for `OwnedView<GetItemRequestView<'static>>`.
pub type OwnedGetItemRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::GetItemRequestView<'static>,
>;
///Shorthand for `OwnedView<GetItemResponseView<'static>>`.
pub type OwnedGetItemResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::GetItemResponseView<'static>,
>;
///Shorthand for `OwnedView<ListItemsRequestView<'static>>`.
pub type OwnedListItemsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListItemsRequestView<'static>,
>;
///Shorthand for `OwnedView<ListItemsResponseView<'static>>`.
pub type OwnedListItemsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListItemsResponseView<'static>,
>;
///Shorthand for `OwnedView<GetPersonRequestView<'static>>`.
pub type OwnedGetPersonRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::GetPersonRequestView<'static>,
>;
///Shorthand for `OwnedView<GetPersonResponseView<'static>>`.
pub type OwnedGetPersonResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::GetPersonResponseView<'static>,
>;
///Shorthand for `OwnedView<ListValuesRequestView<'static>>`.
pub type OwnedListValuesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListValuesRequestView<'static>,
>;
///Shorthand for `OwnedView<ListValuesResponseView<'static>>`.
pub type OwnedListValuesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListValuesResponseView<'static>,
>;
///Shorthand for `OwnedView<ListLatestItemsRequestView<'static>>`.
pub type OwnedListLatestItemsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListLatestItemsRequestView<'static>,
>;
///Shorthand for `OwnedView<ListLatestItemsResponseView<'static>>`.
pub type OwnedListLatestItemsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListLatestItemsResponseView<'static>,
>;
///Shorthand for `OwnedView<ListNextUpRequestView<'static>>`.
pub type OwnedListNextUpRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListNextUpRequestView<'static>,
>;
///Shorthand for `OwnedView<ListNextUpResponseView<'static>>`.
pub type OwnedListNextUpResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListNextUpResponseView<'static>,
>;
///Shorthand for `OwnedView<ListPeopleRequestView<'static>>`.
pub type OwnedListPeopleRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListPeopleRequestView<'static>,
>;
///Shorthand for `OwnedView<ListPeopleResponseView<'static>>`.
pub type OwnedListPeopleResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListPeopleResponseView<'static>,
>;
///Shorthand for `OwnedView<GetLyricsRequestView<'static>>`.
pub type OwnedGetLyricsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::GetLyricsRequestView<'static>,
>;
///Shorthand for `OwnedView<GetLyricsResponseView<'static>>`.
pub type OwnedGetLyricsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::GetLyricsResponseView<'static>,
>;
///Shorthand for `OwnedView<SearchRemoteLyricsRequestView<'static>>`.
pub type OwnedSearchRemoteLyricsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SearchRemoteLyricsRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<SearchRemoteLyricsResponseView<'static>>`.
pub type OwnedSearchRemoteLyricsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SearchRemoteLyricsResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DownloadRemoteLyricsRequestView<'static>>`.
pub type OwnedDownloadRemoteLyricsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DownloadRemoteLyricsRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DownloadRemoteLyricsResponseView<'static>>`.
pub type OwnedDownloadRemoteLyricsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DownloadRemoteLyricsResponseView<
        'static,
    >,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::GetItemResponse>
for crate::proto::mavio::library::v1::__buffa::view::GetItemResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::GetItemResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::GetItemResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListItemsResponse>
for crate::proto::mavio::library::v1::__buffa::view::ListItemsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListItemsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListItemsResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::GetPersonResponse>
for crate::proto::mavio::library::v1::__buffa::view::GetPersonResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::GetPersonResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::GetPersonResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListValuesResponse>
for crate::proto::mavio::library::v1::__buffa::view::ListValuesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListValuesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListValuesResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListLatestItemsResponse>
for crate::proto::mavio::library::v1::__buffa::view::ListLatestItemsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListLatestItemsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListLatestItemsResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListNextUpResponse>
for crate::proto::mavio::library::v1::__buffa::view::ListNextUpResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListNextUpResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListNextUpResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListPeopleResponse>
for crate::proto::mavio::library::v1::__buffa::view::ListPeopleResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListPeopleResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListPeopleResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::GetLyricsResponse>
for crate::proto::mavio::library::v1::__buffa::view::GetLyricsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::GetLyricsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::GetLyricsResponseView<'static>,
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
    crate::proto::mavio::library::v1::SearchRemoteLyricsResponse,
>
for crate::proto::mavio::library::v1::__buffa::view::SearchRemoteLyricsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<
    crate::proto::mavio::library::v1::SearchRemoteLyricsResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SearchRemoteLyricsResponseView<
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
    crate::proto::mavio::library::v1::DownloadRemoteLyricsResponse,
>
for crate::proto::mavio::library::v1::__buffa::view::DownloadRemoteLyricsResponseView<
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
    crate::proto::mavio::library::v1::DownloadRemoteLyricsResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DownloadRemoteLyricsResponseView<
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
pub const ITEM_SERVICE_SERVICE_NAME: &str = "mavio.library.v1.ItemService";
/// Static [`Spec`](::connectrpc::Spec) for the `GetItem` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_GET_ITEM_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/GetItem",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListItems` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_LIST_ITEMS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/ListItems",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `GetPerson` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_GET_PERSON_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/GetPerson",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListValues` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_LIST_VALUES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/ListValues",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListLatestItems` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_LIST_LATEST_ITEMS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/ListLatestItems",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListNextUp` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_LIST_NEXT_UP_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/ListNextUp",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListPeople` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_LIST_PEOPLE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/ListPeople",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `GetLyrics` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_GET_LYRICS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/GetLyrics",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `SearchRemoteLyrics` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_SEARCH_REMOTE_LYRICS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/SearchRemoteLyrics",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `DownloadRemoteLyrics` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const ITEM_SERVICE_DOWNLOAD_REMOTE_LYRICS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.ItemService/DownloadRemoteLyrics",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// ItemService browses and searches library items.
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
pub trait ItemService: Send + Sync + 'static {
    /// Handle the GetItem RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_item<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::GetItemRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::GetItemResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the ListItems RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_items<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::ListItemsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::ListItemsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the GetPerson RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_person<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::GetPersonRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::GetPersonResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListValues lists the genres, tags, studios, artists or production years
    /// of the items the caller may access, with the number of items having
    /// each.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_values<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::ListValuesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::ListValuesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListLatestItems lists the items added last to the libraries the caller
    /// may access, newest first.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_latest_items<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::ListLatestItemsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::ListLatestItemsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListNextUp lists the next episode to watch of each series the caller
    /// has played, the series played most recently first. Episodes being
    /// watched are left to continue watching: ListItems with resumable set,
    /// sorted by last played.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_next_up<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::ListNextUpRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::ListNextUpResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListPeople lists the people credited on the items the caller may
    /// access, with the number of items crediting each.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_people<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::ListPeopleRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::ListPeopleResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetLyrics returns the lyrics of a track, read from the lyric file
    /// beside it (.elrc, .lrc or .txt).
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_lyrics<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::GetLyricsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::GetLyricsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SearchRemoteLyrics asks the lyrics providers for lyrics of a track;
    /// administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn search_remote_lyrics<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::SearchRemoteLyricsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::SearchRemoteLyricsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// DownloadRemoteLyrics saves lyrics SearchRemoteLyrics found beside the
    /// track, as .lrc when synced and .txt otherwise, replacing its other
    /// lyric files; administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn download_remote_lyrics<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::DownloadRemoteLyricsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::DownloadRemoteLyricsResponse,
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
pub trait ItemServiceExt: ItemService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: ItemService> ItemServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                ITEM_SERVICE_SERVICE_NAME,
                "GetItem",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::GetItemRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::GetItemRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_item(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::GetItemResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_GET_ITEM_SPEC)
            .route_view_idempotent(
                ITEM_SERVICE_SERVICE_NAME,
                "ListItems",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::ListItemsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::ListItemsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_items(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::ListItemsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_LIST_ITEMS_SPEC)
            .route_view_idempotent(
                ITEM_SERVICE_SERVICE_NAME,
                "GetPerson",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::GetPersonRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::GetPersonRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_person(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::GetPersonResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_GET_PERSON_SPEC)
            .route_view_idempotent(
                ITEM_SERVICE_SERVICE_NAME,
                "ListValues",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::ListValuesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::ListValuesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_values(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::ListValuesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_LIST_VALUES_SPEC)
            .route_view_idempotent(
                ITEM_SERVICE_SERVICE_NAME,
                "ListLatestItems",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::ListLatestItemsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::ListLatestItemsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_latest_items(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::ListLatestItemsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_LIST_LATEST_ITEMS_SPEC)
            .route_view_idempotent(
                ITEM_SERVICE_SERVICE_NAME,
                "ListNextUp",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::ListNextUpRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::ListNextUpRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_next_up(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::ListNextUpResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_LIST_NEXT_UP_SPEC)
            .route_view_idempotent(
                ITEM_SERVICE_SERVICE_NAME,
                "ListPeople",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::ListPeopleRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::ListPeopleRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_people(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::ListPeopleResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_LIST_PEOPLE_SPEC)
            .route_view_idempotent(
                ITEM_SERVICE_SERVICE_NAME,
                "GetLyrics",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::GetLyricsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::GetLyricsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_lyrics(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::GetLyricsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_GET_LYRICS_SPEC)
            .route_view_idempotent(
                ITEM_SERVICE_SERVICE_NAME,
                "SearchRemoteLyrics",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::SearchRemoteLyricsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::SearchRemoteLyricsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.search_remote_lyrics(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::SearchRemoteLyricsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_SEARCH_REMOTE_LYRICS_SPEC)
            .route_view(
                ITEM_SERVICE_SERVICE_NAME,
                "DownloadRemoteLyrics",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::DownloadRemoteLyricsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::DownloadRemoteLyricsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.download_remote_lyrics(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::DownloadRemoteLyricsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(ITEM_SERVICE_DOWNLOAD_REMOTE_LYRICS_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct ItemServiceRegisterMarker;
impl<S: ItemService> ::connectrpc::ServiceRegister<ItemServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as ItemServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `ItemService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = ItemServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct ItemServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: ItemService> ItemServiceServer<T> {
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
impl<T> Clone for ItemServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: ItemService> ::connectrpc::Dispatcher for ItemServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.library.v1.ItemService/")?;
        match method {
            "GetItem" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ITEM_SERVICE_GET_ITEM_SPEC),
                )
            }
            "ListItems" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ITEM_SERVICE_LIST_ITEMS_SPEC),
                )
            }
            "GetPerson" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ITEM_SERVICE_GET_PERSON_SPEC),
                )
            }
            "ListValues" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ITEM_SERVICE_LIST_VALUES_SPEC),
                )
            }
            "ListLatestItems" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ITEM_SERVICE_LIST_LATEST_ITEMS_SPEC),
                )
            }
            "ListNextUp" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ITEM_SERVICE_LIST_NEXT_UP_SPEC),
                )
            }
            "ListPeople" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ITEM_SERVICE_LIST_PEOPLE_SPEC),
                )
            }
            "GetLyrics" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ITEM_SERVICE_GET_LYRICS_SPEC),
                )
            }
            "SearchRemoteLyrics" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(ITEM_SERVICE_SEARCH_REMOTE_LYRICS_SPEC),
                )
            }
            "DownloadRemoteLyrics" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(ITEM_SERVICE_DOWNLOAD_REMOTE_LYRICS_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.library.v1.ItemService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "GetItem" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::GetItemRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::GetItemRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::GetItemRequest,
                    >::from_parts(&req, &body);
                    svc.get_item(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::GetItemResponse,
                        >(format)
                })
            }
            "ListItems" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::ListItemsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::ListItemsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::ListItemsRequest,
                    >::from_parts(&req, &body);
                    svc.list_items(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::ListItemsResponse,
                        >(format)
                })
            }
            "GetPerson" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::GetPersonRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::GetPersonRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::GetPersonRequest,
                    >::from_parts(&req, &body);
                    svc.get_person(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::GetPersonResponse,
                        >(format)
                })
            }
            "ListValues" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::ListValuesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::ListValuesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::ListValuesRequest,
                    >::from_parts(&req, &body);
                    svc.list_values(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::ListValuesResponse,
                        >(format)
                })
            }
            "ListLatestItems" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::ListLatestItemsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::ListLatestItemsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::ListLatestItemsRequest,
                    >::from_parts(&req, &body);
                    svc.list_latest_items(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::ListLatestItemsResponse,
                        >(format)
                })
            }
            "ListNextUp" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::ListNextUpRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::ListNextUpRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::ListNextUpRequest,
                    >::from_parts(&req, &body);
                    svc.list_next_up(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::ListNextUpResponse,
                        >(format)
                })
            }
            "ListPeople" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::ListPeopleRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::ListPeopleRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::ListPeopleRequest,
                    >::from_parts(&req, &body);
                    svc.list_people(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::ListPeopleResponse,
                        >(format)
                })
            }
            "GetLyrics" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::GetLyricsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::GetLyricsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::GetLyricsRequest,
                    >::from_parts(&req, &body);
                    svc.get_lyrics(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::GetLyricsResponse,
                        >(format)
                })
            }
            "SearchRemoteLyrics" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::SearchRemoteLyricsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::SearchRemoteLyricsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::SearchRemoteLyricsRequest,
                    >::from_parts(&req, &body);
                    svc.search_remote_lyrics(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::SearchRemoteLyricsResponse,
                        >(format)
                })
            }
            "DownloadRemoteLyrics" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::DownloadRemoteLyricsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::DownloadRemoteLyricsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::DownloadRemoteLyricsRequest,
                    >::from_parts(&req, &body);
                    svc.download_remote_lyrics(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::DownloadRemoteLyricsResponse,
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
        let Some(method) = path.strip_prefix("mavio.library.v1.ItemService/") else {
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
        let Some(method) = path.strip_prefix("mavio.library.v1.ItemService/") else {
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
        let Some(method) = path.strip_prefix("mavio.library.v1.ItemService/") else {
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
/// let client = ItemServiceClient::new(conn, config);
/// let response = client.get_item(request).await?;
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
/// let client = ItemServiceClient::new(http, config);
/// let response = client.get_item(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.get_item(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.get_item(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct ItemServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> ItemServiceClient<T>
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
    /// Call the GetItem RPC. Sends a request to /mavio.library.v1.ItemService/GetItem.
    pub async fn get_item(
        &self,
        request: crate::proto::mavio::library::v1::GetItemRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::GetItemResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_item_with_options(request, ::connectrpc::client::CallOptions::default())
            .await
    }
    /// Call the GetItem RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_item_with_options(
        &self,
        request: crate::proto::mavio::library::v1::GetItemRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::GetItemResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_GET_ITEM_SPEC.with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListItems RPC. Sends a request to /mavio.library.v1.ItemService/ListItems.
    pub async fn list_items(
        &self,
        request: crate::proto::mavio::library::v1::ListItemsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListItemsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_items_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListItems RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_items_with_options(
        &self,
        request: crate::proto::mavio::library::v1::ListItemsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListItemsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_LIST_ITEMS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetPerson RPC. Sends a request to /mavio.library.v1.ItemService/GetPerson.
    pub async fn get_person(
        &self,
        request: crate::proto::mavio::library::v1::GetPersonRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::GetPersonResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_person_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetPerson RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_person_with_options(
        &self,
        request: crate::proto::mavio::library::v1::GetPersonRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::GetPersonResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_GET_PERSON_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListValues RPC. Sends a request to /mavio.library.v1.ItemService/ListValues.
    pub async fn list_values(
        &self,
        request: crate::proto::mavio::library::v1::ListValuesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListValuesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_values_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListValues RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_values_with_options(
        &self,
        request: crate::proto::mavio::library::v1::ListValuesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListValuesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_LIST_VALUES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListLatestItems RPC. Sends a request to /mavio.library.v1.ItemService/ListLatestItems.
    pub async fn list_latest_items(
        &self,
        request: crate::proto::mavio::library::v1::ListLatestItemsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListLatestItemsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_latest_items_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListLatestItems RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_latest_items_with_options(
        &self,
        request: crate::proto::mavio::library::v1::ListLatestItemsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListLatestItemsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_LIST_LATEST_ITEMS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListNextUp RPC. Sends a request to /mavio.library.v1.ItemService/ListNextUp.
    pub async fn list_next_up(
        &self,
        request: crate::proto::mavio::library::v1::ListNextUpRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListNextUpResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_next_up_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListNextUp RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_next_up_with_options(
        &self,
        request: crate::proto::mavio::library::v1::ListNextUpRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListNextUpResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_LIST_NEXT_UP_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListPeople RPC. Sends a request to /mavio.library.v1.ItemService/ListPeople.
    pub async fn list_people(
        &self,
        request: crate::proto::mavio::library::v1::ListPeopleRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListPeopleResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_people_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListPeople RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_people_with_options(
        &self,
        request: crate::proto::mavio::library::v1::ListPeopleRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListPeopleResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_LIST_PEOPLE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetLyrics RPC. Sends a request to /mavio.library.v1.ItemService/GetLyrics.
    pub async fn get_lyrics(
        &self,
        request: crate::proto::mavio::library::v1::GetLyricsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::GetLyricsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_lyrics_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetLyrics RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_lyrics_with_options(
        &self,
        request: crate::proto::mavio::library::v1::GetLyricsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::GetLyricsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_GET_LYRICS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SearchRemoteLyrics RPC. Sends a request to /mavio.library.v1.ItemService/SearchRemoteLyrics.
    pub async fn search_remote_lyrics(
        &self,
        request: crate::proto::mavio::library::v1::SearchRemoteLyricsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::SearchRemoteLyricsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.search_remote_lyrics_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SearchRemoteLyrics RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn search_remote_lyrics_with_options(
        &self,
        request: crate::proto::mavio::library::v1::SearchRemoteLyricsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::SearchRemoteLyricsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_SEARCH_REMOTE_LYRICS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DownloadRemoteLyrics RPC. Sends a request to /mavio.library.v1.ItemService/DownloadRemoteLyrics.
    pub async fn download_remote_lyrics(
        &self,
        request: crate::proto::mavio::library::v1::DownloadRemoteLyricsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DownloadRemoteLyricsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.download_remote_lyrics_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DownloadRemoteLyrics RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn download_remote_lyrics_with_options(
        &self,
        request: crate::proto::mavio::library::v1::DownloadRemoteLyricsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DownloadRemoteLyricsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                ITEM_SERVICE_DOWNLOAD_REMOTE_LYRICS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

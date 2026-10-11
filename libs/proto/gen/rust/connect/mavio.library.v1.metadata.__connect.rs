///Shorthand for `OwnedView<UpdateItemRequestView<'static>>`.
pub type OwnedUpdateItemRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::UpdateItemRequestView<'static>,
>;
///Shorthand for `OwnedView<UpdateItemResponseView<'static>>`.
pub type OwnedUpdateItemResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::UpdateItemResponseView<'static>,
>;
///Shorthand for `OwnedView<RefreshItemRequestView<'static>>`.
pub type OwnedRefreshItemRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::RefreshItemRequestView<'static>,
>;
///Shorthand for `OwnedView<RefreshItemResponseView<'static>>`.
pub type OwnedRefreshItemResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::RefreshItemResponseView<'static>,
>;
///Shorthand for `OwnedView<SearchRemoteRequestView<'static>>`.
pub type OwnedSearchRemoteRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SearchRemoteRequestView<'static>,
>;
///Shorthand for `OwnedView<SearchRemoteResponseView<'static>>`.
pub type OwnedSearchRemoteResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SearchRemoteResponseView<'static>,
>;
///Shorthand for `OwnedView<IdentifyItemRequestView<'static>>`.
pub type OwnedIdentifyItemRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::IdentifyItemRequestView<'static>,
>;
///Shorthand for `OwnedView<IdentifyItemResponseView<'static>>`.
pub type OwnedIdentifyItemResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::IdentifyItemResponseView<'static>,
>;
///Shorthand for `OwnedView<ListRemoteImagesRequestView<'static>>`.
pub type OwnedListRemoteImagesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListRemoteImagesRequestView<'static>,
>;
///Shorthand for `OwnedView<ListRemoteImagesResponseView<'static>>`.
pub type OwnedListRemoteImagesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListRemoteImagesResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<SetItemImageRequestView<'static>>`.
pub type OwnedSetItemImageRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SetItemImageRequestView<'static>,
>;
///Shorthand for `OwnedView<SetItemImageResponseView<'static>>`.
pub type OwnedSetItemImageResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SetItemImageResponseView<'static>,
>;
///Shorthand for `OwnedView<DeleteItemImageRequestView<'static>>`.
pub type OwnedDeleteItemImageRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DeleteItemImageRequestView<'static>,
>;
///Shorthand for `OwnedView<DeleteItemImageResponseView<'static>>`.
pub type OwnedDeleteItemImageResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DeleteItemImageResponseView<'static>,
>;
///Shorthand for `OwnedView<SearchSubtitlesRequestView<'static>>`.
pub type OwnedSearchSubtitlesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SearchSubtitlesRequestView<'static>,
>;
///Shorthand for `OwnedView<SearchSubtitlesResponseView<'static>>`.
pub type OwnedSearchSubtitlesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SearchSubtitlesResponseView<'static>,
>;
///Shorthand for `OwnedView<DownloadSubtitleRequestView<'static>>`.
pub type OwnedDownloadSubtitleRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DownloadSubtitleRequestView<'static>,
>;
///Shorthand for `OwnedView<DownloadSubtitleResponseView<'static>>`.
pub type OwnedDownloadSubtitleResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DownloadSubtitleResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListExternalIdKindsRequestView<'static>>`.
pub type OwnedListExternalIdKindsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListExternalIdKindsRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListExternalIdKindsResponseView<'static>>`.
pub type OwnedListExternalIdKindsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListExternalIdKindsResponseView<
        'static,
    >,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::UpdateItemResponse>
for crate::proto::mavio::library::v1::__buffa::view::UpdateItemResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::UpdateItemResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::UpdateItemResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::RefreshItemResponse>
for crate::proto::mavio::library::v1::__buffa::view::RefreshItemResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::RefreshItemResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::RefreshItemResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::SearchRemoteResponse>
for crate::proto::mavio::library::v1::__buffa::view::SearchRemoteResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::SearchRemoteResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SearchRemoteResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::IdentifyItemResponse>
for crate::proto::mavio::library::v1::__buffa::view::IdentifyItemResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::IdentifyItemResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::IdentifyItemResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListRemoteImagesResponse>
for crate::proto::mavio::library::v1::__buffa::view::ListRemoteImagesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::ListRemoteImagesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListRemoteImagesResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::SetItemImageResponse>
for crate::proto::mavio::library::v1::__buffa::view::SetItemImageResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::SetItemImageResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SetItemImageResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::DeleteItemImageResponse>
for crate::proto::mavio::library::v1::__buffa::view::DeleteItemImageResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::DeleteItemImageResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DeleteItemImageResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::SearchSubtitlesResponse>
for crate::proto::mavio::library::v1::__buffa::view::SearchSubtitlesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::SearchSubtitlesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::SearchSubtitlesResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::DownloadSubtitleResponse>
for crate::proto::mavio::library::v1::__buffa::view::DownloadSubtitleResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::library::v1::DownloadSubtitleResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::DownloadSubtitleResponseView<
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
    crate::proto::mavio::library::v1::ListExternalIdKindsResponse,
>
for crate::proto::mavio::library::v1::__buffa::view::ListExternalIdKindsResponseView<
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
    crate::proto::mavio::library::v1::ListExternalIdKindsResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::library::v1::__buffa::view::ListExternalIdKindsResponseView<
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
pub const METADATA_SERVICE_SERVICE_NAME: &str = "mavio.library.v1.MetadataService";
/// Static [`Spec`](::connectrpc::Spec) for the `UpdateItem` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_UPDATE_ITEM_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/UpdateItem",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `RefreshItem` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_REFRESH_ITEM_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/RefreshItem",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `SearchRemote` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_SEARCH_REMOTE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/SearchRemote",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `IdentifyItem` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_IDENTIFY_ITEM_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/IdentifyItem",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListRemoteImages` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_LIST_REMOTE_IMAGES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/ListRemoteImages",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `SetItemImage` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_SET_ITEM_IMAGE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/SetItemImage",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `DeleteItemImage` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_DELETE_ITEM_IMAGE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/DeleteItemImage",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `SearchSubtitles` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_SEARCH_SUBTITLES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/SearchSubtitles",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `DownloadSubtitle` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_DOWNLOAD_SUBTITLE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/DownloadSubtitle",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListExternalIdKinds` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const METADATA_SERVICE_LIST_EXTERNAL_ID_KINDS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.library.v1.MetadataService/ListExternalIdKinds",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// MetadataService corrects and completes the metadata of items:
/// administrators edit and lock it, identify items again against the
/// metadata providers, refresh them, choose their images and download
/// subtitles. In libraries saving local metadata, changes are also written
/// next to the media as NFO files and image files.
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
pub trait MetadataService: Send + Sync + 'static {
    /// UpdateItem changes the fields of an item named by the update mask.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn update_item<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::UpdateItemRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::UpdateItemResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// RefreshItem reads an item's metadata again from its local files and
    /// the providers, sparing locked fields.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn refresh_item<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::RefreshItemRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::RefreshItemResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SearchRemote searches the providers for what an item may be.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn search_remote<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::SearchRemoteRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::SearchRemoteResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// IdentifyItem sets an item's external IDs to a search result's and
    /// replaces its metadata and images with what the providers know of
    /// them, sparing locked fields.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn identify_item<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::IdentifyItemRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::IdentifyItemResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListRemoteImages lists the images the providers have of an item.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_remote_images<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::ListRemoteImagesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::ListRemoteImagesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SetItemImage makes an image, downloaded from a URL or uploaded, the
    /// item's image of a kind; it replaces the first image of that kind.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn set_item_image<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::SetItemImageRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::SetItemImageResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// DeleteItemImage removes an image of an item, and its file when it lies
    /// in the library's folders.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn delete_item_image<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::DeleteItemImageRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::DeleteItemImageResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SearchSubtitles searches the subtitle providers for subtitles of a
    /// video in one language.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn search_subtitles<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::SearchSubtitlesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::SearchSubtitlesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// DownloadSubtitle saves a subtitle found by SearchSubtitles next to the
    /// video, where it becomes one of the video's subtitle streams.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn download_subtitle<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::DownloadSubtitleRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::DownloadSubtitleResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListExternalIdKinds lists the kinds of external IDs items and persons
    /// carry: the built-in ones and those plugins declare.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_external_id_kinds<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::library::v1::ListExternalIdKindsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::library::v1::ListExternalIdKindsResponse,
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
pub trait MetadataServiceExt: MetadataService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: MetadataService> MetadataServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view(
                METADATA_SERVICE_SERVICE_NAME,
                "UpdateItem",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::UpdateItemRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::UpdateItemRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.update_item(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::UpdateItemResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_UPDATE_ITEM_SPEC)
            .route_view(
                METADATA_SERVICE_SERVICE_NAME,
                "RefreshItem",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::RefreshItemRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::RefreshItemRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.refresh_item(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::RefreshItemResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_REFRESH_ITEM_SPEC)
            .route_view_idempotent(
                METADATA_SERVICE_SERVICE_NAME,
                "SearchRemote",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::SearchRemoteRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::SearchRemoteRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.search_remote(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::SearchRemoteResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_SEARCH_REMOTE_SPEC)
            .route_view(
                METADATA_SERVICE_SERVICE_NAME,
                "IdentifyItem",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::IdentifyItemRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::IdentifyItemRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.identify_item(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::IdentifyItemResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_IDENTIFY_ITEM_SPEC)
            .route_view_idempotent(
                METADATA_SERVICE_SERVICE_NAME,
                "ListRemoteImages",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::ListRemoteImagesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::ListRemoteImagesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_remote_images(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::ListRemoteImagesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_LIST_REMOTE_IMAGES_SPEC)
            .route_view(
                METADATA_SERVICE_SERVICE_NAME,
                "SetItemImage",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::SetItemImageRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::SetItemImageRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.set_item_image(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::SetItemImageResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_SET_ITEM_IMAGE_SPEC)
            .route_view(
                METADATA_SERVICE_SERVICE_NAME,
                "DeleteItemImage",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::DeleteItemImageRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::DeleteItemImageRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.delete_item_image(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::DeleteItemImageResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_DELETE_ITEM_IMAGE_SPEC)
            .route_view_idempotent(
                METADATA_SERVICE_SERVICE_NAME,
                "SearchSubtitles",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::SearchSubtitlesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::SearchSubtitlesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.search_subtitles(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::SearchSubtitlesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_SEARCH_SUBTITLES_SPEC)
            .route_view(
                METADATA_SERVICE_SERVICE_NAME,
                "DownloadSubtitle",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::DownloadSubtitleRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::DownloadSubtitleRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.download_subtitle(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::DownloadSubtitleResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_DOWNLOAD_SUBTITLE_SPEC)
            .route_view_idempotent(
                METADATA_SERVICE_SERVICE_NAME,
                "ListExternalIdKinds",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::library::v1::__buffa::view::ListExternalIdKindsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::library::v1::ListExternalIdKindsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_external_id_kinds(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::library::v1::ListExternalIdKindsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(METADATA_SERVICE_LIST_EXTERNAL_ID_KINDS_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct MetadataServiceRegisterMarker;
impl<S: MetadataService> ::connectrpc::ServiceRegister<MetadataServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as MetadataServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `MetadataService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = MetadataServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct MetadataServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: MetadataService> MetadataServiceServer<T> {
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
impl<T> Clone for MetadataServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: MetadataService> ::connectrpc::Dispatcher for MetadataServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.library.v1.MetadataService/")?;
        match method {
            "UpdateItem" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(METADATA_SERVICE_UPDATE_ITEM_SPEC),
                )
            }
            "RefreshItem" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(METADATA_SERVICE_REFRESH_ITEM_SPEC),
                )
            }
            "SearchRemote" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(METADATA_SERVICE_SEARCH_REMOTE_SPEC),
                )
            }
            "IdentifyItem" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(METADATA_SERVICE_IDENTIFY_ITEM_SPEC),
                )
            }
            "ListRemoteImages" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(METADATA_SERVICE_LIST_REMOTE_IMAGES_SPEC),
                )
            }
            "SetItemImage" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(METADATA_SERVICE_SET_ITEM_IMAGE_SPEC),
                )
            }
            "DeleteItemImage" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(METADATA_SERVICE_DELETE_ITEM_IMAGE_SPEC),
                )
            }
            "SearchSubtitles" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(METADATA_SERVICE_SEARCH_SUBTITLES_SPEC),
                )
            }
            "DownloadSubtitle" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(METADATA_SERVICE_DOWNLOAD_SUBTITLE_SPEC),
                )
            }
            "ListExternalIdKinds" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(METADATA_SERVICE_LIST_EXTERNAL_ID_KINDS_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.library.v1.MetadataService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "UpdateItem" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::UpdateItemRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::UpdateItemRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::UpdateItemRequest,
                    >::from_parts(&req, &body);
                    svc.update_item(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::UpdateItemResponse,
                        >(format)
                })
            }
            "RefreshItem" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::RefreshItemRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::RefreshItemRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::RefreshItemRequest,
                    >::from_parts(&req, &body);
                    svc.refresh_item(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::RefreshItemResponse,
                        >(format)
                })
            }
            "SearchRemote" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::SearchRemoteRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::SearchRemoteRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::SearchRemoteRequest,
                    >::from_parts(&req, &body);
                    svc.search_remote(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::SearchRemoteResponse,
                        >(format)
                })
            }
            "IdentifyItem" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::IdentifyItemRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::IdentifyItemRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::IdentifyItemRequest,
                    >::from_parts(&req, &body);
                    svc.identify_item(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::IdentifyItemResponse,
                        >(format)
                })
            }
            "ListRemoteImages" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::ListRemoteImagesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::ListRemoteImagesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::ListRemoteImagesRequest,
                    >::from_parts(&req, &body);
                    svc.list_remote_images(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::ListRemoteImagesResponse,
                        >(format)
                })
            }
            "SetItemImage" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::SetItemImageRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::SetItemImageRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::SetItemImageRequest,
                    >::from_parts(&req, &body);
                    svc.set_item_image(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::SetItemImageResponse,
                        >(format)
                })
            }
            "DeleteItemImage" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::DeleteItemImageRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::DeleteItemImageRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::DeleteItemImageRequest,
                    >::from_parts(&req, &body);
                    svc.delete_item_image(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::DeleteItemImageResponse,
                        >(format)
                })
            }
            "SearchSubtitles" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::SearchSubtitlesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::SearchSubtitlesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::SearchSubtitlesRequest,
                    >::from_parts(&req, &body);
                    svc.search_subtitles(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::SearchSubtitlesResponse,
                        >(format)
                })
            }
            "DownloadSubtitle" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::DownloadSubtitleRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::DownloadSubtitleRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::DownloadSubtitleRequest,
                    >::from_parts(&req, &body);
                    svc.download_subtitle(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::DownloadSubtitleResponse,
                        >(format)
                })
            }
            "ListExternalIdKinds" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::library::v1::ListExternalIdKindsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::library::v1::__buffa::view::ListExternalIdKindsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::library::v1::ListExternalIdKindsRequest,
                    >::from_parts(&req, &body);
                    svc.list_external_id_kinds(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::library::v1::ListExternalIdKindsResponse,
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
        let Some(method) = path.strip_prefix("mavio.library.v1.MetadataService/") else {
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
        let Some(method) = path.strip_prefix("mavio.library.v1.MetadataService/") else {
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
        let Some(method) = path.strip_prefix("mavio.library.v1.MetadataService/") else {
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
/// let client = MetadataServiceClient::new(conn, config);
/// let response = client.update_item(request).await?;
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
/// let client = MetadataServiceClient::new(http, config);
/// let response = client.update_item(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.update_item(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.update_item(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct MetadataServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> MetadataServiceClient<T>
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
    /// Call the UpdateItem RPC. Sends a request to /mavio.library.v1.MetadataService/UpdateItem.
    pub async fn update_item(
        &self,
        request: crate::proto::mavio::library::v1::UpdateItemRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::UpdateItemResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.update_item_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the UpdateItem RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn update_item_with_options(
        &self,
        request: crate::proto::mavio::library::v1::UpdateItemRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::UpdateItemResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_UPDATE_ITEM_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the RefreshItem RPC. Sends a request to /mavio.library.v1.MetadataService/RefreshItem.
    pub async fn refresh_item(
        &self,
        request: crate::proto::mavio::library::v1::RefreshItemRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::RefreshItemResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.refresh_item_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the RefreshItem RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn refresh_item_with_options(
        &self,
        request: crate::proto::mavio::library::v1::RefreshItemRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::RefreshItemResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_REFRESH_ITEM_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SearchRemote RPC. Sends a request to /mavio.library.v1.MetadataService/SearchRemote.
    pub async fn search_remote(
        &self,
        request: crate::proto::mavio::library::v1::SearchRemoteRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::SearchRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.search_remote_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SearchRemote RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn search_remote_with_options(
        &self,
        request: crate::proto::mavio::library::v1::SearchRemoteRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::SearchRemoteResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_SEARCH_REMOTE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the IdentifyItem RPC. Sends a request to /mavio.library.v1.MetadataService/IdentifyItem.
    pub async fn identify_item(
        &self,
        request: crate::proto::mavio::library::v1::IdentifyItemRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::IdentifyItemResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.identify_item_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the IdentifyItem RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn identify_item_with_options(
        &self,
        request: crate::proto::mavio::library::v1::IdentifyItemRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::IdentifyItemResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_IDENTIFY_ITEM_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListRemoteImages RPC. Sends a request to /mavio.library.v1.MetadataService/ListRemoteImages.
    pub async fn list_remote_images(
        &self,
        request: crate::proto::mavio::library::v1::ListRemoteImagesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListRemoteImagesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_remote_images_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListRemoteImages RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_remote_images_with_options(
        &self,
        request: crate::proto::mavio::library::v1::ListRemoteImagesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListRemoteImagesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_LIST_REMOTE_IMAGES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SetItemImage RPC. Sends a request to /mavio.library.v1.MetadataService/SetItemImage.
    pub async fn set_item_image(
        &self,
        request: crate::proto::mavio::library::v1::SetItemImageRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::SetItemImageResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.set_item_image_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SetItemImage RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn set_item_image_with_options(
        &self,
        request: crate::proto::mavio::library::v1::SetItemImageRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::SetItemImageResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_SET_ITEM_IMAGE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DeleteItemImage RPC. Sends a request to /mavio.library.v1.MetadataService/DeleteItemImage.
    pub async fn delete_item_image(
        &self,
        request: crate::proto::mavio::library::v1::DeleteItemImageRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DeleteItemImageResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.delete_item_image_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DeleteItemImage RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn delete_item_image_with_options(
        &self,
        request: crate::proto::mavio::library::v1::DeleteItemImageRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DeleteItemImageResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_DELETE_ITEM_IMAGE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SearchSubtitles RPC. Sends a request to /mavio.library.v1.MetadataService/SearchSubtitles.
    pub async fn search_subtitles(
        &self,
        request: crate::proto::mavio::library::v1::SearchSubtitlesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::SearchSubtitlesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.search_subtitles_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SearchSubtitles RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn search_subtitles_with_options(
        &self,
        request: crate::proto::mavio::library::v1::SearchSubtitlesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::SearchSubtitlesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_SEARCH_SUBTITLES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DownloadSubtitle RPC. Sends a request to /mavio.library.v1.MetadataService/DownloadSubtitle.
    pub async fn download_subtitle(
        &self,
        request: crate::proto::mavio::library::v1::DownloadSubtitleRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DownloadSubtitleResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.download_subtitle_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DownloadSubtitle RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn download_subtitle_with_options(
        &self,
        request: crate::proto::mavio::library::v1::DownloadSubtitleRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::DownloadSubtitleResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_DOWNLOAD_SUBTITLE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListExternalIdKinds RPC. Sends a request to /mavio.library.v1.MetadataService/ListExternalIdKinds.
    pub async fn list_external_id_kinds(
        &self,
        request: crate::proto::mavio::library::v1::ListExternalIdKindsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListExternalIdKindsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_external_id_kinds_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListExternalIdKinds RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_external_id_kinds_with_options(
        &self,
        request: crate::proto::mavio::library::v1::ListExternalIdKindsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::library::v1::__buffa::view::ListExternalIdKindsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                METADATA_SERVICE_LIST_EXTERNAL_ID_KINDS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

///Shorthand for `OwnedView<ListDownloadClientsRequestView<'static>>`.
pub type OwnedListDownloadClientsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListDownloadClientsRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListDownloadClientsResponseView<'static>>`.
pub type OwnedListDownloadClientsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListDownloadClientsResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<GrabReleaseRequestView<'static>>`.
pub type OwnedGrabReleaseRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GrabReleaseRequestView<'static>,
>;
///Shorthand for `OwnedView<GrabReleaseResponseView<'static>>`.
pub type OwnedGrabReleaseResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GrabReleaseResponseView<'static>,
>;
///Shorthand for `OwnedView<ListQueueRequestView<'static>>`.
pub type OwnedListQueueRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListQueueRequestView<'static>,
>;
///Shorthand for `OwnedView<ListQueueResponseView<'static>>`.
pub type OwnedListQueueResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListQueueResponseView<'static>,
>;
///Shorthand for `OwnedView<RemoveFromQueueRequestView<'static>>`.
pub type OwnedRemoveFromQueueRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::RemoveFromQueueRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<RemoveFromQueueResponseView<'static>>`.
pub type OwnedRemoveFromQueueResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::RemoveFromQueueResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListHistoryRequestView<'static>>`.
pub type OwnedListHistoryRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListHistoryRequestView<'static>,
>;
///Shorthand for `OwnedView<ListHistoryResponseView<'static>>`.
pub type OwnedListHistoryResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListHistoryResponseView<'static>,
>;
///Shorthand for `OwnedView<MarkFailedRequestView<'static>>`.
pub type OwnedMarkFailedRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::MarkFailedRequestView<'static>,
>;
///Shorthand for `OwnedView<MarkFailedResponseView<'static>>`.
pub type OwnedMarkFailedResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::MarkFailedResponseView<'static>,
>;
///Shorthand for `OwnedView<ListBlocklistRequestView<'static>>`.
pub type OwnedListBlocklistRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListBlocklistRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListBlocklistResponseView<'static>>`.
pub type OwnedListBlocklistResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListBlocklistResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DeleteBlocklistRequestView<'static>>`.
pub type OwnedDeleteBlocklistRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeleteBlocklistRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<DeleteBlocklistResponseView<'static>>`.
pub type OwnedDeleteBlocklistResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeleteBlocklistResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListManualImportRequestView<'static>>`.
pub type OwnedListManualImportRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListManualImportRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListManualImportResponseView<'static>>`.
pub type OwnedListManualImportResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListManualImportResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ManualImportRequestView<'static>>`.
pub type OwnedManualImportRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ManualImportRequestView<'static>,
>;
///Shorthand for `OwnedView<ManualImportResponseView<'static>>`.
pub type OwnedManualImportResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ManualImportResponseView<
        'static,
    >,
>;
impl ::connectrpc::Encodable<
    crate::proto::mavio::acquisition::v1::ListDownloadClientsResponse,
>
for crate::proto::mavio::acquisition::v1::__buffa::view::ListDownloadClientsResponseView<
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
    crate::proto::mavio::acquisition::v1::ListDownloadClientsResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListDownloadClientsResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GrabReleaseResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::GrabReleaseResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::GrabReleaseResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::GrabReleaseResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListQueueResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::ListQueueResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListQueueResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListQueueResponseView<'static>,
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
    crate::proto::mavio::acquisition::v1::RemoveFromQueueResponse,
>
for crate::proto::mavio::acquisition::v1::__buffa::view::RemoveFromQueueResponseView<
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
    crate::proto::mavio::acquisition::v1::RemoveFromQueueResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::RemoveFromQueueResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListHistoryResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::ListHistoryResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListHistoryResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListHistoryResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::MarkFailedResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::MarkFailedResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::MarkFailedResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::MarkFailedResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListBlocklistResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::ListBlocklistResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ListBlocklistResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListBlocklistResponseView<
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
    crate::proto::mavio::acquisition::v1::DeleteBlocklistResponse,
>
for crate::proto::mavio::acquisition::v1::__buffa::view::DeleteBlocklistResponseView<
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
    crate::proto::mavio::acquisition::v1::DeleteBlocklistResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::DeleteBlocklistResponseView<
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
    crate::proto::mavio::acquisition::v1::ListManualImportResponse,
>
for crate::proto::mavio::acquisition::v1::__buffa::view::ListManualImportResponseView<
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
    crate::proto::mavio::acquisition::v1::ListManualImportResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ListManualImportResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ManualImportResponse>
for crate::proto::mavio::acquisition::v1::__buffa::view::ManualImportResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::acquisition::v1::ManualImportResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::acquisition::v1::__buffa::view::ManualImportResponseView<
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
pub const DOWNLOAD_SERVICE_SERVICE_NAME: &str = "mavio.acquisition.v1.DownloadService";
/// Static [`Spec`](::connectrpc::Spec) for the `ListDownloadClients` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_LIST_DOWNLOAD_CLIENTS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/ListDownloadClients",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `GrabRelease` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_GRAB_RELEASE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/GrabRelease",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListQueue` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_LIST_QUEUE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/ListQueue",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `RemoveFromQueue` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_REMOVE_FROM_QUEUE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/RemoveFromQueue",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListHistory` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_LIST_HISTORY_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/ListHistory",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `MarkFailed` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_MARK_FAILED_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/MarkFailed",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListBlocklist` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_LIST_BLOCKLIST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/ListBlocklist",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `DeleteBlocklist` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_DELETE_BLOCKLIST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/DeleteBlocklist",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `ListManualImport` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_LIST_MANUAL_IMPORT_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/ListManualImport",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ManualImport` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const DOWNLOAD_SERVICE_MANUAL_IMPORT_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.acquisition.v1.DownloadService/ManualImport",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// DownloadService sends releases to the download clients of the download
/// client provider plugins and tracks them until they are imported: the
/// queue, the history and the blocklist. Administrators only.
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
pub trait DownloadService: Send + Sync + 'static {
    /// ListDownloadClients lists the clients every ready download client
    /// provider has configured.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_download_clients<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ListDownloadClientsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ListDownloadClientsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GrabRelease sends a release SearchReleases found for a movie to a
    /// download client, whatever its decision rejected, and tracks it.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn grab_release<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::GrabReleaseRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::GrabReleaseResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListQueue lists the tracked downloads, with what their clients last
    /// told of them.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_queue<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ListQueueRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ListQueueResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// RemoveFromQueue stops tracking a download, optionally removing it
    /// from its client, blocklisting its release and searching again.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn remove_from_queue<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::RemoveFromQueueRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::RemoveFromQueueResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the ListHistory RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_history<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ListHistoryRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ListHistoryResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// MarkFailed marks the download of a grab as failed: its release is
    /// blocklisted and, as the settings say, searched for again.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn mark_failed<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::MarkFailedRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::MarkFailedResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the ListBlocklist RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_blocklist<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ListBlocklistRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ListBlocklistResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// Handle the DeleteBlocklist RPC.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn delete_blocklist<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::DeleteBlocklistRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::DeleteBlocklistResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListManualImport lists the video files of a tracked movie download,
    /// or of a folder that is not one, with the movie each is for, the
    /// quality, languages and release group an import would give them and
    /// why the import specifications reject them, as Radarr's manual import.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_manual_import<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ListManualImportRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ListManualImportResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ManualImport imports a video file of a tracked movie download, or of
    /// a folder that is not one, for the movie chosen whatever the import
    /// specifications reject, and completes a tracked download.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn manual_import<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::acquisition::v1::ManualImportRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::acquisition::v1::ManualImportResponse,
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
pub trait DownloadServiceExt: DownloadService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: DownloadService> DownloadServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "ListDownloadClients",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ListDownloadClientsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ListDownloadClientsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_download_clients(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ListDownloadClientsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_LIST_DOWNLOAD_CLIENTS_SPEC)
            .route_view(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "GrabRelease",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::GrabReleaseRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::GrabReleaseRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.grab_release(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::GrabReleaseResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_GRAB_RELEASE_SPEC)
            .route_view_idempotent(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "ListQueue",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ListQueueRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ListQueueRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_queue(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ListQueueResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_LIST_QUEUE_SPEC)
            .route_view(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "RemoveFromQueue",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::RemoveFromQueueRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::RemoveFromQueueRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.remove_from_queue(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::RemoveFromQueueResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_REMOVE_FROM_QUEUE_SPEC)
            .route_view_idempotent(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "ListHistory",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ListHistoryRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ListHistoryRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_history(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ListHistoryResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_LIST_HISTORY_SPEC)
            .route_view(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "MarkFailed",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::MarkFailedRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::MarkFailedRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.mark_failed(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::MarkFailedResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_MARK_FAILED_SPEC)
            .route_view_idempotent(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "ListBlocklist",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ListBlocklistRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ListBlocklistRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_blocklist(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ListBlocklistResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_LIST_BLOCKLIST_SPEC)
            .route_view(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "DeleteBlocklist",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::DeleteBlocklistRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::DeleteBlocklistRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.delete_blocklist(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::DeleteBlocklistResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_DELETE_BLOCKLIST_SPEC)
            .route_view_idempotent(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "ListManualImport",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ListManualImportRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ListManualImportRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_manual_import(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ListManualImportResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_LIST_MANUAL_IMPORT_SPEC)
            .route_view(
                DOWNLOAD_SERVICE_SERVICE_NAME,
                "ManualImport",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::acquisition::v1::__buffa::view::ManualImportRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::acquisition::v1::ManualImportRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.manual_import(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::acquisition::v1::ManualImportResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(DOWNLOAD_SERVICE_MANUAL_IMPORT_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct DownloadServiceRegisterMarker;
impl<S: DownloadService> ::connectrpc::ServiceRegister<DownloadServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as DownloadServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `DownloadService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = DownloadServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct DownloadServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: DownloadService> DownloadServiceServer<T> {
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
impl<T> Clone for DownloadServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: DownloadService> ::connectrpc::Dispatcher for DownloadServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.acquisition.v1.DownloadService/")?;
        match method {
            "ListDownloadClients" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(DOWNLOAD_SERVICE_LIST_DOWNLOAD_CLIENTS_SPEC),
                )
            }
            "GrabRelease" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(DOWNLOAD_SERVICE_GRAB_RELEASE_SPEC),
                )
            }
            "ListQueue" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(DOWNLOAD_SERVICE_LIST_QUEUE_SPEC),
                )
            }
            "RemoveFromQueue" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(DOWNLOAD_SERVICE_REMOVE_FROM_QUEUE_SPEC),
                )
            }
            "ListHistory" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(DOWNLOAD_SERVICE_LIST_HISTORY_SPEC),
                )
            }
            "MarkFailed" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(DOWNLOAD_SERVICE_MARK_FAILED_SPEC),
                )
            }
            "ListBlocklist" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(DOWNLOAD_SERVICE_LIST_BLOCKLIST_SPEC),
                )
            }
            "DeleteBlocklist" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(DOWNLOAD_SERVICE_DELETE_BLOCKLIST_SPEC),
                )
            }
            "ListManualImport" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(DOWNLOAD_SERVICE_LIST_MANUAL_IMPORT_SPEC),
                )
            }
            "ManualImport" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(DOWNLOAD_SERVICE_MANUAL_IMPORT_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.DownloadService/")
        else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "ListDownloadClients" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ListDownloadClientsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ListDownloadClientsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ListDownloadClientsRequest,
                    >::from_parts(&req, &body);
                    svc.list_download_clients(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ListDownloadClientsResponse,
                        >(format)
                })
            }
            "GrabRelease" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::GrabReleaseRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::GrabReleaseRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::GrabReleaseRequest,
                    >::from_parts(&req, &body);
                    svc.grab_release(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::GrabReleaseResponse,
                        >(format)
                })
            }
            "ListQueue" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ListQueueRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ListQueueRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ListQueueRequest,
                    >::from_parts(&req, &body);
                    svc.list_queue(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ListQueueResponse,
                        >(format)
                })
            }
            "RemoveFromQueue" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::RemoveFromQueueRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::RemoveFromQueueRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::RemoveFromQueueRequest,
                    >::from_parts(&req, &body);
                    svc.remove_from_queue(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::RemoveFromQueueResponse,
                        >(format)
                })
            }
            "ListHistory" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ListHistoryRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ListHistoryRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ListHistoryRequest,
                    >::from_parts(&req, &body);
                    svc.list_history(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ListHistoryResponse,
                        >(format)
                })
            }
            "MarkFailed" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::MarkFailedRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::MarkFailedRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::MarkFailedRequest,
                    >::from_parts(&req, &body);
                    svc.mark_failed(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::MarkFailedResponse,
                        >(format)
                })
            }
            "ListBlocklist" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ListBlocklistRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ListBlocklistRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ListBlocklistRequest,
                    >::from_parts(&req, &body);
                    svc.list_blocklist(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ListBlocklistResponse,
                        >(format)
                })
            }
            "DeleteBlocklist" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::DeleteBlocklistRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::DeleteBlocklistRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::DeleteBlocklistRequest,
                    >::from_parts(&req, &body);
                    svc.delete_blocklist(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::DeleteBlocklistResponse,
                        >(format)
                })
            }
            "ListManualImport" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ListManualImportRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ListManualImportRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ListManualImportRequest,
                    >::from_parts(&req, &body);
                    svc.list_manual_import(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ListManualImportResponse,
                        >(format)
                })
            }
            "ManualImport" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::acquisition::v1::ManualImportRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::acquisition::v1::__buffa::view::ManualImportRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::acquisition::v1::ManualImportRequest,
                    >::from_parts(&req, &body);
                    svc.manual_import(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::acquisition::v1::ManualImportResponse,
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.DownloadService/")
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.DownloadService/")
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
        let Some(method) = path.strip_prefix("mavio.acquisition.v1.DownloadService/")
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
/// let client = DownloadServiceClient::new(conn, config);
/// let response = client.list_download_clients(request).await?;
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
/// let client = DownloadServiceClient::new(http, config);
/// let response = client.list_download_clients(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.list_download_clients(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.list_download_clients(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct DownloadServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> DownloadServiceClient<T>
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
    /// Call the ListDownloadClients RPC. Sends a request to /mavio.acquisition.v1.DownloadService/ListDownloadClients.
    pub async fn list_download_clients(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListDownloadClientsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListDownloadClientsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_download_clients_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListDownloadClients RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_download_clients_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListDownloadClientsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListDownloadClientsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_LIST_DOWNLOAD_CLIENTS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GrabRelease RPC. Sends a request to /mavio.acquisition.v1.DownloadService/GrabRelease.
    pub async fn grab_release(
        &self,
        request: crate::proto::mavio::acquisition::v1::GrabReleaseRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GrabReleaseResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.grab_release_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GrabRelease RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn grab_release_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::GrabReleaseRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::GrabReleaseResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_GRAB_RELEASE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListQueue RPC. Sends a request to /mavio.acquisition.v1.DownloadService/ListQueue.
    pub async fn list_queue(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListQueueRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListQueueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_queue_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListQueue RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_queue_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListQueueRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListQueueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_LIST_QUEUE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the RemoveFromQueue RPC. Sends a request to /mavio.acquisition.v1.DownloadService/RemoveFromQueue.
    pub async fn remove_from_queue(
        &self,
        request: crate::proto::mavio::acquisition::v1::RemoveFromQueueRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::RemoveFromQueueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.remove_from_queue_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the RemoveFromQueue RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn remove_from_queue_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::RemoveFromQueueRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::RemoveFromQueueResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_REMOVE_FROM_QUEUE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListHistory RPC. Sends a request to /mavio.acquisition.v1.DownloadService/ListHistory.
    pub async fn list_history(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListHistoryRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListHistoryResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_history_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListHistory RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_history_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListHistoryRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListHistoryResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_LIST_HISTORY_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the MarkFailed RPC. Sends a request to /mavio.acquisition.v1.DownloadService/MarkFailed.
    pub async fn mark_failed(
        &self,
        request: crate::proto::mavio::acquisition::v1::MarkFailedRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::MarkFailedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.mark_failed_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the MarkFailed RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn mark_failed_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::MarkFailedRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::MarkFailedResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_MARK_FAILED_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListBlocklist RPC. Sends a request to /mavio.acquisition.v1.DownloadService/ListBlocklist.
    pub async fn list_blocklist(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListBlocklistRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListBlocklistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_blocklist_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListBlocklist RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_blocklist_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListBlocklistRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListBlocklistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_LIST_BLOCKLIST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the DeleteBlocklist RPC. Sends a request to /mavio.acquisition.v1.DownloadService/DeleteBlocklist.
    pub async fn delete_blocklist(
        &self,
        request: crate::proto::mavio::acquisition::v1::DeleteBlocklistRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::DeleteBlocklistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.delete_blocklist_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the DeleteBlocklist RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn delete_blocklist_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::DeleteBlocklistRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::DeleteBlocklistResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_DELETE_BLOCKLIST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListManualImport RPC. Sends a request to /mavio.acquisition.v1.DownloadService/ListManualImport.
    pub async fn list_manual_import(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListManualImportRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListManualImportResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_manual_import_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListManualImport RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_manual_import_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ListManualImportRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ListManualImportResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_LIST_MANUAL_IMPORT_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ManualImport RPC. Sends a request to /mavio.acquisition.v1.DownloadService/ManualImport.
    pub async fn manual_import(
        &self,
        request: crate::proto::mavio::acquisition::v1::ManualImportRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ManualImportResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.manual_import_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ManualImport RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn manual_import_with_options(
        &self,
        request: crate::proto::mavio::acquisition::v1::ManualImportRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::acquisition::v1::__buffa::view::ManualImportResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                DOWNLOAD_SERVICE_MANUAL_IMPORT_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

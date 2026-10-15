///Shorthand for `OwnedView<CreateAuthorizationRequestRequestView<'static>>`.
pub type OwnedCreateAuthorizationRequestRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::CreateAuthorizationRequestRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<CreateAuthorizationRequestResponseView<'static>>`.
pub type OwnedCreateAuthorizationRequestResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::CreateAuthorizationRequestResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ClaimServerRequestView<'static>>`.
pub type OwnedClaimServerRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerRequestView<'static>,
>;
///Shorthand for `OwnedView<ClaimServerResponseView<'static>>`.
pub type OwnedClaimServerResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<'static>,
>;
///Shorthand for `OwnedView<ListChangesRequestView<'static>>`.
pub type OwnedListChangesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ListChangesRequestView<'static>,
>;
///Shorthand for `OwnedView<ListChangesResponseView<'static>>`.
pub type OwnedListChangesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<'static>,
>;
///Shorthand for `OwnedView<PublishLibrariesRequestView<'static>>`.
pub type OwnedPublishLibrariesRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::PublishLibrariesRequestView<'static>,
>;
///Shorthand for `OwnedView<PublishLibrariesResponseView<'static>>`.
pub type OwnedPublishLibrariesResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::PublishLibrariesResponseView<'static>,
>;
///Shorthand for `OwnedView<PublishConnectionsRequestView<'static>>`.
pub type OwnedPublishConnectionsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::PublishConnectionsRequestView<'static>,
>;
///Shorthand for `OwnedView<PublishConnectionsResponseView<'static>>`.
pub type OwnedPublishConnectionsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::PublishConnectionsResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<SetAcmeChallengeRequestView<'static>>`.
pub type OwnedSetAcmeChallengeRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::SetAcmeChallengeRequestView<'static>,
>;
///Shorthand for `OwnedView<SetAcmeChallengeResponseView<'static>>`.
pub type OwnedSetAcmeChallengeResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::SetAcmeChallengeResponseView<'static>,
>;
///Shorthand for `OwnedView<ClearAcmeChallengeRequestView<'static>>`.
pub type OwnedClearAcmeChallengeRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ClearAcmeChallengeRequestView<'static>,
>;
///Shorthand for `OwnedView<ClearAcmeChallengeResponseView<'static>>`.
pub type OwnedClearAcmeChallengeResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ClearAcmeChallengeResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<CreateAcmeBindingRequestView<'static>>`.
pub type OwnedCreateAcmeBindingRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::CreateAcmeBindingRequestView<'static>,
>;
///Shorthand for `OwnedView<CreateAcmeBindingResponseView<'static>>`.
pub type OwnedCreateAcmeBindingResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::CreateAcmeBindingResponseView<'static>,
>;
///Shorthand for `OwnedView<ListRelaysRequestView<'static>>`.
pub type OwnedListRelaysRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ListRelaysRequestView<'static>,
>;
///Shorthand for `OwnedView<ListRelaysResponseView<'static>>`.
pub type OwnedListRelaysResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ListRelaysResponseView<'static>,
>;
///Shorthand for `OwnedView<SetRelayRegionRequestView<'static>>`.
pub type OwnedSetRelayRegionRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::SetRelayRegionRequestView<'static>,
>;
///Shorthand for `OwnedView<SetRelayRegionResponseView<'static>>`.
pub type OwnedSetRelayRegionResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::SetRelayRegionResponseView<'static>,
>;
impl ::connectrpc::Encodable<
    crate::proto::mavio::cloud::v1::CreateAuthorizationRequestResponse,
>
for crate::proto::mavio::cloud::v1::__buffa::view::CreateAuthorizationRequestResponseView<
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
    crate::proto::mavio::cloud::v1::CreateAuthorizationRequestResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::CreateAuthorizationRequestResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ClaimServerResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ClaimServerResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ListChangesResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ListChangesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::PublishLibrariesResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::PublishLibrariesResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::PublishLibrariesResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::PublishLibrariesResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::PublishConnectionsResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::PublishConnectionsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::PublishConnectionsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::PublishConnectionsResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::SetAcmeChallengeResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::SetAcmeChallengeResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::SetAcmeChallengeResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::SetAcmeChallengeResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ClearAcmeChallengeResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::ClearAcmeChallengeResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ClearAcmeChallengeResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ClearAcmeChallengeResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::CreateAcmeBindingResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::CreateAcmeBindingResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::CreateAcmeBindingResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::CreateAcmeBindingResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ListRelaysResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::ListRelaysResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::ListRelaysResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::ListRelaysResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::SetRelayRegionResponse>
for crate::proto::mavio::cloud::v1::__buffa::view::SetRelayRegionResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::cloud::v1::SetRelayRegionResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::cloud::v1::__buffa::view::SetRelayRegionResponseView<'static>,
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
pub const SERVER_SERVICE_SERVICE_NAME: &str = "mavio.cloud.v1.ServerService";
/// Static [`Spec`](::connectrpc::Spec) for the `CreateAuthorizationRequest` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_CREATE_AUTHORIZATION_REQUEST_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/CreateAuthorizationRequest",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ClaimServer` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_CLAIM_SERVER_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/ClaimServer",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListChanges` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_LIST_CHANGES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/ListChanges",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `PublishLibraries` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_PUBLISH_LIBRARIES_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/PublishLibraries",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `PublishConnections` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_PUBLISH_CONNECTIONS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/PublishConnections",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `SetAcmeChallenge` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_SET_ACME_CHALLENGE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/SetAcmeChallenge",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `ClearAcmeChallenge` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_CLEAR_ACME_CHALLENGE_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/ClearAcmeChallenge",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// Static [`Spec`](::connectrpc::Spec) for the `CreateAcmeBinding` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_CREATE_ACME_BINDING_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/CreateAcmeBinding",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListRelays` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_LIST_RELAYS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/ListRelays",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `SetRelayRegion` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SERVER_SERVICE_SET_RELAY_REGION_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.cloud.v1.ServerService/SetRelayRegion",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Idempotent);
/// ServerService is mavio.dev's API for servers in mavio.dev mode
/// (docs/accounts.md §3.2, docs/mavio-dev.md §3). Every request is
/// authenticated by a JWT the server signs with its identity key
/// (private_key_jwt): EdDSA, with the server's ID as its kid, iss and sub,
/// mavio.dev's API base URL as its aud, a random jti, and an expiry at
/// most a minute after it was issued, carried as a bearer token. Claiming
/// registers the key, and CreateAuthorizationRequest and ClaimServer carry
/// it; the other methods need a claimed server.
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
pub trait ServerService: Send + Sync + 'static {
    /// CreateAuthorizationRequest pushes the authorization request of the
    /// server's web UI at a redirect URI (docs/accounts.md §6), as RFC 9126
    /// does, and returns the /authorize page where the account approves it.
    /// Approving sends the browser to the redirect URI with an authorization
    /// code of the account for the server and the device key. A request that
    /// claims the server registers it for the account that approves it,
    /// which becomes its administrator (docs/accounts.md §3.2), and the
    /// server then fetches the outcome with ClaimServer; any other request
    /// needs a claimed server and signs in only its administrator.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn create_authorization_request<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::CreateAuthorizationRequestRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::CreateAuthorizationRequestResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ClaimServer gives the account that claimed the server, which becomes
    /// its administrator: with a claim code from mavio.dev/claim, it claims
    /// the server for the code's account, and with an authorization request
    /// that claims the server it gives the account that approved the
    /// request. A request not approved yet fails with FAILED_PRECONDITION;
    /// an unknown, used or expired code, and an unknown, declined or expired
    /// request, with INVALID_ARGUMENT.
    /// Claiming a server again, after its command line unclaimed it, keeps
    /// its changes.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn claim_server<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::ClaimServerRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::ClaimServerResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListChanges gives the server's changes after a cursor, oldest first.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_changes<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::ListChangesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::ListChangesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// PublishLibraries replaces the libraries mavio.dev knows of the
    /// server, which access records name, so that administrators edit access
    /// from anywhere.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn publish_libraries<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::PublishLibrariesRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::PublishLibrariesResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// PublishConnections replaces the URLs clients reach the server at,
    /// which mavio.dev gives the accounts that reach it.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn publish_connections<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::PublishConnectionsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::PublishConnectionsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SetAcmeChallenge publishes the TXT values of the server's ACME DNS-01
    /// challenge at \<ID\>.acme.mavio.dev, the target of
    /// _acme-challenge.\<ID\>.mavio.site, replacing earlier ones
    /// (docs/mavio-dev.md §4.2).
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn set_acme_challenge<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::SetAcmeChallengeRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::SetAcmeChallengeResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ClearAcmeChallenge removes the TXT values once the challenge is done.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn clear_acme_challenge<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::ClearAcmeChallengeRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::ClearAcmeChallengeResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// CreateAcmeBinding gives an external account binding, which a
    /// certificate authority requiring one, such as Google Trust Services,
    /// needs to register the server's ACME account.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn create_acme_binding<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::CreateAcmeBindingRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::CreateAcmeBindingResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListRelays gives mavio.dev's relays, one per region, for the server to
    /// keep a connection to the nearest (docs/mavio-dev.md §5).
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_relays<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::ListRelaysRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::ListRelaysResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SetRelayRegion records the region of the relay the server keeps a
    /// connection to; empty when it keeps none, and no tickets are given for
    /// it then.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn set_relay_region<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::cloud::v1::SetRelayRegionRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::cloud::v1::SetRelayRegionResponse,
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
pub trait ServerServiceExt: ServerService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: ServerService> ServerServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view(
                SERVER_SERVICE_SERVICE_NAME,
                "CreateAuthorizationRequest",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::CreateAuthorizationRequestRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::CreateAuthorizationRequestRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.create_authorization_request(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::CreateAuthorizationRequestResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_CREATE_AUTHORIZATION_REQUEST_SPEC)
            .route_view(
                SERVER_SERVICE_SERVICE_NAME,
                "ClaimServer",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::ClaimServerRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.claim_server(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::ClaimServerResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_CLAIM_SERVER_SPEC)
            .route_view_idempotent(
                SERVER_SERVICE_SERVICE_NAME,
                "ListChanges",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::ListChangesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::ListChangesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_changes(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::ListChangesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_LIST_CHANGES_SPEC)
            .route_view(
                SERVER_SERVICE_SERVICE_NAME,
                "PublishLibraries",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::PublishLibrariesRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::PublishLibrariesRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.publish_libraries(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::PublishLibrariesResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_PUBLISH_LIBRARIES_SPEC)
            .route_view(
                SERVER_SERVICE_SERVICE_NAME,
                "PublishConnections",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::PublishConnectionsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::PublishConnectionsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.publish_connections(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::PublishConnectionsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_PUBLISH_CONNECTIONS_SPEC)
            .route_view(
                SERVER_SERVICE_SERVICE_NAME,
                "SetAcmeChallenge",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::SetAcmeChallengeRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::SetAcmeChallengeRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.set_acme_challenge(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::SetAcmeChallengeResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_SET_ACME_CHALLENGE_SPEC)
            .route_view(
                SERVER_SERVICE_SERVICE_NAME,
                "ClearAcmeChallenge",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::ClearAcmeChallengeRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::ClearAcmeChallengeRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.clear_acme_challenge(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::ClearAcmeChallengeResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_CLEAR_ACME_CHALLENGE_SPEC)
            .route_view(
                SERVER_SERVICE_SERVICE_NAME,
                "CreateAcmeBinding",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::CreateAcmeBindingRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::CreateAcmeBindingRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.create_acme_binding(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::CreateAcmeBindingResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_CREATE_ACME_BINDING_SPEC)
            .route_view_idempotent(
                SERVER_SERVICE_SERVICE_NAME,
                "ListRelays",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::ListRelaysRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::ListRelaysRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_relays(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::ListRelaysResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_LIST_RELAYS_SPEC)
            .route_view(
                SERVER_SERVICE_SERVICE_NAME,
                "SetRelayRegion",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::cloud::v1::__buffa::view::SetRelayRegionRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::cloud::v1::SetRelayRegionRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.set_relay_region(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::cloud::v1::SetRelayRegionResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SERVER_SERVICE_SET_RELAY_REGION_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct ServerServiceRegisterMarker;
impl<S: ServerService> ::connectrpc::ServiceRegister<ServerServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as ServerServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `ServerService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = ServerServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct ServerServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: ServerService> ServerServiceServer<T> {
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
impl<T> Clone for ServerServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: ServerService> ::connectrpc::Dispatcher for ServerServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.cloud.v1.ServerService/")?;
        match method {
            "CreateAuthorizationRequest" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SERVER_SERVICE_CREATE_AUTHORIZATION_REQUEST_SPEC),
                )
            }
            "ClaimServer" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SERVER_SERVICE_CLAIM_SERVER_SPEC),
                )
            }
            "ListChanges" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SERVER_SERVICE_LIST_CHANGES_SPEC),
                )
            }
            "PublishLibraries" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SERVER_SERVICE_PUBLISH_LIBRARIES_SPEC),
                )
            }
            "PublishConnections" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SERVER_SERVICE_PUBLISH_CONNECTIONS_SPEC),
                )
            }
            "SetAcmeChallenge" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SERVER_SERVICE_SET_ACME_CHALLENGE_SPEC),
                )
            }
            "ClearAcmeChallenge" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SERVER_SERVICE_CLEAR_ACME_CHALLENGE_SPEC),
                )
            }
            "CreateAcmeBinding" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SERVER_SERVICE_CREATE_ACME_BINDING_SPEC),
                )
            }
            "ListRelays" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SERVER_SERVICE_LIST_RELAYS_SPEC),
                )
            }
            "SetRelayRegion" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SERVER_SERVICE_SET_RELAY_REGION_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.cloud.v1.ServerService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "CreateAuthorizationRequest" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::CreateAuthorizationRequestRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::CreateAuthorizationRequestRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::CreateAuthorizationRequestRequest,
                    >::from_parts(&req, &body);
                    svc.create_authorization_request(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::CreateAuthorizationRequestResponse,
                        >(format)
                })
            }
            "ClaimServer" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::ClaimServerRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::ClaimServerRequest,
                    >::from_parts(&req, &body);
                    svc.claim_server(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::ClaimServerResponse,
                        >(format)
                })
            }
            "ListChanges" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::ListChangesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::ListChangesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::ListChangesRequest,
                    >::from_parts(&req, &body);
                    svc.list_changes(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::ListChangesResponse,
                        >(format)
                })
            }
            "PublishLibraries" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::PublishLibrariesRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::PublishLibrariesRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::PublishLibrariesRequest,
                    >::from_parts(&req, &body);
                    svc.publish_libraries(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::PublishLibrariesResponse,
                        >(format)
                })
            }
            "PublishConnections" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::PublishConnectionsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::PublishConnectionsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::PublishConnectionsRequest,
                    >::from_parts(&req, &body);
                    svc.publish_connections(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::PublishConnectionsResponse,
                        >(format)
                })
            }
            "SetAcmeChallenge" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::SetAcmeChallengeRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::SetAcmeChallengeRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::SetAcmeChallengeRequest,
                    >::from_parts(&req, &body);
                    svc.set_acme_challenge(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::SetAcmeChallengeResponse,
                        >(format)
                })
            }
            "ClearAcmeChallenge" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::ClearAcmeChallengeRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::ClearAcmeChallengeRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::ClearAcmeChallengeRequest,
                    >::from_parts(&req, &body);
                    svc.clear_acme_challenge(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::ClearAcmeChallengeResponse,
                        >(format)
                })
            }
            "CreateAcmeBinding" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::CreateAcmeBindingRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::CreateAcmeBindingRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::CreateAcmeBindingRequest,
                    >::from_parts(&req, &body);
                    svc.create_acme_binding(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::CreateAcmeBindingResponse,
                        >(format)
                })
            }
            "ListRelays" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::ListRelaysRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::ListRelaysRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::ListRelaysRequest,
                    >::from_parts(&req, &body);
                    svc.list_relays(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::ListRelaysResponse,
                        >(format)
                })
            }
            "SetRelayRegion" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::cloud::v1::SetRelayRegionRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::cloud::v1::__buffa::view::SetRelayRegionRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::cloud::v1::SetRelayRegionRequest,
                    >::from_parts(&req, &body);
                    svc.set_relay_region(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::cloud::v1::SetRelayRegionResponse,
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
        let Some(method) = path.strip_prefix("mavio.cloud.v1.ServerService/") else {
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
        let Some(method) = path.strip_prefix("mavio.cloud.v1.ServerService/") else {
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
        let Some(method) = path.strip_prefix("mavio.cloud.v1.ServerService/") else {
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
/// let client = ServerServiceClient::new(conn, config);
/// let response = client.create_authorization_request(request).await?;
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
/// let client = ServerServiceClient::new(http, config);
/// let response = client.create_authorization_request(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.create_authorization_request(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.create_authorization_request(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct ServerServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> ServerServiceClient<T>
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
    /// Call the CreateAuthorizationRequest RPC. Sends a request to /mavio.cloud.v1.ServerService/CreateAuthorizationRequest.
    pub async fn create_authorization_request(
        &self,
        request: crate::proto::mavio::cloud::v1::CreateAuthorizationRequestRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::CreateAuthorizationRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.create_authorization_request_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the CreateAuthorizationRequest RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn create_authorization_request_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::CreateAuthorizationRequestRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::CreateAuthorizationRequestResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_CREATE_AUTHORIZATION_REQUEST_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ClaimServer RPC. Sends a request to /mavio.cloud.v1.ServerService/ClaimServer.
    pub async fn claim_server(
        &self,
        request: crate::proto::mavio::cloud::v1::ClaimServerRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.claim_server_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ClaimServer RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn claim_server_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::ClaimServerRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ClaimServerResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_CLAIM_SERVER_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListChanges RPC. Sends a request to /mavio.cloud.v1.ServerService/ListChanges.
    pub async fn list_changes(
        &self,
        request: crate::proto::mavio::cloud::v1::ListChangesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_changes_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListChanges RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_changes_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::ListChangesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ListChangesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_LIST_CHANGES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the PublishLibraries RPC. Sends a request to /mavio.cloud.v1.ServerService/PublishLibraries.
    pub async fn publish_libraries(
        &self,
        request: crate::proto::mavio::cloud::v1::PublishLibrariesRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::PublishLibrariesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.publish_libraries_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the PublishLibraries RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn publish_libraries_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::PublishLibrariesRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::PublishLibrariesResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_PUBLISH_LIBRARIES_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the PublishConnections RPC. Sends a request to /mavio.cloud.v1.ServerService/PublishConnections.
    pub async fn publish_connections(
        &self,
        request: crate::proto::mavio::cloud::v1::PublishConnectionsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::PublishConnectionsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.publish_connections_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the PublishConnections RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn publish_connections_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::PublishConnectionsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::PublishConnectionsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_PUBLISH_CONNECTIONS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SetAcmeChallenge RPC. Sends a request to /mavio.cloud.v1.ServerService/SetAcmeChallenge.
    pub async fn set_acme_challenge(
        &self,
        request: crate::proto::mavio::cloud::v1::SetAcmeChallengeRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::SetAcmeChallengeResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.set_acme_challenge_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SetAcmeChallenge RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn set_acme_challenge_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::SetAcmeChallengeRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::SetAcmeChallengeResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_SET_ACME_CHALLENGE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ClearAcmeChallenge RPC. Sends a request to /mavio.cloud.v1.ServerService/ClearAcmeChallenge.
    pub async fn clear_acme_challenge(
        &self,
        request: crate::proto::mavio::cloud::v1::ClearAcmeChallengeRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ClearAcmeChallengeResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.clear_acme_challenge_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ClearAcmeChallenge RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn clear_acme_challenge_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::ClearAcmeChallengeRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ClearAcmeChallengeResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_CLEAR_ACME_CHALLENGE_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the CreateAcmeBinding RPC. Sends a request to /mavio.cloud.v1.ServerService/CreateAcmeBinding.
    pub async fn create_acme_binding(
        &self,
        request: crate::proto::mavio::cloud::v1::CreateAcmeBindingRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::CreateAcmeBindingResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.create_acme_binding_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the CreateAcmeBinding RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn create_acme_binding_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::CreateAcmeBindingRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::CreateAcmeBindingResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_CREATE_ACME_BINDING_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListRelays RPC. Sends a request to /mavio.cloud.v1.ServerService/ListRelays.
    pub async fn list_relays(
        &self,
        request: crate::proto::mavio::cloud::v1::ListRelaysRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ListRelaysResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_relays_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListRelays RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_relays_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::ListRelaysRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::ListRelaysResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_LIST_RELAYS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SetRelayRegion RPC. Sends a request to /mavio.cloud.v1.ServerService/SetRelayRegion.
    pub async fn set_relay_region(
        &self,
        request: crate::proto::mavio::cloud::v1::SetRelayRegionRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::SetRelayRegionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.set_relay_region_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SetRelayRegion RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn set_relay_region_with_options(
        &self,
        request: crate::proto::mavio::cloud::v1::SetRelayRegionRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::cloud::v1::__buffa::view::SetRelayRegionResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SERVER_SERVICE_SET_RELAY_REGION_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

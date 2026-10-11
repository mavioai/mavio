///Shorthand for `OwnedView<GetHealthRequestView<'static>>`.
pub type OwnedGetHealthRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetHealthRequestView<'static>,
>;
///Shorthand for `OwnedView<GetHealthResponseView<'static>>`.
pub type OwnedGetHealthResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetHealthResponseView<'static>,
>;
///Shorthand for `OwnedView<GetSystemInfoRequestView<'static>>`.
pub type OwnedGetSystemInfoRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetSystemInfoRequestView<'static>,
>;
///Shorthand for `OwnedView<GetSystemInfoResponseView<'static>>`.
pub type OwnedGetSystemInfoResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetSystemInfoResponseView<'static>,
>;
///Shorthand for `OwnedView<ListPluginsRequestView<'static>>`.
pub type OwnedListPluginsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListPluginsRequestView<'static>,
>;
///Shorthand for `OwnedView<ListPluginsResponseView<'static>>`.
pub type OwnedListPluginsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListPluginsResponseView<'static>,
>;
///Shorthand for `OwnedView<GetPluginConfigRequestView<'static>>`.
pub type OwnedGetPluginConfigRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetPluginConfigRequestView<'static>,
>;
///Shorthand for `OwnedView<GetPluginConfigResponseView<'static>>`.
pub type OwnedGetPluginConfigResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetPluginConfigResponseView<'static>,
>;
///Shorthand for `OwnedView<SetPluginConfigRequestView<'static>>`.
pub type OwnedSetPluginConfigRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::SetPluginConfigRequestView<'static>,
>;
///Shorthand for `OwnedView<SetPluginConfigResponseView<'static>>`.
pub type OwnedSetPluginConfigResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::SetPluginConfigResponseView<'static>,
>;
///Shorthand for `OwnedView<ListCatalogPluginsRequestView<'static>>`.
pub type OwnedListCatalogPluginsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListCatalogPluginsRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListCatalogPluginsResponseView<'static>>`.
pub type OwnedListCatalogPluginsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListCatalogPluginsResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<InstallPluginRequestView<'static>>`.
pub type OwnedInstallPluginRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::InstallPluginRequestView<'static>,
>;
///Shorthand for `OwnedView<InstallPluginResponseView<'static>>`.
pub type OwnedInstallPluginResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::InstallPluginResponseView<'static>,
>;
///Shorthand for `OwnedView<UninstallPluginRequestView<'static>>`.
pub type OwnedUninstallPluginRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::UninstallPluginRequestView<'static>,
>;
///Shorthand for `OwnedView<UninstallPluginResponseView<'static>>`.
pub type OwnedUninstallPluginResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::UninstallPluginResponseView<'static>,
>;
///Shorthand for `OwnedView<GetServerSettingsRequestView<'static>>`.
pub type OwnedGetServerSettingsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetServerSettingsRequestView<'static>,
>;
///Shorthand for `OwnedView<GetServerSettingsResponseView<'static>>`.
pub type OwnedGetServerSettingsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetServerSettingsResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<UpdateServerSettingsRequestView<'static>>`.
pub type OwnedUpdateServerSettingsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::UpdateServerSettingsRequestView<
        'static,
    >,
>;
///Shorthand for `OwnedView<UpdateServerSettingsResponseView<'static>>`.
pub type OwnedUpdateServerSettingsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::UpdateServerSettingsResponseView<
        'static,
    >,
>;
///Shorthand for `OwnedView<ListDirectoryRequestView<'static>>`.
pub type OwnedListDirectoryRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListDirectoryRequestView<'static>,
>;
///Shorthand for `OwnedView<ListDirectoryResponseView<'static>>`.
pub type OwnedListDirectoryResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListDirectoryResponseView<'static>,
>;
///Shorthand for `OwnedView<ListLogsRequestView<'static>>`.
pub type OwnedListLogsRequestView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListLogsRequestView<'static>,
>;
///Shorthand for `OwnedView<ListLogsResponseView<'static>>`.
pub type OwnedListLogsResponseView = ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListLogsResponseView<'static>,
>;
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetHealthResponse>
for crate::proto::mavio::system::v1::__buffa::view::GetHealthResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetHealthResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetHealthResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetSystemInfoResponse>
for crate::proto::mavio::system::v1::__buffa::view::GetSystemInfoResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetSystemInfoResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetSystemInfoResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListPluginsResponse>
for crate::proto::mavio::system::v1::__buffa::view::ListPluginsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListPluginsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListPluginsResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetPluginConfigResponse>
for crate::proto::mavio::system::v1::__buffa::view::GetPluginConfigResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetPluginConfigResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetPluginConfigResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::SetPluginConfigResponse>
for crate::proto::mavio::system::v1::__buffa::view::SetPluginConfigResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::SetPluginConfigResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::SetPluginConfigResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListCatalogPluginsResponse>
for crate::proto::mavio::system::v1::__buffa::view::ListCatalogPluginsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListCatalogPluginsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListCatalogPluginsResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::InstallPluginResponse>
for crate::proto::mavio::system::v1::__buffa::view::InstallPluginResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::InstallPluginResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::InstallPluginResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::UninstallPluginResponse>
for crate::proto::mavio::system::v1::__buffa::view::UninstallPluginResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::UninstallPluginResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::UninstallPluginResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetServerSettingsResponse>
for crate::proto::mavio::system::v1::__buffa::view::GetServerSettingsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::GetServerSettingsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::GetServerSettingsResponseView<
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
    crate::proto::mavio::system::v1::UpdateServerSettingsResponse,
>
for crate::proto::mavio::system::v1::__buffa::view::UpdateServerSettingsResponseView<
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
    crate::proto::mavio::system::v1::UpdateServerSettingsResponse,
>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::UpdateServerSettingsResponseView<
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListDirectoryResponse>
for crate::proto::mavio::system::v1::__buffa::view::ListDirectoryResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListDirectoryResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListDirectoryResponseView<'static>,
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
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListLogsResponse>
for crate::proto::mavio::system::v1::__buffa::view::ListLogsResponseView<'_> {
    fn encode(
        &self,
        codec: ::connectrpc::CodecFormat,
    ) -> ::std::result::Result<::buffa::bytes::Bytes, ::connectrpc::ConnectError> {
        ::connectrpc::__codegen::encode_view_body(self, codec)
    }
}
impl ::connectrpc::Encodable<crate::proto::mavio::system::v1::ListLogsResponse>
for ::buffa::view::OwnedView<
    crate::proto::mavio::system::v1::__buffa::view::ListLogsResponseView<'static>,
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
pub const SYSTEM_SERVICE_SERVICE_NAME: &str = "mavio.system.v1.SystemService";
/// Static [`Spec`](::connectrpc::Spec) for the `GetHealth` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_GET_HEALTH_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/GetHealth",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `GetSystemInfo` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_GET_SYSTEM_INFO_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/GetSystemInfo",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListPlugins` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_LIST_PLUGINS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/ListPlugins",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `GetPluginConfig` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_GET_PLUGIN_CONFIG_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/GetPluginConfig",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `SetPluginConfig` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_SET_PLUGIN_CONFIG_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/SetPluginConfig",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListCatalogPlugins` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_LIST_CATALOG_PLUGINS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/ListCatalogPlugins",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `InstallPlugin` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_INSTALL_PLUGIN_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/InstallPlugin",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `UninstallPlugin` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_UNINSTALL_PLUGIN_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/UninstallPlugin",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `GetServerSettings` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_GET_SERVER_SETTINGS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/GetServerSettings",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `UpdateServerSettings` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_UPDATE_SERVER_SETTINGS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/UpdateServerSettings",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::Unknown);
/// Static [`Spec`](::connectrpc::Spec) for the `ListDirectory` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_LIST_DIRECTORY_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/ListDirectory",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// Static [`Spec`](::connectrpc::Spec) for the `ListLogs` RPC, as seen by the server; the generated client passes it with [`origin`](::connectrpc::Spec::origin) `Client` (compare across sides with [`Spec::same_method`](::connectrpc::Spec::same_method)).
pub const SYSTEM_SERVICE_LIST_LOGS_SPEC: ::connectrpc::Spec = ::connectrpc::Spec::server(
        "/mavio.system.v1.SystemService/ListLogs",
        ::connectrpc::StreamType::Unary,
    )
    .with_idempotency_level(::connectrpc::IdempotencyLevel::NoSideEffects);
/// SystemService exposes server-level information and health, and the
/// plugins the server runs.
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
pub trait SystemService: Send + Sync + 'static {
    /// GetHealth reports whether the server is ready to serve requests.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_health<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::GetHealthRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::GetHealthResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetSystemInfo describes the running server.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_system_info<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::GetSystemInfoRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::GetSystemInfoResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListPlugins lists the plugins found in the server's plugin folder, by
    /// ID. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_plugins<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::ListPluginsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::ListPluginsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetPluginConfig returns a plugin's configuration. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_plugin_config<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::GetPluginConfigRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::GetPluginConfigResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// SetPluginConfig validates a configuration against the plugin's schema,
    /// delivers it to the running plugin and stores it once the plugin
    /// accepts it. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn set_plugin_config<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::SetPluginConfigRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::SetPluginConfigResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListCatalogPlugins lists the plugins of the configured plugin
    /// catalogs, with the versions the server can run. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_catalog_plugins<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::ListCatalogPluginsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::ListCatalogPluginsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// InstallPlugin installs a plugin from a catalog, or upgrades or
    /// downgrades it to a version, while the server runs on; the plugin keeps
    /// its configuration. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn install_plugin<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::InstallPluginRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::InstallPluginResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// UninstallPlugin stops a plugin and removes its folder and
    /// configuration. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn uninstall_plugin<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::UninstallPluginRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::UninstallPluginResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// GetServerSettings returns the settings administrators change while
    /// the server runs. Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn get_server_settings<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::GetServerSettingsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::GetServerSettingsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// UpdateServerSettings replaces the settings and applies them at once.
    /// Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn update_server_settings<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::UpdateServerSettingsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::UpdateServerSettingsResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListDirectory lists a folder of the server's file system, to choose
    /// library folders; without a path, the file system roots. Administrators
    /// only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_directory<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::ListDirectoryRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::ListDirectoryResponse,
            > + Send + use<'a, Self>,
        >,
    > + Send;
    /// ListLogs lists the server's recent log records, newest first.
    /// Administrators only.
    ///
    /// `'a` lets the response body borrow from `&self` (e.g. server-resident state).
    ///
    /// `request` is borrowed from the request body and is valid for the
    /// duration of the call; message fields are read directly on it
    /// (zero-copy). The response cannot borrow from `request` — use
    /// `.to_owned_message()` (or copy the specific fields) for anything
    /// returned, stored, or moved into `tokio::spawn`.
    fn list_logs<'a>(
        &'a self,
        ctx: ::connectrpc::RequestContext,
        request: ::connectrpc::ServiceRequest<
            '_,
            crate::proto::mavio::system::v1::ListLogsRequest,
        >,
    ) -> impl ::std::future::Future<
        Output = ::connectrpc::ServiceResult<
            impl ::connectrpc::Encodable<
                crate::proto::mavio::system::v1::ListLogsResponse,
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
pub trait SystemServiceExt: SystemService {
    /// Register this service implementation with a Router.
    ///
    /// Takes ownership of the `Arc<Self>` and returns a new Router with
    /// this service's methods registered.
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router;
}
impl<S: SystemService> SystemServiceExt for S {
    fn register(
        self: ::std::sync::Arc<Self>,
        router: ::connectrpc::Router,
    ) -> ::connectrpc::Router {
        router
            .route_view_idempotent(
                SYSTEM_SERVICE_SERVICE_NAME,
                "GetHealth",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::GetHealthRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::GetHealthRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_health(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::GetHealthResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_GET_HEALTH_SPEC)
            .route_view_idempotent(
                SYSTEM_SERVICE_SERVICE_NAME,
                "GetSystemInfo",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::GetSystemInfoRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::GetSystemInfoRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_system_info(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::GetSystemInfoResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_GET_SYSTEM_INFO_SPEC)
            .route_view_idempotent(
                SYSTEM_SERVICE_SERVICE_NAME,
                "ListPlugins",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::ListPluginsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::ListPluginsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_plugins(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::ListPluginsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_LIST_PLUGINS_SPEC)
            .route_view_idempotent(
                SYSTEM_SERVICE_SERVICE_NAME,
                "GetPluginConfig",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::GetPluginConfigRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::GetPluginConfigRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_plugin_config(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::GetPluginConfigResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_GET_PLUGIN_CONFIG_SPEC)
            .route_view(
                SYSTEM_SERVICE_SERVICE_NAME,
                "SetPluginConfig",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::SetPluginConfigRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::SetPluginConfigRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.set_plugin_config(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::SetPluginConfigResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_SET_PLUGIN_CONFIG_SPEC)
            .route_view_idempotent(
                SYSTEM_SERVICE_SERVICE_NAME,
                "ListCatalogPlugins",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::ListCatalogPluginsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::ListCatalogPluginsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_catalog_plugins(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::ListCatalogPluginsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_LIST_CATALOG_PLUGINS_SPEC)
            .route_view(
                SYSTEM_SERVICE_SERVICE_NAME,
                "InstallPlugin",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::InstallPluginRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::InstallPluginRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.install_plugin(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::InstallPluginResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_INSTALL_PLUGIN_SPEC)
            .route_view(
                SYSTEM_SERVICE_SERVICE_NAME,
                "UninstallPlugin",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::UninstallPluginRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::UninstallPluginRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.uninstall_plugin(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::UninstallPluginResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_UNINSTALL_PLUGIN_SPEC)
            .route_view_idempotent(
                SYSTEM_SERVICE_SERVICE_NAME,
                "GetServerSettings",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::GetServerSettingsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::GetServerSettingsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.get_server_settings(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::GetServerSettingsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_GET_SERVER_SETTINGS_SPEC)
            .route_view(
                SYSTEM_SERVICE_SERVICE_NAME,
                "UpdateServerSettings",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::UpdateServerSettingsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::UpdateServerSettingsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.update_server_settings(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::UpdateServerSettingsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_UPDATE_SERVER_SETTINGS_SPEC)
            .route_view_idempotent(
                SYSTEM_SERVICE_SERVICE_NAME,
                "ListDirectory",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::ListDirectoryRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::ListDirectoryRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_directory(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::ListDirectoryResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_LIST_DIRECTORY_SPEC)
            .route_view_idempotent(
                SYSTEM_SERVICE_SERVICE_NAME,
                "ListLogs",
                {
                    let svc = ::std::sync::Arc::clone(&self);
                    ::connectrpc::view_handler_fn(move |
                        ctx,
                        req: ::buffa::view::OwnedView<
                            crate::proto::mavio::system::v1::__buffa::view::ListLogsRequestView<
                                'static,
                            >,
                        >,
                        format|
                    {
                        let svc = ::std::sync::Arc::clone(&svc);
                        async move {
                            let sreq = ::connectrpc::ServiceRequest::<
                                crate::proto::mavio::system::v1::ListLogsRequest,
                            >::from_parts(req.reborrow(), req.bytes());
                            svc.list_logs(ctx, sreq)
                                .await?
                                .encode::<
                                    crate::proto::mavio::system::v1::ListLogsResponse,
                                >(format)
                        }
                    })
                },
            )
            .with_spec(SYSTEM_SERVICE_LIST_LOGS_SPEC)
    }
}
/// Type-inference marker used by [`Router::add_service`](::connectrpc::Router::add_service).
#[doc(hidden)]
pub struct SystemServiceRegisterMarker;
impl<S: SystemService> ::connectrpc::ServiceRegister<SystemServiceRegisterMarker>
for ::std::sync::Arc<S> {
    fn register_service(self, router: ::connectrpc::Router) -> ::connectrpc::Router {
        <S as SystemServiceExt>::register(self, router)
    }
}
/// Monomorphic dispatcher for `SystemService`.
///
/// Unlike `.register(Router)` which type-erases each method into an `Arc<dyn ErasedHandler>` stored in a `HashMap`, this struct dispatches via a compile-time `match` on method name: no vtable, no hash lookup.
///
/// # Example
///
/// ```rust,ignore
/// use connectrpc::ConnectRpcService;
///
/// let server = SystemServiceServer::new(MyImpl);
/// let service = ConnectRpcService::new(server);
/// // hand `service` to axum/hyper as a fallback_service
/// ```
pub struct SystemServiceServer<T> {
    inner: ::std::sync::Arc<T>,
}
impl<T: SystemService> SystemServiceServer<T> {
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
impl<T> Clone for SystemServiceServer<T> {
    fn clone(&self) -> Self {
        Self {
            inner: ::std::sync::Arc::clone(&self.inner),
        }
    }
}
impl<T: SystemService> ::connectrpc::Dispatcher for SystemServiceServer<T> {
    #[inline]
    fn lookup(
        &self,
        path: &str,
    ) -> Option<::connectrpc::dispatcher::codegen::MethodDescriptor> {
        let method = path.strip_prefix("mavio.system.v1.SystemService/")?;
        match method {
            "GetHealth" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SYSTEM_SERVICE_GET_HEALTH_SPEC),
                )
            }
            "GetSystemInfo" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SYSTEM_SERVICE_GET_SYSTEM_INFO_SPEC),
                )
            }
            "ListPlugins" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SYSTEM_SERVICE_LIST_PLUGINS_SPEC),
                )
            }
            "GetPluginConfig" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SYSTEM_SERVICE_GET_PLUGIN_CONFIG_SPEC),
                )
            }
            "SetPluginConfig" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SYSTEM_SERVICE_SET_PLUGIN_CONFIG_SPEC),
                )
            }
            "ListCatalogPlugins" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SYSTEM_SERVICE_LIST_CATALOG_PLUGINS_SPEC),
                )
            }
            "InstallPlugin" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SYSTEM_SERVICE_INSTALL_PLUGIN_SPEC),
                )
            }
            "UninstallPlugin" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SYSTEM_SERVICE_UNINSTALL_PLUGIN_SPEC),
                )
            }
            "GetServerSettings" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SYSTEM_SERVICE_GET_SERVER_SETTINGS_SPEC),
                )
            }
            "UpdateServerSettings" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(false)
                        .with_spec(SYSTEM_SERVICE_UPDATE_SERVER_SETTINGS_SPEC),
                )
            }
            "ListDirectory" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SYSTEM_SERVICE_LIST_DIRECTORY_SPEC),
                )
            }
            "ListLogs" => {
                Some(
                    ::connectrpc::dispatcher::codegen::MethodDescriptor::unary(true)
                        .with_spec(SYSTEM_SERVICE_LIST_LOGS_SPEC),
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
        let Some(method) = path.strip_prefix("mavio.system.v1.SystemService/") else {
            return ::connectrpc::dispatcher::codegen::unimplemented_unary(path);
        };
        let _ = (&ctx, &request, &format);
        match method {
            "GetHealth" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::GetHealthRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::GetHealthRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::GetHealthRequest,
                    >::from_parts(&req, &body);
                    svc.get_health(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::GetHealthResponse,
                        >(format)
                })
            }
            "GetSystemInfo" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::GetSystemInfoRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::GetSystemInfoRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::GetSystemInfoRequest,
                    >::from_parts(&req, &body);
                    svc.get_system_info(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::GetSystemInfoResponse,
                        >(format)
                })
            }
            "ListPlugins" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::ListPluginsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::ListPluginsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::ListPluginsRequest,
                    >::from_parts(&req, &body);
                    svc.list_plugins(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::ListPluginsResponse,
                        >(format)
                })
            }
            "GetPluginConfig" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::GetPluginConfigRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::GetPluginConfigRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::GetPluginConfigRequest,
                    >::from_parts(&req, &body);
                    svc.get_plugin_config(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::GetPluginConfigResponse,
                        >(format)
                })
            }
            "SetPluginConfig" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::SetPluginConfigRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::SetPluginConfigRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::SetPluginConfigRequest,
                    >::from_parts(&req, &body);
                    svc.set_plugin_config(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::SetPluginConfigResponse,
                        >(format)
                })
            }
            "ListCatalogPlugins" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::ListCatalogPluginsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::ListCatalogPluginsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::ListCatalogPluginsRequest,
                    >::from_parts(&req, &body);
                    svc.list_catalog_plugins(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::ListCatalogPluginsResponse,
                        >(format)
                })
            }
            "InstallPlugin" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::InstallPluginRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::InstallPluginRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::InstallPluginRequest,
                    >::from_parts(&req, &body);
                    svc.install_plugin(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::InstallPluginResponse,
                        >(format)
                })
            }
            "UninstallPlugin" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::UninstallPluginRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::UninstallPluginRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::UninstallPluginRequest,
                    >::from_parts(&req, &body);
                    svc.uninstall_plugin(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::UninstallPluginResponse,
                        >(format)
                })
            }
            "GetServerSettings" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::GetServerSettingsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::GetServerSettingsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::GetServerSettingsRequest,
                    >::from_parts(&req, &body);
                    svc.get_server_settings(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::GetServerSettingsResponse,
                        >(format)
                })
            }
            "UpdateServerSettings" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::UpdateServerSettingsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::UpdateServerSettingsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::UpdateServerSettingsRequest,
                    >::from_parts(&req, &body);
                    svc.update_server_settings(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::UpdateServerSettingsResponse,
                        >(format)
                })
            }
            "ListDirectory" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::ListDirectoryRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::ListDirectoryRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::ListDirectoryRequest,
                    >::from_parts(&req, &body);
                    svc.list_directory(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::ListDirectoryResponse,
                        >(format)
                })
            }
            "ListLogs" => {
                let svc = ::std::sync::Arc::clone(&self.inner);
                Box::pin(async move {
                    let body = ::connectrpc::dispatcher::codegen::request_proto_bytes::<
                        crate::proto::mavio::system::v1::ListLogsRequest,
                    >(request.encoded()?, format)?;
                    let req: crate::proto::mavio::system::v1::__buffa::view::ListLogsRequestView<
                        '_,
                    > = ::connectrpc::dispatcher::codegen::decode_borrowed_request_view(
                        &body,
                        ctx.decode_options(),
                    )?;
                    let req = ::connectrpc::ServiceRequest::<
                        crate::proto::mavio::system::v1::ListLogsRequest,
                    >::from_parts(&req, &body);
                    svc.list_logs(ctx, req)
                        .await?
                        .encode::<
                            crate::proto::mavio::system::v1::ListLogsResponse,
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
        let Some(method) = path.strip_prefix("mavio.system.v1.SystemService/") else {
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
        let Some(method) = path.strip_prefix("mavio.system.v1.SystemService/") else {
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
        let Some(method) = path.strip_prefix("mavio.system.v1.SystemService/") else {
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
/// let client = SystemServiceClient::new(conn, config);
/// let response = client.get_health(request).await?;
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
/// let client = SystemServiceClient::new(http, config);
/// let response = client.get_health(request).await?;
/// ```
///
/// # Working with the response
///
/// Unary calls return [`UnaryResponse<OwnedView<FooView>>`](::connectrpc::client::UnaryResponse).
/// [`view()`](::connectrpc::client::UnaryResponse::view) borrows the response
/// message, so field access is zero-copy:
///
/// ```rust,ignore
/// let resp = client.get_health(request).await?;
/// let name: &str = resp.view().name;  // borrow into the response buffer
/// ```
///
/// If you need the owned struct (e.g. to store or pass by value), use
/// [`into_owned()`](::connectrpc::client::UnaryResponse::into_owned):
///
/// ```rust,ignore
/// let owned = client.get_health(request).await?.into_owned();
/// ```
///
/// [`into_view()`](::connectrpc::client::UnaryResponse::into_view) keeps the
/// zero-copy decoded body (an `OwnedView`) without copying; field access on it
/// goes through `.reborrow()`. Streaming responses yield one
/// [`StreamMessage`](::connectrpc::StreamMessage) per received message from
/// `.message().await` — read fields zero-copy through the generated accessor
/// methods (`msg.name()`) or `.view()`, or convert with `.to_owned_message()`.
#[derive(Clone)]
pub struct SystemServiceClient<T> {
    transport: T,
    config: ::connectrpc::client::ClientConfig,
}
impl<T> SystemServiceClient<T>
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
    /// Call the GetHealth RPC. Sends a request to /mavio.system.v1.SystemService/GetHealth.
    pub async fn get_health(
        &self,
        request: crate::proto::mavio::system::v1::GetHealthRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetHealthResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_health_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetHealth RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_health_with_options(
        &self,
        request: crate::proto::mavio::system::v1::GetHealthRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetHealthResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_GET_HEALTH_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetSystemInfo RPC. Sends a request to /mavio.system.v1.SystemService/GetSystemInfo.
    pub async fn get_system_info(
        &self,
        request: crate::proto::mavio::system::v1::GetSystemInfoRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetSystemInfoResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_system_info_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetSystemInfo RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_system_info_with_options(
        &self,
        request: crate::proto::mavio::system::v1::GetSystemInfoRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetSystemInfoResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_GET_SYSTEM_INFO_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListPlugins RPC. Sends a request to /mavio.system.v1.SystemService/ListPlugins.
    pub async fn list_plugins(
        &self,
        request: crate::proto::mavio::system::v1::ListPluginsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListPluginsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_plugins_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListPlugins RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_plugins_with_options(
        &self,
        request: crate::proto::mavio::system::v1::ListPluginsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListPluginsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_LIST_PLUGINS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetPluginConfig RPC. Sends a request to /mavio.system.v1.SystemService/GetPluginConfig.
    pub async fn get_plugin_config(
        &self,
        request: crate::proto::mavio::system::v1::GetPluginConfigRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetPluginConfigResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_plugin_config_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetPluginConfig RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_plugin_config_with_options(
        &self,
        request: crate::proto::mavio::system::v1::GetPluginConfigRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetPluginConfigResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_GET_PLUGIN_CONFIG_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the SetPluginConfig RPC. Sends a request to /mavio.system.v1.SystemService/SetPluginConfig.
    pub async fn set_plugin_config(
        &self,
        request: crate::proto::mavio::system::v1::SetPluginConfigRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::SetPluginConfigResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.set_plugin_config_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the SetPluginConfig RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn set_plugin_config_with_options(
        &self,
        request: crate::proto::mavio::system::v1::SetPluginConfigRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::SetPluginConfigResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_SET_PLUGIN_CONFIG_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListCatalogPlugins RPC. Sends a request to /mavio.system.v1.SystemService/ListCatalogPlugins.
    pub async fn list_catalog_plugins(
        &self,
        request: crate::proto::mavio::system::v1::ListCatalogPluginsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListCatalogPluginsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_catalog_plugins_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListCatalogPlugins RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_catalog_plugins_with_options(
        &self,
        request: crate::proto::mavio::system::v1::ListCatalogPluginsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListCatalogPluginsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_LIST_CATALOG_PLUGINS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the InstallPlugin RPC. Sends a request to /mavio.system.v1.SystemService/InstallPlugin.
    pub async fn install_plugin(
        &self,
        request: crate::proto::mavio::system::v1::InstallPluginRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::InstallPluginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.install_plugin_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the InstallPlugin RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn install_plugin_with_options(
        &self,
        request: crate::proto::mavio::system::v1::InstallPluginRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::InstallPluginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_INSTALL_PLUGIN_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the UninstallPlugin RPC. Sends a request to /mavio.system.v1.SystemService/UninstallPlugin.
    pub async fn uninstall_plugin(
        &self,
        request: crate::proto::mavio::system::v1::UninstallPluginRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::UninstallPluginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.uninstall_plugin_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the UninstallPlugin RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn uninstall_plugin_with_options(
        &self,
        request: crate::proto::mavio::system::v1::UninstallPluginRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::UninstallPluginResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_UNINSTALL_PLUGIN_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the GetServerSettings RPC. Sends a request to /mavio.system.v1.SystemService/GetServerSettings.
    pub async fn get_server_settings(
        &self,
        request: crate::proto::mavio::system::v1::GetServerSettingsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetServerSettingsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.get_server_settings_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the GetServerSettings RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn get_server_settings_with_options(
        &self,
        request: crate::proto::mavio::system::v1::GetServerSettingsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::GetServerSettingsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_GET_SERVER_SETTINGS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the UpdateServerSettings RPC. Sends a request to /mavio.system.v1.SystemService/UpdateServerSettings.
    pub async fn update_server_settings(
        &self,
        request: crate::proto::mavio::system::v1::UpdateServerSettingsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::UpdateServerSettingsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.update_server_settings_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the UpdateServerSettings RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn update_server_settings_with_options(
        &self,
        request: crate::proto::mavio::system::v1::UpdateServerSettingsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::UpdateServerSettingsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_UPDATE_SERVER_SETTINGS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListDirectory RPC. Sends a request to /mavio.system.v1.SystemService/ListDirectory.
    pub async fn list_directory(
        &self,
        request: crate::proto::mavio::system::v1::ListDirectoryRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListDirectoryResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_directory_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListDirectory RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_directory_with_options(
        &self,
        request: crate::proto::mavio::system::v1::ListDirectoryRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListDirectoryResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_LIST_DIRECTORY_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
    /// Call the ListLogs RPC. Sends a request to /mavio.system.v1.SystemService/ListLogs.
    pub async fn list_logs(
        &self,
        request: crate::proto::mavio::system::v1::ListLogsRequest,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListLogsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        self.list_logs_with_options(
                request,
                ::connectrpc::client::CallOptions::default(),
            )
            .await
    }
    /// Call the ListLogs RPC with explicit per-call options. Options override [`ClientConfig`](::connectrpc::client::ClientConfig) defaults.
    pub async fn list_logs_with_options(
        &self,
        request: crate::proto::mavio::system::v1::ListLogsRequest,
        options: ::connectrpc::client::CallOptions,
    ) -> Result<
        ::connectrpc::client::UnaryResponse<
            ::buffa::view::OwnedView<
                crate::proto::mavio::system::v1::__buffa::view::ListLogsResponseView<
                    'static,
                >,
            >,
        >,
        ::connectrpc::ConnectError,
    > {
        ::connectrpc::client::call_unary(
                &self.transport,
                &self.config,
                SYSTEM_SERVICE_LIST_LOGS_SPEC
                    .with_origin(::connectrpc::SpecOrigin::Client),
                request,
                options,
            )
            .await
    }
}

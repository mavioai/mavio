from buf.validate import validate_pb2 as _validate_pb2
from mavio.acquisition.v1 import discover_pb2 as _discover_pb2
from mavio.library.v1 import item_pb2 as _item_pb2
from mavio.library.v1 import library_pb2 as _library_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListHubsRequest(_message.Message):
    __slots__ = ("library_id", "item_limit")
    LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_LIMIT_FIELD_NUMBER: _ClassVar[int]
    library_id: str
    item_limit: int
    def __init__(self, library_id: _Optional[str] = ..., item_limit: _Optional[int] = ...) -> None: ...

class ListHubsResponse(_message.Message):
    __slots__ = ("hubs",)
    HUBS_FIELD_NUMBER: _ClassVar[int]
    hubs: _containers.RepeatedCompositeFieldContainer[Hub]
    def __init__(self, hubs: _Optional[_Iterable[_Union[Hub, _Mapping]]] = ...) -> None: ...

class GetHubRequest(_message.Message):
    __slots__ = ("id", "limit", "offset")
    ID_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    id: str
    limit: int
    offset: int
    def __init__(self, id: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class GetHubResponse(_message.Message):
    __slots__ = ("hub",)
    HUB_FIELD_NUMBER: _ClassVar[int]
    hub: Hub
    def __init__(self, hub: _Optional[_Union[Hub, _Mapping]] = ...) -> None: ...

class Hub(_message.Message):
    __slots__ = ("id", "kind", "library_id", "library_name", "seed_id", "seed_name", "items", "titles", "more")
    ID_FIELD_NUMBER: _ClassVar[int]
    KIND_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_ID_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_NAME_FIELD_NUMBER: _ClassVar[int]
    SEED_ID_FIELD_NUMBER: _ClassVar[int]
    SEED_NAME_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    TITLES_FIELD_NUMBER: _ClassVar[int]
    MORE_FIELD_NUMBER: _ClassVar[int]
    id: str
    kind: _library_pb2.HubKind
    library_id: str
    library_name: str
    seed_id: str
    seed_name: str
    items: _containers.RepeatedCompositeFieldContainer[_item_pb2.Item]
    titles: _containers.RepeatedCompositeFieldContainer[_discover_pb2.DiscoveredTitle]
    more: bool
    def __init__(self, id: _Optional[str] = ..., kind: _Optional[_Union[_library_pb2.HubKind, str]] = ..., library_id: _Optional[str] = ..., library_name: _Optional[str] = ..., seed_id: _Optional[str] = ..., seed_name: _Optional[str] = ..., items: _Optional[_Iterable[_Union[_item_pb2.Item, _Mapping]]] = ..., titles: _Optional[_Iterable[_Union[_discover_pb2.DiscoveredTitle, _Mapping]]] = ..., more: _Optional[bool] = ...) -> None: ...

class HomeLayout(_message.Message):
    __slots__ = ("pinned", "library_ids", "hidden_hub_ids")
    PINNED_FIELD_NUMBER: _ClassVar[int]
    LIBRARY_IDS_FIELD_NUMBER: _ClassVar[int]
    HIDDEN_HUB_IDS_FIELD_NUMBER: _ClassVar[int]
    pinned: bool
    library_ids: _containers.RepeatedScalarFieldContainer[str]
    hidden_hub_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, pinned: _Optional[bool] = ..., library_ids: _Optional[_Iterable[str]] = ..., hidden_hub_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class GetHomeLayoutRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class GetHomeLayoutResponse(_message.Message):
    __slots__ = ("layout",)
    LAYOUT_FIELD_NUMBER: _ClassVar[int]
    layout: HomeLayout
    def __init__(self, layout: _Optional[_Union[HomeLayout, _Mapping]] = ...) -> None: ...

class UpdateHomeLayoutRequest(_message.Message):
    __slots__ = ("layout",)
    LAYOUT_FIELD_NUMBER: _ClassVar[int]
    layout: HomeLayout
    def __init__(self, layout: _Optional[_Union[HomeLayout, _Mapping]] = ...) -> None: ...

class UpdateHomeLayoutResponse(_message.Message):
    __slots__ = ("layout",)
    LAYOUT_FIELD_NUMBER: _ClassVar[int]
    layout: HomeLayout
    def __init__(self, layout: _Optional[_Union[HomeLayout, _Mapping]] = ...) -> None: ...

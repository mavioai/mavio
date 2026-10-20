from buf.validate import validate_pb2 as _validate_pb2
from mavio.library.v1 import item_pb2 as _item_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class PlaylistEntry(_message.Message):
    __slots__ = ("id", "item")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_FIELD_NUMBER: _ClassVar[int]
    id: str
    item: _item_pb2.Item
    def __init__(self, id: _Optional[str] = ..., item: _Optional[_Union[_item_pb2.Item, _Mapping]] = ...) -> None: ...

class ListPlaylistsRequest(_message.Message):
    __slots__ = ("limit", "offset")
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    limit: int
    offset: int
    def __init__(self, limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListPlaylistsResponse(_message.Message):
    __slots__ = ("playlists", "total")
    PLAYLISTS_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    playlists: _containers.RepeatedCompositeFieldContainer[_item_pb2.Item]
    total: int
    def __init__(self, playlists: _Optional[_Iterable[_Union[_item_pb2.Item, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class ListPlaylistEntriesRequest(_message.Message):
    __slots__ = ("id", "limit", "offset")
    ID_FIELD_NUMBER: _ClassVar[int]
    LIMIT_FIELD_NUMBER: _ClassVar[int]
    OFFSET_FIELD_NUMBER: _ClassVar[int]
    id: str
    limit: int
    offset: int
    def __init__(self, id: _Optional[str] = ..., limit: _Optional[int] = ..., offset: _Optional[int] = ...) -> None: ...

class ListPlaylistEntriesResponse(_message.Message):
    __slots__ = ("entries", "total")
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    TOTAL_FIELD_NUMBER: _ClassVar[int]
    entries: _containers.RepeatedCompositeFieldContainer[PlaylistEntry]
    total: int
    def __init__(self, entries: _Optional[_Iterable[_Union[PlaylistEntry, _Mapping]]] = ..., total: _Optional[int] = ...) -> None: ...

class CreatePlaylistRequest(_message.Message):
    __slots__ = ("name", "item_ids")
    NAME_FIELD_NUMBER: _ClassVar[int]
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    name: str
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, name: _Optional[str] = ..., item_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class CreatePlaylistResponse(_message.Message):
    __slots__ = ("playlist",)
    PLAYLIST_FIELD_NUMBER: _ClassVar[int]
    playlist: _item_pb2.Item
    def __init__(self, playlist: _Optional[_Union[_item_pb2.Item, _Mapping]] = ...) -> None: ...

class UpdatePlaylistRequest(_message.Message):
    __slots__ = ("id", "name")
    ID_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    id: str
    name: str
    def __init__(self, id: _Optional[str] = ..., name: _Optional[str] = ...) -> None: ...

class UpdatePlaylistResponse(_message.Message):
    __slots__ = ("playlist",)
    PLAYLIST_FIELD_NUMBER: _ClassVar[int]
    playlist: _item_pb2.Item
    def __init__(self, playlist: _Optional[_Union[_item_pb2.Item, _Mapping]] = ...) -> None: ...

class DeletePlaylistRequest(_message.Message):
    __slots__ = ("id",)
    ID_FIELD_NUMBER: _ClassVar[int]
    id: str
    def __init__(self, id: _Optional[str] = ...) -> None: ...

class DeletePlaylistResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class AddToPlaylistRequest(_message.Message):
    __slots__ = ("id", "item_ids", "position")
    ID_FIELD_NUMBER: _ClassVar[int]
    ITEM_IDS_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    id: str
    item_ids: _containers.RepeatedScalarFieldContainer[str]
    position: int
    def __init__(self, id: _Optional[str] = ..., item_ids: _Optional[_Iterable[str]] = ..., position: _Optional[int] = ...) -> None: ...

class AddToPlaylistResponse(_message.Message):
    __slots__ = ("entry_ids",)
    ENTRY_IDS_FIELD_NUMBER: _ClassVar[int]
    entry_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, entry_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class RemoveFromPlaylistRequest(_message.Message):
    __slots__ = ("id", "entry_ids")
    ID_FIELD_NUMBER: _ClassVar[int]
    ENTRY_IDS_FIELD_NUMBER: _ClassVar[int]
    id: str
    entry_ids: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, id: _Optional[str] = ..., entry_ids: _Optional[_Iterable[str]] = ...) -> None: ...

class RemoveFromPlaylistResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class MovePlaylistEntryRequest(_message.Message):
    __slots__ = ("id", "entry_id", "position")
    ID_FIELD_NUMBER: _ClassVar[int]
    ENTRY_ID_FIELD_NUMBER: _ClassVar[int]
    POSITION_FIELD_NUMBER: _ClassVar[int]
    id: str
    entry_id: str
    position: int
    def __init__(self, id: _Optional[str] = ..., entry_id: _Optional[str] = ..., position: _Optional[int] = ...) -> None: ...

class MovePlaylistEntryResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

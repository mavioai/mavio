from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class LibraryKind(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    LIBRARY_KIND_UNSPECIFIED: _ClassVar[LibraryKind]
    LIBRARY_KIND_MOVIES: _ClassVar[LibraryKind]
    LIBRARY_KIND_SHOWS: _ClassVar[LibraryKind]
    LIBRARY_KIND_MUSIC: _ClassVar[LibraryKind]
    LIBRARY_KIND_MUSIC_VIDEOS: _ClassVar[LibraryKind]
    LIBRARY_KIND_HOME_VIDEOS: _ClassVar[LibraryKind]
    LIBRARY_KIND_BOOKS: _ClassVar[LibraryKind]
    LIBRARY_KIND_PHOTOS: _ClassVar[LibraryKind]
    LIBRARY_KIND_MIXED: _ClassVar[LibraryKind]

class Container(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    CONTAINER_UNSPECIFIED: _ClassVar[Container]
    CONTAINER_FOLDER: _ClassVar[Container]
    CONTAINER_SERIES: _ClassVar[Container]
    CONTAINER_SEASON: _ClassVar[Container]
    CONTAINER_MUSIC_ARTIST: _ClassVar[Container]
LIBRARY_KIND_UNSPECIFIED: LibraryKind
LIBRARY_KIND_MOVIES: LibraryKind
LIBRARY_KIND_SHOWS: LibraryKind
LIBRARY_KIND_MUSIC: LibraryKind
LIBRARY_KIND_MUSIC_VIDEOS: LibraryKind
LIBRARY_KIND_HOME_VIDEOS: LibraryKind
LIBRARY_KIND_BOOKS: LibraryKind
LIBRARY_KIND_PHOTOS: LibraryKind
LIBRARY_KIND_MIXED: LibraryKind
CONTAINER_UNSPECIFIED: Container
CONTAINER_FOLDER: Container
CONTAINER_SERIES: Container
CONTAINER_SEASON: Container
CONTAINER_MUSIC_ARTIST: Container

class Folder(_message.Message):
    __slots__ = ("library_kind", "root", "path", "parent", "season_number", "entries")
    LIBRARY_KIND_FIELD_NUMBER: _ClassVar[int]
    ROOT_FIELD_NUMBER: _ClassVar[int]
    PATH_FIELD_NUMBER: _ClassVar[int]
    PARENT_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    ENTRIES_FIELD_NUMBER: _ClassVar[int]
    library_kind: LibraryKind
    root: str
    path: str
    parent: Container
    season_number: int
    entries: _containers.RepeatedCompositeFieldContainer[FolderEntry]
    def __init__(self, library_kind: _Optional[_Union[LibraryKind, str]] = ..., root: _Optional[str] = ..., path: _Optional[str] = ..., parent: _Optional[_Union[Container, str]] = ..., season_number: _Optional[int] = ..., entries: _Optional[_Iterable[_Union[FolderEntry, _Mapping]]] = ...) -> None: ...

class FolderEntry(_message.Message):
    __slots__ = ("name", "is_dir")
    NAME_FIELD_NUMBER: _ClassVar[int]
    IS_DIR_FIELD_NUMBER: _ClassVar[int]
    name: str
    is_dir: bool
    def __init__(self, name: _Optional[str] = ..., is_dir: _Optional[bool] = ...) -> None: ...

class IgnoreRequest(_message.Message):
    __slots__ = ("folder",)
    FOLDER_FIELD_NUMBER: _ClassVar[int]
    folder: Folder
    def __init__(self, folder: _Optional[_Union[Folder, _Mapping]] = ...) -> None: ...

class IgnoreResponse(_message.Message):
    __slots__ = ("names",)
    NAMES_FIELD_NUMBER: _ClassVar[int]
    names: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, names: _Optional[_Iterable[str]] = ...) -> None: ...

class ResolveRequest(_message.Message):
    __slots__ = ("folder",)
    FOLDER_FIELD_NUMBER: _ClassVar[int]
    folder: Folder
    def __init__(self, folder: _Optional[_Union[Folder, _Mapping]] = ...) -> None: ...

class ResolvedItem(_message.Message):
    __slots__ = ("kind", "entry", "name", "year", "parent_index", "index", "external_ids", "parts")
    class ExternalIdsEntry(_message.Message):
        __slots__ = ("key", "value")
        KEY_FIELD_NUMBER: _ClassVar[int]
        VALUE_FIELD_NUMBER: _ClassVar[int]
        key: str
        value: str
        def __init__(self, key: _Optional[str] = ..., value: _Optional[str] = ...) -> None: ...
    KIND_FIELD_NUMBER: _ClassVar[int]
    ENTRY_FIELD_NUMBER: _ClassVar[int]
    NAME_FIELD_NUMBER: _ClassVar[int]
    YEAR_FIELD_NUMBER: _ClassVar[int]
    PARENT_INDEX_FIELD_NUMBER: _ClassVar[int]
    INDEX_FIELD_NUMBER: _ClassVar[int]
    EXTERNAL_IDS_FIELD_NUMBER: _ClassVar[int]
    PARTS_FIELD_NUMBER: _ClassVar[int]
    kind: _metadata_pb2.MediaKind
    entry: str
    name: str
    year: int
    parent_index: int
    index: int
    external_ids: _containers.ScalarMap[str, str]
    parts: _containers.RepeatedScalarFieldContainer[str]
    def __init__(self, kind: _Optional[_Union[_metadata_pb2.MediaKind, str]] = ..., entry: _Optional[str] = ..., name: _Optional[str] = ..., year: _Optional[int] = ..., parent_index: _Optional[int] = ..., index: _Optional[int] = ..., external_ids: _Optional[_Mapping[str, str]] = ..., parts: _Optional[_Iterable[str]] = ...) -> None: ...

class Subfolder(_message.Message):
    __slots__ = ("name", "parent", "season_number")
    NAME_FIELD_NUMBER: _ClassVar[int]
    PARENT_FIELD_NUMBER: _ClassVar[int]
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    name: str
    parent: Container
    season_number: int
    def __init__(self, name: _Optional[str] = ..., parent: _Optional[_Union[Container, str]] = ..., season_number: _Optional[int] = ...) -> None: ...

class ResolveResponse(_message.Message):
    __slots__ = ("claimed", "folder_item", "items", "subfolders")
    CLAIMED_FIELD_NUMBER: _ClassVar[int]
    FOLDER_ITEM_FIELD_NUMBER: _ClassVar[int]
    ITEMS_FIELD_NUMBER: _ClassVar[int]
    SUBFOLDERS_FIELD_NUMBER: _ClassVar[int]
    claimed: bool
    folder_item: ResolvedItem
    items: _containers.RepeatedCompositeFieldContainer[ResolvedItem]
    subfolders: _containers.RepeatedCompositeFieldContainer[Subfolder]
    def __init__(self, claimed: _Optional[bool] = ..., folder_item: _Optional[_Union[ResolvedItem, _Mapping]] = ..., items: _Optional[_Iterable[_Union[ResolvedItem, _Mapping]]] = ..., subfolders: _Optional[_Iterable[_Union[Subfolder, _Mapping]]] = ...) -> None: ...

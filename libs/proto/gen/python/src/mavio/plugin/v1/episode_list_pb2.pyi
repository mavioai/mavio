from mavio.plugin.v1 import metadata_pb2 as _metadata_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class ListEpisodesRequest(_message.Message):
    __slots__ = ("lookup",)
    LOOKUP_FIELD_NUMBER: _ClassVar[int]
    lookup: _metadata_pb2.Lookup
    def __init__(self, lookup: _Optional[_Union[_metadata_pb2.Lookup, _Mapping]] = ...) -> None: ...

class ListEpisodesResponse(_message.Message):
    __slots__ = ("found", "provider", "seasons", "episodes")
    FOUND_FIELD_NUMBER: _ClassVar[int]
    PROVIDER_FIELD_NUMBER: _ClassVar[int]
    SEASONS_FIELD_NUMBER: _ClassVar[int]
    EPISODES_FIELD_NUMBER: _ClassVar[int]
    found: bool
    provider: str
    seasons: _containers.RepeatedCompositeFieldContainer[ListedSeason]
    episodes: _containers.RepeatedCompositeFieldContainer[ListedEpisode]
    def __init__(self, found: _Optional[bool] = ..., provider: _Optional[str] = ..., seasons: _Optional[_Iterable[_Union[ListedSeason, _Mapping]]] = ..., episodes: _Optional[_Iterable[_Union[ListedEpisode, _Mapping]]] = ...) -> None: ...

class ListedSeason(_message.Message):
    __slots__ = ("season_number", "metadata")
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    season_number: int
    metadata: _metadata_pb2.Metadata
    def __init__(self, season_number: _Optional[int] = ..., metadata: _Optional[_Union[_metadata_pb2.Metadata, _Mapping]] = ...) -> None: ...

class ListedEpisode(_message.Message):
    __slots__ = ("season_number", "episode_number", "absolute_number", "metadata")
    SEASON_NUMBER_FIELD_NUMBER: _ClassVar[int]
    EPISODE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    ABSOLUTE_NUMBER_FIELD_NUMBER: _ClassVar[int]
    METADATA_FIELD_NUMBER: _ClassVar[int]
    season_number: int
    episode_number: int
    absolute_number: int
    metadata: _metadata_pb2.Metadata
    def __init__(self, season_number: _Optional[int] = ..., episode_number: _Optional[int] = ..., absolute_number: _Optional[int] = ..., metadata: _Optional[_Union[_metadata_pb2.Metadata, _Mapping]] = ...) -> None: ...

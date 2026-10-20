import datetime

from buf.validate import validate_pb2 as _validate_pb2
from google.protobuf import timestamp_pb2 as _timestamp_pb2
from google.protobuf.internal import containers as _containers
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class Backup(_message.Message):
    __slots__ = ("name", "size_bytes", "create_time", "version")
    NAME_FIELD_NUMBER: _ClassVar[int]
    SIZE_BYTES_FIELD_NUMBER: _ClassVar[int]
    CREATE_TIME_FIELD_NUMBER: _ClassVar[int]
    VERSION_FIELD_NUMBER: _ClassVar[int]
    name: str
    size_bytes: int
    create_time: _timestamp_pb2.Timestamp
    version: str
    def __init__(self, name: _Optional[str] = ..., size_bytes: _Optional[int] = ..., create_time: _Optional[_Union[datetime.datetime, _timestamp_pb2.Timestamp, _Mapping]] = ..., version: _Optional[str] = ...) -> None: ...

class CreateBackupRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class CreateBackupResponse(_message.Message):
    __slots__ = ("backup",)
    BACKUP_FIELD_NUMBER: _ClassVar[int]
    backup: Backup
    def __init__(self, backup: _Optional[_Union[Backup, _Mapping]] = ...) -> None: ...

class ListBackupsRequest(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

class ListBackupsResponse(_message.Message):
    __slots__ = ("backups",)
    BACKUPS_FIELD_NUMBER: _ClassVar[int]
    backups: _containers.RepeatedCompositeFieldContainer[Backup]
    def __init__(self, backups: _Optional[_Iterable[_Union[Backup, _Mapping]]] = ...) -> None: ...

class DeleteBackupRequest(_message.Message):
    __slots__ = ("name",)
    NAME_FIELD_NUMBER: _ClassVar[int]
    name: str
    def __init__(self, name: _Optional[str] = ...) -> None: ...

class DeleteBackupResponse(_message.Message):
    __slots__ = ()
    def __init__(self) -> None: ...

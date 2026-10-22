from google.protobuf.internal import containers as _containers
from google.protobuf.internal import enum_type_wrapper as _enum_type_wrapper
from google.protobuf import descriptor as _descriptor
from google.protobuf import message as _message
from collections.abc import Iterable as _Iterable, Mapping as _Mapping
from typing import ClassVar as _ClassVar, Optional as _Optional, Union as _Union

DESCRIPTOR: _descriptor.FileDescriptor

class SettingFieldType(int, metaclass=_enum_type_wrapper.EnumTypeWrapper):
    __slots__ = ()
    SETTING_FIELD_TYPE_UNSPECIFIED: _ClassVar[SettingFieldType]
    SETTING_FIELD_TYPE_TEXT: _ClassVar[SettingFieldType]
    SETTING_FIELD_TYPE_NUMBER: _ClassVar[SettingFieldType]
    SETTING_FIELD_TYPE_CHECKBOX: _ClassVar[SettingFieldType]
    SETTING_FIELD_TYPE_SELECT: _ClassVar[SettingFieldType]
    SETTING_FIELD_TYPE_MULTI_SELECT: _ClassVar[SettingFieldType]
    SETTING_FIELD_TYPE_INFO: _ClassVar[SettingFieldType]
    SETTING_FIELD_TYPE_TAG: _ClassVar[SettingFieldType]
SETTING_FIELD_TYPE_UNSPECIFIED: SettingFieldType
SETTING_FIELD_TYPE_TEXT: SettingFieldType
SETTING_FIELD_TYPE_NUMBER: SettingFieldType
SETTING_FIELD_TYPE_CHECKBOX: SettingFieldType
SETTING_FIELD_TYPE_SELECT: SettingFieldType
SETTING_FIELD_TYPE_MULTI_SELECT: SettingFieldType
SETTING_FIELD_TYPE_INFO: SettingFieldType
SETTING_FIELD_TYPE_TAG: SettingFieldType

class SettingField(_message.Message):
    __slots__ = ("name", "label", "help_text", "type", "default_json", "options", "advanced", "secret")
    NAME_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    HELP_TEXT_FIELD_NUMBER: _ClassVar[int]
    TYPE_FIELD_NUMBER: _ClassVar[int]
    DEFAULT_JSON_FIELD_NUMBER: _ClassVar[int]
    OPTIONS_FIELD_NUMBER: _ClassVar[int]
    ADVANCED_FIELD_NUMBER: _ClassVar[int]
    SECRET_FIELD_NUMBER: _ClassVar[int]
    name: str
    label: str
    help_text: str
    type: SettingFieldType
    default_json: str
    options: _containers.RepeatedCompositeFieldContainer[SettingFieldOption]
    advanced: bool
    secret: bool
    def __init__(self, name: _Optional[str] = ..., label: _Optional[str] = ..., help_text: _Optional[str] = ..., type: _Optional[_Union[SettingFieldType, str]] = ..., default_json: _Optional[str] = ..., options: _Optional[_Iterable[_Union[SettingFieldOption, _Mapping]]] = ..., advanced: _Optional[bool] = ..., secret: _Optional[bool] = ...) -> None: ...

class SettingFieldOption(_message.Message):
    __slots__ = ("value_json", "label")
    VALUE_JSON_FIELD_NUMBER: _ClassVar[int]
    LABEL_FIELD_NUMBER: _ClassVar[int]
    value_json: str
    label: str
    def __init__(self, value_json: _Optional[str] = ..., label: _Optional[str] = ...) -> None: ...

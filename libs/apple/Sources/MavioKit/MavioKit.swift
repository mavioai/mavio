/// The shared core's Swift API with the app services above it: accounts,
/// capability probing, downloads and Now Playing; and the messages of the
/// server's API, for the screens the core does not model.
@_exported import MavioCore
@_exported import MavioProto
@_exported import SwiftProtobuf

/// The version of the shared core.
public var coreVersion: String { MavioCore.coreVersion() }

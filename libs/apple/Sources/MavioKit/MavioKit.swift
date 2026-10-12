/// The shared core's Swift API with the app services above it: accounts,
/// capability probing, downloads and Now Playing.
@_exported import MavioCore

/// The version of the shared core.
public var coreVersion: String { MavioCore.coreVersion() }

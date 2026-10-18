#!/bin/sh
# Generates the Swift messages of libs/proto into gen/MavioProto, which the
# package's MavioProto target compiles and the apps send through the core
# (docs/clients.md §4.2). protoc-gen-swift is built from the swift-protobuf
# the package depends on, so that the messages match its runtime.
# Usage: scripts/gen-proto.sh
set -eu
cd "$(dirname "$0")/.."
apple=$(pwd)
# The target's directory, which the package needs to load.
mkdir -p gen/MavioProto
swift build -c release --product protoc-gen-swift
plugin=$(swift build -c release --show-bin-path)/protoc-gen-swift
find gen/MavioProto -name "*.pb.swift" -delete
cd ../proto
buf generate --template "{\"version\":\"v2\",\"plugins\":[{\"local\":\"$plugin\",\"out\":\"$apple/gen/MavioProto\",\"opt\":[\"Visibility=Public\",\"FileNaming=PathToUnderscores\"]}]}" --path mavio

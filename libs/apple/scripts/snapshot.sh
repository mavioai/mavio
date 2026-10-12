#!/bin/sh
# Runs the snapshot tests of the iPhone and iPad (docs/clients.md §6.8) on
# an iOS simulator, the first iPhone xcrun lists. With --record, records
# the references in snapshots/ instead of comparing with them; with --boot,
# boots the simulator and returns, so that it is up by the time the tests
# run.
set -eu
cd "$(dirname "$0")/.."

device=$(xcrun simctl list devices available -j | python3 -c '
import json, sys
devices = json.load(sys.stdin)["devices"]
print(next(d["udid"] for r, ds in sorted(devices.items()) if ".iOS-" in r for d in ds if d["name"].startswith("iPhone")))')
case "${1:-}" in
--record) export TEST_RUNNER_MAVIO_SNAPSHOT_RECORD=1 ;;
--boot)
    xcrun simctl boot "$device"
    exit
    ;;
esac

xcodebuild test -quiet -scheme MavioApple-Package -destination "id=$device" \
    -derivedDataPath build/snapshots/derived -only-testing:MavioUITests \
    -testLanguage en -testRegion US

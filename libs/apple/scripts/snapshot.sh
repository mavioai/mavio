#!/bin/sh
# Runs the snapshot tests of the iPhone and iPad (docs/clients.md §6.8) on
# an iOS simulator, the first iPhone xcrun lists. With --record, records
# the references in snapshots/ instead of comparing with them.
set -eu
cd "$(dirname "$0")/.."

if [ "${1:-}" = --record ]; then
    export TEST_RUNNER_MAVIO_SNAPSHOT_RECORD=1
fi
device=$(xcrun simctl list devices available -j | python3 -c '
import json, sys
devices = json.load(sys.stdin)["devices"]
print(next(d["udid"] for r, ds in sorted(devices.items()) if ".iOS-" in r for d in ds if d["name"].startswith("iPhone")))')

# The simulator boots before the tests: a test runner launched on one still
# booting can hang, and a failure then waits on the simulator's diagnostics,
# which the tests do not need.
xcrun simctl bootstatus "$device" -b >/dev/null
xcodebuild test -quiet -scheme MavioApple-Package -destination "id=$device" \
    -derivedDataPath build/snapshots/derived -only-testing:MavioUITests \
    -testLanguage en -testRegion US -collect-test-diagnostics never

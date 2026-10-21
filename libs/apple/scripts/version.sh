#!/bin/sh
# Sets the version of an Apple app's or extension's built Info.plist as the
# server's and the core's (docs/development.md §7): a release's from
# $MAVIO_VERSION (v0.1.0 or 0.1.0), else 0.0.0 and the commit it is built at,
# else dev. CFBundleShortVersionString is the release's x.y.z, else 0.0.0, as
# the App Store wants it, and MavioVersion the whole version; CFBundleVersion
# is a release's x.y.z as x*10000+y*100+z, else the number of commits.
#
# Usage: version.sh <Info.plist>, from the projects' Version build phase.
set -eu

plist="$1"
release="${MAVIO_VERSION:-}"
release="${release#v}"
if [ -n "$release" ]; then
	if ! printf '%s\n' "$release" | grep -Eq '^[0-9]+\.[0-9]+\.[0-9]+$'; then
		echo "error: MAVIO_VERSION $release is not x.y.z" >&2
		exit 1
	fi
	IFS=. read -r x y z <<EOF
$release
EOF
	if [ "$y" -ge 100 ] || [ "$z" -ge 100 ]; then
		echo "error: MAVIO_VERSION $release has a part over 99" >&2
		exit 1
	fi
	full="$release"
	short="$release"
	build=$((x * 10000 + y * 100 + z))
elif commit=$(git -C "$(dirname "$0")" rev-parse --short=12 HEAD 2>/dev/null); then
	full="0.0.0-$commit"
	short="0.0.0"
	build=$(git -C "$(dirname "$0")" rev-list --count HEAD 2>/dev/null || echo 1)
else
	full="dev"
	short="0.0.0"
	build=1
fi

buddy=/usr/libexec/PlistBuddy
"$buddy" -c "Set :CFBundleShortVersionString $short" -c "Set :CFBundleVersion $build" "$plist"
"$buddy" -c "Delete :MavioVersion" "$plist" >/dev/null 2>&1 || true
"$buddy" -c "Add :MavioVersion string $full" "$plist"

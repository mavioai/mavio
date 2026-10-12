#!/bin/sh
# Builds the shared core (libs/client-rust, crate mavio-uniffi) for the
# Apple platforms into build/MavioCoreFFI.xcframework, a dynamic framework
# per platform, and generates its Swift API into build/MavioCore, which the
# package's MavioCore target compiles. Usage: scripts/build-core.sh
# MAVIO_CORE_PROFILE picks the Cargo profile, release by default; CI builds
# with dev, which compiles much faster. MAVIO_MACOS_ARCHS lists the
# architectures of the macOS framework, arm64 and x86_64 by default; CI
# builds arm64 only.
set -eu
cd "$(dirname "$0")/.."
apple=$(pwd)
core=$apple/../client-rust
out=$apple/build
profile=${MAVIO_CORE_PROFILE:-release}
build=$profile
[ "$profile" = dev ] && build=debug
darwin=""
for arch in ${MAVIO_MACOS_ARCHS:-arm64 x86_64}; do
	[ "$arch" = arm64 ] && arch=aarch64
	darwin="$darwin $arch-apple-darwin"
done

export IPHONEOS_DEPLOYMENT_TARGET=18.0 TVOS_DEPLOYMENT_TARGET=18.0 MACOSX_DEPLOYMENT_TARGET=15.0

cd "$core"
targets="aarch64-apple-ios aarch64-apple-ios-sim aarch64-apple-tvos aarch64-apple-tvos-sim$darwin"
# shellcheck disable=SC2086
rustup target add $targets >/dev/null
# One build, so that Cargo compiles the targets in parallel.
# shellcheck disable=SC2046,SC2086
cargo build --locked --profile "$profile" -p mavio-uniffi --lib $(printf -- ' --target %s' $targets)

# Only what this script writes; build/ holds the other libraries too.
rm -rf "$out/MavioCore" "$out/MavioCoreFFI.xcframework" "$out/gen" "$out/frameworks"
mkdir -p "$out/MavioCore" "$out/frameworks"
cargo run --locked --profile "$profile" -q --bin uniffi-bindgen -- generate \
	--library "target/aarch64-apple-ios/$build/libmavio.dylib" --language swift --out-dir "$out/gen"
mv "$out/gen/MavioCore.swift" "$out/MavioCore/"

# framework <dir> <library> <min-os> <platform>...: wraps a library into
# <dir>/MavioCoreFFI.framework; macOS frameworks are versioned.
framework() {
	dir=$1 lib=$2 minos=$3
	shift 3
	fw=$dir/MavioCoreFFI.framework
	if [ "$1" = MacOSX ]; then
		root=$fw/Versions/A res=$fw/Versions/A/Resources
		id=@rpath/MavioCoreFFI.framework/Versions/A/MavioCoreFFI minkey=LSMinimumSystemVersion
	else
		root=$fw res=$fw
		id=@rpath/MavioCoreFFI.framework/MavioCoreFFI minkey=MinimumOSVersion
	fi
	mkdir -p "$root/Headers" "$root/Modules" "$res"
	cp "$lib" "$root/MavioCoreFFI"
	install_name_tool -id "$id" "$root/MavioCoreFFI"
	cp "$out/gen/MavioCoreFFI.h" "$root/Headers/"
	printf 'framework module MavioCoreFFI {\n    umbrella header "MavioCoreFFI.h"\n    export *\n}\n' \
		>"$root/Modules/module.modulemap"
	platforms=""
	for p in "$@"; do platforms="$platforms<string>$p</string>"; done
	cat >"$res/Info.plist" <<PLIST
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>CFBundleExecutable</key><string>MavioCoreFFI</string>
	<key>CFBundleIdentifier</key><string>dev.mavio.core</string>
	<key>CFBundleInfoDictionaryVersion</key><string>6.0</string>
	<key>CFBundleName</key><string>MavioCoreFFI</string>
	<key>CFBundlePackageType</key><string>FMWK</string>
	<key>CFBundleShortVersionString</key><string>0.0.0</string>
	<key>CFBundleVersion</key><string>1</string>
	<key>CFBundleSupportedPlatforms</key><array>$platforms</array>
	<key>$minkey</key><string>$minos</string>
</dict>
</plist>
PLIST
	if [ "$1" = MacOSX ]; then
		ln -s A "$fw/Versions/Current"
		for f in MavioCoreFFI Headers Modules Resources; do ln -s "Versions/Current/$f" "$fw/$f"; done
	fi
}

# shellcheck disable=SC2046,SC2086
lipo -create $(printf "target/%s/$build/libmavio.dylib " $darwin) -output "$out/gen/libmavio-macos.dylib"
framework "$out/frameworks/ios" "target/aarch64-apple-ios/$build/libmavio.dylib" 18.0 iPhoneOS
framework "$out/frameworks/ios-sim" "target/aarch64-apple-ios-sim/$build/libmavio.dylib" 18.0 iPhoneSimulator
framework "$out/frameworks/tvos" "target/aarch64-apple-tvos/$build/libmavio.dylib" 18.0 AppleTVOS
framework "$out/frameworks/tvos-sim" "target/aarch64-apple-tvos-sim/$build/libmavio.dylib" 18.0 AppleTVSimulator
framework "$out/frameworks/macos" "$out/gen/libmavio-macos.dylib" 15.0 MacOSX
args=""
for p in ios ios-sim tvos tvos-sim macos; do
	args="$args -framework $out/frameworks/$p/MavioCoreFFI.framework"
done
# shellcheck disable=SC2086
xcodebuild -create-xcframework $args -output "$out/MavioCoreFFI.xcframework" >/dev/null
rm -rf "$out/gen" "$out/frameworks"
echo "$out/MavioCoreFFI.xcframework"

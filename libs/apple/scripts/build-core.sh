#!/bin/sh
# Builds the shared core (libs/client-rust, crate mavio-uniffi) for the
# Apple platforms into build/MavioCoreFFI.xcframework, a dynamic framework
# per platform, and generates its Swift API into build/MavioCore, which the
# package's MavioCore target compiles. Usage: scripts/build-core.sh
# MAVIO_CORE_PROFILE picks the Cargo profile, release by default; CI builds
# with dev, which compiles much faster. MAVIO_MACOS_ARCHS lists the
# architectures of the macOS framework, arm64 and x86_64 by default; CI
# builds arm64 only. MAVIO_APPLE_PLATFORMS lists the platforms of the
# XCFramework, of ios, ios-sim, tvos, tvos-sim and macos, all by default; CI
# builds the simulators and macOS only, which its builds and tests run on.
set -eu
cd "$(dirname "$0")/.."
apple=$(pwd)
core=$apple/../client-rust
out=$apple/build
profile=${MAVIO_CORE_PROFILE:-release}
build=$profile
[ "$profile" = dev ] && build=debug
platforms=${MAVIO_APPLE_PLATFORMS:-ios ios-sim tvos tvos-sim macos}
darwin=""
for arch in ${MAVIO_MACOS_ARCHS:-arm64 x86_64}; do
	[ "$arch" = arm64 ] && arch=aarch64
	darwin="$darwin $arch-apple-darwin"
done
# The Rust targets of a platform.
targets_of() {
	case $1 in
	ios) echo aarch64-apple-ios ;;
	ios-sim) echo aarch64-apple-ios-sim ;;
	tvos) echo aarch64-apple-tvos ;;
	tvos-sim) echo aarch64-apple-tvos-sim ;;
	macos) echo "$darwin" ;;
	*)
		echo "unknown platform $1 in MAVIO_APPLE_PLATFORMS" >&2
		exit 1
		;;
	esac
}
targets=""
for p in $platforms; do targets="$targets $(targets_of "$p")"; done

export IPHONEOS_DEPLOYMENT_TARGET=18.0 TVOS_DEPLOYMENT_TARGET=18.0 MACOSX_DEPLOYMENT_TARGET=15.0

cd "$core"
# shellcheck disable=SC2086
rustup target add $targets >/dev/null
# One build, so that Cargo compiles the targets in parallel.
# shellcheck disable=SC2046,SC2086
cargo build --locked --profile "$profile" -p mavio-uniffi --lib $(printf -- ' --target %s' $targets)

# Only what this script writes; build/ holds the other libraries too.
rm -rf "$out/MavioCore" "$out/MavioCoreFFI.xcframework" "$out/gen" "$out/frameworks"
mkdir -p "$out/MavioCore" "$out/frameworks"
# The bindings are the same whichever target's library they come from.
# shellcheck disable=SC2086
set -- $targets
cargo run --locked --profile "$profile" -q --bin uniffi-bindgen -- generate \
	--library "target/$1/$build/libmavio.dylib" --language swift --out-dir "$out/gen"
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

args=""
for platform in $platforms; do
	lib=target/aarch64-apple-$platform/$build/libmavio.dylib
	case $platform in
	ios) framework "$out/frameworks/$platform" "$lib" 18.0 iPhoneOS ;;
	ios-sim) framework "$out/frameworks/$platform" "$lib" 18.0 iPhoneSimulator ;;
	tvos) framework "$out/frameworks/$platform" "$lib" 18.0 AppleTVOS ;;
	tvos-sim) framework "$out/frameworks/$platform" "$lib" 18.0 AppleTVSimulator ;;
	macos)
		# shellcheck disable=SC2046,SC2086
		lipo -create $(printf "target/%s/$build/libmavio.dylib " $darwin) -output "$out/gen/libmavio-macos.dylib"
		framework "$out/frameworks/$platform" "$out/gen/libmavio-macos.dylib" 15.0 MacOSX
		;;
	esac
	args="$args -framework $out/frameworks/$platform/MavioCoreFFI.framework"
done
# shellcheck disable=SC2086
xcodebuild -create-xcframework $args -output "$out/MavioCoreFFI.xcframework" >/dev/null
rm -rf "$out/gen" "$out/frameworks"
echo "$out/MavioCoreFFI.xcframework"

#!/bin/sh
# Builds the shared core (libs/client-rust, crate mavio-uniffi) for the
# Apple platforms into build/MavioCoreFFI.xcframework, a dynamic framework
# per platform, and generates its Swift API into build/MavioCore, which the
# package's MavioCore target compiles. Usage: scripts/build-core.sh
set -eu
cd "$(dirname "$0")/.."
apple=$(pwd)
core=$apple/../client-rust
out=$apple/build
# tvOS is a tier 3 target, built with the standard library on this nightly.
nightly=nightly-2026-10-10

export IPHONEOS_DEPLOYMENT_TARGET=18.0 TVOS_DEPLOYMENT_TARGET=18.0 MACOSX_DEPLOYMENT_TARGET=15.0

cd "$core"
rustup target add aarch64-apple-ios aarch64-apple-ios-sim aarch64-apple-darwin x86_64-apple-darwin >/dev/null
rustup toolchain install "$nightly" --profile minimal --component rust-src >/dev/null
for target in aarch64-apple-ios aarch64-apple-ios-sim aarch64-apple-darwin x86_64-apple-darwin; do
	cargo build --locked --release -p mavio-uniffi --lib --target "$target"
done
for target in aarch64-apple-tvos aarch64-apple-tvos-sim; do
	cargo "+$nightly" build --locked --release -p mavio-uniffi --lib --target "$target" -Zbuild-std=std,panic_abort
done

rm -rf "$out" && mkdir -p "$out/MavioCore" "$out/frameworks"
cargo run --locked --release -q --bin uniffi-bindgen -- generate \
	--library target/aarch64-apple-darwin/release/libmavio.dylib --language swift --out-dir "$out/gen"
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

lipo -create target/aarch64-apple-darwin/release/libmavio.dylib target/x86_64-apple-darwin/release/libmavio.dylib \
	-output "$out/gen/libmavio-macos.dylib"
framework "$out/frameworks/ios" target/aarch64-apple-ios/release/libmavio.dylib 18.0 iPhoneOS
framework "$out/frameworks/ios-sim" target/aarch64-apple-ios-sim/release/libmavio.dylib 18.0 iPhoneSimulator
framework "$out/frameworks/tvos" target/aarch64-apple-tvos/release/libmavio.dylib 18.0 AppleTVOS
framework "$out/frameworks/tvos-sim" target/aarch64-apple-tvos-sim/release/libmavio.dylib 18.0 AppleTVSimulator
framework "$out/frameworks/macos" "$out/gen/libmavio-macos.dylib" 15.0 MacOSX
args=""
for p in ios ios-sim tvos tvos-sim macos; do
	args="$args -framework $out/frameworks/$p/MavioCoreFFI.framework"
done
# shellcheck disable=SC2086
xcodebuild -create-xcframework $args -output "$out/MavioCoreFFI.xcframework" >/dev/null
rm -rf "$out/gen" "$out/frameworks"
echo "$out/MavioCoreFFI.xcframework"

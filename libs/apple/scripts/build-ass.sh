#!/bin/sh
# Builds libass for the Apple platforms into build/Libass.xcframework, a
# static library per platform with FreeType, FriBidi and HarfBuzz linked in
# and Core Text as its font provider, at the versions pinned below. The
# player renders AetherEngine's ASS events with it. Sources and intermediate
# builds are kept in build/libass; a run with the same pins does nothing.
# Needs meson and ninja (mise.toml). Usage: scripts/build-ass.sh
# MAVIO_MACOS_ARCHS lists the architectures of the macOS library, arm64 and
# x86_64 by default, as for the core (build-core.sh).
set -eu
cd "$(dirname "$0")/.."
apple=$(pwd)
work=$apple/build/libass
out=$apple/build/Libass.xcframework

libass=0.17.5
freetype=VER-2-14-3
fribidi=v1.0.17
harfbuzz=12.3.2
archs=${MAVIO_MACOS_ARCHS:-arm64 x86_64}

# The library is built once for each version of this script and macOS
# architectures.
stamp="$(cksum <"$apple/scripts/build-ass.sh") $archs"
if [ "$(cat "$work/stamp" 2>/dev/null)" = "$stamp" ] && [ -d "$out" ]; then
	echo "$out"
	exit 0
fi

# fetch dir repository revision
fetch() {
	[ -d "$1" ] && return
	git init -q "$1"
	git -C "$1" fetch -q --depth 1 "$2" "$3"
	git -C "$1" -c advice.detachedHead=false checkout -q FETCH_HEAD
}

# Each source in a directory named by its version.
src=$work/libass-$libass
fetch "$src" https://github.com/libass/libass "refs/tags/$libass"
# Its dependencies, as meson subprojects.
mkdir -p "$src/subprojects"
# wrap name repository revision dependency
wrap() {
	cat >"$src/subprojects/$1.wrap" <<WRAP
[wrap-git]
directory = $1-$3
url = $2
revision = $3
depth = 1

[provide]
dependency_names = $4
WRAP
}
wrap freetype2 https://gitlab.freedesktop.org/freetype/freetype "$freetype" freetype2
wrap fribidi https://github.com/fribidi/fribidi "$fribidi" fribidi
wrap harfbuzz https://github.com/harfbuzz/harfbuzz "$harfbuzz" harfbuzz

# slice name sdk arch min-os-flag
slice() {
	name=$1 sdk=$2 arch=$3 minos=$4
	sysroot=$(xcrun --sdk "$sdk" --show-sdk-path)
	cc=$(xcrun --sdk "$sdk" -f clang)
	cpu=$arch
	[ "$arch" = arm64 ] && cpu=aarch64
	flags="'-arch', '$arch', '-isysroot', '$sysroot', '$minos'"
	cat >"$work/$name.cross" <<CROSS
[binaries]
c = '$cc'
cpp = '$(xcrun --sdk "$sdk" -f clang++)'
objc = '$cc'
ar = '$(xcrun --sdk "$sdk" -f ar)'
strip = '$(xcrun --sdk "$sdk" -f strip)'
pkg-config = 'false'

[built-in options]
c_args = [$flags]
cpp_args = [$flags]
objc_args = [$flags]
c_link_args = [$flags]
cpp_link_args = [$flags]

[host_machine]
system = 'darwin'
subsystem = '$sdk'
cpu_family = '$cpu'
cpu = '$cpu'
endian = 'little'
CROSS
	build=$work/build-$name
	rm -rf "$build"
	# The x86_64 assembly would need nasm; only Intel Macs run it.
	meson setup "$build" "$src" --cross-file "$work/$name.cross" \
		--buildtype=release --default-library=static --wrap-mode=default \
		--force-fallback-for=freetype2,fribidi,harfbuzz \
		-Dfontconfig=disabled -Dcoretext=enabled -Ddirectwrite=disabled -Dlibunibreak=disabled \
		-Dasm=disabled -Dtest=disabled -Dcompare=disabled -Dprofile=disabled -Dfuzz=disabled -Dcheckasm=disabled \
		-Dfreetype2:zlib=disabled -Dfreetype2:bzip2=disabled -Dfreetype2:png=disabled \
		-Dfreetype2:brotli=disabled -Dfreetype2:harfbuzz=disabled \
		-Dharfbuzz:coretext=disabled -Dharfbuzz:icu=disabled -Dharfbuzz:utilities=disabled \
		>"$work/$name.log"
	ninja -C "$build" >>"$work/$name.log"
	# One archive with its dependencies.
	libtool -static -o "$work/$name.a" "$build/libass/libass.a" \
		$(find "$build/subprojects" -name '*.a') 2>/dev/null
}

slice ios iphoneos arm64 -miphoneos-version-min=18.0
slice ios-sim iphonesimulator arm64 -mios-simulator-version-min=18.0
slice tvos appletvos arm64 -mtvos-version-min=18.0
slice tvos-sim appletvsimulator arm64 -mtvos-simulator-version-min=18.0
macos=""
for arch in $archs; do
	slice "macos-$arch" macosx "$arch" -mmacosx-version-min=15.0
	macos="$macos $work/macos-$arch.a"
done
# shellcheck disable=SC2086
lipo -create $macos -output "$work/macos.a"

# The headers and module map in a directory of their own, apart from those
# of other libraries in the same build.
headers=$work/headers
rm -rf "$headers" && mkdir -p "$headers/Libass"
cp "$src/libass/ass.h" "$src/libass/ass_types.h" "$headers/Libass/"
printf 'module Libass {\n    header "ass.h"\n    link "c++"\n    link framework "CoreText"\n    export *\n}\n' \
	>"$headers/Libass/module.modulemap"
rm -rf "$out"
args=""
for p in ios ios-sim tvos tvos-sim macos; do
	args="$args -library $work/$p.a -headers $headers"
done
# shellcheck disable=SC2086
xcodebuild -create-xcframework $args -output "$out" >/dev/null
echo "$stamp" >"$work/stamp"
echo "$out"

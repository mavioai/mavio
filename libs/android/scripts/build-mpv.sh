#!/bin/sh
# Builds libmpv for Android as mpv-android does, with its build scripts and
# its JNI layer, at the versions pinned below, into player/build/mpv:
# jniLibs/<abi>/ (libmpv, FFmpeg, libc++ and libplayer, the JNI layer) and
# kotlin/, mpv-android's Kotlin API to the JNI layer (is.xyz.mpv.MPVLib).
# FFmpeg and mpv are built as GPL-3.0, as Mavio is licensed. Sources and
# intermediate builds are kept in build/mpv; a run with the same pins does
# nothing. Needs meson and ninja (mise.toml), autotools, pkg-config and
# gperf, and on macOS GNU sed, coreutils and bash 4 or later; nasm is built
# when missing. Usage: scripts/build-mpv.sh
set -eu
cd "$(dirname "$0")/.."
android=$(pwd)
work=$android/build/mpv
out=$android/player/build/mpv
ndk_version=30.0.16248370

mpv_android=c2e02fe111541e4063ab8f868bf629649ed937a3
mpv=88eb9f2e73945d8bc9c133a2f67babaa9af34680
ffmpeg=n9.0.2
libass=0.17.5
libplacebo=v7.360.1
dav1d=1.5.4
nasm=3.02
fontconfig=2.18.3

# The libraries are built once for each version of this script.
stamp=$(cksum <"$android/scripts/build-mpv.sh")
if [ "$(cat "$out/stamp" 2>/dev/null)" = "$stamp" ]; then
	echo "$out"
	exit 0
fi

sdk=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$HOME/Library/Android/sdk}}
ndk=$sdk/ndk/$ndk_version
if [ ! -d "$ndk" ]; then
	yes | "$sdk/cmdline-tools/latest/bin/sdkmanager" --install "ndk;$ndk_version" >/dev/null
fi

# fetch dir repository revision [--recursive]
fetch() {
	[ -d "$1" ] && return
	git init -q "$1"
	git -C "$1" fetch -q --depth 1 "$2" "$3"
	git -C "$1" -c advice.detachedHead=false checkout -q FETCH_HEAD
	if [ "${4:-}" = --recursive ]; then
		git -C "$1" submodule -q update --init --recursive --depth 1
	fi
}

mkdir -p "$work/bin"
export PATH="$work/bin:$PATH"
# mpv-android downloads with wget; curl stands in for it.
printf '#!/bin/sh\nexec curl -fsSL "$1"\n' >"$work/bin/wget"
chmod +x "$work/bin/wget"
if ! command -v nasm >/dev/null; then
	rm -rf "$work/nasm"
	fetch "$work/nasm" https://github.com/netwide-assembler/nasm "refs/tags/nasm-$nasm"
	(cd "$work/nasm" && ./autogen.sh >/dev/null && ./configure -q &&
		make -s -j"$(getconf _NPROCESSORS_ONLN)" nasm && cp nasm "$work/bin/")
fi

fetch "$work/mpv-android" https://github.com/mpv-android/mpv-android "$mpv_android"
scripts=$work/mpv-android/buildscripts
mkdir -p "$scripts/deps" "$scripts/sdk"
fetch "$scripts/deps/mpv" https://github.com/mpv-player/mpv "$mpv"
fetch "$scripts/deps/ffmpeg" https://github.com/FFmpeg/FFmpeg "refs/tags/$ffmpeg"
fetch "$scripts/deps/libass" https://github.com/libass/libass "refs/tags/$libass"
fetch "$scripts/deps/libplacebo" https://github.com/haasn/libplacebo "refs/tags/$libplacebo" --recursive
fetch "$scripts/deps/dav1d" https://github.com/videolan/dav1d "refs/tags/$dav1d"
# Its archives sit behind a browser check; git is not.
fetch "$scripts/deps/fontconfig" https://gitlab.freedesktop.org/fontconfig/fontconfig "refs/tags/$fontconfig"
ln -sfn "$ndk" "$scripts/sdk/android-ndk-r30"

cd "$scripts"
# The rest of the sources, at the versions mpv-android pins.
./include/download-deps.sh
for arch in arm64 armv7l x86_64; do
	# Its build script needs bash 4 or later, which macOS lacks in /bin.
	bash ./buildall.sh --arch "$arch" mpv
done

cd "$work/mpv-android/app/src/main"
PATH="$ndk:$PATH" PREFIX64="$scripts/prefix/arm64" PREFIX32="$scripts/prefix/armv7l" \
	PREFIX_X64="$scripts/prefix/x86_64" ndk-build -s -j"$(getconf _NPROCESSORS_ONLN)"

rm -rf "$out" && mkdir -p "$out/jniLibs" "$out/kotlin/is/xyz/mpv"
cp -R libs/. "$out/jniLibs/"
cp java/is/xyz/mpv/MPVLib.kt "$out/kotlin/is/xyz/mpv/"
echo "$stamp" >"$out/stamp"
echo "$out"

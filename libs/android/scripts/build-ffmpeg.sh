#!/bin/sh
# Builds Media3's FFmpeg decoder extension, which has no published artifact,
# into player/build/ffmpeg: jniLibs/<abi>/libffmpegJNI.so, its JNI layer
# linked with FFmpeg's audio decoders, and java/, its audio renderer, both at
# the Media3 version gradle/libs.versions.toml names. FFmpeg is built as
# LGPL, with only the decoders below. Sources and intermediate builds are
# kept in build/ffmpeg; a run with the same pins does nothing.
# Usage: scripts/build-ffmpeg.sh
set -eu
cd "$(dirname "$0")/.."
android=$(pwd)
work=$android/build/ffmpeg
out=$android/player/build/ffmpeg
ndk_version=30.0.16248370
api=26

media3=$(sed -n 's/^media3 = "\(.*\)"/\1/p' gradle/libs.versions.toml)
ffmpeg=n9.0.2
# The audio formats Media3 hands the extension (FfmpegLibrary), among them
# those Android's decoders lack: DTS, TrueHD and MLP.
decoders="aac ac3 eac3 dca truehd mlp mp3 vorbis opus flac alac pcm_mulaw pcm_alaw amrnb amrwb"

# The libraries are built once for each version of this script and Media3.
stamp="$(cksum <"$android/scripts/build-ffmpeg.sh") $media3"
if [ "$(cat "$out/stamp" 2>/dev/null)" = "$stamp" ]; then
	echo "$out"
	exit 0
fi

sdk=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$HOME/Library/Android/sdk}}
ndk=$sdk/ndk/$ndk_version
if [ ! -d "$ndk" ]; then
	yes | "$sdk/cmdline-tools/latest/bin/sdkmanager" --install "ndk;$ndk_version" >/dev/null
fi
case "$(uname -s)" in
Darwin) host=darwin-x86_64 ;;
*) host=linux-x86_64 ;;
esac
bin=$ndk/toolchains/llvm/prebuilt/$host/bin
jobs=$(getconf _NPROCESSORS_ONLN)

# fetch dir repository revision [path]
fetch() {
	[ -d "$1" ] && return
	git init -q "$1"
	if [ -n "${4:-}" ]; then
		git -C "$1" sparse-checkout set "$4"
	fi
	git -C "$1" fetch -q --depth 1 "$2" "$3"
	git -C "$1" -c advice.detachedHead=false checkout -q FETCH_HEAD
}

rm -rf "$work/media"
fetch "$work/media" https://github.com/androidx/media "refs/tags/$media3" libraries/decoder_ffmpeg
fetch "$work/ffmpeg" https://github.com/FFmpeg/FFmpeg "refs/tags/$ffmpeg"
ext=$work/media/libraries/decoder_ffmpeg/src/main

rm -rf "$out" && mkdir -p "$out/java"
for abi in arm64-v8a armeabi-v7a x86_64; do
	case $abi in
	arm64-v8a) triple=aarch64-linux-android arch=aarch64 cpu=armv8-a flags= asm= ;;
	armeabi-v7a) triple=armv7a-linux-androideabi arch=arm cpu=armv7-a flags="-mfloat-abi=softfp" asm= ;;
	# For emulators on x86 hosts, without the assembly that would need nasm.
	x86_64) triple=x86_64-linux-android arch=x86_64 cpu=x86-64 flags= asm=--disable-asm ;;
	esac
	prefix=$work/prefix/$abi
	enable=""
	for d in $decoders; do enable="$enable --enable-decoder=$d"; done
	(
		cd "$work/ffmpeg"
		make -s distclean >/dev/null 2>&1 || true
		# shellcheck disable=SC2086
		./configure --prefix="$prefix" --target-os=android --enable-cross-compile \
			--arch=$arch --cpu=$cpu --cc="$bin/$triple$api-clang" --cxx="$bin/$triple$api-clang++" \
			--nm="$bin/llvm-nm" --ar="$bin/llvm-ar" --ranlib="$bin/llvm-ranlib" --strip="$bin/llvm-strip" \
			--extra-cflags="-fPIC $flags" --enable-static --disable-shared --disable-doc --disable-programs \
			--disable-everything --disable-avdevice --disable-avformat --disable-swscale --disable-avfilter \
			--disable-symver $asm --disable-vulkan --disable-v4l2-m2m --enable-swresample $enable \
			>/dev/null
		make -s -j"$jobs" >/dev/null
		make -s install-libs install-headers >/dev/null
	)
	mkdir -p "$out/jniLibs/$abi"
	# The JNI layer as Media3's CMakeLists.txt builds it, with libc++
	# linked in and 16 KB pages; FFmpeg's symbols stay inside, apart from
	# libmpv's FFmpeg in the same process.
	"$bin/$triple$api-clang++" -std=c++11 -shared -fPIC -O2 -fvisibility=hidden -fvisibility-inlines-hidden \
		-I"$prefix/include" "$ext/jni/ffmpeg_jni.cc" -o "$out/jniLibs/$abi/libffmpegJNI.so" \
		-L"$prefix/lib" -lswresample -lavcodec -lavutil -landroid -llog -static-libstdc++ \
		-Wl,-Bsymbolic -Wl,--exclude-libs,ALL -Wl,-z,max-page-size=16384 -Wl,--gc-sections -s
done

cp -R "$ext/java/." "$out/java/"
# Its experimental video renderer has no decoder here; Media3 loads it by
# reflection and does without it.
rm "$out/java/androidx/media3/decoder/ffmpeg/ExperimentalFfmpegVideoRenderer.java"
echo "$stamp" >"$out/stamp"
echo "$out"

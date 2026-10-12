#!/bin/sh
# Builds the shared core (libs/client-rust, crate mavio-uniffi) for Android
# into core/build/rust: jniLibs/<abi>/libmavio.so, the Kotlin API UniFFI
# generates in kotlin/, and host/, the core built for this machine, which
# the unit tests load. Usage: scripts/build-core.sh
# MAVIO_CORE_PROFILE picks the Cargo profile, release by default; CI builds
# with dev, which compiles much faster.
set -eu
cd "$(dirname "$0")/.."
android=$(pwd)
core=$android/../client-rust
out=$android/core/build/rust
api=26
ndk_version=30.0.16248370
profile=${MAVIO_CORE_PROFILE:-release}
build=$profile
[ "$profile" = dev ] && build=debug

sdk=${ANDROID_HOME:-${ANDROID_SDK_ROOT:-$HOME/Library/Android/sdk}}
ndk=$sdk/ndk/$ndk_version
if [ ! -d "$ndk" ]; then
	yes | "$sdk/cmdline-tools/latest/bin/sdkmanager" --install "ndk;$ndk_version" >/dev/null
fi
case $(uname -s) in Darwin) host=darwin-x86_64 ;; *) host=linux-x86_64 ;; esac
bin=$ndk/toolchains/llvm/prebuilt/$host/bin

cd "$core"
rustup target add aarch64-linux-android armv7-linux-androideabi x86_64-linux-android >/dev/null
rm -rf "$out" && mkdir -p "$out/jniLibs" "$out/host"
specs="aarch64-linux-android:aarch64-linux-android:arm64-v8a
armv7-linux-androideabi:armv7a-linux-androideabi:armeabi-v7a
x86_64-linux-android:x86_64-linux-android:x86_64"
# One build for every ABI, so that Cargo compiles them in parallel, each
# target with its NDK linker and compiler: target:clang-prefix:ABI.
set --
for spec in $specs; do
	target=${spec%%:*} rest=${spec#*:}
	clang=${rest%%:*}
	env=$(echo "$target" | tr '[:lower:]-' '[:upper:]_')
	export "CARGO_TARGET_${env}_LINKER=$bin/$clang$api-clang" "CC_$target=$bin/$clang$api-clang" "AR_$target=$bin/llvm-ar"
	set -- "$@" --target "$target"
done
cargo build --locked --profile "$profile" -p mavio-uniffi --lib "$@"
for spec in $specs; do
	target=${spec%%:*} abi=${spec##*:}
	mkdir -p "$out/jniLibs/$abi"
	cp "target/$target/$build/libmavio.so" "$out/jniLibs/$abi/"
done

cargo build --locked --profile "$profile" -p mavio-uniffi --lib
case $(uname -s) in Darwin) lib=libmavio.dylib ;; *) lib=libmavio.so ;; esac
cp "target/$build/$lib" "$out/host/"
cargo run --locked --profile "$profile" -q --bin uniffi-bindgen -- generate \
	--library "target/$build/$lib" --language kotlin --no-format --out-dir "$out/kotlin"
echo "$out"

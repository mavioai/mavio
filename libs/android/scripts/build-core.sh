#!/bin/sh
# Builds the shared core (libs/client-rust, crate mavio-uniffi) for Android
# into core/build/rust: jniLibs/<abi>/libmavio.so, the Kotlin API UniFFI
# generates in kotlin/, and host/, the core built for this machine, which
# the unit tests load. Usage: scripts/build-core.sh
set -eu
cd "$(dirname "$0")/.."
android=$(pwd)
core=$android/../client-rust
out=$android/core/build/rust
api=26
ndk_version=30.0.16248370

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
# target clang-prefix ABI
for spec in aarch64-linux-android:aarch64-linux-android:arm64-v8a \
	armv7-linux-androideabi:armv7a-linux-androideabi:armeabi-v7a \
	x86_64-linux-android:x86_64-linux-android:x86_64; do
	target=${spec%%:*} rest=${spec#*:}
	clang=${rest%%:*} abi=${rest#*:}
	env=$(echo "$target" | tr '[:lower:]-' '[:upper:]_')
	env "CARGO_TARGET_${env}_LINKER=$bin/$clang$api-clang" "CC_$target=$bin/$clang$api-clang" "AR_$target=$bin/llvm-ar" \
		cargo build --locked --release -p mavio-uniffi --lib --target "$target"
	mkdir -p "$out/jniLibs/$abi"
	cp "target/$target/release/libmavio.so" "$out/jniLibs/$abi/"
done

cargo build --locked --release -p mavio-uniffi --lib
case $(uname -s) in Darwin) lib=libmavio.dylib ;; *) lib=libmavio.so ;; esac
cp "target/release/$lib" "$out/host/"
cargo run --locked --release -q --bin uniffi-bindgen -- generate \
	--library "target/release/$lib" --language kotlin --no-format --out-dir "$out/kotlin"
echo "$out"

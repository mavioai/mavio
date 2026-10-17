#!/bin/sh
# Copies the shared libraries the app ships with into its resources: libmpv
# with MoltenVK from mavio-deps, and the core that client-rust:build built
# for this machine.
# Usage: scripts/resources.sh
set -eu
cd "$(dirname "$0")/.."
os=$(go env GOOS) arch=$(go env GOARCH)
case $os in
darwin)
	sh ../../tools/deps/fetch.sh mpv-macos >/dev/null
	mkdir -p resources/darwin/lib
	cp build/mpv-macos/lib/libmpv.2.dylib build/mpv-macos/lib/libMoltenVK.dylib resources/darwin/lib/
	core=libmaviocore.dylib
	;;
windows) core=maviocore.dll ;;
*) core=libmaviocore.so ;;
esac
mkdir -p "resources/$os-$arch/lib"
cp "../../libs/client-rust/target/debug/$core" "resources/$os-$arch/lib/"

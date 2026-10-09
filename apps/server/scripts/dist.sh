#!/bin/sh
# Builds the server for every supported platform into dist/, as releases
# ship it: static (CGO_ENABLED=0), without local paths, stamped with
# $VERSION when set. Usage: scripts/dist.sh [os/arch ...]
set -eu
cd "$(dirname "$0")/.."

targets=${*:-"linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"}
ldflags="-s -w"
if [ -n "${VERSION:-}" ]; then
	ldflags="$ldflags -X github.com/mavioai/mavio/apps/server/internal/buildinfo.version=$VERSION"
fi

rm -rf dist
for target in $targets; do
	os=${target%/*}
	arch=${target#*/}
	ext=
	[ "$os" = windows ] && ext=.exe
	out="dist/mavio-$os-$arch$ext"
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "$ldflags" -o "$out" ./cmd/mavio
	echo "$out"
done

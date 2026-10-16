#!/bin/sh
# Builds the server for every supported platform into dist/, as releases
# ship it: static (CGO_ENABLED=0), without local paths, stamped with
# $VERSION when set, beside Mavio's fork of rclone, the build of the release
# mise.toml pins for the platform, checked against its SHA-256. Each
# platform is a folder, dist/mavio-<os>-<arch>, and its archive, a .zip for
# Windows and a .tar.gz otherwise. Downloads of rclone are kept in
# $MAVIO_DIST_CACHE, by default ~/.cache/mavio-dist.
# Usage: scripts/dist.sh [os/arch ...]
set -eu
cd "$(dirname "$0")/.."

targets=${*:-"linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64"}
ldflags="-s -w"
if [ -n "${VERSION:-}" ]; then
	ldflags="$ldflags -X github.com/mavioai/mavio/apps/server/internal/buildinfo.version=$VERSION"
fi
cache=${MAVIO_DIST_CACHE:-${XDG_CACHE_HOME:-$HOME/.cache}/mavio-dist}

# rclone_pin prints the URL and SHA-256 of the rclone build mise.toml pins
# for a mise platform, such as macos-arm64.
rclone_pin() {
	awk -v p="$1" '
		/^\[tools\."http:rclone"\.platforms\]/ { on = 1; next }
		/^\[/ { on = 0 }
		on && $1 == p {
			match($0, /url = "[^"]+"/); url = substr($0, RSTART + 7, RLENGTH - 8)
			match($0, /checksum = "sha256:[0-9a-f]+"/); sum = substr($0, RSTART + 19, RLENGTH - 20)
			print url, sum
		}
	' ../../mise.toml
}

sha256() {
	if command -v sha256sum >/dev/null; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d' ' -f1
}

# fetch_rclone fetches the rclone build of a platform into a folder.
fetch_rclone() {
	os=$1 arch=$2 ext=$3 dir=$4
	case $os in darwin) p=macos ;; *) p=$os ;; esac
	case $arch in amd64) p=$p-x64 ;; *) p=$p-$arch ;; esac
	pin=$(rclone_pin "$p")
	[ -n "$pin" ] || { echo "mise.toml pins no rclone for $p" >&2; return 1; }
	url=${pin% *} sum=${pin#* }
	zip="$cache/$(basename "$url")"
	if [ ! -f "$zip" ] || [ "$(sha256 "$zip")" != "$sum" ]; then
		mkdir -p "$cache"
		curl -fsSLo "$zip.part" "$url"
		mv "$zip.part" "$zip"
	fi
	[ "$(sha256 "$zip")" = "$sum" ] || { echo "$zip: SHA-256 is not $sum" >&2; return 1; }
	unzip -qjo "$zip" "*/rclone$ext" -d "$dir"
}

rm -rf dist
mkdir -p dist
# The platforms build in parallel, as each build's tail is serial.
pids=
for target in $targets; do
	os=${target%/*}
	arch=${target#*/}
	ext=
	[ "$os" = windows ] && ext=.exe
	name=mavio-$os-$arch
	(
		CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "$ldflags" -o "dist/$name/mavio$ext" ./cmd/mavio
		fetch_rclone "$os" "$arch" "$ext" "dist/$name"
		if [ "$os" = windows ]; then
			(cd dist && zip -qr "$name.zip" "$name")
			echo "dist/$name.zip"
		else
			tar -czf "dist/$name.tar.gz" -C dist "$name"
			echo "dist/$name.tar.gz"
		fi
	) &
	pids="$pids $!"
done
failed=0
for pid in $pids; do
	wait "$pid" || failed=1
done
exit $failed

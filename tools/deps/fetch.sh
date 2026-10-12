#!/bin/sh
# Downloads the release asset of a library deps.txt pins from mavio-deps,
# checks its SHA-256 and extracts it into its directory; a library already
# extracted at its pin is left as it is. Downloads are kept in dist/deps.
# Usage: tools/deps/fetch.sh <library>
set -eu
cd "$(dirname "$0")/../.."
root=$(pwd)
library=$1

line=$(awk -v l="$library" '$1 == l { print; exit }' tools/deps/deps.txt)
[ -n "$line" ] || { echo "deps.txt pins no library $library" >&2; exit 1; }
set -- $line
tag=$2 asset=$3 sha256=$4 dir=$root/$5

stamp=$dir/.$library.stamp
if [ "$(cat "$stamp" 2>/dev/null)" = "$tag $sha256" ]; then
	exit 0
fi

download=$root/dist/deps/$asset
digest() { { sha256sum "$1" 2>/dev/null || shasum -a 256 "$1"; } | cut -d' ' -f1; }
if [ ! -f "$download" ] || [ "$(digest "$download")" != "$sha256" ]; then
	mkdir -p "$root/dist/deps"
	curl -fsSL --retry 5 --retry-delay 10 --retry-all-errors -o "$download.part" \
		"https://github.com/mavioai/mavio-deps/releases/download/$tag/$asset"
	got=$(digest "$download.part")
	if [ "$got" != "$sha256" ]; then
		echo "$asset: got SHA-256 $got, want $sha256" >&2
		rm -f "$download.part"
		exit 1
	fi
	mv "$download.part" "$download"
fi

# The asset holds one top-level directory, which replaces the one there.
case $asset in
*.zip) top=$(unzip -Z1 "$download" | head -1) ;;
*) top=$(tar -tzf "$download" | head -1) ;;
esac
top=${top%%/*}
mkdir -p "$dir"
rm -rf "${dir:?}/$top"
case $asset in
*.zip) unzip -q "$download" -d "$dir" ;;
*) tar -xzf "$download" -C "$dir" ;;
esac
echo "$tag $sha256" >"$stamp"
echo "$dir/$top"
